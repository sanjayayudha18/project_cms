# Requirements Document

> **Status:** requirements disetujui 2026-11-02. PO mengikuti saran Kiro untuk D1-D12. Implementasi selesai 2026-11-03 (tasks 1–15); cek browser manual oleh user masih outstanding (task 16).

## Introduction

### Masalah

PO meminta revamp menu impor CSV di `/settings/admin/master-data-io` ("Ekspor & Impor CSV"), dengan syarat: data yang diimpor harus disetujui (maker-checker) sebelum mengubah database.

### Kondisi kode saat ini (dicek 2026-11-02, `dev1` @ `f98bc89`)

**Syarat persetujuan sudah terpenuhi hari ini.** Tidak ada jalur impor yang menulis tabel entitas master data secara langsung:

- **Periksa file** (`POST /api/v1/admin/master-data/import/{entity}/dry-run`, `MasterDataImporter.DryRun` di `backend/internal/service/masterdata_import.go`) hanya memvalidasi dan tidak menulis apa pun. Responsnya hanya angka ringkasan (`total_rows`, `valid_rows`, `creates`, `updates`, `unchanged`) plus maks. 500 kesalahan.
- **Ajukan untuk persetujuan** (`POST .../{entity}/confirm`, `masterdata_import_confirm.go`) menulis staging saja: satu baris `master_data_import_batches`, N baris `master_data_change_requests` berstatus `pending` (`batch_id` terisi), **satu** `approval_requests` (`document_type='master_data'`, `document_id` = baris kepala batch), dan audit `submit_batch`. Idempoten per `(entity, SHA-256 file)` selama batch belum `rejected`/`stale`.
- **Setuju** (`MasterDataApprovalService.applyMasterDataChange`, `masterdata_approval.go`) menerapkan semua baris batch dalam **satu transaksi**. Satu baris gagal (stale, duplikat, overlap) → transaksi rollback, semua baris batch ditandai `stale`, tidak ada yang diterapkan. Gagal commit → baris kepala tetap `approved` dengan `error` terisi; `POST /approvals/{id}/retry-apply` ada tapi tanpa tombol UI.
- **Tolak** → semua baris batch `rejected`, tabel entitas tidak berubah.
- Kebijakan `approval_policies` untuk `master_data`: satu level (seed di `003_master_data.sql`). Maker ≠ checker ditegakkan orchestrator.
- Route FE `requireRoles(["ADMIN","ADMIN_PARAM"])`; backend `RequireRoles("ADMIN","ADMIN_PARAM")` + cek ulang di service. Rate limit 20 periksa+ajukan per user per jam. Batas 5 MiB dan 2.000 baris. Entitas yang bisa diimpor: Vendor, Cabang vendor, Vault vendor, PIC vendor, ATM, Kelolaan ATM.

**Kekurangan yang ditemukan** (dasar revamp):

1. Maker hanya melihat angka ringkasan; tidak ada daftar baris yang akan berubah, tidak ada sebelum/sesudah. Baris aktifkan/nonaktifkan tergabung di hitungan "Ubah".
2. Setelah mengajukan, maker hanya mendapat toast. Tidak ada riwayat impor dan status batch (menunggu, diterapkan, ditolak, gagal diterapkan).
3. Checker (`MasterDataReviewDialog` di `/approvals`) hanya melihat 200 baris pertama dan hanya badge operasi, tanpa diff per field. Checker menyetujui perubahan yang tidak bisa ia lihat isinya.
4. Tidak ada notifikasi ke approver saat batch diajukan, dan tidak ada notifikasi ke maker saat batch diputuskan.
5. Batch yang gagal diterapkan (rollback, `stale`) tidak terlihat di mana pun oleh maker, termasuk penyebabnya. Retry-apply tidak punya tombol.
6. `master_data_import_batches` tidak menyimpan nama file, jadi riwayat tidak bisa menunjukkan file mana yang diajukan.
7. Judul halaman masih "Ekspor & Impor CSV" padahal ekspor sudah pindah ke menu Export Data (`export-menu-revamp`).

### Ruang lingkup

- Halaman impor di CompanyPortal (`features/master-data-io/`): alur bertahap, pratinjau per baris, konfirmasi pengajuan, riwayat impor, detail batch, tombol coba terapkan lagi.
- Backend `backend/`: perluasan respons dry-run (daftar perubahan), endpoint riwayat dan detail batch, simpan nama file, paging pada detail approval master data, notifikasi impor.
- Tinjauan checker untuk batch impor di `MasterDataReviewDialog`.
- Satu migrasi aditif `036` (kolom nama file, D5).

### Tidak termasuk

- Mengganti model persetujuan (tetap satu batch = satu approval, semua atau tidak sama sekali, D1).
- Perubahan aturan validasi, format kolom CSV, batas 5 MiB / 2.000 baris, rate limit, dan entitas yang bisa diimpor.
- Penarikan (withdraw) pengajuan oleh maker (D9).
- Persetujuan sebagian baris (checker menyetujui sebagian, menolak sebagian).
- Impor XLSX, unduh laporan kesalahan sebagai file, impor master data lain (paket, harga paket, pemetaan DSR).
- Perubahan ekspor (`backend-exq`, `/exports`), CRUD satu-record, dan approval dokumen lain.
- Notifikasi untuk perubahan master data satu-record (hanya batch impor, D7).

### Area sensitif (project-context Sec 3 rule 7)

- **Maker-checker:** spec ini menyentuh jalur approve/reject master data (hook notifikasi, paging detail) dan retry-apply. Invarian "tidak ada perubahan entitas sebelum disetujui" dijaga oleh Requirement 1 dan wajib punya test.
- **Migrasi data:** `036` aditif (kolom nullable), tanpa ubah data lama.
- Tidak menyentuh uang, auth, rekonsiliasi.

### Dependency & Scope Note

- `development-plan.md` scope guard rule 2 melarang fitur master data baru (hanya bugfix). Spec ini butuh persetujuan PO dan dicatat sebagai item baru **X.10** (D11) sebelum implementasi.
- Mengikuti pola yang ada: `MasterDataImporter`, `MasterDataImportBatchRepository`, `MasterDataApprovalService`, `internal/notification` (`notification.Service.Send`), `approval_steps.assigned_approver_id` untuk penerima notifikasi.
- Nomor migrasi berikut yang bebas: `036` (file terakhir `035_export_jobs_reports.sql`).

## Glossary

- **ImportPage**: halaman `/settings/admin/master-data-io` di CompanyPortal (`MasterDataIOPage`), setelah revamp berjudul "Impor CSV Data Master".
- **Maker**: pengguna ADMIN atau ADMIN_PARAM yang memeriksa dan mengajukan file impor.
- **Checker**: approver yang ditugaskan pada step `approval_steps` yang sedang pending untuk batch tersebut.
- **Entity_Table**: tabel master data yang diubah oleh applier: vendor, cabang vendor, vault vendor, PIC vendor, ATM, kelolaan ATM.
- **Import_Batch**: satu baris `master_data_import_batches` beserta semua `master_data_change_requests` dengan `batch_id`-nya dan satu `approval_requests` yang dimilikinya.
- **Change_Row**: satu baris `master_data_change_requests` di dalam Import_Batch, berisi `op` (`create`, `update`, `enable`, `disable`), `payload`, dan `before`.
- **MasterDataImporter**: service dry-run dan confirm impor (`masterdata_import.go`, `masterdata_import_confirm.go`).
- **ImportAPI**: endpoint di bawah `/api/v1/admin/master-data/import`.
- **ImportBatchAPI**: endpoint baru riwayat dan detail batch di bawah `/api/v1/admin/master-data/import/batches`.
- **ApprovalAPI**: endpoint di bawah `/api/v1/approvals`.
- **MasterDataApprovalService**: pembungkus orchestrator yang menerapkan/menolak perubahan master data.
- **ReviewDialog**: `MasterDataReviewDialog` di halaman `/approvals`.
- **ImportNotifier**: pengirim notifikasi impor lewat `internal/notification`.
- **Change_Preview**: daftar perubahan per baris di respons dry-run: nomor baris, operasi, kunci data, dan field yang berubah (sebelum, sesudah).
- **Batch_Status**: status tampilan Import_Batch yang diturunkan dari data yang ada (D4): Menunggu persetujuan, Ditolak, Diterapkan, Gagal diterapkan, Disetujui belum diterapkan.

## Requirements

### Requirement 1: Data impor tidak mengubah database sebelum disetujui

**User Story:** Sebagai PO, saya ingin setiap data yang diimpor baru masuk ke database master setelah disetujui checker, supaya tidak ada perubahan master data tanpa kontrol dua orang.

#### Acceptance Criteria

1. WHEN Maker memeriksa file, THE MasterDataImporter SHALL tidak menulis tabel apa pun.
2. WHEN Maker mengajukan file valid, THE MasterDataImporter SHALL hanya menulis Import_Batch (batch, Change_Row berstatus `pending`, satu `approval_requests` beserta step-nya) dan `audit_logs`, dan SHALL tidak mengubah Entity_Table.
3. WHILE `approval_requests` milik Import_Batch belum disetujui di step terakhir, THE Entity_Table SHALL tidak berubah oleh Import_Batch tersebut.
4. WHEN Checker menolak Import_Batch, THE MasterDataApprovalService SHALL menandai semua Change_Row `rejected` dan SHALL tidak mengubah Entity_Table.
5. WHEN Checker menyetujui Import_Batch di step terakhir, THE MasterDataApprovalService SHALL menerapkan semua Change_Row dalam satu transaksi; IF satu Change_Row gagal diterapkan, THEN THE MasterDataApprovalService SHALL tidak menerapkan Change_Row mana pun dari batch itu.
6. IF Maker mencoba menyetujui atau menolak Import_Batch yang ia ajukan sendiri, THEN THE ApprovalAPI SHALL menolak dengan 403 dan Entity_Table tidak berubah.
7. THE MasterDataImporter SHALL membuat tepat satu `approval_requests` per Import_Batch (D1).

### Requirement 2: Halaman impor bertahap

**User Story:** Sebagai Maker, saya ingin halaman impor menuntun saya langkah demi langkah, supaya saya tahu posisi saya dan apa yang terjadi setelah mengajukan.

#### Acceptance Criteria

1. THE ImportPage SHALL berjudul "Impor CSV Data Master" dengan eyebrow "Data master" dan deskripsi yang menyatakan perubahan baru diterapkan setelah disetujui; kartu di halaman Pengaturan SHALL memakai judul yang sama (D10).
2. THE ImportPage SHALL menampilkan tiga langkah berurutan: "1. Pilih data", "2. Unggah dan periksa", "3. Tinjau dan ajukan", dengan penanda langkah aktif berupa nomor dan teks, tidak hanya warna.
3. WHEN Maker mengganti entitas atau file, THE ImportPage SHALL menghapus hasil pemeriksaan sebelumnya dan kembali ke langkah 2.
4. THE ImportPage SHALL tetap menyediakan tombol "Unduh templat", tautan "Ekspor data ini", dan catatan aturan impor yang berlaku sekarang (pemisah, 5 MiB, 2.000 baris, kolom `id`, peringatan Excel, aturan data vs status aktif).
5. THE ImportPage SHALL memiliki dua tab: "Impor baru" dan "Riwayat impor".
6. THE ImportPage SHALL tetap hanya bisa dibuka ADMIN dan ADMIN_PARAM; peran lain melihat halaman Forbidden seperti sekarang.

### Requirement 3: Pratinjau perubahan per baris

**User Story:** Sebagai Maker, saya ingin melihat baris mana yang akan berubah dan nilai sebelum/sesudahnya, supaya saya yakin file saya benar sebelum diajukan.

#### Acceptance Criteria

1. WHEN file valid diperiksa, THE ImportAPI SHALL mengembalikan Change_Preview untuk setiap baris yang bukan "tanpa perubahan": nomor baris di file, operasi (`create`, `update`, `enable`, `disable`), kunci data yang dikenali pengguna (mis. kode vendor, terminal ID), dan daftar field yang berubah dengan nilai sebelum dan sesudah (D2).
2. THE ImportAPI SHALL menghitung ringkasan terpisah untuk Baru, Ubah, Aktifkan, Nonaktifkan, Tanpa perubahan, dan Kesalahan; `updates` lama tetap dikirim untuk kompatibilitas.
3. THE ImportPage SHALL menampilkan Change_Preview dalam tabel berpaging 50 baris dengan filter operasi, badge operasi berupa ikon + label, dan nilai angka rata kanan dengan `tabular-nums`.
4. IF pemeriksaan menemukan kesalahan, THEN THE ImportPage SHALL menampilkan tabel kesalahan (Baris, Kolom, Kesalahan) dengan ikon + teks, menyembunyikan Change_Preview, dan menonaktifkan tombol ajukan, seperti sekarang.
5. THE ImportAPI SHALL tetap tidak menulis apa pun saat pemeriksaan, dan ukuran Change_Preview SHALL dibatasi oleh batas 2.000 baris yang ada.

### Requirement 4: Pengajuan dengan konfirmasi

**User Story:** Sebagai Maker, saya ingin mengonfirmasi ringkasan sebelum mengajukan dan melihat hasil pengajuan dengan jelas.

#### Acceptance Criteria

1. THE ImportPage SHALL mengaktifkan tombol "Ajukan untuk persetujuan" hanya bila hasil pemeriksaan tanpa kesalahan dan ada minimal satu perubahan.
2. WHEN Maker menekan tombol ajukan, THE ImportPage SHALL membuka dialog konfirmasi berisi entitas, nama file, jumlah per operasi, dan kalimat "Data belum berubah sampai pengajuan disetujui." sebelum mengirim.
3. WHEN pengajuan berhasil, THE ImportPage SHALL menampilkan panel hasil berisi nomor batch, jumlah baris, Batch_Status "Menunggu persetujuan", dan tautan ke detail batch di Riwayat impor.
4. WHEN file yang sama sudah diajukan dan batch-nya masih terbuka, THE ImportPage SHALL menampilkan batch yang ada dengan pesan bahwa file sudah diajukan sebelumnya (idempotensi tetap).
5. WHEN pengajuan berhasil, THE MasterDataImporter SHALL menyimpan nama file asli di Import_Batch setelah dibersihkan (hanya nama dasar, karakter kontrol dibuang, maks. 255 karakter) dan mencatatnya di audit `submit_batch` (D5).
6. IF file berubah validitasnya di antara periksa dan ajukan, THEN THE ImportAPI SHALL menolak dengan 422 beserta daftar kesalahan, dan THE ImportPage SHALL menampilkannya seperti hasil pemeriksaan.

### Requirement 5: Riwayat impor

**User Story:** Sebagai Maker, saya ingin melihat semua impor yang sudah diajukan beserta statusnya, supaya saya tahu mana yang sudah diterapkan, ditolak, atau gagal.

#### Acceptance Criteria

1. THE ImportBatchAPI SHALL menyediakan daftar Import_Batch untuk ADMIN dan ADMIN_PARAM, dicek di route dan service; peran lain mendapat 403 (D3).
2. THE ImportBatchAPI SHALL mengembalikan per batch: nomor batch, entitas, nama file (kosong untuk batch lama), jumlah baris, nama pengaju, waktu diajukan, Batch_Status, dan waktu keputusan bila ada; urut terbaru dulu, paging 20.
3. THE ImportBatchAPI SHALL mendukung filter entitas, Batch_Status, dan "hanya milik saya".
4. THE ImportBatchAPI SHALL menurunkan Batch_Status dari data yang ada tanpa kolom status baru (D4): `pending` → Menunggu persetujuan; `rejected` → Ditolak; `applied` → Diterapkan; `stale` → Gagal diterapkan; `approved` → Disetujui belum diterapkan.
5. THE ImportBatchAPI SHALL membaca dari pool primary, karena riwayat dibuka langsung setelah pengajuan (D6).
6. THE ImportPage SHALL menampilkan Batch_Status sebagai badge ikon + label (tidak memakai merah brand) dan waktu dalam WIB.
7. WHEN belum ada batch yang cocok dengan filter, THE ImportPage SHALL menampilkan pesan kosong yang menjelaskan cara mengajukan impor.

### Requirement 6: Detail batch dan coba terapkan lagi

**User Story:** Sebagai Maker, saya ingin membuka satu batch untuk melihat isi dan penyebab kegagalannya, dan mencoba menerapkan ulang bila penyimpanan gagal setelah disetujui.

#### Acceptance Criteria

1. THE ImportBatchAPI SHALL menyediakan detail satu Import_Batch: data ringkas seperti di daftar, jumlah per operasi, dan Change_Row berpaging 50 dengan operasi, kunci data, field yang berubah (sebelum, sesudah), status, dan pesan kesalahan per baris.
2. WHEN Batch_Status adalah Gagal diterapkan, THE ImportPage SHALL menampilkan penyebab dari pesan kesalahan Change_Row, dengan ikon + teks, dan menyatakan bahwa file yang sama boleh diajukan ulang setelah diperbaiki.
3. WHILE Batch_Status adalah Disetujui belum diterapkan, THE ImportPage SHALL menampilkan tombol "Coba terapkan lagi" yang memanggil `POST /api/v1/approvals/{id}/retry-apply` yang sudah ada (D8).
4. THE ImportPage SHALL tidak menampilkan tombol "Coba terapkan lagi" untuk Batch_Status lain.
5. IF nomor batch tidak ada, THEN THE ImportBatchAPI SHALL menjawab 404.

### Requirement 7: Tinjauan checker yang lengkap

**User Story:** Sebagai Checker, saya ingin melihat seluruh isi batch impor termasuk sebelum/sesudah per field, supaya keputusan saya berdasarkan data yang sebenarnya.

#### Acceptance Criteria

1. WHEN Checker membuka batch impor, THE ReviewDialog SHALL menampilkan semua Change_Row berpaging 50 (menggantikan batas 200 baris), masing-masing dengan operasi, kunci data, dan field yang berubah (sebelum, sesudah).
2. THE ApprovalAPI SHALL menerima parameter paging pada `GET /approvals/{id}/master-data` dan mengembalikan total baris; tanpa parameter, perilakunya kompatibel dengan klien lama.
3. THE ReviewDialog SHALL menampilkan entitas, nama file, nama pengaju, jumlah per operasi (Baru, Ubah, Aktifkan, Nonaktifkan), dan catatan bahwa semua baris diterapkan sekaligus.
4. THE ApprovalAPI SHALL tetap hanya memberi detail kepada Maker pengaju atau approver dengan step pending, seperti sekarang.
5. THE ReviewDialog SHALL tetap menampilkan diff field untuk perubahan satu-record seperti sekarang.

### Requirement 8: Notifikasi impor

**User Story:** Sebagai Checker dan Maker, saya ingin diberi tahu saat ada impor yang perlu ditinjau dan saat impor saya diputuskan.

#### Acceptance Criteria

1. WHEN Import_Batch diajukan atau naik ke step berikutnya, THE ImportNotifier SHALL mengirim notifikasi in-app dan email ke approver step yang pending, dengan tautan ke `/approvals` (D7).
2. WHEN Import_Batch selesai diterapkan, ditolak, atau gagal diterapkan, THE ImportNotifier SHALL mengirim notifikasi in-app ke Maker dengan tautan ke detail batch.
3. IF penulisan notifikasi gagal, THEN THE MasterDataImporter dan MasterDataApprovalService SHALL tetap mempertahankan hasil pengajuan atau keputusan, dan mencatat kegagalan di log (D12).
4. WHEN file yang sama diajukan ulang dan mengembalikan batch yang ada, THE ImportNotifier SHALL tidak mengirim notifikasi ulang.

### Requirement 9: Audit

**User Story:** Sebagai auditor, saya ingin jejak lengkap siapa mengajukan dan memutuskan impor.

#### Acceptance Criteria

1. THE MasterDataImporter SHALL menulis audit `submit_batch` dengan entitas, hash file, nama file, dan jumlah baris.
2. THE MasterDataApprovalService SHALL tetap menulis audit approve, apply per Change_Row, reject, dan retry seperti sekarang.
3. THE ImportBatchAPI SHALL tidak menulis audit untuk pembacaan riwayat dan detail.

### Requirement 10: Non-regresi dan gate kualitas

**User Story:** Sebagai pengembang, saya ingin revamp ini tidak mengubah aturan impor, CRUD satu-record, dan ekspor.

#### Acceptance Criteria

1. THE implementasi SHALL tidak mengubah aturan validasi, header CSV, batas 5 MiB / 2.000 baris, rate limit 20 per jam, idempotensi per hash file, dan daftar entitas yang bisa diimpor.
2. THE alur maker-checker satu-record master data dan halaman ekspor SHALL tidak berubah.
3. THE migrasi `036` SHALL aditif (kolom nullable), forward-only, dan diterapkan lewat `go run ./cmd/migrate up`.
4. THE implementasi SHALL lolos `go test ./...` dan `golangci-lint run --build-tags integration ./...` di `backend/`, termasuk integration test dengan Postgres nyata untuk Requirement 1.
5. THE implementasi SHALL lolos `pnpm run lint`, `pnpm run test:coverage` (CompanyPortal 84/80/80/85), `pnpm run build`, dan `check-bundle.mjs`.

## Open Questions

| # | Pertanyaan | Saran Kiro | Status |
| --- | --- | --- | --- |
| D1 | Model persetujuan impor | **Tetap satu batch = satu approval, semua atau tidak sama sekali** (sudah berjalan). Approval per baris berarti sampai 2.000 approval untuk satu file dan bisa menerapkan file setengah jadi. | ✅ mengikuti saran (PO 2026-11-02) |
| D2 | Pratinjau per baris di layar Maker | **Ya**, dry-run mengembalikan Change_Preview (hanya field yang berubah). Maks. 2.000 baris, cukup untuk dipaging di browser. | ✅ mengikuti saran (PO 2026-11-02) |
| D3 | Siapa yang melihat Riwayat impor | **ADMIN dan ADMIN_PARAM melihat semua batch**, dengan filter "hanya milik saya". Peran maker yang sama, jadi riwayat bersama membantu menghindari impor ganda. | ✅ mengikuti saran (PO 2026-11-02) |
| D4 | Status batch | **Diturunkan dari status baris kepala** (`pending`/`approved`/`rejected`/`applied`/`stale`), tanpa kolom status baru. Status ini sudah dijaga oleh alur approve/reject yang ada. | ✅ mengikuti saran (PO 2026-11-02) |
| D5 | Simpan nama file | **Ya**, migrasi `036`: `master_data_import_batches.file_name text NULL`. Batch lama tetap NULL dan tampil kosong. | ✅ mengikuti saran (PO 2026-11-02) |
| D6 | Pool baca riwayat | **Primary**: riwayat dibuka langsung setelah pengajuan (read-after-write); datanya kecil. | ✅ mengikuti saran (PO 2026-11-02) |
| D7 | Notifikasi | **In-app + email ke approver** saat batch menunggu tinjauan; **in-app ke Maker** saat diterapkan/ditolak/gagal. Hanya untuk batch impor; perubahan satu-record tidak ikut (bisa spec terpisah). | ✅ mengikuti saran (PO 2026-11-02) |
| D8 | Tombol "Coba terapkan lagi" | **Ya**, di detail batch, memakai endpoint retry-apply yang ada (otorisasi ADMIN/ADMIN_PARAM tetap). | ✅ mengikuti saran (PO 2026-11-02) |
| D9 | Penarikan pengajuan oleh Maker | **Tidak termasuk.** Orchestrator belum punya status batal; Checker bisa menolak. | ✅ mengikuti saran (PO 2026-11-02) |
| D10 | Judul halaman | **"Impor CSV Data Master"**, rute tetap `/settings/admin/master-data-io` supaya tautan lama tidak putus. | ✅ mengikuti saran (PO 2026-11-02) |
| D11 | Scope guard `development-plan.md` (rule 2: tidak ada fitur master data baru) | **PO menyetujui pengecualian** dan item dicatat sebagai **X.10** di cross-cutting. | ✅ mengikuti saran (PO 2026-11-02) |
| D12 | Kegagalan notifikasi | **Best-effort**: pengajuan/keputusan tetap tersimpan, kegagalan dicatat di log. Inbox `/approvals` tetap sumber utama, jadi checker tetap bisa menemukan batch. | ✅ mengikuti saran (PO 2026-11-02) |
