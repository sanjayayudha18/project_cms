# Implementation Plan: Vendor Branch ATMs

## Overview

This plan implements a read-only "ATM" sub-tab on the vendor branch detail page. It is purely additive: one backend read endpoint (SQL → generated → repository → service → handler → mount) and one frontend panel wired into the existing sub-tab shell. There is no migration and no new index — the join-covering indexes already exist.

Tasks are ordered so each builds on the previous: the SQL query pair comes first, then its generated Go, then the repository that wraps it, then the service (the testable core — dedup, vendor-scope, pagination) with its property/example tests, then the handler with httptest tables, then the mount that wires it into the running server. The frontend follows (types → api → hook → panel → sub-tab wiring → component tests), and a single real-Postgres integration test closes the loop on the SQL behavior.

All backend code mirrors the existing admin vendor package feature (flat JSON, `parsePageParams`, `parsePathID`, `writeError`, `dbReadPool` replica). All frontend code mirrors `VaultsPanel` and `useVendorPackages`.

## Tasks

- [ ] 1. Add SQL query pair for branch-managed ATMs
  - Create `backend/queries/vendor_branch_atms.sql`.
  - `ListBranchATMs` (:many): the `DISTINCT ON (a.id)` dedup subquery joining `atm_vendor_packages` → `vendor_packages` → `atms` LEFT JOIN `locations`, filtered by `vendor_branch_id` and the active-assignment window (`is_active = true AND effective_start_date <= CURRENT_DATE AND (effective_end_date IS NULL OR effective_end_date >= CURRENT_DATE)`), inner `ORDER BY a.id, avp.effective_start_date DESC, avp.id DESC` for a deterministic package-code pick; outer query `ORDER BY terminal_id ASC` with `LIMIT`/`OFFSET`.
  - `CountBranchATMs` (:one): `COUNT(DISTINCT a.id)` over the same branch + active-assignment filter.
  - `GetVendorBranchVendorID` (:one): `SELECT vendor_id FROM vendor_branches WHERE id = $1` — reuse the existing definition if already present in `backend/queries/`; only add it if missing (do not duplicate).
  - _Requirements: 1.1, 1.2, 1.3, 4.1, 4.2, 4.3_
  - _Model: Opus, Effort: High — the DISTINCT ON dedup reconciled with terminal_id ordering via a subquery, plus the COUNT(DISTINCT) that must never exceed returned rows, is the correctness heart of the feature; a wrong join or filter silently returns wrong ATMs._

- [ ] 2. Generate (or hand-write) the db code for the new queries
  - Attempt `sqlc generate` first. If it succeeds, commit the produced `backend/internal/db/vendor_branch_atms.sql.go`.
  - If `sqlc generate` is blocked by the known pre-existing migration bug, hand-write `backend/internal/db/vendor_branch_atms.sql.go` to match sqlc's exact output convention — same package, `pgtype` imports, `ListBranchATMsParams`/`ListBranchATMsRow` structs, `CountBranchATMs`, `GetVendorBranchVendorID`, and `q.db.Query`/`q.db.QueryRow` bodies — mirroring the style of the existing `vendor_branches_admin.sql.go`.
  - Ensure the `Row` struct models nullable columns (`location_name`, `location_city_or_regency`, `priority_class`) with the same nullable `pgtype`/pointer style the existing generated code uses.
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_
  - _Model: Opus, Effort: High — hand-writing generated code must match sqlc's convention exactly (types, nullable handling, query bodies) or the repository/service above it silently mis-maps columns; if generation is blocked, this is delicate mechanical-but-unforgiving work._

- [ ] 3. Implement the branch ATM repository
  - Create `backend/internal/repository/branch_atm_repository.go` with `BranchATMRepository` wrapping `db.New(dbConn)` and `NewBranchATMRepository(dbConn db.DBTX)`.
  - `List(ctx, arg db.ListBranchATMsParams) ([]db.ListBranchATMsRow, error)`, `Count(ctx, branchID int64) (int64, error)`.
  - `BranchVendorID(ctx, branchID int64) (*int64, error)` mapping `pgx.ErrNoRows` → `(nil, nil)`, exactly as `VendorPackageAdminRepository.BranchVendorID` does.
  - _Requirements: 1.3, 1.4, 1.5_
  - _Model: Sonnet, Effort: Medium — thin wrapper over generated queries following an existing repository pattern; the only judgment is the ErrNoRows→nil mapping, which is copied from precedent._

- [ ] 4. Implement the branch ATM service
  - [ ] 4.1 Implement `BranchATMService` with vendor-scope enforcement, dedup-aware mapping, and pagination
    - Create `backend/internal/service/branch_atm.go`: `ErrBranchNotFound`, `BranchATMRepo` interface, `ManagedATM` DTO (pointer fields for nullable `LocationName`, `LocationCityOrRegency`, `PriorityClass`), `ListManagedATMsResult{ATMs, Total}`, `NewBranchATMService`.
    - `List(ctx, vendorID, branchID, pageLimit, pageOffset int64)`: call `repo.BranchVendorID(branchID)`; `nil` → `ErrBranchNotFound` (Req 1.4); non-nil and `!= vendorID` → `ErrBranchNotFound` (Req 1.5); `== vendorID` → call `List` + `Count`, map rows to `ManagedATM` (nullable columns → nil DTO fields), return `ListManagedATMsResult`.
    - _Requirements: 1.1, 1.2, 1.4, 1.5, 1.6, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 4.1, 4.2, 4.3_
    - _Model: Opus, Effort: High — the service is the authorization boundary (defense-in-depth vendor scope) and the dedup/pagination contract; conflating "another vendor's branch" with 403 instead of 404, or mis-mapping nullable columns, is a correctness/security bug._

  - [ ]* 4.2 Write property test — one row per managed ATM (deduplication)
    - **Feature: vendor-branch-atms, Property 1: For any vendor branch and any set of active ATM_Assignments — including an ATM linked by multiple active assignments to packages of that same branch — the full (unpaginated) result of BranchATMService.List contains each managed ATM's atm_id at most once.**
    - Implement `fakeBranchATMRepo` (in-memory branches/packages/assignments/ATMs applying the same active-filter, dedup, ordering, pagination as the SQL) in `backend/internal/service/branch_atm_test.go`; generators produce ATMs with 1..N assignments; min 100 iterations.
    - **Validates: Requirements 1.1, 1.2**
    - _Model: Opus, Effort: High — the repository fake must faithfully replicate the SQL's dedup/filter semantics or the property is vacuous; getting the fake right is subtle._

  - [ ]* 4.3 Write property test — total equals the distinct managed-ATM count
    - **Feature: vendor-branch-atms, Property 2: For any vendor branch, the Total returned by BranchATMService.List equals the number of distinct ATMs managed by that branch under the active-assignment filter, and is independent of the requested page and page size.**
    - Reuse `fakeBranchATMRepo`; assert `Total` invariant across varying page/page_size; min 100 iterations.
    - **Validates: Requirements 4.3, 1.2**
    - _Model: Opus, Effort: High — reasoning about the invariant that Total is page-independent and matches distinct-count requires care in the generator design._

  - [ ]* 4.4 Write property test — pagination partitions the ordered result
    - **Feature: vendor-branch-atms, Property 3: For any vendor branch and any page size within limits, concatenating the atm_id sequences of successive pages reproduces the full deduplicated result exactly once each, in terminal_id ascending order, and no page returns more rows than the effective page size.**
    - Reuse `fakeBranchATMRepo`; generators cover page sizes 1, default 25, >100 (clamp), and past-the-end pages; min 100 iterations.
    - **Validates: Requirements 4.1, 4.2, 4.4, 4.5, 4.6**
    - _Model: Opus, Effort: High — page-boundary and clamping edge cases (empty tail page, size>max, ordering across page seams) are exactly what property tests must reason through._

  - [ ]* 4.5 Write property test — vendor scope enforced independent of branch existence
    - **Feature: vendor-branch-atms, Property 4: For any (vendorId, branchId) pair, BranchATMService.List returns ErrBranchNotFound whenever the branch does not exist, or exists but its vendor_id differs from vendorId; and returns a (possibly empty) result only when the branch exists and belongs to vendorId.**
    - Reuse `fakeBranchATMRepo`; generators produce branches owned by varying vendors plus nonexistent branch IDs; min 100 iterations.
    - **Validates: Requirements 1.4, 1.5**
    - _Model: Opus, Effort: High — this is the security-relevant scope boundary; the generator must cover nonexistent, same-vendor, and cross-vendor cases distinctly._

  - [ ]* 4.6 Write property test — empty branch yields empty page and zero total
    - **Feature: vendor-branch-atms, Property 5: For any existing branch owned by the requested vendor that has no active managed ATMs, BranchATMService.List returns an empty ATM slice and a Total of zero.**
    - Reuse `fakeBranchATMRepo`; generators produce owned branches with zero active assignments (including branches whose only assignments are ended); min 100 iterations.
    - **Validates: Requirement 1.6**
    - _Model: Sonnet, Effort: Medium — a focused invariant on an existing fake; less branching than Properties 1–4._

  - [ ]* 4.7 Write example/unit tests for the service
    - Cross-vendor branch → `ErrBranchNotFound`; nonexistent branch → `ErrBranchNotFound`; null location and null priority_class map to nil DTO fields; deterministic package-code tiebreaker.
    - _Requirements: 1.4, 1.5, 2.6_
    - _Model: Sonnet, Effort: Medium — concrete example assertions complementing the properties; standard table-driven unit work._

- [ ] 5. Checkpoint - Ensure service and query layer build and tests pass
  - Ensure the backend builds and all service tests pass, ask the user if questions arise.
  - _Model: Sonnet, Effort: Medium — triaging build/test failures across the SQL→db→repo→service stack needs judgment but not deep reasoning._

- [ ] 6. Implement the branch ATM handler
  - [ ] 6.1 Implement `AdminBranchATMHandler` (read-only List)
    - Create `backend/internal/handler/admin_branch_atm_handler.go`: `BranchATMServicer` interface, `AdminBranchATMHandler`, `NewAdminBranchATMHandler`, `Routes()` mounting `r.Get("/", h.List)`.
    - `List`: `parsePathID(r, "vendorID")` + `parsePathID(r, "branchID")` → 400 on malformed; `parsePageParams(q)` (default 1/25, clamp 100) → 400 on invalid; call `svc.List(ctx, vendorID, branchID, pageSize, (page-1)*pageSize)`; on success `writeJSON(w, 200, {"atms":..., "page":..., "page_size":..., "total":...})` (flat JSON, no envelope).
    - `handleError`: `errors.Is(err, service.ErrBranchNotFound)` → 404 `not_found`; default → `writeUnexpectedError` (500), matching `admin_vendor_package_handler.go`.
    - _Requirements: 1.4, 1.5, 1.6, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_
    - _Model: Sonnet, Effort: Medium — structurally mirrors an existing vendor-scoped read handler; parsing, error mapping, and flat-JSON shape are established patterns._

  - [ ]* 6.2 Write table-driven httptest tests for the handler
    - Cases: 401 (no auth context), 403 (wrong role via mounted middleware), 400 (bad path id / bad page_size), 404 (service returns `ErrBranchNotFound`), 200 empty (`atms: []`, `total: 0`), 200 with rows (field mapping + flat JSON shape, no envelope, page/page_size echoed and clamped).
    - _Requirements: 3.1, 3.2, 3.3, 1.4, 1.6, 4.3, 4.5, 4.6_
    - _Model: Sonnet, Effort: Medium — comprehensive but conventional httptest table; the 401/403 cases need the middleware wired in the test harness._

- [ ] 7. Mount the endpoint in the API server
  - In `backend/cmd/api/main.go`, next to the other `/api/v1/admin/vendors/{vendorID}/branches/...` mounts: construct `branchATMRepo := repository.NewBranchATMRepository(dbReadPool)` (replica per Req 1.3), `branchATMService := service.NewBranchATMService(branchATMRepo)`, `adminBranchATMHandler := handler.NewAdminBranchATMHandler(branchATMService)`, and `masterDataAdmin.Mount("/api/v1/admin/vendors/{vendorID}/branches/{branchID}/atms", adminBranchATMHandler.Routes())` under the `masterDataAdmin` group that already applies `RequireAuth + RequireRoles("ADMIN", "ADMIN_PARAM")`.
  - _Requirements: 1.3, 3.1, 3.2, 3.3_
  - _Model: Sonnet, Effort: Medium — wiring into an existing router group; choosing dbReadPool (not primary) and the correct guarded group is the one decision, and it follows precedent._

- [ ] 8. Checkpoint - Ensure backend builds and all backend tests pass
  - Ensure the backend compiles with the new mount and all handler/service tests pass, ask the user if questions arise.
  - _Model: Sonnet, Effort: Medium — build/test triage across the full backend slice._

- [ ] 9. Add frontend types and API client function
  - [ ] 9.1 Add types to `frontend/CompanyPortal-Vite/src/features/admin-vendors/types.ts`
    - `ManagedATM` (`atm_id`, `terminal_id`, `location_name: string|null`, `location_city_or_regency: string|null`, `priority_class: string|null`, `is_active`, `package_code`), `BranchATMsListParams` (`page`, `page_size`), `BranchATMsListResponse` (`atms`, `page`, `page_size`, `total`).
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 4.3_
    - _Model: Haiku, Effort: Low — pure type declarations mirroring the endpoint contract; one obvious shape._

  - [ ] 9.2 Add `listBranchATMs` to `frontend/CompanyPortal-Vite/src/features/admin-vendors/api.ts`
    - Mirror `listVendorPackages`: build `URLSearchParams` for `page`/`page_size`, `api.get<BranchATMsListResponse>(\`${BASE}/${vendorId}/branches/${branchId}/atms?...\`)`, return `data`.
    - _Requirements: 5.2_
    - _Model: Haiku, Effort: Low — a single fetch function copied from an existing sibling with the path changed._

- [ ] 10. Add the `useBranchATMs` hook
  - In `frontend/CompanyPortal-Vite/src/features/admin-vendors/hooks.ts`, mirror `useVendorPackages`: `useQuery` with key `["admin-vendors", "children", vendorId, "branches", branchId, "atms", params]` (under the existing children prefix so `useInvalidateList` covers it), `queryFn: () => listBranchATMs(...)`, `placeholderData: keepPreviousData`.
  - _Requirements: 5.2_
  - _Model: Haiku, Effort: Low — one hook copied from an existing sibling; the only choice is the query-key prefix, which follows convention._

- [ ] 11. Implement the ATMsPanel component
  - Create `frontend/CompanyPortal-Vite/src/features/admin-vendors/components/ATMsPanel.tsx` mirroring `VaultsPanel` but read-only (no create/edit/disable, no confirm dialog).
  - Use `DataTable` + `Badge` + `lucide-react` icons. Columns: Terminal ID (`tabular-nums`), Lokasi (name + city, `—` placeholder when `location_name` is null), Priority_Class, Kode Paket, Status (success badge + icon + text for active, danger badge + icon + text for inactive — never color alone).
  - Wire `useBranchATMs(vendorId, branchId, params)` with local pagination state; render loading indicator, error message, and empty-state ("branch has no managed ATMs") states as in `VaultsPanel`.
  - _Requirements: 5.3, 5.4, 5.5, 5.6, 6.1, 6.2, 6.3, 6.4_
  - _Model: Sonnet, Effort: Medium — real component work with several states and design-system rules (badge icon+text, tabular-nums, null placeholder), but a close mirror of an existing panel._

- [ ] 12. Wire the ATM sub-tab into the branch detail page
  - In `VendorBranchDetailPage.tsx`, append `{ id: "atms", label: "ATM" }` to `BRANCH_SUB_TABS` (after the existing Vault/PIC/Paket/Harga Paket entries) and render `{subTab === "atms" && <ATMsPanel vendorId={vendorId} branchId={branchId} />}`.
  - _Requirements: 5.1, 5.2_
  - _Model: Haiku, Effort: Low — adding one tab entry and one conditional render to an existing tab shell._

- [ ]* 13. Write frontend component tests for ATMsPanel
  - `ATMsPanel.test.tsx` (RTL + MSW): renders rows on success; terminal_id column uses `tabular-nums` (6.3); active → success badge with icon + label, inactive → danger badge with icon + label, asserting both icon and text present (6.1, 6.2); null location → placeholder marker not empty cell (6.4); loading indicator (5.3); zero ATMs → empty-state message (5.5); request failure → error message (5.6); "ATM" sub-tab appears and, when selected, triggers the fetch (5.1, 5.2, 5.4).
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 6.1, 6.2, 6.3, 6.4_
  - _Model: Sonnet, Effort: Medium — thorough RTL+MSW suite covering all render states and design-system assertions; conventional but broad._

- [ ]* 14. Write the integration test against real Postgres
  - Backend repo/API integration test (real Postgres, seeded fixtures, 1–3 representative cases): seed a branch with two packages and one ATM assigned via both (assert dedup → one row, correct `package_code`); one ATM with an ended assignment (assert excluded by the active filter); one ATM under a different vendor's branch (assert not returned / 404 for cross-vendor path). Confirms the join, `DISTINCT ON`, `COUNT(DISTINCT ...)`, and `terminal_id` ordering. Not run 100×.
  - _Requirements: 1.1, 1.2, 1.5, 4.1, 4.3_
  - _Model: Opus, Effort: High — validating the real SQL dedup/filter/ordering against a live DB with careful fixture seeding is where SQL-level correctness is actually proven._

- [ ] 15. Final checkpoint - Ensure all tests pass
  - Ensure all backend and frontend tests pass, ask the user if questions arise.
  - _Model: Sonnet, Effort: Medium — final cross-stack build/test triage._

## Notes

- Tasks marked with `*` are optional (tests) and can be skipped for a faster MVP, but the design's Correctness Properties (Properties 1–5) and the integration test are the real safeguards for the dedup/scope/pagination logic — skipping them is not recommended for a feature touching vendor-scope authorization.
- Each task references specific granular requirements for traceability.
- Checkpoints (tasks 5, 8, 15) ensure incremental validation at the service, backend, and full-stack boundaries.
- No migration or new index: the join-covering indexes (`atm_vendor_packages_vendor_package_idx`, `atm_vendor_packages_atm_idx`, `vendor_packages_vendor_branch_idx`) already exist per the design.
- Property tests use a repository fake (`fakeBranchATMRepo`) and a Go property-based library (`pgregory.net/rapid` or the repo's existing choice — do not hand-roll); the integration test uses real Postgres for the SQL-level behavior the fake cannot prove.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1", "9.1"] },
    { "id": 1, "tasks": ["2", "9.2", "12"] },
    { "id": 2, "tasks": ["3", "10"] },
    { "id": 3, "tasks": ["4.1", "11"] },
    { "id": 4, "tasks": ["4.2", "4.3", "4.4", "4.5", "4.6", "4.7", "13"] },
    { "id": 5, "tasks": ["6.1"] },
    { "id": 6, "tasks": ["6.2", "7"] },
    { "id": 7, "tasks": ["14"] }
  ]
}
```
