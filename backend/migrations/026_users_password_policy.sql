-- Add local-password policy columns to users: expiry tracking, forced first-login
-- change, and per-account lockout after repeated failed logins.
--
-- WHY: auth_source='ldap'/'local_dev'-via-Entra users have their password policy
-- enforced by the external directory. auth_source='local'/'local_dev' users (bank-
-- sensitive accounts like ADMIN/APPACCESS, plus vendors before LDAP/Entra is wired)
-- have no directory to enforce 90-day expiry, 7-day warning, or 3x-fail lockout — so
-- CMS has to track and enforce it itself (.kiro/specs/Auth-Local-Lifecycle/task.md).
--
-- SAFETY:
--   * Additive, nullable/defaulted columns. No data rewrite, no backfill.
--   * password_changed_at is nullable: existing users (seeded ldap/local rows) have
--     no known change date. Treat NULL as "policy not yet evaluated" in service code,
--     not as "expired" — evaluation happens the first time a local-auth login runs
--     Task 3's expiry check.
--   * must_change_password / failed_login_attempts default to false/0 so every
--     existing row starts in the "not forced, not locked" state — no behavior change
--     for current logins until Task 3-4's service logic goes live.
--   * locked_until is nullable: NULL = not locked. No CHECK constraint tying it to
--     failed_login_attempts — that invariant (3 fails -> set locked_until) is
--     maintained by the service layer (Task 4), same pattern as users.deleted_at
--     soft-delete being an application-level convention, not a DB trigger.
--   * No CHECK constraint on auth_source: none exists in the live schema (the
--     archived 003_add_local_dev_auth migration was never applied), so there is
--     nothing to adjust here.
--   * No index added: every read is a single-row lookup by username/id (already
--     covered by existing unique/primary key indexes); no query yet scans these
--     columns across rows. Add one if/when an admin "list locked accounts" query
--     needs it.

BEGIN;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS password_changed_at timestamptz;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS must_change_password boolean NOT NULL DEFAULT false;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS failed_login_attempts int NOT NULL DEFAULT 0;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS locked_until timestamptz;

COMMENT ON COLUMN public.users.password_changed_at
    IS 'Local-password policy only (auth_source=local|local_dev): last password change, basis for the 90-day expiry / 7-day warning window. NULL = not yet evaluated. Ignored for auth_source=ldap.';

COMMENT ON COLUMN public.users.must_change_password
    IS 'Local-password policy only: true forces a password change on next login (set by APPACCESS set-initial-password, or after admin unlock). Ignored for auth_source=ldap.';

COMMENT ON COLUMN public.users.failed_login_attempts
    IS 'Local-password policy only: consecutive failed local-login attempts; resets to 0 on success. 3 -> account locked (see locked_until). Ignored for auth_source=ldap.';

COMMENT ON COLUMN public.users.locked_until
    IS 'Local-password policy only: account locked until this timestamp (30 min after the 3rd consecutive failed login), or NULL if not locked. Ignored for auth_source=ldap.';

COMMIT;
