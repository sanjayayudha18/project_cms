# Development Progress — CMS2 Cash Management System

**Last Updated:** 2026-09-29 (synced with `.claude/sdlc/README.md` → "Current features", 2026-09-28)

Branch: `dev1`. No code commits since 2026-09-25 (docs/CLAUDE.md/graphify only).

## Overview

| Feature | SDLC folder | Stage | Status | Outstanding |
|---------|-------------|-------|--------|-------------|
| Vendor upload DSR | `vendor-upload-dsr/` | 6 Maintain | ✅ implemented (`4d4e261`) | `intent.md` PO sign-off (headers still `draft`) |
| Master data (vendor, vault, PIC, ATM, kelolaan) | `master-data/` | 5 Deploy | ✅ implemented, Fase 0–6 (43/43) | Retry-apply has no UI button |
| User session lifetime (1 h, parameterized) | `user-session/` | 4 Test | ✅ implemented (17/17, `7a86eca`) | T5.3 manual expiry test (user) |
| Vendor branch tipe / Branch menu | `vendor-branch-tipe/` | 3 Build | ✅ item 1 implemented 2026-09-23 (18/18) | — |
| Vendor package pricing | `vendor-pricing/` | 3 Build | 🟡 schema (mig 009/010/017) + backend + CompanyPortal panel built | Per-ATM special price not seeded |
| Perbaikan RBAC (kelolaan cabang → vault/PIC/paket) | `perbaikan-rbac/` | 3 Build | 🟡 in progress (9/23) | Backend contract check before frontend |
| `import_jobs`/`export_jobs` + idempotensi (Phase 0.2) | `import-export-jobs/` | 3 Build (plan draft 2026-09-28) | ✅ built Fases 1–7 (Python + frontend); T8 verification/review pending; Go helper deferred to 2.1 | T8.1–T8.2 tests.md + reviews; A17 manual browser check (user) needs `/api/eod` routing fix |
| Laporan DSR telat / tidak kirim (Phase 2.3) | `dsr-late-report/` | 1 Plan (intent draft 2026-09-28) | ⏳ not built | PO acceptance; depends on 0.3 notification + DSR per-cabang schema change |

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

1. Accept `import-export-jobs/plan.md` → build Phase 0.2.
2. Finish `perbaikan-rbac` (14 tasks left).
3. PO acceptance of `dsr-late-report/intent.md` → spec.
4. Session follow-ups (separate features): idle timeout (warn T-5m), session audit events, admin force-logout.
