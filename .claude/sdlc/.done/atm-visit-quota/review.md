# atm-visit-quota — Review (stage 5)

Reviewed 2026-10-01 by Claude (inline review of the full diff: bugs/concurrency, security/RBAC, CLAUDE.md compliance; coverage measured; checks run against dev DB with migration 021).
Separate reviewer agents (code-reviewer / database-reviewer / security-reviewer) not run — available on request.
Status: **R1 fixed; N1 + N3 fixed (2026-10-01, user: "perbaiki N1 dan N3")**; no open Important; no CRITICAL. Code owner merges; AI never self-approves. Manual browser check still outstanding (user).

## Important
| # | Finding | Evidence | Status |
|---|---|---|---|
| R1 | **Approve path for a terminal with no `atms` row was untested.** `vendor_request_items.terminal_id` has no FK to `atms` (only `vendor_request_items_request_fk`), so a manual request can carry such a terminal; `recordVisit` skips it (`atm_not_found`, written to the audit entry). A regression there would block the whole approval. | `go tool cover`: `recordVisit` 62.5 % — the skip branch was never hit. | **Fixed** — `TestIntegration_VisitQuota_UnknownTerminalSkipped`: approve completes, known ATM 4 → 3, exactly 1 visit, ghost terminal gets no kuota row (`ErrAtmNotFound` on read). Green. |

## Nit / minor
| # | Finding | Suggested follow-up |
|---|---|---|
| N1 | `RejectCompletion` with an empty reason returns `ErrRejectReasonEmpty` → 422 with field `rejection_reason`, while the body key is `reason`. The UI blocks empty reasons, so only API clients see it. | **Fixed** — new sentinel `ErrCompletionReasonEmpty` → 422 `writeValidationError("reason", "wajib diisi")`. Tests: `TestRejectCompletion_EmptyReason`, `TestVendorRequestHandler_CompletionRoutes`. |
| N2 | `VisitQuotaCard` imports `ReasonModal` from the page module `VendorRequestDetail.tsx`, so the ATM-profile chunk pulls in the Vendor Request page. | Move `ReasonModal` to its own file (e.g. `components/ui/ReasonModal.tsx`) in a small refactor. |
| N3 | Audit Log page's action filter (`AuditFilterBar.tsx`) has a fixed list without `submit_completion` / `approve_completion` / `reject_completion` / `reset`. Rows display correctly (badge colour is regex-based), they just can't be picked in the filter. | **Fixed** — actions `submit_completion`/`approve_completion`/`reject_completion`/`reset` and entity types `atm_visit_quota`/`atm_visit` added. Test: `AuditLogPage.test.tsx` "offers the atm-visit-quota actions…". |
| N4 | Summary "Reset Kuota Vendor" resolves `vendor_id` by vendor **name** (the recap already groups by name). Two vendors with the same name would merge — pre-existing recap behaviour. | Group the recap by `v.id` if vendor names can repeat. |
| N5 | Any maker (not only the order's creator) may submit the laporan selesai — matches spec FR1 (maker roles), noted so the PO is aware. | None unless PO wants creator-only. |
| N6 | `completionMakerRoles` (service) duplicates `vendorRequestMakerRoles` (handler) — same pattern as the existing `checkerRoles`. | — |

## Security pass
- RBAC at **route and service** for every new action: `/complete` maker roles + `checkCompletionActor`; `/complete/approve|reject` checker roles + reporter ≠ checker (`ErrSelfApproval`); `/atm-visit-quotas` POSTs checker roles + `withTx` re-checks `isChecker`. Covered by `TestCheckCompletionActor`, `TestAtmVisitQuotaHandler_RoleGates`, `TestVendorRequestHandler_CompletionRoutes`, integration AC4/AC6.
- All SQL via sqlc parameters; no string-built SQL. `terminalId` path param is only a bound parameter; frontend `encodeURIComponent`s it.
- No secrets, no new env vars; no `dangerouslySetInnerHTML`; reasons rendered as text.
- No branch scoping for BRANCH-ATM-* roles — same as the rest of Vendor Request today (forecast-browser-summary OQ3), not widened here.

## Bugs / correctness pass
- **Concurrency**: request row `FOR UPDATE` serialises actions on one request (second approve → 409, AC3 sequential). Kuota rows are created with `ON CONFLICT DO NOTHING`, then locked `FOR UPDATE` and decremented in SQL — two approvals of different requests on the same ATM both count (`…_ParallelApprovals`: 5 → 3). Approve, vendor reset and cancel all lock kuota rows in `terminal_id` order (approve: results `ORDER BY terminal_id`; vendor reset: `ORDER BY a.terminal_id`; cancel locks one row) → no lock-order cycle.
- **Idempotency**: `atm_visits` UNIQUE `(vendor_request_id, atm_id)` + `ON CONFLICT DO NOTHING` — a duplicate never decrements.
- **Period boundary**: `reset_at` / `atm_visits.created_at` use `clock_timestamp()` (found by the integration harness: `now()` is constant in a tx). Cancel of a pre-reset visit → 409 (AC8).
- **Atomicity**: any failure inside approve rolls back visits + kuota + status (`…_FailedApproveRollsBack`).
- `cr_frequency` never written — asserted in `…_ResetAndCancel`; `package_frequencies` has no write query anywhere in the diff.
- Unknown package → no guessed kuota (visit recorded with `quota_known=false`; reset → 422).
- `is_requested` in the forecast uses `status NOT IN ('cancelled','rejected')`, so `completion_pending`/`completed` still count as requested — correct.

## Coverage
New backend files (unit + integration): `vendor_request_completion.go` functions 67–100 % (`checkCompletionActor`/`validateCompletionResults` 100 %, `ApproveCompletion` 88 %), `atm_visit_quota.go` 63–100 % (uncovered lines are DB-error wrap branches), `atm_visit_quota_handler.go` 55–100 % (uncovered: unauthenticated-actor and decode-error branches). Frontend: new components have dedicated tests (`VisitQuotaCard.test.tsx`, completion block in `VendorRequestDetail.test.tsx`, `ForecastTable`/`ForecastSummary` tests). Per-file 80 % not reached on every function — gap is error plumbing, not business branches.

## Definition of Done (CLAUDE.md Sec 11)
- [x] Matches requirement + module/table map — tables added to Sec 3 (ATM group), decision in Sec 12, `docs/data-map.md`, `docs/decisions.md`.
- [x] Correct auth path — internal (LDAP/JWT) CompanyPortal only; VendorPortal untouched.
- [x] Maker-checker + audit — laporan selesai is maker-checker (reporter ≠ checker); reset/cancel-visit = documented GR#3 deviation (immediate, audit in same tx, RBAC route + service).
- [x] Reads on replica / writes on primary — **deviation D1 (approved)**: quota reads use primary (read-after-write), same as forecast-browser-summary D1.
- [x] Money as numeric / timestamps timestamptz — no money in this feature; all new timestamps `timestamptz`.
- [x] Tests passing incl. auth/RBAC cases — Go unit + integration, frontend 1036 tests.
- [x] No secrets/config hardcoded; `.env.example` unchanged (no new config).
- [x] Response shape — flat JSON like neighbouring ATM handlers.
- [ ] Builds cleanly in Docker — not run in this session (`go build ./...` and `pnpm build` OK).

## Gates
- [ ] Manual browser check (user) — needs kelolaan seed on dev first (see `tests.md`).
- [ ] Commit split from the uncommitted Forecast Browser changes (plan D3).
- [ ] Code owner merge.
