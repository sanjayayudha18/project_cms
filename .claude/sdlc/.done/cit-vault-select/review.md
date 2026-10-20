# cit-vault-select v3 — Review (stage 5)

Reviewed 2026-10-13 by Claude: inline review of the S0–S7 diff — money/coverage rules, ticket sync + concurrency, area-per-region
admin, vendor party content (no internal data leaked), RBAC/four-eyes, replica vs primary, frontend gating, other consumers of the
changed contracts. Separate reviewer agents (code-reviewer / database-reviewer / security-reviewer) were not run; available on request.

Status: **R1 + R2 fixed; no CRITICAL.** The code owner merges; the AI never self-approves. Manual browser
check outstanding (user). Docker build not run. Nothing committed.

## Important
| # | Finding | Evidence | Status |
|---|---|---|---|
| R1 | **Dev seed still maps branches, not regions.** `backend/scripts/dev/seed_acm_users.sql` (commit 7eb4fb6) fills `acm_area_branches` and backfills plans with the old per-branch join. Since 030 v3 plans come from `acm_area_regions`, so after a fresh seed "Area ACM Dev" has no region and every new request lands in the FR1.4 warning without a plan. Not a prod issue (dev script), but it blocks the manual check. | `grep acm_area_branches backend/scripts/dev/seed_acm_users.sql` → lines 22, 55. | **Fixed 2026-10-13** (user: "update seed ke region"): seed now inserts every free region code of active vendor branches into `acm_area_regions` and backfills one plan per waiting request with the `CreateMissingVaultPlansByRegion` SELECT. Dry run on dev (ROLLBACK): 36 regions mapped, 0 new plans. |
| R2 | **Amount with a leading zero could never be submitted.** The editor kept `"01000000"` as typed; the server returns `"1000000"`, so the dirty check stayed true and "Kirim untuk Approval" showed "Simpan perubahan" forever. | `toInput` compares strings. | **Fixed** (S8): input strips leading zeros; `VaultPlan.test.tsx` asserts it. |

## Checked, no finding
- Money: `bigint` in DB, `int64` in Go, decimal strings in JSON, `strconv.ParseInt` strict; DB CHECK `amount % denom = 0` backs the service.
- Coverage: shortfall per denom blocks submit (422 with the gap), excess needs a 10–500 reason, a denom not ordered counts as excess;
  computed again at submit from current rows, not trusted from the client.
- Tickets: advisory lock per (vault, date), number never reused (unique `ticket_number` + `(vault, date, seq)`), one row per
  (request, vault) for life; reject/cancel never touch tickets; no hard delete (`no_hard_delete_test`).
- One live plan per request: partial unique index + service reuse on re-approval.
- Vendor content: no saldo/capacity/tier/urgent/buffer (unit leak check); vendor scope unchanged (2.2b `ScopeAndNoLeak` green).
- RBAC: route + service; non-member → 404; ADMIN read-only on plans; Area ACM admin = documented GR#3 deviation (same as 2.2a).
- Reads: candidates/list on the replica, writes + read-after-write on the primary.

## Nit / minor
| # | Finding | Suggested follow-up |
|---|---|---|
| N1 | V6 consequence: any vault or amount change resends **all** replenish parties (they see every vault). Recorded in tests. | Accept, or show replenish parties only vaults that serve them, if vendors complain. |
| N2 | Inactive vault tickets (`vault_tickets`) are returned by the API but not shown in the UI; only the active ticket per row. | Add a ticket history list if ACM asks. |
| N3 | Candidates are fetched once with `urgent=true`; the editor hides tier 3 on non-urgent rows. The server still rejects tier 3 without urgent. | None. |
| N4 | `acm_area_branches`, `vendor_request_vault_assignments` and the backend `PUT /admin/acm-areas/{id}/branches` route stay (out of scope); the UI no longer uses them. | Drop in a later cleanup migration. |
| N5 | Coverage gaps are DB-error branches (`resetVaultPlansForReapproval` 65 %, `createVaultPlans` 67 %, `ForRequest` 67 %). Package 81.3 %. | None. |
| N6 | Editor row keys come from a module counter, also bumped when `rowsFromPlan` runs during render for comparison. Harmless (keys only). | None. |
| N7 | `-race` not run locally (CGO_ENABLED=0); parallel ticket issue covered by the advisory-lock integration test. | Run `go test -race` in CI. |
