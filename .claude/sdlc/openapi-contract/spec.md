# Spec: Kontrak OpenAPI BE ↔ FE (BE-10) — pilot Vendor Request

Status: **accepted 2026-10-19** (user: *"Terima spec openapi-contract, S1–S7 sesuai usulan, lanjut plan."*).
Stage: 2 Design selesai. Stage berikut: [`plan.md`](./plan.md). Dasar: [`intent.md`](./intent.md) (accepted 2026-10-19, Q1–Q7 = saran Claude).

## Flag Sec 4 #7

| Flag | Di mana | Penanganan |
|---|---|---|
| Auth | Spec mendokumentasikan `bearerAuth` + respons 401/403 dari `RequireAuth`/`RequireRoles` | Hanya dokumentasi + test; middleware tidak diubah. |
| Money | Field nominal pilot (`amount_replenish`, `total_amount`, `amounts`, …) | Didokumentasikan apa adanya: `integer`, `format: int64`, rupiah penuh. Tidak ada `number`. Tipe TS hasil generate = `number` (aman: 2^53 ≫ nominal IDR). Tidak ada perubahan tipe di wire. |
| Migration / delete / recon | — | Tidak ada. |

Tidak ada perubahan perilaku, response shape, status code, atau path. Ketidaksesuaian yang ditemukan saat menulis spec (mis. dua handler beda bentuk error) **dicatat** di `tests.md`, tidak diperbaiki di sini.

## Baseline (dicek 2026-10-19)
- `/api/v1/vendor-requests` (`handler/vendor_request_handler.go` `Routes()`): 21 operasi; 3 di antaranya hanya terpasang bila `h.vault != nil`, 2 bila `h.vendor != nil`. Mount di `cmd/api/main.go:476` di bawah `RequireAuth`; tiap route punya `RequireRoles`.
- `/api/v1/vendor/replenish-orders` (`handler/vendor_order_handler.go`): 5 operasi, VendorPortal (`features/orders/api.ts`).
- Error body seragam: `ErrorResponse{error, message, details?[{field, message}]}` (`handler/error_response.go`).
- Waktu: `formatTimestamp` = RFC3339 UTC; tanggal = `YYYY-MM-DD`.
- Nominal di pilot = `int64` (bukan string desimal QC F7).
- Handler test memakai fake servicer + `httptest` (`vendor_request_handler_test.go`, `vendor_order_handler_test.go`) — dipakai ulang untuk test kontrak.
- CompanyPortal `features/vendor-request/types.ts` 381 baris / 28 export, ditulis manual. Sisa FE-12 (`VendorRequestCreate*.tsx`, `vendorRequestCreateForm.ts`) **belum di-commit** (session lain).
- CI (`.github/workflows/ci.yml`): job `go`, `go-integration`, `sqlc`, `python`, `python-image`; **belum ada job frontend**. Jenkins direncanakan (`ci-jenkins/`, on hold).

## Functional requirements

**FR1 — Dokumen spec.** `backend/api/openapi.yaml`, OpenAPI **3.0.3** (S1), `info.version: v1`, `servers: [/api/v1]`. Berisi seluruh 26 operasi pilot: path, parameter (path/query), request body, setiap status yang dikembalikan handler (sukses + error), dan schema response. Komponen bersama: `ErrorResponse`, `FieldError`, `bearerAuth`. Nullable = `nullable: true`; waktu `format: date-time`; tanggal `format: date`; nominal `integer/int64`.

**FR2 — Schema ketat.** Schema object response memakai `additionalProperties: false` + `required` untuk field yang selalu ada (S2). Field baru di Go tanpa update spec → test gagal.

**FR3 — Spec valid.** Test Go memuat spec dengan kin-openapi loader dan `doc.Validate()` — spec rusak/ref hilang → gagal.

**FR4 — Cakupan route.** Test Go me-`chi.Walk` `VendorRequestHandler.Routes()` (dengan `vault` + `vendor` terpasang) dan `VendorOrderHandler.Routes()`; himpunan (method, path) harus **sama persis** dengan operasi di spec untuk prefix itu. Route baru tanpa spec, atau spec tanpa route → gagal.

**FR5 — Response cocok spec.** Untuk setiap operasi pilot minimal satu kasus sukses, plus setiap error yang bisa dipicu lewat fake servicer (400/404/409/422 sesuai handler): request lewat `httptest` ke handler → `openapi3filter.ValidateResponse` terhadap spec (router `routers/legacy`, tanpa dependency gorilla/mux — S3). Respons 401/403 dari middleware cukup didokumentasikan; dites satu kali per prefix (S7).

**FR6 — Tipe CompanyPortal di-generate.** `openapi-typescript` (devDependency, versi di-pin) menghasilkan `src/lib/api/schema.d.ts` (di-commit). Script `pnpm run gen:api`. Tipe request/response di `features/vendor-request/types.ts` menjadi alias `components["schemas"][...]`; tipe UI murni (state form, filter) tetap manual. Nama export tidak berubah → pemanggil tidak berubah. Dikerjakan **setelah** sisa FE-12 ter-commit (S5).

**FR7 — File generated tidak basi.** Test vitest memanggil API Node `openapi-typescript` terhadap `backend/api/openapi.yaml` dan membandingkan dengan `schema.d.ts` yang ter-commit (S4). Ikut jalan di `pnpm run test`, tanpa job CI baru.

**FR8 — DoD.** Usulan perubahan CLAUDE.md (S6): Sec 11 + "Endpoint baru/berubah di modul yang ada di `backend/api/openapi.yaml` wajib masuk spec dan lulus test kontrak; modul baru mulai dengan spec"; Sec 2a + perintah `pnpm --dir frontend/CompanyPortal-Vite run gen:api`; Sec 2 tabel stack + baris "API contract: OpenAPI 3.0.3, kin-openapi (test), openapi-typescript (CP dev)".

## NFR
- **NFR1** Dependency: hanya `github.com/getkin/kin-openapi` (diimpor hanya dari `_test.go`; tetap tercatat di `go.mod`, ikut `govulncheck`) dan `openapi-typescript` (devDependency CP). Versi di-pin.
- **NFR2** Test kontrak = unit test tanpa DB → jalan di job CI `go` (`go test -race ./...`) yang sudah ada.
- **NFR3** Test kontrak + route coverage < 5 s total.
- **NFR4** Lint (`golangci-lint`, `biome`) dan `pnpm run build` tetap bersih; coverage gate CP/BE tidak turun.

## Out of scope
- Modul selain dua prefix pilot (master data, ACM `vault-plan`, notifikasi, auth, EOD, dll.) — masuk spec saat disentuh (FR8).
- Tipe VendorPortal di-generate (Q3: menyusul bila pilot lancar).
- Menyajikan `/openapi.json`, Swagger UI (Q5).
- `backend-cit`/`backend-exq` + envelope `pkg/response` (Q7).
- Validasi request di runtime (middleware OpenAPI), perubahan bentuk response, perbaikan inkonsistensi yang ditemukan.
- Job CI frontend (urusan `fe-tech-debt` / `ci-jenkins`).

## Test ↔ requirement

| Test | Bukti | FR |
|---|---|---|
| T1 | Loader + `Validate()` spec lulus | FR1, FR3 |
| T2 | Route set `chi.Walk` == operasi spec (dua prefix) | FR4 |
| T3 | Per operasi: sukses + error yang bisa dipicu, `ValidateResponse` lulus | FR1, FR2, FR5 |
| T4 | Negatif: response dengan field tambahan / field uang string → validasi gagal (membuktikan FR2 ketat) | FR2 |
| T5 | `schema.d.ts` fresh (vitest) | FR7 |
| T6 | `pnpm run build` (tsc) + test + lint CP hijau setelah alias | FR6 |
| T7 | `go test ./...`, `golangci-lint`, `govulncheck` hijau | NFR1–4 |
| — | Manual browser check alur Vendor Request (CP) + Orders (VP) — **outstanding (user)** | GR#10 |

## Keputusan untuk PO (usulan Claude)
- **S1 — Versi OpenAPI: 3.0.3.** Dukungan kin-openapi untuk 3.0 matang; 3.1 masih parsial. openapi-typescript mendukung keduanya. Alternatif 3.1 (`type: [x, "null"]`) — risiko validator.
- **S2 — Schema response ketat (`additionalProperties: false`).** Menangkap field yang lupa didokumentasikan. Tradeoff: menambah field = wajib update spec (memang itu tujuannya).
- **S3 — Router `routers/legacy` kin-openapi.** Tanpa menarik `gorilla/mux`. Alternatif `gorillamux` — dependency transitif tambahan.
- **S4 — Cek kesegaran `schema.d.ts` lewat test vitest**, bukan job CI (belum ada job FE). Pindah ke CI saat job FE/Jenkins ada.
- **S5 — Urutan slice**: (1) spec + test Go (BE saja, bisa mulai sekarang); (2) generate tipe CP setelah FE-12 ter-commit; (3) perubahan CLAUDE.md. Tiap slice commit sendiri.
- **S6 — Teks DoD/Sec 2/2a** seperti FR8 (diterapkan di slice 3, direview seperti kode).
- **S7 — Cakupan error di FR5**: hanya status yang bisa dipicu lewat fake servicer di handler; 401/403 middleware satu kasus per prefix. Alternatif: semua kombinasi role × endpoint — mahal, sudah dites di test RBAC yang ada.
