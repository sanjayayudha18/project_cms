# Implementation Plan

## Overview

Rename PostgreSQL tables `itm_cashpos` and `itm_cashpos_files` to `itm_replenish` and `itm_replenish_files`. This is a metadata-only rename that cascades across migrations, sqlc queries, generated Go code, Python ETL scheduler, integration tests, property tests, and frontend comments.

## Task Dependency Graph

```json
{
  "waves": [
    { "wave": 1, "tasks": ["1"] },
    { "wave": 2, "tasks": ["2"] },
    { "wave": 3, "tasks": ["3.1", "3.2", "3.4", "3.5", "3.6", "3.7"] },
    { "wave": 4, "tasks": ["3.3"] },
    { "wave": 5, "tasks": ["3.8", "3.9"] },
    { "wave": 6, "tasks": ["4"] }
  ]
}
```

## Tasks

- [x] 1. Write bug condition exploration test
  - **Model**: `claude-sonnet-4-20250514` (test writing, grep logic, straightforward assertions)
  - **Property 1: Bug Condition** - Old Table Names Exist in Schema and Code
  - **CRITICAL**: This test MUST FAIL on unfixed code - failure confirms the bug exists
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior - it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate the bug condition (old table names `itm_cashpos` / `itm_cashpos_files` still present)
  - **Scoped PBT Approach**: For this deterministic rename bug, scope the property to concrete references:
    - Query `information_schema.tables` and assert `itm_replenish` and `itm_replenish_files` exist (will FAIL on unfixed schema)
    - Query `pg_indexes` and assert all indexes use `itm_replenish` naming convention (will FAIL on unfixed schema)
    - Grep codebase for `itm_cashpos` references and assert zero matches (will FAIL on unfixed code)
  - Test asserts: after fix, `isBugCondition(ref)` returns false for all references (no `itm_cashpos` / `itm_cashpos_files` anywhere)
  - Run test on UNFIXED code
  - **EXPECTED OUTCOME**: Test FAILS (this is correct - it proves the old names still exist everywhere)
  - Document counterexamples found: tables `itm_cashpos`, `itm_cashpos_files` exist; indexes `itm_cashpos_*` exist; code references in sqlc, ETL, tests, frontend
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7_

- [x] 2. Write preservation property tests (BEFORE implementing fix)
  - **Model**: `claude-sonnet-4-20250514` (property-based test design, seed data generation, assertion logic)
  - **Property 2: Preservation** - Query Results and ETL Behavior Unchanged
  - **IMPORTANT**: Follow observation-first methodology
  - Observe on UNFIXED code:
    - Run `ListATMsWithCashPos` query with seed data → record result set (pagination, sorting, filtering)
    - Run `CountATMsWithCashPos` query → record count
    - Run `GetATMSummary` query → record status computation results
    - Run `GetLastUpdated` query → record timestamp result
    - Verify LEFT JOIN LATERAL returns latest replenishment record per terminal
    - Verify CASCADE delete behavior (delete file record → child rows deleted)
  - Write property-based tests:
    - For random ATM + replenishment seed data, assert `ListATMsWithCashPos` returns correct paginated results matching count
    - For random filter/sort combinations, assert pagination count matches list length
    - For status computation, assert status priority order: unconfigured > no_data > critical > low > normal
    - For duplicate file checksum, assert idempotent rejection (ETL skips already-processed files)
  - Run tests on UNFIXED code (using current `itm_cashpos` tables)
  - **EXPECTED OUTCOME**: Tests PASS (this confirms baseline behavior to preserve)
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

- [x] 3. Fix: Rename itm_cashpos → itm_replenish across full stack

  - [x] 3.1 Create migration file `backend/migrations/010_rename_itm_cashpos_to_itm_replenish.sql`
    - **Model**: `claude-haiku-4-20250414` (mechanical DDL rename, no reasoning needed)
    - UP migration (single transaction):
      - `ALTER TABLE itm_cashpos_files RENAME TO itm_replenish_files;`
      - `ALTER TABLE itm_cashpos RENAME TO itm_replenish;`
      - Rename primary keys: `itm_cashpos_files_pkey` → `itm_replenish_files_pkey`, `itm_cashpos_pkey` → `itm_replenish_pkey`
      - Rename unique constraint: `itm_cashpos_files_checksum_uq` → `itm_replenish_files_checksum_uq`
      - Rename foreign key: `fk_itm_cashpos_file` → `fk_itm_replenish_file`
      - Rename indexes: `itm_cashpos_file_idx` → `itm_replenish_file_idx`, `itm_cashpos_terminal_date_idx` → `itm_replenish_terminal_date_idx`, `itm_cashpos_replenish_date_idx` → `itm_replenish_replenish_date_idx`, `itm_cashpos_branch_code_idx` → `itm_replenish_branch_code_idx`
    - DOWN migration (rollback): reverse all renames to restore original names
    - _Bug_Condition: isBugCondition(input) where input.referencedName IN ["itm_cashpos", "itm_cashpos_files"] OR input.constraintName/indexName MATCHES ".*itm_cashpos.*"_
    - _Expected_Behavior: All schema objects use itm_replenish naming convention_
    - _Preservation: Metadata-only rename, no data movement, all existing rows preserved intact_
    - _Requirements: 2.1, 2.2, 2.7, 2.8, 2.10_

  - [x] 3.2 Update sqlc queries (`backend/queries/atm_portal.sql`)
    - **Model**: `claude-haiku-4-20250414` (find-and-replace in SQL, no reasoning needed)
    - Replace `FROM itm_cashpos cp` → `FROM itm_replenish cp` (3 occurrences: ListATMsWithCashPos, CountATMsWithCashPos, GetATMSummary)
    - Replace `FROM itm_cashpos` → `FROM itm_replenish` in GetLastUpdated query
    - _Bug_Condition: SQL references using old table name itm_cashpos_
    - _Expected_Behavior: All SQL queries reference itm_replenish_
    - _Preservation: Query logic, JOINs, WHERE clauses, and result structure unchanged_
    - _Requirements: 2.3_

  - [x] 3.3 Run `sqlc generate` to regenerate Go code
    - **Model**: `claude-haiku-4-20250414` (shell command execution + build verification)
    - Execute `sqlc generate` from backend directory
    - Verify `backend/internal/db/atm_portal.sql.go` now contains `itm_replenish` references
    - Verify generated code compiles: `go build ./...`
    - _Bug_Condition: Generated Go code contains hardcoded itm_cashpos SQL strings_
    - _Expected_Behavior: Generated Go code contains itm_replenish in all SQL string literals_
    - _Preservation: Function signatures, return types, and query parameters unchanged_
    - _Requirements: 2.4, 2.9_

  - [x] 3.4 Update Python ETL scheduler (`scheduler/itm/itm_cashpos_etl.py`)
    - **Model**: `claude-haiku-4-20250414` (find-and-replace in Python SQL strings)
    - Replace `INSERT INTO itm_cashpos (` → `INSERT INTO itm_replenish (`
    - Replace all `itm_cashpos_files` references → `itm_replenish_files` in SQL strings
    - Replace `DELETE FROM itm_cashpos WHERE` → `DELETE FROM itm_replenish WHERE`
    - Update module docstring and log messages to reference new table names
    - _Bug_Condition: Python SQL strings reference itm_cashpos / itm_cashpos_files_
    - _Expected_Behavior: All Python SQL strings reference itm_replenish / itm_replenish_files_
    - _Preservation: ETL logic, SHA-256 checksum deduplication, file state machine (pending→processing→completed/failed) unchanged_
    - _Requirements: 2.6_

  - [x] 3.5 Update integration tests (`backend/internal/handler/atm_portal_integration_test.go`)
    - **Model**: `claude-haiku-4-20250414` (find-and-replace in test fixtures)
    - Replace `INSERT INTO itm_cashpos_files` → `INSERT INTO itm_replenish_files`
    - Replace `INSERT INTO itm_cashpos` → `INSERT INTO itm_replenish`
    - _Bug_Condition: Test fixtures INSERT into old table names_
    - _Expected_Behavior: Test fixtures INSERT into itm_replenish / itm_replenish_files_
    - _Preservation: Test assertions, seed data structure, and behavioral coverage unchanged_
    - _Requirements: 2.5_

  - [x] 3.6 Update property tests (`backend/internal/service/atm_portal_property_test.go`, `backend/internal/service/atm_portal_filter_property_test.go`)
    - **Model**: `claude-haiku-4-20250414` (find-and-replace in test fixtures and comments)
    - In `atm_portal_property_test.go`: Replace `INSERT INTO itm_cashpos_files` → `INSERT INTO itm_replenish_files`, `INSERT INTO itm_cashpos` → `INSERT INTO itm_replenish`
    - In `atm_portal_filter_property_test.go`: Replace `itm_cashpos` in comments → `itm_replenish`
    - _Bug_Condition: Property test fixtures and comments reference old table names_
    - _Expected_Behavior: All property test references use itm_replenish naming_
    - _Preservation: Property test generators, assertions, and behavioral coverage unchanged_
    - _Requirements: 2.5_

  - [x] 3.7 Update frontend comments (`frontend/CompanyPortal-Vite/src/features/atm-portal/components/DataFreshnessIndicator.tsx`)
    - **Model**: `claude-haiku-4-20250414` (single comment string replace)
    - Replace `itm_cashpos` in comment → `itm_replenish`
    - _Bug_Condition: Frontend comment references old table name_
    - _Expected_Behavior: Comment references itm_replenish_
    - _Preservation: Component behavior, rendering, and props unchanged_
    - _Requirements: 2.5_

  - [x] 3.8 Verify bug condition exploration test now passes
    - **Model**: `claude-haiku-4-20250414` (re-run existing test, check pass/fail)
    - **Property 1: Expected Behavior** - All References Use itm_replenish
    - **IMPORTANT**: Re-run the SAME test from task 1 - do NOT write a new test
    - The test from task 1 encodes the expected behavior (no `itm_cashpos` references remain)
    - Run bug condition exploration test from step 1
    - **EXPECTED OUTCOME**: Test PASSES (confirms all references now use itm_replenish)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7_

  - [x] 3.9 Verify preservation tests still pass
    - **Model**: `claude-haiku-4-20250414` (re-run existing tests, check pass/fail)
    - **Property 2: Preservation** - Query Results and ETL Behavior Unchanged
    - **IMPORTANT**: Re-run the SAME tests from task 2 - do NOT write new tests
    - Run preservation property tests from step 2 against the renamed schema
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions - pagination, filtering, sorting, status computation, ETL idempotency, CASCADE all preserved)
    - Confirm all tests still pass after fix (no regressions)
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

- [ ] 4. Checkpoint - Ensure all tests pass
  - **Model**: `claude-sonnet-4-20250514` (full verification, grep analysis, rollback reasoning)
  - Run full Go test suite: `go test ./...` from backend directory
  - Run Python ETL tests (if any exist)
  - Run final grep verification: `grep -r "itm_cashpos" --include="*.go" --include="*.sql" --include="*.py" --include="*.tsx" --include="*.ts"` — must return ZERO matches
  - Verify migration rollback works: apply down migration, confirm old names restored, then re-apply up migration
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 2.9, 2.10, 3.6, 3.7_


## Notes

- This is a metadata-only rename at the PostgreSQL level (no data movement, no downtime).
- The migration uses `ALTER TABLE ... RENAME TO` which is instantaneous and locks only the catalog.
- All existing data, column definitions, types, and foreign key relationships are preserved.
- The DOWN migration enables safe rollback if issues are discovered post-deploy.
- Property-based tests provide stronger guarantees than unit tests for preservation (they test across the entire input domain rather than specific cases).
