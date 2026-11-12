---
name: cms2-sdlc
description: >-
  Start or advance a CMS2 feature through the 6-stage AI-Native SDLC inside
  .agents/specs/<feature>/ (requirements -> design -> tasks -> test results ->
  review -> incidents), stopping at every human gate. Use when the user starts
  a new feature, asks for a requirements/design/tasks/review/incidents artifact,
  or says "fitur baru" / "spec baru" / "AI-DLC baru" / "lanjut tahap".
---

# CMS2 SDLC Driver (Khusus Gemini / `.agents/`)

Sumber kebenaran: `.agents/rules/GEMINI.md`. Seluruh artefak fitur baru yang dikerjakan oleh Gemini disimpan di `.agents/specs/<feature>/` agar terpisah dari `.claude/` dan `.kiro/`.

## Langkah 0 — Selalu Jalankan Sebelum Menulis Kode

1. Tanyakan kepada user:
   *"Perlu dibuat spec baru (`.agents/specs/<feature>/`) atau cukup dokumentasi bug fix (`.agents/bugfixes.md`)?"*
   Tunggu jawaban user.
   - Jika bug fix -> gunakan skill `cms2-bugfix` dan catat di `.agents/bugfixes.md`.
   - Perubahan yang menyentuh autentikasi, uang/jurnal, rekonsiliasi, migrasi/skema database, atau penghapusan data -> **wajib** melalui alur spec penuh.
2. Periksa `.agents/specs/<feature>/` (dan `.agents/specs/done/`). Jika sudah ada, cari **file terakhir dengan `> Status: accepted`** lalu lanjutkan dari tahap berikutnya.
   Jika kode dan artefak tidak sinkron -> **BERHENTI** dan tanyakan ke user.
3. Nama folder menggunakan format `kebab-case` (misalnya `vendor-pks`, `vendor-export-pks`).

## 6 Tahap SDLC

Setiap file diawali dengan `# <Judul>` lalu baris `> Status: draft`. Hanya user yang mengubahnya menjadi `> Status: accepted YYYY-MM-DD (<role>)`.
**Aturan Satu Dokumen per Giliran**: Tulis satu dokumen artefak dalam satu waktu; selesaikan dan tunggu user me-review serta menyatakan `accepted` pada `requirements.md` sebelum mulai menulis `design.md`, lalu tunggu `design.md` disetujui sebelum menulis `tasks.md`. Dilarang membuat dua file artefak berbeda sekaligus dalam satu giliran.

| # | Artefak (`.agents/specs/<feature>/`) | Gate Manusia (Agen Tidak Pernah Self-Approve) | Tugas Agen |
| :--- | :--- | :--- | :--- |
| **1 Plan & Requirements** | `requirements.md` (`## Introduction`, `## Open Questions`, & EARS Requirements) | PO/User menjawab open questions & menyetujui `requirements.md` (`accepted`) | Tangkap masalah, hasil yang diharapkan, batasan, open questions, dan tulis EARS requirements setelah open questions terjawab. **Berhenti dan tunggu review user sebelum membuat `design.md`.** |
| **2 Design** | `design.md` | Engineer/User menyetujui `design.md` (`accepted`) | Tulis kontrak API, model data, Correctness Properties, out-of-scope; tandai area kritis (auth/uang/recon/migrasi/delete). **Berhenti dan tunggu review user sebelum membuat `tasks.md`.** |
| **3 Build** | `tasks.md` | Engineer menyetujui tasks | Susun daftar task dengan `_Requirements:_`, `_Model:_`, `_Skills:_` (sesuai matriks di `cms-orchestrator.md`), dan `## Task Dependency Graph`. Eksekusi per task/wave dengan diff kecil. |
| **4 Test** | `tasks.md` -> `## Notes` -> `### Test Results` | Build + automated tests hijau | Jalankan perintah verifikasi dari `.agents/rules/GEMINI.md` (`go test ./...`, `golangci-lint`, `pnpm ... test/build/lint`). Catat verifikasi browser manual sebagai `outstanding (user)`. |
| **5 Deploy** | `review.md` | Code owner melakukan merge | Dokumentasikan `## Review Passes`, `## Findings` (Important vs nit), dan checklist `## Definition of Done`. |
| **6 Maintain** | `incidents.md` | On-call | Catat `## Signals` dan `## Incident Log` beserta regression test yang ditambahkan. |

## Aturan Gate

- Agen **dilarang** menulis status `accepted` sendiri. Tanyakan kepada user dan tunggu konfirmasi.
- Jangan melakukan `git commit` kecuali user meminta secara eksplisit.

## Saat Satu Tahap Selesai

1. Perbarui status fitur di `.agents/development-progress.md`.
2. Catat pelajaran penting dari review/insiden ke `.agents/lessons-learned.md`; jika perlu menjadi aturan permanen, usulkan pembaruan pada `.agents/rules/GEMINI.md`.
3. Setelah seluruh tahap selesai dan fitur di-merge, pindahkan folder `.agents/specs/<feature>/` ke `.agents/specs/done/<feature>/`.
4. Jika ada perubahan kode dan `graphify` tersedia di proyek, jalankan `graphify update .`.
