-- DSR "SALDO HARIAN ATM" -- vendor daily ATM cash-balance statement.
-- Source: DSR_DATA/Laporan Saldo Harian DSR <BANK> <VENDOR> Tanggal <D-Month-YYYY>.xlsx
-- One workbook = one report = one sheet ('Daily').
--
-- WHAT THE SHEET IS (see the SALDO HARIAN ATM grid):
--   A denomination-by-denomination cash-flow statement across 7 denom columns
--   (100k, 50k, 20k, 10k, 5k, 2k, 1k) plus a total column. It is printed as two dated
--   sections:
--     Section 1 (report_date, "sampai pukul 00:00"): settled movements
--        SALDO AWAL
--        Penerimaan (receipts): reconciliation cartridge, bongkaran CDM, uang nyangkut CRM,
--                               dari CIMB CIT, penyelesaian klaim nasabah,
--                               penyelesaian klaim selisih kurang fisik (carries a memo no)
--        Pengeluaran (disbursements): cartridge replenishment, setoran ke cash management
--        SALDO AKHIR SAMPAI PUKUL 00:00
--     Section 2 (report_date + 1, "status sementara"): provisional next-day movements
--        Penerimaan (rekonsiliasi cartridge, batal replenishment, lain-lain)
--        Pengeluaran (replenishment, kas cadangan, lain-lain)
--        SALDO SEMENTARA SAMPAI PUKUL 09:00
--        STATUS UANG CADANGAN ATM: Kondisi Layak Edar / Rusak & Tidak Layak Edar / TOTAL
--
-- UNITS (read before writing any ingest code):
--   The workbook's total column is LABELLED "Total Rupiah x1.000", but the per-denom cells
--   already hold FULL RUPIAH values: on the sample, SALDO AWAL 100k=18,574,100 and
--   50k=21,913,800 sum to the printed total 40,487,900. So the "x1.000" is a display note on
--   the total column, not a scale applied to the stored denom cells. Store every denom_* and
--   the *_total columns as FULL IDR, no x1000 conversion. If a future file proves otherwise,
--   fix it in the parser, not here.
--
-- SIGNS: Pengeluaran (disbursement) line items are stored NEGATIVE on the sheet
--   (saldo_akhir = saldo_awal + subtotal_penerimaan + subtotal_pengeluaran works by addition).
--   Keep the vendor's sign verbatim; do not flip it on ingest.
--
-- DERIVED ROWS ARE NOT STORED: every Subtotal / SALDO AWAL-carry / SALDO AKHIR / SALDO
--   SEMENTARA / TOTAL row on the sheet is a spreadsheet formula (=SUM(...), =F12+F19+F22).
--   We store only the leaf line-items that carry entered values, plus SALDO AWAL (the one
--   balance that is an input, not a sum). Subtotals and closing balances are recomputed with
--   SUM() on read; the vendor-stated closing balances land on atm_dsr_saldo_files for an
--   ingest cross-check.
--
-- BROKEN CELLS: this real-world sample is riddled with '#REF!' (broken cross-sheet formulas
--   in the vendor's own workbook). denom_* columns are therefore NULLABLE: a cell that reads
--   as an Excel error ingests as NULL and increments atm_dsr_saldo_files.error_count, rather
--   than aborting the whole EOD batch. Query unparseable reports with WHERE error_count > 0.

BEGIN;

-- ============================================================
-- 1. One row per ingested report file (tracking + report header)
-- ============================================================
CREATE TABLE IF NOT EXISTS public.atm_dsr_saldo_files
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    filename text COLLATE pg_catalog."default" NOT NULL,
    checksum text COLLATE pg_catalog."default",
    status text COLLATE pg_catalog."default" NOT NULL DEFAULT 'pending',
    report_date date NOT NULL,
    status_date date,
    bank text COLLATE pg_catalog."default",
    vendor text COLLATE pg_catalog."default" NOT NULL,
    company text COLLATE pg_catalog."default",
    recipient text COLLATE pg_catalog."default",
    sender text COLLATE pg_catalog."default",
    subject text COLLATE pg_catalog."default",
    prepared_by text COLLATE pg_catalog."default",
    checked_by text COLLATE pg_catalog."default",
    approved_by text COLLATE pg_catalog."default",
    currency text COLLATE pg_catalog."default" NOT NULL DEFAULT 'IDR',
    saldo_akhir_0000_total_idr numeric(20, 2),
    saldo_sementara_0900_total_idr numeric(20, 2),
    status_cadangan_total_idr numeric(20, 2),
    row_count integer,
    success_count integer,
    error_count integer,
    error_message text COLLATE pg_catalog."default",
    processed_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT atm_dsr_saldo_files_pkey PRIMARY KEY (id),
    CONSTRAINT atm_dsr_saldo_files_checksum_uq UNIQUE (checksum),
    CONSTRAINT atm_dsr_saldo_files_date_vendor_uq UNIQUE (report_date, vendor),
    CONSTRAINT atm_dsr_saldo_files_status_chk
        CHECK (status IN ('pending', 'processing', 'completed', 'failed'))
);

COMMENT ON TABLE public.atm_dsr_saldo_files
    IS 'One row per ingested SALDO HARIAN ATM workbook. Idempotent per checksum; re-ingest = delete this row, child rows cascade.';
COMMENT ON COLUMN public.atm_dsr_saldo_files.status
    IS 'pending | processing | completed | failed';
COMMENT ON COLUMN public.atm_dsr_saldo_files.report_date
    IS 'Section-1 date (Tanggal cell, e.g. 2026-07-15). Also parsed from the filename as a cross-check.';
COMMENT ON COLUMN public.atm_dsr_saldo_files.status_date
    IS 'Section-2 provisional date (STATUS SALDO SEMENTARA), normally report_date + 1.';
COMMENT ON COLUMN public.atm_dsr_saldo_files.vendor
    IS 'Vendor label as printed (e.g. BIJAK JAKARTA). Not yet FK to vendors.code.';
COMMENT ON COLUMN public.atm_dsr_saldo_files.company
    IS 'Company header cell, e.g. PT. Bintang Jasa Artha Kelola - Jakarta.';
COMMENT ON COLUMN public.atm_dsr_saldo_files.saldo_akhir_0000_total_idr
    IS 'Vendor-stated SALDO AKHIR SAMPAI PUKUL 00:00 total, full IDR. Ingest cross-check: must equal SALDO AWAL + SUM(penerimaan section d0) + SUM(pengeluaran section d0).';
COMMENT ON COLUMN public.atm_dsr_saldo_files.saldo_sementara_0900_total_idr
    IS 'Vendor-stated SALDO SEMENTARA SAMPAI PUKUL 09:00 total, full IDR. Recomputable cross-check.';
COMMENT ON COLUMN public.atm_dsr_saldo_files.status_cadangan_total_idr
    IS 'Vendor-stated TOTAL STATUS UANG CADANGAN ATM, full IDR (Layak Edar + Rusak).';

-- ============================================================
-- 2. Leaf line-items of the statement (sheet 'Daily'), wide denom layout
-- ============================================================
-- Wide (one column per denom) deliberately: the denom set on this report is fixed at 7
-- (100k..1k) and the whole point of the table is to reproduce and validate the printed
-- statement row-for-row. A long/normalized layout (row x denom, FK to denoms) buys nothing
-- here and makes the subtotal cross-check awkward.
CREATE TABLE IF NOT EXISTS public.atm_dsr_saldo_rows
(
    id bigint NOT NULL GENERATED ALWAYS AS IDENTITY ( INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1 ),
    file_id bigint NOT NULL,
    row_no integer NOT NULL,
    section text COLLATE pg_catalog."default" NOT NULL,
    flow text COLLATE pg_catalog."default" NOT NULL,
    line_label text COLLATE pg_catalog."default" NOT NULL,
    memo_no text COLLATE pg_catalog."default",
    denom_100k numeric(20, 2),
    denom_50k numeric(20, 2),
    denom_20k numeric(20, 2),
    denom_10k numeric(20, 2),
    denom_5k numeric(20, 2),
    denom_2k numeric(20, 2),
    denom_1k numeric(20, 2),
    line_total_idr numeric(20, 2),
    remarks text COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT atm_dsr_saldo_rows_pkey PRIMARY KEY (id),
    CONSTRAINT atm_dsr_saldo_rows_file_row_uq UNIQUE (file_id, row_no),
    CONSTRAINT atm_dsr_saldo_rows_section_chk
        CHECK (section IN ('d0', 'd1')),
    CONSTRAINT atm_dsr_saldo_rows_flow_chk
        CHECK (flow IN ('saldo_awal', 'penerimaan', 'pengeluaran', 'status_cadangan'))
);

COMMENT ON TABLE public.atm_dsr_saldo_rows
    IS 'Leaf line-items of the SALDO HARIAN ATM statement. Subtotal / SALDO AKHIR / SALDO SEMENTARA / TOTAL rows are NOT stored (they are spreadsheet SUMs, recomputed on read).';
COMMENT ON COLUMN public.atm_dsr_saldo_rows.row_no
    IS 'Sheet line order (1..n) across both sections, excluding header and derived subtotal/saldo rows. Stable ordering key for reproducing the statement.';
COMMENT ON COLUMN public.atm_dsr_saldo_rows.section
    IS 'd0 = section 1 (report_date, settled, sampai pukul 00:00). d1 = section 2 (report_date+1, status sementara).';
COMMENT ON COLUMN public.atm_dsr_saldo_rows.flow
    IS 'saldo_awal = opening balance (input, only in d0). penerimaan = receipt line. pengeluaran = disbursement line (values stored negative, verbatim). status_cadangan = STATUS UANG CADANGAN ATM lines (Layak Edar / Rusak & Tidak Layak Edar), d1 only.';
COMMENT ON COLUMN public.atm_dsr_saldo_rows.line_label
    IS 'Uraian text verbatim, e.g. "Dari CIMB Niaga CIT", "Untuk Cartridge Replenishment ATM & CRM", "Kondisi Rusak & Tidak Layak Edar".';
COMMENT ON COLUMN public.atm_dsr_saldo_rows.memo_no
    IS 'Memo/reference embedded in a line label, e.g. 313/ATM/BIJAK/VII/2026 on "Penyelesaian Klaim Selisih Kurang Fisik". NULL when the line carries no memo.';
COMMENT ON COLUMN public.atm_dsr_saldo_rows.denom_100k
    IS 'Value column for the 100,000 denomination, FULL IDR. NULL when the source cell was an Excel error (#REF!) -- counts into atm_dsr_saldo_files.error_count.';
COMMENT ON COLUMN public.atm_dsr_saldo_rows.line_total_idr
    IS 'Total Rupiah column for the line, FULL IDR. Ingest cross-check: must equal SUM(denom_100k..denom_1k) when no denom cell is NULL.';

CREATE INDEX IF NOT EXISTS idx_atm_dsr_saldo_rows_file
    ON public.atm_dsr_saldo_rows (file_id);
CREATE INDEX IF NOT EXISTS idx_atm_dsr_saldo_rows_section_flow
    ON public.atm_dsr_saldo_rows (file_id, section, flow);

-- ============================================================
-- 3. Foreign key (re-ingest = delete the file row, children cascade)
-- ============================================================
ALTER TABLE IF EXISTS public.atm_dsr_saldo_rows
    ADD CONSTRAINT fk_atm_dsr_saldo_rows_file FOREIGN KEY (file_id)
    REFERENCES public.atm_dsr_saldo_files (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE CASCADE;

COMMIT;
