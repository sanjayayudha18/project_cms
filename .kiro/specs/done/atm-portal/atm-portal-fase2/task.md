# Implementation Plan: ATM Portal table switcher and ATM Cashpos view

## Outcome
The internal Company Portal ATM Portal lets an authenticated user choose `ATM Replenish` or `ATM Cashpos`. The existing replenish view remains unchanged in behavior, while the cashpos view reads paginated `itm_cashpos` rows and renders every column from the current schema.

## Scope
- In scope: a selector on the ATM Portal page, URL/query state for the selected view, a cashpos read API path following the existing ATM Portal route conventions, sqlc query/generated types, read-only service/handler mapping, and responsive/accessibility-aware table states.
- Out of scope: changing `itm_cashpos`/`itm_cashpos_files`, ingest behavior, ATM master data, replenishment calculations, writes, exports, approval/audit workflows, dark mode, or unrelated UI refactoring.

## Evidence Reviewed
- `frontend/CompanyPortal-Vite/src/features/atm-portal/AtmPortalScreen.tsx`: composes the current page, URL-owned state, query hook, filters, `AtmTable`, and pagination; current page is replenish-oriented.
- `frontend/CompanyPortal-Vite/src/features/atm-portal/components/AtmTable.tsx`: controlled semantic sortable table with loading/error/empty states and horizontal scrolling.
- `frontend/CompanyPortal-Vite/src/features/atm-portal/useAtmPortalData.ts`: TanStack Query hook calling `/atm-portal/atms` with URL-derived filters, sorting, and pagination.
- `frontend/CompanyPortal-Vite/src/features/atm-portal/useAtmPortalUrlState.ts`: Zod search schema, defaults, parsing, and URL navigation state.
- `frontend/CompanyPortal-Vite/src/features/atm-portal/types.ts`: current replenish response and request types.
- `frontend/CompanyPortal-Vite/src/features/atm-portal/__tests__/atm-portal-components.test.tsx`: existing table state, sorting, composition, retry, filter, and accessibility regression coverage.
- `backend/queries/atm_portal.sql`: `ListATMsWithCashPos`, `CountATMsWithCashPos`, summary, and latest replenish queries. The current list is the active-ATM view joined to the latest `itm_replenish` row; it is not a raw `itm_cashpos` listing.
- `backend/internal/service/atm_portal.go`: `AtmPortalRepository`, `AtmPortalServicer`, `ListATMs`, validation, and sqlc-row mapping.
- `backend/internal/handler/atm_portal_handler.go`: `GET /api/v1/atm-portal/atms`, parameter defaults, response mapping, and current error format.
- `backend/cmd/api/main.go`: mounts the protected ATM Portal router at `/api/v1/atm-portal` and currently constructs `db.New(dbPool)` from the primary pool only.
- `backend/internal/handler/atm_portal_integration_test.go`: build-tagged real-Postgres handler/service/repository round-trip tests mounted at `/api/v1/atm-portal`.
- `backend/migrations/011_itm_cashpos.sql`: source of truth for `itm_cashpos`; it has 23 columns listed below and no schema change is required.

## Requirements and Acceptance Criteria
- [x] The page provides an accessible selector with exactly `ATM Replenish` and `ATM Cashpos` choices.
- [x] Replenish is the default mode and existing `/atms` behavior, filters, sorting, pagination, summary, freshness, retry, and copy remain regression-safe.
- [x] Selecting Cashpos fetches only the cashpos data path and selecting Replenish fetches only the existing replenish path; the selected mode is shareable/back-forward navigable through the existing URL state mechanism.
- [x] Cashpos results are paginated and sorted using the established table controls where the selected mode supports them; switching modes resets only incompatible mode-specific state (at minimum page to 1) and does not silently change replenish query semantics.
- [x] Cashpos response rows include every `itm_cashpos` field: `id`, `file_id`, `cashpos_date`, `terminal_id`, `machine_type`, `teller_id`, `branch_code`, all 16 denomination amount fields, `position_source`, and `created_at`.
- [x] PostgreSQL `numeric` values are transported without floating-point precision loss and rendered as IDR/amount values with tabular, right-aligned formatting consistent with the existing portal.
- [x] Cashpos loading, empty, error, and retry states are visible and announced without breaking the current replenish states.
- [x] The protected endpoint enforces the same authenticated/RBAC boundary as the existing ATM Portal and returns the repository's established response/error shape; no write, approval, or audit action is introduced.
- [x] No migration or new table/column is added.

## Assumptions and Open Questions
- Assumption: `ATM Replenish` refers to the current `/api/v1/atm-portal/atms` response and `AtmTable`, despite the current backend query name `ListATMsWithCashPos`.
- Assumption: Cashpos uses the existing page-size options and generic search/date/sort affordances only where they map to real `itm_cashpos` columns; do not send replenish-only filters such as status, brand, deployment type, or region unless the existing contract is explicitly extended.
- Assumption: Display labels may be humanized, but JSON keys remain snake_case and all schema fields remain present.
- Question/approval needed: the repository currently has `DATABASE_REPLICA_URL` documented but `cmd/api/main.go` creates only a primary `dbPool`; confirm whether this feature should add/complete replica-pool wiring now, or whether the existing primary-only ATM Portal wiring is the approved interim convention. If replica wiring is approved, it must be shared infrastructure rather than a feature-local connection.
- Question/approval needed: confirm whether API numeric fields should follow the current `float64` response convention or be changed for cashpos to decimal strings. The project rule requires no floating-point money; decimal strings (or the repository's existing exact numeric representation) are the safer contract and may require frontend formatter updates.

## Architecture and Data Impact
- Modules/components affected: existing ATM Portal feature in `frontend/CompanyPortal-Vite/src/features/atm-portal`; existing ATM Portal service/handler/query surface in `backend/queries/atm_portal.sql`, `backend/internal/service/atm_portal.go`, and `backend/internal/handler/atm_portal_handler.go`; generated `backend/internal/db/atm_portal.sql.go` via sqlc.
- API or event contract: preserve `GET /api/v1/atm-portal/atms` for replenish. Add only a cashpos read route after confirming the established route naming (preferred minimal shape is a sibling under the same protected router, such as `/cashpos`, but do not implement a new public path until the contract is approved). Response should carry `data`, `total`, `page`, and `page_size` in the same successful list shape used by this handler; errors use `ErrorResponse` from `backend/internal/handler/error_response.go`.
- Database impact: none. Add a sqlc SELECT/count query against existing `public.itm_cashpos`; do not change `backend/migrations/011_itm_cashpos.sql`. Keep raw rows separate from summaries.
- Primary/replica routing: reads belong on the replica when the existing database abstraction supports it. Today the ATM Portal is wired to the primary (`db.New(dbPool)`); do not pretend replica routing exists. Resolve the approval above before changing `cmd/api/main.go` or database construction. Any read-after-write requirement does not apply to this read-only portal view.
- State and audit implications: no state change, maker-checker, audit log, or async job. File-ingest idempotency remains owned by the existing `itm_cashpos_files`/`import_jobs` flow and is not modified.

## ATM Cashpos fields and display order
Render every field from `public.itm_cashpos`, grouped in a wide, horizontally scrollable table:

1. **Technical/source identity**: `id` (integer, plain/tabular), `file_id` (integer, plain/tabular), `created_at` (UTC timestamp in API; display in the portal's established Asia/Jakarta presentation convention with an accessible full value).
2. **Snapshot identity**: `cashpos_date` (business date), `terminal_id` (monospace), `machine_type` (text), `teller_id` (text), `branch_code` (text), `position_source` (text/badge or text, never color-only).
3. **10K denomination**: `starting_cash_10k`, `cash_in_10k`, `cash_out_10k`, `cash_position_10k`.
4. **20K denomination**: `starting_cash_20k`, `cash_in_20k`, `cash_out_20k`, `cash_position_20k`.
5. **50K denomination**: `starting_cash_50k`, `cash_in_50k`, `cash_out_50k`, `cash_position_50k`.
6. **100K denomination**: `starting_cash_100k`, `cash_in_100k`, `cash_out_100k`, `cash_position_100k`.

All 16 denomination fields are `numeric(20,2)` and must be rendered as monetary values with IDR/amount formatting, right alignment, `tabular-nums`, and no float conversion. `cashpos_date` is a date, `created_at` is `timestamptz`; preserve UTC in the API. `id` and `file_id` are technical identifiers but must not be hidden because the request requires every schema field.

## Implementation Steps
### Phase 1: Contract and RED tests
1. **Confirm the mode/query contract and exact numeric representation.**
   - Files: `backend/internal/handler/atm_portal_handler.go`, `frontend/CompanyPortal-Vite/src/features/atm-portal/useAtmPortalUrlState.ts`, `frontend/CompanyPortal-Vite/src/features/atm-portal/types.ts`.
   - Change: Document and test the smallest sibling read route/response shape compatible with the existing handler; retain `/atms` unchanged. Decide whether cashpos amounts are decimal strings or another exact existing representation.
   - Depends on: approval of the two open questions above.
   - Verification: contract tests assert valid/invalid mode and pagination/sort inputs without changing replenish defaults.
   - Risk: High (API and numeric contract).
2. **Add failing backend tests for all-field cashpos listing.**
   - Files: `backend/internal/service/atm_portal_cashpos_test.go` (new co-located test if no closer existing symbol is suitable), `backend/internal/handler/atm_portal_integration_test.go`.
   - Change: RED tests for parameter validation, row mapping of all 23 fields, pagination/count consistency, empty results, database errors, and protected route behavior; seed a cashpos file/row in the existing rollback transaction without modifying migrations.
   - Depends on: Step 1.
   - Verification: targeted `go test` initially fails for the missing cashpos behavior.
   - Risk: Medium.
3. **Add failing frontend tests for selector and cashpos table.**
   - Files: `frontend/CompanyPortal-Vite/src/features/atm-portal/__tests__/atm-portal-components.test.tsx` or new focused tests beside the affected component; `useAtmPortalUrlState.property.test.ts` for URL round-trip coverage.
   - Change: RED tests for mode selection, query invocation, all-field column rendering, loading/empty/error/retry states, accessible labeling, mode switch page reset, and existing replenish regression.
   - Depends on: Step 1.
   - Verification: `pnpm test -- ...` targeted tests fail for the missing selector/cashpos view.
   - Risk: Medium.

### Phase 2: Query and backend implementation
4. **Add sqlc cashpos list/count queries.**
   - Files: `backend/queries/atm_portal.sql` (extend existing feature query file); generated `backend/internal/db/atm_portal.sql.go` (regenerate, do not hand-edit).
   - Change: Select every `itm_cashpos` column explicitly in the field order above, with validated allowlisted sort fields, date/search filters only as approved, deterministic tie-breaking, and page/page_size bounds supplied by the service. Add a matching count query. Never use `SELECT *` because the explicit list makes schema drift visible and preserves the all-field contract.
   - Depends on: Steps 1-3.
   - Verification: `sqlc generate`; targeted SQL/integration tests prove all columns and pagination.
   - Risk: High (numeric mapping and data exposure).
5. **Extend the ATM Portal service with cashpos types, validation, repository interface, and mapping.**
   - Files: `backend/internal/service/atm_portal.go` or a feature-local sibling such as `backend/internal/service/atm_portal_cashpos.go` if that matches current file-size conventions.
   - Change: Add a read-only cashpos result and repository methods; map PostgreSQL dates/timestamps and numeric values exactly; validate page/page_size, search/date/sort inputs and allowlist sort keys. Keep `ListATMs` and all current replenish validation/mapping intact.
   - Depends on: Step 4.
   - Verification: RED tests turn GREEN; assert malformed dates, reversed ranges, unsupported sorts, empty data, and conversion errors.
   - Risk: High (money precision).
6. **Expose the cashpos handler route and response mapping.**
   - Files: `backend/internal/handler/atm_portal_handler.go`; `backend/cmd/api/main.go` only if approved database-pool routing changes are required.
   - Change: Parse cashpos-specific query parameters, call the service, map the successful list to the established response convention, and map validation/database failures without leaking SQL details. Keep `RequireAuth` mounting around both modes and do not alter `/atms` behavior.
   - Depends on: Step 5 and route contract approval.
   - Verification: handler unit/integration tests cover 200, 400, 401/403 through the mounted middleware, 500, empty, and pagination responses.
   - Risk: High (authorization and contract compatibility).
7. **Resolve read-pool routing without feature-local invention.**
   - Files: `backend/cmd/api/main.go` and the actual existing database/config files discovered during implementation; no new environment variable.
   - Change: If approved and supported by existing infrastructure, create/use the configured replica pool for cashpos and reporting reads while preserving primary read-after-write semantics. Otherwise record that current primary-only routing is retained and make no unrelated pool refactor.
   - Depends on: the replica approval question.
   - Verification: repository/service tests assert the intended pool is used; startup/ping and `go test ./...` remain green.
   - Risk: High (topology and operational behavior).

### Phase 3: Frontend mode selector and views
8. **Extend URL-owned state with a table mode.**
   - Files: `frontend/CompanyPortal-Vite/src/features/atm-portal/useAtmPortalUrlState.ts`, `types.ts`, `constants.ts`.
   - Change: Add a validated mode with default `replenish`; omit the default from URLs. Preserve existing search/filter/sort/page parameters for replenish. Define the approved cashpos parameter subset and reset page to 1 on mode changes.
   - Depends on: Step 1.
   - Verification: property tests cover parse/serialize round trips, invalid mode fallback, back/forward-compatible state, and mode switching.
   - Risk: Medium.
9. **Split data fetching by selected mode while preserving query caching.**
   - Files: `frontend/CompanyPortal-Vite/src/features/atm-portal/useAtmPortalData.ts` and possibly a small cashpos hook/type module under the same feature.
   - Change: Keep the existing `/atms` query untouched for replenish; add the approved cashpos request/query key and response type. Include mode and all request params in the key; use existing `keepPreviousData`, retry, and stale-time conventions without showing stale replenish rows as cashpos rows.
   - Depends on: Steps 6 and 8.
   - Verification: hook tests/mock API assertions verify endpoint and query parameters for both modes.
   - Risk: Medium.
10. **Implement the accessible selector and conditional page composition.**
    - Files: `frontend/CompanyPortal-Vite/src/features/atm-portal/AtmPortalScreen.tsx`; new or existing presentational selector component under `frontend/CompanyPortal-Vite/src/features/atm-portal/components/`.
    - Change: Add a labeled native select or equivalent existing control with `ATM Replenish` and `ATM Cashpos`; render current `AtmTable` for replenish and a dedicated cashpos table for cashpos. Keep summary/freshness/filter controls only where their data semantics remain valid, and make retry target the active query.
    - Depends on: Steps 8-9.
    - Verification: component tests switch modes with keyboard, assert only the selected table is visible, preserve replenish rendering, and reset/preserve state as specified.
    - Risk: Medium.
11. **Implement the cashpos table with all columns and states.**
    - Files: new `frontend/CompanyPortal-Vite/src/features/atm-portal/components/AtmCashposTable.tsx`; extend `lib/formatters.ts` only if existing exact numeric/date helpers cannot serve the contract.
    - Change: Use semantic `<table>`, scoped headers, sortable controls only for backend-supported fields, horizontal scroll with a clear table label, column group/order above, skeleton rows, empty message, error/retry row, null-safe display, accessible text for technical IDs and timestamps. Use design tokens, no pure black/white, no color-only status.
    - Depends on: Step 9.
    - Verification: tests assert every field header/cell, formatting, `aria-sort`, loading/empty/error/retry, keyboard access, and narrow viewport usability.
    - Risk: Medium.
12. **Adjust mode-specific filters and pagination without regressing replenish.**
    - Files: `frontend/CompanyPortal-Vite/src/features/atm-portal/components/FilterBar.tsx`, `PaginationControls.tsx`, `AriaLiveRegion.tsx`, and `AtmPortalScreen.tsx` as needed.
    - Change: Reuse existing controls where semantics match; hide/disable replenish-only filters for cashpos rather than sending misleading queries. Ensure labels and live announcements name the active dataset (`ATM Replenish`/`ATM Cashpos`) and pagination remains keyboard accessible and responsive.
    - Depends on: Steps 8-11.
    - Verification: component tests cover empty/error messages for each mode and existing filter/sort/page regression tests remain green.
    - Risk: Medium.

### Phase 4: Integration and quality verification
13. **Regenerate, format, lint, and run backend/frontend tests.**
    - Files: all changed files above; generated sqlc output only through the repository command.
    - Change: Run the repository's existing generation and quality commands; fix implementation issues, not tests; keep coverage at least 80% for affected `internal/*` code.
    - Depends on: Steps 1-12.
    - Verification: `sqlc generate`; `go test ./...`; `golangci-lint run`; `pnpm test`; `pnpm lint`; `pnpm build`; use `task check` if available in `Taskfile.yml`.
    - Risk: Medium.
14. **Run integration/E2E regression checks.**
    - Files: `backend/internal/handler/atm_portal_integration_test.go`; the existing frontend Playwright test location discovered before implementation (TBD if no ATM Portal E2E exists).
    - Change: Verify authenticated page load, mode switching, cashpos all-field rendering, empty/error/retry behavior, URL sharing/back-forward, responsive horizontal scroll, and unchanged replenish flow against real API/Postgres where configured.
    - Depends on: Step 13.
    - Verification: repository Playwright command/configuration and `go test -tags=integration ./backend/internal/handler` (or the actual supported integration command; confirm before running).
    - Risk: High (cross-stack and access control).

## Testing and Verification
- Unit: service validation/mapping and frontend formatter/selector/table tests; table-driven cases for every numeric/date/identity field.
- Integration/API/DB: sqlc queries and handler round trips with real Postgres, including an `itm_cashpos_files` parent and cashpos row, pagination/count, empty results, malformed input, and database failure mapping.
- Frontend component: selector keyboard interaction, active-mode query selection, every cashpos header/value, formatting, loading/empty/error/retry, responsive overflow, and accessible names/live announcements.
- E2E: authenticated ATM Portal loads replenish by default, switches to cashpos, preserves/reloads URL mode, and returns to replenish without regression.
- Security and authorization: existing `RequireAuth`/RBAC coverage for both paths; verify no unauthenticated cashpos access and no vendor-scope bypass is introduced.
- Commands: `sqlc generate`; `go test ./...`; `golangci-lint run`; `pnpm test`; `pnpm lint`; `pnpm build`; `task check` if supported. Run integration/E2E commands only from the repository's configured build/test files.

## Risks and Mitigations
- **Numeric precision or accidental float conversion**: use exact PostgreSQL numeric/sqlc representation and decimal JSON strings unless an existing exact contract is approved; add round-trip tests with large/decimal values.
- **Replica lag or unavailable replica**: use the existing configured read abstraction, define the approved fallback/availability behavior, and never add ad hoc connections in the feature.
- **Breaking replenish behavior**: preserve `/atms`, `AtmTable`, existing URL defaults, filters, and tests; add mode branching around rather than inside the current replenish query semantics.
- **Exposing technical/source data**: require authentication/RBAC, select only the known schema columns, and avoid SQL/error detail leakage while still returning all requested fields to authorized users.
- **Very wide cashpos table on small screens**: use a labeled horizontal scroll region, stable column widths, keyboard-focusable controls, and test at narrow viewport; do not hide requested fields.
- **Schema drift**: explicit query columns plus sqlc generation and an all-field test make future migration changes visible.
- **Stale cross-mode UI data**: mode is part of URL state and TanStack Query keys; render only the active mode's response and reset incompatible pagination state.

## Definition of Done
- [x] Approved mode, endpoint, response, and numeric representation are documented and implemented.
- [x] No application migration/table/column changes were made.
- [x] Replenish remains the default and existing behavior/tests pass.
- [x] Cashpos renders all 23 `itm_cashpos` fields in the specified grouping/order.
- [x] Inputs are validated, protected by existing authentication/RBAC, and errors use current response conventions.
- [x] Read routing follows the approved primary/replica decision.
- [x] Loading, empty, error, retry, responsive, keyboard, and screen-reader behavior is covered.
- [x] Backend and frontend tests, lint, build, sqlc generation, and applicable integration/E2E checks pass with affected backend coverage at least 80%.
- [x] No unrelated refactor, dependency, dark-mode, or ingest change is included.

## Approved decisions (2026-08-21)
1. Route: `GET /api/v1/atm-portal/cashpos` (sibling of `/atms`)
2. Numerics: decimal strings in JSON (no float)
3. Pool: keep primary-only ATM Portal wiring (no replica refactor)

**IMPLEMENTED**
