# Requirements Document

## Introduction

Redesign of the CMS frontend prototype to match the polished HTML reference (`cms-atm-internal-prototype.html`). The redesign transforms the current four-screen prototype (DSR Dashboard, Forecast View, CIT Tracker, Invoice Flow) into a dashboard-centric operational interface with restructured navigation, new screens (Dashboard, Replenishment Schedules, Reconciliation), enhanced layout components (metric strip, attention panel, progress bars), and improved mobile responsiveness. The prototype retains its static JSON mock-data approach with no backend.

## Glossary

- **Prototype_App**: The Vite + React + TypeScript single-page application serving as the stakeholder demo
- **Dashboard_Screen**: The new landing page showing operational greeting, KPI metric strip, replenishment summary table, and needs-attention panel
- **Replenishment_Screen**: The schedule-focused screen replacing the former CIT Tracker, showing vendor routes with progress tracking and filters
- **Reconciliation_Screen**: A new screen comparing counted cash against Corebanking escrow with exception tracking
- **Cash_Count_Screen**: The screen for daily cash counting and recording per ATM (placeholder in navigation)
- **DSR_Reports_Screen**: The Daily Status Report screen (moved to "Control" navigation group)
- **Forecasting_Screen**: The forecast and prediction screen (moved to "Control" navigation group)
- **Invoice_Screen**: The vendor invoice validation screen (moved to "Control" navigation group)
- **App_Shell**: The shared layout including sidebar navigation, top bar with search and notifications, and main content area
- **Metric_Strip**: A horizontal component displaying four KPI cards separated by vertical dividers
- **Attention_Panel**: A sidebar panel listing urgent operational items requiring user action, categorized by severity
- **Progress_Bar**: A visual bar within table rows showing completion percentage of a route
- **Toast_Notification**: A temporary feedback message appearing at the bottom-right of the viewport
- **Brand_Mark**: The "CR" identifier in a rounded red square representing CROWN branding
- **Navigation_Group**: A labeled section within the sidebar grouping related navigation items (Operations, Control)
- **Count_Badge**: A pill-shaped counter next to a navigation item indicating pending items
- **Scrim_Overlay**: A semi-transparent backdrop shown on mobile when the sidebar is open
- **ATM**: Automated Teller Machine, identified by unique ID
- **CIT**: Cash in Transit — physical cash pickup or delivery executed by a vendor
- **DSR**: Daily Status Report — cash position per ATM per day
- **IDR**: Indonesian Rupiah, the currency displayed in all monetary values
- **WIB**: Western Indonesian Time zone (UTC+7)
- **KPI**: Key Performance Indicator displayed in the metric strip

## Requirements

### Requirement 1: Restructured Navigation and Sidebar

**User Story:** As an operator, I want navigation organized by operational function, so that I can quickly find screens relevant to my current task.

#### Acceptance Criteria

1. THE App_Shell SHALL render a sidebar with two Navigation_Groups, each displayed with an uppercase label heading ("Operations" and "Control") styled as a section eyebrow above its navigation items
2. THE "Operations" Navigation_Group SHALL contain the following navigation items in order: Dashboard, Replenishment, Cash Count, and Reconciliation
3. THE "Control" Navigation_Group SHALL contain the following navigation items in order: Vendor Invoices, DSR Reports, and Forecasting
4. THE App_Shell SHALL display a Settings navigation item below the Navigation_Groups, visually separated from both groups by a horizontal divider or equivalent whitespace of at least 16px
5. THE App_Shell SHALL display a Brand_Mark consisting of the letters "CR" in a 38×38px rounded square using the --red-500 background color, adjacent to the text "CROWN" with subtitle "ATM & CIT", separated by 12px horizontal gap
6. WHEN a navigation item has pending items, THE App_Shell SHALL display a Count_Badge next to the item label showing the pending count as a number, displaying "99+" when the count exceeds 99
7. THE App_Shell SHALL display a system status note at the bottom of the sidebar showing operational status text and last sync timestamp formatted as "HH:mm WIB" (e.g., "09:52 WIB")
8. WHEN a navigation item is active, THE App_Shell SHALL highlight it with a --red-50 tinted background and --red-600 text color, distinguishing it from inactive items which use --n-600 text on transparent background
9. THE Prototype_App SHALL define routes for: root (redirects to /dashboard), /dashboard, /replenishment, /cash-count, /reconciliation, /invoices, /reports, /forecast, and /settings paths
10. WHEN the sidebar is collapsed, THE App_Shell SHALL hide Navigation_Group labels, item text labels, the Brand_Mark text, and the status note text, showing only icons in a 64px-wide rail
11. IF a user navigates to an undefined route, THEN THE Prototype_App SHALL render a NotFound view with a link to navigate back to /dashboard

### Requirement 2: Enhanced Top Bar

**User Story:** As an operator, I want quick access to search, notifications, and my profile from any screen, so that I can stay efficient during operations.

#### Acceptance Criteria

1. THE App_Shell SHALL render a top bar with height 72px containing a search input, notification button, and profile section that remains visible on all screens
2. THE top bar search input SHALL display a search icon on the left, placeholder text "Search ATM, vendor, invoice...", and a keyboard shortcut indicator "⌘K" on the right, with a minimum width of 240px
3. WHEN the user presses ⌘K (macOS) or Ctrl+K (Windows/Linux), THE App_Shell SHALL move focus to the top bar search input
4. IF unread notifications exist, THEN THE notification button SHALL display a bell icon with a visible dot indicator; IF no unread notifications exist, THEN THE notification button SHALL display the bell icon without a dot indicator
5. THE profile section SHALL display a 36px circular avatar containing the first letter of the user's first name and first letter of the user's last name (maximum 2 characters), followed by the user's full name and their role label
6. WHILE the viewport width is less than 760px, THE App_Shell SHALL display a hamburger menu button in the top bar to open the sidebar
7. WHILE the viewport width is less than 760px, THE App_Shell SHALL hide the keyboard shortcut indicator and the profile name/role text, showing only the avatar

### Requirement 3: Dashboard Screen — Greeting and Metric Strip

**User Story:** As an operator starting my shift, I want an at-a-glance operational summary, so that I can immediately understand the state of cash operations.

#### Acceptance Criteria

1. THE Dashboard_Screen SHALL display a greeting section containing: the current date formatted as a full weekday and date string in locale format (e.g., "Tuesday, 21 July 2026"), a time-of-day personalized greeting using the user's first name where "Good morning" displays from 00:00–11:59, "Good afternoon" from 12:00–17:59, and "Good evening" from 18:00–23:59 (e.g., "Good morning, Raden"), and a static operational summary sentence describing the day's workload
2. THE Dashboard_Screen SHALL display action buttons in the page header: a secondary "Export DSR" button and a primary "New schedule" button
3. THE Dashboard_Screen SHALL display a Metric_Strip containing exactly four KPI metrics separated by vertical dividers: Managed cash (IDR value abbreviated with "T" suffix for values ≥ 1 triliun or "B" suffix for values in miliar, displayed to one decimal place), ATM availability (percentage displayed to one decimal place), Today's routes (integer count), and Exceptions (integer count)
4. EACH metric in the Metric_Strip SHALL display a label with an icon, a large numeric value using tabular-nums formatting, and a metadata line showing: for Managed cash a comparison versus the previous day (e.g., "↑ 2.4% from yesterday"), for ATM availability the count of online/total ATMs (e.g., "4,812 of 4,875 online"), for Today's routes the completion breakdown (e.g., "142 completed, 42 active"), and for Exceptions the priority indicator (e.g., "3 high priority before 14:00")
5. THE Metric_Strip SHALL load KPI values from the Mock_Data_Layer

### Requirement 4: Dashboard Screen — Replenishment Summary Table

**User Story:** As an operator, I want to see today's active replenishment routes at a glance, so that I can monitor CIT progress without navigating away from the dashboard.

#### Acceptance Criteria

1. THE Dashboard_Screen SHALL display a "Today's replenishment" section with a data table and a "View all" link navigating to the Replenishment_Screen at the /replenishment route
2. THE replenishment summary table SHALL display columns: Route (with region subtitle), Vendor, Machines (integer count), Progress (visual progress bar with fraction label), Status (semantic badge), and Value (IDR amount right-aligned with tabular-nums formatting and dot-separated thousands)
3. THE Progress_Bar SHALL render as a horizontal bar track with a colored fill whose width represents the completion percentage calculated as (completed machines ÷ total machines × 100), accompanied by a text label showing "X of Y" where X is the number of completed machines and Y is the total machine count for that route
4. THE Progress_Bar fill color SHALL vary by status: brand red (--red-500) for in-transit routes, success green (semantic success solid) for completed routes, and warning amber (semantic warning solid) for delayed routes
5. THE replenishment summary table SHALL display status using semantic badges with dot indicator and label: "In transit" (info variant), "Completed" (success variant), "Delayed" (warning variant)
6. THE replenishment summary table SHALL load data from the Mock_Data_Layer with a minimum of 4 route records covering at least 2 distinct statuses
7. THE replenishment summary table SHALL display rows ordered by status priority: delayed routes first, then in-transit routes, then completed routes

### Requirement 5: Dashboard Screen — Attention Panel

**User Story:** As an operator, I want to see items requiring immediate attention in a dedicated panel, so that I can prioritize actions before cutoff times.

#### Acceptance Criteria

1. THE Dashboard_Screen SHALL display an Attention_Panel as a right-side panel alongside the replenishment table, containing a header with title "Needs attention" and a circular count indicator showing total attention items
2. THE Attention_Panel SHALL display a list of attention items, each containing: a categorized icon (in a 36×36px rounded square with semantic background color), a title, a description, and a relative timestamp
3. THE Attention_Panel icons SHALL use semantic coloring: danger (--red-soft background with --red-deep color) for critical items like escrow mismatches, warning (--warning-soft background with --warning color) for delays, and info (--info-soft background with --info color) for informational items like pending approvals
4. THE Attention_Panel SHALL load attention items from the Mock_Data_Layer with a minimum of 4 items spanning at least 2 severity categories
5. THE Dashboard_Screen layout SHALL arrange the replenishment table and Attention_Panel in a two-column grid (grid-template-columns: minmax(0, 1.55fr) minmax(300px, 0.75fr)), with a 28px gap between columns on viewports wider than 1080px
6. THE Attention_Panel SHALL display items ordered by recency, with the most recent item at the top

### Requirement 6: Replenishment Schedules Screen

**User Story:** As an operator, I want to view and filter all replenishment schedules for the day, so that I can track vendor fulfillment and identify delays.

#### Acceptance Criteria

1. THE Replenishment_Screen SHALL display a page header with eyebrow text "Cash operations", title "Replenishment schedules", and description text "Plan and monitor vendor routes, machine loading, and completion evidence."
2. THE Replenishment_Screen SHALL display a toolbar with a date filter (select, defaulting to today's date), a region filter (select with "All regions" default), a vendor filter (select with "All vendors" default), and a result count label showing the number of visible schedules in the format "{count} schedules"
3. THE Replenishment_Screen SHALL display a data table with columns: Schedule (ID with route subtitle), Region, Vendor, Window (time range in HH:mm format), Machines (integer count), Status (semantic badge), and Cash value (IDR amount right-aligned with tabular-nums)
4. THE Replenishment_Screen SHALL display status using semantic badges: "In transit" (info), "Completed" (success), "Delayed" (warning), "Pending vendor" (warning)
5. WHEN a region filter is selected, THE Replenishment_Screen SHALL show only schedules matching the selected region and update the result count
6. WHEN a vendor filter is selected, THE Replenishment_Screen SHALL show only schedules matching the selected vendor and update the result count
7. WHEN both region and vendor filters are active simultaneously, THE Replenishment_Screen SHALL show only schedules matching both criteria and update the result count accordingly
8. WHEN any filter is cleared (reset to default "All" option), THE Replenishment_Screen SHALL restore unfiltered results for that dimension and update the result count
9. IF the active filter combination yields zero matching schedules, THEN THE Replenishment_Screen SHALL display an empty-state message with search icon, title "No schedules found", and hint text "Try a route number, vendor, or region."
10. THE Replenishment_Screen SHALL display action buttons: a secondary "Import plan" button and a primary "New schedule" button
11. THE Replenishment_Screen SHALL load schedule data from the Mock_Data_Layer with a minimum of 5 schedule records distributed across at least 3 regions and 3 vendors

### Requirement 7: Reconciliation Screen

**User Story:** As an operator, I want to compare counted cash against Corebanking escrow records, so that I can identify and resolve discrepancies before the daily cutoff.

#### Acceptance Criteria

1. THE Reconciliation_Screen SHALL display a page header with eyebrow text "Financial control", title "Reconciliation", and description "Compare counted cash with Corebanking escrow and resolve exceptions."
2. THE Reconciliation_Screen SHALL display a warning notice banner containing a triangle-alert icon, bold text indicating the cutoff time (e.g., "Cutoff at 14:00 WIB"), and a description stating the count of high-severity unresolved exceptions requiring operator review
3. THE Reconciliation_Screen SHALL display a toolbar with an exception type filter (options: "Open exceptions", "All records", "Resolved") defaulting to "Open exceptions", a severity filter (options: "All severity", "High", "Medium") defaulting to "All severity", and a result count label displaying the format "{count} exceptions"
4. THE Reconciliation_Screen SHALL display a data table with columns: Machine (ATM ID with last-count timestamp in HH:mm format as subtitle), Location, Counted (IDR amount right-aligned with tabular-nums), Escrow (IDR amount right-aligned with tabular-nums), Difference (IDR amount with color indicating direction), Severity (semantic badge), and Owner (operator name or "Unassigned" if none assigned)
5. IF the Difference value is negative, THEN THE Difference column SHALL display the value in danger color (--red-deep) with a minus prefix; IF the Difference value is positive, THEN THE Difference column SHALL display the value in success color with a plus prefix
6. THE Reconciliation_Screen SHALL display severity using semantic badges: "High" with danger variant and "Medium" with warning variant, each including an icon paired with the label text
7. WHEN a severity filter is selected, THE Reconciliation_Screen SHALL show only records matching the selected severity level and update the result count label to reflect the filtered record count
8. WHEN the exception type filter is changed, THE Reconciliation_Screen SHALL show only records matching the selected exception type and update the result count label to reflect the filtered record count
9. THE Reconciliation_Screen SHALL display action buttons: a secondary "Audit trail" button and a primary "Run reconciliation" button
10. WHEN the operator clicks the "Run reconciliation" button, THE Prototype_App SHALL display a Toast_Notification with a success icon and confirmation message
11. THE Reconciliation_Screen SHALL load reconciliation data from the Mock_Data_Layer with a minimum of 4 exception records covering both High and Medium severity levels
12. IF the active filters yield zero matching records, THEN THE Reconciliation_Screen SHALL display an empty-state message indicating that no exceptions match the current filter criteria

### Requirement 8: Mobile Responsive Layout

**User Story:** As a stakeholder viewing the demo on a mobile device, I want the interface to adapt gracefully, so that I can evaluate the system from any screen size.

#### Acceptance Criteria

1. WHILE the viewport width is less than 760px, THE App_Shell SHALL hide the sidebar from the layout flow and display it as a fixed overlay (positioned above page content at z-index 20) WHEN the user taps the hamburger menu button located in the top bar
2. WHILE the sidebar overlay is open on mobile, THE App_Shell SHALL display a semi-transparent Scrim_Overlay behind the sidebar that closes the sidebar overlay when tapped anywhere on the scrim
3. WHILE the viewport width is less than 760px, THE App_Shell SHALL reduce the top bar height to 64px and reduce the main content area padding to 16px horizontal
4. WHILE the viewport width is between 760px and 1080px, THE Metric_Strip SHALL reflow from a 4-column grid to a 2×2 grid layout with border-bottom on the first two metrics
5. WHILE the viewport width is less than 760px, THE Dashboard_Screen layout SHALL stack the replenishment table and Attention_Panel vertically in a single column instead of side by side
6. WHILE the viewport width is less than 760px, THE Prototype_App SHALL render page header action buttons as full-width (100% container width) buttons stacked vertically with 8px vertical gap between them
7. THE Prototype_App SHALL wrap each data table in a horizontally scrollable container (overflow-x: auto) so that no table content is clipped on viewports narrower than the table's natural width

### Requirement 9: Toast Notification System

**User Story:** As an operator performing actions, I want brief feedback confirmations, so that I know my action was registered without disrupting my workflow.

#### Acceptance Criteria

1. THE Prototype_App SHALL provide a Toast_Notification component that appears at the bottom-right of the viewport (bottom-left and full-width on mobile viewports less than 760px), displaying an icon and message text, and auto-dismissing after 4 seconds
2. THE Toast_Notification SHALL animate in over 220ms by transitioning from opacity 0 and translateY(12px) to opacity 1 and translateY(0), and animate out in reverse
3. THE Toast_Notification SHALL use a dark background (--n-900 / ink color) with light text (--n-0) and the design system shadow for contrast against page content
4. WHEN the "New schedule" primary button is clicked, THE Prototype_App SHALL display a Toast_Notification with a success icon and a message indicating the schedule was created
5. WHEN the "Run reconciliation" primary button is clicked, THE Prototype_App SHALL display a Toast_Notification with a success icon and a message indicating reconciliation was initiated
6. THE Toast_Notification SHALL use an ARIA role of "status" with aria-live set to "polite" so that screen readers announce the message without interrupting the current task
7. IF a new toast is triggered while an existing Toast_Notification is visible, THEN THE Prototype_App SHALL replace the current toast with the new one, restarting the 4-second auto-dismiss timer

### Requirement 10: Visual Design Tokens and Styling

**User Story:** As a developer implementing the redesign, I want consistent design tokens matching the HTML reference, so that the prototype looks polished and production-ready.

#### Acceptance Criteria

1. THE Prototype_App SHALL define CSS custom properties for the color palette using OKLCH color space, including brand red scale (hue 29, 10 shades from 50 through 900), neutral scale (tinted toward brand hue with chroma 0.003–0.009, 10 shades from 0 through 900), and semantic colors (success hue 155, warning hue 78, danger hue 12, info hue 245, each with background-tint, foreground, and solid variants) as defined in the design system steering document
2. THE Prototype_App SHALL use the system font stack: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif
3. THE Prototype_App SHALL apply tabular-nums font-variant-numeric to all numeric cell values in data tables and all metric display values in summary cards
4. THE Prototype_App SHALL use border-radius of 10px for cards and buttons, 12px for table shells, and 999px for badges and pills
5. THE Prototype_App SHALL apply a page-enter animation (fade up from 8px translateY offset over 420ms with custom easing cubic-bezier(0.16, 1, 0.3, 1)) to screen content when navigating between routes
6. WHILE the user has prefers-reduced-motion enabled, THE Prototype_App SHALL disable all CSS animations and transitions by setting animation-duration and transition-duration to 0
7. THE Prototype_App SHALL maintain minimum 44×44px touch targets (width and height) for all interactive elements including buttons, links, icon buttons, and form controls
8. THE Prototype_App SHALL define a type scale with the following size and weight pairings: Display (44px / 700), XL (28px / 700), LG (21px / 600), Base (16px / 400), SM (14px / 400), and XS (12px / 500 uppercase with 0.1em letter-spacing)

### Requirement 11: Mock Data Layer Updates

**User Story:** As a developer, I want mock data that matches the new dashboard structure, so that all screens display realistic operational information.

#### Acceptance Criteria

1. THE Mock_Data_Layer SHALL provide a dashboard KPI data file containing: managed cash total (IDR, numeric value), ATM availability percentage (0–100) with online count and total count, today's route count with completed count and active count summing to the total, and exception count with priority breakdown summing to the total exception count
2. THE Mock_Data_Layer SHALL provide a replenishment schedules data file containing a minimum of 8 schedule records with fields: schedule ID, route code, region, vendor name, time window (start time–end time in HH:mm format), machine count, completion count (less than or equal to machine count), status (one of: "completed", "in-transit", "scheduled", "delayed", "pending-vendor"), and cash value (IDR numeric)
3. THE Mock_Data_Layer SHALL provide a reconciliation exceptions data file containing a minimum of 5 records with fields: ATM ID (referencing a valid ATM record), last count timestamp (ISO 8601 format), location, counted amount (IDR numeric), escrow amount (IDR numeric), difference (equal to counted amount minus escrow amount), severity (one of: "high", "medium"), and owner name
4. THE Mock_Data_Layer SHALL provide an attention items data file containing a minimum of 5 items with fields: category (one of: "danger", "warning", "info"), icon name (matching a valid Lucide icon identifier), title (maximum 80 characters), description (maximum 200 characters), and relative timestamp string
5. THE Mock_Data_Layer SHALL maintain referential consistency such that ATM IDs in reconciliation exceptions reference valid ATM records from atms.json, vendor names in replenishment schedules reference valid vendor name values from vendors.json, and the online count in dashboard KPIs does not exceed the total ATM count
6. THE Mock_Data_Layer SHALL use IDR monetary values formatted with abbreviation suffixes (T for trillion, B for billion, M for million) exclusively in the dashboard KPI display fields, while all other monetary fields in schedules, reconciliation, and attention items SHALL store raw numeric integer values

### Requirement 12: Page-Level Structure and Content Patterns

**User Story:** As a stakeholder reviewing the design, I want consistent page structures across all screens, so that the interface feels cohesive and learnable.

#### Acceptance Criteria

1. EACH screen page SHALL display a page header containing: an eyebrow label (uppercase, font-size 0.75rem, font-weight 800, letter-spacing 0.09em, colored --red-deep), a title (font-size clamp(1.85rem, 3vw, 2.55rem), font-weight 700, letter-spacing -0.045em), and an optional description paragraph (color --muted, max-width 62ch)
2. EACH screen page SHALL limit action buttons in the header to a maximum of one primary button and one or two secondary buttons
3. THE Prototype_App SHALL display table column headers as uppercase labels (font-size 0.7rem, letter-spacing 0.07em) on --surface-2 background with --muted text color
4. THE Prototype_App SHALL display table rows with a hover effect using a subtle warm tint (approximately oklch(98% 0.008 32))
5. WHEN a table cell contains a primary identifier and a secondary descriptor, THE cell SHALL render the identifier in bold (font-weight 700, class cell-main) and the descriptor below it in muted smaller text (font-size 0.75rem, color --muted, class cell-sub)
6. THE Prototype_App SHALL display IDR amounts right-aligned with tabular-nums font-variant-numeric and font-weight of at least 650
