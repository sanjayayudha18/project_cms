# Implementation Plan: Audit Log Viewer

> Read-only viewer over the generic `audit_logs` trail. Backend-first (ATM backend, port 8080, flat JSON) then the CompanyPortal-Vite page. Follows the ATM backend's actual technical-layer layout (handler/service/repository + sqlc). Access: `ADMIN` / `ADMIN_PARAM`.
>
> **STOP-and-confirm gates (project-context Sec 3):** anything touching the `audit_logs` table schema, auth, or RBAC is a confirm-before-migrate step. This spec does NOT write audit entries (read-only) — but it may create the `audit_logs` table if RBAC-Setup Task 3 has not, which is a schema change requiring the usual project-context Sec 2 proposal + approval.

---

## Task 0 — Reconcile the `audit_logs` table with RBAC-Setup (gate)

- [x] Check migrations for an existing `audit_logs` table (RBAC-Setup Task 3, target `023_audit_logs.sql`).
  - **It exists** (`backend/migrations/023_audit_logs.sql`, landed by RBAC-Setup Task 3), with `actor_idx` and `entity_idx` already present. Reusing it as the source of truth. Task 1 skips table creation; only the read-path index migration is needed.
- [x] Confirm `audit_logs` is already listed as a canonical Core table in `project-context.md` Sec 2 (it is pre-approved by name). No new table proposal needed for the base table; the read-path indexes are additive.
- **Done when:** it is unambiguous whether Task 1 creates the table or only adds indexes, and the migration number is agreed. — **Resolved:** index-only migration, `029_audit_logs_read_indexes.sql` (next free number after `028_vendor_requests.sql`).
- _Requirements: dependency note; Req 4.3_
- _Model: Opus, Effort: High — a schema-ownership gate on the shared `audit_logs` table; getting the create-vs-index decision wrong forks the audit table across two specs._

## Task 1 — Migration: `audit_logs` (if absent) + read-path indexes

- [x] Create `backend/migrations/029_audit_logs_read_indexes.sql` (additive, style of `014`: `BEGIN; ... COMMIT;` + WHY/SAFETY header).
  - Table already exists (Task 0), so no table creation here.
  - `actor_idx` and `entity_idx` already exist from `023_audit_logs.sql`. This migration adds only the two still missing: `audit_logs_created_at_id_idx (created_at DESC, id DESC)` and `audit_logs_action_idx (action)` — both `IF NOT EXISTS`.
- [x] **Test:** migration up/down clean; `\d audit_logs` shows the four indexes; a seeded set of rows returns in `created_at DESC, id DESC` order. — **Done (2026-09-21):** the 2026-09-18 baseline squash had dropped `029` (baseline only kept `actor_idx`/`entity_idx`), so it was re-issued as `backend/migrations/006_audit_logs_read_indexes.sql` and applied to the live DB via `localhost:5432` (`host.docker.internal` only resolves inside Docker). `pg_indexes` now lists `audit_logs_action_idx`, `audit_logs_actor_idx`, `audit_logs_created_at_id_idx`, `audit_logs_entity_idx`, `audit_logs_pkey`. Forward-only (project convention), so no down migration; ordering is covered by the Task 2 integration test.
- **Demo:** `EXPLAIN` on a filtered+ordered query shows an index used, not a full seq scan on a large seed. — **Done:** `EXPLAIN select * from audit_logs where action='submit' order by created_at desc, id desc limit 25` → `Index Scan using audit_logs_created_at_id_idx`. Only 763 rows locally, so this shows the index is chosen, not large-scale timing.
- _Requirements: 2.8, 2.11; design "audit_logs schema"_
- _Model: Opus, Effort: High — a DB migration that may create the append-only `audit_logs` table; schema changes to the audit trail are a project-context STOP-and-confirm zone._

## Task 2 — sqlc queries + repository (read-only)

- [x] Add `backend/queries/audit_log.sql`: `ListAuditLogs`, `CountAuditLogs`, `GetAuditLogByID` with `sqlc.narg` nullable filters (actor_id, action, entity_type, entity_id, date_from, date_to) + `LIMIT`/`OFFSET`, per design.md.
- [x] Run `sqlc generate` (config `backend/sqlc.yaml`). Used named `sqlc.arg('limit')`/`sqlc.arg('offset')` from the start (avoided the positional/named mixing question entirely). **Note:** local `sqlc` is v1.31.1 and regenerated 4 unrelated files (`approval.sql.go`, `audit.sql.go`, `auth.sql.go`, `models.go`) with different casing (`IP`→`Ip`) than whatever generated the checked-in code — those were reverted (`git checkout --`) to stay in scope; only `internal/db/audit_log.sql.go` was kept, hand-fixed to use the existing `AuditLog.IP` field name so it compiles against the current struct. Worth a follow-up to pin the project's sqlc version (e.g. in `tools.go` or CI) so this doesn't recur.
- [x] Add `internal/repository/audit_log_repository.go` wrapping `*db.Queries` (pattern from `auth_repository.go`). Returns `db.AuditLog` rows directly rather than a duplicate domain struct — `internal/service` already depends on `internal/db` elsewhere in this codebase (e.g. `atm_portal` service), so a parallel domain type would be pure duplication; `before`/`after` → `json.RawMessage` mapping is Task 3's job (service layer), per design.md's own layer split. Uses `dbPool` (via the `db.DBTX` param) with a `ponytail:` TODO comment for the replica-pool swap, matching `cmd/api/main.go`'s existing convention.
- [x] **Test (integration, real Postgres):** `internal/repository/audit_log_repository_test.go` (`//go:build integration`) seeds 5 rows under a randomized `entity_type` tag and asserts: no-filter ordering (`created_at DESC, id DESC`), `actor_id` filter, `action` filter, combined `entity_type`+`entity_id` filter, inclusive date-range boundaries, `Count` vs `List` length, `Count` vs summed-pages consistency, and `GetByID` (found + before/after passthrough + not-found → nil). Builds and `go vet -tags integration` clean. **Run against the live DB (2026-09-21, `DATABASE_URL` with host `localhost`): `go test -tags integration ./internal/repository/ -run AuditLog` → 9/9 subtests PASS.**
- **Demo:** repository test prints a filtered page + total for a seeded dataset. — **Done:** the 9 subtests above exercise filtered pages, totals and page consistency on a seeded dataset.
- _Requirements: 2.1–2.9, 3.1, 3.4_
- _Model: Sonnet, Effort: Medium — sqlc queries + read-only repository over one table following the existing repository pattern._

## Task 3 — Read service (validation, pagination, mapping)

- [x] Add `internal/service/audit_log_read.go`: `ListAuditLogsParams`, page/size clamp (`[1,100]`, default 25; page ≥ 1), offset compute; `date_from > date_to` → `ValidationError` (reused the type from `atm_portal.go` — same package, no import needed); assembles `{items, page, page_size, total}` via `ListAuditLogsResult`; `GetByID` maps `before`/`after` (`[]byte` from sqlc) to `json.RawMessage` for `AuditLogDetail`. Depends on the Task 2 `AuditLogRepository` through a small interface defined in this package (same convention as `AtmPortalRepository`), satisfied by `*repository.AuditLogRepository`.
- [x] **Test (table-driven):** `internal/service/audit_log_read_test.go` — page_size clamp matrix (0, negative, 10, 100, 101, 1000 → 25/25/10/100/100/100); page floor matrix (0, -3, 1, 7 → 1/1/1/7); date-range invalid → `*ValidationError{Field: "date_from"}`; equal `date_from`==`date_to` allowed (inclusive boundary); clamped page/page_size flow into the repo call (offset math checked); repo error propagation; before/after JSON round-trip via a fake repo (no DB needed — pure unit tests). All pass: `go test ./internal/service/...` green.
- **Demo:** unit test showing clamp + validation matrix green — ran locally, all pass.
- _Requirements: 2.7, 2.10, 3.4_
- _Model: Sonnet, Effort: Medium — read service with pagination clamp + date-range validation; well-specified logic within known patterns._

## Task 4 — HTTP handler (flat JSON) + route wiring

- [x] Add `internal/handler/audit_log_handler.go`: `Routes()` with `GET /` (list) and `GET /{id}` (detail); parses query params (`actor_id`/`entity_id` as optional int64, `action`/`entity_type` as optional string); converts `date_from`/`date_to` (`YYYY-MM-DD` or RFC3339) from WIB (`time.FixedZone`, not `time.LoadLocation` — Asia/Jakarta has no DST and this avoids depending on the distroless image shipping tzdata) to UTC — a bare `date_to` is anchored to end-of-day WIB so the inclusive `created_at <= date_to` filter covers the whole calendar day; flat JSON via `writeJSON`/`writeError`; 400/404/500 mapping per design.md's error table (`*service.ValidationError` → 400, everything else → 500, matching `atm_portal_handler.go`'s `handleServiceError` convention — not DMAA's 503, since this table has no replica-unavailability concern yet).
- [x] Wire in `cmd/api/main.go`: `repository.NewAuditLogRepository(dbPool)` → `service.NewAuditLogReadService(...)` → `handler.NewAuditLogHandler(...)`, mounted at `/api/v1/audit-logs` behind `RequireAuth` + `RequireRoles("ADMIN","ADMIN_PARAM")` (same stack as `/api/v1/admin/approval`), with the same `ponytail:` replica-pool TODO comment used elsewhere in this file.
- [x] **Test (httptest):** `audit_log_handler_test.go` — 200 list happy path + shape (list omits before/after, includes `page`/`page_size`/`total`); all filters + page/page_size pass through correctly, including the WIB→UTC date conversion (verified against exact UTC instants); defaults left at 0 when absent (service clamps, per Task 3); 400 for non-numeric `page`/`page_size`/`actor_id`/`entity_id` and bad date format; 400 for `date_from > date_to` (`*ValidationError` from the service); 500 for other service errors; 200 detail includes `before`/`after` as real JSON objects (round-tripped through `json.Unmarshal` to prove they aren't escaped strings); 400 non-integer `id`; 404 missing id. `audit_log_handler_rbac_test.go` — 401 no token, 403 non-admin role (`ATM-USER`), 200 for `ADMIN_PARAM`, against the real `RequireAuth`/`RequireRoles` middleware (same pattern as `admin_approval_handler_test.go`). All pass; full backend suite (`go build ./...`, `go test ./...`) green, no regressions.
- **Demo:** curl list + detail as ADMIN returns flat JSON; a non-admin token returns 403. — logic verified via httptest against the real middleware, and **live (2026-09-21)** through the running app: signed in as `ADMIN`, `GET /api/v1/audit-logs?page=1&page_size=25` and `...&action=reject` returned 200 with the real rows (16 `reject` entries, matching the DB), and detail rows returned real before/after. No curl demo of the 403: it needs a non-admin token, and the 403 is covered by `audit_log_handler_rbac_test.go`.
- _Requirements: 1.5–1.7, 2.1, 2.9, 2.10, 3.1–3.4, 4.1, 4.2, 5 (data source)_
- _Model: Sonnet, Effort: Medium — HTTP handler + route wiring applying existing RequireAuth/RequireRoles middleware; RBAC is by-the-book here, not novel auth logic._

## Task 5 — Backend quality gate

- [x] `sqlc generate` — reran; same version-mismatch drift as Task 2 (4 unrelated files rewritten with `IP`→`Ip` casing), reverted those again and reapplied the `Ip`→`IP` fix to `audit_log.sql.go`. This will recur on every `sqlc generate` until the sqlc version is pinned (flagged in Task 2, still open). `go build ./...` clean. `go test ./...` green across all packages. `golangci-lint run ./...` — 11 pre-existing issues, all in files this spec never touched (`main.go`'s pre-existing lines, `dsr_upload_handler.go`, `dsr_upload.go`, `dsr_upload_test.go`, `atm_portal_cashpos.go`, `vendor_request.go`); zero lint issues in any `audit_log_*` file or the wiring line added to `main.go`.
- [x] Confirm no write/update/delete route exists on the audit-log mount (Property 6) — `internal/handler/audit_log_handler.go`'s `Routes()` registers only `r.Get("/", ...)` and `r.Get("/{id}", ...)`; `queries/audit_log.sql` contains no INSERT/UPDATE/DELETE. Verified by grep.
- **Test:** full backend suite green (`go test ./...`, all packages `ok`). Coverage on touched files: `audit_log_read.go` (service) 83.3–100% per function, `audit_log_handler.go` (handler) 81.8–100% per function — both comfortably ≥ 80%. `audit_log_repository.go` (repository) shows 0% here only because its test is `//go:build integration`-gated and no live DB is reachable from this session (same gap as Tasks 1–2) — the test itself covers every method; it just hasn't executed against Postgres yet.
- _Requirements: 4.1–4.3; DoD_
- _Model: Sonnet, Effort: Medium — backend quality gate (build/lint/test) that needs judgment to triage failures._

## Task 6 — Frontend types, hooks, route

- [x] Create `src/features/audit-log/types.ts`: `AuditLogListItem`, `AuditLogListResponse`, `AuditLogDetail`, `AuditLogFilters`, `actionBadgeVariant`. Adjusted from design.md's sketch: `actor_id`/`entity_id` are non-nullable `number`, not `number | null` — the real `audit_logs` table (`023_audit_logs.sql`) declares both `NOT NULL`, and the backend types (Task 3) already reflect that.
- [x] Create `hooks/useAuditQueries.ts`: `useAuditLogList` (`placeholderData: keepPreviousData`) and `useAuditLogDetail` (`enabled: id !== null`), using the existing `api` client — same shape as `dmaa-forecast/useDmaaForecastData.ts`.
- [x] Register route `src/routes/audit-logs.tsx` under `protectedRoute` with `requireRoles(["ADMIN","ADMIN_PARAM"])`, wired into `src/main.tsx`'s route tree. Its `component` is a new, intentionally minimal `AuditLogPage` (`src/features/audit-log/components/AuditLogPage.tsx`) that calls `useAuditLogList` with fixed default params — just enough to make the route real and testable. Task 7 replaces its body with `AuditFilterBar`/`AuditLogTable` (URL-synced filters/pagination); Task 8 adds the detail drawer.
- [x] **Test:** `__tests__/mappings.property.test.ts` — `actionBadgeVariant` fuzzed over arbitrary strings always returns one of the five variants, plus fixed cases for every suffix rule. `__tests__/hooks.test.ts` — `auditKeys.list`/`.detail` are deep-equal for identical inputs and change whenever any filter/page/pageSize/id differs, list and detail keys never collide. `src/routes/audit-logs.test.ts` — `requireRoles(["ADMIN","ADMIN_PARAM"])` denies `ATM-USER`/`BRANCH-USER`, allows `ADMIN`/`ADMIN_PARAM`, and throws (redirect) when unauthenticated. All 15 new tests pass (`pnpm vitest run`); `pnpm tsc -b` and `pnpm biome check` show zero errors attributable to any new/edited file (pre-existing, unrelated failures elsewhere confirmed unaffected — see Task 9 note).
- _Requirements: 1.1–1.4, 10.1–10.3, 5.4_
- _Model: Sonnet, Effort: Medium — frontend types, TanStack Query hooks, and route registration following established feature patterns._

## Task 7 — Filter bar + table

- [x] `AuditFilterBar.tsx`: Action + Entity Type FilterSelects, Actor input, Date range (date_from/date_to), Reset; client-side `date_from > date_to` validation (Req 6.5); write filters + page to URL search (Req 6.4); any change resets page to 1 (Req 6.2). — Actor is a numeric-ID input (the API filters by `actor_id`; the list has no user names). The Action/Entity Type option lists are fixed to the values in `audit_logs` today (a `ponytail:` comment marks the upgrade path); URL state in `useAuditLogUrlState.ts`, route `validateSearch` = `AUDIT_LOG_SEARCH_SCHEMA`.
- [x] `AuditLogTable.tsx`: DataTable with the 6 columns, WIB timestamp formatting, action Badge (icon + label), pagination controls from `{page,page_size,total}`, loading skeleton, EmptyState ("Tidak ada log audit"), row click → open drawer. — Added an optional `onRowClick` to the shared `DataTable`; the timestamp is also a real `<button>` so keyboard users can open a row. Client-side column sorting is off (server order is authoritative). Columns are `useMemo`'d: unmemoized, TanStack remounts the row buttons every render and focus can never return to the trigger.
- [x] **Test:** filter change resets to page 1 and refetches with correct params; URL round-trip reconstructs identical filters (Property 12); empty + loading states; pagination next/prev; badges carry icon + label; date-range validation blocks request. — `AuditLogPage.test.tsx`, `useAuditLogUrlState.test.tsx`; also verified live in the browser (filter → `?action=reject` + request `action=reject`; invalid range leaves the URL unchanged and shows the alert).
- _Requirements: 5.2–5.7, 6.1–6.5, 8.1, 8.2, 9.3, 9.4, 9.7_
- _Model: Sonnet, Effort: Medium — filter bar + table with URL sync and pagination; standard React data-table work._

## Task 8 — Detail drawer + before/after diff

- [x] `AuditDetailDrawer.tsx`: fetch on `selectedId`; render metadata (actor, action badge, entity type/id, IP, WIB timestamp); slide-in transition (transform/opacity, 300ms enter / 225ms exit); focus trap; Escape/outside close; loading skeleton; inline error + retry without closing. — **Deviation:** built on the shared `Dialog` (focus trap, Escape/outside close, focus return already there), so it is a centered modal with `Dialog`'s 200ms enter, **not** a slide-in panel with 300/225ms timings. Build a side panel only if the design needs one.
- [x] `BeforeAfterDiff.tsx`: both present → per-top-level-key classification (added/removed/modified/unchanged) with `+ / − / ~` marker + icon + semantic tint; before null → "Dibuat" (after only); after null → "Dihapus" (before only). — pure logic in `lib/diffAuditValues.ts`; nested values compare structurally (one "Berubah" row, no recursive diff).
- [x] **Test:** diff classification correctness across added/removed/modified/unchanged and the null-before / null-after cases (Property 11); drawer opens on row click, closes on Escape, returns focus to trigger; detail error state. — `diffAuditValues.test.ts`, `AuditLogPage.test.tsx` (incl. a focus-return regression test that failed before the `useMemo` fix). Verified live: a real `reject` row shows `state: pending_approval → rejected` (Berubah) and `rejection_reason` (Ditambah).
- _Requirements: 7.1–7.9, 8.3, 9.1, 9.2, 9.6_
- _Model: Sonnet, Effort: High — detail drawer with focus trap plus before/after diff classification; several interacting concerns (a11y, transitions, diff logic) reward extra deliberation even though each is contained and test-covered._

## Task 9 — Frontend quality gate + navigation

- [x] Add the "Audit Log" entry to the internal app sidebar/nav, visible only to ADMIN/ADMIN_PARAM (match existing role-gated nav pattern). — `NAV_CONFIG` item `audit-logs` ("Log Audit", Monitoring group).
- [x] `pnpm lint`, `pnpm test --run`, `pnpm build` for CompanyPortal-Vite. — `pnpm build` passes. `biome check` on every file this spec touched is clean. **The project-wide gate is not fully green, for reasons outside this spec** (files untouched here): 2 `lint/a11y/useSemanticElements` errors in `components/ui/Dialog.tsx:77` and `features/eod-monitoring/components/RetryDrawer.tsx:81`, and 4 failing tests in `features/rbac-settings/__tests__/SettingsHubPage.test.tsx` ("Task 10" admin-card links).
- [x] Verify OKLCH "Merah Sirih" tokens only (no hardcoded hex/rgb); 44px touch targets; semantic landmarks. — no hex/rgb/hsl in the feature (grep); inputs/selects/buttons use `min-h-[44px]`; the page relies on `AppShell`'s `<main>` (no nested landmark).
- **Test:** frontend suite green; a11y assertions pass; coverage ≥ 80% on the feature module. — audit-log suite: 33 tests pass; feature coverage 84% statements / 85% lines (hooks file is 17% because the page tests mock it; its query-key logic is covered by `hooks.test.ts`).
- _Requirements: 9.1–9.7, 10.4, 10.5; DoD_
- _Model: Sonnet, Effort: Medium — role-gated nav entry plus a frontend quality gate (lint/test/build + token/a11y checks) needing failure triage._

## Task 10 — Docs

- [x] Update `project-context.md` if the base `audit_logs` table was created here (mark proposed → exists; note the read-path indexes). Note that module wiring to *write* audit entries remains per-module future work. — **No change needed:** the table was not created here (it predates this spec and is already in the Sec 3 Core group). The read-path indexes are in `backend/migrations/006_audit_logs_read_indexes.sql`; status is tracked in `.claude/development-progress.md`.
- [x] Short note in the spec folder on how other pages can deep-link into the viewer filtered by `entity_type` + `entity_id` (single-entity history), for reuse from invoice/DSR/approval detail screens later. — see [deep-links.md](./deep-links.md) (includes the observed hard-load redirect limitation).
- _Requirements: dependency note; Req 2.5_
- _Model: Haiku, Effort: Low — doc updates only (mark table proposed→exists, add a deep-link note); no code, low judgment._

---

## Sequencing & Dependencies

- **Task 0 is a hard gate** — resolve the `audit_logs` ownership question before any migration.
- Backend Tasks 1→5 must land before the frontend can integrate against real endpoints; Tasks 6→9 can be scaffolded against the design's response shapes in parallel but must be verified against the live API before Task 9's gate.
- If RBAC-Setup Task 3 lands first, Task 1 shrinks to the index-only migration.

## Definition of Done (project-context Sec 11)

- [x] Matches module/table map — `audit_logs` is canonical; no new tables invented; read-only over it
- [x] Correct auth path + scoped RBAC (ADMIN/ADMIN_PARAM at middleware AND route guard)
- [x] No maker-checker needed (read-only, non-state-changing); the viewer itself writes no audit
- [x] Reads use `dbPool` with documented replica TODO; no writes
- [x] Timestamps timestamptz stored UTC, displayed Asia/Jakarta; entity/IP columns tabular-nums
- [x] Tests green incl. RBAC denial (401/403), filter/pagination, date validation, diff classification — for this spec's tests; see the Task 9 note for unrelated project-wide failures
- [x] No secrets hardcoded
- [x] ATM backend keeps flat JSON shape
- [ ] Builds clean (`go build`, `sqlc generate`, `pnpm build`) — `go build` and `pnpm build` pass; `sqlc generate` was not re-run in this pass and still has the version-drift issue from Tasks 2/5 (pin the sqlc version)
