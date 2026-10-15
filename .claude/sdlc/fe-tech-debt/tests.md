# Tests: Perbaikan tech debt frontend (FE-01 – FE-12)

Status: **partial — Fase A slice A1, A2, A4 green 2026-10-15** (Stage 4). Input: `spec.md` (FR-A1, FR-A3), `plan.md` (A1, A2, A4). Belum di-commit. Slice lain (A3, A5, B–F) belum dikerjakan.

## Commands (run 2026-10-15)
| Command | Result |
|---|---|
| `grep -rn "<style" CompanyPortal-Vite/src VendorPortal-Vite/src` | kosong di `.tsx` (A1; satu-satunya hit = `VP/src/assets/vite.svg`, SVG tidak tunduk CSP halaman) |
| `pnpm --dir frontend/CompanyPortal-Vite run build` / `lint` / `test` | built / clean / 126 files, 1054 tests (sesudah A1+A2 dan sesudah A4) |
| `pnpm --dir frontend/VendorPortal-Vite run build` / `lint` / `test` | built / clean / 19 files, 136 tests |
| `dist/index.html` CompanyPortal | satu `<script type="module" src=…>` eksternal, tanpa script/style inline |
| `docker build` kedua portal → `docker run` → `nginx -t` | sukses / `test is successful` |
| `curl -sI` ke `/`, `/login`, aset hashed `/assets/index-*.js`, `/api/v1/health` (502, backend tidak jalan) di kedua image | kelima header (XFO, XCTO, Referrer-Policy, HSTS, CSP per portal) ada di **setiap** respons, termasuk 502 (`always`) |
| `pnpm update @tanstack/react-router` (CP) | 1.170.18 → 1.170.44, range `^1.170.44`; `pnpm why seroval` → 1.6.9 |
| `pnpm audit --prod` (CP) | seroval hilang; sisa 2 high = `xlsx` (FE-02, slice A3, belum dikerjakan) |

## Test → requirement
| Test | Covers |
|---|---|
| T-A1 (lokal: docker build + `nginx -t` + `curl -I`) | FR-A1 header di `/`, route SPA, aset hashed, `/api/`; CSP CompanyPortal dan VendorPortal (Google Fonts, `blob:`) persis spec |
| Test EOD yang ada (`features/eod-monitoring`) tetap hijau | A1: keyframes pindah ke `styles/index.css`, nama animasi tidak berubah |
| `pnpm audit --prod` + `pnpm why seroval` | FR-A3 seroval ≥ 1.6.3 |
| Build + test + lint CompanyPortal sesudah bump | FR-A3 tanpa regresi router |

## Outstanding
- **Manual browser check (user, Golden Rule #10)**: lewat image Docker kedua portal, buka DevTools → Network/Console: header terkirim, **tidak ada pelanggaran CSP** di console (login, dashboard, EOD Monitoring dialog + drawer retry — animasi tetap jalan, VendorPortal font Plus Jakarta Sans/IBM Plex Mono termuat, preview upload gambar `blob:`), navigasi router normal sesudah bump.
- T-A1 sebagai job CI `frontend-image` menunggu E1.
- T-A3 (`pnpm audit --prod --audit-level high` = 0) menunggu A3 (hapus `xlsx`).
