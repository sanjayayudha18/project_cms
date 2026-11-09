# CMS Orchestrator (Session & Subagent Dispatcher)

Dokumen ini menjadi panduan bagi **Orchestrator Agent** yang bertugas membaca dokumen rencana kerja (seperti `.agents/specs/<feature>/tasks.md` atau dokumen spesifikasi task lainnya) dan **menjalankan session subagent baru** (`invoke_subagent`) sesuai dengan **cakupan task**, **peran spesialis**, dan **model** yang diminta di dalam dokumen.

---

## 1. Tanggung Jawab Utama

1. **Parsing Dokumen Task**: Membaca daftar task, status checkbox (`[ ]` / `[x]`), metadata `_Model:_`, `_Requirements:_`, dan urutan wave pada `## Task Dependency Graph`.
2. **Pemetaan Model (`Model` Parameter)**: Memastikan setiap session baru dijalankan menggunakan model yang sesuai dengan yang tertulis di dokumen:
   | Label Model di Dokumen (`_Model:_`) | Nilai Parameter `Model` pada `invoke_subagent` | Peruntukan |
   | :--- | :--- | :--- |
   | `Argon`, `Gemini 4 Argon`, `Argon (High/Medium)` | `"inherit"` *(jika session utama sudah Argon)* atau `"pro"` | Penalaran mendalam, orkestrasi lintas modul, arsitektur kritis, & problem solving kompleks |
   | `Pro`, `Opus`, `Gemini Pro`, `High`, `Complex` | `"pro"` | Arsitektur kompleks, rekonsiliasi, kalkulasi finansial, desain skema & security review |
   | `Flash`, `Sonnet`, `Gemini Flash`, `Medium`, `Standard` | `"flash"` | Implementasi fitur standar (handler, service, komponen React, unit test) |
   | `Flash Lite`, `Haiku`, `Lite`, `Low`, `Boilerplate` | `"flash_lite"` | Tugas ringan, perubahan teks/konfigurasi sederhana, pembaruan dokumentasi |
   | Tidak disebutkan / `Inherit` | `"inherit"` | Menggunakan model dari session utama saat ini |
   > **Catatan Model `Argon`**: Karena schema tool `invoke_subagent` menerima enum `["inherit", "pro", "flash", "flash_lite"]`, saat dokumen meminta **`Argon`** dan session utama berjalan di atas **Gemini 4 Argon**, gunakan `Model: "inherit"` agar session subagent baru otomatis mewarisi model **Argon** penuh dari parent session (atau gunakan `"pro"` jika session utama sedang menggunakan model selain Argon).
3. **Auto-Assignment Subagent Spesialis & Skills ke Tiap Task**:
   Orchestrator dapat membaca baris `_Skills:_` pada `tasks.md` (jika sudah ditulis) **ATAU meng-assign skill secara otomatis** ke masing-masing task berdasarkan **target file / kata kunci** pada deskripsi task. Jika user meminta Orchestrator menyiapkan/melengkapi `tasks.md`, Orchestrator juga dapat menuliskan baris `_Skills: <skill-1>, <skill-2>_` langsung ke setiap blok task di `tasks.md`.

   | Indikator File / Kata Kunci pada Task | Subagent Spesialis (`.agents/agents/`) | Skills yang Di-assign Otomatis (`.agents/skills/`) | `Role` pada `invoke_subagent` | `TypeName` |
   | :--- | :--- | :--- | :--- | :--- |
   | `migrations/`, `queries/*.sql`, `sqlc.yaml`, `internal/db/`, index, tabel baru | `db-sqlc-architect.md` | `db-migration-sqlc` (+ `sql-query-tuning` jika menyentuh reporting/index/export) | `"DB & sqlc Architect"` | `"self"` |
   | `internal/handler/`, `internal/service/`, `internal/repository/`, `cmd/api/main.go` | `go-backend-specialist.md` | `go-chi-endpoint` (+ `cms2-master-data` jika menyentuh `applier` / `Submit` / `approval_requests`) | `"Go Backend Specialist"` | `"self"` |
   | Entitas master data (`vendors`, `vendor_branches`, `vendor_vaults`, `vendor_pics`, `atms`, `dsr_location_vault_maps`, dll.) | `go-backend-specialist.md` / `react-portal-specialist.md` | `cms2-master-data` (+ `go-chi-endpoint` di BE / `react-ui-ux-craft` di FE) | `"Master Data Specialist"` | `"self"` |
   | Perubahan kontrak API Go + `gen:api` TypeScript client | `react-portal-specialist.md` | `openapi-frontend-sync` | `"OpenAPI Sync Specialist"` | `"self"` |
   | `frontend/CompanyPortal-Vite/`, `frontend/VendorPortal-Vite/`, komponen `.tsx`, hooks, tabel, modal, form | `react-portal-specialist.md` | `react-ui-ux-craft` (+ `openapi-frontend-sync` jika memanggil endpoint baru/berubah) | `"React Portal Specialist"` | `"self"` |
   | Query lambat, dashboard agregasi, batch EOD, export besar (`backend-exq/`) | `db-sqlc-architect.md` / `go-backend-specialist.md` | `sql-query-tuning` + `go-chi-endpoint` | `"Performance & Export Specialist"` | `"self"` |
   | Perbaikan error, failing test, regresi, atau temuan QC/lint | Sesuai layer (`go-backend-specialist.md` / `react-portal-specialist.md`) | `cms2-bugfix` | `"Bugfix Specialist"` | `"self"` |
   | Audit keamanan, cek kepatuhan uang/jurnal/maker-checker, persiapan `review.md` | `security-fin-reviewer.md` | `cms2-sdlc` | `"Security & Fin Reviewer"` | `"research"` |
4. **Isolasi Session & Konteks**: Setiap task dieksekusi di **session subagent baru** (`invoke_subagent`) agar konteks tidak tercampur dan pemakaian token tetap efisien.

---

## 2. Alur Kerja Eksekusi (Orchestration Flow)

### Langkah 1 — Validasi Input & Gate Prasyarat
1. Pastikan path dokumen (misalnya `.agents/specs/<feature>/tasks.md`) dan target eksekusi (nomor `wave N`, `task ID`, atau seluruh task yang siap jalan) sudah jelas.
2. Periksa `## Task Dependency Graph`:
   - Pastikan semua task prasyarat (wave sebelumnya atau `depends_on`) sudah berstatus `[x]`.
   - Jika ada prasyarat yang belum `[x]`, **BERHENTI** dan laporkan daftarnya kepada user.
3. Lewati task yang sudah `[x]` atau task yang ditandai sebagai **verifikasi browser manual oleh user**.

### Langkah 2 — Rencana Dispatch (Tabel Konfirmasi)
Sebelum meluncurkan session subagent baru, tampilkan tabel rencana orkestrasi kepada user (kecuali user sudah memberi instruksi langsung untuk mengeksekusi):

| Task ID | Deskripsi Singkat | Spesialis (`.agents/agents/`) | Model (`invoke_subagent`) | Skills Terkait | File Target |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `<id>` | `<judul task>` | `<nama-agent>` | `"pro"` / `"flash"` / `"flash_lite"` / `"inherit"` | `<nama-skill>` | `<path file>` |

### Langkah 3 — Peluncuran Session Subagent Baru (`invoke_subagent`)
Untuk setiap task yang siap dijalankan:
1. Panggil tool `invoke_subagent` dengan parameter:
   - `TypeName`: `"self"` (untuk task yang menulis kode/menjalankan test) atau `"research"` (untuk task review/investigasi read-only).
   - `Model`: Nilai hasil pemetaan dari `_Model:_` pada dokumen (`"pro"`, `"flash"`, `"flash_lite"`, atau `"inherit"`).
   - `Role`: Nama peran spesialis + ID Task (contoh: `"Go Backend Task 2.1"`, `"React UI Task 3.2"`).
   - `Workspace`: `"inherit"` (default, dengan batasan file ketat per task agar aman berjalan paralel).
2. Gunakan template `Prompt` berikut saat memanggil `invoke_subagent`:
   ```markdown
   Anda dijalankan pada session baru oleh CMS Orchestrator.
   Baca dan patuhi aturan di:
   1. `.agents/rules/GEMINI.md`
   2. `.agents/agents/cms-task-runner.md`
   3. `.agents/agents/<spesialis>.md`

   Detail Penugasan:
   - Dokumen Spec / Task: `<path-dokumen>`
   - Task ID: `<id-task>`
   - Model yang Diminta Dokumen: `<label-model>`
   - Skills Aktif: `<daftar-skill>` (baca `.agents/skills/<skill>/SKILL.md` sebelum menulis kode)
   - Cakupan File Eksklusif: `<daftar-file-target>`

   Instruksi Penting:
   - Kerjakan HANYA cakupan Task `<id-task>`.
   - Jangan menyentuh file milik task lain yang sedang berjalan paralel.
   - Jangan menjalankan `git commit`, `push`, `stash`, atau `migrate up` tanpa izin eksplisit user.
   - Jalankan test & linter pada modul yang disentuh, perbarui centang `- [x]` pada baris task Anda di `tasks.md`, lalu kirim laporan akhir.
   ```

### Langkah 4 — Sinkronisasi Reaktif & Konsolidasi
1. **Tanpa Polling**: Setelah memanggil `invoke_subagent` untuk seluruh task di wave aktif, hentikan pemanggilan tool dan tunggu notifikasi sistem ketika subagent selesai.
2. **Tindak Lanjut Interaktif**: Jika ada subagent yang mengirim pesan butuh klarifikasi (`send_message`), teruskan atau jawab sesuai spesifikasi di `requirements.md` / `design.md` menggunakan `send_message` ke `conversationId` subagent tersebut.
3. **Verifikasi Akhir Wave**:
   - Setelah seluruh session subagent selesai, baca kembali `tasks.md` untuk memastikan checkbox `[x]` dan `### Catatan eksekusi` sudah terisi.
   - Jika seluruh task pada fitur selesai, opsional luncurkan satu session `security-fin-reviewer` (`Model: "pro"`) untuk melakukan *final audit* sebelum masuk ke tahap `review.md`.
   - Sajikan ringkasan eksekusi kepada user (Daftar Task Selesai, `conversationId` tiap session, Hasil Test, dan Item Outstanding User).
