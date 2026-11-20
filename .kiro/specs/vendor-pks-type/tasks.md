# Implementation Plan: PKS per Type + No. WOD

> Status: accepted 2026-11-18 (engineer). Berdasarkan `requirements.md` (accepted 2026-11-18, PO) dan `design.md` (accepted 2026-11-18, engineer).

## Overview

Urutan pengerjaan:
1. Migrasi `043` ditulis lalu diterapkan ke dev. Langkah `migrate up` butuh OK user (CLAUDE.md Sec 4 #7).
2. Query dan sqlc untuk dua modul.
3. Backend master type PKS.
4. Backend PKS per type.
5. Daftar vendor.
6. Kontrak bersama `pkg/` dan ekspor `backend-exq`.
7. CompanyPortal.

Spec ditutup dengan gate kualitas, dokumentasi, langkah operasional (input 12 PKS), dan cek browser oleh user.

Integration test berjalan terhadap DB dev (`DATABASE_URL`). Karena itu `043` harus sudah diterapkan ke dev (task 1.3) sebelum test integration apa pun yang menyentuh PKS dijalankan.

Pola yang dirujuk:
- Master kecil dengan maker-checker: `internal/service/masterdata_applier_cash_count_anomaly_category.go` + service/handler `cash_count_anomaly_category` (`041`).
- PKS yang sudah ada: `vendor_pks_admin.go`, `masterdata_applier_vendor_pks.go`, `admin_vendor_pks_handler.go`, `queries/vendor_pks.sql`.
- Lock di tx apply: `masterdata_applier_vendor_branch.go` (`FOR SHARE` region, `042`).
- CSV legacy header: `pkg/csvexport/entities.go` (`LegacyHeaders`, `vendor-export-pks`).
- sqlc v1.31.1 + perbaikan `UserLeafe` → `UserLeave`.
- Skill: `cms2-master-data` (task 3), `ecc:golang-patterns` / `ecc:golang-testing` (Go), `ui-ux-pro-max` (task 7).

### Model Split

**Sonnet-runnable**
- 1.2 Skrip rollback dev — Sonnet, Medium (needs 1.1)
- 1.4 Integration test migrasi — Sonnet, High (needs 1.3, 2.5)
- 2.1–2.5 Query + sqlc — Sonnet, Medium (needs 1.1)
- 3.1 Service master type — Sonnet, High (needs 2.5)
- 3.3 Repo + handler + mount + OpenAPI type — Sonnet, Medium (needs 3.1, 3.2)
- 3.4 Test master type — Sonnet, High (needs 3.3, 1.3)
- 4.3 Handler PKS + OpenAPI — Sonnet, Medium (needs 4.1, 3.3)
- 4.4 Test PKS per type — Sonnet, High (needs 4.2, 4.3, 1.3)
- 5.1 Daftar vendor — Sonnet, High (needs 2.5)
- 5.2 Test daftar vendor — Sonnet, High (needs 5.1, 1.3)
- 6.1–6.4 Kontrak bersama + ekspor + test — Sonnet, Medium/High (needs 2.5)
- 7.1–7.5 Frontend — Sonnet, Medium (needs 4.3, 5.1, 6.3)
- 8. Checkpoint gate kualitas — Sonnet, Medium (needs 1–7)
- 9. Dokumentasi — Haiku, Low (needs 8)
- 10. Langkah operasional (user) — Haiku, Low (needs 9 + deploy dev)
- 11. Cek browser manual (user) — Haiku, Low (needs 9)

**Opus-required**
- 1.1 Migrasi `043`: Opus, High. Mengganti index unik aktif dan index pending yang dipakai maker-checker, serta menambah guard.
- 1.3 Terapkan ke dev: Opus, High. Cek `migrate status`, guard, dan `migrate up`, setiap langkah dengan OK user.
- 3.2 Applier type (`FOR UPDATE` + cek in-use): Opus, High. Ini jalur apply maker-checker, dan lock-nya bersaing dengan applier PKS.
- 4.1 Service PKS (type, WOD, pending per type): Opus, High. Mengubah aturan gerbang maker-checker (D24 diganti A1).
- 4.2 Applier PKS per type (`FOR SHARE` type, ganti hanya type sama): Opus, High. Menjaga invarian satu PKS aktif per (vendor, type) saat approve konkuren.

**Dependensi lintas grup**
- Sonnet 2.x butuh Opus 1.1 (sqlc membaca file migrasi).
- Sonnet 1.4, 3.4, 4.4, 5.2, 6.4 butuh Opus 1.3 (DB dev sudah `043`).
- Sonnet 3.3 butuh Opus 3.2 (registry di `main.go`). Sonnet 4.3 butuh Opus 4.1, dan Sonnet 4.4 butuh Opus 4.2.

## Tasks

- [ ] 1. Migrasi `043_vendor_pks_type.sql`
  _Model: Opus, High — index unik aktif + index pending maker-checker diganti; guard data_
  - [x] 1.1 Tulis `backend/migrations/043_vendor_pks_type.sql` sesuai `design.md` `## Data Models`
    - Header komentar (spec `.kiro/specs/vendor-pks-type/`, forward-only, SAFETY) + `BEGIN;`/`COMMIT;`.
    - Guard: `vendor_pks` berisi → `RAISE EXCEPTION`; change `vendor_pks` pending → `RAISE EXCEPTION`.
    - `vendor_pks_types` + `vendor_pks_types_code_uq` (upper) + trigger `set_updated_at` + seed `FLM`, `CIT`.
    - `vendor_pks.pks_type_id` (FK RESTRICT, NOT NULL), `wod_number` (NOT NULL, CHECK trim + 1–100).
    - `vendor_pks_active_uq` dibuat ulang per (vendor_id, pks_type_id); `vendor_pks_wod_uq` (lower); `vendor_pks_type_active_idx`.
    - `master_data_change_requests_vendor_pks_pending_uq` dibuat ulang per (vendor_id, pks_type_id); index pending WOD dan code type.
    - `COMMENT ON` tabel/kolom baru; komentar `vendor_pks` diperbarui (satu aktif per vendor per type).
    - _Requirements: 1.8, 1.10, 2.7, 2.8, 2.9, 9.1, 9.2_
  - [x] 1.2 Tulis `backend/scripts/dev/rollback_043_vendor_pks_type.sql`
    - Satu transaksi: kembalikan index lama (per vendor), drop index/kolom/tabel baru, `DELETE FROM schema_migrations WHERE version = '043_vendor_pks_type.sql'`. Hanya untuk dev (data PKS kosong).
    - _Requirements: 9.1_
  - [x] 1.3 Terapkan `043` ke dev (butuh OK user)
    - `cd backend && go run ./cmd/migrate status`, cek 0 baris `vendor_pks` + 0 change `vendor_pks` pending, minta OK user, `go run ./cmd/migrate up`, `status` lagi (0 pending). Catat hasilnya di `## Notes`.
    - _Requirements: 9.1, 9.2_
  - [x] 1.4 Integration test migrasi (`internal/repository/migration043_integration_test.go`, `//go:build integration`)
    - Seed `FLM`/`CIT` aktif; dua PKS aktif type sama untuk satu vendor ditolak index, beda type diterima (Property 1, sisi DB); WOD beda huruf besar/kecil ditolak (Property 3, sisi DB); `wod_number` dengan spasi tepi ditolak CHECK; guard dicek lewat menjalankan blok DO terhadap tabel berisi di dalam tx yang di-rollback.
    - _Requirements: 2.7, 9.2, 9.4_

- [ ] 2. Query + sqlc
  _Model: Sonnet, Medium — SQL sesuai design, tanpa keputusan baru_
  - [x] 2.1 `backend/queries/vendor_pks_types.sql` (baru): `ListPksTypes` (+ `in_use`), `GetPksType`, `GetPksTypeByCode`, `ListActivePksTypes`, `CreatePksType`, `UpdatePksTypeName`, `DisablePksType`, `EnablePksType`, `LockPksType`, `LockPksTypeShare`, `PksTypeInUse`, `GetPendingPksTypeChange`, `PksTypeCodePending`
    - _Requirements: 1.3, 1.5, 1.9_
  - [x] 2.2 `backend/queries/vendor_pks.sql`: kolom `pks_type_id`, `wod_number` + join type di semua SELECT; `GetActiveVendorPks(vendor_id, pks_type_id)`; `InsertVendorPks` dengan type + WOD; `GetPendingVendorPksChanges` (many); `WodNumberTaken`
    - _Requirements: 2.1, 2.4, 2.5, 2.8, 3.2_
  - [x] 2.3 `backend/queries/vendors_admin.sql`: hapus join PKS tunggal dari `ListVendorsAdmin`/`CountVendorsAdmin`; filter `pks_type_id` + `pks_status`; query baru `ListVendorPksSummaries(vendor_ids[], today, renewal)` (hanya `FLM_VENDOR`, CROSS JOIN type aktif)
    - _Requirements: 4.1, 4.3, 4.5, 4.6, 4.7_
  - [x] 2.4 Query laporan/ekspor: `backend/queries/vendor_pks_report.sql` (satu baris per PKS aktif + cabang `belum_diisi`, kolom type + WOD), salinannya di `backend-exq`, dan `backend-exq/queries/vendor_export_pks.sql` (join PKS dibuang, sisa kolom direktur)
    - _Requirements: 6.1, 6.2, 7.1_
  - [x] 2.5 `sqlc generate` di `backend/` dan `backend-exq/` (v1.31.1) + perbaikan `UserLeafe` → `UserLeave`; `go build ./...` di kedua modul hijau, atau error compile dicatat sebagai pekerjaan task 3–6
    - _Requirements: 9.1_

- [ ] 3. Backend master type PKS (`cms2-master-data`)
  _Model: Opus (3.2) / Sonnet (3.1, 3.3, 3.4), High — entitas master data baru dengan maker-checker_
  - [x] 3.1 `internal/service/pks_type_admin.go`: `PksTypeAdminService` (`List`, `Get`, `Create`, `Update`, `Disable`, `Enable`) → `MasterDataChangeService.Submit` dengan `entity_type = "vendor_pks_type"`; validasi code `^[A-Z0-9]{2,10}$`, name 1–100 setelah trim, update hanya name, code duplikat (tabel atau pending) → `ErrPksTypeConflict`, disable saat dipakai → `ErrPksTypeInUse`; role dicek ulang
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.7, 8.1_
  - [x] 3.2 `internal/service/masterdata_applier_pks_type.go`: `PksTypeApplier` (create `23505` → conflict; update guarded `is_active`, 0 baris → stale; disable `LockPksType` `FOR UPDATE` + `PksTypeInUse` → `ErrPksTypeInUse`; enable), audit di tx yang sama lewat alur applier yang ada; daftarkan `"vendor_pks_type"` di `masterDataApplierRegistry` (`cmd/api/main.go`)
    - _Requirements: 1.6, 1.7, 1.8, 8.3, 8.4_
  - [x] 3.3 `internal/repository/pks_type_repository.go` (primary + replica), `internal/handler/admin_pks_type_handler.go` (6 rute, JSON datar, 202/409/422), mount `/api/v1/admin/pks-types` di `masterDataAdmin` (`main.go`), path di `backend/api/openapi.yaml`
    - _Requirements: 1.9, 8.1, 8.2, 9.3_
  - [x] 3.4 Test master type: unit table-driven service (validasi, update hanya name, 409); integration Property 4 (submit 409; disable lolos submit lalu PKS dibuat sebelum approve → apply gagal; PKS terminated tetap memblok); stale update; maker ≠ checker; RBAC 403 (role lain) di route dan `Submit`; audit gagal = rollback; list dari replica
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.9, 8.1, 8.2, 8.3, 8.4, 9.4_

- [ ] 4. Backend PKS per type
  _Model: Opus (4.1, 4.2) / Sonnet (4.3, 4.4), High — aturan gerbang maker-checker + invarian satu aktif per (vendor, type)_
  - [x] 4.1 `internal/service/vendor_pks_admin.go`: `Create` menerima `pks_type` (code → id aktif, else 422) + `wod_number` (trim, 1–100 → 422; `WodNumberTaken` → 409); payload semua op (create/disable/enable) membawa `vendor_id` + `pks_type_id`; cek pending per (vendor, type), `23505` index pending → `ErrMasterDataChangePending`; `Get` → `groups` per type (urut code); vendor boleh punya subset type
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.8, 2.10, 3.2, 3.3_
  - [x] 4.2 `internal/service/masterdata_applier_vendor_pks.go`: create = `LockVendorForPks` → `LockPksTypeShare` (nonaktif → stale) → `GetActiveVendorPks(vendor, type)` vs `replaced_pks_id` snapshot (beda → stale) → mark replaced → insert (`23505` WOD → 409 `ErrVendorPksWodTaken`; index aktif → stale) → set replaced_by; disable/enable (terminate/unterminate) per id, tidak menyentuh type lain; tidak ada UPDATE pada `pks_type_id`/`wod_number`
    - _Requirements: 2.5, 2.6, 2.7, 2.9, 3.1_
  - [x] 4.3 `internal/handler/admin_vendor_pks_handler.go`: body create + `pks_type`, `wod_number`; error per field 409/422; respons GET bentuk `groups`; `backend/api/openapi.yaml` diperbarui; `go test ./internal/handler -run OpenAPI` hijau
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.2, 9.3_
  - [x] 4.4 Test PKS per type: unit service (type/WOD/pending per type); integration Property 1 (dua approve konkuren type sama → satu aktif), Property 2 (ganti/terminate type A, baris type B identik), Property 3 (WOD case/trim/riwayat/pending → 409), Property 5 (type nonaktif submit 422, nonaktif saat apply → stale), Property 8 (pending type sama 409, beda type boleh); stale replaced_pks_id; reject tidak mengubah apa pun; RBAC 403; test `vendor-pks` lama yang mengandaikan satu PKS per vendor diperbarui
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9, 2.10, 3.1, 3.2, 3.3, 8.2, 9.4_

- [ ] 5. Daftar vendor
  _Model: Sonnet, High — perubahan kontrak API daftar + filter yang harus sama dengan tampilan_
  - [x] 5.1 `internal/service/vendor_admin.go` + `internal/handler/admin_vendor_handler.go` + repo: param `pks_type` + `pks_status` wajib berdua (satu saja / tidak dikenal / type nonaktif → 400); resolve code → id; item tanpa field PKS tunggal, dengan `pks: [{pks_type, pks_type_name, pks_status, remaining_days}]` dari `ListVendorPksSummaries` (satu query per halaman); vendor INTERNAL `pks: []`; list dari replica
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.6, 4.7_
  - [x] 5.2 Test daftar vendor: unit parse param (400); property `rapid` + integration Property 6 (filter = status tampil, `total` benar) dan Property 7 (satu entry per type aktif, urut code, `belum_diisi` bila kosong, INTERNAL kosong + tidak cocok filter); type nonaktif tidak jadi entry
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 9.4_

- [ ] 6. Kontrak bersama + ekspor `backend-exq`
  _Model: Sonnet, Medium (6.4 High) — kontrak CSV/laporan dua modul_
  - [x] 6.1 `pkg/reports/vendor_pks.go`: `VendorPksHeader` 13 kolom (Type PKS, No. WOD setelah kolom direktur), `VendorPksRow`/`VendorPksCells` + `PksTypeCode`, `WodNumber` lewat `CSVSafe`; validator params `pks_type` + `pks_status` (pasangan wajib)
    - _Requirements: 6.3, 6.4_
  - [x] 6.2 `pkg/csvexport/entities.go`: `VendorReadOnlyColumns` → 3 kolom direktur; Export_Header lama (9 kolom baca-saja) masuk `LegacyHeaders`; impor PKS tetap tidak ada
    - _Requirements: 7.1, 7.2, 7.3_
  - [x] 6.3 `backend-exq`: producer + params `vendor_pks` (cabang `belum_diisi`, filter type + status, 400 untuk pasangan tidak lengkap), RBAC Master_Data_Role, audit dan replica tetap; producer CSV vendor + pratinjau tanpa kolom PKS
    - _Requirements: 6.1, 6.2, 6.4, 7.1_
  - [x] 6.4 Test: unit `pkg/reports` (header, cells, CSVSafe pada type/WOD, validator) dan `pkg/csvexport` (header baru, legacy 9 kolom, Import_Header, header lain ditolak); property Property 10 (round-trip CSV vendor); integration Property 9 (satu baris per PKS aktif; `belum_diisi` = vendor FLM tanpa PKS type itu); 403 untuk role lain; test `vendor-export-pks` diperbarui
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 7.1, 7.2, 7.3, 9.4_

- [x] 7. CompanyPortal (`ui-ux-pro-max`, brand "Merah Sirih")
  _Model: Sonnet, Medium — layar mengikuti pola master data yang ada_
  - [x] 7.1 `pnpm --dir frontend/CompanyPortal-Vite run gen:api` → `src/lib/api/schema.d.ts`; tipe `pksApi.ts`/`types.ts` dari schema (rute PKS + pks-types); `/admin/vendors` tetap ditulis tangan dengan bentuk `pks[]` baru
    - _Requirements: 9.3, 4.2_
  - [x] 7.2 `features/admin-pks-types/` (baru): tabel (code, nama, status, "Dipakai"), dialog tambah/ubah nama, nonaktif/aktif (nonaktif dimatikan bila dipakai + alasan teks), 202 → "menunggu persetujuan", 409/422 per field; route + item menu Pengaturan untuk `ADMIN`/`ADMIN_PARAM`
    - _Requirements: 5.1, 5.6_
  - [x] 7.3 `features/admin-vendors/`: `VendorsTable` grup kolom per type aktif (Status badge ikon + teks, Monitoring; `belum_diisi` → "Tidak ada" netral); `VendorFilterBar` + `useAdminVendorsUrlState` select Type + Status (wajib berdua); kolom PKS tunggal dan filter lama dihapus
    - _Requirements: 5.2, 5.3, 5.6_
  - [x] 7.4 `features/admin-vendors/`: `PksPanel` satu kartu per type (aktif + WOD, riwayat, pending); `PksFormDialog` select type aktif + No. WOD wajib, error 409/422 per field; tombol terminate/batalkan terminate per type, dimatikan hanya saat type itu punya change pending
    - _Requirements: 5.4, 5.5, 5.6_
  - [x] 7.5 Test vitest + RTL: tabel type (disable mati bila dipakai), kolom per type + "Tidak ada", filter pasangan di URL, `PksFormDialog` (type + WOD, 409/422), `PksPanel` per type, keyboard + label aksesibel; schema stale-check hijau; test lama `admin-vendors` diperbarui
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

- [x] 8. Checkpoint gate kualitas
  _Model: Sonnet, Medium — jalankan gate, perbaiki sampai hijau_
  - `cd backend && go test ./...` + `golangci-lint run --build-tags integration ./...`; sama di `backend-exq` dan `pkg`; integration dengan `DATABASE_URL` (localhost); `go test ./internal/handler -run OpenAPI`.
  - `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build`.
  - Coverage `internal/*` ≥ 80% untuk paket yang disentuh.
  - Grep: VendorPortal tidak memanggil `/admin/pks-types` atau `/admin/vendors/*/pks`.
  - Hasil dicatat di `## Notes` → `### Test Results`.
  - _Requirements: 8.5, 9.4_

- [x] 9. Dokumentasi
  _Model: Haiku, Low — teks_
  - `.claude/CLAUDE.md` Sec 3 (bullet `vendor_pks_types` + PKS per type, migrasi `043`) dan Sec 12 (baseline: nomor berikutnya `044`); `.claude/docs/data-map.md` (anchor `vendor-pks-type-migration-043`); `.claude/docs/decisions.md` (Q1–Q9, DD1–DD10; D2/D24/D27 `vendor-pks` dan kolom PKS `vendor-export-pks` digantikan); `.claude/development-progress.md`; `graphify update .`.
  - _Requirements: 9.5_

- [ ] 10. Langkah operasional: input data (user, bukan kode)
  _Model: Haiku, Low — checklist untuk user_
  - Isi `legal_name` ketujuh vendor lewat layar/CSV vendor (maker-checker): `ABA` PT Abacus Cash Solution, `ADV` PT Advantage SCM, `BJK` PT Bintang Jasa Artha Kelola, `BRNK` PT Brinks Solutions Indonesia, `KJR` PT Kelola Jasa Artha, `SSI` PT Swadharma Sarana Informatika, `TAG` PT Tunas Artha Gardatama.
  - Input 12 PKS lewat "Catat PKS baru" (type, No. WOD, judul, 2026-01-01 s/d 2027-12-31), lalu approve oleh checker lain.
  - Tidak ada seed atau skrip yang melewati maker-checker.
  - _Requirements: 10.1, 10.2_

- [ ] 11. Cek browser manual (user)
  _Model: Haiku, Low — dicatat "outstanding (user)", tidak pernah ditandai selesai oleh AI_
  - Layar type PKS (tambah, ubah nama, nonaktif diblok bila dipakai), daftar vendor (kolom per type, filter type + status), detail PKS per type, form PKS (type + WOD, error duplikat), ekspor PKS dan CSV vendor.
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6_

## Notes

### Keputusan yang tercermin
- Q1–Q1d (type bisa bertambah, wajib, layar + maker-checker `ADMIN`/`ADMIN_PARAM`, nonaktif diblok termasuk PKS terminated): task 1.1, 3.x, 7.2.
- Q2/Q2a (kolom per type: Status + Monitoring; filter type + status): task 2.3, 5.x, 7.3.
- Q3 (tidak wajib semua type; "Tidak ada"): task 4.1, 5.1, 7.3. Q4 (WOD wajib, bebas, unik global): task 1.1, 4.x. Q5 (ekspor per PKS; kolom PKS CSV vendor dibuang): task 2.4, 6.x. Q6 (input lewat layar, D16 tetap): task 6.2, 10. Q7 (type ≠ `vendors.kind`): task 5.1. Q8/Q8a/Q9: task 10.
- A1–A5 dan DD1–DD10: sesuai `design.md`.

### Di luar spec ini
- Impor CSV PKS (D16 tetap). Notifikasi PKS mendekati expired. Upload file PKS.

### Deploy
- Urutan: `043` (`migrate up`) → `backend` + `backend-exq` + CompanyPortal **dalam satu rilis**. Kontrak `/admin/vendors` (A5), bentuk detail PKS, dan header ekspor berubah bersamaan.
- Guard `043` menolak DB dengan baris `vendor_pks` atau change PKS pending. Di lingkungan lain, data PKS harus dikosongkan atau dipetakan dulu lewat spec terpisah.

### Definition of Done
- CLAUDE.md Sec 11: maker-checker + audit (Req 8), replica/primary (Req 4.7, 8.5), RBAC di route + service, OpenAPI + `gen:api`, test hijau termasuk RBAC/maker-checker, tanpa secret baru (tidak ada env baru).

### Test Results
- Task 1.3 (2026-11-18): `migrate status` sebelum = 41 applied, 1 pending (`043`); guard dev: 0 baris `vendor_pks`, 0 change `vendor_pks` pending. OK user di sesi ini → `go run ./cmd/migrate up` = "1 applied"; `status` sesudah = 42 applied, 0 pending. Cek: `vendor_pks_types` berisi `CIT`, `FLM` (aktif); index `vendor_pks_active_uq`, `vendor_pks_wod_uq`, `vendor_pks_type_active_idx`, `vendor_pks_types_code_uq`, `master_data_change_requests_vendor_pks_pending_uq` ada.
- Task 1.4 (2026-11-18): `migration043_integration_test.go` (`TestMigration043Constraints`, `TestMigration043Guard`) hijau terhadap dev DB, tapi dijalankan dari salinan di paket sementara (sudah dihapus) karena paket `internal/repository` belum compile saat sqlc 2.5 mengubah tipe (`vendor_pks_admin_repository.go` = pekerjaan task 3–4). Jalankan ulang di paketnya sendiri setelah task 4 selesai. `migration022_integration_test.go` (insert `vendor_pks` tanpa `pks_type_id`/`wod_number`) akan gagal sesudah 043 — perlu disesuaikan di task 4.4/8.
- Task 8 Checkpoint Gate Kualitas (2026-11-19):
  - Backend: `cd backend && go test ./...` PASS; `golangci-lint run --build-tags integration ./...` PASS (0 issues; `ErrVendorPksWodTaken` diberi `//nolint:staticcheck` untuk singkatan "No. WOD").
  - Backend integration: `go test -tags integration ./internal/service ./internal/handler ./internal/repository` PASS (terhadap dev DB localhost).
  - OpenAPI: `go test ./internal/handler -run OpenAPI` PASS (route coverage `/admin/pks-types` dan `/admin/vendors/{vendorID}/pks`, response spec, reject drift).
  - Backend-exq: `cd backend-exq && go test ./...` PASS; `golangci-lint run --build-tags integration ./...` PASS (0 issues; `pgDate` helper dihapus); integration tests PASS.
  - Pkg: `cd pkg && go test ./...` PASS; `golangci-lint run --build-tags integration ./...` PASS (0 issues).
  - Frontend: `pnpm --dir frontend/CompanyPortal-Vite run test` PASS (214 test files / 2.043 tests hijau); `pnpm run lint` PASS (0 errors); `tsc --noEmit` PASS (0 errors); `pnpm run build` PASS (initial JS 200.7 KB <= 300 KB budget).
  - Coverage: `backend/internal/service` = 83.2% (≥ 80%), `backend/internal/handler` = 82.2% (≥ 80%).
  - Portal Isolation: `git grep -E "pks-types|/pks" frontend/VendorPortal-Vite/` = 0 matches (VendorPortal terbukti tidak memanggil endpoint PKS admin/internal).
- Cek browser manual: outstanding (user).

### Catatan eksekusi
- Task 2.5 (2026-11-18): `sqlc generate` (v1.31.1) hijau di `backend/` dan `backend-exq/`; `UserLeafe` → `UserLeave` diperbaiki di `backend/internal/db/{approval.sql.go,models.go}` dan `backend-exq/internal/repository/models.go`. `go build ./...` MERAH, sesuai harapan — pekerjaan caller untuk task 3–6:
  - `backend/internal/repository/vendor_pks_admin_repository.go` (baris 43, 48, 57–58): `GetActiveVendorPks` kini butuh `GetActiveVendorPksParams{VendorID, PksTypeID}` dan mengembalikan `GetActiveVendorPksRow`; `GetVendorPksByID` mengembalikan `GetVendorPksByIDRow`; `GetPendingVendorPksChange` diganti `GetPendingVendorPksChanges` (banyak) → task 4.1. Paket di atasnya (service/handler) belum bisa dicek sampai repo ini hijau.
  - `backend-exq/internal/exportjob/producers.go` (211): `ExportVendorsWithPksBatchParams` tak punya `Today`/`RenewalMonths` lagi → task 6.x.
  - `backend-exq/internal/exportjob/producers_report.go` (±275–302, 10 error): `ExportVendorPksBatchParams` tak punya `AfterID`/`AfterName`; `ExportVendorsWithPksBatchRow` tak punya `PksName`/`EffectiveDate`/`ExpiryDate`/`ValidityMonths`; `x.PksName` kini `string` bukan `*string` → task 6.x.
- Task 7.1 (2026-11-18): `gen:api` → `src/lib/api/schema.d.ts` diperbarui. `pksApi.ts`: tipe `PksType`/`CreatePksTypePayload`/`UpdatePksTypePayload` (dari schema) + `listPksTypes`/`submitPksType`/`updatePksType`/`disablePksType`/`enablePksType` (`/admin/pks-types`). `types.ts`: `VendorPksSummary` = `{pks_type, pks_type_name, pks_status, remaining_days|null}`, `AdminVendor.pks?: VendorPksSummary[]` (field `pks_status` tunggal dibuang), `AdminVendorsListParams.pks_type` (berpasangan dengan `pks_status`), alias `VendorPksGroup`/`VendorPksPending`. `pnpm run test`: 210 file / 2012 test hijau (schema stale-check ikut). `tsc` merah, sesuai harapan: error di komponen yang ditulis ulang 7.2–7.4 — `PksFormDialog.tsx` (69, payload tanpa `pks_type`/`wod_number`), `PksPanel.tsx` (109, 165, detail kini `groups[]`), `VendorsTable.tsx` (59–80, `pks` array / `pks_status` hilang); test lama `VendorListPks.test.tsx` memakai bentuk lama (dibereskan 7.5). Error tsc lain di file test (CommandPalette, ModuleCard, admin-atms, dll.) sudah ada sebelumnya, bukan dari task ini. `pnpm build` tidak dijalankan (akan gagal di tsc sampai 7.2–7.4 selesai).
- Task 5.1 (2026-11-18): `ListVendorPksSummaries.remaining_days` scanned a nullable value into `int32` (sqlc does not infer expression nullability) → changed to `COALESCE(..., 0)::int` in `queries/vendors_admin.sql` + `internal/db/vendors_admin.sql.go` (hand-synced, no regen); the service maps `belum_diisi` → `remaining_days: null`. Service gained `ResolvePksFilterType` + `PksSummaries`; repo gained `GetPksTypeByCode` + `ListPksSummaries` (replica). Stubs added to the fakes in `vendor_admin_test.go` / `admin_vendor_handler_test.go` so they still implement the interfaces; old list/PKS-status tests (`vendor_admin_list_pks_integration_test.go`, handler tests on `pks`/`pks_status`) are task 5.2. `openapi.yaml` list item/params (`pks_type`, `pks[]`) not touched — task 4.3 owns that file. `go build` of `service` still red only from 4.1/4.2 files (`vendor_pks_admin.go:43`, `masterdata_applier_vendor_pks.go:95`); 5.1 files not yet compiled together.

- Task 5.2 (2026-11-18): test daftar vendor — `service/vendor_admin_list_pks_test.go` (baru: `ResolvePksFilterType`, `PksSummaries`, `List`/`Count` mengisi Today/RenewalMonths), `service/vendor_admin_list_pks_integration_test.go` (ditulis ulang: Property 6 filter = status tampil + `Count`, Property 7 satu entry per type aktif urut code / `belum_diisi` + days nil / INTERNAL kosong + tidak cocok filter / type nonaktif tidak jadi entry; `rapid` 100 iterasi, tx rollback), `service/vendor_admin_test.go` (fake repo punya `pksTypeByCodeFunc`/`pksSummariesFunc`), `handler/admin_vendor_handler_test.go` (pasangan `pks_type`+`pks_status` → 400, type nonaktif 400, error lookup 500, bentuk `pks[]` tanpa `pks_status` lama, `PksTypeID` sampai ke List+Count). Hijau: unit service + handler (`AdminVendorHandler_List*`) dan integration `VendorAdminListPks` di `localhost` (PASS, 100 tests). Paket `internal/service` dan `internal/handler` BELUM bisa `go test ./...` utuh karena file test task lain masih lama: `service/vendor_pks_admin_test.go` (4.4, `db.GetPendingVendorPksChangeRow`), `service/masterdata_applier_vendor_pks_integration_test.go:91` (4.4), `service/*cash_count*_integration_test.go` + `masterdata_applier_pks_type_integration_test.go` (helper `changeStatus`/`countInt` — hanya terlihat saat dikompilasi tanpa file lain, bukan error asli), `handler/openapi_contract_test.go:337` (4.3, `service.VendorPksDetail.PksStatus`). Dijalankan dengan daftar file eksplisit tanpa file-file itu.
- Task 4.1 (2026-11-18): `vendor_pks_admin.go` + `internal/repository/vendor_pks_admin_repository.go` (caller fix dari 2.5; repo kini `Active(vendor, type)`, `Pending` many, `PksType`, `PksTypeByCode`, `WodTaken`). `go build ./internal/repository/` + `go vet` hijau. Paket `internal/service` hanya tersisa error di `masterdata_applier_vendor_pks.go` (task 4.2). Untuk 4.2/4.3/4.4:
  - Payload create: key JSON `replaced_pks_id` (sesuai design; field Go tetap `ReplacesPksID`), plus `pks_type_id`, `wod_number`. Terminate/unterminate: `{vendor_id, pks_type_id, reason}`. `Before` = `db.GetVendorPksByIDRow` → `CurrentState` applier harus `GetVendorPksByID` (bentuk sama).
  - Sentinel baru `ErrVendorPksWodTaken` (409 field `wod_number`) — dipakai applier 4.2 dan handler 4.3. Kode type tidak dikenal/nonaktif = `*ValidationError` field `pks_type` (422).
  - `Submit` memetakan semua `23505` ke `ErrMasterDataChangePending`; `SubmitNew` mengecek ulang `WodTaken` saat itu supaya race index pending-WOD jadi `ErrVendorPksWodTaken`.
  - `Get` → `{vendor_id, kind, today, renewal_months, groups:[{pks_type:{code,name}, pks_status, active, history, pending}]}`; `VendorPksRecord` + `wod_number`. Field top-level `pks_status`/`active`/`history`/`pending` dihapus (handler 4.3 + OpenAPI).
  - `vendor_pks_admin_test.go` (unit, mock repo lama) belum diperbarui → task 4.4.
- Task 3.2 (2026-11-18): `PksTypeApplier` + registry `"vendor_pks_type"` di `cmd/api/main.go` (route belum di-mount, task 3.3). Error apply: create `23505` → `ErrPksTypeConflict`; update/enable/disable 0 baris (atau `LockPksType` tanpa baris) → `ErrMasterDataChangeStale` (change `stale`); disable dipakai → `ErrPksTypeInUse` (apply gagal, di-wrap `%w` — handler approve 3.3/approval perlu memetakannya ke 409 `pks_type_in_use`, saat ini jatuh ke error umum apply). `CurrentState` = `GetPksType` (`db.VendorPksType`, sama dengan `Before` dari service 3.1). `go build ./...` hijau; `go test ./internal/service` belum bisa dikompilasi karena `vendor_pks_admin_test.go` (task 4.4); tes applier/integrasi = task 3.4.
- Task 4.2 (2026-11-18): applier create = `LockVendorForPks` → `LockPksTypeShare` (tidak ada / `is_active=false` → stale) → `GetActiveVendorPks(vendor, type)` vs `replaced_pks_id` → mark replaced → `InsertVendorPks` (+ `pks_type_id`, `wod_number` di-trim) → `SetVendorPksReplacedBy`. `23505` `vendor_pks_wod_uq` → `ErrVendorPksWodTaken`; `vendor_pks_active_uq` → stale. disable/enable dan `CurrentState` tidak berubah (per id). `go build ./...` (backend) hijau. `go vet`/`go test ./internal/service/` belum bisa: `vendor_pks_admin_test.go` masih memakai `db.GetPendingVendorPksChangeRow` + `Active(ctx, vendor)` lama → task 4.4. Pemetaan `ErrVendorPksWodTaken` → 409 saat approve ada di jalur approval/handler (bukan file ini), dicek di 4.4.

- Task 4.4 (2026-11-18): test PKS per type. Unit: `service/vendor_pks_admin_test.go` ditulis ulang (type/WOD/pending per type, `replaced_pks_id` hanya type yang sama, `Before` = `GetVendorPksByIDRow`). Integration (`-tags integration`, `localhost`): `masterdata_applier_vendor_pks_integration_test.go` (Property 2 + satu aktif per type via `rapid`; Property 1 paralel same-type 1 applied/1 stale, beda type keduanya applied; Property 5 type nonaktif stale + submit 422; Property 3 WOD case/trim/riwayat/pending di apply dan submit; Property 8 pending per type; stale `replaced_pks_id`; reject tak mengubah apa pun; maker ≠ checker; RBAC 403 di `Submit`), `vendor_pks_pending_integration_test.go` (race submit satu pending per type). Test lama yang ikut diperbarui supaya paket utuh hijau: `handler/openapi_contract_test.go` (fixture `groups`) + `handler/openapi_contract_pks_type_test.go` (baru: kasus respons `/admin/pks-types`, route coverage — spec 3.3 sebelumnya tanpa kasus kontrak), `service/masterdata_import_export_header_test.go` (header ekspor vendor kini 7 kolom; header lama 16 kolom jadi legacy), `service/masterdata_export_roundtrip_integration_test.go` (template 7 kolom), `repository/migration022_integration_test.go` (PKS butuh `pks_type_id` + WOD; cek default `is_director` menggantikan hitung baris dev). Hasil: `go vet ./...` dan `go vet -tags integration ./...` hijau; `go test ./...` (backend) hijau; `go test -tags integration ./internal/service ./internal/handler ./internal/repository` hijau terhadap dev DB. `go test ./internal/handler -run OpenAPI` ikut hijau (fixture dan kasus kontrak diperbarui; 4.3 tetap tidak dicentang di sini). Cek browser manual: outstanding (user). Temuan untuk user (kode produksi, di luar task test): `parseEntityImportCSV` (`masterdata_import.go`) memilih legacy header `[1]` (id,code + 9 kolom read-only) tetapi memanggil `readImportCSV` dengan read-only 3 kolom → file legacy 18 kolom itu kini ditolak sebagai header tidak sesuai; tidak ada test yang menutup kasus ini setelah header ekspor berubah.
- Task 7.3 (2026-11-19): frontend CompanyPortal-Vite: `VendorsTable` grup kolom dinamis per active PKS type (urut code: `Status` badge ikon + teks, `Monitoring` tabular-nums "N hari"/"lewat N hari"/"-"; vendor INTERNAL "-", vendor tanpa PKS badge netral "Tidak ada"); kolom tunggal PKS lama dihapus; `VendorFilterBar` dengan select `Type PKS` dan `Status PKS` (`Status PKS` disabled saat type kosong, perubahan type/status mengupdate pasangan atau mereset keduanya berpasangan dengan `page: 1`); `useAdminVendorsUrlState` skema zod refine validasi pasangan `pks_type` + `pks_status`; `DataTable` diperbarui mendukung `colSpan` header grup; `AdminVendorsPage` menghubungkan `usePksTypesList` dan `exportFlow`; unit test 34 file / 324 test di `src/features/admin-vendors` hijau; `biome check .` lolos 0 error. Cek browser manual: outstanding (user).
- Task 7.4 (2026-11-19): frontend CompanyPortal-Vite: `features/admin-vendors/`: `PksPanel` menampilkan satu kartu per type (`VendorPksGroup`: Active_PKS dengan No. WOD tabular-nums, judul, tanggal efektif, expired, masa berlaku, monitoring sisa hari tabular-nums, status badge ikon + teks; riwayat PKS dengan No. WOD; pending change per type); tombol terminate dan batalkan terminate bertindak pada PKS aktif milik type tersebut dan dimatikan hanya saat type tersebut memiliki change pending atau vendor nonaktif; `PksFormDialog` dengan select `Type PKS` aktif (via `usePksTypesList`) dan No. WOD wajib (teks bebas 1-100 karakter), validasi 409 WOD duplikat & 422 dipetakan ke field terkait; navigasi keyboard & label aksesibel screen reader (`aria-invalid`, `aria-describedby`); aturan dilarang menggunakan em dash (`—`) di UI copy dipatuhi; TypeScript check (`tsc`) & Biome linter lolos 0 error; 34 test file / 326 test di `src/features/admin-vendors` hijau. Cek browser manual: outstanding (user).
- Task 7.5 (2026-11-19): frontend CompanyPortal-Vite testing suite verification: Seluruh test unit/integrasi Vitest + React Testing Library (RTL) terverifikasi 100% hijau: tabel type (`PksTypesTable`, disable tombol nonaktif jika `in_use`), kolom grup dinamis per type aktif + badge netral "Tidak ada" untuk `belum_diisi` (`VendorListPks`), filter berpasangan wajib `pks_type` + `pks_status` di URL state & UI (`useAdminVendorsUrlState`, `VendorKindFilter`), `PksFormDialog` (select active type + input No. WOD, mapping error HTTP 409 duplicate WOD & 422 ke field), `PksPanel` per type (kartu per type, tombol terminate/unterminate per type), aksesibilitas keyboard dan aria attributes (`aria-invalid`, `aria-describedby`), dan schema stale-check (`schema.test.ts`). Tes pratinjau ekspor (`PreviewTable.test.tsx`) disesuaikan untuk badge "Tidak ada" pada label "Belum diisi". Hasil akhir frontend: 214 test files / 2.043 tests lolos hijau (100%), `biome check .` 0 error, `tsc --noEmit` 0 error, dan `pnpm run build` sukses (budget gzip terpenuhi). Cek browser manual: outstanding (user).
- Task 8 (2026-11-19): Gate kualitas menyeluruh backend, backend-exq, pkg, dan frontend: Seluruh test unit dan integrasi lolos hijau. Staticcheck ST1005 pada `ErrVendorPksWodTaken` di-suppress via `//nolint:staticcheck` (singkatan nama resmi "No. WOD"); helper dead-code `pgDate` di `backend-exq/internal/exportjob/producers_vendor_pks_test.go` dibersihkan. Linter `golangci-lint` di `backend`, `backend-exq`, dan `pkg` lolos 0 issue. Kontrak OpenAPI (`-run OpenAPI`) lolos 100% termasuk coverage rute baru `/admin/pks-types` dan `/admin/vendors/{vendorID}/pks`. Coverage paket internal backend yang disentuh: `internal/service` = 83.2% (≥ 80%), `internal/handler` = 82.2% (≥ 80%). Isolasi `VendorPortal` diverifikasi bersih dari rute PKS admin (0 temuan). Build frontend lolos dalam budget ukuran chunk. Cek browser manual: outstanding (user).
- Task 9 (2026-11-19): Dokumentasi & graph synchronization selesai: `.claude/CLAUDE.md` Sec 3 (pencatatan tabel `vendor_pks_types` & multi-PKS type) dan Sec 12 (baseline migrasi `043_vendor_pks_type.sql` applied, migrasi bebas berikutnya `044`); `.claude/docs/data-map.md` (anchor `<a id="vendor-pks-type-migration-043"></a>` memuat skema `vendor_pks_types`, perluasan `vendor_pks` dengan `pks_type_id` & `wod_number`, index parsial unik WOD & active PKS, serta penyesuaian ekspor); `.claude/docs/decisions.md` (dokumentasi lengkap Q1–Q9 & DD1–DD10, menandai model PKS tunggal di D2/D24/D27 dan kolom PKS di `vendor-export-pks` sebagai superseded); `.claude/development-progress.md` dan `.agents/development-progress.md` (status fitur PKS per type dinaikkan ke tahap Testing/Deploy); `graphify update .` dijalankan dan sukses memperbarui AST knowledge graph (18.373 node, 42.410 edge, 987 komunitas). Cek browser manual: outstanding (user).

## Task Dependency Graph

- **Wave 0:** 1.1 (migrasi).
- **Wave 1:** 1.2, 1.3 (butuh 1.1 + OK user), 2.1, 2.2, 2.3, 2.4 (file query berbeda).
- **Wave 2:** 2.5 (sqlc, butuh 2.1–2.4), 1.4 (butuh 1.3).
- **Wave 3:** 3.1, 4.1, 5.1, 6.1, 6.2 (butuh 2.5; file berbeda).
- **Wave 4:** 3.2 (butuh 3.1; `main.go` registry), 4.2 (butuh 4.1), 6.3 (butuh 6.1, 6.2).
- **Wave 5:** 3.3 (butuh 3.2; `main.go` mount + `openapi.yaml`).
- **Wave 6:** 4.3 (butuh 4.1, 3.3; `openapi.yaml`), 3.4 (butuh 3.3), 5.2 (butuh 5.1), 6.4 (butuh 6.3).
- **Wave 7:** 4.4 (butuh 4.2, 4.3), 7.1 (butuh 4.3, 5.1, 6.3).
- **Wave 8:** 7.2, 7.3 (butuh 7.1; folder berbeda).
- **Wave 9:** 7.4 (butuh 7.3; folder `admin-vendors` sama).
- **Wave 10:** 7.5 (butuh 7.2–7.4).
- **Wave 11:** 8 (gate, butuh semua di atas).
- **Wave 12:** 9 (dokumentasi).
- **Wave 13:** 10, 11 (user).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3", "2.1", "2.2", "2.3", "2.4"] },
    { "id": 2, "tasks": ["2.5", "1.4"] },
    { "id": 3, "tasks": ["3.1", "4.1", "5.1", "6.1", "6.2"] },
    { "id": 4, "tasks": ["3.2", "4.2", "6.3"] },
    { "id": 5, "tasks": ["3.3"] },
    { "id": 6, "tasks": ["4.3", "3.4", "5.2", "6.4"] },
    { "id": 7, "tasks": ["4.4", "7.1"] },
    { "id": 8, "tasks": ["7.2", "7.3"] },
    { "id": 9, "tasks": ["7.4"] },
    { "id": 10, "tasks": ["7.5"] },
    { "id": 11, "tasks": ["8"] },
    { "id": 12, "tasks": ["9"] },
    { "id": 13, "tasks": ["10", "11"] }
  ]
}
```
- Task 3.3 (2026-11-18): `internal/repository/pks_type_repository.go` (list on replica, rest on primary), `internal/handler/admin_pks_type_handler.go` (GET/POST `/`, PUT `/{id}`, POST `/{id}/disable|enable`; 202/404/409/422/403), mounted at `/api/v1/admin/pks-types` in `main.go`, paths + `PksTypeID` param + schemas (`CreatePksTypeRequest`, `UpdatePksTypeRequest`, `PksTypeList`) in `openapi.yaml`. Extra scope (user-approved): `approval_handler.go` maps `service.ErrPksTypeInUse` (apply-time disable while in use) → 409 instead of 500. `go build ./...` hijau di `backend`. `go test ./internal/handler -run OpenAPI` BELUM bisa jalan: paket test handler tidak compile (`admin_vendor_handler_test.go:249` memakai field `Pks*` lama di `ListVendorsAdminRow`; `openapi_contract_test.go:337` memakai `VendorPksDetail.PksStatus/Active` lama) — tests milik task 4.3/5.2/4.4. Kontrak OpenAPI untuk path baru belum terverifikasi; ulangi `-run OpenAPI` setelah test lama diperbarui.
- Task 7.2 (2026-11-19): Implementasi `features/admin-pks-types/` (types, api, hooks, PksTypesTable, PksTypeFormDialog, AdminPksTypesPage, index), route `/settings/admin/pks-types` (role `ADMIN`/`ADMIN_PARAM`), registrasi di `routeTree.ts`, kartu menu "Type PKS" di `SettingsHubPage.tsx`, serta entitas `vendor_pks_type` di `pending.ts` & `changeDiff.ts`. Tabel menampilkan tabular-nums untuk code, status dengan icon + label teks (tidak hanya warna), flag "Dipakai", dan tombol nonaktifkan dimatikan dengan alasan teks eksplisit ("Sedang digunakan oleh PKS vendor") jika `in_use = true`. Maker-checker 202 memberikan feedback "Perubahan diajukan dan menunggu persetujuan (#...)". Form menangani validasi uppercase code, nama, dan mapping error 409/422 ke field. Aksesibilitas keyboard dan screen-reader terpenuhi. Semua 30 test di `features/admin-pks-types`, `routes/settings/admin/pks-types.test.ts`, dan `SettingsHubPage.test.tsx` lolos hijau. Biome check & format hijau untuk file baru/terkait. Verifikasi browser manual dicatat sebagai `outstanding (user)`.
