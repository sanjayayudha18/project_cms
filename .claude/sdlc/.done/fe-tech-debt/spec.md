# Spec: Perbaikan tech debt frontend (FE-01 – FE-12)

Status: **accepted 2026-10-12** (user: "terima spec, lanjut plan"; S1–S5 sesuai usulan). Stage: 2 Design — selesai. Stage berikut: `plan.md`.
Sumber: `intent.md` (accepted 2026-10-12, Q1–Q8) + backlog [`FE_debt.md`](../../FE_debt.md).

## ⚠ Flagged (Sec 4 #7)
- **Auth**: FR-B2 mengubah `lib/api/client.ts` (jalur header Bearer + refresh 401). Perilaku untuk request JSON yang ada tidak boleh berubah. Ruang lingkupnya hanya menambah dukungan `FormData`/blob dan memindahkan master-data-io ke jalur yang sama. FE-09 (menyamakan auth Zustand vs Context) **ditunda** (Q8).
- **Delete**: FR-F1 menghapus file kode, route, dan dependency `xlsx`. Tidak ada data atau tabel yang dihapus.
- Tidak ada migrasi skema, tidak ada perubahan backend, response shape API tidak berubah.

## Baseline (diukur 2026-10-12)

| | CompanyPortal | VendorPortal |
|---|---|---|
| Initial JS (1 chunk) | 1.789.895 B raw / **502.601 B gzip** | 565.243 B raw / 165.690 B gzip |
| Artwork login | `public/assets/crown-login-artwork.png` 1.985.321 B | – |
| Coverage (vitest v8) | statements 69,85 · branches 70,11 · functions 64,13 · lines 70,64 | belum terukur (`@vitest/coverage-v8` tidak terpasang) |
| `pnpm audit --prod` | 3 high: `xlsx` ×2, `seroval` (via `@tanstack/router-core@1.171.15`) | 0 |

## Fase & functional requirements

### Fase A — Keamanan

**FR-A1 — Security header di semua respons (FE-01).** Berlaku di `frontend/CompanyPortal-Vite/nginx.conf` dan `frontend/VendorPortal-Vite/nginx.conf`.
- Header dikirim di **setiap** `location` yang melayani HTML/aset. Caranya: satu file snippet per portal (`security-headers.conf`) yang di-`include` di tiap `location` yang punya `add_header`, karena `add_header` level `server` tidak diwarisi.
- Header yang dikirim:
  - `X-Frame-Options: SAMEORIGIN`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Strict-Transport-Security: max-age=31536000` (tanpa `includeSubDomains`/`preload`; TLS di Nginx ini, Q4)
  - `Content-Security-Policy`, **enforce** (Q4):
    - CompanyPortal: `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'`
    - VendorPortal: sama, ditambah `style-src https://fonts.googleapis.com`, `font-src https://fonts.gstatic.com`, `img-src blob:` (Google Fonts di `index.html`; preview upload memakai `URL.createObjectURL` di `components/ui/FileUpload.tsx`).
- `data:` di `img-src`/`font-src` diperlukan karena Vite meng-inline aset < 4 KB.
- Atribut `style` dari React di-set lewat CSSOM, sehingga tidak diblokir `style-src 'self'`. Plan wajib memeriksa bahwa tidak ada library yang menyisipkan tag `<style>` saat runtime. Kalau ada, hanya library itu yang ditangani, bukan dengan melonggarkan ke `'unsafe-inline'` tanpa persetujuan.

**FR-A2 — Hapus `xlsx` dan halaman Unggah DSR CompanyPortal (FE-02, Q2).**
- Hapus route `/forecasting/dsr-upload` (`routes/forecasting/dsr-upload.tsx` + registrasinya di `main.tsx`), `DSRUploadForm.tsx`, `utils/dsr-parser.ts`, ekspornya di `features/forecasting/index.ts`, test terkait, dan dependency `xlsx` (`package.json` + lockfile).
- Komponen yang hanya dipakai halaman itu (misalnya `UploadHistory`) ikut dihapus. Kalau ternyata dipakai di tempat lain, dipertahankan; plan mengecek setiap pemakainya.
- Upload DSR resmi tetap lewat VendorPortal → `/api/v1/dsr` (tidak berubah).

**FR-A3 — `seroval` ≥ 1.6.3 (FE-03).** Naikkan `@tanstack/react-router` CompanyPortal ke versi terbaru di range `^1.x` (1.170.42 saat ini; router-core-nya membawa `seroval ^1.6.7`). Tidak ada dependency baru. VendorPortal tidak terdampak.

**FR-A4 — `.env*` tidak masuk build context Docker (FE-11).** Tambah `.env*` ke `frontend/CompanyPortal-Vite/.dockerignore`. Setelah FR-F1 menghapus `VITE_API_MODE`, build tanpa `.env.local` menghasilkan perilaku yang sama (`VITE_API_BASE_URL` default `/api/v1`).

### Fase B — Ketahanan

**FR-B1 — Error boundary global CompanyPortal (FE-07).** `createRouter({ routeTree, defaultErrorComponent })` di `src/main.tsx`. Komponennya memakai UI feedback yang sudah ada (`components/feedback/ServerError` atau `ErrorBoundary`), berbahasa Indonesia, dan punya tombol coba lagi (`router.invalidate()`). Error render di halaman mana pun tidak lagi menghasilkan layar putih.

**FR-B2 — Upload/download lewat API client (FE-08).**
- `lib/api/client.ts` mendukung body `FormData` (tanpa `JSON.stringify`, tanpa memaksa `Content-Type`) dan respons non-JSON (blob + header `Content-Disposition`).
- Keduanya melewati `injectAuthHeader` dan retry setelah refresh 401 yang sama.
- `features/master-data-io/api.ts` memakai client ini; `fetch` sendiri dan `authHeaders()` dihapus.
- Single-flight refresh yang terduplikasi di `client.ts` (`isRefreshing`/`refreshPromise`) disederhanakan, karena `useAuthStore.refreshToken()` sudah single-flight.
- Perilaku request JSON yang ada tidak berubah. Dibuktikan oleh test yang sudah ada di `lib/api/*.test.ts` plus test baru untuk FormData/blob + 401.

### Fase C — Performa

**FR-C1 — Lazy route CompanyPortal (FE-04, Q7).**
- Komponen route dimuat lazy dengan `lazyRouteComponent` dari TanStack Router; definisi route (path, `beforeLoad`, guard RBAC) tetap eager.
- Target: initial JS untuk `/login` dan `/` (dashboard) **≤ 300 KB gzip** (baseline 502,6 KB). Diukur dari output `vite build`.
- VendorPortal sudah 165,7 KB gzip, di bawah target, jadi **tidak diubah**.

**FR-C2 — Artwork login (FE-04).** `crown-login-artwork.png` (1,99 MB) dikonversi sekali menjadi WebP ≤ 300 KB dengan dimensi tampilan yang sama, lalu `ARTWORK_SRC` di `routes/_auth.tsx` diganti. Konversi memakai tool di luar repo; tidak ada dependency baru.

### Fase D — Data mock (FE-05, Q3)

**FR-D1 — Layar mock modul yang belum dibangun: keluar dari menu dan route tree.** Entri `NAV_CONFIG` / `NAV_ITEMS` dihapus dan route-nya tidak lagi didaftarkan di `main.tsx`, sehingga URL langsung berujung ke halaman not-found. Kode fitur dibiarkan sebagai prototipe UI untuk modul yang nanti dibangun, kecuali yang dihapus di FR-F1. Lihat keputusan S2.

| Portal | Route | Komponen (mock) | Menu sekarang |
|---|---|---|---|
| Company | `/replenishment` ("Jadwal CIT") | `features/replenishment/ReplenishmentScreen` | aktif (ATM-USER/SPV) |
| Company | `/cit` | `features/cit/CitTracker` | sudah di-comment |
| Company | `/forecasting/dsr-dashboard` | `features/dsr` (`useDsrData`) | aktif |
| Company | `/invoice/list` | `features/invoice/InvoiceFlow` | aktif |
| Company | `/invoice/reconciliation` | `features/reconciliation/ReconciliationScreen` | aktif (`disabled: false`) |
| Vendor | `/invoices` | `features/invoices` (`useInvoices`) | aktif |
| Vendor | `/schedule` | `features/schedule` (`useSchedule`) | aktif |
| Vendor | `/orders/$id/evidence` | `features/evidence` (`useEvidence`) | `hiddenFromNav`, tidak ditautkan dari Orders |

Entri menu yang sudah `disabled: true` (cash count, invoice upload, holiday calendar, dll.) tidak diubah.

**FR-D2 — Dashboard CompanyPortal tetap tampil dengan penanda.** Widget `MetricStrip`, `AttentionPanel`, dan `ReplenishmentSummary` (data dari `src/data/*.json`) masing-masing diberi banner "Data contoh — belum terhubung ke sistem". Banner memakai komponen yang sudah ada (`NoticeBanner`) dengan ikon + teks, bukan warna saja (`rules/frontend-ui.md`). Widget dashboard yang memakai API asli tidak diberi banner.

### Fase E — CI & E2E (FE-06, Q5–Q6)

**FR-E1 — GitHub Actions.**
- Satu workflow `.github/workflows/ci.yml` untuk seluruh repo. Fase ini mengisi job frontend; BE-02 menambahkan job backend ke file yang sama.
- Trigger: `push` dan `pull_request` ke `main` dan `dev1`.
- Job frontend, matrix `CompanyPortal-Vite` × `VendorPortal-Vite`; Node 22 dan pnpm 11.20.0 via corepack, sama dengan Dockerfile:
  1. `pnpm install --frozen-lockfile`
  2. `pnpm lint`
  3. `pnpm build` (tsc + vite build; termasuk pengecekan budget FR-C1 untuk CompanyPortal)
  4. `pnpm test` dengan coverage + threshold (FR-E2)
  5. `pnpm audit --prod --audit-level high`
- Job `frontend-image`: `docker build` kedua image, jalankan container, `nginx -t`, lalu `curl -I` pada `/`, `/login`, dan satu aset hashed. Header FR-A1 wajib ada di setiap respons.

**FR-E2 — Threshold coverage Vitest.**
- `vite.config.ts` kedua portal memasang `test.coverage.thresholds`. Nilainya ditentukan di keputusan S1.
- VendorPortal menambah devDependency `@vitest/coverage-v8` dengan versi sama seperti CompanyPortal (`^4.1.10`). Ini dependency dev baru dan **perlu approval** (Sec 4 #4, keputusan S4).

**FR-E3 — E2E Playwright (Q6).**
- Spec E2E untuk alur wajib:
  1. login internal (CompanyPortal)
  2. login vendor (VendorPortal)
  3. buat Vendor Request → submit → approve oleh ATM-SPV
  4. vendor menerima dan menolak order di VendorPortal
- Memakai user dari seed dev (`002_baseline_seed.sql`). Kredensial dibaca dari env, tidak di-hardcode.
- Tempat menjalankan: keputusan S3.
- VendorPortal menambah devDependency `@playwright/test`. Pilihan lain: E2E dipusatkan di CompanyPortal yang sudah punya Playwright dan membuka dua origin. Ditetapkan di plan, mengikuti S4.

### Fase F — Kebersihan

**FR-F1 — Hapus kode mati (FE-10).**
- CompanyPortal:
  - 4 file `*.backup` (`routes/_auth.tsx.backup`, `routes/_auth/login.tsx.backup`, `styles/index.css.backup`, `styles/tokens.css.backup`).
  - `lib/api/stubs/` beserta `ApiMode`/`mode`/`stubLatency` di `lib/api/config.ts`, dan `VITE_API_MODE` dari `.env.local`/dokumentasi.
  - `features/forecast/` + route `/forecasting/forecast` (forecast engine di-drop 2026-10-08; menunya sudah di-comment).
  - Sisa `features/forecasting/` setelah FR-A2 dan FR-D1: folder yang tidak lagi dipakai siapa pun dihapus. `features/dmaa-forecast` (live) tetap.
  - `index.html`: favicon `/vite.svg` (file tidak ada) diganti `/favicon.svg`.
- VendorPortal: sisa template Vite yang tidak di-import, yaitu `src/App.tsx`, `src/App.css`, dan `src/index.css` (`main.tsx` meng-import `styles/index.css`). Plan memastikan tidak ada pemakainya.

**FR-F2 — Pecah `VendorRequestDetail.tsx` (FE-12).** Saat ini 734 baris. Panel aksi dan section diekstrak menjadi komponen di folder yang sama sampai file utama ≤ 400 baris, tanpa perubahan perilaku (test yang ada tetap hijau). `VendorRequestCreate.tsx` (598) tidak diubah karena masih di bawah batas.

## Non-functional
- **NFR1:** perilaku yang tidak disebut di atas tidak berubah. Semua test yang ada tetap hijau di kedua portal, dan `biome check` serta `tsc -b` tetap bersih.
- **NFR2:** `pnpm audit --prod` di kedua portal = 0 high/critical.
- **NFR3:** initial JS CompanyPortal untuk `/login` dan `/` ≤ 300 KB gzip (FR-C1).
- **NFR4:** satu concern per commit, mengikuti urutan fase A → F. Setiap fase bisa di-review dan di-revert sendiri.
- **NFR5:** tidak ada dependency runtime baru. DevDependency baru hanya yang disetujui di S4.
- **NFR6:** manual browser verification oleh user (GR#10): halaman tampil normal dengan CSP enforce (tidak ada pelanggaran CSP di console), error page, lazy route, banner dashboard, menu tanpa layar mock. Dicatat outstanding di `tests.md`.

## Tests (dibuat di stage 4, dilacak di `tests.md`)
| ID | Membuktikan | Bentuk |
|---|---|---|
| T-A1 | Header FR-A1 ada di `/`, route SPA, dan aset hashed kedua portal | job CI `frontend-image` (`curl -I` + `nginx -t`) |
| T-A2 | `xlsx` tidak ada; `/forecasting/dsr-upload` tidak terdaftar | `pnpm why xlsx` kosong + test route tree |
| T-A3 | audit 0 high | `pnpm audit --prod --audit-level high` di CI |
| T-B1 | error render → komponen error, bukan layar kosong | Vitest: route yang melempar error |
| T-B2 | FormData + blob lewat client; 401 → refresh → retry sekali; header Bearer terpasang | Vitest `lib/api/client.test.ts` + test `master-data-io` |
| T-C1 | initial JS ≤ 300 KB gzip | langkah budget di `pnpm build` CI |
| T-D1 | route FR-D1 tidak ada di menu dan route tree | Vitest `navigation.test.ts` + test route tree kedua portal |
| T-D2 | banner tampil di 3 widget mock, tidak tampil di widget API | Vitest komponen dashboard |
| T-E1 | coverage ≥ threshold | `vitest run --coverage` di CI |
| T-E2 | 4 alur E2E lulus | Playwright (tempat sesuai S3) |
| T-F1 | file FR-F1 hilang, build + test hijau | CI |

## Keputusan spec (dikonfirmasi PO 2026-10-12, S1–S5 semua sesuai usulan)
- **S1 — Nilai threshold coverage.** Coverage CompanyPortal saat ini ~70%, jadi threshold 80% akan langsung membuat CI merah. Usul: threshold = **baseline dibulatkan ke bawah** (CompanyPortal 69/70/64/70 untuk statements/branches/functions/lines; VendorPortal diukur di plan) supaya coverage tidak turun. Kenaikan ke 80% menjadi item baru **FE-13** di `FE_debt.md`, dikerjakan bertahap.
- **S2 — "Sembunyikan" = keluar dari menu dan route tree.** Akses lewat URL langsung berujung not-found. Kode fitur tetap disimpan sebagai prototipe UI, kecuali `forecast` dan Unggah DSR yang dihapus. Alternatifnya halaman "Segera hadir" di route lama, tapi itu kode tambahan tanpa manfaat bagi user.
- **S3 — Tempat E2E.** Usul: di AI-DLC ini E2E dijalankan **lokal** (`pnpm e2e`, butuh backend :8080 + DB dev ber-seed). Job E2E di GitHub Actions ditambahkan bersama BE-02, yang memang menyiapkan Postgres + backend di CI. Item ini dicatat di `BE_debt.md` BE-02.
- **S4 — DevDependency baru.** `@vitest/coverage-v8` untuk VendorPortal (wajib untuk FR-E2), dan `@playwright/test` untuk VendorPortal hanya bila E2E tidak dipusatkan di CompanyPortal. Usul: **setujui `@vitest/coverage-v8`**, dan E2E dipusatkan di CompanyPortal (Playwright sudah ada), sehingga Playwright tidak ditambahkan ke VendorPortal.
- **S5 — TLS listener.** Nginx kedua portal sekarang hanya `listen 80`. HSTS hanya berlaku lewat HTTPS. Usul: AI-DLC ini memasang header HSTS saja. Konfigurasi `listen 443 ssl`, sertifikat, dan redirect 80→443 dikerjakan saat setup deploy, dicatat di `docs/deployment.md`. Pilihan lain: blok TLS ditambahkan sekarang dengan path sertifikat dari volume.

## Out of scope
- FE-09: menyamakan auth Zustand vs React Context (Q8, ditunda).
- Membangun API untuk layar mock (Invoice, Reconciliation, Jadwal CIT, DSR dashboard, Vendor Invoices/Schedule/Evidence), karena itu fitur per modul.
- Upload DSR oleh internal atas nama vendor.
- Lazy route dan perubahan bundle VendorPortal.
- Konfigurasi TLS/sertifikat (S5), job backend dan E2E di CI (BE-02).
- Shared package UI antar portal.
