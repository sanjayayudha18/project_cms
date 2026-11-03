# Requirements Document

## Introduction

The ATM Portal provides CMS internal users with a centralized view of all ATMs and their cash position (cashpos) data. The portal joins ATM master data (`atms` table) with the latest transactional replenishment records (`itm_cashpos` table) to display current cash levels per terminal. A key function is flagging ATMs that need replenishment by comparing the latest `refund_total` (remaining cash after refund) against the ATM's configured `low_threshold_amount`. This enables operations teams to proactively schedule replenishment before ATMs run out of cash.

## Glossary

- **ATM_Portal**: The React page component rendered at the `/atm-portal` route, responsible for composing the ATM monitoring and replenishment flag UI
- **ATM_Table**: A TanStack Table component displaying ATM master data joined with latest cash position data, with filtering, sorting, and pagination
- **Replenishment_Flag_Engine**: The backend logic that compares an ATM's latest `refund_total` from `itm_cashpos` against the ATM's `low_threshold_amount` and `critical_threshold_amount` to determine replenishment need
- **Cash_Position**: The latest transactional record from `itm_cashpos` for a given terminal, representing the most recent replenishment event and resulting cash levels
- **Low_Threshold**: The `low_threshold_amount` value configured per ATM in the `atms` table, below which the ATM is flagged for replenishment
- **Critical_Threshold**: The `critical_threshold_amount` value configured per ATM in the `atms` table, indicating urgent replenishment need
- **Refund_Total**: The `refund_total` field from `itm_cashpos`, representing the remaining cash in the ATM after the last refund operation
- **Terminal_ID**: The unique text identifier linking `atms.terminal_id` to `itm_cashpos.terminal_id`
- **ATM_API**: The Go backend REST API handler serving ATM portal endpoints under `/api/v1/atm-portal`
- **ATM_Repository**: The sqlc-generated database access layer for ATM portal queries

## Requirements

### Requirement 1: ATM List API Endpoint

**User Story:** As a frontend developer, I want a paginated API endpoint that returns ATMs with their latest cash position data, so that the ATM Portal table can render efficiently with server-side pagination.

#### Acceptance Criteria

1. WHEN a GET request is made to `/api/v1/atm-portal/atms`, THE ATM_API SHALL return a paginated list of ATMs joined with their latest cash position from `itm_cashpos`.
2. THE ATM_API SHALL accept query parameters for pagination: `page` (default 1) and `page_size` (default 25, maximum 100).
3. THE ATM_API SHALL accept an optional `search` query parameter, and WHEN provided, THE ATM_API SHALL filter results by terminal_id or location name using case-insensitive partial matching (SQL `ILIKE '%term%'`).
4. THE ATM_API SHALL accept an optional `status` query parameter with values `all`, `low`, `critical`, `normal`, `unconfigured`, or `no_data`, and WHEN provided, THE ATM_API SHALL filter ATMs by their replenishment status.
5. THE ATM_API SHALL accept optional `machine_type`, `brand`, `deployment_type`, and `region` query parameters for filtering.
6. THE ATM_API SHALL accept a `sort_by` query parameter with allowed values `terminal_id`, `location`, `last_replenish_date`, `refund_total`, `status` (default `terminal_id`) and `sort_order` parameter with allowed values `asc`, `desc` (default `asc`) for column sorting.
7. THE ATM_API response SHALL include `data` (array of ATM records), `total` (total matching records count), `page`, `page_size`, and `last_updated` fields.
8. WHEN an ATM has no matching record in `itm_cashpos`, THE ATM_API SHALL return the ATM with null cash position fields and a status of `no_data`.
9. THE ATM_API SHALL only return ATMs where `is_active = true` and `deleted_at IS NULL`.
10. WHEN `page` is less than 1 or `page_size` is less than 1 or greater than 100 or `sort_by` is not in the allowed values list, THE ATM_API SHALL return HTTP 400 with a descriptive error message.

### Requirement 2: Replenishment Status Calculation

**User Story:** As an operations manager, I want ATMs automatically flagged based on their cash level relative to configured thresholds, so that I can prioritize replenishment scheduling without manual checks.

#### Acceptance Criteria

1. THE Replenishment_Flag_Engine SHALL compare the latest `refund_total` from `itm_cashpos` for each ATM against the ATM's `low_threshold_amount` and `critical_threshold_amount`.
2. WHEN the latest `refund_total` is less than or equal to `critical_threshold_amount`, THE Replenishment_Flag_Engine SHALL assign the status `critical` to the ATM.
3. WHEN the latest `refund_total` is greater than `critical_threshold_amount` but less than or equal to `low_threshold_amount`, THE Replenishment_Flag_Engine SHALL assign the status `low` to the ATM.
4. WHEN the latest `refund_total` is greater than `low_threshold_amount`, THE Replenishment_Flag_Engine SHALL assign the status `normal` to the ATM.
5. WHEN an ATM has `low_threshold_amount` set to NULL, THE Replenishment_Flag_Engine SHALL assign the status `unconfigured` to the ATM regardless of cashpos data.
6. WHEN an ATM has no records in `itm_cashpos`, THE Replenishment_Flag_Engine SHALL assign the status `no_data` to the ATM.
7. THE Replenishment_Flag_Engine SHALL use only the single most recent record per terminal (determined by `replenish_date` DESC, `replenish_time` DESC) from `itm_cashpos`.
8. WHEN an ATM has `critical_threshold_amount` set to NULL but `low_threshold_amount` is set, THE Replenishment_Flag_Engine SHALL only compare against `low_threshold_amount`, assigning `low` when refund_total <= low_threshold_amount and `normal` otherwise.

### Requirement 3: ATM Portal Page Layout

**User Story:** As an internal CMS user, I want a dedicated page to view all ATMs with their cash status, so that I can monitor the fleet and identify machines needing attention.

#### Acceptance Criteria

1. WHEN the user navigates to `/atm-portal`, THE ATM_Portal SHALL render the ATM monitoring view within the existing AppShell.
2. THE ATM_Portal SHALL render a PageHeader with the title "ATM Portal" and description "Monitor posisi kas dan status replenishment seluruh ATM".
3. THE ATM_Portal SHALL display summary cards above the table showing counts returned by the API in this order: Total (neutral color), Critical (danger), Low (warning), Normal (success).
4. THE ATM_Portal summary cards SHALL display the count values derived from the API response `summary` field which provides counts computed across all ATMs regardless of active filters or pagination.
5. THE ATM_Portal SHALL render the ATM_Table below the summary cards.

### Requirement 4: ATM Table Display

**User Story:** As an operations manager, I want to see ATM details and cash positions in a sortable, filterable table, so that I can quickly find and assess specific ATMs.

#### Acceptance Criteria

1. THE ATM_Table SHALL display the following columns: Terminal ID, Location, Machine Type, Brand, Deployment Type, Last Replenish Date, Refund Total, Threshold, Status.
2. THE ATM_Table Terminal ID column SHALL render values in monospace font.
3. THE ATM_Table Refund Total column SHALL format values as Indonesian Rupiah (e.g., "Rp 45.000.000") with right-alignment and tabular-nums, or a dash "—" when the value is NULL.
4. THE ATM_Table Threshold column SHALL display the `low_threshold_amount` formatted as Indonesian Rupiah with right-alignment and tabular-nums, or a dash "—" when the value is NULL.
5. THE ATM_Table Status column SHALL render a badge with icon and label: "Critical" (danger variant, `AlertTriangle` icon), "Low" (warning variant, `TrendingDown` icon), "Normal" (success variant, `CheckCircle` icon), "Unconfigured" (neutral variant, `Settings` icon), "No Data" (neutral variant, `HelpCircle` icon).
6. THE ATM_Table Last Replenish Date column SHALL format values in "dd MMM yyyy" Indonesian locale (e.g., "15 Jul 2026"), or a dash "—" when the value is NULL.
7. THE ATM_Table SHALL support client-triggered server-side sorting on Terminal ID, Location, Last Replenish Date, Refund Total, and Status columns, displaying an ascending or descending arrow indicator on the currently sorted column.
8. THE ATM_Table SHALL render pagination controls below the table showing current page, total pages, and a page size selector with options 10, 25, 50, and 100 rows per page (default 25).

### Requirement 5: ATM Table Filtering

**User Story:** As an operations manager, I want to filter the ATM list by various criteria, so that I can focus on specific subsets of ATMs relevant to my task.

#### Acceptance Criteria

1. THE ATM_Portal SHALL render a search input above the table that performs case-insensitive partial matching against terminal_id or location name, with a maximum input length of 100 characters.
2. WHEN the user types in the search input, THE ATM_Portal SHALL debounce the input by 300ms before triggering a new API request.
3. THE ATM_Portal SHALL render filter controls for: status (multi-select: critical, low, normal, unconfigured, no_data), machine type (multi-select: ATM, CRM, CDM), brand (multi-select: Hyosung, Wincor, Diebold), and deployment type (select: ONSITE, OFFSITE).
4. WHEN filters are applied, THE ATM_Portal SHALL update the URL query parameters to enable shareable filtered views, and WHEN the page loads with filter query parameters present in the URL, THE ATM_Portal SHALL pre-populate the corresponding filter controls and display the filtered results.
5. WHEN filters are active, THE ATM_Portal SHALL display an active filter count indicator and a "Clear All" action that resets all filter controls to their default unselected state and removes filter query parameters from the URL.
6. WHEN the applied filters or search yield zero matching results, THE ATM_Portal SHALL display an empty-state message indicating no ATMs match the current criteria.
7. IF the API request triggered by a filter or search change fails, THEN THE ATM_Portal SHALL display an error message indicating the failure and retain the previously loaded table data.

### Requirement 6: Navigation Integration

**User Story:** As an internal CMS user, I want to access the ATM Portal from the sidebar navigation, so that I can quickly navigate to the ATM monitoring view.

#### Acceptance Criteria

1. THE Sidebar SHALL display a "Monitoring" navigation group positioned after the "Umum" (general) group in the group order.
2. THE Sidebar SHALL display an "ATM Portal" link within the "Monitoring" navigation group with a `Monitor` lucide-react icon.
3. WHEN the user clicks the "ATM Portal" link, THE Sidebar SHALL navigate to the `/atm-portal` route.
4. WHEN the `/atm-portal` route is active, THE Sidebar SHALL highlight the "ATM Portal" link using the existing active style.

### Requirement 7: Data Freshness Indicator

**User Story:** As an operations manager, I want to know when the cash position data was last updated, so that I can assess whether the displayed information is current enough for decision-making.

#### Acceptance Criteria

1. THE ATM_Portal SHALL display a data freshness indicator above the ATM_Table showing the most recent `replenish_date` and `replenish_time` across the entire `itm_cashpos` table, regardless of active filters or pagination.
2. THE data freshness indicator SHALL format the timestamp in Indonesian locale using the pattern "Data terakhir: dd MMM yyyy, HH:mm" (e.g., "Data terakhir: 15 Jul 2026, 14:30").
3. THE ATM_API response for the `/api/v1/atm-portal/atms` endpoint SHALL include a `last_updated` field containing the most recent `replenish_date` and `replenish_time` combination from all records in `itm_cashpos`, computed independently of any request filter parameters.
4. IF no records exist in `itm_cashpos`, THEN THE ATM_API SHALL return `last_updated` as null and THE ATM_Portal SHALL display "Data terakhir: belum tersedia" in the freshness indicator position.
5. WHEN new ATM data is fetched from the API, THE ATM_Portal SHALL update the data freshness indicator to reflect the `last_updated` value from the latest response.

### Requirement 8: Error and Loading States

**User Story:** As a frontend user, I want clear feedback when data is loading or an error occurs, so that I understand the system state at all times.

#### Acceptance Criteria

1. WHILE the ATM data is loading, THE ATM_Portal SHALL display skeleton placeholders for the 4 summary cards and 5 table rows matching the ATM_Table column layout.
2. IF the ATM_API returns an error, THEN THE ATM_Portal SHALL replace the summary cards and table with an error state displaying an error icon, a user-friendly error message indicating the failure reason (e.g., "Gagal memuat data ATM"), and a "Coba Lagi" (retry) button.
3. WHEN the retry button is clicked, THE ATM_Portal SHALL return to the loading state and re-fetch the ATM data from the API.
4. WHEN the ATM_Table has no results matching the current filters, THE ATM_Portal SHALL display an empty state with the message "Tidak ada ATM yang sesuai filter" and a sub-text "Coba ubah atau hapus filter untuk melihat data" within the table body area.
5. IF the ATM_API returns an error while filters are active, THEN THE ATM_Portal SHALL preserve the current filter selections so the user can retry without re-entering filters.

### Requirement 9: Feature Module Structure

**User Story:** As a developer, I want the ATM Portal feature organized following project conventions, so that the codebase remains consistent and maintainable.

#### Acceptance Criteria

1. THE ATM Portal frontend feature directory SHALL reside at `src/features/atm-portal/` within the CompanyPortal-Vite project, using kebab-case naming consistent with existing feature directories.
2. THE ATM Portal page component SHALL reside in `src/features/atm-portal/AtmPortalScreen.tsx`, exporting a single default or named React component as the top-level page entry point.
3. THE ATM Portal data hook SHALL reside in `src/features/atm-portal/useAtmPortalData.ts`, exporting a custom hook that encapsulates data fetching via TanStack Query.
4. THE ATM Portal TypeScript types SHALL reside in `src/features/atm-portal/types.ts`, exporting all shared type definitions and interfaces used by the ATM Portal feature module.
5. THE ATM Portal feature module SHALL include a barrel export file at `src/features/atm-portal/index.ts` that re-exports the public API of the module.
6. THE ATM Portal backend handler SHALL reside in `backend/internal/handler/atm_portal_handler.go`.
7. THE ATM Portal backend service SHALL reside in `backend/internal/service/atm_portal.go`, containing business logic separated from HTTP concerns.
8. THE ATM Portal sqlc queries SHALL reside in `backend/queries/atm_portal.sql`, containing all SQL queries specific to the ATM Portal feature.
9. THE ATM Portal feature SHALL include a `src/features/atm-portal/__tests__/` directory for co-located frontend test files.

### Requirement 10: Responsive Layout

**User Story:** As an internal CMS user, I want the ATM Portal to remain usable on different screen sizes, so that I can monitor ATMs from various devices.

#### Acceptance Criteria

1. THE summary cards grid SHALL render as 4 columns on browser viewports 1024px wide and above, 2 columns on viewports between 768px and 1023px, and 1 column on viewports below 768px.
2. WHEN the ATM_Table content exceeds the available horizontal space within the AppShell main content area, THE ATM_Table container SHALL display a horizontal scrollbar and allow the user to scroll to reveal all columns without causing horizontal overflow on the page viewport.
3. THE ATM_Portal SHALL render all content within the existing AppShell main content area without producing a horizontal scrollbar on the page viewport at any width from 320px upward, and text content that exceeds its container width SHALL be truncated with an ellipsis or wrapped.
4. WHILE the viewport width is below 768px, THE ATM_Portal interactive controls (filters, action buttons) SHALL maintain a minimum touch-target size of 44×44 CSS pixels.

### Requirement 11: Accessibility

**User Story:** As a user relying on assistive technology, I want the ATM Portal to be accessible, so that I can understand ATM status data regardless of how I interact with the interface.

#### Acceptance Criteria

1. THE ATM_Table SHALL use `<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th scope="col">` for column headers, and `<td>` for data cells, with a `<caption>` element or `aria-label` identifying the table's purpose.
2. THE status badges SHALL convey status via icon and text label, not color alone, ensuring each badge contains a visible text string matching the status name.
3. THE summary cards SHALL each include an `aria-label` attribute containing the card's metric name and current numeric value (e.g., "Total ATM: 150", "Critical: 12").
4. THE pagination controls SHALL include `aria-label` attributes on navigation buttons indicating the target page and total pages (e.g., "Halaman 1 dari 10").
5. THE search input SHALL include a visible `<label>` element or `aria-label` attribute with the text "Cari berdasarkan Terminal ID atau lokasi".
6. THE ATM_Portal interactive elements SHALL be operable via keyboard using Tab for focus navigation and Enter or Space for activation, with a visible focus indicator.
7. WHEN the ATM_Table sort order changes, THE ATM_Table SHALL update the sorted column header with the `aria-sort` attribute set to "ascending" or "descending".
8. WHEN the ATM data transitions between loading, loaded, error, or empty states, THE ATM_Portal SHALL announce the new state to assistive technology using an ARIA live region with `aria-live="polite"`.
