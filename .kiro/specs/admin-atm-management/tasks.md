# Implementation Plan: Admin ATM Management

> Add / edit / disable-enable for **ATMs** (`atms` master table) on the ATM backend (port 8080, flat JSON) then the CompanyPortal-Vite screen. Follows the ATM backend's technical-layer layout (handler/service/repository + sqlc) and mirrors the sibling spec `admin-user-vendor-management`. No new tables — reuses existing `atms` columns; disable = soft-delete, never hard delete. Does NOT modify the read-only `atm-portal` monitoring viewer.
>
> **Inherited decisions (from `admin-user-vendor-management`, confirmed 2026-09-11):**
> 1. **No maker-checker** — apply immediately + mandatory audit.
> 2. **Soft-disable, never hard delete** — guard test extended to `atms`.
> 3. No auth/password concerns (ATMs carry no credentials).
>
> **Role guard — CONFIRMED (2026-09-15):** `/api/v1/admin/atms` + `/admin/atms` guarded by `ADMIN`/`ADMIN_PARAM`; the screen is surfaced as a card in the **Settings hub** (`SettingsHubPage`), not a standalone sidebar item.
>
> **Dependency:** the frontend `Dialog` primitive (`src/components/ui/Dialog.tsx`) comes from the sibling spec. If that spec has not landed, build `Dialog` first (Task 6 note).

---

## Task 0 — Optional search-index decision

- [x] **Role guard — CONFIRMED (2026-09-15):** `/api/v1/admin/atms` and `/admin/atms` guarded by `ADMIN`, `ADMIN_PARAM`; screen surfaced as a Settings-hub card. No longer an open gate.
- [ ] **Search index (optional).** Decide whether to add a trigram index on `atms(terminal_id)` now or defer. Default: deferrable — `ILIKE '%q%'` seq-scan is fine at current volume. Record the decision.
- **Done when:** the search-index decision is written down.
- _Requirements: 2.2_
- _Model: Sonnet, Effort: Low — one scoped decision; the role guard is confirmed and the rest is inherited from the sibling spec._

## Task 1 — (Optional) Migration: ATM search index

- [ ] IF Task 0 approves: create `backend/migrations/031_admin_atm_search_index.sql` (additive, `BEGIN;…COMMIT;`, WHY/SAFETY header, `IF NOT EXISTS`) — trigram GIN index on `atms(terminal_id)`; ensure `pg_trgm` is installed.
- [ ] **Test:** migration applies clean; `\d atms` shows the index; `EXPLAIN` on `ILIKE '%q%'` uses the GIN index on a seeded set. (If `DATABASE_URL` is unreachable, mark applied-and-verified pending a reachable Postgres, as prior specs did.)
- **Demo:** search query plan avoids a full seq scan on a large seed.
- _Requirements: 2.2; design "optional additive index"_
- _Model: Opus, Effort: High — a DB migration (schema change is a project-context STOP-and-confirm zone), even though additive._

## Task 2 — sqlc queries + repository

- [ ] Add `backend/queries/atms_admin.sql`: `ListATMsAdmin`, `CountATMsAdmin`, `GetATMAdminByID`, `CreateATMAdmin`, `UpdateATMAdmin`, `DisableATM`, `EnableATM`, `FindATMByTerminalID` (incl. soft-deleted, for 409 pre-check), `ListLocationsForSelect`, `LocationExists`. Filters via `sqlc.narg` (`q`, `brand`, `machine_type`, `deployment_type`, `priority_class`, `location_id`, `status`); LEFT JOIN `locations` for `location_name`; order `terminal_id ASC, id ASC`; `LIMIT`/`OFFSET` via `sqlc.arg`. Money columns projected as `numeric` (handled as decimal string at the repo boundary, never float).
- [ ] Run `sqlc generate` (`backend/sqlc.yaml`). Per the established precedent, keep only the new `atms_admin.sql.go`, revert unrelated version-drift (`IP`→`Ip`), hand-fix casing to the checked-in convention. If generate is blocked by migration `017`, hand-write `internal/db/atms_admin.sql.go` matching sqlc output.
- [ ] Add `internal/repository/atm_admin_repository.go` wrapping `*db.Queries` (pattern from `auth_repository.go` and the sibling spec's `user_admin_repository.go`), `timestamptzToPtr` + numeric→string at the boundary, `dbPool` (primary) with `ponytail:` replica TODO on list/read/locations methods.
- [ ] **Test (integration, `//go:build integration`, real Postgres):** create→get roundtrip; filter matrix (`q`, brand, machine_type, deployment_type, priority_class, location_id, status active/disabled/all); count vs summed pages; `terminal_id` unique violation surfaces; `DisableATM`/`EnableATM` toggle `is_active`+`deleted_at`; `LocationExists` true/false; `ListLocationsForSelect` ordering. Build + `go vet -tags integration` clean even if a live DB is unreachable this session (note it).
- **Demo:** repository test prints a filtered page + total for a seeded dataset.
- _Requirements: 2.1–2.9, 3.4, 4.3, 5.1–5.2, 6.1–6.2, 11.4, 11.5_
- _Model: Sonnet, Effort: Medium — sqlc queries + repository over one existing table following the established repository pattern._

## Task 3 — ATM admin service (validation + reference checks + audit)

- [ ] Add `internal/service/atm_admin.go`: `ATMAdminService` with `Create`, `Update`, `Disable`, `Enable`, `ListLocations`. Validate required text fields; `priority_class` (when present) ∈ {`VIP`,`Non VIP`,`Industri`}; each monetary amount parses as a non-negative exact decimal (reject non-numeric/negative → 422); reject `terminal_id` change on update (400); 404 on missing id; 400 `invalid_reference` when `location_id` is absent from `locations`; 409 on `terminal_id` conflict (pre-check + `23505` constraint fallback on `atms_terminal_id_uq`). Each success writes `audit.Entry` (`atm_created`/`atm_updated`/`atm_deactivated`/`atm_reactivated`) with before/after, actor, IP through the shared `auditWriter`. `ListLocations` writes no audit.
- [ ] **Test (table-driven, faked repo + faked audit writer):** validation matrix (required, enum, amount non-negative/decimal); immutable-`terminal_id` guard; location reference 400; 404/409 mapping; audit-write-called-exactly-once per success; audit failure ⇒ error surfaced (no silent mutation).
- **Demo:** unit test showing validation + reference + audit-once matrix green.
- _Requirements: 3.1–3.9, 4.1–4.7, 5.1–5.6, 6.1–6.4, 7.1–7.4_
- _Model: Sonnet, Effort: Medium — CRUD service with validation, enum + decimal parsing, reference resolution, and audit wiring in known patterns (no money math, no auth, no state machine)._

## Task 4 — HTTP handler (flat JSON)

- [ ] Add `internal/handler/admin_atm_handler.go`: `AdminATMHandler` injecting `ATMAdminService`; `Routes()` with `List`, `Create`, `ListLocations`, `Get`, `Update`, `Disable`, `Enable` per design's route list. Register static `/locations` before `/{id}` (chi ordering note). Actor from `middleware.GetAuthContext`, IP from `extractClientIP`, id from `parsePathID`. Flat-JSON DTOs mirror `atm_portal_handler.go` (amounts as `*string`/decimal, timestamps RFC3339 pointers). Map errors → status per design's table using `writeJSON`/`writeError`/`writeValidationError`.
- [ ] **Test (httptest, real `RequireAuth`/`RequireRoles`):** 200/201 happy paths + flat-JSON shape (amounts as strings); param parsing; 400/404/409/422 mapping; 401 no token; 403 wrong role (pattern from `admin_approval_handler_test.go`); `/locations` returns options; static-vs-param ordering resolves correctly.
- **Demo:** curl create/list/edit/disable/enable + `/locations` as the guarded role returns flat JSON; wrong role → 403.
- _Requirements: 1.6–1.7, 2.1, 3.1, 3.7, 4.1, 4.6, 5.1–5.6, 6.1, 8.x support_
- _Model: Sonnet, Effort: Medium — HTTP handler applying existing middleware and helper conventions; by-the-book RBAC and error mapping._

## Task 5 — Route wiring + backend quality gate

- [ ] Wire in `cmd/api/main.go` per design's snippet: construct `atmAdminRepo`/`atmAdminSvc`/`adminATMHandler`, mount `/api/v1/admin/atms` behind `RequireAuth` + the confirmed roles (default `RequireRoles("ADMIN","ADMIN_PARAM")`). Keep the `ponytail:` replica TODO convention. Do NOT touch the existing `/api/v1/atm-portal` mount.
- [ ] Extend `internal/repository/no_hard_delete_test.go` (or a sibling) to assert no `DELETE FROM atms` path exists, matching the users/vendors guarantee (Req 11.6).
- [ ] `sqlc generate` (same drift-revert discipline as Task 2), `go build ./...`, `go test ./...`, `golangci-lint run ./...` — zero new lint issues in files this spec touches. Confirm no hard-`DELETE` route on the admin mount.
- **Test:** full backend suite green; coverage ≥ 80% on the new service/handler files (integration-gated repo tests noted if DB unreachable).
- _Requirements: 1.5, 5.3, 11.4–11.6; DoD_
- _Model: Sonnet, Effort: Medium — route wiring + backend quality gate needing judgment to triage failures._

## Task 6 — Frontend feature scaffolding (types/api/hooks/route)

- [ ] Ensure `src/components/ui/Dialog.tsx` exists (from `admin-user-vendor-management`). IF it has not landed, build it first: accessible modal (`role="dialog"`, `aria-modal`, focus trap, Escape + outside-click close, focus return, `aria-live`), transition transform+opacity ~200ms ease-out / exit ~75% (design-system Sec 9).
- [ ] Create `src/features/admin-atms/{types.ts,api.ts,hooks.ts}`: typed request/response shapes matching the flat-JSON backend (amounts as strings, nullable fields); thin `api.ts` wrappers over the existing `api` client; TanStack Query hooks (`useATMsList` with `keepPreviousData`, `useATM`, `useLocationOptions` with long `staleTime`, `useCreateATM`/`useUpdateATM`/`useDisableATM`/`useEnableATM`); namespaced `adminATMsKeys` (list/detail/locations); mutations invalidate the list key.
- [ ] Register route `src/routes/admin.atms.tsx` under `protectedRoute` with `requireRoles(["ADMIN","ADMIN_PARAM"])` (confirmed guard), wired into the route tree, with a minimal page component (fleshed out in Tasks 7–8).
- [ ] **Test:** `requireRoles` allow/deny/redirect (pattern from `audit-logs.test.ts`); query-key equality/uniqueness; (Dialog focus trap + Escape only if built here).
- _Requirements: 1.1–1.4, 9.7, 11.1–11.3_
- _Model: Sonnet, Effort: Medium — feature scaffolding/hooks/route following established patterns (the a11y Dialog, if built here, is the only fiddly part)._

## Task 7 — ATMs screen (table + filters)

- [ ] `AdminATMsPage.tsx` + `ATMFilterBar.tsx` + `ATMsTable.tsx`: PageHeader "Manajemen ATM" + "Tambah ATM"; DataTable columns (Terminal ID, Lokasi, Tipe Mesin, Brand, Deployment, Prioritas badge, Status badge icon+label, Aksi); search (`q`) + Brand/Tipe Mesin/Deployment/Prioritas/Status FilterSelects synced to URL, resetting to page 1 on change; pagination from `{page,page_size,total}`; loading skeleton, EmptyState, inline error + retry, success/failure Toast; amount columns `tabular-nums` right-aligned; disable/enable row action with confirmation.
- [ ] **Test:** filter change resets to page 1 + refetches with correct params; URL round-trip reconstructs filters; Status + Prioritas badges carry icon+label; money columns `tabular-nums`; disable confirmation flow.
- _Requirements: 8.1–8.11, 10.1–10.7_
- _Model: Sonnet, Effort: Medium — data table + filters with URL sync; standard React feature work._

## Task 8 — ATM form dialog

- [ ] `ATMFormDialog.tsx`: RHF + Zod — required text fields; `priority_class` enum select (VIP / Non VIP / Industri); monetary fields as decimal-string inputs validated non-negative, IDR-formatted with `tabular-nums`, submitted as strings; Location select from `useLocationOptions()`. Create vs edit mode (disable Terminal ID in edit). Map server 422/409/400-invalid_reference to field errors via `setError` (409 → Terminal ID, invalid_reference → Lokasi), keep dialog open on error; close + invalidate list on success.
- [ ] **Test:** Zod required/enum/amount/immutable-terminal_id rules; server 409/422/invalid_reference → correct field error and dialog stays open; Terminal ID disabled in edit mode; amount inputs accept/display decimals without float drift; Location select populated from options.
- _Requirements: 9.1–9.7, 10.4, 10.7_
- _Model: Sonnet, Effort: Medium — conditional form with decimal money handling and server-error mapping; standard React feature work._

## Task 9 — Settings hub card + frontend quality gate

- [ ] Surface the ATM screen as a **card in the Settings hub** (`src/features/rbac-settings/components/SettingsHubPage.tsx`), NOT a standalone `NAV_CONFIG` sidebar item (confirmed 2026-09-15). Add a "Manajemen ATM" card (`href: /admin/atms`, icon e.g. `Landmark`/`Server`) alongside the sibling spec's "Manajemen Pengguna"/"Manajemen Vendor" cards. The hub is reached from the existing "Pengaturan" nav item, guarded for `ADMIN`/`ADMIN_PARAM`; the `admin.atms` route guard is `requireRoles(["ADMIN","ADMIN_PARAM"])`. Do NOT add a top-level `NAV_CONFIG` entry.
- [ ] `pnpm lint`, `pnpm test --run`, `pnpm build` for CompanyPortal-Vite.
- [ ] Verify OKLCH "Merah Sirih" tokens only (no hardcoded hex/rgb), one primary action per view, 44px touch targets, semantic landmarks, badges icon+label, amounts right-aligned `tabular-nums`.
- **Test:** frontend suite green; a11y assertions pass; coverage ≥ 80% on the feature module.
- _Requirements: 10.1–10.7, 11.1; DoD_
- _Model: Sonnet, Effort: Medium — role-gated nav + frontend quality gate needing failure triage._

## Task 10 — Docs

- [ ] Note in `project-context.md` that `atms` now has admin CRUD endpoints (`/api/v1/admin/atms`, guarding role per Task 0), that disable is soft-delete (no hard delete, guard test extended to `atms`), and that the read-only `atm-portal` viewer is unchanged. If the `031` index was applied, record it.
- [ ] Short note in the spec folder recording any follow-ups.
- _Requirements: DoD_
- _Model: Haiku, Effort: Low — doc updates only, no code, low judgment._

---

## Sequencing & Dependencies

- Task 0 (role guard) gates Task 5 (route wiring) and Task 6 (route guard). The optional `031` index (Task 1) blocks nothing.
- Backend Tasks 2→5 land before the frontend integrates against real endpoints. Tasks 6→9 can be scaffolded against the design's response shapes in parallel but must be verified against the live API before Task 9's gate.
- The frontend `Dialog` primitive is shared with `admin-user-vendor-management`; if that spec lands first, reuse it — otherwise build it in Task 6.
- This spec is independent of the user/vendor spec at the code level (separate handler/service/repo/query files, separate feature folder) and can proceed in parallel.

## Definition of Done (project-context Sec 11)

- [ ] Matches module/table map — no new tables (optional additive `031` only, approved first); `atms` is canonical
- [ ] Correct auth path + scoped RBAC at middleware AND route guard (role per confirmed Task 0 decision; default `ADMIN`/`ADMIN_PARAM`)
- [ ] Audit written for every create/update/disable/enable; no maker-checker
- [ ] Reads on `dbPool` with replica TODO; writes on primary
- [ ] Money fields as `numeric`/decimal-string, never float; timestamps timestamptz UTC, shown Asia/Jakarta
- [ ] No hard delete for ATMs (guard test extended to `atms`)
- [ ] Existing `atm-portal` read stack untouched
- [ ] Tests green incl. RBAC denial (401/403), validation, uniqueness (409), invalid_reference (400), audit-once
- [ ] No secrets/config hardcoded; ATM backend keeps flat JSON shape
- [ ] Builds clean (`go build`, `sqlc generate` or hand-written equivalent, `pnpm build`)
