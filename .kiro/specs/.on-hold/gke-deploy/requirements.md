# Requirements Document

> **Status: ⏸ ON HOLD (user, 2026-11-04).** GKE belum diperlukan; CROWN tetap berjalan di Compute Engine sesuai `.claude/docs/deployment.md` dan CLAUDE.md Sec 10. Spec ini disimpan untuk dipakai bila nanti pindah ke GKE. Jangan dikerjakan sampai user memintanya lagi. Saat dilanjutkan, cek ulang kondisi kode (Dockerfile, `nginx.conf`, `.env.example`, nomor migrasi) karena bisa sudah berubah.

> Status: **requirements disetujui PO 2026-11-04** (D1–D31 mengikuti saran Kiro; detail yang butuh tim infra dikonfirmasi tim infra). Design: `design.md`. Dipindahkan dari `.claude/sdlc/gke-deploy/intent.md` (draft 2026-11-04).
> Disesuaikan saat menulis design (2026-11-04): Req 7.4 dan D14 memakai uid/gid numerik yang sama (10001:10001) untuk pod Go `backend` dan Python, bukan `fsGroup`, karena `fsGroup` tidak mengubah kepemilikan file di volume NFS (Filestore). Req 9.8 memakai nilai `TRUSTED_PROXIES` Go untuk `RETRY_TRUSTED_PROXIES`, sesuai `.env.example` scheduler, karena request ke scheduler melewati load balancer dan Nginx yang sama.
> Ditambahkan 2026-11-04 saat menyusun tasks: Req 5.8 (penyebaran pod per node/zona) dan Req 5.9 (perkiraan kebutuhan node). Tanpa 5.8, dua pod satu komponen bisa ditaruh di node yang sama, sehingga pod backup ikut mati bersama node itu.
> Item rencana: X.14 di `.kiro/steering/development-plan.md`.

## Introduction

### Masalah

Tim infra menyarankan CROWN pindah dari Compute Engine (GCE) ke Google Kubernetes Engine (GKE) (user, 2026-11-04). Keputusan deploy yang berlaku sekarang (CLAUDE.md Sec 10 dan Sec 12 "Backend topology", `.claude/docs/deployment.md`):

- Satu VM Compute Engine menjalankan `backend` (8080) sebagai container docker compose. `backend-exq` (8081) ikut di VM yang sama mulai rilis ekspor (X.7–X.9). Rencana 2028: satu VM per backend.
- Frontend (CompanyPortal, VendorPortal) di Compute Engine, dengan Nginx di dalam image.
- Python EOD (`eod_retry_scheduler`, `service_dsr_etl`, skrip ETL) di VM yang sama, membaca `FTP_DATA` dari bind mount host.
- Migrasi dijalankan dari host operator (`go run ./cmd/migrate`, butuh Go dan `psql`), di luar 07:00–20:00 WIB.
- Dokumen menulis CI = GitHub Actions dan CD = Cloud Build. Menurut user, CI/CD sebenarnya **Jenkins** (spec `.claude/sdlc/.done/ci-jenkins/` masih on hold).

Sebagian besar image sudah siap untuk K8s. Yang belum siap adalah konfigurasi yang terikat ke Docker di satu host (DNS Docker, `host.docker.internal`, bind mount, network `cms-net`), dan dokumen deploy yang ditulis untuk VM.

### Kondisi kode saat ini (dicek 2026-11-04)

**Sudah siap untuk K8s**
- Semua image multi-stage dan non-root: `backend` dan `backend-exq` (`appuser`, Alpine 3.24), `backend_python` (`app`, uid 10001), kedua frontend (`appuser` di `nginx:alpine`). Semua punya `HEALTHCHECK` Docker.
- `GET /health` (liveness) dan `GET /ready` (db, db_replica, redis; 200/503) di kedua backend Go.
- JWT blacklist dan rate limit disimpan di Redis (`pkg/auth`, `pkg/middleware`), bukan di memori pod.
- Worker email `notification` (`backend/queries/notifications.sql` `ClaimPendingNotificationEmails`) dan worker ekspor (`backend-exq/queries/export_jobs.sql`) mengambil baris dengan `FOR UPDATE SKIP LOCKED`, jadi aman untuk lebih dari satu replica.
- Nomor tiket replenish, tiket vault, kode paket, batch impor, dan dedupe ekspor memakai `pg_advisory_xact_lock`.
- Graceful shutdown pada SIGTERM: `backend` menunggu sampai 15 s (`srv.Shutdown`). `backend-exq` menunggu sampai 15 s untuk HTTP, lalu memberi job ekspor yang sedang berjalan waktu sampai 10 s (`workerShutdownGrace`).
- Config dibaca dari env (`pkg/config`, `getenvDefault` di `main.go`, pydantic-settings di Python).
- `backend-exq` menyimpan file ekspor di DB (`export_job_chunks`), bukan di disk.

**Harus diubah**
1. **Nginx frontend terikat ke Docker.** CompanyPortal `nginx.conf`: `/api/eod/` → `host.docker.internal:8091`, `/api/` → `host.docker.internal:8080`, `/api/v1/exports/` → `backend-exq:8081` (nama container di `cms-net`). VendorPortal `nginx.conf`: `/api/` → `host.docker.internal:8080`. Semua blok memakai `resolver 127.0.0.11` (DNS internal Docker). Kedua image `listen 80` dengan `USER appuser`. Di Docker ini jalan karena Docker mengizinkan user non-root membuka port < 1024 di dalam container; di K8s hal itu tidak dijamin.
2. **`FTP_DATA` dipakai bersama.** Go API menulis upload DSR ke `DSR_UPLOAD_DIR` (default `FTP_DATA/DSR`), lalu memanggil `service_dsr_etl` lewat `DSR_RETRY_SCHEDULER_BASE_URL`. `dsr_etl.py` memindahkan file ke `pending_confirmation/` saat dry-run dan ke `backups/` saat commit (`shutil.move`). ETL DMAA dan ITM juga membaca lalu memindahkan file.
3. **Migrasi butuh `psql`.** `cmd/migrate` menjalankan `psql -f` per file (`runner.go`) dan berhenti bila `psql` tidak ada di `PATH`. Image runtime `backend` hanya berisi binary `api` dan folder `migrations/`. Binary `migrate` dan `psql` tidak ada di image mana pun.
4. **Python EOD harus satu replica.** Scan, auto-retry, dan late check di `lib/services/scheduler_service.py` (APScheduler di dalam proses) tidak memakai lock DB. `replenish_classify` dan `dsr_late_notify` memakai advisory lock, tetapi job lain tidak. Dry-run dan commit DSR bergantung pada file di `pending_confirmation/`.

**Temuan tambahan**
- **Path input ETL dihitung dari lokasi skrip**, bukan dari `*_FTP_DATA_ROOT`. `dsr_etl.py` dan `dmaa_etl.py` memakai `Path(__file__).resolve().parents[2] / "FTP_DATA"`, `itm_cashpos_etl.py` memakai `parents[3]`. Di image (`WORKDIR /app`) ketiganya menghasilkan `/FTP_DATA/...`, padahal komentar `backend_python/Dockerfile` menyuruh mount di `/app/FTP_DATA`. `itm_replenish_etl.py` membaca `ITM_INPUT_DIR` dengan default path Windows (`C:\Users\...`).
- **Skrip ETL membaca koneksi DB dari `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS`**, bukan dari `RETRY_DATABASE_URL`/`DSR_ETL_DATABASE_URL`. Skrip dijalankan sebagai subprocess dari pod scheduler (`lib/services/retry_executor.py`), jadi env itu harus ada di pod yang sama.
- **K8s mengabaikan `HEALTHCHECK` Docker.** Probe harus ditulis di manifest.
- **Timeout unduhan ekspor.** CompanyPortal memakai `proxy_read_timeout 300s` dan `proxy_buffering off` untuk `/api/v1/exports/`. Load balancer di depan cluster punya timeout sendiri.
- **`GCS_BUCKET`** tercantum di daftar env CLAUDE.md Sec 9, tetapi tidak ada kode Go atau Python yang memakai GCS.
- **`VITE_APP_ENV=prod`** dimasukkan ke image CompanyPortal saat build (`ARG` di Dockerfile).
- **`.kiro/specs/backend-exq-deploy-readiness/`** (disetujui PO 2026-11-04, job `go-image` + checklist rilis di-commit `cf53038`) masih berasumsi VM + compose. Dampaknya dicatat di `## Dependency & Scope Note`.

### Ruang lingkup

- Manifest K8s untuk enam komponen (`backend`, `backend-exq`, CompanyPortal, VendorPortal, `eod_retry_scheduler`, `service_dsr_etl`), dengan overlay per lingkungan (D2, D4).
- Konfigurasi Nginx frontend yang jalan di GKE dan di compose lokal dengan image yang sama (D7, D8).
- Image dan Job migrasi (D20).
- Volume bersama `FTP_DATA` tanpa mengubah logika ETL (D12, D15).
- Pemetaan env, ConfigMap, dan Secret per komponen, termasuk `TRUSTED_PROXIES` dan ukuran pool DB (D16–D19).
- Runbook deploy, rollback, dan cutover VM → GKE di `deployment.md` (D25, D31).
- Pembaruan dokumen yang bertentangan (CLAUDE.md, steering, `deployment.md`, catatan di spec terkait).

### Tidak termasuk

- Membuat cluster GKE, VPC, Filestore, Memorystore, Cloud SQL, load balancer, sertifikat, Cloud Armor, IAM, dan Secret Manager. Itu pekerjaan tim infra; spec ini hanya menulis syaratnya.
- Pipeline Jenkins (tetap di spec `ci-jenkins`, D25). Spec ini hanya menyediakan perintah deploy yang nanti dipanggil pipeline.
- Memindahkan `FTP_DATA` ke GCS (D12). Itu butuh dependency baru dan perubahan kode Go + Python.
- Mengubah cara skrip ETL menghitung folder input (D15).
- Memecah skrip ETL menjadi CronJob K8s (D21).
- Autoscaling (HPA) (D17).
- Perubahan logika aplikasi, skema, atau migrasi DB baru.
- Cek manual UI setelah deploy (tugas user, GR#10).

### Area sensitif (project-context Sec 3 rule 7)

- **Migrasi data:** cara menjalankan `cmd/migrate` berubah dari host operator ke Job di cluster. Urutan `status` → `up` → `status`, advisory lock, dan larangan di jam 07:00–20:00 tetap.
- **Auth dan audit:** `TRUSTED_PROXIES` menentukan IP di `audit_logs` dan batas login per IP (`RATE_LIMIT_IP`). Nilai yang salah membuat semua user tercatat dengan IP proxy dan bisa mengunci login massal (QC F13). `JWT_SECRET` harus sama di `backend`, `backend-exq`, dan `RETRY_AUTH_SECRET`.
- **File ingest:** `shutil.move` pada volume bersama. Mount yang salah atau dua scheduler aktif bersamaan bisa membuat file DSR/DMAA/ITM hilang, tidak diproses, atau diproses dua kali.
- Tidak ada perubahan logika uang, rekonsiliasi, atau maker-checker.

## Glossary

- **GKE_Cluster**: cluster GKE tempat CROWN berjalan, dimiliki tim infra (D1).
- **Environment**: lingkungan `dev`, `uat`, atau `prod`, masing-masing satu namespace K8s (D2).
- **Manifest_Set**: manifest K8s di repo ini, berupa base dan overlay per Environment (D4). Lokasi diusulkan `deploy/k8s/`.
- **Backend_Workload**: Deployment + Service untuk `backend` (port 8080).
- **Exq_Workload**: Deployment + Service untuk `backend-exq` (port 8081).
- **CP_Workload**: Deployment + Service untuk CompanyPortal (Nginx + file statis).
- **VP_Workload**: Deployment + Service untuk VendorPortal (Nginx + file statis).
- **Scheduler_Workload**: Deployment + Service untuk `eod_retry_scheduler` (port 8091), termasuk skrip ETL DMAA/ITM yang dijalankan sebagai subprocess.
- **DsrEtl_Workload**: Deployment + Service untuk `service_dsr_etl` (port 8090), termasuk `dsr_etl.py` sebagai subprocess.
- **Go_Workload**: Backend_Workload dan Exq_Workload.
- **Frontend_Workload**: CP_Workload dan VP_Workload.
- **Python_Workload**: Scheduler_Workload dan DsrEtl_Workload.
- **Frontend_Nginx_Config**: konfigurasi Nginx di image CompanyPortal dan VendorPortal (`nginx.conf` atau template yang menggantikannya).
- **Upstream_Env**: variabel env yang dibaca Frontend_Nginx_Config saat start untuk alamat backend dan resolver DNS (D7).
- **Shared_Volume**: volume ReadWriteMany berisi `FTP_DATA` (D12).
- **Migration_Image**: image baru berisi binary `migrate`, `psql`, dan folder `backend/migrations/` dari git SHA yang sama.
- **Migration_Job**: Job K8s yang menjalankan Migration_Image terhadap Cloud SQL primary.
- **App_Config**: ConfigMap per komponen per Environment untuk nilai yang bukan rahasia.
- **App_Secret**: Secret K8s per komponen per Environment, diisi dari Secret Manager (D19).
- **Ingress_Internal**: jalur masuk CompanyPortal (D5, D6).
- **Ingress_Vendor**: jalur masuk VendorPortal (D5, D6).
- **Deploy_Runbook**: bagian GKE di `.claude/docs/deployment.md` (deploy, migrasi, rollback, cutover, verifikasi).
- **Local_Compose**: `backend/docker-compose.yaml`, `backend-exq/docker-compose.yaml`, `frontend/docker-compose.yml`.
- **Proxy_Hop**: setiap alamat yang meneruskan request antara browser dan backend (load balancer Google, pod Nginx frontend).

## Requirements

### Requirement 1: Manifest per komponen

**User Story:** Sebagai operator, saya ingin setiap komponen CROWN punya manifest K8s di repo, supaya deploy ke GKE bisa diulang dan ditinjau seperti kode.

#### Acceptance Criteria

1. THE Manifest_Set SHALL berisi satu Deployment dan satu Service untuk setiap Backend_Workload, Exq_Workload, CP_Workload, VP_Workload, Scheduler_Workload, dan DsrEtl_Workload.
2. THE Manifest_Set SHALL berisi base yang sama untuk semua Environment, dan overlay `dev`, `uat`, `prod` yang hanya mengubah namespace, tag image, jumlah replica, resource, App_Config, dan nama App_Secret.
3. THE Manifest_Set SHALL merujuk setiap image dengan tag git SHA dan tidak pernah memakai tag `latest` atau image tanpa tag.
4. THE Manifest_Set SHALL tidak memuat nilai rahasia (password, `JWT_SECRET`, connection string DB, kredensial SMTP/LDAP, `*_AUTH_SECRET`).
5. WHEN overlay mana pun di-render dengan `kubectl kustomize`, THE Manifest_Set SHALL menghasilkan YAML tanpa error.
6. THE Manifest_Set SHALL mengekspos Go_Workload dan Python_Workload hanya sebagai Service `ClusterIP`, tanpa Ingress, NodePort, atau LoadBalancer.

### Requirement 2: Keamanan pod dan resource

**User Story:** Sebagai tim infra, saya ingin pod CROWN berjalan dengan hak minimum dan batas resource yang jelas, supaya cluster bersama tetap aman dan stabil.

#### Acceptance Criteria

1. THE Manifest_Set SHALL menetapkan `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`, dan `seccompProfile.type: RuntimeDefault` untuk setiap container.
2. THE Manifest_Set SHALL menetapkan `resources.requests` (CPU, memori) dan `resources.limits.memory` untuk setiap container.
3. THE Manifest_Set SHALL tidak memakai `hostNetwork`, `hostPath`, atau container `privileged`.
4. THE Manifest_Set SHALL menetapkan `automountServiceAccountToken: false` untuk workload yang tidak memanggil API K8s atau GCP.

### Requirement 3: Nginx frontend di GKE dan di compose lokal

**User Story:** Sebagai developer, saya ingin image frontend yang sama bisa jalan di GKE dan di compose lokal, supaya yang diuji lokal sama dengan yang di-deploy.

#### Acceptance Criteria

1. THE Frontend_Nginx_Config SHALL membaca alamat upstream (`backend`, `backend-exq`, `eod_retry_scheduler`) dan resolver DNS dari Upstream_Env saat container start, bukan dari nilai yang ditulis tetap di image.
2. THE Frontend_Nginx_Config SHALL listen di port 8080 sehingga user non-root bisa membukanya tanpa pengaturan kernel tambahan (D8).
3. WHEN Upstream_Env diisi nama Service K8s, THE CP_Workload SHALL meneruskan `/api/eod/` ke Scheduler_Workload (prefix dibuang), `/api/v1/exports/` ke Exq_Workload, dan `/api/` lainnya ke Backend_Workload, dengan urutan prioritas yang sama seperti `nginx.conf` sekarang.
4. WHEN Upstream_Env diisi nama Service K8s, THE VP_Workload SHALL meneruskan `/api/` ke Backend_Workload.
5. THE Frontend_Nginx_Config SHALL mempertahankan `proxy_buffering off` dan `proxy_read_timeout 300s` untuk `/api/v1/exports/`, `proxy_cookie_path /api/v1/auth`, header `X-Forwarded-For`/`X-Forwarded-Proto`, `security-headers.conf`, aturan cache file statis, dan SPA fallback.
6. WHEN Local_Compose dijalankan dengan nilai Upstream_Env untuk Docker (`host.docker.internal`, `backend-exq`, resolver `127.0.0.11`), THE Frontend_Workload SHALL berperilaku sama seperti sebelum perubahan, dengan port publish disesuaikan ke 8080.
7. IF Upstream_Env yang wajib tidak diisi, THEN THE Frontend_Nginx_Config SHALL gagal start dengan pesan yang menyebut nama variabelnya.

### Requirement 4: Jalur masuk dan jaringan

**User Story:** Sebagai tim keamanan, saya ingin portal internal dan portal vendor punya jalur masuk terpisah, supaya portal internal tidak terbuka ke internet.

#### Acceptance Criteria

1. THE Ingress_Internal SHALL meneruskan trafik hanya ke CP_Workload dan hanya bisa diakses dari jaringan internal perusahaan (D6).
2. THE Ingress_Vendor SHALL meneruskan trafik hanya ke VP_Workload dan dilindungi kebijakan Cloud Armor (D6).
3. THE Ingress_Internal SHALL memakai timeout backend minimal 300 detik supaya unduhan ekspor tidak terputus sebelum `proxy_read_timeout` Nginx (D10).
4. THE Ingress_Internal dan Ingress_Vendor SHALL menghentikan TLS di load balancer dengan sertifikat yang ditetapkan tim infra (D9).
5. THE Ingress_Internal dan Ingress_Vendor SHALL memakai readiness Frontend_Workload sebagai dasar health check load balancer.
6. THE Deploy_Runbook SHALL mencantumkan tujuan egress yang dibutuhkan pod (Cloud SQL, Memorystore, SMTP relay, LDAP, sumber file DMAA/ITM) sebagai syarat untuk tim infra (D11).

### Requirement 5: Probe, ketersediaan, dan shutdown

**User Story:** Sebagai operator, saya ingin K8s tahu kapan pod sehat dan siap menerima trafik, supaya deploy dan gangguan singkat tidak memutus user.

#### Acceptance Criteria

1. THE Go_Workload SHALL memakai `GET /health` sebagai liveness probe dan `GET /ready` sebagai readiness probe (D22).
2. THE Python_Workload SHALL memakai `GET /health` di port service masing-masing sebagai liveness dan readiness probe.
3. THE Frontend_Workload SHALL memakai `GET /` di port 8080 sebagai liveness dan readiness probe.
4. THE readiness probe Go_Workload SHALL mentoleransi gangguan DB/Redis singkat minimal 30 detik sebelum pod dikeluarkan dari Service.
5. THE Manifest_Set SHALL menetapkan `terminationGracePeriodSeconds` lebih besar dari waktu shutdown aplikasi: minimal 30 detik untuk Backend_Workload dan minimal 40 detik untuk Exq_Workload (15 s HTTP + 10 s job ekspor + cadangan).
6. THE overlay `prod` SHALL menjalankan minimal 2 replica untuk Go_Workload dan Frontend_Workload, dengan PodDisruptionBudget `minAvailable: 1` untuk masing-masing (D23).
7. THE Deploy_Runbook SHALL meminta jendela maintenance node GKE di luar 07:00–20:00 WIB dan di luar jendela scheduler EOD 05:00–09:00 WIB (D23).
8. THE Manifest_Set SHALL menyebar pod Go_Workload dan Frontend_Workload ke node dan zona yang berbeda (`topologySpreadConstraints`), sehingga satu node atau satu zona yang mati tidak mematikan semua pod satu komponen.
9. THE Deploy_Runbook SHALL mencantumkan perkiraan kebutuhan node per Environment (jumlah dan tipe node untuk GKE Standard, atau total resource request untuk Autopilot), dihitung dari resource request di Manifest_Set ditambah satu pod rollout dan Job migrasi, dengan syarat prod tetap menampung semua pod saat satu node mati.

### Requirement 6: Python EOD sebagai singleton

**User Story:** Sebagai operator, saya ingin scheduler EOD dan service DSR tidak pernah berjalan dua kali bersamaan, supaya file tidak diproses ganda dan job retry tidak dobel.

#### Acceptance Criteria

1. THE Scheduler_Workload dan DsrEtl_Workload SHALL masing-masing berjalan dengan tepat 1 replica di setiap Environment.
2. THE Scheduler_Workload dan DsrEtl_Workload SHALL memakai strategi update `Recreate`, sehingga pod lama berhenti sebelum pod baru start.
3. THE Python_Workload SHALL berjalan dengan `TZ=Asia/Jakarta`.
4. THE Scheduler_Workload SHALL berjalan dengan `RETRY_AUTH_MODE=jwt` dan `RETRY_AUTH_SECRET` yang sama dengan `JWT_SECRET` Backend_Workload.
5. THE Python_Workload SHALL menerima env koneksi DB yang dibaca skrip ETL (`DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS`) selain `RETRY_*`/`DSR_ETL_*`, dengan nilai yang menunjuk ke Cloud SQL primary.
6. THE Backend_Workload SHALL memanggil DsrEtl_Workload lewat nama Service K8s di `DSR_RETRY_SCHEDULER_BASE_URL`.
7. THE Manifest_Set SHALL tidak menambahkan HorizontalPodAutoscaler atau PodDisruptionBudget yang bisa membuat Python_Workload lebih dari 1 replica.

### Requirement 7: Volume bersama FTP_DATA

**User Story:** Sebagai operator, saya ingin upload DSR, file DMAA, dan file ITM berada di satu volume yang dibaca semua proses yang membutuhkannya, supaya alur file berjalan seperti di VM.

#### Acceptance Criteria

1. THE Shared_Volume SHALL berupa volume ReadWriteMany yang di-mount oleh Backend_Workload, Scheduler_Workload, dan DsrEtl_Workload (D12).
2. THE Manifest_Set SHALL me-mount Shared_Volume di `/FTP_DATA` pada Python_Workload dan mengisi `RETRY_FTP_DATA_ROOT`, `DSR_ETL_FTP_DATA_ROOT`, dan `ITM_INPUT_DIR` sesuai path itu, sehingga skrip ETL dan service membaca folder yang sama tanpa perubahan kode (D15).
3. THE Manifest_Set SHALL me-mount Shared_Volume pada Backend_Workload dan mengisi `DSR_UPLOAD_DIR` ke subfolder `DSR` di volume itu.
4. THE Manifest_Set SHALL menjalankan Backend_Workload dan Python_Workload dengan uid dan gid numerik yang sama (10001:10001), sehingga file dan folder yang dibuat Go bisa dibaca dan dipindahkan Python tanpa mengubah mode file di kode (D14).
5. WHILE Backend_Workload berjalan dengan lebih dari 1 replica, THE dry-run dan confirm upload DSR SHALL tetap berhasil walaupun kedua request dilayani pod Backend_Workload yang berbeda.
6. THE Deploy_Runbook SHALL mencatat bagaimana file DMAA dan ITM masuk ke Shared_Volume (D13).

### Requirement 8: Migrasi lewat Job

**User Story:** Sebagai operator, saya ingin menjalankan migrasi dari cluster dengan image yang sama SHA-nya dengan rilis, supaya saya tidak butuh host khusus dengan Go dan `psql`.

#### Acceptance Criteria

1. THE Migration_Image SHALL berisi binary `migrate` dari `backend/cmd/migrate`, `psql`, dan folder `backend/migrations/` dari git SHA yang sama dengan image `backend` yang dirilis.
2. THE Migration_Image SHALL berjalan sebagai user non-root.
3. THE Migration_Job SHALL menjalankan `migrate status`, lalu `migrate up` hanya bila `status` tidak melaporkan `MISMATCH` atau file pending yang tidak diharapkan, lalu `migrate status` lagi.
4. THE Migration_Job SHALL memakai `restartPolicy: Never` dan `backoffLimit: 0`, sehingga migrasi yang gagal berhenti tanpa diulang otomatis.
5. THE Migration_Job SHALL membaca `DATABASE_URL` primary dari App_Secret dan tidak menulis password ke argumen perintah atau log.
6. THE Migration_Job SHALL dijalankan secara manual oleh operator (atau oleh pipeline atas perintah operator) di luar 07:00–20:00 WIB, sebelum image baru start. THE Backend_Workload SHALL tidak menjalankan migrasi saat start.
7. THE Deploy_Runbook SHALL menjelaskan cara menjalankan Migration_Job, membaca hasilnya, dan langkah bila gagal (fix forward, tanpa down migration).

### Requirement 9: Config, secret, dan IP client

**User Story:** Sebagai operator, saya ingin setiap variabel env punya tempat yang jelas (ConfigMap atau Secret) dan aturan nilai produksinya, supaya deploy tidak gagal karena config salah.

#### Acceptance Criteria

1. THE Manifest_Set SHALL menyediakan App_Config dan App_Secret untuk setiap komponen, dan setiap variabel di `.env.example` komponen itu masuk ke salah satunya.
2. THE App_Secret SHALL diisi dari Secret Manager lewat mekanisme yang ditetapkan tim infra (D19), dan aplikasi tetap membaca nilainya sebagai env tanpa perubahan kode.
3. THE App_Secret Backend_Workload, Exq_Workload, dan Scheduler_Workload SHALL memakai nilai `JWT_SECRET` / `RETRY_AUTH_SECRET` yang sama dalam satu Environment.
4. THE App_Config SHALL memakai Redis Memorystore yang sama untuk Backend_Workload dan Exq_Workload dalam satu Environment (D18).
5. THE App_Config SHALL memakai nilai yang sama untuk `DSR_LATE_START_DATE` (Backend_Workload, Exq_Workload) dan `RETRY_DSR_LATE_START_DATE` (Scheduler_Workload) dalam satu Environment.
6. THE App_Config Backend_Workload dan Exq_Workload SHALL mengisi `TRUSTED_PROXIES` dengan semua Proxy_Hop antara browser dan backend: rentang IP pod Frontend_Workload dan rentang proxy load balancer yang dipakai (D5, D6), sesempit mungkin.
7. WHEN satu request uji dikirim lewat Ingress_Internal setelah deploy, THE Backend_Workload SHALL mencatat IP browser (bukan IP load balancer atau pod Nginx) di log `http_request` dan di `audit_logs`, dan langkah verifikasi ini SHALL tercantum di Deploy_Runbook.
8. THE App_Config Scheduler_Workload SHALL mengisi `RETRY_TRUSTED_PROXIES` dengan nilai yang sama seperti `TRUSTED_PROXIES` Go_Workload, karena request ke scheduler melewati Proxy_Hop yang sama. THE App_Config DsrEtl_Workload SHALL membiarkan `DSR_ETL_TRUSTED_PROXIES` kosong selama service itu hanya dipanggil Backend_Workload.
9. THE App_Config Go_Workload SHALL menghubungi Cloud SQL lewat cara yang ditetapkan di D16, dengan TLS ke primary dan replica.

### Requirement 10: Ukuran pool DB

**User Story:** Sebagai DBA, saya ingin jumlah koneksi dari semua pod tidak melebihi batas Cloud SQL, supaya scale-out pod tidak membuat koneksi DB ditolak.

#### Acceptance Criteria

1. THE Deploy_Runbook SHALL menghitung `pool_max_conns` per Go_Workload dengan rumus: Σ(jumlah replica maksimum × `pool_max_conns`) + cadangan ≤ `max_connections`, terpisah untuk primary dan replica.
2. THE cadangan SHALL mencakup `superuser_reserved_connections`, Python_Workload beserta subprocess ETL, Migration_Job, dan akses operator.
3. THE overlay setiap Environment SHALL mengisi `pool_max_conns` di `DATABASE_URL` dan `DATABASE_REPLICA_URL` sesuai hitungan itu.
4. WHEN jumlah replica di overlay berubah, THE Deploy_Runbook SHALL meminta `pool_max_conns` dihitung ulang sebelum perubahan dirilis.

### Requirement 11: Log dan metrik

**User Story:** Sebagai app support, saya ingin log dan alert yang sudah dirancang untuk VM tetap jalan di GKE, supaya pemantauan tidak hilang saat pindah.

#### Acceptance Criteria

1. THE Go_Workload dan Python_Workload SHALL menulis log ke stdout dalam format yang sama seperti sekarang, sehingga Cloud Logging GKE menyimpannya sebagai `jsonPayload`.
2. THE Deploy_Runbook SHALL mengganti filter log-based metric `http_request_latency` dan `http_5xx` dari `resource.type="gce_instance"` ke `resource.type="k8s_container"` dengan label container yang sesuai (D27).
3. THE Deploy_Runbook SHALL mempertahankan alert p95 > 3 s dan alert 5xx, dengan catatan bahwa pembuatannya di GCP adalah tugas user/tim infra.

### Requirement 12: Rilis dan rollback

**User Story:** Sebagai operator, saya ingin urutan rilis dan cara rollback yang tertulis, supaya rilis yang gagal bisa dikembalikan tanpa menebak.

#### Acceptance Criteria

1. THE Deploy_Runbook SHALL menetapkan urutan rilis: Migration_Job (bila ada migrasi baru), lalu Backend_Workload, lalu Exq_Workload, lalu Python_Workload, lalu Frontend_Workload.
2. THE Deploy_Runbook SHALL menyatakan bahwa rilis yang mengubah kontrak antar komponen (contoh: vendor-data-export D15) merilis komponen terkait dalam satu jendela rilis.
3. THE Deploy_Runbook SHALL menjelaskan rollback per komponen dengan menerapkan ulang overlay dengan tag git SHA sebelumnya, tanpa rollback migrasi.
4. THE Deploy_Runbook SHALL mencantumkan verifikasi setelah rilis: semua pod `Ready`, `GET /ready` Go_Workload 200 dengan semua check `ok`, satu login lewat setiap Ingress, dan satu ekspor uji lewat Ingress_Internal.
5. THE perintah deploy di Deploy_Runbook SHALL bisa dijalankan manual oleh operator dan dipanggil apa adanya oleh pipeline Jenkins nanti (D24, D25).

### Requirement 13: Cutover dari VM ke GKE

**User Story:** Sebagai PO, saya ingin perpindahan dari VM ke GKE punya langkah yang aman, supaya tidak ada file atau job EOD yang hilang atau berjalan ganda.

#### Acceptance Criteria

1. THE Deploy_Runbook SHALL menjadwalkan cutover di luar 07:00–20:00 WIB dan di luar 05:00–09:00 WIB (D31).
2. THE Deploy_Runbook SHALL menghentikan `eod_retry_scheduler` dan `service_dsr_etl` di VM sebelum Python_Workload di GKE start, sehingga tidak ada dua scheduler aktif pada saat yang sama.
3. THE Deploy_Runbook SHALL menyalin isi `FTP_DATA` dari VM ke Shared_Volume (termasuk `pending_confirmation/` dan `backups/`) dan mencocokkan jumlah file sebelum Python_Workload di GKE start.
4. THE Deploy_Runbook SHALL memindahkan DNS portal ke Ingress_Internal dan Ingress_Vendor hanya setelah verifikasi Req 12.4 lulus di GKE.
5. THE Deploy_Runbook SHALL menyimpan VM dalam keadaan bisa dinyalakan lagi selama periode rollback yang ditetapkan PO, beserta langkah kembali ke VM.

### Requirement 14: Dokumen sesuai keputusan GKE

**User Story:** Sebagai developer dan operator, saya ingin dokumen proyek menggambarkan GKE, supaya tidak ada yang men-deploy berdasarkan petunjuk VM yang sudah tidak berlaku.

#### Acceptance Criteria

1. THE `.claude/docs/deployment.md` SHALL memuat Deploy_Runbook dan mengganti bagian yang menyebut Compute Engine VM, docker compose di VM, `cms-net`, `host.docker.internal`, Cloud Build, dan rencana split VM 2028 untuk produksi.
2. THE `.claude/CLAUDE.md` Sec 9, 10, 12 ("Backend topology"), dan 14 SHALL menyatakan bahwa produksi berjalan di GKE, compose hanya untuk dev lokal, dan CI/CD adalah Jenkins.
3. THE `.claude/docs/decisions.md` SHALL mencatat keputusan pindah ke GKE (tanggal, alasan, keputusan D1–D31 yang disetujui).
4. THE steering `tech.md`, `structure.md`, `project-context.md`, dan `development-plan.md` SHALL diperbarui sesuai Req 14.2.
5. THE spec `.claude/sdlc/.done/ci-jenkins/spec.md` dan `.kiro/specs/backend-exq-deploy-readiness/` SHALL mendapat catatan di kepala dokumen tentang bagian yang digantikan spec ini, tanpa menulis ulang isinya.
6. THE `backend_python/Dockerfile` SHALL memperbaiki komentar path mount `FTP_DATA` sesuai Req 7.2.

### Requirement 15: Non-regresi dan validasi tanpa cluster

**User Story:** Sebagai developer, saya ingin perubahan ini bisa diperiksa sebelum cluster tersedia, dan tidak merusak cara kerja lokal.

#### Acceptance Criteria

1. THE perubahan SHALL tidak mengubah kode Go, kode Python, kode frontend (TypeScript/React), skema DB, atau migrasi.
2. WHEN Local_Compose dijalankan setelah perubahan, THE Backend_Workload, Exq_Workload, dan Frontend_Workload versi compose SHALL start dan lulus healthcheck seperti sebelumnya.
3. THE image `backend`, `backend-exq`, `backend_python`, dan kedua frontend SHALL tetap lulus `scripts/ci/check-image.sh` (non-root + HEALTHCHECK).
4. THE Migration_Image SHALL lulus pemeriksaan non-root, dan `migrate status` dari image itu SHALL berjalan terhadap DB dev lokal.
5. THE validasi SHALL mencakup render semua overlay (Req 1.5) dan grep bahwa Manifest_Set tidak memuat `:latest` atau nilai rahasia (Req 1.3, 1.4).
6. THE verifikasi di cluster nyata (deploy pertama, login, ekspor uji, IP audit, upload DSR, scheduler EOD) SHALL dicatat sebagai pekerjaan user/tim infra yang masih terbuka.

## Open Questions

Saran Kiro untuk ke-31 open question di intent. Semua disetujui PO 2026-11-04 ("setuju ikuti saran"). Untuk keputusan yang bergantung pada tim infra, saran Kiro dipakai sebagai default di design; bila tim infra menetapkan lain, requirement dan design terkait diubah sebelum task yang menyentuhnya dikerjakan.

| ID | Pertanyaan intent | Saran Kiro | Dipakai di | Status |
|---|---|---|---|---|
| D1 | Q1 Pemilik cluster; Autopilot atau Standard? | Ikut standar tim infra. Bila bebas memilih: GKE Autopilot, karena node dikelola Google dan aturan keamanannya sejalan dengan Req 2. | Glossary, Req 2 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D2 | Q2 Lingkungan dan dev lokal | Satu namespace per Environment (`dev`, `uat`, `prod`). Prod di cluster terpisah dari dev/UAT bila tim infra mengizinkan. Dev lokal tetap docker compose. | Req 1.2, 15.2 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D3 | Q3 Region/zone, cluster private | `asia-southeast2` (Jakarta), cluster private (node tanpa IP publik), supaya data tetap di Indonesia dan node tidak terbuka. | Deploy_Runbook | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D4 | Q4 Format manifest dan lokasinya | Kustomize (base + overlay), karena sudah ada di `kubectl` tanpa alat tambahan. Disimpan di repo ini, `deploy/k8s/`. Ganti ke Helm hanya bila tim infra mewajibkan. | Req 1 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D5 | Q5 Jenis ingress | GKE Ingress (Google Cloud Load Balancer) dengan NEG ke pod, kecuali tim infra punya standar Gateway API atau controller sendiri. | Req 4, 9.6 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D6 | Q6 Internal vs publik | CompanyPortal lewat load balancer internal (hanya jaringan perusahaan). VendorPortal lewat load balancer publik dengan Cloud Armor. | Req 4.1, 4.2 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D7 | Q7 Nginx tetap reverse proxy? | Tetap. Nginx di image frontend tetap meneruskan `/api/*`; alamat upstream dan resolver dibaca dari env saat start (fitur template `envsubst` di image `nginx:alpine`). Ingress hanya meneruskan host ke pod frontend. Routing dan header tidak berubah, dan image yang sama dipakai di compose dan GKE. | Req 3 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D8 | Q8 Port non-root | Ganti ke port 8080 di `nginx.conf`, `EXPOSE`, `HEALTHCHECK`, dan port publish compose (`3001:8080`, `3002:8080`). Lebih sederhana daripada membuka port rendah lewat konfigurasi pod. | Req 3.2, 3.6, 5.3 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D9 | Q9 TLS dan sertifikat | TLS berhenti di load balancer. Sertifikat sesuai standar tim infra (Google-managed untuk domain publik VendorPortal; sertifikat perusahaan untuk domain internal bila diwajibkan). Load balancer ke pod memakai HTTP di dalam VPC. | Req 4.4 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D10 | Q10 Timeout load balancer | 300 detik untuk CompanyPortal, sama dengan `proxy_read_timeout` Nginx untuk ekspor. VendorPortal memakai default. | Req 4.3 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D11 | Q11 Egress ke LDAP, SMTP, sumber file | Tim infra menyediakan jalur dan IP keluar tetap (Cloud NAT) bila tujuan memakai whitelist. Spec ini hanya menulis daftar tujuannya. | Req 4.6 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D12 | Q12 `FTP_DATA`: RWX atau GCS | Volume ReadWriteMany (Filestore) dulu, tanpa mengubah logika kode. GCS menjadi spec terpisah karena butuh dependency baru (GR#1) dan perubahan Go + Python. Catatan: Filestore punya ukuran minimum per instance; biayanya diputuskan tim infra. | Req 7, Tidak termasuk | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D13 | Q13 Cara file DMAA/ITM masuk | Kiro tidak bisa menyimpulkan dari repo; tidak ada kode yang mengunduh file DMAA/ITM. Yang dibutuhkan: satu proses di luar cluster (MFT/SFTP/host lain) yang menulis ke share yang sama dengan Shared_Volume. Perlu jawaban tim DMAA/ITM + infra. | Req 7.6 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D14 | Q14 Izin file di volume bersama | Pod Go `backend` dan Python berjalan dengan uid:gid numerik yang sama (10001:10001; disesuaikan saat design, karena `fsGroup` tidak berlaku untuk volume NFS). Mode file Go tetap `0644`/`0755` (tanpa ubah kode) karena share hanya di-mount pod CROWN. Pengetatan ke `0640`/`0750` (QC F5) menjadi follow-up. | Req 7.4 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D15 | Q15 Path mount `FTP_DATA` | Mount di `/FTP_DATA`, sesuai cara skrip ETL menghitung folder di image, dan isi `*_FTP_DATA_ROOT` + `ITM_INPUT_DIR` ke path itu. Skrip ETL tidak diubah. Komentar Dockerfile diperbaiki. | Req 7.2, 14.6 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D16 | Q16 Koneksi Cloud SQL | Private IP langsung dengan `sslmode=verify-ca` (CA server lewat Secret), tanpa sidecar. Cloud SQL Auth Proxy dipakai bila standar tim infra mewajibkan IAM auth. | Req 9.9 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D17 | Q17 Replica, HPA, `max_connections` | Replica tetap tanpa HPA: prod `backend` 2, `backend-exq` 2, tiap frontend 2, tiap Python 1. Dev/UAT 1 per komponen. HPA ditunda supaya hitungan pool tetap pasti. `max_connections` prod dibaca dari tier yang dipilih tim infra. | Req 5.6, 6.1, 10 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D18 | Q18 Redis per lingkungan | Satu Memorystore per Environment, dipakai bersama semua komponen di Environment itu. | Req 9.4 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D19 | Q19 Secret masuk ke pod | Secret Manager disinkronkan ke K8s Secret (External Secrets Operator, atau mekanisme tim infra) lalu dipasang sebagai env. Aplikasi membaca env, jadi mount sebagai file (CSI) akan butuh perubahan kode. | Req 9.2 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D20 | Q20 Cara migrasi | Migration_Image baru (binary `migrate` + `postgresql-client` + `migrations/`) dan Migration_Job yang dijalankan operator di luar 07:00–20:00. Pipeline boleh memicunya hanya atas perintah operator. | Req 8 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D21 | Q21 ETL subprocess atau CronJob | Tetap subprocess di pod scheduler seperti sekarang (tanpa ubah kode). CronJob menjadi spec terpisah bila nanti dibutuhkan. | Req 6, Tidak termasuk | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D22 | Q22 Probe | Liveness `/health` (restart hanya bila proses macet), readiness `/ready` dengan toleransi ≥ 30 s (pod baru tidak menerima trafik sebelum DB dan Redis terjangkau). | Req 5.1, 5.4 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D23 | Q23 Replica minimum, PDB, jendela upgrade | Prod: 2 replica + PDB `minAvailable: 1` untuk Go dan frontend. Python 1 replica tanpa PDB; jeda singkat saat node upgrade diterima. Jendela maintenance GKE 21:00–04:00 WIB (di luar jam kerja dan di luar scheduler 05:00–09:00). | Req 5.6, 5.7 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D24 | Q24 Cara Jenkins deploy | `kubectl apply -k deploy/k8s/overlays/<env>` dengan tag image di-set per SHA, dijalankan dari agent Jenkins dengan kredensial yang dikelola tim internal. GitOps (Config Sync/Argo CD) bila tim infra sudah memakainya. | Req 12.5 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D25 | Q25 `ci-jenkins` bersama fitur ini? | Tidak. Spec ini menyediakan manifest dan perintah deploy manual. Spec `ci-jenkins` tetap on hold; saat dilanjutkan, FR5 diubah ke perintah Req 12.5. | Req 12.5, 14.5 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D26 | Q26 Registry | Tetap Artifact Registry, tag git SHA, tanpa `latest`. | Req 1.3 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D27 | Q27 Log-based metric | Filter diganti ke `resource.type="k8s_container"` di `deployment.md`. Pembuatan metrik dan alert di GCP tetap tugas user/tim infra. | Req 11 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D28 | Q28 Go-live dan rilis ekspor | Bergantung jadwal cluster. Bila rilis ekspor (X.7–X.9) jatuh sebelum cluster siap, rilis ke VM memakai checklist X.13. Bila cluster sudah siap, rilis langsung ke GKE dengan Deploy_Runbook. | Dependency & Scope Note | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |
| D29 | Q29 Nasib `backend-exq-deploy-readiness` | Dibiarkan apa adanya. Job `go-image` dan `check-image.sh` tetap berguna (Req 15.3), checklist VM tetap berlaku sampai cutover (D28). Spec itu hanya diberi catatan kepala (Req 14.5). | Req 14.5, 15.3 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D30 | Q30 Batas fitur | Manifest, template Nginx + port 8080, Migration_Image + Job, pemetaan env, dan dokumen. Tanpa GCS, tanpa perubahan path ETL di kode, tanpa CronJob, tanpa HPA. | Ruang lingkup, Req 15.1 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran) |
| D31 | Q31 Cara cutover | Jeda layanan terencana di luar jam kerja dan di luar 05:00–09:00 (mis. malam akhir pekan): hentikan scheduler di VM, salin `FTP_DATA`, deploy dan verifikasi di GKE, pindahkan DNS. VM disimpan sebagai jalur rollback selama periode yang ditetapkan PO. | Req 13 | ✅ DIPUTUSKAN PO 2026-11-04 (mengikuti saran); detail infra dikonfirmasi tim infra |

## Dependency & Scope Note

- **Butuh dari tim infra sebelum design selesai:** D1, D3, D5, D9, D11, D16, D19 (dan bagian infra dari D2, D4, D12, D13, D17, D18, D23, D24, D28, D31).
- **`.kiro/specs/backend-exq-deploy-readiness/`** (committed `cf53038`, baris X.13 di development-plan masih menulis "belum di-commit"):
  - Req 1–2 (`go-image`, `check-image.sh`): tetap berlaku. Non-root tetap penting di K8s; HEALTHCHECK hanya dipakai compose lokal karena K8s memakai probe (Req 5).
  - Req 3 dan 4 (`deployment.md` versi VM, checklist rilis `backend-exq` dengan `cms-net`, `docker network inspect`, port `8081:8081`, `docker ps`): berlaku sampai cutover (D28), lalu digantikan Deploy_Runbook (Req 14.1). Aturan yang tetap: migrasi `034`/`035` sebelum image baru, rilis bersama D15, rollback per image, `JWT_SECRET` dan `DSR_LATE_START_DATE` sama.
  - Req 5–6: tidak terdampak.
- **`.claude/sdlc/.done/ci-jenkins/spec.md`** FR5 dan S4 (SSH + `docker compose` ke VM) digantikan Req 12.5 saat spec itu dilanjutkan (D25).
- **Tanpa migrasi DB.** Spec ini tidak memakai nomor migrasi baru.
- **Dependency baru:** tidak ada di kode aplikasi. Alat yang dipakai: `kubectl` (Kustomize bawaan) dan paket `postgresql-client` di Migration_Image. Keduanya disebut di design untuk disetujui (GR#1).
