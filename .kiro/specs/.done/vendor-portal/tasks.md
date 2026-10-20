# Implementation Plan: Vendor Portal

## Overview

Build a standalone React SPA at `frontend/VendorPortal-Vite/` that provides CIT vendor personnel with a scoped view into the CIMB Niaga Cash Management System. The portal runs independently in its own Docker container (port 3001), uses TanStack Query with static JSON mock data, and applies the "Merah Menyala" brand theme. Implementation uses React 19, TypeScript, Vite 6, Tailwind CSS 4, React Router v7, and TanStack Table v8.

## Tasks

- [x] 1. Project scaffolding and configuration
  - [x] 1.1 Initialize Vite project with React + TypeScript template
    - Create `frontend/VendorPortal-Vite/` directory
    - Initialize with `pnpm create vite . --template react-ts`
    - Install dependencies: `react-router`, `@tanstack/react-query`, `@tanstack/react-table`, `react-hook-form`, `zod`, `@hookform/resolvers`, `lucide-react`
    - Install dev dependencies: `vitest`, `fast-check`, `@testing-library/react`, `@testing-library/user-event`, `jsdom`, `tailwindcss`, `@tailwindcss/vite`
    - Configure `vite.config.ts` with Tailwind plugin and port 5174
    - Configure `tsconfig.json` and `tsconfig.app.json` with strict mode and path aliases
    - _Requirements: 10.6_

  - [x] 1.2 Configure Tailwind CSS with Merah Menyala theme tokens
    - Create `src/styles/index.css` with Tailwind imports and OKLCH custom properties
    - Define all Merah Menyala color tokens (topbar, sidebar, sidebar-active, surface, semantic colors)
    - Configure Tailwind to use the custom tokens
    - _Requirements: 2.1, 13.6_

  - [x] 1.3 Set up Vitest configuration with jsdom and fast-check
    - Create `vitest.config.ts` with jsdom environment and globals
    - Create `src/test-setup.ts` for test configuration
    - Verify fast-check imports work correctly
    - _Requirements: Design Testing Strategy_

  - [x] 1.4 Create shared TypeScript interfaces and types
    - Create `src/lib/types.ts` with all interfaces: `VendorUser`, `AuthState`, `JwtPayload`, `CITOrder`, `HandoverEvidence`, `EvidenceFile`, `Invoice`, `InvoiceLineItem`, `ReplenishmentSchedule`, `DsrRecord`, `BalanceStatus`, `Notification`
    - _Requirements: 9.1, 9.6_

  - [x] 1.5 Create utility functions (formatters, constants)
    - Create `src/lib/formatters.ts` with `formatIDR`, `formatRp`, `formatBadgeCount`, `truncate`, `getBalanceStatus`
    - Create `src/lib/constants.ts` with navigation items, route paths, thresholds
    - _Requirements: 3.7, 5.5, 6.7, 7.3, 7.8, 2.7, 8.5_

  - [x] 1.6 Write property tests for utility functions
    - **Property 2: IDR Currency Formatting** — verify `formatIDR` and `formatRp` produce correct dot-separated format for any non-negative integer, and parsing back yields original
    - **Property 3: Badge Count Formatting** — verify `formatBadgeCount` returns null for 0, string for 1-99, "99+" for >99
    - **Property 4: String Truncation** — verify `truncate` preserves input when within maxLength, appends "..." when exceeding
    - **Property 5: Balance Status Classification** — verify `getBalanceStatus` returns correct status for all threshold boundaries
    - **Validates: Requirements 3.7, 5.5, 6.7, 7.8, 2.7, 8.5, 7.3, 2.2**

- [x] 2. Authentication module
  - [x] 2.1 Create AuthContext and useAuth hook
    - Create `src/features/auth/AuthContext.tsx` with React context providing `AuthState`, `login`, `logout`
    - Create `src/features/auth/useAuth.ts` hook for consuming auth context
    - Store JWT in memory (React state/ref), decode payload to extract `vendor_id`, `vendor_name`, `display_name`
    - On login, validate credentials against mock vendor users JSON, issue simulated JWT
    - On logout, clear auth state and TanStack Query cache
    - _Requirements: 1.2, 1.4, 1.5, 12.1, 12.6_

  - [x] 2.2 Create ProtectedRoute component
    - Create `src/features/auth/ProtectedRoute.tsx` that checks authentication status
    - If unauthenticated, redirect to `/login` preserving the originally requested URL
    - If authenticated and accessing `/login`, redirect to `/orders`
    - _Requirements: 11.3, 11.4, 11.5_

  - [x] 2.3 Create LoginPage component
    - Create `src/features/auth/LoginPage.tsx` with username/password fields, CIMB Niaga logo, submit button
    - Apply Merah Menyala maroon-red color scheme to login page
    - Implement form validation: minimum 8 char password, both fields required
    - Show loading state on submit, disable button during request
    - Display error messages: "Username atau password salah" for invalid credentials, connection error for network failures
    - Retain username field on error
    - _Requirements: 1.1, 1.3, 1.6, 1.7, 1.8_

  - [x] 2.4 Write property test for authentication route guard
    - **Property 12: Authentication Route Guard Round-Trip** — verify unauthenticated access redirects to /login with preserved path, and post-login redirects to preserved path
    - **Validates: Requirements 11.3, 11.4**

- [x] 3. Application shell and layout
  - [x] 3.1 Create AppShell layout component
    - Create `src/app/AppShell.tsx` with maroon-red top bar, maroon-deep sidebar, and main content area
    - Top bar: vendor company name (truncated at 30 chars), user display name (truncated at 20 chars), logout button
    - Sidebar: navigation links with Lucide icons for all 6 screens
    - Active nav item uses full-red `oklch(54% 0.233 27)` background with white text
    - Wrap content area with React error boundary showing "Terjadi kesalahan" fallback
    - _Requirements: 2.1, 2.2, 2.3, 2.5, 2.8, 2.9_

  - [x] 3.2 Implement responsive sidebar behavior
    - Collapse sidebar to 64px icon-only rail when viewport < 1024px
    - Provide toggle button to expand sidebar as fixed overlay on mobile
    - Ensure keyboard accessibility: Tab navigation, Enter activation, `<nav>` landmark
    - _Requirements: 2.6, 2.10, 13.1_

  - [x] 3.3 Create NotificationBadge component
    - Create `src/components/layout/NotificationBadge.tsx` showing unread count on Notifications nav item
    - Use `formatBadgeCount` for display (numeric 1-99, "99+" for >99, hidden for 0)
    - _Requirements: 2.7, 8.5_

  - [x] 3.4 Set up React Router with route definitions
    - Create `src/app/routes.tsx` with routes: `/login`, `/orders`, `/orders/:id/evidence`, `/invoices`, `/schedule`, `/dsr`, `/notifications`
    - Create `src/app/App.tsx` as root component with QueryClientProvider, AuthProvider, RouterProvider
    - Implement catch-all route for NotFound page showing "Halaman tidak ditemukan" with link to /orders
    - Internal-only routes (admin, forecasting, reconciliation) show same NotFound page
    - _Requirements: 11.1, 11.2, 11.6, 11.7, 11.8, 11.9_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Shared UI components
  - [x] 5.1 Create Badge component
    - Create `src/components/ui/Badge.tsx` with variants: info, warning, success, danger, neutral
    - Each badge renders icon + text label (never color alone)
    - Use semantic colors from Merah Menyala theme tokens
    - _Requirements: 3.3, 5.3, 6.3, 6.4, 7.3, 13.4_

  - [x] 5.2 Create DataTable component (TanStack Table wrapper)
    - Create `src/components/ui/DataTable.tsx` as a generic table wrapper
    - Support sorting (click column headers to toggle asc/desc)
    - Right-align monetary columns with `tabular-nums`
    - Wrap table in horizontally scrollable container for responsive behavior
    - Use semantic HTML `<table>` element
    - _Requirements: 3.9, 7.9, 13.2, 13.3_

  - [x] 5.3 Create supporting UI components
    - Create `src/components/ui/Button.tsx` with primary/secondary/ghost/danger variants
    - Create `src/components/ui/Card.tsx` for summary cards
    - Create `src/components/ui/DatePicker.tsx` for date range selection
    - Create `src/components/ui/EmptyState.tsx` for empty state placeholders
    - Create `src/components/ui/FilterTabs.tsx` for status filter tabs with counts
    - Create `src/components/ui/FileUpload.tsx` with drag-drop, preview, validation (max 5 files, 10MB, accepted types)
    - Ensure all interactive elements have 44×44px minimum touch target and visible focus indicators
    - _Requirements: 3.4, 3.10, 4.2, 4.6, 13.5, 13.7_

  - [x] 5.4 Write unit tests for UI components
    - Test Badge renders correct variant per status mapping
    - Test DataTable sorting interaction
    - Test FileUpload validation feedback (size, type, count limits)
    - Test EmptyState displays correct messages
    - _Requirements: 3.3, 3.9, 4.2, 4.3_

- [x] 6. Mock data layer
  - [x] 6.1 Create vendor users mock data
    - Create `src/data/vendors.json` with 3 vendors (PT Gardanet, PT SSI, PT G4S), each with 2+ user credentials
    - Include: id, username, password placeholder, displayName, vendorId, vendorName, role
    - _Requirements: 9.2_

  - [x] 6.2 Create CIT orders and evidence mock data
    - Create `src/data/orders.json` with 20+ CIT orders distributed across 3 vendors (minimum 5 per vendor)
    - Include all statuses: Scheduled, In Transit, Completed, Failed
    - Create `src/data/evidence.json` with evidence metadata for completed orders
    - Use realistic ATM IDs (ATM-JKT-001, ATM-BDG-003, etc.) across 4+ region prefixes
    - _Requirements: 9.3, 9.4, 9.5, 9.6_

  - [x] 6.3 Create invoices, schedules, DSR, and notifications mock data
    - Create `src/data/invoices.json` with 10+ invoices with line items, referencing valid CIT order IDs
    - Create `src/data/schedules.json` with 15+ replenishment schedules
    - Create `src/data/dsr.json` with 280+ DSR records (20 ATMs × 14 days)
    - Create `src/data/notifications.json` with 10+ notifications covering all 4 types
    - All records include `vendorId` field for scoping
    - _Requirements: 9.1, 9.3, 9.5, 9.6, 9.7_

  - [x] 6.4 Create TanStack Query hooks with vendor scoping
    - Create `src/lib/queryClient.ts` with QueryClient instance
    - Create `src/features/orders/useOrders.ts` — filters orders by vendorId from auth context
    - Create `src/features/evidence/useEvidence.ts` — upload mutation hook
    - Create `src/features/invoices/useInvoices.ts` — filters invoices by vendorId
    - Create `src/features/schedule/useSchedule.ts` — filters schedules by vendorId
    - Create `src/features/dsr/useDsr.ts` — filters DSR by vendorId
    - Create `src/features/notifications/useNotifications.ts` — filters notifications by vendorId, provides markAsRead mutation
    - Each hook reads from static JSON import and filters by `vendor_id` from JWT
    - _Requirements: 9.8, 9.9, 12.1, 12.2_

  - [x] 6.5 Write property tests for data filtering and isolation
    - **Property 1: Vendor Data Isolation** — verify filtering by vendor_id returns only matching records, never records from other vendors
    - **Property 6: Date Range Filtering** — verify date range filter returns exactly records within bounds (inclusive)
    - **Property 7: Status Filter with AND Composition** — verify combined status + date filters equal intersection of independent filters
    - **Validates: Requirements 3.2, 3.4, 3.5, 5.2, 6.2, 7.2, 9.3, 9.9, 12.1, 12.2**

- [x] 7. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Feature pages - CIT Orders
  - [x] 8.1 Implement CIT Orders dashboard page
    - Create `src/features/orders/OrdersPage.tsx` with DataTable, status filter tabs, date range filter
    - Create `src/features/orders/OrderSummaryBar.tsx` showing total counts per status (unfiltered)
    - Default sort: Scheduled Date descending
    - Status badges: Scheduled (info), In Transit (warning), Completed (success), Failed (danger)
    - Empty state when no orders match filters
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 3.10_

  - [x] 8.2 Write property tests for sorting
    - **Property 8: Column Sorting Correctness** — verify ascending produces non-decreasing order, descending produces non-increasing order, output is permutation of input
    - **Validates: Requirements 3.9, 7.9**

- [x] 9. Feature pages - Evidence Upload
  - [x] 9.1 Implement Evidence Upload page
    - Create `src/features/evidence/EvidencePage.tsx` accessible from `/orders/:id/evidence`
    - Create `src/features/evidence/EvidenceForm.tsx` with React Hook Form + Zod validation
    - Fields: file attachment (1-5 files), handover timestamp (not future, not >72h ago), recipient name (max 100 chars), notes (optional, max 500 chars)
    - Show thumbnail previews for image files, remove button per file
    - Display progress indicator and disable submit during upload
    - Show success confirmation on valid submission
    - Display existing evidence in read-only mode if already uploaded
    - Retain form values on error for retry
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8_

  - [x] 9.2 Write property test for file upload validation
    - **Property 9: File Upload Validation** — verify accepted types + size ≤10MB pass, invalid type or size >10MB fail with specific error
    - **Validates: Requirements 4.2**

- [x] 10. Feature pages - Invoices
  - [x] 10.1 Implement Invoice Viewing page
    - Create `src/features/invoices/InvoicesPage.tsx` with invoice table and summary section
    - Create `src/features/invoices/InvoiceDetail.tsx` for expandable line items view
    - Columns: Invoice Number, Period, Total Amount, Line Items Count, Validation Status
    - Status badges: Uploaded (info), Validated (warning), Mismatch Detected (danger), Approved (success with CheckCircle)
    - Summary: total invoiced amount, count per status, approved sum
    - Highlight mismatch line items with danger background tint
    - Keyboard accessible row expansion (Enter/Space)
    - Empty state when no invoices exist
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8_

- [x] 11. Feature pages - Replenishment Schedule
  - [x] 11.1 Implement Schedule page
    - Create `src/features/schedule/SchedulePage.tsx` with grouped-by-date view
    - Date header shows "DD MMM YYYY" with total amount and schedule count per day
    - Default sort: date ascending, priority descending within each group
    - Priority badges: High (danger), Medium (warning), Low (neutral)
    - Status badges: Pending (warning), Confirmed (info), Executed (success), Cancelled (danger)
    - Format amounts with "Rp" prefix
    - Default view: today and future dates
    - Date range filter support
    - Empty state when no schedules match
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9, 6.10_

  - [x] 11.2 Write property tests for schedule grouping and sorting
    - **Property 10: Schedule Date Grouping Aggregation** — verify group totals equal sum of recommendedAmount, counts match, union equals original set
    - **Property 11: Schedule Multi-Level Sort** — verify dates ascending across groups, priorities descending within groups, output is permutation of input
    - **Validates: Requirements 6.5, 6.6**

- [x] 12. Feature pages - DSR Monitor
  - [x] 12.1 Implement DSR Monitor page
    - Create `src/features/dsr/DsrPage.tsx` with data table and summary card
    - Create `src/features/dsr/DsrSummaryCard.tsx` showing total ATMs, Critical count, Low count, total Ending Balance
    - Columns: ATM ID, Location, Date, Beginning Balance, Cash In, Cash Out, Ending Balance, Balance Status
    - Date selector defaulting to 2024-01-15
    - Balance status badges: Critical (danger), Low (warning), Normal (success)
    - Default sort: ATM ID ascending
    - Column sorting support
    - Empty state when no records for selected date
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8, 7.9_

- [x] 13. Feature pages - Notifications
  - [x] 13.1 Implement Notifications page
    - Create `src/features/notifications/NotificationsPage.tsx` with notification list
    - Columns: Timestamp ("DD MMM YYYY HH:mm"), Type, Message (truncated at 120 chars), Read Status
    - Visually differentiate unread (bold + background tint) from read
    - On click: mark as read and navigate to relevant screen based on notification type
    - "Mark All as Read" button (disabled when no unread exist)
    - Update sidebar badge count on read actions
    - Sort by timestamp descending (newest first)
    - Empty state when no notifications
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8_

  - [x] 13.2 Write property test for notification routing
    - **Property 13: Notification Type Routing** — verify each notification type maps deterministically to correct route (/orders, /invoices, /schedule)
    - **Validates: Requirements 8.4**

- [x] 14. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 15. Docker containerization
  - [x] 15.1 Create Docker configuration files
    - Create `frontend/VendorPortal-Vite/Dockerfile` with multi-stage build (pnpm install + build → nginx:alpine)
    - Create `frontend/VendorPortal-Vite/docker-compose.yml` mapping container port 80 to host 3001, configurable via PORT env var
    - Create `frontend/VendorPortal-Vite/nginx.conf` with `try_files $uri $uri/ /index.html` and static asset caching (30+ days)
    - Create `frontend/VendorPortal-Vite/.dockerignore` excluding node_modules, dist, .env, .git
    - Add health check in docker-compose for HTTP connectivity verification
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.7_

- [x] 16. Integration wiring and final verification
  - [x] 16.1 Wire all components together and verify routing
    - Ensure App.tsx correctly wraps providers: QueryClientProvider → AuthProvider → RouterProvider
    - Verify all routes resolve correctly with ProtectedRoute guard
    - Verify browser back/forward navigation works without full page reload
    - Verify direct URL entry renders correct page without prior navigation
    - Ensure NotFound page displays for unknown and internal-only routes
    - Verify vendor data isolation: no UI controls to access other vendors' data
    - _Requirements: 11.8, 11.9, 12.3, 12.4, 12.5_

  - [x] 16.2 Write integration tests
    - Test login → redirect → data display flow
    - Test vendor scoping (logged in as Gardanet sees only Gardanet data)
    - Test navigation between all screens
    - Test error boundary fallback rendering
    - _Requirements: 1.2, 2.4, 12.1, 12.2_

- [x] 17. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- The project is completely isolated from the internal frontend — no shared code, dependencies, or build artifacts
- All monetary values stored as integers (no decimals) and formatted for display with Indonesian locale
- Mock data designed for seamless API swap: change only `queryFn` implementations

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3", "1.4"] },
    { "id": 2, "tasks": ["1.5", "2.1"] },
    { "id": 3, "tasks": ["1.6", "2.2", "2.3", "3.1"] },
    { "id": 4, "tasks": ["2.4", "3.2", "3.3", "3.4"] },
    { "id": 5, "tasks": ["5.1", "5.2", "5.3", "6.1"] },
    { "id": 6, "tasks": ["5.4", "6.2", "6.3"] },
    { "id": 7, "tasks": ["6.4"] },
    { "id": 8, "tasks": ["6.5", "8.1"] },
    { "id": 9, "tasks": ["8.2", "9.1", "10.1"] },
    { "id": 10, "tasks": ["9.2", "11.1", "12.1"] },
    { "id": 11, "tasks": ["11.2", "13.1"] },
    { "id": 12, "tasks": ["13.2", "15.1"] },
    { "id": 13, "tasks": ["16.1"] },
    { "id": 14, "tasks": ["16.2"] }
  ]
}
```
