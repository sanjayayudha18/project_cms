# Spesifikasi Fitur (`.agents/specs/`)

Folder ini menyimpan artefak siklus hidup pengembangan fitur (6-Stage AI-Native SDLC) yang dijalankan melalui **Gemini / Google Antigravity**, terpisah dari `.claude/` dan `.kiro/`.

## Struktur Folder per Fitur

Setiap fitur baru dibuat di dalam subfolder kebab-case `.agents/specs/<nama-fitur>/`:

```text
.agents/specs/<nama-fitur>/
├── requirements.md   # Tahap 1 (Introduction + Open Questions + EARS Requirements)
├── design.md         # Tahap 2 (Arsitektur, Kontrak API, Model Data, Correctness Properties)
├── tasks.md          # Tahap 3 (Rencana Task, Model Split, Task Dependency Graph) & Tahap 4 (Test Results)
├── review.md         # Tahap 5 (Review Passes, Findings, Definition of Done)
└── incidents.md      # Tahap 6 (Signals, Incident Log, Regression Test)
```

## Disiplin Gate

- **Satu Dokumen per Langkah**: Tulis artefak satu per satu secara berurutan (`requirements.md` selesai & di-review dulu → baru buat `design.md` → baru buat `tasks.md`).
- Setiap dokumen diawali dengan baris status di bawah judul utama:
  `> Status: draft`
- Hanya **manusia (user/PO/engineer)** yang berhak mengubah status menjadi:
  `> Status: accepted YYYY-MM-DD (<role>)`
- Agen tidak pernah menyetujui artefak sendiri atau melompati tahap sebelum tahap sebelumnya berstatus `accepted`.
- Setelah seluruh tahap fitur selesai, pindahkan folder `.agents/specs/<nama-fitur>/` ke `.agents/specs/done/<nama-fitur>/`.

## Panduan Pemilihan LLM pada `tasks.md`

Saat menyusun bagian `## Model Split` dan metadata `_Model:_` pada tiap task di `tasks.md`, gunakan acuan pemilihan model Gemini terbaru berikut:

> **Aturan Penting**: **Gemini 4 Neon dan Gemini 4 Carbon hanya akan di-assign secara manual oleh user.** Agen dilarang meng-assign `Neon` atau `Carbon` secara otomatis saat menyusun `tasks.md`; gunakan **`Gemini 4 Argon (High)`** untuk seluruh task berat/kritis yang membutuhkan penalaran tingkat tinggi.

| Model | Pilihan Level Thinking | Keunggulan Utama | Kapan Harus Dipakai? |
| :--- | :--- | :--- | :--- |
| **Gemini 4 Neon** *(Ultra-Frontier — Manual User Only)* | `Low`, `Medium`, `High`, `XHigh` | Kasta tertinggi keluarga Gemini 4 dengan mode penalaran ekstrem hingga `XHigh` untuk logika multi-tahap & finansial paling rumit. | **Hanya di-assign manual oleh user** untuk eskalasi masalah arsitektur/ekonomi/logika ekstrem. |
| **Gemini 4 Carbon** *(Cyber & Deep Reasoning — Manual User Only)* | `Low`, `Medium`, `High` | Penalaran sangat tajam, minim halusinasi, spesialis *security audit*, *vulnerability patching*, & *bug* kritis. | **Hanya di-assign manual oleh user** bila diperlukan untuk eskalasi bug kritis/audit keamanan khusus. |
| **Gemini 4 Argon (High)** *(Default Deep Reasoning di `tasks.md`)* | `High` | Penalaran mendalam + kapasitas *agentic coding* skala besar (1M token). | **Default task berat**: bedah arsitektur skema DB, cari *root-cause bug* rumit, *security & financial audit*, serta *merge conflict* Git kompleks. |
| **Gemini 4 Argon** *(Andalan Utama)* | `Medium`, `Low` | Keseimbangan terbaik antara kecerdasan tinggi & *agentic coding* skala besar (1M token). | **Default fitur**: buat fitur baru *end-to-end*, *refactoring* lintas banyak file, analisis dokumen/PRD tebal, & *PR review*. |
| **Gemini 4 Hydrogen** *(Tercepat Gen-4)* | `Low`, `Medium`, `High` | Evolusi langsung generasi ke-4 pengganti *Flash*: sangat cepat, hemat kuota, dioptimalkan untuk koding harian & *agentic workflow* ringan. | Edit kode/konfigurasi ringan, *generate commit message* Git, buat unit test sederhana, & rangkum dokumen cepat. |
| **Gemini 3.8 Flash** *(Cepat Gen-3.8)* | `Low`, `Medium`, `High` | Respons instan & hemat kuota (digunakan otomatis oleh subagent `Model: "flash"`). | Task ringan pararel via subagent `flash`, edit katalog/konfigurasi sederhana. |
| **Gemini 3.7 / 3.6 Flash & 3.1 Pro** | `Low`, `Medium`, `High` | Model generasi sebelumnya. | *Skip saja* — gunakan **4 Hydrogen / 3.8 Flash** untuk kecepatan atau **4 Argon** untuk kualitas. |
