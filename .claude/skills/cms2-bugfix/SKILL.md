---
name: cms2-bugfix
description: Fix a CMS2 bug end to end — route check (bugfix vs AI-DLC), diagnose, root cause across all callers, failing test first, minimal fix, review, then log it in .claude/bugfixes.md. Use when the user reports something broken, an error, a failing test/build, "bug", "debug", "error", "rusak", "gagal", "tidak jalan", or asks to fix a QC/lint finding.
---

# CMS2 bug fix

Source of truth: CLAUDE.md §4a (rule 0 + rule 3) and the template at the top of
`.claude/bugfixes.md`. If they disagree with this skill, CLAUDE.md wins.

## Step 0 — route, every time

Ask (CLAUDE.md §4a rule 0): *"Perlu dibuat AI-DLC baru (`.claude/sdlc/<feature>/`)
atau cukup dokumentasi bug fix?"* and wait.
- Suggest **AI-DLC** (→ `/cms2-sdlc`) when the fix touches auth, money/journal,
  reconciliation, migration/schema, or deletes (§4 #7), changes an API contract,
  or spans many files. The user still decides; if they pick bugfix for a §4 #7
  area, say so in the log entry ("Auth (Sec 4 #7) di-flag; user memilih dokumentasi bugfix").
- Otherwise suggest bugfix and continue here.

## Steps

| # | Do | Skill |
|---|---|---|
| 1 | **Reproduce + diagnose**: get the exact symptom (screen, endpoint, input, error text). Build hypotheses, narrow to one cause. Don't edit yet. | `mattpocock-skills:diagnosing-bugs` |
| 2 | **Root cause, all callers**: find every caller of the function you'll touch; fix once where they all route through, not only the reported path. | `graphify query/path/explain` (CLAUDE.md §1a) |
| 3 | **Failing test first**: a test that fails because of the bug (red), then the fix (green). Table-driven for Go; integration (`//go:build integration`) when the bug is in SQL/tx/apply. | `mattpocock-skills:tdd`, `ecc:golang-testing` |
| 4 | **Minimal fix**: smallest diff, one concern, no drive-by refactors (§4 #2). Match neighbouring code. | `ecc:golang-patterns` (Go), `ui-ux-pro-max` (UI) |
| 5 | **Verify**: commands from CLAUDE.md §2a for the touched module(s) — tests, `golangci-lint`, `pnpm … test/build/lint`. Fix until green. | `ecc:verification-loop` |
| 6 | **Review the diff**. | `/code-review`; + `/security-review` if input handling, access/scope, or §4 #7 |
| 7 | **Log** in `.claude/bugfixes.md` (below). | — |

### By bug type (use in step 1–4)

| Type | Use |
|---|---|
| Build / compile / type error | `/ecc:build-fix`, agents `ecc:go-build-resolver`, `ecc:react-build-resolver` |
| Button does nothing / wrong UI state though each function looks fine | `ecc:click-path-audit` |
| Error swallowed, wrong fallback, silent failure | agent `ecc:silent-failure-hunter` |
| Master-data approve fails / stale / 409 | `/cms2-master-data` |
| Lint / gocyclo / QC finding | `/simplify` (behaviour unchanged, existing tests stay green unmodified) |
| sqlc output wrong | regenerate with v1.31.1, `UserLeafe`→`UserLeave`, check `git diff --stat backend/internal/db/` |

## Log entry — `.claude/bugfixes.md`

Insert **at the top, below the `---`** (newest first), in the language of
recent entries (Indonesian), using the file's template:

```markdown
## YYYY-MM-DD — <short title>
- **Symptom**: what the user saw
- **Root cause**: why it happened
- **Fix**: what changed (files)
- **Tests**: automated check added/run; manual browser verification: outstanding
- **Commit**: <sha>
```

- **Tests**: name the new test, the commands run, coverage if it moved.
  Manual browser check = **"outstanding"** (or "tidak relevan" for backend-only) —
  never mark it done (Golden Rule #10).
- **Commit**: leave `<sha>` until the user says "commit"; then fill it in.

## Then

- Code changed → `graphify update .`
- A lesson worth keeping → `.claude/lessons-learned.md`.
- Item from `.claude/BE_debt.md` / `FE_debt.md` → update its Status.
- Fix changes a feature's behaviour → note it in that feature's `.claude/sdlc/<feature>/` artifact or ask whether to.

## Don't

- Patch only the symptom path when other callers share the bug.
- Change a test to make it pass unless the test itself is wrong (say why).
- Commit without the user's "commit".
