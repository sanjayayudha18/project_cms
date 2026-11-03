-- DEV ONLY — not a migration. Seeds VendorPortal users for the manual test of cit-send-vendor (terima/tolak order):
--   one vendor-wide VENDOR-USER (vendor_branch_id NULL = sees every branch of its vendor) per active FLM vendor,
--   username vendor.<vendor code lowercase>, local auth, password = dev seed password (password123)
-- Skips integration-test vendors (code IT%) and vendors that already have an active vendor-wide VENDOR-USER.
-- Idempotent: safe to re-run. Written directly (no audit_logs) — dev data, not a user action.
-- Run: psql "postgres://…@localhost:5432/cms" -f backend/scripts/dev/seed_vendor_users.sql
BEGIN;

INSERT INTO public.users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id)
SELECT r.id, 'vendor.' || lower(v.code), v.name || ' Operator Dev', 'vendor.' || lower(v.code) || '@vendor.dev.local',
       false, 'local', '$2a$10$tFDuAumLIoM.bD25Zrt0vuq/rbnslTSeYNvmSD4KS62wobuRmpwsi', v.id
FROM public.vendors v
JOIN public.roles r ON r.role = 'VENDOR-USER'
WHERE v.is_active AND v.kind = 'FLM_VENDOR' AND v.code NOT LIKE 'IT%'
  AND NOT EXISTS (SELECT 1 FROM public.users u
                  WHERE u.vendor_id = v.id AND u.vendor_branch_id IS NULL AND u.is_active AND u.role_id = r.id)
ON CONFLICT DO NOTHING;

-- Bagian 2 (vendor-party-scope, task 13): empat akun ABACUS untuk tes pembagian pihak per peran.
--   VENDOR-ATM-USER = pihak replenish saja; VENDOR-VAULT-USER = pihak vault saja (Req 9.3).
--   Cabang dicari lewat branch_code (ABACUS_004 = Medan, ABACUS_002 = Bandung) di vendor ABA.
--   Email <username>@vendor.dev.local. Idempoten lewat ON CONFLICT (username) DO NOTHING.
INSERT INTO public.users (role_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id, vendor_branch_id)
SELECT r.id, s.username, s.full_name, s.username || '@vendor.dev.local', false, 'local',
       '$2a$10$tFDuAumLIoM.bD25Zrt0vuq/rbnslTSeYNvmSD4KS62wobuRmpwsi', b.vendor_id, b.id
FROM (VALUES
    ('vendor.abacus.mdn.atm',   'VENDOR-ATM-USER',   'ABACUS_004', 'Abacus Medan ATM Dev'),
    ('vendor.abacus.mdn.vault', 'VENDOR-VAULT-USER', 'ABACUS_004', 'Abacus Medan Vault Dev'),
    ('vendor.abacus.bdg.atm',   'VENDOR-ATM-USER',   'ABACUS_002', 'Abacus Bandung ATM Dev'),
    ('vendor.abacus.bdg.vault', 'VENDOR-VAULT-USER', 'ABACUS_002', 'Abacus Bandung Vault Dev')
) AS s(username, role, branch_code, full_name)
JOIN public.roles r ON r.role = s.role
JOIN public.vendors v ON v.code = 'ABA' AND v.is_active
JOIN public.vendor_branches b ON b.vendor_id = v.id AND b.branch_code = s.branch_code AND b.is_active AND b.deleted_at IS NULL
ON CONFLICT (username) DO NOTHING;

COMMIT;
