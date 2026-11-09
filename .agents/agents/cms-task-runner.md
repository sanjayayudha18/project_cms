# CMS Task Runner (Subagent Instruction)

Dokumen ini menjadi panduan bagi subagent yang menjalankan **satu task** (atau satu grup task beserta sub-task-nya) dari file `tasks.md` pada spec CMS2 (misalnya `.agents/specs/<feature>/tasks.md`).

## Input
- `spec`: path folder spec, misalnya `.agents/specs/<feature>/`.
- `task`: ID task, misalnya `1.1`, `4.2`, atau induk seperti `3` (= seluruh sub-task yang belum dicentang secara berurutan).
Jika salah satu tidak tersedia, tanyakan terlebih dahulu dan jangan mengubah kode.

## Langkah Eksekusi
1. **Baca Konteks**: Patuhi seluruh aturan di `.agents/rules/GEMINI.md`. Baca `tasks.md` pada folder spec (Overview, Model Split, blok task terkait, Notes, Task Dependency Graph), lalu baca bagian `requirements.md` dan `design.md` yang dirujuk oleh `_Requirements:_` pada task tersebut.
2. **Aktifkan Skills**: Jika prompt mencantumkan daftar skill yang disetujui user melalui `cms-run-wave`, baca dan ikuti `SKILL.md` dari masing-masing skill di `.agents/skills/<nama-skill>/SKILL.md` sebelum menulis kode.
3. **Cek Dependensi**: Dari `## Task Dependency Graph`, pastikan seluruh task prasyarat sudah berstatus `[x]` di `tasks.md`. Jika belum selesai, **BERHENTI** dan laporkan dependensi yang masih belum selesai.
4. **Kerjakan Task**: Hanya ubah file dan perilaku yang dijelaskan pada blok task tersebut. Jangan melakukan refactor di luar cakupan atau menambah tabel/kolom/endpoint/env baru yang tidak ada di desain.
   - Area kritis (autentikasi, uang, rekonsiliasi, eksekusi migrasi `migrate up`, penghapusan): menulis file migrasi diperbolehkan, tetapi menjalankan `go run ./cmd/migrate up` memerlukan izin eksplisit user. Jangan pernah menjalankan `psql -f` secara manual.
   - Setelah menjalankan `sqlc generate`: perbaiki `UserLeafe` menjadi `UserLeave` di `backend/internal/db/`.
5. **Verifikasi**: Jalankan unit/integration test, linter, dan build pada modul yang disentuh (`go test ./...`, `golangci-lint`, `pnpm ... test|lint|build`). Perbaiki hingga hijau. Jangan pernah mengklaim pengecekan browser manual selesai.
6. **Catat Progress**: Centang checkbox task yang selesai di `tasks.md` (`- [ ]` menjadi `- [x]`, hanya ubah baris milik task Anda). Tambahkan catatan yang diminta di bawah `### Catatan eksekusi`.
7. **Laporkan**: Kirim ringkasan berisi ID task, skill yang digunakan, daftar file yang diubah, hasil pengujian (pass/fail/skipped beserta alasan), serta hal yang masih menunggu tindakan user.

## Pemetaan Skill (`.agents/skills/`)

| Cakupan Task | Skill |
| :--- | :--- |
| Entitas Master Data, Applier, Maker-Checker | `cms2-master-data` |
| Perbaikan Bug / Investigasi Error | `cms2-bugfix` |
| Siklus Artefak SDLC | `cms2-sdlc` |

## Larangan
- Jangan melakukan `git commit`, `push`, `stash`, pindah branch, atau `reset`.
- Jangan menyentuh file milik task lain yang berjalan paralel pada wave yang sama.
- Jangan mengeksekusi task yang ditandai sebagai pengecekan manual oleh user.
