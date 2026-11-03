# Requirements Document

> **Status:** requirements disetujui 2026-10-30. PO memilih mengikuti semua saran Kiro (D1-D11) dan menyetujui item X.9 di `development-plan.md`.
>
> **Catatan 2026-10-30:** Req 5.1 (label tombol ekspor dengan jumlah baris) dan Req 5.9 (catatan pengambilan file) digantikan `export-preview-simplify` (`.kiro/specs/export-preview-simplify/`). Teks requirement di bawah tidak diubah.

## Introduction

### Masalah

Menu Export Data hari ini berbentuk form. Pengguna memilih level (vendor / cabang), vendor, cabang, jenis data, dan status, lalu menekan "Minta ekspor" tanpa pernah melihat isi data yang akan diekspor. Pengguna baru tahu isinya setelah membuka file CSV. Kalau filternya salah, pengguna harus meminta ekspor ulang, dan setiap permintaan memakai satu slot dari batas 3 job aktif.

Revamp yang diminta PO:

1. Pengguna memilih data apa yang diekspor.
2. Aplikasi membuka halaman baru yang menampilkan tabel data tersebut beserta filter.
3. Pengguna mengekspor data sesuai tabel yang tampil.

### Kondisi kode saat ini (dicek 2026-10-30, `dev1` @ `0e2cc18`)

**Frontend CompanyPortal** (`frontend/CompanyPortal-Vite/src/features/exports/`)

- `/settings/admin/exports` (`routes/settings/admin/exports.tsx`, guard `ADMIN`/`ADMIN_PARAM`) merender `ExportDataPage`: kartu "Data Vendor" (`VendorExportForm`), "Harga Paket" (`PackagePriceExportForm`), dan "Ekspor Saya" (`MyExportsTable`). Halaman ini dibuka dari kartu `admin-exports` di `SettingsHubPage.tsx`. Tidak ada item sidebar.
- `/exports` (`routes/exports.tsx`, guard `ADMIN`, `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`) merender `MyExportsPage`: daftar job saja, tanpa form, tanpa item sidebar. Halaman ini hanya dibuka lewat tautan di `ReportExportNotice` dan `ExportProgressDialog`.
- `useExportFlow()` (`hooks.ts`) menangani permintaan, polling job tiap 1 detik, unduh otomatis sekali (`claimAutoDownload`), fase dialog, dan peringatan tutup tab. `ExportProgressDialog` menautkan "Ekspor Saya" ke `/settings/admin/exports` untuk master data dan `/exports` untuk laporan.
- `MasterDataIOPage.tsx` punya bagian ekspor global sendiri (pilih entitas + status, "Minta ekspor" dengan kind `master_data_global`) di samping impor dan unduh templat.
- Layar laporan `DsrLateReportScreen` dan `ReplenishmentScreen` sudah berbentuk tabel + filter + tombol ekspor yang mengirim filter layar (spec `report-export-migration`).
- Komponen yang bisa dipakai ulang: `DataTable` (TanStack Table v8, paging hanya sisi klien), `FilterSelect`, `PageNumbers`, `PageHeader`, `EmptyState`, `NoticeBanner`, `VendorSelect`, `BranchSelect`, `STATUS_OPTIONS`.
- VendorPortal tidak punya kode ekspor.

**Backend `backend-exq`** (`internal/exportjob/`, `internal/handler/exports_handler.go`)

- Kind master data dan perannya (`kinds.go`): `vendor`, `cabang`, `harga_paket`, `master_data_global`, semuanya `ADMIN`/`ADMIN_PARAM`. Kind laporan: `dsr_late_daily`, `dsr_late_summary` (peran `reports.DsrLateViewerRoles`), `replenishment_amount_mismatch`, `replenishment_trips` (peran `reports.ReplenishmentViewerRoles`).
- `ParseParams` (`params.go`) memvalidasi entity per kind, `status` (`active` default, `disabled`, `all`), bentuk scope vendor/cabang, dan `price_level` (khusus `harga_paket`).
- Producer membaca query keyset `WHERE id > after_id ORDER BY id LIMIT batch_size` (`backend/queries/master_data_export.sql`, `backend-exq/queries/vendor_export.sql`). Pada query bersama, `vendor_id`/`branch_id` NULL berarti semua baris.
- **Tidak ada endpoint JSON yang menampilkan baris data** di `backend-exq`. Yang ada hanya permintaan ekspor (`POST`), templat, dan metadata job (`/jobs*`).
- Endpoint daftar di `backend/` (`/api/v1/admin/vendors`, `.../branches`, `.../vaults`, `.../pics`, `.../package-prices`, `/api/v1/admin/atms`) **tidak cocok** dengan parameter ekspor: ada pencarian `q` dan filter yang tidak dikenal ekspor, filter cabang dan `price_level` tidak ada, dan tidak ada daftar global untuk cabang/vault/PIC/kelolaan ATM. Tabel dari endpoint itu tidak menjamin isi file sama dengan tabel.

**Database**: `export_jobs` (migrasi `034`, `035`). Migrasi terakhir `035_export_jobs_reports.sql`.

### Ruang lingkup

- Katalog data ekspor di CompanyPortal (langkah 1).
- Halaman pratinjau per data master: tabel + filter + jumlah baris (langkah 2).
- API pratinjau baru di `backend-exq` yang memakai validasi parameter, query, dan pemetaan sel yang sama dengan ekspor, supaya isi tabel sama dengan isi file.
- Tombol ekspor di halaman pratinjau yang meminta ekspor dengan filter tabel (langkah 3), memakai `useExportFlow` dan `ExportProgressDialog` yang ada.
- Letak menu, rute, dan daftar Ekspor Saya disatukan.
- Form ekspor lama (`VendorExportForm`, `PackagePriceExportForm`) dihapus.

### Tidak termasuk

- Perubahan endpoint permintaan ekspor (`POST /api/v1/exports/...`), siklus `export_jobs`, worker, janitor, batas (3 job aktif, 10 menit, 100 MB / 1.000.000 baris), dan format CSV.
- Filter baru di luar parameter ekspor yang ada, seperti pencarian teks atau pilihan kolom (D7).
- Halaman pratinjau baru untuk laporan DSR telat dan replenishment (D3).
- Ekspor dari VendorPortal, XLSX/PDF, data baru yang belum bisa diekspor hari ini.
- Perubahan impor CSV dan unduh templat.

### Area sensitif (project-context Sec 3 rule 7)

- **RBAC:** API pratinjau menampilkan baris master data. Hak lihatnya harus sama persis dengan hak ekspor per kind, dicek di guard route dan di service.
- **Money:** pratinjau Harga Paket menampilkan `base_price`, `tier_min`, `tier_max`. Nilai harus teks persis dari database, sama dengan file, tanpa float.
- Tidak ada migrasi bila D7 mengikuti saran.

### Dependency & Scope Note

Dibangun di atas `vendor-data-export` (X.7, commit `05af657`) dan `report-export-migration` (X.8, commit `0e2cc18`); keduanya sudah di-commit di `dev1`. Dicatat sebagai item cross-cutting **X.9 Export menu revamp** (PO setuju 2026-10-30).

## Glossary

- **Export_Dataset**: satu jenis data yang bisa dipilih di katalog, mis. "Vault" atau "Harga Paket". Satu Export_Dataset master data dipetakan ke satu atau lebih pasangan (Export_Kind, entity) tergantung filter (lihat Requirement 3).
- **Export_Kind**: nilai `kind` di `export_jobs` dan `KindSpecs` (`vendor`, `cabang`, `harga_paket`, `master_data_global`, dan empat kind laporan).
- **Export_Params**: parameter ekspor tervalidasi hasil `exportjob.ParseParams` (kind, entity, vendor, cabang, status, tingkat harga).
- **ExportCatalogPage**: halaman CompanyPortal untuk memilih Export_Dataset (langkah 1).
- **ExportPreviewPage**: halaman CompanyPortal per Export_Dataset master data yang menampilkan tabel, filter, jumlah baris, dan tombol ekspor (langkah 2 dan 3).
- **Applied_Filter**: filter yang sedang dipakai tabel di ExportPreviewPage, yaitu filter yang menghasilkan baris yang sedang tampil. Berbeda dari isian filter yang sudah diubah tapi belum diterapkan.
- **ExportPreviewAPI**: endpoint JSON baru di `backend-exq` yang mengembalikan kolom, satu halaman baris, dan jumlah total untuk Export_Params.
- **Export_Producer**: producer `backend-exq` yang menulis CSV satu entity.
- **CSV_Contract**: header dan pemetaan sel di `pkg/csvexport`.
- **Export_Roles**: peran yang boleh meminta ekspor satu Export_Kind (`exportjob.KindRoles`).
- **ExportProgressDialog**, **useExportFlow**, **MyExportsTable**: komponen dan hook yang ada di `features/exports/`.
- **Report_Screen**: layar laporan yang sudah punya tabel + filter + tombol ekspor: `DsrLateReportScreen`, `ReplenishmentScreen`.

## Requirements

### Requirement 1: Katalog data ekspor

**User Story:** Sebagai pengguna yang punya hak ekspor, saya ingin memilih data apa yang mau diekspor dari satu daftar, supaya saya tidak perlu tahu kombinasi level dan entitas yang dipakai sistem.

#### Acceptance Criteria

1. WHEN pengguna membuka menu Export Data, THE ExportCatalogPage SHALL menampilkan daftar Export_Dataset yang dikelompokkan menjadi "Data master" dan "Laporan".
2. THE ExportCatalogPage SHALL hanya menampilkan Export_Dataset yang Export_Kind-nya boleh diminta oleh peran pengguna; Export_Dataset lain tidak tampil sama sekali.
3. THE ExportCatalogPage SHALL menampilkan untuk setiap Export_Dataset nama, deskripsi singkat, dan filter yang tersedia.
4. WHEN pengguna memilih Export_Dataset master data, THE CompanyPortal SHALL membuka ExportPreviewPage untuk data tersebut di rute tersendiri.
5. WHEN pengguna memilih Export_Dataset laporan, THE CompanyPortal SHALL membuka Report_Screen yang sudah ada untuk laporan tersebut (D3).
6. IF peran pengguna tidak punya Export_Dataset apa pun, THEN THE CompanyPortal SHALL menolak akses ke ExportCatalogPage dengan perilaku guard rute yang ada.
7. THE ExportCatalogPage SHALL bisa dioperasikan dengan keyboard, dan setiap pilihan data punya nama yang bisa dibaca screen reader.

### Requirement 2: Halaman pratinjau data

**User Story:** Sebagai admin data master, saya ingin melihat tabel data yang akan diekspor beserta filternya sebelum mengekspor, supaya saya yakin isi file sudah benar.

#### Acceptance Criteria

1. THE ExportPreviewPage SHALL menampilkan tabel dengan kolom dan urutan kolom yang sama dengan header CSV_Contract untuk entity yang dipetakan dari Applied_Filter.
2. THE ExportPreviewPage SHALL menampilkan jumlah total baris yang cocok dengan Applied_Filter (D8).
3. THE ExportPreviewPage SHALL menampilkan baris per halaman dengan ukuran 50 dan tombol Sebelumnya / Berikutnya (D9).
4. WHEN pengguna menerapkan filter, THE ExportPreviewPage SHALL memuat ulang tabel dan jumlah total dari halaman pertama dengan filter tersebut.
5. THE ExportPreviewPage SHALL menyimpan Applied_Filter di parameter URL, sehingga muat ulang halaman, tombol kembali browser, dan tautan yang dibagikan menampilkan tabel yang sama.
6. WHILE tabel sedang dimuat, THE ExportPreviewPage SHALL menampilkan status memuat tanpa menghapus filter yang sedang diisi.
7. IF Applied_Filter tidak menghasilkan baris, THEN THE ExportPreviewPage SHALL menampilkan keadaan kosong dengan ikon + teks.
8. IF ExportPreviewAPI gagal, THEN THE ExportPreviewPage SHALL menampilkan pesan galat dengan ikon + teks dan tombol coba lagi.
9. THE ExportPreviewPage SHALL menampilkan angka dan nominal rata kanan dengan `tabular-nums`, nominal dengan font monospace, dan status aktif/nonaktif sebagai badge ikon + teks, sesuai `ui_design.md` Sec 6 dan 8.
10. THE ExportPreviewPage SHALL punya tombol kembali ke ExportCatalogPage.
11. THE tabel dan filter di ExportPreviewPage SHALL bisa dioperasikan dengan keyboard, setiap filter punya label, dan tabel punya caption yang menyebut nama data.

### Requirement 3: Filter per data dan pemetaan ke parameter ekspor

**User Story:** Sebagai admin data master, saya ingin filter di halaman pratinjau sama dengan yang dipakai saat ekspor, supaya file tidak berisi data yang berbeda dari tabel.

#### Acceptance Criteria

1. THE ExportPreviewPage SHALL menyediakan filter dan memetakan Applied_Filter ke Export_Params menurut tabel berikut (D4):

   | Export_Dataset | Filter | Export_Kind / entity |
   | --- | --- | --- |
   | Vendor | status | `master_data_global` / `vendors` |
   | Cabang vendor | vendor (opsional), status | tanpa vendor: `master_data_global` / `vendor-branches`; dengan vendor: `vendor` / `branches` |
   | Vault | vendor (opsional), cabang (opsional, butuh vendor), status | tanpa vendor: `master_data_global` / `vendor-vaults`; vendor: `vendor` / `vaults`; vendor + cabang: `cabang` / `vaults` |
   | PIC vendor | vendor (opsional), cabang (opsional, butuh vendor), status | tanpa vendor: `master_data_global` / `vendor-pics`; vendor: `vendor` / `pics`; vendor + cabang: `cabang` / `pics` |
   | Pemetaan lokasi DSR | vendor (wajib), cabang (opsional), status | vendor: `vendor` / `dsr-location-maps`; vendor + cabang: `cabang` / `dsr-location-maps` |
   | Harga Paket | vendor (wajib), cabang (opsional), tingkat harga, status | `harga_paket` / `package-prices` |
   | ATM | status | `master_data_global` / `atms` |
   | Kelolaan ATM | status | `master_data_global` / `atm-assignments` |

2. THE filter status SHALL menawarkan `active` (default), `disabled`, dan `all`; filter tingkat harga SHALL menawarkan `all` (default), `vendor`, `branch`, `atm`.
3. WHEN pengguna mengosongkan vendor, THE ExportPreviewPage SHALL ikut mengosongkan dan menonaktifkan filter cabang.
4. WHILE filter wajib (vendor untuk Pemetaan lokasi DSR dan Harga Paket) belum diisi, THE ExportPreviewPage SHALL tidak memanggil ExportPreviewAPI dan menampilkan petunjuk untuk memilih vendor.
5. THE pratinjau dan permintaan ekspor SHALL memakai fungsi pemetaan Applied_Filter → Export_Params yang sama, sehingga untuk Applied_Filter yang sama keduanya selalu menghasilkan Export_Kind, entity, vendor, cabang, status, dan tingkat harga yang sama.
6. THE semantik scope SHALL sama dengan ekspor: PIC dengan scope cabang hanya memuat PIC cabang itu (D3 `vendor-data-export`), PIC dengan scope vendor memuat PIC level vendor dan PIC semua cabangnya.

### Requirement 4: API pratinjau di backend-exq

**User Story:** Sebagai pengembang, saya ingin satu endpoint pratinjau yang membaca data dengan aturan yang sama dengan ekspor, supaya tabel dan file tidak bisa berbeda.

#### Acceptance Criteria

1. THE ExportPreviewAPI SHALL menerima parameter yang sama dengan permintaan ekspor master data (kind, entity, vendor, cabang, status, tingkat harga) dan memvalidasinya dengan `exportjob.ParseParams`; parameter tidak valid ditolak 400 dan entity yang tidak cocok dengan kind ditolak 404, sama dengan permintaan ekspor.
2. THE ExportPreviewAPI SHALL mengembalikan daftar kolom (header CSV_Contract), baris halaman yang diminta, jumlah total baris yang cocok, dan kursor halaman berikutnya (kosong bila tidak ada).
3. THE ExportPreviewAPI SHALL menerima ukuran halaman 1 sampai 100 (default 50) dan kursor keyset; ukuran atau kursor tidak valid ditolak 400.
4. THE ExportPreviewAPI SHALL membaca baris dengan query keyset yang sama dengan Export_Producer untuk entity tersebut, dengan urutan `id` naik.
5. THE ExportPreviewAPI SHALL mengisi setiap sel dengan pemetaan sel yang sama dengan Export_Producer (teks nilai database, NULL menjadi sel kosong, numeric dan tanggal sebagai teks tanpa float), tanpa awalan pengaman formula `CSVSafe` yang hanya berlaku di file.
6. THE ExportPreviewAPI SHALL membaca jumlah total dan baris halaman dari replica dalam satu transaksi `REPEATABLE READ READ ONLY`, sehingga keduanya berasal dari snapshot yang sama.
7. THE jumlah total SHALL dihitung dengan kondisi filter yang sama dengan query baris, sehingga untuk snapshot yang sama jumlah total sama dengan jumlah baris yang ditulis Export_Producer.
8. THE ExportPreviewAPI SHALL mengizinkan pratinjau satu Export_Kind hanya untuk Export_Roles kind tersebut, di guard route dan dicek ulang di service; pemanggil tanpa peran ditolak 403.
9. THE ExportPreviewAPI SHALL memakai `as_of` tanggal WIB saat permintaan untuk Harga Paket, sama dengan aturan status Export_Producer.
10. THE ExportPreviewAPI SHALL memakai envelope `pkg/response` dan format galat yang sama dengan endpoint `/api/v1/exports` lain.
11. THE ExportPreviewAPI SHALL tidak menulis `audit_logs` (D5).

### Requirement 5: Ekspor sesuai tabel

**User Story:** Sebagai admin data master, saya ingin menekan satu tombol di halaman pratinjau untuk mengekspor data sesuai tabel, supaya file berisi persis data yang saya lihat.

#### Acceptance Criteria

1. THE ExportPreviewPage SHALL menampilkan tombol ekspor dengan jumlah baris yang akan diekspor, mis. "Ekspor 1.240 baris".
2. WHEN pengguna menekan tombol ekspor, THE ExportPreviewPage SHALL meminta ekspor dengan Export_Params dari Applied_Filter, bukan dari isian filter yang belum diterapkan.
3. THE ekspor SHALL mencakup semua baris yang cocok dengan Applied_Filter di semua halaman, bukan hanya halaman yang tampil (D11).
4. WHILE ada isian filter yang sudah diubah tapi belum diterapkan, THE ExportPreviewPage SHALL menonaktifkan tombol ekspor dan menampilkan petunjuk untuk menerapkan filter dulu.
5. IF jumlah total 0 atau tabel belum termuat, THEN THE ExportPreviewPage SHALL menonaktifkan tombol ekspor.
6. IF jumlah total melebihi 1.000.000 baris, THEN THE ExportPreviewPage SHALL menonaktifkan tombol ekspor dan menampilkan pesan ikon + teks untuk mempersempit filter.
7. WHEN permintaan ekspor diterima, THE ExportPreviewPage SHALL membuka ExportProgressDialog dengan perilaku `vendor-data-export` Requirement 12.8-12.15 (tahap, unduh otomatis sekali, peringatan tutup tab, aksesibilitas).
8. IF permintaan ekspor ditolak (400, 403, 429), THEN THE ExportPreviewPage SHALL menampilkan pesan ikon + teks di dekat tombol ekspor tanpa membuka dialog.
9. THE ExportPreviewPage SHALL memberi tahu pengguna bahwa file diambil saat ekspor diproses, sehingga perubahan data setelah tabel dimuat bisa ikut masuk.
10. FOR ANY Applied_Filter dan snapshot data yang sama, THE baris file ekspor (tanpa BOM, setelah membuang awalan `CSVSafe`) SHALL sama dengan gabungan semua halaman pratinjau, dengan urutan yang sama.

### Requirement 6: Menu, rute, dan Ekspor Saya

**User Story:** Sebagai pengguna yang punya hak ekspor, saya ingin satu menu Export Data yang mudah ditemukan, dengan daftar ekspor saya di tempat yang sama.

#### Acceptance Criteria

1. THE CompanyPortal SHALL menampilkan item sidebar "Export Data" yang membuka ExportCatalogPage di `/exports`, untuk gabungan Export_Roles semua Export_Kind (D2).
2. THE ExportCatalogPage SHALL memuat bagian "Ekspor Saya" yang menampilkan semua Export_Job milik pengguna, baik master data maupun laporan, memakai MyExportsTable (D10).
3. THE ExportPreviewPage SHALL berada di rute anak `/exports/<data>` dengan guard peran sesuai Export_Roles data tersebut.
4. WHEN pengguna membuka `/settings/admin/exports`, THE CompanyPortal SHALL mengalihkan ke `/exports`.
5. THE kartu `admin-exports` di SettingsHubPage SHALL membuka `/exports`.
6. THE tautan "Ekspor Saya" di ExportProgressDialog dan ReportExportNotice SHALL menunjuk ke `/exports` untuk semua Export_Kind.
7. THE MasterDataIOPage SHALL mengganti bagian ekspor globalnya dengan tautan ke ExportPreviewPage entity yang sedang dipilih (D6); impor dan unduh templat tidak berubah.

### Requirement 7: Otorisasi dan audit tidak berubah

**User Story:** Sebagai pemilik sistem, saya ingin revamp ini tidak membuka akses data ke peran baru dan tidak mengubah jejak audit ekspor.

#### Acceptance Criteria

1. THE ekspor master data SHALL tetap hanya bisa dipratinjau dan diminta oleh `ADMIN` dan `ADMIN_PARAM`.
2. THE ekspor laporan SHALL tetap memakai peran Report_Screen yang ada (`reports.DsrLateViewerRoles`, `reports.ReplenishmentViewerRoles`).
3. THE audit `export_requested` saat permintaan dan `master_data_exported` / `report_exported` saat unduh SHALL tidak berubah bentuknya.
4. THE aturan kepemilikan Export_Job (hanya peminta yang melihat dan mengunduh, D19 `vendor-data-export`) SHALL tetap berlaku di bagian Ekspor Saya.

### Requirement 8: Pembersihan UI lama dan kontrak

**User Story:** Sebagai pengembang, saya ingin hanya ada satu jalur ekspor master data di UI dan kontrak API yang selalu sinkron.

#### Acceptance Criteria

1. THE CompanyPortal SHALL menghapus `VendorExportForm`, `PackagePriceExportForm`, dan `ExportDataPage` beserta test-nya, dan komponen bersama yang tidak lagi dipakai.
2. THE kontrak `backend-exq/api/openapi.yaml` SHALL mencakup ExportPreviewAPI, dengan test kontrak yang lulus, dan tipe frontend di-generate ulang dengan `gen:api:exq`.
3. THE endpoint permintaan ekspor, templat, dan `/jobs*` di `backend-exq` SHALL tidak berubah perilaku dan bentuk responsnya.

### Requirement 9: Non-regresi dan gate kualitas

**User Story:** Sebagai pengembang, saya ingin revamp ini tidak merusak ekspor dan layar yang sudah ada.

#### Acceptance Criteria

1. THE Report_Screen, alur ekspor laporan, dan isi file ekspor master data SHALL tidak berubah.
2. THE implementasi SHALL tidak menambah library baru dan tidak menambah migrasi (D7).
3. THE implementasi SHALL lolos gate coverage yang berlaku: Go ≥ 80% untuk `backend-exq/internal/*` dan `pkg/`, CompanyPortal 84/80/80/85 (`pnpm run test:coverage`), plus lint, build, dan `check-bundle.mjs`.

## Open Questions

PO memilih mengikuti semua saran Kiro pada 2026-10-30.

| # | Pertanyaan | Saran Kiro | Status |
| --- | --- | --- | --- |
| D1 | "Endpoint baru" berarti apa? | **Rute halaman baru per data** (`/exports/<data>`) di CompanyPortal **plus API pratinjau JSON baru** di `backend-exq`. Endpoint daftar yang ada di `backend/` tidak bisa dipakai: filternya berbeda dari parameter ekspor dan tidak ada daftar global untuk cabang/vault/PIC/kelolaan ATM, jadi tabel bisa berbeda dari file. | ✅ mengikuti saran (PO 2026-10-30) |
| D2 | Letak menu | **Item sidebar "Export Data"** (`/exports`, grup General) untuk semua peran yang punya hak ekspor apa pun. Alternatif: tetap kartu di Pengaturan, tetapi itu hanya terlihat ADMIN/ADMIN_PARAM/APPACCESS sehingga pengguna laporan tidak menemukan katalog. | ✅ mengikuti saran (PO 2026-10-30) |
| D3 | Data laporan (DSR telat, replenishment) ikut dibuatkan halaman pratinjau baru? | **Tidak.** Kartu laporan di katalog membuka Report_Screen yang ada; layar itu sudah berupa tabel + filter + tombol ekspor sesuai filter layar. Halaman baru hanya menggandakan layar dan query. | ✅ mengikuti saran (PO 2026-10-30) |
| D4 | Bentuk daftar data master | **Berdasarkan data** (8 data, tabel Requirement 3.1), cakupan vendor/cabang dipilih lewat filter. Hari ini pengguna memilih level dulu lalu entitas. Profil satu vendor (kind `vendor`, entity `vendor`, selalu satu baris) tidak ditawarkan; data "Vendor" memakai daftar global. Endpoint-nya tetap ada. | ✅ mengikuti saran (PO 2026-10-30) |
| D5 | Melihat pratinjau dicatat di audit log? | **Tidak**, sama dengan layar daftar admin yang tidak mengaudit baca. Permintaan dan unduhan ekspor tetap diaudit. | ✅ mengikuti saran (PO 2026-10-30) |
| D6 | Bagian ekspor di halaman Ekspor & Impor CSV | **Diganti tautan** ke halaman pratinjau entity yang dipilih, supaya hanya ada satu jalur ekspor. Impor dan templat tetap. | ✅ mengikuti saran (PO 2026-10-30) |
| D7 | Filter tambahan (cari teks, pilih kolom)? | **Tidak di versi ini.** Filter = parameter ekspor yang ada, jadi tidak ada migrasi dan file dijamin sama dengan tabel. Pencarian teks butuh kolom baru di `export_jobs`, perubahan `params_hash`, dan perubahan query producer; lebih baik spec terpisah. | ✅ mengikuti saran (PO 2026-10-30) |
| D8 | Tampilkan jumlah total baris? | **Ya, jumlah persis**, lewat query hitung baru per entity dengan kondisi yang sama. Pengguna perlu tahu ukuran file sebelum mengekspor. | ✅ mengikuti saran (PO 2026-10-30) |
| D9 | Paging tabel | **50 baris per halaman, Sebelumnya / Berikutnya** (keyset, sama dengan producer). Lompat ke halaman N tidak tersedia karena keyset tidak mendukung offset. | ✅ mengikuti saran (PO 2026-10-30) |
| D10 | Daftar Ekspor Saya | **Satu bagian di `/exports`** untuk semua job (master data dan laporan). `MyExportsPage` yang terpisah tidak dipakai lagi. | ✅ mengikuti saran (PO 2026-10-30) |
| D11 | "Ekspor sesuai tabel" = halaman yang tampil saja, atau semua hasil filter? | **Semua baris hasil filter** di semua halaman. Mengekspor 50 baris yang tampil saja jarang berguna dan butuh parameter ekspor baru. | ✅ mengikuti saran (PO 2026-10-30) |
