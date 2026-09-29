-- import_jobs: "current version" uniqueness applies to uploaded files only
-- (.claude/sdlc/import-export-jobs/review.md, decision A).
--
-- A batch-ETL retry drains a whole folder, so several files detected for the same
-- source + date can each legitimately complete. With one completed row per
-- source + date, finishing file B marked file A `superseded` although it was never
-- replaced. Rows created by detect() (detection_source <> 'upload') therefore no
-- longer take part in the index; a completed upload (register(), detection_source =
-- 'upload') still supersedes the rest of its date.
--
-- SAFETY:
--   * Index only: no table or row is changed. Widening the predicate cannot fail
--     on existing data (the old index was stricter).
--   * Rollback: DROP INDEX import_jobs_current_uq, then recreate it without the
--     `detection_source = 'upload'` condition (fails if two detected rows are
--     completed for one source + date by then).
BEGIN;

DROP INDEX public.import_jobs_current_uq;
-- FR5/FR6/FR9: at most one current completed *upload* per source + date.
CREATE UNIQUE INDEX import_jobs_current_uq ON public.import_jobs (source, processing_date)
    WHERE status = 'completed' AND detection_source = 'upload';

COMMIT;
