# Intent: PKS vendor + Limit CIS per area vault

Author: user (product owner), ditulis bersama Claude. Status: **draft 2026-10-05 — menunggu PO accept**.
Stage: 1 Plan. Stage berikut: `spec.md` (setelah intent diterima).
Menyentuh Sec 4 #7: **skema (kolom baru, migrasi `022`)** + **maker-checker master data** → wajib AI-DLC penuh.

## Problem
Sumber: FSD "DSR End To End Cash Management" v1.0 (1-Sept-2026), bab "Management vendor dan master data"
(`DSR End To End Cash Management V01.docx` di repo root). Kutipan FSD:

- Master Data Vendor Inquiry: *"Menu ini digunakan untuk melakukan inquiry data vendor untuk dapat memonitoring masa
  aktif PKS vendor ... System akan melakukan perhitungan untuk durasi expired PKS setiap kali menu ini di akses."*
  Flag *"Green" (expiry date > 9 bulan)* atau *"Amber" (expiry date < 9 bulan)*; *"Untuk expiry date yang lebih kecil dari
  current date akan mengupdate informasi status menjadi "Red" (Expired)"*; *"fitur untuk dapat meng-update field status
  PKS secara manual untuk vendor yang sudah di "Terminate" ... dilakukan secara manual oleh tim OPS berdasarkan
  "Nama Perusahaan""*.
- Master Detail kelolaan ATM vendor: *"Ops team dapat melakukan input dan maintain limit CIS pada masing-masing vendor
  (field Limit CIS "editable")"*, dicari berdasarkan Nama Perusahaan + Area Vault Vendor.
- Field Master Vendor (tabel FSD): Nama Perusahaan, Alamat Kantor Pusat, Nama Direktur Utama, Informasi Contact Direktur
  Utama, PKS Efektif, Tanggal Efektif PKS, Tanggal Expired PKS, Masa Berlaku PKS, Status PKS
  (Active/Expired/Terminated), Monitoring PKS (sisa waktu), Flag.
- Field Master Kelolaan ATM (tabel FSD): Nama Perusahaan, Area Vault Vendor, Alamat Vault, Region, Nama PIC, Phone, Email,
  Service Type (ATM / CASH / ATM & CASH), ATM Escrow, Cash Escrow, Limit CIS.

Kondisi sekarang (dicek di kode/skema 2026-10-05):
- `vendors` punya `code`, `name`, `legal_name`, `npwp`, `hq_address`, `contact_email/phone`, `kind` — **tidak ada** data
  direktur maupun PKS.
- `vendor_branches` (= Area Vault Vendor) punya `branch_name`, `region` — **tidak ada** Limit CIS.
- `vendor_vaults` punya `category` (ATM/CASH/ATM_CASH), lokasi, kapasitas — **tidak ada** nomor escrow
  (`atms.escrow_account` ada, tapi per ATM, bukan per vault).
- Create/update `vendors`, `vendor_branches`, `vendor_vaults` sudah maker-checker (`VendorApplier`,
  `VendorBranchApplier`, `VendorVaultApplier` di `cmd/api/main.go`, endpoint 202); CSV import/export master data sudah
  mencakup ketiga entitas (`internal/service/masterdata_import*.go`).
- Layar: `frontend/CompanyPortal-Vite/src/features/admin-vendors` (detail vendor, cabang, vault).

## Proposed outcome
1. **Data PKS per vendor**: OPS mengisi Nama Direktur Utama, kontak direktur, nama PKS, tanggal efektif dan tanggal
   expired PKS lewat form vendor yang sudah ada (maker-checker).
2. **Monitoring PKS di layar vendor**: setiap kali dibuka, sistem menghitung masa berlaku (tahun), sisa waktu sampai
   expired, status, dan flag:
   - Terminated → status **Terminated** (diset manual).
   - Tanggal expired < hari ini → **Expired**, flag **Red**.
   - Sisa < 9 bulan → **Active**, flag **Amber**.
   - Sisa ≥ 9 bulan → **Active**, flag **Green**.
3. **Terminate PKS manual**: OPS mencari vendor berdasarkan Nama Perusahaan lalu menandai PKS **Terminated** (maker-checker).
4. **Limit CIS per area vault vendor**: OPS mengisi dan mengubah nominal Limit CIS (IDR) per area vault
   (`vendor_branches`), lewat maker-checker.
5. **Nomor escrow per vault**: setiap vault punya satu nomor escrow — vault ATM = ATM Escrow, vault CASH = Cash Escrow.
6. Data baru ikut tampil di layar master vendor / kelolaan, sesuai field FSD di atas.

## Affected users and systems
- Users: tim OPS / Cash & Wealth Management Operation (maker), checker master data; role yang sudah dipakai master
  data (`ADMIN` / `ADMIN_PARAM`).
- Systems: `backend/` (migrasi `022`, `queries/`, service + applier vendor/branch/vault, handler admin vendor),
  `frontend/CompanyPortal-Vite` (`features/admin-vendors`).
- Tabel: diubah `vendors`, `vendor_branches`, `vendor_vaults` (kolom baru). Tidak ada tabel baru.

## Constraints
- Semua create/update lewat **maker-checker master data** yang ada (Submit → `approval_requests` → Applier → 202), audit
  dalam tx yang sama (Sec 5, Sec 12 D1). Tidak ada deviasi GR#3.
- RBAC di route **dan** service (pola `masterDataAdmin`).
- Uang: Limit CIS `numeric(20,2)` + kode mata uang eksplisit (default IDR), tidak boleh float (Sec 6).
- Status/flag/sisa waktu PKS **dihitung saat dibaca**, tidak disimpan (FSD: dihitung tiap menu diakses). Yang disimpan hanya
  tanggal + penanda Terminated.
- Ambang Amber = **9 bulan**, sebagai konstanta/config (FSD juga menyebut 8 bulan — bisa diubah tanpa migrasi).
- Tidak ada hard delete (Sec 12). Bacaan layar boleh dari replica; tulis ke primary (Sec 6).
- Migrasi plain SQL bernomor `022_...` (021 sudah dipakai atm-visit-quota), diterapkan manual ke dev.

## Resolved decisions (tanya-jawab satu per satu dengan user, 2026-10-05)
1. **A1** Kolom disetujui: `vendors.director_name/director_contact/pks_name/pks_effective_date/pks_expiry_date/pks_terminated_at`,
   `vendor_branches.cis_limit_amount/cis_limit_currency`, `vendor_vaults.escrow_account` → migrasi `022`.
2. **A2** Area Vault Vendor = `vendor_branches`. Satu escrow per vault; area dengan ATM + Cash escrow memakai vault ATM dan
   vault CASH terpisah (bukan `ATM_CASH`).
3. **A3** Edit Limit CIS lewat **maker-checker** master data (FSD tidak menyebut checker; GR#3 dipertahankan).
4. **A4** Ambang Amber PKS **9 bulan**, sebagai parameter.
5. **A5** Coverage CIS: rata-rata saldo ≤ limit → Covered (dipakai saat monitoring dibangun nanti).
6. **A6** **Monitoring Limit CIS ditunda** sampai ingest saldo escrow harian ada; fitur ini hanya PKS + input Limit CIS.

## Resolved open questions (tanya-jawab satu per satu dengan user, 2026-10-05)
PKS = Perjanjian Kerja Sama (kontrak bank ↔ vendor).
1. **Satu PKS per vendor**; perpanjangan menimpa kolom di `vendors`, riwayat lewat `audit_logs` (before/after). Tidak ada tabel riwayat.
2. **Terminated = status PKS saja**; vendor tetap aktif (disable vendor tetap aksi terpisah). **Bisa dibatalkan**
   (un-terminate, `pks_terminated_at` dikosongkan) — keduanya lewat maker-checker.
3. Kolom PKS **opsional** (vendor lama boleh kosong). Vendor `kind='INTERNAL'` (ROH) **dikecualikan** — tidak punya PKS,
   tidak dihitung flag-nya.
4. Vendor FLM tanpa tanggal PKS → status **"Belum diisi"**, badge netral (abu-abu), bukan Green/Amber/Red.
5. Kolom baru (PKS, Limit CIS, escrow vault) **ikut CSV import/export** master data yang ada (pola upsert sekarang).
   Replace-all FSD tetap fitur terpisah.
6. Daftar vendor dapat kolom flag + **filter** Green / Amber / Red / Terminated / Belum diisi, dihitung **server-side**
   (konsisten dengan paging yang ada).
7. `vendor_vaults.escrow_account`: opsional, **12 digit angka**, **unik di antara vault aktif**.
   ⚠️ Catatan: bagian cash count FSD menyebut "nomor escrow yang sama dikelola beberapa vendor" — kalau itu terjadi di
   data nyata, aturan unik ini perlu ditinjau ulang (fitur cash count).
8. Akses lihat = **role master data yang ada** (tanpa role baru, mengikuti `role_permissions` menu vendor). Tidak tampil di
   VendorPortal (VendorPortal tidak punya layar master vendor).

Tidak ada open question bisnis tersisa untuk fitur ini.

## Untuk diputuskan di spec (teknis)
- Bentuk penanda Terminated (`pks_terminated_at` + alasan/oleh siapa?) dan op maker-checker-nya (update biasa vs op khusus
  terminate/un-terminate).
- Query flag server-side untuk filter daftar vendor (ekspresi tanggal di SQL vs kolom generated — status tidak disimpan).
- Partial unique index `escrow_account` WHERE vault aktif, dan perilaku CSV import untuk kolom baru.
- Lokasi konstanta 9 bulan (Go const vs env/config) dan cara hitung "sisa bulan" (pembulatan, zona waktu Asia/Jakarta).
- CHECK constraint (`pks_expiry_date >= pks_effective_date`, `cis_limit_amount >= 0`).
- Perubahan respons API (field baru di list/detail vendor, branch, vault) dan komponen UI yang terdampak.

## Out of scope
- **Menu Monitoring Limit CIS** (min/max/rata-rata saldo escrow vs limit) — menunggu ingest saldo escrow (A6).
- Ingest saldo escrow SIBS / Corebanking (`escrow_batch_rows`).
- Upload master **replace-all** versi FSD (fitur terpisah, keputusan #3 FSD alignment).
- Notifikasi/email PKS mendekati expired (menunggu modul notifikasi 0.3).
- Cash count, PIC coordinator/executor, kalender libur.
- Data PIC per area (sudah ada di `vendor_pics`).
