-- Reject overlapping approval_delegations ranges for the same from_user_id.
-- Deferred here from migration 024 (RBAC-Setup Task 4's own comment): Task 8
-- owns the admin endpoints that create these rows, so the overlap guard
-- lands with them.
--
-- WHY: two overlapping delegations for the same absent approver would make
-- ResolveEffectiveApprover's "most recently started wins" tie-break
-- (queries/approval.sql's FindActiveDelegate) silently pick one arbitrarily.
-- Reject the ambiguity at insert time instead.
--
-- SAFETY:
--   * btree_gist is already enabled by migration 022 (approval_policies uses
--     the same EXCLUDE USING gist pattern) — no new extension needed.
--   * EXCLUDE, not a UNIQUE index: a plain unique index cannot express
--     "no overlapping ranges", only exact-duplicate rejection.
--   * No existing approval_delegations rows to conflict with — the table
--     was created empty in 024 and nothing in this codebase writes to it yet.

BEGIN;

ALTER TABLE public.approval_delegations
    ADD CONSTRAINT approval_delegations_no_overlap EXCLUDE USING gist (
        from_user_id WITH =,
        tstzrange(start_at, end_at, '[)') WITH &&
    );

COMMIT;
