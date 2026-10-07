-- 012_drop_vendor_package_price_addons.sql
-- Drop add_cr_price, add_flm_price, extra_amount from vendor_package_prices
-- (explicit user request). No seeded data depends on these columns (P20
-- seed was base_price only; per-ATM "Harga Khusus" / "Biaya Tambahan
-- Operasional" rows were never seeded -- see 009/010 notes).

BEGIN;

ALTER TABLE public.vendor_package_prices
    DROP CONSTRAINT IF EXISTS vpp_amount_chk;

ALTER TABLE public.vendor_package_prices
    ADD CONSTRAINT vpp_amount_chk CHECK (base_price IS NULL OR base_price >= 0);

ALTER TABLE public.vendor_package_prices
    DROP COLUMN IF EXISTS add_cr_price,
    DROP COLUMN IF EXISTS add_flm_price,
    DROP COLUMN IF EXISTS extra_amount;

COMMIT;
