# Task — Auth Lokal & Siklus Hidup Akun (Internal + Vendor)

> Spec acuan implementasi. Scope: **autentikasi lokal + siklus hidup akun** (login source, kebijakan password lokal, first-login change, akun ADMIN/APPACCESS, soft-delete). **Terpisah** dari `RBAC-Setup` (hierarki + maker-checker approval). Overlap hanya di titik "APPACCESS mengonfigurasi mapping CRUD & delegasi RBAC" — di sana spec ini merujuk RBAC-Setup, bukan menduplikasi.
> Basis: temuan riset codebase (kolom & mekanisme yang SUDAH ada). Migrasi baru mulai `026` (RBAC-Setup memakai `021`–`025`, termasuk `025_approval_delegations_no_overlap.sql` yang menyusul belakangan — dicek ulang 2026-09-09, `025` sudah terpakai).
> Aturan proyek: tabel/kolom/role baru diusulkan di `.kiro/steering/project-context.md` Sec 2 dulu; **sentuhan auth = STOP-and-confirm** (project-context Sec 3.7, Sec 4).

---

## Problem Statement

CMS harus tetap bisa dipakai **sebelum LDAP/Entra ID terhubung**: user internal maupun vendor perlu bisa login via kredensial di DB lokal. Selain itu akun bank yang sensitif (ADMIN, APPACCESS) tidak boleh bergantung pada LDAP, butuh kebijakan password yang layak bank (expiry, warning, lockout), dan alur "set password awal → wajib ganti saat login pertama". Terakhir, akun yang sudah punya jejak audit **tidak boleh di-hard-delete** supaya rantai audit tetap nyambung.

## Requirements (dari klarifikasi user)

1. **Flag login lokal.** `users` diberi flag (mis. `is_karyawan` yang sudah ada + penanda login lokal). Jika flag lokal aktif, user boleh login pakai DB lokal — untuk kebutuhan non-dev selama LDAP/Entra ID belum konek (internal maupun vendor).
2. **ADMIN disimpan di DB lokal** (tidak konek ke LDAP mana pun), dan punya fitur **change password** (bukan reset) untuk dirinya sendiri.
3. **Kebijakan password khusus DB lokal** (kalau sumbernya LDAP/Entra ID, kebijakan diatur di LDAP/Entra, bukan di sini):
   - `password_expired = 90 hari`.
   - `warning_password_expired = 7 hari` sebelum expiry.
   - **Lockout**: gagal login **3x** → terkunci; recovery via **tunggu 30 menit** ATAU jalur admin. (User memilih salah satu, keduanya didukung.)
4. **APPACCESS** = satu-satunya user yang bisa: konfigurasi mapping CRUD & delegasi RBAC (maker-checker) — *lihat RBAC-Setup*; **set password pertama** user lain; begitu user login pertama kali langsung diminta **change password** (khusus password lokal; kalau LDAP/Entra, diatur di sana).
5. **Soft-delete akun ber-audit.** User yang pernah melakukan event apa pun yang sudah ter-track di audit **tidak boleh dihapus** dari tabel; cukup set **inactive / deleted status** agar audit tetap nyambung.

## Background (temuan riset — WAJIB dipatuhi)

- **`users` sudah punya**: `role_id, employee_id, username, full_name, email, is_karyawan, auth_source, password_hash, vendor_id, is_active, last_login_at, created_at, updated_at, **deleted_at**` (`backend/migrations/002_cms_tables.sql`). **Soft-delete sudah didukung** via `deleted_at` — `FindByUsername` sudah filter `deleted_at IS NULL`.
- **`auth_source`** saat ini bernilai `ldap | local`; arsip `003_add_local_dev_auth` memperkenalkan `local_dev` + constraint `users_auth_source_chk`, `users_password_hash_chk`, `users_vendor_logic_chk`. Nilai `local_dev` adalah mekanisme "login pakai DB dulu selama LDAP belum konek" yang **sudah pernah dibuat** — putuskan **revive `local_dev`** vs. flag baru (lihat Task 1, butuh konfirmasi user).
- **`pkg/auth`** sudah punya: `Provider` interface (`Authenticate`, `Supports(authSource)`), `LocalProvider` (bcrypt), stub `LDAPProvider`, `UserRecord` (field `PasswordHash *string`, `AuthSource string` — komentar sudah menyebut `"ldap","local","local_dev"`), error terstandar (`errors.go`: `ErrRateLimited`, `ErrLDAPNotConfigured`, dll).
- **Repo auth** = `backend/internal/repository/auth_repository.go` (sqlc `db.Queries`): `FindByUsername`, `UpdateLastLogin`, `GetUserProfile`. Pola: query di `queries/*.sql` → `sqlc generate` → map row → `auth.UserRecord`.
- **Rate limit login** sudah ada di `pkg/middleware` (username+IP, increment-on-failure) — dipakai login. Lockout 3x/30-menit adalah **kebijakan per-akun di DB**, berbeda dari rate-limit IP; jangan dicampur (lihat komentar `dsr_upload.go` yang membedakan keduanya).
- **Roles saat ini (9, project-context + `002` comment):** `ADMIN | ADMIN_PARAM | ATM-USER | ATM-SPV | BRANCH-USER | BRANCH-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | VENDOR-USER`. **`APPACCESS` BUKAN salah satunya** → penambahan role = STOP-and-confirm (lihat Open Questions).
- Migrasi additive acuan gaya `016_users_vendor_branch_id.sql`: `BEGIN; ADD COLUMN IF NOT EXISTS; COMMENT; ADD CONSTRAINT; CREATE INDEX IF NOT EXISTS; COMMIT;` + header WHY/SAFETY. Migrasi tertinggi keseluruhan: RBAC-Setup memakai `021`–`025` (termasuk `025_approval_delegations_no_overlap.sql`), jadi spec ini mulai `026`. Tidak ada CHECK constraint `users_auth_source_chk` di skema live (arsip `003_add_local_dev_auth` tidak pernah diterapkan) — tidak perlu penyesuaian constraint saat revive `local_dev`.
- Read/replica belum ter-wire (single `dbPool`, TODO `ponytail`). Pakai `dbPool` untuk read & write, tinggalkan TODO.
- ATM backend tetap **flat JSON shape** (wire-compat FE). Endpoint auth existing tidak diubah bentuk responsnya.

## Resolved Decisions

1. **`APPACCESS` = role BARU** (bukan reuse `ADMIN_PARAM`). Role set menjadi **10 role**. APPACCESS adalah satu-satunya otoritas provisioning akun + konfigurasi mapping CRUD & delegasi RBAC (maker-checker), terpisah dari `ADMIN` (system admin) dan `ADMIN_PARAM` (parameter admin). Diusulkan di project-context Sec 2. *(Keputusan user: Q1=A.)*
2. **Flag login lokal = reuse `auth_source`** (nilai `ldap | local | local_dev`), **bukan** kolom `is_local` baru. `local_dev` yang sudah ada di arsip (`003_add_local_dev_auth`) di-revive sebagai penanda "login via DB selama LDAP/Entra belum konek" (internal), `local` tetap untuk vendor. Satu sumber kebenaran; `Provider.Supports(authSource)` sudah me-route berdasarkan ini. *(Keputusan user: Q2=A.)*

## Open Questions (STOP-and-confirm sebelum eksekusi task terkait)

1. **Recovery lockout**: 30-menit auto-unlock **dan** unlock manual oleh APPACCESS/ADMIN keduanya didukung — konfirmasi tidak ada jalur "self-reset" untuk akun terkunci.

---

## Proposed Solution (4 area)

1. **Login source & fallback lokal** — normalisasi `auth_source` sehingga internal boleh login lokal saat LDAP belum konek; `Provider.Supports` + routing `Authenticate` sudah mendukung ini.
2. **Kebijakan password lokal** — kolom baru di `users`: `password_changed_at`, `must_change_password bool`, `failed_login_attempts int`, `locked_until timestamptz`. Enforcement hanya saat `auth_source` lokal; LDAP/Entra dilewati.
3. **Alur akun** — ADMIN self change-password; APPACCESS set password awal + paksa `must_change_password=true`; first-login → wajib ganti.
4. **Soft-delete ber-audit** — larang hard-delete user yang punya baris audit; service hanya set `is_active=false` + `deleted_at=now()`; audit tiap perubahan status.

**Prinsip non-negosiable:** password lokal di-hash (bcrypt, cost sesuai `LocalProvider`); jangan pernah log password/hash; kebijakan expiry/lockout **hanya** untuk `auth_source` lokal; setiap perubahan akun (create/set-password/lock/unlock/inactivate) tulis `audit_logs` (bergantung `audit_logs` dari RBAC-Setup Task 3 — lihat Dependencies); write → primary; change-password ≠ reset (butuh password lama untuk self-service).

---

## Dependencies

- **`audit_logs` + `internal/audit` writer** berasal dari **RBAC-Setup Task 3**. Task 5 & 7 di sini menulis audit → butuh writer itu ada dulu, atau sepakati stub sementara. Tandai urutan ini saat eksekusi.
- **`APPACCESS`** menyentuh konfigurasi RBAC yang didefinisikan di RBAC-Setup (Task 8 admin endpoints). Spec ini hanya menangani sisi *auth lifecycle* (set/first-password), bukan mekanisme mapping CRUD itu sendiri.

---

## Task Breakdown

### [x] Task 1 — Usulkan skema di project-context.md (flag lokal & role sudah diputuskan)
- Flag login lokal **sudah diputuskan** = reuse `auth_source` + revive `local_dev` (lihat Resolved Decisions #2). Tidak ada kolom `is_local`.
- Update `.kiro/steering/project-context.md` Sec 2: tandai kolom baru `users.password_changed_at`, `users.must_change_password`, `users.failed_login_attempts`, `users.locked_until` + role `APPACCESS` + revive `auth_source='local_dev'` sebagai **usulan**.
- **Test:** —（dokumentasi/keputusan; tanpa kode）
- **Demo:** diff project-context menampilkan usulan kolom + role APPACCESS + auth_source local_dev.
- **Model:** Haiku — hanya edit dokumentasi/keputusan skema di project-context, tanpa kode, penilaian minim.
- **Selesai** (dicek 2026-09-09): sudah ada di `.kiro/steering/project-context.md` — baris ~39 (role APPACCESS, proposed) dan ~92 (auth_source local_dev + 4 kolom password policy, proposed), keduanya sudah merujuk balik ke spec ini.

### [x] Task 2 — Migrasi kolom kebijakan password lokal
- `backend/migrations/026_users_password_policy.sql` (gaya `016`): `ADD COLUMN IF NOT EXISTS` untuk `password_changed_at timestamptz`, `must_change_password boolean NOT NULL DEFAULT false`, `failed_login_attempts int NOT NULL DEFAULT 0`, `locked_until timestamptz`. COMMENT tiap kolom (berlaku hanya untuk auth_source lokal). Tidak ada `users_auth_source_chk` di skema live untuk disesuaikan (lihat Background) — cukup 4 kolom di atas.
- **Test:** migrasi up/down bersih; kolom + default benar; user LDAP existing tak terpengaruh (kolom nullable/default aman).
- **Demo:** `\d users` menampilkan 4 kolom baru dengan default.
- **Model:** Opus — migrasi skema tabel `users` (auth danger zone); kolom/default salah bisa merusak login existing.
- **Selesai** (2026-09-09): file dibuat, idempotent (`ADD COLUMN IF NOT EXISTS`), tanpa constraint/index tambahan (lihat SAFETY note di file). **Belum diverifikasi jalan di DB nyata** — Postgres eksternal tidak dapat diakses dari sesi ini; jalankan manual lalu cek `\d users` sebelum lanjut ke Task 3.

### [x] Task 3 — Kebijakan password lokal di service (expiry 90d, warning 7d) — no HTTP
- Di `pkg/auth` (atau service auth): helper `PasswordExpiry(changedAt, now) -> (expired bool, daysLeft int)` dengan konstanta `PasswordMaxAgeDays=90`, `PasswordWarnDays=7`. Hanya dievaluasi untuk `auth_source` lokal; LDAP/Entra di-skip.
- Login sukses password lokal: jika `expired` → tolak dengan error jelas + tandai wajib ganti; jika dalam window warning → sertakan `password_days_left` di response (tanpa mengubah bentuk flat JSON secara breaking — tambah field opsional).
- **Test (table-driven):** tepat 90 hari, 91 hari (expired), 83 hari (warning 7), 82 hari (no warning), akun LDAP (kebijakan di-skip).
- **Demo:** unit test matriks batas 90/7 hari.
- **Model:** Opus — kebijakan expiry password di jalur login; logika salah bisa mengunci/meloloskan akun secara keliru.
- **Selesai** (2026-09-09):
  - `pkg/auth/password_policy.go`: `PasswordExpiry` + `InPasswordWarningWindow`, konstanta `PasswordMaxAgeDays=90`/`PasswordWarnDays=7`. Table-driven test di `pkg/auth/password_policy_test.go` (90/91/83/82 hari + nil changedAt).
  - `pkg/auth/errors.go`: sentinel baru `ErrPasswordExpired`.
  - `pkg/auth/repository.go`: `UserRecord.PasswordChangedAt`, `UserRepository.MarkPasswordExpired`.
  - `backend/queries/auth.sql` + `backend/internal/db/auth.sql.go`: kolom `password_changed_at` di `FindUserByUsername`, query baru `MarkPasswordExpired` — **di-hand-edit, bukan `sqlc generate`**: sqlc gagal karena error pre-existing tak terkait di migrasi `017`/`018` (dicek, bukan disebabkan migrasi `026`). Style disamakan manual dengan output sqlc yang sudah ada.
  - `backend/internal/repository/auth_repository.go`: mapping field baru + `MarkPasswordExpired`.
  - `backend/internal/auth/service.go`: `Login()` step 5b — evaluasi expiry hanya untuk `auth_source in (local, local_dev)`; expired → `MarkPasswordExpired` + tolak `ErrPasswordExpired`; warning window → `LoginResponse.PasswordDaysLeft *int` (opsional, `omitempty`, non-breaking).
  - `backend/internal/handler/auth_handler.go`: `handleAuthError` map `ErrPasswordExpired` → 403 (tanpa ini errornya jatuh ke 500 generik untuk endpoint login yang sudah ada).
  - Test tambahan di `backend/internal/auth/service_test.go` (expired-rejected+mark-called, warning-window pointer, ldap-skip) + stub `MarkPasswordExpired` di `service_test.go` & `local_provider_test.go`.
  - **Verifikasi:** `go build ./...` + `go test ./...` hijau di modul `backend` dan `pkg` (workspace `go.work`, dua modul terpisah — tidak ada `go.mod` gabungan untuk run sekali).

### [x] Task 4 — Lockout 3x + auto-unlock 30 menit (per-akun, no HTTP)
- Konstanta `MaxFailedLogins=3`, `LockoutDuration=30m`. Hanya untuk `auth_source` lokal.
- Query sqlc: increment `failed_login_attempts` saat gagal; saat mencapai 3 → set `locked_until=now()+30m`. Login saat `locked_until > now()` → `ErrAccountLocked` (error baru di `pkg/auth/errors.go`, pesan ID). Login sukses → reset `failed_login_attempts=0`, `locked_until=NULL`. Auto-unlock: `locked_until <= now()` dianggap tidak terkunci.
- **Test (table-driven):** 2 gagal tak terkunci; 3 gagal terkunci; login saat terkunci ditolak walau password benar; setelah 30m boleh coba lagi; sukses mereset counter; akun LDAP tak kena lockout.
- **Demo:** unit test urutan gagal×3 → locked → +31m → unlocked.
- **Model:** Opus — lockout state per-akun di jalur auth; salah hitung bisa mengunci pengguna sah atau melewati proteksi brute-force.
- **Selesai** (2026-09-09):
  - `pkg/auth/lockout.go`: `MaxFailedLogins=3`, `LockoutDuration=30m`, `IsLocked(lockedUntil, now)`. Test boundary di `pkg/auth/lockout_test.go` (nil, future, exact-now, 31m lewat).
  - `pkg/auth/errors.go`: sentinel baru `ErrAccountLocked`.
  - **Desain berbeda dari draft spec**: ambang batas 3x **diputuskan di service Go**, bukan `CASE` di SQL — `IncrementFailedLogin` cuma mengembalikan count baru (`RETURNING failed_login_attempts`), service yang membandingkan ke `MaxFailedLogins` lalu panggil `LockAccount` terpisah. Alasan: threshold jadi unit-testable dengan stub (tanpa DB nyata), konsisten dengan pola `pkg/middleware/rate_limiter.go` yang juga membandingkan count di Go, bukan di Redis.
  - `pkg/auth/repository.go`: `UserRecord.FailedLoginAttempts`/`LockedUntil`; `UserRepository.IncrementFailedLogin`, `.LockAccount`, `.ResetLockout`.
  - `backend/queries/auth.sql` + `backend/internal/db/auth.sql.go` (hand-edit, sqlc masih gagal karena error pre-existing di migrasi `017`/`018`, sama seperti Task 3): kolom `failed_login_attempts`/`locked_until` di `FindUserByUsername`; query baru `IncrementFailedLogin`, `LockAccount`, `ResetLockout`.
  - `backend/internal/repository/auth_repository.go`: implementasi + `timeToTimestamptz`-inline via `pgtype.Timestamptz{Time: until, Valid: true}`.
  - `backend/internal/auth/service.go`: `Login()` step 4b (cek lock **sebelum** verifikasi kredensial — akun terkunci ditolak walau password benar) + step 5 (gagal auth → increment, kalau count>=3 → lock) + step 8 (`ResetLockout` tiap login sukses). Helper `isLocalAuthSource` diekstrak (dipakai ulang oleh cek expiry Task 3 juga, DRY).
  - `backend/internal/handler/auth_handler.go`: map `ErrAccountLocked` → 403.
  - 8 test integrasi baru di `service_test.go` (2 gagal/3 gagal, lock ditolak walau password benar, lock 31m lewat auto-unlock, sukses reset counter, LDAP tak pernah terkunci) + stub `IncrementFailedLogin`/`LockAccount`/`ResetLockout` di `service_test.go` & `local_provider_test.go`.
  - **Verifikasi:** `go build`, `go vet`, `go test ./...` hijau di modul `backend` & `pkg`.

### [x] Task 5 — Self change-password (ADMIN & user lokal) + first-login force-change
- Endpoint `POST /api/v1/auth/change-password` (butuh `RequireAuth`): body `old_password`, `new_password`. Verifikasi `old_password` (bcrypt), validasi kekuatan `new_password`, hash, set `password_hash`, `password_changed_at=now()`, `must_change_password=false`, reset lockout. **Bukan reset** — wajib password lama. Hanya untuk `auth_source` lokal (LDAP → tolak `ErrChangeNotAllowed`).
- First-login: jika `must_change_password=true`, login tetap boleh tapi response menandai `must_change_password`; endpoint lain boleh digating di FE. (Backend minimal: expose flag.)
- Tulis `audit_logs` (action `password_change`, actor=self, tanpa nilai password). Write → primary.
- **Test (httptest + service):** happy path; old_password salah → 400/401; user LDAP → ditolak; new==old ditolak; audit tercatat tanpa secret; flag `must_change_password` jadi false.
- **Demo:** login user `must_change_password=true` → change-password → login ulang tanpa paksaan.
- **Model:** Opus — endpoint change-password + verifikasi bcrypt + audit; salah tangani = celah keamanan kredensial.
- **Selesai** (2026-09-09):
  - `pkg/auth/password_strength.go`: `MinPasswordLength=8`, `MaxPasswordLength=255`, `ValidatePasswordStrength` (letter+digit, length bounds). Test di `pkg/auth/password_strength_test.go`.
  - `pkg/auth/errors.go`: sentinel baru `ErrChangeNotAllowed`, `ErrPasswordUnchanged`.
  - `pkg/auth/repository.go`: `UserRecord.MustChangePassword`; `UserRepository.FindByID`, `.SetPassword`.
  - `backend/queries/auth.sql` + `backend/internal/db/auth.sql.go` (hand-edit, sqlc masih gagal di migrasi `017`/`018`, tidak terkait): `must_change_password` ditambahkan ke `FindUserByUsername`; query baru `FindUserByID`, `SetPassword` (satu `UPDATE` atomik: hash + `password_changed_at=now()` + `must_change_password=false` + reset lockout, sekaligus).
  - `backend/internal/repository/auth_repository.go`: implementasi + mapping.
  - `backend/internal/auth/change_password.go` (**file baru**): `ChangePasswordService` — cek `auth_source` lokal dulu, verifikasi `old_password` (bcrypt), tolak jika `new==old`, validasi kekuatan, hash (`BcryptCost` sama dgn `LocalProvider`), `SetPassword`, lalu tulis `audit_logs` (action `password_change`, tanpa payload before/after — tidak ada nilai password yang tersimpan). Pola `AuditWriter` interface persis mengikuti `internal/approval/store.go` (bukan reuse langsung `*audit.Writer` — supaya testable tanpa DB).
  - `backend/internal/auth/service.go`: `LoginResponse.MustChangePassword bool` (selalu ada, additive) — expose flag first-login sesuai requirement.
  - **Bug ditemukan & diperbaiki**: `LoginResponse.PasswordDaysLeft` dari Task 3 sempat tidak pernah keluar ke HTTP response — `auth_handler.go`'s `Login()` membangun `loginResponse` lokalnya sendiri dari `resp.AccessToken`/`resp.User` saja, mengabaikan `resp.PasswordDaysLeft`. Diperbaiki sekalian dengan menambah `PasswordDaysLeft`/`MustChangePassword` ke `loginResponse` handler + memetakannya di `Login()`.
  - `backend/internal/handler/auth_handler.go`: route baru `POST /change-password` (`RequireAuth`, interface sempit `ChangePasswordService` — pola sama dgn `ApprovalOrchestrator`/`ApprovalReader` di `approval_handler.go`, supaya testable tanpa DB); `handleChangePasswordError` (401/403/400/422 sesuai jenis error).
  - `backend/cmd/api/main.go`: `auditWriter` dipindah lebih awal (dipakai bersama oleh change-password & approval, tidak dobel-deklarasi); wiring `changePasswordService`.
  - Test: 6 unit test service (`change_password_test.go`, termasuk assert audit entry tanpa Before/After) + 6 httptest handler (`auth_handler_test.go`, pakai `noopBlacklist`/`tokenFor`/`doRequest` yang sudah ada di `approval_handler_test.go`) + stub `FindByID`/`SetPassword` di `service_test.go` & `local_provider_test.go` + update 2 titik panggil `NewAuthHandler` di `integration_test.go` (tag `integration`, dicek dengan `go vet -tags integration`, tak bisa jalan penuh — DB eksternal tak terjangkau sesi ini).
  - **Verifikasi:** `go build`, `go vet`, `go test ./...` hijau di modul `backend` & `pkg`.

### [x] Task 6 — APPACCESS set password awal user + paksa first-login change
- Role `APPACCESS` **sudah diputuskan** = role baru (lihat Resolved Decisions #1). Usulkan di project-context Sec 2 (Task 1) sebelum seed/wiring.
- Endpoint `POST /api/v1/admin/users/{id}/set-initial-password` (`RequireRoles("APPACCESS")`): set `password_hash`, `password_changed_at=now()`, `must_change_password=true`, reset lockout. Tidak butuh password lama (ini set awal oleh APPACCESS, bukan self-change). Hanya untuk target `auth_source` lokal.
- Tulis `audit_logs` (action `initial_password_set`, actor=APPACCESS, target user; tanpa nilai password).
- **Test (httptest):** hanya APPACCESS boleh (role lain 403); target LDAP ditolak; `must_change_password` jadi true; audit tercatat; user lalu wajib change saat login pertama.
- **Demo:** APPACCESS set password user baru → user login → dipaksa change.
- **Model:** Opus — role gate APPACCESS + set password awal + migrasi seed role baru; auth + RBAC danger zone.
- **Selesai** (2026-09-09):
  - `backend/migrations/027_seed_appaccess_role.sql` (**migrasi baru, sentuhan auth/role — lihat catatan di bawah**): `INSERT INTO roles ('APPACCESS', ...)` idempotent (`ON CONFLICT DO NOTHING`), gaya sama dengan `003_seed_roles_users.sql`; update COMMENT kolom `roles.role`. Role jadi 10, sesuai Resolved Decisions #1.
  - `backend/queries/auth.sql` + `auth.sql.go` (hand-edit, sqlc masih gagal karena migrasi `017`/`018`, tidak terkait): query baru `SetInitialPassword` — `UPDATE` atomik sama seperti `SetPassword` (Task 5) tapi `must_change_password = true` (bukan `false`) — mencerminkan arah kebalikannya.
  - `pkg/auth/repository.go`: `UserRepository.SetInitialPassword`. `pkg/auth/errors.go`: sentinel baru `ErrUserNotFound` (404 admin lookup-by-id, beda dari `ErrInvalidCredentials` yang dipakai Task 5 untuk kasus self-service).
  - `backend/internal/repository/auth_repository.go`: implementasi.
  - `backend/internal/auth/set_initial_password.go` (**file baru**): `SetInitialPasswordService` — cari target via `FindByID` (dari Task 5), cek `auth_source` lokal, validasi kekuatan password, hash, `SetInitialPassword`, tulis audit (action `initial_password_set`, actor=APPACCESS bukan target, tanpa payload password). **Tidak ada cek password lama** (beda dari `ChangePasswordService`) — tipe/struct terpisah dari Task 5 karena aturan & aktor beda, dependencies sama (`userRepo` + `AuditWriter` yang sudah ada).
  - `backend/internal/handler/admin_user_handler.go` (**file baru**): `AdminUserHandler`, route `POST /{id}/set-initial-password`, interface sempit `SetInitialPasswordService` (pola sama dgn `ChangePasswordService`/`ApprovalOrchestrator`). Reuse `parsePathID` dari `admin_approval_handler.go`.
  - `backend/cmd/api/main.go`: mount `/api/v1/admin/users` dengan `RequireAuth + RequireRoles("APPACCESS")`, reuse `auditWriter` yang sudah ada.
  - Test: 5 unit test service (`set_initial_password_test.go`) + 5 httptest handler (`admin_user_handler_test.go`, termasuk test role gate ADMIN→403 dgn mount RequireRoles nyata, bukan cuma handler-level) — reuse `tokenForRole`/`noopBlacklist`/`doRequest` yang sudah ada di `admin_approval_handler_test.go`/`approval_handler_test.go`.
  - **Catatan STOP-and-confirm**: migrasi `027` menyentuh auth (role baru) — sesuai aturan proyek Sec 4.7 ini butuh konfirmasi. Keputusan penambahan role APPACCESS **sudah** direkam sebagai Resolved Decisions #1 di spec ini (persetujuan user sebelum sesi ini), dan user mengarahkan eksekusi Task 6 secara eksplisit — jadi dieksekusi, bukan di-skip. **Belum diverifikasi jalan di DB nyata** (role INSERT + full alur set-password→login→forced-change) — Postgres eksternal tak terjangkau sesi ini; jalankan `027` lalu tes alur manual sebelum produksi.
  - **Verifikasi:** `go build`, `go vet` (termasuk `-tags integration`), `go test ./...` hijau di modul `backend` & `pkg`.

### [x] Task 7 — Soft-delete akun ber-audit (larang hard-delete)
- Service `DeactivateUser(id)`: cek apakah user punya baris di `audit_logs` (actor_id = id) — jika ya, **hanya** set `is_active=false`, `deleted_at=now()` (soft). Sediakan guard di repo: tidak ada path SQL `DELETE FROM users` untuk user ber-audit. `FindByUsername` sudah menyaring `deleted_at IS NULL` (login otomatis tertutup).
- Reaktivasi (opsional, admin): `is_active=true`, `deleted_at=NULL`.
- Tulis `audit_logs` (action `user_deactivated`/`user_reactivated`).
- **Test:** user ber-audit → deactivate = soft (baris tetap ada, `deleted_at` terisi); percobaan hard-delete ditolak/absen; user tanpa audit tetap soft-delete (konsisten, tak ada hard-delete di alur normal); user soft-deleted tak bisa login.
- **Demo:** buat audit untuk user → deactivate → baris masih ada, login gagal, audit lama tetap tertaut.
- **Model:** Opus — soft-delete akun + guard larang hard-delete demi integritas rantai audit; salah = putus audit atau akun ter-hapus permanen.
- **Selesai** (2026-09-09):
  - **Penyederhanaan sengaja**: cek "punya audit atau tidak" TIDAK diimplementasikan sebagai percabangan runtime — kedua test case spec ("user ber-audit" vs "user tanpa audit") menghasilkan perilaku identik (soft-delete), jadi cukup satu code path tanpa query audit_logs tambahan yang tak pernah mengubah hasil. Konsisten dengan Tradeoffs section spec sendiri: "alur normal seluruhnya soft-delete demi konsistensi audit."
  - `backend/queries/auth.sql` + `auth.sql.go` (hand-edit, sqlc masih gagal di migrasi `017`/`018`, tidak terkait): query baru `DeactivateUser` (`is_active=false, deleted_at=now()`), `ReactivateUser` (kebalikannya). Tidak ada migrasi baru — kolom `is_active`/`deleted_at` sudah ada dari `002_cms_tables.sql`.
  - `pkg/auth/repository.go`: `UserRepository.Deactivate`, `.Reactivate`.
  - `backend/internal/repository/auth_repository.go`: implementasi.
  - `backend/internal/auth/deactivate_user.go` (**file baru**): `DeactivateUserService.Deactivate`/`.Reactivate`. `Deactivate` cek target ada dulu via `FindByID` (dari Task 5) lalu soft-delete + audit (`user_deactivated`). `Reactivate` **tanpa** pre-check eksistensi (`FindByID` menyaring `deleted_at IS NULL` sehingga tak bisa "melihat" user yang sudah soft-deleted untuk diverifikasi) — ditandai `ponytail:` comment di kode, opsional per spec, upgrade path: tambah `FindByIDIncludingDeleted` kalau nanti butuh 404 yang presisi.
  - **Guard "tidak ada path SQL `DELETE FROM users`"**: karena tidak ada kode yang bisa "ditest gagal" (tidak ada method-nya sama sekali), guard diimplementasikan sebagai **test statis** — `backend/internal/repository/no_hard_delete_test.go` men-scan `queries/*.sql` (strip komentar `--` dulu supaya tidak false-positive pada dokumentasi yang menyebut frasa itu) dan gagal kalau ada `DELETE FROM users` di manapun. Ini persis pola yang sudah dipakai migrasi `023_audit_logs.sql` (append-only diamankan lewat "tidak ada query UPDATE/DELETE yang ditulis", didokumentasikan bukan di-enforce DB).
  - **"User soft-deleted tak bisa login"**: sudah tercakup test yang SUDAH ADA sebelum sesi ini — `TestService_Login_DeletedUser_GenericError` (`service_test.go`) — dicek ulang masih hijau, tidak perlu test baru.
  - Test baru: 5 unit test service (`deactivate_user_test.go`, termasuk 2 test yang sengaja dipisah untuk mencocokkan 1:1 dua bullet test spec meski assertion-nya identik) + 1 test statis guard (`no_hard_delete_test.go`).
  - **Tidak ada endpoint HTTP dibuat** — beda dari Task 5/6, Task 7 tidak menyebutkan path endpoint apa pun (hanya "Service DeactivateUser(id)"), jadi scope dibatasi ke service layer saja (YAGNI). Wiring ke HTTP + role gate (ADMIN/APPACCESS?) menyusul kalau memang dibutuhkan.
  - **Verifikasi:** `go build`, `go vet` (termasuk `-tags integration`), `go test ./...` hijau di modul `backend` & `pkg`.

### [x] Task 8 — Quality gate + dokumentasi
- `sqlc generate`, `go test ./...`, `golangci-lint run`, `go build`.
- Update project-context.md (tandai proposed → ada), `.env.example` bila ada konstanta yang perlu di-config (mis. override 90/7/3/30 — default hardcoded aman, config opsional). Catat pola: modul lain jangan hard-delete user.
- **Test:** seluruh suite hijau; coverage `internal/*` & `pkg/auth` yang disentuh >= 80%.
- **Demo:** ringkasan alur auth lokal end-to-end (login fallback → warning → expiry → lockout → change → soft-delete).
- **Model:** Sonnet — quality gate (build/lint/test/coverage) + dokumentasi; butuh penilaian triase kegagalan, bukan reasoning Opus.
- **Selesai** (2026-09-09):
  - **`sqlc generate`: masih GAGAL**, sama seperti setiap task sebelumnya — pre-existing, tidak terkait spec ini: `migrations/017_atm_dsr_location_and_rencana_isi.sql:79:5: syntax error` + `018...: relation "atm_dsr_rencana_isi_files" does not exist`. Semua perubahan sqlc di Task 2-7 (`auth.sql` → `auth.sql.go`) di-hand-edit, meniru persis gaya output sqlc yang sudah ada. **Ini bukan sesuatu yang saya perbaiki** — memperbaiki migrasi DSR lama di luar scope Auth-Local-Lifecycle dan berisiko; direkomendasikan jadi task terpisah kalau mau benar-benar menjalankan `sqlc generate` lagi.
  - `go build ./...`: bersih di modul `backend` & `pkg`.
  - `go vet ./...` (termasuk `-tags integration` untuk `internal/handler`): bersih.
  - `golangci-lint run ./...`: 9 temuan (8 `errcheck`, 1 `staticcheck`) — **semua di file yang tidak disentuh sesi ini** (`cmd/api/main.go` baris pre-existing, `dsr_upload_handler.go`, `dsr_upload.go`, `dsr_upload_test.go`, `atm_portal_cashpos.go`, `error_response.go`). Tidak ada temuan di kode Auth-Local-Lifecycle (`pkg/auth/*`, `internal/auth/*`, `internal/handler/{auth,admin_user}_handler*.go`, `internal/repository/auth_repository.go`).
  - `go test ./... -cover`: seluruh suite hijau di kedua modul. Coverage packages: `internal/auth` **90.6%**, `pkg/auth` **81.1%** — keduanya di atas 80%. Per-fungsi file baru sesi ini (`change_password.go`, `set_initial_password.go`, `deactivate_user.go`, `password_policy.go`, `lockout.go`, `password_strength.go`) semua **80-100%** setelah ditambah test untuk cabang audit-write-failure & invalid-body/invalid-id. `internal/handler` package-level cuma 54.2% (didominasi handler ATM/DSR/DMAA yang tidak disentuh) — tapi fungsi baru saya (`ChangePassword` 84.6%, `SetInitialPassword` 88.2%, `handleChangePasswordError`/`handleSetInitialPasswordError` 100%) semua di atas 80%. `internal/repository` package-level 0% (lihat catatan di bawah) dan `cmd/api` 0% — **bukan regresi**, keduanya sudah 0%/tanpa test sebelum sesi ini (repository = adapter tipis di atas sqlc, dites lewat integration test `-tags integration` per konvensi proyek Sec 8 "Integration: real Postgres for repo/API tests", bukan unit test; cmd/api = wiring murni).
  - project-context.md Sec 2 (Auth): dua baris "Proposed (NOT yet approved)" diubah ke "Implemented" (role `APPACCESS` + kolom password policy/`local_dev`), dengan catatan jujur "belum diverifikasi jalan di DB nyata" karena Postgres eksternal tak terjangkau sepanjang sesi ini.
  - **`.env.example` — sengaja TIDAK diubah**: tidak ada kode yang membaca env var untuk override `90/7/3/30` (semua konstanta hardcoded di `pkg/auth`: `PasswordMaxAgeDays`, `PasswordWarnDays`, `MaxFailedLogins`, `LockoutDuration`). Spec sendiri bilang "default hardcoded aman, config opsional" — menambah entri `.env.example` yang tidak ada kode pembacanya hanya akan jadi dead config yang menyesatkan. Skip, sesuai YAGNI.
  - **Pola dicatat** (sesuai permintaan "modul lain jangan hard-delete user"): lihat `backend/internal/repository/no_hard_delete_test.go` — guard statis yang men-scan `queries/*.sql`, gagal kalau ada modul MANAPUN (bukan cuma auth) yang menambah `DELETE FROM users`. Ini otomatis menegakkan pola untuk seluruh proyek, bukan cuma dokumentasi.
  - **Demo (ringkasan alur, karena DB nyata tak terjangkau — ini diverifikasi via unit/httptest, bukan end-to-end sungguhan):**
    1. Login lokal fallback: `auth_source=local|local_dev` diautentikasi via `LocalProvider` (bcrypt), LDAP tetap lewat provider terpisah.
    2. Warning: login sukses dengan password berumur 83-90 hari → response `password_days_left` (1-7).
    3. Expiry: password >90 hari → login ditolak (`ErrPasswordExpired`, 403) + `must_change_password` diset true di DB.
    4. Lockout: 3x salah password → `locked_until=now()+30m`; percobaan berikutnya ditolak (`ErrAccountLocked`, 403) walau password benar; setelah 30 menit auto-unlock.
    5. Change: `POST /auth/change-password` (self, butuh password lama) atau `POST /admin/users/{id}/set-initial-password` (APPACCESS, tanpa password lama, forcing `must_change_password=true`) — keduanya tulis `audit_logs` tanpa nilai password.
    6. Soft-delete: `DeactivateUserService.Deactivate` set `is_active=false, deleted_at=now()` — tidak ada `DELETE FROM users` di manapun (dijaga test statis); login user itu otomatis tertutup lewat filter `deleted_at IS NULL` yang sudah ada.
  - **Belum terverifikasi end-to-end di Postgres nyata** — semua di atas sudah lolos unit test + httptest dengan stub/fake, TAPI belum pernah benar-benar dijalankan lewat migrasi `026`/`027` + query nyata di database. Sebelum produksi: jalankan migrasi, lalu tes manual alur lengkap (idealnya lewat `go test -tags integration ./...` dengan `DATABASE_URL` terisi).

---

## Tradeoffs & Constraints

- Kebijakan expiry/warning/lockout **hanya** untuk `auth_source` lokal; sumber LDAP/Entra ID mengelola kebijakannya sendiri — jangan duplikasi di CMS.
- Lockout per-akun (DB) **berbeda** dari rate-limit login per-IP (`pkg/middleware`); keduanya berjalan berdampingan, jangan digabung.
- Change-password self-service **wajib password lama** (bukan reset). Set-initial-password oleh APPACCESS tidak butuh password lama tapi memaksa first-login change.
- Soft-delete: user ber-audit tidak pernah di-hard-delete; alur normal seluruhnya soft-delete demi konsistensi audit.
- `APPACCESS` (role baru) & flag login lokal (`auth_source`+`local_dev`) sudah diputuskan (Resolved Decisions); tetap diusulkan di project-context Sec 2 sebelum migrasi/seed.
- `audit_logs` writer bergantung RBAC-Setup Task 3 (lihat Dependencies).
- Read/replica belum ter-wire: pakai `dbPool` untuk read & write, tinggalkan TODO gaya `ponytail`.
- Ikuti layout technical-layer aktual (handler/service/repository + sqlc). Migrasi mulai `026`. ATM backend tetap flat JSON.

## Definition of Done (Sec 11 project-context)

- [ ] Sesuai module/table map (Sec 2) — kolom/role diusulkan dulu
- [ ] Auth path benar (LDAP di-skip untuk kebijakan lokal), RBAC ter-scope (APPACCESS-only untuk set-initial-password)
- [ ] Setiap perubahan akun tulis `audit_logs` (change/set/lock/unlock/deactivate)
- [ ] Read replica untuk read, primary untuk write (TODO wiring didokumentasikan)
- [ ] Password di-hash (bcrypt), tidak ada password/hash yang di-log; timestamps timestamptz
- [ ] Tests hijau incl. expiry/lockout/first-login/soft-delete cases
- [ ] Tidak ada secret hardcoded; `.env.example` diperbarui bila perlu
- [ ] ATM backend tetap flat JSON shape (field baru additive/opsional)
- [ ] Build bersih (`go build`, `sqlc generate`)
