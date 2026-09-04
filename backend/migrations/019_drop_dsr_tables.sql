-- Drop the four DSR ingest tables (SALDO HARIAN ATM + RENCANA ISI), reverting the schema
-- introduced by 013_dsr.sql and 017_atm_dsr_location_and_rencana_isi.sql, plus the
-- uploader-accountability columns added by 018_dsr_upload_accountability.sql (which live on
-- the two *_files tables and therefore die with them).
--
-- WHY: the DSR ingest schema is being discarded and will be rebuilt. This removes the tables
-- and all data in them.
--
-- SAFETY -- READ BEFORE APPLYING:
--   * DESTRUCTIVE + IRREVERSIBLE. This permanently deletes every row in all four tables.
--     Take a backup first (pg_dump -t 'atm_dsr_*') if the data has any value.
--   * DDL only; no other table is touched. The FK targets (public.atms, public.users) are
--     NOT dropped -- only the DSR tables that reference them go away.
--   * Nothing outside these four tables references them, so plain DROP TABLE (child rows
--     cascade via their own file-table FKs) is sufficient. Drop order is children-first;
--     no CASCADE keyword needed given that order.
--   * Re-runnable: every statement is IF EXISTS.
--   * sqlc: after applying, regenerate query code (sqlc generate). Any *.sql query files that
--     select from these tables will now fail to generate and must be removed/updated first.
--
-- ROLLBACK (forward-only shop, so documented rather than shipped as a .down.sql -- a down
-- file in this directory would also be picked up by sqlc's schema glob). This path RECREATES
-- the schema only; it does NOT restore data (that requires your pg_dump backup). To roll
-- back, re-apply the original DDL from 013_dsr.sql, then
-- 017_atm_dsr_location_and_rencana_isi.sql, then 018_dsr_upload_accountability.sql, in that
-- order -- they are the authoritative source for these four tables, their indexes,
-- constraints, and comments. Reproducing ~250 lines of CREATE TABLE here verbatim would be a
-- second copy to drift out of sync; point at the originals instead.

BEGIN;

-- Children first (leaf rows), then their file/header tables.
DROP TABLE IF EXISTS public.atm_dsr_rencana_isi_rows;
DROP TABLE IF EXISTS public.atm_dsr_rencana_isi_files;
DROP TABLE IF EXISTS public.atm_dsr_saldo_rows;
DROP TABLE IF EXISTS public.atm_dsr_saldo_files;

COMMIT;
