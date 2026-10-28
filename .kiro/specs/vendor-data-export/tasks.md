# Implementation Plan: Vendor Data Export

## Overview

Urutan implementasi mengikuti risiko. Pertama kunci perilaku lama (golden), lalu pindahkan kontrak CSV ke `pkg/csvexport` dan bagi SQL export, baru bangun `backend-exq` (migrasi, bootstrap, producer, service, worker, janitor, HTTP, OpenAPI). Frontend dikerjakan setelah kontrak OpenAPI ada. Cutover endpoint lama di `backend/` dilakukan paling akhir, setelah CompanyPortal sudah memakai `backend-exq` (D15).

Referensi pola yang dipakai:
- Export lama: `backend/internal/service/masterdata_export.go`, `backend/internal/handler/admin_master_data_export_handler.go` (stream, `ErrAbortHandler`, header unduh).
- Kontrak OpenAPI: `backend/api/` + `backend/internal/handler/openapi_contract_test.go`; tipe TS `frontend/CompanyPortal-Vite/src/lib/api/schema.test.ts`.
- Dialog aksesibel: `frontend/CompanyPortal-Vite/src/components/ui/Dialog.tsx`.
- Migrasi: runner `backend/cmd/migrate` (`go run ./cmd/migrate up`, tidak pernah `psql -f`).
- Dockerfile non-root + HEALTHCHECK: `backend/Dockerfile`.

### Model Split

**Sonnet-runnable**
- 1.1 Golden byte export lama — Sonnet, Medium
- 1.2 Golden header 6 entitas — Sonnet, Low
- 2.1 Paket `pkg/csvexport` — Sonnet, Medium
- 2.2 Test + fuzz `pkg/csvexport` (Property 5) — Sonnet, Medium
- 3.3 Test golden, round trip, regresi importer (Property 3, 4) — Sonnet, High (needs Opus 3.1, 3.2)
- 4 Migrasi `034_export_jobs.sql` — Sonnet, Medium
- 5.1 Bootstrap `backend-exq` (pool replica, TrustedRealIP, ready, env worker) — Sonnet, Medium
- 5.2 Dockerfile non-root + HEALTHCHECK, compose — Sonnet, Low
- 6.1 sqlc `backend-exq` + query export_jobs / vendor_export / audit / user_role — Sonnet, Medium (needs Opus 3.1)
- 6.2 Audit writer `backend-exq` — Sonnet, Medium
- 7.1 `ParseParams`, `ParamsHash`, `FileName` — Sonnet, Medium
- 7.2 Fuzz Property 9, 17 — Sonnet, Medium
- 7.3 Producer vendor / cabang / harga_paket / master_data_global — Sonnet, High
- 7.4 `ChunkWriter` — Sonnet, Medium
- 7.5 Unit test `ChunkWriter` (Property 7) — Sonnet, Medium
- 10.1 Handler + error mapping + streaming unduh — Sonnet, Medium (needs Opus 8)
- 10.2 OpenAPI `backend-exq` + contract test — Sonnet, Medium
- 10.3 Wiring `main.go` (route, worker, janitor, shutdown) — Sonnet, Medium (needs Opus 9)
- 10.4 Handler unit test (Property 17 HTTP) — Sonnet, Medium
- 11.1 Integrasi producer + isi unduhan (Property 1, 2, 6, 7, 8) — Sonnet, High
- 11.2 Integrasi siklus job (Property 10, 11, 12, 15) — Sonnet, High (needs Opus 8, 9)
- 11.3 Integrasi kepemilikan, batas, audit (Property 13, 14, 16) — Sonnet, High (needs Opus 8)
- 11.4 Round trip file level vendor/cabang ke importer (Property 4) — Sonnet, High
- 12.1 Routing Vite + Nginx ke `:8081`, `src/lib/download.ts` — Sonnet, Low
- 12.2 `gen:api:exq` + `exq-schema.d.ts` + test basi — Sonnet, Low
- 13.1 `features/exports` api, hooks, status, autoDownload, beforeunload — Sonnet, Medium
- 13.2 `ExportProgressDialog` — Sonnet, Medium
- 13.3 Form, halaman, route, kartu hub — Sonnet, Medium
- 13.4 Tabel Ekspor Saya — Sonnet, Medium
- 13.5 Test komponen + Property 18 — Sonnet, Medium
- 14.1 `master-data-io` memakai flow asinkron — Sonnet, Medium
- 14.2 Hapus endpoint export lama di `backend/` (cutover) — Sonnet, Medium
- 15 CI: integrasi + sqlc + coverage `backend-exq` — Sonnet, Medium
- 16 Checkpoint gate kualitas — Sonnet, Medium
- 17 Dokumentasi — Haiku, Low (Haiku)
- 18 Cek browser manual (user) — Haiku, Low (Haiku)

**Opus-required**
- 3.1 Bagi SQL export + filter cakupan — Opus, High — kontrak lintas modul; salah filter = bocor data vendor lain atau importer rusak
- 3.2 Refactor importer `backend/` ke `pkg/csvexport` — Opus, High — importer master data (jalur maker-checker); salah format sel = file tak diedit terbaca update
- 8.1 `Service.Create` (advisory lock, dedupe, limit, audit dalam tx) — Opus, High — auth + audit fail-closed + konkurensi
- 8.2 `Service.List` / `Get` (kepemilikan) — Opus, High — akses data pribadi lintas user
- 8.3 `StartDownload` / `CompleteDownload` / `AbortDownload` — Opus, High — mesin status satu-kali-unduh + audit + cek peran dari DB
- 9.1 Worker (klaim SKIP LOCKED, run, shutdown) — Opus, High — konkurensi multi-instance + snapshot replica
- 9.2 Janitor (timeout, lease, kedaluwarsa) — Opus, High — penghapusan data pribadi tepat waktu + transisi status

Dependensi lintas grup: Sonnet 3.3 dan 6.1 menunggu Opus 3.1 (dan 3.2 untuk 3.3). Sonnet 10.1 dan 11.3 menunggu Opus 8. Sonnet 10.3 dan 11.2 menunggu Opus 8 dan 9. Opus 8.1 menunggu Sonnet 6.2 dan 7.1. Opus 9.1 menunggu Sonnet 7.3 dan 7.4.

## Tasks

- [ ] 1. Kunci perilaku export lama sebelum disentuh
  - _Model: Sonnet, Medium — rekam output kode lama apa adanya; tidak ada logika baru._
  - [ ] 1.1 Rekam golden byte export master data global
    - Buat fixture DB deterministik (vendor, cabang, vault, PIC dengan `roles`, ATM, kelolaan; aktif + nonaktif; nilai NULL, numeric, sel berawalan `=`/`+`/`-`).
    - Jalankan `MasterDataExporter.Write` lama untuk 6 entitas × `active|disabled|all`, simpan ke `backend/internal/service/testdata/master_data_export/<entity>-<status>.csv`.
    - Test integrasi `backend/` yang membandingkan output lama dengan golden (lulus sebelum refactor).
    - _Requirements: 13.1_
  - [ ] 1.2 Rekam golden header 6 entitas
    - Simpan header + legacy header dari `exportSpecs` ke `testdata/headers.golden` (dipakai ulang di 2.2).
    - _Requirements: 5.4_
    - _Model: Sonnet, Low — salin header apa adanya._

- [ ] 2. Paket kontrak CSV `pkg/csvexport`
  - _Model: Sonnet, Medium — memindahkan kode murni apa adanya; tanpa DB._
  - [ ] 2.1 Buat `pkg/csvexport`
    - Konstanta entitas, `BOM`, `KeepCurrentCell`, `AssignmentSourceBranch`, `PICRolesNone`; tipe `Entity` + `Lookup`, `ImportableNames`, `ExportOnlyNames`.
    - Tipe baris netral + fungsi `*Cells` untuk 6 entitas, disalin persis dari closure `exportSpecs`.
    - `CSVSafe`, `UnCSVSafe`, `Writer` (BOM + header + `CSVSafe` per sel), `WriteTemplate`.
    - Header export-only `dsr-location-maps` dan `package-prices` sesuai design (Data Models).
    - Belum ada perubahan di `backend/`.
    - _Requirements: 5.5, 5.6, 14.2, 14.3_
  - [ ]* 2.2 Test + fuzz `pkg/csvexport`
    - **Property 5: CSVSafe aman dan bisa dibalik** (fuzz `testing.F` + seed corpus).
    - Snapshot header 6 entitas terhadap `headers.golden`; `WriteTemplate` = BOM + header saja.
    - _Requirements: 5.4, 5.6, 14.2_

- [ ] 3. Bagi SQL export dan pindahkan importer ke kontrak bersama
  - _Model: Opus, High — kontrak lintas modul dan importer master data; salah filter membocorkan data vendor lain, salah format membuat import salah membaca perubahan._
  - [ ] 3.1 Bagi SQL dan tambah filter cakupan
    - Pindahkan `ImportPackageKeys`, `ImportVendorPackageLabels`, `ImportLocationIDs` ke `backend/queries/master_data_import.sql`.
    - Tambah `sqlc.narg('vendor_id')` / `sqlc.narg('branch_id')` pada `ExportVendorsBatch`, `ExportVendorBranchesBatch`, `ExportVendorVaultsBatch`, `ExportVendorPicsBatch` sesuai tabel design; ATM dan kelolaan tidak berubah.
    - `sqlc generate` (v1.31.1) di `backend/` + perbaikan `UserLeafe`.
    - _Requirements: 1.3, 2.3, 4.3, 4.4, 13.2_
  - [ ] 3.2 Refactor `backend/` ke `pkg/csvexport`
    - `exportSpecs` jadi adaptor (baris sqlc -> `csvexport.*Row` -> `*Cells`); header/legacy dari `csvexport.Lookup`.
    - Importer (`loadAll`, validasi header, strip apostrof) memakai `csvexport`; repository batch menerima scope `nil, nil`.
    - `admin_master_data_import_handler.go` memakai `csvexport.Lookup`. Endpoint export lama tetap jalan sampai task 14.2.
    - _Requirements: 13.2, 14.3, 14.4_
  - [ ] 3.3 Test golden, round trip global, regresi importer
    - _Model: Sonnet, High — test atas kode Opus; perlu teliti pada kasus legacy header._
    - **Property 3: Export global byte-identik dengan export lama** (jalur baru `csvexport` + SQL bersama vs golden 1.1).
    - **Property 4: Round trip ke importer** untuk 6 entitas global (file tanpa diedit -> dry-run `unchanged` semua).
    - Seluruh test import yang ada tetap lulus.
    - _Requirements: 13.1, 13.2, 13.3_

- [ ] 4. Migrasi `034_export_jobs.sql`
  - Cek ulang nomor bebas di `backend/migrations/` (terakhir `033`, `022` dicadangkan).
  - Tabel `export_jobs` + `export_job_chunks`, constraint, indeks, trigger `updated_at`, komentar persis seperti design.
  - Terapkan dengan `go run ./cmd/migrate up`; `migrate status` 0 pending.
  - _Requirements: 9.2, 9.3, 10.1, 11.8_
  - _Model: Sonnet, Medium — DDL aditif tanpa data lama; constraint sudah ditulis di design._

- [ ] 5. Bootstrap `backend-exq`
  - _Model: Sonnet, Medium — wiring infrastruktur mengikuti pola `backend/`._
  - [ ] 5.1 Pool replica, middleware, readiness, konfigurasi worker
    - Pool replica dengan fallback primary; `/ready` cek `db`, `db_replica`, `redis`.
    - `TrustedRealIP(cfg.TrustedProxies)` sebagai middleware pertama; `chi.Timeout(30s)` dipindah ke grup route.
    - `exportjob.LoadWorkerConfig` (`EXPORT_WORKERS` 1-8, `EXPORT_POLL_INTERVAL`, `EXPORT_JANITOR_INTERVAL` ≤ 1m; nilai tidak valid = gagal start); `.env.example` diperbarui.
    - Pastikan tidak ada import dari `backend/` (`go list -deps`).
    - _Requirements: 11.1, 11.2, 11.5, 11.7_
  - [ ] 5.2 Dockerfile dan compose
    - Non-root `appuser`, `CGO_ENABLED=0`, `TZ=Asia/Jakarta`, HEALTHCHECK `/health` :8081; compose `name: CMS-Backend-EXQ`.
    - _Requirements: 11.4_
    - _Model: Sonnet, Low — meniru `backend/Dockerfile`._

- [ ] 6. Lapisan data `backend-exq`
  - _Model: Sonnet, Medium — query SQL-first mengikuti design._
  - [ ] 6.1 sqlc dan query
    - `sqlc.yaml`: `queries: ["queries/", "../backend/queries/master_data_export.sql"]`, schema `../backend/migrations/`.
    - `queries/export_jobs.sql` (insert, dedupe, hitung aktif, klaim SKIP LOCKED, transisi bersyarat, list/get milik user, chunk insert/read/delete, janitor), `queries/vendor_export.sql` (`dsr-location-maps`, `package-prices` keyset; numeric/tanggal `::text`, NULL `''`), `queries/audit.sql`, `queries/user_role.sql` (peran aktif user dari DB).
    - `sqlc generate` di `backend-exq`.
    - _Requirements: 3.3, 3.4, 3.5, 5.5, 5.7, 6.2_
  - [ ] 6.2 Audit writer `backend-exq/internal/audit`
    - `Write(ctx, q DBTX, Entry)` hanya insert; tolak `ActorID == 0`.
    - _Requirements: 8.4, 8.10_

- [ ] 7. Inti ekspor: parameter, producer, penyimpanan potongan
  - _Model: Sonnet, High — producer menentukan cakupan data pribadi yang keluar; banyak kombinasi level/entitas/status._
  - [ ] 7.1 `ParseParams`, `ParamsHash`, `FileName`
    - Enum Kind/entitas per Kind, default status `active` dan `price_level` `all`, `ErrValidation` vs `ErrUnknownEntity`, `sanitize`, tanggal WIB.
    - _Requirements: 1.2, 1.4, 2.2, 4.1, 4.2, 5.2_
    - _Model: Sonnet, Medium — fungsi murni dengan enum dari design._
  - [ ]* 7.2 Fuzz parameter dan nama file
    - **Property 9: Nama file aman**; **Property 17: Validasi parameter** (bagian fungsi murni).
    - _Requirements: 4.1, 4.2, 5.2_
    - _Model: Sonnet, Medium — fuzz fungsi murni._
  - [ ] 7.3 Producer
    - `vendor` (5 entitas), `cabang` (3 entitas, PIC cabang saja), `harga_paket` (filter cabang, tingkat harga, status per periode vs `asOf` WIB), `master_data_global` (6 entitas tanpa scope); semua via `csvexport` header/cells, keyset `PageSize`, urut id.
    - File header-only bila kosong; vendor nonaktif tetap boleh; baris profil vendor selalu ada.
    - _Requirements: 1.3, 1.5, 1.6, 2.3, 2.5, 3.1, 3.2, 3.3, 3.4, 3.5, 4.3, 4.4, 4.5, 4.6, 5.3, 5.7, 5.8, 14.1_
  - [ ] 7.4 `ChunkWriter`
    - Buffer ≤ 1 MiB, insert potongan berurutan, hitung byte, `ErrFileTooLarge` > 100 MiB; batas 1.000.000 baris di `emit`.
    - _Requirements: 6.4, 10.1, 10.11_
    - _Model: Sonnet, Medium — io.Writer berbatas, logika sempit._
  - [ ]* 7.5 Unit test `ChunkWriter`
    - **Property 7: Isi unduhan sama dengan yang ditulis** (sisi penulis, sink palsu: batas potongan, nol baris, batas ukuran).
    - _Requirements: 6.4, 10.1, 10.11_
    - _Model: Sonnet, Medium — unit test dengan sink palsu._

- [ ] 8. Service Export_Job
  - _Model: Opus, High — auth, audit fail-closed, konkurensi, dan mesin status satu kali unduh._
  - [ ] 8.1 `Service.Create`
    - Registry `KindSpecs` (peran, action audit unduh, entity_ref per Kind; D23). Cek `UserID` (401) dan peran terhadap `KindRoles[p.Kind]` (403); resolusi vendor/cabang di primary (404 yang sama untuk dua kasus).
    - Tx: advisory lock per user, dedupe `params_hash` (200, tanpa audit), batas 3 aktif (429), insert job `queued`, insert audit `export_requested` (gagal = rollback, 500), lalu `wake()`.
    - `entity_type`/`entity_id`/`after` sesuai design, termasuk `username`, `role`, IP.
    - _Requirements: 1.1, 1.6, 2.1, 2.4, 3.2, 7.2, 7.3, 8.1, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8, 9.1, 9.3, 9.4, 9.5, 14.7_
  - [ ] 8.2 `Service.List` dan `Get`
    - Hanya job milik pemanggil, baca dari primary; job user lain = 404.
    - _Requirements: 6.3, 7.5_
  - [ ] 8.3 `StartDownload`, `OpenChunks`, `CompleteDownload`, `AbortDownload`
    - Cek peran saat ini dari `users`/`roles` terhadap `KindRoles[job.Kind]` (403); klaim atomik `succeeded -> downloading` dengan `expires_at > now()`; 409/410 tanpa audit; audit `master_data_exported` + `row_count` sebelum byte pertama (gagal = abort, 500).
    - Complete: satu tx hapus potongan + `downloaded`; Abort: `downloading -> succeeded`.
    - _Requirements: 7.6, 8.2, 9.11, 10.2, 10.3, 10.4, 10.9_

- [ ] 9. Worker dan janitor
  - _Model: Opus, High — konkurensi multi-instance, snapshot replica, dan penghapusan data pribadi tepat waktu._
  - [ ] 9.1 Worker
    - N goroutine, klaim `FOR UPDATE SKIP LOCKED`, sinyal bangun + polling.
    - Run: timeout 10 menit, tx replica `REPEATABLE READ READ ONLY`, update `row_count` per halaman, sukses = tx `succeeded` + `expires_at = now()+10m`; gagal = hapus potongan + `failed` dengan pesan ringkas (batas, timeout, konflik replica, umum); log terstruktur tanpa isi data.
    - Shutdown: berhenti klaim, tunggu ≤ 10 detik, lepas sisa job ke `queued` + hapus potongan.
    - _Requirements: 6.1, 6.2, 6.3, 9.6, 9.7, 9.8, 9.9, 9.10, 9.12, 10.8, 10.11_
  - [ ] 9.2 Janitor
    - Urutan per tick: `running` > 10 menit -> `failed`; `downloading` > 5 menit -> `succeeded`; `succeeded` lewat `expires_at` -> `expired` + hapus potongan (satu statement); sapu potongan yatim.
    - _Requirements: 9.9, 10.5, 10.6, 10.7_

- [ ] 10. HTTP API, OpenAPI, dan wiring
  - _Model: Sonnet, Medium — handler tipis di atas service; pola stream dan kontrak sudah ada di `backend/`._
  - [ ] 10.1 `exports_handler.go`
    - Route sesuai tabel design (POST per Kind, template sinkron tanpa DB/audit, list, get, file).
    - Pemetaan error -> `pkg/response`; id path tidak valid 400.
    - Unduh: header (`text/csv`, `Content-Disposition`, `Content-Length`, `no-store`, `nosniff`), stream potongan dengan write deadline per potongan, error di tengah = `AbortDownload` + `ErrAbortHandler`, selesai = `CompleteDownload`.
    - _Requirements: 5.1, 5.9, 7.1, 7.4, 8.11, 9.13, 9.14, 10.10, 14.2_
  - [ ] 10.2 Kontrak OpenAPI `backend-exq`
    - `api/openapi.yaml` (3.0.3) + `api/embed.go`; skema Job, envelope, error datar middleware, enum, respons `text/csv`.
    - `openapi_contract_test.go` (spec valid, coverage route dua arah, respons tervalidasi); `kin-openapi v0.151.0` di `go.mod` `backend-exq` (satu-satunya dependensi tambahan).
    - _Requirements: 13.4, 15.1, 15.2_
  - [ ] 10.3 Wiring `cmd/api/main.go`
    - Grup `/api/v1/exports` dengan `RequireAuth`; `RequireRoles("ADMIN","ADMIN_PARAM")` hanya pada route permintaan master data + template; `/jobs*` cukup `RequireAuth` (D23); timeout 30 detik vs 5 menit untuk unduh.
    - Start worker + janitor; urutan shutdown sesuai design.
    - _Requirements: 7.1, 11.7_
  - [ ]* 10.4 Unit test handler
    - Tabel kasus dengan service palsu: status code per error, header unduh, abort di tengah stream, template tanpa audit.
    - **Property 17: Validasi parameter** (lapisan HTTP).
    - _Requirements: 4.2, 5.1, 8.11, 9.13, 9.14_

- [ ] 11. Integrasi `backend-exq` dengan Postgres nyata
  - _Model: Sonnet, High — test property berbasis DB dengan data acak ber-seed; perlu teliti pada konkurensi._
  - [ ] 11.1 Producer dan isi unduhan
    - **Property 1: Isolasi cakupan**; **Property 2: Partisi Status_Filter**; **Property 6: Determinisme dan urutan**; **Property 7: Isi unduhan sama dengan yang ditulis** (ujung ke ujung, lintas batas potongan); **Property 8: Uang dan kapasitas sebagai teks persis**.
    - Tx replica `REPEATABLE READ READ ONLY` terbukti dipakai.
    - _Requirements: 1.3, 1.5, 2.3, 3.1, 3.2, 3.3, 3.4, 4.3, 4.4, 4.5, 4.6, 5.7, 5.8, 5.9, 6.1, 6.2_
  - [ ] 11.2 Siklus job
    - **Property 10: Mesin status hanya mengikuti transisi sah**; **Property 11: Satu kali unduh**; **Property 12: Masa simpan dan pemulihan janitor** (jam yang bisa diatur); **Property 15: Klaim eksklusif** (dua instance Worker).
    - _Requirements: 9.2, 9.6, 9.7, 9.8, 9.9, 9.10, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8_
  - [ ] 11.3 Kepemilikan, batas, audit
    - **Property 13: Kepemilikan job**; **Property 14: Batas aktif dan dedupe** (permintaan bersamaan); **Property 16: Kelengkapan dan isi audit** (termasuk injeksi kegagalan audit dan baris yang terbaca lewat query viewer `ListAuditLogs`).
    - _Requirements: 7.5, 7.6, 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8, 8.9, 9.4, 9.5_
  - [ ] 11.4 Round trip file level vendor/cabang ke importer
    - Di `backend/` (tag `integration`): baris dari SQL bersama dengan scope vendor/cabang -> `csvexport.Writer` -> dry-run importer = `unchanged` semua.
    - **Property 4: Round trip ke importer** (entitas `vendor`, `branches`, `vaults`, `pics`).
    - _Requirements: 5.3, 13.3_

- [ ] 12. Routing dan tipe CompanyPortal
  - _Model: Sonnet, Low — konfigurasi proxy dan codegen mengikuti pola yang ada._
  - [ ] 12.1 Proxy dan helper unduh
    - `vite.config.ts`: `/api/v1/exports` -> `:8081` di atas `/api`; `nginx.conf`: `location /api/v1/exports/` di atas `/api/`, `proxy_buffering off`, `proxy_read_timeout 300s`.
    - Pindahkan helper unduh blob ke `src/lib/download.ts` (perilaku master-data-io belum berubah).
    - _Requirements: 11.3_
  - [ ] 12.2 Tipe dari kontrak `backend-exq`
    - Script `gen:api:exq` -> `src/lib/api/exq-schema.d.ts` (di-commit) + `exq-schema.test.ts` (gagal bila basi).
    - _Requirements: 15.3, 15.4_

- [ ] 13. Fitur `features/exports`
  - _Model: Sonnet, Medium — komponen React dengan pola TanStack Query + `Dialog` yang ada._
  - [ ] 13.1 Api, hooks, status, autoDownload, beforeunload
    - `api.ts`, `types.ts` (dari `exq-schema.d.ts`), `status.ts` (label, ikon, tone), `autoDownload.ts` (`claimAutoDownload` per tab), `useBeforeUnloadGuard`, `useMyExports` (polling 3 detik selama ada job aktif), `useExportJob`, `useRequestExport`, `useExportFlow`.
    - _Requirements: 12.7, 12.9, 12.11, 12.13_
  - [ ] 13.2 `ExportProgressDialog`
    - Fase Antri / Membuat file (+ baris) / Mengunduh / Selesai / Gagal (+ Coba lagi); polling 1 detik; unduh otomatis sekali; beforeunload selama menunggu/mengunduh.
    - Tombol Tutup selalu ada + penjelasan 10 menit; live region; `Dialog` (focus trap, fokus kembali, tanpa blur); ikon memuat hormati `prefers-reduced-motion`.
    - _Requirements: 12.8, 12.9, 12.10, 12.11, 12.12, 12.13, 12.15_
  - [ ] 13.3 Halaman, form, route, kartu hub
    - `ExportDataPage` (Data Vendor, Harga Paket, Ekspor Saya terpisah), `VendorExportForm` (level -> entitas, cabang wajib untuk level Cabang, status default Aktif), `PackagePriceExportForm` (cabang opsional, tingkat harga default Semua).
    - 429 inline (ikon + teks) tanpa membuka dialog; satu tombol primer per menu; label di setiap input.
    - Route lazy `/settings/admin/exports` dengan `requireRoles(["ADMIN","ADMIN_PARAM"])`; kartu "Export Data" di `SettingsHubPage.tsx`, tanpa item sidebar.
    - _Requirements: 3.6, 12.1, 12.2, 12.3, 12.4, 12.5, 12.16, 12.18_
  - [ ] 13.4 `MyExportsTable`
    - Kolom dan badge status sesuai design (ikon + teks, tanpa merah brand), sisa waktu unduh, tombol Unduh manual + keterangan sekali unduh, "Minta ekspor lagi" untuk `downloaded`/`expired`; angka `tabular-nums` rata kanan.
    - _Requirements: 12.6, 12.14, 12.17_
  - [ ]* 13.5 Test komponen dan property
    - Form, 429, tabel, dialog (fase, live region, Tutup, beforeunload), master-data-io memakai flow baru.
    - **Property 18: Unduh otomatis paling banyak sekali** (fast-check urutan status acak).
    - _Requirements: 12.3, 12.5, 12.8, 12.9, 12.10, 12.11, 12.12, 12.13, 12.15, 12.16, 14.6_

- [ ] 14. Cutover export master data global
  - _Model: Sonnet, Medium — mengganti pemanggil lalu menghapus kode lama; urutan rilis penting._
  - [ ] 14.1 `master-data-io` memakai flow asinkron
    - "Unduh data" -> "Minta ekspor" via `useExportFlow` (`master_data_global`), tautan ke Ekspor Saya, template dari `/exports/master-data/{entity}/template`; dry-run/confirm tetap ke `backend/`.
    - _Requirements: 14.6_
  - [ ] 14.2 Hapus endpoint export lama di `backend/`
    - Hapus `admin_master_data_export_handler.go` (+ test), mount di `main.go`, dan fungsi export/template yang tidak lagi dipakai; `/api/v1/admin/master-data/export/*` = 404. Golden 1.1 tetap dijaga oleh 3.3.
    - _Requirements: 14.5_

- [ ] 15. CI untuk `backend-exq`
  - Job `go-integration`: `go test -tags integration` juga di `backend-exq` (tanpa skip) + gate coverage `backend-exq/coverage-thresholds.txt` dan `pkg`.
  - Job `sqlc`: generate + `git diff --exit-code` untuk `backend` dan `backend-exq`.
  - _Requirements: 11.6, 13.5_
  - _Model: Sonnet, Medium — perubahan workflow mengikuti job yang ada._

- [ ] 16. Checkpoint: gate kualitas
  - `go test ./...` + `-tags integration` di `backend`, `backend-exq`, `pkg`; `golangci-lint`, `govulncheck`; coverage Go ≥ 80% `internal/*` dan `pkg/`.
  - CompanyPortal: `pnpm run lint`, `pnpm run test:coverage` (84/80/80/85), `pnpm run build`, `gen:api` dan `gen:api:exq` bersih.
  - Docker build `backend` dan `backend-exq`; image exq berjalan non-root dan healthy.
  - Tidak ada dependensi baru selain `kin-openapi` di `backend-exq`; `backend-exq` tidak mengimpor `backend/`.
  - _Requirements: 11.4, 11.5, 13.4, 13.5_
  - _Model: Sonnet, Medium — triase kegagalan gate._

- [ ] 17. Dokumentasi
  - `.claude/CLAUDE.md` Sec 3 + `.claude/docs/data-map.md` (`export_jobs`, `export_job_chunks`), `tech.md` / `structure.md` (`pkg/csvexport`, `backend-exq` kini punya konten), `rbac-role-feature-matrix.md` (route `backend-exq` ber-guard, route `/settings/admin/exports`), `development-plan.md` (X.7), `.claude/development-progress.md`.
  - _Requirements: 11.8_
  - _Model: Haiku, Low — pembaruan dokumen berdasarkan hasil yang sudah ada._

- [ ] 18. Cek browser manual (user)
  - Docker: CompanyPortal (Nginx) -> `backend-exq` :8081; ekspor level vendor, cabang, Harga Paket, dan master data global; pop-up loading + unduh otomatis (Chrome/Edge, izin multi-download); tutup pop-up lalu unduh manual; tunggu > 10 menit -> Kedaluwarsa; unduh kedua ditolak.
  - Log Audit: filter `export_requested` / `master_data_exported` menampilkan user, IP, waktu.
  - Screen reader membaca tahap dialog; keyboard-only.
  - _Requirements: 8.9, 11.3, 12.1, 12.8, 12.9, 12.12, 12.14, 12.15, 12.18_
  - _Model: Haiku, Low — dijalankan user (Golden Rule #10); agen hanya menyiapkan daftar cek._

## Notes

- Task bertanda `*` adalah test opsional tetapi direkomendasikan; property test yang menjaga data pribadi dan audit (11.x) tidak bertanda opsional.
- Golden pada task 1 **harus** direkam sebelum task 3 menyentuh `backend/`. Bila golden direkam setelah refactor, Property 3 tidak lagi membuktikan apa pun.
- Endpoint lama di `backend/` hanya dihapus (14.2) setelah 14.1 selesai dan keduanya masuk rilis yang sama (D15). Rollback = deploy ulang image sebelumnya untuk `backend` dan CompanyPortal.
- `backend-exq` belum pernah di-deploy. Rilis spec ini sekaligus deployment pertamanya: siapkan `.env` (termasuk `TRUSTED_PROXIES` = alamat Nginx, `DATABASE_REPLICA_URL`, `JWT_SECRET` sama dengan `backend`) sebelum cutover.
- Migrasi `034` aditif dan bisa diterapkan lebih dulu tanpa efek ke fitur lain.
- Spec lanjutan `report-export-migration` (X.8) memakai `export_jobs`, worker, dan `ExportProgressDialog` dari spec ini.

### Definition of Done

- Semua task 1-17 selesai, gate 16 hijau, task 18 dicentang user.
- Semua Correctness Property 1-18 punya test yang lulus.
- `development-plan.md` X.7 diperbarui ke "built", X.8 tetap planned.

## Task Dependency Graph

- **Wave 0:** 1.1, 1.2 (golden, sebelum kode lama disentuh), 4 (migrasi, independen).
- **Wave 1:** 2.1 (butuh 1.2), 5.1, 5.2 (bootstrap exq), 12.1 (proxy + helper unduh).
- **Wave 2:** 2.2 (butuh 2.1), 3.1 (butuh 2.1).
- **Wave 3:** 3.2 (butuh 3.1), 6.1 (butuh 3.1, 4, 5.1).
- **Wave 4:** 3.3 (butuh 1.1, 3.2), 6.2 (butuh 6.1), 7.1 (butuh 6.1), 7.4 (butuh 6.1).
- **Wave 5:** 7.2 (butuh 7.1), 7.3 (butuh 2.1, 6.1, 7.1), 7.5 (butuh 7.4), 8.1 (butuh 6.2, 7.1).
- **Wave 6:** 8.2 (file service yang sama dengan 8.1), 9.1 (butuh 7.3, 7.4).
- **Wave 7:** 8.3 (butuh 8.2), 9.2 (butuh 9.1).
- **Wave 8:** 10.1 (butuh 8.3).
- **Wave 9:** 10.2 (butuh 10.1), 10.3 (butuh 9.2, 10.1), 10.4 (butuh 10.1).
- **Wave 10:** 11.1, 11.2, 11.3 (butuh 10.3), 11.4 (butuh 3.2, 7.3), 12.2 (butuh 10.2).
- **Wave 11:** 13.1 (butuh 12.1, 12.2).
- **Wave 12:** 13.2, 13.4 (butuh 13.1).
- **Wave 13:** 13.3 (butuh 13.2, 13.4), 14.1 (butuh 13.2).
- **Wave 14:** 13.5 (butuh 13.3, 14.1), 14.2 (butuh 14.1).
- **Wave 15:** 15 (butuh 11.x, 14.2).
- **Wave 16:** 16 (gate, butuh semua task sebelumnya).
- **Wave 17:** 17 (dokumentasi, butuh 16).
- **Wave 18:** 18 (cek browser user, butuh 17).

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "4"] },
    { "id": 1, "tasks": ["2.1", "5.1", "5.2", "12.1"] },
    { "id": 2, "tasks": ["2.2", "3.1"] },
    { "id": 3, "tasks": ["3.2", "6.1"] },
    { "id": 4, "tasks": ["3.3", "6.2", "7.1", "7.4"] },
    { "id": 5, "tasks": ["7.2", "7.3", "7.5", "8.1"] },
    { "id": 6, "tasks": ["8.2", "9.1"] },
    { "id": 7, "tasks": ["8.3", "9.2"] },
    { "id": 8, "tasks": ["10.1"] },
    { "id": 9, "tasks": ["10.2", "10.3", "10.4"] },
    { "id": 10, "tasks": ["11.1", "11.2", "11.3", "11.4", "12.2"] },
    { "id": 11, "tasks": ["13.1"] },
    { "id": 12, "tasks": ["13.2", "13.4"] },
    { "id": 13, "tasks": ["13.3", "14.1"] },
    { "id": 14, "tasks": ["13.5", "14.2"] },
    { "id": 15, "tasks": ["15"] },
    { "id": 16, "tasks": ["16"] },
    { "id": 17, "tasks": ["17"] },
    { "id": 18, "tasks": ["18"] }
  ]
}
```
