# Implementation Plan: ATM Portal

## Overview

Build a read-heavy, server-driven monitoring table that joins `atms` master data with the latest `itm_cashpos` transactional record per terminal, computes a replenishment status in SQL, and renders it as a filterable/sortable/paginated table in `CompanyPortal-Vite`. No state machine, no write path — this is pure query + presentation.

**Correction to design.md before starting:** the design doc's "Schema Gap" section (Key Design Decision #4) and column-name mismatch claim (#5) are both stale. Verified against `backend/migrations/008_seed_atms.sql`:
- `atms.brand` and `atms.deployment_type` already exist and are seeded — **no new migration is needed**.
- The threshold columns are already named `low_threshold_amount` / `critical_threshold_amount`, matching `requirements.md` exactly — **no column mapping needed** in the service layer.

Task 1 below replaces the design doc's proposed migration task with a verification step instead.

## Tasks

- [ ] 1. Schema verification (replaces design.md's proposed migration)
  - [ ] 1.1 Verify `atms` table columns against requirements
    - Confirm `terminal_id`, `location_name`, `address`, `machine_type`, `brand`, `deployment_type`, `low_threshold_amount`, `critical_threshold_amount`, `is_active`, `deleted_at` all exist on `atms` (per `008_seed_atms.sql`)
    - Confirm `itm_cashpos` has `terminal_id`, `replenish_date`, `replenish_time`, `refund_total`, `replenish_total`, `escrow` (per `009_itm_cashpos.sql`)
    - Confirm index support for the lateral join: `itm_cashpos_terminal_date_idx (terminal_id, replenish_date)` exists — note it does NOT include `replenish_time`, so document this as a known query-plan cost, not a blocker
    - No migration file created — do not add `0XX_add_brand_deployment_type.sql`
    - _Requirements: 1.9, 2.1, 2.5, 2.8_

- [ ] 2. Backend: sqlc queries
  - [ ] 2.1 Write `backend/queries/atm_portal.sql` with `ListATMsWithCashPos`, `CountATMsWithCashPos`, `GetATMSummary`, `GetLastUpdated`
    - Use `LEFT JOIN LATERAL` on `itm_cashpos` ordered by `replenish_date DESC, replenish_time DESC LIMIT 1`
    - Compute `status` via `CASE` expression per Property 1 (unconfigured → no_data → critical → low → normal precedence)
    - Filter `is_active = true AND deleted_at IS NULL` in every query
    - Support dynamic filters (search ILIKE, status, machine_type, brand, deployment_type, region) and dynamic sort column/direction — build via sqlc conditional args or a query-builder helper since sqlc doesn't support fully dynamic ORDER BY natively
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.8, 1.9, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 9.8_
  - [ ] 2.2 Run `sqlc generate` and verify generated code lands in `backend/internal/db/`
    - _Requirements: 9.8_

- [ ] 3. Backend: service layer
  - [ ] 3.1 Implement `backend/internal/service/atm_portal.go`
    - `ListATMsParams` struct with `validator` tags matching design.md exactly (page >=1, page_size 1-100, status oneof, sort_by oneof, sort_order oneof)
    - `AtmPortalService.ListATMs(ctx, params) (*ListATMsResult, error)` — validates params, delegates to repository, assembles `Data`, `Summary`, `Total`, `Page`, `PageSize`, `LastUpdated`
    - Define `AtmPortalServicer` interface for handler injection (small interface, per Go patterns)
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.10, 9.7_

- [ ] 4. Backend: HTTP handler
  - [ ] 4.1 Implement `backend/internal/handler/atm_portal_handler.go`
    - `AtmPortalHandler.Routes()` returning `chi.Router` with `GET /atms` → `ListATMs`
    - Parse query params, on validation failure return 400 with descriptive Indonesian message via existing `writeError`/`writeValidationError` helpers (match `auth_handler.go` conventions)
    - Wire successful result to JSON response matching the API Contract in design.md
    - _Requirements: 1.10, 9.6_
  - [ ] 4.2 Mount `/api/v1/atm-portal` routes in `backend/cmd/api/main.go`, protected by existing `RequireAuth` middleware
    - _Requirements: 1.1_

- [ ] 5. Backend: property and integration tests (pgregory.net/rapid, requires test DB)
  - [ ] 5.1 Write property tests for status classification and latest-record selection (Properties 1, 2)
    - **Property 1: Replenishment status classification**
    - **Property 2: Latest record selection**
    - **Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8**
  - [ ] 5.2 Write property tests for search, status filter, active-only, pagination, sort order (Properties 3, 4, 5, 6, 7)
    - **Property 3: Search filter correctness** — **Validates: Requirements 1.3**
    - **Property 4: Status filter correctness** — **Validates: Requirements 1.4**
    - **Property 5: Active-only invariant** — **Validates: Requirements 1.9**
    - **Property 6: Pagination correctness** — **Validates: Requirements 1.2**
    - **Property 7: Sort order correctness** — **Validates: Requirements 1.6**
  - [ ] 5.3 Write property test for summary independence from filters (Property 10)
    - **Property 10: Summary independence from filters**
    - **Validates: Requirements 3.4, 7.3**
  - [ ] 5.4 Write integration tests: full handler→service→repository→Postgres round-trip, brand/deployment_type filtering, NULL threshold/refund_total edge cases
    - _Requirements: 1.1, 1.5, 5.3_

- [ ] 6. Checkpoint - Backend API verified
  - Run `go build`, `go vet`, `golangci-lint run`, `go test ./... -race -cover` for the new package files
  - Manually hit `GET /api/v1/atm-portal/atms` against local Postgres and confirm response shape matches design.md contract
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 7. Frontend: types, constants, formatters
  - [ ] 7.1 Create `src/features/atm-portal/types.ts` — `ReplenishmentStatus`, `AtmRecord`, `AtmSummary`, `AtmPortalResponse`, `AtmPortalParams` exactly per design.md
    - _Requirements: 9.4_
  - [ ] 7.2 Create `src/features/atm-portal/constants.ts` — query key, stale time (2 min), status→badge config map
    - _Requirements: 9.3_
  - [ ] 7.3 Create `src/features/atm-portal/lib/formatters.ts` — `formatRupiah` (dot-separated, "—" on null), Indonesian `dd MMM yyyy` date formatter (and `HH:mm` variant for freshness indicator)
    - _Requirements: 4.3, 4.4, 4.6, 7.2_
  - [ ] 7.4 Write property tests for currency and date formatting (Properties 8, 9)
    - **Property 8: Currency formatting** — **Validates: Requirements 4.3, 4.4**
    - **Property 9: Date formatting** — **Validates: Requirements 4.6, 7.2**

- [ ] 8. Frontend: data hook and URL state
  - [ ] 8.1 Create `src/features/atm-portal/useAtmPortalData.ts` — TanStack Query hook, query key includes all params, `staleTime: 2min`, `placeholderData: keepPreviousData`, `refetchOnWindowFocus: true`
    - _Requirements: 1.1 through 1.10, 5.7, 8.3_
  - [ ] 8.2 Implement URL search-param sync via TanStack Router `useSearch()` — filters/sort/page live in URL, defaults omitted, `search` debounced 300ms before URL update
    - _Requirements: 5.2, 5.4_
  - [ ] 8.3 Write property test for URL-filter round-trip (Property 11)
    - **Property 11: URL-filter synchronization (round-trip)**
    - **Validates: Requirements 5.4**

- [ ] 9. Frontend: presentational components
  - [ ] 9.1 `components/StatusBadge.tsx` — icon + label per status (Critical/AlertTriangle/danger, Low/TrendingDown/warning, Normal/CheckCircle/success, Unconfigured/Settings/neutral, No Data/HelpCircle/neutral), text always visible (never color-only)
    - _Requirements: 4.5, 11.2_
  - [ ] 9.2 `components/SummaryCardsGrid.tsx` — Total/Critical/Low/Normal in that order, responsive 4/2/1 columns, `aria-label` per card with metric + value, skeleton variant (4 cards)
    - _Requirements: 3.3, 3.4, 8.1, 10.1, 11.3_
  - [ ] 9.3 `components/DataFreshnessIndicator.tsx` — "Data terakhir: dd MMM yyyy, HH:mm" or "Data terakhir: belum tersedia" when null
    - _Requirements: 7.1, 7.2, 7.4, 7.5_
  - [ ] 9.4 `components/FilterBar.tsx` — search input (300ms debounce, max 100 chars, labeled "Cari berdasarkan Terminal ID atau lokasi"), status/machine_type/brand multi-selects, deployment_type select, active filter count + "Clear All", 44×44px touch targets below 768px
    - _Requirements: 5.1, 5.2, 5.3, 5.5, 10.4, 11.5_
  - [ ] 9.5 `components/AtmTable.tsx` — semantic `<table>`/`<thead>`/`<tbody>`/`<th scope="col">` with `<caption>` or `aria-label`, monospace Terminal ID, right-aligned tabular-nums Rupiah cells, sortable headers with `aria-sort`, horizontal scroll container for overflow, skeleton (5 rows), empty state ("Tidak ada ATM yang sesuai filter" / "Coba ubah atau hapus filter untuk melihat data"), error state (icon + message + "Coba Lagi")
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.6, 4.7, 8.1, 8.2, 8.4, 10.2, 10.3, 11.1, 11.7_
  - [ ] 9.6 `components/PaginationControls.tsx` — current page, total pages, page size selector [10, 25, 50, 100] default 25, `aria-label` on nav buttons (e.g. "Halaman 1 dari 10")
    - _Requirements: 4.8, 11.4_
  - [ ] 9.7 ARIA live region for loading/loaded/error/empty state announcements (`aria-live="polite"`)
    - _Requirements: 11.8_

- [ ] 10. Frontend: page composition
  - [ ] 10.1 `AtmPortalScreen.tsx` — compose PageHeader ("ATM Portal" / "Monitor posisi kas dan status replenishment seluruh ATM"), DataFreshnessIndicator, SummaryCardsGrid, FilterBar, AtmTable, PaginationControls within existing `AppShell`
    - Loading state: skeleton cards + skeleton rows; Error state: retry preserves current filter selections; Retry button re-fetches via `queryClient.refetchQueries`
    - _Requirements: 3.1, 3.2, 3.5, 5.6, 5.7, 8.1, 8.2, 8.3, 8.5, 10.3_
  - [ ] 10.2 `index.ts` barrel export
    - _Requirements: 9.5_
  - [ ] 10.3 Register `/atm-portal` route (TanStack Router file-based route under `_protected`)
    - _Requirements: 3.1_

- [ ] 11. Navigation integration
  - [ ] 11.1 Extend `src/lib/config/navigation.ts` — add `"monitoring"` to `NavGroup` type (after `"general"`), add to `GROUP_LABELS`, add ATM Portal nav item (`Monitor` icon, `/atm-portal`, roles `["ATM-USER", "ATM-SPV"]`, group `"monitoring"`)
    - Verify `Sidebar.tsx` renders the new group in the correct position automatically (it derives order from type/GROUP_LABELS — no Sidebar.tsx changes expected, confirm during task)
    - _Requirements: 6.1, 6.2, 6.3, 6.4_
  - [ ] 11.2 Update/extend `navigation.test.ts` and `navigation.property.test.ts` for the new group and nav item
    - _Requirements: 6.1, 6.2_

- [ ] 12. Checkpoint - Frontend feature verified end to end
  - Run `pnpm tsc -b tsconfig.app.json`, `pnpm lint`, `pnpm test`
  - Manually navigate to `/atm-portal` in dev server, verify table renders, filters/sort/pagination work against the running backend
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 13. Unit tests (example-based, per design.md Testing Strategy table)
  - [ ] 13.1 Frontend component tests: PageHeader content, summary card order, badge icon/label per status, loading skeleton counts, error state content, retry triggers refetch, empty state message, pagination options, Sidebar group ordering, nav link + icon + navigation, semantic table structure
    - _Requirements: 3.2, 3.3, 4.5, 4.8, 6.1, 6.2, 6.3, 8.1, 8.2, 8.3, 8.4, 11.1_

- [ ] 14. Responsive and accessibility pass
  - [ ] 14.1 Verify summary card grid breakpoints (4 col ≥1024px, 2 col 768–1023px, 1 col <768px), no horizontal page overflow from 320px up, table horizontal scroll container, 44×44px touch targets <768px
    - _Requirements: 10.1, 10.2, 10.3, 10.4_
  - [ ] 14.2 Verify keyboard operability (Tab/Enter/Space), visible focus indicators, `aria-sort` updates on sort change, `aria-live` state announcements
    - _Requirements: 11.6, 11.7, 11.8_

- [ ] 15. Final checkpoint - All tests pass
  - Run full backend (`go test ./... -race -cover`) and frontend (`pnpm test`, `pnpm tsc -b`, `pnpm lint`) suites
  - Confirm no regressions in existing Sidebar/navigation tests
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- No new DB migration — corrects design.md's stale schema-gap assumption (see Task 1).
- Status computation stays in SQL (`CASE` expression), not post-processed in Go, per design.md Key Decision #2.
- Backend property tests use `pgregory.net/rapid` against a real test Postgres (query correctness, not pure-function logic) since the status logic lives in SQL.
- Frontend property tests use `fast-check`, minimum 100 iterations, per design.md Testing Strategy.
- All UI labels and messages are in Bahasa Indonesia, matching requirements.md acceptance criteria verbatim where quoted.
- Brand/Deployment Type filters are NOT deferred — the columns exist, so implement them in the same wave as Status/Machine Type filters (this also corrects design.md's "deferred — schema gap" note on those two FilterBar sub-items).

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
