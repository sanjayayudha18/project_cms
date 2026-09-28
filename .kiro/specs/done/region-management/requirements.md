# Requirements Document

## Introduction

Fitur **Region Management** (Manajemen Region) menambahkan pengelolaan data master _region_ pada aplikasi internal CompanyPortal-Vite, disajikan sebagai kartu baru **"Manajemen Region"** di tab **"Data master"** pada halaman Pengaturan (`/settings`). Region adalah entitas master-data yang sudah ada di basis data (tabel `regions`, 13 baris ter-seed) dan menjadi acuan bagi `locations` melalui FK `locations.region_id`, yang kemudian mengalir ke pemfilteran ATM portal, pilihan lokasi pada Manajemen ATM, serta konteks region cabang vendor.

Saat ini region hanya bisa diubah lewat akses basis data langsung. Fitur ini memberi tim admin master-data (peran `ADMIN`/`ADMIN_PARAM`) antarmuka CRUD terkontrol: melihat daftar region, membuat region baru, mengubah nama tampilan, serta menonaktifkan/mengaktifkan region — semuanya dengan penjagaan RBAC, pencatatan audit, dan integritas referensial terhadap lokasi dependen. Fitur mengikuti pola admin-management yang sudah ada di proyek (`admin-atm-management`, `admin-user-vendor-management`): endpoint `/api/v1/admin/...` pada backend ATM (port 8080, bentuk JSON datar untuk kompatibilitas wire) dan pola halaman daftar + form buat/ubah + toggle soft-delete di frontend.

### Ruang lingkup

Termasuk: daftar, buat, ubah nama, nonaktif/aktif region; RBAC; audit; integritas referensial; validasi; kartu + rute frontend.

Tidak termasuk: perubahan `locations`/ATM/vendor; pemindahan lokasi antar-region secara massal; impor/ekspor CSV region (mengikuti "Ekspor & Impor CSV" yang sudah ada bila diperlukan, di luar spec ini); maker-checker penuh (lihat Asumsi A5).

### Perubahan Skema & Asumsi (WAJIB dikonfirmasi sebelum fase Design)

Berikut hal yang **belum** ada di kode/skema dan diusulkan sebagai asumsi. Mohon konfirmasi tiap poin sebelum lanjut ke Design.

- **A1 — Kolom soft-delete belum ada (perubahan skema).** Tabel `regions` saat ini hanya berkolom `id, code, region, created_at, updated_at` (dikonfirmasi di `backend/migrations/001_baseline_schema.sql`). Tidak ada `is_active` maupun `deleted_at`. Fitur nonaktif/aktif (Requirement 4) **memerlukan migrasi aditif** yang menambahkan `is_active boolean NOT NULL DEFAULT true` dan `deleted_at timestamptz NULL`. Nomor migrasi bebas berikutnya adalah **`003_`** (baseline di-squash 2026-09-18). Perubahan tabel `regions` di peta Sec 2 `project-context.md` harus dicatat setelah diterapkan.
- **A2 — `code` immutable setelah dibuat.** Diusulkan `regions.code` tidak dapat diubah setelah create (mengikuti pola `atms.terminal_id`), karena `code` berperan sebagai kunci stabil untuk seed/referensi. Perubahan nama hanya pada kolom `region` (nama tampilan). Konfirmasi.
- **A3 — Keunikan `code`.** Skema saat ini tidak memiliki UNIQUE constraint pada `regions.code`. Diusulkan menambahkan `UNIQUE (code)` (dinormalisasi case-insensitive, mis. disimpan uppercase) pada migrasi `003_`. Konfirmasi apakah keunikan ditegakkan di DB (disarankan) atau hanya di service layer.
- **A4 — Nama tampilan (`region`) wajib diisi.** Kolom `region` saat ini nullable. Fitur ini memperlakukan nama tampilan sebagai wajib pada create/update (validasi service layer), tanpa mengubah nullability kolom (menghindari benturan dengan baris lama). Konfirmasi.
- **A5 — Immediate-apply + audit, bukan maker-checker penuh.** Mengikuti `admin-atm-management`/`admin-user-vendor-management`, perubahan region diusulkan **langsung berlaku** disertai penulisan `audit_logs` wajib dalam transaksi yang sama (kegagalan tulis audit me-rollback perubahan), bukan lewat `approval_requests`. Konfirmasi apakah region cukup dengan kontrol ini atau butuh maker-checker penuh.
- **A6 — Aturan nonaktif region yang punya lokasi dependen.** Diusulkan: region dengan lokasi **aktif** yang masih mereferensikannya **tidak boleh** dinonaktifkan (diblokir dengan pesan jelas). Alternatif (perlu konfirmasi): diizinkan dengan peringatan. Hard-delete region **tidak pernah** diizinkan (integritas referensial + konvensi soft-delete proyek).

## Glossary

- **Region**: Entitas master-data yang mengelompokkan lokasi/cabang secara geografis-operasional. Tersimpan di tabel `regions` dengan `id`, `code`, `region` (nama tampilan), `created_at`, `updated_at`.
- **Code**: Kunci pendek stabil sebuah region (mis. `JKT_CENTRAL`). Berfungsi sebagai pengenal mesin; diusulkan unik dan immutable setelah create (Asumsi A2, A3).
- **Nama tampilan (display name)**: Nilai kolom `region` (mis. "JKT-Central") yang ditampilkan ke pengguna.
- **Ketergantungan lokasi (location dependency)**: Relasi `locations.region_id` → `regions.id`. Sebuah region "punya lokasi dependen" bila ada baris `locations` yang mereferensikan `region.id`.
- **Soft-delete**: Menonaktifkan baris via `is_active=false` + `deleted_at` terisi, tanpa menghapus fisik (`DELETE`). Data tetap ada untuk audit dan integritas referensial.
- **Immediate-apply-with-audit**: Kontrol perubahan di mana mutasi langsung berlaku tetapi wajib disertai penulisan `audit_logs` dalam transaksi yang sama; kegagalan audit membatalkan mutasi. Substitusi maker-checker untuk master-data admin di proyek ini.
- **Maker-checker**: Kontrol dua-orang (pengaju ≠ penyetuju) via `approval_requests`; efek berlaku hanya setelah disetujui. Tidak diusulkan untuk region (Asumsi A5).
- **RegionService**: Komponen backend (service layer `internal/region`) yang menerapkan aturan bisnis region dan RBAC di luar middleware.
- **RegionAPI**: Lapisan handler backend yang memasang rute `/api/v1/admin/regions` dengan bentuk JSON datar.
- **RegionManagementUI**: Fitur frontend CompanyPortal-Vite (halaman daftar + form + toggle) di `/settings/admin/regions`.
- **Primary/Replica**: Pool basis data tulis (primary) dan pool baca laporan (replica), sesuai aturan topologi DB proyek.

## Requirements

### Requirement 1: Melihat daftar region

**User Story:** Sebagai admin master-data, saya ingin melihat daftar region dalam tabel yang dapat dicari dan dipaginasi, agar saya dapat menelusuri dan mengelola data region dengan cepat.

#### Acceptance Criteria

1. WHEN admin membuka halaman `/settings/admin/regions`, THE RegionManagementUI SHALL menampilkan tabel region berisi kolom `code`, nama tampilan, dan status aktif dalam waktu paling lama 3 detik pada persentil ke-95.
2. THE RegionAPI SHALL menyediakan endpoint daftar region yang mendukung paginasi dengan parameter ukuran halaman (nilai antara 1 sampai 100, default 20) dan nomor halaman (bilangan bulat mulai dari 1, default 1).
3. IF parameter ukuran halaman atau nomor halaman berada di luar rentang yang diizinkan atau bukan bilangan bulat positif, THEN THE RegionAPI SHALL menolak permintaan dan mengembalikan pesan kesalahan yang menjelaskan parameter yang tidak valid tanpa mengembalikan data region.
4. WHEN admin memasukkan kata kunci pencarian sepanjang 1 sampai 100 karakter, THE RegionAPI SHALL mengembalikan hanya region yang `code` atau nama tampilannya mengandung kata kunci tersebut tanpa membedakan huruf besar dan kecil.
5. WHEN admin memasukkan kata kunci pencarian yang tidak cocok dengan region mana pun, THE RegionAPI SHALL mengembalikan daftar kosong beserta total jumlah hasil bernilai 0.
6. THE RegionAPI SHALL menyertakan jumlah lokasi dependen per region pada respons daftar sebagai bilangan bulat bernilai 0 atau lebih.
7. THE RegionAPI SHALL melayani permintaan baca daftar region dari pool replica.
8. WHEN daftar region dikembalikan, THE RegionAPI SHALL menyertakan penanda status aktif bertipe boolean untuk setiap region sehingga UI dapat menampilkannya dengan label teks dan ikon, bukan warna saja.

### Requirement 2: Membuat region baru

**User Story:** Sebagai admin master-data, saya ingin membuat region baru dengan kode dan nama tampilan, agar lokasi baru dapat dikelompokkan ke region tersebut.

#### Acceptance Criteria

1. WHEN admin mengirim permintaan pembuatan region dengan `code` sepanjang 1 sampai 20 karakter dan nama tampilan sepanjang 1 sampai 100 karakter yang valid, THE RegionService SHALL menyimpan region baru pada pool primary dengan status aktif dan mengembalikan identitas region yang dibuat.
2. IF `code` ternormalisasi yang dikirim sudah dipakai region lain yang belum dihapus, THEN THE RegionService SHALL menolak permintaan, tidak menyimpan region, dan mengembalikan galat konflik yang menjelaskan bahwa kode duplikat.
3. IF nama tampilan kosong, hanya berisi spasi, atau melebihi 100 karakter, THEN THE RegionService SHALL menolak permintaan, tidak menyimpan region, dan mengembalikan galat validasi yang menjelaskan batasan nama tampilan.
4. IF `code` kosong, hanya berisi spasi, melebihi 20 karakter, atau mengandung karakter selain huruf dan angka, THEN THE RegionService SHALL menolak permintaan, tidak menyimpan region, dan mengembalikan galat validasi yang menjelaskan batasan kode.
5. WHEN region baru berhasil dibuat, THE RegionService SHALL menulis entri `audit_logs` yang mencatat aktor, aksi, nilai sesudah, waktu, dan alamat IP dalam transaksi yang sama dengan penyimpanan region.
6. IF penulisan entri `audit_logs` gagal, THEN THE RegionService SHALL membatalkan transaksi sehingga region tidak tersimpan dan mengembalikan galat kegagalan penyimpanan.
7. THE RegionService SHALL menyimpan `code` dalam bentuk ternormalisasi (huruf besar, tanpa spasi tepi) sebelum menegakkan keunikan.

### Requirement 3: Mengubah nama tampilan region

**User Story:** Sebagai admin master-data, saya ingin mengubah nama tampilan sebuah region, agar penamaan tetap konsisten dan mudah dibaca tanpa mengubah kunci referensinya.

#### Acceptance Criteria

1. WHEN admin mengirim permintaan perubahan nama tampilan (1 sampai 100 karakter setelah pemangkasan spasi tepi) untuk region yang ada, THE RegionService SHALL memperbarui kolom nama tampilan pada pool primary dan menetapkan `updated_at` ke timestamptz UTC saat ini, tanpa mengubah `code`.
2. IF permintaan perubahan menyertakan `code` yang berbeda dari `code` region tersimpan, THEN THE RegionService SHALL menolak permintaan tanpa mengubah data tersimpan dan mengembalikan galat validasi bahwa `code` bersifat immutable.
3. IF nama tampilan baru kosong, hanya berisi spasi, atau melebihi 100 karakter, THEN THE RegionService SHALL menolak permintaan tanpa mengubah data tersimpan dan mengembalikan galat validasi yang menjelaskan batasan nama tampilan.
4. IF region dengan pengenal yang diminta tidak ditemukan, THEN THE RegionService SHALL menolak permintaan tanpa mengubah data apa pun dan mengembalikan galat "region tidak ditemukan".
5. WHEN perubahan nama tampilan berhasil, THE RegionService SHALL menulis tepat satu entri `audit_logs` berisi identitas aktor, aksi, nilai nama tampilan sebelum dan sesudah, waktu (timestamptz UTC), dan alamat IP dalam transaksi database yang sama dengan pembaruan.
6. IF penulisan `audit_logs` gagal, THEN THE RegionService SHALL membatalkan (rollback) pembaruan nama tampilan dan mengembalikan galat kegagalan, sehingga tidak ada perubahan nama tanpa jejak audit.

### Requirement 4: Menonaktifkan dan mengaktifkan region (soft-delete)

**User Story:** Sebagai admin master-data, saya ingin menonaktifkan region yang tidak lagi dipakai dan mengaktifkannya kembali bila perlu, agar daftar region tetap relevan tanpa kehilangan riwayat data.

#### Acceptance Criteria

1. WHEN admin menonaktifkan sebuah region yang berstatus aktif (`is_active=true`), THE RegionService SHALL menandai region tersebut `is_active=false` dan mengisi `deleted_at` dengan timestamptz UTC pada pool primary, tanpa menghapus baris secara fisik.
2. IF region yang akan dinonaktifkan masih direferensikan oleh satu atau lebih lokasi aktif (`is_active=true`), THEN THE RegionService SHALL menolak permintaan penonaktifan, mempertahankan region pada status aktif tanpa perubahan (`is_active=true`, `deleted_at` tetap kosong), dan mengembalikan galat yang menyebutkan jumlah lokasi dependen serta menandakan kegagalan penonaktifan.
3. WHEN admin mengaktifkan kembali region yang berstatus nonaktif (`is_active=false`), THE RegionService SHALL menandai region tersebut `is_active=true` dan mengosongkan `deleted_at` (menjadi NULL) pada pool primary.
4. THE RegionService SHALL menolak setiap permintaan penghapusan fisik region dan hanya mendukung penonaktifan (soft-delete).
5. WHEN status aktif region berubah dari aktif menjadi nonaktif atau dari nonaktif menjadi aktif, THE RegionService SHALL menulis satu entri `audit_logs` berisi identitas aktor, jenis aksi, nilai `is_active` dan `deleted_at` sebelum dan sesudah perubahan, timestamptz UTC, dan alamat IP aktor dalam transaksi database yang sama dengan perubahan status.
6. IF penulisan entri `audit_logs` pada transaksi perubahan status gagal, THEN THE RegionService SHALL membatalkan (rollback) seluruh perubahan status region dan mengembalikan galat yang menandakan perubahan tidak tersimpan, sehingga tidak ada perubahan status tanpa jejak audit.
7. IF admin menonaktifkan region yang sudah berstatus nonaktif (`is_active=false`) atau mengaktifkan region yang sudah berstatus aktif (`is_active=true`), THEN THE RegionService SHALL menolak permintaan tanpa mengubah status region maupun menulis entri `audit_logs`, dan mengembalikan galat konflik yang menyatakan status region tidak berubah.

### Requirement 5: Penjagaan RBAC

**User Story:** Sebagai pemilik sistem, saya ingin hanya peran admin master-data yang dapat mengakses pengelolaan region, agar data master terlindungi dari perubahan tak berwenang.

#### Acceptance Criteria

1. WHERE permintaan menuju rute `/api/v1/admin/regions` membawa token dengan peran selain `ADMIN` atau `ADMIN_PARAM`, THE RegionAPI SHALL menolak permintaan tersebut di lapisan middleware dengan galat otorisasi yang menunjukkan akses ditolak dan TIDAK menjalankan handler rute.
2. WHILE permintaan pengubahan data region diproses di lapisan service, THE RegionService SHALL memverifikasi ulang bahwa peran pemanggil adalah `ADMIN` atau `ADMIN_PARAM` sebelum mengeksekusi operasi tulis, dan IF peran tidak memenuhi, THEN THE RegionService SHALL membatalkan operasi tanpa mengubah data apa pun (rollback penuh) serta mengembalikan galat otorisasi.
3. WHERE pengguna yang masuk tidak memiliki peran `ADMIN` atau `ADMIN_PARAM`, THE RegionManagementUI SHALL menyembunyikan kartu "Manajemen Region" dan mengalihkan setiap upaya membuka rute `/settings/admin/regions` ke tampilan tidak-berwenang tanpa merender komponen pengelolaan region.
4. IF permintaan menuju rute region dikirim tanpa token autentikasi atau dengan token yang tidak valid atau kedaluwarsa, THEN THE RegionAPI SHALL menolak permintaan di lapisan middleware dengan galat tidak-terautentikasi, TIDAK menjalankan handler rute, dan TIDAK mengubah data region.

### Requirement 6: Pencatatan audit

**User Story:** Sebagai auditor, saya ingin setiap perubahan region tercatat, agar setiap tindakan dapat ditelusuri.

#### Acceptance Criteria

1. WHEN region dibuat, diubah, dinonaktifkan, atau diaktifkan, THE RegionService SHALL menulis tepat satu entri `audit_logs` untuk aksi tersebut dalam transaksi database yang sama dengan mutasi region.
2. THE RegionService SHALL mencatat pada setiap entri audit: identitas aktor (ID pengguna), jenis aksi (create, update, deactivate, atau activate), nilai sebelum, nilai sesudah, waktu kejadian dalam timestamptz UTC, dan alamat IP asal permintaan.
3. WHEN aksi adalah pembuatan region, THE RegionService SHALL menuliskan nilai sesudah dan mengosongkan nilai sebelum pada entri audit.
4. WHEN aksi adalah penonaktifan region, THE RegionService SHALL menuliskan nilai sebelum dan sesudah status pada entri audit.
5. IF penulisan `audit_logs` gagal, THEN THE RegionService SHALL membatalkan (rollback) seluruh mutasi region dalam transaksi yang sama, mempertahankan keadaan region sebelum aksi, dan mengembalikan indikasi kesalahan yang menyatakan bahwa aksi gagal karena kegagalan pencatatan audit, sehingga tidak ada perubahan region tanpa jejak audit.

### Requirement 7: Integritas referensial

**User Story:** Sebagai admin master-data, saya ingin sistem mencegah perubahan yang merusak keterkaitan region dengan lokasi, agar data lokasi tetap konsisten.

#### Acceptance Criteria

1. IF diterima permintaan penghapusan fisik (hard delete) region, THEN THE RegionService SHALL menolak permintaan tersebut, mempertahankan baris region tanpa perubahan, dan mengembalikan galat yang menandakan bahwa penghapusan fisik tidak diizinkan.
2. IF sebuah region masih memiliki minimal 1 lokasi aktif (`locations.region_id` merujuk region tersebut dan `is_active = true`) yang mereferensikannya, THEN THE RegionService SHALL mencegah penonaktifan region tersebut, mempertahankan status region tanpa perubahan, dan mengembalikan galat yang menyertakan jumlah lokasi aktif dependen.
3. WHEN region dinonaktifkan sedangkan tidak ada lokasi aktif yang mereferensikannya, THE RegionService SHALL mempertahankan seluruh nilai FK `locations.region_id` yang ada tanpa mengubah, menambah, atau menghapus baris `locations` apa pun.
4. WHEN penonaktifan region berhasil, THE RegionService SHALL menetapkan status region menjadi tidak aktif (`is_active = false`) dan menulis satu entri `audit_logs` yang memuat aktor, aksi, nilai sebelum dan sesudah, waktu (timestamptz), dan alamat IP.

### Requirement 8: Validasi dan penanganan galat

**User Story:** Sebagai admin master-data, saya ingin pesan galat yang jelas dan bentuk respons yang konsisten, agar saya memahami dan memperbaiki masukan yang salah.

#### Acceptance Criteria

1. IF `code` duplikat dikirim pada pembuatan region, THEN THE RegionAPI SHALL menolak operasi tanpa membuat atau mengubah data region apa pun, DAN mengembalikan respons galat berbentuk JSON datar dengan kode status konflik yang memuat pesan yang menyebut nilai kode duplikat tersebut.
2. IF nama tampilan kosong (panjang 0 karakter setelah pemangkasan spasi awal/akhir) dikirim pada pembuatan atau perubahan region, THEN THE RegionAPI SHALL menolak operasi tanpa mengubah data, DAN mengembalikan respons galat berbentuk JSON datar dengan kode status validasi (bad request) yang menyebut nama kolom yang gagal validasi.
3. IF nama tampilan melebihi 100 karakter dikirim pada pembuatan atau perubahan region, THEN THE RegionAPI SHALL menolak operasi tanpa mengubah data, DAN mengembalikan respons galat berbentuk JSON datar dengan kode status validasi (bad request) yang menyebut nama kolom dan batas panjang maksimum yang gagal validasi.
4. IF operasi menyasar region yang tidak ada (pengenal region tidak ditemukan), THEN THE RegionAPI SHALL menolak operasi tanpa mengubah data, DAN mengembalikan respons galat berbentuk JSON datar dengan kode status "tidak ditemukan".
5. THE RegionAPI SHALL menggunakan bentuk respons JSON datar (bukan envelope `pkg/response`) agar konsisten dengan handler admin backend ATM yang ada.
6. IF operasi gagal karena galat validasi, konflik, atau data tidak ditemukan, THEN THE RegionAPI SHALL mempertahankan keadaan data region sebelum operasi tanpa perubahan sebagian (tidak ada penulisan parsial).
7. WHEN galat validasi terjadi, THE RegionManagementUI SHALL menampilkan pesan galat di dekat kolom form terkait beserta ikon dan teks, tanpa mengandalkan warna sebagai satu-satunya penanda galat.

### Requirement 9: Kebutuhan non-fungsional

**User Story:** Sebagai pemilik sistem, saya ingin pengelolaan region mematuhi aturan topologi data dan waktu proyek, agar konsisten dengan modul lain.

#### Acceptance Criteria

1. THE RegionService SHALL melakukan semua operasi tulis region (create, update, disable, enable) pada pool primary.
2. WHEN sebuah permintaan daftar atau laporan region diterima tanpa mendahului operasi tulis dalam alur yang sama, THE RegionService SHALL melayani baca tersebut dari pool replica.
3. WHEN sebuah alur melakukan baca region pada entitas yang sama dalam rentang waktu hingga 5 detik setelah operasi tulis pada entitas tersebut, THE RegionService SHALL membaca dari pool primary.
4. THE RegionService SHALL menyimpan seluruh nilai waktu region dalam tipe `timestamptz` yang disimpan dalam UTC dan ditampilkan dalam zona waktu Asia/Jakarta.
5. IF sebuah operasi tulis region gagal dipersistkan pada pool primary, THEN THE RegionService SHALL membatalkan seluruh perubahan pada operasi tersebut sehingga tidak ada perubahan sebagian yang tersimpan, dan mengembalikan indikasi kegagalan yang menyatakan operasi tidak berhasil.
6. WHEN region baru ditambahkan lewat kartu Settings, THE RegionManagementUI SHALL menyajikannya sebagai kartu "Manajemen Region" pada tab "Data master" mengikuti pola kartu master-data yang ada (rute `/settings/admin/regions`).
