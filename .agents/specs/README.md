# Spesifikasi Fitur (`.agents/specs/`)

Folder ini menyimpan artefak siklus hidup pengembangan fitur (6-Stage AI-Native SDLC) yang dijalankan melalui **Gemini / Google Antigravity**, terpisah dari `.claude/` dan `.kiro/`.

## Struktur Folder per Fitur

Setiap fitur baru dibuat di dalam subfolder kebab-case `.agents/specs/<nama-fitur>/`:

```text
.agents/specs/<nama-fitur>/
├── requirements.md   # Tahap 1 (Introduction + Open Questions) & Tahap 2 (EARS Requirements)
├── design.md         # Tahap 2 (Arsitektur, Kontrak API, Model Data, Correctness Properties)
├── tasks.md          # Tahap 3 (Rencana Task, Model Split, Task Dependency Graph) & Tahap 4 (Test Results)
├── review.md         # Tahap 5 (Review Passes, Findings, Definition of Done)
└── incidents.md      # Tahap 6 (Signals, Incident Log, Regression Test)
```

## Disiplin Gate

- Setiap dokumen diawali dengan baris status di bawah judul utama:
  `> Status: draft`
- Hanya **manusia (user/PO/engineer)** yang berhak mengubah status menjadi:
  `> Status: accepted YYYY-MM-DD (<role>)`
- Agen tidak pernah menyetujui artefak sendiri atau melompati tahap sebelum tahap sebelumnya berstatus `accepted`.
- Setelah seluruh tahap fitur selesai, pindahkan folder `.agents/specs/<nama-fitur>/` ke `.agents/specs/done/<nama-fitur>/`.
