---
name: cms2-sdlc
description: Start or advance a CMS2 feature through the 6-stage AI-Native SDLC as a hybrid Kiro spec (.kiro/specs/<feature>/ requirements → design → tasks → test results → review → incidents), using the right skill per stage and stopping at every human gate. Use when the user starts a new feature, asks for a requirements/design/tasks/review/incidents artifact, or says "fitur baru" / "spec Kiro baru" / "AI-DLC baru" / "lanjut tahap".
---

# CMS2 SDLC driver (hybrid Kiro + gates)

Source of truth: CLAUDE.md §4a + `.kiro/steering/kiro-spec-standard.md` (headings,
traceability checklist, Sec 7 stage/gate map). This skill only adds **which skill
to use per stage** and the gate discipline. If they disagree, CLAUDE.md wins.

## Step 0 — every time, before anything

1. Ask (CLAUDE.md §4a rule 0): *"Perlu dibuat spec Kiro baru (`.kiro/specs/<feature>/`)
   atau cukup dokumentasi bug fix?"* Wait. Bug fix → `.claude/bugfixes.md`
   (newest first, its template), not this skill.
   Anything touching auth, money/journal, reconciliation, migration/schema, or
   deletes → always the full chain.
2. Check `.kiro/specs/<feature>/` (and `.kiro/specs/done/`). If yes, find the
   **latest file with `> Status: accepted`** and continue from the next stage.
   A feature still in `.claude/sdlc/<feature>/` → legacy chain (`.claude/sdlc/README.md`), finish it there.
   If code and artifact disagree → STOP, ask.
3. Folder name: kebab-case, matches existing style (`vendor-pks`, `vendor-export-pks`).
   New spec also gets `.config.kiro` (`{"specId": "<uuid>", "workflowType": "requirements-first", "specType": "feature"}`).

## Stages

Every file starts with `# <Title>` then `> Status: draft`. Only the user changes it
to `> Status: accepted YYYY-MM-DD (<role>)`. One stage at a time; never start the
next stage until the user says the current artifact is accepted.

| # | Artifact | Primary skill | Also use when relevant |
|---|---|---|---|
| 1 Plan | `requirements.md` `## Introduction` (+ open questions) | `ecc:intent-driven-development` | `mattpocock-skills:grilling`, `ecc:product-lens` |
| 2 Design | `requirements.md` (EARS) → `design.md` | `mattpocock-skills:domain-modeling` | `ecc:contract-first`, `ecc:database-migrations` + `ecc:postgres-patterns`, `ecc:architecture-decision-records` (→ `.claude/docs/decisions.md`), `cms2-master-data` |
| 3 Build | `tasks.md` | `/ecc:plan` (wait for OK) | `ecc:golang-patterns`, `ui-ux-pro-max` (UI), `cms2-master-data`; run per task/wave with `cms-task-runner` / `cms-run-wave` |
| 4 Test | `tasks.md` `## Notes` → `### Test Results` | `ecc:tdd-workflow` | `ecc:golang-testing`, `ecc:verification-loop` |
| 5 Deploy | `review.md` | `/code-review` | `/security-review` (mandatory for §4 #7 areas) |
| 6 Maintain | `incidents.md` | `mattpocock-skills:diagnosing-bugs` | `engineering:incident-response`; each incident → regression test; a breach → new spec |

### Stage notes
- **1 Introduction**: originator's own words: problem, outcome, constraints, scope,
  §4 #7 areas. List open questions — **do not answer them**. Gate: PO accepts and
  answers them (mark ✅ DIPUTUSKAN PO <date>) before EARS requirements.
- **2 requirements + design**: Kiro headings exactly (standard Sec 2–3). New
  table/column → propose here (CLAUDE.md §3); a GR#3 deviation must be stated and
  justified. Gate: PO accepts requirements, engineer accepts design.
- **3 tasks.md**: standard Sec 4 (Model Split, `_Requirements:_`, Task Dependency
  Graph) + traceability checklist Sec 5. Keep in sync with the diff. Small diffs.
- **4 Test Results**: commands from CLAUDE.md §2a, date, pass counts/coverage,
  trace to Requirement/Property. Manual browser check = **"outstanding (user)"** —
  never mark done (Golden Rule #10).
- **5 review.md**: `## Review Passes`, `## Findings` (R1…, Important vs nit, status),
  `## Definition of Done` (CLAUDE.md §11). Code owner merges, not Claude.
- **6 incidents.md**: `## Signals`, `## Incident Log` (+ regression test added).

## Gates — never self-approve

- Claude never writes `accepted`. Ask, then wait.
- No commit unless the user says "commit"; then report the SHA.

## When a stage finishes

1. Update `.claude/development-progress.md`.
2. Lessons from review/incidents → `.claude/lessons-learned.md`; a durable rule →
   propose a CLAUDE.md change (reviewed like code).
3. Decision or new table applied → flag the CLAUDE.md §3/§12 update + `docs/data-map.md`.
4. Spec fully done → move to `.kiro/specs/done/`.
5. After code changes: `graphify update .`

## Don't

- `/ecc:prp-prd`, `/ecc:plan-prd` — their PRD duplicates requirements.md.
- `/ecc:feature-dev` — runs end-to-end and skips the human gates.
- Migrate legacy `.claude/sdlc/*` features to Kiro.
