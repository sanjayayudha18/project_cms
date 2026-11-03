# Intent: Perbaikan tech debt frontend (FE-01 – FE-12)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-12** (PO, Q1–Q8 dijawab di bawah).
Stage: 1 Plan. Stage berikut: `spec.md`.
Asal: review arsitektur FE 2026-10-12 → backlog [`.claude/FE_debt.md`](../../FE_debt.md).
Menyentuh Sec 4 #7: **auth** (FE-08: API client + refresh 401; FE-09: store/context auth) dan **delete** (FE-10: hapus file/kode mati, bukan data). Tidak ada migrasi skema, tidak ada perubahan backend.

## Problem
Kata user (2026-10-12): *"buat sdlc untuk perbaikan .claude\FE_debt.md"*.

Fondasi FE sudah modern (React 19, Vite 6, TS strict, TanStack, Biome, 1.205 test hijau). Masalahnya ada di kesiapan produksi. Kondisi saat ini sudah dicek di kode (detail dan bukti per item ada di `FE_debt.md`):

- **Keamanan**
  - Security header Nginx tidak pernah terkirim, karena `add_header` di level `server` tidak diwarisi oleh `location` yang punya `add_header` sendiri. Di VendorPortal header-nya bahkan tidak ada. CSP dan HSTS belum ada di kedua portal. (FE-01)
  - `xlsx@0.18.5` (high: Prototype Pollution + ReDoS) dipakai untuk parse file yang di-upload user. (FE-02)
  - `seroval` (high) masuk sebagai dependency transitif TanStack Router. (FE-03)
- **Performa**: ke-40 route dimuat eager dalam satu bundle 1,79 MB, ditambah artwork login 2 MB. Ini berisiko melanggar NFR dashboard ≤ 3 s p95. (FE-04)
- **Kebenaran data**: 10 layar CompanyPortal dan 3 layar VendorPortal masih menampilkan data mock dari `src/data/*.json`. User bisa melihat angka uang palsu tanpa tahu itu data contoh. (FE-05)
- **Jaminan kualitas**: belum ada CI, belum ada test E2E (Playwright sudah terpasang tapi belum dipakai), dan Vitest belum memasang threshold coverage 80%. (FE-06)
- **Ketahanan**: error render di halaman selain Dashboard berakhir sebagai layar putih (FE-07). Upload/download master data juga tidak ikut alur refresh 401. (FE-08)
- **Pemeliharaan**: pola auth berbeda antar portal (FE-09), ada sisa kode mati dan folder forecast yang tumpang-tindih (FE-10), `.env.local` ikut ke Docker build (FE-11), dan beberapa komponen mendekati batas 800 baris (FE-12).

Temuan tambahan saat menyusun intent ini: halaman CompanyPortal **Unggah DSR** (`/forecasting/dsr-upload`; entri menunya sudah di-comment, tapi route masih terdaftar dan bisa dibuka lewat URL) mem-parse Excel di browser dengan `xlsx`, lalu `POST /forecasting/dsr`. Endpoint itu **tidak ada** di backend Go. Upload DSR yang berjalan adalah alur VendorPortal → `/api/v1/dsr` → Python ETL. Jadi halaman ini mati sekaligus menjadi satu-satunya pemakai `xlsx`.

## Proposed outcome
1. Kedua portal mengirim security header (X-Frame-Options, nosniff, Referrer-Policy, CSP, HSTS) di semua respons HTML dan aset. Kebenarannya dibuktikan dengan test konfigurasi atau `curl -I` terhadap container.
2. `pnpm audit --prod` bersih (0 high) di kedua portal.
3. Route dimuat secara lazy. Initial JS CompanyPortal turun jauh dari 1,79 MB (target angka ditetapkan di spec) dan artwork login dikompres.
4. Tidak ada layar produksi yang menampilkan data mock tanpa penanda. Setiap layar mock diputuskan: disambungkan ke API, diberi banner "data contoh", atau disembunyikan dari menu.
5. Ada CI yang menjalankan lint, tsc, test (dengan threshold coverage), build, dan audit untuk kedua portal. Ada E2E untuk alur kritis.
6. Error render menampilkan halaman error, bukan layar putih. Semua request, termasuk upload/download, lewat satu API client yang menangani refresh 401.
7. Kode mati dihapus, folder forecast dikonsolidasi, dan Docker build tidak membawa `.env*`.

## Affected users and systems
- **Users:**
  - Internal (CompanyPortal) dan vendor (VendorPortal): halaman lebih cepat dimuat.
  - Layar mock berubah (diberi banner atau disembunyikan).
  - Halaman Unggah DSR CompanyPortal kemungkinan dihapus atau dialihkan (Q2).
- **Systems:**
  - `frontend/CompanyPortal-Vite/` dan `frontend/VendorPortal-Vite/`: `nginx.conf`, `Dockerfile`, `.dockerignore`, `package.json`/lockfile, `vite.config.ts`, `src/main.tsx`, `src/routes/**`, `src/lib/api/client.ts`, `src/features/master-data-io/api.ts`, `src/features/forecasting/**`, `src/data/*.json`, serta fitur-fitur pemakai mock.
  - Pipeline CI baru (lokasinya bergantung pada Q5).
  - Backend tidak berubah.

## Constraints
- Stack tetap (CLAUDE.md Sec 2). Dependency baru, atau mengganti `xlsx` dengan versi di luar npm registry, butuh approval (Sec 4 #4).
- Small diffs, satu concern per perubahan: dikerjakan per item atau per kelompok kecil, bukan big-bang.
- Response shape API dan perilaku auth (token di memori, refresh via cookie httpOnly) tidak berubah. FE-08 dan FE-09 hanya merapikan jalur yang sudah ada.
- Brand/UI mengikuti `rules/frontend-ui.md`. Banner "data contoh" tidak boleh mengandalkan warna saja.
- Manual browser verification = tugas user (GR#10). Dicatat sebagai outstanding di `tests.md`.

## Open questions (untuk PO / spec)
1. **Scope & urutan**: semua 12 item dalam satu AI-DLC, atau hanya HIGH (FE-01 … FE-06) dulu sementara MEDIUM/LOW menyusul? Saran Claude: **satu AI-DLC, dibagi fase**. Fase A keamanan (FE-01/02/03/11), Fase B ketahanan (FE-07/08), Fase C performa (FE-04), Fase D data mock (FE-05), Fase E CI/E2E (FE-06), Fase F kebersihan (FE-10/12, FE-09 opsional).
2. **Halaman Unggah DSR CompanyPortal (FE-02)**: endpointnya tidak ada dan upload DSR resmi lewat VendorPortal. Halaman beserta `dsr-parser.ts` dan `xlsx` dihapus? Atau internal perlu bisa upload DSR atas nama vendor (berarti fitur baru di backend, di luar scope ini)? Saran Claude: **hapus** halaman + `xlsx`, sehingga FE-02 selesai tanpa dependency baru.
3. **Layar mock (FE-05)**: perlakuan per layar. Saran Claude, sambil menunggu backend-nya dibangun: **sembunyikan dari menu** layar yang modulnya belum dibangun (Invoice, Reconciliation, CIT tracker, Cash count, layar Vendor Invoices/Schedule/Evidence), dan **beri banner "data contoh"** untuk widget Dashboard yang masih mock. Apakah ada layar yang harus tetap tampil untuk demo ke stakeholder?
4. **CSP (FE-01)**: mulai dengan mode `Content-Security-Policy-Report-Only` dulu, atau langsung enforce? Saran Claude: **langsung enforce** dengan kebijakan `'self'`. Aplikasi tidak memuat script/font dari pihak ketiga; hal ini akan dikonfirmasi di spec. HSTS hanya aktif bila TLS diterminasi di Nginx ini. Di mana TLS diterminasi di produksi?
5. **CI (FE-06)**: platformnya GitHub Actions atau Cloud Build (`docs/deployment.md` mencantumkan Cloud Build)? Satu pipeline bersama BE-02, atau terpisah? Saran Claude: **satu pipeline untuk FE + BE**, platform mengikuti yang dipakai untuk deploy.
6. **E2E (FE-06)**: alur mana yang wajib ada? Saran Claude: login internal + vendor, buat & approve Vendor Request, terima/tolak order di VendorPortal. E2E butuh backend + DB seed: jalan di CI atau hanya lokal dulu?
7. **Target bundle (FE-04)**: angka target initial JS, misalnya ≤ 300 KB gzip untuk halaman login dan dashboard? Atau cukup "dipecah per route" tanpa angka?
8. **FE-09 (samakan auth Zustand vs Context)**: masuk scope sekarang, atau ditunda sampai salah satu auth disentuh? Saran Claude: **ditunda**. Risikonya auth (Sec 4 #7) dan manfaatnya kecil, cukup dicatat.

## Jawaban PO (2026-10-12)
Kata user: *"setuju semua, TLS di Nginx, CI pakai GitHub Actions"*.
1. **Scope**: semua 12 item dalam satu AI-DLC, dibagi fase A–F seperti saran.
2. **Unggah DSR CompanyPortal**: dihapus bersama `dsr-parser.ts` dan `xlsx`.
3. **Layar mock**: modul yang belum dibangun disembunyikan; widget Dashboard yang masih mock diberi banner "data contoh". Daftar per layar ditetapkan di spec.
4. **CSP**: langsung enforce. **TLS diterminasi di Nginx** frontend, jadi HSTS dipasang di Nginx yang sama.
5. **CI**: **GitHub Actions**, satu pipeline bersama BE-02.
6. **E2E**: alur wajib = login internal + vendor, buat & approve Vendor Request, vendor terima/tolak order. Tempat menjalankan (lokal vs CI) diputuskan di spec.
7. **Target bundle**: initial JS ≤ 300 KB gzip untuk halaman login dan dashboard (angka contoh di pertanyaan, disetujui).
8. **FE-09**: ditunda, hanya dicatat.
