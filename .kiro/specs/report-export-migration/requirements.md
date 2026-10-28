# Requirements Document

> **Status:** requirements disetujui 2026-10-28. PO memilih mengikuti semua saran Kiro (D1-D10).

## Introduction

### Masalah

Keputusan D13 spec `vendor-data-export` (2026-10-28): semua kegiatan export data berjalan di `backend-exq`. Spec itu baru memindahkan export master data (D14). Dua export lain masih ada di ATM backend `backend/`, menumpang di endpoint laporan JSON lewat `?format=csv`:

- **Laporan DSR telat** (Phase 2.3): CSV detail harian dan rekap bulanan.
- **Laporan replenishment** (Phase 2.2c): CSV selisih nominal dan CSV trip per ATM.

Spec ini memindahkan keempat CSV tersebut ke `backend-exq` dan memprosesnya lewat mekanisme `export_jobs` yang dibangun `vendor-data-export` (asinkron, pop-up loading, unduh otomatis, sekali unduh, kedaluwarsa 10 menit).

### Kondisi kode saat ini (dicek 2026-10-28)

**Laporan DSR telat** (`backend/internal/handler/dsr_late_report_handler.go`, `backend/internal/service/dsr_late_report.go`, `backend/queries/dsr_late_report.sql`)

- Mount `/api/v1/dsr-late-reports` di belakang `RequireAuth`; grup route `RequireRoles("ATM-USER","ATM-SPV","BRANCH-ATM-SPV","ADMIN")`, dicek ulang di service.
- `GET /daily[?format=csv]`, `GET /summary[?format=csv]`, `GET /warnings` (JSON saja).
- Parameter: `month` (YYYY-MM, wajib, tidak boleh setelah bulan berjalan WIB), `vendor_id`, `vendor_branch_id`, `status` (`ok|late|not_sent|pending`, daily saja), `page`, `page_size`.
- `DSR_LATE_START_DATE` (env, harus sama dengan Python `RETRY_DSR_LATE_START_DATE`). Kosong = laporan mati: respons kosong, CSV hanya header.
- CSV daily: semua baris tanpa batas (`DailyAll`, `page_limit` NULL). Kolom `report_date, vendor_code, vendor, branch_code, branch_name, region_code, deadline_wib, first_received_at_wib, status, late_minutes, receipt_checksum, matched_locations`. Nama file `dsr-late-daily-YYYY-MM.csv`.
- CSV summary: kolom `vendor_code, vendor, branch_code, branch_name, region_code, obliged_days, ok_days, late_days, not_sent_days, pending_days, late_minutes_total`. Nama file `dsr-late-summary-YYYY-MM.csv`.
- Status dihitung saat baca lewat rantai CTE di SQL. `backend_python/dsr_late_notify/queries.py` menyalin sebagian CTE itu, jadi kedua sisi harus berubah bersama.
- Audit: `report_exported`, `entity_type = dsr_late_report`, `entity_id = 0`, `after = {report, filter{month, vendor_id, vendor_branch_id[, status]}}`, ditulis di primary sebelum file dikirim. Baca dari replica.
- Kontrak OpenAPI `backend/api/openapi.yaml` mencakup `/dsr-late-reports/{daily,summary,warnings}`, termasuk respons `text/csv` dan parameter `format`.

**Laporan replenishment** (`backend/internal/handler/replenishment_handler.go`, `backend/internal/service/replenishment_read.go`, `backend/queries/replenish_classifications.sql`)

- Mount `/api/v1/replenishment`; grup route `RequireRoles(ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV)`, dicek ulang di service.
- CSV hanya di `GET /reports/amount-mismatch[?format=csv]` dan `GET /reports/trips[?format=csv]`. `GET /classifications` dan `GET /summary` JSON saja.
- Parameter: `date_from`, `date_to` (wajib, `from <= to`, maksimal 31 hari), `vendor_id`, `region_code`, `classification` (amount-mismatch). Amount-mismatch memaksa `only_diff` + `final_only` dan dibatasi 100.000 klasifikasi; trips tanpa batas.
- CSV amount-mismatch: satu baris per klasifikasi × denom; kolom `report_date, ticket_number, request_number, terminal_id, vendor, branch_code, region_code, scheduled_date, realized_date, classification, workday_offset, denom, order_amount, itm_amount, vendor_amount, diff, currency`. Nama file `replenishment-amount-mismatch-<from>-<to>.csv`.
- CSV trips: kolom `vendor_id, vendor, terminal_id, trips, first_date, last_date`. Nama file `replenishment-trips-<from>-<to>.csv`.
- Data ditulis job Python `replenish_classify` ke `replenish_classifications` dan `replenish_classification_amounts`.
- Audit: `report_exported`, `entity_type = replenishment_report`, `entity_id = 0`, `after = {report, filter{date_from, date_to, vendor_id, region_code}}`. Filter `classification` **tidak** tercatat. Baca dari replica.
- Tidak ada di kontrak OpenAPI (hand-typed).

**Bersama**

- Keempat CSV memakai helper `writeCSV` (`replenishment_handler.go`): `CSVSafe` per sel, **tanpa BOM**, file dibangun penuh di memori dalam satu request (sinkron).
- Frontend CompanyPortal: `features/dsr-late-report/api.ts` dan `features/replenishment/api.ts` memakai `fetch` mentah (bukan klien `api`), satu tombol "Ekspor CSV" di `DsrLateReportScreen.tsx` dan dua tombol di `ReplenishmentScreen.tsx`.
- VendorPortal tidak punya CSV laporan (`/api/v1/vendor/replenish-realizations` JSON saja).

**Ketergantungan pada `vendor-data-export`** (belum dibangun)

- `export_jobs` (migrasi `034`) hanya mengenal Kind `vendor | cabang | harga_paket | master_data_global` dan kolom parameter master data (`vendor_id`, `vendor_branch_id`, `status_filter` wajib, `price_level`).
- Grup route `/api/v1/exports` di design `vendor-data-export` dikunci `RequireRoles("ADMIN","ADMIN_PARAM")`, dan cek peran saat unduh memakai ADMIN/ADMIN_PARAM. Peran laporan (ATM-USER, ATM-SPV, BRANCH-*) akan tertolak.
- Daftar Ekspor Saya berada di `/settings/admin/exports` (ADMIN/ADMIN_PARAM saja).

### Ruang lingkup

- Empat Report_Kind (`dsr_late_daily`, `dsr_late_summary`, `replenishment_amount_mismatch`, `replenishment_trips`) diproses lewat `export_jobs` di `backend-exq`.
- SQL laporan dipakai bersama `backend/` (layar JSON) dan `backend-exq` (CSV) lewat sqlc, tanpa salinan (D1).
- Otorisasi per Report_Kind dengan peran yang sama seperti hari ini.
- Audit permintaan + unduhan, termasuk filter yang hari ini tidak tercatat.
- Tombol ekspor di layar DSR telat dan Replenishment memakai `ExportProgressDialog`.
- Cutover: cabang `format=csv` dan `RecordExport` di `backend/` dihapus, kontrak OpenAPI `backend/` disesuaikan.
- Penyesuaian kecil mekanisme `export_jobs` / `/api/v1/exports` dari `vendor-data-export` supaya bisa dipakai peran non-admin (D9).

### Tidak termasuk

- Memindahkan layar JSON laporan (`/daily`, `/summary`, `/warnings`, `/classifications`, `/summary` replenishment) ke `backend-exq` (D1).
- Mengubah logika status DSR telat, klasifikasi replenishment, atau job Python (`dsr_late_notify`, `replenish_classify`).
- Laporan baru, kolom baru, XLSX/PDF, ekspor dari VendorPortal.
- Mengubah batas rentang 31 hari dan batas 100.000 klasifikasi.

### Area sensitif (project-context Sec 3 rule 7)

- **Auth/RBAC:** peran laporan berbeda per jenis (DSR: tanpa BRANCH-ATM-USER; replenishment: dengan BRANCH-ATM-USER). Mekanisme job dari `vendor-data-export` harus dibuka untuk peran ini tanpa membuka ekspor master data ke mereka.
- **Money:** CSV selisih nominal berisi nominal order/ITM/vendor per denom; harus identik dengan layar.
- **Logika lintas runtime:** SQL DSR telat berbagi CTE dengan Python `dsr_late_notify`; spec ini tidak boleh mengubah hasilnya.
- **Migrasi:** perluasan constraint `export_jobs` (aditif).

### Dependency & Scope Note

Item X.8 di `development-plan.md`. Bergantung penuh pada `vendor-data-export` (X.7): `export_jobs`, worker, janitor, `pkg/csvexport`, `ExportProgressDialog`, routing `/api/v1/exports/` ke `:8081`, dan deployment pertama `backend-exq`. Implementasi dimulai setelah X.7 di-deploy. Penyesuaian D9 sebaiknya dimasukkan ke `vendor-data-export` sebelum spec itu diimplementasikan, supaya tidak ada pekerjaan ulang.

## Glossary

- **Report_Kind**: jenis ekspor laporan: `dsr_late_daily`, `dsr_late_summary`, `replenishment_amount_mismatch`, `replenishment_trips`.
- **ReportExportAPI**: endpoint permintaan ekspor laporan di `backend-exq` di bawah `/api/v1/exports/reports/`.
- **ReportProducer**: producer `backend-exq` yang membuat CSV satu Report_Kind dari SQL laporan bersama.
- **Report_SQL**: file `backend/queries/dsr_late_report.sql` dan `backend/queries/replenish_classifications.sql` (atau bagian query laporannya) yang di-generate sqlc di `backend/` dan `backend-exq`.
- **Legacy_Report_CSV**: CSV yang dihasilkan `backend/` hari ini untuk Report_Kind yang sama (kolom, urutan, format sel, nama file).
- **Report_Viewer_Roles**: peran yang boleh melihat dan mengekspor laporan: DSR telat = `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-SPV`, `ADMIN`; replenishment = `ADMIN`, `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`.
- **Export_Job**, **ExportWorker**, **ExportProgressDialog**, **ExqAuditWriter**, **CSV_Contract**: sama dengan glossary spec `vendor-data-export`.
- **DsrLateReportScreen**, **ReplenishmentScreen**: layar laporan CompanyPortal yang memuat tombol ekspor.

## Requirements

### Requirement 1: Ekspor laporan DSR telat lewat Export_Job

**User Story:** Sebagai pengguna laporan DSR telat, saya ingin mengekspor detail harian dan rekap bulanan sebagai file, supaya bisa dipakai sebagai dasar penalti FLM tanpa membebani API transaksional.

#### Acceptance Criteria

1. WHEN pengguna dengan Report_Viewer_Roles DSR meminta ekspor `dsr_late_daily` atau `dsr_late_summary`, THE ReportExportAPI SHALL membuat satu Export_Job dan menjawab `202` (atau `200` untuk permintaan identik yang masih diproses).
2. THE ReportExportAPI SHALL menerima parameter `month` (wajib, `YYYY-MM`), `vendor_id`, `vendor_branch_id`, dan `status` (hanya `dsr_late_daily`, nilai `ok|late|not_sent|pending`), dengan aturan validasi yang sama dengan `backend/` hari ini.
3. IF `month` kosong, salah format, atau setelah bulan berjalan (WIB), atau `status`/id tidak valid, THEN THE ReportExportAPI SHALL menolak dengan 400 tanpa membuat Export_Job.
4. THE ReportProducer SHALL menyertakan semua baris yang cocok tanpa batas paging (`dsr_late_daily`), atau semua cabang (`dsr_late_summary`), dihitung pada waktu `as_of` = saat Export_Job mulai diproses.
5. WHILE `DSR_LATE_START_DATE` kosong di `backend-exq`, THE ReportProducer SHALL menghasilkan file berisi header saja, sama dengan perilaku sekarang.
6. THE `backend-exq` SHALL membaca `DSR_LATE_START_DATE` dengan format dan aturan yang sama dengan `backend/` (kosong = mati, format salah = gagal start).

### Requirement 2: Ekspor laporan replenishment lewat Export_Job

**User Story:** Sebagai pengguna laporan replenishment, saya ingin mengekspor laporan selisih nominal dan trip per ATM sebagai file, dengan aturan yang sama seperti hari ini.

#### Acceptance Criteria

1. WHEN pengguna dengan Report_Viewer_Roles replenishment meminta ekspor `replenishment_amount_mismatch` atau `replenishment_trips`, THE ReportExportAPI SHALL membuat satu Export_Job dan menjawab `202` (atau `200` untuk permintaan identik yang masih diproses).
2. THE ReportExportAPI SHALL menerima `date_from`, `date_to` (wajib, `YYYY-MM-DD`), `vendor_id`, `region_code`, dan `classification` (hanya amount-mismatch), dengan validasi yang sama dengan `backend/`: `from <= to` dan rentang maksimal 31 hari.
3. IF parameter tidak valid, THEN THE ReportExportAPI SHALL menolak dengan 400 tanpa membuat Export_Job.
4. THE ReportProducer `replenishment_amount_mismatch` SHALL memakai `only_diff = true`, `final_only = true`, dan batas 100.000 klasifikasi yang sama dengan sekarang, menghasilkan satu baris per klasifikasi × denom.
5. THE ReportProducer `replenishment_trips` SHALL memakai filter tanggal realisasi dan pengecualian `void` yang sama dengan query `ReportReplenishTrips` saat ini.

### Requirement 3: Isi file sama dengan export lama dan dengan layar

**User Story:** Sebagai pengguna laporan, saya ingin file hasil ekspor berisi kolom dan angka yang sama dengan export lama dan dengan yang tampil di layar, supaya tidak ada perselisihan angka.

#### Acceptance Criteria

1. THE ReportProducer SHALL menulis kolom, urutan kolom, format sel (tanggal, waktu WIB `YYYY-MM-DD HH:MM:SS`, angka bulat, `matched_locations` digabung `"; "`), dan urutan baris yang identik dengan Legacy_Report_CSV untuk data dan `as_of` yang sama, kecuali perbedaan yang diputuskan di D3.
2. THE ReportProducer SHALL melewatkan setiap sel melalui `CSVSafe` dari CSV_Contract.
3. THE Export_Job SHALL memakai nama file yang sama dengan Legacy_Report_CSV (`dsr-late-daily-YYYY-MM.csv`, `dsr-late-summary-YYYY-MM.csv`, `replenishment-amount-mismatch-<from>-<to>.csv`, `replenishment-trips-<from>-<to>.csv`).
4. THE `backend-exq` dan `backend/` SHALL memakai Report_SQL yang sama (di-generate sqlc di kedua modul), sehingga angka di file dan di layar berasal dari query yang sama; tidak ada salinan SQL laporan di `backend-exq`.
5. THE logika pemetaan baris ke sel (konversi WIB, penggabungan label, sel kosong untuk NULL) SHALL berada di satu tempat bersama (`pkg/`) atau dibuktikan identik oleh test golden, sehingga file dari `backend-exq` tidak bisa berbeda dari Legacy_Report_CSV tanpa test gagal.
6. THE nominal di `replenishment_amount_mismatch` SHALL ditulis sebagai bilangan bulat persis dari database, tanpa konversi float, dengan kolom `currency`.

### Requirement 4: Otorisasi per jenis laporan

**User Story:** Sebagai pemilik sistem, saya ingin hak ekspor laporan sama persis dengan hak melihat laporan hari ini, dan ekspor laporan tidak membuka akses ke ekspor master data.

#### Acceptance Criteria

1. THE ReportExportAPI SHALL mengizinkan permintaan `dsr_late_*` hanya untuk Report_Viewer_Roles DSR dan `replenishment_*` hanya untuk Report_Viewer_Roles replenishment, di guard route dan di layer service.
2. IF pemanggil tidak punya peran untuk Report_Kind yang diminta, THEN THE ReportExportAPI SHALL menolak dengan 403 tanpa membuat Export_Job dan tanpa audit.
3. THE perubahan otorisasi pada `/api/v1/exports` SHALL tetap membatasi ekspor master data (`vendor`, `cabang`, `harga_paket`, `master_data_global`) ke `ADMIN` dan `ADMIN_PARAM`.
4. WHEN Export_File laporan diunduh, THE ExportRequestAPI SHALL memeriksa ulang dari database bahwa user masih aktif dan perannya masih termasuk Report_Viewer_Roles untuk Report_Kind tersebut.
5. THE aturan kepemilikan dari `vendor-data-export` SHALL tetap berlaku: Export_Job hanya terlihat dan bisa diunduh oleh peminta.

### Requirement 5: Audit ekspor laporan

**User Story:** Sebagai auditor internal, saya ingin jejak audit ekspor laporan tetap bisa dicari seperti hari ini, ditambah siapa yang meminta, filter lengkap, dan jumlah baris.

#### Acceptance Criteria

1. WHEN Export_Job laporan dibuat, THE ExqAuditWriter SHALL menulis `export_requested` dalam transaksi yang sama, mengikuti aturan `vendor-data-export` Requirement 8.
2. WHEN Export_File laporan diunduh, THE ExqAuditWriter SHALL menulis baris dengan action `report_exported` (D5) sebelum byte pertama dikirim; gagal audit = 500 tanpa file.
3. THE baris audit laporan SHALL memakai `entity_type` yang sama dengan sekarang (`dsr_late_report`, `replenishment_report`) dan `entity_id = 0`.
4. THE kolom `after` SHALL memuat `report` (`daily`, `summary`, `amount_mismatch`, `trips`, sama dengan nilai sekarang), `filter` lengkap termasuk `status` (DSR daily) dan `classification` (amount-mismatch), `export_job_id`, `username`, `role`, dan pada `report_exported` juga `row_count`.
5. WHEN auditor memfilter viewer audit log dengan action `report_exported`, THE baris dari `backend-exq` SHALL muncul bersama baris lama dari `backend/` dengan bentuk `entity_type` dan `after.report` yang sama.

### Requirement 6: Siklus Export_Job untuk laporan

**User Story:** Sebagai pengguna laporan, saya ingin ekspor laporan berperilaku sama dengan ekspor data vendor, supaya pengalamannya konsisten.

#### Acceptance Criteria

1. THE Export_Job laporan SHALL mengikuti seluruh aturan siklus `vendor-data-export` Requirement 9 dan 10: status, dedupe permintaan identik, batas 3 job aktif per user, batas waktu proses 10 menit, satu kali unduh, hapus setelah unduh, kedaluwarsa 10 menit, batas 100 MB / 1.000.000 baris.
2. THE Export_Job SHALL menyimpan parameter laporan (bulan, rentang tanggal, vendor, cabang, wilayah, status, klasifikasi) sehingga permintaan bisa diproses ulang, ditampilkan di daftar ekspor, dan diminta ulang dengan parameter sama.
3. THE migrasi yang memperluas `export_jobs` untuk Report_Kind SHALL aditif, memakai nomor migrasi bebas berikutnya setelah `034`, dan tidak mengubah baris Export_Job yang sudah ada.
4. THE ReportProducer SHALL membaca data dari replica dalam satu transaksi `REPEATABLE READ READ ONLY`, sama dengan producer lain.

### Requirement 7: Tombol ekspor di layar laporan

**User Story:** Sebagai pengguna laporan, saya ingin menekan tombol ekspor di layar laporan lalu menunggu pop-up sampai file terunduh, sama seperti ekspor data vendor.

#### Acceptance Criteria

1. WHEN pengguna menekan "Ekspor CSV" di DsrLateReportScreen, THE CompanyPortal SHALL meminta ekspor dengan filter layar saat ini (bulan, vendor, cabang, dan status untuk tab detail) dan membuka ExportProgressDialog.
2. WHEN pengguna menekan "Selisih nominal (CSV)" atau "Trip per ATM (CSV)" di ReplenishmentScreen, THE CompanyPortal SHALL meminta ekspor dengan filter layar saat ini dan membuka ExportProgressDialog.
3. THE ExportProgressDialog SHALL berperilaku sama dengan `vendor-data-export` Requirement 12.8-12.15 (tahap, unduh otomatis sekali, peringatan tutup tab, tombol Tutup, aksesibilitas).
4. IF permintaan ditolak 429, THEN THE layar laporan SHALL menampilkan pesan ikon + teks tanpa membuka dialog.
5. THE pengguna laporan SHALL bisa melihat dan mengunduh manual Export_Job laporannya sendiri yang masih `succeeded` dari daftar Ekspor Saya yang bisa diakses perannya (D4).
6. THE tombol ekspor SHALL tetap nonaktif selama filter layar tidak valid (mis. rentang tanggal > 31 hari), sama dengan perilaku sekarang.

### Requirement 8: Cutover export lama di backend

**User Story:** Sebagai pemilik sistem, saya ingin export laporan lama di `backend/` dihapus begitu CompanyPortal memakai `backend-exq`, supaya hanya ada satu jalur export.

#### Acceptance Criteria

1. WHEN CompanyPortal sudah memakai ReportExportAPI, THE `backend/` SHALL berhenti menerima `format=csv` pada `/dsr-late-reports/{daily,summary}` dan `/replenishment/reports/{amount-mismatch,trips}` dalam rilis yang sama (pola D15 `vendor-data-export`); `format=csv` mendapat 400 dengan pesan yang menunjuk ke ekspor baru.
2. THE `backend/` SHALL menghapus `RecordExport` dan helper `writeCSV` yang tidak lagi dipakai.
3. THE kontrak `backend/api/openapi.yaml` SHALL menghapus parameter `format` dan respons `text/csv` dari `/dsr-late-reports/{daily,summary}`, dan `schema.d.ts` di-generate ulang.
4. THE kontrak `backend-exq/api/openapi.yaml` SHALL mencakup endpoint ReportExportAPI, dengan test kontrak yang lulus.
5. THE fungsi `downloadReport` berbasis `fetch` mentah di `features/dsr-late-report/api.ts` dan `features/replenishment/api.ts` SHALL dihapus dan diganti alur `useExportFlow`.

### Requirement 9: Non-regresi

**User Story:** Sebagai pengembang, saya ingin pemindahan ini tidak mengubah layar laporan, job Python, maupun ekspor master data.

#### Acceptance Criteria

1. THE respons JSON `/dsr-late-reports/{daily,summary,warnings}`, `/replenishment/{classifications,summary}`, `/replenishment/reports/{amount-mismatch,trips}` (tanpa `format=csv`), dan `/vendor/replenish-realizations` SHALL tidak berubah.
2. THE hasil status DSR telat dan perilaku `backend_python/dsr_late_notify` SHALL tidak berubah; test kasus bersama (`internal/repository/testdata/dsr_late_cases.json`) tetap lulus.
3. THE ekspor master data dari `vendor-data-export` SHALL tetap hanya bisa diminta `ADMIN`/`ADMIN_PARAM`.
4. THE implementasi SHALL tidak menambah library baru dan lolos gate coverage yang berlaku (Go ≥ 80% `internal/*` termasuk `backend-exq` dan `pkg/`, CompanyPortal 84/80/80/85).

## Open Questions

PO memilih mengikuti semua saran Kiro pada 2026-10-28.

| # | Pertanyaan | Saran Kiro | Status |
| --- | --- | --- | --- |
| D1 | Apa yang dipindah: hanya CSV, atau seluruh sisi baca laporan (JSON layar + CSV)? | **Hanya CSV.** SQL laporan dipakai bersama lewat sqlc di dua modul (pola `vendor-data-export`), jadi angka file dan layar tetap dari satu query tanpa salinan. Memindahkan layar JSON ikut memindahkan kontrak OpenAPI `/dsr-late-reports`, mengubah path frontend, dan menyentuh kode yang baru stabil (Phase 2.2c/2.3), dengan manfaat kecil. Bisa jadi spec terpisah nanti bila `backend-exq` mulai menjadi rumah query dashboard. | ✅ mengikuti saran (PO 2026-10-28) |
| D2 | Ekspor laporan ikut asinkron (`export_jobs`) seperti ekspor data vendor? | **Ya**, satu mekanisme untuk semua export (D7/D13 `vendor-data-export`). Konsekuensi: tombol ekspor laporan sekarang membuka pop-up dan file diunduh otomatis, bukan langsung. | ✅ mengikuti saran (PO 2026-10-28) |
| D3 | Format file: identik byte dengan sekarang (tanpa BOM), atau ditambah BOM UTF-8 seperti export master data? | **Tambah BOM.** Tanpa BOM, Excel salah membaca nama vendor/cabang yang memakai karakter non-ASCII. Selain BOM, isi tetap identik; test golden membandingkan isi setelah BOM. | ✅ mengikuti saran (PO 2026-10-28) |
| D4 | Di mana pengguna laporan (ATM-USER dll.) melihat daftar Ekspor Saya? Halaman `/settings/admin/exports` hanya untuk ADMIN/ADMIN_PARAM. | **Halaman baru `/exports` "Ekspor Saya"** (hanya daftar + unduh manual, tanpa form), terbuka untuk semua peran yang punya hak ekspor apa pun, ditautkan dari pop-up dan dari layar laporan. Tanpa item sidebar. Halaman admin tetap untuk form ekspor master data. | ✅ mengikuti saran (PO 2026-10-28) |
| D5 | Nama action audit saat unduh | **Tetap `report_exported`** dengan `entity_type` yang sama, supaya auditor bisa mencari riwayat lama dan baru dengan satu filter. Permintaan dicatat sebagai `export_requested` seperti ekspor lain. | ✅ mengikuti saran (PO 2026-10-28) |
| D6 | Peran yang boleh ekspor laporan | **Sama dengan peran melihat laporan hari ini** (Report_Viewer_Roles). Termasuk perbedaan yang ada: BRANCH-ATM-USER boleh replenishment, tidak boleh DSR telat. | ✅ mengikuti saran (PO 2026-10-28) |
| D7 | `DSR_LATE_START_DATE` di `backend-exq` | **Env dengan nilai yang sama** dengan `backend/` dan Python, didokumentasikan di `.env.example` ketiganya. Alternatif menyimpannya di DB lebih aman dari salah konfigurasi, tetapi mengubah tiga runtime dan di luar scope. | ✅ mengikuti saran (PO 2026-10-28) |
| D8 | Urutan pengerjaan | **Setelah `vendor-data-export` di-deploy** dan stabil. Spec ini tidak membangun mekanisme job sendiri. | ✅ mengikuti saran (PO 2026-10-28) |
| D9 | `vendor-data-export` mengunci `/api/v1/exports` dan cek peran unduh ke ADMIN/ADMIN_PARAM. Ubah sekarang? | **Ya, ubah design `vendor-data-export` sebelum diimplementasikan**: guard per route permintaan (master data = ADMIN/ADMIN_PARAM), route `/jobs*` cukup `RequireAuth` + kepemilikan, dan cek peran saat unduh memakai daftar peran per Kind. Perilaku untuk ADMIN/ADMIN_PARAM tidak berubah; menghindari refactor ulang kode yang baru jadi. | ✅ mengikuti saran (PO 2026-10-28) |
| D10 | Kolom parameter laporan di `export_jobs` | **Satu kolom `report_params jsonb`** untuk Report_Kind (bulan, rentang, wilayah, status, klasifikasi), plus perluasan constraint `kind` dan `status_filter` menjadi opsional untuk laporan. Lebih sedikit kolom khusus per laporan; validasi tetap di service dan constraint `CHECK` dasar. | ✅ mengikuti saran (PO 2026-10-28) |
