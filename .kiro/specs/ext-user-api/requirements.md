# Requirements Document

> Status: accepted 2026-11-10 (PO)

## Introduction

### Masalah

Permintaan user (2026-11-10): "modul RBAC kita apakah bisa dibuatkan open API agar bisa di hit dari aplikasi lain. coba dulu untuk RBAC create, edit, delete User."

Pemanggilnya adalah aplikasi **IDM** milik CIMB. IDM berjalan di VM lokal (on-prem) CIMB, bukan di cloud, dan sedang dikembangkan supaya bisa memanggil API CROWN. Tugas spec ini: menyiapkan CROWN supaya bisa dipanggil oleh aplikasi lain, dimulai dari create, edit, dan delete user.

Hasil yang diinginkan: IDM bisa membuat, mengubah, dan menonaktifkan user CROWN lewat API dengan kredensial mesin miliknya sendiri, tanpa login sebagai manusia, tanpa bisa menyentuh akun admin, dan setiap perubahan tercatat di audit atas nama IDM.

### Keputusan PO (2026-11-10)

| ID | Keputusan |
| --- | --- |
| D1 | Pemanggil pertama: aplikasi IDM di VM on-prem CIMB. |
| D2 | Auth: OAuth2 client credentials. CROWN mendaftarkan `client_id`/`client_secret` per aplikasi; aplikasi menukarnya dengan token berumur pendek yang punya scope. |
| D3 | Namespace terpisah dari API internal: `/api/ext/v1/...`. |
| D4 | Delete = nonaktifkan (soft-delete). Tidak ada hapus permanen. |
| D5 | Client eksternal tidak boleh memberikan role `ADMIN` atau `APPACCESS`. |
| D6 | Pelaku di `audit_logs` = client app (IDM), bukan user yang diteruskan IDM. |
| D24 | (2026-11-10) Role `EXT-CLIENT` **tetap tampil** di layar Role Management (`ListRolesWithPermissions` tidak difilter). Diterima karena akun layanan tidak bisa login (`is_active=false` + `auth_source=service`), jadi hak menu yang diberikan ke role itu tidak berefek. Hanya form user admin yang menyembunyikannya (Req 9.6). |
| D23 | (2026-11-10) CI tidak dikerjakan di spec ini. Target CI = Jenkins (perkiraan Desember 2026); sampai saat itu deploy manual di Docker lokal dan gate kualitas dijalankan lokal (Req 12.7). `.github/workflows/ci.yml` tidak diubah. |
| D22 | API eksternal dibangun sebagai **modul Go terpisah `backend-extapi/`** (service, container, `.env`, dan port sendiri), bukan di dalam `backend`. CLI ClientRegistry ikut di modul itu. Aturan validasi user yang dipakai bersama dipindah ke `pkg/`. |

### Kondisi kode saat ini (dicek 2026-11-10)

**Endpoint user admin yang sudah ada**
- `backend/internal/handler/admin_user_handler.go` di-mount di `/api/v1/admin/users` (`backend/cmd/api/main.go`) dengan `RequireAuth` + `RequireRoles("APPACCESS", "ADMIN")`. Route: list, get, create (`POST /`), update (`PUT /{id}`), `POST /{id}/disable`, `POST /{id}/enable`, `POST /{id}/set-initial-password`, `GET /vendor-options`.
- `backend/internal/auth/user_admin.go` `UserAdminService.Create`/`Update`: cek ulang role pelaku (`userAdminActorRoles = APPACCESS, ADMIN`), validasi profil (full_name, email `net/mail`, role), aturan vendor (`vendorrole.IsVendor` → wajib `auth_source=local` + `vendor_id`), aturan `auth_source` (`ldap` tanpa password; `local` wajib `vendor_id` + `temporary_password` dari pemanggil), cek unik username/email/employee_id (409, plus pemetaan error 23505), cek referensi role/vendor/supervisor (400 `invalid_reference`). `username` dan `auth_source` tidak bisa diubah.
- **Tidak ada pembatasan role tujuan.** `resolveReferences` menerima role apa pun yang ada di tabel `roles`, termasuk `ADMIN` dan `APPACCESS`.
- Perubahan user langsung berlaku (tanpa `approval_requests`).
- **Audit ditulis di luar transaksi.** `repo.Create`/`Update` jalan di pool, lalu `audit.Write` terpisah. Bila audit gagal, row user tetap tersimpan dan API mengembalikan 500.
- `backend/internal/auth/deactivate_user.go`: `Deactivate` set `is_active=false, deleted_at=now()` lalu audit `user_deactivated`; juga di luar transaksi. Handler `Disable` menolak menonaktifkan diri sendiri dan cek keberadaan dulu. Menonaktifkan user yang sudah nonaktif berakhir 500.
- `backend/internal/repository/no_hard_delete_test.go` menjaga tidak ada `DELETE FROM users`.
- Bentuk error ATM backend: JSON datar `{error, message}` (`handler/error_response.go`), 422 `validation_error` dengan field.

**Token dan middleware**
- `pkg/auth/token_service.go`: HS256 dengan `JWT_SECRET` (min 32 byte). Claim akses: `id`, `username`, `role`, `is_karyawan`, `vendor_id?`, `supervisor_id?`, `approval_level?`, `iat`, `exp`. Tidak ada `iss`, `aud`, `sub`, `scope`, atau konsep principal non-user. `ValidateAccessToken` mewajibkan `id`, `username`, `role` tidak kosong. Token akses tidak bisa dicabut (tidak punya `jti`).
- `pkg/middleware/rbac.go`: `RequireAuth` memasukkan `AuthContext` (user); `RequireRoles` mencocokkan satu role.
- Tidak ada API key, client credentials, mTLS, IP allowlist, atau CORS di sisi Go. Rate limiter (`pkg/middleware/rate_limiter.go`) hanya untuk login, fail-closed 503 bila Redis mati.
- Satu-satunya auth antar-layanan: secret statis `DSR_RETRY_SCHEDULER_AUTH` dari Go ke Python. Python mode `api_key` tidak menulis audit karena `audit_logs.actor_id` wajib user.
- Middleware dasar: `TrustedRealIP(TRUSTED_PROXIES)`, `RequestLogger`, `Recoverer`, `Timeout(30s)`, `BodyLimit` 1 MiB.

**Audit dan users**
- `audit_logs.actor_id bigint NOT NULL` dengan FK `audit_logs_actor_fk` → `users(id)`. Tidak ada kolom tipe pelaku atau client. Pelaku non-user tidak bisa dicatat hari ini.
- `users`: satu `role_id` per user, `auth_source text NOT NULL` (tanpa CHECK; nilai di kode `ldap`, `local`, `local_dev`), `is_active`, `deleted_at`, `supervisor_id`, `approval_level`, kolom vendor.
- Login (`backend/internal/auth/service.go` `selectProvider`) menolak `auth_source` yang tidak dikenal provider mana pun (`ErrUnsupportedAuthSource`).

**Kontrak OpenAPI**
- `backend/api/openapi.yaml` (OpenAPI 3.0.3, `servers: /api/v1`, `bearerAuth`) belum mencakup `/admin/users`. Dites oleh `backend/internal/handler/openapi_contract_test.go` (kin-openapi, `RouteCoverage`, `Responses`, `RejectsDrift`).

**Migrasi**
- File terakhir `038_vendor_cis_limit.sql`. Nomor bebas berikutnya **`039`** (`development-plan.md` masih menulis `038`, sudah basi).

### Ruang lingkup

- Registrasi client mesin (ApiClient) dengan secret ter-hash, scope, dan daftar IP sumber.
- Endpoint token OAuth2 client credentials.
- Middleware auth eksternal yang terpisah dari `RequireAuth` internal.
- API eksternal user: create, update, disable (dan read/enable bila PO setuju di D11, D17).
- Penjaga role istimewa dan akun layanan.
- Audit atas nama client app.
- Kontrak OpenAPI terpisah untuk tim IDM, plus test kontrak.
- Konfigurasi dan dokumentasi deploy (flag, secret, jaringan).

### Tidak termasuk

- Endpoint eksternal untuk modul lain (role management, vendor, master data, approval). Spec ini pilot untuk user saja.
- Login SSO / federasi identitas dari IDM ke CROWN.
- Perubahan perilaku `/api/v1/admin/users` dan layar CompanyPortal, kecuali yang perlu untuk menyembunyikan akun layanan (Req 9).
- Memperbaiki audit di luar transaksi pada jalur admin internal (dicatat sebagai follow-up, bukan dikerjakan di sini).
- Layar admin untuk mengelola client, kecuali PO memilih opsi itu di D9.
- Pipeline CI untuk `backend-extapi` (GitHub Actions maupun Jenkins). Ditambahkan saat migrasi Jenkins (D23).
- Perubahan perilaku `backend-exq` dan `backend_python` (Dockerfile `backend-exq` hanya ikut menyalin `go.mod` modul baru karena `go.work`).

### Area sensitif (project-context Sec 3 rule 7)

- **Auth (tinggi).** Jalur auth baru untuk principal non-manusia, kunci tanda tangan baru, dan pembuatan user dari luar. Kesalahan di sini bisa membuat aplikasi luar membuat admin atau memakai token di API internal.
- **Migrasi.** Tabel baru untuk client (nomor `039`), role/akun layanan bila D16 = opsi a, atau perubahan `audit_logs` bila D16 = opsi b.
- **Data pribadi.** Nama, email, dan NIP karyawan dikirim dari VM on-prem ke CROWN; jalur jaringan perlu diputuskan bersama tim infra (D7).
- Tidak menyentuh uang, jurnal, atau rekonsiliasi.

## Glossary

- **IDM**: aplikasi identity management CIMB di VM on-prem, pemanggil pertama API ini.
- **ApiClient**: satu aplikasi luar yang terdaftar di CROWN (baris di tabel client baru): `client_id`, hash secret, nama, scope yang diizinkan, role yang boleh diberikan, CIDR sumber, status aktif/nonaktif.
- **Client_Secret**: secret acak milik ApiClient. Hanya ditampilkan sekali saat dibuat atau dirotasi.
- **ClientRegistry**: alat untuk membuat, merotasi, menonaktifkan, dan melihat ApiClient (bentuknya diputuskan di D9).
- **ExtApiService**: service Go baru `backend-extapi/` (modul workspace ketiga, container sendiri, port default `8082`) yang hanya melayani `/api/ext/v1/*`, `/health`, dan `/ready` (D22). Hanya service ini yang dibuka ke jalur jaringan IDM.
- **SharedUserRules**: paket `pkg/` berisi aturan validasi user tanpa akses DB (field wajib, format email, pemetaan constraint unik, Privileged_Role, konstanta akun layanan), dipakai `backend` dan ExtApiService.
- **EXT-CLIENT**: role baru tanpa hak menu untuk Service_Actor. Tidak bisa diberikan ke user mana pun lewat API.
- **ExtTokenEndpoint**: `POST /api/ext/v1/oauth/token`, menukar kredensial ApiClient dengan Client_Token.
- **Client_Token**: token akses berumur pendek untuk ApiClient, berisi `iss`, `aud`, `sub` (= `client_id`), `scope`, `jti`, `iat`, `exp`.
- **User_Token**: token akses yang dikeluarkan login user hari ini (`pkg/auth.TokenService`).
- **ExtAuthMiddleware**: middleware baru untuk semua route `/api/ext/v1/*` kecuali ExtTokenEndpoint.
- **ClientContext**: konteks request hasil ExtAuthMiddleware (ApiClient, scope, Service_Actor). Terpisah dari `AuthContext`.
- **ExtUserAPI**: route user eksternal di `/api/ext/v1/users`.
- **ExtUserService**: lapisan service yang menjalankan aturan ExtUserAPI dan memakai ulang aturan validasi `UserAdminService`.
- **Privileged_Role**: role yang tidak pernah boleh diberikan atau disentuh lewat ExtUserAPI: `ADMIN`, `APPACCESS` (D5), ditambah pilihan PO di D12.
- **Service_Actor**: identitas yang dicatat sebagai pelaku `audit_logs` untuk satu ApiClient (bentuknya diputuskan di D16).
- **Managed_User**: user yang boleh disentuh ExtUserAPI: bukan Service_Actor, role saat ini bukan Privileged_Role, dan sesuai jenis user di D13.
- **ExtContract**: file OpenAPI 3.0.3 terpisah untuk ExtUserAPI dan ExtTokenEndpoint (`backend-extapi/api/openapi.yaml`).
- **AdminUserAPI**: `/api/v1/admin/users` yang sudah ada.

## Requirements

### Requirement 1: Registrasi ApiClient

**User Story:** Sebagai tim operasional CROWN, saya ingin mendaftarkan aplikasi luar dengan kredensial dan hak terbatas, supaya hanya aplikasi yang kami setujui yang bisa memanggil API eksternal.

#### Acceptance Criteria

1. THE ClientRegistry SHALL store each ApiClient with a random non-secret `client_id`, a display name, a hash of the Client_Secret, the allowed scopes, the roles it may assign (D12), the allowed source CIDRs, its Service_Actor, an active flag, and creation, rotation, and disable timestamps.
2. WHEN an ApiClient is created or its secret is rotated, THE ClientRegistry SHALL generate a Client_Secret of at least 32 random bytes from a cryptographically secure source, show it to the operator exactly once, and store only a slow one-way hash of it.
3. THE ClientRegistry SHALL never store, log, or return a Client_Secret after the create or rotate output.
4. WHEN a secret is rotated, THE ExtTokenEndpoint SHALL reject the previous secret from that moment.
5. WHEN an ApiClient is disabled, THE ExtAuthMiddleware SHALL reject every Client_Token of that client from the next request, without waiting for token expiry.
6. WHEN an ApiClient is created, rotated, enabled, disabled, or has its scopes, roles, or CIDRs changed, THE ClientRegistry SHALL write an `audit_logs` entry whose payload contains no secret or hash.
7. IF an ApiClient has an empty CIDR list, THEN THE ExtTokenEndpoint and ExtAuthMiddleware SHALL reject all requests for that client.

### Requirement 2: Endpoint token client credentials

**User Story:** Sebagai developer IDM, saya ingin menukar `client_id`/`client_secret` dengan token akses standar OAuth2, supaya IDM bisa memakai library OAuth2 yang umum.

#### Acceptance Criteria

1. WHEN `POST /api/ext/v1/oauth/token` receives `grant_type=client_credentials` (form-urlencoded) with valid credentials of an active ApiClient in an HTTP Basic `Authorization` header, from an IP inside that client's CIDRs, THE ExtTokenEndpoint SHALL respond 200 with `access_token`, `token_type` = `Bearer`, `expires_in` in seconds, and the granted `scope`.
2. WHEN the request has a `scope` parameter, THE ExtTokenEndpoint SHALL grant it only if every requested scope is in the client's allowed scopes; WHEN `scope` is absent, THE ExtTokenEndpoint SHALL grant all allowed scopes.
3. IF a requested scope is not allowed for the client, THEN THE ExtTokenEndpoint SHALL respond 400 with `{"error":"invalid_scope"}`.
4. IF the client is unknown, disabled, outside its CIDRs, or the secret does not match, THEN THE ExtTokenEndpoint SHALL respond 401 with the same `{"error":"invalid_client"}` body and a `WWW-Authenticate: Basic` header for every one of these causes, and SHALL compare secrets in constant time.
5. IF `grant_type` is missing or not `client_credentials`, THEN THE ExtTokenEndpoint SHALL respond 400 with `{"error":"unsupported_grant_type"}`; IF the body is malformed, THEN it SHALL respond 400 with `{"error":"invalid_request"}`.
6. THE ExtTokenEndpoint SHALL NOT issue a refresh token.
7. THE ExtTokenEndpoint SHALL set `Cache-Control: no-store` and `Pragma: no-cache` on every response.
8. THE Client_Token lifetime SHALL be the configured value (D8) and SHALL NOT exceed 60 minutes.
9. WHEN failed token requests for one `client_id` or one source IP exceed the configured limit within the window (D19), THE ExtTokenEndpoint SHALL respond 429 with `Retry-After` until the window ends; IF the rate-limit store is unavailable, THEN it SHALL respond 503 (fail closed, same as login).
10. THE ExtTokenEndpoint SHALL log every issued and failed token request with `client_id` (when present), source IP, and outcome, and SHALL NOT log the secret, the `Authorization` header, or the token.

### Requirement 3: Isolasi token eksternal dan internal

**User Story:** Sebagai security reviewer, saya ingin token aplikasi luar dan token user tidak bisa saling dipakai, supaya kebocoran salah satu tidak membuka API yang lain.

#### Acceptance Criteria

1. THE Client_Token SHALL be signed with a key separate from `JWT_SECRET` (D10) and SHALL carry `iss`, `aud` = `crown-ext`, `sub` = `client_id`, `scope`, `jti`, `iat`, and `exp`.
2. WHEN a Client_Token is sent to any `/api/v1/*` route, THE existing `RequireAuth` SHALL respond 401.
3. WHEN a User_Token is sent to any `/api/ext/v1/*` route, THE ExtAuthMiddleware SHALL respond 401.
4. THE ExtApiService SHALL refuse to start IF the external signing key is missing or shorter than 32 bytes, or IF `JWT_SECRET` is present in its environment (the service never holds the user-token key, so it cannot mint User_Tokens).
5. THE `/api/ext/v1/*` routes SHALL be served only by the ExtApiService; `backend` (port 8080, behind both portal Nginx proxies) and `backend-exq` SHALL NOT mount them, so `/api/ext/*` returns 404 there.
6. THE ExtAuthMiddleware SHALL put only a ClientContext in the request context and SHALL NOT create an `AuthContext`, so no existing handler that reads `AuthContext` accepts a Client_Token.

### Requirement 4: Validasi setiap request eksternal

**User Story:** Sebagai security reviewer, saya ingin setiap panggilan API eksternal diperiksa ulang (token, status client, IP, scope), supaya client yang dicabut atau dicuri kredensialnya segera terhenti.

#### Acceptance Criteria

1. WHEN a request reaches an ExtUserAPI route, THE ExtAuthMiddleware SHALL verify the Bearer token signature (HMAC only), `exp`, `iss`, and `aud`; IF any check fails or the header is missing, THEN it SHALL respond 401 with `{"error":"invalid_token"}` and `WWW-Authenticate: Bearer error="invalid_token"`.
2. WHEN the token is valid, THE ExtAuthMiddleware SHALL load the ApiClient by `sub` from the primary database and SHALL respond 401 `invalid_token` IF the client is missing or disabled.
3. IF the source IP, resolved by the existing `TrustedRealIP`, is outside the client's CIDRs, THEN THE ExtAuthMiddleware SHALL respond 403 with `{"error":"ip_not_allowed"}`.
4. IF the token lacks the scope the route requires (`users:write` for writes, `users:read` for reads), THEN THE ExtAuthMiddleware SHALL respond 403 with `{"error":"insufficient_scope"}` and `WWW-Authenticate: Bearer error="insufficient_scope"`.
5. WHEN one ApiClient exceeds the configured request rate (D19), THE ExtAuthMiddleware SHALL respond 429 with `Retry-After`; IF the rate-limit store is unavailable, THEN it SHALL respond 503.
6. THE ExtUserAPI SHALL keep the existing body limit (1 MiB), the 30 s timeout, and the `RequestLogger`, and the request log SHALL include `client_id`.

### Requirement 5: Membuat user

**User Story:** Sebagai IDM, saya ingin membuat user CROWN untuk karyawan yang disetujui di IDM, supaya akses CROWN tidak perlu dibuat manual oleh admin.

#### Acceptance Criteria

1. WHEN `POST /api/ext/v1/users` with scope `users:write` receives a valid body, THE ExtUserService SHALL create the user and respond 201 with `id`, `username`, `full_name`, `email`, `role` (name), `employee_id`, `supervisor_id`, `approval_level`, `is_active`, and `created_at`.
2. THE ExtUserAPI SHALL accept only the fields allowed for the user type chosen in D13 and SHALL respond 400 `bad_request` naming the field WHEN the body contains an unknown field.
3. THE ExtUserService SHALL apply the same validation as AdminUserAPI create (required fields, email format, uniqueness of username, email, and employee_id, role/supervisor references) and SHALL return the same status codes (422 `validation_error`, 409 `conflict`, 400 `invalid_reference`).
4. IF the requested role is a Privileged_Role or is not in the client's assignable roles, THEN THE ExtUserService SHALL respond 403 with `{"error":"role_not_allowed"}` and create nothing.
5. IF `supervisor_id` points to a user who is not a Managed_User, THEN THE ExtUserService SHALL respond 400 `invalid_reference`.
6. THE ExtUserService SHALL never set a password for a user it creates.

### Requirement 6: Mengubah user

**User Story:** Sebagai IDM, saya ingin mengubah data dan role user CROWN saat data karyawan berubah, supaya akses CROWN mengikuti IDM.

#### Acceptance Criteria

1. WHEN `PUT /api/ext/v1/users/{id}` with scope `users:write` receives a valid body for an active Managed_User, THE ExtUserService SHALL update the editable fields and respond 200 with the same representation as Req 5.1.
2. IF the body tries to change `username` or `auth_source`, THEN THE ExtUserService SHALL respond 400 `bad_request` and change nothing.
3. IF the target does not exist, is disabled, or is a Service_Actor, THEN THE ExtUserService SHALL respond 404 `not_found`.
4. IF the target's current role is a Privileged_Role, or the new role is a Privileged_Role or not in the client's assignable roles, THEN THE ExtUserService SHALL respond 403 `role_not_allowed` and change nothing.
5. THE ExtUserService SHALL apply the same validation and status codes as AdminUserAPI update (Req 5.3), including the self-supervision rule.

### Requirement 7: Menonaktifkan user (delete)

**User Story:** Sebagai IDM, saya ingin menonaktifkan user CROWN saat karyawan keluar atau pindah, supaya aksesnya tertutup tanpa menghapus jejaknya.

#### Acceptance Criteria

1. WHEN the disable endpoint (form decided in D14) with scope `users:write` targets an active Managed_User, THE ExtUserService SHALL set `is_active = false` and `deleted_at = now()` and respond 204.
2. WHEN the target is already disabled, THE ExtUserService SHALL respond 204 without changing the row and without a new audit entry.
3. IF the target does not exist or is a Service_Actor, THEN THE ExtUserService SHALL respond 404 `not_found`.
4. IF the target's role is a Privileged_Role, THEN THE ExtUserService SHALL respond 403 `role_not_allowed`.
5. THE ExtUserService SHALL never hard-delete a `users` row; `no_hard_delete_test.go` SHALL keep passing.
6. THE effect of an external disable on the user's login and existing sessions SHALL be the same as a disable through AdminUserAPI.

### Requirement 8: Membaca dan mengaktifkan kembali user (bergantung D11, D17)

**User Story:** Sebagai IDM, saya ingin mencari user berdasarkan username dan mengaktifkan kembali user yang pernah dinonaktifkan, supaya IDM bisa memastikan hasil setelah timeout dan menangani karyawan yang kembali.

#### Acceptance Criteria

1. WHERE D11 is approved, WHEN `GET /api/ext/v1/users/{id}` or `GET /api/ext/v1/users?username=<u>` is called with scope `users:read`, THE ExtUserAPI SHALL return the Req 5.1 representation of a matching Managed_User, including disabled ones, and 404 otherwise.
2. WHERE D17 is approved, WHEN `POST /api/ext/v1/users/{id}/enable` with scope `users:write` targets a disabled Managed_User, THE ExtUserService SHALL set `is_active = true`, clear `deleted_at`, and respond 200; an already active target SHALL get 200 without a new audit entry.
3. WHERE D17 is approved, IF the enable target does not exist, is a Service_Actor, or has a Privileged_Role, THEN THE ExtUserService SHALL respond 404 or 403 by the same rules as Req 7.3 and 7.4.

### Requirement 9: Penjaga role istimewa dan akun layanan

**User Story:** Sebagai pemilik RBAC CROWN, saya ingin aplikasi luar tidak pernah bisa membuat, mengubah, atau menonaktifkan admin dan akun layanan, supaya kredensial client yang bocor tidak bisa dipakai mengambil alih CROWN.

#### Acceptance Criteria

1. THE Privileged_Role set SHALL be defined in code, not in configuration or database, and SHALL contain at least `ADMIN` and `APPACCESS` (D5), plus any role PO adds in D12.
2. THE ClientRegistry SHALL refuse to save an ApiClient whose assignable roles include a Privileged_Role.
3. THE ExtUserService SHALL enforce Req 5.4, 6.4, 7.4, and 8.3 in the service layer, independent of any route guard.
4. THE Service_Actor of any ApiClient SHALL be unable to log in through any login route and SHALL NOT be returned, changed, or disabled by ExtUserAPI.
5. WHERE D16 is option a (service user row), THE AdminUserAPI list SHALL exclude Service_Actor rows and THE AdminUserAPI get, update, disable, enable, and set-initial-password SHALL respond 404 for a Service_Actor.
6. WHERE D16 is option a, THE AdminUserAPI create and update SHALL respond 400 `invalid_reference` WHEN the role is `EXT-CLIENT` or `supervisor_id` points to a Service_Actor.

### Requirement 10: Audit atas nama client app

**User Story:** Sebagai auditor, saya ingin setiap perubahan user dari IDM tercatat dengan IDM sebagai pelaku, sebelum dan sesudahnya, supaya jejak perubahan bisa ditelusuri.

#### Acceptance Criteria

1. WHEN ExtUserService creates, updates, disables, or enables a user, THE ExtUserService SHALL write an `audit_logs` entry with the Service_Actor of the calling ApiClient as actor (D6), the source IP, and `before`/`after` snapshots, using the same action names as AdminUserAPI (`user_created`, `user_updated`, `user_deactivated`, `user_reactivated`).
2. THE ExtUserService SHALL write the user change and its audit entry in the same database transaction, so a failed audit write leaves no change (D15).
3. THE audit payload SHALL contain no password, hash, secret, or token.
4. THE ClientRegistry list output SHALL show each ApiClient's Service_Actor id and username, so an entry shown as actor `#<id>` in the existing Audit Log viewer can be traced to its client; THE Audit Log viewer itself SHALL stay unchanged.
5. WHEN ExtUserAPI rejects a request (4xx), THE ExtApiService SHALL log it with `client_id`, route, and error code, and SHALL NOT write an `audit_logs` entry.

### Requirement 11: Kontrak OpenAPI untuk tim IDM

**User Story:** Sebagai developer IDM, saya ingin dokumen OpenAPI yang akurat, supaya saya bisa membangun integrasi tanpa menebak bentuk request dan error.

#### Acceptance Criteria

1. THE ExtContract SHALL be a separate OpenAPI 3.0.3 file with `servers` = `/api/ext/v1`, an `oauth2` `clientCredentials` security scheme with the token URL and scopes, and every ExtUserAPI and ExtTokenEndpoint operation, request body, response, and error shape.
2. THE backend test suite SHALL validate ExtUserAPI and ExtTokenEndpoint responses against the ExtContract, check that every mounted `/api/ext/v1` route is in the ExtContract and the reverse, and fail on drift, following `openapi_contract_test.go`.
3. THE ExtContract SHALL state that `/api/ext/v1` is versioned and that a breaking change requires a new version path.
4. THE ExtContract SHALL NOT be served by the running API (D21); it is shared with the IDM team as a file.

### Requirement 12: Konfigurasi, deploy, dan tidak ada regresi

**User Story:** Sebagai tim operasional, saya ingin API eksternal berjalan sebagai service sendiri yang terdokumentasi, supaya bisa dinyalakan, dimatikan, dan di-rollback tanpa menyentuh `backend`.

#### Acceptance Criteria

1. THE ExtApiService SHALL read its database URL, Redis URL, port, trusted proxies, external signing key, token lifetime, and rate limits from its own environment, documented in `backend-extapi/.env.example`; it SHALL run only when its own compose service is started, and stopping it SHALL NOT affect `backend` or `backend-exq`.
2. THE deployment guide (`.claude/docs/deployment.md`) SHALL describe how to start and stop the ExtApiService, create the IDM ApiClient, hand over the secret, rotate it, set `TRUSTED_PROXIES`, and an optional least-privilege database login for the service; the network path stays a placeholder until D7 is settled.
3. THE AdminUserAPI routes, request and response shapes, guards, and CompanyPortal user screens SHALL behave as before, except Req 9.5 and 9.6; their existing tests SHALL pass unchanged or with only additions.
4. THE existing `RequireAuth` behavior for User_Tokens and the existing `backend/api/openapi.yaml` contract test SHALL stay unchanged.
5. New Go code in `backend-extapi/internal/*` and `pkg/*` for this feature SHALL have at least 80% test coverage, including tests for token isolation, scope and IP denial, privileged-role denial, Service_Actor protection, and audit in the same transaction.
6. THE `backend` user admin code and the ExtApiService SHALL both use the SharedUserRules for profile validation, email format, unique-constraint mapping, and Privileged_Role, so the two paths cannot diverge; moving these rules SHALL NOT change any AdminUserAPI response.
7. THE local quality gate SHALL run vet, lint, govulncheck, unit tests, integration tests with coverage thresholds, the sqlc drift check, and the image check (non-root user + HEALTHCHECK) for `backend-extapi`, like the existing modules, and SHALL pass before the feature is done; `backend` and `backend-exq` images SHALL keep building with the new workspace module. Wiring these checks into a CI pipeline is out of scope (D23).

### Requirement 13: Paket informasi koneksi untuk tim IDM

**User Story:** Sebagai tim CROWN, saya ingin menyerahkan satu paket informasi yang lengkap ke tim IDM, supaya IDM bisa menyesuaikan integrasinya sendiri tanpa bertanya ulang ke kami (D7).

#### Acceptance Criteria

1. THE repository SHALL contain an IDM integration guide (Markdown, Indonesian) next to the ExtContract that covers: base URL per environment (placeholder until D7 is settled), the token flow with request and response examples, how to call each user endpoint with examples, every error code with its meaning and the expected client action, token lifetime and renewal, rate limits and `Retry-After`, the retry rule after a timeout (check with `GET ?username=` before resending a create), and that DELETE is a soft-delete.
2. THE guide SHALL list what CROWN needs from the IDM team before go-live: source IP or CIDR, a contact for secret handover, and the roles IDM needs to assign.
3. THE guide and the ExtContract SHALL contain no real secret, `client_id`, internal hostname, or IP address; examples SHALL use clearly fake values.
4. WHEN the ExtContract changes, THE guide SHALL be updated in the same change (checked during review).

## Open Questions

Status: D1–D6 ✅ diputuskan PO 2026-11-10 (lihat Introduction). D8–D21 ✅ **diputuskan PO 2026-11-10, mengikuti saran Kiro.** D7 ✅ **diputuskan PO 2026-11-10: di luar scope spec ini.** CROWN menyiapkan API dan paket informasi koneksi (Req 13); jaringan dan penyesuaian di sisi IDM dikerjakan kemudian oleh tim IDM dan infra. Cek CIDR di aplikasi (Req 1.7, 4.3) tetap dibangun; nilai CIDR diisi saat onboarding. D20: dicek di kode 2026-11-10, refresh token (`GetUserProfile` memfilter `deleted_at IS NULL`) sudah menolak user nonaktif, jadi sesi user yang dinonaktifkan berakhir paling lama saat token akses 15 menit habis.

| ID | Pertanyaan | Opsi | Saran Kiro | Dampak bila PO memilih lain |
| --- | --- | --- | --- | --- |
| D7 | Jalur jaringan IDM (on-prem) → CROWN (GCP Compute Engine) | (a) jalur privat (Interconnect/VPN) + firewall hanya IP IDM; (b) internet lewat Load Balancer + Cloud Armor + TLS | **(a)**, plus cek CIDR di aplikasi (Req 1.7, 4.3) sebagai lapisan kedua. Perlu IP sumber IDM dan konfirmasi jalur dari tim infra/network CIMB. Tidak menghambat requirements, tapi menghambat deploy. | (b): Cloud Armor rule wajib, dan pertimbangkan mTLS. Req 12.2 menyesuaikan. |
| D8 | Umur Client_Token | 5, 15, atau 60 menit | **15 menit**, sama dengan token akses user. IDM cukup minta token baru saat habis. | Req 2.8 tetap membatasi maksimum 60 menit. |
| D9 | Cara mengelola ApiClient | (a) CLI (sejak D22: `backend-extapi/cmd/extclient`) dijalankan operator, dengan `--actor <username APPACCESS>` untuk audit; (b) layar admin di CompanyPortal untuk APPACCESS, dengan maker-checker | **(a) CLI** untuk pilot (satu client, jarang berubah, tanpa UI baru). Layar admin bisa menyusul bila client bertambah. | (b): tambah requirement UI, endpoint admin, dan alur approval; scope lebih besar. |
| D10 | Kunci tanda tangan Client_Token | (a) HS256 dengan secret terpisah `EXT_JWT_SECRET`; (b) pakai `JWT_SECRET` dengan `aud` berbeda; (c) RS256 | **(a).** Token cukup diverifikasi CROWN sendiri; IDM memperlakukannya sebagai token buram. Kunci terpisah berarti kebocoran salah satu tidak membuka yang lain. | (b) melemahkan Req 3.1/3.4. (c) butuh pengelolaan key pair, belum perlu. |
| D11 | Endpoint baca (GET by id, GET by username) | Ya, atau tidak | **Ya**, scope `users:read`. Tanpa ini IDM tidak bisa memastikan apakah create yang timeout berhasil, selain dari 409. | Req 8.1 dihapus. |
| D12 | Role istimewa tambahan dan daftar role per client | Tambah `ADMIN_PARAM`? Pakai daftar role yang boleh diberikan per client? | **Tambah `ADMIN_PARAM`** (admin master data) ke Privileged_Role, dan **wajibkan daftar role per client** (allowlist), dengan Privileged_Role sebagai batas yang tidak bisa ditembus. Allowlist mencegah role baru di masa depan otomatis terbuka untuk IDM. | Tanpa allowlist: semua role non-istimewa terbuka, termasuk role baru nanti. Req 1.1, 5.4, 6.4 menyesuaikan. |
| D13 | Jenis user yang dikelola IDM | (a) hanya karyawan internal `auth_source=ldap`; (b) juga user vendor `local` | **(a).** IDM mengelola karyawan. User vendor butuh `vendor_id`, cabang, dan password sementara yang harus dikirim lewat API. Body create = `username`, `full_name`, `email`, `role`, `employee_id`, `supervisor_id?`, `approval_level?`; `auth_source=ldap` dan `is_karyawan=true` diisi server. | (b): password sementara lewat API, aturan vendor ikut, scope dan risiko lebih besar. |
| D14 | Bentuk endpoint delete | (a) `DELETE /api/ext/v1/users/{id}` (soft, 204); (b) `POST /api/ext/v1/users/{id}/disable` | **(a)**, karena itu yang dicari developer integrasi untuk "delete user". ExtContract menjelaskan bahwa ini soft-delete (D4). | (b): Req 7.1 dan ExtContract menyesuaikan. |
| D15 | Audit dalam transaksi yang sama | Ya untuk jalur eksternal; jalur admin tetap | **Ya untuk jalur eksternal.** Perbaikan jalur admin (audit di luar transaksi hari ini) dicatat sebagai follow-up terpisah, supaya spec ini tidak mengubah perilaku admin. | Bila PO ingin jalur admin ikut diperbaiki: tambah requirement dan test untuk AdminUserAPI. |
| D16 | Bentuk Service_Actor (pelaku audit = client app, D6) | (a) satu baris `users` per client (mis. `svc-idm`), `auth_source='service'`, tanpa password, role baru tanpa hak (`EXT-CLIENT`); (b) kolom baru `audit_logs.actor_client_id`, `actor_id` boleh NULL, CHECK tepat satu terisi | **(a).** Tidak mengubah `audit_logs` (dipakai `backend`, `backend-exq`, Python, dan viewer, semuanya mengasumsikan `actor_id` user). Login sudah menolak `auth_source` yang tidak dikenal. Biaya: akun layanan harus disembunyikan dan dilindungi di AdminUserAPI (Req 9.5). | (b): model data lebih bersih, tapi menyentuh tipe sqlc `audit_logs` di dua modul Go, query viewer, dan penulis audit Python. Req 9.5 hilang, Req 10.4 berubah. |
| D17 | Endpoint enable (aktifkan kembali) | Ya, atau tidak | **Ya.** User nonaktif tetap memegang `username` dan `email`, jadi tanpa enable IDM tidak bisa memulihkan karyawan yang kembali (create akan 409). | Req 8.2–8.3 dihapus; pemulihan lewat admin CROWN. |
| D18 | Maker-checker untuk perubahan dari IDM | (a) langsung berlaku + audit (sama dengan AdminUserAPI hari ini); (b) lewat `approval_requests` di CROWN | **(a).** Persetujuan akses sudah terjadi di IDM; manajemen user CROWN hari ini juga langsung berlaku. Dicatat sebagai deviasi GR#3 di CLAUDE.md Sec 12 bersama alasan ini. | (b): perlu `document_type` baru, policy, applier, dan IDM harus menangani status 202 + polling. |
| D19 | Batas rate | Token: gagal per client/IP per jendela; API: request per client per menit | **Token: 10 gagal per 15 menit per `client_id` dan per IP. API: 60 request per menit per client.** Bisa diubah lewat env. | Angka di Req 2.9, 4.5 menyesuaikan. |
| D20 | Sesi aktif user yang dinonaktifkan | Ikut perilaku disable admin hari ini, atau cabut sesi segera | **Ikut perilaku admin hari ini** (Req 7.6). Design akan mengecek apakah refresh token menolak user nonaktif; bila tidak, itu celah yang sama di AdminUserAPI dan dicatat sebagai follow-up. | Cabut segera: butuh daftar blokir token akses per user, perubahan di `pkg/auth`. |
| D21 | ExtContract disajikan oleh API? | Ya (`GET /api/ext/v1/openapi.yaml`), atau file saja | **File saja**, diberikan ke tim IDM. Tidak membuka peta API ke siapa pun yang bisa menjangkau endpoint. | Ya: tambah route publik dan Req 11.4 berubah. |

## Dependency & Scope Note

- **Scope guard:** item baru **X.17** di tabel cross-cutting `development-plan.md` (user OK 2026-11-10 untuk membuka item dan menulis spec). Build menunggu persetujuan requirements, design, dan tasks.
- **Stack:** tidak butuh library baru. JWT (`golang-jwt/jwt/v5`), bcrypt/`crypto/rand`, Redis (`go-redis/v9`), dan kin-openapi sudah ada. Bila design memilih argon2 untuk hash secret, itu dependensi baru dan butuh persetujuan (GR#1).
- **Migrasi:** `039` untuk tabel ApiClient dan role akun layanan, tetap di `backend/migrations/` (satu-satunya pemilik migrasi). ExtApiService membaca skema yang sama lewat sqlc, seperti `backend-exq`.
- **Topologi (D22):** modul workspace ketiga. `structure.md`, `tech.md`, `project-context.md`, dan `.claude/CLAUDE.md` diperbarui (aturan "backend tidak saling import" berlaku juga untuk `backend-extapi`).
- **Pihak luar:** tim IDM menyesuaikan diri dengan kontrak dan panduan koneksi dari CROWN (Req 13). Tim infra/network CIMB menentukan jalur jaringan, IP sumber, dan TLS saat deploy (D7, di luar scope).
- **Follow-up yang tidak dikerjakan di sini:** audit di luar transaksi pada AdminUserAPI dan `DeactivateUserService`; disable user yang sudah nonaktif di AdminUserAPI berakhir 500.
