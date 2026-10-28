# Requirements Document

## Introduction

### Masalah

Admin master data perlu mengambil data satu vendor (atau satu cabang vendor) per entitas, misalnya daftar vault atau PIC milik satu vendor saja, untuk review PKS, serah terima, atau pengecekan sebelum import massal. Hari ini satu-satunya jalan adalah halaman Import/Export master data, yang mengekspor **satu entitas untuk semua vendor sekaligus**. Admin harus menyaring sendiri di Excel, dan harga paket sama sekali tidak bisa diekspor.

Sekaligus, semua kegiatan export data dipindahkan ke `backend-exq` (D13) dan dijalankan sebagai **proses asinkron** (D7): user meminta ekspor, sistem membuat file di latar belakang, lalu user mengunduhnya dari daftar ekspor.

### Kondisi kode saat ini (dicek 2026-10-28)

- **Export yang ada (ATM backend `backend/`):** `GET /api/v1/admin/master-data/export/{entity}?status=active|disabled|all` (`admin_master_data_export_handler.go`, `service/masterdata_export.go`). Entitas: `vendors`, `vendor-branches`, `vendor-vaults`, `vendor-pics`, `atms`, `atm-assignments`. Sinkron, stream keyset 1000 baris dari replica, CSV ber-BOM UTF-8, sel lewat `CSVSafe`, header stabil ber-natural key. Tidak ada filter vendor/cabang. Tidak menulis audit.
- **Importer ikut memakai `exportSpecs`:** selain header, importer (`masterdata_import.go` `loadAll`) memakai fungsi `page` export (baris DB → sel CSV) untuk memuat data saat ini lalu membandingkannya dengan file (create / update / unchanged). Format sel export harus sama persis dengan pembanding importer, bukan hanya header-nya.
- **Pola audit export laporan:** `dsr_late_report.go` dan `replenishment_read.go` menulis audit `report_exported` di primary sebelum file dikirim.
- **`backend-exq/`:** skeleton. `cmd/api/main.go` hanya punya `/health`, `/ready`, dan grup `RequireAuth` kosong. Hanya pool primary (`pkg/config` sudah membaca `DATABASE_REPLICA_URL`), tanpa `TrustedRealIP`, `RequireRoles`, audit writer, worker, atau query (`queries/` kosong; `sqlc.yaml` memakai schema `../backend/migrations/`). Server memakai chi `Timeout(30s)` dan `WriteTimeout` 30 detik. Image root tanpa HEALTHCHECK (BE-06) dan **belum di-deploy**. CompanyPortal belum merutekan apa pun ke `:8081`.
- **Batas modul:** `backend-exq` tidak boleh mengimpor `backend/` (tech.md prinsip 9). `CSVSafe`, `exportSpecs`, `audit.Writer`, dan `notification` ada di `backend/internal/`. Yang tersedia di `pkg/`: `auth` (klaim JWT `id`, `username`, `role`), `middleware` (`RequireAuth`, `RequireRoles`, `TrustedRealIP`, `RequestLogger`, `BodyLimit`), `config`, `response`, `health`.
- **Tabel:** `export_jobs` sudah ada di daftar nama tabel kanonis (CLAUDE.md Sec 3) tetapi **belum dimigrasi**. Migrasi terakhir `033_dsr_receipts.sql`; nomor bebas berikutnya `034` (`022` dicadangkan). GCS (`GCS_BUCKET`) ada di daftar env tetapi belum terpasang di kode, dan tidak ada library GCS di `go.mod`.
- **Relasi data:** `vendor_branches.vendor_id` → `vendors`; `vendor_vaults.vendor_branch_id` → cabang; `vendor_pics.vendor_id` wajib, `vendor_branch_id` NULL = PIC vendor-wide; `dsr_location_vault_maps` menempel ke vendor dan ke vault; `vendor_package_prices` menempel ke vendor, opsional ke cabang **atau** ATM (`vpp_one_level_chk`), uang `numeric(20,2)` + `currency`.

### Ruang lingkup

- Scope baru **Export Data** (D9), dijalankan di `backend-exq` (D10, D13).
- Ekspor **satu CSV per entitas yang dipilih** (D1), pada **Level_Vendor** atau **Level_Cabang**.
- Ekspor **Harga Paket** sebagai menu terpisah (D4, D4a).
- **Pindah export master data global** yang sudah ada (6 entitas + template kosong) dari `backend/` ke `backend-exq` (D14). Isi file tidak berubah; cara unduhnya menjadi asinkron (D7). Kontrak CSV pindah ke `pkg/` dan dipakai bersama importer (D11). Endpoint lama di `backend/` dihapus di rilis yang sama (D15).
- **Proses asinkron** (D7): tabel `export_jobs` + penyimpanan file, worker di `backend-exq`, daftar ekspor milik user, unduh, masa simpan file.
- Filter status, audit user peminta dan pengunduh (D5, D16), menu Export Data di CompanyPortal (D12), kontrak OpenAPI `backend-exq` (D8).
- Kesiapan `backend-exq` untuk deploy: pool replica, `TrustedRealIP`, guard peran, audit, worker, routing CompanyPortal ke `:8081`, image non-root + HEALTHCHECK.

### Tidak termasuk

- ZIP / bundle multi-file, XLSX, PDF (D1; XLSX/PDF butuh library baru, Golden Rule #1).
- Ekspor data ATM per vendor / per cabang (D2). Export global `atms` dan `atm-assignments` yang sudah ada tetap tersedia karena dipindah (Requirement 14).
- Ekspor Paket Cabang (`vendor_packages_branch`), D4a.
- Pemindahan CSV laporan DSR telat dan laporan replenishment: spec lanjutan `report-export-migration` (D14).
- Import balik untuk entitas yang baru bisa diekspor (mapping DSR, harga paket). File-nya export-only.
- Perubahan perilaku import master data (tetap di `backend/`, sinkron).
- Ekspor dari VendorPortal, ekspor massal semua vendor dalam satu file, ekspor terjadwal / berulang.
- Notifikasi in-app atau email saat ekspor selesai (D20).
- Data transaksional (Vendor Request, DSR, realisasi, order). Dashboard query di `backend-exq`.

### Area sensitif (project-context Sec 3 rule 7)

- **Auth/RBAC:** bulk read data pribadi PIC vendor dan harga kontrak dari backend kedua yang hanya memvalidasi token. File hasil hanya boleh diunduh oleh peminta (D19).
- **Data at rest:** asinkron berarti file berisi data pribadi tersimpan di server sampai diunduh. File dihapus segera setelah berhasil diunduh (D18), dan file yang tidak pernah diunduh dihapus otomatis (D18c).
- **Money:** harga paket dan kapasitas vault diekspor sebagai teks persis dari `numeric`, tidak pernah lewat float.
- **Migrasi:** tabel baru `export_jobs` + `export_job_chunks` (aditif, tanpa menyentuh data yang ada).
- **Infrastruktur:** konten pertama `backend-exq`, jadi memicu deployment pertamanya.

### Dependency & Scope Note

User menyetujui (2026-10-28) scope baru **Export Data** di luar Phase 2, dicatat di `development-plan.md` sebagai item cross-cutting X.7. Export bersifat read-only terhadap data master, jadi tidak melanggar scope guard rule 2. Spec lanjutan `report-export-migration` (X.8) bergantung pada spec ini dan memakai mekanisme `export_jobs` yang sama.

## Glossary

- **ExportRequestAPI**: endpoint `backend-exq` di bawah `/api/v1/exports/` untuk membuat Export_Job, melihat daftar dan status Export_Job, dan mengunduh file hasil.
- **Export_Job**: satu baris `export_jobs` yang mewakili satu permintaan ekspor (satu file CSV), dengan status teknis dan metadata.
- **Export_File**: isi CSV hasil Export_Job, disimpan sebagai potongan di `export_job_chunks` sampai berhasil diunduh satu kali atau kedaluwarsa.
- **ExportWorker**: proses latar belakang di `backend-exq` yang mengambil Export_Job berstatus `queued`, membuat Export_File, lalu menandai hasilnya.
- **VendorExporter**: logika pembuat CSV untuk Level_Vendor dan Level_Cabang.
- **PackagePriceExporter**: logika pembuat CSV Harga Paket.
- **MasterDataExporter**: logika pembuat CSV export master data global (semua vendor) per entitas, pindahan dari `backend/`.
- **TemplateAPI**: endpoint sinkron `backend-exq` untuk template CSV kosong (header saja, tanpa data).
- **ExqAuditWriter**: penulis baris `audit_logs` milik `backend-exq` (append-only, primary).
- **CSV_Contract**: paket bersama di `pkg/` berisi header, format sel per entitas master data, BOM, dan `CSVSafe`; satu-satunya sumber format yang dipakai export (`backend-exq`) dan importer (`backend/`).
- **Existing_Export_Header**: header CSV per entitas di `exportSpecs` (`backend/internal/service/masterdata_export.go`) per 2026-10-28, menjadi acuan snapshot CSV_Contract.
- **Export_Kind**: jenis ekspor: `vendor` (Level_Vendor), `cabang` (Level_Cabang), `harga_paket`, `master_data_global`.
- **Level_Vendor**: cakupan = satu vendor beserta seluruh cabangnya.
- **Level_Cabang**: cakupan = satu cabang vendor.
- **Export_Entity**: entitas yang dipilih untuk satu ekspor (mis. `vaults`, `pics`). Satu Export_Job = satu entitas = satu file CSV.
- **Status_Filter**: parameter `status` = `active` | `disabled` | `all`, makna sama dengan export master data yang ada.
- **Export_Data_Page**: halaman menu Export Data di CompanyPortal (`/settings/admin/exports`), berisi menu Data Vendor, menu Harga Paket, dan daftar Ekspor Saya.
- **Export_Progress_Dialog**: pop-up modal yang terbuka setelah user meminta ekspor, menampilkan proses dengan indikator memuat sampai file selesai diunduh.
- **Master_Data_Admin**: pengguna dengan peran `ADMIN` atau `ADMIN_PARAM`.

## Requirements

### Requirement 1: Ekspor per entitas, level vendor

**User Story:** Sebagai Master_Data_Admin, saya ingin mengekspor satu entitas milik satu vendor (mis. semua vault vendor X), supaya saya tidak perlu mengekspor semua vendor lalu menyaring manual.

#### Acceptance Criteria

1. WHEN Master_Data_Admin meminta ekspor Level_Vendor untuk vendor yang ada dan satu Export_Entity, THE ExportRequestAPI SHALL membuat satu Export_Job untuk entitas tersebut.
2. THE ExportRequestAPI SHALL menerima Export_Entity Level_Vendor berikut: `vendor` (profil vendor, satu baris), `branches`, `vaults`, `pics`, dan `dsr-location-maps`.
3. THE VendorExporter SHALL mengisi Export_File hanya dengan baris milik vendor yang diminta: cabang dengan `vendor_id` = vendor; vault milik cabang vendor; PIC dengan `vendor_id` = vendor (vendor-wide dan cabang); mapping DSR dengan `vendor_id` = vendor.
4. IF Export_Entity tidak dikenal untuk level tersebut, THEN THE ExportRequestAPI SHALL menolak dengan 404.
5. WHEN tidak ada baris yang cocok, THE VendorExporter SHALL tetap membuat Export_File berisi baris header saja.
6. WHEN vendor yang diminta sudah dinonaktifkan, THE ExportRequestAPI SHALL tetap mengizinkan ekspor, dan Status_Filter berlaku untuk baris entitas.

### Requirement 2: Ekspor per entitas, level cabang vendor

**User Story:** Sebagai Master_Data_Admin, saya ingin mengekspor satu entitas milik satu cabang vendor saja, supaya saya bisa meninjau data per wilayah operasional tanpa data cabang lain.

#### Acceptance Criteria

1. WHEN Master_Data_Admin meminta ekspor Level_Cabang untuk cabang milik vendor di path dan satu Export_Entity, THE ExportRequestAPI SHALL membuat satu Export_Job untuk entitas tersebut.
2. THE ExportRequestAPI SHALL menerima Export_Entity Level_Cabang berikut: `vaults`, `pics`, dan `dsr-location-maps`.
3. THE VendorExporter SHALL mengisi Export_File Level_Cabang hanya dengan baris milik cabang: vault dengan `vendor_branch_id` = cabang; PIC dengan `vendor_branch_id` = cabang (PIC vendor-wide tidak ikut, D3); mapping DSR yang vault-nya milik cabang.
4. IF cabang yang diminta tidak ada atau bukan milik vendor di path, THEN THE ExportRequestAPI SHALL menolak dengan 404 tanpa membedakan dua kasus tersebut.
5. WHEN cabang yang diminta sudah dinonaktifkan, THE ExportRequestAPI SHALL tetap mengizinkan ekspor, dan Status_Filter berlaku untuk baris entitas.

### Requirement 3: Ekspor Harga Paket (menu terpisah)

**User Story:** Sebagai Master_Data_Admin, saya ingin ekspor harga paket punya menu sendiri, karena harga kontrak lebih sensitif dan punya filter yang berbeda dari data vendor lain.

#### Acceptance Criteria

1. WHEN Master_Data_Admin meminta ekspor Harga Paket untuk vendor yang ada, THE ExportRequestAPI SHALL membuat satu Export_Job berisi baris `vendor_package_prices` vendor tersebut.
2. WHERE Master_Data_Admin mengisi filter cabang, THE PackagePriceExporter SHALL hanya menyertakan harga dengan `vendor_branch_id` = cabang tersebut; IF cabang bukan milik vendor, THEN THE ExportRequestAPI SHALL menolak dengan 404.
3. WHERE Master_Data_Admin mengisi filter tingkat harga (`vendor` / `branch` / `atm` / `all`, default `all`), THE PackagePriceExporter SHALL hanya menyertakan harga pada tingkat tersebut: `vendor` = `vendor_branch_id` dan `atm_id` NULL; `branch` = `vendor_branch_id` terisi; `atm` = `atm_id` terisi.
4. THE PackagePriceExporter SHALL menerapkan Status_Filter berdasarkan periode: `active` = `effective_end_date IS NULL` atau `>=` tanggal WIB saat Export_Job diproses; `disabled` = `effective_end_date <` tanggal tersebut; `all` = semua.
5. THE PackagePriceExporter SHALL menulis kolom `id`, `vendor_code`, `price_level`, `branch_code`, `terminal_id`, `package`, `package_code`, `machine_group`, `price_class`, `tier_min`, `tier_max`, `base_price`, `currency`, `sla_note`, `effective_start_date`, `effective_end_date` (skema setelah migrasi `012` dan `017`; disesuaikan saat design 2026-10-28), dengan `branch_code`/`terminal_id` kosong bila tidak berlaku.
6. THE ekspor Harga Paket SHALL diminta dari menu terpisah di Export_Data_Page, bukan sebagai pilihan Export_Entity pada ekspor data vendor.

### Requirement 4: Filter status

**User Story:** Sebagai Master_Data_Admin, saya ingin memilih apakah data nonaktif ikut diekspor, supaya file sesuai kebutuhan (operasional saja atau histori lengkap).

#### Acceptance Criteria

1. THE ExportRequestAPI SHALL menerima Status_Filter `active`, `disabled`, atau `all`, dengan default `active` bila tidak diisi.
2. IF Status_Filter atau filter tingkat harga berisi nilai lain, THEN THE ExportRequestAPI SHALL menolak dengan 400 tanpa membuat Export_Job.
3. WHILE Status_Filter = `active`, THE VendorExporter SHALL hanya menyertakan baris cabang, vault, PIC, dan mapping DSR dengan `deleted_at IS NULL`.
4. WHILE Status_Filter = `disabled`, THE VendorExporter SHALL hanya menyertakan baris yang kebalikan dari kondisi aktif pada kriteria 3.
5. WHILE Status_Filter = `all`, THE VendorExporter SHALL menyertakan semua baris tanpa memandang status.
6. THE VendorExporter SHALL selalu mengekspor baris profil vendor pada Export_Entity `vendor` apa pun Status_Filter-nya.

### Requirement 5: Format file CSV

**User Story:** Sebagai Master_Data_Admin, saya ingin file ekspor bisa dibuka langsung di Excel dan sebagian bisa dipakai ulang untuk import, supaya tidak ada konversi manual.

#### Acceptance Criteria

1. THE ExportRequestAPI SHALL mengirim Export_File sebagai `text/csv; charset=utf-8` dengan `Content-Disposition: attachment`, `Cache-Control: no-store`, dan `X-Content-Type-Options: nosniff`.
2. THE Export_Job SHALL menyimpan nama file yang ditentukan saat dibuat: `vendor-<vendor_code>-<entity>-<YYYYMMDD>.csv` (Level_Vendor), `vendor-<vendor_code>-cabang-<branch_code>-<entity>-<YYYYMMDD>.csv` (Level_Cabang), `harga-paket-<vendor_code>[-cabang-<branch_code>]-<YYYYMMDD>.csv` (Harga Paket), dan `master-data-<entity>-<YYYYMMDD>.csv` (master data global, sama dengan nama lama), dengan tanggal WIB saat permintaan dan karakter selain huruf, angka, `-`, `_` pada kode diganti `_`.
3. THE VendorExporter SHALL menulis Export_Entity `vendor`, `branches`, `vaults`, dan `pics` dengan header, urutan kolom, dan format sel dari CSV_Contract entitas `vendors`, `vendor-branches`, `vendor-vaults`, dan `vendor-pics`.
4. THE implementasi SHALL punya test otomatis yang gagal bila header CSV_Contract untuk 6 entitas master data berbeda dari snapshot Existing_Export_Header.
5. THE VendorExporter SHALL menulis `dsr-location-maps` dengan header tetap ber-natural key (`id`, `vendor_code`, `branch_code`, `vault_code`, `dsr_location`, `is_active`).
6. THE ExportWorker SHALL memulai setiap Export_File dengan BOM UTF-8 dan melindungi setiap sel dari formula injection dengan `CSVSafe` dari CSV_Contract.
7. THE ExportWorker SHALL menulis nilai uang dan kapasitas sebagai teks persis dari kolom `numeric`, tanpa konversi float, dan menyertakan kode mata uang bila kolomnya ada.
8. THE ExportWorker SHALL mengurutkan baris berdasarkan `id` naik sehingga ekspor ulang atas data yang sama menghasilkan isi yang sama.
9. WHEN Export_File diunduh, THE ExportRequestAPI SHALL mengirim byte yang sama persis dengan yang ditulis ExportWorker.

### Requirement 6: Konsistensi dan topologi baca/tulis

**User Story:** Sebagai Master_Data_Admin, saya ingin satu file menggambarkan data pada satu saat, supaya tidak ada baris yang hilang atau ganda saat data berubah di tengah ekspor.

#### Acceptance Criteria

1. THE ExportWorker SHALL membaca seluruh data satu Export_File dalam satu transaksi `REPEATABLE READ READ ONLY`, per halaman keyset dengan ukuran halaman terbatas.
2. THE ExportWorker SHALL membaca data master dari pool replica `backend-exq` (primary bila `DATABASE_REPLICA_URL` kosong) dan tidak menulis apa pun ke tabel master data.
3. THE ExportWorker dan ExportRequestAPI SHALL menulis `export_jobs` dan `export_job_chunks` hanya ke primary, dan membaca status serta isi file untuk user dari primary (read-after-write, tidak terkena lag replica).
4. THE ExportWorker SHALL menyimpan Export_File per potongan berukuran terbatas sehingga memori proses tidak tumbuh sebanding ukuran file, baik saat menulis maupun saat diunduh.

### Requirement 7: Otorisasi

**User Story:** Sebagai pemilik sistem, saya ingin ekspor hanya bisa diminta admin master data dan file hanya bisa diunduh peminta, karena file berisi data pribadi PIC dan harga kontrak.

#### Acceptance Criteria

1. THE ExportRequestAPI SHALL dipasang di belakang `RequireAuth` (validasi token dengan `JWT_SECRET` dan blacklist Redis yang sama dengan `backend/`); endpoint permintaan ekspor master data (Export_Kind `vendor`, `cabang`, `harga_paket`, `master_data_global`) dan TemplateAPI SHALL ditambah `RequireRoles("ADMIN","ADMIN_PARAM")`, sedangkan endpoint daftar/status/unduh Export_Job cukup `RequireAuth` + aturan kepemilikan (kriteria 5) + cek peran per Export_Kind (kriteria 6). Hak per Export_Kind didefinisikan di satu registry supaya Export_Kind lain (spec `report-export-migration`) bisa memakai peran sendiri tanpa membuka ekspor master data (D23).
2. THE layer service `backend-exq` SHALL memeriksa ulang peran pemanggil, terlepas dari guard route.
3. IF pemanggil bukan Master_Data_Admin dan meminta ekspor master data atau template, THEN THE ExportRequestAPI SHALL menolak dengan 403.
4. IF pemanggil tidak terautentikasi atau tokennya sudah di-blacklist, THEN THE ExportRequestAPI SHALL menolak dengan 401.
5. THE ExportRequestAPI SHALL hanya menampilkan dan mengizinkan unduh Export_Job milik pemanggil sendiri (D19); IF pemanggil meminta Export_Job milik user lain, THEN THE ExportRequestAPI SHALL menjawab 404.
6. WHEN Export_File diunduh, THE ExportRequestAPI SHALL memeriksa ulang dari database bahwa pemanggil masih aktif dan perannya masih termasuk peran yang diizinkan untuk Export_Kind job tersebut (master data = ADMIN/ADMIN_PARAM), sehingga user yang perannya dicabut setelah meminta ekspor tidak bisa mengunduh.

### Requirement 8: Audit ekspor

**User Story:** Sebagai auditor internal, saya ingin setiap permintaan dan setiap unduhan ekspor mencatat user, data vendor/cabang mana, dan kapan, karena file berisi data pribadi pihak ketiga dan harga kontrak.

#### Acceptance Criteria

1. WHEN Export_Job dibuat, THE ExqAuditWriter SHALL menulis satu baris `audit_logs` di primary dengan action `export_requested`, dalam transaksi yang sama dengan pembuatan Export_Job (gagal audit = Export_Job tidak dibuat, jawab 500).
2. WHEN Export_File diunduh, THE ExqAuditWriter SHALL menulis satu baris `audit_logs` di primary dengan action `master_data_exported` **sebelum** byte pertama dikirim; IF penulisan audit gagal, THEN THE ExportRequestAPI SHALL menjawab 500 tanpa mengirim isi file.
3. THE kedua baris audit SHALL memakai `entity_type` = `vendor` (Level_Vendor, Harga Paket tanpa cabang), `vendor_branch` (Level_Cabang, Harga Paket dengan cabang), atau `master_data` (master data global, `entity_id` = 0), dan `entity_id` = id vendor atau id cabang.
4. THE kedua baris audit SHALL mencatat user: `actor_id` = id user dari JWT pemanggil (tidak pernah 0 atau kosong), dan di kolom `after` disertakan `username` serta `role` dari JWT saat itu.
5. THE kedua baris audit SHALL mencatat IP klien (hasil `TrustedRealIP` atas `r.RemoteAddr`, bukan dari header di handler) dan waktu (`created_at`, timestamptz).
6. THE kedua baris audit SHALL mencatat di kolom `after`: id Export_Job, Export_Kind, Export_Entity, `vendor_id`, `vendor_code`, `branch_id`, `branch_code` (kosong bila tidak ada), Status_Filter, dan filter tingkat harga (Harga Paket); baris `master_data_exported` juga mencatat jumlah baris data di file.
7. IF identitas user pemanggil tidak bisa dibaca dari konteks request, THEN THE ExportRequestAPI SHALL menolak (401) tanpa menulis audit, tanpa membuat Export_Job, dan tanpa mengirim file.
8. WHEN permintaan ditolak (400, 401, 403, 404, 409, 429), THE ExqAuditWriter SHALL tidak menulis baris audit.
9. WHEN auditor memfilter viewer audit log yang ada (`/api/v1/audit-logs`, `features/audit-log`) dengan action `export_requested` / `master_data_exported` atau `actor_id` user tertentu, THE baris ekspor SHALL muncul dengan `actor_id`, entitas, IP, waktu, dan detail `after`, tanpa perubahan pada viewer.
10. THE ExqAuditWriter SHALL hanya menyisipkan baris (tanpa update/delete), sesuai sifat append-only `audit_logs`.
11. THE unduhan template kosong (TemplateAPI) SHALL tidak diaudit karena tidak berisi data (D16).

### Requirement 9: Siklus Export_Job (asinkron)

**User Story:** Sebagai Master_Data_Admin, saya ingin meminta ekspor lalu mengunduhnya setelah siap, supaya ekspor besar tidak gagal karena batas waktu dan saya tidak perlu menunggu di halaman.

#### Acceptance Criteria

1. WHEN permintaan ekspor valid, THE ExportRequestAPI SHALL membuat Export_Job berstatus `queued` dan menjawab `202 Accepted` dengan id, status, dan nama file, tanpa menunggu file selesai dibuat.
2. THE Export_Job SHALL memakai status teknis `queued` → `running` → `succeeded` | `failed`; dari `succeeded` → `downloading` → `downloaded` (unduh berhasil) atau kembali ke `succeeded` (unduh gagal); dari `succeeded` → `expired` bila tidak pernah diunduh sampai batas simpan (D18c). THE sistem SHALL menolak transisi lain.
3. THE Export_Job SHALL menyimpan peminta (`requested_by`), Export_Kind, Export_Entity, parameter (vendor, cabang, Status_Filter, tingkat harga), nama file, waktu dibuat / mulai / selesai / diunduh / kedaluwarsa, jumlah baris data, ukuran file, dan pesan error ringkas bila gagal.
4. WHEN user yang sama meminta ekspor dengan parameter identik selagi Export_Job sebelumnya masih `queued` atau `running`, THE ExportRequestAPI SHALL mengembalikan Export_Job yang sudah ada (`200`) dan tidak membuat yang baru.
5. IF user sudah punya 3 Export_Job berstatus `queued` atau `running` (D21), THEN THE ExportRequestAPI SHALL menolak permintaan baru dengan 429 dan pesan yang menjelaskan batasnya.
6. THE ExportWorker SHALL mengambil Export_Job `queued` dengan penguncian baris (`FOR UPDATE SKIP LOCKED`), sehingga satu Export_Job tidak pernah diproses dua worker sekaligus, termasuk bila `backend-exq` berjalan lebih dari satu instance.
7. WHEN pembuatan Export_File berhasil, THE ExportWorker SHALL menandai Export_Job `succeeded` hanya setelah seluruh potongan file tersimpan, dalam satu transaksi dengan pencatatan jumlah baris dan ukuran file.
8. IF pembuatan Export_File gagal, THEN THE ExportWorker SHALL menandai Export_Job `failed`, menghapus potongan yang sudah tertulis, dan menyimpan pesan error ringkas tanpa isi data atau detail teknis internal.
9. IF Export_Job berstatus `running` lebih lama dari batas waktu proses (10 menit), THEN THE ExportWorker SHALL menandainya `failed` dengan pesan "melewati batas waktu", termasuk Export_Job yang tertinggal saat `backend-exq` dimatikan atau crash.
10. WHEN `backend-exq` dimatikan dengan sinyal normal, THE ExportWorker SHALL berhenti mengambil Export_Job baru dan menyelesaikan atau melepas Export_Job yang sedang berjalan sebelum proses berhenti.
11. WHILE Export_Job berstatus selain `succeeded`, THE ExportRequestAPI SHALL menolak unduh dengan 409 (`queued`, `running`, `failed`, `downloading`) atau 410 (`downloaded`, `expired`), tanpa menulis audit.
12. THE ExportWorker SHALL mencatat ke log terstruktur setiap Export_Job yang mulai, berhasil, dan gagal dengan id, Export_Kind, entitas, durasi, jumlah baris, dan error, tanpa isi data.
13. IF error terjadi setelah byte pertama unduhan terkirim, THEN THE ExportRequestAPI SHALL memutus koneksi (`http.ErrAbortHandler`) sehingga klien menerima unduhan gagal, bukan file terpotong yang tampak lengkap.
14. IF error terjadi sebelum byte pertama dikirim, THEN THE ExportRequestAPI SHALL menjawab JSON error berformat `pkg/response`.

### Requirement 10: Penyimpanan dan masa simpan file

**User Story:** Sebagai pemilik data, saya ingin file ekspor yang berisi data pribadi tidak tersimpan lebih lama dari perlu, supaya risiko kebocoran kecil.

#### Acceptance Criteria

1. THE ExportWorker SHALL menyimpan Export_File di tabel `export_job_chunks` (potongan `bytea` berurutan per Export_Job) di database primary (D17), tanpa library atau layanan penyimpanan baru.
2. WHEN unduhan dimulai, THE ExportRequestAPI SHALL mengubah Export_Job `succeeded` menjadi `downloading` secara atomik, sehingga hanya satu unduhan yang berjalan per Export_Job; permintaan unduh kedua selama itu mendapat 409.
3. WHEN seluruh byte Export_File selesai dikirim ke klien tanpa error tulis, THE ExportRequestAPI SHALL menghapus semua potongan file Export_Job tersebut dan menandainya `downloaded` (beserta waktu unduh) dalam satu transaksi (D18). Baris `export_jobs` tetap disimpan sebagai riwayat.
4. IF unduhan gagal atau terputus sebelum seluruh byte terkirim, THEN THE ExportRequestAPI SHALL mengembalikan Export_Job ke `succeeded` dengan potongan file utuh, sehingga user bisa mengunduh ulang.
5. IF Export_Job tertahan di `downloading` lebih lama dari batas waktu unduh (5 menit, mis. karena proses mati di tengah unduhan), THEN THE ExportWorker SHALL mengembalikannya ke `succeeded`.
6. IF Export_Job `succeeded` tidak diunduh dalam 10 menit setelah file selesai dibuat (D18c), THEN THE ExportWorker SHALL menghapus potongan filenya dan menandainya `expired`, sehingga data pribadi tidak tersimpan tanpa batas. Export_Job berstatus `downloading` tidak dikedaluwarsakan selama unduhannya masih berjalan; bila dikembalikan ke `succeeded` (kriteria 5) setelah lewat 10 menit, ia langsung kedaluwarsa.
7. THE ExportWorker SHALL memeriksa Export_Job yang perlu dikedaluwarsakan atau dipulihkan paling lambat setiap 1 menit, sehingga file tidak tersimpan jauh melewati batas 10 menit.
8. THE ExportWorker SHALL menghapus potongan file milik Export_Job `failed` saat penandaan gagal (Requirement 9.8).
9. THE "berhasil diunduh" SHALL berarti server selesai menulis seluruh byte ke koneksi tanpa error; sistem tidak bisa memastikan browser menyimpan file, jadi bila file hilang di sisi user, user meminta ekspor baru.
10. THE ExportRequestAPI SHALL mengirim Export_File dengan membaca potongan secara berurutan dan mengalirkannya ke klien, tanpa memuat seluruh file ke memori.
11. IF satu Export_File melebihi batas ukuran (100 MB) atau batas jumlah baris (1.000.000 baris), THEN THE ExportWorker SHALL menandai Export_Job `failed` dengan pesan yang menyebut batasnya.

### Requirement 11: Kesiapan backend-exq

**User Story:** Sebagai tim platform, saya ingin `backend-exq` siap dipakai dengan aman saat menerima konten pertamanya, supaya endpoint ekspor tidak lebih lemah dari `backend/`.

#### Acceptance Criteria

1. THE `backend-exq` SHALL membangun pool primary dan pool replica (replica = primary bila `DATABASE_REPLICA_URL` kosong), dan `/ready` SHALL memeriksa keduanya serta Redis.
2. THE `backend-exq` SHALL memasang `TrustedRealIP(TRUSTED_PROXIES)` sebelum middleware lain, sama dengan `backend/`.
3. THE CompanyPortal (Vite dev proxy dan `nginx.conf`) SHALL meneruskan `/api/v1/exports/` ke `backend-exq` (`:8081`) dan meletakkan aturan itu di atas aturan generik `/api/`.
4. THE image `backend-exq` SHALL berjalan sebagai user non-root dan punya HEALTHCHECK `/health` di Dockerfile.
5. THE `backend-exq` SHALL tidak mengimpor paket apa pun dari `backend/` (tech.md prinsip 9).
6. THE pipeline CI SHALL menjalankan build, test, lint, dan coverage gate untuk `backend-exq` seperti modul Go lainnya.
7. THE ExportWorker SHALL berjalan di dalam proses `backend-exq` dan dapat diatur lewat env (interval polling, jumlah worker paralel), dengan default yang tercatat di `.env.example`.
8. THE migrasi `export_jobs` dan `export_job_chunks` SHALL ditulis di `backend/migrations/` (pemilik tunggal migrasi) dengan nomor bebas berikutnya, dan tabelnya dicatat di CLAUDE.md Sec 3.

### Requirement 12: Menu Export Data di CompanyPortal

**User Story:** Sebagai Master_Data_Admin, saya ingin satu menu Export Data untuk meminta ekspor dan melihat status serta mengunduh hasilnya, supaya saya tahu persis apa yang saya unduh dan kapan siap.

#### Acceptance Criteria

1. THE Export_Data_Page SHALL berada di route `/settings/admin/exports`, dijaga `requireRoles(["ADMIN","ADMIN_PARAM"])`, dan dibuka dari kartu "Export Data" di hub Pengaturan (`SettingsHubPage.tsx`) tanpa item sidebar baru (D12).
2. THE Export_Data_Page SHALL menampilkan menu Data Vendor, menu Harga Paket, dan daftar Ekspor Saya sebagai bagian terpisah.
3. WHEN pengguna memakai menu Data Vendor, THE Export_Data_Page SHALL meminta level (Vendor / Cabang), vendor, cabang (wajib bila level Cabang, hanya cabang milik vendor terpilih), Export_Entity yang berlaku untuk level tersebut, dan Status_Filter (default Aktif).
4. WHEN pengguna memakai menu Harga Paket, THE Export_Data_Page SHALL meminta vendor, cabang (opsional), tingkat harga (default Semua), dan Status_Filter (default Aktif).
5. WHEN pengguna menekan tombol Minta Ekspor dan permintaan diterima, THE Export_Data_Page SHALL menampilkan konfirmasi bahwa ekspor sedang diproses dan menambahkan Export_Job ke daftar Ekspor Saya.
6. THE daftar Ekspor Saya SHALL menampilkan nama file, jenis, waktu diminta, status (badge ikon + teks, bukan warna saja: Antri, Diproses, Siap diunduh, Sedang diunduh, Sudah diunduh, Gagal, Kedaluwarsa), jumlah baris, batas waktu unduh untuk status Siap diunduh, waktu diunduh, dan pesan error bila gagal, urut terbaru di atas.
7. WHILE ada Export_Job `queued` atau `running` milik user, THE Export_Data_Page SHALL memperbarui status secara berkala dan berhenti memperbarui saat tidak ada lagi Export_Job aktif.
8. WHEN permintaan ekspor diterima, THE Export_Progress_Dialog SHALL langsung terbuka dan menampilkan tahap proses dengan indikator memuat dan teks: Antri, Membuat file (dengan jumlah baris bila tersedia), Mengunduh, Selesai (D18b).
9. WHILE Export_Progress_Dialog terbuka, THE Export_Progress_Dialog SHALL memantau status Export_Job secara berkala, dan WHEN status menjadi `succeeded`, SHALL langsung memulai unduhan otomatis tanpa klik tambahan (D18a) lewat klien `api` bersama (refresh token 401 tetap berlaku, nama file dari `Content-Disposition`).
10. WHEN unduhan selesai, THE Export_Progress_Dialog SHALL menampilkan "Selesai" dengan nama file dan jumlah baris, lalu bisa ditutup; WHEN Export_Job `failed`, THE Export_Progress_Dialog SHALL menampilkan pesan error (ikon + teks) dan aksi "Coba lagi".
11. WHILE Export_Progress_Dialog menunggu file atau unduhan berjalan, THE CompanyPortal SHALL memperingatkan user lewat dialog konfirmasi browser bila user mencoba menutup atau memuat ulang tab.
12. THE Export_Progress_Dialog SHALL punya tombol "Tutup" yang selalu tersedia; WHEN user menutupnya sebelum unduhan selesai, THE Export_Progress_Dialog SHALL menjelaskan bahwa file tetap bisa diunduh manual dari daftar Ekspor Saya selama 10 menit setelah selesai dibuat (D18c), dan proses ekspor tetap berjalan.
13. THE CompanyPortal SHALL memulai unduhan otomatis paling banyak satu kali per Export_Job, sehingga satu file tidak diunduh ganda.
14. IF unduhan otomatis tidak bisa berjalan atau gagal (dialog ditutup, tab ditutup, sesi habis, jaringan putus, atau browser memblokir), THEN THE Export_Job SHALL tetap `succeeded` sampai kedaluwarsa dan THE daftar Ekspor Saya SHALL menampilkan tombol Unduh manual dengan sisa waktu sebelum kedaluwarsa dan keterangan bahwa file hanya bisa diunduh satu kali.
15. THE Export_Progress_Dialog SHALL memakai `role="dialog"` + `aria-modal="true"`, menjebak fokus selama terbuka, mengembalikan fokus ke tombol pemicu saat ditutup, mengumumkan setiap perubahan tahap lewat live region, tanpa `backdrop-blur` (ui_design.md Sec 10), dan menghormati `prefers-reduced-motion` pada indikator memuat.
16. IF permintaan ekspor ditolak karena batas Export_Job aktif (429), THEN THE Export_Data_Page SHALL menampilkan pesan dengan ikon dan teks yang menjelaskan batasnya, tanpa membuka Export_Progress_Dialog.
17. WHEN Export_Job berstatus `downloaded` atau `expired`, THE Export_Data_Page SHALL tidak menampilkan tombol Unduh dan menawarkan aksi "Minta ekspor lagi" dengan parameter yang sama.
18. THE Export_Data_Page SHALL bisa dioperasikan penuh dengan keyboard, punya label pada setiap input, dan mengikuti token `ui_design.md` (satu tombol primer per menu).

### Requirement 13: Non-regresi

**User Story:** Sebagai pengembang, saya ingin fitur ini tidak mengubah isi file dan perilaku import master data yang sudah ada, supaya file dan alur yang sudah dipakai admin tetap jalan.

#### Acceptance Criteria

1. THE MasterDataExporter SHALL menghasilkan isi CSV (header, urutan kolom, format sel, BOM, `CSVSafe`, urutan baris, makna Status_Filter) yang byte-identik dengan export lama `backend/` untuk data yang sama.
2. THE sistem SHALL mempertahankan perilaku import master data (`dry-run`, `confirm`, validasi header termasuk header lama/legacy) tanpa perubahan.
3. WHEN Export_File dari Export_Entity `vendor`, `branches`, `vaults`, `pics`, atau dari export master data global (6 entitas) diunggah tanpa diedit ke dry-run import entitas yang sesuai, THE importer SHALL menerima header-nya dan melaporkan seluruh baris sebagai `unchanged`.
4. THE implementasi SHALL tidak menambah library baru ke workspace; satu-satunya penambahan adalah `kin-openapi` di `go.mod` `backend-exq` (test only) dengan versi yang sama persis dengan `backend/` (Requirement 15.2).
5. THE implementasi SHALL lolos gate coverage yang berlaku (Go ≥80% `internal/*` termasuk `backend-exq` dan `pkg/`, CompanyPortal 84/80/80/85).

### Requirement 14: Pindah export master data global ke backend-exq

**User Story:** Sebagai pemilik sistem, saya ingin semua kegiatan export data berjalan di `backend-exq`, supaya `backend/` fokus ke transaksi dan export tidak membebani API transaksional.

#### Acceptance Criteria

1. THE ExportRequestAPI SHALL menerima permintaan Export_Kind `master_data_global` untuk entitas `vendors`, `vendor-branches`, `vendor-vaults`, `vendor-pics`, `atms`, dan `atm-assignments` dengan Status_Filter, dan memprosesnya lewat Export_Job seperti ekspor lain (D7).
2. THE TemplateAPI SHALL menyediakan `GET /api/v1/exports/master-data/{entity}/template` secara sinkron, mengirim BOM + baris header saja tanpa akses database, untuk 6 entitas yang sama.
3. THE CSV_Contract SHALL menjadi satu-satunya sumber header, format sel per entitas, BOM, dan `CSVSafe` untuk MasterDataExporter, VendorExporter, TemplateAPI, dan importer `backend/`; tidak ada salinan header atau aturan `CSVSafe` di modul lain.
4. THE importer `backend/` SHALL memakai CSV_Contract untuk validasi header dan untuk memformat data saat ini sebelum dibandingkan dengan file, sehingga format sel export dan pembanding import tidak bisa berbeda.
5. WHEN CompanyPortal sudah memakai `backend-exq` untuk export dan template, THE `backend/` SHALL tidak lagi memasang `/api/v1/admin/master-data/export/*`, dalam rilis yang sama (D15); request ke path lama mendapat 404.
6. THE halaman Import/Export master data (`features/master-data-io`) SHALL meminta export lewat ExportRequestAPI, memakai Export_Progress_Dialog dan unduh otomatis yang sama dengan Requirement 12.8-12.15, dan menampilkan tautan ke daftar Ekspor Saya, mengunduh template dari TemplateAPI, sementara dry-run dan confirm import tetap ke `backend/`.
7. THE export master data global SHALL menulis audit sesuai Requirement 8 (D16).

### Requirement 15: Kontrak OpenAPI backend-exq

**User Story:** Sebagai pengembang, saya ingin endpoint `backend-exq` punya kontrak OpenAPI sejak awal, supaya frontend dan backend tidak berbeda paham soal parameter, status job, dan error (D8, tech.md prinsip 6).

#### Acceptance Criteria

1. THE `backend-exq` SHALL punya kontrak OpenAPI 3.0.3 tulis tangan di `backend-exq/api/openapi.yaml` yang mencakup semua endpoint ExportRequestAPI dan TemplateAPI: path, parameter, skema Export_Job, respons `text/csv`, dan respons error `pkg/response`.
2. THE `backend-exq` SHALL punya test kontrak (kin-openapi, versi sama dengan `backend/`) yang gagal bila route terpasang tidak cocok dengan kontrak atau sebaliknya.
3. THE CompanyPortal SHALL membuat tipe TypeScript dari kontrak tersebut lewat script pnpm terpisah dari `gen:api` yang ada (output terpisah dari `src/lib/api/schema.d.ts`), dan file hasilnya di-commit.
4. THE CompanyPortal SHALL punya test vitest yang gagal bila file tipe hasil generate sudah basi terhadap `backend-exq/api/openapi.yaml`, sama dengan pola `schema.d.ts`.

## Open Questions

| # | Pertanyaan | Keputusan / saran | Status |
| --- | --- | --- | --- |
| D1 | Format: ZIP bundle atau satu CSV per entitas? | Satu CSV per entitas yang dipilih. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D2 | Pilihan ekspor ATM per vendor/cabang? | Tidak. Scope fokus ke data vendor dulu. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D3 | Level cabang: PIC vendor-wide ikut? | Tidak, hanya PIC cabang. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D4 | Harga paket | Menu ekspor terpisah (Requirement 3). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D4a | Harga paket mencakup Paket Cabang? | Hanya `vendor_package_prices`. Paket Cabang tidak diekspor. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D5 | Ekspor ditulis ke `audit_logs`? | Ya, wajib mencatat user (id, username, role), IP, waktu, dan cakupan (Requirement 8). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D6 | Peran yang boleh ekspor | `ADMIN`, `ADMIN_PARAM`. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D7 | Sinkron atau asinkron? | Asinkron (`export_jobs`), supaya ekspor data besar tidak perlu diubah lagi nanti. Template kosong tetap sinkron. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D8 | Endpoint masuk kontrak OpenAPI? | Ya, `backend-exq/api/openapi.yaml` + test kontrak + tipe TS terpisah (Requirement 15). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D9 | Masuk `development-plan.md` sebagai apa? | Scope baru Export Data (item X.7). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D10 | Backend tujuan | `backend-exq`. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D11 | `CSVSafe` + header CSV master data | Pindah ke `pkg/` (CSV_Contract) karena export di `backend-exq` dan import di `backend/` harus memakai satu kontrak. | ✅ mengikuti D13 (2026-10-28) |
| D12 | Lokasi menu di CompanyPortal | Kartu "Export Data" di hub Pengaturan (`/settings/admin/exports`). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D13 | Lokasi semua kegiatan export data | Semua export data di `backend-exq`, termasuk yang sudah ada di `backend/`. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D14 | Pembagian scope | 2 spec. Spec ini = export vendor baru + pindah export master data global (Requirement 14). CSV laporan DSR telat dan replenishment di `report-export-migration`. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D15 | Cutover endpoint lama | Dihapus di rilis yang sama dengan frontend pindah. Rollback = deploy ulang image sebelumnya. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D16 | Export master data global ikut diaudit? | Ya, sama dengan Requirement 8. Template kosong tidak diaudit. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D17 | Tempat menyimpan file hasil ekspor | Tabel `export_job_chunks` di Postgres (potongan `bytea`). Tanpa library baru, tetap jalan saat backend dipisah VM (2028), ikut enkripsi at-rest CloudSQL. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D18 | Masa simpan file | File dihapus dari server segera setelah user berhasil mengunduh (server selesai mengirim seluruh byte). Satu file = satu kali unduh; bila gagal di tengah, file tetap ada dan bisa diunduh ulang (Requirement 10.2-10.5, 10.9). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D18a | Setelah file terbentuk, bagaimana sampai ke user? | Unduh otomatis begitu file siap, tanpa klik tambahan (Requirement 12.9). Tombol Unduh manual tetap ada sebagai cadangan (12.14). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D18b | Bagaimana user menunggu sampai file terunduh? | Pop-up loading (Export_Progress_Dialog) yang menahan user di halaman sampai unduhan selesai, plus peringatan browser bila tab ditutup/dimuat ulang (Requirement 12.8-12.15). Tombol Tutup tetap ada (aksesibilitas); proses tetap jalan dan file bisa diunduh manual dari Ekspor Saya. Tidak ada pemantau di halaman lain. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D18c | File yang tidak diunduh | Kedaluwarsa dan dihapus 10 menit setelah file selesai dibuat (Requirement 10.6, 10.7). | ✅ DIPUTUSKAN PO 2026-10-28 |
| D19 | Siapa yang boleh melihat dan mengunduh hasil ekspor | Hanya user peminta. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D20 | Notifikasi saat ekspor selesai | Tidak di scope ini. Status terlihat di daftar Ekspor Saya yang diperbarui otomatis. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D21 | Batas permintaan | Maksimal 3 Export_Job aktif per user; permintaan identik saat masih diproses mengembalikan job yang sama; batas file 100 MB / 1.000.000 baris; batas waktu proses 10 menit. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D22 | Halaman Import/Export master data ikut asinkron? | Ya (Requirement 14.6). Template tetap unduh langsung. | ✅ DIPUTUSKAN PO 2026-10-28 |
| D23 | Otorisasi `/api/v1/exports` per Export_Kind (dari `report-export-migration` D9) | Guard peran hanya pada endpoint permintaan; `/jobs*` = `RequireAuth` + kepemilikan + cek peran per Kind saat unduh. Perilaku untuk ADMIN/ADMIN_PARAM tidak berubah (Requirement 7.1, 7.3, 7.6). | ✅ mengikuti saran (PO 2026-10-28) |
