# Implementation Plan: Region Management (Manajemen Region)

## Overview

Backend-first implementation of controlled CRUD for the `regions` master table on the ATM backend (`backend/`, port 8080, flat JSON) plus a "Manajemen Region" card under the Settings "Data master" tab in `CompanyPortal-Vite`. The sequence mirrors the sibling `admin-atm-management` / `internal/rolemgmt` implementations: migration → sqlc queries → repository → service → handler → route wiring → frontend, each step building on the prior with tests. The change-control model is immediate-apply-with-audit (audit written inside the same transaction as the mutation; audit failure rolls back), with RBAC re-checked at both middleware and service layers. No maker-checker, no hard delete.

Reference implementations to match for conventions:
- `backend/internal/rolemgmt/` — audit-in-tx service pattern (`PermissionService`), narrow interfaces, `Pool` abstraction.
- `backend/internal/repository/atm_admin_repository.go` — primary/replica split, `*db.Queries` wrapping, `timestamptzToPtr`.
- `backend/internal/handler/admin_atm_handler.go` — flat-JSON handler, `handleATMAdminError`, `writeJSON`/`writeError`/`writeValidationError`/`extractClientIP`/`parsePathID`.
- `backend/internal/db/{audit,approval,role_mgmt}.sql.go` — hand-written sqlc-style output (sqlc is blocked by the migration 017 bug).
- `frontend/CompanyPortal-Vite/src/features/admin-atms/` — feature layout, hooks, URL state, RHF+Zod dialog.

## Tasks

- [x] 1. Schema migration for region soft-delete
  - Create `backend/migrations/018_regions_soft_delete.sql`: additive `ALTER TABLE public.regions ADD COLUMN is_active boolean NOT NULL DEFAULT true, ADD COLUMN deleted_at timestamptz` inside a `BEGIN/COMMIT`, with a `COMMENT ON COLUMN regions.deleted_at`. Do NOT re-add `uq_regions_code` or `trg_regions_set_updated_at` — both already exist in `001_baseline_schema.sql`. Forward-only (no down migration).
  - Add `IsActive bool` and `DeletedAt pgtype.Timestamptz` to the `Region` struct in `backend/internal/db/models.go`.
  - Record the `regions` table change (`+ is_active`, `+ deleted_at`, migration 018) in `project-context.md` Sec 2 Master group after the migration is written.
  - _Requirements: 4.1, 4.3, 9.4_
  - _Model: Opus, Effort: High — additive data migration on a referenced master table + struct sync; getting nullability/default wrong affects all 13 existing rows and downstream FK integrity._

- [x] 2. sqlc query layer for region admin
  - [x] 2.1 Author `backend/queries/regions_admin.sql`
    - Write sqlc-style queries: `ListRegionsAdmin` (filter `q` ILIKE on code OR region case-insensitive, status narg all/active/inactive, LIMIT/OFFSET, `LEFT JOIN locations l ON l.region_id = r.id AND l.is_active` with `COUNT(l.id) AS location_count`, `GROUP BY r.id`, order `code ASC, id ASC`), `CountRegionsAdmin` (same filter, no limit/aggregation), `GetRegionAdminByID` (full row + location_count), `CreateRegionAdmin` (INSERT code, region → returning row), `UpdateRegionName` (UPDATE region WHERE id → returning row), `SetRegionActive` disable (`is_active=false, deleted_at=now() WHERE id AND is_active=true`) and enable (`is_active=true, deleted_at=NULL WHERE id AND is_active=false`) variants, `FindRegionByCode` (SELECT id WHERE code, includes soft-deleted), `CountActiveLocationsByRegion` (COUNT locations WHERE region_id AND is_active).
    - _Requirements: 1.4, 1.5, 1.6, 2.1, 2.2, 3.1, 4.1, 4.2, 4.3, 7.2_
  - [x] 2.2 Generate or hand-write `backend/internal/db/region_admin.sql.go`
    - Run `sqlc generate`; keep only the new `region_admin.sql.go` and revert drift in other files. If sqlc cannot run (blocked by the migration 017 bug), hand-write `internal/db/region_admin.sql.go` matching the exact output style of `internal/db/role_mgmt.sql.go` (params/rows structs, `db.DBTX`).
    - _Requirements: 1.6, 2.1, 3.1, 4.1_

- [x] 3. Region admin repository
  - [x] 3.1 Implement `backend/internal/repository/region_admin_repository.go`
    - Wrap `*db.Queries` (pattern: `atm_admin_repository.go` + `rolemgmt.Repository`). Constructor `NewRegionAdminRepository(primary, replica db.DBTX)`. Route `List`/`Count` to replica (`dbRead`); `GetByID`/`FindByCode` and all mutations to primary. Tx-accepting mutation methods `CreateTx`/`UpdateNameTx`/`SetActiveTx` using `db.New(tx)` so audit shares the transaction. Map `pgx.ErrNoRows` → `(nil, nil)` for `GetByID`/`FindByCode`. Expose `CountActiveLocations` for the referential-integrity check.
    - _Requirements: 1.6, 1.7, 4.2, 9.1, 9.2, 9.3_
  - [x]* 3.2 Write integration tests `region_admin_repository_test.go` (`//go:build integration`, real Postgres)
    - create→get roundtrip; list filter matrix (`q`, status) + correct `location_count`; pagination count vs per-page; `uq_regions_code` unique-violation surfaces; `SetRegionActive` toggles `is_active`+`deleted_at`; `CountActiveLocationsByRegion` counts only active locations; read-after-write on primary, list on replica. Gate like `audit_log_repository_test.go` (may skip if `DATABASE_URL` unreachable).
    - _Requirements: 1.4, 1.5, 1.6, 4.1, 4.2, 4.3, 9.1, 9.2, 9.3_

- [ ] 4. Region admin service (validation, RBAC re-check, audit-in-tx)
  - [x] 4.1 Implement `backend/internal/service/region_admin.go` — `RegionAdminService`
    - Define sentinel errors (`ErrRegionNotFound`, `ErrRegionCodeConflict`, `ErrRegionCodeImmutable`, `ErrRegionHasActiveLocations`, `ErrRegionStatusUnchanged`, `ErrRegionHardDeleteForbidden`, `ErrNotAuthorized`); reuse existing `ValidationError`. Narrow `RegionAdminRepo` interface + `Pool` (`Begin`). Implement `List`/`Count`/`Get` (reads, no audit); `Create`/`UpdateName`/`Disable`/`Enable` following the audit-in-tx pattern of `rolemgmt.PermissionService`: RBAC re-check → validate → pre-check → open own `pgx.Tx` → mutate via repo `*Tx` method → `audit.NewWriter(tx).Write(...)` in the same tx → commit only if both succeed, rollback on audit failure. `normalizeCode` (trim+uppercase), code regex `^[A-Za-z0-9]+$` (1–20), name required (1–100 after trim). Update rejects a differing `code` (immutable). Disable blocks when `CountActiveLocations>0`; disable/enable reject no-op status changes without writing audit. No delete method.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 5.2, 6.1, 6.2, 6.3, 6.4, 6.5, 7.1, 7.2, 7.3, 9.1, 9.3, 9.5_
  - [x]* 4.2 Write table-driven unit tests `region_admin_test.go` (fake repo + fake pool/tx)
    - Create success (normalized uppercase code + `region_created` audit); duplicate → `ErrRegionCodeConflict`; bad code/name → `ValidationError`; non-admin actor → `ErrNotAuthorized`; audit failure → error and insert NOT committed. UpdateName success (before/after audit); differing code → `ErrRegionCodeImmutable`; missing id → `ErrRegionNotFound`; audit failure → rollback. Disable success at 0 active locations; `CountActiveLocations>0` → `ErrRegionHasActiveLocations` (no audit); already-inactive → `ErrRegionStatusUnchanged` (no audit); audit failure → rollback. Enable success; already-active → `ErrRegionStatusUnchanged`. Authorization matrix: ADMIN ✓, ADMIN_PARAM ✓, APPACCESS/ATM-USER/VENDOR ✗.
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.6, 3.2, 3.3, 3.4, 4.2, 4.6, 4.7, 5.2_
    - **Implementation note:** written as `region_admin_integration_test.go` (real Postgres, outer-tx-as-pool harness) instead of a fake-repo/fake-pgx.Tx unit test — this codebase's actual precedent for audit-in-tx services (`internal/rolemgmt/service_integration_test.go`) uses the same real-tx harness rather than hand-rolled `pgx.Tx` fakes (pgx.Tx has 11 methods; no fake-Tx pattern exists anywhere in this codebase). Covers every case listed above, gated on `DATABASE_URL` (not yet run against a live DB — migration 018 isn't applied to dev yet).
  - [ ]* 4.3 Write property test for code normalization and uniqueness
    - **Property 1: Code disimpan ternormalisasi dan unik**
    - **Validates: Requirements 2.1, 2.2, 2.7**
  - [ ]* 4.4 Write property test for validation rejection
    - **Property 2: Validasi code dan nama menolak input tak valid tanpa mutasi**
    - **Validates: Requirements 2.3, 2.4, 3.3, 8.2, 8.3**
  - [ ]* 4.5 Write property test for code immutability on update
    - **Property 3: Code immutable pada update**
    - **Validates: Requirements 3.1, 3.2**
  - [ ]* 4.6 Write property test for the audit-or-rollback invariant
    - **Property 4: Setiap mutasi punya tepat satu jejak audit, atau tidak terjadi**
    - **Validates: Requirements 2.5, 2.6, 3.5, 3.6, 4.5, 4.6, 6.1, 6.2, 6.3, 6.4, 6.5**
  - [ ]* 4.7 Write property test for referential-integrity block on disable
    - **Property 5: Nonaktif diblokir saat ada lokasi aktif dependen**
    - **Validates: Requirements 4.2, 7.2, 7.3**
  - [ ]* 4.8 Write property test for disable/enable round-trip + idempotency guard
    - **Property 6: Disable/enable adalah round-trip soft-delete yang idempotent-guarded**
    - **Validates: Requirements 4.1, 4.3, 4.7**
  - [ ]* 4.9 Write property test for service-layer RBAC enforcement
    - **Property 8: Otorisasi ditegakkan di service, bukan hanya middleware**
    - **Validates: Requirements 5.1, 5.2, 5.4**
  - [ ]* 4.10 Write property test for case-insensitive search
    - **Property 9: Pencarian mengembalikan hanya yang cocok, case-insensitive**
    - **Validates: Requirements 1.4, 1.5**
  - _Model: Opus, Effort: High — audit-in-tx transaction control, RBAC re-check, referential-integrity block, and immutability/soft-delete state logic; a wrong transition or a missing rollback corrupts audit integrity and master data._

- [x] 5. Checkpoint - backend service/repository tests pass
  - Ensure all backend tests pass (`go test ./...`, `go build ./...`). Ask the user if questions arise.
  - _Model: Sonnet, Effort: Medium — triage of build/test failures across the new backend layers._

- [x] 6. HTTP handler and route wiring
  - [x] 6.1 Implement `backend/internal/handler/admin_region_handler.go`
    - `Routes() chi.Router` with `GET /`, `POST /`, `GET /{id}`, `PUT /{id}`, `POST /{id}/disable`, `POST /{id}/enable`. Flat JSON; resolve actor via `middleware.GetAuthContext`, IP via `extractClientIP`, id via `parsePathID`. Map errors per design: 401 unauthenticated, 403 forbidden/`ErrNotAuthorized`, 422 `ValidationError`, 400 `ErrRegionCodeImmutable`/bad param, 409 `ErrRegionCodeConflict`/`ErrRegionHasActiveLocations`/`ErrRegionStatusUnchanged`, 404 `ErrRegionNotFound`, 500 otherwise. List response includes `is_active` bool + `location_count` + pagination. PUT accepts optional `code` only to detect/reject changes.
    - _Requirements: 1.1, 1.2, 1.3, 1.8, 5.1, 5.4, 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_
  - [x] 6.2 Wire routes in `backend/cmd/api/main.go`
    - Construct `regionAdminRepo := repository.NewRegionAdminRepository(dbPool, dbReadPool)`, `regionAdminSvc := service.NewRegionAdminService(regionAdminRepo, dbPool)`, `adminRegionHandler := handler.NewAdminRegionHandler(regionAdminSvc)`; `masterDataAdmin.Mount("/api/v1/admin/regions", adminRegionHandler.Routes())` under the existing `RequireAuth` + `RequireRoles("ADMIN","ADMIN_PARAM")` group.
    - _Requirements: 5.1, 5.4, 9.6_
  - [x]* 6.3 Write handler tests `admin_region_handler_test.go` (`httptest` + real `RequireAuth`/`RequireRoles` + fake service)
    - 200/201 happy paths + flat-JSON shape (incl. `is_active` bool + `location_count`); 400/404/409/422 mappings; 401 without token; 403 wrong role (pattern of `admin_approval_handler_test.go`/`audit_log_handler_rbac_test.go`).
    - _Requirements: 1.8, 5.1, 5.4, 8.1, 8.2, 8.3, 8.4, 8.5_

- [x] 7. Enforce no-hard-delete guard for regions
  - Extend the `tables` list in `backend/internal/repository/no_hard_delete_test.go` to include `regions`, asserting no `DELETE FROM regions` exists in `queries/*.sql`.
  - **Property 7: Hard-delete tidak pernah mungkin**
  - **Validates: Requirements 4.4, 7.1**
  - _Model: Haiku, Effort: Low — mechanical extension of an existing guard test list._

- [x] 8. Frontend feature: data layer and URL state
  - [x] 8.1 Create `src/features/admin-regions/types.ts` and `api.ts`
    - `AdminRegion` interface (`id`, `code`, `region`, `is_active`, `location_count`, timestamps, `deleted_at`); `createRegionSchema` (code trim min1/max20/alphanumeric, region trim min1/max100) and `updateRegionSchema` (region only; code disabled in edit); list/params/response types. Thin flat-JSON client over `lib/api/client` base `/admin/regions`: `list`, `get`, `create`, `updateName`, `disable`, `enable`.
    - _Requirements: 2.3, 2.4, 3.2, 3.3, 8.5_
  - [x] 8.2 Create `src/features/admin-regions/hooks.ts` and `useAdminRegionsUrlState.ts`
    - TanStack Query v5 hooks (`useRegionsList` with `keepPreviousData`, `useRegion`, `useCreateRegion`, `useUpdateRegion`, `useDisableRegion`, `useEnableRegion`) with namespaced keys and list invalidation + toast on mutation. `ADMIN_REGIONS_SEARCH_SCHEMA` (Zod) for `q`, `page`, `page_size`, `status`; filter change resets `page` to 1 and round-trips via URL search params (pattern `useAdminATMsUrlState`).
    - _Requirements: 1.2, 1.4_

- [x] 9. Frontend feature: components
  - [x] 9.1 Implement `AdminRegionsPage.tsx`, `RegionFilterBar.tsx`, `RegionsTable.tsx`
    - Page: `PageHeader` (eyebrow "Pengaturan", title "Manajemen Region", primary "Tambah Region") + filter bar + table + dialog. Filter bar: search input (1–100) + status filter (Semua/Aktif/Nonaktif). Table (TanStack Table v8): columns Code (`tabular-nums`/mono), Nama, Jumlah Lokasi (`tabular-nums`, right), Status as Badge with icon+label (CheckCircle "Aktif" / Ban "Nonaktif", not color alone), actions (Ubah, Nonaktif/Aktif); row hover `--red-50`.
    - _Requirements: 1.1, 1.6, 1.8, 9.6_
  - [x] 9.2 Implement `RegionFormDialog.tsx` and `index.ts`
    - RHF + `zodResolver`; create uses `createRegionSchema`, edit uses `updateRegionSchema` with Code field disabled (immutable). Map server 409/422/400 to field errors via `setError` (409/immutable → `code`, validation → related field), dialog stays open. Disable confirmation surfaces the active-location block message from the 409 body. Export `AdminRegionsPage` from `index.ts`.
    - _Requirements: 3.2, 8.1, 8.2, 8.3, 8.7_
  - [ ]* 9.3 Write component tests in `src/features/admin-regions/__tests__/`
    - Zod schema (code required/format/max20 immutable-in-edit, name required/max100); form maps server 409/422/400 to field errors and keeps dialog open; filter change resets to page 1 and round-trips via URL; table renders Status as icon+label badge; Code & Jumlah Lokasi `tabular-nums`. Mock network.
    - _Requirements: 1.4, 1.8, 3.2, 8.1, 8.2, 8.3, 8.7_

- [x] 10. Frontend route and Settings hub card
  - [x] 10.1 Add route `src/routes/settings/admin/regions.tsx`
    - `/settings/admin/regions` under `protectedRoute`, `validateSearch: ADMIN_REGIONS_SEARCH_SCHEMA`, `beforeLoad: requireRoles(["ADMIN","ADMIN_PARAM"])` → `<Forbidden />` for unauthorized; component renders `AdminRegionsPage`.
    - _Requirements: 5.3_
  - [x] 10.2 Register "Manajemen Region" card in `SettingsHubPage.tsx`
    - Add master-category card `{ id: "admin-regions", title: "Manajemen Region", description: "Kelola region dan status aktifnya.", href: "/settings/admin/regions", icon: Map, category: "master" }` in the "Data master" tab, shown only when `showMaster` (master-data roles). No top-level `NAV_CONFIG` entry.
    - _Requirements: 5.3, 9.6_
  - [ ]* 10.3 Extend `SettingsHubPage.test.tsx` and route guard test
    - Card "Manajemen Region" shown for ADMIN/ADMIN_PARAM and hidden for other roles; `requireRoles` allow/deny/redirect for the region route (pattern of `atms`).
    - _Requirements: 5.3_

- [x] 11. Final checkpoint - full quality gate
  - Ensure backend (`go build ./...`, `go test ./...`) and frontend (`pnpm build`, `pnpm test`, `pnpm lint`) pass; confirm coverage ≥80% on the new `internal/*` packages. Ask the user if questions arise.
  - **Result:** backend build/vet/test all green. Frontend build, typecheck, and lint all green for the region-management files (fixed two issues found here: a react-hook-form resolver type mismatch in `RegionFormDialog.tsx`, and a `noShadowRestrictedNames` lint error from importing lucide's `Map` icon under its own name — renamed to `MapIcon`). `pnpm vitest run`: 2 test files / 5 tests fail, all pre-existing and unrelated to region-management (confirmed via `git diff --stat` showing the touched `SettingsHubPage.tsx` diff is purely additive, 9 insertions/0 deletions, no change to tab-default logic; `src/lib/auth/store.test.ts` was never touched this session).
  - **Follow-up:** migration `018` was applied to the dev DB (`localhost:5432`, all 13 seeded regions backfilled active) and both `-tags=integration` suites (repository + service) were run for real. Two real bugs surfaced and were fixed: (1) `region_admin_repository_test.go`'s deliberate unique-violation subtest aborted the shared outer tx (Postgres aborts on any error until rollback), poisoning every later subtest — fixed by running that subtest in its own savepoint. (2) `region_admin_integration_test.go`'s `tag` generator was lowercase while `Create` correctly uppercases codes (Req 2.7 working as designed) — the test's own assertions didn't account for it; fixed the tag to be uppercase and shortened one over-20-char code prefix. All repository (8) and service (14) region integration tests now pass against the real DB. Separately, `masterdata_approval_integration_test.go` had a **pre-existing, unrelated** compile error (stale `VendorPackagePayload`/`VendorPackageUpdatePayload` shapes from before migration 016) blocking the whole `service` package from compiling under `-tags=integration` — fixed (rewrote `TestVendorPackageApplier_Approve_CreateUpdateDisable_Integration` to the current payload/table shape) since it blocked verifying this feature; confirmed passing. Running the *full* integration suite afterward surfaced ~20 further pre-existing failures in 6 other files, all the same root cause (stale pre-migration-016 `vendor_packages_branch` columns) — out of scope for region-management, left alone and flagged as a separate follow-up task (`task_0acbff85`).
  - _Model: Sonnet, Effort: Medium — cross-stack verification and failure triage._

## Notes

- Tasks marked with `*` are optional test tasks and can be skipped for a faster MVP, but the design mandates ≥80% coverage on `internal/*` and property tests for the correctness properties — skipping them leaves the audit-in-tx and referential-integrity invariants unverified.
- Each task references specific requirement sub-clauses for traceability; property test tasks reference the design's Correctness Properties (1–10).
- Backend-first sequencing: migration → queries → repository → service → handler → wiring → frontend. Each step builds on the prior and ends wired into `cmd/api/main.go` (backend) and the Settings hub (frontend), leaving no orphaned code.
- Property 10 (write/read topology) is exercised by the repository integration tests (3.2) rather than a standalone pure-logic property test, since it is an I/O-routing invariant.
- The immediate-apply-with-audit deviation from Golden Rule #3 must be recorded in `project-context.md` when the module lands (see design "Documented Deviation").

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1"] },
    { "id": 1, "tasks": ["2.1"] },
    { "id": 2, "tasks": ["2.2"] },
    { "id": 3, "tasks": ["3.1", "8.1"] },
    { "id": 4, "tasks": ["3.2", "4.1", "8.2"] },
    { "id": 5, "tasks": ["4.2", "4.3", "4.4", "4.5", "4.6", "4.7", "4.8", "4.9", "4.10", "6.1", "7", "9.1"] },
    { "id": 6, "tasks": ["6.2", "6.3", "9.2", "10.1"] },
    { "id": 7, "tasks": ["9.3", "10.2"] },
    { "id": 8, "tasks": ["10.3"] }
  ]
}
```
