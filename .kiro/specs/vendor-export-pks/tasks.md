# Implementation Plan

## Overview

Urutan: kontrak bersama di `pkg` dulu (kolom baca-saja di `pkg/csvexport`, sel PKS mentah di `pkg/reports`), lalu impor di `backend` dan query + producer di `backend-exq` berjalan paralel, lalu frontend, gate, dokumentasi, dan cek browser oleh user. Tanpa migrasi.

Pola yang diikuti:
- Legacy header + `LegacyFill` di `pkg/csvexport/entities.go` (cara kontrak CSV berubah tanpa merusak impor).
- `ExportVendorPksBatch` + `vendorPksRow` di `backend-exq` (COALESCE hasil fungsi, aturan sel kosong).
- `tableProducer` + `propCases` (pratinjau = file) dari spec `export-menu-revamp`.
- Test round-trip `masterdata_export_roundtrip_integration_test.go` dari spec `vendor-data-export`.

### Model Split

**Sonnet-runnable**
- 1. Kontrak `pkg/csvexport` + sel `pkg/reports` (1.1–1.4): Sonnet, Medium
- 2.2 Property + contoh test importer: Sonnet, Medium (butuh 2.1)
- 2.3 Round-trip integration + golden: Sonnet, Medium (butuh 2.1)
- 3. Query + producer `backend-exq` (3.1–3.5): Sonnet, Medium
- 4. Checkpoint backend: Sonnet, Medium (butuh 2.1)
- 5. Frontend (5.1–5.4): Sonnet, Medium
- 6. Checkpoint frontend: Sonnet, Medium
- 7. Dokumentasi: Haiku, Low (Haiku)
- 8. Cek browser manual oleh user: Haiku, Low (Haiku)

**Opus-required**
- 2.1 Importer menerima Export_Header dan membuang sel baca-saja: Opus, Medium. Zona bahaya: parser impor master data. Salah potong sel bisa membuat data master yang salah masuk ke alur approval.

Dependensi lintas grup: Sonnet 2.2, 2.3, dan 4 butuh Opus 2.1 selesai. Opus 2.1 butuh Sonnet 1.1.

## Tasks

- [x] 1. Kontrak bersama di `pkg`
  - [x] 1.1 `pkg/csvexport`: kolom baca-saja vendor
    - Tambah field `ReadOnlyColumns []string` di `Entity`, salin di `clone()`.
    - Tambah method `ExportHeader()` (salinan baru `Header` + `ReadOnlyColumns`) dan variabel `VendorReadOnlyColumns` (sembilan nama kolom, urutan design "Data Models").
    - Isi `importable[Vendors].ReadOnlyColumns`; entitas lain tetap `nil`. `Header` vendor tidak berubah.
    - _Requirements: 1.1, 2.6, 2.8_
  - [x] 1.2 Unit test `pkg/csvexport`
    - `ExportHeader` vendor = 9 + 9 kolom urut; mutasi hasil `Lookup`/`ExportHeader` tidak mengubah kontrak.
    - Entitas selain vendor: `ReadOnlyColumns` nil dan `ExportHeader() == Header`.
    - Header impor vendor (= header template) tetap 9 kolom yang sama.
    - _Requirements: 2.6, 2.8_
  - [x] 1.3 `pkg/reports`: `VendorPksDetailCells`
    - Fungsi baru yang mengembalikan sembilan sel mentah (tanpa CSVSafe): direktur nama/telepon/email, nama PKS, tanggal efektif, tanggal expired, masa berlaku, sisa hari, label status. Nil = sel kosong.
    - Sebelum refactor: tambah test golden yang membekukan keluaran `VendorPksCells` sekarang (direktur ada/tidak, PKS ada/tidak, INTERNAL, sisa hari negatif, teks berawalan `=`/`+`) dan pastikan hijau terhadap kode lama.
    - `VendorPksCells` dibangun ulang: CSVSafe atas `[code, name]` + CSVSafe atas `VendorPksDetailCells`. Test golden harus tetap hijau.
    - _Requirements: 1.2, 1.4, 1.5, 1.6, 6.3_
  - [x] 1.4 Test `pkg/reports` (Property 6)
    - Property 6 (rapid): untuk sembarang `VendorPksRow`, `VendorPksCells(r)[2:]` = CSVSafe atas `VendorPksDetailCells(r)`, dan dua sel pertama = CSVSafe atas kode dan nama.
    - Unit `VendorPksDetailCells`: sel mentah tanpa CSVSafe (teks berawalan `=` tidak diberi `'`), sel kosong untuk nil.
    - Test golden dari 1.3 dan test report `vendor_pks` yang ada tetap hijau.
    - _Requirements: 6.1, 6.3_
  - _Model: Sonnet, Medium — kontrak bersama dua modul dengan pola yang sudah ada; refactor `VendorPksCells` dijaga test golden._

- [x] 2. Impor vendor di `backend`
  - [x] 2.1 `readImportCSV` menerima Export_Header
    - `exportSpec` mendapat field `readOnly` dari `e.ReadOnlyColumns` (`spec()` di `masterdata_export.go`).
    - `readImportCSV(r, header, legacy, legacyFill, readOnly)`: urutan cek header = Import_Header → `header+readOnly` (hanya bila `readOnly` tidak kosong) → legacy.
    - Bila cocok Export_Header, baris dengan jumlah sel `len(header)+len(readOnly)` dipotong ke `cells[:len(header)]`. Baris dengan jumlah sel lain dibiarkan supaya ditolak pemeriksaan per baris yang ada.
    - Pesan header tidak cocok tetap menyebut Import_Header.
    - `DryRun` (`masterdata_import.go`) meneruskan `spec.readOnly`; konfirmasi memanggil `DryRun`, jadi tidak perlu perubahan lain. Pemanggil lain di test memakai `nil`.
    - `ExportVendorsBatch` dan `MasterDataExportRepository.Vendors` tidak disentuh.
    - _Requirements: 2.1, 2.2, 2.5, 2.8, 3.1_
    - _Model: Opus, Medium — parser impor master data; salah potong sel bisa memasukkan data yang salah ke alur approval._
  - [x] 2.2 Property dan contoh test importer (Property 4, 5)
    - Property 4 (rapid): body CSV vendor valid + sembilan sel baca-saja acak → dry-run file Export_Header sama dengan dry-run file Import_Header (records, errors, creates, updates, changes).
    - Property 5 (rapid): header vendor yang bukan Import_Header maupun Export_Header (kolom kurang, lebih, tertukar, berganti nama, hanya kolom baca-saja) → ditolak dengan pesan yang ada, tanpa baris diproses.
    - Contoh: file Export_Header tanpa edit → 0 perubahan; hanya sel baca-saja diedit → 0 perubahan; baris Export_Header dengan jumlah sel salah → error `jumlah kolom`; file Import_Header tetap diterima.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 3.1, 6.2_
  - [x] 2.3 Round-trip integration dan golden
    - Perluas `TestExportRoundTrip_DryRunIsZeroChanges` dan `TestMasterDataImport_RoundTripOnRealDatabase` dengan varian vendor Export_Header: sel baca-saja diisi data PKS/direktur dari DB dan diisi data acak → 0 create, 0 update.
    - Perbarui `masterdata_export_golden_test.go` bila golden mencetak header spec (tambah baris read-only, header impor tetap).
    - Pastikan template vendor (`csvexport.Lookup(Vendors).Header`) tetap 9 kolom.
    - _Requirements: 2.3, 2.4, 2.6, 6.2_
  - _Model: Opus, Medium untuk 2.1; Sonnet, Medium untuk 2.2 dan 2.3 — lihat Model Split._

- [x] 3. Query dan producer di `backend-exq`
  - [x] 3.1 Query `ExportVendorsWithPksBatch`
    - File baru `backend-exq/queries/vendor_export_pks.sql` sesuai design (kolom + predicate salinan `ExportVendorsBatch`, join salinan `ExportVendorPksBatch`, COALESCE hasil fungsi, keyset `id`).
    - Perbarui komentar `CountVendors` di `export_preview.sql` (twin = query baru).
    - `sqlc generate` (v1.31.1) di `backend-exq` saja; pastikan tidak ada diff di file generated lain.
    - _Requirements: 1.2, 1.3, 1.7, 4.3_
  - [x] 3.2 `vendorsProducer(forceAll, renewalMonths)`
    - Header = `exportHeaderOf(csvexport.Vendors)` (helper baru, `ExportHeader()`).
    - `fetch` memanggil `ExportVendorsWithPksBatch` dengan `Today: wibDate(reports.TodayWIB(asOf))` dan `RenewalMonths`.
    - `cells` = `csvexport.VendorCells(...)` + `reports.VendorPksDetailCells(pksDetailRow(...))`; ekstrak `pksDetailRow` dari `vendorPksRow` (`producers_report.go`) supaya aturan `ExpiryDate.Valid` / `PksStatus != ""` satu tempat.
    - `count` tetap `CountVendors`. `NewProducers` meneruskan `pksRenewalMonths` ke dua entri vendor.
    - `Querier`: ganti `ExportVendorsBatch` dengan `ExportVendorsWithPksBatch`; sesuaikan fake di `producers_test.go`, `producers_property_test.go`, `producers_report_test.go` secukupnya supaya paket tetap terkompilasi.
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8_
  - [x] 3.3 Unit dan property test producer (Property 1 unit, Property 3)
    - Property 1 (rapid, fake querier): sembilan sel terakhir = `VendorPksDetailCells` data yang sama, untuk kombinasi direktur ada/tidak, PKS ada/tidak, FLM/INTERNAL, setiap key status.
    - Property 3: perluas `propCases` "vendors" dan "vendors-forceAll" ke baris query baru; halaman pratinjau = isi file, setiap baris `len(Export_Header)` sel, count = baris.
    - Today WIB: `asOf` 23:30 WIB dan 00:30 WIB memberi tanggal yang benar ke query.
    - _Requirements: 1.1, 1.2, 1.4, 1.5, 1.6, 1.7, 1.8, 6.3_
  - [x] 3.4 Integration test DB nyata (Property 1 DB, Property 2)
    - Seed: vendor FLM dengan PKS aktif + PKS lama (replaced) + direktur; FLM tanpa PKS dan tanpa direktur; INTERNAL dengan direktur; satu vendor nonaktif.
    - Property 1 bagian DB: sel baca-saja (setelah CSVSafe) = sel 3–11 report `vendor_pks` untuk vendor, As_Of_Date, dan renewal months yang sama.
    - Property 2: sembilan sel pertama = `csvexport.VendorCells` baris `ExportVendorsBatch` dengan `id` sama, id dan urutan sama, untuk status `active`/`disabled`/`all` dan scope satu vendor.
    - Satu baris per vendor (PKS lama tidak muncul); INTERNAL tanpa status; FLM tanpa PKS = "Belum diisi".
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 2.3, 2.8, 6.1_
  - [x] 3.5 Akses, replica, dan audit tidak berubah
    - Handler integration: role selain `ADMIN`/`ADMIN_PARAM` mendapat 403 untuk pratinjau dan ekspor vendor (kedua kind); pratinjau tidak menulis audit.
    - Ekspor vendor menulis `export_requested` lalu `master_data_exported` dengan `row_count`, bentuk sama dengan sebelumnya.
    - Pratinjau dan worker membaca lewat reader replica (perilaku yang ada; pastikan test yang ada masih mencakup kind vendor).
    - _Requirements: 3.2, 4.1, 4.2, 4.3, 4.4_
  - _Model: Sonnet, Medium — query + producer mengikuti pola `ExportVendorPksBatch` dan `tableProducer`; kebenaran dijaga property dan integration test._

- [x] 4. Checkpoint backend
  - `go test ./...` di `pkg`, `backend`, `backend-exq`, termasuk `--build-tags integration` dengan `DATABASE_URL` (0 skip).
  - `golangci-lint run --build-tags integration ./...` per modul; `govulncheck` tidak memburuk.
  - `sqlc generate` di `backend-exq` tanpa diff; `backend` tidak perlu regenerasi.
  - `go test ./internal/handler -run OpenAPI` di `backend` dan `backend-exq` hijau (bentuk respons pratinjau tidak berubah).
  - Coverage Go `internal/*` tetap ≥ 80%.
  - _Requirements: 6.3, 6.4_
  - _Model: Sonnet, Medium — gate; butuh penilaian untuk triase kegagalan._

- [x] 5. Frontend CompanyPortal
  - [x] 5.1 `features/admin-vendors`: badge yang bisa dipakai ulang
    - Tambah `pksStatusFromLabel(label): PksStatus | undefined` (kebalikan label `PKS_BADGES`) di `PksStatusBadge.tsx`.
    - Ekspor `PksStatusBadge` dan `pksStatusFromLabel` dari `features/admin-vendors/index.ts`.
    - _Requirements: 5.2_
  - [x] 5.2 `features/exports`: dataset dan tabel pratinjau
    - `datasets.ts` dataset `vendors`: `numericColumns` = `id`, `pks_validity_months`, `pks_remaining_days`; deskripsi "Daftar vendor beserta kontak, alamat kantor pusat, Direktur Utama, dan PKS aktif." Filter tetap Status saja.
    - `PreviewTable.tsx` `buildColumns`: `pks_status` tidak kosong → `PksStatusBadge`; kosong → kosong; label tak dikenal → teks.
    - Route guard `/exports/$dataset` tidak diubah.
    - _Requirements: 4.5, 5.1, 5.2, 5.3, 5.4_
  - [x] 5.3 `features/master-data-io`: catatan impor vendor
    - `ImportWizard.tsx`: saat `entity === "vendors"`, tambah butir petunjuk langkah 1: "File ekspor vendor berisi kolom Direktur Utama dan PKS. Kolom itu hanya untuk dibaca dan diabaikan saat impor. Ubah PKS di tab PKS dan direktur di data PIC."
    - _Requirements: 2.7_
  - [x] 5.4 Test Vitest
    - `PreviewTable`: badge per label, sel kosong kosong, label tak dikenal teks, dua kolom angka rata kanan.
    - `pksStatusFromLabel`: setiap label bolak-balik; daftar label sama dengan `reports.PksStatuses` (disalin eksplisit di test).
    - `datasets.ts`: deskripsi, `numericColumns`, filter vendor tetap Status, peran dataset tetap `ADMIN`/`ADMIN_PARAM`.
    - `ImportWizard`: catatan muncul hanya untuk entitas vendor.
    - _Requirements: 2.7, 4.5, 5.1, 5.2, 5.3, 5.4_
  - _Model: Sonnet, Medium — komponen dan test mengikuti pola yang ada._

- [x] 6. Checkpoint frontend
  - `pnpm --dir frontend/CompanyPortal-Vite run lint`, `run test:coverage` (ambang 84/80/80/85), `run build` (+ `check-bundle.mjs`).
  - `pnpm run gen:api:exq` tanpa diff di `src/lib/api/exq-schema.d.ts`.
  - _Requirements: 6.3, 6.4_
  - _Model: Sonnet, Medium — gate; butuh penilaian untuk triase kegagalan._

- [x] 7. Dokumentasi
  - `development-plan.md` X.16: status built + commit; `.claude/development-progress.md`: entri fitur.
  - `.claude/docs/deployment.md`: urutan deploy `backend` sebelum/bersamaan `backend-exq` untuk rilis ini.
  - Tidak ada perubahan schema, jadi CLAUDE.md Sec 3 tidak berubah.
  - `graphify update .`; bila `graphify-human/features.toml` memetakan ekspor/impor master data, tambahkan file baru `vendor_export_pks.sql` lalu jalankan `py graphify-human/generate.py`.
  - _Requirements: 6.4_
  - _Model: Haiku, Low — edit dokumen mekanis._

- [ ] 8. Cek browser manual (user, GR#10)
  - `/exports/vendors` sebagai ADMIN dan ADMIN_PARAM: kolom direktur + PKS tampil, badge status, angka rata kanan, paging, filter Status.
  - Role lain tidak bisa membuka halaman.
  - Ekspor → unduh → buka di Excel: kolom baru terbaca, teks non-ASCII benar.
  - Impor file hasil ekspor tanpa edit → "Tidak ada perubahan untuk diimpor"; edit hanya kolom PKS → tetap tanpa perubahan; impor file ekspor lama (9 kolom) → diterima.
  - Halaman impor: catatan muncul saat entitas Vendor dipilih.
  - Log Audit: `master_data_exported` tercatat seperti sebelumnya.
  - _Requirements: 2.3, 2.4, 2.7, 4.1, 4.4, 4.5, 5.1, 5.2, 5.3, 5.4_
  - _Model: Haiku, Low — dijalankan user; agent hanya mencatat hasil._

## Notes

### Keputusan yang tercermin
- D1 (opsi a, kolom baca-saja di CSV vendor): 1.1, 2.1, 3.2. D2 (sembilan kolom): 1.1, 3.1. D3 (hanya PKS aktif): 3.1, 3.4. D4 (INTERNAL kosong): 1.3, 3.4. D5 (tanpa filter Status PKS): 5.2. D6 (role sama): 3.5, 5.2. D7 (catatan impor): 5.3. D8 (snake_case + label status): 1.1, 1.3, 5.2.
- DD1 (query baru khusus `backend-exq`): 3.1, 3.4. DD2 (`CountVendors` tetap): 3.1, 3.3. DD3 (`VendorPksDetailCells`): 1.3, 1.4. DD4 (`ReadOnlyColumns`): 1.1, 2.1. DD5 (label → badge): 5.1, 5.2. DD6 (pakai ulang badge): 5.1. DD7 (catatan statis): 5.3.

### Correctness Properties → task
- Property 1: 3.3 (unit), 3.4 (DB). Property 2: 3.4. Property 3: 3.3. Property 4: 2.2. Property 5: 2.2. Property 6: 1.4.

### Deploy
- `backend` (impor menerima Export_Header) dirilis sebelum atau bersamaan dengan `backend-exq` (file mulai berisi kolom baru).
- `VENDOR_PKS_RENEWAL_MONTHS` harus sama di dua backend (sudah wajib sejak `vendor-pks`).
- Tanpa migrasi.

### Definition of Done
- Semua task 1–7 dicentang dan gate 4 + 6 hijau; task 8 dicentang setelah user selesai cek browser.
- `ExportVendorsBatch` dan header impor vendor tidak berubah (cek `git diff` pada `backend/queries/master_data_export.sql` kosong).

### Catatan eksekusi
- 2.1 (keputusan user 2026-11-06): pada file Export_Header, setiap baris yang jumlah selnya bukan `len(header)+len(readOnly)` (18 untuk vendor), termasuk baris 9 sel, ditolak dengan error baris `jumlah kolom N, seharusnya 18` (`importRecord.badCount`). File Import_Header dan legacy tidak berubah.

## Task Dependency Graph

- **Wave 0:** 1.1, 1.3, 3.1, 5.1, 5.3. Tanpa dependensi; file berbeda.
- **Wave 1:** 1.2 (butuh 1.1), 1.4 (butuh 1.3), 2.1 (butuh 1.1), 3.2 (butuh 1.1, 1.3, 3.1), 5.2 (butuh 5.1).
- **Wave 2:** 2.2 (butuh 2.1), 3.3 (butuh 3.2), 5.4 (butuh 5.2, 5.3).
- **Wave 3:** 2.3 (butuh 2.1; setelah 2.2 supaya test `backend/internal/service` diserialkan), 3.4 dan 3.5 (butuh 3.2; file test berbeda dari 3.3), 6 (butuh 5.4).
- **Wave 4:** 4 (butuh 1.2, 1.4, 2.2, 2.3, 3.3, 3.4, 3.5).
- **Wave 5:** 7 (butuh 4 dan 6).
- **Wave 6:** 8 (butuh 7; dijalankan user).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.3", "3.1", "5.1", "5.3"] },
    { "id": 1, "tasks": ["1.2", "1.4", "2.1", "3.2", "5.2"] },
    { "id": 2, "tasks": ["2.2", "3.3", "5.4"] },
    { "id": 3, "tasks": ["2.3", "3.4", "3.5", "6"] },
    { "id": 4, "tasks": ["4"] },
    { "id": 5, "tasks": ["7"] },
    { "id": 6, "tasks": ["8"] }
  ]
}
```
