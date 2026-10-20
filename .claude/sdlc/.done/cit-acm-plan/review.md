# cit-acm-plan — Review (stage 5)

Reviewed 2026-10-08 by Claude (inline review of the S1–S8 diff: state machine/concurrency, RBAC + four-eyes, money/capacity SQL,
notifications, frontend gating; checks re-run against dev DB with migrations 027 + 028).
Separate reviewer agents (code-reviewer / database-reviewer / security-reviewer) not run — available on request.
Status: **R1 + R2 fixed; no open Important; no CRITICAL.** Code owner merges; AI never self-approves. Manual browser check still
outstanding (user). Nothing committed yet.

## Important
| # | Finding | Evidence | Status |
|---|---|---|---|
| R1 | **A branch moved to another ACM area mid-flow broke the plan.** `ListVaultPlanAtms` derived a plan's ATMs purely from the branch's *current* area, contradicting FR7.3 ("mengeluarkan branch tidak mengubah rencana yang sudah ada"). After ADMIN moved a branch whose ATM already had an assignment row, the new area's plan listed that ATM too: saving it hit `vendor_request_vault_assignments_atm_uq` → **500**, while the old plan silently dropped the ATM from its list but its row still counted toward `vault_review`. | Read of `queries/vault_plans.sql` vs spec FR7.3; no test moved a branch after assignment. | **Fixed** — an ATM with an assignment row stays in the plan holding the row; only ATMs without a row follow the current area. Unique-key hit in `SaveAssignments` now maps to a 422 `ValidationError` as backstop. `TestIntegration_VaultPlan_BranchMovedArea` (unassigned ATM moves with branch; assigned ATM stays; other plan cannot take it → 422; both plans approve → `vault_review`). Green. Rejected alternative: gating `vault_review` on the ATM's *current* area — an already-approved plan of the old area is not editable, so the request would get stuck. |
| R2 | **ATM-SPV review notification linked ACM-USERs to a page they cannot open.** `vault_plan.reviewed` went to the area ACM-USERs and the request creator with one link `/replenishment/vendor-requests/{id}`; that route (backend + frontend) excludes ACM roles → 403 for ACM-USER. | `ReviewRequest` in `vault_plan_actions.go`; route roles in `vendor_request_handler.go`. | **Fixed** — one message per area plan to its ACM-USERs linking `/cit/vault-plans/{plan}`, plus one to the creator linking the request. Asserted in `TestIntegration_VaultPlan_BranchMovedArea`. Green. (Fixture now also cleans this test's notifications to the creator, an existing dev user — a pre-existing leak.) |

## Nit / minor
| # | Finding | Suggested follow-up |
|---|---|---|
| N1 | After R1, a branch moved into an area that had no plan for the request gets an **empty** draft plan (FR7.3 creates it; its ATMs stay with the old plan). It never blocks `vault_review` (coverage is per ATM) but shows in the ACM list with 0 ATMs. | Hide 0-ATM plans in the list, or skip creation when every ATM of that branch already has a row — only if ADMIN moves branches mid-flow in practice. |
| N2 | ATM-SPV vault review is not scoped by branch/area — any `ATM-SPV`/`BRANCH-ATM-SPV`/`ADMIN` may review any request, same as the existing Vendor Request approve. | Revisit with Vendor Request scoping if BRANCH-ATM-SPV must be limited to its own branches. |
| N3 | Four-eyes on the ATM-SPV review checks plan submitters/approvers (FR5.3) but not the request **creator**; a creator with a checker role (`ADMIN`) could vault-approve their own request after another checker approved it. Matches the spec text. | Add `req.CreatedBy == actor` → `ErrSelfApproval` if the PO wants it. |
| N4 | Column comment on `vendor_request_vault_assignments.saldo_snapshot` shows `"123000000.00"`; the service writes whole rupiah strings. Documented in data-map. | Fix the comment with the next migration that touches the table. |
| N5 | Frontend fetches candidates once per ATM row; fine for tens of ATMs per area (NFR1 not measured). | Batch endpoint if one plan grows past ~100 ATMs. |
| N6 | `features/vault-plan` imports `ReasonModal` from `vendor-request/VendorRequestDetail` (circular import; safe, render-time only). | Move `ReasonModal` to `components/ui` when next touched. |
| N7 | New JSON handlers decode bodies without `http.MaxBytesReader`, like the neighbouring Vendor Request handlers (only file uploads cap). | Global body-size middleware, project-wide. |
| N8 | `gofmt -l` lists several service files — only CRLF from `core.autocrlf=true` in the working tree; committed blobs are LF and clean (checked with `git show HEAD:… \| gofmt -l`). | None. |

## Security pass
- **RBAC at two layers**:
  - `/api/v1/vault-plans` is `RequireRoles(ACM-USER, ACM-SPV, ADMIN)`. The service re-checks the role per action (save/submit = ACM-USER, approve/reject = ACM-SPV, ADMIN read-only) and checks area membership for every read and write. A non-member gets 404, so other areas are never revealed.
  - Vault review uses the existing Vendor Request checker routes + `isChecker` in the service.
  - Area ACM admin is `ADMIN` at route + service. DSR mapping is `ADMIN`/`ADMIN_PARAM` at route + `Submit`.
- **Maker ≠ checker**:
  - ACM layer: the plan submitter cannot approve or reject it.
  - Across layers: the ATM-SPV reviewer cannot be any plan submitter or approver (FR5.3).
  - Single role per user.
- **Audit + transactions**:
  - Every transition writes `audit_logs` in the same tx.
  - Notifications are written inside the tx (rollback-safe).
  - Lock order is the request `FOR UPDATE`, then the plan, in every path. The parallel-approve test passes 3x.
- **Input**:
  - Urgent reason 10–500 runes and reject reasons 1–500 are enforced in the service and as DB CHECKs.
  - Tier 3 requires urgent (service + DB CHECK).
  - The vault branch must be active, CASH/ATM_CASH and have a region code, re-validated on submit.
  - All SQL goes through sqlc parameters.
- **Money**: int64 whole rupiah in Go, `numeric` truncated in SQL, decimal strings on the wire and in snapshots. No float server-side; the frontend formats the strings as numbers (< 2^53).
- **Frontend**: no `dangerouslySetInnerHTML`. Route guards mirror the backend roles, and the API enforces anyway. No secrets, no new env vars.

## Bugs / correctness pass
- **Capacity**:
  - DSR saldo is one upload per (vendor, report_date) (`dsr_uploads_date_vendor_uq`), so there is no double count across re-uploads.
  - Saldo is "unknown" (not 0) when a branch has no vault, a vault is unmapped, or a denom cell is NULL.
  - Demand per FR3.2 option (a) as decided by the user.
- **State machine**:
  - Legacy `approved` requests keep the old path (`vault_flow=false`).
  - A rejected laporan selesai returns to `ready` only for `vault_flow` requests.
  - Cancel cascades plans to `cancelled`.
  - Covered by the state-machine property test + `vault_flow_integration_test.go`.
- **Read-after-write**: mutations return detail from the primary; lists, detail and candidates read the replica (NFR2).

## Checks run (2026-10-08, after fixes)
| Command | Result |
|---|---|
| `sqlc generate` (v1.31.1) + `UserLeafe` fix | only `vault_plans.sql.go` changed by the fix |
| `go build ./... && go vet -tags integration ./internal/service/` | clean |
| `DATABASE_URL=… go test -tags integration -count=1 ./...` | all packages ok (incl. new `TestIntegration_VaultPlan_BranchMovedArea`) |
| `pnpm --dir frontend/CompanyPortal-Vite run test` / `lint` / `build` | 1071/1071, clean, built (no frontend change in review) |

## Definition of Done (CLAUDE.md Sec 11)
- [x] Matches spec + module/table map (Sec 3 updated with 027/028)
- [x] Auth path (LDAP internal) + scoped RBAC (route + service, area membership)
- [x] Maker-checker + audit (own state machine + Area ACM immediate-apply = documented deviations, Sec 12)
- [x] Reads on replica, writes on primary
- [x] Money as integer rupiah / numeric, timestamps timestamptz
- [x] Tests incl. RBAC, four-eyes, money/capacity, concurrency
- [x] No secrets/config hardcoded (no new env vars)
- [x] Flat JSON matching neighbours in ATM `backend`
- [ ] Builds in Docker — not run in this review
- [ ] Manual browser check — **outstanding (user)**
