---
name: cms2-sdlc
description: Start or advance a CMS2 feature through the 6-stage AI-Native SDLC (.claude/sdlc/<feature>/ intent → spec → plan → tests → review → incidents), using the right skill per stage and stopping at every human gate. Use when the user starts a new feature, asks for an intent/spec/plan/tests/review/incidents artifact, or says "fitur baru" / "AI-DLC baru" / "lanjut tahap".
---

# CMS2 SDLC driver

Source of truth: CLAUDE.md §4a + `.claude/sdlc/README.md` (templates, rules,
"Current features" table). This skill only adds **which skill to use per stage**
and the gate discipline. If they disagree, CLAUDE.md wins.

## Step 0 — every time, before anything

1. Ask (CLAUDE.md §4a rule 0): *"Perlu dibuat AI-DLC baru (`.claude/sdlc/<feature>/`)
   atau cukup dokumentasi bug fix?"* Wait. Bug fix → `.claude/bugfixes.md`
   (newest first, its template), not this skill.
   Anything touching auth, money/journal, reconciliation, migration/schema, or
   deletes → always the full chain.
2. Check `.claude/sdlc/<feature>/` exists. If yes, find the **latest accepted**
   artifact and continue from the next stage. If code and artifact disagree → STOP, ask.
3. Feature folder name: kebab-case, matches existing style (`cit-vault-select`,
   `replenish-ticket`).

## Stages

Write every output **into the stage's file using the README template** — never a
separate PRD/ADR/plan file. One stage at a time; never start the next stage
until the user says the current artifact is accepted.

| # | Artifact | Primary skill | Also use when relevant |
|---|---|---|---|
| 1 Plan | `intent.md` | `ecc:intent-driven-development` | `mattpocock-skills:grilling` (stress-test), `ecc:product-lens` (is it worth building) |
| 2 Design | `spec.md` | `mattpocock-skills:domain-modeling` | `ecc:contract-first` (BE↔FE API), `ecc:database-migrations` + `ecc:postgres-patterns` (new tables/columns), `ecc:architecture-decision-records` (→ `.claude/docs/decisions.md`), `cms2-master-data` (master-data entity) |
| 3 Build | `plan.md` | `/ecc:plan` (plan mode, wait for OK) | `ecc:prp-plan` (deep pattern extraction), `ecc:golang-patterns`, `ui-ux-pro-max` (UI), `cms2-master-data` |
| 4 Test | `tests.md` | `ecc:tdd-workflow` | `ecc:golang-testing`, `ecc:verification-loop` |
| 5 Deploy | `review.md` | `/code-review` | `/security-review` (mandatory for §4 #7 areas) |
| 6 Maintain | `incidents.md` | `mattpocock-skills:diagnosing-bugs` | `engineering:incident-response`; each incident → regression test; a breach → new `intent.md` |

### Stage notes
- **1 intent.md**: originator's own words. List open questions — **do not answer them**.
  Leave stages 2–6 as stubs.
- **2 spec.md**: FR/NFR, API, data model, out-of-scope. Flag every §4 #7 item
  explicitly. New table/column → propose it here (CLAUDE.md §3 rule); a documented
  deviation from GR#3 must be stated and justified.
- **3 plan.md**: files, order, risks, proof (→ tests), alternatives not taken.
  Keep it in sync with the diff while building. Small diffs, one concern.
- **4 tests.md**: commands from CLAUDE.md §2a, targets, test ↔ spec FR trace.
  Fix until green. Manual browser check = **"outstanding (user)"** — never mark done
  (Golden Rule #10).
- **5 review.md**: bugs / security / compliance passes, Important vs nit,
  CLAUDE.md §11 DoD checklist. Code owner merges, not Claude.
- **6 incidents.md**: signals, response tiers, log, evals added.

## Gates — never self-approve

- Stage 1–2: product owner accepts. Stage 3: engineer accepts. Stage 4: build +
  tests green. Stage 5: code owner merges. Claude asks, then waits.
- No commit unless the user says "commit"; then report the SHA.

## When a stage finishes

1. Update the feature's row in `.claude/sdlc/README.md` → "Current features".
2. Update `.claude/development-progress.md`.
3. Lessons from review/incidents → `.claude/lessons-learned.md`; a durable rule →
   propose a CLAUDE.md change (reviewed like code).
4. Decision or new table applied → flag the CLAUDE.md §3/§12 update + `docs/data-map.md`.
5. After code changes: `graphify update .`

## Don't

- `/ecc:prp-prd`, `/ecc:plan-prd` — their PRD duplicates intent/spec.
- `/ecc:feature-dev` — runs end-to-end and skips the human gates.
- Legacy `.kiro/specs/*` features stay there (§4a rule 7); don't migrate them.
