# Implementation Plan: Admin User & Vendor Management

> Add / edit / disable-enable for **users** and **vendors** on the ATM backend (port 8080, flat JSON) then the CompanyPortal-Vite screens. Follows the ATM backend's technical-layer layout (handler/service/repository + sqlc). No new tables — reuses existing `users`/`vendors` columns; disable = soft-delete, never hard delete.
>
> **STOP-and-confirm gates (project-context Sec 3 rule 7).** Resolve the two Open Decisions in requirements.md *before* the tasks they affect:
> - **Task 0a** — which roles guard each screen (default: users→APPACCESS, vendors→ADMIN/ADMIN_PARAM).
> - **Task 0b** — maker-checker vs apply-immediately (default: apply-immediately + audit, matching existing account-provisioning actions).
> Anything touching the DB schema (the optional `030` search-index migration) or auth/RBAC is confirm-before-apply.

---

## Task 0 — Confirm Open Decisions (gate)

- [ ] **0a. Role guards.** Confirm with the team: `/admin/users` + `User_Admin_API` guarded by `APPACCESS`; `/admin/vendors` + `Vendor_Admin_API` guarded by `ADMIN`/`ADMIN_PARAM`. Record the decision here.
- [ ] **0b. Maker-checker.** Confirm whether user/vendor add/edit/disable apply immediately (audit only) or route through `internal/approval` (maker-checker). Default: apply immediately. Record the decision; if maker-checker is required, expand Tasks 4 & 7 per design.md "Alternative: maker-checker".
- [ ] **0c. Search-index migration.** Confirm whether `030_admin_search_indexes.sql` (+ `pg_trgm`) is applied now or deferred. Default: deferrable (ILIKE seq-scan is fine at current volume).
- **Done when:** all three decisions are written down; no downstream task starts on a guessed answer.
- _Requirements: Open Decisions 1–3; Reqs 1, 9_
- _Model: Opus, Effort: High — RBAC + maker-checker on auth-adjacent master data; a wrong call here ripples through every backend and frontend task._

## Task 1 — (Optional) Migration: admin search indexes

- [ ] IF Task 0c approves: create `backend/migrations/030_admin_search_indexes.sql` (additive, `BEGIN;…COMMIT;`, WHY/SAFETY header, all `IF NOT EXISTS`) — trigram GIN indexes on `users(full_name,username,email)` and `vendors(code,name)`; ensure `pg_trgm` is installed.
- [ ] **Test:** migration applies clean; `\d users`/`\d vendors` show the new indexes; `EXPLAIN` on an `ILIKE '%q%'` search uses a GIN index on a seeded set. (Prior specs note `DATABASE_URL` may point at an unreachable `host.docker.internal`; if so, mark as applied-and-verified pending a reachable Postgres.)
- **Demo:** search query plan avoids a full seq scan on a large seed.
- _Requirements: 2.2, 6.2; design "Optional migration 030"_
- _Model: Opus, Effort: High — a DB migration (schema change is a project-context STOP-and-confirm zone), even though additive._

## Task 2 — sqlc queries + repositories

- [ ] Add `backend/queries/users_admin.sql`: `ListUsersAdmin`, `CountUsersAdmin`, `GetUserAdminByID`, `CreateUserAdmin`, `UpdateUserAdmin`, plus `FindUserByEmail`/`FindUserByEmployeeID` (incl. soft-deleted) for 409 pre-checks. Project columns exclude `password_hash`. Filters via `sqlc.narg` (`q`, `role`, `vendor_id`, `status`); order `full_name ASC, id ASC`; `LIMIT`/`OFFSET` via `sqlc.arg`.
- [ ] Add `backend/queries/vendors_admin.sql`: `ListVendorsAdmin`, `CountVendorsAdmin`, `GetVendorAdminByID`, `CreateVendorAdmin`, `UpdateVendorAdmin`, `DisableVendor`, `EnableVendor`, `CountActiveUsersByVendor`.
- [ ] Run `sqlc generate` (`backend/sqlc.yaml`). Per the audit-log-viewer precedent, keep only the new `*.sql.go`, revert unrelated version-drift (`IP`→`Ip`), hand-fix casing to the checked-in convention. If generate is blocked by migration `017`, hand-write `internal/db/users_admin.sql.go`/`vendors_admin.sql.go` matching sqlc output. File a follow-up to pin sqlc version + fix `017`.
- [ ] Add `internal/repository/user_admin_repository.go` and `vendor_admin_repository.go` wrapping `*db.Queries` (pattern from `auth_repository.go`), `timestamptzToPtr` at the boundary, `dbPool` (primary) with `ponytail:` replica TODO on list methods. Never return `password_hash`.
- [ ] **Test (integration, `//go:build integration`, real Postgres):** create→get roundtrip; filter matrix (`q`, role, vendor_id, status active/disabled/all); count vs summed pages; unique violation surfaces; `DisableVendor`/`EnableVendor` toggle `is_active`+`deleted_at`; `CountActiveUsersByVendor`. Build + `go vet -tags integration` clean even if a live DB is unreachable this session (note it, as prior specs did).
- **Demo:** repository test prints a filtered page + total for a seeded dataset.
- _Requirements: 2.1–2.10, 3.5, 4.5, 6.1–6.7, 7.3, 8.5, 14.4_
- _Model: Sonnet, Effort: Medium — sqlc queries + repositories over two existing tables following the established repository pattern._

## Task 3 — Vendor admin service (validation + audit)

- [ ] Add `internal/service/vendor_admin.go`: `VendorAdminService` with `Create`, `Update`, `Disable`, `Enable`. Validate `code`/`name` required, `contact_email` format when present; reject `code` change on update (400); 404 on missing id; 409 on code conflict (pre-check + constraint fallback). `Disable` returns a `LinkedUsersWarning` count via `CountActiveUsersByVendor`. Each success writes `audit.Entry` (`vendor_created`/`vendor_updated`/`vendor_deactivated`/`vendor_reactivated`) with before/after, actor, IP through the shared `auditWriter`.
- [ ] **Test (table-driven, faked repo + faked audit writer):** validation matrix; immutable-code guard; 404/409 mapping; audit-write-called-exactly-once per success; audit failure ⇒ error surfaced (no silent mutation); linked-users warning surfaced on disable.
- **Demo:** unit test showing validation + audit-once matrix green.
- _Requirements: 6.7, 7.1–7.8, 8.1–8.6, 9.1–9.5_
- _Model: Sonnet, Effort: Medium — CRUD service with validation + audit wiring in known patterns (no money/approval state machine at the default decision)._

## Task 4 — User admin service (create/update + wire disable/enable)

- [ ] Add `internal/auth/user_admin.go` (beside `DeactivateUserService`/`SetInitialPasswordService`): `UserAdminService` with `Create` and `Update`. Enforce: required fields + email format; `auth_source` rules (local ⇒ vendor_id required + no password; ldap ⇒ vendor_id absent + no password_hash); role text→role_id resolution; vendor/supervisor reference checks; self-supervision guard (400); immutable `username`/`auth_source` on update (400); 404 on missing id; 409 on email/employee_id conflict. Audit `user_created`/`user_updated` with **sanitized** before/after (never `password_hash`), actor, IP.
- [ ] Do NOT reimplement disable/enable — the handler (Task 5) calls the existing `DeactivateUserService.Deactivate/Reactivate`.
- [ ] **Test (table-driven, faked repo + faked audit writer):** auth_source conditional matrix; immutable-field rejection; self-supervision; reference-resolution failures → 400; 409 conflicts; audit-once + audit-failure-surfaces; **password material absent from audit payload** (assert on the marshalled entry).
- **Demo:** unit test matrix green incl. the "no password in audit" assertion.
- _Requirements: 3.1–3.10, 4.1–4.9, 5.1–5.7, 9.1–9.5_
- _Model: Opus, Effort: High — user provisioning touches auth (auth_source/role/vendor linkage) + the audit-integrity guarantee; a wrong rule leaks a bad login path or an unaudited change._

## Task 5 — HTTP handlers (flat JSON) + route wiring

- [ ] Extend `internal/handler/admin_user_handler.go`: inject `UserAdminService` + `DeactivateUserService` alongside the existing `SetInitialPasswordService`; add `List`, `Get`, `Create`, `Update`, `Disable`, `Enable` per design's `Routes()` list. Actor from `middleware.GetAuthContext`, IP from `extractClientIP`, id from `parsePathID`. Self-disable guard (400 when target == actor). Map errors → status per design's table using `writeJSON`/`writeError`/`writeValidationError`. Responses omit `password_hash`.
- [ ] Add `internal/handler/admin_vendor_handler.go`: `Routes()` with list/get/create/update/disable/enable; same helpers + error mapping; include the linked-users warning in the disable response.
- [ ] Wire both in `cmd/api/main.go` per design's snippet: extend the `APPACCESS` `/api/v1/admin/users` mount (now including the real deactivate service — previously unwired) and add the `/api/v1/admin/vendors` mount behind the confirmed vendor roles. Keep the `ponytail:` replica TODO convention.
- [ ] **Test (httptest, real `RequireAuth`/`RequireRoles`):** 200/201 happy paths + flat-JSON shape (no `password_hash`); param parsing; 400/404/409/422 mapping; 401 no token; 403 wrong role (pattern from `admin_approval_handler_test.go`); self-disable 400. Full backend suite (`go build ./...`, `go test ./...`) green.
- **Demo:** curl create/list/edit/disable/enable as the guarded role returns flat JSON; wrong role → 403.
- _Requirements: 1.7–1.10, 2.1, 2.8, 3.1, 3.8, 4.1, 4.8, 5.1–5.7, 6.1, 6.6, 7.1, 7.5, 8.1–8.6, 14.4, 14.5_
- _Model: Sonnet, Effort: Medium — HTTP handlers + route wiring applying existing middleware and helper conventions; by-the-book RBAC, not novel auth logic._

## Task 6 — Backend quality gate + no-hard-delete guard

- [ ] Extend `internal/repository/no_hard_delete_test.go` (or add a sibling) to assert no `DELETE FROM vendors` path exists, matching the existing users guarantee (Req 14.6).
- [ ] `sqlc generate` (same drift-revert discipline as Task 2), `go build ./...`, `go test ./...`, `golangci-lint run ./...` — zero new lint issues in files this spec touches.
- [ ] Confirm no hard-`DELETE` route on either admin mount; disable/enable are the only lifecycle transitions.
- **Test:** full backend suite green; coverage ≥ 80% on the new service/handler files (integration-gated repo tests noted if DB unreachable).
- _Requirements: 5.3, 8.3, 14.6; DoD_
- _Model: Sonnet, Effort: Medium — backend quality gate needing judgment to triage failures._

## Task 7 — Shared Dialog primitive + frontend scaffolding

- [ ] Add `src/components/ui/Dialog.tsx`: accessible modal (`role="dialog"`, `aria-modal`, focus moves in on open, focus trap, Escape + outside-click close, focus returns to trigger, `aria-live` region). Transition transform+opacity only, ~200ms ease-out enter / exit ~75% (design-system Sec 9).
- [ ] If Task 0a guards `/admin/users` with `APPACCESS`, add `"APPACCESS"` to the `DbRole` union in `lib/auth/store.ts` (Req 1.11).
- [ ] Create `src/features/admin-users/{types.ts,api.ts,hooks.ts}` and `src/features/admin-vendors/{types.ts,api.ts,hooks.ts}`: typed request/response shapes matching the flat-JSON backend; TanStack Query hooks (`useUsersList`/`useUser`/`useCreateUser`/`useUpdateUser`/`useDisableUser`/`useEnableUser` and the vendor equivalents) over the existing `api` client, list hooks with `keepPreviousData`, namespaced query keys, mutations invalidating the list key.
- [ ] Register routes `src/routes/admin.users.tsx` and `admin.vendors.tsx` under `protectedRoute` with `requireRoles([...])`, wired into the route tree, each with a minimal page component to make the route real (fleshed out in Tasks 8–9).
- [ ] **Test:** `requireRoles` allow/deny/redirect (pattern from `audit-logs.test.ts`); Dialog focus trap + Escape close + focus return; query-key equality/uniqueness.
- _Requirements: 1.1–1.6, 1.11, 11.7, 14.1–14.3_
- _Model: Sonnet, Effort: Medium — new accessible Dialog + feature scaffolding/hooks/routes following established patterns; the a11y dialog is the only fiddly part._

## Task 8 — Users screen (table, filters, form)

- [ ] `AdminUsersPage.tsx` + `UserFilterBar.tsx` + `UsersTable.tsx`: PageHeader "Manajemen Pengguna" + "Tambah Pengguna"; DataTable columns (Nama, Username, Email, Role badge, Vendor, Status badge icon+label, Aksi); search (`q`) + Role/Vendor/Status FilterSelects synced to URL, resetting to page 1 on change; pagination from `{page,page_size,total}`; loading skeleton, EmptyState, inline error + retry, success/failure Toast.
- [ ] `UserFormDialog.tsx`: RHF + Zod with the auth_source conditional (`superRefine`: local ⇒ vendor_id required); create vs edit mode (disable Username + Auth Source in edit); map server 422/409 to field errors via `setError`, keep dialog open on error; close + invalidate list on success. Disable/Enable row action with confirmation.
- [ ] **Test:** filter change resets to page 1 + refetches with correct params; URL round-trip reconstructs filters; Zod auth_source conditional; server 409/422 → field error and dialog stays open; status badge carries icon+label; disable confirmation flow.
- _Requirements: 10.1–10.11, 11.1–11.7, 13.3, 13.4, 13.7_
- _Model: Sonnet, Effort: Medium — data table + conditional form with URL sync and server-error mapping; standard React feature work._

## Task 9 — Vendors screen (table, filters, form)

- [ ] `AdminVendorsPage.tsx` + `VendorFilterBar.tsx` + `VendorsTable.tsx` + `VendorFormDialog.tsx`: PageHeader "Manajemen Vendor" + "Tambah Vendor"; DataTable columns (Kode, Nama, Email Kontak, Telepon, Status badge, Aksi); search (`q`) + Status FilterSelect synced to URL; RHF+Zod form (Kode disabled in edit); 409 code-conflict → Kode field error; disable confirmation surfacing the linked-active-users warning (Req 8.5) in the Toast/confirm; loading/empty/error/toast states.
- [ ] **Test:** parallel to Task 8 for vendors; immutable Kode in edit mode; linked-users warning displayed on disable.
- _Requirements: 12.1–12.7, 13.3, 13.4, 13.7_
- _Model: Sonnet, Effort: Medium — mirrors the users screen with simpler fields; standard React feature work._

## Task 10 — Frontend quality gate + navigation

- [ ] Add "Manajemen Pengguna" and "Manajemen Vendor" entries to the internal app sidebar/nav, each visible only to the confirmed guarding role (match the existing role-gated nav pattern).
- [ ] `pnpm lint`, `pnpm test --run`, `pnpm build` for CompanyPortal-Vite.
- [ ] Verify OKLCH "Merah Sirih" tokens only (no hardcoded hex/rgb), one primary action per view, 44px touch targets, semantic landmarks, badges icon+label.
- **Test:** frontend suite green; a11y assertions pass; coverage ≥ 80% on both feature modules.
- _Requirements: 13.1–13.7; DoD_
- _Model: Sonnet, Effort: Medium — role-gated nav + frontend quality gate needing failure triage._

## Task 11 — Docs

- [ ] Note in `project-context.md` that `users`/`vendors` now have admin CRUD endpoints (paths + guarding roles) and that disable is soft-delete (no hard delete, guard test extended to vendors). If `030` was applied, record the new indexes; if maker-checker was chosen (Task 0b), document the integration.
- [ ] Short note in the spec folder recording the resolved Open Decisions and any follow-ups (pin sqlc version, fix migration `017`).
- _Requirements: Open Decisions 1–3; DoD_
- _Model: Haiku, Effort: Low — doc updates only, no code, low judgment._

---

## Sequencing & Dependencies

- **Task 0 is a hard gate.** Resolve the three decisions before any code task.
- Backend Tasks 2→6 land before the frontend integrates against real endpoints. Tasks 7→10 can be scaffolded against the design's response shapes in parallel but must be verified against the live API before Task 10's gate.
- Task 4 (user create/update) and the wiring of the previously-unwired `DeactivateUserService` (Task 5) are the auth-sensitive core — review carefully.
- If maker-checker (Task 0b) is chosen, Tasks 3, 4, 8, 9 grow to handle pending-change storage + apply-on-approve, per design.md's alternative.

## Definition of Done (project-context Sec 11)

- [ ] Matches module/table map — no new tables (optional additive `030` only, approved first)
- [ ] Correct auth path + scoped RBAC at middleware AND route guard (roles per Task 0a)
- [ ] Audit written for every create/update/disable/enable; maker-checker decision (Task 0b) resolved and reflected
- [ ] Reads on `dbPool` with replica TODO; writes on primary
- [ ] No `password_hash` in any response or audit payload; timestamps timestamptz UTC, shown Asia/Jakarta
- [ ] No hard delete for users or vendors (guard test extended to vendors)
- [ ] Tests green incl. RBAC denial (401/403), validation, uniqueness (409), audit-once, self-disable guard
- [ ] No secrets/config hardcoded; ATM backend keeps flat JSON shape
- [ ] Builds clean (`go build`, `sqlc generate` or hand-written equivalent, `pnpm build`)
