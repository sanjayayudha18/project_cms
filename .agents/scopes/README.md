# Scope-Driven Development (`.agents/scopes/`)

Folder ini menyimpan artefak pengembangan berbasis batasan scope (*Scope-Driven Development*) yang dikerjakan melalui **Gemini / Google Antigravity**.

Metode ini digunakan untuk fitur baru atau enhancement yang memiliki batasan waktu/milestone ketat (MVP/Timebox), tanpa menyentuh arsitektur inti finansial atau migrasi database besar, dengan fokus pada prioritas *Must-Have* vs *Nice-to-Have* (MoSCoW) guna mencegah *scope creep*.

## Struktur Folder per Scope

Setiap inisiatif scope dibuat di dalam subfolder kebab-case `.agents/scopes/<nama-scope>/`:

```text
.agents/scopes/<nama-scope>/
├── scope.md   # Batasan MVP: Problem, In-Scope, Out-of-Scope, MoSCoW Prioritization, EARS Criteria
└── tasks.md   # Build Plan ringkas: Task Breakdown, Model Split, Automated Tests, Test Results
```

Setelah selesai di-review dan di-merge, folder scope dipindahkan ke `.agents/scopes/done/<nama-scope>/`.
