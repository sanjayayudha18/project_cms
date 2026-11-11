-- 038_vendor_cis_limit.sql
-- vendor-cis-limit (.claude/sdlc/vendor-cis-limit/, plan P1). Additive, forward-only. Three contents:
--   1. vendor_branches.cis_limit_amount + cis_limit_currency: Limit CIS (= "Limit Asuransi") per area vault vendor
--      (FR1, A1). NULL = belum diisi. Ubah lewat master-data maker-checker (A3).
--   2. vendor_vaults.escrow_account: nomor escrow per vault, 12 digit, opsional, TIDAK unik (FR2, Q7).
--   3. vendor_pics.roles: peran baru 'bm' (Branch Manager area, FR3), hanya untuk PIC yang terikat cabang (S5).
-- Nullable / constant-default columns = no table rewrite. Existing rows: limit NULL, escrow NULL, roles unchanged.
--
-- ROLLBACK (documented, forward-only shop):
--   UPDATE public.vendor_pics SET roles = array_remove(roles, 'bm') WHERE 'bm' = ANY(roles);
--   ALTER TABLE public.vendor_pics DROP CONSTRAINT vendor_pics_bm_branch_chk;
--   ALTER TABLE public.vendor_pics DROP CONSTRAINT vendor_pics_roles_chk;
--   ALTER TABLE public.vendor_pics ADD CONSTRAINT vendor_pics_roles_chk
--     CHECK (roles <@ ARRAY['replenish', 'vault', 'cashcount']::text[]);
--   ALTER TABLE public.vendor_vaults DROP COLUMN escrow_account;
--   ALTER TABLE public.vendor_branches DROP COLUMN cis_limit_currency, DROP COLUMN cis_limit_amount;

BEGIN;

-- 1. Limit CIS per area vault vendor
ALTER TABLE public.vendor_branches
  ADD COLUMN cis_limit_amount numeric(20,2) NULL,
  ADD COLUMN cis_limit_currency character(3) NOT NULL DEFAULT 'IDR',
  ADD CONSTRAINT vendor_branches_cis_limit_amount_chk CHECK (cis_limit_amount > 0);
COMMENT ON COLUMN public.vendor_branches.cis_limit_amount IS
  'Limit CIS (= Limit Asuransi) area vault vendor, dalam cis_limit_currency. NULL = belum diisi. Master-data maker-checker. Migrasi 038.';
COMMENT ON COLUMN public.vendor_branches.cis_limit_currency IS
  'Mata uang cis_limit_amount (ISO 4217). Hanya IDR untuk sekarang (vendor-cis-limit S4). Migrasi 038.';

-- 2. Nomor escrow per vault
ALTER TABLE public.vendor_vaults
  ADD COLUMN escrow_account text NULL,
  ADD CONSTRAINT vendor_vaults_escrow_account_chk CHECK (escrow_account ~ '^[0-9]{12}$');
COMMENT ON COLUMN public.vendor_vaults.escrow_account IS
  'Nomor rekening escrow vault (12 digit, teks). Opsional, tidak unik: satu escrow bisa dikelola beberapa vendor. Master-data maker-checker. Migrasi 038.';

-- 3. Peran PIC bm (Branch Manager), hanya PIC cabang
ALTER TABLE public.vendor_pics DROP CONSTRAINT vendor_pics_roles_chk;
ALTER TABLE public.vendor_pics
  ADD CONSTRAINT vendor_pics_roles_chk CHECK (roles <@ ARRAY['replenish', 'vault', 'cashcount', 'bm']::text[]),
  ADD CONSTRAINT vendor_pics_bm_branch_chk CHECK (NOT ('bm' = ANY(roles)) OR vendor_branch_id IS NOT NULL);
COMMENT ON COLUMN public.vendor_pics.roles IS
  'Peran PIC: replenish (PIC replenish ATM), vault (PIC penyedia uang), cashcount (PIC cash count), bm (Branch Manager area, hanya PIC cabang; migrasi 038). Kosong = kontak saja. Ubah lewat layar = langsung + audit (deviasi GR#3, vendor-party-scope).';

COMMIT;
