# Implementation Plan

## Overview

Urutan pengerjaan: skrip pemeriksa image dan test fixture-nya dulu, karena job CI memanggil keduanya. Setelah itu job `go-image` di `ci.yml`. Perbaikan dokumen berjalan paralel karena file-nya terpisah. Dua task di `deployment.md` diserialkan karena menyentuh file yang sama. Checkpoint menjalankan pemeriksaan grep (Property 5, 6) dan memastikan hanya file di daftar "Files touched" yang berubah. Task terakhir adalah commit dan run GitHub Actions pertama.

Pola yang diikuti: job `python-image` (build saja), `backend/scripts/check-coverage.sh` (skrip bash dipanggil dengan `bash <path>`), dan gaya bahasa Inggris yang sudah ada di `deployment.md`. Tidak ada perubahan kode aplikasi, migrasi, atau dependency.

### Model Split

**Sonnet-runnable**
- 1. Skrip `check-image.sh` + fixture test + cek image nyata — Sonnet, Medium
- 2. Job CI `go-image` + review YAML — Sonnet, Low (needs 1.1)
- 3. `deployment.md`: perbaiki pernyataan usang — Sonnet, Low
- 4. `deployment.md`: checklist rilis deploy pertama `backend-exq` — Sonnet, Medium (needs 3)
- 5. `backend-exq/.env.example`: komentar `REDIS_URL` — Haiku, Low (Haiku)
- 6. `development-plan.md` + `development-progress.md` — Haiku, Low (Haiku) (needs 2.1, 4)
- 7. Checkpoint: grep dokumen + cakupan diff — Sonnet, Medium (needs 1–6)
- 8. Commit + run GitHub Actions pertama — Haiku, Low (Haiku) (needs 7)

**Opus-required**
- none

## Tasks

- [x] 1. Skrip pemeriksa image dan test-nya
  - [x] 1.1 Tulis `scripts/ci/check-image.sh`
    - Isi sesuai design "`scripts/ci/check-image.sh` (baru)": argumen `<image> <module>`, exit 2 bila argumen salah, `docker image inspect` untuk `Config.User` dan `Config.Healthcheck.Test`, bagian user sebelum `:` yang kosong / `root` / `0` → `::error::` + exit 1, healthcheck kosong / `NONE` → `::error::` + exit 1, selain itu cetak user + healthcheck dan exit 0.
    - `set -euo pipefail`; file disimpan LF (dijamin `.gitattributes`).
    - _Requirements: 2.1, 2.2, 2.3_
  - [x] 1.2 Tulis `scripts/ci/check-image.test.sh` (Property 1, 2, 3)
    - Sepuluh fixture dari tabel design, dibuild lewat stdin (`docker build -t <tag> -`) dari `alpine:3.24`; bandingkan exit code dengan harapan; untuk fixture gagal, cek output memuat nama modul fixture, dan untuk fixture root juga nilai user.
    - `trap` menghapus image fixture; exit 1 dengan daftar fixture yang tidak sesuai.
    - Jalankan lokal (Git Bash atau WSL dengan Docker) sampai lulus.
    - _Requirements: 2.2, 2.3, 2.4_
  - [x] 1.3 Cek image nyata (Property 3)
    - Build lokal `docker build -f backend/Dockerfile -t cms-backend:local-check .` dan `docker build -f backend-exq/Dockerfile -t cms-backend-exq:local-check .` dari root repo, lalu `bash scripts/ci/check-image.sh cms-<modul>:local-check <modul>` untuk keduanya: exit 0 tanpa mengubah Dockerfile.
    - Catat output kedua modul di laporan task.
    - _Requirements: 2.4_
  - _Model: Sonnet, Effort: Medium — logika skrip kecil, tapi kasus tepi `user:grup`, `HEALTHCHECK NONE`, dan quoting template `docker` perlu dicek._

- [x] 2. Job CI `go-image`
  - [x] 2.1 Tambah job `go-image` di `.github/workflows/ci.yml`
    - Setelah job `python-image`, sesuai YAML di design: matrix `[backend, backend-exq]`, `fail-fast: false`, checkout, fixture test hanya pada leg `backend`, `docker build -f "<modul>/Dockerfile" -t "cms-<modul>:ci" .`, lalu `bash scripts/ci/check-image.sh`.
    - Tanpa `docker push`, `docker login`, `secrets.*`, atau blok `permissions` di job.
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.1_
  - [x] 2.2 Review YAML job (Property 4)
    - Grep blok job `go-image`: tidak ada `push`, `login`, `secrets.`, `permissions:`; ada `fail-fast: false`; konteks build `.` dengan `-f <modul>/Dockerfile`.
    - Pastikan job lain di `ci.yml` tidak berubah (`git diff` hanya menambah blok baru).
    - _Requirements: 1.2, 1.3, 1.4, 1.5_
  - _Model: Sonnet, Effort: Low — satu blok YAML mengikuti pola `python-image`._

- [x] 3. `deployment.md`: perbaiki pernyataan usang tentang `backend-exq`
  - Terapkan setiap baris tabel "Dokumen: DeploymentDoc" di design: tabel produk #2, paragraf "Current state", kalimat CI dan CD di "Pipeline" (sebut `go-image`, tidak ada job frontend/E2E, belum ada `cloudbuild.yaml`), bullet `backend-exq` di "IP client & proxy tepercaya", bullet primary/replica dan tabel pool.
  - Paragraf "Planned (2028)" (Redis sama, `JWT_SECRET` sama, firewall) tidak diubah.
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_
  - _Model: Sonnet, Effort: Low — teks pengganti sudah ditulis di design; tugasnya menerapkan tanpa merusak teks di sekitarnya._

- [x] 4. `deployment.md`: checklist rilis deploy pertama `backend-exq`
  - Tambah bagian "Release checklist: first `backend-exq` deployment" tepat sebelum "Release checklist: report export (X.8 …)", memakai draft di design: pekerjaan infra, tabel `.env` (semua variabel `backend-exq/.env.example`, termasuk nilai `TRUSTED_PROXIES` dari subnet `cms-net`), urutan (migrasi `034`/`035` → Redis + `cms-net` → `backend` → `backend-exq` → CompanyPortal), rilis bersama (D15), verifikasi (`/health`, `/ready`, ekspor uji + IP di log dan Log Audit), rollback.
  - Cocokkan batas `EXPORT_*` dengan `backend-exq/internal/exportjob/config.go` sebelum menulis.
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_
  - _Model: Sonnet, Effort: Medium — isi konfigurasi harus cocok dengan kode (batas env, alamat proxy, urutan rilis); salah satu nilai bisa membuat deploy pertama gagal atau audit mencatat IP salah._

- [x] 5. `backend-exq/.env.example`: komentar `REDIS_URL`
  - Ganti "(see root docker-compose.yml)" dengan "(Redis + cms-net are created by backend/docker-compose.yaml)". Nama variabel, nilai default, dan urutan tidak berubah.
  - _Requirements: 5.1, 5.2_
  - _Model: Haiku, Effort: Low — satu baris komentar._

- [x] 6. `development-plan.md` dan `development-progress.md`
  - `development-plan.md`: hapus known gap "`backend-exq` image still runs as root without HEALTHCHECK"; kolom Next X.3 dan X.7 sesuai design; baris cross-cutting baru untuk spec ini. Kalimat "`.github/workflows/ci.yml` belum punya job frontend" tetap ada (D1).
  - `.claude/development-progress.md`: satu entri untuk spec ini.
  - _Requirements: 6.1, 6.2, 6.3_
  - _Model: Haiku, Effort: Low — pembaruan dokumen mekanis._

- [x] 7. Checkpoint: grep dokumen dan cakupan diff
  - Property 5: baca semua nama variabel dari `backend-exq/.env.example` (baris aktif dan baris `# NAME=`), pastikan masing-masing muncul di bagian checklist rilis `deployment.md`.
  - Property 6: frasa usang dari tabel requirements tidak ditemukan lagi di file tujuannya ("only empty `doc.go`", "not deployed until it has content", "does not use it yet", "uses the primary only", "frontend + E2E jobs added", "root docker-compose.yml", "still runs as root without HEALTHCHECK").
  - `git diff --stat` hanya memuat file di "Files touched" (design) ditambah file spec ini.
  - Ulangi `bash scripts/ci/check-image.test.sh` sekali setelah semua edit.
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.6, 4.1, 5.1, 5.2, 6.1_
  - _Model: Sonnet, Effort: Medium — triage bila grep menemukan sisa teks usang atau diff memuat file tak terduga._

- [ ] 8. Commit dan run GitHub Actions pertama
  - Agent menyiapkan commit (conventional commit, file disebut satu per satu, tanpa `--no-verify`) setelah user setuju.
  - User push ke `dev1` dan memeriksa tab Actions: kedua leg `go-image (backend)` dan `go-image (backend-exq)` hijau, step fixture test hijau, job lain tidak berubah statusnya. `gh` CLI tidak terpasang, jadi agent tidak bisa memeriksanya.
  - _Requirements: 1.1, 2.4_
  - _Model: Haiku, Effort: Low — commit mekanis; verifikasi run dilakukan user._

## Notes

- Spec ini tanpa perubahan kode aplikasi (Go, Python, frontend), tanpa migrasi, dan tanpa dependency baru. Suite test aplikasi tidak perlu dijalankan ulang.
- Jangan menjalankan template `docker image inspect --format` dengan `join … " "` langsung dari PowerShell 5.1; tanda kutipnya rusak. Jalankan lewat bash (design, Testing Strategy).
- `bash` di `PATH` mesin dev adalah WSL (`C:\Windows\system32\bash.exe`). Pastikan perintah `docker` tersedia di shell itu, atau pakai Git Bash.
- Pekerjaan infra di GCP (Cloud Build, Secret Manager, VM, firewall port 8081) tetap dikerjakan user dan hanya dicatat di checklist rilis (D4).
- Job CI frontend tetap item terbuka di X.3 (D1).

### Definition of Done

- Task 1–7 selesai; fixture test dan pemeriksaan image nyata lulus lokal.
- Property 1–6 masing-masing punya pemeriksaan yang lulus (1.2, 1.3, 2.2, 7).
- Task 8: run GitHub Actions pertama hijau untuk kedua leg `go-image`, dikonfirmasi user.

## Task Dependency Graph

- **Wave 0:** 1.1 (skrip pemeriksa), 3 (`deployment.md` koreksi), 5 (`.env.example`). File berbeda, tanpa dependensi.
- **Wave 1:** 1.2 dan 1.3 (butuh 1.1), 2.1 (memanggil skrip dari 1.1), 4 (file sama dengan 3, jadi sesudah 3).
- **Wave 2:** 2.2 (review YAML dari 2.1), 6 (merujuk `go-image` dari 2.1 dan checklist dari 4).
- **Wave 3:** 7 (checkpoint, butuh semua task 1–6).
- **Wave 4:** 8 (commit dan run CI, butuh 7).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "3", "5"] },
    { "id": 1, "tasks": ["1.2", "1.3", "2.1", "4"] },
    { "id": 2, "tasks": ["2.2", "6"] },
    { "id": 3, "tasks": ["7"] },
    { "id": 4, "tasks": ["8"] }
  ]
}
```
