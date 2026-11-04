# User Company portal

Ini user internal Company Portal dari seed `002_baseline_seed.sql` dan `scripts/dev/seed_acm_users.sql`. Password semuanya `password123`.

| Username | Peran | Nama | Email |
|---|---|---|---|
| `Yudha` | ADMIN | System Administrator | `admin@cimbniaga.co.id` |
| `Rangga` | ADMIN_PARAM | Parameter Admin | `admin.param@cimbniaga.co.id` |
| `atm.user` | ATM-USER | Budi Santoso | `budi.santoso@cimbniaga.co.id` |
| `atm.spv` | ATM-SPV | Dewi Lestari | `dewi.lestari@cimbniaga.co.id` |
| `branch.user` | BRANCH-USER | Andi Prasetyo | `andi.prasetyo@cimbniaga.co.id` |
| `branch.spv` | BRANCH-SPV | Siti Rahayu | `siti.rahayu@cimbniaga.co.id` |
| `branch.atm.user` | BRANCH-ATM-USER | Rizky Firmansyah | `rizky.firmansyah@cimbniaga.co.id` |
| `branch.atm.spv` | BRANCH-ATM-SPV | Maya Putri | `maya.putri@cimbniaga.co.id` |
| `acm.user` | ACM-USER | ACM User Dev | `acm.user@cimbniaga.co.id` |
| `acm.spv` | ACM-SPV | ACM Supervisor Dev | `acm.spv@cimbniaga.co.id` |

Beberapa hal yang perlu diketahui:
- `acm.user` dan `acm.spv` hanya ada kalau `seed_acm_users.sql` sudah dijalankan. Menurut catatan progres, seed ini sudah dijalankan di dev pada 2026-10-13.
- Seed tidak membuat user untuk peran APPACCESS. Kalau perlu, buat user baru lewat Manajemen Pengguna.
- Daftar ini saya ambil dari file seed, bukan dari database. User yang dibuat lewat UI setelah seed, atau email yang sudah diubah, tidak ada di tabel ini. Untuk melihat data sebenarnya, jalankan `SELECT username, email FROM users WHERE vendor_id IS NULL;` di DB dev.

# User Vendor portal
Alamat email dari seed yang sama, semuanya berakhiran `@vendor.dev.local`:

| Username | Email |
|---|---|
| `vendor.aba` | `vendor.aba@vendor.dev.local` |
| `vendor.abacus.mdn.atm` | `vendor.abacus.mdn.atm@vendor.dev.local` |
| `vendor.abacus.mdn.vault` | `vendor.abacus.mdn.vault@vendor.dev.local` |

Password untuk semuanya `password123`.

Alamat ini hanya untuk dev. Domain `vendor.dev.local` tidak bisa menerima email, jadi notifikasi SMTP ke akun ini tidak akan sampai ke inbox mana pun.