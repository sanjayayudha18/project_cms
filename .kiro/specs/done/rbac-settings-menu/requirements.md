# Requirements Document

## Introduction

This feature adds an RBAC management area to the internal Company Portal (frontend/CompanyPortal-Vite). Today the portal has a dead "Pengaturan" (Settings) nav link and a set of write-only admin RBAC endpoints on the ATM backend, but no user interface to view or manage RBAC state, and no list/read endpoints.

The feature delivers a Settings hub landing page with RBAC sub-pages that let authorized administrators view and manage all RBAC entities: user supervisor hierarchy and approval level, approval delegations, user leaves, and approval policies, plus a read-only view of role assignments. It adds the missing GET/list endpoints on the ATM backend (reading from the replica pool), widens the existing role guard to include APPACCESS, and preserves audit logging on every mutation.

Scope constraints: RBAC configuration changes remain direct ADMIN-only administrative actions with no new maker-checker approval gate; no payment execution; no new roles are introduced. The UI follows the internal "Merah Sirih" theme with Bahasa Indonesia labels.

## Glossary

- **Settings_Hub**: The `/settings` landing page in the internal Company Portal that links to RBAC sub-pages.
- **RBAC_Area**: The collection of RBAC management sub-pages under `/settings/rbac/*` (users, delegations, leaves, policies).
- **Authorized_Role**: One of the roles permitted to access the RBAC_Area and its endpoints: `ADMIN`, `ADMIN_PARAM`, or `APPACCESS`.
- **Company_Portal**: The internal React frontend (frontend/CompanyPortal-Vite), authenticated via LDAP-issued JWT.
- **ATM_Backend**: The Go + Chi backend on port 8080 that exposes flat-JSON admin RBAC endpoints and (new) list endpoints.
- **Primary_Pool**: The PostgreSQL primary connection pool used for all writes and transactional reads.
- **Replica_Pool**: The PostgreSQL read replica connection pool used for reporting and list/read queries.
- **Audit_Writer**: The `internal/audit` component that appends an entry to `audit_logs` for every state change (who, what, before/after, when, ip).
- **User_Hierarchy**: A user's `supervisor_id` (self-FK) and `approval_level` (integer) fields.
- **Approval_Delegation**: A record in `approval_delegations` routing approval authority from one user to another over a time window.
- **User_Leave**: A record in `user_leaves` marking a user unavailable over a time window.
- **Approval_Policy**: A record in `approval_policies` mapping a `document_type` plus an amount range (min/max) to a `required_level`.
- **Role_Assignment_View**: A read-only listing of users and their assigned roles.
- **Merah_Sirih_Theme**: The internal design theme (OKLCH tokens, red as a <=10% accent, tables with tabular-nums, status conveyed by badge plus icon, soft red focus ring).

## Requirements

### Requirement 1: Access the Settings hub and RBAC sub-pages

**User Story:** As an authorized administrator, I want to open a Settings hub that links to RBAC management sub-pages, so that I can navigate to the RBAC entity I need to manage.

#### Acceptance Criteria

1. WHEN a user with an Authorized_Role navigates to `/settings`, THE Company_Portal SHALL display the Settings_Hub landing page with navigation links to the RBAC sub-pages `/settings/rbac/users`, `/settings/rbac/delegations`, `/settings/rbac/leaves`, and `/settings/rbac/policies`.
2. WHERE the navigation configuration defines the "settings" item, THE Company_Portal SHALL include `ADMIN`, `ADMIN_PARAM`, and `APPACCESS` in the roles permitted to see the Settings nav item.
3. IF a user whose role is not an Authorized_Role attempts to open `/settings` or any `/settings/rbac/*` route, THEN THE Company_Portal SHALL block access and SHALL NOT render the RBAC_Area content.
4. WHEN a user whose role is not an Authorized_Role views the navigation, THE Company_Portal SHALL omit the Settings nav item from that user's navigation.
5. WHEN a user with an Authorized_Role selects an RBAC sub-page link on the Settings_Hub, THE Company_Portal SHALL route the user to the corresponding `/settings/rbac/*` page.

### Requirement 2: List RBAC entities from the replica

**User Story:** As an authorized administrator, I want each RBAC sub-page to display the current state of its entity, so that I can review existing configuration before making changes.

#### Acceptance Criteria

1. THE ATM_Backend SHALL expose a GET list endpoint for user hierarchy that returns users with their `supervisor_id`, `approval_level`, `role`, and `auth_source`.
2. THE ATM_Backend SHALL expose a GET list endpoint for approval delegations that returns each delegation's `from_user_id`, `to_user_id`, `start_at`, `end_at`, and `reason`.
3. THE ATM_Backend SHALL expose a GET list endpoint for user leaves that returns each leave's `user_id`, `start_at`, `end_at`, and `reason`.
4. THE ATM_Backend SHALL expose a GET list endpoint for approval policies that returns each policy's `document_type`, minimum amount, maximum amount, and `required_level`.
5. WHEN the ATM_Backend serves any RBAC list endpoint, THE ATM_Backend SHALL read from the Replica_Pool.
6. WHEN the ATM_Backend serves any RBAC list endpoint, THE ATM_Backend SHALL return the response as flat JSON consistent with the existing ATM_Backend response shape.
7. WHERE an RBAC list endpoint is defined, THE ATM_Backend SHALL restrict access to the Authorized_Roles `ADMIN`, `ADMIN_PARAM`, and `APPACCESS`.
8. IF a request to an RBAC list endpoint carries a role that is not an Authorized_Role, THEN THE ATM_Backend SHALL respond with HTTP status 403.
9. WHEN an RBAC sub-page loads in the Company_Portal, THE Company_Portal SHALL fetch the corresponding list endpoint and display the returned records in a table.
10. WHILE an RBAC list request is in progress, THE Company_Portal SHALL display a loading state.
11. IF an RBAC list request returns an error, THEN THE Company_Portal SHALL display an error message that includes text and an icon.

### Requirement 3: Widen role authorization to include APPACCESS

**User Story:** As an APPACCESS administrator, I want access to the RBAC management endpoints and pages, so that I can perform account and RBAC configuration within my authority.

#### Acceptance Criteria

1. THE ATM_Backend SHALL authorize requests to the existing admin RBAC write endpoints under `/api/v1/admin/approval` for roles `ADMIN`, `ADMIN_PARAM`, and `APPACCESS`.
2. THE ATM_Backend SHALL authorize requests to the new RBAC list endpoints for roles `ADMIN`, `ADMIN_PARAM`, and `APPACCESS`.
3. WHEN a request without a valid authenticated session reaches any RBAC endpoint, THE ATM_Backend SHALL respond with HTTP status 401.

### Requirement 4: Create and edit user hierarchy

**User Story:** As an authorized administrator, I want to set a user's supervisor and approval level, so that the approval reporting line reflects the current organization.

#### Acceptance Criteria

1. WHEN an authorized administrator submits a hierarchy change for a user with a `supervisor_id` and an `approval_level`, THE ATM_Backend SHALL update that user's `supervisor_id` and `approval_level` using the Primary_Pool.
2. IF a hierarchy change request sets a user's `supervisor_id` equal to that same user's own id, THEN THE ATM_Backend SHALL reject the change and SHALL NOT persist it.
3. WHEN an authorized administrator opens the user hierarchy sub-page, THE Company_Portal SHALL present an editable form for `supervisor_id` and `approval_level` per user.
4. WHEN a hierarchy change succeeds, THE Company_Portal SHALL reflect the updated `supervisor_id` and `approval_level` in the displayed list.
5. IF the ATM_Backend rejects a hierarchy change, THEN THE Company_Portal SHALL display a validation error with text and an icon and SHALL retain the user's entered values.

### Requirement 5: Create and revoke approval delegations

**User Story:** As an authorized administrator, I want to create delegations and revoke them, so that approval authority is correctly routed while a user is unavailable.

#### Acceptance Criteria

1. WHEN an authorized administrator submits a delegation with `from_user_id`, `to_user_id`, `start_at`, `end_at`, and an optional `reason`, THE ATM_Backend SHALL create the Approval_Delegation using the Primary_Pool.
2. THE ATM_Backend SHALL accept and interpret `start_at` and `end_at` delegation timestamps in RFC3339 format.
3. IF a submitted delegation overlaps an existing delegation for the same `from_user_id`, THEN THE ATM_Backend SHALL respond with HTTP status 409 and SHALL NOT create the delegation.
4. WHEN an authorized administrator revokes an existing delegation by id, THE ATM_Backend SHALL remove that Approval_Delegation using the Primary_Pool.
5. WHEN a delegation is created or revoked successfully, THE Company_Portal SHALL update the displayed delegation list to reflect the change.
6. IF the ATM_Backend responds with HTTP status 409 for an overlapping delegation, THEN THE Company_Portal SHALL display a conflict message with text and an icon.

### Requirement 6: Create user leaves

**User Story:** As an authorized administrator, I want to record a user's leave period, so that approval fallback applies while that user is on leave.

#### Acceptance Criteria

1. WHEN an authorized administrator submits a leave with `user_id`, `start_at`, `end_at`, and an optional `reason`, THE ATM_Backend SHALL create the User_Leave using the Primary_Pool.
2. THE ATM_Backend SHALL accept and interpret `start_at` and `end_at` leave timestamps in RFC3339 format.
3. WHEN a leave is created successfully, THE Company_Portal SHALL update the displayed leave list to reflect the new record.
4. IF a leave submission fails validation at the ATM_Backend, THEN THE Company_Portal SHALL display an error message with text and an icon.

### Requirement 7: Manage approval policies

**User Story:** As an authorized administrator, I want to view approval policies mapping document type and amount range to a required level, so that I can confirm which approval level is required for each document type.

#### Acceptance Criteria

1. WHEN an authorized administrator opens the approval policies sub-page, THE Company_Portal SHALL display each Approval_Policy's `document_type`, minimum amount, maximum amount, and `required_level` in a table.
2. WHERE amounts are displayed in the policy table, THE Company_Portal SHALL render amount values using tabular figures and right alignment.
3. WHERE a create or edit endpoint for approval policies is available, THE Company_Portal SHALL provide a form to create or edit an Approval_Policy's `document_type`, amount range, and `required_level`.

### Requirement 8: Audit every RBAC mutation

**User Story:** As a compliance stakeholder, I want every RBAC configuration change recorded, so that the system retains an auditable trail of who changed what and when.

#### Acceptance Criteria

1. WHEN the ATM_Backend persists a user hierarchy change, THE ATM_Backend SHALL write an entry to `audit_logs` via the Audit_Writer capturing actor, action, before and after state, timestamp, and ip.
2. WHEN the ATM_Backend creates or revokes an Approval_Delegation, THE ATM_Backend SHALL write an entry to `audit_logs` via the Audit_Writer capturing actor, action, before and after state, timestamp, and ip.
3. WHEN the ATM_Backend creates a User_Leave, THE ATM_Backend SHALL write an entry to `audit_logs` via the Audit_Writer capturing actor, action, before and after state, timestamp, and ip.
4. WHERE a create or edit operation on an Approval_Policy is performed, THE ATM_Backend SHALL write an entry to `audit_logs` via the Audit_Writer capturing actor, action, before and after state, timestamp, and ip.

### Requirement 9: View role assignments

**User Story:** As an authorized administrator, I want to see which role each user is assigned, so that I can confirm current role assignments alongside the hierarchy.

#### Acceptance Criteria

1. WHEN an authorized administrator views the user hierarchy sub-page, THE Company_Portal SHALL display each user's assigned `role` alongside the User_Hierarchy fields.
2. WHERE a user's role is displayed, THE Company_Portal SHALL render the role using a badge that pairs a label with an icon.

### Requirement 10: UI and theme conformance

**User Story:** As an internal user, I want the RBAC pages to match the internal design system and language, so that the experience is consistent and accessible.

#### Acceptance Criteria

1. THE Company_Portal SHALL render RBAC_Area labels and copy in Bahasa Indonesia.
2. THE Company_Portal SHALL apply the Merah_Sirih_Theme tokens to the RBAC_Area, keeping red as an accent limited to primary actions, active states, and focus.
3. WHERE tabular data is displayed in the RBAC_Area, THE Company_Portal SHALL render tables with tabular figures and right-aligned numeric columns.
4. WHERE an interactive input receives keyboard focus in the RBAC_Area, THE Company_Portal SHALL display a soft red focus ring.
5. WHEN the Company_Portal signals an error or a status in the RBAC_Area, THE Company_Portal SHALL convey it with an icon and text label rather than color alone.

### Requirement 11: Scope exclusions

**User Story:** As a system owner, I want the RBAC settings feature to stay within its defined boundary, so that unrelated controls and behaviors are not introduced.

#### Acceptance Criteria

1. THE ATM_Backend SHALL apply RBAC configuration changes directly as administrative actions without creating a maker-checker approval request.
2. THE RBAC_Area SHALL NOT trigger or execute any payment.
3. THE RBAC_Area SHALL NOT introduce any role beyond the existing defined roles.
