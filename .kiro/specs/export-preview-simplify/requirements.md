# Requirements Document

> **Status:** requirements disetujui 2026-10-30. PO menghapus permintaan "ATM dan Kelolaan ATM dijadikan satu" (D1 dibatalkan) dan mengikuti saran Kiro untuk D2-D5.

## Introduction

### Masalah

Halaman Export Data (`/exports`) dan halaman pratinjau per data (`/exports/<data>`) baru dibangun lewat spec `export-menu-revamp` (X.9, commit `3f56450`). Setelah dicoba, PO meminta empat perbaikan di halaman pratinjau:

1. Kotak pencarian vendor dihilangkan; filter cukup berupa dropdown (contoh `/exports/vendor-pics`).
2. Tombol "Terapkan" diganti menjadi "Search".
3. Tulisan "File diambil saat ekspor diproses; perubahan data setelah tabel dimuat bisa ikut masuk." dihilangkan.
4. Tombol ekspor cukup tombol saja, tanpa jumlah baris.

Permintaan awal kelima (menggabungkan menu ATM dan Kelolaan ATM) dihapus PO pada 2026-10-30 (D1). Katalog tetap menampilkan "ATM" dan "Kelolaan ATM" sebagai dua data terpisah.

### Kondisi kode saat ini (dicek 2026-10-30, `dev1` @ `3f56450`, working tree bersih)

Semua file di `frontend/CompanyPortal-Vite/src/features/exports/`.

- `components/exportFormParts.tsx` → `VendorSelect` merender dua kontrol: input `type="search"` berlabel "Cari vendor" (mengirim `q` ke `useVendorsList`, maks. 100 vendor aktif) dan `FilterSelect` "Vendor". `BranchSelect` sudah berupa satu dropdown. Seed dev punya 6 vendor.
- `components/PreviewFilterBar.tsx` → tombol submit form berlabel "Terapkan". Isian filter adalah draft; draft baru masuk URL (Applied_Filter) setelah tombol ditekan.
- `exportButtonState.ts` → saat draft berbeda dari Applied_Filter, tombol ekspor nonaktif dengan pesan "Terapkan filter dulu". Nonaktif juga saat memuat, total 0, atau total > 1.000.000.
- `components/PreviewExportBar.tsx` → tombol "Ekspor {total} baris" dan paragraf catatan "File diambil saat ekspor diproses; …" (`export-menu-revamp` Req 5.1 dan 5.9).
- `components/ExportPreviewPage.tsx` → kepala halaman menampilkan "Total N baris" dari `total` respons pratinjau (terpisah dari tombol).
- Backend `backend-exq` (API pratinjau dan permintaan ekspor) tidak perlu berubah untuk keempat poin bila D2 mengikuti saran.

### Ruang lingkup

- Perubahan UI di `features/exports/components/` dan `exportButtonState.ts` di CompanyPortal.
- Penyesuaian test yang ada.

### Tidak termasuk

- Perubahan `backend-exq`, `pkg/csvexport`, OpenAPI, format CSV, siklus `export_jobs`.
- Penggabungan menu ATM dan Kelolaan ATM (D1 dibatalkan).
- Perubahan katalog `/exports`, rute, halaman laporan (DSR telat, replenishment), dan bagian Ekspor Saya.
- Dropdown vendor yang bisa diketik (combobox) atau library dropdown baru.

### Area sensitif (project-context Sec 3 rule 7)

Tidak ada. RBAC, isi file, dan audit tidak berubah. Pengaman batas 1.000.000 baris tetap aktif (D2).

### Dependency & Scope Note

Lanjutan `export-menu-revamp` (X.9). Requirement di bawah menggantikan `export-menu-revamp` Req 5.1 (label tombol dengan jumlah baris) dan Req 5.9 (catatan pengambilan file). Pencatatan di `development-plan.md` mengikuti D5.

## Glossary

- **ExportPreviewPage**: halaman `/exports/<data>` yang berisi filter, tabel pratinjau, jumlah total, dan tombol ekspor.
- **PreviewFilterBar**: form filter di ExportPreviewPage. Isiannya draft sampai tombol Search ditekan.
- **Applied_Filter**: filter yang tersimpan di URL dan dipakai tabel serta permintaan ekspor.
- **VendorSelect**: kontrol pemilih vendor di PreviewFilterBar.
- **PreviewExportBar**: area tombol ekspor di ExportPreviewPage.

## Requirements

### Requirement 1: Filter vendor berupa dropdown saja

**User Story:** Sebagai admin data master, saya ingin memilih vendor dari satu dropdown, supaya filter lebih ringkas.

#### Acceptance Criteria

1. THE VendorSelect SHALL tampil sebagai satu dropdown berlabel "Vendor", tanpa kotak "Cari vendor".
2. THE VendorSelect SHALL menawarkan semua vendor aktif yang dikembalikan endpoint daftar vendor dalam satu permintaan (maks. 100, batas endpoint), dengan label `nama (kode)` seperti sekarang (D4).
3. THE perubahan SHALL berlaku di setiap ExportPreviewPage yang punya filter vendor: Cabang vendor, Vault, PIC vendor, Pemetaan lokasi DSR, dan Harga Paket.
4. WHEN pengguna mengosongkan vendor, THE PreviewFilterBar SHALL tetap mengosongkan dan menyembunyikan filter cabang, seperti sekarang.

### Requirement 2: Tombol Search

**User Story:** Sebagai admin data master, saya ingin tombol penerapan filter berlabel "Search".

#### Acceptance Criteria

1. THE PreviewFilterBar SHALL memberi label "Search" pada tombol yang menerapkan filter, menggantikan "Terapkan" (D3).
2. WHILE draft filter berbeda dari Applied_Filter, THE PreviewExportBar SHALL menampilkan pesan ikon + teks "Klik Search untuk memperbarui tabel sebelum mengekspor." menggantikan "Terapkan filter dulu".
3. THE perilaku tombol Search SHALL sama dengan tombol Terapkan sekarang: menulis draft ke URL, mereset ke halaman pertama, bisa dipicu tombol Enter di form, dan nonaktif selama vendor wajib belum dipilih.

### Requirement 3: Catatan pengambilan file dihapus

**User Story:** Sebagai admin data master, saya ingin area ekspor tanpa catatan teknis yang tidak saya perlukan.

#### Acceptance Criteria

1. THE PreviewExportBar SHALL tidak menampilkan tulisan "File diambil saat ekspor diproses; perubahan data setelah tabel dimuat bisa ikut masuk." (menggantikan `export-menu-revamp` Req 5.9).

### Requirement 4: Tombol ekspor tanpa jumlah baris

**User Story:** Sebagai admin data master, saya ingin tombol ekspor yang ringkas.

#### Acceptance Criteria

1. THE PreviewExportBar SHALL menampilkan tombol berlabel "Ekspor" dengan ikon unduh, tanpa angka jumlah baris (menggantikan `export-menu-revamp` Req 5.1).
2. THE tombol ekspor SHALL tetap nonaktif saat draft belum diterapkan, tabel sedang dimuat, total 0, atau total lebih dari 1.000.000 baris, dengan pesan ikon + teks yang sama (kecuali pesan Req 2.2) (D2).
3. THE ExportPreviewPage SHALL tetap menampilkan "Total N baris" di kepala halaman (D2).

### Requirement 5: Non-regresi dan gate kualitas

**User Story:** Sebagai pengembang, saya ingin perbaikan ini tidak mengubah isi file dan perilaku ekspor lain.

#### Acceptance Criteria

1. THE implementasi SHALL tidak mengubah `backend-exq`, `pkg/`, OpenAPI, dan migrasi, dan tidak menambah library.
2. THE filter selain vendor, paging, URL, katalog, dan alur ExportProgressDialog SHALL tidak berubah.
3. THE file ekspor setiap data master SHALL sama dengan sebelumnya untuk filter yang sama, karena parameter ekspor tidak berubah.
4. THE implementasi SHALL lolos `pnpm run lint`, `pnpm run test:coverage` (CompanyPortal 84/80/80/85), `pnpm run build`, dan `check-bundle.mjs`.

## Open Questions

| # | Pertanyaan | Saran Kiro | Status |
| --- | --- | --- | --- |
| D1 | Arti "ATM dan Kelolaan ATM dijadikan satu" | Opsi A (satu menu + dropdown Jenis data), B (satu CSV gabungan; tidak bisa diimpor ulang karena impor mencocokkan header per entity, `id` rancu, dan `terminal_id` dobel), C (dua tabel di satu halaman). | ❌ DIBATALKAN PO 2026-10-30: permintaan dihapus, menu tetap terpisah |
| D2 | "Button export tidak perlu menghitung jumlah row" mencakup apa? | **A. Hanya label tombol** jadi "Ekspor". "Total N baris" di kepala halaman dan pengaman > 1.000.000 baris tetap. **B.** Hapus juga total di kepala halaman dan query hitung di `backend-exq`: pengguna tidak tahu ukuran data, dan ekspor > 1.000.000 baris baru gagal di worker. | ✅ mengikuti saran A (PO 2026-10-30) |
| D3 | Label "Search" berbahasa Inggris, teks UI lain berbahasa Indonesia | Mengikuti permintaan PO: "Search". | ✅ DIPUTUSKAN PO 2026-10-30 |
| D4 | Tanpa kotak pencarian, dropdown hanya memuat 100 vendor aktif pertama (batas endpoint) | **Terima batas ini.** Seed dev 6 vendor; jumlah vendor CIT jauh di bawah 100. Bila suatu hari > 100, ganti ke endpoint lookup vendor tanpa paging di spec terpisah. | ✅ mengikuti saran (PO 2026-10-30) |
| D5 | Pencatatan di `development-plan.md` | **Follow-up di bawah X.9** (seperti `sidebar-settings-footer` di bawah X.1), bukan item baru. | ✅ mengikuti saran (PO 2026-10-30) |
