# Requirements Document — Standarisasi Region

> Status: accepted 2026-11-18 (PO, "Setuju, lanjut") — Introduction + Requirements 1–8.

## Introduction

### Masalah

Kata-kata pemohon (user, 2026-11-17): halaman `/settings/admin/regions` di-revamp. "Jumlah Lokasi" diganti "Jumlah ATM" dan jumlah per cabang vendor (`vendor_branches`, bukan per vault). Sebelum itu, **kode dan nama region distandarkan agar bisa dipakai ATM dan cabang vendor**.

Sekarang CROWN punya dua daftar region yang tidak saling terhubung:
- **Region ATM**: tabel `regions` (13 kode wilayah besar). Dipakai ATM lewat `atms.location_id → locations.region_id`.
- **Region cabang vendor**: kolom teks bebas `vendor_branches.region_code` (34 kode, sebagian wilayah besar dan sebagian per kota) dan `vendor_branches.region` (nama dari `MASTER_ATM_ESQ.FLMVendorRegion`). Tidak ada FK ke `regions`.

Akibatnya angka per region (ATM dan cabang vendor) tidak bisa dihitung dengan satu acuan. Kode yang sama juga ditulis beda format (`JKT_WEST` vs `JKTBAR`), dan nama region cabang kotor ("Jakarta UtaraÂ", "Jawa barat" vs "Jawa Barat").

Spec terkait yang sudah selesai: `.kiro/specs/.done/region-management/` (CRUD region). Spec ini tidak membukanya ulang; yang diubah hanya hal yang disebut di bawah.

### Kondisi kode dan data saat ini (dicek 2026-11-17, DB dev)

**Tabel `regions`** (`001_baseline_schema.sql`, soft-disable `018`)
- Kolom `id, code (UNIQUE), region (nullable), created_at, updated_at, is_active, deleted_at`.
- `code` tidak bisa diubah setelah dibuat (Region Management Req 3.1/3.2). Admin region = deviasi GR#3 (langsung berlaku + audit dalam tx yang sama).
- 29 baris: 13 region asli (`BALI_NUSRA, INDTIM, JABAR, JATENG, JATIM, JKT_CENTRAL, JKT_EAST, JKT_NORTH, JKT_SOUTH, JKT_WEST, KALIMANTAN, SUMATERA, TANGERANG`) + 16 sisa data integration test `ITESTNUM-…` / "Integration Test Region".
- Halaman admin menampilkan `location_count` = `COUNT(locations)` per region (`backend/queries/regions_admin.sql`). Dasar yang sama dipakai untuk menolak disable region yang masih dirujuk lokasi.

**ATM → region**
- `atms.location_id` NOT NULL → `locations.region_id` NOT NULL → `regions`. Semua 1.920 ATM dev punya region. Contoh: JKT_EAST 342, JATENG 237, SUMATERA 229.

**Cabang vendor → region**
- `vendor_branches.region_code` text NULL, tanpa FK. Diisi lewat master-data maker-checker (spec `replenish-ticket`, migrasi `026`) dan CSV cabang vendor. NULL = request untuk cabang itu ditolak 422.
- `vendor_branches.location_id` ada tapi **kosong di 407/407 cabang**.
- 407 cabang: 51 tanpa kode, sisanya 34 kode: `ACEH, AMBON, BALI, BALNUS, BJM, BPN, INDTIM, JABAR, JAMBI, JATENG, JATIM, JEMBER, JKT, JKTBAR, JKTPST, JKTSEL, JKTTIM, JKTUTR, KDI, KUPANG, LAMPUNG, MADIUN, MDN, MDO, MKS, MTR, PGK, PKU, PKY, PLG, PNK, SMD, SUMATERA, TNJ`.
- Hanya 5 kode yang sama persis dengan `regions.code` (INDTIM, JABAR, JATENG, JATIM, SUMATERA).

**Pemakai `region_code` cabang vendor** (kode Go non-test)
- Nomor Vendor Request `REP-<prefix vendor>-<region_code>-<YYYYMMDD>-<NNN>` + snapshot `vendor_requests.region_code` + counter `vendor_request_number_seq.region_code` (`vendor_request_create.go`, `vendor_request.go`, `vendor_request_actions.go`). Nomor lama tidak pernah ditulis ulang.
- Pemenuhan vault ACM: plan dimiliki Area ACM dari `acm_area_regions.region_code` (34 baris dev, semua ke "Area ACM Dev"); tier vault 1/2/3 dibanding region request (`vault_plan.go`, `vault_plan_actions.go`, `vault_allocation.go`).
- Cash count: `cash_count_area_regions.region_code`, `cash_count_pics.region_code` untuk COORDINATOR (`cash_count_area_admin.go`, `cash_count_pic_admin.go` + applier + impor CSV). Dev: 0 baris.
- Laporan: DSR telat (`dsr_late_report.go`), replenish/klasifikasi (`replenishment_read.go`, filter `region_code`), ekspor master data cabang vendor (kolom `region`, `region_code`).
- Validasi impor CSV master data (`masterdata_import_validate.go`, `masterdata_import_confirm.go`), applier cabang vendor, `region_admin.go`, `acm_area_admin.go`.
- Python: tidak ada kode produksi yang membaca `region_code` (hanya test `dsr_late_notify`).

### Ruang lingkup (usulan, menunggu keputusan PO di Open Questions)

- Satu daftar region baku (kode + nama) yang dipakai **ATM dan cabang vendor**.
- Cabang vendor merujuk region baku itu dengan relasi yang dijaga DB, sehingga kode liar tidak bisa masuk lagi.
- Migrasi data kode cabang vendor yang ada ke region baku (tabel pemetaan disetujui PO).
- Pemakai `region_code` (nomor request, Area ACM, cash count, laporan, CSV) tetap jalan dengan kode baku.
- Halaman admin Region: kolom "Jumlah Lokasi" diganti **Jumlah ATM** dan **Jumlah Cabang Vendor**.

### Tidak termasuk

- Jumlah vault per region (user 2026-11-17: cukup per cabang vendor).
- Mengubah struktur `locations` atau `atms`.
- Menulis ulang nomor Vendor Request, tiket, atau snapshot `vendor_requests.region_code` yang sudah terbit.
- Pembersihan data uji `ITESTNUM-…` di DB dev + perbaikan cleanup `vendor_request_number_integration_test.go` (error cleanup diabaikan `_, _ =`). Ditangani sebagai bug fix terpisah di `.claude/bugfixes.md`.
- Aturan tier vault baru di luar efek pemetaan region; pindah buku antar escrow.

### Area sensitif (CLAUDE.md Sec 4 #7)

- **Migrasi data + skema**: kode region di `vendor_branches` (dan kemungkinan `regions.code`, `acm_area_regions`, `cash_count_*`) diubah oleh migrasi. Perlu tabel pemetaan yang disetujui PO dan rencana rollback.
- **Aturan bisnis ikut bergeser**: tier vault (region sama vs beda) dan Area ACM pemilik plan ditentukan oleh region. Kalau granularitas berubah, tier untuk request baru bisa berubah.
- **Maker-checker / deviasi GR#3**: `vendor_branches.region_code` lewat maker-checker; admin `regions` dan Area ACM = deviasi GR#3. Spec harus menyatakan jalur mana yang dipakai untuk perubahan baru.
- **Request yang sedang berjalan**: request/plan/tiket yang belum selesai memegang `region_code` lama.

## Glossary

- **Region_Baku**: satu baris aktif `regions` setelah standarisasi: wilayah besar, kode format cabang vendor (Q2), nama ESQ (Q3).
- **Region_Code_Format**: aturan bentuk kode Region_Baku (huruf besar tanpa garis bawah, gaya `JKTBAR`); pola persis ditetapkan di design.
- **Region_City**: satu baris tabel kota baru (nama final di design) di bawah tepat satu Region_Baku: kode, nama, status aktif (Q1).
- **Region_Mapping**: tabel pemetaan yang disetujui PO di `region-mapping.md` (bagian A–D, keputusan M1–M4).
- **Region_Migration**: migrasi bernomor berikutnya (`042_…` atau setelahnya) yang dijalankan lewat `cmd/migrate up`.
- **Vendor_Branch_Region**: `vendor_branches.region_code` setelah diberi FK ke `regions.code` (Q5a).
- **Unfinished_Vendor_Request**: Vendor Request yang belum berada di status akhir (daftar status akhir ditetapkan di design).
- **Region_Admin_Role**: `ADMIN` atau `ADMIN_PARAM`, role yang sudah menjaga layar Region (`region_admin.go`).
- **RegionAdminAPI**: endpoint `/api/v1/admin/regions*` di ATM `backend` (JSON datar).
- **RegionsPage**: layar CompanyPortal `/settings/admin/regions`.
- **VendorBranchAPI**: jalur maker-checker cabang vendor yang ada (`vendor_branch_admin.go` → `masterdata_applier_vendor_branch.go`) beserta impor CSV cabang vendor.
- **Active_ATM**: baris `atms` dengan `is_active = true`; region-nya `locations.region_id` dari `atms.location_id`.
- **Active_Vendor_Branch**: baris `vendor_branches` dengan `is_active = true`.

## Requirements

### Requirement 1: Region baku

**User Story:** Sebagai admin parameter, saya ingin satu daftar region baku yang sama untuk ATM dan cabang vendor, supaya angka dan aturan per region memakai acuan yang sama.

#### Acceptance Criteria

1. WHEN the Region_Migration runs, THE Region_Migration SHALL change `code` and `region` of every `regions` row listed in Region_Mapping part A in place, keeping its `id`, so `locations` and `atms` rows are not modified (Q6).
2. WHEN Region_Mapping decides that a region is merged into another (M1 option b), THE Region_Migration SHALL move the affected `locations.region_id` to the target Region_Baku and soft-disable the emptied row (`is_active = false`, `deleted_at` set), never deleting it.
3. WHEN a Region_Admin_Role user creates a region, THE RegionAdminAPI SHALL reject a code that does not match the Region_Code_Format with 422 on field `code`.
4. THE RegionAdminAPI SHALL keep rejecting a change of an existing region `code` (Region Management Req 3.1/3.2); only the Region_Migration changes codes, once.
5. WHEN the Region_Migration finishes, THE database SHALL contain no active `regions` row whose code violates the Region_Code_Format, except rows explicitly kept by PO in Region_Mapping (M2).

### Requirement 2: Pencatatan per kota

**User Story:** Sebagai admin parameter, saya ingin kota tercatat di bawah wilayahnya, supaya kode per kota yang dulu dipakai sebagai region cabang vendor tidak hilang.

#### Acceptance Criteria

1. THE database SHALL store each Region_City with a unique code, a name, and exactly one Region_Baku.
2. WHEN the Region_Migration runs, THE Region_Migration SHALL insert the Region_City rows listed in Region_Mapping part B.
3. IF a Region_City would reference a region that does not exist, THEN THE database SHALL reject it.
4. THE system SHALL provide no screen, API, or CSV for Region_City in this feature (table only, Q1); a later feature adds them.

### Requirement 3: Cabang vendor merujuk region baku

**User Story:** Sebagai admin parameter, saya ingin kode region cabang vendor hanya bisa berisi Region_Baku, supaya kode liar tidak masuk lagi.

#### Acceptance Criteria

1. THE database SHALL enforce that every non-NULL Vendor_Branch_Region equals an existing `regions.code` (FK, Q5a).
2. WHEN the Region_Migration runs, THE Region_Migration SHALL set Vendor_Branch_Region of every `vendor_branches` row from Region_Mapping parts B, C, and D, including the 51 branches without a code (Q8).
3. WHEN a Region_Admin_Role or master-data user submits a vendor branch create or update through the VendorBranchAPI with a `region_code` that is not an active Region_Baku, THE VendorBranchAPI SHALL reject it with 422 on field `region_code` and stage nothing.
4. IF the region of a staged vendor branch change is disabled between submit and approve, THEN THE VendorBranchAPI applier SHALL not apply the change and SHALL report it through the existing apply-failure path (409).
5. WHEN a vendor branch CSV import row has a `region_code` that is not an active Region_Baku, THE VendorBranchAPI SHALL report it in the dry-run as an error on field `region_code`.
6. THE VendorBranchAPI SHALL keep staging vendor branch changes through maker-checker (no Golden Rule #3 deviation).

### Requirement 4: Nama region cabang dari region baku

**User Story:** Sebagai pengguna laporan, saya ingin nama region cabang vendor selalu sama dengan nama Region_Baku, supaya tidak ada dua versi nama.

#### Acceptance Criteria

1. WHEN the Region_Migration runs, THE Region_Migration SHALL drop column `vendor_branches.region` after Vendor_Branch_Region is filled (Q5b).
2. WHEN a screen, filter, or export shows the region name of a vendor branch, THE system SHALL read it from `regions.region` through Vendor_Branch_Region; this covers the FLM Vendor Region column and filter of the Vendor Request recommendation, the vendor vault admin list, and the vendor branch master-data CSV export.
3. WHEN the FLM Vendor Region filter "tanpa region" is used, THE system SHALL return branches whose Vendor_Branch_Region is NULL.
4. THE vendor branch master-data CSV export SHALL keep its `region` column, filled from `regions.region`; THE vendor branch CSV import SHALL accept files with a `region` column and ignore its value.

### Requirement 5: Pemakai kode region tetap jalan

**User Story:** Sebagai operator ATM dan ACM, saya ingin nomor request, Area ACM, dan tier vault langsung memakai Region_Baku setelah migrasi, supaya alur harian tidak terputus.

#### Acceptance Criteria

1. IF any Unfinished_Vendor_Request exists when the Region_Migration starts, THEN THE Region_Migration SHALL stop without changing anything and report the count (Q7: dev is cleared first, with the user's OK).
2. WHEN the Region_Migration runs, THE Region_Migration SHALL rewrite `acm_area_regions.region_code`, `cash_count_area_regions.region_code`, and `cash_count_pics.region_code` from Region_Mapping, merging rows that become duplicates into one row per region.
3. WHEN a Vendor Request is created after the Region_Migration, THE system SHALL number it and resolve its Area ACM and vault tiers with the Region_Baku code of its branches.
4. THE Region_Migration SHALL NOT rewrite issued request numbers, ticket numbers, or `vendor_requests.region_code` snapshots of finished requests.

### Requirement 6: Jumlah ATM dan Jumlah Cabang Vendor di halaman Region

**User Story:** Sebagai admin parameter, saya ingin melihat jumlah ATM aktif dan cabang vendor aktif per region, menggantikan Jumlah Lokasi.

#### Acceptance Criteria

1. WHEN a Region_Admin_Role user lists or opens regions, THE RegionAdminAPI SHALL return per region the number of Active_ATM and the number of Active_Vendor_Branch (Q9), read from the replica.
2. THE RegionsPage SHALL show columns "Jumlah ATM" and "Jumlah Cabang Vendor" and SHALL NOT show "Jumlah Lokasi".
3. THE RegionsPage SHALL show both counts as right-aligned `tabular-nums` integers (CLAUDE.md Sec 13).

### Requirement 7: Disable region yang masih dipakai

**User Story:** Sebagai admin parameter, saya ingin region yang masih dipakai lokasi atau cabang vendor aktif tidak bisa dinonaktifkan, supaya tidak ada request baru dengan region nonaktif.

#### Acceptance Criteria

1. IF a Region_Admin_Role user disables a region that is still referenced by a `locations` row, THEN THE RegionAdminAPI SHALL reject it with 409 (existing rule).
2. IF a Region_Admin_Role user disables a region that is still referenced by an Active_Vendor_Branch, THEN THE RegionAdminAPI SHALL reject it with 409 and an error code distinct from the location case (Q10).
3. THE RegionAdminAPI SHALL check both rules in the service layer, in the same transaction as the disable and its audit entry.

### Requirement 8: Migrasi aman dan bisa dijelaskan

**User Story:** Sebagai engineer, saya ingin migrasi region bisa diulang, diperiksa, dan dibatalkan, karena ini migrasi data (CLAUDE.md Sec 4 #7).

#### Acceptance Criteria

1. THE Region_Migration SHALL contain Region_Mapping as data inside the migration file and apply it in one transaction; any failure SHALL leave the database unchanged.
2. IF a `vendor_branches.region_code` or `acm_area_regions.region_code` value is not covered by Region_Mapping, THEN THE Region_Migration SHALL stop without changing anything and name the value.
3. THE spec SHALL ship a rollback script that restores the previous codes, names, and `vendor_branches.region` text from a backup taken by the migration.
4. WHEN the Region_Migration is applied to dev, THE CLAUDE.md Sec 3 table map and `docs/data-map.md` SHALL be updated in the same change.

## Open Questions

Q1–Q10 dijawab PO 2026-11-17 (JABODETABEK sebagai region dibatalkan PO 2026-11-18, lihat Q1). Sisa langkah sebelum migrasi ditulis: draf pemetaan (Q4/Q8) disiapkan tim dan disetujui PO.

1. **Granularitas region baku**: 13 wilayah besar seperti `regions` sekarang, per kota seperti sebagian kode cabang, atau dua tingkat (wilayah → kota)?
   ✅ DIPUTUSKAN PO 2026-11-17: **wilayah besar** = region baku. Pencatatan **per kota** juga disiapkan (tabel baru bila belum ada di DB), di bawah wilayah.
   ❌ DIBATALKAN PO 2026-11-18: JABODETABEK sebagai region baku sendiri sempat diputuskan lalu dibatalkan. JABODETABEK **bukan** region (belum ada di DB maupun kode cabang); region Jakarta dan Tangerang tetap terpisah.
2. **Format kode baku**: ikut `regions` (`JKT_WEST`), ikut cabang (`JKTBAR`), atau daftar baru dari sumber resmi (sebutkan sumbernya)?
   ✅ DIPUTUSKAN PO 2026-11-17: format cabang vendor (**`JKTBAR`**). Konsekuensi: `regions.code` yang sekarang (`JKT_WEST`, dst.) ikut dipetakan ke format ini (lihat Q6).
3. **Sumber nama**: nama region baku diambil dari mana? `regions.region`, `MASTER_ATM_ESQ.FLMVendorRegion`, atau daftar dari bisnis?
   ✅ DIPUTUSKAN PO 2026-11-17: **nama ESQ** (`vendor_branches.region` / `MASTER_ATM_ESQ.FLMVendorRegion`), dirapikan sekali saat migrasi (typo, spasi/karakter aneh, kapitalisasi). Wilayah tanpa nama ESQ (mis. `KALIMANTAN`, `TANGERANG`) diusulkan di draf pemetaan dan disetujui PO.
4. **Tabel pemetaan**: siapa yang menyiapkan pemetaan 34 kode cabang lama → kode baku? Kasus yang harus diputuskan, antara lain: `JKT` (tanpa wilayah), `MTR` (NTB: BALI_NUSRA atau INDTIM?), kota Kalimantan (`BJM, BPN, PNK, SMD, PKY`: KALIMANTAN atau INDTIM?), `TANGERANG` (belum ada cabang).
   ✅ DIPUTUSKAN PO 2026-11-17: **tim (AI) menyiapkan draf pemetaan**, PO menyetujui sebelum migrasi ditulis.
5. **Relasi**: cabang vendor merujuk `regions` lewat `region_id` (FK) atau `region_code` (FK ke `regions.code`)? Kolom teks `vendor_branches.region` dihapus, dipertahankan sebagai info ESQ, atau diganti nama dari `regions`?
   - 5a ✅ DIPUTUSKAN PO 2026-11-17: **FK `vendor_branches.region_code` → `regions.code`** (kolom yang ada, tanpa `region_id` baru). Pemakai `region_code` tidak berubah.
   - 5b ✅ DIPUTUSKAN PO 2026-11-17: **hapus kolom `vendor_branches.region`**; nama diambil dari `regions.region` lewat `region_code`. Pemakai yang ikut diubah: filter/kolom FLM Vendor Region di rekomendasi Vendor Request (`vendor_request.sql`), ekspor CSV cabang vendor (kolom `region` tetap ada, isi dari `regions`; diabaikan saat impor), daftar vault admin (`vendor_vaults_admin.sql`). Drop kolom = Sec 4 #7 (delete), rollback dirinci di design.
6. **`regions.code` immutable**: kalau format kode diganti, boleh migrasi sekali jalan mengubah `regions.code` (aturan immutable tetap berlaku untuk layar)?
   ✅ DIPUTUSKAN PO 2026-11-17: **rename sekali di migrasi** — `code` + `region` diubah pada baris yang sama (`id` tetap, `locations`/ATM tidak disentuh). Kode tetap tidak bisa diubah lewat layar admin.
7. **Tier vault + Area ACM**: perubahan tier dan pemilik Area ACM untuk request baru akibat penggabungan region boleh berlaku langsung? Request/plan yang sedang berjalan pakai region lama atau baru?
   ✅ DIPUTUSKAN PO 2026-11-17: request/plan/tiket yang sedang berjalan **di-clear dulu di DB dev** sebelum migrasi. Jadi tidak ada penanganan request berjalan di migrasi; semua request baru langsung memakai region baku (tier + Area ACM baru). Clear data dev = langkah terpisah dengan OK user saat dijalankan. Catatan: belum ada lingkungan prod (CLAUDE.md Sec 10); kalau prod sudah hidup sebelum fitur ini rilis, pertanyaan ini dibuka lagi.
8. **51 cabang tanpa kode**: diisi di migrasi ini (pemetaan dari PO) atau tetap NULL dan diisi lewat maker-checker?
   ✅ DIPUTUSKAN PO 2026-11-17: **masuk draf pemetaan**. Ke-51 cabang (semua aktif, vendor FLM: BRNK 16, ADV 14, PSG 9, KJR 5, SSI 3, TAG 3, ABA 1; nama ESQ juga kosong) diusulkan wilayahnya dari kota di `branch_name`, disetujui PO bersama pemetaan 34 kode, diisi oleh migrasi.
9. **Jumlah ATM / Cabang Vendor**: hitung yang aktif saja (`atms.is_active`, `vendor_branches.is_active`) atau semua? Jumlah Lokasi tetap dipakai backend untuk menolak disable region?
   ✅ DIPUTUSKAN PO 2026-11-17: hitung **yang aktif saja** (`atms.is_active = true`, `vendor_branches.is_active = true`). Kolom "Jumlah Lokasi" tidak tampil lagi di layar; pengecekan lokasi untuk menolak disable region tetap ada di backend.
10. **Disable region**: apakah region yang masih dirujuk cabang vendor aktif juga ditolak saat disable (sekarang hanya cek lokasi)?
   ✅ DIPUTUSKAN PO 2026-11-17: **ya, tolak**. Disable region ditolak bila masih dirujuk lokasi **atau** cabang vendor aktif (`vendor_branches.is_active = true`). Dicek di service (FK tidak menahan soft-disable).