# Implementation Plan: Daftar PKS + Reminder per PKS

> Status: accepted 2026-11-19 (engineer) — "Accepted, show wave 0". Berdasarkan `requirements.md` (accepted 2026-11-19, PO) dan `design.md` (accepted 2026-11-19, engineer).

## Overview

Urutan pengerjaan:
1. Migrasi `044` ditulis, lalu diterapkan ke dev. Langkah `migrate up` butuh OK user (CLAUDE.md Sec 4 #7).
2. Kontrak bersama `pkg/reports` (role viewer, label status, validator filter, konstanta reminder, header ekspor).
3. Query + sqlc `backend`.
4. Service `backend` (create + reminder, Daftar PKS, daftar vendor).
5. Handler, routing RBAC, OpenAPI, test backend.
6. Ekspor `backend-exq`.
7. CompanyPortal (`features/admin-pks` baru, `admin-vendors`, route, hub).

Spec ditutup dengan gate kualitas, dokumentasi, migrasi pembersih `045` (setelah rilis), dan cek browser oleh user.

Integration test berjalan terhadap DB dev (`DATABASE_URL`, `localhost`), jadi `044` harus sudah diterapkan ke dev (task 1.3) sebelum integration test yang menyentuh PKS dijalankan.

Pola yang dirujuk:
- PKS yang sudah ada: `vendor_pks_admin.go`, `masterdata_applier_vendor_pks.go`, `admin_vendor_pks_handler.go`, `queries/vendor_pks.sql`, `pkg/reports/vendor_pks.go`.
- Deviasi GR#3 dengan audit di tx: peran PIC vendor (`PUT …/pics/{id}/roles`, `vendor-party-scope`).
- Seed menu + `role_permissions`: migrasi `027` (`settings.acm-areas`).
- Daftar admin `LIMIT/OFFSET` + `Count*`: `queries/vendors_admin.sql`.
- sqlc v1.31.1 + perbaikan `UserLeafe` → `UserLeave`.
- Skill: `ecc:golang-patterns` / `ecc:golang-testing` (Go), `ui-ux-pro-max` (task 7).

### Model Split

**Sonnet-runnable**
- 1.2 Skrip rollback dev — Sonnet, Medium (needs 1.1)
- 1.4 Integration test migrasi + Property 1 — Sonnet, High (needs 1.3, 2.1)
- 2.1–2.2 Kontrak `pkg/reports` + test — Sonnet, Medium
- 3.1–3.3 Query + sqlc — Sonnet, Medium (needs 1.1)
- 4.1 Create PKS + `renewal_days` — Sonnet, High (needs 3.3)
- 4.3 `PksListService` — Sonnet, High (needs 3.3, 2.1)
- 4.4 `VendorAdminService` (pks_count + role) — Sonnet, Medium (needs 3.3)
- 5.2 OpenAPI + contract test — Sonnet, Medium (needs 5.1)
- 5.3 Test backend — Sonnet, High (needs 5.1, 1.3)
- 6.1–6.3 Ekspor `backend-exq` — Sonnet, Medium/High (needs 1.1, 2.1)
- 7.1–7.5 Frontend — Sonnet, Medium (needs 5.2)
- 8. Checkpoint gate kualitas — Sonnet, Medium (needs 1–7)
- 9. Dokumentasi — Haiku, Low (needs 8)
- 11. Cek browser manual (user) — Haiku, Low (needs 9)

**Opus-required**
- 1.1 Migrasi `044`: Opus, High. Kolom NOT NULL + backfill + CHECK, fungsi status baru, seed menu/izin yang membuka akses ATM-SPV.
- 1.3 Terapkan ke dev: Opus, High. `migrate status`, guard, `migrate up`, setiap langkah dengan OK user.
- 4.2 `UpdateReminder`: Opus, High. Deviasi GR#3: tulis langsung + audit di tx yang sama, `FOR UPDATE`, RBAC service layer.
- 5.1 Handler + regroup route vendor: Opus, High. Memindah `AdminVendorHandler` ke grup baca baru tanpa membuka endpoint tulis/sub-resource ke ATM-SPV (auth).
- 10. Migrasi pembersih `045`: Opus, Medium. Drop fungsi DB setelah rilis; harus memastikan tidak ada pemanggil.

**Dependensi lintas grup**
- Sonnet 3.x dan 6.1 butuh Opus 1.1 (sqlc membaca file migrasi).
- Sonnet 1.4, 5.3, 6.3 butuh Opus 1.3 (DB dev sudah `044`).
- Opus 4.2 butuh Sonnet 4.1 (file yang sama, `validateRenewalDays`). Opus 5.1 butuh Sonnet 4.1, 4.3, 4.4 dan Opus 4.2. Sonnet 5.2 dan 7.x butuh Opus 5.1.

## Tasks

- [x] 1. Migrasi `044_vendor_pks_list.sql`
  _Model: Opus (1.1, 1.3) / Sonnet (1.2, 1.4), High — migrasi data + akses baru (Sec 4 #7)_
  - [x] 1.1 Tulis `backend/migrations/044_vendor_pks_list.sql` (satu tx): guard PKS dengan masa < 30 hari; `renewal_days integer NOT NULL DEFAULT 270` + backfill `LEAST(270, expiry - effective)` + CHECK 30–3666 + CHECK ≤ masa PKS; fungsi `vendor_pks_record_status(replaced_at, terminated_at, effective, expiry, renewal_days, today)`; menu `settings.pks` + `role_permissions` (ADMIN, ADMIN_PARAM → `settings.pks`; ATM-SPV → `settings` + `settings.pks`); comment kolom/tabel. Fungsi lama tidak di-drop.
    - _Requirements: 1.1, 3.1, 3.2, 3.4, 9.1, 11.3_
  - [x] 1.2 `backend/scripts/dev/rollback_044_vendor_pks_list.sql`: drop constraint + kolom, fungsi baru, `role_permissions` + menu baru.
    - _Requirements: 1.1, 9.1_
  - [x] 1.3 Terapkan ke dev: `go run ./cmd/migrate status` → OK user → `up` → `status` (43 applied, 0 pending); catat di `### Test Results`.
    - _Requirements: 1.1, 9.1_
  - [x] 1.4 Integration test migrasi (`//go:build integration`): CHECK `renewal_days` (29/30/3666/3667, > masa PKS ditolak), default 270, menu + izin per role; **Property 1**: `rapid` membandingkan fungsi status referensi Go dengan `vendor_pks_record_status` di DB pada input acak, termasuk `expiry = today`.
    - _Requirements: 1.1, 3.1, 3.2, 9.1_

- [x] 2. Kontrak bersama `pkg/reports/vendor_pks.go`
  _Model: Sonnet, Medium — konstanta + validator bersama dua backend_
  - [x] 2.1 Tambah `PksViewerRoles` (ADMIN, ADMIN_PARAM, ATM-SPV), `PksStatusLabels` (6 status), `PksListStatusFilters` (+ `terkini`, `semua`), `ParsePksListFilter`, `RenewalDaysMin/Max/Default`, header + row/cells ekspor baru (Masa Berlaku (hari), Reminder (hari), tanpa kolom bulan) dengan nama baru di samping yang lama. **Hanya menambah**: simbol lama (`VendorPksRenewalMonthsDefault`, `ParseVendorPksRenewalMonths`, `ParsePksFilter`, `belum_diisi`, `VendorPksViewerRoles`, header/row lama) dan test-nya tetap ada agar `backend` + `backend-exq` tetap compile; dihapus di 5.3 setelah pemanggil terakhir hilang.
    - _Requirements: 1.5, 3.5, 4.3, 4.4, 10.2, 10.3, 10.5_
  - [x] 2.2 Unit test: `ParsePksListFilter` (default `terkini`, whitelist, upper-case, `q` > 100 ditolak, `belum_diisi` ditolak), header, label.
    - _Requirements: 4.3, 4.4, 10.2, 10.3_

- [x] 3. Query + sqlc `backend`
  _Model: Sonnet, Medium — query baca + satu UPDATE kolom_
  - [x] 3.1 `queries/vendor_pks_list.sql` (baru): `ListPksAdmin`, `CountPksAdmin` (filter status/type/vendor/q, hanya `FLM_VENDOR`, urut nama vendor, type, efektif terbaru, id), `ListPendingPksChanges`, `GetVendorPksForUpdate`, `UpdateVendorPksRenewalDays`.
    - _Requirements: 2.1, 2.6, 3.5, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_
  - [x] 3.2 `queries/vendor_pks.sql`: `InsertVendorPks` + `renewal_days`; hapus `ListVendorPksByVendor`, `GetVendorPksStatus`. `queries/vendors_admin.sql`: hapus join/filter PKS dari `ListVendorsAdmin`/`CountVendorsAdmin`, tambah `pks_count`; hapus `ListVendorPksSummaries`.
    - _Requirements: 1.3, 3.6, 7.1, 7.2_
  - [x] 3.3 `sqlc generate` (v1.31.1) + perbaikan `UserLeafe` → `UserLeave`; repo read-only baru `pks_list_repository.go` (replica untuk list, primary untuk reminder).
    - _Requirements: 2.1, 4.7_

- [x] 4. Service `backend`
  _Model: Sonnet (4.1, 4.3, 4.4) / Opus (4.2), High — validasi kontrak PKS + deviasi GR#3_
  - [x] 4.1 `vendor_pks_admin.go`: `RenewalDays` di request + payload create; `validateRenewalDays` (wajib, 30–3666, ≤ masa PKS → 422 field `renewal_days`); hapus `renewalMonths` dan `Get`. `masterdata_applier_vendor_pks.go`: insert `renewal_days` (payload lama tanpa field → 270). Aturan create/ganti/terminasi lain tidak berubah.
    - _Requirements: 1.2, 1.3, 1.5, 6.7, 11.2_
  - [x] 4.2 `vendor_pks_admin.go` `UpdateReminder`: role ADMIN/ADMIN_PARAM (else `ErrForbidden`); tx primary `GetVendorPksForUpdate` (404) → `validateRenewalDays` terhadap tanggal baris itu → no-op bila nilainya sama → `UpdateVendorPksRenewalDays` + audit `vendor_pks_reminder_updated` (before/after, ip) di tx yang sama; berlaku juga untuk PKS diganti/terminated.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 11.3_
  - [x] 4.3 `pks_list.go` `PksListService.List`: role ∈ `PksViewerRoles`; `ParsePksListFilter` + cek kode type/vendor ada (422); `today = TodayWIB`; gabung baris pending create (status `menunggu_persetujuan`, tanpa id) untuk `terkini`/`semua`; penanda `pending` per (vendor, type); total.
    - _Requirements: 3.3, 3.4, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8_
  - [x] 4.4 `vendor_admin.go`: list mengembalikan `pks_count`, tanpa filter/field PKS per type, hapus `pksToday`/`renewalMonths`; `List`/`Get` menerima ATM-SPV, method tulis tetap ADMIN/ADMIN_PARAM.
    - _Requirements: 7.1, 7.2, 7.5, 9.2, 9.4_

- [x] 5. Handler, routing, OpenAPI, test `backend`
  _Model: Opus (5.1) / Sonnet (5.2, 5.3), High — perubahan grup RBAC route_
  - [x] 5.1 `admin_pks_handler.go` (GET `/api/v1/admin/pks`, PUT `/{pksID}/reminder` dengan guard ADMIN/ADMIN_PARAM); `admin_vendor_pks_handler.go`: hapus GET, body create + `renewal_days`; `cmd/api/main.go`: grup `vendorRead` (ADMIN, ADMIN_PARAM, ATM-SPV) untuk `AdminVendorHandler` + `AdminPksHandler`, rute tulis di `AdminVendorHandler.Routes()` dibungkus `RequireRoles(ADMIN, ADMIN_PARAM)`, sub-resource vendor tetap di `masterDataAdmin`; hapus env `VENDOR_PKS_RENEWAL_MONTHS` dari kode dan `.env.example`.
    - _Requirements: 1.5, 2.5, 4.8, 6.7, 9.2, 9.4_
  - [x] 5.2 `backend/api/openapi.yaml`: `/admin/pks`, `/admin/pks/{pksID}/reminder`, `renewal_days` di create, GET `/admin/vendors/{vendorID}/pks` dihapus, `pks` → `pks_count` di daftar vendor; `go test ./internal/handler -run OpenAPI` hijau.
    - _Requirements: 1.2, 2.1, 4.1, 7.1_
  - [x] 5.3 Test backend: unit table-driven (`validateRenewalDays` **Property 2**, `UpdateReminder` role/404/422/no-op/audit gagal, `PksListService` filter + pending **Property 5**); integration **Property 3** (hanya `renewal_days` berubah + satu audit; audit gagal = rollback), **Property 4** (`rapid` data acak: list per halaman vs count), **Property 7** (matriks role lewat router nyata: ATM-SPV boleh list/detail vendor + Daftar PKS, 403 untuk tulis, reminder, cabang/PIC/harga/vault/mapping DSR), **Property 8** (`pks_count`), **Property 9** (`remaining_days`); regresi create/ganti/terminasi/unik WOD/satu aktif per (vendor, type); test lama yang memakai `renewal_months`/`pks` per type diperbarui. Hapus simbol lama `pkg/reports` yang disisakan 2.1 (+ test-nya di `vendor_pks_test.go`) setelah `git grep` memastikan tidak ada pemanggil di `backend`/`backend-exq` (dihilangkan oleh 4.1, 4.4, 5.1, 6.2); `go build ./...` ketiga modul hijau.
    - _Requirements: 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 3.5, 4.2, 4.3, 4.4, 4.5, 4.6, 4.8, 6.7, 7.1, 7.2, 9.2, 9.4, 11.2, 11.3_

- [x] 6. Ekspor `backend-exq`
  _Model: Sonnet, High — ekspor harus sama persis dengan layar_
  - [x] 6.1 `queries/vendor_pks_report.sql`: tulis ulang `ExportVendorPksBatch`/`CountVendorPksReport` atas semua baris dengan filter `ListPksAdmin` + `vendor_pks_record_status`, keyset `(v.code, t.code, DATE '9999-12-31' - effective, id)`; hapus query `Missing`; `sqlc generate`.
    - _Requirements: 3.4, 10.1, 10.3_
  - [x] 6.2 `internal/exportjob/params.go` (`report_params` = `{status, pks_type, vendor, q}` via `ParsePksListFilter`, filter lama 422), `producers_report.go` (tanpa `renewalMonths`, kolom baru), guard route + service `PksViewerRoles`, `cmd/api/main.go` + `.env.example` tanpa `VENDOR_PKS_RENEWAL_MONTHS`.
    - _Requirements: 1.5, 10.1, 10.2, 10.3, 10.5_
  - [x] 6.3 Test: unit params (filter lama ditolak); integration ekspor (kolom, `CSVSafe`, tanpa baris pending, ATM-SPV boleh, role lain 403, audit + replica); **Property 6**: data acak, himpunan (id, status, sisa hari) dari `ListPksAdmin` = hasil ekspor untuk filter yang sama.
    - _Requirements: 3.4, 10.1, 10.2, 10.3, 10.5_

- [x] 7. CompanyPortal (`ui-ux-pro-max`, brand Merah Sirih)
  _Model: Sonnet, Medium — komponen dalam pola yang ada_
  - [x] 7.1 `pnpm gen:api`; `src/features/admin-pks/`: `api.ts`, `hooks.ts` (`usePksList`, `useUpdatePksReminder`, mutasi create/terminate/unterminate dipindah), `lib/pks.ts` (`formatMasaBerlaku`), `PksStatusBadge` (6 status + `menunggu_persetujuan`, ikon + teks).
    - _Requirements: 3.5, 5.4_
  - [x] 7.2 `PksTable` (kolom D12, prop `vendorCode`/`actions`, baris klik/Enter → `/settings/admin/vendors/{id}?tab=pks`), `PksFilterBar` (Status default Terkini, Type, Vendor, cari; URL state), `PksListPage` (tombol Catat + Ekspor), route `routes/settings/admin/pks.tsx` (`requireRoles(["ATM-SPV"])`, `validateSearch`), `routeTree.ts`, kartu "Daftar PKS" di `SettingsHubPage` (ATM-SPV hanya Daftar PKS + Manajemen Vendor).
    - _Requirements: 5.1, 5.2, 5.3, 5.5, 5.6, 5.7, 9.3, 10.4_
  - [x] 7.3 Dialog + aksi: `PksFormDialog` (Vendor select vendor aktif FLM, Type aktif, Reminder default 270 wajib; mode Ganti dengan Vendor + Type terkunci), `PksReasonDialog`, `PksReminderDialog` (30–3666, ≤ masa PKS); aksi per baris (Ganti/Terminasi hanya Current_PKS, Ubah reminder semua baris, nonaktif + alasan saat pending), kontrol tulis tidak dirender untuk ATM-SPV.
    - _Requirements: 1.4, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_
  - [x] 7.4 `features/admin-vendors/`: `VendorsTable` kolom PKS ("N PKS" / "Belum ada" link `?tab=pks`, INTERNAL "-"); `VendorFilterBar` tanpa filter PKS; `AdminVendorsPage` tanpa ekspor PKS dan tanpa kontrol tulis untuk ATM-SPV; `VendorDetailPage` tab dari `?tab=` (ATM-SPV hanya tab PKS, tanpa `SummaryStrip`/kontrol tulis); `VendorPksTab` (PksTable read-only + "Kelola di Daftar PKS" hanya ADMIN/ADMIN_PARAM, tanpa teks 9 bulan); `vendors.tsx`/`vendor-detail.tsx` `requireRoles` + ATM-SPV; hapus file PKS lama + `types.ts` `pks` → `pks_count`.
    - _Requirements: 7.3, 7.4, 7.5, 8.1, 8.2, 8.3, 8.4, 9.3, 10.4_
  - [x] 7.5 Test vitest + RTL: PksTable (kolom, badge, keyboard, klik baris), PksFilterBar (default Terkini, URL), PksFormDialog (default 270, wajib, Ganti terkunci), PksReminderDialog, aksi tersembunyi/nonaktif (ATM-SPV, pending), `formatMasaBerlaku` (**Property 9**), kolom PKS VendorsTable, VendorDetailPage untuk ATM-SPV, VendorPksTab read-only, SettingsHubPage per role; test lama (`VendorListPks`, `PksPanel`) diperbarui/dihapus; schema stale-check.
    - _Requirements: 1.4, 3.5, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 6.1, 6.3, 6.5, 6.6, 7.3, 7.4, 8.1, 8.2, 8.3, 8.4, 9.3, 10.4_

- [x] 8. Checkpoint gate kualitas
  _Model: Sonnet, Medium — menjalankan gate dan memperbaiki sampai hijau_
  - `backend`, `backend-exq`, `pkg`: `go test ./...`, `go test -tags integration` (DB dev), `golangci-lint run --build-tags integration ./...`; contract test OpenAPI; coverage `internal/*` ≥ 80%. CompanyPortal: `pnpm lint`, `test`, `tsc --noEmit`, `build`. Regresi: layar + API Type PKS tidak berubah; CSV vendor ekspor/impor tidak berubah; tidak ada UPDATE kolom isi PKS selain `renewal_days`. Hasil dicatat di `### Test Results`.
  - _Requirements: 11.1, 11.2, 11.3, 11.4_

- [x] 9. Dokumentasi
  _Model: Haiku, Low — teks_
  - CLAUDE.md Sec 3 (migrasi `044`, `renewal_days`, fungsi status baru, menu `settings.pks`, ATM-SPV) + Sec 12 (deviasi GR#3 ubah reminder, nomor migrasi berikutnya); `.claude/docs/data-map.md`, `.claude/docs/decisions.md`; `development-progress.md`.
  - _Requirements: 1.5, 2.4_

- [x] 10. Migrasi pembersih `045` (setelah rilis ini berjalan di semua lingkungan)
  _Model: Opus, Medium — drop fungsi DB; pastikan tidak ada pemanggil_
  - `git grep` memastikan `vendor_pks_status(` dan `vendor_pks_validity_months(` tidak dipanggil di `backend`, `backend-exq`, `backend_python`; tulis `045` (`DROP FUNCTION`), `migrate up` dengan OK user.
  - _Requirements: 3.6_

- [ ] 11. Cek browser manual (user)
  _Model: Haiku, Low — dilakukan user (Golden Rule #10)_
  - Daftar PKS (filter, Terkini/Diganti/Semua, Catat/Ganti/Terminasi/Ubah reminder, pending, ekspor), daftar vendor kolom PKS, tab PKS read-only, login ATM-SPV (hanya melihat), keyboard. Dicatat "outstanding (user)" sampai user melaporkan.
  - _Requirements: 5.1, 5.7, 6.1, 9.3_

## Notes

### Keputusan yang tercermin
- D2 (Type PKS tetap): task 8 (regresi). D3/D13 (riwayat + filter): 3.1, 4.3, 7.2. D4/D5 (belum berlaku, prioritas): 1.1, 1.4. D6–D8 (reminder hari, batas, backfill): 1.1, 4.1, 7.3. D9 (ubah reminder langsung, semua PKS, audit di tx): 4.2, 5.1, 7.3. D10 (env dihapus): 2.1, 5.1, 6.2. D11/D12 (Masa Berlaku, kolom): 2.1, 7.1, 7.2. D14–D17 (aksi hanya di Daftar PKS, form vendor, Ganti, pending): 7.2, 7.3, 7.4. D18/D24 (halaman + menu): 1.1, 7.2. D19 (kolom PKS vendor): 3.2, 4.4, 7.4. D20 (tab read-only): 7.4. D21–D23 (akses + klik baris): 1.1, 4.4, 5.1, 7.2, 7.4. D25 (ekspor): 6.x, 7.2. D26 (input 12 PKS sekarang): task 10 `vendor-pks-type`, bukan di sini.
- A1–A5: 2.1, 4.3, 6.x, 3.1, 5.2/7.4.

### Di luar spec ini
- Impor CSV PKS, notifikasi Need Renewal, akses ATM-SPV ke tab vendor lain.

### Deploy
- `044` (`migrate up`, di luar jam kantor) → `backend` + `backend-exq` + CompanyPortal **dalam satu rilis** → hapus `VENDOR_PKS_RENEWAL_MONTHS` dari env → task 10 (`045`) setelah rilis stabil.

### Jendela build merah (user 2026-11-19, "Accept red window")
- `backend` boleh gagal compile dari 3.3 sampai 5.1/5.3 (pemanggil query yang dihapus: repository dibersihkan 3.3; service 4.1/4.4; handler 5.1; test lama 5.3). `backend-exq` boleh gagal dari 6.1 sampai 6.2 (kode) / 6.3 (test).
- Selama jendela ini tiap task hanya memverifikasi paket miliknya sendiri dan melaporkan error compile di luar filenya sebagai "diharapkan, diperbaiki task X". Build penuh hijau wajib setelah wave 6, dicek lagi di task 8.

### Definition of Done
- CLAUDE.md Sec 11: RBAC di route + service (Req 9), deviasi GR#3 dengan audit di tx (Req 2), replica untuk baca / primary untuk tulis (Req 4.7, 2.1), OpenAPI + `gen:api`, test hijau termasuk RBAC, tanpa env baru (satu env dihapus).

### Test Results
- 1.3 (2026-11-19): `migrate status` sebelum = 42 applied, 1 pending (`044`), semua checksum ok → OK user di sesi ini → `go run ./cmd/migrate up` = "applying 044_vendor_pks_list.sql / 1 applied" → `status` = **43 applied, 0 pending**, `044` checksum ok. Guard masa < 30 hari tidak terpicu.
- 5.3 selesai (2026-11-19): `backend` `go vet` (+ `-tags integration`) hijau; `go test ./...` hijau termasuk OpenAPI contract (5.2); `go test -tags integration ./internal/service ./internal/repository ./internal/handler` hijau di dev (localhost). Test baru: `service/pks_list_test.go` (PksListService, Property 2, 5, role guard UpdateReminder), `service/vendor_pks_list_integration_test.go` (Property 3, 4, 8, 9, 422/404/no-op/audit gagal rollback, regresi create/ganti/terminasi), `handler/admin_pks_handler_test.go` (GET /admin/pks, PUT reminder, Property 7 matriks role di grup vendorRead). Simbol lama `pkg/reports` dihapus (kecuali `VendorPks`, `TodayWIB`, `VendorPksFileName`). `backend-exq` di luar 5.3 (6.3). Catatan: payload create pra-044 dengan masa < 270 hari gagal apply (CHECK) karena default 270 — pertimbangkan clamp di applier. Sub-resource vendor tetap di grup ADMIN/ADMIN_PARAM di main.go; tidak ada test router nyata, matriks meniru mount `vendorRead`.

- 8 (2026-11-19): `sqlc generate` ulang `backend` + `backend-exq` (v1.31.1, `UserLeafe`→`UserLeave` diperbaiki di keduanya); komentar basi `VENDOR_PKS_RENEWAL_MONTHS` hilang (sumber `queries/vendor_pks_report.sql` sudah bersih, file generated yang basi). `backend`/`backend-exq`/`pkg`: `go vet` (+`-tags integration`) hijau, `go test ./...` + `-tags integration` (DB dev localhost) hijau, `golangci-lint run --build-tags integration` 0 issues, contract OpenAPI hijau di kedua backend. Coverage `internal/*`: backend handler 82.8, service 84.0, audit 81.2, approval 90.4, notification 83.9; **`backend/internal/repository` 63.6% (< 80, sudah ada sebelum spec ini, tidak disentuh)**; exq exportjob 94.0, handler 98.3. CompanyPortal: biome lint, vitest 211 file/2014 test, `tsc --noEmit`, `build` hijau. Regresi: tidak ada diff Type PKS/csvexport; satu-satunya UPDATE isi PKS baru = `renewal_days`. Cek browser manual: outstanding (user).
- 10 (2026-11-19): `git grep` di `backend`, `backend-exq`, `backend_python`, `pkg`, `frontend`: satu-satunya pemanggil = `backend/internal/repository/vendor_pks_sql_integration_test.go` (property test fungsi lama, spec `vendor-pks`) → **dihapus** dengan OK user; DB dev: tidak ada view/fungsi/constraint yang bergantung, `vendor_pks_record_status` tidak memanggilnya. `045_drop_old_vendor_pks_functions.sql` (DROP signature persis dari `022`, tanpa IF EXISTS/CASCADE) + `backend/scripts/dev/rollback_045_drop_old_vendor_pks_functions.sql` (definisi `022` verbatim, belum pernah dijalankan). `status` sebelum = 43 applied, 1 pending → OK user di sesi ini → `migrate up` = 1 applied → `status` = **44 applied, 0 pending**, checksum ok; `pg_proc` fungsi lama = 0, `vendor_pks_record_status` = 1. `sqlc compile` (v1.31.1) OK di `backend` + `backend-exq`. `TestMigration022Schema` kini mengharapkan kedua fungsi sudah di-drop. `backend`: `go vet -tags integration ./...` hijau, `go test ./...` hijau, `go test -tags integration ./internal/repository ./internal/service ./internal/handler` hijau (dev localhost); `backend-exq`: `go test -tags integration ./internal/exportjob ./internal/handler` hijau.

### Catatan eksekusi
- 1.1 (2026-11-19): `044_vendor_pks_list.sql` ditulis, **belum diterapkan** (task 1.3, butuh OK user). Menu `settings.pks` `sort_order` 2 (setelah `settings.roles` 0, `settings.acm-areas` 1). Backfill hanya meng-UPDATE baris yang nilainya bukan 270. Batas jendela deploy: binary lama yang menyetujui PKS bermasa < 270 hari setelah migrasi ditolak CHECK `vendor_pks_renewal_days_le_term_chk` (DEFAULT 270). Cek: `sqlc compile` (v1.31.1) OK; `go test ./cmd/migrate/... ./internal/repository/ -run Migrat|Hard` OK. Eksekusi SQL ke DB belum (1.3/1.4).
- 4.2 (2026-11-19): `UpdateReminder(ctx, actorID, role, pksID, days int32, ip) (db.UpdateVendorPksRenewalDaysRow, error)`. Beda dari design: tipe `VendorPksRecord` sudah dihapus 4.1, jadi yang dikembalikan baris sqlc (JSON `effective_date`/`expiry_date` = `YYYY-MM-DD`, timestamp RFC3339/null); handler 5.1 membungkusnya `{pks: …}`. `days` int32 (sama dengan `validateRenewalDays`). Error: role ≠ ADMIN/ADMIN_PARAM → `ErrMasterDataForbidden` (seperti 4.3; `ErrForbidden` tidak ada), tidak ada → `ErrVendorPksNotFound`, 422 → `*ValidationError` field `renewal_days`. **Constructor berubah**: `NewVendorPksAdminService(repo, changes, pool)` (pool primary, `VendorPicRolePool`) — pemanggil `cmd/api/main.go` diperbaiki 5.1, `vendor_pks_admin_test.go` + `masterdata_applier_vendor_pks_integration_test.go` diperbaiki 5.3. Cek: `go build ./internal/service/` OK; unit test reminder (design: role, 404, 422, no-op, audit gagal → rollback) belum ditulis — milik 5.3/integrasi (paket test masih merah).
- 5.1 (2026-11-19): `main.go` grup `vendorRead` = RequireAuth + `RequireRoles(reports.PksViewerRoles...)`; hanya `/api/v1/admin/vendors` (AdminVendorHandler) dan `/api/v1/admin/pks` (AdminPksHandler) di-mount di sana. `AdminVendorHandler.Routes()`: GET `/` + `/{id}` terbuka untuk grup, POST/PUT/disable/enable lewat `r.With(RequireRoles("ADMIN","ADMIN_PARAM"))`; handler meneruskan `authCtx.Role` ke `List/Count/Get/GetKindAndDirector`, `ErrVendorReadForbidden` → 403; daftar vendor `pks` → `pks_count`, query `pks_type`/`pks_status` tidak dibaca lagi (diabaikan, bukan 400). `AdminPksHandler`: GET `/` (page/page_size bukan angka → 400, kosong → default service), PUT `/{pksID}/reminder` guard ADMIN/ADMIN_PARAM di route → 200 `{pks: <baris sqlc>}`; error lewat `handleVendorPksError` (403/404/422). Semua sub-resource vendor (cabang, PIC, harga, paket, vault, mapping DSR, tulis PKS) tetap di `masterDataAdmin`. `NewVendorPksAdminService(..., dbPool)`; `PksListService` repo replica, type repo dipakai bersama `PksTypeAdminService`, vendor finder = `vendorAdminRepo`. Env `VENDOR_PKS_RENEWAL_MONTHS` dihapus dari `main.go` + `backend/.env.example`. Cek: `go build ./...` (`backend`) **OK**; `go vet ./...` gagal compile test seperti diharapkan → 5.3: `service/vendor_admin_test.go` (`db.ListVendorPksSummariesParams`), `handler/admin_vendor_handler_test.go` (`service.VendorPksSummary`, fake servicer tanpa `role`), plus test PKS handler/service lama. Contract test OpenAPI belum dijalankan (5.2). Catatan untuk 5.3: `backend/internal/db/vendor_pks_report.sql.go` masih memuat `RenewalMonths` (file sqlc `backend`, bukan bagian 5.1).

## Task Dependency Graph

- **Wave 0:** 1.1 (migrasi), 2.1 (kontrak `pkg/reports`) — tanpa dependensi.
- **Wave 1:** 1.2, 1.3 (butuh 1.1; 1.3 butuh OK user), 2.2 (butuh 2.1), 3.1, 3.2 (butuh 1.1).
- **Wave 2:** 3.3 (butuh 3.1, 3.2), 6.1 (butuh 1.1, 2.1).
- **Wave 3:** 1.4 (butuh 1.3, 2.1), 4.1, 4.3, 4.4 (butuh 3.3), 6.2 (butuh 6.1).
- **Wave 4:** 4.2 (butuh 4.1, file sama), 6.3 (butuh 6.2, 1.3).
- **Wave 5:** 5.1 (butuh 4.1–4.4).
- **Wave 6:** 5.2, 5.3 (butuh 5.1; 5.3 juga butuh 1.3).
- **Wave 7:** 7.1 (butuh 5.2 untuk `gen:api`).
- **Wave 8:** 7.2 (butuh 7.1).
- **Wave 9:** 7.3, 7.4 (butuh 7.2; file berbeda).
- **Wave 10:** 7.5 (butuh 7.3, 7.4).
- **Wave 11:** 8 (butuh semua task 1–7).
- **Wave 12:** 9 (butuh 8).
- **Wave 13:** 10 (butuh rilis), 11 (butuh 9).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1"] },
    { "id": 1, "tasks": ["1.2", "1.3", "2.2", "3.1", "3.2"] },
    { "id": 2, "tasks": ["3.3", "6.1"] },
    { "id": 3, "tasks": ["1.4", "4.1", "4.3", "4.4", "6.2"] },
    { "id": 4, "tasks": ["4.2", "6.3"] },
    { "id": 5, "tasks": ["5.1"] },
    { "id": 6, "tasks": ["5.2", "5.3"] },
    { "id": 7, "tasks": ["7.1"] },
    { "id": 8, "tasks": ["7.2"] },
    { "id": 9, "tasks": ["7.3", "7.4"] },
    { "id": 10, "tasks": ["7.5"] },
    { "id": 11, "tasks": ["8"] },
    { "id": 12, "tasks": ["9"] },
    { "id": 13, "tasks": ["10", "11"] }
  ]
}
```
