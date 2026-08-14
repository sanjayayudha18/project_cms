
BEGIN;

-- =====================================================
-- Seed regions
-- =====================================================
INSERT INTO public.regions (code, region)
SELECT s.code, s.region
FROM (
    VALUES
        ('JKT', 'Jakarta'),
        ('TGR', 'Tangerang')
) AS s(code, region)
WHERE NOT EXISTS (
    SELECT 1
    FROM public.regions r
    WHERE r.code = s.code
);

-- =====================================================
-- Seed locations
-- Matches schema:
--   region_id BIGINT NOT NULL
--   type TEXT NOT NULL
--   name TEXT NOT NULL
--   address_line1 TEXT NOT NULL
--   city_or_regency TEXT NOT NULL
--   province TEXT NOT NULL
-- =====================================================
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
        ('JKT', 'VIP',    'JKT.EBL CENTRAL PARK CRM 1',               'JKT.EBL CENTRAL PARK CRM 1',               NULL, NULL, NULL, NULL, NULL, 'UNKNOWN', 'DKI Jakarta', NULL, 'ID'),
        ('JKT', 'VIP',    'JKT.EBL.CITY WALK.CRM',                    'JKT.EBL.CITY WALK.CRM',                    NULL, NULL, NULL, NULL, NULL, 'UNKNOWN', 'DKI Jakarta', NULL, 'ID'),
        ('TGR', 'Normal', 'TGR.SPBU34-15404 CIRENDEU CRM',            'TGR.SPBU34-15404 CIRENDEU CRM',            NULL, NULL, NULL, NULL, NULL, 'Tangerang Selatan', 'Banten', NULL, 'ID'),
        ('JKT', 'Normal', 'JKT.SPBU KEMANGGISAN CRM',                 'JKT.SPBU KEMANGGISAN CRM',                 NULL, NULL, NULL, NULL, NULL, 'Jakarta Barat', 'DKI Jakarta', NULL, 'ID'),
        ('JKT', 'Normal', 'JKT.SPBU Benzine 34-11405 Palmerah Utara', 'JKT.SPBU Benzine 34-11405 Palmerah Utara', NULL, NULL, NULL, NULL, NULL, 'Jakarta Barat', 'DKI Jakarta', NULL, 'ID'),
        ('JKT', 'Normal', 'JKT.CRM.Senayan Park',                     'JKT.CRM.Senayan Park',                     NULL, NULL, NULL, NULL, NULL, 'Jakarta Pusat', 'DKI Jakarta', NULL, 'ID')
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

COMMIT;