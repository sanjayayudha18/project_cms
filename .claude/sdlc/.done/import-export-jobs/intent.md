# Intent: `import_jobs` / `export_jobs` + helper idempotensi file

Author: TODO (product owner / tech lead). Status: draft — menunggu penerimaan.
Stage: 1 Plan. Trigger ke stage berikut: product owner menerima file ini.
Roadmap: Phase 0.2 (`.kiro/steering/development-plan.md`). Aturan: CLAUDE.md Sec 5 (Idempotency).

## Problem
CLAUDE.md Sec 5 mewajibkan semua ingest file (DSR/invoice/escrow) idempotent per
file hash lewat `import_jobs`, tapi tabel `import_jobs`/`export_jobs` belum ada.
Tiap fitur sekarang membuat mekanismenya sendiri: `dsr_uploads.checksum`,
`dmaa_files`, `itm_*_files`, `master_data_import_batches` (SHA-256). Modul
berikutnya (forecast input uploads, invoice, escrow batch) akan menambah salinan
lagi bila tidak ada fondasi bersama.

## Proposed outcome
- Satu tempat untuk mencatat setiap import/export: status, `file_hash` unik,
  `processing_date`, jumlah baris OK/gagal, error, siapa, kapan.
- Satu helper bersama yang menolak / melewati file yang sama (hash sama) secara konsisten.
- Modul baru (2.1 forecast, 3 invoice, 4 escrow) cukup memakai helper ini, tidak membuat sendiri.

## Affected users and systems
- Users: tidak langsung (fondasi). Tidak langsung: Operator/Admin yang memonitor import/export.
- Systems: `backend/` (modul/paket baru bersama), migrasi baru mulai `019_`,
  mekanisme yang sudah ada: `dsr_uploads`, `dmaa_files`, `itm_cashpos_files`,
  `itm_replenish_files`, `master_data_import_batches`, pipeline Python `backend_python/`.

## Constraints
- Tabel baru → migrasi (Sec 4 #7: STOP & flag). Nama `import_jobs`/`export_jobs` sudah ada di peta tabel Sec 3; kolom belum.
- Tidak boleh merusak alur yang sudah jalan (DSR upload, DMAA/ITM ETL, CSV master data).
- Tidak ada dependency baru.
- Tulis ke primary; daftar/monitoring baca dari replica.
- Setiap perubahan status tercatat di `audit_logs` bila merupakan aksi user.

## Resolved decisions (tanya-jawab dengan user, 2026-09-28)
1. **Mekanisme lama dibiarkan.** `dsr_uploads.checksum`, `dmaa_files`, `itm_*_files`, `master_data_import_batches` tetap jalan apa adanya; hanya modul baru (forecast input, invoice, escrow) yang memakai `import_jobs`. Tidak ada data migration.
2. **Go + Python.** Helper idempotensi dibuat di Go dan di Python (`backend_python/`), keduanya menulis tabel `import_jobs` yang sama — hanya untuk ingest **baru**; ETL Python yang ada tidak diubah (konsisten dengan #1). Kontrak (kolom, status, aturan hash) harus satu, dua implementasi dijaga sinkron.
3. **Revisi = versi baru.** File dengan hash berbeda untuk sumber + tanggal yang sama menjadi job baru; job sebelumnya ditandai `superseded`, tidak dihapus. Hash sama persis → skip (no-op). Modul pemakai membaca versi terakhir yang berhasil.
4. **`import_jobs` saja dulu.** `export_jobs` ditunda sampai ada ekspor async/besar (Phase 0.5); ekspor CSV yang ada tetap streaming langsung.
5. **Monitoring digabung ke EOD monitoring** (`features/eod-monitoring`), bukan layar baru. Dibaca dari replica, dibatasi role admin/app-support (Sec 14).
6. **Retensi: simpan permanen dulu.** Tidak ada purge; kebijakan retensi diputuskan bersama compliance nanti.

## Amendment (2026-09-28, saat menyusun spec)
Ditemukan registry file generik `retry_file_tracking` (+ `late_detections`, `scan_runs`,
`retry_audit_logs`) di `backend_python/lib/services/`, dipakai kedua service retry dan halaman EOD
monitoring, tapi skemanya tidak pernah dimigrasi (hanya di arsip `012_retry_scheduler.sql`). User memutuskan:
- **#1 dipersempit:** ETL lama + tabel `*_files`/`dsr_uploads`/`master_data_import_batches` tetap tidak
  diubah, **tetapi** retry scheduler Python pindah ke `import_jobs` — `retry_file_tracking` digantikan,
  tidak dibuat.
- **#5 diperjelas:** monitoring memakai API Python yang sudah ada (`/status`, `/summary`, `/late`, …) yang
  kini membaca `import_jobs`; tidak ada endpoint Go baru. Bergantung pada perbaikan routing `/api/eod/*`
  (task terpisah).

## Out of scope
- Tabel `export_jobs` (keputusan #4).
- Migrasi/penggantian mekanisme idempotensi lama (keputusan #1).
- Worker async / antrian (diputuskan per fitur saat 2.1, sesuai plan).
- Ekspor XLSX/PDF (Phase 0.5).
- Penyimpanan file (`internal/document`, Phase 0.4).
