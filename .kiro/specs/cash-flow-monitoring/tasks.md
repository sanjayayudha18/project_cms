# Implementation Plan: Cash Flow Monitoring

## Overview

Implement a read-only Cash Flow Monitoring dashboard page at `/cash-flow` following the feature-module pattern. The page composes KPI stats cards, a Recharts vendor bar chart, and an ATM level table — all powered by a TanStack Query hook with mock data. Integration into the sidebar navigation and route configuration completes the feature.

## Tasks

- [x] 1. Set up feature module structure and types
  - [x] 1.1 Create TypeScript interfaces and type definitions
    - Create `src/features/cash-flow/types.ts` with all interfaces: `TrendDirection`, `CashLevelTier`, `StatsCardData`, `VendorDayFlow`, `VendorConfig`, `AtmLevel`, `CashFlowSummary`, `UseCashFlowDataReturn`
    - _Requirements: 8.3_

  - [x] 1.2 Create constants and vendor color configuration
    - Create `src/features/cash-flow/constants.ts` with `VENDOR_COLORS` array (5 vendors with OKLCH colors meeting AA contrast) and chart configuration values
    - _Requirements: 4.4, 8.5_

  - [x] 1.3 Install recharts dependency
    - Run `pnpm add recharts` in the frontend directory
    - _Requirements: 4.3_

- [x] 2. Implement data fetching hook
  - [x] 2.1 Create useCashFlowData TanStack Query hook
    - Create `src/features/cash-flow/useCashFlowData.ts` with `useQuery` using key `['cash-flow', 'summary']`, mock fetch function with realistic prototype data (Rp 48,2 M, Rp 21,7 M, Rp 26,5 M, Rp 3,9 M / 6 orders), 5-min staleTime
    - _Requirements: 6.1, 6.2, 6.3_

  - [x] 2.2 Write unit tests for useCashFlowData hook
    - Test query key correctness, mock data shape conformance to `CashFlowSummary` interface, loading/error state handling
    - _Requirements: 6.1, 6.2, 6.3_

- [x] 3. Implement StatsCard components
  - [x] 3.1 Create StatsCard component
    - Create `src/features/cash-flow/StatsCard.tsx` with KPI display: icon, label, formatted value, optional subtitle, and TrendIndicator sub-component with sr-only text and semantic color classes
    - _Requirements: 3.2, 3.3, 3.4, 3.5, 3.6, 9.3_

  - [x] 3.2 Create StatsCardGrid wrapper component
    - Create `src/features/cash-flow/StatsCardGrid.tsx` with responsive grid: 4-col at ≥768px, 2-col at ≥480px, 1-col below 480px
    - _Requirements: 3.1, 3.7_

  - [x] 3.3 Write property test for trend indicator correctness
    - **Property 1: Trend indicator accessibility and correctness**
    - Verify that for any `StatsCardData` with non-null trend, the rendered TrendIndicator includes text-based direction AND correct semantic color class (`text-success-fg` for up, `text-danger-fg` for down)
    - **Validates: Requirements 3.6, 9.3**

  - [x] 3.4 Write unit tests for StatsCard
    - Test rendering of all 4 card variants, value formatting, trend arrow display, responsive grid breakpoints
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

- [x] 4. Implement VendorBarChart component
  - [x] 4.1 Create VendorBarChart with Recharts
    - Create `src/features/cash-flow/VendorBarChart.tsx` using `ResponsiveContainer`, `BarChart`, `Bar`, `XAxis`, `YAxis`, `CartesianGrid`, `Tooltip`, `Legend` from recharts. Render grouped bars per vendor with OKLCH colors, min-height 240px, `aria-label` for accessibility
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 9.1_

  - [x] 4.2 Write property test for vendor color contrast
    - **Property 2: Vendor chart color contrast**
    - Verify that for any pair of vendor colors in `VENDOR_COLORS`, the WCAG 2.1 AA contrast ratio against chart background (`--n-0`: `oklch(0.992 0.003 29)`) is at least 3:1
    - **Validates: Requirements 4.4**

  - [x] 4.3 Write unit tests for VendorBarChart
    - Test chart renders with correct vendor bars, legend presence, aria-label, minimum height constraint
    - _Requirements: 4.1, 4.3, 4.5, 4.6, 9.1_

- [x] 5. Checkpoint - Verify stats and chart components
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Implement AtmLevelTable component
  - [x] 6.1 Create AtmLevelTable with progress bars
    - Create `src/features/cash-flow/AtmLevelTable.tsx` with `getCashLevelTier` function, `AtmLevelRow` sub-component rendering monospace ATM ID, semantic-colored progress bar with ARIA attributes (`role="progressbar"`, `aria-valuenow`, `aria-valuemin=0`, `aria-valuemax=100`), and right-aligned tabular-nums percentage
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 9.2_

  - [x] 6.2 Write property test for ATM level tier mapping
    - **Property 3: ATM level semantic color mapping**
    - Verify that for any percentage in [0, 100], `getCashLevelTier` returns `'success'` when ≥50, `'warning'` when 20–49, `'danger'` when <20
    - **Validates: Requirements 5.5**

  - [x] 6.3 Write property test for ARIA progressbar attributes
    - **Property 4: ARIA progressbar attribute completeness**
    - Verify that for any rendered `AtmLevelRow`, the progress bar has `role="progressbar"` with `aria-valuenow` equal to percentage, `aria-valuemin=0`, `aria-valuemax=100`
    - **Validates: Requirements 9.2**

  - [x] 6.4 Write unit tests for AtmLevelTable
    - Test tier color assignment, monospace rendering, tabular-nums alignment, ARIA attributes on progress bars
    - _Requirements: 5.3, 5.4, 5.5, 5.6, 9.2_

- [x] 7. Compose CashFlowScreen page
  - [x] 7.1 Create CashFlowScreen with loading/error states
    - Create `src/features/cash-flow/CashFlowScreen.tsx` composing PageHeader (title, description), DataSourceBadge (info variant, Database icon, "Sumber: EOD H-1"), StatsCardGrid, split layout grid (`grid-cols-[1.5fr_1fr]` at ≥1024px), VendorBarChartPanel, AtmLevelTablePanel, skeleton loading state, error state with retry
    - _Requirements: 2.1, 2.2, 2.3, 6.4, 6.5, 7.1, 7.2, 7.3, 9.4_

  - [x] 7.2 Create barrel export index.ts
    - Create `src/features/cash-flow/index.ts` exporting `CashFlowScreen`
    - _Requirements: 8.5_

  - [x] 7.3 Write unit tests for CashFlowScreen
    - Test page renders with all sections, loading skeleton display, error state with retry button, heading hierarchy (h1, h2)
    - _Requirements: 2.1, 6.4, 6.5, 9.4_

- [x] 8. Integrate navigation and routing
  - [x] 8.1 Add Monitoring nav group to constants.ts
    - Update `src/lib/constants.ts`: import `Activity` icon, append a "Monitoring" `NavGroup` with a single item `{ path: '/cash-flow', label: 'Cash Flow Monitoring', icon: Activity }` after existing groups
    - _Requirements: 1.1, 1.3, 1.4_

  - [x] 8.2 Add /cash-flow route to routes.tsx
    - Update `src/app/routes.tsx`: import `CashFlowScreen` from `@/features/cash-flow`, add route `{ path: '/cash-flow', element: <CashFlowScreen /> }` before the catch-all
    - _Requirements: 1.2_

  - [x] 8.3 Write integration tests for navigation and routing
    - Test sidebar shows "Cash Flow Monitoring" link in Monitoring group, route renders CashFlowScreen, active state highlights
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 9. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Recharts must be installed (task 1.3) before VendorBarChart implementation
- Mock data matches ClickUp prototype values for visual consistency during development

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "1.3"] },
    { "id": 1, "tasks": ["2.1"] },
    { "id": 2, "tasks": ["2.2", "3.1", "3.2"] },
    { "id": 3, "tasks": ["3.3", "3.4", "4.1"] },
    { "id": 4, "tasks": ["4.2", "4.3", "6.1"] },
    { "id": 5, "tasks": ["6.2", "6.3", "6.4"] },
    { "id": 6, "tasks": ["7.1"] },
    { "id": 7, "tasks": ["7.2", "7.3", "8.1", "8.2"] },
    { "id": 8, "tasks": ["8.3"] }
  ]
}
```
