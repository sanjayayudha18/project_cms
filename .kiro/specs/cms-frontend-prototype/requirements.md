# Requirements Document

## Introduction

Frontend prototype for the CIMB Niaga Cash Management System (CMS). This is a stakeholder demo application — a unified single-page app showing both internal (CodexCash) and vendor portal flows. The prototype uses hardcoded JSON mock data powering real React components in a near-production frontend shell. No backend, no database, no real authentication. The goal is to demonstrate key operational screens so stakeholders can validate the UX and approve full development.

## Glossary

- **Prototype_App**: The unified Vite + React + TypeScript single-page application serving as the stakeholder demo
- **DSR_Dashboard**: The Daily Status Report screen showing cash positions per ATM/CRM per day
- **Forecast_View**: The screen displaying H+2 (2-day-ahead) cash need predictions and replenishment schedules
- **CIT_Tracker**: The Cash-in-Transit order tracking screen showing pickup/delivery status by vendor
- **Invoice_Flow**: The invoice validation workflow screen (upload, validate, approve)
- **Mock_Data_Layer**: Static JSON files containing realistic banking data (IDR amounts, ATM IDs, vendor names) that power all screens
- **Role_Switcher**: A UI control allowing demo users to switch between roles (Admin, Operator, Manager, Vendor) without authentication
- **App_Shell**: The shared layout including sidebar navigation, top bar, and role indicator
- **ATM**: Automated Teller Machine, identified by unique ATM ID
- **CIT**: Cash in Transit — physical cash pickup or delivery executed by a vendor
- **DSR**: Daily Status Report — cash position per ATM/CRM per day
- **Replenishment**: Instruction and execution to refill ATM cash
- **Forecast_H2**: 2-day-ahead cash need prediction per ATM/vendor
- **Maker_Checker**: Two-person approval control where maker creates and checker approves
- **IDR**: Indonesian Rupiah, the currency displayed in all monetary values

## Requirements

### Requirement 1: Application Shell and Navigation

**User Story:** As a stakeholder reviewing the demo, I want a consistent layout with sidebar navigation and role context, so that I can understand the system's information architecture.

#### Acceptance Criteria

1. THE Prototype_App SHALL render an App_Shell with a collapsible sidebar (expanded width between 240px and 280px, collapsed width between 56px and 72px), a top bar, and a main content area
2. WHEN the sidebar toggle button is clicked, THE App_Shell SHALL transition the sidebar between expanded state (showing icon and label for each navigation item) and collapsed state (showing icon only) within 300ms
3. THE App_Shell SHALL display the current active role in the top bar and provide a role switcher control that allows selection among Admin, Operator, Manager, and Vendor, defaulting to Admin on initial load
4. THE App_Shell SHALL provide sidebar navigation links to DSR Dashboard, Forecast View, CIT Tracker, and Invoice Flow screens
5. WHEN a navigation link is clicked, THE Prototype_App SHALL route to the corresponding screen without full page reload
6. THE App_Shell SHALL visually differentiate the currently active navigation item from inactive items in the sidebar by applying a distinct background fill or font weight to the active item

### Requirement 2: Role Switching for Demo

**User Story:** As a presenter demoing the system, I want to switch between user roles instantly, so that I can show how different roles experience the same screens.

#### Acceptance Criteria

1. THE Role_Switcher SHALL allow selection between Admin, Operator, Manager, and Vendor roles from a control placed in the top bar
2. WHEN the Prototype_App loads, THE Prototype_App SHALL default to the Admin role as the active selection
3. WHEN a role is selected, THE Prototype_App SHALL update the displayed role label in the top bar within 200ms
4. WHEN the Vendor role is selected, THE Prototype_App SHALL display only the CIT Order Tracking and Invoice Validation navigation items, hiding DSR Dashboard and Forecast View
5. WHEN an internal role (Admin, Operator, Manager) is selected, THE Prototype_App SHALL display all four navigation items: DSR Dashboard, Forecast View, CIT Order Tracking, and Invoice Validation

### Requirement 3: DSR Dashboard

**User Story:** As an Operator, I want to view daily cash positions per ATM, so that I can monitor which machines need attention.

#### Acceptance Criteria

1. THE DSR_Dashboard SHALL display a data table with columns: ATM ID, Location, Vendor, Beginning Balance (IDR), Cash In (IDR), Cash Out (IDR), Ending Balance (IDR), and Status
2. THE DSR_Dashboard SHALL load data from the Mock_Data_Layer containing a minimum of 20 ATM records with IDR amounts ranging from 0 to 999,999,999,999 (in whole rupiah, no decimals)
3. THE DSR_Dashboard SHALL display monetary values right-aligned with tabular number formatting and explicit IDR currency label
4. THE DSR_Dashboard SHALL display status using semantic badges with both icon and text label, where status is derived from the Ending Balance: "Critical" when Ending Balance is below 50,000,000 IDR, "Low" when Ending Balance is between 50,000,000 and 150,000,000 IDR (inclusive), and "Normal" when Ending Balance is above 150,000,000 IDR
5. WHEN a column header is clicked, THE DSR_Dashboard SHALL sort the table by that column, toggling between ascending order on first click and descending order on subsequent click of the same column
6. THE DSR_Dashboard SHALL display a summary card showing total Beginning Balance, total Cash In, total Cash Out, and total Ending Balance across all ATM records for the selected date
7. THE DSR_Dashboard SHALL provide a date selector defaulting to the current mock date to simulate daily report viewing
8. WHEN a date is selected from the date selector, THE DSR_Dashboard SHALL reload the table and summary card with data corresponding to the selected date

### Requirement 4: Forecast and Replenishment Schedule View

**User Story:** As an Operator, I want to see the H+2 forecast and upcoming replenishment schedule, so that I can plan vendor cash needs.

#### Acceptance Criteria

1. THE Forecast_View SHALL display a table with columns: ATM ID, Location, Vendor, Current Balance (IDR), Predicted Usage H+1 (IDR), Predicted Usage H+2 (IDR), Recommended Replenishment (IDR), and Priority — with all IDR amounts formatted using dot-separated thousands (e.g., "1.250.000"), right-aligned, and rendered in tabular-nums
2. THE Forecast_View SHALL load forecast data from the Mock_Data_Layer with a minimum of 15 ATM forecast records
3. THE Forecast_View SHALL display priority levels using semantic badges: High (danger), Medium (warning), Low (neutral)
4. THE Forecast_View SHALL display the forecast table sorted by Priority descending (High first, then Medium, then Low) by default
5. THE Forecast_View SHALL display a replenishment schedule section listing upcoming orders for the next 3 calendar days (H+1, H+2, H+3), grouped by vendor then by date, with a minimum of 5 mock schedule entries
6. WHEN a priority filter is applied, THE Forecast_View SHALL show only ATMs matching the selected priority level, and update the summary card total to reflect only the filtered ATMs
7. WHEN the priority filter is cleared, THE Forecast_View SHALL restore the full unfiltered forecast table and reset the summary card total to the amount across all ATMs
8. THE Forecast_View SHALL display a summary card showing total recommended replenishment amount across all ATMs, formatted as IDR with dot-separated thousands
9. IF the applied priority filter matches no ATM records, THEN THE Forecast_View SHALL display an empty-state message indicating no ATMs match the selected priority

### Requirement 5: CIT Order Tracking

**User Story:** As a Manager, I want to track Cash-in-Transit orders and their execution status, so that I can ensure vendors fulfill their obligations.

#### Acceptance Criteria

1. THE CIT_Tracker SHALL display a table with columns: Order ID, ATM ID, Vendor, Order Date, Scheduled Date, Amount (IDR), Status, and Evidence — where Evidence displays a clickable link label if evidence exists, or a dash character if no evidence is attached
2. THE CIT_Tracker SHALL load CIT order data from the Mock_Data_Layer with a minimum of 15 order records distributed across at least 3 distinct vendors, covering all 4 status values (Scheduled, In Transit, Completed, Failed)
3. THE CIT_Tracker SHALL display order status using semantic badges with icon and label: Scheduled (info), In Transit (warning), Completed (success), Failed (danger)
4. WHEN a status filter is applied, THE CIT_Tracker SHALL show only orders matching the selected status, and WHEN both status and vendor filters are active simultaneously, THE CIT_Tracker SHALL show only orders matching both criteria
5. WHEN a vendor filter is applied, THE CIT_Tracker SHALL show only orders belonging to the selected vendor
6. THE CIT_Tracker SHALL display a summary showing order counts per status category, updated to reflect currently visible (filtered) results
7. IF the active filter combination yields zero matching orders, THEN THE CIT_Tracker SHALL display an empty-state message indicating no orders match the current filters, and the summary SHALL show zero for all status categories
8. THE CIT_Tracker SHALL display the table sorted by Scheduled Date in descending order (most recent first) by default, and SHALL format Amount values as right-aligned IDR currency with thousands separators

### Requirement 6: Invoice Validation Flow

**User Story:** As a Manager, I want to validate and approve vendor invoices against executed CIT/replenishment orders, so that I can confirm correctness before handoff.

#### Acceptance Criteria

1. THE Invoice_Flow SHALL display a three-step workflow indicator showing: Upload, Validate, and Approve stages, with the current stage highlighted based on the selected invoice's validation status
2. THE Invoice_Flow SHALL display an invoice list table with columns: Invoice Number, Vendor, Period, Total Amount (IDR, right-aligned, tabular-nums), Line Items Count, and Validation Status displayed as a semantic badge
3. THE Invoice_Flow SHALL load invoice data from the Mock_Data_Layer with a minimum of 8 invoice records distributed across the following validation states: Uploaded, Validated, Approved, and Mismatch Detected
4. WHEN an invoice row is clicked, THE Invoice_Flow SHALL display invoice detail showing line items in a table with columns: Description, Invoiced Amount (IDR), Matched Order Reference, Expected Amount (IDR), Variance (IDR), and Match Status badge
5. THE Invoice_Flow SHALL display line item match status using semantic badges: Matched (success), Mismatch (danger), Pending Review (warning)
6. WHILE the Manager role is active, WHEN an invoice in Validated state is selected, THE Invoice_Flow SHALL display an Approve button
7. WHEN the Approve button is clicked, THE Invoice_Flow SHALL update the invoice status to Approved in the UI and display a confirmation message indicating the invoice number, approver name, and timestamp of approval
8. THE Invoice_Flow SHALL implement maker-checker display by showing the validator name and approver name as labeled, separate fields within the invoice detail view, enforcing that the validator and approver are different actors
9. IF a non-Manager role is active, THEN THE Invoice_Flow SHALL hide the Approve button and display the invoice detail in read-only mode

### Requirement 7: Mock Data Layer

**User Story:** As a developer building the prototype, I want structured mock data files with realistic banking values, so that the demo looks credible to stakeholders.

#### Acceptance Criteria

1. THE Mock_Data_Layer SHALL provide static JSON files for DSR records, forecast data, CIT orders, and invoice data, with a minimum of 15 records per entity type
2. THE Mock_Data_Layer SHALL use realistic ATM IDs following the pattern used by CIMB Niaga (e.g., ATM-JKT-001), providing at minimum 15 unique ATM IDs spanning at least 3 region prefixes
3. THE Mock_Data_Layer SHALL use realistic vendor names representing cash logistics companies, with at minimum 3 distinct vendors
4. THE Mock_Data_Layer SHALL use IDR monetary values in integer format (no decimals) representing realistic ATM cash amounts ranging from IDR 50,000,000 to IDR 500,000,000
5. THE Mock_Data_Layer SHALL provide data with consistent referential relationships such that every CIT order references a valid ATM ID present in DSR data, every invoice references valid CIT order IDs, and every forecast record references a valid ATM ID
6. THE Mock_Data_Layer SHALL include records covering at minimum the following states per entity: DSR records (uploaded, validated, reconciled), CIT orders (requested, assigned, in-transit, completed, cancelled), invoices (uploaded, validated, disputed, approved), and forecast runs (pending, completed)
7. THE Mock_Data_Layer SHALL provide date fields spanning a contiguous 7-day period so that daily DSR positions, H+2 forecast horizons, and CIT scheduling sequences are demonstrable across multiple days

### Requirement 8: Docker Containerization

**User Story:** As a developer, I want a simple Docker setup to build and serve the prototype, so that stakeholders can run the demo without a local development environment.

#### Acceptance Criteria

1. THE Prototype_App SHALL include a multi-stage Dockerfile where the first stage runs `pnpm install` and `pnpm build` to produce Vite production output, and the second stage copies the build artifacts into an `nginx:alpine` image for serving
2. THE Prototype_App SHALL include a docker-compose.yml with a single frontend service that maps container port 80 to host port 3000 by default, configurable via an environment variable
3. WHEN `docker compose up` is executed, THE Prototype_App SHALL return an HTTP 200 response on the root path (http://localhost:3000/) within 30 seconds of the command completing
4. THE Prototype_App SHALL include a .dockerignore file excluding node_modules, dist, .env, and .git directories
5. THE Prototype_App SHALL include an Nginx configuration that uses `try_files $uri $uri/ /index.html` so that client-side routes (DSR dashboard, forecast, CIT tracking, invoice validation) return the SPA index instead of a 404 on direct access or page refresh

### Requirement 9: Routing and Client-Side Navigation

**User Story:** As a user navigating the demo, I want direct URLs for each screen, so that I can bookmark or share specific views.

#### Acceptance Criteria

1. THE Prototype_App SHALL use React Router for client-side routing with browser history mode
2. THE Prototype_App SHALL define routes for: root (redirects to /dsr), /dsr, /forecast, /cit, and /invoice paths, where each path renders its corresponding module screen
3. WHEN an unknown route is accessed, THE Prototype_App SHALL display a 404 page containing a link that navigates the user to /dsr
4. THE Prototype_App SHALL provide a persistent navigation element visible on all routed screens that contains links to each defined route (/dsr, /forecast, /cit, /invoice)
5. THE Prototype_App SHALL configure Nginx to return index.html with HTTP 200 for all route paths to support client-side routing in the Docker container
6. WHEN a user navigates to a defined route via direct URL entry in the browser address bar, THE Prototype_App SHALL render the corresponding module screen without requiring navigation from the root path first

### Requirement 10: Responsive Layout and Accessibility

**User Story:** As a stakeholder viewing the demo on different devices, I want the interface to be usable on desktop and tablet, so that I can evaluate it in a meeting room or at my desk.

#### Acceptance Criteria

1. WHILE the viewport width is less than 1024px, THE App_Shell SHALL render the sidebar as an icon-only rail (no text labels visible) by default
2. WHILE the viewport width is less than 1024px, THE Prototype_App SHALL wrap each data table in a horizontally scrollable container so that no table content is clipped, overlapped, or causes the page body to exceed viewport width
3. THE Prototype_App SHALL use semantic HTML elements (nav, main, header, table) for screen reader compatibility
4. THE Prototype_App SHALL never signal status with color alone — all status indicators SHALL pair color with an icon and a text label
5. THE Prototype_App SHALL maintain a minimum touch target size of 44×44px for all interactive elements (buttons, links, form controls, and row actions)
6. WHILE the viewport width is between 768px and 1023px, THE Prototype_App SHALL display all page content without horizontal overflow on the body element and without any interactive element being unreachable by scroll or tap
