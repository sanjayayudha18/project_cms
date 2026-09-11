# Requirements Document

## Introduction

The ATM Profile page provides CMS internal users with a single-ATM detail view, accessible by clicking a terminal ID in the ATM Portal list. It consolidates historical replenishment data (`itm_replenish`) and cash position snapshots (`itm_cashpos`) for one specific ATM, enabling operations teams to review an individual machine's cash activity over time. This is a read-only feature reusing the same authenticated/RBAC boundary as the existing ATM Portal.

## Glossary

- **ATM_Profile**: The React page component rendered at the `/atm-portal/$terminalId` route, displaying combined replenishment history and cash position data for a single ATM
- **ATM_Profile_API**: The Go backend REST API handler serving ATM Profile endpoints under `/api/v1/atm-portal/atms/:terminalId`
- **ATM_Portal**: The existing ATM Portal list page at `/atm-portal` displaying all ATMs with filtering, sorting, and pagination
- **Replenish_Table**: A TanStack Table component displaying historical replenishment records from `itm_replenish` for the selected ATM
- **Cashpos_Table**: A TanStack Table component displaying historical cash position snapshots from `itm_cashpos` for the selected ATM
- **Terminal_ID**: The unique text identifier (`atms.terminal_id`) used as the URL parameter and join key across `itm_replenish` and `itm_cashpos`
- **ATM_Header**: The top section of the ATM Profile page displaying ATM master data (terminal ID, location, machine type, brand, model, deployment type, operation hours, thresholds)
- **Tab_Navigation**: The UI control allowing users to switch between Replenish and Cashpos data views within the ATM Profile page

## Requirements

### Requirement 1: Navigation from ATM Portal to ATM Profile

**User Story:** As an operations manager, I want to click on a terminal ID in the ATM Portal list to navigate to that ATM's profile page, so that I can quickly drill down into individual ATM details.

#### Acceptance Criteria

1. THE ATM_Portal terminal ID column SHALL render each terminal ID as a clickable link navigating to `/atm-portal/$terminalId`, visually distinguished from non-interactive text using the standard link style.
2. WHEN the user clicks a terminal ID link in the ATM_Portal, THE ATM_Portal SHALL navigate to the ATM Profile page for the selected terminal using client-side routing without a full page reload, and update the browser URL to `/atm-portal/$terminalId`.
3. THE ATM_Profile page SHALL display a breadcrumb navigation showing "ATM Portal" as a link back to `/atm-portal` followed by the current terminal ID as the active breadcrumb item.
4. WHEN the user clicks the "ATM Portal" breadcrumb link, THE ATM_Profile SHALL navigate back to the ATM Portal list page using client-side routing without a full page reload.
5. WHEN the user navigates to `/atm-portal/$terminalId` and the terminal ID does not exist in the system, THE ATM_Profile page SHALL display an error indication stating that the terminal was not found, along with a link to navigate back to the ATM Portal list.
6. WHEN the user navigates to the ATM Profile page via a terminal ID link, THE ATM_Portal SHALL push the route onto browser history so that the browser back button returns the user to the ATM Portal list at the previous scroll position.

### Requirement 2: ATM Profile Header

**User Story:** As an operations manager, I want to see the ATM's master data at the top of the profile page, so that I can identify the machine and understand its configuration at a glance.

#### Acceptance Criteria

1. WHEN the ATM Profile page loads for a valid terminal_id, THE ATM_Header SHALL display the following ATM master data fields: Terminal ID, Location Name, Location Address, Machine Type, Brand, Model, Deployment Type, Operation Hours (displaying the stored value such as "24_HOURS" or "BUSINESS_HOURS"), Capacity Amount, Low Threshold Amount, and Critical Threshold Amount.
2. THE ATM_Header SHALL format monetary fields (Capacity Amount, Low Threshold Amount, Critical Threshold Amount) as Indonesian Rupiah with dot-separated thousands and "Rp" prefix (e.g., "Rp 100.000.000"), with right-alignment and tabular-nums font-variant.
3. WHEN a monetary field value is NULL, THE ATM_Header SHALL display an em dash "—" in place of the value.
4. THE ATM_Header Terminal ID SHALL render in monospace font consistent with the ATM Portal list styling.
5. THE ATM_Header SHALL display the ATM's current replenishment status as a badge using the same icon, label, and variant mapping as the ATM Portal list (Critical/danger, Low/warning, Normal/success, Unconfigured/neutral, No Data/neutral), where status is derived from: unconfigured when low_threshold_amount is NULL, no_data when no itm_replenish record exists, critical when refund_total ≤ critical_threshold_amount, low when refund_total ≤ low_threshold_amount, otherwise normal.
6. IF the requested terminal_id does not exist or belongs to an inactive ATM (is_active = false), THEN THE System SHALL display an error indication informing the user that the ATM was not found, without rendering the header fields.
7. WHILE the ATM Profile page is fetching data, THE ATM_Header SHALL display a loading skeleton placeholder matching the layout dimensions of the header fields.

### Requirement 3: ATM Profile API — Master Data Endpoint

**User Story:** As a frontend developer, I want an API endpoint that returns ATM master data for a specific terminal, so that the ATM Profile header can display configuration details.

#### Acceptance Criteria

1. WHEN a GET request is made to `/api/v1/atm-portal/atms/:terminalId`, THE ATM_Profile_API SHALL return HTTP 200 with the ATM master data in the established `pkg/response` envelope, including these fields: terminal_id, location_name, address, machine_type, brand, model, deployment_type, operation_hours, capacity_amount, low_threshold_amount, critical_threshold_amount, is_active, and replenishment_status. Money fields (capacity_amount, low_threshold_amount, critical_threshold_amount) SHALL be serialized as decimal strings, and nullable fields SHALL serialize as JSON `null` when no value exists.
2. IF the `:terminalId` path parameter does not match any record in the `atms` table, or the matched ATM has `is_active = false`, or the matched ATM has `deleted_at IS NOT NULL`, THEN THE ATM_Profile_API SHALL return HTTP 404 with error code "not_found" and message "ATM tidak ditemukan".
3. THE ATM_Profile_API SHALL compute replenishment_status by selecting the latest `itm_replenish` record for the terminal (ordered by replenish_date DESC, replenish_time DESC) and applying the following precedence: "unconfigured" when low_threshold_amount IS NULL; "no_data" when no itm_replenish record exists; "critical" when critical_threshold_amount IS NOT NULL AND refund_total <= critical_threshold_amount; "low" when refund_total <= low_threshold_amount; otherwise "normal".
4. THE ATM_Profile_API SHALL only be accessible to authenticated users, enforced by the same `RequireAuth` middleware applied to the existing ATM Portal route group.
5. IF the `:terminalId` path parameter is empty or contains only whitespace, THEN THE ATM_Profile_API SHALL return HTTP 400 with error code "bad_request" and a message indicating the terminal ID is required.
6. WHEN the endpoint processes the request successfully, THE ATM_Profile_API SHALL respond within 500ms under normal database load for this single-record lookup.

### Requirement 4: ATM Profile API — Replenishment History Endpoint

**User Story:** As a frontend developer, I want a paginated API endpoint that returns replenishment history for a specific ATM, so that the ATM Profile can display historical replenishment data.

#### Acceptance Criteria

1. WHEN a GET request is made to `/api/v1/atm-portal/atms/:terminalId/replenish`, THE ATM_Profile_API SHALL return a paginated list of records from `itm_replenish` filtered by the specified terminal_id.
2. THE ATM_Profile_API SHALL accept query parameters for pagination: `page` (default 1) and `page_size` (default 25, maximum 100).
3. THE ATM_Profile_API SHALL accept optional `date_from` and `date_to` query parameters in the format YYYY-MM-DD.
4. WHEN `date_from` and/or `date_to` are provided with valid YYYY-MM-DD values, THE ATM_Profile_API SHALL filter records where `replenish_date` falls within the inclusive date range.
5. IF `date_from` or `date_to` is provided but is not a valid YYYY-MM-DD date string, THEN THE ATM_Profile_API SHALL return HTTP 400 with an error message indicating the invalid date format.
6. IF `date_from` is after `date_to`, THEN THE ATM_Profile_API SHALL return an empty `data` array with `total` set to 0.
7. THE ATM_Profile_API SHALL return results sorted by `replenish_date` descending and `replenish_time` descending (most recent first).
8. THE ATM_Profile_API response SHALL include `data` (array of replenishment records), `total` (total matching records count), `page`, and `page_size` fields. Each record in `data` SHALL contain: `replenish_date` (YYYY-MM-DD string), `replenish_time` (HH:MM:SS string), `terminal_id`, `machine_type`, `teller_id`, `branch_code`, `escrow`, `refund_total`, `replenish_total`, `refund_denom_50k`, `refund_denom_100k`, `replenish_denom_50k`, and `replenish_denom_100k`. All monetary fields SHALL be serialized as decimal strings.
9. WHEN the requested terminal_id does not exist in the `atms` table, THE ATM_Profile_API SHALL return HTTP 404 with an error message indicating ATM not found.
10. IF `page` is less than 1 or `page_size` is less than 1 or greater than 100, THEN THE ATM_Profile_API SHALL return HTTP 400 with a descriptive error message.
11. WHEN no replenishment records exist for the specified terminal_id, THE ATM_Profile_API SHALL return an empty `data` array with `total` set to 0.

### Requirement 5: ATM Profile API — Cash Position History Endpoint

**User Story:** As a frontend developer, I want a paginated API endpoint that returns cash position history for a specific ATM, so that the ATM Profile can display historical cashpos data.

#### Acceptance Criteria

1. WHEN a GET request is made to `/api/v1/atm-portal/atms/:terminalId/cashpos`, THE ATM_Profile_API SHALL return a paginated list of records from `itm_cashpos` filtered by the specified terminal_id, sorted by `cashpos_date` descending (most recent first).
2. THE ATM_Profile_API SHALL accept query parameters for pagination: `page` (integer, default 1, minimum 1) and `page_size` (integer, default 25, minimum 1, maximum 100).
3. WHEN optional `date_from` and/or `date_to` query parameters are provided (format: YYYY-MM-DD), THE ATM_Profile_API SHALL filter records by `cashpos_date` within the inclusive date range. WHEN only `date_from` is provided, THE ATM_Profile_API SHALL return records from that date onward. WHEN only `date_to` is provided, THE ATM_Profile_API SHALL return records up to and including that date.
4. THE ATM_Profile_API response SHALL include: `data` (array of cash position record objects), `total` (integer count of all matching records), `page` (current page number), and `page_size` (current page size). Each record object in `data` SHALL include all columns from `itm_cashpos` (id, file_id, cashpos_date, terminal_id, machine_type, teller_id, branch_code, all 16 denomination fields, position_source, created_at), with denomination fields represented as decimal strings with 2 decimal places (e.g., "1500000.00").
5. WHEN the requested terminal_id does not exist in the `atms` table, THE ATM_Profile_API SHALL return HTTP 404 with an error message indicating the ATM was not found.
6. IF `page` or `page_size` is not a valid integer, or `page` is less than 1, or `page_size` is less than 1 or greater than 100, THEN THE ATM_Profile_API SHALL return HTTP 400 with an error message indicating the invalid parameter.
7. IF `date_from` or `date_to` is provided but does not match YYYY-MM-DD format or is not a valid calendar date, THEN THE ATM_Profile_API SHALL return HTTP 400 with an error message indicating the invalid date format.
8. IF both `date_from` and `date_to` are provided and `date_from` is later than `date_to`, THEN THE ATM_Profile_API SHALL return HTTP 400 with an error message indicating the invalid date range.
9. WHEN no cash position records exist for the specified terminal_id (or within the filtered date range), THE ATM_Profile_API SHALL return HTTP 200 with an empty `data` array and `total` set to 0.

### Requirement 6: ATM Profile Page Layout

**User Story:** As an internal CMS user, I want a clear, organized ATM Profile page that shows both replenishment history and cash position data, so that I can review all relevant cash data for a specific ATM in one place.

#### Acceptance Criteria

1. WHEN the user navigates to `/atm-portal/$terminalId`, THE ATM_Profile SHALL render the ATM detail view within the existing AppShell.
2. THE ATM_Profile SHALL render a PageHeader with the eyebrow "ATM Portal", the title set to the terminal ID, and the description set to the ATM's location name; IF the location name is unavailable or empty, THEN THE ATM_Profile SHALL display a dash "—" as the PageHeader description.
3. THE ATM_Profile SHALL render the ATM_Header section below the PageHeader displaying ATM master data.
4. THE ATM_Profile SHALL render the Tab_Navigation below the ATM_Header with two tabs labeled "Replenish" and "Cashpos" in that order (left to right).
5. WHEN the "Replenish" tab is active, THE ATM_Profile SHALL display the Replenish_Table showing replenishment history for the current ATM and hide the Cashpos_Table.
6. WHEN the "Cashpos" tab is active, THE ATM_Profile SHALL display the Cashpos_Table showing cash position history for the current ATM and hide the Replenish_Table.
7. WHEN the ATM_Profile page first loads without a tab query parameter, THE Tab_Navigation SHALL activate the "Replenish" tab by default.
8. WHEN the user activates a tab, THE ATM_Profile SHALL update the URL query parameter `tab` to the active tab value (`replenish` or `cashpos`), and WHEN the page loads with a valid `tab` query parameter, THE ATM_Profile SHALL activate the corresponding tab.

### Requirement 7: Replenishment History Table

**User Story:** As an operations manager, I want to see the replenishment history for a specific ATM in a paginated table, so that I can review past replenishment events and their cash amounts.

#### Acceptance Criteria

1. THE Replenish_Table SHALL display the following columns: Replenish Date, Replenish Time, Refund Denom 10k, Refund Denom 20k, Refund Denom 50k, Refund Denom 100k, Refund Total, Replenish Denom 10k, Replenish Denom 20k, Replenish Denom 50k, Replenish Denom 100k, Replenish Total, and Escrow, sourced from the `itm_replenish` table filtered by the selected ATM's `terminal_id`.
2. THE Replenish_Table date column SHALL format values in "dd MMM yyyy" Indonesian locale (e.g., "15 Jul 2026").
3. THE Replenish_Table monetary columns SHALL format values as Indonesian Rupiah (e.g., "Rp 45.000.000") with right-alignment and tabular-nums.
4. THE Replenish_Table SHALL render pagination controls below the table showing current page, total pages, and a page size selector with options 10, 25, 50, and 100 rows per page (default 25).
5. THE Replenish_Table SHALL display a date range filter above the table allowing users to filter records by `replenish_date` range, defaulting to the last 30 days when no range is explicitly selected.
6. THE Replenish_Table SHALL sort records by `replenish_date` descending then `replenish_time` descending (most recent first) as the default sort order.
7. WHEN no replenishment records match the current filter criteria (including the default date range), THE Replenish_Table SHALL display an empty state with the message "Belum ada data replenish untuk ATM ini".
8. IF the API request to fetch replenishment data fails, THEN THE Replenish_Table SHALL display an error message indicating the data could not be loaded and provide a retry action.

### Requirement 8: Cash Position History Table

**User Story:** As an operations manager, I want to see the cash position snapshots for a specific ATM in a paginated table, so that I can review how cash levels have changed over time.

#### Acceptance Criteria

1. THE Cashpos_Table SHALL display the following columns: Cashpos Date, Teller ID, Branch Code, Position Source, and denomination-level columns grouped by denomination (10K, 20K, 50K, 100K) showing Starting Cash, Cash In, Cash Out, and Cash Position for each.
2. THE Cashpos_Table date column SHALL format values in "dd MMM yyyy" Indonesian locale (e.g., "15 Jul 2026").
3. THE Cashpos_Table denomination columns SHALL format values as whole numbers (no decimal places) with thousand separators (e.g., "1.250.000") and right-alignment using tabular-nums.
4. THE Cashpos_Table SHALL render pagination controls below the table showing current page, total pages, and a page size selector with options 10, 25, 50, and 100 rows per page (default 25).
5. THE Cashpos_Table SHALL display a date range filter above the table allowing users to filter records by cashpos_date range, defaulting to the last 30 days on initial load.
6. IF the user selects a date range where the end date is earlier than the start date, THEN THE Cashpos_Table SHALL disable the apply/search action and display a validation message indicating the end date must be equal to or later than the start date.
7. THE Cashpos_Table SHALL sort rows by cashpos_date descending (newest first) by default.
8. WHEN no cash position records exist for the current filter criteria, THE Cashpos_Table SHALL display an empty state with the message "Belum ada data cashpos untuk ATM ini".
9. WHEN the Cashpos_Table content exceeds the available horizontal space, THE Cashpos_Table container SHALL display a horizontal scrollbar to reveal all denomination columns.
10. WHILE the Cashpos_Table is fetching data from the server, THE Cashpos_Table SHALL display a loading skeleton or spinner in place of the table rows.

### Requirement 9: Error and Loading States

**User Story:** As a frontend user, I want clear feedback when ATM Profile data is loading or an error occurs, so that I understand the system state at all times.

#### Acceptance Criteria

1. WHILE the ATM master data is loading, THE ATM_Profile SHALL display skeleton placeholders (animated pulse rows) for the ATM_Header fields and Tab_Navigation area, rendering a minimum of 3 skeleton rows for header metadata.
2. IF the ATM_Profile_API returns a 404 error for the master data endpoint, THEN THE ATM_Profile SHALL display a full-page not-found state with the message "ATM tidak ditemukan" and a link to return to the ATM Portal list.
3. IF the ATM_Profile_API returns an HTTP 5xx error or a network timeout for the master data endpoint, THEN THE ATM_Profile SHALL display a full-page error state with a message indicating the ATM data failed to load, an AlertCircle icon in --danger-fg color, and a "Coba Lagi" (retry) button.
4. WHILE a data table (Replenish or Cashpos) is loading, THE ATM_Profile SHALL display 5 skeleton rows matching the respective table column count and layout.
5. IF the replenishment or cash position history API returns an error, THEN THE ATM_Profile SHALL display an inline error message within the affected table area with an AlertCircle icon and a "Coba Lagi" (retry) button, while preserving the ATM_Header display unchanged.
6. WHEN the retry button is clicked, THE ATM_Profile SHALL immediately display the loading state (skeleton placeholders) for the failed section and re-invoke the TanStack Query refetch for the failed endpoint.
7. IF the ATM_Profile_API request does not receive a response within 30 seconds, THEN THE ATM_Profile SHALL treat the request as failed and display the corresponding error state as defined in criterion 3 or criterion 5 depending on which section timed out.

### Requirement 10: Feature Module Structure

**User Story:** As a developer, I want the ATM Profile feature organized following project conventions, so that the codebase remains consistent and maintainable.

#### Acceptance Criteria

1. THE ATM Profile frontend components SHALL reside within the existing `src/features/atm-portal/` directory, sharing the feature module with the ATM Portal list since both belong to the same domain.
2. THE ATM Profile page component SHALL reside in `src/features/atm-portal/AtmProfileScreen.tsx`, exporting a named React component `AtmProfileScreen` as the page entry point.
3. THE ATM Profile data hooks SHALL reside in `src/features/atm-portal/useAtmProfileData.ts`, exporting three named custom hooks: `useAtmMasterData` (fetches ATM master data), `useAtmReplenishHistory` (fetches paginated replenishment records), and `useAtmCashposHistory` (fetches paginated cash position records), each encapsulating data fetching via TanStack Query.
4. THE ATM Profile TypeScript types SHALL be added to the existing `src/features/atm-portal/types.ts` file.
5. THE ATM Profile route SHALL be defined in `src/routes/atm-portal.$terminalId.tsx` following TanStack Router file-based routing conventions for dynamic segments.
6. THE ATM Profile backend endpoints SHALL be added to the existing `backend/internal/handler/atm_portal_handler.go` handler, mounted as sub-routes under the existing ATM Portal router at paths `:terminalId`, `:terminalId/replenish`, and `:terminalId/cashpos`.
7. THE ATM Profile sqlc queries SHALL be added to the existing `backend/queries/atm_portal.sql` file.
8. THE ATM Profile sub-components (ATM_Header, Replenish_Table, Cashpos_Table, Tab_Navigation) SHALL reside in `src/features/atm-portal/components/` directory, each as a separate file exporting a named React component.

### Requirement 11: Responsive Layout

**User Story:** As an internal CMS user, I want the ATM Profile page to remain usable on different screen sizes, so that I can review ATM data from various devices.

#### Acceptance Criteria

1. WHILE the viewport width is 1024px or above, THE ATM_Header fields grid SHALL render as a 3-column layout; WHILE the viewport width is between 768px and 1023px, THE ATM_Header fields grid SHALL render as a 2-column layout; WHILE the viewport width is below 768px, THE ATM_Header fields grid SHALL render as a single-column stacked layout.
2. WHEN the Cashpos_Table or Replenish_Table content exceeds the available horizontal space, THE table container SHALL display a horizontal scrollbar without causing horizontal overflow on the page viewport.
3. WHILE the viewport width is 320px or above, THE ATM_Profile SHALL render all content within the existing AppShell main content area without producing a horizontal scrollbar on the page viewport.
4. WHILE the viewport width is below 768px, THE ATM_Profile interactive controls (tabs, pagination, date filters) SHALL maintain a minimum touch-target size of 44×44 CSS pixels.

### Requirement 12: Accessibility

**User Story:** As a user relying on assistive technology, I want the ATM Profile page to be accessible, so that I can navigate and understand ATM data regardless of how I interact with the interface.

#### Acceptance Criteria

1. THE Tab_Navigation SHALL use an accessible tab pattern with `role="tablist"` on the container, `role="tab"` on each tab button, and `role="tabpanel"` on the active content area, with `aria-selected="true"` on the active tab and `aria-selected="false"` on inactive tabs, `aria-controls` on each tab referencing its panel's `id`, and `aria-labelledby` on each panel referencing its controlling tab's `id`.
2. THE Tab_Navigation SHALL support keyboard navigation: Arrow Left/Right to move focus between tabs, Enter or Space to activate the focused tab, Home to move focus to the first tab, End to move focus to the last tab, and WHEN a tab is activated, THE Tab_Navigation SHALL move DOM focus to the newly displayed tabpanel.
3. THE Replenish_Table and Cashpos_Table SHALL use semantic HTML table elements (`<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th scope="col">`, `<td>`) with a `<caption>` element or `aria-label` identifying the table's dataset name (e.g., "Riwayat Replenish" or "Riwayat Cash Position").
4. THE breadcrumb navigation SHALL use a `<nav aria-label="Breadcrumb">` element with an ordered list (`<ol>`) and `aria-current="page"` on the current terminal ID item.
5. THE ATM_Header monetary values SHALL include `aria-label` attributes providing the field label followed by the numeric value with currency (e.g., "Low threshold: Rp 50.000.000"), and WHEN a monetary value is NULL, THE `aria-label` SHALL indicate the absence (e.g., "Low threshold: tidak tersedia").
6. WHEN a table transitions between loading, loaded, error, or empty states, THE ATM_Profile SHALL announce the new state using an ARIA live region (`aria-live="polite"`, `aria-atomic="true"`) with distinct announcement text per state: loading ("Memuat data [dataset]…"), error ("Gagal memuat data [dataset]"), empty ("Tidak ada data [dataset] yang sesuai filter"), and loaded ("Menampilkan [count] baris [dataset]").
7. THE pagination controls SHALL include `aria-label` attributes on Previous/Next navigation buttons indicating the target page number and total pages (e.g., "Halaman 2 dari 10"), and WHEN a navigation button is disabled, THE `aria-label` SHALL indicate no further navigation is available (e.g., "Tidak ada halaman sebelumnya").
8. THE ATM_Profile SHALL ensure all interactive elements (tabs, pagination buttons, date filter inputs, breadcrumb links) display a visible focus indicator on `:focus-visible` consisting of a 2px ring offset distinguishable from the surrounding background with a contrast ratio of at least 3:1 against adjacent colors.
