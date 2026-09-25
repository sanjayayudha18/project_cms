# Requirements Document

## Introduction

The Audit Log Viewer is an admin-facing capability in the CMS internal app that surfaces the platform-wide `audit_logs` trail — the append-only record of every state-changing action (who, what, before/after, when, IP) mandated by the project's non-negotiable audit rule (project-context Sec 4). It has two parts:

1. A **read API** on the ATM backend (`GET /api/v1/audit-logs` + `GET /api/v1/audit-logs/{id}`) that queries the generic `audit_logs` table with filters and pagination, returning the ATM backend's existing flat JSON shape for wire compatibility.
2. A **single-page admin screen** at `/audit-logs` in `frontend/CompanyPortal-Vite/` that lists audit entries in a filterable, paginated table and shows a before/after detail view per entry.

This is a **read-only** viewer. It does not create, edit, or delete audit entries — the append-only `audit_logs` table is written only by the `internal/audit` writer (owned by RBAC-Setup Task 3). Access is restricted to `ADMIN` and `ADMIN_PARAM` roles.

## Dependency & Scope Note

- This spec **depends on** the generic `audit_logs` table and the `internal/audit` writer defined in `.kiro/specs/RBAC-Setup/task.md` Task 3. As of this writing, only migration `021_users_hierarchy.sql` exists; `audit_logs` (migration `023`) is **not yet created**. The Audit Log Viewer's backend task list therefore either follows RBAC-Setup Task 3 or creates the `audit_logs` table itself if RBAC-Setup Task 3 is not yet done (see tasks.md — coordinate to avoid a duplicate migration).
- This viewer is distinct from the **EOD retry audit** (`retry_audit_logs`, migration `012`) surfaced by the `eod-monitoring-frontend` spec. That is a domain-specific retry trail on the Python EOD service. This spec targets the **generic platform `audit_logs`** written by CMS modules (approval transitions, master-data changes, DSR/invoice actions).
- Scope is the viewer only: reading, filtering, paginating, and displaying existing audit entries. Wiring individual modules to *write* audit entries is out of scope here (that is per-module work).

## Glossary

- **Audit_Log_Page**: The single route at `/audit-logs` within CompanyPortal-Vite, restricted to `ADMIN` and `ADMIN_PARAM`.
- **Audit_Log_Table**: A TanStack Table v8 instance listing audit entries with pagination and sorting.
- **Audit_Detail_Drawer**: A side panel that opens on row click, showing the full entry including before/after JSON diff.
- **Audit_Filter_Bar**: The set of filter controls above the table: actor, action, entity type, date range.
- **Audit_Log_Entry**: One row of the `audit_logs` table — `id, actor_id, action, entity_type, entity_id, before jsonb, after jsonb, ip, created_at`.
- **Audit_Read_API**: The ATM backend endpoints `GET /api/v1/audit-logs` (list) and `GET /api/v1/audit-logs/{id}` (detail).
- **Flat_JSON_Shape**: The ATM backend's existing response convention (flat JSON, not the `pkg/response` envelope) used for wire compatibility with the internal frontend.
- **API_Client**: The existing `api` client in `lib/api/client.ts` that injects the Bearer token and handles 401 refresh.
- **Badge_Component**: The existing shared `Badge` supporting variants `success | warning | danger | info | neutral`.
- **Cursor_Or_Offset_Page**: The pagination model returned by the list endpoint (page number + page size + total, or a next-cursor token).
- **TanStack_Query**: TanStack Query v5, used for server state, caching, and pagination.

## Requirements

### Requirement 1: Route and Access Control

**User Story:** As a system administrator, I want the Audit Log page restricted to admin roles, so that the platform audit trail is not exposed to unauthorized users.

#### Acceptance Criteria

1. THE Audit_Log_Page SHALL be registered as a file-based route under the protected route layout at the path `/audit-logs`.
2. THE Audit_Log_Page SHALL use the `requireRoles` guard with allowed roles `ADMIN` and `ADMIN_PARAM` in the route's `beforeLoad`.
3. WHEN a user with a role other than `ADMIN` or `ADMIN_PARAM` attempts to access `/audit-logs`, THE Audit_Log_Page SHALL display a Forbidden state and SHALL NOT render audit data.
4. WHEN an unauthenticated user attempts to access `/audit-logs`, THE Audit_Log_Page SHALL redirect to `/login` with the original path preserved in the redirect search parameter.
5. THE Audit_Read_API SHALL be mounted behind `RequireAuth` and `RequireRoles("ADMIN", "ADMIN_PARAM")` middleware in `cmd/api/main.go`.
6. WHEN the Audit_Read_API receives a request without a valid token, THE Audit_Read_API SHALL respond with HTTP 401.
7. WHEN the Audit_Read_API receives a request from a role other than `ADMIN` or `ADMIN_PARAM`, THE Audit_Read_API SHALL respond with HTTP 403.

### Requirement 2: List Endpoint — Filtering and Pagination

**User Story:** As a compliance officer, I want to filter and page through audit entries on the server, so that I can find specific actions without loading the entire table.

#### Acceptance Criteria

1. THE Audit_Read_API SHALL expose `GET /api/v1/audit-logs` accepting optional query parameters: `actor_id`, `action`, `entity_type`, `entity_id`, `date_from`, `date_to`, `page`, `page_size`.
2. WHEN `actor_id` is provided, THE Audit_Read_API SHALL return only entries whose `actor_id` matches.
3. WHEN `action` is provided, THE Audit_Read_API SHALL return only entries whose `action` matches exactly.
4. WHEN `entity_type` is provided, THE Audit_Read_API SHALL return only entries whose `entity_type` matches exactly.
5. WHEN `entity_type` AND `entity_id` are both provided, THE Audit_Read_API SHALL return only entries matching both — the full history of one entity.
6. WHEN `date_from` and/or `date_to` are provided, THE Audit_Read_API SHALL filter entries by `created_at` within the inclusive range, interpreting the boundaries as Asia/Jakarta local time converted to UTC.
7. THE Audit_Read_API SHALL default `page` to 1 and `page_size` to 25 when not provided, and SHALL cap `page_size` at 100.
8. THE Audit_Read_API SHALL return entries ordered by `created_at` descending (newest first), with `id` descending as a stable tiebreaker.
9. THE Audit_Read_API SHALL return, in the Flat_JSON_Shape, the page of entries plus pagination metadata: `page`, `page_size`, `total`.
10. WHEN an invalid parameter is supplied (non-numeric `page`/`page_size`, malformed date, `date_from` after `date_to`), THE Audit_Read_API SHALL respond with HTTP 400 and a descriptive error.
11. THE Audit_Read_API SHALL execute list queries against the read path (the current single `dbPool`, with a documented TODO to route to the replica once wired, per project-context Sec 5).

### Requirement 3: Detail Endpoint

**User Story:** As a compliance officer, I want to open a single audit entry and see its full before/after payload, so that I can verify exactly what changed.

#### Acceptance Criteria

1. THE Audit_Read_API SHALL expose `GET /api/v1/audit-logs/{id}` returning the full Audit_Log_Entry including `before` and `after` JSON.
2. WHEN the requested `id` does not exist, THE Audit_Read_API SHALL respond with HTTP 404 and a descriptive error.
3. WHEN the requested `id` is not a valid integer, THE Audit_Read_API SHALL respond with HTTP 400.
4. THE Audit_Read_API SHALL return `before` and `after` as JSON objects (or null) in the Flat_JSON_Shape, preserving the stored payload without transformation.

### Requirement 4: Read-Only Guarantee

**User Story:** As an auditor, I want assurance that the viewer cannot alter the trail, so that the audit record remains trustworthy.

#### Acceptance Criteria

1. THE Audit_Read_API SHALL expose only read (`GET`) operations — no create, update, or delete routes.
2. THE Audit_Read_API SHALL NOT write to `audit_logs` for read operations (viewing the audit trail is not itself an audited action in this spec).
3. THE `audit_logs` table SHALL remain append-only as defined by the `internal/audit` writer; this spec SHALL NOT add update or delete paths to it.

### Requirement 5: Page Layout and Table

**User Story:** As an operations support engineer, I want a clear table of recent audit activity, so that I can scan what happened across the platform.

#### Acceptance Criteria

1. THE Audit_Log_Page SHALL render a PageHeader with the title "Audit Log" and a subtitle describing the page purpose.
2. THE Audit_Log_Table SHALL render a DataTable with columns: Waktu (Timestamp), Aktor (Actor), Aksi (Action, as Badge), Tipe Entitas (Entity Type), ID Entitas (Entity ID), and IP.
3. THE Audit_Log_Table SHALL render the Waktu column formatted in Asia/Jakarta timezone with format `DD MMM YYYY HH:mm:ss`.
4. THE Audit_Log_Table SHALL map the `action` value to a Badge_Component variant using a documented action-to-variant map, and SHALL render each Badge with both an icon and a text label — never color alone.
5. THE Audit_Log_Table SHALL render pagination controls (previous/next and current page indicator) driven by the list endpoint's pagination metadata.
6. WHEN no entries match the current filters, THE Audit_Log_Table SHALL display an EmptyState with the message "Tidak ada log audit".
7. THE Audit_Log_Table SHALL display a loading skeleton while the list query is in pending state.
8. WHEN a user clicks a row, THE Audit_Log_Page SHALL open the Audit_Detail_Drawer for that entry.

### Requirement 6: Filter Bar

**User Story:** As a compliance officer, I want filter controls that map to the server-side filters, so that I can narrow the trail to a person, action, entity, or time window.

#### Acceptance Criteria

1. THE Audit_Filter_Bar SHALL provide controls for: Action (FilterSelect), Entity Type (FilterSelect), Actor (text or select input mapping to `actor_id`), and a Date Range (date_from / date_to).
2. WHEN the user changes any filter, THE Audit_Log_Page SHALL reset to page 1 and refetch the list with the updated filter parameters.
3. THE Audit_Filter_Bar SHALL provide a "Reset" control that clears all filters and returns to the default unfiltered, page-1 view.
4. THE Audit_Filter_Bar SHALL reflect the active filters in the page URL search parameters, so that a filtered view is shareable and survives refresh.
5. WHEN the Date Range is set with `date_from` after `date_to`, THE Audit_Filter_Bar SHALL surface a validation message and SHALL NOT issue the request.

### Requirement 7: Detail Drawer with Before/After Diff

**User Story:** As a compliance officer, I want to see the before and after state of an audited change side by side, so that I can understand precisely what was modified.

#### Acceptance Criteria

1. WHEN the Audit_Detail_Drawer opens, THE Audit_Detail_Drawer SHALL fetch the full entry from `GET /api/v1/audit-logs/{id}` using TanStack_Query.
2. THE Audit_Detail_Drawer SHALL display the entry metadata: actor, action (as Badge), entity type, entity id, IP, and timestamp (Asia/Jakarta, `DD MMM YYYY HH:mm:ss`).
3. THE Audit_Detail_Drawer SHALL render the `before` and `after` payloads as formatted, readable JSON.
4. WHEN both `before` and `after` are present, THE Audit_Detail_Drawer SHALL visually indicate which top-level fields changed (added, removed, or modified).
5. WHEN `before` is null (a create action), THE Audit_Detail_Drawer SHALL show only the `after` payload labeled as the created state.
6. WHEN `after` is null (a delete action), THE Audit_Detail_Drawer SHALL show only the `before` payload labeled as the deleted state.
7. THE Audit_Detail_Drawer SHALL slide in from the right with a 300ms ease-out transition animating `transform` and `opacity`, and close on Escape or outside click at 75% of the enter duration.
8. THE Audit_Detail_Drawer SHALL trap keyboard focus while open and return focus to the triggering row when closed.
9. THE Audit_Detail_Drawer SHALL display a loading skeleton while the detail query is pending.

### Requirement 8: Loading and Error States

**User Story:** As an operations support engineer, I want clear feedback during loading and on failure, so that I can distinguish "no data" from "system error".

#### Acceptance Criteria

1. WHILE the list query is pending on initial load, THE Audit_Log_Page SHALL render skeleton placeholders matching the table layout.
2. IF the list query fails, THEN THE Audit_Log_Page SHALL display an inline error containing the error message from the API response, with a "Coba Lagi" (Retry) button that refetches.
3. IF the detail query fails, THEN THE Audit_Detail_Drawer SHALL display an inline error with a retry control, without closing the drawer.
4. WHEN a 401 occurs during any request, THE API_Client SHALL handle token refresh transparently per the existing pattern.

### Requirement 9: Responsive Layout and Accessibility

**User Story:** As a user on varied screen sizes, I want the page usable and accessible, so that I can review audit data effectively.

#### Acceptance Criteria

1. THE Audit_Log_Page SHALL ensure all interactive elements (buttons, filter controls, table rows, drawer close) have a minimum touch target of 44x44 CSS pixels.
2. THE Audit_Log_Page SHALL use semantic HTML landmarks: `main` for page content, `section` with accessible labels, and `table` within the DataTable.
3. THE Audit_Log_Table SHALL render the table with horizontal scroll on narrow viewports rather than truncating columns.
4. THE Audit_Log_Page SHALL render all Badges with icon plus label text, never signalling meaning by color alone.
5. THE Audit_Log_Page SHALL use the design system's OKLCH color tokens ("Merah Sirih", internal theme) exclusively — no hardcoded hex or RGB.
6. THE Audit_Log_Page SHALL announce dynamic changes (drawer open, toast) via an `aria-live` region.
7. THE ID Entitas and IP columns SHALL use `tabular-nums` where they render numeric or address-like values for column alignment.

### Requirement 10: Feature Module Organization

**User Story:** As a developer, I want the viewer organized to the project's conventions, so that it stays maintainable.

#### Acceptance Criteria

1. THE frontend feature code SHALL reside in `src/features/audit-log/` following the project's feature-based organization.
2. THE feature SHALL define TypeScript types for all API response shapes in a dedicated types file.
3. THE feature SHALL define TanStack Query hooks (list + detail) in a dedicated hooks file.
4. THE feature SHALL reuse existing shared components (Badge, DataTable, PageHeader, FilterSelect, EmptyState, Card, Button, Skeleton, Toast) without duplicating them.
5. THE feature SHALL define feature-specific components (Audit_Detail_Drawer, Audit_Filter_Bar, before/after diff renderer) within the feature module's components directory.
6. THE backend code SHALL follow the ATM backend's actual technical-layer layout: `internal/handler` (audit_log_handler), `internal/service` (audit read service), `internal/repository` + sqlc `queries/*.sql`, returning the Flat_JSON_Shape.
