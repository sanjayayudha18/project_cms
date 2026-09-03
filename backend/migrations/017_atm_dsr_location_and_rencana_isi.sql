-- Extends the DSR ingest (013_dsr.sql) with:
--   (a) multi-location support on the 'Daily' sheet -- some vendor workbooks (e.g. TAG JAKARTA)
--       repeat the whole SALDO HARIAN ATM block once per vault/site (LENTENG AGUNG, BINTARO,
--       BEKASI, CIMONE, ...) inside a single sheet, ending in a derived SALDO GABUNGAN
--       (combined) row. The 013_dsr.sql schema assumed one location per file (true for
--       single-vault vendors like BIJAK) and has no location column at all.
--   (b) the 'Rencana Isi' sheet -- next-day per-ATM cash-fill plan. Not covered by any active
--       migration (a prior draft lived only in migrations/archives, never applied).
--
-- Source: DSR_DATA/Laporan Harian|Saldo Harian DSR <BANK> <VENDOR> Tanggal <D Month YYYY>.xls(x)
--
-- UNITS: 'Rencana Isi' fill/balance columns are FULL RUPIAH already on both sample workbooks
--   (TAG: 400000 = Rp400,000; BIJAK: 250000 = Rp250,000) despite the "(x 1.000)" header label --
--   same false-scale-label pattern as 013_dsr.sql's Daily sheet. Store verbatim, no x1000 scale.
--
-- DERIVED ROWS ARE NOT STORED: 'Sub Total' / 'TOTAL' rows on Rencana Isi are spreadsheet sums,
--   recomputed on read (same convention as atm_dsr_saldo_rows).

BEGIN;

-- ============================================================
-- 1. 'Daily' sheet: add location to support multi-vault workbooks
-- ============================================================
ALTER TABLE IF EXISTS public.atm_dsr_saldo_rows
    ADD COLUMN IF NOT EXISTS location text COLLATE pg_catalog."default";

COMMENT ON COLUMN public.atm_dsr_saldo_rows.location
    IS 'Vault/site block header the row belongs to, verbatim (e.g. LENTENG AGUNG, BINTARO, BEKASI, CIMONE). NULL for single-vault vendor workbooks (e.g. BIJAK) that print only one block.';

CREATE INDEX IF NOT EXISTS idx_atm_dsr_saldo_rows_file_location
    ON public.atm_dsr_saldo_rows (file_id, location);

ALTER TABLE IF EXISTS public.atm_dsr_saldo_files
    ADD COLUMN IF NOT EXISTS saldo_gabungan_total_idr numeric(20, 2);

COMMENT ON COLUMN public.atm_dsr_saldo_files.saldo_gabungan_total_idr
    IS 'Vendor-stated SALDO GABUNGAN (combined across all vault blocks) total, full IDR. Only present on multi-location workbooks; NULL on single-vault ones.';

-- ============================================================
-- 2. One row per ingested 'Rencana Isi' sheet-dataset (tracking + header)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.atm_dsr_rencana_isi_files
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    filename text COLLATE pg_catalog."default" NOT NULL,
    checksum text COLLATE pg_catalog."default",
    status text COLLATE pg_catalog."default" NOT NULL DEFAULT 'pending',
    report_date date NOT NULL,
    plan_date date,
    vendor text COLLATE pg_catalog."default" NOT NULL,
    currency text COLLATE pg_catalog."default" NOT NULL DEFAULT 'IDR',
    row_count integer,
    success_count integer,
    error_count integer,
    error_message text COLLATE pg_catalog."default",
    processed_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT atm_dsr_rencana_isi_files_pkey PRIMARY KEY (id),
    CONSTRAINT atm_dsr_rencana_isi_files_checksum_uq UNIQUE (checksum),
    CONSTRAINT atm_dsr_rencana_isi_files_date_vendor_uq UNIQUE (report_date, vendor),
    CONSTRAINT atm_dsr_rencana_isi_files_status_chk
        CHECK (status IN ('pending', 'processing', 'completed', 'failed'))
);

COMMENT ON TABLE public.atm_dsr_rencana_isi_files
    IS 'One row per ingested RENCANA PENGISIAN (Rencana Isi sheet) dataset. Idempotent per checksum; re-ingest = delete this row, child rows cascade.';
COMMENT ON COLUMN public.atm_dsr_rencana_isi_files.report_date
    IS 'Same workbook/report_date as the paired atm_dsr_saldo_files row (same file, different sheet). Not FK''d together: sheets are ingested independently and either may succeed alone.';
COMMENT ON COLUMN public.atm_dsr_rencana_isi_files.plan_date
    IS 'Tanggal cell on the Rencana Isi sheet: the day being planned, normally report_date + 1.';

-- ============================================================
-- 3. Leaf rows: next-day per-ATM fill plan (sheet 'Rencana Isi')
-- ============================================================
CREATE TABLE IF NOT EXISTS public.atm_dsr_rencana_isi_rows
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    file_id bigint NOT NULL,
    row_no integer NOT NULL,
    atm_terminal_id text COLLATE pg_catalog."default" NOT NULL,
    atm_id bigint,
    atm_location text COLLATE pg_catalog."default",
    denom_config text COLLATE pg_catalog."default",
    fill_100k_idr numeric(20, 2) NOT NULL DEFAULT 0,
    fill_50k_idr numeric(20, 2) NOT NULL DEFAULT 0,
    splank_balance_0800_idr numeric(20, 2) NOT NULL DEFAULT 0,
    remarks text COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT atm_dsr_rencana_isi_rows_pkey PRIMARY KEY (id),
    CONSTRAINT atm_dsr_rencana_isi_rows_file_row_uq UNIQUE (file_id, row_no)
);

-- Keyed on row_no, not atm_terminal_id: a vendor may legitimately list the same ATM twice
-- in one plan. Preserve both, dedupe downstream.
COMMENT ON COLUMN public.atm_dsr_rencana_isi_rows.row_no
    IS 'Sheet line order (1..n), excluding header and Sub Total/TOTAL rows.';
COMMENT ON COLUMN public.atm_dsr_rencana_isi_rows.atm_terminal_id
    IS 'ATM ID column as sent by the vendor (e.g. 5902, A353, ZZVX). Kept verbatim, never overwritten -- this is the evidence of what the vendor claimed.';
COMMENT ON COLUMN public.atm_dsr_rencana_isi_rows.atm_id
    IS 'Resolved at ingest from atm_terminal_id. NULL = ATM not in master data; the file still ingests (count into atm_dsr_rencana_isi_files.error_count). Query unresolved rows with WHERE atm_id IS NULL.';
COMMENT ON COLUMN public.atm_dsr_rencana_isi_rows.denom_config
    IS 'Denom / "50/100" column as printed: 50 | 100 | TST. TST rows are recyclers and carry both fill_100k_idr and fill_50k_idr.';
COMMENT ON COLUMN public.atm_dsr_rencana_isi_rows.splank_balance_0800_idr
    IS 'Saldo Splank Pukul 08:00 column, full IDR.';
COMMENT ON COLUMN public.atm_dsr_rencana_isi_rows.remarks
    IS 'Keterangan column verbatim, e.g. "PENDING DSR TGL 16/07/2026".';

CREATE INDEX IF NOT EXISTS idx_atm_dsr_rencana_isi_rows_file
    ON public.atm_dsr_rencana_isi_rows (file_id);
CREATE INDEX IF NOT EXISTS idx_atm_dsr_rencana_isi_rows_terminal
    ON public.atm_dsr_rencana_isi_rows (atm_terminal_id);
-- Serves both the atms join and the WHERE atm_id IS NULL unresolved-ATM check
-- (btree indexes NULLs), and backs the ON DELETE SET NULL cascade.
CREATE INDEX IF NOT EXISTS idx_atm_dsr_rencana_isi_rows_atm
    ON public.atm_dsr_rencana_isi_rows (atm_id);

-- ============================================================
-- 4. Foreign keys (re-ingest = delete the file row, children cascade)
-- ============================================================
ALTER TABLE IF EXISTS public.atm_dsr_rencana_isi_rows
    ADD CONSTRAINT fk_atm_dsr_rencana_isi_rows_file FOREIGN KEY (file_id)
    REFERENCES public.atm_dsr_rencana_isi_files (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE CASCADE;

-- Soft link to master data. Nullable on purpose: a vendor file must ingest even when the
-- ATM is unknown, so an unresolvable row lands with atm_id = NULL instead of aborting the
-- EOD batch. SET NULL (not CASCADE) on delete: retiring an ATM must never delete the
-- vendor's plan rows -- they are operational evidence.
ALTER TABLE IF EXISTS public.atm_dsr_rencana_isi_rows
    ADD CONSTRAINT fk_atm_dsr_rencana_isi_rows_atm FOREIGN KEY (atm_id)
    REFERENCES public.atms (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE SET NULL;

-- Hard link on the vendor's own terminal id, since atms.terminal_id is UNIQUE.
-- atm_terminal_id is NOT NULL (always present on the sheet) and this FK requires it to
-- already exist in atms -- a row whose vendor-sent ATM ID is unknown will now fail to
-- ingest, unlike the atm_id soft link above. Master data (atms) must be seeded/synced
-- before running an ingest that references it.
ALTER TABLE IF EXISTS public.atm_dsr_rencana_isi_rows
    ADD CONSTRAINT fk_atm_dsr_rencana_isi_rows_terminal FOREIGN KEY (atm_terminal_id)
    REFERENCES public.atms (terminal_id) MATCH SIMPLE
    ON UPDATE CASCADE
    ON DELETE NO ACTION;

COMMIT;
