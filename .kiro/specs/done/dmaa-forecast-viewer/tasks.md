# Implementation Plan: DMAA Forecast Viewer

## Overview

A read-only paginated viewer for ATM cash forecast data from the `dmaa_atm_forecast` table. The implementation adds a backend GET endpoint (Go + Chi + sqlc) with server-side pagination, filtering, and sorting using the read replica, and a frontend page (React + TanStack Router/Query/Table) with URL state synchronization. Follows existing patterns from `atm_portal_handler.go` and `forecast/` feature module.

## Tasks

- [x] 1. Backend: sqlc queries and repository layer
  - [x] 1.1 Create sqlc query file `backend/queries/dmaa_forecast.sql` with ListDmaaForecast and CountDmaaForecast queries
    - Write ListDmaaForecast (:many) with dynamic WHERE for date_from, date_to, terminal_id (ILIKE), and dynamic ORDER BY via CASE expressions for all 7 sortable columns
    - Write CountDmaaForecast (:one) with same WHERE filters, no pagination
    - Use `sqlc.narg` for nullable filter params, `@` for sort_by/sort_order/page_size/offset
    - _Requirements: 3.1, 4.1–4.4, 5.1–5.2, 6.1–6.4, 8.1_

  - [x] 1.2 Run `sqlc generate` and verify generated Go code compiles
    - Ensure generated `ListDmaaForecast` and `CountDmaaForecast` functions exist in `backend/internal/db/`
    - _Requirements: 8.1_

- [x] 2. Backend: service layer with validation
  - [x] 2.1 Create `backend/internal/service/dmaa_forecast.go` with DmaaForecastServicer interface and DmaaForecastService implementation
    - Define `ListDmaaForecastParams` struct (Page, PageSize, DateFrom, DateTo, TerminalID, SortBy, SortOrder)
    - Define `DmaaForecastRow` domain struct and `ListDmaaForecastResult` with pagination metadata
    - Define `DmaaForecastRepository` interface (ListDmaaForecast, CountDmaaForecast)
    - Implement `ListDmaaForecast` method: validate params → compute offset → call repo for rows + count → assemble result with TotalPages = ceil(TotalRows / PageSize)
    - _Requirements: 3.1–3.5, 4.1–4.5, 5.1–5.2, 6.1–6.5, 8.1_

  - [x] 2.2 Implement parameter validation in the service
    - Validate page >= 1, page_size in [1, 100]
    - Validate date_from/date_to parse to YYYY-MM-DD when non-empty
    - Validate sort_by against allowed column whitelist: terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund, created_at
    - Validate sort_order is "asc" or "desc"
    - Return typed `*ValidationError` for invalid params (maps to 400 in handler)
    - _Requirements: 3.5, 4.5, 6.5_

  - [x]* 2.3 Write unit tests for DmaaForecastService validation logic
    - Test valid params pass through
    - Test page < 1 returns error, page_size out of range returns error
    - Test invalid date format returns error, valid dates pass
    - Test invalid sort_by column returns error, all allowed columns pass
    - Test invalid sort_order returns error
    - _Requirements: 3.5, 4.5, 6.5_

- [x] 3. Backend: handler and route registration
  - [x] 3.1 Create `backend/internal/handler/dmaa_forecast_handler.go` with DmaaForecastHandler
    - Implement `NewDmaaForecastHandler(svc service.DmaaForecastServicer)` constructor
    - Implement `Routes() chi.Router` returning router with `r.Get("/", h.ListDmaaForecast)`
    - Implement `ListDmaaForecast` handler: parse query params → call service → write JSON response envelope
    - Implement `parseDmaaForecastParams(q url.Values)` using existing `parseIntParam` and `queryOrDefault` helpers
    - Implement `handleServiceError` mapping ValidationError→400, DB errors→503
    - Implement `toDmaaForecastResponse` to convert service result to JSON envelope with `data` array and `pagination` object
    - _Requirements: 3.1–3.5, 4.1–4.5, 5.1–5.2, 6.1–6.5, 8.2–8.4_

  - [x] 3.2 Register the DMAA forecast route in `backend/cmd/api/main.go`
    - Instantiate DmaaForecastService with `db.New(dbPool)` (read replica pool when available, dbPool for now)
    - Instantiate DmaaForecastHandler with the service
    - Mount at `/api/v1/dmaa-forecast` with `custommw.RequireAuth(tokenService)` and `custommw.RequireRoles("ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV", "ADMIN", "ADMIN_PARAM")`
    - _Requirements: 2.1–2.4, 8.3_

  - [x]* 3.3 Write unit tests for DmaaForecastHandler query parameter parsing
    - Test valid params are parsed correctly
    - Test missing params use defaults (page=1, page_size=25, sort_by=periode_pred, sort_order=desc)
    - Test non-numeric page/page_size returns 400
    - _Requirements: 3.2–3.5, 6.3–6.4_

- [x] 4. Checkpoint — Backend verification
  - Ensure all tests pass (`go test ./...`), ask the user if questions arise.

- [x] 5. Frontend: types and TanStack Query hook
  - [x] 5.1 Create `frontend/CompanyPortal-Vite/src/features/dmaa-forecast/types.ts`
    - Define `DmaaForecastRow` interface (terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund, created_at)
    - Define `PaginationMeta` interface (page, page_size, total_rows, total_pages)
    - Define `DmaaForecastResponse` interface (data: DmaaForecastRow[], pagination: PaginationMeta)
    - _Requirements: 7.1_

  - [x] 5.2 Create `frontend/CompanyPortal-Vite/src/features/dmaa-forecast/useDmaaForecastData.ts`
    - Define `DmaaForecastQueryParams` interface matching URL search params
    - Implement `useDmaaForecastData` hook using `useQuery` with queryKey `["dmaa-forecast", params]`
    - Map camelCase params to snake_case API params (pageSize→page_size, dateFrom→date_from, etc.)
    - Use `placeholderData: keepPreviousData` for smooth pagination transitions
    - Use existing `apiClient` from `@/lib/api/client`
    - _Requirements: 3.6, 9.1–9.3_

- [x] 6. Frontend: feature components
  - [x] 6.1 Create `frontend/CompanyPortal-Vite/src/features/dmaa-forecast/DmaaForecastFilters.tsx`
    - Add date picker inputs for dateFrom and dateTo
    - Add text input for terminal ID search with placeholder "Cari Terminal ID..."
    - On filter change, call navigate with updated search params and reset page to 1
    - _Requirements: 4.6–4.7, 5.3–5.4_

  - [x] 6.2 Create `frontend/CompanyPortal-Vite/src/features/dmaa-forecast/DmaaForecastTable.tsx`
    - Define column definitions for: Terminal ID, DMAA File ID, Periode Prediksi, Denominasi, Jumlah Replenish, Jumlah Refund, Dibuat Pada
    - Monetary amounts (amount_replenish, amount_refund) right-aligned with `tabular-nums` and IDR formatting
    - Dates (periode_pred) formatted as DD MMM YYYY in Asia/Jakarta timezone
    - Timestamps (created_at) formatted as DD MMM YYYY HH:mm in Asia/Jakarta timezone
    - Denomination values formatted with dot-thousands separator (e.g., 100.000)
    - Sortable column headers with visual direction indicator
    - Empty state message when no rows match filters
    - _Requirements: 7.1–7.6, 6.6–6.7, 9.3_

  - [x] 6.3 Create `frontend/CompanyPortal-Vite/src/features/dmaa-forecast/DmaaForecastView.tsx`
    - Compose DmaaForecastFilters + DmaaForecastTable + pagination controls
    - Read search params from TanStack Router's `useSearch`
    - Pass params to `useDmaaForecastData` hook
    - Handle loading state with loading indicator in table area
    - Handle error state with error message and retry button
    - Wire pagination controls to navigate with updated page/pageSize
    - Wire sort column click to navigate with updated sortBy/sortOrder
    - _Requirements: 3.6–3.7, 6.6–6.7, 9.1–9.3, 10.1–10.2_

  - [x] 6.4 Create `frontend/CompanyPortal-Vite/src/features/dmaa-forecast/index.ts` barrel export
    - Export DmaaForecastView as the main entry point
    - _Requirements: 1.1_

- [x] 7. Frontend: route and navigation registration
  - [x] 7.1 Create route file `frontend/CompanyPortal-Vite/src/routes/forecasting/dmaa-forecast.tsx`
    - Use `createRoute` with path `/forecasting/dmaa-forecast` and parent `protectedRoute`
    - Implement `validateSearch` with Zod schema: page (int, min 1, catch 1), pageSize (int, min 1, max 100, catch 25), dateFrom (string optional), dateTo (string optional), terminalId (string optional), sortBy (string, catch "periode_pred"), sortOrder (enum asc/desc, catch "desc")
    - Set component to `DmaaForecastView`
    - _Requirements: 10.1–10.3_

  - [x] 7.2 Register the route in `frontend/CompanyPortal-Vite/src/main.tsx`
    - Import `dmaaForecastRoute` from routes/forecasting/dmaa-forecast
    - Add to the protectedRoute children array alongside existing forecasting routes
    - _Requirements: 1.2_

  - [x] 7.3 Add navigation entry in `frontend/CompanyPortal-Vite/src/lib/config/navigation.ts`
    - Add nav item with id "dmaa-forecast", label "DMAA Forecast", icon TrendingUp (or BarChart3 to differentiate from Forecasting), href "/forecasting/dmaa-forecast", roles ["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"], group "forecasting"
    - Place after existing "forecast" entry in the NAV_CONFIG array
    - _Requirements: 1.1–1.4_

- [x] 8. Final checkpoint — Full verification
  - Ensure all tests pass (backend: `go test ./...`, frontend: `pnpm test --run`), ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- The backend uses the existing `db.New(dbPool)` pattern — when read replica pool is set up separately, swap to `dbRead` pool
- Follow the existing `atm_portal_handler.go` pattern for parseIntParam, queryOrDefault, writeJSON, writeError, handleServiceError helpers
- Follow the existing `forecast/` feature module structure for component organization
- IDR formatting and date formatting should reuse existing utility functions from `@/lib/` if available
- Property tests validate universal correctness properties from the design document
- Checkpoints ensure incremental validation

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "5.1"] },
    { "id": 1, "tasks": ["1.2", "5.2"] },
    { "id": 2, "tasks": ["2.1", "6.1"] },
    { "id": 3, "tasks": ["2.2", "6.2"] },
    { "id": 4, "tasks": ["2.3", "3.1", "6.3"] },
    { "id": 5, "tasks": ["3.2", "3.3", "6.4"] },
    { "id": 6, "tasks": ["7.1", "7.3"] },
    { "id": 7, "tasks": ["7.2"] }
  ]
}
```
