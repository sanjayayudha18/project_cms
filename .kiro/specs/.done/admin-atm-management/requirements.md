# Requirements Document

## Introduction

Admin ATM Management is an admin-facing capability in the CMS internal app (`frontend/CompanyPortal-Vite`) that lets an authorized administrator **add**, **edit**, and **disable/enable** records in the **`atms`** master table (ATM/CRM machine master data): terminal id, location linkage, machine type/brand/model, operating profile, deployment type, capacity and threshold amounts, and escrow/priority attributes.

It is the **third leg** of the admin master-data set. Users and Vendors are already covered by the sibling spec `admin-user-vendor-management` (`/api/v1/admin/users`, `/api/v1/admin/vendors`, `/admin/users`, `/admin/vendors`). This spec adds the parallel ATM stack and deliberately reuses that spec's decisions and patterns; it does **not** re-open or duplicate the user/vendor work.

It has two parts:

1. A set of **write + read APIs** on the ATM backend (`backend/`, port 8080) under a new `/api/v1/admin/atms` group, returning the ATM backend's existing **flat JSON** shape (not the `pkg/response` envelope) for wire compatibility.
2. An **admin screen** in `CompanyPortal-Vite`: an ATM management screen at `/admin/atms`, a filterable, paginated list with add / edit / disable-enable actions.

**Disable is a soft action, never a hard delete.** `atms` has the same `is_active` + `deleted_at` columns as `users`/`vendors` and follows the identical soft-delete pattern. This spec extends the existing `backend/internal/repository/no_hard_delete_test.go` guarantee to cover `atms`.

Every state-changing action writes an entry to the append-only `audit_logs` trail (project-context Sec 4, non-negotiable), recording actor, action, before/after, and IP.

## Scope & Grounding Notes (what exists today)

Confirmed against the codebase at spec authoring time:

- **ATMs**: schema exists (`002_cms_tables.sql`, altered by `030`). `atms` has `id`, `terminal_id` (UNIQUE), `location_id` (FK → `locations`, NOT NULL), `machine_type`, `brand`, `model`, `operation_hours`, `deployment_type` (all NOT NULL text), `capacity_amount`, `low_threshold_amount`, `critical_threshold_amount` (all `numeric(20,2)`, nullable), `blacklisted` (bool, default false), `is_active` (bool, default true), `escrow_account` (text, from `030`), `priority_class` (text `VIP | Non VIP | Industri`, from `030`), `created_at`, `updated_at`, `deleted_at`.
- **ATM read stack exists (read-only).** `internal/handler/atm_portal_handler.go` (`AtmPortalHandler`) mounts GET-only routes under `/api/v1/atm-portal`: `/atms` (paginated/filtered list + summary), `/atms/{terminalId}` (profile), plus replenish/cashpos history. Backed by `internal/service/atm_portal.go` (`AtmPortalService`). **This is a monitoring viewer, not master-data CRUD** — it computes replenishment status and joins cash-position data. This spec does NOT modify the atm-portal viewer; it adds a separate admin CRUD stack, exactly as `admin-user-vendor-management` did rather than overloading the read viewer.
- **No ATM create/update/disable/enable exists anywhere.**
- **Locations**: `locations` table exists (`002`), `atms.location_id` is a NOT NULL FK. A new ATM MUST reference an existing location. There is no location CRUD in scope here (read-only, to populate a Location select / validate the reference).
- **Related child tables**: `atm_denoms` (FK → `atms`, `ON DELETE CASCADE`) and `atm_vendor_packages` (FK → `atms`, `ON DELETE CASCADE`). This spec does NOT manage denomination or vendor-package assignment (out of scope); it only manages the `atms` row itself. Because disable is soft (never a SQL `DELETE`), the CASCADE never fires — child rows are preserved for disabled ATMs.
- **RBAC wiring**: `r.With(custommw.RequireAuth(tokenService), custommw.RequireRoles(...)).Mount(...)` in `cmd/api/main.go`.
- **Audit**: `auditWriter := audit.NewWriter(dbPool)` shared; `auditWriter.Write(ctx, audit.Entry{...})`.
- **Flat JSON helpers**: `writeJSON` / `writeError` / `writeValidationError` / `writeForbidden` / `writeUnauthorized` / `extractClientIP` / `parsePathID` in `internal/handler/error_response.go`.
- **Frontend**: the sibling spec introduces `src/components/ui/Dialog.tsx` (accessible modal) and the `admin-users` / `admin-vendors` feature layout. This spec reuses that same `Dialog` primitive and mirrors the feature structure in a new `src/features/admin-atms/`. Shared UI (`DataTable`, `PageHeader`, `Badge`, `FilterSelect`, `Button`, `Card`, `EmptyState`, `Toast`) already exists.
- **sqlc**: `.sql` sources in `backend/queries/*.sql`, generated `backend/internal/db/*.sql.go`. `sqlc generate` is currently blocked by a pre-existing bug in migration `017` (missing table name); new queries may need hand-written `.sql.go` matching sqlc output until `017` is fixed (same precedent as audit-log-viewer and admin-user-vendor-management).

## Resolved Decisions (inherited from `admin-user-vendor-management`, confirmed 2026-09-11) + ATM-specific

The ATM stack follows the same three resolved decisions as the user/vendor stack, with one ATM-specific role question flagged:

1. **No maker-checker — CONFIRMED (inherited).** Add/edit/disable/enable of ATMs **apply immediately with mandatory audit**, matching the user/vendor decision and the existing account-provisioning actions. No approval gate. Requirement 8 is the binding audit guarantee.
2. **Soft-disable, no hard delete — CONFIRMED (inherited).** Disable = `is_active=false`, `deleted_at=now()`; enable reverses it. No hard `DELETE` route; the guard test is extended to `atms`.
3. **No password/auth concerns.** Unlike users, ATMs carry no auth material, so the temporary-password rule is N/A here.

- **ATM role guard — CONFIRMED (2026-09-15).** `/api/v1/admin/atms` and `/admin/atms` are guarded by `ADMIN` and `ADMIN_PARAM` (matching vendors). The screen is surfaced as a card in the Settings hub. This resolves the prior open question.

## Glossary

- **Admin_ATMs_Page**: The route at `/admin/atms` in CompanyPortal-Vite listing and managing ATM records.
- **ATM_Form_Dialog**: The add/edit form (modal) for a single ATM record.
- **ATM_Admin_API**: The ATM backend endpoints under `/api/v1/admin/atms` (list, get, create, update, disable, enable).
- **Flat_JSON_Shape**: The ATM backend's flat JSON response convention (`{error, message, details}` on error), not the `pkg/response` `{success,data}` envelope.
- **Soft_Disable**: Setting `is_active=false` and `deleted_at=now()` on a record, never issuing SQL `DELETE`.
- **Enable**: Reversing Soft_Disable — `is_active=true`, `deleted_at=NULL`.
- **Audit_Writer**: The existing `internal/audit` `Writer` constructed once as `audit.NewWriter(dbPool)`.
- **API_Client**: The existing `api` client in `lib/api/client.ts` (Bearer token injection, single-flight 401 refresh).
- **Badge_Component**: The existing shared `Badge` with variants `success | warning | danger | info | neutral`.
- **DataTable**: The existing TanStack-Table-backed `DataTable` component.
- **TanStack_Query**: TanStack Query v5, used for server state, caching, and mutations.
- **Dialog**: The accessible modal primitive at `src/components/ui/Dialog.tsx` introduced by `admin-user-vendor-management`, reused here.

## Requirements

### Requirement 1: Routes and Access Control

**User Story:** As a system administrator, I want the ATM management page restricted to the correct admin roles, so that ATM master data is not exposed to or edited by unauthorized users.

#### Acceptance Criteria

1. THE Admin_ATMs_Page SHALL be registered as a route under `protectedRoute` at path `/admin/atms`.
2. THE Admin_ATMs_Page SHALL apply the `requireRoles` guard in `beforeLoad` with allowed roles `ADMIN` and `ADMIN_PARAM` (`ADMIN`/`ADMIN_PARAM` bypass all role checks by the existing helper behavior).
2a. THE Admin_ATMs_Page SHALL be reached from the **Settings hub** (`/settings`, `SettingsHubPage`) via a dedicated "Manajemen ATM" card, NOT via a standalone top-level `NAV_CONFIG` sidebar item. The Settings hub is accessible to `ADMIN` and `ADMIN_PARAM`. (Confirmed 2026-09-15.)
3. WHEN a user whose role is not permitted attempts to access the page, THE page SHALL render the existing Forbidden (403) state and SHALL NOT render or fetch management data.
4. WHEN an unauthenticated user attempts to access the page, THE page SHALL redirect to `/login` with the original path preserved in the `redirect` search parameter (existing `protectedRoute` behavior).
5. THE ATM_Admin_API SHALL be mounted behind `RequireAuth` and `RequireRoles("ADMIN","ADMIN_PARAM")` in `cmd/api/main.go`.
6. WHEN the API receives a request without a valid token, IT SHALL respond with HTTP 401.
7. WHEN the API receives a request from a role that is not permitted, IT SHALL respond with HTTP 403.

### Requirement 2: List ATMs — Filtering and Pagination

**User Story:** As an administrator, I want to filter and page through ATM records, so that I can find the ATM I need to manage.

#### Acceptance Criteria

1. THE ATM_Admin_API SHALL expose `GET /api/v1/admin/atms` accepting optional query parameters: `q` (matches `terminal_id`), `brand`, `machine_type`, `deployment_type`, `priority_class`, `location_id`, `status` (`active` | `disabled` | `all`), `page`, `page_size`.
2. WHEN `q` is provided, THE ATM_Admin_API SHALL return ATMs whose `terminal_id` matches the term (case-insensitive, substring).
3. WHEN `brand`, `machine_type`, `deployment_type`, `priority_class`, or `location_id` is provided, THE ATM_Admin_API SHALL return only ATMs matching that exact value.
4. WHEN `status=active`, THE ATM_Admin_API SHALL return only ATMs with `deleted_at IS NULL`; WHEN `status=disabled`, only `deleted_at IS NOT NULL`; WHEN `status=all` or absent, both, with a default of `active`.
5. THE ATM_Admin_API SHALL default `page` to 1 and `page_size` to 25, and SHALL cap `page_size` at 100.
6. THE ATM_Admin_API SHALL order results by `terminal_id` ascending, with `id` ascending as a stable tiebreaker.
7. THE ATM_Admin_API SHALL return, in the Flat_JSON_Shape, the page of ATMs (id, terminal_id, location_id, location_name, machine_type, brand, model, operation_hours, deployment_type, capacity_amount, low_threshold_amount, critical_threshold_amount, blacklisted, priority_class, escrow_account, is_active, deleted_at) plus pagination metadata `page`, `page_size`, `total`.
8. THE ATM_Admin_API SHALL render monetary amount fields (`capacity_amount`, `low_threshold_amount`, `critical_threshold_amount`) as decimal strings or nullable numbers (never float-truncated), consistent with the existing atm-portal handler's money-as-string convention (project-context Sec 5, money never float).
9. WHEN an invalid parameter is supplied (non-numeric `page`/`page_size`/`location_id`, unknown `status`), THE ATM_Admin_API SHALL respond with HTTP 400 and a descriptive error.

### Requirement 3: Create ATM

**User Story:** As an administrator, I want to add a new ATM record, so that a newly deployed machine is registered in the system.

#### Acceptance Criteria

1. THE ATM_Admin_API SHALL expose `POST /api/v1/admin/atms` accepting: `terminal_id`, `location_id`, `machine_type`, `brand`, `model`, `operation_hours`, `deployment_type` (all required), `capacity_amount`, `low_threshold_amount`, `critical_threshold_amount`, `priority_class`, `escrow_account`, `blacklisted` (all optional).
2. THE ATM_Admin_API SHALL validate that the required fields are present and non-empty.
3. WHEN `location_id` does not reference an existing `locations` row, THE ATM_Admin_API SHALL respond with HTTP 400 with an `invalid_reference` error.
4. WHEN `terminal_id` collides with an existing ATM (including a soft-disabled one), THE ATM_Admin_API SHALL respond with HTTP 409 identifying `terminal_id` as the conflicting field.
5. WHEN `priority_class` is provided but is not one of `VIP`, `Non VIP`, `Industri`, THE ATM_Admin_API SHALL respond with HTTP 400 (or 422 field validation).
6. WHEN any monetary amount is provided, THE ATM_Admin_API SHALL parse it as an exact decimal (`numeric(20,2)`), rejecting non-numeric or negative values with HTTP 422; amounts SHALL never be stored via a lossy float path.
7. WHEN creation succeeds, THE ATM_Admin_API SHALL respond with HTTP 201 and the created ATM in the Flat_JSON_Shape.
8. WHEN creation succeeds, THE ATM_Admin_API SHALL write an `audit_logs` entry with action `atm_created`, entity_type `atm`, entity_id = new ATM id, `before` null, `after` = the created record, actor = the acting admin, and the actor IP.
9. WHEN any validation fails, THE ATM_Admin_API SHALL respond with HTTP 400/422 (matching the backend's existing `writeError`/`writeValidationError` convention) and SHALL NOT create the ATM or write an audit entry.

### Requirement 4: Edit ATM

**User Story:** As an administrator, I want to edit an existing ATM's details, so that I can correct machine attributes, thresholds, or location.

#### Acceptance Criteria

1. THE ATM_Admin_API SHALL expose `PUT /api/v1/admin/atms/{id}` accepting the editable fields: `location_id`, `machine_type`, `brand`, `model`, `operation_hours`, `deployment_type`, `capacity_amount`, `low_threshold_amount`, `critical_threshold_amount`, `priority_class`, `escrow_account`, `blacklisted`.
2. THE ATM_Admin_API SHALL NOT allow editing `terminal_id` via this endpoint (the terminal id is the stable machine identifier); a request attempting to change it SHALL be rejected with HTTP 400.
3. WHEN the target `id` does not exist, THE ATM_Admin_API SHALL respond with HTTP 404.
4. WHEN `location_id` is changed to a value that does not reference an existing location, THE ATM_Admin_API SHALL respond with HTTP 400 (`invalid_reference`).
5. WHEN `priority_class` or a monetary amount is invalid, THE ATM_Admin_API SHALL respond per Requirements 3.5 / 3.6.
6. WHEN the update succeeds, THE ATM_Admin_API SHALL respond with HTTP 200 and the updated record.
7. WHEN the update succeeds, THE ATM_Admin_API SHALL write an `audit_logs` entry with action `atm_updated`, `before` = the record prior to the change and `after` = the record after, actor, and IP.

### Requirement 5: Disable and Enable ATM

**User Story:** As an administrator, I want to disable a decommissioned ATM and re-enable it if needed, without erasing its history or cash records.

#### Acceptance Criteria

1. THE ATM_Admin_API SHALL expose `POST /api/v1/admin/atms/{id}/disable` performing Soft_Disable (`is_active=false`, `deleted_at=now()`).
2. THE ATM_Admin_API SHALL expose `POST /api/v1/admin/atms/{id}/enable` performing Enable (`is_active=true`, `deleted_at=NULL`).
3. THE ATM_Admin_API SHALL NOT expose any hard-`DELETE` route for ATMs; child rows in `atm_denoms` / `atm_vendor_packages` and historical cash/replenishment data referencing the ATM SHALL remain intact (soft-disable never triggers the CASCADE).
4. WHEN disable targets a non-existent ATM, THE ATM_Admin_API SHALL respond with HTTP 404.
5. WHEN disable succeeds, THE ATM_Admin_API SHALL respond with HTTP 200 and write an `audit_logs` entry with action `atm_deactivated`, actor, and IP.
6. WHEN enable succeeds, THE ATM_Admin_API SHALL respond with HTTP 200 and write an `audit_logs` entry with action `atm_reactivated`, actor, and IP.

### Requirement 6: Location Options (read-only)

**User Story:** As an administrator, I want to pick a location from a list when creating or editing an ATM, so that I set a valid `location_id` without memorizing ids.

#### Acceptance Criteria

1. THE ATM_Admin_API SHALL expose `GET /api/v1/admin/atms/locations` returning the id + display name (and optionally city/province) of `locations` for populating the Location select.
2. THE endpoint SHALL return locations ordered by `name` ascending in the Flat_JSON_Shape.
3. Reads (list/get/locations) SHALL NOT write audit entries.
4. IF the location set is large enough that a full list is impractical, THEN the endpoint MAY accept a `q` substring filter on location `name`; at current volumes a full list is acceptable.

### Requirement 7: Audit Guarantee

**User Story:** As an auditor, I want every add/edit/disable/enable action on ATMs recorded, so that ATM master-data changes are fully traceable.

#### Acceptance Criteria

1. THE ATM_Admin_API SHALL write exactly one `audit_logs` entry per successful state-changing action (create, update, disable, enable) via the Audit_Writer.
2. THE audit entry SHALL record actor id (the acting admin), action, entity_type (`atm`), entity_id, `before`/`after` payloads where applicable, and the actor IP (extracted via the existing `extractClientIP` helper).
3. IF the audit write fails, THEN the action SHALL surface an error (matching the existing services' behavior where a failed audit write returns an error), so that no silent unaudited mutation occurs.
4. Read (list/get/locations) operations SHALL NOT write audit entries.

### Requirement 8: ATM Management Screen

**User Story:** As an administrator, I want a clear screen to view and manage ATMs, so that I can perform add/edit/disable from one place.

#### Acceptance Criteria

1. THE Admin_ATMs_Page SHALL render a `PageHeader` titled "Manajemen ATM" with a subtitle and a primary "Tambah ATM" action button.
2. THE Admin_ATMs_Page SHALL render a `DataTable` with columns: Terminal ID, Lokasi (location_name), Tipe Mesin, Brand, Deployment, Prioritas (priority_class as Badge), Status (active/disabled, as Badge with icon + label), and an Aksi (actions) column.
3. THE Status Badge SHALL render both an icon and a text label — never signalled by color alone.
4. THE Admin_ATMs_Page SHALL provide filter controls mapping to the list endpoint: a search input (`q` for terminal_id), and `FilterSelect`s for Brand, Tipe Mesin, Deployment, Prioritas, and Status (Aktif / Nonaktif / Semua).
5. WHEN the administrator changes any filter, THE Admin_ATMs_Page SHALL reset to page 1 and refetch with the updated parameters, reflecting active filters in the URL search parameters.
6. WHEN the administrator clicks "Tambah ATM", THE Admin_ATMs_Page SHALL open the ATM_Form_Dialog in create mode.
7. WHEN the administrator triggers Edit on a row, THE Admin_ATMs_Page SHALL open the ATM_Form_Dialog pre-filled with that ATM's current values in edit mode.
8. WHEN the administrator triggers Disable/Enable on a row, THE Admin_ATMs_Page SHALL show a confirmation prompt and, on confirm, call the corresponding endpoint and refetch the list.
9. WHEN no ATMs match the current filters, THE Admin_ATMs_Page SHALL display an `EmptyState`.
10. THE Admin_ATMs_Page SHALL display a loading skeleton while the list query is pending and an inline error with a retry control if it fails.
11. WHEN a mutation (create/update/disable/enable) succeeds or fails, THE Admin_ATMs_Page SHALL show a `Toast` reflecting the outcome and, on success, invalidate the list query.

### Requirement 9: ATM Form (Add / Edit)

**User Story:** As an administrator, I want a guided form for creating and editing ATMs, so that I enter valid data and set a valid location.

#### Acceptance Criteria

1. THE ATM_Form_Dialog SHALL be built with React Hook Form + Zod (project tech standard), validating required fields client-side before submit.
2. THE ATM_Form_Dialog SHALL present fields: Terminal ID, Lokasi (select, from Requirement 6), Tipe Mesin, Brand, Model, Jam Operasi (operation_hours), Deployment Type, Kapasitas (capacity_amount), Threshold Rendah (low_threshold_amount), Threshold Kritis (critical_threshold_amount), Prioritas (priority_class select: VIP / Non VIP / Industri), Escrow Account, Blacklisted (boolean).
3. IN edit mode, THE ATM_Form_Dialog SHALL disable Terminal ID (immutable per Requirement 4.2).
4. THE monetary fields SHALL accept and display exact decimal input, formatted for IDR with `tabular-nums`, and submit as strings/decimals (never lossy float).
5. WHEN the server returns a field-level validation (422), bad-reference (400), or conflict (409) error, THE ATM_Form_Dialog SHALL surface the message against the relevant field (e.g. 409 → Terminal ID, invalid_reference → Lokasi) and SHALL NOT close.
6. WHEN submission succeeds, THE ATM_Form_Dialog SHALL close and the list SHALL refetch.
7. THE ATM_Form_Dialog SHALL be keyboard-accessible via the shared `Dialog`: focus moves into the dialog on open, is trapped while open, and returns to the trigger on close; Escape closes the dialog.

### Requirement 10: Responsive Layout and Accessibility

**User Story:** As a user on varied screen sizes, I want the page usable and accessible, so that I can manage records effectively.

#### Acceptance Criteria

1. THE page SHALL ensure all interactive elements (buttons, filter controls, table row actions, dialog controls) have a minimum touch target of 44x44 CSS pixels.
2. THE page SHALL use semantic HTML landmarks: `main` for page content and a `table` within the DataTable; the dialog SHALL use `role="dialog"` with an accessible label and `aria-modal`.
3. THE DataTable SHALL scroll horizontally on narrow viewports rather than truncating columns.
4. THE page SHALL render all Badges (Status, Prioritas) with icon plus label text, never signalling meaning by color alone (design-system hard rule).
5. THE page SHALL use the design system's OKLCH "Merah Sirih" (internal) tokens exclusively — no hardcoded hex or RGB, one primary action per view.
6. THE page SHALL announce dynamic changes (dialog open, toast) via an `aria-live` region.
7. THE amount and id columns SHALL use `tabular-nums` for alignment; amounts SHALL be right-aligned per the design system's money rule.

### Requirement 11: Feature Module Organization and Backend Layout

**User Story:** As a developer, I want the feature organized to the project's conventions, so that it stays maintainable and matches the codebase.

#### Acceptance Criteria

1. THE frontend feature code SHALL reside in `src/features/admin-atms/` (feature-based organization), mirroring the `admin-users` / `admin-vendors` layout from the sibling spec.
2. THE feature SHALL define TypeScript types for all API request/response shapes in a dedicated types file, TanStack Query hooks (list/get/locations + mutations) in a dedicated hooks file, and feature-specific components (page, filter bar, table, form dialog) in a components directory.
3. THE feature SHALL reuse existing shared components (DataTable, PageHeader, Badge, FilterSelect, EmptyState, Card, Button, Toast) and the `Dialog` primitive introduced by `admin-user-vendor-management`, without duplicating them.
4. THE backend code SHALL follow the ATM backend's actual technical-layer layout: `internal/handler` (new `admin_atm_handler.go`), `internal/service` (new `atm_admin.go`), `internal/repository` (new `atm_admin_repository.go`) + sqlc `backend/queries/atms_admin.sql`, returning the Flat_JSON_Shape. It SHALL NOT modify the existing `atm_portal_*` read stack.
5. THE new backend write paths SHALL use `dbPool` (primary) for writes and reads-after-write, with a documented `ponytail:` replica TODO for the list/read/locations endpoints matching the convention used elsewhere in `cmd/api/main.go`.
6. THE feature SHALL extend the existing `no_hard_delete_test.go` guarantee to cover ATMs (no hard `DELETE` on `atms`).
