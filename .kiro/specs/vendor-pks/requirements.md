# Requirements Document

> **Status: disetujui PO 2026-11-05.** Dipindahkan dari `.claude/sdlc/vendor-pks/intent.md` (draft revisi 2026-11-05; draft pertama 2026-10-05 sebagai `vendor-pks-cis-limit`). Keputusan bisnis PO dari intent (R1–R5, A1, A4, Q1–Q8, O1–O8) dibawa sebagai **D1–D19**. Saran Kiro untuk hal teknis **D20–D31** disetujui PO 2026-11-05 ("setuju dengan saran kiro"). Saat design ditulis (2026-11-05), criteria 3.8, 8.6, dan 12.1 disesuaikan dengan kode yang ada (lihat catatan di bawah tabel saran Kiro). **Revisi 2026-11-05 (D32, PO):** Direktur Utama dicatat sebagai PIC vendor bertanda `vendor_pics.is_director`, bukan kolom baru di `vendors`; Req 1 dan 2 ditulis ulang, D5 digantikan, D10 diubah.

## Introduction

### Masalah

FSD "DSR End To End Cash Management" v1.0 (1-Sept-2026), bab "Management vendor dan master data" (`DSR End To End Cash Management V01.docx` di repo root), meminta menu Master Data Vendor Inquiry untuk memantau masa aktif PKS (Perjanjian Kerja Sama, kontrak bank dengan vendor). Sistem menghitung durasi expired PKS setiap kali menu dibuka, memberi tanda per vendor (masih lama, mendekati expired, expired), dan tim OPS bisa menandai PKS vendor yang diputus kontraknya sebagai Terminated secara manual berdasarkan Nama Perusahaan. Field Master Vendor di FSD juga memuat Nama Direktur Utama dan kontaknya.

CROWN belum menyimpan data PKS maupun data direktur, sehingga tim OPS memantau masa kontrak vendor di luar sistem.

Desain layar dari user (2026-11-05): `.document/master_data/desain-dari-user_vendor/Field-Master-data-vendor-pks.xlsx`.
- Sheet **PKS**: No, Nama Perusahaan, PKS Efektif (judul dokumen, mis. *Perubahan dan Pernyataan Kembali Perjanjian Induk Kerjasama Jasa Layanan Cash Replenishment, Cash Collection, dan First Line Maintenance ATM, CDM, TST*), Tanggal Efektif PKS, Tanggal Expired PKS, Masa Berlaku PKS, Status PKS, Monitoring PKS (sisa hari sampai expired), state (green / amber / red).
- Sheet **Master inquiry**: cari per Nama vendor + status expired PKS; kolom Nama Perusahaan, Nama Dirut, Contact Dirut, Service Type, Nama PKS, tanggal efektif, tanggal expired, Status (Aktif / Need Renewal / Expired); tombol "Upload Report" dan "generate report".
- Sheet **Detail inquiry** (Limit CIS / asuransi per area) dan **Menu Input Parameter** (TBA) bukan bagian fitur ini (D19).

### Kondisi kode saat ini (dicek 2026-11-05)

**Tabel `vendors`** (`001_baseline_schema.sql` + `003_master_data.sql` + `009_vendor_package_prices.sql`)
- Kolom: `id, code, name, contact_email, contact_phone, hq_address, is_active, created_at, updated_at, deleted_at, request_prefix, legal_name, npwp, kind`.
- `kind` CHECK `IN ('FLM_VENDOR', 'INTERNAL')`, default `FLM_VENDOR`.
- Tidak ada kolom direktur. Tidak ada tabel PKS/kontrak di skema.

**Tabel `vendor_pics`** (`003_master_data.sql` + `032_vendor_party_scope.sql`)
- Kolom: `id, vendor_id, vendor_branch_id (NULL = PIC tingkat vendor), name, position, phone, email, is_notification_recipient, is_active, created_at, updated_at, deleted_at, roles text[]`.
- `roles` ⊆ `{replenish, vault, cashcount}`; kosong = kontak saja. Create tanpa `roles` mendapat ketiganya (`AllPicRoles`).
- PIC adalah kontak, bukan akun: tidak ada relasi ke `users` dan tidak bisa login.
- Create/update/disable/enable PIC lewat maker-checker (`VendorPicAdminService` → `VendorPicApplier`). Ubah `roles` lewat layar berlaku langsung + audit (`VendorPicRoleService.SetRoles`, deviasi GR#3 dari `vendor-party-scope`).
- Penerima notifikasi dipilih dari `is_notification_recipient = true` (+ `roles` untuk notifikasi per cabang) di `backend/queries/notifications.sql` dan `backend_python/dsr_late_notify/queries.py`.

**Maker-checker master data**
- `/api/v1/admin/vendors` (`backend/internal/handler/admin_vendor_handler.go`): `GET /`, `POST /`, `GET /{id}`, `PUT /{id}`, `POST /{id}/disable`, `POST /{id}/enable`. List memakai `page`, `page_size`, `status`, `q`, dan mengembalikan `{vendors, page, page_size, total}` (JSON datar, ATM backend).
- Mount lewat `masterDataAdmin` di `backend/cmd/api/main.go` (`RequireAuth` + `RequireRoles("ADMIN", "ADMIN_PARAM")`); role yang sama dicek ulang di `MasterDataChangeService.Submit`.
- Write = stage change → `approval_requests` (`document_type = master_data`) → 202 → `Applier` dari `masterDataApplierRegistry` dijalankan saat approval final. Registry sekarang: `vendor, vendor_branch, vendor_vault, vendor_pic, atm, vendor_package, vendor_package_price, atm_assignment, dsr_location_map`.
- `ErrMasterDataChangePending` (409) menolak change kedua untuk entitas yang sama selama masih pending. `ErrMasterDataChangeStale` menandai change `stale` dan tidak diterapkan bila entitas berubah antara submit dan approve.
- Repo vendor dibangun dengan `dbPool` (primary) + `dbReadPool` (replica).

**CSV master data**
- Header vendor di `pkg/csvexport/entities.go` dan `masterdata_export_test.go`: `id, code, name, legal_name, npwp, contact_email, contact_phone, hq_address, is_active`.
- Import vendor (`masterdata_import_confirm.go`) memanggil `Vendors.Create/Update` dengan field yang sama.
- Header PIC: `id, vendor_code, branch_code, name, position, phone, email, is_notification_recipient, is_active, roles`, dengan legacy header tanpa `roles`.

**Ekspor asinkron** (`backend-exq`)
- `export_jobs` (mig `034`, `035`), `KindSpecs` di `backend-exq/internal/exportjob/kinds.go` (kind master data dibatasi `ADMIN/ADMIN_PARAM`), Report_Kind laporan di `pkg/reports/reports.go`.

**Lain-lain**
- Tabel `documents` ada di peta tabel, tapi modul dokumen (Phase 0.4) ditunda ke Phase 3/5.
- `backend/api/openapi.yaml` belum memuat `/admin/vendors`.
- Layar: `frontend/CompanyPortal-Vite/src/features/admin-vendors/`.
- Migrasi terakhir `036_master_data_import_batches_file_name.sql`. Nomor **`022`** belum terpakai dan dicadangkan untuk fitur ini (`development-plan.md`).

### Ruang lingkup

- Direktur Utama sebagai PIC tingkat vendor bertanda `is_director` (hanya pencatatan: tanpa login, tanpa roles, bukan penerima notifikasi), termasuk CSV PIC (D32).
- Tabel PKS per vendor dengan riwayat, satu PKS aktif per vendor (D1–D3).
- Catat PKS baru, terminate, dan batalkan terminate lewat maker-checker master data (D3, D7, D17).
- Status PKS, masa berlaku, dan sisa hari dihitung saat dibaca (D6, D14, D15, D18).
- Daftar vendor dengan kolom PKS dan filter status server-side (D11).
- Ekspor CSV daftar vendor + PKS sesuai filter lewat `backend-exq` (D16).

### Tidak termasuk

- Limit CIS, nomor escrow per vault, field sheet Detail inquiry, monitoring Limit CIS → fitur `.claude/sdlc/vendor-cis-limit/` (D19).
- Service Type per vendor (D13). Kategori vault (`vendor_vaults.category`) tidak berubah.
- Upload file PKS (D4). Menyusul bersama modul dokumen (Phase 0.4), tetap opsional.
- Upload massal data PKS (D16).
- Upload master replace-all versi FSD (fitur terpisah, keputusan #3 FSD alignment).
- Notifikasi/email PKS mendekati expired (fitur lanjutan; modul notifikasi 0.3 sudah ada).
- Tautan amandemen ke PKS induk (D3).
- Layar PKS di VendorPortal (D12).
- Role baru (D12).
- Cash count, PIC coordinator/executor, kalender libur.

### Area sensitif (project-context Sec 3 rule 7)

- **Migrasi skema:** tabel baru + satu kolom `vendor_pics.is_director` (default `false`) dengan constraint (`022`). Aditif, tidak mengubah baris yang ada. Tabel `vendors` tidak diubah.
- **Notifikasi:** Director_PIC tidak boleh ikut terpilih sebagai penerima notifikasi DSR telat/replenish.
- **Maker-checker master data:** entitas master data baru (`vendor_pks`) dengan tiga operasi (catat baru yang menutup PKS aktif, terminate, batalkan terminate). Tidak ada deviasi Golden Rule #3.
- **Konsistensi status:** status dihitung, tidak disimpan. Filter, layar, dan CSV harus memakai aturan yang sama.

## Glossary

- **PKS**: Perjanjian Kerja Sama, kontrak bank dengan vendor.
- **Vendor_FLM**: baris `vendors` dengan `kind = 'FLM_VENDOR'`.
- **Vendor_Internal**: baris `vendors` dengan `kind = 'INTERNAL'` (ROH). Tidak punya PKS (D8).
- **PKS_Record**: satu baris di tabel PKS (nama final di design, usulan `vendor_pks`): `vendor_id`, `pks_name`, `effective_date`, `expiry_date`, penanda digantikan (`replaced_at`), penanda terminate (`terminated_at`, alasan, pelaku).
- **Active_PKS**: PKS_Record milik satu vendor yang belum digantikan. Paling banyak satu per vendor (D2).
- **Replaced_PKS**: PKS_Record yang sudah digantikan oleh PKS_Record lebih baru. Masuk riwayat, tidak bisa diubah.
- **Terminated_PKS**: Active_PKS yang punya penanda terminate.
- **PKS_Change**: master-data change pending untuk PKS satu vendor, salah satu dari `create`, `terminate`, `unterminate`.
- **Today_WIB**: tanggal kalender Asia/Jakarta di server saat request (atau job ekspor) diproses (D30).
- **Renewal_Threshold**: ambang Need Renewal, default 9 bulan kalender, bisa diubah tanpa migrasi (D6, D20).
- **PKS_Status**: satu label hasil hitung: `aktif` (Aktif), `need_renewal` (Need Renewal), `expired` (Expired), `terminated` (Terminated), `belum_diisi` (Belum diisi). Vendor_Internal tidak punya PKS_Status (D29).
- **Masa_Berlaku**: lama PKS dari `effective_date` sampai `expiry_date`, dalam bulan kalender penuh (D15, D21).
- **Monitoring_PKS**: sisa hari dari Today_WIB sampai `expiry_date`, atau jumlah hari lewat bila Expired (D18).
- **Master_Data_Role**: `ADMIN`, `ADMIN_PARAM` (D12).
- **PksStatusRule**: satu aturan hitung PKS_Status, Masa_Berlaku, dan Monitoring_PKS yang dipakai list, detail, filter, dan ekspor.
- **VendorAdminAPI**: `/api/v1/admin/vendors` yang sudah ada (list, detail, create, update).
- **VendorPksAPI**: endpoint baru untuk PKS satu vendor (riwayat, submit PKS baru, terminate, batalkan terminate) di bawah `/api/v1/admin/vendors/{id}`.
- **MasterDataChangeService**: service stage change master data yang sudah ada.
- **Director_PIC**: baris `vendor_pics` aktif dengan `is_director = true`: PIC tingkat vendor (`vendor_branch_id` NULL), `roles` kosong, `is_notification_recipient = false`. Paling banyak satu per vendor. Hanya pencatatan, tidak punya login (D32).
- **VendorPicAPI**: `/api/v1/admin/vendors/{vendorID}/pics` yang sudah ada (list, detail, create, update, disable, enable).
- **VendorPicApplier**: applier `vendor_pic` yang sudah ada.
- **VendorPicRoleService**: service ubah `roles` PIC yang sudah ada (langsung + audit).
- **VendorPksApplier**: applier baru untuk entitas `vendor_pks`.
- **MasterDataCsv**: CSV import/export master data yang sudah ada (`pkg/csvexport`, `masterdata_import*.go`), termasuk entitas PIC.
- **ExportAPI**: `/api/v1/exports/*` di `backend-exq`.
- **VendorAdminScreen**: layar master vendor di `frontend/CompanyPortal-Vite/src/features/admin-vendors/`.
- **AuditWriter**: `internal/audit`, menulis `audit_logs`.

## Requirements

### Requirement 1: Direktur Utama sebagai PIC vendor

**User Story:** Sebagai tim OPS, saya ingin mencatat Direktur Utama vendor sebagai PIC vendor bertanda khusus, supaya data direktur tersimpan di data kontak vendor yang sudah ada tanpa memberi login, peran PIC, atau notifikasi.

#### Acceptance Criteria

1. WHEN a Master_Data_Role user submits a vendor PIC create or update with `is_director = true`, THE VendorPicAPI SHALL stage it through the existing PIC maker-checker flow and return 202.
2. IF a PIC with `is_director = true` has a `vendor_branch_id`, THEN THE VendorPicAPI SHALL reject the request with 422 on field `vendor_branch_id`.
3. IF a PIC with `is_director = true` has `is_notification_recipient = true`, THEN THE VendorPicAPI SHALL reject the request with 422 on field `is_notification_recipient`.
4. WHEN a create or update that sets `is_director = true` is approved, THE VendorPicApplier SHALL store the PIC with an empty `roles` set and `is_notification_recipient = false`.
5. IF the vendor already has another active Director_PIC, THEN THE VendorPicAPI SHALL reject a create, update, or enable that would make a second one with 409; IF two such changes are both approved, THEN THE VendorPicApplier SHALL apply the first and mark the other `stale`.
6. IF a roles change through the VendorPicRoleService would give a Director_PIC a non-empty role set, THEN THE VendorPicRoleService SHALL reject it with 409 and change nothing.
7. WHEN a PIC update omits `is_director`, THE VendorPicAPI SHALL keep the stored flag; WHEN it sends `is_director = false`, THE VendorPicApplier SHALL clear the flag and leave the PIC's roles empty until they are changed.
8. THE system SHALL NOT create a `users` row, credentials, or any login for a Director_PIC; a Director_PIC is a contact record only (D32).
9. THE database SHALL guarantee that every active Director_PIC is vendor-wide, has an empty role set, is not a notification recipient, and is the only active Director_PIC of its vendor, so the existing notification queries never select it.
10. THE VendorPicAPI SHALL return `is_director` in the PIC list and detail responses, and THE VendorAdminAPI SHALL return the Director_PIC `name`, `phone`, and `email` in the vendor list and detail responses (null when there is none).

### Requirement 2: Penanda direktur di CSV PIC

**User Story:** Sebagai tim OPS, saya ingin penanda Direktur Utama ikut di CSV import dan export PIC, supaya data direktur bisa diisi massal dengan alur CSV yang sudah ada.

#### Acceptance Criteria

1. THE MasterDataCsv SHALL add an `is_director` column (`true`/`false`) at the end of the vendor PIC CSV header, in the same order for the import template, the import parser, and the `backend-exq` export (`pkg/csvexport`).
2. WHEN a PIC CSV row has `is_director = true`, THE MasterDataCsv SHALL validate it in the dry-run with the same rules as 1.2, 1.3, and 1.5 (a `roles` cell other than blank or `none` is an error on field `roles`), and SHALL stage it through the same create/update path as Requirement 1 with an empty role set.
3. WHEN a PIC CSV uses one of the previous headers without `is_director`, THE MasterDataCsv SHALL accept it as a legacy header and keep the stored flag of updated rows (new rows get `false`); IF the header matches neither the current nor a legacy header, THEN THE MasterDataCsv SHALL reject the file with its existing header-mismatch error.
4. THE vendor CSV header (`vendors`) SHALL stay unchanged (D32).
5. THE MasterDataCsv SHALL NOT accept PKS data in any CSV import (D16).

### Requirement 3: Catat PKS baru

**User Story:** Sebagai tim OPS, saya ingin mencatat PKS vendor (termasuk perpanjangan dan "Perubahan dan Pernyataan Kembali") sebagai catatan baru, supaya PKS terbaru langsung dipantau dan PKS lama tetap tersimpan sebagai riwayat.

#### Acceptance Criteria

1. WHEN a Master_Data_Role user submits a PKS for a Vendor_FLM with `pks_name`, `effective_date`, and `expiry_date`, THE VendorPksAPI SHALL stage a PKS_Change of type `create` through MasterDataChangeService and return 202, without changing any PKS_Record.
2. IF `pks_name` is empty after trim or longer than 500 characters, THEN THE VendorPksAPI SHALL reject the request with 422 and name the field.
3. IF `effective_date` or `expiry_date` is missing or not a valid `YYYY-MM-DD` date, or `expiry_date` is before `effective_date`, THEN THE VendorPksAPI SHALL reject the request with 422 and name the field.
4. THE VendorPksAPI SHALL accept an `effective_date` in the past or in the future, and SHALL NOT compare the new dates with the dates of earlier PKS_Records (D23).
5. IF the vendor does not exist, THEN THE VendorPksAPI SHALL reject the request with 404; IF the vendor is a Vendor_Internal, THEN with 422; IF the vendor is disabled (`is_active = false`), THEN with 409 (D29).
6. IF the vendor already has a pending PKS_Change of any type, THEN THE VendorPksAPI SHALL reject the request with 409 (D24).
7. WHEN a `create` PKS_Change is approved, THE VendorPksApplier SHALL, in one database transaction, mark the current Active_PKS (if any) as replaced with the replacement time and new PKS_Record reference, insert the new PKS_Record as the Active_PKS, and write the audit entry (D17).
8. IF, at approval time, the vendor's Active_PKS is not the one recorded when the change was submitted, or the vendor is no longer an active Vendor_FLM, THEN THE VendorPksApplier SHALL mark the change `stale` and apply nothing, following the existing stale rule.
9. WHEN a PKS_Change is rejected, THE VendorPksApplier SHALL leave all PKS_Records unchanged.
10. WHERE the Active_PKS is a Terminated_PKS, THE VendorPksAPI SHALL still accept a new PKS; on approval the Terminated_PKS SHALL move to history with its terminate data kept (D26).

### Requirement 4: Satu PKS aktif dan riwayat PKS

**User Story:** Sebagai tim OPS, saya ingin setiap vendor punya satu PKS aktif dengan riwayat PKS sebelumnya, supaya jelas kontrak mana yang berlaku dan apa yang pernah berlaku.

#### Acceptance Criteria

1. THE database SHALL guarantee at most one Active_PKS per vendor through a constraint, so concurrent approvals cannot leave two Active_PKS for one vendor.
2. THE system SHALL never hard-delete a PKS_Record and SHALL never change `pks_name`, `effective_date`, or `expiry_date` of a stored PKS_Record (D25).
3. WHEN a Master_Data_Role user opens a vendor's PKS, THE VendorPksAPI SHALL return the Active_PKS (or none) and all Replaced_PKS sorted by replacement time, newest first, each with `pks_name`, dates, replacement time, and terminate data.
4. WHILE a PKS_Change is pending for a vendor, THE VendorPksAPI SHALL return it with type, submitted values, maker, and submit time, read from the primary database.
5. THE VendorPksAPI SHALL return Masa_Berlaku for every PKS_Record, and PKS_Status and Monitoring_PKS only for the Active_PKS.

### Requirement 5: Terminate dan batalkan terminate PKS

**User Story:** Sebagai tim OPS, saya ingin menandai PKS vendor yang kontraknya diputus sebagai Terminated dan bisa membatalkannya, supaya status kontrak benar tanpa menonaktifkan vendornya.

#### Acceptance Criteria

1. WHEN a Master_Data_Role user submits a terminate for a vendor's Active_PKS with a reason, THE VendorPksAPI SHALL stage a PKS_Change of type `terminate` and return 202.
2. IF the vendor has no Active_PKS, or the Active_PKS is already a Terminated_PKS, THEN THE VendorPksAPI SHALL reject the terminate with 409.
3. WHEN a `terminate` change is approved, THE VendorPksApplier SHALL set the terminate time, the maker, and the reason on the Active_PKS, and SHALL NOT change `vendors.is_active` (D7).
4. WHEN a Master_Data_Role user submits an un-terminate for a Terminated_PKS that is still the Active_PKS, with a reason, THE VendorPksAPI SHALL stage a PKS_Change of type `unterminate` and return 202; IF the Active_PKS is not terminated, THEN THE VendorPksAPI SHALL reject it with 409.
5. WHEN an `unterminate` change is approved, THE VendorPksApplier SHALL clear the terminate data on the Active_PKS; the earlier terminate and un-terminate SHALL stay visible in `audit_logs`.
6. IF a terminate or un-terminate targets a Replaced_PKS, THEN THE VendorPksAPI SHALL reject it with 409.
7. IF the terminate or un-terminate reason is empty after trim or longer than 500 characters, THEN THE VendorPksAPI SHALL reject the request with 422.

### Requirement 6: Status PKS dihitung saat dibaca

**User Story:** Sebagai tim OPS, saya ingin status PKS dihitung setiap kali daftar dibuka, supaya label selalu sesuai tanggal hari ini tanpa job harian.

#### Acceptance Criteria

1. THE PksStatusRule SHALL compute PKS_Status from the vendor's `kind`, its Active_PKS, and Today_WIB at read time; THE system SHALL NOT store PKS_Status.
2. THE PksStatusRule SHALL apply these rules in order (D22): Vendor_FLM without Active_PKS → `belum_diisi`; Terminated_PKS → `terminated`; `expiry_date` < Today_WIB → `expired`; `expiry_date` < Today_WIB + Renewal_Threshold → `need_renewal`; otherwise → `aktif`.
3. THE PksStatusRule SHALL add Renewal_Threshold as calendar months to Today_WIB, clamped to the last day of the target month (e.g. 2026-05-31 + 9 bulan = 2027-02-28) (D20).
4. WHILE `expiry_date` equals Today_WIB, THE PksStatusRule SHALL treat the PKS as still valid (not `expired`) (D20).
5. THE PksStatusRule SHALL compute the status of an Active_PKS whose `effective_date` is after Today_WIB with the same rules, without a separate "belum berlaku" status (D17).
6. THE PksStatusRule SHALL give no PKS_Status to a Vendor_Internal; such vendors SHALL match no `pks_status` filter value (D8, D29).
7. THE system SHALL read Renewal_Threshold from configuration with default 9 months, so it can change without a migration (D6).
8. THE PksStatusRule SHALL produce the same PKS_Status for the same vendor and Today_WIB in the list, the detail, the `pks_status` filter, and the CSV export.

### Requirement 7: Masa berlaku dan monitoring sisa hari

**User Story:** Sebagai tim OPS, saya ingin melihat lama kontrak dan sisa hari sampai expired, supaya saya tahu kapan harus memulai perpanjangan.

#### Acceptance Criteria

1. THE PksStatusRule SHALL compute Masa_Berlaku as the number of full calendar months from `effective_date` to `expiry_date` + 1 day (D21).
2. THE VendorAdminScreen SHALL display Masa_Berlaku as "N tahun M bulan", omitting a zero part (e.g. 24 bulan → "2 tahun", 18 bulan → "1 tahun 6 bulan", 8 bulan → "8 bulan"), and "< 1 bulan" when Masa_Berlaku is 0 (D21).
3. WHILE PKS_Status is `aktif` or `need_renewal`, THE PksStatusRule SHALL compute Monitoring_PKS as `expiry_date` − Today_WIB in days, and THE VendorAdminScreen SHALL display it as "N hari" (0 on the expiry day).
4. WHILE PKS_Status is `expired`, THE VendorAdminScreen SHALL display Monitoring_PKS as "lewat N hari", where N = Today_WIB − `expiry_date`, never as a negative number (D18).
5. WHILE PKS_Status is `terminated` or `belum_diisi`, or the vendor is a Vendor_Internal, THE VendorAdminScreen SHALL display Monitoring_PKS as "-" (D22).
6. THE VendorAdminAPI and VendorPksAPI SHALL return Masa_Berlaku (months) and Monitoring_PKS (days, signed) as integers, so the screen and the CSV show the same numbers.

### Requirement 8: Daftar vendor dengan kolom PKS dan filter status

**User Story:** Sebagai tim OPS, saya ingin mencari vendor per nama dan per status PKS di daftar vendor, supaya vendor yang perlu perpanjangan langsung terlihat.

#### Acceptance Criteria

1. THE VendorAdminAPI SHALL return, for each vendor in the list, the Director_PIC name, phone, and email, the Active_PKS `pks_name`, `effective_date`, `expiry_date`, Masa_Berlaku, Monitoring_PKS, and PKS_Status (null fields when there is no Active_PKS).
2. WHEN the list request has `pks_status` with one of `aktif | need_renewal | expired | terminated | belum_diisi`, THE VendorAdminAPI SHALL return only vendors with that PKS_Status, combined with the existing `q` and `status` filters, and `total` SHALL count the filtered rows.
3. IF `pks_status` has any other value, THEN THE VendorAdminAPI SHALL reject the request with 400.
4. THE VendorAdminAPI SHALL compute the `pks_status` filter in the database query with the same rules as 6.2–6.4, so paging and `total` stay correct.
5. THE VendorAdminAPI SHALL keep the existing `q` search on Nama Perusahaan (`vendors.name`) unchanged.
6. THE VendorAdminAPI SHALL read the list from the replica pool, and the vendor detail and PKS detail from the primary pool, as the vendor detail does today (read-after-write after submit and approval).

### Requirement 9: Layar vendor (CompanyPortal)

**User Story:** Sebagai tim OPS, saya ingin melihat dan mengelola PKS dari layar vendor yang sudah ada, supaya tidak perlu menu baru.

#### Acceptance Criteria

1. THE VendorAdminScreen SHALL show PKS_Status as a badge with icon and text: Aktif (success), Need Renewal (warning), Expired (danger), Terminated (neutral), Belum diisi (neutral); status SHALL never be shown by color alone (D14).
2. THE VendorAdminScreen SHALL add to the vendor list the columns Nama Direktur Utama, Nama PKS, Tanggal Expired PKS, Monitoring PKS, and Status PKS, and a Status PKS filter with the five labels plus "Semua" (D27).
3. THE VendorAdminScreen SHALL add a "Direktur Utama" checkbox to the vendor-wide PIC form (when checked: no branch, no notification, no roles; the roles action is hidden for that PIC), mark the Director_PIC with a "Direktur Utama" badge in the PIC list, and show the Director_PIC name, phone, and email read-only in the vendor detail; the vendor create/edit form SHALL stay unchanged.
4. THE VendorAdminScreen SHALL show a PKS section in the vendor detail with the Active_PKS (nama, tanggal efektif, tanggal expired, Masa_Berlaku, Monitoring_PKS, status), the PKS history, and the pending PKS_Change if any.
5. THE VendorAdminScreen SHALL offer "Catat PKS baru", "Terminate PKS", and "Batalkan terminate" only when the action is allowed by Requirements 3 and 5, and SHALL disable all three while a PKS_Change is pending.
6. THE VendorAdminScreen SHALL hide the PKS section and PKS actions for a Vendor_Internal and show "-" in its PKS columns.
7. THE VendorAdminScreen SHALL show the 202 result as "menunggu persetujuan" and map 422/404/409 to field or banner messages, following the existing master-data dialogs.
8. THE VendorAdminScreen SHALL be usable by keyboard, label every form field and filter for screen readers, use `tabular-nums` for dates and day counts, and use no em dash in UI copy.

### Requirement 10: Ekspor CSV daftar vendor dan PKS

**User Story:** Sebagai tim OPS, saya ingin mengekspor daftar vendor dan PKS sesuai filter ke CSV ("generate report"), supaya bisa dilaporkan di luar sistem.

#### Acceptance Criteria

1. WHEN a Master_Data_Role user requests a PKS export with the current list filters (`q`, `status`, `pks_status`), THE ExportAPI SHALL create an async `export_jobs` job through the existing export flow.
2. THE ExportAPI SHALL produce a CSV with the same vendors as the list under the same filters, computed with Today_WIB at production time.
3. THE ExportAPI SHALL write the columns Kode Vendor, Nama Perusahaan, Nama Direktur Utama, Telepon Direktur Utama, Email Direktur Utama, Nama PKS, Tanggal Efektif PKS, Tanggal Expired PKS, Masa Berlaku (bulan), Monitoring PKS (hari), Status PKS, with every text cell through `CSVSafe` (D27).
4. IF the requester does not hold a Master_Data_Role, THEN THE ExportAPI SHALL reject the request with 403.
5. THE ExportAPI SHALL audit the request and the download with the existing export audit actions.
6. THE ExportAPI SHALL read export data from the replica pool.

### Requirement 11: RBAC, audit, dan topologi DB

**User Story:** Sebagai pemilik sistem, saya ingin setiap perubahan direktur dan PKS terkendali dan tercatat, supaya data kontrak vendor bisa diaudit.

#### Acceptance Criteria

1. THE VendorPksAPI SHALL be mounted behind the existing `masterDataAdmin` guard (Master_Data_Role), and THE MasterDataChangeService SHALL re-check the role on every PKS_Change submit.
2. IF a user without a Master_Data_Role calls any VendorPksAPI endpoint, THEN THE VendorPksAPI SHALL respond 403 and stage nothing.
3. WHEN a PKS_Change or a Director_PIC change is submitted, approved and applied, or rejected, THE AuditWriter SHALL write an `audit_logs` row (actor, action, before, after, IP) in the same transaction as the state change; IF the audit write fails, THEN the change SHALL roll back.
4. THE approval of a PKS_Change SHALL go through the existing master-data approval flow, so the checker is never the maker.
5. THE system SHALL write PKS_Records and Director_PIC changes to the primary database only, and SHALL store `effective_date`, `expiry_date` as `date` and every timestamp as `timestamptz` (UTC).
6. THE VendorPortal SHALL NOT expose PKS or Director_PIC data (D12).

### Requirement 12: Migrasi, kontrak API, dan regresi

**User Story:** Sebagai developer, saya ingin perubahan skema aditif dan perilaku lama tetap utuh, supaya fitur ini aman diterapkan ke DB yang sudah berisi data.

#### Acceptance Criteria

1. THE migration `022_vendor_pks.sql` SHALL add the PKS table (with the `expiry_date >= effective_date` CHECK and the one-Active_PKS constraint) and the `vendor_pics.is_director boolean NOT NULL DEFAULT false` column with the Director_PIC constraints, without changing `vendors`, and THE migration `037_export_jobs_vendor_pks.sql` SHALL admit the PKS export kind in `export_jobs` and add the Director_PIC empty-roles constraint (which needs `vendor_pics.roles` from `032`); both SHALL leave existing rows unchanged and SHALL be applied with `go run ./cmd/migrate up`.
2. THE existing vendor and PIC endpoints, the vendor and PIC CSV round-trip, the notification recipient queries, and the vendor export kinds SHALL keep their current behavior apart from the added fields; existing tests SHALL stay green.
3. THE new VendorPksAPI endpoints SHALL be described in `backend/api/openapi.yaml`, and CompanyPortal SHALL use the generated types for them; the existing `/admin/vendors` endpoints stay hand-typed (D31).
4. THE test suite SHALL cover the PksStatusRule boundaries (expiry day, threshold day, month-end clamp, leap year), maker-checker gates (pending 409, stale, maker ≠ checker), RBAC denials, the one-Active_PKS guarantee under concurrent approvals, and filter = displayed status, and the existing coverage gates SHALL still pass.
5. THE migration and the new PKS entity SHALL be recorded in `.claude/CLAUDE.md` Sec 3 and `development-plan.md` when applied.

## Open Questions

### Keputusan PO dari intent (✅ DIPUTUSKAN PO)

| ID | Asal di intent | Keputusan | Dipakai di |
| --- | --- | --- | --- |
| D1 | R1 (2026-11-05) | ✅ PKS disimpan di **tabel sendiri**, bukan kolom `vendors`, karena PKS punya riwayat (perpanjangan, "Perubahan dan Pernyataan Kembali"). | Req 3, 4, 12.1 |
| D2 | R2 | ✅ **Satu vendor, satu PKS aktif.** | Req 4.1 |
| D3 | R3 | ✅ PKS baru (termasuk "Perubahan dan Pernyataan Kembali") = **baris baru**; saat disetujui PKS aktif lama ditutup. **Tidak ada tautan ke PKS induk** (`parent_pks_id` tidak dibuat). | Req 3.7 |
| D4 | R4 + O1 | ✅ Upload file PKS ada tapi tidak wajib, dan **ditunda**: fitur ini tanpa upload; menyusul bersama modul dokumen (Phase 0.4). Pilihan tidak diambil: bytea di DB, GCS sekarang. | Tidak termasuk |
| D5 | R5 + A1 | ~~Nama + kontak Direktur Utama = kolom di `vendors`~~ → **digantikan D32** (2026-11-05). | – |
| D6 | A4 | ✅ Ambang Need Renewal **9 bulan**, sebagai parameter (FSD juga menyebut 8 bulan). | Req 6.7 |
| D7 | Q2 | ✅ **Terminated = status PKS saja**; vendor tetap aktif. Bisa dibatalkan; keduanya lewat maker-checker. | Req 5 |
| D8 | Q3 | ✅ Data PKS **opsional**. Vendor `kind='INTERNAL'` (ROH) dikecualikan, tidak punya status. | Req 3.5, 6.6 |
| D9 | Q4 | ✅ Vendor FLM tanpa PKS → **"Belum diisi"**, badge netral. | Req 6.2, 9.1 |
| D10 | Q5 | ✅ Data direktur **ikut CSV import/export** (pola upsert sekarang), sejak D32 lewat CSV PIC (kolom `is_director`), bukan CSV vendor. Data PKS tidak lewat CSV import. | Req 2 |
| D11 | Q6 | ✅ Daftar vendor dapat kolom status + filter lima label, dihitung **server-side**. | Req 8 |
| D12 | Q8 | ✅ Akses = **role master data yang ada** (`ADMIN`/`ADMIN_PARAM`), tanpa role baru. Tidak tampil di VendorPortal. | Req 11 |
| D13 | O2 | ✅ Service Type = pilihan kategori vault yang sudah ada (`vendor_vaults.category`: ATM / CASH / ATM_CASH), tanpa kolom baru. Kolom Service Type di Master inquiry per vendor **dihapus**. | Tidak termasuk |
| D14 | O3 | ✅ **Satu kolom label**: Aktif (hijau) / Need Renewal (kuning, sisa < 9 bulan) / Expired (merah) / Terminated / Belum diisi (abu-abu). Warna mengikuti label, tidak ada kolom flag terpisah. | Req 6.2, 9.1 |
| D15 | O4 | ✅ Masa Berlaku **dihitung dari tanggal**, tidak diisi manual, tidak disimpan. Contoh 2026-01-01 → 2027-12-31 = 2 tahun (angka `1` di xlsx tidak dipakai). | Req 7.1 |
| D16 | O5 | ✅ **"Generate report" masuk** (ekspor CSV sesuai filter lewat `backend-exq`). **Upload massal data PKS tidak dibangun** (±9 vendor). | Req 2.5, 10 |
| D17 | O6 | ✅ PKS baru bertanggal efektif di masa depan **langsung menggantikan** PKS lama saat disetujui; peringatan Need Renewal hilang. | Req 3.7, 6.5 |
| D18 | O7 | ✅ Monitoring dalam **hari saja** ("421 hari"); Expired tampil "lewat N hari", bukan negatif. | Req 7.3, 7.4 |
| D19 | O8 | ✅ **Limit CIS dipisah** ke `.claude/sdlc/vendor-cis-limit/`; fitur ini = PKS + data direktur. | Tidak termasuk |

### Saran Kiro untuk hal teknis (✅ DIPUTUSKAN PO 2026-11-05, mengikuti saran Kiro)

| ID | Pertanyaan | Saran Kiro | Dipakai di |
| --- | --- | --- | --- |
| D20 | Bagaimana "sisa < 9 bulan" dan hari expired dihitung? | Tambah 9 bulan kalender ke Today_WIB (tanggal di-clamp ke akhir bulan). Need Renewal bila `expiry_date` < hasilnya. Pada hari expired PKS masih berlaku (sisa 0 hari); Expired mulai hari berikutnya. | Req 6.2–6.4 |
| D21 | Satuan dan pembulatan Masa Berlaku untuk masa yang tidak genap tahun? | Bulan kalender penuh dari `effective_date` sampai `expiry_date` + 1 hari, sisa hari dibulatkan ke bawah. Tampil "N tahun M bulan" tanpa bagian nol; "< 1 bulan" bila 0. | Req 7.1, 7.2 |
| D22 | Urutan prioritas label dan tampilan Terminated? | Terminated > Expired > Need Renewal > Aktif. Monitoring untuk Terminated dan Belum diisi tampil "-". Badge Terminated netral dengan ikon dan teks sendiri, supaya tidak tertukar dengan Expired. | Req 6.2, 7.5, 9.1 |
| D23 | Validasi tanggal PKS baru? | Hanya `expiry_date >= effective_date` (juga CHECK di DB). Tanggal efektif boleh lampau atau masa depan; tidak dibandingkan dengan PKS lama. | Req 3.3, 3.4 |
| D24 | Bolehkah dua PKS_Change pending untuk satu vendor? | Tidak. Satu PKS_Change pending per vendor (create/terminate/un-terminate), selebihnya 409, mengikuti `ErrMasterDataChangePending`. Perubahan direktur (entitas `vendor`) tidak terhalang oleh PKS_Change pending. | Req 3.6 |
| D25 | Bagaimana mengoreksi PKS yang salah ketik? | Tidak ada edit PKS_Record. Koreksi = catat PKS baru (menggantikan yang salah, yang lama tetap di riwayat). Konsisten dengan "PKS lama tidak dihapus atau ditimpa". **Perlu konfirmasi PO**: alternatifnya edit Active_PKS lewat maker-checker. | Req 4.2 |
| D26 | Bolehkah mencatat PKS baru saat PKS aktif Terminated? | Boleh (vendor dikontrak lagi). Terminated_PKS masuk riwayat dengan data terminate tetap. Batalkan terminate hanya untuk Active_PKS. | Req 3.10, 5.4, 5.6 |
| D27 | Kolom mana di daftar vendor, mana di detail dan CSV? | Daftar: Nama Direktur Utama, Nama PKS, Tanggal Expired, Monitoring, Status PKS (tambahan dari kolom sekarang). Detail: semua field + riwayat. CSV: semua kolom Req 10.3. Kontak direktur dan tanggal efektif tidak di daftar supaya tabel tetap terbaca. | Req 9.2, 10.3 |
| D28 | Cara mencari vendor untuk terminate ("berdasarkan Nama Perusahaan", FSD)? | Pakai pencarian `q` daftar vendor yang sudah ada (nama), lalu aksi di detail vendor. Tanpa layar pencarian baru. | Req 8.5, 9.5 |
| D29 | Vendor Internal dan vendor nonaktif? | Internal: tanpa PKS_Status, tidak cocok dengan filter `pks_status` mana pun, submit PKS ditolak 422. Nonaktif: PKS_Status tetap dihitung dan tampil, tapi submit PKS_Change ditolak 409 sampai vendor diaktifkan. | Req 3.5, 6.6, 9.6 |
| D30 | "Hari ini" dari mana? | Tanggal Asia/Jakarta di server saat request atau job ekspor diproses, tidak dari klien. | Glossary, Req 6.1, 10.2 |
| D31 | Kontrak OpenAPI? | Endpoint PKS baru masuk `backend/api/openapi.yaml` (aturan "modul baru mulai dari spec"), tipe FE di-generate. Endpoint `/admin/vendors` lama tetap hand-typed (BE-10 deferred). | Req 12.3 |

Semua saran D20–D31 disetujui PO 2026-11-05. D25 (koreksi = catat PKS baru, tanpa edit) ikut disetujui.

### Revisi PO 2026-11-05

| ID | Keputusan | Dipakai di |
| --- | --- | --- |
| D32 | ✅ **Direktur Utama dicatat di `vendor_pics`, bukan kolom baru di `vendors`** (PO: "masukan saja ke table pic vendor, jangan buat field baru di table vendors"). Opsi A dipilih: kolom penanda `vendor_pics.is_director` + constraint maksimal satu direktur aktif per vendor, hanya PIC tingkat vendor. Direktur **hanya pencatatan**: tidak bisa login, tidak punya roles, bukan penerima notifikasi. Opsi tidak diambil: B (kenali dari teks `position`), C (nilai baru di `roles`). Menggantikan D5; mengubah D10. | Req 1, 2, 8.1, 9.3, 10.3, 12.1 |

**Penyesuaian criteria saat design (2026-11-05):**
- **2.3:** import sudah mendukung legacy header (`pkg/csvexport` `LegacyHeaders` + `KeepCurrentCell`, dipakai `vendor_branches`). File lama tanpa kolom direktur tetap diterima dan nilai direktur tidak berubah, ketimbang ditolak. Sejak D32 aturan ini berlaku untuk CSV PIC.
- **3.8:** applier juga menandai `stale` bila vendor sudah nonaktif atau bukan FLM saat approve, supaya PKS tidak tercatat untuk vendor yang tidak lagi memenuhi 3.5.
- **12.1:** perubahan CHECK `export_jobs` tidak bisa di `022`: pada DB baru, `022` jalan sebelum `034` yang membuat `export_jobs`. Bagian itu pindah ke `037`.
- **1.3, 3.2, 3.3, 3.5, 5.7, 9.7, D29:** error validasi field memakai **422** (`writeValidationError`, `error = validation_error`, `details[]` per field), sama dengan endpoint master data yang ada, bukan 400. 400 tetap untuk query tidak valid (8.3) dan body bukan JSON.
- **8.6:** detail vendor sekarang dibaca dari primary (`VendorAdminRepository.GetByID`), bukan replica. Detail PKS mengikuti pola itu.

### Untuk diputuskan di design (teknis)

- Nama final tabel PKS (usulan `vendor_pks`), kolom, dan bentuk constraint satu Active_PKS (partial unique index `WHERE replaced_at IS NULL`).
- Bentuk endpoint VendorPksAPI dan payload PKS_Change (`entity_type = vendor_pks`, satu applier untuk tiga operasi).
- Query status server-side untuk filter (join ke Active_PKS, Today_WIB sebagai parameter query, bukan `now()` di SQL supaya bisa diuji).
- Lokasi Renewal_Threshold (Go const + env override vs `pkg/config`).
- Jenis ekspor PKS: kind master data di `exportjob.KindSpecs` atau Report_Kind di `pkg/reports`.
- Komponen UI yang terdampak di `features/admin-vendors`.

## Dependency & Scope Note

- Bergantung pada maker-checker master data, `backend-exq` export, dan `pkg/csvexport` yang sudah ada. Tidak bergantung pada modul dokumen (D4).
- Fitur ini dikecualikan dari scope guard rule 2 (no new master-data features) hanya bila user menyetujui mulai build; `development-plan.md` mencatatnya di "Not built" sampai itu.
- `vendor-cis-limit` (D19) bisa memakai kolom direktur dari fitur ini, tapi tidak sebaliknya.
