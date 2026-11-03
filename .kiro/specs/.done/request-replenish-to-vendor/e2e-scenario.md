# E2E Scenario: Request Replenish to Vendor (create → submit → approve)

> **Status: RUN COMPLETE (2026-09-11).** Executed end-to-end against a live local stack (Postgres, Redis via Docker, Go backend on :8080, CompanyPortal-Vite on :5173, two browser sessions for maker/checker). All steps A–H passed after one blocking bug (found and fixed — see §7). See §7 for results and bugs found.

This exercises the full stack (PostgreSQL → Go ATM backend on :8080 → CompanyPortal-Vite on :5173) for the flow: browse DMAA forecast → create draft → submit → approve, with a maker/checker four-eyes split. It validates the 12 implemented tasks work together, not just as isolated unit tests.

> **Recommended Claude model (overall run): Opus — Effort: High.** This is a live-stack diagnostic across DB + Go backend + React frontend where the interesting work is *interpreting failures*: correlating a symptom (undefined UI cell, 500, silent stub call) with the known-risks list (§6), the stale-sqlc blast radius, and money/four-eyes correctness. That judgment is Opus territory. The tiers below are per-section — mechanical setup is lighter, the money/four-eyes/audit checks are the heavy ones. See `.kiro/steering/model-recommendation.md` for the tier heuristic.

---

## 0. Why this run matters (from tasks.md verification notes)

Tasks 7–10 all recorded "full browser E2E not possible in this sandbox" (no live backend/DB, memory-only auth token). Unit tests pass (vendor-request suite 21/21) but the assembled flow has never run against a real API. This run-book closes that gap and is most likely to surface: the stale-sqlc blast radius (task 5/12), the `VITE_API_MODE` stub-vs-real switch, and any request/response shape mismatch between the hand-written frontend types and the Go response structs.

---

## 1. Preconditions

> **Model: Sonnet — Effort: Medium.** DB/seed/Redis setup and DMAA-row seeding follow explicit SQL; real work but well-specified. Bump to Opus only if the seed FK chain (`dmaa_files` → `dmaa_atm_forecast`) fights back and you need to reason about schema state.

### 1.1 Database
- A reachable PostgreSQL with the CMS schema. A local dev DB already exists per task 12: `postgres://postgres:1818@localhost:5432/cms`, and **migration 028 is already applied** there (`vendor_requests` + `vendor_request_items` present, verified column-for-column).
- If starting fresh, apply all migrations in `backend/migrations/` in order (001 … 028). Seed data comes from the `*_seed_*.sql` migrations (003 seeds roles + users).

### 1.2 Seed users (from `003_seed_roles_users.sql`, all local auth, password `password123`)

| Purpose | username | role | Notes |
|---|---|---|---|
| **Maker** | `atm.user` | ATM-USER | creates + submits the request |
| **Checker** | `atm.spv` | ATM-SPV | approves/rejects; must differ from maker (four-eyes) |
| (alt maker) | `branch.atm.user` | BRANCH-ATM-USER | |
| (alt checker) | `branch.atm.spv` | BRANCH-ATM-SPV | |
| (admin) | `Yudha` | ADMIN | can do both roles, but four-eyes still blocks self-approve |

Auth is `auth_source='local'` (bcrypt), so **no LDAP needed** for this run. Login endpoint: `POST /api/v1/auth/login` with `{"username","password"}` (internal portal — no `X-Portal-Type` header required for local users).

### 1.3 DMAA forecast data (REQUIRED — likely the first blocker)
The create flow validates every item against `dmaa_atm_forecast` (`ForecastRowExists` per item). The browse endpoint returns rows for a single `forecast_date` (= `periode_pred`). So there must be at least one `dmaa_atm_forecast` row whose `periode_pred` equals the date you browse.

- **Verify first:** `SELECT periode_pred, count(*) FROM dmaa_atm_forecast GROUP BY 1 ORDER BY 1;`
- Pick a `forecast_date` that has rows. If the table is empty (fresh DB — the DMAA ETL is a separate Python job that may not have run locally), insert a couple of rows manually so the browse returns something. Minimal insert (needs a `dmaa_files` parent row for the FK):

```sql
INSERT INTO public.dmaa_files (name, status, is_valid, file_date)
VALUES ('e2e-manual.csv', 'processed', true, CURRENT_DATE)
RETURNING id;  -- note the id, use below as :file_id

INSERT INTO public.dmaa_atm_forecast
  (terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund)
VALUES
  ('ATM001', :file_id, CURRENT_DATE, 100000, 500000000, 0),
  ('ATM002', :file_id, CURRENT_DATE, 100000, 250000000, 0);
```
> Note: `terminal_id` in DB = the requirements' "atm_id". The browse API accepts an `atm_id` query param but returns `terminal_id` in the body.

### 1.4 Redis
`main.go` wires a Redis client (JWT blacklist, rate-limit). Per `.env.example`, `REDIS_URL=redis://cms-redis:6379` (Docker) — for a bare-metal run point it at a local Redis (`redis://localhost:6379`). If Redis is down, login/refresh may fail; start a local Redis or the `cms-redis` container first.

---

## 2. Start the stack (commands — DO NOT run yet)

> **Model: Sonnet — Effort: Low.** Copy `.env`, start backend + Vite dev server. Mechanical, but Sonnet (not Haiku) because the `VITE_API_MODE=real` vs `stub` switch is a silent-failure trap worth reasoning about (§6.1).

Windows PowerShell, from the workspace root. Postgres is external (already running).

### 2.1 Backend (ATM, port 8080)
```powershell
# backend/.env — copy from example and point DATABASE_URL at the local DB
#   DATABASE_URL=postgres://postgres:1818@localhost:5432/cms?sslmode=disable
#   REDIS_URL=redis://localhost:6379
#   JWT_SECRET=<any 32+ byte string>
# Then, from backend/:
go run ./cmd/api        # or: air   (hot reload)
```
Health check: `GET http://localhost:8080/health` (or `/healthz` — confirm the exact path in `cmd/api/main.go` during the run).

### 2.2 Frontend (CompanyPortal-Vite, Vite dev server)
The dev server proxies `/api` → `http://localhost:8080` (see `vite.config.ts`). **Critical:** the API client defaults to `VITE_API_MODE=stub` (see `src/lib/api/config.ts`) — in stub mode the UI never calls the real backend. For a true E2E you MUST run in `real` mode:

```powershell
# frontend/CompanyPortal-Vite/.env.local
#   VITE_API_MODE=real
#   VITE_API_BASE_URL=/api/v1        # proxied to :8080; leave default
# Then, from frontend/CompanyPortal-Vite/:
pnpm install
pnpm dev                 # Vite dev server, default http://localhost:5173
```
> Do not use `docker compose` for the frontend here — `frontend/docker-compose.yml` builds the Nginx production image (port 3001) and has no `/api` dev proxy, so it won't reach a bare-metal backend without extra wiring. Use `pnpm dev` for this walkthrough.

---

## 3. Walkthrough steps + expected results

> **Model: Opus — Effort: High.** The heart of the run. Steps A–D are executable-and-observe (Sonnet-ish), but the checks that matter — E (four-eyes self-approve block), F (checker approve + actor stamping), G (audit-trail state chaining + jsonb round-trip), H (reject/revise + 422 validation) — are money/approval-correctness verifications where a wrong pass/fail call hides a real maker-checker bug. Run and interpret these with Opus.

Two browser sessions (or two profiles / one normal + one incognito) so maker and checker are logged in independently. Steps below give the **UI action** and the **underlying API call** so the run can also be done purely via API (curl/Postman) if the UI blocks.

### Step A — Maker logs in
- **UI:** open `http://localhost:5173`, log in as `atm.user` / `password123`.
- **API:** `POST /api/v1/auth/login` → 200, body has `access_token` + `user`. Refresh token set as httpOnly cookie.
- **Expect:** lands on dashboard; sidebar (general group) shows "Forecast Browser" and "Vendor Request" (nav entries added in task 11 for ATM-USER).

### Step B — Browse forecast, select items
- **UI:** open **Forecast Browser** (`/replenishment/forecast-browser`). Pick the `forecast_date` that has DMAA rows (default is next business day — change it to the seeded date, e.g. today, if needed). Table lists rows; tick 1–2 rows; summary bar shows selected count + total IDR; click **"Buat Vendor Request"**.
- **API:** `GET /api/v1/vendor-requests/forecast?forecast_date=YYYY-MM-DD&page=1&page_size=20` → 200 `{data:[{terminal_id,periode_pred,denom,amount_replenish,amount_refund,dmaa_file_id}], pagination:{page,page_size,total_count,total_pages}}`.
- **Expect:** navigates to the create page carrying the selected rows (via the Zustand `selectionStore` from task 7). **Watch for:** empty table = wrong date or empty `dmaa_atm_forecast` (see 1.3).

### Step C — Create draft
- **UI:** on the create page, review items, optionally edit `amount_replenish`, add a note, click **"Save as Draft"**.
- **API:** `POST /api/v1/vendor-requests` body `{forecast_date, notes, items:[{terminal_id, periode_pred, denom, amount_replenish}]}` → **201** with the detail: `id`, `request_number` = `VR-YYYYMMDD-NNNN`, `status:"draft"`, `items`, `total_amount`.
- **Expect:** navigates to the detail page showing status **draft**. **Watch for:** 400 `bad_request` "items invalid" → an item doesn't match a real forecast row (terminal_id/periode_pred/denom mismatch). 400 duplicate → same triple twice.

### Step D — Submit for approval
- **UI:** on the draft detail page (as creator), click **"Submit"**.
- **API:** `POST /api/v1/vendor-requests/{id}/submit` → 200, status now **pending_approval**, `submitted_at` set.
- **Expect:** detail refetches (TanStack Query invalidation) and shows **pending_approval**; maker's action buttons collapse to Cancel. **Watch for:** 409 conflict if the request has zero items (shouldn't happen here); 403 if a non-creator tries.

### Step E — Maker CANNOT self-approve (four-eyes negative check)
- **API (as `atm.user`, the maker):** `POST /api/v1/vendor-requests/{id}/approve` → **403** `forbidden` "Checker tidak boleh sama dengan pembuat request (four-eyes)".
- **UI:** the maker's detail view should not even render an Approve button (task 10 gates it to non-creator checkers). Confirm both: no button in UI, and 403 if forced via API.

### Step F — Checker logs in and approves
- **UI:** second session, log in as `atm.spv` / `password123`. Open **Vendor Request** list (`/replenishment/vendor-requests`), find the pending request (status badge = warning), open it, click **"Approve"**.
- **API:** `GET /api/v1/vendor-requests?status=pending_approval` → shows the row. `POST /api/v1/vendor-requests/{id}/approve` → 200, status **approved**, `approved_at` + `approved_by` = checker.
- **Expect:** detail shows **approved** (success badge), approver name + timestamp. This is the happy-path terminal state for this feature (processing/completed are out of scope).

### Step G — Audit trail
- **API (as checker or admin):** `GET /api/v1/vendor-requests/{id}/audit-log` → 200 `{data:[…]}` with entries in created_at ASC order: `create` (previous_state null → draft), `submit` (draft → pending_approval), `approve` (pending_approval → approved), each with `performed_by` and `performed_at`.
- **Expect:** three entries, states chained correctly, actor ids = maker for create/submit, checker for approve. **Watch for:** this reads `audit_logs` and projects `before.state`/`after.state` — confirm the jsonb mapping (task 3.7/3.8) actually round-trips.

### Step H (optional) — Reject path on a second request
Repeat A–D to make another pending request, then as checker `POST /{id}/reject` with `{"rejection_reason":"melebihi plafon"}` → 200 status **rejected**. Empty/whitespace reason → **422** `validation_error`. Then as maker `POST /{id}/revise` → 200 back to **draft**.

---

## 4. Cross-cutting things to verify during the run

> **Model: Opus — Effort: Medium.** Money integrity (no float drift, create-total == list `total_amount`), RBAC gate correctness, read-after-write on the primary pool, and frontend/Go response-shape parity are exactly the golden-rule danger zones — a silent mismatch here (200 response, `undefined` cells) is easy to wave through and expensive to miss.

- **Money integrity:** amounts are integer IDR (bigint) end to end; the UI shows `tabular-nums`, right-aligned, dot thousands (e.g. `Rp 500.000.000`). No floats, no rounding drift between create total and list `total_amount`.
- **RBAC route gates:** a viewer-only role (none in seed except via admin) — at minimum confirm VENDOR-USER / BRANCH-USER get 403 on `POST /` (maker-only) and on `/approve` (checker-only).
- **Read-after-write:** after submit/approve the detail refetch reflects new state immediately (reads use primary pool, so no replica-lag staleness).
- **Request number format:** matches `^VR-\d{8}-\d{4}$`, encodes the forecast date, stays constant across transitions.
- **Response-shape parity:** frontend types (`src/features/vendor-request/types.ts`, snake_case verbatim) match the Go JSON structs (`vendor_request_response.go`). A silent mismatch shows as `undefined` cells in the UI even though the API returns 200.

---

## 5. Endpoint quick reference (as wired in `vendor_request_handler.go`)

> **Model: Haiku — Effort: Low.** Static reference table; no reasoning, just lookup while running the steps.

| Method | Path | Role gate | Step |
|---|---|---|---|
| GET | `/api/v1/vendor-requests/forecast` | viewer (ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV) | B |
| POST | `/api/v1/vendor-requests` | maker (ADMIN, ATM-USER, BRANCH-ATM-USER) | C |
| PUT | `/api/v1/vendor-requests/{id}/items` | maker | (edit) |
| POST | `/api/v1/vendor-requests/{id}/submit` | maker | D |
| POST | `/api/v1/vendor-requests/{id}/approve` | checker (ADMIN, ATM-SPV, BRANCH-ATM-SPV) | E/F |
| POST | `/api/v1/vendor-requests/{id}/reject` | checker | H |
| POST | `/api/v1/vendor-requests/{id}/revise` | maker | H |
| POST | `/api/v1/vendor-requests/{id}/cancel` | maker ∪ checker (union) | — |
| GET | `/api/v1/vendor-requests` | viewer | F |
| GET | `/api/v1/vendor-requests/{id}` | viewer | C–G |
| GET | `/api/v1/vendor-requests/{id}/audit-log` | audit (ADMIN, ATM-SPV, BRANCH-ATM-SPV) | G |

All mounted behind `RequireAuth`; per-route `RequireRoles` adds the subset above. Actor-level rules (creator-only, four-eyes) are enforced inside the service, not the route.

---

## 6. Known risks to watch (from prior task notes)

> **Model: Opus — Effort: Medium.** This is the diagnostic playbook — when something breaks, matching the symptom to the right cause here (stub-mode trap, empty DMAA table, stale-sqlc scan failure, memory-only token) is the reasoning that makes or breaks the run.

1. **Stub mode trap:** if `VITE_API_MODE` isn't `real`, the UI silently uses stubs and the whole run is meaningless. Verify by watching the Network tab for real calls to `:8080`.
2. **Empty DMAA table:** most likely first blocker (see 1.3). The ETL is a separate Python job; a fresh local DB won't have forecast rows.
3. **Stale sqlc / hand-patched db code (task 5/12):** the vendor_request db code is partly hand-written to sqlc's convention. If any query result doesn't scan into its struct at runtime, that's the symptom — capture the exact Go error.
4. **Auth token is memory-only:** a full page refresh drops the access token (no persist middleware). Use in-app navigation; if refreshing, re-login. The httpOnly refresh cookie may still recover the session via `/refresh`.
5. **Health-check path unconfirmed:** verify the actual liveness path in `cmd/api/main.go` during the run (assumed `/health`).

---

## 7. Findings (fill in when the run is executed)

> **Model: Sonnet — Effort: Low.** Recording results into the table is straightforward; the hard interpretation already happened in §3/§4/§6. Escalate a specific bug write-up to Opus only if root-causing it needs deep reasoning.

| Step | Result (pass/fail) | HTTP status | Notes / deviation |
|---|---|---|---|
| A login (maker) | PASS | 200 | `X-Portal-Type: company` header IS required for local users, contradicting §1.2's "no X-Portal-Type header required" note — omitting it gives 422. The Vite frontend sends it automatically. |
| B browse forecast | PASS | 200 | Default forecast_date (next business day) had no rows, as predicted by §1.3. Used seeded date 2026-07-17 (222 rows). Selected 2 items, total Rp 295.000.000 — matched sum exactly, tabular-nums/dot-thousands formatting correct. |
| C create draft | PASS | 201 | `VR-20260717-0001`, matches `^VR-\d{8}-\d{4}$`, status draft, total carried over correctly. |
| D submit | PASS | 200 | Status → pending_approval, `submitted_at` set, maker's Approve button correctly absent from UI. |
| E self-approve blocked | PASS | 403 | Blocked as expected, but error body is generic `{"error":"forbidden","message":"Anda tidak memiliki akses ke resource ini"}` — NOT the specific four-eyes message `"Checker tidak boleh sama dengan pembuat request (four-eyes)"` implied by §3 Step E. Behavior is correct; message text is a doc/code mismatch, not a bug. |
| F checker approve | PASS | 200 | Second browser tab in the same profile auto-logged-in as the maker via the shared httpOnly refresh cookie — had to explicitly log out and log in as `atm.spv`. Approved successfully; detail immediately showed approver name + timestamp (read-after-write via primary pool confirmed). |
| G audit trail | PASS | 200 | 3 entries, ASC order, state chain null→draft→pending_approval→approved, `performed_by` = maker (id 3) for create/submit, checker (id 4) for approve. jsonb round-trip correct. |
| H reject/revise (opt) | PASS | 422 then 200 then 200 | Second request `VR-20260718-0001`. Empty `rejection_reason` → 422 `validation_error` on field `rejection_reason` ("wajib diisi"). Valid reason → 200 rejected. Maker `/revise` → 200 back to draft. |

**Bugs found:**
1. **[FIXED, blocking] `users` table missing columns added by migration 026 (`password_changed_at`, `must_change_password`, `failed_login_attempts`, `locked_until`) — migrations 026 and 027 had never been applied to the local dev DB**, even though 028 (`vendor_requests`) was. Every login (any user, any role) failed with `503 service_unavailable` because `FindUserByUsername`'s JOIN query referenced nonexistent columns. Root cause: DB error from `pgx` is swallowed into a generic `auth.ErrServiceUnavailable` with no server-side log of the underlying SQL error (`internal/auth/service.go:89-92`) — this made the real cause (a schema mismatch, not Redis/DB *connectivity*) much harder to find than it should have been; only visible by reproducing the exact query by hand in psql. **Fix applied:** ran `026_users_password_policy.sql` and `027_seed_appaccess_role.sql` (both additive/idempotent, `IF NOT EXISTS` / `ON CONFLICT DO NOTHING`) against the local `cms` DB. Confirms the exact "stale migration state" risk flagged in §6.3, though the actual mismatch was missing migrations rather than stale sqlc codegen.
2. **[Minor, doc bug]** §1.2 says local-user login needs "no `X-Portal-Type` header" — false; the backend returns 422 without it. Not a code bug (frontend always sends it), just a stale run-book note.
3. **[Minor, doc bug]** §3 Step E's expected 403 message text doesn't match the actual generic forbidden response. Either the doc should be updated or the service should return the more specific four-eyes message.
4. **[Observation, not a bug]** Two browser tabs in the same profile cannot hold two independent logged-in sessions — the httpOnly `refresh_token` cookie is shared per-origin, so logging in as the checker in a second tab silently reuses/overwrites the maker's session. Confirms §6.4's "auth token is memory-only" risk in practice; the run-book's "two browser sessions (or two profiles)" instruction should say *two separate profiles/incognito*, not just "two tabs."

**§4 Response-shape parity — verified separately (2026-09-11), field-by-field diff of `frontend/CompanyPortal-Vite/src/features/vendor-request/types.ts` vs `backend/internal/handler/vendor_request_response.go` + request bodies in `vendor_request_handler.go`:**
- `ForecastRow`/`forecastRowResponse`, `PaginationMeta`/`vendorRequestPagination`, `UserRef`/`vendorRequestUserRef`, `VendorRequestItem`/`vendorRequestItemResponse`, `VendorRequestDetail`/`vendorRequestDetailResponse` (16 fields), `VendorRequestSummary`/`vendorRequestSummaryResponse` (13 fields), `VendorRequestListResponse`, `AuditLogEntry`/`auditLogEntryResponse`, `AuditLogResponse` — **all match exactly**, same field names, same nullability (pointer ↔ `| null`).
- `CreateVendorRequestPayload`/`createVendorRequestBody`, `VendorRequestItemInput`/`itemInputPayload`, `RejectVendorRequestPayload`/`rejectBody` — **match exactly**.
- One gap: `PUT /{id}/items` (`updateItemsBody`, Go) has no corresponding payload type in `types.ts` — the "Edit Item" flow isn't wired into the frontend yet (per §5's endpoint table, no walkthrough step exercises it). Not a parity bug, just an unbuilt feature; flag if/when Edit Item is implemented.
- Conclusion: no `undefined`-cell risk for anything the UI currently uses. §4 is fully closed.

**Follow-up actions:**
- Add a real migration-state check (e.g. a tracked `schema_migrations` table, since none exists — migrations here are applied ad hoc) so a dev DB silently missing an additive migration doesn't manifest as an opaque 503 on every login.
- Log the underlying error (with `slog`, not just swallowing it) before mapping to `ErrServiceUnavailable` in `internal/auth/service.go` and the other call sites listed in `pkg/auth/errors.go` — would have cut this run's debugging time from ~10 minutes to under 1.
- Reconcile §1.2 and §3 Step E wording against actual behavior (both are doc-only fixes).
- Optional: make the four-eyes rejection return `pkgauth`'s specific message instead of the generic RBAC-style forbidden body, if the more specific UX is wanted.
