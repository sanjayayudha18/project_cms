# Implementation Plan: Frontend Consolidation

## Overview

Konsolidasi 8 modul fitur dari Source_App (`frontend/`) ke Target_App (`frontend2/`) secara incremental, lalu rename `frontend2/` menjadi `frontend/`. Setiap modul di-port satu per satu dengan verifikasi TypeScript compilation dan test pass di setiap langkah. Urutan port mengikuti dependency: Dashboard → Cash Flow → CIT → DSR → Forecast → Invoice → Reconciliation → Replenishment.

## Tasks

- [x] 1. Prepare shared infrastructure and dependencies
  - [x] 1.1 Copy static JSON data files from Source_App to Target_App
    - Copy all 10 required JSON files (`atms.json`, `attention-items.json`, `cit-orders.json`, `dashboard-kpi.json`, `dsr.json`, `forecast.json`, `invoices.json`, `reconciliation-exceptions.json`, `replenishment-schedules.json`, `vendors.json`) from `frontend/src/data/` to `frontend2/src/data/`
    - Ensure byte-for-byte copies with no structural modification
    - Do NOT copy VendorPortal-only files (`evidence.json`, `notifications.json`, `orders.json`, `schedules.json`)
    - _Requirements: 12.1, 12.2, 12.3_

  - [x] 1.2 Add missing dependencies to Target_App
    - Add `recharts` (version `^3.x`) to `frontend2/package.json` production dependencies
    - Run `pnpm install` and verify zero unresolved peer-dependency warnings
    - Verify `pnpm build` produces a successful production bundle
    - _Requirements: 11.1, 11.5_

  - [x] 1.3 Copy shared utility files from Source_App to Target_App
    - Copy `formatCurrency.ts`, `formatters.ts`, `filters.ts`, `deriveStatus.ts` from `frontend/src/lib/` to `frontend2/src/lib/utils/` (only if not already present in Target_App)
    - Merge any type constants (CitStatus, DsrStatus, etc.) from `constants.ts` into feature-local type files later
    - Update all import paths to use `@/lib/utils/` alias
    - Verify `tsc --noEmit` passes with zero errors
    - _Requirements: 1.8, 11.3_

  - [x] 1.4 Write property test for static JSON data integrity
    - **Property 1: Static JSON Data Integrity**
    - Verify that each of the 10 static JSON files in Target_App is byte-for-byte identical to Source_App's copy
    - Use `fast-check` to generate arbitrary file selections from the required set and compare contents
    - **Validates: Requirements 1.6, 12.2**

- [x] 2. Port Dashboard module
  - [x] 2.1 Create Dashboard feature directory and components
    - Create `frontend2/src/features/dashboard/` directory
    - Port `DashboardScreen`, `MetricStrip`, `AttentionPanel`, `ReplenishmentSummary` components from Source_App
    - Create `frontend2/src/features/dashboard/types.ts` with local type definitions
    - Create `frontend2/src/features/dashboard/index.ts` barrel export
    - Replace any `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore` from `@/lib/auth/store`
    - Update all import paths to use `@/` aliases
    - Replace hardcoded color values with OKLCH design token references from `tokens.css`
    - Translate any English user-facing labels to Bahasa Indonesia (except permitted terms)
    - _Requirements: 1.1, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 2.5, 15.1, 15.2, 15.4_

  - [x] 2.2 Create Dashboard route file and replace existing dashboard
    - Create route file at `frontend2/src/routes/_protected/index.tsx` that renders `DashboardScreen` at `/`
    - Replace the existing Stub_API-based dashboard (MetricCard + ActivityFeed) with the ported full dashboard
    - Ensure the dashboard reads from static JSON files via direct import (not Stub_API)
    - Implement responsive grid layout: single column below 760px, 2-column at 1080px+ (ReplenishmentSummary ~1.55fr, AttentionPanel min 300px)
    - Implement error state with error indicator icon, message, and retry action for JSON load failures
    - _Requirements: 2.5, 2.6, 2.7_

  - [x] 2.3 Write property test for AttentionPanel rendering
    - **Property 2: AttentionPanel Renders All Required Fields**
    - Use `fast-check` to generate arbitrary arrays of attention items with category, title, description, and timestamp
    - Verify rendered output contains every item's title, description, category-appropriate icon indicator, and relative timestamp string
    - **Validates: Requirements 2.3**

  - [x] 2.4 Write property test for status priority sort
    - **Property 3: Status Priority Sort Invariant**
    - Use `fast-check` to generate arbitrary arrays of replenishment schedules with mixed statuses
    - Verify `sortByStatusPriority` produces a totally ordered array where each element has priority ≥ the next
    - **Validates: Requirements 2.4, 9.4**

  - [x] 2.5 Write unit tests for Dashboard components
    - Test `MetricStrip` renders exactly 4 KPI cards with correct data from `dashboard-kpi.json`
    - Test `DashboardScreen` greeting shows authenticated user's first name and today's date in `id-ID` locale
    - Test error state rendering when JSON import fails
    - Test empty state behavior
    - _Requirements: 2.1, 2.2, 2.6_

- [x] 3. Port Cash Flow module
  - [x] 3.1 Create Cash Flow feature directory and components
    - Create `frontend2/src/features/cash-flow/` directory
    - Port `CashFlowScreen`, `StatsCardGrid`, `AtmLevelTable`, `VendorBarChart` components from Source_App
    - Create `frontend2/src/features/cash-flow/types.ts` with local type definitions
    - Create `frontend2/src/features/cash-flow/index.ts` barrel export
    - Replace `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore`
    - Update import paths to `@/` aliases; read ATM data from `@/data/atms.json`
    - Replace hardcoded colors with OKLCH design tokens
    - Translate English labels to Bahasa Indonesia
    - _Requirements: 1.1, 1.4, 1.5, 3.1, 3.2, 3.4, 15.1, 15.2_

  - [x] 3.2 Create Cash Flow route file and NAV_CONFIG entry
    - Create route file at `frontend2/src/routes/_protected/cash-flow.tsx` rendering `CashFlowScreen`
    - Add NAV_CONFIG entry: `{ id: "cash-flow", label: "Cash Flow Monitoring", icon: Activity, href: "/cash-flow", roles: ["ATM_Support", "Cash_Management"], group: "general" }`
    - Implement error state with message and retry control for data fetch failures
    - Verify `recharts` import works correctly in `VendorBarChart`
    - _Requirements: 3.1, 3.3, 3.5, 3.6_

  - [x] 3.3 Write unit tests for Cash Flow components
    - Test `StatsCardGrid` renders correct statistics from ATM data
    - Test `VendorBarChart` renders without errors with valid data
    - Test error and empty state rendering
    - _Requirements: 3.2, 3.6_

- [x] 4. Port CIT Tracker module
  - [x] 4.1 Create CIT Tracker feature directory and components
    - Create `frontend2/src/features/cit/` directory
    - Port `CitTracker`, `CitSummary`, `CitTable` components from Source_App
    - Create `frontend2/src/features/cit/types.ts` with `CitStatus`, `CitOrder` type definitions
    - Create `frontend2/src/features/cit/index.ts` barrel export
    - Replace `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore`
    - Update import paths; read from `@/data/cit-orders.json`
    - Ensure `CitTable` uses TanStack Table with columns: Order ID, ATM ID, Vendor, Order Date, Scheduled Date, Amount (IDR, right-aligned), Status (badge), Evidence (link)
    - Implement filter logic for status and vendor selection updating both `CitSummary` counts and `CitTable` rows
    - Implement empty state message for no matching filter results
    - Replace hardcoded colors with OKLCH tokens; translate labels to Bahasa Indonesia
    - _Requirements: 1.1, 1.4, 1.5, 4.1, 4.2, 4.3, 4.4, 4.6, 4.7, 15.1, 15.2_

  - [x] 4.2 Create CIT route file and NAV_CONFIG entry
    - Create route file at `frontend2/src/routes/_protected/cit.tsx` rendering `CitTracker`
    - Add NAV_CONFIG entry: `{ id: "cit", label: "CIT Tracker", icon: Truck, href: "/cit", roles: ["ATM_Support", "Cash_Management"], group: "general" }`
    - _Requirements: 4.1, 4.5_

  - [x] 4.3 Write property test for CIT filter consistency
    - **Property 4: CIT Filter Consistency**
    - Use `fast-check` to generate arbitrary arrays of CIT orders and filter combinations (status + vendor)
    - Verify: (a) every returned row matches all active filters, (b) CitSummary counts match actual filtered counts per status, (c) no matching item is excluded
    - **Validates: Requirements 4.2, 4.6**

  - [x] 4.4 Write unit tests for CIT Tracker components
    - Test `CitSummary` renders correct counts per status category
    - Test `CitTable` columns render correctly with TanStack Table
    - Test filter interaction updates both summary and table
    - Test empty state message display
    - _Requirements: 4.2, 4.3, 4.6, 4.7_

- [x] 5. Checkpoint - Verify first 3 modules
  - Ensure `tsc --noEmit` passes, `pnpm test` passes with zero failures, and `pnpm build` succeeds
  - Ask the user if questions arise

- [x] 6. Port DSR Dashboard module
  - [x] 6.1 Create DSR feature directory and components
    - Create `frontend2/src/features/dsr/` directory
    - Port `DsrDashboard`, `DsrSummary`, `DsrTable` components from Source_App
    - Create `frontend2/src/features/dsr/types.ts` with local type definitions
    - Create `frontend2/src/features/dsr/index.ts` barrel export
    - Replace `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore`
    - Update import paths; read from `@/data/dsr.json`
    - Ensure `DsrTable` uses TanStack Table with columns: ATM ID, Location, Vendor, Beginning Balance, Cash In, Cash Out, Ending Balance, Status
    - Implement `DsrSummary` showing aggregated totals (Total Beginning Balance, Total Cash In, Total Cash Out, Total Ending Balance)
    - Implement empty state message for no DSR records
    - Replace hardcoded colors with OKLCH tokens; translate labels to Bahasa Indonesia
    - _Requirements: 1.1, 1.4, 1.5, 5.1, 5.2, 5.3, 5.4, 5.7, 15.1, 15.2_

  - [x] 6.2 Create DSR Dashboard route file and NAV_CONFIG entry
    - Create route file at `frontend2/src/routes/_protected/forecasting/dsr-dashboard.tsx` rendering `DsrDashboard`
    - Add NAV_CONFIG entry: `{ id: "dsr-dashboard", label: "DSR Dashboard", icon: BarChart3, href: "/forecasting/dsr-dashboard", roles: ["ATM_Support", "Cash_Management", "Vendor"], group: "forecasting" }`
    - Verify the existing DSR upload route at `/forecasting/dsr-upload` remains functional and unmodified
    - _Requirements: 5.1, 5.5, 5.6_

  - [x] 6.3 Write property test for DSR summary aggregation
    - **Property 5: DSR Summary Aggregation Invariant**
    - Use `fast-check` to generate arbitrary non-empty arrays of DSR records
    - Verify `totalBeginningBalance == sum(records.map(r => r.beginningBalance))` and likewise for Cash In, Cash Out, Ending Balance
    - **Validates: Requirements 5.2**

  - [x] 6.4 Write unit tests for DSR Dashboard components
    - Test `DsrSummary` renders correct aggregated totals
    - Test `DsrTable` columns render correctly
    - Test empty state message when no records available
    - _Requirements: 5.2, 5.3, 5.7_

- [x] 7. Port Forecast module
  - [x] 7.1 Create Forecast feature directory and components
    - Create `frontend2/src/features/forecast/` directory
    - Port `ForecastView`, `ForecastTable`, `ScheduleList` components from Source_App
    - Create `frontend2/src/features/forecast/types.ts` with local type definitions
    - Create `frontend2/src/features/forecast/index.ts` barrel export
    - Replace `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore`
    - Update import paths; read from `@/data/forecast.json`
    - Implement empty state message for zero records or parse failure
    - Replace hardcoded colors with OKLCH tokens; translate labels to Bahasa Indonesia
    - _Requirements: 1.1, 1.4, 1.5, 6.1, 6.2, 6.3, 6.5, 15.1, 15.2_

  - [x] 7.2 Create Forecast route file and NAV_CONFIG entry
    - Create route file at `frontend2/src/routes/_protected/forecasting/forecast.tsx` rendering `ForecastView`
    - Add NAV_CONFIG entry: `{ id: "forecast", label: "Forecasting", icon: TrendingUp, href: "/forecasting/forecast", roles: ["ATM_Support", "Cash_Management"], group: "forecasting" }`
    - _Requirements: 6.1, 6.4_

  - [x] 7.3 Write unit tests for Forecast components
    - Test `ForecastTable` renders forecast records correctly
    - Test `ScheduleList` renders replenishment schedules
    - Test empty state message display
    - _Requirements: 6.2, 6.5_

- [x] 8. Port Invoice module
  - [x] 8.1 Create Invoice feature directory and components
    - Create `frontend2/src/features/invoice/` directory
    - Port `InvoiceFlow`, `InvoiceDetail` components from Source_App
    - Create `frontend2/src/features/invoice/types.ts` with local type definitions
    - Create `frontend2/src/features/invoice/index.ts` barrel export
    - Replace `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore`
    - Update import paths; read from `@/data/invoices.json`
    - Implement invoice list table with columns: invoice number, period, total amount, line items count, validation status
    - Implement expandable row detail showing line items with: description, invoiced amount, matched order reference, expected amount, variance, match status
    - Implement empty state message for no invoices
    - Replace hardcoded colors with OKLCH tokens; translate labels to Bahasa Indonesia
    - _Requirements: 1.1, 1.4, 1.5, 7.1, 7.2, 7.3, 7.4, 15.1, 15.2_

  - [x] 8.2 Create Invoice route file and NAV_CONFIG entry
    - Create route file at `frontend2/src/routes/_protected/invoice/list.tsx` rendering `InvoiceFlow`
    - Add NAV_CONFIG entry: `{ id: "invoice-list", label: "Daftar Invoice", icon: FileText, href: "/invoice/list", roles: ["WMO", "Finance", "Vendor"], group: "invoice" }`
    - _Requirements: 7.1, 7.5_

  - [x] 8.3 Write property test for invoice expansion line items
    - **Property 6: Invoice Expansion Shows Complete Line Items**
    - Use `fast-check` to generate arbitrary invoice objects with N line items (N ≥ 1)
    - Verify expanding the row renders exactly N line item entries with all required fields
    - **Validates: Requirements 7.3**

  - [x] 8.4 Write unit tests for Invoice components
    - Test `InvoiceFlow` renders invoice list table correctly
    - Test row expansion shows `InvoiceDetail` with all line item fields
    - Test empty state message display
    - _Requirements: 7.2, 7.3, 7.4_

- [x] 9. Port Reconciliation module
  - [x] 9.1 Create Reconciliation feature directory and components
    - Create `frontend2/src/features/reconciliation/` directory
    - Port `ReconciliationScreen` and `reconciliation.utils.ts` from Source_App
    - Create `frontend2/src/features/reconciliation/types.ts` with local type definitions
    - Create `frontend2/src/features/reconciliation/index.ts` barrel export
    - Replace `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore`
    - Update import paths; read from `@/data/reconciliation-exceptions.json`
    - Preserve `reconciliation.utils.ts` filtering logic with identical behavior
    - Implement empty state message for zero exception records
    - Replace hardcoded colors with OKLCH tokens; translate labels to Bahasa Indonesia
    - _Requirements: 1.1, 1.4, 1.5, 8.1, 8.2, 8.4, 8.5, 15.1, 15.2_

  - [x] 9.2 Create Reconciliation route file and enable NAV_CONFIG entry
    - Create route file at `frontend2/src/routes/_protected/invoice/reconciliation.tsx` rendering `ReconciliationScreen`
    - Update existing NAV_CONFIG reconciliation entry: set `disabled: false` for the entry at `/invoice/reconciliation`
    - _Requirements: 8.1, 8.3_

  - [x] 9.3 Write property test for reconciliation filter equivalence
    - **Property 7: Reconciliation Filter Behavioral Equivalence**
    - Use `fast-check` to generate arbitrary arrays of reconciliation exceptions and filter criteria
    - Verify Target_App's `reconciliation.utils.ts` filter produces identical results to Source_App's version
    - **Validates: Requirements 8.4**

  - [x] 9.4 Write unit tests for Reconciliation components
    - Test `ReconciliationScreen` renders exception data correctly
    - Test filtering behavior matches expected output
    - Test empty state message display
    - _Requirements: 8.2, 8.4, 8.5_

- [x] 10. Checkpoint - Verify modules 4-7
  - Ensure `tsc --noEmit` passes, `pnpm test` passes with zero failures, and `pnpm build` succeeds
  - Ask the user if questions arise

- [x] 11. Port Replenishment module
  - [x] 11.1 Create Replenishment feature directory and components
    - Create `frontend2/src/features/replenishment/` directory
    - Port `ReplenishmentScreen` and `replenishment.utils.ts` (including `sortByStatusPriority` and `filterSchedules`) from Source_App
    - Create `frontend2/src/features/replenishment/types.ts` with local type definitions
    - Create `frontend2/src/features/replenishment/index.ts` barrel export
    - Replace `react-router-dom` imports with TanStack Router equivalents
    - Replace `RoleContext`/`useRole` with `useAuthStore`
    - Update import paths; read from `@/data/replenishment-schedules.json`
    - Preserve `sortByStatusPriority` and `filterSchedules` with identical input/output behavior
    - Implement empty state indicator for no records or load failure
    - Replace hardcoded colors with OKLCH tokens; translate labels to Bahasa Indonesia
    - _Requirements: 1.1, 1.4, 1.5, 9.1, 9.2, 9.4, 9.5, 15.1, 15.2_

  - [x] 11.2 Create Replenishment route file and NAV_CONFIG entry
    - Create route file at `frontend2/src/routes/_protected/replenishment.tsx` rendering `ReplenishmentScreen`
    - Add NAV_CONFIG entry: `{ id: "replenishment", label: "Pengisian Ulang", icon: Truck, href: "/replenishment", roles: ["ATM_Support", "Cash_Management"], group: "general" }`
    - _Requirements: 9.1, 9.3_

  - [x] 11.3 Write property test for replenishment filterSchedules equivalence
    - **Property 8: Replenishment filterSchedules Behavioral Equivalence**
    - Use `fast-check` to generate arbitrary arrays of replenishment schedules and filter criteria
    - Verify Target_App's `filterSchedules` produces identical results to Source_App's version
    - **Validates: Requirements 9.4**

  - [x] 11.4 Write unit tests for Replenishment components
    - Test `ReplenishmentScreen` renders schedule data correctly
    - Test `sortByStatusPriority` ordering
    - Test `filterSchedules` filtering
    - Test empty state indicator display
    - _Requirements: 9.2, 9.4, 9.5_

- [x] 12. Finalize Navigation Configuration
  - [x] 12.1 Verify and update NAV_CONFIG with all 8 module entries
    - Verify `src/lib/config/navigation.ts` contains exactly 8 new/updated entries: dashboard, cash-flow, cit, dsr-dashboard, forecast, invoice-list, reconciliation, replenishment
    - Ensure all labels use Bahasa Indonesia (except permitted English terms)
    - Ensure all entries have correct role-based visibility per RBAC pattern
    - Verify no entry points to VendorPortal
    - Set `disabled: false` for any previously disabled entries now active
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

  - [x] 12.2 Write property test for RBAC navigation filtering
    - **Property 9: RBAC Navigation Filtering Correctness**
    - Use `fast-check` to generate arbitrary user role combinations
    - Verify `filterNavByRoles` returns only entries where user roles match the entry's roles array; Admin sees all entries
    - **Validates: Requirements 10.3**

  - [x] 12.3 Write unit tests for NAV_CONFIG entries
    - Test all 8 entries present with correct href, label, icon, and roles
    - Test no VendorPortal entries exist
    - Test disabled entries are set to `false`
    - _Requirements: 10.1, 10.4, 10.5_

- [x] 13. Port tests and cross-cutting verification
  - [x] 13.1 Port existing test files from Source_App feature modules
    - Copy `__tests__/` directories from each ported Source_App feature module into corresponding Target_App feature directories
    - Update all import paths to use `@/` path alias prefix
    - Replace `react-router-dom` test utilities (`MemoryRouter`, `useNavigate` mocks) with TanStack Router test utilities
    - Fix any test failures caused by import path or router utility changes
    - Verify all ported tests pass alongside pre-existing Target_App tests
    - _Requirements: 13.1, 13.2, 13.3, 13.5_

  - [x] 13.2 Write property test for test import path alias compliance
    - **Property 12: Test Import Path Alias Compliance**
    - Use `fast-check` with file enumeration to verify all test files in ported feature `__tests__/` directories use `@/` prefix for cross-feature imports
    - **Validates: Requirements 13.1**

  - [x] 13.3 Write property test for no hardcoded colors in ported files
    - **Property 10: No Hardcoded Color Values in Ported Files**
    - Scan all `.tsx` and `.ts` files within `src/features/` directories for hex (`#[0-9a-fA-F]{3,8}`), RGB (`rgb(`), and HSL (`hsl(`) patterns
    - Verify zero matches in any ported module files
    - **Validates: Requirements 15.2**

  - [x] 13.4 Write property test for UI strings language compliance
    - **Property 11: UI Strings Language Compliance**
    - Verify all user-facing text in ported feature components is either Bahasa Indonesia or consists exclusively of permitted English terms
    - **Validates: Requirements 15.1, 15.5**

- [x] 14. Checkpoint - All modules ported verification
  - Ensure `tsc --noEmit` passes, `pnpm test` passes with zero failures, and `pnpm build` succeeds from `frontend2/`
  - Verify all 8 feature modules are present in `frontend2/src/features/`
  - Verify all 10 static JSON files are in `frontend2/src/data/`
  - Ask the user if questions arise

- [x] 15. Final rename and archive
  - [x] 15.1 Move VendorPortal to project root
    - Move `frontend/VendorPortal-Vite/` to project root as `VendorPortal-Vite/`
    - Verify VendorPortal files are intact and unmodified at new location
    - _Requirements: 14.1, 14.7_

  - [x] 15.2 Archive original frontend and rename frontend2
    - Rename `frontend/` to `frontend-archive/`
    - Rename `frontend2/` to `frontend/`
    - _Requirements: 14.2, 14.3_

  - [x] 15.3 Verify build and tests from new frontend path
    - Run `pnpm build` from new `frontend/` path — must exit with code 0
    - Run `pnpm test` from new `frontend/` path — must pass with zero failures
    - If any failures, update internal path references (import aliases, tsconfig paths, vite config base) to reflect `frontend/` directory name
    - _Requirements: 14.4, 14.5, 14.6_

- [x] 16. Final checkpoint - Consolidation complete
  - Ensure `pnpm build` and `pnpm test` pass from `frontend/`
  - Verify `VendorPortal-Vite/` at project root is functional and unmodified
  - Verify `frontend-archive/` exists as archive of old Source_App
  - Ask the user if questions arise

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation after batches of module ports
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples, edge cases, and component rendering
- The port sequence (Dashboard → Cash Flow → CIT → DSR → Forecast → Invoice → Reconciliation → Replenishment) follows dependency order from the design
- All ported code must use TypeScript with `@/` path aliases, TanStack Router, Zustand auth, and OKLCH design tokens
- Existing Target_App tests must continue passing throughout the entire porting process

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2"] },
    { "id": 1, "tasks": ["1.3"] },
    { "id": 2, "tasks": ["1.4", "2.1"] },
    { "id": 3, "tasks": ["2.2"] },
    { "id": 4, "tasks": ["2.3", "2.4", "2.5", "3.1"] },
    { "id": 5, "tasks": ["3.2", "4.1"] },
    { "id": 6, "tasks": ["3.3", "4.2"] },
    { "id": 7, "tasks": ["4.3", "4.4", "6.1"] },
    { "id": 8, "tasks": ["6.2", "7.1"] },
    { "id": 9, "tasks": ["6.3", "6.4", "7.2"] },
    { "id": 10, "tasks": ["7.3", "8.1"] },
    { "id": 11, "tasks": ["8.2", "9.1"] },
    { "id": 12, "tasks": ["8.3", "8.4", "9.2"] },
    { "id": 13, "tasks": ["9.3", "9.4", "11.1"] },
    { "id": 14, "tasks": ["11.2"] },
    { "id": 15, "tasks": ["11.3", "11.4", "12.1"] },
    { "id": 16, "tasks": ["12.2", "12.3", "13.1"] },
    { "id": 17, "tasks": ["13.2", "13.3", "13.4"] },
    { "id": 18, "tasks": ["15.1"] },
    { "id": 19, "tasks": ["15.2"] },
    { "id": 20, "tasks": ["15.3"] }
  ]
}
```
