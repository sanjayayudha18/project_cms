# Design Document: Request Replenish to Vendor

## Overview

This feature lets internal Cash Management staff turn DMAA ATM forecast data into a cash replenishment request (a "Vendor Request") that flows through a controlled state machine with maker-checker approval before it becomes actionable for a CIT vendor.

The DMAA forecast data already exists in PostgreSQL (`dmaa_atm_forecast` + `dmaa_files`, populated by an existing Python ETL). This feature adds:

- Two new tables (`vendor_requests`, `vendor_request_items`) owned by a new migration in `backend/migrations/`.
- A backend feature slice in the existing **flat** layout (`internal/handler`, `internal/service`, `queries/`, sqlc-generated `internal/db`) — not a new `internal/vendor_request/` package, matching how `dmaa_forecast` and `atm_portal` are already organized.
- A self-contained state machine in the service layer (draft → pending_approval → approved/rejected → …), with the four-eyes rule enforced in the service.
- Audit entries via the existing append-only `audit.Writer` / `audit_logs` table.
- Frontend pages in `CompanyPortal-Vite`: a Forecast Browser, a Create page, a List page, and a Detail page.

### Design decision: dedicated state machine, not the generic `approval.Orchestrator`

The requirements (Req 2, 5–10, 15–16) describe a self-contained lifecycle with request-owned columns (`submitted_at`, `approved_by`, `rejection_reason`, `request_number`) and a specific role gate (Maker = ATM-USER/BRANCH-ATM-USER, Checker = ATM-SPV/BRANCH-ATM-SPV). This is intentionally implemented as a **dedicated `VendorRequestService` state machine**, not by delegating to `internal/approval`.

Rationale:
- The generic `approval.Orchestrator` keys approval state on a separate `approval_requests` row per `(document_type, document_id)` and drives a level-based hierarchy (`approval_policies`, `approval_steps`, delegations). The requirements here specify a single-hop maker→checker gate driven by role, with state stored **on** the vendor request itself. Forcing that through the generic orchestrator would split the request's truth across two tables and lose the request-number/timestamp semantics the requirements demand.
- Per project-context "Approval integration pattern", modules *may* use the orchestrator, but this feature's requirements are explicit and simpler (one approval level, role-based). Keeping the state machine local keeps the DMAA→vendor flow auditable and self-contained.

> **Decision (confirmed):** The team has chosen the dedicated state machine approach. This feature does **not** use the generic `internal/approval` orchestrator. Requirements 2/5/6/15 stand as written. This is the locked design for the DMAA→vendor replenishment flow.

### Terminology note: `terminal_id` vs `atm_id`

The requirements use `atm_id` for the forecast identifier. The actual `dmaa_atm_forecast` column is **`terminal_id`** (composite PK `dmaa_file_id, terminal_id, periode_pred, denom`). The design stores the identifier as `terminal_id` in the DB and API for consistency with the existing DMAA viewer, and the frontend labels it "ATM ID" for the user. Where this document says `terminal_id`, it maps to the requirements' `atm_id`.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Frontend (CompanyPortal-Vite)                                             │
│                                                                            │
│  /replenishment/forecast-browser   → pick date, select forecast rows      │
│  /replenishment/vendor-requests/new → review items, save/submit           │
│  /replenishment/vendor-requests      → list + filters + status badges     │
│  /replenishment/vendor-requests/$id  → detail + actions (submit/approve…)  │
│        │  TanStack Router (URL state) · TanStack Query (cache)             │
│        │  TanStack Table · React Hook Form + Zod                          │
└────────┼───────────────────────────────────────────────────────────────────┘
         │  /api/v1/vendor-requests/*
         ▼
┌──────────────────────────────────────────────────────────────────────────┐
│  Backend (Go + Chi, flat layout)                                           │
│                                                                            │
│  handler/vendor_request_handler.go                                         │
│        │  parse + auth (middleware.GetAuthContext) + JSON envelope         │
│        ▼                                                                    │
│  service/vendor_request.go   ← STATE MACHINE + four-eyes + number gen      │
│        │            │                                                       │
│        │            └────────▶ audit.Writer  → audit_logs (append-only)    │
│        ▼                                                                    │
│  db/ (sqlc)  ← queries/vendor_request.sql                                  │
│        │  reads: dmaa_atm_forecast (validate items)                        │
│        │  writes: vendor_requests, vendor_request_items (primary pool)     │
└────────┼───────────────────────────────────────────────────────────────────┘
         ▼
┌──────────────────────────────────────────────────────────────────────┐
│  PostgreSQL                                                            │
│  vendor_requests · vendor_request_items · dmaa_atm_forecast · users    │
│  Writes → primary pool.  Forecast browse (read) → replica when wired.  │
└──────────────────────────────────────────────────────────────────────┘
```

State machine (Vendor Request lifecycle):

```
                submit                 approve
   draft ───────────────▶ pending_approval ───────────▶ approved ──process──▶ processing
     │  ▲                    │      │                                            │   │
cancel│  │revise      reject │      │cancel                              complete│   │fail
     ▼  │                    ▼      ▼                                            ▼   ▼
  cancelled              rejected  cancelled                              completed failed
              (rejected ──revise──▶ draft)
```

Valid transitions (Req 2.1): `draft→pending_approval`, `draft→cancelled`, `pending_approval→approved`, `pending_approval→rejected`, `pending_approval→cancelled`, `rejected→draft`, `approved→processing`, `processing→completed`, `processing→failed`. Any other transition is rejected without mutating state (Req 2.2).

Actor rules:
- Maker (creator only): submit, cancel-from-draft, revise (Req 2.4).
- Checker (ATM-SPV / BRANCH-ATM-SPV, ≠ creator): approve, reject, cancel-from-pending (Req 2.5, 2.7, 2.12).
- Four-eyes: checker ≠ maker on approve/reject (Req 2.7, 2.8).

---

## Components

### Backend

#### 1. Migration — `backend/migrations/028_vendor_requests.sql`

New migration (next free number is 028). Wrapped in `BEGIN/COMMIT`, text+CHECK for status (no ENUM), FKs to `users(id)`, reuses the existing `set_updated_at()` trigger function (migration 014). Includes a reversible down migration as a commented block, consistent with the repo's convention.

```sql
BEGIN;

CREATE TABLE IF NOT EXISTS public.vendor_requests
(
    id                bigint GENERATED BY DEFAULT AS IDENTITY,
    request_number    text        NOT NULL,
    forecast_date     date        NOT NULL,
    status            text        NOT NULL DEFAULT 'draft',
    notes             text,
    created_by        bigint      NOT NULL,
    approved_by       bigint,
    rejected_by       bigint,
    rejection_reason  text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    submitted_at      timestamptz,
    approved_at       timestamptz,
    rejected_at       timestamptz,
    CONSTRAINT vendor_requests_pkey PRIMARY KEY (id),
    CONSTRAINT vendor_requests_number_uq UNIQUE (request_number),
    CONSTRAINT vendor_requests_status_chk CHECK (status IN
        ('draft','pending_approval','approved','rejected','processing','completed','failed','cancelled')),
    CONSTRAINT vendor_requests_created_by_fk FOREIGN KEY (created_by)
        REFERENCES public.users (id) ON UPDATE NO ACTION ON DELETE RESTRICT,
    CONSTRAINT vendor_requests_approved_by_fk FOREIGN KEY (approved_by)
        REFERENCES public.users (id) ON UPDATE NO ACTION ON DELETE RESTRICT,
    CONSTRAINT vendor_requests_rejected_by_fk FOREIGN KEY (rejected_by)
        REFERENCES public.users (id) ON UPDATE NO ACTION ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS vendor_requests_status_idx        ON public.vendor_requests(status);
CREATE INDEX IF NOT EXISTS vendor_requests_forecast_date_idx ON public.vendor_requests(forecast_date);
CREATE INDEX IF NOT EXISTS vendor_requests_created_by_idx    ON public.vendor_requests(created_by);

CREATE TABLE IF NOT EXISTS public.vendor_request_items
(
    id                bigint GENERATED BY DEFAULT AS IDENTITY,
    vendor_request_id bigint      NOT NULL,
    terminal_id       text        NOT NULL,   -- maps to requirements' atm_id
    periode_pred      date        NOT NULL,
    denom             int         NOT NULL,
    amount_replenish  bigint      NOT NULL DEFAULT 0,
    amount_refund     bigint      NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT vendor_request_items_pkey PRIMARY KEY (id),
    CONSTRAINT vendor_request_items_denom_chk CHECK (denom > 0),
    CONSTRAINT vendor_request_items_replenish_chk CHECK (amount_replenish >= 0),
    CONSTRAINT vendor_request_items_refund_chk CHECK (amount_refund >= 0),
    CONSTRAINT vendor_request_items_request_fk FOREIGN KEY (vendor_request_id)
        REFERENCES public.vendor_requests (id) ON UPDATE NO ACTION ON DELETE CASCADE,
    CONSTRAINT vendor_request_items_uq UNIQUE (vendor_request_id, terminal_id, periode_pred, denom)
);

CREATE INDEX IF NOT EXISTS vendor_request_items_request_idx  ON public.vendor_request_items(vendor_request_id);
CREATE INDEX IF NOT EXISTS vendor_request_items_terminal_idx ON public.vendor_request_items(terminal_id, periode_pred);

DROP TRIGGER IF EXISTS trg_vendor_requests_set_updated_at ON public.vendor_requests;
CREATE TRIGGER trg_vendor_requests_set_updated_at
    BEFORE UPDATE ON public.vendor_requests
    FOR EACH ROW
    WHEN (OLD.* IS DISTINCT FROM NEW.*)
    EXECUTE FUNCTION public.set_updated_at();

COMMENT ON TABLE public.vendor_requests IS 'Cash replenishment request to a CIT vendor, composed from DMAA forecast rows; state machine + maker-checker.';
COMMENT ON COLUMN public.vendor_requests.status IS 'draft | pending_approval | approved | rejected | processing | completed | failed | cancelled';

COMMIT;

-- DOWN (reverse dependency order):
--   DROP TRIGGER IF EXISTS trg_vendor_requests_set_updated_at ON public.vendor_requests;
--   DROP TABLE IF EXISTS public.vendor_request_items;
--   DROP TABLE IF EXISTS public.vendor_requests;
```

Note: the status CHECK includes `'failed'` because the state machine defines `processing→failed` (Req 2.1), even though requirement 1.3 lists a shorter set. The superset is safe and keeps the CHECK consistent with the transitions.

#### 2. sqlc queries — `backend/queries/vendor_request.sql`

SQL-first (sqlc, pgx/v5). Key queries:

- `CreateVendorRequest` (:one) — insert header (draft), returns the row.
- `InsertVendorRequestItem` (:one) / batch insert loop within a tx.
- `DeleteVendorRequestItems` (:exec) — used by the full-replacement update (Req 8.2).
- `GetVendorRequest` (:one) / `GetVendorRequestForUpdate` (:one, `FOR UPDATE`) — status transitions read-then-write under a tx.
- `ListVendorRequestItems` (:many).
- `UpdateVendorRequestStatus` (:one) — sets status + the relevant timestamp/actor columns.
- `ListVendorRequests` (:many) + `CountVendorRequests` (:one) — filters: status (any-of), forecast_date, created_by; sorted `created_at DESC`; with `item_count` and `total_amount` via a lateral/subquery aggregate.
- `MaxRequestNumberSeqForDate` (:one) — `SELECT max(seq)` derived from `request_number` for a `forecast_date`, for number generation (Req 15.2).
- `ListForecastForDate` (:many) + `CountForecastForDate` (:one) — reads `dmaa_atm_forecast WHERE periode_pred = @forecast_date` with optional `terminal_id` ILIKE filter, paginated (Req 3).
- `ForecastRowsExist` (:one) — validates that submitted items reference real `(terminal_id, periode_pred, denom)` forecast rows (Req 4.4, 8.6).

Run `sqlc generate` from `backend/`; generated code lands in `backend/internal/db`.

#### 3. Service — `backend/internal/service/vendor_request.go`

Owns validation, the state machine, four-eyes enforcement, request-number generation, and audit writes. Defines its own repository interface (Go convention). Writes use the primary pool.

```go
package service

// VendorRequestServicer is the interface the handler depends on.
type VendorRequestServicer interface {
    BrowseForecast(ctx context.Context, p BrowseForecastParams) (*BrowseForecastResult, error)
    Create(ctx context.Context, actor Actor, in CreateVendorRequestInput) (*VendorRequestDetail, error)
    UpdateItems(ctx context.Context, actor Actor, id int64, items []ItemInput) (*VendorRequestDetail, error)
    Submit(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error)
    Approve(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error)
    Reject(ctx context.Context, actor Actor, id int64, reason string) (*VendorRequestDetail, error)
    Revise(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error)
    Cancel(ctx context.Context, actor Actor, id int64) (*VendorRequestDetail, error)
    List(ctx context.Context, p ListVendorRequestParams) (*ListVendorRequestResult, error)
    Get(ctx context.Context, id int64) (*VendorRequestDetail, error)
    AuditLog(ctx context.Context, id int64) ([]AuditEntry, error)
}

// Actor carries who is acting, resolved from the JWT by the handler.
type Actor struct {
    UserID int64
    Roles  []string
    IP     string
}
```

**State machine** — a transition table plus a guard:

```go
type action string // submit, cancel, approve, reject, revise, process, complete, fail

var transitions = map[string]map[action]string{
    "draft":            {"submit": "pending_approval", "cancel": "cancelled"},
    "pending_approval": {"approve": "approved", "reject": "rejected", "cancel": "cancelled"},
    "rejected":         {"revise": "draft"},
    "approved":         {"process": "processing"},
    "processing":       {"complete": "completed", "fail": "failed"},
}

// next returns (newState, ok). ok=false → ErrInvalidTransition (Req 2.2).
func next(cur string, a action) (string, bool) {
    m, ok := transitions[cur]
    if !ok { return "", false }
    to, ok := m[a]
    return to, ok
}
```

Each mutating method runs inside a single DB transaction: `GetVendorRequestForUpdate` → guard state (`ErrInvalidTransition` → 409) → guard actor (`ErrNotCreator` / `ErrNotChecker` → 403) → guard four-eyes (`ErrSelfApproval` → 403) → `UpdateVendorRequestStatus` → `audit.Writer.Write`. Sentinel errors:

```go
var (
    ErrNotFound           = errors.New("vendor request not found")
    ErrInvalidTransition  = errors.New("invalid state transition")
    ErrNotCreator         = errors.New("only the creator can perform this action")
    ErrNotChecker         = errors.New("only a checker can perform this action")
    ErrSelfApproval       = errors.New("four-eyes: checker must differ from maker")
    ErrEmptyItems         = errors.New("request must contain at least one item")
    ErrRejectReasonEmpty  = errors.New("rejection reason is required")
    ErrInvalidItems       = errors.New("one or more items are invalid")
    ErrDuplicateItems     = errors.New("duplicate item in payload")
    ErrNumberExhausted    = errors.New("request number sequence exhausted for date")
    ErrNumberGeneration   = errors.New("could not generate request number")
)
```

**Request number generation** (Req 15): pattern `VR-YYYYMMDD-NNNN` where `YYYYMMDD` = `forecast_date`, `NNNN` = zero-padded per-date sequence from 0001, max 9999. Within the create transaction: read `MaxRequestNumberSeqForDate`, increment, format, insert; on a `vendor_requests_number_uq` violation retry up to 3 times (`ErrNumberGeneration` after exhaustion); if next > 9999 → `ErrNumberExhausted`.

**Item validation** (Req 4, 8): reject duplicate `(terminal_id, periode_pred, denom)` within the payload (`ErrDuplicateItems`); every item must match a real `dmaa_atm_forecast` row via `ForecastRowsExist` (`ErrInvalidItems`); a manual `amount_replenish` override differing from the forecast is allowed but recorded in the audit metadata (Req 4.5).

**Roles** (constants): Maker = {ATM-USER, BRANCH-ATM-USER, ADMIN}; Checker = {ATM-SPV, BRANCH-ATM-SPV, ADMIN}. ADMIN is treated as maker-capable for create/submit/revise/cancel and checker-capable for approve/reject, but the four-eyes rule still blocks self-approval.

#### 4. Audit — reuse `internal/audit`

Every transition writes an `audit_logs` row via `audit.Writer.Write(ctx, audit.Entry{...})`. The requirements (Req 16.1) ask for `previous_state`, `new_state`, and `metadata`; the existing `audit_logs` table has `before jsonb` / `after jsonb` (no dedicated state/metadata columns). Mapping:

- `EntityType = "vendor_request"`, `EntityID = <vendor_request_id>`, `ActorID = actor.UserID`, `IP = actor.IP`.
- `Action` ∈ {create, submit, approve, reject, revise, cancel, update_items}.
- `Before = {"state": <previous_state>}` (nil for create).
- `After  = {"state": <new_state>, ... metadata}` where metadata carries `rejection_reason` (reject), `items_added/items_removed/items_modified` (update_items), or `amount_overrides` (create with override).

The `GET /{id}/audit-log` endpoint reads `audit_logs WHERE entity_type='vendor_request' AND entity_id=$id ORDER BY created_at ASC`, projecting `before.state`/`after.state` back out as `previous_state`/`new_state` in the response so the API contract in Req 16.4 holds without a schema change.

> This reuses the pre-approved `audit_logs` table rather than adding a `vendor_request_audit` table. No new audit table is proposed. Flagged as a deliberate mapping.

#### 5. Handler — `backend/internal/handler/vendor_request_handler.go`

HTTP only: decode/validate input, resolve `Actor` from `middleware.GetAuthContext` + `extractClientIP`, call the service, map errors, write JSON with the existing flat helpers (`writeJSON`, `writeError`, `writeValidationError`, `writeForbidden`, `writeUnauthorized`). Responses use the ATM backend's flat JSON shape (`{data, pagination}` for lists), consistent with the DMAA viewer.

Error mapping:

| Service error | HTTP |
|---|---|
| `ErrNotFound` | 404 `not_found` |
| `ErrInvalidTransition` | 409 `conflict` |
| `ErrEmptyItems` | 409 `conflict` |
| `ErrNotCreator` / `ErrNotChecker` / `ErrSelfApproval` | 403 `forbidden` |
| `ErrDuplicateItems` / `ErrInvalidItems` | 400 `bad_request` |
| `ErrRejectReasonEmpty` | 422 `validation_error` |
| `ErrNumberExhausted` / `ErrNumberGeneration` | 409 `conflict` / 500 `internal_error` |
| other | 500 `internal_error` |

#### 6. Route registration — `backend/cmd/api/main.go`

```go
vendorRequestService := service.NewVendorRequestService(dbPool, audit.NewWriter(dbPool))
vendorRequestHandler := handler.NewVendorRequestHandler(vendorRequestService)
r.With(custommw.RequireAuth(tokenService)).
    Mount("/api/v1/vendor-requests", vendorRequestHandler.Routes())
```

Mounted behind `RequireAuth` only; fine-grained role checks (maker vs checker) happen in the service where the four-eyes and role logic already live, matching how `/api/v1/approvals` is mounted. Read-only browse/list/detail endpoints additionally accept the viewer role set; the service/handler enforces the write-role subset per endpoint.

---

### Frontend (`CompanyPortal-Vite`)

Feature slice `src/features/vendor-request/` and routes under `src/routes/replenishment/`.

```
src/features/vendor-request/
├── index.ts
├── types.ts                       # VendorRequest, VendorRequestItem, ForecastRow, enums
├── api.ts                         # apiClient calls (@/lib/api/client)
├── hooks.ts                       # useForecastBrowse, useVendorRequests, useVendorRequest, mutations
├── ForecastBrowser.tsx            # Req 11
├── VendorRequestCreate.tsx        # Req 12 (RHF + Zod)
├── VendorRequestList.tsx          # Req 13
├── VendorRequestDetail.tsx        # Req 14
├── StatusBadge.tsx                # design-system badge mapping
└── __tests__/
```

Routes (TanStack Router, `validateSearch` with Zod for URL state):

| Route | Component | Req |
|---|---|---|
| `/replenishment/forecast-browser` | ForecastBrowser | 11 |
| `/replenishment/vendor-requests` | VendorRequestList | 13 |
| `/replenishment/vendor-requests/new` | VendorRequestCreate | 12 |
| `/replenishment/vendor-requests/$id` | VendorRequestDetail | 14 |

- **ForecastBrowser**: date picker defaulting to next business day; TanStack Table with row selection + Select All; debounced (300ms) terminal filter preserving selection; summary bar with selected count + total (IDR, `tabular-nums`) + primary `--red-500` "Create Vendor Request" button that passes selection via route state. Loading skeleton, empty state, error+retry.
- **VendorRequestCreate**: read-only forecast date, notes textarea (≤500), editable `amount_replenish` per row (RHF+Zod, 1–999,999,999 positive int), remove-row, live total. "Save as Draft" (secondary) → `POST /vendor-requests`; "Submit for Approval" (primary `--red-500`) → create then `POST /{id}/submit`; if submit fails after create, navigate to detail (draft) + error toast. Redirect non-maker roles to dashboard.
- **VendorRequestList**: TanStack Table, status badges per design system, filters (status multi-select, forecast date, request-number search debounced 300ms), server-side pagination (default 10; sizes 10/20/50), row-click to detail, "New Vendor Request" button for maker roles. Empty + error states.
- **VendorRequestDetail**: full header (null fields omitted), items table + summary, action buttons conditioned on state × role × creator (Edit Items / Submit / Cancel for draft-creator; Approve / Reject for pending-checker-non-creator; Revise for rejected-creator; Cancel for pending-creator). Reject modal with reason (1–500, confirm disabled until valid). Actions disable during in-flight calls, refetch via TanStack Query invalidation on success, error toast (5s) on failure, 404 not-found state, loading skeleton.

Badge mapping (design system Section 8): draft = neutral, pending_approval = warning, approved = success, rejected = danger, processing = info, completed = success, cancelled = neutral (strikethrough). Never brand red for status.

Navigation: add entries to `NAV_CONFIG` in `src/lib/config/navigation.ts` under a Replenishment group for roles ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV.

---

## Interfaces

### API Contract

Base path `/api/v1/vendor-requests`. All endpoints require authentication (401 `unauthorized` if missing/invalid token). Money is integer IDR (bigint); dates are `YYYY-MM-DD`; timestamps are RFC3339.

| Method | Path | Purpose | Write roles | Req |
|---|---|---|---|---|
| GET | `/forecast?forecast_date=&terminal_id=&page=&page_size=` | Browse DMAA forecast for a date | ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV | 3 |
| POST | `/` | Create draft from selected items | ADMIN, ATM-USER, BRANCH-ATM-USER | 4 |
| PUT | `/{id}/items` | Replace all items (draft only) | ADMIN, ATM-USER, BRANCH-ATM-USER | 8 |
| POST | `/{id}/submit` | draft → pending_approval | ADMIN, ATM-USER, BRANCH-ATM-USER | 5 |
| POST | `/{id}/approve` | pending_approval → approved | ADMIN, ATM-SPV, BRANCH-ATM-SPV | 6 |
| POST | `/{id}/reject` | pending_approval → rejected (reason) | ADMIN, ATM-SPV, BRANCH-ATM-SPV | 6 |
| POST | `/{id}/revise` | rejected → draft | ADMIN, ATM-USER, BRANCH-ATM-USER | 7 |
| POST | `/{id}/cancel` | draft/pending → cancelled | creator or checker per state | 10 |
| GET | `/` | List with filters + pagination | ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV | 9 |
| GET | `/{id}` | Detail with items | same viewer set | 9 |
| GET | `/{id}/audit-log` | Audit trail | ADMIN, ATM-SPV, BRANCH-ATM-SPV | 16 |

**GET `/forecast`** success (200):

```json
{
  "data": [
    { "terminal_id": "ATM001", "periode_pred": "2026-09-12", "denom": 100000,
      "amount_replenish": 500000000, "amount_refund": 0, "dmaa_file_id": 42 }
  ],
  "pagination": { "page": 1, "page_size": 20, "total_count": 137, "total_pages": 7 }
}
```

**POST `/`** body → 201:

```json
{
  "forecast_date": "2026-09-12",
  "notes": "prioritas wilayah Jakarta",
  "items": [
    { "terminal_id": "ATM001", "periode_pred": "2026-09-12", "denom": 100000, "amount_replenish": 500000000 }
  ]
}
```

**GET `/`** row shape (Req 9.4): `id, request_number, forecast_date, status, notes, item_count, total_amount, created_by {id, full_name}, approved_by {id, full_name}?, created_at, submitted_at?, approved_at?, rejected_at?` plus `pagination {page, page_size, total_count, total_pages}`.

**POST `/{id}/reject`** body: `{ "rejection_reason": "amount melebihi plafon" }` (required, 1–500 chars after trim).

**GET `/{id}/audit-log`** row shape (Req 16.4): `id, entity_type, entity_id, action, performed_by, performed_at, previous_state, new_state, metadata` (projected from `audit_logs.actor_id/created_at/before.state/after`).

### Error responses

| Status | Code | Condition |
|---|---|---|
| 400 | `bad_request` | invalid query/body params, duplicate or unknown forecast items |
| 401 | `unauthorized` | missing/invalid token |
| 403 | `forbidden` | wrong role, not creator, or four-eyes violation |
| 404 | `not_found` | vendor request id does not exist |
| 409 | `conflict` | state does not allow the action; empty items on submit; number exhausted |
| 422 | `validation_error` | missing/whitespace rejection reason |
| 500 | `internal_error` | unexpected failure (incl. number generation exhaustion of retries) |

---

## Data Models

### Backend (service-level)

```go
type ItemInput struct {
    TerminalID      string // atm_id in requirements
    PeriodePred     time.Time
    Denom           int32
    AmountReplenish int64
}

type CreateVendorRequestInput struct {
    ForecastDate time.Time
    Notes        string      // <= 500
    Items        []ItemInput // >= 1
}

type VendorRequestDetail struct {
    ID              int64
    RequestNumber   string
    ForecastDate    time.Time
    Status          string
    Notes           string
    CreatedBy       UserRef
    ApprovedBy      *UserRef
    RejectedBy      *UserRef
    RejectionReason string
    CreatedAt       time.Time
    UpdatedAt       time.Time
    SubmittedAt     *time.Time
    ApprovedAt      *time.Time
    RejectedAt      *time.Time
    Items           []VendorRequestItem
    TotalAmount     int64
}

type UserRef struct { ID int64; FullName string }
```

### Frontend (TypeScript)

```typescript
export type VendorRequestStatus =
  | "draft" | "pending_approval" | "approved" | "rejected"
  | "processing" | "completed" | "failed" | "cancelled";

export interface ForecastRow {
  terminal_id: string;
  periode_pred: string;
  denom: number;
  amount_replenish: number;
  amount_refund: number;
  dmaa_file_id: number;
}

export interface VendorRequestItem {
  id: number;
  terminal_id: string;
  periode_pred: string;
  denom: number;
  amount_replenish: number;
  amount_refund: number;
}

export interface VendorRequestSummary {
  id: number;
  request_number: string;
  forecast_date: string;
  status: VendorRequestStatus;
  notes: string | null;
  item_count: number;
  total_amount: number;
  created_by: { id: number; full_name: string };
  approved_by?: { id: number; full_name: string } | null;
  created_at: string;
  submitted_at?: string | null;
  approved_at?: string | null;
  rejected_at?: string | null;
}
```

---

## Error Handling

- **Transactional integrity**: create, update-items (full replace), and every status transition run in one DB transaction on the primary pool. A failed audit write rolls back the transition (audit and state stay consistent).
- **Concurrency on transitions**: `GetVendorRequestForUpdate` (`SELECT … FOR UPDATE`) serializes concurrent actions on the same request; the second actor sees the already-changed state and gets 409.
- **Number-generation races** (Req 15.3): rely on the `request_number` UNIQUE constraint; on violation, retry up to 3 times, then `ErrNumberGeneration` (500) and no row created.
- **Item validation** happens before any write; invalid/duplicate items → 400 with the offending items identified, no partial persistence.
- **Read-after-write**: detail/list refetch after a mutation reads the primary pool to avoid replica lag. Forecast browse may use the replica when the `dbRead` pool is wired.
- **Frontend**: loading skeletons, empty states, retry on fetch error, error toasts (5s) on mutation failure, and the create-then-submit split failure handled by landing on the draft detail page.

---

## Correctness Properties

### Property 1: Only valid transitions mutate state
*For any* current state `s` and action `a`, the request's state changes to `t` **iff** `(s, a) → t` is in the transition table; otherwise the state is unchanged and the service returns `ErrInvalidTransition` (409).
**Validates: Requirements 2.1, 2.2**

### Property 2: Four-eyes principle
*For any* approve or reject action, the acting user's id differs from `created_by`; a self-approval/self-rejection is rejected (403) with no state change.
**Validates: Requirements 2.7, 2.8, 6.6**

### Property 3: Actor-role gating
*For any* action, submit/cancel-from-draft/revise succeed only for the creator; approve/reject/cancel-from-pending succeed only for a Checker role. Unauthorized actors get 403 with no state change.
**Validates: Requirements 2.4, 2.5, 2.12, 5.5, 6.9, 7.4, 10.4**

### Property 4: Audit entry per transition
*For any* successful state transition, exactly one append-only `audit_logs` row is written with the acting user, action, previous state (in `before`), and new state (in `after`); no code path updates or deletes an audit row.
**Validates: Requirements 2.3, 16.1, 16.6**

### Property 5: Request-number uniqueness and format
*For any* created request, `request_number` matches `^VR-\d{8}-\d{4}$`, is unique, encodes the `forecast_date`, and is immutable across all subsequent transitions.
**Validates: Requirements 15.1, 15.6**

### Property 6: Item validity and no duplicates
*For any* create or update-items payload, every persisted item references an existing `dmaa_atm_forecast (terminal_id, periode_pred, denom)` row and no two persisted items share the same `(terminal_id, periode_pred, denom)`.
**Validates: Requirements 4.4, 4.6, 8.6**

### Property 7: Amount override is honored and logged
*For any* item whose `amount_replenish` differs from its matching forecast row, the request stores the payload value and the discrepancy appears in the audit metadata.
**Validates: Requirements 4.5**

### Property 8: Rejection requires a reason
*For any* reject action, a missing or whitespace-only `rejection_reason` is rejected (422) with no state change; a valid reason is persisted with `rejected_by` and `rejected_at`.
**Validates: Requirements 2.11, 6.4, 6.12**

### Property 9: Submit requires at least one item
*For any* submit action on a request with zero items, the service returns 409 and the state stays `draft`.
**Validates: Requirements 5.4**

### Property 10: Pagination metadata consistency
*For any* list or forecast-browse request over `N` matching rows, `total_pages == ceil(total_count / page_size)`, `len(data) <= page_size`, and echoed `page`/`page_size` match the request.
**Validates: Requirements 3.5, 9.5**

### Property 11: List total_amount equals item sum
*For any* listed request, `total_amount` equals the sum of its items' `amount_replenish`.
**Validates: Requirements 9.4**

### Property 12: Timestamp/actor stamping on transition
*For any* transition to pending_approval/approved/rejected, the service stamps `submitted_at` / (`approved_at`, `approved_by`) / (`rejected_at`, `rejected_by`, `rejection_reason`) respectively.
**Validates: Requirements 2.9, 2.10, 2.11**

### Property 13: URL state round-trip (frontend)
*For any* valid combination of list/browse search params, encoding to URL and parsing back via `validateSearch` yields the same state.
**Validates: Requirements 13.3, 13.4**

### Property 14: Status badge mapping (frontend)
*For any* status value, the rendered badge variant follows the fixed mapping (draft=neutral, pending_approval=warning, approved=success, rejected=danger, processing=info, completed=success, cancelled=neutral) and never uses brand red.
**Validates: Requirements 13.2, 14.1**
