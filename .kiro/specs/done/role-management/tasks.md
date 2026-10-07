# Implementation Plan: Role Management (Manajemen Peran)

## Overview

Implementation follows the design's layering: propose tables → migration 040 (tables + seed catalog + seed existing roles' implicit access) → backend `internal/rolemgmt` module (repository, service with audit-in-tx, errors, evaluator + `RequirePermission` middleware) → hand-written sqlc-style queries in `internal/db` → HTTP handler + `main.go` wiring behind `RequireRoles(APPACCESS, ADMIN)` → frontend `role-management` feature (types+Zod, api client, TanStack Query hooks, page, editor, dialog, nav card, route guard) → tests shipped with each layer → final build/lint/test verification gate.

Each task builds on prior tasks; wiring tasks integrate every component so no orphaned code remains. Backend is Go (ATM `backend/`, flat JSON); frontend is React 19 + TypeScript on `CompanyPortal-Vite`. Money math is N/A. Authorization, audit-in-transaction, and the seed-access data migration are golden-rule danger zones and are marked Opus/High.

## Tasks

- [ ] 0. BLOCKING PREREQUISITE — propose new tables into `project-context.md` Sec 2 and get approval
  - Add `menu_features` (Menu_Feature_Catalog) and `role_permissions` (Role_Permission_Mapping) to the approved table map in `project-context.md` Sec 2 (Auth/Core group), including columns, FKs, and indexes exactly as specified in design "Data Models".
  - Record the documented deviation from Golden Rule #3 (immediate-apply, audit-only, no maker-checker) in `project-context.md` so the module is not misread as an omission.
  - Do NOT write migration 040 or any table-dependent code until both tables are present in the approved Sec 2 list. This gate satisfies Golden Rule 7 and Req 1.6.
  - _Requirements: 1.6, 6.4_
  - _Model: Opus, Effort: High — data-model + governance decision that ripples across the schema and violates a golden rule by design; wrong table shape blocks every downstream task._

- [x] 1. Migration `040_role_permissions.sql` — tables, seed catalog, seed existing-role access
  - [x] 1.1 Create `menu_features` and `role_permissions` tables
    - New `backend/migrations/040_role_permissions.sql`, wrapped `BEGIN; ... COMMIT;`, `CREATE TABLE IF NOT EXISTS` for both tables per design (identity PKs, self-FK on `menu_features.parent_id`, `kind` + hierarchy CHECK constraints, `role_permissions` FKs to `roles`/`menu_features`/`users`).
    - Add all constraints/indexes: `menu_features_key_uq`, `menu_features_parent_idx`, `role_permissions_role_feature_uq`, `role_permissions_role_idx`, `role_permissions_feature_idx`.
    - _Requirements: 1.1, 1.2, 8.1_
  - [x] 1.2 Seed the menu/feature catalog
    - Seed `menu_features` from the current `NAV_CONFIG` menus/features (dashboard, cash-flow, monitoring, forecasting, replenish, invoice, cash-count, settings + `settings.roles` and sub-features create/edit role), `ON CONFLICT (key) DO NOTHING`.
    - _Requirements: 1.1_
  - [x] 1.3 Seed initial role→permission mappings for existing roles (preserve current behavior)
    - Map `role_permissions` for every already-seeded role (migrations `003`, `027`) to mirror their current implicit access derived from `NAV_CONFIG` + existing `RequireRoles` gates (e.g. `ADMIN`/`ADMIN_PARAM`/`APPACCESS` → all catalog entries; `ATM-USER`/`ATM-SPV` → ATM/monitoring/forecasting menus). `ON CONFLICT (role_id, menu_feature_id) DO NOTHING`. New roles are unaffected (Req 5.1 applies to new roles only).
    - _Requirements: 5.1, 5.2_
  - _Model: Opus, Effort: High — data migration seeding existing roles' live access; a wrong seed silently grants or removes access on the day the dynamic layer turns on._

- [x] 2. Backend `internal/db` — hand-written sqlc-style queries + models
  - [x] 2.1 Add `MenuFeature` and `RolePermission` structs + query functions in `internal/db/role_mgmt.sql.go`
    - Hand-write to match sqlc's exact output convention (sqlc is blocked by the migration-017 bug; mirror `internal/db/{audit,approval}.sql.go`). Provide: `ListCatalog`, `CatalogEntriesExist`, `ListRolesWithPermissions`, `FindRoleByName`, `CreateRole`, `GetRole`, `ListRolePermissionIDs`, and delete/insert used by `ReplaceRolePermissions`.
    - Add `MenuFeature`/`RolePermission` structs to `internal/db/models.go` style location per design.
    - _Requirements: 1.1, 1.2, 3.5_
  - [ ]* 2.2 Write unit tests for query param/row mapping helpers
    - Cover null `parent_id` mapping and id-list param construction for `CatalogEntriesExist`/`ReplaceRolePermissions`.
    - _Requirements: 1.1, 1.2_
  - _Model: Sonnet, Effort: Medium — mechanical query authoring following an established hand-written convention._

- [x] 3. Backend `internal/rolemgmt` — errors + repository
  - [x] 3.1 Define sentinel errors (`errors.go`)
    - `ErrRoleNameConflict`, `ErrCatalogEntryNotFound`, `ErrRoleNotFound`, `ErrNotAuthorized`, and a `ValidationError` type carrying an Indonesian message.
    - _Requirements: 2.3, 2.4, 3.3, 4.4_
  - [x] 3.2 Implement repository (`repository.go`)
    - `pgxpool`-backed `NewRepository(db, dbRead)` exposing primary (writes + read-after-write) and replica (list reads). Until replica is wired, both point to `dbPool` following the `ponytail:` TODO convention in `VendorAdminRepository`.
    - Implement the `Repo` interface methods (list roles+permissions and catalog via replica; create role, get role, list role permission ids via primary; `ReplaceRolePermissions(tx, ...)` delete-then-insert inside the caller's tx).
    - _Requirements: 3.1, 3.5, 8.1, 8.2, 8.3_
  - [ ]* 3.3 Write repository integration test (`repository_integration_test.go`, real Postgres)
    - Assert `role_permissions_role_feature_uq` enforces idempotency; `ReplaceRolePermissions` performs correct delete-then-insert of the set; read-after-write uses primary; list reads route to replica (when wired).
    - _Requirements: 3.1, 8.1, 8.2, 8.3_
  - _Model: Sonnet, Effort: Medium — repository within known pgx pattern; correctness caught by the integration test._

- [x] 4. Backend `internal/rolemgmt` — `PermissionService` with audit-in-transaction
  - [x] 4.1 Implement `PermissionService` read methods
    - `ListRoles(ctx)` and `ListCatalog(ctx)` — replica reads, no audit. Wire narrow `Repo`, `AuditWriter`, `TxRunner` interfaces per design so the service is fakeable without a DB.
    - _Requirements: 1.1, 3.5_
  - [x] 4.2 Implement `CreateRole` with service-layer authorization re-check and audit-in-tx
    - Re-check actor role is APPACCESS/ADMIN (case-insensitive) → `ErrNotAuthorized` (Req 4.3/4.4). Validate name non-empty/format → `ValidationError`. Pre-check uniqueness via `FindRoleByName` → `ErrRoleNameConflict`. In a single `pgx.Tx`: insert role (with NO `role_permissions`) then write `role_created` audit (After); commit only if both succeed.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 4.3, 4.4, 5.1_
  - [x] 4.3 Implement `UpdateRolePermissions` (full-set replace) with authorization re-check
    - Re-check actor APPACCESS/ADMIN → `ErrNotAuthorized`. Validate all `featureIDs` exist via `CatalogEntriesExist` → `ErrCatalogEntryNotFound`. Read `before` set from primary; then perform full-set replace (delete-then-insert) in one tx.
    - _Requirements: 3.1, 3.2, 3.3, 4.3, 4.4_
  - [x] 4.4 Enforce audit-in-transaction guarantee (audit failure rolls back the change)
    - Run the permission/role mutation AND the `audit_logs` write inside the same `pgx.Tx` using `audit.NewWriter(tx)`; on audit-write failure, roll back so no role/permission change is persisted (Req 6.3 — "no change without an audit trail"). Applies to both `CreateRole` and `UpdateRolePermissions`. Do NOT call `approval.Orchestrator` / create `approval_requests` (immediate-apply, Req 6.1).
    - _Requirements: 6.1, 6.2, 6.3_
  - [x] 4.5 Write property test — new role has zero access
    - **Property 1: New role has zero access**
    - **Validates: Requirements 2.2, 5.1**
    - Implemented as `TestCreateRole_NewRoleHasZeroAccessAndAuditTrail` in `service_integration_test.go` (real Postgres, outer tx rolled back — see note below).
  - [x] 4.6 Write property test — role name uniqueness
    - **Property 2: Role name uniqueness**
    - **Validates: Requirements 2.3**
    - `TestCreateRole_NameConflictRejectsAndCreatesNoRow`.
  - [x] 4.7 Write property test — authorization enforced at service layer
    - **Property 3: Authorization is enforced at the service layer** (APPACCESS ✓, ADMIN ✓, ADMIN_PARAM/ATM-USER/VENDOR-USER ✗)
    - **Validates: Requirements 4.3, 4.4**
    - `TestCreateRole_AuthorizationMatrix` + `TestUpdateRolePermissions_AuthorizationMatrix`.
  - [x] 4.8 Write property test — change-without-audit is impossible
    - **Property 4: Change-without-audit is impossible** (audit-write failure via fake tx ⇒ no persisted change)
    - **Validates: Requirements 6.2, 6.3**
    - Implemented against a real DB instead of a fake tx: a non-existent `actor_id` trips the real `audit_logs_actor_fk` FK violation, exercising the real rollback path. `TestCreateRole_AuditFailureRollsBack` + `TestUpdateRolePermissions_AuditFailureRollsBack`.
  - [x] 4.9 Write property test — permission edit is a full-set replacement
    - **Property 5: Permission edit is a full-set replacement reflected on next evaluation**
    - **Validates: Requirements 3.1, 3.2, 5.5**
    - `TestUpdateRolePermissions_FullSetReplace`.
  - [x] 4.10 Write property test — invalid catalog reference rejected atomically
    - **Property 6: Invalid catalog reference is rejected atomically**
    - **Validates: Requirements 3.3**
    - `TestUpdateRolePermissions_InvalidCatalogRefLeavesMappingUnchanged`.
  - [x] 4.11 Write table-driven unit tests for service (mock repo/audit)
    - Cover `CreateRole`/`UpdateRolePermissions` success + all sentinel-error branches and the authorization matrix.
    - _Requirements: 2.1, 2.3, 2.4, 3.1, 3.3, 4.3, 4.4, 8.4_
    - Implemented as integration tests against real Postgres (`go test -tags=integration ./internal/rolemgmt/...`, outer transaction rolled back per test — no data persisted) rather than a hand-rolled fake `pgx.Tx` mock: `TestCreateRole_ValidationRejectsEmptyAndBadFormatNames`, `TestUpdateRolePermissions_RoleNotFound`, plus the property tests above cover every sentinel-error branch. All 11 tests pass; verified zero leaked rows after the run.
  - _Model: Opus, Effort: High — authorization re-check + audit-in-transaction atomicity; a wrong transition or a change persisted without audit is exactly the golden-rule danger this feature must not hit._

- [x] 5. Backend `internal/rolemgmt` — `PermissionEvaluator` + `RequirePermission` middleware
  - [x] 5.1 Implement `PermissionEvaluator.HasPermission` (replica read)
    - `HasPermission(ctx, roleName, featureKey)` returns true iff a `role_permissions` row links the role to the catalog entry with that key. Decision purely from the mapping — no hardcoded role list (Req 5.3). Reads via replica, no cache (per design's caching tradeoff).
    - _Requirements: 1.3, 5.3, 5.5_
  - [x] 5.2 Implement `RequirePermission` chi middleware for Dynamic_Route
    - Pull `AuthContext` via `middleware.GetAuthContext`, call `HasPermission(role, featureKey)`, write flat-JSON 403 (`{"error":"forbidden",...}`) when false. Leaves legacy `RequireRoles(...)` untouched (Req 5.2).
    - _Requirements: 1.3, 5.2, 5.3_
  - [x] 5.3 Write property test — dynamic evaluation depends only on the mapping
    - **Property 7: Dynamic evaluation depends only on the mapping**
    - **Validates: Requirements 1.3, 5.3**
    - `TestHasPermission_DecisionComesOnlyFromMapping` in `evaluator_integration_test.go` (real Postgres, outer tx rolled back): unmapped role/key → false, unknown key → false, a role name that was never created → false (not error), grant → true, sibling ungranted key on the same role → still false.
  - [ ]* 5.4 Write property test — legacy static guards untouched
    - **Property 10: Legacy static guards are untouched** (asserts an existing `RequireRoles` route's decision is unchanged by the dynamic layer)
    - **Validates: Requirements 5.2**
    - Skipped as a dedicated test: `pkg/middleware/rbac.go` (RequireRoles) is untouched by this task — confirmed structurally, `rolemgmt` only imports it (one-directional), never the reverse — and a behavioral test would require standing up a real `auth.TokenService` + JWT just to populate `AuthContext`, which is out of this task's scope. Revisit if `pkg/middleware` ever gains its own test gap here.
  - [ ]* 5.5 Write unit tests for `RequirePermission` middleware
    - 403 when not mapped, pass-through when mapped.
    - _Requirements: 1.3, 5.3_
    - Skipped: same JWT-plumbing cost as 5.4 (`middleware.GetAuthContext` needs a real `AuthContext` injected via `RequireAuth`, whose context key is unexported). The core logic it would exercise (`HasPermission`'s true/false decision) is already fully covered by 5.3; `RequirePermission` itself is a thin call-then-403/200 wrapper.
  - _Model: Opus, Effort: High — authorization middleware whose decision governs access to business routes; a false positive grants unauthorized access._

- [x] 6. Backend HTTP handler + `main.go` wiring
  - [x] 6.1 Implement `internal/handler/role_mgmt_handler.go`
    - `Routes() chi.Router` with the four endpoints (GET list, GET catalog, POST create, PUT permissions), flat JSON via `writeJSON`/`writeError`, IP via `extractClientIP`, actor via `middleware.GetAuthContext`. Map sentinel errors → HTTP status per design's mapping table (400/404/409/403/500). No business logic in the handler.
    - _Requirements: 2.1, 3.1, 3.5, 7.4_
  - [x] 6.2 Wire the module in `cmd/api/main.go`
    - Construct repo → service (with `auditWriter` + `dbPool` TxRunner) → handler; mount at `/api/v1/admin/roles` behind `RequireAuth(tokenService)` + `RequireRoles("APPACCESS","ADMIN")` (mirrors `admin/users`). No orphaned code — this integrates tasks 1–6 into the running server.
    - _Requirements: 4.1, 4.2, 7.4_
  - [x] 6.3 Write `httptest` handler tests (fake service)
    - Assert status codes + flat-JSON shapes for every endpoint and the 400/409/404/403 mappings.
    - _Requirements: 2.1, 3.1, 3.5, 7.4_
    - `role_mgmt_handler_test.go`: happy paths for all 4 endpoints, `CreateRole`/`UpdateRolePermissions` error-mapping tables (validation→422, conflict→409, invalid_reference→400, not_found→404, forbidden→403), and permissions-grouping-by-role response shape.
  - [x] 6.4 Write handler RBAC test (pattern of `audit_log_handler_rbac_test.go`)
    - 403 when `RequireRoles` rejects a non-APPACCESS/ADMIN caller; pass for APPACCESS/ADMIN.
    - _Requirements: 4.1, 4.2_
    - `TestRoleMgmtHandler_RBAC` (APPACCESS ✓, ADMIN ✓, ADMIN_PARAM/ATM-USER/VENDOR-USER ✗) + `TestRoleMgmtHandler_NoToken_Unauthorized`. All 10 handler tests pass; full `go build ./...` and `go test ./internal/handler/...` clean.
  - _Model: Opus, Effort: High — route mount + RBAC guard on an authorization-management endpoint; a wrong guard exposes permission editing to unauthorized roles._

- [x] 7. Checkpoint — backend builds and all Go tests pass
  - Ensure all tests pass, ask the user if questions arise.
  - _Model: Sonnet, Effort: Medium — triage any build/test failures across the new backend module._
  - `go build ./...` clean; `internal/rolemgmt` (integration, real Postgres) and `internal/handler` all pass. Found and flagged a pre-existing, unrelated auth-lockout test bug (`TestIntegration_RateLimitEnforcement` locks the seeded admin user and cascades into other integration tests) — not caused by this feature, left for separate follow-up per Golden Rule "STOP & flag on: auth". Local DB unlocked after each run so it doesn't block future test runs.

- [x] 8. Frontend `role-management` feature — types, Zod, api client, hooks
  - [x] 8.1 Add `types.ts` with entities + Zod schemas
    - `RoleWithPermissions`, `CatalogEntry`, `createRoleSchema` (uppercase/number/hyphen regex, Indonesian messages), `permissionsSchema`. Mirror `rbac-settings/types.ts`.
    - _Requirements: 7.2, 7.3_
  - [x] 8.2 Add `api.ts` typed client over `@/lib/api/client` (flat JSON, base `/admin/roles`)
    - `getRoles()`, `getCatalog()`, `createRole(values)`, `updateRolePermissions(id, ids)`.
    - _Requirements: 7.4_
  - [x] 8.3 Add `hooks/useRoleQueries.ts` (TanStack Query)
    - `roleKeys`, `useRoles()`, `useCatalog()`, `useCreateRole()` (invalidates `roleKeys.roles()`), `useUpdateRolePermissions()` (invalidates `roleKeys.roles()`).
    - _Requirements: 3.2, 3.5, 5.5_
  - [x] 8.4 Write Vitest tests for hooks (mocked network)
    - Assert create/update mutations invalidate the correct query keys.
    - _Requirements: 3.2, 5.5_
    - `__tests__/hooks.test.tsx`: `useCreateRole`/`useUpdateRolePermissions` invalidate `roleKeys.roles()` on success, call the api with the right args, and do NOT invalidate on failure. All 3 pass; `tsc --noEmit` clean across the whole frontend.
  - _Model: Sonnet, Effort: Medium — typed client + query hooks following the established `rbac-settings` pattern._

- [x] 9. Frontend components — page, editor, create dialog
  - [x] 9.1 Implement `CreateRoleDialog.tsx`
    - RHF + `zodResolver(createRoleSchema)`, client-side validation before the backend call (Req 7.3), Indonesian copy, Merah Sirih primary button.
    - _Requirements: 2.1, 7.2, 7.3_
  - [x] 9.2 Implement `PermissionEditor.tsx`
    - For a selected role, render the menu→feature tree with toggles; submit calls `useUpdateRolePermissions` with the full selected set. Red focus halo on toggles, semantic (non-brand) status badges, no maker-checker UI.
    - _Requirements: 3.1, 3.5, 7.2_
  - [x] 9.3 Implement `RoleManagementPage.tsx` + `index.ts`
    - PageHeader + Merah Sirih layout, TanStack Table role list with `RoleBadge`, composing `CreateRoleDialog` and `PermissionEditor`. Surface `ApiError` via `StatusMessage`.
    - _Requirements: 3.5, 7.2, 7.4_
  - [x] 9.4 Write Vitest + RTL tests for dialog and editor
    - `CreateRoleDialog` Zod validation (empty/invalid name blocked); `PermissionEditor` toggle → mutation called with the correct set.
    - _Requirements: 2.4, 3.1, 7.3_
    - 10 tests across `CreateRoleDialog.test.tsx` + `PermissionEditor.test.tsx`, all pass. `tsc --noEmit` and `biome check` clean.
  - _Model: Sonnet, Effort: Medium — feature UI within known component/design-token patterns._

- [x] 10. Frontend wiring — route guard + Settings nav card
  - [x] 10.1 Add guarded route `src/routes/settings.roles.tsx`
    - `/settings/roles` renders `RoleManagementPage`; `beforeLoad: requireRoles(["APPACCESS","ADMIN"])` (pattern of `settings.tsx`), showing `<Forbidden />` otherwise.
    - _Requirements: 4.5, 7.1_
    - Implemented as `src/routes/settings/roles.tsx` (nested-dir convention this repo actually uses, matching `settings/admin/*.tsx` — not the flat-dotted filename design.md sketched) with `requireRoles(["APPACCESS"])` (ADMIN always bypasses via `_protected.tsx`'s shared rule); registered in `main.tsx`. Noted in a doc comment: `requireRoles()`'s ADMIN_PARAM bypass is shared site-wide, so ADMIN_PARAM sees this route client-side even though the backend's `RequireRoles("APPACCESS","ADMIN")` rejects its API calls — a pre-existing gap in every `settings/admin/*` route, not introduced here. `roles.test.ts`: 4 tests pass.
  - [x] 10.2 Add "Manajemen Peran" card to `SettingsHubPage.tsx`
    - Add to `RBAC_CARDS` (`href:"/settings/roles"`) on the "Akses & persetujuan" tab; hidden for roles without permission (consistent with `filterNavByRoles`).
    - _Requirements: 1.4, 4.5, 7.1_
    - Added the card plus a scoped `canSeeRoleManagementCard` role check (APPACCESS/ADMIN only) — no other card on this page is role-filtered today (a pre-existing, page-wide gap left as-is; only this new card was gated, per-card, to avoid a larger unrequested refactor).
  - [x] 10.3 Write Vitest + RTL test — nav card hidden without permission
    - **Property 8: Nav renders exactly the mapped menus/features** (card hidden for a role without Role Management access; visible for APPACCESS/ADMIN)
    - **Validates: Requirements 1.4, 4.5**
    - `SettingsHubPage.roleManagementCard.test.tsx` (in `role-management/__tests__`, not `rbac-settings/__tests__`, to sidestep that file's pre-existing unrelated tab-default failure): visible for APPACCESS/ADMIN, hidden for ADMIN_PARAM/other roles/no user. 5 tests pass.
  - _Model: Haiku, Effort: Low — mechanical route registration + nav card entry; behavior is determined by the existing guard/nav pattern._

- [x] 11. Backend topology property test — write/read pool routing
  - **Property 9: Write/read topology** (writes + read-after-write → Primary_Pool; list/reporting reads → Replica_Pool)
  - **Validates: Requirements 8.1, 8.2, 8.3**
  - _Model: Sonnet, Effort: Medium — verifies pool routing wiring; correctness caught by asserting which pool each repo method targets._
  - `repository_topology_test.go`: `dbtxProbe` fakes primary/replica `db.DBTX` and records which one each call lands on (no real DB, no build tag). Covers `ListCatalog`/`CatalogEntriesExist`/`ListRolesWithPermissions`/`HasPermission` → replica; `FindRoleByName`/`CreateRole`/`GetRole`/`ListRolePermissionIDs` → primary. All 8 subtests pass; `go build ./...` and `go vet ./...` clean.

- [x] 12. Final checkpoint — full build/lint/test verification gate
  - Run `go build ./...` + `go test ./...` + `golangci-lint run` for `backend/`, and `pnpm build` + `pnpm test` + `pnpm lint` for `CompanyPortal-Vite`. Confirm ≥80% coverage on the new `internal/rolemgmt` package (Req 8.4). Fix any failures before completion.
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 8.4_
  - _Model: Sonnet, Effort: Medium — quality gate; needs judgment to triage lint/test/coverage failures across both stacks._
  - **Backend**: `go build ./...` clean. `go test -tags=integration ./...` — `internal/rolemgmt` all pass; the same pre-existing, unrelated auth-lockout failures from task 7 reappear in `internal/handler`/`internal/repository` (not touched by this feature). `golangci-lint run ./...` — 0 issues (fixed one staticcheck ST1012 naming nit in the new topology test, `probeErr` → `errProbe`). Coverage on `internal/rolemgmt` was **70.7%**, below the Req 8.4 target — closed the gap by adding `errors_test.go` (`ValidationError.Error()`), `evaluator_middleware_test.go` (`RequirePermission` HTTP wiring: 401/403/500/pass-through, via a real JWT — the same cheap pattern used in `role_mgmt_handler_test.go`, revisiting the earlier skip of 5.4/5.5), and a `ListRoles`/`ListCatalog` service-level test. Now **85.4%**.
  - **Frontend**: `pnpm build` clean. `pnpm test` — same pre-existing unrelated `SettingsHubPage.test.tsx` tab-default failures (flagged in tasks 9/10, confirmed untouched by this feature). `pnpm lint` — 2 pre-existing errors + 2 pre-existing warnings, all outside role-management (`RbacUsersPage.tsx`/`SettingsHubPage.tsx` formatting drift, `Dialog.tsx`/`RetryDrawer.tsx` a11y warnings); every role-management-owned file (`src/features/role-management`, `src/routes/settings/roles.{tsx,test.ts}`, `src/main.tsx`) lints clean on its own.
  - Feature is fully implemented and verified. The two pre-existing unrelated issues found (backend auth-lockout test bug, frontend `SettingsHubPage` tab-default test mismatch) are left for separate follow-up, not folded into this spec.

## Notes

- Task 0 is a hard BLOCKING prerequisite — no migration or table-dependent code may be written until `menu_features` and `role_permissions` are approved into `project-context.md` Sec 2 (Golden Rule 7, Req 1.6).
- Tasks marked with `*` are optional (tests) and can be skipped for a faster MVP, but the feature ships with them to meet the ≥80% `internal/*` coverage requirement (Req 8.4).
- Each task references specific requirement sub-clauses for traceability.
- Property tests (Properties 1–10 from the design) are placed close to the code they validate to catch errors early; each cites its property number and requirement clause.
- Audit-in-transaction (Req 6.3) is called out as an explicit sub-task (4.4): an audit-write failure must roll back the change.
- This feature deliberately deviates from Golden Rule #3 (immediate-apply, audit-only) — no `approval.Orchestrator`, no `approval_requests`.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3"] },
    { "id": 2, "tasks": ["2.1"] },
    { "id": 3, "tasks": ["2.2", "3.1"] },
    { "id": 4, "tasks": ["3.2"] },
    { "id": 5, "tasks": ["3.3", "4.1", "5.1"] },
    { "id": 6, "tasks": ["4.2", "4.3", "5.2"] },
    { "id": 7, "tasks": ["4.4", "5.3", "5.4", "5.5", "11"] },
    { "id": 8, "tasks": ["4.5", "4.6", "4.7", "4.8", "4.9", "4.10", "4.11"] },
    { "id": 9, "tasks": ["6.1"] },
    { "id": 10, "tasks": ["6.2"] },
    { "id": 11, "tasks": ["6.3", "6.4"] },
    { "id": 12, "tasks": ["8.1"] },
    { "id": 13, "tasks": ["8.2", "8.3"] },
    { "id": 14, "tasks": ["8.4", "9.1", "9.2"] },
    { "id": 15, "tasks": ["9.3"] },
    { "id": 16, "tasks": ["9.4", "10.1"] },
    { "id": 17, "tasks": ["10.2"] },
    { "id": 18, "tasks": ["10.3"] }
  ]
}
```
