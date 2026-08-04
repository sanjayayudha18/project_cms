# Requirements Document

## Introduction

Vendor Portal for the CIMB Niaga Cash Management System (CMS). This is a completely separate frontend SPA from the internal CodexCash app, running in its own Docker container with separate Dockerfile, Nginx configuration, and port. The portal allows CIT vendor personnel (PT Gardanet, PT SSI, PT G4S) to log in with local credentials, view their assigned CIT orders, upload handover evidence, view invoices, check replenishment schedules, and monitor DSR data for their assigned ATMs. Vendors are scoped to their own assignments only and cannot access other vendors' data or internal-only features. The portal uses the "Merah Menyala" brand theme (bold, brand-forward with maroon-red top bar and full-red active sidebar). Initial implementation is a prototype with static JSON mock data that can be swapped for real API calls later.

## Glossary

- **Vendor_Portal**: The separate React + TypeScript + Vite SPA serving as the vendor-facing application, located at `frontend/VendorPortal-Vite/`
- **Vendor_User**: A CIT vendor personnel (from PT Gardanet, PT SSI, or PT G4S) who authenticates with local credentials stored in the CMS database
- **Auth_Module**: The authentication component that validates local credentials (bcrypt/argon2), issues JWT tokens with `auth_source=local` and `role=Vendor` claims, and manages session state
- **CIT_Order**: A Cash-in-Transit order representing a scheduled physical cash pickup or delivery assigned to a specific vendor
- **Handover_Evidence**: Photographic or document proof uploaded by a vendor upon completing a CIT delivery
- **Invoice_View**: The screen where vendors view their submitted invoices and current validation status
- **Replenishment_Schedule**: A planned cash replenishment instruction assigned to a vendor for execution
- **DSR_View**: The Daily Status Report screen showing cash positions for ATMs assigned to the logged-in vendor
- **Notification_Center**: The in-app notification component that surfaces new assignments, order status changes, and other vendor-relevant events
- **App_Shell**: The shared layout including maroon-red top bar, full-red active sidebar, and main content area styled with the "Merah Menyala" theme
- **Mock_Data_Layer**: Static JSON files containing realistic vendor-scoped data that power all screens during the prototype phase
- **JWT**: JSON Web Token issued upon successful authentication, containing `auth_source=local`, `role=Vendor`, and `vendor_id` claims
- **Vendor_Scope**: The data isolation principle ensuring each vendor sees only ATMs, orders, invoices, and schedules assigned to them
- **ATM**: Automated Teller Machine identified by a unique ATM ID
- **CIT**: Cash in Transit, the physical cash pickup or delivery executed by a vendor
- **DSR**: Daily Status Report showing cash position per ATM per day
- **IDR**: Indonesian Rupiah, the currency displayed in all monetary values

## Requirements

### Requirement 1: Vendor Authentication

**User Story:** As a Vendor_User, I want to log in with my local credentials, so that I can securely access my assigned work within the portal.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL display a login screen with username and password input fields, a submit button, and the CIMB Niaga logo styled with the "Merah Menyala" maroon-red color scheme
2. WHEN a Vendor_User submits valid credentials, THE Auth_Module SHALL issue a JWT containing `auth_source=local`, `role=Vendor`, and the corresponding `vendor_id` claim, and redirect the user to the CIT Orders dashboard
3. IF a Vendor_User submits invalid credentials, THEN THE Auth_Module SHALL display an error message stating "Username atau password salah" without revealing which field is incorrect
4. WHEN a Vendor_User is authenticated, THE Vendor_Portal SHALL store the JWT in memory (not localStorage) and include it as a Bearer token in the Authorization header of all subsequent API requests
5. WHEN the JWT expires or is invalid, THE Vendor_Portal SHALL redirect the Vendor_User to the login screen and clear all cached data
6. THE Vendor_Portal SHALL enforce a minimum password length of 8 characters and prevent form submission when either field is empty
7. WHILE the login request is in progress, THE Vendor_Portal SHALL disable the submit button and display a loading indicator to prevent duplicate submissions
8. IF a network error occurs during login, THEN THE Vendor_Portal SHALL display an error message indicating a connection issue and allow retry without clearing the username field

### Requirement 2: Application Shell and Navigation (Merah Menyala Theme)

**User Story:** As a Vendor_User, I want a consistent branded layout with clear navigation, so that I can efficiently access my assigned work.

#### Acceptance Criteria

1. THE App_Shell SHALL render a maroon-red top bar using `oklch(40% 0.155 26)`, a sidebar with full-red active state using `oklch(54% 0.233 27)`, and a light content surface using `oklch(99.5% 0.003 40)` with text color `oklch(25% 0.02 28)`
2. THE App_Shell SHALL display the authenticated vendor company name (truncated with ellipsis at 30 characters) and user display name (truncated with ellipsis at 20 characters) in the top bar
3. THE App_Shell SHALL provide sidebar navigation links to: CIT Orders, Upload Evidence, Invoices, Replenishment Schedule, DSR Monitor, and Notifications
4. WHEN a navigation link is clicked, THE Vendor_Portal SHALL route to the corresponding screen without full page reload and within 300ms
5. THE App_Shell SHALL visually differentiate the active navigation item by applying the full-red `oklch(54% 0.233 27)` background with white text, while inactive items use the maroon-deep `oklch(30% 0.11 25)` sidebar background with muted text
6. WHILE the viewport width is less than 1024px, THE App_Shell SHALL collapse the sidebar to icon-only rail mode (64px width) by default, and SHALL provide a toggle button to expand the sidebar as a fixed overlay
7. THE App_Shell SHALL display a notification badge on the Notifications navigation item showing the unread count as a numeric value up to 99, displaying "99+" when the count exceeds 99, and hiding the badge when the unread count is zero
8. WHEN the logout button in the top bar is clicked, THE App_Shell SHALL clear the local session data and redirect the user to the login screen
9. IF session clearing fails during logout, THEN THE App_Shell SHALL still redirect the user to the login screen and display an error indication that the session could not be fully cleared
10. THE App_Shell sidebar navigation SHALL be accessible via keyboard, with each navigation item reachable by Tab key, activatable by Enter key, and the sidebar region identified by a navigation ARIA landmark

### Requirement 3: CIT Order Dashboard

**User Story:** As a Vendor_User, I want to view all CIT orders assigned to my company, so that I can track scheduled pickups and deliveries.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL display a CIT orders table with columns: Order ID, ATM ID, Location, Order Type (Pickup/Delivery), Scheduled Date, Amount (IDR), and Status
2. THE Vendor_Portal SHALL load CIT order data scoped to the authenticated vendor only, with a minimum of 20 mock records in the prototype
3. THE Vendor_Portal SHALL display order status using semantic badges with icon and label: Scheduled (info), In Transit (warning), Completed (success), Failed (danger)
4. THE Vendor_Portal SHALL display status filter tabs for All, Scheduled, In Transit, Completed, and Failed, with "All" selected by default on page load. WHEN a status filter tab is selected, THE Vendor_Portal SHALL show only orders matching the selected status while preserving any active date range filter (both filters combine with AND logic)
5. WHEN a date range filter is applied, THE Vendor_Portal SHALL show only orders with Scheduled Date falling within the specified range (inclusive of both start and end dates), combined with any active status filter using AND logic. IF only a start date is provided, THEN THE Vendor_Portal SHALL show orders from that date onward. IF only an end date is provided, THEN THE Vendor_Portal SHALL show orders up to and including that date
6. THE Vendor_Portal SHALL display a summary bar above the table showing total counts per status (Scheduled, In Transit, Completed, and Failed) reflecting the full unfiltered dataset, regardless of active filters
7. THE Vendor_Portal SHALL format Amount values as right-aligned IDR currency with dot-separated thousands (e.g., "IDR 250.000.000") using tabular-nums
8. THE Vendor_Portal SHALL sort the table by Scheduled Date descending (most recent first) by default
9. WHEN a column header is clicked, THE Vendor_Portal SHALL sort the table by that column. IF the column is not currently sorted, THEN the initial sort direction SHALL be ascending. IF the column is already sorted, THEN THE Vendor_Portal SHALL toggle between ascending and descending order
10. IF no orders match the active combination of status and date range filters, THEN THE Vendor_Portal SHALL display an empty state message indicating no orders match the current filters

### Requirement 4: Handover Evidence Upload

**User Story:** As a Vendor_User, I want to upload photographic proof when I complete a delivery, so that the bank has evidence of successful handover.

#### Acceptance Criteria

1. WHEN a CIT order with status "Completed" or "In Transit" is selected, THE Vendor_Portal SHALL display an evidence upload form with fields for: file attachment (minimum 1, maximum 5 files required), handover timestamp (must not be in the future and must not be more than 72 hours in the past relative to submission time), recipient name (maximum 100 characters), and optional notes (maximum 500 characters)
2. THE Vendor_Portal SHALL accept image files (JPEG, PNG) and PDF documents with a maximum file size of 10MB per file and a maximum of 5 files per submission
3. IF a file exceeds 10MB or is not an accepted format, THEN THE Vendor_Portal SHALL display a validation error message specifying the constraint violated and prevent upload
4. WHEN the evidence form is submitted with valid data, THE Vendor_Portal SHALL display a success confirmation and update the order's evidence indicator to show "Uploaded" with a timestamp
5. WHILE an upload is in progress, THE Vendor_Portal SHALL display a progress indicator and disable the submit button to prevent duplicate submissions
6. WHEN the Vendor_User attaches image files, THE Vendor_Portal SHALL display a thumbnail preview for each attached image and provide a remove button per file so the Vendor_User can verify and manage attachments before submission
7. IF a CIT order already has evidence uploaded, THEN THE Vendor_Portal SHALL display the existing evidence (thumbnails and metadata) in read-only mode with a label indicating the upload date
8. IF an upload fails due to a network error or server error, THEN THE Vendor_Portal SHALL display an error message indicating the failure, retain all form field values and attached files, and allow the Vendor_User to retry submission without re-entering data

### Requirement 5: Invoice Viewing

**User Story:** As a Vendor_User, I want to view my submitted invoices and their validation status, so that I can track payment progress.

#### Acceptance Criteria

1. THE Invoice_View SHALL display a table with columns: Invoice Number, Period, Total Amount (IDR), Line Items Count, and Validation Status
2. WHILE the user is authenticated as a Vendor_User, THE Invoice_View SHALL load and display only invoices belonging to that vendor's `vendorId`, with a minimum of 10 mock records in the prototype
3. THE Invoice_View SHALL display validation status using semantic badges with icon and label: Uploaded (info), Validated (warning), Mismatch Detected (danger), and Approved (success with a CheckCircle icon distinguishing it from Validated)
4. WHEN an invoice row is clicked or activated via keyboard (Enter or Space), THE Invoice_View SHALL display invoice detail showing line items with columns: Description, Invoiced Amount (IDR), Matched Order Reference, Expected Amount (IDR), Variance (IDR), and Match Status badge
5. THE Invoice_View SHALL format all monetary values as right-aligned IDR with dot-separated thousands using tabular-nums font variant
6. THE Invoice_View SHALL display a summary section above the table showing: total invoiced amount (sum of all displayed invoices), count of invoices per validation status (Uploaded, Validated, Mismatch Detected, Approved), and the sum of amounts for invoices with Approved status
7. IF an invoice has status "Mismatch Detected", THEN THE Invoice_View SHALL highlight each line item with matchStatus "Mismatch" using a danger background tint and display the variance amount in that row
8. IF the vendor has no invoices, THEN THE Invoice_View SHALL display an empty state message indicating no invoices are available for the current vendor

### Requirement 6: Replenishment Schedule

**User Story:** As a Vendor_User, I want to view upcoming replenishment schedules assigned to my company, so that I can prepare resources for execution.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL display a replenishment schedule table with columns: Schedule ID, ATM ID, Location, Scheduled Date, Recommended Amount (IDR), Priority, and Status
2. THE Vendor_Portal SHALL load replenishment schedule data scoped to the authenticated vendor only, with a minimum of 15 mock records in the prototype
3. THE Vendor_Portal SHALL display priority levels using semantic badges: High (danger), Medium (warning), Low (neutral)
4. THE Vendor_Portal SHALL display schedule status using semantic badges: Pending (warning), Confirmed (info), Executed (success), Cancelled (danger)
5. THE Vendor_Portal SHALL group the schedule view by date, showing a date header (formatted as "DD MMM YYYY", e.g. "15 Jan 2025") for each day with the total Recommended Amount and schedule count for that day
6. THE Vendor_Portal SHALL display the schedule sorted by Scheduled Date ascending (nearest first) and then by Priority descending (High first) within each date group
7. THE Vendor_Portal SHALL format Recommended Amount as right-aligned IDR with "Rp" prefix, dot-separated thousands, and no decimal places (e.g. "Rp 150.000.000") using tabular-nums
8. WHEN the page is initially loaded, THE Vendor_Portal SHALL display all schedule entries with Scheduled Date from the current date onward (today and future dates)
9. WHEN a date range filter is applied, THE Vendor_Portal SHALL show only schedule entries with Scheduled Date within the specified start and end dates (inclusive)
10. IF the current view contains no schedule entries (due to filtering or absence of data), THEN THE Vendor_Portal SHALL display an empty state message indicating no schedules are found for the selected period

### Requirement 7: DSR Monitor (Vendor-Scoped)

**User Story:** As a Vendor_User, I want to view daily cash positions for ATMs assigned to my company, so that I can anticipate upcoming replenishment needs.

#### Acceptance Criteria

1. THE DSR_View SHALL display a data table with columns: ATM ID, Location, Date, Beginning Balance (IDR), Cash In (IDR), Cash Out (IDR), Ending Balance (IDR), and Balance Status
2. THE DSR_View SHALL load DSR data scoped to ATMs assigned to the authenticated vendor only, with a minimum of 15 mock records in the prototype
3. THE DSR_View SHALL display balance status using semantic badges with icon and text: Critical (danger) when Ending Balance is strictly below 50,000,000 IDR, Low (warning) when Ending Balance is greater than or equal to 50,000,000 and less than or equal to 150,000,000 IDR, and Normal (success) when Ending Balance is strictly above 150,000,000 IDR
4. THE DSR_View SHALL provide a date selector defaulting to the prototype mock date of 2024-01-15
5. WHEN a date is selected, THE DSR_View SHALL reload the table with data for the selected date, sorted by ATM ID ascending as the default order
6. IF no DSR records exist for the selected date, THEN THE DSR_View SHALL display an empty state message indicating no DSR data is available for that date, in place of the data table
7. THE DSR_View SHALL display a summary card showing: total ATMs monitored (count of distinct ATM IDs), count of Critical ATMs, count of Low ATMs, and total Ending Balance across all assigned ATMs for the selected date
8. THE DSR_View SHALL format all monetary values as right-aligned IDR with dot-separated thousands using tabular-nums
9. WHEN a column header is clicked, THE DSR_View SHALL sort the table by that column, toggling between ascending and descending order on consecutive clicks

### Requirement 8: In-App Notifications

**User Story:** As a Vendor_User, I want to receive notifications about new assignments and order status changes, so that I can respond promptly.

#### Acceptance Criteria

1. THE Notification_Center SHALL display a list of notifications with columns: Timestamp (formatted as "DD MMM YYYY HH:mm"), Type, Message (truncated at 120 characters with ellipsis if exceeded), and Read Status
2. THE Notification_Center SHALL load mock notification data with a minimum of 10 records covering notification types: New Assignment, Order Status Changed, Invoice Status Updated, and Schedule Updated
3. THE Notification_Center SHALL visually differentiate unread notifications from read notifications using a bold font weight and a background tint on the unread row
4. WHEN a notification is clicked, THE Notification_Center SHALL mark the notification as read and navigate to the relevant screen: CIT Orders for New Assignment notifications, CIT Orders for Order Status Changed notifications, Invoice View for Invoice Status Updated notifications, and Schedule View for Schedule Updated notifications
5. THE App_Shell SHALL display the unread notification count as a badge on the Notifications sidebar item, showing the numeric count for values 1–99 and "99+" for counts exceeding 99, updated whenever notifications are marked as read
6. THE Notification_Center SHALL sort notifications by timestamp descending (newest first) by default
7. THE Notification_Center SHALL provide a "Mark All as Read" button that sets all notifications to read status and resets the badge count to zero, and THE Notification_Center SHALL disable the button when no unread notifications exist
8. IF no notifications exist, THEN THE Notification_Center SHALL display an empty state message indicating that there are no notifications

### Requirement 9: Mock Data Layer (Vendor-Scoped)

**User Story:** As a developer building the prototype, I want structured mock data files with realistic vendor-scoped banking data, so that the portal demonstrates proper data isolation.

#### Acceptance Criteria

1. THE Mock_Data_Layer SHALL provide static JSON files for: vendor users (with username, password, display name, and role fields per user), CIT orders, handover evidence (photo URL and timestamp per completed CIT order), invoices with line items, replenishment schedules, DSR records, and notifications (with type, message, timestamp, and read-status fields per notification)
2. THE Mock_Data_Layer SHALL define at minimum 3 vendor accounts (PT Gardanet, PT SSI, PT G4S) with distinct `vendor_id` values, and each vendor SHALL have at minimum 2 user credentials containing username, hashed-password placeholder, display name, and role
3. THE Mock_Data_Layer SHALL scope all business data (CIT orders, invoices, schedules, DSR records) to specific vendors via a `vendor_id` field, ensuring no data record is shared across vendors, with at minimum 5 CIT orders, 1 invoice, and 5 DSR records per vendor
4. THE Mock_Data_Layer SHALL use realistic ATM IDs following the pattern ATM-{REGION}-{NUMBER} (e.g., ATM-JKT-001, ATM-BDG-003, ATM-SBY-012) with at minimum 20 unique ATM IDs distributed across at minimum 4 region prefixes
5. THE Mock_Data_Layer SHALL use IDR monetary values in integer format (no decimals) representing realistic ATM cash amounts ranging from IDR 25,000,000 to IDR 500,000,000
6. THE Mock_Data_Layer SHALL provide data with consistent referential relationships: every CIT order references a valid ATM ID, every invoice references valid CIT order IDs, every schedule references a valid ATM ID, and every DSR record references an ATM ID assigned to the corresponding vendor
7. THE Mock_Data_Layer SHALL include date fields spanning a contiguous 14-day period with DSR records for all 20 ATMs on each day (minimum 280 total DSR records), to support daily DSR viewing, multi-day schedule planning, and order history
8. THE Mock_Data_Layer SHALL be consumed via TanStack Query hooks with `queryFn` implementations that read from static JSON imports, structured so that swapping to real API calls requires changing only the `queryFn` implementation
9. WHEN a TanStack Query hook is invoked with a specific `vendor_id` parameter, THE Mock_Data_Layer SHALL return only records whose `vendor_id` matches the parameter, returning an empty array if no records match

### Requirement 10: Docker Containerization

**User Story:** As a developer, I want the Vendor Portal to run in its own Docker container separate from the internal app, so that both can be deployed and scaled independently.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL include its own Dockerfile at `frontend/VendorPortal-Vite/Dockerfile` using a multi-stage build: first stage runs `pnpm install --frozen-lockfile` and `pnpm build`, second stage copies build artifacts into an `nginx:alpine` image and copies the nginx configuration file into `/etc/nginx/conf.d/default.conf`
2. THE Vendor_Portal SHALL include its own `docker-compose.yml` defining a single service that maps container port 80 to host port 3001 by default (distinct from the internal app on port 3000), configurable via the `PORT` environment variable (e.g., `PORT=3002` maps host port 3002 to container port 80)
3. WHEN `docker compose up` is executed in the `frontend/VendorPortal-Vite/` directory, THE Vendor_Portal SHALL return an HTTP 200 response with content-type `text/html` on the root path (`/`) within 30 seconds of the command completing
4. THE Vendor_Portal SHALL include a `.dockerignore` file excluding at minimum `node_modules`, `dist`, `.env`, and `.git`
5. THE Vendor_Portal SHALL include an Nginx configuration that uses `try_files $uri $uri/ /index.html` to support client-side routing, and caches static assets (files matching js, css, png, jpg, svg, woff, woff2 extensions) with an `expires` directive of at least 30 days
6. THE Vendor_Portal SHALL maintain its own `package.json`, `vite.config.ts`, `tsconfig.json`, and `tailwind.config` independent from the internal frontend, with no shared `node_modules` directory, no shared build output directory, and no import paths referencing files outside the `frontend/VendorPortal-Vite/` directory
7. THE Vendor_Portal Docker service SHALL define a health check that verifies HTTP connectivity to the root path, enabling container orchestrators to determine service readiness within 5 seconds per check attempt

### Requirement 11: Routing and Access Control

**User Story:** As a Vendor_User, I want to direct URLs for each screen and enforcement that I cannot access internal-only features, so that bookmarks work and security boundaries are maintained.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL use React Router for client-side routing with browser history mode (pushState URLs without hash fragments)
2. THE Vendor_Portal SHALL define routes: `/login`, `/orders` (CIT Orders dashboard), `/orders/:id/evidence` (evidence upload), `/invoices`, `/schedule`, `/dsr`, and `/notifications`
3. WHEN an unauthenticated user accesses any route other than `/login`, THE Vendor_Portal SHALL preserve the originally requested URL and redirect to `/login` within 100ms of route evaluation
4. WHEN an authenticated user completes login after being redirected from a protected route, THE Vendor_Portal SHALL redirect to the originally requested URL rather than a default route
5. WHEN an authenticated user accesses `/login` directly, THE Vendor_Portal SHALL redirect to `/orders`
6. WHEN an unknown route is accessed, THE Vendor_Portal SHALL display a page containing the text "Page not found" (or equivalent localized message), a visual indicator (icon or illustration), and a navigation link back to `/orders`
7. WHEN a user navigates to a route designated as internal-only (admin settings, user management, forecasting, reconciliation, or approval workflows), THE Vendor_Portal SHALL display the same "Page not found" page as for unknown routes, without revealing that the route exists on the internal system
8. WHEN a user navigates to a defined route via direct URL entry or browser refresh, THE Vendor_Portal SHALL render the corresponding screen fully without requiring prior navigation from root, and browser back/forward buttons SHALL navigate between previously visited routes without full page reload
9. THE Vendor_Portal SHALL perform all route transitions as client-side navigations without triggering a full page reload, except for the initial page load

### Requirement 12: Vendor Data Isolation

**User Story:** As a system administrator, I want the portal to enforce strict data isolation between vendors, so that no vendor can view another vendor's operational data.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL filter all data queries at the API layer by the `vendor_id` extracted from the authenticated JWT claims, regardless of parameters supplied by the client
2. WHEN the Mock_Data_Layer is queried, THE Mock_Data_Layer SHALL return only records matching the logged-in vendor's `vendor_id`
3. THE Vendor_Portal SHALL NOT display any UI controls that would allow a Vendor_User to select or search for other vendors' data
4. IF a Vendor_User attempts to access a resource (order, invoice, schedule) not assigned to their vendor, THEN THE Vendor_Portal SHALL display an access denied message and log the attempt including the authenticated user identifier, the requested resource identifier, and a timestamp
5. THE Vendor_Portal SHALL NOT expose aggregate data across multiple vendors in any summary, chart, or count display
6. IF the authenticated JWT does not contain a valid `vendor_id` claim, THEN THE Vendor_Portal SHALL deny access to all vendor-scoped resources and display an error message indicating an invalid session

### Requirement 13: Responsive Layout and Accessibility

**User Story:** As a Vendor_User accessing the portal from various devices, I want the interface to be usable on desktop and tablet, so that I can check assignments from the field.

#### Acceptance Criteria

1. WHILE the viewport width is less than 1024px, THE App_Shell SHALL collapse the sidebar to a 64px-wide icon-only rail using the maroon-deep `oklch(30% 0.11 25)` background color, hiding navigation labels and displaying only icons
2. WHILE the viewport width is less than 1024px, THE Vendor_Portal SHALL wrap each data table in a horizontally scrollable container with a visible horizontal scrollbar so that no content is clipped and the user can discover overflow content
3. THE Vendor_Portal SHALL use semantic HTML landmarks — `<nav>` for navigation regions, `<main>` for primary content, `<header>` for the top bar, `<table>` for tabular data, and `<form>` for input groups — so that screen readers can announce page regions and allow landmark-based navigation
4. THE Vendor_Portal SHALL never signal status with color alone — every status indicator (Badge component instances conveying state such as order status, validation status, or schedule compliance) SHALL render both a descriptive icon and a text label alongside the color
5. THE Vendor_Portal SHALL maintain a minimum touch target size of 44×44 CSS pixels (width and height, including padding) for all interactive elements such as buttons, links, and form controls
6. THE Vendor_Portal SHALL maintain WCAG 2.1 AA color contrast ratio (minimum 4.5:1 for text below 18.66px bold or 24px regular, minimum 3:1 for text at or above those thresholds) between text and its background across all content areas, including text rendered on the maroon top bar `oklch(40% 0.155 26)` and the maroon-deep sidebar `oklch(30% 0.11 25)`
7. WHEN a Vendor_User navigates via keyboard, THE Vendor_Portal SHALL display a visible focus indicator (matching the design system focus ring) on the currently focused interactive element, and all interactive elements SHALL be reachable via sequential Tab key navigation without requiring a pointing device
