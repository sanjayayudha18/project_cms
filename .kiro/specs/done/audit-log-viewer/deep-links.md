# Deep-linking into the Audit Log Viewer

Other screens (invoice, DSR, approval detail) can show "who did what to *this* record" by linking to the viewer pre-filtered by entity. The viewer keeps all of its state in the URL, so a link is just a path plus search params.

## URL contract

`/audit-logs` accepts these search params (all optional; defaults are omitted from the URL):

| Param | Type | Notes |
|---|---|---|
| `entity_type` | string | e.g. `vendor_request`, `approval_request`, `master_data_change_request`, `user` |
| `entity_id` | integer | pair it with `entity_type` for a single-entity history |
| `action` | string | e.g. `approve`, `reject` |
| `actor_id` | integer | who performed the action |
| `date_from`, `date_to` | `YYYY-MM-DD` | Asia/Jakarta calendar days, inclusive; `date_from` must not be after `date_to` |
| `page`, `page_size` | integer | `page_size` 1-100, default 25 |

Single-entity history (newest first):

```tsx
import { Link } from "@tanstack/react-router";

<Link to="/audit-logs" search={{ entity_type: "vendor_request", entity_id: request.id }}>
  Riwayat audit
</Link>
```

The schema is `AUDIT_LOG_SEARCH_SCHEMA` in `frontend/CompanyPortal-Vite/src/features/audit-log/useAuditLogUrlState.ts`. There is no `entity_id` input in the filter bar: a deep-linked entity filter is applied and visible through the results, and **Atur Ulang** clears it.

## Access

The route requires `ADMIN` or `ADMIN_PARAM` (`requireRoles`), and so does `GET /api/v1/audit-logs`. Only render the link for those roles; other roles get a 403 from the API even if they reach the page.

## Hard loads (refresh, pasted URL, new tab)

A deep link works on a full page load as well as from a `<Link>` click. This used to fail (observed 2026-09-21: any protected route landed on `/`), from two causes, both fixed:

1. **Guard ran before the session was restored.** On a hard load `isAuthLoading` is true and `user` is null while the refresh cookie is exchanged. `requireRoles` treated that as "not signed in" and redirected to `/login`, which bounced the signed-in user to `/`. `requireRoles` now does nothing while auth is loading, and `RootComponent` calls `router.invalidate()` after `initialize()` resolves so the role checks and the requested path are honoured.
2. **`initialize()` raced itself.** React StrictMode runs the root effect twice in dev; refresh tokens rotate, so the second `POST /auth/refresh` got a 401 and overwrote the restored session with "unauthenticated" (intermittent, which is why it seemed to depend on the route). `initialize()` is now single-flight, like `refreshToken()`.

Not changed: the login page does not read the `?redirect=` param, so a user who is signed **out** still lands on `/` after logging in rather than on the requested page.

## Backend

`GET /api/v1/audit-logs?entity_type=&entity_id=` is served by `audit_logs_entity_idx`; the `(created_at DESC, id DESC)` ordering uses `audit_logs_created_at_id_idx` (migration `006_audit_logs_read_indexes.sql`).
