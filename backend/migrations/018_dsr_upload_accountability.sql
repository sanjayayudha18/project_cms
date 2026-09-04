-- Adds uploader accountability to the DSR file tables (013_dsr.sql, extended by
-- 017_atm_dsr_location_and_rencana_isi.sql), for the vendor-upload-dsr feature
-- (.claude/sdlc/vendor-upload-dsr/). No audit_logs/notifications tables exist yet
-- in this codebase, so this migration scopes down to exactly the one column this
-- feature needs instead of building that shared platform infra as a side effect
-- (see plan.md "Corrections from spec.md" #2).

BEGIN;

ALTER TABLE IF EXISTS public.atm_dsr_saldo_files
    ADD COLUMN IF NOT EXISTS uploaded_by_user_id bigint;

COMMENT ON COLUMN public.atm_dsr_saldo_files.uploaded_by_user_id
    IS 'users.id of the vendor user who uploaded this file, resolved server-side from the JWT. NULL if uploaded by a non-portal path (e.g. manual backfill).';

ALTER TABLE IF EXISTS public.atm_dsr_saldo_files
    ADD CONSTRAINT fk_atm_dsr_saldo_files_uploaded_by FOREIGN KEY (uploaded_by_user_id)
    REFERENCES public.users (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE SET NULL;

ALTER TABLE IF EXISTS public.atm_dsr_rencana_isi_files
    ADD COLUMN IF NOT EXISTS uploaded_by_user_id bigint;

COMMENT ON COLUMN public.atm_dsr_rencana_isi_files.uploaded_by_user_id
    IS 'users.id of the vendor user who uploaded this file, resolved server-side from the JWT. NULL if uploaded by a non-portal path (e.g. manual backfill).';

ALTER TABLE IF EXISTS public.atm_dsr_rencana_isi_files
    ADD CONSTRAINT fk_atm_dsr_rencana_isi_files_uploaded_by FOREIGN KEY (uploaded_by_user_id)
    REFERENCES public.users (id) MATCH SIMPLE
    ON UPDATE NO ACTION
    ON DELETE SET NULL;

COMMIT;
