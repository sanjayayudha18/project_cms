---
name: cms-run-wave
description: >-
  Dispatch one wave of tasks from a CMS2 spec's "## Task Dependency Graph"
  (e.g. .agents/specs/<feature>/tasks.md) using Antigravity subagents, each
  following .agents/agents/cms-task-runner.md. Use when the user says
  "jalankan wave N <spec>" or "run wave N".
---

# CMS Run Wave (Antigravity Subagent Dispatcher)

Skill ini menjalankan satu *wave* paralel (atau daftar task terpilih) dari bagian `## Task Dependency Graph` pada file `tasks.md` di dalam folder spec (misalnya `.agents/specs/<feature>/tasks.md`) dengan mengikuti panduan orkestrasi di **`.agents/agents/cms-orchestrator.md`**.

## Input
- Folder spec (default: `.agents/specs/<feature>/`) + nomor wave (`N`) atau ID task. Jika belum disebutkan oleh user, tanyakan terlebih dahulu.

## Langkah Eksekusi

1. **Baca Panduan Orchestrator & `tasks.md`**:
   - Baca `.agents/agents/cms-orchestrator.md`.
   - Ambil daftar ID task untuk wave `N` dari blok JSON di bawah `## Task Dependency Graph` pada `tasks.md`.
2. **Cek Gate Wave Sebelumnya**: Pastikan seluruh task pada wave sebelum `N` sudah berstatus `[x]`. Jika masih ada yang `[ ]`, tampilkan daftarnya kepada user dan berhenti (kecuali user menginstruksikan untuk tetap lanjut).
3. **Petakan Model, Subagent Spesialis & Skill per Task**:
   - Periksa baris `_Model:_` pada task (atau warisi dari parent task-nya) dan petakan ke parameter `Model` pada tool `invoke_subagent` Antigravity:
     - Model Codename / Deep Reasoning (`Gemini 4 Argon (High)` / `Argon High` / `Gemini 4 Argon` / `Argon`, atau `Carbon` bila di-assign manual oleh user) -> `Model: "inherit"` (mewarisi `Argon (High/Medium)` atau `Carbon` dari session utama; beri catatan pengingat ke user pada langkah konfirmasi jika dropdown model session utama saat ini belum sesuai) atau fallback `"pro"`
     - Kompleks / Arsitektur / High effort (`Opus` / `Pro` / `High`) -> `Model: "pro"`
     - Standar / Implementasi menengah (`Sonnet` / `Flash` / `Medium`) -> `Model: "flash"`
     - Ringan / Boilerplate (`Haiku` / `Flash Lite` / `Low`) -> `Model: "flash_lite"`
     - Jika tidak ditentukan -> `Model: "inherit"`
   - Pilih subagent spesialis dari `.agents/agents/` berdasarkan domain file yang disentuh oleh task:
     - Database / Migrasi / `sqlc` -> `db-sqlc-architect.md`
     - Go Backend (`backend/`, `backend-exq/`, `pkg/`) -> `go-backend-specialist.md`
     - React Frontend (`CompanyPortal-Vite`, `VendorPortal-Vite`) -> `react-portal-specialist.md`
     - Audit / Review Keamanan & Finansial -> `security-fin-reviewer.md`
     - Lintas layer / umum -> `cms-task-runner.md`
   - Lewati (dan beri tahu user) task yang sudah `[x]` atau task yang ditandai sebagai pengecekan manual user.
   - Pilih skill yang relevan dari `.agents/skills/` berdasarkan area yang disentuh oleh task (misalnya `cms2-master-data` jika menyentuh maker-checker master data).
4. **Konfirmasi ke User & Gate Dropdown Model**:
   - Tampilkan tabel rencana eksekusi wave (`Task ID` · `Judul` · `Spesialis` · `Model Dokumen -> Parameter Subagent` · `Skills`) dan tunggu persetujuan user.
   - Jika di dalam wave terdapat task yang meminta **`Carbon`** atau **`Argon`** (yang dipetakan ke `Model: "inherit"`), tampilkan pengingat agar user menyetel dropdown *Model Selection* ke **`Gemini 4 Carbon`** atau **`Gemini 4 Argon`** sebelum membalas *"Lanjut"* (task `Flash` tetap dipanggil dengan `Model: "flash"` agar hemat kuota).
   - Jika satu wave memiliki task **`Argon` sekaligus `Carbon`**, pecah peluncurannya menjadi 2 kloter konfirmasi (**Kloter A: `Argon` + `Flash`**, lalu **Kloter B: `Carbon`**).
5. **Dispatch Session Subagent Baru**: Setelah user menyetujui, panggil tool `invoke_subagent` (`TypeName: "self"` untuk implementasi atau `"research"` untuk audit read-only, `Workspace: "inherit"`, serta `Model` sesuai pemetaan di atas) untuk setiap task di dalam wave tersebut dengan prompt:
   > Bertindaklah sebagai subagent spesialis pada session baru ini: baca dan patuhi `.agents/rules/GEMINI.md`, `.agents/agents/cms-task-runner.md`, serta `.agents/agents/<spesialis>.md`.
   > - `spec`: `<path-spec>`
   > - `task`: `<id-task>`
   > - `model`: `<model-yang-diminta-dokumen>`
   > - `skills`: `<daftar-skill-yang-disetujui>` (baca `.agents/skills/<skill>/SKILL.md` terkait sebelum menulis kode).
   > Task lain dalam wave yang sama mungkin berjalan paralel di workspace ini: hanya ubah file yang menjadi cakupan task Anda, jangan melakukan git commit, dan jangan berpindah branch.
6. **Konsolidasi Hasil**: Setelah semua subagent pada wave `N` selesai dan mengirimkan laporan, periksa status centang `[x]` di `tasks.md`, rangkum hasilnya kepada user (selesai / terblokir / test yang di-skip), dan tanyakan apakah ingin melanjutkan ke wave berikutnya. Jangan pernah melakukan commit otomatis.
