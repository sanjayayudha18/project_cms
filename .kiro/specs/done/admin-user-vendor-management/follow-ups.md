# Follow-ups — admin-user-vendor-management

Recorded at the close of this spec (Task 11, 2026-09-16). Not tracked elsewhere yet — no `.kiro` spec exists for these two items on their own, so this is the record until one is filed.

## 1. Pin the sqlc version

`sqlc generate` was run repeatedly during Tasks 2 and 6 and consistently drifted unrelated files on every invocation:
- `models.go`'s `AuditLog.IP` field re-casing to `Ip`.
- `queries/approval.sql`'s `user_leaves` table getting inflected to the Go type `UserLeafe` instead of `UserLeave` (a genuine sqlc v1.31.1 inflector bug — no working config fix was found; see the doc comment above `CreateUserLeave` in `backend/queries/approval.sql`).
- Unrelated regeneration of `auth.sql.go`, `audit.sql.go`, `audit_log.sql.go` with cosmetic diffs against files that were otherwise untouched.

Every one of these was worked around by hand (revert unrelated files via `git checkout --`, `sed`-fix the casing/typo) rather than fixed at the source, both in this spec and in the prior RBAC Settings Menu work referenced in `tasks.md`'s Task 2 notes. Pin the exact sqlc version in `backend/tools.go` (or wherever the project tracks tool versions) and in CI, so `sqlc generate` is reproducible and this drift stops recurring on every regenerate.

## 2. Fix migration `017`

`sqlc generate` is currently blocked outright by a pre-existing bug in migration `017`: a missing table name (see `requirements.md`'s sqlc note and `design.md`'s "sqlc-generate blocker" callout). Until `017` is fixed, any new query file requires the same manual `internal/db/*.sql.go` hand-authoring workaround this spec used for `users_admin.sql.go`/`vendors_admin.sql.go`. Fixing `017` removes the need for that workaround on the next spec that touches sqlc.

## Not a follow-up here, for context

The search-index migration originally scoped for the admin Users/Vendors screens was deferred at Task 0/1 of this spec (see `tasks.md`) — not an oversight, a resolved decision to ship without it. If search-index needs come up again, re-open that decision rather than assuming this file covers it.
