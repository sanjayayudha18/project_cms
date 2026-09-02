# Requirements Document

## Introduction

Fitur ini menjadikan menu DSR (Daily Status Report) sebagai tampilan utama (landing page) bagi Vendor_User di Vendor Portal CMS CIMB Niaga. Setelah login berhasil, Vendor_User langsung diarahkan ke DSR Home, bukan ke dashboard CIT Orders seperti sebelumnya. DSR Home menampilkan identitas vendor yang sedang login (nama vendor) beserta daftar cabang/lokasi yang di-assign ke vendor tersebut, dan menampilkan data DSR (posisi kas per ATM/CRM per hari) yang di-scope hanya ke penugasan (assignment) milik vendor tersebut.

Fitur ini adalah penyempurnaan terfokus dari spec `vendor-portal` yang lebih luas. Fitur ini tidak mengganti seluruh portal, hanya mengubah default landing view menjadi DSR Home dan menambahkan panel identitas vendor + daftar cabang assigned di atas tampilan DSR. Backend menggunakan ATM backend (`backend/`, port 8080) yang memiliki modul `internal/dsr`, `internal/vendor`, `internal/assignment`, `internal/location`, dan `internal/atm`, dengan respons JSON datar (flat) untuk kompatibilitas wire. Scope data vendor ditegakkan berdasarkan `vendor_id` dari klaim JWT, bukan dari parameter yang dikirim klien.

## Glossary

- **Vendor_Portal**: Aplikasi SPA React + TypeScript + Vite yang menghadap ke vendor, berlokasi di `frontend/VendorPortal-Vite/`, menggunakan tema "Merah Menyala"
- **Vendor_User**: Personel vendor CIT (PT Gardanet, PT SSI, atau PT G4S) yang melakukan autentikasi dengan kredensial lokal yang tersimpan di database CMS
- **DSR_Home**: Halaman utama (landing page) Vendor Portal yang menampilkan identitas vendor, daftar cabang yang di-assign, dan tabel data DSR ter-scope vendor
- **Vendor_Identity_Panel**: Komponen pada DSR_Home yang menampilkan nama vendor yang sedang login dan daftar cabang/lokasi yang di-assign ke vendor tersebut
- **DSR**: Daily Status Report, laporan posisi kas per ATM/CRM per hari
- **DSR_Record**: Satu baris data DSR yang berisi posisi kas satu ATM pada satu tanggal tertentu
- **Assigned_Branch**: Cabang atau lokasi (dari tabel `locations`) yang di-assign ke vendor melalui penugasan vendor-ATM/lokasi (`vendor_assignments`)
- **Vendor_Assignment**: Penugasan yang menghubungkan vendor ke ATM dan/atau lokasi tertentu (tabel `vendor_assignments`), menjadi dasar scope data vendor
- **JWT**: JSON Web Token yang diterbitkan saat autentikasi berhasil, berisi klaim `auth_source=local`, `role=Vendor`, dan `vendor_id`
- **Vendor_Scope**: Prinsip isolasi data yang memastikan setiap vendor hanya melihat ATM, lokasi, dan DSR yang di-assign kepadanya
- **ATM**: Automated Teller Machine yang diidentifikasi oleh ID ATM unik
- **Location**: Cabang atau lokasi tempat ATM berada (tabel `locations`)
- **IDR**: Rupiah Indonesia, mata uang yang ditampilkan pada semua nilai moneter
- **Auth_Module**: Komponen autentikasi yang memvalidasi kredensial lokal dan mengelola sesi berbasis JWT
- **App_Shell**: Layout bersama yang mencakup top bar maroon-red dan sidebar dengan tema "Merah Menyala"

## Requirements

### Requirement 1: DSR Home sebagai Landing Page Vendor

**User Story:** Sebagai Vendor_User, saya ingin langsung diarahkan ke menu DSR setelah login, sehingga saya dapat segera melihat posisi kas ATM yang menjadi tanggung jawab saya tanpa navigasi tambahan.

#### Acceptance Criteria

1. WHEN seorang Vendor_User berhasil melakukan autentikasi, THE Vendor_Portal SHALL mengarahkan Vendor_User ke DSR_Home sebagai tampilan default
2. WHEN seorang Vendor_User yang sudah terautentikasi mengakses route root aplikasi, THE Vendor_Portal SHALL menampilkan DSR_Home
3. WHEN seorang Vendor_User yang sudah terautentikasi mengakses route `/login` secara langsung, THE Vendor_Portal SHALL mengarahkan Vendor_User ke DSR_Home
4. WHILE DSR_Home sedang ditampilkan, THE App_Shell SHALL menandai item navigasi DSR sebagai item aktif dengan gaya visual item aktif sesuai tema "Merah Menyala"
5. WHEN seorang Vendor_User yang belum terautentikasi mengakses route DSR_Home, THE Vendor_Portal SHALL menyimpan URL yang diminta dan mengarahkan Vendor_User ke `/login`
6. WHEN seorang Vendor_User menyelesaikan login setelah diarahkan dari route DSR_Home yang dilindungi, THE Vendor_Portal SHALL mengarahkan Vendor_User ke URL DSR_Home yang tersimpan tersebut
7. WHEN Vendor_Portal menyelesaikan pengalihan (redirect) ke DSR_Home setelah login berhasil, THE Vendor_Portal SHALL menyelesaikan pengalihan dan merender DSR_Home dalam waktu maksimal 3 detik pada persentil ke-95
8. IF sesi terautentikasi kedaluwarsa saat Vendor_User berada pada route DSR_Home yang dilindungi, THEN THE Vendor_Portal SHALL menyimpan URL yang diminta dan mengarahkan Vendor_User ke `/login`
9. IF URL pengalihan yang tersimpan tidak valid atau usang setelah login, THEN THE Vendor_Portal SHALL mengarahkan Vendor_User ke root DSR_Home sebagai fallback

### Requirement 2: Tampilan Identitas Vendor dan Cabang yang Di-assign

**User Story:** Sebagai Vendor_User, saya ingin melihat nama perusahaan vendor saya dan daftar cabang yang di-assign ke saya di halaman DSR utama, sehingga saya mengetahui cakupan lokasi yang menjadi tanggung jawab saya.

#### Acceptance Criteria

1. THE Vendor_Identity_Panel SHALL menampilkan nama vendor yang sedang login sesuai klaim `vendor_id` pada JWT
2. THE Vendor_Identity_Panel SHALL menampilkan daftar Assigned_Branch yang di-assign ke vendor yang sedang login, dengan menampilkan nama cabang/lokasi untuk setiap entri, diurutkan berdasarkan nama cabang secara ascending
3. THE Vendor_Identity_Panel SHALL menampilkan jumlah total Assigned_Branch sebagai nilai numerik untuk vendor yang sedang login
4. WHEN Vendor_Identity_Panel dimuat, THE Vendor_Portal SHALL mengambil daftar Assigned_Branch yang di-scope hanya ke `vendor_id` dari klaim JWT
5. WHEN Vendor_Identity_Panel dimuat, THE Vendor_Portal SHALL menampilkan nama vendor dan daftar Assigned_Branch dalam waktu maksimal 3 detik pada persentil ke-95
6. IF pengambilan daftar Assigned_Branch gagal karena kesalahan jaringan atau kesalahan server, THEN THE Vendor_Identity_Panel SHALL menampilkan pesan kesalahan inline dengan tombol coba lagi (retry)
7. IF vendor yang sedang login tidak memiliki Assigned_Branch, THEN THE Vendor_Identity_Panel SHALL menampilkan pesan empty state yang menyatakan bahwa belum ada cabang yang di-assign
8. WHEN nama vendor melebihi 40 karakter, THE Vendor_Identity_Panel SHALL memotong teks dengan elipsis dan menyediakan teks lengkap melalui atribut title
9. THE Vendor_Identity_Panel SHALL menampilkan nama vendor dan daftar cabang sesuai tema "Merah Menyala"

### Requirement 3: Tabel Data DSR Ter-scope Vendor

**User Story:** Sebagai Vendor_User, saya ingin melihat posisi kas harian untuk ATM yang di-assign ke vendor saya di halaman DSR utama, sehingga saya dapat mengantisipasi kebutuhan pengisian ulang kas.

#### Acceptance Criteria

1. THE DSR_Home SHALL menampilkan tabel DSR dengan kolom: ID ATM, Lokasi (Cabang), Tanggal, Saldo Awal (IDR), Cash In (IDR), Cash Out (IDR), Saldo Akhir (IDR), dan Status Saldo
2. WHEN DSR_Home dimuat, THE DSR_Home SHALL memuat DSR_Record yang di-scope hanya ke ATM yang di-assign ke vendor yang sedang login berdasarkan `vendor_id` dari klaim JWT
3. THE DSR_Home SHALL memformat semua nilai moneter sebagai IDR rata kanan dengan pemisah ribuan berupa titik menggunakan tabular-nums
4. THE DSR_Home SHALL menampilkan tanggal DSR dalam zona waktu Asia/Jakarta
5. THE DSR_Home SHALL menyediakan pemilih tanggal (date selector) dan menampilkan data DSR untuk tanggal yang dipilih
6. WHEN DSR_Home dimuat pertama kali, THE DSR_Home SHALL menetapkan tanggal terpilih ke tanggal DSR terbaru yang tersedia bagi vendor yang sedang login, atau ke tanggal hari ini dalam zona waktu Asia/Jakarta jika tidak ada DSR_Record yang tersedia
7. WHEN sebuah tanggal dipilih pada pemilih tanggal, THE DSR_Home SHALL memuat ulang tabel dengan DSR_Record untuk tanggal tersebut, diurutkan berdasarkan ID ATM secara ascending sebagai urutan default
8. THE DSR_Home SHALL menampilkan Status Saldo dengan tingkatan yang konsisten dengan Requirement 4: Kritis saat Saldo Akhir strictly di bawah 50.000.000 IDR, Rendah saat Saldo Akhir lebih besar atau sama dengan 50.000.000 dan lebih kecil atau sama dengan 150.000.000 IDR, dan Normal saat Saldo Akhir strictly di atas 150.000.000 IDR
9. IF tidak ada DSR_Record untuk tanggal yang dipilih, THEN THE DSR_Home SHALL menampilkan pesan empty state yang menyatakan tidak ada data DSR untuk tanggal tersebut, menggantikan tabel data
10. IF vendor yang sedang login tidak memiliki ATM yang di-assign, THEN THE DSR_Home SHALL menampilkan pesan empty state yang menyatakan tidak ada ATM yang di-assign
11. IF pemuatan DSR_Record gagal karena kesalahan jaringan atau kesalahan server, THEN THE DSR_Home SHALL menampilkan pesan kesalahan inline dengan tombol coba lagi (retry) dan mempertahankan tanggal yang dipilih
12. WHEN sebuah header kolom diklik, THE DSR_Home SHALL mengurutkan tabel berdasarkan kolom tersebut, bergantian antara urutan ascending dan descending pada klik berturut-turut

### Requirement 4: Ringkasan DSR Vendor

**User Story:** Sebagai Vendor_User, saya ingin melihat ringkasan posisi kas untuk seluruh ATM yang di-assign ke saya, sehingga saya dapat menilai kondisi keseluruhan dengan cepat.

#### Acceptance Criteria

1. THE DSR_Home SHALL menampilkan kartu ringkasan yang menunjukkan: jumlah ATM yang dipantau (jumlah ID ATM unik), jumlah ATM berstatus Kritis, jumlah ATM berstatus Rendah, dan total Saldo Akhir seluruh ATM yang di-assign untuk tanggal yang dipilih
2. THE DSR_Home SHALL menampilkan Status Saldo menggunakan badge semantik dengan ikon dan teks: Kritis (danger) saat Saldo Akhir strictly di bawah 50.000.000 IDR, Rendah (warning) saat Saldo Akhir lebih besar atau sama dengan 50.000.000 dan lebih kecil atau sama dengan 150.000.000 IDR, dan Normal (success) saat Saldo Akhir strictly di atas 150.000.000 IDR
3. THE DSR_Home SHALL memformat total Saldo Akhir pada kartu ringkasan sebagai IDR rata kanan dengan pemisah ribuan berupa titik menggunakan tabular-nums
4. THE DSR_Home SHALL menghitung nilai ringkasan hanya dari DSR_Record yang di-scope ke vendor yang sedang login untuk tanggal yang dipilih
5. THE DSR_Home SHALL tidak menampilkan data agregat lintas vendor pada kartu ringkasan mana pun
6. WHEN DSR_Home dimuat pertama kali, THE DSR_Home SHALL menghitung nilai kartu ringkasan untuk tanggal terpilih awal yang konsisten dengan Requirement 3 (tanggal DSR terbaru yang tersedia, atau tanggal hari ini dalam zona waktu Asia/Jakarta jika tidak ada DSR_Record)
7. IF vendor yang sedang login tidak memiliki ATM yang di-assign atau tidak ada DSR_Record untuk tanggal yang dipilih, THEN THE DSR_Home SHALL menampilkan kartu ringkasan dengan nilai nol pada seluruh metrik
8. WHEN DSR_Home dimuat, THE DSR_Home SHALL menampilkan kartu ringkasan dalam waktu maksimal 3 detik pada persentil ke-95

### Requirement 5: Isolasi Data Vendor pada DSR Home

**User Story:** Sebagai administrator sistem, saya ingin DSR Home menegakkan isolasi data ketat antar vendor, sehingga tidak ada vendor yang dapat melihat data operasional vendor lain.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL memfilter seluruh kueri data DSR dan cabang berdasarkan `vendor_id` yang diekstrak dari klaim JWT terautentikasi, dan SHALL mengabaikan setiap `vendor_id` atau identifier vendor lain yang dikirim melalui parameter permintaan klien
2. WHEN backend menerima permintaan data DSR atau cabang dari Vendor Portal, THE Vendor_Portal SHALL menegakkan Vendor_Scope pada lapisan middleware dan lapisan service, sehingga permintaan yang lolos middleware namun gagal pemeriksaan scope pada lapisan service tetap ditolak
3. THE DSR_Home SHALL tidak menampilkan kontrol UI apa pun (termasuk selektor vendor, filter vendor, atau kolom pencarian vendor) yang memungkinkan Vendor_User memilih, mencari, atau menampilkan data milik vendor selain vendor yang tertaut pada sesi terautentikasi
4. IF seorang Vendor_User mengirim permintaan untuk mengakses DSR_Record atau cabang yang tidak di-assign ke vendornya, THEN THE Vendor_Portal SHALL menolak permintaan tanpa mengembalikan data sumber daya tersebut, menampilkan pesan yang menyatakan akses ditolak, dan mencatat satu entri audit yang memuat identitas pengguna terautentikasi, identifikasi sumber daya yang diminta, dan timestamp dalam UTC
5. IF klaim JWT tidak memuat `vendor_id` yang valid, atau `vendor_id` tidak merujuk pada vendor aktif yang terdaftar, THEN THE Vendor_Portal SHALL menolak akses ke seluruh sumber daya ter-scope vendor dan menampilkan pesan yang menyatakan sesi tidak valid
6. WHEN backend menerima permintaan data DSR atau cabang tanpa token JWT atau dengan token JWT yang gagal validasi, THE Vendor_Portal SHALL menolak permintaan tanpa mengembalikan data ter-scope vendor dan menampilkan pesan yang menyatakan sesi tidak valid

### Requirement 6: Pengambilan Data DSR Home melalui API Backend

**User Story:** Sebagai developer, saya ingin DSR Home mengambil data cabang dan DSR melalui ATM backend dengan scope vendor yang benar, sehingga tampilan konsisten dengan kontrak API yang ada.

#### Acceptance Criteria

1. THE Vendor_Portal SHALL mengambil daftar Assigned_Branch dan DSR_Record melalui ATM backend pada port 8080
2. THE Vendor_Portal SHALL menyertakan JWT sebagai Bearer token pada header Authorization untuk setiap permintaan data ke ATM backend
3. IF permintaan data ke ATM backend tidak menyertakan JWT yang valid pada header Authorization, THEN THE ATM_Backend SHALL menolak permintaan tanpa mengembalikan data DSR atau cabang dan mengembalikan indikasi kesalahan otorisasi kepada pemanggil
4. WHEN ATM backend melayani permintaan data DSR atau cabang untuk Vendor Portal, THE ATM_Backend SHALL mengembalikan respons dalam bentuk JSON datar (flat) sesuai kompatibilitas wire yang ada
5. WHEN ATM backend membaca data DSR atau cabang untuk keperluan tampilan, THE ATM_Backend SHALL membaca dari read replica
6. WHEN Vendor_Portal memanggil endpoint data dengan parameter `vendor_id`, THE ATM_Backend SHALL mengabaikan `vendor_id` yang dikirim klien dan menggunakan `vendor_id` dari klaim JWT
7. WHEN ATM backend mengembalikan daftar Assigned_Branch dan DSR_Record, THE ATM_Backend SHALL hanya menyertakan data yang tercakup dalam assignment `vendor_id` dari klaim JWT dan mengecualikan data vendor lain
8. WHEN Vendor_Portal mengirim permintaan data DSR atau cabang, THE ATM_Backend SHALL merespons dalam waktu maksimal 3 detik pada persentil ke-95
9. IF permintaan data ke ATM backend gagal karena kesalahan jaringan, kesalahan server, atau tidak menerima respons dalam 10 detik, THEN THE DSR_Home SHALL menampilkan pesan kesalahan inline yang mengindikasikan kegagalan pengambilan data, menampilkan tombol coba lagi (retry), dan mempertahankan tanggal yang dipilih

### Requirement 7: Aksesibilitas dan Tema pada DSR Home

**User Story:** Sebagai Vendor_User yang mengakses portal dari berbagai perangkat, saya ingin DSR Home dapat digunakan dan sesuai standar aksesibilitas, sehingga saya dapat memeriksa data dari lapangan.

#### Acceptance Criteria

1. WHEN DSR_Home menampilkan indikator Status Saldo, THE DSR_Home SHALL menampilkan tiga elemen secara bersamaan untuk setiap indikator: warna semantik, ikon yang unik per status, dan label teks status
2. THE DSR_Home SHALL menggunakan landmark HTML semantik `<main>` untuk konten utama dan elemen `<table>` dengan struktur baris dan kolom (header kolom melalui `<th>` dan baris melalui `<tr>`) untuk data tabular sehingga pembaca layar dapat mengumumkan region halaman serta baris dan kolom tabel
3. WHILE lebar viewport kurang dari 1024px, THE DSR_Home SHALL membungkus tabel DSR dalam kontainer yang dapat digulir horizontal dengan scrollbar horizontal yang terlihat sehingga seluruh sel tabel tetap dapat diakses dan tidak ada sel yang tersembunyi atau terpotong
4. THE DSR_Home SHALL mempertahankan rasio kontras warna WCAG 2.1 AA (minimum 4.5:1 untuk teks di bawah 18,66px bold atau 24px reguler, minimum 3:1 untuk teks pada atau di atas ambang tersebut) antara teks dan latarnya, termasuk teks label pada badge Status Saldo terhadap latar badge tersebut
5. WHEN seorang Vendor_User bernavigasi menggunakan keyboard, THE DSR_Home SHALL menampilkan indikator fokus yang terlihat pada elemen interaktif yang sedang difokuskan dengan rasio kontras non-teks minimum 3:1 terhadap warna sekitarnya
6. WHEN seorang Vendor_User bernavigasi menggunakan tombol Tab, THE DSR_Home SHALL menjangkau seluruh elemen interaktif secara berurutan tanpa jebakan keyboard (keyboard trap)
