# Tests: Replenishment — realisasi pengisian ATM vs order (Phase 2.2c)

Status: **green 2026-10-14** (Stage 4). Input: `spec.md` (FR1–FR7, NFR), `plan.md` (S1–S9). Belum di-commit.

## Commands (run 2026-10-14)
| Command | Result |
|---|---|
| `psql … -f backend/migrations/031_replenish_realization.sql` (dev) | COMMIT (S1) |
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe` | hanya query terkait berubah |
| `cd backend && go build ./... && go vet -tags integration ./...` | clean |
| `cd backend && go test ./...` | all ok |
| `cd backend && DATABASE_URL=… go test -tags integration -p 1 -count=1 ./...` | all packages ok, **0 skip** (`TestIntegration_ApplyVaultTickets` yang gagal saat S4 kini lulus) |
| coverage (integration) | `internal/service` **81.6 %**, `internal/handler` **80.7 %**; file baru 53–100 % per fungsi (sisa = cabang error DB / `ListRealizations` handler 53.8 %) |
| `golangci-lint run --build-tags integration ./...` | 0 issues |
| `cd backend_python && py -3 -m unittest discover` (dengan `DATABASE_URL` localhost) | **77 tests OK** (replenish_classify 12: 10 rules + 2 integration; +2 dari review R2–R4) |
| `pnpm --dir frontend/CompanyPortal-Vite run test` / `lint` / `build` | 126 files / 1054 tests / clean / built |
| `pnpm --dir frontend/VendorPortal-Vite run test` / `lint` / `build` | 19 files / 136 tests / clean / built |
| sisa data tes di dev | 0 baris `vendor_request_realizations` / `replenish_classifications` / `holidays`; `REP-ABA-MDN-20261014-001` tetap `sent_to_vendor` |

## Test → requirement
| Test | Covers |
|---|---|
| `TestValidateHolidayName`, `TestParseHolidayCSV` | FR1.2 nama 1–100; FR1.3 format CSV, BOM, batas baris/ukuran, tanggal dobel di file |
| `TestAdminHolidayHandler`, `TestAdminHolidayHandler_Import` | FR1.2 API (`{id}`), RBAC 403 non-ADMIN, 409 tanggal aktif; FR1.3 multipart |
| `TestIntegration_HolidayAdmin` | FR1.2 unik tanggal aktif, enable bentrok → 409; FR1.3 semua-atau-tidak, tanggal aktif dilewati + dilaporkan; FR1.4 audit 5 aksi dalam tx, ADMIN di service |
| `AdminHolidays.test.tsx` (CompanyPortal, 5) | FR1.2/1.3 UI list, tambah, nonaktif, import |
| `TestValidateRealization` (16), `TestValidateCompletionResults` | FR2.2 K1 tanggal (≥ jadwal − 7, ≤ hari ini WIB), bulat ≥ 0, kelipatan denom, hanya denom order, minimal satu > 0, `failed` tanpa nominal; FR2.3 semua ATM tepat sekali |
| `TestVendorOrderHandler_Realization` | FR2.2 API, 404 scope, 409 status, 422 validasi |
| `TestIntegration_VendorOrder_Realization` | FR2.1 scope 404 + pihak vault ditolak + vendor lain / pihak `withdrawn` ditolak (review R1); FR2.2 status; FR2.3 `completion_pending` + `tickets.result`, resubmit menimpa; FR2.4 approve → kuota terpotong sekali, reject → `vendor_accepted`; FR2.8 notifikasi; FR7 audit dalam tx |
| `TestIntegration_VendorOrder_CompletionGateAndCancel`, `TestVendorRequestHandler_CompletionRoutes`, `TestCheckCompletionActor`, `TestCompletionTransitions`, `TestRejectCompletion_EmptyReason` | FR2.5 `/complete` → 409 `completion_by_vendor` untuk `vendor_sent`, legacy tetap ATM-USER, VENDOR-USER ditolak di jalur internal; state machine laporan selesai |
| `TestIntegration_VaultFlow_CompletionFromReadyReturnsToReady` | FR2.2/2.4 legacy `ready` |
| `replenish_classify/test_rules.py` (9) | FR1.1 hari kerja (weekend + libur); FR3.3 terdekat, seri → tanggal lalu jam, satu ITM satu tiket, di luar jendela/terminal lain diabaikan; FR3.4 offset → kelas; FR3.5 jendela tertutup; FR3.6 nominal + diff |
| `replenish_classify/test_job_integration.py` (2) | review R2 pengisian pindah tiket / void melepas ITM, R3 request ditolak → pending lalu void, R4 laporan vendor setelah final; FR3.1 idempoten (run ke-2 = 0 perubahan), final tidak disentuh; FR3.2/K2 `void`; FR3.4 `late` melewati hari libur (FR1.6); FR3.6 `has_amount_diff`, `vendor_mismatch`; FR3.7 `vendor_report_late` + notifikasi sekali per SPV (K3); FR3.8 `no_order`; tx di-rollback |
| `TestReplenishmentHandler`, `TestIntegration_ReplenishmentRead` | FR4.1 filter/paging, > 31 hari → 400; FR4.2 summary; FR4.3 RBAC; FR6.1/6.2 report; FR6.3 CSV (header, angka bulat, `currency`), audit `report_exported` |
| `TestVendorOrderHandler_ListRealizations` | FR5.2 scope vendor |
| `Replenishment.test.tsx`, `Dashboard.test.tsx` (CompanyPortal) | FR4.1 tabel, filter, badge berlabel, penanda, tombol CSV, error; FR4.2 summary dari API |
| `VendorRequestDetail` tests (CompanyPortal) | FR2.5 tombol "Laporkan Selesai" hanya legacy; `vendor_accepted` → menunggu laporan vendor; kolom laporan vendor |
| `Realization.test.tsx` (VendorPortal, 8), `integration.test.tsx` | FR5.1 form (kelipatan, K1, submit, 409/404, alasan tolak, read-only); FR5.2 halaman Realisasi + menu |

## Uji ITM di dev (S9 — langkah untuk user)
File: `backend/scripts/dev/itm_replenish_test_sample.csv` (sudah dicek dengan parser ETL + `rules`, tanpa DB).
1. VendorPortal login `vendor.aba` → Orders → terima `REP-ABA-MDN-20261014-001` (semua pihak, termasuk vault) → status `vendor_accepted`.
2. Salin CSV ke `FTP_DATA/ITM/atm_replenish/ATM_Replenish_15_Okt_2026.csv`, jalankan ETL ITM replenish (scheduler `POST /process/itm_replenish` atau script ETL).
3. `cd backend_python && py -3 -m replenish_classify.run --date 2026-10-19 --start-date 2026-10-14`
4. Harapan:

| Terminal | Kelas | Offset | Selisih |
|---|---|---|---|
| 4608 | `on_schedule` | 0 | — |
| 4686 | `late` | +1 | — |
| 6629 | `on_schedule` | 0 | 100K −10.000.000 |

Tanpa laporan vendor: ketiganya `vendor_report_late` + satu notifikasi per ATM-SPV/BRANCH-ATM-SPV. Tanpa `--date` (hari ini 14-10) → `pending`.

## Catatan / gap
- **Manual browser check: outstanding (user, Golden Rule #10)** — Kalender Libur, VendorPortal Lapor Realisasi + Realisasi, layar Replenishment + CSV, dashboard, detail Vendor Request (approve/reject laporan vendor).
- **R1 (K3) belum ada tes silang**: notifikasi yang ditulis Python belum diuji terbaca lewat query/API Go `notifications`. Kolom disalin manual; bawa ke `review.md`.
- **R3**: ETL ITM hanya membaca 50K/100K → order 10K/20K selalu tampil selisih (diterima, FR3.6).
- Kait scheduler (`_finish_retry`/`run_manual_process` + cron 09:15) tidak punya tes otomatis; tercakup langkah uji ITM di atas.
- Build Docker belum dijalankan. `-race` tidak tersedia lokal (CGO_ENABLED=0).
