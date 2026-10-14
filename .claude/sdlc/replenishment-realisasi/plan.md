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

### S1 — Migrasi 031 + query + sqlc
- `backend/migrations/031_replenish_realization.sql`: `holidays`, `vendor_request_realizations`, `vendor_request_realization_amounts`, `replenish_classifications`, `replenish_classification_amounts` (spec Data model) + index filter layar (`scheduled_date`, `vendor_id`, `classification`). Additive saja.
- `backend/queries/holidays_admin.sql`, `vendor_realizations.sql`, `replenish_classifications.sql`; `sqlc generate` (v1.31.1) + fix `UserLeafe`.
- Resubmit realisasi = `UPDATE` + upsert amounts per denom (set denom = denom order, tetap) → `no_hard_delete_test.go` tetap hijau.
- Terapkan ke dev **setelah izin user** (`psql -f`).
- Bukti: migrasi bersih di dev; `go build ./...`.

### S2 — Logika murni Go (unit test, tanpa DB)
- `internal/service/replenish_realization_rules.go`: validasi laporan (FR2.2, K1: tanggal ≥ jadwal − 7 hari, ≤ hari ini WIB; nominal bulat ≥ 0, kelipatan denom, hanya denom order, minimal satu > 0 bila success; semua ATM tepat sekali).
- Bukti: table-driven test, termasuk batas K1 dan denom asing.

### S3 — Hari Libur admin (FR1)
- `service/holiday_admin.go` + `handler/admin_holiday_handler.go` (pola region): list/create/update name/disable/enable/import CSV; audit dalam tx; `ADMIN` di route + service. Mount `/api/v1/admin/holidays` di `cmd/api/main.go`.
- Bukti: unit (validasi, RBAC service) + integration (audit rollback, unik aktif, import semua-atau-tidak).

### S4 — Laporan realisasi vendor (FR2)
- File baru `service/vendor_order_realization.go` (`vendor_order.go` sudah 457 baris): `SubmitRealization(va, partyID, rows)` — party harus `replenish` + scope; status request `vendor_accepted|approved|ready`; tulis realisasi + amounts + `tickets.result`; request → `completion_pending` (`completion_submitted_by` = user vendor); audit + notifikasi ATM-SPV/BRANCH-ATM-SPV dalam tx.
- `SubmitCompletion` internal → `ErrCompletionByVendor` (409 `completion_by_vendor`).
- Detail order (VendorPortal) + detail request (internal): sertakan laporan realisasi per ATM.
- Approve/reject tidak berubah; tambah notifikasi ke vendor pelapor (FR2.8).
- Bukti: integration — scope 404, pihak vault ditolak, status salah 409, resubmit menimpa, kuota terpotong sekali, audit + notifikasi rollback bersama tx.

### S5 — Job klasifikasi Python (FR3)
- `backend_python/replenish_classify/`: `workdays.py` (hari kerja + offset dari `holidays`), `matching.py` (FR3.3 terdekat + tie-break, satu ITM satu tiket), `classify.py` (kelas, final, nominal, flags, `void`, `no_order`), `notify.py` (K3: insert `notifications` + `notification_emails` ATM-SPV/BRANCH-ATM-SPV aktif, satu tx dengan `late_notified_at`), `run.py` (CLI `--date`, env `REPLENISH_CLASSIFY_START_DATE`, advisory lock).
- Kait scheduler: setelah job `itm_replenish` `completed` (`_finish_retry`, `run_manual_process`) + cron harian 09:15 WIB (menutup jendela meski tidak ada file baru). Aksi → `retry_audit_logs` + log.
- Bukti: `unittest` stdlib untuk `workdays`/`matching`/`classify` murni (fixture "terisi > 1 kali sehari"); integration (skip tanpa `DATABASE_URL`): idempoten (2× = sama), final tidak berubah, notifikasi sekali.

### S6 — API baca internal + VendorPortal + report CSV (FR4, FR5.2, FR6)
- `service/replenishment_read.go` + `handler/replenishment_handler.go`: `classifications` (filter, paging, maks 31 hari), `summary`, `reports/amount-mismatch`, `reports/trips` (`?format=csv`, audit `report_exported`). Replica. Role + scope BRANCH-* seperti Vendor Request list.
- `GET /api/v1/vendor/replenish-realizations` (scope vendor, replica).
- Bukti: handler test (RBAC, > 31 hari → 400, CSV header + angka bulat), test routing replica (pola `admin_repository_topology_test.go`).

### S7 — CompanyPortal (FR4, FR1, FR2.5)
- `features/replenishment` ganti mock (tabel, filter, paging, export); `ReplenishmentSummary.tsx` ke API; `features/admin-holidays` baru (pola `admin-regions`, + import CSV); `CompletionPanels.tsx`: form ATM-USER diganti tampilan laporan vendor, approve/reject tetap.
- Bukti: vitest per layar (render, filter, error, menu per role), `pnpm run build|test|lint`.

### S8 — VendorPortal (FR5)
- `features/orders/OrderDetailPage.tsx`: form **Lapor Realisasi** (pihak replenish, status diizinkan, alasan reject tampil); halaman `features/realizations` baru + menu.
- Bukti: vitest (kelipatan denom, submit, 409/404), build/test/lint.

### S9 — Data uji + docs
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
