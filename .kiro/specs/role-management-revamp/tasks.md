# Implementation Plan: Revamp Manajemen Peran (deskripsi, Portal Utama/Cabang, UI)

> Status: accepted 2026-11-25 (engineer). Berdasarkan `requirements.md` (accepted 2026-11-25, PO; D1–D11) dan `design.md` (accepted 2026-11-25, engineer; DQ1–DQ3 = rekomendasi, DQ4 = tanpa mockup).

## Overview

Urutan pengerjaan:
1. **Pre-check** (task 1): cek fakta yang belum dipastikan di design, yaitu kolom `locations` yang rusak, celah `backend-extapi`, dan guard sumber opsi peran (DQ3).
2. **Migrasi `047`** (task 2): ditulis lalu diterapkan ke dev. `migrate up` butuh OK user (CLAUDE.md Sec 4 #7).
3. **`rolemgmt`** (task 3): query, service, dan handler untuk portal, scope, dan jumlah user.
4. **Cakupan pihak** (task 4): `vendorrole` berbasis prefix + `ScopeReader`; service vendor membaca scope dari DB.
5. **Penegakan Portal_Cabang** (task 5): middleware, mount, izin saya, dan OpenAPI.
6. **CompanyPortal**: layar Manajemen Peran (task 6), lalu form user + `RoleBadge` (task 7).
7. **VendorPortal** (task 8): sidebar dan route mengikuti izin.

Spec ditutup dengan gate kualitas, dokumentasi, dan cek browser oleh user. `/security-review` dijalankan di tahap 5 (`review.md`).

**Kompilasi tetap hijau per wave:** `vendorrole.All`/`PartyRoles`/`ForParty` baru dihapus di task 5.2, setelah semua pemanggil di service (4.2) dan handler (5.2) dipindah.

**Jendela merah (dev saja):** setelah 2.3 (grant vendor ada) dan sebelum 5.2 (penegakan), mencabut izin vendor belum berpengaruh. Rilis memuat semuanya sekaligus.

Pola yang dirujuk:
- Migrasi katalog + grant + guard + post-check: `046_permission_catalog_coverage.sql` (+ skrip rollback dev).
- Integration test migrasi: `backend/internal/rolemgmt/permission_catalog_coverage_test.go`.
- Middleware + evaluator: `backend/internal/rolemgmt/evaluator.go`.
- Store izin + guard route: CompanyPortal `lib/auth/store.ts`, `routes/_protected.tsx` (`requirePermission`).
- sqlc v1.31.1 + perbaikan `UserLeafe` → `UserLeave`.
- Skill: `ecc:golang-patterns` / `ecc:golang-testing` (Go); `ui-ux-pro-max` + `ecc:react-patterns` / `ecc:react-testing` / `ecc:frontend-a11y` (task 6–8).

### Model Split

**Opus-required** (zona bahaya: auth/RBAC, migrasi data)
- 2.1 Migrasi `047`: kolom + CHECK, backfill, grant, perbaikan teks
- 2.3 Terapkan `047` ke dev (butuh OK user)
- 3.2 Service `rolemgmt`: validasi portal/scope, lintas portal, audit di tx
- 4.1 `vendorrole` berbasis prefix + `ScopeReader`: definisi "peran vendor"
- 4.2 Service vendor membaca scope (data pihak vendor)
- 4.3 Aturan peran × jenis user (`user_admin`, `backend-extapi`)
- 5.1 `RequireVendorPermission` + `RequireVendorRole`: keputusan akses inti
- 5.2 Pemasangan di `main.go` + handler vendor; hapus daftar tetap

**Sonnet-runnable**
- 1 Pre-check
- 2.2 Skrip rollback dev (needs 2.1)
- 2.4 Integration test migrasi, Property 1, 2, 3 (needs 2.3)
- 3.1 Query + sqlc (needs 2.1)
- 3.3 Handler role management (needs 3.2)
- 3.4 Test `rolemgmt`, Property 4 (needs 3.3)
- 4.4 Test cakupan pihak, Property 6 (needs 4.2, 4.3)
- 5.3 Izin saya + `party_scope` di `/auth/me` (needs 3.1)
- 5.4 Test penegakan, Property 5, 7, 8 (needs 5.2, 5.3, 2.3)
- 5.5 OpenAPI + kontrak + `gen:api` (needs 5.2)
- 6.1 API/types/hooks role management (needs 3.3)
- 6.2 Halaman: tab, URL state, master-detail, pencarian (needs 6.1)
- 6.3 Dialog Buat Peran + editor izin (needs 6.2)
- 6.4 Test layar (needs 6.3)
- 7.1 Form user: opsi peran dari API (needs 1, 6.1)
- 7.2 `RoleBadge` + hapus daftar tetap
- 7.3 Test form user + badge (needs 7.1, 7.2)
- 8.1 VendorPortal: izin di auth + filter sidebar
- 8.2 VendorPortal: guard route + `NoAccess` (needs 8.1)
- 8.3 VendorPortal: label cakupan dari `/auth/me` (needs 5.3, 8.1)
- 8.4 Test VendorPortal, Property 9 (needs 8.2, 8.3)
- 9 Gate kualitas

**Haiku**
- 10 Dokumentasi
- 11 Cek browser (dilakukan user)

## Tasks

- [x] 1. Pre-check fakta design
  _Model: Sonnet, Medium — baca kode/DB, perilaku tidak diubah_
  - Kolom `locations` yang memuat `â€”` (seed `002` baris 961, 1500, 1742; cek juga di DB dev).
  - Apakah `backend-extapi` bisa memberi peran `VENDOR-*` (`pkg/userrules`, allowlist `assignable_roles` client).
  - Bandingkan guard `GET /api/v1/admin/roles` dengan guard admin users, lalu putuskan DQ3: pakai endpoint itu atau tambah `GET /api/v1/admin/users/role-options`.
  - Catat hasilnya di `## Notes` → `### Pre-check`. Bila ada hasil yang mengubah design → STOP, tanya user.
  _Requirements: 1.1, 2.5, 8.1_

- [ ] 2. Migrasi `047_role_portal.sql`
  _Model: Opus (2.1, 2.3) / Sonnet (2.2, 2.4), High — migrasi data + RBAC (Sec 4 #7)_
  - [x] 2.1 Tulis `backend/migrations/047_role_portal.sql` (UTF-8, satu tx, pola 046):
    - guard: peran vendor, `EXT-CLIENT`, dan user ADMIN harus ada;
    - perbaiki `â€”` → `—` di `roles.description` dan kolom `locations` hasil task 1, hanya pada baris yang cocok;
    - `roles.portal` + CHECK nilai + `roles_portal_prefix_chk`, backfill `vendor`/`service`;
    - `roles.party_scope` + CHECK subset + wajib untuk vendor, backfill sesuai `PartyRoles` lama;
    - `menu_features.portal`; menu `vendor-dsr`, `vendor-orders`, `vendor-realizations`;
    - grant awal D6 (`VENDOR-VAULT-USER` tanpa Realisasi), `granted_by` = ADMIN pertama, `ON CONFLICT DO NOTHING`;
    - post-check; header berisi rollback; `COMMENT ON COLUMN`.
    _Requirements: 1.1, 1.2, 1.3, 2.1, 2.6, 3.1, 3.3, 7.2_
  - [x] 2.2 `backend/scripts/dev/rollback_047_role_portal.sql`: hapus grant dan key `vendor-*`, lalu `DROP COLUMN` tiga kolom baru. Perbaikan teks tidak dibalik.
    _Requirements: 2.1_
  - [x] 2.3 `go run ./cmd/migrate status` → **minta OK user** → `go run ./cmd/migrate up` ke dev; catat hasil `status`.
    _Requirements: 1.1, 3.3_
  - [x] 2.4 Integration test migrasi (`//go:build integration`, skip tanpa `DATABASE_URL`):
    - **Property 1**: langkah perbaikan teks dijalankan ulang tanpa mengubah apa pun, tidak ada `â€”` tersisa, dan hasilnya `—`;
    - **Property 2**: key `vendor-*` per peran = `NAV_ITEMS` lama dikurangi `hideRealizations`;
    - **Property 3**: insert yang melanggar prefix/scope ditolak CHECK.
    _Requirements: 1.1, 1.2, 2.1, 3.3, 7.2, 9.2_

- [ ] 3. `rolemgmt`: portal, scope, jumlah user
  _Model: Sonnet (3.1, 3.3, 3.4) / Opus (3.2), High — RBAC + audit di tx_
  - [x] 3.1 Query role management di `backend/queries/`:
    - `ListRolesWithPermissions` ditambah `portal`, `party_scope`, `active_user_count` (LEFT JOIN, user aktif, replica);
    - `ListCatalog` ditambah `portal`; `CreateRole` ditambah `portal` dan `party_scope`;
    - `GetRolePartyScope(role)` dan `ListVendorRolesForParty(party)`;
    - `Repository.PartyScope` (replica) mengimplementasikan `vendorrole.ScopeReader`;
    - `sqlc generate` (v1.31.1), lalu perbaiki `UserLeafe` → `UserLeave`.
    _Requirements: 2.1, 6.3, 7.4_
  - [x] 3.2 `PermissionService`:
    - `CreateRole` menerima `Portal` dan `PartyScope` dengan aturan design (prefix `VENDOR-` ⇔ vendor; scope wajib untuk vendor dan dilarang untuk main), pelanggaran → `ValidationError` 422;
    - `resolvePermissionSet` menolak katalog dari portal lain → `ErrRoleNotAssignable`; `assignable_roles` hanya berlaku untuk katalog `main`;
    - audit `role_created` mencatat portal dan scope; guard `APPACCESS`/`ADMIN` di service tetap.
    _Requirements: 2.2, 2.3, 2.4, 3.4, 3.5, 5.4, 7.1, 7.3_
  - [x] 3.3 `role_mgmt_handler.go`: request create ditambah `portal`/`party_scope`; response list/catalog memuat field baru; PUT izin memakai decoder strict (field asing → 422).
    _Requirements: 2.3, 5.2, 6.3, 7.3_
  - [x] 3.4 Test table-driven + integration:
    - validasi `CreateRole` (portal × prefix × scope);
    - **Property 4**: izin lintas portal → 422, `role_permissions` tidak berubah;
    - `ADMIN_PARAM` → 403 di route dan service;
    - `active_user_count`; audit gagal → rollback.
    _Requirements: 2.2, 2.3, 2.4, 3.4, 3.5, 5.4, 6.3, 7.1, 7.3, 9.2_

- [ ] 4. Cakupan pihak dari DB
  _Model: Opus (4.1, 4.2, 4.3) / Sonnet (4.4), High — scope data vendor_
  - [x] 4.1 `backend/internal/vendorrole/vendorrole.go`: `IsVendor` berbasis prefix `VENDOR-`; tambah interface `ScopeReader`. `All`/`PartyRoles`/`ForParty` sementara tetap ada (dihapus di 5.2).
    _Requirements: 2.1, 7.4_
  - [x] 4.2 Service:
    - `service/vendor_order.go`: `PartyRoles(va.Role)` diganti `scopes.PartyScope(ctx, va.Role)`; hasil kosong → 403;
    - `service/vendor_party.go:338`: `ForParty` diganti `ListVendorRolesForParty` di tx pemanggil;
    - wiring `ScopeReader` di `cmd/api/main.go`.
    _Requirements: 7.4, 7.5_
  - [x] 4.3 `auth/user_admin.go` `vendorFields`: peran non-vendor dengan `vendor_id` → `ReferenceError`. Di `backend-extapi` / `pkg/userrules`, tolak peran `VENDOR-*` bila task 1 menemukan celah. Privileged dan `EXT-CLIENT` tidak berubah.
    _Requirements: 2.5, 2.6_
  - [x] 4.4 Test:
    - **Property 6**: peran baru dengan `party_scope` {replenish}, {vault}, {replenish,vault} menghasilkan list/decide/realization yang sama dengan `VENDOR-ATM-USER` / `VENDOR-VAULT-USER` / `VENDOR-USER`;
    - `ListVendorRolesForParty` = `ForParty` lama untuk peran lama;
    - kombinasi peran × `auth_source` × `vendor_id` di `user_admin`;
    - error baca scope → fail closed (**Property 7**).
    _Requirements: 2.5, 2.6, 7.4, 7.5, 9.2_

- [ ] 5. Penegakan izin Portal_Cabang
  _Model: Opus (5.1, 5.2) / Sonnet (5.3, 5.4, 5.5), High — guard route vendor_
  - [x] 5.1 `rolemgmt`:
    - `RequireVendorPermission(e, key)`: non-vendor lolos tanpa query; vendor dicek ke replica; error → 500 + `slog.Error`;
    - `RequireVendorRole()`: 403 untuk non-vendor;
    - `VendorAlwaysOnKeys = {"vendor-notifications"}` di `keys.go`.
    _Requirements: 3.2, 4.4, 4.5, 4.6, 4.7_
  - [x] 5.2 Pemasangan sesuai tabel mount design:
    - `main.go`: `/vendor/replenish-orders` → `vendor-orders`, `/vendor/replenish-realizations` → `vendor-realizations`, `/dsr` → `vendor-dsr`;
    - `vendor_order_handler.go`: `RequireRoles(vendorrole.All)` → `RequireVendorRole()`; `POST /{id}/realization` ditambah `vendor-realizations` lewat fungsi `perm` yang disuntik;
    - `replenishment_handler.go:180` → `RequireVendorRole()`;
    - hapus `vendorrole.All`/`PartyRoles`/`ForParty` dan perbarui tesnya.
    _Requirements: 4.4, 4.5, 4.6, 7.4_
  - [x] 5.3 `auth_handler.go`: `MyPermissions` untuk peran vendor = grant `vendor-*` + `VendorAlwaysOnKeys`; `EXT-CLIENT` → `[]`. `/auth/me` ditambah `party_scope` (dibaca per request).
    _Requirements: 4.1, 7.6_
  - [x] 5.4 Test handler, middleware, dan integration:
    - **Property 5**: vendor tanpa key → 403 di tiap mount (termasuk POST realization); dengan key → bukan 403; internal di `/dsr` → sama seperti sebelumnya;
    - **Property 7**: error repo → 500, handler tidak dipanggil;
    - **Property 8**: setelah izin dicabut, request berikutnya 403 tanpa login ulang;
    - izin saya untuk `VENDOR-USER`, `VENDOR-VAULT-USER`, `EXT-CLIENT`.
    _Requirements: 3.2, 4.1, 4.4, 4.5, 4.6, 4.7, 9.2_
  - [x] 5.5 `backend/api/openapi.yaml`: pastikan respons 403 terdokumentasi di `/vendor/replenish-orders*`. Jalankan `go test ./internal/handler -run OpenAPI` dan `pnpm --dir frontend/CompanyPortal-Vite run gen:api` (commit `schema.d.ts` bila berubah).
    _Requirements: 9.1_

- [ ] 6. CompanyPortal: layar Manajemen Peran (`ui-ux-pro-max`, Merah Sirih)
  _Model: Sonnet, High — UI besar, keputusan akses tetap di backend_
  - [x] 6.1 `features/role-management/{api,types}.ts` + hooks: `portal`, `party_scope`, `active_user_count`, katalog per portal, create dengan portal + scope.
    _Requirements: 5.2, 6.3_
  - [x] 6.2 `RoleManagementPage`:
    - search params `portal`/`role`/`q` (zod, fallback `main`);
    - `PortalTabs` (tablist ARIA, navigasi panah);
    - master-detail, peran pertama terpilih otomatis; pencarian nama/deskripsi tanpa membedakan huruf besar/kecil;
    - baris berisi badge, deskripsi 2 baris + `title`, jumlah user (`tabular-nums`), chip scope (ikon + teks);
    - 4 state + "tidak ada hasil"; layar < `lg` ditumpuk dengan pindah fokus;
    - deskripsi ditampilkan apa adanya.
    _Requirements: 1.4, 5.1, 5.2, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_
  - [x] 6.3 `CreateRoleDialog`: prop `portal`; untuk vendor, prefix `VENDOR-` terkunci, radio scope wajib, judul "Buat Peran Portal Cabang". `PermissionEditor`: hanya katalog tab aktif, tombol Simpan/Batalkan, konfirmasi bila pindah peran dengan perubahan belum tersimpan, pesan untuk 422 `role_not_assignable`.
    _Requirements: 2.2, 2.4, 3.2, 5.3, 7.1_
  - [x] 6.4 Test vitest + RTL: tab ↔ URL, nilai portal tidak valid → Portal Utama, pencarian dan "tidak ada hasil", pilihan otomatis, dialog vendor (prefix + scope wajib), editor hanya memuat katalog portal aktif, 4 state, axe tanpa pelanggaran.
    _Requirements: 1.4, 5.1, 5.2, 5.3, 6.1, 6.2, 6.4, 6.5, 7.1, 9.2_

- [ ] 7. CompanyPortal: form user + `RoleBadge`
  _Model: Sonnet, Medium — UI; aturan ditegakkan backend (4.3)_
  - [x] 7.1 `features/admin-users`: opsi peran diambil dari API sesuai hasil DQ3 (task 1) dan difilter per jenis user (vendor → `vendor`, internal → `main`). `userFormSchema.ts` tidak lagi memakai daftar `VENDOR-*` tetap; cek vendor memakai prefix.
    _Requirements: 8.1, 8.2_
  - [x] 7.2 `rbac-settings/types.ts`, `lib/auth/store.ts`: daftar tetap diganti cek prefix. `RoleBadge`: peran tidak dikenal → varian netral, nama tetap ditampilkan.
    _Requirements: 8.2, 8.3_
  - [x] 7.3 Test: opsi peran per jenis user, peran vendor baru muncul, `RoleBadge` netral, error 422 dari backend tampil di field.
    _Requirements: 2.5, 8.1, 8.2, 8.3, 9.2_

- [ ] 8. VendorPortal: sidebar dan route mengikuti izin
  _Model: Sonnet, Medium — UI; backend tetap menolak (5.2)_
  - [x] 8.1 `features/auth`: ambil `GET /api/v1/auth/me/permissions` setelah login/refresh (TanStack Query, disimpan di context auth). `lib/constants.ts`: `NAV_ITEMS` ditambah `key`. `AppShell`: filter dengan izin + Notifikasi; `hideRealizations` dihapus.
    _Requirements: 4.1, 4.2_
  - [x] 8.2 `beforeLoad` di `/dsr`, `/orders`, `/orders/$id`, `/realizations`: tanpa key → redirect ke item nav pertama yang dimiliki, atau ke halaman baru `NoAccess` (brand Merah Menyala, ada tombol logout).
    _Requirements: 4.3_
  - [x] 8.3 `partyScopeLabel` membaca `party_scope` dari `/auth/me`, bukan dari nama peran.
    _Requirements: 7.6_
  - [x] 8.4 Test (**Property 9**): sidebar = izin ∪ Notifikasi untuk beberapa himpunan izin, redirect route, `NoAccess`, label scope.
    _Requirements: 4.2, 4.3, 7.6, 9.2_

- [x] 9. Gate kualitas
  _Model: Sonnet, Medium — jalankan dan perbaiki sampai hijau_
  - `cd backend && go test ./...` (+ integration dengan `DATABASE_URL` localhost), `golangci-lint run --build-tags integration ./...`, `govulncheck ./...`;
  - `cd backend-extapi && go test ./...` dan `cd pkg && go test ./...` bila disentuh;
  - `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build` dan `pnpm --dir frontend/VendorPortal-Vite run test|lint|build`;
  - coverage `internal/rolemgmt` dan `internal/vendorrole` ≥ 80%; hasil dicatat di `### Test Results`.
  _Requirements: 9.1, 9.2_

- [x] 10. Dokumentasi
  _Model: Haiku, Low — teks_
  - CLAUDE.md Sec 3 (kolom `roles.portal`, `roles.party_scope`, `menu_features.portal`, migrasi `047`) dan Sec 12 (portal dikunci ke prefix; katalog Portal_Cabang mencabut D6 `permission-catalog-coverage` untuk VendorPortal); `.claude/docs/data-map.md`, `.claude/docs/decisions.md`, `development-progress.md`; jalankan `graphify update .`.
  _Requirements: 2.1, 3.1_

- [ ] 11. Cek browser manual (dilakukan user)
  - CompanyPortal: kedua tab, pencarian, Buat Peran vendor, ubah izin, deskripsi bersih, form user.
  - VendorPortal: login dengan tiga peran vendor lama + satu peran vendor baru; sidebar dan route sesuai izin; setelah izin dicabut, menu hilang dan API membalas 403.
  - Dicatat "outstanding (user)"; AI tidak pernah menandainya selesai.
  _Requirements: 9.3_

## Notes

### Pre-check

Task 1, 2026-11-25. Hanya baca (file + SELECT ke dev); tidak ada yang diubah.

**1. Teks rusak (mojibake)**
- `roles.description`: 9 peran baseline (`ADMIN`, `ADMIN_PARAM`, `ATM-USER`, `ATM-SPV`, `BRANCH-USER`, `BRANCH-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`, `VENDOR-USER`) memuat `â€”` (hex `c3a2e282ace2809d`) di dev dan di seed `002` baris 6676–6684. Peran lain bersih. `menu_features`: 0 baris rusak.
- `locations`: **16 baris di dev** (id 159, 229, 256, 455, 460, 565, 911, 1097, 1167, 1171, 1417, 1450, 1529, 1692, 1889, 1892); seed `002` hanya memuat 3 (baris 961, 1500, 1742: id 911, 1450, 1692), jadi DB yang dibangun dari migrasi saja cuma punya 3. Kolom rusak: `name` dan `address_line1` (16 baris, isinya sama); `address_line2` dan `city_or_regency` 0.
- **Bukan satu pola.** Di dev ada `Ã‚Â ` (spasi non-pemisah ter-encode ganda, termasuk deret 12 kali), `ÃƒÂ©` (é) dan `Ã¢â‚¬â€œ` (tanda hubung en `–`, ter-encode ganda; ini yang ada di seed). Pola `â€”` → `—` di design **tidak mengenai satu pun baris `locations`**.
- Dampak ke task 2.1: perbaikan `roles.description` aman dengan satu `REPLACE` (`â€”` → `—`). Untuk `locations`, pakai pemetaan per pola (`Ã‚Â ` → spasi biasa atau dibuang, `ÃƒÂ©` → `é`, `Ã¢â‚¬â€œ` → `–`) atau per id; pilihan hasil akhir milik task 2.1/PO. Ini soal isi data, bukan struktur, jadi design tidak berubah, tetapi cakupan "perbaiki teks `locations`" lebih besar daripada yang tersirat di task. Property 1 (idempoten, tanpa sisa) harus mencakup ketiga pola, bukan `â€”` saja.

**2. `backend-extapi` dan peran `VENDOR-*`: celah ada**
- `pkg/userrules.PrivilegedRoles` = `ADMIN`, `APPACCESS`, `ADMIN_PARAM`, `EXT-CLIENT`. Peran `VENDOR-*` tidak termasuk.
- CHECK `api_clients.assignable_roles` (migrasi `039`) hanya menolak empat peran itu. Jadi `extclient set-grants` boleh memasukkan `VENDOR-*` ke allowlist client, dan `CanAssign` (`extauth/client.go:69`, `extuser/service.go:161`) lalu meloloskannya.
- Ext API hanya mengelola user `ldap` (`managedSource`) tanpa `vendor_id`, sedangkan peran vendor memerlukan `auth_source=local` + `vendor_id` (`auth/user_admin.go` `vendorFields`). Hasilnya user berperan vendor tanpa vendor, tidak konsisten.
- Rekomendasi task 4.3: tolak peran berprefix `VENDOR-` di jalur ext API (`CanAssign`/`pkg/userrules`). Jangan ubah `PrivilegedRoles`: tes `privileged_sync_test.go` mengikatnya ke CHECK `039`. Memperkeras CHECK di DB = migrasi baru, di luar task ini.
- `vendorFields` di `backend/internal/auth/user_admin.go`: peran non-vendor hanya menolak `vendor_branch_id`; `vendor_id` pada peran non-vendor tidak ditolak di fungsi itu (sesuai bahasa task 4.3).

**3. DQ3: sumber opsi peran form user**
- `main.go:266-289`: `/api/v1/admin/users` dan `/api/v1/admin/roles` memakai guard yang **sama**: `RequireAuth` + `RequireRoles("APPACCESS","ADMIN")`. Siapa pun yang bisa membuka form user bisa memanggil `GET /api/v1/admin/roles`.
- Keputusan: **pakai `GET /api/v1/admin/roles`** (setelah 3.1 memuat `portal`); tidak perlu `GET .../users/role-options`. Tidak ada celah akses baru. Respons membawa izin dan `active_user_count`, lebih besar dari kebutuhan form, wajar untuk puluhan peran.

**Ringkasan:** tidak ada hasil yang mengubah design, jadi tidak ada STOP. Satu hal memperbesar cakupan 2.1: kerusakan `locations` punya tiga pola dan 16 baris di dev.

### Catatan eksekusi

- **2.1 (2026-11-25)**: `047_role_portal.sql` ditulis, belum diterapkan (task 2.3). Kolom `locations` yang rusak = `name` + `address_line1` (id 911, 1450, 1692; seed `002` baris 961, 1500, 1742). Polanya **bukan** `â€”` seperti di design, melainkan `Ã¢â‚¬â€œ` (en dash yang rusak dua kali); diganti dengan karakter aslinya `–` (U+2013), bukan `—`. `roles.description`: `â€”` → `—`, 9 baris. Pola dicek byte-per-byte terhadap seed `002` (3 dan 9 baris cocok). Post-check juga gagal bila teks rusak masih tersisa. Belum dicek sintaksnya terhadap Postgres (tidak ada DB uji; dicek saat `migrate up` di 2.3). Property 1 di 2.4 harus memakai pola `locations` ini.
- **2.3 (2026-11-25)**: `migrate status` sebelum: 45 applied (s.d. `046`, semua checksum `ok`), 1 pending (`047`). OK user di sesi ini → `migrate up`: `applying 047_role_portal.sql` / `1 applied`, exit 0, post-check lolos (sintaks 047 terbukti di Postgres). `status` sesudah: **46 applied, 0 pending**, `047` checksum `ok`. Cek dev: `roles.portal` = `vendor` untuk 3 peran `VENDOR-*` (party_scope `{replenish}`, `{replenish,vault}`, `{vault}`), `service` untuk `EXT-CLIENT`, `main` untuk 18 peran lain; grant vendor: `VENDOR-USER` + `VENDOR-ATM-USER` = `vendor-dsr, vendor-orders, vendor-realizations`, `VENDOR-VAULT-USER` = `vendor-dsr, vendor-orders` (D6); `locations` 911, 1450, 1692 kini memuat `–`. 13 baris `locations` dev lain dengan pola rusak berbeda dibiarkan (keputusan user, di luar cakupan).

- **6.4 (2026-11-25)**: dependensi dev baru `vitest-axe` ^0.1.0 di CompanyPortal (disetujui user; dipasang sesi orkestrator); matcher didaftarkan di `src/test/setup.ts`. Test baru: `useRoleManagementUrlState.test.ts`, vendor dialog + axe di `CreateRoleDialog.test.tsx`, katalog portal aktif + axe (tab main/vendor) di `RoleManagementPage.test.tsx`. Aturan `color-contrast` dimatikan di axe (jsdom tanpa canvas; kontras dicek visual). Axe tanpa pelanggaran. `tsc` bersih, lint bersih, vitest penuh 210 file / 2023 test lulus. Cek browser: outstanding (user).
- **2.4 (2026-11-25)**: `backend/internal/rolemgmt/role_portal_migration_integration_test.go` (`//go:build integration`, tx di-rollback). Property 1 memakai pola `locations` `Ã¢â‚¬â€œ` → `–` (id 911, 1450, 1692) dan `â€”` → `—` (9 peran); 13 baris dev lain tidak diasersi. Run `go test -tags integration ./internal/rolemgmt -run 'Property[123]_'` dengan DATABASE_URL localhost: 3 PASS (bukan skip).
- **5.1 (2026-11-25)**: `RequireVendorPermission(e, key)` + `RequireVendorRole()` di `internal/rolemgmt/evaluator.go` (cek vendor = `vendorrole.IsVendor`, prefix; `key` kosong → panic saat mount), `VendorAlwaysOnKeys` di `keys.go`. Belum dipasang ke route (task 5.2). Test `vendor_middleware_test.go` (table-driven, fake replica): vendor dengan/tanpa key, peran vendor baru berprefix, internal lolos tanpa query, error → 500 tanpa handler (P7), cabut izin → 403 di request berikutnya (P8), 401 tanpa token. `go build ./...`, `go vet`, `golangci-lint ./internal/rolemgmt/...` (0 issues), `go test ./internal/rolemgmt` PASS; coverage paket 68.6% tanpa integration (gate 80% = task 9). `-race` tidak jalan (CGO mati di mesin ini).
- **3.2 (2026-11-25)**: `service.go`: `CreateRoleRequest` + `Portal`/`PartyScope`; `validatePortalScope` (portal wajib `main`/`vendor`; vendor = prefix `VENDOR-` + scope subset tidak kosong dari {replenish, vault}, disimpan urut tanpa duplikat; main = tanpa prefix `VENDOR-` + scope kosong) → `ValidationError` sebelum query/tx. `resolvePermissionSet(db.Role, …)`: id katalog yang dikirim dengan `portal ≠ role.portal` → `RoleNotAssignableError` (juga bila sudah dimiliki, dan untuk peran `service`); `assignable_roles` hanya untuk katalog `main`. Audit `role_created` = baris `db.Role` (sudah memuat `portal` + `party_scope`); guard APPACCESS/ADMIN tidak berubah. Test lama disesuaikan (`Portal: PortalMain`, termasuk satu baris di `evaluator_integration_test.go`); test baru di `service_test.go`: `TestValidatePortalScope`, `TestCreateRole_InvalidPortalTouchesNothing`, `TestResolvePermissionSet_CrossPortal` (3.4 boleh memperluas). **Sampai 3.3 selesai, `POST /admin/roles` membalas 422 `portal`** karena handler belum meneruskan `portal`. `go test` (unit + `-tags integration`, DATABASE_URL localhost, tidak ada skip) hijau, diulang tanpa overlay setelah 5.1 selesai: `go vet -tags integration`, `go test -tags integration ./internal/rolemgmt` PASS, `golangci-lint ./internal/rolemgmt/...` 0 issues, `go build ./...` OK.
- **4.2 (2026-11-25)**: `NewVendorOrderService(pool, readPool, scopes vendorrole.ScopeReader)`; `scope()` membaca `scopes.PartyScope` (replica), error → error dibungkus (500, fail closed), kosong → `ErrNotAuthorized` (403). `notifyParties` memakai `q.ListVendorRolesForParty` di tx pemanggil. `main.go` hanya menyuntik `roleMgmtRepo`; integration test vendor order memakai `rolemgmt.NewRepository`. `go build ./...`, `go test ./...` hijau; `go test -tags integration ./internal/service ./internal/notification` (DATABASE_URL localhost) hijau, bukan skip. Tes Property 6/7 milik 4.4.
- **5.2 (2026-11-25)**: `main.go`: `vendorPerm(key)` = `rolemgmt.RequireVendorPermission(permEval, key)`; `/api/v1/dsr` → `authed + vendorPerm("vendor-dsr")`, `/vendor/replenish-orders` → `vendor-orders`, `/vendor/replenish-realizations` → `vendor-realizations`; satu `vendorOrderHandler` untuk kedua mount. `NewVendorOrderHandler(svc, perm VendorPermFunc)`: `perm` nil → panic saat start (fail closed); `Routes()` dan `RealizationRoutes()` memakai `rolemgmt.RequireVendorRole()`; `POST /{id}/realization` ditambah `perm("vendor-realizations")`. `vendorrole.All`/`PartyRoles`/`ForParty` (+ tesnya) dihapus; konstanta nama peran tetap (dipakai fixture test). Pemanggil test yang dipindah: `notification/integration_test.go`, `service/vendor_request_rules_integration_test.go` (+ `VENDOR-AUDIT`), `handler/{vendor_order,replenishment,openapi_contract}_handler_test.go` (helper `allowVendorPerm`). `cmd/api/route_permissions_test.go`: tiga baris kunci vendor dengan `minMW 2` menggantikan pengecualian `/dsr` dan `/vendor`. Tes baru: `TestVendorOrderHandler_RealizationNeedsKey`, `TestNewVendorOrderHandler_NilPermPanics`. `go build ./...`, `go vet` (+ `-tags integration`), `golangci-lint` untuk paket yang disentuh (0 issues), `go test ./...` hijau kecuali `TestBodyLimit_Router` (gagal karena decoder strict PUT izin dari 3.3 menolak field `pad` di tes itu, bukan dari 5.2); `go test -tags integration` (DATABASE_URL localhost) `notification`, `service`, `rolemgmt`, `cmd/api` PASS, bukan skip. Property 5/7/8 di level mount = task 5.4.
- **4.3 (2026-11-25)**: `vendorFields`: peran non-vendor + `vendor_id` (sumber bukan `ldap`) → `ReferenceError{Field:"vendor_id"}` (400 `invalid_reference`). LDAP + `vendor_id` tetap `ErrLdapFieldsNotAllowed` (kontrak lama dipertahankan); celah sisa: **update** user LDAP dengan peran non-vendor + `vendor_id` tetap lolos seperti sebelumnya (di luar Req 2.5). `pkg/userrules.IsVendorRole` (prefix `VENDOR-`, trim + case-insensitive, sama dengan `vendorrole.IsVendor`); `extauth.Client.CanAssign` menolaknya → ext API membalas **403 `role_not_allowed`** (kontrak yang sudah ada untuk peran di luar allowlist), **bukan 422** seperti teks Req 2.5; ganti ke 422 = perubahan kontrak OpenAPI extapi, perlu keputusan PO. **Keputusan user 2026-11-25: tetap 403 `role_not_allowed`** (deviasi tercatat dari teks Req 2.5; kontrak ext API tidak berubah). `PrivilegedRoles`, CHECK `039`, `EXT-CLIENT` tidak berubah; tanpa migrasi. Test lama yang memberi peran internal + `vendor_id` pada user local disesuaikan (kini `VENDOR-USER`, atau diharapkan ditolak); tambah `TestVendorFields_RoleAuthSourceVendorID`, `TestIsVendorRole`, kasus `VENDOR-*` di `TestClientCanAssign`. `go test ./...` hijau di `pkg` dan `backend-extapi`; `backend`: `internal/auth` hijau (unit + `-tags integration`, DATABASE_URL localhost, tanpa skip), `go build ./...` OK; satu-satunya merah `internal/handler` `TestBodyLimit_Router`, akibat `DisallowUnknownFields` baru di `role_mgmt_handler.go` milik task sibling (test memakai field `pad`), bukan task ini.

- **10 (2026-11-25, dokumentasi)**: `CLAUDE.md` Sec 3 (DB map `roles.portal`, `roles.party_scope`, `menu_features.portal`, migrasi `047`), Sec 12 (portal dikunci ke prefix; katalog Portal_Cabang menggantikan pengecualian VendorPortal di `permission-catalog-coverage` D6), Sec 12 migration list (`047` applied dev 2026-11-25, 46 applied, 0 pending, rollback dev, next free `048`); `.claude/docs/data-map.md` (section "Role portal (migration 047)"); `.claude/docs/decisions.md` (tiga keputusan user 2026-11-25: ext API 403 `role_not_allowed` vs 422 di Req 2.5, `vitest-axe` 0.1.0, perbaikan data hanya 9 `roles.description` + 3 `locations` seed); `.claude/development-progress.md` (baris role management). `graphify update .` dijalankan. Tidak ada kode, migrasi, atau tes yang diubah.

### Test Results

Task 9, 2026-11-25. `DATABASE_URL` = localhost:5432 (dev), integration dijalankan dengan `-tags integration -p 1`.

| Cek | Hasil |
|---|---|
| `backend`: `go build ./...`, `go vet ./...`, `go test ./... -count=1` | PASS |
| `backend`: `go test -tags integration ./... -p 1` | PASS; 0 `--- SKIP` pada rolemgmt/vendorrole/handler/service/cmd/api/auth/notification (bukan skip) |
| `backend`: `golangci-lint run --build-tags integration ./...` | PASS, 0 issues |
| `backend`: `govulncheck ./...` | PASS; 0 kerentanan terpanggil (1 di modul yang di-require, tidak dipanggil kode) |
| `backend-extapi`: `go test ./...` | PASS |
| `pkg`: `go test ./...` | PASS |
| Coverage `internal/rolemgmt` | 91.7% dengan integration (74.8% unit saja); ≥ 80% OK |
| Coverage `internal/vendorrole` | 100.0% |
| Coverage lain (info): `internal/auth` 92.0%, `internal/handler` 82.8% (integration) | - |
| CompanyPortal `test` | PASS 210 file / 2023 test |
| CompanyPortal `lint` (biome) | PASS |
| CompanyPortal `build` (`tsc -b` + vite + check-bundle) | PASS (initial JS 201.4 KB gzip, budget 300) |
| VendorPortal `test` | PASS 30 file / 262 test |
| VendorPortal `lint` (biome) | PASS |
| VendorPortal `build` | PASS |

**Perbaikan di task ini (semua berasal dari perubahan spec ini):**
- CompanyPortal `build` gagal `tsc` (3 error, lint/vitest tidak menangkapnya): `admin-users/lib/useRoleOptions.ts` membandingkan `Portal` dengan `"service"` → dicast ke `string`; `role-management/components/CreateRoleDialog.tsx` resolver/`onSubmit` bertipe `DialogFormValues` (field `party_scope`) → `zodResolver(...) as unknown as Resolver<DialogFormValues>`, `onSubmit(values: DialogFormValues)` dan `description` dikirim eksplisit. Perilaku tidak berubah.
- VendorPortal `lint` gagal (5 file format, ujung baris CRLF; HEAD memakai LF): `biome check . --write` pada `AppShell.tsx`, `AuthContext.tsx`, `vendorScope.test.tsx`, `types.ts`, `vendorRoles.ts`.

**Catatan:** `exports.preview.test.tsx` sekali time-out 5 dtk saat dijalankan paralel dengan suite Go (flaky karena beban); lulus sendiri (21/21) dan lulus di run penuh ulang. `TestBodyLimit_Router` (merah di catatan 5.2/4.3) kini hijau. `-race` tidak dijalankan (CGO mati di mesin ini).

**Cek browser manual (task 11): outstanding (user).**

## Task Dependency Graph

- **Wave 0:** 1 (pre-check), 2.1 (migrasi), 4.1 (`vendorrole`), 8.1 (VendorPortal auth + sidebar; key sudah ditetapkan di design).
- **Wave 1:** 2.2 (needs 2.1), 2.3 (needs 2.1 + OK user), 8.2 (needs 8.1).
- **Wave 2:** 2.4 (needs 2.3), 3.1 (needs 2.1).
- **Wave 3:** 3.2 (needs 3.1), 4.2 (needs 3.1, 4.1), 5.1 (needs 3.1).
- **Wave 4:** 3.3 (needs 3.2), 4.3 (needs 1, 4.1), 5.2 (needs 4.2, 5.1; menyentuh `main.go` setelah 4.2), 5.3 (needs 3.1).
- **Wave 5:** 3.4 (needs 3.3), 4.4 (needs 4.2, 4.3), 5.4 (needs 5.2, 5.3, 2.3), 5.5 (needs 5.2), 6.1 (needs 3.3).
- **Wave 6:** 6.2 (needs 6.1), 7.1 (needs 1, 6.1), 7.2, 8.3 (needs 5.3, 8.1).
- **Wave 7:** 6.3 (needs 6.2), 7.3 (needs 7.1, 7.2), 8.4 (needs 8.2, 8.3).
- **Wave 8:** 6.4 (needs 6.3).
- **Wave 9:** 9 (gate kualitas, setelah semua task kode).
- **Wave 10:** 10 (dokumentasi), 11 (cek browser oleh user).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "2.1", "4.1", "8.1"] },
    { "id": 1, "tasks": ["2.2", "2.3", "8.2"] },
    { "id": 2, "tasks": ["2.4", "3.1"] },
    { "id": 3, "tasks": ["3.2", "4.2", "5.1"] },
    { "id": 4, "tasks": ["3.3", "4.3", "5.2", "5.3"] },
    { "id": 5, "tasks": ["3.4", "4.4", "5.4", "5.5", "6.1"] },
    { "id": 6, "tasks": ["6.2", "7.1", "7.2", "8.3"] },
    { "id": 7, "tasks": ["6.3", "7.3", "8.4"] },
    { "id": 8, "tasks": ["6.4"] },
    { "id": 9, "tasks": ["9"] },
    { "id": 10, "tasks": ["10", "11"] }
  ]
}
```
