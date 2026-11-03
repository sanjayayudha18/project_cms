-- Master-data import revamp (.kiro/specs/master-data-import-revamp, D5):
-- remember the uploaded file name so the import history can show it.
-- SAFETY: additive nullable column; existing batches stay NULL. Forward-only.
BEGIN;
ALTER TABLE public.master_data_import_batches
    ADD COLUMN file_name text NULL,
    ADD CONSTRAINT master_data_import_batches_file_name_chk
        CHECK (file_name IS NULL OR char_length(file_name) BETWEEN 1 AND 255);
COMMENT ON COLUMN public.master_data_import_batches.file_name IS
    'Original upload file name (base name, control chars stripped, max 255). NULL for batches before 036.';
COMMIT;
