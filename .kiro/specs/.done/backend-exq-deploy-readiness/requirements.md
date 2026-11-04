# Requirements Document

> Status: **requirements disetujui PO 2026-11-04** (D1–D7 mengikuti saran Kiro). Design: `design.md`.
> Tambahan setelah persetujuan, ditemukan saat menulis design (2026-11-04): definisi Root_User mencakup bentuk `user:grup`, Req 2.3 menghitung `HEALTHCHECK NONE` sebagai tidak ada, dan Req 3.6 (paragraf "Pipeline" mengklaim job frontend + E2E sudah ada, padahal belum).
> Item rencana: X.3 (tech debt BE) dan prasyarat rilis X.7/X.8/X.9 di `.kiro/steering/development-plan.md`.

## Introduction

Fitur ekspor (X.7 `vendor-data-export`, X.8 `report-export-migration`, X.9 `export-menu-revamp`) berjalan di `backend-exq` (:8081). Modul itu belum pernah di-deploy. Spec ini menyiapkan deploy pertamanya: CI harus menangkap Dockerfile yang rusak, dan dokumen deploy harus sesuai dengan kode.

### Kondisi kode saat ini (dicek 2026-11-04)

- `backend-exq/Dockerfile` sudah multi-stage, berjalan sebagai `appuser` (non-root), dan punya `HEALTHCHECK` ke `/health`. Pengerasan ini masuk di `540ee96` (vendor-data-export). `docker build -f backend-exq/Dockerfile .` dari root repo berhasil secara lokal, dan `docker image inspect` menunjukkan `User=appuser` dengan HEALTHCHECK `wget … http://localhost:8081/health`.
- `backend/Dockerfile` punya pola yang sama (non-root + HEALTHCHECK :8080).
- `.github/workflows/ci.yml` berisi job `go`, `go-integration`, `sqlc`, `python`, dan `python-image`. Image yang dibuild baru `backend_python`. Belum ada job yang membuild `backend/Dockerfile` atau `backend-exq/Dockerfile`, jadi Dockerfile Go yang rusak baru ketahuan saat deploy.
- `backend-exq/cmd/api/main.go` sudah memakai `TrustedRealIP(cfg.TrustedProxies)` dan pool replica (`DATABASE_REPLICA_URL`; worker dan previewer membaca dari replica, sedangkan scope dan job dari primary).
- `frontend/CompanyPortal-Vite/nginx.conf` meneruskan `/api/v1/exports/` ke `backend-exq:8081` di network `cms-net` dengan `proxy_buffering off`. `vite.config.ts` meneruskan `/api/v1/exports` ke `localhost:8081`.
- `backend-exq/docker-compose.yaml` memakai network eksternal `cms-net` yang dibuat `backend/docker-compose.yaml`.
- Tidak ada file konfigurasi Cloud Build (`cloudbuild*.yaml`) di repo.

### Dokumen yang tidak sesuai dengan kode

| File | Pernyataan | Kenyataan |
|---|---|---|
| `.claude/docs/deployment.md` tabel produk #2 dan paragraf "Current state" | `backend-exq` hanya berisi `doc.go` kosong dan belum di-deploy sampai ada isi | Sudah berisi API ekspor, worker, dan janitor; deploy pertama ikut rilis ekspor |
| `deployment.md` paragraf "Pipeline" | CD hanya membuild `backend/Dockerfile`; `backend-exq/Dockerfile` ditambahkan nanti | Image `backend-exq` dibutuhkan untuk rilis ekspor |
| `deployment.md` bagian "IP client & proxy tepercaya" | `backend-exq` membaca `TRUSTED_PROXIES` tetapi belum memakainya | `TrustedRealIP` sudah dipasang; IP di audit ekspor bergantung padanya |
| `deployment.md` bagian "Ukuran pool DB" | Hanya `backend` yang memakai pool replica; `backend-exq` hanya primary | `backend-exq` juga membuat pool replica bila `DATABASE_REPLICA_URL` diisi |
| `backend-exq/.env.example` | Komentar `REDIS_URL` merujuk "root docker-compose.yml" | File itu tidak ada; Redis dan `cms-net` dibuat oleh `backend/docker-compose.yaml` |
| `.kiro/steering/development-plan.md` known gaps | "`backend-exq` image still runs as root without HEALTHCHECK" | Sudah non-root dengan HEALTHCHECK sejak `540ee96` |

### Ruang lingkup

- Job CI baru yang membuild image `backend` dan `backend-exq`, lalu memeriksa bahwa image berjalan sebagai non-root dan punya HEALTHCHECK.
- Perbaikan pernyataan usang tentang `backend-exq` di `deployment.md`, `backend-exq/.env.example`, dan `development-plan.md`.
- Checklist rilis untuk deploy pertama `backend-exq` di `deployment.md`.

### Tidak termasuk

- Perubahan kode aplikasi Go, Python, atau frontend.
- Membuat VM, Secret Manager, Artifact Registry, atau konfigurasi Cloud Build di GCP. Itu pekerjaan infra oleh user.
- Push image dari GitHub Actions ke registry mana pun (D2).
- Job CI frontend (lint, test, build CP dan VP) (D1).
- Menjalankan container di CI (D5).
- Merge `dev1` → `main` (X.6).

### Area sensitif

Tidak ada perubahan logika uang, rekonsiliasi, maker-checker, atau migrasi data. Checklist rilis menyebut konfigurasi auth (`JWT_SECRET` harus sama dengan `backend`) dan IP audit (`TRUSTED_PROXIES`). Ini hanya dokumentasi konfigurasi, bukan perubahan kode auth.

## Glossary

- **CI_Workflow**: `.github/workflows/ci.yml`, CI aktif di GitHub Actions.
- **GoImageJob**: job baru di CI_Workflow yang membuild image Docker untuk modul Go `backend` dan `backend-exq`.
- **Image_Check**: langkah di GoImageJob yang membaca `docker image inspect` dan memeriksa `Config.User` serta `Config.Healthcheck`.
- **DeploymentDoc**: `.claude/docs/deployment.md`.
- **EXQ_ReleaseChecklist**: bagian baru di DeploymentDoc untuk deploy pertama `backend-exq`.
- **EXQ_EnvExample**: `backend-exq/.env.example`.
- **DevelopmentPlan**: `.kiro/steering/development-plan.md`.
- **Root_User**: nilai `Config.User` pada image yang membuat proses berjalan sebagai root: kosong, `root`, `0`, atau bentuk `user:grup` yang bagian user-nya `root` atau `0` (mis. `root:root`, `0:0`).

## Requirements

### Requirement 1: CI membuild image Go

**User Story:** Sebagai developer, saya ingin CI membuild image `backend` dan `backend-exq` di setiap push dan PR, supaya Dockerfile yang rusak ketahuan sebelum deploy.

#### Acceptance Criteria

1. WHEN ada push atau pull request ke `main` atau `dev1`, THE CI_Workflow SHALL menjalankan GoImageJob untuk modul `backend` dan `backend-exq`.
2. THE GoImageJob SHALL membuild setiap image dari root repo dengan Dockerfile modul masing-masing (`-f <modul>/Dockerfile .`), sama seperti `backend/docker-compose.yaml` dan `backend-exq/docker-compose.yaml` (`context: ..`).
3. IF build salah satu image gagal, THEN THE GoImageJob SHALL gagal untuk modul itu, sementara build modul lain tetap berjalan sampai selesai.
4. THE GoImageJob SHALL tidak mem-push image ke registry mana pun dan tidak memakai secret repository.
5. THE GoImageJob SHALL hanya memakai izin `contents: read`, sama seperti workflow saat ini.

### Requirement 2: CI menjaga image tetap non-root dengan HEALTHCHECK

**User Story:** Sebagai developer, saya ingin CI gagal kalau image Go berjalan sebagai root atau kehilangan HEALTHCHECK, supaya pengerasan image tidak hilang diam-diam.

#### Acceptance Criteria

1. WHEN image selesai dibuild, THE Image_Check SHALL membaca `Config.User` dan `Config.Healthcheck` dari `docker image inspect`.
2. IF `Config.User` adalah Root_User, THEN THE Image_Check SHALL menggagalkan job dengan pesan error yang menyebut nama modul dan nilai user.
3. IF `Config.Healthcheck` tidak ada, test-nya kosong, atau test-nya `NONE` (dari `HEALTHCHECK NONE`), THEN THE Image_Check SHALL menggagalkan job dengan pesan error yang menyebut nama modul.
4. WHEN image `backend` dan `backend-exq` saat ini diperiksa, THE Image_Check SHALL lulus tanpa perubahan pada kedua Dockerfile.

### Requirement 3: Dokumen deploy sesuai kode

**User Story:** Sebagai operator yang men-deploy, saya ingin `deployment.md` menggambarkan `backend-exq` apa adanya, supaya saya tidak salah mengatur VM, pool, atau proxy.

#### Acceptance Criteria

1. THE DeploymentDoc SHALL menyatakan bahwa `backend-exq` berisi API ekspor, worker, dan janitor, dan bahwa deploy pertamanya ikut rilis ekspor (X.7–X.9).
2. THE DeploymentDoc SHALL menyebut `backend-exq/Dockerfile` di paragraf "Pipeline" sebagai image yang dibuild CD bersama `backend/Dockerfile`, dan menyebut GoImageJob di bagian CI.
3. THE DeploymentDoc SHALL menyatakan bahwa `backend-exq` memakai `TRUSTED_PROXIES` lewat `TrustedRealIP`, dan bahwa IP di audit ekspor (`export_requested`, `master_data_exported`, `report_exported`) bergantung pada nilai itu.
4. THE DeploymentDoc SHALL menyatakan bahwa `backend-exq` membuat pool replica bila `DATABASE_REPLICA_URL` diisi, lalu memperbarui tabel dan rumus ukuran pool agar menghitung `backend-exq` di primary dan replica.
5. THE DeploymentDoc SHALL tetap memuat rencana pemisahan VM 2028, aturan Redis bersama, dan aturan `JWT_SECRET` yang sama di kedua backend.
6. THE DeploymentDoc SHALL menyebut job CI sesuai isi CI_Workflow dan menyatakan bahwa job frontend dan E2E belum ada, menggantikan klaim bahwa job itu sudah ditambahkan.

### Requirement 4: Checklist rilis deploy pertama backend-exq

**User Story:** Sebagai operator, saya ingin satu checklist rilis untuk deploy pertama `backend-exq`, supaya fitur ekspor bisa dirilis tanpa menebak urutan dan konfigurasi.

#### Acceptance Criteria

1. THE EXQ_ReleaseChecklist SHALL mencantumkan setiap variabel di EXQ_EnvExample beserta aturan nilai produksinya: `JWT_SECRET` sama dengan `backend`, `TRUSTED_PROXIES` berisi alamat Nginx CompanyPortal, `DATABASE_URL` ke primary, `DATABASE_REPLICA_URL` ke replica, `REDIS_URL` ke Redis yang sama dengan `backend`, `DSR_LATE_START_DATE` sama di tiga runtime, serta batas `EXPORT_WORKERS`, `EXPORT_POLL_INTERVAL`, dan `EXPORT_JANITOR_INTERVAL`.
2. THE EXQ_ReleaseChecklist SHALL menyatakan bahwa migrasi `034_export_jobs` dan `035_export_jobs_reports` dijalankan sebelum image baru start, dengan merujuk prosedur migrasi yang sudah ada di DeploymentDoc.
3. THE EXQ_ReleaseChecklist SHALL menetapkan urutan start: Redis dan network `cms-net`, lalu `backend`, lalu `backend-exq`, lalu frontend.
4. THE EXQ_ReleaseChecklist SHALL mencantumkan langkah verifikasi setelah start: `/health` 200, `/ready` 200 dengan semua check `ok`, dan satu ekspor uji lewat Nginx CompanyPortal. Untuk ekspor uji itu, log `http_request` di `backend-exq` dan Log Audit harus menunjukkan IP pengguna, bukan alamat Nginx. (Berbeda dengan `backend`, `backend-exq` tidak mencetak log `trusted proxies configured` saat start; lihat D7.)
5. THE EXQ_ReleaseChecklist SHALL menyatakan bahwa CompanyPortal (rute ekspor baru), `backend` (endpoint ekspor lama dihapus), dan `backend-exq` dirilis bersama, sesuai D15 di `vendor-data-export`.
6. THE EXQ_ReleaseChecklist SHALL menjelaskan rollback per image: deploy ulang image `backend`, `backend-exq`, dan CompanyPortal sebelumnya; migrasi `034`/`035` tidak di-rollback.
7. THE EXQ_ReleaseChecklist SHALL mencantumkan pekerjaan infra yang berada di luar repo: menambahkan `backend-exq` ke Cloud Build, membuat secret di Secret Manager, dan menutup port 8081 dari luar VM.

### Requirement 5: Komentar .env.example yang benar

**User Story:** Sebagai developer, saya ingin komentar di `backend-exq/.env.example` merujuk file yang benar-benar ada, supaya setup lokal tidak membingungkan.

#### Acceptance Criteria

1. THE EXQ_EnvExample SHALL merujuk `backend/docker-compose.yaml` sebagai pembuat container Redis dan network `cms-net`, menggantikan rujukan ke "root docker-compose.yml".
2. THE EXQ_EnvExample SHALL tidak mengubah nama variabel, nilai default, atau urutan variabel yang sudah ada.

### Requirement 6: Development plan sesuai kondisi

**User Story:** Sebagai PO, saya ingin `development-plan.md` tidak lagi mencatat gap yang sudah tertutup, supaya prioritas berikutnya diambil dari data yang benar.

#### Acceptance Criteria

1. THE DevelopmentPlan SHALL tidak lagi mencantumkan "`backend-exq` image still runs as root without HEALTHCHECK" di bagian known gaps.
2. THE DevelopmentPlan SHALL memperbarui kolom Next pada X.3 dan X.7 untuk menyebut spec ini, GoImageJob, dan EXQ_ReleaseChecklist.
3. THE DevelopmentPlan SHALL tetap mencatat job CI frontend sebagai item terbuka bila D1 diputuskan di luar scope.

## Open Questions

Semua keputusan di bawah disetujui PO pada 2026-11-04 sesuai saran Kiro.

| ID | Pertanyaan | Saran Kiro | Status |
|---|---|---|---|
| D1 | Apakah job CI frontend (pnpm 12, lint, `test:coverage`, build CP dan VP) ikut spec ini? | Tidak. Job itu tidak terkait deploy `backend-exq`, ukurannya lebih besar, dan butuh keputusan cache serta waktu run. Dikerjakan di spec terpisah. | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D2 | Apakah GoImageJob mem-push image ke Artifact Registry? | Tidak. CD tetap di Cloud Build (`deployment.md`). Push dari GitHub Actions butuh kredensial GCP di repo. | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D3 | Apakah CI memeriksa non-root dan HEALTHCHECK, atau cukup build? | Periksa keduanya lewat `docker image inspect`. Biayanya satu langkah shell, dan pemeriksaan ini mencegah pengerasan BE-06 hilang tanpa ketahuan. | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D4 | Apakah spec ini membuat `cloudbuild.yaml`? | Tidak. Belum ada konfigurasi Cloud Build di repo, dan isinya bergantung pada project GCP yang belum disiapkan. Checklist rilis mencatatnya sebagai pekerjaan infra. | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D5 | Apakah CI menjalankan container untuk smoke test `/health`? | Tidak. `backend-exq` butuh Postgres dan Redis saat start. Menyiapkan keduanya untuk smoke test memperbesar job, padahal integrasi DB sudah diuji di `go-integration`. | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D6 | Apakah image frontend (CP dan VP) ikut dibuild di GoImageJob? | Tidak. Image frontend masuk ke job CI frontend (D1). | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D7 | `backend` mencetak `trusted proxies configured count=N` saat start, `backend-exq` tidak. Apakah log yang sama ditambahkan ke `backend-exq`? | Tidak di spec ini, karena spec ini tanpa perubahan kode aplikasi. Verifikasi memakai `remote_ip` di log request dan IP di Log Audit (Req 4.4). Bisa ditambahkan nanti sebagai bugfix satu baris bila PO mau. | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
