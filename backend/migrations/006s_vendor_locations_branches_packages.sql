-- =====================================================
-- 007_seed_vendor_locations_branches_packages.sql
-- =====================================================

-- -----------------------------------------------------
-- 1) Seed regions needed by vendor branch locations
-- -----------------------------------------------------



INSERT INTO public.regions (code, region)
SELECT s.code, s.region
FROM (
    VALUES
        ('JKT', 'Jakarta'),
        ('BDG', 'Bandung')
) AS s(code, region)
WHERE NOT EXISTS (
    SELECT 1
    FROM public.regions r
    WHERE r.code = s.code
);

-- -----------------------------------------------------
-- 2) Seed locations for vendor branches
--    Assumption: these are branch/HQ operating locations
-- -----------------------------------------------------
INSERT INTO public.locations (
    region_id,
    type,
    name,
    address_line1,
    address_line2,
    rt,
    rw,
    kelurahan,
    kecamatan,
    city_or_regency,
    province,
    postal_code,
    country_code
)
SELECT
    r.id,
    s.type,
    s.name,
    s.address_line1,
    s.address_line2,
    s.rt,
    s.rw,
    s.kelurahan,
    s.kecamatan,
    s.city_or_regency,
    s.province,
    s.postal_code,
    s.country_code
FROM (
    VALUES
        (
            'JKT',
            'Normal',
            'Crown Teknologi Jakarta Branch',
            'Jakarta',
            NULL,
            NULL,
            NULL,
            NULL,
            NULL,
            'Jakarta',
            'DKI Jakarta',
            NULL,
            'ID'
        ),
        (
            'BDG',
            'Normal',
            'Mitra ATM Service Bandung Branch',
            'Bandung',
            NULL,
            NULL,
            NULL,
            NULL,
            NULL,
            'Bandung',
            'Jawa Barat',
            NULL,
            'ID'
        )
) AS s(
    region_code,
    type,
    name,
    address_line1,
    address_line2,
    rt,
    rw,
    kelurahan,
    kecamatan,
    city_or_regency,
    province,
    postal_code,
    country_code
)
JOIN public.regions r
    ON r.code = s.region_code
WHERE NOT EXISTS (
    SELECT 1
    FROM public.locations l
    WHERE l.name = s.name
);

-- -----------------------------------------------------
-- 3) Seed vendor_branches
--    Uses vendors.code + locations.name
-- -----------------------------------------------------
INSERT INTO public.vendor_branches (
    vendor_id,
    branch_code,
    branch_name,
    location_id,
    is_active
)
SELECT
    v.id,
    s.branch_code,
    s.branch_name,
    l.id,
    s.is_active
FROM (
    VALUES
        ('VND001', 'VND001-JKT-001', 'Crown Teknologi Jakarta Branch', 'Crown Teknologi Jakarta Branch', true),
        ('VND002', 'VND002-BDG-001', 'Mitra ATM Service Bandung Branch', 'Mitra ATM Service Bandung Branch', true)
) AS s(vendor_code, branch_code, branch_name, location_name, is_active)
JOIN public.vendors v
    ON v.code = s.vendor_code
JOIN public.locations l
    ON l.name = s.location_name
WHERE NOT EXISTS (
    SELECT 1
    FROM public.vendor_branches vb
    WHERE vb.vendor_id = v.id
      AND vb.branch_code = s.branch_code
);

-- -----------------------------------------------------
-- 4) Seed vendor_packages
--    Uses vendor code + branch code
-- -----------------------------------------------------
INSERT INTO public.vendor_packages (
    vendor_branch_id,
    code,
    type,
    price
)
SELECT
    vb.id,
    s.package_code,
    s.package_type,
    s.price
FROM (
    VALUES
        ('VND001', 'VND001-JKT-001', 'PKG-CASH', 'cash_replenishment', 1500000.00),
        ('VND001', 'VND001-JKT-001', 'PKG-FULL', 'full_service',       2500000.00),
        ('VND002', 'VND002-BDG-001', 'PKG-CASH', 'cash_replenishment', 1400000.00),
        ('VND002', 'VND002-BDG-001', 'PKG-FLM',  'flm',                1200000.00)
) AS s(vendor_code, branch_code, package_code, package_type, price)
JOIN public.vendors v
    ON v.code = s.vendor_code
JOIN public.vendor_branches vb
    ON vb.vendor_id = v.id
   AND vb.branch_code = s.branch_code
WHERE NOT EXISTS (
    SELECT 1
    FROM public.vendor_packages vp
    WHERE vp.vendor_branch_id = vb.id
      AND vp.code = s.package_code
);

commit;