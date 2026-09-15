-- CIT-2 enhancement columns on vendor_requests and vendor_request_items.
-- See .kiro/specs/cit-vendor-request-enhancements/design.md, migration
-- 034_vendor_requests_cit2_columns.sql, and tasks.md Task 1.1.
--
-- WHY: adds the soft-cancel flag (Req 5), the manual-request category +
-- vendor-facing replenish date (Req 2, 3), and the is_manual read-path
-- discriminator (Q4), plus manual-only context columns on line items so a
-- manual request (no DMAA row to re-derive from) stays reconstructable.
--
-- SAFETY:
--   * All columns additive (ADD COLUMN IF NOT EXISTS), no data migration of
--     existing rows.
--   * is_canceled / is_manual default false -- existing rows are neither
--     canceled nor manual.
--   * request_category / replenish_date are nullable with NO default --
--     existing rows predate categories and a chosen replenish date, and must
--     NOT be silently backfilled to a guessed value. Per design.md's
--     resolution of the Req 2 vs Req 3 scope question: request_category is a
--     Manual_Request-only concept (Req 3, is_manual = true); the standard,
--     non-manual Forecast-Browser create flow (Req 2) also leaves it NULL by
--     design, not just legacy rows.
--   * vendor_request_items.brand / lokasi_atm are nullable and populated only
--     for manual rows (Q4) -- DMAA-backed rows stay NULL and continue to
--     derive display values at read time from the existing join chain
--     (update-cit-forecast-browser), unchanged.
--   * request_category CHECK admits NULL so legacy and non-manual rows are
--     not forced into a category.

BEGIN;

ALTER TABLE public.vendor_requests
    ADD COLUMN IF NOT EXISTS is_canceled      boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS request_category text,
    ADD COLUMN IF NOT EXISTS replenish_date   date,
    ADD COLUMN IF NOT EXISTS is_manual        boolean NOT NULL DEFAULT false;

ALTER TABLE public.vendor_requests
    DROP CONSTRAINT IF EXISTS vendor_requests_category_chk;

ALTER TABLE public.vendor_requests
    ADD CONSTRAINT vendor_requests_category_chk
    CHECK (request_category IS NULL
        OR request_category IN ('planned', 'emergency', 'additional'));

COMMENT ON COLUMN public.vendor_requests.is_canceled
    IS 'Soft-cancel flag (Req 5); row/items are never deleted on cancel.';

COMMENT ON COLUMN public.vendor_requests.request_category
    IS 'Manual_Request classification: planned | emergency | additional. NULL for legacy rows AND for standard non-manual (Forecast Browser) requests -- see design.md Q-resolution.';

COMMENT ON COLUMN public.vendor_requests.replenish_date
    IS 'Vendor-facing delivery date chosen by the operator (Req 2/3), distinct from forecast_date. NULL only for legacy rows.';

COMMENT ON COLUMN public.vendor_requests.is_manual
    IS 'True when items were entered directly without a matching dmaa_atm_forecast row (Req 3).';

CREATE INDEX IF NOT EXISTS vendor_requests_is_canceled_idx
    ON public.vendor_requests (is_canceled);

CREATE INDEX IF NOT EXISTS vendor_requests_replenish_date_idx
    ON public.vendor_requests (replenish_date);

ALTER TABLE public.vendor_request_items
    ADD COLUMN IF NOT EXISTS brand      text,
    ADD COLUMN IF NOT EXISTS lokasi_atm text;

COMMENT ON COLUMN public.vendor_request_items.brand
    IS 'Manual-request brand (Req 3, Q4); NULL for DMAA-backed rows, which derive it at read time.';

COMMENT ON COLUMN public.vendor_request_items.lokasi_atm
    IS 'Manual-request ATM location (Req 3, Q4); NULL for DMAA-backed rows, which derive it at read time.';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.vendor_request_items DROP COLUMN IF EXISTS lokasi_atm;
--   ALTER TABLE public.vendor_request_items DROP COLUMN IF EXISTS brand;
--   DROP INDEX IF EXISTS vendor_requests_replenish_date_idx;
--   DROP INDEX IF EXISTS vendor_requests_is_canceled_idx;
--   ALTER TABLE public.vendor_requests DROP CONSTRAINT IF EXISTS vendor_requests_category_chk;
--   ALTER TABLE public.vendor_requests DROP COLUMN IF EXISTS is_manual;
--   ALTER TABLE public.vendor_requests DROP COLUMN IF EXISTS replenish_date;
--   ALTER TABLE public.vendor_requests DROP COLUMN IF EXISTS request_category;
--   ALTER TABLE public.vendor_requests DROP COLUMN IF EXISTS is_canceled;
