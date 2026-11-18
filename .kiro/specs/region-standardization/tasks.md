# Implementation Plan: Standarisasi Region

> Status: accepted 2026-11-18 (engineer, "Setuju, mulai")

## Overview

Urutan: migrasi `042` dulu (data + skema), terapkan ke dev setelah request berjalan di-clear, lalu query + sqlc, lalu backend Region admin dan cabang vendor, lalu fixture integration test yang ikut terdampak FK/drop kolom, lalu CompanyPortal. Checkpoint gate kualitas, dokumentasi, dan cek browser manual menutup spec.

Integration test berjalan terhadap DB dev (`DATABASE_URL`), jadi `042` harus sudah diterapkan ke dev (task 1.4) sebelum test integration apa pun yang menyentuh cabang vendor dijalankan ulang.

Pola yang dirujuk:
- Migrasi data satu transaksi + blok ROLLBACK di header: `backend/migrations/040_vendor_vaults_revamp.sql`.
- Region admin (deviasi GR#3, audit dalam tx): `backend/internal/service/region_admin.go`, `internal/handler/admin_region_handler.go`.
- Cabang vendor maker-checker: `internal/service/vendor_branch_admin.go`, `masterdata_applier_vendor_branch.go`; CSV `masterdata_import_validate.go`, `masterdata_import_confirm.go`, `pkg/csvexport/entities.go`.
- sqlc v1.31.1 + fix `UserLeafe` (`backend/CLAUDE.md`).

### Model Split

**Sonnet-runnable**
- 1.2 Script rollback — Sonnet, Medium (needs 1.1)
- 1.3 Integration test hasil migrasi — Sonnet, High (needs 1.4)
- 2. Query + sqlc — Sonnet, Medium (needs 1.1)
- 3.2 Repo + handler Region — Sonnet, Medium (needs 2.3, 3.1)
- 3.3 Test Region admin — Sonnet, Medium (needs 3.2, 1.4)
- 4.1 Submit cabang vendor — Sonnet, Medium (needs 2.3)
- 4.3 CSV cabang vendor — Sonnet, Medium (needs 2.3)
- 4.4 Test cabang vendor — Sonnet, High (needs 4.1–4.3, 1.4)
- 5. Fixture integration test lama — Sonnet, Medium (needs 1.4, 2.3)
- 6. Frontend — Sonnet, Medium (6.3 needs 6.1, 6.2)
- 7. Checkpoint gate kualitas — Sonnet, Medium (needs 1–6)
- 8. Dokumentasi — Haiku, Low (needs 7)
- 9. Cek browser manual (user) — Haiku, Low (needs 8)

**Opus-required**
- 1.1 Migrasi `042` — Opus, High — migrasi data + drop kolom + FK pada DB berisi data; peta region dipakai nomor request, tier vault, Area ACM
- 1.4 Terapkan ke dev — Opus, High — urutan clear data dev + uji guard + `migrate up`, setiap langkah dengan OK user
- 3.1 Service Region (format kode, disable dalam tx + lock) — Opus, High — kunci baris yang bersaing dengan apply cabang vendor
- 4.2 Applier cabang vendor (`FOR SHARE` region) — Opus, High — jalur apply maker-checker master data

**Dependensi lintas grup**
- Sonnet 2.x butuh Opus 1.1 (sqlc membaca file migrasi).
- Sonnet 1.3, 3.3, 4.4, 5 butuh Opus 1.4 (DB dev sudah `042`).
- Sonnet 3.2 butuh Opus 3.1 (error + interface repo). Sonnet 4.4 butuh Opus 4.2.

## Tasks

- [x] 1. Migrasi `042_region_standardization.sql`
  - [x] 1.1 Tulis `backend/migrations/042_region_standardization.sql`
    - Header komentar (spec `.kiro/specs/region-standardization/`, region-mapping.md accepted 2026-11-18, forward-only) + `BEGIN;` / `COMMIT;` + blok `-- ROLLBACK (documented, forward-only shop):` seperti `040`.
    - Guard 1: `vendor_requests.status NOT IN ('completed','cancelled','failed')` > 0 → `RAISE EXCEPTION` dengan jumlahnya.
    - Temp table `ON COMMIT DROP`: `m_region` (13 baris A), `m_code` (34 kode B, INDTIM → INDTIM sebagai default), `m_branch` (override per `branch_code`: 51 cabang C, 16 cabang INDTIM Kalimantan → `KAL`, 3 cabang `JKT` D), `m_city` (22 kota B, dengan kode region tujuan).
    - Guard 2: kode `vendor_branches`/`acm_area_regions`/`cash_count_area_regions`/`cash_count_pics` yang tidak tercakup; cabang aktif ber-kode NULL tanpa override; dua kode lama → satu kode baru di Area ACM/area cash count berbeda → `RAISE EXCEPTION` menyebut nilainya.
    - `migration_042_backup (entity, entity_id, payload jsonb, PK (entity, entity_id))` diisi sebelum perubahan.
    - Update `regions` by id; `CREATE TABLE region_cities` sesuai design (`## Data Models`) + index + trigger `set_updated_at` + `COMMENT ON` + seed dari `m_city`; update `vendor_branches.region_code` (override per cabang dulu, lalu per kode); remap + lebur `acm_area_regions`, `cash_count_area_regions`, update `cash_count_pics.region_code`.
    - `ADD CONSTRAINT vendor_branches_region_code_fk FOREIGN KEY (region_code) REFERENCES regions(code) ON UPDATE RESTRICT ON DELETE RESTRICT`; `DROP COLUMN vendor_branches.region`.
    - Tidak menyentuh `atms`, `locations`, `vendor_requests`, `vendor_request_tickets`, `vendor_request_number_seq`. Tidak menulis `audit_logs`.
    - _Requirements: 1.1, 1.2, 1.5, 2.1, 2.2, 2.3, 3.1, 3.2, 4.1, 5.1, 5.2, 5.4, 8.1, 8.2, 8.3_
  - [x] 1.2 Tulis `backend/scripts/dev/rollback_042_region_standardization.sql`
    - Isi = blok ROLLBACK header 1.1: tambah kembali `vendor_branches.region`, pulihkan `regions`, `vendor_branches`, `acm_area_regions`, `cash_count_*` dari `migration_042_backup`, drop FK + `region_cities` + backup, lalu `DELETE FROM schema_migrations WHERE version = '042_region_standardization.sql'`. Satu transaksi.
    - _Requirements: 8.3_
    - _Model: Sonnet, Medium — kebalikan langsung dari 1.1 dengan data backup._
  - [x] 1.3 Integration test hasil migrasi
    - `backend/internal/repository/migration042_integration_test.go` (`//go:build integration`): `region_cities` ada + 22 baris, setiap kota merujuk region yang ada; FK `vendor_branches_region_code_fk` ada; kolom `vendor_branches.region` tidak ada; tidak ada cabang aktif ber-kode NULL; setiap `regions` aktif non-uji cocok `^[A-Z0-9]{2,10}$`; insert cabang dengan kode tak dikenal → 23503; `migration_042_backup` berisi semua `vendor_branches`.
    - **Property 2**; Property 1/7/9 diverifikasi di 1.4 (butuh keadaan sebelum migrasi).
    - _Requirements: 1.5, 2.1, 2.3, 3.1, 3.2, 4.1, 8.3_
    - _Model: Sonnet, High — test skema + data dengan pola integration yang ada._
  - [x] 1.4 Terapkan ke DB dev (setiap langkah dengan OK user)
    - Catat sebelum: jumlah `atms` per `locations.region_id`, daftar Unfinished_Vendor_Request (dev 2026-11-18: 10), request selesai beserta `request_number`/`region_code`.
    - Uji guard (Property 7): `go run ./cmd/migrate up` selagi request belum di-clear → harus gagal; `migrate status` tetap pending `042`, data tidak berubah.
    - Clear Unfinished_Vendor_Request di dev (daftar ditunjukkan dulu ke user; hapus anak-anaknya: tiket, plan/alokasi/tiket vault, party, realisasi, notifikasi terkait).
    - `go run ./cmd/migrate up` → `status` tanpa MISMATCH.
    - Bandingkan sesudah: jumlah ATM per `region_id` sama (Property 1); request selesai tidak berubah (Property 9). Catat hasil di `## Notes`.
    - _Requirements: 1.1, 5.1, 5.4, 8.1_
  - _Model: Opus, High — migrasi data + drop kolom + FK pada DB berisi data; peta dipakai nomor request, tier vault, dan Area ACM._

- [x] 2. Query + sqlc
  - [x] 2.1 `backend/queries/regions_admin.sql`
    - `ListRegionsAdmin` + `GetRegionAdminByID`: ganti `COUNT(l.id) AS location_count` (+ `LEFT JOIN locations`, `GROUP BY`) dengan subquery `atm_count` dan `vendor_branch_count` (aktif saja) sesuai design.
    - Baru: `LockRegionForUpdate :one` (`SELECT … FROM regions WHERE id = $1 FOR UPDATE`), `CountActiveVendorBranchesByRegionCode :one`. `CountActiveLocationsByRegion` tetap.
    - _Requirements: 6.1, 7.1, 7.2, 7.3_
  - [x] 2.2 Pembaca region cabang vendor
    - `vendor_request.sql`: `flm_vendor_region` = `r.region` lewat `LEFT JOIN regions r ON r.code = vb.region_code`; filter nama pakai `r.region`; `no_region` = `vb.region_code IS NULL`; `ListDistinctVendorBranchRegions` dari `regions`.
    - `vendor_vaults_admin.sql`, `master_data_export.sql`: kolom `region` dari `r.region`.
    - `vendor_branches_admin.sql`: hapus `region` dari insert/update/select; baru `FindActiveRegionByCode :one`, `LockActiveRegionByCodeForShare :one` (`… AND is_active FOR SHARE`).
    - _Requirements: 3.3, 3.4, 4.2, 4.3, 4.4_
  - [x] 2.3 `sqlc generate` (v1.31.1) + fix `UserLeafe` → `UserLeave`; `git diff --stat backend/internal/db/` hanya berisi file dari 2.1/2.2 + `models.go` (struct `VendorBranch` tanpa `Region`, struct `RegionCity` baru). Perbaiki compile error di pemakai `Region` yang hilang hanya sejauh build lulus (logika di task 3–4).
    - _Requirements: 4.1, 4.2_
  - _Model: Sonnet, Medium — SQL-first dengan pola query yang ada; drop kolom membuat sqlc menemukan pembaca yang terlewat._

- [x] 3. Region admin backend
  - [x] 3.1 `internal/service/region_admin.go`
    - Pola kode bersama `^[A-Z0-9]{2,10}$` (setelah normalisasi) untuk `regionCodeRe`; `branchRegionCodeRe` di `vendor_branch_admin.go` merujuk pola yang sama.
    - `ErrRegionHasActiveVendorBranches`; `RegionAdminRepo` + `CountActiveVendorBranchesByRegionCodeTx`, `CountActiveLocationsTx`, `LockForUpdateTx`.
    - `setActive(false)`: semua cek di dalam tx — lock baris region, hitung lokasi (`ErrRegionHasActiveLocations`), hitung cabang aktif (`ErrRegionHasActiveVendorBranches`), lalu update + audit. Enable tidak berubah.
    - _Requirements: 1.3, 1.4, 7.1, 7.2, 7.3_
    - _Model: Opus, High — kunci baris yang bersaing dengan apply cabang vendor (Property 4)._
  - [x] 3.2 Repo + handler
    - `internal/repository/region_admin_repository.go`: method tx baru; List tetap `dbRead`.
    - `internal/handler/admin_region_handler.go`: JSON `atm_count`, `vendor_branch_count`, tanpa `location_count`; `ErrRegionHasActiveVendorBranches` → 409 `conflict` dengan pesan error.
    - _Requirements: 6.1, 7.2_
    - _Model: Sonnet, Medium — lapisan tipis di atas service._
  - [x] 3.3 Test
    - Service unit (repo palsu, table-driven): format kode (`JKTBAR` ok; `JKT_WEST`, `A`, 11 karakter, `jkt-1` ditolak 422), disable ditolak karena lokasi / cabang aktif tanpa audit, cabang nonaktif tidak menghalangi, RBAC ditolak.
    - Handler: JSON field baru, 409 cabang aktif.
    - Repo integration: `atm_count`/`vendor_branch_count` hanya menghitung yang aktif.
    - **Property 5**, **Property 6**.
    - _Requirements: 1.3, 6.1, 7.1, 7.2_
    - _Model: Sonnet, Medium — test tabel terhadap aturan yang sudah ditetapkan._

- [x] 4. Cabang vendor backend
  - [x] 4.1 `internal/service/vendor_branch_admin.go` + repo cabang vendor
    - Hapus `Region` dari payload create/update; pastikan decoder handler tidak menolak field `region` dari client lama (tanpa `DisallowUnknownFields`).
    - Submit: kode non-NULL bukan region aktif → `ValidationError{Field: "region_code"}` 422, tidak ada staging.
    - _Requirements: 3.3, 3.6, 4.1_
    - _Model: Sonnet, Medium — validasi tambahan pada alur submit yang ada._
  - [x] 4.2 `internal/service/masterdata_applier_vendor_branch.go`
    - Di tx apply: `LockActiveRegionByCodeForShare`; tidak ada → `ErrVendorBranchRegionInactive`, jalur `MarkApplyFailed` → approve 409. Kode NULL dilewati.
    - _Requirements: 3.4_
    - _Model: Opus, High — jalur apply maker-checker master data + lock lawan disable region._
  - [x] 4.3 CSV cabang vendor (`masterdata_import_validate.go`, `masterdata_import_confirm.go`)
    - Dry-run: `region_code` non-kosong bukan region aktif → error field `region_code`; `<keep>` tetap.
    - Confirm: nilai kolom `region` diabaikan; header + legacy header `pkg/csvexport/entities.go` tidak berubah.
    - _Requirements: 3.5, 4.4_
    - _Model: Sonnet, Medium — perluasan validasi CSV dengan pola yang ada._
  - [x] 4.4 Test
    - Unit: submit region aktif/nonaktif/tidak ada/NULL; CSV dry-run kode salah; confirm mengabaikan `region`.
    - Integration: approve change cabang setelah region di-disable → apply gagal 409; disable region selagi apply cabang memegang `FOR SHARE` → salah satu gagal (**Property 4**); rekomendasi Vendor Request, daftar vault, ekspor CSV menampilkan `regions.region` (**Property 8**); **Property 3**.
    - _Requirements: 3.3, 3.4, 3.5, 4.2, 4.3, 4.4_
    - _Model: Sonnet, High — test konkurensi + beberapa pembaca._

- [x] 5. Fixture integration test lama
  - Cari semua `*_integration_test.go` (backend, backend-exq, pkg) dan test Python yang `INSERT`/`UPDATE` `vendor_branches.region` atau `region_code`; ubah: hapus kolom `region`, insert region dengan kode yang dipakai cabang (format `^[A-Z0-9]{2,10}$`), bersihkan di cleanup. Diketahui: `vendor_request_number_integration_test.go`, `forecast_query_integration_test.go`, `vendor_request_cit2_filter_integration_test.go`, `atm_visit_quota_integration_test.go`, `vendor_request_summary_integration_test.go`, `backend_python/dsr_late_notify/test_job_integration.py`.
  - Jangan memperbaiki cleanup `_, _ =` di sini (bug fix terpisah, lihat Notes).
  - _Requirements: 3.1, 4.1_
  - _Model: Sonnet, Medium — perubahan fixture mekanis._

- [x] 6. Frontend CompanyPortal
  - [x] 6.1 `features/admin-regions`: `types.ts` (`atm_count`, `vendor_branch_count`), `RegionsTable.tsx` kolom "Jumlah ATM" + "Jumlah Cabang Vendor" rata kanan `tabular-nums`, tanpa "Jumlah Lokasi"; pesan 409 tampil.
    - _Requirements: 6.2, 6.3, 7.2_
  - [x] 6.2 `features/admin-vendors`: `vendorBranchFormSchema.ts`, `VendorBranchFormDialog.tsx`, `VendorBranchEditPage.tsx` — hapus input "Wilayah"; "Kode Region" = select region aktif (`GET /api/v1/admin/regions?status=active&page_size=100`, label `kode — nama`), kosong boleh; payload tanpa `region`. `BranchesPanel.tsx`/`VaultsPanel.tsx` tidak berubah.
    - _Requirements: 3.3, 4.2_
  - [x] 6.3 Test vitest: RegionsTable kolom baru; form cabang tanpa Wilayah + select region + payload; update test yang ada (`AdminRegionsPage.test.tsx`, `AdminRegions.test.tsx`, test form cabang).
    - _Requirements: 3.3, 6.2_
  - _Model: Sonnet, Medium — komponen React dengan pola dialog master data dan `rules/frontend-ui.md`._

- [x] 7. Checkpoint gate kualitas
  - `cd backend && go test ./...` dan `go test -tags integration ./...` (0 SKIP), `backend-exq` + `pkg` `go test ./...`, `golangci-lint run --build-tags integration ./...` (backend, backend-exq, pkg), `go test ./internal/handler -run OpenAPI`, `migrate status` tanpa MISMATCH, `pnpm --dir frontend/CompanyPortal-Vite run test|lint|build`, `backend_python` unittest yang disentuh di task 5. Hasil ke `## Notes` → `### Test Results`.
  - _Requirements: semua_
  - _Model: Sonnet, Medium — menjalankan gate dan triase kegagalan._

- [x] 8. Dokumentasi
  - CLAUDE.md Sec 3 (`region_cities`, FK `vendor_branches.region_code`, drop `vendor_branches.region`, migrasi `042` + next free `043`) dan Sec 12 (catatan baseline migrasi); `.claude/docs/data-map.md` (`region_cities`, `migration_042_backup` sementara); `.claude/docs/decisions.md` (Q1–Q10, M1–M4); `.claude/development-progress.md`.
  - _Requirements: 8.4_
  - _Model: Haiku, Low — edit dokumen sesuai hasil yang sudah ada._

- [ ] 9. Cek browser manual (user)
  - Halaman Region: kolom Jumlah ATM + Jumlah Cabang Vendor, disable region yang masih dipakai cabang → pesan 409. Form cabang vendor: select region, tanpa Wilayah. Rekomendasi Vendor Request: filter FLM Vendor Region berisi nama baku.
  - _Requirements: 3.3, 4.2, 6.2, 7.2_
  - _Model: Haiku, Low — dijalankan user; agen hanya mencatat hasil._

## Notes

### Keputusan yang tercermin
- Q1 (wilayah besar + kota): 1.1. Q2/Q3/Q6 (format, nama ESQ, rename sekali): 1.1, 3.1. Q4/Q8 + M1–M4 (pemetaan): 1.1. Q5a (FK `region_code`): 1.1, 4.1, 4.2. Q5b (drop `region`): 1.1, 2.2, 4.1, 4.3, 6.2. Q7 (clear request dev): 1.4. Q9 (aktif saja): 2.1, 3.3. Q10 (disable ditolak cabang aktif): 2.1, 3.1. Status akhir Vendor Request `completed`/`cancelled`/`failed` (gate design): 1.1.

### Di luar spec ini
- Bug fix terpisah (`.claude/bugfixes.md`): hapus sisa data uji `ITESTNUM` di dev (16 region, 16 lokasi, 15 ATM) dan perbaiki cleanup `vendor_request_number_integration_test.go` yang mengabaikan error (`_, _ =`). Tidak memblokir 042; sebaiknya sebelum 1.4 supaya angka halaman Region bersih.
- FK `acm_area_regions` / `cash_count_*` → `regions.code`: follow-up bila diinginkan.

### Deploy
- Belum ada prod (CLAUDE.md Sec 10). Kalau prod hidup sebelum rilis, Q7 dibuka lagi.
- `042` drop `vendor_branches.region`: `backend` versi lama tidak boleh berjalan setelah migrasi. Urutan: stop `backend` → `migrate up` → start `backend` baru.

### Definition of Done
- Semua leaf task dicentang, gate task 7 hijau, dokumentasi task 8, cek browser task 9 oleh user.
- Cabang vendor tetap maker-checker; region admin tetap audit dalam tx; tidak ada hard delete master data; list di replica, tulis di primary.

### Catatan eksekusi
- Task 1.1 (2026-11-18): dry-run di dev dalam transaksi yang di-ROLLBACK (guard request berjalan dilewati karena 10 request dev belum di-clear): 13 region baku; 407 cabang, aktif per region BALNUS 14, INDTIM 24, JABAR 40, JATENG 54, JATIM 61, JKTBAR 22, JKTPST 22, JKTSEL 20, JKTTIM 32, JKTUTR 15, KAL 23, SUMATERA 78, TGR 2; cabang aktif tanpa kode 0; 22 kota; `acm_area_regions` 34 → 12 baris (TGR tanpa Area ACM — ADMIN perlu memetakan); backup regions 29 / vendor_branches 407 / acm 34. Setelah rollback: kolom `vendor_branches.region` masih ada, `region_cities` tidak ada. Backup `migration_042_backup` memakai kunci `entity_key text` (bukan `entity_id bigint` seperti di design) karena `acm_area_regions`/`cash_count_area_regions` tidak punya id.
- Task 6.1–6.3 (2026-11-18): `RegionsTable` kolom Jumlah ATM + Jumlah Cabang Vendor (`meta.align: "right"` → DataTable memberi `text-right tabular-nums`); `createRegionSchema` + `maxLength` input kode ikut aturan baru 2–10 huruf/angka (pasangan task 3.1). Form cabang: input Wilayah dihapus, "Kode Region" = `RegionCodeSelect` (komponen baru `admin-vendors/components/RegionCodeSelect.tsx`, memakai `useRegionsList` dari `admin-regions/hooks`; select terdaftar baru dipasang setelah daftar region dimuat supaya nilai tersimpan tidak jatuh ke ""; kode tersimpan yang tidak aktif tetap tampil sebagai "(nonaktif)"). Payload create/update tanpa `region`. `vitest run src/features/admin-vendors src/features/admin-regions`: 37 file, 355 test lulus; `tsc -p tsconfig.app.json --noEmit` bersih; `biome check` 93 file bersih. Belum jalan terhadap backend baru (task 3.2 belum ada `atm_count`/`vendor_branch_count`).
- Task 1.2, 2.1, 2.2 (2026-11-18): rollback script dengan guard `migration_042_backup` harus ada. `regions_admin.sql`: subquery `atm_count`/`vendor_branch_count` (tanpa `GROUP BY`), `CountActiveVendorBranchesByRegionCode`, `LockRegionForUpdate`. `vendor_branches_admin.sql`: List memakai `LEFT JOIN regions r` untuk nama `region`; **Get tidak memuat nama region** (snapshot stale-check tidak boleh berubah karena region di-rename); Create/Update tanpa `region`; `FindActiveRegionByCode`, `LockActiveRegionByCodeForShare`. `vendor_request.sql`: 3 query forecast `LEFT JOIN regions vbr`, `no_region` = `vb.region_code IS NULL`, `ListDistinctVendorBranchRegions` dari `regions`. `vendor_vaults_admin.sql` (List + Get) dan `master_data_export.sql` (ekspor cabang + vault) `LEFT JOIN regions rg`. Temuan untuk task berikut: (a) `backend-exq/sqlc.yaml` ikut mengompilasi `../backend/queries/master_data_export.sql` + skema `../backend/migrations/` → task 2.3 harus `sqlc generate` di **dua modul**; (b) `GetVendorVaultAdminByID` (snapshot change vault) kini membawa nama dari `regions`, jadi change vault pending ikut stale bila nama region diubah (dan saat 042 mengganti nama); (c) fixture integration yang menulis `vendor_branches.region` (task 5) juga ada di `atm_assignment_source_integration_test.go`, `atm_assignment_vendor_apply_integration_test.go`, `masterdata_applier_cash_count_pic_integration_test.go`, `masterdata_export_scoped_roundtrip_integration_test.go`, `masterdata_import_cash_count_pic_integration_test.go`.
- Task 1.4 (2026-11-18, dev, OK user per langkah): Unfinished_Vendor_Request ternyata **10** (5 request lama `VR-…` tanpa vendor, id 1–5, + id 28, 774, 927, 928, 7545); daftar pertama hanya 5 karena query memakai `JOIN vendors`. Uji guard: `migrate up` gagal `region std 042: 10 unfinished vendor requests`, `042` tetap pending, skema tidak berubah (**Property 7**). Kesepuluh request di-soft-cancel (`status='cancelled'`, `is_canceled=true`, `cancellation_reason` menyebut 042; status lama disimpan di scratchpad sesi `cancel_042_prev_status.txt`; tidak ada yang dihapus) — termasuk `REP-ABA-MDN-20261014-001` yang dulu dipakai untuk cek browser kirim-ke-vendor. Change `vendor_branch`/`vendor_vault` pending sebelum apply = 0. `migrate up` → `1 applied`; `status` 41 applied, 0 pending, tanpa MISMATCH. Sesudah: jumlah ATM per `locations.region_id` identik dengan sebelum (**Property 1**); 13 region baku aktif; cabang aktif tanpa kode 0; 22 kota; `acm_area_regions` 12 baris; FK ada; `migration_042_backup` 470 baris. Request selesai sebelum migrasi = 0, jadi Property 9 tidak punya data uji di dev (042 memang tidak menyentuh `vendor_requests`). **Sejak titik ini `backend` di working tree tidak cocok dengan skema dev sampai task 2.3–4 selesai.**
- Task 2.3 (2026-11-18): `sqlc generate` v1.31.1 di `backend` + `backend-exq`, `UserLeafe` → `UserLeave` di `backend/internal/db/models.go`, `approval.sql.go`, `backend-exq/internal/repository/models.go`. File yang berubah: `master_data_export`, `models`, `regions_admin`, `vendor_branches_admin`, `vendor_request`, `vendor_vaults_admin` (+ `vendors_admin.sql.go`, yang sudah termodifikasi sebelum sesi ini — filter `kind` milik perubahan user yang belum di-commit). **Efek samping:** sqlc membaca `CREATE TEMP TABLE` di 042 sebagai tabel, jadi `models.go` di kedua modul kini memuat struct mati `MAcm`, `MBranch`, `MCcArea`, `MCity`, `MCode`, `MRegion` (tidak dipakai kode). Perbaikan bersih = bungkus pembuatan temp table di 042 dalam blok `DO`/`EXECUTE`; karena 042 sudah applied di dev, itu butuh rollback → edit → `migrate up` lagi (keputusan user). Compile fix minimal: applier cabang tanpa `Region`; handler Region JSON `atm_count`/`vendor_branch_count` (bagian task 3.2); `GET` detail cabang tanpa `region`; test handler/repo Region pakai field baru. `go build ./...` + `go vet -tags integration ./...` bersih; `go test ./...` backend + backend-exq lulus.
- Struct mati dirapikan (2026-11-18, OK user "Rapikan sekarang"): `rollback_042_region_standardization.sql` dijalankan di dev → `042` pending lagi, `vendor_branches` (`region` + `region_code`) identik byte-per-byte dengan snapshot sebelum 042 (md5), 13 kode lama kembali, `acm_area_regions` 34 baris, `region_cities` + backup hilang (**Req 8.3 terbukti di dev**). Lalu 042 diubah: keenam `CREATE TEMP TABLE` (termasuk 2 CTAS) dijalankan lewat `DO $$ BEGIN EXECUTE '…'; END $$;` supaya sqlc tidak membuat model; `migrate up` lagi → 41 applied, 0 pending; ATM per region identik, 13 region baku, cabang aktif tanpa kode 0, 22 kota, acm 12, backup 470. `sqlc generate` ulang dua modul (+ fix `UserLeafe`): struct `MAcm`…`MRegion` hilang, `Migration042Backup` + `RegionCity` tetap. Build/vet/unit test dua modul + `TestMigration042*` lulus.
- Wave 3 (2026-11-18): **3.1** `regionCodeRe = branchRegionCodeRe` (`^[A-Z0-9]{2,10}$` setelah normalisasi; pesan "kode harus 2-10 huruf/angka …"); `ErrRegionHasActiveVendorBranches`; `setActive(false)` kini membuka tx dulu lalu `ensureRegionUnused` (lock `FOR UPDATE` → hitung lokasi → hitung cabang aktif) sebelum update + audit. Property test region-management (`region_admin_property_test.go`) disesuaikan ke aturan 2–10. **3.2** repo `LockForUpdateTx`, `CountActiveLocationsTx`, `CountActiveVendorBranchesTx`; handler memetakan error baru ke 409 `conflict` (JSON `atm_count`/`vendor_branch_count` sudah di 2.3). **4.1** `VendorBranchAdminRepo.IsActiveRegion` (+ repo, primary); `ensureActiveRegion` di Create/Update → 422 `region_code` "region tidak dikenal atau nonaktif"; field `Region` dihapus dari payload + body handler (client lama yang masih kirim `region` tidak ditolak — decoder tanpa `DisallowUnknownFields`). **4.2** `lockActiveRegion` (`LockActiveRegionByCodeForShare`) di apply create/update → `ErrVendorBranchRegionInactive` (jalur apply-failure). **4.3** query `ImportActiveRegionCodes` (`master_data_import.sql`, sqlc ulang backend saja), `MasterDataImportRepo.ActiveRegionCodes`, `env.regions` dimuat untuk impor cabang; dry-run menolak kode bukan region aktif (`<keep>` dan kosong lolos); confirm mengabaikan kolom `region`. `go build` + `go vet -tags integration` bersih, `go test ./...` backend lulus, `backend-exq` build. Test baru untuk perilaku ini = task 3.3 / 4.4.
- Wave 4–5 (2026-11-18): **5** helper `service/region_fixture_integration_test.go` (`itRegionCode`, `seedTestRegion`; region yang dibuat lewat pool dicatat lalu dihapus oleh `TestMain` setelah semua test paket selesai dan hanya bila tidak dipakai — `t.Cleanup` LIFO bisa jalan sebelum cleanup cabang fixture; region run lain, mis. `ITESTNUM`, tidak disentuh). Fixture diubah: `atm_assignment_source`, `atm_assignment_vendor_apply` (`seedVendorWide`), `atm_visit_quota`, `forecast_query`, `vendor_request_summary` (`seedBranchPackage`), `masterdata_export_scoped_roundtrip`, `vendor_request_number` (`ITEST`), `vault_plan` (`ITEST`/`ITOTHER`), `vendor_request_ticket` (`ITESTB`), cash count area/PIC applier + impor PIC, `region_admin_integration_test` (tag 3 karakter, prefix disingkat ke aturan 2–10), `repository/dsr_late_report_repository_integration_test` (region dari `dsr_late_cases.json` dibuat di tx), `backend-exq` `report_producers_integration_test` (region `MDN` bernama `T61 Region` + langkah cleanup-nya), Python `dsr_late_notify/test_job_integration.py` (region dibuat di tx). **3.3/4.4** unit: `TestVendorBranchAdminService_RegionMustBeActive` (Property 3), `TestImport_VendorBranchRegionMustBeActive`, `TestAdminRegionHandler_Disable_HasActiveVendorBranches_409`; integration `service/region_standardization_integration_test.go`: `TestRegionStd_CountsAndDisableGuards` (Property 5, 6), `TestRegionStd_ApplierRejectsInactiveRegion` (Req 3.4), `TestRegionStd_DisableWaitsForBranchApply` (Property 4: disable menunggu `FOR SHARE` apply, lalu ditolak 409), `TestRegionStd_ReadersUseRegionName` (Property 8). **Bug ditemukan test baru dan diperbaiki:** dry-run CSV cabang masih menghitung perubahan kolom `region` sebagai update → `importEnv.validateRow` kini menimpa `region` dengan nilai saat ini untuk cabang (Req 4.4). Hasil: `go test ./...` backend + backend-exq lulus; integration backend + exq: semua test region-standardization dan fixture terdampak lulus; tidak ada region/vendor uji tersisa di dev setelah run. **Kegagalan integration yang tersisa sudah ada sebelum spec ini (bukan region):** `TestMigration022Schema` (8 `vendor_pics` direktur di dev dari `seed_vendor_contacts.sql`), `TestExportRoundTrip_VendorExportHeader_ZeroChanges` (header CSV vendor 16 vs 18 kolom — ikut perubahan vendor admin yang belum di-commit), `backend-exq` P11/P33 (fixture masih memakai `vendor_vaults.min_capacity_amount` yang di-rename migrasi 040); Python `test_recipients_idempotent_and_readable_by_go` (4 vs 3, bergantung data dev, sudah tercatat di vendor-pks).
- Task 1.3 (2026-11-18): `backend/internal/repository/migration042_integration_test.go` — `TestMigration042Schema`, `TestMigration042Data`, `TestMigration042RegionCodeFK` lulus terhadap dev.
- Deploy 1.4: change `vendor_branch` yang pending saat 042 diterapkan akan stale saat di-approve (snapshot memuat `region`, `updated_at` tersentuh) — hitung dulu.

### Test Results
- 2026-11-18, dev, task 7:
  - `golangci-lint run --build-tags integration ./...`: backend 0 isu, backend-exq 0 isu, pkg 0 isu.
  - `go test ./...`: backend, backend-exq, pkg lulus.
  - `go test -tags integration -count=1 -p 1 ./...` backend + backend-exq: semua test spec ini lulus — `TestMigration042Schema/Data/RegionCodeFK` (Property 2; Req 1.5, 2, 3.1, 4.1, 8.3), `TestRegionStd_CountsAndDisableGuards` (Property 5, 6), `TestRegionStd_ApplierRejectsInactiveRegion` (Req 3.4), `TestRegionStd_DisableWaitsForBranchApply` (Property 4), `TestRegionStd_ReadersUseRegionName` (Property 8), region admin integration, dan semua fixture terdampak (task 5). Gagal di luar spec ini (sudah ada sebelumnya, lihat Catatan eksekusi wave 4–5): `TestMigration022Schema`, `TestExportRoundTrip_VendorExportHeader_ZeroChanges`, backend-exq P11/P33.
  - Unit: `TestVendorBranchAdminService_RegionMustBeActive` (Property 3), `TestImport_VendorBranchRegionMustBeActive` (Req 3.5, 4.4), `TestAdminRegionHandler_Disable_HasActiveVendorBranches_409` (Req 7.2), `region_admin_property_test.go` (Req 1.3).
  - Property 1, 7, 9: diverifikasi saat 042 diterapkan ke dev (task 1.4: ATM per region identik, guard menolak saat ada request belum selesai, `vendor_requests` tidak disentuh); rollback dibuktikan di dev (Req 8.3).
  - `go test ./internal/handler -run OpenAPI`: backend + backend-exq lulus. `migrate status`: 41 applied, 0 pending, tanpa MISMATCH.
  - CompanyPortal: `biome check` 632 file bersih; `vitest run` 210 file / 2012 test lulus; `build` ok (initial JS 200.4 KB gzip dari budget 300 KB).
  - Python `dsr_late_notify.test_job_integration`: 8/9 lulus; 1 gagal lama (`test_recipients_idempotent_and_readable_by_go`, 4 vs 3, bergantung data dev).
  - Cek browser manual: **outstanding (user)** — task 9.

## Task Dependency Graph

- **Wave 0:** 1.1 migrasi; 6.1, 6.2 frontend (kontrak sudah ditetapkan design; file berbeda).
- **Wave 1:** 1.2 rollback (butuh 1.1); 1.4 terapkan ke dev (butuh 1.1, OK user); 2.1, 2.2 query (butuh 1.1; file berbeda).
- **Wave 2:** 2.3 sqlc (butuh 1.1, 2.1, 2.2); 1.3 test migrasi (butuh 1.4).
- **Wave 3:** 3.1 service Region; 4.1 submit cabang; 4.2 applier; 4.3 CSV (semua butuh 2.3; file berbeda).
- **Wave 4:** 3.2 repo + handler Region (butuh 3.1); 5 fixture lama (butuh 1.4, 2.3); 6.3 test frontend (butuh 6.1, 6.2).
- **Wave 5:** 3.3 test Region (butuh 3.2, 1.4); 4.4 test cabang (butuh 4.1–4.3, 1.4).
- **Wave 6:** 7 checkpoint (butuh semua task 1–6).
- **Wave 7:** 8 dokumentasi (butuh 7).
- **Wave 8:** 9 cek browser user (butuh 8).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "6.1", "6.2"] },
    { "id": 1, "tasks": ["1.2", "1.4", "2.1", "2.2"] },
    { "id": 2, "tasks": ["2.3", "1.3"] },
    { "id": 3, "tasks": ["3.1", "4.1", "4.2", "4.3"] },
    { "id": 4, "tasks": ["3.2", "5", "6.3"] },
    { "id": 5, "tasks": ["3.3", "4.4"] },
    { "id": 6, "tasks": ["7"] },
    { "id": 7, "tasks": ["8"] },
    { "id": 8, "tasks": ["9"] }
  ]
}
```
