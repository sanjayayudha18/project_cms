# Implementation Plan: ATM Portal Fix 1

## Overview

Incremental enhancement on the shipped ATM Portal (`atm-portal` spec). Two additive changes only — no schema migration, no new endpoints, no write path.

1. **Date range filter** — filter the list by `last_replenish_date` (from latest `itm_cashpos` per terminal) using optional `date_from` / `date_to` query params (inclusive, `YYYY-MM-DD`). Empty = no bound. Combines with existing filters via AND. Sync to URL like other filters. Summary cards remain **unfiltered** (Property 10 from original spec still holds).
2. **Total Replenish column** — surface existing API field `replenish_total` in `AtmTable` as **"Total Replenish"** (right-aligned Rupiah via `formatRupiah`, "—" on null). Optional sort by `replenish_total`.

**Frontend root (explicit):** `frontend/CompanyPortal-Vite/src/features/atm-portal/`

**Verified against live code (no migration needed):**
- Backend already returns `replenish_total` (`atm_portal.sql` lateral join + handler JSON + FE `AtmRecord.replenish_total`) — table simply does not render it yet.
- `itm_cashpos.replenish_date` is `date NOT NULL`; index `itm_cashpos_replenish_date_idx` exists.
- Date filter applies to the **latest** cashpos row's `last_replenish_date` (same LATERAL source as today), not historical rows. Terminals with no cashpos (`last_replenish_date IS NULL`) are **excluded** when either bound is set.

## Tasks

- [x] 1. Backend: SQL date-range filters + sort by replenish_total
  - [x] 1.1 Extend `backend/queries/atm_portal.sql` — `ListATMsWithCashPos` **and** `CountATMsWithCashPos` (filters must stay identical or pagination `total` drifts)
    - **List:** add optional bounds on `sub.last_replenish_date`:
      - `(sqlc.arg('date_from')::text = '' OR sub.last_replenish_date >= sqlc.arg('date_from')::date)`
      - `(sqlc.arg('date_to')::text = '' OR sub.last_replenish_date <= sqlc.arg('date_to')::date)`
    - **Count (required — easy to miss):** today Count's LATERAL/CTE does not project a date column. Implementers must:
      1. Include `lcp.replenish_date` in the Count LATERAL select
      2. Project `lcp.replenish_date AS last_replenish_date` into Count's `sub` SELECT
      3. Apply the **same** two `date_from` / `date_to` predicates on Count's WHERE as on List
    - Empty string = no filter (same convention as `search` / `region`); do not pass SQL NULL inconsistently
    - Do **not** add date filters to `GetATMSummary` or `GetLastUpdated` (summary stays global / independent of list filters — Property 10)
    - Add ORDER BY arms for `sort_by = 'replenish_total'` asc/desc (mirror `refund_total` pattern exactly, including NULL sort behavior)
    - _Touches: List + Count only_
  - [x] 1.2 Run `sqlc generate` and confirm `backend/internal/db/atm_portal.sql.go` picks up `DateFrom`, `DateTo` on **both** List and Count params, and replenish_total sort params
    - _No hand-edit of generated files_

- [x] 2. Backend: service + handler params
  - [x] 2.1 Extend `ListATMsParams` in `backend/internal/service/atm_portal.go`
    - Fields: `DateFrom string`, `DateTo string` (JSON `date_from` / `date_to`)
    - Validation (hand `validate()` — tags alone are not enforced):
      - Each non-empty value must parse as `YYYY-MM-DD` (use `time.Parse("2006-01-02", ...)`); Indonesian error e.g. `date_from harus berformat YYYY-MM-DD`
      - If both set and `date_from > date_to` → validation error e.g. `date_from tidak boleh lebih besar dari date_to`
    - Extend `sort_by` oneof / slices allow-list **and** `validate()` error message allow-list with `replenish_total`
    - **Wiring (required):** pass `DateFrom`/`DateTo` into **both** `ListATMsWithCashPosParams` and `CountATMsWithCashPosParams` inside `ListATMs` (empty string when unset)
  - [x] 2.2 Extend `parseListATMsParams` in `backend/internal/handler/atm_portal_handler.go`
    - Read `date_from`, `date_to` from query string (no defaults — empty means unbound)
    - No response shape change required (`replenish_total` already on `atmPortalRow`)

- [x] 3. Backend: tests
  - [x] 3.1 Property / integration coverage for date filter
    - **Property: Date range filter correctness** — for any fixtures with known `last_replenish_date`, filtered rows satisfy `date_from ≤ last_replenish_date ≤ date_to` (inclusive); only-from / only-to / both / neither cases
    - NULL `last_replenish_date` excluded when any bound is active
    - Invalid format and `date_from > date_to` → 400
    - Summary object identical across differently date-filtered requests (extends Property 10)
    - Extend `drawFilterParams` / sort allow-lists in `atm_portal_*_property_test.go` so random filters include dates and `replenish_total`
  - [x] 3.2 Sort by `replenish_total` asc/desc (extend existing sort property or example test)
  - [x] 3.3 Checkpoint — `go test` for atm_portal packages; manual `GET /api/v1/atm-portal/atms?date_from=...&date_to=...`
    - Note: integration property tests skip without `DATABASE_URL`; unit validation tests pass. `go build ./cmd/api` OK.

- [x] 4. Frontend: types, URL state, data hook
  - Paths under `frontend/CompanyPortal-Vite/src/features/atm-portal/`
  - [x] 4.1 Extend `AtmPortalParams` in `types.ts` with `date_from: string` and `date_to: string` (empty string default)
  - [x] 4.2 Extend `ATM_PORTAL_SEARCH_SCHEMA`, defaults, `parseSearchParams`, `omitDefaults` in `useAtmPortalUrlState.ts`
    - Defaults: both `""`; omit from URL when empty
    - Zod: optional strings (format validated server-side; client may lightly check `YYYY-MM-DD` before navigate if cheap)
  - [x] 4.3 Extend `buildQueryString` in `useAtmPortalData.ts` to include `date_from` / `date_to`
  - [x] 4.4 Extend Property 11 URL round-trip generators to include the new keys; update `mockParams` if present

- [x] 5. Frontend: FilterBar date controls
  - Paths under `frontend/CompanyPortal-Vite/src/features/atm-portal/`
  - [x] 5.1 Add date range UI to `components/FilterBar.tsx`
    - Two native `<input type="date">` (match DSR pattern in `DsrDashboard.tsx`) labeled e.g. **"Dari tanggal"** / **"Sampai tanggal"**
    - Controlled via props: `dateFrom`, `dateTo`, `onFilterChange` partial includes `date_from` | `date_to`
    - **Widen** `onFilterChange` / prop `Pick<>` types (today limited to status/machine/brand/deployment only)
    - `min-h-[44px]` touch targets consistent with existing controls
    - Changing either date resets page to 1 (via existing `setParams` parent pattern)
    - Include non-empty date bounds in **active filter count**; **Clear All** clears both
  - [x] 5.2 Wire props through `AtmPortalScreen.tsx` from `useAtmPortalUrlState`

- [x] 6. Frontend: AtmTable Total Replenish column + sort
  - Paths under `frontend/CompanyPortal-Vite/src/features/atm-portal/`
  - [x] 6.1 Update `components/AtmTable.tsx`
    - Insert column after **Refund Total** (before Threshold):
      - `{ key: "replenish_total", label: "Total Replenish", sortable: true, align: "right" }`
    - Cell: `formatRupiah(atm.replenish_total)` (already handles null → "—"); `tabular-nums` + right align
    - Header click uses existing sort toggle; parent already passes `sortBy`/`sortOrder`/`onSortChange`
    - Bump loading skeleton cell count / `colSpan` if any so column count matches
    - Bump `min-w-[...]` if needed so horizontal scroll still works
  - [x] 6.2 Allow `sort_by=replenish_total` in FE URL schema / any client-side allow-lists (property test generators)

- [x] 7. Frontend: tests + checkpoint
  - [x] 7.1 Component tests
    - FilterBar renders both date inputs; change fires `onFilterChange` with `date_from`/`date_to`
    - Clear All clears dates; active count includes date bounds
    - AtmTable header includes "Total Replenish"; row shows formatted `replenish_total` (and "—" when null)
    - Sort header for Total Replenish invokes `onSortChange("replenish_total", ...)`
  - [x] 7.2 Checkpoint
    - `pnpm tsc -b tsconfig.app.json`, `pnpm test` (atm-portal): **44 passed**
    - Backend: `go build ./cmd/api`, validation unit tests pass; integration properties skip without DATABASE_URL
    - Manual browser check left for user (date URL sync + Total Replenish column)

## Notes

- **No DB migration.** `replenish_total` and `replenish_date` already exist on `itm_cashpos`.
- **API is additive only:** new optional query params; response body unchanged.
- **Summary independence:** date filters must not affect `GetATMSummary` / summary cards (same rule as status/search filters today). Optional later UX copy (out of scope): ringkasan global, tidak mengikuti filter tanggal.
- **Filter field:** `last_replenish_date` of the latest cashpos row only — not a full history search across all `itm_cashpos` rows.
- **List vs Count parity:** date predicates and `last_replenish_date` projection must match on both queries (pagination integrity).
- **UI language:** labels/messages stay Bahasa Indonesia where user-facing copy is new (date labels); column header **"Total Replenish"** per product request (English, parallel to existing "Refund Total" / "Last Replenish Date").
- **Reuse:** native `type="date"` like DSR; do not pull VendorPortal `DatePicker` or any new date library.
- **Contract SoT (optional, ~15 min):** if parent `atm-portal/design.md` query-param table is still treated as contract, add one-line notes for `date_from`/`date_to` and `sort_by=replenish_total`.
- **Out of scope:** region filter UI (param already exists server-side but unused in FilterBar), escrow column, historical multi-row cashpos browser, CSV export, VendorPortal changes, replica/`dbRead` cutover, API money `float64`→decimal refactor, OpenAPI regen unless already maintained for this route, maker-checker (N/A — read-only).

## Definition of Done

- [x] List + Count filters identical for dates
- [x] `GetATMSummary` / `GetLastUpdated` unchanged; Property 10 still green with date params
- [x] `sort_by=replenish_total` allowed BE + FE + tests
- [x] Invalid date / `from > to` → 400, Indonesian messages
- [x] NULL last date excluded when any bound set
- [x] FE URL round-trip (Property 11) includes `date_from`/`date_to`
- [x] Table shows Total Replenish via `formatRupiah`; null → "—"
- [x] No migration; no new endpoint; no VendorPortal touch
- [x] `sqlc generate` only for generated Go; no hand-edits

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2"] },
    { "id": 2, "tasks": ["2.1", "2.2"] },
    { "id": 3, "tasks": ["3.1", "3.2"] },
    { "id": 4, "tasks": ["3.3"] },
    { "id": 5, "tasks": ["4.1"] },
    { "id": 6, "tasks": ["4.2"] },
    { "id": 7, "tasks": ["4.3"] },
    { "id": 8, "tasks": ["4.4", "5.1", "6.1"] },
    { "id": 9, "tasks": ["5.2", "6.2"] },
    { "id": 10, "tasks": ["7.1"] },
    { "id": 11, "tasks": ["7.2"] }
  ]
}
```

**Phase grouping:**
- **Phase A (backend):** waves 0–4 — SQL (List+Count) → sqlc → service/handler → tests → checkpoint
- **Phase B (frontend data path):** waves 5–7 sequential (`4.1` → `4.2` → `4.3`), then wave 8+ for UI/tests
