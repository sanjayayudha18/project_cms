# Intent: Pisah akun vendor per peran (vault vs pengisian ATM) + cakupan cabang di VendorPortal Orders

Author: user (product owner), ditulis bersama Claude.
Status: **draft — 2026-10-20**, menunggu PO accept.
Stage: 1 Plan. Stage berikut: `spec.md`.
Menyentuh Sec 4 #7: **auth/RBAC (scoping user vendor per peran + cabang)**, kemungkinan **skema** (penanda peran di
akun) → wajib AI-DLC penuh.

## Problem — kata user (2026-10-20)
Saat cek manual `REP-ABA-MDN-20261014-001` di VendorPortal (login `vendor.aba`), tiket tampil dua baris: *Vault
(penyedia uang)* dan *Replenish (pelaksana)*, dua-duanya Abacus Medan.
> *"Vendor vault dengan Vendor pengisian ATM dipisah (dalam arti 2 user yg berbeda). Maka tiket nya tidak akan
> tercampur antara vendor penyedia uang dan vendor pengisian uang."*
>
> *"cabang region medan tidak perlu bisa lihat request ke cabang Jakarta."*

Kondisi sekarang (dicek di kode/DB dev, 2026-10-20):
- Cakupan user vendor (2.2b, `backend/queries/vendor_parties.sql:138-184`): `users.vendor_id` (claim JWT) +
  `users.vendor_branch_id` dibaca dari DB per call; `NULL` = semua cabang vendornya. **Tidak ada pembeda peran** —
  user melihat party `replenish` **dan** `vault` di cabang dalam cakupannya.
- Semua user vendor memakai role `VENDOR-USER`.
- Dev: `vendor.aba` (seed `backend/scripts/dev/seed_vendor_users.sql`) vendor-wide (`vendor_branch_id` NULL) → melihat
  semua cabang ABA. Pembatasan per cabang **sudah didukung kode**, hanya belum dipakai di data.
- Notifikasi vendor (2.2b) dikirim ke user dalam cakupan branch (user branch itu + user vendor-wide) — juga tanpa
  pembeda peran.

## Proposed outcome
1. Akun vendor punya **peran**: penyedia uang (vault) **atau** pengisian ATM (replenish). Akun vault hanya melihat dan
   memutuskan tiket peran vault; akun replenish hanya tiket peran replenish. Tiket tidak tercampur.
2. Akun vendor cabang Medan **tidak** melihat request untuk cabang Jakarta (dan sebaliknya).
3. Notifikasi vendor mengikuti cakupan yang sama (peran + cabang).

## Affected users and systems
- Users: vendor (VendorPortal Orders), ADMIN/APPACCESS (membuat/mengatur akun vendor di `admin/users`).
- Systems: `backend/` (scope query party, notifikasi vendor, RBAC route + service, admin users), `frontend/VendorPortal-Vite`
  (Orders), `frontend/CompanyPortal-Vite` (form user vendor), dev seed akun vendor.

## Constraints
- RBAC di route **dan** service; tiket di luar cakupan → 404 (sama seperti 2.2b).
- Perubahan akun lewat `admin/users` (apply langsung, bukan maker-checker — Sec 12) + `audit_logs`.
- Tidak mengubah alur keputusan party (terima/tolak), status Vendor Request, atau isi `content` snapshot.
- Flat JSON seperti handler ATM `backend` lain; endpoint di `openapi.yaml` ikut diperbarui.

## Open questions (belum dijawab — untuk PO)
1. **Penanda peran**: role baru (mis. `VENDOR-VAULT` / `VENDOR-REPLENISH`) atau kolom baru di `users`
   (mis. `vendor_party_role` = replenish | vault)?
2. Boleh ada akun yang memegang **dua peran** sekaligus (mis. vendor kecil, satu orang), atau wajib tepat satu?
3. Akun vendor yang sudah ada (`VENDOR-USER`, tanpa peran): dianggap dua peran, salah satu, atau diblokir sampai diatur?
4. **Cakupan Medan vs Jakarta**: cukup per **cabang vendor** (`vendor_branch_id`, sudah ada), atau per **region**
   (satu akun melihat semua cabang vendor di region `MDN`)? Akun vendor-wide (tanpa cabang) masih diizinkan?
5. Akun vendor-wide di seed dev (`vendor.<code>`): dipecah per cabang × peran, atau dibiarkan?
6. Notifikasi vendor: hanya ke akun dengan peran + cabang yang cocok? PIC vendor (`vendor_pics`) ikut dipisah per peran?
7. Laporan realisasi (2.2c, oleh party replenish): hanya akun peran replenish — konfirmasi.
8. Siapa yang mengatur peran/cabang akun vendor: ADMIN/APPACCESS di CompanyPortal saja?

## Stages 2–6
- `spec.md`, `plan.md`, `tests.md`, `review.md`, `incidents.md` — stub, dibuat setelah intent diterima.
