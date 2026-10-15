# Frontend Tech Debt

Hasil review arsitektur FE (CompanyPortal-Vite + VendorPortal-Vite) terhadap standar aplikasi modern, 2026-10-12. Satu baris per item; ubah **Status** saat dikerjakan (`open` → `in-progress` → `done <sha>`). Sebelum mengerjakan item, tanya dulu: AI-DLC baru atau cukup `bugfixes.md` (CLAUDE.md Sec 4a rule 0).

Pasangan: [BE_debt.md](./BE_debt.md). AI-DLC perbaikan: [`sdlc/fe-tech-debt/`](./sdlc/fe-tech-debt/intent.md).

## Baseline saat review

| | CompanyPortal | VendorPortal |
|---|---|---|
| `tsc -b` | bersih | bersih |
| `biome check` | 480 file, 0 issue | 97 file, 0 issue |
| `vitest run` | 127 file, 1078 test lulus | 18 file, 127 test lulus |
| `pnpm audit --prod` | 3 high (xlsx ×2, seroval) | 0 |
| Bundle JS | 1,79 MB, 1 chunk | 565 KB, 1 chunk |

Sudah baik (jangan dirombak): React 19 + Vite 6 + TS strict (`noUncheckedIndexedAccess`), TanStack Router/Query/Table, Zustand, RHF + Zod, Tailwind v4, Biome; struktur `features/<domain>/`; access token di memori + refresh via cookie httpOnly (single-flight); RBAC di route; Docker multi-stage + Nginx non-root.

## HIGH

| ID | Item | Bukti | Perbaikan | Status |
|---|---|---|---|---|
| FE-01 | Security header Nginx tidak terkirim | `frontend/CompanyPortal-Vite/nginx.conf`: `add_header` di level `server` tidak diwarisi oleh `location /` dan `location ~* \.(js\|css…)` yang punya `add_header` sendiri. `frontend/VendorPortal-Vite/nginx.conf` tanpa security header sama sekali. Tidak ada CSP/HSTS. | Pindahkan header ke setiap `location` (atau `include` snippet bersama), tambah CSP + HSTS; berlaku di kedua portal. | done fb3b6fd |
| FE-02 | `xlsx@0.18.5` rentan (Prototype Pollution, ReDoS) dan dipakai untuk parse file user | `frontend/CompanyPortal-Vite/src/features/forecasting/utils/dsr-parser.ts` (dipakai `DSRUploadForm.tsx`) | Hapus parse client-side (DSR diproses Python ETL) atau pindah ke build resmi SheetJS ≥ 0.20.2. Perlu approval dependency (Sec 4 #4). | done 1fbd67d |
| FE-03 | `seroval` high (transitif via TanStack Router) | `pnpm audit --prod` CompanyPortal | Bump `@tanstack/react-router` sampai `seroval` ≥ 1.6.3. | done fb3b6fd |
| FE-04 | Tanpa code splitting: 40 route di-import eager | `frontend/CompanyPortal-Vite/src/main.tsx`; bundle 1,79 MB; `crown-login-artwork.png` 2 MB | Lazy route bawaan TanStack Router (`lazyRouteComponent` / `.lazy.tsx`); kompres artwork ke WebP/AVIF. Target NFR dashboard ≤ 3 s p95. | done d25ae2b — slice C1+C2: 32 file route CP (33 route) `lazyRouteComponent`, `scripts/check-bundle.mjs` di `build`; initial JS entry+`/login`+`/` 502,6 → 172,6 KB gzip (budget 300); artwork PNG 1,99 MB → WebP 98 KB |
| FE-05 | Layar produksi masih memakai data mock `src/data/*.json` | Company: `features/dashboard/{MetricStrip,AttentionPanel,ReplenishmentSummary}`, `features/cit`, `features/dsr`, `features/forecast`, `features/invoice`, `features/reconciliation`, `features/replenishment`. Vendor: `features/{invoices,schedule,evidence}` | Per layar: sambungkan ke API, atau beri banner "data contoh" / sembunyikan menu sampai backend siap. | done d25ae2b — slice D1+D2: route + menu `cit`, `dsr-dashboard`, `invoice/list`, `invoice/reconciliation` (CP) dan `invoices`, `schedule`, `orders/$id/evidence` (VP) dihapus, folder fitur tetap; banner "Data contoh" di `MetricStrip` + `AttentionPanel`. `/replenishment` + `ReplenishmentSummary` **tetap** (sudah pakai API sejak 2.2c, keputusan user 2026-10-15) |
| FE-06 | Tanpa CI, tanpa E2E, tanpa threshold coverage | Tidak ada `.github/workflows` / Cloud Build; Playwright terpasang tapi 0 test E2E; `vite.config.ts` tanpa `coverage.thresholds` (target 80%, CLAUDE.md Sec 8) | Pipeline: lint + tsc + test + build + audit untuk kedua portal (gabung dengan BE-02). E2E untuk alur kritis (login, vendor request, approval). | open |

## MEDIUM

| ID | Item | Bukti | Perbaikan | Status |
|---|---|---|---|---|
| FE-07 | Error boundary tidak terpasang di router CompanyPortal | `ErrorBoundary` hanya di `features/dashboard/DashboardScreen.tsx`; router tanpa `defaultErrorComponent` → error render = layar putih | `createRouter({ routeTree, defaultErrorComponent })`. VendorPortal sudah ada di `app/AppShell.tsx`. | done e498519 |
| FE-08 | Upload/download master-data melewati API client | `frontend/CompanyPortal-Vite/src/features/master-data-io/api.ts:79,104` memakai `fetch` sendiri → tidak ikut refresh 401 | Tambah dukungan `FormData`/blob di `lib/api/client.ts` lalu pakai di sini. | done e498519 — `ApiResponse.headers`, `responseType: "blob"`, FormData; `parseApiError` kini juga membawa `errors` ke `details` |
| FE-09 | Pola auth berbeda antar portal + UI/util terduplikasi | Company: Zustand `lib/auth/store.ts`; Vendor: React Context `features/auth/AuthContext.tsx`; `components/ui` dobel | Samakan saat salah satu disentuh; shared package hanya bila duplikasi mulai merepotkan. | done (uncommitted, 2026-10-15) — `sdlc/fe-auth-unify` opsi (b): VP init `/refresh` single-flight + token tanpa jeda effect; CP `formatIDR` → `formatRp`/`formatThousands`/`formatDecimalIDR`. Opsi (a) Zustand di VP ditunda (pemicu di spec), (c) shared package ditolak |
| FE-10 | Sisa kode mati / tumpang-tindih | 4 file `*.backup` di `src/` (`routes/_auth.tsx.backup`, `routes/_auth/login.tsx.backup`, `styles/index.css.backup`, `styles/tokens.css.backup`); `lib/api/stubs` + `VITE_API_MODE` tidak dipakai; folder `forecast` / `forecasting` / `dmaa-forecast` tumpang-tindih padahal forecast engine di-drop 2026-10-08 | Hapus backup + stubs; konsolidasi folder forecast (cek route & menu dulu). | in-progress 2026-10-15 — `*.backup`, `lib/api/stubs`, `VITE_API_MODE`/`apiConfig.mode` dihapus; folder forecast TIDAK dikonsolidasi (lihat tests.md) |

## LOW

| ID | Item | Bukti | Perbaikan | Status |
|---|---|---|---|---|
| FE-11 | `.env.local` ikut ke Docker build context | `frontend/CompanyPortal-Vite/.dockerignore` tidak memuat `.env*` (VendorPortal sudah) | Tambah `.env*` ke `.dockerignore`; set `VITE_*` lewat build arg. | done e498519 |
| FE-12 | Komponen mendekati batas 800 baris | `features/vendor-request/VendorRequestDetail.tsx` (734), `VendorRequestCreate.tsx` (598) | Pecah saat disentuh berikutnya. | open |
| FE-13 | Coverage FE di bawah target 80% | CompanyPortal 2026-10-12: statements 69,85 · branches 70,11 · functions 64,13 · lines 70,64. Threshold belum terpasang saat itu (FR-E2 tidak jadi); dipasang oleh `sdlc/fe-coverage` C0 | Naikkan threshold bertahap ke 80% sambil menambah test. | in-progress 2026-10-15 — `sdlc/fe-coverage` C0–C4: basis `src/**`, threshold CP 78/73/74/79, VP 81/73/77/82, gate `pnpm run test:coverage`; C5 berikutnya |
