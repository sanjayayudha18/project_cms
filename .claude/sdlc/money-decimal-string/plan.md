# Plan: Nilai uang di API sebagai string desimal (QC F7)

Status: **accepted 2026-10-12** (user: "terima plan, commit, lanjut coding S1-S7"). Stage: 3 Build.
Sumber: `spec.md` (accepted 2026-10-12, S1–S3), `intent.md` (accepted 2026-10-12).

## Titik sentuh yang sudah dicek di kode (2026-10-12)
- Backend service:
  - `internal/service/atm_portal.go` — `AtmWithCashPos` (`:124-137`), `rowToAtmWithCashPos` (`:285-322`), `numericToFloat64Ptr` (`:327`).
  - `internal/service/atm_portal_profile.go` — `GetATMProfile` (`:185-198`), `latestRefundTotal` (`:233`), `computeReplenishmentStatus` (`:250`), `numericToDecimalStringPtr` (`:421`).
  - `internal/service/dsr_upload.go` — `DsrDailyRow`/`DsrRencanaIsiRow` (`:336-361`), `dailyRowsToDto` (`:567`), `rencanaIsiRowsToDto` (`:601`).
- Backend handler: `internal/handler/atm_portal_handler.go:313-319`, `internal/handler/dsr_upload_handler.go:326-344`.
- Test backend yang menyentuh field ini:
  - `internal/service/atm_portal_profile_test.go` (tabel status, `*float64`)
  - `internal/service/atm_portal_filter_property_test.go:379-398` (`nullableFloatOrderOK` atas `RefundTotal`/`ReplenishTotal` hasil service)
  - `internal/handler/atm_portal_integration_test.go:209-213,317-339` (assert `*float64`)
  - `internal/service/dsr_upload_read_test.go:163,172`
  - `internal/handler/dsr_upload_handler_test.go`
  - `atm_portal_property_test.go` hanya memakai `float64` untuk generator data uji + baca `pgtype` dari DB — tidak terdampak tipe service, dibiarkan.
- CompanyPortal (`frontend/CompanyPortal-Vite/src/features/atm-portal/`):
  - `types.ts:24-30` (`AtmRecord`), `components/AtmTable.tsx:18,150-152`.
  - `lib/formatters.ts:54` (`formatRupiah`, hanya dipakai `AtmTable` + `lib/formatters.property.test.ts:30` describe "Property 8"), `__tests__/atm-portal-components.test.tsx:48-61,250-257` (fixture number + assert `Rp 5.000.000`).
- VendorPortal (`frontend/VendorPortal-Vite/src/`):
  - `features/dsr/dsrUploadApi.ts:143-161`, `features/dsr/DsrDetailDialog.tsx:11` (`formatIDR` lokal), `features/dsr/DsrUploadDialog.tsx:19` (`formatDecimalIDR` lokal, `Number()`).
  - `lib/formatters.ts` + `lib/__tests__/formatters.property.test.ts`.

## Slices (urutan kerja; satu commit untuk semua karena kontrak harus rilis bersamaan)

### S1 — Status profil eksak (FR3, S1)
- `atm_portal_profile.go`: helper `numericToRat(n pgtype.Numeric) (*big.Rat, error)` = `numericToDecimalStringPtr` → `big.Rat.SetString` (NULL → nil; NaN/∞ ditolak oleh helper yang ada, tidak duplikasi logika).
- `computeReplenishmentStatus(low, critical *big.Rat, hasReplenish bool, refund *big.Rat) string` — prioritas + batas `<=` sama, perbandingan `Cmp`.
- `latestRefundTotal` mengembalikan `*big.Rat`; `GetATMProfile` memakai `numericToRat` untuk kedua threshold.
- Test T4: `atm_portal_profile_test.go` tabel lama → `rat("…")` + kasus pecahan (`1000000.01` vs low `1000000.00` → normal; sama persis → low; refund == critical → critical); unit `numericToRat` (NULL, eksponen positif/negatif, NaN → error).

### S2 — List ATM (FR1)
- `AtmWithCashPos` 5 field → `*string` via `numericToDecimalStringPtr`; `atmResponse` → `*string`.
- Test T1: unit `rowToAtmWithCashPos` (`"1250000.50"`, `"9999999999999999.99"`, NULL → nil). T2: `atm_portal_integration_test.go` assert string (`"200.00"`, `"100.00"`, null tetap). `atm_portal_filter_property_test.go`: `nullableFloatOrderOK` → `nullableDecimalOrderOK` (parse `big.Rat` di test).

### S3 — DSR sheet detail (FR2)
- `DsrDailyRow`/`DsrRencanaIsiRow` 11 field → `*string` via `numericToDecimalStringPtr`; struct handler ikut.
- Test T3: `dsr_upload_read_test.go:163,172` → `"1500000.00"`/`"2000000.00"`; `dsr_upload_handler_test.go` assert JSON `"line_total_idr":"1500000.00"` dan `"denom_100k":null`.

### S4 — Bersih-bersih (FR3)
- Hapus `numericToFloat64Ptr`. T6: `grep -n float64` di `internal/service` + `internal/handler` non-test hanya menyisakan komentar `atm_admin.go:454` / `atm_portal_cashpos.go:213`; `go vet`, `golangci-lint`, `go test` (+ integration) hijau.

### S5 — CompanyPortal (FR4, S2)
- `types.ts`: 5 field `AtmRecord` → `string | null`.
- `AtmTable.tsx`: `formatRupiah` → `formatRupiahDecimal`.
- Hapus `formatRupiah` + describe "Property 8" di `formatters.property.test.ts` (`rupiahFormatter` ikut dihapus bila tak terpakai).
- Fixture test → string (`"1500000.00"` dst.); assert `Rp 5.000.000` tetap; tambah 1 kasus pecahan `Rp 1.250.000,50`.
- T7: `pnpm --dir frontend/CompanyPortal-Vite run test`, `build`, `lint`.

### S6 — VendorPortal (FR5, S3)
- `lib/formatters.ts`: `formatDecimalIDR(value: string | null | undefined): string` — tanpa `Number()`, grouping titik, pecahan non-nol dengan koma (trailing zero dibuang), null/undefined/"" → `"-"`. Hasil sama dengan `toLocaleString("id-ID")` untuk nilai yang ada (`"1500000.00"` → `1.500.000`).
- `DsrDetailDialog.tsx` + `DsrUploadDialog.tsx`: hapus formatter lokal, impor `formatDecimalIDR`.
- `dsrUploadApi.ts`: 11 field detail → `string | null`.
- T8: unit `formatDecimalIDR` di `lib/__tests__/formatters.property.test.ts` (bulat, pecahan, negatif, sangat besar tanpa pembulatan, null); test `DsrUploadDialog` yang ada tetap hijau; `pnpm --dir frontend/VendorPortal-Vite run test`, `build`, `lint`.

### S7 — Dokumen
- `tests.md` (stage 4) setelah semua hijau; `.claude/sdlc/README.md`; `backend_QC_fixes.md` F7 (+ baris prioritas `:35` yang masih "intent draft") + log.

## Bukti selesai
- `go test ./...` + `-tags integration` di `backend` hijau; lint hanya 2 gocyclo `masterdata_import` (ditunda).
- T1–T8 lulus; kedua frontend `test`/`build`/`lint` hijau.
- Manual (T9, tugas user, GR#10): list ATM portal + detail DSR vendor + dry-run DSR tampil sama seperti sebelum rilis. Outstanding.

## Risiko
- **R1 (utama)**: kontrak number → string. Backend tanpa frontend baru → kolom uang list ATM / detail DSR tampil salah (`formatRupiah(string)`). Mitigasi: satu commit + satu rilis (intent Q6); catat di commit message.
- **R2**: list ATM sekarang menampilkan pecahan non-nol (`Rp 1.250.000,50`) yang dulu dibulatkan — disetujui PO (S2).
- **R3**: status profil vs list bisa beda untuk nilai pecahan bila dulu float membulatkan — sekarang keduanya eksak (list di SQL, profil `big.Rat`), jadi lebih konsisten.
- **R4**: `big.Rat` per request profil — 3 nilai, biaya diabaikan.

## Di luar plan
- Rename field, status list di SQL, formatter `number` lain (invoice, dashboard) yang tidak memakai 3 endpoint ini, Python, skema.
