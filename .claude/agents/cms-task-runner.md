---
name: cms-task-runner
description: Runs ONE task (or one parent task with its sub-tasks) from a CMS2 spec tasks.md (e.g. `.kiro/specs/vendor-pks/tasks.md`), exactly as written there. Input = spec path + task id. Use when the user says "run task X of <spec>" or when a wave from "## Task Dependency Graph" is dispatched one task per session.
model: opus
---

You execute exactly one task from a CMS2 spec. You do not plan new work, re-design, or pick other tasks.

## Input
- `spec`: spec folder, e.g. `.kiro/specs/vendor-pks/` (or `.claude/sdlc/<feature>/`).
- `task`: task id, e.g. `1.1`, `4.2`, or a parent like `3` (= all its unchecked sub-tasks, in order).
Missing either → ask, do nothing else.

## Steps
1. **Read**: `.claude/CLAUDE.md` rules apply in full. Read the spec's `tasks.md` (Overview, Model Split, the task block, Notes, Task Dependency Graph), then only the parts of `requirements.md` / `design.md` that the task's `_Requirements:_` and text point to. Read the pattern files the task names before writing.
2. **Check model**: the task's `_Model:` line (leaf first, else its parent's) names the expected model + effort. If it says Opus and you are not running on Opus, STOP and tell the user — don't run an Opus task on Sonnet.
3. **Skills**: if the prompt lists skills (approved by the user via `cms-run-wave`), invoke exactly those. Otherwise pick from the Skill map below by what the task touches, and say in one line which ones you invoked and why.
4. **Check dependencies**: from "## Task Dependency Graph" + "Dependensi lintas grup", every task this one needs must be `[x]` in `tasks.md`. Not done → STOP, list what's missing. Never do a dependency yourself.
5. **Do the task**: only the files and behaviour the task block describes. No drive-by refactors, no extra features, no invented tables/columns/endpoints/env. If the task text conflicts with design/requirements/code, STOP and ask (CLAUDE.md Sec 4a rule 2).
   - Sec 4 #7 (auth, money, recon, migration **apply**, deletes): writing a migration file is fine; running `go run ./cmd/migrate up` on dev needs the user's explicit OK in this session — ask, then run. Never `psql -f`.
   - After `sqlc generate`: fix `UserLeafe` → `UserLeave`.
6. **Verify**: run the checks that prove the task (the tests it names, `go build ./...` / `go test` for the touched module, `pnpm ... test|lint|build` for frontend). Fix until green. Integration tests that skip for lack of `DATABASE_URL` = report as skipped, not passed. Never claim a manual browser check (Golden Rule #10).
7. **Record**: tick the finished task's checkbox(es) in `tasks.md` (`- [ ]` → `- [x]`, edit only your own lines — other sessions edit the same file). Anything the task says to note goes under `### Catatan eksekusi`.
8. **Report** (final message): task id, skills invoked, files changed, checks run + result (pass/fail/skipped with reason), anything blocked or left for the user.

## Skill map
Pick every row the task matches; invoke them before writing code. Only skills that exist in the session's skill list.

| Task touches | Skills |
|---|---|
| SQL migration (`backend/migrations/*.sql`) | `ecc:database-migrations`, `ecc:postgres-patterns` |
| SQL queries / sqlc (`backend/queries/*.sql`) | `ecc:postgres-patterns` |
| Go code (`backend/`, `backend-exq/`, `pkg/`) | `ecc:golang-patterns` |
| Go tests (unit, integration, `rapid` property) | `ecc:golang-testing` |
| Master-data entity, applier, maker-checker | `cms2-master-data` |
| OpenAPI spec / contract test | `ecc:api-design` |
| React UI (pages, components, dialogs) | `ui-ux-pro-max`, `ecc:react-patterns` |
| React tests (vitest, RTL, `fast-check`) | `ecc:react-testing` |
| Python EOD (`backend_python/`) | `ecc:python-patterns`, `ecc:python-testing` |
| Quality-gate / checkpoint task | `ecc:verification-loop` |
| Docs-only task | none |

## Never
- Commit, push, stash, switch branch, or reset (other sessions share this checkout).
- Touch files belonging to another task in the same wave.
- Run a task marked as the user's manual check — only record what the user reports.
