-- Rebuilds the DSR ingest schema after 019_drop_dsr_tables.sql.
-- Scope: sheets 'Daily' and 'Rencana Isi' only. Other sheets in the workbook
-- (Berita Acara, REKON ATM/CRM/CDM, VALUES, penyelesaian klaim, DSR KASET) are
-- deliberately NOT modelled.
--
-- Source sample: DSR_DATA/Laporan Saldo Harian DSR CIMB NIAGA BIJAK JAKARTA
--                Tanggal 15 Juli 2026.xlsx
--
-- Changes vs the dropped 013 + 017 + 018 design:
--   1. ONE upload row per workbook instead of two parallel *_files tables. Both
--      sheets arrive in the same file (same checksum, vendor, report_date); two
--      header tables duplicated filename/checksum/status/uploaded_by and let the
--      pair drift apart. Per-sheet ingest state lives in columns on the one row.
--   2. (unchanged) Amounts are stored VERBATIM as printed on the sheet, no
--      rescaling -- same convention as 013/017. Note the sheets label their
--      money columns "x 1.000" (Daily "Total Rupiah x1.000", Rencana Isi
--      "(x 1.000)") and carry helper columns holding value/1000, so a printed
--      250000 may mean Rp250,000,000. Any x1.000 interpretation is a READ-side
--      decision; this schema keeps the vendor's number untouched as evidence.
--   3. The hard FK to atms.terminal_id is gone. It contradicted the nullable
--      atm_id soft link on the same table: CRM terminals (ZZVX, ZZWU, ZZUF)
--      missing from master data would abort the whole ingest.
--
-- CROSS-CHECKS the ingest should assert (store the deltas, never silently fix):
--   * daily line_total_idr = SUM(denom_100k_idr..denom_1k_idr) when no denom is NULL
--   * saldo_akhir_0000_idr = saldo_awal + SUM(penerimaan d0) + SUM(pengeluaran d0)
--   * rencana_isi_subtotal_idr = SUM(fill_100k_idr + fill_50k_idr), and equals the
--     Daily d1 "Subtotal Pengeluaran" total
--     (sample: 4,800,000 + 21,400,000 = 26,200,000 on both sheets)
--
-- BROKEN CELLS: vendor workbooks contain '#REF!' cells. An unreadable amount
-- ingests as NULL and increments the sheet's error_count rather than failing the
-- batch. DERIVED ROWS ARE NOT STORED: Subtotal / SALDO AKHIR / SALDO SEMENTARA /
-- TOTAL / Sub Total are spreadsheet SUMs, recomputed on read.

BEGIN;

-- ============================================================
-- 1. One row per ingested workbook (covers both sheets)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.dsr_uploads
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    filename text NOT NULL,
    checksum text,
    vendor text NOT NULL,
    report_date date NOT NULL,
    uploaded_by_user_id bigint,

    -- header block of the 'Daily' sheet (rows 1-6), verbatim
    bank text,
    company text,
    recipient text,
    sender text,
    subject text,
    prepared_by text,
    checked_by text,
    approved_by text,

    currency text NOT NULL DEFAULT 'IDR',

    -- per-sheet ingest state: either sheet may succeed alone
    daily_status text NOT NULL DEFAULT 'pending',
    daily_row_count integer,
    daily_error_count integer,
    rencana_isi_status text NOT NULL DEFAULT 'pending',
    rencana_isi_row_count integer,
    rencana_isi_error_count integer,
    error_message text,

    -- dates and vendor-stated totals, kept for cross-check (verbatim as printed)
    daily_status_date date,
    rencana_isi_plan_date date,
    saldo_akhir_0000_idr numeric(20, 2),
    saldo_sementara_0900_idr numeric(20, 2),
    status_cadangan_idr numeric(20, 2),
    saldo_gabungan_idr numeric(20, 2),
    rencana_isi_subtotal_idr numeric(20, 2),

    processed_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),

    CONSTRAINT dsr_uploads_pkey PRIMARY KEY (id),
    CONSTRAINT dsr_uploads_checksum_uq UNIQUE (checksum),
    CONSTRAINT dsr_uploads_date_vendor_uq UNIQUE (report_date, vendor),
    CONSTRAINT dsr_uploads_daily_status_chk
        CHECK (daily_status IN ('pending', 'processing', 'completed', 'failed')),
    CONSTRAINT dsr_uploads_rencana_isi_status_chk
        CHECK (rencana_isi_status IN ('pending', 'processing', 'completed', 'failed')),
    CONSTRAINT fk_dsr_uploads_uploaded_by FOREIGN KEY (uploaded_by_user_id)
        REFERENCES public.users (id) ON DELETE SET NULL
);

COMMENT ON TABLE public.dsr_uploads
    IS 'One row per ingested DSR workbook. Idempotent per checksum; re-ingest = delete this row, both sheets child rows cascade.';
COMMENT ON COLUMN public.dsr_uploads.report_date
    IS 'Daily sheet Tanggal cell (e.g. 2026-07-15). Cross-checked against the filename.';
COMMENT ON COLUMN public.dsr_uploads.vendor
    IS 'Vendor label as printed (e.g. BIJAK JAKARTA). Not yet FK to vendors.code.';
COMMENT ON COLUMN public.dsr_uploads.uploaded_by_user_id
    IS 'users.id of the vendor user who uploaded, resolved server-side from the JWT. NULL for manual backfill.';
COMMENT ON COLUMN public.dsr_uploads.daily_status_date
    IS 'Section-2 provisional date on Daily (STATUS SALDO SEMENTARA), normally report_date + 1.';
COMMENT ON COLUMN public.dsr_uploads.rencana_isi_plan_date
    IS 'Tanggal cell on Rencana Isi: the day being planned, normally report_date + 1.';
COMMENT ON COLUMN public.dsr_uploads.rencana_isi_subtotal_idr
    IS 'Vendor-stated Sub Total of the fill plan. Must equal the Daily d1 Subtotal Pengeluaran total -- the one cross-sheet tie in the workbook.';

-- ============================================================
-- 2. 'Daily' sheet: leaf line-items of the vault cash statement
-- ============================================================
-- Wide denom layout on purpose: the denom set is fixed at 7 (100k..1k) and this
-- table exists to reproduce and validate the printed statement row-for-row.
CREATE TABLE IF NOT EXISTS public.dsr_daily_rows
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    upload_id bigint NOT NULL,
    row_no integer NOT NULL,
    location text,
    section text NOT NULL,
    flow text NOT NULL,
    line_label text NOT NULL,
    memo_no text,
    denom_100k_idr numeric(20, 2),
    denom_50k_idr numeric(20, 2),
    denom_20k_idr numeric(20, 2),
    denom_10k_idr numeric(20, 2),
    denom_5k_idr numeric(20, 2),
    denom_2k_idr numeric(20, 2),
    denom_1k_idr numeric(20, 2),
    line_total_idr numeric(20, 2),
    remarks text,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT dsr_daily_rows_pkey PRIMARY KEY (id),
    CONSTRAINT dsr_daily_rows_upload_row_uq UNIQUE (upload_id, row_no),
    CONSTRAINT dsr_daily_rows_section_chk CHECK (section IN ('d0', 'd1')),
    CONSTRAINT dsr_daily_rows_flow_chk
        CHECK (flow IN ('saldo_awal', 'penerimaan', 'pengeluaran', 'status_cadangan')),
    CONSTRAINT fk_dsr_daily_rows_upload FOREIGN KEY (upload_id)
        REFERENCES public.dsr_uploads (id) ON DELETE CASCADE
);

COMMENT ON COLUMN public.dsr_daily_rows.row_no
    IS 'Sheet line order (1..n) across both sections, excluding headers and derived subtotal/saldo rows. Stable ordering key for reproducing the statement.';
COMMENT ON COLUMN public.dsr_daily_rows.location
    IS 'Vault/site block header the row belongs to (e.g. LENTENG AGUNG, BINTARO). NULL for single-vault workbooks like BIJAK, which print one block.';
COMMENT ON COLUMN public.dsr_daily_rows.section
    IS 'd0 = section 1 (report_date, settled, sampai pukul 00:00). d1 = section 2 (daily_status_date, status sementara).';
COMMENT ON COLUMN public.dsr_daily_rows.flow
    IS 'saldo_awal = opening balance (d0 only). penerimaan / pengeluaran = receipt / disbursement line, sign stored verbatim as printed. status_cadangan = STATUS UANG CADANGAN ATM lines (Layak Edar / Rusak), d1 only.';
COMMENT ON COLUMN public.dsr_daily_rows.line_label
    IS 'Uraian text verbatim, e.g. Dari CIMB Niaga CIT, Untuk Cartridge Replenishment ATM & CRM.';
COMMENT ON COLUMN public.dsr_daily_rows.memo_no
    IS 'Memo/reference embedded in a line label, e.g. 313/ATM/BIJAK/VII/2026. NULL when the line carries none.';
COMMENT ON COLUMN public.dsr_daily_rows.denom_100k_idr
    IS 'Value attributable to the 100,000 denomination, verbatim as printed (the sheet labels this block "Denom (lembar)" but prints money). NULL when the source cell was an Excel error -- counts into daily_error_count.';
COMMENT ON COLUMN public.dsr_daily_rows.line_total_idr
    IS 'Total Rupiah x1.000 column, verbatim as printed. Cross-check: equals SUM(denom_*_idr) when no denom cell is NULL.';

CREATE INDEX IF NOT EXISTS idx_dsr_daily_rows_upload
    ON public.dsr_daily_rows (upload_id);
CREATE INDEX IF NOT EXISTS idx_dsr_daily_rows_section_flow
    ON public.dsr_daily_rows (upload_id, section, flow);

-- ============================================================
-- 3. 'Rencana Isi' sheet: next-day per-ATM fill plan
-- ============================================================
CREATE TABLE IF NOT EXISTS public.dsr_rencana_isi_rows
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
    upload_id bigint NOT NULL,
    row_no integer NOT NULL,
    atm_terminal_id text NOT NULL,
    atm_id bigint,
    atm_location text,
    denom_config text,
    fill_100k_idr numeric(20, 2),
    fill_50k_idr numeric(20, 2),
    splank_balance_0800_idr numeric(20, 2),
    remarks text,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT dsr_rencana_isi_rows_pkey PRIMARY KEY (id),
    CONSTRAINT dsr_rencana_isi_rows_upload_row_uq UNIQUE (upload_id, row_no),
    CONSTRAINT fk_dsr_rencana_isi_rows_upload FOREIGN KEY (upload_id)
        REFERENCES public.dsr_uploads (id) ON DELETE CASCADE,
    -- Soft link only. A vendor file must ingest even when the ATM is unknown to
    -- master data (CRM terminals ZZVX/ZZWU/ZZUF in the sample), so the row lands
    -- with atm_id = NULL. SET NULL, never CASCADE: retiring an ATM must not
    -- delete the vendor's plan rows -- they are operational evidence.
    CONSTRAINT fk_dsr_rencana_isi_rows_atm FOREIGN KEY (atm_id)
        REFERENCES public.atms (id) ON DELETE SET NULL
);

-- Keyed on row_no, not atm_terminal_id: a vendor may legitimately list the same
-- ATM twice in one plan. Preserve both, dedupe downstream.
COMMENT ON COLUMN public.dsr_rencana_isi_rows.atm_terminal_id
    IS 'ATM ID column as sent by the vendor (e.g. 2440, A353, ZZVX). Verbatim, never overwritten -- this is the evidence of what the vendor claimed.';
COMMENT ON COLUMN public.dsr_rencana_isi_rows.atm_id
    IS 'Resolved at ingest from atm_terminal_id. NULL = ATM not in master data; the file still ingests (counts into rencana_isi_error_count). Find unresolved rows WHERE atm_id IS NULL.';
COMMENT ON COLUMN public.dsr_rencana_isi_rows.atm_location
    IS 'Lokasi ATM column verbatim, e.g. JKT.AMBASADOR 1.';
COMMENT ON COLUMN public.dsr_rencana_isi_rows.denom_config
    IS 'Denom (x 1.000) column as printed: 50 | 100 | TST. TST rows are recyclers and carry both fill_100k_idr and fill_50k_idr.';
COMMENT ON COLUMN public.dsr_rencana_isi_rows.splank_balance_0800_idr
    IS 'Saldo Splank Pukul 08:00 column, verbatim as printed. 0 on TST/recycler rows in the sample.';
COMMENT ON COLUMN public.dsr_rencana_isi_rows.remarks
    IS 'Keterangan column verbatim, e.g. PENDING DSR TGL 16/07/2026.';

CREATE INDEX IF NOT EXISTS idx_dsr_rencana_isi_rows_upload
    ON public.dsr_rencana_isi_rows (upload_id);
CREATE INDEX IF NOT EXISTS idx_dsr_rencana_isi_rows_terminal
    ON public.dsr_rencana_isi_rows (atm_terminal_id);
-- Serves the atms join, the WHERE atm_id IS NULL unresolved check, and the
-- ON DELETE SET NULL cascade.
CREATE INDEX IF NOT EXISTS idx_dsr_rencana_isi_rows_atm
    ON public.dsr_rencana_isi_rows (atm_id);

COMMIT;
