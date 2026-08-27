# Implementation Plan: EOD Monitoring Frontend

## Overview

Build the EOD Monitoring admin page within `frontend/CompanyPortal-Vite/` — a single-page scrollable view with four section cards (Summary, File Status, Late Alerts, Audit Log), 60-second polling, manual retry with confirmation dialog, and a side drawer for retry history. The page is restricted to ADMIN and ADMIN_PARAM roles and consumes the Go backend proxy endpoints under `/api/eod/*`.

Implementation follows the project's feature-based module pattern (`src/features/eod-monitoring/`) and reuses existing shared components (Badge, Button, Card, DataTable, EmptyState, FilterSelect, PageHeader, SummaryCard, Toast).

## Tasks

- [ ] 1. Set up feature module structure and types
  - [ ] 1.1 Create feature directory and barrel export
    - Create `src/features/eod-monitoring/` directory structure: `index.ts`, `types.ts`, `hooks/`, `components/`, `__tests__/`
    - Create `index.ts` barrel export (empty for now, will be populated as components are built)
    - _Requirements: 12.1_

  - [ ] 1.2 Define TypeScript types and constants
    - Create `src/features/eod-monitoring/types.ts` with all API response types: `ApiEnvelope<T>`, `EodSummary`, `SummaryCounts`, `FileType`, `ProcessingStatus`, `FileStatusItem`, `FileStatusResponse`, `FileStatusRow`, `RetryAttempt`, `FileHistoryResponse`, `ManualRetryResponse`, `LateDetectionItem`, `AuditLogEntry`, `TriggerType`, `BadgeVariant`
    - Define `STATUS_BADGE_MAP`, `TRIGGER_BADGE_MAP`, `OUTCOME_BADGE_MAP`, `STATUS_LABELS`, `FILE_TYPE_LABELS`, `TRIGGER_LABELS` constants
    - Define `STATUS_BADGE_CONFIG` with icon mappings (using lucide-react icons: CheckCircle, XCircle, AlertTriangle, Loader, Clock)
    - Define `flattenFileStatus` utility function
    - _Requirements: 12.2, 4.3, 8.5, 8.6_

  - [ ] 1.3 Create timestamp formatting utilities
    - Create `src/features/eod-monitoring/utils.ts` with: `formatWibDateTime` (DD MMM YYYY HH:mm in Asia/Jakarta), `formatWibDateTimeSec` (DD MMM YYYY HH:mm:ss), `formatSlaTime` (HH:mm WIB), `formatDuration` (locale-formatted ms or "-")
    - Use `date-fns` + `date-fns-tz` for timezone conversion (check if `date-fns-tz` is installed, otherwise use `Intl.DateTimeFormat` with `timeZone: "Asia/Jakarta"`)
    - _Requirements: 4.5, 7.5, 8.7, 8.8_

- [ ] 2. Implement TanStack Query hooks
  - [ ] 2.1 Create query hooks file
    - Create `src/features/eod-monitoring/hooks/useEodQueries.ts`
    - Define `eodKeys` query key factory: `all`, `summary(date)`, `status(date)`, `history(fileId)`, `late(date)`, `audit(date, fileType, trigger)`
    - Implement `useEodSummary(processingDate, refetchInterval)` — GET `/api/eod/summary`
    - Implement `useEodStatus(processingDate, refetchInterval)` — GET `/api/eod/status`
    - Implement `useFileHistory(fileId)` — GET `/api/eod/status/{file_id}/history`, enabled only when fileId is not null
    - Implement `useEodLate(processingDate, refetchInterval)` — GET `/api/eod/late`
    - Implement `useEodAudit(processingDate, fileType, trigger, refetchInterval)` — GET `/api/eod/audit` with filter params
    - Implement `useRetryFile()` mutation — POST `/api/eod/retry/{file_id}`, onSuccess invalidates all `eodKeys.all` queries
    - All hooks use the existing `api` client from `@/lib/api/client`
    - All hooks unwrap `ApiEnvelope` and throw on `status === "error"`
    - _Requirements: 3.1, 4.1, 5.1, 7.1, 8.1, 6.5, 6.6, 9.1, 9.2, 12.3_

  - [ ]* 2.2 Write unit tests for query hooks
    - Test query key generation for all `eodKeys` factories
    - Test that `useFileHistory` is disabled when fileId is null
    - Test that `useRetryFile` invalidates queries on success
    - Mock API responses to test envelope unwrapping and error throwing
    - _Requirements: 12.3_

- [ ] 3. Implement utility and header components
  - [ ] 3.1 Create PollingToggle component
    - Create `src/features/eod-monitoring/components/PollingToggle.tsx`
    - Render a toggle switch showing polling state (active/inactive)
    - Accept `enabled: boolean` and `onChange: (enabled: boolean) => void` props
    - Display label text indicating current state
    - Ensure minimum 44x44px touch target
    - _Requirements: 9.3, 11.2_

  - [ ] 3.2 Create ProcessingDatePicker component
    - Create `src/features/eod-monitoring/components/ProcessingDatePicker.tsx`
    - Render a date input that defaults to current date in Asia/Jakarta timezone
    - Accept `value: string` and `onChange: (date: string) => void` props
    - Format value as YYYY-MM-DD for API consumption
    - _Requirements: 2.3, 2.5, 9.5_

  - [ ] 3.3 Create LastUpdatedIndicator component
    - Create `src/features/eod-monitoring/components/LastUpdatedIndicator.tsx`
    - Display "Terakhir diperbarui: {timestamp}" using most recent `dataUpdatedAt` from queries
    - Format in Asia/Jakarta timezone using `formatWibDateTime`
    - _Requirements: 9.4_

  - [ ] 3.4 Create SectionErrorState component
    - Create `src/features/eod-monitoring/components/SectionErrorState.tsx`
    - Render AlertTriangle icon + error message text + "Coba Lagi" secondary Button
    - Accept `message: string` and `onRetry: () => void` props
    - Use `aria-hidden="true"` on decorative icon
    - _Requirements: 3.5, 10.2, 10.4_

- [ ] 4. Implement Summary Section
  - [ ] 4.1 Create SummarySection component
    - Create `src/features/eod-monitoring/components/SummarySection.tsx`
    - Fetch data using `useEodSummary(processingDate, refetchInterval)`
    - Render 6 SummaryCard components (reuse shared `SummaryCard`) for: Pending, Processing, Completed, Failed, Max Retries Exhausted, Late
    - Use responsive grid: `grid-cols-1 md:grid-cols-2 lg:grid-cols-3`
    - Apply `tabular-nums` on count values
    - Render loading skeleton when query is pending
    - Render `SectionErrorState` when query fails
    - Wrap in `<section aria-labelledby="summary-title">`
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 11.1, 11.3_

  - [ ]* 4.2 Write property test for summary card completeness
    - **Property 3: Summary card completeness**
    - For any valid EodSummary with 6 count fields, assert exactly 6 SummaryCard components render with matching values
    - **Validates: Requirements 3.2**

- [ ] 5. Implement File Status Table
  - [ ] 5.1 Create FileStatusSection component
    - Create `src/features/eod-monitoring/components/FileStatusSection.tsx`
    - Fetch data using `useEodStatus(processingDate, refetchInterval)`
    - Flatten response using `flattenFileStatus` utility
    - Render shared `DataTable` with columns: File Type, Filename, Status (Badge with icon+label), Retry Count, Detected At, Last Retry At, Failure Reason
    - Map `processing_status` to Badge variant using `STATUS_BADGE_CONFIG` (always icon + label, never color alone)
    - Right-align Retry Count column with `tabular-nums`
    - Format timestamps using `formatWibDateTime`
    - Enable column sorting on File Type, Status, Retry Count, Detected At
    - Handle row click → call `onFileSelect(fileId)` prop to open drawer
    - Render `EmptyState` when no files exist
    - Render loading skeleton / `SectionErrorState` for loading/error states
    - Make rows focusable and activatable with Enter/Space for keyboard navigation
    - Wrap in `<section aria-labelledby="file-status-title">`
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.9, 11.2, 11.3_

  - [ ]* 5.2 Write property test for status-to-badge mapping
    - **Property 5: Status-to-badge mapping correctness**
    - For all ProcessingStatus values, assert correct BadgeVariant mapping
    - For all TriggerType values, assert correct BadgeVariant mapping
    - For all outcome values, assert correct BadgeVariant mapping
    - **Validates: Requirements 4.3, 8.5, 8.6**

- [ ] 6. Checkpoint
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 7. Implement Retry Drawer
  - [ ] 7.1 Create RetryDrawer component
    - Create `src/features/eod-monitoring/components/RetryDrawer.tsx`
    - Slide in from right: 300ms `cubic-bezier(0.22, 1, 0.36, 1)` on `transform` + `opacity`, exit at 225ms (75% of enter)
    - Accept `fileId: string | null` and `onClose: () => void` props
    - Fetch history using `useFileHistory(fileId)` when fileId is non-null
    - Display file metadata: filename, file type label, checksum (truncated to 12 chars + ellipsis), current status Badge
    - Render `RetryAttemptList`: attempt number, trigger Badge (Auto/Manual), started_at, duration (ms), outcome Badge, error detail (if failed)
    - Render "Retry Manual" primary Button: visible+enabled when status is `failed` or `max_retries_exhausted`, visible+disabled when `processing`, NOT rendered when `completed` or `pending`
    - Render close button (X icon) with min 44x44px touch target
    - Close on Escape key or click outside
    - Implement focus trap: store trigger element ref, move focus into drawer on open, return focus to trigger on close
    - Show loading skeleton while history query is pending
    - Fixed width: 400px on desktop/tablet, full width on mobile
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 5.9, 5.10, 11.2, 11.4_

  - [ ]* 7.2 Write property test for retry button visibility state machine
    - **Property 9: Retry button visibility state machine**
    - For all processing_status values, assert correct button state: failed/max_retries_exhausted→enabled, processing→disabled, completed/pending→hidden
    - **Validates: Requirements 5.5, 5.6, 5.7**

- [ ] 8. Implement Retry Confirmation Dialog
  - [ ] 8.1 Create RetryConfirmationDialog component
    - Create `src/features/eod-monitoring/components/RetryConfirmationDialog.tsx`
    - Accept `file: FileStatusItem | null` and `onClose: () => void` props
    - Display file metadata: filename, file type label, current retry count
    - Display warning message about re-executing ETL script
    - Render "Konfirmasi Retry" primary button and "Batal" secondary button
    - On confirm: call `useRetryFile().mutate(fileId)` with `onSuccess` (close + success toast + invalidate queries) and `onError` (close + error toast)
    - While mutation in-flight: disable both buttons, show spinner on confirm button
    - Implement focus trap and return focus to retry button on close
    - Entry animation: 200ms scale+opacity, exit 150ms
    - Handle 409 (file already completed) with error toast message from response
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9, 10.5, 11.5_

  - [ ]* 8.2 Write property test for confirmation dialog metadata display
    - **Property 12: Confirmation dialog metadata display**
    - For any FileStatusItem, assert dialog displays filename, file_type label, retry_count
    - **Validates: Requirements 6.2**

- [ ] 9. Implement Late Alerts Section
  - [ ] 9.1 Create LateAlertsSection component
    - Create `src/features/eod-monitoring/components/LateAlertsSection.tsx`
    - Fetch data using `useEodLate(processingDate, refetchInterval)`
    - Render each `LateDetectionItem` as a `LateAlertCard`
    - Display: file type label, SLA deadline (formatted as `HH:mm WIB` via `formatSlaTime`), detected_at timestamp, resolution Badge
    - Badge mapping: `is_resolved === false` → warning Badge "Late" with icon, `is_resolved === true` → success Badge "Resolved" with icon
    - Render `EmptyState` with message "Tidak ada file terlambat" when no late detections
    - Show loading skeleton for pending state, `SectionErrorState` for errors
    - Wrap in `<section aria-labelledby="late-alerts-title">`
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 11.3_

  - [ ]* 9.2 Write property test for late alert badge mapping
    - **Property 14: Late alert field completeness and badge mapping**
    - For any LateDetectionItem, assert correct badge: `is_resolved=false`→warning "Late", `is_resolved=true`→success "Resolved"
    - Assert file_type, sla_deadline, detected_at are displayed
    - **Validates: Requirements 7.2, 7.3, 7.4, 7.5**

- [ ] 10. Implement Audit Log Section
  - [ ] 10.1 Create AuditLogSection component
    - Create `src/features/eod-monitoring/components/AuditLogSection.tsx`
    - Manage local filter state: `fileType` and `trigger` (default: null = "Semua")
    - Render shared `FilterSelect` for File Type (options: Semua, DMAA, ITM Cash Position, ITM Replenishment) and Trigger Type (options: Semua, Auto, Manual)
    - Fetch data using `useEodAudit(processingDate, fileType, trigger, refetchInterval)`
    - Render shared `DataTable` with columns: Timestamp, Event Type, Trigger (Badge), File Type, Initiated By, Outcome (Badge), Duration, Error Detail
    - Map trigger to Badge: auto→info, manual→neutral
    - Map outcome to Badge: completed→success, failed→danger, null→info
    - Format timestamps with `formatWibDateTimeSec` (DD MMM YYYY HH:mm:ss in WIB)
    - Right-align Duration column with `tabular-nums`, format as "N ms" or "-"
    - Render `EmptyState` with message "Tidak ada log audit" when no records
    - Refetch on filter change
    - Wrap in `<section aria-labelledby="audit-log-title">`
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8, 8.9, 11.3_

  - [ ]* 10.2 Write property test for audit filter propagation
    - **Property 15: Audit filter propagation**
    - For any filter combination, assert the query includes correct parameters
    - **Validates: Requirements 8.3**

- [ ] 11. Checkpoint
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 12. Implement Page Orchestrator and Route
  - [ ] 12.1 Create EodMonitoringPage orchestrator component
    - Create `src/features/eod-monitoring/components/EodMonitoringPage.tsx`
    - Manage page state: `processingDate` (default: today in WIB), `pollingEnabled` (default: true), `selectedFileId` (default: null), `retryDialogFile` (default: null)
    - Compute `refetchInterval = pollingEnabled ? 60_000 : false`
    - Render shared `PageHeader` with title "EOD Monitoring" and actions slot containing `ProcessingDatePicker`, `PollingToggle`, `LastUpdatedIndicator`
    - Render sections in order: `SummarySection`, `FileStatusSection`, `LateAlertsSection`, `AuditLogSection`
    - Pass `processingDate` and `refetchInterval` to all sections
    - Pass `onFileSelect` callback to `FileStatusSection` to set `selectedFileId`
    - Render `RetryDrawer` with `fileId={selectedFileId}` and onClose resets selectedFileId
    - Wire "Retry Manual" button in drawer to set `retryDialogFile` and open `RetryConfirmationDialog`
    - Render `RetryConfirmationDialog` with `file={retryDialogFile}` and onClose resets it
    - Wrap page content in `<main>` landmark
    - Render `aria-live="polite"` region for dynamic announcements (toast container)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 9.1, 9.2, 9.4, 9.5, 10.1, 10.2, 10.3, 11.3, 11.6_

  - [ ] 12.2 Register route file
    - Create `src/routes/eod-monitoring.tsx`
    - Register route under `protectedRoute` at path `/eod-monitoring`
    - Apply `requireRoles(["ADMIN", "ADMIN_PARAM"])` in `beforeLoad`
    - Import `EodMonitoringPage` from `@/features/eod-monitoring`
    - Follow existing route pattern (see `cash-flow.tsx` as reference)
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

  - [ ] 12.3 Update barrel export
    - Update `src/features/eod-monitoring/index.ts` to export `EodMonitoringPage` and all public types
    - _Requirements: 12.1_

- [ ] 13. Final integration and verification
  - [ ]* 13.1 Write integration test for EodMonitoringPage
    - Test section isolation: mock one section API to fail, assert other sections render normally
    - Test date change triggers all section refetches
    - Test polling toggle enables/disables refetchInterval
    - Test retry flow: row click → drawer opens → retry button → dialog → confirm → toast
    - Test role guard: non-admin user sees forbidden state
    - _Requirements: 10.3, 2.5, 9.1, 9.2, 6.5, 6.6, 1.2, 1.3_

  - [ ]* 13.2 Write property tests for timestamp formatting
    - **Property 7: Timestamp WIB formatting**
    - For any valid ISO 8601 string, assert `formatWibDateTime` produces `DD MMM YYYY HH:mm` in Asia/Jakarta
    - Assert `formatWibDateTimeSec` produces `DD MMM YYYY HH:mm:ss` in Asia/Jakarta
    - **Property 18: SLA deadline formatting**
    - For any "HH:mm" or "HH:mm:ss" input, assert `formatSlaTime` produces "HH:mm WIB"
    - **Property 16: Duration formatting**
    - For any numeric ms, assert locale-formatted number + " ms"; for null assert "-"
    - **Validates: Requirements 4.5, 7.5, 8.7, 8.8**

- [ ] 14. Final checkpoint
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Reuse existing shared components (`Badge`, `Button`, `Card`, `DataTable`, `EmptyState`, `FilterSelect`, `PageHeader`, `SummaryCard`, `Toast`) — do NOT duplicate them
- Check if `date-fns-tz` is installed; if not, use `Intl.DateTimeFormat` with `timeZone: "Asia/Jakarta"` as a zero-dependency alternative
- Follow existing route registration pattern (reference: `src/routes/cash-flow.tsx`)
- Follow existing feature module pattern (reference: `src/features/dashboard/`)
- All timestamps display in Asia/Jakarta (WIB) timezone
- All monetary/numeric columns use `tabular-nums` and right-alignment
- Status badges ALWAYS include icon + label text (never color alone) per design system accessibility rule
- The page uses OKLCH design tokens from the Merah Sirih theme (internal app)

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "1.3"] },
    { "id": 1, "tasks": ["2.1", "3.1", "3.2", "3.3", "3.4"] },
    { "id": 2, "tasks": ["2.2", "4.1", "5.1", "9.1", "10.1"] },
    { "id": 3, "tasks": ["4.2", "5.2", "7.1", "9.2", "10.2"] },
    { "id": 4, "tasks": ["7.2", "8.1"] },
    { "id": 5, "tasks": ["8.2", "12.1"] },
    { "id": 6, "tasks": ["12.2", "12.3"] },
    { "id": 7, "tasks": ["13.1", "13.2"] }
  ]
}
```
