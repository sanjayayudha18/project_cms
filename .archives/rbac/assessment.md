# Assessment — RBAC, Hierarchy & Multi-Layer Approval

> Source-of-truth assessment for the CMS **RBAC + approval hierarchy** stack: role-string authorization, the data-driven `role_permissions` catalog, and the maker-checker approval engine (`internal/approval` + `internal/audit` + `internal/rolemgmt`).
> Status: source-of-truth assessment · Date: 2026-09-28 · Phase: **implemented** (RBAC-Setup Tasks 1-9, role-management, rbac-settings-menu — all in `.kiro/specs/done/`).
> Sources: `rbac-design.md`, `rbac-role-feature-matrix.md`, `project-context.md` (Sec 2, 4, 12), `development-plan.md`, `.kiro/specs/done/{RBAC-Setup,role-management,rbac-settings-menu}`, and inspection of the actual code (`backend/internal/{approval,audit,rolemgmt}`, `backend/cmd/api/main.go`, `pkg/middleware/rbac.go`, `backend/migrations/`, `frontend/CompanyPortal-Vite/src/lib/config/navigation.ts`).

---

## 1. Executive Summary

RBAC in CMS is **not one system — it is three parallel authorization axes** that must be read together. This is the single most important fact for anyone touching auth here, and the reason this assessment exists as a source of truth.

1. **Role-string authorization (`RequireRoles`)** — the *actual* backend enforcement today. `pkg/middleware/rbac.go` compares the single JWT `role` claim against a hardcoded allow-list per route in `backend/cmd/api/main.go` (case-insensitive). There is **no** `RequirePermission` / `PermissionEvaluator` wired into the middleware chain.
2. **Data-driven permission catalog (`role_permissions`)** — `menu_features` × `roles`, admin-editable via the "Manajemen Peran" screen (`internal/rolemgmt`). Today it drives **frontend nav rendering + the Role Management UI only**. It is *not yet* the backend enforcement authority, though the role-management spec intends it to eventually replace the hardcoded guards.
3. **Maker-checker approval hierarchy** — a separate axis keyed off `supervisor_id` + `approval_level` (**not** `role`). Implemented in `internal/approval` (chain resolver, effective-approver/delegation resolver, orchestrator) + `internal/audit`. This is why `/api/v1/approvals` is `RequireAuth`-only: per-request authorization happens inside the orchestrator, not via `RequireRoles`.

**Current state: built and wired.** All three axes exist in code and are exercised by tests. `internal/approval`, `internal/audit`, and `internal/rolemgmt` are real packages (not stubs). The 10 seeded roles, the `menu_features` catalog (8 menus + 3 nested Settings features), and the `role_permissions` grants live in `002_baseline_seed.sql`. Admin endpoints (`/api/v1/admin/approval/*`, `/api/v1/admin/roles`) and `/api/v1/approvals/*` are mounted.

**Readiness:** ✅ **In production-intent use — treat as live auth code.** Any change here hits `project-context.md` Golden Rule #7 (auth = STOP-and-confirm danger zone). This document catalogs what is real, the deliberate asymmetries, and the known gaps — so changes are grounded in the as-built system, not the spec's aspirations.

---

## 2. Scope — the three axes

| Axis | Keyed on | Enforced where | Backing code |
| --- | --- | --- | --- |
| Role-string authz | JWT `role` claim (single string) | `RequireRoles(...)` per route in `cmd/api/main.go` | `pkg/middleware/rbac.go` |
| Permission catalog | `role_permissions` (role × menu_feature) | Frontend nav + Role Management UI only (not backend enforcement yet) | `internal/rolemgmt` (evaluator, service, repository) |
| Approval hierarchy | `users.supervisor_id` + `users.approval_level` | Inside `approval.Orchestrator` (not middleware) | `internal/approval`, `internal/audit` |

### Design principles (from `rbac-design.md`, non-negotiable)
- **Role vs Hierarchy are separate concerns.** Role = capability; `supervisor_id` + `approval_level` = reporting line. Never conflate them.
- **Pure tree hierarchy.** One direct supervisor per user (`supervisor_id`). Matrix/multi-supervisor is out of scope.
- **Threshold-driven levels.** `approval_policies` maps `(document_type, amount range) → required_level`, admin-configurable.
- **Block-leave fallback.** An approver on leave (`user_leaves` overlapping now) routes to an active delegate (`approval_delegations`), guarded `delegate != maker`.
- **maker != checker**; every transition writes `audit_logs` (who/what/before/after/ip); writes → primary; idempotent per `(document_type, document_id)`.

---

## 3. Current State (from code inspection)

| Component | Status | Evidence |
| --- | --- | --- |
| `pkg/middleware/rbac.go` (`RequireAuth` / `RequireRoles`) | ✅ Built | Role-string allow-list, case-insensitive; the real backend gate |
| `internal/approval` | ✅ Built | `orchestrator.go`, `chain.go`, `effective_approver.go`, `repository.go`, `store.go`, `admin.go`, `errors.go` + unit/integration/e2e tests |
| `internal/audit` | ✅ Built | `writer.go` append-only writer + `writer_test.go` |
| `internal/rolemgmt` | ✅ Built | `evaluator.go`, `service.go`, `repository.go`, `errors.go` + integration/middleware/topology tests |
| 10 roles seeded | ✅ Built | `002_baseline_seed.sql` `roles` |
| `menu_features` catalog (8 menus + 3 Settings features) | ✅ Built | `002_baseline_seed.sql` (ids 1-11) |
| `role_permissions` grants | ✅ Built | `002_baseline_seed.sql` |
| Hierarchy columns `users.supervisor_id` + `approval_level` | ✅ Built | Originally migrations `021`; folded into `001_baseline_schema.sql` at the 2026-09-18 squash |
| `approval_policies` / `approval_requests` / `approval_steps` / `approval_delegations` / `user_leaves` | ✅ Built | Originally `022`-`025`; folded into `001_baseline_schema.sql` |
| `audit_logs` + read indexes | ✅ Built | `001_baseline_schema.sql` + `006_audit_logs_read_indexes.sql` |
| Admin endpoints `/api/v1/admin/{approval,roles}` | ✅ Mounted | `cmd/api/main.go` |
| `/api/v1/approvals` (`RequireAuth`-only) | ✅ Mounted | Orchestrator enforces maker≠checker |
| Frontend "Manajemen Peran" screen + nav filtering | ✅ Built | `features` + `lib/config/navigation.ts` (`filterNavByRoles`) |
| `RequirePermission` middleware (DB-catalog backend enforcement) | ❌ Not built | No permission-evaluating middleware in the request chain; catalog drives frontend only |
| `backend-cit` role guards | ❌ Not built | `/health` + `RequireAuth`-only placeholder group |
| Planned roles (`ACM-*`, `CMOC-*`, `VENDOR-USER-CMOC`) | ❌ Not seeded | Recorded in matrix as planned; not in `roles`, `DbRole` union, or any guard |
| `Vendor CIT` / `Vendor CIT Supervisor` sub-roles | ❌ Spec-only | Mentioned in `project-context.md`; not in seed or `DbRole` union |

> **Migration-numbering note (important).** `rbac-design.md`, `RBAC-Setup/task.md`, and `project-context.md` all cite migrations `021`-`025` for the approval/hierarchy schema. Those files no longer exist under those numbers: the **2026-09-18 baseline squash** folded `001`-`040` into `001_baseline_schema.sql` + `002_baseline_seed.sql`, and the originals moved to `backend/migrations/archives/2026-09-18_pre-baseline/`. Current live migrations run `001`-`018`; the RBAC schema is now inside the baseline, not in a `021`-numbered file. Do not go looking for `021_users_hierarchy.sql` in the live migrations dir.

---

## 4. The 10 roles (seeded)

| id | Role (exact string) | Portal | Description |
|----|--------------------|--------|-------------|
| 1 | `ADMIN` | Internal | Super administrator — full system access |
| 2 | `ADMIN_PARAM` | Internal | Parameter admin — master data |
| 3 | `ATM-USER` | Internal | ATM ops — DSR upload, cash count, order entry |
| 4 | `ATM-SPV` | Internal | ATM supervisor — approve orders, review forecasts |
| 5 | `BRANCH-USER` | Internal | Branch ops staff |
| 6 | `BRANCH-SPV` | Internal | Branch supervisor |
| 7 | `BRANCH-ATM-USER` | Internal | Combined branch + ATM ops |
| 8 | `BRANCH-ATM-SPV` | Internal | Combined branch + ATM supervisor |
| 9 | `VENDOR-USER` | Vendor | CIT vendor user |
| 10 | `APPACCESS` | Internal | Account provisioning + RBAC CRUD-mapping/delegation config |

Role strings are the canonical identifiers — keep them verbatim (`ATM-USER`, not `atm_user`). Guards compare case-insensitively, but the seed strings are authoritative. New roles created after seeding start with **zero** feature access by design.

---

## 5. Data Model (as-built)

All in `001_baseline_schema.sql` (money = numeric, timestamps timestamptz):

- **`roles`** — `id, role, description`. No permission columns (permissions live in `role_permissions`).
- **`users`** — includes `role_id`, `supervisor_id` (self-FK, nullable, `CHECK supervisor_id <> id`), `approval_level int`, `vendor_id`, `vendor_branch_id`, `is_karyawan`, `auth_source`, `is_active`, plus the local-password-policy columns (`password_changed_at`, `must_change_password`, `failed_login_attempts`, `locked_until`).
- **`menu_features`** — catalog: `id`, `parent_id` (self-FK, NULL = menu, non-NULL = feature), `key` (unique, e.g. `settings.roles`), `label`, `kind` (menu|feature), `sort_order`, `is_active`.
- **`role_permissions`** — many-to-many `roles` × `menu_features`; row presence = grant. UNIQUE `(role_id, menu_feature_id)`, `granted_by` FK → `users`.
- **`approval_policies`** — `(document_type, min_amount, max_amount) → required_level`, `is_active`; anti-overlap per document_type range.
- **`approval_requests`** — one per `(document_type, document_id)`; `maker_id`, `amount`, `required_level`, `status`.
- **`approval_steps`** — per-level trail on a request; `step_level`, `assigned_approver_id`, `acted_by_id`, `status`, `acted_at`.
- **`approval_delegations`** — `from_user_id`, `to_user_id`, `start_at`, `end_at`, `reason` (leave fallback).
- **`user_leaves`** — `user_id`, `start_at`, `end_at`, `reason`.
- **`audit_logs`** — `actor_id`, `action`, `entity_type`, `entity_id`, `before` jsonb, `after` jsonb, `ip`, `created_at`; read indexes from `006`.

---

## 6. Approval flow (as-built, `approval.Orchestrator`)

```
Maker submit -> lookup approval_policies (document_type + amount -> required_level)
             -> create approval_request(pending) + generate steps L2..required_level from supervisor chain
             -> each step: if assigned approver on leave (user_leaves overlaps now), route to active delegate (approval_delegations), guard delegate != maker
             -> approve rises to next step / reject stops
             -> final step approved => request approved => document effect applies
```

Lifecycle: `draft -> pending -> (pending step advance) -> approved | rejected`.

**Integration contract for other modules** (do NOT build a second state machine):
1. Construct once at startup: `repo := approval.NewRepository(dbPool)`; `orch := approval.NewOrchestrator(repo, repo, repo, audit.NewWriter(dbPool), nil)`.
2. Submit: `orch.SubmitForApproval(ctx, makerID, "<document_type>", documentID, amount, ip)` → `(request, created bool, err)`. `created=false` → a request already exists (idempotent; map to 409).
3. Add an `approval_policies` row for your `document_type` or `SubmitForApproval` returns `ErrPolicyNotFound`.
4. Approve/Reject: `orch.Approve(...)` / `orch.Reject(...)`. Map `ErrNotAuthorized`→403, `ErrRequestNotPending`→409.
5. Apply your module's effect **only after** `request.Status == "approved"` — never optimistically at submit.
6. Hierarchy + leave/delegation + audit trail come for free. There is no event bus yet (poll the store or listen for the `final_approve` audit entry).

---

## 7. Enforcement map (backend vs frontend)

### Backend route guards (`cmd/api/main.go`) — the REAL enforcement
| Route prefix | Guard |
|---|---|
| `/api/v1/auth` | public |
| `/api/v1/admin/users` | APPACCESS, ADMIN, ADMIN_PARAM |
| `/api/v1/admin/vendors` | ADMIN, ADMIN_PARAM |
| `/api/v1/admin/atms` | ADMIN, ADMIN_PARAM |
| `/api/v1/admin/roles` | APPACCESS, ADMIN (self-guarded static to avoid bootstrap deadlock) |
| `/api/v1/admin/approval` | ADMIN, ADMIN_PARAM, APPACCESS |
| `/api/v1/audit-logs` | ADMIN, ADMIN_PARAM |
| `/api/v1/dmaa-forecast` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV, ADMIN, ADMIN_PARAM |
| `/api/v1/atm-portal` | `RequireAuth` only |
| `/api/v1/dsr` | `RequireAuth` only |
| `/api/v1/approvals` | `RequireAuth` only (orchestrator enforces maker≠checker) |
| `/api/v1/vendor-requests` | `RequireAuth` only (subset enforced in handler+service) |

### Frontend (CompanyPortal-Vite) — evaluated independently
- **Nav visibility** (`filterNavByRoles`): ADMIN + ADMIN_PARAM see every item (early return); `roles: ["*"]` = all authenticated; otherwise role must be in the item's list.
- **Route guards** (`requireRoles([...])` in `_protected.tsx` route files): e.g. `/settings/roles` = **APPACCESS only**; `/settings/admin/users` = **APPACCESS only** (stricter than backend); `/audit-logs`, `/eod-monitoring` = ADMIN, ADMIN_PARAM.
- **Settings hub cards** (`SettingsHubPage.tsx`): only "Manajemen Peran" is card-gated (APPACCESS or ADMIN); other cards rely on their route guard.
- VendorPortal-Vite has **no** centralized nav config equivalent.

---

## 8. Known asymmetries & gaps (source of truth)

Deliberate or to-be-fixed — enumerated so nobody "fixes" an intentional one or misses a real one:

1. **DB catalog is not backend enforcement.** `role_permissions` drives frontend nav + Role Management UI only. Backend still uses hardcoded `RequireRoles`. The role-management spec intends the catalog to replace the guards; that migration has **not** happened. Until it does, editing a role's grants in "Manajemen Peran" changes what the user *sees*, not what the backend *allows*.
2. **`/admin/users` asymmetry (deliberate).** Backend allows APPACCESS+ADMIN+ADMIN_PARAM; frontend route+card is APPACCESS-only.
3. **`RequireAuth`-only routes.** `dsr`, `atm-portal`, `vendor-requests`, `approvals` are not role-gated at the route. For `approvals` this is by design (hierarchy handles it). For the others it is a coarser-than-catalog gap — any authenticated user (incl. `VENDOR-USER`) reaches them at the HTTP layer; finer authz is absent or pushed to the service layer.
4. **Settings hub card leak.** ADMIN_PARAM sees non-role-management cards on the hub (only the route guard stops them). Documented gap.
5. **`backend-cit` has no role guards** — skeleton only.
6. **Role Management immediate-apply.** Deliberate, documented deviation from Golden Rule #3: create/update on `menu_features`/`role_permissions` apply **immediately, no maker-checker**. Substitute control = mandatory `audit_logs` write in the **same DB transaction** (audit failure rolls back the change) + re-checked APPACCESS/ADMIN authz at route and service layers.
7. **Planned-but-unseeded roles.** `ACM-USER`, `ACM-SPV`, `CMOC-USER`, `CMOC-SPV`, `CMOC-PIC`, `VENDOR-USER-CMOC` — planned, portal/description TBD, not in seed/`DbRole`/guards. `Vendor CIT` + `Vendor CIT Supervisor` are spec-only.
8. **`sqlc` regen blocked.** `sqlc generate` was blocked by a pre-existing bug in old migration 017 (missing table name); `internal/db/{audit,approval}.sql.go` were hand-written to match sqlc's output convention and should be regenerated once that is resolved. (Verify current state — this note predates the baseline squash.)
9. **Live-DB verification.** `APPACCESS` seeding + the local-password-policy columns were noted as **not verified against a live DB** in an earlier session (external Postgres unreachable). Confirm `SELECT * FROM roles WHERE role='APPACCESS'` and `\d users` before relying in production.

---

## 9. Open Questions (for RBAC evolution)

1. **Catalog-as-enforcement cutover** — when `role_permissions` becomes the backend authority, what replaces `RequireRoles`? A `RequirePermission(featureKey)` middleware reading the catalog? What is the cache/invalidation story (Redis? per-request DB read)?
2. **Coarse `RequireAuth`-only routes** — should `dsr` / `atm-portal` / `vendor-requests` get explicit role or permission gates, or is service-layer scoping the intended design?
3. **Planned roles** — business definition + portal + feature grants for `ACM-*` / `CMOC-*` / `VENDOR-USER-CMOC` before seeding.
4. **Vendor-side hierarchy** — approval hierarchy scope is "internal CIMB first"; vendor maker-checker (e.g. `Vendor CIT Supervisor`) is deferred. When does it land, and does it reuse the same `approval_policies`/chain machinery?
5. **Multi-supervisor** — pure-tree is a deliberate limitation. Confirm matrix hierarchy stays out of scope.
6. **Policy overlap validation** — confirm the anti-overlap constraint on `approval_policies` ranges is enforced at DB + service layer (not just documented).

---

## 10. Recommendations & Model per Change Type

RBAC is built; treat it as maintenance/evolution, not greenfield. Any change is auth-adjacent (Golden Rule #7 — STOP & confirm).

| Change type | Model | Effort | Reason |
| --- | --- | --- | --- |
| Add/seed a new role (+ `role_permissions` grants) | Sonnet | Medium | Seed + matrix update; mechanical but touches authz surface |
| Add a `menu_features` entry + nav wiring | Haiku | Low | Catalog row + nav config, well-specified |
| Change a `RequireRoles` guard on a route | Opus | High | Directly alters backend enforcement; wrong allow-list = access leak or lockout |
| Catalog-as-enforcement cutover (`RequirePermission`) | Opus | High | Replaces the real gate; cross-cutting, cache/invalidation, migration risk |
| Change approval chain / effective-approver / delegation logic | Opus | High | Maker-checker correctness; a wrong transition corrupts approval state |
| New `approval_policies` threshold row for a module | Sonnet | Medium | Config row + range validation |
| Wire an existing module into `approval.Orchestrator` | Sonnet | Medium | Follow the Sec 6 integration contract, known pattern |
| Update `rbac-*.md` / matrix docs after a change | Haiku | Low | Doc sync, mechanical |

---

## 11. Definition of Done (per project-context Sec 11) for any RBAC change

- [ ] Correct axis touched (role-string guard vs `role_permissions` catalog vs approval hierarchy) — do not conflate role with hierarchy
- [ ] Backend `RequireRoles` (or `RequirePermission` post-cutover) matches intended access; frontend nav/route/card guards kept consistent
- [ ] Maker-checker via `approval.Orchestrator` only — no second state machine; effect applied only after `approved`
- [ ] Every state change writes `audit_logs` (who/what/before/after/when/ip); Role Management audit write is in the same DB transaction as the mutation
- [ ] Writes → primary; reads/reporting → replica; read-after-write uses primary
- [ ] Exact role strings preserved verbatim; new roles start with zero grants
- [ ] `rbac-design.md` + `rbac-role-feature-matrix.md` + this assessment + `project-context.md` Sec 2/12 updated in sync
- [ ] Tests passing (auth path, RBAC denials, maker≠checker, delegation/leave fallback, chain resolution), coverage ≥80% on `internal/*`
- [ ] Builds cleanly in Docker
