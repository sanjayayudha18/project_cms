# Tests: Kontrak OpenAPI BE ↔ FE (BE-10)

Stage: 4 Test — **slice 1 green 2026-10-19** (BE spec + test kontrak Go). Slice 2 (tipe CompanyPortal) dan slice 3 (CLAUDE.md DoD) belum dibangun.
Dasar: [`plan.md`](./plan.md) (accepted 2026-10-19), [`spec.md`](./spec.md) T1–T7.

## Slice 1 — run 2026-10-19 (uncommitted)

| Perintah (dari `backend/`) | Hasil |
|---|---|
| `go test ./internal/handler/ -run OpenAPI -v` | PASS — T1 spec valid; T2 route coverage 2 prefix; T3 71 kasus response; T4 5 negatif |
| `go test ./...` | PASS semua paket (unit; integration skip tanpa `DATABASE_URL`, sama seperti sebelumnya) |
| `go test -race ./...` | tidak jalan lokal (Windows, cgo off) — jalan di job CI `go` |
| `golangci-lint run --build-tags integration ./...` | 0 issues |
| `govulncheck ./...` | 0 vuln terjangkau; 1 di module yang di-require = GO-2026-5932 `x/crypto` (sudah accepted, BE-12), bukan dari kin-openapi |
| `go mod tidy` | `kin-openapi v0.151.0` direct; indirect baru: `go-openapi/jsonpointer`, `go-openapi/swag/jsonname`, `oasdiff/yaml`, `oasdiff/yaml3`, `santhosh-tekuri/jsonschema/v6`, `rogpeppe/go-internal`. **Tidak** menarik `gorilla/mux` (S3). |

RED → GREEN: run pertama gagal di T1 (`response "BadRequest": extra sibling fields` — koma di description flow-map YAML tidak di-quote); setelah di-quote semua hijau. T4 "date as timestamp" ditambahkan untuk membuktikan `format: date` vs `date-time` benar-benar divalidasi (lulus = validator menolak).

## Trace test ↔ requirement

| Test | Bukti | FR |
|---|---|---|
| T1 `TestOpenAPIContract_SpecValid` | loader + `doc.Validate()` | FR1, FR3 |
| T2 `TestOpenAPIContract_RouteCoverage` | `chi.Walk` (handler dengan vault + vendor side) == operasi spec, per prefix | FR4 |
| T3 `TestOpenAPIContract_Responses` | 26 operasi: sukses (fixture lengkap + minimal/null), error per operasi, semua kelas `handleError` lewat `/submit`, 401/403 middleware per prefix; `IncludeResponseStatus` → status tak terdokumentasi gagal | FR1, FR2, FR5 |
| T4 `TestOpenAPIContract_RejectsDrift` | field tambahan, uang sebagai string, field hilang, tanggal sebagai timestamp, status 418 → semua ditolak | FR2 |
| T5 / T6 | slice 2 — belum | FR6, FR7 |
| T7 | baris tabel di atas | NFR1–4 |
| Manual | alur Vendor Request (CP) + Orders (VP) di browser — **outstanding (user)**; slice 1 tidak mengubah perilaku runtime | GR#10 |

## Temuan (dicatat, tidak diperbaiki — spec mengikuti kode)

| # | Temuan | Di mana |
|---|---|---|
| F1 | Tanggal di vendor orders dikirim sebagai timestamp RFC3339 (`replenish_date`, `realized_date` = `time.Time` → `"2026-10-20T00:00:00Z"`), sedangkan `/vendor-requests` mengirim `YYYY-MM-DD`. FE harus memotong sendiri. | `service.VendorOrderSummary`, `RealizationView` |
| F2 | Uang campur bentuk: `int64` JSON integer di vendor request/order; string desimal di vault plan (`order`, `amounts`, `saldo_snapshot`, `capacity_snapshot`, `order_totals`) dan `escrow` forecast. | `vault_plan.go` `amountsJSON`, `ForecastRow.Escrow` |
| F3 | 401 dari middleware memakai `error: "token_invalid"`, 401 dari handler `error: "unauthorized"` (bentuk sama, kode beda). | `pkg/middleware/rbac.go`, `handler/error_response.go` |
| F4 | Timestamp vault plan (`formatTimestamptz`) tidak di-`UTC()` seperti `formatTimestamp`; offset bisa bukan `Z`. Tetap `date-time` valid. | `handler/admin_user_handler.go:445` |
| F5 | Validasi query vendor orders → 422 `validation_error`; validasi query vendor requests → 400 `bad_request`. | dua `handleError` |
| F6 | Bentuk vault plan dibangun dengan `map[string]any` (`vaultPlanJSON`), jadi compiler tidak menjaga bentuknya — kini hanya spec + T3 yang menjaga. | `handler/vault_plan_handler.go:124` |
| F7 | `PartyContent` di-decode dari snapshot JSON lama; array yang belum ada saat snapshot ditulis keluar `null` → didokumentasikan `nullable`. | `service.PartyContent` |
| F8 | `handleError` vendor request dipakai semua operasi → spec mendokumentasikan himpunan error yang sama (400/401/403/404/409/422/500) di setiap operasi, termasuk GET. | `vendor_request_handler.go:581` |

## Slice 2 / 3
Belum dijalankan. Gate slice 2: sisa FE-12 (`VendorRequestCreate*.tsx`, `vendorRequestCreateForm.ts`) ter-commit.
