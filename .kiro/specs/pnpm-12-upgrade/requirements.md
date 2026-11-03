# Requirements Document

> **Status:** requirements disetujui 2026-11-03. PO mengikuti saran Kiro untuk D1-D6 (bagian Open Questions).

## Introduction

### Masalah

Saat menjalankan pnpm di folder frontend muncul notifikasi "Update available! 11.20.0 → 12.16.1" (changelog: <https://pnpm.io/v/12.16.1>). Kedua portal masih memakai pnpm 11.20.0. User meminta spec untuk naik ke pnpm 12.16.1.

pnpm 12 adalah major release: inti pnpm ditulis ulang dalam Rust dan dikirim sebagai binary native. Menurut [What's different in pnpm 12](https://pnpm.io/blog/whats-different-in-pnpm-12), perintah, flag, setting, dan format lockfile pnpm 11 tetap berlaku, dengan beberapa pengecualian. Yang relevan untuk repo ini:

- `pnpm-workspace.yaml` yang berisi setting tak dikenal gagal dengan `ERR_PNPM_UNRECOGNIZED_WORKSPACE_SETTINGS` bila proyek mem-pin versi pnpm ([OHIF issue #6247](https://github.com/ohif/viewers/issues/6247)).
- Lockfile dari graph dependency yang siklik bisa berubah sekali saat install pertama yang me-resolve ulang. `--frozen-lockfile` memakai lockfile apa adanya.
- `pnpm install --frozen-lockfile false` dan `--resolution-only` ditolak.
- pnpm versi lama bisa gagal berpindah ke pnpm 12 (`ENOEXEC`) karena shim pnpm 12 adalah skrip tanpa shebang ([NiharDaily, 2026-10-16](https://www.nihardaily.com/posts/pnpm-12-upgrade-what-breaks-in-ci-and-how-to-fix-it)).
- `pnpm self-update` ke 12 butuh pnpm 11.10.0 atau lebih baru ([pnpm installation](https://pnpm.io/installation)).
- pnpm 12 punya binary untuk Linux musl x64/arm64 (dipakai `node:22-alpine`) dan Windows x64/arm64. Paket npm `pnpm` butuh Node.js 18 atau lebih baru.

Isi sumber di atas sudah diparafrasekan.

### Kondisi kode saat ini (dicek 2026-11-03, branch `main` @ `56b9969`, working tree punya perubahan yang belum di-commit di `navigation.ts`, `exports.preview.test.tsx`, `development-plan.md`)

- `frontend/CompanyPortal-Vite/package.json` dan `frontend/VendorPortal-Vite/package.json`: `"packageManager": "pnpm@11.20.0+sha512.9a6f…bfee"`. Tidak ada field `engines` atau `pnpm`.
- `frontend/CompanyPortal-Vite/Dockerfile` dan `frontend/VendorPortal-Vite/Dockerfile` baris 3: `RUN corepack enable && corepack install --global pnpm@11.20.0` di atas `node:22-alpine`, lalu `COPY pnpm-lock.yaml package.json pnpm-workspace.yaml ./` dan `RUN pnpm install --frozen-lockfile`.
- `pnpm-workspace.yaml` di kedua portal hanya berisi `allowBuilds` (`@biomejs/biome`, `esbuild`). Setting ini dikenal pnpm 12.
- `pnpm-lock.yaml` di kedua portal: `lockfileVersion: '9.0'`. Tidak ada dependency git (`git@`, `codeload`, `tarball:`).
- Tidak ada `.npmrc` di repo. Tidak ada pemakaian `--resolution-only` atau `--frozen-lockfile false`.
- `.github/workflows/ci.yml` **tidak punya job frontend** (hanya go, go-integration, sqlc, python, python-image). Spec lama `fe-tech-debt` menyebut job frontend dengan pnpm 11.20.0 via corepack, tapi job itu tidak ada di file sekarang. Jadi pnpm hanya dijalankan di mesin developer dan di Docker build.
- `frontend/dev.ps1` baris 52: pesan "Install it first: npm install -g pnpm  (or: corepack enable)".
- Mesin developer ini: pnpm global 11.9.0, Node 24.15.0. pnpm 11.9.0 di bawah 11.10.0, jadi `pnpm self-update` ke 12 tidak tersedia.
- Registry npm (dicek 2026-11-03): `latest` = `latest-12` = 12.16.1 (rilis 2026-10-27), `latest-11` = 11.28.9, `next-12` = 12.17.0 (rilis 2026-10-30, belum dipromosikan ke `latest`). 12.16.1 adalah rilis stabil terbaru. Perbaikan di 12.17.0 yang terkait Docker (izin file untuk user lain, pnpm/pnpm#17435) tidak mengenai repo ini, karena stage `builder` menjalankan semua perintah pnpm sebagai satu user dan stage serve hanya menyalin `dist`. sha512 tarball `pnpm@12.16.1` (hex) = `091d1dee7927575661b25dab3c2ae31cafc1c54d6b71ed6a6600cf0c1cb10434a447eae14015f1b23e2e6bedeed79df94a932e2164d2832280dce3fb32f656d1`.
- Dokumen yang menyebut versi pnpm: `.claude/docs/arsitektur-teknologi.md` baris 74 (`pnpm | 11.20.0`), `.kiro/steering/lessons-learned.md` dan `.claude/lessons-learned.md` (catatan pnpm 11 dan anjuran `npm install -g pnpm@<exact-version>` di Dockerfile karena corepack bisa gagal di `node:22-alpine`).

### Ruang lingkup

- Pin `packageManager` di kedua `package.json` ke pnpm 12.16.1 dengan hash sha512.
- Cara install pnpm di kedua Dockerfile.
- Verifikasi lockfile dan `pnpm-workspace.yaml` dengan pnpm 12.
- Gate kualitas kedua portal dijalankan ulang dengan pnpm 12.
- Pesan petunjuk di `frontend/dev.ps1` dan dokumentasi (lessons-learned, arsitektur-teknologi, development-plan, development-progress).

### Tidak termasuk

- Upgrade dependency aplikasi (React, Vite, Vitest, Biome, dan lainnya). Versi resolved di lockfile tidak boleh berubah.
- Ganti image dasar Docker (`node:22-alpine`, `nginx:alpine`) atau versi Node.
- Menambah job frontend ke `.github/workflows/ci.yml` (lihat D4).
- Fitur baru pnpm 12 (remote task cache, `globalShims`, `devEngines.runtime`, `pnpm runtime`).
- Backend Go dan Python.

### Area sensitif (project-context Sec 3 rule 7)

Tidak menyentuh auth, uang, rekonsiliasi, migrasi data, atau delete. Risikonya ada di supply chain build: binary pnpm diunduh dari registry npm. Pin `packageManager` dengan hash sha512 dan versi exact di Dockerfile menjaga versi yang terpasang tetap satu dan bisa diverifikasi. Golden Rule #1 (stack tetap): pnpm sudah bagian dari stack, ini upgrade versi tool, bukan tool baru. Tetap butuh persetujuan PO karena major version (D1).

### Dependency & Scope Note

Belum ada item untuk pekerjaan ini di `development-plan.md`. Scope guard rule 1 mewajibkan item dicatat dulu dengan OK user (D5). Pekerjaan ini tidak bergantung pada fase mana pun dan tidak mengubah perilaku aplikasi.

## Glossary

- **PackageManagerPin**: field `packageManager` di `package.json` masing-masing portal, format `pnpm@<versi>+sha512.<hex>`.
- **PortalDockerfile**: `frontend/CompanyPortal-Vite/Dockerfile` atau `frontend/VendorPortal-Vite/Dockerfile`, stage `builder`.
- **Lockfile**: `pnpm-lock.yaml` di folder portal.
- **WorkspaceConfig**: `pnpm-workspace.yaml` di folder portal.
- **QualityGate**: rangkaian perintah per portal: `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm test`, `pnpm run test:coverage` (threshold CP 84/80/80/85, VP 93/84/90/93), `pnpm build` (CP termasuk `scripts/check-bundle.mjs`), `pnpm audit --prod`, dan khusus CP `pnpm run gen:api` + `pnpm run gen:api:exq` tanpa diff.
- **DevSetupHint**: pesan petunjuk instalasi pnpm di `frontend/dev.ps1`.
- **Portal**: CompanyPortal-Vite (CP) atau VendorPortal-Vite (VP).

## Requirements

### Requirement 1: Pin versi pnpm 12.16.1 di kedua portal

**User Story:** Sebagai developer, saya ingin kedua portal mem-pin versi pnpm yang sama dan exact, supaya setiap mesin dan setiap build memakai pnpm yang sama.

#### Acceptance Criteria

1. THE PackageManagerPin di CompanyPortal-Vite dan VendorPortal-Vite SHALL bernilai `pnpm@12.16.1+sha512.<hex>` dengan hex sha512 tarball `pnpm@12.16.1` dari registry npm.
2. THE PackageManagerPin di kedua portal SHALL identik satu sama lain.
3. WHEN perubahan selesai, THE folder `frontend/` SHALL tidak lagi memuat string `11.20.0` di `package.json` maupun Dockerfile.
4. THE PackageManagerPin SHALL berupa versi exact, bukan range atau tag (`latest`, `latest-12`).

### Requirement 2: Docker build memakai pnpm 12.16.1

**User Story:** Sebagai developer, saya ingin image Docker kedua portal ter-build dengan pnpm 12.16.1, supaya image produksi memakai versi yang sama dengan pin.

#### Acceptance Criteria

1. THE PortalDockerfile SHALL memasang pnpm dengan `npm install -g pnpm@12.16.1` (versi exact, D2) dan SHALL tidak lagi memanggil `corepack`.
2. THE PortalDockerfile SHALL tetap meng-COPY `pnpm-lock.yaml`, `package.json`, dan `pnpm-workspace.yaml` sebelum `pnpm install`.
3. THE PortalDockerfile SHALL tetap menjalankan `pnpm install --frozen-lockfile` dalam bentuk flag tanpa argumen nilai.
4. WHEN `docker build` dijalankan untuk masing-masing portal, THE build SHALL selesai tanpa error, termasuk tanpa `ENOEXEC`, `ERR_PNPM_UNRECOGNIZED_WORKSPACE_SETTINGS`, dan `ERR_PNPM_IGNORED_BUILDS`.
5. WHEN stage `builder` berjalan, THE perintah `pnpm --version` di dalam image SHALL mencetak `12.16.1`.
6. THE stage serve (`nginx:alpine`, user non-root, HEALTHCHECK, header keamanan) SHALL tidak berubah.
7. WHEN container hasil build dijalankan, THE HEALTHCHECK SHALL berstatus healthy.

### Requirement 3: Lockfile dan WorkspaceConfig tetap valid

**User Story:** Sebagai developer, saya ingin upgrade pnpm tidak mengubah dependency yang ter-install, supaya perilaku aplikasi tidak berubah.

#### Acceptance Criteria

1. WHEN `pnpm install --frozen-lockfile` dijalankan dengan pnpm 12.16.1 di masing-masing portal, THE install SHALL berhasil tanpa mengubah Lockfile.
2. THE himpunan pasangan `nama@versi` yang resolved di Lockfile SHALL sama sebelum dan sesudah upgrade.
3. IF `pnpm install` tanpa frozen menghasilkan diff Lockfile, THEN THE diff SHALL hanya berisi perubahan format/peer variant pnpm 12 dan di-commit dalam perubahan yang sama, sesuai D3.
4. IF diff Lockfile mengubah versi resolved suatu package, THEN THE pekerjaan SHALL berhenti dan dilaporkan ke user sebelum lanjut.
5. THE WorkspaceConfig SHALL hanya berisi setting yang dikenal pnpm 12.16.1, dan `allowBuilds` untuk `@biomejs/biome` dan `esbuild` SHALL tetap ada.
6. WHEN install selesai, THE build script `@biomejs/biome` dan `esbuild` SHALL berjalan (binary `biome` dan `esbuild` bisa dieksekusi).

### Requirement 4: Gate kualitas tidak turun

**User Story:** Sebagai developer, saya ingin semua gate kualitas frontend tetap hijau setelah upgrade, supaya upgrade tool tidak menyembunyikan regresi.

#### Acceptance Criteria

1. WHEN QualityGate dijalankan dengan pnpm 12.16.1 di CompanyPortal-Vite, THE setiap perintah SHALL selesai dengan exit code 0.
2. WHEN QualityGate dijalankan dengan pnpm 12.16.1 di VendorPortal-Vite, THE setiap perintah SHALL selesai dengan exit code 0.
3. THE jumlah test yang lulus di masing-masing portal SHALL sama dengan hasil sebelum upgrade, dengan 0 gagal.
4. THE coverage masing-masing portal SHALL tidak di bawah threshold yang berlaku (CP 84/80/80/85, VP 93/84/90/93).
5. THE `pnpm audit --prod` di kedua portal SHALL melaporkan 0 vulnerability.
6. WHEN `pnpm run gen:api` dan `pnpm run gen:api:exq` dijalankan di CompanyPortal-Vite, THE `src/lib/api/schema.d.ts` dan `src/lib/api/exq-schema.d.ts` SHALL tidak berubah.

### Requirement 5: Setup developer lokal

**User Story:** Sebagai developer di Windows, saya ingin tahu cara pindah ke pnpm 12 di mesin saya, supaya `pnpm dev` dan `dev.ps1` tetap jalan setelah pin berubah.

#### Acceptance Criteria

1. THE DevSetupHint SHALL menyebut perintah `npm install -g pnpm@12.16.1` (D2) dan tidak lagi menyarankan `corepack enable`.
2. THE dokumentasi upgrade SHALL menjelaskan bahwa pnpm global di bawah 11.10.0 tidak bisa memakai `pnpm self-update` dan harus dipasang ulang lewat `npm install -g pnpm@12.16.1`.
3. THE dokumentasi upgrade SHALL menjelaskan gejala `ENOEXEC` / "Failed to switch pnpm" saat pnpm lama bertemu pin pnpm 12, beserta perbaikannya (pasang pnpm 12 global terlebih dulu, jangan hapus cache).
4. WHEN `frontend/dev.ps1` dijalankan setelah pnpm 12.16.1 terpasang, THE dev server portal yang dipilih SHALL start tanpa error dari pnpm.

### Requirement 6: Dokumentasi dan pencatatan

**User Story:** Sebagai anggota tim, saya ingin versi pnpm dan pelajaran upgrade tercatat di dokumen proyek, supaya dokumen tidak basi dan upgrade berikutnya lebih mudah.

#### Acceptance Criteria

1. THE `.claude/docs/arsitektur-teknologi.md` SHALL mencatat pnpm 12.16.1 untuk CP dan VP.
2. THE `.kiro/steering/lessons-learned.md` dan `.claude/lessons-learned.md` SHALL memuat entri upgrade pnpm 11 → 12 (setting workspace tak dikenal, lockfile siklik, flag yang ditolak, `ENOEXEC`, metode install Docker), dan kedua file SHALL berisi teks entri yang sama.
3. WHERE entri lessons-learned lama menyebut perilaku khusus pnpm 11, THE entri tersebut SHALL tetap ada dan diberi catatan apakah masih berlaku di pnpm 12.
4. THE `.kiro/steering/development-plan.md` SHALL memuat item cross-cutting X.12 "pnpm 12 upgrade" dengan status dan langkah berikutnya (D5).
5. WHEN pekerjaan selesai, THE `.claude/development-progress.md` SHALL memuat entri changelog upgrade pnpm 12.16.1, termasuk satu baris untuk developer lain: pasang ulang pnpm global dengan `npm install -g pnpm@12.16.1` (D6).
6. THE baris X.3 di `.kiro/steering/development-plan.md` SHALL mencatat temuan bahwa `.github/workflows/ci.yml` tidak punya job frontend, sebagai item terbuka di luar spec ini (D4).

## Open Questions

| ID | Pertanyaan | Saran Kiro | Status |
| --- | --- | --- | --- |
| D1 | Target versi: pnpm 12.16.1 (major, rewrite Rust) atau tetap di line 11 dengan naik ke 11.28.9 (`latest-11`, berisi fix keamanan 11.28.5)? | 12.16.1 sesuai permintaan. `latest` di npm sudah menunjuk ke line 12, repo tidak punya dependency git, `.npmrc`, atau flag yang ditolak, dan `pnpm-workspace.yaml` hanya berisi `allowBuilds`. Risiko utamanya hanya Docker build dan mesin developer, dan keduanya diverifikasi di spec ini. | ✅ DIPUTUSKAN PO 2026-11-03 (ikut saran) |
| D2 | Cara memasang pnpm di Dockerfile: tetap `corepack install --global pnpm@12.16.1` atau ganti ke `npm install -g pnpm@12.16.1`? | `npm install -g pnpm@12.16.1`. lessons-learned sudah mencatat corepack bisa gagal di `node:22-alpine` untuk pnpm 11, shim pnpm 12 punya isu `ENOEXEC` saat dijalankan oleh launcher lain, dan Node 25+ tidak lagi membawa corepack. Integritas tetap terjaga oleh hash di PackageManagerPin. | ✅ DIPUTUSKAN PO 2026-11-03 (ikut saran) |
| D3 | Lockfile: biarkan apa adanya (cukup `--frozen-lockfile` lolos) atau paksa regenerate sekali dengan pnpm 12? | Biarkan apa adanya. Jalankan satu `pnpm install` tanpa frozen untuk cek diff. Bila ada diff format/peer variant, commit di perubahan yang sama supaya churn tidak muncul diam-diam di PR lain. Bila versi resolved berubah, berhenti dan lapor (Req 3.4). | ✅ DIPUTUSKAN PO 2026-11-03 (ikut saran) |
| D4 | `ci.yml` tidak punya job frontend. Tambahkan job frontend (pnpm 12.16.1, lint, test, build) di spec ini? | Tidak di spec ini (YAGNI, satu concern per perubahan). Catat sebagai temuan di `development-plan.md` X.3 dan putuskan terpisah. | ✅ DIPUTUSKAN PO 2026-11-03 (ikut saran) |
| D5 | Catat pekerjaan ini sebagai item cross-cutting baru di `development-plan.md` (X.12)? | Ya, X.12 "pnpm 12 upgrade", sesuai scope guard rule 1. | ✅ DIPUTUSKAN PO 2026-11-03 (ikut saran) |
| D6 | Developer lain yang memakai pnpm global 10.x atau 11.x di bawah 11.10.0: cukup didokumentasikan, atau perlu pengumuman/checklist khusus? | Cukup dokumentasi (Req 5) plus satu baris di ringkasan commit. Tim kecil dan pin `packageManager` memunculkan error yang jelas. | ✅ DIPUTUSKAN PO 2026-11-03 (ikut saran) |
