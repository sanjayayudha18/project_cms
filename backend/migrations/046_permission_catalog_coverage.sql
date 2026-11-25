-- 046_permission_catalog_coverage.sql
-- Katalog izin lengkap + izin dipakai (.kiro/specs/permission-catalog-coverage/, requirements accepted PO 2026-11-20,
-- design accepted engineer 2026-11-20, tasks accepted 2026-11-24; task 1.1). Req 1.1, 1.2, 1.3, 2.1-2.5, 3.6.
--
--   1. Guard: berhenti bila menu induk, role matriks, atau user ADMIN (granted_by) tidak ada (pola 027/041, gagal keras).
--   2. menu_features.assignable_roles text[] NULL (DQ1): NULL = semua role internal boleh diberi; terisi = hanya role
--      itu (guard role tetap pada endpoint baca layar, tabel mount design). Ditegakkan PUT izin role (422, task 2.4).
--   3. Menu baru approvals (sort 9) + exports (sort 10).
--   4. 19 fitur Requirement 1.1; parent lewat SELECT id ... WHERE key = <parent>.
--   5. assignable_roles untuk key dengan guard baca tetap + Locked_Key (D8).
--   6. Grant awal matriks D9 + parent menu tiap key anak; Always_On_Key (dashboard, approvals) dan Locked_Key
--      (settings.roles*, settings.users) tidak diberi grant. granted_by = user ADMIN pertama.
--   7. Post-check: ke-21 key baru ada.
--   Tidak diubah: key/baris menu_features dan role_permissions yang sudah ada (Req 1.2, 2.5). Menu disabled di NAV_CONFIG
--   tidak didaftarkan (D11).
--
-- SAFETY (Sec 4 #7: migrasi skema + data RBAC):
--   * Satu tx (Req 2.3, 2.4). menu_features kecil (< 50 baris): ADD COLUMN nullable tanpa default = tanpa rewrite,
--     ACCESS EXCLUSIVE sebentar. Skema + data sengaja satu file (pola 027/041/044; tabel katalog kecil).
--   * Additive + idempoten: ON CONFLICT DO NOTHING; UPDATE assignable_roles hanya baris yang berbeda.
--   * Binary lama tidak membaca assignable_roles, jadi tetap jalan sesudah 046.
--
-- ROLLBACK (forward-only shop; dev script = backend/scripts/dev/rollback_046_permission_catalog_coverage.sql, task 1.2):
--   BEGIN;
--   DELETE FROM public.role_permissions WHERE menu_feature_id IN (SELECT id FROM public.menu_features WHERE key IN (
--     'approvals','exports','replenish.forecast-browser','replenish.vendor-requests','replenish.schedule',
--     'monitoring.atm-portal','monitoring.dsr-late-report','monitoring.eod-monitoring','monitoring.audit-logs',
--     'forecasting.dmaa-forecast','forecasting.holidays','settings.users','settings.vendors','settings.atms',
--     'settings.regions','settings.pks-types','settings.master-data-io','settings.rbac-users','settings.rbac-policies',
--     'settings.rbac-delegations','settings.rbac-leaves'));
--   -- grant parent menu (replenish, monitoring, forecasting, settings) dari 046 tidak dipisahkan dari grant lama
--   -- (tanpa penanda); biarkan, atau hapus per role sesuai matriks D9 bila DB belum pernah punya grant itu.
--   DELETE FROM public.menu_features WHERE key IN (<21 key di atas>);
--   ALTER TABLE public.menu_features DROP COLUMN assignable_roles;
--   COMMIT;
BEGIN;

-- 1. Guard ------------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  missing text;
BEGIN
  SELECT string_agg(k, ', ' ORDER BY k) INTO missing
  FROM unnest(ARRAY['replenish', 'monitoring', 'forecasting', 'settings', 'settings.roles']) AS k
  WHERE NOT EXISTS (SELECT 1 FROM public.menu_features mf WHERE mf.key = k);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION '046: menu_features induk tidak ada: %', missing;
  END IF;

  SELECT string_agg(r, ', ' ORDER BY r) INTO missing
  FROM unnest(ARRAY['ADMIN', 'ADMIN_PARAM', 'APPACCESS', 'ATM-USER', 'ATM-SPV', 'BRANCH-ATM-USER', 'BRANCH-ATM-SPV']) AS r
  WHERE NOT EXISTS (SELECT 1 FROM public.roles ro WHERE ro.role = r);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION '046: role matriks D9 tidak ada: %', missing;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM public.users u JOIN public.roles ar ON ar.id = u.role_id WHERE ar.role = 'ADMIN') THEN
    RAISE EXCEPTION '046: tidak ada user ADMIN untuk role_permissions.granted_by';
  END IF;
END
$$;

-- 2. Kolom assignable_roles (DQ1, Req 3.6) ------------------------------------------------------------------------------
ALTER TABLE public.menu_features ADD COLUMN assignable_roles text[];

COMMENT ON COLUMN public.menu_features.assignable_roles IS
    'Role yang boleh diberi izin key ini lewat PUT /api/v1/admin/roles/{id}/permissions. NULL = semua role internal. Terisi untuk key yang endpoint baca layarnya punya guard role tetap (data master, deviasi GR#3, auth) dan untuk Locked_Key; grant di luar daftar ditolak 422 role_not_assignable. Katalog hanya bisa mencabut akses di layar itu, tidak memperluasnya. Migrasi 046.';

-- 3. Menu baru (Req 1.1) -------------------------------------------------------------------------------------------------
INSERT INTO public.menu_features (parent_id, key, label, kind, sort_order) VALUES
    (NULL, 'approvals', 'Persetujuan', 'menu', 9),
    (NULL, 'exports',   'Ekspor',      'menu', 10)
ON CONFLICT (key) DO NOTHING;

-- 4. Fitur baru (Req 1.1) ------------------------------------------------------------------------------------------------
INSERT INTO public.menu_features (parent_id, key, label, kind, sort_order)
SELECT p.id, v.key, v.label, 'feature', v.sort_order
FROM (VALUES
    ('replenish',   'replenish.forecast-browser', 'Rekomendasi CIT',          0),
    ('replenish',   'replenish.vendor-requests',  'Request CIT',              1),
    ('replenish',   'replenish.schedule',         'Jadwal CIT',               2),
    ('monitoring',  'monitoring.atm-portal',      'ATM Portal',               0),
    ('monitoring',  'monitoring.dsr-late-report', 'Laporan DSR Telat',        1),
    ('monitoring',  'monitoring.eod-monitoring',  'EOD Monitoring',           2),
    ('monitoring',  'monitoring.audit-logs',      'Log Audit',                3),
    ('forecasting', 'forecasting.dmaa-forecast',  'DMAA Forecast',            0),
    ('forecasting', 'forecasting.holidays',       'Kalender Libur',           1),
    ('settings',    'settings.users',             'Pengguna',                 3),
    ('settings',    'settings.vendors',           'Vendor',                   4),
    ('settings',    'settings.atms',              'ATM',                      5),
    ('settings',    'settings.regions',           'Region',                   6),
    ('settings',    'settings.pks-types',         'Type PKS',                 7),
    ('settings',    'settings.master-data-io',    'Impor/Ekspor Data Master', 8),
    ('settings',    'settings.rbac-users',        'Pengguna RBAC',            9),
    ('settings',    'settings.rbac-policies',     'Kebijakan RBAC',           10),
    ('settings',    'settings.rbac-delegations',  'Delegasi',                 11),
    ('settings',    'settings.rbac-leaves',       'Cuti',                     12)
) AS v(parent_key, key, label, sort_order)
JOIN public.menu_features p ON p.key = v.parent_key
ON CONFLICT (key) DO NOTHING;

-- 5. assignable_roles (tabel mount design, kolom "Guard role tetap" pada endpoint baca) ----------------------------------
-- Tidak diisi (NULL): key yang endpoint bacanya hanya dijaga RequirePermission (guard aksi tidak menghalangi membuka
-- layar), mis. replenish.vendor-requests, settings.vendors (vendorRead), settings.pks, cit.vault-plan.
UPDATE public.menu_features mf
SET assignable_roles = v.roles
FROM (VALUES
    ('settings.atms',             ARRAY['ADMIN', 'ADMIN_PARAM']),
    ('settings.regions',          ARRAY['ADMIN', 'ADMIN_PARAM']),
    ('settings.pks-types',        ARRAY['ADMIN', 'ADMIN_PARAM']),
    ('settings.master-data-io',   ARRAY['ADMIN', 'ADMIN_PARAM']),
    ('settings.rbac-users',       ARRAY['ADMIN', 'ADMIN_PARAM', 'APPACCESS']),
    ('settings.rbac-policies',    ARRAY['ADMIN', 'ADMIN_PARAM', 'APPACCESS']),
    ('settings.rbac-delegations', ARRAY['ADMIN', 'ADMIN_PARAM', 'APPACCESS']),
    ('settings.rbac-leaves',      ARRAY['ADMIN', 'ADMIN_PARAM', 'APPACCESS']),
    ('settings.acm-areas',        ARRAY['ADMIN']),
    ('forecasting.holidays',      ARRAY['ADMIN']),
    ('cash-count.parameters',     ARRAY['ADMIN', 'CC-CWM', 'CC-CCR']),
    ('cash-count.pic-mapping',    ARRAY['ADMIN', 'CC-CWM']),
    -- Locked_Key (D8): guard statis APPACCESS/ADMIN; editor menampilkannya terkunci.
    ('settings.roles',            ARRAY['APPACCESS', 'ADMIN']),
    ('settings.roles.create',     ARRAY['APPACCESS', 'ADMIN']),
    ('settings.roles.edit',       ARRAY['APPACCESS', 'ADMIN']),
    ('settings.users',            ARRAY['APPACCESS', 'ADMIN'])
) AS v(key, roles)
WHERE mf.key = v.key
  AND mf.assignable_roles IS DISTINCT FROM v.roles;

-- 6. Grant awal matriks D9 (Req 2.1, 2.2, 2.5) ---------------------------------------------------------------------------
-- Parent menu ikut diberi grant bila role mendapat >= 1 anaknya (tampilan editor konsisten; penegakan memakai key layar).
WITH grants (role, key) AS (
    SELECT r.role, k
    FROM (VALUES
        ('ADMIN', ARRAY['cash-flow', 'exports',
                        'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
                        'monitoring.atm-portal', 'monitoring.dsr-late-report', 'monitoring.eod-monitoring',
                        'monitoring.audit-logs',
                        'forecasting.dmaa-forecast', 'forecasting.holidays',
                        'settings.vendors', 'settings.atms', 'settings.regions', 'settings.pks-types',
                        'settings.master-data-io',
                        'settings.rbac-users', 'settings.rbac-policies', 'settings.rbac-delegations',
                        'settings.rbac-leaves']),
        ('ADMIN_PARAM', ARRAY['cash-flow',
                        'monitoring.atm-portal', 'monitoring.audit-logs',
                        'forecasting.dmaa-forecast',
                        'settings.vendors', 'settings.atms', 'settings.regions', 'settings.pks-types',
                        'settings.master-data-io',
                        'settings.rbac-users', 'settings.rbac-policies', 'settings.rbac-delegations',
                        'settings.rbac-leaves']),
        ('APPACCESS', ARRAY['monitoring.eod-monitoring',
                        'settings.rbac-users', 'settings.rbac-policies', 'settings.rbac-delegations',
                        'settings.rbac-leaves']),
        ('ATM-USER', ARRAY['cash-flow', 'exports',
                        'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
                        'monitoring.atm-portal', 'monitoring.dsr-late-report',
                        'forecasting.dmaa-forecast']),
        ('ATM-SPV', ARRAY['cash-flow', 'exports',
                        'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
                        'monitoring.atm-portal', 'monitoring.dsr-late-report',
                        'forecasting.dmaa-forecast',
                        'settings.vendors']),
        ('BRANCH-ATM-USER', ARRAY['exports',
                        'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
                        'forecasting.dmaa-forecast']),
        ('BRANCH-ATM-SPV', ARRAY['exports',
                        'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
                        'monitoring.dsr-late-report',
                        'forecasting.dmaa-forecast'])
    ) AS r(role, keys)
    CROSS JOIN LATERAL unnest(r.keys) AS k
),
with_parents (role, menu_feature_id) AS (
    SELECT g.role, mf.id FROM grants g JOIN public.menu_features mf ON mf.key = g.key
    UNION
    SELECT g.role, mf.parent_id FROM grants g JOIN public.menu_features mf ON mf.key = g.key
    WHERE mf.parent_id IS NOT NULL
)
INSERT INTO public.role_permissions (role_id, menu_feature_id, granted_by)
SELECT ro.id, wp.menu_feature_id,
       (SELECT u.id FROM public.users u JOIN public.roles ar ON ar.id = u.role_id
        WHERE ar.role = 'ADMIN' ORDER BY u.id LIMIT 1)
FROM with_parents wp
JOIN public.roles ro ON ro.role = wp.role
ON CONFLICT (role_id, menu_feature_id) DO NOTHING;

-- 7. Post-check ------------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  n int;
BEGIN
  SELECT count(*) INTO n FROM public.menu_features WHERE key IN (
    'approvals', 'exports', 'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
    'monitoring.atm-portal', 'monitoring.dsr-late-report', 'monitoring.eod-monitoring', 'monitoring.audit-logs',
    'forecasting.dmaa-forecast', 'forecasting.holidays', 'settings.users', 'settings.vendors', 'settings.atms',
    'settings.regions', 'settings.pks-types', 'settings.master-data-io', 'settings.rbac-users',
    'settings.rbac-policies', 'settings.rbac-delegations', 'settings.rbac-leaves');
  IF n <> 21 THEN
    RAISE EXCEPTION '046: diharapkan 21 key katalog baru, ditemukan %', n;
  END IF;
END
$$;

COMMIT;
