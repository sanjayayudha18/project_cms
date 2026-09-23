# Plan: Kelolaan Cabang â†’ Vault/PIC/Paket per-Cabang (Vendor Detail)

**Route**: `http://localhost:5174/settings/admin/vendors/2`
**Status**: Draft revisi, belum diimplementasikan; verifikasi kontrak backend diperlukan sebelum implementasi frontend.
**Terakhir disesuaikan**: 2026-09-22, mengikuti migrasi 007 (`branch_coverage_areas`) dan 008 (`vendor_vaults.category` jadi tiga nilai).
**Tanggal**: 2026-09-22

## Latar Belakang / Temuan Awal

Menurut plan awal, backend sudah memiliki:
- `vendor_vaults.vendor_branch_id`: NOT NULL, wajib per-cabang.
- `vendor_packages.vendor_branch_id`: NOT NULL, wajib per-cabang.
- `vendor_pics.vendor_branch_id`: nullable by design, mendukung PIC vendor-wide maupun per-cabang.
- CRUD maker-checker untuk branch/vault/pic/package melalui `MasterDataChangeService.Submit` dan registry `Applier`, termasuk create/update/disable/enable dengan respons 202 staged.

Referensi implementasi yang harus diperiksa:
- [admin_vendor_branch_handler.go](../../../backend/internal/handler/admin_vendor_branch_handler.go)
- [admin_vendor_vault_handler.go](../../../backend/internal/handler/admin_vendor_vault_handler.go)
- [admin_vendor_pic_handler.go](../../../backend/internal/handler/admin_vendor_pic_handler.go)
- [admin_vendor_package_handler.go](../../../backend/internal/handler/admin_vendor_package_handler.go)
- [VendorDetailPage.tsx](../../../frontend/CompanyPortal-Vite/src/features/admin-vendors/components/VendorDetailPage.tsx)

Menurut plan awal, frontend saat ini menampilkan tab flat read-only Info/Cabang/Vault/PIC/Paket melalui `ChildTab` dan `DataTable`, tanpa drill-down cabang atau dialog CRUD child entities.

**Batas verifikasi**: klaim tentang schema, endpoint, UI, dan maker-checker di atas berasal dari plan awal, bukan hasil pemeriksaan source code dalam review ini. Jangan menganggap backend lengkap sebelum checklist verifikasi berikut selesai.

### Perubahan schema setelah plan ini ditulis (migrasi 007 & 008, sudah diterapkan ke DB)

- `vendor_vaults.category` **tidak lagi biner**. Constraint `vendor_vaults_category_chk` sekarang `IN ('ATM','CASH','ATM_CASH')`; nilai sebelumnya seluruhnya placeholder `'ATM'` dan kini terisi dari daftar master vendor. Validasi backend di [vendor_vault_admin.go](../../../backend/internal/service/vendor_vault_admin.go) sudah menerima ketiganya.
- `vendor_vaults.type` adalah kolom legacy yang **di-mirror dari `category`** pada create maupun update ([vendor_vaults_admin.sql](../../../backend/queries/vendor_vaults_admin.sql)). Dialog vault tidak boleh mengekspos `type` sebagai field terpisah.
- Tabel baru `branch_coverage_areas` (cakupan kota per cabang, many-to-many) kini menjadi child keempat dari cabang. Belum punya repo/handler/endpoint — lihat "Yang Tidak Dikerjakan".
- Data cabang bertambah: 3 vendor baru (Brinks, Kejar, Prosegur), 51 cabang baru, total 407 cabang (308 di antaranya milik ROH). Vendor 2 (Abacus) pada route contoh sekarang punya 9 cabang.

## Keputusan Desain

1. Gunakan filter server-side dan pagination untuk data per-cabang; batas `page_size=100` bukan jaminan seluruh data sudah diambil.
2. HTTP 202 berarti pengajuan diterima untuk approval, bukan perubahan master data sudah berlaku.
3. Pisahkan akses PIC vendor-wide dari PIC per-cabang; `NULL` tetap valid untuk scope vendor-wide.
4. Sediakan create/edit/disable/enable untuk keempat entity, termasuk cabang.
5. Validasi relasi vendor, cabang, dan child entity di backend, bukan hanya melalui field hidden/read-only di frontend.

## 1. Verifikasi Kontrak Sebelum Implementasi

Hasil verifikasi 2026-09-22 (diperiksa langsung di source, bukan asumsi):

- [x] Schema/nullability `vendor_branch_id`: **sesuai klaim plan awal**. NOT NULL di `vendor_vaults` dan `vendor_packages`, nullable di `vendor_pics` (`sqlc.narg` pada create/update).
- [x] Endpoint empat entity **ada dan ter-mount**, semuanya di bawah route group `masterDataAdmin` (RBAC `ADMIN`/`ADMIN_PARAM`) di [main.go](../../../backend/cmd/api/main.go) L303/L310/L317/L324: `/api/v1/admin/vendors/{vendorID}/{branches|vaults|pics|packages}`. Keempat handler punya List/Get/Create/Update/toggle.
- [x] Pagination/sorting/count: `List...Admin` + `Count...Admin` sudah berpasangan dengan filter identik (`vendor_id`, `q`, `status`), ordering deterministik (`ORDER BY <natural key> ASC, id ASC`), `LIMIT/OFFSET` via `page_limit`/`page_offset`.
- [x] Maker-checker: keempat `Applier` terdaftar di `ApplierRegistry` ([main.go](../../../backend/cmd/api/main.go) L217-224) — `vendor_branch`, `vendor_vault`, `vendor_pic`, `vendor_package`. Sumber status pengajuan = `master_data_change_requests` + `approval_requests`, sesuai Sec 12 CLAUDE.md.
- [x] Ownership divalidasi ulang saat apply: setiap Applier mengambil "before" snapshot dan menandai `stale` bila entity berubah sejak submit.
- [x] Pending create tanpa ID master: didukung — `master_data_change_requests` menyimpan payload dengan `entity_id` NULL untuk op `create`.

**Blocker yang tersisa — dua item, belum bisa dicentang:**

- [ ] **Filter `branch_id` belum ada sama sekali.** `ListVendorVaultsAdmin`/`ListVendorPicsAdmin`/`ListVendorPackagesAdmin` beserta `Count`-nya hanya memfilter `vendor_id`, `q`, `status`. Tidak ada parameter cabang, dan tidak ada filter eksplisit untuk PIC vendor-wide (`vendor_branch_id IS NULL`). Kontraknya harus **didefinisikan**, bukan sekadar diverifikasi. Ini pekerjaan §2 dan prasyarat seluruh drill-down cabang.
- [ ] **Aturan disable cabang dengan child aktif belum ada.** [masterdata_applier_vendor_branch.go](../../../backend/internal/service/masterdata_applier_vendor_branch.go) L57-62 memanggil `DisableVendorBranch` tanpa pemeriksaan child apa pun, sehingga vault/PIC/paket aktif menggantung di cabang nonaktif tanpa peringatan. Plan ini memilih default "tolak sampai child ditangani", tapi **butuh konfirmasi bisnis** sebelum diimplementasikan.

Status frontend: `types.ts` baru punya `AdminVendor`, `AdminVendorsListParams`, `AdminVendorsListResponse` — belum ada tipe untuk keempat child entity. Di `components/` baru ada `VendorFormDialog.tsx`; keempat dialog child belum dibuat. Jadi §3 dan §4 sepenuhnya pekerjaan baru.

Jika kontrak atau kemampuan yang dibutuhkan belum tersedia, selesaikan gap backend tersebut sebelum UI terkait dinyatakan selesai. Pertahankan pola existing; jangan membuat endpoint atau kolom baru tanpa kebutuhan yang terverifikasi.

## 2. Backend: Filtering, Pagination, dan Integritas Scope

### Filter list/count

Tambahkan atau gunakan filter opsional `branch_id` pada `List...Admin` dan `Count...Admin` untuk vaults/pics/packages:
- [vendor_vaults_admin.sql](../../../backend/queries/vendor_vaults_admin.sql)
- `vendor_pics_admin.sql`
- `vendor_packages_admin.sql`
- Pass-through handler, service, repository, dan hasil generate query sesuai pola proyek.

Ketentuan:
- Filter diterapkan sebelum pagination, dengan ordering deterministik.
- List dan count menggunakan kondisi vendor, cabang, scope PIC, serta status/search yang sama bila tersedia.
- Untuk PIC vendor-wide, sediakan filter eksplisit yang berarti `vendor_branch_id IS NULL`. Gunakan kontrak existing atau definisikan kontraknya setelah verifikasi; tidak mengirim `branch_id` tidak otomatis berarti vendor-wide.
- Request tanpa filter cabang tetap mengikuti perilaku existing agar kompatibel.
- Validasi `branch_id` berada di vendor pada URL; jangan mengembalikan data lintas vendor.

**Fallback sementara**: filtering client-side hanya boleh dipakai bila seluruh halaman untuk scope terkait telah diambil dan kelengkapannya dapat dibuktikan dari kontrak pagination. Jika satu halaman gagal atau pengambilan belum selesai, jangan tampilkan hasil sebagai dataset lengkap atau empty state final. Default implementasi tetap server-side; jangan menunggu jumlah data melebihi 100 baru memperbaikinya.

### Validasi mutation dan approval

- Branch yang dipilih harus milik vendor pada route.
- Target update/disable/enable harus milik vendor dan scope yang berwenang diakses.
- Vault/package wajib memiliki cabang valid; PIC boleh tanpa cabang hanya pada flow vendor-wide.
- Validasi kembali relasi dan eligibility saat apply karena kondisi dapat berubah setelah pengajuan.
- Hidden/read-only field bukan kontrol keamanan; backend tetap menolak manipulasi payload.
- Perpindahan antar-cabang atau vendor bukan bagian scope ini; jangan mengizinkannya tanpa flow dan aturan eksplisit.
- Gunakan aturan existing untuk child aktif saat cabang dinonaktifkan. Jika belum ada, default rancangan adalah menolak disable sampai child aktif ditangani, bukan cascade diam-diam; konfirmasi aturan bisnis sebelum implementasi.
- Pengajuan disable cabang yang masih pending tidak langsung mengubah status efektif cabang.

## 3. Frontend: Data Layer

### `types.ts`

Tambahkan tipe eksplisit `AdminVendorBranch`, `AdminVendorVault`, `AdminVendorPic`, dan `AdminVendorPackage` untuk menggantikan `Record<string, unknown>`.
- Samakan tipe ID dengan kontrak API.
- `vendor_branch_id` wajib untuk vault/package, nullable untuk PIC.
- `AdminVendorVault.category` adalah enum tiga nilai `'ATM' | 'CASH' | 'ATM_CASH'`, bukan boolean/biner. Jangan memodelkan sebagai dua pilihan.
- Pisahkan tipe create/update payload, pagination response, dan staged submission response.
- Pisahkan status master data dari status request approval. Nama enum/field mengikuti backend yang terverifikasi.

### `api.ts`

Tambahkan create/update/disable/enable untuk branches/vaults/pics/packages, mengikuti pola vendor-level existing.

Endpoint yang disebutkan plan awal dan harus diverifikasi:
```text
/admin/vendors/{vendorId}/{branches|vaults|pics|packages}
/admin/vendors/{vendorId}/{kind}/{id}
/admin/vendors/{vendorId}/{kind}/{id}/disable
/admin/vendors/{vendorId}/{kind}/{id}/enable
```

Teruskan parameter pagination, `branch_id`, dan scope PIC yang relevan pada operasi list. Parse respons 202 sebagai submission acknowledgement, bukan sebagai entity master yang sudah tersimpan.

### `hooks.ts`

- Tambahkan mutation hook untuk empat aksi pada empat entity.
- Query key list mencakup vendorId, kind, branch/scope, pagination, serta filter/sort yang memengaruhi response.
- Gunakan prefix `['admin-vendors', 'children', vendorId, kind]` untuk invalidasi seluruh variasi list entity terkait.
- Simpan submission acknowledgement untuk feedback segera dan refresh sumber status request yang authoritative.
- Setelah approval/rejection diketahui, refresh request status dan query master terkait; gunakan mekanisme existing, atau refetch saat focus/manual refresh bila belum ada push update.
- Jangan melakukan optimistic update yang menampilkan perubahan staged sebagai data aktif.
- Saat pindah cabang, reset pagination dan selection; jangan tampilkan hasil cabang sebelumnya sebagai data cabang baru.

## 4. Frontend: Navigasi, Dialog, dan Aksi

### `VendorDetailPage.tsx`

- Pertahankan tab Info.
- Tab Cabang menjadi list yang dapat dipilih melalui mouse dan keyboard, dengan state `selectedBranchId`.
- Sediakan create/edit/disable/enable cabang, sesuai permission dan status efektifnya.
- Pilihan cabang membuka panel Vault/PIC/Paket dengan query server-side untuk cabang tersebut.
- Tampilkan nama cabang aktif agar scope aksi selalu jelas.
- Sediakan entry/tab **PIC Vendor-wide** yang tetap dapat diakses tanpa memilih cabang, termasuk bila vendor belum memiliki cabang.
- PIC vendor-wide tidak dimasukkan ke panel PIC cabang secara implisit.
- Hindari hilangnya akses ke data lama saat restrukturisasi tab flat.
- Tampilkan loading, error/retry, empty state setelah query lengkap, dan pagination secara terpisah.

### Dialog form per entity

Mirror pola [VendorFormDialog.tsx](../../../frontend/CompanyPortal-Vite/src/features/admin-vendors/components/VendorFormDialog.tsx) dan `lib/vendorFormSchema.ts`:
- `VendorBranchFormDialog`
- `VendorVaultFormDialog`
- `VendorPicFormDialog`
- `VendorPackageFormDialog`

Ketentuan:
- Pada flow per-cabang, isi `vendor_branch_id` dari cabang aktif; field read-only/hidden, bukan dropdown bebas.
- Pada flow PIC vendor-wide, kirim representasi tanpa cabang sesuai kontrak API dan tampilkan scope secara jelas.
- Kunci context vendor/cabang selama dialog terbuka, atau tutup/reset dialog saat scope berubah, agar submit tidak memakai context yang berbeda.
- Validasi field mengikuti backend dan tampilkan error pada field/form yang sesuai.
- Disable/enable memerlukan konfirmasi UI dan mengikuti maker-checker yang sama dengan create/edit.

## 5. Maker-Checker: Status, Konflik, dan Feedback

- Respons 202: tampilkan **Pengajuan dikirim, menunggu persetujuan**. Jangan menyatakan perubahan sudah aktif.
- Sumber badge pending adalah data request approval existing yang diverifikasi, bukan sekadar hasil invalidasi list master atau flag lokal permanen.
- Pending create muncul sebagai pengajuan terpisah di scope terkait karena row master mungkin belum ada.
- Pending update/disable/enable ditautkan ke entity terkait; data/status efektif lama tetap ditampilkan sampai approval berhasil diterapkan.
- Setelah approval dan apply berhasil, refresh master data dan hilangkan badge pending yang sudah selesai.
- Jika rejected, data efektif tetap tidak berubah; tampilkan status dan alasan penolakan bila tersedia serta alur perbaikan/pengajuan ulang sesuai aturan existing.
- Kegagalan apply tidak boleh ditampilkan sebagai perubahan efektif yang sukses; ikuti state lifecycle backend.
- Cegah double-click selama submit dan blokir aksi yang berkonflik dengan pending request sesuai aturan existing.
- Backend tetap menjadi otoritas pencegahan duplikasi/konflik, termasuk lintas tab atau user; bila belum ada proteksinya, masukkan perbaikannya sebagai prerequisite.
- Permission frontend hanya mengatur tampilan; authorization tetap ditegakkan server-side.

## Yang Tidak Dikerjakan

- Tidak menambah kolom DB baru hanya untuk relasi cabang yang menurut plan awal sudah tersedia.
- Tidak membangun UI/endpoint untuk `branch_coverage_areas`. Tabel sudah ada dari migrasi 007 tetapi belum punya repo/handler; bila kelak dikelola dari layar, harus mengikuti pola maker-checker (queries -> repo read-only -> service `Submit` -> `Applier` di `main.go` -> handler 202), bukan menulis tabel langsung.
- Tidak menduplikasi endpoint CRUD existing; gap API hanya diperbaiki bila hasil verifikasi membuktikan kebutuhan.
- Tidak mengubah PIC menjadi branch-only.
- Tidak melakukan cascade disable child secara implisit.
- Tidak menambahkan perpindahan child antar-cabang/vendor.
- Tidak membangun ulang workflow approval; integrasikan pola existing dan tutup gap yang relevan.

## Urutan Eksekusi

1. Verifikasi kontrak backend, sumber status approval, ownership, dan aturan cabang nonaktif.
2. Implementasikan filter list/count dan pagination serta tutup gap validasi/konflik yang ditemukan.
3. Perbarui `types.ts`, `api.ts`, dan `hooks.ts`, termasuk pemisahan master data dan submission state.
4. Buat empat dialog beserta flow PIC vendor-wide dan aksi cabang.
5. Restrukturisasi `VendorDetailPage.tsx` untuk drill-down cabang, pagination, permission, dan feedback pending.
6. Tambahkan pengujian otomatis frontend/backend, lalu manual browser check dan walkthrough approval.

## Pengujian dan Acceptance Criteria

Extend `__tests__/VendorDetailPage.test.tsx` dan pengujian backend terkait:
- [ ] Klik cabang A/B menampilkan data scope yang benar; perpindahan cepat tidak membocorkan hasil cabang sebelumnya.
- [ ] Dataset vendor lebih dari 100 baris, termasuk child cabang yang hanya muncul setelah halaman pertama, tetap dapat ditemukan melalui pagination/filter. Vendor ROH (308 cabang) adalah kasus uji nyata untuk ini.
- [ ] Total/count konsisten dengan filter vendor/cabang/scope PIC; urutan halaman deterministik.
- [ ] PIC vendor-wide dapat dibaca dan dikelola tanpa memilih cabang serta tidak tercampur dengan PIC cabang.
- [ ] Create/edit/disable/enable diuji untuk branch, vault, PIC, dan package, termasuk payload dan permission.
- [ ] Respons 202 menghasilkan feedback pending, bukan perubahan efektif pada master data.
- [ ] Pending create terlihat tanpa ID master; pending request tetap terlihat setelah reload melalui sumber authoritative.
- [ ] Approval/apply memperbarui data efektif; rejection mempertahankan data lama dan menampilkan status yang benar; kegagalan apply tidak dilaporkan sukses.
- [ ] Double-click, pengajuan berulang, dan konflik lintas tab/user ditangani; server menolak konflik sesuai kebijakan.
- [ ] Manipulasi vendorId/branchId/entityId lintas vendor ditolak pada list/mutation dan divalidasi ulang saat apply.
- [ ] Vault/package menolak cabang kosong; PIC vendor-wide menerima scope NULL sesuai kontrak.
- [ ] Form vault menerima ketiga nilai `category` (`ATM`, `CASH`, `ATM_CASH`) dan menolak nilai lain; `type` tidak diekspos sebagai field terpisah.
- [ ] Disable cabang dengan child aktif mengikuti aturan bisnis terverifikasi, tanpa cascade diam-diam.
- [ ] Loading, empty, error/retry, pagination, dan pergantian context dialog berfungsi tanpa submit salah cabang.
- [ ] Alur maker/checker mengikuti aturan otorisasi dan pemisahan kewenangan existing.

Implementasi selesai bila seluruh acceptance criteria relevan lulus dan kontrak backend yang sebelumnya diasumsikan sudah diverifikasi.

## Catatan

- Manual browser check pada `http://localhost:5174/settings/admin/vendors/2` dilakukan oleh user sesuai prosedur proyek yang disebutkan dalam plan awal.
- Rujukan plan awal: `CLAUDE.md` Sec 0 poin 10 dan Sec 12, keputusan "Master-data maker-checker & scope". Isi rujukan tersebut belum diverifikasi dalam revisi dokumen ini.
- Revisi ini memperbarui rencana, bukan mengimplementasikan atau menguji perubahan kode.
