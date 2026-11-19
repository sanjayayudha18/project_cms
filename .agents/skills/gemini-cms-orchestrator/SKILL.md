---
name: gemini-cms-orchestrator
description: >-
  Dispatch satu wave task dari "## Task Dependency Graph" pada spec CMS2
  (misalnya .agents/specs/<feature>/tasks.md) menggunakan subagent Antigravity
  secara paralel/terisolasi, memetakan model (Opus/Sonnet/Haiku -> Gemini 4
  Argon/Neon/Hydrogen/Flash/Carbon), dan mematuhi cms-task-runner.md serta
  spesialis terkait. Gunakan saat user mengatakan "jalankan wave N <spec>",
  "run wave N", atau memanggil /gemini-cms-orchestrator.
---

# Gemini CMS Orchestrator (Antigravity Subagent Dispatcher)

Skill ini adalah versi **Google Gemini / Antigravity** dari task orchestrator CMS2. Bertugas membaca dokumen rencana kerja (`.agents/specs/<feature>/tasks.md`), melakukan gate-check prasyarat, memetakan model Claude/Gemini ke parameter `invoke_subagent`, menentukan subagent spesialis & skills, lalu meluncurkan subagent terisolasi per task dalam satu *wave* paralel secara reaktif tanpa polling.

Dokumen panduan arsitektur lengkap: **`.agents/agents/cms-orchestrator.md`**.

---

## 1. Input

- **Path Spec**: Path folder spec (default: `.agents/specs/<feature>/`, misalnya `.agents/specs/vendor-pks-enhancement/`).
- **Target Wave / Task**: Nomor wave `N` dari `## Task Dependency Graph` atau ID task spesifik (misal `1.1`, `3.2`).
- *Jika belum disebutkan oleh user, tanyakan terlebih dahulu sebelum mengubah kode atau meluncurkan subagent.*

---

## 2. Alur Eksekusi (6 Langkah)

### Langkah 1: Baca Dokumen Spec & Validasi Gate Prasyarat
1. Buka dan baca `tasks.md` pada folder spec terkait.
2. Ambil daftar ID task untuk wave `N` dari blok JSON di bawah heading `## Task Dependency Graph`:
   ```json
   {
     "waves": [
       { "id": 0, "tasks": ["1.1", "2.1"] },
       { "id": 1, "tasks": ["1.2", "1.3", "2.2", "3.1", "3.2"] }
     ]
   }
   ```
3. **Gate Wave Sebelumnya**:
   - Periksa seluruh task pada wave `< N` (atau task pada daftar `depends_on`).
   - Pastikan semuanya sudah bertanda centang `- [x]`.
   - **Jika masih ada task yang belum centang (`- [ ]`)**: **BERHENTI**. Tampilkan daftar task yang belum selesai kepada user dan minta konfirmasi apakah ingin tetap lanjut atau menyelesaikan prasyarat terlebih dahulu.
4. **Filter Task**:
   - Lewati task yang sudah bertanda `- [x]`.
   - Lewati task yang bertanda verifikasi manual user (misalnya `Cek browser manual (user)`) dan catat sebagai `outstanding (user)`.

---

### Langkah 2: Pemetaan Model (Claude & Gemini -> Parameter Subagent)
Periksa metadata `_Model:_` pada task (atau warisi dari parent task-nya jika berupa sub-task tanpa deklarasi model mandiri). 

Karena schema tool Antigravity `invoke_subagent` menerima enum `["inherit", "pro", "flash", "flash_lite"]`, gunakan pemetaan standar berikut:

| Label di Dokumen (`_Model:_`) | Model Gemini yang Dituju | Parameter `Model` di `invoke_subagent` | Peruntukan Utama |
| :--- | :--- | :--- | :--- |
| `Opus 5.5`, `Opus`, `Argon High`, `Gemini 4 Argon (High)` | **Gemini 4 Argon (High)** | `"inherit"` *(dropdown utama diatur ke Argon High)* atau `"pro"` | Deep reasoning, bedah arsitektur skema DB, root-cause bug rumit, security/financial audit, merge conflict |
| `Gemini 4 Argon`, `Argon`, `Argon (Medium)` | **Gemini 4 Argon** | `"inherit"` *(dropdown utama diatur ke Argon)* atau `"pro"` | Flagship agentic coding (1M token), fitur baru end-to-end, refactoring multi-file, analisis PRD tebal |
| `Gemini 4 Carbon`, `Carbon` | **Gemini 4 Carbon** | `"inherit"` *(manual assign only)* | Kasus eskalasi kritis / audit keamanan finansial tingkat tinggi |
| `Neon`, `Gemini 4 Neon (XHigh/High/Medium)` | **Gemini 4 Neon** | `"inherit"` *(dropdown utama diatur ke Neon)* atau `"pro"` | Penalaran adaptif gesit, optimasi query/algoritma rumit, refactoring presisi tinggi |
| `Sonnet 5.5`, `Sonnet`, `Hydrogen`, `Gemini 4 Hydrogen` | **Gemini 4 Hydrogen** / **Gemini 3.8 Flash (High)** | `"flash"` *(atau `"inherit"` jika dropdown utama Hydrogen)* | Daily driver seimbang: implementasi CRUD/fitur standar, wiring handler-service-repo, komponen React, bugfix terisolasi |
| `Flash`, `Gemini 3.8 Flash`, `Medium` | **Gemini 3.8 Flash** | `"flash"` | Cepat & hemat kuota: edit kode ringan, unit test standar, boilerplate |
| `Haiku 5.5`, `Haiku`, `Low`, `Flash Lite` | **Gemini 3.8 Flash Lite** | `"flash_lite"` | Task sangat ringan, formatting, typo, update dokumen non-kritis |
| Tidak ditentukan / `Inherit` | *(Sesuai session utama)* | `"inherit"` | Mewarisi model aktif session utama |

---

### Langkah 3: Auto-Assignment Spesialis & Skills
Pilih subagent spesialis dari `.agents/agents/` dan skill dari `.agents/skills/` berdasarkan cakupan file dan deskripsi task:

| Domain / File Target Task | Subagent Spesialis (`.agents/agents/`) | Skills yang Di-assign (`.agents/skills/`) | `Role` Subagent | `TypeName` |
| :--- | :--- | :--- | :--- | :--- |
| Spec SDLC 6-Tahap (`.agents/specs/<feature>/`) | `cms2-sdlc.md` | `cms2-sdlc`, `graphify` | `"CMS2 SDLC Architect"` | `"self"` |
| `migrations/`, `queries/*.sql`, `sqlc.yaml`, `internal/db/` | `db-sqlc-architect.md` | `db-migration-sqlc` (+ `sql-query-tuning` jika ada query berat/reporting) | `"DB & sqlc Architect"` | `"self"` |
| `internal/handler/`, `internal/service/`, `internal/repository/`, `cmd/api/` | `go-backend-specialist.md` | `go-chi-endpoint` (+ `cms2-master-data` jika menyentuh maker-checker/applier) | `"Go Backend Specialist"` | `"self"` |
| Entitas master data (`vendors`, `vendor_pks`, `vendor_vaults`, `atms`, dll.) | `go-backend-specialist.md` / `react-portal-specialist.md` | `cms2-master-data` | `"Master Data Specialist"` | `"self"` |
| Sinkronisasi OpenAPI backend -> `gen:api` frontend | `react-portal-specialist.md` | `openapi-frontend-sync` | `"OpenAPI Sync Specialist"` | `"self"` |
| Frontend React (`CompanyPortal-Vite/`, `VendorPortal-Vite/`, `.tsx`) | `react-portal-specialist.md` | `react-ui-ux-craft` | `"React Portal Specialist"` | `"self"` |
| Export async (`backend-exq/`), query lambat, tuning | `db-sqlc-architect.md` / `go-backend-specialist.md` | `sql-query-tuning`, `go-chi-endpoint` | `"Performance Specialist"` | `"self"` |
| Perbaikan bug / error / failing test | Sesuai layer (`go-backend-` / `react-portal-`) | `cms2-bugfix` | `"Bugfix Specialist"` | `"self"` |
| Audit keamanan, keuangan, rekonsiliasi, maker-checker | `security-fin-reviewer.md` | `cms2-sdlc` | `"Security & Fin Reviewer"` | `"research"` |
| Task umum / lintas modul | `cms-task-runner.md` | Sesuai domain | `"CMS Task Runner"` | `"self"` |

---

### Langkah 4: Tampilkan Rencana & Gate Dropdown Model
Tampilkan tabel rencana konfirmasi ke user:

```markdown
### Rencana Eksekusi Wave <N> (<Spec>)

| Task ID | Judul Task | Spesialis | Model Dokumen -> Parameter | Skills Aktif | Target File |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 1.1 | Migrasi skema PKS | db-sqlc-architect.md | Argon High -> "inherit" | db-migration-sqlc | backend/migrations/044_*.sql |
| 2.1 | Kontrak DTO Report | go-backend-specialist.md | Flash -> "flash" | go-chi-endpoint | pkg/reports/pks.go |
```

**Aturan Gate Dropdown Model Session**:
1. **Wave Murni `Flash`**: User dapat tetap berada di **Gemini 3.8 Flash** dan langsung membalas *"Lanjut"*.
2. **Wave dengan Model Gemini 4 (`Argon`, `Neon`, `Carbon`, atau `Hydrogen`)**:
   - Berikan pengingat jelas di bawah tabel:
     > ⚠️ **Pengingat Dropdown Model**: Wave ini memiliki task yang meminta model **Gemini 4 Argon / Neon** (via `Model: "inherit"`). Pastikan dropdown **Model Selection** di chat saat ini sudah Anda setel ke model tersebut sebelum membalas *"Lanjut"*. (Task `Flash` tetap berjalan hemat dengan `Model: "flash"`).
3. **Wave Campuran Multi-Model Gemini 4 (misal `Argon` + `Carbon`)**:
   - Pecah peluncuran menjadi **Kloter A** (model pertama + Flash), tunggu dispatch, lalu minta user ubah dropdown untuk **Kloter B**.

---

### Langkah 5: Dispatch Subagent (`invoke_subagent`)
Setelah user mengonfirmasi, panggil `invoke_subagent` untuk setiap task dalam wave tersebut:

- **Parameter**:
  - `TypeName`: `"self"` (untuk implementasi kode) atau `"research"` (untuk audit/review read-only).
  - `Workspace`: `"inherit"` (semua subagent bekerja di workspace yang sama dengan pembatasan file ketat).
  - `Model`: hasil pemetaan (`"inherit"`, `"pro"`, `"flash"`, `"flash_lite"`).
  - `Role`: `"<Nama Spesialis> Task <id>"` (contoh: `"DB Specialist Task 1.1"`).
- **Template Prompt Subagent**:
  ```markdown
  Anda dijalankan pada session baru oleh Gemini CMS Orchestrator.
  Baca dan patuhi aturan di:
  1. `.agents/rules/GEMINI.md`
  2. `.agents/agents/cms-task-runner.md`
  3. `.agents/agents/<spesialis>.md`

  Detail Penugasan:
  - Dokumen Spec: <path-spec>
  - Task ID: <id-task>
  - Model yang Diminta Dokumen: <label-model>
  - Skills Aktif: <daftar-skill> (wajib baca `.agents/skills/<skill>/SKILL.md` sebelum menulis kode)
  - Cakupan File Eksklusif: <daftar-file-target>

  Instruksi Penting:
  - Kerjakan HANYA cakupan Task <id-task>.
  - Jangan menyentuh file milik task lain yang berjalan paralel pada wave ini.
  - Dilarang keras melakukan `git commit`, `push`, `stash`, ganti branch, atau `migrate up` tanpa izin eksplisit user.
  - Jalankan test & linter pada modul yang disentuh (`go test ./...`, `golangci-lint`, `pnpm test|lint`).
  - Setelah verifikasi berhasil, perbarui centang `- [x]` pada baris task Anda di `tasks.md` dan tambahkan catatan di bawah `### Catatan eksekusi`.
  - Kirim laporan akhir terstruktur (File diubah, Status test, Outstanding).
  ```

---

### Langkah 6: Sinkronisasi Reaktif & Konsolidasi Hasil
1. **Reaktif Tanpa Polling**: Setelah memanggil `invoke_subagent`, hentikan giliran (*call no more tools*). Antigravity akan otomatis membangunkan Orchestrator saat subagent mengirim laporan.
2. **Klarifikasi**: Jika subagent mengirim pertanyaan via pesan, jawab atau teruskan ke user via `send_message`.
3. **Konsolidasi**:
   - Setelah seluruh subagent pada wave `N` selesai, periksa status centang `- [x]` pada `tasks.md`.
   - Sajikan laporan ringkas kepada user:
     - Daftar Task Selesai & Conversation ID masing-masing subagent.
     - Ringkasan perubahan file & status pengujian (automated tests green).
     - Daftar item *outstanding* (misal verifikasi browser manual).
   - Tanyakan persetujuan user untuk melanjutkan ke wave berikutnya (`Wave N+1`).
   - **Jangan pernah melakukan git commit otomatis.**
