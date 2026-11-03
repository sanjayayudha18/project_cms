-- 032_vendor_party_scope.sql
-- Cakupan akun vendor per pihak + peran PIC (.kiro/specs/vendor-party-scope, design "Data Models").
--
--   * roles: VENDOR-ATM-USER (pihak replenish saja), VENDOR-VAULT-USER (pihak vault saja);
--     role_permissions disalin dari VENDOR-USER (Req 1.1, 1.2).
--   * users.vendor_branch_id harus cabang milik users.vendor_id (Req 2.5): FK komposit (pola 030).
--   * vendor_pics.roles: peran PIC replenish/vault/cashcount, baris lama = ketiganya (Req 5.1, 5.2).
--
-- SAFETY (Sec 4 #7, auth/RBAC): additive. Pre-check gagal keras bila ada user dengan cabang vendor lain
-- (dev: 0 baris, dicek 2026-10-20). Satu tx. Forward-only: no down migration (project convention).
BEGIN;

-- 1. Role vendor baru + hak menu = VENDOR-USER
INSERT INTO public.roles (role, description) VALUES
  ('VENDOR-ATM-USER',   'Vendor pengisian ATM: pihak replenish saja (vendor-party-scope)'),
  ('VENDOR-VAULT-USER', 'Vendor penyedia uang di vault: pihak vault saja (vendor-party-scope)')
ON CONFLICT (role) DO NOTHING;

INSERT INTO public.role_permissions (role_id, menu_feature_id, granted_by)
SELECT nr.id, rp.menu_feature_id, rp.granted_by
FROM public.role_permissions rp
JOIN public.roles vu ON vu.id = rp.role_id AND vu.role = 'VENDOR-USER'
CROSS JOIN public.roles nr
WHERE nr.role IN ('VENDOR-ATM-USER', 'VENDOR-VAULT-USER')
ON CONFLICT (role_id, menu_feature_id) DO NOTHING;

-- 2. Cabang akun harus milik vendor akun. Lebih ketat dari FK MATCH SIMPLE: cabang terisi + vendor NULL juga ditolak.
DO $$
DECLARE
  bad_count bigint;
  bad_ids   text;
BEGIN
  SELECT count(*), string_agg(u.id::text, ', ' ORDER BY u.id)
    INTO bad_count, bad_ids
  FROM public.users u
  JOIN public.vendor_branches b ON b.id = u.vendor_branch_id
  WHERE u.vendor_id IS DISTINCT FROM b.vendor_id;

  IF bad_count > 0 THEN
    RAISE EXCEPTION '032: % user dengan vendor_branch_id bukan milik users.vendor_id (users.id: %); perbaiki data sebelum migrasi 032',
      bad_count, left(bad_ids, 500);
  END IF;
END $$;

ALTER TABLE public.vendor_branches
  ADD CONSTRAINT vendor_branches_id_vendor_uq UNIQUE (id, vendor_id);
ALTER TABLE public.users
  ADD CONSTRAINT users_vendor_branch_owner_fk
  FOREIGN KEY (vendor_branch_id, vendor_id) REFERENCES public.vendor_branches (id, vendor_id);

COMMENT ON COLUMN public.users.vendor_branch_id IS 'Optional: pins a vendor user to one vendor_branches row. Must belong to the same vendor as vendor_id (FK users_vendor_branch_owner_fk, migration 032). NULL = vendor-wide or internal/LDAP user.';

-- 3. Peran PIC: DEFAULT mengisi semua baris lama dengan ketiga peran
ALTER TABLE public.vendor_pics
  ADD COLUMN roles text[] NOT NULL DEFAULT ARRAY['replenish', 'vault', 'cashcount']::text[],
  ADD CONSTRAINT vendor_pics_roles_chk CHECK (roles <@ ARRAY['replenish', 'vault', 'cashcount']::text[]);
COMMENT ON COLUMN public.vendor_pics.roles IS
  'Peran PIC: replenish (PIC replenish ATM), vault (PIC penyedia uang), cashcount (PIC cash count). Kosong = kontak saja. Ubah lewat layar = langsung + audit (deviasi GR#3, vendor-party-scope).';

COMMIT;
