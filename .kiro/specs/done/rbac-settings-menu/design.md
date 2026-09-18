# Design Document

## Overview

This feature turns the currently dead "Pengaturan" nav link into a working Settings hub for RBAC management in the internal Company Portal, and fills the read-side gap on the ATM backend. Today the backend exposes only write endpoints for hierarchy, delegations, and leaves (`AdminApprovalHandler`), guarded by `RequireRoles("ADMIN","ADMIN_PARAM")`; there is no way to *list* RBAC state and no UI.

The design:

- Adds a Settings hub landing page at `/settings` linking to four RBAC sub-pages (`/settings/rbac/users`, `/settings/rbac/delegations`, `/settings/rbac/leaves`, `/settings/rbac/policies`), all route-gated to the Authorized_Roles.
- Adds a new `src/features/rbac-settings` feature module mirroring the existing `audit-log` feature layout (types, hooks with TanStack Query, components, api client via `@/lib/api/client`).
- Adds GET/list endpoints on the ATM backend reading from the **replica** pool, returning flat JSON, alongside the existing write handler. Policy create/edit endpoints are added (see decision below).
- Widens the backend `RequireRoles` guard on `/api/v1/admin/approval` and the new list routes to include `APPACCESS`, and updates the frontend nav item + route guards to `ADMIN`, `ADMIN_PARAM`, `APPACCESS`.
- Preserves `audit_logs` writes on every mutation (already present on the write handler; added to the new policy create/edit path).

No new tables are introduced. RBAC changes stay direct administrative actions with no maker-checker gate (Requirement 11).

### Requirements mapping

| Requirement | Covered by |
| --- | --- |
| 1 — Settings hub + sub-pages, nav + route gating | Frontend: `SettingsHubPage`, route tree under `src/routes/settings/`, `navigation.ts` roles + `filterNavByRoles`, `requireRoles` guard |
| 2 — List endpoints from replica | Backend: `RbacListHandler` (GET users/delegations/leaves/policies) on replica pool; frontend list hooks + tables, loading/error states |
| 3 — Widen auth to APPACCESS | Backend: widen `RequireRoles` in `main.go`; frontend: nav roles + route guards |
| 4 — Create/edit hierarchy | Existing `PUT /users/{id}/hierarchy`; frontend hierarchy form (RHF+Zod) |
| 5 — Create/revoke delegations (409) | Existing `POST /delegations` (409), `DELETE /delegations/{id}`; frontend delegation form + conflict handling |
| 6 — Create leaves | Existing `POST /leaves`; frontend leave form |
| 7 — Manage policies | Backend: `GET /policies` + `POST /policies` + `PUT /policies/{id}` (decision below); frontend policy table + form, tabular-nums |
| 8 — Audit every mutation | Existing audit writes on hierarchy/delegation/leave; new audit writes on policy create/edit |
| 9 — View role assignments | Users list returns `role`; frontend role badge |
| 10 — UI/theme conformance | Merah Sirih tokens, Bahasa Indonesia, tabular-nums, soft red focus ring, icon+text status |
| 11 — Scope exclusions | No maker-checker on RBAC config, no payment, no new roles |

### Decision: approval-policy create/edit endpoints

Requirement 7 treats create/edit as conditional ("WHERE a create or edit endpoint ... is available"). We **add** a minimal `POST /policies` and `PUT /policies/{id}` because the cost is low (the pattern mirrors the existing delegation/leave create handlers, the table already exists, and `AdminStore`/audit wiring is already in place). This satisfies Requirement 7.3 fully rather than shipping list-only. Delete is out of scope (not requested).

## Architecture

```
Company Portal (React 19 / Vite / TanStack Router+Query)
  src/routes/settings/                route tree, requireRoles gating
  src/features/rbac-settings/         feature module (types, hooks, components, api)
        │ fetch (flat JSON)  via @/lib/api/client
        ▼
ATM Backend (Go + Chi, port 8080)
  /api/v1/admin/approval   (RequireAuth + RequireRoles ADMIN, ADMIN_PARAM, APPACCESS)
    ├─ writes  → AdminApprovalHandler (existing)      → Primary_Pool + Audit_Writer
    └─ reads   → RbacListHandler       (new)          → Replica_Pool
        ▼
PostgreSQL   Primary (writes/txn reads)   Replica (list/read)
Tables (existing): users, roles, approval_delegations, user_leaves, approval_policies
```

### Frontend route tree

TanStack Router file-based routes under `src/routes/`, each attaching to `protectedRoute` and using `requireRoles(["ADMIN","ADMIN_PARAM","APPACCESS"])` (same helper as `audit-logs.tsx`):

```
src/routes/
  settings.tsx                     → path "/settings"                → SettingsHubPage
  settings/rbac/users.tsx          → path "/settings/rbac/users"     → RbacUsersPage
  settings/rbac/delegations.tsx    → path "/settings/rbac/delegations" → RbacDelegationsPage
  settings/rbac/leaves.tsx         → path "/settings/rbac/leaves"    → RbacLeavesPage
  settings/rbac/policies.tsx       → path "/settings/rbac/policies"  → RbacPoliciesPage
```

Each route registers in the router tree the same way `auditLogsRoute` does. Route-level `beforeLoad: requireRoles([...])` satisfies Requirement 1.3 (blocks non-authorized roles from `/settings/*`).

### Feature module layout (`src/features/rbac-settings/`)

Mirrors `audit-log`:

```
src/features/rbac-settings/
  index.ts                    # barrel exports (pages, hooks, types)
  types.ts                    # entity types, Zod schemas, badge variant maps
  api.ts                      # api client fns (get/post/put via @/lib/api/client)
  hooks/
    useRbacQueries.ts         # useQuery list hooks + useMutation write hooks, rbacKeys
  components/
    SettingsHubPage.tsx       # landing hub with cards/links to sub-pages
    RbacUsersPage.tsx         # users+hierarchy table + inline hierarchy form
    RbacDelegationsPage.tsx   # delegations table + create form + revoke action
    RbacLeavesPage.tsx        # leaves table + create form
    RbacPoliciesPage.tsx      # policies table + create/edit form
    RoleBadge.tsx             # role badge (label + icon)
    StatusMessage.tsx         # shared loading/error/empty (icon + text)
  __tests__/                  # component tests mirroring audit-log/__tests__
```

Query keys follow the audit-log convention:

```ts
export const rbacKeys = {
  all: ["rbac-settings"] as const,
  users: () => [...rbacKeys.all, "users"] as const,
  delegations: () => [...rbacKeys.all, "delegations"] as const,
  leaves: () => [...rbacKeys.all, "leaves"] as const,
  policies: () => [...rbacKeys.all, "policies"] as const,
};
```

Mutations invalidate the relevant key on success so the table refreshes (Requirements 4.4, 5.5, 6.3).

### Navigation update

In `src/lib/config/navigation.ts`, widen the `settings` item roles from `["ADMIN"]` to `["ADMIN","ADMIN_PARAM","APPACCESS"]` (Requirement 1.2). `filterNavByRoles` already returns all items for `ADMIN`/`ADMIN_PARAM`; add an early check (or rely on the explicit role list) so `APPACCESS` also sees the item, and non-authorized roles have it omitted (Requirement 1.4). The `settings` item already points at `/settings`.

### Backend handler placement

Add a sibling `RbacListHandler` (new file `backend/internal/handler/rbac_list_handler.go`) rather than overloading `AdminApprovalHandler`, keeping read (replica) and write (primary) stores cleanly separated. Both handlers mount under `/api/v1/admin/approval`:

- `AdminApprovalHandler.Routes()` — existing writes (primary + audit).
- `RbacListHandler.Routes()` — new GETs (replica) + policy create/edit (primary + audit).

Policy create/edit is a write, so it lives on a store backed by the primary pool with audit; it is grouped with the RBAC list/policy handler for cohesion (all policy operations in one place). The mount in `main.go` composes both routers under the same prefix and role guard.

## Backend design

### `RequireRoles` widening (`backend/cmd/api/main.go`)

Change the admin approval mount to include `APPACCESS` and mount the new list handler under the same guard:

```go
adminApprovalHandler := handler.NewAdminApprovalHandler(approvalRepo, auditWriter)

// New: read repository on the replica pool; policy write store on primary.
rbacReadRepo := repository.NewRbacReadRepository(dbReadPool) // replica
rbacPolicyStore := repository.NewApprovalPolicyStore(dbPool)  // primary
rbacListHandler := handler.NewRbacListHandler(rbacReadRepo, rbacPolicyStore, auditWriter)

r.With(
    custommw.RequireAuth(tokenService),
    custommw.RequireRoles("ADMIN", "ADMIN_PARAM", "APPACCESS"),
).Route("/api/v1/admin/approval", func(ar chi.Router) {
    ar.Mount("/", adminApprovalHandler.Routes())
    ar.Mount("/", rbacListHandler.Routes())
})
```

> Note: the audit-log handler's `main.go` comment shows `dbRead`/replica wiring is pending ("ponytail: swap dbPool for the dbRead pool"). The read repository is written to take a replica pool; if the replica pool wiring is not yet live in this codebase, pass the primary pool at construction with the same pending-swap comment, so the repository API does not change when the replica lands. This keeps Requirement 2.5 satisfied at the repository seam.

### New sqlc queries (`backend/internal/db`)

Add read queries (SELECT, replica-safe) and policy write queries. Regenerate with `sqlc generate`.

- `ListUserHierarchy` → `SELECT id, supervisor_id, approval_level, role, auth_source FROM users ORDER BY id` (Requirement 2.1, 9.1).
- `ListDelegations` → `SELECT id, from_user_id, to_user_id, start_at, end_at, reason FROM approval_delegations WHERE revoked_at IS NULL ORDER BY start_at DESC` (Requirement 2.2).
- `ListLeaves` → `SELECT id, user_id, start_at, end_at, reason FROM user_leaves ORDER BY start_at DESC` (Requirement 2.3).
- `ListApprovalPolicies` → `SELECT id, document_type, min_amount, max_amount, required_level FROM approval_policies ORDER BY document_type, min_amount` (Requirement 2.4).
- `CreateApprovalPolicy` / `UpdateApprovalPolicy` → insert/update on primary (Requirement 7.3, 8.4).

> Project context notes `sqlc generate` is currently blocked by a pre-existing bug in migration 017. If regeneration remains blocked, hand-write the query functions in `internal/db` matching sqlc's output convention (as was done for `audit`/`approval`), and regenerate once 017 is fixed. This is a known constraint, not new scope.

### API contract (new endpoints)

All under `/api/v1/admin/approval`, guarded by `RequireAuth` + `RequireRoles("ADMIN","ADMIN_PARAM","APPACCESS")`. Flat JSON (no envelope), consistent with the existing ATM handler. Unauthenticated → 401 (Requirement 3.3); wrong role → 403 (Requirement 2.8).

| Method | Path | Pool | Response (200) |
| --- | --- | --- | --- |
| GET | `/users/hierarchy` | Replica | `{ "users": [ { "id", "supervisor_id"?, "approval_level"?, "role", "auth_source" } ] }` |
| GET | `/delegations` | Replica | `{ "delegations": [ { "id", "from_user_id", "to_user_id", "start_at", "end_at", "reason"? } ] }` (RFC3339 timestamps) |
| GET | `/leaves` | Replica | `{ "leaves": [ { "id", "user_id", "start_at", "end_at", "reason"? } ] }` |
| GET | `/policies` | Replica | `{ "policies": [ { "id", "document_type", "min_amount", "max_amount", "required_level" } ] }` |
| POST | `/policies` | Primary + audit | `201 { "id", "document_type", "min_amount", "max_amount", "required_level" }` |
| PUT | `/policies/{id}` | Primary + audit | `200 { ...updated policy }` |

Existing write endpoints (unchanged, listed for completeness): `PUT /users/{id}/hierarchy`, `POST /delegations` (409 on overlap), `DELETE /delegations/{id}`, `POST /leaves`.

Amounts (`min_amount`, `max_amount`) are serialized as numeric strings to preserve exact decimal precision (money is `numeric`, never float — project golden rule). The frontend renders them right-aligned with tabular figures.

Error shape reuses the existing `writeError` helper (`{ "error": { "code", "message" } }` style already used by `AdminApprovalHandler`), so the frontend `ApiError` handling from the audit-log feature applies unchanged.

### Policy create/edit request bodies

```json
// POST /policies
{ "document_type": "invoice", "min_amount": "0", "max_amount": "100000000", "required_level": 2 }
// PUT /policies/{id}  (same fields)
```

Validation: `document_type` non-empty; `min_amount` <= `max_amount`; `required_level` positive integer. On success, write `audit_logs` (`admin_create_policy` / `admin_update_policy`) with actor, before/after, ip (Requirement 8.4).

## Data model

No new tables or columns. Reuses `users`, `roles`, `approval_delegations`, `user_leaves`, `approval_policies`. New artifacts are read queries and policy write queries only (listed above). No migration required beyond confirming the existing `approval_policies` columns match (`document_type`, `min_amount`/`max_amount`, `required_level`).

## Sequence flows

### Create delegation (with 409 handling)

```
Admin → RbacDelegationsPage: fill form (from, to, start_at, end_at, reason)
Form (RHF+Zod) validates → useCreateDelegation.mutate(payload)
  → api.post("/admin/approval/delegations", payload)
    → RequireAuth (401 if no session) → RequireRoles (403 if not authorized)
    → AdminApprovalHandler.CreateDelegation → store.CreateDelegation (Primary)
       ├─ overlap? → approval.ErrDelegationOverlap → handleError → 409
       └─ ok → audit.Write(admin_create_delegation) → 201 delegationResponse
onSuccess → invalidate rbacKeys.delegations() → table refetches (5.5)
onError(409) → StatusMessage conflict: icon + "Rentang delegasi tumpang tindih..." (5.6)
```

### Edit hierarchy

```
Admin → RbacUsersPage: edit supervisor_id + approval_level inline (RHF+Zod)
  self-supervisor blocked client-side (mirror server guard) AND server-side
  → useSetHierarchy.mutate({id, supervisor_id, approval_level})
  → api.put("/admin/approval/users/{id}/hierarchy", body)
    → SetHierarchy: supervisor_id == id → 400 (4.2); else store.SetUserHierarchy (Primary)
    → audit.Write(admin_set_hierarchy) → 200 {id, supervisor_id, approval_level}
onSuccess → invalidate rbacKeys.users() → list shows updated values (4.4)
onError(400) → inline validation error (icon+text), retain entered values (4.5)
```

## Frontend component design

- **SettingsHubPage** — landing hub. A small set of link cards (not an identical repeated grid — vary layout per Merah Sirih ban #5): "Hierarki Pengguna", "Delegasi Persetujuan", "Cuti Pengguna", "Kebijakan Persetujuan", each routing to its sub-page (Requirement 1.1, 1.5). Icons from Lucide (`Users`, `GitBranch`, `CalendarOff`, `Scale`).
- **Per-entity table pages** — TanStack-styled table (plain semantic table matching audit-log): uppercase `--n-500` headers on `--n-50`, `--n-100` row dividers, row hover `--red-50`. Numeric columns (`approval_level`, amounts) right-aligned with `tabular-nums` (Requirements 7.2, 10.3). Status/role via badges (icon + label), never color alone (Requirements 9.2, 10.5).
- **Forms (RHF + Zod)** — delegation, leave, hierarchy, policy forms. Zod schemas in `types.ts`. RFC3339 datetime inputs for `start_at`/`end_at` (delegation/leave). Focus ring is the soft red halo `box-shadow: 0 0 0 3px var(--red-100)`, border `--red-400` (Requirement 10.4). One primary (red-500) button per form; secondary/ghost otherwise.
- **StatusMessage** — shared loading spinner, error banner (`AlertTriangle` icon + Bahasa Indonesia text), and empty state, so every list satisfies Requirements 2.10, 2.11, 5.6, 6.4, 10.5.
- **RoleBadge** — maps `role` → `{ label, icon, variant }` using the semantic badge tokens (info/neutral), no brand red (Requirement 9.2).

All copy in Bahasa Indonesia (Requirement 10.1); Merah Sirih tokens only, red as <=10% accent (Requirement 10.2).

## Error handling

- **Backend**: reuse `writeError`/`writeUnauthorized`. Bad path id / body → 400; unauthenticated → 401; wrong role → 403 (handled by `RequireRoles` middleware before the handler); delegation overlap → 409; unexpected → 500. Policy validation failures → 400 with a Bahasa Indonesia message.
- **Frontend**: TanStack Query `error` from `ApiError`. List errors → `StatusMessage` error (icon + text). Mutation errors surfaced inline (hierarchy) or as a banner (delegation 409, leave/policy validation), always icon + text, retaining entered values on validation failure.

## Testing strategy

The design has no universal "for all inputs" correctness properties suitable for property-based testing — it is CRUD-over-HTTP plus UI rendering and route gating. Per the PBT guidance (simple CRUD, UI rendering, and infrastructure wiring are not PBT candidates), testing is example-based and integration:

- **Backend handler tests** (Go, table-driven, `*_test.go` co-located): for each list endpoint and policy write, cover success (correct JSON shape, pool selection via a mock/read store), 401 (no auth context), 403 (wrong role — via middleware test), 400 (bad body/id, policy min>max), 409 (delegation overlap already covered on the write handler; assert unchanged). Mock the read repository and policy store; assert `audit.Write` is called on policy create/edit with actor/action/before/after/ip.
- **Backend repository tests**: real Postgres integration test asserting list queries read expected rows and that reads hit the replica repository (or the pending-swap seam). Assert money amounts serialize as exact numeric strings.
- **Frontend component tests** (mirror `src/features/audit-log/__tests__` and `audit-logs.test.ts`): render each page with mocked query hooks — loading state renders spinner, error renders icon+text, populated list renders rows with tabular-nums on numeric columns and role badge with icon. Form tests: Zod validation (self-supervisor block, min>max, required fields), successful submit invalidates the query key, 409 renders conflict message. Route-guard test asserting non-authorized roles are blocked from `/settings/*` (mirror `_protected` role test) and that the nav item is filtered out.

Coverage target >= 80% on new `internal/*` code (project rule). No merge with failing/skipped tests.

## Security

- RBAC enforced at the middleware layer (`RequireRoles` including `APPACCESS`) for both writes and the new reads, and reflected at the frontend route guard + nav filter (defense in depth; the backend guard is authoritative).
- Reads use the replica pool; writes and audit use the primary pool (Requirements 2.5, 4.1, 5.1, 5.4, 6.1).
- Every mutation writes `audit_logs` (Requirement 8) — already present on hierarchy/delegation/leave; added to policy create/edit.
- No secrets logged; timestamps stored UTC (timestamptz), displayed Asia/Jakarta; money as numeric strings on the wire.

## Non-goals (restated)

- No maker-checker approval gate on RBAC configuration changes — they apply directly (Requirement 11.1).
- No payment trigger or execution from the RBAC area (Requirement 11.2).
- No new roles introduced; only widening authorization to the existing `APPACCESS` role (Requirement 11.3).
- No policy delete endpoint (not requested).
- No changes to the vendor portal (this feature is internal Company Portal only).
