-- import_jobs: one registry for every file ingest (.claude/sdlc/import-export-jobs,
-- spec rev 2). Replaces retry_file_tracking, which only ever existed in the
-- archived pre-baseline 012_retry_scheduler.sql and was never applied. The
-- retry scheduler's companion tables (late_detections, scan_runs,
-- retry_audit_logs) are created here from that archive, except
-- retry_audit_logs.file_id, which now references import_jobs(id).
--
-- Concurrency is enforced by the three partial unique indexes, not by app
-- code (NFR1): the helper maps 23505 on their names to duplicate / in-progress.
--
-- SAFETY:
--   * Additive: four new tables, no existing table or row is touched.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

CREATE TABLE public.import_jobs (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source            text        NOT NULL,
    processing_date   date        NULL,
    file_hash         text        NOT NULL,
    original_filename text        NOT NULL,
    file_path         text        NULL,
    runtime           text        NOT NULL,
    detection_source  text        NOT NULL DEFAULT 'upload',
    status            text        NOT NULL DEFAULT 'pending',
    version           integer     NOT NULL DEFAULT 1,
    supersedes_job_id bigint      NULL REFERENCES public.import_jobs(id),
    row_count         integer     NULL,
    error_count       integer     NULL,
    error_message     text        NULL,
    auto_retry_count  integer     NOT NULL DEFAULT 0,
    max_retries       integer     NOT NULL DEFAULT 3,
    last_retry_at     timestamp with time zone NULL,
    created_by        bigint      NULL REFERENCES public.users(id),
    started_at        timestamp with time zone NULL,
    finished_at       timestamp with time zone NULL,
    created_at        timestamp with time zone NOT NULL DEFAULT now(),
    updated_at        timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT import_jobs_source_chk    CHECK (source ~ '^[a-z][a-z0-9_]{1,62}$'),
    CONSTRAINT import_jobs_hash_chk      CHECK (file_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT import_jobs_runtime_chk   CHECK (runtime IN ('go', 'python')),
    CONSTRAINT import_jobs_detection_chk CHECK (detection_source IN ('upload', 'not_processed', 'input_remaining')),
    CONSTRAINT import_jobs_status_chk    CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'max_retries_exhausted', 'superseded')),
    CONSTRAINT import_jobs_version_chk   CHECK (version >= 1),
    CONSTRAINT import_jobs_retry_chk     CHECK (auto_retry_count >= 0 AND max_retries >= 0),
    CONSTRAINT import_jobs_counts_chk    CHECK (coalesce(row_count, 0) >= 0 AND coalesce(error_count, 0) >= 0)
);

-- FR2/FR3/FR5a: at most one non-superseded row per file hash.
CREATE UNIQUE INDEX import_jobs_hash_uq ON public.import_jobs (source, file_hash)
    WHERE status <> 'superseded';
-- FR4: at most one pending/processing run per source + date (NULL dates never collide, FR4a).
CREATE UNIQUE INDEX import_jobs_inflight_uq ON public.import_jobs (source, processing_date)
    WHERE status IN ('pending', 'processing');
-- FR5/FR6/FR9: at most one current completed version per source + date.
CREATE UNIQUE INDEX import_jobs_current_uq ON public.import_jobs (source, processing_date)
    WHERE status = 'completed';
-- Retry scheduler + monitoring.
CREATE INDEX import_jobs_retry_idx ON public.import_jobs (status, source) WHERE status = 'failed';
CREATE INDEX import_jobs_date_idx  ON public.import_jobs (processing_date, source);
CREATE INDEX import_jobs_list_idx  ON public.import_jobs (created_at DESC, id DESC);

CREATE TRIGGER trg_import_jobs_set_updated_at BEFORE UPDATE ON public.import_jobs
    FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();

COMMENT ON TABLE public.import_jobs IS 'One row per ingested file. Idempotent per (source, file_hash); a different hash for a completed (source, processing_date) becomes version+1 and supersedes the old row on completion. Status machine: see .claude/sdlc/import-export-jobs/spec.md FR10.';
COMMENT ON COLUMN public.import_jobs.source IS 'dmaa | itm_cashpos | itm_replenish | dsr | new ingest sources';
COMMENT ON COLUMN public.import_jobs.processing_date IS 'NULL = source is not per date; such rows only follow the hash rule (FR4a).';
COMMENT ON COLUMN public.import_jobs.status IS 'pending | processing | completed | failed | max_retries_exhausted | superseded';
COMMENT ON COLUMN public.import_jobs.updated_at IS 'Also the staleness clock: pending/processing older than the per-source limit is marked failed (FR8).';

-- Companion tables from archived 012_retry_scheduler.sql (FR15).
CREATE TABLE public.late_detections (
    id              uuid NOT NULL DEFAULT gen_random_uuid(),
    file_type       varchar(20) NOT NULL,
    processing_date date NOT NULL,
    sla_deadline    time NOT NULL,
    detected_at     timestamp with time zone NOT NULL DEFAULT now(),
    resolved_at     timestamp with time zone,
    is_resolved     boolean NOT NULL DEFAULT false,
    created_at      timestamp with time zone NOT NULL DEFAULT now(),
    updated_at      timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT late_detections_pkey PRIMARY KEY (id),
    CONSTRAINT uq_late_detection UNIQUE (file_type, processing_date)
);

CREATE INDEX idx_late_detections_date ON public.late_detections (processing_date);

CREATE TRIGGER trg_late_detections_set_updated_at BEFORE UPDATE ON public.late_detections
    FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION public.set_updated_at();

-- Append-only: no UPDATE/DELETE is exposed by the application layer.
CREATE TABLE public.retry_audit_logs (
    id              uuid NOT NULL DEFAULT gen_random_uuid(),
    event_type      varchar(30) NOT NULL,
    trigger_type    varchar(10) NOT NULL,
    file_id         bigint REFERENCES public.import_jobs(id),
    file_type       varchar(20) NOT NULL,
    file_checksum   varchar(64),
    processing_date date NOT NULL,
    initiated_by    varchar(200) NOT NULL,
    outcome         varchar(20),
    duration_ms     integer,
    error_detail    text,
    created_at      timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT retry_audit_logs_pkey PRIMARY KEY (id)
);

COMMENT ON COLUMN public.retry_audit_logs.event_type IS 'retry_initiated | retry_completed';
COMMENT ON COLUMN public.retry_audit_logs.trigger_type IS 'auto | manual';

CREATE INDEX idx_retry_audit_date    ON public.retry_audit_logs (processing_date);
CREATE INDEX idx_retry_audit_type    ON public.retry_audit_logs (file_type);
CREATE INDEX idx_retry_audit_trigger ON public.retry_audit_logs (trigger_type);
CREATE INDEX idx_retry_audit_file    ON public.retry_audit_logs (file_id);

CREATE TABLE public.scan_runs (
    id             uuid NOT NULL DEFAULT gen_random_uuid(),
    scan_type      varchar(20) NOT NULL,
    started_at     timestamp with time zone NOT NULL,
    finished_at    timestamp with time zone,
    status         varchar(20) NOT NULL DEFAULT 'running',
    files_detected integer DEFAULT 0,
    error_message  text,
    created_at     timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT scan_runs_pkey PRIMARY KEY (id)
);

COMMENT ON COLUMN public.scan_runs.scan_type IS 'failure_detection | late_detection';
COMMENT ON COLUMN public.scan_runs.status IS 'running | success | failed';

CREATE INDEX idx_scan_runs_started ON public.scan_runs (started_at DESC);

COMMIT;
