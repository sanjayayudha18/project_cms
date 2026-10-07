# Requirements Document

## Introduction

The EOD Monitoring Frontend is a single-page admin screen within the CMS internal app (`frontend/CompanyPortal-Vite/`) that provides real-time visibility into the End-of-Day ETL file processing pipeline. It displays processing summaries, file status with retry history, late SLA alerts, and an audit log — all consumed from the Go backend proxy to the Python EOD Retry Scheduler microservice.

The page is a scrollable layout with four section cards (Summary, File Status Table, Late Alerts, Audit Log), supports 60-second polling with a toggle, and allows manual retry via confirmation dialog. Access is restricted to ADMIN and ADMIN_PARAM roles.

## Glossary

- **EOD_Monitoring_Page**: The single scrollable route at `/eod-monitoring` within the CompanyPortal-Vite frontend, restricted to ADMIN and ADMIN_PARAM roles
- **Summary_Section**: The top section of the page displaying aggregate counts of file processing states for the selected processing_date
- **File_Status_Table**: A TanStack Table v8 instance displaying per-file processing status grouped by file type, with sortable columns and row-click interaction
- **Late_Alerts_Section**: A card section showing file types that breached their SLA deadline for the selected processing_date
- **Audit_Log_Section**: A filterable table displaying retry audit trail entries for the selected processing_date
- **Retry_Drawer**: A side panel (drawer) that slides in from the right when a file row is clicked, showing the retry attempt history for that file
- **Retry_Confirmation_Dialog**: A modal dialog requiring explicit user confirmation before executing a manual retry POST request
- **Polling_Toggle**: A UI control that enables or disables automatic 60-second data refetch across all queries on the page
- **Processing_Date_Picker**: A date input control that determines the processing_date query parameter sent to all API endpoints
- **API_Client**: The existing `api` client in `lib/api/client.ts` that injects Bearer tokens and handles 401 refresh
- **Badge_Component**: The existing `Badge` UI component supporting variants: success, warning, danger, info, neutral
- **Status_Badge_Map**: The mapping from processing status to Badge variant — Completed→success, Failed→danger, max_retries_exhausted→danger, Processing→info, Late→warning, Pending→neutral
- **TanStack_Query**: TanStack Query v5 used for server state management, caching, and polling via `refetchInterval`
- **Go_Backend_Proxy**: The Go backend endpoints under `/api/eod/` that proxy requests to the Python Retry Scheduler microservice
- **Response_Envelope**: The API response structure `{ status: string, data: T | null, error: string | null }`

## Requirements

### Requirement 1: Route and Access Control

**User Story:** As a system administrator, I want the EOD Monitoring page to be accessible only to authorized admin roles, so that sensitive operational data is protected from unauthorized users.

#### Acceptance Criteria

1. THE EOD_Monitoring_Page SHALL be registered as a file-based route under the protected route layout at the path `/eod-monitoring`.
2. THE EOD_Monitoring_Page SHALL use the `requireRoles` guard with allowed roles `ADMIN` and `ADMIN_PARAM` in the route's `beforeLoad`.
3. WHEN a user with a role other than ADMIN or ADMIN_PARAM attempts to access `/eod-monitoring`, THE EOD_Monitoring_Page SHALL display a 403 Forbidden state.
4. WHEN an unauthenticated user attempts to access `/eod-monitoring`, THE EOD_Monitoring_Page SHALL redirect to `/login` with the original path preserved in the redirect search parameter.

### Requirement 2: Page Layout and Structure

**User Story:** As an operations support engineer, I want a single scrollable page with clearly separated sections, so that I can quickly scan the overall EOD status and drill into specific areas.

#### Acceptance Criteria

1. THE EOD_Monitoring_Page SHALL render a PageHeader component with the title "EOD Monitoring" and a subtitle describing the page purpose.
2. THE EOD_Monitoring_Page SHALL render four section cards in vertical order: Summary_Section, File_Status_Table, Late_Alerts_Section, Audit_Log_Section.
3. THE EOD_Monitoring_Page SHALL render a Processing_Date_Picker control in the page header area that defaults to the current date in Asia/Jakarta timezone.
4. THE EOD_Monitoring_Page SHALL render a Polling_Toggle control in the page header area that defaults to enabled (polling active).
5. WHEN the user changes the Processing_Date_Picker value, THE EOD_Monitoring_Page SHALL refetch all section data using the newly selected processing_date.

### Requirement 3: Summary Section

**User Story:** As an operations support engineer, I want to see aggregate counts of file processing states at a glance, so that I know immediately whether any files need attention.

#### Acceptance Criteria

1. THE Summary_Section SHALL fetch data from `GET /api/eod/summary?processing_date={date}` using TanStack_Query.
2. THE Summary_Section SHALL display SummaryCard components for each count: Pending, Processing, Completed, Failed, Max Retries Exhausted, and Late.
3. THE Summary_Section SHALL render SummaryCard values using `tabular-nums` for digit alignment.
4. THE Summary_Section SHALL display a loading skeleton while the query is in pending state.
5. IF the summary API request fails, THEN THE Summary_Section SHALL display an error state with the error message from the Response_Envelope `error` field.

### Requirement 4: File Status Table

**User Story:** As an operations support engineer, I want to see the processing status of every file for the selected date with sortable columns, so that I can identify which files are problematic and prioritize intervention.

#### Acceptance Criteria

1. THE File_Status_Table SHALL fetch data from `GET /api/eod/status?processing_date={date}` using TanStack_Query.
2. THE File_Status_Table SHALL render a DataTable with columns: File Type, Filename, Status (as Badge), Retry Count, Detected At, Last Retry At, and Failure Reason.
3. THE File_Status_Table SHALL map the `processing_status` field to Badge_Component variants using Status_Badge_Map: `completed`→success, `failed`→danger, `max_retries_exhausted`→danger, `processing`→info, `pending`→neutral.
4. THE File_Status_Table SHALL display Status badges with both an icon and label text, never by color alone.
5. THE File_Status_Table SHALL render the Detected At and Last Retry At columns formatted in Asia/Jakarta timezone with format `DD MMM YYYY HH:mm`.
6. THE File_Status_Table SHALL support column sorting on File Type, Status, Retry Count, and Detected At columns.
7. WHEN a user clicks a row in the File_Status_Table, THE EOD_Monitoring_Page SHALL open the Retry_Drawer for the selected file.
8. THE File_Status_Table SHALL display an EmptyState component when no files exist for the selected processing_date.
9. THE File_Status_Table SHALL render the Retry Count column right-aligned with `tabular-nums` styling.

### Requirement 5: Retry History Drawer

**User Story:** As an operations support engineer, I want to see the full retry history for a file in a side drawer, so that I can understand what has been attempted and decide whether to trigger a manual retry.

#### Acceptance Criteria

1. WHEN the Retry_Drawer is opened for a file, THE Retry_Drawer SHALL fetch data from `GET /api/eod/status/{file_id}/history` using TanStack_Query.
2. THE Retry_Drawer SHALL slide in from the right side of the viewport with a 300ms ease-out transition animating `transform` and `opacity`.
3. THE Retry_Drawer SHALL display the file metadata: filename, file type, checksum (truncated to first 12 characters), and current processing status as a Badge.
4. THE Retry_Drawer SHALL display a list of retry attempts showing: attempt number, trigger type (Auto/Manual as Badge), started at, duration (milliseconds), outcome (as Badge), and error detail (if failed).
5. THE Retry_Drawer SHALL render a "Retry Manual" Button (primary variant) at the bottom of the drawer when the file's processing_status is `failed` or `max_retries_exhausted`.
6. WHEN the file's processing_status is `completed`, THE Retry_Drawer SHALL NOT render the manual retry button.
7. WHEN the file's processing_status is `processing`, THE Retry_Drawer SHALL render the manual retry button in disabled state.
8. THE Retry_Drawer SHALL render a close button (X icon) in the top-right corner with a minimum 44x44px touch target.
9. WHEN the user clicks outside the Retry_Drawer or presses Escape, THE Retry_Drawer SHALL close with an exit transition at 75% of the enter duration (225ms).
10. THE Retry_Drawer SHALL display a loading skeleton while the history query is in pending state.

### Requirement 6: Manual Retry with Confirmation

**User Story:** As an operations support engineer, I want a confirmation step before triggering a manual retry, so that I do not accidentally re-run ETL processing.

#### Acceptance Criteria

1. WHEN the user clicks the "Retry Manual" button in the Retry_Drawer, THE EOD_Monitoring_Page SHALL display the Retry_Confirmation_Dialog.
2. THE Retry_Confirmation_Dialog SHALL display the filename, file type, and current retry count of the target file.
3. THE Retry_Confirmation_Dialog SHALL display a warning message explaining that a manual retry will re-execute the ETL script for the file.
4. THE Retry_Confirmation_Dialog SHALL render a "Konfirmasi Retry" primary button and a "Batal" secondary button.
5. WHEN the user clicks "Konfirmasi Retry", THE EOD_Monitoring_Page SHALL send a `POST /api/eod/retry/{file_id}` request using the API_Client.
6. WHEN the manual retry POST request succeeds, THE EOD_Monitoring_Page SHALL close the dialog, display a success Toast, and invalidate the status and summary queries to trigger refetch.
7. IF the manual retry POST request fails with HTTP 409, THEN THE EOD_Monitoring_Page SHALL close the dialog and display an error Toast with the message from the Response_Envelope `error` field.
8. IF the manual retry POST request fails with any other error, THEN THE EOD_Monitoring_Page SHALL close the dialog and display an error Toast with the error message.
9. WHILE the manual retry POST request is in-flight, THE Retry_Confirmation_Dialog SHALL disable both buttons and show a loading indicator on the confirm button.

### Requirement 7: Late Alerts Section

**User Story:** As an operations support engineer, I want to see which file types have breached their SLA deadline, so that I can escalate upstream data delivery issues.

#### Acceptance Criteria

1. THE Late_Alerts_Section SHALL fetch data from `GET /api/eod/late?processing_date={date}` using TanStack_Query.
2. THE Late_Alerts_Section SHALL display each late detection record as a card or row containing: file type, SLA deadline, detected at timestamp, and resolution status.
3. THE Late_Alerts_Section SHALL render unresolved late records with a warning Badge displaying "Late" and an icon.
4. THE Late_Alerts_Section SHALL render resolved late records with a success Badge displaying "Resolved" and an icon.
5. THE Late_Alerts_Section SHALL display the SLA deadline formatted as `HH:mm WIB`.
6. WHEN no late detections exist for the selected processing_date, THE Late_Alerts_Section SHALL display an EmptyState with the message "Tidak ada file terlambat".
7. THE Late_Alerts_Section SHALL display a loading skeleton while the query is in pending state.

### Requirement 8: Audit Log Section

**User Story:** As a compliance officer, I want to view and filter the audit trail of retry actions, so that I can verify who triggered retries and their outcomes for regulatory review.

#### Acceptance Criteria

1. THE Audit_Log_Section SHALL fetch data from `GET /api/eod/audit?processing_date={date}` using TanStack_Query, passing optional `file_type` and `trigger` filter parameters.
2. THE Audit_Log_Section SHALL render FilterSelect controls for File Type (options: DMAA, ITM Cash Position, ITM Replenishment, Semua) and Trigger Type (options: Auto, Manual, Semua).
3. WHEN the user changes a filter value, THE Audit_Log_Section SHALL refetch audit data with the updated filter parameters.
4. THE Audit_Log_Section SHALL render a DataTable with columns: Timestamp, Event Type, Trigger (as Badge), File Type, Initiated By, Outcome (as Badge), Duration, and Error Detail.
5. THE Audit_Log_Section SHALL map trigger types to Badge variants: `auto`→info, `manual`→neutral.
6. THE Audit_Log_Section SHALL map outcome values to Badge variants: `completed`→success, `failed`→danger, null (in-progress)→info.
7. THE Audit_Log_Section SHALL display timestamps in Asia/Jakarta timezone with format `DD MMM YYYY HH:mm:ss`.
8. THE Audit_Log_Section SHALL display the Duration column right-aligned with `tabular-nums` styling, formatted as milliseconds with "ms" suffix.
9. WHEN no audit records exist for the selected filters, THE Audit_Log_Section SHALL display an EmptyState with the message "Tidak ada log audit".

### Requirement 9: Polling and Data Freshness

**User Story:** As an operations support engineer, I want the page to automatically refresh data every 60 seconds, so that I see near-real-time processing status without manually refreshing the browser.

#### Acceptance Criteria

1. WHILE the Polling_Toggle is enabled, THE EOD_Monitoring_Page SHALL configure all TanStack_Query instances with a `refetchInterval` of 60000 milliseconds.
2. WHILE the Polling_Toggle is disabled, THE EOD_Monitoring_Page SHALL set `refetchInterval` to `false` on all TanStack_Query instances, stopping automatic refetch.
3. THE Polling_Toggle SHALL display its current state visually — active (polling on) or inactive (polling off) — using a toggle switch or similar control.
4. THE EOD_Monitoring_Page SHALL display a "Last updated" timestamp showing the most recent successful data fetch time, formatted in Asia/Jakarta timezone.
5. WHEN the user manually changes the Processing_Date_Picker, THE EOD_Monitoring_Page SHALL immediately refetch all queries regardless of polling state.

### Requirement 10: Loading and Error States

**User Story:** As an operations support engineer, I want clear visual feedback during data loading and when errors occur, so that I can distinguish between "no data" and "system error."

#### Acceptance Criteria

1. WHILE any section query is in pending state on initial load, THE EOD_Monitoring_Page SHALL render skeleton placeholders matching the expected layout dimensions for that section.
2. IF a section query fails, THEN THE EOD_Monitoring_Page SHALL display an inline error message within that section card containing the error text from the Response_Envelope `error` field, without affecting other sections.
3. THE EOD_Monitoring_Page SHALL isolate failures per section — a failed summary query SHALL NOT prevent the File_Status_Table from rendering its data.
4. WHEN a section is in error state, THE EOD_Monitoring_Page SHALL render a "Coba Lagi" (Retry) button that triggers a manual refetch for that section's query.
5. WHILE a manual retry POST request is in-flight, THE EOD_Monitoring_Page SHALL render a loading spinner on the confirm button and disable form interaction in the Retry_Confirmation_Dialog.

### Requirement 11: Responsive Layout and Accessibility

**User Story:** As a user accessing the page from different screen sizes, I want the layout to remain usable and accessible, so that I can operate the monitoring page effectively.

#### Acceptance Criteria

1. THE EOD_Monitoring_Page SHALL render the Summary_Section cards in a responsive grid: 3 columns on desktop (≥1024px), 2 columns on tablet (≥768px), and 1 column on mobile (<768px).
2. THE EOD_Monitoring_Page SHALL ensure all interactive elements (buttons, toggles, table rows, drawer close) have a minimum touch target of 44x44 CSS pixels.
3. THE EOD_Monitoring_Page SHALL use semantic HTML landmarks: `main` for page content, `section` with accessible labels for each card section, `table` elements within DataTable.
4. THE Retry_Drawer SHALL trap keyboard focus while open and return focus to the triggering row when closed.
5. THE Retry_Confirmation_Dialog SHALL trap keyboard focus while open and return focus to the retry button when closed.
6. THE EOD_Monitoring_Page SHALL announce dynamic content changes (toast notifications, polling updates) via an `aria-live` region.
7. THE EOD_Monitoring_Page SHALL use the design system's OKLCH color tokens and CIMB Red palette exclusively — no hardcoded hex or RGB values.

### Requirement 12: Feature Module Organization

**User Story:** As a developer, I want the EOD Monitoring feature organized following the project's feature-based structure, so that code is maintainable and discoverable.

#### Acceptance Criteria

1. THE EOD_Monitoring_Page feature code SHALL reside in `src/features/eod-monitoring/` following the project's feature-based organization pattern.
2. THE EOD_Monitoring_Page SHALL define TypeScript types for all API response shapes in a dedicated types file within the feature module.
3. THE EOD_Monitoring_Page SHALL define TanStack Query hooks (useEodSummary, useEodStatus, useFileHistory, useEodLate, useEodAudit, useRetryFile) in a dedicated hooks file within the feature module.
4. THE EOD_Monitoring_Page SHALL reuse existing shared components (Badge, SummaryCard, DataTable, PageHeader, FilterSelect, Toast, Button, Card, EmptyState) without duplicating them.
5. THE EOD_Monitoring_Page SHALL define feature-specific components (RetryDrawer, RetryConfirmationDialog, LateAlertCard, PollingToggle) within the feature module's components directory.
