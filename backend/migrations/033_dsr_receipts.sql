-- 033_dsr_receipts.sql
-- dsr-late-report (Phase 2.3, .claude/sdlc/dsr-late-report/spec.md, S1): append-only
-- record of the first successful DSR receipt per vendor + report date. Written by
-- backend_python/dsr/dsr_etl.py (--mode commit) in the same tx as dsr_uploads.
-- Additive only. No FK to dsr_uploads: that row is replaced on revision.

BEGIN;

CREATE TABLE public.dsr_receipts (
    id                  bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_label        text        NOT NULL,
    vendor_id           bigint      REFERENCES public.vendors (id),
    report_date         date        NOT NULL,
    checksum            text        NOT NULL,
    locations           text[]      NOT NULL DEFAULT '{}',
    uploaded_by_user_id bigint      REFERENCES public.users (id),
    received_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT dsr_receipts_checksum_chk CHECK (checksum ~ '^[0-9a-f]{64}$'),
    CONSTRAINT dsr_receipts_uq UNIQUE (vendor_label, report_date, checksum)
);

CREATE INDEX dsr_receipts_date_vendor_idx ON public.dsr_receipts (report_date, vendor_id);

COMMENT ON TABLE public.dsr_receipts IS 'Append-only: one row per confirmed DSR commit (daily_status completed). Never updated or deleted; a revision adds a row. Basis of the DSR late report (first_received_at = MIN(received_at)).';
COMMENT ON COLUMN public.dsr_receipts.vendor_label IS 'dsr_uploads.vendor as uploaded (raw label).';
COMMENT ON COLUMN public.dsr_receipts.vendor_id IS 'Resolved at commit by vendors.name = vendor_label; NULL when unresolved (shown as a warning, counted for no branch).';
COMMENT ON COLUMN public.dsr_receipts.report_date IS 'DSR report date the upload covers.';
COMMENT ON COLUMN public.dsr_receipts.checksum IS 'SHA-256 hex of the uploaded file; same file committed again does not add a row.';
COMMENT ON COLUMN public.dsr_receipts.locations IS 'Distinct non-empty btrim(dsr_daily_rows.location) labels of that upload; mapped to vaults via dsr_location_vault_maps.';
COMMENT ON COLUMN public.dsr_receipts.uploaded_by_user_id IS 'Vendor user who confirmed the upload, if known.';
COMMENT ON COLUMN public.dsr_receipts.received_at IS 'Commit/confirmation time (spec K2), not file staging time.';

COMMIT;
