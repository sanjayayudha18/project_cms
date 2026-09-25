# Design Document: Admin ATM Management

## Overview

This feature adds admin CRUD-with-soft-disable for the **`atms`** master table on the ATM backend (`backend/`, port 8080) and the internal app (`frontend/CompanyPortal-Vite`). It is the third leg of the admin master-data set, following the sibling spec `admin-user-vendor-management` (users + vendors). It reuses that spec's decisions, layering, flat-JSON contract, audit path, and frontend primitives; it does not touch the user/vendor stacks or the existing read-only `atm-portal` monitoring viewer.

It is **backend-first**: define queries → repository → service → handler → route wiring, verify against the flat-JSON contract, then build the screen.

The design deliberately reuses what already exists:

- **Audit** goes through the shared `audit.NewWriter(dbPool)` `Writer`, exactly as the user/vendor admin services do.
- **Flat JSON** responses use the existing `writeJSON`/`writeError`/`writeValidationError`/`writeForbidden`/`extractClientIP`/`parsePathID` helpers in `internal/handler/error_response.go`.
- **Money-as-decimal-string** matches the existing `atm_portal_handler.go` convention (numeric never rendered through a lossy float path).
- **Frontend** reuses `protectedRoute` + `requireRoles`, `DataTable`, `PageHeader`, `Badge`, `FilterSelect`, `Toast`, the `api` client, the auth store, and the `Dialog` primitive that `admin-user-vendor-management` adds to `src/components/ui/`.

> **Golden-rule gates (project-context Sec 3 rule 7).** This spec touches **master data** but NOT auth or money math (ATM rows carry no credentials; the amount fields are static config thresholds, not transactional postings). It is therefore lower-risk than the user stack. The inherited decisions apply: (1) apply-immediately-with-audit, no maker-checker; (2) soft-disable, no hard delete. The **role guard is confirmed (2026-09-15): `ADMIN`/`ADMIN_PARAM`**, and the screen is surfaced as a card in the Settings hub (see "Menu placement" below).

## Architecture

### Request flow (create ATM, representative)

```
CompanyPortal-Vite  ──POST /api/v1/admin/atms──▶  RequireAuth → RequireRoles(ADMIN, ADMIN_PARAM)
  ATM_Form_Dialog (RHF+Zod)                          │
  useCreateATM (TanStack Query mutation)             ▼
                                              AdminATMHandler.Create
                                                    │  decode body, parse amounts as decimal, extractClientIP
                                                    ▼
                                              ATMAdminService.Create
                                                    │  validate required fields, priority_class enum,
                                                    │  amounts >= 0; resolve+check location_id exists;
                                                    │  terminal_id uniqueness pre-check
                                                    ├──▶ ATMAdminRepository.Create  (primary pool)
                                                    └──▶ auditWriter.Write(Entry{action:"atm_created", before:nil, after:<record>})
                                                    ▼
                                              201 + created ATM (flat JSON)
```

Update, disable, enable are structurally identical (`ATMAdminService.Update/Disable/Enable`).

### Layer responsibilities

- **Handler** (`internal/handler/admin_atm_handler.go`): HTTP only — decode body, parse path/query params, extract actor id from `middleware.GetAuthContext`, extract IP via `extractClientIP`, map service errors → status codes, write flat JSON. No business logic.
- **Service** (`internal/service/atm_admin.go`, beside the existing `atm_portal.go`): validation (required fields, `priority_class` enum, monetary parse + non-negative), reference resolution (`location_id` existence), uniqueness pre-check (`terminal_id`), ordering the repository write and the audit write. Owns the "no unaudited mutation" guarantee.
- **Repository** (`internal/repository/atm_admin_repository.go`): wraps `*db.Queries`; converts nullable `pgtype.*` ↔ pointers at the boundary (existing `timestamptzToPtr` / numeric-handling convention from `atm_portal` reads). Uses `dbPool` (primary) with a `ponytail:` replica TODO on the list/read queries.
- **DB** (`internal/db` + `backend/queries/atms_admin.sql`): sqlc-generated (or hand-written to match, see sqlc note).

### Data model (no new tables)

All columns already exist in `atms` (from `002`, `030`). **No migration is needed.** Optionally, an additive trigram index on `atms(terminal_id)` mirrors the sibling spec's optional `030_admin_search_indexes.sql`, but at current volumes `ILIKE '%q%'` on `terminal_id` is fine — deferrable, and any schema change is an approve-first item.

**`atms`** (from `002`, `030`): `id, terminal_id (uniq), location_id→locations (NOT NULL), machine_type, brand, model, operation_hours, deployment_type (all NOT NULL text), capacity_amount, low_threshold_amount, critical_threshold_amount (numeric(20,2), nullable), blacklisted (bool default false), is_active (bool default true), escrow_account (text, 030), priority_class (text 'VIP'|'Non VIP'|'Industri', 030), created_at, updated_at, deleted_at`.

**`locations`** (from `002`): read-only here — `id, name, city_or_regency, province, …`. Used to validate `location_id` and to populate the Location select (Requirement 6).

**Child tables** (`atm_denoms`, `atm_vendor_packages`) both FK → `atms` with `ON DELETE CASCADE`. Because disable is a soft `UPDATE` (never a `DELETE`), CASCADE never fires; child rows survive for disabled ATMs. Out of scope: this feature manages the `atms` row only.

## Backend components

### sqlc queries — `backend/queries/atms_admin.sql`

New query file, kept separate from the atm-portal read queries. Representative set:

- `ListATMsAdmin` — filters `q` (`ILIKE` on `terminal_id` via `sqlc.narg`), `brand`, `machine_type`, `deployment_type`, `priority_class`, `location_id`, status (via `deleted_at IS NULL` / `IS NOT NULL` toggled by a `sqlc.narg('status')` mapping), `LIMIT`/`OFFSET`; LEFT JOIN `locations` to return `location_name`. Order `terminal_id ASC, id ASC`.
- `CountATMsAdmin` — same filters, no limit.
- `GetATMAdminByID` — full row incl. `location_name`.
- `CreateATMAdmin` — INSERT returning the row.
- `UpdateATMAdmin` — UPDATE editable columns by id, `updated_at = now()`, returning the row.
- `DisableATM` (`is_active=false, deleted_at=now()` WHERE id AND deleted_at IS NULL) / `EnableATM` (`is_active=true, deleted_at=NULL` WHERE id).
- `FindATMByTerminalID` (incl. soft-deleted) for the 409 pre-check.
- `ListLocationsForSelect` — `id, name, city_or_regency, province` ordered by `name` (Requirement 6).
- `LocationExists` — existence check for `location_id` references.

> **sqlc-generate blocker (established precedent).** `sqlc generate` is blocked by a pre-existing bug in migration `017` (missing table name) and a local sqlc version that rewrites unrelated files with different casing (`IP`→`Ip`). Follow the audit-log-viewer / admin-user-vendor-management precedent: author the `.sql`, run generate, keep **only** the new `atms_admin.sql.go` and revert unrelated drift, hand-fixing casing to the checked-in convention. If generate cannot run at all, hand-write `internal/db/atms_admin.sql.go` matching sqlc's exact output style. A follow-up to pin the sqlc version and fix `017` is already tracked by the sibling spec.

### Repository — `internal/repository/atm_admin_repository.go`

Wrap `*db.Queries` (pattern from `auth_repository.go` / the sibling spec's `user_admin_repository.go`). Convert nullable timestamps via `timestamptzToPtr`; convert `numeric(20,2)` columns to decimal strings / `*string` at the boundary the same way `atm_portal` reads do (never through `float64` for storage). Constructed with the primary `dbPool`; list/read/locations methods carry a `ponytail:` TODO to swap to the replica pool once wired.

### Service — `internal/service/atm_admin.go`

Define a narrow interface (mirroring the sibling spec's `UserAdminService`/`VendorAdminService`) so handler tests fake it without a DB.

`ATMAdminService`:
- `Create(ctx, actorID, req CreateATMRequest, actorIP) (ATM, error)` — validate required text fields present; `priority_class` (when present) ∈ {`VIP`,`Non VIP`,`Industri`}; each monetary amount parses as a non-negative exact decimal; `LocationExists(location_id)` (400 `invalid_reference` if not); `terminal_id` uniqueness pre-check (409 if taken, incl. soft-deleted); insert; then `auditWriter.Write(Entry{Action:"atm_created", EntityType:"atm", EntityID:new.ID, Before:nil, After:new})`.
- `Update(ctx, actorID, id, req UpdateATMRequest, actorIP) (ATM, error)` — load existing (404 if absent); reject `terminal_id` change (400); location/enum/amount checks as above; update; audit `atm_updated` with before/after.
- `Disable(ctx, actorID, id, actorIP) error` — 404 if absent; soft-disable; audit `atm_deactivated`.
- `Enable(ctx, actorID, id, actorIP) error` — 404 if absent; enable; audit `atm_reactivated`.
- `ListLocations(ctx) ([]LocationOption, error)` — read-through for the select (no audit).

The audit `before`/`after` payload is the ATM record struct (no sensitive material to sanitize, unlike users). Money fields serialize as decimal strings in the audit payload too.

### Handler — `internal/handler/admin_atm_handler.go` (new)

`AdminATMHandler.Routes()`:
```
r.Get("/", h.List)                       // GET  /api/v1/admin/atms
r.Post("/", h.Create)                    // POST /api/v1/admin/atms
r.Get("/locations", h.ListLocations)     // GET  /api/v1/admin/atms/locations
r.Get("/{id}", h.Get)                    // GET  /api/v1/admin/atms/{id}
r.Put("/{id}", h.Update)                 // PUT  /api/v1/admin/atms/{id}
r.Post("/{id}/disable", h.Disable)       // POST .../disable
r.Post("/{id}/enable", h.Enable)         // POST .../enable
```
> Route ordering note: register the static `/locations` before the `/{id}` param route so chi resolves it correctly (same pattern main.go already documents for `/users/hierarchy` vs `/users/{id}/hierarchy`).

Actor id from `middleware.GetAuthContext(r.Context())`; IP from `extractClientIP(r)`; path id via `parsePathID(r,"id")`. Flat-JSON DTOs mirror `atm_portal_handler.go`'s `atmPortalRow` style (amounts as `*string`/decimal, timestamps as RFC3339 pointers).

### Error → HTTP mapping (flat JSON)

| Condition | Status | Helper |
|---|---|---|
| Missing/invalid token | 401 | `writeUnauthorized` |
| Role not permitted | 403 | (middleware) / `writeForbidden` |
| Field validation (required/format/enum/amount) | 422 | `writeValidationError` |
| Bad query/path param, immutable `terminal_id` change, bad status enum | 400 | `writeError(...,"bad_request",...)` |
| Missing referenced `location_id` | 400 | `writeError(...,"invalid_reference",...)` |
| Not found (id) | 404 | `writeError(...,"not_found",...)` |
| Unique conflict (`terminal_id`) | 409 | `writeError(...,"conflict",...)` |
| DB/audit failure | 500 | `writeError(...,"internal_error",...)` |

409 detection: prefer an explicit `FindATMByTerminalID` pre-check (clear per-field message), falling back to mapping the Postgres unique-violation (`pgconn.PgError` code `23505`, constraint `atms_terminal_id_uq`) to `terminal_id` to stay correct under races.

### Route wiring — `cmd/api/main.go`

Role guard defaults to `ADMIN`/`ADMIN_PARAM` (confirm the OPEN DECISION first):
```go
atmAdminRepo := repository.NewATMAdminRepository(dbPool)
atmAdminSvc  := service.NewATMAdminService(atmAdminRepo, auditWriter)
adminATMHandler := handler.NewAdminATMHandler(atmAdminSvc)
r.With(custommw.RequireAuth(tokenService), custommw.RequireRoles("ADMIN","ADMIN_PARAM")).
    Mount("/api/v1/admin/atms", adminATMHandler.Routes())
```

## Frontend components

### Structure

```
src/features/admin-atms/
  types.ts            // AdminATM, ListParams, ListResponse, CreateATMPayload, UpdateATMPayload, LocationOption
  api.ts              // thin wrappers over lib/api/client
  hooks.ts            // useATMsList, useATM, useLocationOptions, useCreateATM, useUpdateATM, useDisableATM, useEnableATM
  components/
    AdminATMsPage.tsx
    ATMFilterBar.tsx
    ATMsTable.tsx
    ATMFormDialog.tsx
src/routes/
  admin.atms.tsx      // requireRoles(["ADMIN","ADMIN_PARAM"]) under protectedRoute
```

Reuses `src/components/ui/Dialog.tsx` from the sibling spec (no new UI primitive here). If `admin-atms` lands before `admin-user-vendor-management`, the `Dialog` primitive becomes a shared dependency to build first; the tasks note this ordering.

### Routing & RBAC

Register `admin.atms.tsx` under `protectedRoute` with `beforeLoad: requireRoles(["ADMIN","ADMIN_PARAM"])`, exactly like `audit-logs.tsx` / the sibling spec's `admin.vendors.tsx`. `requireRoles` returns `{ forbidden: true }` for the component to render the shared 403 state.

### Forms

React Hook Form + Zod (tech.md). The Zod schema encodes: required text fields; `priority_class` enum; monetary fields as decimal-string inputs validated non-negative (parsed with a decimal-safe helper, formatted IDR with `tabular-nums`, submitted as strings). Terminal ID disabled in edit mode. Server 422/409/400-invalid_reference map onto RHF field errors via `setError` (409 → Terminal ID, invalid_reference → Lokasi) so the dialog shows the message inline and stays open. Location select is populated from `useLocationOptions()`.

### Data & state

TanStack Query v5: list hooks use `placeholderData: keepPreviousData` for smooth pagination; the location-options query uses a long `staleTime` (master data); mutations invalidate the list key on success and drive a `Toast`. Query keys namespaced (`adminATMsKeys.list(params)`, `.detail(id)`, `.locations()`). Filters live in URL search params and any filter change resets `page` to 1.

### Design tokens

"Merah Sirih" internal theme only. One primary action per view (the "Tambah ATM" button). Status and Prioritas as `Badge` with icon + label — never color alone. Amount and id columns `tabular-nums`, amounts right-aligned. No side-stripe borders, no gradient text (design-system Sec 10 bans).

## Menu placement — inside the Settings hub (confirmed 2026-09-15)

Surface the ATM screen as a **card in the Settings hub** (`/settings`, `src/features/rbac-settings/components/SettingsHubPage.tsx`), NOT as a standalone top-level `NAV_CONFIG` sidebar item — matching the sibling spec's "Manajemen Pengguna"/"Manajemen Vendor" placement. Add a "Manajemen ATM" card (`href: /admin/atms`, Lucide icon e.g. `Landmark`/`Server`) to the hub's card list.

**Access: `ADMIN` and `ADMIN_PARAM`.** The `/settings` route is guarded by `requireRoles(["ADMIN","ADMIN_PARAM","APPACCESS"])` and `admin.atms.tsx` by `requireRoles(["ADMIN","ADMIN_PARAM"])`; both roles reach the hub and the card target. No top-level `NAV_CONFIG` entry is added — the hub is the single entry point.

## Testing strategy

- **Repository (integration, real Postgres, `//go:build integration`)**: create→get roundtrip; list filter matrix (`q`, brand, machine_type, deployment_type, priority_class, location_id, status); pagination count vs summed pages; `terminal_id` uniqueness violation surfaces; disable/enable toggles `is_active`+`deleted_at`; `LocationExists`; `ListLocationsForSelect` ordering. Gated like `audit_log_repository_test.go` (may be unrunnable if `DATABASE_URL` host is unreachable — note it).
- **Service (table-driven, faked repo + faked audit writer)**: validation matrix (required fields, `priority_class` enum, amount non-negative/decimal, immutable `terminal_id`); `location_id` reference resolution → 400; `terminal_id` conflict → 409; 404 on missing id; **audit-write-called-exactly-once per success**; **audit failure ⇒ error surfaced, no silent mutation**.
- **Handler (httptest, real `RequireAuth`/`RequireRoles`)**: 200/201 happy paths + flat-JSON shape (amounts as strings); 400/404/409/422 mapping; 401 no token, 403 wrong role (pattern from `admin_approval_handler_test.go`); `/locations` returns options; static-vs-param route ordering works.
- **`no_hard_delete_test.go`**: extend to assert no `DELETE FROM atms` path exists (grep-style guard as done for users; the sibling spec extends it to vendors — this adds atms).
- **Frontend**: `requireRoles` allow/deny/redirect (pattern from `audit-logs.test.ts`); Zod schema (required + enum + amount + immutable terminal_id); form maps server 409/422/invalid_reference to field errors; filter change resets to page 1 and round-trips through URL; table renders Status + Prioritas badges with icon+label; money columns `tabular-nums`; Dialog focus trap + Escape close. RTL + Vitest; coverage ≥ 80% on the feature module.

## Definition of Done (project-context Sec 11)

- [ ] Matches module/table map — no new tables (optional additive index migration only, approve-first); `atms` is canonical
- [ ] Correct auth path + scoped RBAC at middleware AND route guard (role per confirmed OPEN DECISION; default `ADMIN`/`ADMIN_PARAM`)
- [ ] Audit written for every create/update/disable/enable; no maker-checker (inherited Decision 1)
- [ ] Reads on `dbPool` with documented replica TODO; writes on primary; no reads on replica in read-after-write flows
- [ ] Money fields as `numeric`/decimal-string, never float; timestamps timestamptz UTC, displayed Asia/Jakarta
- [ ] No hard delete for ATMs (guard test extended to `atms`)
- [ ] Existing `atm-portal` read stack untouched
- [ ] Tests green incl. RBAC denial (401/403), validation, uniqueness (409), invalid_reference (400), audit-once
- [ ] No secrets/config hardcoded; `.env.example` unchanged (no new config)
- [ ] ATM backend keeps flat JSON shape
- [ ] Builds clean (`go build ./...`, `sqlc generate` or hand-written equivalent, `pnpm build`)
