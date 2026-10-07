# Implementation Plan: Replenishment Request Enhancements

## Overview

Rencana implementasi ini menerjemahkan `design.md` menjadi langkah-langkah koding inkremental, **backend-first lalu frontend**, persis mengikuti daftar "Files touched" pada design. Semua perubahan bersifat **additive** dan memakai ulang rantai yang sudah ada (`request-replenish-to-vendor` + `cit-vendor-request-enhancements`/CIT-2): `queries/vendor_request.sql → internal/db/*.sql.go → internal/service/vendor_request*.go → internal/handler/vendor_request_*.go → features/vendor-request/*`.

Prinsip yang dipegang (Golden Rules `project-context.md`): flat JSON additive-only (Req 6.1), maker-checker + `audit.Writer` tx-scoped (Req 2.5/3.6/6.3), money = `numeric`/decimal string (Req 5.10), RBAC dua lapis (middleware + service), dan migrasi additive forward-only yang **di-STOP-and-confirm lebih dulu** bila menyentuh skema (Golden Rule Sec 3 rule 7).

Bahasa implementasi mengikuti design apa adanya: **Go** untuk backend (`backend/`, flat JSON, port 8080) dan **TypeScript + React** untuk frontend (`frontend/CompanyPortal-Vite`, tema "Merah Sirih"). Tidak ada pemilihan bahasa baru — design memakai bahasa konkret, bukan pseudocode.

Property-based test memakai `pgregory.net/rapid` (sudah dipakai di `internal/service/atm_portal_property_test.go`), minimum 100 iterasi, satu Correctness Property → satu property test, dengan komentar `// Feature: replenishment-request-enhancements, Property {n}: {teks}`.

## Tasks

- [x] 1. Backend: perluas query Forecast (SQL) untuk PriorityClass, Paket (LATERAL deterministik), Escrow (LATERAL)
  - Edit `queries/vendor_request.sql` — `ListForecastForDate`: tambah SELECT `a.priority_class AS priority_class` (dari join `atms` yang sudah ada); perluas LATERAL paket agar mengembalikan `vp.code AS paket` dengan pemilihan satu paket aktif deterministik (`is_active`, dalam rentang `effective_start/end_date` pada `forecast_date`, `ORDER BY avp.effective_start_date DESC, avp.id DESC LIMIT 1`); tambah LATERAL baru untuk `itm_replenish.escrow` (`ORDER BY cp.replenish_date DESC, cp.replenish_time DESC LIMIT 1`) mengikuti pola `atm_portal.sql` `ListATMsWithCashPos`.
  - Jangan ubah `WHERE`/`ORDER BY`/pagination pada `ListForecastForDate` sehingga row-count & identity `(terminal_id, periode_pred, denom)` tak berubah (Req 5.12).
  - Biarkan `CountForecastForDate` tak berubah (semua LATERAL `LIMIT 1`, tak memengaruhi count).
  - _Requirements: 5.2, 5.3, 5.4, 5.5, 5.12_
  - _Model: Sonnet, Effort: Medium — SQL LATERAL + tie-breaker deterministik dalam pola repo yang sudah ada; risiko terbatas pada satu query read._

- [x] 2. Backend: regenerate / hand-edit sqlc untuk row struct Forecast (sqlc caveat)
  - [x] 2.1 Perbarui `internal/db/vendor_request.sql.go` untuk mencerminkan kolom baru
    - Jalankan `sqlc generate` lalu **simpan HANYA diff `vendor_request*.sql.go`** (buang perubahan file generated lain yang tak terkait — sqlc caveat project-context), ATAU edit manual mengikuti konvensi output sqlc: `ListForecastForDateRow` bertambah `PriorityClass *string`, `Paket *string`, `Escrow pgtype.Numeric`.
    - Pastikan scan order kolom sesuai SELECT baru dari Task 1; tidak menyentuh row struct query lain.
    - _Requirements: 5.6, 6.1_
    - _Model: Sonnet, Effort: Medium — mekanis tapi butuh judgment untuk mengisolasi diff yang benar dari output sqlc yang lebih luas._

- [x] 3. Backend: service DTO ForecastRow + isian BrowseForecast untuk kolom baru
  - [x] 3.1 Tambah field pada `ForecastRow` di `internal/service/vendor_request.go`
    - Tambah `PriorityClass string`, `Paket string`, `Escrow *string` (decimal string; nil = tidak tersedia). Pertahankan `AmountRefund` (tetap ada di DTO & wire — Req 5.1 hanya menghapus render kolom, bukan field).
    - _Requirements: 5.2, 5.3, 5.5, 5.10_
    - _Model: Sonnet, Effort: Medium — perubahan struct + mapping numeric→string dalam pola yang ada._
  - [x] 3.2 Isi kolom baru di `BrowseForecast` (`internal/service/vendor_request_actions.go`)
    - Isi `PriorityClass: notesOrEmpty(r.PriorityClass)`, `Paket: notesOrEmpty(r.Paket)`, `Escrow: numericToDecimalStringPtrOrNil(r.Escrow)` (reuse `numericToDecimalStringPtr` dari `atm_portal_profile.go`; nil untuk NULL sehingga UI render `"-"`, bukan `"0.00"`).
    - _Requirements: 5.5, 5.10, 5.11_
    - _Model: Sonnet, Effort: Medium — reuse konverter numeric yang ada; hati-hati NULL→nil bukan 0._

- [x] 4. Backend: response Forecast additive (priority_class / paket / escrow)
  - Edit `internal/handler/vendor_request_response.go` — `forecastRowResponse`: tambah tiga field JSON **di belakang** field lama: `PriorityClass string \`json:"priority_class"\``, `Paket string \`json:"paket"\``, `Escrow *string \`json:"escrow"\`` (decimal string; null bila tak tersedia). Jangan menghapus `amount_refund` dari response.
  - _Requirements: 5.1, 5.6, 6.1_
  - _Model: Sonnet, Effort: Medium — penambahan field additive; jaga urutan/ nama untuk kompatibilitas wire._

- [ ] 5. Backend (Req 1): validasi kategori→tanggal untuk semua create + branching resolusi item berbasis kategori + persist & audit kategori
  - [x] 5.1 Pindahkan validasi kategori→tanggal keluar dari blok `IsManual` di `Create` (`internal/service/vendor_request_actions.go`)
    - Panggil `validateCategoryDateConsistency(in.RequestCategory, in.ReplenishDate)` untuk **semua** create (DMAA-backed maupun manual). Fungsi ini sudah menegakkan Planned→{H+1}, Emergency→{H+0}, Additional→{H+0,H+1,H+2}, default-branch menolak kategori tak dikenal (Req 1.11) dan tanggal inkonsisten (Req 1.10) tanpa persist.
    - _Requirements: 1.3, 1.4, 1.5, 1.10, 1.11_
    - _Model: Opus, Effort: High — aturan tanggal per-kategori pada jalur create yang salah bisa mem-persist request dengan tanggal/kategori keliru; sentuh logika validasi inti._
  - [x] 5.2 Ganti branch resolusi item dari berbasis `IsManual` menjadi berbasis kategori
    - `emergency`/`additional` → `acceptItems(...)` (terima tanpa wajib match DMAA — Req 1.7, 1.8); selain itu (`planned` atau kategori kosong pada `UpdateItems`) → `resolveItems(...)` (wajib match DMAA, kembalikan `InvalidItemsError` menamai item non-matching — Req 1.6, tidak persist).
    - Untuk Emergency/Additional DMAA-backed, pertahankan `periode_pred` yang dikirim bila ada, jatuh ke `replenishDate` hanya bila kosong (hindari regresi identity `(terminal_id, periode_pred, denom)`).
    - Pastikan `UpdateItems` tetap memanggil `resolveItems` (kategori kosong → cabang `else`).
    - _Requirements: 1.6, 1.7, 1.8_
    - _Model: Opus, Effort: High — branching yang salah menerima/menolak item lintas tipe merusak jaminan DMAA-linkage; edge case periode_pred._
  - [x] 5.3 Persist `request_category` untuk semua create (bukan hanya manual) + audit selalu catat kategori
    - Ganti `categoryOrNil(in)` agar mem-persist `request_category` bila salah satu `{planned, emergency, additional}` di-set, terlepas dari `is_manual` (Req 1.9); CHECK `vendor_requests_category_chk` sebagai jaring kedua.
    - Perluas blok audit `Create` agar **selalu** mencatat `metadata["request_category"]`, `state=draft`, actor, timestamp UTC (dari `audit.Writer`) — sebelumnya hanya untuk manual (Req 1.13).
    - _Requirements: 1.9, 1.13_
    - _Model: Opus, Effort: High — persist kategori + audit tepat-satu adalah money/approval-adjacent; kesalahan merusak trail & data kategori._
  - [ ]* 5.4 Property test — Property 1: konsistensi kategori↔Replenish_Date (pure, rapid)
    - **Property 1: Konsistensi kategori terhadap Replenish_Date**
    - **Validates: Requirements 1.3, 1.4, 1.5, 1.9, 1.10, 1.11**
    - Generate kategori (termasuk string invalid) × tanggal; assert accept-iff-allowed relatif H-today Asia/Jakarta; tolak menamai field; tanpa DB.
    - _Model: Sonnet, Effort: Medium — property murni pada fungsi validasi._
  - [ ]* 5.5 Property test — Property 2: Planned menolak item non-DMAA (integration, rapid)
    - **Property 2: Planned menolak item tanpa padanan DMAA**
    - **Validates: Requirements 1.6**
    - Seed `dmaa_atm_forecast`; generate campuran match/non-match; assert reject + item non-matching dinamai + tidak persist (Postgres nyata dalam tx + rollback).
    - _Model: Sonnet, Effort: Medium — integration property pada resolveItems._
  - [ ]* 5.6 Property test — Property 3: Emergency/Additional menerima item apa pun (integration, rapid)
    - **Property 3: Emergency dan Additional menerima item apa pun**
    - **Validates: Requirements 1.7, 1.8**
    - Generate item apa pun; assert accept untuk emergency & additional (Postgres nyata dalam tx).
    - _Model: Sonnet, Effort: Medium — integration property._
  - [ ]* 5.7 Property test — Property 4: tepat satu audit pada create (integration, rapid)
    - **Property 4: Tepat satu entri audit pada create**
    - **Validates: Requirements 1.13**
    - Assert delta `audit_logs` == 1 dengan `action="create"`, memuat operator + `request_category` + timestamp UTC.
    - _Model: Sonnet, Effort: Medium — integration property pada audit delta._
  - [ ]* 5.8 Unit test — RBAC create + persist kategori (example/edge)
    - Non-maker `POST /` → 403 (middleware + service); kategori tersimpan sesuai input untuk jalur DMAA-backed & manual.
    - _Requirements: 1.12_
    - _Model: Sonnet, Effort: Medium — unit RBAC + persistence._

- [x] 6. Backend (Req 4): ubah format Request-ID menjadi ber-hyphen
  - Edit `createWithRetryingNumber` di `internal/service/vendor_request_actions.go`: ganti perakitan string dari `fmt.Sprintf("REP%s%s%03d", prefix, dateSeg, seq)` menjadi `fmt.Sprintf("REP-%s-%s-%03d", prefix, dateSeg, seq)`. Jangan ubah sumber `prefix` (`resolveVendorPrefix`/`fallbackVendorPrefix`), `dateSeg` (`ReplenishDate.Format("20060102")`), `seq` (`NextRequestNumberSeq`), retry ≤5× (`ErrNumberGeneration`), exhaustion (`ErrNumberExhausted`), atau concurrency (upsert `ON CONFLICT`).
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.9_
  - _Model: Opus, Effort: High — request-number danger zone; walau satu baris, harus menjaga keunikan/urutan/retry/exhaustion tetap utuh._
  - [x]* 6.1 Property test — Property 13: format Request_Number (pure, rapid)
    - **Property 13: Format Request_Number**
    - **Validates: Requirements 4.1, 4.2**
    - Assert regex `^REP-[A-Z]{3}-\d{8}-\d{3}$` pada nomor yang dirakit; tanpa DB.
    - _Model: Sonnet, Effort: Medium — property murni pada perakitan string._
  - [x]* 6.2 Property test — Property 14: urutan per-vendor-per-hari (integration, rapid)
    - **Property 14: Urutan Request_Number per-vendor-per-hari**
    - **Validates: Requirements 4.3**
    - N create dalam satu scope `(vendor, replenish_date)`; assert urut `001..N` menaik & distinct (Postgres nyata).
    - _Model: Sonnet, Effort: Medium — integration property pada sequence._
  - [x]* 6.3 Property test — Property 15: keunikan Request_Number (integration, rapid)
    - **Property 15: Keunikan Request_Number**
    - **Validates: Requirements 4.4, 4.7**
    - Banyak create lintas scope + konkuren; assert pairwise distinct & tersimpan di `request_number` (Postgres nyata).
    - _Model: Sonnet, Effort: Medium — integration property pada uniqueness/concurrency._
  - [x]* 6.4 Unit test — retry, exhaustion, legacy, search (example/edge)
    - Retry pada unique-violation ≤5× lalu `ErrNumberGeneration` (forced conflict); `last_seq=999` → `ErrNumberExhausted`; legacy `REP…` tanpa separator tidak ditulis ulang (Req 4.8); search list ber-hyphen full + partial cocok via `ILIKE` (Req 4.9).
    - _Requirements: 4.5, 4.6, 4.8, 4.9_
    - _Model: Sonnet, Effort: Medium — unit example/edge di sekitar generator._

- [x] 7. Backend (Req 3): perluas state machine + checkActor approved-cancel (Checker-only)
  - [x] 7.1 Tambah edge transisi & cabang `checkActor` di `internal/service/vendor_request.go`
    - Tambah `"approved": {actionCancel: "cancelled"}` pada map `transitions` (satu-satunya edge baru; `approved→processing`/`processing→completed` tetap di luar scope).
    - Perluas `checkActor` cabang `actionCancel`: `draft` → creator-only; `approved` → **Checker-only** (`!isChecker(actor.Role)` → `ErrNotChecker`, tanpa four-eyes karena ini pembatalan langsung, Checker-yang-creator tetap boleh); `pending_approval` → union creator-OR-non-creator-checker (perilaku lama tak berubah, Req 3.12).
    - Tambah sentinel `ErrCancelReasonEmpty`.
    - _Requirements: 3.1, 3.5, 3.7, 3.12, 6.4_
    - _Model: Opus, Effort: High — state-machine + authorization branch; transisi/otorisasi yang salah membuka pembatalan tak sah atas order approved._
  - [x]* 7.2 Property test — Property 8: himpunan transisi state machine (pure, rapid)
    - **Property 8: Himpunan transisi state machine**
    - **Validates: Requirements 2.6, 3.12, 6.4**
    - Seluruh `(status, action)`; assert allowed-set (termasuk `approved --cancel--> cancelled`) + illegal→state tetap; tanpa DB.
    - _Model: Sonnet, Effort: Medium — property murni pada nextState._
  - [x]* 7.3 Property test — Property 10: otorisasi cancel-approved hanya Checker (integration, rapid)
    - **Property 10: Otorisasi cancel-approved hanya Checker**
    - **Validates: Requirements 3.7**
    - Generate role; allow iff checker; role lain → error otorisasi + state tetap (Postgres nyata).
    - _Model: Sonnet, Effort: Medium — integration property pada checkActor._

- [x] 8. Backend (Req 3): Cancel(reason) dengan validasi + audit after.reason + handler body parse
  - [x] 8.1 Ubah `Cancel` menerima alasan di `internal/service/vendor_request_actions.go`
    - Ubah signature ke `Cancel(ctx, actor, id, reason string)` (atau `CancelWithReason`): validasi `reason` 1–500 non-whitespace (`strings.TrimSpace`; kosong → `ErrCancelReasonEmpty`; >500 → `*ValidationError{Field:"cancellation_reason"}`).
    - Pertahankan urutan guard yang ada: lock (`GetVendorRequestForUpdate`) → `is_canceled` guard (`ErrAlreadyCanceled`, Req 3.8 idempotensi tanpa audit duplikat) → `nextState` → `checkActor` → `SoftCancelVendorRequest` (set `status='cancelled'`+`is_canceled=true`, tidak hapus row/items, Req 3.4) → audit `action="cancel"` dalam satu tx, tanpa gate approval kedua (Req 3.5).
    - Tambahkan `after["reason"] = reason` pada audit entry cancel; `before` memuat status + `is_canceled=false` (Req 3.6).
    - _Requirements: 3.2, 3.3, 3.4, 3.5, 3.6, 3.8_
    - _Model: Opus, Effort: High — pembatalan order approved: idempotensi, preservasi baris, audit tepat-satu dengan reason; money/approval-critical._
  - [x] 8.2 Handler `Cancel` parse body `{cancellation_reason}` di `internal/handler/vendor_request_handler.go`
    - Berhenti memakai `doTransition` (no-body) untuk cancel; parse `cancellation_reason` dari body JSON (pola `rejectBody`) lalu panggil `Cancel(..., reason)`. Tambah mapping `ErrCancelReasonEmpty` → `writeValidationError` (seperti `ErrRejectReasonEmpty`).
    - _Requirements: 3.2, 3.3, 3.6_
    - _Model: Sonnet, Effort: Medium — parsing body + mapping error dalam pola handler yang ada._
  - [x]* 8.3 Property test — Property 5: validasi alasan (reject & cancel) (pure, rapid)
    - **Property 5: Validasi alasan (reject dan cancel)**
    - **Validates: Requirements 2.3, 2.4, 3.2, 3.3**
    - Generate string; accept iff `1 ≤ len(trim) ≤ 500`; bila ditolak aksi tak dikirim & state tetap; tanpa DB.
    - _Model: Sonnet, Effort: Medium — property murni pada validator alasan bersama._
  - [x]* 8.4 Property test — Property 9: cancel-approved preservasi baris + satu audit (integration, rapid)
    - **Property 9: Cancel-approved mempertahankan baris dan menulis satu audit**
    - **Validates: Requirements 3.4, 3.6**
    - Seed `approved` + N items; assert `is_canceled=true`, row+items tetap ada, tepat satu audit `action="cancel"` memuat actor/before/after/reason/IP/UTC.
    - _Model: Sonnet, Effort: Medium — integration property pada Cancel._
  - [x]* 8.5 Property test — Property 11: idempotensi cancel (integration, rapid)
    - **Property 11: Idempotensi cancel**
    - **Validates: Requirements 3.8**
    - Cancel dua kali; upaya kedua → conflict, `is_canceled` tetap true, tak ada audit cancel duplikat.
    - _Model: Sonnet, Effort: Medium — integration property idempotensi._
  - [x]* 8.6 Property test — Property 12: canceled dikecualikan dari active-flow reads (integration, rapid)
    - **Property 12: Request yang dibatalkan dikecualikan dari active-flow reads**
    - **Validates: Requirements 3.9**
    - Data campuran canceled; default exclude, `include_canceled=true` include.
    - _Model: Sonnet, Effort: Medium — integration property pada List/Count filter._
  - [x]* 8.7 Unit test — RBAC cancel-approved (example)
    - Non-checker `POST /{id}/cancel` pada `approved` → 403 (middleware + service); state tetap.
    - _Requirements: 3.7, 6.2_
    - _Model: Sonnet, Effort: Medium — unit RBAC._

- [x] 9. Backend (Req 2): verifikasi backend Reject tidak berubah (verification)
  - Konfirmasi via test + baca kode bahwa backend `Reject` sudah lengkap dan tidak butuh perubahan: `pending_approval → rejected`, four-eyes `ErrSelfApproval` → 403, `ErrInvalidTransition` → 409 (Req 2.6), `ErrNotChecker` → 403 (Req 2.8), gate route `POST /{id}/reject` = `vendorRequestCheckerRoles`. Tulis/lengkapi test yang membuktikan ini; tidak ada edit kode produksi backend untuk Req 2.
  - _Requirements: 2.5, 2.6, 2.7, 2.8_
  - [x]* 9.1 Property test — Property 6: reject pending→rejected dengan satu audit (integration, rapid)
    - **Property 6: Reject memindahkan pending ke rejected dengan satu audit**
    - **Validates: Requirements 2.5**
    - Seed `pending_approval`; reject oleh checker non-creator dengan alasan valid; assert `rejected` + reason tersimpan + tepat satu audit (actor/action/before/after/IP/UTC).
    - _Model: Sonnet, Effort: Medium — integration property pada Reject._
  - [x]* 9.2 Property test — Property 7: four-eyes creator tak boleh approve/reject (integration, rapid)
    - **Property 7: Four-eyes — creator tidak boleh approve/reject**
    - **Validates: Requirements 2.7, 6.3**
    - Actor=creator approve/reject → denied (`ErrSelfApproval`), state tetap.
    - _Model: Sonnet, Effort: Medium — integration property four-eyes._

- [ ] 10. Backend: kontrak wire & non-regresi (contract/example tests)
  - [ ]* 10.1 Contract test — additive-only forecast & vendor-request response
    - Golden/contract test: superset field, tak ada rename/removal, `amount_refund` tetap dikirim; `priority_class`/`paket`/`escrow` hadir; `is_canceled` hadir di list/detail.
    - _Requirements: 5.6, 6.1_
    - _Model: Sonnet, Effort: Medium — contract test kompatibilitas wire._
  - [ ]* 10.2 Unit test — escrow serialisasi & paket determinisme & money/topology
    - Escrow `numeric(20,2)` → decimal string (bukan JSON number), NULL → tidak di-substitute 0 (Req 5.10, 5.11); integration seed ATM >1 paket aktif → satu paket deterministik berulang (Req 5.4); routing read/write tak berubah (Req 6.5); money numeric + timestamptz UTC (Req 6.6).
    - _Requirements: 5.4, 5.10, 5.11, 6.5, 6.6_
    - _Model: Sonnet, Effort: Medium — unit + integration edge cases._

- [x] 11. Backend (Req 3.11): STOP & konfirmasi keputusan skema cancellation_reason SEBELUM migrasi 035
  - **STOP-and-confirm (Golden Rule Sec 3 rule 7).** Sebelum menulis migrasi apa pun, tanyakan & dapatkan konfirmasi user untuk paparan `cancellation_reason` di respons list/detail (Req 3.11). Design men-default ke **Opsi B** (migrasi additive nullable) tetapi mem-flag-nya untuk sign-off. Sajikan dua opsi dan tunggu keputusan:
    - **Opsi B (default design):** buat `migrations/035_vendor_requests_cancellation_reason.sql` — `ALTER TABLE vendor_requests ADD COLUMN IF NOT EXISTS cancellation_reason text;` (nullable, no default, **no backfill**, forward-only); `SoftCancelVendorRequest` di-extend menyimpan kolom; `forecastRowResponse`/detail/list response tambah `cancellation_reason *string` additive.
    - **Opsi A (fallback, lean):** tanpa migrasi — alasan hanya hidup di `audit_logs.after.reason`, dibaca via `GET /{id}/audit-log` yang sudah ada; detail page menampilkannya dari audit log. Tidak ada perubahan skema.
  - **Keputusan user (2026-09-15): Opsi B.** Diimplementasikan sebagai `migrations/038_vendor_requests_cancellation_reason.sql` (035-037 sudah dipakai task 1/6 untuk `request_prefix`/`request_number_seq`, jadi nomor bergeser ke 038 — additive/nullable/no-backfill/forward-only, sama seperti spesifikasi Opsi B). `SoftCancelVendorRequest` di-extend (`SoftCancelVendorRequestParams{ID, CancellationReason}`); `VendorRequestDetail`/`VendorRequestSummary` (service) dan `vendorRequestDetailResponse`/`vendorRequestSummaryResponse` (handler) dapat field `CancellationReason *string` additive. `forecastRowResponse` **tidak** disentuh — referensi task doc ke situ adalah salah ketik (kolom itu milik Forecast Browser, bukan vendor request cancellation).
  - _Requirements: 3.11, 6.7_
  - _Model: Opus, Effort: High — perubahan skema pada tabel money/approval; harus di-STOP-and-confirm dan additive/forward-only tanpa backfill._

- [x] 12. Checkpoint — backend build & tests
  - Ensure all tests pass, ask the user if questions arise. Jalankan `go build ./...`, `go test ./...`, `golangci-lint run` pada `backend/`; triase kegagalan sebelum lanjut ke frontend.
  - **Hasil:** `go build ./...`, `go vet ./...` / `-tags integration`, dan `go test ./...` bersih. Migrasi 038 diterapkan ke dev DB lokal, lalu seluruh suite `-tags integration` dijalankan sungguhan (real Postgres): semua test `vendor_request`/`Cancel`/`RequestNumber`/`Reject` lulus — termasuk yang menemukan **bug nyata**: retry request-number (Task 6) mem-batalkan seluruh tx terluar pada unique-violation, membuat retry kedua selalu gagal dengan Postgres 25P02 alih-alih benar-benar retry. Diperbaiki dengan membungkus tiap attempt dalam SAVEPOINT (pgx nested tx via `tx.Begin`) di `createWithRetryingNumber`/`attemptCreateWithNumber` (`vendor_request_actions.go`). Juga memperbaiki assertion test sendiri yang salah asumsi format JSON compact (jsonb Postgres punya spasi setelah `:`). `golangci-lint run`: nol temuan baru di file yang disentuh task ini; menghapus dead code `makerRoles`/`isMaker` (tak pernah dipanggil, sudah ada sebelum sesi ini) di `vendor_request.go`. 4 test auth (`TestIntegration_RateLimitEnforcement` dkk.) gagal karena state rate-limit Redis tersisa dari run integration berulang — pre-existing, di luar scope package ini, tidak disentuh.
  - _Model: Sonnet, Effort: Medium — verification gate; butuh judgment untuk triase, bukan reasoning Opus._

- [x] 13. Frontend: types.ts additions (ForecastRow + Cancel payload)
  - Edit `features/vendor-request/types.ts`: `ForecastRow` tambah `priority_class: string`, `paket: string`, `escrow: string | null`; pastikan `CreateVendorRequestPayload` punya `request_category` (dipakai jalur DMAA-backed); tambah `CancelVendorRequestPayload { cancellation_reason: string }`. (Jika Opsi B pada Task 11 dipilih, tambahkan juga `cancellation_reason: string | null` pada tipe detail/list.)
  - _Requirements: 1.9, 3.11, 5.6_
  - _Model: Haiku, Effort: Low — penambahan tipe mekanis, satu obvious path._

- [ ] 14. Frontend: api.ts & hooks.ts cancel-with-reason
  - [x] 14.1 `api.ts`: `cancelVendorRequest(id, reason)` mengirim body `{ cancellation_reason }`
    - Ubah dari call no-body menjadi kirim body JSON additive; forecast query tak berubah.
    - _Requirements: 3.2, 3.11_
    - _Model: Sonnet, Effort: Medium — perubahan client API dalam pola yang ada._
  - [x] 14.2 `hooks.ts`: `useCancelVendorRequest` menerima `{ id, reason }`
    - Teruskan reason ke `cancelVendorRequest`; jaga invalidation/refetch yang ada.
    - _Requirements: 3.2_
    - _Model: Sonnet, Effort: Medium — mutation hook dalam pola TanStack Query yang ada._

- [x] 15. Frontend (Req 1): kontrol Request_Type pada jalur DMAA-backed di VendorRequestCreate
  - Edit `VendorRequestCreate.tsx`: tampilkan kontrol Request_Type (Planned/Emergency/Additional) juga pada jalur DMAA-backed (bukan hanya manual). Initial state "belum dipilih" (`category: VendorRequestCategory | "" = ""`) agar Req 1.2 terpicu; wajib dipilih sebelum submit (Req 1.1), kosong → blok submit + error field (Req 1.2). Set Replenish_Date otomatis: Planned → `jakartaCalendarDateISO(1)` (H+1), Emergency → `jakartaCalendarDateISO(0)` (H+0); Additional → dropdown terbatas persis `{H+0,H+1,H+2}` via `additionalDateOptions` (Req 1.5), tanpa nilai lain. `toPayload` mengirim `request_category` terpilih untuk jalur DMAA-backed; `is_manual` tetap `false`. Token Merah Sirih + `min-h-[44px] min-w-[44px]` pada setiap kontrol (Req 1.14).
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.14_
  - _Model: Sonnet, Effort: Medium — kontrol form + aturan tanggal per-tipe reuse helper Asia/Jakarta yang ada._
  - [ ]* 15.1 Component test — Request_Type wajib + aturan tanggal + touch target
    - Kontrol wajib pada jalur DMAA-backed; submit tanpa tipe diblok + error field (Req 1.1, 1.2); Planned→H+1, Emergency→H+0, Additional dropdown {H+0,H+1,H+2} (Req 1.3–1.5); touch target 44×44 (Req 1.14); detail menampilkan kategori sebagai teks (Req 1.15).
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.14, 1.15_
    - _Model: Sonnet, Effort: Medium — component test Vitest + Testing Library._

- [x] 16. Frontend (Req 2 & 3): VendorRequestDetail reject affordance + approved-cancel control + reason modal
  - [x] 16.1 Reject: `variant="danger"` + ikon + teks (Req 2)
    - Render tombol Tolak dengan `variant="danger"` + ikon (`Ban`/`XCircle` lucide-react) + teks "Tolak" di samping Approve untuk `pending_approval ∧ isChecker ∧ !isCreator` (gate `canApproveReject` yang ada sudah benar). Pertahankan `RejectModal` (alasan 1–500 non-whitespace, konfirmasi disabled saat invalid); pastikan pesan error menyebut alasan tidak valid (Req 2.3, 2.4). Tombol hanya render saat `pending_approval` (Req 2.9), creator tidak melihatnya (Req 2.7).
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.7, 2.9, 2.10_
    - _Model: Sonnet, Effort: Medium — affordance/rendering fix + danger pattern; logika gating sudah ada._
  - [x] 16.2 Cancel approved: control + modal alasan (Req 3)
    - Perluas `canCancel` agar mencakup `approved` untuk Checker: `((isCreator || (isChecker && !isCreator)) && (status==="draft" || status==="pending_approval")) || (isChecker && status==="approved")` (Req 3.1). Ganti call `cancelMutation` langsung dengan modal alasan (pola `RejectModal`, 1–500 non-whitespace, `variant="danger"`, ikon+teks) yang memanggil `useCancelVendorRequest({ id, reason })` (Req 3.2, 3.3). Pastikan `StatusBadge` "Dibatalkan" (ikon `Ban` + teks) tetap render saat `is_canceled` (Req 3.10). Jika Opsi B pada Task 11: tampilkan `cancellation_reason` dari payload; jika Opsi A: dari audit-log endpoint.
    - _Requirements: 3.1, 3.2, 3.3, 3.10, 3.11_
    - _Model: Sonnet, Effort: Medium — visibility gate + reason modal dalam pola yang ada._
  - [ ]* 16.3 Component test — Reject & Cancel visibility + modal + badge
    - Table-driven `(status × role × isCreator)` untuk visibilitas Reject: hanya `pending_approval ∧ checker ∧ ¬creator` (Req 2.1, 2.2, 2.9); tombol danger+ikon+teks (Req 2.10); modal alasan 1–500 (Req 2.3, 2.4). Table-driven `(status × role)` visibilitas Cancel pada `approved` untuk checker (Req 3.1); modal alasan (Req 3.2, 3.3); badge "Dibatalkan" ikon+teks (Req 3.10); field `is_canceled`(+`cancellation_reason`) additive di list/detail (Req 3.11).
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.9, 2.10, 3.1, 3.2, 3.3, 3.10, 3.11_
    - _Model: Sonnet, Effort: Medium — component test table-driven._

- [x] 17. Frontend (Req 5): ForecastTable — hapus Amount Refund, tambah PriorityClass/Paket/Escrow
  - Edit `ForecastTable.tsx`: hapus definisi kolom render `amount_refund` dari array `columns` (Req 5.1; field tetap ada di tipe/wire). Tambah tiga kolom: `PriorityClass` (`accessorKey:"priority_class"`, cell `getValue<string>() || "-"`, `enableSorting:false`, Req 5.2/5.7 hyphen polos bukan em-dash); `Paket` (`accessorKey:"paket"`, cell `|| "-"`, `enableSorting:false`, Req 5.3/5.8); `Escrow` (`accessorKey:"escrow"`, `null → "-"` Req 5.11, selain itu IDR right-aligned `tabular-nums` via `formatIDR`/`meta.align="right"` Req 5.9 — parse decimal string hanya untuk display, jangan float-round untuk komputasi). Pertahankan selection/sort/pagination/loading/error/empty dan identity `forecastRowId = terminal_id|periode_pred|denom` (Req 5.12, 5.13); `colSpan` menyesuaikan `columns.length` otomatis.
  - _Requirements: 5.1, 5.2, 5.3, 5.7, 5.8, 5.9, 5.11, 5.12, 5.13_
  - _Model: Sonnet, Effort: Medium — kolom TanStack Table + format IDR dalam pola yang ada._
  - [x]* 17.1 Component test — ForecastTable kolom (extend ForecastTable.test.tsx)
    - Tidak ada kolom "Amount Refund" (Req 5.1); PriorityClass/Paket/Escrow menampilkan nilai sumber (Req 5.2, 5.3, 5.5); null/empty → `"-"` bukan em-dash, escrow null → `"-"` bukan `0` (Req 5.7, 5.8, 5.11); escrow right-aligned `tabular-nums` IDR (Req 5.9); sort/selection/pagination/loading/error/empty tetap; identity `forecastRowId` tak berubah (Req 5.12, 5.13).
    - _Requirements: 5.1, 5.2, 5.3, 5.5, 5.7, 5.8, 5.9, 5.11, 5.12, 5.13_
    - _Model: Sonnet, Effort: Medium — extend component test yang ada._

- [x] 18. Final checkpoint — build + lint + test verification gate (backend & frontend)
  - Ensure all tests pass, ask the user if questions arise. Backend: `go build ./...`, `go test ./...`, `golangci-lint run` (`backend/`). Frontend: `pnpm build`, `pnpm test --run`, `pnpm lint` (`frontend/CompanyPortal-Vite/`). Verifikasi cakupan Req 6.8 (Request_Type + aturan tanggal + penolakan tanggal-inkonsisten; visibilitas Reject + transisi + penolakan creator; cancel-approved preservation/is_canceled/reason/audit/idempotensi/otorisasi; nomor ber-hyphen + urutan/keunikan/exhaustion; perubahan kolom Forecast). Tidak ada merge dengan test gagal/di-skip; target coverage ≥80% pada `internal/*`.
  - **Hasil:**
    - **Backend**: `go build ./...` bersih. `go vet ./...` bersih. `go test ./...` (non-integration) semua lulus. `go test -tags integration ./...` terhadap Postgres dev lokal nyata: `internal/service` (semua test `vendor_request`/`Cancel`/`Reject`/`RequestNumber`/`FourEyes`), `internal/approval`, `internal/audit`, `internal/repository` — semua PASS. 4 test di `internal/handler` (`TestIntegration_RateLimitEnforcement` dkk.) gagal karena state rate-limit Redis tersisa dari run berulang — pre-existing, paket `auth`, di luar scope fitur ini (dikonfirmasi ulang, sama seperti temuan Task 12). `golangci-lint run`: 9 temuan, semua pre-existing di file tak terkait (`cmd/api/main.go`, `dsr_upload*.go`, `atm_portal_cashpos.go`) — nol temuan baru di `vendor_request*`/`vendor_request_*`.
    - **Frontend**: `pnpm build` (`tsc -b && vite build`) gagal — tapi murni pada ~80 error TypeScript pre-existing di 7 file tak terkait (`ModuleCard.property.test.tsx`, `CitTracker.test.tsx`, dashboard/invoice/reconciliation property tests, `Replenishment.test.tsx`, `lib/api/config.ts`, `login.property.test.ts`, `contrast.property.test.ts`) — dikonfirmasi via `git status`/`git diff` tak tersentuh sesi ini; `npx tsc --build --force` yang di-grep ke `vendor-request` sudah bersih sepanjang Task 13-17. `pnpm test`: 606/629 lulus; 23 gagal murni di 5 file pre-existing tak terkait (Replenishment, dashboard attention-panel, CIT coupling, stubs) — `vendor-request` sendiri 60/60 lulus. `pnpm lint`: 2 error + 1 warning, semua pre-existing (termasuk `navigation.ts` yang sudah `M` di git status sebelum sesi ini mulai) — `npx biome check src/features/vendor-request` terpisah: 0 temuan.
    - **Req 6.8 checklist**: Request_Type + aturan tanggal + tolak tanggal-inkonsisten (Task 5, 15) ✓; visibilitas Reject + transisi + tolak creator (Task 9, 16.1) ✓; cancel-approved preservation/is_canceled/reason/audit/idempotensi/otorisasi (Task 7, 8, 16.2) ✓; nomor ber-hyphen + urutan/keunikan/exhaustion (Task 6) ✓; perubahan kolom Forecast (Task 1-4, 17) ✓.
    - **Kesimpulan**: fitur replenishment-request-enhancements (Task 1-17) siap merge — nol regresi baru, semua kegagalan yang ditemukan pre-existing & di luar scope (dikonfirmasi via git status/diff, bukan diasumsikan).
  - _Requirements: 6.8_
  - _Model: Sonnet, Effort: Medium — verification gate lintas stack; triase kegagalan._

## Notes

- Task yang ditandai `*` bersifat opsional (unit/property/component/contract test) dan bisa di-skip untuk MVP cepat, tetapi Req 6.8 mensyaratkan test lulus sebelum merge — jangan skip untuk jalur produksi.
- Setiap task merujuk sub-requirement granular untuk traceability, dan tiap property test secara eksplisit merujuk satu Correctness Property (1–15) dari design.
- Backend-first lalu frontend, persis mengikuti daftar "Files touched" pada design.
- **Task 11 (Req 3.11) di-STOP-and-confirm**: tidak menulis migrasi 035 sebelum user memilih Opsi B (migrasi additive) atau Opsi A (audit-log-only, tanpa migrasi). Task 13/14/16 menyesuaikan sumber `cancellation_reason` sesuai keputusan itu.
- Setiap tier/effort mengikuti steering `model-recommendation`: Opus/High untuk zona-bahaya money/approval/state-machine/request-number/skema (Task 5, 6, 7, 8, 11); Sonnet/Medium untuk handler/service/query/komponen/test normal; Haiku/Low untuk edit tipe mekanis (Task 13).
- sqlc caveat (Task 2): regenerate lalu simpan hanya diff `vendor_request*.sql.go`, atau edit manual mengikuti konvensi output sqlc.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "5.1", "7.1", "13", "4"] },
    { "id": 1, "tasks": ["2.1", "5.2", "3.1", "14.1", "15", "17"] },
    { "id": 2, "tasks": ["5.3", "3.2", "14.2", "16.1"] },
    { "id": 3, "tasks": ["6", "16.2"] },
    { "id": 4, "tasks": ["8.1"] },
    { "id": 5, "tasks": ["8.2"] },
    { "id": 6, "tasks": ["9", "11", "5.4", "5.5", "5.6", "5.7", "5.8", "6.1", "6.2", "6.3", "6.4", "7.2", "7.3", "8.3", "8.4", "8.5", "8.6", "8.7", "9.1", "9.2", "10.1", "10.2", "15.1", "16.3", "17.1"] }
  ]
}
```

> Catatan penjadwalan: task yang menulis file yang sama diserialkan ke wave berbeda untuk menghindari konflik. `internal/service/vendor_request_actions.go` disentuh oleh `5.1`→`5.2`→`5.3`→`3.2`→`6`→`8.1` (masing-masing wave menaik). `internal/service/vendor_request.go` disentuh oleh `7.1`(wave 0) lalu `3.1`(wave 1). `internal/db/vendor_request.sql.go` (`2.1`) menunggu `1`. `internal/handler/vendor_request_handler.go` (`8.2`) menunggu `8.1`. `internal/handler/vendor_request_response.go` (`4`) edit tunggal, aman di wave 0. Frontend `VendorRequestDetail.tsx` disentuh `16.1`(wave 2)→`16.2`(wave 3); `api.ts`(`14.1`)→`hooks.ts`(`14.2`) berurutan. Checkpoint (`12`, `18`) dan parent murni tidak masuk graph. Task `9` (verifikasi backend Reject, tanpa edit produksi) dan `11` (gated STOP-and-confirm) berada di wave akhir bersama seluruh test.
