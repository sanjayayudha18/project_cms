# Implementation Plan: RBAC Settings Menu

## Overview

Fill the read-side gap on the ATM backend and turn the dead "Pengaturan" nav link into a working RBAC Settings hub in the internal Company Portal. Work proceeds backend-first (queries → repositories → handler → auth widening → tests), then frontend (feature module scaffolding → shared components → hub → per-entity pages → routes → nav → tests), ending in a full verification gate. Each task builds on the prior one and ends wired into the app.

This design is CRUD-over-HTTP plus UI rendering and route gating, with no universal "for all inputs" correctness properties — testing is example-based and integration (no property-based test tasks).

## Tasks

- [ ] 1. Backend: RBAC read + policy-write sqlc queries
  - Add read queries in `backend/internal/db`: `ListUserHierarchy` (`id, supervisor_id, approval_level, role, auth_source` from `users`), `ListDelegations` (non-revoked, ordered by `start_at DESC`), `ListLeaves` (ordered by `start_at DESC`), `ListApprovalPolicies` (`id, document_type, min_amount, max_amount, required_level` ordered by `document_type, min_amount`).
  - Add policy write queries `CreateApprovalPolicy` and `UpdateApprovalPolicy` (insert/update on `approval_policies`).
  - Run `sqlc generate`. If blocked by the known pre-existing migration-017 bug, hand-write the query functions in `internal/db` matching sqlc's output convention (as done for `audit`/`approval`) and leave a comment to regenerate once 017 is fixed.
  - Serialize `min_amount`/`max_amount` as numeric strings (money is `numeric`, never float).
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 7.3, 8.4_
  - _Model: Sonnet, Effort: Medium — SQL-first read/write queries within a known pattern, but the sqlc-017 blocker needs judgment to hand-write correctly and money must stay numeric._

- [ ] 2. Backend: RBAC repositories (replica read + primary policy write)
  - [ ] 2.1 Implement `RbacReadRepository` on the Replica_Pool in `backend/internal/repository`, exposing `ListUserHierarchy`, `ListDelegations`, `ListLeaves`, `ListApprovalPolicies`. If the replica pool wiring is not yet live, accept a pool at construction and pass the primary pool with a pending-swap comment so the API is stable when the replica lands.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 9.1_
    - _Model: Sonnet, Effort: Medium — repository over generated queries; the replica-seam decision needs care but follows the audit-log precedent._
  - [ ] 2.2 Implement `ApprovalPolicyStore` on the Primary_Pool exposing `CreateApprovalPolicy` and `UpdateApprovalPolicy`, returning the persisted row (with before-state for audit on update).
    - _Requirements: 7.3, 8.4_
    - _Model: Sonnet, Effort: Medium — primary-pool write store returning before/after for the audit trail._

- [ ] 3. Backend: RbacListHandler (list reads + policy create/edit)
  - [ ] 3.1 Create `backend/internal/handler/rbac_list_handler.go` with `NewRbacListHandler(readRepo, policyStore, auditWriter)` and `Routes()`. Add GET `/users/hierarchy`, `/delegations`, `/leaves`, `/policies` reading via `RbacReadRepository`, returning flat JSON (`{ "users": [...] }` etc., RFC3339 timestamps, amounts as numeric strings) using the existing `writeError`/`writeUnauthorized` helpers.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.6, 2.9, 9.1_
    - _Model: Sonnet, Effort: Medium — read handlers over the repository, matching the existing flat-JSON ATM shape._
  - [ ] 3.2 Add POST `/policies` and PUT `/policies/{id}` to the handler: parse body, validate (`document_type` non-empty, `min_amount` <= `max_amount`, `required_level` positive integer → 400 with Bahasa Indonesia message), write via `ApprovalPolicyStore` on the Primary_Pool, then `audit.Write` (`admin_create_policy`/`admin_update_policy`) with actor, before/after, ip. Return 201/200 with the policy body.
    - _Requirements: 7.3, 8.4, 11.1_
    - _Model: Opus, Effort: High — mutation path touching money validation + mandatory audit-on-write; a missed audit or a wrong min/max guard is a compliance/correctness defect._

- [ ] 4. Backend: widen auth and mount handler in main.go
  - Change the `/api/v1/admin/approval` mount in `backend/cmd/api/main.go` to `RequireRoles("ADMIN", "ADMIN_PARAM", "APPACCESS")` (covering the existing write endpoints).
  - Construct `RbacReadRepository` (replica) and `ApprovalPolicyStore` (primary), build `RbacListHandler`, and mount it under the same `RequireAuth` + `RequireRoles` guard and prefix alongside `AdminApprovalHandler`.
  - _Requirements: 2.7, 2.8, 3.1, 3.2, 3.3_
  - _Model: Opus, Effort: High — widening an RBAC authorization guard on live admin write endpoints; getting the role list or mount composition wrong is an auth regression._

- [ ] 5. Backend: handler and repository tests
  - [ ]* 5.1 Table-driven handler tests (co-located `rbac_list_handler_test.go`): each list endpoint success (correct JSON shape via mock read store), 401 (no auth context), 403 (non-authorized role via middleware), 400 (bad body/id, policy `min_amount` > `max_amount`), and assert `audit.Write` is called on policy create/edit with actor/action/before/after/ip. Confirm the delegation 409 overlap path on the write handler stays unchanged.
    - _Requirements: 2.8, 3.3, 5.3, 5.6, 7.3, 8.4_
    - _Model: Opus, Effort: High — asserting the auth matrix and audit-on-mutation invariants; these tests are the safety net for the auth/money/audit changes._
  - [ ]* 5.2 Repository integration tests (real Postgres): list queries return expected rows, reads go through the read repository (or the pending-swap seam), and money amounts serialize as exact numeric strings.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
    - _Model: Sonnet, Effort: Medium — integration coverage over real queries and the read/write pool split._

- [ ] 6. Checkpoint - backend green
  - Ensure `go test ./...` and `golangci-lint run` pass for the backend. Ask the user if questions arise.
  - _Requirements: 2.5, 3.1, 8.4_
  - _Model: Sonnet, Effort: Medium — triaging build/lint/test failures needs judgment but not deep reasoning._

- [ ] 7. Frontend: rbac-settings feature module scaffolding
  - [ ] 7.1 Create `src/features/rbac-settings/types.ts`: entity types (user+hierarchy, delegation, leave, policy), Zod schemas for the hierarchy/delegation/leave/policy forms (self-supervisor guard, `min_amount` <= `max_amount`, positive `required_level`, RFC3339 datetimes), and badge variant maps for roles.
    - _Requirements: 4.2, 7.3, 9.2_
    - _Model: Sonnet, Effort: Medium — Zod schemas encode the client-side validation rules that mirror server guards._
  - [ ] 7.2 Create `src/features/rbac-settings/api.ts`: client fns for GET users/delegations/leaves/policies and POST/PUT policies, plus the existing hierarchy/delegation/leave writes, via `@/lib/api/client` reusing `ApiError`.
    - _Requirements: 2.9, 4.1, 5.1, 5.4, 6.1, 7.3_
    - _Model: Sonnet, Effort: Medium — typed API client layer over flat-JSON endpoints._
  - [ ] 7.3 Create `src/features/rbac-settings/hooks/useRbacQueries.ts`: `rbacKeys` factory, list `useQuery` hooks, and write `useMutation` hooks that invalidate the relevant key on success.
    - _Requirements: 2.9, 2.10, 4.4, 5.5, 6.3_
    - _Model: Sonnet, Effort: Medium — TanStack Query wiring with cache invalidation on mutation._

- [ ] 8. Frontend: shared components (StatusMessage, RoleBadge)
  - [ ] 8.1 `StatusMessage.tsx`: loading spinner, error banner (`AlertTriangle` icon + Bahasa Indonesia text), and empty state — status conveyed by icon + text, never color alone.
    - _Requirements: 2.10, 2.11, 10.5_
    - _Model: Haiku, Effort: Low — small presentational component with fixed states._
  - [ ] 8.2 `RoleBadge.tsx`: map `role` → `{ label, icon, variant }` using semantic badge tokens (info/neutral, no brand red), pairing label with a Lucide icon.
    - _Requirements: 9.2, 10.5_
    - _Model: Haiku, Effort: Low — presentational badge with a static role→variant map._

- [ ] 9. Frontend: SettingsHubPage landing
  - Build `SettingsHubPage.tsx` with a small set of varied link cards (not an identical repeated grid) to the four sub-pages — "Hierarki Pengguna", "Delegasi Persetujuan", "Cuti Pengguna", "Kebijakan Persetujuan" — Bahasa Indonesia copy, Lucide icons (`Users`, `GitBranch`, `CalendarOff`, `Scale`), Merah Sirih tokens.
  - _Requirements: 1.1, 1.5, 10.1, 10.2_
  - _Model: Sonnet, Effort: Medium — layout must avoid the repeated-card-grid ban and follow theme rules._

- [ ] 10. Frontend: RbacUsersPage (hierarchy + roles)
  - Build `RbacUsersPage.tsx`: users+hierarchy table (uppercase headers, row dividers, `approval_level` right-aligned tabular-nums), role via `RoleBadge`, and an inline hierarchy edit form (RHF+Zod) for `supervisor_id` + `approval_level`. Block self-supervisor client-side (mirror server guard), keep entered values on rejection, show inline validation errors (icon + text), and invalidate `rbacKeys.users()` on success. Use `StatusMessage` for loading/error.
  - _Requirements: 4.2, 4.3, 4.4, 4.5, 9.1, 9.2, 10.3, 10.4, 10.5_
  - _Model: Sonnet, Effort: Medium — form with the self-supervisor guard and error/retain behavior; standard page assembly otherwise._

- [ ] 11. Frontend: RbacDelegationsPage (create + revoke + 409)
  - Build `RbacDelegationsPage.tsx`: delegations table, create form (`from_user_id`, `to_user_id`, RFC3339 `start_at`/`end_at`, optional `reason`), and a revoke action. On 409, render a conflict message (icon + Bahasa Indonesia text) via `StatusMessage`; invalidate `rbacKeys.delegations()` on create/revoke success. Loading/error via `StatusMessage`.
  - _Requirements: 5.1, 5.2, 5.4, 5.5, 5.6, 10.4, 10.5_
  - _Model: Sonnet, Effort: Medium — form plus explicit 409 conflict handling and cache refresh._

- [ ] 12. Frontend: RbacLeavesPage (create)
  - Build `RbacLeavesPage.tsx`: leaves table and create form (`user_id`, RFC3339 `start_at`/`end_at`, optional `reason`). Show validation errors with icon + text, invalidate `rbacKeys.leaves()` on success, loading/error via `StatusMessage`.
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 10.4, 10.5_
  - _Model: Sonnet, Effort: Medium — straightforward create form following the delegation page pattern._

- [ ] 13. Frontend: RbacPoliciesPage (list + create/edit)
  - Build `RbacPoliciesPage.tsx`: policies table with `document_type`, `min_amount`/`max_amount` (right-aligned tabular-nums), and `required_level`, plus a create/edit form (RHF+Zod: non-empty `document_type`, `min_amount` <= `max_amount`, positive `required_level`). Invalidate `rbacKeys.policies()` on success; loading/error via `StatusMessage`.
  - _Requirements: 7.1, 7.2, 7.3, 10.3, 10.4, 10.5_
  - _Model: Sonnet, Effort: Medium — money-bearing table + create/edit form; amounts stay numeric strings and align right._

- [ ] 14. Frontend: routes under src/routes/settings/
  - Add `settings.tsx` (path `/settings` → `SettingsHubPage`) and `settings/rbac/{users,delegations,leaves,policies}.tsx`, each attached to `protectedRoute` with `beforeLoad: requireRoles(["ADMIN","ADMIN_PARAM","APPACCESS"])`, and register them in the router tree the same way `auditLogsRoute` is registered.
  - _Requirements: 1.1, 1.3, 1.5, 3.2_
  - _Model: Haiku, Effort: Low — mechanical route scaffolding + registration following an existing template._

- [ ] 15. Frontend: navigation update
  - In `src/lib/config/navigation.ts`, widen the `settings` item roles to `["ADMIN","ADMIN_PARAM","APPACCESS"]` and ensure `filterNavByRoles` shows the item to `APPACCESS` and omits it for non-authorized roles.
  - _Requirements: 1.2, 1.4_
  - _Model: Haiku, Effort: Low — a role-list edit and a filter check; low blast radius, covered by tests._

- [ ] 16. Frontend: component and guard tests
  - [ ]* 16.1 Page render tests (mirror `src/features/audit-log/__tests__`): each page renders loading (spinner), error (icon + text), and populated table (rows with tabular-nums on numeric columns, `RoleBadge` with icon) using mocked query hooks.
    - _Requirements: 2.10, 2.11, 7.2, 9.2, 10.3, 10.5_
    - _Model: Sonnet, Effort: Medium — RTL tests over mocked hooks across the four pages._
  - [ ]* 16.2 Form and interaction tests: Zod validation (self-supervisor block, `min_amount` > `max_amount`, required fields), successful submit invalidates the query key, and delegation 409 renders the conflict message.
    - _Requirements: 4.2, 4.5, 5.5, 5.6, 6.4, 7.3_
    - _Model: Sonnet, Effort: Medium — validation and mutation-success/conflict behaviors need careful assertions._
  - [ ]* 16.3 Route-guard and nav-filter tests: non-authorized roles are blocked from `/settings/*` (mirror the `_protected` role test) and the Settings nav item is filtered out for them but shown to `APPACCESS`.
    - _Requirements: 1.2, 1.3, 1.4, 3.2_
    - _Model: Sonnet, Effort: Medium — guard/filter tests protect the auth-widening change on the frontend._

- [ ] 17. Final verification
  - Run `go test ./...`, `golangci-lint run`, `pnpm test`, `pnpm lint`, and `pnpm build` for the affected frontend; fix any failures until all gates pass.
  - _Requirements: 1.1, 2.5, 3.1, 8.4_
  - _Model: Sonnet, Effort: Medium — full quality gate; triaging failures needs judgment but rarely deep reasoning._

## Notes

- Tasks marked with `*` are optional (tests) and can be skipped for a faster MVP, though the auth/audit test tasks (5.1, 16.3) are strongly recommended given the auth-widening scope.
- Each task references specific requirement sub-clauses for traceability.
- Backend precedes frontend so the endpoints exist before the UI consumes them; the checkpoint (task 6) and final verification (task 17) enforce incremental validation.
- No property-based test tasks: the feature is CRUD-over-HTTP + UI rendering + route gating, which the PBT guidance excludes.
- Known constraint: `sqlc generate` may be blocked by a pre-existing migration-017 bug; hand-write query functions in `internal/db` matching sqlc's convention if so (task 1).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1"] },
    { "id": 1, "tasks": ["2.1", "2.2"] },
    { "id": 2, "tasks": ["3.1", "3.2"] },
    { "id": 3, "tasks": ["4"] },
    { "id": 4, "tasks": ["5.1", "5.2", "7.1"] },
    { "id": 5, "tasks": ["7.2", "8.1", "8.2"] },
    { "id": 6, "tasks": ["7.3", "9"] },
    { "id": 7, "tasks": ["10", "11", "12", "13"] },
    { "id": 8, "tasks": ["14", "15"] },
    { "id": 9, "tasks": ["16.1", "16.2", "16.3"] }
  ]
}
```
