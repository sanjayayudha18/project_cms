# Implementation Plan

> **Status: ⏸ ON HOLD (user, 2026-11-04).** GKE belum diperlukan; CROWN tetap berjalan di Compute Engine sesuai `.claude/docs/deployment.md` dan CLAUDE.md Sec 10. Spec ini disimpan untuk dipakai bila nanti pindah ke GKE. Jangan dikerjakan sampai user memintanya lagi. Saat dilanjutkan, cek ulang kondisi kode (Dockerfile, `nginx.conf`, `.env.example`, nomor migrasi) karena bisa sudah berubah.
> **Draft belum lengkap:** hanya task 1–5 yang tertulis. Task 6–12 (`render.sh`/`deploy.sh`, validator, checkpoint, Deploy_Runbook, dokumen, verifikasi cluster), `## Notes`, dan `## Task Dependency Graph` belum ditulis. Daftar task di `### Model Split` sudah mencakup semuanya dan bisa dipakai sebagai kerangka.

## Overview

Urutan pengerjaan mengikuti dependensi file:

1. Image yang berubah dulu: frontend (template Nginx + port 8080) dan image migrasi baru. Keduanya bisa diuji lokal dengan Docker tanpa cluster.
2. Manifest base (`deploy/k8s/base/`), lalu overlay `dev`/`uat`/`prod` dan kustomization Job migrasi. Task overlay diserialkan karena semuanya menyentuh `overlays/<env>/kustomization.yaml`.
3. Skrip `render.sh`/`deploy.sh` dan validator `scripts/k8s/check_manifests.py`, lalu checkpoint yang menjalankan semua pemeriksaan tanpa cluster.
4. Dokumen: Deploy_Runbook di `deployment.md` (diserialkan, satu file), lalu CLAUDE.md, steering, `decisions.md`, dan catatan kepala spec terkait.
5. Checkpoint akhir non-regresi lokal, lalu verifikasi di cluster yang dikerjakan user/tim infra.

Pola yang diikuti: `scripts/ci/check-image.sh` + `check-image.test.sh` (skrip bash kecil dengan test), stdlib `unittest` untuk test Python (CLAUDE.md Sec 2a), bahasa Inggris di `deployment.md`. Tidak ada perubahan kode Go/Python/TypeScript, skema, atau migrasi (Req 15.1). Alat: `kubectl` (Kustomize bawaan), `docker`, `bash` (Git Bash/WSL di Windows), Python 3 + PyYAML yang sudah ada di lingkungan `backend_python` (DD5).

Task ditulis mengikuti DD1–DD7 di `design.md`. Nilai yang bergantung pada tim infra (nama host, sertifikat, policy Cloud Armor, rentang IP proxy, PV Filestore, tier Cloud SQL) diisi dengan placeholder bernama jelas (`<INFRA:...>`) dan didaftarkan di Deploy_Runbook; validator tidak menolak placeholder ini.

### Model Split

**Sonnet-runnable**
- 1. Image frontend: template Nginx, cek env, Dockerfile, compose, uji image — Sonnet, Medium
- 2. Image migrasi `backend/Dockerfile.migrate` + cek lokal — Sonnet, Medium
- 3. Manifest base (shared, Go, frontend, Python, kustomization) — Sonnet, Medium
- 4.3 Overlay: replicas, resources, PDB prod — Sonnet, Low (needs Opus 4.2)
- 4.4 Overlay: Ingress, BackendConfig, FrontendConfig, sertifikat — Sonnet, Medium (needs 4.3)
- 4.5 Overlay: `db-budget` — Sonnet, Medium (needs 4.4)
- 6. `render.sh` dan `deploy.sh` — Sonnet, Medium
- 7. Validator manifest + test mutasi — Sonnet, High (7.2 needs Opus 4.1, 4.2 and Opus 5)
- 8. Checkpoint tanpa cluster — Sonnet, Medium (needs 1–7)
- 9.3 Runbook: pool DB, log dan metrik — Sonnet, Low (needs Opus 9.2)
- 10. Dokumen lain (CLAUDE.md, steering, decisions, catatan kepala, komentar Dockerfile, progress) — Sonnet, Low (needs 8)
- 11. Checkpoint akhir non-regresi lokal — Sonnet, Medium (needs 1–10)
- 12. Verifikasi di cluster (user/tim infra, outstanding) — Haiku, Low (Haiku) (needs 11)

**Opus-required**
- 4.1 Overlay: ConfigMap per komponen (`TRUSTED_PROXIES`, `RETRY_AUTH_MODE`, path `FTP_DATA`, env DB skrip ETL) — Opus, High — auth/audit: IP client di `audit_logs` dan rate limit login, file ingest
- 4.2 Overlay: kontrak Secret + anotasi sumber (`JWT_SECRET` = `RETRY_AUTH_SECRET`, `REDIS_URL` sama) — Opus, High — auth: secret JWT dan blacklist harus sama di tiga runtime
- 5. Kustomization Job migrasi — Opus, Medium — live-data migration: gerbang `status && up && status`, tanpa retry
- 9.1 Runbook: prasyarat infra, setup volume, node, egress — Opus, High — file ingest di volume bersama, izin file
- 9.2 Runbook: migrasi, deploy, verifikasi IP, rollback — Opus, High — live-data migration + verifikasi IP audit
- 9.4 Runbook: cutover VM → GKE — Opus, High — live data: salin `FTP_DATA`, cegah dua scheduler aktif
- 9.5 Runbook: pindahkan bagian VM ke "Legacy" — Opus, Medium — dokumen deploy produksi yang masih berlaku sampai cutover

Dependensi lintas grup: Sonnet 4.3 menunggu Opus 4.2 (file `kustomization.yaml` sama); Sonnet 7.2 menunggu Opus 4.1, 4.2, 5 (test me-render overlay dan migrate); Sonnet 9.3 menunggu Opus 9.2 (file `deployment.md` sama); Opus 9.4 menunggu Sonnet 9.3; Sonnet 10 dan 11 menunggu Opus 9.5.
## Tasks

- [ ] 1. Image frontend: template Nginx, cek env wajib, port 8080, uid 10001
  - [ ] 1.1 CompanyPortal: `nginx.conf` → `nginx.conf.template`, skrip cek env, Dockerfile
    - `git mv frontend/CompanyPortal-Vite/nginx.conf frontend/CompanyPortal-Vite/nginx.conf.template`, lalu ubah sesuai design "Template Nginx": `listen 8080`; tiap `resolver 127.0.0.11` → `resolver ${NGINX_LOCAL_RESOLVERS}`; `host.docker.internal:8091` → `${CROWN_EOD_UPSTREAM}`, `backend-exq:8081` → `${CROWN_EXQ_UPSTREAM}`, `host.docker.internal:8080` → `${CROWN_BACKEND_UPSTREAM}`. Direktif lain dan urutan `location` tidak berubah.
    - Baru `frontend/CompanyPortal-Vite/docker-entrypoint.d/05-require-upstream-env.sh` (`#!/bin/sh`, `set -eu`): exit 1 dengan `05-require-upstream-env: <NAME> is required` bila `CROWN_BACKEND_UPSTREAM`, `CROWN_EXQ_UPSTREAM`, atau `CROWN_EOD_UPSTREAM` kosong; exit 1 bila `/etc/nginx/conf.d` tidak bisa ditulis.
    - Dockerfile stage serve sesuai design "Dockerfile frontend": `addgroup -g 10001` / `adduser -u 10001`, hapus `conf.d/default.conf`, `chown` termasuk `/etc/nginx/conf.d`, `ENV NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1` + `NGINX_ENVSUBST_FILTER`, `COPY` template ke `/etc/nginx/templates/default.conf.template`, `COPY --chmod=0755` skrip ke `/docker-entrypoint.d/`, `EXPOSE 8080`, HEALTHCHECK ke `127.0.0.1:8080`. Stage build tidak berubah.
    - _Requirements: 3.1, 3.2, 3.3, 3.5, 3.7_
  - [ ] 1.2 VendorPortal: sama dengan 1.1 untuk `frontend/VendorPortal-Vite/`
    - Template hanya punya `/api/` → `${CROWN_BACKEND_UPSTREAM}`; skrip cek hanya `CROWN_BACKEND_UPSTREAM`.
    - _Requirements: 3.1, 3.2, 3.4, 3.5, 3.7_
  - [ ] 1.3 Compose lokal
    - `frontend/docker-compose.yml`: port `3001:8080` dan `3002:8080`, healthcheck `127.0.0.1:8080`, blok `environment:` dengan nilai compose dari tabel design (`http://host.docker.internal:8080`, `http://backend-exq:8081`, `http://host.docker.internal:8091`).
    - `frontend/VendorPortal-Vite/docker-compose.yml`: port `${PORT:-3001}:8080`, healthcheck 8080, `CROWN_BACKEND_UPSTREAM`.
    - _Requirements: 3.6_
  - [ ] 1.4 `scripts/k8s/test-frontend-nginx.sh` (Property 10, 11)
    - Build kedua image dari folder app; jalankan dengan `--user 10001:10001 --cap-drop ALL --security-opt no-new-privileges`.
    - Kasus: env nilai compose → `docker exec … nginx -T` memuat semua direktif Property 11 dengan urutan `location` sama seperti `nginx.conf` di `HEAD~` (ambil lewat `git show`), perbedaan hanya `listen` dan baris `resolver`/`set $..._upstream`; env nilai GKE (FQDN `*.cms-dev.svc.cluster.local`) → nilai muncul di `set`, tidak ada `host.docker.internal`; resolver = nameserver di `/etc/resolv.conf` container; setiap variabel wajib dikosongkan satu per satu → container exit ≠ 0 dan log memuat nama variabel; `GET /` di 8080 → 200.
    - Jalankan `bash scripts/ci/check-image.sh <image> <app>` untuk kedua image (non-root + HEALTHCHECK).
    - Exit 1 dengan daftar kasus gagal; `trap` menghapus container dan image uji.
    - _Requirements: 3.1, 3.2, 3.5, 3.6, 3.7, 15.3_
  - _Model: Sonnet, Effort: Medium — template + entrypoint mengikuti fitur resmi image `nginx:alpine`; kasus tepi ada di quoting `envsubst` dan urutan skrip `/docker-entrypoint.d/`._

- [ ] 2. Image migrasi
  - [ ] 2.1 `backend/Dockerfile.migrate`
    - Isi sesuai design "`backend/Dockerfile.migrate` (baru)": build `./cmd/migrate` dari root repo (konteks sama dengan `backend/Dockerfile`), runtime `alpine:3.24` + `ca-certificates tzdata` + paket client Postgres 18 (cek nama paket di Alpine 3.24 dulu, mis. `apk search postgresql18-client`), user `10001:10001`, `COPY migrations`, `ENTRYPOINT ["./migrate", "-dir", "migrations"]`, `CMD ["status"]`, `TZ=Asia/Jakarta`. Tanpa HEALTHCHECK.
    - Root `.dockerignore` (allowlist) sudah memuat `backend/`, `pkg/`, `go.work*`, `backend-exq/`; pastikan tidak perlu diubah.
    - _Requirements: 8.1, 8.2_
  - [ ] 2.2 Cek lokal image migrasi
    - `docker build -f backend/Dockerfile.migrate -t cms-backend-migrate:local .` dari root repo.
    - `docker image inspect --format '{{.Config.User}}'` → `10001:10001` (jalankan dari bash, bukan PowerShell 5.1).
    - `docker run --rm -e DATABASE_URL=<dev lokal, host.docker.internal, tanpa password di log> cms-backend-migrate:local status` → `001`–`036` applied, 0 pending, exit 0. `psql --version` di image = 18.x. Tidak menjalankan `up`.
    - _Requirements: 8.1, 8.2, 15.4_
  - _Model: Sonnet, Effort: Medium — Dockerfile mengikuti `backend/Dockerfile`; cek hanya `status` (baca saja) ke DB dev._

- [ ] 3. Manifest base `deploy/k8s/base/`
  - [ ] 3.1 `base/shared/`: PVC `ftp-data` dan NetworkPolicy
    - `ftp-data-pvc.yaml` sesuai design "Shared_Volume": `ReadWriteMany`, `storageClassName: ""`, `volumeName` di-patch overlay.
    - `networkpolicy.yaml` sesuai design "NetworkPolicy": default-deny ingress, izin per pasangan (frontend 8080 dari mana saja; `backend` dari kedua frontend; `backend-exq` dan `eod-retry-scheduler` dari `companyportal`; `service-dsr-etl` dari `backend`).
    - Label `app.kubernetes.io/part-of: crown`, `app.kubernetes.io/name: shared`.
    - _Requirements: 1.6, 7.1_
  - [ ] 3.2 `base/backend/` dan `base/backend-exq/`: Deployment + Service
    - Pod/container security context sesuai design "Pod security dan resource" (`runAsUser`/`runAsGroup` 10001, `runAsNonRoot`, `seccompProfile`, `allowPrivilegeEscalation: false`, `drop: ALL`, `automountServiceAccountToken: false`, `enableServiceLinks: false`); resource dari tabel design.
    - Probe dari tabel design (liveness `/health`, readiness `/ready` period 10 s failure 3, startup period 5 s failure 12); `terminationGracePeriodSeconds` 30 (`backend`) dan 40 (`backend-exq`); RollingUpdate `maxSurge: 1`, `maxUnavailable: 0`; `topologySpreadConstraints` per node (`DoNotSchedule`) dan per zona (`ScheduleAnyway`) sesuai design "Penyebaran pod".
    - `envFrom`: ConfigMap `<komponen>-config` dan Secret `<komponen>-secrets`; volume Secret `cloudsql-ca` di `/etc/cloudsql` (read-only); `backend` me-mount PVC `ftp-data` di `/FTP_DATA`.
    - Image `cms-backend-api:GIT_SHA` / `cms-backend-exq:GIT_SHA`; Service `ClusterIP` port 8080 / 8081.
    - _Requirements: 1.1, 1.6, 2.1, 2.2, 2.3, 2.4, 5.1, 5.4, 5.5, 5.8, 7.3, 7.4, 11.1_
  - [ ] 3.3 `base/companyportal/` dan `base/vendorportal/`: Deployment + Service
    - Security context dan resource sama seperti 3.2 (uid 10001); probe `GET /` di 8080; RollingUpdate dan `topologySpreadConstraints` sama dengan 3.2; `envFrom` ConfigMap saja.
    - Service `ClusterIP` port 8080 dengan anotasi `cloud.google.com/neg: '{"ingress": true}'` dan `cloud.google.com/backend-config` (nama BackendConfig per komponen).
    - _Requirements: 1.1, 1.6, 2.1, 2.2, 2.3, 2.4, 5.3, 5.8_
  - [ ] 3.4 `base/eod-retry-scheduler/` dan `base/service-dsr-etl/`: Deployment + Service
    - Image `cms-eod:GIT_SHA` untuk keduanya, `command` sesuai design "Workload Python"; `replicas: 1`, `strategy: {type: Recreate}`; probe `/health` di 8091 / 8090; env `TZ=Asia/Jakarta`; PVC `ftp-data` di `/FTP_DATA`; Secret `cloudsql-ca`; security context dan resource dari design.
    - Service `ClusterIP` port 8091 / 8090. Tanpa HPA, tanpa PDB, tanpa `topologySpreadConstraints`.
    - _Requirements: 1.1, 1.6, 2.1, 2.2, 2.3, 2.4, 5.2, 6.1, 6.2, 6.3, 6.7, 7.1, 7.2, 7.4, 11.1_
  - [ ] 3.5 `base/kustomization.yaml`
    - `resources:` semua file 3.1–3.4; `labels` `app.kubernetes.io/part-of: crown` (tanpa mengubah selector); setiap objek komponen punya `app.kubernetes.io/name: <komponen>`.
    - `kubectl kustomize deploy/k8s/base` tanpa error.
    - _Requirements: 1.1, 1.3, 1.5_
  - _Model: Sonnet, Effort: Medium — YAML K8s standar, nilai sudah ditetapkan di design; perlu teliti pada security context dan probe._

- [ ] 4. Overlay `dev`, `uat`, `prod`
  - [ ] 4.1 ConfigMap per komponen (`configMapGenerator` dari `overlays/<env>/config/<komponen>.env`)
    - Buat `overlays/<env>/kustomization.yaml` (`resources: ../../base`, `namespace: cms-<env>`, `images:` dengan `newName` registry per env, `configMapGenerator`).
    - Isi setiap key ConfigMap dari tabel design "Kontrak config dan secret": `APP_ENV=production`, `TZ=Asia/Jakarta`, `DSR_UPLOAD_DIR=/FTP_DATA/DSR`, `DSR_RETRY_SCHEDULER_BASE_URL` + `CROWN_*_UPSTREAM` dengan FQDN `*.cms-<env>.svc.cluster.local`, `RETRY_AUTH_MODE=jwt`, `RETRY_FTP_DATA_ROOT`/`DSR_ETL_FTP_DATA_ROOT=/FTP_DATA`, `ITM_INPUT_DIR=/FTP_DATA/ITM/atm_replenish`, `DB_HOST`/`DB_PORT`/`DB_NAME`/`DB_USER`, `PGSSLMODE=verify-ca`, `PGSSLROOTCERT=/etc/cloudsql/server-ca.pem`.
    - `TRUSTED_PROXIES` (`backend`, `backend-exq`) = `RETRY_TRUSTED_PROXIES` = daftar Proxy_Hop dari design (rentang pod `<INFRA:pod-cidr>`, `130.211.0.0/22`, `35.191.0.0/16`, `<INFRA:proxy-only-subnet>`, IP LB `<INFRA:...>/32`); `DSR_ETL_TRUSTED_PROXIES=` kosong.
    - `DSR_LATE_START_DATE` (`backend`, `backend-exq`) = `RETRY_DSR_LATE_START_DATE` per env; `REDIS_URL` tidak di ConfigMap (Secret).
    - Variabel `.env.example` yang dibiarkan default (`RETRY_*_INPUT_DIR` dll.) dicatat sebagai key dengan nilai kosong hanya bila aplikasi memperlakukan kosong = default; bila tidak, cantumkan sebagai pengecualian bernama di validator (task 7.1) dengan alasan.
    - Cek: Go menolak `TRUSTED_PROXIES` yang tidak valid, jadi placeholder `<INFRA:...>` hanya untuk overlay yang belum dipakai; overlay yang di-deploy wajib terisi nilai nyata (dicatat di Runbook 9.1).
    - _Requirements: 6.4, 6.5, 6.6, 7.2, 7.3, 9.1, 9.4, 9.5, 9.6, 9.8, 9.9_
  - [ ] 4.2 Kontrak Secret dan anotasi sumber
    - Deployment merujuk `<komponen>-secrets` dan `cloudsql-ca`; Job migrate merujuk `migrate-secrets` (task 5). Tanpa objek `Secret` berisi data di repo.
    - Anotasi `crown.cimbniaga.co.id/secret-source` per key di Deployment (patch overlay): `JWT_SECRET` (`backend`, `backend-exq`) dan `RETRY_AUTH_SECRET` menunjuk entri Secret Manager yang sama (`crown-<env>-jwt-secret`); `REDIS_URL` `backend` dan `backend-exq` menunjuk `crown-<env>-redis-url`; key lain menunjuk entri sendiri.
    - Tulis daftar key per Secret (sama dengan tabel design) sebagai `overlays/<env>/SECRETS.md` untuk tim infra.
    - _Requirements: 1.4, 9.2, 9.3, 9.4_
    - _Model: Opus, Effort: High — `JWT_SECRET` yang berbeda di salah satu runtime membuat login/EOD monitoring gagal; secret yang bocor ke repo = insiden._
  - [ ] 4.3 Replicas, resources, PDB prod
    - Patch `replicas` sesuai tabel "Jumlah pod per workload": dev/uat 1 per komponen; prod 2 untuk `backend`, `backend-exq`, `companyportal`, `vendorportal`, 1 untuk Python.
    - `overlays/prod/pdb.yaml`: PodDisruptionBudget `minAvailable: 1` untuk empat komponen itu; tidak ada PDB untuk Python.
    - _Requirements: 1.2, 5.6, 6.1, 6.7_
    - _Model: Sonnet, Effort: Low — angka sudah di design._
  - [ ] 4.4 Ingress, BackendConfig, FrontendConfig, sertifikat
    - Sesuai tabel design "Ingress": CompanyPortal `gce-internal`, BackendConfig `timeoutSec: 300` + health check `/` 8080, sertifikat `<INFRA:internal-cert>`; VendorPortal `gce`, BackendConfig `securityPolicy: <INFRA:cloud-armor-policy>` + health check sama, `ManagedCertificate` + `FrontendConfig` redirect HTTPS; host `<INFRA:host-internal-<env>>` / `<INFRA:host-vendor-<env>>`.
    - PV Filestore: patch `volumeName: ftp-data-<env>` di PVC (PV dibuat tim infra).
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_
    - _Model: Sonnet, Effort: Medium — anotasi GKE Ingress spesifik; nilai infra masih placeholder._
  - [ ] 4.5 `db-budget.yaml` per overlay
    - ConfigMap `db-budget` (tidak di-mount ke pod mana pun): `max_connections_primary`/`_replica` (`<INFRA:...>` → untuk validasi isi angka contoh tier, mis. 100, ditandai di komentar), `reserve_primary: 30`, `reserve_replica: 26`, `pool_max_conns_primary`/`_replica` per Go_Workload sesuai rumus design.
    - _Requirements: 10.1, 10.2, 10.3_
    - _Model: Sonnet, Effort: Medium — rumus sudah di design; hitungan harus memasukkan `maxSurge`._
  - _Model: Opus, Effort: High — ConfigMap dan Secret menentukan IP audit, rate limit login, dan secret JWT di tiga runtime (sub-task 4.3–4.5 Sonnet, lihat catatan masing-masing)._

- [ ] 5. Kustomization Job migrasi `deploy/k8s/migrate/`
  - `base/job-status.yaml` dan `base/job-up.yaml` sesuai design "Job": label `crown.cimbniaga.co.id/job: status|up`, `migrate-up` menjalankan `./migrate -dir migrations status && ./migrate -dir migrations up && ./migrate -dir migrations status`, `restartPolicy: Never`, `backoffLimit: 0`, `ttlSecondsAfterFinished: 604800`, `activeDeadlineSeconds: 3600`, security context + resource dari design, `DATABASE_URL` hanya dari `secretKeyRef` `migrate-secrets`, `PGSSLROOTCERT` + volume `cloudsql-ca`.
  - `overlays/<env>/kustomization.yaml`: namespace + `images:` `cms-backend-migrate`.
  - Tidak ada Job migrasi di `deploy/k8s/base` atau overlay workload.
  - _Requirements: 8.3, 8.4, 8.5, 8.6_
  - _Model: Opus, Effort: Medium — Job ini menjalankan migrasi ke Cloud SQL primary; retry otomatis atau gerbang `status` yang terlewat bisa menerapkan file yang salah ke data hidup._
