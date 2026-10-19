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

Last synced: 2026-10-07 (`dev1` = `main`, PR #1 merged `38eb0a1`). "Stage" = furthest artifact present. "Status" = actual code state (commits / CLAUDE.md Sec 12), which may be ahead of the artifact headers.

| Feature | Folder | Stage | Status |
|---------|--------|-------|--------|
| Vendor upload DSR | `vendor-upload-dsr/` | 6 Maintain (all 6 artifacts, headers still `draft`) | implemented (commit `4d4e261`); `tests.md` re-verified 2026-10-07 (T1/T3/T5/T7 pass, T4 removed, T6 obsolete; T2 live-DB test outstanding); `intent.md` still awaiting PO sign-off |
| Master data (vendor, vault, PIC, ATM, kelolaan) | `master-data/` | 5 Deploy (`review.md`) | implemented, Fase 0–6 (plan 43/43); retry-apply has no UI button yet |
| User session lifetime (1 h, parameterized) | `user-session/` | 4 Test (`tests.md`) | implemented (plan 17/17, commit `7a86eca`); T5.3 manual expiry test outstanding (user) |
| Vendor package pricing | `vendor-pricing/` | 3 Build (`plan.md`, `schema.dbml`) | schema (mig 009/010/017) + backend + CompanyPortal panel built; per-ATM special price not seeded |
| Vendor branch tipe / Branch menu | `vendor-branch-tipe/` | 3 Build (`plan.md`) | item 1 implemented 2026-09-23 (plan 18/18) |
| Kelolaan cabang → vault/PIC/paket (perbaikan RBAC) | `perbaikan-rbac/` | 3 Build (`plan.md`) | mostly built (backend contract, dialogs, branch drill-down, vendor-wide PIC, pending badges); 2026-09-30 G1 server-side paging + branch search, G2 pending creates visible, G3 tests; next: approve/reject/apply-failure + conflict tests, manual browser check (user) |
| `import_jobs`/`export_jobs` + idempotensi (Phase 0.2) | `import-export-jobs/` | 4 Test (`tests.md`, `review.md`) | built Fases 1–7; T8.1 tests green, T8.2 reviews fixed (migration 020, DSR path traversal); Go helper deferred to the first Go file ingest (2.1 forecast engine dropped 2026-10-08); TZ Asia/Jakarta deploy prereq; remaining: A17 manual browser check (user), `/api/eod` routing ready (S6) |
| Penetapan branch vault per ATM oleh ACM (Phase 2.2a) | `cit-acm-plan/` | 5 Deploy (`review.md`, 2026-10-08) | ✅ built S1–S8 (mig 027 + comment 028 applied to dev); review R1 (branch pindah area) + R2 (link notifikasi) fixed, no open Important; tests green; not committed | manual browser check (user); commit; then 2.2b |
| Penetapan vault ACM v3: bagi nominal request ke vault (CASH/ATM_CASH) + tiket pemenuhan vault + area per region | `cit-vault-select/` | 5 Deploy (`review.md`, 2026-10-13) | ✅ built S0–S8 (mig `030_vault_allocation.sql` applied to dev); Go unit + integration (0 skip, service 81.3 %) + CompanyPortal 1080 + VendorPortal 128 green, lint/build clean; review R2 fixed, **R1 open** (dev seed maps branches, not regions); not committed; next: manual browser check (user, map regions first), commit |
| Kirim Vendor Request ke vendor + terima/tolak (Phase 2.2b) | `cit-send-vendor/` | 5 Deploy (`review.md`, 2026-10-09) | built S1–S9 (mig 029 applied to dev), uncommitted; review R1 fixed, no open Important (10 nits); next: manual browser check (user), commit → code owner merge |
| Uang di API sebagai string desimal (QC F7) | `money-decimal-string/` | 4 Test (`tests.md` green 2026-10-12) | built; manual check outstanding (user) |
| IP client tepercaya / anti-spoof `X-Forwarded-For` (QC F13) | `client-ip-trust/` | 4 Test (`tests.md` green 2026-10-12) | built; manual check outstanding (user) |
| Replenishment realisasi vs order (Phase 2.2c) | `replenishment-realisasi/` | 5 Deploy (`review.md`, 2026-10-14) | built S1–S9 (mig 031 applied to dev); review R1 (scope lintas vendor), R2 (ITM pindah tiket), R3 (void request ditolak, keputusan PO), R4 (laporan vendor setelah final), R5 (CSV injection) fixed, no open Important (6 nits); Go integration 0 skip, Python 77, CompanyPortal 1054, VendorPortal 136 green; not committed; next: manual browser check + uji ITM (user), commit → code owner merge |
| Laporan DSR telat / tidak kirim (Phase 2.3) | `dsr-late-report/` | 1 Plan (`intent.md`, draft 2026-09-28) | not built; depends on 0.3 notification + DSR schema change (per-cabang); open questions resolved 2026-09-28, awaiting PO acceptance |
| Forecast Browser ringkasan vendor × region | `forecast-browser-summary/` | 5 Deploy (`review.md`, 2026-09-30) | committed `7f6d16b` + review R1/R2 fix `a9873d2`; tests/lint/build green; no open review findings; merged to main; manual browser check outstanding (user) |
| Kuota kunjungan replenish per ATM | `atm-visit-quota/` | 5 Deploy (`review.md`, 2026-10-01) | built, committed in `ff005fa`, merged to main; migrasi 021 applied to dev; tests/lint/build green; review R1 fixed, no open Important (6 nits noted); next: manual browser check (user, needs kelolaan seed) |
| Kelolaan ATM: paket khusus cabang atau vendor-wide | `atm-package-source/` | 5 Deploy (`review.md`, 2026-10-07) | built, committed `ff005fa` (satu commit bersama fitur lain, tidak dipisah per fitur; mig 023 applied to dev); backend unit+integration, frontend tests/lint/build green; review R1 (I1 cakupan, I2 mode-check Applier) fixed, no open Important; fixes `321a10b`, `00b05a1`; merged to main; next: manual browser check (user) |
| PKS vendor + Limit CIS per area vault (FSD v1.0) | `vendor-pks-cis-limit/` | 1 Plan (`intent.md`, draft 2026-10-05) | not built; schema A1–A6 + Q1–Q8 resolved 2026-10-05 (migrasi 022); no open questions; awaiting PO accept → spec |
| Notifikasi in-app + SMTP (Phase 0.3) | `notification/` | 5 Deploy (`review.md`, 2026-10-07) | ✅ built (`eb457cf`; mig 025 applied to dev); review R1–R3 fixed (uncommitted); coverage 83.9 % | code owner merge; manual browser check (user); real SMTP send (needs relay creds) |
| Nomor tiket replenish per ATM | `replenish-ticket/` | 5 Deploy (`review.md` 2026-10-08) | built (uncommitted): migrasi 026 applied to dev (174 tiket backfill, 356 cabang ber-kode), backend + CompanyPortal; tests green; review R1/R2 fixed, no open Important (6 nits); next: manual browser check (user), isi `region_code` 51 cabang kosong, commit → code owner merge |
| Perbaikan tech debt frontend (FE-01 – FE-12, `FE_debt.md`) | `fe-tech-debt/` | 4 Test (partial: `tests.md` green 2026-10-15 for A1, A2, A4) | FE-01 (A1+A2) + FE-03 (A4) committed `fb3b6fd`; manual browser CSP check outstanding; Fase F built 2026-10-19, committed `b007b64` (F1 FE-10 dead code removed, F2 FE-12 `VendorRequestDetail.tsx` split); manual browser check outstanding (user); A3, A5, B–E on hold until user says go |
| Coverage FE naik ke 80 % (FE-13) | `fe-coverage/` | 4 Test (`tests.md` green 2026-10-19; C0–C4 `1aadb47`, C5 `d195c38`) | target reached: CP 84.05/80.08/80.54/85.28 (threshold 84/80/80/85), VP 93.12/84.12/90.71/93.97 (threshold 93/84/90/93); gate `pnpm run test:coverage`; double `/api/v1` prefix in EOD Monitoring + vendorSide fixed 2026-10-19 (bugfixes.md, uncommitted); open findings: replenishment report raw fetch, VaultsPanel capacity without currency; next `review.md` |
| Pola auth & util antar portal (FE-09) | `fe-auth-unify/` | 4 Test (`tests.md` green 2026-10-15) | U1–U3 committed `1aadb47`: VP init `/refresh` single-flight + `applyToken`, CP `formatRp`/`formatThousands`/`formatDecimalIDR`; RED→GREEN recorded; manual login/logout/refresh check outstanding (user); next `review.md` |
| Perbaikan tech debt backend (BE-01 – BE-12, `BE_debt.md`) | `be-tech-debt/` | 4 Test (partial: Fase A plan accepted + `tests.md` green 2026-10-15) | BE-06 (A1 Alpine 3.24) + BE-12 (A2 x/crypto v0.56.0, A3 `dmaa/bak` removed) committed `fb3b6fd` + `9a5af6d`; Fase B (CI: `ci.yml` jobs go/go-integration/sqlc/python + deployment.md) accepted + green locally 2026-10-15, committed `8b3a32b` + `077ce05`, B5 e2e deferred; fase C–G plan accepted 2026-10-15; C1 coverage gate committed `ddefb12`; Fase G built 2026-10-19, committed `b007b64` (G1 BE-07 split service files, G2 BE-11 `backend-cit` out of deploy docs, G3 BE-10 OpenAPI deferred, docs only) |
| CI/CD pindah ke Jenkins (gantikan GitHub Actions + Cloud Build) | `ci-jenkins/` | 2 Design (`spec.md`, draft 2026-10-15) | not built; intent accepted 2026-10-15 (scope CI + deploy, start ~Dec 2026); `ci.yml` stays live until Jenkins green; spec open: O1–O5 + S1–S7 to confirm; **on hold** (user 2026-10-15) — resume: decide S1–S7, O1–O5, accept spec → plan |
| Kontrak OpenAPI BE ↔ FE (BE-10) | `openapi-contract/` | 4 Test (slice 1–3 done, `tests.md` green 2026-10-19) | slice 1 committed `d1e7082`: `backend/api/openapi.yaml` (26 ops) + Go contract test (kin-openapi); slice 2 built 2026-10-19 (uncommitted): openapi-typescript 7.13.0 → CP `src/lib/api/schema.d.ts` + freshness test T5, vendor-request wire types aliased (T6), spec gained status/event enums; build/test/lint green; slice 3 done 2026-10-19 (CLAUDE.md Sec 2/2a/11 DoD, uncommitted); next: commit slice 2+3 (user), then 5 Deploy `review.md`. Plan accepted 2026-10-19; spec accepted 2026-10-19 (S1–S7); intent accepted 2026-10-19 (pilot = `/vendor-requests` + `/vendor/replenish-orders`, YAML manual `backend/api/openapi.yaml`, kin-openapi contract test, openapi-typescript for CompanyPortal, repo only, DoD rule, backend-exq later); spec S1–S7 awaiting PO |
| Penyimpanan dokumen (Phase 0.4) | `document/` | 1 Plan (`intent.md` accepted 2026-10-07) — **deferred** to Phase 3/5 | not built; Q1–Q8 answered | spec when invoice / cash count starts |

None of these went through the full chain in order (legacy, pre-Sec 4a). Only `vendor-upload-dsr/` has `intent.md`/`spec.md`. New features start at `intent.md`.
