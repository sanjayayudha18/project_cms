-- 023_atm_assignment_vendor_source.sql
-- Kelolaan ATM bisa memakai paket vendor-wide (label dari vendor_package_prices)
-- selain paket khusus cabang (.claude/sdlc/atm-package-source/spec.md, accepted
-- 2026-10-07).
--
--   * atm_vendor_packages.vendor_package_id jadi nullable. Mode 'branch' (lama):
--     hanya vendor_package_id terisi. Mode 'vendor' (baru): vendor_id +
--     vendor_branch_id + package (label, join ke package_frequencies.package_code).
--   * avp_source_chk menjamin tepat satu mode; avp_vendor_mode_uq mencegah
--     duplikat mode 'vendor'. atm_vendor_packages_no_overlap (migrasi 003)
--     tetap menjaga tumpang tindih lintas mode (atm_id + rentang tanggal).
--
-- SAFETY:
--   * Additive; semua baris existing punya vendor_package_id terisi dan kolom
--     baru NULL, jadi avp_source_chk valid tanpa backfill.
--   * Forward-only: no down migration (project convention); rollback di bawah
--     hanya aman selama belum ada baris mode 'vendor'.
BEGIN;

ALTER TABLE public.atm_vendor_packages
    ALTER COLUMN vendor_package_id DROP NOT NULL,
    ADD COLUMN vendor_id        bigint REFERENCES public.vendors(id),
    ADD COLUMN vendor_branch_id bigint REFERENCES public.vendor_branches(id),
    ADD COLUMN package          text;

ALTER TABLE public.atm_vendor_packages
    ADD CONSTRAINT avp_source_chk CHECK (
        (vendor_package_id IS NOT NULL AND vendor_id IS NULL AND vendor_branch_id IS NULL AND package IS NULL)
     OR (vendor_package_id IS NULL AND vendor_id IS NOT NULL AND vendor_branch_id IS NOT NULL
         AND package IS NOT NULL AND btrim(package) <> '')
    );

CREATE UNIQUE INDEX avp_vendor_mode_uq
    ON public.atm_vendor_packages (atm_id, vendor_id, vendor_branch_id, package, effective_start_date)
    WHERE vendor_package_id IS NULL;

CREATE INDEX avp_vendor_branch_idx
    ON public.atm_vendor_packages (vendor_branch_id) WHERE vendor_branch_id IS NOT NULL;

COMMENT ON COLUMN public.atm_vendor_packages.package
    IS 'Mode vendor-wide: label paket persis seperti vendor_package_prices.package (mis. PAKET 4). NULL pada mode paket cabang.';

COMMIT;

-- ROLLBACK (hanya bila belum ada baris mode vendor-wide):
--   DROP INDEX IF EXISTS public.avp_vendor_branch_idx;
--   DROP INDEX IF EXISTS public.avp_vendor_mode_uq;
--   ALTER TABLE public.atm_vendor_packages DROP CONSTRAINT IF EXISTS avp_source_chk;
--   ALTER TABLE public.atm_vendor_packages DROP COLUMN package, DROP COLUMN vendor_branch_id, DROP COLUMN vendor_id;
--   ALTER TABLE public.atm_vendor_packages ALTER COLUMN vendor_package_id SET NOT NULL;
