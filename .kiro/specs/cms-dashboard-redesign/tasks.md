# Implementation Plan: CMS Dashboard Redesign

## Overview

Transform the existing four-screen CMS prototype into a dashboard-centric operational interface. The implementation progresses from foundational layers (tokens, data, utilities) through shared components (layout, UI) to feature screens (Dashboard, Replenishment, Reconciliation), ending with responsive behavior and integration wiring. Each task builds incrementally on prior work so no code is orphaned.

## Tasks

- [x] 1. Update design tokens and global styles
  - [x] 1.1 Update CSS custom properties in `src/styles/index.css`
    - Add full OKLCH color scales: brand red (hue 29, shades 50–900), neutrals (tinted hue 29, shades 0–900), semantic colors (success hue 155, warning hue 78, danger hue 12, info hue 245) with bg-tint, foreground, and solid variants
    - Add type scale tokens (Display 44/700, XL 28/700, LG 21/600, Base 16/400, SM 14/400, XS 12/500 uppercase)
    - Add spacing scale (4, 8, 12, 16, 24, 32, 48, 72px), radius tokens (sm 4px, md 6px, lg 10px, pill 999px), shadow tokens
    - Add system font stack: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif
    - Add page-enter animation keyframe (fade up 8px over 420ms with cubic-bezier(0.16, 1, 0.3, 1))
    - Add `prefers-reduced-motion` media query disabling all animations
    - _Requirements: 10.1, 10.2, 10.4, 10.5, 10.6, 10.8_

- [x] 2. Create mock data files and utility functions
  - [x] 2.1 Create mock data JSON files in `src/data/`
    - Create `dashboard-kpi.json` with managed cash, ATM availability, today's routes, exceptions (all fields per design)
    - Create `replenishment-schedules.json` with minimum 8 schedule records across 3+ regions and 3+ vendors
    - Create `reconciliation-exceptions.json` with minimum 5 records referencing valid ATM IDs from `atms.json`
    - Create `attention-items.json` with minimum 5 items spanning danger, warning, and info categories
    - Ensure referential integrity: ATM IDs reference `atms.json`, vendor names reference `vendors.json`, `atmOnline ≤ atmTotal`, `completionCount ≤ machineCount`, `difference = counted - escrow`
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6_

  - [x] 2.2 Create formatter utilities in `src/lib/formatters.ts`
    - Implement `formatIDR(value: number): string` — abbreviation with T/B/M suffix to one decimal
    - Implement `formatIDRFull(value: number): string` — dot-separated thousands
    - Implement `getGreeting(hour: number): string` — morning/afternoon/evening
    - Implement `formatFullDate(date: Date): string` — full weekday + date locale string
    - Implement `progressPercent(completed: number, total: number): number` — guarded against division by zero
    - Implement `formatBadgeCount(count: number): string | null` — null for 0, "99+" for >99
    - Implement `getInitials(name: string): string` — first letter of first + last name, uppercase
    - Implement `formatDifference(value: number): { text: string; colorClass: string }` — signed prefix with semantic class
    - _Requirements: 1.6, 2.5, 3.1, 3.3, 4.3, 7.5, 11.6_

  - [x] 2.3 Write property tests for `formatBadgeCount`
    - **Property 1: Badge count display formatting**
    - **Validates: Requirements 1.6**

  - [x] 2.4 Write property tests for `getInitials`
    - **Property 2: Avatar initials extraction**
    - **Validates: Requirements 2.5**

  - [x] 2.5 Write property tests for `getGreeting`
    - **Property 3: Time-of-day greeting**
    - **Validates: Requirements 3.1**

  - [x] 2.6 Write property tests for `progressPercent`
    - **Property 4: Progress bar calculation**
    - **Validates: Requirements 4.3**

  - [x] 2.7 Write property tests for `formatDifference`
    - **Property 8: Difference sign formatting**
    - **Validates: Requirements 7.5**

  - [x] 2.8 Write property test for mock data referential integrity
    - **Property 10: Mock data referential integrity**
    - **Validates: Requirements 11.5**

- [x] 3. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Implement filter and sort logic
  - [x] 4.1 Create replenishment filter/sort utilities in `src/features/replenishment/replenishment.utils.ts`
    - Implement `sortByStatusPriority(records: ReplenishmentSchedule[]): ReplenishmentSchedule[]` — delayed → in-transit → completed ordering
    - Implement `filterSchedules(records, region, vendor): ReplenishmentSchedule[]` — combined AND filter, "All" means no constraint
    - _Requirements: 4.7, 6.5, 6.6, 6.7, 6.8_

  - [x] 4.2 Create reconciliation filter utilities in `src/features/reconciliation/reconciliation.utils.ts`
    - Implement `filterExceptions(records, severity, exceptionType): ReconciliationException[]` — combined AND filter
    - _Requirements: 7.7, 7.8_

  - [x] 4.3 Write property tests for status-priority sort
    - **Property 5: Status-priority sort ordering**
    - **Validates: Requirements 4.7**

  - [x] 4.4 Write property tests for replenishment combined filter
    - **Property 6: Replenishment combined filter correctness**
    - **Validates: Requirements 6.5, 6.6, 6.7**

  - [x] 4.5 Write property test for filter clear restores full dataset
    - **Property 7: Filter clear restores full dataset**
    - **Validates: Requirements 6.8**

  - [x] 4.6 Write property tests for reconciliation filter
    - **Property 9: Reconciliation filter correctness**
    - **Validates: Requirements 7.7, 7.8**

- [x] 5. Implement shared UI components
  - [x] 5.1 Create `src/components/ui/ProgressBar.tsx`
    - Render horizontal bar with fill width based on `progressPercent(completed, total)`
    - Fill color varies by status: `--red-500` for in-transit, success solid for completed, warning solid for delayed
    - Display "X of Y" text label beside bar
    - _Requirements: 4.3, 4.4_

  - [x] 5.2 Create `src/components/ui/PageHeader.tsx`
    - Render eyebrow (XS uppercase, --red-deep color), title (clamp size, 700 weight), optional description (--muted, max-width 62ch)
    - Accept `actions` slot for button placement (max one primary + one/two secondary)
    - _Requirements: 12.1, 12.2_

  - [x] 5.3 Create `src/components/ui/NoticeBanner.tsx`
    - Render icon, bold title, description text
    - Support variant prop for warning/danger/info semantic coloring
    - _Requirements: 7.2_

  - [x] 5.4 Create Toast context and component in `src/components/ui/Toast.tsx` and `src/context/ToastContext.tsx`
    - Create `ToastContext` with `showToast(text, icon?)` method
    - Create `ToastProvider` wrapping app with state management (single active toast, replacement behavior)
    - Create `Toast` component rendering at bottom-right (bottom-left full-width on mobile <760px)
    - Animate in: opacity 0 → 1, translateY(12px) → 0 over 220ms; animate out in reverse
    - Auto-dismiss after 4 seconds; replacement restarts timer
    - Use `role="status"` and `aria-live="polite"` for accessibility
    - Style with --n-900 background, --n-0 text, design system shadow
    - _Requirements: 9.1, 9.2, 9.3, 9.6, 9.7_

  - [x] 5.5 Write unit tests for Toast context behavior
    - Test `showToast` triggers render, auto-dismiss timing, replacement behavior
    - _Requirements: 9.1, 9.7_

- [x] 6. Redesign sidebar navigation
  - [x] 6.1 Refactor `src/lib/constants.ts` — update navigation structure
    - Replace flat `NAV_ITEMS` array with `NAV_GROUPS: NavGroup[]` containing two groups: "Operations" (Dashboard, Replenishment, Cash Count, Reconciliation) and "Control" (Vendor Invoices, DSR Reports, Forecasting)
    - Add separate `SETTINGS_NAV` item
    - Update `NavItem` interface to include optional `badge?: number` field
    - Update icons: Dashboard → LayoutDashboard, Replenishment → Truck, Cash Count → Calculator, Reconciliation → Scale, Vendor Invoices → FileText, DSR Reports → BarChart3, Forecasting → TrendingUp, Settings → Settings
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.6_

  - [x] 6.2 Redesign `src/components/layout/Sidebar.tsx`
    - Add `mobileOpen` and `onMobileClose` props
    - Render Brand_Mark: "CR" in 38×38px rounded square (--red-500 bg), "CROWN" text with "ATM & CIT" subtitle
    - Render two NavGroups with uppercase section eyebrow labels
    - Render Settings item below groups, separated by divider/16px whitespace
    - Render system status note at bottom: operational status text + "HH:mm WIB" timestamp
    - Active state: --red-50 background, --red-600 text; inactive: --n-600 text, transparent bg
    - Collapsed state (64px rail): hide labels, Brand_Mark text, status note, group labels — show icons only
    - Support badge rendering next to nav item labels (using `formatBadgeCount`)
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.10_

  - [x] 6.3 Write unit tests for Sidebar rendering
    - Test NavGroup labels render, active state styling, collapsed behavior hides text, badge display
    - _Requirements: 1.1, 1.2, 1.8, 1.10_

- [x] 7. Redesign top bar and AppShell
  - [x] 7.1 Redesign `src/components/layout/TopBar.tsx`
    - Accept `onMenuClick` prop for mobile hamburger
    - Render search input: search icon left, placeholder "Search ATM, vendor, invoice...", "⌘K" shortcut indicator right, min-width 240px
    - Implement Cmd+K / Ctrl+K keyboard shortcut to focus search input
    - Render notification button with bell icon + dot indicator (conditional on unread state)
    - Render profile section: 36px circular avatar with initials (using `getInitials`), full name, role label
    - Mobile (<760px): show hamburger button, hide shortcut indicator and profile name/role text (avatar only)
    - Height: 72px desktop, 64px mobile
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7_

  - [x] 7.2 Refactor `src/app/AppShell.tsx`
    - Add mobile sidebar state (`mobileOpen`) and toggle logic
    - Wrap content in `ToastProvider`
    - Render `ToastPortal` for toast rendering
    - Pass `onMenuClick` to TopBar, `mobileOpen`/`onMobileClose` to Sidebar
    - Render Scrim overlay when mobile sidebar is open (semi-transparent, closes sidebar on click)
    - Apply page-enter animation to `<Outlet />` content area
    - Mobile (<760px): sidebar as fixed overlay at z-index 20, content padding 16px horizontal
    - _Requirements: 8.1, 8.2, 8.3, 9.1_

  - [x] 7.3 Update `src/app/routes.tsx` — add new routes
    - Root `/` redirects to `/dashboard`
    - Add routes: `/dashboard`, `/replenishment`, `/cash-count` (placeholder), `/reconciliation`, `/settings` (placeholder)
    - Keep existing routes: `/invoices`, `/reports` (renamed from /dsr), `/forecast`
    - Wildcard `*` renders NotFound with link to `/dashboard`
    - _Requirements: 1.9, 1.11_

  - [x] 7.4 Write unit tests for TopBar keyboard shortcut and responsive behavior
    - Test Cmd+K focuses search, hamburger renders on mobile, profile text hidden on mobile
    - _Requirements: 2.3, 2.6, 2.7_

- [x] 8. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Implement Dashboard screen
  - [x] 9.1 Create `src/features/dashboard/DashboardScreen.tsx`
    - Render PageHeader with greeting section: full date string (via `formatFullDate`), personalized greeting (via `getGreeting` + user first name), static operational summary sentence
    - Render action buttons: secondary "Export DSR", primary "New schedule" (triggers toast on click)
    - Compose MetricStrip, replenishment summary table section, and AttentionPanel in grid layout
    - Grid layout: `grid-template-columns: minmax(0, 1.55fr) minmax(300px, 0.75fr)` with 28px gap on >1080px
    - Mobile (<760px): stack replenishment table and AttentionPanel vertically
    - _Requirements: 3.1, 3.2, 4.1, 5.5, 8.5, 9.4_

  - [x] 9.2 Create `src/features/dashboard/MetricStrip.tsx`
    - Render 4 KPI cards separated by vertical dividers: Managed Cash, ATM Availability, Today's Routes, Exceptions
    - Each metric: label with icon, large numeric value (tabular-nums), metadata line with contextual info
    - Load data from `dashboard-kpi.json`, format using `formatIDR` for managed cash
    - Responsive: 4-column grid >1080px, 2×2 grid 760–1080px (border-bottom on first two), single column <760px
    - _Requirements: 3.3, 3.4, 3.5, 8.4, 10.3_

  - [x] 9.3 Create `src/features/dashboard/ReplenishmentSummary.tsx`
    - Render "Today's replenishment" section header with "View all" link to `/replenishment`
    - Render DataTable with columns: Route (+ region subtitle), Vendor, Machines, Progress (ProgressBar), Status (badge), Value (IDR right-aligned tabular-nums)
    - Load data from `replenishment-schedules.json`, sort by status priority, display minimum 4 routes with 2+ distinct statuses
    - _Requirements: 4.1, 4.2, 4.3, 4.5, 4.6, 4.7, 12.3, 12.4, 12.5, 12.6_

  - [x] 9.4 Create `src/features/dashboard/AttentionPanel.tsx`
    - Render header "Needs attention" with circular count indicator
    - Render list of attention items: categorized icon (36×36px rounded square with semantic bg), title, description, relative timestamp
    - Semantic icon coloring: danger (--red-soft bg, --red-deep color), warning (--warning-soft bg, --warning color), info (--info-soft bg, --info color)
    - Load from `attention-items.json`, ordered by recency (most recent first)
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.6_

  - [x] 9.5 Write unit tests for DashboardScreen composition
    - Test MetricStrip renders 4 metrics, AttentionPanel renders items, ReplenishmentSummary has "View all" link
    - _Requirements: 3.3, 4.1, 5.1_

- [x] 10. Implement Replenishment Schedules screen
  - [x] 10.1 Create `src/features/replenishment/ReplenishmentScreen.tsx`
    - Render PageHeader with eyebrow "Cash operations", title "Replenishment schedules", description text
    - Render toolbar: date filter (select, defaults to today), region filter ("All regions" default), vendor filter ("All vendors" default), result count label "{count} schedules"
    - Render DataTable with columns: Schedule (ID + route subtitle), Region, Vendor, Window (HH:mm range), Machines, Status (semantic badge), Cash value (IDR right-aligned tabular-nums)
    - Status badges: "In transit" (info), "Completed" (success), "Delayed" (warning), "Pending vendor" (warning)
    - Filter logic using `filterSchedules` utility; update count on filter change
    - Empty state when zero results: search icon, title "No schedules found", hint "Try a route number, vendor, or region."
    - Render action buttons: secondary "Import plan", primary "New schedule" (triggers toast)
    - Wrap table in horizontally scrollable container for mobile
    - Load data from `replenishment-schedules.json`
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9, 6.10, 6.11, 8.7, 9.4, 12.1, 12.3, 12.4, 12.5, 12.6_

  - [x] 10.2 Write unit tests for ReplenishmentScreen filters and empty state
    - Test filter interactions update results, combined filters work, empty state renders
    - _Requirements: 6.5, 6.7, 6.9_

- [x] 11. Implement Reconciliation screen
  - [x] 11.1 Create `src/features/reconciliation/ReconciliationScreen.tsx`
    - Render PageHeader with eyebrow "Financial control", title "Reconciliation", description text
    - Render NoticeBanner: triangle-alert icon, "Cutoff at 14:00 WIB" bold text, description with high-severity unresolved count
    - Render toolbar: exception type filter ("Open exceptions" default, "All records", "Resolved"), severity filter ("All severity" default, "High", "Medium"), result count "{count} exceptions"
    - Render DataTable with columns: Machine (ATM ID + HH:mm timestamp subtitle), Location, Counted (IDR right-aligned tabular-nums), Escrow (IDR right-aligned tabular-nums), Difference (colored + signed via `formatDifference`), Severity (semantic badge with icon), Owner ("Unassigned" if null)
    - Filter logic using `filterExceptions` utility; update count on filter change
    - Empty state when zero results
    - Render action buttons: secondary "Audit trail", primary "Run reconciliation" (triggers toast)
    - Wrap table in horizontally scrollable container for mobile
    - Load data from `reconciliation-exceptions.json`
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8, 7.9, 7.10, 7.11, 7.12, 8.7, 9.5, 12.1, 12.3, 12.4, 12.5, 12.6_

  - [x] 11.2 Write unit tests for ReconciliationScreen filters and difference formatting
    - Test severity filter, exception type filter, difference color rendering, empty state
    - _Requirements: 7.5, 7.7, 7.8, 7.12_

- [x] 12. Mobile responsive behavior and page-enter animation
  - [x] 12.1 Apply responsive breakpoints and mobile adjustments across all screens
    - Sidebar: fixed overlay on <760px with scrim, z-index 20
    - TopBar: hamburger button on <760px, reduced height to 64px
    - Content padding: 16px horizontal on <760px
    - MetricStrip: 2×2 grid at 760–1080px, single column at <760px
    - Dashboard grid: stack vertically on <760px
    - Action buttons: full-width stacked with 8px gap on <760px
    - Tables: `overflow-x: auto` wrapper on all data tables
    - Touch targets: minimum 44×44px on all interactive elements
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 10.7_

  - [x] 12.2 Apply page-enter animation to route transitions
    - Add animation class to `<Outlet />` wrapper in AppShell
    - Fade up from 8px translateY over 420ms with cubic-bezier(0.16, 1, 0.3, 1)
    - Respect `prefers-reduced-motion` (animation disabled)
    - _Requirements: 10.5, 10.6_

  - [x] 12.3 Write unit tests for responsive layout breakpoint logic
    - Test sidebar overlay renders on mobile, scrim closes sidebar, hamburger visible <760px
    - _Requirements: 8.1, 8.2, 2.6_

- [x] 13. Integration wiring and final cleanup
  - [x] 13.1 Wire `ToastProvider` into `App.tsx` and connect toast triggers
    - Ensure "New schedule" buttons (Dashboard + Replenishment) trigger success toast
    - Ensure "Run reconciliation" button triggers success toast
    - Verify toast replacement behavior works app-wide
    - _Requirements: 9.4, 9.5, 9.7_

  - [x] 13.2 Remove deprecated code and update imports
    - Remove old `RoleSwitcher` component from TopBar (replaced by profile section)
    - Update App.tsx to remove TanStack Query provider if unused (static data only)
    - Clean up old `/dsr` and `/cit` route references, replace with `/reports` and `/replenishment`
    - Ensure all feature exports are correctly wired in routes
    - _Requirements: 1.9_

  - [x] 13.3 Write integration tests for navigation flow and toast triggers
    - Test route navigation between all screens, active sidebar states, toast appears on button clicks
    - _Requirements: 1.8, 1.9, 9.4, 9.5_

- [x] 14. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document using fast-check
- Unit tests validate specific examples and edge cases using Vitest + React Testing Library
- The project uses TypeScript throughout with React 19, Vite 6, and Tailwind CSS 4 (OKLCH-native)
- All mock data is static JSON — no API calls, no TanStack Query needed for new screens
- Design tokens follow the OKLCH color space as defined in the steering document

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1", "2.2"] },
    { "id": 2, "tasks": ["2.3", "2.4", "2.5", "2.6", "2.7", "2.8", "4.1", "4.2"] },
    { "id": 3, "tasks": ["4.3", "4.4", "4.5", "4.6", "5.1", "5.2", "5.3", "5.4"] },
    { "id": 4, "tasks": ["5.5", "6.1"] },
    { "id": 5, "tasks": ["6.2", "7.1"] },
    { "id": 6, "tasks": ["6.3", "7.2", "7.3"] },
    { "id": 7, "tasks": ["7.4", "9.1", "9.2", "9.3", "9.4"] },
    { "id": 8, "tasks": ["9.5", "10.1"] },
    { "id": 9, "tasks": ["10.2", "11.1"] },
    { "id": 10, "tasks": ["11.2", "12.1", "12.2"] },
    { "id": 11, "tasks": ["12.3", "13.1", "13.2"] },
    { "id": 12, "tasks": ["13.3"] }
  ]
}
```
