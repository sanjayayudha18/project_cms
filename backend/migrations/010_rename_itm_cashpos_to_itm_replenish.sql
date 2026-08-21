-- Rename itm_cashpos and itm_cashpos_files to itm_replenish and itm_replenish_files
-- Metadata-only rename: no data movement, all rows and relationships preserved.
-- Tables are renamed first (files before main), then constraints and indexes.

BEGIN;

-- Rename file tracking table and its constraints
ALTER TABLE itm_cashpos_files RENAME TO itm_replenish_files;
ALTER TABLE itm_replenish_files RENAME CONSTRAINT itm_cashpos_files_pkey TO itm_replenish_files_pkey;
ALTER TABLE itm_replenish_files RENAME CONSTRAINT itm_cashpos_files_checksum_uq TO itm_replenish_files_checksum_uq;

-- Rename main replenishment table and its constraints
ALTER TABLE itm_cashpos RENAME TO itm_replenish;
ALTER TABLE itm_replenish RENAME CONSTRAINT itm_cashpos_pkey TO itm_replenish_pkey;
ALTER TABLE itm_replenish RENAME CONSTRAINT fk_itm_cashpos_file TO fk_itm_replenish_file;

-- Rename indexes on the main replenishment table
ALTER INDEX itm_cashpos_file_idx RENAME TO itm_replenish_file_idx;
ALTER INDEX itm_cashpos_terminal_date_idx RENAME TO itm_replenish_terminal_date_idx;
ALTER INDEX itm_cashpos_replenish_date_idx RENAME TO itm_replenish_replenish_date_idx;
ALTER INDEX itm_cashpos_branch_code_idx RENAME TO itm_replenish_branch_code_idx;

COMMIT;
