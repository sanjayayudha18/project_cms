# Design Document

> Status: accepted 2026-11-20 (engineer) — DQ1–DQ3 diterima sesuai rekomendasi ("Terima semua").

## Overview

Spec ini melanjutkan `role-management` (`.kiro/specs/.done/role-management/`). Komponen dasarnya sudah ada: katalog `menu_features`, pemetaan `role_permissions`, editor izin, dan middleware `rolemgmt.RequirePermission`. Yang belum ada adalah katalog yang lengkap dan pemakaian izin itu. Desain ini menambah:

1. **Migrasi `046_permission_catalog_coverage.sql`**: 2 menu + 19 fitur baru, grant awal sesuai matriks D9, dan satu kolom baru `menu_features.assignable_roles` (lihat DQ1).
2. **Backend ATM**: `RequirePermission` dapat menerima beberapa key (any-of) dan Always_On_Key. Middleware ini dipasang di setiap mount layar. Ditambah endpoint `GET /api/v1/auth/me/permissions` dan filter key terkunci di `PUT /api/v1/admin/roles/{id}/permissions`.
3. **CompanyPortal**: store izin, `NAV_CONFIG[].permission` menggantikan `roles`, kartu hub memakai key, `requirePermission(key)` menggantikan `requireRoles` dan benar-benar menampilkan 403, serta editor menandai key selalu aktif/terkunci.
4. **Test pengaman** (Req 9): test cakupan route ↔ katalog di frontend, test matriks migrasi, dan test 403 per mount.

Aturan inti: **izin layar menentukan siapa boleh *membuka* layar dan membaca datanya. Aturan role yang sudah ada untuk aksi (maker/checker/approve) dan untuk penulisan data master tetap berlaku (D4).**

## Architecture

```mermaid
flowchart LR
  subgraph CompanyPortal
    LOGIN[login / app load] --> PS[permissionStore\nGET /auth/me/permissions]
    PS --> NAV[Sidebar + CommandPalette\nNAV_CONFIG.permission]
    PS --> HUB[SettingsHubPage\ncard.permission]
    PS --> RG[requirePermission(key)\nbeforeLoad → Forbidden 403]
    ED[PermissionEditor] -->|PUT /admin/roles/:id/permissions| API
    ED -->|role sendiri berubah| PS
  end
  subgraph backend ATM
    API[RoleMgmtHandler] -->|strip Always_On/Locked, tx + audit| PRI[(primary)]
    ME[AuthHandler.MyPermissions] --> PRI
    MW[RequirePermission any-of] --> EV[PermissionEvaluator] --> REP[(replica)]
    MW --> H[handler mount layar] --> SVC[service: cek role aksi tetap]
  end
```

Alur baca izin:
- **Middleware** membaca replica per request, sama seperti desain role-management (tanpa cache). Jeda replica beberapa milidetik bisa terjadi; ini sudah diterima di spec role-management.
- **`/auth/me/permissions`** membaca primary. Endpoint ini dipanggil sekali per muat aplikasi dan sesudah editor menyimpan izin role sendiri (Req 8.3), jadi harus membaca data terbaru.

## Components and Interfaces

### Backend ATM

#### `rolemgmt` — evaluator dan middleware

- `AlwaysOnKeys = []string{"dashboard", "approvals"}` dan `LockedKeys = []string{"settings.roles", "settings.roles.create", "settings.roles.edit", "settings.users"}` didefinisikan di `rolemgmt/keys.go`. Key yang sama ditulis ulang di frontend `lib/auth/permissionKeys.ts`; test di kedua sisi membandingkannya dengan seed migrasi.
- `RequirePermission(e, keys ...string)` mengizinkan request bila salah satu key dimiliki role. Kalau salah satu key adalah Always_On_Key dan role-nya internal, request langsung diizinkan. Signature lama yang menerima satu key tetap jalan karena parameternya variadic.
- `Repository.HasAnyPermission(ctx, role, keys []string)` adalah satu query replica `EXISTS … key = ANY($2)` dengan sqlc baru `HasAnyRolePermission`.
- Kalau query error, middleware membalas 500 `internal_error` (fail closed, perilaku yang sudah ada) dan mencatat `slog.Error` yang berisi role dan key, tanpa token.
- `IsInternalRole(role)`: role selain `VENDOR-*` dan `EXT-CLIENT`.

#### `GET /api/v1/auth/me/permissions` (baru)

- Dipasang di `AuthHandler.Routes()` di belakang `RequireAuth`.
- Response `200 {"permissions": ["dashboard", "approvals", "monitoring.audit-logs", …]}`. Isinya Effective_Permissions (Glossary), urut menurut `sort_order`.
- Role `VENDOR-*` dan `EXT-CLIENT` mendapat `{"permissions": []}` (Req 4.3).
- Query sqlc baru `ListRoleFeatureKeys(role)` di primary. Always_On_Key dan Locked_Key ditambahkan di Go.
- Ditambahkan ke `backend/api/openapi.yaml` hanya bila modul `auth` sudah ada di spec. Saat ini tidak, jadi cukup test handler biasa.

#### `PUT /api/v1/admin/roles/{id}/permissions` (diubah)

- Service `UpdateRolePermissions` membuang ID katalog milik Always_On_Key dan Locked_Key dari set yang dikirim **dan** dari set yang sudah tersimpan sebelum menghitung diff. Dengan begitu grant lama di 002 (mis. `settings.roles` untuk ADMIN) tidak terhapus diam-diam (Req 3.5).
- **DQ1, bila diterima:** grant ke role di luar `assignable_roles` sebuah key ditolak dengan 422 `role_not_assignable` dan pesan berisi label layar.
- Audit dalam transaksi tetap seperti sekarang (Req 8.4).

#### Pemetaan mount → Screen_Key

Kolom "Guard role tetap" berisi guard yang **tetap ada** di samping `RequirePermission`. Ada dua jenis:
- **Aksi**: aturan bisnis per route (D4).
- **Data master**: CLAUDE.md §12 D1–D4 mewajibkan `ADMIN`/`ADMIN_PARAM` di route dan di `Submit`, dan deviasi GR#3 mewajibkan role dicek ulang di route.

Untuk mount yang punya guard tetap, katalog hanya bisa **mencabut** akses, tidak memperluasnya. DQ1 mencegah editor menawarkan perluasan seperti itu.

| Mount (`backend/cmd/api/main.go`) | Screen_Key | Guard role sekarang | Guard role tetap |
|---|---|---|---|
| `/api/v1/dmaa-forecast` | `forecasting.dmaa-forecast` | ATM-USER, ATM-SPV, BRANCH-ATM-*, ADMIN, ADMIN_PARAM | — (diganti) |
| `/api/v1/atm-portal` | `monitoring.atm-portal` | hanya RequireAuth | — (**diperketat**, lihat DQ3) |
| `/api/v1/audit-logs` | `monitoring.audit-logs` | ADMIN, ADMIN_PARAM | — |
| `/api/v1/dsr-late-reports` | `monitoring.dsr-late-report` | `reports.DsrLateViewerRoles` (handler `r.Use`) | — |
| `/api/v1/replenishment` (`Routes`, bukan `RealizationRoutes`) | `replenish.schedule` | `reports.ReplenishmentViewerRoles` | — |
| `/api/v1/vendor-requests` route viewer (`/forecast*`, `/vendors`, `GET /`, `GET /{id}`, `/vault-assignments`, `/vendor-parties`) | any-of `replenish.vendor-requests`, `replenish.forecast-browser` | `vendorRequestViewerRoles` | — |
| `/api/v1/vendor-requests` route aksi (create/submit/approve/…/audit-log) | `replenish.vendor-requests` | maker/checker/cancel/audit roles | **aksi** |
| `/api/v1/atm-visit-quotas` GET | `replenish.vendor-requests` | `vendorRequestViewerRoles` | — |
| `/api/v1/atm-visit-quotas` reset/cancel | `replenish.vendor-requests` | `vendorRequestCheckerRoles` | **aksi** |
| `/api/v1/vault-plans` | `cit.vault-plan` | ACM-USER, ACM-SPV, ADMIN (+ service cek anggota area) | — (service tetap) |
| `/api/v1/admin/vendors` (vendorRead) | any-of `settings.vendors`, `settings.pks` | `reports.PksViewerRoles` | tulis: ADMIN, ADMIN_PARAM (**aksi**) |
| `/api/v1/admin/pks` | `settings.pks` | `reports.PksViewerRoles` | reminder: ADMIN, ADMIN_PARAM (**aksi**) |
| `masterDataAdmin`: `/admin/atms`, `/admin/atms/{id}/assignments` | `settings.atms` | ADMIN, ADMIN_PARAM | **data master** |
| `masterDataAdmin`: `/admin/vendors/{id}/{branches,vaults,pics,pks,packages,package-prices,dsr-location-maps}`, `/branches/{id}/atms` | `settings.vendors` | ADMIN, ADMIN_PARAM | **data master** |
| `masterDataAdmin`: `/admin/regions` | `settings.regions` | ADMIN, ADMIN_PARAM | **data master** |
| `masterDataAdmin`: `/admin/pks-types` | `settings.pks-types` | ADMIN, ADMIN_PARAM | **data master** |
| `masterDataAdmin`: `/admin/master-data/import`, `/changes` | `settings.master-data-io` | ADMIN, ADMIN_PARAM | **data master** |
| `/api/v1/cash-count/{category-params,anomaly-categories,checklist-items}` | `cash-count.parameters` | CC-CWM/CC-CCR + ADMIN | **data master** |
| `/api/v1/cash-count/{areas,pics,import}` | `cash-count.pic-mapping` | CC-CWM (+ ADMIN) | **data master** |
| `/api/v1/admin/acm-areas` | `settings.acm-areas` | ADMIN | **deviasi GR#3** |
| `/api/v1/admin/holidays` | `forecasting.holidays` | ADMIN (handler) | **deviasi GR#3** |
| `/api/v1/admin/approval/*` (RBAC) | per route: `settings.rbac-users` (`/users/*hierarchy`), `settings.rbac-policies`, `settings.rbac-delegations`, `settings.rbac-leaves` | ADMIN, ADMIN_PARAM, APPACCESS | **auth** (konfigurasi approval) |
| `/api/v1/admin/users`, `/api/v1/admin/roles` | Locked_Key | APPACCESS, ADMIN | statis (D8), **tanpa** RequirePermission |
| `/api/v1/approvals`, `/api/v1/notifications`, `/api/v1/auth/*`, `/api/v1/dsr` | — | RequireAuth / per route | tidak diubah (always-on, infrastruktur, atau dipakai VendorPortal) |
| `/api/v1/vendor/*` | — | `vendorrole.All` | tidak diubah (D6) |

Catatan:
- `/api/v1/cash-flow/summary` yang dipanggil `features/cash-flow` **belum ada** di backend, jadi `cash-flow` hanya dijaga di frontend sampai API-nya dibangun.
- EOD Monitoring memanggil Python `eod_retry_scheduler` (`RETRY_AUTH_MODE=jwt`, ADMIN/APPACCESS). Python di luar scope (sama dengan D10), jadi `monitoring.eod-monitoring` hanya dijaga di frontend dan di gate Python yang sudah ada.
- `exports` dijaga di frontend, sedangkan API-nya dijaga `backend-exq` (D10).

### CompanyPortal

#### `lib/auth/permissionStore.ts` (baru)

- Store Zustand dengan bentuk `{ keys: Set<string>, status: "idle" | "loading" | "ready" | "error", load(), has(key), hasAny(keys) }`, mengikuti pola `useAuthStore` yang sudah ada.
- `load()` dipanggil sesudah login, saat sesi dipulihkan, dan dari `PermissionEditor` sesudah menyimpan izin role milik user sendiri.
- Bila `error`: `has()` hanya `true` untuk Always_On_Key (Req 5.5).

#### `lib/config/navigation.ts`

- `NavItem.roles` diganti `permission: ScreenKey`. `filterNavByRoles` diganti `filterNavByPermissions(items, has)` dan bypass ADMIN/ADMIN_PARAM dihapus (Req 5.3).
- Selama `status !== "ready"`, sidebar hanya menampilkan item Always_On_Key (Req 5.4).
- `ScreenKey` adalah union literal di `permissionKeys.ts`, jadi salah ketik gagal di typecheck.

#### `SettingsHubPage`

- `HubCard.permission: ScreenKey`. `canSeeCard` dan `ATM_SPV_CARD_IDS` dihapus. Kartu `add-user` memakai `settings.users`, sedangkan Locked_Key dicek dengan role APPACCESS/ADMIN.
- Kategori atau grup kosong disembunyikan seperti sekarang.
- Route `/settings` terlihat bila user punya setidaknya satu key `settings.*`, `forecasting.holidays`, atau `exports` yang punya kartu di hub.

#### `routes/_protected.tsx`

- `requirePermission(key: ScreenKey | ScreenKey[], opts?: { roles?: DbRole[] })` menggantikan `requireRoles`:
  - belum login → redirect `/login`;
  - izin masih dimuat → menunggu (pola `isAuthLoading` yang sudah ada);
  - tidak punya key, atau `opts.roles` diberikan dan role tidak termasuk → `throw` error `ForbiddenError`.
- `ForbiddenError` ditangkap oleh `errorComponent` di `protectedRoute`, yang me-render halaman 403. Ini memperbaiki temuan `{forbidden:true}` yang tidak pernah dibaca (Req 6.1).
- `opts.roles` dipakai hanya untuk sub-halaman yang lebih sempit (Req 6.3): `/exports/$dataset` (ADMIN, ADMIN_PARAM).
- Locked route (`/settings/roles`, `/settings/admin/users`, `/users/new`) memakai `requirePermission("settings.roles" | "settings.users")`. Effective_Permissions hanya memuat key ini untuk APPACCESS/ADMIN, jadi hasilnya sama dengan guard statis.

#### `PermissionEditor`

- Key yang ada di `AlwaysOnKeys` ditampilkan tercentang, `disabled`, dengan badge "Selalu aktif".
- Key yang ada di `LockedKeys` tidak bisa dicentang dan berbadge "Hanya APPACCESS/ADMIN".
- DQ1: role di luar `assignable_roles` membuat kotak centang `disabled` dengan tooltip "Layar ini hanya untuk role: …".
- Payload PUT tidak pernah berisi key selalu aktif/terkunci. Backend tetap memfilternya sebagai pengaman kedua.

## Data Models

### Migrasi `046_permission_catalog_coverage.sql` (satu tx, additive, `ON CONFLICT DO NOTHING`)

1. **Kolom baru (DQ1)**: `menu_features.assignable_roles text[] NULL`. `NULL` berarti semua role internal boleh diberi. Diisi untuk key yang punya guard tetap (tabel mount di atas), mis. `settings.atms` → `{ADMIN,ADMIN_PARAM}`. Ini perubahan skema dan perlu persetujuan (CLAUDE.md §3).
2. **Menu baru**: `approvals` "Persetujuan" (`sort_order` 9) dan `exports` "Ekspor" (10).
3. **Fitur baru**: 19 baris dari Requirement 1.1, dengan parent dari `menu_features` lewat `SELECT id … WHERE key = '<parent>'`. Bila parent tidak ada, migrasi gagal keras (pola `027`/`041`).
4. **Grant awal**: `role_permissions`, `granted_by` = ADMIN pertama (pola `027`). Bila tidak ada user ADMIN, migrasi gagal keras.
5. **Rollback** didokumentasikan di header (forward-only): hapus `role_permissions` lalu `menu_features` untuk key-key baru, lalu `DROP COLUMN assignable_roles`. Ditambah `backend/scripts/dev/rollback_046_permission_catalog_coverage.sql`.

### Matriks grant awal (D9)

Aturan pemilihan:
- Ambil role yang **lolos guard backend** endpoint baca layar itu, karena backend satu-satunya penahan yang benar-benar bekerja (Introduction).
- Bila backend tidak menjaga (tidak ada API, Python, atau hanya RequireAuth), pakai role `NAV_CONFIG` ditambah ADMIN dan ADMIN_PARAM (bypass lama).
- Hanya role internal. Locked_Key dan Always_On_Key tidak diberi grant.
- Parent menu ikut diberi grant bila role mendapat setidaknya satu anaknya, supaya tampilan editor konsisten. Penegakan hanya memakai key layar.

| Screen_Key | ADMIN | ADMIN_PARAM | APPACCESS | ATM-USER | ATM-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | lain |
|---|---|---|---|---|---|---|---|---|
| `cash-flow` | ✓ | ✓ | | ✓ | ✓ | | | |
| `exports` | ✓ | | | ✓ | ✓ | ✓ | ✓ | |
| `replenish.forecast-browser` | ✓ | | | ✓ | ✓ | ✓ | ✓ | |
| `replenish.vendor-requests` | ✓ | | | ✓ | ✓ | ✓ | ✓ | |
| `replenish.schedule` | ✓ | | | ✓ | ✓ | ✓ | ✓ | |
| `monitoring.atm-portal` | ✓ | ✓ | | ✓ | ✓ | | | |
| `monitoring.dsr-late-report` | ✓ | | | ✓ | ✓ | | ✓ | |
| `monitoring.eod-monitoring` | ✓ | | ✓ | | | | | |
| `monitoring.audit-logs` | ✓ | ✓ | | | | | | |
| `forecasting.dmaa-forecast` | ✓ | ✓ | | ✓ | ✓ | ✓ | ✓ | |
| `forecasting.holidays` | ✓ | | | | | | | |
| `settings.vendors` | ✓ | ✓ | | | ✓ | | | |
| `settings.atms`, `.regions`, `.pks-types`, `.master-data-io` | ✓ | ✓ | | | | | | |
| `settings.rbac-users`, `-policies`, `-delegations`, `-leaves` | ✓ | ✓ | ✓ | | | | | |
| *(sudah ada, tidak diubah)* `settings.pks`, `settings.acm-areas`, `cash-count.*`, `cit.vault-plan` | sesuai 027/041/044 | | | | | | | CC-*, ACM-* |

### Perubahan yang terlihat user (perlu disetujui, DQ2)

Menurut aturan di atas, **akses** API tidak berubah, tetapi **menu yang tampil** berubah untuk beberapa role:
- **ADMIN_PARAM kehilangan menu** yang sekarang tampil tetapi API-nya menolak: Rekomendasi CIT, Request CIT, Jadwal CIT, Laporan DSR Telat, EOD Monitoring, Kalender Libur, Ekspor.
- **BRANCH-ATM-USER/SPV mendapat menu** Jadwal CIT dan Ekspor. API keduanya sudah mengizinkan mereka; selama ini hanya menunya yang tidak ada.
- **BRANCH-ATM-SPV mendapat menu** Laporan DSR Telat (API sudah mengizinkan).
- **APPACCESS mendapat menu** EOD Monitoring (gate Python sudah mengizinkan).
- **ATM Portal diperketat** (DQ3): sekarang API-nya terbuka untuk semua user yang login, termasuk vendor. Sesudah rilis hanya role di matriks yang bisa membukanya.

## Correctness Properties

### Property 1: Katalog menutup semua route
*For any* route terlindungi di `routeTree` CompanyPortal dan *for any* item `NAV_CONFIG` yang tidak `disabled`, key yang disebutnya ada di daftar key katalog hasil migrasi 001–046.
**Validates: Requirements 1.1, 1.4, 9.1**

### Property 2: Grant awal sama dengan matriks
*For any* role seed dan *for any* Screen_Key baru, baris `role_permissions` ada sesudah migrasi 046 jika dan hanya jika sel matriks D9 bertanda ✓.
**Validates: Requirements 2.1, 2.2, 9.3**

### Property 3: Migrasi idempoten dan additive
*For any* DB yang sudah menjalankan 046, menjalankan isi 046 sekali lagi tidak mengubah jumlah baris `menu_features` maupun `role_permissions`, dan tidak ada baris yang ada sebelum 046 yang hilang atau berubah.
**Validates: Requirements 2.3, 2.5**

### Property 4: Keputusan middleware = Effective_Permissions
*For any* role internal R dan *for any* daftar key K pada sebuah mount, `RequirePermission(K)` mengizinkan R jika dan hanya jika K ∩ (grant R ∪ Always_On_Key) tidak kosong. Untuk role non-internal hasilnya selalu ditolak, kecuali mount yang tidak memakai RequirePermission.
**Validates: Requirements 3.1, 7.1, 7.2, 7.3**

### Property 5: Fail closed
*For any* error dari repository izin, middleware membalas 500 dan handler tidak pernah dipanggil.
**Validates: Requirement 7.4**

### Property 6: Key selalu aktif/terkunci kebal editor
*For any* payload PUT izin role, set grant tersimpan untuk Always_On_Key dan Locked_Key sesudah PUT sama dengan sebelum PUT.
**Validates: Requirements 3.4, 3.5**

### Property 7: Guard tetap tidak terlewati
*For any* mount bertanda aksi, data master, deviasi GR#3, atau auth, dan *for any* role yang punya Screen_Key tetapi tidak termasuk role guard tetap, request ditolak 403.
**Validates: Requirements 7.5, 3.3**

### Property 8: Sidebar = izin
*For any* himpunan izin S yang dimuat, item yang dirender sidebar, command palette, dan hub sama dengan item yang key-nya ada di S (atau hanya Always_On_Key bila status `error`/`loading`).
**Validates: Requirements 5.1–5.5**

### Property 9: Route guard menolak tanpa izin
*For any* route dan *for any* himpunan izin yang tidak memuat key route itu, navigasi me-render halaman 403 dan komponen halaman tidak di-mount.
**Validates: Requirements 6.1, 6.2, 6.3, 6.5**

### Property 10: Izin saya
*For any* user internal, `GET /auth/me/permissions` = grant role (primary) ∪ Always_On_Key ∪ (Locked_Key bila APPACCESS/ADMIN). Untuk role non-internal hasilnya `[]`.
**Validates: Requirements 4.1–4.4, 8.3**

## Error Handling

| Kondisi | Backend | Frontend |
|---|---|---|
| Tidak punya key | 403 `{"error":"forbidden","message":"Anda tidak memiliki akses ke resource ini"}` (bentuk yang sudah ada) | Halaman 403 "Anda tidak memiliki akses ke halaman ini" + tautan ke Dashboard |
| Repo izin error | 500 `internal_error`, `slog.Error` (role, keys; tanpa token) | Pesan error umum yang sudah ada |
| `/auth/me/permissions` gagal | — | Sidebar hanya Dashboard + Persetujuan, banner "Menu gagal dimuat" + tombol "Coba lagi" (Req 5.5) |
| PUT berisi key terkunci | Key diabaikan, 200 | Editor tidak pernah mengirimnya |
| PUT grant di luar `assignable_roles` (DQ1) | 422 `role_not_assignable` | Kotak centang `disabled` + tooltip |
| Migrasi tanpa ADMIN / parent hilang | Gagal keras, rollback tx | — |

## Testing Strategy

**Go (backend, `go test ./...`, integration di-skip bila `DATABASE_URL` kosong):**
- `rolemgmt`: table-driven `RequirePermission` (any-of, Always_On_Key, role non-internal, repo error → 500 dan handler tidak dipanggil), untuk Property 4 dan 5.
- `rolemgmt` service: PUT dengan Always_On_Key/Locked_Key dan dengan grant lama `settings.roles` → set tersimpan tidak berubah (Property 6). Bila DQ1 diterima, 422.
- `handler`: `GET /auth/me/permissions` untuk ADMIN, APPACCESS, ATM-USER, VENDOR-USER, dan tanpa token (Property 10).
- `handler` RBAC denial (integration): untuk setiap baris tabel mount, role tanpa key → 403, role dengan key → bukan 403. Untuk mount dengan guard tetap, role dengan key tetapi di luar role guard → 403 (Property 7).
- Test pemetaan route: walk `chi.Walk` atas router dari `main.go` (dibangun lewat fungsi `buildRouter` yang diekstrak), dan setiap route di prefix yang sudah dipindah harus punya key (Req 9.2).
- Migrasi (integration, pola `vendor_pks_list_migration_integration_test.go`): matriks role × key = tabel D9 (Property 2), dan menjalankan isi 046 dua kali tidak menambah baris (Property 3).

**Vitest (CompanyPortal):**
- `navigation.property.test.ts` diperbarui: `filterNavByPermissions` dengan himpunan acak (Property 8).
- Test cakupan baru: setiap route di `routeTree` yang memakai `requirePermission` punya key di `SCREEN_KEYS`, setiap item `NAV_CONFIG` aktif punya key, dan `SCREEN_KEYS` sama dengan daftar key di migrasi (diekspor ke fixture JSON oleh test Go) (Property 1).
- `_protected` guard: tanpa key → 403 dan komponen tidak di-render, `opts.roles` sempit, loading, error (Property 9).
- `SettingsHubPage`, `PermissionEditor` (badge, disabled, payload tanpa key terkunci), `permissionStore` (load/error/reload sesudah simpan).

**Lainnya:** `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build`, `golangci-lint`, `/security-review` (tahap 5, wajib karena auth). Cek browser manual = **outstanding (user)**.

## Open Design Questions (✅ DIPUTUSKAN 2026-11-20, "Terima semua" = rekomendasi)

- **DQ1 — Kolom `assignable_roles`.** Tanpa kolom ini, editor bisa memberi `settings.atms` ke ATM-USER: menunya tampil tetapi API menolak karena guard data master tetap ADMIN/ADMIN_PARAM. **Rekomendasi:** tambah kolom `menu_features.assignable_roles text[]` (NULL = bebas) dan AC baru **3.6** "IF grant di luar assignable_roles, THEN Role_Permission_API SHALL menolak 422". Karena ini mengubah requirements yang sudah diterima, perlu persetujuan PO. Alternatif: tanpa kolom, editor hanya menampilkan peringatan.
- **DQ2 — Perubahan menu yang terlihat** (daftar di Data Models). **Rekomendasi:** terima. Menu jadi sesuai dengan API yang benar-benar bisa dipakai.
- **DQ3 — ATM Portal diperketat.** API `/api/v1/atm-portal` sekarang terbuka untuk semua token, termasuk vendor. **Rekomendasi:** terima pengetatan ini sebagai perbaikan keamanan, dengan grant ADMIN, ADMIN_PARAM, ATM-USER, ATM-SPV.

## Files touched (perkiraan)

- `backend/migrations/046_permission_catalog_coverage.sql`, `backend/scripts/dev/rollback_046_permission_catalog_coverage.sql`
- `backend/queries/role_mgmt.sql` (+ `internal/db/role_mgmt.sql.go` regen, ingat fix `UserLeafe`)
- `backend/internal/rolemgmt/{keys.go,evaluator.go,repository.go,service.go}` + test
- `backend/internal/handler/{auth_handler.go,vendor_request_handler.go,atm_visit_quota_handler.go,dsr_late_report_handler.go,replenishment_handler.go,admin_holiday_handler.go,admin_vendor_handler.go,admin_pks_handler.go}` + test
- `backend/cmd/api/main.go` (mount + ekstrak `buildRouter` untuk test route walk)
- `frontend/CompanyPortal-Vite/src/lib/auth/{permissionStore.ts,permissionKeys.ts}`, `lib/config/navigation.ts`, `routes/_protected.tsx`, semua route yang memakai `requireRoles` (±30 file), `features/rbac-settings/components/SettingsHubPage.tsx`, `features/role-management/components/PermissionEditor.tsx`, `components/layout/{Sidebar,CommandPalette}.tsx` + test
- `.claude/CLAUDE.md` §3/§12 dan `docs/data-map.md` (sesudah migrasi diterapkan)
