-- CSV import batches (plan.md T5.4): one row per confirmed import file, so the
-- same file (same entity + SHA-256) is idempotent -- a re-upload returns the
-- existing batch instead of staging the changes twice -- and so the change
-- requests it created (master_data_change_requests.batch_id) share ONE approval.
--
-- SAFETY:
--   * Additive: new table + a nullable FK on an already-nullable column
--     (every existing batch_id is NULL, so the FK validates trivially).
--   * Forward-only: no down migration ships (project convention).

BEGIN;

CREATE TABLE public.master_data_import_batches (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity text NOT NULL,
    file_hash text NOT NULL,
    maker_id bigint NOT NULL REFERENCES public.users(id),
    row_count integer NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT master_data_import_batches_hash_chk CHECK (file_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT master_data_import_batches_rows_chk CHECK (row_count > 0)
);

CREATE INDEX master_data_import_batches_lookup_idx
    ON public.master_data_import_batches USING btree (entity, file_hash, id DESC);

ALTER TABLE public.master_data_change_requests
    ADD CONSTRAINT master_data_change_requests_batch_fk
    FOREIGN KEY (batch_id) REFERENCES public.master_data_import_batches(id);

COMMENT ON TABLE public.master_data_import_batches IS 'One confirmed master-data CSV import. Idempotent per (entity, file_hash) while its changes are not rejected/stale; all its change requests share one approval_requests row (document_id = the first change request).';

COMMIT;
