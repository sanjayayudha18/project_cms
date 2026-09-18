# Requirements Document

## Introduction

This spec updates the existing **Forecast Browser** page (`/replenishment/forecast-browser`, nav label "Rekomendasi CIT") in the internal CMS app (`frontend/CompanyPortal-Vite`) and its supporting ATM backend endpoint (`GET /api/v1/vendor-requests/forecast`, port 8080, flat JSON). The page lets an ATM operator browse DMAA replenishment recommendations (`dmaa_atm_forecast` rows) for a forecast date and select rows to compose a Vendor Request to the CIT vendor.

The feature already exists (shipped by the `request-replenish-to-vendor` spec, now in `.kiro/specs/done/`). This spec is a **targeted enhancement**, not a rebuild. Three changes:

1. **Pagination controls live inside the table component.** Today Prev/Next and "Halaman X dari Y" render as a separate block on the page (`ForecastBrowser.tsx`), disconnected from the table markup (`ForecastTable.tsx`). Move pagination into a table footer so the table owns its own paging UI, and add a page-size selector plus a result-count summary (matching the richer pattern already used by `VendorRequestList.tsx`).
2. **"Pilih Semua Rekomendasi" button — a true cross-page select-all.** Today the header checkbox selects only the current fetched page (25 rows). Add an explicit button that selects **every** DMAA recommendation matching the current forecast date + ATM filter, across all pages, not just the visible page.
3. **Four new context columns.** The table currently shows only `terminal_id`, `denom`, `amount_replenish`, `amount_refund`. Add **Lokasi ATM** (ATM location), **Brand**, **FLM Vendor**, and **FLM Vendor Region** so operators can see which ATM, run by which vendor in which region, each recommendation belongs to. These require new joins in the backend forecast query.

Scope is the Forecast Browser page and its `forecast` endpoint only. The Vendor Request creation flow, list, and detail pages are unchanged except where the carried-over selection payload must accommodate the four new fields (display-only; not persisted on `vendor_request_items`).

## Dependency & Scope Note

- **Reuses** the existing chain: SQL `ListForecastForDate`/`CountForecastForDate` (`backend/queries/vendor_request.sql`) → service `BrowseForecast` (`internal/service/vendor_request_actions.go`, `ForecastRow` in `vendor_request.go`) → handler `forecastRowResponse`/`toForecastResponse` (`internal/handler/vendor_request_response.go`) → frontend `useForecastBrowse` hook / `ForecastRow` type / `ForecastTable` / `ForecastBrowser`.
- **No new tables; one additive-column migration.** Three of the four columns read via joins over existing tables (`atms`, `locations`, `atm_vendor_packages`, `vendor_packages`, `vendor_branches`, `vendors`). FLM Vendor Region needs the new column `vendor_branches.region` from migration `030_master_atm_esq_columns.sql` (additive, nullable, approved). No new tables and no data migration.
- **Data-population caveat.** `vendor_packages` and `atm_vendor_packages` are EMPTY in the live schema (only an archived dummy seed exists) and `vendor_branches.region` is newly added and unbackfilled. Until the MASTER_ATM_ESQ vendor-package seed lands, FLM Vendor and FLM Vendor Region resolve to NULL and render as the `"-"` placeholder — expected, not a bug (LEFT JOINs keep every recommendation row).
- **sqlc caveat (project-context, RBAC-Setup):** `sqlc generate` is currently blocked by a pre-existing bug in migration 017. If regeneration cannot run, the generated `internal/db/*.sql.go` for the two forecast queries must be hand-edited to sqlc's output convention, as prior specs did. This is called out in tasks.md.
- **The four new fields are display-only.** They are NOT written to `vendor_request_items` (which stays `terminal_id, periode_pred, denom, amount_replenish, amount_refund`). They inform the operator's selection but do not change what a Vendor Request persists.

## Glossary

- **Forecast_Browser_Page**: The route `/replenishment/forecast-browser` in CompanyPortal-Vite, rendered by `ForecastBrowser.tsx`, role-gated to `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`.
- **Forecast_Table**: The `useReactTable` (TanStack Table v8) instance in `ForecastTable.tsx` listing DMAA recommendation rows with row selection and per-column sorting.
- **Forecast_Row**: One DMAA recommendation — the tuple `(terminal_id, periode_pred, denom)` with `amount_replenish`, `amount_refund`, `dmaa_file_id`, sourced from `dmaa_atm_forecast`.
- **Forecast_API**: The ATM backend endpoint `GET /api/v1/vendor-requests/forecast` accepting `forecast_date`, `atm_id`, `page`, `page_size`.
- **Select_All_Recommendations**: The new button that selects every Forecast_Row matching the active forecast date + ATM filter across all pages, not only the fetched page.
- **Table_Pagination_Footer**: The pagination UI (page-size selector, result count, page indicator, Prev/Next) rendered inside the Forecast_Table component.
- **Lokasi_ATM**: The ATM's location display name — `locations.name`, reached via `atms.location_id`.
- **Brand**: The ATM machine brand — `atms.brand`.
- **FLM_Vendor**: The vendor (FLM/CIT) servicing the ATM — `vendors.name`, reached via the active `atm_vendor_packages` → `vendor_packages` → `vendor_branches` → `vendors` chain.
- **FLM_Vendor_Region**: The vendor's regional grouping — `vendor_branches.region` (source column `MASTER_ATM_ESQ.FLMVendorRegion`, e.g. "Jakarta Timur"), reached via the same active `atm_vendor_packages` → `vendor_packages` → `vendor_branches` chain as FLM_Vendor. This is NOT `regions.region` (that is the ATM's geographic `AreaMesin`) and NOT `vendor_branches.branch_name` (the vendor sub-region). Column added by migration `030_master_atm_esq_columns.sql`.
- **Selection_Store**: The existing Zustand store `usePendingVendorRequestSelection` (`selectionStore.ts`) that carries the selected rows into the Vendor Request creation route.

## Requirements

### Requirement 1: Pagination controls inside the table

**User Story:** As an ATM operator, I want the paging controls attached to the forecast table with a page-size choice and a total count, so that I can navigate large recommendation sets without hunting for separate controls.

#### Acceptance Criteria

1. THE Forecast_Table SHALL render a Table_Pagination_Footer within the table component itself, not as a separate block owned by the page.
2. THE Table_Pagination_Footer SHALL display the current page number and the total number of pages, driven by the `pagination` metadata (`page`, `total_pages`) returned by the Forecast_API.
3. THE Table_Pagination_Footer SHALL display the total result count (`total_count`) for the active forecast date and ATM filter.
4. THE Table_Pagination_Footer SHALL provide a page-size selector offering at least the options 25, 50, and 100.
5. WHEN the operator changes the page size, THE Forecast_Browser_Page SHALL reset to page 1 and refetch with the new `page_size`.
6. THE Table_Pagination_Footer SHALL provide Previous and Next controls that are disabled at the first and last page respectively.
7. WHEN there is exactly one page of results, THE Table_Pagination_Footer SHALL disable both Previous and Next controls.
8. WHEN the operator changes the forecast date or the ATM filter, THE Forecast_Browser_Page SHALL reset to page 1.
9. THE Table_Pagination_Footer SHALL use the design-system tokens and the existing `Button` component (secondary variant for Prev/Next), consistent with the Merah Sirih internal theme, with a minimum 44px touch target on interactive controls.

### Requirement 2: Select-all-recommendations button (cross-page)

**User Story:** As an ATM operator, I want a single button that selects every DMAA recommendation for the chosen date and filter, so that I do not have to page through and tick each row to build a full Vendor Request.

#### Acceptance Criteria

1. THE Forecast_Browser_Page SHALL provide a Select_All_Recommendations button labeled in Indonesian (e.g. "Pilih Semua Rekomendasi").
2. WHEN the operator activates Select_All_Recommendations, THE Forecast_Browser_Page SHALL select every Forecast_Row matching the active forecast date and ATM filter across all pages, not only the currently fetched page.
3. WHEN a cross-page selection is active, THE Forecast_Browser_Page SHALL show the total selected item count and the summed `amount_replenish` total across all selected rows (`tabular-nums`, IDR).
4. THE Forecast_Browser_Page SHALL provide a way to clear the entire selection (e.g. "Bersihkan Pilihan") that resets both the selected rows and the per-page checkbox state.
5. WHEN the operator changes the forecast date, THE Forecast_Browser_Page SHALL clear any existing selection.
6. WHEN the operator changes the ATM filter, THE Forecast_Browser_Page SHALL either clear the selection or clearly indicate that the selection may include rows not matching the new filter. (Design.md picks one; the criterion is that stale selection state is never silently misrepresented.)
7. WHEN no rows match the active date and filter, THE Select_All_Recommendations button SHALL be disabled.
8. WHILE a Select_All_Recommendations fetch is in progress, THE Forecast_Browser_Page SHALL indicate the loading state and SHALL NOT lose a partial selection the operator made manually.
9. WHEN the operator proceeds to create a Vendor Request, THE Forecast_Browser_Page SHALL carry the complete cross-page selection (not just the visible page) into the Selection_Store.
10. THE Select_All_Recommendations action SHALL respect a safety cap consistent with the Vendor Request item limit (payloads capped at 1000 items per `request-replenish-to-vendor`); WHEN the matching set exceeds the cap, THE Forecast_Browser_Page SHALL inform the operator rather than silently truncating.

### Requirement 3: Four new context columns

**User Story:** As an ATM operator, I want each recommendation to show the ATM location, brand, FLM vendor, and vendor region, so that I can compose Vendor Requests with the right operational context without cross-referencing master data.

#### Acceptance Criteria

1. THE Forecast_API SHALL return, for each Forecast_Row, the Lokasi_ATM (`locations.name`), Brand (`atms.brand`), FLM_Vendor (`vendors.name`), and FLM_Vendor_Region (`vendor_branches.region`).
2. THE Forecast_API SHALL resolve FLM_Vendor via the ATM's **active** vendor package only (`atm_vendor_packages.is_active = true` and effective-date-current), so that a single ATM does not appear multiple times due to historical vendor-package rows.
3. WHEN an ATM has no active vendor package, THE Forecast_API SHALL return the row with a null/empty FLM_Vendor rather than dropping the row (a recommendation without a resolvable vendor is still shown, with the join done via `LEFT JOIN`).
4. WHEN a `dmaa_atm_forecast` row has no matching `atms` record, THE Forecast_API SHALL still return the recommendation row (LEFT JOIN), with the new columns null/empty, so no recommendation is hidden by a missing master-data join.
5. THE Forecast_Table SHALL display four new columns — Lokasi ATM, Brand, FLM Vendor, FLM Vendor Region — with clear Indonesian/English headers consistent with existing column labels.
6. WHERE a new column value is null or empty, THE Forecast_Table SHALL render a neutral placeholder (e.g. an em-dash substitute such as "-", never the literal em-dash per the design bans) rather than blank ambiguity.
7. THE Forecast_Table SHALL keep the existing sortable columns (ATM ID default ascending, Denominasi, Amount Replenish) working; the four new columns MAY be non-sortable if server-side sort is not extended.
8. THE new columns SHALL NOT change the Forecast_Row identity (`terminal_id|periode_pred|denom`) used for selection; adding columns SHALL NOT break existing row selection or the cross-page selection from Requirement 2.
9. THE addition of the new columns SHALL NOT alter what a Vendor Request persists — `vendor_request_items` remains `terminal_id, periode_pred, denom, amount_replenish, amount_refund`.

### Requirement 4: No regressions to the existing flow

**User Story:** As a maintainer, I want the enhancement to preserve the current contract and behavior, so that the Vendor Request creation flow and wire compatibility are not broken.

#### Acceptance Criteria

1. THE Forecast_API SHALL keep its existing flat JSON response shape (ATM backend convention), extending the per-row object additively with the four new fields — no field renames or removals.
2. THE Forecast_API SHALL keep the base ordering `terminal_id ASC` and the existing `forecast_date` / `atm_id` / `page` / `page_size` parameters and their defaults.
3. THE existing role gate (`ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`) on both the route and the endpoint SHALL remain unchanged.
4. THE reads for the forecast browse SHALL use the read path consistent with the existing implementation (project-context Sec 5 read/write split; reporting-style reads may target the replica where the existing code already does).
5. THE enhancement SHALL ship with tests covering: the extended SQL join (active-vendor resolution + LEFT JOIN row-preservation), the extended response mapping, the page-size change/reset behavior, and the cross-page select-all selection math.
