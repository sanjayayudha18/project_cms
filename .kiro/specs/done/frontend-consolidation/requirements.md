# Requirements Document

## Introduction

Konsolidasi dua codebase frontend menjadi satu aplikasi kanonik. `frontend2/` (dibangun dari spec cms-app-foundation) menjadi basis arsitektur karena sudah memiliki TanStack Router file-based, auth Zustand, RBAC, stub API layer, OKLCH design tokens, error boundary, dan toast system. Delapan modul fitur dari `frontend/` akan di-port satu per satu ke dalam `frontend2/`, lalu `frontend2/` di-rename menjadi `frontend/` (menggantikan versi lama yang di-archive).

## Glossary

- **Target_App**: Aplikasi `frontend2/` yang menjadi basis kanonik setelah konsolidasi
- **Source_App**: Aplikasi `frontend/` yang berisi 8 modul fitur untuk di-port
- **Feature_Module**: Satu unit fitur bisnis yang terisolasi di `src/features/{nama}/`, berisi komponen, hooks, types, dan utils
- **Route_File**: File route TanStack Router di `src/routes/` yang mendefinisikan halaman dan loader-nya
- **NAV_CONFIG**: Array konfigurasi navigasi sidebar yang menentukan item menu, ikon, href, dan role-based visibility
- **Stub_API**: Service layer di Target_App yang mengembalikan data mock berdasarkan `VITE_API_MODE=stub`
- **Static_JSON**: File data JSON di `src/data/` yang digunakan Source_App sebagai sumber data sementara
- **Port_Process**: Proses memindahkan satu Feature_Module dari Source_App ke Target_App dengan adaptasi arsitektur
- **VendorPortal**: Subfolder `frontend/VendorPortal-Vite/` yang merupakan aplikasi terpisah dan TIDAK termasuk migrasi ini

## Requirements

### Requirement 1: Feature Module Porting Architecture

**User Story:** As a developer, I want each feature module ported independently into the Target_App architecture, so that every module works in isolation tanpa merusak modul lain.

#### Acceptance Criteria

1. WHEN a Feature_Module is ported, THE Target_App SHALL contain the module directory at `src/features/{module-name}/` with all component files, hook files, type definition files, and utility files that are referenced by the module's own imports
2. WHEN a Feature_Module is ported, THE Target_App SHALL have a corresponding Route_File at `src/routes/_protected/` that registers the module's page(s) in the TanStack Router file-based route tree under the protected layout
3. WHEN a Feature_Module is ported, THE Target_App SHALL pass its existing TypeScript compilation (`tsc --noEmit`) and all pre-existing unit tests (`pnpm test`) with zero new errors or failures
4. WHEN a Feature_Module uses `react-router-dom` APIs (useNavigate, useLocation, useParams, useSearchParams, Link, NavLink, Outlet), THE Port_Process SHALL replace them with TanStack Router equivalents (useNavigate, useRouterState, useParams, useSearch, Link from `@tanstack/react-router`)
5. WHEN a Feature_Module references `RoleContext` or `DevRoleSwitcher`, THE Port_Process SHALL replace those references with `useAuthStore` from `@/lib/auth/store`
6. WHEN a Feature_Module reads from Static_JSON files, THE Port_Process SHALL copy the relevant JSON files to Target_App's `src/data/` directory and update import paths to use the `@/data/{filename}.json` alias
7. IF a dependency required by a Feature_Module is missing from Target_App's `package.json`, THEN THE Port_Process SHALL add that dependency to Target_App's `package.json` and run `pnpm install` before the module is ported
8. WHEN a Feature_Module imports shared utilities from the source `src/lib/` (e.g., formatters, constants, filters), THE Port_Process SHALL either reuse the equivalent utility already present in Target_App's `src/lib/` or copy the missing utility file into Target_App's `src/lib/` with updated import paths
9. IF a ported Feature_Module references shared UI components from the source `src/components/` directory, THEN THE Port_Process SHALL verify those components exist in Target_App's `src/components/` or copy them with their dependencies before completing the port

### Requirement 2: Dashboard Module Port

**User Story:** As an operator, I want the full dashboard dari Source_App (MetricStrip, AttentionPanel, ReplenishmentSummary) tersedia di Target_App, so that saya mendapat overview lengkap operasi harian.

#### Acceptance Criteria

1. THE Target_App SHALL render `DashboardScreen` at the root protected route `/`, displaying a greeting header with the authenticated user's first name and today's date formatted in locale `id-ID`
2. WHEN `DashboardScreen` is rendered, THE Target_App SHALL display a `MetricStrip` component containing exactly 4 KPI cards (Managed Cash, ATM Availability, Today's Routes, Exceptions) with each card showing a label, a numeric value in `tabular-nums` formatting, and a meta description line, sourced from `dashboard-kpi.json`
3. WHEN `DashboardScreen` is rendered, THE Target_App SHALL display an `AttentionPanel` component listing attention items from `attention-items.json`, where each item shows a category-colored icon (danger, warning, or info), a title, a description, and a relative timestamp
4. WHEN `DashboardScreen` is rendered, THE Target_App SHALL display a `ReplenishmentSummary` component containing a data table with columns: Route (code + region), Vendor, Machines, Progress (bar showing completionCount/machineCount), Status (badge), and Value (IDR-formatted, right-aligned), sourced from `replenishment-schedules.json` and sorted by status priority
5. THE Target_App dashboard SHALL read metric data exclusively from static JSON files (`dashboard-kpi.json`, `attention-items.json`, `replenishment-schedules.json`) via direct import, replacing the existing Stub_API-based dashboard implementation (MetricCard + ActivityFeed)
6. IF any of the 3 static JSON data files fails to load or contains malformed data, THEN THE Target_App SHALL display an error state with an error indicator icon, a message describing the failure, and a retry action, without crashing the application
7. THE Target_App dashboard layout SHALL use a responsive grid: single column below 760px, a 2-column layout (ReplenishmentSummary at approximately 1.55fr and AttentionPanel at minimum 300px) at 1080px and above

### Requirement 3: Cash Flow Module Port

**User Story:** As a cash management operator, I want the cash flow monitoring screen available di Target_App, so that saya bisa memonitor arus kas ATM harian.

#### Acceptance Criteria

1. WHEN the operator navigates to `/cash-flow`, THE Target_App SHALL render `CashFlowScreen` via a Route_File registered in the TanStack Router protected route tree
2. WHEN the `/cash-flow` route is rendered, THE Target_App SHALL display `StatsCardGrid`, `AtmLevelTable`, and `VendorBarChart` components within the cash-flow feature layout
3. THE Target_App SHALL list `recharts` as a production dependency in `package.json` so that `VendorBarChart` can render bar charts
4. THE Target_App cash-flow module SHALL read ATM-level data from the `atms.json` Static_JSON file
5. THE NAV_CONFIG SHALL include a navigation entry for cash-flow with label "Cash Flow Monitoring", icon from Lucide, and path `/cash-flow`
6. IF the cash-flow data fetch fails or the JSON is empty, THEN THE Target_App SHALL display an error message indicating the failure and a retry control

### Requirement 4: CIT Tracker Module Port

**User Story:** As an ATM support operator, I want the CIT (Cash-in-Transit) tracker available di Target_App, so that saya bisa melacak status order CIT.

#### Acceptance Criteria

1. THE Target_App SHALL register a route at `/cit` within the protected layout, rendering the `CitTracker` component
2. WHEN the `/cit` route is active, THE Target_App SHALL display a CitSummary component showing counts per status category (Scheduled, In Transit, Completed, Failed) and a CitTable component listing CIT orders below it
3. THE Target_App CitTable SHALL use TanStack Table and display columns: Order ID, ATM ID, Vendor, Order Date, Scheduled Date, Amount (IDR, right-aligned), Status (badge), and Evidence (link)
4. THE Target_App cit module SHALL read CIT order data from the `src/data/cit-orders.json` Static_JSON file
5. THE NAV_CONFIG SHALL include an entry for CIT tracker with label "CIT Tracker", path `/cit`, and icon from Lucide React
6. WHEN the user selects a status or vendor filter, THE CitTracker SHALL display only CIT orders matching the selected filter values, updating both CitSummary counts and CitTable rows
7. IF the filtered CIT order list is empty, THEN THE Target_App SHALL display an empty state message indicating no orders match the current filters

### Requirement 5: DSR Dashboard Module Port

**User Story:** As an ATM support operator, I want the DSR (Daily Status Report) dashboard and table views available di Target_App, so that saya bisa melihat ringkasan dan detail DSR yang sudah diunggah.

#### Acceptance Criteria

1. WHEN the dsr module is ported, THE Target_App SHALL register a Route_File at `/forecasting/dsr-dashboard` rendering `DsrDashboard` as a child of the protected route
2. WHEN DSR data is loaded, THE Target_App DsrDashboard page SHALL render a DsrSummary component displaying aggregated totals (Total Beginning Balance, Total Cash In, Total Cash Out, Total Ending Balance) and a DsrTable component listing individual ATM DSR records
3. THE Target_App dsr module SHALL use TanStack Table for DsrTable rendering with columns: ATM ID, Location, Vendor, Beginning Balance, Cash In, Cash Out, Ending Balance, and Status
4. THE Target_App dsr module SHALL read DSR data from the `dsr.json` Static_JSON file
5. THE Target_App SHALL keep the existing DSR upload route at `/forecasting/dsr-upload` functional and navigable, coexisting with the new `/forecasting/dsr-dashboard` route without modifying the upload feature's behavior
6. WHEN the dsr-dashboard route is registered, THE NAV_CONFIG SHALL include a navigation entry with href `/forecasting/dsr-dashboard` within the forecasting group
7. IF no DSR records are available, THEN THE Target_App DsrDashboard SHALL display an empty state message indicating no data is available

### Requirement 6: Forecast Module Port

**User Story:** As a cash management operator, I want the forecast view and schedule list available di Target_App, so that saya bisa melihat proyeksi kebutuhan kas dan jadwal pengisian.

#### Acceptance Criteria

1. THE Target_App SHALL register a route at `/forecasting/forecast` that renders the `ForecastView` component
2. WHEN the user navigates to the forecast route, THE Target_App SHALL display both the `ForecastTable` component showing forecast records and the `ScheduleList` component showing replenishment schedules within the `ForecastView` page
3. THE Target_App forecast module SHALL read forecast data from the `forecast.json` Static_JSON file
4. THE NAV_CONFIG SHALL include a navigation entry for forecast within the forecasting navigation group, visible to internal roles
5. IF the `forecast.json` file contains zero records or fails to parse, THEN THE Target_App SHALL display an empty state message within the `ForecastView` instead of an empty table

### Requirement 7: Invoice Module Port

**User Story:** As a WMO/Finance operator, I want the invoice flow screen (list + detail) available di Target_App, so that saya bisa melihat dan mengelola invoice vendor.

#### Acceptance Criteria

1. WHEN the invoice module is ported, THE Target_App SHALL register a route at `/invoice/list` rendering `InvoiceFlow` that contains an invoice list table and an expandable invoice detail view
2. WHEN a user navigates to `/invoice/list`, THE Target_App SHALL display the invoice list table showing columns: invoice number, period, total amount, line items count, and validation status sourced from the `invoices.json` Static_JSON file
3. WHEN a user clicks or activates an invoice row in the list, THE Target_App SHALL expand the row to reveal the `InvoiceDetail` view showing line items with description, invoiced amount, matched order reference, expected amount, variance, and match status
4. IF the `invoices.json` Static_JSON file fails to load or returns no records, THEN THE Target_App SHALL display an empty state message indicating no invoices are available
5. WHEN the invoice route is registered, THE NAV_CONFIG SHALL include a navigation entry for invoice list within the invoice group with appropriate role access

### Requirement 8: Reconciliation Module Port

**User Story:** As a WMO/Finance operator, I want the reconciliation screen available di Target_App, so that saya bisa melihat dan menyelesaikan exception rekonsiliasi.

#### Acceptance Criteria

1. WHEN the reconciliation module is ported, THE Target_App SHALL register a route at `/invoice/reconciliation` rendering `ReconciliationScreen`
2. THE Target_App reconciliation module SHALL read exception data from the `reconciliation-exceptions.json` Static_JSON file
3. WHEN the reconciliation module is active, THE NAV_CONFIG SHALL enable the existing reconciliation entry (set `disabled: false`) at `/invoice/reconciliation`
4. THE Target_App reconciliation module SHALL preserve the `reconciliation.utils.ts` logic from Source_App with identical filtering behavior
5. IF the `reconciliation-exceptions.json` file is empty or contains zero records, THEN THE Target_App SHALL display an empty-state message indicating no exceptions are available

### Requirement 9: Replenishment Module Port

**User Story:** As a cash management operator, I want the replenishment screen available di Target_App, so that saya bisa melihat jadwal dan status pengisian ulang ATM.

#### Acceptance Criteria

1. WHEN the replenishment module is ported, THE Target_App SHALL register a route at `/replenishment` that renders the `ReplenishmentScreen` component
2. THE Target_App replenishment module SHALL read and parse all records from the `replenishment-schedules.json` Static_JSON file
3. WHEN the replenishment route is registered, THE NAV_CONFIG SHALL include a navigation entry with path `/replenishment`, label in Bahasa Indonesia, and icon from Lucide React, visible to all authenticated internal users
4. THE Target_App replenishment module SHALL preserve the `sortByStatusPriority` and `filterSchedules` functions from Source_App `replenishment.utils.ts` with identical input/output behavior
5. IF the `replenishment-schedules.json` file fails to load or contains no records, THEN THE Target_App SHALL render the ReplenishmentScreen with an empty-state indicator and no application error

### Requirement 10: Navigation Configuration Update

**User Story:** As a user, I want all ported features visible dan accessible via sidebar navigation, so that saya bisa navigasi ke semua modul yang tersedia.

#### Acceptance Criteria

1. WHEN all feature modules are ported, THE NAV_CONFIG SHALL include entries for: dashboard, cash-flow, cit, dsr-dashboard, forecast, invoice list, reconciliation, and replenishment — totalling exactly 8 new or updated navigation entries
2. THE NAV_CONFIG entries SHALL use Bahasa Indonesia labels consistent with Source_App's navigation naming
3. THE NAV_CONFIG entries SHALL assign role-based visibility following the RBAC pattern defined in Target_App's auth system
4. WHEN a NAV_CONFIG entry that was previously `disabled: true` becomes active due to porting, THE Target_App SHALL set that entry's `disabled` field to `false`
5. THE NAV_CONFIG SHALL NOT include any entry pointing to the VendorPortal application

### Requirement 11: Dependency Alignment

**User Story:** As a developer, I want all shared and new dependencies installed and compatible in Target_App, so that ported features compile and render without missing module errors.

#### Acceptance Criteria

1. WHEN the cash-flow module is ported, THE Target_App `package.json` SHALL include `recharts` (version ^3.x) as a production dependency and the application SHALL compile without errors when importing recharts components
2. THE Target_App SHALL retain all existing production dependencies (`@tanstack/react-router`, `@tanstack/react-query`, `@tanstack/react-table`, `zustand`, `lucide-react`, `xlsx`, `zod`, `react-hook-form`, `clsx`, `tailwind-merge`)
3. IF a ported Feature_Module imports a utility, type, or constant not present in Target_App, THEN THE Port_Process SHALL create or copy that utility into `src/lib/` or the feature's local directory, and the Target_App SHALL compile successfully with zero TypeScript errors
4. THE Target_App SHALL use Biome (as `@biomejs/biome` in devDependencies) for linting and formatting
5. WHEN a new dependency is added during porting, THE Port_Process SHALL verify that `pnpm install` completes without unresolved peer-dependency warnings and `pnpm build` produces a successful production bundle

### Requirement 12: Static Data Files Migration

**User Story:** As a developer, I want all required static JSON data files available in Target_App, so that ported features can read stub data during development.

#### Acceptance Criteria

1. WHEN feature modules are ported, THE Target_App SHALL contain exactly the following 10 Static_JSON files in `src/data/`: `atms.json`, `attention-items.json`, `cit-orders.json`, `dashboard-kpi.json`, `dsr.json`, `forecast.json`, `invoices.json`, `reconciliation-exceptions.json`, `replenishment-schedules.json`, `vendors.json`
2. THE Static_JSON files SHALL be exact copies from Source_App's `src/data/` directory with no structural modification
3. THE Target_App `src/data/` directory SHALL NOT contain any file that exists only in VendorPortal's `src/data/` directory (i.e., `evidence.json`, `notifications.json`, `orders.json`, `schedules.json`)

### Requirement 13: Test Preservation

**User Story:** As a developer, I want existing tests from both codebases preserved and passing, so that regressions are caught during and after porting.

#### Acceptance Criteria

1. WHEN a Feature_Module with existing `__tests__/` directory is ported, THE Target_App SHALL include those test files with all import paths updated to use Target_App's path aliases (`@/` prefix)
2. WHEN a module port is completed, THE Target_App SHALL pass `pnpm test` with zero test failures, including all pre-existing Target_App tests
3. WHEN test files reference `react-router-dom` test utilities (e.g., `MemoryRouter`, `useNavigate` mocks), THE Port_Process SHALL replace them with TanStack Router test utilities or remove router-specific mocking as appropriate
4. THE Target_App SHALL use Vitest as the test runner with `fast-check` available for property-based tests
5. IF a ported test file fails after import path and router utility adaptation, THEN THE Port_Process SHALL fix the test to pass against the ported component's actual behavior without removing test assertions or reducing coverage

### Requirement 14: Final Rename and Archive

**User Story:** As a developer, I want the consolidation finalized by renaming `frontend2/` to `frontend/`, so that the project has one canonical frontend directory.

#### Acceptance Criteria

1. WHEN all 8 feature modules are ported and each passes `pnpm build` and `pnpm test` in Target_App, THE Port_Process SHALL move the VendorPortal subfolder from `frontend/VendorPortal-Vite/` to the project root as `VendorPortal-Vite/` so it is independent of the frontend directory
2. WHEN the VendorPortal subfolder has been moved out, THE Port_Process SHALL rename the original `frontend/` directory to `frontend-archive/`
3. WHEN the original frontend is archived, THE Port_Process SHALL rename `frontend2/` to `frontend/`
4. WHEN the rename is complete, THE Target_App SHALL pass `pnpm build` with exit code 0 from the new `frontend/` path
5. WHEN the rename is complete, THE Target_App SHALL pass `pnpm test` with exit code 0 and zero test failures from the new `frontend/` path
6. IF `pnpm build` or `pnpm test` fails after the rename, THEN THE Port_Process SHALL update all internal path references (e.g., import aliases, tsconfig paths, vite config base) to reflect the new `frontend/` directory name before re-verifying
7. THE VendorPortal subfolder SHALL remain functional at its relocated project-root path (`VendorPortal-Vite/`) and SHALL NOT have any of its files modified by the rename operation

### Requirement 15: UI Language and Design Consistency

**User Story:** As a user, I want all UI labels and design tokens consistent across ported features, so that the app feels unified.

#### Acceptance Criteria

1. THE Target_App SHALL display all UI labels in Bahasa Indonesia after porting, with only the following English terms permitted in user-facing text: industry acronyms (CIT, DSR, ATM, WMO, CPC), product names (Dashboard), and unit abbreviations (Rp, pcs, kg)
2. WHEN ported features use Tailwind utility classes referencing design tokens, THE Target_App SHALL use OKLCH-based CSS custom properties from `tokens.css` and SHALL NOT contain hardcoded hex, RGB, or HSL color values in ported component files
3. THE Target_App SHALL use the existing design token system (brand red hue 29, warm neutrals, semantic colors as defined in `tokens.css`) for all ported feature styling
4. WHEN ported features use color values that do not exist in Target_App's token system, THE Port_Process SHALL map them to the token with the same semantic role (e.g., Source_App success green maps to `--success-solid`, Source_App warning yellow maps to `--warning-solid`)
5. IF a ported Feature_Module contains English user-facing labels not in the permitted list defined in criterion 1, THEN THE Port_Process SHALL translate those labels to Bahasa Indonesia before the module is considered ported
