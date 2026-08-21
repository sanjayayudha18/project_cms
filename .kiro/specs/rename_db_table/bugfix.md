# Bugfix Requirements Document

## Introduction

Two PostgreSQL tables in the CMS database were created with incorrect names (`itm_cashpos` and `itm_cashpos_files`) that do not reflect their actual business domain purpose. These tables store ITM replenishment data parsed from CSV files, not generic "cash position" data. The correct names are `itm_replenish` and `itm_replenish_files`. Since these tables are already integrated across the full stack (migrations, sqlc queries, generated Go code, service layer, handler layer, integration tests, property tests, Python ETL scheduler, and frontend comments), the rename must cascade safely to all references.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN the database schema is inspected THEN the system uses table name `public.itm_cashpos` which does not accurately describe its domain purpose (ITM replenishment records)

1.2 WHEN the database schema is inspected THEN the system uses table name `public.itm_cashpos_files` which does not accurately describe its domain purpose (ITM replenishment file tracking)

1.3 WHEN sqlc queries reference the tables THEN they use `itm_cashpos` and `itm_cashpos_files` in SQL statements (backend/queries/atm_portal.sql)

1.4 WHEN the generated Go DB layer references the tables THEN it contains hardcoded `itm_cashpos` table names in SQL string literals (backend/internal/db/atm_portal.sql.go)

1.5 WHEN integration tests and property tests seed data THEN they INSERT into `itm_cashpos` and `itm_cashpos_files` using the incorrect names

1.6 WHEN the Python ETL scheduler ingests CSV data THEN it INSERTs into `itm_cashpos` and queries `itm_cashpos_files` using the incorrect names (scheduler/itm/itm_cashpos_etl.py)

1.7 WHEN the migration file defines table constraints and indexes THEN they use naming conventions based on `itm_cashpos` (e.g., `itm_cashpos_pkey`, `itm_cashpos_files_checksum_uq`, `fk_itm_cashpos_file`, `itm_cashpos_file_idx`, `itm_cashpos_terminal_date_idx`, `itm_cashpos_replenish_date_idx`, `itm_cashpos_branch_code_idx`)

### Expected Behavior (Correct)

2.1 WHEN the database schema is inspected THEN the system SHALL use table name `public.itm_replenish` to accurately describe its domain purpose

2.2 WHEN the database schema is inspected THEN the system SHALL use table name `public.itm_replenish_files` to accurately describe its domain purpose

2.3 WHEN sqlc queries reference the tables THEN they SHALL use `itm_replenish` and `itm_replenish_files` in all SQL statements

2.4 WHEN sqlc generate is run THEN the generated Go DB layer SHALL contain `itm_replenish` table names in all SQL string literals

2.5 WHEN integration tests and property tests seed data THEN they SHALL INSERT into `itm_replenish` and `itm_replenish_files`

2.6 WHEN the Python ETL scheduler ingests CSV data THEN it SHALL INSERT into `itm_replenish` and query `itm_replenish_files`

2.7 WHEN the new migration is applied THEN all constraints and indexes SHALL be renamed to use `itm_replenish` convention (e.g., `itm_replenish_pkey`, `itm_replenish_files_checksum_uq`, `fk_itm_replenish_file`, `itm_replenish_file_idx`, `itm_replenish_terminal_date_idx`, `itm_replenish_replenish_date_idx`, `itm_replenish_branch_code_idx`)

2.8 WHEN the rename migration is applied THEN it SHALL execute as a single transaction using `ALTER TABLE ... RENAME TO` and `ALTER INDEX ... RENAME TO` statements to ensure atomicity

2.9 WHEN the Go backend is built after the rename THEN it SHALL compile without errors with all references updated

2.10 WHEN existing data is queried after the migration THEN all existing rows SHALL be preserved intact (rename is metadata-only, no data loss)

### Unchanged Behavior (Regression Prevention)

3.1 WHEN the ATM portal queries ATM cash position data THEN the system SHALL CONTINUE TO return correct paginated, filtered, sorted results with latest replenishment data per terminal

3.2 WHEN the data freshness indicator queries the most recent record THEN the system SHALL CONTINUE TO return the correct last_updated timestamp (or null when no records exist)

3.3 WHEN the Python ETL scheduler processes a new CSV file THEN the system SHALL CONTINUE TO perform idempotent ingest using SHA-256 checksum deduplication

3.4 WHEN a previously-processed CSV file is encountered THEN the system SHALL CONTINUE TO skip it based on the checksum uniqueness constraint

3.5 WHEN the ATM portal queries a terminal with no replenishment data THEN the system SHALL CONTINUE TO return null for cash-position-derived fields (LEFT JOIN LATERAL behavior preserved)

3.6 WHEN integration tests and property tests run THEN they SHALL CONTINUE TO pass with the same assertions and behavioral coverage

3.7 WHEN the migration is rolled back (down migration) THEN the system SHALL CONTINUE TO restore the original table names without data loss

---

## Bug Condition (Formal)

```pascal
FUNCTION isBugCondition(X)
  INPUT: X of type DatabaseTableReference
  OUTPUT: boolean
  
  // Returns true when the reference uses the old incorrect table name
  RETURN X.tableName = "itm_cashpos" OR X.tableName = "itm_cashpos_files"
END FUNCTION
```

```pascal
// Property: Fix Checking — All table references use correct names
FOR ALL X WHERE isBugCondition(X) DO
  result ← resolveTableName'(X)
  ASSERT result.tableName = "itm_replenish" OR result.tableName = "itm_replenish_files"
END FOR
```

```pascal
// Property: Preservation Checking — Non-affected queries behave identically
FOR ALL X WHERE NOT isBugCondition(X) DO
  ASSERT F(X) = F'(X)
END FOR
```

**Key Definitions:**
- **F**: The system before the rename (uses `itm_cashpos` / `itm_cashpos_files`)
- **F'**: The system after the rename (uses `itm_replenish` / `itm_replenish_files`)
- The rename is metadata-only at the DB level — data, column definitions, types, and constraints remain functionally identical
