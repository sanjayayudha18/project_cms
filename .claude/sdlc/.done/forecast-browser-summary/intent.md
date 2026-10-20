# Intent: Forecast Browser — ringkasan ATM yang perlu replenish (tanpa pilih vendor/region satu per satu)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-09-30** (user menjawab OQ1–5 dan meminta lanjut).
Stage: 1 Plan — selesai. Stage berikut: `spec.md`.
Halaman: `/replenishment/forecast-browser` (`frontend/CompanyPortal-Vite/src/features/vendor-request/ForecastBrowser.tsx`).
Spec lama yang disentuh: `.kiro/specs/done/cit-vendor-request-enhancements` (CIT-2 Req 1.4 — FLM Vendor + Region wajib),
`.kiro/specs/done/update-cit-forecast-browser`, `.kiro/specs/done/replenishment-request-enhancements`.

## Problem
Kata user (2026-09-30): *"saya ingin user lebih mudah melihat atm mana saja yg perlu replenish. saat ini user harus
pilih satu2 per vendor dan per region. saya takutkan akan ada yg miss kalau desain seperti ini. saya ingin desain yg
tetap simple tapi tetap informatif."*

Kondisi sekarang (dicek di kode):
- Tabel rekomendasi DMAA baru muncul setelah **FLM Vendor dan FLM Vendor Region dua-duanya dipilih** (CIT-2 Req 1.4);
  backend `BrowseForecast` juga menolak request tanpa keduanya.
- User tidak tahu kombinasi vendor × region mana yang punya rekomendasi, dan mana yang sudah dibuatkan Vendor Request —
  harus mencoba satu per satu.
- ATM **tanpa paket vendor aktif** (`flm_vendor` kosong) tidak pernah bisa tampil lewat filter mana pun → hilang tanpa peringatan.
- Tidak ada penanda ATM yang sudah masuk Vendor Request → risiko request dobel atau terlewat.

## Proposed outcome
- Saat halaman dibuka (tanggal forecast default H+1 kerja), user langsung melihat **ringkasan**:
  - KPI: total ATM perlu isi · sudah di-request · belum di-request · tanpa vendor aktif.
  - Tabel per **Vendor × Region**: jumlah ATM, total amount (IDR), sudah, belum, status (label + ikon, bukan warna saja),
    diurutkan dari "belum" terbanyak.
  - Baris khusus **"Tanpa vendor aktif"** dengan peringatan "perlu master data".
- Klik satu baris ringkasan → tabel detail yang sudah ada, filter vendor + region terisi otomatis.
- Di tabel detail, ATM yang sudah masuk Vendor Request ditandai **"Sudah di-request"** dan tidak bisa dicentang.
- Setelah Vendor Request dibuat dan user kembali, angka "belum" turun → ringkasan berfungsi sebagai checklist harian.

## Affected users and systems
- Users: ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV (role route yang sudah ada).
- Systems: `backend/` (`internal/service/vendor_request_*`, `internal/handler`, `queries/vendor_request.sql`),
  `frontend/CompanyPortal-Vite/src/features/vendor-request/`. Tabel yang dibaca: `dmaa_atm_forecast`, `atms`,
  `atm_vendor_packages`, `vendor_packages_branch`, `vendor_branches`, `vendors`, `vendor_requests`, `vendor_request_items`.

## Constraints
- **Read-only**: tidak ada tabel/kolom/migrasi baru; tidak mengubah alur create/approve Vendor Request.
- Aturan **1 Vendor Request = 1 vendor** (CIT-2 Req 4) tetap berlaku.
- Ringkasan dibaca dari **read replica** (Sec 6); penanda "sudah di-request" di halaman yang sama boleh tertinggal
  sebentar (replica lag) — backend create tetap sumber kebenaran.
- Resolusi vendor/region per ATM harus **sama persis** dengan `ListForecastForDate` (paket aktif per tanggal forecast),
  supaya angka ringkasan = jumlah baris di detail.
- Uang: amount dijumlahkan di SQL (numeric/bigint), tampil `tabular-nums`, IDR eksplisit, rata kanan (Sec 13).
- NFR: ringkasan ≤3 s p95 (setara dashboard, Sec 3a).
- Response flat JSON seperti handler ATM tetangga (Sec 5).

## Resolved decisions (tanya-jawab dengan user, 2026-09-30)
1. Dokumentasi lewat AI-DLC penuh (folder ini), bukan `bugfixes.md`.
2. **"Sudah di-request"** = ATM (terminal_id + periode_pred + denom) ada di `vendor_request_items` milik Vendor Request
   berstatus apa pun **kecuali `cancelled`/`rejected`** (juga `is_canceled=true` dihitung batal). ATM dari request yang
   dibatalkan/ditolak kembali dihitung "belum" dan boleh di-request ulang.

## Open questions — dijawab user 2026-09-30
1. Filter FLM Vendor/Region di tabel detail **boleh "Semua"** (tidak wajib lagi — mengubah CIT-2 Req 1.4).
   Tombol "Buat Vendor Request" tetap butuh satu vendor.
2. ATM "Tanpa vendor aktif": **cukup ditampilkan** (tanpa Manual Request dari halaman ini).
3. **Tidak** ada pembatasan data per cabang untuk BRANCH-ATM-USER/SPV (sama dengan hari ini).
4. Ekspor CSV ringkasan: **nanti** (out of scope).
5. ATM yang sudah di-request **tetap tampil** di tabel detail, dengan penanda.

## Out of scope
- Perubahan rumus/forecast DMAA, alur approval Vendor Request, atau pembuatan request otomatis.
- Perbaikan master data ATM tanpa vendor (hanya ditampilkan).
- Ekspor CSV ringkasan (OQ4) dan pembatasan data per cabang (OQ3).
