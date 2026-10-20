# Spec: Nilai uang di API sebagai string desimal, bukan `float64` (QC F7)

Status: **accepted 2026-10-12** (user: "terima spec, commit, lanjut plan"; S1–S3 dikonfirmasi). Stage: 2 Design — selesai. Stage berikut: `plan.md`.
Sumber: `intent.md` (accepted 2026-10-12, Q1–Q6).

## ⚠ Flagged (Sec 4 #7)
- **Uang**: representasi nominal IDR di 3 endpoint + logika status replenishment (`critical`/`low`/`normal`) di profil ATM.
- **Kontrak API dua frontend**: tipe field berubah `number` → `string` (nama field tetap). Backend + CompanyPortal + VendorPortal dirilis bersamaan (intent Q6).
- Tidak ada migrasi skema, tidak ada endpoint baru, routing replica tidak berubah. Python tidak disentuh.

## Kondisi di kode (dicek 2026-10-12)
- `numericToFloat64Ptr` (`backend/internal/service/atm_portal.go:327`, `pgtype.Numeric.Float64Value()`) dipakai di:
  - `rowToAtmWithCashPos` (`atm_portal.go:286-302`) → `AtmWithCashPos` (`:131-137`) → handler `atm_portal_handler.go:313-319`.
  - `GetATMProfile` (`atm_portal_profile.go:185-198`) + `latestRefundTotal` (`:233`) → **hanya** untuk `computeReplenishmentStatus` (`:250`); nilai yang dikirim profil sudah string.
  - DSR sheet detail (`dsr_upload.go:572-612`) → `DsrDailyRow`/`DsrRencanaIsiRow` (`:342-361`) → handler `dsr_upload_handler.go:326-344`.
- Status di **list** ATM dihitung di SQL (`queries/atm_portal.sql:37,122,173`, numeric) — sudah eksak, tidak diubah.
- Helper string yang ada: `numericToDecimalString` (`atm_portal_cashpos.go:214`, minimal 2 digit pecahan, mis. `"1250000.00"`; NULL → `"0.00"`) dan `numericToDecimalStringPtr` (`atm_portal_profile.go:421`, NULL → `nil`).
- Frontend:
  - CompanyPortal `features/atm-portal/types.ts:24-30` (`AtmRecord`, `number | null`), dipakai hanya `AtmTable.tsx` (`formatRupiah`, `:150-152`) + fixture test `atm-portal-components.test.tsx:48-61`. `formatRupiah(number)` (`lib/formatters.ts:54`) hanya dipakai di sini dan di `formatters.property.test.ts`. Sudah ada `formatRupiahDecimal(string)` (`:86`, tanpa `Number()`), dipakai profil/cashpos.
  - VendorPortal `features/dsr/dsrUploadApi.ts:143-161` (`number | null`) → `DsrDetailDialog.tsx` (formatter lokal `formatIDR(number)` `:11`, `toLocaleString`). Dry-run (`DsrUploadDialog.tsx`) sudah menerima string tetapi formatter lokalnya `formatDecimalIDR` (`:19`) memakai `Number(value)`.

## Functional requirements

### FR1 — Backend: list ATM portal
- `AtmWithCashPos.LowThresholdAmount/CriticalThresholdAmount/RefundTotal/ReplenishTotal/Escrow` → `*string` via `numericToDecimalStringPtr`; handler `atmResponse` field yang sama → `*string`.
- JSON: `low_threshold`, `critical_threshold`, `refund_total`, `replenish_total`, `escrow` = string desimal (`"1250000.00"`) atau `null`. Nama field, urutan, sorting (SQL) tidak berubah.

### FR2 — Backend: DSR sheet detail (VendorPortal)
- `DsrDailyRow.Denom100k…Denom1k`, `LineTotalIdr`, `DsrRencanaIsiRow.Fill100kIdr/Fill50kIdr/SplankBalance0800Idr` → `*string` via `numericToDecimalStringPtr`; handler struct ikut. NULL tetap `null`.

### FR3 — Backend: status replenishment eksak
- `computeReplenishmentStatus` menerima nilai uang sebagai `pgtype.Numeric` dan membandingkan secara eksak (keputusan **S1**). Urutan prioritas tetap: `unconfigured` > `no_data` > `critical` > `low` > `normal`; batas inklusif (`<=`) tetap.
- `latestRefundTotal` mengembalikan `pgtype.Numeric`.
- `numericToFloat64Ptr` **dihapus**; tidak ada lagi `float64` untuk uang di `internal/service` dan `internal/handler` (non-test).

### FR4 — CompanyPortal
- `AtmRecord` 5 field → `string | null`. `AtmTable.tsx` memakai `formatRupiahDecimal` (keputusan **S2**). Tampilan nilai bulat sama persis (`Rp 1.250.000`, null → `—`), `tabular-nums`, rata kanan.
- Tanpa `Number()`/`parseFloat` atas nilai uang.

### FR5 — VendorPortal
- `DsrSheetDetailDailyRow`/`DsrSheetDetailRencanaIsiRow` field uang → `string | null`. `DsrDetailDialog.tsx` memformat string tanpa `Number()` (keputusan **S3**). Tampilan nilai bulat sama (`1.250.000`, null → `-`).

## Non-functional
- **NFR1**: tanpa library baru (Go `math/big`/`pgtype`; FE string murni).
- **NFR2**: tidak ada aritmetika uang di FE (intent Q4); FE hanya memformat.
- **NFR3**: hasil status profil untuk data yang ada identik dengan sebelumnya (dibuktikan test tabel + property test yang sudah ada, dikonversi ke Numeric).

## Tests (dibuat di stage 4, dilacak di `tests.md`)
| ID | Uji | Req |
| --- | --- | --- |
| T1 | `rowToAtmWithCashPos`: numeric → string eksak (`"1250000.50"`, besar `9999999999999999.99`), NULL → nil | FR1 |
| T2 | Handler list ATM (integration yang ada, `atm_portal_integration_test.go`): JSON field uang bertipe string / null | FR1 |
| T3 | DSR sheet detail (service `dsr_upload_read_test.go` + handler `dsr_upload_handler_test.go`): string eksak + null | FR2 |
| T4 | `computeReplenishmentStatus`: tabel lama dikonversi ke Numeric + kasus batas pecahan (refund `1000000.01` vs low `1000000.00` → `normal`; sama persis → `low`; critical == refund → `critical`) | FR3, NFR3 |
| T5 | Property test status/filter yang ada (`atm_portal_property_test.go`, `atm_portal_filter_property_test.go`) tetap lulus dengan input Numeric | FR3, NFR3 |
| T6 | Lint/grep: tidak ada `float64` uang di `internal/service` + `internal/handler` non-test; `numericToFloat64Ptr` tidak ada | FR3 |
| T7 | CompanyPortal: fixture `AtmRecord` string; tabel menampilkan `Rp 1.250.000`, null → `—`; `pnpm test` + `build` + `lint` | FR4 |
| T8 | VendorPortal: `DsrDetailDialog` render string (bulat + pecahan + null); `pnpm test` + `build` + `lint` | FR5 |
| T9 | Manual browser (tugas user, GR#10): list ATM portal + detail DSR vendor tampil sama seperti sebelum rilis | FR4, FR5 |

## Keputusan spec (dijawab PO 2026-10-12, S1–S3 semua sesuai usulan)
- **S1** ✅ helper Numeric di Go. **S1 — Cara membandingkan uang di Go**: (a) bandingkan `pgtype.Numeric` langsung lewat helper kecil `numericCmp` (`big.Int` diskalakan ke eksponen yang sama), atau (b) pindahkan status profil ke SQL seperti list. Usulan: **(a)** — perubahan kecil, test tabel tetap di Go, query profil tidak berubah.
- **S2** ✅ `formatRupiahDecimal`, hapus `formatRupiah`. **S2 — Format pecahan di list ATM**: `formatRupiah` sekarang **membulatkan** ke rupiah penuh; `formatRupiahDecimal` (dipakai profil/cashpos) menampilkan pecahan bila bukan nol (`Rp 1.250.000,50`). Usulan: **pakai `formatRupiahDecimal`** (list dan profil konsisten; nilai bulat tampil sama) dan **hapus `formatRupiah`** + property test-nya (tidak ada pemakai lain).
- **S3** ✅ detail + dry-run. **S3 — Formatter VendorPortal**: satu formatter string `formatDecimalIDR` di `src/lib/formatters.ts` tanpa `Number()` (grouping titik, pecahan non-nol dengan koma), dipakai `DsrDetailDialog` **dan** `DsrUploadDialog` (dry-run, sekarang memakai `Number()`). Usulan: **ya, keduanya** — satu jalur format, tidak lewat float sama sekali. Alternatif: hanya `DsrDetailDialog` (dry-run dibiarkan).

## Out of scope
- Rename field (`low_threshold` → `low_threshold_amount`) — intent Q5.
- Status list ATM di SQL (sudah eksak).
- `formatIDR`/`formatRp(number)` lain di VendorPortal/CompanyPortal yang tidak memakai ketiga endpoint ini (mis. invoice, dashboard mock) — dicatat bila ditemukan float uang dari API lain, bukan diperbaiki di sini.
- Python EOD, skema DB.
