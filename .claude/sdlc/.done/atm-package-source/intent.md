# Intent: Pilihan sumber paket pada Kelolaan ATM (paket khusus cabang vs paket vendor-wide)

Author: user (product owner), ditulis bersama Claude. Status: **draft 2026-10-07** — menunggu jawaban pertanyaan terbuka + penerimaan PO.
Stage: 1 Plan. Stage berikut: `spec.md`.
Menyentuh Sec 4 #7: **skema (FK `atm_vendor_packages`)** + **alur maker-checker (`ATMAssignmentApplier`)** → wajib AI-DLC penuh (dikonfirmasi user 2026-10-07).

## Problem
Kata user (2026-10-07, dengan screenshot dialog "Kelolaan ATM 1004"): *"claude perlu perbaikan untuk pemilihan paket ATM.
user tetap bisa memilih mau pakai paket khusus per cabang vendor atau paket yg mencangkup seluruh vendor."*

Kondisi sekarang (dicek di kode):
- Dialog `ATMAssignmentsDialog.tsx` → bagian "Tetapkan paket baru": Vendor → Cabang → Paket. Dropdown Paket diisi
  `listVendorPackages(vendorId, {branch_id})`, yaitu **hanya** `vendor_packages_branch` (harga khusus cabang, migrasi 016).
  Opsi cabang "Semua cabang" hanya melonggarkan filter `branch_id`; ia tidak memunculkan paket vendor-wide.
- Paket yang mencakup seluruh vendor ada di tabel lain: `vendor_package_prices` (pohon PT / cabang / override ATM, migrasi 009/017).
  Pemilih paket tidak membacanya.
- `atm_vendor_packages.vendor_package_id` adalah **FK NOT NULL ke `vendor_packages_branch`** (baseline; dicatat sebagai BREAKING di
  migrasi 016). Maka sekarang sebuah ATM **tidak bisa** dikelola dengan paket vendor-wide tanpa perubahan skema.
- Konsumen jalur `atms → atm_vendor_packages → vendor_packages_branch` (kuota kunjungan `atm-visit-quota`, `ListForecastForDate`,
  harga/penagihan) mengasumsikan sumbernya selalu `vendor_packages_branch`.

## Proposed outcome
1. Di dialog Kelolaan ATM, user memilih **jenis paket**: *paket khusus cabang* (dari `vendor_packages_branch`) **atau**
   *paket vendor-wide* (dari `vendor_package_prices`). Keduanya tetap tersedia; user yang memutuskan per penetapan.
2. Paket yang dipilih tersimpan sebagai kelolaan ATM (periode efektif, maker-checker, audit tetap seperti sekarang).
3. Daftar periode di dialog menunjukkan paket aktif beserta jenis sumbernya.

## Affected users and systems
- Users: ADMIN / ADMIN_PARAM (menetapkan kelolaan ATM); siapa pun yang membaca paket aktif ATM (SPV, operator).
- Systems: `backend/` (migrasi baru, `queries/atm_assignment*.sql`, `service/atm_assignment*`, `ATMAssignmentApplier`, handler),
  `frontend/CompanyPortal-Vite/src/features/admin-atms/` (dialog, api, types) dan `admin-vendors` (sumber opsi paket).
- Tabel: diubah `atm_vendor_packages` (cara merujuk paket); dibaca `vendor_package_prices`, `vendor_packages_branch`,
  `package_frequencies`, `atms`. Perubahan skema diajukan di spec dan ke CLAUDE.md Sec 3 sebelum migrasi.

## Constraints
- Maker-checker + `audit_logs` pada setiap penetapan (Sec 5, Sec 12 D1–D4); RBAC di route dan service.
- Tidak ada hard delete; periode efektif tidak boleh tumpang tindih per ATM (aturan yang sudah ada).
- Baris `atm_vendor_packages` yang sudah ada tetap valid dan tidak dimigrasi paksa.
- Money numeric, bukan float (Sec 6); tidak ada perubahan pada harga itu sendiri.
- Konsumen paket aktif ATM (kuota kunjungan, forecast, harga) tidak boleh rusak diam-diam: harus eksplisit menangani kedua sumber.
- Sebelum migrasi, rancangan skema disetujui (CLAUDE.md Sec 3: "Need a new table/column? Propose here FIRST").

## Resolved decisions (tanya-jawab satu per satu dengan user, 2026-10-07)
1. Paket vendor-wide dipilih sebagai **label paket** (mis. "PAKET 4") milik satu vendor; harga dicari saat dibutuhkan lewat
   `machine_group`/`price_class` ATM dan tingkat tarif yang berlaku. Bukan baris tarif tertentu.
2. Skema: `atm_vendor_packages.vendor_package_id` jadi **nullable**; tambah kolom `vendor_id`, `vendor_branch_id` + `package` (label); CHECK **tepat satu mode terisi**
   (cabang: hanya `vendor_package_id`; vendor-wide: `vendor_id` + `vendor_branch_id` + `package`). Tidak ada tabel baru; baris lama tidak berubah.
   (Direvisi 2026-10-07, lihat butir 6: cabang pengelola disimpan eksplisit di mode vendor-wide.)
3. Kuota kunjungan: di mode vendor-wide label `package` langsung dipakai join ke `package_frequencies.package_code`
   (default; mode cabang tetap memakai `vendor_packages_branch.package_code`). *Default Claude, bukan pertanyaan ke user — koreksi bila salah.*
4. Konsumen yang harus ikut menangani dua sumber (query yang join `atm_vendor_packages` → `vendor_packages_branch`):
   `atms_admin.sql`, `atm_assignments_admin.sql`, `atm_visit_quota.sql`, `vendor_request.sql`, `vendor_branch_atms.sql`, `master_data_export.sql`.
5. Mode vendor-wide **wajib divalidasi**: pengajuan ditolak bila vendor tidak punya tarif aktif untuk label itu pada `machine_group`/`price_class` ATM.
   Mode cabang tetap tanpa validasi (gap diterima 2026-09-23).
6. UI (koreksi user 2026-10-07): **cabang vendor pengelola ATM tetap dipilih di kedua mode** (Vendor → Cabang selalu tampil dan dipakai).
   Yang berbeda hanya sumber **Paket**: pilihan jenis paket (Khusus cabang / Seluruh vendor) di atas dropdown Paket. Mode khusus cabang:
   paket dari cabang terpilih. Mode seluruh vendor: label paket milik vendor terpilih, cabang tetap tercatat sebagai pengelola.
   Opsi "Semua cabang" dihapus (cabang wajib dipilih).

## Untuk diputuskan di spec (teknis)
- Nama kolom/constraint pasti, exclusion "tidak tumpang tindih per ATM" lintas dua mode, bentuk respons API (`source`), cara
  `ListATMAssignments` menampilkan kode paket mode vendor-wide, dan perubahan tiap query konsumen di atas.

## Out of scope
- Mengubah harga/tier di `vendor_package_prices` atau `vendor_packages_branch`.
- Migrasi massal baris kelolaan lama ke sumber lain.
- Layar baru untuk `package_frequencies`.
