# Requirements Document

## Introduction

Fitur **Vendor Party Scope** memisahkan akun vendor di VendorPortal menurut **peran pihak**: penyedia uang (**vault**) atau pengisian ATM (**replenish**), membatasinya per **cabang vendor**, dan memisahkan **PIC vendor** per peran. Sumber: `.claude/sdlc/vendor-party-scope/intent.md` (draft 2026-10-20). Keputusan PO diambil satu per satu pada 2026-10-20 (bagian "Keputusan PO"); D1–D9 semuanya diputuskan.

Masalah yang dilaporkan user (cek manual `REP-ABA-MDN-20261014-001`, login vendor Abacus): tiket tampil dua baris, *Vault (penyedia uang)* dan *Replenish (pelaksana)*, keduanya Abacus Medan, di satu akun. User ingin akun yang berbeda sehingga tiket penyedia uang dan pengisian ATM tidak tercampur, dan akun cabang Medan tidak melihat request untuk cabang lain.

Kondisi sekarang (dicek di kode 2026-10-20):
- Cakupan user vendor (Phase 2.2b) = `users.vendor_id` (claim JWT, dicocokkan dengan DB) + `users.vendor_branch_id` (dibaca dari DB per request; `NULL` = semua cabang vendor). Tidak ada pembeda peran: semua akun vendor memakai role `VENDOR-USER` dan melihat pihak `replenish` **dan** `vault` (`backend/queries/vendor_parties.sql`, `backend/internal/service/vendor_order.go`).
- Notifikasi pihak (`notifyParties`, `backend/internal/service/vendor_party.go`) dikirim ke user vendor cabang itu atau vendor-wide + email PIC cabang itu atau vendor-wide, tanpa peran.
- `vendor_pics` tidak punya kolom peran. PIC dikelola lewat maker-checker master data (`/api/v1/admin/vendors/{vendorID}/pics`, `ADMIN`/`ADMIN_PARAM`) dan ikut ekspor/impor CSV master data.
- `/api/v1/admin/users` hanya mengatur `vendor_id`. **`vendor_branch_id` tidak bisa diatur lewat API/UI** dan tidak ada cek bahwa cabang milik vendor.
- Form user di CompanyPortal memuat vendor dari `/api/v1/admin/vendors` (guard `ADMIN`/`ADMIN_PARAM`), sehingga **APPACCESS tidak bisa memilih vendor** di form user.
- Role `VENDOR-USER` ditulis langsung di: guard login/refresh VendorPortal (`AuthContext.tsx`), guard tolak-vendor CompanyPortal (`lib/auth/store.ts`), `RequireRoles("VENDOR-USER")` di `/vendor/replenish-orders` dan `/vendor/replenish-realizations`, `vendorUserRole` di service, daftar role di form admin users dan badge RBAC.

### Ruang lingkup

Termasuk: dua role vendor baru; cakupan role + cabang di semua endpoint `/api/v1/vendor/*`; notifikasi pihak per role + cabang; peran PIC vendor (layar, CSV, notifikasi); pengaturan role/cabang akun vendor lewat admin users (backend + CompanyPortal); tampilan cakupan di VendorPortal; seed dev.

Tidak termasuk: cakupan per region; laporan realisasi parsial per cabang; notifikasi ke PIC cash count (Phase 5 cash count masih parkir); scoping upload DSR per role/cabang (tetap per vendor, ketiga role vendor boleh upload); halaman mock VendorPortal (Schedule, Invoices, Evidence).

### ⚠ Area sensitif (project-context Sec 3 rule 7)

- **Auth/RBAC**: role baru + cakupan data vendor berubah di semua endpoint vendor dan guard kedua portal; `ADMIN_PARAM` kehilangan akses `/api/v1/admin/users` (D8). Risiko utama: tiket role lain atau cabang lain bocor ke akun yang salah, atau akun internal masuk VendorPortal.
- **Skema**: migrasi `032_vendor_party_scope.sql` (role baru, FK komposit `users`, kolom peran `vendor_pics`). `022` tetap dicadangkan untuk vendor-pks-cis-limit.
- **Deviasi terdokumentasi dari Golden Rule #3**: perubahan **peran PIC** lewat layar langsung berlaku (tanpa maker-checker) + audit dalam tx yang sama. Wajib dicatat di CLAUDE.md Sec 12.
- **Pengecualian scope guard** (development-plan rule 2, "No new master-data features"): perubahan PIC vendor diizinkan eksplisit oleh PO untuk fitur ini.

### Keputusan PO (2026-10-20)

- **D1 (Q1) — Role baru, bukan kolom.** Ganti peran = ganti role; berlaku setelah token lama ditolak (lihat Requirement 3.8).
- **D2 (Q2, Q3) — `VENDOR-USER` tetap ada sebagai akun dua peran.** Melihat dan memutuskan pihak `replenish` dan `vault` di cakupan cabangnya, persis seperti sekarang. Tidak ada akun yang diblokir, tidak ada migrasi data role. Satu user = satu role (`users.role_id`).
- **D3 — Nama role: `VENDOR-ATM-USER` (pengisian ATM, pihak `replenish`) dan `VENDOR-VAULT-USER` (penyedia uang, pihak `vault`).** Ketiga role vendor = role VendorPortal (`auth_source=local`, `vendor_id` wajib).
- **D4 (Q4) — Cakupan per cabang vendor (`users.vendor_branch_id`), bukan per region.** Akun vendor-wide (`NULL`, "Semua cabang") tetap diizinkan untuk ketiga role vendor.
- **D5 (Q5) — Seed dev**: akun lama (`vendor.<code>`, `vendor.ssi`, `vendor.bijak`) tetap `VENDOR-USER`. Untuk ABACUS ditambah empat akun berbatas cabang: Medan dan satu cabang non-Medan, masing-masing × {`VENDOR-ATM-USER`, `VENDOR-VAULT-USER`}.
- **D6a (Q6) — Notifikasi pihak ke akun user**: pihak `replenish` → `VENDOR-ATM-USER` + `VENDOR-USER`; pihak `vault` → `VENDOR-VAULT-USER` + `VENDOR-USER`; hanya cabang pihak atau vendor-wide.
- **D6b — PIC vendor dipisah per peran**: `replenish` (PIC replenish ATM), `vault` (PIC penyedia uang di vault), `cashcount` (PIC cash count). Email pihak ke PIC hanya bila peran cocok.
- **D6c — Satu PIC boleh lebih dari satu peran.**
- **D6d — Semua PIC yang sudah ada diberi ketiga peran oleh migrasi.**
- **D6e — Peran PIC lewat layar langsung berlaku** (tanpa maker-checker, audit dalam tx). Field PIC lain tetap maker-checker.
- **D6f — Pengubah peran PIC: `ADMIN` dan `ADMIN_PARAM`.**
- **D6g — CSV PIC membaca dan menulis peran**; lewat impor, peran berlaku saat batch disetujui (maker-checker impor).
- **D7 (Q7) — Realisasi untuk `VENDOR-ATM-USER` dan `VENDOR-USER`**; `VENDOR-VAULT-USER` tidak.
- **D8 (Q8) — ✅ DIPUTUSKAN PO 2026-10-20 (opsi C): `/api/v1/admin/users` hanya untuk `APPACCESS` dan `ADMIN`; `ADMIN_PARAM` dicabut.** Berlaku untuk **semua** akun (bukan hanya akun vendor) dan semua endpoint di router itu, termasuk `set-initial-password`. Perubahan RBAC yang disengaja di luar inti fitur; `ADMIN_PARAM` tetap mengelola master data lain.
- **D9 — ✅ DIPUTUSKAN PO 2026-10-20 (opsi A): lookup vendor + cabang khusus form user** (`GET /api/v1/admin/users/vendor-options`), guard sama dengan `/admin/users` (`APPACCESS`, `ADMIN`). Hanya vendor + cabang aktif dengan field yang dibutuhkan form; APPACCESS tidak mendapat akses ke master data vendor lain.

## Glossary

- **Pihak (party)**: baris `vendor_request_vendor_parties` = (Vendor Request, cabang vendor, peran `replenish` | `vault`). Satu cabang bisa menjadi dua pihak di request yang sama.
- **Role vendor**: salah satu dari `VENDOR-USER`, `VENDOR-ATM-USER`, `VENDOR-VAULT-USER`.
- **Peran yang diizinkan (allowed party roles)**: `VENDOR-USER` → {`replenish`, `vault`}; `VENDOR-ATM-USER` → {`replenish`}; `VENDOR-VAULT-USER` → {`vault`}.
- **Cabang akun**: `users.vendor_branch_id`; `NULL` = **vendor-wide**.
- **Cakupan (scope)**: himpunan pihak yang boleh dilihat dan diputuskan sebuah akun vendor.
- **Peran PIC**: anggota `vendor_pics.roles` ⊆ {`replenish`, `vault`, `cashcount`}.
- **VendorScope**: komponen backend yang menghitung cakupan dari `users` + `roles` (primary) per request (`VendorOrderService.scope`).
- **VendorOrderAPI**: `/api/v1/vendor/replenish-orders*` dan `/api/v1/vendor/replenish-realizations`.
- **UserAdminAPI**: `/api/v1/admin/users*`.
- **PicAdminAPI**: `/api/v1/admin/vendors/{vendorID}/pics*`.
- **PartyNotifier**: `notifyParties` + `internal/notification`.
- **VendorPortalUI**: `frontend/VendorPortal-Vite`. **CompanyPortalUI**: `frontend/CompanyPortal-Vite`.

## Requirements

### Requirement 1: Role vendor baru

**User Story:** Sebagai admin akun, saya ingin role vendor yang memisahkan penyedia uang dan pengisian ATM, agar tiket kedua peran tidak tercampur di satu akun.

#### Acceptance Criteria

1. THE migrasi SHALL menambah role `VENDOR-ATM-USER` dan `VENDOR-VAULT-USER` ke `roles` (idempoten terhadap `roles_role_key`).
2. THE migrasi SHALL memberi kedua role baru `role_permissions` yang sama dengan `VENDOR-USER` saat migrasi dijalankan.
3. THE system SHALL tidak mengubah role akun yang sudah ada.
4. THE system SHALL memperlakukan ketiga role vendor sebagai role VendorPortal: wajib `auth_source=local` dan `vendor_id` terisi.

### Requirement 2: Mengatur role dan cabang akun vendor

**User Story:** Sebagai admin akun (APPACCESS/ADMIN), saya ingin mengatur role dan cabang akun vendor dari CompanyPortal, agar tidak perlu mengubah database langsung.

#### Acceptance Criteria

0. THE UserAdminAPI SHALL hanya mengizinkan `APPACCESS` dan `ADMIN` di route (`RequireRoles`) untuk semua endpoint `/api/v1/admin/users*`; `ADMIN_PARAM` dan role lain → 403 (D8).

1. THE UserAdminAPI SHALL menerima `vendor_branch_id` pada create dan update, dan mengembalikan `vendor_branch_id` + `vendor_branch_name` pada list, detail, create, dan update.
2. WHEN role = salah satu role vendor, THE UserAdminAPI SHALL mewajibkan `auth_source=local` dan `vendor_id` (400 per field).
3. WHEN role bukan role vendor, THE UserAdminAPI SHALL menolak `vendor_branch_id` yang terisi (400 `vendor_branch_id`).
4. IF `vendor_branch_id` bukan cabang aktif milik `vendor_id`, THEN THE UserAdminAPI SHALL menolak dengan 400 `vendor_branch_id`, termasuk saat `vendor_id` diganti tanpa mengganti cabang.
5. THE database SHALL menolak `vendor_branch_id` yang bukan milik `vendor_id` (FK komposit), sebagai lapis kedua.
6. WHEN create atau update berhasil, THE UserAdminAPI SHALL menulis `audit_logs` (`user_created` / `user_updated`) yang memuat role dan `vendor_branch_id` pada before/after.
7. THE UserAdminAPI SHALL menyediakan `GET /api/v1/admin/users/vendor-options` dengan guard yang sama dengan `/admin/users` (`APPACCESS`, `ADMIN`): vendor aktif (`id`, `code`, `name`) + cabang aktif (`id`, `vendor_id`, `branch_code`, `branch_name`, `region_code`), dibaca dari replica.
8. THE CompanyPortalUI SHALL menampilkan ketiga role vendor di pilihan role form user, dengan label yang menjelaskan perannya.
9. WHEN role vendor dipilih, THE CompanyPortalUI SHALL menampilkan select Vendor dan select **Cabang** ("Semua cabang" + cabang aktif vendor terpilih) dari `vendor-options`.
10. WHEN vendor diganti di form, THE CompanyPortalUI SHALL mengosongkan pilihan cabang.
11. THE CompanyPortalUI SHALL memuat vendor di form dan filter user dari `vendor-options` (bukan `/admin/vendors`), sehingga APPACCESS dapat memakai form.
12. THE CompanyPortalUI SHALL menampilkan kolom Cabang di tabel user.
13. WHILE role = `ADMIN_PARAM`, THE CompanyPortalUI SHALL tidak menampilkan kartu/menu Manajemen User. (Catatan: `requireRoles()` di frontend selalu meloloskan `ADMIN`/`ADMIN_PARAM`; backend tetap penjaga utama — gap yang sudah ada di route settings lain.)

### Requirement 3: Cakupan Orders per role dan cabang

**User Story:** Sebagai user vendor, saya ingin hanya melihat tiket untuk peran dan cabang akun saya, agar tidak salah memutuskan tiket milik peran atau cabang lain.

#### Acceptance Criteria

1. THE VendorOrderAPI SHALL mengizinkan ketiga role vendor di route (`RequireRoles`) dan service.
2. THE VendorScope SHALL mendefinisikan cakupan sebagai: `users.vendor_id` = claim `vendor_id` ∧ `party.role` ∈ peran yang diizinkan role akun ∧ (`users.vendor_branch_id IS NULL` ∨ `party.vendor_branch_id` = `users.vendor_branch_id`).
3. THE VendorOrderAPI SHALL menerapkan cakupan di query SQL untuk daftar, hitung, detail, terima, tolak, dan laporan realisasi.
4. IF pihak yang diminta di luar cakupan (peran lain, cabang lain, atau vendor lain), THEN THE VendorOrderAPI SHALL menjawab 404.
5. WHEN satu cabang menjadi pihak `replenish` dan `vault` di request yang sama, THE VendorOrderAPI SHALL menampilkan pihak `replenish` hanya ke `VENDOR-ATM-USER` dan `VENDOR-USER`, dan pihak `vault` hanya ke `VENDOR-VAULT-USER` dan `VENDOR-USER`.
6. WHILE role akun = `VENDOR-USER`, THE VendorOrderAPI SHALL berperilaku seperti sebelum fitur ini (kedua peran pihak).
7. THE system SHALL tidak mengubah alur terima/tolak pihak, status Vendor Request, atau `content` snapshot pihak.
8. IF role di JWT berbeda dengan role akun di DB (role diganti setelah login), THEN THE VendorScope SHALL menolak dengan 403, tanpa menunggu token kedaluwarsa.
9. THE VendorScope SHALL membaca role dan cabang dalam satu query cakupan (primary), tanpa query tambahan per request.

### Requirement 4: Realisasi

**User Story:** Sebagai user vendor pengisian ATM, saya ingin laporan realisasi hanya ada di akun yang mengisi ATM, agar akun penyedia uang tidak bisa melapor realisasi.

#### Acceptance Criteria

1. IF `VENDOR-VAULT-USER` mengirim `POST /api/v1/vendor/replenish-orders/{id}/realization`, THEN THE VendorOrderAPI SHALL menjawab 404 (pihak replenish di luar cakupan).
2. WHEN `VENDOR-VAULT-USER` memanggil `GET /api/v1/vendor/replenish-realizations`, THE VendorOrderAPI SHALL menjawab 403.
3. THE VendorOrderAPI SHALL tetap mewajibkan semua pihak replenish request berada di cakupan pelapor (`checkReportsWholeRequest`), sehingga request multi-cabang dilaporkan akun vendor-wide.
4. WHEN `VENDOR-ATM-USER` atau `VENDOR-USER` memanggil `GET /api/v1/vendor/replenish-realizations`, THE VendorOrderAPI SHALL mengembalikan klasifikasi vendor dan cabang akun seperti sekarang.

### Requirement 5: Peran PIC vendor

**User Story:** Sebagai admin master data, saya ingin menandai peran setiap PIC vendor, agar email tiket hanya diterima PIC yang mengurus peran itu.

#### Acceptance Criteria

1. THE system SHALL menyimpan peran PIC sebagai himpunan ⊆ {`replenish`, `vault`, `cashcount`}; himpunan kosong diizinkan (PIC hanya sebagai kontak).
2. THE migrasi SHALL memberi ketiga peran ke semua PIC yang sudah ada.
3. THE PicAdminAPI SHALL menyediakan `PUT /api/v1/admin/vendors/{vendorID}/pics/{id}/roles` (body `{roles: [...]}`) untuk `ADMIN` dan `ADMIN_PARAM`, dicek di route dan service.
4. WHEN peran PIC diubah lewat endpoint itu, THE PicAdminAPI SHALL menerapkannya langsung dan menulis `audit_logs` (`vendor_pic_roles_updated`, before/after) dalam transaksi yang sama; IF tulis audit gagal, THEN perubahan SHALL dibatalkan.
5. IF nilai peran tidak dikenal, duplikat, atau PIC bukan milik `vendorID`/nonaktif, THEN THE PicAdminAPI SHALL menolak (400 `roles` / 404).
6. WHEN PIC baru dibuat lewat form (maker-checker), THE PicAdminAPI SHALL menerima peran di payload create dan menerapkannya saat disetujui; tanpa peran = ketiga peran.
7. THE alur maker-checker update PIC dari form SHALL tidak mengubah peran.
8. THE PicAdminAPI SHALL mengembalikan `roles` pada list dan detail PIC.
9. THE CompanyPortalUI SHALL menampilkan peran PIC di daftar PIC (label teks), checkbox peran di form buat PIC, dan aksi **Ubah peran** yang langsung berlaku dengan penanda jelas bahwa perubahan tidak lewat persetujuan.

### Requirement 6: Peran PIC di CSV master data

**User Story:** Sebagai admin master data, saya ingin peran PIC ikut ekspor/impor CSV, agar pengisian massal tidak perlu diulang di layar.

#### Acceptance Criteria

1. THE ekspor CSV PIC SHALL menyertakan kolom `roles` berisi kode peran dipisah `;` (mis. `replenish;vault`), `none` untuk himpunan kosong.
2. THE impor CSV PIC SHALL memvalidasi kolom `roles` (kode dikenal, tanpa duplikat) di dry-run.
3. WHEN baris impor membuat PIC baru dengan `roles` kosong, THE system SHALL memberi ketiga peran; WHEN baris impor mengubah PIC dengan `roles` kosong, THE system SHALL tidak mengubah perannya.
4. WHEN batch impor disetujui, THE system SHALL menerapkan peran dari CSV bersama field lain (maker-checker impor yang ada).
5. THE impor SHALL tetap menerima header CSV PIC lama tanpa kolom `roles` (diperlakukan sebagai kosong).

### Requirement 7: Notifikasi pihak per role, cabang, dan peran PIC

**User Story:** Sebagai user vendor atau PIC, saya ingin hanya menerima notifikasi tiket untuk peran dan cabang saya.

#### Acceptance Criteria

1. WHEN pihak dikirim, dikirim ulang, ditarik, atau dibatalkan, THE PartyNotifier SHALL mengirim in-app + email hanya ke user aktif vendor itu dengan role yang mengizinkan peran pihak dan (`vendor_branch_id` = cabang pihak ∨ `NULL`).
2. THE PartyNotifier SHALL mengirim email ke PIC aktif penerima notifikasi vendor itu (cabang pihak atau vendor-wide) hanya bila `roles` PIC memuat peran pihak.
3. THE PartyNotifier SHALL tidak mengirim email pihak ke PIC berdasarkan peran `cashcount`.
4. THE system SHALL tidak mengubah notifikasi keputusan realisasi (ke pelapor) dan notifikasi internal.

### Requirement 8: Guard portal dan tampilan VendorPortal

**User Story:** Sebagai user vendor, saya ingin bisa masuk dengan role baru dan melihat cakupan akun saya.

#### Acceptance Criteria

1. THE VendorPortalUI SHALL menerima ketiga role vendor saat login dan refresh token, dan menolak role lain.
2. THE CompanyPortalUI SHALL menolak ketiga role vendor saat login dan refresh token.
3. THE `GET /api/v1/auth/me` SHALL mengembalikan `vendor_name`, `vendor_branch_id`, `vendor_branch_name` (nullable) tanpa mengubah field yang sudah ada.
4. THE VendorPortalUI SHALL menampilkan cakupan akun di header/sidebar: "<vendor> · <cabang | Semua cabang> · <Pengisian ATM | Penyedia uang | Pengisian ATM + Penyedia uang>".
5. WHILE role = `VENDOR-VAULT-USER`, THE VendorPortalUI SHALL menyembunyikan menu Realisasi dan menampilkan "Tidak tersedia untuk peran Anda" bila `/realizations` dibuka langsung.
6. THE VendorPortalUI SHALL tidak menyampaikan status atau peran hanya lewat warna.

### Requirement 9: Kontrak API dan data dev

**User Story:** Sebagai engineer, saya ingin kontrak dan data dev diperbarui, agar frontend dan uji manual memakai bentuk yang benar.

#### Acceptance Criteria

1. THE `backend/api/openapi.yaml` SHALL memperbarui deskripsi role path `/vendor/replenish-orders*` menjadi ketiga role vendor, tanpa mengubah skema `VendorOrder*`.
2. THE `frontend/CompanyPortal-Vite/src/lib/api/schema.d.ts` SHALL di-regenerate (`gen:api`) bila `openapi.yaml` berubah.
3. THE `backend/scripts/dev/seed_vendor_users.sql` SHALL membuat akun sesuai D5 dan tetap idempoten.

### Requirement 10: Pengujian

**User Story:** Sebagai reviewer keamanan, saya ingin bukti otomatis bahwa cakupan tidak bocor sebelum fitur di-merge.

#### Acceptance Criteria

1. THE test suite SHALL memuat integration test Postgres nyata yang membuktikan: (a) `VENDOR-VAULT-USER` Medan tidak melihat pihak replenish Medan di request yang sama, (b) `VENDOR-ATM-USER` Medan tidak melihat pihak replenish cabang lain, (c) `VENDOR-USER` tetap melihat kedua peran, (d) token dengan role lama ditolak setelah role diganti, (e) notifikasi pihak hanya ke user dan PIC yang cocok, (f) cabang bukan milik vendor ditolak service dan FK, (g) gagal audit membatalkan perubahan peran PIC.
2. THE kode baru di `backend/internal/*` SHALL memiliki coverage ≥ 80%; frontend mengikuti threshold coverage yang ada (CP 84/80/80/85, VP 93/84/90/93).
