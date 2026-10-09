# Intent: Nilai uang di API sebagai string desimal, bukan `float64` (QC F7)

Author: user (product owner), ditulis bersama Claude. Status: **draft 2026-10-09** — menunggu PO accept.
Stage: 1 Plan. Stage berikut: `spec.md`.
Asal: Backend QC 2026-10-09, temuan H2 (`backend_QC_result.md`), item F7 (`backend_QC_fixes.md`).
Menyentuh Sec 4 #7: **uang** (representasi nominal IDR + logika status berbasis nominal) + **kontrak API dua frontend** → wajib AI-DLC penuh. Tidak ada migrasi skema.

## Problem
Kata user (2026-10-09): *"cek quality code di backend saat ini"* → *"lanjut F7"*.

CLAUDE.md Sec 6: *"Monetary/cash amounts as numeric (or integer minor units). NEVER float/double."* Di DB semua kolom sudah `numeric`, tetapi beberapa jalur Go mengubahnya ke `float64` lewat `numericToFloat64Ptr` (`backend/internal/service/atm_portal.go:327`, memanggil `pgtype.Numeric.Float64Value()`).

Kondisi sekarang (dicek di kode, 2026-10-09):

| Endpoint | Field `float64` di JSON | Konsumen |
| --- | --- | --- |
| `GET /api/v1/atm-portal/atms` (list) | `low_threshold`, `critical_threshold`, `refund_total`, `replenish_total`, `escrow` | CompanyPortal `features/atm-portal` (`types.ts:24-30` `number \| null`, `AtmTable.tsx` → `formatRupiah`) |
| `GET /api/v1/dsr/uploads/daily/{fileId}` (VendorPortal) | `denom_100k`…`denom_1k`, `line_total_idr` | VendorPortal `features/dsr` (`dsrUploadApi.ts:143-150` `number \| null`, `DsrDetailDialog.tsx` → `formatIDR`) |
| `GET /api/v1/dsr/uploads/rencana-isi/{fileId}` (VendorPortal) | `fill_100k_idr`, `fill_50k_idr`, `splank_balance_0800_idr` | VendorPortal `features/dsr` (`dsrUploadApi.ts:159-161`) |

Plus **logika, bukan hanya tampilan**: `computeReplenishmentStatus` (`atm_portal_profile.go:250`) membandingkan `refund_total` vs `low/critical_threshold` dalam `float64` untuk status `critical`/`low`/`normal` di profil ATM (response profil sendiri sudah string).

Sudah benar (pola yang ditiru): profil ATM, riwayat replenish/cashpos per ATM, dry-run DSR, admin ATM — semua mengirim **string desimal** (`numericToDecimalString(Ptr)`, `atm_portal_cashpos.go:214`, `atm_portal_profile.go:421`). Jadi API ATM portal hari ini **campur**: endpoint list memakai number, endpoint detail memakai string untuk nilai sejenis.

Risiko nyata saat ini rendah (nilai IDR bulat aman di float64 sampai 2⁵³), tetapi: (1) melanggar aturan non-negotiable, (2) perbandingan status di float bisa salah untuk nilai pecahan, (3) kontrak tidak konsisten membuat frontend punya dua jalur format, (4) pola ini akan menular ke fitur rekonsiliasi berikutnya.

## Proposed outcome
1. **Tidak ada `float64` untuk uang** di `backend/internal/service` dan `internal/handler`; `numericToFloat64Ptr` dihapus.
2. Ketiga endpoint di atas mengirim nominal sebagai **string desimal** (sama seperti endpoint lain); nilai NULL tetap `null`.
3. Status replenishment dihitung dengan **perbandingan desimal eksak** (mis. `math/big`, atau di SQL) — hasil sama untuk data yang ada.
4. Frontend CompanyPortal (atm-portal list) dan VendorPortal (detail DSR) menerima string dan menampilkan **tanpa `parseFloat` untuk perhitungan**; format tetap IDR, `tabular-nums`, rata kanan (Sec 13).

## Affected users and systems
- Users: ATM-USER/ATM-SPV dkk. (ATM portal list), vendor (detail DSR di VendorPortal). Tampilan **tidak boleh berubah**.
- Systems: `backend/internal/service/{atm_portal,atm_portal_profile,dsr_upload}.go`, `backend/internal/handler/{atm_portal_handler,dsr_upload_handler}.go`; `frontend/CompanyPortal-Vite/src/features/atm-portal`; `frontend/VendorPortal-Vite/src/features/dsr`. Python dan DB tidak berubah.

## Constraints
- Stack tetap; tidak ada library baru (stdlib `math/big` / `pgtype` sudah tersedia).
- Response shape ATM `backend` tetap flat JSON (Sec 5); hanya tipe field yang berubah.
- Read tetap di replica (Sec 6) — tidak ada perubahan routing.
- Kontrak berubah → backend dan kedua frontend rilis bersamaan.
- Manual browser verification = tugas user (GR#10).

## Open questions (untuk PO / spec)
1. **Sorting** list ATM per `refund_total` / `replenish_total`: di SQL atau di frontend? Kalau di frontend, sort string akan salah — sort harus tetap numerik.
2. Ada konsumen lain dari ketiga endpoint (export, skrip, test e2e) selain dua frontend di atas?
3. Format string: ikuti `numericToDecimalString` yang ada (mis. `"1250000.00"`)? Kolom NULL-able tetap `null`, bukan `"0.00"`?
4. Apakah frontend boleh melakukan aritmetika (total/agregat) atas nilai ini di masa depan? Kalau ya, perlu util desimal di FE (tanpa library baru) — atau aturan: agregat selalu dari backend.
5. Perlu sekalian menyamakan nama field list (`low_threshold`) dengan profil (`low_threshold_amount`)? (Saran Claude: **tidak**, di luar scope — hanya tipe.)
6. Satu rilis untuk backend + kedua frontend OK, atau perlu masa transisi (field lama + baru)?
