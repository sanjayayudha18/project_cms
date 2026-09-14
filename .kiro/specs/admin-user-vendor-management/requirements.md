# Requirements Document

## Introduction

Admin User & Vendor Management is an admin-facing capability in the CMS internal app (`frontend/CompanyPortal-Vite`) that lets an authorized administrator **add**, **edit**, and **disable/enable** two kinds of master records:

1. **Users** — the `users` table (internal LDAP users and vendor/local users), including role assignment, vendor linkage, and reporting-line / approval-level fields.
2. **Vendors** — the `vendors` master table (CIT/FLM vendor companies), including code, name, and contact details.

It has two parts:

1. A set of **write + read APIs** on the ATM backend (`backend/`, port 8080) under `/api/v1/admin/users` and a new `/api/v1/admin/vendors` group, returning the ATM backend's existing **flat JSON** shape (not the `pkg/response` envelope) for wire compatibility.
2. Two **admin screens** in `CompanyPortal-Vite`: a Users management screen at `/admin/users` and a Vendors management screen at `/admin/vendors`, each a filterable, paginated list with add / edit / disable-enable actions.

**Disable is a soft action, never a hard delete.** Users are disabled via the existing soft-delete pattern (`is_active=false`, `deleted_at=now()`); vendors have the same `is_active` + `deleted_at` columns and follow the same pattern. Hard `DELETE` on `users` is already forbidden and enforced by `backend/internal/repository/no_hard_delete_test.go`; this spec extends the same guarantee to `vendors`.

Every state-changing action writes an entry to the append-only `audit_logs` trail (project-context Sec 4, non-negotiable), recording actor, action, before/after, and IP.

## Scope & Grounding Notes (what exists today)

Confirmed against the codebase at spec authoring time:

- **Users**: schema exists (`002_cms_tables.sql`, altered by `016`, `021`, `026`). `users` has `is_active` + `deleted_at`. Soft-delete/reactivate is **implemented but unwired**: `internal/auth/deactivate_user.go` (`DeactivateUserService.Deactivate/Reactivate`, audited) + repo methods `AuthRepository.Deactivate/Reactivate` + sqlc `DeactivateUser`/`ReactivateUser`, but no route is mounted in `cmd/api/main.go`. `set-initial-password` (APPACCESS) is the only mounted admin-user route. **No CreateUser / UpdateUser exists anywhere.**
- **Vendors**: schema exists (`002_cms_tables.sql`, seeded by `005`). `vendors` has `code` (UNIQUE), `name`, `contact_email`, `contact_phone`, `hq_address`, `is_active`, `deleted_at`. **No vendor CRUD of any kind exists** — `vendor_requests` (`028`) is a different domain (replenishment orders), not vendor master data.
- **Roles**: `roles` table, vocabulary per `002` comment + `027` (`APPACCESS`). Seeded, read-only in this spec (no role CRUD).
- **RBAC wiring**: `r.With(custommw.RequireAuth(tokenService), custommw.RequireRoles(...)).Mount(...)` in `cmd/api/main.go`. `/api/v1/admin/users` is guarded by `RequireRoles("APPACCESS")`; `/api/v1/admin/approval` and `/api/v1/audit-logs` by `RequireRoles("ADMIN","ADMIN_PARAM")`.
- **Audit**: `auditWriter := audit.NewWriter(dbPool)` shared; `auditWriter.Write(ctx, audit.Entry{...})`.
- **Approval / maker-checker**: `internal/approval` orchestrator exists and is the single approval state machine (project-context Sec 2 "Approval integration pattern").
- **Frontend**: no existing admin/user/vendor area. `routes/_protected.tsx` exports `protectedRoute` + `requireRoles(allowedRoles: DbRole[])`. Shared UI in `src/components/ui/`: `DataTable`, `PageHeader`, `Badge`, `FilterSelect`, `Button`, `Card`, `EmptyState`, `Toast`. **No Modal/Drawer component exists** — a form modal/drawer is new. `DbRole` in `lib/auth/store.ts` does **not** yet include `"APPACCESS"`.
- **sqlc**: `.sql` sources in `backend/queries/*.sql`, generated `backend/internal/db/*.sql.go`. `sqlc generate` is currently blocked by a pre-existing bug in migration `017` (missing table name); new queries may need hand-written `.sql.go` matching sqlc output until `017` is fixed (see design.md).

## Resolved Decisions (confirmed by product owner, 2026-09-11)

1. **Role guards — CONFIRMED.** User management (`Admin_Users_Page` + `User_Admin_API`) is guarded by `APPACCESS`; vendor management (`Admin_Vendors_Page` + `Vendor_Admin_API`) is guarded by `ADMIN`/`ADMIN_PARAM`. (`ADMIN`/`ADMIN_PARAM` also bypass the user-page `requireRoles` guard by the existing helper behavior.)
2. **No maker-checker — CONFIRMED.** Add/edit/disable/enable of users and vendors **apply immediately with mandatory audit**, matching the existing account-provisioning actions (`set-initial-password`, `deactivate`). No approval gate. Requirement 9 is the binding audit guarantee. (A maker-checker variant remains sketched in design.md only as future reference; it is out of scope here.)
3. **Temporary password on user create — CONFIRMED.** When creating a **local** user, the admin/APPACCESS sets a **temporary password** at creation time. The account is created with `must_change_password=true` so the user can log in and is then routed straight to the change-password screen on first login (reusing the existing `must_change_password` policy and self-service change-password flow). LDAP/internal users never get a local password.

## Glossary

- **Admin_Users_Page**: The route at `/admin/users` in CompanyPortal-Vite listing and managing user records.
- **Admin_Vendors_Page**: The route at `/admin/vendors` in CompanyPortal-Vite listing and managing vendor records.
- **User_Form_Dialog**: The add/edit form (modal or drawer) for a single user record.
- **Vendor_Form_Dialog**: The add/edit form (modal or drawer) for a single vendor record.
- **User_Admin_API**: The ATM backend endpoints under `/api/v1/admin/users` (list, get, create, update, disable, enable).
- **Vendor_Admin_API**: The ATM backend endpoints under `/api/v1/admin/vendors` (list, get, create, update, disable, enable).
- **Flat_JSON_Shape**: The ATM backend's flat JSON response convention (`{error, message, details}` on error), not the `pkg/response` `{success,data}` envelope.
- **Soft_Disable**: Setting `is_active=false` and `deleted_at=now()` on a record, never issuing SQL `DELETE`.
- **Enable**: Reversing Soft_Disable — `is_active=true`, `deleted_at=NULL`.
- **Audit_Writer**: The existing `internal/audit` `Writer` constructed once as `audit.NewWriter(dbPool)`.
- **API_Client**: The existing `api` client in `lib/api/client.ts` (Bearer token injection, single-flight 401 refresh).
- **Badge_Component**: The existing shared `Badge` with variants `success | warning | danger | info | neutral`.
- **DataTable**: The existing TanStack-Table-backed `DataTable` component.
- **TanStack_Query**: TanStack Query v5, used for server state, caching, and mutations.

## Requirements

### Requirement 1: Routes and Access Control

**User Story:** As a system administrator, I want the user and vendor management pages restricted to the correct admin roles, so that master data is not exposed to or edited by unauthorized users.

#### Acceptance Criteria

1. THE Admin_Users_Page SHALL be registered as a file-based route under `protectedRoute` at path `/admin/users`.
2. THE Admin_Vendors_Page SHALL be registered as a file-based route under `protectedRoute` at path `/admin/vendors`.
3. THE Admin_Users_Page SHALL apply the `requireRoles` guard in `beforeLoad` with allowed role `APPACCESS` (`ADMIN`/`ADMIN_PARAM` bypass all role checks by the existing helper behavior).
4. THE Admin_Vendors_Page SHALL apply the `requireRoles` guard in `beforeLoad` with allowed roles `ADMIN` and `ADMIN_PARAM`.
5. WHEN a user whose role is not permitted attempts to access either page, THE page SHALL render the existing Forbidden (403) state and SHALL NOT render or fetch management data.
6. WHEN an unauthenticated user attempts to access either page, THE page SHALL redirect to `/login` with the original path preserved in the `redirect` search parameter (existing `protectedRoute` behavior).
7. THE User_Admin_API SHALL be mounted behind `RequireAuth` and `RequireRoles("APPACCESS")` in `cmd/api/main.go`.
8. THE Vendor_Admin_API SHALL be mounted behind `RequireAuth` and `RequireRoles("ADMIN","ADMIN_PARAM")` in `cmd/api/main.go`.
9. WHEN either API receives a request without a valid token, IT SHALL respond with HTTP 401.
10. WHEN either API receives a request from a role that is not permitted, IT SHALL respond with HTTP 403.
11. IF the frontend `DbRole` union does not include a role required by these guards (e.g. `"APPACCESS"`), THEN that role SHALL be added to the `DbRole` union in `lib/auth/store.ts` as part of this feature.

### Requirement 2: List Users — Filtering and Pagination

**User Story:** As an administrator, I want to filter and page through user accounts, so that I can find the account I need to manage.

#### Acceptance Criteria

1. THE User_Admin_API SHALL expose `GET /api/v1/admin/users` accepting optional query parameters: `q` (matches username, full_name, or email), `role`, `vendor_id`, `status` (`active` | `disabled` | `all`), `page`, `page_size`.
2. WHEN `q` is provided, THE User_Admin_API SHALL return users whose username, full_name, or email matches the term (case-insensitive, substring).
3. WHEN `role` is provided, THE User_Admin_API SHALL return only users whose role matches.
4. WHEN `vendor_id` is provided, THE User_Admin_API SHALL return only users linked to that vendor.
5. WHEN `status=active`, THE User_Admin_API SHALL return only users with `deleted_at IS NULL`; WHEN `status=disabled`, only users with `deleted_at IS NOT NULL`; WHEN `status=all` or absent, both, with a default of `active`.
6. THE User_Admin_API SHALL default `page` to 1 and `page_size` to 25, and SHALL cap `page_size` at 100.
7. THE User_Admin_API SHALL order results by `full_name` ascending, with `id` ascending as a stable tiebreaker.
8. THE User_Admin_API SHALL return, in the Flat_JSON_Shape, the page of users (id, username, full_name, email, role, is_karyawan, auth_source, vendor_id, is_active, deleted_at, supervisor_id, approval_level, last_login_at) plus pagination metadata `page`, `page_size`, `total`.
9. THE User_Admin_API SHALL NOT include `password_hash` in any response.
10. WHEN an invalid parameter is supplied (non-numeric `page`/`page_size`/`vendor_id`, unknown `status`), THE User_Admin_API SHALL respond with HTTP 400 and a descriptive error.

### Requirement 3: Create User

**User Story:** As an administrator, I want to add a new user account, so that a new internal or vendor user can access the system.

#### Acceptance Criteria

1. THE User_Admin_API SHALL expose `POST /api/v1/admin/users` accepting: `username`, `full_name`, `email`, `role` (or `role_id`), `is_karyawan`, `auth_source` (`ldap` | `local`), `temporary_password` (required when `auth_source=local`), `employee_id` (optional), `vendor_id` (optional), `supervisor_id` (optional), `approval_level` (optional).
2. THE User_Admin_API SHALL validate that `username`, `full_name`, `email`, `role`, and `auth_source` are present and non-empty, and that `email` is a valid email format.
3. WHEN `auth_source=local`, THE User_Admin_API SHALL require `vendor_id` to be present (vendor/local users are vendor-scoped per schema comment) AND require a `temporary_password` that passes the existing `ValidatePasswordStrength` policy; IT SHALL store the bcrypt hash of that password (via the existing `SetInitialPassword` repository path) and set `must_change_password=true`, so the user can log in and is then routed to the change-password screen on first login (Resolved Decision 3).
4. WHEN `auth_source=ldap`, THE User_Admin_API SHALL require `vendor_id` to be absent, SHALL reject any `temporary_password` in the request with HTTP 400, and SHALL store no `password_hash`.
5. WHEN `username`, `email`, or `employee_id` collides with an existing record (including a soft-disabled one), THE User_Admin_API SHALL respond with HTTP 409 and a descriptive error identifying the conflicting field.
6. WHEN `role` / `role_id` does not resolve to an existing role, THE User_Admin_API SHALL respond with HTTP 400.
7. WHEN `vendor_id` or `supervisor_id` is provided but does not reference an existing record, THE User_Admin_API SHALL respond with HTTP 400.
8. WHEN creation succeeds, THE User_Admin_API SHALL respond with HTTP 201 and the created user (without `password_hash`) in the Flat_JSON_Shape.
9. WHEN creation succeeds, THE User_Admin_API SHALL write an `audit_logs` entry with action `user_created`, entity_type `user`, entity_id = new user id, `before` null, `after` = the created record (excluding any password material), actor = the acting admin, and the actor IP.
10. WHEN any validation fails, THE User_Admin_API SHALL respond with HTTP 400 (or 422 for field validation, matching the backend's existing `writeValidationError` convention) and SHALL NOT create the user or write an audit entry.

### Requirement 4: Edit User

**User Story:** As an administrator, I want to edit an existing user's details, so that I can correct information or change role, vendor, or reporting line.

#### Acceptance Criteria

1. THE User_Admin_API SHALL expose `PUT /api/v1/admin/users/{id}` accepting the editable fields: `full_name`, `email`, `role`/`role_id`, `is_karyawan`, `employee_id`, `vendor_id`, `supervisor_id`, `approval_level`.
2. THE User_Admin_API SHALL NOT allow editing `username` or `auth_source` via this endpoint (identity and auth-path are immutable post-creation); a request attempting to change them SHALL be rejected with HTTP 400.
3. THE User_Admin_API SHALL NOT set or change `password_hash` via this endpoint (password changes go through the existing `set-initial-password` / self-service change-password flows).
4. WHEN the target `id` does not exist (or is soft-disabled and not being edited by an enable action), THE User_Admin_API SHALL respond with HTTP 404.
5. WHEN `email` or `employee_id` is changed to a value that collides with a different existing record, THE User_Admin_API SHALL respond with HTTP 409.
6. WHEN `supervisor_id` equals the user's own `id`, THE User_Admin_API SHALL respond with HTTP 400 (the schema CHECK forbids self-supervision).
7. WHEN referenced `role`, `vendor_id`, or `supervisor_id` does not exist, THE User_Admin_API SHALL respond with HTTP 400.
8. WHEN the update succeeds, THE User_Admin_API SHALL respond with HTTP 200 and the updated record (without `password_hash`).
9. WHEN the update succeeds, THE User_Admin_API SHALL write an `audit_logs` entry with action `user_updated`, `before` = the record prior to the change and `after` = the record after, actor, and IP.

### Requirement 5: Disable and Enable User

**User Story:** As an administrator, I want to disable a user so they can no longer log in, and re-enable them if needed, without erasing their history.

#### Acceptance Criteria

1. THE User_Admin_API SHALL expose `POST /api/v1/admin/users/{id}/disable` performing Soft_Disable (`is_active=false`, `deleted_at=now()`) via the existing `DeactivateUserService.Deactivate` path.
2. THE User_Admin_API SHALL expose `POST /api/v1/admin/users/{id}/enable` performing Enable (`is_active=true`, `deleted_at=NULL`) via the existing `DeactivateUserService.Reactivate` path.
3. THE User_Admin_API SHALL NOT expose any hard-`DELETE` route for users; the append-only audit linkage (`audit_logs.actor_id -> users.id`) SHALL remain intact for disabled users.
4. WHEN disable targets a non-existent user, THE User_Admin_API SHALL respond with HTTP 404.
5. WHEN disable succeeds, THE User_Admin_API SHALL respond with HTTP 200 and write an `audit_logs` entry with action `user_deactivated` (existing action name), actor, and IP.
6. WHEN enable succeeds, THE User_Admin_API SHALL respond with HTTP 200 and write an `audit_logs` entry with action `user_reactivated` (existing action name), actor, and IP.
7. THE User_Admin_API SHALL prevent an administrator from disabling their own account (self-lockout guard), responding with HTTP 400 if `{id}` equals the acting user's id.

### Requirement 6: List Vendors — Filtering and Pagination

**User Story:** As an administrator, I want to filter and page through vendor companies, so that I can find the vendor I need to manage.

#### Acceptance Criteria

1. THE Vendor_Admin_API SHALL expose `GET /api/v1/admin/vendors` accepting optional query parameters: `q` (matches code or name), `status` (`active` | `disabled` | `all`), `page`, `page_size`.
2. WHEN `q` is provided, THE Vendor_Admin_API SHALL return vendors whose `code` or `name` matches (case-insensitive, substring).
3. WHEN `status=active`, THE Vendor_Admin_API SHALL return only vendors with `deleted_at IS NULL`; `status=disabled` returns only `deleted_at IS NOT NULL`; `status=all` or absent returns both, default `active`.
4. THE Vendor_Admin_API SHALL default `page` to 1 and `page_size` to 25, capping `page_size` at 100.
5. THE Vendor_Admin_API SHALL order results by `name` ascending, `id` ascending as tiebreaker.
6. THE Vendor_Admin_API SHALL return, in the Flat_JSON_Shape, the page of vendors (id, code, name, contact_email, contact_phone, hq_address, is_active, deleted_at) plus pagination metadata `page`, `page_size`, `total`.
7. WHEN an invalid parameter is supplied, THE Vendor_Admin_API SHALL respond with HTTP 400.

### Requirement 7: Create and Edit Vendor

**User Story:** As an administrator, I want to add a new vendor company and edit its details, so that vendor master data stays accurate.

#### Acceptance Criteria

1. THE Vendor_Admin_API SHALL expose `POST /api/v1/admin/vendors` accepting: `code`, `name`, `contact_email` (optional), `contact_phone` (optional), `hq_address` (optional).
2. THE Vendor_Admin_API SHALL require `code` and `name` to be present and non-empty, and SHALL validate `contact_email` as an email format when provided.
3. WHEN `code` collides with an existing vendor (including a soft-disabled one), THE Vendor_Admin_API SHALL respond with HTTP 409.
4. WHEN creation succeeds, THE Vendor_Admin_API SHALL respond with HTTP 201 and the created vendor, and SHALL write an `audit_logs` entry with action `vendor_created`, entity_type `vendor`, `before` null, `after` = created record, actor, IP.
5. THE Vendor_Admin_API SHALL expose `PUT /api/v1/admin/vendors/{id}` accepting `name`, `contact_email`, `contact_phone`, `hq_address`.
6. THE Vendor_Admin_API SHALL NOT allow editing `code` via update (vendor code is the stable external identifier); a request attempting to change it SHALL be rejected with HTTP 400.
7. WHEN the target vendor `id` does not exist, THE Vendor_Admin_API SHALL respond with HTTP 404.
8. WHEN the update succeeds, THE Vendor_Admin_API SHALL respond with HTTP 200 and write an `audit_logs` entry with action `vendor_updated`, `before`/`after`, actor, IP.

### Requirement 8: Disable and Enable Vendor

**User Story:** As an administrator, I want to disable a vendor that is no longer active and re-enable it if needed, without losing its historical records.

#### Acceptance Criteria

1. THE Vendor_Admin_API SHALL expose `POST /api/v1/admin/vendors/{id}/disable` performing Soft_Disable (`is_active=false`, `deleted_at=now()`).
2. THE Vendor_Admin_API SHALL expose `POST /api/v1/admin/vendors/{id}/enable` performing Enable (`is_active=true`, `deleted_at=NULL`).
3. THE Vendor_Admin_API SHALL NOT expose any hard-`DELETE` route for vendors; existing rows referencing a vendor SHALL remain intact.
4. WHEN disable targets a non-existent vendor, THE Vendor_Admin_API SHALL respond with HTTP 404.
5. WHEN a vendor is disabled WHILE it still has active (non-disabled) users linked to it, THE Vendor_Admin_API SHALL surface a warning in the response (the disable still succeeds), so the admin is aware linked users remain able to log in.
6. WHEN disable succeeds, THE Vendor_Admin_API SHALL write an `audit_logs` entry with action `vendor_deactivated`, actor, IP; enable writes `vendor_reactivated`.

### Requirement 9: Audit Guarantee

**User Story:** As an auditor, I want every add/edit/disable/enable action on users and vendors recorded, so that master-data changes are fully traceable.

#### Acceptance Criteria

1. THE User_Admin_API and Vendor_Admin_API SHALL write exactly one `audit_logs` entry per successful state-changing action (create, update, disable, enable) via the Audit_Writer.
2. THE audit entry SHALL record actor id (the acting admin), action, entity_type (`user` | `vendor`), entity_id, `before`/`after` payloads where applicable, and the actor IP (extracted from the request per the existing `extractClientIP` helper).
3. THE audit entry SHALL NEVER include password material (`password_hash`, plaintext passwords) in `before` or `after`.
4. IF the audit write fails, THEN the action SHALL surface an error (matching the existing services' behavior where a failed audit write returns an error), so that no silent unaudited mutation occurs.
5. Read (list/get) operations SHALL NOT write audit entries.

### Requirement 10: Users Management Screen

**User Story:** As an administrator, I want a clear screen to view and manage users, so that I can perform add/edit/disable from one place.

#### Acceptance Criteria

1. THE Admin_Users_Page SHALL render a `PageHeader` titled "Manajemen Pengguna" with a subtitle and a primary "Tambah Pengguna" action button.
2. THE Admin_Users_Page SHALL render a `DataTable` with columns: Nama (full_name), Username, Email, Role (as Badge), Vendor, Status (active/disabled, as Badge with icon + label), and an Aksi (actions) column.
3. THE Status Badge SHALL render both an icon and a text label — never signalled by color alone.
4. THE Admin_Users_Page SHALL provide filter controls mapping to the list endpoint: a search input (`q`), a Role `FilterSelect`, a Vendor `FilterSelect`, and a Status `FilterSelect` (Aktif / Nonaktif / Semua).
5. WHEN the administrator changes any filter, THE Admin_Users_Page SHALL reset to page 1 and refetch with the updated parameters, reflecting active filters in the URL search parameters.
6. WHEN the administrator clicks "Tambah Pengguna", THE Admin_Users_Page SHALL open the User_Form_Dialog in create mode.
7. WHEN the administrator triggers Edit on a row, THE Admin_Users_Page SHALL open the User_Form_Dialog pre-filled with that user's current values in edit mode.
8. WHEN the administrator triggers Disable/Enable on a row, THE Admin_Users_Page SHALL show a confirmation prompt and, on confirm, call the corresponding endpoint and refetch the list.
9. WHEN no users match the current filters, THE Admin_Users_Page SHALL display an `EmptyState`.
10. THE Admin_Users_Page SHALL display a loading skeleton while the list query is pending and an inline error with a retry control if it fails.
11. WHEN a mutation (create/update/disable/enable) succeeds or fails, THE Admin_Users_Page SHALL show a `Toast` reflecting the outcome and, on success, invalidate the list query.

### Requirement 11: User Form (Add / Edit)

**User Story:** As an administrator, I want a guided form for creating and editing users, so that I enter valid data and understand which fields apply to which auth source.

#### Acceptance Criteria

1. THE User_Form_Dialog SHALL be built with React Hook Form + Zod (project tech standard), validating required fields client-side before submit.
2. THE User_Form_Dialog SHALL present fields: Username, Nama Lengkap, Email, Role (select), Auth Source (`ldap`/`local`), Karyawan (boolean), Employee ID (optional), Vendor (select, required when Auth Source = `local`), Password Sementara (required when Auth Source = `local`, create mode only), Supervisor (optional), Approval Level (optional).
3. WHEN Auth Source = `local`, THE User_Form_Dialog SHALL require and show the Vendor and Password Sementara fields, and SHALL communicate that the user will be forced to change this password on first login; WHEN `ldap`, it SHALL hide/disable Vendor and Password Sementara and SHALL communicate that no password is set here.
3a. WHEN the server rejects the `temporary_password` for weak strength (422 via `ValidatePasswordStrength`), THE User_Form_Dialog SHALL surface the message against the Password Sementara field and SHALL NOT close.
4. IN edit mode, THE User_Form_Dialog SHALL disable Username and Auth Source (immutable per Requirement 4.2).
5. WHEN the server returns a field-level validation (422) or conflict (409) error, THE User_Form_Dialog SHALL surface the message against the relevant field and SHALL NOT close.
6. WHEN submission succeeds, THE User_Form_Dialog SHALL close and the list SHALL refetch.
7. THE User_Form_Dialog SHALL be keyboard-accessible: focus moves into the dialog on open, is trapped while open, and returns to the trigger on close; Escape closes the dialog.

### Requirement 12: Vendors Management Screen and Form

**User Story:** As an administrator, I want a screen to view and manage vendors with an add/edit form, so that vendor master data is maintainable from the internal app.

#### Acceptance Criteria

1. THE Admin_Vendors_Page SHALL render a `PageHeader` titled "Manajemen Vendor" with a primary "Tambah Vendor" action.
2. THE Admin_Vendors_Page SHALL render a `DataTable` with columns: Kode (code), Nama (name), Email Kontak, Telepon, Status (Badge icon + label), and Aksi.
3. THE Admin_Vendors_Page SHALL provide a search input (`q`) and a Status `FilterSelect`, reflected in URL search parameters, resetting to page 1 on change.
4. THE Vendor_Form_Dialog SHALL be built with React Hook Form + Zod, with fields Kode, Nama, Email Kontak, Telepon, Alamat HQ; Kode SHALL be disabled in edit mode (immutable per Requirement 7.6).
5. WHEN the server returns a 409 code conflict, THE Vendor_Form_Dialog SHALL surface the message against the Kode field and SHALL NOT close.
6. WHEN Disable is triggered on a vendor with active linked users, THE Admin_Vendors_Page SHALL display the backend warning (Requirement 8.5) in the confirmation or resulting Toast.
7. THE Admin_Vendors_Page SHALL show loading, empty, error, and toast states equivalent to the users page (Requirement 10.9–10.11).

### Requirement 13: Responsive Layout and Accessibility

**User Story:** As a user on varied screen sizes, I want the pages usable and accessible, so that I can manage records effectively.

#### Acceptance Criteria

1. THE pages SHALL ensure all interactive elements (buttons, filter controls, table row actions, dialog controls) have a minimum touch target of 44x44 CSS pixels.
2. THE pages SHALL use semantic HTML landmarks: `main` for page content and a `table` within the DataTable; dialogs SHALL use `role="dialog"` with an accessible label and `aria-modal`.
3. THE DataTables SHALL scroll horizontally on narrow viewports rather than truncating columns.
4. THE pages SHALL render all Badges with icon plus label text, never signalling meaning by color alone (design-system hard rule; error/status never color-only).
5. THE pages SHALL use the design system's OKLCH "Merah Sirih" (internal) tokens exclusively — no hardcoded hex or RGB, one primary action per view.
6. THE pages SHALL announce dynamic changes (dialog open, toast) via an `aria-live` region.
7. THE ID / numeric / phone columns SHALL use `tabular-nums` where they render numeric values for alignment.

### Requirement 14: Feature Module Organization and Backend Layout

**User Story:** As a developer, I want the feature organized to the project's conventions, so that it stays maintainable and matches the codebase.

#### Acceptance Criteria

1. THE frontend feature code SHALL reside in `src/features/admin-users/` and `src/features/admin-vendors/` (feature-based organization), sharing a small `src/features/admin/` for any common admin primitives (e.g. the form dialog shell) if warranted.
2. Each feature SHALL define TypeScript types for all API request/response shapes in a dedicated types file, TanStack Query hooks (list/get + mutations) in a dedicated hooks file, and feature-specific components (page, form dialog) in a components directory.
3. The features SHALL reuse existing shared components (DataTable, PageHeader, Badge, FilterSelect, EmptyState, Card, Button, Toast) without duplicating them; a new accessible dialog/modal primitive MAY be added to `src/components/ui/` since none exists yet.
4. THE backend code SHALL follow the ATM backend's actual technical-layer layout: `internal/handler` (admin_user_handler extended + new admin_vendor_handler), `internal/service` (or `internal/auth` for user-write logic, matching where `DeactivateUserService`/`SetInitialPasswordService` live), `internal/repository` + sqlc `backend/queries/*.sql`, returning the Flat_JSON_Shape.
5. THE new backend write paths SHALL use `dbPool` (primary) for writes and reads-after-write, with a documented replica TODO for list/read endpoints matching the convention used elsewhere in `cmd/api/main.go`.
6. THE feature SHALL extend the existing `no_hard_delete_test.go` guarantee to cover vendors (no hard `DELETE` on `vendors`).
