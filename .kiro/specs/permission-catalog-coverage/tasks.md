# Implementation Plan: Katalog izin lengkap + izin dipakai

> Status: accepted 2026-11-24 (engineer) — "Terima, siapkan Wave 0". Berdasarkan `requirements.md` (accepted 2026-11-20, PO; + AC 3.6) dan `design.md` (accepted 2026-11-20, engineer).

## Overview

Urutan pengerjaan:
1. Migrasi `046` ditulis, lalu diterapkan ke dev. Langkah `migrate up` butuh OK user (CLAUDE.md Sec 4 #7).
2. Inti `rolemgmt` di backend ATM: konstanta key, query sqlc, middleware any-of, dan filter di editor (PUT).
3. Endpoint `GET /api/v1/auth/me/permissions`.
4. Pemasangan `RequirePermission` di setiap mount sesuai tabel design, lalu test route-walk dan test RBAC denial.
5. CompanyPortal inti: key + store izin + guard route `requirePermission` dengan halaman 403.
6. CompanyPortal navigasi: sidebar, command palette, hub Pengaturan, dan semua route.
7. Editor izin (badge, nonaktif, `assignable_roles`).

Spec ditutup dengan gate kualitas, dokumentasi, dan cek browser oleh user. `/security-review` dijalankan di tahap 5 (`review.md`).

Integration test berjalan terhadap DB dev (`DATABASE_URL`, `localhost`), jadi `046` harus sudah diterapkan (task 1.3) sebelum task 1.4 dan 4.5.

**Jendela merah:** antara task 4.2/4.3 (backend memakai izin) dan task 6.3 (frontend memakai izin), menu bisa tampil untuk role yang API-nya sudah ditolak. Hanya terjadi di dev, dan rilis memuat keduanya sekaligus.

Pola yang dirujuk:
- Seed menu + `role_permissions` + `granted_by` ADMIN pertama: migrasi `027`, `041`, `044`.
- Integration test migrasi: `backend/internal/repository/vendor_pks_list_migration_integration_test.go`.
- Middleware + evaluator: `backend/internal/rolemgmt/evaluator.go` (+ `evaluator_integration_test.go`).
- Guard route frontend: `routes/_protected.tsx` (`requireRoles`, `isAuthLoading`).
- Store: `lib/auth/store.ts` (`useAuthStore`).
- sqlc v1.31.1 + perbaikan `UserLeafe` → `UserLeave`.
- Skill: `ecc:golang-patterns` / `ecc:golang-testing` (Go), `ecc:react-patterns` / `ecc:react-testing` / `ui-ux-pro-max` (task 5–7).

### Model Split

**Opus-required** (zona bahaya: auth/RBAC, migrasi data)
- 1.1 Migrasi `046` — matriks grant, kolom baru, gagal keras
- 1.3 Terapkan `046` ke dev — butuh OK user
- 2.3 `RequirePermission` any-of + Always_On + role non-internal — keputusan akses inti
- 2.4 Filter Always_On/Locked + 422 `assignable_roles` di PUT izin — RBAC + audit di tx
- 4.2 Pemasangan di `main.go` — salah mount = kebocoran atau terkunci
- 4.3 Pemasangan per route di handler — memisahkan guard viewer dari guard aksi

**Sonnet-runnable**
- 1.2 Skrip rollback dev (needs 1.1)
- 1.4 Integration test migrasi, Property 2 + 3 (needs 1.3)
- 2.1 Konstanta key Go
- 2.2 Query + sqlc (needs 1.1)
- 2.5 Test `rolemgmt`, Property 4, 5, 6 (needs 2.3, 2.4)
- 3.1 Endpoint izin saya (needs 2.2, 2.3)
- 3.2 Test endpoint, Property 10 (needs 3.1)
- 4.1 Ekstrak `buildRouter` (tanpa ubah perilaku)
- 4.4 Test route-walk (needs 4.2, 4.3)
- 4.5 Test RBAC denial, Property 7 (needs 4.2, 4.3, 1.3)
- 5.1 Key + store izin frontend
- 5.2 `requirePermission` + halaman 403 (needs 5.1)
- 5.3 Test guard + store, Property 9 (needs 5.2)
- 6.1 Sidebar + command palette (needs 5.1)
- 6.2 Hub Pengaturan (needs 5.1)
- 6.3 Ganti `requireRoles` di semua route (needs 5.2)
- 6.4 Test nav + cakupan katalog, Property 1 + 8 (needs 6.1–6.3, 1.4)
- 7.1 Editor izin (needs 5.1, 2.4)
- 7.2 Test editor (needs 7.1)
- 8 Gate kualitas

**Haiku**
- 9 Dokumentasi
- 10 Cek browser (dilakukan user)

## Tasks

- [ ] 1. Migrasi `046_permission_catalog_coverage.sql`
  _Model: Opus (1.1, 1.3) / Sonnet (1.2, 1.4), High — migrasi data + RBAC (Sec 4 #7)_
  - [x] 1.1 Tulis `backend/migrations/046_permission_catalog_coverage.sql`, satu tx:
    - `ALTER TABLE menu_features ADD COLUMN assignable_roles text[]` + `COMMENT`;
    - menu `approvals` (sort 9), `exports` (sort 10);
    - 19 fitur dari Requirement 1.1, parent via `SELECT id … WHERE key = …`;
    - isi `assignable_roles` untuk key dengan guard tetap (tabel mount design);
    - grant awal sesuai matriks D9 + parent menu, `granted_by` = ADMIN pertama, `ON CONFLICT DO NOTHING`;
    - header berisi rollback.
    _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 2.5, 3.6_
  - [x] 1.2 `backend/scripts/dev/rollback_046_permission_catalog_coverage.sql` (hapus grant + baris key baru, `DROP COLUMN assignable_roles`).
    _Requirements: 2.4_
  - [x] 1.3 `go run ./cmd/migrate status`, lalu **minta OK user**, lalu `go run ./cmd/migrate up` ke dev; catat hasil `status`.
    _Requirements: 2.3_
  - [x] 1.4 Integration test migrasi (`//go:build integration`, skip tanpa `DATABASE_URL`):
    - **Property 2**: matriks role × key = tabel D9;
    - **Property 3**: isi 046 dijalankan ulang di tx yang di-rollback, jumlah baris sama, baris lama utuh;
    - tulis fixture `frontend/CompanyPortal-Vite/src/lib/auth/__fixtures__/catalog-keys.json` (daftar key katalog) bila env `UPDATE_CATALOG_FIXTURE=1`.
    _Requirements: 2.1, 2.2, 2.3, 2.5, 9.3_

- [ ] 2. Inti `rolemgmt` backend ATM
  _Model: Sonnet (2.1, 2.2, 2.5) / Opus (2.3, 2.4), High — keputusan akses + audit_
  - [x] 2.1 `backend/internal/rolemgmt/keys.go`: `AlwaysOnKeys`, `LockedKeys`, `IsInternalRole(role)`, `IsAlwaysOn`, `IsLocked`.
    _Requirements: 3.1, 4.3_
  - [x] 2.2 `backend/queries/role_mgmt.sql`: `HasAnyRolePermission(role, keys[])`, `ListRoleFeatureKeys(role)`, dan kolom `assignable_roles` di query katalog/izin. Jalankan `sqlc generate` (v1.31.1) lalu perbaiki `UserLeafe` → `UserLeave`.
    _Requirements: 4.1, 7.1, 3.6_
  - [x] 2.3 `RequirePermission(e, keys ...string)`:
    - any-of;
    - Always_On_Key + role internal → izinkan tanpa query;
    - role non-internal → 403;
    - error repo → 500 + `slog.Error` (role, keys).
    `Repository.HasAnyPermission` di replica.
    _Requirements: 3.1, 7.1, 7.2, 7.3, 7.4, 8.1_
  - [x] 2.4 `PermissionService.UpdateRolePermissions`:
    - buang ID Always_On/Locked dari set baru **dan** set lama sebelum diff;
    - grant di luar `assignable_roles` → `ErrRoleNotAssignable` → handler 422 `role_not_assignable` (pesan berisi label layar);
    - audit di tx tetap.
    _Requirements: 3.5, 3.6, 8.4_
  - [x] 2.5 Test table-driven:
    - **Property 4** (any-of, always-on, non-internal);
    - **Property 5** (repo error → 500, handler tidak dipanggil);
    - **Property 6** (PUT dengan key selalu aktif/terkunci + grant lama `settings.roles` ADMIN tidak berubah);
    - 422 `assignable_roles`.
    _Requirements: 3.1, 3.5, 3.6, 7.1, 7.2, 7.3, 7.4, 8.4_

- [ ] 3. Endpoint `GET /api/v1/auth/me/permissions`
  _Model: Sonnet, Medium — endpoint baca di pola auth handler_
  - [x] 3.1 `AuthHandler.MyPermissions`, di belakang `RequireAuth`, baca primary:
    - grant role ∪ Always_On ∪ (Locked bila APPACCESS/ADMIN);
    - role non-internal → `[]`;
    - flat JSON `{"permissions": [...]}` urut `sort_order`.
    _Requirements: 4.1, 4.2, 4.3, 4.4, 8.3_
  - [x] 3.2 Test handler (**Property 10**): ADMIN, APPACCESS, ATM-USER, VENDOR-USER, tanpa token → 401, dan sesudah PUT izin → hasil baru.
    _Requirements: 4.1, 4.2, 4.3, 4.4_

- [ ] 4. Pasang `RequirePermission` di backend ATM
  _Model: Sonnet (4.1, 4.4, 4.5) / Opus (4.2, 4.3), High — perubahan grup RBAC route_
  - [x] 4.1 Ekstrak pembuatan router di `backend/cmd/api/main.go` ke `buildRouter(deps)` tanpa mengubah perilaku, supaya test bisa mem-walk route. Jalankan `go test ./...` hijau sebelum lanjut.
    _Requirements: 9.2_
  - [x] 4.2 `main.go`, sesuai tabel mount design:
    - `dmaa-forecast`, `audit-logs`, `atm-portal` (diperketat, DQ3), dan `vault-plans`: `RequireRoles` viewer diganti `RequirePermission`;
    - `vendorRead` diganti any-of `settings.vendors`/`settings.pks`;
    - `masterDataAdmin`, cash-count, `acm-areas`, dan `/admin/approval/*`: `RequirePermission` (per route untuk RBAC) **ditambahkan** di depan `RequireRoles` yang tetap;
    - `/admin/users`, `/admin/roles`, `/approvals`, `/notifications`, `/auth`, `/dsr`, dan `/vendor/*` tidak diubah.
    _Requirements: 3.3, 7.1, 7.2, 7.5, 7.6, 7.7_
  - [x] 4.3 Handler:
    - `vendor_request_handler.go`: route viewer → any-of `replenish.vendor-requests`/`replenish.forecast-browser`; route aksi → `RequirePermission("replenish.vendor-requests")` + role aksi tetap;
    - `atm_visit_quota_handler.go`: sama, viewer + aksi;
    - `dsr_late_report_handler.go`: `r.Use` → `monitoring.dsr-late-report`;
    - `replenishment_handler.go` `Routes` → `replenish.schedule` (`VendorRoutes` tetap);
    - `admin_holiday_handler.go`: tambah `forecasting.holidays` + ADMIN tetap;
    - `admin_vendor_handler.go` / `admin_pks_handler.go`: guard tulis tetap.
    Evaluator disuntik lewat konstruktor `With…`.
    _Requirements: 7.1, 7.2, 7.5_
  - [x] 4.4 Test route-walk: `chi.Walk` atas `buildRouter`; setiap route di prefix yang dipindah punya pemetaan key (tabel di test = tabel design), dan prefix yang dikecualikan terdaftar eksplisit.
    _Requirements: 9.2_
  - [x] 4.5 Integration test RBAC denial per baris tabel mount (**Property 4, 7**):
    - role tanpa key → 403;
    - role dengan key → bukan 403;
    - mount dengan guard tetap: role ber-key di luar guard → 403;
    - VENDOR-USER → 403 di `atm-portal`;
    - Manajemen Peran tetap untuk APPACCESS/ADMIN.
    _Requirements: 3.3, 7.1, 7.2, 7.3, 7.5, 7.6, 9.4_

- [ ] 5. CompanyPortal inti: key, store izin, guard route
  _Model: Sonnet, High — guard akses frontend_
  - [x] 5.1 `src/lib/auth/permissionKeys.ts`: union `ScreenKey`, `SCREEN_KEYS`, `ALWAYS_ON_KEYS`, `LOCKED_KEYS`. `src/lib/auth/permissionStore.ts` (Zustand):
    - `load()` memanggil `/api/v1/auth/me/permissions`, dipanggil sesudah login dan saat sesi dipulihkan;
    - `status` = `idle | loading | ready | error`;
    - `has` / `hasAny`; saat `error` hanya Always_On.
    `logout` mengosongkan store.
    _Requirements: 4.1, 5.4, 5.5, 8.2_
  - [x] 5.2 `routes/_protected.tsx`:
    - `requirePermission(key | key[], { roles? })` menggantikan `requireRoles`; tanpa bypass ADMIN/ADMIN_PARAM;
    - menunggu saat `isAuthLoading` / izin `loading`;
    - tidak punya key → `throw new ForbiddenError()`;
    - `errorComponent` di `protectedRoute` me-render halaman 403 (Merah Sirih, tautan ke Dashboard).
    _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_
  - [x] 5.3 Test (**Property 9**):
    - tanpa key → 403 dan komponen tidak di-mount;
    - `opts.roles` sempit;
    - belum login → `/login`;
    - loading menunggu;
    - store: `error` → hanya Always_On, `load` ulang.
    _Requirements: 5.5, 6.1, 6.2, 6.3, 6.4, 6.5_

- [ ] 6. CompanyPortal navigasi + route
  _Model: Sonnet, Medium — mengganti sumber visibilitas di pola yang ada_
  - [x] 6.1 `lib/config/navigation.ts`:
    - `roles` → `permission: ScreenKey`;
    - `filterNavByPermissions` (tanpa bypass), dan `getVisibleNav` memakai store;
    - `Sidebar` + `CommandPalette`: saat loading hanya Always_On; saat error tampil banner "Menu gagal dimuat" + "Coba lagi".
    _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_
  - [x] 6.2 `SettingsHubPage`: `HubCard.permission`, hapus `canSeeCard`/`ATM_SPV_CARD_IDS`, kategori kosong tetap disembunyikan. Route `/settings` memakai `requirePermission` any-of key kartu hub.
    _Requirements: 5.1, 5.2_
  - [x] 6.3 Ganti `requireRoles` di semua route (`routes/**`) dengan `requirePermission(<key>)` sesuai Requirement 1.1. Tambahkan guard ke route yang belum punya (`/approvals` = Always_On, `/cash-flow`, `/atm-portal*`, `/replenishment`, `/vendor-requests/new`). `/exports/$dataset` memakai `{ roles: ["ADMIN","ADMIN_PARAM"] }`. Hapus `requireRoles` bila tidak dipakai lagi.
    _Requirements: 1.4, 6.2, 6.3_
  - [x] 6.4 Test:
    - **Property 8**: `navigation.property.test.ts` diperbarui (himpunan izin acak = item terlihat; loading/error = Always_On);
    - **Property 1**: test cakupan baru — setiap route terlindungi dan setiap item `NAV_CONFIG` aktif punya key; `SCREEN_KEYS` = `catalog-keys.json`;
    - test `SettingsHubPage` (kartu per izin) diperbarui;
    - test lama yang memakai role diperbarui ke izin.
    _Requirements: 1.1, 1.4, 5.1, 5.2, 5.3, 5.4, 5.5, 9.1_

- [ ] 7. Editor izin (`features/role-management`)
  _Model: Sonnet, Medium — komponen dalam pola yang ada_
  - [x] 7.1 `PermissionEditor`:
    - Always_On tercentang + nonaktif + badge "Selalu aktif";
    - Locked nonaktif + badge "Hanya APPACCESS/ADMIN";
    - role di luar `assignable_roles` → nonaktif + tooltip daftar role;
    - payload PUT tanpa key Always_On/Locked;
    - pesan error 422 `role_not_assignable`;
    - sesudah simpan role milik sendiri → `permissionStore.load()`.
    Tipe katalog ditambah `assignable_roles`.
    _Requirements: 3.2, 3.4, 3.6, 8.3_
  - [x] 7.2 Test editor: badge + nonaktif, payload tanpa key terkunci, error 422, reload izin sendiri.
    _Requirements: 3.2, 3.4, 3.6, 8.3_

- [x] 8. Gate kualitas
  _Model: Sonnet, Medium — menjalankan gate dan memperbaiki sampai hijau_
  - `cd backend && go test ./...` + `go test -tags integration ./...` (DB dev), `golangci-lint run --build-tags integration ./...`, `govulncheck ./...`;
  - `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build`;
  - `graphify update .`;
  - catat hasil di `## Notes` → `### Test Results`.
  _Requirements: 9.1, 9.2, 9.3, 9.4_

- [x] 9. Dokumentasi
  _Model: Haiku, Low — teks_
  - CLAUDE.md §3: baris migrasi `046` + kolom `assignable_roles`;
  - CLAUDE.md §12: katalog izin dipakai sidebar/route/API, Always_On/Locked, guard tetap;
  - `docs/data-map.md` dan `docs/decisions.md` (D1–D11, DQ1–DQ3);
  - `development-progress.md`.
  _Requirements: 7.6, 7.7_

- [ ] 10. Cek browser manual — **dilakukan user** (Golden Rule #10)
  _Model: Haiku, Low — dilakukan user_
  Login per role (ADMIN, ADMIN_PARAM, APPACCESS, ATM-USER, ATM-SPV, BRANCH-ATM-SPV, ACM-USER, CC-CWM): sidebar dan hub sesuai matriks D9, URL langsung tanpa izin → 403, dan cabut/beri izin di `/settings/roles` → berlaku sesudah reload. Hasil dicatat "outstanding (user)" sampai user melapor.
  _Requirements: 5.1, 6.1, 8.2_

## Notes

### Keputusan yang tercermin
- D1 (per layar) → 1.1, 6.3. D2 (tanpa izin aksi) → 4.3 (role aksi tetap). D3 (backend ikut) → 4.2–4.5. D4 (role di service tetap) → 4.2, 4.3. D5 (tanpa bypass ADMIN) → 1.1, 5.2, 6.1. D6 (VendorPortal di luar) → 2.1, 4.2. D7 (Always_On) → 2.1, 2.3, 7.1. D8 (Locked) → 2.4, 4.2, 7.1. D9 (grant awal) → 1.1, 1.4. D10 (`backend-exq` tetap) → 4.2. D11 (menu nonaktif tidak didaftarkan) → 1.1, 6.4.
- DQ1 (`assignable_roles`) → 1.1, 2.4, 7.1. DQ2 (perubahan menu) → 1.1, 10. DQ3 (ATM Portal diperketat) → 4.2, 4.5.

### Di luar spec ini
- Izin di `backend-exq` (ekspor) dan Python EOD; API `cash-flow` yang belum ada; VendorPortal.

### Deploy
- Urutan rilis: migrasi `046` → backend ATM → CompanyPortal, dalam satu jendela rilis di luar 07:00–20:00. Backend lama tetap jalan dengan `046` karena migrasinya additive.

### Definition of Done
- CLAUDE.md §11, ditambah `/security-review` lulus di `review.md` (auth, Sec 4 #7).

### Test Results
- **Task 8 gate kualitas (2026-11-24)**:
  - Backend: `go build ./...` OK; `go test -count=1 ./...` hijau (10 paket ber-test `ok`); `go test -count=1 -tags integration ./...` (DB dev, `DATABASE_URL` host.docker.internal→localhost) hijau (11 paket `ok`, tanpa FAIL); `golangci-lint run --build-tags integration ./...` 0 issues.
  - `govulncheck ./...`: 0 kerentanan terpanggil, 0 di paket yang diimpor; 1 di modul yang di-require, tidak dipanggil kode: **GO-2026-5932**, `golang.org/x/crypto@v0.58.0`, fixed `v0.58.1-0.20261009181718-2f1f834330b2` — **tidak dinaikkan** (menunggu keputusan user; bukan akibat spec ini).
  - Frontend CompanyPortal-Vite: `pnpm test` 208 file / 1998 test hijau; `tsc --noEmit` bersih; `pnpm lint` 0 error, 0 warning (2 warning `useExhaustiveDependencies` di `lib/config/navigation.ts` dari 6.1 diperbaiki: komentar `biome-ignore` dipindah ke atas `const visible = useMemo(`; perilaku sama, `src/lib/config` + `components/layout` 156 test hijau ulang); `pnpm build` OK (initial JS 201.4 KB gzip, budget 300 KB).
  - Cek browser manual (task 10): **outstanding (user)**.

### Catatan eksekusi
- **9 (2026-11-24)**: CLAUDE.md §3 (bullet `046` + `assignable_roles`), §12 (katalog dipakai sidebar/route/API, Always_On/Locked, guard tetap; bullet baseline ke `046`, next `047`). `.claude/docs/data-map.md` (bagian migrasi 046 + aturan `assignable_roles`) dan `.claude/docs/decisions.md` (D1–D11, DQ1–DQ3). `development-progress.md`: baris spec, build + tes selesai, review (stage 5 + `/security-review`) dan cek browser outstanding. Tidak ada perilaku kode yang berubah.
- **1.1 (2026-11-24)**: `046` ditulis, belum diterapkan (task 1.3). Guard gagal keras bila menu induk, role matriks D9, atau user ADMIN tidak ada; post-check 21 key baru. `assignable_roles` diisi hanya untuk key yang endpoint **baca**-nya punya guard role tetap: `settings.atms/.regions/.pks-types/.master-data-io` {ADMIN, ADMIN_PARAM}, `settings.rbac-*` {ADMIN, ADMIN_PARAM, APPACCESS}, `settings.acm-areas` + `forecasting.holidays` {ADMIN}, `cash-count.parameters` {ADMIN, CC-CWM, CC-CCR}, `cash-count.pic-mapping` {ADMIN, CC-CWM}, Locked_Key {APPACCESS, ADMIN}. NULL untuk key yang hanya punya guard aksi (`replenish.vendor-requests`, `settings.vendors` lewat `vendorRead`, `settings.pks`, `cit.vault-plan`) — keputusan penafsiran, koreksi di review bila salah. Sort fitur baru: replenish 0–2, monitoring 0–3, forecasting 0–1, settings 3–12. SQL belum dijalankan ke DB mana pun; diverifikasi oleh 1.3 + 1.4.
- **2.3 (2026-11-24)**: `ReplicaReader` kini satu method `HasAnyPermission(ctx, role, keys)`; `Repository.HasAnyPermission` (replica, sqlc `HasAnyRolePermission`) + entri di `repository_topology_test.go`. `PermissionEvaluator.HasPermission` tetap ada (delegasi satu key, tanpa aturan Always_On) untuk `evaluator_integration_test.go`. `RequirePermission` urutannya: tanpa auth → 401; role non-internal → 403 tanpa query; ada Always_On_Key → lolos tanpa query; selain itu satu query any-of; error → 500 + `slog.ErrorContext` (role, keys, error). Pemanggilan tanpa key → panic saat mount (gagal saat start, bukan saat request). Locked_Key tidak diberi perlakuan khusus di middleware (design: `/admin/users`/`/admin/roles` tidak memakai `RequirePermission`). Test middleware lama dipindah dari key `dashboard` (kini Always_On) ke `monitoring.audit-logs`; ditambah tabel kecil any-of/always-on/non-internal + panic. Tabel Property 4/5 lengkap tetap di 2.5. `go test ./internal/rolemgmt/ ./internal/handler/ ./cmd/...` hijau; integration rolemgmt **skip** (tanpa `DATABASE_URL`); `golangci-lint --build-tags integration ./internal/rolemgmt/...` 0 issues.
- **2.4 (2026-11-24)**: `resolvePermissionSet` (service.go): set tersimpan = (kiriman − Always_On/Locked) ∪ (grant lama yang Always_On/Locked); lewat `ReplaceRolePermissions` yang sama (delete-then-insert), jadi `granted_by` baris Always_On/Locked yang dipertahankan berganti ke aktor — set-nya tetap (Property 6). Katalog dibaca lewat `ListCatalog` (replica, hanya aktif); ID di luar katalog aktif lolos tanpa aturan. `assignable_roles` hanya dicek untuk grant **baru** (belum dimiliki role), case-insensitive — grant lama yang melanggar tidak memblokir simpan (keputusan penafsiran, koreksi di review bila salah). Error `*RoleNotAssignableError{Label, Roles}` (`errors.Is` → `ErrRoleNotAssignable`) → handler 422 `role_not_assignable`, pesan `Layar "<label>" hanya untuk role: …`. Respons + audit `after` = set yang benar-benar disimpan. Integration test lama `evaluator_integration_test.go` / `service_integration_test.go` dipindah dari key `dashboard` (kini Always_On, dibuang editor) ke `monitoring.audit-logs`. Catatan untuk 7.1: `catalogEntryToResponse` di handler belum mengirim `assignable_roles`. Cek: `go test ./internal/rolemgmt/ ./internal/handler/ ./cmd/...` hijau; `go test -tags integration ./internal/rolemgmt/` (DB dev) hijau; `-tags integration -run RoleMgmt ./internal/handler/` hijau; golangci-lint 0 issues.
- **1.3 (2026-11-24)**: `migrate status` sebelum: 44 applied, 1 pending (`046_permission_catalog_coverage.sql`). OK user di sesi ini → `go run ./cmd/migrate up` (DATABASE_URL localhost): `applying 046_permission_catalog_coverage.sql` → `1 applied`, exit 0, tanpa error (guard + post-check 21 key di dalam 046 lolos). `status` sesudah: 45 applied, 0 pending, checksum ok.
- **6.3 (2026-11-24)**: semua `routes/**` memakai `requirePermission(<key>)`: monitoring.{audit-logs,dsr-late-report,eod-monitoring,atm-portal}, forecasting.{dmaa-forecast,holidays}, replenish.{forecast-browser,vendor-requests,schedule}, cit.vault-plan, cash-count.{parameters,pic-mapping}, settings.{users (+`/users/new`),vendors (4 route),atms,regions,pks,pks-types,master-data-io,acm-areas,roles,rbac-*}; `/exports` = `exports`, `/exports/$dataset` = `requirePermission("exports", { roles: ["ADMIN","ADMIN_PARAM"] })`. Guard baru: `/approvals`, `/cash-flow`, `/atm-portal*`, `/replenishment`, dan halaman induk `/cash-count`, `/forecasting`, `/invoice` (key menu). `/replenishment/vendor-requests/new`: guard izin dulu, lalu redirect maker-role Req 12.10 tetap. `/settings/admin/exports` (redirect) tanpa guard. **`requireRoles` TIDAK dihapus**: ±12 test route lama (audit-logs, atms, regions, … `.test.ts`) memanggilnya langsung sebagai fungsi generik dengan daftar role lokal; menghapusnya = mengubah test-test itu, yaitu cakupan 6.4. Hapus bersama pembaruan 6.4. Cabang `context.forbidden` di komponen route kini mati (tidak berbahaya), ikut dibersihkan di 6.4.
- **Perbaikan test merah wave 0–2 (disetujui user "OK, Wave 3 + perbaiki", app tidak diubah)**: (1) `routes/_protected.property.test.ts` ditulis ulang ke `requirePermission` (Property 3/4: tanpa user → redirect, tanpa key → `ForbiddenError` termasuk ADMIN, ada key → lolos, any-of). (2) `SettingsHubPage.roleManagementCard.test.tsx`: visibilitas lewat `usePermissionStore` (`settings.roles`). (3) `_auth/login.test.tsx`: hitung hanya POST `/auth/login` (login juga memanggil `/auth/me/permissions`). Tambahan karena guard route berubah: `settings.test.ts` (rbac-*), `exports.test.ts`, `exports.preview.test.tsx` dipindah ke izin; helper test me-no-op `load` karena login memicu auto-load store yang menimpa key uji. Hasil `pnpm test`: 215 file / 2021 test hijau; `tsc --noEmit` bersih. `pnpm lint`: **merah** hanya di file milik task lain (`lib/auth/permissionStore.ts` organizeImports, `lib/config/navigation.ts:407-409` useExhaustiveDependencies + suppression tak terpakai) — bukan file 6.3. Cek browser: outstanding (user).
- **4.3 (2026-11-24)**: konstruktor `WithPermissions(e *rolemgmt.PermissionEvaluator)` di `VendorRequestHandler`, `AtmVisitQuotaHandler`, `DsrLateHandler`, `ReplenishmentHandler` (`Routes` saja), `AdminHolidayHandler`; evaluator `nil` = guard role lama persis (build hijau sampai 4.2 memasangnya di `main.go`). Helper di `vendor_request_handler.go`: `viewGuard` (izin **menggantikan** guard role viewer) dan `actionGuard` (izin **di depan** guard role aksi yang tetap, D2/D4). Vendor Request: viewer (`/forecast*`, `/vendors`, `GET /`, `GET /{id}`, `/vault-assignments`, `/vendor-parties`) any-of `replenish.vendor-requests`/`replenish.forecast-browser`; aksi + `/audit-log` `replenish.vendor-requests` + role maker/checker/cancel/audit. Kuota kunjungan: GET `replenish.vendor-requests` (ganti), reset/cancel `replenish.vendor-requests` + checker. DSR telat `monitoring.dsr-late-report`, Replenishment `Routes` `replenish.schedule` (ganti; service tetap cek role). Hari libur `forecasting.holidays` + ADMIN. `admin_vendor_handler.go` / `admin_pks_handler.go` **tidak diubah**: guard tulis sudah di handler, izin baca ada di mount `main.go` (4.2). Test: `permission_guard_test.go` (view/action × injeksi/nil, hari libur). `go test ./internal/handler/ ./internal/rolemgmt/ ./cmd/...` hijau; golangci-lint `./internal/handler/...` 0 issues.
- **4.2 (2026-11-24)**: `buildRouter` membuat satu `permEval := rolemgmt.NewPermissionEvaluator(roleMgmtRepo)` + helper `perm(keys...)` dan `authed`. **Diganti** (RequireRoles viewer dihapus): `atm-portal` → `monitoring.atm-portal` (diperketat, DQ3), `dmaa-forecast` → `forecasting.dmaa-forecast`, `audit-logs` → `monitoring.audit-logs`, `vault-plans` → `cit.vault-plan` (service tetap cek anggota area/role), `vendorRead` (`/admin/vendors`) → any-of `settings.vendors`/`settings.pks`, `/admin/pks` → `settings.pks` saja (tidak lagi lewat `vendorRead`); `reports.PksViewerRoles` tetap dicek di service. **Ditambahkan di depan guard role tetap**: `masterDataAdmin(key)` kini fungsi per key — `settings.atms` (`/admin/atms`, `/{atmID}/assignments`, `assignment-package-options`), `settings.vendors` (semua sub-resource `/admin/vendors/{vendorID}/…`), `settings.master-data-io` (`/changes`, `/import`), `settings.regions`, `settings.pks-types`; cash count `cashCount(key, roles...)` — `cash-count.parameters` (category-params CC-CWM+ADMIN, anomaly-categories/checklist-items CC-CCR+ADMIN), `cash-count.pic-mapping` (areas/pics CC-CWM+ADMIN, import CC-CWM); `acm-areas` `settings.acm-areas` + ADMIN; `/admin/approval/*` per route `settings.rbac-{users,delegations,leaves,policies}` + ADMIN/ADMIN_PARAM/APPACCESS. **Handler guard 4.3** dipasang via `.WithPermissions(permEval)`: VendorRequest, AtmVisitQuota, DsrLate, Replenishment (`Routes`), AdminHoliday — mount-nya tetap RequireAuth saja (tanpa guard ganda). Tidak diubah: `/admin/users`, `/admin/roles`, `/approvals`, `/notifications`, `/auth` (termasuk `WithPermissions(roleMgmtRepo)` dari 3.1), `/dsr`, `/vendor/*`. Urutan RequirePermission → RequireRoles tidak teramati dari luar (body 403 identik), hanya menambah satu query replica untuk role yang ditolak role guard. Cek: `go build ./...`, `go vet ./cmd/api`, `go test ./...` hijau; `go test -count=1 -tags integration ./internal/handler ./cmd/api ./internal/rolemgmt` (DB dev) hijau, 0 skip; `golangci-lint run --build-tags integration ./...` 0 issues. Cakupan per mount dibuktikan nanti oleh 4.4 (route-walk) + 4.5 (RBAC denial).
- **4.5 (2026-11-24)**: `backend/cmd/api/route_rbac_integration_test.go` (`//go:build integration`, skip tanpa `DATABASE_URL`; host.docker.internal → localhost di test). Memakai `buildRouter` asli di atas DB dev + miniredis, token JWT per role (secret sama dengan config), hanya GET, **tanpa tulis ke DB** (tidak ada grant/user dibuat). `TestRBACDenialPerMountRow`: 33 probe × semua role di DB; 403 tepat bila role non-internal, tanpa key, atau di luar guard tetap; kunci diambil dari `routePolicies` (4.4), grant dibaca langsung dari `role_permissions`; ada cek matriks tidak kosong. `TestRBACRoleManagementStaysStatic`: `/admin/roles` + `/admin/users` hanya APPACCESS/ADMIN. Hasil: kedua test PASS (bukan skip), `go test -count=1 -tags integration ./...` hijau semua paket, `golangci-lint run --build-tags integration ./...` 0 issues. Tidak ada baris yang salah tolak/izin.

## Task Dependency Graph

- **Wave 0:** 1.1 migrasi, 2.1 konstanta Go, 5.1 key + store frontend (file terpisah).
- **Wave 1:** 1.2 rollback (needs 1.1), 2.2 query + sqlc (needs 1.1), 4.1 `buildRouter`, 5.2 guard route (needs 5.1).
- **Wave 2:** 1.3 terapkan ke dev (needs 1.1, OK user), 2.3 middleware (needs 2.1, 2.2), 5.3 test guard (needs 5.2), 6.1 nav (needs 5.1), 6.2 hub (needs 5.1).
- **Wave 3:** 1.4 test migrasi (needs 1.3), 2.4 PUT izin (needs 2.2; tidak bersamaan dengan 2.3 karena `repository.go`), 6.3 route (needs 5.2).
- **Wave 4:** 2.5 test rolemgmt (needs 2.3, 2.4), 3.1 izin saya (needs 2.2, 2.3), 4.3 handler (needs 2.3), 7.1 editor (needs 5.1, 2.4).
- **Wave 4b:** 4.2 `main.go` (needs 4.1, 2.3, 3.1, 4.3 — dipindah 2026-11-24, user: 3.1 mendaftarkan route di `main.go` dan 4.2 memasang konstruktor `With…` dari 4.3).
- **Wave 5:** 3.2 test izin saya (needs 3.1), 4.4 route-walk (needs 4.2, 4.3), 6.4 test nav + cakupan (needs 6.1–6.3, 1.4), 7.2 test editor (needs 7.1).
- **Wave 6:** 4.5 RBAC denial (needs 4.2, 4.3, 1.3).
- **Wave 7:** 8 gate kualitas.
- **Wave 8:** 9 dokumentasi.
- **Wave 9:** 10 cek browser (user).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1", "5.1"] },
    { "id": 1, "tasks": ["1.2", "2.2", "4.1", "5.2"] },
    { "id": 2, "tasks": ["1.3", "2.3", "5.3", "6.1", "6.2"] },
    { "id": 3, "tasks": ["1.4", "2.4", "6.3"] },
    { "id": 4, "tasks": ["2.5", "3.1", "4.3", "7.1"] },
    { "id": "4b", "tasks": ["4.2"] },
    { "id": 5, "tasks": ["3.2", "4.4", "6.4", "7.2"] },
    { "id": 6, "tasks": ["4.5"] },
    { "id": 7, "tasks": ["8"] },
    { "id": 8, "tasks": ["9"] },
    { "id": 9, "tasks": ["10"] }
  ]
}
```
