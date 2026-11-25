# Requirements Document

> Status: accepted 2026-11-25 (PO) — Introduction + keputusan D1–D11, Glossary + Requirements 1–9 ("Accept, go to design").

## Introduction

### Masalah

Pada 2026-11-25 user membuka Manajemen Peran (`/settings/roles`, kartu di hub Pengaturan) dan meminta:

> "saya ingin revamp … bagian 'Menejemen Peran', tolong buat spec:
> 1. ada symbol2 pada deskripsi.
> 2. saya ingin ada menu di manajemen peran untuk portal utama dan portal cabang.
> 3. revamp juga UI nya dengan UI/UX pro max.
> 4. perbaiki juga button 'kembali ke pengaturan' …"

Poin 4 dipisah menjadi Small Work (`.claude/small-works.md`, entri 2026-11-25), karena perbaikannya cukup satu baris di komponen bersama `BackLink` dan tidak hanya menyangkut Manajemen Peran. Spec ini mencakup poin 1–3. Untuk poin 2, user memperjelas (2026-11-25) bahwa "portal cabang" = **VendorPortal** (cabang vendor). Spec ini melanjutkan `.kiro/specs/.done/role-management/` dan `.kiro/specs/permission-catalog-coverage/`.

1. **Simbol aneh di deskripsi peran.** Deskripsi 9 peran bawaan (`ADMIN` … `VENDOR-USER`) tampil seperti `Super administrator â€” full system access`. Penyebabnya, seed `backend/migrations/002_baseline_seed.sql` menyimpan em dash (`—`) dengan encoding yang sudah rusak (UTF-8 terbaca sebagai Windows-1252 lalu disimpan ulang), sehingga teks rusak itu ikut masuk ke DB. Masalah yang sama ada di 3 baris `locations`. Peran dari migrasi berikutnya (`032`, `041`) tidak terkena.
2. **Manajemen Peran hanya mengatur CompanyPortal.** Katalog izin (`menu_features`) dan penegakannya (`permission-catalog-coverage`, migrasi `046`) hanya berlaku untuk CompanyPortal. VendorPortal dan role `VENDOR-*` sengaja dikeluarkan (D6 spec itu, PO 2026-11-20). Akibatnya, menu VendorPortal (Dashboard, DSR, Orders, Realisasi, Rekonsiliasi, Peramalan, Notifikasi, Admin) tidak bisa diatur per peran. User ingin dua bagian di Manajemen Peran: **Portal Utama** (CompanyPortal) dan **Portal Cabang** (VendorPortal).
3. **Tampilan layar.** Layar saat ini berupa tabel peran dan panel izin dalam grid dua kolom. Panel izin baru muncul setelah klik "Atur Izin", belum ada pencarian atau filter, deskripsi ditampilkan mentah, dan belum ada pembeda per portal. User ingin layar ini di-revamp dengan skill `ui-ux-pro-max` (brand Merah Sirih, CLAUDE.md §13).

### Hasil yang diharapkan

- Deskripsi peran tampil bersih tanpa karakter rusak, baik di Manajemen Peran maupun di semua layar lain yang membaca `roles.description`.
- Manajemen Peran punya pemisah yang jelas antara **Portal Utama** dan **Portal Cabang**. Izin menu VendorPortal bisa diatur per peran vendor dan, sesuai keputusan PO (Q3), benar-benar dipakai oleh VendorPortal.
- Layar Manajemen Peran lebih mudah dipakai dan konsisten dengan brand: punya 4 state UI (loading, kosong, error, berhasil), responsif, dan aksesibel.
- Akses peran yang sudah ada tidak berubah tepat setelah rilis, sehingga tidak ada user vendor yang tiba-tiba kehilangan atau mendapat menu.

### Kondisi kode saat ini (dicek 2026-11-25)

- Layar: `frontend/CompanyPortal-Vite/src/features/role-management/` (`RoleManagementPage.tsx`, `PermissionEditor.tsx`, `CreateRoleDialog.tsx`). Kolom Deskripsi menampilkan `row.original.description` apa adanya.
- Penjaga Manajemen Peran: `RequireRoles("APPACCESS","ADMIN")` statis (anti bootstrap deadlock) ditambah `Locked` key `settings.roles`. Spec ini tidak mengubah kebijakan tersebut.
- `menu_features` tidak punya kolom portal. Kolom `assignable_roles text[]` (migrasi `046`) membatasi role yang boleh diberi izin; NULL berarti semua role internal.
- VendorPortal punya route `dashboard`, `dsr`, `forecasting`, `orders` (+ `$id`), `realizations`, `reconciliation`, `notifications`, dan `admin`. Tidak ada kode yang membaca `role_permissions`. Endpoint vendor (`/api/v1/vendor/*`) tetap memakai guard role yang ada (D6 `permission-catalog-coverage`).
- Peran vendor: `VENDOR-USER`, `VENDOR-ATM-USER`, `VENDOR-VAULT-USER` (cakupan pihak replenish/vault, migrasi `032`).

### Batasan

- Stack tetap (CLAUDE.md §2), tanpa library baru. UI memakai `ui-ux-pro-max`; aturan brand §13 menang atas sarannya.
- Mengubah izin peran adalah deviasi GR#3 yang sudah terdokumentasi (Role Management): langsung berlaku, `audit_logs` ditulis dalam tx yang sama, dan `APPACCESS`/`ADMIN` dicek di route dan service. Spec ini tidak menambah deviasi baru.
- Flat JSON di `backend/internal/handler`. Endpoint baru atau yang berubah masuk `backend/api/openapi.yaml` bila modulnya sudah tercakup di sana.
- Uji browser manual tetap tugas user (Golden Rule #10).

### Area CLAUDE.md §4 #7 yang tersentuh

- **Data migration**: perbaikan teks `roles.description` (dan `locations`, jika ikut — lihat Q1). Dibuat sebagai migrasi baru `047_*`, bukan edit `002` (checksum).
- **Auth/RBAC**: katalog dan penegakan izin VendorPortal. Kemungkinan perlu kolom baru (portal) di `menu_features`, perubahan sidebar/route VendorPortal, dan mungkin endpoint `/api/v1/vendor/*`.
- Karena itu wajib memakai rantai Kiro bergerbang penuh, termasuk security review di tahap 5.

### Di luar cakupan (usulan, menunggu konfirmasi)

- Tombol "Kembali ke Pengaturan" (Small Work terpisah).
- Endpoint `/api/ext/*` dan role `EXT-CLIENT`.
- Mengganti guard statis Manajemen Peran itu sendiri.

### Keputusan PO

Q1 dan Q2 dipilih satu per satu. Q3–Q9 dijawab "setuju dengan semua rekomendasi" (2026-11-25), jadi isinya adalah rekomendasi AI yang disetujui PO.

- **D1 (Q1) ✅ DIPUTUSKAN PO 2026-11-25** — Perbaiki semua 12 baris rusak dari seed `002` (9 `roles.description` dan 3 `locations`) dalam satu migrasi `047`. Tidak ada validasi baru di form.
- **D2 (Q2) ✅ DIPUTUSKAN PO 2026-11-25** — Dua tab dalam satu halaman: `/settings/roles?portal=main|vendor`. Tetap satu kartu di hub Pengaturan; default tab Portal Utama.
- **D3 (Q3) ✅ DIPUTUSKAN PO 2026-11-25** — Penegakan penuh, sama seperti Portal Utama (`permission-catalog-coverage`): VendorPortal membaca "izin saya" untuk sidebar dan route, dan `RequirePermission` dipasang di `/api/v1/vendor/*` di depan guard role yang sudah ada (guard lama tidak dilepas). Ini mencabut D6 `permission-catalog-coverage` khusus untuk VendorPortal; `EXT-CLIENT` tetap di luar.
- **D4 (Q4) ✅ DIPUTUSKAN PO 2026-11-25** — Tab Portal Utama hanya menampilkan peran internal; tab Portal Cabang hanya peran vendor. Satu peran hanya milik satu portal dan hanya bisa diberi izin dari katalog portalnya sendiri.
- **D5 (Q5) ✅ DIPUTUSKAN PO 2026-11-25** — Katalog VendorPortal per menu (Dashboard, DSR, Orders, Realisasi, Rekonsiliasi, Peramalan, Notifikasi, Admin). Izin aksi tidak dimasukkan; aksi tetap diatur oleh aturan pihak vendor yang ada (migrasi `032`).
  - **Koreksi fakta (AI, 2026-11-25, saat menulis EARS — perlu dikonfirmasi PO di gerbang requirements):** daftar menu di atas salah. VendorPortal sebenarnya hanya punya 4 menu (`frontend/VendorPortal-Vite/src/lib/constants.ts` `NAV_ITEMS`): **DSR Menu** (`/dsr`), **CIT Orders** (`/orders`), **Realisasi** (`/realizations`), dan **Notifications** (`/notifications`). `/dashboard` hanya redirect ke `/orders`, sedangkan `/admin`, `/reconciliation`, dan `/forecasting` sengaja ditampilkan sebagai NotFound. Karena itu katalog Portal Cabang = 3 menu yang bisa diatur, dan Notifikasi selalu aktif (Requirement 3).
- **D6 (Q6) ✅ DIPUTUSKAN PO 2026-11-25** — Grant awal di migrasi = persis menu yang bisa dibuka tiap peran `VENDOR-*` saat ini, jadi akses tidak berubah saat rilis. Peran vendor baru mulai tanpa akses, sama seperti peran internal.
- **D7 (Q7) ✅ DIPUTUSKAN PO 2026-11-25** — Peran punya portal yang tersimpan eksplisit (usulan: kolom `roles.portal` = `main|vendor`, backfill dari prefix `VENDOR-`; nama dan bentuk final ditetapkan di design). Peran yang dibuat di tab Portal Cabang menjadi peran vendor dan hanya bisa di-assign ke user vendor (`auth_source=local`). Peran yang dibuat di tab Portal Utama tidak bisa di-assign ke user vendor.
- **D8 (Q8) ✅ DIPUTUSKAN PO 2026-11-25** — Isi revamp UI: master-detail (daftar peran di kiri, editor izin selalu terlihat di kanan, peran pertama terpilih otomatis), pencarian peran, jumlah user per peran, dan deskripsi tampil bersih. Perbandingan izin antar peran tidak dimasukkan. Mockup dibuat di tahap design (Artifact "Design").
- **D9 (Q9) ✅ DIPUTUSKAN PO 2026-11-25** — Pengelola Portal Cabang sama dengan Portal Utama: `APPACCESS`/`ADMIN`, dicek di route dan service. `ADMIN_PARAM` tidak termasuk.

### Pertanyaan terbuka (sudah dijawab, lihat Keputusan PO)

- **Q1 — Cakupan perbaikan karakter rusak.** Hanya `roles.description`, atau juga 3 baris `locations` yang terkena masalah yang sama? Perlukah pencegahan ke depan, misalnya validasi di form "Buat Peran" yang menolak karakter rusak?
- **Q2 — Bentuk "menu portal".** Dua tab dalam satu halaman (Portal Utama | Portal Cabang), atau dua sub-menu/route terpisah di hub Pengaturan?
- **Q3 — Penegakan di VendorPortal.** Apakah centang izin Portal Cabang harus benar-benar mengatur sidebar dan route VendorPortal? Ini butuh perubahan VendorPortal dan endpoint "izin saya" untuk vendor. Apakah juga perlu mengatur API `/api/v1/vendor/*` (`RequirePermission`)? Atau tahap pertama cukup katalog dan tampilan saja?
- **Q4 — Peran yang tampil di tiap portal.** Portal Utama berisi semua peran non-`VENDOR-*` dan Portal Cabang hanya `VENDOR-*`? Bolehkah satu peran punya izin di kedua portal?
- **Q5 — Granularitas katalog VendorPortal.** Cukup per menu (Dashboard, DSR, Orders, …), atau sampai level fitur/aksi (misalnya "Orders → terima/tolak", "Realisasi → kirim laporan")?
- **Q6 — Grant awal.** Setelah rilis, apakah semua peran `VENDOR-*` diberi izin persis sesuai menu yang bisa mereka buka sekarang, sehingga akses tidak berubah? Bagaimana dengan peran vendor baru yang dibuat dari layar: apakah mulai tanpa akses, seperti peran internal?
- **Q7 — "Buat Peran" per portal.** Jika peran baru dibuat di tab Portal Cabang, apakah otomatis menjadi peran vendor yang bisa di-assign ke user `auth_source=local`? Saat ini, apa pembeda peran vendor dan internal di DB selain prefix nama?
- **Q8 — Isi revamp UI.** Fitur mana yang diinginkan: pencarian/filter peran, editor izin yang selalu terlihat (master-detail), ringkasan jumlah user per peran, perbandingan izin antar peran, atau lainnya? Mockup dibuat di tahap design (Artifact "Design").
- **Q9 — Siapa yang boleh mengatur Portal Cabang.** Sama dengan Portal Utama (`APPACCESS`/`ADMIN`), atau `ADMIN_PARAM` juga boleh?

### Keputusan PO lanjutan

- **D10 (Q10) ✅ DIPUTUSKAN PO 2026-11-25** ("Yes, follow the proposal") — Saat membuat peran di tab Portal Cabang, admin wajib memilih Party_Scope (replenish / vault / keduanya). Pilihan disimpan di peran, dibaca oleh guard dan service, dan menggantikan `vendorrole.All`/`PartyRoles` yang ditulis tetap. Tiga peran yang ada di-backfill sesuai `PartyRoles` sekarang. Party_Scope tidak bisa diubah setelah peran dibuat (usulan AI, dikonfirmasi di gerbang requirements).
- **D11 (Q11) ✅ DIPUTUSKAN PO 2026-11-25** — Form user CompanyPortal membaca daftar peran beserta portalnya dari API, bukan dari daftar `VENDOR-*` yang ditulis tetap. `RoleBadge` memakai warna netral untuk peran yang tidak dikenal.

### Pertanyaan dari penulisan EARS (sudah dijawab, lihat D10–D11)

- **Q10 — Cakupan pihak untuk peran vendor baru.** Cakupan pihak vendor (replenish, vault, atau keduanya) saat ini ditulis tetap per nama peran di `backend/internal/vendorrole/vendorrole.go` (`All`, `PartyRoles`). Kode itu dipakai oleh guard route VendorPortal, `service/vendor_order.go`, `replenishment_handler.go`, dan query pihak vendor. Peran vendor baru dari layar (D7) tidak dikenal di sana, sehingga user dengan peran itu ditolak oleh semua route vendor. Usulan AI: saat membuat peran di tab Portal Cabang, admin wajib memilih cakupan pihak (replenish / vault / keduanya). Pilihan ini disimpan di peran (kolom baru; bentuknya diputuskan di design) dan dibaca oleh guard dan service, menggantikan daftar tetap. Tiga peran yang ada di-backfill sesuai `PartyRoles` sekarang. Alternatif: peran vendor baru selalu mendapat cakupan "keduanya", tanpa pilihan.
- **Q11 — Daftar role tetap di CompanyPortal.** `admin-users/lib/userFormSchema.ts`, `rbac-settings/types.ts`, `RoleBadge.tsx`, dan `lib/auth/store.ts` memakai daftar `VENDOR-*` yang ditulis tetap. Usulan AI: form user membaca daftar peran beserta portalnya dari API, sehingga peran vendor baru bisa dipilih untuk user vendor (D7). `RoleBadge` memakai warna netral untuk peran yang tidak dikenal.

## Glossary

- **Portal_Utama**: CompanyPortal (`frontend/CompanyPortal-Vite`), untuk user internal (LDAP).
- **Portal_Cabang**: VendorPortal (`frontend/VendorPortal-Vite`), untuk user vendor (`auth_source=local`).
- **Role_Portal**: portal pemilik sebuah peran, bernilai `main` atau `vendor` (D7). Satu peran tepat satu portal.
- **Vendor_Catalog**: baris `menu_features` milik Portal_Cabang untuk DSR Menu, CIT Orders, dan Realisasi (nama key final ditetapkan di design).
- **Always_On**: key katalog yang selalu lolos dan tidak bisa dicabut (pola `permission-catalog-coverage`). Untuk Portal_Cabang: Notifikasi.
- **Party_Scope**: cakupan pihak vendor sebuah peran: `replenish`, `vault`, atau keduanya (Q10).
- **Role_Management_Screen**: `/settings/roles` di Portal_Utama.

## Requirements

### Requirement 1: Perbaikan deskripsi rusak

**User Story:** Sebagai admin, saya ingin deskripsi peran dan lokasi tampil tanpa simbol aneh, agar mudah dibaca.

#### Acceptance Criteria

1. WHEN migrasi `047` dijalankan, THE database SHALL mengganti `â€”` dengan `—` pada 9 `roles.description` dan 3 `locations` yang terkena dari seed `002` (D1).
2. THE migrasi SHALL hanya mengubah baris yang memuat `â€”`, sehingga menjalankannya di DB yang sudah bersih tidak mengubah apa pun.
3. THE migrasi SHALL tidak mengubah file `002` (checksum di `schema_migrations` tetap).
4. WHEN Role_Management_Screen menampilkan deskripsi, THE screen SHALL menampilkan teks persis seperti di DB, tanpa transformasi di frontend.

### Requirement 2: Portal pemilik peran

**User Story:** Sebagai admin, saya ingin setiap peran jelas milik Portal Utama atau Portal Cabang, agar izin dan user yang diberikan tidak tercampur.

#### Acceptance Criteria

1. THE database SHALL menyimpan Role_Portal untuk setiap peran. Peran yang sudah ada di-backfill: `VENDOR-*` → `vendor`, sisanya → `main` (D7).
2. WHEN peran dibuat dari tab Portal_Utama, THE service SHALL menyimpan Role_Portal `main`; WHEN dibuat dari tab Portal_Cabang, THE service SHALL menyimpan `vendor`.
3. IF ada permintaan mengubah Role_Portal peran yang sudah ada, THEN THE service SHALL menolak dengan 422.
4. IF izin dari katalog portal lain diberikan ke sebuah peran, THEN THE service SHALL menolak dengan 422 `role_not_assignable` dan tidak menulis apa pun (D4).
5. IF user vendor (`auth_source=local`) diberi peran ber-Role_Portal `main`, atau user internal diberi peran `vendor`, THEN THE service pengelola user (admin users dan `backend-extapi`) SHALL menolak dengan 422 (D7).
6. THE `EXT-CLIENT` dan peran istimewa `pkg/userrules.PrivilegedRoles` SHALL tetap berperilaku seperti sekarang dan tidak tampil di tab Portal_Cabang.

### Requirement 3: Katalog izin Portal Cabang

**User Story:** Sebagai admin, saya ingin mengatur menu VendorPortal yang bisa dibuka setiap peran vendor.

#### Acceptance Criteria

1. THE `menu_features` SHALL memuat Vendor_Catalog untuk DSR Menu, CIT Orders, dan Realisasi dengan penanda portal `vendor`; baris yang sudah ada ditandai `main` (D5).
2. THE Notifikasi Portal_Cabang SHALL menjadi Always_On dan tidak tampil sebagai centang yang bisa dicabut.
3. WHEN migrasi dijalankan, THE database SHALL memberi grant awal yang sama dengan akses sekarang: `VENDOR-USER` dan `VENDOR-ATM-USER` → DSR, Orders, Realisasi; `VENDOR-VAULT-USER` → DSR, Orders (D6). Realisasi saat ini disembunyikan untuk peran vault di `AppShell.tsx`.
4. WHEN peran vendor baru dibuat, THE service SHALL membuatnya tanpa izin (D6).
5. THE pemberian dan pencabutan izin Portal_Cabang SHALL mengikuti deviasi GR#3 Role Management yang sudah ada: langsung berlaku, `audit_logs` (before/after) ditulis dalam tx yang sama, dan audit gagal = rollback.

### Requirement 4: Penegakan izin di VendorPortal

**User Story:** Sebagai admin, saya ingin izin yang dicentang benar-benar menentukan menu dan API yang bisa dipakai user vendor.

#### Acceptance Criteria

1. WHEN user vendor login, THE VendorPortal SHALL membaca izinnya dari `GET /api/v1/auth/me/permissions`, dan endpoint itu SHALL mengembalikan key Vendor_Catalog milik perannya. Ini menggantikan daftar kosong untuk `VENDOR-*` di `permission-catalog-coverage` Req 4.3; untuk `EXT-CLIENT` tetap kosong.
2. THE VendorPortal SHALL hanya menampilkan item sidebar untuk key yang dimiliki, ditambah Notifikasi (Always_On), dan aturan tetap `hideRealizations` dihapus.
3. IF user vendor membuka route yang key-nya tidak dimiliki, THEN THE VendorPortal SHALL mengarahkan ke menu pertama yang dimiliki, atau ke layar "tidak ada akses" bila tidak ada sama sekali.
4. IF pemanggil ber-Role_Portal `vendor` memanggil API milik menu yang key-nya tidak dimiliki, THEN THE backend SHALL membalas 403 (D3). Pemetaannya: `/api/v1/vendor/replenish-orders*` → Orders; `/api/v1/vendor/replenish-realizations*` dan `POST /api/v1/vendor/replenish-orders/{id}/realization` → Realisasi; `/api/v1/dsr*` → DSR.
5. THE pengecekan izin Portal_Cabang SHALL dipasang di depan guard role yang ada, dan guard lama tetap dipertahankan: katalog hanya bisa mempersempit akses, tidak memperluas (D3).
6. WHILE pemanggil ber-Role_Portal `main`, THE pengecekan izin Portal_Cabang di `/api/v1/dsr*` SHALL tidak berlaku, sehingga akses internal ke endpoint itu tidak berubah.
7. IF izin sebuah peran dicabut, THEN THE backend SHALL menolak panggilan berikutnya dari user dengan peran itu tanpa perlu login ulang, karena izin dibaca per request seperti di Portal_Utama.

### Requirement 5: Tab portal di Manajemen Peran

**User Story:** Sebagai admin, saya ingin memisahkan pengaturan peran Portal Utama dan Portal Cabang dalam satu halaman.

#### Acceptance Criteria

1. THE Role_Management_Screen SHALL menampilkan dua tab, "Portal Utama" dan "Portal Cabang", yang tersimpan di URL `?portal=main|vendor`. Tanpa parameter atau dengan nilai tidak valid, tab yang terbuka adalah Portal Utama (D2).
2. WHEN sebuah tab dipilih, THE screen SHALL hanya menampilkan peran dengan Role_Portal itu dan katalog izin portal itu (D4).
3. WHEN "Buat Peran" dipakai di sebuah tab, THE dialog SHALL membuat peran untuk portal tab tersebut dan menyebut portalnya di judul dialog.
4. THE akses ke screen dan API-nya SHALL tetap `APPACCESS`/`ADMIN`, dicek di route dan service untuk kedua tab; `ADMIN_PARAM` ditolak (D9).

### Requirement 6: Revamp UI Manajemen Peran

**User Story:** Sebagai admin, saya ingin layar Manajemen Peran cepat dipakai dan rapi.

#### Acceptance Criteria

1. THE screen SHALL memakai tata letak master-detail: daftar peran di kiri dan editor izin selalu terlihat di kanan. Peran pertama terpilih otomatis, dan pilihan tersimpan di URL (D8).
2. THE daftar peran SHALL bisa dicari berdasarkan nama atau deskripsi, tanpa membedakan huruf besar/kecil, di sisi klien.
3. THE daftar peran SHALL menampilkan jumlah user aktif per peran. Jumlah ini dihitung di backend (replica) dan dikembalikan oleh API daftar peran.
4. THE screen SHALL punya 4 state: loading, kosong, error dengan tombol coba lagi, dan berhasil, termasuk tampilan "tidak ada hasil" untuk pencarian.
5. THE screen SHALL memenuhi brand Merah Sirih (CLAUDE.md §13, `rules/frontend-ui.md`): bisa dipakai dengan keyboard, fokus terlihat, target sentuh ≥ 44 px, status tidak hanya dibedakan dengan warna, dan responsif (di layar sempit daftar dan editor ditumpuk).
6. THE desain SHALL dibuat dengan skill `ui-ux-pro-max`. ~~Mockup di Artifact "Design" sebelum kode~~ — dikesampingkan user 2026-11-25 ("tidak perlu mockup"); layout di `design.md` jadi acuan.

### Requirement 7: Cakupan pihak peran vendor

**User Story:** Sebagai admin, saya ingin peran vendor baru langsung bisa bekerja dengan cakupan pihak yang saya pilih, tanpa perubahan kode.

#### Acceptance Criteria

1. WHEN peran dibuat di tab Portal_Cabang, THE dialog SHALL mewajibkan pilihan Party_Scope (replenish / vault / keduanya), dan THE service SHALL menolak tanpa Party_Scope dengan 422 (D10).
2. THE database SHALL menyimpan Party_Scope per peran vendor dan backfill peran yang ada: `VENDOR-USER` → keduanya, `VENDOR-ATM-USER` → replenish, `VENDOR-VAULT-USER` → vault (sama dengan `PartyRoles` sekarang).
3. IF ada permintaan Party_Scope untuk peran ber-Role_Portal `main`, atau permintaan mengubah Party_Scope peran yang sudah ada, THEN THE service SHALL menolak dengan 422.
4. THE guard route VendorPortal dan service pihak vendor (`vendor_order`, `replenishment_handler`, query pihak vendor) SHALL menentukan "peran vendor" dari Role_Portal dan pihak yang boleh dilihat atau diputuskan dari Party_Scope, menggantikan daftar tetap di `vendorrole.go`.
5. WHEN user dengan peran vendor baru (Party_Scope vault) membuka Orders, THE backend SHALL memperlakukannya sama persis dengan `VENDOR-VAULT-USER` sekarang (setara perilaku).
6. THE tampilan cakupan pihak di VendorPortal (`partyScopeLabel`) SHALL membaca Party_Scope, bukan nama peran.

### Requirement 8: Daftar peran dinamis di CompanyPortal

**User Story:** Sebagai admin user, saya ingin peran vendor baru muncul di form user tanpa perubahan kode.

#### Acceptance Criteria

1. THE form tambah/edit user SHALL mengambil daftar peran beserta Role_Portal dari API dan hanya menawarkan peran `vendor` untuk user vendor dan peran `main` untuk user internal (D11, Req 2.5).
2. THE daftar `VENDOR-*` yang ditulis tetap di `admin-users/lib/userFormSchema.ts`, `rbac-settings/types.ts`, dan `lib/auth/store.ts` SHALL diganti dengan data dari API atau Role_Portal di token/`/me`.
3. WHEN `RoleBadge` menerima peran yang tidak dikenal, THE badge SHALL tampil dengan warna netral dan tetap menampilkan nama perannya.

### Requirement 9: Kontrak dan pengujian

**User Story:** Sebagai engineer, saya ingin perubahan ini terkunci oleh kontrak dan tes.

#### Acceptance Criteria

1. WHEN endpoint di modul yang sudah tercakup di `backend/api/openapi.yaml` berubah, THE spec SHALL diperbarui dan lolos tes `-run OpenAPI`, lalu `gen:api` dijalankan ulang.
2. THE tes SHALL mencakup: migrasi idempoten (Req 1); penolakan lintas portal (Req 2.4, 2.5); grant awal = akses sekarang (Req 3.3); 403 per menu vendor tanpa perubahan untuk pemanggil internal (Req 4.4, 4.6); penolakan `ADMIN_PARAM` (Req 5.4); Party_Scope wajib dan setara perilaku dengan peran lama (Req 7.1, 7.5); form user hanya menawarkan peran dari portal yang cocok (Req 8.1); serta komponen tab dan pencarian di kedua frontend.
3. THE uji browser manual SHALL dicatat sebagai "outstanding (user)" (Golden Rule #10).
