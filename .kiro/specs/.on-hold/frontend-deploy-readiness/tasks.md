# Implementation Plan

> Status: **draft 2026-11-04, menunggu persetujuan PO.** Requirements dan design disetujui 2026-11-04.

## Overview

Urutan: buktikan masalah dulu di Linux (task 1, termasuk gate berhenti bila masalah tidak terbukti), cari tag + digest base image (task 2.1), lalu ubah konfigurasi (task 2.2–5.1, file berbeda), verifikasi di Docker Desktop dan Linux (task 4.4, 5.2, 6), baru tulis dokumentasi dari hasil yang terukur (task 7). Tidak ada perubahan kode aplikasi.

Semua perintah dan hasil verifikasi dicatat di `.kiro/specs/frontend-deploy-readiness/evidence.md` (dibuat di task 1.1), satu bagian `##` per task. Karena semua task verifikasi menulis file itu, task-task tersebut diserialkan di Task Dependency Graph.

Pola rujukan: job `go-image` dan `scripts/ci/check-image.sh` (spec `backend-exq-deploy-readiness`), bugfix HEALTHCHECK 2026-11-03 (`.claude/bugfixes.md`), bagian "IP client & proxy tepercaya" di `.claude/docs/deployment.md`.

### Model Split

**Sonnet-runnable**
- 1.1 Harness Linux_Probe — Sonnet, High
- 1.2 Probe HEAD di Linux_Engine — Sonnet, High
- 1.3 Probe HEAD di Docker_Desktop — Sonnet, High
- 1.4 Gate hasil probe + pilih varian R/S — Sonnet, High
- 2.1 Cari tag persis + digest index — Sonnet, Low
- 2.2 Ubah `FROM` kedua Dockerfile — Sonnet, Low
- 3.1 CP_Nginx upstream — Sonnet, Medium (needs 1.4)
- 3.2 VP_Nginx upstream — Sonnet, Medium (needs 1.4)
- 3.3 Review diff nginx (Property 5) — Sonnet, Medium
- 4.1 Base_Compose — Sonnet, Medium
- 4.2 Prod_Override — Sonnet, Medium
- 4.3 Hapus VP_Standalone_Compose + catatan — Sonnet, Medium
- 4.4 `docker compose config` (Property 6, 7) — Sonnet, Medium
- 5.1 Job CI `fe-image` — Sonnet, Low
- 5.2 Jalankan langkah `fe-image` lokal (Property 8) — Sonnet, Low
- 6.1 Verifikasi Docker_Desktop — Sonnet, High
- 6.2 Verifikasi Linux_Engine + Prod_Override — Sonnet, High
- 6.3 Upstream mati saat start (Property 4) — Sonnet, High
- 6.4 Permukaan tidak berubah (Property 9) — Sonnet, High
- 6.5 Bersihkan harness — Sonnet, High
- 7.1 `deployment.md` deploy frontend, TLS/LB, Pipeline, EOD routing — Sonnet, Medium
- 7.3 `development-plan.md` + `development-progress.md` — Sonnet, Medium
- 8. Checkpoint — Sonnet, Medium
- 9. Daftar outstanding user — Haiku, Low (Haiku)
- 10. Commit setelah user menulis "commit" — Haiku, Low (Haiku)

**Opus-required**
- 7.2 `deployment.md` nilai `TRUSTED_PROXIES` / `RETRY_TRUSTED_PROXIES` — Opus, High — auth: nilai salah membuat semua user berbagi batas login per IP dan `audit_logs.ip` salah

Dependensi lintas grup: Opus 7.2 butuh hasil terukur Sonnet 6.1 dan 6.2 (`remote_ip` backend, alamat sumber di stub EOD). Sonnet 8 butuh Opus 7.2 selesai.

## Tasks

- [ ] 1. Linux_Probe: buktikan masalah #1 sebelum perbaikan
  - [ ] 1.1 Siapkan harness dan `evidence.md`
    - Pastikan working tree bersih untuk file frontend (`git status -- frontend .github`); catat HEAD.
    - Build di Docker Desktop dari HEAD: `probe-cp:head` (`frontend/CompanyPortal-Vite`), `probe-vp:head` (`frontend/VendorPortal-Vite`), `cms-backend-api` dan `cms-backend-exq` (lewat compose backend masing-masing, `build` saja).
    - `docker run -d --privileged --name fe-dind -v "<repo>:/repo:ro" docker:29-dind`; pindahkan image dengan `docker save -o $env:TEMP\fe-probe.tar …` → `docker cp` → `docker exec fe-dind docker load -i …` (jangan lewat pipe PowerShell).
    - Di `fe-dind`: network `cms-net`, `cms-redis` (`redis:7-alpine`), `backend` (`--network-alias backend`, `--env-file /repo/backend/.env`, `--add-host host.docker.internal:<H>`, `-p 8080:8080`), `backend-exq` (alias `backend-exq`, env `/repo/backend-exq/.env`, `--add-host` sama). `<H>` = `getent hosts host.docker.internal` di `fe-dind`.
    - Stub EOD: container nginx `--network host` di `fe-dind`, port 8091, `/health` menjawab `eod-stub remote=$remote_addr uri=$request_uri`.
    - Bila Postgres dev tidak tercapai, ganti backend dengan stub (design, Linux_Probe langkah 5), catat, dan laporkan ke user sebelum lanjut.
    - Buat `evidence.md` dengan bagian "Harness" (perintah, versi image, `<H>`).
    - _Requirements: 1.1, 1.4_
  - [ ] 1.2 Probe HEAD di Linux_Engine
    - Jalankan `probe-cp:head` (3001) dan `probe-vp:head` (3002) di `cms-net` dalam `fe-dind`, tanpa `extra_hosts`.
    - `wget -S -O-` dari `fe-dind` ke `/api/v1/auth/me` (CP, VP), `/api/v1/exports/jobs` (CP), `/api/eod/health` (CP); catat status + `docker logs` nginx.
    - 1.2(a)/(b): `getent hosts host.docker.internal` di container CP tanpa dan dengan `--add-host host.docker.internal:host-gateway`.
    - 1.2(c): ulangi request `/api/eod/health` dan `/api/v1/auth/me` ke container (b); catat apakah resolver nginx me-resolve nama itu.
    - _Requirements: 1.1, 1.2, 1.4_
  - [ ] 1.3 Probe HEAD di Docker_Desktop (pembanding)
    - `docker compose -f backend/docker-compose.yaml up -d`, `backend-exq` compose up, `eod_retry_scheduler` di host bila bisa dijalankan; Base_Compose HEAD `up -d --build`.
    - Request yang sama dari host (`curl.exe -s -i`); langkah 1.2(c) dengan container CP tambahan yang memakai `--add-host host.docker.internal:host-gateway`.
    - _Requirements: 1.3, 1.4_
  - [ ] 1.4 Gate hasil probe
    - IF `/api/` CP dan VP berfungsi di Linux_Engine tanpa perubahan: berhenti, laporkan ke user, jangan kerjakan task 2.2 dan seterusnya.
    - Else: pilih varian `/api/eod/` (R bila 1.2(c) berhasil di Linux_Engine dan Docker_Desktop, selain itu S), tulis alasannya di `evidence.md`, laporkan ringkas ke user.
    - _Requirements: 1.5_
  - _Model: Sonnet, High — debugging jaringan Docker lintas dua engine dengan banyak kemungkinan hasil; tidak ada perubahan kode._

- [ ] 2. Pin base image
  - [ ] 2.1 Cari tag persis dan digest index
    - `docker buildx imagetools inspect node:22.23.4-alpine3.24` dan `nginx:1.31.8-alpine3.24`; ambil baris `Digest:` (index multi-arch). Bila tag `-alpine3.24` tidak ada, pakai `-alpine` dengan versi yang sama.
    - Catat tag + digest di `evidence.md`.
    - _Requirements: 5.1, 5.2_
  - [ ] 2.2 Ubah `FROM` di `frontend/CompanyPortal-Vite/Dockerfile` dan `frontend/VendorPortal-Vite/Dockerfile`
    - Hanya dua baris `FROM` per file; HEALTHCHECK `127.0.0.1`, `USER appuser`, `LABEL`, langkah build tidak berubah.
    - _Requirements: 5.1, 5.2, 5.3_
  - _Model: Sonnet, Low — dua baris per file, nilai sudah ditentukan design dan task 2.1._

- [ ] 3. Upstream nginx (D1)
  - [ ] 3.1 `frontend/CompanyPortal-Vite/nginx.conf`
    - `/api/`: `set $backend_upstream http://backend:8080;` + komentar; resolver tetap.
    - `/api/eod/`: varian dari task 1.4 (R = tidak berubah; S = `rewrite … break;` + `proxy_pass http://host.docker.internal:8091;`, tanpa resolver/variabel).
    - `listen 80`, `/api/v1/exports/`, header, `proxy_cookie_path`, security headers, cache tidak berubah.
    - _Requirements: 2.1, 2.3, 2.4, 2.5, 2.6, 2.7, 6.3, 11.3_
  - [ ] 3.2 `frontend/VendorPortal-Vite/nginx.conf`
    - `/api/`: `set $backend_upstream http://backend:8080;` + komentar. Sisanya tidak berubah.
    - _Requirements: 2.2, 2.5, 2.6, 2.7, 6.3, 11.3_
  - [ ] 3.3 Review diff nginx (Property 5)
    - `git diff -- frontend/CompanyPortal-Vite/nginx.conf frontend/VendorPortal-Vite/nginx.conf`: hanya baris upstream (dan blok `/api/eod/` bila varian S) serta komentar. Tempel diff di `evidence.md`.
    - _Requirements: 2.6, 11.3_
  - _Model: Sonnet, Medium — perubahan satu-dua baris, tetapi jalur `/api/v1/auth` dan IP klien lewat blok ini; diff harus ditinjau ketat._

- [ ] 4. Compose
  - [ ] 4.1 `frontend/docker-compose.yml`
    - `image: ${FE_REGISTRY:-}companyportal-vite-fe:${IMAGE_TAG:-local}` dan `${FE_REGISTRY:-}vendorportal-vite-fe:${IMAGE_TAG:-local}`.
    - Service CP: `extra_hosts: ["host.docker.internal:host-gateway"]` + komentar. Port `3001:80`/`3002:80`, healthcheck, network, restart tidak berubah.
    - _Requirements: 3.3, 4.1, 6.1_
  - [ ] 4.2 `frontend/docker-compose.prod.yml` (baru)
    - Isi sesuai design (Prod_Override): `build: !reset null`, `image` dengan `${FE_REGISTRY:?…}` + `${IMAGE_TAG:?…}`, `ports: !override` dengan `${FE_BIND_ADDR:?…}`; komentar perintah `pull` / `up -d --no-build` dan syarat Compose ≥ 2.24.4.
    - _Requirements: 4.2, 4.3, 6.2_
  - [ ] 4.3 Hapus `frontend/VendorPortal-Vite/docker-compose.yml`
    - Tambah satu catatan di bagian atas `.kiro/specs/.on-hold/gke-deploy/design.md` dan `tasks.md` bahwa file itu dihapus 2026-11-04 (spec `frontend-deploy-readiness`). `bugfixes.md` dan `.kiro/specs/.done/**` tidak diubah.
    - Grep ulang rujukan aktif lain (di luar sejarah); bila ada, perbarui.
    - _Requirements: 7.1, 7.2_
  - [ ] 4.4 `docker compose config` (Property 6, Property 7)
    - Base saja tanpa env: image `…:local`, port `3001:80`/`3002:80`, ada `build`, `config --services` memuat CP + VP.
    - Base + Prod_Override dengan `FE_REGISTRY=probe.registry.local/`, `IMAGE_TAG=<sha>`, `FE_BIND_ADDR=10.0.0.5`: tepat satu port per service di `10.0.0.5`, tanpa `build`, image sesuai.
    - Base + Prod_Override dengan masing-masing dari tiga variabel dikosongkan: gagal, pesan menyebut variabel.
    - Pastikan env shell bersih sebelum tiap run (`Remove-Item Env:FE_*`, `Env:IMAGE_TAG`). Tempel bagian `image`/`ports` hasil `config` di `evidence.md`.
    - _Requirements: 3.3, 4.1, 4.2, 4.3, 6.1, 6.2, 7.3, 10.1_
  - _Model: Sonnet, Medium — semantik merge Compose (`!reset`, `!override`, `:?`) mudah salah tanpa diuji; diuji di 4.4._

- [ ] 5. Job CI image frontend (D6)
  - [ ] 5.1 Tambah job `fe-image` di `.github/workflows/ci.yml`
    - Sesuai design (FeImageJob): matrix `include` portal + image huruf kecil, `fail-fast: false`, build dengan context folder portal, `check-image.sh`, `nginx -t` dengan `--add-host host.docker.internal:host-gateway`. Tanpa push, tanpa secret, tanpa izin tambahan.
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_
  - [ ] 5.2 Jalankan langkah `fe-image` secara lokal (Property 8)
    - Untuk kedua portal: `docker build -t <image>:ci frontend/<portal>`, `bash scripts/ci/check-image.sh <image>:ci frontend/<portal>` (Git Bash; bila tidak ada, jalankan di `fe-dind`), `docker run --rm --add-host host.docker.internal:host-gateway --entrypoint nginx <image>:ci -t`, dan `nginx -v` = versi ter-pin. Catat hasil di `evidence.md`.
    - _Requirements: 5.3, 5.4, 8.3, 8.4_
  - _Model: Sonnet, Low — job mengikuti pola `go-image` yang sudah ada._

- [ ] 6. Verifikasi sesudah perbaikan
  - [ ] 6.1 Docker_Desktop (Property 1, 2, 3)
    - Backend + `backend-exq` compose up, `eod_retry_scheduler` di host (`python -m eod_retry_scheduler.run` dari `backend_python/`), lalu `docker compose -f frontend/docker-compose.yml up -d --build` tanpa env dan tanpa `.env` baru.
    - Kedua container `healthy`. Dari host (`curl.exe`): `:3001/` dan `:3002/` = 200; `/api/v1/auth/me` (CP, VP) = 401 dari Go; `:3001/api/v1/exports/jobs` = 401 dari `backend-exq`; `:3001/api/eod/health` (dan `?probe=1`) dijawab EOD.
    - Catat `remote_ip` di log `http_request` `backend` dan `backend-exq`. Bila EOD tidak bisa dijalankan di host, catat dan jadikan outstanding.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 10.2, 10.3, 10.4_
  - [ ] 6.2 Linux_Engine dengan Prod_Override (Property 1, 2, 3, 7)
    - Load image baru ke `fe-dind`, tag ulang `probe.registry.local/<image>:<sha>`; `docker compose -f /repo/frontend/docker-compose.yml -f /repo/frontend/docker-compose.prod.yml up -d --no-build` dengan `FE_REGISTRY`, `IMAGE_TAG`, `FE_BIND_ADDR=<IP eth0 fe-dind>`.
    - Kedua container `healthy`; Upstream_Check yang sama dengan 6.1 ke `FE_BIND_ADDR`; pastikan port tidak terbuka di `0.0.0.0` (`docker port`, `ss -ltn`).
    - Catat `remote_ip` backend dan `remote=` dari stub EOD (dasar `TRUSTED_PROXIES` dan `RETRY_TRUSTED_PROXIES`), serta subnet `cms-net` (`docker network inspect`).
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.7, 4.3, 6.2, 10.2, 10.3, 10.4_
  - [ ] 6.3 Upstream mati saat start (Property 4)
    - Di `fe-dind`: hentikan `backend`, `backend-exq`, stub EOD; restart container frontend; cek `healthy`, `/` = 200, ketiga path API = 502. Nyalakan lagi upstream; cek path kembali dijawab upstream tanpa restart nginx.
    - _Requirements: 2.5_
  - [ ] 6.4 Permukaan yang tidak boleh berubah (Property 9)
    - `git diff --stat <HEAD awal>` dan `git status`: tidak ada perubahan di `frontend/*/src/**`, `vite.config.ts` kedua portal, `frontend/dev.ps1`, `security-headers.conf`, `backend/`, `backend-exq/`, `backend_python/`, `pkg/`. Tidak ada file `docker:dind`/harness di repo.
    - _Requirements: 3.4, 11.1, 11.2_
  - [ ] 6.5 Bersihkan harness
    - Hapus `fe-dind` (`docker rm -f`), volume anonimnya, `$env:TEMP\fe-probe.tar`, image `probe-*`. Container lokal hasil 6.1 dibiarkan jalan (lingkungan kerja user). Catat di `evidence.md`.
    - _Requirements: 11.2_
  - _Model: Sonnet, High — verifikasi lintas dua engine; hasil 6.1/6.2 menjadi dasar nilai `TRUSTED_PROXIES` produksi._

- [ ] 7. Dokumentasi
  - [ ] 7.1 `.claude/docs/deployment.md`: deploy frontend, TLS/LB, Pipeline, EOD routing
    - Tabel produk #1 (D7); paragraf "Pipeline" (job `fe-image`, lint/test frontend tetap X.3, `cloudbuild.yaml` spec terpisah); bagian baru "Deploy frontend (CompanyPortal + VendorPortal)" sesuai design (prasyarat termasuk Compose ≥ 2.24.4, tabel env, build + push dengan `VITE_APP_ENV`, deploy, verifikasi tanpa browser, rollback, TLS/LB/firewall, catatan internal LB, update pin image manual); "EOD monitoring routing" (jalur `host-gateway`, varian R/S, firewall 8091).
    - Perintah verifikasi yang ditulis = perintah yang lolos di 6.1/6.2.
    - _Requirements: 4.4, 6.4, 9.1, 9.4_
  - [ ] 7.2 `.claude/docs/deployment.md`: `TRUSTED_PROXIES` dan `RETRY_TRUSTED_PROXIES`
    - Bagian "IP client & proxy tepercaya": nilai produksi `backend` = subnet `cms-net` + `35.191.0.0/16` + `130.211.0.0/22` + `<lb-ip>/32` (urutan pembacaan `X-Forwarded-For` dijelaskan), ganti teks "Docker network gateway"; `backend-exq` sama; `RETRY_TRUSTED_PROXIES` = alamat yang terukur di 6.2; port 8080 tidak perlu di-publish untuk frontend; dampak bila salah (batas login per IP bersama, `audit_logs.ip` = IP proxy).
    - Release checklist `backend-exq`, baris `TRUSTED_PROXIES`: sesuaikan ("tidak lagi berbeda dari `backend`").
    - Nilai harus cocok dengan bukti 6.1/6.2; yang belum bisa diukur (alamat GFE, `<lb-ip>`) ditandai "verifikasi saat deploy pertama".
    - _Requirements: 9.2, 9.3_
  - [ ] 7.3 `.kiro/steering/development-plan.md` dan `.claude/development-progress.md`
    - X.15: status built, ringkasan, langkah berikutnya (cek browser, run CI pertama, deploy pertama). Satu entri di `development-progress.md`.
    - _Requirements: 9.5_
  - _Model: Opus, High — 7.2 menentukan nilai `TRUSTED_PROXIES` produksi (zona auth: batas login per IP dan IP audit). 7.1 dan 7.3 Sonnet, Medium._

- [ ] 8. Checkpoint
  - Ulangi 4.4 (tiga kombinasi `config`) dan `nginx -t` dari 5.2 terhadap working tree akhir.
  - Jalankan checklist traceability spec (`kiro-spec-standard` Sec 5) terhadap `evidence.md`: setiap Property 1–9 punya bukti.
  - `git status` dan `git diff --stat`: hanya file di "Files touched" design + `evidence.md`.
  - Laporkan hasil ke user. Jangan commit.
  - _Requirements: 10.1, 10.4, 10.6, 11.1_
  - _Model: Sonnet, Medium — gate verifikasi; butuh penilaian bila ada hasil yang tidak cocok._

- [ ] 9. Daftar outstanding untuk user
  - Tulis di `evidence.md` dan laporan akhir: cek browser manual (login CP dan VP lewat `localhost:3001`/`3002`, EOD Monitoring, satu unduhan ekspor, IP di Log Audit), run GitHub Actions pertama `fe-image` setelah push, dan verifikasi `remote_ip` saat deploy pertama ke VM (Golden Rule #10).
  - _Requirements: 10.5_
  - _Model: Haiku, Low — hanya mencatat._

- [ ] 10. Commit
  - Hanya setelah user menulis "commit". Pesan conventional commit; stage file per nama (tanpa `git add -A`).
  - _Requirements: 10.6_
  - _Model: Haiku, Low — mekanis, menunggu instruksi user._

## Notes

### Aturan varian `/api/eod/`
Ditentukan di task 1.4 dari hasil 1.2(c) dan 1.3: varian R hanya bila resolver nginx me-resolve `host.docker.internal` dari `extra_hosts` di **kedua** engine; selain itu varian S. Tidak perlu keputusan PO baru (design disetujui dengan aturan ini).

### Catatan Windows
- Jangan memindahkan image lewat pipe PowerShell (`docker save | docker load`); pakai file `-o` / `-i`.
- Pakai `curl.exe` (bukan alias `curl` = `Invoke-WebRequest` di Windows PowerShell 5).
- Path repo mengandung spasi (`RB Yudha Rangga`, OneDrive); selalu dikutip saat bind mount.
- Env Compose di PowerShell: `$env:IMAGE_TAG = "…"`; hapus dengan `Remove-Item Env:IMAGE_TAG` sebelum menguji Base_Compose tanpa env.

### Keamanan harness
`fe-dind` berjalan `--privileged` dan memuat `backend/.env` (secret dev) lewat bind mount read-only. Hanya untuk verifikasi lokal, dihapus di 6.5. Tidak ada secret yang ditulis ke `evidence.md`.

### Definition of Done
- Semua task 1–8 selesai; task 9 tertulis; task 10 menunggu user.
- `evidence.md` memuat bukti Property 1–9 dan hasil probe sebelum perbaikan.
- `docker compose -f frontend/docker-compose.yml up` di Docker Desktop tetap jalan tanpa langkah tambahan.
- Tidak ada perubahan di permukaan yang dilarang (Property 9).

## Task Dependency Graph

- **Wave 0:** 1.1 harness dan `evidence.md`.
- **Wave 1:** 1.2 probe Linux (butuh 1.1).
- **Wave 2:** 1.3 probe Docker Desktop (butuh 1.1; diserialkan karena menulis `evidence.md`).
- **Wave 3:** 1.4 gate (butuh 1.2, 1.3).
- **Wave 4:** 2.1 tag + digest (menulis `evidence.md`).
- **Wave 5:** 2.2, 3.1, 3.2, 4.1, 4.2, 4.3, 5.1 (file berbeda; butuh 1.4, dan 2.2 butuh 2.1).
- **Wave 6:** 3.3 review diff nginx (butuh 3.1, 3.2).
- **Wave 7:** 4.4 `compose config` (butuh 4.1, 4.2, 4.3).
- **Wave 8:** 5.2 langkah `fe-image` lokal (butuh 2.2, 3.1, 3.2, 5.1).
- **Wave 9:** 6.1 verifikasi Docker Desktop (butuh wave 5–8).
- **Wave 10:** 6.2 verifikasi Linux + Prod_Override (butuh wave 5–8).
- **Wave 11:** 6.3 upstream mati (butuh 6.2).
- **Wave 12:** 6.4 permukaan tidak berubah (butuh semua edit).
- **Wave 13:** 6.5 bersihkan harness (butuh 6.2, 6.3).
- **Wave 14:** 7.1 dokumen deploy (butuh 6.1, 6.2).
- **Wave 15:** 7.2 `TRUSTED_PROXIES` (butuh 6.1, 6.2; file sama dengan 7.1).
- **Wave 16:** 7.3 development-plan + progress.
- **Wave 17:** 8 checkpoint (butuh semua di atas).
- **Wave 18:** 9 outstanding, 10 commit (10 menunggu user menulis "commit").

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2"] },
    { "id": 2, "tasks": ["1.3"] },
    { "id": 3, "tasks": ["1.4"] },
    { "id": 4, "tasks": ["2.1"] },
    { "id": 5, "tasks": ["2.2", "3.1", "3.2", "4.1", "4.2", "4.3", "5.1"] },
    { "id": 6, "tasks": ["3.3"] },
    { "id": 7, "tasks": ["4.4"] },
    { "id": 8, "tasks": ["5.2"] },
    { "id": 9, "tasks": ["6.1"] },
    { "id": 10, "tasks": ["6.2"] },
    { "id": 11, "tasks": ["6.3"] },
    { "id": 12, "tasks": ["6.4"] },
    { "id": 13, "tasks": ["6.5"] },
    { "id": 14, "tasks": ["7.1"] },
    { "id": 15, "tasks": ["7.2"] },
    { "id": 16, "tasks": ["7.3"] },
    { "id": 17, "tasks": ["8"] },
    { "id": 18, "tasks": ["9", "10"] }
  ]
}
```
