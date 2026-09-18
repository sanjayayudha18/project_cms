-- Seed vendors.request_prefix for the six vendors seeded by
-- 005_seed_vendors.sql, per the Q1 mapping in
-- .kiro/specs/cit-vendor-request-enhancements/design.md and tasks.md Task 1.3.
--
-- WHY: TAG/ADV/BJK were requested directly (Req 4.2); ABA/ROH/SSI are the
-- deterministic derivation for vendors with no requested prefix. This is an
-- UPDATE of six known master-data rows, not a data migration of transactional
-- data, but is still flagged for confirmation (STOP-and-confirm, Task 1.5)
-- because it fixes the vendor -> prefix contract every future request number
-- depends on -- in particular 'ABA' for ABACUS, confirmed with the user
-- (2026-09-14) since it has no directly-requested value.
--
-- SAFETY:
--   * Idempotent: re-running sets the same six values again, no-op if
--     already applied.
--   * Only touches the six rows matched by `code`; any other vendor keeps
--     request_prefix NULL and falls back to the service-side deterministic
--     derivation (Q1) -- never left in an invalid state.

BEGIN;

UPDATE public.vendors SET request_prefix = 'TAG' WHERE code = 'TAG';
UPDATE public.vendors SET request_prefix = 'ADV' WHERE code = 'ADVANTAGE';
UPDATE public.vendors SET request_prefix = 'BJK' WHERE code = 'BIJAK';
UPDATE public.vendors SET request_prefix = 'ABA' WHERE code = 'ABACUS';
UPDATE public.vendors SET request_prefix = 'ROH' WHERE code = 'ROH';
UPDATE public.vendors SET request_prefix = 'SSI' WHERE code = 'SSI';

COMMIT;

-- ROLLBACK (documented, forward-only shop):
--   UPDATE public.vendors SET request_prefix = NULL
--   WHERE code IN ('TAG', 'ADVANTAGE', 'BIJAK', 'ABACUS', 'ROH', 'SSI');
