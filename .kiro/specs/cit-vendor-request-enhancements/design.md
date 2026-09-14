# Design: CIT Vendor Request Enhancements ("CIT 2")

## Overview

Five enhancements to the existing CIT vendor-request / replenishment slice (`request-replenish-to-vendor`, now in `.kiro/specs/done/`), building on the in-flight `update-cit-forecast-browser` sibling (the four context columns + cross-page select-all). This spec adds:

1. **Forecast Browser filters** by Brand (optional, empty-sentinel), FLM Vendor (required, no ALL), FLM Vendor Region (required, no ALL) on top of the existing date + ATM-ID filters.
2. **Create-page context columns** (Brand / FLM Vendor / FLM Vendor Region) plus a chooseable `replenish_date` (default H+1 Asia/Jakarta).
3. **Manual requests** without a DMAA source row, gated by a `request_category` (Planned / Emergency / Additional) that governs the allowed replenish date.
4. **New request-number format** `REP` + 3-char vendor prefix + `YYYYMMDD` + 3-digit sequence (`REPTAG20260720001`), replacing `VR-YYYYMMDD-NNNN`.
5. **Soft-cancel** via a new `is_canceled` boolean (row never deleted), cancelable by Maker (creator) or SPV (checker), audited.

The whole change stays within the existing vertical slice and keeps the ATM backend's flat JSON contract (additive fields only). Downstream `approved → processing → completed` execution remains out of scope.

## Architecture

The layers touched, top to bottom, consistent with the sibling spec's slice diagram:

```
ForecastBrowser.tsx        ── + Brand / FLM Vendor / FLM Vendor Region filters (FLM+Region required, block fetch)
VendorRequestCreate.tsx    ── + context columns, replenish_date input, category selector, manual-entry rows
VendorRequestList/Detail   ── + replenish_date field, category field, canceled badge (icon+text)
  └─ hooks.ts / api.ts / types.ts ── + filter params, replenish_date, request_category, is_canceled, cancel already wired
       └─ GET /forecast (+brand/flm_vendor/flm_vendor_region params)
          POST / (+ replenish_date, request_category, is_manual)
          POST /{id}/cancel (now sets is_canceled)
          GET / , GET /{id} (+ replenish_date, request_category, is_canceled additive fields)
            └─ service.BrowseForecast / Create / Cancel (+ number generator, category rules, manual accept)
                 └─ backend/queries/vendor_request.sql (+ filter params, seq table, cancel query, category-aware create)
                      └─ backend/migrations/034..037 (additive DDL only — is_canceled, request_category, replenish_date, vendor_id, request_prefix, sequence table)
```

### Dependency & prerequisite notes

- **Depends on `update-cit-forecast-browser`** for `ForecastRow`'s four context fields (`lokasi_atm`, `brand`, `flm_vendor`, `flm_vendor_region`) and the `ListForecastForDate` join chain. Requirement 1 (filters) and Requirement 2 (create-page columns) assume those fields are present. If that sibling has not landed, its column + join work is a prerequisite, not duplicated here.
- **Active-vendor resolution** used for the number prefix (Q1/Q2) reuses the exact LATERAL join chain the sibling built: `atms → atm_vendor_packages (active) → vendor_packages → vendor_branches → vendors`. See `update-cit-forecast-browser/design.md` "Data model — the join chain".
- **ATM backend flat JSON** (project-context Sec 2 / Sec 4): every `vendor_requests` endpoint extends its response object additively — no renames, no removals — for wire compatibility with both frontends.

---

## STOP-and-confirm gates (project-context Golden Rule 7)

Every schema change below is **additive, forward-only DDL, no data migration of existing rows**, under `backend/migrations/`. The current highest migration is `033_fix_vendor_branch_trailing_space.sql`, so the new files take numbers **034–037**. These touch schema, money-adjacent columns, and the vendor reference that drives the request number — **each must be confirmed by the user before migrating**. Nothing here should be applied to the live external Postgres without sign-off.

### Migration `034_vendor_requests_cit2_columns.sql` (Req 2, 3, 5)

Additive columns on `vendor_requests`:

```sql
ALTER TABLE public.vendor_requests
    ADD COLUMN IF NOT EXISTS is_canceled      boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS request_category text,
    ADD COLUMN IF NOT EXISTS replenish_date   date,
    ADD COLUMN IF NOT EXISTS is_manual        boolean NOT NULL DEFAULT false;

ALTER TABLE public.vendor_requests
    ADD CONSTRAINT vendor_requests_category_chk
    CHECK (request_category IS NULL
        OR request_category IN ('planned', 'emergency', 'additional'));

CREATE INDEX IF NOT EXISTS vendor_requests_is_canceled_idx
    ON public.vendor_requests (is_canceled);
CREATE INDEX IF NOT EXISTS vendor_requests_replenish_date_idx
    ON public.vendor_requests (replenish_date);
```

- `is_canceled boolean NOT NULL DEFAULT false` — Req 5.1, soft-cancel flag. Safe on existing rows (all default to `false`).
- `request_category text` **nullable** with a CHECK that admits NULL — Req 3.9. Nullable (not defaulted) is deliberate: existing rows predate categories and must **not** be silently reclassified as `planned`. New DMAA-backed and manual requests always set it (see Q5 / service rules); only legacy rows keep NULL.
- `replenish_date date` **nullable** — Req 2.5 / 2.9 / 3. Nullable so existing rows backfill as NULL; new rows always set it. The detail/list responses render NULL as `null` (see handler).
- `is_manual boolean NOT NULL DEFAULT false` — resolves Q4-adjacent read logic: distinguishes a manual request (skip DMAA re-derivation) from a DMAA-backed one, without inferring it from `request_category` (Emergency/Additional can technically match DMAA too). Existing rows are all DMAA-backed → `false`.

### Migration `035_vendors_request_prefix.sql` (Req 4, Q1)

Additive column on `vendors` holding the deterministic 3-char prefix (Q1 recommendation — see Data model):

```sql
ALTER TABLE public.vendors
    ADD COLUMN IF NOT EXISTS request_prefix char(3);

ALTER TABLE public.vendors
    ADD CONSTRAINT vendors_request_prefix_chk
    CHECK (request_prefix IS NULL OR request_prefix ~ '^[A-Z]{3}$');
```

Nullable char(3) with a CHECK enforcing exactly 3 uppercase A–Z. Nullable so the migration is additive; a companion **seed** migration (below) populates the six known vendors. The service's deterministic fallback (Q1) covers any vendor row with a NULL/absent prefix so a missing seed never produces a malformed number.

### Migration `036_seed_vendor_request_prefix.sql` (Req 4, Q1 — data seed, idempotent)

Populates the prefix for the six seeded vendors (`005_seed_vendors.sql`). This is a **seed UPDATE of the six known master-data rows, not a data migration of transactional data** — still flagged for confirmation because it fixes the vendor→prefix contract that request numbers depend on:

```sql
UPDATE public.vendors SET request_prefix = 'TAG' WHERE code = 'TAG';
UPDATE public.vendors SET request_prefix = 'ADV' WHERE code = 'ADVANTAGE';
UPDATE public.vendors SET request_prefix = 'BJK' WHERE code = 'BIJAK';
UPDATE public.vendors SET request_prefix = 'ABC' WHERE code = 'ABACUS';
UPDATE public.vendors SET request_prefix = 'ROH' WHERE code = 'ROH';
UPDATE public.vendors SET request_prefix = 'SSI' WHERE code = 'SSI';
```

### Migration `037_vendor_request_number_seq.sql` (Req 4, Q3)

A DB-backed sequence table for atomic per-scope increment (see Data model / Q3):

```sql
CREATE TABLE IF NOT EXISTS public.vendor_request_number_seq
(
    vendor_id   bigint NOT NULL,
    seq_date    date   NOT NULL,
    last_seq    int    NOT NULL DEFAULT 0,
    CONSTRAINT vendor_request_number_seq_pkey PRIMARY KEY (vendor_id, seq_date),
    CONSTRAINT vendor_request_number_seq_last_chk CHECK (last_seq >= 0 AND last_seq <= 999),
    CONSTRAINT vendor_request_number_seq_vendor_fk FOREIGN KEY (vendor_id)
        REFERENCES public.vendors (id) MATCH SIMPLE
        ON UPDATE NO ACTION ON DELETE RESTRICT
);
```

Also add the resolved vendor reference on the request header (Q2), so the number's vendor is persisted and reproducible, and cancel/list can display it:

```sql
ALTER TABLE public.vendor_requests
    ADD COLUMN IF NOT EXISTS vendor_id bigint;

ALTER TABLE public.vendor_requests
    ADD CONSTRAINT vendor_requests_vendor_fk FOREIGN KEY (vendor_id)
        REFERENCES public.vendors (id) MATCH SIMPLE
        ON UPDATE NO ACTION ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS vendor_requests_vendor_id_idx
    ON public.vendor_requests (vendor_id);
```

`vendor_id` is **nullable** (existing `VR-…` rows have no resolved vendor and are not rewritten — Req 4.9). New rows always set it.

> The four migrations may be split or combined at implementation time, but the DDL above is the confirmed set. **All require user confirmation before applying.**

---

## Open questions — resolved

### Q1 — Vendor prefix mapping (Req 4.2, 4.3) — RESOLVED

**Recommendation: a `vendors.request_prefix char(3)` column, seeded per vendor, with a deterministic service-side fallback.**

The mapping:

| `vendors.code` | `request_prefix` | Source |
| --- | --- | --- |
| `TAG` | `TAG` | requested directly (Req 4.2) |
| `ADVANTAGE` | `ADV` | requested abbreviation (Req 4.2) |
| `BIJAK` | `BJK` | requested abbreviation (Req 4.2) |
| `ABACUS` | `ABC` | derived (no requested prefix) |
| `ROH` | `ROH` | code already 3 chars |
| `SSI` | `SSI` | code already 3 chars |

**Fallback for an unmapped/NULL prefix** (Req 4.3, deterministic, never empty/truncated/non-3-char): uppercase the vendor's `code`, strip non-`[A-Z0-9]`, then:
- if ≥ 3 chars → take the first 3;
- if < 3 chars → right-pad with `X` to 3 (e.g. `AB` → `ABX`, `A` → `AXX`).

This guarantees a 3-uppercase-char prefix for any conceivable vendor and is fully deterministic given the code.

**Why a column, not a hardcoded map or code table:** A hardcoded Go map would drift from `vendors` (a new vendor added via master-data admin would have no prefix and silently hit the fallback with no way for ops to override). A separate code table is over-engineering for a one-column attribute of an existing entity. A nullable `request_prefix` on `vendors` (a) lets APPACCESS/ADMIN-PARAM set the intended prefix per vendor as master data, (b) keeps the fallback in code for safety, and (c) is a purely additive column. The seed (migration 036) sets the six known values; the fallback covers everything else.

> **STOP-and-confirm:** the specific `ABC` for ABACUS is a design choice (ABACUS has no requested prefix) — confirm this exact value with the user before seeding, since a request number is an auditor-facing identifier.

### Q2 — Which vendor drives the number (Req 4.2) — RESOLVED

**Recommendation: a vendor request is constrained to exactly one CIT vendor, resolved and persisted on `vendor_requests.vendor_id`.**

Requirement 1 now **requires** the operator to pick exactly one FLM Vendor on the Forecast Browser before any rows display (Req 1.2, 1.4). Every selected forecast row therefore already shares one vendor. So:

- **DMAA-backed create:** the vendor is the FLM Vendor the operator filtered by. The create payload carries `vendor_id` (resolved from the required `flm_vendor` filter). The service **validates** that every item's active vendor package resolves to the same `vendor_id` (defence-in-depth against a stale selection), rejecting a mixed-vendor payload with a validation error.
- **Manual create (Req 3):** the operator explicitly selects the vendor on the create page (a required single-select, same vendor list). The chosen `vendor_id` is sent in the payload.

Persisting `vendor_id` (rather than re-deriving per read) makes the number reproducible and the request self-describing even if an ATM's active vendor package changes later. This is the Q4 persistence principle applied to the one field the number depends on.

> **STOP-and-confirm:** constraining a request to one vendor is a behavioral narrowing of the old "any items" create. It is consistent with Req 1's required single FLM Vendor, but confirm no workflow needs a multi-vendor request before implementing the same-vendor validation.

### Q3 — Sequence scope (Req 4.5, 4.7) — RESOLVED

**Recommendation: per-vendor-per-day**, matching the `REP` + prefix + `YYYYMMDD` + seq shape (each vendor+date gets its own `001..999`).

**Atomic increment:** a dedicated `vendor_request_number_seq(vendor_id, seq_date, last_seq)` table (migration 037) incremented with an atomic upsert inside the create transaction:

```sql
-- name: NextRequestNumberSeq :one
INSERT INTO vendor_request_number_seq (vendor_id, seq_date, last_seq)
VALUES (sqlc.arg('vendor_id')::bigint, sqlc.arg('seq_date')::date, 1)
ON CONFLICT (vendor_id, seq_date)
DO UPDATE SET last_seq = vendor_request_number_seq.last_seq + 1
RETURNING last_seq;
```

The `ON CONFLICT … DO UPDATE … RETURNING` is a single atomic statement: concurrent creates in the same (vendor, date) scope serialize on the row lock and each gets a distinct `last_seq`. This is stronger than the current `MAX(SUBSTRING(...))` approach (which races) and removes the need to parse the number text.

**Exhaustion (Req 4.7):** the CHECK `last_seq <= 999` fails the upsert once the scope hits 1000; the service maps that to `ErrNumberExhausted` (already exists) → HTTP 409 with an explicit "sequence exhausted for this vendor/date" message, and the transaction rolls back so no malformed/colliding number is persisted. The service also guards `if last_seq > 999` explicitly before formatting, so exhaustion is caught even if the CHECK were ever relaxed.

**Concurrency retry (Req 4.6):** the `vendor_requests_number_uq` unique constraint remains the backstop. On the rare unique-violation (e.g. a manual row inserted with a colliding legacy number), the service retries number generation up to **5 attempts** (Req 4.6 raises the current 3 to 5), re-running the atomic increment each time.

### Q4 — Context-field persistence for manual requests (Req 3, sibling reconciliation) — RESOLVED

**Recommendation: persist context fields on `vendor_request_items` for MANUAL requests only; keep them display-only (derive-at-read-time) for DMAA-backed rows.**

The sibling spec deliberately does **not** persist `lokasi_atm/brand/flm_vendor/flm_vendor_region` on items for DMAA-backed rows — they are re-derivable from the DMAA join chain at read time, so persisting them would risk staleness. For a **manual** request there is no DMAA row to re-derive from, so those fields would be irretrievably lost.

Resolution:
- **Header:** `vendor_requests.vendor_id` (Q2) captures the request's single vendor — the one context field the number and RBAC depend on — for both manual and DMAA-backed requests.
- **Items (manual only):** add nullable additive columns to `vendor_request_items` to persist the operator-entered context for manual rows. To keep migration 034 tight, these live in the same file:

```sql
ALTER TABLE public.vendor_request_items
    ADD COLUMN IF NOT EXISTS brand    text,
    ADD COLUMN IF NOT EXISTS lokasi_atm text;
```

  For DMAA-backed rows these stay NULL and the read path derives display values from the DMAA/master-data joins (unchanged from the sibling). For manual rows the service persists what the operator entered. `flm_vendor`/`flm_vendor_region` are **not** persisted per-item — they are a property of the request's single vendor (`vendor_id`), resolved once at read time from `vendors`/`vendor_branches`, avoiding per-row duplication.

> This keeps the sibling's "display-only for DMAA rows" contract intact while making manual requests self-describing. Confirm the per-item `brand`/`lokasi_atm` columns before migrating.

### Q5 — `forecast_date` vs `replenish_date` semantics (Req 2, 3, 4.4) — RESOLVED

**Definitions:**
- **`forecast_date`** keeps its existing meaning: *the DMAA `periode_pred` the request was drawn from*. For **manual** requests (no DMAA row), `forecast_date` is set to the **request creation date** (today, Asia/Jakarta) — a stable, non-null anchor, since the column is `NOT NULL` and manual requests have no DMAA periode. This is recorded as an intentional convention (not a real forecast periode) and is why `is_manual` exists to distinguish the two.
- **`replenish_date`** is the new vendor-facing delivery date the operator chooses (Req 2.3–2.5): default H+1; Planned = H+1; Emergency = H+0; Additional = H+0/H+1/H+2. Stored on `vendor_requests.replenish_date`.

**Which date drives the request-number date segment (Req 4.4): `replenish_date`.** The number is an operational identifier for cash delivered on a day; the vendor-facing date is `replenish_date`. Using `replenish_date` also makes the per-vendor-per-day sequence scope (Q3) align with the day cash is actually delivered, which is the day operators reason about. `forecast_date` remains a provenance field, not the number's date.

> Consequence: two requests to the same vendor for the same delivery day share a sequence scope regardless of which DMAA periode they were drawn from — the intended behavior.

---

## Data Models

### `vendor_requests` (after migrations 034, 037)

Existing columns unchanged. New:

| Column | Type | Null | Meaning |
| --- | --- | --- | --- |
| `is_canceled` | `boolean` | not null, default `false` | Req 5 soft-cancel flag |
| `request_category` | `text` | nullable, CHECK in (`planned`,`emergency`,`additional`) | Req 3 category; NULL only for legacy rows |
| `replenish_date` | `date` | nullable | Req 2 vendor-facing delivery date; NULL only for legacy rows |
| `is_manual` | `boolean` | not null, default `false` | manual vs DMAA-backed; drives read-path derivation |
| `vendor_id` | `bigint` | nullable, FK `vendors(id)` | Q2 single resolved vendor; NULL only for legacy rows |

`status` and `is_canceled` are **orthogonal**: `status` stays the state-machine value (`cancelled` remains a terminal status the existing transition sets), while `is_canceled` is the soft-cancel marker. Cancel sets **both** (`status='cancelled'` via the existing transition path AND `is_canceled=true`) so old readers keying on `status` and new readers keying on `is_canceled` agree. See Error Handling for the ordering.

### `vendor_request_items` (after migration 034)

| Column | Type | Null | Meaning |
| --- | --- | --- | --- |
| `brand` | `text` | nullable | manual-request brand (Q4); NULL for DMAA rows (derived at read) |
| `lokasi_atm` | `text` | nullable | manual-request location (Q4); NULL for DMAA rows (derived at read) |

Money columns (`amount_replenish`, `amount_refund`, `denom`) unchanged — `bigint`/`int`, integer minor units, existing CHECKs (`denom > 0`, `amount_* >= 0`).

### `vendors` (after migration 035)

| Column | Type | Null | Meaning |
| --- | --- | --- | --- |
| `request_prefix` | `char(3)` | nullable, CHECK `^[A-Z]{3}$` | Q1 vendor prefix; seeded for 6, fallback in service |

### `vendor_request_number_seq` (migration 037)

| Column | Type | Meaning |
| --- | --- | --- |
| `vendor_id` | `bigint` PK part, FK `vendors(id)` | sequence scope: vendor |
| `seq_date` | `date` PK part | sequence scope: `replenish_date` (Q5) |
| `last_seq` | `int` CHECK `0..999` | last-issued sequence for the scope |

PK `(vendor_id, seq_date)` gives one row per scope; the upsert increments it atomically. No `updated_at` needed — the row is only ever incremented, tracked by `last_seq`.

---

## Components and Interfaces

### Backend changes (per requirement)

#### Requirement 1 — Forecast Browser filters

**SQL — `backend/queries/vendor_request.sql`.** Extend `ListForecastForDate` and `CountForecastForDate` with three new filter args, applied on the already-joined context columns (the sibling's join chain provides `a.brand`, `v.name`, `vb.region`):

```sql
  AND (sqlc.arg('brand')::text = ''            OR LOWER(a.brand)   = LOWER(sqlc.arg('brand')::text))
  AND (sqlc.arg('flm_vendor')::text = ''       OR LOWER(v.name)    = LOWER(sqlc.arg('flm_vendor')::text))
  AND (sqlc.arg('flm_vendor_region')::text= '' OR LOWER(vb.region) = LOWER(sqlc.arg('flm_vendor_region')::text))
```

- Case-insensitive exact match via `LOWER(...) = LOWER(...)` (Req 1.6–1.8), **not** ILIKE — these are exact single-value matches, unlike the substring `terminal_id` filter.
- Brand uses the empty-string sentinel = "no filter" (Req 1.9), matching the existing `terminal_id` convention.
- FLM Vendor and FLM Vendor Region also accept the empty sentinel at the SQL layer (keeps the query uniform), but the **service and frontend require a concrete value** (Req 1.2–1.5) — the SQL never receives empty for those in practice because the service rejects a browse missing either.
- `CountForecastForDate` **must** now include the same joins as `ListForecastForDate` (it previously counted bare `dmaa_atm_forecast`), because filtering on `a.brand` / `v.name` / `vb.region` requires those tables to be joined. The LATERAL active-package resolution still yields ≤1 row per forecast row, so the count stays correct. This is a change from the sibling's note that Count needs no joins — the filters force it.
- Logical AND with existing date + terminal filters is automatic (all in one `WHERE`) — Req 1.10.

**Service — `BrowseForecastParams` + `BrowseForecast`.** Add `Brand`, `FLMVendor`, `FLMVendorRegion string` to `BrowseForecastParams`. In `BrowseForecast`:
- Require `FLMVendor != ""` and `FLMVendorRegion != ""` → else `ValidationError{Field:"flm_vendor"/"flm_vendor_region", Message:"wajib dipilih"}` (Req 1.4). This is the server-side backstop for the frontend's block-fetch behavior.
- Length-bound each of the three to ≤ 255 chars → else `ValidationError` (Req 1.14).
- Pass all three to `ListForecastForDate`/`CountForecastForDate`.

**Handler — `BrowseForecast`.** Read `brand`, `flm_vendor`, `flm_vendor_region` from query params additively (Req 1.16); existing `forecast_date`, `atm_id`, `page`, `page_size` unchanged. Empty Brand param → identical pre-enhancement result for the selected vendor/region (Req 1.16).

#### Requirement 2 — Create-page context columns + replenish_date

**SQL.** `CreateVendorRequest` gains `replenish_date`, `request_category`, `is_manual`, `vendor_id` columns:

```sql
INSERT INTO vendor_requests
    (request_number, forecast_date, replenish_date, request_category, is_manual, vendor_id, status, notes, created_by)
VALUES ($1, $2, $3, $4, $5, $6, 'draft', $7, $8)
RETURNING *;
```

`InsertVendorRequestItem` gains nullable `brand`, `lokasi_atm` (Q4, manual only).

**Service.** `CreateVendorRequestInput` gains `ReplenishDate time.Time`, `RequestCategory string`, `IsManual bool`, `VendorID int64`. Validation:
- `replenish_date` required, valid calendar date → else `ValidationError{Field:"replenish_date"}` (Req 2.6).
- `replenish_date` not earlier than today Asia/Jakarta → else `ValidationError{Field:"replenish_date"}` (Req 2.7). Compare in `Asia/Jakarta` (`time.LoadLocation`), truncated to date.
- Persist `replenish_date` exactly as the selected calendar date (Req 2.5).

**Handler + response.** `createVendorRequestBody` gains `replenish_date`, `request_category`, `is_manual`, `vendor_id`. `vendorRequestDetailResponse` and `vendorRequestSummaryResponse` gain `replenish_date *string` (nullable → `null` when unset, formatted `YYYY-MM-DD`), `request_category *string`, `is_canceled bool`, `is_manual bool` — all additive (Req 2.10, 6.1). `mapDetail`/list mapping copy them through.

#### Requirement 3 — Manual requests + category

**Service — category & manual rules** (all enforced at service layer, Req 3.13):
- `request_category` must be one of `planned|emergency|additional` → else `ValidationError{Field:"request_category"}` (Req 3.11).
- Zero items → `ErrEmptyItems` (Req 3.12).
- **Date-consistency (Req 3.10)** against `replenish_date` in Asia/Jakarta:
  - `planned` → `replenish_date` must equal H+1 (else conflict `ValidationError`).
  - `emergency` → must equal H+0.
  - `additional` → must be H+0, H+1, or H+2; any other rejected (Req 3.5).
- **Item resolution branches on `is_manual` / category:**
  - `planned` (or any DMAA-backed create) → call existing `resolveItems` (validates each item against `dmaa_atm_forecast`, rejecting unmatched with `InvalidItemsError`) — Req 3.8.
  - `emergency`/`additional` manual → **accept items without DMAA match** (Req 3.7): a new `acceptManualItems` path that validates shape (terminal_id 1–64 chars, denom > 0, amount_replenish > 0 — Req 3.6, 3.14) and sets `amount_refund = 0` (no DMAA row to copy from), persisting operator-entered `brand`/`lokasi_atm`.
- Persist `request_category` and `is_manual` on the header (Req 3.9).
- **Audit (Req 3.13):** exactly one `audit_logs` "create" entry, `After` metadata includes `request_category` and `is_manual`.

Money stays integer minor units; reject non-positive denom / negative amount with a field-named `ValidationError` (Req 3.14).

#### Requirement 4 — New request-number generator

Replace `createWithRetryingNumber`'s body:
1. Resolve `vendor_id` (from payload, Q2) and its `request_prefix` (query `vendors.request_prefix`); apply the Q1 fallback if NULL.
2. Compute the date segment from `replenish_date` (Q5) → `YYYYMMDD`.
3. Atomic `NextRequestNumberSeq(vendor_id, replenish_date)` → `last_seq`. If `last_seq > 999` → `ErrNumberExhausted` (Req 4.7).
4. Format `fmt.Sprintf("REP%s%s%03d", prefix, dateSeg, last_seq)` — fixed 17 chars (Req 4.1).
5. Insert; on `vendor_requests_number_uq` violation, retry up to **5** attempts (Req 4.6), re-incrementing the sequence each time. After 5 → `ErrNumberGeneration`.

Existing `VR-…` rows are never rewritten (Req 4.9); `request_number` stays on `vendor_requests.request_number`, unique constraint preserved (Req 4.8); list request-number search (ILIKE) still matches the stored value (Req 4.10).

The old `MaxRequestNumberSeqForDate` query is retained only if any test depends on it; the new generator does not use it (it read a 4-digit `\d{4}$` suffix that no longer matches the format). Recommend removing it once the generator is switched, and dropping the `MaxRequestNumberSeqForDate` method from `VendorRequestRepository`.

#### Requirement 5 — Soft-cancel

**SQL.** Add a cancel-specific update (keeps the shared `UpdateVendorRequestStatus` untouched):

```sql
-- name: SoftCancelVendorRequest :one
UPDATE vendor_requests
SET status = 'cancelled', is_canceled = true
WHERE id = sqlc.arg('id')::bigint
RETURNING *;
```

List/count queries gain an `include_canceled` filter (default excludes `is_canceled = true`) — Req 5.5:

```sql
  AND (sqlc.arg('include_canceled')::boolean = true OR vr.is_canceled = false)
```

**Service — `Cancel`.** Extend the existing transition flow:
- Guard already-canceled: if `req.IsCanceled` → `ErrAlreadyCanceled` (new sentinel) → HTTP 409, no duplicate audit (Req 5.7).
- State guard: only cancelable statuses (`draft`, `pending_approval`) via the existing `transitions` table → else `ErrInvalidTransition` (Req 5.9).
- Actor guard: existing `checkActor` union rule (creator Maker OR non-creator Checker) already matches Req 5.3 — keep it, add nothing (Req 5.8). Enforced at middleware (`vendorRequestCancelRoles`) AND service (`checkActor`).
- Set `status='cancelled'` **and** `is_canceled=true` in one tx (SoftCancelVendorRequest).
- Audit: exactly one `cancel` entry with `Before={is_canceled:false, state:prev}`, `After={is_canceled:true, state:'cancelled'}`, actor id, IP, UTC timestamp (Req 5.4).

Row and items are never deleted (Req 5.2) — `SoftCancelVendorRequest` only flips flags.

**Handler/response.** `is_canceled` exposed additively on detail + list (Req 5.10). List endpoint reads `include_canceled` query param (default false → active-only, Req 5.5).

#### RBAC (Req 6.2, project-context Sec 4)

Route gates unchanged: `vendorRequestViewerRoles`, `vendorRequestMakerRoles`, `vendorRequestCheckerRoles`, `vendorRequestCancelRoles` (already admits both Maker + Checker families). Service `checkActor` remains the source of truth for the four-eyes rule and the cancel union. No role changes — Req 6.2 preserved.

#### Read/write split (Req 6.4, project-context Sec 5)

Unchanged from current: the service is instantiated with the **primary** pool; all writes and read-after-write (`Create`/`Cancel` return `s.Get(...)`) stay on primary. Browse/list reporting reads follow the existing implementation's path. Money stays numeric/integer minor units; timestamps `timestamptz` stored UTC (Req 6.5).

---

### Frontend changes (per requirement)

All in `frontend/CompanyPortal-Vite/src/features/vendor-request/`, Merah Sirih tokens, 44px min touch targets.

#### Requirement 1 — filters (`ForecastBrowser.tsx`, `types.ts`, `api.ts`, `hooks.ts`)

- Add three filter controls beside the existing date + ATM-ID inputs:
  - **FLM Vendor** — required single-`<select>`, **no ALL/empty option** (Req 1.2). Options sourced from the vendor list (a small `useVendors`-style fetch or the existing vendor master endpoint).
  - **FLM Vendor Region** — required single-`<select>`, no ALL (Req 1.3). Options are the distinct regions (from vendor-branch master data).
  - **Brand** — optional `<select>` with a leading **"Semua"** (ALL) option mapping to empty string (Req 1.9).
- **Block fetch until both FLM Vendor and Region are chosen** (Req 1.4, 1.5): pass `enabled = flmVendor !== "" && flmVendorRegion !== ""` to `useForecastBrowse` (it already accepts an `enabled` arg). While unselected, render a prompt panel: "Pilih FLM Vendor dan FLM Vendor Region untuk menampilkan rekomendasi." plus a **link to `/forecasting/dmaa-forecast`** for the all-DMAA view (Req 1.12).
- On any filter change: reset `page` to 1 and refetch within 500ms (Req 1.11) — reuse the existing debounce pattern; the vendor/region/brand selects can refetch immediately (no debounce needed for discrete selects).
- Extend `BrowseForecastParams` (types.ts) with `brand?`, `flmVendor`, `flmVendorRegion`; `buildForecastQuery` (api.ts) sets `brand` (only if non-empty), `flm_vendor`, `flm_vendor_region`. `fetchAllForecastForSelection` must forward the same three filters so select-all respects them.
- Empty result → existing `ForecastTable` empty state, success with zero rows, no error (Req 1.13).

#### Requirement 2 — create-page columns + replenish_date (`VendorRequestCreate.tsx`)

- Add **Brand, FLM Vendor, FLM Vendor Region** columns to the items table, carried on the `ForecastRow` objects already in `selectionStore`. Render `"-"` (plain hyphen, never em-dash) for empty/null (Req 2.2).
- Add a **Replenish Date `<input type="date">`**, default **H+1 Asia/Jakarta**. Reuse/extend `lib/nextBusinessDay.ts` — but Req 2.4 asks for *calendar* H+1 (day after today), not next *business* day; add a `tomorrowJakartaISO()` helper computing the Asia/Jakarta calendar day + 1 (the existing `nextBusinessDayISO` skips weekends, which is the wrong default here). Document the distinction.
- Client-side validation mirrors the server: reject missing/invalid date and past dates, showing a field error and preserving input (Req 2.6, 2.7).
- `toPayload` sends `replenish_date` additively.

#### Requirement 3 — manual requests + category (`VendorRequestCreate.tsx`)

- Add a **"Buat Manual"** entry path (a mode toggle or a separate action from the Forecast Browser's "Buat Vendor Request") that starts a create with no `selectionStore` selection (Req 3.1) — the create page must no longer bail to the empty state when `pending` is null if the operator explicitly chose manual mode.
- **Request Category selector** (radio or `<select>`: Planned / Emergency / Additional), required before entering items (Req 3.2). Category drives the replenish-date control:
  - Planned → replenish_date locked to H+1 (Req 3.3), treated as DMAA-matching.
  - Emergency → locked to H+0 (Req 3.4).
  - Additional → date picker constrained to H+0/H+1/H+2 (Req 3.5).
- **Vendor single-select** (required for manual, Q2) so the number prefix resolves.
- **Manual item rows:** operator adds rows with terminal_id (1–64 chars), denom (positive int), amount_replenish (positive int) — Req 3.6. Zod schema extended; `is_manual: true` and `request_category` + `vendor_id` sent in payload.
- The existing DMAA-selection path sends `is_manual: false` and (for Planned) is validated against DMAA server-side (Req 3.8).

#### Requirement 5 — cancel + canceled badge (`VendorRequestDetail.tsx`, `VendorRequestList.tsx`, `StatusBadge.tsx`)

- **Cancel button** available to Maker (creator) AND SPV (checker) — widen `VendorRequestDetail.tsx`'s `canCancel` from the current creator-only UI to match the backend union (`isCreator || (isChecker && !isCreator)`) for `draft`/`pending_approval` (Req 5.3). The cancel mutation is already wired (`useCancelVendorRequest`).
- **Canceled badge** paired with icon + text, never color alone (Req 5.6, design accessibility rule): when `is_canceled` is true, render a `Badge` (neutral variant) with a `Ban`/`XCircle` Lucide icon + label "Dibatalkan" on both list rows and the detail header. Keep the existing strikethrough for the `cancelled` status too. `StatusBadge` (or a new `CanceledBadge`) reads `is_canceled` additively.
- **List default hides canceled** (Req 5.5): the list hook omits `include_canceled` (server default excludes); an optional "Tampilkan yang dibatalkan" toggle sets `include_canceled=true`.
- Detail + list `replenish_date` and `request_category` shown as labeled fields distinct from `forecast_date` (Req 2.9): add to `DetailFields` (e.g. "Tanggal Replenish", "Kategori"), rendering `"-"` when null.

#### Types / api / hooks

- `types.ts`: `VendorRequestDetail` + `VendorRequestSummary` gain `replenish_date: string | null`, `request_category: "planned"|"emergency"|"additional"|null`, `is_canceled: boolean`, `is_manual: boolean`. `CreateVendorRequestPayload` gains `replenish_date`, `request_category`, `is_manual`, `vendor_id`, and per-item optional `brand`/`lokasi_atm`. `BrowseForecastParams` gains the three filters. `ListVendorRequestParams` gains `includeCanceled?: boolean`.
- `api.ts`: query builders forward the new params; `createVendorRequest` payload extended.
- `hooks.ts`: no structural change; the cancel mutation already invalidates the whole prefix.

---

## Error Handling

- **Filters block fetch:** while FLM Vendor / Region unselected, no request fires; a prompt panel + DMAA-forecast link is shown (Req 1.5, 1.12). This is a distinct state from "empty result" (Req 1.13, existing empty table).
- **Filter too long (>255):** server 400 with the offending field; the frontend keeps the previously displayed result set (Req 1.14) — do not clear the table on a rejected filter change.
- **replenish_date invalid/past:** inline field error, form input preserved, no navigation (Req 2.6, 2.7).
- **Manual Planned with unmatched items:** server 400 (`InvalidItemsError`) names the items; the create form stays populated (Req 3.8).
- **Number exhausted:** 409 "sequence exhausted for this vendor/date"; surfaced as a toast (Req 4.7).
- **Already canceled:** 409; the detail refetches and shows the canceled badge; no duplicate audit (Req 5.7).
- **Loading:** reuse existing `ForecastTable` skeleton and `VendorRequestDetail` skeleton; `colSpan` on empty/error rows updated for the create table's new columns.

## Read/write split, money, timestamps

- Writes + read-after-write on **primary** (unchanged); reporting reads follow existing paths, replica where the current code already uses it (Req 6.4).
- All monetary amounts stay `bigint`/`int` integer minor units, never float (Req 3.14, 6.5). Currency IDR implicit as today.
- Timestamps `timestamptz`, stored UTC, displayed Asia/Jakarta; the H+0/H+1/H+2 date math is done in `Asia/Jakarta` on both client and server so the boundary agrees (Req 2.4, 3.3–3.5).

---

## Correctness Properties

These are the design's key invariants, checked via the example-based and integration tests described in the Testing strategy section below (which explains why generated-input property-based testing is not used for this CRUD-and-DDL change surface).

### Property 1: Request-number format and uniqueness

For every newly created Vendor_Request, the generated request_number matches the fixed 17-character shape `REP` + 3 uppercase chars + `YYYYMMDD` + a 3-digit zero-padded sequence, and is unique across all vendor_requests (the `vendor_requests_number_uq` constraint holds). The 3-digit sequence is per (vendor_id, replenish_date) and never exceeds 999; on exhaustion the create is rejected rather than producing a malformed or colliding number. Verified by the number-generation unit + concurrency/exhaustion integration tests (Testing strategy, Req 4).

**Validates: Requirements 4.1, 4.5, 4.6, 4.7, 4.8**

### Property 2: Soft-cancel preserves data and audit integrity

For every successful cancel, the vendor_requests row and all its vendor_request_items still exist (no deletion), is_canceled transitions false→true exactly once, and exactly one `cancel` audit_logs entry is written; a repeated cancel is rejected (409) and writes no additional audit entry. Verified by the soft-cancel service + handler tests (Testing strategy, Req 5).

**Validates: Requirements 5.2, 5.4, 5.7**

## Testing strategy

Property-based testing is **not** appropriate here: the changes are CRUD extensions, SQL filters, additive migrations, UI rendering, and a small deterministic string generator. The number generator and category date rules are the only "logic" and are best covered by focused table-driven unit tests plus integration tests against real Postgres for the concurrency/uniqueness guarantees. Per the workflow's PBT-applicability guidance (simple CRUD, config-like validation, UI rendering, DB-atomicity), example-based + integration tests give better signal than generated inputs. No Correctness Properties section.

Mapping to Req 6.7:

**Backend — filters (Req 1).** Integration test (real Postgres, `//go:build integration`): seed forecast rows across two brands, two vendors, two regions; assert
- Brand + FLM Vendor + Region combined returns only rows matching all three (AND) — Req 1.10;
- omitting Brand (empty sentinel) returns the full set for the vendor+region unchanged — Req 1.9, 1.16;
- case-insensitive match — Req 1.6–1.8;
- Count matches List under the same filters (joins added to Count) — pagination correctness;
- service rejects a browse with empty flm_vendor / flm_vendor_region — Req 1.4; and a >255-char filter — Req 1.14.

**Backend — replenish_date + category date rules (Req 2, 3).** Table-driven service unit tests (mock repo):
- default/explicit replenish_date persisted exactly (Req 2.5); missing/invalid/past rejected with field error (Req 2.6, 2.7);
- planned=H+1, emergency=H+0, additional∈{H+0,H+1,H+2} accepted; mismatches rejected with the conflict error (Req 3.3–3.5, 3.10);
- invalid category rejected (Req 3.11); zero items rejected (Req 3.12).

**Backend — manual accept vs Planned validate (Req 3).** Service tests:
- emergency/additional manual items with no DMAA row are accepted, `amount_refund=0`, `brand`/`lokasi_atm` persisted (Req 3.7);
- planned items validated against DMAA, unmatched named in `InvalidItemsError`, request not persisted (Req 3.8);
- non-positive denom / negative amount rejected (Req 3.14);
- exactly one create audit entry recording category + is_manual (Req 3.13).

**Backend — number generation (Req 4).** Unit tests for the formatter (prefix mapping incl. the fallback for ABACUS/unmapped → deterministic 3-char, 17-char fixed length — Req 4.1–4.3) + integration tests for the sequence:
- per-vendor-per-day scoping: two vendors same day get independent `001`; same vendor two days get independent `001` (Req 4.5);
- concurrent creates in one scope get unique numbers (spawn N goroutines against real Postgres; assert N distinct numbers, no gaps beyond retries) — Req 4.6;
- exhaustion at 999 → `ErrNumberExhausted`, nothing persisted (Req 4.7);
- uniqueness constraint preserved; legacy `VR-…` untouched (Req 4.8, 4.9).

**Backend — soft-cancel (Req 5).** Service + handler tests:
- cancel sets `is_canceled=true` AND `status='cancelled'`, row + items preserved (Req 5.2);
- exactly one audit entry with before/after is_canceled, actor, IP, UTC (Req 5.4);
- idempotency: second cancel → 409, no duplicate audit (Req 5.7);
- authz: creator Maker OK, non-creator SPV OK, unrelated actor 403 (middleware + service) — Req 5.3, 5.8;
- non-cancelable status → invalid-transition (Req 5.9);
- default list excludes canceled; `include_canceled=true` includes them (Req 5.5).

**Backend — no regression (Req 6).** Handler tests assert existing flat JSON fields unchanged and new fields additive (Req 6.1); role gates 401/403 preserved (Req 6.2); state machine transitions unchanged (Req 6.3).

**Frontend.** Component tests (Vitest + RTL):
- filters: fetch blocked until FLM Vendor + Region chosen, prompt + DMAA link shown; Brand ALL passes through; page resets to 1 on filter change (Req 1.4, 1.5, 1.9, 1.11);
- create page: replenish_date defaults to H+1 Jakarta; category selector locks/constrains the date per rules; manual rows accept terminal/denom/amount; context columns render `"-"` for empty (Req 2.2–2.4, 3.2–3.6);
- detail/list: canceled badge shows icon+text (not color alone); replenish_date + category shown as distinct labeled fields; cancel button visible to Maker and SPV (Req 5.3, 5.6, 2.9).

---

## sqlc regeneration caveat (project-context; matches prior specs)

`sqlc generate` rewrites unrelated already-generated files that have drifted (`approval.sql.go`, `audit.sql.go`, `auth.sql.go`, `models.go`). After regenerating: `git checkout --` everything except `backend/internal/db/vendor_request.sql.go`, keeping only the diff for the changed/new queries (`ListForecastForDate`, `CountForecastForDate`, `CreateVendorRequest`, `InsertVendorRequestItem`, `SoftCancelVendorRequest`, `NextRequestNumberSeq`, list/count `include_canceled`), plus the new `models.go` structs for the added columns — hand-edit those into `vendor_request.sql.go`/`models.go` to sqlc's convention if the regen is too noisy. Nullable columns emit as Go pointers (`*string`, `*time.Time`) because `backend/sqlc.yaml` sets `emit_pointers_for_null_types: true` — `replenish_date`, `request_category`, `vendor_id`, item `brand`/`lokasi_atm` all come out as pointers.

---

## Files touched

**Backend:**
- `backend/migrations/034_vendor_requests_cit2_columns.sql` — new: `is_canceled`, `request_category` (+CHECK), `replenish_date`, `is_manual`, item `brand`/`lokasi_atm`, indexes
- `backend/migrations/035_vendors_request_prefix.sql` — new: `vendors.request_prefix` (+CHECK)
- `backend/migrations/036_seed_vendor_request_prefix.sql` — new: seed the six vendor prefixes
- `backend/migrations/037_vendor_request_number_seq.sql` — new: `vendor_request_number_seq` table + `vendor_requests.vendor_id` (+FK, index)
- `backend/queries/vendor_request.sql` — filters on `ListForecastForDate`/`CountForecastForDate`; `CreateVendorRequest`/`InsertVendorRequestItem` new columns; `SoftCancelVendorRequest`; `NextRequestNumberSeq`; list/count `include_canceled`; retire `MaxRequestNumberSeqForDate`
- `backend/internal/db/vendor_request.sql.go`, `backend/internal/db/models.go` — regenerate (keep only vendor_request diff) or hand-edit
- `backend/internal/service/vendor_request.go` — `BrowseForecastParams` +3 filters; `CreateVendorRequestInput` +replenish/category/manual/vendor; `ForecastRow` unchanged (sibling); new `ErrAlreadyCanceled`; repo interface +`NextRequestNumberSeq`/`SoftCancelVendorRequest`, −`MaxRequestNumberSeqForDate`; prefix/fallback helper
- `backend/internal/service/vendor_request_actions.go` — filter validation in `BrowseForecast`; category/date/manual rules + `acceptManualItems` in `Create`; new number generator; `Cancel` sets `is_canceled` + already-canceled guard; list `include_canceled`
- `backend/internal/handler/vendor_request_handler.go` — read new query/body params (brand/flm_vendor/flm_vendor_region, replenish_date, request_category, is_manual, vendor_id, include_canceled); map `ErrAlreadyCanceled`→409
- `backend/internal/handler/vendor_request_response.go` — additive response fields (`replenish_date`, `request_category`, `is_canceled`, `is_manual`)
- tests: `vendor_request_test.go`, `vendor_request_forecast_test.go`, `vendor_request_handler_test.go`, + new integration tests for filters/sequence

**Frontend (`frontend/CompanyPortal-Vite/src/features/vendor-request/`):**
- `types.ts` — filter params; detail/summary/payload new fields; `ListVendorRequestParams.includeCanceled`
- `api.ts` — forward filters (incl. select-all), `include_canceled`, extended create payload
- `hooks.ts` — `useForecastBrowse` enabled-gate usage (already supported); optional vendor/region option hooks
- `ForecastBrowser.tsx` — three filter controls, block-fetch + prompt + DMAA link, page reset on filter change
- `ForecastTable.tsx` — no new columns needed (filters live on the page); `colSpan` unchanged from sibling
- `VendorRequestCreate.tsx` — context columns, replenish_date input (H+1 Jakarta), category selector + manual mode + vendor select + manual rows
- `VendorRequestList.tsx` — canceled badge, replenish_date column, include-canceled toggle
- `VendorRequestDetail.tsx` — widen `canCancel` to Maker+SPV, canceled badge, replenish_date + category fields
- `StatusBadge.tsx` (or new `CanceledBadge.tsx`) — canceled marker with icon + text
- `lib/nextBusinessDay.ts` — add `tomorrowJakartaISO()` (calendar H+1, distinct from business-day skip)
- tests co-located under `__tests__/`

---

## Phase note

This is the design phase of the requirements-first workflow. Requirements (6) are approved; this design resolves all five open questions (Q1–Q5) with recommendations and flags the schema/seed changes (migrations 034–037) as STOP-and-confirm gates requiring user sign-off before any migration is applied to the external Postgres. If any Q1 prefix value (notably `ABC` for ABACUS), the one-vendor-per-request constraint (Q2), or the `replenish_date`-drives-the-number decision (Q5) needs to change, that is a requirements-clarification return before tasks are authored.
