-- Seed: Roles and Users
-- Run after 002_cms_tables.sql
-- Passwords are bcrypt hash of 'password123' (cost 10)
-- In production, change passwords immediately or use LDAP auth.

BEGIN;

-- ============================================================
-- 1. Seed Roles
-- ============================================================
INSERT INTO public.roles (role, description) VALUES
    ('ADMIN',           'Super administrator — full system access'),
    ('ADMIN_PARAM',     'Parameter admin — manage master data (ATMs, regions, vendors, denoms)'),
    ('ATM-USER',        'ATM operations staff — daily DSR upload, cash count, order entry'),
    ('ATM-SPV',         'ATM supervisor — approve orders, review forecasts, monitor cash flow'),
    ('BRANCH-USER',     'Branch operations staff — branch cash management tasks'),
    ('BRANCH-SPV',      'Branch supervisor — approve branch-level transactions'),
    ('BRANCH-ATM-USER', 'Branch ATM operator — combined branch + ATM operational role'),
    ('BRANCH-ATM-SPV',  'Branch ATM supervisor — approve combined branch + ATM operations'),
    ('VENDOR-USER',     'CIT vendor user — view schedules, upload delivery confirmations')
ON CONFLICT (role) DO NOTHING;

-- ============================================================
-- 2. Seed a vendor (needed for VENDOR-USER FK)
-- ============================================================
INSERT INTO public.vendors (code, name, contact_email, contact_phone, hq_address, is_active) VALUES
    ('SSI', 'PT Sinergi Sarana Integrasi', 'ops@ssi.co.id', '021-5551234', 'Jl. Jend. Sudirman Kav. 52-53, Jakarta', true)
ON CONFLICT (code) DO NOTHING;

-- ============================================================
-- 3. Seed Users (one per role for development/testing)
-- ============================================================
-- bcrypt hash for 'password123': $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
INSERT INTO public.users (role_id, employee_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id, is_active) VALUES
    (
        (SELECT id FROM public.roles WHERE role = 'ADMIN'),
        'EMP001',
        'Yudha',
        'System Administrator',
        'admin@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'ADMIN_PARAM'),
        'EMP002',
        'Rangga',
        'Parameter Admin',
        'admin.param@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'ATM-USER'),
        'EMP003',
        'atm.user',
        'Budi Santoso',
        'budi.santoso@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'ATM-SPV'),
        'EMP004',
        'atm.spv',
        'Dewi Lestari',
        'dewi.lestari@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'BRANCH-USER'),
        'EMP005',
        'branch.user',
        'Andi Prasetyo',
        'andi.prasetyo@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'BRANCH-SPV'),
        'EMP006',
        'branch.spv',
        'Siti Rahayu',
        'siti.rahayu@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'BRANCH-ATM-USER'),
        'EMP007',
        'branch.atm.user',
        'Rizky Firmansyah',
        'rizky.firmansyah@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'BRANCH-ATM-SPV'),
        'EMP008',
        'branch.atm.spv',
        'Maya Putri',
        'maya.putri@cimbniaga.co.id',
        true,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        NULL,
        true
    ),
    (
        (SELECT id FROM public.roles WHERE role = 'VENDOR-USER'),
        NULL,
        'vendor.ssi',
        'Ahmad Hidayat',
        'ahmad.hidayat@ssi.co.id',
        false,
        'local',
        '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
        (SELECT id FROM public.vendors WHERE code = 'SSI'),
        true
    )
ON CONFLICT (username) DO NOTHING;

COMMIT;
