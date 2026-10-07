-- Runtime tables for multi-layer maker-checker approval (RBAC-Setup Task 4):
-- approval_requests (one per document), approval_steps (per-level trail),
-- approval_delegations (leave fallback), user_leaves.
--
-- WHY: approval_policies (022) only says how high a request must climb.
-- These tables are the actual state machine: one request per document,
-- one step row per level it must pass through, plus the two tables that
-- let a step route around an absent approver.
--
-- SAFETY:
--   * All four tables are new; no existing data affected.
--   * approval_requests: UNIQUE (document_type, document_id) makes "submit" idempotent —
--     a second submit for the same document is a conflict, not a duplicate row.
--   * approval_requests.status / approval_steps.status use text + CHECK, not a native
--     Postgres ENUM — consistent with the rest of this codebase (e.g. retry_audit_logs,
--     itm_cashpos_file use text + a documenting comment, no ENUM type anywhere in
--     migrations/). CHECK gives the same validation without ALTER TYPE friction later.
--   * approval_steps: UNIQUE (request_id, step_level) — one step row per level per
--     request, so the resolver (Task 5) can never generate duplicate steps.
--   * approval_delegations / user_leaves: CHECK start_at < end_at rejects an inverted
--     range at insert time. Overlap-range rejection (EXCLUDE) is deferred to Task 8,
--     which owns the admin endpoints that create these rows — out of scope here.
--   * approval_delegations: CHECK from_user_id <> to_user_id — cannot delegate to self.

BEGIN;

CREATE TABLE IF NOT EXISTS public.approval_requests
(
    id bigserial NOT NULL,
    maker_id bigint NOT NULL,
    document_type text COLLATE pg_catalog."default" NOT NULL,
    document_id bigint NOT NULL,
    amount numeric(20, 2) NOT NULL,
    required_level int NOT NULL,
    status text COLLATE pg_catalog."default" NOT NULL DEFAULT 'draft',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT approval_requests_pkey PRIMARY KEY (id),
    CONSTRAINT approval_requests_document_uq UNIQUE (document_type, document_id),
    CONSTRAINT approval_requests_status_chk CHECK (status IN ('draft', 'pending', 'approved', 'rejected')),
    CONSTRAINT approval_requests_maker_fk FOREIGN KEY (maker_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON TABLE public.approval_requests
    IS 'One row per document under approval. Idempotent per (document_type, document_id).';

COMMENT ON COLUMN public.approval_requests.status
    IS 'draft | pending | approved | rejected';

CREATE INDEX IF NOT EXISTS approval_requests_maker_idx
    ON public.approval_requests(maker_id);

CREATE INDEX IF NOT EXISTS approval_requests_status_idx
    ON public.approval_requests(status);

CREATE TABLE IF NOT EXISTS public.approval_steps
(
    id bigserial NOT NULL,
    request_id bigint NOT NULL,
    step_level int NOT NULL,
    assigned_approver_id bigint NOT NULL,
    acted_by_id bigint,
    status text COLLATE pg_catalog."default" NOT NULL DEFAULT 'pending',
    acted_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT approval_steps_pkey PRIMARY KEY (id),
    CONSTRAINT approval_steps_request_level_uq UNIQUE (request_id, step_level),
    CONSTRAINT approval_steps_status_chk CHECK (status IN ('pending', 'approved', 'rejected')),
    CONSTRAINT approval_steps_request_fk FOREIGN KEY (request_id)
        REFERENCES public.approval_requests (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT approval_steps_assigned_approver_fk FOREIGN KEY (assigned_approver_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT approval_steps_acted_by_fk FOREIGN KEY (acted_by_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON TABLE public.approval_steps
    IS 'One row per approval level a request must pass through. acted_by_id may differ from assigned_approver_id (delegate acted instead).';

COMMENT ON COLUMN public.approval_steps.status
    IS 'pending | approved | rejected';

COMMENT ON COLUMN public.approval_steps.acted_by_id
    IS 'Who actually acted on this step — the assigned approver, or their delegate if on leave. NULL until acted.';

CREATE INDEX IF NOT EXISTS approval_steps_request_idx
    ON public.approval_steps(request_id);

CREATE INDEX IF NOT EXISTS approval_steps_assigned_approver_idx
    ON public.approval_steps(assigned_approver_id);

CREATE TABLE IF NOT EXISTS public.approval_delegations
(
    id bigserial NOT NULL,
    from_user_id bigint NOT NULL,
    to_user_id bigint NOT NULL,
    start_at timestamp with time zone NOT NULL,
    end_at timestamp with time zone NOT NULL,
    reason text COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT approval_delegations_pkey PRIMARY KEY (id),
    CONSTRAINT approval_delegations_range_chk CHECK (start_at < end_at),
    CONSTRAINT approval_delegations_not_self_chk CHECK (from_user_id <> to_user_id),
    CONSTRAINT approval_delegations_from_fk FOREIGN KEY (from_user_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION,
    CONSTRAINT approval_delegations_to_fk FOREIGN KEY (to_user_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON TABLE public.approval_delegations
    IS 'Explicit fallback: while from_user_id cannot act (see user_leaves), to_user_id acts in their place. Overlap-range rejection is enforced at the admin endpoint layer (Task 8), not here.';

CREATE INDEX IF NOT EXISTS approval_delegations_from_idx
    ON public.approval_delegations(from_user_id);

CREATE INDEX IF NOT EXISTS approval_delegations_to_idx
    ON public.approval_delegations(to_user_id);

CREATE TABLE IF NOT EXISTS public.user_leaves
(
    id bigserial NOT NULL,
    user_id bigint NOT NULL,
    start_at timestamp with time zone NOT NULL,
    end_at timestamp with time zone NOT NULL,
    reason text COLLATE pg_catalog."default",
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT user_leaves_pkey PRIMARY KEY (id),
    CONSTRAINT user_leaves_range_chk CHECK (start_at < end_at),
    CONSTRAINT user_leaves_user_fk FOREIGN KEY (user_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
);

COMMENT ON TABLE public.user_leaves
    IS 'Explicit unavailability windows. The resolver (Task 5) checks for an overlap with now() to decide whether an approver is on leave.';

CREATE INDEX IF NOT EXISTS user_leaves_user_idx
    ON public.user_leaves(user_id);

COMMIT;
