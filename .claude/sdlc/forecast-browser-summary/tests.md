# Tests: Forecast Browser — ringkasan vendor × region

Status: build + automated tests **green** (2026-09-30). Stage: 4 Test.
Input: `plan.md` (accepted 2026-09-30). Trigger ke stage berikut: review (`review.md`) → code owner merge.
**Manual browser check: OUTSTANDING — dilakukan user** (Golden Rule #10). Tidak ditandai selesai di sini.

## Perintah & hasil (2026-09-30)
| Perintah | Hasil |
|---|---|
| `cd backend && sqlc generate` (v1.31.1) + fix `UserLeafe`→`UserLeave` | OK; `git diff --stat internal/db` hanya `vendor_request.sql.go` |
| `cd backend && go build ./... && go vet ./internal/...` | OK |
| `cd backend && go test ./...` | OK (semua paket) |
| `cd backend && DATABASE_URL=<dev, localhost> go test -tags integration ./internal/service ./internal/handler ./internal/repository -count=1` | OK |
| `gofmt` pada file yang diubah (setelah strip CRLF) | bersih |
| `pnpm --dir frontend/CompanyPortal-Vite run test` | 121 file, 1020 test lulus |
| `pnpm --dir frontend/CompanyPortal-Vite run lint` (biome) | bersih, 0 warning |
| `pnpm --dir frontend/CompanyPortal-Vite run build` | OK (warning chunk >500 kB sudah ada sebelumnya) |
| `EXPLAIN ANALYZE SummarizeForecastForDate` di dev, tanggal tersibuk (2026-07-20, 513 rekomendasi, 1.919 ATM) | **13 ms** (target ≤ 3 s p95) |

## Traceability test → spec
| Spec | Test |
|---|---|
| FR1 totals = Σ grup, unrequested = atm − requested, grup "" → unassigned | `service/vendor_request_forecast_test.go` `TestForecastSummary_TotalsAreSumOfGroups` |
| FR1 tanggal tanpa data → `groups: []`, totals nol | `TestForecastSummary_EmptyDateReturnsEmptyGroups` |
| FR1 validasi tanggal → 400 | `TestForecastSummary_ValidatesDate`, `handler` `TestVendorRequestHandler_ForecastSummary_ValidationError` |
| FR1 route, RBAC 401/403/200, bentuk JSON flat | `handler` `TestVendorRequestHandler_ForecastSummary` |
| FR1 invarian Σ grup = distinct terminal list; grup (V,R) = list terfilter; Count = len(List) | `vendor_request_summary_integration_test.go` subtest (a)(b)(f) |
| Definisi "sudah": cancelled/rejected = belum; ATM 2 denom, 1 di-request = belum | integration subtest (c)(d) |
| FR2 vendor/region opsional; `unassigned` + vendor/region → error | `TestBrowseForecast_ValidatesCIT2Filters` (kasus "wajib dipilih" lama diganti) |
| FR2 parse `unassigned` (""/false/true/abc→400) | `TestVendorRequestHandler_BrowseForecast_UnassignedParam` |
| FR2 `unassigned=true` hanya ATM tanpa paket | integration subtest (e) |
| FR3 `is_requested` map + wire + nilai per baris dari DB | `TestBrowseForecast_MapsIsRequested`, `TestVendorRequestHandler_BrowseForecast_IsRequestedOnWire`, integration "is_requested on list rows" |
| FR4 KPI, status berlabel (bukan warna saja), Rp tabular-nums, `aria-current`, callback, empty, error | `ForecastSummary.test.tsx` |
| FR4 badge "Sudah di-request"/"Tanpa vendor"/"Belum", checkbox disabled | `ForecastTable.test.tsx` "request status" |
| FR4 halaman memuat ringkasan + detail tanpa filter | `ForecastBrowser.test.tsx` "loads the recap and the unfiltered detail table…" |
| FR4 klik grup → filter vendor+region; grup tanpa vendor → `unassigned=true` + chip | "clicking a recap row…", "clicking the no-vendor recap row…" |
| FR4 pilihan 2 vendor → tombol Buat disabled + teks bantu | "disables Buat Vendor Request…" |
| FR4 Pilih Semua butuh vendor; melewati `is_requested` | "disables Pilih Semua until a vendor is chosen…" |
| Regresi perilaku lama (paging, select-all cap 1000, reset pilihan, brand) | test lama `ForecastBrowser.test.tsx` tetap lulus |

## Manual check (OUTSTANDING — user)
Jalankan backend :8080 + `pnpm --dir frontend/CompanyPortal-Vite run dev`, login ATM-USER, buka `/replenishment/forecast-browser`:
- [ ] AC1 ringkasan + detail tampil tanpa memilih filter.
- [ ] AC2 grup "Tanpa vendor aktif" muncul (bila ada) → klik → detail hanya ATM itu; chip bisa dihapus.
- [ ] AC3 klik grup (V,R) → jumlah ATM distinct di detail = kolom ATM ringkasan.
- [ ] AC4 buat Vendor Request sebagian ATM → kembali → "Sudah" naik, "Belum" turun, badge "Sudah di-request", checkbox disabled.
- [ ] AC5 batalkan/tolak request itu → ATM kembali "Belum".
- [ ] AC6 pilih ATM dua vendor → tombol Buat disabled + teks bantu.
- [ ] Tampilan layar sempit (tabel scroll horizontal, KPI 2 kolom).
- Catatan dev: `vendor_packages_branch`/`atm_vendor_packages` perlu di-seed ulang (CLAUDE.md Sec 12) — tanpa itu semua ATM masuk grup "Tanpa vendor aktif".

## Catatan
- Select-all cap 1000 dihitung dari total baris filter (termasuk yang sudah di-request) — lebih konservatif, tidak pernah melebihi cap.
- Di luar scope, ditemukan saat cek format: `internal/handler/admin_region_handler.go` sudah tidak gofmt-clean sebelum perubahan ini.
