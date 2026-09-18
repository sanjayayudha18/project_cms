-- Seed the APPACCESS role — the 10th internal role, sole authority for
-- account provisioning (set-initial-password, force first-login change) and
-- for configuring the RBAC CRUD-mapping/delegation (RBAC-Setup), distinct
-- from ADMIN (system admin) and ADMIN_PARAM (master-data admin).
--
-- WHY: .kiro/specs/Auth-Local-Lifecycle/task.md Resolved Decisions #1 — user
-- confirmed APPACCESS = a new role (not a reuse of ADMIN_PARAM). It must
-- exist as a real roles row before Task 6's RequireRoles("APPACCESS")
-- set-initial-password endpoint can ever match a real user. Already listed
-- as "proposed" in project-context.md Sec 2.
--
-- SAFETY:
--   * ON CONFLICT (role) DO NOTHING mirrors 003_seed_roles_users.sql's seed
--     style — safe to re-run, no-op if already seeded.
--   * Does not touch any existing role or user row. No user is assigned
--     this role by this migration; provisioning an APPACCESS user is a
--     separate, deliberate admin action outside this spec's scope.

BEGIN;

INSERT INTO public.roles (role, description) VALUES
    ('APPACCESS', 'Application access admin — sole authority for account provisioning (set/force initial password) and RBAC CRUD-mapping/delegation config')
ON CONFLICT (role) DO NOTHING;

COMMENT ON COLUMN public.roles.role
    IS 'ADMIN | ADMIN_PARAM | ATM-USER | ATM-SPV | BRANCH-USER | BRANCH-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | VENDOR-USER | APPACCESS';

COMMIT;
