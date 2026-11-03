# Implementation Plan: Vendor Package Code Split

## Overview

This plan splits the single identifier on `public.vendor_package_prices` into two concepts: the existing `package_code` column is **renamed** to `package` (the shared human-readable label that groups rows), and a new per-row unique `package_code` column is **added** and backfilled deterministically. The ripple beyond the migration is narrow and mechanical: the sqlc queries carry both columns and filter the list by the label; the generated db structs gain `Package`; the service DTO/payloads split the two fields; the applier generates the per-row code at apply-on-approve time inside the transaction; the handler emits both fields; and the frontend renders a "Kode Paket" column (the per-row code) and a "Paket" column (the label).

Tasks are ordered so each builds on the previous, following the design's layering (Migration → SQL → generated db → service DTO → applier generator → handler → frontend → docs). The migration comes first because everything downstream references the two-column shape. The real-Postgres migration test and the service property tests are the true safeguards for the constraint rebuild and the concurrent code generator, and are marked optional (`*`) as tests, but skipping them is not recommended for a change touching a money table's overlap-integrity constraint.

Everything preserved unchanged is preserved deliberately: money-as-decimal-string, maker-checker staging (202 pending), the overlap 409, the `ADMIN`/`ADMIN_PARAM` guard, primary-write/replica-read routing, and flat JSON with no envelope.

## Tasks

- [ ] 1. Write Migration 017 (rename + add + backfill + validate + rebuild constraints)
  - Create `backend/migrations/017_vendor_package_code_split.sql` as ONE transaction (`BEGIN…COMMIT`, Req 4.1) in the design's exact order: (1) `ALTER TABLE public.vendor_package_prices RENAME COLUMN package_code TO package` preserving data + `NOT NULL` + grouping role (Req 1.1, 1.2, 1.3); (2) `ADD COLUMN package_code text` nullable during backfill (Req 2.1); (3) backfill `PKG<digits>_<vendor.code>_<seq3>` via the `WITH numbered` CTE joining `vendors`, using `COALESCE(NULLIF(regexp_replace(package,'\D','','g'),''),'0')` for the digitless-label fallback token `'0'` (Req 2.2, 2.3, Design Decision 1) and `lpad(ROW_NUMBER() OVER (PARTITION BY vendor_id ORDER BY id),3,'0')` for the per-vendor sequence.
  - Add the pre-commit validation `DO $$ … $$` block that `RAISE EXCEPTION`s (aborting the tx) on any duplicate `package_code`, any null `package`, or any null `package_code` (Req 4.3, 4.4, 4.5).
  - Lock down: `ALTER COLUMN package_code SET NOT NULL` + `ADD CONSTRAINT vendor_package_prices_package_code_key UNIQUE (package_code)` — global uniqueness (Req 2.5, 2.6).
  - Rebuild `vpp_no_overlap` (`DROP` then `ADD CONSTRAINT … EXCLUDE USING gist`) substituting `package WITH =` for the old package-code partition, EVERYTHING ELSE identical to migration 009 including the `COALESCE(...,0)` branch/ATM coalescing and both `int4range`/`daterange` `&&` terms (Req 1.4, 3.1, 3.2, 3.3, 3.4); `DROP INDEX IF EXISTS vpp_lookup_idx` then `CREATE INDEX vpp_lookup_idx ON … (vendor_id, package, machine_group, price_class)` (Req 1.5). Leave `vpp_branch_idx`/`vpp_atm_idx` untouched.
  - In the file header, document the reverse-migration reasoning (not an executed `down`): drop the unique constraint, drop the rebuilt overlap constraint + lookup index, drop the generated `package_code` column (data derivable), rename `package` back to `package_code`, recreate `vpp_no_overlap`/`vpp_lookup_idx` on `package_code` as 009 defined them — lossless because the grouping column's data never moves (Req 4.6).
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 3.1, 3.2, 3.3, 3.4, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_
  - _Model: Opus, Effort: High — data migration touching a money table's overlap-integrity constraint; a wrong rebuild of `vpp_no_overlap` silently loses overlap protection, and a wrong backfill/validation order can commit corrupt or duplicate codes._

- [ ]* 2. Write the real-Postgres migration/integration test
  - In `backend/internal/repository` (or the repo's migration-test harness), seed pre-migration-shape rows (label in `package_code`, no per-row column), run `017`, and assert against real Postgres: `package` holds the old `package_code` values verbatim with `NOT NULL` still enforced (Property 1); `package_code` is non-null, globally unique, matches `^PKG.+_.+_\d{3}$`, with per-vendor sequences `001,002,…` in `id` order (Properties 1, 4); a seeded digitless `"PAKET"` label backfills to `PKG0_<code>_NNN` (Design Decision 1 fallback); a second insert on the same grain (vendor_id, `package`, machine_group, price_class, level, overlapping tier + period) still raises `23P01` (Property 2); `vpp_lookup_idx` exists on `(vendor_id, package, machine_group, price_class)` and `vpp_branch_idx`/`vpp_atm_idx` are untouched; a forced validation failure (injected duplicate) leaves the table in its pre-migration shape (Req 4.5). One or two representative fixtures suffice.
  - **Feature: vendor-package-code-split, Property 1: backfill uniqueness, totality, and label preservation.**
  - **Feature: vendor-package-code-split, Property 2: overlap invariance across the rename.**
  - **Validates: Requirements 1.1, 1.3, 1.4, 2.4, 2.5, 3.1, 3.2, 3.3, 3.4, 4.3, 4.4, 4.5**
  - _Model: Opus, Effort: High — this is the true safeguard for the constraint rebuild; proving the overlap constraint still fires and the backfill is unique/total against a live DB with careful fixture seeding is where migration correctness is actually proven._

- [ ] 3. Update the sqlc queries to carry both columns
  - In `backend/queries/vendor_package_prices_admin.sql`: add `package` and `package_code` to the SELECT lists of `ListVendorPackagePricesAdmin`, `GetVendorPackagePriceAdminByID`, and the `RETURNING` of create/update/disable; change the list/count filter from the old `package_code` narg to `sqlc.narg('package')::text` matching the `package` **label** column (Req 6.5, Design Decision 2); lead `ORDER BY` with `package ASC` (same values as before, behaviorally identical ordering).
  - `CreateVendorPackagePriceAdmin` inserts BOTH `package` (label from payload) and `package_code` (a new server-generated bound arg) and `RETURNING` both; `CountVendorPackagePricesAdmin` mirrors the same `WHERE` with the `package` filter; `UpdateVendorPackagePriceAdmin`/`DisableVendorPackagePriceAdmin` keep their SET lists (grain + code immutable, Req 6.4/7.4) but add both columns to their SELECT/`RETURNING` projections so the DTO round-trips.
  - _Requirements: 6.1, 6.5, 5.1_
  - _Model: Sonnet, Effort: Medium — SQL edits following the existing query file's shape; the one judgment (filter narg renamed to the label column) follows Design Decision 2._

- [ ] 4. Regenerate (or hand-write) the generated db code
  - Attempt `sqlc generate` first. If it succeeds, commit the produced `backend/internal/db/vendor_package_prices_admin.sql.go`.
  - If `sqlc generate` is blocked by the known pre-existing archived-migration bug (project-context Sec 12), hand-write `backend/internal/db/vendor_package_prices_admin.sql.go` to match sqlc's exact convention: `CreateVendorPackagePriceAdminParams` gains `Package string` and keeps `PackageCode string` (now the per-row code); all `*Row` structs (`ListVendorPackagePricesAdminRow`, `GetVendorPackagePriceAdminByIDRow`, create/update `RETURNING` rows) gain `Package string` in the position matching the SELECT column order; the list/count `*Params` filter field is renamed `PackageCode *string` → `Package *string`. Keep the same `pgtype` imports and `q.db.Query`/`QueryRow` bodies and ordinal scan order as the existing file.
  - _Requirements: 6.1, 6.5_
  - _Model: Opus, Effort: High if hand-writing — generated code must match sqlc's convention exactly (field order, nullable handling, scan order) or every layer above it silently mis-maps columns; try `sqlc generate` first and only hand-write on the documented block._

- [ ] 5. Update the service DTO, create payload, and mappers
  - In `backend/internal/service/vendor_package_price_admin.go`: the `VendorPackagePrice` read DTO gains `Package string` (label) alongside `PackageCode string` (per-row code); `VendorPackagePriceCreatePayload` gains `Package string json:"package"` and **drops** the `package_code` json field so a client-supplied code has no struct field and is silently ignored by `encoding/json` (Req 5.2, 6.3); `VendorPackagePriceContentPayload` (update path) stays unchanged — `base_price`, `sla_note`, `effective_end_date` only, so both `package` and `package_code` remain immutable on update (Req 6.4, 7.4).
  - `validatePriceGrain` validates `Package` (trim, required) in place of the old package-code field; the `newVendorPackagePrice`/`fromListRow`/`fromGetRow` mappers gain the `Package` argument and populate both DTO fields. Keep `base_price` a decimal string end to end, never float (Req 6.6).
  - _Requirements: 5.2, 6.3, 6.4, 6.6, 7.4_
  - _Model: Sonnet, Effort: Medium — DTO/payload field split following the existing service shape; the immutability guarantee falls out of the unchanged content payload rather than new logic._

- [ ] 6. Implement the per-row Package_Code generator in the applier
  - In `backend/internal/service/masterdata_applier_vendor_package_price.go`, add `nextPackageCode(ctx, tx pgx.Tx, vendorID int64, label string) (string, error)` computed on the SAME apply transaction as the INSERT (Req 5.4): (1) `SELECT pg_advisory_xact_lock($1)` keyed on `vendorID` to serialize same-vendor applies (Design Decision 4); (2) `SELECT code FROM vendors WHERE id=$1` for `<vendor.code>`; (3) `SELECT COALESCE(MAX((regexp_replace(package_code,'^.*_',''))::int),0)` over this vendor's existing codes (covers backfilled + generated, both share the format); (4) `fmt.Sprintf("PKG%s_%s_%03d", digitsOrFallback(label), vendorCode, maxSeq+1)` where `digitsOrFallback` returns the label's numeric chars or `"0"` (Design Decision 1). Add the `digitsOrFallback` helper.
  - Wire the generated code into the applier's `create` branch: pass it as the new `PackageCode` arg to `CreateVendorPackagePriceAdmin` while `Package` receives the payload label (Req 5.1). Extend `mapPriceDBError` to translate SQLSTATE `23505` (unique violation on `vendor_package_prices_package_code_key`) into a distinct retryable/conflict error the maker-checker layer can retry (Req 5.5, final backstop); keep the `23P01` → `ErrVendorPackagePriceOverlap` → 409 path unchanged (Req 7.5). The disable branch still ends the period, never hard-deletes or re-enables (Req 7.3).
  - _Requirements: 5.1, 5.3, 5.4, 5.5, 7.3, 7.5, 9.2_
  - _Model: Opus, Effort: High — concurrency (advisory xact lock) + uniqueness + generate-on-approve timing + in-tx sequence read is the correctness heart of Req 5; a stale MAX read off the wrong connection, or a missed 23505 mapping, produces colliding or gap-burning codes._

- [ ]* 7. Write service tests for the generator (properties + examples)
  - [ ]* 7.1 Property test — code-generation uniqueness under concurrency
    - **Feature: vendor-package-code-split, Property 3: For any sequence of create-applies — including two applies for the same vendor executed concurrently — every committed `package_code` is globally distinct, the global `UNIQUE(package_code)` is never violated in committed state, and a rejected or still-pending change request consumes no sequence value.**
    - Using the repo's PBT library (`pgregory.net/rapid` or the vendored choice — do not hand-roll), min 100 iterations: generate create-apply sequences over varied vendors/labels against a repo that honors the advisory lock + unique constraint; assert committed codes are distinct, simulated concurrent same-vendor applies never both commit the same code, and a "rejected"/"pending" apply leaves the vendor's next seq unchanged.
    - **Validates: Requirements 5.3, 5.4, 5.5**
    - _Model: Opus, Effort: High — modeling concurrent applies and the reject/pending "no-consumption" invariant faithfully (lock + unique backstop) is subtle; a vacuous fake makes the property meaningless._
  - [ ]* 7.2 Property test — code format and per-vendor monotonicity
    - **Feature: vendor-package-code-split, Property 4: For any label and vendor, a generated or backfilled `package_code` matches `^PKG.+_<vendor.code>_\d{3}$`, and within a single vendor the trailing 3-digit sequence is strictly increasing in create order (generated codes continue past backfilled codes without collision).**
    - Min 100 iterations; include a backfilled-continuation case (seed codes at seq `005`, assert the next generated code is `006`).
    - **Validates: Requirements 2.2, 2.3, 5.1**
    - _Model: Opus, Effort: High — the format regex + monotonic-continuation-past-backfill invariant rewards generated-input coverage across label shapes and existing row sets._
  - [ ]* 7.3 Example/unit tests — client code ignored + update immutability
    - **Feature: vendor-package-code-split, Property 5 (examples): a Create payload carrying `package_code` persists only the server-generated code; an Update mutates neither `package` nor `package_code`.**
    - Table-driven examples: create with a `package_code` in the body → persisted row has the generated code, not the client's (Req 5.2, 6.3); update attempting `package`/`package_code`/grain change → those fields unchanged (Req 6.4, 7.4).
    - **Validates: Requirements 5.2, 6.3, 6.4, 7.4**
    - _Model: Sonnet, Effort: Medium — concrete example assertions complementing the properties; standard table-driven service work._

- [ ] 8. Update the handler to emit both fields and filter by label
  - In `backend/internal/handler/admin_vendor_package_price_handler.go`: `packagePriceToResponse` emits both `"package": p.Package` (label) and `"package_code": p.PackageCode` (per-row code) in the flat `map[string]any`, all other fields unchanged (Req 6.1, 6.2); the `List` handler reads query param `package` (the label) instead of the old `package_code` and binds it to the renamed `Package *string` params field (Req 6.5, Design Decision 2). Create still returns `202` (Req 7.1); overlap still maps to `409` via `ErrVendorPackagePriceOverlap` (Req 7.5); the `RequireRoles("ADMIN","ADMIN_PARAM")` guard is unchanged (Req 9.1).
  - _Requirements: 6.1, 6.2, 6.5, 7.1, 7.5, 9.1_
  - _Model: Sonnet, Effort: Medium — response field addition + filter-param rename on an existing handler; error mapping and flat-JSON shape are established patterns._

- [ ]* 9. Write handler httptest tests
  - Table-driven httptest in `admin_vendor_package_price_handler_test.go`: List/Get responses include both `package` and `package_code` (Property 5, Req 6.1); Create with a body carrying `package_code` → value ignored, response 202, persisted row (via fake applier) has a server-generated code (Req 5.2, 7.1); Update body attempting `package`/`package_code`/grain change → those fields unchanged (Req 6.4, 7.4); overlap error → 409 (Req 7.5); wrong role → 403 (Req 9.1); flat JSON shape (no envelope, Req 6.2); List `package` filter narrows by label (Req 6.5).
  - **Feature: vendor-package-code-split, Property 5: API round-trip contract.**
  - **Validates: Requirements 5.2, 6.1, 6.2, 6.3, 6.4, 6.5, 7.1, 7.4, 7.5, 9.1**
  - _Model: Sonnet, Effort: Medium — comprehensive but conventional httptest table; the 403 case needs the role middleware wired in the test harness._

- [ ] 10. Checkpoint - Ensure the ATM backend builds, tests, and lints clean
  - Run `go build ./...`, `go test ./...`, and `golangci-lint run` for the ATM backend. Ensure all migration/service/handler tests pass; ask the user if questions arise.
  - _Model: Sonnet, Effort: Medium — build/test/lint triage across the migration → sqlc → service → applier → handler slice needs judgment but not deep reasoning._

- [ ] 11. Add the `package` field to the frontend types
  - In `frontend/CompanyPortal-Vite/src/features/admin-vendors/types.ts`: `AdminVendorPackagePrice` gains `package: string` (label) alongside the existing `package_code: string` (per-row code); the list params type's filter field becomes `package?: string`.
  - _Requirements: 6.1, 6.5_
  - _Model: Haiku, Effort: Low — pure type declarations mirroring the endpoint contract; one obvious shape._

- [ ] 12. Split the Harga Paket panel into "Kode Paket" and "Paket" columns
  - In `frontend/CompanyPortal-Vite/src/features/admin-vendors/components/PackagePricesPanel.tsx`: split the single column that today binds to `package_code` into a **Kode Paket** column bound to `package_code` rendered `font-mono tabular-nums` (it is a machine code, Req 8.1) and a **Paket** column bound to `package` (the label, Req 8.2); the `useMemo` sort and any label filter key on `package` (Req 8.3); the disable-confirm dialog message references `pendingDisable?.package` (the human label) rather than the code. Render `base_price` from the API decimal string, never `Number()`-ed (Req 8.5).
  - _Requirements: 8.1, 8.2, 8.3, 8.5_
  - _Model: Sonnet, Effort: Medium — real column/sort/filter work with design-system rules (mono/tabular code, label grouping), but a close edit of the existing panel._

- [ ] 13. Update the create form to collect the label only
  - In `frontend/CompanyPortal-Vite/src/features/admin-vendors/components/VendorPackagePriceFormDialog.tsx`: the create form collects the `package` **label** and sends it as `package`; remove the `package_code` input entirely — the code is server-generated and must not be collected (Req 8.4). Update mode still edits only content fields (base price, SLA note, effective end date).
  - _Requirements: 8.4_
  - _Model: Sonnet, Effort: Medium — form field swap (remove code input, send label) on the existing dialog; the immutability of the code is enforced by simply not collecting it._

- [ ]* 14. Write frontend component tests for the panel and form
  - `PackagePricesPanel.test.tsx` (RTL + MSW): both "Kode Paket" (`package_code`, mono/tabular) and "Paket" (`package`) columns render (Req 8.1, 8.2); grouping/sorting/filtering keys on the `package` label (Req 8.3); the create form (`VendorPackagePriceFormDialog`) has **no** code input and submits `package` only (Req 8.4); `base_price` renders from the decimal string and is never `Number()`-ed (Req 8.5).
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_
  - _Model: Sonnet, Effort: Medium — thorough RTL+MSW suite covering the two-column render, label grouping, and money-as-string assertions; conventional but broad._

- [ ] 15. Reconcile the sibling `vendor-detail-revamp` doc note
  - Update the reconciliation caveat in `.kiro/specs/vendor-detail-revamp` (design/tasks) that currently says "render `package_code` verbatim, do not synthesize codes": after `017`, `package_code` **is** the real persisted per-row code and `package` is the label the Harga Paket revamp groups and filters by. Adjust only the note; do not otherwise change that spec's scope.
  - _Requirements: 6.1, 8.1, 8.2_
  - _Model: Haiku, Effort: Low — a doc-note edit reconciling a sibling spec to the resolved two-field reality._

- [ ] 16. Record the applied schema change in steering
  - In `.kiro/steering/project-context.md` Sec 2, record the renamed `package` column and the new per-row `package_code` column of `vendor_package_prices` (apply-then-record rule, Req 10.1), and note that any join between price rows and `package_frequencies` now matches `vendor_package_prices.package = package_frequencies.package_code` (Req 10.2).
  - _Requirements: 10.1, 10.2_
  - _Model: Haiku, Effort: Low — a steering-doc edit recording the applied schema change and the new join key; no code judgment._

- [ ] 17. Final checkpoint - Ensure full backend + frontend build/test/lint green
  - Run the full quality gate: ATM backend `go build ./...` + `go test ./...` + `golangci-lint run`, and frontend `pnpm build` + `pnpm test` + `pnpm lint` in `CompanyPortal-Vite`. Ensure everything is green; ask the user if questions arise.
  - _Model: Sonnet, Effort: Medium — final cross-stack build/test/lint triage._

## Notes

- Tasks marked with `*` are optional (tests) and can be skipped for a faster MVP, but the design's Correctness Properties (Properties 1–5) and the real-Postgres migration test are the real safeguards for the constraint rebuild, the concurrent code generator, and the API round-trip — skipping them is not recommended for a change touching a money table's overlap-integrity constraint.
- Each task references specific granular requirements for traceability, and every task carries a `Model:` + `Effort:` note per `.kiro/steering/model-recommendation.md`.
- Checkpoints (tasks 10, 17) ensure incremental validation at the backend and full-stack boundaries.
- Open questions from requirements are resolved by the design: the digitless-label fallback token is `'0'` (Design Decision 1, Open Q1); the list filter param is renamed `package_code` → `package` with no legacy alias (Design Decision 2, Open Q2); the code is generated at apply-on-approve time (Design Decision 3, Open Q3). Confirm the fallback token with business before relying on production data.
- The migration is forward-only per repo convention; the reverse-migration reasoning lives in the `017` file header, not an executed `down` (Req 4.6).
- Property tests use the repo's PBT library (`pgregory.net/rapid` or the vendored choice — do not hand-roll); the migration test uses real Postgres for the constraint/backfill behavior a fake cannot prove (project-context Sec 7).

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1"] },
    { "id": 1, "tasks": ["2", "3", "11"] },
    { "id": 2, "tasks": ["4", "15", "16"] },
    { "id": 3, "tasks": ["5", "12", "13"] },
    { "id": 4, "tasks": ["6", "14"] },
    { "id": 5, "tasks": ["7.1", "7.2", "7.3"] },
    { "id": 6, "tasks": ["8"] },
    { "id": 7, "tasks": ["9"] }
  ]
}
```
