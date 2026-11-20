# Requirements Document

> Status: accepted 2026-11-17 (PO) — Introduction diterima ("Terima, lanjut EARS"); Q1–Q9 ✅ DIPUTUSKAN PO 2026-11-17 (ikut rekomendasi); Requirements diterima ("Ya, lanjut design").

## Introduction

### Masalah

User mereview menu Pengaturan di CompanyPortal (dev `http://localhost:3001`) pada 2026-11-17 dan mencatat (kata-kata user, `.claude/review-pengaturan-2026-11-17.md`):

1. "pada menu banyak menu tidak terdapat tombol back, contoh di `/settings/admin/users`"
2. "pada `/settings/admin/vendors` buat data Dummy, dan filter khusus untuk ROH, lalu hapus tulisan 'Lihat Ekspor Saya'"
3. "tolong buat seed PIC vendor wide dan PIC cabang masing-masing vendor cukup buat 2 seed saja (vendor wide 2, vendor cabang 2)"

### Hasil yang diharapkan

- Setiap halaman Pengaturan punya jalan kembali yang jelas.
- Halaman Vendor berisi data contoh di dev, bisa difilter ke ROH, dan tidak menampilkan tautan "Lihat Ekspor Saya".
- Tiap vendor di dev punya PIC contoh: 2 vendor-wide + 2 per cabang.

### Kondisi kode saat ini (dicek 2026-11-17)

**Halaman Pengaturan** (`frontend/CompanyPortal-Vite/src/routes/settings/`)
- Hub `/settings` (`routes/settings.tsx`, `SettingsHubPage` dari `features/rbac-settings`).
- Route admin: `acm-areas`, `atms`, `exports` (redirect), `holidays`, `master-data-io`, `regions`, `users`, `vendors`, `vendor-detail` (`/settings/admin/vendors/$vendorId`), `vendor-branch-detail`, `vendor-branch-edit`; plus `settings/roles.tsx` dan `settings/rbac/`.
- Belum ada komponen tombol Back bersama. Tautan kembali yang sudah ada: `VendorHeader.tsx` (`← Kembali ke daftar` → `/settings/admin/vendors`), `VendorBranchDetailPage.tsx`, `VendorBranchEditPage.tsx`.

**Halaman Vendor** (`features/admin-vendors/components/AdminVendorsPage.tsx`)
- Filter list backend (`GET /api/v1/admin/vendors`, `admin_vendor_handler.go` `List`): `page`, `page_size`, `status`, `q`, `pks_status`; nilai tidak valid → 400 `bad_request`. **Belum ada filter `kind`** (ROH = `vendors.kind = 'INTERNAL'`, CLAUDE.md §3). Detail vendor sudah mengembalikan `kind`.
- "Lihat Ekspor Saya" berasal dari komponen bersama `features/exports/components/ReportExportNotice.tsx` (`<Link to="/exports">`, juga menampilkan pesan error permintaan ekspor), dipakai juga oleh `DsrLateReportScreen` dan `ReplenishmentScreen`. Teks yang sama ada di `ExportProgressDialog.tsx`.
- Tombol ekspor PKS di halaman Vendor mengirim `status` + `pks_status` ke report `vendor-pks` (`backend-exq`).
- `/admin/vendors` belum ada di `backend/api/openapi.yaml`.

**Seed dev**
- Pola yang sudah ada: `backend/scripts/dev/*.sql` (mis. `seed_vendor_users.sql`) = "DEV ONLY — not a migration", idempoten, ditulis langsung tanpa `audit_logs`, dijalankan manual dengan `psql -f`. Lewati vendor integration-test (`code LIKE 'IT%'`).
- `002_baseline_seed.sql` tidak berisi `vendors` maupun `vendor_pics`.
- `vendor_pics`: `vendor_branch_id` NULL = PIC tingkat vendor; `roles` ⊆ {replenish, vault, cashcount}, default ketiganya; `is_director` = Direktur Utama (constraint mig `037`). Create PIC lewat layar = maker-checker.

### Ruang lingkup

- Tombol/tautan Back di halaman Pengaturan yang belum punya.
- Filter Jenis vendor (Semua / Vendor FLM / ROH) di daftar vendor (UI + parameter list backend).
- Hapus "Lihat Ekspor Saya" di halaman Vendor saja.
- Script seed dev: vendor dummy + PIC vendor-wide dan PIC cabang.

### Tidak termasuk

- Perubahan skema; migrasi apa pun.
- Mengubah alur maker-checker vendor/PIC.
- "Lihat Ekspor Saya" di Laporan DSR telat, Replenishment, dan dialog progres ekspor (Q6).
- Vault, ATM, paket, dan PIC Direktur Utama di data dummy (Q3, Q7).

### Area sensitif (CLAUDE.md §4 #7)

- Tidak ada: seed berupa script dev, bukan migrasi (Q5); tidak menyentuh auth, uang, rekonsiliasi, atau delete. Filter `kind` hanya baca (replica, sama seperti filter lain).

### Keputusan PO (✅ DIPUTUSKAN PO 2026-11-17, ikut rekomendasi AI)

- **Q1** Back menuju **halaman induk** dengan tautan tetap (bukan `history.back`): daftar → hub `/settings`; detail → daftarnya.
- **Q2** Berlaku untuk **semua halaman `/settings/*`** (termasuk `roles`, `rbac`) kecuali hub `/settings`. Tautan kembali yang sudah ada tidak diubah.
- **Q3** **6 vendor dummy: 5 FLM + 1 ROH**, tiap vendor **2 cabang**; tanpa vault/ATM/paket; kode vendor berawalan khusus dummy.
- **Q4** Dropdown **"Jenis vendor": Semua / Vendor FLM / ROH**, server-side (parameter `kind`).
- **Q5** **Script dev** di `backend/scripts/dev/`; tidak ada migrasi.
- **Q6** "Lihat Ekspor Saya" dihapus **hanya di halaman Vendor**.
- **Q7** **2 PIC vendor-wide per vendor + 2 PIC per cabang aktif**; semua vendor aktif selain `IT%`, termasuk ROH; `roles` ketiganya; `is_notification_recipient = false`; `is_director = false`.
- **Q8** Untuk **vendor yang sudah ada dan vendor dummy**; PIC seed ditandai email `@pic.dev.local`; vendor/cabang yang sudah punya PIC seed tidak ditambah lagi; PIC lain tidak disentuh.

## Glossary

- **SettingsPage**: halaman CompanyPortal di bawah `/settings/*`, kecuali hub `/settings`.
- **ParentPage**: halaman induk SettingsPage — hub `/settings` untuk halaman daftar/menu, halaman daftar untuk halaman detail/edit.
- **BackLink**: tautan "← Kembali" di bagian atas SettingsPage yang menuju ParentPage.
- **VendorListAPI**: `GET /api/v1/admin/vendors` di `backend/` (`AdminVendorHandler.List`).
- **VendorListPage**: halaman `/settings/admin/vendors` (`AdminVendorsPage`).
- **KindFilter**: dropdown "Jenis vendor" di VendorListPage, nilai Semua / Vendor FLM (`FLM_VENDOR`) / ROH (`INTERNAL`).
- **DevSeedScript**: file SQL di `backend/scripts/dev/`, DEV ONLY, idempoten, dijalankan manual dengan `psql -f`.
- **DummyVendor**: vendor yang dibuat DevSeedScript, kode berawalan khusus dummy.
- **SeedPic**: baris `vendor_pics` yang dibuat DevSeedScript, ditandai email berakhiran `@pic.dev.local`.
- **EligibleVendor**: vendor aktif (`is_active`, `deleted_at IS NULL`) yang kodenya tidak berawalan `IT`.

## Requirements

### Requirement 1: Tombol kembali di halaman Pengaturan

**User Story:** Sebagai admin, saya ingin setiap halaman Pengaturan punya tombol kembali, agar saya bisa kembali ke menu induk tanpa memakai tombol browser atau sidebar.

#### Acceptance Criteria

1. THE SettingsPage SHALL menampilkan satu BackLink di bagian atas halaman.
2. WHEN admin mengaktifkan BackLink, THE SettingsPage SHALL berpindah ke ParentPage-nya.
3. THE BackLink SHALL menuju ParentPage yang sama baik halaman dibuka lewat navigasi maupun langsung dari URL.
4. WHERE SettingsPage sudah punya tautan kembali ke ParentPage, THE SettingsPage SHALL mempertahankan tautan itu tanpa menambah BackLink kedua.
5. THE BackLink SHALL bisa dipakai dengan keyboard dan punya nama aksesibel yang menyebut tujuannya.
6. THE hub `/settings` SHALL tidak menampilkan BackLink.

### Requirement 2: Filter Jenis vendor (ROH) di daftar vendor

**User Story:** Sebagai admin, saya ingin memfilter daftar vendor ke ROH atau vendor FLM, agar saya bisa melihat unit internal CIMB terpisah dari vendor FLM.

#### Acceptance Criteria

1. WHEN VendorListAPI menerima parameter `kind` bernilai `FLM_VENDOR` atau `INTERNAL`, THE VendorListAPI SHALL hanya mengembalikan vendor dengan `kind` itu, dan `total` SHALL dihitung dengan filter yang sama.
2. WHEN parameter `kind` kosong atau tidak dikirim, THE VendorListAPI SHALL mengembalikan vendor semua jenis seperti sekarang.
3. IF parameter `kind` berisi nilai lain, THEN THE VendorListAPI SHALL menolak dengan 400 `bad_request`.
4. THE VendorListAPI SHALL menggabungkan `kind` dengan `status`, `q`, `pks_status`, dan paging yang sudah ada.
5. THE VendorListAPI SHALL membaca dari replica dan tetap dibatasi role yang sama dengan list sekarang (`ADMIN`, `ADMIN_PARAM`).
6. THE VendorListPage SHALL menampilkan KindFilter dengan pilihan Semua, Vendor FLM, dan ROH; default Semua.
7. WHEN admin memilih nilai KindFilter, THE VendorListPage SHALL memuat ulang daftar dari halaman 1 dengan filter itu dan menyimpannya di URL search, sama seperti filter lain di halaman itu.

### Requirement 3: Hapus "Lihat Ekspor Saya" di halaman Vendor

**User Story:** Sebagai admin, saya tidak ingin melihat tautan "Lihat Ekspor Saya" di halaman Vendor.

#### Acceptance Criteria

1. THE VendorListPage SHALL tidak menampilkan tautan "Lihat Ekspor Saya".
2. WHEN permintaan ekspor dari VendorListPage gagal, THE VendorListPage SHALL tetap menampilkan pesan error permintaan ekspor.
3. THE Laporan DSR telat, Replenishment, dan dialog progres ekspor SHALL tetap menampilkan "Lihat Ekspor Saya" seperti sekarang.

### Requirement 4: Seed vendor dummy

**User Story:** Sebagai developer/tester, saya ingin data vendor contoh di DB dev, agar halaman Vendor dan filter ROH bisa dicoba.

#### Acceptance Criteria

1. THE DevSeedScript SHALL membuat 6 DummyVendor: 5 dengan `kind = FLM_VENDOR` dan 1 dengan `kind = INTERNAL`, semua aktif.
2. THE DevSeedScript SHALL membuat 2 `vendor_branches` aktif untuk tiap DummyVendor.
3. THE DevSeedScript SHALL mengisi setiap kolom wajib dan constraint tabel yang dipakai, termasuk `region_code` cabang dari region yang ada di dev.
4. WHEN DevSeedScript dijalankan lagi, THE DevSeedScript SHALL tidak membuat DummyVendor atau cabang duplikat dan tidak mengubah data yang sudah ada.
5. THE DevSeedScript SHALL tidak membuat vault, ATM, paket, atau PKS.
6. THE DevSeedScript SHALL bukan file migrasi dan diberi header "DEV ONLY — not a migration" beserta cara menjalankannya.

### Requirement 5: Seed PIC vendor-wide dan PIC cabang

**User Story:** Sebagai developer/tester, saya ingin setiap vendor di dev punya PIC contoh, agar fitur yang memakai PIC bisa dicoba.

#### Acceptance Criteria

1. FOR each EligibleVendor (termasuk DummyVendor dan ROH), THE DevSeedScript SHALL membuat 2 SeedPic tingkat vendor (`vendor_branch_id` NULL).
2. FOR each `vendor_branches` aktif milik EligibleVendor, THE DevSeedScript SHALL membuat 2 SeedPic untuk cabang itu.
3. THE SeedPic SHALL punya `roles` = {replenish, vault, cashcount}, `is_notification_recipient = false`, `is_director = false`, dan `is_active = true`.
4. WHERE vendor atau cabang sudah punya SeedPic, THE DevSeedScript SHALL tidak menambah SeedPic untuk vendor/cabang itu.
5. THE DevSeedScript SHALL tidak mengubah atau menonaktifkan PIC yang bukan SeedPic.
6. THE DevSeedScript SHALL menjalankan seed PIC setelah seed vendor dummy, sehingga DummyVendor ikut mendapat SeedPic dalam satu kali jalan.

### Requirement 6: Kontrak API dan regresi

**User Story:** Sebagai engineer, saya ingin perubahan ini teruji dan tidak merusak perilaku yang ada.

#### Acceptance Criteria

1. THE VendorListAPI SHALL punya test untuk tiap nilai `kind`, nilai tidak valid (400), dan kombinasi dengan filter lain.
2. THE VendorListPage SHALL punya component test untuk KindFilter dan untuk tidak adanya "Lihat Ekspor Saya".
3. THE BackLink SHALL punya component test yang memastikan tujuan ParentPage per halaman.
4. THE gate `cd backend && go test ./...`, `pnpm --dir frontend/CompanyPortal-Vite run test`, `lint`, dan `build` SHALL lulus.
5. Pemeriksaan browser manual SHALL dicatat "outstanding (user)" (Golden Rule #10).

## Open Questions

- **Q9** ✅ DIPUTUSKAN PO 2026-11-17: **tidak ikut**. Tombol ekspor PKS di halaman Vendor saat ini mengirim filter `status` + `pks_status`. Apakah filter Jenis vendor juga ikut ke ekspor? Rekomendasi: **tidak** (fitur ini tanpa perubahan `backend-exq`; isi CSV mengikuti filter lain seperti sekarang). Bila ya, ekspor `vendor-pks` di `backend-exq` + `pkg/reports` ikut diubah.
- *Untuk design (teknis)*: bentuk BackLink (satu komponen bersama vs tautan per halaman), daftar ParentPage per route, dan prefix kode DummyVendor.
