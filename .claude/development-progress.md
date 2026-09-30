# Development Progress — CMS2 Cash Management System

**Last Updated:** 2026-09-30

Branch: `dev1` (latest: `531d9ef` perbaikan RBAC G2, `332b340` child-panel paging). PR dev1 → main not opened yet.

## Overview

| Feature | SDLC folder | Stage | Status | Outstanding |
|---------|-------------|-------|--------|-------------|
| Vendor upload DSR | `vendor-upload-dsr/` | 6 Maintain | ✅ implemented (`4d4e261`) | `intent.md` PO sign-off (headers still `draft`) |
| Master data (vendor, vault, PIC, ATM, kelolaan) | `master-data/` | 5 Deploy | ✅ implemented, Fase 0–6 (43/43) | Retry-apply has no UI button |
| User session lifetime (1 h, parameterized) | `user-session/` | 4 Test | ✅ implemented (17/17, `7a86eca`) | T5.3 manual expiry test (user) |
| Vendor branch tipe / Branch menu | `vendor-branch-tipe/` | 3 Build | ✅ item 1 implemented 2026-09-23 (18/18) | — |
| Vendor package pricing | `vendor-pricing/` | 3 Build | 🟡 schema (mig 009/010/017) + backend + CompanyPortal panel built | Per-ATM special price not seeded |
| Perbaikan RBAC (kelolaan cabang → vault/PIC/paket) | `perbaikan-rbac/` | 3 Build | 🟡 mostly built — backend contract, 4 dialogs, branch drill-down, vendor-wide PIC, pending badges; 2026-09-30 server-side paging + branch search (G1), pending creates visible (G2), tests (G3) | Automated tests for approve/reject/apply-failure, double-submit/cross-tab conflict, per-entity CRUD payloads; manual browser check (user) |
| `import_jobs`/`export_jobs` + idempotensi (Phase 0.2) | `import-export-jobs/` | 4 Test | ✅ built Fases 1–7 (Python + frontend); T8.1 tests green, T8.2 reviews fixed (migration 020, DSR path traversal); Go helper deferred to 2.1 | A17 manual browser check of EOD monitoring (user) — `/api/eod` routing ready (S6) |
| Laporan DSR telat / tidak kirim (Phase 2.3) | `dsr-late-report/` | 1 Plan (intent draft 2026-09-28) | ⏳ not built | PO acceptance; depends on 0.3 notification + DSR per-cabang schema change |
| Forecast Browser ringkasan vendor × region | `forecast-browser-summary/` | 5 Deploy (review 2026-09-30) | ✅ built (`7f6d16b`) + R1 fix uncommitted | manual browser check (user) → commit R1 → merge |

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

1. User: A17 manual check of EOD monitoring (closes Phase 0.2) + manual check of perbaikan-rbac (ROH paging, pending creates).
2. Finish `perbaikan-rbac` acceptance: approve/reject/apply-failure states, double-submit + cross-tab conflict (check backend rejects duplicates first), per-entity CRUD tests.
3. PR dev1 → main.
4. PO acceptance of `dsr-late-report/intent.md` → spec.
5. Session follow-ups (separate features): idle timeout (warn T-5m), session audit events, admin force-logout.
