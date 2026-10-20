# Requirements Document

## Introduction

CMS App Foundation defines the foundational user interface shell for the End-to-End Cash Management System (CMS) at CIMB Niaga STCC. This spec follows an outside-in approach: deliver a demoable UI with working navigation, authentication, dashboard, and one complete end-to-end flow (DSR Upload) before backend services are fully implemented. API calls are stubbed initially and replaced with real backends incrementally.

The foundation provides the structural skeleton upon which all three functional modules (ATM Cash Forecasting, Vendor Invoice Payment, Cash Count) will be built.

## Glossary

- **CMS**: The End-to-End Cash Management System application
- **App_Shell**: The persistent layout frame containing sidebar navigation, header bar, and main content area
- **Sidebar**: The collapsible vertical navigation panel on the left side of the App_Shell
- **Header**: The horizontal bar at the top of the App_Shell displaying user identity, notifications, and global actions
- **Auth_Service**: The frontend authentication module responsible for login, session management, and token refresh
- **API_Client**: The typed HTTP client generated from the OpenAPI spec that communicates with the backend
- **Router**: TanStack Router instance managing file-based, type-safe route navigation
- **Dashboard**: The landing page after login showing summary metrics and quick-access widgets
- **DSR**: Daily Status Report uploaded by vendors containing vault balance, fill plan, reconciliation result, and shortage claims
- **DSR_Upload_Flow**: The complete user journey from file selection through parsing, validation, preview, and submission of a DSR Excel file
- **RBAC**: Role-Based Access Control determining which navigation items and actions a user can access
- **Session**: The authenticated user state stored client-side as a JWT token with associated role claims
- **Stub_API**: A mock service layer returning static or generated data to enable frontend development before backend completion
- **Protected_Route**: A route that requires valid authentication and appropriate role authorization before rendering
- **Toast**: A non-blocking notification element that appears temporarily to confirm actions or report errors

## Requirements

### Requirement 1: Application Shell Layout

**User Story:** As an STCC team member, I want a consistent application layout with sidebar navigation and header, so that I can navigate the system efficiently across all modules.

#### Acceptance Criteria

1. THE App_Shell SHALL render a fixed-position Sidebar on the left, a Header at the top, and a scrollable main content area occupying the remaining viewport space
2. THE Sidebar SHALL display navigation items grouped by module: Forecasting, Invoice, Cash Count, and a general section for Dashboard and Settings
3. WHEN a user clicks a Sidebar navigation item, THE Router SHALL navigate to the corresponding route without a full page reload
4. WHILE SPA route transitions are occurring, THE App_Shell SHALL prevent full page reloads; any full page reload during route navigation is a violation
5. THE Sidebar SHALL visually indicate the currently active route by highlighting the corresponding navigation item with the brand accent color
6. WHEN the viewport width is below 1024px, THE Sidebar SHALL collapse to an icon-only state with a toggle button to expand
7. THE Header SHALL display the authenticated user full name, role badge, and a logout action
8. THE App_Shell SHALL render all text labels in Bahasa Indonesia

### Requirement 2: Authentication and Session Management

**User Story:** As an STCC team member, I want to log in with my credentials and have my session persist, so that I can access the system securely without repeated logins.

#### Acceptance Criteria

1. WHEN an unauthenticated user accesses any Protected_Route, THE Router SHALL redirect the user to the login page
2. THE Auth_Service SHALL present a login form requesting email and password fields
3. WHEN valid credentials are submitted, THE Auth_Service SHALL store the returned JWT token in memory and set a refresh token in an httpOnly cookie
4. WHEN a login attempt fails, THE Auth_Service SHALL display an inline error message indicating invalid credentials without revealing which field is incorrect
5. WHILE a valid Session exists, THE Auth_Service SHALL refresh the access token before expiry using the refresh token
6. WHEN the user clicks the logout action, THE Auth_Service SHALL clear the Session and redirect to the login page
7. IF the refresh token is expired or revoked, THEN THE Auth_Service SHALL clear the Session and redirect to the login page with a session-expired notification

### Requirement 3: Role-Based Navigation Visibility

**User Story:** As a system administrator, I want navigation items to reflect user roles, so that each user sees only the modules they have permission to access.

#### Acceptance Criteria

1. THE Sidebar SHALL render navigation items based on the role claims present in the authenticated Session
2. WHEN a user with the ATM_Support role is authenticated, THE Sidebar SHALL display Forecasting module navigation items
3. WHEN a user with the Vendor role is authenticated, THE Sidebar SHALL display only DSR Upload, Fill Instruction Download, and Invoice Upload navigation items
4. WHEN a user with the Cash_Count_PIC role is authenticated, THE Sidebar SHALL display Cash Count module navigation items
5. WHEN a user with the WMO role is authenticated, THE Sidebar SHALL display Invoice module navigation items
6. IF a user navigates directly to a route outside their role permissions, THEN THE Router SHALL display an unauthorized access page with a return-to-dashboard link
7. THE Sidebar SHALL hide the Settings navigation item from users without the Admin role

### Requirement 4: Dashboard Landing Page

**User Story:** As an STCC team member, I want a dashboard showing relevant summary information after login, so that I can quickly assess the current operational status.

#### Acceptance Criteria

1. WHEN an authenticated user navigates to the root path, THE Router SHALL render the Dashboard page
2. THE Dashboard SHALL display summary metric cards showing: total ATM/CRM machines active, pending fill instructions today, open reconciliation items, and pending approvals count
3. THE Dashboard SHALL display metric values using the tabular-nums font variant and right-aligned IDR formatting
4. THE Dashboard SHALL display a recent activity feed showing the last 10 system events relevant to the user role
5. WHEN the API_Client fails to fetch dashboard data, THE Dashboard SHALL hide the metric cards section completely and display a retry button and/or an error state message instead of blank content; the retry button and error message MAY appear independently of each other
6. THE Dashboard SHALL render metric cards using placeholder data from the Stub_API until explicitly configured to use the real backend; the system SHALL NOT automatically detect or switch to the real backend without explicit configuration

### Requirement 5: Module Landing Pages

**User Story:** As an STCC team member, I want landing pages for each module, so that I can access all sub-functions within Forecasting, Invoice, and Cash Count from a clear entry point.

#### Acceptance Criteria

1. THE Router SHALL define routes for three module landing pages: /forecasting, /invoice, and /cash-count
2. WHEN a user navigates to a module landing page, THE CMS SHALL render a page title, module description, and navigation cards linking to each sub-function within that module
3. THE Forecasting landing page SHALL display navigation cards for: DSR Receipt, Fill Instruction, Fill Validation, Cash Supply, H+2 Projection, and Holiday Calendar
4. THE Invoice landing page SHALL display navigation cards for: Invoice Upload, Reconciliation, Charge Calculation, and Document Generation
5. THE Cash Count landing page SHALL display navigation cards for: Scheduling, Balance Tier Analysis, Execution (Berita Acara), Checklists, Reconciliation, and Recapitulation
6. WHEN a sub-function is not yet implemented, THE module landing page SHALL display the navigation card in a disabled state with a "Segera Hadir" label

### Requirement 6: DSR Upload Flow (End-to-End)

**User Story:** As a Vendor user, I want to upload the daily DSR Excel file and see validation results, so that I can confirm the daily report was received and processed correctly.

#### Acceptance Criteria

1. WHEN a Vendor user navigates to /forecasting/dsr-upload, THE CMS SHALL render a file upload interface accepting .xlsx and .xls files
2. THE DSR_Upload_Flow SHALL restrict file size to a maximum of 10MB and reject files exceeding this limit with a descriptive error message
3. WHEN a valid Excel file is selected, THE DSR_Upload_Flow SHALL parse the file client-side and display a preview table of the first 20 rows with column headers
4. THE preview table SHALL display vault balance, vendor fill plan, reconciliation result, and shortage claim columns mapped from the uploaded file
5. WHEN the user confirms the upload, THE API_Client SHALL send the parsed data to the POST /api/v1/forecasting/dsr endpoint
6. WHEN the backend explicitly returns an acceptance response, THE DSR_Upload_Flow SHALL display a success Toast with the upload timestamp and record count
7. IF the backend rejects the upload due to validation errors, THEN THE DSR_Upload_Flow SHALL display a validation error summary listing each row number and field that failed validation
8. IF the uploaded file structure does not match the expected DSR template columns, THEN THE DSR_Upload_Flow SHALL reject the file at the preview step with a message identifying the missing or unexpected columns
9. THE DSR_Upload_Flow SHALL display an upload history table below the upload form showing the most recent 30 uploads with date, filename, row count, and status regardless of total upload count

### Requirement 7: Design Token Integration

**User Story:** As a developer, I want the application to use the OKLCH design token system consistently, so that the UI matches the CIMB Niaga brand identity and remains visually cohesive.

#### Acceptance Criteria

1. THE App_Shell SHALL define CSS custom properties for all OKLCH color tokens specified in the design system (brand red hue 29, neutrals hue 29, semantic colors)
2. THE App_Shell SHALL use the system font stack for all text rendering
3. THE App_Shell SHALL apply tabular-nums font variant to all numeric displays including amounts, counts, and table cells containing numbers
4. THE App_Shell SHALL apply the 4pt spacing scale using CSS custom properties for all padding, margin, and gap values
5. THE App_Shell SHALL use brand red (hue 29) exclusively for primary actions and identity elements, and danger red (hue 12) exclusively for destructive actions and error states; pages containing only destructive actions MAY use only danger red without requiring brand red presence, but WHEN both primary and destructive action types are present on a page, both colors SHALL be used simultaneously
6. THE App_Shell SHALL render all monetary values in IDR format with thousand separators and right alignment

### Requirement 8: API Client with Stub Layer

**User Story:** As a frontend developer, I want an API client that can switch between stubbed and real backends, so that UI development proceeds independently of backend completion.

#### Acceptance Criteria

1. THE API_Client SHALL be generated from the OpenAPI specification and expose typed request and response interfaces for all endpoints
2. WHILE the backend endpoint is unavailable, THE Stub_API SHALL intercept requests and return realistic mock data with simulated latency between 200ms and 800ms
3. WHEN an explicit configuration change switches the API_Client to the real backend, THE API_Client SHALL route all new and in-progress requests to the real backend immediately without requiring frontend code changes; the switch SHALL NOT occur via automatic detection
4. THE API_Client SHALL include an authorization header with the JWT token from the active Session on every request to a Protected_Route endpoint
5. IF the API_Client receives a 401 response, THEN THE Auth_Service SHALL attempt a token refresh before retrying the original request once
6. IF the retry also receives a 401 response, THEN THE Auth_Service SHALL clear the Session and redirect to the login page

### Requirement 9: Error Handling and Loading States

**User Story:** As an STCC team member, I want clear feedback during data loading and when errors occur, so that I understand the system state and can take corrective action.

#### Acceptance Criteria

1. WHILE the API_Client is fetching data, THE CMS SHALL display a skeleton loading state matching the expected content layout
2. WHEN a network request fails with a connection error, THE CMS SHALL display a retry action and a message indicating network unavailability
3. WHEN a network request fails with a server error (HTTP 500), THE CMS SHALL display a generic error message without exposing technical details to the user
4. THE CMS SHALL display Toast notifications for successful mutations (uploads, approvals, saves) that auto-dismiss after 5 seconds
5. THE CMS SHALL display Toast notifications for failed mutations that persist until manually dismissed by the user
6. IF a page component throws an unhandled error, THEN THE CMS SHALL render an error boundary fallback with a page-reload option instead of a blank screen

### Requirement 10: Accessibility and Internationalization Foundation

**User Story:** As a user with diverse abilities, I want the application to meet basic accessibility standards, so that I can operate the system effectively with keyboard and assistive technology.

#### Acceptance Criteria

1. THE App_Shell SHALL support full keyboard navigation including focus management when routes change
2. THE Sidebar SHALL be navigable via arrow keys when focused, and items SHALL be activatable with Enter or Space
3. THE CMS SHALL provide aria-label attributes on all icon-only buttons and interactive elements without visible text
4. THE CMS SHALL maintain a minimum contrast ratio of 4.5:1 for normal text and 3:1 for large text against their background colors
5. THE CMS SHALL set the document language attribute to "id" (Bahasa Indonesia)
6. WHEN a Toast notification appears, THE CMS SHALL announce the notification content to screen readers using an aria-live polite region; THE Toast SHALL display visually regardless of whether the screen reader announcement succeeds
