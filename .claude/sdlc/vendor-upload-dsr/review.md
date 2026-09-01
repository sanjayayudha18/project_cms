# Review + deploy: vendor upload DSR

Status: draft. Stage: 5 Deploy. Reads: the diff, `spec.md`, `plan.md`.
The agent acts up to the production gate and nothing past it. Human approves via branch protection.

## Review passes (tag each finding by pass)
- Bugs: logic errors, broken edge cases, sign/rounding on money, subtle regressions.
- Security: vendor scoping (FR8), no secrets in diff, PII in logs, injection on upload.
- Compliance: matches `spec.md` + `plan.md`; maker-checker + audit wired; primary/replica routing.

## Important vs Nit
Reserve "Important" for anything that breaks behaviour, leaks data, or breaches a
policy (auth, money, reconciliation, migrations, deletes - CLAUDE.md Sec 4). Style/naming = nit.

## Definition of Done gate (CLAUDE.md Sec 11)
- [ ] Matches requirement + module/table map
- [ ] Correct auth path (vendor local), scoped RBAC
- [ ] Audit_log wired; maker-checker decision resolved
- [ ] Reads on replica, writes on primary
- [ ] Money numeric, timestamps timestamptz
- [ ] Tests passing incl. auth/RBAC/money cases
- [ ] No secrets/config hardcoded; .env.example updated if needed
- [ ] Builds cleanly in Docker

## Approval gates (hooks / branch protection)
- [ ] Code-owner approval required (branch protection).
- [ ] Migration/infra edits need change ticket (none expected here).
- [ ] Production deploy needs named release authorization.

## Findings
<!-- Filled by the reviewer agent + humans. -->
- (none yet)
