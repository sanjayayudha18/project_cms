---
name: cms-run-wave
description: >-
  Dispatch one wave of tasks from a CMS2 spec's "## Task Dependency Graph"
  (e.g. .agents/specs/<feature>/tasks.md) using Antigravity subagents, each
  following .agents/agents/cms-task-runner.md. Use when the user says
  "jalankan wave N <spec>" or "run wave N".
---

# CMS Run Wave (Antigravity Subagent Dispatcher)

Skill ini menjalankan satu *wave* paralel dari bagian `## Task Dependency Graph` pada file `tasks.md` di dalam folder spec (misalnya `.agents/specs/<feature>/tasks.md`).

## Input
- Folder spec (default: `.agents/specs/<feature>/`) + nomor wave (`N`). Jika belum disebutkan oleh user, tanyakan terlebih dahulu.

## Langkah Eksekusi

1. **Baca `tasks.md`**: Ambil daftar ID task untuk wave `N` dari blok JSON di bawah `## Task Dependency Graph`.
2. **Cek Gate Wave Sebelumnya**: Pastikan seluruh task pada wave sebelum `N` sudah berstatus `[x]`. Jika masih ada yang `[ ]`, tampilkan daftarnya kepada user dan berhenti (kecuali user menginstruksikan untuk tetap lanjut).
3. **Petakan Model & Skill per Task**:
   - Periksa baris `_Model:` pada task (atau warisi dari parent task-nya) dan petakan ke parameter `Model` pada tool `invoke_subagent` Antigravity:
     - Kompleks / Arsitektur / High effort (`Opus` / `Pro`) -> `Model: "pro"`
     - Standar / Implementasi menengah (`Sonnet` / `Flash`) -> `Model: "flash"`
     - Ringan / Boilerplate (`Haiku` / `Flash Lite`) -> `Model: "flash_lite"`
     - Jika tidak ditentukan -> `Model: "inherit"`
   - Lewati (dan beri tahu user) task yang sudah `[x]` atau task yang ditandai sebagai pengecekan manual user.
   - Pilih skill yang relevan dari `.agents/skills/` berdasarkan area yang disentuh oleh task (misalnya `cms2-master-data` jika menyentuh maker-checker master data).
4. **Konfirmasi ke User**: Tampilkan tabel rencana eksekusi wave (`Task ID` · `Judul` · `Model Subagent` · `Skills`) dan tunggu persetujuan user.
5. **Dispatch Subagent**: Setelah user menyetujui, panggil tool `invoke_subagent` (`TypeName: "self"`, `Workspace: "inherit"`) untuk setiap task di dalam wave tersebut dengan prompt:
   > Bertindaklah sebagai `cms-task-runner`: baca dan patuhi `.agents/agents/cms-task-runner.md` serta `.agents/rules/GEMINI.md`.
   > - `spec`: `<path-spec>`
   > - `task`: `<id-task>`
   > - `skills`: `<daftar-skill-yang-disetujui>` (baca `.agents/skills/<skill>/SKILL.md` terkait sebelum menulis kode).
   > Task lain dalam wave yang sama mungkin berjalan paralel di workspace ini: hanya ubah file yang menjadi cakupan task Anda, jangan melakukan git commit, dan jangan berpindah branch.
6. **Konsolidasi Hasil**: Setelah semua subagent pada wave `N` selesai dan mengirimkan laporan, periksa status centang `[x]` di `tasks.md`, rangkum hasilnya kepada user (selesai / terblokir / test yang di-skip), dan tanyakan apakah ingin melanjutkan ke wave berikutnya. Jangan pernah melakukan commit otomatis.
