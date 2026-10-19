# Plan: Kontrak OpenAPI BE ↔ FE (BE-10) — pilot Vendor Request

Status: **accepted 2026-10-19** (user: *"Terima plan openapi-contract, mulai build slice 1."*).
Stage: 3 Build. Dasar: [`spec.md`](./spec.md) (accepted 2026-10-19, S1–S7 sesuai usulan). Stage berikut: `tests.md`.
Skill saat build: `ecc:golang-patterns` + `ecc:golang-testing` (slice 1), `ecc:contract-first`.

## Versi dependency (dicek 2026-10-19)
- `github.com/getkin/kin-openapi` **v0.151.0** (terbaru di proxy). Saat `go get`: catat dependency transitif baru di `go.sum`, `go mod tidy`, `govulncheck` harus 0 vuln terjangkau. Bila v0.151.0 butuh `go` > 1.26 atau menarik `gorilla/mux` lewat paket yang kita impor → STOP, lapor, pilih versi lebih lama.
- `openapi-typescript` **7.13.0** (rilis ≥ 2 minggu lalu), devDependency CP, di-pin tanpa `^`. Peer `typescript ^5` cocok dengan CP `^5.7.0`.

## Slice 1 — Spec + test kontrak Go (BE saja; bisa mulai sekarang)

| # | File | Isi |
|---|---|---|
| 1.1 | `backend/api/openapi.yaml` (baru) | OpenAPI 3.0.3, `servers: /api/v1`, `bearerAuth`, komponen `ErrorResponse`/`FieldError`, 21 operasi `/vendor-requests*` + 5 `/vendor/replenish-orders*`. Schema dibaca dari struct response di `handler/vendor_request_response.go`, `vendor_order_handler.go`, `vendor_request_vault_handler.go`, `vendor_request_vendor_handler.go` + tipe `service.*` yang di-encode langsung. Status per operasi dari `writeError`/`writeJSON` di handler + mapping error service. `additionalProperties: false` di setiap object response (S2). |
| 1.2 | `backend/api/embed.go` (baru, ±5 baris) | `package api` + `//go:embed openapi.yaml` → `var Spec []byte`. Supaya test di `internal/handler` memuat spec tanpa path relatif. Tidak diimpor kode produksi. |
| 1.3 | `backend/internal/handler/openapi_contract_test.go` (baru) | Helper: load + `Validate()` (T1); `chi.Walk` dua `Routes()` (vault + vendor side terpasang memakai fake yang sudah ada) vs `doc.Paths` dengan prefix `/vendor-requests` dan `/vendor/replenish-orders` (T2); `checkContract(t, req, rec)` → `routers/legacy` find route + `openapi3filter.ValidateResponse` (T3). Tabel kasus per operasi: sukses + error yang bisa dipicu fake (S7); 401/403 satu kali per prefix lewat `RequireAuth`/`RequireRoles`. T4 negatif: response sintetis dengan field tambahan + nominal string → validasi harus gagal. |
| 1.4 | `backend/go.mod`, `go.sum` | `go get github.com/getkin/kin-openapi@v0.151.0`. |
| 1.5 | fake di `vendor_request_handler_test.go` / `vendor_order_handler_test.go` | Hanya bila perlu fixture yang mengisi semua field (nullable terisi + kosong). Tidak mengubah assertion test lama. **Built:** file test lama tidak diubah; `fakeVendorRequestServicer`/`fakeVaultReviewer` dipakai ulang dengan fixture lengkap, ditambah `contractPartyReader` + `contractOrderSvc` lokal di file test kontrak (fake lama mengembalikan slice nil yang tidak pernah dikirim service asli). |

Urutan: 1.4 → 1.2 → 1.1 per kelompok operasi (list/get → create/items → transisi → completion → vault → vendor-side → vendor orders) → 1.3 setelah tiap kelompok (RED bila spec belum memuat operasi itu di T2, GREEN setelah ditambah).
Bukti: `cd backend && go test ./internal/handler/ -run OpenAPI -v`, `go test ./...`, `golangci-lint run --build-tags integration ./...`, `govulncheck ./...`.
Temuan inkonsistensi (bentuk error beda, field `omitempty` yang kadang hilang, dll.) → dicatat di `tests.md` "Temuan", spec mengikuti perilaku sekarang.

## Slice 2 — Tipe CompanyPortal di-generate (setelah FE-12 sisa ter-commit)

Gate: `git status` tidak lagi menunjukkan `VendorRequestCreate*.tsx` / `vendorRequestCreateForm.ts` belum ter-commit. Kalau masih → slice 2 menunggu, slice 3 boleh jalan.

| # | File | Isi |
|---|---|---|
| 2.1 | `frontend/CompanyPortal-Vite/package.json` + lockfile | devDependency `openapi-typescript@7.13.0`; script `"gen:api": "openapi-typescript ../../backend/api/openapi.yaml -o src/lib/api/schema.d.ts"`. |
| 2.2 | `src/lib/api/schema.d.ts` (generated, di-commit) | Header "generated — jangan edit". Biome ignore untuk file ini bila lint mengeluh. |
| 2.3 | `src/lib/api/schema.test.ts` (baru) | T5: API Node `openapiTS` + `astToString` dari YAML → sama persis dengan isi `schema.d.ts`; pesan gagal "jalankan pnpm run gen:api". |
| 2.4 | `src/features/vendor-request/types.ts` | Interface yang mencerminkan wire (mis. `VendorRequestDetail`, `VendorRequestSummary`, `ForecastRow`, `AuditLogEntry`, payload create/reject/cancel) → `export type X = components["schemas"]["X"]`. Nama export tetap. Tipe UI-only (`FetchAllForecastResult`, params, konstanta) tetap manual. Bila alias membuat tsc gagal di pemanggil = drift nyata → dicatat di `tests.md`, diperbaiki di sisi FE hanya bila spec (yang sudah lulus T3) benar. |
| 2.5 | `src/features/vendor-request/vendorSide.ts` | Tipe vendor parties / vault assignments → alias yang sama. |

Bukti: `pnpm --dir frontend/CompanyPortal-Vite run build|test|lint`, `test:coverage` gate tidak turun.

## Slice 3 — DoD + dokumentasi (S6)
| # | File | Isi |
|---|---|---|
| 3.1 | `.claude/CLAUDE.md` | Sec 2 baris "API contract"; Sec 2a perintah `gen:api` + `go test ./internal/handler -run OpenAPI`; Sec 11 butir DoD (teks FR8). Ditunjukkan sebagai diff ke user sebelum disimpan. |
| 3.2 | `.claude/BE_debt.md` BE-10, `sdlc/README.md`, `development-progress.md` | Status. |
| 3.3 | `graphify update .` | Setelah kode berubah. |

## Risiko
- **Spec salah tapi lolos** karena fixture fake tidak mengisi field nullable → T3 tidak pernah melihat bentuk aslinya. Mitigasi: fixture per operasi mengisi semua field (dua varian: null + terisi) untuk tipe utama.
- **Encoder Go berbeda dari asumsi** (`omitempty`, `map[string]int64` → object, slice nil → `null` bukan `[]`). Mitigasi: spec ditulis dari struct tag, lalu T3 yang memutuskan; slice nil didokumentasikan `nullable: true` bila memang bisa `null`.
- **kin-openapi `routers/legacy`** mencocokkan path dengan prefix server; test memakai URL penuh `/api/v1/...`.
- **Bentrok dengan session lain** di `features/vendor-request/` → gate slice 2.
- Spec besar (26 operasi) → dikerjakan per kelompok, commit hanya bila user bilang "commit".

## Alternatif yang tidak diambil
- Generate spec dari kode (swag) — Q2.
- Validasi lewat test integration (DB nyata) — lebih lambat, butuh `DATABASE_URL`; fake sudah cukup untuk bentuk response.
- Validator schema buatan sendiri — Q4.

## Commit
Tidak ada commit sampai user bilang "commit". Per slice, stage hanya file slice itu + hunk BE-10/README; jangan sentuh perubahan FE-12 milik session lain.
