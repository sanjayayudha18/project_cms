# Intent: Limit CIS per area vault vendor + escrow vault

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-11-06** (user: "Terima, buat spec"); open questions dijawab 2026-11-06.
Dipisah dari `vendor-pks-cis-limit` (O8, PO 2026-11-05: "Pisah, PKS dulu"); fitur PKS lanjut di `.kiro/specs/vendor-pks/`.
Stage: 1 Plan. Stage berikut: `spec.md` (setelah intent diterima).
Menyentuh Sec 4 #7: **skema (kolom baru)** + **uang (limit)** + **maker-checker master data** → wajib AI-DLC penuh.

## Problem
Sumber: FSD "DSR End To End Cash Management" v1.0 (1-Sept-2026), bab "Management vendor dan master data":
- Master Detail kelolaan ATM vendor: *"Ops team dapat melakukan input dan maintain limit CIS pada masing-masing vendor
  (field Limit CIS "editable")"*, dicari berdasarkan Nama Perusahaan + Area Vault Vendor.
- Field Master Kelolaan ATM (tabel FSD): Nama Perusahaan, Area Vault Vendor, Alamat Vault, Region, Nama PIC, Phone, Email,
  Service Type (ATM / CASH / ATM & CASH), ATM Escrow, Cash Escrow, Limit CIS.

Desain layar dari user: `.document/master_data/desain-dari-user_vendor/Field-Master-data-vendor-pks.xlsx`, sheet
**Detail inquiry**: cari per Nama Perusahaan + Status Asuransi (picklist RGA); kolom Nama Perusahaan, Region, Location,
Nama BM, Contact BM, Service Type, No Rek ATM, No Rek Cash, Limit Asuransi, Posisi Saldo (*"ambil dari SIBS posisi 1 bulan
terakhir"*), Status (Covered / Over, *"Rules Parameterize"*); tombol "generate report". Sheet **Menu Input Parameter**: TBA.

Kondisi sekarang (DB dev 2026-11-05): `vendor_branches` (= Area Vault Vendor) tanpa Limit CIS; `vendor_vaults` tanpa nomor
escrow (`atms.escrow_account` ada, per ATM); belum ada data saldo escrow SIBS / Corebanking di CROWN.

## Keputusan yang dibawa dari `vendor-pks-cis-limit` (PO 2026-10-05 — dikonfirmasi ulang saat intent ini dibahas)
- **A1 (bagian CIS)** Kolom disetujui: `vendor_branches.cis_limit_amount/cis_limit_currency`, `vendor_vaults.escrow_account`.
  Migrasi semula `022`; `022` kini dipakai `vendor-pks` → fitur ini memakai nomor bebas berikutnya.
- **A2** Area Vault Vendor = `vendor_branches`. Satu escrow per vault; area dengan ATM + Cash escrow memakai vault ATM dan
  vault CASH terpisah (bukan `ATM_CASH`).
- **A3** Edit Limit CIS lewat **maker-checker** master data (GR#3 dipertahankan).
- **A5** Coverage CIS: rata-rata saldo ≤ limit → Covered.
- **A6** **Monitoring Limit CIS ditunda** sampai ingest saldo escrow harian ada.
- **Q7** `vendor_vaults.escrow_account`: opsional, 12 digit angka, unik di antara vault aktif (⚠️ FSD cash count menyebut
  escrow yang sama bisa dikelola beberapa vendor — tinjau ulang).
- **O2 (dari vendor-pks)** Service Type = kategori vault yang dipilih user (`vendor_vaults.category`), tanpa kolom baru.

## Open questions (semua dijawab 2026-11-06)
1. ~~"Limit Asuransi" di xlsx = "Limit CIS" di FSD (satu nilai yang sama), atau dua hal berbeda?~~
   **Dijawab 2026-11-06 (user):** sama, satu nilai → satu kolom `vendor_branches.cis_limit_amount` per area vault vendor.
2. ~~Nama BM + Contact BM: siapa BM (Branch Manager vendor per area?), dan apakah cukup memakai `vendor_pics` (peran baru)
   atau kolom baru?~~
   **Dijawab 2026-11-06 (user):** BM = Branch Manager vendor per area vault; pakai `vendor_pics` yang terikat ke cabang itu
   dengan peran baru `bm` di `vendor_pics.roles` (CHECK ⊆ {replenish, vault, cashcount} dari migrasi 032 perlu diperluas).
   Tanpa kolom baru di `vendor_branches`.
> Pertanyaan 3–7 + tinjauan Q7: user 2026-11-06 *"semua open question saya ikuti saran"* → jawaban = saran Claude.
3. ~~Kolom "Location" di Detail inquiry = lokasi area vault (`vendor_branches.location_id`) atau lokasi vault?~~
   **Dijawab:** lokasi area vault (`vendor_branches.location_id`); satu baris inquiry per area vault vendor (grain = Limit CIS).
4. ~~No Rek ATM / No Rek Cash = `escrow_account` vault ATM / vault CASH (A2)?~~
   **Dijawab:** ya — No Rek ATM = `escrow_account` vault aktif kategori `ATM`, No Rek Cash = vault aktif kategori `CASH` di
   area itu. Lebih dari satu vault per kategori → semua nomor ditampilkan; tidak ada → kosong.
5. ~~Posisi Saldo SIBS + status Covered / Over: masuk fitur ini atau tetap ditunda (A6)?~~
   **Dijawab:** tetap ditunda (A6) sampai ingest saldo escrow harian ada. Kolom Posisi Saldo + Status tidak dibangun sekarang.
6. ~~"Rules Parameterize" + sheet Menu Input Parameter (TBA): parameter apa saja?~~
   **Dijawab:** ditunda bersama #5 (rules hanya dipakai untuk status Covered / Over). Aturan awal tetap A5.
7. ~~Filter "Status Asuransi" + "generate report" untuk Detail inquiry: masuk fitur ini?~~
   **Dijawab:** filter Status Asuransi ditunda bersama #5 (bergantung pada status). Layar Detail inquiry masuk fitur ini
   (cari per Nama Perusahaan, read replica); "generate report" (CSV) ditunda ke fase monitoring.
- **Tinjauan Q7 (`escrow_account` unik):** **Dijawab:** tidak unik (FSD cash count: satu escrow bisa dikelola beberapa
  vendor). Tetap opsional + validasi 12 digit angka.

## Out of scope (sementara)
- PKS vendor + data direktur → `.kiro/specs/vendor-pks/`.
- Ingest saldo escrow SIBS / Corebanking (`escrow_batch_rows`), kecuali diputuskan lain di pertanyaan 5.
