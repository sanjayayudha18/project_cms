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

COMMIT;
