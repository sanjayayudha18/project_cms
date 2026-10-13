# Plan: Perbaikan tech debt frontend (FE-01 – FE-12)

Status: **accepted 2026-10-12** (user: "plan diterima, tapi jangan coding dulu"). Stage: 3 Build — coding belum dimulai, menunggu perintah user.
Sumber: `spec.md` (accepted 2026-10-12, S1–S5), `intent.md` (accepted 2026-10-12).
Singkatan path: `CP/` = `frontend/CompanyPortal-Vite/`, `VP/` = `frontend/VendorPortal-Vite/`.

## Titik sentuh yang sudah dicek di kode (2026-10-12)
- **Nginx**: `CP/nginx.conf` punya `add_header` di level `server` (tidak diwarisi oleh `location /` dan `location ~* \.(js|css…)`); `VP/nginx.conf` tanpa security header. Kedua Dockerfile meng-`COPY nginx.conf /etc/nginx/conf.d/default.conf`.
- **CSP blocker**: `CP/src/features/eod-monitoring/components/RetryConfirmationDialog.tsx:112` dan `RetryDrawer.tsx:192` merender `<style>` inline (keyframes `eod-dialog-*` / `eod-drawer-*`). Ini akan diblokir `style-src 'self'`. Satu-satunya `createElement("style")` di bundle berasal dari React DOM (hoisting `<style precedence>`), yang hanya aktif bila komponen merender `<style>`. VendorPortal tidak punya `<style>` inline.
- **VendorPortal font**: `VP/index.html` memuat Google Fonts (Plus Jakarta Sans, IBM Plex Mono), dipakai `VP/src/styles/index.css:93-95` → CSP butuh `fonts.googleapis.com` + `fonts.gstatic.com`.
- **xlsx**: hanya dipakai `CP/src/features/forecasting/utils/dsr-parser.ts` ← `components/DSRUploadForm.tsx` ← `routes/forecasting/dsr-upload.tsx`. `UploadHistory` + `hooks/useDSRHistory` hanya dipakai route itu. `DSRPreviewTable`, `dsr-validation.ts`, `types.ts`, dan `dsr.property.test.ts` hanya melayani alur yang sama. Akibatnya seluruh `features/forecasting/` menjadi tidak terpakai.
- **Router**: `CP/src/main.tsx` meng-import 42 file route secara eager; `createRouter({ routeTree })` tanpa `defaultErrorComponent`. TanStack Router terpasang 1.170.18 (router-core 1.171.15, `seroval` rentan); 1.170.42 tersedia di range `^1.93.0`.
- **API client**: `CP/src/lib/api/client.ts` selalu `JSON.stringify` + `Content-Type: application/json`, selalu `response.json()`; single-flight sendiri (`isRefreshing`/`refreshPromise`) di atas `useAuthStore.refreshToken()` yang sudah single-flight. Test: `lib/api/client.test.ts` (header, method, 401 refresh). `CP/src/features/master-data-io/api.ts:62-112` memakai `fetch` + `authHeaders()` sendiri.
- **Mock**: CompanyPortal nav (`lib/config/navigation.ts`, `NAV_CONFIG`) aktif untuk `replenishment`, `dsr-dashboard`, `invoice-list`, `reconciliation`; `cit` dan `forecast` sudah di-comment. Test `lib/config/navigation.test.ts` merujuk id tersebut. VendorPortal: `VP/src/lib/constants.ts` (`ROUTES` + nav), `VP/src/main.tsx`, `VP/src/test/routerTestUtils.tsx:106-116`, dan test `routeMapping`, `routeGuard`, `integration` merujuk `/invoices`, `/schedule`, `/orders/$id/evidence`.
- **Dashboard mock**: `CP/src/features/dashboard/{MetricStrip,AttentionPanel,ReplenishmentSummary}.tsx` import `@/data/*.json`. `components/ui/NoticeBanner.tsx` tersedia.
- **Coverage**: CompanyPortal sudah punya `@vitest/coverage-v8 ^4.1.10`; VendorPortal belum.
- **E2E**: `@playwright/test` hanya di CompanyPortal, tanpa config dan tanpa folder `e2e/` (`.dockerignore` CP sudah mengecualikan `e2e`).
- **Lain-lain**:
  - `CP/index.html` favicon `/vite.svg` (file tidak ada; yang ada `public/favicon.svg`).
  - `VP/src/{App.tsx,App.css,index.css}` tidak di-import siapa pun.
  - `VendorRequestDetail.tsx`: komponen utama baris 84–594, lalu `DetailFields`, `ReasonModal` (diekspor), `DetailSkeleton`, `NotFoundState`.
  - Konversi gambar: Pillow 12.3 tersedia lewat `py -3` (tool lokal, bukan dependency repo).

## Slices (urutan kerja; satu commit per slice, NFR4)

### Fase A — Keamanan

**A1 — Pindahkan `<style>` inline EOD ke CSS global** (prasyarat A2)
- Keyframes `eod-dialog-enter`, `eod-dialog-backdrop-enter`, `eod-drawer-enter`, `eod-drawer-backdrop-enter` dipindah ke `CP/src/styles/index.css`. Blok `<style>` di `RetryConfirmationDialog.tsx` dan `RetryDrawer.tsx` dihapus.
- Bukti: test EOD yang ada tetap hijau, dan `grep "<style" CP/src` kosong.

**A2 — Security header + CSP (FR-A1)**
- File baru `CP/security-headers.conf` dan `VP/security-headers.conf` berisi 5 header (`always`). CSP per portal persis seperti spec.
- `nginx.conf` kedua portal: `include /etc/nginx/snippets/security-headers.conf;` di level `server` (untuk `/api/`, yang tidak punya `add_header`) **dan** di setiap `location` yang punya `add_header` (`/` dan aset statis). Blok header lama di `CP/nginx.conf` dihapus.
- Kedua Dockerfile: `COPY security-headers.conf /etc/nginx/snippets/security-headers.conf`.
- Bukti: T-A1 (job CI di E1); lokal `docker build` + `curl -I`.

**A3 — Hapus Unggah DSR + `xlsx` (FR-A2)**
- Hapus `CP/src/routes/forecasting/dsr-upload.tsx`, import + registrasi `dsrUploadRoute` di `main.tsx`, dan seluruh `CP/src/features/forecasting/` (lihat titik sentuh: tidak ada pemakai lain), termasuk test-nya.
- Hapus entri menu `dsr-upload` yang di-comment di `navigation.ts`.
- `pnpm remove xlsx`.
- Bukti: T-A2 (`pnpm why xlsx` kosong; route tree tanpa `/forecasting/dsr-upload`).

**A4 — Bump TanStack Router (FR-A3)**
- `pnpm update @tanstack/react-router` di CompanyPortal (lockfile ikut berubah).
- Bukti: `pnpm audit --prod` = 0 high (setelah A3); test + build hijau.

**A5 — `.dockerignore` (FR-A4)**
- Tambah `.env*` ke `CP/.dockerignore`.
- Bukti: `docker build` sukses tanpa `.env.local` ikut ke context.

### Fase B — Ketahanan

**B1 — `defaultErrorComponent` (FR-B1)**
- Komponen baru `CP/src/components/feedback/RouteError.tsx`: pesan Indonesia dari `sanitizeServerError` + tombol "Coba lagi" yang memanggil `router.invalidate()`. Diekspor lewat `components/feedback/index.ts`.
- `CP/src/main.tsx`: `createRouter({ routeTree, defaultErrorComponent: RouteError })`.
- Bukti: T-B1 (Vitest: route anak yang melempar error → teks error + tombol tampil).

**B2 — API client untuk FormData/blob (FR-B2)** ⚠ auth
- `client.ts`:
  - `RequestConfig` mendapat `responseType?: "json" | "blob"`.
  - Body `FormData` dikirim apa adanya; `Content-Type` hanya diset untuk body non-FormData.
  - Fungsi baru `apiDownload(path)` → `{ blob, filename }` melalui `executeRequest` yang sama.
  - `isRefreshing`/`refreshPromise` lokal dihapus; langsung `await useAuthStore.getState().refreshToken()`.
- `master-data-io/api.ts`: `download`/`upload` memakai `apiDownload` / `api.post(path, form)`; `authHeaders()` dan import `apiConfig`/`useAuthStore` dihapus. Mapping error ke `ImportApiError` (status + `errors[]`) tetap.
- Bukti: T-B2. Test `client.test.ts` yang ada tetap hijau tanpa diubah. Test baru: FormData tanpa `Content-Type` JSON; blob + filename; 401 → refresh → retry untuk FormData; dua request 401 bersamaan → `refreshToken` dipanggil sekali. Test `master-data-io` untuk import error 422 (`errors[]`).

### Fase C — Performa

**C1 — Lazy route (FR-C1)**
- Di setiap file `CP/src/routes/**`: `component: X` (import dari `@/features/...`) diganti `component: lazyRouteComponent(() => import("@/features/..."), "X")`. Import statis komponen dihapus; `beforeLoad`/guard tetap.
- Route yang komponennya didefinisikan inline di file route dibiarkan bila kecil (wrapper header + komponen fitur). Komponen fitur di dalamnya tetap di-lazy lewat `lazyRouteComponent` pada file route itu sendiri, dengan memindahkan wrapper ke file feature. Daftar finalnya dicatat di commit.
- `CP/vite.config.ts`: `build.manifest: true`.
- Script baru `CP/scripts/check-bundle.mjs` (Node stdlib `zlib` + `fs`): baca `dist/.vite/manifest.json`, hitung gzip dari closure import statis entry `index.html` + chunk route `/login` dan `/` (dashboard), gagal bila > 300 KB.
- `package.json` `build`: `… && vite build && node scripts/check-bundle.mjs`.
- Bukti: T-C1 (output script; baseline 502,6 KB).

**C2 — Artwork WebP (FR-C2)**
- Konversi sekali dengan `py -3` + Pillow (di luar repo, tidak ada dependency baru) → `CP/public/assets/crown-login-artwork.webp` ≤ 300 KB dengan resolusi sama.
- `ARTWORK_SRC` di `routes/_auth.tsx` diganti; PNG dihapus.
- Bukti: ukuran file; tampilan dicek user (NFR6).

### Fase D — Data mock

**D1 — Keluarkan layar mock dari menu + route tree (FR-D1, S2)**
- CompanyPortal:
  - Hapus entri `replenishment`, `dsr-dashboard`, `invoice-list`, `reconciliation` (+ blok `cit` yang di-comment) dari `NAV_CONFIG`.
  - Hapus import/registrasi `replenishmentRoute`, `citRoute`, `dsrDashboardRoute`, `invoiceListRoute`, `reconciliationRoute` di `main.tsx`, beserta file route-nya (`routes/replenishment.tsx`, `routes/cit.tsx`, `routes/forecasting/dsr-dashboard.tsx`, `routes/invoice/list.tsx`, `routes/invoice/reconciliation.tsx`).
  - Folder fitur (`features/replenishment`, `cit`, `dsr`, `invoice`, `reconciliation`) tetap sebagai prototipe.
  - Cek `routes/forecasting/index.tsx` dan `routes/invoice/index.tsx` (hub): tautan ke route yang dihapus ikut dihapus.
- VendorPortal:
  - Hapus `INVOICES`, `SCHEDULE`, `EVIDENCE` dari `ROUTES` + nav di `lib/constants.ts`, `invoicesRoute`/`scheduleRoute`/`evidenceRoute` di `main.tsx`, dan file `routes/invoices.tsx`, `routes/schedule.tsx`, `routes/orders.$id.evidence.tsx`.
  - Update `test/routerTestUtils.tsx` dan test yang merujuknya (`routeMapping`, `routeGuard`, `integration`, `dataFilters`/`sorting` bila memakai route).
  - Komentar `/orders/123/evidence` di `LoginPage.tsx`/`ProtectedRoute.tsx` diganti contoh yang masih ada.
- Bukti: T-D1 (`navigation.test.ts` + test route tree di kedua portal: path tidak terdaftar, URL langsung → not-found).

**D2 — Banner "Data contoh" di Dashboard (FR-D2)**
- `MetricStrip`, `AttentionPanel`, `ReplenishmentSummary` masing-masing merender `NoticeBanner` (ikon + teks "Data contoh — belum terhubung ke sistem") di atas kontennya.
- Bukti: T-D2 (Vitest: banner ada di 3 widget; widget dashboard berbasis API tidak memilikinya).

### Fase E — CI & E2E

**E1 — GitHub Actions (FR-E1)**
- `.github/workflows/ci.yml`. Trigger: `push`/`pull_request` ke `main`, `dev1`.
- Job `frontend`, matrix `app: [CompanyPortal-Vite, VendorPortal-Vite]`, `working-directory: frontend/${{ matrix.app }}`, Node 22 + `corepack enable`:
  1. `pnpm install --frozen-lockfile`
  2. `pnpm lint`
  3. `pnpm build` (CP menjalankan budget C1)
  4. `pnpm test -- --coverage`
  5. `pnpm audit --prod --audit-level high`
- Job `frontend-image`, matrix yang sama:
  1. `docker build`
  2. `docker run -d -p 8080:80`
  3. `docker exec … nginx -t`
  4. `curl -sI` ke `/`, `/login`, dan aset hashed pertama di `dist/assets` (dibaca dari container), lalu cek kelima header dengan `grep -i`.
- Job backend dan E2E ditambahkan oleh BE-02 (S3).
- Bukti: run hijau setelah user push. Saya tidak push; commit/push hanya atas perintah user.

**E2 — Threshold coverage (FR-E2, S1, S4)**
- VP: `pnpm add -D @vitest/coverage-v8@^4.1.10` (disetujui S4). Ukur baseline VP.
- Kedua `vite.config.ts`: `test.coverage = { provider: "v8", include: ["src/**"], thresholds: { statements, branches, functions, lines } }`. Nilainya = baseline dibulatkan ke bawah (CP 69/70/64/70; VP hasil ukur).
- Bukti: T-E1 (`vitest run --coverage` lulus di CI).

**E3 — E2E Playwright di CompanyPortal (FR-E3, S3, S4)**
- `CP/playwright.config.ts`: `testDir: "e2e"`, base URL dari env `E2E_COMPANY_URL` (default `http://localhost:5173`) dan `E2E_VENDOR_URL` (default `http://localhost:5174`), chromium saja.
- `CP/e2e/`: `auth.spec.ts` (login internal + vendor), `vendor-request.spec.ts` (buat → submit → approve ATM-SPV), `vendor-order.spec.ts` (vendor terima + tolak).
  - Kredensial dari env `E2E_*_EMAIL` / `E2E_*_PASSWORD`, tidak ada default di kode.
  - Data dibuat oleh test itu sendiri (request baru per run) supaya bisa diulang.
- `package.json`: `"e2e": "playwright test"`.
- Lingkungan: backend :8080 + DB dev ber-seed + kedua dev server. Instal browser `pnpm exec playwright install chromium` mengunduh ±150 MB, jadi **minta izin user dulu** saat sampai di langkah ini.
- Dokumentasi menjalankan + daftar env di `CP/README.md`.
- `CP/vite.config.ts` `test.include` tetap `src/**`, jadi Vitest tidak memungut `e2e/`.
- Bukti: T-E2 dijalankan lokal; hasilnya dicatat di `tests.md`.

### Fase F — Kebersihan

**F1 — Kode mati (FR-F1)**
- CompanyPortal:
  - Hapus 4 `*.backup`.
  - Hapus `lib/api/stubs/`; di `lib/api/config.ts` hapus `ApiMode`/`mode`/`stubLatency` + `resolveMode`; ekspor `ApiMode` di `lib/api/index.ts` ikut dihapus; test config/stubs disesuaikan.
  - `VITE_API_MODE` dihapus dari `.env.local` dan `vite-env.d.ts` bila dideklarasikan di sana.
  - Hapus `features/forecast/` + `routes/forecasting/forecast.tsx` + registrasinya + blok menu `forecast` yang di-comment.
  - `index.html` favicon → `/favicon.svg`.
- VendorPortal: hapus `src/App.tsx`, `src/App.css`, `src/index.css` (cek dulu tidak ada import/test).
- Bukti: T-F1 (build + test + lint hijau).

**F2 — Pecah `VendorRequestDetail.tsx` (FR-F2)**
- Pindahkan `DetailFields`, `ReasonModal` (+ ekspornya; import pemakai diperbarui), `DetailSkeleton`, `NotFoundState` ke `VendorRequestDetailParts.tsx`.
- Ekstrak panel aksi maker/checker (tombol + modal + handler mutation) dari komponen utama ke `VendorRequestDetailActions.tsx`. Konstanta peran/status ikut ke file yang memakainya.
- Target file utama ≤ 400 baris; perilaku tidak berubah.
- Bukti: test `features/vendor-request/__tests__` yang ada tetap hijau tanpa diubah (selain path import).

## Bukti selesai (per slice dan di akhir)
- Kedua portal: `pnpm lint` dan `pnpm build` (CP termasuk budget) hijau; `pnpm test -- --coverage` lulus threshold; `pnpm audit --prod --audit-level high` = 0.
- `docker build` kedua image + `curl -I` menunjukkan 5 header.
- E2E 4 alur lulus lokal.
- `graphify update .` setelah perubahan kode (CLAUDE.md 1a).
- Manual browser check oleh user (NFR6): CSP tanpa pelanggaran di console (kedua portal, termasuk EOD dialog/drawer dan font VendorPortal), error page, lazy route, banner dashboard, menu tanpa layar mock, artwork login.
- Status FE-01 … FE-12 di `FE_debt.md` diperbarui (`done <sha>`) per fase setelah commit.

## Risiko
- **CSP memblokir sesuatu yang belum terdeteksi**, misalnya `style` dari library saat runtime. Mitigasi: A1 + pengecekan bundle sudah dilakukan; console dicek di manual check. Kalau ternyata ada, perbaiki sumbernya. Melonggarkan CSP butuh persetujuan PO.
- **B2 menyentuh jalur auth.** Mitigasi: test lama tidak diubah, plus test baru untuk 401 bersamaan. Perilaku logout/redirect tetap di `useAuthStore`.
- **C1 mengubah 42 file route sekaligus.** Mitigasi: perubahannya mekanis (satu pola), guard tidak disentuh, test route + build menjaga.
- **D1 bisa memutus tautan** dari halaman hub atau notifikasi ke route yang dihapus. Mitigasi: grep path sebelum hapus; VendorPortal notifikasi tidak merujuk route tersebut (sudah dicek).
- **E1 belum bisa dibuktikan sebelum push.** Workflow ditulis lalu divalidasi oleh run GitHub setelah user push.
- **Threshold VP belum diketahui.** Kalau baseline VP sangat rendah, angkanya tetap dipasang apa adanya dan dicatat di FE-13.

## Di luar plan
- FE-09 (auth Zustand vs Context), TLS listener/sertifikat (S5), job backend + E2E di CI (BE-02), API untuk layar mock, lazy route VendorPortal.
