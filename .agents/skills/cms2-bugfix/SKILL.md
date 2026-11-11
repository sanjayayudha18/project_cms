---
name: cms2-bugfix
description: >-
  Fix a CMS2 bug end to end — route check (bugfix vs full SDLC spec), diagnose,
  root cause across all callers, failing test first, minimal fix, verify, and
  log it in .agents/bugfixes.md. Use when the user reports something broken, an
  error, a failing test/build, "bug", "debug", "error", "rusak", "gagal",
  "tidak jalan", or asks to fix a QC/lint finding.
---

# CMS2 Bug Fix Workflow (Khusus Gemini / `.agents/`)

Sumber kebenaran: `.agents/rules/GEMINI.md` dan template di bagian atas `.agents/bugfixes.md`.

## Langkah 0 — Konfirmasi Jalur Terlebih Dahulu

Selalu tanyakan kepada user sebelum mengubah kode:
*"Perlu dibuat spec baru (`.agents/specs/<feature>/`) atau cukup dokumentasi bug fix (`.agents/bugfixes.md`)?"* dan tunggu jawaban user.

- Sarankan **spec penuh** (aktifkan skill `cms2-sdlc`) apabila perbaikan menyentuh autentikasi, perhitungan uang/jurnal, rekonsiliasi, migrasi/skema database, penghapusan data, mengubah kontrak API, atau mencakup banyak file lintas modul. Jika user tetap memilih jalur bugfix untuk area sensitif tersebut, catat secara eksplisit di entri log (`"Area sensitif di-flag; user memilih jalur dokumentasi bugfix"`).
- Jika bug terisolasi/kecil, sarankan jalur bugfix dan lanjutkan langkah di bawah ini.

## Langkah Pengerjaan

| # | Tindakan | Catatan |
| :--- | :--- | :--- |
| **1** | **Reproduksi & Diagnosis** | Dapatkan gejala pasti (layar, endpoint, input, teks error). Susun hipotesis dan persempit ke satu akar penyebab sebelum mengedit file. |
| **2** | **Cek Seluruh Caller** | Temukan semua pemanggil fungsi yang bermasalah (gunakan `graphify query/path/explain` jika tersedia). Perbaiki di titik sumbernya, bukan hanya di satu jalur gejala. |
| **3** | **Failing Test Terlebih Dahulu (TDD)** | Buat test yang gagal karena bug tersebut (red), baru terapkan perbaikan hingga lulus (green). Gunakan table-driven test untuk Go; gunakan `//go:build integration` jika bug berada di query SQL/transaksi/applier. |
| **4** | **Perbaikan Minimal** | Buat diff sekecil mungkin, fokus pada satu masalah, jangan melakukan refactor sampingan. Ikuti pola kode di sekitarnya. Jika menyentuh master-data, ikuti skill `cms2-master-data`. |
| **5** | **Verifikasi** | Jalankan perintah pengujian & linter dari `.agents/rules/GEMINI.md` untuk modul yang disentuh (`go test ./...`, `golangci-lint`, `pnpm ... test/build/lint`). Pastikan semuanya hijau. |
| **6** | **Review Diff** | Periksa kembali keamanan input, otorisasi RBAC, serta tidak adanya regresi. |
| **7** | **Catat di `.agents/bugfixes.md`** | Tambahkan entri baru di urutan paling atas (di bawah garis `---`). |

## Format Log — `.agents/bugfixes.md`

Sisipkan di bagian paling atas di bawah `---` (terbaru di atas) dalam bahasa Indonesia:

```markdown
## YYYY-MM-DD — <judul singkat>
- **Symptom**: apa yang dilihat/dialami oleh user
- **Root cause**: mengapa bug terjadi
- **Fix**: perubahan yang dilakukan (daftar file)
- **Tests**: automated test yang ditambahkan/dijalankan; manual browser verification: outstanding
- **Commit**: <sha>
```

- **Tests**: sebutkan nama test baru dan perintah verifikasi yang dijalankan. Verifikasi browser manual wajib ditulis **`outstanding`** (atau `tidak relevan` untuk backend-only) — jangan pernah diklaim selesai oleh agen.
- **Commit**: biarkan `<sha>` kosong/placeholder sampai user meminta commit.

## Setelah Selesai

- Jika kode berubah dan `graphify` tersedia -> jalankan `graphify update .`
- Jika ada pelajaran berharga -> catat di `.agents/lessons-learned.md`.
- Jika perubahan memengaruhi perilaku fitur di `.agents/specs/<feature>/` -> beri tahu user apakah spec perlu diperbarui.

## Larangan

- Jangan hanya menambal di hilir (symptom) jika pemanggil lain memiliki bug yang sama di hulu.
- Jangan mengubah ekspektasi test yang sudah ada hanya agar test lulus, kecuali test tersebut memang salah (jelaskan alasannya kepada user).
- Jangan melakukan `git commit` tanpa instruksi eksplisit dari user.
