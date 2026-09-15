-- vendors.request_prefix: the 3-char vendor prefix embedded in the new
-- REP<prefix><YYYYMMDD><seq> request-number format (Req 4).
-- See .kiro/specs/cit-vendor-request-enhancements/design.md Q1 and
-- tasks.md Task 1.2.
--
-- WHY: a request number must encode its CIT vendor. A column on vendors
-- (rather than a hardcoded Go map) lets master-data admin set/override the
-- prefix per vendor; the service's deterministic fallback (Q1) covers any
-- vendor with a NULL prefix so a missing seed never produces a malformed
-- number.
--
-- SAFETY:
--   * Additive, nullable column -- no existing vendor row is affected until
--     migration 036 seeds the six known vendors.
--   * CHECK enforces exactly 3 uppercase A-Z when set, or NULL.

BEGIN;

ALTER TABLE public.vendors
    ADD COLUMN IF NOT EXISTS request_prefix char(3);

ALTER TABLE public.vendors
    DROP CONSTRAINT IF EXISTS vendors_request_prefix_chk;

ALTER TABLE public.vendors
    ADD CONSTRAINT vendors_request_prefix_chk
    CHECK (request_prefix IS NULL OR request_prefix ~ '^[A-Z]{3}$');

COMMENT ON COLUMN public.vendors.request_prefix
    IS '3-uppercase-char prefix embedded in vendor_requests.request_number (Req 4, Q1). NULL falls back to a deterministic service-side derivation from vendors.code.';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   ALTER TABLE public.vendors DROP CONSTRAINT IF EXISTS vendors_request_prefix_chk;
--   ALTER TABLE public.vendors DROP COLUMN IF EXISTS request_prefix;
