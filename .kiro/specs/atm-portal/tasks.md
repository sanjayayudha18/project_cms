# Implementation Plan: ATM Portal

## Overview

Build a read-heavy, server-driven monitoring table that joins `atms` master data with the latest `itm_cashpos` transactional record per terminal, computes a replenishment status in SQL, and renders it as a filterable/sortable/paginated table in `CompanyPortal-Vite`. No state machine, no write path — this is pure query + presentation.

**Correction to design.md before starting:** the design doc's "Schema Gap" section (Key Design Decision #4) and column-name mismatch claim (#5) are both stale. Verified against `backend/migrations/008_seed_atms.sql`:
- `atms.brand` and `atms.deployment_type` already exist and are seeded — **no new migration is needed**.
- The threshold columns are already named `low_threshold_amount` / `critical_threshold_amount`, matching `requirements.md` exactly — **no column mapping needed** in the service layer.

Task 1 below replaces the design doc's proposed migration task with a verification step instead.

## Tasks

- [x] 1. Schema verification (replaces design.md's proposed migration)
  - [x] 1.1 Verify `atms`, `locations`, `regions` table columns against requirements
    - Confirm `terminal_id`, `machine_type`, `brand`, `deployment_type`, `low_threshold_amount`, `critical_threshold_amount`, `is_active`, `deleted_at`, `location_id` (FK) all exist on `atms` (per `008_seed_atms.sql`)
    - **Correction:** `location_name`, `address`, and `region` are NOT columns on `atms` — verified live against the running `cms` database. `atms` only carries `location_id`. These fields live on `locations` (`name`, `address_line1`, `address_line2`, `region_id`) and `regions` (`region`), reached via `atms.location_id → locations.id → locations.region_id → regions.id`
    - Address composition: `address_line1` only (confirmed — `address_line2` is not surfaced in the response)
    - Confirm `itm_cashpos` has `terminal_id`, `replenish_date`, `replenish_time`, `refund_total`, `replenish_total`, `escrow` (per `009_itm_cashpos.sql`)
    - Confirm index support for the lateral join: `itm_cashpos_terminal_date_idx (terminal_id, replenish_date)` exists — note it does NOT include `replenish_time`, so document this as a known query-plan cost, not a blocker
    - No migration file created — do not add `0XX_add_brand_deployment_type.sql`
    - _Requirements: 1.9, 2.1, 2.5, 2.8_
    - _Model: Sonnet 5 — low. Mostly confirming facts already verified live against the `cms` database in this session; no design judgment needed._

- [ ] 2. Backend: sqlc queries
  - [x] 2.1 Write `backend/queries/atm_portal.sql` with `ListATMsWithCashPos`, `CountATMsWithCashPos`, `GetATMSummary`, `GetLastUpdated`
    - `JOIN locations l ON l.id = a.location_id` and `JOIN regions r ON r.id = l.region_id` for `location_name` (`l.name`), `address` (`l.address_line1` + nullable `l.address_line2`), and `region` (`r.region`) — these are NOT columns on `atms` directly (see Task 1.1 correction)
    - Use `LEFT JOIN LATERAL` on `itm_cashpos` ordered by `replenish_date DESC, replenish_time DESC LIMIT 1`
    - Compute `status` via `CASE` expression per Property 1 (unconfigured → no_data → critical → low → normal precedence), using `low_threshold_amount` / `critical_threshold_amount`
    - Filter `is_active = true AND deleted_at IS NULL` in every query; `GetATMSummary` does not need the `locations`/`regions` joins since it only aggregates status counts
    - Support dynamic filters (search ILIKE, status, machine_type, brand, deployment_type, region) and dynamic sort column/direction — build via sqlc conditional args or a query-builder helper since sqlc doesn't support fully dynamic ORDER BY natively
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.8, 1.9, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 9.8_
    - _Model: Sonnet 5 — high. Highest-risk task in the plan: two-table join, lateral join, precedence-ordered CASE logic, and fully dynamic filter/sort construction that sqlc doesn't natively support — get this wrong and every downstream task inherits the bug._
  - [x] 2.2 Run `sqlc generate` and verify generated code lands in `backend/internal/db/`
    - _Requirements: 9.8_
    - _Model: Sonnet 5 — low. Running a codegen tool and checking the output landed; no authorship._

- [ ] 3. Backend: service layer
  - [x] 3.1 Implement `backend/internal/service/atm_portal.go`
    - `ListATMsParams` struct with `validator` tags matching design.md exactly (page >=1, page_size 1-100, status oneof, sort_by oneof, sort_order oneof)
    - `AtmPortalService.ListATMs(ctx, params) (*ListATMsResult, error)` — validates params, delegates to repository, assembles `Data`, `Summary`, `Total`, `Page`, `PageSize`, `LastUpdated`
    - Define `AtmPortalServicer` interface for handler injection (small interface, per Go patterns)
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.10, 9.7_
    - _Model: Sonnet 5 — medium. Validation-tag correctness matters but the shape is fully specified by design.md; low ambiguity._

- [ ] 4. Backend: HTTP handler
  - [x] 4.1 Implement `backend/internal/handler/atm_portal_handler.go`
    - `AtmPortalHandler.Routes()` returning `chi.Router` with `GET /atms` → `ListATMs`
    - Parse query params, on validation failure return 400 with descriptive Indonesian message via existing `writeError`/`writeValidationError` helpers (match `auth_handler.go` conventions)
    - Wire successful result to JSON response matching the API Contract in design.md
    - _Requirements: 1.10, 9.6_
    - _Model: Sonnet 5 — medium. Mirrors `auth_handler.go` conventions closely; mostly pattern-matching existing code._
  - [x] 4.2 Mount `/api/v1/atm-portal` routes in `backend/cmd/api/main.go`, protected by existing `RequireAuth` middleware
    - _Requirements: 1.1_
    - _Model: Sonnet 5 — low. One-line wiring, confirmed `RequireAuth` already exists at `internal/middleware/rbac.go:27`._

- [ ] 5. Backend: property and integration tests (pgregory.net/rapid, requires test DB)
  - [x] 5.1 Write property tests for status classification and latest-record selection (Properties 1, 2)
    - **Property 1: Replenishment status classification**
    - **Property 2: Latest record selection**
    - **Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8**
    - _Model: Sonnet 5 — high. Directly verifies Task 2.1's precedence logic against `pgregory.net/rapid`-generated inputs; needs to reason carefully about boundary cases (NULL thresholds, refund_total exactly equal to a threshold) to be a real check rather than a rubber stamp._
  - [x] 5.2 Write property tests for search, status filter, active-only, pagination, sort order (Properties 3, 4, 5, 6, 7)
    - **Property 3: Search filter correctness** — **Validates: Requirements 1.3**
    - **Property 4: Status filter correctness** — **Validates: Requirements 1.4**
    - **Property 5: Active-only invariant** — **Validates: Requirements 1.9**
    - **Property 6: Pagination correctness** — **Validates: Requirements 1.2**
    - **Property 7: Sort order correctness** — **Validates: Requirements 1.6**
    - _Model: Sonnet 5 — medium. Five properties but each is a mechanical invariant check (substring containment, filter equivalence, pagination arithmetic) once the generators are set up._
  - [x] 5.3 Write property test for summary independence from filters (Property 10)
    - **Property 10: Summary independence from filters**
    - **Validates: Requirements 3.4, 7.3**
    - _Model: Sonnet 5 — medium. Single, well-defined invariant (two differently-filtered requests, same summary object)._
  - [x] 5.4 Write integration tests: full handler→service→repository→Postgres round-trip, brand/deployment_type filtering, NULL threshold/refund_total edge cases
    - _Requirements: 1.1, 1.5, 5.3_
    - _Model: Sonnet 5 — medium. Uses the now-fixed `runMigrations` harness against the real `cms` DB; mostly assembling known-good fixtures and asserting response shape._

- [x] 6. Checkpoint - Backend API verified
  - Run `go build`, `go vet`, `golangci-lint run`, `go test ./... -race -cover` for the new package files
  - Manually hit `GET /api/v1/atm-portal/atms` against local Postgres and confirm response shape matches design.md contract
  - Ensure all tests pass, ask the user if questions arise.
  - _Model: Sonnet 5 — low. Running existing commands and comparing output against a spec; escalate only if a failure needs root-causing._

- [ ] 7. Frontend: types, constants, formatters
  - [x] 7.1 Create `src/features/atm-portal/types.ts` — `ReplenishmentStatus`, `AtmRecord`, `AtmSummary`, `AtmPortalResponse`, `AtmPortalParams` exactly per design.md
    - _Requirements: 9.4_
    - _Model: Sonnet 5 — low. Types are given verbatim in design.md; transcription work._
  - [x] 7.2 Create `src/features/atm-portal/constants.ts` — query key, stale time (2 min), status→badge config map
    - _Requirements: 9.3_
    - _Model: Sonnet 5 — low. Values are specified exactly (2 min stale time, status→badge mapping from Task 9.1)._
  - [x] 7.3 Create `src/features/atm-portal/lib/formatters.ts` — `formatRupiah` (dot-separated, "—" on null), Indonesian `dd MMM yyyy` date formatter (and `HH:mm` variant for freshness indicator)
    - _Requirements: 4.3, 4.4, 4.6, 7.2_
    - _Model: Sonnet 5 — medium. Locale formatting has easy-to-miss edge cases (null, zero, very large numbers) that Property 8/9 will catch, but worth care up front._
  - [x] 7.4 Write property tests for currency and date formatting (Properties 8, 9)
    - **Property 8: Currency formatting** — **Validates: Requirements 4.3, 4.4**
    - **Property 9: Date formatting** — **Validates: Requirements 4.6, 7.2**
    - _Model: Sonnet 5 — medium. Pure-function property tests with `fast-check`; straightforward once formatters exist, but boundary values (0, null, 10^12) need deliberate generators._

- [ ] 8. Frontend: data hook and URL state
  - [x] 8.1 Create `src/features/atm-portal/useAtmPortalData.ts` — TanStack Query hook, query key includes all params, `staleTime: 2min`, `placeholderData: keepPreviousData`, `refetchOnWindowFocus: true`
    - _Requirements: 1.1 through 1.10, 5.7, 8.3_
    - _Model: Sonnet 5 — medium. Query key composition and TanStack Query config are pattern-matchable against existing hooks in `src/features/*`._
  - [x] 8.2 Implement URL search-param sync via TanStack Router `useSearch()` — filters/sort/page live in URL, defaults omitted, `search` debounced 300ms before URL update
    - _Requirements: 5.2, 5.4_
    - _Model: Sonnet 5 — medium. Debounce + URL sync timing has real footguns (stale closures, sync loops); worth care but the pattern is well-established in React._
  - [x] 8.3 Write property test for URL-filter round-trip (Property 11)
    - **Property 11: URL-filter synchronization (round-trip)**
    - **Validates: Requirements 5.4**
    - _Model: Sonnet 5 — medium. Single round-trip invariant (serialize → parse → equal) with `fast-check`._

- [ ] 9. Frontend: presentational components
  - [x] 9.1 `components/StatusBadge.tsx` — icon + label per status (Critical/AlertTriangle/danger, Low/TrendingDown/warning, Normal/CheckCircle/success, Unconfigured/Settings/neutral, No Data/HelpCircle/neutral), text always visible (never color-only)
    - _Requirements: 4.5, 11.2_
    - _Model: Sonnet 5 — low. Small, fully enumerated icon/label/color mapping — no design decisions left open._
  - [x] 9.2 `components/SummaryCardsGrid.tsx` — Total/Critical/Low/Normal in that order, responsive 4/2/1 columns, `aria-label` per card with metric + value, skeleton variant (4 cards)
    - _Requirements: 3.3, 3.4, 8.1, 10.1, 11.3_
    - _Model: Sonnet 5 — low. Straightforward responsive grid, spec is explicit down to column counts per breakpoint._
  - [x] 9.3 `components/DataFreshnessIndicator.tsx` — "Data terakhir: dd MMM yyyy, HH:mm" or "Data terakhir: belum tersedia" when null
    - _Requirements: 7.1, 7.2, 7.4, 7.5_
    - _Model: Sonnet 5 — low. Two-branch text component built on Task 7.3's formatter._
  - [x] 9.4 `components/FilterBar.tsx` — search input (300ms debounce, max 100 chars, labeled "Cari berdasarkan Terminal ID atau lokasi"), status/machine_type/brand multi-selects, deployment_type select, active filter count + "Clear All", 44×44px touch targets below 768px
    - _Requirements: 5.1, 5.2, 5.3, 5.5, 10.4, 11.5_
    - _Model: Sonnet 5 — medium. Most stateful presentational component in the set — several controlled inputs, debounce, and a derived active-filter count to keep in sync._
  - [x] 9.5 `components/AtmTable.tsx` — semantic `<table>`/`<thead>`/`<tbody>`/`<th scope="col">` with `<caption>` or `aria-label`, monospace Terminal ID, right-aligned tabular-nums Rupiah cells, sortable headers with `aria-sort`, horizontal scroll container for overflow, skeleton (5 rows), empty state ("Tidak ada ATM yang sesuai filter" / "Coba ubah atau hapus filter untuk melihat data"), error state (icon + message + "Coba Lagi")
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.6, 4.7, 8.1, 8.2, 8.4, 10.2, 10.3, 11.1, 11.7_
    - _Model: Sonnet 5 — high. Most complex single component in the plan: five distinct render states (loading/loaded/empty/error/sorted), semantic-HTML + `aria-sort` correctness, and it's the component every other Task 9 item and Task 13.1's tests hang off of — a mistake here compounds._
  - [x] 9.6 `components/PaginationControls.tsx` — current page, total pages, page size selector [10, 25, 50, 100] default 25, `aria-label` on nav buttons (e.g. "Halaman 1 dari 10")
    - _Requirements: 4.8, 11.4_
    - _Model: Sonnet 5 — low. Fixed option set, simple prev/next + page-size logic._
  - [x] 9.7 ARIA live region for loading/loaded/error/empty state announcements (`aria-live="polite"`)
    - _Requirements: 11.8_
    - _Model: Sonnet 5 — low. Small, focused piece, but verify the announcement text actually changes on each state transition (a static `aria-live` region that never re-renders its text is a common a11y bug)._

- [ ] 10. Frontend: page composition
  - [x] 10.1 `AtmPortalScreen.tsx` — compose PageHeader ("ATM Portal" / "Monitor posisi kas dan status replenishment seluruh ATM"), DataFreshnessIndicator, SummaryCardsGrid, FilterBar, AtmTable, PaginationControls within existing `AppShell`
    - Loading state: skeleton cards + skeleton rows; Error state: retry preserves current filter selections; Retry button re-fetches via `queryClient.refetchQueries`
    - _Requirements: 3.1, 3.2, 3.5, 5.6, 5.7, 8.1, 8.2, 8.3, 8.5, 10.3_
    - _Model: Sonnet 5 — medium. Pure composition of already-built pieces, but the error-retry-preserves-filters requirement needs deliberate state wiring, not just JSX assembly._
  - [x] 10.2 `index.ts` barrel export
    - _Requirements: 9.5_
    - _Model: Sonnet 5 — low. Re-export statements only._
  - [x] 10.3 Register `/atm-portal` route (TanStack Router file-based route under `_protected`)
    - _Requirements: 3.1_
    - _Model: Sonnet 5 — low. Confirmed pattern already exists verbatim at `src/routes/replenishment.tsx`; copy the shape._

- [ ] 11. Navigation integration
  - [x] 11.1 Extend `src/lib/config/navigation.ts` — add `"monitoring"` to `NavGroup` type (after `"general"`), add to `GROUP_LABELS`, add ATM Portal nav item (`Monitor` icon, `/atm-portal`, roles `["ATM-USER", "ATM-SPV"]`, group `"monitoring"`)
    - Verify `Sidebar.tsx` renders the new group in the correct position automatically (it derives order from type/GROUP_LABELS — no Sidebar.tsx changes expected, confirm during task)
    - _Requirements: 6.1, 6.2, 6.3, 6.4_
    - _Model: Sonnet 5 — low. Additive edits to an existing, well-structured config file; design.md gives the exact diff._
  - [x] 11.2 Update/extend `navigation.test.ts` and `navigation.property.test.ts` for the new group and nav item
    - _Requirements: 6.1, 6.2_
    - _Model: Sonnet 5 — low. Extending existing test patterns for one more nav item._

- [x] 12. Checkpoint - Frontend feature verified end to end (tsc/test automated; live-backend nav/filter/sort/pagination verified live in browser after Docker/Redis started)
  - Run `pnpm tsc -b tsconfig.app.json`, `pnpm lint`, `pnpm test`
  - Manually navigate to `/atm-portal` in dev server, verify table renders, filters/sort/pagination work against the running backend
  - Ensure all tests pass, ask the user if questions arise.
  - _Model: Sonnet 5 — low. Running existing tooling and eyeballing the live page; escalate only if something fails and needs diagnosis._

- [ ] 13. Unit tests (example-based, per design.md Testing Strategy table)
  - [x] 13.1 Frontend component tests: PageHeader content, summary card order, badge icon/label per status, loading skeleton counts, error state content, retry triggers refetch, empty state message, pagination options, Sidebar group ordering, nav link + icon + navigation, semantic table structure
    - _Requirements: 3.2, 3.3, 4.5, 4.8, 6.1, 6.2, 6.3, 8.1, 8.2, 8.3, 8.4, 11.1_
    - _Model: Sonnet 5 — medium. Wide surface (11 requirements) but each assertion is a simple example-based check against already-built components — breadth, not depth._

- [ ] 14. Responsive and accessibility pass
  - [x] 14.1 Verify summary card grid breakpoints (4 col ≥1024px, 2 col 768–1023px, 1 col <768px), no horizontal page overflow from 320px up, table horizontal scroll container, 44×44px touch targets <768px
    - _Requirements: 10.1, 10.2, 10.3, 10.4_
    - _Model: Sonnet 5 — medium. Manual/visual verification across breakpoints; needs a real browser check (resize_window / preview), not just code reading._
  - [x] 14.2 Verify keyboard operability (Tab/Enter/Space), visible focus indicators, `aria-sort` updates on sort change, `aria-live` state announcements
    - _Requirements: 11.6, 11.7, 11.8_
    - _Model: Sonnet 5 — medium. Same as 14.1 — needs actual keyboard-driven interaction in a browser, not code inspection alone._

- [x] 15. Final checkpoint - All tests pass (`-race` unavailable — no CGO/C compiler in this environment, pre-existing limitation)
  - Run full backend (`go test ./... -race -cover`) and frontend (`pnpm test`, `pnpm tsc -b`, `pnpm lint`) suites
  - Confirm no regressions in existing Sidebar/navigation tests
  - Ensure all tests pass, ask the user if questions arise.
  - _Model: Sonnet 5 — low. Running full suites and confirming green; escalate only on an unexpected failure._

## Notes

- No new DB migration — corrects design.md's stale schema-gap assumption (see Task 1).
- Status computation stays in SQL (`CASE` expression), not post-processed in Go, per design.md Key Decision #2.
- Backend property tests use `pgregory.net/rapid` against a real test Postgres (query correctness, not pure-function logic) since the status logic lives in SQL.
- Frontend property tests use `fast-check`, minimum 100 iterations, per design.md Testing Strategy.
- All UI labels and messages are in Bahasa Indonesia, matching requirements.md acceptance criteria verbatim where quoted.
- Brand/Deployment Type filters are NOT deferred — the columns exist, so implement them in the same wave as Status/Machine Type filters (this also corrects design.md's "deferred — schema gap" note on those two FilterBar sub-items).
- **Model/effort recommendation:** every task can run on Sonnet 5 without switching models — nothing here needs Opus. Effort is annotated per-task above (`_Model:_` line) rather than fixed at one level for the whole plan, because the tasks are not uniformly complex:
  - **High effort:** 2.1 (dynamic SQL: joins + lateral join + precedence CASE + dynamic filter/sort), 5.1 (property tests directly verifying 2.1's precedence logic), 9.5 (`AtmTable` — five render states, `aria-sort`, everything else depends on it). Get these three right; they're where correctness risk concentrates.
  - **Medium effort:** service/handler layer, most property tests, the data hook + URL sync, `FilterBar`, page composition, unit tests, and the manual responsive/a11y passes (14.1–14.2, which need real browser interaction, not just code review).
  - **Low effort:** schema verification (already confirmed live in this session), codegen/wiring steps, checkpoints (running existing tooling), and the small fully-specified presentational components (`StatusBadge`, `SummaryCardsGrid`, `DataFreshnessIndicator`, `PaginationControls`, nav config).
  - Running the whole plan at a single "medium" setting would work but wastes effort on the low-complexity ~40% of tasks and under-invests in the three high-risk ones — the per-task split gets better quality where it matters without paying for it everywhere.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1"] },
    { "id": 2, "tasks": ["2.2"] },
    { "id": 3, "tasks": ["3.1"] },
    { "id": 4, "tasks": ["4.1"] },
    { "id": 5, "tasks": ["4.2"] },
    { "id": 6, "tasks": ["5.1", "5.2", "5.3", "5.4"] },
    { "id": 7, "tasks": ["6"] },
    { "id": 8, "tasks": ["7.1", "7.2", "7.3"] },
    { "id": 9, "tasks": ["7.4", "8.1"] },
    { "id": 10, "tasks": ["8.2"] },
    { "id": 11, "tasks": ["8.3", "9.1", "9.2", "9.3", "9.4", "9.6", "9.7"] },
    { "id": 12, "tasks": ["9.5"] },
    { "id": 13, "tasks": ["10.1"] },
    { "id": 14, "tasks": ["10.2", "10.3", "11.1"] },
    { "id": 15, "tasks": ["11.2"] },
    { "id": 16, "tasks": ["12"] },
    { "id": 17, "tasks": ["13.1"] },
    { "id": 18, "tasks": ["14.1", "14.2"] },
    { "id": 19, "tasks": ["15"] }
  ]
}
```
