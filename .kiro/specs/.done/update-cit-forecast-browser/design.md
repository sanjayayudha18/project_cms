# Design: Update CIT Forecast Browser

## Overview

Three enhancements to the existing Forecast Browser (`request-replenish-to-vendor`, now in `.kiro/specs/done/`), touching the full vertical slice. Adds no new tables; depends on one additive-column migration (`030_master_atm_esq_columns.sql`, `vendor_branches.region` for FLM Vendor Region):

1. **Pagination moved into the table** with a page-size selector and result count.
2. **Cross-page "Pilih Semua Rekomendasi"** button (today's select-all is current-page-only).
3. **Four new context columns** (Lokasi ATM, Brand, FLM Vendor, FLM Vendor Region) via joins in the forecast query.

The layers touched, top to bottom:

```
ForecastBrowser.tsx  ── page state: selection, page, page_size, select-all fetch
  └─ ForecastTable.tsx ── columns (+4) + Table_Pagination_Footer
       └─ useForecastBrowse (hooks.ts) ── + a "fetch all matching ids" query for select-all
            └─ GET /api/v1/vendor-requests/forecast (+ new columns; page_size passthrough already exists)
                 └─ service.BrowseForecast → ForecastRow (+4 fields)
                      └─ ListForecastForDate / CountForecastForDate (+ joins)
```

## STOP-and-confirm gates (project-context Sec 3)

- **FLM_Vendor_Region source (RESOLVED).** Parsing the real MASTER_ATM_ESQ source (`document/MASTER_ATM_ESQ.csv.xlsx`) settled the earlier ambiguity: the file has THREE distinct region columns — `AreaMesin` (16 vals → `regions.region`, the ATM's geographic area), `FLMVendorRegion` (51 vals, e.g. "Jakarta Timur", the vendor's regional grouping), and `FLMVendorSubRegion` (349 vals → `vendor_branches.branch_name`). "FLM Vendor Region" = `FLMVendorRegion`, which had no home column. **It is now `vendor_branches.region`**, added by migration `030_master_atm_esq_columns.sql`. It is NOT `regions.region` (that would show the geographic area, wrong) and NOT `branch_name`. Because the FLM-vendor chain already passes through `vendor_branches`, exposing it needs no extra join — just select `vb.region`.
- **Schema change: migration 030 (approved, additive).** This design now depends on migration `030_master_atm_esq_columns.sql` (`vendor_branches.region` + `atms.escrow_account`, both nullable text, additive DDL only). That is the one schema change; the forecast query itself adds no tables.
- **Data-population caveat (not a code gate).** `vendor_packages` and `atm_vendor_packages` are EMPTY in the live schema and `vendor_branches.region` is newly added (unbackfilled). Until the MASTER_ATM_ESQ vendor-package seed lands, the FLM Vendor and FLM Vendor Region columns correctly resolve to NULL → rendered as the `"-"` placeholder. The LEFT JOINs guarantee no recommendation row is dropped meanwhile (Req 3.3, 3.4).

## Data model — the join chain (real column names, verified against `002_cms_tables.sql`)

`dmaa_atm_forecast` has no FK to `atms`; the only link is `dmaa_atm_forecast.terminal_id = atms.terminal_id` (both text; `atms.terminal_id` is UNIQUE).

```
dmaa_atm_forecast.terminal_id
  └─(text match)→ atms.terminal_id
        atms.brand                         → Brand
        atms.location_id
          └→ locations.id
                locations.name             → Lokasi ATM
        atms.id
          └→ atm_vendor_packages.atm_id   (WHERE is_active = true AND effective range current)
                atm_vendor_packages.vendor_package_id
                  └→ vendor_packages.id
                        vendor_packages.vendor_branch_id
                          └→ vendor_branches.id
                                vendor_branches.vendor_id → vendors.id
                                  └→ vendors.name          → FLM Vendor
                                vendor_branches.region     → FLM Vendor Region  (migration 030)
```

Note: `regions` is no longer joined — `regions.region` is the ATM's geographic `AreaMesin`, a different concept from the vendor's `FLMVendorRegion`. FLM Vendor Region comes off `vendor_branches.region`, which the vendor chain already reaches, so both `flm_vendor` and `flm_vendor_region` come from the single LATERAL active-package resolution.

**Active-vendor resolution (Req 3.2).** `atm_vendor_packages` can hold multiple rows per ATM (historical + current), keyed unique on `(atm_id, vendor_package_id, effective_start_date)`. A naive join multiplies forecast rows. Resolve the single active package with a `LATERAL` subquery (or a filtered `LEFT JOIN` guaranteed to yield ≤1 row):

```sql
LEFT JOIN LATERAL (
    SELECT vp.vendor_branch_id
    FROM atm_vendor_packages avp
    JOIN vendor_packages vp ON vp.id = avp.vendor_package_id
    WHERE avp.atm_id = a.id
      AND avp.is_active = true
      AND avp.effective_start_date <= sqlc.arg('forecast_date')::date
      AND (avp.effective_end_date IS NULL OR avp.effective_end_date >= sqlc.arg('forecast_date')::date)
    ORDER BY avp.effective_start_date DESC
    LIMIT 1
) active_pkg ON true
```

This guards against row multiplication and picks the package effective on the forecast date. The LATERAL returns `vendor_branch_id`; the outer query then `LEFT JOIN`s `vendor_branches` (yielding both `vendors.name` via `vendor_id` and `vendor_branches.region`) — so FLM Vendor and FLM Vendor Region both fall out of the same active-package resolution. All master-data joins are `LEFT JOIN` so a missing `atms`/`locations`/vendor never drops a recommendation (Req 3.3, 3.4).

## Backend changes

### SQL — `backend/queries/vendor_request.sql`

Extend `ListForecastForDate` to select the four new columns and add the joins above. Keep the existing `WHERE periode_pred = :forecast_date AND (:terminal_id = '' OR terminal_id ILIKE …)`, the `ORDER BY dmaa_atm_forecast.terminal_id ASC`, and the `LIMIT/OFFSET`. Qualify columns (`f.terminal_id`, etc.) now that multiple tables are joined.

`CountForecastForDate` does **not** need the master-data joins (it counts `dmaa_atm_forecast` rows only, and the LATERAL vendor join yields ≤1 row so it cannot change the count). Leave `CountForecastForDate` as-is to keep the count cheap and correct.

New added selections:
```
       l.name          AS lokasi_atm,
       a.brand         AS brand,
       v.name          AS flm_vendor,
       vb.region       AS flm_vendor_region
```
with `LEFT JOIN atms a ON a.terminal_id = f.terminal_id`, `LEFT JOIN locations l ON l.id = a.location_id`, the LATERAL active-package join, `LEFT JOIN vendor_branches vb ON vb.id = active_pkg.vendor_branch_id`, `LEFT JOIN vendors v ON v.id = vb.vendor_id`. (No `regions` join — see the Data model note; `flm_vendor_region` is `vb.region` from migration 030, not `regions.region`.)

**sqlc regeneration caveat (RESOLVED — Task 1).** `sqlc generate` runs successfully (the migration-017-era issue does not block it from running), but it rewrites unrelated already-generated files (`approval.sql.go`, `audit.sql.go`, `auth.sql.go`, `models.go`) that have drifted from a clean regen over time. Run it, then `git checkout --` everything except `internal/db/vendor_request.sql.go` — keep only the `ListForecastForDate`/`CountForecastForDate` diff. Confirmed the four new fields come out as `*string` (nullable pointers), not `pgtype.Text` — `backend/sqlc.yaml` sets `emit_pointers_for_null_types: true`, which is why every other nullable LEFT JOIN column in this codebase is already a Go pointer, not `pgtype.Text`.

### Service — `internal/service/vendor_request.go` + `vendor_request_actions.go`

Add to `ForecastRow`:
```go
LokasiATM        string
Brand            string
FLMVendor        string
FLMVendorRegion  string
```
In `BrowseForecast`'s mapping loop, copy the four db fields, converting `*string` → `string` (empty string when nil — see the sqlc regeneration caveat above; the generated type is a pointer, not `pgtype.Text`). No new validation; params unchanged.

### Handler — `internal/handler/vendor_request_response.go`

Extend `forecastRowResponse` additively (Req 4.1):
```go
LokasiATM       string `json:"lokasi_atm"`
Brand           string `json:"brand"`
FLMVendor       string `json:"flm_vendor"`
FLMVendorRegion string `json:"flm_vendor_region"`
```
Map them in `toForecastResponse`. No change to `forecastResponse` pagination wrapper. `page_size` passthrough already works (`BrowseForecast` already accepts it) — Req 1 is served without endpoint change beyond the columns.

## Frontend changes

### Types — `features/vendor-request/types.ts`

Extend `ForecastRow` with the four snake_case fields (`lokasi_atm`, `brand`, `flm_vendor`, `flm_vendor_region`), matching the wire shape (no camelCase transform, per existing convention).

### Table + pagination footer — `ForecastTable.tsx`

- Add four `ColumnDef` entries after `terminal_id`: Lokasi ATM, Brand, FLM Vendor, FLM Vendor Region. Left-aligned text. `enableSorting: false` (server sort not extended — Req 3.7). Cell renders value or `"-"` placeholder when empty (Req 3.6; never the literal em-dash — design ban).
- Add a **Table_Pagination_Footer** rendered inside this component (below `<table>` or as a `<tfoot>`-adjacent block): result count ("Menampilkan N dari M"), page indicator ("Halaman X dari Y"), page-size `<select>` (25/50/100), and Prev/Next `Button`s (secondary variant, min 44px). Footer is driven by props passed from the page: `pagination` meta, `page`, `pageSize`, and callbacks `onPageChange`, `onPageSizeChange`.
- Keep the existing `select` header checkbox (current-page toggle) — the cross-page button in Req 2 is additive, not a replacement.

Reference the existing `VendorRequestList.tsx` pagination markup for the page-size selector styling so both tables match.

### Page state + cross-page select-all — `ForecastBrowser.tsx`

- Replace the constant `PAGE_SIZE = 25` with `pageSize` state (default 25); reset `page` to 1 on page-size change (Req 1.5), date change, and filter change (Req 1.8, 2.5).
- Move the standalone pagination block out; pass pagination props down to `ForecastTable`.
- **Select_All_Recommendations button + Clear button** in the summary bar:
  - On click, fetch **all matching rows** for the active `forecastDate` + `debouncedAtmId`. The endpoint hard-rejects `page_size > 100` (`BrowseForecast` validation, `backend/internal/service/vendor_request_actions.go`: `if params.PageSize < 1 || params.PageSize > 100 { return ValidationError }`) — it does **not** clamp, it returns a 400. So a single `page_size=1000` request is not an option. Page through at `page_size=100` (the server max), accumulating rows across `page=1..total_pages`, stopping early and reporting the over-cap message (Req 2.10) once accumulated rows exceed the Vendor Request item limit (1000) — no need to fetch further pages past the cap. Prefer a dedicated hook `useForecastSelectAll` that is **disabled by default** and triggered imperatively (TanStack Query `fetchQuery` in a loop, or a manual paging loop calling the same `api.ts` fetch function) so the heavy fetch only runs on demand, not on every render.
  - Merge the fetched rows into `selectedRowsMap` (the existing cross-page selection map), preserving any manual ticks already made (Req 2.8). Set the current-page `rowSelection` for visible rows so checkboxes reflect the selection.
  - If the matching count exceeds 1000, do not silently truncate — show a message (toast or inline note) telling the operator the set is too large and to narrow the filter (Req 2.10).
  - "Bersihkan Pilihan" clears `selectedRowsMap` and `rowSelection` (Req 2.4).
- Filter-change behavior (Req 2.6): clear the selection on ATM filter change (simplest, never misleading). Document this in the button's helper text so operators know selection resets when they change the filter.
- `handleCreateClick` already reads `Object.values(selectedRowsMap)` — the cross-page selection flows into `Selection_Store` unchanged (Req 2.9). The extra display fields ride along on the row objects but are not required by the create payload (Req 3.9); the create mapper already picks only the five persisted fields.

### Hook — `features/vendor-request/hooks.ts` + `api.ts`

- `useForecastBrowse` gains no signature change (it already takes `pageSize`).
- Add `fetchAllForecastForSelection(params)` in `api.ts` calling the same `/forecast` endpoint at `page_size=100` (the confirmed server max — `BrowseForecast` rejects `page_size > 100` with a 400, it does not clamp), looping `page=1..total_pages` and accumulating rows, stopping once the accumulated count exceeds the 1000-item Vendor Request cap (Req 2.10) rather than fetching further pages. Expose via `useForecastSelectAll` or a `queryClient.fetchQuery` helper keyed off `vendorRequestKeys.forecast({...allPages})`.

## Selection math (cross-page)

The existing `selectedRowsMap: Record<string, ForecastRow>` keyed by `forecastRowId` already survives pagination. Select-all simply bulk-populates it. Selected count = `Object.keys(selectedRowsMap).length`; total = `Σ amount_replenish`. This is unchanged logic, just fed more rows at once — low regression risk.

## Error, empty, loading states

- Reuse `ForecastTable`'s existing skeleton/error/empty rows; `colSpan` must be updated to the new column count (was 5, becomes 9).
- Select-all in-flight: disable the button, show a spinner label ("Memilih…"); on error, surface a retry-able message and keep any prior manual selection intact.

## Testing strategy

**Backend**
- SQL/repository integration test (real Postgres, `//go:build integration`): seed an ATM with two `atm_vendor_packages` rows (one inactive/expired, one active) and assert the forecast row resolves the **active** vendor only and appears exactly once (Req 3.2). Seed a `dmaa_atm_forecast` row whose `terminal_id` has no `atms` match and assert the row still returns with empty new columns (Req 3.4). Seed an ATM with no active package → empty FLM vendor, row still present (Req 3.3).
- Handler test (httptest): response JSON includes the four new fields; existing fields and pagination unchanged (Req 4.1); role gate 401/403 preserved (Req 4.3).

**Frontend**
- `ForecastTable` renders the four columns and the `"-"` placeholder for empty values (Req 3.6); `colSpan` on empty/error rows equals 9.
- Page-size change resets to page 1 and refetches (Req 1.5); Prev/Next disabled at bounds (Req 1.6, 1.7).
- Cross-page select-all: with a mocked all-rows response spanning >1 page, the selection map contains every returned row and the summary total sums their `amount_replenish` (Req 2.2, 2.3); a prior manual tick is preserved after select-all (Req 2.8); over-cap response shows the warning and does not truncate silently (Req 2.10); changing the date clears the selection (Req 2.5).

## Files touched

Backend:
- `backend/queries/vendor_request.sql` — extend `ListForecastForDate` (joins + 4 columns)
- `backend/internal/db/vendor_request.sql.go` — regenerate or hand-edit `ListForecastForDate`
- `backend/internal/service/vendor_request.go` — `ForecastRow` +4 fields
- `backend/internal/service/vendor_request_actions.go` — mapping loop
- `backend/internal/handler/vendor_request_response.go` — `forecastRowResponse` + `toForecastResponse`

Frontend (`frontend/CompanyPortal-Vite/src/features/vendor-request/`):
- `types.ts` — `ForecastRow` +4 fields
- `ForecastTable.tsx` — 4 columns + Table_Pagination_Footer
- `ForecastBrowser.tsx` — pageSize state, select-all + clear, pagination props
- `hooks.ts` / `api.ts` — select-all fetch helper
