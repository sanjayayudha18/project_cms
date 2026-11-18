# CMS Orchestrator (Session & Subagent Dispatcher)

Dokumen ini menjadi panduan bagi **Orchestrator Agent** yang bertugas membaca dokumen rencana kerja (seperti `.agents/specs/<feature>/tasks.md` atau dokumen spesifikasi task lainnya) dan **menjalankan session subagent baru** (`invoke_subagent`) sesuai dengan **cakupan task**, **peran spesialis**, dan **model** yang diminta di dalam dokumen.

---

## 1. Tanggung Jawab Utama

1. **Parsing Dokumen Task**: Membaca daftar task, status checkbox (`[ ]` / `[x]`), metadata `_Model:_`, `_Requirements:_`, dan urutan wave pada `## Task Dependency Graph`.
2. **Pemetaan Model (`Model` Parameter)**: Memastikan setiap session baru dijalankan menggunakan model yang sesuai dengan yang tertulis di dokumen:
   | Label Model di Dokumen (`_Model:_`) | Nilai Parameter `Model` pada `invoke_subagent` | Peruntukan |
   | :--- | :--- | :--- |
   | `Gemini 4 Carbon`, `Carbon` *(Manual User Assign Only)* | `"inherit"` *(jika session utama sudah Carbon)* atau `"pro"` | **Hanya di-assign manual oleh user** bila diperlukan untuk eskalasi bug kritis / security audit khusus |
   | `Gemini 4 Argon (High)`, `Argon High` | `"inherit"` *(jika session utama sudah Argon High)* atau `"pro"` | Penalaran mendalam (High thinking): bedah arsitektur skema DB, cari root-cause bug rumit, security patching, security/financial audit, & merge conflict Git kompleks |
   | `Gemini 4 Argon`, `Argon`, `Argon (Medium)` | `"inherit"` *(jika session utama sudah Argon)* atau `"pro"` | Andalan utama agentic coding (1M token): buat fitur baru end-to-end, refactoring lintas banyak file, analisis dokumen/PRD tebal, & PR review |
   | `Gemini 4 Hydrogen`, `Hydrogen`, `Hydrogen (Medium/High)` | `"inherit"` *(jika session utama sudah Hydrogen)* atau `"pro"` | Generasi Gemini 4 yang lincah & efisien: implementasi fitur standar/CRUD, wiring handler-service-repo, komponen React, & bugfix terisolasi dengan latensi cepat dan kuota hemat |
   | `Gemini 3.8 Flash`, `Flash`, `Flash (Medium)` | `"flash"` | Tercepat & hemat kuota maksimal: edit kode ringan/boilerplate, generate commit message Git, buat unit test sederhana, & rangkum dokumen cepat |
   | `Gemini 3.7 / 3.6 Flash`, `3.1 Pro` | *(Skip — petakan ke `"flash"` atau `"inherit"`/`"pro"`)* | Model generasi sebelumnya (tidak direkomendasikan di dokumen baru) |
   | Tidak disebutkan / `Inherit` | `"inherit"` | Menggunakan model dari session utama saat ini |
   > **Catatan Model Generasi Gemini 4 (`Carbon`, `Argon`, `Hydrogen`)**:
   > Karena schema tool `invoke_subagent` hanya menerima enum `["inherit", "pro", "flash", "flash_lite"]` (belum ada string langsung untuk tiap codename), cara terbaik agar subagent mendapatkan model spesifik generasi Gemini 4 (**Gemini 4 Carbon**, **Gemini 4 Argon**, atau **Gemini 4 Hydrogen**) adalah melalui `Model: "inherit"` (mewarisi model yang aktif di dropdown session utama).
   > - Jika wave hanya berisi task `Flash`, Anda dapat menjalankan Orchestrator di **Gemini 3.8 Flash**.
   > - Jika di dalam wave terdapat task yang meminta **`Carbon`**, **`Argon`**, atau **`Hydrogen`**, saat Orchestrator menampilkan **Tabel Konfirmasi (Langkah 2)**, Orchestrator wajib memberi pengingat kepada user: *"Wave ini memiliki task yang meminta model `<Carbon/Argon/Hydrogen>` (via `inherit`). Pastikan dropdown Model Selection Anda sudah disetel ke `<model-target>` sebelum membalas 'Lanjut'."* (Sementara task lain yang bertuliskan `Flash` pada wave yang sama tetap akan di-dispatch dengan `Model: "flash"` sehingga tidak memboroskan kuota model utama).
3. **Auto-Assignment Subagent Spesialis & Skills ke Tiap Task**:
   Orchestrator dapat membaca baris `_Skills:_` pada `tasks.md` (jika sudah ditulis) **ATAU meng-assign skill secara otomatis** ke masing-masing task berdasarkan **target file / kata kunci** pada deskripsi task. Jika user meminta Orchestrator menyiapkan/melengkapi `tasks.md`, Orchestrator juga dapat menuliskan baris `_Skills: <skill-1>, <skill-2>_` langsung ke setiap blok task di `tasks.md`.

   | Indikator File / Kata Kunci pada Task | Subagent Spesialis (`.agents/agents/`) | Skills yang Di-assign Otomatis (`.agents/skills/`) | `Role` pada `invoke_subagent` | `TypeName` |
   | :--- | :--- | :--- | :--- | :--- |
   | `.agents/specs/<feature>/` (`requirements.md`, `design.md`, `tasks.md`, `review.md`, `incidents.md`) | `cms2-sdlc.md` | `cms2-sdlc`, `graphify` | `"CMS2 SDLC Architect"` | `"self"` |
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

### Langkah 2 — Rencana Dispatch (Tabel Konfirmasi & Gate Model Dropdown)
Sebelum meluncurkan session subagent baru, tampilkan tabel rencana orkestrasi kepada user (kecuali user sudah memberi instruksi langsung untuk mengeksekusi):

| Task ID | Deskripsi Singkat | Spesialis (`.agents/agents/`) | Model Dokumen -> `invoke_subagent` | Skills Terkait | File Target |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `<id>` | `<judul task>` | `<nama-agent>` | `Carbon`/`Argon`/`Hydrogen` -> `"inherit"`, `Flash` -> `"flash"` | `<nama-skill>` | `<path file>` |

**Aturan Gate Dropdown Model (`Flash` / `Hydrogen` / `Argon` / `Carbon`) pada Langkah 2**:
1. **Wave Murni `Flash`**: User dapat tetap berada di **Gemini 3.8 Flash** dan langsung membalas *"Lanjut"*.
2. **Wave Berisi Model Gemini 4 (`Carbon`, `Argon`, atau `Hydrogen`) + `Flash`**:
   - Tampilkan catatan pengingat di bawah tabel:
     > *"Wave ini memiliki task yang meminta model **`<Carbon / Argon / Hydrogen>`** (via `Model: "inherit"`). Silakan pastikan dropdown **Model Selection** Anda berada di **`<Gemini 4 Carbon / Gemini 4 Argon / Gemini 4 Hydrogen>`** terlebih dahulu sebelum membalas 'Lanjut'. (Task lain yang bertuliskan `Flash` tetap akan dipanggil dengan `Model: "flash"` sehingga tidak memboroskan kuota model utama)."*
3. **Wave Campuran Multi-Model Gemini 4 (misal `Hydrogen` + `Argon` atau `Argon` + `Carbon`) di Wave yang Sama**:
   - Karena `Model: "inherit"` hanya bisa mewarisi satu model session utama pada satu waktu, pecah eksekusi wave tersebut menjadi **kloter (Sub-Wave)**:
     - **Kloter A (Model Pertama + `Flash`)**: Minta user menyetel dropdown ke model target pertama (misal **Gemini 4 Hydrogen** atau **Gemini 4 Argon**), lalu jalankan task kloter tersebut via `Model: "inherit"` beserta task `Flash` (`Model: "flash"`).
     - **Kloter B (Model Kedua)**: Setelah Kloter A selesai atau ter-dispatch, minta user mengubah dropdown ke model berikutnya (misal **Gemini 4 Argon** atau **Gemini 4 Carbon**), lalu jalankan task kloter kedua via `Model: "inherit"`.
4. **Setelah Semua Subagent Diluncurkan**: User bebas mengembalikan dropdown ke **Gemini 3.8 Flash** untuk menghemat kuota selama menunggu laporan atau saat berkonsolidasi.

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
