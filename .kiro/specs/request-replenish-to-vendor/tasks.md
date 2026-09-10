# Implementation Plan: Request Replenish to Vendor

## Overview

This feature turns DMAA ATM forecast data into a Vendor Request that flows through a self-contained state machine (draft → pending_approval → approved/rejected → processing → completed/failed, plus cancel/revise) with maker-checker approval and an append-only audit trail. The backend follows the existing flat layout (`internal/handler`, `internal/service`, `queries/` → sqlc `internal/db`) and the ATM backend's flat JSON response shape. The frontend adds a Forecast Browser, Create, List, and Detail page in `CompanyPortal-Vite`. See `design.md` for the state machine, schema, API contract, and correctness properties.

## Tasks

- [ ] 1. Backend: database migration
  - [ ] 1.1 Create `backend/migrations/028_vendor_requests.sql`
    - Wrap in BEGIN/COMMIT; create `vendor_requests` (id identity, request_number unique, forecast_date, status text+CHECK, notes, created_by/approved_by/rejected_by FK users(id) ON DELETE RESTRICT, rejection_reason, timestamps)
    - Create `vendor_request_items` (id identity, vendor_request_id FK ON DELETE CASCADE, terminal_id, periode_pred, denom int CHECK > 0, amount_replenish/amount_refund bigint CHECK >= 0, created_at) with UNIQUE (vendor_request_id, terminal_id, periode_pred, denom)
    - Add indexes: vendor_requests(status), (forecast_date), (created_by); vendor_request_items(vendor_request_id), (terminal_id, periode_pred)
    - Attach `trg_vendor_requests_set_updated_at` reusing the existing `set_updated_at()` function (migration 014)
    - Add COMMENT ON table/status; include commented reversible DOWN block (trigger → child table → parent table)
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7_

- [ ] 2. Backend: sqlc queries and repository generation
  - [ ] 2.1 Create `backend/queries/vendor_request.sql`
    - CreateVendorRequest (:one), InsertVendorRequestItem (:one), DeleteVendorRequestItems (:exec)
    - GetVendorRequest (:one), GetVendorRequestForUpdate (:one, FOR UPDATE), ListVendorRequestItems (:many)
    - UpdateVendorRequestStatus (:one) setting status + relevant timestamp/actor columns
    - ListVendorRequests (:many) + CountVendorRequests (:one): filter status (any-of), forecast_date, created_by; sort created_at DESC; include item_count + total_amount aggregate; join users for full_name
    - MaxRequestNumberSeqForDate (:one) for number generation
    - ListForecastForDate (:many) + CountForecastForDate (:one): dmaa_atm_forecast WHERE periode_pred = @forecast_date, optional terminal_id ILIKE, paginated
    - ForecastRowsExist (:one) to validate items against dmaa_atm_forecast
    - _Requirements: 3.2, 3.4, 4.4, 8.2, 8.6, 9.1, 9.4, 15.2, 16.4_
  - [ ] 2.2 Run `sqlc generate` from `backend/` and verify generated code compiles
    - Confirm generated funcs exist in `backend/internal/db`
    - _Requirements: 1.1, 1.2_

- [ ] 3. Backend: service layer — state machine, validation, number generation, audit
  - [ ] 3.1 Create `backend/internal/service/vendor_request.go` core scaffolding
    - Define VendorRequestServicer interface, Actor struct, input/result/detail types (CreateVendorRequestInput, ItemInput, ListVendorRequestParams, VendorRequestDetail, UserRef)
    - Define VendorRequestRepository interface (satisfied by *db.Queries + pool for tx) and sentinel errors (ErrNotFound, ErrInvalidTransition, ErrNotCreator, ErrNotChecker, ErrSelfApproval, ErrEmptyItems, ErrRejectReasonEmpty, ErrInvalidItems, ErrDuplicateItems, ErrNumberExhausted, ErrNumberGeneration)
    - Implement NewVendorRequestService(pool, auditWriter)
    - _Requirements: 2.2, 2.6, 2.8_
  - [ ] 3.2 Implement the transition table and guards
    - transitions map + next(cur, action) helper; state guard → ErrInvalidTransition
    - Actor-role guards: maker (ATM-USER/BRANCH-ATM-USER/ADMIN) for submit/cancel-draft/revise; checker (ATM-SPV/BRANCH-ATM-SPV/ADMIN) for approve/reject/cancel-pending
    - Four-eyes guard: actor.UserID != created_by on approve/reject
    - _Requirements: 2.1, 2.2, 2.4, 2.5, 2.7, 2.8, 2.12_
  - [ ] 3.3 Implement BrowseForecast
    - Validate forecast_date (YYYY-MM-DD required), page/page_size (default 1/20, max 100), optional terminal_id
    - Call ListForecastForDate + CountForecastForDate; assemble pagination metadata
    - _Requirements: 3.1, 3.2, 3.3, 3.5, 3.6, 3.7, 3.8_
  - [ ] 3.4 Implement Create (draft) with request-number generation
    - Validate >=1 item, notes <= 500, no duplicate (terminal_id, periode_pred, denom) in payload, all items exist in dmaa_atm_forecast
    - Generate VR-YYYYMMDD-NNNN (per-date sequence, max 9999) inside a tx with up-to-3 retry on unique violation
    - Insert header + items; honor amount_replenish override, record discrepancy for audit
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 15.1, 15.2, 15.3, 15.4, 15.5, 15.6_
  - [ ] 3.5 Implement UpdateItems (draft-only, full replacement)
    - Guard draft state + creator; validate items (same rules as create); replace all items in a single tx
    - _Requirements: 8.2, 8.4, 8.5, 8.6_
  - [ ] 3.6 Implement Submit, Approve, Reject, Revise, Cancel
    - Each: tx → GetVendorRequestForUpdate → state guard → actor guard → four-eyes (approve/reject) → UpdateVendorRequestStatus → stamp timestamps/actors → audit write
    - Submit rejects zero-item requests (ErrEmptyItems → 409); Reject requires non-empty trimmed reason (<=500)
    - _Requirements: 2.3, 2.9, 2.10, 2.11, 5.2, 5.3, 5.4, 6.3, 6.4, 7.2, 7.3, 10.2, 10.3_
  - [ ] 3.7 Implement List, Get, AuditLog
    - List: validate filters/pagination, return summaries with item_count + total_amount + user full_name
    - Get: return detail with items; ErrNotFound when absent
    - AuditLog: read audit_logs WHERE entity_type='vendor_request' AND entity_id=$id ORDER BY created_at ASC; project before.state/after.state → previous_state/new_state
    - _Requirements: 9.1, 9.2, 9.4, 9.5, 9.6, 16.4, 16.6_
  - [ ] 3.8 Wire audit writes for every transition
    - EntityType="vendor_request"; Action ∈ {create, submit, approve, reject, revise, cancel, update_items}; Before={"state":prev}; After={"state":new, ...metadata}
    - metadata: rejection_reason (reject), items_added/removed/modified (update_items), amount_overrides (create)
    - _Requirements: 2.3, 16.1, 16.2, 16.3, 16.6_
  - [ ]* 3.9 Unit tests for the state machine and guards
    - Table-driven: every valid transition succeeds, every invalid one returns ErrInvalidTransition without mutation
    - Four-eyes: self-approve/self-reject rejected; role gating; empty-items submit; missing rejection reason
    - Number generation format + per-date sequencing + retry-on-conflict
    - _Requirements: 2.1, 2.2, 2.4, 2.5, 2.7, 2.8, 5.4, 6.4, 15.1, 15.3_

- [ ] 4. Backend: handler and route registration
  - [ ] 4.1 Create `backend/internal/handler/vendor_request_handler.go`
    - NewVendorRequestHandler(svc); Routes() mounting GET /forecast, POST /, PUT /{id}/items, POST /{id}/{submit,approve,reject,revise,cancel}, GET /, GET /{id}, GET /{id}/audit-log
    - Resolve Actor from middleware.GetAuthContext + extractClientIP; decode/validate bodies; parse query params (parseIntParam, queryOrDefault)
    - Map service errors to HTTP per design table; responses use flat JSON ({data, pagination} for lists)
    - _Requirements: 3.1, 3.7, 3.8, 3.9, 4.1, 4.8, 5.1, 5.5, 5.6, 5.7, 5.8, 6.1, 6.2, 6.5, 6.6, 6.7, 6.8, 6.9, 6.10, 6.11, 6.12, 7.1, 7.5, 7.6, 7.7, 8.1, 8.3, 8.7, 8.8, 9.6, 9.7, 9.8, 10.1, 10.5, 10.6, 10.7, 10.8, 16.4, 16.5, 16.7_
  - [ ] 4.2 Register routes in `backend/cmd/api/main.go`
    - Instantiate service with dbPool + audit.NewWriter(dbPool) and handler; mount at /api/v1/vendor-requests behind custommw.RequireAuth (role subset enforced in service/handler)
    - _Requirements: 3.9, 4.8, 5.8, 6.9, 7.7, 8.8, 9.8, 10.8, 16.5_
  - [ ]* 4.3 Handler tests for param parsing and error mapping
    - Valid/missing params, non-numeric pagination → 400; state conflict → 409; wrong role/self-approval → 403; missing reason → 422; unknown id → 404
    - _Requirements: 3.8, 5.3, 6.6, 6.12, 9.7_

- [ ] 5. Checkpoint — Backend verification
  - Ensure the module builds and `go test ./...` passes from `backend/`. The state-machine-vs-orchestrator decision is resolved (dedicated state machine, confirmed). Ask the user only if the audit_logs mapping (see design.md) needs revisiting.

- [ ] 6. Frontend: types, API client, and query hooks
  - [ ] 6.1 Create `src/features/vendor-request/types.ts` and `api.ts`
    - Types: VendorRequestStatus, ForecastRow, VendorRequestItem, VendorRequestSummary, VendorRequestDetail, pagination
    - api.ts: apiClient calls for all endpoints (browse, create, updateItems, submit, approve, reject, revise, cancel, list, get, auditLog), mapping camelCase ↔ snake_case
    - _Requirements: 3.4, 9.4, 9.6_
  - [ ] 6.2 Create `src/features/vendor-request/hooks.ts`
    - useForecastBrowse, useVendorRequests (list), useVendorRequest (detail) with useQuery + keepPreviousData
    - Mutation hooks for create/submit/approve/reject/revise/cancel/updateItems with query invalidation
    - _Requirements: 11.11, 13.9, 14.10_

- [ ] 7. Frontend: Forecast Browser page
  - [ ] 7.1 Create `ForecastBrowser.tsx` + route `src/routes/replenishment/forecast-browser.tsx`
    - Date picker defaulting to next business day; TanStack Table with row selection + Select All (post-filter)
    - Debounced (300ms) terminal filter preserving selection; summary bar (selected count, total IDR tabular-nums, primary --red-500 "Create Vendor Request" enabled only when >=1 selected)
    - Navigate to create page passing selected items + forecast_date via route state; clear selection on date change
    - Loading skeleton, empty state, error + retry
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8, 11.9, 11.11, 11.12_

- [ ] 8. Frontend: Vendor Request Create page
  - [ ] 8.1 Create `VendorRequestCreate.tsx` + route `src/routes/replenishment/vendor-requests/new.tsx`
    - Read-only forecast date; notes textarea (<=500); editable amount_replenish per row (right-aligned tabular-nums); remove-row; live total (IDR)
    - RHF + Zod: >=1 item, amounts positive int 1..999,999,999; buttons disabled while invalid; inline errors
    - "Save as Draft" → POST create → detail; "Submit for Approval" → create then submit → detail; on submit-after-create failure navigate to draft detail + error toast; preserve input on error
    - Redirect non-maker roles (not ADMIN/ATM-USER/BRANCH-ATM-USER) to dashboard
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7, 12.8, 12.9, 12.10_

- [ ] 9. Frontend: Vendor Request List page
  - [ ] 9.1 Create `StatusBadge.tsx` and `VendorRequestList.tsx` + route `src/routes/replenishment/vendor-requests/index.tsx`
    - Status badges per design system (draft=neutral, pending_approval=warning, approved=success, rejected=danger, processing=info, completed=success, cancelled=neutral strikethrough); never brand red
    - TanStack Table columns: Request Number, Forecast Date, Status, Items Count, Total Amount (tabular-nums IDR), Created By, Created At, View Detail
    - Filters: status multi-select, forecast date, request-number search (debounced 300ms, max 50); server-side pagination (default 10, sizes 10/20/50), sort created_at DESC; row-click to detail
    - "New Vendor Request" primary button for maker roles; empty state; error + retry; URL state via validateSearch (Zod)
    - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6, 13.7, 13.8, 13.9_

- [ ] 10. Frontend: Vendor Request Detail page
  - [ ] 10.1 Create `VendorRequestDetail.tsx` + route `src/routes/replenishment/vendor-requests/$id.tsx`
    - Header (omit null fields), items table + summary (total tabular-nums IDR), status badge
    - Conditional actions by state × role × creator: draft-creator (Edit Items, Submit, Cancel); pending-checker-non-creator (Approve, Reject); rejected-creator (Revise); pending-creator (Cancel)
    - Reject modal: reason textarea (1..500), Confirm disabled until valid, dismiss without action
    - Disable actions during in-flight call with per-button loading; refetch (query invalidation) on success; error toast (5s) on failure; 404 not-found state; loading skeleton
    - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5, 14.6, 14.7, 14.8, 14.9, 14.10, 14.11, 14.12, 14.13, 14.14_

- [ ] 11. Frontend: navigation registration
  - [ ] 11.1 Add nav entries in `src/lib/config/navigation.ts`
    - Replenishment group items for Forecast Browser and Vendor Requests, roles ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV
    - _Requirements: 11.10, 13.7, 14.14_

- [ ] 12. Final checkpoint — Full verification
  - Ensure backend `go test ./...` and frontend `pnpm test --run` pass; run `sqlc generate` and confirm the migration applies cleanly against a real Postgres. Ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional test tasks and can be deferred for a faster MVP, but backend money/state-machine tests are strongly recommended before wiring the frontend.
- **Resolved:** this feature uses a **dedicated state machine** (confirmed by the team), not the generic `internal/approval` orchestrator. Requirements 2/5/6/15 stand as written; Task 3 implements the local state machine.
- **Still open (minor):** Req 16's `previous_state`/`new_state`/`metadata` are mapped onto the existing `audit_logs.before`/`after` jsonb columns rather than adding new columns. If the team later prefers a dedicated `vendor_request_audit` table, revisit Task 3.8 and the migration. This does not block implementation.
- Follow existing patterns: `dmaa_forecast_handler.go` / `atm_portal_handler.go` (parseIntParam, queryOrDefault, writeJSON/writeError), `approval_handler.go` (Actor from middleware.GetAuthContext, extractClientIP), and `audit.Writer` for audit writes.
- `terminal_id` in the schema/API maps to the requirements' `atm_id`; the frontend labels it "ATM ID".
- Writes use the primary pool; read-after-write (detail/list refetch) uses the primary pool. Forecast browse may use the read replica once the `dbRead` pool is wired.
- Money is integer IDR (bigint) end to end; format with the existing IDR utility on the frontend.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "6.1"] },
    { "id": 1, "tasks": ["2.1", "6.2"] },
    { "id": 2, "tasks": ["2.2"] },
    { "id": 3, "tasks": ["3.1"] },
    { "id": 4, "tasks": ["3.2", "3.3"] },
    { "id": 5, "tasks": ["3.4", "3.5"] },
    { "id": 6, "tasks": ["3.6", "3.7", "3.8"] },
    { "id": 7, "tasks": ["3.9", "4.1"] },
    { "id": 8, "tasks": ["4.2", "4.3"] },
    { "id": 9, "tasks": ["5"] },
    { "id": 10, "tasks": ["7.1", "8.1"] },
    { "id": 11, "tasks": ["9.1", "10.1"] },
    { "id": 12, "tasks": ["11.1"] },
    { "id": 13, "tasks": ["12"] }
  ]
}
```
