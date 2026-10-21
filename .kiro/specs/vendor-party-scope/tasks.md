# Implementation Plan: Vendor Party Scope

## Overview

Backend-first: migrasi → paket `vendorrole` → cakupan Orders/Realisasi → notifikasi → admin users → peran PIC (layar + CSV) → `/auth/me` → frontend kedua portal → seed + kontrak → checkpoint. Setiap langkah membawa test-nya. Cakupan selalu di SQL, dibaca dari primary; daftar di replica. Peran PIC lewat layar = immediate-apply + audit dalam tx (deviasi GR#3 terdokumentasi); create PIC dan impor CSV tetap maker-checker.

Referensi pola:
- `backend/internal/service/vendor_order.go` (`scope`, `getScoped`) + `backend/queries/vendor_parties.sql` — cakupan vendor.
- `backend/internal/service/region_admin.go` / `internal/rolemgmt/service.go` — audit dalam tx.
- `backend/internal/service/masterdata_applier_vendor_pic.go`, `masterdata_export.go`, `masterdata_import_*.go` — PIC maker-checker + CSV.
- `backend/migrations/030_vault_allocation.sql` — FK komposit + unique target.

### Model Split

**Sonnet-runnable**
- 2. Paket `vendorrole` — Haiku, Low (Haiku)
- 4. Notifikasi pihak per role + peran PIC — Sonnet, Medium (perlu 2)
- 7. Peran PIC di CSV master data — Sonnet, Medium (perlu 6)
- 8. `/auth/me` + konteks vendor — Sonnet, Low (perlu 1)
- 9. Checkpoint backend — Sonnet, Medium (perlu 1–8)
- 10. Kontrak OpenAPI — Haiku, Low (Haiku) (perlu 3)
- 11. VendorPortal — Sonnet, Medium (perlu 8, 10)
- 12. CompanyPortal — Sonnet, Medium (perlu 5, 6)
- 13. Seed dev + dokumentasi — Haiku, Low (Haiku) (perlu 1)
- 14. Checkpoint akhir — Sonnet, Medium (perlu semua)

**Opus-required**
- 1. Migrasi `032_vendor_party_scope.sql` — Opus, High — migrasi live-data auth/RBAC (role baru + FK komposit pada `users`)
- 3. Cakupan Orders + Realisasi per role — Opus, High — auth/RBAC, kebocoran data antar pihak/cabang (perlu 1, 2)
- 5. Admin users: role vendor + cabang + vendor-options — Opus, Medium — provisioning akun (auth) + pencabutan akses `ADMIN_PARAM` (perlu 1, 2)
- 6. Peran PIC: layar + maker-checker create — Opus, High — deviasi GR#3 (immediate-apply + audit-in-tx) berinteraksi dengan Applier maker-checker (perlu 1)

Lintas grup: Sonnet 4 dan 8 bisa jalan paralel dengan Opus 3/5 setelah 1 + 2 selesai; Sonnet 7 dan 12 menunggu Opus 6; Sonnet 12 juga menunggu Opus 5.

## Tasks

- [x] 1. Migrasi `032_vendor_party_scope.sql`
  - Buat `backend/migrations/032_vendor_party_scope.sql` sesuai design "Data model": insert dua role (`ON CONFLICT (role) DO NOTHING`), salin `role_permissions` dari `VENDOR-USER`, pre-check + `vendor_branches_id_vendor_uq` + `users_vendor_branch_owner_fk`, `vendor_pics.roles text[]` DEFAULT ketiga peran + CHECK + COMMENT. Satu tx, forward-only.
  - Jalankan `cd backend && go run ./cmd/migrate status` lalu `up` di dev (DATABASE_URL via `localhost`); cek `SELECT role FROM roles WHERE role LIKE 'VENDOR-%'` dan `\d vendor_pics`.
  - Perbarui `backend/internal/db/models.go` (`VendorPic.Roles []string`) lewat `sqlc generate` (v1.31.1; hand-fix `UserLeafe → UserLeave`).
  - Catat di CLAUDE.md Sec 3 + `docs/data-map.md`.
  - _Requirements: 1.1, 1.2, 1.3, 2.5, 5.1, 5.2_
  - _Model: Opus, Effort: High — migrasi auth/RBAC + FK baru pada `users` live; pre-check yang salah bisa memblokir deploy atau meloloskan data cabang tak sah._

- [x] 2. Paket `backend/internal/vendorrole`
  - [x] 2.1 `vendorrole.go`: konstanta `User`, `ATMUser`, `VaultUser`; `All`; `IsVendor`; `PartyRoles`; `ForParty` (design "Pemetaan role → peran pihak").
    - _Requirements: 1.4, 3.2, 7.1_
  - [x] 2.2 `vendorrole_test.go`: tabel semua role (termasuk non-vendor → nil/false) dan `ForParty("replenish"|"vault"|"cashcount"|"")`.
    - _Requirements: 3.2, 7.1, 7.3_
  - _Model: Haiku, Effort: Low — pemetaan statis kecil, sepenuhnya ditentukan design._

- [x] 3. Cakupan Orders + Realisasi per role
  - [x] 3.1 `backend/queries/vendor_parties.sql`: `GetUserVendorScope` + `r.role`; `ListVendorOrders`, `CountVendorOrders`, `GetVendorOrder` + `p.role = ANY(sqlc.arg('party_roles')::text[])`. `sqlc generate`.
    - _Requirements: 3.2, 3.3, 3.9_
  - [x] 3.2 `service/vendor_order.go`: hapus `vendorUserRole`; `scope()` pakai `vendorrole.PartyRoles`, tolak role JWT ≠ DB; `vendorScope.partyRoles` diteruskan ke semua query cakupan.
    - _Requirements: 3.1, 3.2, 3.4, 3.6, 3.8_
  - [x] 3.3 `service/replenishment_read.go` `ListRealizations`: `VENDOR-VAULT-USER` → `ErrNotAuthorized`.
    - _Requirements: 4.2, 4.4_
  - [x] 3.4 Handler: `RequireRoles(vendorrole.All...)` di `vendor_order_handler.go` `Routes()` dan `replenishment_handler.go` `RealizationRoutes()`; perbarui `vendor_order_handler_test.go` (role baru lolos route; role internal 403).
    - _Requirements: 3.1_
  - [x] 3.5 Integration test `vendor_order_integration_test.go`: satu request dengan pihak replenish + vault Abacus Medan dan replenish cabang lain; kasus Req 10.1 (a)–(d): vault Medan tidak melihat replenish Medan; ATM-user Medan tidak melihat cabang lain; `VENDOR-USER` melihat keduanya (regresi); role diganti setelah token terbit → 403; vault → `POST …/realization` 404 dan daftar realisasi 403. Membuktikan Property 3 (regresi `VENDOR-USER`) dan Property 4 (token basi).
    - _Requirements: 3.4, 3.5, 3.6, 3.7, 3.8, 4.1, 4.2, 4.3, 10.1_
  - [ ]* 3.6 Property test (rapid): Property 1 + 2 (tanpa kebocoran peran/cabang) atas kombinasi acak role × cabang akun × pihak.
    - _Requirements: 3.2, 10.1_
  - _Model: Opus, Effort: High — inti RBAC/cakupan data antar vendor; kebocoran = insiden keamanan; banyak cabang kasus (role × cabang × token basi)._

- [ ] 4. Notifikasi pihak per role + peran PIC
  - [x] 4.1 `backend/queries/notifications.sql`: `ListActiveUserRecipientsByVendorBranches` → `(branch_id, roles text[])` dengan join `roles`; `ListVendorBranchNotificationPicEmails` → `(branch_id, pic_role)` dengan `pic_role = ANY(p.roles)`. `sqlc generate`.
    - _Requirements: 7.1, 7.2_
  - [x] 4.2 `internal/notification/notification.go`: ganti `VendorBranchIDs` dengan `VendorParties []VendorPartyTarget{BranchID, UserRoles, PicRole}`; dedup tetap. Perbarui `notification_test.go` + `integration_test.go`.
    - _Requirements: 7.1, 7.2_
  - [x] 4.3 `service/vendor_party.go`: `partyNotice.role`; `notifyParties` kirim `VendorPartyTarget{…, UserRoles: vendorrole.ForParty(role), PicRole: role}`.
    - _Requirements: 7.1, 7.3_
  - [x] 4.4 Integration test (Req 10.1 e): pihak vault Medan → hanya `VENDOR-VAULT-USER` Medan/vendor-wide + `VENDOR-USER` + PIC dengan peran `vault`; PIC `cashcount`-saja tidak menerima; notifikasi realisasi dan internal tidak berubah. Membuktikan Property 5.
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 10.1_
  - _Model: Sonnet, Effort: Medium — perluasan recipient yang sudah ada; risiko terkendali oleh integration test._

- [x] 5. Admin users: role vendor + cabang + vendor-options
  - [x] 5.1 `backend/queries/users_admin.sql`: `vendor_branch_id` di list/get/create/update (+ `vendor_branch_name` via `LEFT JOIN`); `VendorBranchBelongsToVendor`; `ListVendorOptionsAdmin`, `ListVendorBranchOptionsAdmin`. `sqlc generate`.
    - _Requirements: 2.1, 2.4, 2.7_
  - [x] 5.2 `internal/auth/user_admin.go`: `VendorBranchID` di request; validasi `vendorFields` (role vendor ⇒ `local` + `vendor_id`; non-vendor ⇒ tanpa cabang; cabang milik vendor aktif); audit before/after memuat field baru.
    - _Requirements: 1.4, 2.2, 2.3, 2.4, 2.6_
  - [x] 5.3 `handler/admin_user_handler.go`: body/respons `vendor_branch_id` + `vendor_branch_name`; `GET /vendor-options` (didaftarkan sebelum `/{id}`, replica). `cmd/api/main.go`: guard `/api/v1/admin/users` → `RequireRoles("APPACCESS", "ADMIN")` (D8); re-check role yang sama di `UserAdminService.Create`/`Update`. Perbarui komentar handler ("APPACCESS-only") agar sesuai.
    - _Requirements: 2.0, 2.1, 2.7_
  - [x] 5.4 Test: unit `user_admin` (matriks role × auth_source × vendor × cabang) + integration (Req 10.1 f: service 400 dan FK DB menolak cabang vendor lain); handler test guard: `APPACCESS`/`ADMIN` lolos, `ADMIN_PARAM` 403 di list/create/update/`set-initial-password`/`vendor-options`.
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 10.1_
  - _Model: Opus, Effort: Medium — provisioning akun (auth) dengan aturan validasi baru; polanya sudah ada, cabang kasusnya terbatas._

- [x] 6. Peran PIC: layar (immediate + audit) dan maker-checker create
  - [x] 6.1 `backend/queries/vendor_pics_admin.sql`: `roles` di list/get/create; `GetVendorPicRolesForUpdate`; `SetVendorPicRoles`. `UpdateVendorPicAdmin` tidak menyentuh `roles`. `sqlc generate`.
    - _Requirements: 5.1, 5.7, 5.8_
  - [x] 6.2 `service/vendor_pic_admin.go` + `masterdata_applier_vendor_pic.go`: `Roles *[]string` di `VendorPicPayload` (nil = ketiga peran) dan `VendorPicUpdatePayload` (nil = tidak diubah); Applier update memanggil `SetVendorPicRoles` di tx apply bila terisi.
    - _Requirements: 5.6, 5.7, 6.3, 6.4_
  - [x] 6.3 `service/vendor_pic_roles.go` (baru) `SetRoles`: re-check `ADMIN`/`ADMIN_PARAM`, normalisasi (trim, sort, unique, kode dikenal), tx `FOR UPDATE` → update → `audit.NewWriter(tx)` `vendor_pic_roles_updated` → commit; tanpa perubahan = sukses tanpa audit.
    - _Requirements: 5.3, 5.4, 5.5_
  - [x] 6.4 `handler/admin_vendor_pic_handler.go`: `PUT /{id}/roles`; respons list/detail memuat `roles`.
    - _Requirements: 5.3, 5.8_
  - [x] 6.5 Integration test: set peran sukses + 1 audit; audit gagal → rollback (Req 10.1 g); peran tak dikenal/duplikat 400; PIC vendor lain/nonaktif 404; non-ADMIN 403; create via maker-checker dengan dan tanpa peran; update form tidak mengubah peran.
    - _Requirements: 5.3, 5.4, 5.5, 5.6, 5.7, 10.1_
  - [ ]* 6.6 Property test: Property 6 (audit-or-nothing peran PIC).
    - _Requirements: 5.4_
  - _Model: Opus, Effort: High — deviasi GR#3 (immediate-apply) pada master data + interaksi dengan Applier maker-checker; audit-in-tx harus benar._

- [x] 7. Peran PIC di CSV master data
  - [x] 7.1 `masterdata_export.go`: header PIC + `roles`; sel `strings.Join(roles, ";")` atau `none`.
    - _Requirements: 6.1_
  - [x] 7.2 `masterdata_import_csv.go` / `_validate.go` / `_confirm.go`: header lama sebagai legacy (`roles` kosong); `parsePicRoles` (kosong → nil, `none` → `[]`, split `;`, validasi); `picFields` mengisi `Roles`.
    - _Requirements: 6.2, 6.3, 6.4, 6.5_
  - [x] 7.3 Test: `masterdata_import_test.go` (dry-run nilai salah/duplikat, header lama), `masterdata_import_integration_test.go` round-trip ekspor → impor (Property 7), create tanpa peran = ketiga peran, update kosong = tidak berubah.
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_
  - _Model: Sonnet, Effort: Medium — perluasan kolom CSV di pipeline yang sudah ada; aturan kosong/none perlu teliti tapi terbatas._

- [x] 8. `/auth/me` + konteks vendor
  - `queries/auth.sql` `GetMe` + `vendor_name`, `vendor_branch_id`, `vendor_branch_name`; handler/respons `/me` menambah ketiganya (`null` eksplisit). Test handler `/me` untuk user vendor-wide, berbatas cabang, dan internal.
  - _Requirements: 8.3_
  - _Model: Sonnet, Effort: Low — tambahan field read-only pada respons yang ada._

- [ ] 9. Checkpoint backend
  - `cd backend && go test ./...` + integration (`DATABASE_URL` diisi, 0 skip untuk test baru), `golangci-lint run --build-tags integration ./...`, coverage paket baru ≥ 80%, `go test ./internal/handler -run OpenAPI`.
  - _Requirements: 10.2_
  - _Model: Sonnet, Effort: Medium — triase kegagalan build/test/lint._

- [ ] 10. Kontrak OpenAPI
  - `backend/api/openapi.yaml`: deskripsi role path `/vendor/replenish-orders*` → ketiga role vendor; tanpa perubahan skema. `pnpm --dir frontend/CompanyPortal-Vite run gen:api`, commit `schema.d.ts`.
  - _Requirements: 9.1, 9.2_
  - _Model: Haiku, Effort: Low — teks deskripsi + regenerasi tipe._

- [x] 11. VendorPortal
  - [x] 11.1 `src/lib/vendorRoles.ts` (`VENDOR_ROLES`, `isVendorRole`, `partyRolesOf`) + `DbRole` union di `src/lib/types.ts`.
    - _Requirements: 8.1_
  - [x] 11.2 `features/auth/AuthContext.tsx`: kedua guard pakai `isVendorRole`; `AuthUser` + `vendorName`, `vendorBranchId`, `vendorBranchName`.
    - _Requirements: 8.1, 8.3_
  - [x] 11.3 Shell: baris cakupan (Req 8.4); nav Realisasi disembunyikan untuk vault; `routes/realizations.tsx` "Tidak tersedia untuk peran Anda" (ikon + teks).
    - _Requirements: 8.4, 8.5, 8.6_
  - [x] 11.4 Test Vitest: guard 3 role diterima + role internal ditolak (init + refresh); nav dan halaman Realisasi per role; baris cakupan vendor-wide vs cabang. Threshold VP 93/84/90/93.
    - _Requirements: 8.1, 8.4, 8.5, 10.2_
  - _Model: Sonnet, Effort: Medium — guard auth portal (kritis tapi sederhana) + UI kecil._

- [ ] 12. CompanyPortal
  - [ ] 12.1 `lib/auth/store.ts`: tolak ketiga role vendor (login + refresh); `UserRole` + dua role. `features/rbac-settings` `types.ts` + `RoleBadge.tsx` untuk role baru.
    - _Requirements: 8.2_
  - [ ] 12.2 `features/admin-users`: `ROLE_OPTIONS` + label; `userFormSchema` aturan role vendor + `vendor_branch_id`; hook `useVendorOptions`; `UserFormDialog` select Cabang (reset saat vendor berubah); `UsersTable` kolom Cabang; `AdminUsersPage` + `UserFilterBar` pakai `useVendorOptions`. Kartu Manajemen User di `SettingsHubPage` disembunyikan untuk `ADMIN_PARAM` (D8).
    - _Requirements: 2.8, 2.9, 2.10, 2.11, 2.12, 2.13_
  - [ ] 12.3 `features/admin-vendors`: kolom Peran di `PicsPanel`; checkbox peran di `VendorPicFormDialog` (create saja); `PicRolesDialog` + `useSetPicRoles` dengan catatan "Langsung berlaku, tanpa persetujuan".
    - _Requirements: 5.9_
  - [ ] 12.4 Test Vitest: guard tolak role vendor; schema + form user (cabang reset, validasi, APPACCESS memuat vendor-options); `PicRolesDialog` + kolom peran. Threshold CP 84/80/80/85.
    - _Requirements: 2.8, 2.9, 2.10, 2.11, 5.9, 8.2, 10.2_
  - _Model: Sonnet, Effort: Medium — komponen React mengikuti pola yang ada; guard auth ikut diuji._

- [ ] 13. Seed dev + dokumentasi
  - `backend/scripts/dev/seed_vendor_users.sql`: empat akun ABACUS (design "Seed dev"), cari cabang via `branch_code`, idempoten. Jalankan di dev.
  - Catat deviasi peran PIC di CLAUDE.md Sec 12; perbarui `rbac-role-feature-matrix` steering (role baru), `.claude/development-progress.md`, baris fitur di `.claude/sdlc/README.md`; `graphify update .`.
  - _Requirements: 9.3_
  - _Model: Haiku, Effort: Low — SQL seed sederhana + pembaruan dokumen._

- [ ] 14. Checkpoint akhir
  - Backend + kedua frontend: build, lint, test, coverage hijau. Manual browser check = **outstanding (user)**: login `vendor.abacus` + empat akun seed, buka `REP-ABA-MDN-…`, cek notifikasi, ubah peran PIC, ekspor/impor CSV PIC, buat akun vendor dari APPACCESS.
  - _Model: Sonnet, Effort: Medium — triase lintas tiga codebase._

## Notes

- Task bertanda `*` (3.6, 6.6) opsional (property test); integration test Req 10.1 tetap wajib sebelum merge.
- Setiap `sqlc generate` wajib v1.31.1, lalu hand-fix `UserLeafe → UserLeave`. Task 3.1, 4.1, 5.1, 6.1, dan 8 masing-masing menyentuh file query berbeda; `models.go` hanya berubah di task 1.
- Task 5.3 mencabut akses `ADMIN_PARAM` ke `/api/v1/admin/users*` (D8). Perubahan RBAC ini disengaja, bukan efek samping.
- Deviasi GR#3 (peran PIC immediate-apply) wajib dicatat di CLAUDE.md Sec 12 (task 13).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "2.1", "11.1", "12.1"] },
    { "id": 1, "tasks": ["2.2", "3.1", "4.1", "5.1", "6.1", "8", "13"] },
    { "id": 2, "tasks": ["3.2", "4.2", "5.2", "6.2", "6.3", "11.2"] },
    { "id": 3, "tasks": ["3.3", "3.4", "4.3", "5.3", "6.4", "7.1", "7.2", "11.3"] },
    { "id": 4, "tasks": ["3.5", "4.4", "5.4", "6.5", "7.3", "10", "11.4", "12.2", "12.3"] },
    { "id": 5, "tasks": ["3.6", "6.6", "12.4"] }
  ]
}
```

> Catatan penjadwalan: checkpoint (9, 14) dan parent murni (2–7, 11, 12) tidak masuk graph. `service/vendor_order.go` disentuh berurutan 3.2 → 3.3 (beda wave). 7.1/7.2 menunggu payload `Roles` dari 6.2. 11.2 menunggu respons `/me` dari 8. 12.2 menunggu API admin users (5.3), dan 12.3 menunggu endpoint peran PIC (6.4). 10 menunggu guard route 3.4.
