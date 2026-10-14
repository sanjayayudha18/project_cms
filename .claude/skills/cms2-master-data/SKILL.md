---
name: cms2-master-data
description: Checklist for adding or changing a CMS2 master-data entity behind the maker-checker engine (queries → read-only repo → service Submit → Applier → main.go registry → handler 202 → tests → frontend). Use when touching vendors, vendor_branches, vendor_vaults, vendor_pics, vendor_packages_branch, vendor_package_prices, atms, atm_vendor_packages, dsr_location_vault_maps, or adding a new master-data table/screen.
---

# CMS2 master-data maker-checker

Rule (CLAUDE.md §5, §12 D1–D4): a master-data create/update/disable/enable never
writes the entity directly. It is staged as a `master_data_change_requests` row +
`approval_requests`, the endpoint answers **202**, and the row changes only when a
checker approves — via the entity's `Applier`, in one tx with the audit entry.

**Not this pattern** (documented deviations — don't copy them here): `admin/users`,
Role Management, Area ACM admin (apply immediately, audit in same tx). Vendor
Request / vault plans / vendor parties have their own state machines.

**Before starting**: a new entity or column is a schema change → full SDLC chain
(`.claude/sdlc/<feature>/`, CLAUDE.md §4a). Ask the user first.

## Reference implementation

Smallest complete example — copy its shape: `dsr_location_map` (migration 027).

| Layer | File |
|---|---|
| SQL | `backend/queries/dsr_location_maps_admin.sql` |
| Repo (read-only) | `backend/internal/repository/dsr_location_map_admin_repository.go` |
| Service (validate + Submit) | `backend/internal/service/dsr_location_map_admin.go` |
| Applier | `backend/internal/service/masterdata_applier_dsr_location_map.go` |
| Registry + route | `backend/cmd/api/main.go` (`masterDataApplierRegistry`, `masterDataAdmin.Mount`) |
| Handler | `backend/internal/handler/admin_dsr_location_map_handler.go` |
| Tests | `*_test.go` next to each, + `masterdata_applier_dsr_location_map_integration_test.go` |

Engine (don't change unless asked): `masterdata_change.go` (`Submit`, `SubmitRequest`),
`masterdata_applier.go` (`Applier`, `ApplierRegistry`), `masterdata_approval.go`
(apply-on-approve, staleness check, audit).

## Checklist

### 1. Queries — `backend/queries/<entity>_admin.sql`
- `List…Admin :many` with `status` = `active|disabled|all`.
- `Get…AdminByID :one` **including disabled rows**. This one query is both the
  service's `Before` snapshot and the applier's `CurrentState` — same shape, or
  every update goes stale.
- Pre-check reads the service needs (uniqueness, ownership, FK still active).
- Mutations `Create… :one RETURNING`, `Update… :one RETURNING`, `Disable… :exec`,
  `Enable… :exec` — called **only** from the applier.
- Re-guard FK/ownership inside the INSERT/UPDATE (`INSERT … SELECT … WHERE active`):
  state can change between submit and approve; no row → applier error → 409.
- Disable = `is_active=false, deleted_at=now()`. **Never `DELETE FROM`**; add the
  table to both lists in `internal/repository/no_hard_delete_test.go`.
  Effective-dated tables (`vendor_packages_branch`, `vendor_package_prices`) close
  `effective_end_date` instead.
- Then `cd backend && sqlc generate` (v1.31.1) → fix `UserLeafe`→`UserLeave` →
  `git diff --stat internal/db/` shows only your files.

### 2. Repository — read-only
- `New…Repository(dbConn, readConn db.DBTX)`: list screens on `readQueries`
  (replica); submit-time lookups on `queries` (primary — read-after-write).
- `pgx.ErrNoRows` → `nil, nil`. No mutation methods.

### 3. Service — validate, then `Submit`
- Depend on a small interface (`VendorVaultSubmitter` already exists: just `Submit`).
- Normalise input (trim), force path-scoped ids (`req.VendorID = vendorID`),
  return `&ValidationError{Field, Message}` (Indonesian messages) for 422.
- `load()` treats another parent's id as **not found** (no cross-vendor leaks).
- Sentinel errors `Err<Entity>NotFound` / `Err<Entity>Conflict`.
- `Submit(ctx, makerID, SubmitRequest{EntityType, Op, EntityID, Payload, Before}, actorIP)`:
  create → `EntityID=nil`, no `Before`; update/disable/enable → `EntityID` + `Before`
  from `Get…AdminByID`. `Submit` itself re-checks `ADMIN`/`ADMIN_PARAM`, rejects a
  second pending change on the same entity (`ErrMasterDataChangePending`).
- Server-generated values (e.g. `vendor_package_prices.package_code`) are created
  at **apply** time, never taken from the client.

### 4. Applier — mechanical write only
- `type <Entity>Applier struct{}` with `Apply(ctx, tx, change)` switching on
  `change.Op` and `CurrentState(ctx, tx, id)` = `Get…AdminByID`.
- Unmarshal `change.Payload` into the service's payload type; **no business
  re-validation** (that ran at submit), but map guard failures (`pgx.ErrNoRows`,
  unique violation) to clear errors → 409.
- Return `(entityID, after, err)`; disable/enable return `map[string]bool{"is_active": …}`.

### 5. Wire up — `backend/cmd/api/main.go`
- Add `"<entity_type>": service.<Entity>Applier{}` to `masterDataApplierRegistry`
  (missing entry = "no applier registered" at approve time).
- Mount the handler on `masterDataAdmin` (RequireAuth + ADMIN/ADMIN_PARAM) —
  RBAC at route **and** in `Submit`.

### 6. Handler — flat JSON, 202
- Small servicer interface; routes `GET /`, `POST /`, `PUT /{id}`,
  `POST /{id}/disable`, `POST /{id}/enable`.
- Success → `writeJSON(w, http.StatusAccepted, changeRequestAcceptedResponse(change))`.
- Error map: `ValidationError`→422 (`writeValidationError`), NotFound→404,
  Conflict / `ErrMasterDataChangePending`→409, `ErrMasterDataForbidden`→403,
  else `writeUnexpectedError`. Client IP only via `extractClientIP(r)`.
- **No `pkg/response` envelope** in ATM `backend/` (CLAUDE.md §3).

### 7. Tests (CLAUDE.md §8)
- Service unit (mock repo + submitter): validation, not-found across parents,
  conflict, `Before` passed on update/disable/enable, nil on create.
- Handler: 202 body, each error → status, missing auth → 401.
- Integration (`//go:build integration`, skips without `DATABASE_URL`): full
  lifecycle through `approveChange` (create/update/disable/enable), stale `Before`
  → not applied, apply-time guard → error. Use the `harness`/`testHelpers` seeds.
- `no_hard_delete_test.go` lists updated.
- Run: `cd backend && go test ./...` and `golangci-lint run --build-tags integration ./...`.

### 8. Frontend — `frontend/CompanyPortal-Vite`
- Add the entity_type to `PendingEntityType` in `src/features/master-data/pending.ts`
  and a label in `ENTITY_LABELS` in `src/features/approvals/lib/changeDiff.ts`
  (checker screen).
- Panel uses `usePendingEntityIds` / `usePendingCreates` to badge rows; show
  `pendingApprovalMessage()` (`src/features/master-data/changeRequest.ts`) after a 202.
- Browser check is the user's job (Golden Rule #10) — mark it outstanding.

### 9. Optional, ask first
- CSV import/export (`masterdata_import*.go`, `masterdata_export.go`): not every
  entity has one; the FSD replace-all flow is pending (CLAUDE.md §12).
- Update `docs/data-map.md`, CLAUDE.md §3 table list, `development-progress.md`.
