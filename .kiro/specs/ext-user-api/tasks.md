# Implementation Plan

> Status: accepted 2026-11-10 (engineer)

## Overview

Urutan pengerjaan mengikuti `design.md`: skema dulu, lalu aturan bersama di `pkg/`, lalu penjagaan akun layanan di `backend`, lalu modul baru `backend-extapi` dari bawah ke atas (config → repository → `extauth` → `extuser` → handler → CLI), lalu kontrak, image, dan dokumen. CI tidak dikerjakan (D23); gate kualitas dijalankan lokal di task 12. Setiap langkah meninggalkan workspace dalam keadaan bisa di-build: `go.work` dan baris `COPY` di Dockerfile `backend`/`backend-exq` diubah dalam task yang sama (4.1), supaya image dua backend lama tidak rusak di tengah jalan.

Referensi pola yang dipakai ulang:
- Modul terpisah: `backend-exq/` (`go.mod`, `sqlc.yaml`, `cmd/api/main.go`, Dockerfile, compose, `coverage-thresholds.txt`).
- Transaksi + audit: `backend/internal/service/holiday_admin.go` `inTx`.
- Rate limit Redis fail-closed: `pkg/middleware/rate_limiter.go`.
- Kontrak OpenAPI: `backend/internal/handler/openapi_contract_test.go`.
- Migrasi: `backend/cmd/migrate` (`go run ./cmd/migrate up`, jangan `psql -f`).

### Model Split

**Sonnet-runnable**
- 1. Migrasi `039` — Sonnet, Medium
- 2. Aturan bersama + perubahan kecil `pkg/` — Sonnet, Medium
- 3.5 Test penjagaan akun layanan di `backend` — Sonnet, Medium (butuh 3.4)
- 3.6 Test statis tidak ada route `/api/ext` di `backend`/`backend-exq` — Sonnet, Low
- 4. Kerangka modul `backend-extapi`, config, repository — Sonnet, Medium (butuh 1, 2)
- 7.2 Handler user + pemetaan error — Sonnet, Medium (butuh Opus 6.1, Opus 5.5)
- 7.3 Router + `cmd/api/main.go` — Sonnet, Medium (butuh 7.1, 7.2)
- 7.4 Test handler, router, kebocoran secret — Sonnet, Medium (butuh 7.3)
- 9. ExtContract + test kontrak — Sonnet, Medium (butuh 7.3)
- 10. Dockerfile dan compose — Sonnet, Medium (butuh 7.3, 8.1)
- 11.1 Panduan integrasi IDM — Sonnet, Medium (butuh 9.1)
- 11.2 Panduan deploy — Sonnet, Medium (butuh 10.1)
- 12. Checkpoint gate kualitas — Sonnet, Medium
- 13. Cek manual (user) — Sonnet, Low

**Haiku-runnable**
- 11.3 Dokumen topologi dan progres — Haiku, Low (pembaruan dokumen mekanis, butuh 1.2, 11.2)

**Opus-required**
- 3. Penjagaan akun layanan di kode admin user `backend` (3.1–3.4) — Opus, Medium — auth: mengubah kode admin user yang sudah jalan; salah alias tipe atau salah filter membuka akun layanan atau merusak admin.
- 5. Paket `extauth` (token, secret, rate limit, middleware) — Opus, High — auth: isolasi token, verifikasi secret, cek client/IP/scope.
- 6. Service `extuser` (create/update/disable/enable + audit dalam transaksi) — Opus, High — auth + RBAC: role istimewa, akun layanan, `FOR UPDATE`, audit atomik.
- 7.1 Endpoint token — Opus, High — auth: pesan gagal seragam, dummy hash, rate limit gagal.
- 8. CLI `extclient` — Opus, High — auth: membuat dan merotasi kredensial, akun layanan, allowlist role.

**Dependensi lintas grup:** Sonnet 4.3 butuh Sonnet 1.1 (skema untuk sqlc). Opus 5.1 butuh Sonnet 2.1 dan 4.1. Opus 6.1 butuh Sonnet 4.3. Sonnet 7.2 butuh Opus 5.5 dan 6.1. Sonnet 10.1 butuh Opus 8.1. Sonnet 3.5 butuh Opus 3.4.

## Tasks

- [ ] 1. Migrasi `039_ext_api_clients`
  - [x] 1.1 Tulis `backend/migrations/039_ext_api_clients.sql` sesuai design `## Data Models`: role `EXT-CLIENT` (tanpa `role_permissions`), komentar `users.auth_source`, tabel `api_clients` dengan CHECK `client_id`, `name`, `credential_version`, `scopes`, dan `assignable_roles` (cermin `PrivilegedRoles`), `allowed_cidrs cidr[]`, FK `service_user_id` (UNIQUE) dan `created_by` ke `users`. Cek ulang nomor `039` belum terpakai.
    - _Requirements: 1.1, 9.2, 9.4_
  - [x] 1.2 Terapkan ke dev dengan `go run ./cmd/migrate up` dari `backend/` (DB lewat `localhost:5432`), cek `go run ./cmd/migrate status`, `\d api_clients`, dan `SELECT * FROM roles WHERE role = 'EXT-CLIENT'`. Bila DB dev tidak terjangkau, catat di `## Notes` → `### Test Results` sebagai outstanding.
    - _Requirements: 1.1_
  - _Model: Sonnet, Medium — skema aditif, tidak menyentuh baris yang ada; CHECK role istimewa dijaga test sinkronisasi di 4.4._

- [ ] 2. Aturan bersama dan perubahan kecil di `pkg/`
  - [x] 2.1 Buat `pkg/userrules/userrules.go`: konstanta (`AuthSourceLDAP`, `AuthSourceService`, `ServiceRole`, `PgUniqueViolation`), `PrivilegedRoles` (`ADMIN`, `APPACCESS`, `ADMIN_PARAM`, `EXT-CLIENT`), `IsPrivileged`, `ValidateProfile` (urutan field dan pesan sama persis dengan `validateProfile` di `backend`), `IsValidEmail`, `Uniqueness`, `ConflictFieldFromConstraint`, `NilIfEmpty`, `ConflictError`, `ReferenceError`. Tulis `userrules_test.go` (tabel kasus disalin dari test admin) dan `userrules_property_test.go` (rapid, **Property 11**: hasil sama dengan salinan fungsi lama di file test). Tambah `github.com/cimb-niaga/cms/pkg/userrules 100` ke `pkg/coverage-thresholds.txt` (format import path penuh, sama dengan baris yang ada).
    - _Requirements: 5.3, 6.5, 9.1, 12.6_
  - [x] 2.2 Ekspor `config.ParseTrustedProxies` di `pkg/config/config.go` (pembungkus fungsi yang ada, `Load` tidak berubah) + test.
    - _Requirements: 12.1_
  - [x] 2.3 `pkg/middleware/request_logger.go`: holder `LogFields` di context (`WithLogFields`/`LogFieldsFrom`), atribut `client_id` ditulis hanya bila terisi; baris log yang ada tidak berubah. Test: holder terisi → atribut ada; tidak terisi → baris sama seperti sebelumnya; nilai tidak pernah dibaca dari header request.
    - _Requirements: 4.6, 10.5_
  - _Model: Sonnet, Medium — kode murni tanpa DB; risiko utama perubahan pesan validasi, dijaga property test golden._

- [ ] 3. Penjagaan akun layanan di `backend`
  - [x] 3.1 Refactor `backend/internal/auth/user_admin.go` supaya memakai `pkg/userrules`: alias `type ConflictError = userrules.ConflictError` dan `type ReferenceError = userrules.ReferenceError`; `validateProfile`, `isValidEmail`, `uniqueness`, `nilIfEmpty` diganti panggilan `userrules`; `conflictFieldFromPgError` tetap di `backend` dan memanggil `ConflictFieldFromConstraint`. Jalankan `go test ./internal/auth ./internal/handler` tanpa mengubah test yang ada.
    - _Requirements: 12.3, 12.6_
  - [x] 3.2 `backend/queries/users_admin.sql`: `ListUsersAdmin` dan `CountUsersAdmin` dengan `AND u.auth_source <> 'service'`; `UserAdminExists` dengan `AND auth_source <> 'service'`; query daftar role untuk form dengan `WHERE role <> 'EXT-CLIENT'`. `sqlc generate` (v1.31.1) lalu perbaikan `UserLeafe → UserLeave`.
    - _Requirements: 9.5, 9.6_
  - [x] 3.3 `backend/internal/auth/user_admin.go`: `resolveReferences` menolak role `userrules.ServiceRole` dengan `ReferenceError{role, "tidak dapat diberikan"}`; `Update` mengembalikan `ErrUserNotFound` untuk target `AuthSource == userrules.AuthSourceService`.
    - _Requirements: 9.5, 9.6_
  - [x] 3.4 `backend/internal/handler/admin_user_handler.go`: `Get`, `Update`, `Disable`, `Enable`, `SetInitialPassword` mengembalikan 404 `not_found` untuk baris akun layanan (pre-check `userAdminSvc.Get`; `Update` dan `SetInitialPassword` mendapat pre-check baru).
    - _Requirements: 9.5_
  - [x] 3.5 Test (**Property 4** sisi admin dan login): handler admin 404 untuk akun layanan di lima endpoint; create/update dengan role `EXT-CLIENT` atau supervisor akun layanan → 400 `invalid_reference`; integration repository: list/count tanpa akun layanan, `SupervisorExists` false untuk akun layanan; login akun layanan gagal (`is_active=false` dan `auth_source=service`). Semua test admin yang sudah ada tetap hijau. **Seed:** `backend/internal/handler/integration_test.go` hanya memuat `001` + `002`, jadi role `EXT-CLIENT` dari `039` tidak ada di DB test handler; test membuat role (`INSERT ... ON CONFLICT DO NOTHING`) dan akun layanan sendiri di setup, tanpa mengubah helper bersama.
    - _Requirements: 9.4, 9.5, 9.6, 12.3_
  - [x] 3.6 `backend/cmd/api/no_ext_routes_test.go`: test statis yang memindai file Go non-test di `backend/` dan `backend-exq/` dan gagal bila ada string `/api/ext` (**Property 13** sisi `backend`/`backend-exq`).
    - _Requirements: 3.5_
  - _Model: Opus, Medium — auth: mengubah kode admin user yang sudah jalan; salah alias atau filter membuka akun layanan atau mengubah respons admin. 3.5 Sonnet, Medium dan 3.6 Sonnet, Low (test; lihat Model Split)._

- [ ] 4. Kerangka modul `backend-extapi`, config, dan repository
  - [x] 4.1 Buat `backend-extapi/go.mod` (`module github.com/cimb-niaga/cms/backend-extapi`, `require`/`replace` pkg seperti `backend-exq`), tambah `./backend-extapi` ke `go.work`, `!backend-extapi/` ke `.dockerignore`, dan `COPY backend-extapi/go.mod backend-extapi/go.sum ./backend-extapi/` ke `backend/Dockerfile` dan `backend-exq/Dockerfile` dalam perubahan yang sama. Tambah `sqlc.yaml` (pola `backend-exq`), `coverage-thresholds.txt`, `.env.example` (nilai contoh, catatan `JWT_SECRET` tidak boleh ada). Cek `go build ./...` di tiga modul dan `docker build` `backend` + `backend-exq` tetap berhasil.
    - _Requirements: 12.1, 12.7_
  - [x] 4.2 `backend-extapi/internal/config/config.go`: `Load(getenv)` dan `LoadCLI(getenv)` sesuai tabel design (wajib `DATABASE_URL`, `REDIS_URL`, `EXT_JWT_SECRET` ≥ 32 byte; `JWT_SECRET` harus tidak ada; TTL 1–60 menit; batas rate dalam rentang; error menyebut nama env, bukan nilai). Test (**Property 14**).
    - _Requirements: 2.8, 3.4, 12.1_
  - [x] 4.3 Query `backend-extapi/queries/{api_clients,ext_users,audit}.sql` sesuai design, `sqlc generate`; `internal/repository/ApiClientRepository` (konversi `cidr[]` → `[]netip.Prefix`, pool primary), `internal/repository/TxRunner` (BEGIN/COMMIT/ROLLBACK), `internal/audit/writer.go` (`Before` + `After`, per-`DBTX`, tolak actor 0 dan action kosong).
    - _Requirements: 1.1, 4.2, 10.1, 10.2_
  - [x] 4.4 Test repository: integration `api_client_repository_integration_test.go` (CHECK `assignable_roles` menolak role istimewa, CHECK `client_id`, konversi CIDR), `privileged_sync_test.go` (daftar di CHECK migrasi `039` sama dengan `userrules.PrivilegedRoles`), `no_hard_delete_test.go` untuk `backend-extapi/queries` (**Property 12**), unit test audit writer.
    - _Requirements: 7.5, 9.2_
  - _Model: Sonnet, Medium — kerangka mengikuti pola `backend-exq`; query dan writer mengikuti pola yang ada._

- [ ] 5. Paket `backend-extapi/internal/extauth`
  - [x] 5.1 `client.go` (`ValidScope`, `ParseScopes`, `Client`, `AllowsIP` false untuk daftar kosong, `CanAssign` lewat `userrules.IsPrivileged` + allowlist, `HasScope`) dan `secret.go` (`NewClientID`, `NewSecret` dari `crypto/rand`, `HashSecret` bcrypt cost 12, `VerifySecret`, dummy hash, `ValidClientIDFormat`) + unit test.
    - _Requirements: 1.2, 1.7, 2.4_
  - [x] 5.2 `token.go`: `Issue` (HS256, `typ: at+jwt`, `iss`, `aud=crown-ext`, `sub`, `scope`, `ver`, `jti`, `iat`, `exp`) dan `Validate` (hanya HS256, wajib `iss`/`aud`/`exp`/`sub`/`jti`/`ver`, scope valid) + unit test (alg `none`/RS256 ditolak, `aud`/`iss` salah, kedaluwarsa, claim hilang).
    - _Requirements: 2.6, 2.8, 3.1_
  - [x] 5.3 `token_property_test.go` (rapid): **Property 1** (token `extauth` ditolak `pkg/auth.ValidateAccessToken` dan token `pkg/auth` ditolak `Validate`, termasuk saat kuncinya sama) dan **Property 9** (scope yang diberikan selalu subset).
    - _Requirements: 2.1, 2.2, 2.3, 3.1, 3.2, 3.3_
  - [x] 5.4 `ratelimit.go`: `TokenFailures` (per `client_id` dan per IP, `client_id` tidak valid hanya per IP) dan `APIRate` (per menit), fail closed. `ratelimit_test.go` dengan `miniredis` (sudah dipakai `pkg`): batas, reset, TTL (`FastForward`), dan fail closed (server miniredis ditutup → `ErrUnavailable`). Tanpa Redis nyata, tanpa `REDIS_URL`.
    - _Requirements: 2.9, 4.5_
  - [x] 5.5 `middleware.go`: `RequireClient` (Bearer → `Validate` → client dari primary → aktif + `ver` → IP → rate → `ClientContext`, isi `LogFields.ClientID`) dan `RequireScope`, dengan `WWW-Authenticate` sesuai design; tidak pernah membuat `AuthContext`. `middleware_test.go` dengan **Property 7** (rapid: kombinasi aktif/ver/CIDR/scope).
    - _Requirements: 1.4, 1.5, 1.7, 3.6, 4.1, 4.2, 4.3, 4.4, 4.5_
  - _Model: Opus, High — auth: isolasi token, verifikasi secret, pemeriksaan client/IP/scope; kesalahan membuka API ke pemanggil yang tidak sah._

- [ ] 6. Service `backend-extapi/internal/extuser`
  - [x] 6.1 `service.go`: `Actor`, `Store`, `Tx`, `Reader`, `Service` dengan `Create`, `Update`, `Disable`, `Enable`, `Get`, `GetByUsername` sesuai design (aturan Managed_User; `CanAssign` sebelum DB; `LockUser` `FOR UPDATE`; cek unik, role, supervisor; insert ldap tanpa password; no-op tanpa audit untuk disable/enable berulang; audit `user_created`/`user_updated`/`user_deactivated`/`user_reactivated` dengan actor akun layanan dan IP; pemetaan 23505). Implementasi `Store`/`Tx`/`Reader` di paket baru `internal/extuser/pgstore` di atas query 4.3, `repository.TxRunner`, dan `audit.Writer` (bukan di `internal/repository`: import cycle dengan `audit`; revisi design 2026-11-10, user OK).
    - _Requirements: 5.1, 5.3, 5.4, 5.5, 5.6, 6.1, 6.3, 6.4, 6.5, 7.1, 7.2, 7.3, 7.4, 7.6, 8.1, 8.2, 8.3, 9.3, 10.1, 10.2, 10.3_
  - [x] 6.2 `service_test.go` dengan fake `Tx`/`Store` untuk semua cabang, dan `service_property_test.go` (rapid): **Property 2**, **Property 3**, **Property 10**, serta **Property 5** bagian rollback (fake `WriteAudit` gagal → tidak ada commit).
    - _Requirements: 5.4, 6.4, 7.2, 8.2, 9.3, 10.2_
  - [x] 6.3 `service_integration_test.go` (DB nyata): create/update/disable/enable sungguhan dengan satu audit per perubahan dan `actor_id = service_user_id` (**Property 5**), rollback saat audit gagal (store pembungkus yang memaksa error setelah update), `FOR UPDATE` menahan perubahan role bersamaan, dan user yang dinonaktifkan tidak bisa login maupun refresh (sama dengan disable admin).
    - _Requirements: 7.6, 10.1, 10.2_
  - _Model: Opus, High — auth + RBAC: role istimewa, akun layanan, penguncian baris, dan audit atomik._

- [ ] 7. Handler, router, dan bootstrap `backend-extapi`
  - [x] 7.1 `internal/handler/token.go`: alur 8 langkah di design (header no-store, form + `grant_type`, Basic, cek rate gagal, lookup + dummy hash + aktif + IP dengan 401 `invalid_client` identik, scope, issue + reset, log `ext_token_issued`/`ext_token_denied` tanpa secret/token), body error RFC 6749.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.9, 2.10_
  - [x] 7.2 `internal/handler/users.go` + `errors.go`: Content-Type, `DisallowUnknownFields`, validasi `{id}` dan `?username=`, representasi `ExtUser`, 201 + `Location`, 204, pemetaan error sesuai tabel design, log warn 4xx dengan `client_id`, `route`, `error` tanpa body.
    - _Requirements: 5.1, 5.2, 6.2, 7.1, 8.1, 10.5_
  - [x] 7.3 `internal/handler/router.go` `NewRouter` (middleware chain, `/health`, `/ready`, route token dan user dengan scope, NotFound/MethodNotAllowed JSON) dan `cmd/api/main.go` (config, pool primary, Redis, server timeout, graceful shutdown 15 detik).
    - _Requirements: 3.5, 4.6, 12.1_
  - [x] 7.4 Test: `token_test.go` (**Property 8**: semua sebab `invalid_client` menghasilkan body identik; header), `users_test.go`, `router_test.go` (**Property 13** sisi `backend-extapi` lewat `chi.Walk`), `secret_leak_test.go` (**Property 6**: tangkap `slog`, body respons, dan payload audit; tidak ada secret, hash, token, `Authorization`).
    - _Requirements: 1.3, 2.4, 2.10, 3.5, 10.3_
  - _Model: Opus, High — 7.1 adalah endpoint auth (pesan seragam, dummy hash, rate gagal); 7.2–7.4 Sonnet-runnable (lihat Model Split)._

- [ ] 8. CLI `backend-extapi/cmd/extclient`
  - [x] 8.1 Perintah `create`, `rotate`, `disable`, `enable`, `set-grants`, `list` sesuai design: `--actor` harus APPACCESS aktif; role dicek ke tabel `roles` dan `!IsPrivileged`; CIDR tanpa `/0` (daftar kosong = peringatan); scope minimal satu; akun layanan `svc-<slug>`; secret hanya ke stdout sekali; `rotate` menaikkan `credential_version`; semua tulis + audit (`api_client_*`, `user_created`) dalam satu transaksi; `list` menampilkan `service_user_id` dan `service_username`.
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 9.2, 10.4_
  - [x] 8.2 `cmd/extclient/main_test.go` (integration): setiap perintah terhadap DB; actor bukan APPACCESS ditolak; role istimewa dan `/0` ditolak; audit tercatat per aksi tanpa secret/hash (**Property 6** sisi CLI); token lama ditolak setelah `rotate` (memakai `extauth`).
    - _Requirements: 1.3, 1.4, 1.6, 9.2, 10.4_
  - _Model: Opus, High — auth: membuat dan merotasi kredensial, membuat akun layanan, dan menjaga allowlist role._

- [ ] 9. ExtContract
  - [x] 9.1 `backend-extapi/api/openapi.yaml` (OpenAPI 3.0.3, `servers: /api/ext/v1`, `oauth2` clientCredentials + `basicAuth`, semua operasi, schema request/respons/error, catatan versi path, DELETE = soft-delete, batas rate) dan `api/embed.go` (`Spec`). Tidak di-serve oleh API.
    - _Requirements: 11.1, 11.3, 11.4_
  - [x] 9.2 `internal/handler/openapi_contract_test.go`: spec valid, cakupan route dua arah terhadap `NewRouter`, respons divalidasi `openapi3filter`, test drift (pola `backend/internal/handler/openapi_contract_test.go`).
    - _Requirements: 11.2_
  - _Model: Sonnet, Medium — kontrak mengikuti handler yang sudah jadi; test mengikuti pola yang ada._

- [ ] 10. Image dan compose
  - [x] 10.1 `backend-extapi/Dockerfile` (pola `backend-exq`, build `./cmd/api` dan `./cmd/extclient`, `appuser`, `EXPOSE 8082`, HEALTHCHECK `:8082/health`) dan `backend-extapi/docker-compose.yaml` (`cms-net` external, port `8082`, `env_file`, healthcheck). Cek `docker build -f backend-extapi/Dockerfile .` dan `bash scripts/ci/check-image.sh cms-backend-extapi:ci backend-extapi`.
    - _Requirements: 12.1, 12.7_
  - _Model: Sonnet, Medium — mengikuti pola image `backend-exq` yang sudah ada._

- [ ] 11. Dokumen
  - [x] 11.1 `backend-extapi/api/panduan-integrasi-idm.md` (bahasa Indonesia): base URL per lingkungan (placeholder), alur token dengan contoh `curl` dan respons, contoh setiap endpoint user, tabel kode error + arti + tindakan IDM, umur token dan pembaruan, batas rate + `Retry-After`, aturan retry setelah timeout, DELETE = soft-delete, daftar yang dibutuhkan CROWN dari IDM; semua contoh memakai nilai palsu yang jelas. Cocokkan setiap kode error dan field dengan `openapi.yaml`.
    - _Requirements: 13.1, 13.2, 13.3, 13.4_
  - [x] 11.2 `.claude/docs/deployment.md`: "Release checklist: first `backend-extapi` deployment" (urutan migrasi `039` → `backend` baru → start `backend-extapi` → `extclient create` → serahkan paket IDM), menyalakan/mematikan service, rotasi secret, `TRUSTED_PROXIES`, jalur jaringan sebagai placeholder D7, dan skrip GRANT opsional untuk login DB terbatas (service dan CLI).
    - _Requirements: 12.2_
  - [x] 11.3 Perbarui `.claude/CLAUDE.md` (Sec 3: tabel `api_clients`, role `EXT-CLIENT`, `auth_source=service`; Sec 12: deviasi GR#3 dan topologi), `.kiro/steering/structure.md`, `tech.md`, `project-context.md` (modul workspace ketiga, port 8082, aturan tidak saling import), `development-plan.md` (X.17 status, nomor migrasi bebas berikutnya), dan `.claude/development-progress.md`.
    - _Requirements: 12.2_
  - _Model: Sonnet, Medium — 11.1 harus akurat terhadap kontrak; 11.3 Haiku, Low (pembaruan dokumen mekanis; lihat Model Split)._

- [ ] 12. Checkpoint: gate kualitas
  - Jalankan per modul (`backend`, `backend-exq`, `backend-extapi`, `pkg`): `go vet ./...`, `golangci-lint run --build-tags integration ./...`, `govulncheck ./...`, `go test -race ./...`, dan integration (`-tags integration`, `DATABASE_URL`, tanpa skip) dengan `backend/scripts/check-coverage.sh <test.log> <coverage-thresholds.txt>` terhadap file threshold masing-masing modul. `sqlc generate` di `backend` dan `backend-extapi` tanpa diff. `go test ./internal/handler -run OpenAPI` di `backend` dan `backend-extapi`. `docker build` + `check-image.sh` untuk tiga image. Catat hasil (tanggal, angka lulus, coverage) di `## Notes` → `### Test Results`.
  - _Requirements: 12.3, 12.4, 12.5, 12.7_
  - _Model: Sonnet, Medium — triase kegagalan lintas modul._

- [ ] 13. Cek manual (user)
  - Di dev: jalankan `backend-extapi`, `extclient create` dengan CIDR mesin sendiri, minta token dan panggil create/get/update/delete/enable dengan `curl` mengikuti panduan 11.1; ulangi dari IP di luar CIDR (harus 403) dan dengan token user CompanyPortal (harus 401); cek Log Audit di CompanyPortal (actor = id akun layanan) dan bahwa akun layanan tidak muncul di Manajemen User.
  - _Requirements: 4.3, 10.1, 13.1_
  - _Model: Sonnet, Low — dijalankan user; AI hanya membantu triase bila ada temuan._

## Notes

### Test Results

Belum ada (diisi saat task 12). Cek browser/manual task 13: **outstanding (user)**.

### Definition of Done

- [ ] Semua task 1–12 selesai dan gate task 12 hijau; task 13 tercatat outstanding untuk user.
- [ ] Setiap acceptance criterion Req 1–13 dirujuk task dan diuji (checklist traceability `kiro-spec-standard.md` Sec 5).
- [ ] Migrasi `039` diterapkan ke dev dan dicatat di `.claude/CLAUDE.md` Sec 3.
- [ ] Tidak ada secret, `client_id` asli, hostname, atau IP internal di repo (`.env.example`, panduan, kontrak, test).
- [ ] Deviasi GR#3 dan topologi modul ketiga tercatat di `.claude/CLAUDE.md` Sec 12.
- [ ] `backend` dan `backend-exq` berperilaku sama untuk semua endpoint selain penjagaan akun layanan (Req 12.3, 12.4).

### Catatan pengerjaan

- **Commit.** Saran pemisahan: (1) task 1 + 2, (2) task 3, (3) task 4–8, (4) task 9–11. Commit dibuat oleh user (atau atas permintaan user).
- **sqlc.** Selalu v1.31.1. Di `backend`, setelah `sqlc generate` perbaiki `UserLeafe → UserLeave` di `internal/db/`; `backend-extapi` (seperti `backend-exq`) **tidak** di-hand-fix: rename `user_leaves` di `sqlc.yaml` tidak berefek, jadi `models.go` tetap `UserLeafe` dan `sqlc diff` harus bersih. Jangan pakai tipe itu di kode `backend-extapi`.
- **Urutan workspace.** Jangan menambah `./backend-extapi` ke `go.work` tanpa baris `COPY` di Dockerfile `backend` dan `backend-exq` (task 4.1), atau image keduanya gagal di-build.
- **Redis untuk test.** Test rate limit memakai `github.com/alicebob/miniredis/v2` (sudah ada di `pkg/go.mod`, bukan dependensi baru), jadi tidak butuh Redis nyata untuk test. Redis nyata hanya untuk cek manual task 13.
- **Follow-up di luar spec ini:** audit di luar transaksi pada AdminUserAPI dan `DeactivateUserService`; disable user yang sudah nonaktif di AdminUserAPI berakhir 500; layar admin untuk mengelola ApiClient bila client bertambah (D9); jalur jaringan dan TLS (D7); job CI `backend-extapi` saat migrasi Jenkins (D23, perkiraan Desember 2026).

### Catatan eksekusi

- **Wave 0 (2026-11-10, dicek sesi orkestrator setelah keenam task selesai):** `go build ./...` + `go vet ./...` hijau di `pkg`, `backend`, `backend-exq` (`backend-extapi` belum punya package). `go test ./userrules ./config ./middleware` di `pkg` dan `./cmd/api` di `backend` lulus. `docker build` image `backend` dan `backend-exq` berhasil dengan `go.work` baru (cek 4.1). Migrasi `039` belum diterapkan (task 1.2).
- **Wave 1 (2026-11-10):** 1.2 diterapkan dari sesi orkestrator atas OK user ("Ya, terapkan 039"): `migrate up` → 38 applied, 0 pending, checksum ok; `\d api_clients` sesuai design (CHECK `assignable_roles`, `client_id`, `scopes`, FK `service_user_id`/`created_by`); role `EXT-CLIENT` ada (id 1286). 3.1, 4.2, 5.1: `go build` + `go vet` hijau di empat modul; test `backend/internal/auth`, `internal/handler`, `cmd/api` lulus tanpa perubahan test; `backend-extapi/internal/config` 100%, `internal/extauth` 88,6% (paket masih sebagian, threshold 90 dicek di task 12). `-race` tidak bisa (cgo mati di mesin ini); test `-tags integration` belum dijalankan.
- **Wave 2 (2026-11-10):** 3.2, 4.3, 5.2 selesai. `go build` + `go vet` hijau di empat modul; unit test `backend/internal/...` dan `cmd/...` lulus; integration `backend/internal/repository -run UserAdmin` lulus terhadap DB dev (039 sudah ada). `backend-extapi`: `extauth` 92,2%, `config` 100%; `repository` dan `audit` belum punya test (task 4.4). `sqlc diff` di `backend` hanya berisi hand-fix `UserLeave` yang memang disengaja; di `backend-extapi` sesi orkestrator menjalankan ulang `sqlc generate` tanpa hand-fix (sesi 4.3 sempat mengubah ke `UserLeave`, tidak dipakai kode mana pun) sehingga `sqlc diff` bersih, sama dengan `backend-exq`.
- **Wave 3 (2026-11-10):** 3.3, 4.4, 5.3, 5.4 selesai. `go build` + `go vet` hijau di empat modul; unit test `backend` lulus. `backend-extapi` `go test -tags integration ./...` terhadap DB dev lulus: `audit` 94,4%, `config` 100%, `extauth` 94,6%, `repository` 16,4% (paket hasil sqlc, dikecualikan dari threshold). `sqlc diff` bersih. DB dev bersih setelah test (0 baris `api_clients`, 0 akun `auth_source='service'`).
- **Wave 4 (2026-11-10):** 3.4, 5.5, 6.1 selesai. `go build` + `go vet` hijau di empat modul; unit test `backend` lulus; `backend-extapi` integration lulus (`extauth` 96,5%, `audit` 94,4%, `config` 100%); `sqlc diff` bersih. `extuser` dan `extuser/pgstore` belum punya test (task 6.2, 6.3). Revisi design: Store/Tx/Reader di `internal/extuser/pgstore` karena import cycle `repository` ↔ `audit` (user OK). **Untuk task 12:** tambahkan baris threshold `github.com/cimb-niaga/cms/backend-extapi/internal/extuser/pgstore` (paket baru, belum ada di design) setelah 6.3 memberi angka coverage.
- **Wave 5 (2026-11-10):** 3.5, 6.2, 7.1, 8.1 selesai. `go build` + `go vet` hijau di empat modul. `backend` integration (`internal/auth`, `internal/handler`, `internal/repository`, `cmd/...`) lulus terhadap DB dev. `backend-extapi` integration lulus: `extuser` 100%, `extauth` 96,5%, `audit` 94,4%, `config` 100%, `handler` 74,2% (baru token + smoke test; 7.2–7.4 menyusul); `cmd/extclient` dan `extuser/pgstore` 0% sampai 8.2 dan 6.3. `sqlc diff` bersih. DB dev bersih setelah test (0 `api_clients`, 0 akun layanan; role `EXT-CLIENT` tetap 1 dari migrasi).
- **Wave 6 (2026-11-10):** 7.2 dan 8.2 selesai (`cmd/extclient` 81,7%, DB bersih). Test 6.3 menemukan bug di `LockUserForExt` (JOIN `roles` + `FOR UPDATE` → 404 palsu saat role diubah bersamaan). Diperbaiki dari sesi orkestrator atas OK user: role lewat subquery skalar, `sqlc generate`, `sqlc diff` bersih, tipe baris tidak berubah; `TestService_ForUpdateSerialisesConcurrentRoleChange` lulus. `extuser/pgstore` 88,5% → baris threshold untuk task 12: `github.com/cimb-niaga/cms/backend-extapi/internal/extuser/pgstore 85`.
- **Wave 7 (2026-11-10):** 7.3 selesai: `router.go` + `cmd/api/main.go`; `go build`, `go vet`, dan build binary `cmd/api` hijau. Smoke run lokal belum dilakukan (sesi 7.3 diblokir classifier; di mesin ini tidak ada Redis lokal). Route dan middleware dites di 7.4 (`chi.Walk`) dan end-to-end di cek manual task 13.
- **Wave 8 (2026-11-10):** 7.4 selesai (`internal/handler` 96,0%). 10.1 selesai: image `cms-backend-extapi` ter-build, `check-image.sh` lulus (user `appuser`, HEALTHCHECK). 9.1 ditulis oleh sesinya tapi tidak dicentang; sesi orkestrator memvalidasi `openapi.yaml` dengan kin-openapi (`Validate` lulus, 7 operasi = 7 route di `router.go`) lalu mencentang 9.1. Test kontrak sungguhan = task 9.2.
- **Wave 9 (2026-11-10):** 9.2 selesai: `TestOpenAPIContract_{SpecValid,RouteCoverage,Responses,RejectsDrift}` lulus; kin-openapi v0.151.0 masuk `backend-extapi/go.mod`. 11.1 (`backend-extapi/api/panduan-integrasi-idm.md`) dan 11.2 (release checklist `backend-extapi` di `.claude/docs/deployment.md`) selesai. Sesi orkestrator memindai panduan + kontrak: tidak ada IP, hanya `client_id` palsu `crown_aaaa…` (Req 13.3).
- **5.1 (2026-11-10):** `backend-extapi/go.mod` kini `require golang.org/x/crypto v0.58.0` (versi yang sama dengan `backend`, bukan dependensi baru); `go get` menaikkan direktif `go` ke `1.26.0` (syarat x/crypto v0.58.0, sama dengan `backend` dan `go.work`). `VerifySecret("", s)` = client tidak dikenal: tetap bcrypt-compare ke dummy hash (cost 12, dibuat sekali lewat `sync.OnceValue`) dan selalu `false`, jadi handler 7.1 cukup memberi hash kosong. `AllowsIP` meng-`Unmap` alamat IPv4-mapped. `go vet` + `go test -cover ./internal/extauth` lulus, coverage 91.9% (yang tidak tercakup hanya cabang error `crypto/rand`, yang tidak bisa terjadi di Go ≥1.24). `-race` tidak bisa dijalankan di mesin ini (cgo mati).
- **5.2 (2026-11-10):** `go mod tidy` menambah `golang-jwt/jwt/v5 v5.3.1` + `google/uuid v1.6.0` ke `backend-extapi/go.mod` (versi sama dengan `pkg`/`backend`). Lebih ketat dari design, tanpa mengubah kontrak: `Validate` juga mewajibkan header `typ: at+jwt` dan `iat`, memakai jam `TokenService.now` (`jwt.WithTimeFunc`), dan scope hanya dipisah spasi (koma ditolak); `Issue` menolak scope kosong/tidak dikenal/di luar `Client.Scopes`, `client_id` kosong, dan `ver <= 0`. Semua penolakan `Validate` membungkus `ErrInvalidToken` (untuk 401 di 5.5). `go vet` + `go test -cover ./internal/extauth` lulus, coverage 92.2%; `go build ./...` modul hijau. `-race` tidak bisa (cgo mati).
- **5.4 (2026-11-10):** `ratelimit.go`: `TokenFailures` (`Check`/`RecordFailure`/`ResetClient`) dan `APIRate.Allow`; semua return `(retryAfter, err)` dengan `retryAfter > 0` = 429, error Redis dibungkus `ErrUnavailable` (503). GET+TTL dan INCR+EXPIRE NX masing-masing satu pipeline; key tanpa TTL dilaporkan sebagai satu jendela penuh; Retry-After API = sisa menit berjalan. `go mod tidy` memindah `go-redis/v9` ke direct dan menambah `miniredis/v2 v2.38.0` (versi sama dengan `pkg`). `go vet` + `go test -cover ./internal/extauth` lulus (94.6%); fungsi `ratelimit.go` 95–100%. Tanpa Redis nyata.
- **5.3 (2026-11-10):** `go mod tidy` menambah `pgregory.net/rapid v1.3.0` (versi sama dengan `pkg`/`backend`). Property 1 memakai jam dinding (pkg/auth tidak punya jam injeksi), kunci acak 32–64 byte dengan opsi kunci sama, dan cek sanity bahwa tiap layanan menerima token miliknya. Bagian Property 1 "`RequireClient` tidak menaruh `AuthContext`" belum diuji di sini (milik 5.5, `middleware.go`). Property 9 diuji di level `ParseScopes` + `Issue`/`Validate`; respons 400 `invalid_scope` milik handler 7.1. Tidak ada bug di `token.go`. `go vet` + `go test -cover ./internal/extauth -rapid.checks=1000` lulus, coverage 92.2%.
- **5.5 (2026-11-10):** `middleware.go`: `RequireClient` urut Bearer (skema tidak peka huruf) → `Validate` → `GetByClientID` → aktif + `client_id` + `ver` (401 `invalid_token`) → isi `LogFields.ClientID` → IP dari `r.RemoteAddr` (IP polos atau ip:port, IPv4-mapped di-unmap; tidak bisa di-parse = 403) → `APIRate.Allow` (429 + `Retry-After` dibulatkan ke atas, min 1; Redis mati 503) → `ClientContext`. Dua pilihan di luar teks design (tolong dicek reviewer): (1) error lookup DB → 500 `internal_error` (design tidak menyebut); (2) `GrantedScopes` = scope token ∩ `Client.Scopes` saat ini, jadi `set-grants` yang mencabut scope langsung berlaku tanpa menunggu token kedaluwarsa (lebih ketat, kontrak sama). `client_id` sudah dicatat untuk 403 `ip_not_allowed`/`insufficient_scope`/429 (client sudah terverifikasi), tidak untuk 401. Counter rate tidak disentuh untuk request yang ditolak sebelum cek IP. `RequireScope` tanpa `ClientContext` → 401. Bagian Property 1 "tidak ada `AuthContext`" diuji di sini (`GetAuthContext` false). `go vet` + `go test -cover ./internal/extauth` lulus, coverage 96.5%; Property 7 100 kasus rapid. `golangci-lint`: G101 false positive pada konstanta header di-`nolint`; sisa SA4000 di `secret_test.go:83` (milik 5.1, tidak diubah). `-race` tidak bisa (cgo mati).
- **8.1 (2026-11-10):** `cmd/extclient/main.go` (dispatch, flag, koneksi lazy: `--help` dan validasi argumen tanpa DB) + `commands.go`. Exit code 0/1/2 (2 = argumen salah). Lebih ketat dari design tanpa ubah kontrak: CIDR dengan host bit (mis. `10.0.0.10/24`) ditolak dengan saran bentuk mask; `disable`/`enable` ke status yang sama = no-op tanpa tulis dan tanpa audit; audit `api_client_secret_rotated` berisi `credential_version` lama/baru saja. Hanya query 4.3 + `GetRoleIDByName` (`ext_users.sql`); tanpa perubahan `go.mod`. `go build ./...` + `go vet ./cmd/...` hijau; cek dry-run argumen (role istimewa, `/0`, scope kosong/tak dikenal, client id salah, `DATABASE_URL` kosong) sesuai. Tidak dijalankan ke DB dev (integration = 8.2).
- **8.2 (2026-11-10):** `cmd/extclient/main_test.go` (tag `integration`, skip tanpa `DATABASE_URL`) memanggil `run` langsung ke DB dev; CLI commit sungguhan, jadi semua baris (actor `it82-<hex>-*`, client, akun `svc-it82-…`, audit milik actor itu) dibuat dengan prefix per run dan dihapus di `t.Cleanup` (dicek: 0 sisa). Cakupan: create (client + akun layanan EXT-CLIENT nonaktif tanpa password, hash memverifikasi secret, `user_created` + `api_client_created` oleh actor, ip `cli`), nama ganda ditolak, CIDR kosong = peringatan; actor ATM-USER / APPACCESS nonaktif / tidak dikenal ditolak di 5 perintah tulis tanpa perubahan dan tanpa audit; 4 role istimewa + `/0` IPv4/IPv6 ditolak (exit 2) di create dan set-grants, role tak dikenal exit 1; set-grants mengganti daftar yang diberi saja + audit before/after; disable/enable no-op tanpa audit; rotate → `credential_version` 2, secret lama tidak lolos `VerifySecret`, token lama 401 dan token baru 200 lewat `extauth.RequireClient` + repo nyata (miniredis untuk rate), token client nonaktif 401; list memuat `service_user_id`/`service_username` tanpa secret. **Property 6** sisi CLI: payload audit tidak memuat secret, hash, `secret_hash`, `client_secret`; test tidak pernah mencetak stdout CLI. `go vet -tags integration` hijau; `go test -tags integration ./cmd/extclient` lulus, coverage 81,7%. Tidak ada bug di kode non-test.

## Task Dependency Graph

- **Wave 0:** 1.1 (migrasi), 2.1, 2.2, 2.3 (`pkg/`), 4.1 (kerangka modul + `go.work` + Dockerfile COPY), 3.6 (test statis, tanpa dependensi).
- **Wave 1:** 1.2 (butuh 1.1), 3.1 (butuh 2.1), 4.2 (butuh 2.2, 4.1), 5.1 (butuh 2.1, 4.1).
- **Wave 2:** 3.2 (butuh 1.1; sqlc membaca skema), 4.3 (butuh 1.1, 4.1), 5.2 (butuh 5.1).
- **Wave 3:** 3.3 (butuh 3.1, 3.2; file sama dengan 3.1), 4.4 (butuh 4.3), 5.3 (butuh 5.2), 5.4 (butuh 5.1).
- **Wave 4:** 3.4 (butuh 3.3), 5.5 (butuh 2.3, 4.3, 5.2, 5.4), 6.1 (butuh 2.1, 4.3, 5.1).
- **Wave 5:** 3.5 (butuh 3.2, 3.4), 6.2 (butuh 6.1), 7.1 (butuh 4.3, 5.1, 5.2, 5.4), 8.1 (butuh 4.3, 5.1).
- **Wave 6:** 6.3 (butuh 6.1), 7.2 (butuh 5.5, 6.1), 8.2 (butuh 5.2, 8.1).
- **Wave 7:** 7.3 (butuh 4.2, 7.1, 7.2).
- **Wave 8:** 7.4 (butuh 7.3), 9.1 (butuh 7.3), 10.1 (butuh 7.3, 8.1).
- **Wave 9:** 9.2 (butuh 9.1), 11.1 (butuh 9.1), 11.2 (butuh 10.1, 8.1).
- **Wave 10:** 11.3 (butuh 1.2, 11.2).
- **Wave 11:** 12 (gate; butuh semua task 1–11).
- **Wave 12:** 13 (cek manual user; butuh 12).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1", "2.2", "2.3", "4.1", "3.6"] },
    { "id": 1, "tasks": ["1.2", "3.1", "4.2", "5.1"] },
    { "id": 2, "tasks": ["3.2", "4.3", "5.2"] },
    { "id": 3, "tasks": ["3.3", "4.4", "5.3", "5.4"] },
    { "id": 4, "tasks": ["3.4", "5.5", "6.1"] },
    { "id": 5, "tasks": ["3.5", "6.2", "7.1", "8.1"] },
    { "id": 6, "tasks": ["6.3", "7.2", "8.2"] },
    { "id": 7, "tasks": ["7.3"] },
    { "id": 8, "tasks": ["7.4", "9.1", "10.1"] },
    { "id": 9, "tasks": ["9.2", "11.1", "11.2"] },
    { "id": 10, "tasks": ["11.3"] },
    { "id": 11, "tasks": ["12"] },
    { "id": 12, "tasks": ["13"] }
  ]
}
```
