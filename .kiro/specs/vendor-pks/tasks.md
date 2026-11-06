# Implementation Plan: PKS vendor

## Overview

Urutan implementasi mengikuti lapisan: skema dulu (migrasi `022` untuk PKS + penanda direktur di `vendor_pics` + fungsi status, `037` untuk kind ekspor + constraint roles direktur), lalu kontrak bersama di `pkg/` (`pkg/reports`, `pkg/csvexport`), query + sqlc di dua modul, lalu backend (Direktur Utama sebagai PIC → daftar vendor dengan status PKS → service + applier PKS → handler + OpenAPI), ekspor di `backend-exq`, dan terakhir CompanyPortal. Checkpoint gate kualitas, dokumentasi, dan cek browser manual menutup spec.

Revisi 2026-11-05 (D32): Direktur Utama dicatat sebagai PIC tingkat vendor bertanda `vendor_pics.is_director` (hanya pencatatan: tanpa login, tanpa roles, bukan penerima notifikasi). Tabel `vendors`, form vendor, dan CSV vendor tidak berubah.

Pola yang dirujuk:
- Entitas master data + applier: `backend/internal/service/masterdata_applier_vendor_pic.go`, `masterdata_applier_dsr_location_map.go`, registry di `backend/cmd/api/main.go`.
- PIC: `backend/internal/service/vendor_pic_admin.go` (validate + Submit), `vendor_pic_roles.go` (ubah roles langsung + audit).
- Legacy header CSV: `pkg/csvexport/entities.go` (`VendorPICs.LegacyHeaders`, `VendorBranches` + `KeepCurrentCell`).
- Report kind ekspor: `backend-exq/internal/exportjob/report_params.go` (DSR telat), migrasi `035_export_jobs_reports.sql`.
- Kontrak OpenAPI: `backend/api/openapi.yaml` + `backend/internal/handler/openapi_contract_test.go`.

### Model Split

**Sonnet-runnable**
- 1.3 Integration test migrasi `022` + `037` — Sonnet, Medium (needs 1.2)
- 1.4 Property test fungsi status + masa berlaku (Property 1, 2) — Sonnet, High (needs 1.2)
- 2. Migrasi `037` (kind ekspor + constraint roles direktur) — Sonnet, Medium
- 3. `pkg/reports` + `pkg/csvexport` — Sonnet, Medium
- 4. Query + sqlc dua modul — Sonnet, Medium (needs 1.1, 2.1)
- 5. Direktur Utama sebagai PIC (service, applier, roles guard, handler, CSV) — Sonnet, High (needs 3, 4, 7.3)
- 6. Daftar vendor dengan status PKS + direktur — Sonnet, Medium (needs 4, 7.2 untuk urutan `main.go`)
- 7.4 Unit test `VendorPksAdminService` — Sonnet, Medium (needs 7.1)
- 8. Handler PKS + OpenAPI + contract test — Sonnet, Medium (needs 6.1, 7)
- 9. Ekspor `vendor_pks` di `backend-exq` — Sonnet, Medium (needs 2, 3, 4)
- 10. Frontend tipe, API, hooks, helper PKS — Sonnet, Medium (needs 5.3, 8.2, 9.2)
- 11. Frontend layar vendor, PIC, approval, ekspor — Sonnet, Medium (needs 10)
- 12. Checkpoint gate kualitas — Sonnet, Medium (needs semua task 1–11)
- 13. Dokumentasi — Haiku, Low (Haiku) (needs 12)
- 14. Cek browser manual (user) — Haiku, Low (Haiku) (needs 13)

**Opus-required**
- 1.1, 1.2 Migrasi `022` (tabel `vendor_pks`, `vendor_pics.is_director` + constraint, fungsi status, index pending) + apply ke dev — Opus, High — migrasi skema pada DB berisi data + aturan status yang dipakai semua bacaan
- 7.1, 7.2, 7.3, 7.5 Service submit PKS, `VendorPksApplier`, cabang stale di `applyOne`, integration test konkurensi — Opus, High — maker-checker master data, satu PKS aktif di bawah approve paralel, perubahan jalur apply bersama

**Dependensi lintas grup**
- Sonnet 1.3, 1.4, 4 butuh Opus 1.1 (dan 1.2 untuk test DB).
- Opus 7.1 butuh Sonnet 4.4 (sqlc). Sonnet 7.4 butuh Opus 7.1.
- Sonnet 5.2 butuh Opus 7.3 (cabang stale di `applyOne` dipakai applier PIC untuk direktur kedua).
- Sonnet 6.1 dan 8.1 menyentuh `backend/cmd/api/main.go` setelah Opus 7.2 (registry applier); diserialkan.
- Sonnet 8 dan 12 butuh semua sub-task Opus 7 selesai.

## Tasks

- [ ] 1. Migrasi `022_vendor_pks.sql`: tabel PKS, penanda direktur, fungsi status
  - [x] 1.1 Tulis `backend/migrations/022_vendor_pks.sql`
    - Header komentar (spec `.kiro/specs/vendor-pks/`, aditif, forward-only) + `BEGIN;` / `COMMIT;` + blok `-- ROLLBACK (documented, forward-only shop):` seperti `003`/`023`.
    - `vendor_pics.is_director boolean NOT NULL DEFAULT false`, `vendor_pics_director_chk` (`NOT is_director OR (vendor_branch_id IS NULL AND is_notification_recipient = false)`), `vendor_pics_director_uq (vendor_id) WHERE is_director AND deleted_at IS NULL`, `COMMENT ON COLUMN`. Tidak menyentuh `vendors` dan tidak merujuk `roles` (kolom itu dibuat `032`).
    - Tabel `vendor_pks` sesuai design (`## Data Models`): FK `vendors`/`users`, `vendor_pks_dates_chk`, `vendor_pks_terminate_chk`, CHECK `replaced_by_pks_id`, partial unique `vendor_pks_active_uq (vendor_id) WHERE replaced_at IS NULL`, index `vendor_pks_vendor_idx`, trigger `set_updated_at`, `COMMENT ON`.
    - Fungsi `vendor_pks_status(...)` (`LANGUAGE sql IMMUTABLE`) dan `vendor_pks_validity_months(...)` (`plpgsql IMMUTABLE`, koreksi ±1 terhadap `p_effective + make_interval(months => n) <= p_expiry + 1`).
    - Index `master_data_change_requests_vendor_pks_pending_uq` pada `((payload->>'vendor_id')::bigint) WHERE entity_type = 'vendor_pks' AND status = 'pending'`.
    - Tanpa `parent_pks_id` (D3), tanpa data seed.
    - _Requirements: 1.9, 3.6, 4.1, 4.2, 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 7.1, 11.5, 12.1_
  - [x] 1.2 Terapkan ke DB dev
    - Sebelum apply: hitung change PIC `pending` (`SELECT count(*) FROM master_data_change_requests WHERE entity_type = 'vendor_pic' AND status = 'pending'`) dan catat di `## Notes`; change `update`/`disable`/`enable` itu akan `stale` saat di-approve (snapshot PIC mendapat key `is_director`).
    - `cd backend && go run ./cmd/migrate up` (menerapkan `022` dan `037` bila 2.1 sudah ada), lalu `go run ./cmd/migrate status` tanpa MISMATCH. Jangan `psql -f`.
    - _Requirements: 12.1_
  - [x] 1.3 Integration test migrasi `022` + `037`
    - `backend/internal/repository/migration022_integration_test.go` (`//go:build integration`): tabel, kolom, CHECK, index, fungsi ada; `vendors` tidak punya kolom baru; baris `vendor_pics` lama `is_director = false`; direktur dengan cabang, `is_notification_recipient = true`, atau `roles` tidak kosong ditolak; dua direktur aktif satu vendor ditolak, direktur nonaktif + direktur aktif boleh; `expiry < effective` ditolak; dua PKS aktif satu vendor ditolak; dua change `vendor_pks` pending untuk `vendor_id` sama ditolak.
    - _Requirements: 1.9, 4.1, 12.1_
    - _Model: Sonnet, Medium — test skema dengan pola integration yang ada._
  - [x] 1.4 Property test fungsi SQL (Property 1, Property 2)
    - `backend/internal/repository/vendor_pks_sql_integration_test.go` dengan `pgregory.net/rapid`: tanggal acak 2000–2100 termasuk akhir bulan dan 29 Februari, `renewal_months` 1–24, `kind` dua nilai, PKS ada/tidak, terminated/tidak.
    - Model referensi Go `addMonthsClamped` + aturan Req 6.2 di file test. Kasus tetap: `2026-05-31 + 9 bulan = 2027-02-28`, hari expired = bukan `expired`, `2026-01-01 → 2027-12-31 = 24 bulan`.
    - **Property 1: Aturan status PKS**, **Property 2: Masa berlaku dalam bulan penuh**.
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 7.1_
    - _Model: Sonnet, High — banyak kasus batas tanggal, tapi tidak menyentuh data produksi._
  - _Model: Opus, High — migrasi skema pada DB berisi data + fungsi status yang dipakai list, filter, detail, dan ekspor._

- [ ] 2. Migrasi `037_export_jobs_vendor_pks.sql`
  - [x] 2.1 Tulis `backend/migrations/037_export_jobs_vendor_pks.sql`
    - Drop + add `export_jobs_kind_chk`, `export_jobs_params_kind_chk`, `export_jobs_scope_chk` dari `035` dengan `'vendor_pks'` di cabang kind laporan (`status_filter IS NULL`, `report_params` objek, tanpa `vendor_id`/`vendor_branch_id`). `COMMENT ON COLUMN report_params` menyebut filter `q, status, pks_status`.
    - `vendor_pics_director_roles_chk`: `NOT is_director OR roles = '{}'::text[]` (butuh `roles` dari `032`). Header komentar menyebut kedua isi.
    - _Requirements: 1.9, 10.1, 12.1_
  - [x] 2.2 Integration test migrasi `037` (diterapkan di 1.2)
    - `backend-exq/internal/exportjob/migration037_integration_test.go`: kind `vendor_pks` dengan `report_params` objek diterima; dengan `status_filter`, `vendor_id`, atau `report_params` NULL ditolak; kasus valid dari `migration035_integration_test.go` tetap lolos.
    - _Requirements: 12.1, 12.2_
  - _Model: Sonnet, Medium — mengikuti pola `035` persis; constraint saja, tanpa perubahan data._

- [ ] 3. Kontrak bersama di `pkg/`
  - [x] 3.1 `pkg/reports`: konstanta dan helper PKS
    - File baru `pkg/reports/vendor_pks.go`: `VendorPks = "vendor_pks"`, `VendorPksViewerRoles`, `PksStatuses` (kunci → label CSV), `VendorPksRenewalMonthsDefault = 9`, `ParseVendorPksRenewalMonths` (kosong → 9, selain 1–24 → error), `TodayWIB(now)`, `VendorPksHeader`, `VendorPksRow`, `VendorPksCells` (nama/telepon/email direktur, teks lewat `csvexport.CSVSafe`, angka bulat, sel kosong bila tanpa PKS/direktur, label Indonesia dari `PksStatuses`), nama file `vendor-pks-YYYYMMDD.csv`.
    - _Requirements: 6.1, 6.7, 10.3_
  - [x] 3.2 `pkg/csvexport`: kolom `is_director` di CSV PIC
    - `VendorPICs.Header` + `is_director` di akhir; `LegacyHeaders` = header 10 kolom saat ini + header 9 kolom lama; `LegacyFill` = `{roles: "", is_director: KeepCurrentCell}`.
    - `VendorPICRow` + `IsDirector bool`; cell `true`/`false`. `Vendors` tidak berubah.
    - _Requirements: 2.1, 2.3, 2.4_
  - [x] 3.3 Unit test `pkg/`
    - `pkg/reports`: `ParseVendorPksRenewalMonths` (kosong, 0, 25, "abc", 9), `TodayWIB` di batas 16:59/17:00 UTC, golden CSV `VendorPksCells` (non-ASCII, sel diawali `=` diberi apostrof, vendor tanpa PKS/direktur).
    - `pkg/csvexport`: kontrak header PIC + dua legacy header, header vendor tetap, `Lookup` tidak membocorkan slice internal.
    - _Requirements: 2.1, 2.3, 2.4, 6.7, 10.3_
  - _Model: Sonnet, Medium — kontrak kecil dengan pola yang sudah ada (DSR telat, legacy header PIC)._

- [ ] 4. Query dan sqlc
  - [x] 4.1 `backend/queries/vendors_admin.sql`
    - `GetVendorAdminByID`, `CreateVendorAdmin`, `UpdateVendorAdmin` tidak berubah (DD11).
    - `GetVendorDirectorPic :one` (baru): `id, name, phone, email` Director_PIC aktif satu vendor.
    - `ListVendorsAdmin` + `CountVendorsAdmin`: `LEFT JOIN vendor_pks ap ... replaced_at IS NULL`, `LEFT JOIN vendor_pics dp ... is_director AND deleted_at IS NULL`, kolom `v.kind`, direktur, ringkasan PKS aktif, `pks_validity_months`, `pks_remaining_days`, `pks_status` (cast `::text`/`::int`), param `today`, `renewal_months`, filter `sqlc.narg('pks_status')`. Predicate `q`/`status` tidak berubah.
    - _Requirements: 1.10, 8.1, 8.2, 8.4, 8.5_
  - [x] 4.2 `backend/queries/vendor_pks.sql` (baru) + `vendor_pics_admin.sql`
    - `vendor_pks.sql`: `GetVendorPksVendor`, `LockVendorForPks` (`FOR UPDATE`), `GetActiveVendorPks`, `GetVendorPksByID`, `ListVendorPksByVendor` (+ `vendor_pks_validity_months`, urut aktif lalu riwayat terbaru), `GetPendingVendorPksChange` (+ nama maker), `MarkVendorPksReplaced`, `InsertVendorPks`, `SetVendorPksReplacedBy`, `TerminateVendorPks`, `UnterminateVendorPks` (guard `replaced_at IS NULL` + status terminate). Tidak ada query yang meng-UPDATE `pks_name`, `effective_date`, atau `expiry_date`.
    - `vendor_pics_admin.sql`: `is_director` di `ListVendorPicsAdmin`, `GetVendorPicAdminByID`, `CreateVendorPicAdmin`, `UpdateVendorPicAdmin`; `GetVendorPicRolesForUpdate` mengembalikan `roles, is_director`; `FindOtherActiveDirector` (baru).
    - _Requirements: 1.5, 1.6, 1.10, 3.7, 4.2, 4.3, 4.4, 5.3, 5.5_
  - [x] 4.3 Query ekspor
    - `backend/queries/vendor_pks_report.sql` (baru): `ExportVendorPksBatch` (join direktur, keyset `(name, id)`, filter `q`/`status` disalin dari `ListVendorsAdmin`, `pks_status`) dan `CountVendorPksReport`.
    - `backend/queries/master_data_export.sql`: `ExportVendorPicsBatch` + `is_director`; `ExportVendorsBatch` tidak berubah.
    - _Requirements: 2.1, 10.2, 10.3_
  - [x] 4.4 Generate sqlc di dua modul
    - `cd backend && sqlc generate` (v1.31.1), perbaiki `UserLeafe → UserLeave`; `backend-exq/sqlc.yaml` + `../backend/queries/vendor_pks_report.sql`, lalu `cd backend-exq && sqlc generate`. `go build ./...` di kedua modul (perbaiki pemakai struct yang berubah tanpa mengubah perilaku).
    - _Requirements: 12.2_
  - _Model: Sonnet, Medium — SQL-first dengan pola query yang ada; filter status memanggil fungsi dari 1.1._

- [ ] 5. Direktur Utama sebagai PIC
  - [x] 5.1 `VendorPicAdminService`
    - `VendorPicUpdatePayload` + `IsDirector *bool`. `validate` memakai flag efektif (request atau `before`): direktur + cabang → 422 `vendor_branch_id`; direktur + `is_notification_recipient` → 422; direktur + roles tidak kosong → 422 `roles`; set `Roles = &[]string{}` untuk direktur; `FindOtherActiveDirector` → `ErrVendorDirectorExists` (409). `Enable` direktur nonaktif → cek direktur lain (409). Update `is_director = false` → flag dilepas, roles tetap kosong.
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.7_
  - [x] 5.2 `VendorPicApplier`
    - `create`: teruskan `IsDirector`; roles dari payload (kosong untuk direktur). `update`: `IsDirector` nil → nilai sekarang; menjadi direktur → `SetVendorPicRoles([])` di tx yang sama. Unique/check violation constraint direktur → `ErrMasterDataChangeStale` (pakai cabang 7.3).
    - _Requirements: 1.4, 1.5, 1.7, 11.3_
  - [x] 5.3 `VendorPicRoleService` + `AdminVendorPicHandler`
    - `SetRoles`: direktur + set tidak kosong → `ErrVendorDirectorNoRoles` (409), tanpa tulis dan audit.
    - Handler PIC: body create/update + `is_director` (`*bool`); response list/detail + `is_director`; error baru → 409 dengan pesan Indonesia.
    - _Requirements: 1.1, 1.6, 1.8, 1.10_
  - [x] 5.4 CSV PIC di `backend/`
    - `masterdata_export.go`: kolom `is_director` dari `ExportVendorPicsBatch`.
    - `masterdata_import_validate.go` + `masterdata_import_confirm.go`: parse `is_director`; `KeepCurrentCell` → nil; baris direktur divalidasi seperti 5.1 di dry-run (termasuk dua direktur dalam satu file dan direktur lain yang sudah aktif). Tidak ada entitas PKS di import; header vendor tidak berubah.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  - [x] 5.5 Test direktur (Property 9, Property 11)
    - `vendor_pic_admin_test.go`, `vendor_pic_roles_test.go`, handler PIC test: setiap aturan 5.1/5.3.
    - `vendor_pic_director_integration_test.go` (`rapid`): urutan acak create/update/disable/enable/roles/CSV termasuk dua approve paralel yang sama-sama menjadikan direktur → satu `applied`, satu `stale`; satu direktur aktif per vendor; direktur selalu tingkat vendor, roles kosong, bukan penerima notifikasi; `ListVendorNotificationPicEmails` dan `ListVendorBranchNotificationPicEmails` tidak mengembalikan direktur; jumlah baris `users` tidak berubah.
    - `masterdata_import_test.go` + `masterdata_export_test.go`: header PIC baru, dua legacy header (flag tetap), round-trip export → import = 0 perubahan, header vendor tetap.
    - `backend_python/dsr_late_notify/test_job_integration.py`: PIC direktur tidak masuk daftar penerima.
    - Test PIC dan vendor yang sudah ada tetap hijau.
    - **Property 9: Invarian Direktur Utama**, **Property 11: CSV PIC round-trip dengan penanda direktur**.
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 2.1, 2.2, 2.3, 2.4, 12.2_
  - _Model: Sonnet, High — perluasan alur PIC yang ada dengan aturan yang menyentuh routing notifikasi; banyak kasus batas tapi pola sudah jelas._

- [ ] 6. Daftar vendor dengan status PKS dan direktur
  - [x] 6.1 Konfigurasi ambang dan jam
    - `backend/cmd/api/main.go`: baca `VENDOR_PKS_RENEWAL_MONTHS` lewat `reports.ParseVendorPksRenewalMonths`, `exit 1` bila tidak valid; teruskan ke `NewVendorAdminService` (dan simpan untuk 8.1). `VendorAdminService` memegang `renewalMonths` + `now func() time.Time`.
    - `backend/.env.example`: `VENDOR_PKS_RENEWAL_MONTHS=9` + catatan harus sama dengan `backend-exq`.
    - _Requirements: 6.1, 6.7_
  - [x] 6.2 List, Count, dan detail
    - Service: `List`/`Count` mengisi `Today = reports.TodayWIB(now())`, `RenewalMonths`, `PksStatus`.
    - Handler: parse `pks_status` (lima nilai; lain → 400 `bad_request`); response list + `kind`, `director`, `pks_status`, `pks`; detail + `kind`, `director` dari `GetVendorDirectorPic` (primary). List tetap dari replica.
    - _Requirements: 1.10, 6.6, 7.6, 8.1, 8.2, 8.3, 8.6_
  - [x] 6.3 Test daftar (Property 4 bagian daftar)
    - Integration `vendor_admin_list_pks_integration_test.go`: data vendor + PKS + direktur acak (`rapid`), `today` tetap; hasil dengan filter = hasil tanpa filter yang disaring `pks_status`; `total` = jumlah baris terfilter di semua halaman; vendor internal tidak cocok filter mana pun; `q` nama tetap bekerja; direktur nonaktif tidak tampil.
    - Handler test: `pks_status=xyz` → 400; response memuat `director`, `pks_status`, `pks` sesuai baris. Test vendor yang ada tetap hijau.
    - **Property 4: Filter, daftar, dan ekspor memakai status yang sama** (bagian daftar).
    - _Requirements: 1.10, 6.6, 6.8, 7.6, 8.1, 8.2, 8.3, 8.4, 8.5, 12.2_
  - _Model: Sonnet, Medium — perluasan query/handler list dengan parameter baru._

- [ ] 7. Service dan applier PKS (maker-checker)
  - [x] 7.1 `VendorPksAdminService` + repository
    - `backend/internal/repository/vendor_pks_admin_repository.go` (baru, baca primary) dan `backend/internal/service/vendor_pks_admin.go` (baru): `Get`, `SubmitNew`, `SubmitTerminate`, `SubmitUnterminate` dengan urutan cek di design; error domain `ErrVendorPksNotFound`, `ErrVendorPksInternalVendor`, `ErrVendorPksVendorInactive`, `ErrVendorPksNotActive`, `ErrVendorPksAlreadyTerminated`, `ErrVendorPksNotTerminated`.
    - Submit lewat `MasterDataChangeService.Submit` (`entity_type = vendor_pks`; `create` dengan `replaces_pks_id`; terminate = `disable`, batalkan = `enable`, `before` = `GetVendorPksByID`).
    - `Get` mengembalikan `today`, `renewal_months`, `pks_status`, `active` (+ `remaining_days`), `history`, `pending`.
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.10, 4.3, 4.4, 4.5, 5.1, 5.2, 5.4, 5.6, 5.7, 11.1_
  - [x] 7.2 `VendorPksApplier` + registrasi
    - `backend/internal/service/masterdata_applier_vendor_pks.go` (baru): `create` (lock vendor `FOR UPDATE`, cek FLM aktif + `replaces_pks_id`, tandai lama digantikan, insert baru, set `replaced_by_pks_id`; data terminate lama tetap), `disable` / `enable` dengan guard nol baris → `ErrMasterDataChangeStale`, `CurrentState` = `GetVendorPksByID`. Tidak menyentuh `vendors.is_active`.
    - Daftarkan `"vendor_pks": service.VendorPksApplier{}` di `masterDataApplierRegistry` (`backend/cmd/api/main.go`).
    - _Requirements: 3.7, 3.8, 3.9, 3.10, 5.3, 5.5, 11.3, 11.5_
  - [x] 7.3 Cabang stale di `MasterDataApprovalService.applyOne`
    - `errors.Is(applyErr, ErrMasterDataChangeStale)` → `MarkStale` + return `ErrMasterDataChangeStale`; error lain tetap `MarkApplyFailed`.
    - _Requirements: 1.5, 3.8, 12.2_
  - [x] 7.4 Unit test `VendorPksAdminService`
    - Mock repo + submitter: setiap cabang validasi (nama kosong/501 karakter, tanggal salah format, `expiry < effective`, tanggal lampau/masa depan diterima), vendor 404 / internal 422 / nonaktif 409, pending 409, terminate ganda / batalkan yang tidak terminated / PKS digantikan 409, alasan kosong; payload `replaces_pks_id` nil vs ada; `before` terminate = snapshot baris.
    - _Requirements: 3.2, 3.3, 3.4, 3.5, 3.6, 5.2, 5.4, 5.6, 5.7_
    - _Model: Sonnet, Medium — test tabel terhadap aturan yang sudah ditetapkan._
  - [x] 7.5 Integration test applier dan approval (Property 5, 6, 7, 8)
    - `masterdata_applier_vendor_pks_integration_test.go`: urutan create acak (`rapid`) → satu aktif, riwayat lengkap, kolom tidak berubah; dua approve paralel (goroutine) untuk vendor yang sama → satu `applied`, satu `stale`, tetap satu aktif.
    - Stale: `replaces_pks_id` basi, vendor dinonaktifkan sebelum approve, baris terminate berubah → `stale`, tidak ada tulis.
    - Terminate → batalkan → baris kembali seperti semula (kecuali `updated_at`), `vendors` tidak berubah, satu audit `master_data_applied` per langkah; PKS baru di atas PKS Terminated memindahkan yang lama ke riwayat dengan data terminate (Req 3.10); reject tidak mengubah PKS; checker = maker ditolak orchestrator.
    - `vendor_pks_pending_integration_test.go`: submit paralel campuran create/terminate → satu pending, sisanya 409.
    - Regresi `applyOne`: applier lain yang gagal tetap `approved` + `error`. `no_hard_delete_test.go` mencakup `vendor_pks`; test pemindai memastikan `vendor_pks.sql` tidak meng-UPDATE `pks_name`/`effective_date`/`expiry_date`.
    - **Property 5: Satu PKS aktif dan riwayat tidak berubah**, **Property 6: Satu change PKS pending per vendor**, **Property 7: Change basi tidak pernah diterapkan**, **Property 8: Terminate tidak mengubah vendor dan bisa dibatalkan**.
    - _Requirements: 3.6, 3.7, 3.8, 3.9, 3.10, 4.1, 4.2, 5.3, 5.5, 11.3, 11.4, 12.2, 12.4_
  - _Model: Opus, High — maker-checker master data, satu PKS aktif di bawah approve paralel, dan perubahan jalur apply yang dipakai semua entitas._

- [ ] 8. Handler PKS, mount, dan kontrak OpenAPI
  - [x] 8.1 `AdminVendorPksHandler` + mount
    - `backend/internal/handler/admin_vendor_pks_handler.go` (baru): `GET /`, `POST /`, `POST /{pksID}/terminate`, `POST /{pksID}/unterminate`; body sesuai design; 202 `changeRequestAcceptedResponse`; `handleVendorPksError` sesuai tabel `## Error Handling` (422 validasi, 400 body bukan JSON, 404, 409, 403, 422 chain incomplete, 500 tanpa detail).
    - `main.go`: `masterDataAdmin.Mount("/api/v1/admin/vendors/{vendorID}/pks", ...)` dengan `NewVendorPksAdminService(repo, masterDataChangeService, renewalMonths)`.
    - _Requirements: 3.1, 4.3, 4.4, 4.5, 5.1, 5.4, 11.1, 11.2_
  - [x] 8.2 OpenAPI + contract test
    - `backend/api/openapi.yaml`: empat operasi + schema `VendorPksDetail`, `VendorPksRecord`, `VendorPksPending`, `SubmitVendorPksRequest`, `VendorPksReasonRequest`, `ChangeRequestAccepted`.
    - `openapi_contract_test.go`: prefix `/admin/vendors/{vendorID}/pks` di peta `mounted`; `contractCase` 200/202/403/404/409/422.
    - _Requirements: 12.3_
  - [x] 8.3 Handler test (Property 10 bagian backend)
    - `admin_vendor_pks_handler_test.go`: pemetaan setiap error ke status + body; tabel role di luar `ADMIN`/`ADMIN_PARAM` → 403 dan submitter tidak dipanggil.
    - **Property 10: Hanya role master data yang bisa memakai PKS** (bagian `backend`).
    - _Requirements: 3.5, 3.6, 11.1, 11.2_
  - _Model: Sonnet, Medium — handler tipis di atas service, mengikuti pola handler master data._

- [ ] 9. Ekspor `vendor_pks` di `backend-exq`
  - [x] 9.1 Kind, params, producer, route
    - `report_params.go`: `KindVendorPks`, `ReportParams.Q`, `VendorStatus`, `PksStatus`, cabang `ParseReportParams` (`q` trim ≤ 100, `status` default `active`, `pks_status` kunci `reports.PksStatuses`; salah → 400 `reports.ValidationError`).
    - `kinds.go`: `KindSpecs[KindVendorPks]` (`reports.VendorPksViewerRoles`, `ActionReportExported`, `fixed("vendor_pks_report")`).
    - `producers.go`: `vendorPksProducer(renewalMonths)` dengan `reports.TodayWIB(asOf)` + keyset `ExportVendorPksBatch`, replica; `Querier` + dua method.
    - `exports_handler.go`: `POST /reports/vendor-pks` di `reportRoutes`.
    - `backend-exq/cmd/api/main.go` baca `VENDOR_PKS_RENEWAL_MONTHS` (exit 1 bila salah); `backend-exq/.env.example`.
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5, 10.6_
  - [x] 9.2 OpenAPI `backend-exq`
    - `backend-exq/api/openapi.yaml`: `POST /exports/reports/vendor-pks` (param `q`, `status`, `pks_status`) dan `vendor_pks` di enum kind job; contract test `backend-exq` hijau.
    - _Requirements: 10.1_
  - [x] 9.3 Test ekspor (Property 4 bagian ekspor, Property 10 bagian ekspor)
    - `report_params_test.go`: cabang `vendor_pks` (default, nilai salah, `q` 101 karakter, hash stabil).
    - Integration producer: untuk data acak dan filter acak, id vendor + `pks_status` + masa berlaku + sisa hari + direktur sama dengan model referensi yang dipakai 6.3; header + label CSV; audit `export_requested` dan `report_exported` saat unduh.
    - Role non-master data → 403, tidak ada baris `export_jobs`.
    - **Property 4** (bagian ekspor), **Property 10** (bagian ekspor).
    - _Requirements: 6.8, 10.2, 10.3, 10.4, 10.5, 10.6_
  - _Model: Sonnet, Medium — report kind baru mengikuti pola DSR telat._

- [ ] 10. Frontend: tipe, API, hooks, helper PKS
  - [x] 10.1 Generate tipe + API + hooks + URL state
    - `pnpm --dir frontend/CompanyPortal-Vite run gen:api` dan `gen:api:exq`; commit `schema.d.ts` + `exq-schema.d.ts`.
    - `features/admin-vendors/types.ts` (hand-typed): `kind`, `director`, `PksStatus`, `pks_status`, `pks`, param `pks_status`; tipe + payload PIC dengan `is_director`. Payload vendor tetap.
    - `features/admin-vendors/pksApi.ts` (baru, tipe dari `schema.d.ts`); `hooks.ts`: `useVendorPks`, `useSubmitVendorPks`, `useTerminateVendorPks`, `useUnterminateVendorPks` dengan invalidasi `adminVendorKeys.all`, `["master-data","pending"]`, key PKS.
    - `useAdminVendorsUrlState.ts`: `pks_status` opsional, tidak masuk URL bila kosong, reset `page` ke 1.
    - _Requirements: 1.10, 8.2, 9.2, 12.3_
  - [x] 10.2 Helper format + badge (Property 3)
    - `features/admin-vendors/lib/pks.ts`: `formatValidity`, `formatMonitoring`. `components/PksStatusBadge.tsx`: lima varian dengan ikon + label, vendor internal "-".
    - `lib/pks.test.ts` dengan `fast-check`; `PksStatusBadge.test.tsx`.
    - **Property 3: Format masa berlaku dan monitoring**.
    - _Requirements: 7.2, 7.3, 7.4, 7.5, 9.1, 9.6_
  - _Model: Sonnet, Medium — tipe dan helper murni dengan pola fitur yang ada._

- [ ] 11. Frontend: layar vendor, PIC, approval, ekspor
  - [x] 11.1 Daftar vendor
    - `VendorFilterBar.tsx`: select "Status PKS" (Semua + lima label) berlabel. `VendorsTable.tsx`: kolom Nama Direktur Utama, Nama PKS, Tanggal Expired PKS, Monitoring PKS (`tabular-nums`), Status PKS.
    - `AdminVendorsPage.tsx`: tombol sekunder "Ekspor CSV" → `requestReportExport("vendor-pks", {q, status, pks_status})` lewat `useExportFlow`.
    - _Requirements: 8.2, 9.2, 10.1_
  - [x] 11.2 Direktur Utama di PIC dan info vendor
    - `VendorPicFormDialog.tsx` + `lib/vendorPicFormSchema.ts`: checkbox "Direktur Utama" untuk PIC tingkat vendor; saat dicentang cabang dan "Penerima notifikasi" dikunci, teks bantuan "hanya untuk pencatatan: tanpa login, tanpa peran PIC, tanpa notifikasi"; zod menolak kombinasi terlarang; 409 → banner.
    - `PicsPanel.tsx`: badge `info` "Direktur Utama" (`BadgeCheck`); aksi "Atur peran" disembunyikan untuk direktur. `PicRolesDialog.tsx`: pesan bila server mengembalikan 409.
    - `InfoTab.tsx`: blok baca-saja Direktur Utama (nama, telepon, email) atau "Belum dicatat" dengan petunjuk ke tab PIC. `VendorFormDialog.tsx` tidak berubah.
    - _Requirements: 1.1, 1.2, 1.3, 1.5, 1.6, 1.10, 9.3_
  - [x] 11.3 Tab PKS di detail vendor
    - `VendorDetailPage.tsx`: tab "PKS" (tersembunyi untuk vendor internal). `PksPanel.tsx`: kartu PKS aktif, tabel riwayat, blok pending; tombol "Catat PKS baru" / "Terminate PKS" / "Batalkan terminate" sesuai state, nonaktif selama pending atau vendor nonaktif.
    - `PksFormDialog.tsx` (zod `expiry >= effective`), `PksReasonDialog.tsx` (alasan wajib ≤ 500); 202 → "menunggu persetujuan", 422 → pesan per field, 404/409 → banner. Copy tanpa em dash.
    - _Requirements: 9.4, 9.5, 9.6, 9.7, 9.8_
  - [x] 11.4 Label approval inbox dan Ekspor Saya
    - `features/approvals/lib/changeDiff.ts`: `vendor_pks` = "PKS vendor"; op label per entitas (`disable` → "Terminate PKS", `enable` → "Batalkan terminate"); diff terminate menampilkan nama PKS + alasan; field `is_director` pada diff PIC berlabel "Direktur Utama".
    - `features/exports/status.ts`: `KIND_LABEL.vendor_pks`, `REPORT_KEY_BY_KIND.vendor_pks`.
    - _Requirements: 10.1, 11.4_
  - [x] 11.5 Test komponen
    - `PksPanel`, `PksFormDialog`, `PksReasonDialog`: tombol per state (aktif, terminated, pending, vendor nonaktif, internal), validasi, pesan 202/422/404/409, label untuk screen reader, navigasi keyboard.
    - `VendorPicFormDialog`, `PicsPanel`, `PicRolesDialog`: checkbox direktur mengunci cabang/notifikasi, badge, aksi peran tersembunyi, pesan 409.
    - `VendorFilterBar` / `useAdminVendorsUrlState`, `VendorsTable`, `InfoTab`, `changeDiff.test.ts`, `exports/api.test.ts` (jumlah kind 9).
    - _Requirements: 1.10, 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7, 9.8_
  - _Model: Sonnet, Medium — komponen React dengan pola dialog master data dan token `ui_design.md` yang ada._

- [x] 12. Checkpoint: gate kualitas
  - `backend`, `backend-exq`, `pkg`: `go test ./...` + integration (`DATABASE_URL` diisi, 0 skip), `golangci-lint run --build-tags integration ./...`, `govulncheck ./...`, `sqlc generate` tanpa diff di dua modul, `go test ./internal/handler -run OpenAPI`.
  - `backend_python`: `python -m unittest` untuk `dsr_late_notify` (0 skip dengan `DATABASE_URL`).
  - `go run ./cmd/migrate status` tanpa MISMATCH.
  - CompanyPortal: `gen:api` + `gen:api:exq` tanpa diff, `pnpm run lint`, `pnpm run test:coverage` (84/80/80/85), `pnpm run build`.
  - VendorPortal: `git diff --stat frontend/VendorPortal-Vite` kosong (tidak ada PKS/direktur), `pnpm run test` hijau.
  - Traceability: semua criteria dan Property 1–11 punya test hijau.
  - _Requirements: 11.6, 12.2, 12.4_
  - _Model: Sonnet, Medium — menjalankan gate dan triase kegagalan._

- [x] 13. Dokumentasi
  - `.claude/CLAUDE.md` Sec 3: ganti baris "Approved 2026-10-05 ... `vendors.director_*`/`pks_*`" dengan `vendor_pks` + `vendor_pics.is_director` (mig `022`, constraint roles di `037`) dan kind `vendor_pks` (mig `037`); env `VENDOR_PKS_RENEWAL_MONTHS`.
  - `.claude/docs/data-map.md`, `.kiro/steering/development-plan.md` (status, migrasi terpakai, next free `038`), `.claude/development-progress.md`, `.claude/sdlc/README.md`, `.claude/docs/deployment.md` (env baru di kedua backend + cek change `vendor_pic` pending sebelum migrasi).
  - _Requirements: 12.5_
  - _Model: Haiku, Low — edit dokumen sesuai hasil yang sudah ada._

- [ ] 14. Cek browser manual (user)
  - User: daftar vendor (kolom, filter Status PKS, pencarian nama, Ekspor CSV + unduh, Excel non-ASCII); PIC Direktur Utama (centang di form PIC, cabang/notifikasi terkunci, badge, tidak bisa atur peran, direktur kedua ditolak, tampil di info vendor dan daftar vendor); tab PKS (catat baru, terminate, batalkan terminate, riwayat, pending); approval oleh checker berbeda; label di inbox approval; vendor internal tanpa tab PKS; role non-admin tidak bisa masuk; keyboard + screen reader. Hasil dicatat di `## Notes`.
  - _Requirements: 9.8_
  - _Model: Haiku, Low — dijalankan user; agen hanya mencatat hasil._

## Notes

### Keputusan yang tercermin
- D1–D3, D17, D24–D26: tasks 1.1, 4.2, 7.1, 7.2, 7.5. D4, D13, D19 (tidak termasuk): tidak ada task. D6, D20, D30: tasks 1.1, 3.1, 6.1, 9.1. D7: 7.2, 7.5. D8, D29: 1.1, 7.1, 11.3. D9, D14, D22: 1.1, 10.2. D10 (CSV PIC): 3.2, 5.4. D11, D28: 4.1, 6.2, 11.1. D12: 8.1, 9.1, 12. D15, D21: 1.1, 10.2. D16: 5.4, 9. D18: 10.2. D23: 7.1. D27: 11.1, 3.1. D31: 8.2, 10.1. D32 (direktur sebagai PIC): 1.1, 2.1, 3.2, 4.1, 4.2, 5, 6.2, 11.2. DD1–DD11 (design): 1.1, 2.1, 4.1, 5.2, 6.1, 7.2, 7.3, 9.1, 11.1.

### Deploy
- Change `vendor_pic` yang `pending` saat `022` diterapkan menjadi `stale` saat di-approve; maker submit ulang. Hitung sebelum migrasi (task 1.2) dan beri tahu checker. Change `vendor` pending tidak terdampak (DD11).
- `VENDOR_PKS_RENEWAL_MONTHS` harus bernilai sama di `backend` dan `backend-exq`.
- `022` bernomor lebih kecil dari `036` yang sudah diterapkan di dev; runner tetap menerapkannya (setiap file yang belum ada di `schema_migrations`).

### Definition of Done
- Semua leaf task dicentang, gate task 12 hijau, dokumentasi task 13 diperbarui, cek browser task 14 oleh user selesai.
- Maker-checker + audit untuk semua tulis; tidak ada hard delete; bacaan list/ekspor dari replica, detail dan tulis di primary.
- Director_PIC tidak pernah punya login, roles, atau notifikasi.

### Catatan eksekusi
- (diisi saat implementasi: jumlah change `vendor_pic` pending di task 1.2, hasil cek browser task 14)
- Task 1.2 (2026-11-05, dev): change `vendor_pic` `pending` sebelum apply = **0** (tidak ada yang akan `stale`). `migrate up` menerapkan `022_vendor_pks.sql` + `037_export_jobs_vendor_pks.sql`; `migrate status` = 36 applied, 0 pending, tanpa MISMATCH.
- Task 7.1 (2026-11-05): dua perubahan `backend/queries/vendor_pks.sql` (sqlc ulang, hanya `vendor_pks.sql.go` berubah): (a) `GetPendingVendorPksChange` memakai `sqlc.arg('vendor_id')::bigint` — `$1` polos membuat sqlc menghasilkan parameter `payload []byte` untuk perbandingan bigint; (b) query baru `GetVendorPksStatus` (memanggil `vendor_pks_status()` dengan input yang sudah dimuat, `''` = INTERNAL) supaya detail memakai aturan SQL yang sama dengan list/filter/ekspor (Req 6.8). `VendorPksAdminRepository` = primary saja. Payload diekspor sebagai `VendorPksCreatePayload` / `VendorPksReasonPayload` untuk applier 7.2; `Before` terminate/batalkan = `db.VendorPk` dari `GetVendorPksByID`.
- Task 7.5 (2026-11-05): approve paralel diuji dengan change pertama sudah `approved` (belum applied) saat change kedua disubmit — satu-satunya cara dua create untuk vendor yang sama bisa bersamaan, karena index pending hanya mengizinkan satu `pending` per vendor; kedua apply lalu jalan paralel lewat `RetryApply`. Harness `harness()` tidak mendaftarkan `vendor_pks`; test menambahkannya sendiri (`pksHarness`) dan menghapus baris `vendor_pks` sebelum cleanup vendor. Test pemindai kolom immutable ada di `no_hard_delete_test.go` (`TestQueries_VendorPksImmutableColumns`).
- Task 12 (2026-11-06, dev, gate kualitas): **hijau kecuali masalah lama di bawah.** `backend` `go test -tags integration ./...` + `backend-exq` + `pkg`: lulus, 0 SKIP (backend dan exq dijalankan berurutan; dijalankan paralel pada DB yang sama, `TestW11Property11SingleDownload` gagal karena antrean `claim` dipakai bersama, lulus saat sendirian). `golangci-lint --build-tags integration`: awalnya 4 temuan gosec G115 (`int32(renewalMonths)` di `vendor_admin.go` x2, `vendor_pks_admin.go`, `producers_report.go`), diberi `//nolint:gosec` (nilai dibatasi `ParseVendorPksRenewalMonths`), lalu 0 isu di backend, backend-exq, pkg. `govulncheck`: 0 kerentanan terpanggil (backend melaporkan 1 di modul yang di-require tapi tidak dipanggil). `sqlc generate` v1.31.1 di dua modul (+ perbaikan `UserLeafe`) selesai; tidak ada perubahan setelah regenerasi, tetapi hasil dihasilkan ulang di working tree yang belum di-commit sehingga tidak ada pembanding HEAD. `go test ./internal/handler -run OpenAPI` lulus di dua modul. `migrate status`: 36 applied, 0 pending, tanpa MISMATCH. CompanyPortal: `gen:api` + `gen:api:exq` identik, `lint` bersih, `test:coverage` 1939/1939 (87.05/82.74/83.61/88.07 >= 84/80/80/85), `build` ok (initial JS 199.5 KB gzip dari budget 300). VendorPortal: `git diff --stat` kosong, `test` 250/250. `backend_python` `dsr_late_notify` unittest: 16/17 lulus, 0 skip; satu gagal = masalah lama (lihat berikut). Traceability: Property 1–11 punya test (pencarian nama `Property<N>` di test Go/TS/Py, bukan pemetaan per kriteria satu-satu).
  - Masalah lama (bukan dari spec ini): (1) `test_recipients_idempotent_and_readable_by_go` mengharap 3 cabang telat, dapat 4 (bergantung data dev; versi HEAD gagal sama). (2) `TestW11Property15ClaimOnce` flaky pada seed acak (sekali gagal, 6 ulang lulus; tidak muncul pada dua run ini). (3) `gofmt -l` menandai `openapi_contract_test.go`, `user_admin_vendor_test.go`, `dsr_late_report_repository_test.go` di HEAD; `admin_vendor_pic_handler_test.go` juga ditandai tetapi hanya karena akhir baris CRLF di working copy Windows (diff = seluruh file; isi LF di HEAD bersih).

## Task Dependency Graph

- **Wave 0:** 1.1 (migrasi `022`), 2.1 (migrasi `037`), 3.1 (`pkg/reports`), 3.2 (`pkg/csvexport`). Saling independen.
- **Wave 1:** 1.2 apply migrasi (butuh 1.1, 2.1); 3.3 test `pkg/` (butuh 3.1, 3.2).
- **Wave 2:** 1.3, 1.4 test migrasi + fungsi (butuh 1.2); 2.2 test `037` (butuh 1.2); 4.1, 4.2, 4.3 query (butuh 1.1, 2.1).
- **Wave 3:** 4.4 sqlc dua modul (butuh 4.1–4.3).
- **Wave 4:** 5.1 service PIC; 7.1 service PKS; 7.3 `applyOne`; 9.1 ekspor `backend-exq` (semua butuh 4.4; 9.1 juga 3.1, 2.1).
- **Wave 5:** 5.2 applier PIC (butuh 5.1, 7.3); 5.3 roles + handler PIC (butuh 5.1); 7.2 applier PKS + registry `main.go` (butuh 7.1, 7.3); 7.4 unit test (butuh 7.1); 9.2 OpenAPI exq (butuh 9.1).
- **Wave 6:** 5.4 CSV PIC (butuh 3.2, 5.1, 5.2); 6.1 config `main.go` + service (butuh 7.2, serial `main.go`); 7.5 integration PKS (butuh 7.1–7.3); 9.3 test ekspor (butuh 9.1, 9.2).
- **Wave 7:** 5.5 test direktur (butuh 5.1–5.4); 6.2 list + detail (butuh 6.1, 5.3); 8.1 handler PKS + mount `main.go` (butuh 6.1, 7.2).
- **Wave 8:** 6.3 test list (butuh 6.2); 8.2 OpenAPI + contract (butuh 8.1); 8.3 handler test (butuh 8.1).
- **Wave 9:** 10.1 gen tipe + API + hooks (butuh 5.3, 6.2, 8.2, 9.2).
- **Wave 10:** 10.2 helper + badge (butuh 10.1).
- **Wave 11:** 11.1, 11.2, 11.3, 11.4 layar (butuh 10.1, 10.2; file berbeda).
- **Wave 12:** 11.5 test komponen (butuh 11.1–11.4).
- **Wave 13:** 12 checkpoint (butuh semua task 1–11).
- **Wave 14:** 13 dokumentasi (butuh 12).
- **Wave 15:** 14 cek browser user (butuh 13).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "2.1", "3.1", "3.2"] },
    { "id": 1, "tasks": ["1.2", "3.3"] },
    { "id": 2, "tasks": ["1.3", "1.4", "2.2", "4.1", "4.2", "4.3"] },
    { "id": 3, "tasks": ["4.4"] },
    { "id": 4, "tasks": ["5.1", "7.1", "7.3", "9.1"] },
    { "id": 5, "tasks": ["5.2", "5.3", "7.2", "7.4", "9.2"] },
    { "id": 6, "tasks": ["5.4", "6.1", "7.5", "9.3"] },
    { "id": 7, "tasks": ["5.5", "6.2", "8.1"] },
    { "id": 8, "tasks": ["6.3", "8.2", "8.3"] },
    { "id": 9, "tasks": ["10.1"] },
    { "id": 10, "tasks": ["10.2"] },
    { "id": 11, "tasks": ["11.1", "11.2", "11.3", "11.4"] },
    { "id": 12, "tasks": ["11.5"] },
    { "id": 13, "tasks": ["12"] },
    { "id": 14, "tasks": ["13"] },
    { "id": 15, "tasks": ["14"] }
  ]
}
```