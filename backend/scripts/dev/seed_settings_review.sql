-- DEV ONLY — not a migration. Seed for the settings-menu-review spec (.kiro/specs/settings-menu-review):
--   Bagian 1: 5 FLM vendors (DMY1..DMY5) + 1 internal unit (DMYROH), 2 branches each. No vault/ATM/package/PKS.
--   Bagian 2: 2 seed PICs per eligible vendor (vendor-wide) + 2 per active branch of an eligible vendor.
-- Idempotent: safe to re-run; existing rows are never changed. Written directly (no audit_logs) — dev data, not a user action.
-- Run: psql -v ON_ERROR_STOP=1 "postgres://…@localhost:5432/cms" -f backend/scripts/dev/seed_settings_review.sql
BEGIN;

-- Bagian 1 — vendor dummy (Req 4)
INSERT INTO public.vendors (code, name, legal_name, contact_email, kind, request_prefix, is_active)
SELECT s.code, s.name, s.name, lower(s.code) || '@vendor.dev.local', s.kind, s.prefix, true
FROM (VALUES
    ('DMY1',   'Dummy Vendor 1', 'FLM_VENDOR', 'DMA'),
    ('DMY2',   'Dummy Vendor 2', 'FLM_VENDOR', 'DMB'),
    ('DMY3',   'Dummy Vendor 3', 'FLM_VENDOR', 'DMC'),
    ('DMY4',   'Dummy Vendor 4', 'FLM_VENDOR', 'DMD'),
    ('DMY5',   'Dummy Vendor 5', 'FLM_VENDOR', 'DME'),
    ('DMYROH', 'Dummy ROH',      'INTERNAL',   NULL)
) AS s(code, name, kind, prefix)
ON CONFLICT (code) DO NOTHING;

-- 2 branches per dummy vendor. region_code has no FK (CHECK ^[A-Z0-9]{2,10}$ only): JKT/BDG are plain labels.
INSERT INTO public.vendor_branches (vendor_id, branch_code, branch_name, region_code, is_active)
SELECT v.id, v.code || '_' || b.suffix, b.branch_name, b.region_code, true
FROM public.vendors v
CROSS JOIN (VALUES
    ('001', 'Cabang Jakarta', 'JKT'),
    ('002', 'Cabang Bandung', 'BDG')
) AS b(suffix, branch_name, region_code)
WHERE v.code IN ('DMY1', 'DMY2', 'DMY3', 'DMY4', 'DMY5', 'DMYROH')
ON CONFLICT (branch_code) DO NOTHING;

-- Bagian 2 — PIC (Req 5). EligibleVendor = active, not soft-deleted, code not starting with 'IT'.
-- Insert-only; NOT EXISTS keyed on the @pic.dev.local marker so re-runs add nothing and real PICs are never touched.
-- Vendor-wide: 2 per EligibleVendor without a seed PIC at vendor level.
INSERT INTO public.vendor_pics (vendor_id, vendor_branch_id, name, "position", phone, email,
                                is_notification_recipient, is_active, roles, is_director)
SELECT v.id, NULL, v.name || ' PIC ' || n, 'PIC Vendor', '0812000000' || n,
       'pic' || n || '.' || lower(v.code) || '@pic.dev.local',
       false, true, ARRAY['replenish', 'vault', 'cashcount'], false
FROM public.vendors v CROSS JOIN generate_series(1, 2) n
WHERE v.is_active AND v.deleted_at IS NULL AND v.code NOT LIKE 'IT%'
  AND NOT EXISTS (SELECT 1 FROM public.vendor_pics p
                  WHERE p.vendor_id = v.id AND p.vendor_branch_id IS NULL
                    AND p.email LIKE '%@pic.dev.local');

-- Branch-level: 2 per active branch of an EligibleVendor without a seed PIC for that branch.
INSERT INTO public.vendor_pics (vendor_id, vendor_branch_id, name, "position", phone, email,
                                is_notification_recipient, is_active, roles, is_director)
SELECT v.id, b.id, b.branch_name || ' PIC ' || n, 'PIC Cabang', '0813000000' || n,
       'pic' || n || '.' || lower(b.branch_code) || '@pic.dev.local',
       false, true, ARRAY['replenish', 'vault', 'cashcount'], false
FROM public.vendor_branches b
JOIN public.vendors v ON v.id = b.vendor_id
CROSS JOIN generate_series(1, 2) n
WHERE b.is_active AND v.is_active AND v.deleted_at IS NULL AND v.code NOT LIKE 'IT%'
  AND NOT EXISTS (SELECT 1 FROM public.vendor_pics p
                  WHERE p.vendor_branch_id = b.id
                    AND p.email LIKE '%@pic.dev.local');

COMMIT;
