# Implementation Plan: ATM Profile

## Overview

The ATM Profile feature adds a single-ATM detail view to the existing ATM Portal module. Implementation follows a backend-first approach: sqlc queries → service layer → HTTP handlers → frontend types/hooks → UI components → wiring and accessibility. Each task builds incrementally so there is no orphaned code.

## Tasks

- [ ] 1. Backend: sqlc queries for ATM Profile
  - [ ] 1.1 Add ATM Profile SQL queries to `backend/queries/atm_portal.sql`
    - Add `GetATMByTerminalID` query joining `atms` with `locations`, filtering `is_active = true` and `deleted_at IS NULL`
    - Add `GetLatestReplenishForTerminal` query returning `refund_total` for status computation
    - Add `ListReplenishByTerminal` query with pagination and optional date_from/date_to filtering, ordered by replenish_date DESC, replenish_time DESC
    - Add `CountReplenishByTerminal` query mirroring the list filters
    - Add `ListCashposByTerminal` query with pagination and optional date_from/date_to filtering, ordered by cashpos_date DESC, id DESC
    - Add `CountCashposByTerminal` query mirroring the list filters
    - Run `sqlc generate` to produce type-safe Go code
    - _Requirements: 3.1, 4.1, 4.2, 4.3, 4.4, 4.7, 5.1, 5.2, 5.3_

- [ ] 2. Backend: Service layer for ATM Profile
  - [ ] 2.1 Create `backend/internal/service/atm_portal_profile.go` with profile service methods
    - Implement `GetATMProfile(ctx, terminalID)` — validate terminal ID, fetch ATM master data from read replica, compute replenishment status using precedence logic (unconfigured → no_data → critical → low → normal), return `ATMProfileResult`
    - Implement `ListATMReplenish(ctx, params)` — validate pagination/date params, query read replica, return paginated result
    - Implement `ListATMCashpos(ctx, params)` — validate pagination/date params, query read replica, return paginated result
    - Add types: `ATMProfileResult`, `ListATMReplenishParams`, `ListATMReplenishResult`, `ReplenishRecord`, `ListATMCashposParams`, `ListATMCashposResult`, `CashposRecord`
    - Extend the `AtmPortalServicer` interface with the three new methods
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 4.1–4.11, 5.1–5.9_

  - [ ]* 2.2 Write property tests for replenishment status computation (Go/rapid)
    - **Property 4: Replenishment Status Computation Precedence**
    - Generate random combinations of low_threshold_amount (nullable), critical_threshold_amount (nullable), refund_total (nullable), verify strict precedence order
    - **Validates: Requirements 2.5, 3.3**

  - [ ]* 2.3 Write property tests for replenish query invariants (Go/rapid)
    - **Property 5: Replenish Query Result Invariants**
    - Generate test records and query params, verify terminal_id match, date range bounds, and descending order
    - **Validates: Requirements 4.1, 4.4, 4.7**

  - [ ]* 2.4 Write property tests for cashpos query invariants (Go/rapid)
    - **Property 6: Cashpos Query Result Invariants**
    - Generate test records and query params, verify terminal_id match, date range bounds, and descending order
    - **Validates: Requirements 5.1, 5.3**

  - [ ]* 2.5 Write property tests for pagination total consistency (Go/rapid)
    - **Property 9: Pagination Total Consistency**
    - Generate varying page sizes and record counts, verify `total >= len(data)` and when `total <= page_size` then `len(data) == total`
    - **Validates: Requirements 4.8, 5.4**

  - [ ]* 2.6 Write property tests for ATM Profile response completeness (Go/rapid)
    - **Property 8: ATM Profile Response Completeness**
    - Generate random active ATM records, verify all required fields present with correct types
    - **Validates: Requirements 3.1**

- [ ] 3. Backend: HTTP handlers for ATM Profile
  - [ ] 3.1 Add handler methods to `backend/internal/handler/atm_portal_handler.go`
    - Implement `GetATMProfile` handler: parse `terminalId` path param, validate non-empty, call service, serialize with `pkg/response` envelope, monetary fields as decimal strings or JSON null
    - Implement `ListATMReplenish` handler: parse path param + query params (page, page_size, date_from, date_to), validate formats, call service, serialize response
    - Implement `ListATMCashpos` handler: parse path param + query params, validate, handle date_from > date_to as 400 error per Req 5.8, call service, serialize response
    - Register routes: `r.Get("/atms/{terminalId}", h.GetATMProfile)`, `r.Get("/atms/{terminalId}/replenish", h.ListATMReplenish)`, `r.Get("/atms/{terminalId}/cashpos", h.ListATMCashpos)` within existing `RequireAuth` route group
    - _Requirements: 3.1–3.6, 4.1–4.11, 5.1–5.9_

  - [ ]* 3.2 Write unit tests for ATM Profile handlers
    - Test param parsing, error responses for empty terminalId (400), invalid page/page_size (400), invalid date format (400), not found (404), success (200)
    - Test RequireAuth middleware enforcement
    - File: `backend/internal/handler/atm_portal_profile_test.go`
    - _Requirements: 3.2, 3.4, 3.5, 4.5, 4.9, 4.10, 5.5, 5.6, 5.7, 5.8_

- [ ] 4. Checkpoint — Backend complete
  - Ensure all backend tests pass (`go test ./...`), sqlc generates cleanly, and `golangci-lint run` has no errors. Ask the user if questions arise.

- [ ] 5. Frontend: Types and data hooks
  - [ ] 5.1 Add ATM Profile TypeScript types to `frontend/CompanyPortal-Vite/src/features/atm-portal/types.ts`
    - Add `AtmProfileMasterData` interface (all master data fields + replenishment_status)
    - Add `AtmReplenishRecord` interface
    - Add `AtmReplenishResponse` interface (paginated)
    - Add `AtmCashposProfileResponse` interface (paginated, reuse existing `AtmCashposRecord` if compatible or add new)
    - Add `AtmProfileTab` type (`"replenish" | "cashpos"`)
    - _Requirements: 10.4_

  - [ ] 5.2 Create `frontend/CompanyPortal-Vite/src/features/atm-portal/useAtmProfileData.ts`
    - Implement `useAtmMasterData(terminalId)` hook using TanStack Query — queryKey: `["atm-profile", "master", terminalId]`, staleTime 5min, retry 1
    - Implement `useAtmReplenishHistory(terminalId, params)` hook — queryKey includes params, staleTime 2min, `placeholderData: keepPreviousData`, `enabled: isActiveTab`
    - Implement `useAtmCashposHistory(terminalId, params)` hook — same pattern as replenish
    - All hooks use the API client to call the correct endpoints
    - _Requirements: 10.3, 9.6_

- [ ] 6. Frontend: Utility formatters
  - [ ] 6.1 Create or extend currency/date formatting utilities
    - Implement `formatRupiah(value: string | null): string` — "Rp " prefix, dot-separated thousands, em dash for null
    - Implement `formatDateIndonesian(isoDate: string): string` — "dd MMM yyyy" with Indonesian month abbreviations
    - Implement `formatWholeNumber(value: string): string` — dot-separated thousands, no decimals
    - Place in shared utility location (e.g., `src/lib/formatters.ts` or co-located with ATM Portal)
    - _Requirements: 2.2, 2.3, 7.2, 7.3, 8.2, 8.3_

  - [ ]* 6.2 Write property tests for formatRupiah (fast-check + Vitest)
    - **Property 1: Rupiah Currency Formatting**
    - Generate random numeric strings (including zero, large values, decimals), verify "Rp " prefix + dot-separated thousands pattern; verify null → "—"
    - **Validates: Requirements 2.2, 2.3, 7.3**

  - [ ]* 6.3 Write property tests for formatDateIndonesian (fast-check + Vitest)
    - **Property 2: Indonesian Date Formatting**
    - Generate random valid ISO date strings, verify "dd MMM yyyy" pattern with Indonesian month abbreviations, verify round-trip parsability
    - **Validates: Requirements 7.2, 8.2**

  - [ ]* 6.4 Write property tests for formatWholeNumber (fast-check + Vitest)
    - **Property 3: Whole Number Formatting with Dot Separators**
    - Generate random numeric strings, verify dot-separated output, no decimals, numeric value preserved
    - **Validates: Requirements 8.3**

- [ ] 7. Frontend: ATM Profile sub-components
  - [ ] 7.1 Create `AtmHeader` component in `src/features/atm-portal/components/AtmHeader.tsx`
    - Display all master data fields in responsive grid (3-col ≥1024px, 2-col 768–1023px, 1-col <768px)
    - Format monetary fields with `formatRupiah`, show em dash for null
    - Terminal ID in monospace font
    - StatusBadge showing replenishment status (Critical/danger, Low/warning, Normal/success, Unconfigured/neutral, No Data/neutral)
    - `aria-label` on monetary fields with currency value or "tidak tersedia" for null
    - Loading skeleton state (minimum 3 skeleton rows)
    - _Requirements: 2.1–2.7, 9.1, 11.1, 12.5_

  - [ ] 7.2 Create `TabNavigation` component in `src/features/atm-portal/components/TabNavigation.tsx`
    - Accessible tab pattern: `role="tablist"`, `role="tab"`, `role="tabpanel"`, `aria-selected`, `aria-controls`, `aria-labelledby`
    - Keyboard navigation: ArrowLeft/Right, Enter/Space, Home/End
    - Two tabs: "Replenish" and "Cashpos"
    - Sync active tab with URL `?tab=` query parameter
    - Focus moves to tabpanel on activation
    - Minimum 44×44px touch targets on mobile
    - _Requirements: 6.4–6.8, 12.1, 12.2, 12.8_

  - [ ]* 7.3 Write property test for tab state URL round-trip (fast-check + Vitest)
    - **Property 7: Tab State URL Round-Trip**
    - Generate valid tab values, verify setTab → readTabFromURL round-trip consistency
    - **Validates: Requirements 6.8**

  - [ ] 7.4 Create `ReplenishTable` component in `src/features/atm-portal/components/ReplenishTable.tsx`
    - TanStack Table with columns: Replenish Date, Replenish Time, Refund Denom 10k/20k/50k/100k, Refund Total, Replenish Denom 10k/20k/50k/100k, Replenish Total, Escrow
    - Date column formatted with `formatDateIndonesian`, monetary columns with `formatRupiah` (right-aligned, tabular-nums)
    - Date range filter above table, defaulting to last 30 days
    - Pagination controls: current page, total pages, page size selector (10, 25, 50, 100; default 25)
    - Empty state: "Belum ada data replenish untuk ATM ini"
    - Error state: AlertCircle icon + message + "Coba Lagi" retry button
    - Loading state: 5 skeleton rows
    - Semantic HTML table (`<table>`, `<thead>`, `<tbody>`, `<th scope="col">`) with `aria-label` "Riwayat Replenish"
    - Horizontal scroll when content overflows
    - _Requirements: 7.1–7.8, 9.4, 9.5, 11.2, 12.3, 12.7_

  - [ ] 7.5 Create `CashposProfileTable` component in `src/features/atm-portal/components/CashposProfileTable.tsx`
    - TanStack Table with columns: Cashpos Date, Teller ID, Branch Code, Position Source, and grouped denomination columns (10K, 20K, 50K, 100K × Starting Cash, Cash In, Cash Out, Cash Position)
    - Date column with `formatDateIndonesian`, denomination columns with `formatWholeNumber` (right-aligned, tabular-nums, dot separators)
    - Date range filter above table, defaulting to last 30 days
    - Date validation: disable apply when end < start, show validation message
    - Pagination controls: same pattern as ReplenishTable (10, 25, 50, 100; default 25)
    - Empty state: "Belum ada data cashpos untuk ATM ini"
    - Loading state: skeleton rows/spinner
    - Horizontal scroll container for wide denomination columns
    - Semantic HTML table with `aria-label` "Riwayat Cash Position"
    - _Requirements: 8.1–8.10, 9.4, 9.5, 11.2, 12.3, 12.7_

- [ ] 8. Frontend: ATM Profile page and routing
  - [ ] 8.1 Create route file `frontend/CompanyPortal-Vite/src/routes/atm-portal.$terminalId.tsx`
    - Define TanStack Router route with `terminalId` dynamic segment
    - Export route component rendering `AtmProfileScreen`
    - _Requirements: 10.5_

  - [ ] 8.2 Create `AtmProfileScreen` in `src/features/atm-portal/AtmProfileScreen.tsx`
    - Render within existing AppShell
    - Breadcrumb navigation: `<nav aria-label="Breadcrumb">` with `<ol>`, "ATM Portal" link to `/atm-portal`, current terminal ID with `aria-current="page"`
    - PageHeader with eyebrow "ATM Portal", title = terminal ID, description = location name (or "—" if empty)
    - AtmHeader section with master data
    - TabNavigation with Replenish and Cashpos tabs
    - Active tabpanel rendering ReplenishTable or CashposProfileTable
    - ARIA live region (`aria-live="polite"`, `aria-atomic="true"`) for state announcements
    - Default tab = "Replenish" when no `?tab` param
    - Full-page not-found state on 404 ("ATM tidak ditemukan" + link back)
    - Full-page error state on 5xx/timeout (AlertCircle --danger-fg + "Coba Lagi" button)
    - 30-second request timeout handling
    - _Requirements: 1.1–1.6, 6.1–6.8, 9.1–9.7, 10.1, 10.2, 12.4, 12.6, 12.8_

  - [ ]* 8.3 Write property test for ARIA live region announcements (fast-check + Vitest)
    - **Property 10: ARIA Live Region State Announcements**
    - Generate state transitions (loading → loaded/error/empty), verify distinct non-empty announcement strings per state
    - **Validates: Requirements 12.6**

- [ ] 9. Frontend: Navigation integration
  - [ ] 9.1 Update ATM Portal list to make terminal ID column a clickable link
    - Render terminal ID as a TanStack Router `<Link>` to `/atm-portal/$terminalId`
    - Apply standard link styling (visually distinguished from non-interactive text)
    - Client-side navigation (no full page reload), pushes to browser history
    - _Requirements: 1.1, 1.2, 1.6_

- [ ] 10. Checkpoint — All tests pass
  - Ensure all frontend tests pass (`pnpm test --run`), backend tests pass (`go test ./...`), linting is clean. Ask the user if questions arise.

- [ ] 11. Frontend: Component unit tests
  - [ ]* 11.1 Write unit tests for AtmHeader component
    - Test renders all fields, handles null monetary (em dash), correct status badge variant, skeleton loading state
    - _Requirements: 2.1–2.7, 9.1_

  - [ ]* 11.2 Write unit tests for TabNavigation component
    - Test ARIA attributes, keyboard navigation, tab activation, URL sync
    - _Requirements: 6.4–6.8, 12.1, 12.2_

  - [ ]* 11.3 Write unit tests for ReplenishTable and CashposProfileTable
    - Test column rendering, empty state message, error state with retry, pagination controls, date filter
    - _Requirements: 7.1–7.8, 8.1–8.10_

  - [ ]* 11.4 Write unit tests for AtmProfileScreen
    - Test breadcrumb structure, full-page error/not-found states, retry behavior, loading states
    - _Requirements: 1.3–1.5, 9.1–9.7_

- [ ] 12. Final checkpoint — Ensure all tests pass
  - Run full test suite: `go test ./...` and `pnpm test --run`. Ensure `golangci-lint run` and `pnpm lint` pass. Ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Backend queries use read replica pool for all operations (read-only feature)
- Monetary fields serialized as decimal strings to avoid float precision loss
- All endpoints wrapped in existing `RequireAuth` middleware (no new auth logic needed)

## Claude Model Recommendations

| Task Group | Recommended Model | Rationale |
|------------|------------------|-----------|
| 1.1 (sqlc queries) | Sonnet | Straightforward SQL, pattern-matching from existing queries. No deep reasoning needed. |
| 2.1 (Service layer) | Sonnet | Business logic is well-defined in the design doc. Status computation is a simple conditional chain. |
| 2.2–2.6 (Property tests) | Opus | Property-based test design requires reasoning about invariants and edge cases. Worth the extra intelligence. |
| 3.1 (HTTP handlers) | Sonnet | Boilerplate-heavy handler code following existing patterns in the codebase. |
| 3.2 (Handler unit tests) | Sonnet | Table-driven tests following existing test patterns. |
| 5.1 (TypeScript types) | Haiku | Pure type definitions copied from the design doc. Minimal reasoning. |
| 5.2 (Data hooks) | Sonnet | TanStack Query hooks following established patterns. Moderate complexity. |
| 6.1 (Formatters) | Sonnet | Small utility functions with clear specs. |
| 6.2–6.4 (Formatter property tests) | Opus | Designing good property generators and shrink strategies benefits from deeper reasoning. |
| 7.1 (AtmHeader) | Sonnet | UI component with clear requirements. Responsive grid + conditional rendering. |
| 7.2 (TabNavigation) | Opus | Accessible tab pattern (WAI-ARIA) + keyboard navigation + URL sync. Multiple interacting concerns. |
| 7.3 (Tab property test) | Sonnet | Simple round-trip property, well-scoped. |
| 7.4–7.5 (Tables) | Sonnet | TanStack Table setup following existing CashposTable pattern. Column config heavy but mechanical. |
| 8.1 (Route file) | Haiku | Single-line route definition. Trivial. |
| 8.2 (AtmProfileScreen) | Opus | Page orchestration: error boundaries, loading states, tab routing, ARIA live regions, breadcrumbs. Most complex frontend task. |
| 8.3 (ARIA property test) | Sonnet | Focused property with clear state machine. |
| 9.1 (Navigation link) | Haiku | Add a `<Link>` component to an existing table column. Minimal change. |
| 11.1–11.4 (Unit tests) | Sonnet | Component tests with React Testing Library. Standard patterns. |

**Summary:** Use **Opus** for tasks requiring architectural reasoning (property test design, complex UI orchestration with accessibility). Use **Sonnet** for the majority of implementation tasks. Use **Haiku** for trivial boilerplate (type defs, route files, single-line changes).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1", "5.1"] },
    { "id": 2, "tasks": ["2.2", "2.3", "2.4", "2.5", "2.6", "3.1", "5.2", "6.1"] },
    { "id": 3, "tasks": ["3.2", "6.2", "6.3", "6.4", "7.1", "7.2"] },
    { "id": 4, "tasks": ["7.3", "7.4", "7.5"] },
    { "id": 5, "tasks": ["8.1", "9.1"] },
    { "id": 6, "tasks": ["8.2"] },
    { "id": 7, "tasks": ["8.3", "11.1", "11.2", "11.3", "11.4"] }
  ]
}
```
