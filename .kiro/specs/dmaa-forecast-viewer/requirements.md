# Requirements Document

## Introduction

The DMAA Forecast Viewer provides a read-only page within the CMS internal portal for viewing ATM cash forecast data produced by the DMAA prediction engine. The data resides in the `dmaa_atm_forecast` table and includes per-terminal, per-denomination predicted replenishment and refund amounts for future dates. The page is placed under the Forecasting (Peramalan) navigation group and supports server-side pagination, date range filtering, terminal ID search, and column sorting. No data mutation, export, or row selection is provided.

## Glossary

- **DMAA Forecast Viewer**: The read-only page displaying records from the `dmaa_atm_forecast` database table.
- **Forecast API**: The backend REST endpoint serving paginated, filtered, and sorted forecast data from the read replica database.
- **Frontend Application**: The CMS internal portal SPA built with React, TypeScript, TanStack Router, TanStack Query, and TanStack Table.
- **RBAC Middleware**: The Chi middleware (`RequireRoles`) that enforces role-based access on API endpoints.
- **Read Replica**: The PostgreSQL read replica database instance used for read-only queries (reporting, dashboards).
- **Terminal ID**: A unique text identifier for an ATM, referencing `atms(terminal_id)`.
- **DMAA File ID**: A reference to the DMAA prediction batch file that produced the forecast row, referencing `dmaa_files(id)`.
- **Denomination (denom)**: The bank note denomination value in IDR (e.g., 50000, 100000).
- **Periode Pred**: The predicted date for which the forecast applies.
- **Amount Replenish**: The predicted cash replenishment amount (in IDR, stored as bigint).
- **Amount Refund**: The predicted cash refund amount (in IDR, stored as bigint).

## Requirements

### Requirement 1: Navigation Entry

**User Story:** As an internal CMS user with forecast access, I want a navigation link to the DMAA Forecast Viewer in the Peramalan group, so that I can access the forecast data alongside related forecasting pages.

#### Acceptance Criteria

1. THE Frontend Application SHALL display a navigation item labeled "DMAA Forecast" within the "Peramalan" (forecasting) group.
2. THE Frontend Application SHALL assign the route path `/forecasting/dmaa-forecast` to the DMAA Forecast Viewer navigation item.
3. THE Frontend Application SHALL restrict visibility of the DMAA Forecast Viewer navigation item to users with roles ATM-USER, ATM-SPV, BRANCH-ATM-USER, or BRANCH-ATM-SPV.
4. WHILE the user role is ADMIN or ADMIN_PARAM, THE Frontend Application SHALL display the DMAA Forecast Viewer navigation item regardless of the explicit roles list.

### Requirement 2: Role-Based Access Control

**User Story:** As a system administrator, I want the DMAA Forecast Viewer restricted to authorized roles, so that only users with forecast-related responsibilities can view the data.

#### Acceptance Criteria

1. THE Forecast API SHALL accept requests only from authenticated users with roles ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV, ADMIN, or ADMIN_PARAM.
2. WHEN an unauthenticated request is received, THE Forecast API SHALL respond with HTTP 401 and error code "unauthorized".
3. WHEN an authenticated user with an unauthorized role requests the endpoint, THE Forecast API SHALL respond with HTTP 403 and error code "forbidden".
4. THE RBAC Middleware SHALL enforce role checks at the route level using the existing `RequireRoles` middleware function.

### Requirement 3: Server-Side Pagination

**User Story:** As an ATM operations user, I want paginated forecast results, so that the page loads efficiently even with large datasets.

#### Acceptance Criteria

1. THE Forecast API SHALL accept `page` and `page_size` query parameters for pagination.
2. WHEN no `page` parameter is provided, THE Forecast API SHALL default to page 1.
3. WHEN no `page_size` parameter is provided, THE Forecast API SHALL default to 25 rows per page.
4. THE Forecast API SHALL return pagination metadata including `page`, `page_size`, `total_rows`, and `total_pages` in the response.
5. WHEN `page` or `page_size` contains a non-numeric value, THE Forecast API SHALL respond with HTTP 400 and error code "bad_request".
6. THE Frontend Application SHALL display pagination controls allowing the user to navigate between pages.
7. THE Frontend Application SHALL allow the user to change the page size.

### Requirement 4: Date Range Filtering

**User Story:** As an ATM operations user, I want to filter forecast data by prediction date range, so that I can focus on specific future periods.

#### Acceptance Criteria

1. THE Forecast API SHALL accept optional `date_from` and `date_to` query parameters in ISO 8601 date format (YYYY-MM-DD).
2. WHEN `date_from` is provided, THE Forecast API SHALL return only rows where `periode_pred` is on or after the specified date.
3. WHEN `date_to` is provided, THE Forecast API SHALL return only rows where `periode_pred` is on or before the specified date.
4. WHEN both `date_from` and `date_to` are provided, THE Forecast API SHALL return only rows where `periode_pred` falls within the inclusive date range.
5. WHEN `date_from` or `date_to` contains an invalid date format, THE Forecast API SHALL respond with HTTP 400 and error code "bad_request".
6. THE Frontend Application SHALL provide date picker inputs for specifying the date range filter.
7. WHEN the user changes the date range filter, THE Frontend Application SHALL reset pagination to page 1.

### Requirement 5: Terminal ID Search

**User Story:** As an ATM operations user, I want to search forecast data by terminal ID, so that I can find forecasts for a specific ATM.

#### Acceptance Criteria

1. THE Forecast API SHALL accept an optional `terminal_id` query parameter for text-based search.
2. WHEN `terminal_id` is provided, THE Forecast API SHALL return only rows where the `terminal_id` column contains the search value as a case-insensitive substring match.
3. WHEN the user enters a terminal ID search value, THE Frontend Application SHALL reset pagination to page 1.
4. THE Frontend Application SHALL provide a text input field for terminal ID search with a placeholder indicating its purpose.

### Requirement 6: Column Sorting

**User Story:** As an ATM operations user, I want to sort forecast data by any column, so that I can organize the view according to my analysis needs.

#### Acceptance Criteria

1. THE Forecast API SHALL accept `sort_by` and `sort_order` query parameters.
2. THE Forecast API SHALL support sorting by the columns: terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund, and created_at.
3. WHEN no `sort_by` parameter is provided, THE Forecast API SHALL default to sorting by `periode_pred`.
4. WHEN no `sort_order` parameter is provided, THE Forecast API SHALL default to descending order.
5. WHEN `sort_by` contains an unsupported column name, THE Forecast API SHALL respond with HTTP 400 and error code "bad_request".
6. THE Frontend Application SHALL allow the user to click column headers to toggle sort direction.
7. THE Frontend Application SHALL display a visual indicator on the currently sorted column showing the sort direction.

### Requirement 7: Data Display

**User Story:** As an ATM operations user, I want to see forecast data in a clear tabular format, so that I can review predicted replenishment and refund amounts.

#### Acceptance Criteria

1. THE Frontend Application SHALL display forecast data in a table with columns: Terminal ID, DMAA File ID, Periode Prediksi, Denominasi, Jumlah Replenish, Jumlah Refund, and Dibuat Pada.
2. THE Frontend Application SHALL display monetary amounts (amount_replenish, amount_refund) right-aligned with tabular number formatting and IDR currency formatting.
3. THE Frontend Application SHALL display date values (periode_pred) formatted as DD MMM YYYY in the Asia/Jakarta timezone.
4. THE Frontend Application SHALL display timestamp values (created_at) formatted as DD MMM YYYY HH:mm in the Asia/Jakarta timezone.
5. THE Frontend Application SHALL display denomination values as formatted numbers (e.g., "50.000", "100.000").
6. WHEN the API returns zero rows matching the current filters, THE Frontend Application SHALL display an empty state message indicating no forecast data matches the current criteria.

### Requirement 8: Backend Data Access

**User Story:** As a system architect, I want forecast queries to use the read replica, so that read-heavy dashboard traffic does not impact the primary database.

#### Acceptance Criteria

1. THE Forecast API SHALL execute all database queries for the DMAA Forecast Viewer against the read replica connection pool.
2. THE Forecast API SHALL return the response using the standard JSON envelope format consistent with other CMS endpoints.
3. THE Forecast API SHALL be mounted at the path `/api/v1/dmaa-forecast`.
4. IF the read replica is unavailable, THEN THE Forecast API SHALL respond with HTTP 503 and error code "service_unavailable".

### Requirement 9: Loading and Error States

**User Story:** As an internal CMS user, I want clear feedback when data is loading or an error occurs, so that I understand the current state of the page.

#### Acceptance Criteria

1. WHILE the forecast data is being fetched, THE Frontend Application SHALL display a loading indicator within the table area.
2. IF the API request fails, THEN THE Frontend Application SHALL display an error message describing the failure with an option to retry the request.
3. THE Frontend Application SHALL not display a blank or broken table layout during loading or error states.

### Requirement 10: URL State Synchronization

**User Story:** As an ATM operations user, I want my filter, sort, and pagination state reflected in the URL, so that I can share or bookmark a specific view.

#### Acceptance Criteria

1. WHEN the user changes pagination, filters, or sort options, THE Frontend Application SHALL update the browser URL query parameters to reflect the current state.
2. WHEN the page loads with query parameters in the URL, THE Frontend Application SHALL apply those parameters as the initial filter, sort, and pagination state.
3. THE Frontend Application SHALL use the query parameters: `page`, `pageSize`, `dateFrom`, `dateTo`, `terminalId`, `sortBy`, and `sortOrder`.
