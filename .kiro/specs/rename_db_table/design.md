# Rename DB Table (itm_cashpos → itm_replenish) Bugfix Design

## Overview

The tables `itm_cashpos` and `itm_cashpos_files` were created with names that don't reflect their business domain (ITM replenishment data). This fix renames them to `itm_replenish` and `itm_replenish_files` via a new migration, then cascades the rename across all application code references: sqlc queries, Python ETL scheduler, integration tests, property tests, and frontend comments. The rename is metadata-only at the PostgreSQL level (no data movement, no downtime).

## Glossary

- **Bug_Condition (C)**: Any code or schema reference that uses the old table names `itm_cashpos` or `itm_cashpos_files`
- **Property (P)**: After the fix, all references resolve to `itm_replenish` or `itm_replenish_files` and produce correct query results
- **Preservation**: All existing query behavior (pagination, filtering, sorting, LEFT JOIN LATERAL, data freshness, ETL idempotency) must remain functionally identical
- **itm_cashpos**: The incorrectly-named table storing per-ATM replenishment event rows (to be renamed `itm_replenish`)
- **itm_cashpos_files**: The incorrectly-named file-tracking table for idempotent CSV ingest (to be renamed `itm_replenish_files`)
- **sqlc**: SQL-first code generator that produces type-safe Go from `.sql` query files
- **golang-migrate**: Versioned migration tool used for schema changes (`backend/migrations/`)

## Bug Details

### Bug Condition

The bug manifests when any part of the system references the tables by their incorrect legacy names (`itm_cashpos`, `itm_cashpos_files`). While this doesn't cause runtime failures today, it creates a semantic mismatch between the domain concept (ITM replenishment) and the schema/code naming, violating naming conventions and making the codebase harder to reason about.

**Formal Specification:**
```
FUNCTION isBugCondition(input)
  INPUT: input of type CodeOrSchemaReference
  OUTPUT: boolean
  
  RETURN input.referencedName IN ["itm_cashpos", "itm_cashpos_files"]
         OR input.constraintName MATCHES ".*itm_cashpos.*"
         OR input.indexName MATCHES ".*itm_cashpos.*"
END FUNCTION
```

### Examples

- `FROM itm_cashpos cp WHERE cp.terminal_id = a.terminal_id` in sqlc queries → should be `FROM itm_replenish cp WHERE cp.terminal_id = a.terminal_id`
- `INSERT INTO itm_cashpos (file_id, ...)` in Python ETL → should be `INSERT INTO itm_replenish (file_id, ...)`
- `SELECT id FROM itm_cashpos_files WHERE checksum = %s` in Python ETL → should be `SELECT id FROM itm_replenish_files WHERE checksum = %s`
- `INSERT INTO itm_cashpos_files (...)` in integration tests → should be `INSERT INTO itm_replenish_files (...)`
- Constraint `itm_cashpos_pkey` → should be `itm_replenish_pkey`
- Index `itm_cashpos_terminal_date_idx` → should be `itm_replenish_terminal_date_idx`

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**
- ATM portal queries (ListATMsWithCashPos, CountATMsWithCashPos, GetATMSummary, GetLastUpdated) must continue returning identical results for the same data
- LEFT JOIN LATERAL subquery pattern must continue returning the latest replenishment record per terminal
- Status computation logic (unconfigured > no_data > critical > low > normal) must remain identical
- Python ETL idempotent ingest via SHA-256 checksum must continue working
- File tracking (pending → processing → completed/failed) state machine must be preserved
- CASCADE delete behavior (itm_replenish rows deleted when parent file record is deleted) must be preserved
- All indexes must continue supporting the same query patterns with equivalent performance

**Scope:**
All inputs that do NOT reference the old table/constraint/index names should be completely unaffected by this fix. This includes:
- Queries against `atms`, `locations`, `regions`, and all other tables
- The ATM portal's filtering, sorting, and pagination logic
- The status computation CASE expression
- The CSV parsing logic in the Python ETL
- Authentication, RBAC, and all other middleware

## Hypothesized Root Cause

Based on the bug description, the root cause is straightforward:

1. **Original naming decision**: When the migration (`009_itm_cashpos.sql`) was created, the tables were named based on the source file pattern (`ATM_Cashpos_*.csv`) rather than the business domain concept (ITM replenishment records). The CSV files have since been renamed to `ATM_Replenish_*.csv` but the DB schema was never updated.

2. **Cascading references**: The incorrect name propagated to all layers that reference the tables: sqlc queries, generated Go code, Python ETL SQL strings, integration tests, property tests, and frontend comments.

3. **No runtime error**: Because the old name is consistent across all layers, the system works correctly — the bug is purely semantic/naming, not behavioral.

## Correctness Properties

Property 1: Bug Condition - All Table References Use Correct Names

_For any_ code or schema reference where the bug condition holds (isBugCondition returns true — i.e., references `itm_cashpos` or `itm_cashpos_files`), the fixed system SHALL use `itm_replenish` or `itm_replenish_files` respectively, and all queries SHALL resolve successfully against the renamed tables.

**Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7**

Property 2: Preservation - Query Results and ETL Behavior Unchanged

_For any_ query or ETL operation that does NOT depend on the literal table name (i.e., the logical behavior), the fixed system SHALL produce exactly the same results as the original system, preserving pagination, filtering, sorting, status computation, idempotent ingest, and CASCADE delete behavior.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7**

## Fix Implementation

### Changes Required

Assuming our root cause analysis is correct (naming inconsistency, metadata-only fix):

**File**: `backend/migrations/010_rename_itm_cashpos_to_itm_replenish.sql`

**Purpose**: New migration that renames tables, constraints, and indexes atomically.

**Specific Changes**:
1. **Table renames** (within a single transaction):
   - `ALTER TABLE itm_cashpos_files RENAME TO itm_replenish_files;`
   - `ALTER TABLE itm_cashpos RENAME TO itm_replenish;`

2. **Primary key constraint renames**:
   - `ALTER TABLE itm_replenish_files RENAME CONSTRAINT itm_cashpos_files_pkey TO itm_replenish_files_pkey;`
   - `ALTER TABLE itm_replenish RENAME CONSTRAINT itm_cashpos_pkey TO itm_replenish_pkey;`

3. **Unique constraint rename**:
   - `ALTER TABLE itm_replenish_files RENAME CONSTRAINT itm_cashpos_files_checksum_uq TO itm_replenish_files_checksum_uq;`

4. **Foreign key rename**:
   - `ALTER TABLE itm_replenish RENAME CONSTRAINT fk_itm_cashpos_file TO fk_itm_replenish_file;`

5. **Index renames**:
   - `ALTER INDEX itm_cashpos_file_idx RENAME TO itm_replenish_file_idx;`
   - `ALTER INDEX itm_cashpos_terminal_date_idx RENAME TO itm_replenish_terminal_date_idx;`
   - `ALTER INDEX itm_cashpos_replenish_date_idx RENAME TO itm_replenish_replenish_date_idx;`
   - `ALTER INDEX itm_cashpos_branch_code_idx RENAME TO itm_replenish_branch_code_idx;`

6. **Down migration** (rollback): reverse all renames to restore original names.

---

**File**: `backend/queries/atm_portal.sql`

**Purpose**: Update all SQL references from `itm_cashpos` to `itm_replenish`.

**Specific Changes**:
1. Replace `FROM itm_cashpos cp` with `FROM itm_replenish cp` (3 occurrences in ListATMsWithCashPos, CountATMsWithCashPos, GetATMSummary)
2. Replace `FROM itm_cashpos` with `FROM itm_replenish` in GetLastUpdated query

---

**File**: `backend/internal/db/atm_portal.sql.go`

**Purpose**: Regenerated automatically by `sqlc generate` after updating the query file. No manual edit needed.

---

**File**: `scheduler/itm/itm_cashpos_etl.py`

**Purpose**: Update SQL string constants and comments.

**Specific Changes**:
1. Replace `INSERT INTO itm_cashpos (` with `INSERT INTO itm_replenish (`
2. Replace all `itm_cashpos_files` references with `itm_replenish_files` in SQL strings
3. Replace `DELETE FROM itm_cashpos WHERE` with `DELETE FROM itm_replenish WHERE`
4. Update module docstring and log messages to reference new names

---

**File**: `backend/internal/handler/integration_test.go`

**Purpose**: Update migration file reference.

**Specific Changes**:
1. Update reference to migration file `009_itm_cashpos.sql` (path reference only, if applicable)

---

**File**: `backend/internal/handler/atm_portal_integration_test.go`

**Purpose**: Update INSERT statements in test fixtures.

**Specific Changes**:
1. Replace `INSERT INTO itm_cashpos_files` with `INSERT INTO itm_replenish_files`
2. Replace `INSERT INTO itm_cashpos` with `INSERT INTO itm_replenish`

---

**File**: `backend/internal/service/atm_portal_property_test.go`

**Purpose**: Update INSERT statements in property test fixtures.

**Specific Changes**:
1. Replace `INSERT INTO itm_cashpos_files` with `INSERT INTO itm_replenish_files`
2. Replace `INSERT INTO itm_cashpos` with `INSERT INTO itm_replenish`

---

**File**: `backend/internal/service/atm_portal_filter_property_test.go`

**Purpose**: Update comment references.

**Specific Changes**:
1. Replace `itm_cashpos` in comments with `itm_replenish`

---

**File**: `frontend/CompanyPortal-Vite/src/features/atm-portal/components/DataFreshnessIndicator.tsx`

**Purpose**: Update comment referencing old table name.

**Specific Changes**:
1. Replace `itm_cashpos` in comment with `itm_replenish`

## Testing Strategy

### Validation Approach

The testing strategy follows a two-phase approach: first, verify the migration executes correctly and renames all objects, then verify all application code references compile/run against the new names with unchanged behavior.

### Exploratory Bug Condition Checking

**Goal**: Confirm that the old table names exist in the current schema and that all code references resolve against them. This establishes the baseline before applying the rename.

**Test Plan**: Query `pg_catalog` to enumerate all objects named `itm_cashpos*` (tables, constraints, indexes). Run the existing test suite to confirm everything passes with old names.

**Test Cases**:
1. **Schema Inventory Test**: Query `information_schema.tables` and `pg_indexes` to list all `itm_cashpos*` objects (confirms baseline)
2. **Existing Test Suite**: Run `go test ./...` to confirm all tests pass before the rename
3. **Python ETL Dry Run**: Verify ETL connects and queries `itm_cashpos_files` successfully
4. **sqlc Generate**: Run `sqlc generate` to confirm current queries compile against old schema

**Expected Counterexamples**:
- All queries resolve correctly against `itm_cashpos` / `itm_cashpos_files` (no failures expected — the "bug" is semantic, not behavioral)

### Fix Checking

**Goal**: Verify that after the migration and code changes, all table references use `itm_replenish` / `itm_replenish_files` and resolve correctly.

**Pseudocode:**
```
FOR ALL reference WHERE isBugCondition(reference) DO
  result := resolveReference_fixed(reference)
  ASSERT result.resolvedTableName IN ["itm_replenish", "itm_replenish_files"]
  ASSERT result.queryExecutesSuccessfully = true
END FOR
```

### Preservation Checking

**Goal**: Verify that for all queries and operations, the fixed system produces the same results as the original system (since the rename is metadata-only, all data and behavior must be identical).

**Pseudocode:**
```
FOR ALL query WHERE NOT isBugCondition(query) DO
  ASSERT executeQuery_original(query) = executeQuery_fixed(query)
END FOR
```

**Testing Approach**: Since this is a metadata-only rename, preservation is guaranteed at the PostgreSQL level (ALTER TABLE RENAME does not touch data). The key risk is application-layer references that weren't updated. Property-based testing validates that:
- Random ATM configurations produce identical query results before and after
- Random CSV files are ingested identically before and after
- The status computation logic is unchanged

**Test Plan**: Run the full existing test suite (integration tests + property tests) after applying the migration and code changes. All tests must pass with zero behavioral difference.

**Test Cases**:
1. **Query Result Preservation**: Verify ListATMsWithCashPos returns same results for same seed data
2. **Status Computation Preservation**: Verify status CASE expression produces same results
3. **ETL Idempotency Preservation**: Verify duplicate file checksum is still rejected
4. **CASCADE Preservation**: Verify deleting a file record still cascades to child rows

### Unit Tests

- Verify migration up/down executes without error on a clean schema
- Verify `sqlc generate` produces compilable Go code after query updates
- Verify Python ETL SQL constants reference `itm_replenish` (grep/static analysis)
- Verify no remaining `itm_cashpos` references exist in the codebase (grep check)

### Property-Based Tests

- Generate random ATM + replenishment data, verify ListATMsWithCashPos query returns correct results against `itm_replenish`
- Generate random filter/sort combinations, verify pagination count matches list length
- Generate random CSV data, verify ETL inserts into `itm_replenish` correctly

### Integration Tests

- Run existing `atm_portal_integration_test.go` against the renamed schema — must pass unchanged
- Run existing `atm_portal_property_test.go` against the renamed schema — must pass unchanged
- Verify migration rollback (down) restores original table names and all queries still work
