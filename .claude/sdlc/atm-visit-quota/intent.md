# Intent: Kuota kunjungan replenish per ATM (sisa kunjungan dari paket)

Author: user (product owner), ditulis bersama Claude. Status: **accepted 2026-10-01** (user: "diterima, lanjut").
Stage: 1 Plan — selesai. Stage berikut: `spec.md`.
Menyentuh Sec 4 #7: **skema (tabel baru)** + **alur approval (state machine Vendor Request)** → wajib AI-DLC penuh.

## Problem
Kata user (2026-09-30): *"pada DB table paket kita perlu menambahkan informasi "jumlah_kunjungan". misal informasi paket
atm sama PAKET 5 maka jumlah kunjungan 5 jika PAKET 4 maka jumlah kunjungan 4. saat vendor selesai replenish, dan laporan
replenish sudah di approve oleh user SPV system harus bisa mengurangi dari nilai jumlah kunjungan. misal tercatat jumlah
kunjungan = 5, vendor selesai replenish, dan laporan replenish sudah di approve oleh user SPV, maka jumlah kunjungan pada
ATM itu adalah 4"*

Lanjutan: *"ok pakai cr_frequency, dan sepertinya perlu table baru untuk mencatat kunjungan per ATM"* ·
*"buat table baru jangan mengedit cr_frequency"*

Kondisi sekarang (dicek di kode):
- Kuota kunjungan kontrak **sudah ada**: `package_frequencies.cr_frequency` (migrasi 009), grain `(package_code, machine_group)`.
  PAKET 3/4/5/6 → 3/4/5/6 untuk ATM maupun CDM_CRM.
- Tidak ada pencatatan kunjungan replenish per ATM → sisa kuota tidak diketahui.
- Alur "laporan replenish selesai → approve SPV" **belum ada**: `vendor_requests` hanya punya approval order
  (pending_approval → approved); `approved → processing → completed` ada di CHECK constraint tapi tidak dipakai endpoint
  mana pun (`backend/internal/service/vendor_request.go:419`). VendorPortal orders/evidence masih mock.
- Jalur ATM → kuota: `atms` → `atm_vendor_packages` (aktif) → `vendor_packages_branch` (`package_code`, `machine_group`)
  → `package_frequencies`. `vendor_packages_branch.package_code` adalah **teks bebas** (hanya validasi "wajib diisi",
  `service/vendor_package_admin.go:187`), tanpa FK ke `package_frequencies` → kecocokan tidak dijamin.
  `vendor_packages_branch`/`atm_vendor_packages` **kosong di dev** sejak migrasi 011 (perlu seed ulang untuk uji).
- Modul notifikasi (tabel `notifications`, in-app/SMTP) **belum ada**.

## Proposed outcome
1. **Laporan selesai replenish** pada Vendor Request berstatus `approved`: ATM-USER menandai request selesai dan per ATM
   memilih **berhasil / gagal**, lalu mengajukan ke SPV.
2. **SPV approve** laporan → setiap ATM **berhasil** tercatat 1 kunjungan dan sisa kunjungan ATM itu berkurang 1.
   ATM gagal tidak mengurangi kuota. SPV **tolak** (dengan alasan) → request kembali ke `approved`, ATM-USER bisa mengajukan
   laporan ulang; kuota tidak berubah.
3. **Sisa kunjungan per ATM disimpan di tabel baru** (bukan di `cr_frequency`). Contoh: PAKET 5, 1 kunjungan di-approve → sisa 4.
4. **Kelebihan kuota**: sisa = 0 tidak memblokir. Kunjungan tetap tercatat dan ditandai **kelebihan kuota**; SPV melihat
   peringatan di layar saat approve, ATM-USER pembuat laporan melihat badge "Kelebihan kuota".
5. **Reset manual**: ATM-SPV / BRANCH-ATM-SPV / ADMIN me-reset sisa kunjungan ke `cr_frequency` paket aktif — per ATM
   (dari Profil ATM) atau massal semua ATM kelolaan satu vendor (dari detail vendor `/settings/admin/vendors/:id`).
6. **Koreksi**: ATM-SPV membatalkan satu kunjungan yang salah catat (soft-cancel + alasan wajib) → sisa naik 1.
7. **Tampilan sisa kunjungan**: Profil ATM, dan Forecast Browser / Buat Vendor Request (saat memilih ATM).

## Affected users and systems
- Users: ATM-USER (pembuat laporan selesai), ATM-SPV / BRANCH-ATM-SPV (approver, reset, koreksi), ADMIN (reset).
- Systems: `backend/` (migrasi baru, `queries/`, `service/vendor_request*`, service kuota baru, `handler/`),
  `frontend/CompanyPortal-Vite` (Vendor Request detail, Profil ATM, Forecast Browser/Create, detail vendor).
- Tabel: baru (kuota/kunjungan per ATM — nama & kolom ditetapkan di spec, diajukan ke CLAUDE.md Sec 3);
  dibaca `package_frequencies`, `atm_vendor_packages`, `vendor_packages_branch`, `atms`, `vendor_requests`,
  `vendor_request_items`; diubah `vendor_requests` (status/transisi baru).

## Constraints
- `package_frequencies.cr_frequency` **read-only** — tidak pernah diubah/dikurangi oleh fitur ini; tidak ada kolom `jumlah_kunjungan`.
- Maker (ATM-USER) ≠ checker (SPV) pada laporan selesai (Sec 5); RBAC di route **dan** service.
- Setiap perubahan (laporan, approve/tolak, reset, pembatalan kunjungan) menulis `audit_logs` dalam tx yang sama.
- Idempoten: satu laporan yang sama tidak boleh mengurangi kuota dua kali (mis. double-click approve).
- Kurangi sisa secara atomik di DB (tanpa race antar approve paralel pada ATM yang sama).
- Tulis ke primary; tampilan sisa di Profil ATM/Forecast Browser boleh dari replica (Sec 6).
- Tidak ada hard delete (Sec 12): kunjungan dibatalkan dengan soft-cancel.
- `package_code` ATM yang tidak cocok dengan `package_frequencies` → kuota **"tidak diketahui"**, jangan ditebak
  (pola sama dengan `price_machine_group` NULL = gagal keras, Sec 3).

## Resolved decisions (tanya-jawab satu per satu dengan user, 2026-09-30)
1. Kuota dari `package_frequencies.cr_frequency`; tidak ada kolom `jumlah_kunjungan`.
2. `cr_frequency` tidak diedit; kunjungan/sisa per ATM di **tabel baru**.
3. Laporan replenish = langkah "selesai" pada Vendor Request `approved` (opsi a), di-approve SPV.
4. Maker laporan selesai = **ATM-USER internal** (bukan vendor; VendorPortal tetap mock).
5. Isi laporan = **per ATM berhasil/gagal**; hanya ATM berhasil mengurangi kuota.
6. Laporan ditolak SPV → **kembali ke ATM-USER** (request kembali `approved`), bisa diajukan ulang; kuota tidak berubah.
7. Periode kuota **tanpa reset otomatis**; diperbarui manual = **reset ke `cr_frequency`** (bukan angka bebas).
8. Reset **per ATM + massal per vendor**.
9. Reset oleh **ATM-SPV / BRANCH-ATM-SPV / ADMIN**, langsung berlaku + audit (tanpa maker-checker).
10. Sisa = 0 → **tetap boleh**, ditandai kelebihan kuota.
11. Penerima peringatan: **SPV yang approve** + **ATM-USER pembuat laporan**.
12. Bentuk peringatan: **di layar dulu** (peringatan saat approve + badge); notifikasi in-app/email menyusul modul 0.3.
13. **Hanya CR**; FLM (`flm_frequency`) di luar scope.
14. **Semua jenis request** (reguler dan darurat/adhoc) mengurangi kuota.
15. Ganti paket → sisa **tetap**, paket baru berlaku setelah reset manual.
16. Tampilan sisa: **Profil ATM** + **Forecast Browser / Buat Vendor Request**.
17. Tombol reset: per ATM di **Profil ATM**, massal di **detail vendor**.
18. Koreksi: **SPV membatalkan kunjungan** (soft-cancel + alasan wajib), sisa naik 1, tercatat audit.

## Untuk diputuskan di spec (teknis, bukan keputusan bisnis)
- Nama & kolom tabel baru; perlu satu tabel (sisa per ATM) atau dua (sisa per ATM + log kunjungan per laporan) —
  koreksi (keputusan 18) dan penanda kelebihan kuota butuh riwayat per kunjungan.
- Nilai awal sisa untuk ATM yang belum punya baris (mis. = `cr_frequency` paket aktif saat kunjungan pertama).
- Nama status baru di `vendor_requests` (mis. `completion_pending`) vs memakai `processing`/`completed` yang sudah ada di CHECK constraint.
- Kolom per ATM hasil laporan (berhasil/gagal) — di `vendor_request_items` atau tabel laporan terpisah.

## Out of scope
- Laporan selesai dari VendorPortal (vendor) — VendorPortal backend belum ada.
- Kunjungan FLM / `flm_frequency`.
- Notifikasi in-app/email (menunggu modul notifikasi 0.3).
- Penalti/tagihan kelebihan kuota (modul invoice).
- Mengubah nilai `cr_frequency` atau layar admin `package_frequencies`; validasi/FK `package_code` ke `package_frequencies`.
- Halaman khusus daftar kuota kunjungan.
