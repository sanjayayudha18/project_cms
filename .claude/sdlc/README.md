# AI-Native SDLC

This folder holds one sub-folder per feature. Each feature moves through six
stages, and every stage commits one machine-readable artifact that the next
stage reads. The chain of committed files is the audit trail: who asked for
what, what the agent produced, and who approved it.

Reference: https://claude.com/blog/the-ai-native-sdlc-playbook

## Layout

```
.claude/sdlc/
├── README.md              <- you are here (how the workflow works)
└── <feature-name>/        <- one folder per feature (e.g. vendor-upload-dsr/)
    ├── intent.md          Stage 1 Plan
    ├── spec.md            Stage 2 Design
    ├── plan.md            Stage 3 Build
    ├── tests.md           Stage 4 Test
    ├── review.md          Stage 5 Deploy
    └── incidents.md       Stage 6 Maintain
```

## Artifact chain (fill in order; commit each before starting the next)

| Stage | Artifact | Who drives | Trigger to next stage |
|-------|----------|------------|-----------------------|
| 1 Plan | `intent.md` | Product owner + Claude | product owner accepts intent |
| 2 Design | `spec.md` | Product owner + Claude (skills applied) | product owner accepts spec |
| 3 Build | `plan.md` | Engineer + Claude Code plan mode | engineer accepts plan |
| 4 Test | `tests.md` | Engineer (feedback loop) | build + tests green |
| 5 Deploy | `review.md` | Reviewer agent + code owner | PR merged past gates |
| 6 Maintain | `incidents.md` | Monitoring / on-call | control-band breach writes a new `intent.md` |

The loop closes: a breach, ticket, or alert in Stage 6 writes a new `intent.md`
and the cycle restarts.

## How to start a new feature

1. Create a folder: `.claude/sdlc/<feature-name>/`.
2. Copy the six artifact files from an existing feature (e.g. `vendor-upload-dsr/`)
   as templates, or generate them with Claude from the templates below.
3. Fill `intent.md` first, in the originator's own words. Leave the rest as stubs.
4. Advance one stage at a time. Do not start a stage until the previous
   artifact is accepted and committed.

## Rules that apply to every feature

Pulled from `.claude/CLAUDE.md` (the project source of truth). Restate the
feature-specific slice of these in each `intent.md` / `spec.md`:

- Stack is fixed. Follow the module + table map. No invented endpoints/tables/env.
- Plan before non-trivial work; small diffs, one concern.
- Money as numeric / integer minor units, never float. Timestamps timestamptz (UTC).
- Writes on primary, reads/reporting on replica.
- State changes on financial/master data go through maker-checker + write `audit_logs`.
- File ingests idempotent per checksum.
- STOP and flag on: auth, money/journal, reconciliation, data migrations, deletes.
- Every feature ships with tests; coverage >= 80% on `internal/*`.

## Artifact templates (what each file is for)

- `intent.md` — the problem in the originator's words: problem, proposed outcome,
  affected users/systems, constraints, open questions.
- `spec.md` — requirements (functional + non-functional), API surface, data model,
  out-of-scope, and flagged concerns to resolve with policy owners before build.
- `plan.md` — files that change, order of work, risks, proof (maps to tests),
  alternatives not taken. Produced in Claude Code plan mode; kept in sync with the diff.
- `tests.md` — build/test/lint commands, quantifiable targets, test cases traced
  back to spec requirements.
- `review.md` — review passes (bugs / security / compliance), Important-vs-nit,
  the Definition of Done gate, and approval gates.
- `incidents.md` — monitored signals, response tiers, incident log, and evals
  added from incidents.

## Current features

| Feature | Folder | Status |
|---------|--------|--------|
| Vendor upload DSR | `vendor-upload-dsr/` | intent drafted |
