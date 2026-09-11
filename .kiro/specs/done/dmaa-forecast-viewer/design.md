# Design Document: DMAA Forecast Viewer

## Overview

A read-only page for viewing ATM cash forecast data from the `dmaa_atm_forecast` table. The feature consists of a backend GET endpoint with server-side pagination, filtering, and sorting, plus a frontend page using TanStack Table and TanStack Query with URL state synchronization.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Frontend (CompanyPortal-Vite)                                      │
│                                                                     │
│  Route: /forecasting/dmaa-forecast                                  │
│  ┌──────────────────┐   ┌────────────────┐   ┌──────────────────┐  │
│  │ TanStack Router  │──▶│ TanStack Query │──▶│  TanStack Table  │  │
│  │ (URL search      │   │ (fetch + cache) │   │  (headless       │  │
│  │  params state)   │   │                 │   │   rendering)     │  │
│  └──────────────────┘   └────────┬───────┘   └──────────────────┘  │
│                                  │                                  │
└──────────────────────────────────┼──────────────────────────────────┘
                                   │ GET /api/v1/dmaa-forecast
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Backend (Go + Chi)                                                 │
│                                                                     │
│  ┌─────────────┐   ┌──────────────────┐   ┌─────────────────────┐  │
│  │  Handler    │──▶│     Service      │──▶│   Repository        │  │
│  │ (parse QP,  │   │ (validate,       │   │   (sqlc-generated   │  │
│  │  write JSON)│   │  orchestrate)    │   │    from dbRead)     │  │
│  └─────────────┘   └──────────────────┘   └──────────┬──────────┘  │
│                                                       │             │
└───────────────────────────────────────────────────────┼─────────────┘
                                                        │
                                                        ▼
                                              ┌──────────────────┐
                                              │  PostgreSQL      │
                                              │  Read Replica    │
                                              └──────────────────┘
```

---

## Components

### Backend

#### 1. Handler — `internal/handler/dmaa_forecast_handler.go`

Responsible for HTTP concerns only: parsing query parameters, calling the service, and writing JSON responses via the shared `writeJSON` / `writeError` helpers.

```go
package handler

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/cimb-niaga/cms/backend/internal/service"
)

type DmaaForecastHandler struct {
	service service.DmaaForecastServicer
}

func NewDmaaForecastHandler(svc service.DmaaForecastServicer) *DmaaForecastHandler {
	return &DmaaForecastHandler{service: svc}
}

func (h *DmaaForecastHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.ListDmaaForecast)
	return r
}

func (h *DmaaForecastHandler) ListDmaaForecast(w http.ResponseWriter, r *http.Request) {
	params, err := parseDmaaForecastParams(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.service.ListDmaaForecast(r.Context(), params)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toDmaaForecastResponse(result))
}
```

Query parameter parsing follows the existing `parseIntParam` / `queryOrDefault` pattern:

```go
func parseDmaaForecastParams(q url.Values) (service.ListDmaaForecastParams, error) {
	page, err := parseIntParam(q, "page", defaultPage)
	if err != nil {
		return service.ListDmaaForecastParams{}, fmt.Errorf("page harus berupa angka")
	}
	pageSize, err := parseIntParam(q, "page_size", defaultPageSize)
	if err != nil {
		return service.ListDmaaForecastParams{}, fmt.Errorf("page_size harus berupa angka")
	}

	return service.ListDmaaForecastParams{
		Page:       page,
		PageSize:   pageSize,
		DateFrom:   q.Get("date_from"),
		DateTo:     q.Get("date_to"),
		TerminalID: q.Get("terminal_id"),
		SortBy:     queryOrDefault(q, "sort_by", "periode_pred"),
		SortOrder:  queryOrDefault(q, "sort_order", "desc"),
	}, nil
}
```

Error mapping (`handleServiceError`) delegates to `writeError` / `writeServiceUnavailable` as appropriate.

#### 2. Service — `internal/service/dmaa_forecast.go`

Business logic layer: validates params, calls repository, assembles result. Defines its own repository interface per Go convention.

```go
package service

import "context"

// ListDmaaForecastParams holds validated query parameters.
type ListDmaaForecastParams struct {
	Page       int
	PageSize   int
	DateFrom   string // YYYY-MM-DD or empty
	DateTo     string // YYYY-MM-DD or empty
	TerminalID string // substring search or empty
	SortBy     string
	SortOrder  string // "asc" or "desc"
}

// DmaaForecastRow is the domain representation of a single forecast row.
type DmaaForecastRow struct {
	TerminalID      string
	DmaaFileID      int64
	PeriodePred     time.Time
	Denom           int
	AmountReplenish int64
	AmountRefund    int64
	CreatedAt       time.Time
}

// ListDmaaForecastResult wraps the paginated response.
type ListDmaaForecastResult struct {
	Rows       []DmaaForecastRow
	Page       int
	PageSize   int
	TotalRows  int64
	TotalPages int
}

// DmaaForecastServicer is the interface the handler depends on.
type DmaaForecastServicer interface {
	ListDmaaForecast(ctx context.Context, params ListDmaaForecastParams) (*ListDmaaForecastResult, error)
}

// DmaaForecastRepository is the interface the service depends on (satisfied by sqlc).
type DmaaForecastRepository interface {
	ListDmaaForecast(ctx context.Context, arg db.ListDmaaForecastParams) ([]db.ListDmaaForecastRow, error)
	CountDmaaForecast(ctx context.Context, arg db.CountDmaaForecastParams) (int64, error)
}

// DmaaForecastService implements DmaaForecastServicer.
type DmaaForecastService struct {
	repo DmaaForecastRepository
}
```

**Validation rules** (in `params.validate()`):
- `page >= 1`
- `1 <= page_size <= 100`
- `date_from` / `date_to` parse to `YYYY-MM-DD` when non-empty
- `sort_by` must be in allowed set: `terminal_id`, `dmaa_file_id`, `periode_pred`, `denom`, `amount_replenish`, `amount_refund`, `created_at`
- `sort_order` must be `asc` or `desc`

**Pagination**: `TotalPages = ceil(TotalRows / PageSize)`.

#### 3. Repository (sqlc) — `queries/dmaa_forecast.sql`

Two queries: one for listing with LIMIT/OFFSET, one for counting (same filters, no pagination).

```sql
-- name: ListDmaaForecast :many
SELECT terminal_id, dmaa_file_id, periode_pred, denom,
       amount_replenish, amount_refund, created_at
FROM dmaa_atm_forecast
WHERE
  (sqlc.narg('date_from')::date IS NULL OR periode_pred >= sqlc.narg('date_from')::date)
  AND (sqlc.narg('date_to')::date IS NULL OR periode_pred <= sqlc.narg('date_to')::date)
  AND (sqlc.narg('terminal_id')::text IS NULL OR terminal_id ILIKE '%' || sqlc.narg('terminal_id')::text || '%')
ORDER BY
  CASE WHEN @sort_by = 'terminal_id' AND @sort_order = 'asc' THEN terminal_id END ASC,
  CASE WHEN @sort_by = 'terminal_id' AND @sort_order = 'desc' THEN terminal_id END DESC,
  CASE WHEN @sort_by = 'dmaa_file_id' AND @sort_order = 'asc' THEN dmaa_file_id END ASC,
  CASE WHEN @sort_by = 'dmaa_file_id' AND @sort_order = 'desc' THEN dmaa_file_id END DESC,
  CASE WHEN @sort_by = 'periode_pred' AND @sort_order = 'asc' THEN periode_pred END ASC,
  CASE WHEN @sort_by = 'periode_pred' AND @sort_order = 'desc' THEN periode_pred END DESC,
  CASE WHEN @sort_by = 'denom' AND @sort_order = 'asc' THEN denom END ASC,
  CASE WHEN @sort_by = 'denom' AND @sort_order = 'desc' THEN denom END DESC,
  CASE WHEN @sort_by = 'amount_replenish' AND @sort_order = 'asc' THEN amount_replenish END ASC,
  CASE WHEN @sort_by = 'amount_replenish' AND @sort_order = 'desc' THEN amount_replenish END DESC,
  CASE WHEN @sort_by = 'amount_refund' AND @sort_order = 'asc' THEN amount_refund END ASC,
  CASE WHEN @sort_by = 'amount_refund' AND @sort_order = 'desc' THEN amount_refund END DESC,
  CASE WHEN @sort_by = 'created_at' AND @sort_order = 'asc' THEN created_at END ASC,
  CASE WHEN @sort_by = 'created_at' AND @sort_order = 'desc' THEN created_at END DESC
LIMIT @page_size OFFSET @offset;

-- name: CountDmaaForecast :one
SELECT count(*) FROM dmaa_atm_forecast
WHERE
  (sqlc.narg('date_from')::date IS NULL OR periode_pred >= sqlc.narg('date_from')::date)
  AND (sqlc.narg('date_to')::date IS NULL OR periode_pred <= sqlc.narg('date_to')::date)
  AND (sqlc.narg('terminal_id')::text IS NULL OR terminal_id ILIKE '%' || sqlc.narg('terminal_id')::text || '%');
```

The repository is instantiated with the **read replica pool** (`dbRead`).

#### 4. Route Registration

Mounted at the router level with RBAC middleware:

```go
// In cmd/api route setup
r.Route("/api/v1/dmaa-forecast", func(r chi.Router) {
    r.Use(middleware.RequireRoles("ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV", "ADMIN", "ADMIN_PARAM"))
    r.Mount("/", dmaaForecastHandler.Routes())
})
```

---

### Frontend

#### 1. Route — `src/routes/forecasting/dmaa-forecast.tsx`

Uses TanStack Router's `validateSearch` for type-safe URL state.

```typescript
import { createRoute } from "@tanstack/react-router";
import { z } from "zod";
import { protectedRoute, requireRoles } from "../_protected";
import { DmaaForecastView } from "@/features/dmaa-forecast";

const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  pageSize: z.number().int().min(1).max(100).catch(25),
  dateFrom: z.string().optional(),
  dateTo: z.string().optional(),
  terminalId: z.string().optional(),
  sortBy: z.string().catch("periode_pred"),
  sortOrder: z.enum(["asc", "desc"]).catch("desc"),
});

export const dmaaForecastRoute = createRoute({
  path: "/forecasting/dmaa-forecast",
  getParentRoute: () => protectedRoute,
  validateSearch: (search) => searchSchema.parse(search),
  beforeLoad: requireRoles(["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"]),
  component: DmaaForecastView,
});
```

#### 2. Feature Module — `src/features/dmaa-forecast/`

```
src/features/dmaa-forecast/
├── index.ts                    # barrel export
├── DmaaForecastView.tsx        # page container (filters + table)
├── DmaaForecastTable.tsx       # column definitions + DataTable wrapper
├── DmaaForecastFilters.tsx     # date pickers + terminal search input
├── useDmaaForecastData.ts      # TanStack Query hook
├── types.ts                    # response/row TypeScript types
└── __tests__/                  # co-located tests
```

#### 3. TanStack Query Hook — `useDmaaForecastData.ts`

```typescript
import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { apiClient } from "@/lib/api/client";
import type { DmaaForecastResponse } from "./types";

interface DmaaForecastQueryParams {
  page: number;
  pageSize: number;
  dateFrom?: string;
  dateTo?: string;
  terminalId?: string;
  sortBy: string;
  sortOrder: "asc" | "desc";
}

export function useDmaaForecastData(params: DmaaForecastQueryParams) {
  return useQuery({
    queryKey: ["dmaa-forecast", params],
    queryFn: () =>
      apiClient.get<DmaaForecastResponse>("/api/v1/dmaa-forecast", {
        params: {
          page: params.page,
          page_size: params.pageSize,
          date_from: params.dateFrom,
          date_to: params.dateTo,
          terminal_id: params.terminalId,
          sort_by: params.sortBy,
          sort_order: params.sortOrder,
        },
      }),
    placeholderData: keepPreviousData,
  });
}
```

#### 4. Table Component — `DmaaForecastTable.tsx`

Uses the project's headless `DataTable` component with column definitions following existing patterns (right-aligned monetary columns, `tabular-nums`, `formatIDR`).

#### 5. Navigation Entry

Add to `NAV_CONFIG` in `navigation.ts`:

```typescript
{
  id: "dmaa-forecast",
  label: "DMAA Forecast",
  icon: TrendingUp,
  href: "/forecasting/dmaa-forecast",
  roles: ["ATM-USER", "ATM-SPV", "BRANCH-ATM-USER", "BRANCH-ATM-SPV"],
  group: "forecasting",
},
```

---

## Interfaces

### API Contract

**GET** `/api/v1/dmaa-forecast`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `page_size` | int | 25 | Rows per page (max 100) |
| `date_from` | string | — | Filter: periode_pred >= date (YYYY-MM-DD) |
| `date_to` | string | — | Filter: periode_pred <= date (YYYY-MM-DD) |
| `terminal_id` | string | — | Substring search (case-insensitive) |
| `sort_by` | string | `periode_pred` | Column to sort |
| `sort_order` | string | `desc` | `asc` or `desc` |

**Success Response (200)**:

```json
{
  "data": [
    {
      "terminal_id": "ATM001",
      "dmaa_file_id": 42,
      "periode_pred": "2025-01-15",
      "denom": 100000,
      "amount_replenish": 500000000,
      "amount_refund": 0,
      "created_at": "2025-01-13T10:30:00+07:00"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 25,
    "total_rows": 1250,
    "total_pages": 50
  }
}
```

**Error Responses**:

| Status | Code | Condition |
|--------|------|-----------|
| 400 | `bad_request` | Invalid params (non-numeric page, bad date, unknown sort column) |
| 401 | `unauthorized` | Missing/invalid auth token |
| 403 | `forbidden` | Authenticated but role not permitted |
| 503 | `service_unavailable` | Read replica unreachable |

---

## Data Models

### Backend Types

```go
// Service params (validated before reaching repo)
type ListDmaaForecastParams struct {
    Page       int    // >= 1
    PageSize   int    // 1–100
    DateFrom   string // "" or YYYY-MM-DD
    DateTo     string // "" or YYYY-MM-DD
    TerminalID string // "" or substring
    SortBy     string // allowed column
    SortOrder  string // "asc" | "desc"
}

// Service result
type ListDmaaForecastResult struct {
    Rows       []DmaaForecastRow
    Page       int
    PageSize   int
    TotalRows  int64
    TotalPages int
}

type DmaaForecastRow struct {
    TerminalID      string
    DmaaFileID      int64
    PeriodePred     time.Time
    Denom           int
    AmountReplenish int64
    AmountRefund    int64
    CreatedAt       time.Time
}
```

### Frontend Types

```typescript
export interface DmaaForecastRow {
  terminal_id: string;
  dmaa_file_id: number;
  periode_pred: string;       // ISO date "YYYY-MM-DD"
  denom: number;
  amount_replenish: number;   // bigint as number (safe for display)
  amount_refund: number;
  created_at: string;         // ISO timestamp
}

export interface PaginationMeta {
  page: number;
  page_size: number;
  total_rows: number;
  total_pages: number;
}

export interface DmaaForecastResponse {
  data: DmaaForecastRow[];
  pagination: PaginationMeta;
}
```

---

## Error Handling

| Layer | Error | Response |
|-------|-------|----------|
| Handler | Non-numeric page/page_size | 400 `bad_request` with Indonesian message |
| Service | Invalid date format | 400 `bad_request` |
| Service | Unsupported sort_by column | 400 `bad_request` |
| Service | page_size out of range | 400 `bad_request` |
| Repository | DB connection failure | 503 `service_unavailable` |
| Middleware | No auth token | 401 `unauthorized` |
| Middleware | Insufficient role | 403 `forbidden` |
| Frontend | API error | Error state with retry button |
| Frontend | Loading | Skeleton/spinner within table area |

Service errors are typed: `*ValidationError` maps to 400, DB errors map to 503 via `handleServiceError`.

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Role-based access control

*For any* user role, the DMAA Forecast API grants access if and only if the role is in the set {ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV, ADMIN, ADMIN_PARAM}. Navigation visibility follows the same rule.

**Validates: Requirements 1.3, 1.4, 2.1, 2.3**

### Property 2: Pagination metadata consistency

*For any* valid page and page_size request against a dataset of N total rows, the response satisfies: `total_pages == ceil(total_rows / page_size)` AND `len(data) <= page_size` AND `pagination.page == requested page` AND `pagination.page_size == requested page_size`.

**Validates: Requirements 3.1, 3.4**

### Property 3: Invalid pagination parameter rejection

*For any* non-numeric string provided as `page` or `page_size`, the API responds with HTTP 400 and error code `bad_request`.

**Validates: Requirements 3.5**

### Property 4: Date range filter correctness

*For any* valid date_from and/or date_to and any dataset, every row in the response satisfies: `periode_pred >= date_from` (when date_from provided) AND `periode_pred <= date_to` (when date_to provided).

**Validates: Requirements 4.2, 4.3, 4.4**

### Property 5: Invalid date rejection

*For any* string that does not match the YYYY-MM-DD format provided as `date_from` or `date_to`, the API responds with HTTP 400 and error code `bad_request`.

**Validates: Requirements 4.5**

### Property 6: Terminal ID substring filter

*For any* non-empty terminal_id search value S and any dataset, every row in the response has a `terminal_id` that contains S as a case-insensitive substring.

**Validates: Requirements 5.2**

### Property 7: Sort ordering

*For any* allowed sort_by column and sort_order direction, the returned rows are ordered according to that column in the specified direction.

**Validates: Requirements 6.2**

### Property 8: Invalid sort_by rejection

*For any* string not in the allowed column set {terminal_id, dmaa_file_id, periode_pred, denom, amount_replenish, amount_refund, created_at} provided as `sort_by`, the API responds with HTTP 400 and error code `bad_request`.

**Validates: Requirements 6.5**

### Property 9: IDR monetary formatting

*For any* integer amount, the `formatIDR` function produces a string with dot-separated thousands and "Rp" prefix (e.g., 500000000 → "Rp 500.000.000").

**Validates: Requirements 7.2**

### Property 10: Date formatting (DD MMM YYYY)

*For any* valid date value, the date format function produces output matching the pattern DD MMM YYYY in Asia/Jakarta timezone.

**Validates: Requirements 7.3**

### Property 11: Timestamp formatting (DD MMM YYYY HH:mm)

*For any* valid timestamp value, the timestamp format function produces output matching the pattern DD MMM YYYY HH:mm in Asia/Jakarta timezone.

**Validates: Requirements 7.4**

### Property 12: Denomination number formatting

*For any* positive integer denomination, the format function produces a dot-separated thousands string (e.g., 100000 → "100.000").

**Validates: Requirements 7.5**

### Property 13: Filter change resets pagination

*For any* current page state where page > 1, changing any filter (dateFrom, dateTo, terminalId) resets the page to 1.

**Validates: Requirements 4.7, 5.3**

### Property 14: URL state round-trip

*For any* valid combination of search parameters (page, pageSize, dateFrom, dateTo, terminalId, sortBy, sortOrder), encoding them into URL query params and then parsing back via the route's `validateSearch` produces the same state.

**Validates: Requirements 10.1, 10.2**
