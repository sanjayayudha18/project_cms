---
name: cms2-scope
description: Start or advance a CMS2 scope with Scope Driven Development (ScDD) — roadmap README + one phase file per phase (AC → design → tasks → test results → review → incidents) under .claude/dev-plan/plan-perscope/dev-phase-perscope/<n>-<scope>/, stopping at every human gate. Use when the user says "ScDD", "scope driven", "rencana per fase", "lanjut fase", or picks ScDD at CLAUDE.md §4a rule 0.
---

# CMS2 ScDD driver

Source of truth: CLAUDE.md §4a + `.kiro/steering/scope-driven-standard.md` (checklist,
gates, templates, traceability). This skill adds **order + which skill per step**.
If they disagree, CLAUDE.md wins, then the standard.

## Step 0 — every time

1. Method chosen? If not, ask CLAUDE.md §4a rule 0 (Kiro / ScDD / bugfix) with
   your suggestion from the entry checklist (standard Sec 2). Wait.
2. Inputs: scope number + name, area file `.claude/dev-plan/plan-perscope/<n>-<scope>.md`,
   source document + section. Missing → ask.
3. Folder exists? Read `README.md` + every phase file's `> Status:` and continue from
   the first unfinished gate. Code vs file disagree → STOP, ask.
4. Read the source section **fully** before writing AC — never AC from headings alone.

## Steps

| # | Do | Gate (user) | Skills |
|---|---|---|---|
| R | `README.md` from template: per-phase checklist (✅/❌ + evidence), phase table, dependencies, cross-phase decisions. Failing points → `<n>.0-keputusan.md` (questions only; **don't answer them**) | Roadmap accepted | `mattpocock-skills:grilling` for unclear splits |
| 1 | One phase file (only phases whose checklist is ✅): AC with source lines → Design → `## Gerbang Sec 4 #7` → Tasks + `### Task Dependency Graph`. Run standard Sec 7 traceability | Phase plan accepted | `mattpocock-skills:domain-modeling`, `ecc:database-migrations` + `ecc:postgres-patterns` (schema), `ecc:contract-first` (API), `cms2-master-data` (master data) |
| 2 | Execute tasks: `claude-cms-orchestrator` (`<phase file>`, wave N) or `cms-task-runner` per task. Fill `## Test Results` | Build + tests green | `ecc:tdd-workflow`, `ecc:golang-testing`, `ecc:verification-loop` |
| 3 | `## Review` (bugs/security/compliance, R1…, DoD §11) | Code owner merges → status `done <date> <sha>` | `/code-review`; `/security-review` mandatory for §4 #7 phases |
| — | After release: `## Incidents` + regression test | On-call | `mattpocock-skills:diagnosing-bugs` |

One phase file at a time unless the user asks for more. Phases may build in
parallel only where README dependencies allow.

## Gates — never self-approve

- Claude never writes `accepted` or `done`. Ask, then wait.
- Ambiguity mid-build → set the phase `blocked`, log the question, hold affected
  tasks, ask. Answer → new AC (never reuse numbers) → gate 1 again; >1 phase
  affected → README gate again.
- Migration on dev (`go run ./cmd/migrate up`) needs the user's OK in-session.
- No commit unless the user says "commit"; then report the SHA.

## At every gate

1. Update the README phase table and the scope's single row in `.claude/development-progress.md`.
2. Decision or new table applied → flag CLAUDE.md §3/§12 + `docs/data-map.md`.
3. Lessons → `.claude/lessons-learned.md`.
4. After code changes: `graphify update .`

## Don't

- Write a `requirements.md` / PRD (`/ecc:prp-prd`, `/ecc:plan-prd`) — AC replace it.
- `/ecc:feature-dev` — skips the gates.
- Move a Kiro spec already started into ScDD.
- Put area checklist or PO conflicts in README — they live only in the area file.
