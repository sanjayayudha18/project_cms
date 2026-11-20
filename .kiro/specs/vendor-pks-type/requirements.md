# Requirements Document

> Status: accepted 2026-11-18 (PO) — Introduction, Glossary, Requirements 1–10, asumsi A1–A5.

## Introduction

### Masalah

User (2026-11-18) punya daftar PKS vendor dalam CSV dan ingin datanya masuk ke CROWN. Kolom CSV: `Status, Type, Nama Vendor, Judul PKS, No. WOD, Tgl Efektif, Tgl Masa Akhir`. Ada 12 baris, semuanya berstatus `Aktif`, berlaku 1-Jan-26 s/d 31-Dec-27:

| Type | Vendor | No. WOD |
|---|---|---|
| FLM | PT Abacus Cash Solution | 1893/TO/XI/2025 |
| FLM | PT Advantage SCM | 1895/TO/XI/2025 |
| FLM | PT Bintang Jasa Artha Kelola | 1892/TO/XI/2025 |
| FLM | PT Brinks Solutions Indonesia | 1897/TO/XI/2025 |
| FLM | PT Swadharma Sarana Informatika | 1896/TO/XI/2025 |
| FLM | PT Tunas Artha Gardatama | 1894/TO/XI/2025 |
| CIT | PT Abacus Cash Solution | 1919/TO/XI/2025 |
| CIT | PT Brinks Solutions Indonesia | 1922/TO/XI/2025 |
| CIT | PT Kelola Jasa Artha | 1923/TO/XI/2025 |
| CIT | PT Advantage SCM | 1920/TO/XI/2025 |
| CIT | PT Swadharma Sarana Informatika | 1925/TO/XI/2025 |
| CIT | PT Bintang Jasa Artha Kelola | 1921/TO/XI/2025 |

Judul PKS FLM: *Perubahan dan Pernyataan Kembali Perjanjian Induk Kerjasama Jasa Layanan Cash Replenishment, Cash Collection, dan First Line Maintenance ATM, CDM, TST*. Judul PKS CIT: *Perubahan dan Pernyataan Kembali Perjanjian Induk Kerjasama tentang Jasa Layanan Uang Tunai dan Barang Berharga*.

Satu vendor bisa terikat **dua PKS aktif sekaligus**, satu FLM dan satu CIT, masing-masing dengan nomor WOD sendiri. Lima vendor di CSV mengalami ini. Data tersebut tidak bisa disimpan di CROWN karena:

1. **Satu PKS aktif per vendor.** `vendor_pks_active_uq` (`022`) mengunci `(vendor_id) WHERE replaced_at IS NULL`. PKS kedua akan *menggantikan* PKS pertama (D1, D17 spec `vendor-pks`).
2. **Kolom Type dan No. WOD tidak ada.** `vendor_pks` hanya menyimpan `pks_name, effective_date, expiry_date`.
3. **PKS tidak bisa diimpor lewat CSV.** Spec `vendor-pks` D16 / Req 2.5 menolak data PKS di semua impor CSV.

Kolom `Status` di CSV tidak perlu disimpan, karena sistem menghitungnya lewat `vendor_pks_status()` (D6/D22).

### Kondisi kode saat ini (dicek 2026-11-18)

- **Tabel `vendor_pks`** (`022`): `id, vendor_id, pks_name, effective_date, expiry_date, created_by, replaced_at, replaced_by_pks_id, terminated_at, terminated_by, termination_reason, created_at, updated_at`. Ada `vendor_pks_active_uq` (satu aktif per vendor) dan `master_data_change_requests_vendor_pks_pending_uq` (satu change `vendor_pks` pending per vendor, D24). Di DB dev masih **0 baris**.
- **Status**: `vendor_pks_status(kind, expiry, terminated_at, today, renewal_months)` dan `vendor_pks_validity_months()`. Status dihitung per vendor dari *satu* PKS aktif.
- **Pemakai "satu PKS aktif per vendor"**:
  - daftar vendor + filter `pks_status`: `backend/queries/vendors_admin.sql`
  - PKS admin (create/replace/terminate/batalkan terminate): `backend/queries/vendor_pks.sql`, `vendor_pks_admin.go`, `masterdata_applier_vendor_pks.go`, `admin_vendor_pks_handler.go`, `backend/api/openapi.yaml`
  - laporan PKS: `backend/queries/vendor_pks_report.sql`, `pkg/reports`, `backend-exq` `vendor_pks_report.sql` + kind ekspor `vendor_pks` (`037`)
  - CSV vendor: kolom baca-saja PKS aktif (spec `vendor-export-pks`), `backend-exq/queries/vendor_export_pks.sql`
  - layar: `frontend/CompanyPortal-Vite/src/features/admin-vendors/`
- **Vendor di DB dev**: `vendors.name` berisi nama pendek (`Abacus`, `Advantage`, `Bijak`, `Brinks`, `Kejar`, `Prosegur`, `SSI`, `TAG`, `ROH`), dan `legal_name` masih kosong untuk semua vendor. Ada juga sisa vendor tes `ITESTNUM-*`.
- **`vendors.kind`** = `FLM_VENDOR | INTERNAL` (vendor FLM vs unit internal CIMB). Ini bukan type PKS.
- Nomor migrasi berikutnya: `043`.

### Hasil yang diinginkan

- Satu vendor bisa punya PKS aktif per type (FLM, CIT), masing-masing lengkap dengan No. WOD, judul, tanggal efektif, dan tanggal masa akhir.
- Status, masa berlaku, monitoring sisa hari, terminate, dan riwayat tetap berjalan, kini per PKS per type.
- 12 baris CSV di atas bisa masuk ke sistem (caranya tergantung jawaban Q6).

### Batasan

- Menyentuh skema (migrasi `043`), jadi termasuk area CLAUDE.md Sec 4 #7: wajib rantai bergate penuh, persetujuan skema, dan uji maker-checker + RBAC.
- Maker-checker master data tetap berlaku (Sec 5, D1–D4 Sec 12). Tidak ada deviasi GR#3 yang diusulkan.
- `vendor_pks` tetap tanpa hard-delete. `pks_name`, tanggal, serta kolom baru tidak pernah di-UPDATE (pola `022`).
- Baca di replica, tulis di primary. Respons ATM `backend` tetap JSON datar, `backend-exq` tetap memakai envelope `pkg/response`.
- Spec `vendor-pks` (tahap 3, menunggu cek browser) dan `vendor-export-pks` **tidak dibuka ulang**. Perubahan perilaku dicatat di sini sebagai pengganti (supersede) criteria terkait.

### Ruang lingkup (usulan, menunggu jawaban pertanyaan)

- Kolom type PKS dan No. WOD di `vendor_pks`, serta satu PKS aktif per (vendor, type).
- Create/replace/terminate/batalkan terminate PKS per type lewat maker-checker.
- Status PKS, daftar vendor, filter, laporan, ekspor, dan CSV vendor disesuaikan dengan PKS per type.
- Cara memasukkan 12 baris CSV ini (Q6).

### Tidak termasuk

- Upload file PKS (D4 `vendor-pks`, tetap menunggu modul dokumen).
- Notifikasi PKS mendekati expired.
- Limit CIS / escrow (`vendor-cis-limit`).
- Mengubah `vendors.kind`.

### Pertanyaan terbuka (semua dijawab PO 2026-11-18)

| # | Pertanyaan |
|---|---|
| Q1 | Type PKS apa saja: hanya `FLM` dan `CIT`, atau bisa bertambah (perlu tabel master type)? Apakah type wajib diisi untuk setiap PKS? — ✅ DIPUTUSKAN PO 2026-11-18: **type bisa bertambah** (awal `FLM`, `CIT`), sehingga perlu daftar type yang bisa dikelola, bukan CHECK tetap. Bagian "wajib diisi" belum dijawab → Q1b. |
| Q1a | Siapa yang menambah type baru, dan lewat apa: (a) layar master data dengan maker-checker (pola Sec 12, `ADMIN`/`ADMIN_PARAM`), atau (b) hanya lewat migrasi/seed oleh tim dev (tanpa layar)? Apakah type boleh dinonaktifkan (soft-disable) bila masih dipakai PKS aktif? — ✅ DIPUTUSKAN PO 2026-11-18: **(a) layar master data + maker-checker** (entitas master data baru, pola Sec 12: queries → repo read-only → service `Submit` → `Applier` di `main.go` → handler 202; checklist `cms2-master-data`). Aturan nonaktif type yang masih dipakai belum dijawab → Q1c. |
| Q1c | Type yang masih dipakai PKS aktif: (a) tidak bisa dinonaktifkan sampai semua PKS-nya diganti/terminated (pola region 042, dicek di tx apply), atau (b) boleh dinonaktifkan, PKS yang ada tetap berjalan tetapi type tidak bisa dipilih untuk PKS baru? Siapa maker-nya: `ADMIN`/`ADMIN_PARAM` seperti master data lain? — ✅ DIPUTUSKAN PO 2026-11-18: **(a) diblok**: type yang masih dipakai PKS aktif (`replaced_at IS NULL`, termasuk yang terminated — Q1d) tidak bisa dinonaktifkan; dicek saat submit **dan** di tx apply setelah `FOR UPDATE` pada baris type (pola region `042`). Maker role belum dijawab → Q1d. |
| Q1d | (i) Maker/checker type PKS: `ADMIN`/`ADMIN_PARAM` seperti master data lain? (ii) PKS yang sudah **terminated** tetapi belum diganti (`replaced_at IS NULL`, `terminated_at` terisi): apakah tetap dihitung "masih dipakai" sehingga memblok nonaktif type? — ✅ DIPUTUSKAN PO 2026-11-18: (i) maker + checker **`ADMIN`/`ADMIN_PARAM`** (route + `Submit`, maker ≠ checker); (ii) **ya, memblok**: "dipakai" = ada PKS dengan type itu dan `replaced_at IS NULL`, terminated atau tidak (karena batalkan terminate bisa menghidupkannya lagi). |
| Q1b | Apakah setiap PKS wajib punya type? (Di DB dev belum ada baris `vendor_pks`, jadi tidak perlu backfill.) — ✅ DIPUTUSKAN PO 2026-11-18: **wajib** (type PKS `NOT NULL`, harus type aktif saat submit dan apply). |
| Q2 | Daftar vendor dan filter `pks_status`: tampilkan status FLM dan CIT di dua kolom terpisah, satu baris per PKS, atau satu status ringkas per vendor (mis. yang terburuk)? — ✅ DIPUTUSKAN PO 2026-11-18: **kolom per type** di daftar vendor (satu baris per vendor; satu grup kolom PKS per type). Karena type bisa bertambah (Q1), kolom dibentuk dari daftar type **aktif**, bukan di-hardcode; API mengembalikan PKS per type sebagai daftar per vendor. Jumlah kolom ikut bertambah tiap type baru (diterima PO). Isi grup kolom dan filter → Q2a. |
| Q2a | (i) Kolom apa per type di daftar: sekarang daftar vendor menampilkan Nama PKS, Tanggal Expired, Monitoring, Status PKS (D27 `vendor-pks`). Tetap empat per type, atau cukup Status (+ Monitoring) per type dan sisanya di detail? (ii) Filter `pks_status`: pilih type + status (mis. "CIT = expired"), atau satu status yang cocok bila **salah satu** type berstatus itu? — ✅ DIPUTUSKAN PO 2026-11-18 (setuju rekomendasi): (i) per type hanya **Status + Monitoring (sisa hari)**; Nama PKS, No. WOD, tanggal efektif/expired di detail vendor (menggantikan sebagian D27 `vendor-pks` untuk daftar); (ii) filter = **pilih type + status** (mis. "CIT = expired"). |
| Q3 | Apakah setiap vendor FLM wajib punya PKS FLM *dan* CIT? Contoh: Kelola Jasa Artha hanya punya CIT. Apakah PKS FLM-nya dianggap `belum_diisi`, atau type yang tidak dimiliki vendor tidak ditampilkan? — ✅ DIPUTUSKAN PO 2026-11-18 (setuju rekomendasi): **tidak wajib** punya PKS untuk setiap type. Type tanpa PKS tampil netral "Tidak ada" (bukan peringatan); filter type + `belum_diisi` tetap menemukan vendor tanpa PKS untuk type itu. |
| Q4 | No. WOD: wajib atau opsional? Harus unik (global atau per vendor)? Formatnya dikunci (`NNNN/TO/<bulan romawi>/YYYY`) atau teks bebas? — ✅ DIPUTUSKAN PO 2026-11-18 (setuju rekomendasi): No. WOD **wajib**, teks bebas 1–100 karakter setelah trim, **unik global** di semua baris `vendor_pks` (termasuk riwayat), case-insensitive; format tidak dikunci. |
| Q5 | Ekspor dan laporan PKS (kind `vendor_pks`, kolom PKS di CSV vendor): satu baris per PKS, atau satu baris per vendor dengan kolom per type? — ✅ DIPUTUSKAN PO 2026-11-18 (setuju rekomendasi): laporan/ekspor kind `vendor_pks` = **satu baris per PKS** (+ kolom Type, No. WOD). CSV vendor (`vendor-export-pks`): **kolom PKS dihapus** (koreksi Kiro saat menulis EARS: ada **6** kolom PKS `pks_name, pks_effective_date, pks_expiry_date, pks_validity_months, pks_remaining_days, pks_status`, bukan 5), kolom direktur tetap (menggantikan sebagian `vendor-export-pks`; format CSV vendor berubah, header lama tetap diterima impor sebagai legacy). |
| Q6 | Bagaimana 12 baris ini masuk: (a) impor CSV PKS baru (membatalkan D16), (b) input manual lewat layar, atau (c) seed/skrip satu kali? — ✅ DIPUTUSKAN PO 2026-11-18 (setuju rekomendasi): **(b) input manual lewat layar** (maker-checker). D16 tetap berlaku; tanpa seed/skrip yang melewati maker-checker. Impor CSV PKS = spec terpisah bila nanti dibutuhkan. |
| Q7 | Kolom `Type` di CSV tidak sama dengan `vendors.kind` (FLM_VENDOR/INTERNAL), betul? Dan apakah vendor yang hanya punya PKS CIT (mis. Kelola Jasa Artha) tetap `kind = FLM_VENDOR`? — ✅ DIPUTUSKAN PO 2026-11-18 (setuju rekomendasi): type PKS **terpisah** dari `vendors.kind`; vendor dengan PKS CIT saja tetap `FLM_VENDOR`; vendor `INTERNAL` (ROH) tetap tanpa PKS. |
| Q8 | Konfirmasi pemetaan nama CSV ke vendor CROWN: Abacus Cash Solution → `ABA`, Advantage SCM → `ADV`, **Bintang Jasa Artha Kelola → `BJK` ("Bijak")?**, Brinks Solutions Indonesia → `BRNK`, **Kelola Jasa Artha → `KJR` ("Kejar")?**, Swadharma Sarana Informatika → `SSI`, Tunas Artha Gardatama → `TAG`. Perlukah `legal_name` diisi dengan nama PT sekalian (perubahan master data vendor, maker-checker)? — ✅ DIPUTUSKAN PO 2026-11-18: pemetaan **benar semua** (termasuk `BJK` = PT Bintang Jasa Artha Kelola, `KJR` = PT Kelola Jasa Artha). Bagian `legal_name` belum dijawab → Q8a. |
| Q8a | Isi `vendors.legal_name` ketujuh vendor dengan nama PT dari CSV? Itu perubahan master data vendor lewat maker-checker yang ada (layar/CSV vendor), **di luar** spec ini — cukup dicatat sebagai langkah operasional, atau tidak perlu? — ✅ DIPUTUSKAN PO 2026-11-18: **ya, langkah operasional** (bukan kode): isi `legal_name` ketujuh vendor dengan nama PT dari CSV lewat layar/CSV vendor yang ada (maker-checker), sebelum PKS diinput. |
| Q9 | Prosegur (`PSG`) tidak ada di CSV. Apakah memang tidak punya PKS aktif? — ✅ DIPUTUSKAN PO 2026-11-18: **betul, Prosegur tidak punya PKS aktif**; tampil "Tidak ada" di setiap kolom type (Q3). |

## Glossary

Istilah dari spec `vendor-pks` (Vendor_FLM, Vendor_Internal, Replaced_PKS, Terminated_PKS, Today_WIB, Renewal_Threshold, Masa_Berlaku, Monitoring_PKS, Master_Data_Role, Director_PIC, MasterDataChangeService, AuditWriter, ExportAPI) tetap berlaku. Yang baru atau berubah:

- **PKS_Type**: satu baris master type PKS (nama tabel final di design, usulan `vendor_pks_types`): `code` (mis. `FLM`, `CIT`), `name`, aktif/nonaktif. Awal: `FLM`, `CIT` (Q1).
- **Active_PKS_Type**: PKS_Type yang aktif. Hanya ini yang bisa dipilih untuk PKS baru dan yang menjadi kolom di daftar vendor.
- **PKS_Record**: satu baris `vendor_pks`, kini dengan PKS_Type (wajib, Q1b) dan `wod_number` (wajib, Q4).
- **Active_PKS**: PKS_Record yang belum digantikan. **Paling banyak satu per (vendor, PKS_Type)** (menggantikan D2 `vendor-pks`).
- **PKS_Type_In_Use**: PKS_Type yang masih punya PKS_Record dengan `replaced_at IS NULL`, terminated atau tidak (Q1d).
- **WOD_Number**: nomor WOD PKS, teks bebas 1–100 karakter setelah trim, unik global tanpa membedakan huruf besar/kecil di semua PKS_Record (Q4).
- **PKS_Change**: master-data change pending untuk PKS satu (vendor, PKS_Type): `create`, `terminate`, `unterminate`.
- **PKS_Type_Change**: master-data change pending untuk satu PKS_Type: `create`, `update`, `disable`, `enable`.
- **PKS_Status**: label per (Vendor_FLM, PKS_Type), aturan sama dengan Req 6 `vendor-pks`. Nilai `belum_diisi` ditampilkan di layar sebagai **"Tidak ada"** (netral, Q3).
- **PksTypeAPI**: endpoint baru master type PKS di bawah `/api/v1/admin/pks-types` (path final di design).
- **PksTypeApplier**: applier baru untuk entitas `vendor_pks_type`.
- **VendorPksAPI**: `/api/v1/admin/vendors/{vendorID}/pks…` yang sudah ada, kini dengan PKS_Type dan WOD_Number.
- **VendorPksApplier**: applier `vendor_pks` yang sudah ada.
- **VendorAdminAPI**: `/api/v1/admin/vendors` yang sudah ada.
- **VendorCsv**: CSV impor/ekspor vendor (`pkg/csvexport` `Vendors`, `VendorReadOnlyColumns`, ekspor `backend-exq`).
- **PksTypeScreen**: layar baru master type PKS di CompanyPortal.
- **VendorAdminScreen**: layar vendor yang sudah ada (`features/admin-vendors/`).

## Requirements

### Requirement 1: Master type PKS

**User Story:** Sebagai tim OPS, saya ingin mengelola daftar type PKS (FLM, CIT, dan type lain nanti) lewat layar dengan maker-checker, supaya type baru bisa ditambah tanpa rilis kode.

#### Acceptance Criteria

1. WHEN a Master_Data_Role user submits a PKS_Type create with `code` and `name`, THE PksTypeAPI SHALL stage a PKS_Type_Change through MasterDataChangeService and return 202, without changing any PKS_Type (Q1a).
2. IF `code` does not match `^[A-Z0-9]{2,10}$` or `name` is empty after trim or longer than 100 characters, THEN THE PksTypeAPI SHALL reject the request with 422 and name the field.
3. IF another PKS_Type already has the same `code` (case-insensitive), or a pending PKS_Type_Change would create it, THEN THE PksTypeAPI SHALL reject the create with 409.
4. WHEN a Master_Data_Role user submits an update, THE PksTypeAPI SHALL accept a change of `name` only; `code` SHALL be immutable after create.
5. WHEN a Master_Data_Role user submits a disable for a PKS_Type, THE PksTypeAPI SHALL reject it with 409 IF the PKS_Type is a PKS_Type_In_Use (Q1c, Q1d).
6. WHEN a PKS_Type disable is approved, THE PksTypeApplier SHALL lock the PKS_Type row (`FOR UPDATE`) and re-check PKS_Type_In_Use inside the apply transaction; IF it is in use, THEN the change SHALL fail with 409 and apply nothing (pola region `042`).
7. IF a PKS_Type already has a pending PKS_Type_Change, THEN THE PksTypeAPI SHALL reject another change for it with 409; IF the PKS_Type changed between submit and approval, THEN THE PksTypeApplier SHALL mark the change `stale` and apply nothing.
8. THE system SHALL never hard-delete a PKS_Type; disable = soft-disable, and enable makes it an Active_PKS_Type again.
9. THE PksTypeAPI SHALL list PKS_Types (active and inactive, with an in-use flag) from the replica pool and return detail from the primary pool.
10. THE migration SHALL seed the PKS_Types `FLM` and `CIT` as active.

### Requirement 2: PKS dengan type dan No. WOD

**User Story:** Sebagai tim OPS, saya ingin mencatat PKS vendor lengkap dengan type dan No. WOD, supaya satu vendor bisa punya PKS FLM dan PKS CIT yang berjalan bersamaan.

#### Acceptance Criteria

1. WHEN a Master_Data_Role user submits a PKS create, THE VendorPksAPI SHALL require `pks_type` (an Active_PKS_Type) and `wod_number` in addition to the existing `pks_name`, `effective_date`, `expiry_date` rules (Req 3 `vendor-pks`), and return 202.
2. IF `pks_type` is missing, unknown, or not an Active_PKS_Type, THEN THE VendorPksAPI SHALL reject the request with 422 on field `pks_type` (Q1b).
3. IF `wod_number` is empty after trim or longer than 100 characters, THEN THE VendorPksAPI SHALL reject the request with 422 on field `wod_number`; THE VendorPksAPI SHALL NOT enforce a format pattern (Q4).
4. IF `wod_number` (trimmed, case-insensitive) equals the WOD_Number of any PKS_Record, including Replaced_PKS, or of another pending PKS_Change, THEN THE VendorPksAPI SHALL reject the request with 409 on field `wod_number` (Q4).
5. WHEN a `create` PKS_Change is approved, THE VendorPksApplier SHALL replace only the Active_PKS of the **same** (vendor, PKS_Type), leave Active_PKS of other PKS_Types unchanged, and insert the new PKS_Record, in one transaction with the audit entry.
6. IF, at approval time, the PKS_Type is no longer active, the WOD_Number is now taken, or the Active_PKS of that (vendor, PKS_Type) is not the one recorded at submit, THEN THE VendorPksApplier SHALL mark the change `stale` (or fail with 409 for a constraint conflict) and apply nothing.
7. THE database SHALL guarantee at most one Active_PKS per (vendor, PKS_Type) and a unique WOD_Number (trimmed, case-insensitive) through constraints, so concurrent approvals cannot break either rule.
8. IF a vendor already has a pending PKS_Change for the same PKS_Type, THEN THE VendorPksAPI SHALL reject another one with 409; pending PKS_Changes for different PKS_Types of one vendor SHALL be allowed (menggantikan D24 `vendor-pks`).
9. THE system SHALL never change `pks_type` or `wod_number` of a stored PKS_Record, as for the other PKS fields (D25 `vendor-pks`).
10. THE VendorPksAPI SHALL accept PKS creates for a Vendor_FLM for any subset of PKS_Types; a vendor is not required to have a PKS for every PKS_Type (Q3, Q7).

### Requirement 3: Terminate, riwayat, dan detail per type

**User Story:** Sebagai tim OPS, saya ingin terminate, batalkan terminate, dan riwayat PKS berlaku per type, supaya pemutusan PKS CIT tidak menyentuh PKS FLM vendor yang sama.

#### Acceptance Criteria

1. THE VendorPksAPI SHALL keep the terminate and un-terminate rules of Req 5 `vendor-pks`, applied to one PKS_Record; a terminate or un-terminate SHALL change no PKS_Record of another PKS_Type.
2. WHEN a Master_Data_Role user opens a vendor's PKS, THE VendorPksAPI SHALL return, per PKS_Type that the vendor has any PKS_Record for, the Active_PKS and its history (newest first), each with PKS_Type, WOD_Number, `pks_name`, dates, Masa_Berlaku, replacement and terminate data, and every pending PKS_Change with its PKS_Type.
3. THE VendorPksAPI SHALL return PKS_Status and Monitoring_PKS for every Active_PKS, computed with the existing PksStatusRule per (vendor, PKS_Type).

### Requirement 4: Daftar vendor dengan kolom per type

**User Story:** Sebagai tim OPS, saya ingin melihat status PKS setiap type di daftar vendor dan memfilter per type + status, supaya vendor yang PKS CIT-nya expired langsung terlihat.

#### Acceptance Criteria

1. THE VendorAdminAPI SHALL return, for each vendor in the list, a `pks` list with one entry per Active_PKS_Type (ordered by `code`), each with `pks_type` code and name, PKS_Status, and Monitoring_PKS; an entry with no Active_PKS SHALL have PKS_Status `belum_diisi` and null Monitoring_PKS (Q2, Q2a, Q3).
2. THE VendorAdminAPI SHALL stop returning the single-PKS list fields of Req 8.1 `vendor-pks` (`pks_name`, `effective_date`, `expiry_date`, Masa_Berlaku, Monitoring_PKS, PKS_Status at vendor level); the Director_PIC fields stay unchanged.
3. WHEN the list request has `pks_type` (an Active_PKS_Type code) and `pks_status` (one of `aktif | need_renewal | expired | terminated | belum_diisi`), THE VendorAdminAPI SHALL return only vendors whose PKS_Status for that PKS_Type equals the value, combined with `q` and `status`, and `total` SHALL count the filtered rows (Q2a).
4. IF only one of `pks_type` and `pks_status` is sent, or either has an unknown value, THEN THE VendorAdminAPI SHALL reject the request with 400.
5. THE VendorAdminAPI SHALL compute the filter in the database query with the same PksStatusRule as the displayed `pks` entries, so paging, `total`, and the shown status agree.
6. THE VendorAdminAPI SHALL give a Vendor_Internal an empty `pks` list, and a Vendor_Internal SHALL match no `pks_status` filter (D8 `vendor-pks`, Q7).
7. THE VendorAdminAPI SHALL keep reading the list from the replica pool.

### Requirement 5: Layar

**User Story:** Sebagai tim OPS, saya ingin mengelola type PKS dan PKS per type dari CompanyPortal, supaya data CSV PKS bisa diinput lewat layar.

#### Acceptance Criteria

1. THE PksTypeScreen SHALL list PKS_Types with code, name, status, and in-use flag, and offer create, edit name, disable, and enable for Master_Data_Role users, showing 202 as "menunggu persetujuan"; disable SHALL be unavailable for a PKS_Type_In_Use, with the reason shown as text.
2. THE VendorAdminScreen SHALL show, per Active_PKS_Type, a column group with Status (badge with icon and text) and Monitoring ("N hari", "lewat N hari", or "-"); PKS_Status `belum_diisi` SHALL be shown as a neutral "Tidak ada" badge (Q2a, Q3).
3. THE VendorAdminScreen SHALL replace the vendor list columns Nama PKS, Tanggal Expired PKS, Monitoring PKS, Status PKS (D27 `vendor-pks`) with the per-type column groups, and the Status PKS filter with a PKS_Type select plus a status select, both required together.
4. THE VendorAdminScreen SHALL show the vendor's PKS in the detail grouped per PKS_Type (Active_PKS with WOD_Number, judul, tanggal efektif, tanggal expired, Masa_Berlaku, Monitoring, status; history; pending change).
5. THE "Catat PKS baru" form SHALL require a PKS_Type (select of Active_PKS_Types) and WOD_Number; "Terminate PKS" and "Batalkan terminate" SHALL act on one PKS_Type's Active_PKS and be disabled only while that PKS_Type has a pending PKS_Change.
6. THE screens SHALL be usable by keyboard, label every field and filter for screen readers, use `tabular-nums` for dates and day counts, never show status by color alone, and use no em dash in UI copy.

### Requirement 6: Laporan / ekspor PKS satu baris per PKS

**User Story:** Sebagai tim OPS, saya ingin ekspor PKS berisi satu baris per PKS lengkap dengan type dan No. WOD, supaya laporan cocok dengan daftar PKS yang saya kirim.

#### Acceptance Criteria

1. THE ExportAPI SHALL produce the `vendor_pks` export with one row per Active_PKS of the vendors matching `q` and `status` (Q5).
2. WHEN the export request has `pks_type` and `pks_status`, THE ExportAPI SHALL keep only rows of that PKS_Type with that PKS_Status; WHEN `pks_status = belum_diisi`, THE ExportAPI SHALL write one row per Vendor_FLM without an Active_PKS of that PKS_Type, with empty PKS cells.
3. THE ExportAPI SHALL write the columns Kode Vendor, Nama Perusahaan, Nama Direktur Utama, Telepon Direktur Utama, Email Direktur Utama, Type PKS, No. WOD, Nama PKS, Tanggal Efektif PKS, Tanggal Expired PKS, Masa Berlaku (bulan), Monitoring PKS (hari), Status PKS, every text cell through `CSVSafe`.
4. THE ExportAPI SHALL validate `pks_type` + `pks_status` with the same rules as Req 4.4 and keep the existing RBAC (Master_Data_Role, else 403), audit, and replica reads.

### Requirement 7: CSV vendor tanpa kolom PKS

**User Story:** Sebagai tim OPS, saya ingin CSV vendor tetap bisa dipakai bolak-balik setelah PKS menjadi per type, supaya file lama tidak ditolak.

#### Acceptance Criteria

1. THE VendorCsv export (preview and file) SHALL drop the six PKS read-only columns `pks_name, pks_effective_date, pks_expiry_date, pks_validity_months, pks_remaining_days, pks_status` and keep the three Director_PIC read-only columns (Q5, menggantikan sebagian `vendor-export-pks`).
2. THE VendorCsv import SHALL accept the new Export_Header and, as legacy headers, the previous Export_Header (with the nine read-only columns) and the Import_Header, dropping read-only cells before validation; a file matching none SHALL be rejected with the existing header-mismatch error.
3. THE VendorCsv import SHALL NOT accept PKS data (D16 `vendor-pks` tetap berlaku, Q6).

### Requirement 8: RBAC, audit, topologi DB

**User Story:** Sebagai pemilik sistem, saya ingin type PKS dan PKS per type terkendali dan tercatat seperti master data lain, supaya data kontrak vendor bisa diaudit.

#### Acceptance Criteria

1. THE PksTypeAPI SHALL be mounted behind the existing `masterDataAdmin` guard (Master_Data_Role), and THE MasterDataChangeService SHALL re-check the role on every PKS_Type_Change submit (Q1d).
2. IF a user without a Master_Data_Role calls any PksTypeAPI or VendorPksAPI endpoint, THEN the API SHALL respond 403 and stage nothing.
3. THE approval of a PKS_Type_Change or PKS_Change SHALL go through the existing master-data approval flow, so the checker is never the maker.
4. WHEN a PKS_Type_Change or PKS_Change is submitted, applied, or rejected, THE AuditWriter SHALL write an `audit_logs` row (actor, action, before, after, IP) in the same transaction as the state change; IF the audit write fails, THEN the change SHALL roll back. No GR#3 deviation.
5. THE system SHALL write PKS_Types and PKS_Records to the primary database only, and SHALL NOT expose them in the VendorPortal.

### Requirement 9: Migrasi, kontrak API, regresi

**User Story:** Sebagai developer, saya ingin perubahan skema aditif dan aman, supaya bisa diterapkan lewat `migrate up` tanpa merusak data.

#### Acceptance Criteria

1. THE migration `043` SHALL add the PKS_Type table (seeded `FLM`, `CIT`), add the PKS_Type reference and `wod_number` to `vendor_pks` as `NOT NULL`, replace the one-Active_PKS-per-vendor index with one per (vendor, PKS_Type), add the WOD_Number unique index, change the pending-change index to per (vendor, PKS_Type), and admit the new master-data entity type; it SHALL be applied with `go run ./cmd/migrate up`.
2. IF `vendor_pks` has rows when `043` runs, THEN the migration SHALL stop with an error instead of guessing a PKS_Type or WOD_Number (dev has 0 rows, 2026-11-18).
3. THE PksTypeAPI endpoints and the changed VendorPksAPI endpoints SHALL be described in `backend/api/openapi.yaml`, pass the contract test (`-run OpenAPI`), and CompanyPortal SHALL use the regenerated types (`gen:api`).
4. THE test suite SHALL cover: one Active_PKS per (vendor, PKS_Type) under concurrent approvals, replace touching only the same PKS_Type, WOD_Number uniqueness (case, trim, history, pending), PKS_Type disable blocked when in use (submit and apply, terminated included), maker-checker gates (pending 409 per type, stale, maker ≠ checker), RBAC denials, filter = displayed status per type, export rows per PKS, and the vendor CSV legacy headers; existing tests SHALL stay green.
5. THE migration, the new table, and the entity SHALL be recorded in `.claude/CLAUDE.md` Sec 3, `.claude/docs/data-map.md`, and `.claude/docs/decisions.md` when applied.

### Requirement 10: Memasukkan 12 PKS dari CSV

**User Story:** Sebagai tim OPS, saya ingin 12 PKS dari daftar CSV tercatat di CROWN, supaya monitoring PKS FLM dan CIT berjalan.

#### Acceptance Criteria

1. THE system SHALL let the 12 PKS rows be entered through the VendorAdminScreen PKS form with maker-checker; no seed or script SHALL bypass maker-checker (Q6).
2. THE operational steps (fill `legal_name` of the seven vendors through the existing vendor maker-checker, Q8a; then enter the 12 PKS with the vendor mapping of Q8) SHALL be written in `tasks.md` as a non-code task for the user.

## Asumsi Kiro (perlu dicek PO)

| # | Asumsi | Requirement |
|---|---|---|
| A1 | Pending change PKS dibatasi per (vendor, type), bukan per vendor, supaya PKS FLM dan CIT bisa diajukan bersamaan. | 2.8 |
| A2 | Aturan `code` type PKS sama dengan region (`^[A-Z0-9]{2,10}$`), nama 1–100 karakter, `code` tidak bisa diubah. | 1.2, 1.4 |
| A3 | Ekspor `vendor_pks` tanpa filter hanya berisi PKS aktif. Vendor yang tidak punya PKS sama sekali (mis. Prosegur) baru muncul kalau filter `pks_status = belum_diisi`. | 6.1, 6.2 |
| A4 | Filter daftar vendor: `pks_type` dan `pks_status` wajib dikirim berdua. | 4.4 |
| A5 | API daftar vendor berhenti mengirim field PKS tunggal (perubahan kontrak, dirilis bersama CompanyPortal). | 4.2 |
