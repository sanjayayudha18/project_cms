# Implementation Plan: CMS Frontend Prototype

## Overview

Build a frontend-only stakeholder demo SPA using Vite + React 19 + TypeScript. The app has four operational screens (DSR Dashboard, Forecast View, CIT Tracker, Invoice Flow) powered by static JSON mock data, with role-based navigation and Docker containerization. Implementation follows a bottom-up approach: project scaffolding → shared utilities/components → mock data → feature screens → Docker packaging.

## Tasks

- [x] 1. Project scaffolding and configuration
  - [x] 1.1 Initialize Vite + React + TypeScript project with core dependencies
    - Run `pnpm create vite frontend --template react-ts` and install: react@19, react-dom@19, react-router-dom, @tanstack/react-query@5, @tanstack/react-table@8, tailwindcss@4, lucide-react
    - Install dev dependencies: vitest, @testing-library/react, @testing-library/jest-dom, jsdom, fast-check, playwright, @vitejs/plugin-react
    - Configure `vite.config.ts` with path alias `@/` → `src/`
    - Configure `tsconfig.json` with strict mode and path aliases
    - Configure `vitest.config.ts` with jsdom environment
    - Create directory structure: `src/app/`, `src/components/ui/`, `src/components/layout/`, `src/features/`, `src/context/`, `src/data/`, `src/lib/`, `src/styles/`
    - _Requirements: 8.1, 9.1_

  - [x] 1.2 Configure Tailwind CSS 4 with OKLCH design tokens
    - Create `src/styles/index.css` with Tailwind directives and CSS custom properties for all OKLCH tokens (brand red scale, neutrals, semantic colors from steering doc)
    - Configure system font stack with `tabular-nums` utility class
    - Set up spacing scale (4pt base), radius tokens, and shadow tokens
    - _Requirements: 10.3, 10.4_

- [x] 2. Shared utilities and lib layer
  - [x] 2.1 Implement currency formatting utility (`src/lib/formatCurrency.ts`)
    - Implement `formatIDR(amount: number): string` using `Intl.NumberFormat('id-ID')` for dot-separated thousands, no decimals
    - Implement `parseIDR(formatted: string): number` for round-trip parsing
    - Export both functions
    - _Requirements: 3.3, 4.1_

  - [x] 2.2 Write property test for currency formatting
    - **Property 2: Currency formatting consistency**
    - Test that for any non-negative integer, `parseIDR(formatIDR(n))` round-trips back to original value
    - Test output uses dot-separated thousands with no decimal places
    - Use fast-check with `{ numRuns: 100 }`
    - **Validates: Requirements 3.3, 4.1**

  - [x] 2.3 Implement status derivation utility (`src/lib/deriveStatus.ts`)
    - Implement `deriveStatus(endingBalance: number): 'Critical' | 'Low' | 'Normal'`
    - Critical: < 50,000,000; Low: 50,000,000–150,000,000 inclusive; Normal: > 150,000,000
    - _Requirements: 3.4_

  - [x] 2.4 Write property test for status derivation
    - **Property 3: Status derivation from ending balance thresholds**
    - Test totality: function always returns exactly one of three values for any non-negative integer
    - Test boundary correctness at 49,999,999 / 50,000,000 / 150,000,000 / 150,000,001
    - Use fast-check with `{ numRuns: 100 }`
    - **Validates: Requirements 3.4**

  - [x] 2.5 Implement filter utilities (`src/lib/filters.ts`)
    - Implement `filterByField<T>(records: T[], field: keyof T, value: T[keyof T] | null): T[]` — returns all records when value is null, filtered subset otherwise
    - Implement `compoundFilter<T>(records: T[], filters: Partial<Record<keyof T, unknown>>): T[]` — applies all non-null filters simultaneously
    - _Requirements: 4.6, 4.7, 5.4, 5.5_

  - [x] 2.6 Write property test for single-filter correctness
    - **Property 4: Single-filter state management**
    - Test: filtering returns only matching records (no false positives), all matching records (no false negatives)
    - Test: clearing filter (null) restores original array
    - Use fast-check with `{ numRuns: 100 }`
    - **Validates: Requirements 4.6, 4.7, 4.9**

  - [x] 2.7 Write property test for compound-filter correctness
    - **Property 5: Compound-filter correctness**
    - Test: compound filter returns exactly those records satisfying ALL active criteria
    - Test: summary counts match actual counts in filtered result
    - Use fast-check with `{ numRuns: 100 }`
    - **Validates: Requirements 5.4, 5.5, 5.6, 5.7**

  - [x] 2.8 Implement constants and type definitions (`src/lib/constants.ts`)
    - Define `Role` type: `'Admin' | 'Operator' | 'Manager' | 'Vendor'`
    - Define all status enums, nav items array with `internalOnly` flag
    - Implement `filterNavByRole(items: NavItem[], role: Role): NavItem[]`
    - _Requirements: 2.4, 2.5_

  - [x] 2.9 Write property test for role-based navigation visibility
    - **Property 1: Role-based navigation visibility**
    - Test: for any set of nav items and any role, item is visible iff role is internal OR item.internalOnly is false
    - Test: Vendor role sees only items with `internalOnly === false`
    - Use fast-check with `{ numRuns: 100 }`
    - **Validates: Requirements 2.4, 2.5**

  - [x] 2.10 Configure TanStack Query client (`src/lib/queryClient.ts`)
    - Create QueryClient with `staleTime: Infinity` defaults for mock data
    - Export configured client instance
    - _Requirements: 7.1_

- [x] 3. Shared UI components
  - [x] 3.1 Implement Badge component (`src/components/ui/Badge.tsx`)
    - Accept `variant` (success, warning, danger, info, neutral), `icon` (Lucide icon), and `label` props
    - Render pill shape with icon + text label — never color alone
    - Map variants to semantic OKLCH tokens from design system
    - _Requirements: 3.4, 4.3, 5.3, 10.4_

  - [x] 3.2 Implement DataTable component (`src/components/ui/DataTable.tsx`)
    - Generic TanStack Table wrapper accepting `data`, `columns`, `defaultSorting`, and `emptyMessage`
    - Implement sortable column headers (click to toggle asc/desc)
    - Render empty state when `data.length === 0`
    - Wrap table in horizontally scrollable container for responsive support
    - Right-align columns marked with `meta.align: 'right'`
    - Apply `tabular-nums` to numeric cells
    - _Requirements: 3.1, 3.5, 4.1, 5.1, 10.2_

  - [x] 3.3 Implement SummaryCard, Card, EmptyState, FilterSelect, DatePicker, Button, WorkflowSteps components
    - `SummaryCard`: label + formatted value (currency/number/text)
    - `Card`: wrapper with shadow and padding tokens
    - `EmptyState`: icon + message for no-data states
    - `FilterSelect`: dropdown with placeholder, nullable value, 44×44px touch target
    - `DatePicker`: date input defaulting to mock current date
    - `Button`: primary (red-500), secondary, ghost, danger variants; 44×44px min touch target
    - `WorkflowSteps`: three-step indicator with completed/current/upcoming states
    - _Requirements: 3.6, 3.7, 4.8, 5.6, 6.1, 10.5_

- [x] 4. Mock data layer
  - [x] 4.1 Create mock data JSON files
    - `src/data/vendors.json`: 3+ vendors (PT Gardanet, PT SSI, PT G4S)
    - `src/data/atms.json`: 20+ ATMs across 3 region prefixes (JKT, BDG, SBY), pattern ATM-{REGION}-{NNN}
    - `src/data/dsr.json`: 7 days × 20+ ATMs = 140+ records with realistic IDR amounts (50M–500M range), statuses derived from ending balance
    - `src/data/forecast.json`: 15+ records covering High/Medium/Low priorities
    - `src/data/cit-orders.json`: 15+ orders across 3+ vendors, all 4 statuses (Scheduled, In Transit, Completed, Failed)
    - `src/data/invoices.json`: 8+ invoices with line items, all 4 validation states, maker-checker names
    - Ensure referential integrity: CIT orders reference valid ATM IDs, invoices reference valid CIT order IDs, forecasts reference valid ATM IDs
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7_

  - [x] 4.2 Write property test for mock data referential integrity
    - **Property 6: Mock data referential integrity**
    - Test: every CIT order's atmId exists in atms.json
    - Test: every CIT order's vendorId exists in vendors.json
    - Test: every invoice's vendorId exists in vendors.json
    - Test: every invoice line item's matchedOrderRef exists in cit-orders.json (when non-null)
    - Test: every forecast record's atmId exists in atms.json
    - Test: every DSR record's atmId exists in atms.json
    - Use fast-check with `{ numRuns: 100 }`
    - **Validates: Requirements 7.5**

- [x] 5. App shell, routing, and role context
  - [x] 5.1 Implement RoleContext (`src/context/RoleContext.tsx`)
    - Create context with `role`, `setRole`, `isInternal` computed property
    - Default to Admin role on initial load
    - Provide fallback value if consumed outside Provider (log warning in dev)
    - _Requirements: 1.3, 2.1, 2.2_

  - [x] 5.2 Implement Sidebar and TopBar layout components
    - `src/components/layout/Sidebar.tsx`: collapsible (240–280px expanded, 56–72px collapsed), transition within 300ms, icon + label when expanded, icon-only when collapsed, active item highlighted with distinct fill/weight
    - `src/components/layout/TopBar.tsx`: displays current role label, contains RoleSwitcher
    - `src/components/layout/RoleSwitcher.tsx`: dropdown to switch roles, updates context within 200ms
    - Auto-collapse sidebar to icon-only rail when viewport < 1024px
    - _Requirements: 1.1, 1.2, 1.6, 2.3, 10.1_

  - [x] 5.3 Implement App routing and AppShell
    - `src/app/routes.tsx`: define routes `/` (redirect → /dsr), `/dsr`, `/forecast`, `/cit`, `/invoice`, `*` (NotFound)
    - `src/app/AppShell.tsx`: layout wrapping sidebar + topbar + `<Outlet />`
    - `src/app/App.tsx`: providers (QueryClientProvider, RoleProvider, BrowserRouter) wrapping AppShell
    - `src/components/NotFound.tsx`: 404 page with link to /dsr
    - Gate navigation: Vendor role sees only CIT + Invoice nav items
    - Use semantic HTML (`<nav>`, `<main>`, `<header>`)
    - _Requirements: 1.4, 1.5, 2.4, 2.5, 9.1, 9.2, 9.3, 9.4, 10.3_

- [x] 6. Checkpoint - Core infrastructure complete
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. DSR Dashboard feature
  - [x] 7.1 Implement DSR Dashboard screen (`src/features/dsr/`)
    - Create `dsr.types.ts` with DsrRecord interface
    - Create `useDsrData.ts` TanStack Query hook filtering by selected date
    - Create `DsrTable.tsx` with columns: ATM ID, Location, Vendor, Beginning Balance, Cash In, Cash Out, Ending Balance, Status
    - Create `DsrSummary.tsx` with summary cards: total Beginning Balance, Cash In, Cash Out, Ending Balance
    - Create `DsrDashboard.tsx` composing DatePicker + Summary + Table
    - Status badge derived from ending balance (Critical/Low/Normal with icon + label)
    - Monetary values right-aligned, tabular-nums, IDR formatted
    - Column sorting on header click (toggle asc/desc)
    - Date selector defaults to mock current date, changing date reloads data
    - Empty state when no records for selected date
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8_

  - [x] 7.2 Write unit tests for DSR Dashboard
    - Test summary card totals compute correctly
    - Test status badge mapping from ending balance
    - Test date filtering returns correct records
    - _Requirements: 3.4, 3.6, 3.8_

- [x] 8. Forecast View feature
  - [x] 8.1 Implement Forecast View screen (`src/features/forecast/`)
    - Create `forecast.types.ts` with ForecastRecord interface
    - Create `useForecastData.ts` TanStack Query hook
    - Create `ForecastTable.tsx` with columns: ATM ID, Location, Vendor, Current Balance, Predicted Usage H+1, Predicted Usage H+2, Recommended Replenishment, Priority
    - Create `ScheduleList.tsx` showing replenishment schedule grouped by vendor then date for next 3 days (H+1, H+2, H+3), min 5 entries
    - Create `ForecastView.tsx` composing FilterSelect (priority) + SummaryCard (total replenishment) + ForecastTable + ScheduleList
    - Default sort: Priority descending (High → Medium → Low)
    - Priority filter: show only matching ATMs, update summary total to filtered subset
    - Clear filter restores full table and total
    - Empty state when filter matches zero records
    - All IDR amounts dot-separated thousands, right-aligned, tabular-nums
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.9_

  - [x] 8.2 Write unit tests for Forecast View
    - Test priority filter returns correct subset
    - Test summary total reflects filtered results
    - Test default sort order
    - Test empty state triggers on no-match filter
    - _Requirements: 4.4, 4.6, 4.7, 4.9_

- [x] 9. CIT Tracker feature
  - [x] 9.1 Implement CIT Tracker screen (`src/features/cit/`)
    - Create `cit.types.ts` with CitOrder interface
    - Create `useCitData.ts` TanStack Query hook
    - Create `CitTable.tsx` with columns: Order ID, ATM ID, Vendor, Order Date, Scheduled Date, Amount (IDR), Status, Evidence (clickable link or dash)
    - Create `CitSummary.tsx` showing order counts per status category, updated on filter
    - Create `CitTracker.tsx` composing status filter + vendor filter + summary + table
    - Status badges: Scheduled (info), In Transit (warning), Completed (success), Failed (danger) — each with icon + label
    - Compound filtering: status + vendor filters active simultaneously
    - Default sort: Scheduled Date descending
    - Amount: right-aligned IDR with thousands separators
    - Empty state when combined filters yield zero results; summary shows zero for all categories
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8_

  - [x] 9.2 Write unit tests for CIT Tracker
    - Test compound filter (status + vendor) returns correct intersection
    - Test summary counts match filtered result
    - Test empty state and zero counts on no-match
    - _Requirements: 5.4, 5.5, 5.6, 5.7_

- [x] 10. Invoice Flow feature
  - [x] 10.1 Implement Invoice Flow screen (`src/features/invoice/`)
    - Create `invoice.types.ts` with Invoice and InvoiceLineItem interfaces
    - Create `useInvoiceData.ts` TanStack Query hook
    - Create `InvoiceList.tsx` with columns: Invoice Number, Vendor, Period, Total Amount (IDR), Line Items Count, Validation Status badge
    - Create `InvoiceDetail.tsx` showing line items table (Description, Invoiced Amount, Matched Order Reference, Expected Amount, Variance, Match Status badge) + validator/approver fields + Approve button
    - Create `InvoiceFlow.tsx` composing WorkflowSteps + InvoiceList + InvoiceDetail
    - Three-step workflow indicator (Upload, Validate, Approve) — current step derived from selected invoice status
    - Row click shows detail panel
    - Match status badges: Matched (success), Mismatch (danger), Pending Review (warning)
    - Approve button visible only when Manager role AND invoice is Validated
    - Clicking Approve: update status to Approved in UI, show confirmation with invoice number, approver name, timestamp
    - Maker-checker: show validator and approver as separate labeled fields
    - Non-Manager roles: hide Approve button, read-only detail
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9_

  - [x] 10.2 Write unit tests for Invoice Flow
    - Test Approve button visibility logic (Manager + Validated only)
    - Test approval updates status and shows confirmation
    - Test read-only mode for non-Manager roles
    - Test workflow step derivation from invoice status
    - _Requirements: 6.6, 6.7, 6.9_

- [x] 11. Checkpoint - All feature screens complete
  - Ensure all tests pass, ask the user if questions arise.

- [x] 12. Docker containerization
  - [x] 12.1 Create Docker configuration files
    - Create multi-stage `Dockerfile`: stage 1 runs `pnpm install && pnpm build`, stage 2 copies dist into `nginx:alpine`
    - Create `docker-compose.yml` with single frontend service mapping port 80 → host 3000 (configurable via env var)
    - Create `.dockerignore` excluding node_modules, dist, .env, .git
    - Create `nginx.conf` with `try_files $uri $uri/ /index.html` for SPA client-side routing support
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 9.5_

  - [x] 12.2 Write integration test for Docker build
    - Verify Dockerfile builds successfully
    - Verify nginx serves index.html on root path
    - Verify client-side routes return 200 (not 404)
    - _Requirements: 8.3, 8.5_

- [x] 13. Responsive layout and accessibility polish
  - [x] 13.1 Implement responsive breakpoints and accessibility compliance
    - Viewport < 1024px: sidebar renders as icon-only rail by default
    - Viewport 768px–1023px: no horizontal overflow, all elements reachable
    - Data tables wrapped in horizontally scrollable containers
    - Semantic HTML: `<nav>`, `<main>`, `<header>`, `<table>`
    - All interactive elements: min 44×44px touch target
    - Status indicators: always icon + text label (never color alone)
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_

- [x] 14. E2E tests with Playwright
  - [x] 14.1 Write E2E tests for critical user flows
    - Navigation flow: sidebar links route correctly without full reload
    - Role switching: Vendor hides DSR/Forecast; internal roles restore all items
    - DSR date selection: changing date updates table content
    - CIT filtering: compound filters narrow results, clear restores
    - Invoice approval: Manager + Validated invoice → Approve → status updates
    - 404 handling: unknown path shows NotFound with link to /dsr
    - Responsive sidebar: viewport < 1024px collapses to icon-only
    - _Requirements: 1.5, 2.4, 3.8, 5.4, 6.7, 9.3, 10.1_

- [x] 15. Final checkpoint - All tests pass and app is demo-ready
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- TanStack Query wraps mock data to simulate async — swap queryFn for real API later
- All monetary formatting uses Indonesian locale (dot thousands separator, no decimals)
- Design system tokens (OKLCH) from the steering doc must be applied throughout

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "2.10"] },
    { "id": 2, "tasks": ["2.1", "2.3", "2.5", "2.8"] },
    { "id": 3, "tasks": ["2.2", "2.4", "2.6", "2.7", "2.9", "3.1", "3.2", "3.3"] },
    { "id": 4, "tasks": ["4.1", "5.1"] },
    { "id": 5, "tasks": ["4.2", "5.2", "5.3"] },
    { "id": 6, "tasks": ["7.1", "8.1", "9.1", "10.1"] },
    { "id": 7, "tasks": ["7.2", "8.2", "9.2", "10.2"] },
    { "id": 8, "tasks": ["12.1", "13.1"] },
    { "id": 9, "tasks": ["12.2", "14.1"] }
  ]
}
```
