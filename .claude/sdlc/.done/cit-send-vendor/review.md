# cit-send-vendor — Review (stage 5)

Reviewed 2026-10-09 by Claude: an inline review of the S1–S9 diff. It covered:
- state machine, resend and concurrency
- vendor scope / cross-vendor leakage
- RBAC and four-eyes
- notifications and audit
- frontend gating
- other consumers of `vendor_requests.status`
- coverage of the new service files

Checks were re-run against the dev DB with migration 029. Separate reviewer agents (code-reviewer / database-reviewer /
security-reviewer) were not run; they are available on request.

Status: **R1 fixed; no open Important; no CRITICAL.** The code owner merges; the AI never self-approves. Manual browser check
is still outstanding (user). Docker build not run. Nothing committed yet.

## Important
| # | Finding | Evidence | Status |
|---|---|---|---|
| R1 | **The internal "Status Vendor" panel read was never exercised against real rows.** `VendorOrderService.RequestParties` (`ListVendorPartiesForRequest` + `ListVendorPartyEventsForRequest`) was 0% covered. The handler test used a fake and S1 only executed the SQL on empty input, so a wrong join / NULL scan (e.g. `decided_by_name` via LEFT JOIN) would have surfaced only in the browser. | `go tool cover -func` on the integration run: `RequestParties 0.0%`. | **Fixed.** `TestIntegration_VendorOrder_ReplenishReject` now asserts 4 parties (replenish listed first), the rejected party's branch, reason, decider and time, and 4 `sent` + 1 `rejected` events with actor and time. The panel order changed to replenish → vault (`ORDER BY p.role`). Green. |

## Found and fixed during build (recorded in `plan.md`)
- **S5:** `UpdateVendorRequestCompletion` stamped `completion_rejected_*` only for `approved`/`ready`. A laporan selesai rejected back to `vendor_accepted` therefore lost its reason and rejecter. `vendor_accepted` was added, and `TestIntegration_VendorOrder_CompletionGateAndCancel` asserts it.
- **S8:** the `TestProperty8_StateMachineTransitionSet` spec table still expected `vault_review → ready`. Since S3 it passed only because rapid had not drawn that pair. The table was updated (and the stale rapid `.fail` record removed); `-count=20` is green.
- **S3:** the fixture cleanup did not remove party rows. A failed run leaked request 2500 + 2 vendors in dev; these were removed once, scoped to that run. `cleanupVendorParties` now runs in both fixtures.
- **S7:** the VendorPortal `routeGuard` property test timed out (40 interactive runs per `it` against the 5 s default) once the suite grew. It now has an explicit 30 s timeout and uses `findByTestId` after `settle()`.

## Nit / minor
| # | Finding | Suggested follow-up |
|---|---|---|
| N1 | The VendorPortal API returns the raw `request_status` (e.g. `vault_assignment`, `vendor_rejected`). It is not sensitive, and the UI maps it to "Sedang direvisi CIMB" / "Dibatalkan", but it exposes internal workflow names. | Map server-side to `active` / `revising` / `cancelled` if the vendor UI ever needs more. |
| N2 | Notifications go one per party: a vendor-wide user whose vendor holds 3 parties of one request gets 3 in-app + 3 emails. | Group per (request, user) in `notifyParties` if vendors complain. |
| N3 | While a request is back with ACM/ATM-SPV, pending parties are frozen (FR4.5). If their content is unchanged on resend they become decidable again **without** a notification (FR2.2 notifies only changed parties). | Send a "dapat diputuskan lagi" notice to unchanged pending parties on resend, if the PO wants it. |
| N4 | `GET /vendor-requests/{id}/vendor-parties` uses the Vendor Request viewer roles without branch scoping, same as the existing request detail (2.2a N2). | Revisit with Vendor Request scoping. |
| N5 | Coverage: the new service functions are at 72–100% (uncovered = DB error branches; `afterVaultReject` / `moveRequest` notifier-nil paths). The `internal/handler` unit package is 66.8% — it was under 80% before this feature. | Raise handler coverage as its own task. |
| N6 | JSON bodies (`/reject`, `/vendor-return`) are decoded without `http.MaxBytesReader`, like the neighbouring handlers (2.2a N7). | Project-wide body-size middleware. |
| N7 | VendorPortal suite still flakes occasionally: fast-check counterexample `/login?redirect=0` → `SearchParamError` (`z.string()` in `routes/login.tsx`). This is an old bug, outside 2.2b. | Separate task chip "Fix VendorPortal login crash on numeric redirect". |
| N8 | `data/orders.json` is kept: it is no longer imported by the app, only by `dataFilters.property.test.ts`. | Drop it with that test's next rewrite. |
| N9 | `-race` cannot run locally (CGO_ENABLED=0). The parallel-accept test is stable `-count=5` thanks to the request `FOR UPDATE` lock. | Run `go test -race` in CI. |
| N10 | `gofmt -l` lists a few service files: CRLF from `core.autocrlf=true` only (same as 2.2a N8). | None. |

## Security pass
- **Vendor scope (Sec 4 #7, the main risk):**
  - The route requires `RequireRoles("VENDOR-USER")`.
  - The service then requires: role `VENDOR-USER`; a `users.vendor_id` (read from the primary per call) equal to the JWT `vendor_id` claim, else `ErrNotAuthorized`; and `users.vendor_branch_id` pinning when set.
  - Scope is applied in SQL (`b.vendor_id = $vendor AND (pin IS NULL OR p.vendor_branch_id = pin)`). The URL id is a party id, so every read and action is one scoped lookup.
  - Out of scope → 404, also for accept/reject.
  - `TestIntegration_VendorOrder_ScopeAndNoLeak` covers another vendor, a sibling branch, an internal user and a stale claim.
- **Content whitelist:** `content` is built only from the columns of `ListRequestPartyRows` (no saldo/kapasitas, tier, urgent, price). `TestIntegration_VendorParty_SendAndResync` asserts none of those words appear, and that a vault party holds only its own ATM.
- **Four-eyes / RBAC:**
  - `vendor-return` is checker roles at the route plus `isChecker && != creator` in `checkActor`.
  - Cancel of the 3 new statuses is checker-only.
  - VENDOR-USER cannot reach the internal routes, and internal users cannot reach the vendor routes (handler tests).
- **Audit + transactions:**
  - Every send/resend/withdraw (request audit `after.vendor_parties`), decision (party audit), request transition, return and cancel is audited in the same tx.
  - Party events are append-only, and parties/events are never deleted (`no_hard_delete_test`).
  - Notifications are written in the tx.
  - Lock order is the request `FOR UPDATE` → parties in every path (send, decide, cancel), so the last-accept transition happens once.
- **Input:**
  - Reasons are trimmed, 10–500 (vendor) / 1–500 (return), and DB CHECKs back them.
  - `party_status` is whitelisted; dates are parsed strictly.
  - Notification links are relative paths only (`/orders/{id}`, `/cit/vault-plans/{id}`, `/replenishment/vendor-requests/{id}`).
- **Money:** `amount_replenish` stays bigint full IDR end to end, with `currency: "IDR"` explicit. The UI uses `tabular-nums`, right-aligned.

## Compliance (CLAUDE.md Sec 11 DoD)
- [x] Matches spec FR1–FR9 / Sec 3 table map (CLAUDE.md Sec 3 + data-map updated, migration 029).
- [x] Auth path: vendor = local JWT, scoped; internal unchanged.
- [x] Maker-checker: the request flow is unchanged; vendor decisions are a **documented deviation** (own state machine, audited) in CLAUDE.md Sec 12 / decisions.md.
- [x] Reads: vendor list + internal panel on the replica; scope, detail and actions on the primary.
- [x] Money bigint / IDR explicit; timestamps `timestamptz`.
- [x] Tests incl. RBAC, scope/leak, four-eyes, concurrency: Go integration, VendorPortal 113, CompanyPortal 1077.
- [x] No new env/config; no secrets.
- [x] Flat JSON, matching the ATM backend.
- [ ] Docker build — not run.
- [ ] Manual browser check — outstanding (user). Seed first: Area ACM + DSR mapping + vendor users per branch.
