# Design Document: Admin User & Vendor Management

## Overview

This feature adds admin CRUD-with-soft-disable for two master entities — **users** and **vendors** — on the ATM backend (`backend/`, port 8080) and the internal app (`frontend/CompanyPortal-Vite`). It is **backend-first**: define migrations/queries → repository → service → handler → route wiring, verify against the flat-JSON contract, then build the two screens.

The design deliberately reuses what already exists rather than inventing parallel machinery:

- **Users disable/enable** is already implemented as `internal/auth.DeactivateUserService` (audited soft-delete/reactivate) — this feature **wires it into routes** and adds the missing **create** and **update** operations.
- **Vendors** have no code yet; this feature adds the full repository/service/handler stack for vendor create/update/disable/enable, mirroring the user pattern.
- **Audit** goes through the shared `audit.NewWriter(dbPool)` `Writer`, exactly as `DeactivateUserService`/`SetInitialPasswordService` do today.
- **Flat JSON** responses use the existing `writeJSON`/`writeError`/`writeValidationError`/`writeForbidden`/`extractClientIP` helpers in `internal/handler/error_response.go`.
- **Frontend** reuses `protectedRoute` + `requireRoles`, `DataTable`, `PageHeader`, `Badge`, `FilterSelect`, `Toast`, the `api` client, and the auth store; adds one new accessible dialog primitive (none exists yet).

> **Golden-rule gates (project-context Sec 3 rule 7).** This spec touches **auth-adjacent data** (users, roles, vendor linkage) and **master data**. Two decisions are surfaced in requirements.md "Open Decisions" and MUST be confirmed before the corresponding tasks: (1) which roles guard each screen, (2) whether add/edit/disable route through maker-checker. The default position — user mgmt under `APPACCESS`, vendor mgmt under `ADMIN`/`ADMIN_PARAM`, apply-immediately-with-audit (no maker-checker) matching the existing account-provisioning actions — is what the design below implements. A maker-checker variant is sketched in "Alternative: maker-checker" so the pivot is cheap if the team requires it.

## Architecture

### Request flow (create user, representative)

```
CompanyPortal-Vite  ──POST /api/v1/admin/users──▶  RequireAuth → RequireRoles(APPACCESS)
  User_Form_Dialog (RHF+Zod)                          │
  useCreateUser (TanStack Query mutation)             ▼
                                              AdminUserHandler.Create
                                                    │  parse+decode body, extractClientIP
                                                    ▼
                                              UserAdminService.Create
                                                    │  validate, resolve role_id, uniqueness pre-checks
                                                    │  (no password on create — Open Decision 3)
                                                    ├──▶ UserAdminRepository.Create  (primary pool)
                                                    └──▶ auditWriter.Write(Entry{action:"user_created", before:nil, after:<record>})
                                                    ▼
                                              201 + created user (flat JSON, no password_hash)
```

Disable/enable reuse the existing `DeactivateUserService` unchanged. Vendor flow is structurally identical with `VendorAdminService`/`VendorAdminRepository` and `ADMIN`/`ADMIN_PARAM` guard.

### Layer responsibilities

- **Handler** (`internal/handler`): HTTP only — decode body, parse path/query params, extract actor id from `middleware.GetAuthContext`, extract IP via `extractClientIP`, map service errors → status codes, write flat JSON. No business logic.
- **Service** (`internal/auth` for user writes to sit beside the existing `DeactivateUserService`/`SetInitialPasswordService`; `internal/service` for vendor writes): validation, uniqueness/reference resolution, ordering the repository write and the audit write. Owns the "no unaudited mutation" guarantee.
- **Repository** (`internal/repository`): wraps `*db.Queries`; converts nullable `pgtype.*` ↔ pointers at the boundary (existing `timestamptzToPtr` convention). Uses `dbPool` (primary) with a `ponytail:`-style replica TODO on the list queries.
- **DB** (`internal/db` + `backend/queries/*.sql`): sqlc-generated (or hand-written to match, see sqlc note).

### Data model (no new tables)

All columns already exist. **No migration is needed for schema** — only, optionally, additive indexes to support case-insensitive search and the status filter.

**`users`** (from `002`, `016`, `021`, `026`): `id, role_id→roles, employee_id (uniq, nullable), username (uniq), full_name, email (uniq), is_karyawan, auth_source ('ldap'|'local'), password_hash (local only), vendor_id→vendors (nullable), vendor_branch_id→vendor_branches (nullable), is_active, last_login_at, supervisor_id (self-FK, CHECK ≠ id), approval_level, password_changed_at, must_change_password, failed_login_attempts, locked_until, created_at, updated_at, deleted_at`.

**`vendors`** (from `002`): `id, code (uniq), name, contact_email, contact_phone, hq_address, is_active, created_at, updated_at, deleted_at`.

**`roles`** (from `002`,`027`): `id, role (uniq), description`. Read-only here (populate the Role select).

#### Optional migration `030_admin_search_indexes.sql` (additive, needs approval)

To keep `q` search off sequential scans as the tables grow, add trigram indexes (the `pg_trgm` extension is already listed in tech.md). Additive, `IF NOT EXISTS`, `BEGIN;…COMMIT;` with a WHY/SAFETY header, matching `014`/`029` style:

```
CREATE INDEX IF NOT EXISTS users_fullname_trgm_idx  ON public.users  USING gin (full_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS users_username_trgm_idx  ON public.users  USING gin (username  gin_trgm_ops);
CREATE INDEX IF NOT EXISTS users_email_trgm_idx     ON public.users  USING gin (email     gin_trgm_ops);
CREATE INDEX IF NOT EXISTS vendors_code_trgm_idx    ON public.vendors USING gin (code gin_trgm_ops);
CREATE INDEX IF NOT EXISTS vendors_name_trgm_idx    ON public.vendors USING gin (name gin_trgm_ops);
```

At current data volumes (hundreds of users, dozens of vendors) `ILIKE '%q%'` seq scans are fine, so this migration is **optional / deferrable**. If deferred, search still works. Because any schema change is a project-context STOP-and-confirm item, treat `030` as approve-before-apply; the feature does not otherwise require it. `pg_trgm` must be confirmed installed (`CREATE EXTENSION IF NOT EXISTS pg_trgm;` — itself a privileged, confirm-first op).

## Backend components

### sqlc queries — `backend/queries/users_admin.sql`, `backend/queries/vendors_admin.sql`

New query files (kept separate from `auth.sql` to avoid churn on the auth queries). Representative set:

Users:
- `ListUsersAdmin` — filters `q` (ILIKE across username/full_name/email via `sqlc.narg`), `role`, `vendor_id`, status (via `deleted_at IS NULL` / `IS NOT NULL` toggled by a `sqlc.narg('status')` mapped in the query), `LIMIT`/`OFFSET`; joins `roles` to return the role text. Order `full_name ASC, id ASC`.
- `CountUsersAdmin` — same filters, no limit.
- `GetUserAdminByID` — full row incl. role text, excluding password_hash from the projection.
- `CreateUserAdmin` — INSERT returning the row (no `password_hash` set).
- `UpdateUserAdmin` — UPDATE editable columns by id, `updated_at = now()`, returning the row.
- (disable/enable reuse existing `DeactivateUser`/`ReactivateUser`.)
- `FindUserByEmail` / `FindUserByEmployeeID` (incl. soft-deleted) for 409 pre-checks — or rely on the unique-constraint violation mapping (see error mapping).

Vendors:
- `ListVendorsAdmin`, `CountVendorsAdmin`, `GetVendorAdminByID`, `CreateVendorAdmin`, `UpdateVendorAdmin`, `DisableVendor` (`is_active=false, deleted_at=now()`), `EnableVendor` (`is_active=true, deleted_at=NULL`), `CountActiveUsersByVendor` (for Requirement 8.5 warning).

> **sqlc-generate blocker (steering + audit-log-viewer precedent).** `sqlc generate` is blocked by a pre-existing bug in migration `017` (missing table name) and a local sqlc version (v1.31.1) that rewrites unrelated files with different casing (`IP`→`Ip`). Follow the audit-log-viewer precedent: author the `.sql`, run generate, keep **only** the new `*.sql.go` files and revert unrelated drift, hand-fixing casing to match the checked-in convention. If generate cannot run at all, hand-write `internal/db/users_admin.sql.go` / `vendors_admin.sql.go` matching sqlc's exact output style (as `audit.sql.go`/`approval.sql.go` were). A follow-up to pin the sqlc version (`tools.go`/CI) and fix `017` is noted in tasks.

### Repository — `internal/repository/user_admin_repository.go`, `vendor_admin_repository.go`

Wrap `*db.Queries` (pattern from `auth_repository.go`). Return `db.*` row structs where the service already depends on `internal/db` (matching the audit-log-viewer decision to avoid duplicate domain structs), converting nullable timestamps via `timestamptzToPtr`. Never project `password_hash` into any list/get result. Constructed with the primary `dbPool`; list methods carry a `ponytail:` TODO to swap to the replica pool once wired.

### Service — `internal/auth/user_admin.go`, `internal/service/vendor_admin.go`

Define narrow interfaces (mirroring the `SetInitialPasswordService`/`ApprovalOrchestrator` narrow-interface pattern) so handler tests fake them without a DB.

`UserAdminService`:
- `Create(ctx, actorID, req CreateUserRequest, actorIP) (User, error)` — validate required fields + email format (`go-playground/validator` per tech.md, or explicit checks consistent with existing `ValidatePasswordStrength` style); resolve `role`→`role_id`; enforce auth_source rules (local ⇒ vendor_id required, no password; ldap ⇒ vendor_id absent, no password); reference-check vendor/supervisor; insert; then `auditWriter.Write(Entry{Action:"user_created", EntityType:"user", EntityID:new.ID, Before:nil, After:sanitize(new)})`.
- `Update(ctx, actorID, id, req UpdateUserRequest, actorIP) (User, error)` — load existing (404 if absent); reject username/auth_source changes; self-supervision guard; reference checks; update; audit `user_updated` with before/after (sanitized).
- Disable/enable are **not** re-implemented — the handler calls the existing `DeactivateUserService`.

`VendorAdminService`: `Create`, `Update`, `Disable`, `Enable`, each audited (`vendor_created`/`vendor_updated`/`vendor_deactivated`/`vendor_reactivated`). `Disable` also calls `CountActiveUsersByVendor` and returns a `LinkedUsersWarning` count so the handler can include it.

Sanitization rule: the audit `after`/`before` payload for users is a struct that **excludes** `password_hash` (Requirement 9.3). Marshalled to JSON by the `Writer`.

### Handler — `internal/handler/admin_user_handler.go` (extend), `admin_vendor_handler.go` (new)

Extend `AdminUserHandler.Routes()`:
```
r.Get("/", h.List)                       // GET  /api/v1/admin/users
r.Post("/", h.Create)                    // POST /api/v1/admin/users
r.Get("/{id}", h.Get)                    // GET  /api/v1/admin/users/{id}
r.Put("/{id}", h.Update)                 // PUT  /api/v1/admin/users/{id}
r.Post("/{id}/disable", h.Disable)       // POST .../disable
r.Post("/{id}/enable", h.Enable)         // POST .../enable
r.Post("/{id}/set-initial-password", h.SetInitialPassword)  // existing
```
`AdminVendorHandler.Routes()` mirrors this under `/api/v1/admin/vendors` (no set-initial-password). Actor id from `middleware.GetAuthContext(r.Context())`; IP from `extractClientIP(r)`; path id via `parsePathID(r,"id")` (existing helper).

Self-lockout guard (Requirement 5.7): in `Disable`, if `targetID == authCtx.UserID` → `writeError(w, 400, "self_disable_forbidden", ...)`.

### Error → HTTP mapping (flat JSON)

| Condition | Status | Helper |
|---|---|---|
| Missing/invalid token | 401 | `writeUnauthorized` |
| Role not permitted | 403 | (middleware) / `writeForbidden` |
| Field validation (required/format) | 422 | `writeValidationError` |
| Bad query/path param, immutable-field change, self-supervision, bad status enum | 400 | `writeError(...,"bad_request",...)` |
| Unknown role / missing referenced vendor/supervisor | 400 | `writeError(...,"invalid_reference",...)` |
| Not found (id) | 404 | `writeError(...,"not_found",...)` |
| Unique conflict (username/email/employee_id/code) | 409 | `writeError(...,"conflict",...)` |
| DB/audit failure | 500 | `writeError(...,"internal_error",...)` |

409 detection: prefer explicit pre-check queries (clear per-field message) OR map the Postgres unique-violation (`pgconn.PgError` code `23505`, matching on constraint name `users_username_key`/`users_email_key`/`users_employee_id_key`/`vendors_code_key`) to the offending field. Design chooses **pre-check for the create form's UX** (returns which field), falling back to constraint mapping to stay correct under races.

### Route wiring — `cmd/api/main.go`

```go
// users: extend existing APPACCESS group with the write/read admin service
userAdminRepo := repository.NewUserAdminRepository(dbPool)
userAdminSvc  := auth.NewUserAdminService(userAdminRepo, roleRepo, auditWriter)
deactivateSvc := auth.NewDeactivateUserService(userRepo, auditWriter) // now actually wired
adminUserHandler := handler.NewAdminUserHandler(setInitialPasswordService, userAdminSvc, deactivateSvc)
r.With(custommw.RequireAuth(tokenService), custommw.RequireRoles("APPACCESS")).
    Mount("/api/v1/admin/users", adminUserHandler.Routes())

// vendors: new ADMIN/ADMIN_PARAM group
vendorAdminRepo := repository.NewVendorAdminRepository(dbPool)
vendorAdminSvc  := service.NewVendorAdminService(vendorAdminRepo, auditWriter)
adminVendorHandler := handler.NewAdminVendorHandler(vendorAdminSvc)
r.With(custommw.RequireAuth(tokenService), custommw.RequireRoles("ADMIN","ADMIN_PARAM")).
    Mount("/api/v1/admin/vendors", adminVendorHandler.Routes())
```
(Exact role sets pending Open Decision 1.)

## Frontend components

### Structure

```
src/features/admin-users/
  types.ts            // AdminUser, ListParams, ListResponse, CreateUserPayload, UpdateUserPayload
  api.ts              // thin wrappers over lib/api/client
  hooks.ts            // useUsersList, useUser, useCreateUser, useUpdateUser, useDisableUser, useEnableUser
  components/
    AdminUsersPage.tsx
    UserFilterBar.tsx
    UsersTable.tsx
    UserFormDialog.tsx
src/features/admin-vendors/
  (parallel: types/api/hooks + AdminVendorsPage, VendorFilterBar, VendorsTable, VendorFormDialog)
src/components/ui/
  Dialog.tsx          // NEW accessible modal primitive (focus trap, Escape, aria-modal, aria-live)
src/routes/
  admin.users.tsx     // requireRoles([...])  under protectedRoute
  admin.vendors.tsx   // requireRoles(["ADMIN","ADMIN_PARAM"])
```

### Routing & RBAC

Register both routes under `protectedRoute` with `beforeLoad: requireRoles([...])`, exactly like `audit-logs.tsx`. If `APPACCESS` guards `/admin/users`, add `"APPACCESS"` to the `DbRole` union in `lib/auth/store.ts` (Requirement 1.11). `requireRoles` already returns `{ forbidden: true }` for the component to render the shared 403 state.

### Dialog primitive

No modal/drawer exists in `components/ui/`. Add a minimal `Dialog` (native `<dialog>` or a portal + `role="dialog"`/`aria-modal="true"`): focus moves in on open, focus trap while open, Escape + outside-click close, focus returns to trigger, `aria-live` region for announcements. Transition per design-system Sec 9: `transform`+`opacity` only, ~200ms ease-out enter, exit ~75% of enter. Both form dialogs render inside it.

### Forms

React Hook Form + Zod (tech.md). The Zod schema encodes the auth_source conditional (local ⇒ vendor_id required) as a `superRefine`. Server 422/409 map onto RHF field errors via `setError` so the dialog shows the message inline and stays open. Follow the concrete RHF pattern already used in `features/vendor-request/VendorRequestCreate.tsx` for consistency.

### Data & state

TanStack Query v5: list hooks use `placeholderData: keepPreviousData` for smooth pagination; mutations invalidate the list key on success and drive a `Toast`. Query keys namespaced (`adminUsersKeys.list(params)`, `.detail(id)`). Filters live in URL search params (shareable, survive refresh), and any filter change resets `page` to 1.

### Design tokens

"Merah Sirih" internal theme only. One primary action per view (the "Tambah …" button). Status as `Badge` with icon + label (`success` = Aktif, `neutral`/`danger` = Nonaktif) — never color alone. Amount/id/phone columns `tabular-nums`. No side-stripe borders, no gradient text (design-system Sec 10 bans).

## Alternative: maker-checker (if Open Decision 2 requires it)

If the team rules that master-data add/edit/disable must be two-person-approved, do **not** build a second state machine. Instead, per project-context Sec 2 "Approval integration pattern":

1. Add `approval_policies` rows for `document_type` `user` and `vendor` (a level threshold; amount is 0/N-A for master data — confirm the policy shape works for non-monetary documents, since `SubmitForApproval` takes an `amount`).
2. The service's `Create`/`Update`/`Disable` calls `orch.SubmitForApproval(ctx, makerID, "user"|"vendor", entityID, 0, ip)` and stores the *intended* change as a pending payload, applying the actual mutation only after the request reaches `approved` (never optimistically).
3. UI gains a "pending approval" status and the change waits for a checker (maker ≠ checker).

This is materially more work (pending-change storage, apply-on-approve worker/poll) and is why the default is apply-immediately-with-audit. Flagged for explicit decision.

## Testing strategy

- **Repository (integration, real Postgres, `//go:build integration`)**: create→get roundtrip; list filter matrix (`q`, role, vendor_id, status); pagination count vs summed pages; uniqueness violation surfaces; disable/enable toggles `is_active`+`deleted_at`; `CountActiveUsersByVendor`. Gated like `audit_log_repository_test.go` (may be unrunnable if `DATABASE_URL` host is unreachable — note it, as prior specs did).
- **Service (table-driven, faked repo + faked audit writer)**: validation matrix (required fields, email format, auth_source conditional, self-supervision, immutable fields); role/reference resolution; **audit-write-called-exactly-once per success**; **audit failure ⇒ error surfaced, no silent mutation**; password material never in audit payload.
- **Handler (httptest, real `RequireAuth`/`RequireRoles`)**: 200/201 happy paths + flat-JSON shape (no `password_hash`); 400/404/409/422 mapping; 401 no token, 403 wrong role (pattern from `admin_approval_handler_test.go`); self-disable 400.
- **`no_hard_delete_test.go`**: extend to assert no `DELETE FROM vendors` path exists (grep-style guard as done for users).
- **Frontend**: `requireRoles` allow/deny/redirect (pattern from `audit-logs.test.ts`); Zod schema (auth_source conditional); form maps server 409/422 to field errors; filter change resets to page 1 and round-trips through URL; table renders status badge with icon+label; Dialog focus trap + Escape close. RTL + Vitest; coverage ≥ 80% on the feature modules.

## Definition of Done (project-context Sec 11)

- [ ] Matches module/table map — no new tables (optional additive index migration only, approve-first); users/vendors are canonical
- [ ] Correct auth path + scoped RBAC at middleware AND route guard (roles per confirmed Open Decision 1)
- [ ] Audit written for every create/update/disable/enable; maker-checker decision (Open Decision 2) explicitly resolved and reflected
- [ ] Reads on `dbPool` with documented replica TODO; writes on primary; no reads on replica in read-after-write flows
- [ ] No `password_hash` in any response or audit payload; timestamps timestamptz UTC, displayed Asia/Jakarta
- [ ] No hard delete for users or vendors (guard test extended to vendors)
- [ ] Tests green incl. RBAC denial (401/403), validation, uniqueness (409), audit-once, self-disable guard
- [ ] No secrets/config hardcoded; `.env.example` unchanged (no new config)
- [ ] ATM backend keeps flat JSON shape
- [ ] Builds clean (`go build ./...`, `sqlc generate` or hand-written equivalent, `pnpm build`)
