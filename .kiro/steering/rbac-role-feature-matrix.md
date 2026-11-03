---
inclusion: auto
name: rbac-role-feature-matrix
description: Per-role feature access map for CMS — the 14 roles, the menu/feature catalog, which features each role can access, backend route guards, and frontend nav/route guards. Use when working on roles, permissions, nav visibility, route guards, or answering "what can role X access".
---

# RBAC Role → Feature Matrix (CMS)

> Companion to `rbac-design.md` (which covers the hierarchy + maker-checker approval model). This doc maps **each role to the features/menus it can access**, and reconciles the two parallel authorization systems in the codebase.
>
> Last verified against code: **2026-10-23** (`dev1`, migrations through `032`).
>
> Source of truth (read together):
> - Roles + `menu_features` catalog + `role_permissions` grants: `backend/migrations/002_baseline_seed.sql`, then `027_acm_vault_assignment.sql` (ACM roles + CIT menu) and `032_vendor_party_scope.sql` (vendor party roles)
> - Actual backend enforcement: `RequireRoles(...)` at the mount in `backend/cmd/api/main.go` **and** inside handler `Routes()` (e.g. `vendor_request_handler.go`, `vendor_order_handler.go`, `admin_holiday_handler.go`)
> - Frontend nav visibility: `frontend/CompanyPortal-Vite/src/lib/config/navigation.ts`
> - Frontend route guards: `frontend/CompanyPortal-Vite/src/routes/**` (`requireRoles(...)` from `_protected.tsx`)
> - Middleware: `pkg/middleware/rbac.go`

---

## ⚠️ Two parallel authorization systems

The codebase has **two** authz layers that must be read together — they roughly agree but have deliberate asymmetries:

1. **`role_permissions` DB catalog** (`menu_features` × `roles`). Data-driven, admin-editable via the "Manajemen Peran" screen. Today it drives **frontend nav rendering + the Role Management UI only**.
2. **Hardcoded `RequireRoles(...)` route guards**, at the mount in `cmd/api/main.go` or inside a handler's `Routes()`. This is the **actual backend enforcement** today. A mount that shows only `RequireAuth` in `main.go` may still be role-guarded per route inside the handler — always open the handler before calling a route "unguarded".

There is **no** `RequirePermission` / `PermissionEvaluator` in `pkg/middleware/rbac.go` — authz is **role-string driven** (`RequireRoles` compares the single JWT `role` claim against an allow-list, case-insensitive). The role-management spec intends the DB catalog to eventually replace the hardcoded guards, but that has not happened yet.

**Approval/maker-checker is a third, separate axis.** It keys off `supervisor_id` + `approval_level` (not `role`) — see `rbac-design.md`. That is why `/api/v1/approvals` is `RequireAuth`-only.

---

## 1. The 14 roles

| Role (exact string) | Portal | Seeded in | Description |
|--------------------|--------|-----------|-------------|
| `ADMIN` | Internal | baseline `002` | Super administrator — full system access |
| `ADMIN_PARAM` | Internal | baseline `002` | Parameter admin — master data (ATMs, regions, vendors, denoms) |
| `ATM-USER` | Internal | baseline `002` | ATM operations staff — Vendor Request maker, monitoring |
| `ATM-SPV` | Internal | baseline `002` | ATM supervisor — Vendor Request checker, vault review |
| `BRANCH-USER` | Internal | baseline `002` | Branch operations staff — branch cash management |
| `BRANCH-SPV` | Internal | baseline `002` | Branch supervisor — approve branch-level transactions |
| `BRANCH-ATM-USER` | Internal | baseline `002` | Branch ATM operator — Vendor Request maker (branch scope) |
| `BRANCH-ATM-SPV` | Internal | baseline `002` | Branch ATM supervisor — Vendor Request checker (branch scope) |
| `VENDOR-USER` | Vendor | baseline `002` | Vendor user, both parties (replenish + vault), vendor-wide or branch-scoped |
| `APPACCESS` | Internal | baseline `002` | Application access admin — account provisioning + RBAC CRUD-mapping/delegation config |
| `ACM-USER` | Internal | `027` | ACM staff — allocates the request nominal to vaults (Penetapan Vault) |
| `ACM-SPV` | Internal | `027` | ACM supervisor — approves the area vault plan |
| `VENDOR-ATM-USER` | Vendor | `032` | Vendor pengisian ATM — replenish party only, branch scope `users.vendor_branch_id` |
| `VENDOR-VAULT-USER` | Vendor | `032` | Vendor penyedia uang di vault — vault party only, branch scope `users.vendor_branch_id` |

Role `id`s for `027`/`032` rows are identity-assigned at migration time; reference roles by the `role` string, never by id.

Shared role lists in code: `vendorrole.All` (`backend/internal/vendorrole`) = the three vendor roles; `vendorRequestViewerRoles` = ADMIN, ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV; `vendorRequestMakerRoles` = ADMIN, ATM-USER, BRANCH-ATM-USER; `vendorRequestCheckerRoles` = ADMIN, ATM-SPV, BRANCH-ATM-SPV (`backend/internal/handler/vendor_request_handler.go`).

### Planned / not yet seeded

These roles are **not yet in the DB seed (`roles`), the frontend `DbRole` union, or any route guard**. Descriptions and portal assignment are TBD (confirm with business before implementing).

| Role (exact string) | Portal | Description |
|---|---|---|
| `CMOC-USER` | TBD | TBD |
| `CMOC-SPV` | TBD | TBD |
| `CMOC-PIC` | TBD | TBD |
| `VENDOR-USER-CMOC` | TBD | TBD |

> The `Vendor CIT` / `Vendor CIT Supervisor` sub-roles mentioned in `project-context.md` are **spec-only**. The implemented vendor split is `VENDOR-ATM-USER` / `VENDOR-VAULT-USER` (party), not uploader vs supervisor.

---

## 2. Menu / feature catalog (`menu_features`)

9 top-level menus + 4 nested features. Ids 1-11 come from the baseline; the `027` rows are identity-assigned.

| id | parent | key | label | kind | Source |
|----|--------|-----|-------|------|--------|
| 1 | — | `dashboard` | Dashboard | menu | baseline |
| 2 | — | `cash-flow` | Cash Flow Monitoring | menu | baseline |
| 3 | — | `monitoring` | Monitoring | menu | baseline |
| 4 | — | `forecasting` | Peramalan | menu | baseline |
| 5 | — | `replenish` | Replenish | menu | baseline |
| 6 | — | `invoice` | Tagihan | menu | baseline |
| 7 | — | `cash-count` | Perhitungan Kas | menu | baseline |
| 8 | — | `settings` | Pengaturan | menu | baseline |
| 9 | 8 | `settings.roles` | Manajemen Peran | feature | baseline |
| 10 | 9 | `settings.roles.create` | Buat Peran | feature | baseline |
| 11 | 9 | `settings.roles.edit` | Ubah Izin Peran | feature | baseline |
| — | — | `cit` | CIT | menu | `027` |
| — | `cit` | `cit.vault-plan` | Penetapan Vault | feature | `027` |
| — | `settings` | `settings.acm-areas` | Area ACM | feature | `027` |

---

## 3. Role × feature grants (from `role_permissions` seed)

Legend: ✅ granted · — not granted. New roles created after seeding start with **zero** access by design.

| Feature ↓ / Role → | ADMIN | ADMIN_PARAM | APPACCESS | ATM-USER | ATM-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | BRANCH-USER | BRANCH-SPV | VENDOR-USER* | ACM-USER | ACM-SPV |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `dashboard` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — | — |
| `cash-flow` | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | — | — | — | — | — |
| `monitoring` | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | — | — | — | — | — |
| `forecasting` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | ✅ | — | — |
| `replenish` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | — | — | — |
| `invoice` | ✅ | ✅ | ✅ | — | — | — | — | ✅ | ✅ | ✅ | — | — |
| `cash-count` | ✅ | ✅ | ✅ | — | — | ✅ | ✅ | — | — | — | — | — |
| `settings` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — | — | — |
| `settings.roles` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — | — | — |
| `settings.roles.create` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — | — | — |
| `settings.roles.edit` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — | — | — |
| `cit` | ✅ | — | — | — | — | — | — | — | — | — | ✅ | ✅ |
| `cit.vault-plan` | ✅ | — | — | — | — | — | — | — | — | — | ✅ | ✅ |
| `settings.acm-areas` | ✅ | — | — | — | — | — | — | — | — | — | — | — |

\* `VENDOR-ATM-USER` and `VENDOR-VAULT-USER` carry the same grants as `VENDOR-USER` (copied in `032`). Party (replenish vs vault) + branch scoping is enforced in the data layer of VendorPortal Orders / Realisasi / notifications, not in this menu matrix.

**Summary per role:**
- **ADMIN** — all 14 catalog entries.
- **ADMIN_PARAM / APPACCESS** — the 11 baseline entries (no CIT, no Area ACM).
- **ATM-USER / ATM-SPV** — dashboard, cash-flow, monitoring, forecasting, replenish.
- **BRANCH-ATM-USER / BRANCH-ATM-SPV** — dashboard, forecasting, replenish, cash-count.
- **BRANCH-USER / BRANCH-SPV** — dashboard, invoice.
- **VENDOR-USER / VENDOR-ATM-USER / VENDOR-VAULT-USER** — dashboard, forecasting, invoice.
- **ACM-USER / ACM-SPV** — cit, cit.vault-plan only (no dashboard grant).

---

## 4. Backend route guards

All routes except `/api/v1/auth` are wrapped in `RequireAuth` first. This table is the **real enforcement**. "Mount" = guard in `main.go`; "Handler" = guard inside the handler's `Routes()`.

| Route prefix | Guard | Where |
|---|---|---|
| `/api/v1/auth` | public (login/refresh) | — |
| `/api/v1/admin/users` | `APPACCESS`, `ADMIN` (ADMIN_PARAM removed, vendor-party-scope D8; re-checked in `UserAdminService`) | Mount |
| `/api/v1/admin/roles` | `APPACCESS`, `ADMIN` (static roles to avoid bootstrap deadlock) | Mount |
| `/api/v1/admin/approval` | `ADMIN`, `ADMIN_PARAM`, `APPACCESS` (hierarchy/delegation/leave/policy) | Mount |
| `/api/v1/admin/vendors` (+ `/branches`, `/branches/{id}/atms`, `/vaults`, `/pics`, `/packages`, `/package-prices`, `/dsr-location-maps`) | `ADMIN`, `ADMIN_PARAM` | Mount (`masterDataAdmin` group) |
| `/api/v1/admin/atms` (+ `/assignments`, `/assignment-package-options`) | `ADMIN`, `ADMIN_PARAM` | Mount (`masterDataAdmin`) |
| `/api/v1/admin/regions` | `ADMIN`, `ADMIN_PARAM` | Mount (`masterDataAdmin`) |
| `/api/v1/admin/master-data/{changes,import}` | `ADMIN`, `ADMIN_PARAM` | Mount (`masterDataAdmin`) |
| `/api/v1/admin/acm-areas` | `ADMIN` | Mount |
| `/api/v1/admin/holidays` | `ADMIN` (+ service re-check) | Handler |
| `/api/v1/audit-logs` | `ADMIN`, `ADMIN_PARAM` | Mount |
| `/api/v1/dmaa-forecast` | `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`, `ADMIN`, `ADMIN_PARAM` | Mount |
| `/api/v1/vault-plans` | `ACM-USER`, `ACM-SPV`, `ADMIN` (membership + per-action role re-checked in service; ADMIN read-only) | Mount |
| `/api/v1/vendor-requests` | per route: viewer / maker / checker / cancel / audit role lists (see Sec 1) | Handler |
| `/api/v1/vendor/replenish-orders` | `vendorrole.All` (+ service re-checks role, party, branch scope) | Handler |
| `/api/v1/vendor/replenish-realizations` | `vendorrole.All` | Handler |
| `/api/v1/replenishment` | `vendorRequestViewerRoles` (+ service re-check) | Handler |
| `/api/v1/atm-visit-quotas` | GET viewer roles; reset/cancel checker roles | Handler |
| `/api/v1/atm-portal` | any authenticated (`RequireAuth` only) | — |
| `/api/v1/dsr` | any authenticated (`RequireAuth` only; vendor scoping in service) | — |
| `/api/v1/notifications` | any authenticated (own notifications only) | — |
| `/api/v1/approvals` | any authenticated (orchestrator enforces maker≠checker + hierarchy) | — |

> `backend-exq` (renamed from `backend-cit` 2026-10-20) mounts the export API `/api/v1/exports` (spec `vendor-data-export`) behind `RequireAuth`; `/health` and `/ready` are unguarded. The old `/api/v1/admin/master-data/export/*` route no longer exists in `backend/`.

| Route prefix (`backend-exq`) | Guard | Where |
|---|---|---|
| `POST /api/v1/exports/vendors/{vendorID}/{entity}`, `POST …/branches/{branchID}/{entity}`, `POST …/package-prices`, `POST …/master-data/{entity}`, `GET …/master-data/{entity}/template` | `ADMIN`, `ADMIN_PARAM` (per kind, `exportjob.KindRoles`) | Handler (+ service re-check on create) |
| `GET /api/v1/exports/jobs`, `GET /api/v1/exports/jobs/{jobID}` | any authenticated; own jobs only (another user's job = 404) | Service |
| `GET /api/v1/exports/jobs/{jobID}/file` | owner; role re-checked from DB at download (403) | Service |
| `POST /api/v1/exports/reports/dsr-late-daily`, `POST …/reports/dsr-late-summary` | `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-SPV`, `ADMIN` (`reports.DsrLateViewerRoles`, per `KindSpecs`) | Handler (`RequireRoles`) |
| `POST /api/v1/exports/reports/replenishment-amount-mismatch`, `POST …/reports/replenishment-trips` | `ADMIN`, `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV` (`reports.ReplenishmentViewerRoles`, per `KindSpecs`) | Handler (`RequireRoles`) |

> Report CSV (spec `report-export-migration`, 2026-10-29): `format=csv` on `backend/` `/dsr-late-reports/{daily,summary}` and `/replenishment/reports/{amount-mismatch,trips}` now returns 400 `validation_error` (field `format`). The CSV is produced only by the `backend-exq` report routes above; the JSON responses are unchanged.

**Gaps to be aware of:** `/api/v1/dsr` and `/api/v1/atm-portal` are `RequireAuth`-only at the HTTP layer, so any authenticated user (including vendor roles) reaches them; finer-grained authz is either in the service layer or absent.

---

## 5. Frontend guards (CompanyPortal-Vite)

Three layers, evaluated independently of the backend. CompanyPortal rejects all three vendor roles at login and refresh (`lib/auth/store.ts`).

### (a) Sidebar visibility — `filterNavByRoles()`
- **ADMIN and ADMIN_PARAM see every nav item** (early return, ignores the item's `roles` list).
- Items with `roles: ["*"]` show for all authenticated users.
- Otherwise the user's role must be in the item's `roles` array.

Active nav items (`navigation.ts`, mock screens removed from the menu by FE-05):

| Nav item (label) | Route | Roles |
|---|---|---|
| Dashboard | `/` | `*` |
| Cash Flow Monitoring | `/cash-flow` | ATM-USER, ATM-SPV |
| Persetujuan | `/approvals` | `*` |
| Pengaturan | `/settings` | ADMIN, ADMIN_PARAM, APPACCESS |
| Rekomendasi CIT | `/replenishment/forecast-browser` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| Request CIT | `/replenishment/vendor-requests` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| Jadwal CIT | `/replenishment` | ATM-USER, ATM-SPV |
| Penetapan Vault | `/cit/vault-plans` | ACM-USER, ACM-SPV |
| ATM Portal | `/atm-portal` | ATM-USER, ATM-SPV |
| EOD Monitoring | `/eod-monitoring` | ADMIN, ADMIN_PARAM |
| Log Audit | `/audit-logs` | ADMIN, ADMIN_PARAM |
| DMAA Forecast | `/forecasting/dmaa-forecast` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| Kalender Libur | `/settings/admin/holidays` | ADMIN |

Commented out or `disabled`: Instruksi Pengisian, Validasi Pengisian, Cash Supply, Proyeksi H+2, Unggah Invoice, Perhitungan Beban, Pembuatan Dokumen, and all Perhitungan Kas items (Penjadwalan, Analisis Tier Saldo, Pelaksanaan (BA), Checklist, Rekonsiliasi, Rekapitulasi).

### (b) Route guards — `requireRoles([...])` in route files
`requireRoles()` lets ADMIN/ADMIN_PARAM through on every route (shared bypass in `_protected.tsx`), so the lists below are the extra roles allowed.

| Route | Roles |
|---|---|
| `/settings` | ADMIN, ADMIN_PARAM, APPACCESS |
| `/settings/roles` | **APPACCESS only** |
| `/settings/rbac/{users,policies,leaves,delegations}` | ADMIN, ADMIN_PARAM, APPACCESS |
| `/settings/admin/users` | **APPACCESS only** |
| `/settings/admin/vendors`, `/settings/admin/vendors/$vendorId`, `.../branches/$branchId/edit` | ADMIN, ADMIN_PARAM |
| `/settings/admin/atms`, `/settings/admin/regions`, `/settings/admin/master-data-io` | ADMIN, ADMIN_PARAM |
| `/settings/admin/exports` | ADMIN, ADMIN_PARAM (kartu "Export Data" di `SettingsHubPage`, tanpa item sidebar) |
| `/settings/admin/holidays`, `/settings/admin/acm-areas` | ADMIN |
| `/replenishment/forecast-browser`, `/replenishment/vendor-requests`, `/replenishment/vendor-requests/$id` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| `/replenishment/vendor-requests/new` | maker roles (own redirect, not `requireRoles`) |
| `/cit/vault-plans` | ACM-USER, ACM-SPV, ADMIN (same as backend) |
| `/forecasting/dmaa-forecast` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| `/eod-monitoring` | ADMIN, ADMIN_PARAM |
| `/audit-logs` | ADMIN, ADMIN_PARAM |
| `/exports` (Ekspor Saya, spec `report-export-migration`) | union of report viewer roles: ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV, ADMIN (+ ADMIN_PARAM via bypass); no sidebar item |

### (c) Settings hub card visibility — `SettingsHubPage.tsx`
Only the "Manajemen Peran" card is role-gated at the card level (`canSeeRoleManagementCard` → APPACCESS or ADMIN). Every other card relies solely on its target route's `beforeLoad` guard (a known, documented gap: ADMIN_PARAM sees the other cards on the hub).

> VendorPortal-Vite has **no** centralized nav config equivalent to `navigation.ts`. It accepts the three vendor roles (`src/lib/vendorRoles.ts`, `VENDOR_ROLES`).

---

## 6. Known asymmetries (intentional or to-be-fixed)

- **ADMIN_PARAM frontend bypass** — `filterNavByRoles()` and `requireRoles()` let ADMIN_PARAM see every menu and route, but the backend rejects ADMIN_PARAM on `/admin/users`, `/admin/roles`, `/admin/acm-areas`, `/admin/holidays`, and `/vault-plans`. ADMIN_PARAM reaching those screens gets 403s from the API.
- **`/admin/users`** — backend allows `APPACCESS` + `ADMIN`; frontend route is `APPACCESS`-only (plus the ADMIN/ADMIN_PARAM bypass). Deliberate.
- **DB catalog vs route guards** — `role_permissions` grants `settings` to ADMIN/ADMIN_PARAM/APPACCESS, but `/settings/roles` and `/settings/admin/users` are frontend-gated to APPACCESS. ACM roles have no `dashboard` grant but `Dashboard` nav is `*`. The DB catalog is not yet the enforcement authority.
- **`RequireAuth`-only routes** — `dsr`, `atm-portal`, `notifications`, `approvals` are not role-gated at the route. For `approvals` this is by design (hierarchy handles it); for `notifications` each user only reads their own; `dsr` / `atm-portal` are a coarser-than-catalog gap.

---

## Conventions

- Update this matrix whenever a role is added, a `menu_features` entry changes, a `role_permissions` seed grant changes, or a `RequireRoles(...)` / `requireRoles(...)` guard changes (in `main.go` **or** a handler's `Routes()`).
- When the role-management catalog becomes the backend enforcement authority (replacing hardcoded `RequireRoles`), revise Section 4 and the "two parallel systems" note.
- Keep the exact role strings verbatim (`ATM-USER`, not `atm_user`) — guards compare case-insensitively but the seed strings are the canonical identifiers.
