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
├── changes.md                   # Log perubahan kecil bukan bug (Small Work Dev)
├── development-progress.md      # Tracking status fitur yang dikerjakan oleh Gemini
├── lessons-learned.md           # Catatan pembelajaran dari review/insiden
├── agents/
│   ├── gemini-cms-devplanner.md # Agent perancang & pengawal AI-Native SDLC (5 Metode Pengembangan)
│   ├── gemini-cms-orchestrator.md # Orchestrator pembuka session baru sesuai task & model di dokumen
│   ├── cms-task-runner.md       # Instruksi dasar eksekutor task per wave
│   ├── db-sqlc-architect.md     # Subagent spesialis PostgreSQL, migrasi & sqlc v1.31.1
│   ├── go-backend-specialist.md # Subagent spesialis Go + Chi v5 (backend/, backend-exq/, pkg/)
│   ├── react-portal-specialist.md # Subagent spesialis React + Vite & UI/UX kedua portal
│   └── security-fin-reviewer.md # Subagent auditor keamanan & aturan finansial (read-only)
├── rules/
│   ├── GEMINI.md                # Aturan utama & single source of truth (auto-loaded oleh Antigravity, batas ≤ 24 KB/file)
│   ├── graphify.md              # Aturan navigasi knowledge graph AST lokal
│   └── steering/                # Panduan teknis domain mendalam:
│       ├── database.md          # Skema DB, SQLC, topologi db/dbRead, migrasi 001-045
│       ├── backend.md           # Go workspace (go.work), Flat vs Envelope JSON, Python EOD, deploy GCP
│       ├── security-compliance.md # Maker-checker, audit logs atomik, deviasi GR#3, RBAC, LDAP vs lokal
│       ├── frontend.md          # Dual portal CompanyPortal vs VendorPortal, brand, OpenAPI sync, 4 UI state
│       └── sdlc-protocols.md    # 5 Metode SDLC, sequential human gate, the leash Sec 4, tech-debt
├── scopes/                      # Scope-Driven Dev (scope.md & tasks.md untuk target MVP/Timebox)
│   └── done/
├── ui-revamps/                  # UI Revamp Dev (audit.md, design-artifact.md, ui-spec.md, tasks.md)
│   └── done/
├── skills/
│   ├── cms2-sdlc/
│   │   └── SKILL.md             # Driver 6-tahap AI-Native SDLC
│   ├── cms2-master-data/
│   │   └── SKILL.md             # Checklist 9-langkah entitas master-data (maker-checker)
│   ├── cms2-bugfix/
│   │   └── SKILL.md             # Workflow end-to-end perbaikan bug
│   ├── gemini-cms-orchestrator/
│   │   └── SKILL.md             # Dispatcher paralel task per wave menggunakan subagent Gemini (Antigravity)
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
    ├── gemini-sdlc.md           # Panduan siklus hidup spec fitur & 6-stage AI-Native SDLC di .agents/specs/
    └── done/
        └── .gitkeep             # Arsip spec fitur yang sudah selesai
```

## Ketentuan Penting Antigravity

1. **Batas Ukuran File Rule (`24 KB`)**: Setiap file `.md` di dalam `.agents/rules/` dibatasi maksimal **24.000 bytes (24 KB)**. Jika aturan bertambah panjang, pecah menjadi beberapa file modular (misalnya `01-core.md`, `02-data-map.md`, `03-ui.md`).
2. **Progressive Disclosure pada Skills**: Hanya `name` dan `description` pada YAML frontmatter `SKILL.md` yang dimuat di awal sesi; isi lengkap `SKILL.md` baru dibaca saat skill diaktifkan.
3. **Isolasi Artefak**: Semua spec, progress, dan log bugfix yang dijalankan melalui Gemini disimpan di dalam `.agents/` agar tidak bentrok dengan `.claude/` maupun `.kiro/`.
