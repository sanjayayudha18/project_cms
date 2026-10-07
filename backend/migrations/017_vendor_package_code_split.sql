-- 017_vendor_package_code_split.sql
-- Pisah satu kolom identifier di vendor_package_prices jadi dua konsep.
-- Requirement/design: .kiro/specs/vendor-package-code-split/{requirements,design}.md
--
-- Kolom package_code hari ini menyimpan LABEL paket ("PAKET 3") dan jadi kunci
-- pengelompokan di vpp_no_overlap + vpp_lookup_idx. Migrasi ini:
--   1. RENAME package_code -> package (data, NOT NULL, peran grouping tetap).
--   2. ADD package_code text baru: kode unik per baris, backfill deterministik
--      PKG<digit-dari-package>_<vendor.code>_<urutan-3-digit-per-vendor>.
--
-- REVERSE (tidak dieksekusi sebagai down, forward-only per konvensi repo):
--   a. DROP CONSTRAINT vendor_package_prices_package_code_key
--   b. DROP CONSTRAINT vpp_no_overlap; DROP INDEX vpp_lookup_idx
--   c. ALTER TABLE ... DROP COLUMN package_code (data turunan, aman hilang)
--   d. ALTER TABLE ... RENAME COLUMN package TO package_code (data label utuh)
--   e. Buat ulang vpp_no_overlap/vpp_lookup_idx pada package_code seperti 009
-- Lossless kedua arah karena data kolom grouping tidak pernah berpindah; hanya
-- kolom per-baris yang dibuat/dibuang.

BEGIN;

-- 1. Rename kolom grouping. Data, NOT NULL, dan peran grouping tetap (Req 1.1-1.3).
ALTER TABLE public.vendor_package_prices RENAME COLUMN package_code TO package;

-- 2. Tambah kolom kode per-baris baru, nullable selama backfill (Req 2.1).
ALTER TABLE public.vendor_package_prices ADD COLUMN package_code text;

-- 3. Backfill deterministik (Req 2.2/2.3).
--    <digits> = karakter numerik dari label, atau token fallback '0' saat label
--    tanpa digit. <vendor.code> dari vendors. <seq> = nomor urut 3-digit per vendor.
WITH numbered AS (
    SELECT vpp.id,
           v.code AS vendor_code,
           COALESCE(NULLIF(regexp_replace(vpp.package, '\D', '', 'g'), ''), '0') AS digits,
           lpad(ROW_NUMBER() OVER (PARTITION BY vpp.vendor_id ORDER BY vpp.id)::text, 3, '0') AS seq
    FROM public.vendor_package_prices vpp
    JOIN public.vendors v ON v.id = vpp.vendor_id
)
UPDATE public.vendor_package_prices vpp
SET package_code = 'PKG' || n.digits || '_' || n.vendor_code || '_' || n.seq
FROM numbered n
WHERE n.id = vpp.id;

-- 4. Validasi pre-commit (Req 4.3/4.4/4.5). Kegagalan apa pun -> abort transaksi.
DO $$
DECLARE dup int; null_pkg int; null_code int;
BEGIN
    SELECT count(*) INTO dup FROM (
        SELECT package_code FROM public.vendor_package_prices
        GROUP BY package_code HAVING count(*) > 1
    ) d;
    IF dup > 0 THEN RAISE EXCEPTION '017: % duplicate package_code values', dup; END IF;

    SELECT count(*) INTO null_pkg  FROM public.vendor_package_prices WHERE package IS NULL;
    IF null_pkg > 0 THEN RAISE EXCEPTION '017: % null package values', null_pkg; END IF;

    SELECT count(*) INTO null_code FROM public.vendor_package_prices WHERE package_code IS NULL;
    IF null_code > 0 THEN RAISE EXCEPTION '017: % null package_code values', null_code; END IF;
END $$;

-- 5. Kunci kolom baru (Req 2.6).
ALTER TABLE public.vendor_package_prices ALTER COLUMN package_code SET NOT NULL;
ALTER TABLE public.vendor_package_prices
    ADD CONSTRAINT vendor_package_prices_package_code_key UNIQUE (package_code);

-- 6. Bangun ulang overlap + lookup, package menggantikan peran package_code lama.
--    Selebihnya identik dengan 009 (Req 1.4/1.5, 3.x).
ALTER TABLE public.vendor_package_prices DROP CONSTRAINT vpp_no_overlap;
ALTER TABLE public.vendor_package_prices
    ADD CONSTRAINT vpp_no_overlap EXCLUDE USING gist (
        vendor_id                     WITH =,
        package                       WITH =,
        machine_group                 WITH =,
        price_class                   WITH =,
        COALESCE(vendor_branch_id, 0) WITH =,
        COALESCE(atm_id, 0)           WITH =,
        int4range(tier_min, COALESCE(tier_max, 999999999), '[]') WITH &&,
        daterange(effective_start_date, effective_end_date, '[]') WITH &&
    );

DROP INDEX IF EXISTS vpp_lookup_idx;
CREATE INDEX vpp_lookup_idx
    ON public.vendor_package_prices (vendor_id, package, machine_group, price_class);

COMMENT ON COLUMN public.vendor_package_prices.package
    IS 'Label paket yang dibagi banyak baris harga ("PAKET 3"), kunci pengelompokan. Sebelum migrasi 017 bernama package_code. Bergabung ke package_frequencies.package_code.';

COMMENT ON COLUMN public.vendor_package_prices.package_code
    IS 'Kode unik per baris ("PKG3_ABA_001"), dibuat server saat apply-on-approve. Format PKG<digit-label>_<vendor.code>_<urutan-3-digit>. Migrasi 017.';

COMMIT;
