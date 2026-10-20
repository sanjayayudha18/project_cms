# notification (Phase 0.3) — Review (stage 5)

Reviewed 2026-10-07 by Claude (inline review of commit `eb457cf` — bugs/concurrency, security/RBAC, CLAUDE.md compliance; coverage measured; checks run against dev DB with migration 025).
Separate reviewer agents (code-reviewer / database-reviewer / security-reviewer) not run — available on request.
Status: **R1–R3 fixed in this review (uncommitted)**; no open Important; no CRITICAL. Code owner merges; AI never self-approves. Manual browser check + real SMTP send still outstanding (user).

## Important
| # | Finding | Evidence | Status |
|---|---|---|---|
| R1 | **Stale notification list.** Both portals use a global `staleTime` of 5 min (`CompanyPortal routes/__root.tsx`, `VendorPortal lib/queryClient.ts`). The unread badge polls every 60 s regardless, but re-opening the bell panel / re-visiting the Notifikasi page within 5 min served the cached list — the badge said "1 baru" while the list did not show it. | Reproduced by tests that mirror the app `staleTime`: both fail without the fix. | **Fixed** — `staleTime: 0` on the list queries (`CompanyPortal features/notifications/hooks.ts` `useRecentNotifications`, `VendorPortal useNotifications`). Tests: `NotificationBell.test.tsx` "refetches the list every time the panel is opened", `useNotifications.test.tsx` "refetches the list when the page mounts again". |
| R2 | **Coverage below the 80 % rule** (CLAUDE.md Sec 8): `internal/notification` 77.1 % — `NewWorker`/`Run` 0 %, `tick` error path, SMTP dial failure untested. | `go test -tags integration -coverprofile` | **Fixed** — `TestWorker_RunStopsOnCancel`, `TestWorker_TickKeepsRetryingPurgeAfterError` (a failed purge is retried next tick, not marked done), `TestNewWorker_Defaults`, `TestSMTPMailer_DialFailure`, `TestSMTPMailer_DefaultTimeouts` → **83.9 %**. `handler/notification_handler.go` 77.8–100 % per func. |
| R3 | **`Message.Link` not validated.** Both portals navigate to `link` as-is (`router.history.push` / `navigate({ href })`). Today the only caller passes a constant in-app path, but a future caller could store an absolute or `javascript:` URL. | `notification.go` `validate` | **Fixed** — `Send` rejects a link that is not an in-app path (`/…`, not `//…`) with `ErrInvalidMessage`. Test cases added to `TestSend_ValidatesMessage` (absolute, protocol-relative, `javascript:`). |

## Nit / minor
| # | Finding | Suggested follow-up |
|---|---|---|
| N1 | `sendBatch` keeps the claim tx open while talking to SMTP (marked `ponytail:`). On shutdown mid-batch, or if recording a result fails, the batch rolls back and those emails are sent again (at-least-once). | Claim → commit `sending` → send outside tx → mark, if duplicates ever matter. |
| N2 | Rows written `pending` while SMTP was on stay `pending` forever if SMTP is later switched off, and go out in a burst when it is switched back on. Retention never purges `pending`. | Mark stale `pending` (> N days) as `failed` in the purge, when this happens in practice. |
| N3 | The approving ATM-SPV also receives the over-quota notification about their own approval. | Matches spec S1 ("semua ATM-SPV aktif"); exclude the actor only if the PO asks. |
| N4 | No rate limit on `/api/v1/notifications` — same as other authenticated ATM endpoints; polling is 1/min/user. | Covered by any future global limiter. |
| N5 | VendorPortal page shows the newest 100, no paging (`ponytail:` in the hook). | Add paging when a vendor gets > 100 within the 90-day retention. |
| N6 | VendorPortal `lib/types.ts` `Notification` + `data/notifications.json` mock are now only used by two property tests that check the mock itself (`routing.property.test.ts`, `dataFilters.property.test.ts`). | Remove the mock + those assertions in a cleanup change. |
| N7 | `go test -race` not run (Windows without cgo). The worker is single-goroutine; `lastPurge` is only touched there. | Run `-race` in CI (Linux). |

## Security pass
- **Scoping**: every handler uses the JWT caller (`actorFromRequest` → claim `id`); there is no user parameter. `MarkNotificationRead` filters `WHERE id = $1 AND recipient_user_id = $2` → someone else's id is a 404 with no existence leak. Covered by `TestNotificationHandler_MarkReadOwnOnly`, `TestIntegration_RepositoryScopesToOwner`. Route behind `RequireAuth` (all roles incl. `VENDOR-USER`) — intended: the data is self-scoped.
- **Email header injection**: addresses normalised + `net/mail.ParseAddress` (must equal the bare address) at enqueue; the mailer re-rejects CR/LF and invalid recipients; subject CR/LF stripped then RFC 2047 Q-encoded; body quoted-printable. `TestSMTPMailer_RejectsHeaderInjection`, `TestSend_DedupesUsersAndEmails` (invalid PIC address skipped).
- **SMTP**: STARTTLS whenever offered (TLS ≥ 1.2, `ServerName` verified); `PlainAuth` refuses to send credentials over plain text to a non-localhost host (net/smtp). Credentials only from env, never logged; `last_error` truncated to 500 chars; logs carry `notification_email_id`/status/attempt only — no address or body.
- **XSS**: titles/bodies rendered as React text in both portals; no `dangerouslySetInnerHTML`. Links restricted to in-app paths (R3).
- All SQL via sqlc parameters. No new dependency (stdlib `net/smtp`, `mime`, `net/mail`).

## Bugs / correctness pass
- **Atomicity**: `Send` uses the caller's tx-bound queries; a notify failure rolls back the whole approve (visits + kuota + status) — `TestIntegration_OverQuotaNotifyFailureRollsBackApprove`; a rolled-back caller tx leaves no rows — `TestIntegration_SendRollsBackWithCallerTx`.
- **Concurrency**: outbox claimed with `FOR UPDATE SKIP LOCKED` → two `cmd/api` instances never send the same row in parallel; purge is idempotent. Notification write happens inside the approve tx that already holds the vendor-request row lock — no new lock ordering.
- **Retry**: 4 attempts total (1 + 3 retries after 1/5/15 min), then `failed` — `TestWorker_SendBatch`.
- **Read/write split**: list + unread-count on the replica, mark read/read-all on the primary (`repository_topology_test.go`); frontends update their cache after a write instead of refetching from a lagging replica (FR4.5).
- **Shutdown**: worker context is cancelled after the signal; `Run` returns (`TestWorker_RunStopsOnCancel`).

## CLAUDE.md compliance (Sec 11 DoD)
- [x] Module/table map: `notifications` was listed; `notification_emails` approved with the spec and added to Sec 3 (Core); migration `025` recorded in Sec 12.
- [x] Auth path unchanged; RBAC = self-scoping at service query level (route: `RequireAuth`).
- [x] Maker-checker: not applicable (notifications are side effects); the triggering approve keeps its own maker-checker + audit. **Documented deviations** (Sec 12): no `audit_logs` per notification / mark-read; retention is a hard delete after 90 days. `no_hard_delete_test` guards master-data tables only — `notifications` is not in its list on purpose.
- [x] Replica for reads, primary for writes (Sec 6).
- [x] Timestamps `timestamptz`, shown in Asia/Jakarta (CompanyPortal bell `timeZone: "Asia/Jakarta"` + "WIB"; VendorPortal page fixed to `Asia/Jakarta` too). No money fields.
- [x] Tests passing (Go unit + integration, both portals). Coverage `internal/notification` 83.9 %.
- [x] No secrets hardcoded; `backend/.env.example` updated (`SMTP_*`, `NOTIFICATION_EMAIL_POLL_INTERVAL`).
- [x] Response shape: flat JSON like neighbouring ATM handlers (Sec 5).
- [ ] Builds in Docker — not run here (no Docker build in this session); `go build ./...` and both `vite build` OK.
- [ ] Manual browser check — **user** (Golden Rule #10).
- [ ] Real SMTP relay send — waiting for relay host/credentials.

## Checks run (2026-10-07, after R1–R3)
| Command | Result |
|---|---|
| `cd backend && go build ./... && go vet ./internal/notification/` | OK |
| `cd backend && DATABASE_URL=<dev, localhost> go test -count=1 -tags integration ./internal/...` | OK (all packages) |
| `go test -tags integration -coverprofile … ./internal/notification/` | 83.9 % |
| `gofmt -l` on notification files | clean |
| `pnpm --dir frontend/CompanyPortal-Vite exec vitest run` / `lint` / `build` | 123 files, 1051 tests pass / clean / OK |
| `pnpm --dir frontend/VendorPortal-Vite exec vitest run` / `build` | 17 files, 105 tests pass (pre-existing flaky `routeGuard` passed this run) / OK |
| R1 tests with the fix removed | both fail (1 failed each) → fix restored, both pass |
