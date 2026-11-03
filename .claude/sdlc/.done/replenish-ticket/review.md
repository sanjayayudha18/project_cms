# replenish-ticket — Review (stage 5)

Reviewed 2026-10-08 by Claude (inline review of the full diff: bugs/concurrency, security/RBAC, CLAUDE.md compliance; coverage measured; checks run against dev DB with migration 026).
Separate reviewer agents (code-reviewer / database-reviewer / security-reviewer) not run — available on request.
Status: **R1 + R2 fixed; no open Important; no CRITICAL.** Code owner merges; AI never self-approves. Manual browser check still outstanding (user).

## Important
| # | Finding | Evidence | Status |
|---|---|---|---|
| R1 | **FR1.3 (NNN past 999 → 422, whole create rolled back) had no test.** The mapping in `applyTicketPlan` keys on the constraint name `vendor_request_tickets_seq_chk`; a renamed constraint would turn the 422 into a 500 silently. | `tests.md` "Not covered" list. | **Fixed** — `TestIntegration_Ticket_SeqExhaustedRejects`: ticket pushed to seq 999 → next create 422 "sudah mencapai 999", request count unchanged. Green. |
| R2 | **`ticketDate` fallback for old `VR-` requests (no `replenish_date`) was untested** — `UpdateItems` on such a draft would issue tickets on that date; a wrong timezone would put NNN on the wrong day versus the migration backfill. | `go tool cover`: `ticketDate` 50 %. | **Fixed** — `TestTicketDate`: 18:30 UTC → next day WIB, same rule as migration FR7.3. Green. |

## Nit / minor
| # | Finding | Suggested follow-up |
|---|---|---|
| N1 | `PUT /admin/vendors/{id}/branches/{id}` without `region_code` in the body stages `region_code = NULL` (nil = cleared). CompanyPortal always sends the field; only a hand-written API client could drop it, and the change still goes through maker-checker with before/after. | Tri-state field (absent = keep) if external API clients ever appear. |
| N2 | Each issued ticket takes a transaction-scoped advisory lock held until commit. `Create` has no item cap (UpdateItems caps 1000), so one huge create holds one lock per ATM in the shared lock table (`max_locks_per_transaction × max_connections`). | Cap `Create` items like `UpdateItems` if requests ever approach that size; today requests are tens–low hundreds of ATMs. |
| N3 | Cancelled / rejected requests keep their tickets active, so their NNN stays consumed (spec FR3.1, intent Q2: numbers never reused). NNN therefore counts *issued* trips per ATM per day, not completed ones. | None — matches the accepted spec; noted for the PO/vendor reading the number. |
| N4 | `validateItemsSingleVendor` maps terminal → cabang; if one ATM appears with two `periode_pred`s whose kelolaan resolve to different cabang, the last one wins for the region check. | Only possible when a kelolaan changes inside one request's periods; revisit if it happens. |
| N5 | Pre-existing, out of scope: vendor-branch CSV import never sends `category`, so CSV create/update always fails validation. | Spawned as a separate task ("Fix vendor-branch CSV import missing category"). |
| N6 | Pre-existing gofmt drift in `masterdata_export.go` (struct field alignment on the untouched `page` line). | Fix in an unrelated formatting change, not here. |

**Risk retracted (raised in the build summary):** a `vendor_branch` update staged *before* migration 026 does **not** wipe the seeded code on approve. Its `Before` snapshot lacks `region_code`, so `statesEqual` (`masterdata_approval.go`) sees a difference → the change is marked **stale** and not applied; the maker resubmits. Dev had 0 pending vendor-branch changes anyway.

## Security pass
- No new endpoints and no new roles. Vendor-request create/edit/completion keep their route + service RBAC (maker/checker); vendor-branch create/update keep `ADMIN`/`ADMIN_PARAM` at route + inside `Submit`.
- `region_code` validated server-side (`normalizeBranchRegionCode`, regex `^[A-Z0-9]{2,10}$`) for the form and CSV, plus DB CHECK `vendor_branches_region_code_chk` as the last net. It ends up inside the request number, so the strict charset also keeps the number parseable.
- All SQL via sqlc parameters; the advisory-lock key is `hashtextextended(<param> || '|' || <param>)`, no string-built SQL. The ticket number is assembled only in SQL from validated parts.
- No secrets, no new env vars; frontend renders ticket/region as text (no `dangerouslySetInnerHTML`).

## Bugs / correctness pass
- **Concurrency**:
  - Create takes the counter row lock `(vendor, region_code, date)` first (inside the savepoint), then the ticket advisory locks in `terminal_id` order. Two creates for the same vendor/region/date serialise on the counter row before any ticket lock.
  - Creates for different vendors and `UpdateItems` (request row `FOR UPDATE`, then ticket locks in `terminal_id` order) always take ticket locks in the same order, so there is no lock cycle.
  - `UNIQUE (terminal_id, replenish_date, seq)` is the backstop if the lock were ever skipped.
  - `…_ConcurrentCreatesGetDistinctNumbers` (one ATM, parallel creates) exercises this and passes.
- **Atomicity**:
  - Tickets are issued in the create/edit tx. A 422 from the region check or seq exhaustion rolls back the whole request (R1 test).
  - Laporan selesai writes `result` in the completion tx. An ATM without an active ticket makes the query return 0 rows, which surfaces as an error instead of silently skipping.
- **Old data**:
  - Migration 026 is one tx with a count guard before `DROP TABLE`.
  - Old numbers are untouched. Old requests (`region_code` NULL) skip the region check on edit, and their tickets use the backfill date rule.
  - The seq table uses `UNIQUE NULLS NOT DISTINCT`, so old NULL-region counters stay unique.
- **Edit rules**: the pure `planTickets` (unit-tested in 6 cases) is applied in the order deactivate → reactivate → issue. That order is what the partial unique index `(request_number, terminal_id) WHERE is_active` requires (integration test: 1 active / 2 total after 50K → MIX → 50K).
- **Invariant in the DB**: `vendor_request_tickets_inactive_chk` means an inactive ticket always has `deactivated_at` set and never carries a `result` (FR9.4).
- **No deletes**: no query deletes tickets. The FK is `ON DELETE RESTRICT`, and only test cleanup deletes rows.

## Coverage (service package, `-tags integration`, dev DB)
- **New file `vendor_request_ticket.go`:**
  - `singleRegion`, `wantedCodes`, `planTickets`, `addTo`, `sortedKeys`: 100 %
  - `applyTicketPlan`: 82 %
  - `denomCode`: 80 %
  - `ticketDate`: now 100 % (R2)
  - `normalizeBranchRegionCode`: 100 %
- **Touched functions:**
  - `createWithRetryingNumber`: 88 %
  - `attemptCreateWithNumber`: 80 %
  - `SubmitCompletion`: 82 %
  - `ApproveCompletion`: 90 %
  - `requestAtmStatuses`: 92 %
  - `Create`: 72 %
  - `Get`: 70 %
  - `UpdateItems`: 63 %
  - `validateItemsSingleVendor`: 60 %
- What's uncovered is mostly DB-error wrap branches.
- Package total: 69.4 %. That figure includes untouched legacy files, so per-file 80 % is not reached everywhere. The gap is error plumbing, not business branches.

## Definition of Done (CLAUDE.md Sec 11)
- [x] **Matches requirement + module/table map.** `vendor_request_tickets` and `region_code` are in Sec 3, the decision is in Sec 12 (migration 026), and `docs/data-map.md` and the dbml are updated.
- [x] **Correct auth path.** Internal CompanyPortal only; VendorPortal is untouched (spec Q5).
- [x] **Maker-checker + audit.**
  - `region_code` uses the existing vendor-branch maker-checker flow.
  - Request create/update_items audits carry `region_code` and the ticket changes.
  - Laporan selesai stays maker-checker.
  - No GR#3 deviation.
  - The migration seed of master data is outside maker-checker, like other migration seeds (accepted in spec/plan).
- [x] **Reads on replica / writes on primary.** Unchanged: `Get` keeps its existing primary read (read-after-write after create/edit).
- [x] **Money as numeric / timestamps timestamptz.** No money here; new timestamps are `timestamptz`, dates are `date`.
- [x] **Tests passing incl. auth/RBAC cases.** Go unit + integration green, frontend 1055 tests green. RBAC paths are unchanged and their existing tests are green.
- [x] **No secrets/config hardcoded.** `.env.example` unchanged.
- [x] **Response shape.** Flat JSON with additive fields `ticket_number` and `region_code`.
- [ ] **Builds cleanly in Docker.** Not run in this session; `go build ./...` and `pnpm build` are OK.

## Gates
- [ ] Manual browser check (user), covering:
  - "No. Tiket" in the detail and laporan selesai tables
  - "Kode Region" in the vendor cabang form and table
  - the 422 message when creating a request for a cabang without a code
- [ ] Fill `region_code` for the cabang still NULL (51 with an empty region text in dev) through master data. Until then, requests for their ATMs are rejected.
- [ ] Commit (feature is uncommitted) → code owner merge.
- [ ] Deploy note: run migration 026 with a `pg_dump --data-only -t vendor_request_atm_results -t vendor_request_number_seq -t vendor_branches` backup first. Tell vendors and report readers about the new request-number format.
