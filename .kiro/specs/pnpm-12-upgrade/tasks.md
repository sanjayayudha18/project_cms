# Implementation Plan

> **Status:** draft 2026-11-03. Requirements dan design disetujui 2026-11-03 (D1-D6 ikut saran Kiro).

## Overview

Upgrade pnpm 11.20.0 → 12.16.1 di CompanyPortal-Vite (CP) dan VendorPortal-Vite (VP). Urutan kerja:

1. Catat item X.12 di `development-plan.md` dulu (scope guard rule 1).
2. Ambil baseline dengan pnpm 11.20.0: snapshot Lockfile dan QualityGate kedua portal.
3. Pasang pnpm 12.16.1 global, lalu ganti pin `packageManager`, baris install di Dockerfile, dan pesan `dev.ps1`.
4. Verifikasi Lockfile (Property 3), jalankan ulang QualityGate (Property 4, 5), lalu Docker build (Property 6).
5. Dokumen (Property 7) dan laporan akhir.

Pola rujukan: lessons-learned "pnpm major upgrade + Docker" dan "pnpm 11 Docker builds: must COPY `pnpm-workspace.yaml`". Tidak ada kode aplikasi yang berubah. Jangan menimpa perubahan yang belum di-commit di `development-plan.md`, `navigation.ts`, dan `exports.preview.test.tsx`.

### Model Split

**Sonnet-runnable**
- 1. Catat X.12 + temuan CI di `development-plan.md` — Haiku, Low (Haiku)
- 2. Baseline pnpm 11.20.0 — Sonnet, Medium
- 3. Pasang pnpm 12.16.1 global — Sonnet, Low
- 4. Ganti pin, Dockerfile, `dev.ps1` — Sonnet, Low (needs 2, 3)
- 5. Verifikasi Lockfile dan WorkspaceConfig — Sonnet, Medium (needs 4)
- 6. Checkpoint QualityGate pnpm 12 vs baseline — Sonnet, Medium (needs 2, 5)
- 7. Docker build kedua portal — Sonnet, Medium (needs 5)
- 8. Cek manual `dev.ps1` — Haiku, Low (Haiku) (needs 4)
- 9. Dokumentasi — Haiku, Low (Haiku) (needs 6, 7)
- 10. Checkpoint akhir dan laporan — Sonnet, Medium (needs semua)

**Opus-required**
- none. Tidak menyentuh auth, uang, approval, rekonsiliasi, migrasi data, atau orkestrasi lintas modul.

## Tasks

- [ ] 1. Catat item X.12 dan temuan CI di `development-plan.md`
  - Tambah baris X.12 "pnpm 12 upgrade (`.kiro/specs/pnpm-12-upgrade/`)" di tabel Cross-cutting items: status 🚧, langkah berikutnya "implementasi tasks 2-10".
  - Di baris X.3, tambah temuan: `.github/workflows/ci.yml` tidak punya job frontend (spec `fe-tech-debt` menyebutnya), item terbuka di luar spec ini (D4).
  - Edit dengan `str_replace` pada baris terkait saja. File ini punya perubahan yang belum di-commit, jangan ditimpa.
  - _Requirements: 6.4, 6.6_
  - _Model: Haiku, Effort: Low — dua edit baris tabel, isinya sudah ditentukan design._

- [ ] 2. Baseline dengan pnpm 11.20.0 (sebelum file apa pun diubah)
  - [ ] 2.1 Snapshot kunci `packages:` Lockfile kedua portal
    - Ambil semua kunci `nama@versi` di bagian `packages:` dari `frontend/CompanyPortal-Vite/pnpm-lock.yaml` dan `frontend/VendorPortal-Vite/pnpm-lock.yaml`, simpan ke `$env:TEMP\pnpm12-baseline-cp.txt` dan `$env:TEMP\pnpm12-baseline-vp.txt` (di luar repo).
    - Catat jumlah kunci per portal.
    - _Requirements: 3.2_
  - [ ] 2.2 QualityGate CP baseline
    - Di `frontend/CompanyPortal-Vite`: pastikan `pnpm --version` mencetak 11.20.0, lalu jalankan semua perintah QualityGate (design tabel QualityGate #1-#7).
    - Catat exit code per perintah, jumlah test lulus/gagal, dan empat angka coverage di catatan task.
    - Bila baseline sudah merah, berhenti dan lapor ke user. Upgrade tidak dilanjutkan di atas baseline yang merah.
    - _Requirements: 4.1, 4.3, 4.4, 4.5, 4.6_
  - [ ] 2.3 QualityGate VP baseline
    - Sama dengan 2.2 di `frontend/VendorPortal-Vite` (perintah #1-#6).
    - _Requirements: 4.2, 4.3, 4.4, 4.5_
  - _Model: Sonnet, Effort: Medium — menjalankan gate dan memilah kegagalan baseline butuh penilaian._

- [ ] 3. Pasang pnpm 12.16.1 global di mesin developer
  - Catat `pnpm --version` dan `where.exe pnpm` sebelum install.
  - Jalankan `npm install -g pnpm@12.16.1`. Dari folder di luar portal (misalnya root repo), `pnpm --version` harus mencetak 12.16.1.
  - Bila `where.exe pnpm` menunjukkan salinan pnpm lama yang menutupi pnpm 12, laporkan lokasinya ke user sebelum menghapus apa pun.
  - Catat setiap gejala yang muncul (misalnya `ENOEXEC`) untuk bahan entri lessons-learned di 9.2.
  - _Requirements: 5.2, 5.3_
  - _Model: Sonnet, Effort: Low — satu perintah install, penilaian hanya bila PATH bermasalah._

- [ ] 4. Ganti pin, Dockerfile, dan pesan `dev.ps1`
  - [ ] 4.1 Ganti `packageManager` di kedua `package.json`
    - Hitung hex dari `npm view pnpm@12.16.1 dist.integrity`, bandingkan dengan hex di design (`091d1dee…f656d1`). Bila berbeda, berhenti dan lapor.
    - Tulis `pnpm@12.16.1+sha512.<hex>` di `frontend/CompanyPortal-Vite/package.json` dan `frontend/VendorPortal-Vite/package.json`.
    - _Requirements: 1.1, 1.2, 1.4_
  - [ ] 4.2 Ganti baris install pnpm di kedua Dockerfile
    - Baris 3 `RUN corepack enable && corepack install --global pnpm@11.20.0` → `RUN npm install -g pnpm@12.16.1` di `frontend/CompanyPortal-Vite/Dockerfile` dan `frontend/VendorPortal-Vite/Dockerfile`.
    - Baris COPY, `pnpm install --frozen-lockfile`, perintah build, dan stage serve tidak diubah.
    - _Requirements: 2.1, 2.2, 2.3, 2.6_
  - [ ] 4.3 Ganti pesan petunjuk di `frontend/dev.ps1`
    - Baris 52 → `Write-Error "pnpm not found on PATH. Install it first: npm install -g pnpm@12.16.1"`.
    - _Requirements: 5.1_
  - [ ] 4.4 Cek Property 1 dan Property 2
    - Property 1: kedua nilai `packageManager` sama persis dengan `pnpm@12.16.1+sha512.<hex dari registry>`, tanpa range atau tag.
    - Property 2: `Select-String` untuk `11.20.0` di `frontend/*/package.json`, `frontend/*/Dockerfile`, `frontend/dev.ps1`, dan untuk `corepack` di kedua Dockerfile dan `dev.ps1` → hasil harus kosong.
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 2.1, 5.1_
  - _Model: Sonnet, Effort: Low — edit satu baris per file, nilai sudah ditentukan design._

- [ ] 5. Verifikasi Lockfile dan WorkspaceConfig (Property 3)
  - [ ] 5.1 Install frozen dengan pnpm 12.16.1
    - Di kedua portal: `pnpm --version` harus 12.16.1, lalu `pnpm install --frozen-lockfile`. Install harus lolos tanpa `ERR_PNPM_UNRECOGNIZED_WORKSPACE_SETTINGS` atau `ERR_PNPM_IGNORED_BUILDS`.
    - `git diff --exit-code -- frontend/CompanyPortal-Vite/pnpm-lock.yaml frontend/VendorPortal-Vite/pnpm-lock.yaml` harus bersih.
    - `pnpm-workspace.yaml` tidak diubah dan masih berisi `allowBuilds` untuk `@biomejs/biome` dan `esbuild`.
    - _Requirements: 3.1, 3.5_
  - [ ] 5.2 Install tanpa frozen dan bandingkan dengan snapshot
    - Di kedua portal: `pnpm install` (tanpa frozen). Bila Lockfile berubah, ambil ulang kunci `packages:` dan `Compare-Object` dengan snapshot 2.1.
    - Daftar sama, diff hanya format/peer variant → biarkan diff untuk di-commit bersama upgrade (D3) dan catat ringkasannya.
    - Ada `nama@versi` yang bertambah, hilang, atau berubah → `git checkout -- <pnpm-lock.yaml>`, berhenti, dan lapor ke user.
    - _Requirements: 3.2, 3.3, 3.4_
  - [ ] 5.3 Cek build script dependency jalan
    - Di kedua portal: `pnpm exec biome --version` dan `pnpm exec esbuild --version` harus mencetak versi tanpa error.
    - _Requirements: 3.6_
  - _Model: Sonnet, Effort: Medium — perlu memilah diff Lockfile antara format dan perubahan versi._

- [ ] 6. Checkpoint: QualityGate pnpm 12.16.1 vs baseline (Property 4, Property 5)
  - [ ] 6.1 QualityGate CP
    - Jalankan perintah #1-#7 di `frontend/CompanyPortal-Vite`. Setiap perintah exit 0.
    - Bandingkan dengan baseline 2.2: jumlah test lulus sama, 0 gagal, coverage tidak di bawah 84/80/80/85, `pnpm audit --prod` 0 vulnerability.
    - Property 5: setelah `gen:api` + `gen:api:exq`, `git diff --exit-code -- frontend/CompanyPortal-Vite/src/lib/api/` bersih.
    - Jangan ubah test atau threshold untuk meloloskan gate. Bila hasil berbeda dari baseline, ikuti tabel Error Handling di design dan lapor.
    - _Requirements: 4.1, 4.3, 4.4, 4.5, 4.6_
  - [ ] 6.2 QualityGate VP
    - Jalankan perintah #1-#6 di `frontend/VendorPortal-Vite`, bandingkan dengan baseline 2.3 (threshold 93/84/90/93).
    - _Requirements: 4.2, 4.3, 4.4, 4.5_
  - _Model: Sonnet, Effort: Medium — memilah kegagalan gate dan membandingkan dengan baseline._

- [ ] 7. Docker build kedua portal (Property 6)
  - [ ] 7.1 Docker build CP
    - `docker build -t userportal-vite-fe:pnpm12 frontend/CompanyPortal-Vite` selesai tanpa error.
    - `docker build --target builder -t cp-builder:pnpm12 frontend/CompanyPortal-Vite`, lalu `docker run --rm cp-builder:pnpm12 pnpm --version` → `12.16.1`.
    - `docker run -d` image hasil build, tunggu, lalu `docker inspect --format '{{.State.Health.Status}}'` → `healthy`. Hentikan dan hapus container setelahnya.
    - _Requirements: 2.4, 2.5, 2.7_
  - [ ] 7.2 Docker build VP
    - Sama dengan 7.1 untuk `frontend/VendorPortal-Vite` (image `vendorportal-vite-fe:pnpm12`, builder `vp-builder:pnpm12`).
    - _Requirements: 2.4, 2.5, 2.7_
  - [ ] 7.3 Cek stage serve tidak berubah
    - `git diff -- frontend/CompanyPortal-Vite/Dockerfile frontend/VendorPortal-Vite/Dockerfile` hanya menunjukkan baris 3.
    - _Requirements: 2.2, 2.3, 2.6_
  - _Model: Sonnet, Effort: Medium — Docker build bisa gagal karena binary platform; perlu triase log._

- [ ] 8. Cek manual `dev.ps1` dengan pnpm 12.16.1
  - Jalankan `./dev.ps1 company` lalu `./dev.ps1 vendor` dari `frontend/` (user, atau agen sebagai proses background yang dihentikan setelah cek). Vite harus start dan menampilkan URL lokal tanpa error dari pnpm.
  - _Requirements: 5.4_
  - _Model: Haiku, Effort: Low — cek start dev server, tidak ada keputusan._

- [ ] 9. Dokumentasi (Property 7)
  - [ ] 9.1 Update `.claude/docs/arsitektur-teknologi.md`
    - Baris 74: `pnpm | 11.20.0` → `pnpm | 12.16.1`.
    - _Requirements: 6.1_
  - [ ] 9.2 Entri lessons-learned di dua file
    - Tambah entri "pnpm 11 → 12 upgrade" (isi sesuai design, plus gejala nyata dari task 3 dan 5) di `.kiro/steering/lessons-learned.md` dan `.claude/lessons-learned.md`, teks sama persis.
    - Beri catatan status pada tiga entri pnpm lama sesuai tabel design, termasuk koreksi `lockfileVersion` (pnpm 11 di repo ini menulis `9.0`, bukan `10.0`).
    - _Requirements: 5.2, 5.3, 6.2, 6.3_
  - [ ] 9.3 Changelog dan status X.12
    - Tambah entri di `.claude/development-progress.md`: upgrade pnpm 12.16.1, hasil gate, ringkasan diff Lockfile (atau "tidak ada diff"), dan satu baris untuk developer lain: pasang ulang pnpm global dengan `npm install -g pnpm@12.16.1` (D6).
    - Update baris X.12 di `development-plan.md`: status ✅ built, langkah berikutnya commit.
    - _Requirements: 6.4, 6.5_
  - [ ] 9.4 Cek Property 7
    - Semua dokumen di tabel "Dokumen yang diubah" menyebut pnpm 12.16.1. `Compare-Object` teks entri pnpm 12 di dua file lessons-learned → kosong.
    - _Requirements: 5.2, 5.3, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_
  - _Model: Haiku, Effort: Low — edit dokumen dengan isi yang sudah ditentukan design._

- [ ] 10. Checkpoint akhir dan laporan ke user
  - Ulangi cek Property 1-7 secara ringkas dan kumpulkan hasilnya.
  - Laporkan ke user: tabel baseline vs pnpm 12 (exit code, jumlah test, coverage, audit) per portal, status Docker build, diff Lockfile, dan daftar file yang berubah.
  - Commit hanya setelah user OK, dengan conventional commit (misalnya `build(frontend): upgrade pnpm 11.20.0 -> 12.16.1`). Stage per nama file: file di tabel "Files touched" design plus folder `.kiro/specs/pnpm-12-upgrade/`. Perubahan lain yang belum di-commit (`navigation.ts`, `exports.preview.test.tsx`) tidak ikut. `development-plan.md` sudah punya perubahan lain sebelum spec ini; tanyakan ke user apakah file itu di-commit utuh atau hanya hunk X.12/X.3.
  - _Requirements: 1.3, 2.4, 3.2, 4.3, 6.5_
  - _Model: Sonnet, Effort: Medium — merangkum hasil dan memastikan semua property terpenuhi sebelum commit._

## Notes

### Definition of Done

- Property 1-7 terverifikasi dan hasilnya tercatat di catatan task.
- QualityGate kedua portal hijau dengan pnpm 12.16.1 dan setara baseline.
- Docker build kedua portal berhasil, builder mencetak `12.16.1`, container healthy.
- Lockfile tanpa perubahan versi resolved.
- Dokumen diperbarui, X.12 tercatat.

### Catatan

- CI GitHub Actions tidak menguji frontend (D4). Task 6 dan 7 wajib dijalankan lokal sebelum commit.
- Task 2 harus selesai sebelum task 3 dan 4 supaya baseline benar-benar pnpm 11.20.0. Setelah pnpm 12 global terpasang dan pin masih 11.20.0, pnpm 12 akan pindah ke 11.20.0. Itu tetap benar untuk baseline, tapi urutan di atas menghindari kebingungan.
- Snapshot 2.1 disimpan di `$env:TEMP`, bukan di repo.
- Rollback: revert commit upgrade. pnpm 12 global otomatis pindah ke versi pin 11.20.0.
- `.opencode/opencode.md` dan spec di `.done/` yang menyebut pnpm 11 tidak diubah (di luar Req 6).

## Task Dependency Graph

- **Wave 0:** 1 (catat X.12, scope guard) dan 2.1 (snapshot Lockfile). Keduanya tanpa dependensi.
- **Wave 1:** 2.2 dan 2.3 (baseline gate CP dan VP, folder berbeda).
- **Wave 2:** 3 (pnpm 12 global), setelah baseline selesai.
- **Wave 3:** 4.1, 4.2, 4.3 (file berbeda), setelah 3.
- **Wave 4:** 4.4 (cek Property 1-2), setelah 4.1-4.3.
- **Wave 5:** 5.1 (install frozen), setelah 4.4.
- **Wave 6:** 5.2 (install tanpa frozen, menyentuh Lockfile yang sama dengan 5.1).
- **Wave 7:** 5.3 (cek biome/esbuild), setelah install selesai.
- **Wave 8:** 6.1 dan 6.2 (gate pnpm 12), setelah 5.3 dan baseline 2.2/2.3.
- **Wave 9:** 7.1 dan 7.2 (Docker build), setelah Lockfile final di 5.2.
- **Wave 10:** 7.3 (diff Dockerfile) dan 8 (cek `dev.ps1`).
- **Wave 11:** 9.1, 9.2, 9.3 (file berbeda), setelah gate dan Docker selesai. 9.3 menyentuh `development-plan.md` yang sama dengan task 1, sudah di wave berbeda.
- **Wave 12:** 9.4 (cek Property 7), setelah 9.1-9.3.
- **Wave 13:** 10 (checkpoint akhir), setelah semua task.

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "2.1"] },
    { "id": 1, "tasks": ["2.2", "2.3"] },
    { "id": 2, "tasks": ["3"] },
    { "id": 3, "tasks": ["4.1", "4.2", "4.3"] },
    { "id": 4, "tasks": ["4.4"] },
    { "id": 5, "tasks": ["5.1"] },
    { "id": 6, "tasks": ["5.2"] },
    { "id": 7, "tasks": ["5.3"] },
    { "id": 8, "tasks": ["6.1", "6.2"] },
    { "id": 9, "tasks": ["7.1", "7.2"] },
    { "id": 10, "tasks": ["7.3", "8"] },
    { "id": 11, "tasks": ["9.1", "9.2", "9.3"] },
    { "id": 12, "tasks": ["9.4"] },
    { "id": 13, "tasks": ["10"] }
  ]
}
```
