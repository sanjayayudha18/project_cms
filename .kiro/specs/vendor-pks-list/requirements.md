# Requirements Document

> Status: accepted 2026-11-19 (PO) — Introduction, Glossary, Requirements 1–11, keputusan D1–D26, asumsi A1–A5 ("Setuju, lanjut design").

## Introduction

User (2026-11-19) melihat layar Type PKS hasil spec `vendor-pks-type` dan bertanya kenapa daftar PKS yang ia kirim (CSV `Status, Type, Nama Vendor, Judul PKS, No. WOD, Tgl Efektif, Tgl Masa Akhir`, 12 baris) tidak tampil sebagai tabel di layar. Di `vendor-pks-type`, PKS hanya bisa dilihat per vendor (tab PKS detail vendor), daftar vendor menampilkan grup kolom Status + Monitoring per type, dan bentuk "satu baris per PKS" hanya ada di ekspor CSV `vendor_pks`.

User ingin:
1. Satu layar **Daftar PKS** berisi semua PKS semua vendor, satu baris per PKS, dengan kolom seperti CSV-nya, dan menjadi satu-satunya tempat aksi PKS.
2. Daftar vendor (`/settings/admin/vendors`) tidak lagi menampilkan grup kolom PKS per type; diganti satu kolom **PKS** berisi link untuk melihat PKS vendor tersebut.
3. Ambang pengingat **Need Renewal** bisa diatur sesuai kebutuhan, per PKS, dalam hari.

Tabel PKS bersifat append: contoh PKS NCR 2025-2026 nanti diikuti PKS NCR baru 2026-2027; PKS lama tetap tercatat.

### Kondisi kode saat ini (dicek 2026-11-19)

- `vendor-pks-type` accepted 2026-11-18 dan sudah dibangun (task 1–9 selesai; tersisa task 10 input data oleh user dan task 11 cek browser manual). Migrasi terakhir `043_vendor_pks_type.sql`; nomor bebas berikutnya `044`.
- Tabel `vendor_pks` (022 + 043): `id, vendor_id, pks_type_id, wod_number, pks_name, effective_date, expiry_date, created_by, replaced_at, replaced_by_pks_id, terminated_at, terminated_by, termination_reason, created_at, updated_at`. Satu PKS aktif per (vendor, type) (`vendor_pks_active_uq`, `replaced_at IS NULL`). Judul, tanggal, type, WOD tidak pernah di-UPDATE; tidak pernah hard-delete. Semua tulis lewat maker-checker `entity_type = 'vendor_pks'`.
- PKS baru yang disetujui langsung menggantikan PKS aktif lama, termasuk bila tanggal efektifnya di masa depan (D17 `vendor-pks`).
- Status dihitung saat dibaca oleh fungsi DB `vendor_pks_status(kind, expiry, terminated_at, today, renewal_months)`; `today` = tanggal Asia/Jakarta dari server (D30 `vendor-pks`). Urutan: terminated → expired → need_renewal (`expiry < today + N bulan`) → aktif. `effective_date` tidak dipakai. Baris yang sudah diganti tidak punya status.
- Ambang Need Renewal = env `VENDOR_PKS_RENEWAL_MONTHS` (default 9, 1–24), dibaca `backend` dan `backend-exq`.
- "Masa berlaku" sekarang = panjang kontrak dalam bulan (`vendor_pks_validity_months()`, dipakai `backend/queries/vendor_pks.sql` dan `vendor_pks_report.sql`); "Monitoring" = sisa hari sampai expired.
- Daftar vendor: grup kolom Status + Monitoring per type aktif (`VendorsTable.tsx`) dan filter berpasangan `pks_type` + `pks_status`.
- Tab PKS detail vendor (`PksPanel.tsx`): kartu PKS aktif per type, riwayat, pending change, tombol "Catat PKS baru", terminasi/batalkan terminasi.
- Semua route `/api/v1/admin/vendors/*` di grup `RequireRoles("ADMIN", "ADMIN_PARAM")` (`backend/cmd/api/main.go`).
- Ekspor `vendor_pks` (`backend-exq`): satu baris per PKS aktif, filter `q`, `status` vendor, `pks_type` + `pks_status`.

### Ruang lingkup

- Layar + API **Daftar PKS** (semua vendor, satu baris per PKS termasuk riwayat), dengan filter, aksi PKS, dan ekspor.
- Kolom baru `vendor_pks.renewal_days` (ambang Need Renewal per PKS, hari) + ubah reminder langsung.
- Aturan status baru: label `belum_berlaku` dan `diganti`; Need Renewal memakai `renewal_days`.
- "Monitoring" diganti nama "Masa Berlaku" (sisa hari); kolom panjang kontrak (bulan) dihapus.
- Daftar vendor: kolom PKS berupa link; grup kolom dan filter PKS per type dihapus.
- Tab PKS detail vendor menjadi read-only.
- Akses baca untuk `ATM-SPV`.

### Tidak termasuk

- Layar Type PKS dan tabel `vendor_pks_types`: **tidak berubah** (D2).
- Model PKS append + penggantian saat disetujui (D17 `vendor-pks`): tidak berubah; tidak ada dua PKS hidup per (vendor, type).
- Impor CSV PKS (D16 `vendor-pks` tetap; spec terpisah bila perlu).
- Akses `ATM-SPV` ke tab vendor selain PKS (Info, Cabang, PIC, Harga Paket, Mapping DSR) dan ke ekspor/impor CSV vendor.
- Notifikasi/email Need Renewal.

### Area sensitif (CLAUDE.md Sec 4 #7)

- **Migrasi data**: `044` menambah kolom NOT NULL + backfill, mengganti fungsi `vendor_pks_status()`, menghapus `vendor_pks_validity_months()` bila tak ada pemakai lain, menambah menu feature + `role_permissions`.
- **Auth/RBAC**: `ATM-SPV` mendapat akses baca baru ke endpoint vendor dan PKS.
- **Deviasi Golden Rule #3**: ubah reminder berlaku langsung tanpa maker-checker (D9), pola sama dengan peran PIC vendor dan Role Management.

### Keputusan PO (grill 2026-11-19)

| ID | Pertanyaan | Keputusan |
|---|---|---|
| D1 | Dokumentasi | ✅ Spec Kiro baru; `vendor-pks-type` tetap sebagai jejak, bagian yang digantikan disebut eksplisit. |
| D2 | Type PKS | ✅ Tabel `vendor_pks_types`, layar Type PKS, dan maker-checker-nya tetap (ralat user atas usulan awal menghapusnya). |
| D3 | Riwayat di Daftar PKS | ✅ PKS lama (diganti) ikut tampil, dengan filter. Filter Status default **Terkini** (PKS yang belum diganti); pilihan lain: per status, Diganti, Semua. |
| D4 | PKS baru yang belum berlaku | ✅ Label hitung baru **Belum berlaku** bila `effective_date > Today_WIB`; PKS lama tetap Diganti saat disetujui (D17 tetap). |
| D5 | Prioritas label | ✅ terminated → belum_berlaku → expired → need_renewal → aktif. Terminasi menang atas belum berlaku; belum berlaku menang atas need renewal. |
| D6 | Ambang Need Renewal | ✅ Per PKS, dalam **hari**, kolom `renewal_days`, rentang 30–3666, wajib, default di form 270. Need renewal bila `expiry_date < Today_WIB + renewal_days`. |
| D7 | Batas terhadap masa PKS | ✅ `renewal_days` ≤ `expiry_date − effective_date` (hari); lebih besar ditolak. |
| D8 | PKS yang sudah ada | ✅ Diisi 270 oleh migrasi. |
| D9 | Ubah reminder | ✅ Langsung berlaku tanpa maker-checker (bukan perubahan isi PKS), hanya `ADMIN`/`ADMIN_PARAM`, untuk **semua** PKS termasuk yang diganti; `audit_logs` dalam tx yang sama (deviasi GR#3). |
| D10 | Env lama | ✅ `VENDOR_PKS_RENEWAL_MONTHS` dihapus dari `backend` dan `backend-exq`. |
| D11 | Nama kolom | ✅ "Monitoring" diganti nama **"Masa Berlaku"** (sisa hari, dihitung saat dibaca). Kolom panjang kontrak (bulan) dihapus dari layar dan ekspor. |
| D12 | Kolom Daftar PKS | ✅ Status, Type, Nama Vendor, Judul PKS, No. WOD, Tgl Efektif, Tgl Masa Akhir, Masa Berlaku, Reminder (hari). |
| D13 | Filter | ✅ Filter PKS pindah dari daftar vendor ke Daftar PKS: Status, Type, Vendor (`?vendor=<kode>`), cari judul/WOD. |
| D14 | Tempat aksi | ✅ Catat PKS baru, Ganti, Terminasi/batalkan, Ubah reminder **hanya** di Daftar PKS. Tab PKS detail vendor read-only. |
| D15 | Catat PKS baru | ✅ Form memilih Vendor (vendor aktif `FLM_VENDOR` saja) + Type + No. WOD + Judul + tanggal + Reminder. |
| D16 | Ganti | ✅ Aksi baris "Ganti" membuka form yang sama dengan Vendor + Type terisi dan terkunci. |
| D17 | Pending | ✅ PKS baru pending tampil sebagai baris "Menunggu persetujuan" tanpa aksi; baris yang terkena change pending diberi penanda; aksi untuk (vendor, type) itu nonaktif dengan alasan tertulis. |
| D18 | Halaman | ✅ Menu Pengaturan → Daftar PKS, `/settings/admin/pks`, mendukung `?vendor=<kode>`. |
| D19 | Daftar vendor | ✅ Grup kolom PKS per type dan filter `pks_type`+`pks_status` dihapus; satu kolom **PKS**: "N PKS" / "Belum ada" sebagai link ke tab PKS vendor; vendor INTERNAL "-" tanpa link. |
| D20 | Tab PKS vendor | ✅ Tabel yang sama (komponen sama) berisi PKS vendor ini, read-only, plus link "Kelola di Daftar PKS". Teks "Need Renewal mulai 9 bulan…" dihapus. |
| D21 | Akses lihat | ✅ Daftar PKS + ekspor: `ADMIN`, `ADMIN_PARAM`, `ATM-SPV`. Aksi PKS: hanya `ADMIN`/`ADMIN_PARAM`. |
| D22 | ATM-SPV di halaman vendor | ✅ Daftar vendor + tab PKS read-only; tab lain disembunyikan dan endpoint-nya tetap tertutup untuk ATM-SPV. |
| D23 | Klik baris Daftar PKS | ✅ Membuka tab PKS vendor tersebut, untuk ketiga role. |
| D24 | Menu | ✅ Menu feature baru Daftar PKS di-seed untuk `ADMIN`, `ADMIN_PARAM`, `ATM-SPV` di `044`, diatur lewat Role Management. |
| D25 | Ekspor | ✅ Ekspor `vendor_pks` dipindah ke Daftar PKS, filter sama, + kolom Reminder (hari). |
| D26 | Input 12 PKS | ✅ Boleh sekarang lewat tab PKS yang ada (task 10 `vendor-pks-type`); `044` mengisi reminder 270. |

### Asumsi (perlu konfirmasi PO saat menyetujui requirements)

| ID | Asumsi | Criteria |
|---|---|---|
| A1 | Nilai filter `belum_diisi` (vendor tanpa PKS type tertentu) hilang dari filter dan ekspor, karena Daftar PKS hanya berisi baris PKS; vendor tanpa PKS terlihat dari kolom "Belum ada" di daftar vendor. | 4.4, 10.2 |
| A2 | Baris pending create tidak ikut ekspor (belum menjadi PKS). | 10.1 |
| A3 | Daftar PKS dipaginasi server seperti daftar vendor; urutan nama vendor, kode type, tanggal efektif terbaru dulu. | 4.6 |
| A4 | Cari (`q`) mencocokkan judul PKS dan No. WOD (case-insensitive, sebagian). | 4.4 |
| A5 | Perubahan kontrak API daftar vendor (field `pks` per type diganti `pks_count`) dirilis bersama CompanyPortal. | 7.1 |

## Glossary

- **PKS_Record**: satu baris `vendor_pks`.
- **Current_PKS**: PKS_Record dengan `replaced_at IS NULL` (paling banyak satu per vendor+type).
- **Replaced_PKS**: PKS_Record dengan `replaced_at` terisi.
- **Pending_PKS_Create**: change `vendor_pks` op `create` berstatus pending di `master_data_change_requests`.
- **Active_PKS_Type**: baris `vendor_pks_types` dengan `is_active = true`.
- **Today_WIB**: tanggal kalender Asia/Jakarta di server saat request atau job ekspor diproses (D30 `vendor-pks`).
- **Renewal_Days**: `vendor_pks.renewal_days`, ambang Need Renewal per PKS_Record dalam hari.
- **Masa_Berlaku**: sisa hari `expiry_date − Today_WIB` sebuah PKS_Record, dihitung saat dibaca (menggantikan istilah "Monitoring"; istilah Masa_Berlaku bulan di `vendor-pks` tidak dipakai lagi).
- **PKS_Status**: label hitung per PKS_Record: `diganti`, `terminated`, `belum_berlaku`, `expired`, `need_renewal`, `aktif`.
- **Master_Data_Role**: `ADMIN` atau `ADMIN_PARAM`.
- **PKS_Viewer_Role**: `ADMIN`, `ADMIN_PARAM`, atau `ATM-SPV`.
- **Vendor_FLM** / **Vendor_Internal**: vendor `kind = FLM_VENDOR` / `INTERNAL`.
- **PksStatusRule**: aturan status di DB (fungsi `vendor_pks_status()` versi `044`).
- **PksListAPI**: endpoint baca baru untuk Daftar PKS.
- **PksReminderAPI**: endpoint baru untuk mengubah Renewal_Days.
- **VendorPksAPI**: endpoint PKS yang ada di `/api/v1/admin/vendors/{vendorID}/pks*` (create, terminate, un-terminate, detail).
- **VendorAdminAPI**: endpoint daftar/detail vendor yang ada (`/api/v1/admin/vendors`).
- **ExportAPI**: ekspor kind `vendor_pks` di `backend-exq`.
- **Migration_044**: migrasi `044_*.sql`.
- **PksListScreen**: layar baru Daftar PKS di CompanyPortal (`/settings/admin/pks`).
- **PksTable**: komponen tabel PKS yang dipakai PksListScreen dan VendorPksTab.
- **VendorAdminScreen**: daftar dan detail vendor yang ada (`/settings/admin/vendors`).
- **VendorPksTab**: tab PKS di detail vendor (`/settings/admin/vendors/{id}`).

## Requirements

### Requirement 1: Reminder per PKS

**User Story:** Sebagai tim OPS, saya ingin mengatur berapa hari sebelum expired sebuah PKS mulai berstatus Need Renewal, supaya pengingat sesuai kebutuhan tiap kontrak.

#### Acceptance Criteria

1. THE Migration_044 SHALL add `vendor_pks.renewal_days integer NOT NULL` with a CHECK of 30–3666 and a CHECK that `renewal_days <= expiry_date - effective_date`, and SHALL set `renewal_days = 270` on every existing PKS_Record before the NOT NULL is applied (D6, D7, D8).
2. WHEN a PKS create change is submitted, THE VendorPksAPI SHALL require `renewal_days` as an integer 30–3666 and not greater than `expiry_date - effective_date` in days, and SHALL reject a missing or invalid value with 422 and a field error (D6, D7).
3. WHEN a PKS create change is approved, THE VendorPksAPI SHALL store the submitted `renewal_days` on the new PKS_Record together with the other fields (D6).
4. THE PKS create form SHALL show a Reminder (hari) field prefilled with 270 that cannot be left empty (D6).
5. THE `backend` and `backend-exq` SHALL stop reading `VENDOR_PKS_RENEWAL_MONTHS`, and it SHALL be removed from both `.env.example` files (D10).

### Requirement 2: Ubah reminder langsung

**User Story:** Sebagai admin, saya ingin mengubah reminder sebuah PKS tanpa approval, karena reminder tidak mengubah isi kontrak.

#### Acceptance Criteria

1. WHEN a Master_Data_Role user submits a new `renewal_days` for a PKS_Record, THE PksReminderAPI SHALL update only `renewal_days` of that PKS_Record immediately, without `master_data_change_requests` or `approval_requests` (D9).
2. THE PksReminderAPI SHALL accept the change for any PKS_Record, including Replaced_PKS and terminated PKS (D9).
3. THE PksReminderAPI SHALL validate `renewal_days` with the rules of Req 1.2 against that PKS_Record's own dates, and reject an invalid value with 422.
4. THE PksReminderAPI SHALL write an `audit_logs` entry (actor, PKS_Record id, `renewal_days` before and after, time, ip) in the same transaction as the update; IF the audit write fails, THEN THE PksReminderAPI SHALL roll back the update (D9).
5. IF the caller is not a Master_Data_Role user, THEN THE PksReminderAPI SHALL reject the request with 403, checked at the route and in the service layer.
6. IF the PKS_Record does not exist, THEN THE PksReminderAPI SHALL return 404.
7. THE PksReminderAPI SHALL leave every other column of the PKS_Record unchanged; `pks_name`, dates, `pks_type_id`, and `wod_number` stay never-updated.

### Requirement 3: Aturan status PKS

**User Story:** Sebagai tim OPS, saya ingin label PKS sesuai keadaan kontrak (sudah diganti, belum berlaku, perlu perpanjangan), supaya monitoring tidak menyesatkan.

#### Acceptance Criteria

1. THE PksStatusRule SHALL label a PKS_Record in this order (D4, D5, D6): Replaced_PKS → `diganti`; `terminated_at` set → `terminated`; `effective_date` > Today_WIB → `belum_berlaku`; `expiry_date` < Today_WIB → `expired`; `expiry_date` < Today_WIB + `renewal_days` days → `need_renewal`; otherwise → `aktif`.
2. WHILE `expiry_date` equals Today_WIB, THE PksStatusRule SHALL treat the PKS_Record as not `expired` (D20 `vendor-pks` tetap).
3. THE PksStatusRule SHALL take Today_WIB from the server (Go) as today does, never from the client.
4. THE Migration_044 SHALL replace `vendor_pks_status()` with the rule of 3.1, and every caller in `backend` and `backend-exq` SHALL use the new rule so the list, detail, filter, and export always agree.
5. THE system SHALL compute Masa_Berlaku as `expiry_date − Today_WIB` in days on read, never store it, and show it as "N hari", "lewat N hari", or "-" (for `diganti` and `terminated`) under the label "Masa Berlaku" (D11).
6. THE system SHALL stop returning and showing the contract length in months; IF `vendor_pks_validity_months()` has no remaining caller, THEN THE Migration_044 SHALL drop it (D11).

### Requirement 4: API Daftar PKS

**User Story:** Sebagai tim OPS, saya ingin melihat semua PKS semua vendor dalam satu daftar, supaya bisa memantau PKS lintas vendor.

#### Acceptance Criteria

1. THE PksListAPI SHALL return one row per PKS_Record with: id, vendor id, vendor code, vendor name, PKS type code, `wod_number`, `pks_name`, `effective_date`, `expiry_date`, `renewal_days`, Masa_Berlaku, PKS_Status, and the pending change of that (vendor, type) if any (D12, D17).
2. WHEN the filter Status is `terkini` or `semua`, THE PksListAPI SHALL also return each Pending_PKS_Create as a row with status `menunggu_persetujuan`, its submitted fields, and no id (D17).
3. THE PksListAPI SHALL default the Status filter to `terkini` (Current_PKS only) and accept `terkini`, `semua`, `diganti`, `aktif`, `need_renewal`, `belum_berlaku`, `expired`, `terminated` (D3).
4. THE PksListAPI SHALL accept the optional filters `pks_type` (a PKS type code), `vendor` (a vendor code), and `q` (case-insensitive partial match on `pks_name` or `wod_number`), combined with AND, and reject an unknown filter value with 422 (D13, A1, A4).
5. THE PksListAPI SHALL never return Vendor_Internal rows (they have no PKS).
6. THE PksListAPI SHALL paginate on the server and order rows by vendor name, type code, then `effective_date` newest first, and return `total` for the filtered rows (A3).
7. THE PksListAPI SHALL read from the replica pool.
8. IF the caller is not a PKS_Viewer_Role user, THEN THE PksListAPI SHALL reject the request with 403, checked at the route and in the service layer (D21).

### Requirement 5: Layar Daftar PKS

**User Story:** Sebagai tim OPS, saya ingin layar Daftar PKS dengan kolom seperti daftar PKS saya, supaya data mudah dicocokkan dan dipantau.

#### Acceptance Criteria

1. THE PksListScreen SHALL be reachable at `/settings/admin/pks` from menu Pengaturan → Daftar PKS, and SHALL be shown only to roles granted its menu feature (D18, D24).
2. THE PksListScreen SHALL show the columns Status, Type, Nama Vendor, Judul PKS, No. WOD, Tgl Efektif, Tgl Masa Akhir, Masa Berlaku, Reminder (hari) (D12).
3. THE PksListScreen SHALL offer the filters Status (default Terkini), Type, Vendor, and a search box for judul/WOD, keep them in the URL, and apply `?vendor=<kode>` on load (D3, D13, D18).
4. THE PksListScreen SHALL show each status as a badge with icon and text, `belum_berlaku` and `diganti` included, never by color alone.
5. WHEN a row of a PKS_Record is activated (click or Enter), THE PksListScreen SHALL open the VendorPksTab of that vendor (D23).
6. THE PksListScreen SHALL show a Pending_PKS_Create row as "Menunggu persetujuan" without actions, and mark a row whose (vendor, type) has a pending change with the pending operation as text (D17).
7. THE PksListScreen and PksTable SHALL be usable by keyboard, label every field and filter for screen readers, use `tabular-nums` for dates and day counts, and use no em dash in UI copy.

### Requirement 6: Aksi PKS di Daftar PKS

**User Story:** Sebagai admin, saya ingin mencatat, mengganti, menterminasi, dan mengubah reminder PKS dari satu layar, supaya alur kerja PKS ada di satu tempat.

#### Acceptance Criteria

1. WHERE the user is a Master_Data_Role user, THE PksListScreen SHALL offer "Catat PKS baru", and per row "Ganti", "Terminasi" / "Batalkan terminasi", and "Ubah reminder"; for any other role these controls SHALL not be rendered (D14, D21).
2. THE "Catat PKS baru" form SHALL ask Vendor (active Vendor_FLM only), Type (Active_PKS_Type only), No. WOD, Judul PKS, Tgl Efektif, Tgl Masa Akhir, and Reminder (hari) and submit through the existing VendorPksAPI create (maker-checker, 202 shown as "menunggu persetujuan") (D15).
3. THE "Ganti" action SHALL open the same form with Vendor and Type prefilled and locked to the row's values, and submit as a replacement of that Current_PKS (D16).
4. THE "Terminasi" / "Batalkan terminasi" actions SHALL use the existing VendorPksAPI terminate/un-terminate (maker-checker, reason rules unchanged), and SHALL be offered only on a Current_PKS.
5. THE "Ubah reminder" action SHALL be offered on every PKS_Record row and call the PksReminderAPI; success SHALL refresh the row without a "menunggu persetujuan" message (D9).
6. WHILE a (vendor, type) has a pending `vendor_pks` change, THE PksListScreen SHALL disable Catat (for that vendor+type), Ganti, and Terminasi/Batalkan for it and show the reason as text; "Ubah reminder" SHALL stay available (D17).
7. THE VendorPksAPI write endpoints and PksReminderAPI SHALL stay Master_Data_Role only, checked at the route and in the service layer (D21).

### Requirement 7: Daftar vendor dengan kolom PKS

**User Story:** Sebagai tim OPS, saya ingin daftar vendor yang ringkas dengan satu link PKS, supaya detail PKS dilihat di tempatnya.

#### Acceptance Criteria

1. THE VendorAdminAPI SHALL return per vendor `pks_count` = the number of Current_PKS of that vendor, and SHALL stop returning the per-type `pks` list (status + Monitoring per type) (D19, A5).
2. THE VendorAdminAPI SHALL stop accepting the `pks_type` and `pks_status` list filters (D13, D19).
3. THE VendorAdminScreen SHALL remove the per-type PKS column groups and the PKS type + status filter, and add one column "PKS" (D19).
4. THE "PKS" column SHALL show "N PKS" for a Vendor_FLM with N ≥ 1 Current_PKS and "Belum ada" when N = 0, both as a link to that vendor's VendorPksTab, and "-" without a link for a Vendor_Internal (D19).
5. THE Director_PIC fields and every other vendor list column, filter, and behaviour SHALL stay unchanged.

### Requirement 8: Tab PKS vendor read-only

**User Story:** Sebagai pengguna, saya ingin melihat semua PKS satu vendor dari halaman vendornya, supaya tidak perlu mencari di daftar besar.

#### Acceptance Criteria

1. THE VendorPksTab SHALL show the PksTable filtered to that vendor, with the same columns and Status filter (default Terkini) as the PksListScreen (D20).
2. THE VendorPksTab SHALL offer no write action (no Catat, Ganti, Terminasi, Batalkan, Ubah reminder) for any role (D14).
3. THE VendorPksTab SHALL show a link "Kelola di Daftar PKS" to `/settings/admin/pks?vendor=<kode>`, shown only to Master_Data_Role users (D20).
4. THE VendorPksTab SHALL drop the text "Need Renewal mulai 9 bulan sebelum tanggal expired" (D20).

### Requirement 9: Akses ATM-SPV

**User Story:** Sebagai ATM-SPV, saya ingin melihat PKS vendor tanpa bisa mengubahnya, supaya bisa memantau kontrak vendor.

#### Acceptance Criteria

1. THE Migration_044 SHALL add a menu feature for Daftar PKS under Pengaturan and grant it to `ADMIN`, `ADMIN_PARAM`, and `ATM-SPV` in `role_permissions` (D24).
2. THE VendorAdminAPI SHALL allow `ATM-SPV` to read the vendor list, the vendor detail header, and the vendor's PKS list, and SHALL keep every other vendor endpoint (cabang, PIC, vault, harga paket, mapping DSR, ekspor/impor CSV vendor, all writes) Master_Data_Role only, checked at the route and in the service layer (D22).
3. WHEN an `ATM-SPV` user opens a vendor detail, THE VendorAdminScreen SHALL show only the PKS tab and hide every write control on the page (D22).
4. THE system SHALL reject any write by an `ATM-SPV` user to PKS, PKS reminder, or vendor data with 403 (D21, D22).

### Requirement 10: Ekspor PKS

**User Story:** Sebagai tim OPS, saya ingin mengekspor Daftar PKS sesuai filter yang saya lihat, supaya laporan cocok dengan layar.

#### Acceptance Criteria

1. THE ExportAPI SHALL produce the `vendor_pks` export with one row per PKS_Record matching the PksListAPI filters (Status incl. `terkini`/`semua`/`diganti`, `pks_type`, `vendor`, `q`), excluding Pending_PKS_Create rows (D25, A2).
2. THE ExportAPI SHALL stop accepting the `pks_status` value `belum_diisi` and the vendor-level `status` filter of `vendor-pks-type` Req 6 (A1).
3. THE ExportAPI SHALL write the columns Kode Vendor, Nama Perusahaan, Nama Direktur Utama, Telepon Direktur Utama, Email Direktur Utama, Type PKS, No. WOD, Nama PKS, Tanggal Efektif PKS, Tanggal Expired PKS, Masa Berlaku (hari), Reminder (hari), Status PKS, every text cell through `CSVSafe` (D11, D25).
4. THE PksListScreen SHALL offer the export button with the current filters; THE VendorAdminScreen SHALL no longer offer the PKS export (D25).
5. THE ExportAPI SHALL allow PKS_Viewer_Role users, else 403, and keep the existing audit and replica reads (D21).

### Requirement 11: Tidak berubah

**User Story:** Sebagai engineer, saya ingin perubahan ini tidak merusak perilaku PKS dan vendor yang sudah ada.

#### Acceptance Criteria

1. THE system SHALL keep `vendor_pks_types`, the Type PKS screen, and its maker-checker unchanged (D2).
2. THE system SHALL keep the PKS create/replace/terminate/un-terminate maker-checker, the one-Current_PKS-per-(vendor, type) rule, the WOD uniqueness, and immediate replacement on approval (D17 `vendor-pks`) unchanged.
3. THE system SHALL never hard-delete a PKS_Record and never UPDATE any PKS_Record column other than `renewal_days`, `replaced_at`, `replaced_by_pks_id`, and the terminate columns.
4. THE vendor CSV export/import (`vendor-pks-type` Req 7) SHALL stay unchanged.
