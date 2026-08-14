-- =====================================================
-- 008_seed_atms_and_related.sql
-- =====================================================

BEGIN;

-- -----------------------------------------------------
-- 1) Seed 10 ATMs
-- -----------------------------------------------------
INSERT INTO public.atms (
    terminal_id,
    location_id,
    machine_type,
    brand,
    model,
    operation_hours,
    deployment_type,
    capacity_amount,
    low_threshold_amount,
    critical_threshold_amount,
    blacklisted,
    is_active
)
SELECT
    s.terminal_id,
    l.id,
    s.machine_type,
    s.brand,
    s.model,
    s.operation_hours,
    s.deployment_type,
    s.capacity_amount,
    s.low_threshold_amount,
    s.critical_threshold_amount,
    s.blacklisted,
    s.is_active
FROM (
    VALUES
        ('ATM-JKT-0001', 'JKT.EBL CENTRAL PARK CRM 1',               'CRM', 'NCR',     'SelfServ 87', '24x7',         'Off-Premise', 120000000.00, 40000000.00, 20000000.00, false, true),
        ('ATM-JKT-0002', 'JKT.EBL.CITY WALK.CRM',                    'CRM', 'NCR',     'SelfServ 87', '24x7',         'Off-Premise', 120000000.00, 40000000.00, 20000000.00, false, true),
        ('ATM-TGR-0003', 'TGR.SPBU34-15404 CIRENDEU CRM',            'CRM', 'Diebold', 'DN Series',   '24x7',         'Off-Premise', 100000000.00, 35000000.00, 15000000.00, false, true),
        ('ATM-JKT-0004', 'JKT.SPBU KEMANGGISAN CRM',                 'CRM', 'Diebold', 'DN Series',   '24x7',         'Off-Premise', 100000000.00, 35000000.00, 15000000.00, false, true),
        ('ATM-JKT-0005', 'JKT.SPBU Benzine 34-11405 Palmerah Utara', 'ATM', 'Wincor',  'ProCash 285', '24x7',         'Off-Premise',  80000000.00, 30000000.00, 12000000.00, false, true),
        ('ATM-JKT-0006', 'JKT.CRM.Senayan Park',                     'CRM', 'NCR',     'SelfServ 87', '24x7',         'Off-Premise', 150000000.00, 50000000.00, 25000000.00, false, true),
        ('ATM-JKT-0007', 'JKT.EBL CENTRAL PARK CRM 1',               'ATM', 'Wincor',  'ProCash 285', '24x7',         'Off-Premise',  90000000.00, 30000000.00, 15000000.00, false, true),
        ('ATM-JKT-0008', 'JKT.EBL.CITY WALK.CRM',                    'ATM', 'NCR',     'SelfServ 22', '24x7',         'Off-Premise',  90000000.00, 30000000.00, 15000000.00, false, true),
        ('ATM-TGR-0009', 'TGR.SPBU34-15404 CIRENDEU CRM',            'ATM', 'Diebold', 'DN Series',   '24x7',         'Off-Premise',  85000000.00, 28000000.00, 14000000.00, false, true),
        ('ATM-JKT-0010', 'JKT.CRM.Senayan Park',                     'CRM', 'NCR',     'SelfServ 87', 'branch_hours', 'On-Premise',  110000000.00, 36000000.00, 18000000.00, false, true)
) AS s(
    terminal_id,
    location_name,
    machine_type,
    brand,
    model,
    operation_hours,
    deployment_type,
    capacity_amount,
    low_threshold_amount,
    critical_threshold_amount,
    blacklisted,
    is_active
)
JOIN public.locations l
    ON l.name = s.location_name
WHERE NOT EXISTS (
    SELECT 1
    FROM public.atms a
    WHERE a.terminal_id = s.terminal_id
);

-- -----------------------------------------------------
-- 2) Seed ATM denom mappings
--    Each ATM gets IDR 50,000 and 100,000
-- -----------------------------------------------------
INSERT INTO public.atm_denoms (
    atm_id,
    denom_id
)
SELECT
    a.id,
    d.id
FROM public.atms a
JOIN (
    VALUES
        ('ATM-JKT-0001'),
        ('ATM-JKT-0002'),
        ('ATM-TGR-0003'),
        ('ATM-JKT-0004'),
        ('ATM-JKT-0005'),
        ('ATM-JKT-0006'),
        ('ATM-JKT-0007'),
        ('ATM-JKT-0008'),
        ('ATM-TGR-0009'),
        ('ATM-JKT-0010')
) AS t(terminal_id)
    ON t.terminal_id = a.terminal_id
JOIN public.denoms d
    ON d.denom IN (50000.00, 100000.00)
JOIN public.currencies c
    ON c.id = d.curr_id
   AND c.code = 'IDR'
WHERE NOT EXISTS (
    SELECT 1
    FROM public.atm_denoms ad
    WHERE ad.atm_id = a.id
      AND ad.denom_id = d.id
);

-- -----------------------------------------------------
-- 3) Seed ATM vendor package assignments
-- -----------------------------------------------------
INSERT INTO public.atm_vendor_packages (
    atm_id,
    vendor_package_id,
    effective_start_date,
    effective_end_date,
    is_active
)
SELECT
    a.id,
    vp.id,
    s.effective_start_date,
    s.effective_end_date,
    s.is_active
FROM (
    VALUES
        ('ATM-JKT-0001', 'VND001-JKT-001', 'PKG-FULL', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-JKT-0002', 'VND001-JKT-001', 'PKG-FULL', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-TGR-0003', 'VND001-JKT-001', 'PKG-CASH', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-JKT-0004', 'VND001-JKT-001', 'PKG-CASH', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-JKT-0005', 'VND001-JKT-001', 'PKG-CASH', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-JKT-0006', 'VND001-JKT-001', 'PKG-FULL', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-JKT-0007', 'VND002-BDG-001', 'PKG-CASH', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-JKT-0008', 'VND002-BDG-001', 'PKG-FLM',  DATE '2026-08-01', NULL::DATE, true),
        ('ATM-TGR-0009', 'VND002-BDG-001', 'PKG-CASH', DATE '2026-08-01', NULL::DATE, true),
        ('ATM-JKT-0010', 'VND001-JKT-001', 'PKG-FULL', DATE '2026-08-01', NULL::DATE, true)
) AS s(
    terminal_id,
    branch_code,
    package_code,
    effective_start_date,
    effective_end_date,
    is_active
)
JOIN public.atms a
    ON a.terminal_id = s.terminal_id
JOIN public.vendor_branches vb
    ON vb.branch_code = s.branch_code
JOIN public.vendor_packages vp
    ON vp.vendor_branch_id = vb.id
   AND vp.code = s.package_code
WHERE NOT EXISTS (
    SELECT 1
    FROM public.atm_vendor_packages avp
    WHERE avp.atm_id = a.id
      AND avp.vendor_package_id = vp.id
      AND avp.effective_start_date = s.effective_start_date
);

COMMIT;