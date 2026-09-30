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

Last synced: 2026-09-30. "Stage" = furthest artifact present. "Status" = actual code state (commits / CLAUDE.md Sec 12), which may be ahead of the artifact headers.

| Feature | Folder | Stage | Status |
|---------|--------|-------|--------|
| Vendor upload DSR | `vendor-upload-dsr/` | 6 Maintain (all 6 artifacts, headers still `draft`) | implemented (commit `4d4e261`); `intent.md` still awaiting PO sign-off |
| Master data (vendor, vault, PIC, ATM, kelolaan) | `master-data/` | 5 Deploy (`review.md`) | implemented, Fase 0–6 (plan 43/43); retry-apply has no UI button yet |
| User session lifetime (1 h, parameterized) | `user-session/` | 4 Test (`tests.md`) | implemented (plan 17/17, commit `7a86eca`); T5.3 manual expiry test outstanding (user) |
| Vendor package pricing | `vendor-pricing/` | 3 Build (`plan.md`, `schema.dbml`) | schema (mig 009/010/017) + backend + CompanyPortal panel built; per-ATM special price not seeded |
| Vendor branch tipe / Branch menu | `vendor-branch-tipe/` | 3 Build (`plan.md`) | item 1 implemented 2026-09-23 (plan 18/18) |
| Kelolaan cabang → vault/PIC/paket (perbaikan RBAC) | `perbaikan-rbac/` | 3 Build (`plan.md`) | mostly built (backend contract, dialogs, branch drill-down, vendor-wide PIC, pending badges); 2026-09-30 G1 server-side paging + branch search, G2 pending creates visible, G3 tests; next: approve/reject/apply-failure + conflict tests, manual browser check (user) |
| `import_jobs`/`export_jobs` + idempotensi (Phase 0.2) | `import-export-jobs/` | 4 Test (`tests.md`, `review.md`) | built Fases 1–7; T8.1 tests green, T8.2 reviews fixed (migration 020, DSR path traversal); Go helper deferred to forecast 2.1; TZ Asia/Jakarta deploy prereq; remaining: A17 manual browser check (user), `/api/eod` routing ready (S6) |
| Laporan DSR telat / tidak kirim (Phase 2.3) | `dsr-late-report/` | 1 Plan (`intent.md`, draft 2026-09-28) | not built; depends on 0.3 notification + DSR schema change (per-cabang); open questions resolved 2026-09-28, awaiting PO acceptance |
| Forecast Browser ringkasan vendor × region | `forecast-browser-summary/` | 5 Deploy (`review.md`, 2026-09-30) | committed `7f6d16b`; review R1 (branch without region, opsi A) + R2 fixed after commit (uncommitted); tests/lint/build green; manual browser check outstanding (user) |

None of these went through the full chain in order (legacy, pre-Sec 4a). Only `vendor-upload-dsr/` has `intent.md`/`spec.md`. New features start at `intent.md`.
