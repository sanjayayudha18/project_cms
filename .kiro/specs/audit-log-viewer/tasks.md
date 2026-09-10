# Implementation Plan: Audit Log Viewer

> Read-only viewer over the generic `audit_logs` trail. Backend-first (ATM backend, port 8080, flat JSON) then the CompanyPortal-Vite page. Follows the ATM backend's actual technical-layer layout (handler/service/repository + sqlc). Access: `ADMIN` / `ADMIN_PARAM`.
>
> **STOP-and-confirm gates (project-context Sec 3):** anything touching the `audit_logs` table schema, auth, or RBAC is a confirm-before-migrate step. This spec does NOT write audit entries (read-only) — but it may create the `audit_logs` table if RBAC-Setup Task 3 has not, which is a schema change requiring the usual project-context Sec 2 proposal + approval.

---

## Task 0 — Reconcile the `audit_logs` table with RBAC-Setup (gate)

- [ ] Check migrations for an existing `audit_logs` table (RBAC-Setup Task 3, target `023_audit_logs.sql`).
  - **If it exists:** reuse it as the source of truth. Skip Task 1's table creation; keep only the read-path index migration.
  - **If it does not exist:** confirm with the owner that this spec may create it (identical columns to RBAC-Setup's design so the two specs converge on one table). Reserve the next free migration number at implementation time.
- [ ] Confirm `audit_logs` is already listed as a canonical Core table in `project-context.md` Sec 2 (it is pre-approved by name). No new table proposal needed for the base table; the read-path indexes are additive.
- **Done when:** it is unambiguous whether Task 1 creates the table or only adds indexes, and the migration number is agreed.
- _Requirements: dependency note; Req 4.3_
- _Model: Opus — a schema-ownership gate on the shared `audit_logs` table; getting the create-vs-index decision wrong forks the audit table across two specs._

## Task 1 — Migration: `audit_logs` (if absent) + read-path indexes

- [ ] Create `backend/migrations/0XX_audit_logs_read_indexes.sql` (additive, style of `016`: `BEGIN; ... COMMIT;` + WHY/SAFETY header):
  - If Task 0 determined the table is absent, this migration also creates the append-only `audit_logs` table exactly per design.md (bigserial id, actor_id, action, entity_type, entity_id, before/after jsonb, ip, created_at timestamptz default now()).
  - Add indexes: `(created_at DESC, id DESC)`, `(actor_id)`, `(entity_type, entity_id)`, `(action)` — all `IF NOT EXISTS`.
- [ ] **Test:** migration up/down clean; `\d audit_logs` shows the four indexes; a seeded set of rows returns in `created_at DESC, id DESC` order.
- **Demo:** `EXPLAIN` on a filtered+ordered query shows an index used, not a full seq scan on a large seed.
- _Requirements: 2.8, 2.11; design "audit_logs schema"_
- _Model: Opus — a DB migration that may create the append-only `audit_logs` table; schema changes to the audit trail are a project-context STOP-and-confirm zone._

## Task 2 — sqlc queries + repository (read-only)

- [ ] Add `backend/queries/audit_log.sql`: `ListAuditLogs`, `CountAuditLogs`, `GetAuditLogByID` with `sqlc.narg` nullable filters (actor_id, action, entity_type, entity_id, date_from, date_to) + `LIMIT`/`OFFSET`, per design.md.
- [ ] Run `sqlc generate` (config `backend/sqlc.yaml`). If mixed positional/named args are rejected, switch limit/offset to `sqlc.arg('limit')`/`sqlc.arg('offset')` and regenerate.
- [ ] Add `internal/repository/audit_log_repository.go` wrapping `*db.Queries`, mapping db rows → domain structs (pattern from `auth_repository.go`). Map `before`/`after` to `json.RawMessage`. Use `dbPool` with a `// TODO(ponytail): route to replica` comment.
- [ ] **Test (integration, real Postgres):** seed rows; assert each single filter, combined filters (entity_type+entity_id = one entity's history), date-range boundaries (inclusive), ordering, and `Count` vs page consistency.
- **Demo:** repository test prints a filtered page + total for a seeded dataset.
- _Requirements: 2.1–2.9, 3.1, 3.4_
- _Model: Sonnet — sqlc queries + read-only repository over one table following the existing repository pattern._

## Task 3 — Read service (validation, pagination, mapping)

- [ ] Add `internal/service/audit_log_read.go`: `ListAuditLogsParams`, page/size clamp (`[1,100]`, default 25; page ≥ 1), offset compute; `date_from > date_to` → `ValidationError` (reuse type from `atm_portal.go`); assemble `{items, page, page_size, total}`; map `before`/`after` for detail.
- [ ] **Test (table-driven):** page_size clamp (0, negative, 101 → clamped); default when absent; page floor at 1; date-range invalid → ValidationError; before/after preserved as JSON (not stringified).
- **Demo:** unit test showing clamp + validation matrix green.
- _Requirements: 2.7, 2.10, 3.4_
- _Model: Sonnet — read service with pagination clamp + date-range validation; well-specified logic within known patterns._

## Task 4 — HTTP handler (flat JSON) + route wiring

- [ ] Add `internal/handler/audit_log_handler.go`: `Routes()` with `GET /` (list) and `GET /{id}` (detail); parse query params; convert `date_from`/`date_to` (`YYYY-MM-DD` or RFC3339, Asia/Jakarta → UTC); flat JSON via `writeJSON`/`writeError`; 400/404 mapping per design.md error table.
- [ ] Wire in `cmd/api/main.go`: `r.With(mw.RequireAuth, mw.RequireRoles("ADMIN","ADMIN_PARAM")).Mount("/api/v1/audit-logs", handler.Routes())`.
- [ ] **Test (httptest):** 200 list happy path + shape (list omits before/after, includes page/page_size/total); 200 detail includes before/after as JSON; 400 bad page/date; 400 date_from>date_to; 400 non-integer id; 404 missing id; 401 no token; 403 wrong role.
- **Demo:** curl list + detail as ADMIN returns flat JSON; a non-admin token returns 403.
- _Requirements: 1.5–1.7, 2.1, 2.9, 2.10, 3.1–3.4, 4.1, 4.2, 5 (data source)_
- _Model: Sonnet — HTTP handler + route wiring applying existing RequireAuth/RequireRoles middleware; RBAC is by-the-book here, not novel auth logic._

## Task 5 — Backend quality gate

- [ ] `sqlc generate`, `go build ./...`, `go test ./...`, `golangci-lint run` for `backend/`.
- [ ] Confirm no write/update/delete route exists on the audit-log mount (Property 6).
- **Test:** full backend suite green; coverage ≥ 80% on touched `internal/*`.
- _Requirements: 4.1–4.3; DoD_
- _Model: Sonnet — backend quality gate (build/lint/test) that needs judgment to triage failures._

## Task 6 — Frontend types, hooks, route

- [ ] Create `src/features/audit-log/types.ts`: `AuditLogListItem`, `AuditLogListResponse`, `AuditLogDetail`, `AuditLogFilters`, `actionBadgeVariant`, labels.
- [ ] Create `hooks/useAuditQueries.ts`: `useAuditLogList` (with `placeholderData: keepPrevious`) and `useAuditLogDetail` (enabled when id set), using the existing `api` client.
- [ ] Register route `src/routes/audit-logs.tsx` under `protectedRoute` with `requireRoles(["ADMIN","ADMIN_PARAM"])`.
- [ ] **Test:** property tests for `actionBadgeVariant` (every input → one of five variants); hook query-key stability; route guard denies non-admin.
- _Requirements: 1.1–1.4, 10.1–10.3, 5.4_
- _Model: Sonnet — frontend types, TanStack Query hooks, and route registration following established feature patterns._

## Task 7 — Filter bar + table

- [ ] `AuditFilterBar.tsx`: Action + Entity Type FilterSelects, Actor input, Date range (date_from/date_to), Reset; client-side `date_from > date_to` validation (Req 6.5); write filters + page to URL search (Req 6.4); any change resets page to 1 (Req 6.2).
- [ ] `AuditLogTable.tsx`: DataTable with the 6 columns, WIB timestamp formatting, action Badge (icon + label), pagination controls from `{page,page_size,total}`, loading skeleton, EmptyState ("Tidak ada log audit"), row click → open drawer.
- [ ] **Test:** filter change resets to page 1 and refetches with correct params; URL round-trip reconstructs identical filters (Property 12); empty + loading states; pagination next/prev; badges carry icon + label; date-range validation blocks request.
- _Requirements: 5.2–5.7, 6.1–6.5, 8.1, 8.2, 9.3, 9.4, 9.7_
- _Model: Sonnet — filter bar + table with URL sync and pagination; standard React data-table work._

## Task 8 — Detail drawer + before/after diff

- [ ] `AuditDetailDrawer.tsx`: fetch on `selectedId`; render metadata (actor, action badge, entity type/id, IP, WIB timestamp); slide-in transition (transform/opacity, 300ms enter / 225ms exit); focus trap; Escape/outside close; loading skeleton; inline error + retry without closing.
- [ ] `BeforeAfterDiff.tsx`: both present → per-top-level-key classification (added/removed/modified/unchanged) with `+ / − / ~` marker + icon + semantic tint; before null → "Dibuat" (after only); after null → "Dihapus" (before only).
- [ ] **Test:** diff classification correctness across added/removed/modified/unchanged and the null-before / null-after cases (Property 11); drawer opens on row click, closes on Escape, returns focus to trigger; detail error state.
- _Requirements: 7.1–7.9, 8.3, 9.1, 9.2, 9.6_
- _Model: Sonnet — detail drawer with focus trap plus before/after diff classification; several interacting concerns (a11y, transitions, diff logic) but all contained and test-covered._

## Task 9 — Frontend quality gate + navigation

- [ ] Add the "Audit Log" entry to the internal app sidebar/nav, visible only to ADMIN/ADMIN_PARAM (match existing role-gated nav pattern).
- [ ] `pnpm lint`, `pnpm test --run`, `pnpm build` for CompanyPortal-Vite.
- [ ] Verify OKLCH "Merah Sirih" tokens only (no hardcoded hex/rgb); 44px touch targets; semantic landmarks.
- **Test:** frontend suite green; a11y assertions pass; coverage ≥ 80% on the feature module.
- _Requirements: 9.1–9.7, 10.4, 10.5; DoD_
- _Model: Sonnet — role-gated nav entry plus a frontend quality gate (lint/test/build + token/a11y checks) needing failure triage._

## Task 10 — Docs

- [ ] Update `project-context.md` if the base `audit_logs` table was created here (mark proposed → exists; note the read-path indexes). Note that module wiring to *write* audit entries remains per-module future work.
- [ ] Short note in the spec folder on how other pages can deep-link into the viewer filtered by `entity_type` + `entity_id` (single-entity history), for reuse from invoice/DSR/approval detail screens later.
- _Requirements: dependency note; Req 2.5_
- _Model: Haiku — doc updates only (mark table proposed→exists, add a deep-link note); no code, low judgment._

---

## Sequencing & Dependencies

- **Task 0 is a hard gate** — resolve the `audit_logs` ownership question before any migration.
- Backend Tasks 1→5 must land before the frontend can integrate against real endpoints; Tasks 6→9 can be scaffolded against the design's response shapes in parallel but must be verified against the live API before Task 9's gate.
- If RBAC-Setup Task 3 lands first, Task 1 shrinks to the index-only migration.

## Definition of Done (project-context Sec 11)

- [ ] Matches module/table map — `audit_logs` is canonical; no new tables invented; read-only over it
- [ ] Correct auth path + scoped RBAC (ADMIN/ADMIN_PARAM at middleware AND route guard)
- [ ] No maker-checker needed (read-only, non-state-changing); the viewer itself writes no audit
- [ ] Reads use `dbPool` with documented replica TODO; no writes
- [ ] Timestamps timestamptz stored UTC, displayed Asia/Jakarta; entity/IP columns tabular-nums
- [ ] Tests green incl. RBAC denial (401/403), filter/pagination, date validation, diff classification
- [ ] No secrets hardcoded
- [ ] ATM backend keeps flat JSON shape
- [ ] Builds clean (`go build`, `sqlc generate`, `pnpm build`)
