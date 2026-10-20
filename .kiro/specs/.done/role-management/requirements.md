# Requirements Document

## Introduction

Fitur **Role Management** (Manajemen Peran) menambahkan menu baru di bawah **Pengaturan (Settings)** pada frontend internal CompanyPortal-Vite. Fitur ini didukung oleh lapisan izin (permission layer) berbasis data di backend ATM (`backend/`, port 8080).

Tujuan utama: pengguna dengan role `APPACCESS` atau `ADMIN` dapat (a) membuat role baru, dan (b) mengatur menu dan fitur mana yang dapat diakses oleh tiap role. Perubahan berlaku seketika (immediate-apply) tanpa gerbang maker-checker, tetapi setiap operasi buat/ubah/hapus menulis ke `audit_logs`.

Fitur ini menggantikan model izin yang saat ini hardcoded. Tabel `roles` (migration 002) hanya menyimpan `id`, `role`, `description` tanpa pemetaan izin apa pun. Fitur ini memperkenalkan katalog menu/fitur dan pemetaan `role_permissions` yang dibaca saat runtime oleh middleware backend dan navigasi frontend.

### Tradeoff Deviation Notice (explicit)

Golden Rule #3 pada `project-context.md` menyatakan bahwa setiap perubahan create/update/delete pada data konfigurasi/master normalnya melewati maker-checker (`approval_requests`). Untuk fitur ini, pengguna **secara eksplisit memilih immediate-apply (audit-only, tanpa maker-checker)**. Requirement 6 mendokumentasikan penyimpangan ini sebagai keputusan sadar; kontrol pengganti adalah audit trail lengkap (who/what/before/after/when/ip) plus pembatasan otorisasi ke `APPACCESS`/`ADMIN`.

## Glossary

- **Role_Management_System**: Modul backend + frontend yang mengelola pembuatan role dan pemetaan izin menu/fitur.
- **Permission_Middleware**: Lapisan middleware backend ATM yang mengevaluasi izin dinamis berbasis DB untuk route yang mengonsultasi permission layer.
- **Permission_Service**: Lapisan service backend yang menegakkan otorisasi dan aturan bisnis izin, terpisah dari HTTP handler.
- **Menu_Feature_Catalog**: Tabel katalog yang mendaftarkan menu dan fitur yang tersedia untuk dipetakan ke role.
- **Role_Permission_Mapping**: Tabel pemetaan (`role_permissions`) yang menghubungkan role dengan entri di Menu_Feature_Catalog.
- **Audit_Writer**: Komponen `internal/audit` yang menulis entri append-only ke `audit_logs`.
- **APPACCESS**: Role dengan wewenang penyediaan akun dan konfigurasi RBAC/permission.
- **ADMIN**: Role administrator sistem.
- **Nav_Renderer**: Komponen frontend CompanyPortal-Vite yang merender navigasi di bawah Pengaturan berdasarkan izin yang dibaca dari backend.
- **Primary_Pool**: Koneksi database primary (write) via `pkg/database`.
- **Replica_Pool**: Koneksi database read replica untuk operasi baca/pelaporan.
- **Static_Guarded_Route**: Route backend yang sudah ada dan memakai `RequireRoles(...)` hardcoded.
- **Dynamic_Route**: Route baru yang mengonsultasi Permission_Middleware alih-alih guard statis.

## Requirements

### Requirement 1: Data-driven permission catalog & mapping

**User Story:** Sebagai administrator sistem, saya ingin izin menu/fitur disimpan di database sebagai katalog dan pemetaan, sehingga akses dapat diubah tanpa deploy ulang kode.

#### Acceptance Criteria

1. THE Role_Management_System SHALL menyimpan daftar menu dan fitur yang dapat dipetakan pada Menu_Feature_Catalog di database.
2. THE Role_Management_System SHALL menyimpan hubungan antara satu role dan satu entri Menu_Feature_Catalog pada Role_Permission_Mapping di database.
3. WHEN Permission_Middleware mengevaluasi izin untuk sebuah Dynamic_Route, THE Permission_Middleware SHALL membaca izin dari Role_Permission_Mapping saat runtime.
4. WHEN Nav_Renderer merender navigasi di bawah Pengaturan, THE Nav_Renderer SHALL menampilkan hanya menu dan fitur yang dipetakan ke role pengguna aktif berdasarkan data yang dibaca dari backend.
5. WHEN seorang administrator mengubah Role_Permission_Mapping untuk sebuah role, THE Role_Management_System SHALL menerapkan perubahan akses tersebut pada evaluasi izin berikutnya tanpa memerlukan restart layanan.
6. WHERE sebuah tabel atau kolom database baru diperlukan untuk Menu_Feature_Catalog atau Role_Permission_Mapping, THE Role_Management_System SHALL mensyaratkan tabel/kolom tersebut diusulkan pada `project-context.md` Sec 2 dan disetujui sebelum migrasi dibuat.

### Requirement 2: Create new role

**User Story:** Sebagai pengguna APPACCESS atau ADMIN, saya ingin membuat role baru, sehingga struktur peran dapat berkembang sesuai kebutuhan organisasi.

#### Acceptance Criteria

1. WHEN pengguna APPACCESS atau ADMIN mengirim permintaan pembuatan role dengan nama dan deskripsi yang valid, THE Role_Management_System SHALL membuat entri role baru pada Primary_Pool.
2. WHEN sebuah role baru berhasil dibuat, THE Role_Management_System SHALL memberikan role tersebut tanpa akses menu atau fitur apa pun secara default.
3. IF nama role yang dikirim sudah ada, THEN THE Role_Management_System SHALL menolak permintaan dan mengembalikan pesan kesalahan yang menyatakan nama role telah digunakan.
4. IF nama role kosong atau tidak memenuhi aturan validasi, THEN THE Role_Management_System SHALL menolak permintaan dan mengembalikan detail kesalahan validasi.
5. WHEN sebuah role baru berhasil dibuat, THE Audit_Writer SHALL menulis entri `audit_logs` yang berisi aktor, aksi, nilai sesudah, waktu, dan alamat IP.

### Requirement 3: Edit role menu & feature access

**User Story:** Sebagai pengguna APPACCESS atau ADMIN, saya ingin mengatur menu dan fitur yang dapat diakses tiap role, sehingga tiap peran hanya melihat kemampuan yang relevan.

#### Acceptance Criteria

1. WHEN pengguna APPACCESS atau ADMIN mengirim perubahan pemetaan menu/fitur untuk sebuah role, THE Role_Management_System SHALL memperbarui Role_Permission_Mapping pada Primary_Pool.
2. WHEN perubahan pemetaan berhasil disimpan, THE Role_Management_System SHALL menerapkan akses baru pada evaluasi izin berikutnya untuk pengguna dengan role tersebut.
3. IF perubahan merujuk entri Menu_Feature_Catalog yang tidak ada, THEN THE Role_Management_System SHALL menolak permintaan dan mengembalikan pesan kesalahan.
4. WHEN pemetaan menu/fitur sebuah role diubah, THE Audit_Writer SHALL menulis entri `audit_logs` yang berisi aktor, aksi, nilai sebelum, nilai sesudah, waktu, dan alamat IP.
5. WHEN pengguna membuka halaman Role Management, THE Role_Management_System SHALL menampilkan pemetaan menu/fitur saat ini untuk tiap role berdasarkan pembacaan dari Replica_Pool.

### Requirement 4: Authorization enforcement (menu + service layer)

**User Story:** Sebagai pemilik sistem, saya ingin hanya APPACCESS dan ADMIN yang dapat membuka dan mengubah Role Management, sehingga izin tidak dapat dimodifikasi oleh peran yang tidak berwenang.

#### Acceptance Criteria

1. WHEN pengguna dengan role APPACCESS atau ADMIN mengakses endpoint Role Management, THE Permission_Middleware SHALL mengizinkan permintaan diteruskan.
2. IF pengguna tanpa role APPACCESS atau ADMIN mengakses endpoint Role Management, THEN THE Permission_Middleware SHALL menolak permintaan dan mengembalikan status 403.
3. WHEN Permission_Service memproses operasi buat atau ubah role, THE Permission_Service SHALL memverifikasi ulang bahwa aktor memiliki role APPACCESS atau ADMIN sebelum menerapkan perubahan.
4. IF Permission_Service menerima operasi dari aktor tanpa role APPACCESS atau ADMIN, THEN THE Permission_Service SHALL menolak operasi dan mengembalikan kesalahan otorisasi.
5. WHERE pengguna tidak memiliki izin untuk menu Role Management, THE Nav_Renderer SHALL menyembunyikan entri menu Role Management di bawah Pengaturan.

### Requirement 5: New-role behavior & boundary with legacy static guards

**User Story:** Sebagai arsitek sistem, saya ingin lapisan izin dinamis terpisah bersih dari guard statis yang ada, sehingga tidak ada kopling ke `RequireRoles(...)` legacy.

#### Acceptance Criteria

1. WHEN sebuah role baru dibuat, THE Role_Management_System SHALL memberi role tersebut nol akses hingga izin diberikan melalui Menu_Feature_Catalog.
2. THE Role_Management_System SHALL membiarkan Static_Guarded_Route yang memakai `RequireRoles(...)` tetap tidak berubah.
3. WHEN sebuah Dynamic_Route dievaluasi, THE Permission_Middleware SHALL menentukan akses hanya dari Role_Permission_Mapping tanpa merujuk daftar role statis yang hardcoded.
4. THE Role_Management_System SHALL memisahkan konsep akses menu/fitur dari konsep hierarki approval/delegasi/policy pada modul rbac-settings yang sudah ada.

### Requirement 6: Audit-only change gating (no maker-checker) — documented deviation

**User Story:** Sebagai pemilik sistem, saya ingin perubahan Role Management berlaku seketika dengan jejak audit penuh, sehingga administrasi peran cepat namun tetap dapat dilacak.

#### Acceptance Criteria

1. WHEN operasi buat, ubah, atau hapus pada Role Management berhasil, THE Role_Management_System SHALL menerapkan perubahan seketika tanpa membuat `approval_requests`.
2. WHEN operasi buat, ubah, atau hapus pada Role Management berhasil, THE Audit_Writer SHALL menulis entri `audit_logs` yang berisi aktor, aksi, nilai sebelum, nilai sesudah, waktu, dan alamat IP.
3. IF penulisan `audit_logs` gagal untuk sebuah operasi perubahan, THEN THE Role_Management_System SHALL menggagalkan operasi tersebut dan mengembalikan kesalahan sehingga perubahan tidak diterapkan tanpa jejak audit.
4. THE Role_Management_System SHALL mencatat penyimpangan dari Golden Rule #3 (immediate-apply tanpa maker-checker) sebagai keputusan desain yang disengaja pada dokumentasi fitur.

### Requirement 7: Frontend menu under Settings (Pengaturan)

**User Story:** Sebagai pengguna APPACCESS atau ADMIN, saya ingin mengelola role dari menu Pengaturan yang konsisten dengan pola UI yang ada, sehingga pengalaman tetap familiar.

#### Acceptance Criteria

1. WHERE pengguna memiliki izin Role Management, THE Nav_Renderer SHALL menampilkan entri menu Role Management di bawah Pengaturan.
2. WHEN halaman Role Management dirender, THE Role_Management_System SHALL menggunakan tema "Merah Sirih" dan pola UI yang konsisten dengan modul rbac-settings (RoleBadge, layout, form Zod + React Hook Form, salinan Bahasa Indonesia).
3. WHEN administrator mengirim form pembuatan atau pengubahan role, THE Role_Management_System SHALL memvalidasi input di sisi klien menggunakan skema Zod sebelum memanggil backend.
4. WHEN backend mengembalikan respons untuk endpoint Role Management, THE Role_Management_System SHALL menggunakan bentuk JSON datar (flat) yang konsisten dengan backend ATM, bukan envelope `pkg/response`.

### Requirement 8: Data integrity & topology

**User Story:** Sebagai insinyur backend, saya ingin operasi Role Management mengikuti aturan topologi database dan integritas data CMS, sehingga konsisten dengan sistem lainnya.

#### Acceptance Criteria

1. WHEN Role_Management_System melakukan operasi tulis, THE Role_Management_System SHALL menggunakan Primary_Pool.
2. WHEN Role_Management_System melakukan operasi baca untuk pelaporan atau tampilan daftar, THE Role_Management_System SHALL menggunakan Replica_Pool.
3. WHERE sebuah alur melakukan baca segera setelah tulis pada data yang sama, THE Role_Management_System SHALL membaca dari Primary_Pool untuk menghindari lag replika.
4. THE Role_Management_System SHALL menyertakan pengujian dengan cakupan minimal 80% pada paket `internal/*` yang terkait, termasuk kasus otorisasi APPACCESS/ADMIN, penolakan role lain, dan penulisan audit.
