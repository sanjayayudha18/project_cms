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

| Username | Peran | Cabang | Email |
|---|---|---|---|
| `vendor.aba` | VENDOR-USER (semua cabang vendor ABA) | semua | `vendor.aba@vendor.dev.local` |
| `vendor.abacus.mdn.atm` | VENDOR-ATM-USER (pihak replenish) | Medan (`ABACUS_004`) | `vendor.abacus.mdn.atm@vendor.dev.local` |
| `vendor.abacus.mdn.vault` | VENDOR-VAULT-USER (pihak vault) | Medan (`ABACUS_004`) | `vendor.abacus.mdn.vault@vendor.dev.local` |
| `vendor.abacus.bdg.atm` | VENDOR-ATM-USER (pihak replenish) | Bandung (`ABACUS_002`) | `vendor.abacus.bdg.atm@vendor.dev.local` |
| `vendor.abacus.bdg.vault` | VENDOR-VAULT-USER (pihak vault) | Bandung (`ABACUS_002`) | `vendor.abacus.bdg.vault@vendor.dev.local` |

Password untuk semuanya `password123`.

- Semua akun di atas dibuat oleh `backend/scripts/dev/seed_vendor_users.sql`. Script itu juga membuat satu akun `vendor.<kode vendor huruf kecil>` untuk setiap vendor FLM aktif.
- Alamat ini hanya untuk dev. Domain `vendor.dev.local` tidak bisa menerima email, jadi notifikasi SMTP ke akun ini tidak akan sampai ke inbox mana pun.

---

# Checklist verifikasi sebelum merge `dev1` → `main`

Daftar ini menggabungkan semua cek browser manual yang masih terbuka di spec (Golden Rule #10). Urutannya per alur bisnis, jadi satu sesi bisa menutup beberapa spec sekaligus. Centang `[x]` dan tulis temuan di bawah tiap bagian. Kalau ada temuan, catat dulu; perbaikannya dikerjakan terpisah sebagai bugfix.

Disusun 2026-11-04 dari: `.claude/sdlc/.done/{cit-vault-select,cit-send-vendor,replenishment-realisasi,perbaikan-rbac,vendor-upload-dsr}/`, `.claude/sdlc/dsr-late-report/`, `.kiro/specs/.done/{vendor-party-scope,sidebar-settings-footer}/`, `.kiro/specs/{vendor-data-export,report-export-migration,export-menu-revamp,export-preview-simplify,master-data-import-revamp}/`.

## 0. Persiapan (sekali di awal)

- [ ] DB dev hidup di `localhost:5432`. Dari `backend/`: `go run ./cmd/migrate status` menunjukkan `001`–`036` applied, tanpa MISMATCH.
- [ ] Redis hidup di `localhost:6379`.
- [ ] Dari host shell, `DATABASE_URL` memakai `localhost`, bukan `host.docker.internal`.
- [ ] Jalankan service:
  - `backend`: `go run ./cmd/api` (:8080)
  - `backend-exq`: `go run ./cmd/api` dari `backend-exq/` (:8081). Wajib untuk bagian 4 (Export).
  - Python EOD: `python -m eod_retry_scheduler.run` (:8091) dan `python -m service_dsr_etl.run` (:8090) dari `backend_python/`. Wajib untuk bagian 2 dan 3.
  - CompanyPortal: `pnpm --dir frontend/CompanyPortal-Vite run dev` (:5173)
  - VendorPortal: `pnpm --dir frontend/VendorPortal-Vite run dev` (:5174)
- [ ] Seed dev sudah dijalankan (cukup sekali, semuanya idempoten): `seed_acm_users.sql`, `seed_vendor_users.sql`, `dsr_late_report_seed.sql`. Lokasinya `backend/scripts/dev/`.
- [ ] Pengaturan → Area ACM: "Area ACM Dev" sudah mencakup region yang dipakai untuk tes.
- [ ] ADMIN sudah mengisi pemetaan lokasi DSR → vault (`dsr_location_vault_maps`). Tanpa pemetaan ini, laporan DSR telat dan notifikasinya kosong.

**Temuan:**

## 1. Alur order ATM (2.2a, 2.2b, 2.2d, 2.2c)

Pakai satu Vendor Request, mis. `REP-ABA-MDN-…`, dari awal sampai realisasi.

### 1a. Penetapan vault oleh ACM (2.2a, `cit-vault-select`)
- [ ] `atm.spv` approve Vendor Request → muncul langkah Penetapan Vault.
- [ ] `acm.user`: pilih cabang → vault, isi nominal per vault (CASH / ATM_CASH), isi kurang/lebih/buffer, Simpan, lalu Kirim.
- [ ] `acm.spv` approve penetapan vault. Maker tidak boleh sama dengan checker.
- [ ] `atm.spv` melihat hasil penetapan di detail Vendor Request. Total alokasi sama dengan nominal request.
- [ ] Setiap alokasi punya tiket fulfillment vault.

### 1b. Kirim ke vendor (2.2b, `cit-send-vendor`)
- [ ] Setelah approve, status request menjadi "Terkirim ke Vendor".
- [ ] VendorPortal `vendor.aba`: order tampil di Orders, detail bisa dibuka, ada Terima/Tolak.
- [ ] Tolak sebagai pihak vault → layar Penetapan Vault di CompanyPortal menampilkan "Ditolak vendor".
- [ ] Tolak sebagai pihak replenish → muncul aksi "Kembalikan ke Pembuat".
- [ ] Semua pihak terima → status "vendor_accepted", tombol Laporkan Selesai muncul.
- [ ] Notifikasi in-app (bell) muncul di kedua portal untuk setiap langkah.

### 1c. Pembagian akun vendor per peran dan cabang (2.2d, `vendor-party-scope`)
- [ ] `vendor.abacus.mdn.atm` hanya melihat baris pihak replenish Medan. Tidak ada baris vault, tidak ada cabang Bandung.
- [ ] `vendor.abacus.mdn.vault` hanya melihat baris pihak vault Medan.
- [ ] `vendor.abacus.bdg.*` tidak melihat order Medan sama sekali.
- [ ] `vendor.aba` (VENDOR-USER) melihat kedua pihak di semua cabang.
- [ ] Notifikasi hanya sampai ke akun yang sesuai pihak dan cabangnya.
- [ ] Ubah peran PIC vendor (langsung berlaku + tercatat di Log Audit).
- [ ] Ekspor lalu impor CSV PIC: kolom `roles` ikut.
- [ ] Buat akun vendor baru dari Manajemen Pengguna (login `Yudha`): pilih vendor, peran vendor, cakupan cabang. Login pertama meminta ganti password.

### 1d. Realisasi vs order (2.2c, `replenishment-realisasi`)
- [ ] Login `Yudha` (ADMIN), buka Kalender Libur (`/settings/admin/holidays`, lewat menu Forecasting atau kartu "Hari Libur" di Pengaturan): tambah satu hari libur, lalu nonaktifkan lagi. Perubahan tercatat di Log Audit.
- [ ] VendorPortal: Lapor Realisasi (tanggal + nominal), lalu cek layar Realisasi.
- [ ] CompanyPortal detail Vendor Request: approve dan reject laporan realisasi vendor.
- [ ] Uji ITM dev: proses `backend/scripts/dev/itm_replenish_test_sample.csv` (atau jalankan manual run). Klasifikasi muncul: sesuai jadwal / maju / mundur / tidak dilakukan / tanpa order. Hitungan hari memakai hari kerja (libur dilewati).
- [ ] Kalau ITM dan laporan vendor sama-sama ada, nilai ITM yang dipakai.
- [ ] Layar Replenishment: filter jalan, ekspor CSV jalan (sekarang lewat `backend-exq`, lihat bagian 4b).
- [ ] Diterima, bukan bug: order 10K/20K selalu tampil selisih karena ETL ITM hanya membaca 50K/100K (R3).

**Temuan:**

## 2. DSR (1.3, 2.3)

### 2a. Upload DSR vendor (1.3, `vendor-upload-dsr`)
- [ ] VendorPortal: upload file DSR → pratinjau dry-run tampil (jumlah baris, kesalahan kalau ada).
- [ ] Confirm → data tersimpan, riwayat upload bertambah.
- [ ] Upload file yang sama lagi → ditolak sebagai duplikat (checksum sama).
- [ ] Upload file yang isinya diubah untuk tanggal yang sama → diterima sebagai revisi.

### 2b. Laporan DSR telat (2.3, `dsr-late-report`)
Data uji: `dsr_late_report_seed.sql` (vendor Bijak, `report_date` 2026-07-15). Pastikan `DSR_LATE_START_DATE` (Go) dan `RETRY_DSR_LATE_START_DATE` (Python) berisi tanggal yang sama. Kalau kosong, laporan mati.
- [ ] Menu laporan tampil untuk ATM-USER, ATM-SPV, BRANCH-ATM-SPV, ADMIN. Tidak tampil untuk peran lain, dan akun vendor mendapat 403 di `/api/v1/dsr-late-reports`.
- [ ] Tab harian, rekap bulanan, dan peringatan bisa dibuka. Status tampil sebagai teks + ikon (OK / TELAT / TIDAK KIRIM / BELUM JATUH TEMPO), tidak hanya warna.
- [ ] Upload sebelum 09:00 WIB → OK. Tepat 09:00:00 → OK. 09:00:01 → TELAT.
- [ ] Revisi untuk tanggal yang sama → status tetap ditentukan upload pertama.
- [ ] Label lokasi DSR yang tidak ada di pemetaan → muncul di tab peringatan, cabang tidak dihitung TIDAK KIRIM.
- [ ] Hari ini sebelum 09:00 → BELUM JATUH TEMPO. Sesudah 09:00 tanpa upload → TIDAK KIRIM.
- [ ] Notifikasi 09:00 WIB (uji nyata): cabang yang belum kirim memberi notifikasi ke user vendor + PIC penerima notifikasi; ATM-USER mendapat satu ringkasan; email masuk outbox `notification_emails`.

**Temuan:**

## 3. Master data (1.2, X.10)

### 3a. Drill-down cabang vendor (1.2, `perbaikan-rbac`)
Buka `http://localhost:5173/settings/admin/vendors/2` sebagai `Yudha`. (Plan lama menulis port 5174, tapi halaman admin ada di CompanyPortal.)
- [ ] Klik cabang A lalu cabang B dengan cepat → data tidak tercampur.
- [ ] Vendor ROH (308 cabang): paging dan pencarian cabang menemukan cabang di halaman belakang.
- [ ] Create / edit / disable / enable untuk cabang, vault, PIC, paket → respons "menunggu persetujuan", data belum berubah.
- [ ] Checker approve → data berubah. Reject → data lama tetap, status ditolak tampil.
- [ ] Double-click Simpan atau ajukan dua kali → tidak ada dua pengajuan. Pengajuan kedua untuk record yang sama ditolak (409).
- [ ] Form vault menerima kategori ATM, CASH, ATM_CASH.
- [ ] PIC vendor-wide bisa dikelola tanpa memilih cabang dan tidak tercampur dengan PIC cabang.
- [ ] Disable cabang yang masih punya child aktif → ditolak dengan pesan jelas.

### 3b. Impor CSV data master (X.10, `master-data-import-revamp`)
- [ ] Jalankan checklist lengkap di `.kiro/specs/master-data-import-revamp/browser-checklist.md` (bagian 1–11). Checklist itu sudah rinci, jadi tidak disalin ke sini.
- [ ] Isi juga bagian 10 di checklist itu (enam keputusan tampilan yang perlu Anda nilai).

**Temuan:**

## 4. Export (X.7, X.8, X.9)

Sejak 2026-11-03 item sidebar "Export Data" dihapus. Buka `/exports` lewat kartu di Pengaturan (ADMIN / ADMIN_PARAM / APPACCESS) atau ketik URL langsung. `backend-exq` (:8081) harus hidup.

### 4a. Ekspor data master (X.7 `vendor-data-export`, X.9 `export-menu-revamp`)
- [ ] Katalog `/exports` per peran: ADMIN dan ADMIN_PARAM melihat data yang sesuai; ATM-USER tidak bisa membuka katalog master data.
- [ ] Pratinjau tiap data master (`/exports/<data>`): tabel, filter, jumlah baris, paging.
- [ ] Filter lalu Search → URL ikut berubah; URL itu dibuka di tab lain menampilkan filter yang sama.
- [ ] Tombol Ekspor nonaktif sebelum filter diterapkan.
- [ ] Ekspor level vendor, cabang, Harga Paket, dan master data global.
- [ ] Pop-up loading muncul, lalu file terunduh otomatis (Chrome/Edge; izinkan multi-download kalau ditanya).
- [ ] Isi file sama dengan isi tabel pratinjau.
- [ ] Tutup pop-up sebelum selesai → file tetap bisa diunduh manual dari Ekspor Saya.
- [ ] Tunggu lebih dari 10 menit tanpa mengunduh → status Kedaluwarsa.
- [ ] Unduh kedua untuk file yang sama → ditolak (file dihapus setelah unduhan pertama).
- [ ] `/settings/admin/exports` mengalihkan ke `/exports`.
- [ ] Tautan dari halaman Impor CSV Data Master ke ekspor masih jalan.
- [ ] Log Audit: filter `export_requested` dan `master_data_exported` menampilkan user, IP, waktu.

### 4b. Ekspor laporan (X.8 `report-export-migration`)
- [ ] `atm.spv`: ekspor DSR telat harian (dengan filter status) dan rekap bulanan.
- [ ] `branch.atm.user`: ekspor selisih nominal dan trip berhasil; ekspor DSR telat tidak tersedia.
- [ ] Pop-up + unduh otomatis jalan; file muncul di Ekspor Saya (`/exports`).
- [ ] File dibuka di Excel: karakter non-ASCII (nama dengan huruf khusus) tampil benar.
- [ ] Log Audit: filter `report_exported` menampilkan baris lama dan baru.

### 4c. Pratinjau yang disederhanakan (X.9 follow-up `export-preview-simplify`)
- [ ] `/exports/vendor-pics`: hanya dropdown Vendor (semua vendor aktif tampil, tanpa kotak "Cari vendor"); Cabang muncul setelah vendor dipilih.
- [ ] Tombol bernama "Search"; tombol "Ekspor" tanpa angka jumlah baris; teks "Total N baris" tetap ada; catatan pengambilan file sudah hilang.
- [ ] `/exports/package-prices` dan `/exports/dsr-location-maps`: vendor wajib dipilih, Search nonaktif sebelum itu.
- [ ] Ekspor satu data menghasilkan CSV yang sama seperti sebelumnya.

### 4d. Aksesibilitas export
- [ ] Semua langkah di atas bisa dilakukan dengan keyboard saja (Tab, Enter, Esc).
- [ ] Screen reader (NVDA/Narator) membaca tahap dialog ekspor, dropdown Vendor, dan tombol Search.

**Temuan:**

## 5. App shell CompanyPortal (X.1, `app-shell-revamp` + `sidebar-settings-footer`)

- [ ] Sidebar mode Expanded dan Rail (tombol Ciutkan/Lebarkan) jalan; animasi lebar mati kalau OS memakai "reduce motion".
- [ ] `Yudha` (ADMIN): link Pengaturan ada di paling bawah Sidebar, tidak ada di Topbar.
- [ ] `atm.user` (ATM-USER): link Pengaturan tidak muncul.
- [ ] Mode Rail: tooltip "Pengaturan" muncul saat hover dan saat fokus keyboard, tidak terpotong tepi Sidebar.
- [ ] Tidak ada lagi teks "Lingkungan" atau nomor versi di Sidebar.
- [ ] Ctrl+K membuka command palette; memilih "Pengaturan" membuka `/settings`.
- [ ] Toggle tema Sidebar (gelap/terang) jalan; konten dan Topbar tetap terang.
- [ ] Font Plus Jakarta Sans termuat tanpa request ke domain luar (cek tab Network).

**Temuan:**

## 6. Build Docker yang belum pernah dijalankan

Dari root repo:
- [ ] `docker compose -f backend/docker-compose.yaml up -d --build` → container `backend` healthy (`/health` 200).
- [ ] `docker compose -f backend-exq/docker-compose.yaml up -d --build` → container `backend-exq` hidup di :8081.
- [ ] Build image `backend_python` (Dockerfile di `backend_python/`) → kedua service FastAPI healthy.
- [ ] `docker compose -f frontend/docker-compose.yml up -d --build` → kedua portal tersaji lewat Nginx; CompanyPortal bisa memanggil `backend-exq` (bagian 4 diulang singkat lewat Nginx).

**Temuan:**

## 7. Merge `dev1` → `main` (X.6)

Lakukan setelah bagian 0–6 selesai dan temuan yang menghalangi sudah diperbaiki.
- [ ] Centang task cek manual di spec masing-masing (atau minta Kiro mencentangnya berdasarkan file ini):
  - `.kiro/specs/vendor-data-export/tasks.md` task 18
  - `.kiro/specs/report-export-migration/tasks.md` task 11
  - `.kiro/specs/export-menu-revamp/tasks.md` task 14
  - `.kiro/specs/export-preview-simplify/tasks.md` task 8
  - `.kiro/specs/master-data-import-revamp/tasks.md` task 16
  - `.kiro/specs/.done/sidebar-settings-footer/tasks.md` task 4
  - `tests.md` / `review.md` di `.claude/sdlc/.done/{cit-vault-select,cit-send-vendor,replenishment-realisasi,vendor-upload-dsr}/` dan `.claude/sdlc/dsr-late-report/`
- [ ] `git fetch` lalu pastikan `dev1` tidak tertinggal dari `origin/main`.
- [ ] Buka Pull Request `dev1` → `main` di GitHub. GitHub Actions (`.github/workflows/ci.yml`) harus hijau. Catatan: CI belum punya job frontend, jadi jalankan `pnpm run lint`, `test:coverage`, dan `build` di kedua portal secara lokal sebelum merge.
- [ ] Review oleh code owner, lalu merge. Jangan force push.
- [ ] Perbarui `development-plan.md` (X.6 dan status snapshot) setelah merge.

**Hasil akhir:** _(diisi)_ lulus / ada temuan: …
