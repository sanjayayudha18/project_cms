# Intent: Kirim Vendor Request `ready` ke vendor + terima/tolak di VendorPortal (Phase 2.2b)

Author: user (product owner), ditulis bersama Claude.
Status: **accepted — 2026-10-09** (user: "diterima, lanjut spec").
Stage: 1 Plan. Stage berikut: `spec.md`.
Menyentuh Sec 4 #7: **auth/RBAC (scoping user vendor per vendor/branch, data lintas vendor)**, **alur approval Vendor
Request (status baru, alur balik)**, **skema baru** → wajib AI-DLC penuh.

## Problem — kata user (2026-10-09)
> *"di phase ini vendor hanya melihat dan menerima request di VendorPortal dan menerima notifikasi"*

Kondisi sekarang (dicek di kode, 2026-10-09):
- Vendor Request (2.2a, `ef6dbfe`): ATM-USER buat → ATM-SPV approve → `vault_assignment` → ACM-USER pilih branch vault
  per ATM → ACM-SPV approve per area → `vault_review` → ATM-SPV approve → **`ready`**. Laporan selesai dari `ready`
  (atau `approved` untuk request lama).
- Status yang ada (`vendor_requests_status_chk`, mig 027): `draft`, `pending_approval`, `approved`, `rejected`,
  `processing`, `completed`, `failed`, `cancelled`, `completion_pending`, `vault_assignment`, `vault_review`, `ready`.
- `/api/v1/vendor-requests` hanya untuk user internal; vendor tidak bisa melihat request apa pun.
- VendorPortal halaman **Orders** dan **Schedule** masih **mock** (`@/data/orders.json`, `schedules.json`).
- `users.vendor_id` (wajib untuk user vendor) + `users.vendor_branch_id` (opsional, mengikat ke satu cabang).
- `internal/notification` (0.3) sudah ada: in-app + email outbox, VendorPortal sudah punya API notifikasi.

## Proposed outcome (alur fase ini)
1. Saat Vendor Request menjadi **`ready`**, CROWN **otomatis** mengirimnya ke vendor (Q2) → status **`sent_to_vendor`**.
2. Penerima (Q3) = **branch replenish** (pengelola kelolaan ATM) **dan branch vault** (penyedia uang dari 2.2a). Satu
   request bisa tampil ke beberapa branch/vendor; branch vault vendor lain hanya melihat ATM yang uangnya ia sediakan.
3. Cakupan user vendor (Q3): user dengan `vendor_branch_id` → hanya branch itu; user tanpa branch → semua branch
   vendornya.
4. Tiap **(request × branch)** memberi satu keputusan (Q4, Q5): **Terima**, atau **Tolak + alasan wajib**.
5. Penolakan (Q6):
   - **Branch vault menolak** → ATM terkait kembali ke **ACM area** untuk pilih branch vault lain → ACM-SPV → ATM-SPV →
     `ready` → dikirim ulang **hanya ke branch yang berubah**.
   - **Branch replenish menolak** → request kembali ke **ATM-SPV / BRANCH-ATM-SPV** → batal, atau edit lalu ulang dari
     ATM-SPV approve. Kelolaan tidak diubah.
   - Selama ada penolakan, request tidak dianggap diterima penuh; penerimaan branch lain tetap tercatat.
6. Semua branch menerima → status **`vendor_accepted`** (Q10). Laporan selesai replenish **hanya dari
   `vendor_accepted`**. Request lama `approved`, dan request yang sudah `ready` sebelum rilis 2.2b, **melewati** langkah
   vendor (tetap boleh laporan selesai) — sama seperti F12 di 2.2a.
7. Tidak ada batas waktu terima/tolak (Q9); internal memantau status "belum diterima" di layar.
8. Batal / edit setelah terkirim (Q12):
   - **Batal** (ATM-SPV, dari `sent_to_vendor` / `vendor_accepted`): hilang dari daftar aktif vendor, tampil
     "Dibatalkan" (tidak dihapus); semua branch terkait dinotifikasi.
   - **Edit** hanya setelah branch replenish menolak; setelah dikirim ulang, semua branch (termasuk yang sudah menerima)
     **menerima ulang**.
   - Di luar itu request terkirim **tidak bisa diedit** → batal lalu buat baru.

### Data yang dilihat vendor (Q7)
- **Branch replenish**: nomor request, tanggal replenish, daftar ATM (terminal ID, lokasi, nomor tiket), nominal order per
  denom, **branch vault** tempat ambil uang (nama cabang, vendor, alamat vault).
- **Branch vault**: nomor request, tanggal replenish, ATM yang uangnya ia sediakan + nominal per denom, **total yang harus
  disiapkan per denom**, **branch replenish** yang mengambil.
- **Tidak ditampilkan**: saldo DSR / kapasitas vault, tier, alasan urgent, catatan internal ACM, harga paket.

### Notifikasi (Q8) — via `internal/notification` (in-app + email)
- **Vendor**, saat terkirim / dikirim ulang / dibatalkan: semua user vendor aktif dalam cakupan branch (user branch itu +
  user vendor tanpa branch).
- **Internal**: branch vault menolak → ACM-USER + ACM-SPV area; branch replenish menolak → ATM-SPV / BRANCH-ATM-SPV +
  pembuat request; semua branch menerima → pembuat request + ATM-SPV. Tidak ada notifikasi per penerimaan.

### VendorPortal (Q11)
- Halaman **Orders** diganti data nyata: daftar + detail request, tombol Terima / Tolak. **Schedule tetap mock**.

## Affected users and systems
- Users: vendor (branch replenish + branch vault, VendorPortal), ATM-SPV / BRANCH-ATM-SPV (tindak lanjut tolak replenish,
  batal), ATM-USER (edit + notifikasi), ACM-USER / ACM-SPV (revisi vault setelah tolak), ADMIN (read).
- Systems: `backend/` (status baru, keputusan vendor per request × branch, endpoint vendor-scoped, notifikasi, gate laporan
  selesai), `frontend/VendorPortal-Vite` (Orders), `frontend/CompanyPortal-Vite` (status vendor per branch di detail
  Vendor Request, tombol tindak lanjut).
- Skema (nama di spec): status `sent_to_vendor` + `vendor_accepted` di `vendor_requests`; tabel keputusan vendor per
  (request, branch, peran replenish/vault) dengan riwayat.

## Constraints
- Vendor **hanya** melihat data dalam cakupannya — RBAC di route **dan** service; ID request lain → 404.
- Keputusan vendor + semua transisi status menulis `audit_logs` dalam tx; tidak ada hard delete.
- Uang `numeric`, IDR eksplisit; total per denom dihitung dari data order, bukan input vendor.
- Kelolaan / branch replenish tidak diubah. Tidak ada pindah buku / escrow.
- Respon flat JSON seperti handler ATM `backend` lainnya.

## Resolved decisions (user, 2026-10-09)
1. Q1 — Fase ini: vendor **melihat + menerima** request di VendorPortal + notifikasi. Alur pickup (petugas, nopol,
   validasi Provider, serah terima) **tidak** termasuk.
2. Q2 — Kirim **otomatis saat `ready`**.
3. Q3 — Penerima **branch replenish + branch vault**; cakupan user per `vendor_branch_id` / per vendor.
4. Q4 — **Terima atau Tolak + alasan**.
5. Q5 — Keputusan **per (request × branch)**.
6. Q6 — Tolak vault → ACM; tolak replenish → ATM-SPV; tidak diterima penuh selama ada penolakan.
7. Q7 — Data vendor sesuai daftar di atas; data internal disembunyikan.
8. Q8 — Notifikasi sesuai daftar di atas.
9. Q9 — **Tanpa batas waktu**.
10. Q10 — Status `sent_to_vendor` → `vendor_accepted`; laporan selesai hanya dari `vendor_accepted`; request lama melewati.
11. Q11 — Ganti halaman **Orders**; Schedule tetap mock.
12. Q12 — Batal oleh ATM-SPV + notifikasi vendor; edit hanya setelah tolak replenish, semua branch terima ulang.

## Open questions (untuk spec, bukan intent)
- Status persis request setelah branch replenish menolak (kembali ke `pending_approval` atau status baru) dan siapa yang
  boleh edit (ATM-USER pembuat).
- Bentuk transisi "kembali ke ACM" untuk sebagian ATM: plan area yang terkena kembali `draft`, area lain tetap approved.
- Request yang sudah `ready` di dev saat rilis: dibiarkan (melewati) — dikonfirmasi lewat penanda di migrasi.

## Out of scope
- Alur cash pickup FSD (data petugas, nopol, validasi Provider, serah terima, close task) — fase berikutnya.
- Halaman Schedule VendorPortal (tetap mock).
- Batas waktu / SLA / penalti vendor.
- Pindah buku / escrow / SOF / BDS.
- Realisasi vs order, `holidays` (2.2c).
- Mengubah kelolaan / branch replenish ATM.
