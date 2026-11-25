-- rollback_046_permission_catalog_coverage.sql
-- Undo backend/migrations/046_permission_catalog_coverage.sql (spec .kiro/specs/permission-catalog-coverage/, task 1.2, Req 2.4).
-- DEV ONLY. Run by hand, with the user's OK:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -f backend/scripts/dev/rollback_046_permission_catalog_coverage.sql
-- Stop the new backend first: after this script the pre-046 backend (no assignable_roles, no catalog keys) matches the schema.
-- NOT reversed (046 gave no marker, so they cannot be told apart from older grants; ON CONFLICT DO NOTHING kept old rows):
--   * grants on pre-existing keys: parent menus replenish / monitoring / forecasting / settings and 'cash-flow'.
--   Remove them per role by hand if the dev DB never had them before 046.
-- Existing keys and role_permissions rows from before 046 are untouched. 046 never edited them (Req 1.2, 2.5).

BEGIN;

-- 1. grants on the 21 new keys (also covers the new menus approvals / exports)
DELETE FROM public.role_permissions rp
USING public.menu_features mf
WHERE rp.menu_feature_id = mf.id
  AND mf.key IN (
    'approvals', 'exports', 'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
    'monitoring.atm-portal', 'monitoring.dsr-late-report', 'monitoring.eod-monitoring', 'monitoring.audit-logs',
    'forecasting.dmaa-forecast', 'forecasting.holidays', 'settings.users', 'settings.vendors', 'settings.atms',
    'settings.regions', 'settings.pks-types', 'settings.master-data-io', 'settings.rbac-users',
    'settings.rbac-policies', 'settings.rbac-delegations', 'settings.rbac-leaves');

-- 2. the 21 catalog rows (children first is not needed: only the two new menus are parents, and they have no children)
DELETE FROM public.menu_features
WHERE key IN (
    'approvals', 'exports', 'replenish.forecast-browser', 'replenish.vendor-requests', 'replenish.schedule',
    'monitoring.atm-portal', 'monitoring.dsr-late-report', 'monitoring.eod-monitoring', 'monitoring.audit-logs',
    'forecasting.dmaa-forecast', 'forecasting.holidays', 'settings.users', 'settings.vendors', 'settings.atms',
    'settings.regions', 'settings.pks-types', 'settings.master-data-io', 'settings.rbac-users',
    'settings.rbac-policies', 'settings.rbac-delegations', 'settings.rbac-leaves');

-- 3. assignable_roles (drops the values set on old keys too)
ALTER TABLE public.menu_features DROP COLUMN IF EXISTS assignable_roles;

-- migration bookkeeping: let `migrate up` apply 046 again
DELETE FROM public.schema_migrations WHERE version = '046_permission_catalog_coverage.sql';

COMMIT;
