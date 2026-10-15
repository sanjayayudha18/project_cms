# Tests: Perbaikan tech debt frontend (FE-01 – FE-12)

Status: **partial — Fase A slice A1, A2, A4 green 2026-10-15** (Stage 4). Input: `spec.md` (FR-A1, FR-A3), `plan.md` (A1, A2, A4). Committed `fb3b6fd`. Slice lain (A3, A5, B–F) belum dikerjakan.

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
- T-A3 lokal sudah 0 (lihat slice A3); sebagai job CI menunggu E1.

## Slice A3 — FE-02 hapus Unggah DSR + `xlsx` (2026-10-15, belum commit)
| Command | Result |
|---|---|
| grep pemakai `features/forecasting` / `xlsx` / `dsr-upload` di `CP/src` (sebelum hapus) | hanya `routes/forecasting/dsr-upload.tsx`, registrasinya di `main.tsx`, menu yang di-comment di `navigation.ts`, dan **kartu "Unggah DSR" di `routes/forecasting/index.tsx`** (tidak disebut di plan; ikut dihapus agar tidak jadi link mati). `xlsx` di `RetryDrawer.test.tsx` hanya string nama file. |
| `pnpm remove xlsx` → `pnpm why xlsx` | kosong |
| T-A2: `grep -r "forecasting/dsr-upload" dist` + cari `xlsx`/SheetJS di `dist/assets/*.js` sesudah build | tidak ada hit (route tidak terdaftar). Belum ada test unit route tree: `main.tsx` langsung me-mount app saat di-import. |
| `pnpm audit --prod` (CP) | `No known vulnerabilities found` |
| `pnpm --dir frontend/CompanyPortal-Vite run build` / `lint` / `test` | built / clean (465 files) / 123 files, 1001 tests (2 file test DSR ikut terhapus) |

- Dihapus: `routes/forecasting/dsr-upload.tsx`, seluruh `features/forecasting/` (`DSRUploadForm`, `DSRPreviewTable`, `UploadHistory`, `useDSRHistory`, `dsr-parser`, `dsr-validation` + 2 test), import + registrasi `dsrUploadRoute` di `main.tsx`, menu `dsr-upload` yang di-comment, kartu "Unggah DSR" di halaman `/forecasting`, dependency `xlsx`. `navigation.test.ts` memakai id `dsr-upload` sebagai fixture saja, tidak diubah.

### Outstanding
- **Manual browser check (user, Golden Rule #10)**: halaman `/forecasting` (Peramalan) tampil tanpa kartu Unggah DSR, `/forecasting/dsr-upload` jatuh ke not-found, menu sidebar Peramalan normal.

## Batch FE-07 / FE-08 / FE-10 / FE-11 (2026-10-15, belum commit)
| Command | Result |
|---|---|
| `pnpm --dir frontend/CompanyPortal-Vite run build` / `lint` / `test` | built / clean (476 files) / 125 files, 1043 tests |

- FE-07: `main.tsx` `createRouter({ routeTree, defaultErrorComponent })` memakai `DefaultErrorFallback` (kini diekspor dari `components/feedback`). Tidak ada unit test render-error router; diverifikasi lewat build/typecheck.
- FE-08: `lib/api/client.ts` mendukung `FormData` (tanpa `Content-Type` JSON) dan `responseType: "blob"`, `ApiResponse.headers`; `features/master-data-io/api.ts` memakai `api.get/post` sehingga ikut refresh 401. Test baru di `client.test.ts` (FormData tanpa Content-Type, blob + headers, 401 refresh + retry FormData). `admin-holidays/api.ts:74` masih `fetch` sendiri dengan pola sama — di luar batch, kandidat berikutnya.
- FE-10: hapus 4 `*.backup`, `lib/api/stubs/` (+ bagian Property 17 di `api.property.test.ts`), `apiConfig.mode/stubLatency`/`ApiMode` (`.env.local` `VITE_API_MODE=real` gitignored, tidak lagi dibaca). **Folder `forecast` / `forecasting` / `dmaa-forecast` tidak disentuh**: ketiganya masih dipakai route aktif (`routes/forecasting/*`; `dsr-upload` memakai `features/forecasting/components`, `dsr-dashboard` memakai `features/dsr`) dan menu Sidebar. Konsolidasi butuh keputusan produk (route `/forecasting/forecast` & `/forecasting/dmaa-forecast` masih dipakai atau tidak).
- FE-11: `.env` + `.env.*` ditambah ke `CompanyPortal-Vite/.dockerignore`.

### Outstanding
- **Manual browser check (user, Golden Rule #10)**: Master Data I/O (export/template download, dry-run + confirm import, 401 → refresh tidak logout), dan tampilan fallback saat render error route.
