# Data Map — detailed notes

> Moved verbatim from `.claude/CLAUDE.md` Sec 3 on 2026-09-29 to keep CLAUDE.md small. CLAUDE.md keeps the canonical table list + the rules; this file keeps column-level detail and migration history. "Sec N" references point to CLAUDE.md.

## Role Management tables (`menu_features`, `role_permissions`)

*   `menu_features` — Menu_Feature_Catalog. `id` bigint identity PK · `parent_id` bigint self-FK → `menu_features(id)` ON DELETE RESTRICT, nullable (NULL = top-level menu, non-NULL = feature under a menu) · `key` text UNIQUE (stable machine key, e.g. `settings.roles`) · `label` text · `kind` text CHECK IN ('menu','feature') + CHECK hierarchy (`menu`⇔`parent_id IS NULL`) · `sort_order` integer default 0 · `is_active` boolean default true · `created_at`/`updated_at` timestamptz default now(). Index on `parent_id`.
*   `role_permissions` — Role_Permission_Mapping (many-to-many `roles`↔`menu_features`; row presence = grant). `id` bigint identity PK · `role_id` bigint FK → `roles(id)` ON DELETE CASCADE · `menu_feature_id` bigint FK → `menu_features(id)` ON DELETE CASCADE · `granted_by` bigint FK → `users(id)` · `created_at` timestamptz default now(). UNIQUE `(role_id, menu_feature_id)`; indexes on `role_id` and `menu_feature_id`.

## Master data — per-table notes

*   **Vendor package pricing** (plan: `.claude/sdlc/vendor-pricing/plan.md`; migrations `009_vendor_package_prices.sql` + `010_vendor_package_prices_data.sql`, applied to dev DB 2026-09-22). `vendor_packages.price`/`priority_class` were dropped — every row there is now just a kelolaan/frequency link, never a price. `ROH` (`vendors.kind='INTERNAL'`) is not an FLM vendor: it marks ATMs sited at a CIMB branch office, staffed by branch personnel, never billed. Its 300 `ROH_0xx` branches moved to the new `cimb_branches` table (backfilled onto `atms.cimb_branch_id`); its ~485 per-branch packages collapsed into 2 internal packages (`vendor_packages.vendor_branch_id IS NULL`). Real prices live in `vendor_package_prices` — three levels in one table (PT base / branch override / per-ATM override, distinguished by which of `vendor_branch_id`/`atm_id` is set, nullable per-field so an override can touch one price field and inherit the rest), effective-dated, `EXCLUDE USING gist` prevents overlapping tier+date ranges for the same vendor/package/machine-group/class/level. CR/FLM contract frequency is separate, in `package_frequencies(package_code, machine_group)` — frequency differs by machine type (e.g. PAKET 3 ATM = FLM 4, but PAKET 3 CDM/CRM = FLM 6), so it can't live as a column on `vendor_packages`. `atms.price_machine_group`/`price_class` are generated columns bridging the DB's 3-valued `machine_type`/`priority_class` onto the document's 2×2 tariff grid (ATM vs CDM_CRM; REGULAR vs VIP_INDUSTRI) — unmapped values resolve to NULL so lookups fail hard instead of silently mis-pricing. Seed data (P19) is **partial**: Advantage/Bijak/TAG/Abacus fully seeded from `archives/Harga Paket per vendor FLM.docx`; SSI only has Paket 4/5 (doc has no Paket 3/6 SSI pricing); per-ATM "Harga Khusus" (17 ATMs) and "Biaya Tambahan Operasional" (3 ATMs) are **not seeded** — both need a human-verified `vendor_id`/`atm_id` the source document doesn't state, deferred to the P20 admin screen. Backend (queries → repo → service with maker-checker → handler → `Applier`) and the CompanyPortal screen **are now built** — see `vendor-package-code-split` note below and Sec 12.
*   **`vendor_package_prices` identifier split into `package`/`package_code`** (plan: `.kiro/specs/vendor-package-code-split`; migration `017_vendor_package_code_split.sql`, applied to dev DB 2026-09-24). The column that used to hold the label ("PAKET 3") and double as the grouping key was **renamed** `package_code` → `package` (data, `NOT NULL`, and its role in `vpp_no_overlap`/`vpp_lookup_idx` all preserved — those two were rebuilt to reference `package` instead, otherwise byte-for-byte identical to migration 009's definitions). A **new** `package_code text` column was added, backfilled deterministically as `PKG<digits-of-package>_<vendor.code>_<3-digit per-vendor sequence ordered by id>` (digitless labels fall back to `PKG0_...`), then locked down with `NOT NULL` + a global `UNIQUE(package_code)` (`vendor_package_prices_package_code_key`). Any join from a price row to its frequency now matches `vendor_package_prices.package = package_frequencies.package_code` (that table's own `package_code` column is unchanged — it's still the label). On create, `package_code` is generated server-side by `VendorPackagePriceApplier.nextPackageCode` at apply-on-approve time (never client-supplied) inside the same transaction as the insert, serialized per-vendor via `pg_advisory_xact_lock`, with the global unique constraint as the final collision backstop (`23505` → `ErrVendorPackageCodeConflict`). The admin API (`/api/v1/admin/vendors/{vendorId}/package-prices`) now returns both fields; list/count filter by the `package` label (query param renamed from `package_code`, no legacy alias). `PackagePricesPanel.tsx` shows two columns — "Kode Paket" (`package_code`, mono/tabular) and "Paket" (`package`) — and the create form collects only the `package` label.
*   Known gap: `backend/internal/handler/integration_test.go` only runs migrations `001`+`002` against the test DB, so `003`–`010` (including this pricing schema) are not yet exercised by that integration test. Pre-existing, tracked separately.
*   `regions` gained soft-delete columns (migration `018_regions_soft_delete.sql`, `.kiro/specs/region-management`, additive, applied to dev DB — verified 2026-09-25) — `+ is_active boolean NOT NULL DEFAULT true` (backfills all 13 existing rows as active), `+ deleted_at timestamptz NULL`. `uq_regions_code` and the `updated_at` trigger already existed in the baseline, so this migration only adds the two soft-disable columns. Enables the upcoming "Manajemen Region" admin CRUD (immediate-apply-with-audit, like `role-management`, not maker-checker) — disable is blocked while any active `locations` row still references the region; never a hard `DELETE`.
*   `branch_coverage_areas` (migration `007_branch_coverage_areas.sql`, applied 2026-09-22) — kota yang dilayani sebuah cabang vendor, **many-to-many**: satu cabang melayani banyak kota dan satu kota dilayani banyak cabang vendor, sehingga tidak muat di `vendor_branches.region` (satu kolom text, dan `branch_code` UNIQUE). `id` bigint identity PK · `vendor_branch_id` bigint FK → `vendor_branches(id)` ON DELETE CASCADE · `city` text · `created_at` timestamptz default now(). UNIQUE `(vendor_branch_id, city)`; index on `city`. Seeded with 171 baris dari `archives/vault dan cashpoint/vendor_vault_region_branch.csv` (55 cabang × 87 kota). **Read-only saat ini** — belum punya queries/repo/handler; bila kelak dikelola dari layar, wajib mengikuti pola maker-checker master data (Sec 12: queries → repo read-only → service `Submit` → `Applier` di `main.go` → handler 202), bukan menulis tabel langsung.
*   `vendor_vaults.category` diperluas dari biner `('ATM','CASH')` menjadi `('ATM','CASH','ATM_CASH')` di migration `008_vendor_vault_types.sql` (applied 2026-09-22): daftar master vendor menyebut sebagian cabang melayani ATM sekaligus CASH. Sebelumnya seluruh 348 baris adalah placeholder `'ATM'` hasil backfill di `003_master_data.sql`; kini 99 cabang non-ROH terisi nilai sebenarnya (44 ATM_CASH / 43 CASH / 12 ATM), sedangkan 308 cabang ROH tidak ada di daftar itu dan masih placeholder. Kolom legacy `vendor_vaults.type` **selalu di-mirror dari `category`** (lihat `Create`/`UpdateVendorVaultAdmin` di `backend/queries/vendor_vaults_admin.sql`) — jangan perlakukan sebagai field independen. Migration 008 juga menambah vendor `BRINKS`/`KEJAR`/`PROSEGUR` + 51 cabang baru (total 407 cabang).

## ATM visit quota (migration 021)

Plan/spec: `.claude/sdlc/atm-visit-quota/`. Migration `021_atm_visit_quota.sql`, applied to dev DB 2026-10-01.

*   **Kuota source**: `package_frequencies.cr_frequency` for `(vendor_packages_branch.package_code, atms.price_machine_group)` of the ATM's active `atm_vendor_packages` row (same LATERAL + `avp.id DESC` tie-breaker as `ListForecastForDate`). Read-only — this feature never writes `package_frequencies`. `package_code` is free text with no FK, so a label without a `package_frequencies` row (or a NULL `price_machine_group`) = **kuota tidak diketahui**: never guessed.
*   `vendor_requests`: CHECK `vendor_requests_status_chk` gains `completion_pending`. New nullable columns `completion_submitted_by/at` (maker of the laporan selesai), `completion_approved_by/at`, `completion_rejected_by/at`, `completion_rejection_reason` (FKs → `users`). Flow: `approved → completion_pending → completed`; reject → back to `approved` (rejected_* kept until the next submit clears them). `cancel` is not allowed from `completion_pending`.
*   ~~`vendor_request_atm_results`~~ — dropped by migration 026; the per-ATM `result` now lives on the active `vendor_request_tickets` row (see below).
*   `atm_visit_quotas` — PK/FK `atm_id → atms`; `package_code` + `quota_total` (> 0) are a snapshot taken at reset or at the first counted visit; `remaining` integer, **may be negative** (display: sisa = max(remaining, 0), kelebihan = max(−remaining, 0)); `reset_at` (period start, `clock_timestamp()`), `reset_by` (NULL = auto-created by the first visit). `updated_at` trigger. No automatic reset — a checker resets it to the active package's `cr_frequency`; a package change does not touch the row until then.
*   `atm_visits` — identity PK; `atm_id`, `vendor_request_id`; UNIQUE `(vendor_request_id, atm_id)` (double approve can't decrement twice); `quota_known` (false = recorded without a kuota row); `is_over_quota`; `created_at` (`clock_timestamp()`, so the period boundary `created_at >= reset_at` orders correctly even inside one tx); soft-cancel `cancelled_at/by/cancel_reason` (all-or-none CHECK). Index `(atm_id, created_at DESC)`. Never hard-deleted; cancelling a current-period visit gives the kuota back (+1).

## Replenish ticket + request region (migration 026)

Plan/spec: `.claude/sdlc/replenish-ticket/`. Migration `026_replenish_ticket_region.sql`, applied to dev DB 2026-10-08 (backfill: 174 tickets for 174 (request, ATM) pairs; 356 cabang seeded with a code, 66 left NULL).

*   `vendor_branches.region_code` text, CHECK `^[A-Z0-9]{2,10}$`, nullable. Seeded from the free-text `region` (table in spec FR11.3); `region` itself is unchanged (Forecast Browser still uses it). Edited via the vendor-branch maker-checker form/CSV only. NULL = requests for ATMs of that cabang are rejected (422).
*   `vendor_requests.region_code` — snapshot at create; NULL = request numbered before 026.
*   `vendor_request_number_seq` — + `region_code`; PK replaced by `UNIQUE NULLS NOT DISTINCT (vendor_id, region_code, seq_date)` (old rows keep NULL).
*   `vendor_request_tickets` — identity PK; `request_number` FK → `vendor_requests(request_number)` ON DELETE RESTRICT; `terminal_id` text; `denom_code` `<n>K|MIX`; `replenish_date` (old `VR-` requests: `created_at` in Asia/Jakarta); `seq` 1..999 (`vendor_request_tickets_seq_chk`); `ticket_number` UNIQUE; `is_active`, `deactivated_at`; `result` `success|failed` + `result_updated_at`. UNIQUE `(terminal_id, replenish_date, seq)`, UNIQUE `(request_number, terminal_id, denom_code)`, partial UNIQUE `(request_number, terminal_id) WHERE is_active`; CHECK inactive ⇒ `deactivated_at` set and `result` NULL. Never deleted.

## ACM vault assignment (migration 027, + comment 028)

Plan/spec: `.claude/sdlc/cit-acm-plan/` (Phase 2.2a). Migration `027_acm_vault_assignment.sql` applied to dev DB 2026-10-08; `028_dsr_flow_comment.sql` (column comment only) applied 2026-10-08.

*   `vendor_requests` — status CHECK + `vault_assignment`, `vault_review`, `ready`. `vault_flow` bool (true once ATM-SPV approves after 2.2a; laporan selesai ditolak → `ready`, else → `approved`), `vault_reviewed_by/_at`, `vault_rejection_reason` (1–500, set by ATM-SPV reject, shown to ACM on the plan).
*   `dsr_daily_rows.flow` + `saldo_akhir` (d0, one row per denom per **located** vault block; the total block is not stored). `dsr_daily_rows.location` is now filled per block by the ETL (was always NULL before S2).
*   `dsr_location_vault_maps` — (`vendor_id`, `dsr_location` label) → `vendor_vaults.id`; partial UNIQUE `(vendor_id, lower(dsr_location)) WHERE is_active`; soft-disable. Master data via maker-checker (entity `dsr_location_map`, label immutable, update = vault only). DSR → vendor join is `dsr_uploads.vendor = vendors.name` (name, not id).
*   **Saldo branch vault** (computed, not stored): Σ `saldo_akhir` of DSR `daily_status='completed'`, `report_date` = replenish date, over every active vault of the branch via the mapping; **unknown** (not 0) if the branch has no vault, or any active vault has no mapping / no row that day.
*   `acm_areas` (name unique among active, soft-disable; disabling releases its branches), `acm_area_branches` (link, UNIQUE `vendor_branch_id` = one area per branch, rows deleted on unassign — audited), `acm_area_members` (link, ACM-USER/ACM-SPV, user may be in many areas). ADMIN, immediate apply + audit in tx (GR#3 deviation, Sec 12).
*   `vendor_request_vault_plans` — one per (request, area), UNIQUE; status `draft → pending_acm_approval → acm_approved`, reject → `draft`, `cancelled` on request cancel; `submitted_*`, `approved_*`, `rejected_*`, `rejection_reason` 1–500.
*   `vendor_request_vault_assignments` — one per (request, terminal_id), UNIQUE; `replenish_branch_id`, `vault_branch_id`, `tier` 1–3, `is_urgent` + `urgent_reason` 10–500 (CHECK), tier 3 ⇒ urgent (CHECK); `saldo_snapshot`/`capacity_snapshot` jsonb `{"<denom>": "<whole IDR string>"}` (`null` = unknown; the DB comment shows `.00` but the service writes whole rupiah), `capacity_warning`. Replaced (deleted + re-inserted, audited) only while the plan is `draft`.
*   Roles `ACM-USER`, `ACM-SPV`; `menu_features` `cit`, `cit.vault-plan`, `settings.acm-areas` + `role_permissions`.

## Send to vendor (migration 029)

Plan/spec: `.claude/sdlc/cit-send-vendor/` (Phase 2.2b). Migration `029_vendor_request_send.sql` applied to dev DB 2026-10-09.

*   `vendor_requests` — status CHECK + `sent_to_vendor`, `vendor_accepted`, `vendor_rejected`. `vendor_sent` bool (true once sent; laporan selesai ditolak → `vendor_accepted`), `sent_at` (last (re)send), `vendor_accepted_at`. ATM-SPV vault approve goes **straight** to `sent_to_vendor`; a `ready` row in the DB is a legacy 2.2a request (no marker, no backfill).
*   `vendor_request_vendor_parties` — one per (request, `vendor_branch_id`, `role` replenish|vault), UNIQUE; derived from `vendor_request_vault_assignments` (each distinct `replenish_branch_id` / `vault_branch_id`; a branch that is both = two parties). `status` pending → accepted | rejected (reason 10–500, CHECK) | withdrawn (no longer in the request); CHECKs tie `decided_by/_at` to accepted/rejected. `content` jsonb = what the vendor saw: `{role, branch, atms[{terminal_id, lokasi, ticket_number, denoms[{denom, amount}], counterpart{…, address for a vault}}], totals[{denom, amount}], currency:"IDR"}` — built only from whitelisted columns (no saldo/kapasitas, tier, urgent, price). Never deleted.
*   `vendor_request_vendor_party_events` — append-only history: `sent`, `resent`, `accepted`, `rejected`, `withdrawn`, `cancelled` + actor, reason, content snapshot.
*   Vendor scope (not stored): `users.vendor_id` must equal the JWT `vendor_id` claim; `users.vendor_branch_id` NULL = every branch of the vendor, else that branch only — read from the primary per call.
*   Resend rules: new party → `sent`; changed content, or previously rejected/withdrawn → `resent` (pending); unchanged pending/accepted kept; missing → `withdrawn`. Re-approval after an edit (`approved_at` later than every party `sent_at`) resends every party. A vault rejection moves the request with `SetVendorRequestStatusOnly` so `approved_at` is untouched.
*   `vendor_request_vault_plans` reused: a vault rejection sets only the affected plans back to `draft` with `rejected_by` = the vendor user; `GetVaultPlan.rejected_by_vendor` drives the "Ditolak vendor" label.
*   **Since migration 030 (cit-vault-select v3)** parties are derived from the request ATMs (replenish = active kelolaan branch on the replenish date) and the live plan's `vendor_request_vault_allocations` (vault = each allocation's `vault_branch_id`). `content` is now `{role, branch, atms[{terminal_id, lokasi, ticket_number, denoms}], vaults[{ticket_number, vault{id, code, address}, branch, denoms}], replenish_branches[], totals, currency}` — no `counterpart`. Replenish party: own ATMs + **every** vault of the request, totals = its ATM orders. Vault party: own vaults + the request's replenish branches, totals = its allocations. Address = `COALESCE(vault.location_id, branch.location_id)` location.

## Vault allocation per request (migration 030)

Plan/spec: `.claude/sdlc/cit-vault-select/` (v3). Migration `030_vault_allocation.sql` applied to dev DB 2026-10-13. Replaces the 2.2a per-ATM assignment and area-per-branch for new plans.

*   `acm_area_regions` — (`acm_area_id`, `region_code`) PK, UNIQUE `region_code` (one area per region), CHECK `^[A-Z0-9]{2,10}$`, `created_by`. Pure link table, rows deleted on unassign (audited, `acm_area_regions_set`); disabling an area releases its regions. Plan owner = the area holding `vendor_requests.region_code`; no area (or request without region code) → request waits in `vault_assignment` + ADMIN warning.
*   `vendor_request_vault_plans` — now **one live plan per request**: partial UNIQUE `vendor_request_vault_plans_live_uq (vendor_request_id) WHERE status <> 'cancelled'` (the old UNIQUE (request, area) stays). `buffer_reason` 10–500 (CHECK), required at submit when any denom is allocated above the order.
*   `vendor_request_vault_allocations` — one row per (plan, vault), UNIQUE; `vendor_vault_id` + `vault_branch_id` with composite FK → `vendor_vaults (id, vendor_branch_id)` (needs new UNIQUE `vendor_vaults_id_branch_uq`); `tier` 1–3, tier 3 ⇒ urgent (CHECK), `urgent_reason` 10–500 iff urgent (CHECK); `saldo_snapshot` / `capacity_snapshot` jsonb `{"<denom>": "<whole IDR>"}` (null = unknown), `capacity_warning`. Replace-all while `draft` (hard delete, documented, like 027 assignments). Eligible vault: active, `category IN ('CASH','ATM_CASH')`, branch active with a region code.
*   `vendor_request_vault_allocation_amounts` — (`allocation_id`, `denom`) PK, ON DELETE CASCADE; `denom` in 100000…1000 (CHECK), `amount` bigint > 0 and `amount % denom = 0` (CHECK).
*   `vendor_request_vault_tickets` — tiket pemenuhan vault, one row per (request, vault) for life (UNIQUE); `ticket_number` `<vault_code>_<YYYYMMDD replenish_date>_<NNN>` UNIQUE, `seq` 1–999 UNIQUE per (vault, date), issued under an advisory lock. Synced at **submit**: kept vault keeps its number, dropped vault → `is_active=false` + `deactivated_at` (CHECK), returning vault → same number reactivated. Reject/cancel never touch tickets. Never deleted.
*   **Capacity** (computed, not stored): vault saldo on the date (DSR `saldo_akhir` via `dsr_location_vault_maps`, unknown ≠ 0) − Σ allocations to that vault of other live plans of `vault_flow` requests on the same date. Warning only, never a block.
*   `vendor_request_vault_assignments` (027) and `acm_area_branches` are no longer written or read for new plans; kept (table comment), drop in a later cleanup.

## Replenish realization (migration 031)

Plan/spec: `.claude/sdlc/replenishment-realisasi/` (Phase 2.2c). Migration `031_replenish_realization.sql` applied to dev DB 2026-10-14. Money = `bigint` whole IDR.

*   `holidays` — `holiday_date`, `name` (1–100, CHECK), `is_active`/`deleted_at` (CHECK `is_active = (deleted_at IS NULL)`), `created_by`. Partial UNIQUE `holidays_active_date_uq (holiday_date) WHERE is_active` — one active row per date; an old inactive row for the same date may exist (API addresses rows by `id`). Workday = Mon–Fri not in the active set. Empty after migration; ADMIN fills it (`/api/v1/admin/holidays`, immediate + audit in tx). Soft-disable only.
*   `vendor_request_realizations` — the vendor's realisation report, one per replenish ticket (UNIQUE `ticket_id`), + `vendor_request_id`, `realized_date` (NULL = failed), `submitted_by`/`submitted_at`. Resubmit after reject overwrites (history in `audit_logs`).
*   `vendor_request_realization_amounts` — (`realization_id`, `denom`) PK, ON DELETE CASCADE; `amount` ≥ 0 and a multiple of `denom` (CHECK).
*   `replenish_classifications` — written only by the Python job `backend_python/replenish_classify/`. One row per active replenish ticket (UNIQUE `ticket_id`) or, for `no_order`, per unmatched `itm_replenish` row (UNIQUE `itm_replenish_id`, FK `ON DELETE SET NULL` so an ITM re-upload never fails). `classification` `pending|on_schedule|early|late|not_done|no_order|void`; `workday_offset` −2..2; `is_final` ⇔ not `pending` ⇔ `finalized_at` set (CHECK) — final rows are never recomputed. Flags `has_amount_diff`, `vendor_mismatch`, `vendor_report_late` (+ `late_notified_at`). `report_date` = generated `COALESCE(scheduled_date, realized_date)` (screen/report filter). The `updated_at` trigger has no `WHEN` clause (a whole-row compare can't see the generated column).
*   `replenish_classification_amounts` — (`classification_id`, `denom`) PK: `order_amount`, `itm_amount`, `vendor_amount` (NULL = no vendor report), `diff` generated `itm − order`. Inputs + delta stored → reproducible (Sec 6). The ITM ETL only fills 50K/100K, so 10K/20K orders always show a diff (accepted, FR3.6).
*   No change to `itm_replenish`, `vendor_requests` statuses or `vendor_request_tickets`. All new tables are in `no_hard_delete_test.go`'s guard list.

## Vendor party scope (migration 032)

Spec: `.kiro/specs/vendor-party-scope/` (legacy Kiro, task 1). Migration `032_vendor_party_scope.sql` applied to dev DB 2026-10-20.

*   `roles` — new `VENDOR-ATM-USER` (sees only the replenish party of its branch) and `VENDOR-VAULT-USER` (only the vault party). Their `role_permissions` are copied from `VENDOR-USER` (dev: 3 each). `VENDOR-USER` stays vendor-wide over both parties. Code refers to roles by name, never by id.
*   `users.vendor_branch_id` — composite FK `users_vendor_branch_owner_fk (vendor_branch_id, vendor_id) → vendor_branches (id, vendor_id)` (target UNIQUE `vendor_branches_id_vendor_uq`), so an account's cabang always belongs to its own vendor. MATCH SIMPLE: checked only when both are set. The old single-column `users_vendor_branch_fk` stays. The migration pre-check also rejected a cabang with `vendor_id` NULL (dev: 0 rows on 2026-10-20).
*   `vendor_pics.roles text[] NOT NULL DEFAULT {replenish,vault,cashcount}` + CHECK `vendor_pics_roles_chk` (subset of those three). Empty = contact only. Existing rows got all three. Set as a whole set, sorted and unique in the service. A screen edit applies immediately + audit in tx (GR#3 deviation, recorded in CLAUDE.md Sec 12 by task 13). Create/CSV stays maker-checker.

## ATM assignment package source (migration 023)

Plan/spec: `.claude/sdlc/atm-package-source/`. Migration `023_atm_assignment_vendor_source.sql`, applied to dev DB 2026-10-07.

*   `atm_vendor_packages` has two mutually exclusive package sources, enforced by `avp_source_chk`: **branch** — `vendor_package_id` (FK → `vendor_packages_branch`) set, the new columns NULL (all existing rows); **vendor-wide** — `vendor_package_id` NULL and `vendor_id` (FK → `vendors`), `vendor_branch_id` (FK → `vendor_branches`, the managing cabang), `package` (text label, = `vendor_package_prices.package`, joins `package_frequencies.package_code`) all set.
*   `avp_vendor_mode_uq` — unique `(atm_id, vendor_id, vendor_branch_id, package, effective_start_date)` where `vendor_package_id IS NULL`; the old unique `(atm_id, vendor_package_id, effective_start_date)` still covers branch rows. `atm_vendor_packages_no_overlap` (atm + date range, active rows) guards overlap **across** both modes. `avp_vendor_branch_idx` on `vendor_branch_id`.
*   Vendor-wide validation (service, not DB): vendor active + `kind='FLM_VENDOR'`, branch of that vendor and active, ATM has `price_machine_group`/`price_class`, and a `vendor_package_prices` row for `(vendor, package, machine_group, price_class)` effective on the start date whose `atm_id` is NULL or this ATM. Repeated at apply.
*   Readers resolve label `COALESCE(vendor_packages_branch.package_code, avp.package)` and cabang `COALESCE(vendor_packages_branch.vendor_branch_id, avp.vendor_branch_id)`.

## Notifications (migration 025)

Plan/spec: `.claude/sdlc/notification/`. Migration `025_notifications.sql`, applied to dev DB 2026-10-07.

*   `notifications` — one row per recipient **user** (fan-out at send time; role/vendor recipients are expanded to active users then). `type` (stable code, e.g. `visit_quota.over_quota`), `title` (1–120), `body` (≤ 1000), `link` (path in the recipient's portal), `entity_type`/`entity_id` (optional source), `is_read`, `read_at`, `created_at`. Indexes: `(recipient_user_id, created_at DESC, id DESC)`, partial `(recipient_user_id) WHERE is_read = false` (unread polling), `(created_at)` (retention). Only the recipient reads/updates it.
*   `notification_emails` — outbox, one row per unique (lowercased) address; `notification_id` NULL for `vendor_pics` recipients (email only, `is_notification_recipient = true`). `status` `pending|sent|failed|skipped` (`skipped` = `SMTP_HOST` empty), `attempts`, `last_error` (≤ 500 chars, no credentials), `next_attempt_at`, `sent_at`. Partial index on `next_attempt_at WHERE status = 'pending'`; claimed `FOR UPDATE SKIP LOCKED`.
*   Retention: worker hard-deletes `notifications` older than 90 days and final-status `notification_emails` older than 90 days, once per 24 h (pending rows are never purged).

## Proposed — FSD v1.0 (2026-10-01) — NOT approved, NOT migrated

Source: FSD "DSR End To End Cash Management" v1.0 field tables (Master Vendor, Master Kelolaan ATM, Monitoring Limit CIS, Parameter Cash Count, Cash Count Planning). Status: proposal awaiting user approval → then migration `021+` via an AI-DLC chain. Reuse existing tables first; new tables only where nothing fits.

### A. New columns on existing tables

| Table | Column | Type | FSD field | Notes |
|---|---|---|---|---|
| `vendors` | `director_name` | text | Nama Direktur Utama | |
| `vendors` | `director_contact` | text | Informasi Contact Direktur Utama (number and email) | free text; split phone/email if needed |
| `vendors` | `pks_name` | text | PKS Efektif | contract title |
| `vendors` | `pks_effective_date` | date | Tanggal Efektif PKS | |
| `vendors` | `pks_expiry_date` | date | Tanggal Expired PKS | CHECK `>= pks_effective_date` |
| `vendors` | `pks_terminated_at` | timestamptz NULL | Status PKS = Terminated | set manually by OPS (via maker-checker vendor update) |
| `vendor_branches` | `cis_limit_amount` | numeric(20,2) NULL | Limit CIS | per area vault vendor; CHECK `>= 0` |
| `vendor_branches` | `cis_limit_currency` | char(3) NOT NULL DEFAULT 'IDR' | — | currency always explicit (Sec 6) |
| `vendor_vaults` | `escrow_account` | text NULL | ATM Escrow / Cash Escrow | SIBS account number, text like `atms.escrow_account`; ATM vault → ATM escrow, CASH vault → Cash escrow; "Tidak Ada ATM/CASH" = no vault of that category |

**Not stored (computed on read):** PKS status Active/Expired, Masa Berlaku (years), remaining months, flag Green/Amber/Red — FSD: recalculated every time the menu is opened. Terminated is the only stored state (`pks_terminated_at`).

**Already covered by existing columns:** Nama Perusahaan → `vendors.legal_name`/`name` · Alamat Kantor Pusat → `vendors.hq_address` · Area Vault Vendor → `vendor_branches.branch_name` · Region → `vendor_branches.region` · Alamat Vault → `vendor_vaults.location_id` · Nama PIC/Phone/Email → `vendor_pics` (with `vendor_branch_id`) · Service Type (ATM / CASH / ATM & CASH) → derived from the area's `vendor_vaults.category`.

### B. New tables

**1. `holidays`** (Core) — **migrated 2026-10-14 in `031`** (see [Replenish realization](#replenish-realization-migration-031); final shape differs: `created_by`, partial unique on active rows). Non-working dates; working day = Mon–Fri and not in this table. Used by cash count date generation, "3 working days before month end", and replenishment holiday-adjusted classification (URS).
`id bigint pk · holiday_date date NOT NULL UNIQUE · name text NOT NULL · is_active boolean NOT NULL DEFAULT true · deleted_at timestamptz · created_at · updated_at`. Soft-disable only.

**2. `cash_count_category_params`** (ATM) — H/M/L ranges + onsite frequency. Effective-dated like `vendor_package_prices` ("disable" closes `effective_end_date`); maker-checker + audit.
`id · category text CHECK (HIGH|MEDIUM|LOW) · min_amount numeric(20,2) NULL · max_amount numeric(20,2) NULL · currency_code char(3) NOT NULL DEFAULT 'IDR' · onsite_visits int NOT NULL CHECK (>= 0) · period_months int NOT NULL DEFAULT 6 · effective_start_date date NOT NULL · effective_end_date date NULL · created_at · updated_at`. No overlapping active period per category (EXCLUDE gist). Seed per FSD: HIGH > 15 M → 3×, MEDIUM 5–15 M → 2×, LOW < 5 M → 1×.

**3. `cash_count_pics`** (ATM) — PIC Coordinator per region, PIC Executor per area. Uploaded via the master-data upload flow (replace-all, maker-checker, audit).
`id · role text CHECK (COORDINATOR|EXECUTOR) · user_id bigint FK users NOT NULL · region_id bigint FK regions NULL · area text NULL · is_active · deleted_at · created_at · updated_at`. CHECK: COORDINATOR → `region_id` NOT NULL; EXECUTOR → `area` NOT NULL.

**4. `cash_count_schedules`** (name already approved 2026-09-25 — column proposal) — one row per area vault vendor per month; stores the category inputs so the plan is reproducible (Sec 6).
`id · vendor_branch_id FK NOT NULL · plan_start_month date NOT NULL (6-month plan) · visit_month date NOT NULL (1st of month) · avg_daily_balance_6m numeric(20,2) · avg_daily_balance_1m numeric(20,2) · category_6m text · category_effective text (after up-only rule) · visit_method text CHECK (ONSITE|VIRTUAL) · planned_date date NULL · coordinator_pic_id FK cash_count_pics · executor_pic_id FK cash_count_pics NULL · status text (DRAFT|SUBMITTED|CONFIRMED|DONE|CANCELLED) · submitted_by FK users · submitted_at · confirmed_at · created_at · updated_at`. UNIQUE `(vendor_branch_id, visit_month)`. "Exclude last year's date" queries this table.

**5. `cash_count_reschedule_requests`** (ATM) — executor reschedule request → coordinator decision.
`id · schedule_id FK cash_count_schedules NOT NULL · requested_by FK users · reason text · proposed_date date NULL · status text CHECK (PENDING|APPROVED|REJECTED) · decided_by FK users NULL · decided_at · created_at`. Never deleted; FSD "history 1 month" = UI filter, not retention.

### C. No new table needed
- **Daily escrow balance** (CIS min/max/average, cash count category): read from the planned `escrow_batch_rows` (Integration group, not yet created) — needs `account_number`, `balance_date`, `closing_balance` per row.
- **Monitoring Limit CIS**: computed on the replica per month (min/max/avg of ATM + Cash escrow vs `cis_limit_amount`) — no result table.
- **Daily time windows** (DSR 06:30–09:00, final order 12:30, …): config/constants.

### D. Open questions
1. FSD Kelolaan sample: is "Area Vault Vendor" = `vendor_branches` (one ATM vault + one CASH vault per area)? An `ATM_CASH`-category vault would need two escrow numbers.
2. Daily escrow balance source: the FSD says "link to SIBS"; Sec 3 plans a Corebanking batch file. Is it the same file?
3. CIS limit edit: FSD has no checker step; Golden Rule #3 requires maker-checker for master data. Keep maker-checker?
4. Executor "area": what values (FSD sample shows Jabodetabek / Outregion) — free text or a new lookup? What is "PIC CC STCC / Non STCC" in the planning sample?
5. H/M/L boundaries: inclusive or exclusive (e.g. exactly 15 M)?
6. Out of scope here: forecast complaint/project uploads (Phase 2.1 `forecast_*` tables) and the Rec 7 recap (output file, no table).

### E. Answers 2026-10-05 (section A approved — PKS + Limit CIS)
- **A1** Columns in table A approved as proposed → migration `022` (021 is taken by atm-visit-quota).
- **A2** Area Vault Vendor = `vendor_branches`. One `escrow_account` per vault; an area with both ATM and Cash escrow has one ATM vault + one CASH vault. `ATM_CASH` vaults are not used for areas with two escrow numbers.
- **A3** Limit CIS edits go through master-data maker-checker (Submit → approval → Applier → 202), no GR#3 deviation.
- **A4** PKS Amber threshold = **9 months**, kept as a constant/config (FSD also says 8 — BA may still change it).
- **A5** Coverage: average daily balance **≤** limit → Covered; > limit → Not Covered; no limit set → "Limit belum diisi".
- **A6** Monitoring Limit CIS is **deferred** until daily escrow balance ingest (SIBS/Corebanking → `escrow_batch_rows`) exists. The first feature = PKS + Limit CIS input only.
- Still open (tables B): everything for cash count (sections B/D questions 2, 4, 5). `holidays` was resolved by 2.2c (migration 031).

## DSR receipts (migration 033)

Source: `.claude/sdlc/dsr-late-report/spec.md` (S1, K2). File: `backend/migrations/033_dsr_receipts.sql` (applied to dev 2026-10-27 via `migrate up`). Additive only; forward-only.

- **Table `dsr_receipts`** — append-only, one row per confirmed DSR commit (`dsr_etl.py --mode commit`, `daily_status` completed), written in the same tx as `dsr_uploads`.
  - `id` bigint identity PK
  - `vendor_label` text NOT NULL — `dsr_uploads.vendor` as uploaded (raw label)
  - `vendor_id` bigint NULL, FK `vendors(id)` — resolved at commit by the unique vendor code from the upload filename (review R1, 2026-10-27; the DB column comment in 033 still says `vendors.name`, migration not edited); `vendor_label` keeps the name; NULL only on pre-fix rows (warned, counted for no branch)
  - `report_date` date NOT NULL — DSR report date the upload covers
  - `checksum` text NOT NULL — SHA-256 hex of the file, CHECK `^[0-9a-f]{64}$`
  - `locations` text[] NOT NULL DEFAULT `'{}'` — distinct non-empty `btrim(dsr_daily_rows.location)` labels of that upload
  - `uploaded_by_user_id` bigint NULL, FK `users(id)` — vendor user who confirmed, if known
  - `received_at` timestamptz NOT NULL DEFAULT now() — commit/confirmation time (K2), not staging time
  - UNIQUE `(vendor_label, report_date, checksum)` — same file committed again adds no row
  - Index `dsr_receipts_date_vendor_idx (report_date, vendor_id)`
- **No FK to `dsr_uploads`**: that row is replaced on revision, so a receipt must not depend on it.
- **Reads**: "first received" = `MIN(received_at)` per (vendor, report_date). Revisions add rows; they never change the earlier one.
- **Append-only rule**: never UPDATE or DELETE. Enforced by convention and code review (no trigger in 033).
- Uses: DSR late report (`/api/v1/dsr-late-reports`, replica reads). Mapping of `locations` to vaults goes through `dsr_location_vault_maps` (027) — no change to that table.

## Export jobs (migration 034)

Spec: `.kiro/specs/vendor-data-export/` (design "Data Models"). Migration `034_export_jobs.sql`, applied to dev DB 2026-10-28 via `migrate up`. Additive; no change to master data or `audit_logs`.

*   **`export_jobs`** — identity PK. `requested_by` FK → `users(id)` NOT NULL. `kind` CHECK IN (`vendor`, `cabang`, `harga_paket`, `master_data_global`); `entity` text. `vendor_id` FK → `vendors`, `vendor_code`; `vendor_branch_id` FK → `vendor_branches`, `branch_code`. `status_filter` CHECK (`active`, `disabled`, `all`); `price_level` CHECK (`vendor`, `branch`, `atm`, `all`), set only for `harga_paket`. `params_hash`, `file_name` NOT NULL. `status` CHECK (`queued`, `running`, `succeeded`, `failed`, `downloading`, `downloaded`, `expired`), default `queued`. `row_count` bigint default 0; `byte_size`, `chunk_count`, `error_message` nullable. Timestamps: `created_at`, `started_at`, `finished_at`, `expires_at`, `download_started_at`, `downloaded_at`, `expired_at`, `updated_at` (trigger `set_updated_at`, fires only on a real change).
*   **Constraints**: `export_jobs_scope_chk` (scope per kind: `master_data_global` = no vendor/branch; `vendor` = vendor only; `cabang` = vendor + branch; `harga_paket` = vendor); `export_jobs_price_level_kind_chk` (`price_level` NOT NULL iff `harga_paket`); `export_jobs_ready_chk` (`succeeded`, `downloading`, `downloaded` require `finished_at`, `expires_at`, `byte_size`, `chunk_count`).
*   **Indexes**: `export_jobs_active_dedupe_uq` UNIQUE `(requested_by, params_hash)` WHERE status IN (`queued`, `running`); `export_jobs_queue_idx` `(id)` WHERE `queued`; `export_jobs_requester_idx` `(requested_by, created_at DESC, id DESC)`; `export_jobs_janitor_idx` `(status)` WHERE `running`, `succeeded`, `downloading`.
*   **Lifecycle**: rows are never deleted (history). Every status change is one conditional `UPDATE`; zero rows = transition rejected (design, Req 9.2).
*   **`export_job_chunks`** — PK `(export_job_id, seq)`; `export_job_id` FK → `export_jobs(id)` ON DELETE CASCADE; `seq` integer CHECK `>= 0`; `data` bytea NOT NULL. Holds the CSV bytes of one job in chunks of at most 1 MiB (100 MiB per job, enforced by the writer, not by the DB). Deleted on successful download, on failure, or 10 minutes after ready (janitor). Empty for terminal jobs.
*   **Known risk** (design, Error Handling): chunk bytes replicate to the replica and enter WAL (max 100 MiB per job, lifetime about 10 minutes). Accepted at current volume.

### Migration 035 (`035_export_jobs_reports.sql`)

Spec: `.kiro/specs/report-export-migration/` (design "Data Models"). Applied to dev DB 2026-10-29 via `migrate up`. Additive: adds `report_params jsonb` (object of report filters: `month` or `date_from`/`date_to`, `vendor_id`, `vendor_branch_id`, `region_code`, `status`, `classification`; NULL for master-data kinds), widens `export_jobs_kind_chk` with `dsr_late_daily`, `dsr_late_summary`, `replenishment_amount_mismatch`, `replenishment_trips`, and widens `export_jobs_scope_chk` so report kinds carry no vendor/branch. `status_filter` becomes nullable and is NULL for report kinds. New `export_jobs_params_kind_chk`: master-data kinds need `status_filter` and no `report_params`; report kinds need `report_params` (object) and no `status_filter`. `export_jobs_price_level_kind_chk` unchanged. Audit for report downloads is `report_exported` with the legacy `entity_type` (`dsr_late_report` or `replenishment_report`) and `entity_id` 0.
