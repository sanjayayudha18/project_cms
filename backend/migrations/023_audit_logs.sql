-- audit_logs: append-only trail of who did what to which entity, before/after state.
-- Backs the project-wide non-negotiable "every state change writes audit_logs"
-- (RBAC-Setup Task 3; also the generic audit sink for other modules going forward).
--
-- WHY: retry_audit_logs (012) is domain-specific to the retry scheduler. There is no
-- generic audit table yet, even though audit_logs is already a pre-approved Core table
-- (project-context.md Sec 2).
--
-- SAFETY:
--   * New table, no existing data affected.
--   * Append-only by convention: only an INSERT query is ever written in queries/audit.sql
--     (Task 3 code) — no UPDATE/DELETE query is defined, so no code path can mutate or
--     remove a row. There is a single DB role in this project (no separate app role to
--     REVOKE UPDATE/DELETE from), so this is enforced at the application layer, not the
--     database layer; documented here so it isn't mistaken for an oversight.
--   * actor_id NOT NULL + FK to users(id): every audit entry has a known actor. System/
--     batch-originated audit entries are out of scope until a concrete need appears.
--   * before/after are jsonb, both nullable (e.g. a create has no "before", a delete-less
--     approval flow may have no "after" at a given step).

BEGIN;

CREATE TABLE IF NOT EXISTS public.audit_logs
(
    id bigserial NOT NULL,
    actor_id bigint NOT NULL,
    action text COLLATE pg_catalog."default" NOT NULL,
    entity_type text COLLATE pg_catalog."default" NOT NULL,
    entity_id bigint NOT NULL,
    before jsonb,
    after jsonb,
    ip text COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT audit_logs_pkey PRIMARY KEY (id),
    CONSTRAINT audit_logs_actor_fk FOREIGN KEY (actor_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON TABLE public.audit_logs
    IS 'Append-only audit trail: who (actor_id) did what (action) to which entity, before/after state, from where (ip). No update/delete query exists in code.';

COMMENT ON COLUMN public.audit_logs.action
    IS 'Free-form verb, e.g. submit | approve | reject | create | update.';

COMMENT ON COLUMN public.audit_logs.entity_type
    IS 'Domain entity the action targets, e.g. approval_request, invoice.';

CREATE INDEX IF NOT EXISTS audit_logs_entity_idx
    ON public.audit_logs(entity_type, entity_id);

CREATE INDEX IF NOT EXISTS audit_logs_actor_idx
    ON public.audit_logs(actor_id);

COMMIT;
