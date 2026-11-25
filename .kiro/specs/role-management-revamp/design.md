# Design Document

> Status: accepted 2026-11-25 (engineer) — "tidak perlu mockup, lanjut buat task": DQ1–DQ3 = rekomendasi, DQ4 = tanpa mockup (Req 6.6 dikesampingkan user).

## Overview

Spec ini melanjutkan `role-management` dan `permission-catalog-coverage`. Isinya tiga perubahan:

1. **Data**: migrasi `047` memperbaiki teks rusak, menambah portal ke peran dan katalog, menambah cakupan pihak ke peran vendor, dan memberi grant awal Portal_Cabang (Req 1, 2, 3, 7).
2. **Backend ATM**: `vendorrole` tidak lagi memakai daftar nama peran yang ditulis tetap. Peran vendor dikenali dari prefix `VENDOR-`, dan cakupan pihak dibaca dari DB. Middleware izin Portal_Cabang dipasang di depan guard lama, dan `/auth/me/permissions` mengembalikan key vendor (Req 4, 7).
3. **Frontend**: Manajemen Peran memakai dua tab dengan tata letak master-detail (Req 5, 6). VendorPortal membaca izin untuk sidebar dan route (Req 4). Form user CompanyPortal membaca daftar peran dari API (Req 8).

Keputusan kunci (DQ1, butuh persetujuan engineer): **Role_Portal disimpan di kolom dan dikunci ke prefix nama oleh CHECK constraint** (`portal='vendor'` ⇔ `role LIKE 'VENDOR-%'`). Dengan begitu, semua kode yang sudah membedakan peran lewat prefix (`rolemgmt.IsInternalRole`, CompanyPortal `lib/auth/store.ts`, `RoleBadge`) otomatis tetap benar untuk peran baru, tanpa query tambahan per request. Peran baru di tab Portal_Cabang selalu diberi prefix `VENDOR-` oleh dialog dan service.

## Architecture

```
            CompanyPortal /settings/roles?portal=main|vendor&role=<id>
                 │  GET /api/v1/admin/roles        (+ portal, party_scope, active_user_count)
                 │  GET /api/v1/admin/roles/catalog (+ portal)
                 │  POST /api/v1/admin/roles        (+ portal, party_scope)
                 │  PUT  /api/v1/admin/roles/{id}/permissions (cek portal katalog = portal peran)
                 ▼
  backend ATM ── rolemgmt (service + repo) ── roles, menu_features, role_permissions
                 │
  VendorPortal ──┤ GET /api/v1/auth/me/permissions → ["vendor-dsr","vendor-orders",...]
                 │ /api/v1/vendor/replenish-orders*        → RequireVendorPermission("vendor-orders")
                 │   POST /{id}/realization                → RequireVendorPermission("vendor-realizations")
                 │ /api/v1/vendor/replenish-realizations*  → RequireVendorPermission("vendor-realizations")
                 │ /api/v1/dsr*                            → RequireVendorPermission("vendor-dsr")  (internal lolos)
                 ▼
          vendorrole.IsVendor(role)   = prefix "VENDOR-"            (tanpa DB)
          ScopeReader.PartyScope      = roles.party_scope (replica)  (menggantikan PartyRoles/ForParty)
```

Urutan middleware di mount vendor: `RequireAuth` → `RequireVendorPermission(key)` → guard role (`RequireRoles(vendorrole.All...)` diganti `RequireVendorRole()` berbasis prefix) → handler. Katalog hanya bisa mempersempit akses (Req 4.5).

## Components and Interfaces

### Backend ATM

**`internal/vendorrole`**
- `IsVendor(role)` → `strings.HasPrefix(strings.ToUpper(strings.TrimSpace(role)), "VENDOR-")`. Konstanta `User`/`ATMUser`/`VaultUser` tetap ada untuk tes dan backfill.
- `All`, `PartyRoles`, dan `ForParty` dihapus. Penggantinya:
  - Interface `ScopeReader { PartyScope(ctx, role string) ([]string, error) }`, diimplementasikan `rolemgmt.Repository` (replica, `SELECT party_scope FROM roles WHERE role = $1`). Peran tidak dikenal atau scope NULL menghasilkan `nil`, dan pemanggil menolaknya (fail closed).
  - Penerima notifikasi pihak (`service/vendor_party.go:338`) memakai query `ListVendorRolesForParty(party)` (`SELECT role FROM roles WHERE portal='vendor' AND $1 = ANY(party_scope)`) di tx pemanggil, menggantikan `ForParty`.
- Middleware baru `RequireVendorRole()`: 403 bila `!IsVendor(role)`, dengan body sama seperti `RequireRoles`.

**`internal/service/vendor_order.go`**: `vendorrole.PartyRoles(va.Role)` diganti `s.scopes.PartyScope(ctx, va.Role)`. Hasil kosong → 403, sama dengan perilaku sekarang untuk peran non-vendor. `ScopeReader` diterima lewat constructor dan dipasang di `main.go`.

**`internal/service/vendor_request_completion.go`, `internal/auth/user_admin.go`**: tetap memakai `IsVendor`, yang kini berbasis prefix, jadi pemanggil tidak berubah. `vendorFields` ditambah satu aturan: peran non-vendor dengan `vendor_id` terisi ditolak dengan `ReferenceError{Field:"vendor_id"}` (Req 2.5). User internal (LDAP) dengan peran vendor sudah ditolak oleh aturan `auth_source=local`.

**`backend-extapi`**: membuat user LDAP. Task 1 memastikan peran vendor ditolak di sana; bila belum, aturan `IsVendor` → tolak ditambahkan di `pkg/userrules` (Req 2.5).

**`internal/rolemgmt`**
- `RequireVendorPermission(e, key)`: tanpa auth → 401; `!vendorrole.IsVendor(role)` → lolos tanpa query (Req 4.6); peran vendor → `HasAnyPermission(role, [key])` di replica, tidak cocok → 403, error → 500 + `slog.Error` (fail closed). `RequirePermission` yang ada tidak diubah.
- `keys.go`: `VendorAlwaysOnKeys = []string{"vendor-notifications"}`. Key ini tidak dimasukkan ke katalog: frontend selalu menampilkan Notifikasi, dan `/api/v1/notifications` tidak diberi guard katalog (Req 3.2).
- `ListRoles` menambah `portal`, `party_scope`, dan `active_user_count` (`COUNT(users) WHERE is_active AND deleted_at IS NULL`, LEFT JOIN, replica) (Req 6.3).
- `ListCatalog` menambah `portal`.
- `CreateRole(req{Role, Description, Portal, PartyScope})`:
  - `portal` wajib ∈ {`main`,`vendor`}; selain itu 422.
  - `vendor`: nama wajib berprefix `VENDOR-` (dialog mengisinya otomatis, Req 5.3), dan `party_scope` wajib subset tidak kosong dari {`replenish`,`vault`} (Req 7.1).
  - `main`: nama tidak boleh berprefix `VENDOR-`, dan `party_scope` harus kosong (Req 7.3).
  - Audit `role_created` mencatat portal dan scope.
- `UpdateRolePermissions`: `resolvePermissionSet` menolak entri katalog dengan `portal ≠ role.portal` → 422 `role_not_assignable` (Req 2.4). Aturan `assignable_roles` hanya berlaku untuk katalog `main`.
- Tidak ada endpoint untuk mengubah portal atau scope (Req 2.3, 7.3). Body `PUT` yang menyertakan field tersebut ditolak 422 lewat decoder strict.

**`internal/handler/auth_handler.go` `MyPermissions`**: peran vendor → `ListRoleFeatureKeys` (kini memuat key `vendor-*`) + `VendorAlwaysOnKeys`; `EXT-CLIENT` → `[]` (Req 4.1). `/auth/me` menambah `party_scope` (dibaca dari DB per request) untuk VendorPortal (Req 7.6).

**`cmd/api/main.go`**, mount:

| Mount | Sebelum | Sesudah |
|---|---|---|
| `/api/v1/vendor/replenish-orders` | `RequireAuth` + handler `RequireRoles(vendorrole.All)` | `RequireAuth` + `RequireVendorPermission("vendor-orders")` + handler `RequireVendorRole()`; `POST /{id}/realization` diberi tambahan `RequireVendorPermission("vendor-realizations")` di `Routes()` |
| `/api/v1/vendor/replenish-realizations` | sama | `RequireVendorPermission("vendor-realizations")` + `RequireVendorRole()` |
| `/api/v1/dsr` | `RequireAuth` | `RequireAuth` + `RequireVendorPermission("vendor-dsr")` (internal lolos; guard di handler tetap) |
| route vendor di `replenishment_handler.go:180` | `RequireRoles(vendorrole.All)` | `RequireVendorRole()` |

`VendorOrderHandler` menerima fungsi `perm(key)` agar route realization bisa memakai key-nya sendiri.

**OpenAPI**: `/vendor/replenish-orders*` sudah ada di `backend/api/openapi.yaml`. 403 dari katalog memakai skema error yang sama dengan guard role. Bila 403 belum terdokumentasi, tambahkan, lalu jalankan tes kontrak (Req 9.1). `/admin/roles` tidak ada di spec OpenAPI (bukan modul pilot), jadi tidak ditambahkan.

### CompanyPortal

**`features/role-management`** (revamp, `ui-ux-pro-max`, Merah Sirih):
- `RoleManagementPage`: state di URL `?portal=main|vendor&role=<id>&q=<cari>` lewat search params TanStack (validasi zod, fallback `main`) (Req 5.1, 6.1).
- Header: `BackLink` (rata kiri setelah Small Work) + `PageHeader`, dengan "Buat Peran" sebagai satu-satunya aksen merah.
- `PortalTabs`: tablist ARIA (`role="tablist"`, panah kiri/kanan, `aria-selected`), "Portal Utama" / "Portal Cabang" beserta jumlah peran.
- Master (kiri, ±360 px): input cari ber-`aria-label` dengan tombol bersihkan. Tiap baris peran berisi `RoleBadge`, deskripsi 2 baris (`line-clamp`, `title` = teks penuh), jumlah user aktif (`tabular-nums`), dan untuk vendor chip cakupan pihak (ikon + teks, bukan warna saja). Ada state kosong per tab dan "tidak ada hasil" dengan tombol hapus pencarian.
- Detail (kanan): ringkasan peran (nama, deskripsi, portal, cakupan, jumlah user) dan `PermissionEditor` yang hanya memuat katalog tab aktif. Tombol Simpan dan Batalkan muncul saat ada perubahan, dengan konfirmasi bila pindah peran sebelum menyimpan.
- `CreateRoleDialog` menerima `portal`. Di tab vendor, prefix `VENDOR-` tampil terkunci dan cakupan pihak (radio Replenish / Vault / Keduanya) wajib dipilih. Judul dialog: "Buat Peran Portal Cabang".
- Layar < `lg`: master dan detail ditumpuk. Memilih peran menggulir ke detail dan memindahkan fokus ke judulnya.
- State: skeleton saat loading, error dengan tombol coba lagi, kosong, dan berhasil (Req 6.4).

**Form user (`features/admin-users`)**: opsi peran diambil dari API (lihat DQ3), lalu difilter sesuai jenis user: vendor → `portal='vendor'`, internal → `main`. `userFormSchema.ts`, `rbac-settings/types.ts`, dan `lib/auth/store.ts` tidak lagi memakai daftar `VENDOR-*` yang ditulis tetap; pemeriksaan "vendor" memakai prefix (Req 8.1, 8.2).

**`RoleBadge`**: peran yang tidak dikenal → varian netral; prefix `VENDOR-` → varian vendor (Req 8.3).

### VendorPortal

- `features/auth`: setelah login atau refresh, ambil `GET /api/v1/auth/me/permissions` (TanStack Query, disimpan di context auth).
- `lib/constants.ts` `NAV_ITEMS`: tiap item diberi `key` (`vendor-dsr`, `vendor-orders`, `vendor-realizations`, `vendor-notifications`). `AppShell` memfilter dengan izin, dan `hideRealizations` dihapus (Req 4.2).
- `beforeLoad` route `/dsr`, `/orders`, `/orders/$id`, dan `/realizations` mengecek key. Tanpa izin → redirect ke item nav pertama yang dimiliki, atau ke halaman baru `NoAccess` bila tidak ada sama sekali (Req 4.3).
- `partyScopeLabel(role)` diganti dengan label dari `party_scope` di `/auth/me` (Req 7.6).

## Data Models

### Migrasi `047_role_portal.sql` (satu tx, pola 046)

1. **Guard**: berhenti bila `VENDOR-USER`, `VENDOR-ATM-USER`, `VENDOR-VAULT-USER`, `EXT-CLIENT`, atau user ADMIN (untuk `granted_by`) tidak ada.
2. **Teks rusak (Req 1)**:
   ```sql
   UPDATE public.roles SET description = replace(description, 'â€”', '—'), updated_at = now()
    WHERE description LIKE '%â€”%';
   -- locations: kolom yang terkena dipastikan di task (seed 002 baris 961, 1500, 1742), pola yang sama per kolom.
   ```
   Idempoten, karena baris yang sudah bersih tidak cocok dengan `LIKE`. File disimpan sebagai UTF-8; tes memastikan hasilnya `—` (Property 1).
3. **`roles.portal`** `text NOT NULL DEFAULT 'main'`, CHECK `IN ('main','vendor','service')`. Backfill: `VENDOR-%` → `vendor`, `EXT-CLIENT` → `service` (tidak tampil di tab mana pun, Req 2.6). CHECK `roles_portal_prefix_chk`: `(portal = 'vendor') = (role LIKE 'VENDOR-%')`.
4. **`roles.party_scope`** `text[] NULL`, CHECK `party_scope <@ ARRAY['replenish','vault']` dan `(portal = 'vendor') = (party_scope IS NOT NULL AND cardinality(party_scope) > 0)`. Backfill: `VENDOR-USER` → `{replenish,vault}`, `VENDOR-ATM-USER` → `{replenish}`, `VENDOR-VAULT-USER` → `{vault}`.
5. **`menu_features.portal`** `text NOT NULL DEFAULT 'main'`, CHECK `IN ('main','vendor')`.
6. **Katalog vendor (Req 3.1)**: menu `vendor-dsr` "DSR Menu" (0), `vendor-orders` "CIT Orders" (1), `vendor-realizations` "Realisasi" (2), semuanya `portal='vendor'`, `ON CONFLICT (key) DO NOTHING`.
7. **Grant awal (Req 3.3)**: `VENDOR-USER` dan `VENDOR-ATM-USER` → ketiga key; `VENDOR-VAULT-USER` → `vendor-dsr` dan `vendor-orders`. `granted_by` = user ADMIN pertama, `ON CONFLICT DO NOTHING`.
8. **Post-check**: 3 key vendor ada, dan tidak ada peran vendor tanpa scope.

Rollback (dev script `backend/scripts/dev/rollback_047_role_portal.sql`): hapus grant dan key `vendor-*`, lalu `DROP COLUMN` `menu_features.portal`, `roles.party_scope`, dan `roles.portal`. Perbaikan teks tidak dibalik.

Binary lama tetap jalan setelah `047`, karena semua kolom baru punya default atau nullable dan tidak dibaca oleh binary lama. Peran vendor baru hanya bisa dibuat oleh binary baru, jadi daftar tetap lama tidak pernah bertemu peran baru.

### sqlc

Di `backend/queries/` (file query role management yang ada): `ListRolesWithPermissions` (+ portal, party_scope, active_user_count), `ListCatalog` (+ portal), `CreateRole` (+ portal, party_scope), `GetRolePartyScope`, `ListVendorRolesForParty`. Setelah `sqlc generate`, lakukan perbaikan `UserLeafe` → `UserLeave` (CLAUDE.md §2a).

### Perubahan yang terlihat user

Tidak ada perubahan akses saat rilis, karena grant awal = akses sekarang (Property 2) dan `VENDOR-VAULT-USER` tetap tidak melihat Realisasi. Yang berubah: tampilan Manajemen Peran, teks deskripsi, dan Portal_Cabang yang kini bisa diatur.

## Correctness Properties

### Property 1: Perbaikan teks idempoten
Menjalankan langkah 2 dua kali menghasilkan DB yang sama, dan setelahnya tidak ada `roles`/`locations` yang memuat `â€”`. _Validates: Req 1.1, 1.2_

### Property 2: Grant awal = akses sekarang
Untuk setiap peran `VENDOR-*`, key `vendor-*` setelah `047` sama dengan menu yang terlihat sebelum `047` (`NAV_ITEMS` minus `hideRealizations`). _Validates: Req 3.3_

### Property 3: Portal, prefix, dan scope selalu konsisten
Untuk setiap baris `roles`: `portal='vendor'` ⇔ prefix `VENDOR-` ⇔ `party_scope` tidak kosong. Dijaga CHECK constraint, dan dites dengan insert yang melanggar. _Validates: Req 2.1, 7.2, 7.3_

### Property 4: Tidak ada izin lintas portal
`PUT /roles/{id}/permissions` dengan entri katalog dari portal lain selalu 422 dan tidak mengubah `role_permissions`. _Validates: Req 2.4_

### Property 5: Katalog vendor hanya mempersempit
Request vendor lolos ⇒ (punya key) ∧ (lolos guard lama). Mencabut key menghasilkan 403. Untuk peran non-vendor di `/api/v1/dsr*`, keputusannya sama persis dengan sebelum `047`. _Validates: Req 4.4, 4.5, 4.6_

### Property 6: Setara perilaku cakupan pihak
Peran baru dengan `party_scope = X` mendapat hasil `ListOrders`/decide/`SubmitRealization` yang sama dengan peran lama yang scope-nya `X`. Dites table-driven terhadap tiga peran lama. _Validates: Req 7.4, 7.5_

### Property 7: Fail closed
Error query izin atau scope → 500/403, tidak pernah lolos. _Validates: Req 4.4, 7.4_

### Property 8: Izin dibaca per request
Pencabutan izin berlaku di request berikutnya tanpa login ulang (tanpa cache). _Validates: Req 4.7_

### Property 9: Sidebar dan route VendorPortal = izin
Untuk setiap himpunan izin: item nav = key yang dimiliki ∪ {Notifikasi}, dan route tanpa izin dialihkan. _Validates: Req 4.2, 4.3_

## Error Handling

| Kasus | Respons |
|---|---|
| Buat peran: portal tidak valid / scope hilang / prefix salah | 422 `validation_error` + `field` |
| Field portal/scope ikut di PUT izin | 422 (decoder strict) |
| Izin dari katalog portal lain | 422 `role_not_assignable` |
| User: peran tidak cocok dengan jenis user | 422 `ReferenceError` (`vendor_id`) / `ValidationError` (`auth_source`) |
| Vendor tanpa key menu | 403, body sama dengan `RequireRoles` |
| Error query izin/scope | 500 `internal_error` + `slog.Error` (role, key); fail closed |
| Peran vendor tanpa scope (dicegah CHECK) | 403 |

## Testing Strategy

- **Migrasi (integration, butuh `DATABASE_URL`)**: `047` dijalankan dua kali (P1), insert yang melanggar ditolak constraint (P3), grant awal per peran (P2).
- **rolemgmt**: validasi `CreateRole` table-driven (portal × prefix × scope), penolakan lintas portal (P4), `ADMIN_PARAM` ditolak (Req 5.4), `active_user_count`.
- **Middleware**: `RequireVendorPermission` table-driven: vendor dengan/tanpa key, internal lolos, error → 500 (P5, P7, P8).
- **vendor_order / vendor_party**: setara perilaku tiga scope (P6); `ListVendorRolesForParty` = hasil `ForParty` lama untuk peran lama.
- **user_admin**: kombinasi peran × `auth_source` × `vendor_id` (Req 2.5).
- **Kontrak**: `go test ./internal/handler -run OpenAPI`; `pnpm --dir frontend/CompanyPortal-Vite run gen:api`.
- **CompanyPortal (vitest + RTL)**: tab dan URL, pencarian dan "tidak ada hasil", master-detail dengan pilihan otomatis, dialog vendor (prefix terkunci, scope wajib), filter peran di form user, `RoleBadge` netral, axe.
- **VendorPortal (vitest)**: sidebar = izin (P9), redirect route, `NoAccess`.
- Manual browser: outstanding (user).

## Open Design Questions (✅ DIPUTUSKAN 2026-11-25, "tidak perlu mockup, lanjut buat task")

- DQ1–DQ3 mengikuti rekomendasi: portal dikunci ke prefix, Notifikasi di luar katalog, dan opsi peran untuk form user ditentukan di task 1.
- **DQ4: tanpa mockup.** Req 6.6 (mockup sebelum kode) dikesampingkan oleh user. Layout di section CompanyPortal di atas menjadi acuan UI, dan `ui-ux-pro-max` tetap dipakai saat menulis kode.

Teks pertanyaan asli:

- **DQ1**: Role_Portal sebagai kolom yang dikunci ke prefix `VENDOR-` lewat CHECK (rekomendasi), atau kolom bebas tanpa ikatan prefix? Opsi kedua butuh lookup DB di setiap `IsInternalRole`/`IsVendor` dan di frontend.
- **DQ2**: Notifikasi VendorPortal di luar katalog dan selalu tampil (rekomendasi), atau masuk katalog sebagai key Always_On?
- **DQ3**: Opsi peran untuk form user diambil dari `GET /admin/roles` bila guard-nya sama dengan layar user, atau dari endpoint ringan `GET /admin/users/role-options` di bawah guard admin users (rekomendasi bila guard berbeda; dicek di task 1)?
- **DQ4**: Mockup UI (Req 6.6) dibuat sekarang di Artifact "Design" dengan `ui-ux-pro-max` sebagai bagian dari gerbang design ini (rekomendasi)?

## Files touched (perkiraan)

- `backend/migrations/047_role_portal.sql`, `backend/scripts/dev/rollback_047_role_portal.sql`
- `backend/queries/*` (role management), `backend/internal/db/*` (sqlc)
- `backend/internal/vendorrole/vendorrole.go`, `backend/internal/rolemgmt/{service,repository,evaluator,keys}.go`
- `backend/internal/service/{vendor_order,vendor_party}.go`, `backend/internal/auth/user_admin.go`
- `backend/internal/handler/{auth_handler,vendor_order_handler,replenishment_handler,role_mgmt_handler}.go`, `backend/cmd/api/main.go`, `backend/api/openapi.yaml`
- `pkg/userrules` (bila task 1 menemukan celah di extapi)
- CompanyPortal: `features/role-management/*`, `features/admin-users/lib/userFormSchema.ts`, `features/rbac-settings/{types.ts,components/RoleBadge.tsx}`, `lib/auth/store.ts`
- VendorPortal: `lib/constants.ts`, `app/AppShell.tsx`, `features/auth/*`, route `dsr`/`orders`/`realizations`, `NoAccess` baru
