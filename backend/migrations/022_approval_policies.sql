-- approval_policies: maps (document_type, amount range) -> required approval level.
-- Threshold table for maker-checker routing (RBAC-Setup, internal scope).
--
-- WHY: "how high does approval climb" must be admin-configurable per document type
-- and value range, not hardcoded. approval_requests (Task 4) will look up this table
-- to derive required_level at submit time.
--
-- SAFETY:
--   * New table, no existing data affected.
--   * EXCLUDE USING gist prevents overlapping [min_amount, max_amount) ranges for the
--     same active document_type — a lookup by (document_type, amount) must resolve to
--     exactly one policy. Requires the btree_gist extension (enabled below, additive,
--     no effect on existing types/tables).
--   * CHECK min_amount < max_amount rejects an inverted/empty range at insert time.
--   * is_active lets admin retire a policy without deleting history; the exclusion
--     constraint only applies while is_active, so a retired policy's range can be
--     reused by a new one.

BEGIN;

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS public.approval_policies
(
    id bigserial NOT NULL,
    document_type text COLLATE pg_catalog."default" NOT NULL,
    min_amount numeric(20, 2) NOT NULL,
    max_amount numeric(20, 2) NOT NULL,
    required_level int NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT approval_policies_pkey PRIMARY KEY (id),
    CONSTRAINT approval_policies_amount_range_chk CHECK (min_amount < max_amount),
    CONSTRAINT approval_policies_no_overlap EXCLUDE USING gist (
        document_type WITH =,
        numrange(min_amount, max_amount, '[)') WITH &&
    ) WHERE (is_active)
);

COMMENT ON TABLE public.approval_policies
    IS 'Threshold config: (document_type, amount range) -> required_level for maker-checker approval routing.';

COMMENT ON COLUMN public.approval_policies.min_amount
    IS 'Inclusive lower bound of the amount range this policy covers.';

COMMENT ON COLUMN public.approval_policies.max_amount
    IS 'Exclusive upper bound of the amount range this policy covers.';

COMMENT ON COLUMN public.approval_policies.required_level
    IS 'Approval level (matches users.approval_level) the request must climb to when it falls in this range.';

CREATE INDEX IF NOT EXISTS approval_policies_document_type_idx
    ON public.approval_policies(document_type)
    WHERE is_active;

-- Seed example: invoice under 100jt requires L2, 100jt and above requires L3.
INSERT INTO public.approval_policies (document_type, min_amount, max_amount, required_level)
VALUES
    ('invoice', 0, 100000000, 2),
    ('invoice', 100000000, 999999999999999.99, 3)
ON CONFLICT DO NOTHING;

COMMIT;
