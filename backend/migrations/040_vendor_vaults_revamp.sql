-- 040_vendor_vaults_revamp.sql
-- Restrukturisasi skema vendor_vaults & vendor_branches untuk penyesuaian
-- Export Data, CSV Import, dan Form Admin UI (.agents/specs/vendor-vaults-export-revamp/).
--
-- Perubahan utama (forward-only, satu transaksi):
--   1. vendor_vaults: rename min_capacity_amount -> min_balance dan
--      max_capacity_amount -> max_balance (tetap numeric(20,2), tanpa float).
--   2. vendor_vaults: tambah vault_name (text NOT NULL setelah backfill deterministik
--      <vendorPrefix>_<area>_V<3-digit>, contoh: ABA_Bali_V001) dan alamat (text NULL),
--      beserta trigger BEFORE INSERT agar pembuatan baris tanpa vault_name eksplisit
--      otomatis terisi sesuai urutan cabang.
--   3. Pindahkan cis_limit_amount (numeric(20,2) NULL CHECK > 0) dan
--      cis_limit_currency (character(3) NOT NULL DEFAULT 'IDR') dari vendor_branches
--      ke vendor_vaults, lalu hapus keduanya dari vendor_branches.
--   4. Ganti vendor_vaults.escrow_account menjadi dua kolom opsional 12 digit
--      tidak unik: escrow_cash dan escrow_atm.
--
-- ROLLBACK (documented, forward-only shop):
--   BEGIN;
--   ALTER TABLE public.vendor_vaults ADD COLUMN escrow_account text NULL
--     CONSTRAINT vendor_vaults_escrow_account_chk CHECK (escrow_account ~ '^[0-9]{12}$');
--   UPDATE public.vendor_vaults SET escrow_account = COALESCE(escrow_cash, escrow_atm)
--     WHERE escrow_cash IS NOT NULL OR escrow_atm IS NOT NULL;
--   ALTER TABLE public.vendor_vaults
--     DROP CONSTRAINT IF EXISTS vendor_vaults_escrow_cash_chk,
--     DROP CONSTRAINT IF EXISTS vendor_vaults_escrow_atm_chk,
--     DROP COLUMN IF EXISTS escrow_cash,
--     DROP COLUMN IF EXISTS escrow_atm;
--   ALTER TABLE public.vendor_branches
--     ADD COLUMN cis_limit_amount numeric(20,2) NULL,
--     ADD COLUMN cis_limit_currency character(3) NOT NULL DEFAULT 'IDR',
--     ADD CONSTRAINT vendor_branches_cis_limit_amount_chk CHECK (cis_limit_amount > 0);
--   UPDATE public.vendor_branches b
--   SET cis_limit_amount = sub.cis_limit_amount, cis_limit_currency = sub.cis_limit_currency
--   FROM (
--     SELECT DISTINCT ON (vendor_branch_id) vendor_branch_id, cis_limit_amount, cis_limit_currency
--     FROM public.vendor_vaults WHERE cis_limit_amount IS NOT NULL
--     ORDER BY vendor_branch_id, id ASC
--   ) sub WHERE sub.vendor_branch_id = b.id;
--   ALTER TABLE public.vendor_vaults
--     DROP CONSTRAINT IF EXISTS vendor_vaults_cis_limit_amount_chk,
--     DROP COLUMN IF EXISTS cis_limit_currency,
--     DROP COLUMN IF EXISTS cis_limit_amount;
--   DROP TRIGGER IF EXISTS trg_vendor_vaults_fill_vault_name ON public.vendor_vaults;
--   DROP FUNCTION IF EXISTS public.vendor_vaults_fill_vault_name();
--   DROP FUNCTION IF EXISTS public.next_vendor_vault_name(bigint);
--   DROP FUNCTION IF EXISTS public.format_vendor_vault_base_name(text, text, text, text, text);
--   DROP INDEX IF EXISTS public.vendor_vaults_vault_name_idx;
--   ALTER TABLE public.vendor_vaults
--     DROP CONSTRAINT IF EXISTS vendor_vaults_branch_vault_name_uq,
--     DROP CONSTRAINT IF EXISTS vendor_vaults_vault_name_chk,
--     DROP COLUMN IF EXISTS alamat,
--     DROP COLUMN IF EXISTS vault_name;
--   ALTER TABLE public.vendor_vaults RENAME COLUMN min_balance TO min_capacity_amount;
--   ALTER TABLE public.vendor_vaults RENAME COLUMN max_balance TO max_capacity_amount;
--   COMMIT;

BEGIN;

-- 1. Rename kolom kapasitas di vendor_vaults (tetap numeric(20,2)).
ALTER TABLE public.vendor_vaults RENAME COLUMN min_capacity_amount TO min_balance;
ALTER TABLE public.vendor_vaults RENAME COLUMN max_capacity_amount TO max_balance;

COMMENT ON COLUMN public.vendor_vaults.min_balance IS
    'Saldo minimum vault dalam currency_code (numeric(20,2), tanpa float). Rename dari min_capacity_amount pada migrasi 040.';
COMMENT ON COLUMN public.vendor_vaults.max_balance IS
    'Saldo maksimum vault dalam currency_code (numeric(20,2), tanpa float). Rename dari max_capacity_amount pada migrasi 040.';

-- 2. Fungsi pembentuk prefix & penamaan otomatis vault_name (<vendorPrefix>_<area>_V<3-digit>).
CREATE FUNCTION public.format_vendor_vault_base_name(
    p_vendor_code text,
    p_vendor_name text,
    p_request_prefix text,
    p_branch_code text,
    p_branch_name text
) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
  SELECT
    COALESCE(
      NULLIF(btrim(p_request_prefix), ''),
      rpad(left(regexp_replace(upper(COALESCE(p_vendor_code, '')), '[^A-Z0-9]', '', 'g'), 3), 3, 'X')
    )
    || '_'
    || COALESCE(
      NULLIF(
        regexp_replace(
          btrim(
            regexp_replace(
              btrim(COALESCE(p_branch_name, '')),
              '^' || regexp_replace(btrim(COALESCE(p_vendor_name, '')), '([.*+?^${}()|[\]\\])', '\\\1', 'g') || '\s+',
              '',
              'i'
            )
          ),
          '\s+',
          '_',
          'g'
        ),
        ''
      ),
      regexp_replace(btrim(COALESCE(NULLIF(p_branch_code, ''), 'VAULT')), '\s+', '_', 'g')
    )
$$;

COMMENT ON FUNCTION public.format_vendor_vault_base_name(text, text, text, text, text) IS
    'Membentuk bagian dasar <vendorPrefix>_<area> untuk vendor_vaults.vault_name (contoh: ABA_Bali). Migrasi 040.';

CREATE FUNCTION public.next_vendor_vault_name(p_vendor_branch_id bigint)
RETURNS text
LANGUAGE plpgsql AS $$
DECLARE
  v_base    text;
  v_escaped text;
  v_seq     int;
BEGIN
  SELECT public.format_vendor_vault_base_name(v.code, v.name, v.request_prefix::text, b.branch_code, b.branch_name)
    INTO v_base
  FROM public.vendor_branches b
  JOIN public.vendors v ON v.id = b.vendor_id
  WHERE b.id = p_vendor_branch_id;

  IF v_base IS NULL THEN
    v_base := 'VLT_BRANCH_' || COALESCE(p_vendor_branch_id::text, '0');
  END IF;

  v_escaped := regexp_replace(v_base, '([.*+?^${}()|[\]\\])', '\\\1', 'g');

  SELECT COALESCE(MAX(substring(vl.vault_name FROM '_V([0-9]+)$')::int), 0) + 1
    INTO v_seq
  FROM public.vendor_vaults vl
  WHERE vl.vendor_branch_id = p_vendor_branch_id
    AND vl.vault_name ~ ('^' || v_escaped || '_V[0-9]+$');

  RETURN v_base || '_V' || lpad(v_seq::text, 3, '0');
END
$$;

COMMENT ON FUNCTION public.next_vendor_vault_name(bigint) IS
    'Menghasilkan vault_name berikutnya untuk satu cabang dengan format <vendorPrefix>_<area>_V<3-digit> mulai dari V001. Migrasi 040.';

-- 3. Tambah kolom vault_name & alamat di vendor_vaults, lalu backfill vault_name.
ALTER TABLE public.vendor_vaults
    ADD COLUMN vault_name text NULL,
    ADD COLUMN alamat text NULL;

WITH vault_backfill AS (
    SELECT vl.id,
           public.format_vendor_vault_base_name(v.code, v.name, v.request_prefix::text, b.branch_code, b.branch_name)
               || '_V'
               || lpad(row_number() OVER (PARTITION BY vl.vendor_branch_id ORDER BY vl.id)::text, 3, '0') AS computed_vault_name
    FROM public.vendor_vaults vl
    JOIN public.vendor_branches b ON b.id = vl.vendor_branch_id
    JOIN public.vendors v ON v.id = b.vendor_id
    WHERE vl.vault_name IS NULL OR btrim(vl.vault_name) = ''
)
UPDATE public.vendor_vaults vl
SET vault_name = bf.computed_vault_name
FROM vault_backfill bf
WHERE bf.id = vl.id;

ALTER TABLE public.vendor_vaults
    ALTER COLUMN vault_name SET NOT NULL,
    ADD CONSTRAINT vendor_vaults_vault_name_chk CHECK (length(btrim(vault_name)) > 0),
    ADD CONSTRAINT vendor_vaults_branch_vault_name_uq UNIQUE (vendor_branch_id, vault_name);

CREATE INDEX vendor_vaults_vault_name_idx ON public.vendor_vaults USING btree (vault_name);

CREATE FUNCTION public.vendor_vaults_fill_vault_name()
RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.vault_name IS NULL OR btrim(NEW.vault_name) = '' THEN
    NEW.vault_name := public.next_vendor_vault_name(NEW.vendor_branch_id);
  ELSE
    NEW.vault_name := btrim(NEW.vault_name);
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER trg_vendor_vaults_fill_vault_name
    BEFORE INSERT ON public.vendor_vaults
    FOR EACH ROW
    EXECUTE FUNCTION public.vendor_vaults_fill_vault_name();

COMMENT ON COLUMN public.vendor_vaults.vault_name IS
    'Nama tampilan/identitas vault pada cabang (format default <vendorPrefix>_<area>_V<3-digit>, contoh: ABA_Bali_V001). Unik per cabang. Migrasi 040.';
COMMENT ON COLUMN public.vendor_vaults.alamat IS
    'Alamat fisik vault vendor (opsional). Master-data maker-checker. Migrasi 040.';

-- 4. Pindahkan cis_limit_amount & cis_limit_currency dari vendor_branches ke vendor_vaults.
ALTER TABLE public.vendor_vaults
    ADD COLUMN cis_limit_amount numeric(20,2) NULL,
    ADD COLUMN cis_limit_currency character(3) NOT NULL DEFAULT 'IDR',
    ADD CONSTRAINT vendor_vaults_cis_limit_amount_chk CHECK (cis_limit_amount > 0);

UPDATE public.vendor_vaults vl
SET cis_limit_amount = b.cis_limit_amount,
    cis_limit_currency = b.cis_limit_currency
FROM public.vendor_branches b
WHERE b.id = vl.vendor_branch_id
  AND b.cis_limit_amount IS NOT NULL;

ALTER TABLE public.vendor_branches
    DROP CONSTRAINT IF EXISTS vendor_branches_cis_limit_amount_chk,
    DROP COLUMN IF EXISTS cis_limit_currency,
    DROP COLUMN IF EXISTS cis_limit_amount;

COMMENT ON COLUMN public.vendor_vaults.cis_limit_amount IS
    'Limit CIS (= Limit Asuransi) per vault dalam cis_limit_currency (numeric(20,2) > 0, NULL = belum diisi). Dipindahkan dari vendor_branches pada migrasi 040.';
COMMENT ON COLUMN public.vendor_vaults.cis_limit_currency IS
    'Mata uang cis_limit_amount (ISO 4217, default IDR). Dipindahkan dari vendor_branches pada migrasi 040.';

-- 5. Pecah escrow_account menjadi escrow_cash & escrow_atm di vendor_vaults.
ALTER TABLE public.vendor_vaults
    ADD COLUMN escrow_cash text NULL CONSTRAINT vendor_vaults_escrow_cash_chk CHECK (escrow_cash ~ '^[0-9]{12}$'),
    ADD COLUMN escrow_atm  text NULL CONSTRAINT vendor_vaults_escrow_atm_chk  CHECK (escrow_atm ~ '^[0-9]{12}$');

UPDATE public.vendor_vaults
SET escrow_cash = CASE WHEN category IN ('CASH', 'ATM_CASH') THEN escrow_account ELSE NULL END,
    escrow_atm  = CASE WHEN category IN ('ATM', 'ATM_CASH')  THEN escrow_account ELSE NULL END
WHERE escrow_account IS NOT NULL;

ALTER TABLE public.vendor_vaults
    DROP CONSTRAINT IF EXISTS vendor_vaults_escrow_account_chk,
    DROP COLUMN IF EXISTS escrow_account;

COMMENT ON COLUMN public.vendor_vaults.escrow_cash IS
    'Nomor rekening escrow Cash vault (12 digit angka, opsional, tidak unik). Master-data maker-checker. Migrasi 040.';
COMMENT ON COLUMN public.vendor_vaults.escrow_atm IS
    'Nomor rekening escrow ATM vault (12 digit angka, opsional, tidak unik). Master-data maker-checker. Migrasi 040.';

COMMIT;
