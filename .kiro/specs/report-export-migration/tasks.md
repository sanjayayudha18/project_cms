# Implementation Plan: Report Export Migration

## Overview

Spec ini dikerjakan **setelah `vendor-data-export` (X.7) di-deploy** (D8): `export_jobs` (migrasi `034`), worker, janitor, `KindSpecs` (D23 di X.7), `pkg/csvexport`, `ExportProgressDialog`, dan routing `/api/v1/exports/` ke `:8081` harus sudah ada.

Urutan mengikuti risiko yang sama dengan X.7: rekam golden CSV laporan lama dulu, lalu buat `pkg/reports` dan pindahkan `backend/` ke paket itu (layar tidak berubah), tambah migrasi `035`, bangun sisi `backend-exq` (sqlc, params, `KindSpecs`, producer, route), lalu frontend, dan terakhir cutover `format=csv` di `backend/`.

Referensi pola:
- Golden + kontrak bersama: task 1-3 `vendor-data-export`.
- Producer, `KindSpecs`, route `/api/v1/exports`: design `vendor-data-export` (Components, `backend-exq/internal/exportjob`).
- Handler laporan lama: `backend/internal/handler/{dsr_late_report,replenishment}_handler.go`.
- Kasus status DSR bersama Python: `backend/internal/repository/testdata/dsr_late_cases.json`.

### Model Split

**Sonnet-runnable**
- 1.1 Golden CSV laporan lama — Sonnet, Medium
- 2.1 Paket `pkg/reports` — Sonnet, Medium
- 2.2 Test + fuzz `pkg/reports` (Property 3, 7) — Sonnet, Medium
- 3.2 Test layar = file + regresi `backend/` (Property 2) — Sonnet, High (needs Opus 3.1)
- 4.1 Migrasi `035_export_jobs_reports.sql` — Sonnet, Medium
- 4.2 Test migrasi (Property 9) — Sonnet, Medium
- 5.1 sqlc `backend-exq` membaca SQL laporan — Sonnet, Low
- 5.2 `ReportParams`, `ParseReportParams`, hash — Sonnet, Medium
- 5.4 ReportProducer — Sonnet, High (needs Opus 5.3 untuk registry Kind)
- 5.5 `DSR_LATE_START_DATE` di `backend-exq` + `.env.example` tiga runtime — Sonnet, Low
- 5.6 Route `/exports/reports/*` + OpenAPI `backend-exq` — Sonnet, Medium (needs Opus 5.3)
- 6.1 Integrasi producer (Property 1, 7, 10) — Sonnet, High
- 6.2 Integrasi peran dan audit (Property 4, 5, 6) — Sonnet, High (needs Opus 5.3)
- 6.3 Integrasi parameter dan dedupe (Property 3, 8) — Sonnet, High
- 7.1 `features/exports` untuk laporan + halaman `/exports` — Sonnet, Medium
- 7.2 Tombol ekspor di layar DSR telat dan Replenishment — Sonnet, Medium
- 7.3 Test frontend (Property 12) — Sonnet, Medium
- 8.1 Cutover `format=csv` di `backend/` — Sonnet, Medium
- 8.2 OpenAPI `backend/` + regenerate tipe — Sonnet, Low
- 8.3 Test cutover (Property 11) — Sonnet, Medium
- 9 Checkpoint gate kualitas — Sonnet, Medium
- 10 Dokumentasi — Haiku, Low (Haiku)
- 11 Cek browser manual (user) — Haiku, Low (Haiku)

**Opus-required**
- 3.1 `backend/` memakai `pkg/reports` (peran, validasi, start date) — Opus, Medium — daftar peran layar laporan adalah guard RBAC; salah salin = akses laporan berubah
- 5.3 `KindSpecs` laporan + bentuk audit — Opus, High — auth per Kind tanpa membuka ekspor master data, audit `report_exported` harus tetap bisa dicari bersama riwayat lama

Dependensi lintas grup: Sonnet 3.2 menunggu Opus 3.1. Sonnet 5.4, 5.6, 6.2 menunggu Opus 5.3. Opus 3.1 menunggu Sonnet 2.1. Opus 5.3 menunggu Sonnet 2.1 dan 5.2.

## Tasks

- [ ] 1. Kunci CSV laporan lama sebelum disentuh
  - [x] 1.1 Rekam golden CSV empat laporan
    - _Catatan 2026-10-29: golden direkam dari fake reader di level handler (bukan fixture DB, sama dengan pilihan golden X.7), test `backend/internal/handler/report_csv_golden_test.go` (`-update` untuk merekam ulang), 6 file di `testdata/report_csv/`. Nama file dicek lewat `Content-Disposition`. Konsekuensi: 6.1 (Property 1) harus men-seed DB dengan baris yang menghasilkan view yang sama._
    - Fixture DB deterministik: vendor FLM + cabang + vault + `dsr_location_vault_maps` + `dsr_receipts` (OK, telat, tidak kirim, pending), `replenish_classifications` + `replenish_classification_amounts` (beberapa denom, selisih, `void`), nama dengan karakter non-ASCII dan sel berawalan `=`/`+`/`-`.
    - Jalankan handler lama dengan `format=csv` untuk `daily` (dengan dan tanpa `status`), `summary`, `amount-mismatch` (dengan dan tanpa `classification`), `trips`, dengan `now`/`as_of` tetap; simpan ke `backend/internal/handler/testdata/report_csv/` beserta nama file dari `Content-Disposition`.
    - Test integrasi yang membandingkan handler lama dengan golden (lulus sebelum perubahan apa pun).
    - _Requirements: 3.1, 3.3_
  - _Model: Sonnet, Medium — merekam output kode lama apa adanya._

- [ ] 2. Paket kontrak laporan `pkg/reports`
  - _Model: Sonnet, Medium — memindahkan kode murni apa adanya._
  - [ ] 2.1 Buat `pkg/reports`
    - Konstanta Report_Kind, `ReplenishmentMaxRangeDays`, `ReplenishmentExportLimit`, `DsrLateViewerRoles`, `ReplenishmentViewerRoles`, `DsrLateStatuses`, `ReplenishmentClasses`, `WIB`.
    - `ParseDsrLateStartDate`, `ParseDsrLateMonth`, `ValidateDsrLateMonth`, `ValidateReplenishmentRange` (pesan error sama dengan sekarang).
    - Tipe baris netral, `Header`, `DsrLateDailyCells`, `DsrLateSummaryCells`, `AmountMismatchLines`, `TripCells`, `FileName`; helper sel lama (`wibCell` dengan `t.In(WIB)`, `dateCell`, `str`, `int64Cell`, `intCell`, `itoa`) dipindah sebagai fungsi privat.
    - Penulisan file memakai `csvexport.Writer` (BOM + `CSVSafe`, D3).
    - _Requirements: 1.2, 1.6, 2.2, 3.1, 3.2, 3.3, 3.5, 3.6, 4.1_
  - [ ]* 2.2 Test + fuzz `pkg/reports`
    - Cells/Header/FileName terhadap golden 1.1 (baris data setelah BOM).
    - **Property 3: Validasi parameter setara** (fuzz validator terhadap tabel kasus dari kode lama, `now` acak).
    - **Property 7: Gerbang tanggal mulai DSR** (bagian `ParseDsrLateStartDate`).
    - _Requirements: 1.2, 1.3, 1.6, 2.2, 2.3, 3.1, 3.6_

- [ ] 3. `backend/` memakai kontrak bersama
  - [ ] 3.1 Ganti konstanta dan validator lokal dengan `pkg/reports`
    - `dsr_late_report.go`, `replenishment_read.go`, handler laporan, dan `cmd/api/main.go` memakai peran, status, kelas, rentang, batas, dan `ParseDsrLateStartDate` dari `pkg/reports`.
    - Daftar peran disalin persis (DSR tanpa BRANCH-ATM-USER, replenishment dengan BRANCH-ATM-USER); route guard dan cek service memakai variabel yang sama.
    - Cabang `format=csv` lama masih jalan (dihapus di task 8).
    - _Requirements: 3.5, 4.1, 9.1_
    - _Model: Opus, Medium — guard RBAC layar laporan; perubahannya mekanis tetapi salah salin mengubah akses._
  - [ ]* 3.2 Test layar = file dan regresi
    - **Property 2: Layar sama dengan file** (JSON `/daily` semua halaman, `/summary`, `/reports/{amount-mismatch,trips}` dipetakan lewat `pkg/reports` = baris golden).
    - Respons JSON semua endpoint laporan tidak berubah; `dsr_late_cases.json` tetap lulus.
    - _Requirements: 3.4, 3.5, 9.1, 9.2_
    - _Model: Sonnet, High — test lintas layar dan file, banyak kombinasi filter._

- [ ] 4. Migrasi `035_export_jobs_reports.sql`
  - _Model: Sonnet, Medium — DDL aditif pada tabel X.7; constraint sudah ditulis di design._
  - [ ] 4.1 Tulis dan terapkan migrasi
    - Cek nomor bebas (setelah `034_export_jobs.sql` X.7; `022` dicadangkan).
    - Kolom `report_params jsonb`, `kind_chk` diperluas, `status_filter` nullable, `export_jobs_params_kind_chk`, `scope_chk` baru, komentar kolom; persis seperti design.
    - `go run ./cmd/migrate up`; `migrate status` 0 pending.
    - _Requirements: 6.2, 6.3_
  - [ ]* 4.2 Test migrasi
    - **Property 9: Migrasi tidak merusak job lama** (baris X.7 sebelum/sesudah, kombinasi Kind × parameter yang salah ditolak).
    - _Requirements: 6.3_

- [ ] 5. Sisi `backend-exq`
  - [ ] 5.1 sqlc membaca SQL laporan bersama
    - `backend-exq/sqlc.yaml`: tambah `../backend/queries/dsr_late_report.sql` dan `../backend/queries/replenish_classifications.sql`; `sqlc generate`; tidak ada salinan SQL laporan di `backend-exq/queries/`.
    - _Requirements: 3.4_
    - _Model: Sonnet, Low — konfigurasi codegen._
  - [ ] 5.2 `ReportParams`, `ParseReportParams`, hash
    - Struct sesuai design, parsing query memakai validator `pkg/reports`, parameter yang tidak berlaku untuk Kind diabaikan, `ParamsHash` dari JSON kanonis, `entity` = nilai `report` lama.
    - _Requirements: 1.2, 1.3, 2.2, 2.3, 6.2_
    - _Model: Sonnet, Medium — validasi mengikuti paket bersama._
  - [ ] 5.3 `KindSpecs` laporan dan bentuk audit
    - Empat entri: peran dari `pkg/reports`, `DownloadAction = report_exported`, `EntityRef` tetap (`dsr_late_report` / `replenishment_report`, id 0).
    - `after` untuk `export_requested` dan `report_exported`: `report` lama, `filter` lengkap (termasuk `status` dan `classification`), `export_job_id`, `username`, `role`, `row_count` (unduh saja).
    - Kind master data tetap ADMIN/ADMIN_PARAM; cek peran saat unduh membaca `KindSpecs[job.Kind].Roles` dari DB (sudah ada di X.7).
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3, 5.4_
    - _Model: Opus, High — auth per Kind dan audit yang harus menyatu dengan riwayat lama._
  - [ ] 5.4 ReportProducer
    - Empat producer sesuai tabel design: `ListDsrLateDaily` berhalaman di dalam snapshot, `ListDsrLateSummary`, `ListReplenishClassifications` (`only_diff`, `final_only`, total ≤ 100.000) + `ListReplenishClassificationAmounts`, `ReportReplenishTrips`; `as_of` = waktu job mulai; `DSR_LATE_START_DATE` kosong = header saja.
    - Header, sel, nama file dari `pkg/reports`; nominal bilangan bulat apa adanya.
    - _Requirements: 1.4, 1.5, 2.4, 2.5, 3.1, 3.2, 3.3, 3.6, 6.1, 6.4_
    - _Model: Sonnet, High — paging di dalam snapshot dan ekspansi per denom harus sama persis dengan file lama._
  - [ ] 5.5 `DSR_LATE_START_DATE` di `backend-exq`
    - `cmd/api/main.go` memakai `reports.ParseDsrLateStartDate` (salah format = gagal start) dan meneruskannya ke producer DSR.
    - Komentar "harus sama" di `.env.example` `backend`, `backend-exq`, `backend_python` (`RETRY_DSR_LATE_START_DATE`).
    - _Requirements: 1.5, 1.6_
    - _Model: Sonnet, Low — wiring env._
  - [ ] 5.6 Route `/exports/reports/*` dan OpenAPI
    - Empat route POST dengan `RequireRoles` per daftar peran laporan, timeout 30 detik, error `pkg/response`; `/jobs*` X.7 tidak berubah.
    - `backend-exq/api/openapi.yaml`: path baru, `ExportJob.kind` diperluas, `report_params`; test kontrak X.7 lulus.
    - _Requirements: 1.1, 2.1, 4.1, 4.2, 8.4_
    - _Model: Sonnet, Medium — handler tipis mengikuti pola X.7._

- [ ] 6. Integrasi `backend-exq` dengan Postgres nyata
  - _Model: Sonnet, High — property berbasis DB dengan data acak ber-seed._
  - [ ] 6.1 Producer
    - **Property 1: File sama dengan export lama** (golden 1.1, setelah BOM); **Property 7: Gerbang tanggal mulai DSR** (env kosong = header saja); **Property 10: Snapshot konsisten** (data berubah di antara halaman).
    - _Requirements: 1.4, 1.5, 2.4, 2.5, 3.1, 3.2, 3.3, 3.6, 6.1, 6.4_
  - [ ] 6.2 Peran dan audit
    - **Property 4: Matriks peran** (14 peran × 8 Kind, termasuk master data tetap admin); **Property 5: Cek peran ulang saat unduh**; **Property 6: Bentuk audit laporan** (termasuk query `ListAuditLogs` dengan baris lama dari `backend/`).
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3, 5.4, 5.5, 9.3_
  - [ ] 6.3 Parameter dan dedupe
    - **Property 3: Validasi parameter setara** (lapisan HTTP, tanpa job/audit saat 400); **Property 8: Parameter tersimpan dan dedupe**.
    - _Requirements: 1.3, 2.3, 6.1, 6.2_

- [ ] 7. Frontend CompanyPortal
  - _Model: Sonnet, Medium — komponen React dengan pola X.7._
  - [ ] 7.1 `features/exports` untuk laporan dan halaman Ekspor Saya
    - `requestReportExport`, varian `{ kind: "report" }` di `useExportFlow`, label jenis laporan di `status.ts`, tautan dialog ke `/exports`.
    - `MyExportsPage` + route lazy `/exports` dengan peran gabungan laporan (tanpa item sidebar); "Minta ekspor lagi" memakai `report_params`.
    - Regenerate `exq-schema.d.ts` (`gen:api:exq`).
    - _Requirements: 6.2, 7.3, 7.5_
  - [ ] 7.2 Tombol ekspor di layar laporan
    - `DsrLateReportScreen`: "Ekspor CSV" -> `useExportFlow` dengan filter layar (status hanya tab detail). `ReplenishmentScreen`: "Selisih nominal (CSV)" dan "Trip per ATM (CSV)", tetap nonaktif saat rentang tidak valid.
    - 429 inline (ikon + teks) tanpa dialog; tautan Ekspor Saya.
    - Hapus `downloadReport` berbasis `fetch` mentah di kedua `api.ts`.
    - _Requirements: 7.1, 7.2, 7.4, 7.6, 8.5_
  - [ ]* 7.3 Test frontend
    - **Property 12: Tombol ekspor mengirim filter layar** (fast-check state filter acak); halaman `/exports`; tidak ada `fetch` mentah tersisa untuk CSV.
    - _Requirements: 7.1, 7.2, 7.4, 7.5, 7.6, 8.5_

- [ ] 8. Cutover export laporan di `backend/`
  - _Model: Sonnet, Medium — menghapus jalur lama setelah frontend pindah; urutan rilis penting._
  - [ ] 8.1 Hapus cabang `format=csv`
    - Empat endpoint mengembalikan 400 field `format` dengan pesan yang menunjuk menu ekspor; hapus `writeCSV`, helper sel lokal, `RecordExport`, `DailyAll`; respons JSON tidak berubah.
    - Golden 1.1 tetap dijaga oleh 3.2 dan 6.1.
    - _Requirements: 8.1, 8.2_
  - [ ] 8.2 OpenAPI `backend/` dan tipe
    - Hapus `DsrLateFormat` dan respons `text/csv` dari `/dsr-late-reports/{daily,summary}`; `pnpm run gen:api`; test kontrak `backend/` lulus.
    - _Requirements: 8.3_
  - [ ]* 8.3 Test cutover
    - **Property 11: Cutover** (`format=csv` = 400 tanpa audit/file; tanpa `format` JSON tidak berubah).
    - _Requirements: 8.1, 9.1_

- [ ] 9. Checkpoint: gate kualitas
  - `go test ./...` + `-tags integration` di `backend`, `backend-exq`, `pkg`; `golangci-lint`, `govulncheck`; coverage Go ≥ 80% `internal/*` dan `pkg/`.
  - CompanyPortal: `pnpm run lint`, `pnpm run test:coverage` (84/80/80/85), `pnpm run build`, `gen:api` + `gen:api:exq` bersih.
  - Python: `dsr_late_notify` test tetap lulus (SQL tidak berubah).
  - Ekspor master data X.7 tetap hanya ADMIN/ADMIN_PARAM; tidak ada library baru.
  - _Requirements: 9.2, 9.3, 9.4_
  - _Model: Sonnet, Medium — triase kegagalan gate._

- [ ] 10. Dokumentasi
  - `.claude/CLAUDE.md` Sec 3 + `data-map.md` (`export_jobs.report_params`), `rbac-role-feature-matrix.md` (route `/api/v1/exports/reports/*`, `/exports`, `format=csv` dihapus), `tech.md` (`pkg/reports`), `development-plan.md` X.8, `.claude/development-progress.md`.
  - Daftar cek deploy: nilai `DSR_LATE_START_DATE` sama di `backend`, `backend-exq`, `backend_python`.
  - _Requirements: 1.6, 6.3_
  - _Model: Haiku, Low — pembaruan dokumen._

- [ ] 11. Cek browser manual (user)
  - Sebagai ATM-SPV: ekspor DSR telat harian (filter status) dan rekap; sebagai BRANCH-ATM-USER: ekspor selisih nominal dan trip, DSR telat tidak tersedia; pop-up + unduh otomatis; Ekspor Saya di `/exports`; file dibuka di Excel (karakter non-ASCII benar).
  - Log Audit: filter `report_exported` menampilkan baris lama dan baru.
  - _Requirements: 4.1, 5.5, 7.1, 7.2, 7.3, 7.5_
  - _Model: Haiku, Low — dijalankan user (Golden Rule #10); agen hanya menyiapkan daftar cek._

## Notes

- **Prasyarat:** X.7 (`vendor-data-export`) sudah di-deploy, termasuk `KindSpecs` (D23 di X.7). Jangan mulai task 4-8 sebelum itu.
- Golden 1.1 **harus** direkam sebelum task 3 dan 8 menyentuh `backend/`.
- Task 8 hanya dirilis bersama task 7 (pola D15 X.7). Rollback = deploy ulang image `backend` dan CompanyPortal sebelumnya.
- SQL laporan tidak diubah. Bila suatu saat diubah, `backend_python/dsr_late_notify/queries.py` dan `dsr_late_cases.json` ikut diperbarui (di luar scope ini).
- Task bertanda `*` adalah test opsional tetapi direkomendasikan; task 6.x (peran, audit, golden) tidak opsional.

### Definition of Done

- Task 1-10 selesai, gate 9 hijau, task 11 dicentang user.
- Property 1-12 punya test yang lulus.
- `development-plan.md` X.8 diperbarui ke "built".

## Task Dependency Graph

- **Wave 0:** 1.1 (golden), 4.1 (migrasi, butuh X.7 `034`).
- **Wave 1:** 2.1 (butuh 1.1 sebagai acuan), 4.2 (butuh 4.1), 5.1 (sqlc, independen).
- **Wave 2:** 2.2 (butuh 2.1), 3.1 (butuh 2.1), 5.2 (butuh 2.1, 5.1), 5.5 (butuh 2.1).
- **Wave 3:** 3.2 (butuh 3.1), 5.3 (butuh 5.2).
- **Wave 4:** 5.4 (butuh 5.1, 5.3, 5.5).
- **Wave 5:** 5.6 (butuh 5.3, 5.4).
- **Wave 6:** 6.1, 6.2, 6.3 (butuh 5.6, 4.1).
- **Wave 7:** 7.1 (butuh 5.6).
- **Wave 8:** 7.2 (butuh 7.1).
- **Wave 9:** 7.3 (butuh 7.2), 8.1 (butuh 7.2, 3.2).
- **Wave 10:** 8.2 (butuh 8.1), 8.3 (butuh 8.1).
- **Wave 11:** 9 (gate, butuh semua task sebelumnya).
- **Wave 12:** 10 (butuh 9).
- **Wave 13:** 11 (butuh 10).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "4.1"] },
    { "id": 1, "tasks": ["2.1", "4.2", "5.1"] },
    { "id": 2, "tasks": ["2.2", "3.1", "5.2", "5.5"] },
    { "id": 3, "tasks": ["3.2", "5.3"] },
    { "id": 4, "tasks": ["5.4"] },
    { "id": 5, "tasks": ["5.6"] },
    { "id": 6, "tasks": ["6.1", "6.2", "6.3"] },
    { "id": 7, "tasks": ["7.1"] },
    { "id": 8, "tasks": ["7.2"] },
    { "id": 9, "tasks": ["7.3", "8.1"] },
    { "id": 10, "tasks": ["8.2", "8.3"] },
    { "id": 11, "tasks": ["9"] },
    { "id": 12, "tasks": ["10"] },
    { "id": 13, "tasks": ["11"] }
  ]
}
```