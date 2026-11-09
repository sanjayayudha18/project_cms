# Struktur Folder `.agents/` (Google Antigravity / Gemini)

Folder `.agents/` adalah pusat konfigurasi, aturan (*rules*), *skills*, *plugins*, *hooks*, serta artefak SDLC khusus **Gemini / Google Antigravity** pada proyek **CMS2 (CROWN)**. Folder ini terpisah sepenuhnya dari `.claude/` dan `.kiro/`.

## Peta Direktori

```text
.agents/
├── README.md                    # Panduan struktur .agents/
├── hooks.json                   # Konfigurasi lifecycle hooks Antigravity
├── skills.json                  # Manifest pendaftaran path skills
├── plugins.json                 # Manifest pendaftaran path plugins
├── bugfixes.md                  # Log bugfix yang dikerjakan oleh Gemini
├── development-progress.md      # Tracking status fitur yang dikerjakan oleh Gemini
├── lessons-learned.md           # Catatan pembelajaran dari review/insiden
├── agents/
│   ├── cms-orchestrator.md      # Orchestrator pembuka session baru sesuai task & model di dokumen
│   ├── cms-task-runner.md       # Instruksi dasar eksekutor task per wave
│   ├── db-sqlc-architect.md     # Subagent spesialis PostgreSQL, migrasi & sqlc v1.31.1
│   ├── go-backend-specialist.md # Subagent spesialis Go + Chi v5 (backend/, backend-exq/, pkg/)
│   ├── react-portal-specialist.md # Subagent spesialis React + Vite & UI/UX kedua portal
│   └── security-fin-reviewer.md # Subagent auditor keamanan & aturan finansial (read-only)
├── rules/
│   └── GEMINI.md                # Aturan proyek (auto-loaded oleh Antigravity, batas ≤ 24 KB/file)
├── skills/
│   ├── cms2-sdlc/
│   │   └── SKILL.md             # Driver 6-tahap AI-Native SDLC
│   ├── cms2-master-data/
│   │   └── SKILL.md             # Checklist 9-langkah entitas master-data (maker-checker)
│   ├── cms2-bugfix/
│   │   └── SKILL.md             # Workflow end-to-end perbaikan bug
│   ├── cms-run-wave/
│   │   └── SKILL.md             # Dispatcher paralel task per wave menggunakan subagent
│   ├── db-migration-sqlc/
│   │   └── SKILL.md             # Workflow migrasi PostgreSQL, soft-delete & sqlc v1.31.1
│   ├── go-chi-endpoint/
│   │   └── SKILL.md             # Checklist pembuatan endpoint Go + Chi v5 & unit test
│   ├── openapi-frontend-sync/
│   │   └── SKILL.md             # Sinkronisasi kontrak OpenAPI Go ke TypeScript React
│   ├── react-ui-ux-craft/
│   │   └── SKILL.md             # Standar UI/UX React + Vite (4 state wajib, IDR, portal)
│   └── sql-query-tuning/
│       └── SKILL.md             # Panduan optimasi query SQL, indexing & routing dbRead
├── plugins/
│   └── .gitkeep                 # Tempat menyimpan bundle plugin lokal
└── specs/
    ├── README.md                # Panduan siklus hidup spec fitur di .agents/specs/
    └── done/
        └── .gitkeep             # Arsip spec fitur yang sudah selesai
```

## Ketentuan Penting Antigravity

1. **Batas Ukuran File Rule (`24 KB`)**: Setiap file `.md` di dalam `.agents/rules/` dibatasi maksimal **24.000 bytes (24 KB)**. Jika aturan bertambah panjang, pecah menjadi beberapa file modular (misalnya `01-core.md`, `02-data-map.md`, `03-ui.md`).
2. **Progressive Disclosure pada Skills**: Hanya `name` dan `description` pada YAML frontmatter `SKILL.md` yang dimuat di awal sesi; isi lengkap `SKILL.md` baru dibaca saat skill diaktifkan.
3. **Isolasi Artefak**: Semua spec, progress, dan log bugfix yang dijalankan melalui Gemini disimpan di dalam `.agents/` agar tidak bentrok dengan `.claude/` maupun `.kiro/`.
