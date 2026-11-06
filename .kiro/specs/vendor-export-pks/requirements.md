# Requirements Document

> **Status: disetujui PO 2026-11-06.** D1–D8 di `## Open Questions` ✅ diputuskan PO 2026-11-06 (semua mengikuti saran Kiro, D1 = opsi a).

## Introduction

### Masalah

Halaman ekspor data master vendor (`/exports/vendors`, CompanyPortal) hanya menampilkan kolom tabel `vendors`: `id, code, name, legal_name, npwp, contact_email, contact_phone, hq_address, is_active`. Data Direktur Utama dan PKS aktif tidak ada di sana, padahal user mencarinya di halaman itu. Saat ini data itu hanya ada di Pengaturan → Admin → Vendor (kolom tabel, filter Status PKS, tombol "Ekspor CSV" yang membuat report `vendor_pks`).

Hasil yang diinginkan: di pratinjau dan file ekspor vendor, user melihat per vendor:
- **Direktur Utama:** nama, telepon, email.
- **PKS aktif:** nama PKS, tanggal berlaku, tanggal expired, masa berlaku (bulan), sisa hari, status PKS.

### Kondisi kode saat ini (dicek 2026-11-06)

**Halaman dan endpoint**
- `/exports/vendors` didefinisikan di `frontend/CompanyPortal-Vite/src/features/exports/datasets.ts` (dataset `vendors`, filter hanya Status). `toParams` memetakannya ke kind `master_data_global`, entity `vendors`, jadi halaman ini memanggil `POST /api/v1/exports/master-data/vendors` dan `GET /api/v1/exports/preview/master-data/vendors`, **bukan** `POST /exports/vendors/{vendorID}/vendor`.
- Route `/exports/vendors/{vendorID}/vendor` (kind `vendor`, entity `vendor`) mengekspor satu baris profil vendor. Route itu memakai producer yang sama (`vendorsProducer(true)`), jadi perubahan kolom di producer berlaku untuk keduanya.
- Producer: `backend-exq/internal/exportjob/producers.go` `vendorsProducer`. Header `csvexport.Vendors`, sel `csvexport.VendorCells(csvexport.VendorRow(r))` (konversi struct langsung dari baris sqlc), query `ExportVendorsBatch`.
- `ExportVendorsBatch` ada di `backend/queries/master_data_export.sql` dan **dibagi** dengan `backend`. Di `backend`, query ini dipakai `MasterDataExportRepository.Vendors` untuk membaca keadaan sekarang saat impor. Query ini tidak boleh berubah bentuk tanpa memengaruhi impor.
- Pratinjau (`backend-exq/internal/exportjob/preview.go`): kolom = header CSV, sel mentah, keyset `id`, `limit` 1–100, scope dibaca dari primary, baris dari replica dalam satu snapshot, tanpa audit. Producer file dan pratinjau berbagi `fetch` + `cells`, jadi pratinjau tidak bisa berbeda dari file.
- Frontend `PreviewTable.tsx` membangun kolom dari `data.columns` server. `numericColumns` dataset `vendors` = `["id"]`.

**Kontrak CSV dan impor**
- `pkg/csvexport/entities.go`: `Entity{Header, LegacyHeaders, LegacyFill}`. Header vendor 9 kolom, tanpa legacy header.
- `backend/internal/service/masterdata_import_csv.go` `readImportCSV`: baris pertama harus sama persis dengan header atau salah satu `LegacyHeaders`. Legacy header saat ini selalu **lebih sedikit** kolom (kolom yang hilang diisi `LegacyFill`). Belum ada mekanisme untuk kolom **tambahan** yang diabaikan; header dengan kolom ekstra ditolak dengan pesan "kolom header tidak sesuai; diharapkan persis: …".
- Template impor `GET /api/v1/exports/master-data/{entity}/template` menulis header dari `csvexport.Lookup`.
- Test round-trip (export → dry-run impor = 0 create, 0 update): `masterdata_import_integration_test.go` `TestMasterDataImport_RoundTripOnRealDatabase`, `masterdata_export_roundtrip_integration_test.go`, `masterdata_export_scoped_roundtrip_integration_test.go`; golden `masterdata_export_golden_test.go`; property `backend-exq/internal/exportjob/producers_property_test.go`.

**Report `vendor_pks` yang sudah ada (spec `vendor-pks`)**
- `pkg/reports/vendor_pks.go`: header Indonesia 11 kolom ("Kode Vendor" … "Status PKS"), `VendorPksCells` (CSVSafe pada teks, tanggal, angka, label status dari `PksStatuses`, sel kosong bila nil), file `vendor-pks-YYYYMMDD.csv`, `VendorPksViewerRoles = ADMIN, ADMIN_PARAM`.
- Query `ExportVendorPksBatch` (`backend/queries/vendor_pks_report.sql`, dibagi dengan `backend-exq`): `LEFT JOIN vendor_pks ... replaced_at IS NULL` + `LEFT JOIN vendor_pics ... is_director AND deleted_at IS NULL`, COALESCE untuk hasil fungsi, keyset `(name, id)`.
- Producer `vendorPksProducer` bukan `PreviewProducer`, jadi report ini tidak punya pratinjau (preview mengembalikan 404 `ErrUnknownEntity`). Audit unduh = `report_exported`.
- `vendor_pks_status()` dan `vendor_pks_validity_months()` ada di migrasi `022`. `VENDOR_PKS_RENEWAL_MONTHS` dibaca di `backend/cmd/api/main.go` dan `backend-exq/cmd/api/main.go` (nilainya harus sama).
- Director_PIC dijamin paling banyak satu per vendor oleh partial unique index `vendor_pics_director_uq`, jadi LEFT JOIN tidak menggandakan baris.

**Role dan audit**
- Kind `vendor`, `cabang`, `harga_paket`, `master_data_global`: role `ADMIN`, `ADMIN_PARAM` (`kinds.go` `masterDataAdmin`), dicek di route (`RequireRoles(KindRoles[kind])`) dan di service (`allowed`). Route frontend `/exports/$dataset` juga `ADMIN`, `ADMIN_PARAM`.
- Audit: `export_requested` saat job dibuat (satu transaksi), `master_data_exported` saat unduh (dengan `row_count`).
- `VendorPksViewerRoles` sama dengan role ekspor master data, jadi pertanyaan role (D6) tidak mengubah siapa yang bisa melihat data hari ini.

**Spec terkait**
- `vendor-pks` Req 2.4: "THE vendor CSV header (`vendors`) SHALL stay unchanged (D32)". Opsi a mengubah keputusan ini (lihat D1). Req 2.5 (PKS tidak masuk lewat impor CSV) tetap berlaku.
- Migrasi terakhir `037_export_jobs_vendor_pks.sql`. Fitur ini tidak butuh migrasi (data, fungsi, dan index sudah ada).

### Ruang lingkup

- Kolom Direktur Utama dan PKS aktif (baca-saja) di pratinjau dan file ekspor vendor dari `backend-exq`, untuk kind `master_data_global`/`vendors` dan `vendor`/`vendor`.
- Penyesuaian parser impor vendor supaya file ekspor lama dan baru tetap bisa diimpor, dengan kolom baca-saja dibuang sebelum validasi.
- Tampilan kolom baru di `/exports/vendors`.

### Tidak termasuk

- Mengubah PKS atau direktur lewat CSV atau lewat halaman ekspor.
- Riwayat PKS (PKS yang sudah digantikan), kecuali PO memilih lain di D3.
- Filter Status PKS di halaman ekspor, kecuali PO memilih lain di D5.
- Perubahan report `vendor_pks`, layar Admin Vendor, tab PKS, atau CSV PIC.
- Tabel atau migrasi baru.
- VendorPortal.

### Area sensitif (project-context Sec 3 rule 7)

- **Parser impor master data.** Perubahan di `readImportCSV` dan kontrak `pkg/csvexport` menyentuh integritas data master (round-trip harus tetap 0 perubahan). Tidak menyentuh uang, auth, approval, rekonsiliasi.
- **Data pribadi direktur vendor** (nama, telepon, email). Data yang sama sudah diekspor ke role yang sama lewat report `vendor_pks`, jadi tidak ada audiens baru.
- **Migrasi:** tidak ada, jadi tidak ada item CLAUDE.md Sec 4 #7.

## Glossary

- **VendorExportPage**: halaman pratinjau `/exports/vendors` di CompanyPortal (`features/exports/`, dataset `vendors`).
- **PreviewAPI**: `GET /api/v1/exports/preview/master-data/vendors` dan `GET /api/v1/exports/preview/vendors/{vendorID}/vendor` di `backend-exq`.
- **ExportAPI**: `POST /api/v1/exports/master-data/vendors` dan `POST /api/v1/exports/vendors/{vendorID}/vendor` di `backend-exq`, beserta worker dan unduh file job-nya.
- **VendorProducer**: `vendorsProducer` di `backend-exq/internal/exportjob/producers.go`, sumber header dan sel untuk PreviewAPI dan file ExportAPI.
- **VendorCsvContract**: entitas `vendors` di `pkg/csvexport` (header, cells, legacy header).
- **Import_Header**: 9 kolom vendor saat ini (`id` … `is_active`). Satu-satunya kolom yang diproses impor.
- **Read_Only_Columns**: kolom direktur dan PKS yang ditambahkan di belakang Import_Header pada file ekspor. Tidak pernah diproses impor.
- **Export_Header**: Import_Header diikuti Read_Only_Columns.
- **MasterDataImporter**: impor CSV master data di `backend` (`masterdata_import*.go`, `readImportCSV`), termasuk dry-run, template, dan halaman impor CompanyPortal.
- **VendorPksReport**: report `vendor_pks` yang sudah ada (`pkg/reports/vendor_pks.go`, `ExportVendorPksBatch`).
- **Active_PKS**: baris `vendor_pks` vendor dengan `replaced_at IS NULL` (paling banyak satu).
- **Director_PIC**: baris `vendor_pics` vendor dengan `is_director = true` dan `deleted_at IS NULL` (paling banyak satu).
- **PksStatusRule**: fungsi SQL `vendor_pks_status()` dengan tanggal hari ini (WIB) dan `VENDOR_PKS_RENEWAL_MONTHS`.
- **As_Of_Date**: tanggal WIB yang dipakai PksStatusRule dan sisa hari: tanggal snapshot job untuk file, tanggal permintaan untuk pratinjau.

## Requirements

### Requirement 1: Kolom Direktur Utama dan PKS aktif di ekspor vendor

**User Story:** Sebagai admin master data, saya ingin melihat Direktur Utama dan PKS aktif setiap vendor di pratinjau dan file ekspor vendor, supaya saya tidak perlu membuka layar Admin Vendor untuk data itu.

#### Acceptance Criteria

1. THE VendorProducer SHALL use the Export_Header for both kinds (`master_data_global`/`vendors` and `vendor`/`vendor`): the Import_Header followed by the Read_Only_Columns in this order: `director_name`, `director_phone`, `director_email`, `pks_name`, `pks_effective_date`, `pks_expiry_date`, `pks_validity_months`, `pks_remaining_days`, `pks_status` (D2, D8).
2. WHEN the PreviewAPI returns a page or the ExportAPI writes a file, THE VendorProducer SHALL fill the Read_Only_Columns of each vendor from its Director_PIC and Active_PKS with the same values VendorPksReport gives for that vendor on the same As_Of_Date and the same `VENDOR_PKS_RENEWAL_MONTHS` (dates `YYYY-MM-DD`, months and days as integers, status as the Indonesian label of `reports.PksStatuses`).
3. THE VendorProducer SHALL emit exactly one row per vendor and SHALL read only the Active_PKS; replaced PKS rows SHALL NOT appear (D3).
4. WHEN a vendor has no Director_PIC, THE VendorProducer SHALL leave `director_name`, `director_phone`, and `director_email` empty.
5. WHEN a vendor of kind FLM has no Active_PKS, THE VendorProducer SHALL leave the PKS cells empty except `pks_status`, which SHALL be the "Belum diisi" label, the same as VendorPksReport.
6. WHEN a vendor has kind `INTERNAL`, THE VendorProducer SHALL leave all PKS cells, including `pks_status`, empty (D4); its director cells follow 1.4.
7. THE VendorProducer SHALL keep the existing row order (`id`), status filter, keyset paging, and row count, so the Read_Only_Columns do not change which vendors are exported.
8. THE ExportAPI SHALL write every Read_Only_Columns cell through the existing CSV writer, so text cells get the existing CSVSafe treatment and phone numbers stay readable.

### Requirement 2: Impor CSV vendor tetap aman

**User Story:** Sebagai admin master data, saya ingin file ekspor vendor lama dan baru tetap bisa saya impor, supaya kolom baru tidak merusak alur impor yang sudah ada.

#### Acceptance Criteria

1. THE MasterDataImporter SHALL accept a vendor CSV whose header is the Import_Header (files exported before this change) and a vendor CSV whose header is the Export_Header (files exported after this change).
2. WHEN a vendor CSV uses the Export_Header, THE MasterDataImporter SHALL drop the Read_Only_Columns cells before validation and diffing; their values SHALL NOT be validated, stored, staged in a change snapshot, or shown as a change in the dry-run.
3. WHEN a file produced by the ExportAPI with the Export_Header is imported without edits, THE MasterDataImporter SHALL report zero creates and zero updates in the dry-run; the same SHALL hold for a file with the Import_Header.
4. WHEN a user edits only Read_Only_Columns cells in an exported file and imports it, THE MasterDataImporter SHALL report zero creates and zero updates for those rows.
5. IF a vendor CSV header matches neither the Import_Header nor the Export_Header, THEN THE MasterDataImporter SHALL reject the file with its existing header-mismatch error, which names the Import_Header.
6. THE vendor import template SHALL keep the Import_Header only.
7. WHILE the vendor entity is selected on the master-data import page, THE MasterDataImporter screen SHALL show a short note that the director and PKS columns of an exported file are ignored during import (D7).
8. THE header contracts, legacy headers, and import behavior of every other master-data entity, and the `ExportVendorsBatch` query used by the MasterDataImporter to read current vendor state, SHALL stay unchanged.

### Requirement 3: Kolom baca-saja

**User Story:** Sebagai pemilik proses PKS, saya ingin data PKS dan direktur hanya bisa diubah lewat alurnya sendiri, supaya maker-checker dan audit PKS tidak terlewati.

#### Acceptance Criteria

1. THE PreviewAPI, ExportAPI, and MasterDataImporter SHALL NOT create, update, or delete `vendor_pks` or `vendor_pics` rows; PKS changes stay limited to the PKS tab (maker-checker) and director changes to the PIC flow.
2. THE PreviewAPI SHALL stay read-only and SHALL write no audit row, as today.

### Requirement 4: Akses, replica, dan audit

**User Story:** Sebagai pemilik keamanan aplikasi, saya ingin kolom baru mengikuti kontrol akses dan audit ekspor vendor yang sudah ada, supaya tidak ada jalur akses baru.

#### Acceptance Criteria

1. THE PreviewAPI and ExportAPI SHALL allow the vendor export with the Read_Only_Columns only to the roles of the existing kinds (`ADMIN`, `ADMIN_PARAM`), checked at the route guard and again in the service (D6).
2. WHEN a role outside that list calls the PreviewAPI or ExportAPI for vendors, THE PreviewAPI and ExportAPI SHALL deny it with the existing 403 response.
3. THE PreviewAPI and ExportAPI SHALL read vendor, Active_PKS, and Director_PIC rows from the replica pool, in the same snapshot as the row count, and SHALL keep scope lookups on the primary, as today.
4. THE ExportAPI SHALL keep the existing audit actions (`export_requested` when the job is created, `master_data_exported` with `row_count` on download) with their current shape; no new audit action SHALL be added.
5. THE VendorExportPage route guard SHALL stay `ADMIN`, `ADMIN_PARAM`.

### Requirement 5: Tampilan di halaman ekspor vendor

**User Story:** Sebagai admin master data, saya ingin kolom direktur dan PKS mudah dibaca di pratinjau, supaya saya bisa memeriksa data sebelum mengekspor.

#### Acceptance Criteria

1. THE VendorExportPage SHALL show the Read_Only_Columns returned by the PreviewAPI after the existing columns, with `pks_validity_months` and `pks_remaining_days` right-aligned in tabular numerals.
2. THE VendorExportPage SHALL render `pks_status` with the same badge (icon + label) as the Admin Vendor screen, and an empty cell as empty.
3. THE VendorExportPage SHALL update the `vendors` dataset description to mention Direktur Utama and PKS aktif.
4. THE VendorExportPage SHALL keep its existing filter (Status only), paging, and export button (D5).

### Requirement 6: Regresi dan test

**User Story:** Sebagai tim pengembang, saya ingin perubahan ini terbukti tidak merusak ekspor dan impor yang ada, supaya bisa dirilis tanpa cek manual yang panjang.

#### Acceptance Criteria

1. THE test suite SHALL show that for any vendor set, the Read_Only_Columns cells equal the matching VendorPksReport cells for the same As_Of_Date and renewal months.
2. THE test suite SHALL extend the vendor round-trip tests to both headers (2.3) and to edited Read_Only_Columns (2.4), and SHALL cover the header-mismatch rejection (2.5) and the unchanged template (2.6).
3. THE existing export, preview, import, and VendorPksReport tests SHALL stay green, the preview/file equality property SHALL hold for the Export_Header, and the existing coverage gates (Go and CompanyPortal) SHALL still pass.
4. THE change SHALL add no table and no migration; `backend-exq/api/openapi.yaml` and `pnpm run gen:api:exq` SHALL stay consistent (the preview response shape does not change).

## Open Questions

Status: ✅ **DIPUTUSKAN PO 2026-11-06, D1–D8 mengikuti saran Kiro.** Kolom "Dampak bila PO memilih lain" disimpan sebagai catatan alasan.

| ID | Pertanyaan | Opsi | Saran Kiro (= keputusan) | Dampak bila PO memilih lain |
| --- | --- | --- | --- | --- |
| D1 | Pendekatan | (a) tambah kolom baca-saja ke CSV vendor, diabaikan saat impor; (b) pilihan ekspor terpisah "PKS vendor" di `/exports/vendors` memakai report `vendor_pks`; (c) hanya di pratinjau, file tetap | **(a).** Satu file vendor berisi semua data yang dicari user, dan pratinjau = file tanpa kerja tambahan karena producer yang sama. Biaya: mekanisme baru di parser impor (kolom tambahan dibuang, Req 2) dan perubahan atas `vendor-pks` Req 2.4. Alasan Req 2.4 dulu adalah direktur pindah ke CSV PIC, bukan keamanan impor; dengan kolom baca-saja, impor tetap memproses 9 kolom yang sama. | **(b):** impor tidak disentuh (Req 2 hilang), header Indonesia dan audit `report_exported` ikut report. Perlu pratinjau untuk kind report: `vendorPksProducer` belum `PreviewProducer` dan keyset-nya `(name, id)`, sedangkan cursor pratinjau `after` berupa `id`. User mendapat dua file yang harus digabung manual lewat kode vendor. **(c):** tidak memenuhi "file ekspor"; tidak disarankan. |
| D2 | Kolom wajib | Semua 9 kolom, atau sebagian | **Semua** (urutan di Req 1.1), sama dengan isi VendorPksReport tanpa kode/nama vendor yang sudah ada. | Req 1.1, 5.1 menyesuaikan. |
| D3 | Riwayat PKS | Hanya aktif, atau ikut riwayat | **Hanya PKS aktif.** Riwayat sudah ada di tab PKS detail vendor. Riwayat membuat lebih dari satu baris per vendor, yang merusak kunci `id` untuk impor dan keyset. | Riwayat butuh dataset terpisah (bukan CSV vendor); Req 1.3 berubah. |
| D4 | Vendor INTERNAL (ROH) | Sel kosong, atau "-" | **Kosong**, sama dengan VendorPksReport dan layar Admin Vendor. "-" akan membuat dua file untuk data yang sama berbeda isi. | Req 1.6 dan test parity 6.1 menyesuaikan. |
| D5 | Filter Status PKS | Ya, atau tidak | **Tidak** di spec ini. Filter itu sudah ada di Admin Vendor bersama ekspor report `vendor_pks`. Menambahkannya di sini butuh parameter baru di preview, export, `export_jobs.params`, dan query hitung. | Tambah requirement filter (API, `export_jobs` params, count, UI, test). |
| D6 | Role | Sama dengan ekspor vendor sekarang, atau hanya `ADMIN`/`ADMIN_PARAM` | **Sama dengan ekspor vendor sekarang.** Hari ini keduanya sudah `ADMIN`, `ADMIN_PARAM`, jadi tidak ada perbedaan. | Bila role ekspor master data nanti diperluas, kolom ini ikut terbuka; PO bisa meminta kolom disembunyikan untuk role selain `VendorPksViewerRoles` (butuh header per role). |
| D7 | Catatan di halaman impor | Ya, atau tidak | **Ya**, teks statis singkat saat entitas vendor dipilih, supaya user yang mengedit kolom PKS di Excel tahu nilainya tidak diproses. | Req 2.7 dihapus. |
| D8 | Nama kolom dan nilai status | Nama kolom snake_case Inggris atau label Indonesia; status sebagai label ("Aktif") atau key (`aktif`) | **Nama kolom snake_case** (mengikuti kolom vendor yang sudah ada), **status sebagai label Indonesia** (sama dengan report dan layar Admin Vendor; kolom ini untuk dibaca, bukan diimpor). | Req 1.1, 1.2, 5.2 menyesuaikan. |

## Dependency & Scope Note

- **Scope guard:** `development-plan.md` rule 2 melarang fitur master data baru kecuali dengan izin user, dan rule 1 meminta item tercatat di rencana. Fitur ini butuh izin user sebelum build dan entri baru (usul: **X.16**) di tabel cross-cutting `development-plan.md`.
- Bergantung pada spec yang sudah dibangun: `vendor-pks` (data PKS, Director_PIC, VendorPksReport), `vendor-data-export` (export jobs), `export-menu-revamp` + `export-preview-simplify` (halaman pratinjau), `master-data-import-revamp` (impor).
- Deploy: `backend` versi baru harus jalan sebelum atau bersamaan dengan `backend-exq` versi baru. File dengan Export_Header ditolak oleh `backend` versi lama.
