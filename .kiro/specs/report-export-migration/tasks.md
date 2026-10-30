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

- [x] 1. Kunci CSV laporan lama sebelum disentuh
  - [x] 1.1 Rekam golden CSV empat laporan
    - _Catatan 2026-10-29: golden direkam dari fake reader di level handler (bukan fixture DB, sama dengan pilihan golden X.7), test `backend/internal/handler/report_csv_golden_test.go` (`-update` untuk merekam ulang), 6 file di `testdata/report_csv/`. Nama file dicek lewat `Content-Disposition`. Konsekuensi: 6.1 (Property 1) harus men-seed DB dengan baris yang menghasilkan view yang sama._
    - Fixture DB deterministik: vendor FLM + cabang + vault + `dsr_location_vault_maps` + `dsr_receipts` (OK, telat, tidak kirim, pending), `replenish_classifications` + `replenish_classification_amounts` (beberapa denom, selisih, `void`), nama dengan karakter non-ASCII dan sel berawalan `=`/`+`/`-`.
    - Jalankan handler lama dengan `format=csv` untuk `daily` (dengan dan tanpa `status`), `summary`, `amount-mismatch` (dengan dan tanpa `classification`), `trips`, dengan `now`/`as_of` tetap; simpan ke `backend/internal/handler/testdata/report_csv/` beserta nama file dari `Content-Disposition`.
    - Test integrasi yang membandingkan handler lama dengan golden (lulus sebelum perubahan apa pun).
    - _Requirements: 3.1, 3.3_
  - _Model: Sonnet, Medium — merekam output kode lama apa adanya._

- [x] 2. Paket kontrak laporan `pkg/reports`
  - _Model: Sonnet, Medium — memindahkan kode murni apa adanya._
  - [x] 2.1 Buat `pkg/reports`
    - _Catatan 2026-10-29: `pkg/reports/{reports,cells}.go`; error = `reports.ValidationError` (3.1 perlu `errors.As` ke tipe ini); `FileName` mengembalikan nama lengkap `.csv`; `WriteCSV` = `csvexport.Writer`. `cells_golden_test.go` membandingkan 6 golden 1.1 (setelah BOM) + nama file, input DSR UTC membuktikan `In(WIB)`. Coverage 55% — dinaikkan di 2.2._
    - Konstanta Report_Kind, `ReplenishmentMaxRangeDays`, `ReplenishmentExportLimit`, `DsrLateViewerRoles`, `ReplenishmentViewerRoles`, `DsrLateStatuses`, `ReplenishmentClasses`, `WIB`.
    - `ParseDsrLateStartDate`, `ParseDsrLateMonth`, `ValidateDsrLateMonth`, `ValidateReplenishmentRange` (pesan error sama dengan sekarang).
    - Tipe baris netral, `Header`, `DsrLateDailyCells`, `DsrLateSummaryCells`, `AmountMismatchLines`, `TripCells`, `FileName`; helper sel lama (`wibCell` dengan `t.In(WIB)`, `dateCell`, `str`, `int64Cell`, `intCell`, `itoa`) dipindah sebagai fungsi privat.
    - Penulisan file memakai `csvexport.Writer` (BOM + `CSVSafe`, D3).
    - _Requirements: 1.2, 1.6, 2.2, 3.1, 3.2, 3.3, 3.5, 3.6, 4.1_
  - [x]* 2.2 Test + fuzz `pkg/reports`
    - _Catatan 2026-10-29: `pkg/reports/validate_test.go` (tabel kasus lama termasuk batas bulan WIB vs UTC, 31/32 hari; `FuzzValidateDsrLateMonth` + `FuzzValidateReplenishmentRange` vs predikat referensi, 20 detik masing-masing hijau; `TestParseDsrLateStartDate`). Coverage `pkg/reports` 100%._
    - Cells/Header/FileName terhadap golden 1.1 (baris data setelah BOM).
    - **Property 3: Validasi parameter setara** (fuzz validator terhadap tabel kasus dari kode lama, `now` acak).
    - **Property 7: Gerbang tanggal mulai DSR** (bagian `ParseDsrLateStartDate`).
    - _Requirements: 1.2, 1.3, 1.6, 2.2, 2.3, 3.1, 3.6_

- [x] 3. `backend/` memakai kontrak bersama
  - [x] 3.1 Ganti konstanta dan validator lokal dengan `pkg/reports`
    - _Catatan 2026-10-29: konstanta/peran/WIB lokal dihapus dari `dsr_late_report.go`, `replenishment_read.go` dan `dsr_late_report_handler.go`; route DSR telat + replenishment dan `authorize` service memakai `reports.DsrLateViewerRoles` / `reports.ReplenishmentViewerRoles` (nilai identik; `vendorRequestViewerRoles` tetap untuk route Vendor Request). Validator service = pembungkus `pkg/reports` + `reportErr` (→ `service.ValidationError`, pesan sama). `main.go` memakai `reports.ParseDsrLateStartDate`; test lamanya pindah ke `pkg/reports`. `format=csv` tidak disentuh. Unit + integrasi laporan + golden 1.1 hijau, lint 0._
    - `dsr_late_report.go`, `replenishment_read.go`, handler laporan, dan `cmd/api/main.go` memakai peran, status, kelas, rentang, batas, dan `ParseDsrLateStartDate` dari `pkg/reports`.
    - Daftar peran disalin persis (DSR tanpa BRANCH-ATM-USER, replenishment dengan BRANCH-ATM-USER); route guard dan cek service memakai variabel yang sama.
    - Cabang `format=csv` lama masih jalan (dihapus di task 8).
    - _Requirements: 3.5, 4.1, 9.1_
    - _Model: Opus, Medium — guard RBAC layar laporan; perubahannya mekanis tetapi salah salin mengubah akses._
  - [x]* 3.2 Test layar = file dan regresi
    - _Catatan 2026-10-29: `backend/internal/handler/report_screen_file_test.go` (JSON layar → `pkg/reports` = golden 1.1 byte-exact; `/daily` semua halaman pada page_size 1/2/3/50 ± status, summary, amount-mismatch ± classification, trips). `backend/internal/service/report_screen_file_integration_test.go` read-only: `Daily` semua halaman = `DailyAll` setelah dipetakan, lebar baris = header. DB dev kosong untuk keempat laporan, jadi integrasi hanya membuktikan kasus kosong; bukti utama tetap test handler. `dsr_late_cases.json` + test JSON lama hijau._
    - **Property 2: Layar sama dengan file** (JSON `/daily` semua halaman, `/summary`, `/reports/{amount-mismatch,trips}` dipetakan lewat `pkg/reports` = baris golden).
    - Respons JSON semua endpoint laporan tidak berubah; `dsr_late_cases.json` tetap lulus.
    - _Requirements: 3.4, 3.5, 9.1, 9.2_
    - _Model: Sonnet, High — test lintas layar dan file, banyak kombinasi filter._

- [x] 4. Migrasi `035_export_jobs_reports.sql`
  - _Model: Sonnet, Medium — DDL aditif pada tabel X.7; constraint sudah ditulis di design._
  - [x] 4.1 Tulis dan terapkan migrasi
    - _Catatan 2026-10-29: DDL persis design; diterapkan ke dev via `migrate up` (izin user), `status` = 33 applied, 0 pending. Unit test `backend` + `backend-exq` hijau. `status_filter` kini nullable: regenerate sqlc `backend-exq` (5.1) mengubah tipenya ke `pgtype.Text`._
    - Cek nomor bebas (setelah `034_export_jobs.sql` X.7; `022` dicadangkan).
    - Kolom `report_params jsonb`, `kind_chk` diperluas, `status_filter` nullable, `export_jobs_params_kind_chk`, `scope_chk` baru, komentar kolom; persis seperti design.
    - `go run ./cmd/migrate up`; `migrate status` 0 pending.
    - _Requirements: 6.2, 6.3_
  - [x]* 4.2 Test migrasi
    - _Catatan 2026-10-29: `backend-exq/internal/exportjob/migration035_integration_test.go`, 17 kasus (8 diterima, 9 ditolak dengan SQLSTATE 23514 + nama constraint), tiap kasus di tx yang di-rollback; DB dev 0 baris sisa._
    - **Property 9: Migrasi tidak merusak job lama** (baris X.7 sebelum/sesudah, kombinasi Kind × parameter yang salah ditolak).
    - _Requirements: 6.3_

- [x] 5. Sisi `backend-exq`
  - [x] 5.1 sqlc membaca SQL laporan bersama
    - _Catatan 2026-10-29: dua file SQL ditambah ke `sqlc.yaml`, `sqlc generate` v1.31.1 + fix `UserLeafe`. `status_filter` kini `*string`: `job.go`/`service.go`/`download_test.go` disesuaikan (insert master data masih `&p.StatusFilter`; Kind laporan menulis NULL di 5.2+)._
    - `backend-exq/sqlc.yaml`: tambah `../backend/queries/dsr_late_report.sql` dan `../backend/queries/replenish_classifications.sql`; `sqlc generate`; tidak ada salinan SQL laporan di `backend-exq/queries/`.
    - _Requirements: 3.4_
    - _Model: Sonnet, Low — konfigurasi codegen._
  - [x] 5.2 `ReportParams`, `ParseReportParams`, hash
    - _Catatan 2026-10-29: `backend-exq/internal/exportjob/report_params.go` (+ test). `ParseReportParams(kind, q, now)` mengembalikan `Params{Kind, Entity, Report}`; error = `ErrValidation` membungkus `*reports.ValidationError`. Rentang tanggal dan bulan divalidasi di parser (dulu di service). `page`/`page_size`/`only_diff` tidak ikut (amount-mismatch selalu `only_diff`+`final_only`). `region_code` ikut di trips seperti filter audit lama, walau SQL trips tidak memakainya (5.4 abaikan). `ParamsHash` cabang laporan = JSON `{kind, entity, report}`; hash master data tidak berubah._
    - Struct sesuai design, parsing query memakai validator `pkg/reports`, parameter yang tidak berlaku untuk Kind diabaikan, `ParamsHash` dari JSON kanonis, `entity` = nilai `report` lama.
    - _Requirements: 1.2, 1.3, 2.2, 2.3, 6.2_
    - _Model: Sonnet, Medium — validasi mengikuti paket bersama._
  - [x] 5.3 `KindSpecs` laporan dan bentuk audit
    - _Catatan 2026-10-29: `kinds.go` + `ActionReportExported`, `fixed(entity_type)` (id 0), empat entri dengan peran `pkg/reports`; route master data X.7 tetap memakai Kind-nya sendiri, jadi tidak terbuka untuk peran laporan. `InsertExportJob` menulis `report_params` (query + sqlc), `status_filter` NULL untuk Kind laporan (`statusFilterOf`), `FileName` laporan = `reports.FileName`, `jobFromRow` membaca `report_params`. `requestedAfter` untuk Kind laporan = `{export_job_id, export_kind, report, filter, username, role}` (+ `row_count` saat unduh, jalur X.7); `filter` memakai kunci lama (null bila kosong) + `status`/`classification`. Bila `report_params` tidak bisa dibaca, `Report` = nil: 5.4 wajib menggagalkan job seperti itu, bukan mengekspor tanpa filter. Test `kinds_report_test.go` (peran 13 × 4 Kind, audit, nama file, round trip); integrasi X.7 + lint hijau, DB dev 0 sisa._
    - Empat entri: peran dari `pkg/reports`, `DownloadAction = report_exported`, `EntityRef` tetap (`dsr_late_report` / `replenishment_report`, id 0).
    - `after` untuk `export_requested` dan `report_exported`: `report` lama, `filter` lengkap (termasuk `status` dan `classification`), `export_job_id`, `username`, `role`, `row_count` (unduh saja).
    - Kind master data tetap ADMIN/ADMIN_PARAM; cek peran saat unduh membaca `KindSpecs[job.Kind].Roles` dari DB (sudah ada di X.7).
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3, 5.4_
    - _Model: Opus, High — auth per Kind dan audit yang harus menyatu dengan riwayat lama._
  - [x] 5.4 ReportProducer
    - _Catatan 2026-10-29: `backend-exq/internal/exportjob/producers_report.go` (+ test). `NewProducers(dsrStart)` menggabungkan producer master data + laporan (`DefaultProducers()` = `NewProducers(nil)`); `main.go` meneruskan `dsrLateStart`. Daily dan amount-mismatch berhalaman 1000 (OFFSET, aman karena satu snapshot REPEATABLE READ); amount-mismatch `only_diff`+`final_only`, berhenti diam-diam di 100.000 klasifikasi seperti kode lama; trips mengabaikan `region_code`. `as_of` = waktu worker mulai memproduksi. Start date nil = header saja; `Report` nil atau tanggal rusak = job gagal (`errNoReportParams`). Output test = 6 golden 1.1 byte demi byte setelah BOM. Coverage exportjob 94,6%._
    - Empat producer sesuai tabel design: `ListDsrLateDaily` berhalaman di dalam snapshot, `ListDsrLateSummary`, `ListReplenishClassifications` (`only_diff`, `final_only`, total ≤ 100.000) + `ListReplenishClassificationAmounts`, `ReportReplenishTrips`; `as_of` = waktu job mulai; `DSR_LATE_START_DATE` kosong = header saja.
    - Header, sel, nama file dari `pkg/reports`; nominal bilangan bulat apa adanya.
    - _Requirements: 1.4, 1.5, 2.4, 2.5, 3.1, 3.2, 3.3, 3.6, 6.1, 6.4_
    - _Model: Sonnet, High — paging di dalam snapshot dan ekspansi per denom harus sama persis dengan file lama._
  - [x] 5.5 `DSR_LATE_START_DATE` di `backend-exq`
    - _Catatan 2026-10-29: `cmd/api/main.go` parse lewat `reports.ParseDsrLateStartDate`, salah format = `os.Exit(1)` sebelum pool dibuat; nilai di variabel lokal `dsrLateStart` (`_ =` sementara, 5.4 meneruskannya ke producer DSR). Komentar "sama di tiga runtime" di `.env.example` `backend`, `backend-exq` (+ `DSR_LATE_START_DATE=`), `backend_python/eod_retry_scheduler`._
    - `cmd/api/main.go` memakai `reports.ParseDsrLateStartDate` (salah format = gagal start) dan meneruskannya ke producer DSR.
    - Komentar "harus sama" di `.env.example` `backend`, `backend-exq`, `backend_python` (`RETRY_DSR_LATE_START_DATE`).
    - _Requirements: 1.5, 1.6_
    - _Model: Sonnet, Low — wiring env._
  - [x] 5.6 Route `/exports/reports/*` dan OpenAPI
    - _Catatan 2026-10-29: `POST /api/v1/exports/reports/{dsr-late-daily,dsr-late-summary,replenishment-amount-mismatch,replenishment-trips}` di `exports_handler.go`, guard `KindRoles[kind]`, timeout 30 detik dari mount X.7; 202 baru / 200 job aktif sama (X.7). 400 laporan = envelope `validation_error` + `details[{field,message}]` (sama dengan body `backend/` lama), tanpa job/audit; error master data tetap `bad_request`. Job JSON: `report_params` (null untuk master data), `status_filter` nullable. OpenAPI: 4 path, `ExportKind` diperluas, skema `ReportParams`, `ErrorEnvelope.error.details`. Test `exports_reports_test.go` + kasus kontrak; handler coverage 98,1%._
    - Empat route POST dengan `RequireRoles` per daftar peran laporan, timeout 30 detik, error `pkg/response`; `/jobs*` X.7 tidak berubah.
    - `backend-exq/api/openapi.yaml`: path baru, `ExportJob.kind` diperluas, `report_params`; test kontrak X.7 lulus.
    - _Requirements: 1.1, 2.1, 4.1, 4.2, 8.4_
    - _Model: Sonnet, Medium — handler tipis mengikuti pola X.7._

- [x] 6. Integrasi `backend-exq` dengan Postgres nyata
  - _Model: Sonnet, High — property berbasis DB dengan data acak ber-seed._
  - [x] 6.1 Producer
    - _Catatan 2026-10-29: `backend-exq/internal/exportjob/report_producers_integration_test.go`, 14 subtest. Fixture `T61_*` di DB dev (vendor, cabang, vault, map, receipt, classification, user) dibersihkan; sisa 0. Property 1: `Service.Create` + `worker.produce` + chunk dibaca balik = BOM + CSV persis (nama file dicek). Property 7: start date nil / sesudah data = header saja; start 06-03 memotong hari. Property 10: 1200 baris (>1 halaman), tulisan dari koneksi lain setelah halaman 1 tidak mengubah file. Urutan denom per klasifikasi = `denom DESC` dari query (sama dengan kode lama; fake golden 1.1 kebetulan urut lain)._
    - **Property 1: File sama dengan export lama** (golden 1.1, setelah BOM); **Property 7: Gerbang tanggal mulai DSR** (env kosong = header saja); **Property 10: Snapshot konsisten** (data berubah di antara halaman).
    - _Requirements: 1.4, 1.5, 2.4, 2.5, 3.1, 3.2, 3.3, 3.6, 6.1, 6.4_
  - [x] 6.2 Peran dan audit
    - _Catatan 2026-10-29: `backend-exq/internal/handler/exports_reports_integration_test.go` (router asli + token asli + Postgres dev, fixture `T62_*` dibersihkan, sisa 0). Property 4: 14 peran di tabel `roles` × 8 Kind (202 + job + audit ⇔ peran di `KindSpecs`; master data tetap ADMIN/ADMIN_PARAM) + tanpa token 401. Property 5: peran DB diganti ACM-USER → unduh 403 tanpa audit/klaim, dipulihkan → 200 + satu `report_exported`. Property 6: bentuk `after` empat Kind laporan, `entity_id` 0, IP; `ListAuditLogs` (SQL `backend/queries/audit_log.sql` dijalankan mentah karena `backend/internal/db` tidak bisa diimpor) mengembalikan baris lama + baru._
    - **Property 4: Matriks peran** (14 peran × 8 Kind, termasuk master data tetap admin); **Property 5: Cek peran ulang saat unduh**; **Property 6: Bentuk audit laporan** (termasuk query `ListAuditLogs` dengan baris lama dari `backend/`).
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 5.1, 5.2, 5.3, 5.4, 5.5, 9.3_
  - [x] 6.3 Parameter dan dedupe
    - _Catatan 2026-10-29: `backend-exq/internal/handler/exports_reports_params_integration_test.go` (memakai env 6.2, fixture `T63_*`, tanpa worker; sisa 0). Property 3: 30 query salah → 400 `validation_error` + `details` = pesan lama, tanpa job/audit; 8 parameter tak berlaku diabaikan (202). Property 8: `report_params`/`status_filter` NULL/`entity`/`file_name` tersimpan; dedupe queued/running (urutan query beda = job sama), filter beda = job baru, setelah failed/expired = job baru; job aktif ke-4 → 429. **Temuan:** `go test -tags integration ./...` paralel antar-package bisa membuat `TestW11Property15ClaimOnce` (X.7) gagal, karena worker W11 mengklaim job `queued` milik test handler 6.2/6.3 (klaim tidak dibatasi per user). Lulus dengan `-p 1` dan saat dijalankan sendiri. Diatasi: `.github/workflows/ci.yml` langkah integrasi `backend` dan `backend-exq`/`pkg` memakai `-p 1` (keputusan user); jalankan lokal dengan `-p 1` juga. Jenkins belum diubah._
    - **Property 3: Validasi parameter setara** (lapisan HTTP, tanpa job/audit saat 400); **Property 8: Parameter tersimpan dan dedupe**.
    - _Requirements: 1.3, 2.3, 6.1, 6.2_

- [x] 7. Frontend CompanyPortal
  - _Model: Sonnet, Medium — komponen React dengan pola X.7._
  - [x] 7.1 `features/exports` untuk laporan dan halaman Ekspor Saya
    - _Catatan 2026-10-29: `exq-schema.d.ts` di-regenerate; `requestReportExport` + varian `{ kind: "report" }` (hook `useExportFlow` tidak perlu diubah); `KIND_LABEL` 8 Kind di `status.ts`; `MyExportsTable` "Minta ekspor lagi" memakai `report_params` (tersembunyi bila null); dialog menautkan `/exports` hanya untuk job laporan (`<a href>` biasa, muat ulang halaman penuh); `MyExportsPage` + route lazy `/exports` (guard gabungan peran laporan; `requireRoles` juga meloloskan ADMIN/ADMIN_PARAM), tanpa sidebar. `downloadReport` belum dihapus (7.2). lint, 1713 test, coverage 86,06/81,57/82,69/87,14, build 179 KB hijau. Cek browser: outstanding (user)._
    - `requestReportExport`, varian `{ kind: "report" }` di `useExportFlow`, label jenis laporan di `status.ts`, tautan dialog ke `/exports`.
    - `MyExportsPage` + route lazy `/exports` dengan peran gabungan laporan (tanpa item sidebar); "Minta ekspor lagi" memakai `report_params`.
    - Regenerate `exq-schema.d.ts` (`gen:api:exq`).
    - _Requirements: 6.2, 7.3, 7.5_
  - [x] 7.2 Tombol ekspor di layar laporan
    - _Catatan 2026-10-29: `DsrLateReportScreen` "Ekspor CSV" → `dsr-late-daily`/`dsr-late-summary` sesuai tab (status hanya tab harian); `ReplenishmentScreen` dua tombol → amount-mismatch (+classification) / trips, nonaktif saat rentang salah. Komponen bersama baru `features/exports/components/ReportExportNotice.tsx`: 429 inline (ikon + teks, `role="alert"`, tanpa dialog) + tautan "Lihat Ekspor Saya" `/exports`; `ExportProgressDialog` dipasang di kedua layar. `downloadReport` dihapus dari kedua `api.ts`. Sisa `format=csv` hanya di komentar `schema.d.ts` hasil generate (hilang di 8.2). lint, 1714 test, coverage 86,23/81,83/82,79/87,3, build 184,4 KB hijau. Cek browser: outstanding (user)._
    - `DsrLateReportScreen`: "Ekspor CSV" -> `useExportFlow` dengan filter layar (status hanya tab detail). `ReplenishmentScreen`: "Selisih nominal (CSV)" dan "Trip per ATM (CSV)", tetap nonaktif saat rentang tidak valid.
    - 429 inline (ikon + teks) tanpa dialog; tautan Ekspor Saya.
    - Hapus `downloadReport` berbasis `fetch` mentah di kedua `api.ts`.
    - _Requirements: 7.1, 7.2, 7.4, 7.6, 8.5_
  - [x]* 7.3 Test frontend
    - _Catatan 2026-10-29: Property 12 dengan fast-check (sudah devDependency) pada layar asli: `dsr-late-report/__tests__/exportRequest.property.test.tsx` (numRuns 40) dan `replenishment/__tests__/exportRequest.property.test.tsx` (numRuns 50, rentang tidak sah = tombol nonaktif tanpa request). `exports/noRawCsvFetch.test.ts` memindai sumber kedua fitur (`downloadReport`/`format=csv`/`fetch(`). `MyExportsPage.test.tsx` + job laporan & master data. 1726 test, lint, build hijau._
    - **Property 12: Tombol ekspor mengirim filter layar** (fast-check state filter acak); halaman `/exports`; tidak ada `fetch` mentah tersisa untuk CSV.
    - _Requirements: 7.1, 7.2, 7.4, 7.5, 7.6, 8.5_

- [x] 8. Cutover export laporan di `backend/`
  - _Model: Sonnet, Medium — menghapus jalur lama setelah frontend pindah; urutan rilis penting._
  - [x] 8.1 Hapus cabang `format=csv`
    - _Catatan 2026-10-29: `rejectCSVFormat` (statement pertama tiap handler, sesudah guard peran, sebelum parsing/service/audit) → 400 `validation_error` field `format`; format lain diabaikan seperti dulu. Dihapus: `writeCSV` + helper sel, `dsrLateExportFilter`/`exportFilter`, `RecordExport` + `DailyAll` (service + interface), konstanta entity audit; konstruktor kehilangan pool primary (`NewDsrLateReportService(readPool, start)`, `NewReplenishmentReadService(readPool)`). `report_csv_golden_test.go` kini hanya fake reader + `reportGoldenDir` (tanpa `-update`); golden dijaga 3.2 + `pkg/reports` + 6.1. Kasus kontrak `format=csv` diubah ke 400 (yaml di 8.2). Unit + lint hijau; integrasi `backend` hijau kecuali dua round-trip master data `vendor-branches` X.7 — penyebabnya di luar 8.1: fixture `ITESTNUM-*/ITV2-*/ITB2-*` dari `vault_plan_integration_test.go` + `vendor_request_number_integration_test.go` menumpuk di DB dev (2037 cabang > batas import 2000)._
    - Empat endpoint mengembalikan 400 field `format` dengan pesan yang menunjuk menu ekspor; hapus `writeCSV`, helper sel lokal, `RecordExport`, `DailyAll`; respons JSON tidak berubah.
    - Golden 1.1 tetap dijaga oleh 3.2 dan 6.1.
    - _Requirements: 8.1, 8.2_
  - [x] 8.2 OpenAPI `backend/` dan tipe
    - _Catatan 2026-10-29: `DsrLateFormat` + respons `text/csv` dihapus dari `/dsr-late-reports/{daily,summary}`; param `format` tidak dicantumkan lagi, deskripsi menyebut CSV pindah ke API ekspor dan `format=csv` = 400 (`BadRequest` sudah ada). Path replenishment report memang tidak ada di spec. `schema.d.ts` di-regenerate; kasus kontrak 400 dari 8.1 tetap lulus. Handler Go + CompanyPortal lint/1726 test/build hijau (stderr `window.scrollTo` jsdom dari router = noise, bukan kegagalan)._
    - Hapus `DsrLateFormat` dan respons `text/csv` dari `/dsr-late-reports/{daily,summary}`; `pnpm run gen:api`; test kontrak `backend/` lulus.
    - _Requirements: 8.3_
  - [x]* 8.3 Test cutover
    - _Catatan 2026-10-29: `backend/internal/handler/report_cutover_property_test.go`: 4 endpoint × tiap peran yang diizinkan × 40 query acak (seed 11) + `format=csv` → selalu 400 `format`, JSON, fake service tidak tersentuh; tanpa `format`/`format=json`/`format=` → 200, body = output service. `report_cutover_integration_test.go` (service asli, DB dev, read-only): `format=csv` → 400 tanpa baris `audit_logs` baru. Suite integrasi `backend -p 1` hijau, lint 0._
    - **Property 11: Cutover** (`format=csv` = 400 tanpa audit/file; tanpa `format` JSON tidak berubah).
    - _Requirements: 8.1, 9.1_

- [x] 9. Checkpoint: gate kualitas
  - _Catatan 2026-10-29:_
    - _Go `backend`, `backend-exq`, `pkg`: `go test -tags integration -p 1 -cover ./...` hijau terhadap DB dev, 0 skip; `export_jobs` sisa 0, vendor/cabang tetap 9/407. `-race` tidak bisa lokal (cgo mati di Windows), jalan di CI._
    - _Coverage lolos `check-coverage.sh`. Threshold `pkg/reports 100` ditambahkan; `backend-exq` exportjob 94, handler 98._
    - _`golangci-lint` 0 issue di tiga modul._
    - _`govulncheck`: GO-2026-6809 (`crypto/x509` stdlib) di ketiga modul, sama dengan X.7. Perbaikannya bump Go 1.26.10; toolchain sengaja tidak diubah._
    - _CompanyPortal: lint, 1726 test, coverage 86,23/81,87/82,79/87,32, build 184,4 KB hijau. `gen:api` + `gen:api:exq` konsisten (test staleness hijau)._
    - _Python `dsr_late_notify` 16 test OK._
    - _Matriks peran 6.2 hijau: master data tetap ADMIN/ADMIN_PARAM. `go.mod`/`go.sum`/`package.json`/lockfile tidak berubah, jadi tidak ada library baru._
  - `go test ./...` + `-tags integration` di `backend`, `backend-exq`, `pkg`; `golangci-lint`, `govulncheck`; coverage Go ≥ 80% `internal/*` dan `pkg/`.
  - CompanyPortal: `pnpm run lint`, `pnpm run test:coverage` (84/80/80/85), `pnpm run build`, `gen:api` + `gen:api:exq` bersih.
  - Python: `dsr_late_notify` test tetap lulus (SQL tidak berubah).
  - Ekspor master data X.7 tetap hanya ADMIN/ADMIN_PARAM; tidak ada library baru.
  - _Requirements: 9.2, 9.3, 9.4_
  - _Model: Sonnet, Medium — triase kegagalan gate._

- [x] 10. Dokumentasi
  - _Catatan 2026-10-29: `.claude/CLAUDE.md` Sec 3 (`pkg/reports`, `backend-exq` + ekspor laporan, mig `035` di Core) + Sec 12 (baseline "up to `035_`"); `data-map.md` subbagian "Migration 035"; `rbac-role-feature-matrix.md` (4 route `/api/v1/exports/reports/*`, `/exports`, `format=csv` = 400); `tech.md` (`pkg/reports`); `development-plan.md` X.8 = built + baris 2.3 (CSV pindah ke X.8); `development-progress.md` baris X.8; `docs/deployment.md` "Release checklist: report export (X.8)" (`DSR_LATE_START_DATE` sama di tiga runtime, `035` sebelum rilis, task 7 + 8 satu rilis, rollback image) + CI `-p 1`._
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