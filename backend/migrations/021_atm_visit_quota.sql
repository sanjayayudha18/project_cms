-- 021_atm_visit_quota.sql
-- Kuota kunjungan replenish per ATM + laporan selesai Vendor Request
-- (.claude/sdlc/atm-visit-quota/spec.md, accepted 2026-10-01).
--
--   * vendor_requests: status baru 'completion_pending' (approved ->
--     completion_pending -> completed / kembali approved bila ditolak) +
--     kolom aktor/waktu laporan selesai.
--   * vendor_request_atm_results: hasil berhasil/gagal per ATM dari laporan.
--   * atm_visit_quotas: sisa kunjungan per ATM (boleh negatif = kelebihan).
--   * atm_visits: log kunjungan; satu per (request, ATM); soft-cancel.
--
-- package_frequencies.cr_frequency TIDAK diubah: hanya dibaca sebagai kuota.
--
-- SAFETY:
--   * Additive, kecuali vendor_requests_status_chk yang di-drop+add dalam tx
--     yang sama dengan daftar nilai lama + 'completion_pending'.
--   * Forward-only: no down migration (project convention).
BEGIN;

ALTER TABLE public.vendor_requests DROP CONSTRAINT vendor_requests_status_chk;
ALTER TABLE public.vendor_requests ADD CONSTRAINT vendor_requests_status_chk CHECK (status = ANY (ARRAY[
    'draft'::text, 'pending_approval'::text, 'approved'::text, 'rejected'::text,
    'processing'::text, 'completed'::text, 'failed'::text, 'cancelled'::text,
    'completion_pending'::text]));

ALTER TABLE public.vendor_requests
    ADD COLUMN completion_submitted_by bigint REFERENCES public.users(id),
    ADD COLUMN completion_submitted_at timestamp with time zone,
    ADD COLUMN completion_approved_by bigint REFERENCES public.users(id),
    ADD COLUMN completion_approved_at timestamp with time zone,
    ADD COLUMN completion_rejected_by bigint REFERENCES public.users(id),
    ADD COLUMN completion_rejected_at timestamp with time zone,
    ADD COLUMN completion_rejection_reason text;

COMMENT ON COLUMN public.vendor_requests.completion_submitted_by IS
  'Maker laporan selesai replenish (ATM-USER). Checker laporan harus berbeda. Migrasi 021.';

CREATE TABLE public.vendor_request_atm_results (
    vendor_request_id bigint NOT NULL REFERENCES public.vendor_requests(id),
    terminal_id text NOT NULL,
    result text NOT NULL,
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT pk_vendor_request_atm_results PRIMARY KEY (vendor_request_id, terminal_id),
    CONSTRAINT vendor_request_atm_results_result_chk CHECK (result IN ('success', 'failed'))
);

COMMENT ON TABLE public.vendor_request_atm_results IS
  'Hasil laporan selesai per ATM (terminal distinct dari vendor_request_items). Ditimpa saat laporan diajukan ulang setelah ditolak. Migrasi 021.';

CREATE TABLE public.atm_visit_quotas (
    atm_id bigint NOT NULL REFERENCES public.atms(id),
    package_code text NOT NULL,
    quota_total integer NOT NULL,
    remaining integer NOT NULL,
    reset_at timestamp with time zone NOT NULL DEFAULT now(),
    reset_by bigint REFERENCES public.users(id),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT pk_atm_visit_quotas PRIMARY KEY (atm_id),
    CONSTRAINT atm_visit_quotas_total_chk CHECK (quota_total > 0)
);

COMMENT ON TABLE public.atm_visit_quotas IS
  'Sisa kunjungan replenish per ATM. quota_total = snapshot package_frequencies.cr_frequency saat reset/pembuatan; remaining dikurangi 1 per kunjungan, boleh negatif (= kelebihan kuota). Tidak ada reset otomatis. Migrasi 021.';
COMMENT ON COLUMN public.atm_visit_quotas.reset_by IS
  'NULL = baris dibuat otomatis oleh kunjungan pertama, bukan reset manual.';

CREATE TRIGGER trg_atm_visit_quotas_set_updated_at BEFORE UPDATE ON public.atm_visit_quotas
    FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();

CREATE TABLE public.atm_visits (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    atm_id bigint NOT NULL REFERENCES public.atms(id),
    vendor_request_id bigint NOT NULL REFERENCES public.vendor_requests(id),
    quota_known boolean NOT NULL,
    is_over_quota boolean NOT NULL DEFAULT false,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    cancelled_at timestamp with time zone,
    cancelled_by bigint REFERENCES public.users(id),
    cancel_reason text,
    CONSTRAINT atm_visits_req_atm_uq UNIQUE (vendor_request_id, atm_id),
    CONSTRAINT atm_visits_cancel_chk CHECK (
        (cancelled_at IS NULL) = (cancelled_by IS NULL)
        AND (cancelled_at IS NULL) = (cancel_reason IS NULL))
);

CREATE INDEX atm_visits_atm_created_idx ON public.atm_visits (atm_id, created_at DESC);

COMMENT ON TABLE public.atm_visits IS
  'Satu kunjungan replenish berhasil per (vendor_request, ATM), dibuat saat SPV approve laporan selesai. UNIQUE mencegah pengurangan ganda. Soft-cancel saja, tidak pernah dihapus. Migrasi 021.';

COMMIT;
