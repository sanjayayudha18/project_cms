# Tests / feedback loop: Nilai uang di API sebagai string desimal (QC F7)

Status: **green 2026-10-12**. Stage: 4 Test. Reads: `plan.md` (accepted 2026-10-12), `spec.md`.

## Commands (dijalankan 2026-10-12)
- `backend`: `gofmt -l` kosong; `go build ./...`, `go vet -tags integration ./...` bersih; `go test ./...` hijau; `DATABASE_URL=<dev, localhost> go test -tags integration -count=1 -p 1 ./...` hijau (8 paket ok).
- `backend`: `golangci-lint run --build-tags integration ./...` → 2 temuan gocyclo `masterdata_import` (ditunda di F10), tidak ada yang baru.
- CompanyPortal: `pnpm test` 127 file / 1078 test hijau; `build` hijau; `lint` (biome) bersih.
- VendorPortal: `pnpm test` 18 file / 123 test hijau; `build` hijau. `lint` sudah gagal di HEAD sebelum F7 (1 error format lama di `DsrUploadDialog.test.tsx` + warning a11y/non-null lama); untuk 5 file yang diubah F7: HEAD 3 error / 6 warning → sekarang 0 error / 6 warning.

## Test cases (trace ke spec)
| ID | Req | Test | Hasil |
| --- | --- | --- | --- |
| T1 | FR1 | `service/atm_portal_profile_test.go` `TestRowToAtmWithCashPos_MoneyAsDecimalString`: `"1000000.00"`, `"1250000.50"`, `"9999999999999999.99"`, `"0.00"`, NULL → nil | ✅ |
| T2 | FR1 | `handler/atm_portal_integration_test.go` (integration): `refund_total`/`low_threshold` = `"200.00"`/`"100.00"` (decode ke `*string` gagal bila JSON number), null tetap null | ✅ |
| T3 | FR2 | `service/dsr_upload_read_test.go` (`"1500000.00"`, `"2000000.00"`, denom NULL → nil) + `handler/dsr_upload_handler_test.go` (JSON `"line_total_idr":"1500000.00"`, `"denom_100k":null`, `"fill_100k_idr":"2000000.00"`) | ✅ |
| T4 | FR3, NFR3 | `TestComputeReplenishmentStatus` (`*big.Rat`): 7 kasus lama + batas pecahan (`1000000.01` → normal, sama persis → low, critical `500000.50` → critical, selisih 0,01 di atas presisi float → normal); `TestNumericToRat` (NULL, eksponen positif/negatif, NaN → error) | ✅ |
| T5 | FR3, NFR3 | Property test integration `atm_portal_property_test.go` + `atm_portal_filter_property_test.go` (sort order dibandingkan `big.Rat`, `nullableDecimalOrderOK`) | ✅ |
| T6 | FR3 | `grep float64` di `internal/service` + `internal/handler` non-test: hanya 2 komentar; `numericToFloat64Ptr` dihapus | ✅ |
| T7 | FR4 | `atm-portal-components.test.tsx`: fixture string, `Rp 5.000.000`, null → `—`, pecahan `Rp 1.250.000,50`; test/build/lint | ✅ |
| T8 | FR5 | `lib/__tests__/formatters.property.test.ts` `formatDecimalIDR`: bulat, pecahan (`,50`), negatif, 16 digit tanpa pembulatan, non-desimal apa adanya, null/undefined/"" → `-`; property: sama dengan `toLocaleString("id-ID")` untuk rupiah bulat 0–10¹² | ✅ |
| T9 | FR4, FR5 | Manual browser (tugas user, GR#10) | **outstanding** |

## Deviasi dari plan (dikonfirmasi user 2026-10-12)
- **`formatRupiah` tidak dihapus**: ternyata juga dipakai `features/dmaa-forecast/DmaaForecastTable.tsx:131` (grep plan hanya di folder atm-portal). User: *"Pertahankan untuk DMAA"*. DMAA mengirim `amount_replenish`/`amount_refund` sebagai `int64` rupiah bulat (`service/dmaa_forecast.go:70`) — sesuai Sec 6 (integer), jadi bukan temuan baru.
- **`formatRupiahDecimal` diperbaiki**: pecahan non-nol dulu tampil `,5` (nol dibuang) walau komentar fungsinya menulis `,50`. User: *"Perbaiki ke 2 digit"*. Ikut berlaku di profil/riwayat replenish/cashpos untuk nilai pecahan; nilai bulat tidak berubah. `formatDecimalIDR` VendorPortal memakai aturan yang sama (dulu `toLocaleString` menampilkan `,5`).

## Manual (tugas user, GR#10) — outstanding
- CompanyPortal ATM Portal: list (Refund Total, Total Replenish, Low Threshold) + profil ATM (status, header) tampil sama seperti sebelum rilis.
- VendorPortal DSR: detail Saldo + Rencana Isi dan dry-run upload tampil sama.
- Rilis backend + kedua frontend bersamaan (R1).
