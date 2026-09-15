# Implementation Plan: CIT Vendor Request Enhancements ("CIT 2")

## Overview

This plan implements the five CIT-2 enhancements on the existing CIT vendor-request slice, backend-first then frontend, following the design's "Files touched" list and per-requirement breakdown. It reuses the existing chain: `backend/queries/vendor_request.sql` → `internal/service/vendor_request*.go` → `internal/handler/vendor_request_*.go` → `frontend/CompanyPortal-Vite/src/features/vendor-request/`.

Task 1 is a STOP-and-confirm gate: migrations 034–037 are authored, then require explicit user sign-off before being applied to the external, non-dockerized Postgres (Golden Rule 7). Three design decisions are flagged inside that gate for confirm-before-implementing: the `ABC` prefix for ABACUS (Q1), the one-vendor-per-request constraint (Q2), and `replenish_date` driving the number's date segment (Q5).

Languages are fixed by the design: **Go** (ATM backend) and **TypeScript/React** (CompanyPortal-Vite). No language selection needed.

## Tasks

- [ ] 1. Author additive migrations 034–037 and STOP for confirmation before applying
  - _Model: Opus, Effort: High — schema + vendor-reference DDL that the request number and soft-cancel depend on; touches live external Postgres, so a wrong column/constraint/seed is expensive and must be confirmed (Golden Rule 7)._
  - [x] 1.1 Write `backend/migrations/034_vendor_requests_cit2_columns.sql`
    - Additive `ALTER TABLE public.vendor_requests`: `is_canceled boolean NOT NULL DEFAULT false`, `request_category text` (nullable) with `vendor_requests_category_chk` CHECK admitting NULL or `planned|emergency|additional`, `replenish_date date` (nullable), `is_manual boolean NOT NULL DEFAULT false`
    - Additive `ALTER TABLE public.vendor_request_items`: `brand text` (nullable), `lokasi_atm text` (nullable) — manual-only persistence per Q4
    - Add `vendor_requests_is_canceled_idx` and `vendor_requests_replenish_date_idx`
    - Use `IF NOT EXISTS` on all adds; forward-only, no data migration of existing rows
    - _Requirements: 3.9, 5.1, 2.5, 6.6_
  - [x] 1.2 Write `backend/migrations/035_vendors_request_prefix.sql`
    - Additive `vendors.request_prefix char(3)` (nullable) with `vendors_request_prefix_chk` CHECK `request_prefix IS NULL OR request_prefix ~ '^[A-Z]{3}$'`
    - _Requirements: 4.2, 4.3, 6.6_
  - [x] 1.3 Write `backend/migrations/036_seed_vendor_request_prefix.sql`
    - Idempotent `UPDATE public.vendors SET request_prefix = ...` for the six seeded codes: `TAG`→`TAG`, `ADVANTAGE`→`ADV`, `BIJAK`→`BJK`, `ABACUS`→`ABC` (originally proposed), `ROH`→`ROH`, `SSI`→`SSI`
    - Flag the ABACUS value for ABACUS as a confirm-before-seeding item (Q1) — auditor-facing identifier
    - **Update (2026-09-14, at apply time):** re-asked the user for sign-off on the exact ABACUS value before applying; user chose `ABA` over the originally-authored `ABC`. Migration 036, `design.md`, and `vendor_request_prefix_test.go`'s comments were all updated to `ABA` before applying (the fallback function `fallbackVendorPrefix("ABACUS")` already independently produces `ABA`, so seed and fallback now agree)
    - _Requirements: 4.2, 4.3, 6.6_
  - [x] 1.4 Write `backend/migrations/037_vendor_request_number_seq.sql`
    - Create `public.vendor_request_number_seq (vendor_id bigint, seq_date date, last_seq int NOT NULL DEFAULT 0)`, PK `(vendor_id, seq_date)`, CHECK `last_seq BETWEEN 0 AND 999`, FK `vendor_id → vendors(id)` ON DELETE RESTRICT
    - Additive `vendor_requests.vendor_id bigint` (nullable) with FK `→ vendors(id)` ON DELETE RESTRICT and `vendor_requests_vendor_id_idx`
    - _Requirements: 4.5, 4.7, 6.6_
  - [x] 1.5 STOP and obtain explicit user confirmation before applying migrations 034–037
    - Presented the four migrations and the three flagged decisions for sign-off: `ABC` prefix for ABACUS (Q1), one-vendor-per-request constraint (Q2), `replenish_date` drives the number date segment (Q5)
    - User confirmed: write only, do NOT run `migrate ... up` against the external Postgres yet (Golden Rule 7) — migrations remain unapplied
    - _Requirements: 6.6_
  - [x] 1.6 Apply migrations 034–037 (user requested "lanjut apply migrations 034-037", 2026-09-14)
    - Target: `backend/.env`'s `DATABASE_URL` (`host.docker.internal:5432/cms`, resolved to `localhost` from the host shell) — confirmed via `psql` as the developer's own local Postgres 18 instance, not a shared/remote server
    - Re-confirmed the ABACUS prefix design choice before applying (see 1.3 update) — user picked `ABA` over the originally-authored `ABC`
    - Verified pre-state: none of the CIT-2 columns existed yet on `vendor_requests`/`vendor_request_items`/`vendors`, and the six vendor `code` values matched migration 036's expectations exactly
    - Applied all four migrations via `psql "$DATABASE_URL" -f migrations/03N_*.sql` in order inside their own `BEGIN`/`COMMIT` blocks; verified post-state (columns/constraints/indexes/table exist, `vendors.request_prefix` seeded correctly for all six vendors)
    - _Requirements: 6.6_

- [ ] 2. Extend SQL queries and regenerate sqlc (keep only vendor_request diff)
  - _Model: Opus, Effort: High — the atomic `NextRequestNumberSeq` upsert underpins request-number uniqueness/concurrency, and Count now needs joins that affect pagination correctness; a subtle SQL error corrupts numbering or filtering._
  - [x] 2.1 Add filter args to `ListForecastForDate` and `CountForecastForDate` in `backend/queries/vendor_request.sql`
    - Add `brand`, `flm_vendor`, `flm_vendor_region` args using empty-sentinel + `LOWER(...) = LOWER(...)` case-insensitive exact match on `a.brand`, `v.name`, `vb.region`
    - Add the same join chain (`atms → atm_vendor_packages active → vendor_packages → vendor_branches → vendors`) to `CountForecastForDate` so it can filter on the joined columns while still yielding ≤1 row per forecast row
    - _Requirements: 1.6, 1.7, 1.8, 1.9, 1.10_
  - [x] 2.2 Add `NextRequestNumberSeq :one` atomic upsert query
    - `INSERT ... VALUES (vendor_id, seq_date, 1) ON CONFLICT (vendor_id, seq_date) DO UPDATE SET last_seq = last_seq + 1 RETURNING last_seq`
    - _Requirements: 4.5, 4.6, 4.7_
  - [x] 2.3 Extend `CreateVendorRequest` and `InsertVendorRequestItem`
    - `CreateVendorRequest` inserts `replenish_date`, `request_category`, `is_manual`, `vendor_id`
    - `InsertVendorRequestItem` inserts nullable `brand`, `lokasi_atm` (manual-only)
    - _Requirements: 2.5, 3.9, 4.2_
  - [x] 2.4 Add `SoftCancelVendorRequest :one` and `include_canceled` filter
    - `SoftCancelVendorRequest` sets `status = 'cancelled', is_canceled = true WHERE id = ... RETURNING *`
    - Add `include_canceled` arg to list/count queries: `AND (include_canceled = true OR vr.is_canceled = false)`
    - Deferred: `MaxRequestNumberSeqForDate` kept (not retired) until Task 6.1 actually replaces its caller (`createWithRetryingNumber`) — removing it now, before the generator switch, would break the build for no benefit in the interim
    - _Requirements: 5.2, 5.5_
  - [x] 2.5 Regenerate sqlc and keep only the vendor_request diff
    - Ran `sqlc generate` (v1.31.1); reverted the incidental rewrite of `approval.sql.go`/`audit.sql.go`/`audit_log.sql.go`/`auth.sql.go` (pre-existing unrelated schema drift, per the sqlc caveat) via `git checkout --`; hand-edited `models.go` to add only the CIT-2 fields (`Vendor.RequestPrefix`, `VendorRequest.{IsCanceled,RequestCategory,ReplenishDate,IsManual,VendorID}`, `VendorRequestItem.{Brand,LokasiAtm}`, new `VendorRequestNumberSeq`) instead of keeping the full regenerated file; kept the regenerated `vendor_request.sql.go` as-is (matches hand-edited `models.go`)
    - Verified: `go build ./...`, `go vet ./...`, `go test ./internal/service/...` all pass
    - _Requirements: 6.1_

- [ ] 3. Backend service — filters, prefix/fallback, sentinels, repo interface
  - _Model: Opus, Effort: High — introduces the vendor-prefix resolution + deterministic fallback that feeds the auditor-facing request number and the one-vendor-per-request validation; getting the fallback or vendor resolution wrong yields malformed identifiers._
  - [x] 3.1 Extend `BrowseForecastParams` and repo interface in `backend/internal/service/vendor_request.go`
    - Added `Brand`, `FLMVendor`, `FLMVendorRegion string` to `BrowseForecastParams`
    - Added `NextRequestNumberSeq` and `SoftCancelVendorRequest` to the repo interface; removed `MaxRequestNumberSeqForDate` from the interface (kept on `*db.Queries`/SQL per Task 2.4's deferral — `createWithRetryingNumber` still calls it concretely until Task 6.1)
    - Added `ErrAlreadyCanceled` sentinel
    - Also added a small new query `GetVendorForRequestNumber` (vendors.id/code/request_prefix) needed by 3.2 — not put on the interface since the generator uses the tx-scoped `*db.Queries` concretely, same as `ForecastRowExists`/`resolveItems`
    - _Requirements: 1.6, 4.5, 5.7_
  - [x] 3.2 Add vendor-prefix resolver + deterministic fallback helper
    - `resolveVendorPrefix`/`fallbackVendorPrefix` in `vendor_request_actions.go`: query `vendors.request_prefix`; when NULL, apply the Q1 fallback: uppercase `code`, strip non-`[A-Z0-9]`, take first 3 or right-pad with `X` to exactly 3 chars — never empty/truncated/non-3-char
    - _Requirements: 4.2, 4.3_
  - [x]* 3.3 Unit-test the prefix resolver + fallback
    - `vendor_request_prefix_test.go`, table-driven: seeded-length codes, 1/2-char codes pad with X, lowercase/non-alnum input, empty input — all assert exactly 3 uppercase chars. Note: `ABACUS`'s actual prefix is the seeded `'ABC'` (migration 036, a deliberate STOP-and-confirm value), not the fallback's own first-3-chars result (`'ABA'`) — the fallback only runs when no seed exists, so the test checks the fallback function's own output, not the seeded value
    - _Requirements: 4.2, 4.3, 6.7_

- [ ] 4. Backend service — BrowseForecast filter validation
  - _Model: Sonnet, Effort: Medium — standard request validation wiring on top of the new params._
  - [x] 4.1 Validate filters in `BrowseForecast` (`vendor_request_actions.go`)
    - Required non-empty `FLMVendor` and `FLMVendorRegion` → `ValidationError` (server backstop for block-fetch); length-bound all three to ≤255 chars → `ValidationError`; pass all three through to `ListForecastForDate`/`CountForecastForDate`
    - Added `TestBrowseForecast_ValidatesCIT2Filters` (unit, table-driven, no DB) covering the rejection cases and the all-valid pass-through case; fixed the pre-existing `TestBrowseForecast_MapsNewContextFields` to supply the now-required `FLMVendor`/`FLMVendorRegion`
    - _Requirements: 1.4, 1.5, 1.14_
  - [x]* 4.2 Integration test forecast filters (real Postgres, `//go:build integration`)
    - `vendor_request_cit2_filter_integration_test.go`, reusing `forecast_query_integration_test.go`'s seeding harness: seeds rows across two vendors sharing a brand/region; asserts combined AND filtering (1.10), empty-Brand pass-through unchanged (1.9, 1.16), case-insensitive match (1.6–1.8), Count matches List under filters. Empty flm_vendor/region and >255-char rejection are unit-tested in 4.1 (service-layer validation, no DB needed) rather than duplicated here
    - Verified: skips cleanly via `t.Skip` when `DATABASE_URL` is unset (confirmed in this environment); compiles under `go build -tags=integration ./...`
    - _Requirements: 6.7_

- [ ] 5. Backend service — replenish_date, category/date rules, manual accept vs Planned validate
  - _Model: Opus, Effort: High — category-to-date consistency and manual-vs-DMAA branching are the core new business logic; a wrong branch persists cash requests on the wrong day or bypasses DMAA validation._
  - [x] 5.1 Extend `CreateVendorRequestInput` and persist replenish_date
    - Added `ReplenishDate time.Time`, `RequestCategory string`, `IsManual bool`, `VendorID int64` to `CreateVendorRequestInput`; added `Brand`/`LokasiATM` to `ItemInput` (Q4 manual-item context)
    - `validateReplenishDate` (new): required + not earlier than today Asia/Jakarta (2.6, 2.7) — string-format parsing already happens in the not-yet-updated handler, same as `ForecastDate`; persists exactly as selected via `createWithRetryingNumber`'s now-extended `CreateVendorRequestParams` (2.5)
    - Added `wibZone`/`jakartaCalendarDate` to `vendor_request.go`, reusing the project's existing fixed-offset-not-`time.LoadLocation` convention from `handler.wibZone` (distroless image has no IANA tzdata)
    - _Requirements: 2.5, 2.6, 2.7_
  - [x] 5.2 Implement category + date-consistency rules in `Create`, scoped to `is_manual == true` only
    - `validateCategoryDateConsistency` (new), called only when `in.IsManual`: `request_category ∈ {planned,emergency,additional}` (3.11); Asia/Jakarta date rules planned=H+1, emergency=H+0, additional∈{H+0,H+1,H+2} (3.5, 3.10). Zero items is already `ErrEmptyItems` via the pre-existing `validateItemsPayload` call, unconditionally (3.12)
    - When `is_manual == false`: none of the above run; `categoryOrNil` forces `request_category` to NULL in the DB write regardless of any stray input value
    - _Requirements: 3.5, 3.10, 3.11, 3.12_
  - [x] 5.3 Implement manual-item acceptance vs Planned DMAA validation + one-vendor validation
    - `Create` branches: `is_manual && category ∈ {emergency,additional}` → new `acceptManualItems` (validates terminal_id 1–64 chars, denom > 0, amount_replenish > 0 — 3.6, 3.14; sets `amount_refund = 0`; anchors `periode_pred` to `replenish_date` since manual items have no DMAA periode; passes operator `brand`/`lokasi_atm` through to `InsertVendorRequestItem` — 3.7). Everything else (non-manual, or manual+planned) → unchanged `resolveItems` (3.8)
    - New `validateItemsSingleVendor` + new query `GetActiveVendorForTerminal` (mirrors `ListForecastForDate`'s active-package LATERAL join for one terminal): every resolved item's ATM must resolve to the request's `vendor_id` as of its own `periode_pred`; no resolvable active vendor also counts as a mismatch (Q2 defence-in-depth)
    - `request_category`/`is_manual` persisted on the header; exactly one `create` audit entry now also records `is_manual` always and `request_category` when manual (3.13)
    - _Requirements: 3.6, 3.7, 3.8, 3.13, 3.14_
  - [x]* 5.4 Unit-test replenish_date + category rules
    - `vendor_request_cit2_rules_test.go`: `TestValidateReplenishDate` (missing/past/today/future) and `TestValidateCategoryDateConsistency` (planned/emergency/additional boundaries + invalid category), both pure-function table-driven tests, no DB
    - Note: `Create()` itself can't be unit-tested against a mock repo — like the base spec's `resolveItems`, it opens a real `pgx.Tx` and calls `db.New(tx)` (a concrete type), a pattern this codebase only ever exercises via integration tests (see `forecast_query_integration_test.go`), never mocks. Extracting the new business rules into pure functions (`validateReplenishDate`, `validateCategoryDateConsistency`, `acceptManualItems`) makes them unit-testable directly, which is the practical equivalent within this codebase's existing conventions
    - _Requirements: 6.7_
  - [x]* 5.5 Unit-test manual accept vs Planned validate
    - `TestAcceptManualItems` (same file): valid manual item accepted with `amount_refund=0` and `brand`/`lokasi_atm` passed through; empty/over-64-char terminal_id, non-positive denom, non-positive amount all rejected with the right `ValidationError.Field`
    - Not covered here (pre-existing gap, unchanged by CIT-2, would need a real-Postgres integration test per the note above): Planned-category unmatched-item → `InvalidItemsError` (this is the base spec's `resolveItems`, which already had zero test coverage before this spec) and the one-audit-entry assertion (needs a real tx)
    - _Requirements: 6.7_

- [ ] 6. Backend service — new request-number generator
  - _Model: Opus, Effort: High — the 17-char generator with atomic per-scope sequence, bounded retry, and exhaustion handling is uniqueness/concurrency-critical; a race or off-by-one collides auditor-facing numbers._
  - [x] 6.1 Replace `createWithRetryingNumber` body
    - Resolves `vendor_id` + prefix via `resolveVendorPrefix` (Task 3.2); date segment from `replenish_date` (Q5) → `YYYYMMDD`; `NextRequestNumberSeq(vendor_id, replenish_date)` → `seq`; the `vendor_request_number_seq_last_chk` CHECK violation is detected via new `isSequenceExhausted` (Postgres code `23514` + constraint name) and mapped to `ErrNumberExhausted`, plus a defensive `if seq > 999` guard per design.md (4.7)
    - Formats `fmt.Sprintf("REP%s%s%03d", prefix, dateSeg, seq)` (fixed 17 chars, 4.1); `maxAttempts` raised 3→5 (4.6); on `vendor_requests_number_uq` violation retries, re-running `NextRequestNumberSeq` each time, then `ErrNumberGeneration`; never rewrites legacy `VR-…` (4.9), uniqueness constraint untouched (4.8)
    - Retired `MaxRequestNumberSeqForDate`: removed from `queries/vendor_request.sql`, regenerated (only the `vendor_request.sql.go`/`models.go` diff kept, same drift-revert dance as Tasks 2/3/5) — confirmed zero remaining code references
    - _Requirements: 4.1, 4.4, 4.5, 4.6, 4.7, 4.8, 4.9_
  - [x]* 6.2 Integration-test sequence scoping/concurrency/exhaustion (real Postgres)
    - `vendor_request_number_integration_test.go`: unlike the read-only forecast integration tests (one rolled-back tx), this needs a genuine `pgxpool.Pool` and real commits to exercise true concurrency — seeds one ATM+vendor+active package (skips DMAA entirely by using manual Emergency creates, Req 3.7), explicit FK-safe cleanup via `t.Cleanup`
    - `TestIntegration_RequestNumber_PerVendorPerDayScoping`: same vendor+date scope increments 001→002, fixed 17-char format (4.5); `TestIntegration_RequestNumber_ConcurrentCreatesGetDistinctNumbers`: 10 goroutines racing one scope all get distinct numbers, no duplicates (4.6); `TestIntegration_RequestNumber_ExhaustionRejectsWithoutPersisting`: pre-seeds `last_seq=999`, next create → `ErrNumberExhausted`, `vendor_requests` row count unchanged (4.7)
    - Uniqueness (4.8) and legacy-untouched (4.9) are implicit in the above (a real collision would fail the concurrency test; nothing here ever touches a `VR-…` row) rather than separately asserted
    - Verified: skips cleanly via `t.Skip` when `DATABASE_URL` is unset; compiles under `go build -tags=integration ./...`
    - _Requirements: 6.7_

- [ ] 7. Backend service — soft-cancel
  - _Model: Opus, Effort: High — maker-checker + audit + idempotency change to a terminal state; a wrong guard or double-audit corrupts the approval/audit trail._
  - [x] 7.1 Extend `Cancel` in `vendor_request_actions.go`
    - `Cancel` no longer routes through the shared `transition()` helper (that stays for Submit/Approve/Reject/Revise) — it has its own tx/lock/guard/audit flow since it needs `SoftCancelVendorRequest` instead of `UpdateVendorRequestStatus`
    - Already-canceled guard: `req.IsCanceled` → `ErrAlreadyCanceled` (5.7, no duplicate audit, checked before the state guard per design.md ordering); state guard via existing `nextState`/transitions table (`draft`,`pending_approval`) → `ErrInvalidTransition` (5.9); unchanged `checkActor` union (creator Maker OR non-creator Checker, 5.3, 5.8)
    - Sets `status='cancelled'` AND `is_canceled=true` in one statement via `SoftCancelVendorRequest`; writes exactly one `cancel` audit entry with `Before={state,is_canceled:false}`/`After={state:cancelled,is_canceled:true}`, actor, IP (timestamp handled by the existing `audit.Writer`, same as every other transition); never deletes row/items (5.2)
    - _Requirements: 5.2, 5.3, 5.4, 5.7, 5.8, 5.9_
  - [x] 7.2 Add `include_canceled` to list read path
    - `ListVendorRequestParams`/`db.ListVendorRequestsParams`/`db.CountVendorRequestsParams` gained `IncludeCanceled`; default `false` excludes canceled rows (SQL layer from Task 2.4), `include_canceled=true` includes them (5.5)
    - _Requirements: 5.5_
  - [x]* 7.3 Service + handler test soft-cancel
    - `vendor_request_cancel_integration_test.go` (real Postgres, same rationale as Task 5/6: `Cancel` opens a real tx via `db.New(tx)`, not mockable) — reuses Task 6's harness + a new `fetchUserIDs` helper (Cancel's authz union + the `audit_logs.actor_id` FK both need real, distinct `users.id` rows)
    - `TestIntegration_Cancel_SoftCancelPreservesRowAndAudits`: both flags set, item row count unchanged, exactly one `cancel` audit entry (5.2, 5.4); `TestIntegration_Cancel_AlreadyCanceledRejectsWithoutDuplicateAudit`: second cancel → `ErrAlreadyCanceled`, audit count still 1 (5.7); `TestIntegration_Cancel_AuthorizationUnionRule`: non-creator Checker OK, unrelated Maker → `ErrNotAuthorized` (5.3, 5.8); `TestIntegration_Cancel_NonCancelableStatusRejected`: cancel on an approved request → `ErrInvalidTransition` (5.9)
    - Handler-level test (list `include_canceled` toggle, HTTP 409 mapping) deferred to Task 8, which is when the handler actually reads/exposes these fields
    - Verified: skips cleanly via `t.Skip` when `DATABASE_URL` is unset; compiles under `go build -tags=integration ./...`
    - _Requirements: 6.7_

- [ ] 8. Backend handler — read new params and expose additive response fields
  - _Model: Sonnet, Effort: Medium — additive request/response plumbing within the established flat-JSON handler pattern._
  - [x] 8.1 Read new query/body params in `vendor_request_handler.go`
    - `BrowseForecast`: reads `brand`, `flm_vendor`, `flm_vendor_region` additively (existing `forecast_date`/`atm_id`/`page`/`page_size` unchanged, 1.16)
    - `Create`: body gains `replenish_date`, `request_category`, `is_manual`, `vendor_id`; when `is_manual`, `forecast_date` is no longer parsed from the body but defaults to today in Asia/Jakarta (Q5) via the existing `wibZone`; `itemInputPayload` gains `brand`/`lokasi_atm`; `toItemInputs` becomes manual-aware — `periode_pred` stays required for non-manual items (incl. `UpdateItems`, unchanged/always non-manual) but is optional (left zero-valued, the service anchors it to `replenish_date`) for manual items
    - `List`: reads `include_canceled` (`"true"` → include, anything else/absent → exclude, matching `ListVendorRequestParams.IncludeCanceled`'s default-false)
    - `handleError`: `ErrAlreadyCanceled` → 409 (`ErrNumberExhausted` → 409 already existed from the base spec)
    - _Requirements: 1.16, 2.10, 5.5, 5.7_
  - [x] 8.2 Add additive response fields in `vendor_request_response.go`
    - Added `ReplenishDate *time.Time`/`RequestCategory *string`/`IsCanceled bool`/`IsManual bool` to the service-layer `VendorRequestDetail`/`VendorRequestSummary` structs first (they didn't carry these at all before this task), plus a `dateToPtr` helper and the missing `is_canceled`/`request_category`/`replenish_date`/`is_manual` columns on `ListVendorRequests`'s SELECT (only `GetVendorRequestDetail`'s `vr.*` had picked them up automatically)
    - `vendorRequestDetailResponse` + `vendorRequestSummaryResponse` gain `replenish_date *string` (YYYY-MM-DD or null, reusing the existing `formatDatePtr` from `atm_portal_handler.go` rather than duplicating it), `request_category *string`, `is_canceled bool`, `is_manual bool`; `mapDetail`/list mapping copy them through — no renames/removals (6.1)
    - _Requirements: 2.9, 2.10, 5.10, 6.1_
  - [x]* 8.3 Handler test additive shape + regressions
    - Extended `fakeVendorRequestServicer` to capture the last `Create`/`List` input (needed to actually verify body/query parsing, not just response shape)
    - New: `TestVendorRequestHandler_Get_CIT2FieldsAreAdditive` (raw-JSON key check, same pattern as the sibling spec's forecast test) + `_CIT2FieldsNullForLegacyRow`; `TestVendorRequestHandler_Cancel_AlreadyCanceled_Returns409`; `TestVendorRequestHandler_List_IncludeCanceledParam`; `TestVendorRequestHandler_Create_ManualRequestParsing` (vendor_id/replenish_date/category/is_manual/item brand+lokasi_atm all parsed, forecast_date defaults, periode_pred stays zero) + `_NonManualRequiresPeriodePred`
    - Role-gate (6.2) and state-machine (6.3) regressions were already covered by pre-existing tests, unaffected by this task's changes
    - _Requirements: 6.7_

- [x] 9. Checkpoint — backend builds and tests pass
  - `go build ./...`, `go vet ./...`, `go test ./...` — all clean, zero failures, across every package (not just vendor_request)
  - `go build -tags=integration ./...`, `go vet -tags=integration ./...`, `go test -tags=integration ./...` — all clean; every new integration test (Tasks 4.2, 6.2, 7.3) skips cleanly via `t.Skip` since `DATABASE_URL` is unset in this environment — none have actually run against real Postgres yet
  - `-race` unavailable in this environment (no C compiler for cgo — `CGO_ENABLED=1 go test -race` fails with "gcc not found"); noted as an environment limitation, not attempted further
  - `gofmt -l .`: only pre-existing drift in files this spec never touched, plus one trailing-blank-line in a new test file (fixed)
  - `golangci-lint run` on the touched packages: 9 findings, all confirmed via `git show HEAD` to pre-exist in files this spec never touched (dsr_upload*, atm_portal_cashpos.go, and unused `isMaker`/`makerRoles` in vendor_request.go dead since the base spec) — zero findings introduced by Tasks 1–8
  - No questions arose; backend slice (Tasks 1-8) is green and ready for the frontend tasks (10+)

- [x] 10. Frontend — types, api, hooks
  - _Model: Sonnet, Effort: Medium — typed contract extensions and query-builder wiring within existing patterns._
  - [x] 10.1 Extend `types.ts`
    - `BrowseForecastParams` + `brand?`, `flmVendor` (required), `flmVendorRegion` (required); new `VendorRequestCategory` union; `VendorRequestDetail`/`VendorRequestSummary` + `replenish_date: string|null`, `request_category: VendorRequestCategory|null`, `is_canceled: boolean`, `is_manual: boolean`; `CreateVendorRequestPayload` + `replenish_date` (required), `request_category?`, `is_manual?`, `vendor_id` (required), `forecast_date` now optional (omitted for manual); `VendorRequestItemInput` gains optional `brand?`/`lokasi_atm?` and `periode_pred` becomes optional (omitted for manual items); `ListVendorRequestParams` + `includeCanceled?`
    - Deliberately did NOT add `brand`/`lokasi_atm` to the response-side `VendorRequestItem` type: the backend (Task 8) never wired them into `vendorRequestItemResponse` — Task 8.2/design.md's response-field list only covers header-level additions, so the frontend response type stays in sync with what the API actually returns
    - _Requirements: 1.16, 2.10, 5.10_
  - [x] 10.2 Extend `api.ts` query builders
    - `buildForecastQuery` sets `brand` only if non-empty, plus required `flm_vendor`/`flm_vendor_region`; `fetchAllForecastForSelection`'s `Pick` widened to forward the same three; `buildListQuery` forwards `include_canceled`; `createVendorRequest` needed no code change (thin passthrough — the extended payload type from 10.1 covers it)
    - _Requirements: 1.10, 5.5, 2.10_
  - [x] 10.3 Wire hooks
    - `useForecastBrowse` already accepted an external `enabled` flag — no change needed; `useForecastSelectAll`'s `Pick` widened to match `fetchAllForecastForSelection`
    - **Blocker found and resolved**: the required FLM Vendor / FLM Vendor Region selects (Req 1.2-1.4) and the manual-request vendor select (Req 3, Q2) need a vendor/region option source, but no such backend endpoint existed at all (design.md assumed an "existing vendor master endpoint" that was never actually built). Confirmed with the user, then added a new additive endpoint `GET /api/v1/vendor-requests/vendors` (backend: new queries `ListActiveVendors`/`ListDistinctVendorBranchRegions`, `VendorOptionsResult`/`ListVendorOptions` on the service, route + response mapping on the handler, one handler test) returning `{vendors: [{id,name}], regions: [string]}`, gated by the existing `vendorRequestViewerRoles`. Added the matching frontend `VendorOption`/`VendorOptionsResponse` types, `fetchVendorOptions`, and a new `useVendorOptions()` hook (5-minute staleTime, master data)
    - _Requirements: 1.4, 1.5_

- [x] 11. Frontend — Forecast Browser filters
  - _Model: Sonnet, Effort: Medium — standard filter UI + block-fetch gating within the existing page._
  - [x] 11.1 Add filter controls + block-fetch to `ForecastBrowser.tsx`
    - Required FLM Vendor `<select>` (no ALL, 1.2) and FLM Vendor Region `<select>` (no ALL, 1.3), both sourced from the new Task 10.3 `useVendorOptions()`; optional Brand `<select>` with leading "Semua"→empty (1.9), reusing the same static 3-brand list already established in `atm-portal/components/FilterBar.tsx` (no distinct-brands endpoint exists, and 3 known values didn't warrant one — first drafted this as deriving from loaded rows, caught and fixed as broken/circular before finishing)
    - `filtersReady = flmVendor !== "" && flmVendorRegion !== ""` passed as `useForecastBrowse`'s `enabled` arg (1.4, 1.5); prompt panel + `Link` to `/forecasting/dmaa-forecast` rendered in place of the table while unselected (1.5, 1.12); all three selects refetch immediately on change (discrete selects, no debounce needed per design.md) and reset page to 1 (1.11); same `min-h-[44px]` token styling as the file's existing inputs (1.15); empty result still uses `ForecastTable`'s existing empty state (1.13, unchanged)
    - **Regression fix required**: the pre-existing `ForecastBrowser.test.tsx` (sibling `update-cit-forecast-browser` spec) assumed forecast fetching was unconditional — all 7 tests broke (real `Link` crashing without a `RouterProvider`, plus no fetch ever firing). Fixed by stubbing `Link` in the `@tanstack/react-router` mock, adding `/vendors` handling to the existing `mockSingleResponse`/`mockPagedDataset` helpers, and making `renderPage()` select the two required filters (awaiting the options to actually load first, not just the `<select>` element) before returning
    - _Requirements: 1.2, 1.3, 1.4, 1.5, 1.9, 1.11, 1.12, 1.13, 1.15_
  - [x]* 11.2 Component-test filters (Vitest + RTL)
    - New describe block: prompt-panel-shown + zero forecast calls while unselected + DMAA link `href`; fetch fires with `flm_vendor`/`flm_vendor_region` once both are chosen; Brand left at "Semua" omits the `brand` param entirely; changing Brand after paging to page 2 resets to `page=1` and includes the new `brand` value
    - Verified: could not visually verify in the browser preview (dev server redirects to a login wall with no backend/DB session available in this environment) — relied on the 11 passing RTL tests instead, which assert the actual DOM/query behavior directly
    - _Requirements: 6.7_

- [x] 12. Frontend — Create page context columns, replenish_date, manual mode + category
  - _Model: Sonnet, Effort: Medium — feature-rich but pattern-consistent form work; category/date coupling mirrors the confirmed server rules._
  - [x] 12.1 Add `tomorrowJakartaISO()` to `lib/nextBusinessDay.ts`
    - Added a general `jakartaCalendarDateISO(offsetDays, from?)` (fixed UTC+7 offset, matching the backend's `jakartaCalendarDate` convention — no DST in Asia/Jakarta) plus `tomorrowJakartaISO()` as its H+1 convenience wrapper; documented the distinction from the existing business-day-skipping `nextBusinessDayISO`
    - _Requirements: 2.4_
  - [x] 12.2 Add context columns + replenish_date input to `VendorRequestCreate.tsx`
    - Brand / FLM Vendor / FLM Vendor Region columns read from the form's per-item state (seeded from `selectionStore` rows at mount, not re-derived by index — avoids desync with row deletion), `displayOrDash()` renders a plain "-" for empty/null (2.1, 2.2); `<input type="date">` defaults to `tomorrowJakartaISO()` with `min` = today (2.3, 2.4); missing/past-date client-side error shown inline without navigating, input preserved (2.6, 2.7); `toPayload` sends `replenish_date` (2.10)
    - **Found and fixed while implementing**: every create (not just manual, per Q2) needs a resolved `vendor_id`, but `selectionStore`'s `PendingVendorRequestSelection` only carried `{forecastDate, items}`. Added `vendorId` to that interface and wired `ForecastBrowser.tsx`'s `handleCreateClick` to resolve it from the already-selected FLM Vendor filter via `useVendorOptions()` — required regression-adjacent backend-contract work this task's Requirement 2.10 (`vendor_id` in the payload) depends on
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.6, 2.7, 2.10_
  - [x] 12.3 Add manual mode + category selector + vendor select + manual rows
    - "Buat Manual" button on the empty-state gate starts a create with no `selectionStore` selection (3.1); required Request Category `<select>` (3.2) via `handleCategoryChange`: Planned locks the date input to H+1 and disables it (3.3), Emergency locks to H+0 (3.4), Additional swaps to a `<select>` of exactly the 3 allowed dates (3.5); required Vendor `<select>` sourced from Task 10's `useVendorOptions()` (Q2); manual rows (`emptyManualItem()` + `append`/`remove`) with editable terminal_id (≤64 chars via Zod), denom, amount_replenish, and optional brand/lokasi_atm (3.6, Q4); unified Zod `itemSchema` (DMAA fields optional, used only for display) so one form/schema serves both modes; `toPayload` sends `is_manual`, `request_category` (manual only), `vendor_id`; the DMAA path sends `is_manual: false` and omits `request_category` entirely, matching the backend's Task 5 resolution (3.8)
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.8_
  - [x]* 12.4 Component-test create page
    - New describe blocks in `VendorRequestCreate.test.tsx`: replenish_date defaults to `tomorrowJakartaISO()`, context columns render "-" for empty (never em-dash); Planned/Emergency lock+disable the date input to H+1/H+0, Additional renders a 3-option H+0/H+1/H+2 select; a manually added row accepts terminal_id/denom/amount_replenish and enables submission once a vendor is chosen
    - Regression fix required: `PendingVendorRequestSelection` gaining `vendorId` broke the pre-existing test file's 3 `setPending(...)` calls (base Req 12 spec) — fixed by adding `vendorId: 1`
    - Verified: 10/10 new + existing tests pass; full `vendor-request` suite 44/44; `tsc -b --noEmit` clean except the pre-existing, expected Task 13 `VendorRequestDetail.test.tsx` gap
    - _Requirements: 6.7_

- [x] 13. Frontend — List/Detail replenish_date, category, canceled badge, cancel authz
  - _Model: Sonnet, Effort: Medium — display fields + badge + widening an existing permission check to match the backend union._
  - [x] 13.1 Canceled badge with icon + text in `StatusBadge.tsx` (or new `CanceledBadge.tsx`)
    - When `is_canceled` true, render neutral `Badge` with `Ban`/`XCircle` Lucide icon + "Dibatalkan" label — never color alone (5.6); keep strikethrough for `cancelled` status
    - Implemented as a new `isCanceled?: boolean` prop on the existing `StatusBadge` (not a separate `CanceledBadge.tsx`): when true it short-circuits to `<Badge variant="neutral" icon={Ban} label="Dibatalkan" />`, called alongside (not instead of) the normal status badge at both call sites so the underlying status stays visible
    - _Requirements: 5.6_
  - [x] 13.2 Update `VendorRequestList.tsx`
    - Canceled badge on rows, `replenish_date` column, default hides canceled (omit `include_canceled`), optional "Tampilkan yang dibatalkan" toggle → `include_canceled=true` (5.5); replenish_date shown as labeled field distinct from forecast/periode (2.9)
    - `showCanceled` kept as local `useState` (not URL state) — simplest fit since the toggle isn't required to be shareable/bookmarkable; `ListVendorRequestParams.includeCanceled` and `api.ts`'s query-param omission when `false` already existed from Task 8, so the default-hide behavior needed no backend-facing change
    - _Requirements: 2.9, 5.5, 5.6_
  - [x] 13.3 Update `VendorRequestDetail.tsx`
    - Widen `canCancel` to Maker+SPV union `isCreator || (isChecker && !isCreator)` for `draft`/`pending_approval` (5.3); canceled badge in header (5.6); "Tanggal Replenish" + "Kategori" labeled fields distinct from forecast_date, "-" when null (2.9)
    - Replaced the old comment block that had deliberately *restricted* Cancel to isCreator-only (a Task-11-era note explaining a mismatch with the backend's already-wider `checkActor`) — this task closes that gap so the UI now matches the backend's permission source of truth
    - _Requirements: 2.9, 5.3, 5.6_
  - [x]* 13.4 Component-test detail/list
    - Canceled badge shows icon+text (not color alone); replenish_date + category shown as distinct labeled fields; cancel button visible to Maker and SPV
    - Fixed the already-known typecheck gap: `VendorRequestDetail.test.tsx`'s `BASE_DETAIL` fixture was missing the CIT-2 fields (`replenish_date`, `request_category`, `is_canceled`, `is_manual`) added to `VendorRequestDetail` in Task 8 — added them, plus 4 new tests (labeled fields render "-" when null, canceled badge, SPV-sees-Cancel on both draft and pending_approval)
    - Updated a pre-existing test ("pending_approval + non-creator checker sees Approve/Reject — not Cancel") whose assertion was now outdated by 13.3's intentional widening — removed the stale "not Cancel" assertion
    - New `VendorRequestList.test.tsx` (none existed before): Tanggal Replenish column, canceled badge scoped to the row (a bare `getByText("Dibatalkan")` collided with the status-filter dropdown's own "Dibatalkan" option — fixed by scoping to the row via `closest("tr")`), and the include_canceled toggle wiring
    - Verified: full `vendor-request` suite 51/51 (up from 44/44); `tsc -b --noEmit` has zero vendor-request errors (confirmed the remaining ~209 lines of tsc output are pre-existing on the base commit via `git stash` diff, unrelated to this spec)
    - _Requirements: 6.7_

- [x] 14. Final quality gate — full build/lint/test both stacks
  - _Model: Sonnet, Effort: Medium — triage failures across backend and both frontend concerns._
  - Backend: `go build ./...`, `go vet ./...`, `go test ./...`. Frontend (`frontend/CompanyPortal-Vite/`): `pnpm lint`, `pnpm test --run`, `pnpm build`. Ensure all pass, ask the user if questions arise.
  - Backend (`backend/`, `backend-cit/`): `go build`/`go vet`/`go test` all clean on both modules, incl. `-tags=integration` build/vet/test on `backend/` (integration tests skip without a live `DATABASE_URL`, unchanged from earlier tasks).
  - Frontend `pnpm lint`: found 9 Biome errors — 5 were in files this session touched (formatting in `api.ts`/`hooks.ts`/`VendorRequestCreate.tsx`/`ForecastBrowser.test.tsx`, plus an invalid-anchor a11y warning in the new `VendorRequestList.test.tsx`'s mocked `Link`) and were fixed (`biome check --write` for formatting, real route path for the anchor). The other 4 (`RetryDrawer.tsx` a11y, `ReplenishmentScreen.tsx` format, 2× `navigation.ts`) are pre-existing on the base commit (confirmed via `git stash`) and out of this spec's scope — left untouched.
  - Frontend `pnpm test --run`: vendor-request suite 51/51. Full repo suite has 22 pre-existing failures across 5 files (`Replenishment.test.tsx`, `NavGroup`-related, CIT `useCitData`/QueryClient wiring, `stubs/index.test.ts`) — confirmed identical failing files on the base commit via `git stash`, unrelated to CIT-2.
  - Frontend `pnpm build`: **fails**, but only because `tsc -b` surfaces the same ~209 lines of pre-existing type errors across untouched files (property tests, `contrast.property.test.ts`, `config.ts`'s `ImportMeta.env`, etc.) already present on the base commit before this spec started — confirmed via `git stash` comparison. Zero errors originate from any `vendor-request` file. This spec's own build health is clean; the repo-wide build was already broken beforehand and fixing it is out of scope for CIT-2.

## Notes

- Tasks marked with `*` are optional test sub-tasks and can be skipped for a faster MVP; core implementation sub-tasks are never optional.
- Each task references specific requirements for traceability.
- Task 1.5 is a hard STOP gate: no migration is applied to the external Postgres without explicit user sign-off (Golden Rule 7).
- Per the sqlc caveat, regeneration (Task 2.5) keeps only the `vendor_request*.sql.go` diff and hand-edits `models.go` to sqlc convention; nullable columns emit as pointers.
- The design has no Correctness Properties requiring property-based tests; tests are example-based unit + integration (backend) and component (frontend), mapped to Req 6.7.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.2", "1.3", "1.4"] },
    { "id": 1, "tasks": ["1.5"] },
    { "id": 2, "tasks": ["2.1", "2.2", "2.3", "2.4"] },
    { "id": 3, "tasks": ["2.5"] },
    { "id": 4, "tasks": ["3.1"] },
    { "id": 5, "tasks": ["3.2", "4.1", "7.2"] },
    { "id": 6, "tasks": ["3.3", "4.2", "5.1"] },
    { "id": 7, "tasks": ["5.2", "5.3"] },
    { "id": 8, "tasks": ["5.4", "5.5", "6.1"] },
    { "id": 9, "tasks": ["6.2", "7.1"] },
    { "id": 10, "tasks": ["7.3", "8.1"] },
    { "id": 11, "tasks": ["8.2"] },
    { "id": 12, "tasks": ["8.3", "10.1"] },
    { "id": 13, "tasks": ["10.2", "10.3", "12.1"] },
    { "id": 14, "tasks": ["11.1", "12.2", "13.1"] },
    { "id": 15, "tasks": ["11.2", "12.3", "13.2", "13.3"] },
    { "id": 16, "tasks": ["12.4", "13.4"] }
  ]
}
```
