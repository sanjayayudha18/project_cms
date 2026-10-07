---
inclusion: auto
name: rbac-role-feature-matrix
description: Per-role feature access map for CMS — the 10 roles, the menu/feature catalog, which features each role can access, backend route guards, and frontend nav/route guards. Use when working on roles, permissions, nav visibility, route guards, or answering "what can role X access".
---

# RBAC Role → Feature Matrix (CMS)

> Companion to `rbac-design.md` (which covers the hierarchy + maker-checker approval model). This doc maps **each role to the features/menus it can access**, and reconciles the two parallel authorization systems in the codebase.
>
> Source of truth (read together):
> - Roles + `menu_features` catalog + `role_permissions` grants: `backend/migrations/002_baseline_seed.sql`
> - Actual backend enforcement: `RequireRoles(...)` in `backend/cmd/api/main.go`
> - Frontend nav visibility: `frontend/CompanyPortal-Vite/src/lib/config/navigation.ts`
> - Frontend route guards: `frontend/CompanyPortal-Vite/src/routes/**` (`requireRoles(...)` in `_protected.tsx`)
> - Middleware: `pkg/middleware/rbac.go`

---

## ⚠️ Two parallel authorization systems

The codebase has **two** authz layers that must be read together — they roughly agree but have deliberate asymmetries:

1. **`role_permissions` DB catalog** (`menu_features` × `roles`). Data-driven, admin-editable via the "Manajemen Peran" screen. Today it drives **frontend nav rendering + the Role Management UI only**.
2. **Hardcoded `RequireRoles(...)` route guards** in `backend/cmd/api/main.go`. This is the **actual backend enforcement** today.

There is **no** `RequirePermission` / `PermissionEvaluator` in `pkg/middleware/rbac.go` — authz is **role-string driven** (`RequireRoles` compares the single JWT `role` claim against an allow-list, case-insensitive). The role-management spec intends the DB catalog to eventually replace the hardcoded guards, but that has not happened yet.

**Approval/maker-checker is a third, separate axis.** It keys off `supervisor_id` + `approval_level` (not `role`) — see `rbac-design.md`. That is why `/api/v1/approvals` is `RequireAuth`-only.

---

## 1. The 10 roles

| id | Role (exact string) | Portal | Description |
|----|--------------------|--------|-------------|
| 1 | `ADMIN` | Internal | Super administrator — full system access |
| 2 | `ADMIN_PARAM` | Internal | Parameter admin — master data (ATMs, regions, vendors, denoms) |
| 3 | `ATM-USER` | Internal | ATM operations staff — daily DSR upload, cash count, order entry |
| 4 | `ATM-SPV` | Internal | ATM supervisor — approve orders, review forecasts, monitor cash flow |
| 5 | `BRANCH-USER` | Internal | Branch operations staff — branch cash management |
| 6 | `BRANCH-SPV` | Internal | Branch supervisor — approve branch-level transactions |
| 7 | `BRANCH-ATM-USER` | Internal | Branch ATM operator — combined branch + ATM ops |
| 8 | `BRANCH-ATM-SPV` | Internal | Branch ATM supervisor — approve combined branch + ATM ops |
| 9 | `VENDOR-USER` | Vendor | CIT vendor user — view schedules, upload delivery confirmations |
| 10 | `APPACCESS` | Internal | Application access admin — account provisioning + RBAC CRUD-mapping/delegation config |

### Planned / not yet seeded

These roles are **not yet in the DB seed (`roles`), the frontend `DbRole` union, or any route guard** — they are recorded here as planned. Descriptions and portal assignment are TBD (confirm with business before implementing).

| Role (exact string) | Portal | Description |
|---|---|---|
| `ACM-USER` | TBD | TBD |
| `ACM-SPV` | TBD | TBD |
| `CMOC-USER` | TBD | TBD |
| `CMOC-SPV` | TBD | TBD |
| `CMOC-PIC` | TBD | TBD |
| `VENDOR-USER-CMOC` | TBD | TBD |

> The `Vendor CIT` / `Vendor CIT Supervisor` sub-roles mentioned in `project-context.md` are **spec-only** — not present in the seed data or the frontend `DbRole` union yet.

---

## 2. Menu / feature catalog (`menu_features`)

8 top-level menus + 3 nested features under Settings:

| id | parent | key | label | kind |
|----|--------|-----|-------|------|
| 1 | — | `dashboard` | Dashboard | menu |
| 2 | — | `cash-flow` | Cash Flow Monitoring | menu |
| 3 | — | `monitoring` | Monitoring | menu |
| 4 | — | `forecasting` | Peramalan | menu |
| 5 | — | `replenish` | Replenish | menu |
| 6 | — | `invoice` | Tagihan | menu |
| 7 | — | `cash-count` | Perhitungan Kas | menu |
| 8 | — | `settings` | Pengaturan | menu |
| 9 | 8 | `settings.roles` | Manajemen Peran | feature |
| 10 | 9 | `settings.roles.create` | Buat Peran | feature |
| 11 | 9 | `settings.roles.edit` | Ubah Izin Peran | feature |

---

## 3. Role × feature grants (from `role_permissions` seed)

Legend: ✅ granted · — not granted. New roles created after seeding start with **zero** access by design.

| Feature ↓ / Role → | ADMIN | ADMIN_PARAM | APPACCESS | ATM-USER | ATM-SPV | BRANCH-ATM-USER | BRANCH-ATM-SPV | BRANCH-USER | BRANCH-SPV | VENDOR-USER |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `dashboard` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `cash-flow` | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | — | — | — |
| `monitoring` | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | — | — | — |
| `forecasting` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | ✅ |
| `replenish` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | — | — | — |
| `invoice` | ✅ | ✅ | ✅ | — | — | — | — | ✅ | ✅ | ✅ |
| `cash-count` | ✅ | ✅ | ✅ | — | — | ✅ | ✅ | — | — | — |
| `settings` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — |
| `settings.roles` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — |
| `settings.roles.create` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — |
| `settings.roles.edit` | ✅ | ✅ | ✅ | — | — | — | — | — | — | — |

**Summary per role:**
- **ADMIN / ADMIN_PARAM / APPACCESS** — all 11 features (full catalog).
- **ATM-USER / ATM-SPV** — dashboard, cash-flow, monitoring, forecasting, replenish.
- **BRANCH-ATM-USER / BRANCH-ATM-SPV** — dashboard, forecasting, replenish, cash-count.
- **BRANCH-USER / BRANCH-SPV** — dashboard, invoice.
- **VENDOR-USER** — dashboard, forecasting, invoice.

### Planned roles — no feature grants yet

The following roles have **no `menu_features` granted yet** (intentionally left blank pending business definition). They are not seeded into `role_permissions`.

| Feature ↓ / Role → | ACM-USER | ACM-SPV | CMOC-USER | CMOC-SPV | CMOC-PIC | VENDOR-USER-CMOC |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| `dashboard` | — | — | — | — | — | — |
| `cash-flow` | — | — | — | — | — | — |
| `monitoring` | — | — | — | — | — | — |
| `forecasting` | — | — | — | — | — | — |
| `replenish` | — | — | — | — | — | — |
| `invoice` | — | — | — | — | — | — |
| `cash-count` | — | — | — | — | — | — |
| `settings` | — | — | — | — | — | — |
| `settings.roles` | — | — | — | — | — | — |
| `settings.roles.create` | — | — | — | — | — | — |
| `settings.roles.edit` | — | — | — | — | — | — |

---

## 4. Backend route guards (`backend/cmd/api/main.go`)

All routes are wrapped in `RequireAuth` first. This table is the **real enforcement**.

| Route prefix | Role guard |
|---|---|
| `/api/v1/auth` | public (login/refresh) |
| `/api/v1/admin/users` | `APPACCESS`, `ADMIN`, `ADMIN_PARAM` |
| `/api/v1/admin/vendors` | `ADMIN`, `ADMIN_PARAM` |
| `/api/v1/admin/atms` | `ADMIN`, `ADMIN_PARAM` |
| `/api/v1/admin/roles` | `APPACCESS`, `ADMIN` (self-guarded by static roles to avoid bootstrap deadlock) |
| `/api/v1/admin/approval` | `ADMIN`, `ADMIN_PARAM`, `APPACCESS` (hierarchy/delegation/leave/policy) |
| `/api/v1/audit-logs` | `ADMIN`, `ADMIN_PARAM` |
| `/api/v1/dmaa-forecast` | `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`, `ADMIN`, `ADMIN_PARAM` |
| `/api/v1/atm-portal` | any authenticated (`RequireAuth` only) |
| `/api/v1/dsr` | any authenticated (`RequireAuth` only) |
| `/api/v1/approvals` | any authenticated (`RequireAuth` only — orchestrator enforces maker≠checker) |
| `/api/v1/vendor-requests` | any authenticated (`RequireAuth` only — per-route/actor subset inside handler+service) |

> `backend-cit` has **no** role-guarded routes yet — only `/health` + a `RequireAuth`-only placeholder group.

**Gaps to be aware of:** `/api/v1/dsr`, `/api/v1/atm-portal`, `/api/v1/vendor-requests` are `RequireAuth`-only. Any authenticated user (including `VENDOR-USER`) can reach them at the HTTP layer; finer-grained authz is either absent or pushed to the service layer.

---

## 5. Frontend guards (CompanyPortal-Vite)

Three layers, evaluated independently of the backend.

### (a) Sidebar visibility — `filterNavByRoles()`
- **ADMIN and ADMIN_PARAM see every nav item** (early return, ignores the item's `roles` list).
- Items with `roles: ["*"]` show for all authenticated users (e.g. `dashboard`).
- Otherwise the user's role must be in the item's `roles` array.

Active nav items and their allowed roles:

| Nav item (label) | Roles |
|---|---|
| Dashboard | `*` |
| Cash Flow Monitoring | ATM-USER, ATM-SPV |
| Pengaturan | ADMIN, ADMIN_PARAM, APPACCESS |
| Rekomendasi CIT | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| Request CIT | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| Jadwal CIT | ATM-USER, ATM-SPV |
| ATM Portal | ATM-USER, ATM-SPV |
| EOD Monitoring | ADMIN, ADMIN_PARAM |
| DSR Dashboard | ATM-USER, ATM-SPV, VENDOR-USER |
| DMAA Forecast | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| Daftar Invoice | BRANCH-USER, BRANCH-SPV, VENDOR-USER |
| Rekonsiliasi | BRANCH-USER, BRANCH-SPV |

(Several other items exist but are `disabled` or commented out: Kalender Libur, Unggah Invoice, Penjadwalan, Pelaksanaan BA, Rekapitulasi, etc.)

### (b) Route guards — `requireRoles([...])` in route files
| Route | Roles |
|---|---|
| `/settings` | ADMIN, ADMIN_PARAM, APPACCESS |
| `/settings/roles` | **APPACCESS only** |
| `/settings/rbac/{users,policies,leaves,delegations}` | ADMIN, ADMIN_PARAM, APPACCESS |
| `/settings/admin/users` | **APPACCESS only** (stricter than backend, which also allows ADMIN/ADMIN_PARAM) |
| `/settings/admin/vendors` | ADMIN, ADMIN_PARAM |
| `/settings/admin/atms` | ADMIN, ADMIN_PARAM |
| `/replenishment/forecast-browser`, `/replenishment/vendor-requests` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| `/replenishment/vendor-requests/new` | MAKER_ROLES (maker subset) |
| `/forecasting/dmaa-forecast` | ATM-USER, ATM-SPV, BRANCH-ATM-USER, BRANCH-ATM-SPV |
| `/eod-monitoring` | ADMIN, ADMIN_PARAM |
| `/audit-logs` | ADMIN, ADMIN_PARAM |

### (c) Settings hub card visibility — `SettingsHubPage.tsx`
Only the "Manajemen Peran" card is role-gated at the card level (`canSeeRoleManagementCard` → APPACCESS or ADMIN). Every other card relies solely on its target route's `beforeLoad` guard (a known, documented gap: ADMIN_PARAM sees the other cards on the hub).

> VendorPortal-Vite has **no** centralized nav config equivalent to `navigation.ts`.

---

## 6. Known asymmetries (intentional or to-be-fixed)

- **`/admin/users`** — backend allows `APPACCESS`+`ADMIN`+`ADMIN_PARAM`; frontend route+card is `APPACCESS`-only. Deliberate.
- **DB catalog vs route guards** — `role_permissions` grants `settings` to ADMIN/ADMIN_PARAM/APPACCESS, but `/settings/roles` and `/settings/admin/users` are frontend-gated to APPACCESS only. The DB catalog is not yet the enforcement authority.
- **`RequireAuth`-only routes** — `dsr`, `atm-portal`, `vendor-requests`, `approvals` are not role-gated at the route. For `approvals` this is by design (hierarchy handles it); the others are a coarser-than-catalog gap.

---

## Conventions

- Update this matrix whenever a role is added, a `menu_features` entry changes, a `role_permissions` seed grant changes, or a `RequireRoles(...)` / `requireRoles(...)` guard changes.
- When the role-management catalog becomes the backend enforcement authority (replacing hardcoded `RequireRoles`), revise Section 4 and the "two parallel systems" note.
- Keep the exact role strings verbatim (`ATM-USER`, not `atm_user`) — guards compare case-insensitively but the seed strings are the canonical identifiers.
