# GEMINI.md — Cash Management System (CMS2 / CROWN) for ATM & CIT

> **AI: BACA INI TERLEBIH DAHULU DI SETIAP SESI.** *Single source of truth* untuk Google Antigravity / Gemini. Jika pekerjaan Anda bertentangan dengan dokumen ini, **BERHENTI dan tanyakan ke user**. Jangan pernah mengarang endpoint, tabel, kolom, env var, atau modul.  
> **Batas ukuran file di `.agents/rules/` adalah 24 KB** — detail teknis domain per bagian telah didelegasikan secara utuh ke folder [`.agents/rules/steering/`](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering).  
> **Penjelasan sederhana konsep bisnis CMS**: [.claude/eli5.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.claude/eli5.md) · **Status modul terbangun vs pending**: [.agents/development-progress.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/development-progress.md) (perbarui setiap kali sebuah tahap/spec selesai).

---

## 0. Golden Rules (Inviolable Guardrails)

1.  **Stack is FIXED (Sec 2)**: Dilarang menambah library atau framework baru tanpa persetujuan eksplisit user.
2.  **Follow Module & Table Map (Sec 3)**: Jangan membuat modul atau tabel di luar daftar resmi kecuali diminta ([steering/backend.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/backend.md) & [steering/database.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/database.md)).
3.  **Maker-Checker + Atomic Audit Log (Sec 5)**: Setiap aksi yang mengubah state data master atau finansial wajib mematuhi maker-checker (`approval_requests` / `master_data_change_requests`) + menulis `audit_logs` dalam transaksi (`tx`) yang sama (lihat pengecualian terdokumentasi di [steering/security-compliance.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/security-compliance.md)).
4.  **Money = `numeric` / Integer Minor Units (`bigint` IDR), NEVER Float (Sec 6)**: Dilarang keras menggunakan `float` atau `double` untuk nilai uang di SQL, Go, Python, maupun TypeScript.
5.  **Plan Before Non-Trivial Work**: Buat rencana terlebih dahulu, kerjakan dalam diff kecil, satu *concern* per perubahan.
6.  **DB Topology (Sec 6)**: Operasi write/update + transactional read-after-write → **Primary pool (`db`)**; reporting, dashboard, cash monitoring, dan async export → **Read replica pool (`dbRead`)**.
7.  **Two Isolated Frontends (Sec 5 & 13)**: `CompanyPortal-Vite` (Internal — LDAP bind → JWT, port 5173) dan `VendorPortal-Vite` (Vendor — kredensial lokal DB → JWT, port 5174). Jaga tetap terpisah ([steering/frontend.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/frontend.md)).
8.  **Two Backend Entrypoints, Shared Codebase (Sec 14)**: `cmd/api` (transaksional Go, jam kerja 07:00–20:00 WIB) + pipeline EOD di Python (`backend_python/`, berbasis file; scheduler memindai/retry 05:00–09:00 WIB). Logika Go berbagi `internal/*` dan `pkg/` ([steering/backend.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/backend.md)).
9.  **EOD Batch Output is Source of Truth (Sec 14)**: Ditulis ke PostgreSQL (bukan Redis), dilacak di tabel run (`import_jobs`/`scan_runs`), dan memberi sinyal kesiapan (`completed`/`success`) sebelum dibaca modul transaksional.
10. **Manual Browser Verification is the User's Job**: Pengujian/pengecekan manual di browser dilakukan oleh **user**, bukan AI. Kirimkan kode + pemeriksaan otomatis (`go test`, `pnpm test`, `lint`, `typecheck`, `build`), lalu nyatakan secara jujur bahwa verifikasi browser manual berstatus **`outstanding (user)`**.

---

## 0a. Development Stage & DB Schema Policy

*   **Fase Saat Ini**: Early development (greenfield) — **bukan lagi MVP** (target penuh adalah cakupan **FSD v1.0 §A**).
*   **Schema is MUTABLE**: Perubahan database (tabel, kolom, dan migrasi baru di `backend/migrations/NNN_*.sql`) diharapkan seiring berkembangnya kebutuhan. Keputusan struktural yang mengubah peta modul/tabel wajib diperbarui di dokumentasi setelah diterapkan.

---

## 1. System Overview & Graphify Navigation

*   **Nama Sistem**: **CROWN — THE Cash Management System** (ATM & CIT).
*   **Cakupan Utama (FSD v1.0 §A)**: (1) Manajemen vendor & master data, (2) Intake forecast DMAA, (3) Forecasting kas ATM harian — DMAA + DSR vendor → penyesuaian nominal → order replenishment final, (4) Pemenuhan (*fulfilment*) order replenish ATM & alokasi vault CIT/ACM, (5) Jadwal & eksekusi cash count (vault + mesin selektif), (6) Invoicing trip replenish ATM (tanpa eksekusi pembayaran; pembayaran di SMART). Juga tercakup dalam rencana: rekonsiliasi escrow vs Corebanking, jurnal/rekonsiliasi CIT, dan EOD Final Realisasi.
*   **Stage**: Greenfield, dibangun dengan AI melalui **AI-Native SDLC** (bukan *vibe-coded*) dengan human gate di setiap tahap.
*   **Roles Utama**: Admin (`ADMIN`, `ADMIN_PARAM`, `APPACCESS`), Operator (`ATM-USER`, `ACM-USER`), Manager/Approver (`ATM-SPV`, `BRANCH-ATM-SPV`, `ACM-SPV`), Cash Count (`CC-CWM`, `CC-COORDINATOR`, `CC-EXECUTOR`, `CC-CCR`, `CC-RECON`, `CC-D3`, `CC-D2`), Vendor (`VENDOR-USER`, `VENDOR-ATM-USER`, `VENDOR-VAULT-USER`), Branch/Internal User, dan Service (`EXT-CLIENT`).
*   **Eksplorasi Kode dengan `graphify` (Sec 1a)**:
    *   Saat `graphify-out/graph.json` tersedia, jalankan `graphify query "<pertanyaan>"`, `graphify path "<A>" "<B>"`, atau `graphify explain "<konsep>"` sebelum melakukan grepping luas.
    *   Gunakan `graphify-out/wiki/index.md` jika tersedia; baca `graphify-out/GRAPH_REPORT.md` hanya untuk tinjauan arsitektur luas.
    *   Setelah memodifikasi kode pada sesi ini, jalankan **`graphify update .`** (AST-only, tanpa biaya API).

---

## 2. Tech Stack (Source of Truth) & Deterministic Commands

| Layer | Pilihan Teknologi |
|---|---|
| **Backend (Go Workspace)** | Go + Chi v5 — `go.work` menghubungkan `pkg/`, `backend/` (`:8080`, Flat JSON), `backend-exq/` (`:8081`, envelope `pkg/response`), `backend-extapi/` (`:8082`, CIMB IDM OAuth2) |
| **DB Pool** | `pgx v5` / `pgxpool` — primary (`db`) + replica (`dbRead`) dibuat di `backend/cmd/api/main.go` |
| **SQL Codegen** | `sqlc` **v1.31.1** (terkunci) — `backend/sqlc.yaml`, query di `backend/queries/*.sql` |
| **API Contract** | OpenAPI 3.0.3 (`backend/api/openapi.yaml`); `kin-openapi` **v0.151.0** (Go test), `openapi-typescript` **7.13.0** (`CompanyPortal-Vite`) |
| **EOD / ETL Pipeline** | Python — FastAPI + pandas + psycopg (`backend_python/`: `service_dsr_etl` `:8090`, `eod_retry_scheduler` `:8091`) |
| **Frontend** | React + Vite (disajikan via Nginx): `CompanyPortal-Vite` (`:5173`) & `VendorPortal-Vite` (`:5174`) |
| **Database & Cache** | PostgreSQL eksternal (NON-dockerized, primary + read replica) & Redis (Memorystore di prod) |
| **Auth, Email & Cloud** | Internal: LDAP → JWT · Vendor: Kredensial lokal DB → JWT · SMTP Relay Perusahaan · Docker + GCP (1 Compute Engine VM di belakang GCP Load Balancer) |

### 2a. Perintah Utama (Commands)

```bash
# --- Go Backend (jalankan per modul; terhubung lewat go.work) ---
cd backend && go test ./...                                    # ATM/CIT backend (integration test skip jika DATABASE_URL kosong)
cd backend-exq && go test ./...                                # Async export & monitoring backend (:8081)
cd backend-extapi && go test ./...                             # External user API untuk CIMB IDM (:8082)
cd pkg && go test ./...                                        # Shared library
cd backend && go run ./cmd/api                                 # Start ATM API :8080 (backend-exq: :8081, backend-extapi: :8082)

# --- Linter & Vulnerability Scan (konfigurasi: root .golangci.yml) ---
cd backend && golangci-lint run --build-tags integration ./...
cd backend && govulncheck ./...

# --- SQLC Codegen (WAJIB v1.31.1; setelah generate, ubah manual UserLeafe -> UserLeave di backend/internal/db/) ---
cd backend && sqlc generate

# --- OpenAPI Contract (setelah mengedit backend/api/openapi.yaml atau handler terkait) ---
cd backend && go test ./internal/handler -run OpenAPI          # Validasi spec + route + shape response
pnpm --dir frontend/CompanyPortal-Vite run gen:api             # Regenerasi src/lib/api/schema.d.ts (wajib di-commit!)

# --- Frontends (pnpm; script identik di kedua aplikasi) ---
pnpm --dir frontend/CompanyPortal-Vite run dev|build|test|lint # Internal portal (:5173)
pnpm --dir frontend/VendorPortal-Vite  run dev|build|test|lint # Vendor portal (:5174)

# --- Python EOD (jalankan dari backend_python/ agar import `lib` ter-resolve; stdlib unittest, BUKAN pytest) ---
cd backend_python && python -m eod_retry_scheduler.run         # Scheduler :8091
cd backend_python && python -m service_dsr_etl.run             # DSR ETL :8090
cd backend_python && python -m unittest discover -s dsr && python -m unittest discover -s itm/cashpos
cd backend_python && python -m unittest lib.test_import_jobs -v

# --- Migrasi Database (JANGAN PERNAH jalankan psql -f manual; dari shell host gunakan localhost:5432, bukan host.docker.internal) ---
cd backend && go run ./cmd/migrate status                      # Cek applied/pending + verifikasi checksum SHA-256
cd backend && go run ./cmd/migrate up                          # Terapkan migrasi pending di bawah advisory lock
```

---

## 3. Modular Steering Index (Detail Lengkap `CLAUDE.md` Sec 3–15)

Untuk menjaga setiap file di bawah batas 24 KB tanpa kehilangan satu pun detail aturan dan tabel dari `.claude/CLAUDE.md`, seluruh spesifikasi teknis dibagi ke dalam 5 dokumen di [`.agents/rules/steering/`](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering):

| File Steering | Cakupan Section `CLAUDE.md` | Kapan Wajib Dibaca & Diikuti |
|---|---|---|
| **[database.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/database.md)** | **Sec 0a, 2a, 3 (DB Map), 6, 12 (Migrations 001–045)** | Saat menyentuh skema DB, menulis query `sqlc`, menjalankan `cmd/migrate` (next free: `046`), memeriksa nama tabel/kolom resmi (Auth, Core, Master, PKS, ATM, CIT, Cash Count, Finance, Escrow), atau aturan `db` vs `dbRead`. |
| **[backend.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/backend.md)** | **Sec 3 (Go Layout & Domains), 3a (Business Rules), 8, 9, 10, 14 (Python EOD)** | Saat menulis kode Go (`backend/`, `backend-exq/`, `backend-extapi/`, `pkg/`), formula Order ATM vs EOD Final Realisasi, pipeline `backend_python/`, pengujian Go (coverage $\ge 80\%$), konfigurasi `.env`/Docker, atau arsitektur GCP VM. |
| **[security-compliance.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/security-compliance.md)** | **Sec 5 (Cross-Cutting Rules) & Sec 12 (Resolved Decisions & GR#3 Deviations)** | Saat mengimplementasikan auth LDAP/lokal, RBAC ganda, engine maker-checker master data (`Submit` → `202` → `Applier`), 9 deviasi resmi GR#3 (apply langsung + audit tx), `TrustedRealIP`, tiket replenish, vault ACM v3, kirim ke vendor, atau realisasi 2.2c. |
| **[frontend.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/frontend.md)** | **Sec 3 (Frontends), 8, 11, 13 (UI/UX Brand), 14 (`/api/eod/*` Proxy)** | Saat membangun komponen React di `CompanyPortal-Vite` ("Merah Sirih") atau `VendorPortal-Vite` ("Merah Menyala"), sinkronisasi OpenAPI (`gen:api`), format uang `tabular-nums` IDR, 4 status wajib UI, atau proxy `/api/eod/*`. |
| **[sdlc-protocols.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/sdlc-protocols.md)** | **Sec 1a, 4 (The Leash), 4a (6-Stage SDLC & ScDD), 15 (Where Docs Live)** | Saat memulai task baru (Langkah 0: 5 metode pengembangan), melewati *sequential human gate* (`requirements.md` → `design.md` → `tasks.md` → `review.md` → `incidents.md`), memeriksa **Gerbang Sec 4 #7**, atau melacak tech-debt (`FE_debt.md`/`BE_debt.md`). |

---

## 4. AI Collaboration Rules & Alur SDLC `.agents/` (Sec 4 & 4a)

1.  **Langkah 0 — Wajib Tanya & Rekomendasikan 5 Metode Sebelum Coding**:
    Setiap ada permintaan fitur atau perbaikan, sajikan 5 pilihan metode beserta evaluasi checklist dan rekomendasinya, lalu tunggu keputusan user:
    *   **1. Spec-Driven Dev** (`.agents/specs/<feature>/` → selesai di `.agents/specs/done/<feature>/`): Alur formal 6 tahap (`requirements.md` → `design.md` → `tasks.md` → `### Test Results` → `review.md` → `incidents.md`) untuk fitur besar atau yang menyentuh area kritis **Sec 4 #7**.
    *   **2. Scope-Driven Dev / ScDD** (`.agents/scopes/<feature>/` → selesai di `.agents/scopes/done/<feature>/`): Alur berbasis AC bernomor (`scope.md` → `tasks.md`) untuk fitur yang kebutuhannya sudah jelas di FSD/URS/DSR.
    *   **3. UI Revamp Dev** (`.agents/ui-revamps/<screen>/` → selesai di `.agents/ui-revamps/done/<screen>/`): Alur khusus perombakan antarmuka (`audit.md` → `desain artifact` → `ui-spec.md` → `tasks.md`) untuk redesain layar, table overhaul, audit 4 status UI, dan brand alignment tanpa migrasi DB.
    *   **4. Bug Fix** ([`.agents/bugfixes.md`](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/bugfixes.md)): Diagnosa → tulis *failing test* terlebih dahulu → minimal fix → verifikasi → catat di `bugfixes.md` (gunakan skill `cms2-bugfix`).
    *   **5. Small Work Dev** ([`.agents/changes.md`](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/changes.md)): perubahan kecil **bukan bug** (ubah label/teks, tambah/sembunyikan/urutkan elemen UI, tweak perilaku yang ada, docs/tooling/config): satu concern, ≤5 file, tanpa skema/migrasi, tanpa endpoint baru/berubah, tanpa aturan bisnis baru, di luar Sec 4 #7, bukan redesain layar. **Gate**: user OK AC sebelum koding.
2.  **STOP & Flag pada Area Kritis (Sec 4 #7)**:
    Perubahan yang menyentuh **auth, perhitungan uang/jurnal, rekonsiliasi, migrasi data/skema DB, atau penghapusan data** tidak pernah dikecualikan — wajib menggunakan Spec-Driven Dev atau ScDD dengan `## Gerbang Sec 4 #7` lengkap dan persetujuan manusia.
3.  **Satu Dokumen SDLC per Langkah (Sequential Human Gate)**:
    Tulis artefak SDLC **satu per satu secara berurutan** dan tunggu status `accepted` dari user sebelum lanjut ke tahap berikutnya. **Dilarang keras** membuat dua artefak tahap berbeda (misal `requirements.md` dan `design.md`) sekaligus dalam satu giliran.
4.  **Saran Langkah Selanjutnya**: Di akhir setiap penyelesaian task, berikan saran langkah logis berikutnya kepada user.

---

## 5. Glossary (Sec 7)

| Istilah | Arti dalam Domain CMS2 / CROWN |
|---|---|
| **DSR** | *Daily Status Report*: laporan posisi kas per ATM/CRM per hari dari vendor (batas kirim 09:00:00 WIB). |
| **CIT** | *Cash in Transit*: penjemputan/pengantaran fisik uang tunai oleh vendor. |
| **Replenishment** | Instruksi dan eksekusi pengisian ulang uang tunai mesin ATM. |
| **Vault** | Khazanah kas vendor (`ATM`, `CASH`, `ATM_CASH`) yang menyimpan uang tunai milik bank. |
| **Escrow Reconciliation** | Pencocokan posisi kas CMS terhadap batch file rekening escrow dari Corebanking. |
| **Forecast (DMAA)** | Prediksi kebutuhan kas per ATM/denominasi/tanggal dari tim DMAA (`Order_All_*.xlsx`); disesuaikan CROWN dengan DSR menjadi order replenishment final. |
| **Cash Count** | Perhitungan fisik uang tunai: di vault vendor + mesin ATM secara selektif. |
| **Maker-Checker** | Kontrol persetujuan dua pihak (`Maker != Checker`) untuk perubahan data master dan finansial. |
| **Invoicing Trip** | Perhitungan tagihan trip bulanan vendor dari realisasi replenishment yang dieksekusi (FSD §6) beserta alur review/banding & approval (tanpa eksekusi pembayaran). |

---

## 6. Definition of Done Checklist (Sec 11)

Sebelum menyatakan suatu pekerjaan selesai, pastikan seluruh poin berikut terpenuhi:
- [ ] Sesuai dengan requirement serta peta modul & tabel resmi (Sec 3 / [steering/database.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/database.md)).
- [ ] Jalur autentikasi tepat (LDAP untuk internal / kredensial lokal untuk vendor) dan RBAC ter-scope di route + service layer.
- [ ] Maker-checker + penulisan `audit_logs` dalam transaksi yang sama telah terhubung (Sec 5 / [steering/security-compliance.md](file:///c:/Users/RB%20Yudha%20Rangga/OneDrive/Documents/Development/CMS2/.agents/rules/steering/security-compliance.md)).
- [ ] Operasi read berat/reporting menggunakan replica (`dbRead`), operasi write & read-after-write menggunakan primary (`db`).
- [ ] Nilai uang bertipe `numeric` / `bigint` IDR (tanpa `float`), dan timestamp bertipe `timestamptz`.
- [ ] Seluruh automated test lulus (`go test ./...`, `pnpm test`, `lint`, `build`) termasuk kasus uji auth, RBAC, dan perhitungan uang.
- [ ] Tidak ada secret atau konfigurasi sensitif yang di-hardcode; `.env.example` diperbarui jika ada env var baru.
- [ ] Bentuk response JSON sesuai standar: envelope `pkg/response` di `backend-exq` (`:8081`); Flat JSON yang konsisten dengan handler sekitar di `backend` (`:8080`).
- [ ] Endpoint baru/berubah pada modul yang tercakup di `backend/api/openapi.yaml` telah ditambahkan ke spec, lulus `go test ./internal/handler -run OpenAPI`, dan `pnpm gen:api` (`schema.d.ts`) sudah dijalankan.
- [ ] Verifikasi manual di browser dicatat sebagai **`outstanding (user)`** (Golden Rule #10), dan `graphify update .` dijalankan jika ada perubahan file kode.
