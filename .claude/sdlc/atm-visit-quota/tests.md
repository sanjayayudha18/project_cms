# Tests: Kuota kunjungan replenish per ATM + laporan selesai Vendor Request

Status: build + automated tests **green** (2026-10-01). Stage: 4 Test.
Input: `plan.md` (accepted 2026-10-01). Trigger ke stage berikut: review (`review.md`) → code owner merge.
**Manual browser check: OUTSTANDING — dilakukan user** (Golden Rule #10). Tidak ditandai selesai di sini.
Prasyarat uji browser: seed ulang `vendor_packages_branch` + `atm_vendor_packages` di dev (kosong sejak migrasi 011) dengan `package_code` = label di `package_frequencies` (mis. `PAKET 5`), lalu dua user berbeda (ATM-USER pelapor, ATM-SPV penyetuju).

## Perintah & hasil (2026-10-01)
| Perintah | Hasil |
|---|---|
| `psql … -f backend/migrations/021_atm_visit_quota.sql` (dev, localhost) | OK (disetujui user), 3 tabel ada |
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe`→`UserLeave` | OK; diff `internal/db`: `models.go`, `vendor_request.sql.go`, baru `atm_visit_quota.sql.go` |
| `cd backend && go build ./... && go vet ./...` | OK |
| `cd backend && go test -count=1 ./...` | OK (semua paket) |
| `cd backend && DATABASE_URL=<dev, localhost> go test -tags integration -count=1 ./internal/service ./internal/handler ./internal/repository` | OK (termasuk 7 test `TestIntegration_VisitQuota_*`; ke-7 `…_UnknownTerminalSkipped` ditambah di review R1); DB dev bersih setelahnya (0 baris `IVQ*`) |
| `gofmt -l` pada file Go yang dibuat/diubah | bersih |
| `pnpm --dir frontend/CompanyPortal-Vite run test` | 122 file, 1036 test lulus |
| `pnpm --dir frontend/CompanyPortal-Vite run lint` (biome, seluruh proyek) | bersih |
| `pnpm --dir frontend/CompanyPortal-Vite run build` (`tsc -b tsconfig.app.json && vite build`) | OK (warning chunk >500 kB sudah ada sebelumnya) |

## Traceability test → spec
| Spec | Test |
|---|---|
| FR1 transisi `approved→completion_pending→completed`/`→approved`; tidak ada cancel dari `completion_pending`; approve ganda = transisi invalid | `service/vendor_request_completion_test.go` `TestCompletionTransitions`; integration `…_ApproveDecrements`, `…_RejectAndResubmit` |
| FR1 maker = ATM-USER/BRANCH-ATM-USER/ADMIN; checker ≠ pelapor; ATM-USER tidak bisa approve | `TestCheckCompletionActor`; integration `…_RejectAndResubmit` (AC4) |
| FR1.1 hasil wajib untuk setiap terminal, tanpa duplikat/terminal asing/nilai lain | `TestValidateCompletionResults`; integration `…_RejectAndResubmit` |
| FR1.2 approve atomik (kunjungan + kuota + status + audit satu tx) | integration `…_FailedApproveRollsBack` (AC9: aktor tak dikenal → rollback, tanpa baris kuota, status tetap `completion_pending`) |
| FR1.3 tolak → `approved`, kuota tetap, ajukan ulang menimpa hasil | integration `…_RejectAndResubmit` (AC5) |
| FR2.1/2.2 PAKET 5 → 4; ATM gagal tidak berubah; 2 baris item per ATM = 1 kunjungan | integration `…_ApproveDecrements` (AC1) |
| FR2.2 dua approve paralel pada ATM yang sama (tanpa baris kuota) → turun dua kali, tanpa error | integration `…_ParallelApprovals` (AC3 paralel, koneksi terpisah) |
| FR2.3 kelebihan kuota: remaining −1, sisa 0 + kelebihan 1, `is_over_quota`, approve tetap sukses | integration `…_OverQuotaStillApproves` (AC2); `TestVisitSisa` |
| FR3.2/3.3 batal kunjungan +1, alasan wajib, batal ganda / sebelum reset → 409 | integration `…_ResetAndCancel` (AC8) |
| FR4.1 reset ATM = `cr_frequency`, `cr_frequency` tidak berubah, ATM-USER ditolak, paket tak dikenali → 422 | integration `…_ResetAndCancel` (AC6) |
| FR4.2 reset vendor: known di-reset, unknown di-skip | integration `…_ResetAndCancel` (AC7) |
| Sec 5 audit tiap aksi | integration `…_ResetAndCancel` (AC9: ≥ 3 baris `atm_visit_quota`/`atm_visit`) |
| FR5 route role gate + mapping 400/403/404/409/422 | `handler/atm_visit_quota_handler_test.go` `TestAtmVisitQuotaHandler_RoleGates`, `…_ErrorMapping`, `TestVendorRequestHandler_CompletionRoutes` |
| FR5 `GET /forecast` kolom `visit_remaining`/`visit_quota_total`, ringkasan `vendor_id` | integration forecast tests yang ada tetap lulus (query diubah); UI `ForecastTable.test.tsx`, `ForecastSummary.test.tsx` |
| FR6.1 dialog laporan: default Berhasil, toggle Gagal, payload | `__tests__/VendorRequestDetail.test.tsx` "report dialog defaults…" |
| FR6.2 tombol review hanya checker ≠ pelapor; peringatan ATM yang akan melebihi kuota | `VendorRequestDetail.test.tsx` "completion_pending…", "approve dialog warns…" |
| FR6.3 tabel per ATM: hasil, sisa/kuota, badge "Kelebihan kuota" (teks + ikon) | `VendorRequestDetail.test.tsx` "completed: per-ATM table…" |
| FR6.5 kartu Profil ATM: tampilan, "Paket tidak dikenali", reset 2 langkah, batal dengan alasan, tersembunyi saat 403 | `atm-portal/__tests__/VisitQuotaCard.test.tsx` |
| FR6.6 kolom Sisa Kunjungan (`-` bila null, penanda habis teks + ikon); reset vendor hanya checker, 2 langkah | `ForecastTable.test.tsx` "Sisa Kunjungan", `ForecastSummary.test.tsx` "Reset Kuota Vendor" |

## Belum diuji / catatan
- **Manual browser check (user)**: alur lengkap lapor → setujui (dengan peringatan) → sisa di Profil ATM & Forecast Browser → batal kunjungan → reset ATM/vendor; tampilan `<dialog>` dan toast.
- NFR "reset massal ≤ 3 s" belum diukur — dev DB tidak punya kelolaan ATM (migrasi 011). Ukur setelah seed (`EXPLAIN ANALYZE ListAtmsForVendorQuotaReset`).
- Sudah ada sebelumnya, di luar fitur ini: `tsc -b tsconfig.test.json` punya ±139 error tipe lama (noUncheckedIndexedAccess, fixture lama mis. `VendorRequestCreate.test.tsx` tanpa `is_requested`); `gofmt -l` menandai `vendor_request_summary_integration_test.go`, `admin_region_handler.go` dll. (CRLF/format lama, tidak disentuh).
