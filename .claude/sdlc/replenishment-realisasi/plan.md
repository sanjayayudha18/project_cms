# Plan: Replenishment — realisasi pengisian ATM vs order (Phase 2.2c)

Author: Claude, dari `spec.md` (accepted 2026-10-14). Status: **accepted 2026-10-14** (engineer: *"accept"*).
Stage: 3 Build. Stage berikut: `tests.md`.

## Titik sentuh yang sudah dicek di kode (2026-10-14)
- Laporan selesai: `backend/internal/service/vendor_request_completion.go` (`SubmitCompletion` → `tickets.result` + `completion_pending`; `ApproveCompletion` potong kuota; `RejectCompletion` → `vendor_accepted` untuk alur vendor). Handler `completionBody` di `vendor_request_handler.go:368`.
- VendorPortal Orders 2.2b: `service/vendor_order.go` — endpoint per **party id** (`/api/v1/vendor/replenish-orders/{partyID}`), scope `vendorScope` (vendor JWT + branch dari DB), `decide` = pola tx + audit + notifier. Realisasi ikut pola ini (path spec `{id}` = party id pihak `replenish`).
- Admin langsung-berlaku + audit dalam tx (deviasi GR#3): `service/region_admin.go` + `handler/admin_region_handler.go` + `queries/regions_admin.sql` → pola untuk `holidays`.
- Export CSV stdlib: `service/masterdata_export.go` + `handler/admin_master_data_export_handler.go`.
- Python EOD: `backend_python/lib/services/scheduler_service.py` — job APScheduler (scan, auto-retry, late check); `_finish_retry` / `run_manual_process` menandai `import_jobs` `completed` → titik kait klasifikasi. ITM CSV kolom `ATMID`, `TGLREPLENISH` (1YYMMDD), `REPLENISHDENOM50K/100K` (10K/20K tidak dibaca ETL → selalu 0).
- Frontend: CompanyPortal `features/replenishment/ReplenishmentScreen.tsx` + `features/dashboard/ReplenishmentSummary.tsx` (mock), `features/vendor-request/CompletionPanels.tsx` (form ATM-USER), `features/admin-regions` (pola admin); VendorPortal `features/orders/OrderDetailPage.tsx`.
- Data dev: lihat `spec.md` → "Data dev" (ITM hanya 2026-08-18..21, 0 overlap dengan tiket).

## Slices (satu concern per slice, tes hijau sebelum lanjut)

### S1 — Migrasi 031 + query + sqlc — ✅ DONE 2026-10-14 (mig 031 applied to dev)
> Catatan implementasi: nominal `bigint` full IDR (pola 030, Sec 6 "integer minor units"), bukan `numeric(20,0)`;
> `itm_replenish_id` FK `ON DELETE SET NULL` (ETL menghapus baris file yang diproses ulang, R2) + snapshot
> `realized_date/time`; `report_date` generated = COALESCE(jadwal, realisasi) untuk filter layar; trigger
> `updated_at` klasifikasi tanpa `WHEN` (Postgres menolak whole-row ref + kolom generated). Query klasifikasi
> (baca) ditulis di S6 bersama handler; Python menulis tabel klasifikasi langsung (S5). Guard
> `no_hard_delete_test.go` + `holidays`, `vendor_request_realizations`, `replenish_classifications`.
- `backend/migrations/031_replenish_realization.sql`: `holidays`, `vendor_request_realizations`, `vendor_request_realization_amounts`, `replenish_classifications`, `replenish_classification_amounts` (spec Data model) + index filter layar (`scheduled_date`, `vendor_id`, `classification`). Additive saja.
- `backend/queries/holidays_admin.sql`, `vendor_realizations.sql`, `replenish_classifications.sql`; `sqlc generate` (v1.31.1) + fix `UserLeafe`.
- Resubmit realisasi = `UPDATE` + upsert amounts per denom (set denom = denom order, tetap) → `no_hard_delete_test.go` tetap hijau.
- Terapkan ke dev **setelah izin user** (`psql -f`).
- Bukti: migrasi bersih di dev; `go build ./...`.

### S2 — Logika murni Go (unit test, tanpa DB) — ✅ DONE 2026-10-14 (`validateRealization`, 16 kasus; reuse `validateCompletionResults`)
- `internal/service/replenish_realization_rules.go`: validasi laporan (FR2.2, K1: tanggal ≥ jadwal − 7 hari, ≤ hari ini WIB; nominal bulat ≥ 0, kelipatan denom, hanya denom order, minimal satu > 0 bila success; semua ATM tepat sekali).
- Bukti: table-driven test, termasuk batas K1 dan denom asing.

### S3 — Hari Libur admin (FR1) — ✅ DONE 2026-10-14
> Catatan implementasi: `service/holiday_admin.go` (pool + `db.Queries` langsung, pola `VendorOrderService`; ADMIN di
> route + service; `inTx` = perubahan + `audit_logs` satu tx; unik tanggal aktif dicek + `holidays_active_date_uq`
> backstop; enable ditolak bila tanggal sudah dipakai baris aktif lain; import CSV `holiday_date,name` semua-atau-tidak,
> maks 1000 baris / 1 MB, tanggal yang sudah aktif dilewati + dilaporkan, satu audit `holiday_imported`; pakai ulang
> `utf8BOM`). `handler/admin_holiday_handler.go` → `/api/v1/admin/holidays` (`GET`, `POST`, `PUT /{id}`,
> `POST /{id}/disable|enable`, `POST /import` multipart) — `{id}` bukan `{date}` (satu tanggal bisa punya baris nonaktif
> lama). CompanyPortal `features/admin-holidays` + route `/settings/admin/holidays` (ADMIN), kartu Settings Hub, menu
> "Kalender Libur" (placeholder lama diaktifkan, roles ADMIN). Tes: unit parser, handler, integration (unik, enable
> bentrok, audit = 5, import skip, file salah tidak menulis apa pun, RBAC), vitest 5. Full Go + golangci-lint 0;
> CompanyPortal 1054 hijau.
- `service/holiday_admin.go` + `handler/admin_holiday_handler.go` (pola region): list/create/update name/disable/enable/import CSV; audit dalam tx; `ADMIN` di route + service. Mount `/api/v1/admin/holidays` di `cmd/api/main.go`.
- Bukti: unit (validasi, RBAC service) + integration (audit rollback, unik aktif, import semua-atau-tidak).

### S4 — Laporan realisasi vendor (FR2) — ✅ DONE 2026-10-14
> Catatan implementasi: request lama tanpa party vendor (`vendor_sent = false`) tetap dilapor ATM-USER (keputusan
> user 2026-10-14, spec FR2.2/2.5 disesuaikan); `POST /vendor-requests/{id}/complete` → 409 `completion_by_vendor`
> hanya untuk request `vendor_sent`, dan VENDOR-USER selalu ditolak di jalur internal. `runCompletionTx` = state
> machine laporan selesai yang dipakai bersama. Request dengan > 1 branch replenish hanya bisa dilapor user
> vendor-wide (`checkReportsWholeRequest`, ponytail). Endpoint: `POST /api/v1/vendor/replenish-orders/{partyID}/realization`.
> Detail VendorPortal + internal (`atms[].realized_date/realized_amounts`) menampilkan laporan. Cleanup fixture tes
> (`cleanupVendorParties`) + hapus `vendor_request_realizations`; tes realisasi hapus `atm_visits` sendiri.
> Tes: handler (`TestVendorOrderHandler_Realization`), integration (`TestIntegration_VendorOrder_Realization`,
> `CompletionGateAndCancel` diubah). Full `go test -tags integration ./...`: hijau kecuali
> `TestIntegration_ApplyVaultTickets` (pre-existing: memakai `max(id)` vendor_requests = request dev asli yang sudah
> punya tiket vault; bukan regresi S4, tx di-rollback).
- File baru `service/vendor_order_realization.go` (`vendor_order.go` sudah 457 baris): `SubmitRealization(va, partyID, rows)` — party harus `replenish` + scope; status request `vendor_accepted|approved|ready`; tulis realisasi + amounts + `tickets.result`; request → `completion_pending` (`completion_submitted_by` = user vendor); audit + notifikasi ATM-SPV/BRANCH-ATM-SPV dalam tx.
- `SubmitCompletion` internal → `ErrCompletionByVendor` (409 `completion_by_vendor`).
- Detail order (VendorPortal) + detail request (internal): sertakan laporan realisasi per ATM.
- Approve/reject tidak berubah; tambah notifikasi ke vendor pelapor (FR2.8).
- Bukti: integration — scope 404, pihak vault ditolak, status salah 409, resubmit menimpa, kuota terpotong sekali, audit + notifikasi rollback bersama tx.

### S5 — Job klasifikasi Python (FR3) — ✅ DONE 2026-10-14
> Catatan implementasi: `replenish_classify/rules.py` (murni: hari kerja, jendela, offset, matching, nominal),
> `job.py` (asyncpg, satu tx + `pg_try_advisory_xact_lock`), `run.py` (CLI). Env pakai prefix scheduler:
> `RETRY_REPLENISH_CLASSIFY_START_DATE` (kosong = job tidak jalan) + `RETRY_NOTIFY_EMAIL_ENABLED`
> (email outbox `pending` hanya bila Go punya SMTP, selain itu `skipped` — sama dengan `notification.Send`).
> Hook: `SchedulerService._finish_retry` / `run_manual_process` setelah `itm_replenish` sukses + cron 09:15 WIB
> (hanya di service yang memegang `itm_replenish`). Aksi → `retry_audit_logs` (`classification_run`).
> Keputusan kecil: notifikasi telat = **satu ringkasan per ATM-SPV/BRANCH-ATM-SPV per run** (bukan per tiket,
> hindari spam), link `/replenishment`; "laporan telat" = jendela tertutup dan `tickets.result` masih kosong (berlaku
> juga untuk request lama yang dilapor ATM-USER); klasifikasi `pending` menyimpan hasil sementara di
> `realized_*`/`workday_offset` (CHECK: pending ⇔ belum final). Ponytail: `LOOKBACK_DAYS = 31` (ITM lebih tua tidak
> dilihat lagi). Review 2026-10-14: R2 baris pending melepas `itm_replenish_id` sebelum matching (void juga), R3 request
> ditolak → void setelah jendela tertutup, R4 laporan vendor setelah final memperbarui kolom vendor saja.
> Tes: `test_rules.py` (9), `test_job_integration.py` (pending → final, void, no_order, hari libur, selisih, beda
> laporan vendor, notifikasi sekali, idempoten; tx di-rollback). Semua tes Python hijau (75).
- `backend_python/replenish_classify/`: `workdays.py` (hari kerja + offset dari `holidays`), `matching.py` (FR3.3 terdekat + tie-break, satu ITM satu tiket), `classify.py` (kelas, final, nominal, flags, `void`, `no_order`), `notify.py` (K3: insert `notifications` + `notification_emails` ATM-SPV/BRANCH-ATM-SPV aktif, satu tx dengan `late_notified_at`), `run.py` (CLI `--date`, env `REPLENISH_CLASSIFY_START_DATE`, advisory lock).
- Kait scheduler: setelah job `itm_replenish` `completed` (`_finish_retry`, `run_manual_process`) + cron harian 09:15 WIB (menutup jendela meski tidak ada file baru). Aksi → `retry_audit_logs` + log.
- Bukti: `unittest` stdlib untuk `workdays`/`matching`/`classify` murni (fixture "terisi > 1 kali sehari"); integration (skip tanpa `DATABASE_URL`): idempoten (2× = sama), final tidak berubah, notifikasi sekali.

### S6 — API baca internal + VendorPortal + report CSV (FR4, FR5.2, FR6) — ✅ DONE 2026-10-14
> Catatan implementasi: `queries/replenish_classifications.sql` (list/count + amounts, summary per kelas, instruksi
> harian FNC 003 dari tiket instruksi, trips), `service/replenishment_read.go` (`ReplenishmentReadService`: read di
> replica, audit export di primary; `VendorOrderService.ListRealizations` scope vendor/branch),
> `handler/replenishment_handler.go` (`/api/v1/replenishment/{classifications,summary,reports/amount-mismatch,reports/trips}`,
> `?format=csv` → audit `report_exported` lalu CSV stdlib; `/api/v1/vendor/replenish-realizations`). Role = viewer Vendor
> Request (`ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV`) — list Vendor Request juga tidak menyaring data
> per cabang untuk BRANCH-*, jadi sama. Audit export: `entity_type = replenishment_report`, `entity_id = 0` (kolom
> NOT NULL, tidak ada entitas tunggal), report + filter di `after`. CSV dibangun penuh di memori (maks 31 hari).
> Tes: handler (`TestReplenishmentHandler`, `TestVendorOrderHandler_ListRealizations`), integration
> (`TestIntegration_ReplenishmentRead`). Full `go test -tags integration -p 1 ./...` hijau, golangci-lint 0.
- `service/replenishment_read.go` + `handler/replenishment_handler.go`: `classifications` (filter, paging, maks 31 hari), `summary`, `reports/amount-mismatch`, `reports/trips` (`?format=csv`, audit `report_exported`). Replica. Role + scope BRANCH-* seperti Vendor Request list.
- `GET /api/v1/vendor/replenish-realizations` (scope vendor, replica).
- Bukti: handler test (RBAC, > 31 hari → 400, CSV header + angka bulat), test routing replica (pola `admin_repository_topology_test.go`).

### S7 — CompanyPortal (FR4, FR1, FR2.5) — ✅ DONE 2026-10-14 (kecuali UI Hari Libur → ikut S3)
> Catatan implementasi: `features/replenishment/api.ts` (hook React Query + unduh CSV) + `ReplenishmentScreen.tsx`
> ditulis ulang (filter tanggal maks 31 hari, vendor, region, klasifikasi, hanya-selisih; paging server; badge label +
> offset hari kerja; penanda beda laporan vendor / laporan telat; 2 tombol CSV). Mock dihapus: `replenishment.utils.ts`,
> `types.ts`, `data/replenishment-schedules.json`, `dashboard/replenishment.utils.ts` + 2 tes properti mock.
> `dashboard/ReplenishmentSummary.tsx` → `/replenishment/summary` (ATM + nominal instruksi hari ini, klasifikasi 7 hari).
> Detail Vendor Request: tombol "Laporkan Selesai" hanya untuk request lama (`approved`/`ready`); `vendor_accepted`
> menampilkan keterangan "menunggu laporan realisasi dari vendor"; tabel per ATM + kolom tanggal & nominal laporan
> vendor. Tes: vitest 1049 hijau (Replenishment 6, Dashboard + summary, VendorRequestDetail disesuaikan), biome bersih,
> `pnpm run build` OK; `tsc -b` 117 error lama di test file lain (tidak bertambah). Manual browser check: outstanding (user).
- `features/replenishment` ganti mock (tabel, filter, paging, export); `ReplenishmentSummary.tsx` ke API; `features/admin-holidays` baru (pola `admin-regions`, + import CSV); `CompletionPanels.tsx`: form ATM-USER diganti tampilan laporan vendor, approve/reject tetap.
- Bukti: vitest per layar (render, filter, error, menu per role), `pnpm run build|test|lint`.

### S8 — VendorPortal (FR5) — ✅ DONE 2026-10-14
> Catatan implementasi: `features/orders/RealizationPanel.tsx` di detail order pihak replenish yang `accepted`:
> form per ATM (berhasil/gagal, tanggal, nominal per pecahan diisi awal dari order / laporan sebelumnya), validasi klien
> sama dengan S2 (K1 tanggal, kelipatan pecahan, minimal satu nominal), alasan tolak CIMB tampil, laporan terkirim tampil
> read-only saat tidak bisa diedit. `api.ts`/`useOrders.ts`: `submitRealization`, `fetchRealizations`, hook. Halaman
> `features/realizations/RealizationsPage.tsx` + route `/realizations` + menu "Realisasi" (klasifikasi milik vendor,
> filter kelas + tanggal maks 31 hari, paging). Tes: `Realization.test.tsx` (8); VendorPortal vitest 136 hijau, biome
> bersih, `pnpm run build` (tsc -b) OK. Manual browser check: outstanding (user).
- `features/orders/OrderDetailPage.tsx`: form **Lapor Realisasi** (pihak replenish, status diizinkan, alasan reject tampil); halaman `features/realizations` baru + menu.
- Bukti: vitest (kelipatan denom, submit, 409/404), build/test/lint.

### S9 — Data uji + docs — ✅ DONE 2026-10-14
> Selesai: `backend/scripts/dev/itm_replenish_test_sample.csv` (3 baris untuk `REP-ABA-MDN-20261014-001`, jadwal Rabu
> 2026-10-14: 4608 50K 400 jt tgl 14 → `on_schedule` tanpa selisih; 4686 50K 245 jt tgl 15 → `late` +1; 6629 100K
> 600 jt tgl 14 vs order 610 jt → `on_schedule` + `has_amount_diff` −10 jt). Langkah jalankan → `tests.md`. Docs:
> CLAUDE.md Sec 3 (031) + Sec 12 (2.2c) + daftar migrasi s.d. 031; `docs/data-map.md` (bagian 031, `holidays` proposed →
> migrated); `docs/decisions.md` (2.2c); spec API path `/complete` dibetulkan.
- `backend/scripts/dev/itm_replenish_test_sample.csv` (format ETL ITM) untuk tiket uji (mis. `REP-ABA-MDN-20261014-001`: 3 ATM, jadwal 2026-10-14) — satu tepat, satu mundur 1 hari kerja, satu nominal beda — + langkah jalankan di `tests.md`.
- CLAUDE.md Sec 3 (tabel 031) + Sec 12 (keputusan 2.2c, deviasi GR#3 holidays, K3) + `docs/data-map.md` + `docs/decisions.md`; `graphify update .` dari root.

## Urutan
S1 → S2 → (S3 ∥ S4) → S5 → S6 → (S7 ∥ S8) → S9. S5 butuh S1 (tabel) + S4 (laporan vendor untuk `vendor_mismatch`).

## Risks
- **R1 Kontrak notifikasi Python↔Go (K3)**: kolom disalin di Python. Mitigasi: integration test — baris dari Python terbaca oleh query Go `notifications`; komentar silang di kedua sisi.
- **R2 `itm_replenish.id`**: dicek 2026-10-14 — ada `itm_replenish_pkey` → FK `replenish_classifications.itm_replenish_id → itm_replenish(id)` aman. Catatan: ETL memindahkan file versi lama (`superseded`) — bila baris ITM dihapus/diganti saat re-upload, FK `ON DELETE RESTRICT` bisa menggagalkan ETL; cek perilaku ETL di S1, pilih `ON DELETE SET NULL` + klasifikasi kembali `pending` bila perlu.
- **R3 ETL ITM hanya baca 50K/100K**: order denom 10K/20K selalu "selisih". Diterima (FR3.6); dicatat di `tests.md`.
- **R4 Job vs ETL bersamaan**: advisory lock + hanya baca file `completed`.
- **R5 Maker laporan selesai pindah ke vendor**: request terbuka langsung ikut alur baru; `vendor_request_state_machine_property_test.go` diperbarui.

## Alternatif yang tidak diambil
- Klasifikasi di Go (goroutine) — PO pilih Python EOD (Sec 14).
- jsonb untuk nominal — Sec 6 minta `numeric` per denom.
- Hapus klasifikasi saat tiket nonaktif — `void` (K2, no hard delete).
- Endpoint laporan di luar Orders — reuse scope + pola tx 2.2b.
