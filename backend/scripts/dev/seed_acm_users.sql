-- DEV ONLY — not a migration. Seeds a manual-test setup for the ACM vault flow (cit-vault-select v3):
--   * acm.user (ACM-USER) + acm.spv (ACM-SPV), local auth, password = dev seed password (password123)
--   * one area "Area ACM Dev" holding every region code (active vendor branches) not yet in an area,
--     both users as members
-- Idempotent: safe to re-run. Written directly (no audit_logs) — dev data, not a user action.
-- Run: psql "postgres://…@localhost:5432/cms" -f backend/scripts/dev/seed_acm_users.sql
BEGIN;

INSERT INTO public.users (role_id, employee_id, username, full_name, email, is_karyawan, auth_source, password_hash)
SELECT r.id, v.emp, v.username, v.full_name, v.email, true, 'local',
       '$2a$10$tFDuAumLIoM.bD25Zrt0vuq/rbnslTSeYNvmSD4KS62wobuRmpwsi'
FROM (VALUES
    ('ACM-USER', 'EMPACM01', 'acm.user', 'ACM User Dev',       'acm.user@cimbniaga.co.id'),
    ('ACM-SPV',  'EMPACM02', 'acm.spv',  'ACM Supervisor Dev', 'acm.spv@cimbniaga.co.id')
) AS v(role, emp, username, full_name, email)
JOIN public.roles r ON r.role = v.role
ON CONFLICT (username) DO NOTHING;

INSERT INTO public.acm_areas (name, created_by)
SELECT 'Area ACM Dev', (SELECT id FROM public.users WHERE username = 'Yudha')
WHERE NOT EXISTS (SELECT 1 FROM public.acm_areas WHERE lower(name) = 'area acm dev' AND is_active);

INSERT INTO public.acm_area_regions (acm_area_id, region_code, created_by)
SELECT DISTINCT a.id, vb.region_code::text, a.created_by
FROM public.acm_areas a
JOIN public.vendor_branches vb ON vb.is_active AND vb.region_code IS NOT NULL
WHERE lower(a.name) = 'area acm dev' AND a.is_active
ON CONFLICT DO NOTHING;  -- region already in another area stays there

INSERT INTO public.acm_area_members (acm_area_id, user_id, created_by)
SELECT a.id, u.id, a.created_by
FROM public.acm_areas a
JOIN public.users u ON u.username IN ('acm.user', 'acm.spv')
WHERE lower(a.name) = 'area acm dev' AND a.is_active
ON CONFLICT DO NOTHING;

-- What the Area ACM admin service does after a region mapping (acm_area_admin.go):
-- create the missing draft plan (one per request) for requests already waiting in vault_assignment + audit each.
-- Same SELECT as queries/vault_allocations.sql CreateMissingVaultPlansByRegion. No in-app notification (dev seed).
WITH created AS (
    INSERT INTO public.vendor_request_vault_plans (vendor_request_id, acm_area_id)
    SELECT vr.id, ar.acm_area_id
    FROM public.vendor_requests vr
    JOIN public.acm_area_regions ar ON ar.region_code = vr.region_code
    JOIN public.acm_areas a ON a.id = ar.acm_area_id AND a.is_active
    WHERE vr.status = 'vault_assignment'
      AND NOT EXISTS (SELECT 1 FROM public.vendor_request_vault_plans p
                      WHERE p.vendor_request_id = vr.id AND p.status <> 'cancelled')
    ON CONFLICT DO NOTHING
    RETURNING id, vendor_request_id, acm_area_id
)
INSERT INTO public.audit_logs (actor_id, action, entity_type, entity_id, after)
SELECT (SELECT id FROM public.users WHERE username = 'Yudha'), 'vault_plan_created', 'vendor_request_vault_plan', c.id,
       jsonb_build_object('vendor_request_id', c.vendor_request_id, 'acm_area_id', c.acm_area_id, 'status', 'draft')
FROM created c;

COMMIT;
