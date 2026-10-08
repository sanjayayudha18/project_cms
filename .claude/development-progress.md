# Development Progress — CMS2 Cash Management System

**Last Updated:** 2026-10-08

Branch: `dev1` = `main` (PR #1 dev1 → main merged, `38eb0a1`; latest `87b2d78`). Uncommitted: Phase 0.1 + intents 0.3/0.4 (2026-10-07).

## Overview

| Feature | SDLC folder | Stage | Status | Outstanding |
|---------|-------------|-------|--------|-------------|
| Vendor upload DSR | `vendor-upload-dsr/` | 6 Maintain | ✅ implemented (`4d4e261`); `tests.md` re-verified vs two-phase flow 2026-10-07 (Phase 1.3): T1/T3/T5/T7 pass, T4 removed, T6 obsolete | T2 live-DB integration test; manual browser check (user); `intent.md` PO sign-off (headers still `draft`) |
| Master data (vendor, vault, PIC, ATM, kelolaan) | `master-data/` | 5 Deploy | ✅ implemented, Fase 0–6 (43/43) | Retry-apply has no UI button |
| User session lifetime (1 h, parameterized) | `user-session/` | 4 Test | ✅ implemented (17/17, `7a86eca`) | T5.3 manual expiry test (user) |
| Vendor branch tipe / Branch menu | `vendor-branch-tipe/` | 3 Build | ✅ item 1 implemented 2026-09-23 (18/18) | — |
| Vendor package pricing | `vendor-pricing/` | 3 Build | 🟡 schema (mig 009/010/017) + backend + CompanyPortal panel built | Per-ATM special price not seeded |
| Perbaikan RBAC (kelolaan cabang → vault/PIC/paket) | `perbaikan-rbac/` | 3 Build | 🟡 mostly built — backend contract, 4 dialogs, branch drill-down, vendor-wide PIC, pending badges; 2026-09-30 server-side paging + branch search (G1), pending creates visible (G2), tests (G3) | Automated tests for approve/reject/apply-failure, double-submit/cross-tab conflict, per-entity CRUD payloads; manual browser check (user) |
| `import_jobs`/`export_jobs` + idempotensi (Phase 0.2) | `import-export-jobs/` | 4 Test | ✅ built Fases 1–7 (Python + frontend); T8.1 tests green, T8.2 reviews fixed (migration 020, DSR path traversal); Go helper deferred to 2.1 | A17 manual browser check of EOD monitoring (user) — `/api/eod` routing ready (S6) |
| Laporan DSR telat / tidak kirim (Phase 2.3) | `dsr-late-report/` | 1 Plan (intent draft 2026-09-28) | ⏳ not built | PO acceptance; depends on 0.3 notification + DSR per-cabang schema change |
| Forecast Browser ringkasan vendor × region | `forecast-browser-summary/` | 5 Deploy (review 2026-09-30) | ✅ built, merged to main (`7f6d16b`, R1 `a9873d2`) | manual browser check (user) |
| Kelolaan ATM: paket khusus cabang / vendor-wide | `atm-package-source/` | 5 Deploy (review 2026-10-07) | ✅ built, merged to main (`ff005fa` + fixes `321a10b`, `00b05a1`; mig 023 applied to dev); tests green; review R1 fixed | manual browser check (user) |
| Kuota kunjungan replenish per ATM | `atm-visit-quota/` | 5 Deploy (review 2026-10-01) | ✅ built, merged to main (committed in `ff005fa`; mig 021 applied to dev); review R1 fixed | manual browser check (user, seed kelolaan dulu) |
| Nomor tiket replenish per ATM | `replenish-ticket/` | 5 Deploy (review 2026-10-08) | ✅ built (uncommitted); mig 026 applied to dev; review R1/R2 fixed | manual browser check (user); isi `region_code` cabang yang masih kosong; commit → code owner merge |
| Vendor PKS / CIS limit | `vendor-pks-cis-limit/` | 1 Plan (intent draft 2026-10-05) | ⏳ not built; columns approved 2026-10-05 (CLAUDE.md Sec 3), migration `022` not created | PO accept intent → spec → migration 022 |
| Read-replica routing (Phase 0.1) | — (`bugfixes.md` 2026-10-07) | done | ✅ Go `main.go` + vendor/ATM admin repos split; Python `db_read_pool` for monitoring APIs; tests green | manual smoke of list pages + EOD monitoring (user) |
| Notifikasi in-app + SMTP (Phase 0.3) | `notification/` | 5 Deploy (review 2026-10-07) | ✅ built (`eb457cf`, review R1–R3 fixed): `internal/notification`, `/api/v1/notifications`, outbox worker (stdlib SMTP), CompanyPortal bell, VendorPortal API; over-quota = first consumer; mig 025 applied to dev | review; manual browser check (user); real SMTP send |
| Penyimpanan dokumen (Phase 0.4) | `document/` | 1 Plan (intent accepted) | ⏸ deferred to Phase 3/5 (PO 2026-10-07) | spec when first consumer starts |

Manual browser verification is always the user's job (Golden Rule #10) and stays "outstanding" until the user confirms it.

---

## Feature notes

### User Session Lifetime (Absolute 1 Hour, Parameterized)
- Sessions capped at `SESSION_MAX_LIFETIME` (default 1h, min 5m) after login, regardless of activity; refresh rotation inherits the original deadline.
- On expiry both portals redirect to login with a notice.
- Files: `pkg/config`, `pkg/auth/token_service`, `backend/internal/handler/auth_handler`, CompanyPortal/VendorPortal auth stores. Coverage `pkg/auth` 83.9%.
- Tests: `.claude/sdlc/user-session/tests.md`.

---

## Next Phase

1. User: manual browser checks — A17 EOD monitoring (closes Phase 0.2), perbaikan-rbac (ROH paging, pending creates), Forecast Browser, kelolaan ATM paket source, kuota kunjungan.
2. Finish `perbaikan-rbac` acceptance: approve/reject/apply-failure states, double-submit + cross-tab conflict (check backend rejects duplicates first), per-entity CRUD tests.
3. `notification/` (0.3): commit review fixes R1–R3 → code owner merge; user: browser check of bell + VendorPortal page; real SMTP send once relay creds exist. 0.4 + 0.5 deferred.
4. PO acceptance of `dsr-late-report/intent.md` → spec.
5. `vendor-pks-cis-limit`: spec → migration 022.
6. Session follow-ups (separate features): idle timeout (warn T-5m), session audit events, admin force-logout.
