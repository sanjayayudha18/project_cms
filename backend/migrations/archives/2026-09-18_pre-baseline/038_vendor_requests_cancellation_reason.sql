-- vendor_requests.cancellation_reason: the reason text captured when a
-- request is canceled (Req 3.2/3.3/3.6/3.11).
-- See .kiro/specs/replenishment-request-enhancements/tasks.md Task 11,
-- Opsi B (user-confirmed 2026-09-15): expose cancellation_reason in the
-- list/detail response instead of leaving it reachable only via the
-- audit-log endpoint (Opsi A).
--
-- WHY: the reason was already written into audit_logs.after (Task 8) since
-- that never required a schema decision. A dedicated column lets the list/
-- detail response and the frontend read it directly, without a second
-- request to GET /{id}/audit-log for something as basic as "why was this
-- canceled".
--
-- SAFETY:
--   * Additive, nullable column, no default, no backfill -- every existing
--     row (including already-canceled ones, whose reason only ever lived in
--     audit_logs) reads back NULL, exactly like every other CIT-2/
--     replenishment-request-enhancements legacy-row column (request_category,
--     replenish_date, etc., migration 034).
--   * Forward-only: no down migration ships (see project convention, e.g.
--     035/037's "ROLLBACK (documented)" comments) -- rollback instructions
--     are documented here instead.

BEGIN;

ALTER TABLE public.vendor_requests
    ADD COLUMN IF NOT EXISTS cancellation_reason text;

COMMENT ON COLUMN public.vendor_requests.cancellation_reason
    IS 'Reason captured on cancel (Req 3.2/3.3). NULL for non-canceled rows and for rows canceled before this column existed (their reason lives only in audit_logs.after).';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.vendor_requests DROP COLUMN IF EXISTS cancellation_reason;
