# Status Snapshot

> Dipindah dari `.kiro/steering/development-plan.md` (2026-11-12). Snapshot per 2026-10-30, sebagian bisa sudah basi — cek kode + `../development-progress.md`. Ringkasan: [`dev-plan.md`](./dev-plan.md).

Penyelesaian keseluruhan: **~58%** (perkiraan kasar; ~55% pada 2026-10-23, ~45-50% pada 2026-10-08). "Selesai" = tersambung ke route, didukung query nyata, teruji — bukan sekadar folder atau UI mock.

Branch: `dev1` **159 commit di depan** `main` (`dev1` terbaru `b102082`, 2026-11-03). Merge `dev1` → `main` masih pending.

Migrasi: baseline `001_baseline_schema.sql` + `002_baseline_seed.sql` (di-squash 2026-09-18; `001`–`040` lama ada di `backend/migrations/archives/2026-09-18_pre-baseline/`). Rentang saat ini `001`–`037` (celah: `024` dibuang; 36 file; nomor bebas berikutnya `038`), dilacak di `schema_migrations` oleh runner kustom `backend/cmd/migrate` (BE-03, `88a3139`). Dev di-baseline `--through 031` pada 2026-10-15; `032` applied 2026-10-20; `033_dsr_receipts` applied 2026-10-27; `034_export_jobs` applied 2026-10-28; `035_export_jobs_reports` applied 2026-10-29; `036_master_data_import_batches_file_name` applied ke dev 2026-11-02; `022_vendor_pks` + `037_export_jobs_vendor_pks` applied ke dev 2026-11-05.

## Sudah dibangun dan tersambung (diverifikasi di kode)

| Area | Endpoint / lokasi | Catatan |
| --- | --- | --- |
| Auth | `/api/v1/auth` | Local + JWT, siklus hidup password, sesi absolut 1 jam (`SESSION_MAX_LIFETIME`). Role vendor ditolak di CompanyPortal (login + refresh) |
| Approval | `/api/v1/approvals`, `/api/v1/admin/approval` | Orchestrator maker-checker, chain resolver, delegasi/cuti |
| Audit | `/api/v1/audit-logs` + `features/audit-log` | Writer append-only + viewer, indeks baca (mig `006`) |
| Role management | `/api/v1/admin/roles` | APPACCESS/ADMIN. Terapkan langsung + audit dalam tx (deviasi terdokumentasi, CLAUDE.md Sec 12) |
| Admin users | `/api/v1/admin/users` | **APPACCESS + ADMIN** (ADMIN_PARAM dihapus, vendor-party-scope D8). Role vendor + cakupan cabang + lookup `vendor-options` |
| Master data (maker-checker) | `/api/v1/admin/vendors` (+ `/branches`, `/branches/{id}/atms`, `/vaults`, `/pics`, `/packages`, `/package-prices`, `/dsr-location-maps`), `/api/v1/admin/atms` (+ `/assignments`), `/api/v1/admin/master-data/{changes,export,import}` | Semua write di-stage → 202 → terapkan saat disetujui lewat `Applier`. Impor/ekspor CSV (kolom `roles` PIC sejak mig `032`) |
| Regions | `/api/v1/admin/regions` + `features/admin-regions` | Terapkan langsung + audit. Mig `018` |
| Area ACM | `/api/v1/admin/acm-areas` | ADMIN, area per region (cit-vault-select v3) |
| Hari libur | `/api/v1/admin/holidays` | ADMIN, terapkan langsung + audit dalam tx (deviasi GR#3). Mig `031` |
| Portal ATM | `/api/v1/atm-portal` | Monitoring baca-saja (replica) |
| Forecast DMAA | `/api/v1/dmaa-forecast` | Viewer baca-saja. CROWN tidak menjalankan model forecasting; `amount_replenish` DMAA dipakai apa adanya **saat ini** — penyesuaian DSR (FSD §3 langkah 8) hadir bersama 2.4 (keputusan 2026-11-12) |
| Ringkasan Forecast Browser | `features/forecast` (tampilan ringkasan) | Ringkasan vendor × region (`7f6d16b`) |
| DSR (ATM) | `/api/v1/dsr` | Upload DSR vendor, dry-run/konfirmasi dua fase; Python ETL `service_dsr_etl` |
| Laporan DSR telat (2.3) | `/api/v1/dsr-late-reports` (`/daily`, `/summary`, `/warnings`; CSV pindah ke `backend-exq` X.8) + `features/dsr-late-report` + Python `dsr_late_notify/` | ATM-USER/ATM-SPV/BRANCH-ATM-SPV/ADMIN, baca dari replica, ekspor CSV beraudit, notifikasi telat 09:00 WIB. `dsr_receipts` append-only (mig `033`). `DSR_LATE_START_DATE` kosong = laporan mati (`67a2d72`) |
| Vendor request | `/api/v1/vendor-requests` | DMAA → order replenish, state machine maker-checker, review vault. Pilot kontrak OpenAPI (`backend/api/openapi.yaml`) |
| Rencana vault (ACM) | `/api/v1/vault-plans` | ACM-USER/ACM-SPV/ADMIN. Alokasikan nominal request ke vault + tiket pemenuhan vault (mig `027`, `028`, `030`) |
| Order vendor | `/api/v1/vendor/replenish-orders` | VendorPortal terima/tolak per (request × cabang), dicakup per peran pihak vendor (mig `029`, `032`) |
| Realisasi replenish | `/api/v1/vendor/replenish-realizations`, `/api/v1/replenishment` + Python `replenish_classify/` | Klasifikasi realisasi vs order dalam hari kerja, laporan vendor, CSV. Mig `031` |
| Notifikasi (Phase 0.3) | `internal/notification`, `/api/v1/notifications` + bell CP + API VP | In-app + worker outbox SMTP (stdlib). Mig `025` |
| Import jobs + idempotensi (Phase 0.2) | `import_jobs` (mig `019`/`020`) + helper Python + frontend | Idempoten per hash file; helper Go ditunda sampai ingest file Go pertama |
| EOD ETL (Python) | `backend_python/{dmaa,itm,eod_retry_scheduler,replenish_classify}` + `features/eod-monitoring` | Ini **adalah** runtime EOD (keputusan 2026-09-25) |
| Kuota kunjungan ATM / sumber paket / tiket replenish | mig `021`, `023`, `026` | Fitur sudah di-merge; sebagian `region_code` cabang masih kosong |
| Hardening platform | kedua backend + kedua portal | Routing read replica (0.1), `RequestLogger` + `/ready`, batas body 1 MiB + timeout 30s, IP klien tepercaya, uang sebagai string desimal, header CSP, lazy routes, gate coverage FE (CP 84/80/80/85, VP 93/84/90/93), gate coverage Go, CI GitHub Actions |

## Belum dibangun
- **Operasional ATM:** 2.4 pengecualian ATM bermasalah. `internal/cashcount` (Phase 5).
- **Finance:** `internal/invoice`, `internal/reconciliation`.
- **Platform:** `internal/document` (ditunda ke Phase 3/5), `internal/export` XLSX/PDF (saat ini hanya CSV; ditunda).
- **Integrasi:** `internal/corebanking` (ingest escrow).
- **`backend-exq/`** (di-rename dari `backend-cit` 2026-10-20, `8b41612`): ekspor async sudah dibangun (X.7–X.9), query monitoring dashboard belum. Belum di-deploy; deployment pertama bersamaan dengan rilis ekspor. Fungsi CIT ada di `backend/` (keputusan 2026-10-08).
- **Kalkulasi EOD Final Realisasi** + tabel `eod_runs` + alert penyelesaian terlambat.
- **Vendor PKS** (`.kiro/specs/vendor-pks/`): requirements disetujui PO 2026-11-05 (D1–D31), design + tasks 2026-11-05 (14 task, 42 leaf). Migrasi `022` (PKS + `vendor_pics.is_director`) + `037` (kind ekspor `vendor_pks` + constraint roles direktur) applied ke dev 2026-11-05. Tasks 1–13 selesai (dokumentasi 2026-11-06); task 14 cek browser manual oleh user masih outstanding. D32: direktur dicatat sebagai PIC, `vendors` tidak berubah. **Vendor CIS limit** (`vendor-cis-limit/`): split off 2026-11-05, intent draft, belum dibahas.
- **Tabel belum dimigrasi:** `documents`, `invoice_*`, `*_reconciliation_results`, semua `cit_*`, semua `escrow_*`, `cash_count_schedules`/`cash_count_evidences` (nama disetujui, belum dimigrasi). `replenishment_instructions` dibuang: Vendor Request yang disetujui adalah instruksinya.

## Frontend mock (data hardcode, tidak didukung API)
Layar mock dihapus dari menu/route dengan banner "Data contoh" di dashboard (FE-05, `d25ae2b`).
- **CompanyPortal:** `features/dashboard` (MetricStrip, AttentionPanel), `invoice`, `reconciliation`, `cit` (CitTracker), `cash-flow`, sebagian `dsr` (`useDsrData`).
- **VendorPortal:** jadwal, invoice, evidence.
Masing-masing hanya disambungkan oleh item fase pemiliknya.

## Known gap yang diterima (JANGAN diperbaiki kecuali diminta)
- `atm_vendor_packages.vendor_package_id` tidak divalidasi terhadap `price_machine_group`/`price_class` ATM (keputusan user 2026-09-23).
- `vendor_packages_branch` dan `atm_vendor_packages` di-truncate oleh mig `011`; dev di-seed ulang 2026-09-24. DB baru yang dibangun hanya dari migrasi tetap kosong untuk keduanya.
- Seed `vendor_package_prices` parsial (tanpa SSI Paket 3/6, tanpa Harga Khusus per-ATM / Biaya Tambahan).
- `backend/internal/handler/integration_test.go` hanya menjalankan `001`+`002`; `003`–`035` tidak diuji olehnya (CI `go-integration` menerapkannya lewat `cmd/migrate up`).
