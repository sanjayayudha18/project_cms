-- Add users.supervisor_id and users.approval_level to model the reporting-line
-- hierarchy used for maker-checker approval routing (RBAC-Setup, internal scope).
--
-- WHY: RBAC role strings (roles.role) model capability, not "who reports to whom".
-- Approval routing needs to walk a chain of supervisors up to a required level
-- (see approval_policies, proposed separately). Neither exists on users today.
--
-- SAFETY:
--   * Additive, nullable columns. No data rewrite, no default backfill.
--   * NULLable on purpose: not every user has a supervisor (top of the tree) or a
--     defined approval_level yet (existing 9 seeded users predate this feature).
--   * supervisor_id is a self-FK on users(id): a pure tree, one supervisor per user.
--     CHECK prevents a user being their own supervisor; the FK does not by itself
--     prevent longer cycles (A -> B -> A) or multi-level cycles — those are the
--     application's responsibility when wiring the hierarchy (Task 8, admin-only).
--   * Index on supervisor_id mirrors the users_vendor_idx / users_role_idx convention
--     for the recursive "who reports up to me" / "walk my chain" queries.

BEGIN;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS supervisor_id bigint;

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS approval_level int;

COMMENT ON COLUMN public.users.supervisor_id
    IS 'Direct supervisor in the reporting line (self-FK, tree). NULL = top of hierarchy or not yet assigned.';

COMMENT ON COLUMN public.users.approval_level
    IS 'Explicit approval level for maker-checker routing, independent of role_id. NULL = not an approver.';

ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_supervisor_not_self_chk;

ALTER TABLE public.users
    ADD CONSTRAINT users_supervisor_not_self_chk CHECK (supervisor_id IS DISTINCT FROM id);

ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_supervisor_fk;

ALTER TABLE public.users
    ADD CONSTRAINT users_supervisor_fk FOREIGN KEY (supervisor_id)
    REFERENCES public.users (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE NO ACTION;

CREATE INDEX IF NOT EXISTS users_supervisor_idx
    ON public.users(supervisor_id);

COMMIT;
