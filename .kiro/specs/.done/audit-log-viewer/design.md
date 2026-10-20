# Design Document: Audit Log Viewer

## Overview

The Audit Log Viewer is a read-only, admin-only capability with two halves:

1. **Backend read API** on the ATM backend (`backend/`, port 8080): `GET /api/v1/audit-logs` (filtered, paginated list) and `GET /api/v1/audit-logs/{id}` (single entry with full before/after JSON). It reads the generic, append-only `audit_logs` table and returns the ATM backend's existing **flat JSON shape** (not the `pkg/response` envelope) for wire compatibility with the internal frontend.

2. **Frontend page** in `frontend/CompanyPortal-Vite/` at `/audit-logs`: a paginated, filterable table (`DataTable`) with a detail drawer that renders a before/after diff. Access is restricted to `ADMIN` and `ADMIN_PARAM` via the existing `_protected` route guard.

The viewer never writes to `audit_logs`; the table stays append-only, owned by the `internal/audit` writer from RBAC-Setup Task 3.

---

## Dependency Position

```
RBAC-Setup Task 3  ──►  023_audit_logs.sql        (append-only table)
                        internal/audit.Write(...)  (the sole writer)
                                │
                                │  reads
                                ▼
audit-log-viewer  ──►  internal/handler/audit_log_handler.go   (GET list + GET detail)
                       internal/service/audit_log_read.go       (filter/paginate/map)
                       internal/repository + queries/audit_log.sql (sqlc)
                                │  flat JSON
                                ▼
                       frontend/CompanyPortal-Vite  /audit-logs
```

**Coordination rule:** if RBAC-Setup Task 3 has already created `023_audit_logs.sql`, this spec reuses it as-is and adds only read queries. If not, this spec's Backend Task 1 creates that exact migration (same columns, same append-only intent) so the two specs converge on one table — never two. The migration number is whatever is next free at implementation time (`022`+); confirm before creating to avoid a collision with an in-flight RBAC-Setup migration.

---

## Backend Design

### `audit_logs` schema (canonical, from RBAC-Setup)

```sql
-- 0XX_audit_logs.sql  (append-only; created by RBAC-Setup Task 3 or here if absent)
CREATE TABLE IF NOT EXISTS public.audit_logs (
    id          bigserial PRIMARY KEY,
    actor_id    bigint,                       -- nullable: system/batch actions
    action      text        NOT NULL,         -- e.g. 'approval.submit', 'atm.update'
    entity_type text        NOT NULL,         -- e.g. 'approval_request', 'atm'
    entity_id   bigint,                       -- nullable: some actions have no single entity
    before      jsonb,                        -- null for create
    after       jsonb,                        -- null for delete
    ip          text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Read-path indexes for the viewer's filters + default ordering.
CREATE INDEX IF NOT EXISTS audit_logs_created_at_idx  ON public.audit_logs (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS audit_logs_actor_idx       ON public.audit_logs (actor_id);
CREATE INDEX IF NOT EXISTS audit_logs_entity_idx      ON public.audit_logs (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS audit_logs_action_idx      ON public.audit_logs (action);
```

If the base table is created by RBAC-Setup, this spec contributes only the read-path indexes above (in a follow-on additive migration) — no column changes.

### Layer layout (ATM backend, actual technical-layer convention)

```
backend/
  internal/
    handler/audit_log_handler.go        # Routes(): GET /, GET /{id}; flat JSON via writeJSON/writeError
    service/audit_log_read.go           # filter validation, pagination clamp, row -> response mapping
    repository/audit_log_repository.go  # wraps *db.Queries, maps db rows -> domain structs
    db/                                 # sqlc-generated (do not hand-edit)
  queries/audit_log.sql                 # sqlc: ListAuditLogs, CountAuditLogs, GetAuditLogByID
```

### sqlc queries (`queries/audit_log.sql`)

The list query uses nullable filter parameters so a single query covers all combinations (the `sqlc.narg` / `COALESCE`-guard pattern). Ordering and pagination are fixed by the query; `page_size`/offset come from the service after clamping.

```sql
-- name: ListAuditLogs :many
SELECT id, actor_id, action, entity_type, entity_id, before, after, ip, created_at
FROM audit_logs
WHERE (sqlc.narg('actor_id')::bigint    IS NULL OR actor_id    = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::text        IS NULL OR action      = sqlc.narg('action'))
  AND (sqlc.narg('entity_type')::text   IS NULL OR entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('entity_id')::bigint   IS NULL OR entity_id   = sqlc.narg('entity_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz   IS NULL OR created_at <= sqlc.narg('date_to'))
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditLogs :one
SELECT count(*)
FROM audit_logs
WHERE (sqlc.narg('actor_id')::bigint    IS NULL OR actor_id    = sqlc.narg('actor_id'))
  AND (sqlc.narg('action')::text        IS NULL OR action      = sqlc.narg('action'))
  AND (sqlc.narg('entity_type')::text   IS NULL OR entity_type = sqlc.narg('entity_type'))
  AND (sqlc.narg('entity_id')::bigint   IS NULL OR entity_id   = sqlc.narg('entity_id'))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::timestamptz   IS NULL OR created_at <= sqlc.narg('date_to'));

-- name: GetAuditLogByID :one
SELECT id, actor_id, action, entity_type, entity_id, before, after, ip, created_at
FROM audit_logs
WHERE id = $1;
```

> `LIMIT`/`OFFSET` are positional (`$1`/`$2`) while filters are named (`sqlc.narg`). sqlc supports mixing; confirm generation with `sqlc generate` under the existing `backend/sqlc.yaml` (`sql_package: pgx/v5`, `emit_pointers_for_null_types`, `emit_json_tags`). If mixing positional and named args is rejected by the configured sqlc version, switch `LIMIT`/`OFFSET` to `sqlc.arg('limit')` / `sqlc.arg('offset')`.

### Read path / replica note

Per project-context Sec 5, reads should target the replica. The replica pool is **not yet wired** (single `dbPool` + `ponytail` TODO). The repository uses `dbPool` for these read queries and leaves a `// TODO(ponytail): route to replica pool once wired` comment, matching the convention established in RBAC-Setup.

### Service — validation and mapping (`service/audit_log_read.go`)

```go
type ListAuditLogsParams struct {
    ActorID    *int64
    Action     *string
    EntityType *string
    EntityID   *int64
    DateFrom   *time.Time // UTC, already converted from Asia/Jakarta by the handler
    DateTo     *time.Time
    Page       int32
    PageSize   int32
}

const (
    defaultPageSize int32 = 25
    maxPageSize     int32 = 100
)
```

Service responsibilities:
- Clamp `PageSize` into `[1, maxPageSize]`, default `defaultPageSize`; `Page >= 1`, default `1`; compute `offset = (page-1) * pageSize`.
- Reject `DateFrom` after `DateTo` with a `ValidationError` (same type used in `service/atm_portal.go`).
- Run `CountAuditLogs` + `ListAuditLogs` and assemble the paginated result.
- Map `before`/`after` (`[]byte` / `pgtype.JSONB` from sqlc) to `json.RawMessage` so the handler emits them as JSON objects, not escaped strings.

### Handler — flat JSON (`handler/audit_log_handler.go`)

Mirrors `dsr_upload_handler.go`: `Routes() chi.Router`, `writeJSON`/`writeError` helpers from `internal/handler/error_response.go`, actor/ip from `middleware.GetAuthContext` + `RealIP` (though this endpoint does not write audit, it may log access at debug level only).

```go
func (h *AuditLogHandler) Routes() chi.Router {
    r := chi.NewRouter()
    r.Get("/", h.List)        // GET /api/v1/audit-logs
    r.Get("/{id}", h.GetByID) // GET /api/v1/audit-logs/{id}
    return r
}
```

The handler parses query params, converts `date_from`/`date_to` (accepted as `YYYY-MM-DD` or RFC3339, interpreted in Asia/Jakarta) to UTC, calls the service, and writes flat JSON.

**List response (flat JSON):**

```json
{
  "data": [
    {
      "id": 4211,
      "actor_id": 7,
      "action": "approval.submit",
      "entity_type": "approval_request",
      "entity_id": 88,
      "ip": "10.20.1.5",
      "created_at": "2026-09-08T02:14:33Z"
    }
  ],
  "page": 1,
  "page_size": 25,
  "total": 3120
}
```

> The list response omits `before`/`after` to keep pages light; the full payload is fetched on demand via the detail endpoint.

**Detail response (flat JSON):**

```json
{
  "id": 4211,
  "actor_id": 7,
  "action": "approval.submit",
  "entity_type": "approval_request",
  "entity_id": 88,
  "before": null,
  "after": { "status": "pending", "required_level": 3, "amount": "150000000.00" },
  "ip": "10.20.1.5",
  "created_at": "2026-09-08T02:14:33Z"
}
```

### Route wiring (`cmd/api/main.go`)

```go
auditLogHandler := handler.NewAuditLogHandler(auditLogService)
r.With(mw.RequireAuth, mw.RequireRoles("ADMIN", "ADMIN_PARAM")).
    Mount("/api/v1/audit-logs", auditLogHandler.Routes())
```

### Error mapping

| Condition | HTTP | code |
|-----------|------|------|
| Missing/invalid token | 401 | (middleware) |
| Role not ADMIN/ADMIN_PARAM | 403 | (middleware) |
| Bad param (page/page_size/date) | 400 | `bad_request` |
| `date_from` after `date_to` | 400 | `bad_request` |
| Detail id not an integer | 400 | `bad_request` |
| Detail id not found | 404 | `not_found` |
| DB/internal failure | 500 | `internal_error` |

---

## Frontend Design

### Feature module structure

```
src/features/audit-log/
├── index.ts                       # barrel export
├── types.ts                       # API types, action-badge map, labels
├── hooks/
│   └── useAuditQueries.ts         # useAuditLogList, useAuditLogDetail
├── components/
│   ├── AuditLogPage.tsx           # orchestrator: URL-synced filters + pagination state
│   ├── AuditFilterBar.tsx         # Action / Entity Type / Actor / Date range + Reset
│   ├── AuditLogTable.tsx          # DataTable + pagination controls
│   ├── AuditDetailDrawer.tsx      # right drawer, fetches detail, renders diff
│   ├── BeforeAfterDiff.tsx        # before/after JSON diff renderer
│   └── SectionErrorState.tsx      # inline error + "Coba Lagi"
└── __tests__/
    ├── AuditLogPage.test.tsx
    ├── hooks.test.ts
    └── mappings.property.test.ts
```

### Route registration (`src/routes/audit-logs.tsx`)

```typescript
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";
import { AuditLogPage } from "@/features/audit-log";

export const auditLogsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/audit-logs",
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AuditLogPage,
});
```

### Types (`types.ts`)

```typescript
export interface AuditLogListItem {
  id: number;
  actor_id: number | null;
  action: string;
  entity_type: string;
  entity_id: number | null;
  ip: string | null;
  created_at: string; // ISO 8601 (UTC)
}

export interface AuditLogListResponse {
  data: AuditLogListItem[];
  page: number;
  page_size: number;
  total: number;
}

export interface AuditLogDetail extends AuditLogListItem {
  before: Record<string, unknown> | null;
  after: Record<string, unknown> | null;
}

export interface AuditLogFilters {
  actor_id: string | null;
  action: string | null;
  entity_type: string | null;
  entity_id: string | null;
  date_from: string | null; // YYYY-MM-DD (Asia/Jakarta)
  date_to: string | null;
}

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

// action -> badge variant. Suffix convention: *.delete/reject -> danger,
// *.approve -> success, *.submit/create -> info, *.update -> warning, else neutral.
export function actionBadgeVariant(action: string): BadgeVariant {
  if (/\.(delete|reject|fail)/.test(action)) return "danger";
  if (/\.(approve|complete|create)/.test(action)) return "success";
  if (/\.(submit|request)/.test(action)) return "info";
  if (/\.(update|edit)/.test(action)) return "warning";
  return "neutral";
}
```

> The action-to-variant map is heuristic on the `action` string convention. As real action names stabilize (from the `internal/audit` writer + module wiring), replace the regex with an explicit lookup table. Every badge still renders icon + label, so a wrong color never hides meaning.

### Query hooks (`hooks/useAuditQueries.ts`)

```typescript
export const auditKeys = {
  all: ["audit-log"] as const,
  list: (filters: AuditLogFilters, page: number, pageSize: number) =>
    [...auditKeys.all, "list", filters, page, pageSize] as const,
  detail: (id: number | null) => [...auditKeys.all, "detail", id] as const,
};

export function useAuditLogList(filters: AuditLogFilters, page: number, pageSize: number) {
  return useQuery({
    queryKey: auditKeys.list(filters, page, pageSize),
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set("page", String(page));
      params.set("page_size", String(pageSize));
      for (const [k, v] of Object.entries(filters)) if (v) params.set(k, v);
      const res = await api.get<AuditLogListResponse>(`/api/v1/audit-logs?${params}`);
      return res.data;
    },
    placeholderData: (prev) => prev, // keep prior page visible during pagination
  });
}

export function useAuditLogDetail(id: number | null) {
  return useQuery({
    queryKey: auditKeys.detail(id),
    queryFn: async () => {
      const res = await api.get<AuditLogDetail>(`/api/v1/audit-logs/${id}`);
      return res.data;
    },
    enabled: id !== null,
  });
}
```

### URL-synced filters + pagination

`AuditLogPage` reads/writes filters and page into the route search params (TanStack Router `useSearch` / `navigate({ search })`), satisfying Requirement 6.4 (shareable, refresh-safe). Changing any filter resets `page` to 1 (Requirement 6.2).

### Before/After diff (`BeforeAfterDiff.tsx`)

- Both present: compute the union of top-level keys, classify each as `unchanged | added | removed | modified` by shallow compare (deep-equal for nested objects via `JSON.stringify`), and render two columns with changed keys highlighted using semantic tokens (added → success tint, removed → danger tint, modified → warning tint). Never color alone — prefix changed keys with a `+ / − / ~` marker and an icon.
- `before` null → single "Dibuat" (created) panel showing `after`.
- `after` null → single "Dihapus" (deleted) panel showing `before`.

### Timestamp formatting

Reuse the WIB formatter convention from the EOD spec (`formatWibDateTimeSec` → `DD MMM YYYY HH:mm:ss`, `Asia/Jakarta`). If `date-fns-tz` is not installed, use `Intl.DateTimeFormat` with `timeZone: "Asia/Jakarta"` (no new dependency).

### Component responsibilities

| Component | Responsibility |
|-----------|----------------|
| `AuditLogPage` | Orchestrates URL-synced filters + page state, opens/closes the detail drawer via `selectedId`, renders PageHeader + filter bar + table. |
| `AuditFilterBar` | Renders Action/Entity/Actor/Date-range controls + Reset; validates date range client-side (Req 6.5); pushes changes to URL search. |
| `AuditLogTable` | Renders DataTable (6 columns), pagination controls from `{page, page_size, total}`, loading skeleton, empty state, row-click → open drawer. |
| `AuditDetailDrawer` | Fetches detail on `selectedId`, renders metadata + `BeforeAfterDiff`; focus trap, Escape/outside close, slide transition. |
| `BeforeAfterDiff` | Renders before/after JSON with change classification. |
| `SectionErrorState` | Inline error + "Coba Lagi" refetch. |

---

## Correctness Properties

### Property 1: Role-based access denial (frontend + backend)
*For any* user whose role is not `ADMIN` and not `ADMIN_PARAM`, navigating to `/audit-logs` SHALL render a Forbidden state and no audit data; likewise the `Audit_Read_API` SHALL respond 403. (Validates Req 1.3, 1.7)

### Property 2: Filter parameter fidelity
*For any* combination of non-null filters, the list request SHALL include exactly those filter values as query parameters and the backend SHALL return only rows matching all of them. (Validates Req 2.2–2.6, 6.2)

### Property 3: Page size clamping
*For any* requested `page_size`, the effective page size used by the query SHALL be within `[1, 100]`, defaulting to 25 when absent. (Validates Req 2.7)

### Property 4: Deterministic ordering
*For any* result page, entries SHALL be ordered by `created_at` descending then `id` descending, producing a stable, non-overlapping sequence across pages. (Validates Req 2.8)

### Property 5: Date range validation
*For any* `date_from` strictly after `date_to`, the backend SHALL respond 400 and the frontend SHALL block the request with a validation message. (Validates Req 2.10, 6.5)

### Property 6: Read-only surface
*For any* HTTP method other than GET on the audit-log routes, the API SHALL NOT expose a create/update/delete handler; no code path in this feature SHALL write, update, or delete `audit_logs` rows. (Validates Req 4.1–4.3)

### Property 7: Detail payload passthrough
*For any* existing entry, the detail endpoint SHALL return `before`/`after` as JSON objects (or null) byte-equivalent in structure to what is stored — no re-encoding as strings, no field dropping. (Validates Req 3.4)

### Property 8: Not-found and bad-id handling
*For any* non-existent integer id the detail endpoint SHALL return 404; *for any* non-integer id it SHALL return 400. (Validates Req 3.2, 3.3)

### Property 9: Action-to-badge mapping + accessibility
*For any* `action` string, `actionBadgeVariant` SHALL return one of the five variants, and the rendered Badge SHALL always contain both an icon and a text label. (Validates Req 5.4, 9.4)

### Property 10: Timestamp WIB formatting
*For any* valid ISO 8601 timestamp, the displayed value SHALL be `DD MMM YYYY HH:mm:ss` in `Asia/Jakarta`. (Validates Req 5.3, 7.2)

### Property 11: Diff classification correctness
*For any* pair of before/after objects, each top-level key SHALL be classified as added (only in after), removed (only in before), modified (in both, different value), or unchanged (in both, equal). (Validates Req 7.4)

### Property 12: URL filter round-trip
*For any* set of active filters, serializing to URL search params and re-parsing SHALL reconstruct the identical filter state (refresh/share safe). (Validates Req 6.4)

---

## Testing Strategy

**Backend (Go, table-driven + httptest):**
- Repository/integration against a real Postgres (per project-context Sec 7): seed `audit_logs`, assert filter combinations, ordering, pagination boundaries, count vs page correctness.
- Service unit tests: page/size clamping, date-range validation, before/after mapping to `json.RawMessage`.
- Handler `httptest`: 200 list/detail happy path; 400 bad params; 404 missing id; 401 no token; 403 wrong role; flat-JSON shape assertions.

**Frontend (Vitest + RTL):**
- Property tests for `actionBadgeVariant`, WIB formatting, diff classification, URL round-trip.
- Component tests: table renders rows + pagination, empty state, loading skeleton, filter change resets to page 1 and refetches, row click opens drawer, drawer renders diff, error state + retry.
- Accessibility: badges carry icon+label, 44px touch targets, focus trap in drawer, semantic landmarks.

**Coverage:** ≥ 80% on touched `internal/*` and the feature module (project-context Sec 7).

---

## Security Constraints

1. Route guard `requireRoles(["ADMIN","ADMIN_PARAM"])` in `beforeLoad`; backend `RequireRoles` at the mount, enforced independently of the frontend (defense in depth, project-context Sec 4 RBAC "at middleware AND service layer").
2. All requests use the existing `api` client (Bearer token injection + 401 refresh).
3. `before`/`after` payloads may contain sensitive field values already recorded by writers; the viewer displays them only to ADMIN/ADMIN_PARAM and never logs full payloads server-side. IP is shown as stored.
4. No new outbound network calls; no third-party data egress.

---

## Dependencies

No new backend dependencies (Chi, pgx, sqlc, go-playground/validator already in use). No new frontend dependencies required; `date-fns-tz` is optional (fallback to `Intl.DateTimeFormat`). Reuses shared components: Badge, DataTable, PageHeader, FilterSelect, EmptyState, Card, Button, Skeleton, Toast.
