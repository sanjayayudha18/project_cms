-- vendor-data-export job engine. Every status transition is one guarded UPDATE:
-- zero rows = transition rejected (design "Transisi status dan penjaganya").

-- name: LockExportJobsForUser :exec
-- Serialises one user's Create (dedupe + active-limit check) inside a tx.
SELECT pg_advisory_xact_lock(hashtextextended('export_jobs:' || sqlc.arg('user_id')::bigint::text, 0));

-- name: GetVendorCodeByID :one
-- Vendor exists (active or not, Req 1.6) -> its code.
SELECT code FROM vendors WHERE id = sqlc.arg('vendor_id')::bigint;

-- name: GetVendorBranchForVendor :one
-- No row when the cabang does not belong to that vendor (same 404 as unknown).
SELECT branch_code FROM vendor_branches
WHERE id = sqlc.arg('branch_id')::bigint AND vendor_id = sqlc.arg('vendor_id')::bigint;

-- name: FindActiveExportJobByHash :one
SELECT * FROM export_jobs
WHERE requested_by = sqlc.arg('requested_by')::bigint
  AND params_hash = sqlc.arg('params_hash')::text
  AND status IN ('queued', 'running');

-- name: CountActiveExportJobs :one
SELECT count(*) FROM export_jobs
WHERE requested_by = sqlc.arg('requested_by')::bigint AND status IN ('queued', 'running');

-- name: InsertExportJob :one
INSERT INTO export_jobs (requested_by, kind, entity, vendor_id, vendor_code, vendor_branch_id,
                         branch_code, status_filter, price_level, params_hash, file_name, report_params)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: ClaimNextExportJob :one
UPDATE export_jobs SET status = 'running', started_at = now()
WHERE id = (SELECT j.id FROM export_jobs j WHERE j.status = 'queued'
            ORDER BY j.id FOR UPDATE SKIP LOCKED LIMIT 1)
RETURNING *;

-- name: UpdateExportJobProgress :execrows
UPDATE export_jobs SET row_count = sqlc.arg('row_count')::bigint
WHERE id = sqlc.arg('id')::bigint AND status = 'running';

-- name: MarkExportJobSucceeded :execrows
UPDATE export_jobs
SET status = 'succeeded', finished_at = now(),
    expires_at = now() + make_interval(secs => sqlc.arg('retention_seconds')::double precision),
    row_count = sqlc.arg('row_count')::bigint,
    byte_size = sqlc.arg('byte_size')::bigint,
    chunk_count = sqlc.arg('chunk_count')::int
WHERE id = sqlc.arg('id')::bigint AND status = 'running';

-- name: MarkExportJobFailed :execrows
UPDATE export_jobs
SET status = 'failed', finished_at = now(), error_message = sqlc.arg('error_message')::text
WHERE id = sqlc.arg('id')::bigint AND status = 'running';

-- name: ReleaseExportJob :execrows
UPDATE export_jobs SET status = 'queued', started_at = NULL
WHERE id = sqlc.arg('id')::bigint AND status = 'running';

-- name: StartExportJobDownload :one
UPDATE export_jobs SET status = 'downloading', download_started_at = now()
WHERE id = sqlc.arg('id')::bigint AND requested_by = sqlc.arg('requested_by')::bigint
  AND status = 'succeeded' AND expires_at > now()
RETURNING *;

-- name: CompleteExportJobDownload :execrows
UPDATE export_jobs SET status = 'downloaded', downloaded_at = now()
WHERE id = sqlc.arg('id')::bigint AND status = 'downloading';

-- name: AbortExportJobDownload :execrows
UPDATE export_jobs SET status = 'succeeded'
WHERE id = sqlc.arg('id')::bigint AND status = 'downloading';

-- name: GetExportJobForUser :one
SELECT * FROM export_jobs
WHERE id = sqlc.arg('id')::bigint AND requested_by = sqlc.arg('requested_by')::bigint;

-- name: ListExportJobsForUser :many
SELECT * FROM export_jobs
WHERE requested_by = sqlc.arg('requested_by')::bigint
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('row_limit')::int;

-- name: InsertExportJobChunk :exec
INSERT INTO export_job_chunks (export_job_id, seq, data) VALUES ($1, $2, $3);

-- name: GetExportJobChunk :one
SELECT data FROM export_job_chunks
WHERE export_job_id = sqlc.arg('export_job_id')::bigint AND seq = sqlc.arg('seq')::int;

-- name: DeleteExportJobChunks :exec
DELETE FROM export_job_chunks WHERE export_job_id = sqlc.arg('export_job_id')::bigint;

-- Janitor (all idempotent, primary) ------------------------------------------

-- name: FailTimedOutExportJobs :many
-- running past JobTimeout -> failed + chunks deleted, one statement. Returns failed ids.
WITH f AS (
    UPDATE export_jobs
    SET status = 'failed', finished_at = now(), error_message = sqlc.arg('error_message')::text
    WHERE status = 'running'
      AND started_at < now() - make_interval(secs => sqlc.arg('timeout_seconds')::double precision)
    RETURNING id
), d AS (
    DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM f)
)
SELECT id FROM f;

-- name: RecoverStaleDownloads :execrows
UPDATE export_jobs SET status = 'succeeded'
WHERE status = 'downloading'
  AND download_started_at < now() - make_interval(secs => sqlc.arg('lease_seconds')::double precision);

-- name: ExpireExportJobs :many
WITH e AS (
    UPDATE export_jobs SET status = 'expired', expired_at = now()
    WHERE status = 'succeeded' AND expires_at <= now()
    RETURNING id
), d AS (
    DELETE FROM export_job_chunks WHERE export_job_id IN (SELECT id FROM e)
)
SELECT id FROM e;

-- name: DeleteOrphanExportJobChunks :execrows
DELETE FROM export_job_chunks c
USING export_jobs j
WHERE j.id = c.export_job_id AND j.status NOT IN ('running', 'succeeded', 'downloading');
