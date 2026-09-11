# Requirements Document

## Introduction

The Cash Flow Monitoring page provides an at-a-glance operational dashboard for CMS internal users to monitor daily cash positions across vendors and ATMs. The page surfaces end-of-day (EOD) summary data from a read replica, displaying aggregate KPI cards, a 7-day bar chart of cash flow per vendor, and a real-time ATM cash level table. This is a read-only monitoring view with no write actions.

## Glossary

- **CashFlowPage**: The React page component rendered at the `/cash-flow` route, responsible for composing the Cash Flow Monitoring UI
- **StatsCard**: A metric card component displaying a single KPI value with label, icon, and optional trend indicator
- **VendorBarChart**: A Recharts-based grouped bar chart showing daily cash flow amounts per vendor over a 7-day period
- **AtmLevelTable**: A table displaying ATM identifiers alongside a progress bar and percentage indicating remaining cash level
- **PageHeader**: The existing shared component (`@/components/ui/PageHeader`) providing eyebrow, title, description, and action slots
- **DataSourceBadge**: A pill-shaped Badge component indicating the data freshness source (EOD H-1)
- **useCashFlowData**: A TanStack Query-based hook that fetches or mocks cash flow monitoring data for the page
- **Sidebar**: The existing navigation sidebar component reading from `NAV_GROUPS` in `constants.ts`
- **Vendor**: A CIT (Cash-in-Transit) service provider company (e.g., Abacus, Bijak Jakarta, Advantage)

## Requirements

### Requirement 1: Navigation Integration

**User Story:** As an internal CMS user, I want to access Cash Flow Monitoring from the sidebar navigation, so that I can quickly view cash positions without memorizing URLs.

#### Acceptance Criteria

1. THE Sidebar SHALL display a "Cash Flow Monitoring" link within a "Monitoring" navigation group with an `Activity` lucide-react icon.
2. WHEN the user navigates to `/cash-flow`, THE CashFlowPage SHALL render the Cash Flow Monitoring view.
3. WHEN the `/cash-flow` route is active, THE Sidebar SHALL highlight the "Cash Flow Monitoring" link with the active style.
4. THE Sidebar SHALL position the "Monitoring" navigation group after the existing "Operations" and "Control" groups.

### Requirement 2: Page Header

**User Story:** As an internal CMS user, I want to see a clear title and context about the data source, so that I understand what the page shows and how fresh the data is.

#### Acceptance Criteria

1. THE CashFlowPage SHALL render a PageHeader with the title "Cash Flow Monitoring".
2. THE PageHeader SHALL display a description reading "Baca dari read replica · basis summary EOD per 00:00 · periode Jul 2026".
3. THE CashFlowPage SHALL render a DataSourceBadge with the `info` variant, a `Database` icon, and the label "Sumber: EOD H-1" adjacent to the page header.

### Requirement 3: KPI Stats Cards

**User Story:** As an operations manager, I want to see key cash flow metrics at a glance, so that I can assess the overall cash position without drilling into details.

#### Acceptance Criteria

1. THE CashFlowPage SHALL render exactly 4 StatsCard components in a responsive grid row.
2. THE first StatsCard SHALL display the label "Total Kas Beredar" with a `Banknote` icon and the aggregate cash-in-circulation value formatted in Indonesian Rupiah with "M" suffix (e.g., "Rp 48,2 M").
3. THE second StatsCard SHALL display the label "Saldo Vault Vendor" with a `Landmark` icon and the total vendor vault balance formatted in Indonesian Rupiah with "M" suffix.
4. THE third StatsCard SHALL display the label "Kas di Mesin ATM" with a `Cpu` icon and the total ATM cash value formatted in Indonesian Rupiah with "M" suffix.
5. THE fourth StatsCard SHALL display the label "Drop CIT Hari Ini" with a `Truck` icon, the daily CIT drop amount formatted in Indonesian Rupiah with "M" suffix, and the order count (e.g., "6 order").
6. WHEN a StatsCard value has an associated trend, THE StatsCard SHALL display a trend indicator showing direction (up arrow or down arrow) and percentage change with appropriate semantic color (success for up, danger for down).
7. THE StatsCard grid SHALL collapse from a 4-column layout to a 2-column layout on viewports narrower than 768px, and to a 1-column layout on viewports narrower than 480px.

### Requirement 4: Vendor Bar Chart

**User Story:** As an operations manager, I want to see daily cash flow broken down by vendor over the past 7 days, so that I can identify vendor-level trends and anomalies.

#### Acceptance Criteria

1. THE CashFlowPage SHALL render a VendorBarChart panel within the left section of a split layout.
2. THE VendorBarChart panel header SHALL display the title "Cash Flow Harian per Vendor" and a pill badge indicating the period "7 hari" with a `Calendar` icon.
3. THE VendorBarChart SHALL render a grouped bar chart using the Recharts library with one bar group per day for the past 7 days.
4. THE VendorBarChart SHALL display one color-coded bar per vendor within each day group, using distinct OKLCH colors that meet WCAG 2.1 AA contrast ratio against the chart background.
5. THE VendorBarChart SHALL render a legend below the chart displaying vendor names with their corresponding color swatches.
6. THE VendorBarChart container SHALL maintain a minimum height of 240px to prevent chart compression.

### Requirement 5: ATM Cash Level Table

**User Story:** As an operations manager, I want to see the current cash level of individual ATMs, so that I can identify machines requiring replenishment.

#### Acceptance Criteria

1. THE CashFlowPage SHALL render an AtmLevelTable panel within the right section of the split layout.
2. THE AtmLevelTable panel header SHALL display the title "Level Kas per ATM".
3. THE AtmLevelTable SHALL display columns for ATM identifier, a visual progress bar, and a percentage value.
4. THE AtmLevelTable SHALL render ATM identifiers in monospace font (e.g., "ATM-00417").
5. THE AtmLevelTable progress bar SHALL use semantic colors: success color (green) for levels at or above 50%, warning color (amber) for levels between 20% and 49%, and danger color (red) for levels below 20%.
6. THE AtmLevelTable percentage values SHALL use tabular-nums font-variant and right-align within the column.

### Requirement 6: Data Fetching

**User Story:** As a developer, I want a proper data-fetching hook using TanStack Query, so that the Cash Flow page is ready for Go backend integration with minimal refactoring.

#### Acceptance Criteria

1. THE useCashFlowData hook SHALL use TanStack Query's `useQuery` with a unique query key of `['cash-flow', 'summary']`.
2. THE useCashFlowData hook SHALL return typed data conforming to a `CashFlowSummary` TypeScript interface that includes stats cards data, vendor chart data, and ATM level data.
3. THE useCashFlowData hook SHALL provide realistic mock data matching the ClickUp prototype values during development (total kas beredar: Rp 48,2 M, saldo vault: Rp 21,7 M, kas ATM: Rp 26,5 M, drop CIT: Rp 3,9 M / 6 orders).
4. WHILE the useCashFlowData hook is in a loading state, THE CashFlowPage SHALL display skeleton placeholders in place of the stats cards, chart, and table.
5. IF the useCashFlowData hook returns an error, THEN THE CashFlowPage SHALL display an error message with a retry action.

### Requirement 7: Responsive Layout

**User Story:** As an internal CMS user, I want the Cash Flow page to remain usable on different screen sizes, so that I can monitor cash positions from various devices.

#### Acceptance Criteria

1. THE split layout (chart + table) SHALL render as a two-column grid (ratio approximately 1.5fr : 1fr) on viewports 1024px and wider.
2. WHEN the viewport is narrower than 1024px, THE split layout SHALL stack the VendorBarChart above the AtmLevelTable in a single-column layout.
3. THE CashFlowPage SHALL render all content within the existing AppShell main content area without horizontal scrolling at any supported viewport width (320px minimum).

### Requirement 8: Feature Module Structure

**User Story:** As a developer, I want the Cash Flow feature organized following project conventions, so that the codebase remains consistent and maintainable.

#### Acceptance Criteria

1. THE CashFlowPage source code SHALL reside in `src/features/cash-flow/CashFlowScreen.tsx`.
2. THE useCashFlowData hook SHALL reside in `src/features/cash-flow/useCashFlowData.ts`.
3. THE TypeScript type definitions SHALL reside in `src/features/cash-flow/types.ts`.
4. THE sub-components (StatsCard, VendorBarChart, AtmLevelTable) SHALL reside in `src/features/cash-flow/` as separate module files.
5. THE feature module SHALL export the page component via a barrel `src/features/cash-flow/index.ts` file.
6. THE feature tests SHALL reside in `src/features/cash-flow/__tests__/` directory.

### Requirement 9: Accessibility

**User Story:** As a user relying on assistive technology, I want the Cash Flow page to be accessible, so that I can understand cash position data regardless of how I interact with the interface.

#### Acceptance Criteria

1. THE VendorBarChart SHALL include an accessible description via an `aria-label` attribute describing the chart purpose (e.g., "Bar chart showing daily cash flow per vendor for the past 7 days").
2. THE AtmLevelTable progress bars SHALL include `role="progressbar"` with `aria-valuenow`, `aria-valuemin`, and `aria-valuemax` attributes.
3. THE StatsCard trend indicators SHALL convey direction via text content (arrow character or sr-only label), not color alone.
4. THE page SHALL maintain a logical heading hierarchy (h1 for page title, h2 for panel titles).
