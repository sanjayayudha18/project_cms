# Requirements Document

## Introduction

This feature splits the single identifier on the `public.vendor_package_prices` table into two distinct concepts so the Harga Paket screen can present two columns exactly as drawn in the crown-vendor-detail design:

- a human-readable, shared package **label** ("PAKET 1", "PAKET 2", ...) that **groups** the tiers, machine groups, price classes, and override levels that make up one commercial package, and
- a per-row **unique machine code** ("PKG1_ABA_001", "PKG1_ABA_002", ...) that identifies each individual price row.

Today the column `vendor_package_prices.package_code` holds the **label** ("PAKET 3") and serves as the grouping key across many rows. This feature therefore performs a **rename** of the existing column `package_code` -> `package` (the grouping key keeps its data, its `NOT NULL` constraint, and its role in every constraint and index that references it), plus an **add** of a new `package_code text` column holding the per-row unique code, backfilled deterministically as `PKG<digits-of-package>_<vendor.code>_<3-digit sequence per vendor ordered by id>`.

The change is a schema migration plus the code changes needed to keep the existing admin Harga Paket API and screen correct. It is delivered under the fixed CMS rules: schema is mutable in the current MVP stage but the change is recorded in `project-context.md` Sec 2 after it is applied; every state-changing action stays behind the master-data maker-checker engine and writes an audit log; writes go to the primary and reads go to the replica; overlap protection and the money-as-decimal-string rule are preserved unchanged. This feature feeds the separately-planned `vendor-detail-revamp` Harga Paket revamp: the "Kode Paket" column becomes the real persisted per-row `package_code`, and the "Paket" column and its filter group by the new `package` label.

The exact fallback token for a package label that contains no digits, and whether the legacy list filter parameter name `package_code` must be retained for backward compatibility, are open decisions flagged below for confirmation.

## Glossary

- **CMS**: The Cash Management System, comprising the ATM backend, the internal CompanyPortal frontend, and the shared PostgreSQL database.
- **Price_Row**: One `vendor_package_prices` row — a single priced grain (one vendor, package, machine group, price class, tier range, override level, and effective period).
- **Package_Label**: The human-readable grouping value ("PAKET 3") that identifies a commercial package shared across many Price_Rows. After this feature it is stored in the `vendor_package_prices.package` column.
- **Package_Code**: The per-row unique machine code ("PKG1_ABA_001") identifying a single Price_Row. After this feature it is stored in the `vendor_package_prices.package_code` column.
- **Vendor_Code**: The `vendors.code` value (for example "ABA") embedded in a generated Package_Code.
- **Package_Code_Format**: The Package_Code string form `PKG<digits>_<Vendor_Code>_<sequence>`, where `<digits>` is the numeric portion of the Package_Label, `<Vendor_Code>` is the owning vendor's code, and `<sequence>` is a 3-digit zero-padded per-vendor running number.
- **Price_Grain**: The set of fields that make one Price_Row distinct for overlap purposes: vendor, Package_Label, machine group, price class, override level (branch or ATM), tier range, and effective period.
- **Overlap_Constraint**: The GiST `EXCLUDE` constraint `vpp_no_overlap` that prevents two Price_Rows from overlapping on the same Price_Grain and period.
- **Lookup_Index**: The index `vpp_lookup_idx` supporting Price_Row lookups by vendor, grouping key, machine group, and price class.
- **Package_Frequencies**: The `package_frequencies` table whose primary key `(package_code, machine_group)` uses the Package_Label in its `package_code` column.
- **Package_Price_Endpoint**: The admin HTTP endpoints under `/api/v1/admin/vendors/{vendorID}/package-prices` (list, get, create, update, disable) that manage Price_Rows in flat JSON.
- **Package_Price_Service**: The backend service layer behind the Package_Price_Endpoint that validates, generates Package_Codes, and routes changes through the maker-checker engine.
- **MasterData_MakerChecker**: The existing master-data maker-checker engine that stages create, update, and disable actions as a pending change request and applies them only on approval.
- **Admin_Operator**: An authenticated internal user holding the `ADMIN` or `ADMIN_PARAM` role, authorized to manage vendor master data.
- **Package_Prices_Panel**: The frontend component (`PackagePricesPanel.tsx`) that renders the Harga Paket list and create form.
- **Migration_017**: The single numbered migration file (`017`) in `backend/migrations/` that performs the schema change and backfill.

## Requirements

### Requirement 1: Rename the grouping column to `package`

**User Story:** As a CMS maintainer, I want the existing grouping identifier renamed from `package_code` to `package`, so that the column name reflects its true role as a shared package label and frees the `package_code` name for the per-row code.

#### Acceptance Criteria

1. WHEN Migration_017 is applied, THE CMS SHALL rename the `vendor_package_prices.package_code` column to `package` while preserving every existing value in that column unchanged.
2. WHEN Migration_017 is applied, THE CMS SHALL preserve the `NOT NULL` constraint on the renamed `package` column.
3. WHEN Migration_017 is applied, THE CMS SHALL retain the grouping role of the renamed `package` column such that all Price_Rows that previously shared a `package_code` value continue to share the same `package` value.
4. WHEN Migration_017 is applied, THE CMS SHALL rebuild the Overlap_Constraint so that it partitions the Price_Grain by the `package` column rather than by the new `package_code` column.
5. WHEN Migration_017 is applied, THE CMS SHALL rebuild the Lookup_Index so that it references the `package` column rather than the new `package_code` column.

### Requirement 2: Add and backfill the per-row `package_code`

**User Story:** As an Admin_Operator, I want each existing price row to receive a unique machine code, so that every individual row can be identified independently of its shared package label.

#### Acceptance Criteria

1. WHEN Migration_017 is applied, THE CMS SHALL add a new `package_code text` column to `vendor_package_prices`.
2. WHEN Migration_017 backfills the new `package_code` column, THE CMS SHALL set each Price_Row's Package_Code to the Package_Code_Format `PKG<digits>_<Vendor_Code>_<sequence>`, where `<digits>` is the numeric portion extracted from that row's Package_Label, `<Vendor_Code>` is the owning vendor's `code`, and `<sequence>` is a 3-digit zero-padded running number assigned per vendor in ascending order of row `id`.
3. IF a Price_Row's Package_Label contains no numeric digits, THEN THE CMS SHALL substitute a deterministic fallback token for the `<digits>` portion so that the generated Package_Code remains non-null and stable across re-runs.
4. WHEN Migration_017 completes the backfill, THE CMS SHALL hold a non-null Package_Code for every existing Price_Row.
5. WHEN Migration_017 completes the backfill, THE CMS SHALL enforce that no two Price_Rows share the same Package_Code value across all vendors.
6. WHEN Migration_017 completes the backfill and validation, THE CMS SHALL apply a `NOT NULL` constraint and a global `UNIQUE(package_code)` constraint to the new `package_code` column.

### Requirement 3: Preserve overlap integrity across the rename

**User Story:** As a CMS maintainer, I want overlap protection to behave identically after the migration, so that the split does not allow two conflicting prices for the same package grain and period.

#### Acceptance Criteria

1. WHEN the migration completes, THE CMS SHALL reject any attempt to insert or activate a Price_Row that overlaps an existing Price_Row on the same Price_Grain (vendor, `package`, machine group, price class, override level, tier range) and overlapping effective period.
2. WHEN the Overlap_Constraint evaluates a candidate Price_Row, THE CMS SHALL use the `package` label column as the package partition of the Price_Grain.
3. WHEN two Price_Rows share the same Price_Grain and overlapping period but hold different Package_Code values, THE CMS SHALL treat them as overlapping and reject the second.
4. WHERE a Price_Grain includes a null branch override or a null ATM override, THE CMS SHALL evaluate overlap using the same null-coalescing behavior that the Overlap_Constraint applied before the migration.

### Requirement 4: Migration safety and validation

**User Story:** As a CMS maintainer, I want the schema change applied safely and validated before commit, so that a partial or incorrect migration cannot corrupt price data.

#### Acceptance Criteria

1. WHEN Migration_017 runs, THE CMS SHALL perform the rename, column add, backfill, constraint rebuild, and new constraints within a single database transaction.
2. WHEN Migration_017 executes, THE CMS SHALL be the migration numbered `017` in `backend/migrations/`.
3. WHILE Migration_017 is within its transaction and before it commits, THE CMS SHALL validate that no two Price_Rows share the same `package_code` value.
4. WHILE Migration_017 is within its transaction and before it commits, THE CMS SHALL validate that no Price_Row has a null `package` value and no Price_Row has a null `package_code` value.
5. IF any pre-commit validation in Migration_017 fails, THEN THE CMS SHALL abort the transaction so that no partial change is persisted.
6. WHEN Migration_017 is authored, THE CMS SHALL document the reasoning for reversing the change, including how the `package` column reverts to `package_code` and how the added per-row column and its constraints are removed.

### Requirement 5: Server-side Package_Code generation on create

**User Story:** As an Admin_Operator, I want the system to generate the per-row code automatically when I create a price, so that I never type a machine code and codes never collide.

#### Acceptance Criteria

1. WHEN an Admin_Operator submits a create request for a Price_Row, THE Package_Price_Service SHALL generate the Package_Code in the Package_Code_Format for the owning vendor without requiring the operator to supply a Package_Code.
2. IF a create request payload includes a Package_Code value, THEN THE Package_Price_Endpoint SHALL reject the supplied value and rely on server generation instead of persisting the client value.
3. WHEN the Package_Price_Service generates a Package_Code, THE Package_Price_Service SHALL produce a value that does not collide with any existing Package_Code, including Package_Codes belonging to other vendors.
4. WHEN a create request is routed through MasterData_MakerChecker, THE Package_Price_Service SHALL assign the definitive Package_Code at the point the change is applied on approval, so that a rejected or still-pending change request does not consume a Package_Code sequence value.
5. IF two Price_Row create requests for the same vendor are applied concurrently, THEN THE Package_Price_Service SHALL still produce distinct Package_Code values, relying on the global `UNIQUE(package_code)` constraint as the final collision guard.

### Requirement 6: API contract exposes both label and code

**User Story:** As a frontend consumer, I want the API to return both the package label and the per-row code, so that the Harga Paket screen can show two columns and group rows by label.

#### Acceptance Criteria

1. WHEN the Package_Price_Endpoint returns a Price_Row in a list or get response, THE Package_Price_Endpoint SHALL include both the `package` label field and the `package_code` per-row field.
2. WHEN the Package_Price_Endpoint serializes a Price_Row, THE Package_Price_Endpoint SHALL emit the response as flat JSON without a response envelope.
3. WHEN the Package_Price_Endpoint accepts a create payload, THE Package_Price_Endpoint SHALL accept the `package` label as a Price_Grain field and SHALL NOT accept a client-supplied `package_code`.
4. WHEN the Package_Price_Endpoint accepts an update payload, THE Package_Price_Endpoint SHALL treat both `package` and `package_code` as immutable and SHALL only allow changes to base price, SLA note, and effective end date.
5. WHERE the list endpoint accepts a package filter parameter, THE Package_Price_Endpoint SHALL filter by the `package` label grouping value.
6. WHEN the base price is present in any request or response, THE Package_Price_Endpoint SHALL represent the base price as an exact decimal string and SHALL NOT represent it as a floating-point number.

### Requirement 7: Maker-checker, audit, and immutability preserved

**User Story:** As a CMS maintainer, I want the maker-checker controls unchanged, so that this feature does not weaken the master-data governance already in place.

#### Acceptance Criteria

1. WHEN an Admin_Operator submits a create, update, or disable action on a Price_Row, THE Package_Price_Endpoint SHALL stage the action through MasterData_MakerChecker and return an HTTP 202 pending response.
2. WHEN a staged Price_Row change is approved, THE Package_Price_Service SHALL apply the effect and write an audit log entry recording the actor, action, and affected Price_Row.
3. IF an Admin_Operator submits a disable action on a Price_Row, THEN THE Package_Price_Service SHALL end the row's effective period by setting the effective end date and SHALL NOT hard-delete or re-enable the Price_Row.
4. WHEN an Admin_Operator submits an update to a Price_Row, THE Package_Price_Service SHALL reject changes to the Price_Grain fields (vendor, `package`, machine group, price class, tier range, override level, effective start date) and to `package_code`.
5. IF an applied Price_Row change would violate the Overlap_Constraint, THEN THE Package_Price_Endpoint SHALL return an HTTP 409 status surfaced from the overlap error.

### Requirement 8: Harga Paket frontend shows both columns

**User Story:** As an Admin_Operator, I want the Harga Paket list to show a "Kode Paket" column and a "Paket" column, so that I can read the per-row code and group rows by their package label.

#### Acceptance Criteria

1. WHEN the Package_Prices_Panel renders the Price_Row list, THE Package_Prices_Panel SHALL display a "Kode Paket" column bound to each row's per-row `package_code`.
2. WHEN the Package_Prices_Panel renders the Price_Row list, THE Package_Prices_Panel SHALL display a "Paket" column bound to each row's `package` label.
3. WHERE the Package_Prices_Panel groups or filters Price_Rows, THE Package_Prices_Panel SHALL group or filter by the `package` label value.
4. WHEN the Package_Prices_Panel renders the create form, THE Package_Prices_Panel SHALL collect the `package` label from the operator and SHALL NOT collect a `package_code` from the operator.
5. WHEN the Package_Prices_Panel displays a base price, THE Package_Prices_Panel SHALL render the value from the decimal string supplied by the API without converting it to a floating-point number.

### Requirement 9: Authorization and database topology unchanged

**User Story:** As the CMS, I want authorization and read/write routing unchanged, so that this feature keeps the platform's access and topology guarantees.

#### Acceptance Criteria

1. IF a request to the Package_Price_Endpoint carries a valid authentication token whose role is neither `ADMIN` nor `ADMIN_PARAM`, THEN THE Package_Price_Endpoint SHALL return an HTTP 403 status.
2. WHEN the Package_Price_Service persists a Price_Row change, THE Package_Price_Service SHALL execute the write against the primary database connection.
3. WHEN the Package_Price_Service reads Price_Rows for a list or get response, THE Package_Price_Service SHALL execute the read against the read replica connection.

### Requirement 10: Record the schema change in steering

**User Story:** As a CMS maintainer, I want the applied schema change recorded in the project context, so that the module and table map stays the source of truth per the apply-then-record rule.

#### Acceptance Criteria

1. WHEN Migration_017 has been applied, THE CMS SHALL record the renamed `package` column and the new per-row `package_code` column of `vendor_package_prices` in `project-context.md` Section 2.
2. WHEN the schema change is recorded, THE CMS SHALL note that any join between Price_Rows and Package_Frequencies now matches `vendor_package_prices.package` to `package_frequencies.package_code`.

## Open Questions and Assumptions

1. **Fallback token for label without digits (Requirement 2.3)** — needs business confirmation. Assumption: when a Package_Label contains no numeric digits, the `<digits>` portion is replaced by a fixed deterministic token (for example `0`) so the generated Package_Code stays non-null, unique, and stable across migration re-runs. Confirm the exact token before implementation.
2. **List filter parameter name (Requirement 6.5)** — needs decision. The list endpoint historically exposed a `package_code` filter that matched the label. Assumption/recommendation: introduce a `package` filter that groups by the label and carry both fields in the response; decide whether the legacy `package_code` filter parameter name must be retained as an alias for backward compatibility or removed.
3. **Generation timing under maker-checker (Requirement 5.4)** — assumption: the definitive Package_Code is assigned at apply-on-approve time (not at staging time) so pending or rejected requests never burn a sequence value. Confirm this timing is acceptable given that the staged request will not display a final Package_Code until approval.
