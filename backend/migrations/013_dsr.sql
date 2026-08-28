-- DSR (Laporan Saldo Harian) vendor daily report ingest.
-- Source: DSR_DATA/Laporan Saldo Harian DSR <BANK> <VENDOR> Tanggal <D Month YYYY>.xlsx
-- Two sheets, one file = one report:
--   'Rencana Isi'                   -> dsr_replenish_plan_rows  (next-day fill plan per ATM)
--   'penyelesaian klaim sel kurang' -> dsr_shortage_claims      (physical shortage claim settlement)
--
-- UNITS (read before writing any ingest code):
--   The workbook prints most cash columns at x1.000 scale ("Denom (x 1.000)", "100 (x1000)").
--   Every *_amount_idr column here is FULL RUPIAH: multiply the sheet value by 1000 on ingest.
--   The one exception is the claim sheet NOMINAL column, which is already full rupiah -- see
--   the comment on dsr_shortage_claims.nominal_amount_idr. Mixing these up costs 3 zeroes.
--
-- Helper columns (the unnamed '1000' columns H/I/J on 'Rencana Isi') are derived and not stored.
-- 'Sub Total' / 'TOTAL' footer rows are not stored as rows; the vendor-stated plan subtotals
-- land on dsr_files for ingest cross-check, claim totals are recomputable with SUM().

BEGIN;

-- ============================================================
-- 1. One row per ingested report file (tracking + report header)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.dsr_files
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    filename text COLLATE pg_catalog."default" NOT NULL,
    checksum text COLLATE pg_catalog."default",
    status text COLLATE pg_catalog."default" NOT NULL DEFAULT 'pending',
    report_date date NOT NULL,
    plan_date date,
    vendor text COLLATE pg_catalog."default" NOT NULL,
    bank text COLLATE pg_catalog."default",
    prepared_by text COLLATE pg_catalog."default",
    checked_by text COLLATE pg_catalog."default",
    approved_by text COLLATE pg_catalog."default",
    currency text COLLATE pg_catalog."default" NOT NULL DEFAULT 'IDR',
    plan_total_100k_amount_idr numeric(20, 2),
    plan_total_50k_amount_idr numeric(20, 2),
    row_count integer,
    success_count integer,
    error_count integer,
    error_message text COLLATE pg_catalog."default",
    processed_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT dsr_files_pkey PRIMARY KEY (id),
    CONSTRAINT dsr_files_checksum_uq UNIQUE (checksum),
    CONSTRAINT dsr_files_date_vendor_uq UNIQUE (report_date, vendor)
);

COMMENT ON COLUMN public.dsr_files.status
    IS 'pending | processing | completed | failed';
COMMENT ON COLUMN public.dsr_files.report_date
    IS 'Report date from the filename (Tanggal 15 Juli 2026 -> 2026-07-15)';
COMMENT ON COLUMN public.dsr_files.plan_date
    IS 'Tanggal cell on the Rencana Isi sheet: the day being planned, normally report_date + 1';
COMMENT ON COLUMN public.dsr_files.vendor
    IS 'Vendor label as printed (e.g. BIJAK JAKARTA); not yet FK to vendors.code';
COMMENT ON COLUMN public.dsr_files.plan_total_100k_amount_idr
    IS 'Vendor-stated Sub Total row, full IDR. Ingest must verify it equals SUM(fill_100k_amount_idr).';
COMMENT ON COLUMN public.dsr_files.plan_total_50k_amount_idr
    IS 'Vendor-stated Sub Total row, full IDR. Ingest must verify it equals SUM(fill_50k_amount_idr).';

-- ============================================================
-- 2. Next-day replenishment plan (sheet 'Rencana Isi')
-- ============================================================
CREATE TABLE IF NOT EXISTS public.dsr_replenish_plan_rows
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    file_id bigint NOT NULL,
    row_no integer NOT NULL,
    atm_terminal_id text COLLATE pg_catalog."default" NOT NULL,
    atm_id bigint,
    atm_location text COLLATE pg_catalog."default",
    denom_config text COLLATE pg_catalog."default",
    fill_100k_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    fill_50k_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    splank_balance_0800_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    remarks text COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT dsr_replenish_plan_rows_pkey PRIMARY KEY (id),
    CONSTRAINT dsr_replenish_plan_rows_file_row_uq UNIQUE (file_id, row_no)
);

-- Keyed on row_no, not atm_terminal_id: the vendor legitimately sends the same ATM twice in
-- one plan (e.g. a zero-value row followed by the real one). Preserve both, dedupe downstream.
COMMENT ON COLUMN public.dsr_replenish_plan_rows.row_no
    IS 'Sheet line order (1..n), excluding header and Sub Total rows';
COMMENT ON COLUMN public.dsr_replenish_plan_rows.atm_terminal_id
    IS 'ATM ID column as sent by the vendor (e.g. 2440, A353, ZZVX). Kept verbatim, never overwritten -- this is the evidence of what the vendor claimed.';
COMMENT ON COLUMN public.dsr_replenish_plan_rows.atm_id
    IS 'Resolved at ingest from atm_terminal_id. NULL = ATM not in master data; the file still ingests (count them into dsr_files.error_count). Query unresolved rows with WHERE atm_id IS NULL.';
COMMENT ON COLUMN public.dsr_replenish_plan_rows.denom_config
    IS 'Denom (x 1.000) column as printed: 50 | 100 | TST. TST rows are recyclers and carry both 100k and 50k fills.';
COMMENT ON COLUMN public.dsr_replenish_plan_rows.splank_balance_0800_amount_idr
    IS 'Saldo Splank Pukul 08:00, full IDR (sheet value x 1000)';

CREATE INDEX IF NOT EXISTS idx_dsr_replenish_plan_rows_file
    ON public.dsr_replenish_plan_rows (file_id);
CREATE INDEX IF NOT EXISTS idx_dsr_replenish_plan_rows_terminal
    ON public.dsr_replenish_plan_rows (atm_terminal_id);
-- Serves both the atms join and the WHERE atm_id IS NULL unresolved-ATM check
-- (btree indexes NULLs), and backs the ON DELETE SET NULL cascade.
CREATE INDEX IF NOT EXISTS idx_dsr_replenish_plan_rows_atm
    ON public.dsr_replenish_plan_rows (atm_id);

-- ============================================================
-- 3. Physical shortage claims (sheet 'penyelesaian klaim sel kurang')
-- ============================================================
CREATE TABLE IF NOT EXISTS public.dsr_shortage_claims
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    file_id bigint NOT NULL,
    row_no integer NOT NULL,
    vendor text COLLATE pg_catalog."default" NOT NULL,
    claim_date date,
    nominal_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    atm_terminal_id text COLLATE pg_catalog."default",
    atm_id bigint,
    replenish_date date,
    due_date date,
    memo_no text COLLATE pg_catalog."default",
    resolution text COLLATE pg_catalog."default",
    remarks text COLLATE pg_catalog."default",
    settle_100k_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    settle_50k_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    settle_20k_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    settle_total_amount_idr numeric(20, 2) NOT NULL DEFAULT 0,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT dsr_shortage_claims_pkey PRIMARY KEY (id),
    CONSTRAINT dsr_shortage_claims_file_row_uq UNIQUE (file_id, row_no),
    CONSTRAINT dsr_shortage_claims_resolution_chk
        CHECK (resolution IS NULL OR resolution IN ('DITERIMA', 'DITOLAK', 'INVESTIGASI'))
);

COMMENT ON COLUMN public.dsr_shortage_claims.nominal_amount_idr
    IS 'NOMINAL column, ALREADY full rupiah on the sheet (do NOT x1000). Negative = selisih kurang.';
COMMENT ON COLUMN public.dsr_shortage_claims.resolution
    IS 'PENYELESAIAN column; vendor dropdown: DITERIMA | DITOLAK | INVESTIGASI. NULL = not yet decided.';
COMMENT ON COLUMN public.dsr_shortage_claims.settle_total_amount_idr
    IS 'TOTAL column, full IDR. Ingest must verify it equals settle_100k + settle_50k + settle_20k, and that ABS(nominal_amount_idr) matches it when resolution = DITERIMA.';
COMMENT ON COLUMN public.dsr_shortage_claims.memo_no
    IS 'MEMO column, e.g. 313/ATM/BIJAK/VII/2026';
COMMENT ON COLUMN public.dsr_shortage_claims.atm_terminal_id
    IS 'ID column as sent by the vendor. Kept verbatim, never overwritten.';
COMMENT ON COLUMN public.dsr_shortage_claims.atm_id
    IS 'Resolved at ingest from atm_terminal_id. NULL = ATM not in master data; the claim still ingests. Query unresolved rows with WHERE atm_id IS NULL.';

CREATE INDEX IF NOT EXISTS idx_dsr_shortage_claims_file
    ON public.dsr_shortage_claims (file_id);
CREATE INDEX IF NOT EXISTS idx_dsr_shortage_claims_terminal
    ON public.dsr_shortage_claims (atm_terminal_id);
CREATE INDEX IF NOT EXISTS idx_dsr_shortage_claims_memo
    ON public.dsr_shortage_claims (memo_no);
CREATE INDEX IF NOT EXISTS idx_dsr_shortage_claims_atm
    ON public.dsr_shortage_claims (atm_id);

-- ============================================================
-- 4. Foreign keys (re-ingest = delete the dsr_files row, children cascade)
-- ============================================================
ALTER TABLE IF EXISTS public.dsr_replenish_plan_rows
    ADD CONSTRAINT fk_dsr_replenish_plan_rows_file FOREIGN KEY (file_id)
    REFERENCES public.dsr_files (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE CASCADE;

ALTER TABLE IF EXISTS public.dsr_shortage_claims
    ADD CONSTRAINT fk_dsr_shortage_claims_file FOREIGN KEY (file_id)
    REFERENCES public.dsr_files (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE CASCADE;

-- Soft link to master data. Nullable on purpose: a vendor file must ingest even when the
-- ATM is unknown, so an unresolvable row lands with atm_id = NULL instead of aborting the
-- EOD batch. SET NULL (not CASCADE) on delete: retiring an ATM must never delete the
-- vendor's report rows -- they are financial evidence.
ALTER TABLE IF EXISTS public.dsr_replenish_plan_rows
    ADD CONSTRAINT fk_dsr_replenish_plan_rows_atm FOREIGN KEY (atm_id)
    REFERENCES public.atms (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE SET NULL;

ALTER TABLE IF EXISTS public.dsr_shortage_claims
    ADD CONSTRAINT fk_dsr_shortage_claims_atm FOREIGN KEY (atm_id)
    REFERENCES public.atms (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE SET NULL;

COMMIT;
