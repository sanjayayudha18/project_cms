-- 034_export_jobs.sql
-- vendor-data-export (.kiro/specs/vendor-data-export/design.md, Data Models): async
-- export jobs run by backend-exq. export_jobs = one request (one CSV file) and its
-- technical status; export_job_chunks = the file itself, kept only until it is
-- downloaded once, fails, or expires 10 minutes after it is ready.
-- Additive only: no change to master data or audit_logs.

BEGIN;

CREATE TABLE public.export_jobs (
    id                  bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    requested_by        bigint      NOT NULL REFERENCES public.users(id),
    kind                text        NOT NULL CONSTRAINT export_jobs_kind_chk
                            CHECK (kind IN ('vendor', 'cabang', 'harga_paket', 'master_data_global')),
    entity              text        NOT NULL,
    vendor_id           bigint      REFERENCES public.vendors(id),
    vendor_code         text,
    vendor_branch_id    bigint      REFERENCES public.vendor_branches(id),
    branch_code         text,
    status_filter       text        NOT NULL CONSTRAINT export_jobs_status_filter_chk
                            CHECK (status_filter IN ('active', 'disabled', 'all')),
    price_level         text        CONSTRAINT export_jobs_price_level_chk
                            CHECK (price_level IN ('vendor', 'branch', 'atm', 'all')),
    params_hash         text        NOT NULL,
    file_name           text        NOT NULL,
    status              text        NOT NULL DEFAULT 'queued' CONSTRAINT export_jobs_status_chk
                            CHECK (status IN ('queued', 'running', 'succeeded', 'failed',
                                              'downloading', 'downloaded', 'expired')),
    row_count           bigint      NOT NULL DEFAULT 0,
    byte_size           bigint,
    chunk_count         integer,
    error_message       text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    started_at          timestamptz,
    finished_at         timestamptz,
    expires_at          timestamptz,
    download_started_at timestamptz,
    downloaded_at       timestamptz,
    expired_at          timestamptz,
    updated_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT export_jobs_scope_chk CHECK (
           (kind = 'master_data_global' AND vendor_id IS NULL     AND vendor_branch_id IS NULL)
        OR (kind = 'vendor'             AND vendor_id IS NOT NULL AND vendor_branch_id IS NULL)
        OR (kind = 'cabang'             AND vendor_id IS NOT NULL AND vendor_branch_id IS NOT NULL)
        OR (kind = 'harga_paket'        AND vendor_id IS NOT NULL)),
    CONSTRAINT export_jobs_price_level_kind_chk CHECK ((kind = 'harga_paket') = (price_level IS NOT NULL)),
    CONSTRAINT export_jobs_ready_chk CHECK (
        status NOT IN ('succeeded', 'downloading', 'downloaded')
        OR (finished_at IS NOT NULL AND expires_at IS NOT NULL AND byte_size IS NOT NULL AND chunk_count IS NOT NULL))
);

-- Dedupe permintaan identik yang masih diproses (Req 9.4), pengaman kedua di bawah advisory lock.
CREATE UNIQUE INDEX export_jobs_active_dedupe_uq
    ON public.export_jobs (requested_by, params_hash) WHERE status IN ('queued', 'running');
-- Antrian worker (Req 9.6).
CREATE INDEX export_jobs_queue_idx ON public.export_jobs (id) WHERE status = 'queued';
-- Daftar Ekspor Saya (Req 12.6).
CREATE INDEX export_jobs_requester_idx ON public.export_jobs (requested_by, created_at DESC, id DESC);
-- Janitor (Req 10.5-10.7).
CREATE INDEX export_jobs_janitor_idx ON public.export_jobs (status)
    WHERE status IN ('running', 'succeeded', 'downloading');

CREATE TRIGGER trg_export_jobs_set_updated_at BEFORE UPDATE ON public.export_jobs
    FOR EACH ROW WHEN (old.* IS DISTINCT FROM new.*) EXECUTE FUNCTION public.set_updated_at();

CREATE TABLE public.export_job_chunks (
    export_job_id bigint  NOT NULL REFERENCES public.export_jobs(id) ON DELETE CASCADE,
    seq           integer NOT NULL CONSTRAINT export_job_chunks_seq_chk CHECK (seq >= 0),
    data          bytea   NOT NULL,
    PRIMARY KEY (export_job_id, seq)
);

COMMENT ON TABLE public.export_jobs IS
    'Permintaan ekspor asinkron (backend-exq). Status teknis queued -> running -> succeeded|failed; succeeded -> downloading -> downloaded|succeeded; succeeded -> expired. Baris disimpan sebagai riwayat, file ada di export_job_chunks. Migrasi 034.';
COMMENT ON TABLE public.export_job_chunks IS
    'Isi file CSV ekspor per potongan (<= 1 MiB). Berisi data pribadi PIC / harga kontrak: dihapus setelah berhasil diunduh, saat gagal, atau 10 menit setelah siap. Migrasi 034.';

COMMIT;
