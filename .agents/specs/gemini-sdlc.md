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

Saat menyusun bagian `## Model Split` dan metadata `_Model:_` pada tiap task di `tasks.md`, gunakan acuan pemilihan model berikut:

> **Aturan Penting**: **Gemini 4 Carbon hanya akan di-assign secara manual oleh user.** Agen dilarang meng-assign `Gemini 4 Carbon` secara otomatis saat menyusun `tasks.md`; gunakan **`Gemini 4 Argon (High)`** untuk seluruh task berat/kritis yang membutuhkan penalaran tingkat tinggi.

| Model | Keunggulan Utama | Kapan Harus Dipakai? |
| :--- | :--- | :--- |
| **Gemini 4 Carbon (Terkuat — Manual User Only)** | Penalaran paling tajam, minim halusinasi, jago security audit & bug kritis. | **Hanya di-assign manual oleh user** bila diperlukan untuk eskalasi bug kritis/audit khusus. |
| **Gemini 4 Argon (High)** | Penalaran mendalam (High thinking) + kapasitas agentic coding skala besar (1M token). | Bedah arsitektur skema DB, cari root-cause bug rumit, security & financial audit, serta merge conflict Git yang kompleks. |
| **Gemini 4 Argon (Andalan Utama / Medium)** | Keseimbangan terbaik antara kecerdasan tinggi & agentic coding skala besar (1M token). | Buat fitur baru end-to-end, refactoring lintas banyak file, analisis dokumen/PRD tebal, & PR review. |
| **Gemini 3.8 Flash (Tercepat)** | Respons instan, ringan, hemat kuota (versi terbaik dibanding 3.7 / 3.6 Flash). | Edit kode ringan, generate commit message Git, buat unit test sederhana, & rangkum dokumen cepat. |
| **Gemini 3.7 / 3.6 Flash & 3.1 Pro** | Model generasi sebelumnya. | Skip saja — gunakan 3.8 Flash untuk kecepatan atau 4 Argon untuk kualitas. |
