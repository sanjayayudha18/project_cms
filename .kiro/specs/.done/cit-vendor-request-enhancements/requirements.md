# Requirements Document

## Introduction

This spec is a **targeted enhancement** of the existing CIT vendor-request / replenishment flow in the internal CMS app (`frontend/CompanyPortal-Vite`) and its ATM backend (`backend/`, port 8080, flat JSON). The base flow — Forecast Browser, Vendor Request create / list / detail, and the maker-checker state machine (`draft → pending_approval → approved/rejected → …`) — was shipped by the `request-replenish-to-vendor` spec (now in `.kiro/specs/done/`). A sibling in-flight spec, `update-cit-forecast-browser`, adds the four context columns (Lokasi ATM, Brand, FLM Vendor, FLM Vendor Region) to the forecast rows and the cross-page select-all. This spec builds on both and does **not** re-specify them.

Five distinct changes are requested (the user calls this "CIT 2"):

1. **Forecast Browser filters by Brand, FLM Vendor, and FLM Vendor Region.** These three fields already exist on each forecast row (`atms.brand`, `vendors.name`, `vendor_branches.region`, exposed by `update-cit-forecast-browser`). Add filter controls that narrow the displayed recommendations by these values, alongside the existing forecast-date + ATM (terminal) filters. The FLM Vendor and FLM Vendor Region filters are required single-value selections (no ALL / unfiltered option); only the Brand filter stays optional. The unfiltered all-DMAA view lives on the separate DMAA forecast page (`/forecasting/dmaa-forecast`), which is out of scope here.

2. **Surface Brand / FLM Vendor / FLM Vendor Region on the Vendor Request creation page, plus a chooseable replenish date (default H+1).** Today `VendorRequestCreate.tsx` shows only ATM ID, Denominasi, and Amount Replenish, and the "date" is a display-only `forecast_date` carried from the browser. Add the three context columns to the create table and let the operator choose the replenish date (defaulting to tomorrow, H+1).

3. **Manual replenish requests without a DMAA recommendation.** Today every item must match a `dmaa_atm_forecast` row (service `resolveItems` rejects unmatched items with `ErrInvalidItems`). Add a manual-request path where the operator first picks a **Request Category** — Planned, Emergency, or Additional — that governs the allowed/default replenish date, then enters ATM/terminal + amounts directly, without a DMAA source row.

4. **New request-number format.** Today the number is `VR-YYYYMMDD-NNNN` (e.g. `VR-20260720-0001`), generated in `createWithRetryingNumber`. Change it to `REP` + a 3-character vendor prefix (e.g. TAG / ADV / BJK) + `YYYYMMDD` + a 3-digit sequence — e.g. `REPTAG20260720001`.

5. **Soft-cancel by both SPV and Maker roles.** Today `Cancel` sets `status='cancelled'` and is allowed for the creator (any state) or a non-creator checker (pending only). Introduce an explicit `is_canceled` boolean so a cancel never deletes the DB row; a canceled request is visibly marked, excluded from active flows, and the cancel is audited. Both the Maker (creator) and the SPV (checker) roles may cancel.

Scope is the Forecast Browser page, the Vendor Request create / list / detail pages, the ATM backend `vendor_requests` module (SQL, service, handler), and the additive migrations they require. Downstream `approved → processing → completed` execution is out of scope.

## Dependency & Scope Note

- **Reuses** the existing chain: SQL `backend/queries/vendor_request.sql` → service `internal/service/vendor_request.go` + `vendor_request_actions.go` → handler `internal/handler/vendor_request_response.go` → frontend `features/vendor-request/` (`ForecastBrowser.tsx`, `ForecastTable.tsx`, `VendorRequestCreate.tsx`, `VendorRequestList.tsx`, `VendorRequestDetail.tsx`, `hooks.ts`, `api.ts`, `types.ts`, `selectionStore.ts`).
- **Depends on `update-cit-forecast-browser`** for the four context columns on forecast rows (`lokasi_atm`, `brand`, `flm_vendor`, `flm_vendor_region`) and the `ForecastRow` type/wire fields. Requirement 1 (filters) and Requirement 2 (create-page columns) assume those fields are present. If that sibling spec has not landed, its column work is a prerequisite, not a duplicate here.
- **Schema changes (all additive; propose here, confirm before migrating — Golden Rule).** New forward-only migration(s) under `backend/migrations/` (next number after `033`), each additive DDL only, no data migration:
  - `vendor_requests.is_canceled boolean NOT NULL DEFAULT false` (Requirement 5). Soft-cancel flag.
  - `vendor_requests.request_category text` (Requirement 3) — one of `planned | emergency | additional`, with a CHECK. Nullable or defaulted (design.md to decide; existing rows predate categories).
  - `vendor_requests.replenish_date date` (Requirement 2, 3) — the chosen date to the vendor, distinct from `forecast_date`. Nullable for backfill of existing rows.
  - Possibly `vendor_requests.vendor_id bigint` / a resolved vendor reference to support the request-number vendor prefix (Requirement 4) if the vendor is not otherwise derivable at create time. Flag for resolution.
  - Possibly additive columns on `vendor_request_items` if manual/context fields must persist (Requirement 3, open question below).
- **sqlc caveat (project-context, RBAC-Setup / update-cit-forecast-browser).** `sqlc generate` may rewrite unrelated already-generated files. Regenerate, then keep only the `vendor_request*.sql.go` diff (`git checkout --` the rest), or hand-edit to sqlc's output convention as prior specs did. Called out for tasks.md.
- **ATM backend keeps flat JSON** (project-context Sec 2 / Sec 4): the `vendor_requests` endpoints extend response objects **additively** — no field renames or removals — for wire compatibility with both frontends.
- **Open questions to resolve in design.md** (flagged, not silently decided here):
  - **Q1 — Vendor prefix mapping.** The requested prefixes are `TAG` / `ADV` / `BJK`, but seeded `vendors.code` values are `TAG`, `ADVANTAGE`, `BIJAK`, `ABACUS`, `ROH`, `SSI` (`005_seed_vendors.sql`). `TAG` maps directly; `ADV`→`ADVANTAGE` and `BJK`→`BIJAK` are truncations/abbreviations, and `ABACUS`/`ROH`/`SSI` have no requested prefix. The exact `vendors.code` → 3-char-prefix mapping (and the fallback for a vendor with no defined prefix) must be resolved and stored/derived deterministically.
  - **Q2 — Which vendor drives the number.** A vendor request today has no single vendor column; the vendor is derived per ATM via the active vendor package (which can differ across items). The request-number prefix needs one vendor per request. Resolve: does the create flow require all items to share one vendor, or is the vendor chosen explicitly at request creation?
  - **Q3 — Sequence scope.** Is the 3-digit sequence per-vendor-per-day, per-day-global, or per-vendor-global? 3 digits caps at 999 per scope — confirm the scope and the exhaustion behavior. The current implementation is per-`forecast_date` global with 4 digits.
  - **Q4 — Context-field persistence for manual requests.** In `update-cit-forecast-browser` the four context fields are display-only (NOT persisted to `vendor_request_items`). For manual requests (no DMAA row to re-derive from), decide whether Brand / FLM Vendor / FLM Vendor Region / Lokasi ATM must be persisted (on the item or header) so the request stays reconstructable, or whether they remain derived-at-read-time.
  - **Q5 — `forecast_date` vs `replenish_date` semantics.** Today `forecast_date` doubles as the effective date. With a chooseable replenish date and categories, decide whether `forecast_date` still means "the DMAA periode this was drawn from" and `replenish_date` is the new vendor-facing date, and how manual (no-DMAA) requests populate `forecast_date`.

## Glossary

- **Forecast_Browser_Page**: The route `/replenishment/forecast-browser` (nav "Rekomendasi CIT") in CompanyPortal-Vite, rendered by `ForecastBrowser.tsx`, role-gated to `ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`.
- **Forecast_Table**: The TanStack Table v8 instance in `ForecastTable.tsx` listing DMAA recommendation rows.
- **Forecast_Row**: One DMAA recommendation — `(terminal_id, periode_pred, denom)` with `amount_replenish`, `amount_refund`, `dmaa_file_id`, and the context fields `lokasi_atm`, `brand`, `flm_vendor`, `flm_vendor_region`.
- **Forecast_API**: The ATM backend endpoint `GET /api/v1/vendor-requests/forecast`.
- **Vendor_Request**: A `vendor_requests` row — a cash replenishment order to a CIT vendor with a maker-checker state machine.
- **Vendor_Request_Create_Page**: The route `/replenishment/vendor-requests/new`, rendered by `VendorRequestCreate.tsx`.
- **Vendor_Request_List_Page**: The route `/replenishment/vendor-requests`, rendered by `VendorRequestList.tsx`.
- **Vendor_Request_Detail_Page**: The route `/replenishment/vendor-requests/{id}`, rendered by `VendorRequestDetail.tsx`.
- **Vendor_Request_Service**: The backend `VendorRequestService` (`internal/service/vendor_request*.go`) owning validation, the state machine, and audit writes.
- **Brand**: The ATM machine brand — `atms.brand`; exposed on Forecast_Row as `brand`.
- **FLM_Vendor**: The CIT vendor servicing the ATM — `vendors.name` via the active `atm_vendor_packages → vendor_packages → vendor_branches → vendors` chain; exposed as `flm_vendor`.
- **FLM_Vendor_Region**: The vendor's regional grouping — `vendor_branches.region` (migration 030); exposed as `flm_vendor_region`.
- **Request_Category**: The manual-request classification chosen by the operator — one of Planned, Emergency, Additional (see below).
- **Planned_Category**: A Request_Category where the request matches DMAA data and is for H+1 (tomorrow).
- **Emergency_Category**: A Request_Category where the request may or may not exist in DMAA and is for replenishment on H+0 (the request date itself).
- **Additional_Category**: A Request_Category where the request is not in DMAA and is for filling on H+0, H+1, or H+2 at the operator's choice.
- **Replenish_Date**: The date the cash is to be delivered to the vendor/ATM, chosen by the operator (default H+1); stored on the Vendor_Request as `replenish_date`.
- **Manual_Request**: A Vendor_Request whose items are entered directly by the operator without a matching `dmaa_atm_forecast` row.
- **Request_Number**: The unique human-readable identifier on a Vendor_Request (`vendor_requests.request_number`), format changing per Requirement 4.
- **Vendor_Prefix**: The 3-character code embedded in a Request_Number identifying the CIT vendor (e.g. TAG / ADV / BJK), derived from `vendors.code`.
- **Is_Canceled**: The new boolean soft-cancel flag on a Vendor_Request (`vendor_requests.is_canceled`).
- **Maker_Role**: The role set that creates/submits/revises a request — `ADMIN`, `ATM-USER`, `BRANCH-ATM-USER` (service `makerRoles`).
- **Checker_Role**: The role set that approves/rejects — `ADMIN`, `ATM-SPV`, `BRANCH-ATM-SPV` (service `checkerRoles`); the SPV role.
- **Audit_Log**: The append-only `audit_logs` trail written on every state change (project-context Sec 4), via the transaction-scoped `audit.Writer`.

## Requirements

### Requirement 1: Forecast Browser filters — Brand, FLM Vendor, FLM Vendor Region

**User Story:** As an ATM operator, I want to filter the forecast recommendations by Brand, FLM Vendor, and FLM Vendor Region, so that I can narrow a large recommendation set to the ATMs, vendor, or region I am composing a request for.

#### Acceptance Criteria

1. THE Forecast_Browser_Page SHALL provide filter controls for Brand, FLM_Vendor, and FLM_Vendor_Region in addition to the existing forecast-date and ATM (terminal) filters.
2. THE Forecast_Browser_Page SHALL require the operator to select exactly one specific FLM_Vendor value, and the FLM_Vendor filter SHALL offer no ALL or empty option.
3. THE Forecast_Browser_Page SHALL require the operator to select exactly one specific FLM_Vendor_Region value, and the FLM_Vendor_Region filter SHALL offer no ALL or empty option.
4. THE Forecast_Browser_Page SHALL require an FLM_Vendor selection and an FLM_Vendor_Region selection before displaying any recommendations.
5. WHILE either the FLM_Vendor filter or the FLM_Vendor_Region filter has no selection, THE Forecast_Browser_Page SHALL NOT issue a forecast fetch and SHALL display a prompt directing the operator to select an FLM_Vendor and an FLM_Vendor_Region.
6. WHEN the operator selects a Brand value, THE Forecast_API SHALL return only Forecast_Rows whose `brand` field is an exact case-insensitive match of the selected value.
7. WHEN the operator selects a FLM_Vendor value, THE Forecast_API SHALL return only Forecast_Rows whose `flm_vendor` field is an exact case-insensitive match of the selected value.
8. WHEN the operator selects a FLM_Vendor_Region value, THE Forecast_API SHALL return only Forecast_Rows whose `flm_vendor_region` field is an exact case-insensitive match of the selected value.
9. WHERE the Brand filter is set to the empty-sentinel value (empty string), THE Forecast_API SHALL apply no filtering on the `brand` field, consistent with the existing `terminal_id` filter convention; the FLM_Vendor and FLM_Vendor_Region filters SHALL NOT use an empty sentinel because a specific value is required for each.
10. WHEN the operator applies the Brand filter together with the required FLM_Vendor and FLM_Vendor_Region filters, THE Forecast_API SHALL return only Forecast_Rows matching all applied filters (logical AND), combined with the existing date and ATM filters.
11. WHEN the operator changes any of the Brand, FLM_Vendor, or FLM_Vendor_Region filters, THE Forecast_Browser_Page SHALL reset the current page to page 1 and issue a refetch within 500 milliseconds of the change.
12. THE Forecast_Browser_Page SHALL direct operators who want to view all DMAA data to the DMAA forecast page (`/forecasting/dmaa-forecast`) by way of a link or helper text, and that all-data view SHALL be out of scope for this spec.
13. IF a selected filter value matches no Forecast_Rows for the active forecast date, THEN THE Forecast_Table SHALL display the existing empty state, return a success response with zero rows, and SHALL NOT display an error indication.
14. IF a submitted Brand, FLM_Vendor, or FLM_Vendor_Region filter value exceeds 255 characters, THEN THE Forecast_API SHALL reject the request and return an error response indicating the invalid filter value, while preserving the previously displayed result set.
15. THE filter controls SHALL use the design-system tokens and existing form/input components consistent with the Merah Sirih internal theme, with a minimum 44px by 44px touch target on every interactive filter control.
16. THE Forecast_API SHALL keep its existing flat JSON response shape and its existing `forecast_date`, `atm_id`, `page`, and `page_size` parameters, adding the new filter parameters additively such that omitting the optional Brand parameter returns a result set identical to the pre-enhancement behavior for the selected FLM_Vendor and FLM_Vendor_Region.

### Requirement 2: Create-page context columns and chooseable replenish date

**User Story:** As an ATM operator, I want the Vendor Request creation table to show Brand, FLM Vendor, and FLM Vendor Region and let me choose the replenish date (defaulting to tomorrow), so that I can review the operational context and set when cash is delivered before submitting.

#### Acceptance Criteria

1. THE Vendor_Request_Create_Page SHALL display, for each item row, the Brand, FLM_Vendor, and FLM_Vendor_Region carried from the Forecast_Row selection, in addition to the existing ATM ID, Denominasi, and Amount Replenish columns.
2. WHERE a Brand, FLM_Vendor, or FLM_Vendor_Region value is empty or null, THE Vendor_Request_Create_Page SHALL render a plain hyphen "-" placeholder rather than a blank cell, and SHALL NOT render an em-dash.
3. THE Vendor_Request_Create_Page SHALL provide a single Replenish_Date input that accepts one calendar date.
4. WHEN the Vendor_Request_Create_Page loads, THE Vendor_Request_Create_Page SHALL default the Replenish_Date to H+1, computed as the calendar day immediately after the current date in the Asia/Jakarta time zone.
5. WHEN the operator saves or submits a Vendor_Request, THE Vendor_Request_Service SHALL persist the chosen Replenish_Date on the Vendor_Request exactly as the selected calendar date.
6. IF the operator submits a Vendor_Request with a missing Replenish_Date or a Replenish_Date that is not a valid calendar date, THEN THE Vendor_Request_Service SHALL reject the request, retain any previously entered create-page input without persisting the Vendor_Request, and return a validation error identifying the Replenish_Date field.
7. IF the operator submits a Replenish_Date earlier than the current date in the Asia/Jakarta time zone, THEN THE Vendor_Request_Service SHALL reject the request, retain the create-page input without persisting the Vendor_Request, and return a validation error identifying the Replenish_Date field.
8. THE addition of the context columns SHALL NOT change the Forecast_Row identity used for selection and SHALL NOT alter which fields the create payload persists as line items, except where Requirement 3 (manual requests) or Q4 resolution requires persisting context fields.
9. THE Vendor_Request_Detail_Page and Vendor_Request_List_Page SHALL display the Replenish_Date as a labeled field distinct from the forecast/periode date wherever a date is shown for a Vendor_Request.
10. THE create, list, and detail responses SHALL include the Replenish_Date additively in the existing flat JSON shape, with no renaming or removal of existing fields.

### Requirement 3: Manual replenish requests with a Request Category

**User Story:** As an ATM operator, I want to create a replenishment request manually — choosing a Request Category and entering ATM and amounts directly — without needing a matching DMAA recommendation, so that I can handle planned, emergency, and additional cash needs that DMAA did not forecast.

#### Acceptance Criteria

1. THE Vendor_Request_Create_Page SHALL let the operator start a Manual_Request without any Forecast_Row selection.
2. WHEN starting a Manual_Request, THE Vendor_Request_Create_Page SHALL require the operator to choose a Request_Category of exactly one of Planned_Category, Emergency_Category, or Additional_Category before entering items.
3. WHEN the operator chooses Planned_Category, THE Vendor_Request_Create_Page SHALL set the Replenish_Date to H+1 and SHALL treat the request as matching DMAA data.
4. WHEN the operator chooses Emergency_Category, THE Vendor_Request_Create_Page SHALL set the Replenish_Date to H+0 (the request date itself).
5. WHEN the operator chooses Additional_Category, THE Vendor_Request_Create_Page SHALL allow the operator to choose a Replenish_Date of H+0, H+1, or H+2 in the Asia/Jakarta time zone and SHALL reject any other value.
6. WHILE a Manual_Request is being composed, THE Vendor_Request_Create_Page SHALL let the operator enter, for each item, an ATM/terminal identifier of 1 to 64 characters, a positive integer denomination, and a positive integer amount replenish.
7. WHEN a Manual_Request item does not match any `dmaa_atm_forecast` row, THE Vendor_Request_Service SHALL accept the item rather than rejecting it as an invalid item, provided the request is flagged as a Manual_Request of a valid Request_Category.
8. WHEN the operator submits a Planned_Category request, THE Vendor_Request_Service SHALL validate each item against `dmaa_atm_forecast` and SHALL reject the request identifying every item that does not match, without persisting the Vendor_Request.
9. THE Vendor_Request_Service SHALL persist the chosen Request_Category on the Vendor_Request.
10. IF the Replenish_Date is inconsistent with the chosen Request_Category (for example an Emergency_Category request with a Replenish_Date other than H+0, or a Planned_Category request with a Replenish_Date other than H+1), THEN THE Vendor_Request_Service SHALL reject the request with a validation error naming the conflict and SHALL NOT persist the Vendor_Request.
11. IF a submitted Request_Category is not one of Planned_Category, Emergency_Category, or Additional_Category, THEN THE Vendor_Request_Service SHALL reject the request with a validation error identifying the Request_Category field.
12. IF a Manual_Request is submitted with zero items, THEN THE Vendor_Request_Service SHALL reject the request with a validation error and SHALL NOT persist the Vendor_Request.
13. THE Vendor_Request_Service SHALL enforce the Request_Category and manual-item rules at both the middleware role gate and the service layer (project-context Sec 4 RBAC), and SHALL write exactly one Audit_Log entry on creation recording the Request_Category and whether the request is a Manual_Request.
14. WHERE amounts are entered manually, THE Vendor_Request_Service SHALL store monetary amounts as integer minor units / numeric, never floating point (project-context Sec 5), and SHALL reject non-positive denominations and negative amounts with a validation error identifying the offending field.

### Requirement 4: New request-number format

**User Story:** As an operator and an auditor, I want the request number to encode the vendor and date in a readable, sortable format, so that I can identify a request's vendor and day at a glance.

#### Acceptance Criteria

1. WHEN a Vendor_Request is created, THE Vendor_Request_Service SHALL generate a Request_Number composed of the literal prefix `REP`, followed by a Vendor_Prefix of exactly 3 uppercase characters, followed by an 8-character date segment in `YYYYMMDD` format, followed by a 3-digit zero-padded decimal sequence in the range `001` to `999`, producing a fixed-length 17-character value (for example `REPTAG20260720001`).
2. THE Vendor_Request_Service SHALL derive the Vendor_Prefix from the request's CIT vendor per the mapping resolved in design.md (Q1), where `vendors.code` = `TAG` maps to `TAG`, and the `ADV`/`BJK` prefixes map to their corresponding `vendors.code` values.
3. IF a vendor has no defined Vendor_Prefix mapping, THEN THE Vendor_Request_Service SHALL apply the deterministic fallback resolved in design.md (Q1) to produce a 3-character uppercase prefix, and SHALL NOT produce an empty, truncated, or non-3-character prefix.
4. THE date segment of the Request_Number SHALL use the date resolved in design.md (Q5) — either the Replenish_Date or the forecast/effective date — applied consistently for every Vendor_Request.
5. THE Vendor_Request_Service SHALL scope the 3-digit sequence per the scope resolved in design.md (Q3 — per-vendor-per-day, per-day-global, or per-vendor-global), SHALL start the sequence at `001` for the first Vendor_Request in a given scope, and SHALL increment it by exactly 1 for each subsequent Vendor_Request within that same scope.
6. WHEN two or more Vendor_Requests are created concurrently within the same sequence scope, THE Vendor_Request_Service SHALL guarantee unique Request_Numbers, and IF a unique-constraint conflict on `vendor_requests_number_uq` occurs, THEN THE Vendor_Request_Service SHALL retry number generation up to a bounded maximum of 5 attempts before returning an explicit error.
7. IF the sequence for a scope would exceed the 3-digit maximum of 999, THEN THE Vendor_Request_Service SHALL reject the creation, return an explicit error indicating sequence exhaustion for that scope, and SHALL NOT persist a malformed or colliding Request_Number.
8. THE Request_Number SHALL remain unique across all Vendor_Requests (`vendor_requests_number_uq` preserved) and SHALL be stored on `vendor_requests.request_number` as it is today.
9. THE Request_Number format change SHALL apply to newly created Vendor_Requests only and SHALL NOT rewrite existing `VR-YYYYMMDD-NNNN` numbers.
10. THE list and detail responses SHALL return the Request_Number in the existing `request_number` field with no shape change, and the Vendor_Request_List_Page request-number search SHALL continue to match against the stored value.

### Requirement 5: Soft-cancel by SPV and Maker roles

**User Story:** As a Maker or an SPV, I want to cancel a vendor request without deleting it, so that the record is retained and audited while being removed from active flows.

#### Acceptance Criteria

1. THE Vendor_Request table SHALL carry an Is_Canceled boolean field, defaulting to false, created by an additive migration if it does not already exist.
2. WHEN an authorized actor cancels a Vendor_Request, THE Vendor_Request_Service SHALL set Is_Canceled to true and SHALL NOT delete the `vendor_requests` row or any of its `vendor_request_items`.
3. THE Vendor_Request_Service SHALL permit a cancel action only for the request creator (a Maker_Role) or an SPV (a Checker_Role) actor, and SHALL deny it for all other actors.
4. WHEN a cancel completes successfully, THE Vendor_Request_Service SHALL write exactly one Audit_Log entry recording the actor identifier, the action value `cancel`, the prior Is_Canceled state (false), the resulting Is_Canceled state (true), the actor IP, and the UTC timestamp (project-context Sec 4).
5. WHILE a Vendor_Request has Is_Canceled true, THE Vendor_Request_Service SHALL exclude it from active-flow reads (approval queues and default active listings) unless the request explicitly sets a canceled filter to true.
6. WHEN the Vendor_Request_List_Page or Vendor_Request_Detail_Page renders a Vendor_Request with Is_Canceled true, THE page SHALL display a canceled badge or label paired with visible text and SHALL NOT signal the canceled state by color alone (design accessibility rule).
7. IF an actor attempts to cancel a Vendor_Request whose Is_Canceled is already true, THEN THE Vendor_Request_Service SHALL reject the action with a conflict error indicating the request is already canceled, SHALL preserve the existing canceled state, and SHALL NOT write a duplicate cancel Audit_Log entry.
8. IF an actor who is neither the request creator nor a Checker_Role attempts to cancel a Vendor_Request, THEN THE Vendor_Request_Service SHALL reject the action with an authorization error and SHALL leave Is_Canceled unchanged, enforced at both the middleware role gate and the service layer.
9. IF an actor attempts to cancel a Vendor_Request whose current status is not in the set of cancelable statuses defined in design.md (consistent with the existing state machine), THEN THE Vendor_Request_Service SHALL reject the action with an invalid-transition error and SHALL leave Is_Canceled unchanged.
10. THE list and detail responses SHALL expose the Is_Canceled state as an additive field in the existing flat JSON shape, with no renames or removals of existing fields.

### Requirement 6: No regressions to the existing flow

**User Story:** As a maintainer, I want these enhancements to preserve the current contracts and behavior, so that the existing Forecast Browser, Vendor Request flow, maker-checker gates, and wire compatibility are not broken.

#### Acceptance Criteria

1. THE ATM backend endpoints SHALL keep their existing flat JSON response shapes, extending objects additively with the new fields (Replenish_Date, Request_Category, Is_Canceled) — no field renames or removals (project-context Sec 2/Sec 4).
2. THE existing role gates (`ATM-USER`, `ATM-SPV`, `BRANCH-ATM-USER`, `BRANCH-ATM-SPV`) on both the routes and the endpoints SHALL remain unchanged, and the four-eyes rule (checker must differ from maker) SHALL remain enforced.
3. THE existing maker-checker state machine (`draft → pending_approval → approved/rejected`, revise, and the current cancel transition) SHALL continue to function, with cancel now also setting Is_Canceled per Requirement 5.
4. THE reads for browse, list, and detail SHALL use the read/write path consistent with the existing implementation (project-context Sec 5: writes and read-after-write on primary; reporting-style reads may target the replica where the existing code already does).
5. THE monetary amounts SHALL remain numeric / integer minor units and timestamps SHALL remain timestamptz stored UTC (project-context Sec 5).
6. THE new schema fields SHALL be introduced only via additive, forward-only migrations under `backend/migrations/`, each proposed and confirmed before migrating (Golden Rule), with no data migration of existing rows.
7. THE enhancement SHALL ship with tests covering: the three new Forecast_API filters (including combined AND filtering and empty-sentinel pass-through), the Replenish_Date default and category-driven date rules, the manual-request acceptance path versus the Planned DMAA-validation path, the new Request_Number generation and per-scope sequence/uniqueness/exhaustion, and the soft-cancel behavior (row preserved, Is_Canceled set, audit written, idempotency, and Maker/SPV authorization).
