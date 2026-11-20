# Penyesuaian Export & Skema Vendor Vaults, Vendor Branches, Vendors, dan Menu Export PKS (`vendor-vaults-export-revamp`)

> Status: accepted 2026-11-11 (user)

## Introduction

### Latar Belakang & Masalah
Menu **Export Data** (`http://localhost:3001/exports`) dan skema master data terkait memerlukan restrukturisasi pada empat area utama:

1. **Vendor Vaults (`/exports/vendor-vaults` & tabel `vendor_vaults`)**:
   - Menghilangkan tampilan kolom `id`, `vendor_code`, `branch_code`, `location_id`, `latitude`, dan `longitude` dari tabel/export `/exports/vendor-vaults`.
   - Menampilkan **`no`** (nomor urut), **`branch_name`**, dan **`vault_name`** (jika belum ada, dibuat/di-backfill dengan format `<vendorPrefix>_<region>_V<3-digit>`, contoh: `ABA_Bali_V001`).
   - Menambahkan kolom **`region`** (dari `vendor_branches.region`) dan **`alamat`** (kolom baru di `vendor_vaults`).
   - Mengubah nama kolom kapasitas menjadi **`min_balance`** (dari `min_capacity_amount`) dan **`max_balance`** (dari `max_capacity_amount`).
   - Memindahkan kolom **`cis_limit_amount`** (beserta `cis_limit_currency`) secara fisik (**Opsi 1**) dari tabel `vendor_branches` ke tabel `vendor_vaults`.
   - Mengganti kolom tunggal **`escrow_account`** menjadi dua kolom terpisah: **`escrow_cash`** dan **`escrow_atm`**.

2. **Vendor Branches (`/exports/vendor-branches` & tabel `vendor_branches`)**:
   - Menghapus `cis_limit_amount` (dan `cis_limit_currency`) dari `vendor_branches` serta dari export/import `/exports/vendor-branches` dan form cabang karena telah dipindahkan ke `vendor_vaults`.

3. **Vendors (`/exports/vendors`)**:
   - Menghilangkan kolom **`id`** dan **`code`** dari tabel `/exports/vendors`.
   - Memisahkan seluruh kolom Direktur Utama & PKS dari `/exports/vendors` sehingga `/exports/vendors` fokus pada profil vendor (`name`, `legal_name`, `npwp`, `contact_email`, `contact_phone`, `hq_address`, `is_active`).

4. **Menu Export PKS Tersendiri (`/exports/vendor-pks`)**:
   - Membuat menu export tersendiri di katalog `/exports` khusus untuk data Vendor + Direktur Utama + PKS (sesuai struktur `vendor_pks` yang sudah ada di backend).

### Hasil yang Diharapkan (Proposed Outcome)
- Skema database terbarui melalui migrasi forward-only (`040_vendor_vaults_revamp.sql`).
- Tabel pratinjau & unduhan CSV pada `/exports/vendor-vaults`, `/exports/vendor-branches`, `/exports/vendors`, dan menu baru `/exports/vendor-pks` tampil sesuai daftar dan urutan kolom yang telah ditetapkan.
- Perubahan diterapkan secara menyeluruh pada Tabel Preview, File CSV Export, CSV Import, dan Form Admin UI (`VendorVaultFormDialog` & `VendorBranchFormDialog`).

---

## Keputusan & Jawaban Open Questions (Dikonfirmasi User — 2026-11-11)

1. **A1 — Urutan & Daftar Kolom `/exports/vendor-vaults` (14 Kolom)**:
   Urutan kolom yang ditampilkan:
   1. `no`
   2. `branch_name`
   3. `vault_name` *(jika belum ada, format `<vendorPrefix>_<area/region>_V<3 digit mulai 001>`, contoh: `ABA_Bali_V001`)*
   4. `category`
   5. `operating_hours`
   6. `currency_code`
   7. `region`
   8. `alamat`
   9. `min_balance` *(rename dari `min_capacity_amount`)*
   10. `max_balance` *(rename dari `max_capacity_amount`)*
   11. `cis_limit_amount`
   12. `escrow_cash`
   13. `escrow_atm`
   14. `is_active`
   *(Kolom `id`, `vendor_code`, `branch_code`, `location_id`, `latitude`, `longitude` tidak dimunculkan).*

2. **A2 — Sumber Data `region` & `alamat`**:
   - `region` diambil dari `vendor_branches.region` melalui `JOIN vendor_branches`.
   - `alamat` menjadi kolom baru di tabel `vendor_vaults`.

3. **A3 — Pemisahan `/exports/vendors` dan Menu Export PKS**:
   - Kolom Direktur Utama (`director_name`, `director_phone`, `director_email`) + PKS ikut di menu **Export PKS (`/exports/vendor-pks`)**.
   - `/exports/vendors` hanya menampilkan profil vendor tanpa `id` & `code`: `name`, `legal_name`, `npwp`, `contact_email`, `contact_phone`, `hq_address`, `is_active`.

4. **A4 — Cakupan Perubahan**:
   - Diterapkan menyeluruh: **Skema DB + Tabel Preview + File CSV Export + CSV Import + Form Admin UI** (`VendorVaultFormDialog` & `VendorBranchFormDialog`).

---

## Requirements (EARS Format)

### Requirement 1: Restrukturisasi Skema & Kolom Vendor Vaults (`/exports/vendor-vaults`)
1. ** R1.1 (Urutan & Visibilitas Kolom)**: WHEN pengguna membuka pratinjau `http://localhost:3001/exports/vendor-vaults` atau mengunduh file CSV `vendor-vaults`, THE SYSTEM SHALL menampilkan tepat 14 kolom dengan urutan:
   `no`, `branch_name`, `vault_name`, `category`, `operating_hours`, `currency_code`, `region`, `alamat`, `min_balance`, `max_balance`, `cis_limit_amount`, `escrow_cash`, `escrow_atm`, `is_active`.
2. **R1.2 (Penyembunyian Kolom Lama)**: THE SYSTEM SHALL NOT menampilkan kolom `id`, `vendor_code`, `branch_code`, `location_id`, `latitude`, dan `longitude` pada `/exports/vendor-vaults`.
3. **R1.3 (Penamaan Otomatis `vault_name`)**: WHERE sebuah baris `vendor_vaults` belum memiliki `vault_name` (atau saat backfill migrasi dan pembuatan vault baru tanpa `vault_name` eksplisit), THE SYSTEM SHALL membentuk `vault_name` dengan format `<vendorPrefix>_<BranchArea>_V<3-digit-seq>` (contoh: `ABA_Bali_V001`, menggunakan `vendors.request_prefix` dan urutan 3 digit mulai dari `001`).
4. **R1.4 (Pemisahan `escrow_cash` & `escrow_atm`)**: WHEN data vault dibuat atau diperbarui melalui Form Admin UI atau CSV Import, THE SYSTEM SHALL memvalidasi `escrow_cash` dan `escrow_atm` sebagai string 12 digit angka opsional (`^[0-9]{12}$` atau `NULL`, tidak unik).
5. **R1.5 (Pemindahan `cis_limit_amount` ke `vendor_vaults`)**: THE SYSTEM SHALL menyimpan `cis_limit_amount` (`numeric(20,2) NULL CHECK (cis_limit_amount > 0)`) dan `cis_limit_currency` (`character(3) NOT NULL DEFAULT 'IDR'`) pada tabel `vendor_vaults` serta menghapusnya dari `vendor_branches`.

### Requirement 2: Penyesuaian Vendor Branches (`/exports/vendor-branches`)
1. **R2.1**: WHEN pengguna membuka `/exports/vendor-branches` atau mengunduh CSV `vendor-branches`, THE SYSTEM SHALL menampilkan kolom cabang tanpa `cis_limit_amount`.
2. **R2.2**: WHEN pengguna membuka Form Tambah/Ubah Cabang (`VendorBranchFormDialog`), THE SYSTEM SHALL tidak lagi menampilkan input `cis_limit_amount` (karena telah berpindah ke `VendorVaultFormDialog`).

### Requirement 3: Penyesuaian Export Vendors (`/exports/vendors`)
1. **R3.1**: WHEN pengguna membuka `http://localhost:3001/exports/vendors` atau mengunduh CSV `vendors`, THE SYSTEM SHALL hanya menampilkan kolom profil vendor:
   `name`, `legal_name`, `npwp`, `contact_email`, `contact_phone`, `hq_address`, `is_active` (tanpa `id`, `code`, `director_*`, dan `pks_*`).

### Requirement 4: Menu Export PKS Tersendiri (`/exports/vendor-pks`)
1. **R4.1**: WHEN pengguna dengan role `ADMIN` atau `ADMIN_PARAM` membuka katalog `/exports`, THE SYSTEM SHALL menampilkan kartu menu dataset **PKS Vendor** (`/exports/vendor-pks`).
2. **R4.2**: WHEN pengguna membuka `/exports/vendor-pks` atau mengekspor CSV-nya, THE SYSTEM SHALL menampilkan data Vendor beserta Direktur Utama dan PKS aktif (`vendor_code`, `vendor_name`, `director_name`, `director_phone`, `director_email`, `pks_name`, `pks_effective_date`, `pks_expiry_date`, `pks_validity_months`, `pks_remaining_days`, `pks_status`).

### Requirement 5: Maker-Checker, Audit, & Presisi Uang (Golden Rules)
1. **R5.1**: WHEN perubahan pada `vendor_vaults` atau `vendor_branches` diajukan melalui Form Admin UI maupun CSV Import, THE SYSTEM SHALL memprosesnya melalui `MasterDataChangeService.Submit` (`202 Accepted`) dan mencatat `audit_logs` dalam transaksi yang sama saat approve.
2. **R5.2**: THE SYSTEM SHALL merepresentasikan `min_balance`, `max_balance`, dan `cis_limit_amount` menggunakan `numeric(20,2)` di database dan string desimal di JSON/CSV tanpa menggunakan `float`.
