-- 022_vendor_pks.sql
-- PKS vendor + Direktur Utama sebagai PIC (.kiro/specs/vendor-pks/, disetujui
-- PO 2026-11-05; D1-D3, D8, D9, D14, D15, D20-D22, D24, D29, D32).
--
--   * vendor_pics.is_director: penanda Direktur Utama (hanya pencatatan: tanpa
--     login, tanpa roles, bukan penerima notifikasi). Hanya PIC tingkat vendor,
--     maksimal satu direktur aktif per vendor. Constraint roles direktur
--     (vendor_pics.roles dari 032) ada di 037, karena pada DB baru 022 jalan
--     sebelum 032.
--   * vendor_pks: PKS per vendor dengan riwayat; satu PKS aktif per vendor
--     (replaced_at IS NULL). Tanpa parent_pks_id (D3). Tidak pernah di-hard-delete.
--   * vendor_pks_status() + vendor_pks_validity_months(): aturan status dan masa
--     berlaku yang dipakai list, filter, detail, dan ekspor (Req 6, 7).
--   * master_data_change_requests_vendor_pks_pending_uq: satu change vendor_pks
--     pending per vendor (D24, Req 3.6).
--   * Tabel vendors TIDAK berubah (D32).
--
-- SAFETY:
--   * Additive. is_director NOT NULL DEFAULT false = tanpa rewrite (PG 11+);
--     semua baris lama false, jadi vendor_pics_director_chk dan _uq valid tanpa
--     backfill. Tabel vendor_pks baru (kosong) dan belum ada change vendor_pks,
--     jadi index dibuat inline. Tanpa data seed.
--   * Change vendor_pic yang pending saat ini akan stale saat di-approve
--     (snapshot mendapat key is_director) -- hitung sebelum apply (task 1.2).
--   * Forward-only: no down migration (project convention).
BEGIN;

-- ---------------------------------------------------------------------------
-- vendor_pics: penanda Direktur Utama
-- ---------------------------------------------------------------------------
ALTER TABLE public.vendor_pics
    ADD COLUMN is_director boolean NOT NULL DEFAULT false;

ALTER TABLE public.vendor_pics
    ADD CONSTRAINT vendor_pics_director_chk CHECK (
        NOT is_director OR (vendor_branch_id IS NULL AND is_notification_recipient = false)
    );

CREATE UNIQUE INDEX vendor_pics_director_uq
    ON public.vendor_pics (vendor_id)
    WHERE is_director AND deleted_at IS NULL;

COMMENT ON COLUMN public.vendor_pics.is_director IS
    'Direktur Utama vendor (D32). Hanya pencatatan: tanpa login, tanpa roles (037), bukan penerima notifikasi, selalu tingkat vendor (vendor_branch_id NULL). Maksimal satu direktur aktif per vendor (vendor_pics_director_uq).';

-- ---------------------------------------------------------------------------
-- vendor_pks
-- ---------------------------------------------------------------------------
CREATE TABLE public.vendor_pks (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    vendor_id          bigint NOT NULL REFERENCES public.vendors(id) ON DELETE RESTRICT,
    pks_name           text   NOT NULL,
    effective_date     date   NOT NULL,
    expiry_date        date   NOT NULL,
    created_by         bigint NOT NULL REFERENCES public.users(id),
    replaced_at        timestamptz,
    replaced_by_pks_id bigint REFERENCES public.vendor_pks(id),
    terminated_at      timestamptz,
    terminated_by      bigint REFERENCES public.users(id),
    termination_reason text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT vendor_pks_name_chk CHECK (char_length(btrim(pks_name)) BETWEEN 1 AND 500),
    CONSTRAINT vendor_pks_dates_chk CHECK (expiry_date >= effective_date),
    CONSTRAINT vendor_pks_replaced_chk CHECK (replaced_by_pks_id IS NULL OR replaced_at IS NOT NULL),
    CONSTRAINT vendor_pks_terminate_chk CHECK (
        (terminated_at IS NULL AND terminated_by IS NULL AND termination_reason IS NULL)
     OR (terminated_at IS NOT NULL AND terminated_by IS NOT NULL
         AND char_length(btrim(termination_reason)) BETWEEN 1 AND 500)
    )
);

CREATE UNIQUE INDEX vendor_pks_active_uq
    ON public.vendor_pks (vendor_id)
    WHERE replaced_at IS NULL;

CREATE INDEX vendor_pks_vendor_idx
    ON public.vendor_pks (vendor_id, replaced_at DESC);

CREATE TRIGGER trg_vendor_pks_set_updated_at
    BEFORE UPDATE ON public.vendor_pks
    FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*))
    EXECUTE FUNCTION public.set_updated_at();

COMMENT ON TABLE public.vendor_pks IS
    'PKS (perjanjian kerja sama) per vendor dengan riwayat (D1). Satu PKS aktif per vendor (replaced_at IS NULL, vendor_pks_active_uq). Ditulis hanya lewat maker-checker master data (entity_type vendor_pks). pks_name/effective_date/expiry_date tidak pernah di-UPDATE; tidak pernah di-hard-delete. Riwayat batalkan terminate hanya di audit_logs.';
COMMENT ON COLUMN public.vendor_pks.replaced_at IS
    'NULL = PKS aktif vendor. Diisi saat PKS baru disetujui dan menggantikannya (D17).';
COMMENT ON COLUMN public.vendor_pks.replaced_by_pks_id IS
    'PKS yang menggantikan baris ini; hanya terisi bila replaced_at terisi.';
COMMENT ON COLUMN public.vendor_pks.terminated_at IS
    'Diisi saat terminate disetujui; dikosongkan lagi (bersama terminated_by + termination_reason) saat batalkan terminate disetujui.';
COMMENT ON COLUMN public.vendor_pks.created_by IS
    'Maker change create (users.id).';

-- ---------------------------------------------------------------------------
-- Fungsi status + masa berlaku (Req 6, 7)
-- ---------------------------------------------------------------------------
-- Label PKS satu vendor. NULL untuk vendor INTERNAL; 'belum_diisi' bila tidak
-- ada PKS aktif (p_expiry NULL). Prioritas: terminated > expired > need_renewal
-- > aktif (D22). Hari expired masih berlaku; expired mulai hari berikutnya (D20).
-- date + N bulan di-clamp ke akhir bulan (2026-05-31 + 9 bulan = 2027-02-28).
CREATE FUNCTION public.vendor_pks_status(
    p_kind text,
    p_expiry date,
    p_terminated_at timestamptz,
    p_today date,
    p_renewal_months int
) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN p_kind = 'INTERNAL' THEN NULL
    WHEN p_expiry IS NULL THEN 'belum_diisi'
    WHEN p_terminated_at IS NOT NULL THEN 'terminated'
    WHEN p_expiry < p_today THEN 'expired'
    WHEN p_expiry < (p_today + make_interval(months => p_renewal_months))::date THEN 'need_renewal'
    ELSE 'aktif'
  END
$$;

COMMENT ON FUNCTION public.vendor_pks_status(text, date, timestamptz, date, int) IS
    'Label PKS: NULL (INTERNAL), belum_diisi, terminated, expired, need_renewal, aktif. p_today = tanggal Asia/Jakarta dari server (D30); p_renewal_months = VENDOR_PKS_RENEWAL_MONTHS.';

-- Bulan kalender penuh dari p_effective sampai p_expiry + 1 hari (D21): n
-- terbesar dengan (p_effective + n bulan, di-clamp ke akhir bulan) <= p_expiry + 1.
-- Kandidat dari age(), lalu dikoreksi karena age() dan penambahan interval tidak
-- selalu sepakat di akhir bulan. NULL bila salah satu argumen NULL; 0 bila < 1 bulan.
CREATE FUNCTION public.vendor_pks_validity_months(p_effective date, p_expiry date)
RETURNS int
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
  v_end date;
  v_age interval;
  n     int;
BEGIN
  IF p_effective IS NULL OR p_expiry IS NULL THEN
    RETURN NULL;
  END IF;
  v_end := p_expiry + 1;
  v_age := age(v_end::timestamp, p_effective::timestamp);
  n := (extract(year FROM v_age) * 12 + extract(month FROM v_age))::int;
  WHILE n > 0 AND (p_effective + make_interval(months => n))::date > v_end LOOP
    n := n - 1;
  END LOOP;
  WHILE (p_effective + make_interval(months => n + 1))::date <= v_end LOOP
    n := n + 1;
  END LOOP;
  RETURN GREATEST(n, 0);
END
$$;

COMMENT ON FUNCTION public.vendor_pks_validity_months(date, date) IS
    'Masa berlaku PKS dalam bulan kalender penuh (sisa hari dibulatkan ke bawah, D21). 2026-01-01 -> 2027-12-31 = 24.';

-- ---------------------------------------------------------------------------
-- Satu change vendor_pks pending per vendor (D24, Req 3.6)
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX master_data_change_requests_vendor_pks_pending_uq
    ON public.master_data_change_requests (((payload->>'vendor_id')::bigint))
    WHERE entity_type = 'vendor_pks' AND status = 'pending';

COMMIT;

-- ROLLBACK (documented, forward-only shop; aman hanya selama vendor_pks kosong
-- dan belum ada direktur yang dicatat):
--   BEGIN;
--   DROP INDEX IF EXISTS public.master_data_change_requests_vendor_pks_pending_uq;
--   DROP FUNCTION IF EXISTS public.vendor_pks_validity_months(date, date);
--   DROP FUNCTION IF EXISTS public.vendor_pks_status(text, date, timestamptz, date, int);
--   DROP TABLE IF EXISTS public.vendor_pks;
--   DROP INDEX IF EXISTS public.vendor_pics_director_uq;
--   ALTER TABLE public.vendor_pics DROP CONSTRAINT IF EXISTS vendor_pics_director_chk;
--   ALTER TABLE public.vendor_pics DROP COLUMN IF EXISTS is_director;
--   COMMIT;
