-- vendor_request_number_seq: DB-backed atomic per-scope sequence for the new
-- request-number generator, plus vendor_requests.vendor_id (the resolved
-- single vendor a request's number is derived from).
-- See .kiro/specs/cit-vendor-request-enhancements/design.md Q2/Q3 and
-- tasks.md Task 1.4.
--
-- WHY: the new REP<prefix><YYYYMMDD><seq> format needs one atomically
-- incrementing 001-999 sequence per (vendor, replenish_date) scope (Q3).
-- A dedicated table with an `INSERT ... ON CONFLICT DO UPDATE ... RETURNING`
-- upsert is a single atomic statement, stronger than the existing
-- MAX(SUBSTRING(...)) approach it replaces, which races under concurrent
-- creates. vendor_requests.vendor_id (Q2) persists the one vendor a request
-- is constrained to, so the number stays reproducible even if an ATM's
-- active vendor package changes later.
--
-- SAFETY:
--   * New table, no existing data affected.
--   * last_seq CHECK (0..999) makes sequence exhaustion a constraint
--     violation the service maps to an explicit error (Req 4.7), rather than
--     silently wrapping or producing a 4-digit value.
--   * vendor_requests.vendor_id is additive and nullable -- existing
--     'VR-...' rows have no resolved vendor and are not rewritten (Req 4.9).
--   * Both FKs to vendors(id) use ON DELETE RESTRICT, consistent with the
--     existing vendor_requests created_by/approved_by/rejected_by FKs
--     (028_vendor_requests.sql): a vendor referenced by a sequence scope or
--     a request must remain traceable, not silently orphaned.

BEGIN;

CREATE TABLE IF NOT EXISTS public.vendor_request_number_seq
(
    vendor_id   bigint NOT NULL,
    seq_date    date   NOT NULL,
    last_seq    int    NOT NULL DEFAULT 0,
    CONSTRAINT vendor_request_number_seq_pkey PRIMARY KEY (vendor_id, seq_date),
    CONSTRAINT vendor_request_number_seq_last_chk CHECK (last_seq >= 0 AND last_seq <= 999),
    CONSTRAINT vendor_request_number_seq_vendor_fk FOREIGN KEY (vendor_id)
        REFERENCES public.vendors (id) MATCH SIMPLE
        ON UPDATE NO ACTION ON DELETE RESTRICT
);

COMMENT ON TABLE public.vendor_request_number_seq
    IS 'Atomic per-(vendor, replenish_date) sequence backing the REP<prefix><YYYYMMDD><seq> request-number format (Req 4, Q3).';

ALTER TABLE public.vendor_requests
    ADD COLUMN IF NOT EXISTS vendor_id bigint;

ALTER TABLE public.vendor_requests
    DROP CONSTRAINT IF EXISTS vendor_requests_vendor_fk;

ALTER TABLE public.vendor_requests
    ADD CONSTRAINT vendor_requests_vendor_fk FOREIGN KEY (vendor_id)
        REFERENCES public.vendors (id) MATCH SIMPLE
        ON UPDATE NO ACTION ON DELETE RESTRICT;

COMMENT ON COLUMN public.vendor_requests.vendor_id
    IS 'Single resolved CIT vendor this request is constrained to (Req 4, Q2); drives the request-number vendor prefix. NULL only for legacy VR-... rows.';

CREATE INDEX IF NOT EXISTS vendor_requests_vendor_id_idx
    ON public.vendor_requests (vendor_id);

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   DROP INDEX IF EXISTS vendor_requests_vendor_id_idx;
--   ALTER TABLE public.vendor_requests DROP CONSTRAINT IF EXISTS vendor_requests_vendor_fk;
--   ALTER TABLE public.vendor_requests DROP COLUMN IF EXISTS vendor_id;
--   DROP TABLE IF EXISTS public.vendor_request_number_seq;
