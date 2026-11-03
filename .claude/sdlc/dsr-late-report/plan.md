# Plan: Laporan DSR telat / tidak kirim — dasar penalti FLM (Phase 2.3)

Author: Claude, dari `spec.md` (accepted 2026-10-27, S1–S5 + K1–K5). Status: **accepted 2026-10-27** (engineer).
Stage: 3 Build. Stage berikut: `tests.md`.

## Titik sentuh yang sudah dicek di kode (2026-10-27)
- **Ingest DSR**: `backend_python/dsr/dsr_etl.py` memakai psycopg **sync**. `_upsert_upload(conn, path, vendor_name, uploaded_by)` (±baris 482) menulis `dsr_uploads` + `dsr_daily_rows` + `dsr_rencana_isi_rows` dalam satu `conn.transaction()`. Saat revisi, baris lama di-`DELETE`. Checksum yang sama → `"skipped"`. Fungsi ini dipanggil dari `process_file()` oleh dua jalur:
  - `--mode commit` (konfirmasi vendor dari Go → `service_dsr_etl` `/process/dsr/commit`);
  - batch-drain tanpa `--mode` (file yang diletakkan langsung di `FTP_DATA/DSR`, cron safety net).
  Tes yang ada (`dsr/test_dsr_etl.py`) murni parser, tanpa DB.
- **Pola K3 Python**: `backend_python/replenish_classify/job.py`:
  - `_notify_late` menulis `notifications` + `notification_emails` (`pending` bila `RETRY_NOTIFY_EMAIL_ENABLED`, selain itu `skipped`);
  - `run_once` menulis `retry_audit_logs`;
  - kunci: `pg_try_advisory_xact_lock`;
  - CLI `run.py`;
  - integration test `test_job_integration.py` (tx di-rollback, skip tanpa `DATABASE_URL`).
- **Scheduler**: `backend_python/lib/services/scheduler_service.py` `start()` mendaftarkan job APScheduler `CronTrigger(..., timezone=WIB)`. Klasifikasi 2.2c terdaftar dengan `CLASSIFY_CRON` hanya bila sumbernya ada di `file_types`. Settings: `eod_retry_scheduler/config.py` (`env_prefix="RETRY_"`, `replenish_classify_start_date`, `notify_email_enabled`). `service_dsr_etl` memakai `SchedulerService` yang sama dengan Settings lain.
- **Go baca + ekspor CSV + audit**: `internal/service/replenishment_read.go` (`ReplenishmentReadService`, `authorize`, `RecordExport` → `report_exported`), `internal/handler/replenishment_handler.go` (`?format=csv`), mount `/api/v1/replenishment` di `cmd/api/main.go` ±baris 480. Env Go dibaca dengan `getenvDefault` di `main.go`.
- **Penerima vendor**: `backend/queries/notifications.sql` → `ListActiveUserRecipientsByVendorBranches`, `ListVendorBranchNotificationPicEmails` (aturan yang disalin ke Python, K3).
- **OpenAPI**: `backend/api/openapi.yaml` + `internal/handler/openapi_contract_test.go` (route set == spec, kasus respons per operasi) + CompanyPortal `pnpm run gen:api` → `src/lib/api/schema.d.ts` (dijaga vitest freshness).
- **Frontend**: `src/lib/config/navigation.ts` (`NAV_CONFIG`, grup `monitoring`, `roles`); pola layar + CSV: `features/replenishment/{ReplenishmentScreen.tsx,api.ts,__tests__/}`.
- **Migrasi**: `backend/cmd/migrate status|up` (Sec 2a); file terakhir `032_vendor_party_scope.sql`. Penjaga `internal/repository/no_hard_delete_test.go`.

## Keputusan plan (accepted engineer 2026-10-27)
- **D1** Receipt ditulis di `_upsert_upload` (satu titik), jadi file yang masuk lewat batch-drain (`FTP_DATA/DSR` langsung) juga menjadi receipt, dengan `received_at` = saat ingest. Spec FR1.1 hanya menyebut `--mode commit`. Alternatif: lewatkan flag dari `main()` sehingga hanya commit yang menulis receipt. Usulan: satu titik, karena file batch-drain juga DSR yang benar-benar diterima.
- **D2** SQL `obliged`/`hits` (spec "Bentuk query") ditulis **sekali** sebagai SQL di `backend/queries/dsr_late_report.sql` (sqlc). Python menyalin SQL yang sama ke `dsr_late_notify/queries.py` dengan komentar silang. Satu fixture kasus (`testdata`) dipakai oleh tes Go dan Python (R2). Tidak dibuat VIEW/fungsi SQL di DB, supaya migrasi 033 tetap hanya satu tabel.
- **D3** Konversi tanggal WIB di SQL: `(ts AT TIME ZONE 'Asia/Jakarta')::date` untuk `created_at`/`deleted_at`/`received_at`; deadline = `(X + time '09:00') AT TIME ZONE 'Asia/Jakarta'`. Tidak bergantung pada TZ sesi DB.

## Slices (satu concern per slice, tes hijau sebelum lanjut)

### P1 — Migrasi 033 `dsr_receipts` (spec Data model, S1) · **sonnet, effort low**
- `backend/migrations/033_dsr_receipts.sql`: `BEGIN; CREATE TABLE public.dsr_receipts (...)` sesuai spec (CHECK checksum sha256 hex, UNIQUE `(vendor_label, report_date, checksum)`, indeks `(report_date, vendor_id)`, FK `vendors`, `users`) + `COMMENT ON` tabel/kolom; `COMMIT;`. Additive saja.
- Append-only: tambahkan `dsr_receipts` ke `internal/repository/no_hard_delete_test.go` (tidak ada `DELETE FROM dsr_receipts` di kode Go). Python tidak pernah menulis `UPDATE`/`DELETE` untuk tabel ini (ditinjau di P2).
- `cd backend && go run ./cmd/migrate status` (harus 1 pending: 033). **`migrate up` ke dev hanya setelah OK user** (Sec 4 #7, CLAUDE.md 2a).
- Bukti: `migrate status` sebelum/sesudah; `go test ./internal/repository -run NoHardDelete`; CI `go-integration` membangun DB lewat `migrate up` (033 ikut).
- Risiko: tidak ada data yang diubah; rollback = `DROP TABLE` di migrasi baru bila perlu.

### P2 — Receipt di ingest DSR (FR1, K2, D1) · **sonnet, effort medium** · butuh P1
- `backend_python/dsr/dsr_etl.py`:
  - di `_upsert_upload`, setelah `INSERT dsr_uploads` dan hanya bila `daily_status == "completed"`, `INSERT INTO dsr_receipts (...) ON CONFLICT (vendor_label, report_date, checksum) DO NOTHING` dalam `conn.transaction()` yang sama;
  - `vendor_id` di-resolve dengan `SELECT id FROM vendors WHERE name = %s` (NULL bila tidak ada, FR1.4);
  - `locations` = distinct `btrim(row["location"])` non-kosong dari `daily_rows`;
  - `received_at` default `now()`.
  - Jalur `skipped` dan dry-run tidak berubah.
- Tes:
  - unit (stdlib `unittest`, tanpa DB): fungsi murni `receipt_locations(daily_rows)`;
  - integration baru `dsr/test_dsr_receipts_integration.py` (skip tanpa `DATABASE_URL`, tx rollback): sukses → 1 receipt; failed Daily → 0; revisi → 2 receipt, yang pertama tidak berubah; checksum sama → tetap 1; vendor tak dikenal → `vendor_id` NULL; insert receipt gagal → `dsr_uploads` ikut rollback.
- Bukti: `cd backend_python && python -m unittest discover -s dsr` + integration.
- Risiko R1 (commit DSR gagal bila receipt gagal) → dicakup tes rollback.

### P3 — SQL laporan + repo read-only Go (FR2, FR3, D2, D3) · **opus, effort high** · butuh P1 · paralel dengan P2
- `backend/queries/dsr_late_report.sql` (sqlc):
  - `ListDsrLateDaily` (filter vendor/cabang/status, paging);
  - `CountDsrLateDaily`;
  - `ListDsrLateSummary` (agregat per cabang);
  - `ListDsrLateUnmappedVaults` (S2: vault aktif tanpa map + `branch_excluded`);
  - `ListDsrLateUnmappedLabels`;
  - `ListDsrLateUnresolvedVendors`.
  - Semua memakai CTE yang sama: `days` (`generate_series` bulan ∩ [start, hari ini WIB]) → `counted_vaults(X)` (S5: `is_active`, `created_at` WIB ≤ X, ada map aktif) → `obliged` (FLM, S3 aktif-pada-X) → `hits` (`unnest(locations)` ⨝ map ⨝ counted vault) → `first_received_at` → status (K1, K4).
- `sqlc generate` (v1.31.1) + fix `UserLeafe` → `UserLeave`; `git diff --stat backend/internal/db/` hanya file yang diharapkan.
- `internal/repository/dsr_late_report_repository.go`: read-only, pakai `dbRead` (replica).
- Bukti — integration `dsr_late_report_repository_integration_test.go` (`//go:build integration`, skip tanpa `DATABASE_URL`), fixture tabel kasus dari `testdata/dsr_late_cases.json` (dipakai juga di P6):
  - OK tepat 09:00:00, TELAT 09:00:01, TIDAK KIRIM, BELUM JATUH TEMPO hari ini;
  - revisi tidak menggeser waktu;
  - label tak ter-map;
  - cabang yang semua vault-nya belum ter-map dikeluarkan;
  - vault dibuat setelah X tidak dihitung;
  - cabang nonaktif dengan `deleted_at` > X tetap wajib;
  - ROH/INTERNAL dikecualikan;
  - start date.
  Ditambah test topologi replica (pola `admin_repository_topology_test.go`) dan `EXPLAIN` untuk satu bulan pada data dev (catat di `tests.md`).
- Risiko: performa `generate_series × cabang` (NFR ≤ 3 s p95) → indeks yang ada + `(report_date, vendor_id)` dari 033; ukur di dev.

### P4 — Service + handler + CSV + audit + RBAC (FR4, FR7) · **sonnet, effort medium** · butuh P3
- `internal/service/dsr_late_report.go`:
  - `DsrLateReportService` dengan `authorize` (`ATM-USER`, `ATM-SPV`, `BRANCH-ATM-SPV`, `ADMIN`; tanpa scope, S4);
  - validasi `month` (format, bukan setelah bulan berjalan → 400), `status`, paging;
  - `RecordExport` → `audit_logs` `report_exported`, entity `dsr_late_report`, `entity_id 0`, `after` = report + filter (pola `ReplenishmentReadService`);
  - `startDate` dari `DSR_LATE_START_DATE` (kosong = laporan selalu kosong + `start_date: null`).
- `internal/handler/dsr_late_report_handler.go`:
  - `GET /daily`, `/summary`, `/warnings`;
  - `?format=csv` → audit dulu, lalu `encoding/csv` (+ `utf8BOM` yang sudah ada);
  - flat JSON;
  - status berupa kode `ok|late|not_sent|pending`.
- `cmd/api/main.go`: `getenvDefault("DSR_LATE_START_DATE", "")` → parse (salah format = gagal start); mount `/api/v1/dsr-late-reports` dengan `RequireAuth` + `RequireRoles(...)`.
- Bukti:
  - handler test (403 per role lain, 400 `month`, CSV header + baris, audit gagal → 500 tanpa CSV);
  - unit service (RBAC di service);
  - integration ekspor menulis 1 `audit_logs`;
  - `cd backend && go test ./... && golangci-lint run --build-tags integration ./...`.

### P5 — OpenAPI + kontrak + tipe FE (DoD Sec 11) · **sonnet, effort low** · butuh P4
- `backend/api/openapi.yaml`: 3 path `/dsr-late-reports/{daily,summary,warnings}`, parameter (`month`, `vendor_id`, `vendor_branch_id`, `status`, `page`, `page_size`, `format`), skema strict, respons 200 JSON, 200 `text/csv`, 400, 401, 403, 500.
- `openapi_contract_test.go`: kasus respons untuk tiap operasi baru (test gagal bila operasi tanpa kasus).
- `pnpm --dir frontend/CompanyPortal-Vite run gen:api` → `schema.d.ts` di-commit bersama.
- Bukti: `cd backend && go test ./internal/handler -run OpenAPI`; vitest freshness hijau.

### P6 — Job notifikasi 09:00 Python (FR5, K3, K5) · **opus, effort high** · butuh P1 (+ SQL P3) · paralel dengan P4/P5/P7
- `backend_python/dsr_late_notify/` (pola `replenish_classify`):
  - `queries.py`: SQL `obliged`/`hits` salinan P3 (D2) untuk X = hari ini, plus penerima vendor (salinan aturan `notifications.sql` 029/032) dan `ATM-USER` aktif;
  - `message.py` (murni): menyusun title ≤120/body ≤1000, "dan N lainnya", satu notifikasi per penerima berisi daftar cabangnya;
  - `job.py`: `run_once(pool, today, start_date, trigger, email_enabled)`:
    - satu tx;
    - `pg_advisory_xact_lock`;
    - cek `retry_audit_logs` sukses untuk `(dsr_late_notify, X)` → keluar;
    - insert `notifications` (`type='dsr_late'`) + `notification_emails` (`pending`/`skipped`);
    - insert `retry_audit_logs` `success`;
    - gagal → rollback + baris `failed` di tx terpisah;
  - `run.py`: CLI `--date`, `--start-date`.
- `eod_retry_scheduler/config.py`: `dsr_late_start_date: date | None = None` (`RETRY_DSR_LATE_START_DATE`). `lib/services/scheduler_service.py`: daftarkan `CronTrigger(hour=9, minute=0, timezone=WIB)` id `dsr_late_notify` hanya bila `getattr(settings, "dsr_late_start_date", None)` terisi (jadi `service_dsr_etl` tidak ikut).
- Tes:
  - `test_message.py` (murni);
  - `test_job_integration.py` (skip tanpa `DATABASE_URL`, rollback): kirim sekali; run ke-2 → 0; cabang yang sudah kirim tidak dapat; cabang belum ter-map tidak dapat; PIC tanpa `is_notification_recipient`/peran salah tidak dapat; user vendor-wide dapat satu notifikasi gabungan; `ATM-USER` dapat ringkasan; email `skipped` saat SMTP mati; start date kosong → tidak jalan; fixture `testdata/dsr_late_cases.json` dari P3 memberi daftar "belum kirim" yang sama dengan status Go.
  - Satu tes membaca baris notifikasi Python dengan query Go `notifications` (R3).
- Bukti: `cd backend_python && python -m unittest discover -s dsr_late_notify` + `lib/services/test_scheduler_service.py` (job terdaftar / tidak).

### P7 — Layar CompanyPortal (FR6) · **sonnet, effort medium** · butuh P5 · invoke `ui-ux-pro-max` dulu (aturan Sec 13 menang)
- `src/lib/config/navigation.ts`: item `dsr-late-report` di grup `monitoring`, roles 4, route `/dsr-late-report`; daftar route di router.
- `src/features/dsr-late-report/`:
  - `api.ts` (React Query, tipe dari `schema.d.ts`, unduh CSV);
  - `DsrLateReportScreen.tsx` (filter bulan/vendor/cabang/status; tab Rekap + Detail; paging server; tombol CSV per tab);
  - `StatusBadge.tsx` (label + ikon untuk OK/TELAT/TIDAK KIRIM/BELUM JATUH TEMPO, aria-label deadline);
  - `WarningsPanel.tsx` (vault/cabang belum ter-map + "cabang dikeluarkan", label tak ter-map, vendor tak ter-resolve; link master data untuk ADMIN);
  - keadaan loading/error/kosong sebelum start date;
  - angka `tabular-nums` rata kanan.
- Bukti: vitest (render per tab, filter → query param, status berupa teks bukan hanya warna, menu tampil untuk 4 role dan tidak untuk role lain, error/kosong, CSV dipanggil) + `pnpm --dir frontend/CompanyPortal-Vite run build|test|lint`. Manual browser check: **outstanding (user)**.

### P8 — Data uji + docs · **haiku, effort low** · butuh P1–P7 (docs bisa mulai paralel setelah P4/P6)
- `backend/scripts/dev/dsr_late_report_seed.sql`: map lokasi DSR untuk satu vendor dev (mis. Prosegur/Bijak) + 1 vault per cabang. Langkah uji manual (upload sebelum/sesudah 09:00, revisi, label salah) ditulis di `tests.md`.
- `.env.example`: `backend/` (`DSR_LATE_START_DATE=`) + `backend_python/eod_retry_scheduler` (`RETRY_DSR_LATE_START_DATE=`), dengan catatan "nilai harus sama" (K5).
- CLAUDE.md Sec 3 (tabel `dsr_receipts`, migrasi 033) + Sec 12 (keputusan 2.3: S1–S5, K3 penulis notifikasi Python kedua) + daftar migrasi s.d. 033; `docs/data-map.md` (bagian 033); `docs/decisions.md` (2.3); `graphify update .` dari root.
- Bukti: diff docs ditinjau; tidak ada `.env` yang ter-commit.

### P9 — Checkpoint akhir · **sonnet, effort medium** · setelah semua
- Go ×3 modul `go test ./...` + `go test -tags integration -p 1 ./...` (0 skip dengan `DATABASE_URL`), coverage `internal/*` ≥ ambang CI, `golangci-lint`, `govulncheck`.
- Python `unittest` semua paket (`dsr`, `itm/cashpos`, `replenish_classify`, `dsr_late_notify`, `lib`).
- CompanyPortal `build|test|lint` + coverage; `gen:api` bersih (tanpa diff).
- `migrate status` dev = 0 pending (setelah OK user di P1).
- Docker build `backend` + `backend_python` image.
- Tulis `tests.md` (trace tes → FR) dan siapkan `review.md`. Manual browser check tetap **outstanding (user)**.

## Urutan & paralel
```
P1 ──► P2
   ├─► P3 ──► P4 ──► P5 ──► P7 ──┐
   └─► P6 (setelah SQL P3 stabil) ┴─► P8 ──► P9
```
- Paralel aman: **P2 ∥ P3** (Python ingest vs Go queries, file berbeda); **P6 ∥ P4/P5/P7** (Python vs Go/FE). P6 mulai setelah SQL P3 di-merge (D2).
- Jangan paralel: P4 dan P5 (keduanya menyentuh `handler/` + kontrak); P7 butuh `schema.d.ts` dari P5. `cmd/api/main.go` hanya disentuh P4; `scheduler_service.py` hanya P6.
- Bila dijalankan sebagai sesi terpisah di folder yang sama: tiap sesi hanya menyentuh file slice-nya, tanpa commit dan tanpa ganti branch.

## Risks
- **R1 Commit DSR ikut gagal bila receipt gagal** (satu tx, FR1.1): disengaja supaya receipt tidak pernah hilang; dicakup tes rollback P2.
- **R2 Logika "sudah kirim" di Go dan Python**: D2 (SQL sama + fixture bersama); review P6 membandingkan SQL baris per baris.
- **R3 Kontrak notifikasi Python↔Go**: penulis K3 kedua; tes P6 membaca baris dengan query Go.
- **R4 Zona waktu**: D3; tes batas 09:00:00 vs 09:00:01 dan hari pergantian UTC (00:00–07:00 WIB).
- **R5 Performa** bulan penuh × cabang: ukur `EXPLAIN ANALYZE` di P3; bila > 3 s, tambah indeks di migrasi berikutnya (bukan mengubah 033 yang sudah diterapkan).
- **R6 Migrasi dev**: `migrate up` hanya dengan OK user; 033 tidak boleh diedit setelah diterapkan (checksum MISMATCH).
- **R7 sqlc**: v1.31.1 + fix `UserLeafe` setiap generate; cek `git diff --stat backend/internal/db/`.
- **R8 Go-live tanpa map lokasi** (spec R6): laporan kosong sampai ADMIN mengisi `dsr_location_vault_maps`; ditulis di catatan rilis P8.

## Alternatif yang tidak diambil
- VIEW/fungsi SQL bersama Go+Python: menambah objek ke migrasi 033 dan menyulitkan perubahan; D2 memilih salinan SQL + fixture bersama.
- Job notifikasi di Go (cron goroutine): spec #14 memilih Python `eod_retry_scheduler`.
- Menyimpan status harian (tabel hasil): spec #7 memilih laporan hidup; status dihitung saat dibaca.
- Mengikat receipt hanya ke `--mode commit`: lihat D1.
