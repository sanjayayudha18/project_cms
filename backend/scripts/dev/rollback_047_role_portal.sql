-- rollback_047_role_portal.sql
-- Undo backend/migrations/047_role_portal.sql (spec .kiro/specs/role-management-revamp/, task 2.2, Req 2.1).
-- DEV ONLY. Run by hand, with the user's OK:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -f backend/scripts/dev/rollback_047_role_portal.sql
-- Stop the new backend first: after this script the pre-047 backend (no roles.portal / party_scope, no vendor-* keys) matches the schema.
-- NOT reversed: the text fixes (roles.description em dash, locations en dash). They only repaired broken text (Req 1.1, 1.2).
-- NOT reversed: VENDOR-* roles created after 047 through the new screen stay in `roles`; delete them by hand if unwanted.
-- Roles and users from before 047 are untouched.

BEGIN;

-- 1. grants on the 3 vendor catalog keys (covers the initial D6 grants and any grant made later on the screen)
DELETE FROM public.role_permissions rp
USING public.menu_features mf
WHERE rp.menu_feature_id = mf.id
  AND mf.key IN ('vendor-dsr', 'vendor-orders', 'vendor-realizations');

-- 2. the 3 catalog rows (all are top-level menus, no children)
DELETE FROM public.menu_features
WHERE key IN ('vendor-dsr', 'vendor-orders', 'vendor-realizations');

-- 3. columns. DROP COLUMN also drops the CHECK constraints that use the column.
--    party_scope goes before portal: roles_party_scope_portal_chk uses both.
ALTER TABLE public.menu_features DROP COLUMN IF EXISTS portal;   -- menu_features_portal_chk
ALTER TABLE public.roles DROP COLUMN IF EXISTS party_scope;      -- roles_party_scope_values_chk, roles_party_scope_portal_chk
ALTER TABLE public.roles DROP COLUMN IF EXISTS portal;           -- roles_portal_chk, roles_portal_prefix_chk

-- migration bookkeeping: let `migrate up` apply 047 again
DELETE FROM public.schema_migrations WHERE version = '047_role_portal.sql';

COMMIT;
