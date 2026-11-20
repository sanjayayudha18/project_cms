# Steering: Backend Architecture, Business Rules, Python EOD & Deployment

> **Scope**: Go workspace (`go.work`), layered architecture, domain modules, always-on business rules, Python EOD pipeline (`backend_python/`), testing, Docker, dan GCP deployment.  
> **Source of Truth**: Disinkronkan dari `.claude/CLAUDE.md` (Sec 3, 3a, 8, 9, 10, 14).  
> **Referenced by**: [GEMINI.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/GEMINI.md)

---

## 1. Go Workspace & Module Layout (Sec 3)

Root repository adalah Go workspace (`go.work`) yang menghubungkan 4 sibling modules secara acyclic (ditegakkan oleh compiler):

```plain
CMS2/
  go.work                    # links backend/, backend-exq/, backend-extapi/, pkg/
  pkg/                       # shared infra (tanpa dependensi ke backend mana pun):
    auth/                    # JWT TokenService, blacklist, Provider/UserRepository interfaces
    middleware/              # RequireAuth, RequireRoles, rate limiter, TrustedRealIP
    config/                  # env config loader, Load(defaultPort)
    response/                # {success,data} envelope (dipakai oleh backend-exq; TIDAK dipakai oleh backend)
    csvexport/               # CSV contract (CSVSafe, Writer, 6 master-data entities); shared by backend + backend-exq
    reports/                 # report CSV contract (Report_Kind, viewer roles, validators, cells); shared by backend + backend-exq
    userrules/               # aturan validasi user bersama antara backend/ dan backend-extapi/
  backend/                   # ATM & CIT core backend — own go.mod, port 8080
    cmd/api/main.go          # bootstrap + primary/replica pgxpool + route & Applier registration
    internal/<layer>/        # handler -> service -> repository -> db (sqlc)
    migrations/              # satu-satunya pemilik seluruh migrasi DB (termasuk tabel CIT)
  backend-exq/               # Async export jobs (API + worker + janitor) + dashboard monitoring queries (read replica). Own go.mod, port 8081
    cmd/api/main.go          # health check + RequireAuth-protected route group; hanya memvalidasi token, tidak menerbitkan
  backend-extapi/            # External user API untuk CIMB IDM: hanya /api/ext/v1/*, /health, /ready; OAuth2 client credentials (api_clients); memakai EXT_JWT_SECRET (menolak start jika JWT_SECRET diset); CLI cmd/extclient. Own go.mod, port 8082
```

*   **Aturan Dependensi**: `backend/`, `backend-exq/`, dan `backend-extapi/` hanya bergantung pada `pkg/` — **tidak pernah saling mengimpor satu sama lain**.
*   **Standar Bentuk Response JSON**:
    *   `backend/` (ATM & CIT, port 8080): Mempertahankan **Flat JSON** (mis. `{"access_token": ...}`) demi kompatibilitas wire dengan `CompanyPortal-Vite` dan `VendorPortal-Vite`. CIT dibangun di dalam `backend/` (keputusan 2026-10-08) sehingga ikut menggunakan Flat JSON.
    *   `backend-exq/` (port 8081): Menggunakan envelope `pkg/response` (`{"success": true, "data": ...}`).

---

## 2. Layering & Logical Domain Scope (`backend/internal/`)

### 2.1 Arsitektur Berlapis (Layered, bukan satu package per domain)
```
handler/ (HTTP) ──► service/ (business rules, maker-checker Submit, appliers) ──► repository/ (read-only repos di atas sqlc) ──► db/ (sqlc-generated)
```
*   **Cross-cutting packages**: `approval/`, `audit/`, `auth/`, `rolemgmt/`, `notification/`.
*   Jangan pernah mengedit `backend/internal/db/` secara manual kecuali perbaikan inflector `UserLeafe` → `UserLeave`.

### 2.2 Cakupan Domain Logis (Buat HANYA domain berikut kecuali diminta)
*   **Platform Core**: `internal/auth` (LDAP + local, JWT, `/me`) · `internal/user` · `internal/audit` · `internal/approval` (maker-checker) · `internal/document` · `internal/notification` (in-app + SMTP) · `internal/export` (CSV/XLSX/PDF)
*   **Master Data**: `internal/vendor` · `internal/vendorpic` · `internal/vault` · `internal/location` · `internal/atm` · `internal/assignment`
*   **ATM Operations**: `internal/dsr` · `internal/replenishment` · `internal/cashcount` (vault + selective machine cash count — BA/checklist/photo/e-sign/recap report → rekonsiliasi berjalan eksternal di Rec 7)
*   **Finance**: `internal/invoice` (invoicing trip FSD v1.0 §6: CROWN menghitung tagihan trip vendor + review/banding vendor + approval — **TANPA eksekusi pembayaran**) · `internal/reconciliation`
*   **CIT** (dibangun di `backend/`): `internal/cit` · `internal/journal` · CIT reconciliation (order vs DSR vs journal)
*   **Integration**: `internal/corebanking` (ingest + parse batch file escrow → masuk ke rekonsiliasi)

### 2.3 Standar Kode Go & Design Patterns (Kiro Steering Sync)
*   **Immutability (Prinsip Utama)**: Selalu buat objek/struct baru daripada memutasi objek eksisting secara in-place untuk mencegah side-effect konkurensi tersembunyi.
*   **Small Interfaces**: Definisikan interface di tempat interface tersebut dikonsumsi (consumer package / layer service), bukan di tempat interface diimplementasikan (repository package).
*   **Dependency Injection**: Gunakan fungsi konstruktor (`NewService(repo Repository, ...)`) untuk menginjeksikan dependency, mempermudah mocking saat pengujian.
*   **Functional Options**: Gunakan pola functional options (`type Option func(*Server)`) untuk konstruktor dengan opsi konfigurasi yang kompleks atau opsional.
*   **Code Hygiene & Batas Ukuran**:
    *   Fungsi < 50 baris kode.
    *   Nesting kontrol aliran < 4 tingkat kedalaman.
    *   File terfokus dan modular (< 800 baris, tipikal 200–400 baris).
    *   Selalu tangani error secara eksplisit di setiap level; jangan pernah mengabaikan atau menelan error (`_ = err`).

---

## 3. Always-On Business Rules & NFRs (Sec 3a)

> Ringkasan URS lengkap ada di [.claude/docs/requirements.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/requirements.md). Apa pun di sana yang belum ada di kode/skema adalah spesifikasi, bukan perilaku live — periksa kode terlebih dahulu.

*   **Forecast / Order ATM (Keputusan 2026-11-12, menggantikan 2026-10-08)**:
    *   CROWN tidak menjalankan model prediksi sendiri — tim DMAA mengirim `Order_All_*.xlsx` (siap pukul 06:00 WIB), dimuat oleh `backend_python/dmaa/dmaa_etl.py` ke `dmaa_files` + `dmaa_atm_forecast`.
    *   Nominal `amount_replenish` dari DMAA **bukan lagi angka order final**: CROWN menyesuaikannya dengan DSR vendor menjadi order replenishment final sesuai FSD v1.0 §3 (langkah 8):
        $$\text{Order} = (\text{Forecast Replenish} + \text{Plan Replenish}) - (\text{Saldo DSR} + \text{Forecast Refund})$$
        pada pukul 09:30–10:00 WIB (`Plan Replenish` = file forecast H-1), upload complaint/project dengan drop duplikat ATM ID + email info, pengisian order nominal kosong oleh ATM Monitoring, penyesuaian manual 11:45–12:00, order final tampil 12:30. *(Catatan: saat ini masih dalam antrean spec 2.4a; Vendor Request eksisting masih mengambil angka DMAA langsung. `amount_refund = 0` sampai DMAA mengirimnya).*
    *   Eksklusi ATM bermasalah ("problem") dan daftar input FSD (complaint/project/problem/adjustment) adalah tugas CROWN.
    *   **Penting**: Jangan campurkan formula *Order ATM* harian ini dengan formula *EOD Final Realisasi* di bagian 4.3!
*   **Batas Waktu DSR**: Deadline upload DSR adalah **09:00:00 WIB inklusif**; DSR terlambat atau tidak dikirim masuk ke laporan penalti FLM (`dsr_receipts`).
*   **Aturan Order Ekstra**: Tidak ada order tambahan untuk ATM yang sudah memiliki order emergency/adhoc yang diterbitkan hingga H-1 atau aktif pada H0; ATM berstatus "problem" dikecualikan dari rekomendasi.
*   **Key NFRs**: Dashboard ≤ 3 detik (p95) · Upload DSR ≤ 30 detik/dokumen · 300 pengguna konkuren · Jam operasional kantor 07:00–20:00 WIB, uptime sistem 24×7.

---

## 4. Backend Split: Transactional vs Batch Python EOD (Sec 14)

**Satu sistem, dua entrypoint**:
1. `cmd/api` (Go): Server transaksional yang selalu menyala dan melayani kedua frontend (aktif di jam kerja 07:00–20:00 WIB).
2. `backend_python/` (Python): Pipeline End-of-Day (EOD) & ETL berbasis file; scheduler memindai dan mencoba ulang pada pukul **05:00–09:00 WIB**.

### 4.1 Layout `backend_python/`
```plain
backend_python/
  dmaa/dmaa_etl.py                  # FTP_DATA/DMAA/Order_All_*.xlsx -> dmaa_files, dmaa_atm_forecast
  itm/cashpos/itm_cashpos_etl.py    # FTP_DATA/ITM CSV cash-position snapshot -> itm_cashpos_files, itm_cashpos (+ test)
  itm/replenish/itm_replenish_etl.py# FTP_DATA/ITM CSV -> itm_replenish_files, itm_replenish
  dsr/dsr_etl.py                    # vendor DSR workbook (sheets 'Daily' + 'Rencana Isi') -> dsr_uploads, dsr_daily_rows, dsr_rencana_isi_rows, dsr_receipts (+ test)
  replenish_classify/               # klasifikasi realisasi replenish vs ITM (±2 hari kerja, cron 09:15 WIB + after itm_replenish)
  lib/                              # shared: database.py (pool), schemas, import_jobs.py, services/ (scheduler_service, detector, retry_executor, audit_service), utils/ (checksum, timezone WIB)
  eod_retry_scheduler/              # FastAPI, port 8091, env prefix RETRY_  — menjalankan/retry dmaa, itm_cashpos, itm_replenish
  service_dsr_etl/                  # FastAPI, port 8090, env prefix DSR_ETL_ — khusus DSR (vendor-upload dry-run/commit)
```

### 4.2 Kontrak & Operasional Python EOD
*   **ETL Scripts**: CLI mandiri yang membaca dari `FTP_DATA/<DMAA|DSR|ITM>/`, idempoten per checksum SHA-256, memindahkan file selesai ke `backup/`. Nilai uang di-parse sebagai `Decimal`, **tidak pernah float**.
*   **Kontrak Nama File DSR** (ditulis oleh Go `backend/internal/service/dsr_upload.go`): `<vendor_code>__<uploaded_by_user_id>__<original_filename>` (format 2 bagian tanpa user_id juga diterima). Nominal DSR adalah **IDR penuh** meskipun header tertulis `(x 1.000)`.
*   **Dua Layanan FastAPI (Jangan Digabung)**:
    *   Berbagi rute dasar: `GET /health`, `/status`, `/status/{file_id}/history`, `/summary`, `/late`, `/audit`; `POST /process/{file_type}`, `/retry/{file_id}`.
    *   `service_dsr_etl` (`:8090`) menambahkan `POST /process/dsr/dry-run` + `/process/dsr/commit` (tidak diekspos ke browser).
    *   **Frontend Routing (`/api/eod/*`)**: Halaman `features/eod-monitoring` memanggil `/api/eod/*`. Vite dev proxy dan `nginx.conf` meneruskan `/api/eod/<path>` → `eod_retry_scheduler` `:8091/<path>` (prefix dihapus), dijalankan dengan `RETRY_AUTH_MODE=jwt` dan `RETRY_AUTH_SECRET` = Go `JWT_SECRET` (hanya lolos untuk token `ADMIN`/`APPACCESS`).
*   **Scheduler & SLA**: Memindai setiap 15 menit (`*_SCAN_INTERVAL_MINUTES`) antara pukul 05:00–09:00 WIB, auto-retry hingga 3 kali setiap 30 menit. **SLA (WIB)**: DMAA **06:00**, ITM cashpos/replenish **07:00**, DSR **09:00** — lewat SLA masuk ke `late_detections`.
*   **Prasyarat Deploy Host**: Host yang menjalankan Python ETL/scheduler **wajib menggunakan `TZ=Asia/Jakarta`** (karena `file_date` DMAA diturunkan dari mtime file dalam TZ host).
*   **Window Overlap Rule**: Pukul 07:00–09:00 WIB beririsan dengan awal jam kerja. Jauhkan pekerjaan batch berat (backfill, reprocess banyak tanggal) dari jam 07:00–20:00 WIB.

### 4.3 Formula EOD Final Realisasi & Handoff Rules (NON-NEGOTIABLE)
*   **Formula EOD (Backdated Summary)**:
    $$\text{Final Realisasi} = \text{Rekomendasi DMAA} - (\text{Saldo DSR} + \text{Refund Horizon H-2})$$
    dihitung per vendor untuk menghasilkan ringkasan harian yang dibaca modul transaksional pada hari kerja berikutnya.
*   **Aturan Handoff**:
    1. Output EOD ditulis ke **PostgreSQL sebagai single source of truth** (Redis hanya cache di atasnya).
    2. Setiap eksekusi dilacak di tabel tracking (`import_jobs`, `scan_runs`, `retry_audit_logs`).
    3. Modul transaksional hanya membaca data setelah status run tercatat **`completed` / `success`** (jangan pernah membaca data parsial/in-progress).
    4. Email alert dikirim ke `ADMIN`/`APPACCESS` via SMTP jika EOD gagal atau belum selesai saat jam kerja dimulai.

### 4.4 Arsitektur Orkestrasi Flow Panjang / Multi-Step Process (Kiro Steering Sync)
*   **State Machine Berbasis DB (Bukan Request Monolitik)**: Jangan eksekusi multi-step process panjang dalam satu request HTTP blocking. Pecah menjadi finite state machine dengan transisi terdefinisi (`draft` $\to$ `uploaded` $\to$ `validated` $\to$ `pending_approval` $\to$ `approved` $\to$ `processing` $\to$ `completed` / `failed` / `rejected`).
*   **Pemisahan Command & Processing**:
    *   Endpoint API hanya menerima request, validasi skema, simpan record state awal (`pending`/`queued`), tulis `audit_logs` tx, dan return tracking ID/status (202 Accepted).
    *   Pekerjaan berat dijalankan oleh background worker / batch processor; UI melakukan polling status atau refresh berkala.
*   **Database Single Source of Truth**: Status transisi, detail kegagalan (error log), dan hasil antar-step wajib tersimpan di PostgreSQL. Redis hanya diizinkan untuk marker antrean singkat, signal, atau cache status.
*   **3 Arketipe Alur Panjang CMS2**:
    1.  *Human Workflow*: Entity menyimpan status bisnis $\to$ `approval_requests` sebagai gate resmi $\to$ mutasi final dieksekusi setelah persetujuan $\to$ audit log atomik di setiap perubahan status.
    2.  *File-Processing Workflow*: Satu `import_jobs` per hash SHA-256 file $\to$ parsing $\to$ baris mentah dipisah dari summary $\to$ resume dari step aman terakhir saat retry.
    3.  *Batch/EOD Workflow*: Berjalan via runner terisolasi per `processing_date` $\to$ modul transaksional hanya membaca status `completed`/`success` $\to$ SMTP alert otomatis saat gagal.
*   **Pola Layering Orchestrator**:
    *   `Handler`: Validasi payload, auth, dan response envelope.
    *   `Orchestrator Service`: Mengatur urutan step, transisi state yang valid, retry, dan failure recovery.
    *   `Domain Service`: Mengeksekusi aturan bisnis murni per domain (misal: invoice calc, matching reconciliation).
    *   `Repository`: Akses data murni.
*   **Idempotensi per Step**: Setiap step wajib aman dipanggil ulang tanpa efek samping ganda (tidak double insert, double ledger posting, atau duplicate notification). Gunakan idempotency key atau status guard di level query SQL (`WHERE status = 'expected_state'`).

---

## 5. Testing Standards (Sec 8)

*   **Go**: `go test ./...`. Gunakan table-driven tests untuk service dengan mock repository untuk unit test.
*   **Integration Tests**: Menggunakan PostgreSQL nyata (`//go:build integration`, otomatis skip jika `DATABASE_URL` kosong).
*   **Coverage Target**: Minimal **80%** pada `internal/*`. Dilarang merge jika ada test yang gagal atau di-skip tanpa alasan.
*   **Kasus Uji Wajib**: LDAP vs local auth, gerbang maker-checker, penolakan RBAC (403/404), aritmatika uang (`numeric`/`bigint`), selisih (delta) rekonsiliasi, dan routing replica (`dbRead`) vs primary (`db`).

---

## 6. Environment, Docker & GCP Deployment (Sec 9 & 10)

### 6.1 Variabel Environment Utama (`.env`)
```plain
APP_ENV=development
PORT=8080
DATABASE_URL=postgres://user:pass@primary-host:5432/cms
DATABASE_REPLICA_URL=postgres://user:pass@replica-host:5432/cms
REDIS_URL=redis://localhost:6379
JWT_SECRET=change_me
SESSION_MAX_LIFETIME=1h
TRUSTED_PROXIES=            # Daftar CIDR/IP proxy (Nginx/LB) yang X-Forwarded-For-nya dipercaya; kosong = tidak ada (QC F13)
LDAP_URL=ldap://ldap.company.local:389
LDAP_BASE_DN=dc=company,dc=local
LDAP_BIND_DN=
LDAP_BIND_PASSWORD=
SMTP_HOST=smtp.company.local
SMTP_PORT=587
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM=cms-noreply@company.co.id
GCS_BUCKET=
LOG_LEVEL=info
```
*   **Larangan**: Jangan pernah komit `.env` (`backend/.env` maupun `backend_python/.env`). Jangan pernah mencatat secret, JWT, password LDAP/SMTP ke dalam log.

### 6.2 Docker & `.dockerignore`
*   `backend/Dockerfile` dan `backend-exq/Dockerfile` adalah multi-stage build (distroless/alpine, non-root, `HEALTHCHECK` pada `/health`) yang di-build dari **repo root (`context: ..` / `.`)** agar `pkg/` ter-resolve melalui `go.work`.
*   Root `.dockerignore` adalah allowlist untuk build backend (`go.work*`, `pkg/`, `backend/`, `backend-exq/`; tidak pernah menyertakan `.env`).

### 6.3 Arsitektur Deployment GCP (Keputusan 2026-11-05)
*   **Target Produksi**: **SATU Compute Engine VM** menjalankan seluruh komponen — kedua frontend (Nginx) dan seluruh backend (`backend` :8080, `backend-exq` :8081, `backend-extapi` :8082, serta Python EOD :8090/:8091).
*   Trafik FE dan BE dipisahkan oleh **GCP Load Balancer** di depan VM.
*   `backend` dan `backend-exq` wajib berbagi **CloudSQL yang sama**, **Redis (Memorystore) yang sama** (untuk JWT blacklist & rate limit), dan **`JWT_SECRET` yang sama** (hanya `backend` ATM yang menerbitkan token user; `backend-exq` hanya memvalidasi).
*   Image Docker wajib di-tag dengan **git SHA** (dilarang menggunakan tag `latest`); simpan SHA sebelumnya agar siap di-rollback kapan saja. Detail lengkap: [.claude/docs/deployment.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/docs/deployment.md).
