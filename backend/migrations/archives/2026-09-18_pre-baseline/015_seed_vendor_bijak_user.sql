-- Seed: a login user for vendor BIJAK, scoped to the "Bijak Jakarta" branch.
-- Run after 003_seed_roles_users.sql (roles + VENDOR-USER), 005_seed_vendors.sql (BIJAK),
-- 006_seed_vendor_branches_fixed.sql (BIJAK_001 = "Bijak Jakarta"), and
-- 016_users_vendor_branch_id.sql (adds users.vendor_branch_id).
--
-- Password is the bcrypt hash of 'password123' (cost 10) — same dev credential used in
-- 003_seed_roles_users.sql. Change it immediately in any non-dev environment.
--
-- The user is scoped to vendor BIJAK and pinned to the "Bijak Jakarta" branch via
-- users.vendor_branch_id (added in migration 016).

BEGIN;

-- Guard: fail loudly if the "Bijak Jakarta" branch is missing, so this seed never silently
-- assigns a user to a vendor whose intended branch was never seeded.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM public.vendor_branches vb
        JOIN public.vendors v ON v.id = vb.vendor_id
        WHERE v.code = 'BIJAK'
          AND vb.branch_code = 'BIJAK_001'
    ) THEN
        RAISE EXCEPTION 'Branch BIJAK_001 (Bijak Jakarta) not found. Run 006_seed_vendor_branches_fixed.sql first.';
    END IF;
END $$;

-- Login user for vendor BIJAK. auth_source = local (vendor portal credentials).
-- bcrypt('password123', 10) = $2a$10$tFDuAumLIoM.bD25Zrt0vuq/rbnslTSeYNvmSD4KS62wobuRmpwsi
INSERT INTO public.users (
    role_id, employee_id, username, full_name, email,
    is_karyawan, auth_source, password_hash, vendor_id, vendor_branch_id, is_active
) VALUES (
    (SELECT id FROM public.roles WHERE role = 'VENDOR-USER'),
    NULL,
    'vendor.bijak',
    'Bijak Jakarta Operator',
    'ops.jakarta@bijak.co.id',
    false,
    'local',
    '$2a$10$tFDuAumLIoM.bD25Zrt0vuq/rbnslTSeYNvmSD4KS62wobuRmpwsi',
    (SELECT id FROM public.vendors WHERE code = 'BIJAK'),
    (SELECT id FROM public.vendor_branches WHERE branch_code = 'BIJAK_001'),
    true
)
ON CONFLICT (username) DO NOTHING;

-- Idempotent branch assignment: if the user already existed (e.g. this seed was applied
-- before migration 016 added vendor_branch_id), pin it to the "Bijak Jakarta" branch now.
UPDATE public.users
SET vendor_branch_id = (SELECT id FROM public.vendor_branches WHERE branch_code = 'BIJAK_001'),
    vendor_id        = (SELECT id FROM public.vendors WHERE code = 'BIJAK')
WHERE username = 'vendor.bijak'
  AND vendor_branch_id IS DISTINCT FROM (SELECT id FROM public.vendor_branches WHERE branch_code = 'BIJAK_001');

COMMIT;
