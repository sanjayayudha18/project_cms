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

**1. `holidays`** (Core) — non-working dates; working day = Mon–Fri and not in this table. Used by cash count date generation, "3 working days before month end", and replenishment holiday-adjusted classification (URS).
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
- Still open (tables B): everything for cash count, holidays (sections B/D questions 2, 4, 5).
