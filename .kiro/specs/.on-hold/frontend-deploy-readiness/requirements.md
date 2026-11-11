# Requirements Document

> Status: **requirements disetujui PO 2026-11-04** (D1–D7 mengikuti saran Kiro). Acceptance criteria yang bergantung pada keputusan diberi tanda *(D#)*. Design: `design.md`.
> Item rencana: **X.15** di tabel cross-cutting `.kiro/steering/development-plan.md` (ditambahkan 2026-11-04 atas persetujuan PO, scope guard rule 1).

## Introduction

`frontend/docker-compose.yml` berjalan di Docker Desktop (Windows), tetapi belum siap dijalankan di Compute Engine (VM Linux dengan Docker Engine). Spec ini membuat image dan compose frontend bisa di-deploy ke VM tanpa merusak alur lokal yang sekarang.

### Kondisi kode saat ini (dicek 2026-11-04, HEAD `320886e` di `dev1`)

Temuan review 2026-11-04 diperiksa ulang terhadap kode. Semua temuan terbukti di kode; temuan #1 di Linux belum dibuktikan dengan menjalankan container (itu Req 1).

| # | Temuan review | Hasil cek kode |
|---|---|---|
| 1 | Nginx mem-proxy `/api` ke `host.docker.internal` | **Benar.** `CompanyPortal-Vite/nginx.conf`: `/api/` → `http://host.docker.internal:8080`, `/api/eod/` → `http://host.docker.internal:8091` (prefix dibuang). `VendorPortal-Vite/nginx.conf`: `/api/` → `host.docker.internal:8080`. Semua lewat `resolver 127.0.0.11 valid=30s ipv6=off` + `set $var …; proxy_pass $var;`. `frontend/docker-compose.yml` tidak punya `extra_hosts`. Blok CP `/api/v1/exports/` (bukan `/api/exports/`) → `http://backend-exq:8081` lewat `cms-net`, `proxy_buffering off`, `proxy_read_timeout 300s`. VP tidak punya blok ekspor dan `src/` VP tidak memanggil `/exports` (tidak perlu). |
| 2 | Image tanpa versi, di-build di mesin, belum ada `cloudbuild.yaml` | **Benar.** `image: companyportal-vite-fe` dan `image: vendorportal-vite-fe` (tag implisit `latest`), keduanya punya `build:`. Tidak ada `cloudbuild*.yaml` di repo. `deployment.md` (#8, paragraf "Pipeline") mewajibkan push ke Artifact Registry dengan tag git SHA. Catatan: nama CP tidak konsisten (`image: companyportal-vite-fe`, tetapi `container_name` dan `LABEL org.opencontainers.image.title` = `userportal-vite-fe`). |
| 3 | `FROM nginx:alpine` tidak di-pin | **Benar**, di kedua Dockerfile. Stage builder `FROM node:22-alpine` juga tidak di-pin. Saat dicek: `nginx:alpine` lokal = nginx 1.31.8 (mainline), Alpine 3.24.3, digest `sha256:a3554c0d0b48…7417`; `node:22-alpine` = Node 22.23.4, Alpine 3.24.3. |
| 4 | Nginx hanya `listen 80`, port publish ke semua interface, TLS tidak terdokumentasi | **Benar.** `ports: "3001:80"` (CP) dan `"3002:80"` (VP). `deployment.md` tidak membahas TLS atau load balancer frontend. Tambahan: cookie refresh dibuat `Secure: true` (`backend/internal/handler/auth_handler.go` baris 333, 346) dan `security-headers.conf` mengirim HSTS, jadi produksi wajib HTTPS di sisi browser. |
| 5 | Compose kedua `VendorPortal-Vite/docker-compose.yml` | **Benar.** Service `vendor-portal`, `build: .`, port `${PORT:-3001}:80` (bentrok dengan CP `3001`), tanpa `cms-net` dan tanpa `name:`. Masih dirujuk oleh `.kiro/specs/.on-hold/gke-deploy/` (design baris 294, 747; tasks baris 60); rujukan lain hanya catatan sejarah (`bugfixes.md`, `.done/vendor-portal/tasks.md`). |
| 6 | CI tanpa job frontend | **Benar.** `.github/workflows/ci.yml` berisi `go`, `go-integration`, `sqlc`, `python`, `python-image`, `go-image`. `scripts/ci/check-image.sh <image> <label>` (non-root + HEALTHCHECK) bisa dipakai untuk image frontend tanpa diubah. |

Fakta lain yang memengaruhi keputusan:

- **Topologi VM tidak jelas.** Tabel produk `deployment.md` menulis Compute Engine #1 "Frontend" dan #2 "Backend" sebagai baris terpisah. Sebaliknya `frontend/docker-compose.yml` bergabung ke network `cms-net` (eksternal) yang dibuat `backend/docker-compose.yaml`, dan proxy ekspor ke `backend-exq:8081` hanya bisa jalan bila frontend dan backend berada di Docker host yang sama. Lihat D7.
- `backend/docker-compose.yaml`: service `backend` (container `cms-backend`) di `cms-net`, port `8080:8080`; Redis `6379:6379`. `backend-exq/docker-compose.yaml`: `backend-exq` di `cms-net`, port `8081:8081`.
- `eod_retry_scheduler` (:8091) berjalan di host VM, bukan container (`deployment.md` "EOD monitoring routing"), dan bind `0.0.0.0` (`backend_python/eod_retry_scheduler/run.py` baris 15).
- `pkg/middleware.TrustedRealIP` hanya membaca `X-Forwarded-For` (dari kanan, melewati hop tepercaya) bila koneksi datang dari `TRUSTED_PROXIES`; `X-Real-IP` diabaikan. `deployment.md` saat ini menulis nilai prod `backend` = "Docker network gateway on the VM" (jalur `host.docker.internal`) dan nilai `backend-exq` = subnet `cms-net`.
- Docker lokal: Docker Desktop 4.98.0 (Engine 29.9.2). WSL hanya berisi distro `docker-desktop` (tidak ada distro Linux dengan Docker Engine sendiri), jadi pembuktian Linux memakai `docker:dind` (Req 1).
- `vite.config.ts` kedua portal dan `frontend/dev.ps1` mem-proxy langsung ke `localhost:8080/8081/8091`; tidak memakai nginx.

### Ruang lingkup

- `frontend/CompanyPortal-Vite/nginx.conf`, `frontend/VendorPortal-Vite/nginx.conf` (hanya blok upstream).
- `frontend/CompanyPortal-Vite/Dockerfile`, `frontend/VendorPortal-Vite/Dockerfile` (hanya baris `FROM`).
- `frontend/docker-compose.yml` (env dengan default lokal) dan file baru `frontend/docker-compose.prod.yml`.
- `frontend/VendorPortal-Vite/docker-compose.yml` *(D5)*.
- Job CI image frontend di `.github/workflows/ci.yml` *(D6)*.
- `.claude/docs/deployment.md` dan `.kiro/steering/development-plan.md`.

### Tidak termasuk

- Kode aplikasi frontend (`src/**`), `vite.config.ts`, `frontend/dev.ps1`, `security-headers.conf`.
- Backend, `backend-exq`, `backend_python`, `pkg/` (termasuk `TrustedRealIP`) dan compose backend, kecuali disetujui lewat Open Questions.
- `cloudbuild.yaml` *(D3, saran: spec terpisah)*.
- Job CI lint/test/coverage/build frontend *(D6, saran: tetap X.3)*.
- Membuat VM, load balancer, sertifikat, firewall, atau Artifact Registry di GCP (pekerjaan infra oleh user; spec ini hanya mendokumentasikan).
- Spec `gke-deploy` (on hold).

### Temuan di luar scope (dicatat, tidak diperbaiki di sini)

- `backend/docker-compose.yaml` mem-publish Redis `6379:6379` dan `8080:8080` ke semua interface. Bila file itu dipakai apa adanya di VM, Redis terbuka ke jaringan VM. Perlu diputuskan di spec deploy backend.
- Image backend juga belum bertag (`cms-backend-api`, `cms-backend-exq`); ikut D3.
- Port `8091` EOD bind `0.0.0.0` di host; perlu firewall VPC.

### Area sensitif (project-context Sec 3 rule 7)

- **Auth:** jalur `/api/v1/auth` (cookie refresh `Secure`, `proxy_cookie_path`) melewati nginx yang diubah. Kode auth tidak berubah.
- **IP klien:** perubahan jalur nginx → backend mengubah alamat yang dilihat backend, jadi mengubah nilai `TRUSTED_PROXIES` produksi. Nilai salah = semua user berbagi batas login per IP (`RATE_LIMIT_IP`) dan `audit_logs.ip` mencatat IP nginx.
- Tidak ada perubahan uang, rekonsiliasi, maker-checker, atau migrasi.

## Glossary

- **Base_Compose**: `frontend/docker-compose.yml`. Konfigurasi lokal; dipakai juga sebagai dasar produksi.
- **Prod_Override**: file baru `frontend/docker-compose.prod.yml`, dipakai bersama Base_Compose (`-f docker-compose.yml -f docker-compose.prod.yml`) hanya di VM.
- **VP_Standalone_Compose**: `frontend/VendorPortal-Vite/docker-compose.yml`.
- **CP_Nginx** / **VP_Nginx**: `nginx.conf` CompanyPortal / VendorPortal di dalam image.
- **Frontend_Image**: image CP (`frontend/CompanyPortal-Vite/Dockerfile`) dan VP (`frontend/VendorPortal-Vite/Dockerfile`).
- **Backend_API**: service `backend` (Go, :8080) dari `backend/docker-compose.yaml`, di `cms-net`.
- **EXQ_API**: service `backend-exq` (Go, :8081), di `cms-net`.
- **EOD_Service**: `eod_retry_scheduler` (Python, :8091), berjalan di host (bukan container).
- **Linux_Engine**: Docker Engine Linux tanpa fitur Docker Desktop. Di mesin ini: container `docker:dind` (privileged) di Docker Desktop.
- **Docker_Desktop**: Docker Desktop di Windows (lingkungan lokal developer).
- **Linux_Probe**: prosedur pembuktian yang menjalankan Frontend_Image di Linux_Engine sebelum perbaikan.
- **Upstream_Check**: permintaan `wget`/`curl` (dari host atau dari container, tanpa browser) ke nginx frontend yang membuktikan sebuah path sampai ke upstream yang benar.
- **Image_Check**: `scripts/ci/check-image.sh` (gagal bila image berjalan sebagai root atau tanpa HEALTHCHECK).
- **FeImageJob**: job baru di CI_Workflow yang membuild Frontend_Image *(D6)*.
- **CI_Workflow**: `.github/workflows/ci.yml`.
- **Dev_Proxy**: `frontend/CompanyPortal-Vite/vite.config.ts`, `frontend/VendorPortal-Vite/vite.config.ts`, `frontend/dev.ps1`.
- **DeploymentDoc**: `.claude/docs/deployment.md`.
- **DevelopmentPlan**: `.kiro/steering/development-plan.md`.

## Requirements

### Requirement 1: Bukti masalah #1 di Linux sebelum perbaikan

**User Story:** Sebagai developer, saya ingin bukti bahwa proxy `/api` gagal di Docker Engine Linux sebelum konfigurasi diubah, supaya perbaikan menjawab masalah nyata dan bukan dugaan.

#### Acceptance Criteria

1. WHEN belum ada file frontend yang diubah, THE Linux_Probe SHALL membuild Frontend_Image dari HEAD, menjalankannya di Linux_Engine bersama Backend_API dan EXQ_API di network `cms-net`, lalu mencatat status HTTP dan baris error log nginx untuk `/api/`, `/api/v1/exports/` (CP), dan `/api/eod/` (CP).
2. THE Linux_Probe SHALL mencatat, di dalam container frontend di Linux_Engine, apakah `host.docker.internal` bisa di-resolve oleh (a) `getent hosts` tanpa `extra_hosts`, (b) `getent hosts` dengan `extra_hosts: host.docker.internal:host-gateway`, dan (c) nginx lewat `resolver 127.0.0.11` + `proxy_pass $variable` dengan `extra_hosts` yang sama.
3. THE Linux_Probe SHALL menjalankan langkah yang sama di Docker_Desktop sebagai pembanding.
4. THE Linux_Probe SHALL menyimpan perintah dan hasilnya di folder spec sebelum task perbaikan dimulai.
5. IF Linux_Probe menunjukkan `/api/` CP dan VP berfungsi di Linux_Engine tanpa perubahan, THEN pekerjaan perbaikan SHALL berhenti dan hasilnya dilaporkan ke PO sebelum Requirement 2 dikerjakan.

### Requirement 2: Upstream nginx berfungsi di Linux_Engine dan Docker_Desktop *(D1, D7)*

**User Story:** Sebagai operator, saya ingin CP dan VP meneruskan `/api` ke backend yang benar di VM Linux maupun di laptop, supaya konfigurasi yang sama bisa dipakai di dua tempat.

#### Acceptance Criteria

1. WHEN CP berjalan dan Backend_API sudah up, THE CP_Nginx SHALL meneruskan `/api/` (selain `/api/eod/` dan `/api/v1/exports/`) ke Backend_API, sehingga `GET /api/v1/auth/me` tanpa token dijawab oleh Backend_API (401 dengan body JSON Go), bukan 502/504 dari nginx, di Linux_Engine dan Docker_Desktop.
2. WHEN VP berjalan dan Backend_API sudah up, THE VP_Nginx SHALL memenuhi kriteria 2.1 untuk `/api/`.
3. WHEN CP berjalan dan EXQ_API sudah up, THE CP_Nginx SHALL tetap meneruskan `/api/v1/exports/` ke EXQ_API lewat `cms-net` dengan `proxy_buffering off` dan `proxy_read_timeout 300s`.
4. WHEN CP berjalan dan EOD_Service berjalan di host, THE CP_Nginx SHALL meneruskan `/api/eod/<path>` ke EOD_Service `/<path>` (prefix dibuang), sehingga `GET /api/eod/health` dijawab EOD_Service, di Linux_Engine dan Docker_Desktop.
5. IF Backend_API, EXQ_API, atau EOD_Service belum up saat container frontend start, THEN THE CP_Nginx dan VP_Nginx SHALL tetap start dan healthy, dan path yang upstream-nya mati dijawab 502 (perilaku sekarang, tidak crash saat start).
6. THE CP_Nginx dan VP_Nginx SHALL tetap mengirim header `Host`, `X-Real-IP`, `X-Forwarded-For` (`$proxy_add_x_forwarded_for`), `X-Forwarded-Proto`, dan `proxy_cookie_path /api/v1/auth /api/v1/auth` seperti sekarang.
7. THE CP_Nginx dan VP_Nginx SHALL memakai konfigurasi upstream yang sama untuk lokal dan produksi; bila nilainya harus berbeda, nilai itu datang dari env dengan default lokal.

### Requirement 3: Alur lokal tidak berubah

**User Story:** Sebagai developer, saya ingin alur lokal yang sekarang tetap jalan tanpa langkah tambahan, supaya perbaikan deploy tidak mengganggu kerja harian.

#### Acceptance Criteria

1. WHEN `docker compose -f backend/docker-compose.yaml up -d` sudah jalan lalu `docker compose -f frontend/docker-compose.yml up -d --build` dijalankan di Docker_Desktop tanpa env tambahan dan tanpa file `.env` baru, THE Base_Compose SHALL menjalankan CP di `http://localhost:3001` dan VP di `http://localhost:3002`, keduanya berstatus `healthy`.
2. WHEN kondisi 3.1 terpenuhi, THE CP_Nginx dan VP_Nginx SHALL memenuhi Requirement 2.1, 2.2, dan 2.3 di Docker_Desktop.
3. THE Base_Compose SHALL memberi default lokal untuk setiap variabel env baru, sehingga `docker compose -f frontend/docker-compose.yml config` berhasil tanpa env apa pun.
4. THE Dev_Proxy SHALL tidak berubah (`git diff` kosong untuk ketiga file), dan `./dev.ps1 company` serta `./dev.ps1 vendor` tetap memakai proxy Vite.

### Requirement 4: Image bertag dan diambil dari registry di produksi *(D3)*

**User Story:** Sebagai operator, saya ingin VM menjalankan image dengan tag git SHA dari Artifact Registry, supaya deploy bisa diulang dan rollback cukup dengan tag sebelumnya.

#### Acceptance Criteria

1. THE Base_Compose SHALL membentuk nama image dari env: prefix registry (default kosong) dan `IMAGE_TAG` (default `local`), dengan nama image per portal yang sama dengan sekarang (`companyportal-vite-fe`, `vendorportal-vite-fe`).
2. IF Prod_Override dipakai tanpa `IMAGE_TAG` atau tanpa prefix registry, THEN `docker compose … config` SHALL gagal dengan pesan yang menyebut variabel yang kurang, sehingga tag `latest` atau `local` tidak pernah terpakai di produksi.
3. WHEN Prod_Override dipakai, THE Prod_Override SHALL membuat VM menarik image dari registry dan tidak membuild image di VM.
4. THE DeploymentDoc SHALL mendokumentasikan perintah build dan push Frontend_Image dengan tag git SHA (dari host operator), termasuk build arg `VITE_APP_ENV` CP yang tertanam di bundle.

### Requirement 5: Base image ter-pin *(D2)*

**User Story:** Sebagai developer, saya ingin versi base image tetap, supaya perubahan upstream (seperti bug HEALTHCHECK IPv6 2026-11-03) tidak masuk diam-diam.

#### Acceptance Criteria

1. THE Frontend_Image SHALL memakai stage serve `nginx` dengan versi persis menurut D2, bukan tag bergerak `nginx:alpine`.
2. THE Frontend_Image SHALL memakai stage builder `node` dengan versi persis menurut D2, bukan tag bergerak `node:22-alpine`.
3. WHEN Frontend_Image dibuild, THE Frontend_Image SHALL lolos Image_Check, dan HEALTHCHECK tetap memakai `http://127.0.0.1:80/` (regresi bugfix 2026-11-03).
4. WHEN container Frontend_Image dijalankan, `nginx -v` SHALL mencetak versi yang di-pin.

### Requirement 6: Binding port dan terminasi TLS *(D4)*

**User Story:** Sebagai operator, saya ingin port frontend di VM hanya bisa dicapai dari load balancer dan HTTPS berakhir di tempat yang jelas, supaya portal tidak terbuka lewat HTTP polos.

#### Acceptance Criteria

1. THE Base_Compose SHALL tetap mem-publish CP di `3001` dan VP di `3002` untuk lokal.
2. WHEN Prod_Override dipakai, `docker compose … config` SHALL menunjukkan tepat satu mapping port per service dengan alamat bind menurut D4 (mapping dasar diganti, tidak ditambah).
3. THE CP_Nginx dan VP_Nginx SHALL tetap `listen 80` di dalam container bila D4 memilih terminasi TLS di luar container.
4. THE DeploymentDoc SHALL menyebut lokasi terminasi TLS, sumber sertifikat, aturan firewall yang membatasi sumber ke port frontend, dan alasan HTTPS wajib (cookie refresh `Secure`, HSTS).

### Requirement 7: Satu compose frontend *(D5)*

**User Story:** Sebagai developer, saya ingin hanya ada satu compose frontend, supaya tidak ada konfigurasi kedua yang port-nya bentrok dan tidak bisa mencapai backend.

#### Acceptance Criteria

1. THE VP_Standalone_Compose SHALL dihapus.
2. WHEN VP_Standalone_Compose dihapus, rujukan aktif ke file itu SHALL diperbarui atau diberi catatan; rujukan sejarah (`bugfixes.md`, `.kiro/specs/.done/**`) dibiarkan.
3. WHEN `docker compose -f frontend/docker-compose.yml config --services` dijalankan, hasilnya SHALL memuat service CP dan VP.

### Requirement 8: CI membuild image frontend *(D6)*

**User Story:** Sebagai developer, saya ingin CI membuild image CP dan VP di setiap push dan PR, supaya Dockerfile atau `nginx.conf` yang rusak ketahuan sebelum deploy.

#### Acceptance Criteria

1. WHEN ada push atau pull request ke `main` atau `dev1`, THE CI_Workflow SHALL menjalankan FeImageJob untuk CP dan VP, dan kegagalan satu portal tidak membatalkan yang lain.
2. THE FeImageJob SHALL membuild setiap image dengan context folder portal, sama seperti Base_Compose.
3. THE FeImageJob SHALL menjalankan Image_Check pada setiap image.
4. THE FeImageJob SHALL menjalankan `nginx -t` di dalam image yang dibuild dan gagal bila konfigurasi tidak valid.
5. THE FeImageJob SHALL tidak mem-push image, tidak memakai secret, dan hanya memakai izin `contents: read`.

### Requirement 9: Dokumentasi deploy frontend

**User Story:** Sebagai operator, saya ingin langkah deploy frontend tertulis lengkap, supaya deploy pertama ke VM tidak bergantung pada ingatan developer.

#### Acceptance Criteria

1. THE DeploymentDoc SHALL memuat bagian deploy frontend: prasyarat (network `cms-net` dan backend sudah up, versi Docker Compose minimum bila Prod_Override memerlukannya), tabel env (nama, default lokal, nilai produksi), perintah `pull` dan `up` dengan Prod_Override, langkah verifikasi tanpa browser, dan rollback ke tag sebelumnya.
2. THE DeploymentDoc SHALL menjelaskan nilai `TRUSTED_PROXIES` produksi `backend` dan `backend-exq` sesuai jalur nginx → backend yang dipilih di D1 dan jalur LB → nginx di D4, beserta dampaknya bila salah (batas login per IP bersama, `audit_logs.ip` = IP proxy), dan mengganti teks lama yang tidak lagi berlaku.
3. THE DeploymentDoc SHALL menjelaskan nilai `RETRY_TRUSTED_PROXIES` EOD_Service sesuai jalur `/api/eod/` yang dipilih di D1.
4. THE DeploymentDoc SHALL memperbarui paragraf "EOD monitoring routing" dan "Pipeline" (job frontend menurut D6, status `cloudbuild.yaml` menurut D3).
5. THE DevelopmentPlan SHALL memuat item X.15 untuk spec ini dengan status dan langkah berikutnya.

### Requirement 10: Verifikasi tanpa browser

**User Story:** Sebagai PO, saya ingin bukti otomatis atau berbasis perintah bahwa konfigurasi baru bekerja, supaya cek browser manual hanya menguji alur pengguna.

#### Acceptance Criteria

1. WHEN perbaikan selesai, `docker compose config` SHALL berhasil untuk Base_Compose saja dan untuk Base_Compose + Prod_Override (dengan env produksi diisi), dan SHALL gagal untuk Base_Compose + Prod_Override tanpa `IMAGE_TAG`.
2. WHEN perbaikan selesai, build kedua Frontend_Image SHALL berhasil, dan kedua container SHALL berstatus `healthy` di Linux_Engine dan Docker_Desktop.
3. WHEN container berjalan, Upstream_Check SHALL membuktikan di Linux_Engine dan Docker_Desktop: `/` = 200 (CP, VP), `/api/v1/auth/me` dijawab Backend_API (CP, VP), `/api/v1/exports/…` dijawab EXQ_API (CP), dan `/api/eod/health` dijawab EOD_Service (CP).
4. THE hasil 10.1–10.3 SHALL dicatat di folder spec beserta perintahnya.
5. THE cek browser manual (login CP dan VP, halaman EOD Monitoring, satu unduhan ekspor, IP di Log Audit) SHALL ditulis sebagai outstanding untuk user (Golden Rule #10).
6. THE perubahan SHALL di-commit hanya setelah user menulis "commit".

### Requirement 11: Batas perubahan

**User Story:** Sebagai PO, saya ingin perubahan terbatas pada konfigurasi deploy frontend dan dokumentasinya, supaya stack tetap dan risiko kecil.

#### Acceptance Criteria

1. THE perubahan SHALL tidak menyentuh `frontend/*/src/**`, Dev_Proxy, `security-headers.conf`, `backend/`, `backend-exq/`, `backend_python/`, dan `pkg/`, kecuali disetujui di Open Questions.
2. THE perubahan SHALL tidak menambah tool, library, atau base image baru ke repo selain versi ter-pin dari image yang sudah dipakai (`nginx`, `node`); `docker:dind` hanya dipakai untuk verifikasi lokal dan tidak masuk repo atau CI.
3. THE header keamanan (CSP, HSTS, `X-Frame-Options`, `nosniff`, `Referrer-Policy`) dan aturan cache SPA/aset di CP_Nginx dan VP_Nginx SHALL tidak berubah.

## Open Questions

✅ **Semua DIPUTUSKAN PO 2026-11-04: mengikuti saran Kiro** (jawaban "setuju").

### D1: Upstream nginx → backend ✅

| Opsi | Cara | Dampak `TRUSTED_PROXIES` | Catatan |
|---|---|---|---|
| A | `extra_hosts: host.docker.internal:host-gateway` di Base_Compose, nginx tetap ke `host.docker.internal:8080/8091` | Backend melihat koneksi dari gateway Docker (lewat port host yang di-publish). Nilai = IP gateway, diukur dari log `remote_ip`. | Port 8080 harus di-publish di alamat yang bisa dicapai dari bridge Docker (tidak bisa hanya `127.0.0.1`), jadi butuh firewall VPC. Resolver nginx (`127.0.0.11` + variabel) kemungkinan tidak membaca `/etc/hosts` yang diisi `extra_hosts`, jadi blok upstream mungkin harus jadi `proxy_pass` statis (di-resolve saat start); Linux_Probe membuktikan ini. |
| B | `/api/` → `backend:8080` lewat `cms-net` (sama seperti ekspor); `/api/eod/` tetap ke host | Backend melihat IP container CP/VP di `cms-net`. Nilai = subnet `cms-net`, sama dengan `backend-exq`, satu nilai untuk dua backend Go. | Port 8080 tidak perlu di-publish di produksi. Konfigurasi sama untuk lokal dan prod. Lokal "nginx container + backend `go run` di host" tidak lagi didukung (alur lokal yang diwajibkan memakai backend compose, jadi tidak terdampak). EOD masih perlu jalur ke host. |

**Saran Kiro: B untuk `/api/`, ditambah `extra_hosts: host.docker.internal:host-gateway` khusus untuk `/api/eod/`.** `host-gateway` juga didukung Docker Desktop, jadi lokal tetap jalan. Bila Linux_Probe membuktikan resolver nginx tidak membaca `/etc/hosts`, blok `/api/eod/` memakai `proxy_pass` statis ke `host.docker.internal:8091` (alamat gateway selalu ada, jadi start tidak gagal), sedangkan blok `cms-net` tetap memakai resolver + variabel agar nginx tidak crash bila backend belum up (Req 2.5). Dampak: `TRUSTED_PROXIES` `backend` berubah dari "gateway Docker" menjadi subnet `cms-net`; `RETRY_TRUSTED_PROXIES` EOD = IP gateway yang dilihat EOD_Service.

### D2: Versi pin base image ✅

- Opsi: tag persis (`nginx:1.31.8-alpine3.24`) atau tag + digest (`nginx:1.31.8-alpine3.24@sha256:…`). Pertanyaan tambahan: tetap di mainline 1.31 (yang dipakai `nginx:alpine` sekarang) atau pindah ke stable.
- **Saran Kiro: tag persis + digest, tetap mainline 1.31.8** (versi yang sudah berjalan, jadi tidak ada perubahan perilaku), dan builder `node:22.23.4-alpine3.24@sha256:…`. Digest menjamin build identik; harganya, patch keamanan harus di-bump manual (belum ada Dependabot/Renovate). Tag persis yang tersedia dicek di task sebelum ditulis.

### D3: Tag image, registry, dan `cloudbuild.yaml` ✅

- **Saran Kiro:** env `FE_REGISTRY` (prefix, mis. `asia-southeast2-docker.pkg.dev/<project>/<repo>/`, default kosong) dan `IMAGE_TAG` (default `local`; wajib di Prod_Override). Nama image tetap seperti sekarang; ketidakkonsistenan `userportal-vite-fe` vs `companyportal-vite-fe` dibiarkan (hanya `container_name` dan label, tidak memengaruhi deploy).
- **`cloudbuild.yaml` di luar spec ini.** Isinya butuh project ID, region, nama repository Artifact Registry, dan service account yang belum ada, dan sebaiknya mencakup keempat image (`backend`, `backend-exq`, CP, VP) sekaligus. Spec ini mendokumentasikan build + push manual dari host operator dengan tag `git rev-parse --short HEAD`.

### D4: Binding port dan terminasi TLS ✅

- Opsi a: TLS di GCP external Application Load Balancer (sertifikat Google-managed, Cloud Armor #11 dipasang di LB), VM tanpa IP publik, nginx tetap HTTP :80 di container.
- Opsi b: TLS di reverse proxy di host VM, port container bind `127.0.0.1`. Ini menambah komponen dan pengelolaan sertifikat manual.
- **Saran Kiro: opsi a.** Konsekuensi: LB menghubungi VM lewat IP internal VM, jadi bind `127.0.0.1` tidak bisa dicapai LB. Port di-publish di alamat dari env (`FE_BIND_ADDR`, default lokal `0.0.0.0`, prod = IP internal VM), dan firewall VPC hanya mengizinkan rentang LB/health check Google (35.191.0.0/16, 130.211.0.0/22, dicek ulang ke dokumentasi GCP saat design). `TRUSTED_PROXIES` backend perlu ditambah rentang tersebut (dan alamat LB bila muncul di `X-Forwarded-For`); nilai persisnya diverifikasi dari log `remote_ip` saat deploy pertama.

### D5: Nasib `VendorPortal-Vite/docker-compose.yml` ✅

- **Saran Kiro: hapus.** File itu duplikat service VP di Base_Compose, port default-nya bentrok dengan CP, dan tanpa `cms-net` proxy `/api` tidak bisa mencapai backend di Linux. Rujukan di `gke-deploy` (on hold) diberi catatan.

### D6: Job CI frontend ✅

- **Saran Kiro: job image (build CP + VP, Image_Check, `nginx -t`) masuk spec ini**, karena itu yang menjaga kesiapan deploy. Job lint/test/coverage/build frontend tetap item X.3 (spec `fe-tech-debt`).

### D7: Topologi VM frontend ✅

- Opsi: frontend di VM yang sama dengan backend (sesuai compose sekarang) atau VM terpisah (sesuai baris #1/#2 tabel `deployment.md`).
- **Saran Kiro: VM yang sama untuk sekarang.** Compose frontend sudah bergantung pada `cms-net` dan ekspor sudah lewat `cms-net`. VM terpisah membuat `cms-net` dan `host.docker.internal` tidak bisa dipakai; upstream harus dari env (IP internal VM backend, lewat template `envsubst` bawaan image nginx) dan scope spec ini membesar. Bila PO memilih VM terpisah, D1 ditulis ulang. Tabel `deployment.md` diperjelas sesuai keputusan.
