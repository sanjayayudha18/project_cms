-- 047_role_portal.sql
-- Revamp Manajemen Peran: portal peran + cakupan pihak + katalog Portal_Cabang (.kiro/specs/role-management-revamp/,
-- requirements accepted PO 2026-11-25, design + tasks accepted engineer 2026-11-25; task 2.1).
-- Req 1.1, 1.2, 1.3, 2.1, 2.6, 3.1, 3.3, 7.2.
--
--   1. Guard: berhenti bila VENDOR-USER, VENDOR-ATM-USER, VENDOR-VAULT-USER, EXT-CLIENT, atau user ADMIN
--      (granted_by) tidak ada (pola 046, gagal keras).
--   2. Teks rusak seed 002 (Req 1.1, 1.2), hanya baris yang cocok (idempoten):
--      * roles.description: 'â€”' (mojibake satu kali dari em dash) -> '—' (U+2014), 9 baris (seed 002 id 1-9).
--      * locations.name + locations.address_line1, id 911, 1450, 1692 (seed 002 baris 961, 1500, 1742):
--        'Ã¢â‚¬â€œ' (mojibake dua kali dari en dash) -> '–' (U+2013, karakter aslinya). Pola ini berbeda dari
--        'â€”' yang ditulis di design; ditemukan saat task 2.1.
--      File 002 tidak diubah (Req 1.3).
--   3. roles.portal text NOT NULL DEFAULT 'main' IN (main, vendor, service); backfill VENDOR-% -> vendor,
--      EXT-CLIENT -> service (Req 2.1, 2.6). roles_portal_prefix_chk: portal='vendor' <=> role LIKE 'VENDOR-%' (DQ1).
--   4. roles.party_scope text[] NULL, subset {replenish, vault}, wajib tidak kosong untuk vendor dan NULL selain vendor;
--      backfill sama dengan vendorrole.PartyRoles (Req 7.2).
--   5. menu_features.portal text NOT NULL DEFAULT 'main' IN (main, vendor) (Req 3.1).
--   6. Katalog vendor: menu vendor-dsr, vendor-orders, vendor-realizations (Req 3.1). Notifikasi tidak masuk katalog
--      (DQ2, Req 3.2).
--   7. Grant awal D6 = akses sekarang (Req 3.3): VENDOR-USER + VENDOR-ATM-USER -> ketiga key; VENDOR-VAULT-USER ->
--      vendor-dsr + vendor-orders (tanpa Realisasi). granted_by = user ADMIN pertama.
--   8. Post-check: 3 key vendor ada, tidak ada peran vendor tanpa scope, tidak ada teks rusak tersisa.
--
-- SAFETY (Sec 4 #7: migrasi skema + data RBAC):
--   * Satu tx. roles dan menu_features kecil; locations ribuan baris tapi hanya 3 baris di-UPDATE (filter LIKE).
--     ADD COLUMN dengan default konstan = tanpa rewrite, ACCESS EXCLUSIVE sebentar. Skema + data satu file (pola 046;
--     backfill harus selesai sebelum CHECK dipasang).
--   * Idempoten untuk data: UPDATE teks hanya baris yang memuat pola rusak; INSERT ... ON CONFLICT DO NOTHING.
--   * Binary lama tidak membaca kolom baru (semua punya default atau nullable), jadi tetap jalan sesudah 047.
--     INSERT roles dari binary lama tanpa portal = 'main': peran 'VENDOR-*' baru dari binary lama ditolak
--     roles_portal_prefix_chk (aman, daftar tetap lama tidak pernah bertemu peran vendor baru).
--
-- ROLLBACK (forward-only shop; dev script = backend/scripts/dev/rollback_047_role_portal.sql, task 2.2):
--   BEGIN;
--   DELETE FROM public.role_permissions WHERE menu_feature_id IN (SELECT id FROM public.menu_features
--     WHERE key IN ('vendor-dsr', 'vendor-orders', 'vendor-realizations'));
--   DELETE FROM public.menu_features WHERE key IN ('vendor-dsr', 'vendor-orders', 'vendor-realizations');
--   ALTER TABLE public.menu_features DROP COLUMN portal;   -- ikut menghapus menu_features_portal_chk
--   ALTER TABLE public.roles DROP COLUMN party_scope;      -- ikut menghapus roles_party_scope_*_chk
--   ALTER TABLE public.roles DROP COLUMN portal;           -- ikut menghapus roles_portal_chk + roles_portal_prefix_chk
--   COMMIT;
--   Perbaikan teks (langkah 2) tidak dibalik.
BEGIN;

-- 1. Guard ------------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  missing text;
BEGIN
  SELECT string_agg(r, ', ' ORDER BY r) INTO missing
  FROM unnest(ARRAY['VENDOR-USER', 'VENDOR-ATM-USER', 'VENDOR-VAULT-USER', 'EXT-CLIENT']) AS r
  WHERE NOT EXISTS (SELECT 1 FROM public.roles ro WHERE ro.role = r);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION '047: role tidak ada: %', missing;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM public.users u JOIN public.roles ar ON ar.id = u.role_id WHERE ar.role = 'ADMIN') THEN
    RAISE EXCEPTION '047: tidak ada user ADMIN untuk role_permissions.granted_by';
  END IF;
END
$$;

-- 2. Teks rusak (Req 1.1, 1.2) ----------------------------------------------------------------------------------------
UPDATE public.roles
SET description = replace(description, 'â€”', '—'), updated_at = now()
WHERE description LIKE '%â€”%';

UPDATE public.locations
SET name          = replace(name, 'Ã¢â‚¬â€œ', '–'),
    address_line1 = replace(address_line1, 'Ã¢â‚¬â€œ', '–'),
    updated_at    = now()
WHERE name LIKE '%Ã¢â‚¬â€œ%' OR address_line1 LIKE '%Ã¢â‚¬â€œ%';

-- 3. roles.portal (Req 2.1, 2.6, DQ1) -------------------------------------------------------------------------------------
ALTER TABLE public.roles ADD COLUMN portal text NOT NULL DEFAULT 'main';

UPDATE public.roles SET portal = 'vendor'  WHERE role LIKE 'VENDOR-%' AND portal <> 'vendor';
UPDATE public.roles SET portal = 'service' WHERE role = 'EXT-CLIENT'  AND portal <> 'service';

ALTER TABLE public.roles
    ADD CONSTRAINT roles_portal_chk CHECK (portal IN ('main', 'vendor', 'service')),
    ADD CONSTRAINT roles_portal_prefix_chk CHECK ((portal = 'vendor') = (role LIKE 'VENDOR-%'));

COMMENT ON COLUMN public.roles.portal IS
    'Portal pemilik peran: main = CompanyPortal (tab Portal Utama), vendor = VendorPortal (tab Portal Cabang), service = akun layanan (EXT-CLIENT, tidak tampil di tab mana pun). Dikunci ke prefix nama oleh roles_portal_prefix_chk (vendor <=> VENDOR-%), jadi cek prefix di kode tetap benar. Tidak bisa diubah setelah peran dibuat. Migrasi 047.';

-- 4. roles.party_scope (Req 7.2) ----------------------------------------------------------------------------------------
ALTER TABLE public.roles ADD COLUMN party_scope text[];

UPDATE public.roles r
SET party_scope = v.scope
FROM (VALUES
    ('VENDOR-USER',       ARRAY['replenish', 'vault']),
    ('VENDOR-ATM-USER',   ARRAY['replenish']),
    ('VENDOR-VAULT-USER', ARRAY['vault'])
) AS v(role, scope)
WHERE r.role = v.role
  AND r.party_scope IS DISTINCT FROM v.scope;

-- Peran VENDOR-* lain (mis. dibuat manual di dev) tidak punya scope: ADD CONSTRAINT di bawah gagal keras.
ALTER TABLE public.roles
    ADD CONSTRAINT roles_party_scope_values_chk CHECK (party_scope <@ ARRAY['replenish', 'vault']),
    ADD CONSTRAINT roles_party_scope_portal_chk
        CHECK ((portal = 'vendor') = (party_scope IS NOT NULL AND cardinality(party_scope) > 0));

COMMENT ON COLUMN public.roles.party_scope IS
    'Cakupan pihak peran vendor: subset tidak kosong dari {replenish, vault} (replenish = pihak replenish Vendor Request, vault = pihak vault). Wajib untuk portal vendor, NULL untuk portal lain (roles_party_scope_portal_chk). Menggantikan daftar tetap vendorrole.PartyRoles/ForParty. Tidak bisa diubah setelah peran dibuat. Migrasi 047.';

-- 5. menu_features.portal (Req 3.1) -------------------------------------------------------------------------------------
ALTER TABLE public.menu_features ADD COLUMN portal text NOT NULL DEFAULT 'main';

ALTER TABLE public.menu_features
    ADD CONSTRAINT menu_features_portal_chk CHECK (portal IN ('main', 'vendor'));

COMMENT ON COLUMN public.menu_features.portal IS
    'Portal pemilik key katalog: main = CompanyPortal, vendor = VendorPortal (Portal_Cabang). Izin hanya boleh diberikan ke peran dengan roles.portal yang sama (422 role_not_assignable); assignable_roles hanya berlaku untuk main. Migrasi 047.';

-- 6. Katalog vendor (Req 3.1) -------------------------------------------------------------------------------------------
INSERT INTO public.menu_features (parent_id, key, label, kind, sort_order, portal) VALUES
    (NULL, 'vendor-dsr',          'DSR Menu',   'menu', 0, 'vendor'),
    (NULL, 'vendor-orders',       'CIT Orders', 'menu', 1, 'vendor'),
    (NULL, 'vendor-realizations', 'Realisasi',  'menu', 2, 'vendor')
ON CONFLICT (key) DO NOTHING;

-- 7. Grant awal D6 (Req 3.3) ------------------------------------------------------------------------------------------
INSERT INTO public.role_permissions (role_id, menu_feature_id, granted_by)
SELECT ro.id, mf.id,
       (SELECT u.id FROM public.users u JOIN public.roles ar ON ar.id = u.role_id
        WHERE ar.role = 'ADMIN' ORDER BY u.id LIMIT 1)
FROM (VALUES
    ('VENDOR-USER',       'vendor-dsr'),
    ('VENDOR-USER',       'vendor-orders'),
    ('VENDOR-USER',       'vendor-realizations'),
    ('VENDOR-ATM-USER',   'vendor-dsr'),
    ('VENDOR-ATM-USER',   'vendor-orders'),
    ('VENDOR-ATM-USER',   'vendor-realizations'),
    ('VENDOR-VAULT-USER', 'vendor-dsr'),
    ('VENDOR-VAULT-USER', 'vendor-orders')
) AS g(role, key)
JOIN public.roles ro ON ro.role = g.role
JOIN public.menu_features mf ON mf.key = g.key
ON CONFLICT (role_id, menu_feature_id) DO NOTHING;

-- 8. Post-check -----------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  n int;
BEGIN
  SELECT count(*) INTO n FROM public.menu_features
  WHERE key IN ('vendor-dsr', 'vendor-orders', 'vendor-realizations') AND portal = 'vendor';
  IF n <> 3 THEN
    RAISE EXCEPTION '047: diharapkan 3 key katalog vendor, ditemukan %', n;
  END IF;

  SELECT count(*) INTO n FROM public.roles
  WHERE portal = 'vendor' AND (party_scope IS NULL OR cardinality(party_scope) = 0);
  IF n <> 0 THEN
    RAISE EXCEPTION '047: % peran vendor tanpa party_scope', n;
  END IF;

  SELECT count(*) INTO n FROM public.roles WHERE description LIKE '%â€”%';
  IF n <> 0 THEN
    RAISE EXCEPTION '047: % roles.description masih memuat teks rusak', n;
  END IF;

  SELECT count(*) INTO n FROM public.locations
  WHERE name LIKE '%Ã¢â‚¬â€œ%' OR address_line1 LIKE '%Ã¢â‚¬â€œ%';
  IF n <> 0 THEN
    RAISE EXCEPTION '047: % locations masih memuat teks rusak', n;
  END IF;
END
$$;

COMMIT;
