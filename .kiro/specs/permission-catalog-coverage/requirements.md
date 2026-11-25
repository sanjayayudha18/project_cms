# Requirements Document

> Status: accepted 2026-11-20 (PO) — Introduction ("Terima, jawab Q1–Q11"), keputusan D1–D11, Glossary + Requirements 1–9 ("Terima, lanjut design"); AC 3.6 ditambahkan dan diterima 2026-11-20 (DQ1, "Terima semua").

## Introduction

### Masalah

Pada 2026-11-20 user membuka Manajemen Peran (`http://localhost:3001/settings/roles`), melihat panel **"Izin — ADMIN"**, lalu bertanya: *"cek pada manajemen role apakah semua fitur sudah masuk ke 'pengaturan izin akses'"*. Hasil pengecekan: **belum**. Ada dua masalah:

1. **Katalog izin tidak lengkap.** Daftar izin berasal dari tabel `menu_features`, yang hanya berisi 9 menu dan 9 sub-fitur. Lebih dari 20 layar CompanyPortal tidak punya baris di sana, sehingga izinnya tidak bisa diatur per role.
2. **Izin yang dicentang belum dipakai.** Sidebar dan route frontend masih memakai daftar role hardcoded, dan semua route backend masih memakai `RequireRoles(...)` statis. Mencentang atau menghapus centang izin tidak mengubah apa yang bisa dilihat atau diakses user.

Spec asli `role-management` (`.kiro/specs/.done/role-management/`) sebenarnya sudah menjanjikan katalog yang *"dibaca saat runtime oleh middleware backend dan navigasi frontend"* (requirements.md baris 9) dan menyiapkan `Dynamic_Route`. Namun belum ada satu pun route yang memakainya.

### Hasil yang diharapkan

- Setiap layar atau menu CompanyPortal yang bisa dibuka user muncul di "Pengaturan Izin Akses", dengan pengelompokan menu → fitur yang sama seperti di sidebar.
- Izin yang dicentang benar-benar menentukan apa yang tampil di sidebar dan route yang bisa dibuka, serta (sesuai keputusan PO, lihat Q3) API yang boleh dipanggil.
- Role yang sudah ada tetap memiliki akses yang sama seperti sekarang tepat setelah rilis, sehingga tidak ada user yang tiba-tiba kehilangan atau mendapat akses.

### Kondisi kode saat ini (dicek 2026-11-20)

**Katalog (`menu_features`, migrasi `002`, `027`, `041`, `044`)**
- Menu (`kind='menu'`): `dashboard`, `cash-flow`, `monitoring`, `forecasting`, `replenish`, `invoice`, `cash-count`, `settings`, `cit`.
- Fitur (`kind='feature'`): `settings.roles` (+ `settings.roles.create`, `settings.roles.edit`), `settings.acm-areas`, `settings.pks`, `cash-count.parameters`, `cash-count.pic-mapping`, `cit.vault-plan`.
- Constraint: `menu` tanpa parent, `feature` wajib punya parent; `key` unik. Grant = baris di `role_permissions`. Role baru mulai tanpa akses.

**Layar CompanyPortal tanpa baris katalog** (`frontend/CompanyPortal-Vite/src/routes/`)
- Umum: `/approvals` (Persetujuan), `/exports` (Ekspor Saya).
- Replenish: `/replenishment/forecast-browser` (Rekomendasi CIT), `/replenishment/vendor-requests` (+ `new`, `$id`) (Request CIT), `/replenishment` (Jadwal CIT).
- Monitoring: `/atm-portal` (+ `$terminalId`), `/dsr-late-report`, `/eod-monitoring`, `/audit-logs`.
- Peramalan: `/forecasting/dmaa-forecast`, `/settings/admin/holidays` (Kalender Libur, di sidebar grup Peramalan).
- Pengaturan: `admin/users` (+ `users-new`), `admin/vendors` (+ detail vendor, detail/edit cabang), `admin/atms`, `admin/regions`, `admin/pks-types`, `admin/master-data-io`, `admin/exports`, `rbac/users`, `rbac/policies`, `rbac/delegations`, `rbac/leaves`.
- Menu `cash-flow`, `monitoring`, `forecasting`, `replenish`, `invoice`, `dashboard` tidak punya fitur anak sama sekali.

**Penegakan saat ini**
- Frontend: `src/lib/config/navigation.ts` `NAV_CONFIG[].roles` + `filterNavByRoles` (ADMIN/ADMIN_PARAM melihat semua); route memakai `requireRoles([...])` di `beforeLoad`. Tidak ada kode frontend yang membaca `role_permissions`.
- Backend: `rolemgmt.RequirePermission(evaluator, featureKey)` (`backend/internal/rolemgmt/evaluator.go:45`) sudah ada dan dites, tetapi **tidak dipasang di route mana pun**. Semua grup route memakai `custommw.RequireRoles(...)`, dan banyak service mengecek role lagi di service layer (CLAUDE.md §5 RBAC).
- Manajemen Peran sendiri sengaja dijaga `RequireRoles("APPACCESS","ADMIN")` statis untuk menghindari bootstrap deadlock (komentar `backend/cmd/api/main.go` ±baris 188).
- Guard route frontend `requireRoles` (`routes/_protected.tsx:69`) melewatkan ADMIN/ADMIN_PARAM dan, untuk role lain yang tidak cocok, hanya mengembalikan `{ forbidden: true }`. Nilai itu tidak dibaca di mana pun, jadi halaman tetap tampil (dicatat juga di komentar `routes/replenishment/vendor-requests/new.tsx`). Saat ini yang benar-benar menahan akses hanyalah backend.
- `backend-exq` (ekspor, port 8081) memvalidasi token sendiri dengan aturan role sendiri (`pkg/reports` viewer roles).
- VendorPortal (role `VENDOR-*`) adalah SPA terpisah; katalog saat ini hanya untuk CompanyPortal.

### Ruang lingkup (usulan, menunggu PO)

- Menambah baris `menu_features` (migrasi baru, nomor bebas `046`) untuk semua layar CompanyPortal di atas, dengan grant awal `role_permissions` yang meniru akses role sekarang.
- Endpoint "izin saya" (atau menambah izin ke respons `/me`) agar frontend tahu key yang dimiliki user.
- Sidebar, kartu hub Pengaturan, dan guard route CompanyPortal membaca izin dari katalog.
- Penegakan backend lewat `RequirePermission`, dengan cakupan sesuai keputusan Q3.

### Tidak termasuk (usulan)

- VendorPortal dan role `VENDOR-*` (lihat Q6).
- Izin per aksi (tombol buat/ubah/hapus/approve) di luar yang sudah ada (`settings.roles.create/edit`), lihat Q2.
- Mengubah aturan maker-checker (maker ≠ checker tetap ditegakkan di service).
- Cache Redis untuk evaluasi izin (keputusan desain role-management: tanpa cache sampai ada bukti perlu).

### Area Sec 4 #7

- **Auth/RBAC**: mengubah sumber kebenaran akses dari kode ke data. Salah grant berarti kebocoran akses atau user terkunci.
- **Migrasi data**: baris katalog baru + backfill `role_permissions`.
- Butuh rantai bergate penuh, test RBAC denial per route, dan `/security-review` di tahap 5.

### Keputusan PO (2026-11-20)

Q1–Q3 dijawab satu per satu; Q4–Q11 dijawab "setuju dengan semua saran" (= rekomendasi Claude di bawah).

- **D1 (Q1) ✅ DIPUTUSKAN PO 2026-11-20** — Satu baris katalog per item sidebar / kartu hub Pengaturan. Sub-halaman (`new`, `$id`, detail/edit) ikut izin layar induknya.
- **D2 (Q2) ✅ DIPUTUSKAN PO 2026-11-20** — Cukup izin "buka layar". Tidak ada izin aksi baru; `settings.roles.create/edit` yang sudah ada tetap.
- **D3 (Q3) ✅ DIPUTUSKAN PO 2026-11-20** — Frontend **dan** backend: endpoint milik sebuah layar dijaga `RequirePermission` dengan key layar yang sama.
- **D4 (Q4) ✅ DIPUTUSKAN PO 2026-11-20** — Aturan bisnis di service (siapa boleh approve, maker ≠ checker, cakupan vendor/cabang) tetap berbasis role. Katalog hanya mengatur "boleh membuka layar/endpoint".
- **D5 (Q5) ✅ DIPUTUSKAN PO 2026-11-20** — ADMIN dan ADMIN_PARAM ikut katalog seperti role lain (tidak ada bypass otomatis), supaya centang di panel "Izin — ADMIN" benar-benar berlaku. Grant awal = semua layar yang sekarang bisa mereka buka (D9).
- **D6 (Q6) ✅ DIPUTUSKAN PO 2026-11-20** — VendorPortal dan role `VENDOR-*` di luar spec ini.
- **D7 (Q7) ✅ DIPUTUSKAN PO 2026-11-20** — Dashboard dan Persetujuan selalu terbuka untuk semua role internal (Persetujuan = kotak masuk maker-checker). Di daftar izin keduanya tampil sebagai "selalu aktif" dan tidak bisa dicabut.
- **D8 (Q8) ✅ DIPUTUSKAN PO 2026-11-20** — Manajemen Peran (`settings.roles*`) dan Pengguna (`admin/users`) tetap dijaga `RequireRoles("APPACCESS","ADMIN")` statis untuk mencegah terkunci (bootstrap). Di daftar izin keduanya tampil terkunci untuk APPACCESS/ADMIN.
- **D9 (Q9) ✅ DIPUTUSKAN PO 2026-11-20** — Backfill `role_permissions` meniru akses sekarang (gabungan `NAV_CONFIG.roles`, `requireRoles` route, dan `RequireRoles` backend). Akses tiap role tidak berubah saat rilis.
- **D10 (Q10) ✅ DIPUTUSKAN PO 2026-11-20** — `backend-exq` tetap memakai viewer role `pkg/reports`; dipindah ke katalog di spec lanjutan bila perlu.
- **D11 (Q11) ✅ DIPUTUSKAN PO 2026-11-20** — Menu yang masih `disabled` (Unggah Invoice, Penjadwalan, Pelaksanaan BA, Rekapitulasi) tidak didaftarkan sekarang; didaftarkan oleh spec yang membangun fiturnya.

### Pertanyaan terbuka (dijawab, lihat Keputusan PO)

- **Q1 — Granularitas.** Satu baris katalog per **layar/item sidebar** (mis. `replenish.vendor-requests`), atau per halaman termasuk sub-halaman (mis. `replenish.vendor-requests.new`, `.detail`)?
- **Q2 — Izin aksi.** Cukup izin "lihat/buka layar", atau perlu izin aksi terpisah (buat, ubah, approve, ekspor) seperti `settings.roles.create/edit`?
- **Q3 — Cakupan backend.** (a) Katalog hanya mengatur tampilan frontend (sidebar + route), backend tetap `RequireRoles`; (b) backend juga pindah ke `RequirePermission` untuk semua route CompanyPortal; (c) bertahap: frontend dulu, backend per modul di spec lanjutan?
- **Q4 — Role vs izin di service layer.** Banyak service mengecek role secara langsung (mis. checker harus `ATM-SPV`). Apakah pengecekan itu tetap berbasis role (aturan bisnis), dan hanya "boleh membuka layar/endpoint" yang pindah ke katalog?
- **Q5 — Hak istimewa ADMIN/ADMIN_PARAM.** Sekarang keduanya melihat semua menu otomatis. Tetap otomatis (bypass katalog) atau ikut katalog seperti role lain?
- **Q6 — VendorPortal.** Ikut masuk katalog (role `VENDOR-*`) atau tetap di luar?
- **Q7 — Layar khusus.** Dashboard dan Persetujuan saat ini terbuka untuk semua role (`"*"`). Tetap selalu terbuka, atau bisa dicabut lewat katalog?
- **Q8 — Bootstrap.** Manajemen Peran tetap dijaga `APPACCESS`/`ADMIN` statis (tidak bisa dicabut lewat katalog)? Berlaku juga untuk `admin/users`?
- **Q9 — Grant awal.** Backfill `role_permissions` dibuat persis sama dengan `NAV_CONFIG.roles` + `requireRoles` sekarang, atau PO ingin menetapkan matriks role × layar baru?
- **Q10 — `backend-exq`.** Ekspor laporan memakai daftar viewer role di `pkg/reports`. Ikut katalog atau tetap?
- **Q11 — Menu disabled.** Item yang sekarang `disabled: true` (Unggah Invoice, Penjadwalan, Pelaksanaan BA, Rekapitulasi) ikut didaftarkan sekarang atau menunggu fiturnya dibangun?

## Glossary

- **Permission_Catalog**: tabel `menu_features`; satu baris per menu atau layar (D1).
- **Screen_Key**: nilai `menu_features.key` milik satu layar, mis. `monitoring.audit-logs`. Menu tanpa fitur anak (mis. `cash-flow`) adalah layar itu sendiri.
- **Role_Permission_Mapping**: tabel `role_permissions`; baris ada = role punya izin.
- **Always_On_Key**: Screen_Key yang selalu diizinkan untuk semua role internal dan tidak bisa dicabut: `dashboard`, `approvals` (D7).
- **Locked_Key**: Screen_Key yang aksesnya tetap dijaga role statis `APPACCESS`/`ADMIN` dan tidak bisa diubah lewat katalog: `settings.roles`, `settings.roles.create`, `settings.roles.edit`, `settings.users` (D8).
- **Effective_Permissions**: himpunan Screen_Key yang dimiliki user = baris Role_Permission_Mapping untuk role-nya ∪ Always_On_Key ∪ Locked_Key bila role-nya `APPACCESS` atau `ADMIN`.
- **My_Permissions_API**: endpoint backend ATM yang mengembalikan Effective_Permissions user yang sedang login.
- **Permission_Middleware**: `rolemgmt.RequirePermission` di backend ATM (sudah ada, belum dipasang).
- **Role_Permission_API**: endpoint Manajemen Peran yang menyimpan izin role (`PUT /api/v1/admin/roles/{id}/permissions`, sudah ada).
- **Nav_Renderer**: sidebar, command palette, dan kartu hub Pengaturan di CompanyPortal.
- **Route_Guard**: `beforeLoad` route CompanyPortal yang menentukan boleh tidaknya halaman dibuka.
- **Permission_Editor**: panel "Pengaturan Izin Akses" di `/settings/roles`.
- **Catalog_Migration**: migrasi `046` yang menambah baris katalog dan grant awal.

## Requirements

### Requirement 1: Katalog memuat semua layar CompanyPortal

**User Story:** Sebagai APPACCESS/ADMIN, saya ingin setiap layar CompanyPortal ada di daftar izin, supaya saya bisa mengatur akses setiap role ke setiap layar.

#### Acceptance Criteria

1. THE Permission_Catalog SHALL berisi satu baris untuk setiap layar di tabel berikut, dengan parent dan label seperti tertulis (key final ditetapkan di design):

   | Menu (parent) | Screen_Key usulan | Label | Route (sub-halaman ikut induk) |
   |---|---|---|---|
   | — (menu baru) | `approvals` | Persetujuan | `/approvals` |
   | — (menu baru) | `exports` | Ekspor | `/exports`, `/exports/$dataset` |
   | `replenish` | `replenish.forecast-browser` | Rekomendasi CIT | `/replenishment/forecast-browser` |
   | `replenish` | `replenish.vendor-requests` | Request CIT | `/replenishment/vendor-requests`, `/new`, `/$id` |
   | `replenish` | `replenish.schedule` | Jadwal CIT | `/replenishment` |
   | `monitoring` | `monitoring.atm-portal` | ATM Portal | `/atm-portal`, `/atm-portal/$terminalId` |
   | `monitoring` | `monitoring.dsr-late-report` | Laporan DSR Telat | `/dsr-late-report` |
   | `monitoring` | `monitoring.eod-monitoring` | EOD Monitoring | `/eod-monitoring` |
   | `monitoring` | `monitoring.audit-logs` | Log Audit | `/audit-logs` |
   | `forecasting` | `forecasting.dmaa-forecast` | DMAA Forecast | `/forecasting/dmaa-forecast` |
   | `forecasting` | `forecasting.holidays` | Kalender Libur | `/settings/admin/holidays` |
   | `settings` | `settings.users` | Pengguna | `/settings/admin/users`, `/users/new` |
   | `settings` | `settings.vendors` | Vendor | `/settings/admin/vendors` + detail vendor + detail/edit cabang |
   | `settings` | `settings.atms` | ATM | `/settings/admin/atms` |
   | `settings` | `settings.regions` | Region | `/settings/admin/regions` |
   | `settings` | `settings.pks-types` | Type PKS | `/settings/admin/pks-types` |
   | `settings` | `settings.master-data-io` | Impor/Ekspor Data Master | `/settings/admin/master-data-io` |
   | `settings` | `settings.rbac-users` | Pengguna RBAC | `/settings/rbac/users` |
   | `settings` | `settings.rbac-policies` | Kebijakan RBAC | `/settings/rbac/policies` |
   | `settings` | `settings.rbac-delegations` | Delegasi | `/settings/rbac/delegations` |
   | `settings` | `settings.rbac-leaves` | Cuti | `/settings/rbac/leaves` |

2. THE Permission_Catalog SHALL tetap memuat baris yang sudah ada (`dashboard`, `cash-flow`, `monitoring`, `forecasting`, `replenish`, `invoice`, `cash-count`, `settings`, `cit`, `settings.roles*`, `settings.acm-areas`, `settings.pks`, `cash-count.parameters`, `cash-count.pic-mapping`, `cit.vault-plan`) tanpa mengubah `key`-nya.
3. THE Permission_Catalog SHALL tidak memuat baris untuk item yang masih `disabled` di `NAV_CONFIG` (D11).
4. WHEN sebuah route CompanyPortal baru ditambahkan di kemudian hari, THE Route_Guard SHALL mewajibkan route itu menyebut Screen_Key yang ada di Permission_Catalog (ditegakkan oleh test, lihat Requirement 9).

### Requirement 2: Grant awal meniru akses saat ini

**User Story:** Sebagai PO, saya ingin akses setiap role tidak berubah saat rilis, supaya tidak ada user yang tiba-tiba kehilangan atau mendapat akses.

#### Acceptance Criteria

1. WHEN Catalog_Migration dijalankan, THE Catalog_Migration SHALL memberi setiap role izin ke tepat layar yang sekarang bisa dibuka role itu, yaitu role yang lolos guard backend endpoint layar tersebut (D9). Matriks role × layar ditetapkan di design.
2. WHEN Catalog_Migration dijalankan, THE Catalog_Migration SHALL memberi ADMIN dan ADMIN_PARAM izin ke semua layar yang sekarang bisa mereka buka, karena bypass otomatis dihapus (D5).
3. THE Catalog_Migration SHALL bersifat additive, berjalan dalam satu transaksi, dan aman dijalankan ulang (`ON CONFLICT DO NOTHING`).
4. IF Catalog_Migration gagal di tengah jalan, THEN THE Catalog_Migration SHALL membatalkan seluruh perubahan.
5. THE Catalog_Migration SHALL tidak menghapus atau mengubah baris Role_Permission_Mapping yang sudah ada.

### Requirement 3: Layar selalu aktif dan layar terkunci

**User Story:** Sebagai APPACCESS/ADMIN, saya ingin layar inti tidak bisa dicabut tanpa sengaja dan Manajemen Peran tidak bisa terkunci, supaya sistem selalu bisa dikelola.

#### Acceptance Criteria

1. THE Permission_Middleware SHALL selalu mengizinkan Always_On_Key untuk semua role internal, tanpa membaca Role_Permission_Mapping (D7).
2. THE Permission_Editor SHALL menampilkan Always_On_Key dengan tanda "selalu aktif" dan kotak centang yang tidak bisa diubah.
3. THE backend ATM SHALL tetap menjaga endpoint Manajemen Peran dan `/api/v1/admin/users` dengan `RequireRoles("APPACCESS","ADMIN")` statis (D8).
4. THE Permission_Editor SHALL menampilkan Locked_Key dengan tanda "hanya APPACCESS/ADMIN" dan kotak centang yang tidak bisa diubah.
5. IF permintaan ubah izin role berisi Always_On_Key atau Locked_Key, THEN THE Role_Permission_API SHALL mengabaikan key tersebut tanpa menyimpannya, dan mengembalikan hasil yang sama dengan permintaan tanpa key itu.
6. IF permintaan ubah izin role memberi Screen_Key ke role di luar `menu_features.assignable_roles` key itu, THEN THE Role_Permission_API SHALL menolak dengan 422 `role_not_assignable`, dan THE Permission_Editor SHALL menampilkan kotak centang itu nonaktif dengan daftar role yang diizinkan. *(Ditambahkan 2026-11-20 dari design DQ1, diterima PO "Terima semua".)*

### Requirement 4: Endpoint izin saya

**User Story:** Sebagai user CompanyPortal, saya ingin aplikasi tahu layar apa saja yang boleh saya buka, supaya menu dan halaman menyesuaikan izin role saya.

#### Acceptance Criteria

1. WHEN user internal yang login memanggil My_Permissions_API, THE My_Permissions_API SHALL mengembalikan Effective_Permissions role user itu sebagai daftar Screen_Key, dalam format flat JSON seperti handler ATM lain.
2. IF pemanggil tidak membawa token valid, THEN THE My_Permissions_API SHALL membalas 401.
3. IF pemanggil adalah role `VENDOR-*` atau `EXT-CLIENT`, THEN THE My_Permissions_API SHALL membalas daftar kosong (D6).
4. WHEN izin role diubah di Permission_Editor, THE My_Permissions_API SHALL mengembalikan izin baru pada panggilan berikutnya tanpa restart server.

### Requirement 5: Sidebar dan hub Pengaturan mengikuti izin

**User Story:** Sebagai user, saya ingin hanya melihat menu yang boleh saya buka, supaya tidak ada menu yang berakhir di halaman ditolak.

#### Acceptance Criteria

1. THE Nav_Renderer SHALL menampilkan sebuah item sidebar, item command palette, atau kartu hub Pengaturan hanya jika Screen_Key item itu ada di Effective_Permissions user.
2. WHEN sebuah grup sidebar atau tab hub Pengaturan tidak berisi item yang terlihat, THE Nav_Renderer SHALL menyembunyikan grup atau tab itu.
3. THE Nav_Renderer SHALL tidak lagi memakai daftar `roles` di `NAV_CONFIG` maupun bypass ADMIN/ADMIN_PARAM untuk menentukan item yang terlihat (D5).
4. WHILE Effective_Permissions belum selesai dimuat, THE Nav_Renderer SHALL tidak menampilkan item yang belum pasti diizinkan.
5. IF My_Permissions_API gagal dimuat, THEN THE Nav_Renderer SHALL hanya menampilkan Always_On_Key dan pesan bahwa menu gagal dimuat, dengan aksi coba lagi.

### Requirement 6: Route frontend ditolak bila tanpa izin

**User Story:** Sebagai user, saya ingin halaman yang tidak boleh saya buka benar-benar ditolak, supaya akses tidak hanya bergantung pada sidebar.

#### Acceptance Criteria

1. WHEN user membuka route yang Screen_Key-nya tidak ada di Effective_Permissions, THE Route_Guard SHALL menampilkan halaman 403 dan tidak me-render isi halaman.
2. THE Route_Guard SHALL memakai Screen_Key layar induk untuk sub-halaman (`new`, `$id`, detail, edit) (D1).
3. WHERE sebuah sub-halaman sekarang dibatasi ke role yang lebih sempit daripada layar induknya (mis. `/exports/$dataset` hanya ADMIN/ADMIN_PARAM), THE Route_Guard SHALL tetap memakai batasan role sempit itu selain Screen_Key induk (D4).
4. IF user belum login, THEN THE Route_Guard SHALL mengarahkan ke `/login` seperti sekarang.
5. THE Route_Guard SHALL tidak lagi memakai bypass ADMIN/ADMIN_PARAM (D5).

### Requirement 7: Backend menegakkan izin layar

**User Story:** Sebagai PO, saya ingin izin yang dicabut juga menolak panggilan API langsung, supaya katalog benar-benar menjadi batas akses (D3).

#### Acceptance Criteria

1. WHEN endpoint backend ATM yang dipakai sebuah layar dipanggil, THE Permission_Middleware SHALL mengizinkan permintaan hanya jika role pemanggil punya Screen_Key layar tersebut. Pemetaan endpoint → Screen_Key ditetapkan di design.
2. WHERE satu endpoint dipakai lebih dari satu layar, THE Permission_Middleware SHALL mengizinkan permintaan jika role pemanggil punya salah satu Screen_Key layar-layar itu.
3. IF role pemanggil tidak punya Screen_Key yang diperlukan, THEN THE Permission_Middleware SHALL membalas 403 dengan bentuk respons error yang sama seperti `RequireRoles` sekarang.
4. IF pembacaan Role_Permission_Mapping gagal, THEN THE Permission_Middleware SHALL menolak permintaan (fail closed) dan mencatat error di log server tanpa data rahasia.
5. THE backend ATM SHALL tetap menjalankan pengecekan role di service layer (approve, maker ≠ checker, cakupan vendor/cabang) tanpa perubahan (D4).
6. THE backend ATM SHALL tetap memakai guard yang ada untuk endpoint VendorPortal (`/api/v1/vendor/*`) dan `/api/ext/*` (D6).
7. THE `backend-exq` SHALL tetap memakai aturan viewer role `pkg/reports` (D10).

### Requirement 8: Perubahan izin langsung berlaku

**User Story:** Sebagai APPACCESS/ADMIN, saya ingin perubahan izin berlaku tanpa menunggu deploy, supaya pengaturan akses bisa dipakai sehari-hari.

#### Acceptance Criteria

1. WHEN izin role disimpan, THE Permission_Middleware SHALL memakai izin baru pada permintaan berikutnya tanpa restart.
2. WHEN user yang sedang login memuat ulang aplikasi atau login kembali, THE Nav_Renderer SHALL memakai izin terbaru.
3. WHEN APPACCESS/ADMIN menyimpan izin role miliknya sendiri, THE Nav_Renderer SHALL memuat ulang Effective_Permissions user itu tanpa perlu reload manual.
4. THE Role_Permission_API SHALL tetap menulis `audit_logs` dalam transaksi yang sama untuk setiap perubahan izin, seperti sekarang.

### Requirement 9: Pengaman agar katalog tidak tertinggal lagi

**User Story:** Sebagai engineer, saya ingin test gagal bila ada layar atau endpoint tanpa Screen_Key, supaya masalah ini tidak terulang saat fitur baru dibangun.

#### Acceptance Criteria

1. THE CompanyPortal test suite SHALL gagal bila ada route terlindungi atau item `NAV_CONFIG` aktif yang tidak menyebut Screen_Key, atau menyebut key yang tidak ada di daftar key katalog.
2. THE backend ATM test suite SHALL gagal bila ada route di grup yang sudah dipindah ke Permission_Middleware yang tidak punya pemetaan Screen_Key.
3. THE backend ATM test suite SHALL memverifikasi, per role seed, bahwa hasil Catalog_Migration memberi akses yang sama dengan matriks D9 di design.
4. THE backend ATM test suite SHALL memuat test RBAC denial: role tanpa Screen_Key menerima 403 untuk setiap endpoint layar yang dipindah.
