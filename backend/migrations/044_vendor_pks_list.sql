-- 044_vendor_pks_list.sql
-- Daftar PKS + reminder per PKS (.kiro/specs/vendor-pks-list/, requirements accepted PO 2026-11-19,
-- design accepted engineer 2026-11-19; task 1.1). Req 1.1, 3.1, 3.2, 3.4, 9.1, 11.3.
--
--   1. Guard: berhenti bila ada vendor_pks dengan masa (expiry - effective) < 30 hari; baris itu tidak bisa
--      memenuhi CHECK minimum renewal_days.
--   2. vendor_pks.renewal_days: ambang Need Renewal per PKS dalam hari. NOT NULL DEFAULT 270, backfill
--      LEAST(270, masa PKS) (D8), CHECK 30-3666 dan CHECK <= masa PKS (D7).
--   3. vendor_pks_record_status(): aturan status per baris PKS (D4, D5, D6):
--      diganti > terminated > belum_berlaku > expired > need_renewal > aktif; expiry = today belum expired.
--   4. Menu settings.pks "Daftar PKS" di bawah Pengaturan; role_permissions ADMIN, ADMIN_PARAM -> settings.pks;
--      ATM-SPV -> settings + settings.pks (D24).
--   5. COMMENT tabel/kolom: renewal_days satu-satunya kolom isi yang boleh di-UPDATE (Req 11.3).
--   Tidak di-drop: vendor_pks_status() dan vendor_pks_validity_months() -- binary lama masih memanggilnya
--   selama jendela deploy; drop di migrasi pembersih 045 setelah rilis (design, Req 3.6).
--
-- SAFETY (Sec 4 #7: migrasi skema + data + RBAC):
--   * Satu tx. vendor_pks kecil (dev: 0 baris), jadi ADD COLUMN + backfill + ADD CONSTRAINT (validasi penuh)
--     inline aman; DEFAULT 270 konstan = tanpa table rewrite.
--   * DEFAULT 270 dipertahankan agar binary lama (tanpa renewal_days) tetap bisa INSERT antara migrasi dan
--     deploy; API baru selalu mengirim nilai eksplisit (Req 1.2). Batas: binary lama yang menyetujui PKS
--     dengan masa < 270 hari di jendela itu akan ditolak CHECK -- jendela deploy di luar jam kantor.
--   * Menu/izin: ON CONFLICT DO NOTHING; granted_by = user ADMIN pertama (pola 027).
--   * Forward-only: no down migration (project convention). Rollback dev =
--     backend/scripts/dev/rollback_044_vendor_pks_list.sql (task 1.2).
BEGIN;

-- 1. Guard ------------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  bad_ids text;
BEGIN
  SELECT string_agg(id::text, ', ' ORDER BY id) INTO bad_ids
  FROM public.vendor_pks
  WHERE expiry_date - effective_date < 30;
  IF bad_ids IS NOT NULL THEN
    RAISE EXCEPTION '044: vendor_pks dengan masa < 30 hari tidak bisa memenuhi CHECK renewal_days (id: %)', bad_ids;
  END IF;
END
$$;

-- 2. renewal_days (Req 1.1) ----------------------------------------------------------------------------------------------
ALTER TABLE public.vendor_pks ADD COLUMN renewal_days integer NOT NULL DEFAULT 270;

-- D8; LEAST hanya berbeda dari 270 untuk PKS yang masanya < 270 hari.
UPDATE public.vendor_pks
SET renewal_days = LEAST(270, expiry_date - effective_date)
WHERE renewal_days IS DISTINCT FROM LEAST(270, expiry_date - effective_date);

ALTER TABLE public.vendor_pks
    ADD CONSTRAINT vendor_pks_renewal_days_chk CHECK (renewal_days BETWEEN 30 AND 3666),
    ADD CONSTRAINT vendor_pks_renewal_days_le_term_chk CHECK (renewal_days <= expiry_date - effective_date);

-- 3. Fungsi status per baris PKS (Req 3.1, 3.2, 3.4) ---------------------------------------------------------------------
CREATE FUNCTION public.vendor_pks_record_status(
    p_replaced_at   timestamptz,
    p_terminated_at timestamptz,
    p_effective     date,
    p_expiry        date,
    p_renewal_days  int,
    p_today         date
) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN p_replaced_at   IS NOT NULL                THEN 'diganti'
    WHEN p_terminated_at IS NOT NULL                THEN 'terminated'
    WHEN p_effective     > p_today                  THEN 'belum_berlaku'
    WHEN p_expiry        < p_today                  THEN 'expired'
    WHEN p_expiry        < p_today + p_renewal_days THEN 'need_renewal'
    ELSE 'aktif'
  END
$$;

COMMENT ON FUNCTION public.vendor_pks_record_status(timestamptz, timestamptz, date, date, int, date) IS
    'Label satu baris vendor_pks: diganti > terminated > belum_berlaku > expired > need_renewal > aktif (D4-D6). Hari expiry_date masih berlaku; expired mulai hari berikutnya. need_renewal bila expiry < today + renewal_days hari. p_today = tanggal Asia/Jakarta dari server (Go), bukan klien. Menggantikan vendor_pks_status() (dihapus di 045). Migrasi 044.';

-- 4. Menu + izin (Req 9.1) -----------------------------------------------------------------------------------------------
INSERT INTO public.menu_features (parent_id, key, label, kind, sort_order)
SELECT id, 'settings.pks', 'Daftar PKS', 'feature', 2 FROM public.menu_features WHERE key = 'settings'
ON CONFLICT (key) DO NOTHING;

INSERT INTO public.role_permissions (role_id, menu_feature_id, granted_by)
SELECT r.id, mf.id, (SELECT u.id FROM public.users u JOIN public.roles ar ON ar.id = u.role_id
                     WHERE ar.role = 'ADMIN' ORDER BY u.id LIMIT 1)
FROM public.roles r
JOIN public.menu_features mf ON mf.key = ANY (CASE r.role
        WHEN 'ADMIN'       THEN ARRAY['settings.pks']
        WHEN 'ADMIN_PARAM' THEN ARRAY['settings.pks']
        WHEN 'ATM-SPV'     THEN ARRAY['settings', 'settings.pks']
    END)
WHERE r.role IN ('ADMIN', 'ADMIN_PARAM', 'ATM-SPV')
ON CONFLICT (role_id, menu_feature_id) DO NOTHING;

-- 5. Comment (Req 11.3) --------------------------------------------------------------------------------------------------
COMMENT ON TABLE public.vendor_pks IS
    'PKS (perjanjian kerja sama) per vendor per type dengan riwayat (D1; 043). Satu PKS aktif per (vendor, type) (replaced_at IS NULL, vendor_pks_active_uq); vendor boleh punya subset type. Create/ganti/terminasi hanya lewat maker-checker master data (entity_type vendor_pks). Kolom yang boleh di-UPDATE hanya renewal_days (044, langsung + audit di tx yang sama), replaced_at/replaced_by_pks_id, dan kolom terminate; pks_name/effective_date/expiry_date/pks_type_id/wod_number tidak pernah di-UPDATE; tidak pernah di-hard-delete. Status dihitung oleh vendor_pks_record_status() (044). Riwayat batalkan terminate hanya di audit_logs.';
COMMENT ON COLUMN public.vendor_pks.renewal_days IS
    'Ambang Need Renewal dalam hari: need_renewal bila expiry_date < hari ini + renewal_days. 30-3666 dan <= expiry_date - effective_date. Diisi saat create (maker-checker); diubah langsung oleh ADMIN/ADMIN_PARAM dari Daftar PKS dengan audit vendor_pks_reminder_updated di tx yang sama, juga untuk PKS diganti/terminated. Satu-satunya kolom isi PKS yang boleh di-UPDATE. Default 270 hanya untuk binary lama selama jendela deploy. Migrasi 044.';

COMMIT;
