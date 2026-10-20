# Tests: Kontrak OpenAPI BE ↔ FE (BE-10)

Stage: 4 Test — **slice 1 green 2026-10-19** (BE spec + test kontrak Go), **slice 2 green 2026-10-19** (tipe CompanyPortal). **slice 3 done 2026-10-19** (CLAUDE.md Sec 2/2a/11, diff diterima user).
Dasar: [`plan.md`](./plan.md) (accepted 2026-10-19), [`spec.md`](./spec.md) T1–T7.

## Slice 1 — run 2026-10-19 (committed `d1e7082`)

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
| T5 `src/lib/api/schema.test.ts` | `openapiTS` + `astToString` dari YAML == isi `schema.d.ts` (normalisasi CRLF); RED dibuktikan dengan baris tambahan di `schema.d.ts` → gagal "schema.d.ts is stale" | FR6 |
| T6 `tsc -b tsconfig.app.json` (dalam `build`) | tipe wire `features/vendor-request/types.ts` + `vendorSide.ts` = alias `components["schemas"]`; pemanggil lulus typecheck | FR7 |
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

## Slice 2 — run 2026-10-19 (committed `062b0d9`)

Gate terpenuhi: `git status` bersih, sisa FE-12 sudah ter-commit (`b007b64`/`36abcf8`).

| Perintah (dari `frontend/CompanyPortal-Vite`) | Hasil |
|---|---|
| `pnpm add -D --save-exact openapi-typescript@7.13.0` | devDependency di-pin; lockfile +33 paket (redocly/openapi-core dkk.), sisanya hanya varian peer `supports-color` |
| `pnpm run gen:api` | `src/lib/api/schema.d.ts` 1.9k baris |
| `pnpm exec vitest run src/lib/api/schema.test.ts` | PASS (T5); RED dicek: baris tambahan → FAIL, `gen:api` → PASS |
| `pnpm run build` | PASS (tsc app + vite + bundle budget 172,5 KB / 300 KB) |
| `pnpm run test:coverage` | 157 file / 1386 test PASS; all files 84,82 % stmt / 80,74 % branch — ambang tidak turun |
| `pnpm run lint` | PASS setelah `schema.d.ts` di-ignore di `biome.json` (file generated, format biome ≠ output generator) |
| `tsc -p tsconfig.test.json` | 122 error **sebelum dan sesudah** (identik per file; dibandingkan dengan `types.ts`/`vendorSide.ts` HEAD) — utang lama, tidak di-gate CI, bukan dari slice ini |
| `go test ./internal/handler/ -run OpenAPI` (backend) | PASS setelah enum ditambah ke spec |

Perubahan spec di slice 2 (lebih ketat, T3 tetap hijau): schema `VendorRequestStatus` (enum 15 nilai = `vendor_requests_status_chk` migrasi 029) dipakai `VendorRequestDetail.status`/`VendorRequestSummary.status`; `RequestVendorPartyEvent.event` enum 6 nilai (CHECK migrasi 029). Tanpa ini alias akan melebarkan union FE menjadi `string`.

Drift FE ↔ spec yang terlihat saat alias (typecheck tetap lulus, tidak ada kode FE diubah):

| # | Temuan | Arah |
|---|---|---|
| F9 | `AuditLogEntry.metadata` bisa `null` (spec) — tipe manual lama bilang selalu object | FE kini dipaksa menangani null |
| F10 | `CreateVendorRequestPayload.request_category` wajib (spec, sesuai validasi BE); `vendor_id` opsional (spec) — tipe manual lama kebalikannya | FE sudah selalu mengirim keduanya; tipe kini mengikuti BE |
| F11 | Nama berbeda: `PaginationMeta`=`Pagination`, `VendorParty*`=`RequestVendorParty*`, `ForecastSummaryCounts` = `ForecastSummaryTotals` tanpa `unassigned_atm_count` | nama export FE dipertahankan sebagai alias |

## Review R1 — 2026-10-19 (committed `a1b78c9`)
`TestOpenAPIContract_Responses` gagal bila operasi spec tidak punya kasus response (`review.md` R1): hijau 26/26, RED dengan operasi probe. Manual browser check tetap **outstanding (user)**.

## Slice 3 — 2026-10-19 (committed `062b0d9`)
CLAUDE.md: Sec 2 baris "API contract", Sec 2a perintah `go test ./internal/handler -run OpenAPI` + `gen:api`, Sec 11 butir DoD kontrak (FR8, teks Inggris). Diff ditunjukkan dan diterima user (*"Diff CLAUDE.md slice 3 diterima, simpan dan update status docs."*). Docs saja, tanpa perubahan kode/test.
