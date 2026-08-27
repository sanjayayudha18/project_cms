-- EOD Retry Scheduler operational tables.
-- Owned/written by the standalone scheduler/retry_scheduler service.
-- Reads dmaa_files/itm_cashpos_files/itm_replenish_files for status but never writes them.

BEGIN;

CREATE TABLE IF NOT EXISTS public.retry_file_tracking
(
    id                 uuid NOT NULL DEFAULT gen_random_uuid(),
    file_type          varchar(20) NOT NULL,
    filename           varchar(500) NOT NULL,
    file_path          varchar(1000) NOT NULL,
    file_checksum      varchar(64) NOT NULL,
    processing_date    date NOT NULL,
    detection_source   varchar(20) NOT NULL,
    failure_reason     text,
    processing_status  varchar(30) NOT NULL DEFAULT 'failed',
    auto_retry_count   integer NOT NULL DEFAULT 0,
    max_retries        integer NOT NULL DEFAULT 3,
    detected_at        timestamptz NOT NULL DEFAULT now(),
    last_retry_at      timestamptz,
    completed_at       timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT retry_file_tracking_pkey PRIMARY KEY (id),
    CONSTRAINT uq_retry_file_checksum_date UNIQUE (file_checksum, processing_date)
);

COMMENT ON COLUMN public.retry_file_tracking.file_type
    IS 'dmaa | itm_cashpos | itm_replenish';
COMMENT ON COLUMN public.retry_file_tracking.detection_source
    IS 'not_processed | input_remaining';
COMMENT ON COLUMN public.retry_file_tracking.processing_status
    IS 'pending | processing | completed | failed | max_retries_exhausted';

CREATE INDEX IF NOT EXISTS idx_retry_file_tracking_date ON public.retry_file_tracking(processing_date);
CREATE INDEX IF NOT EXISTS idx_retry_file_tracking_status ON public.retry_file_tracking(processing_status);
CREATE INDEX IF NOT EXISTS idx_retry_file_tracking_type ON public.retry_file_tracking(file_type);

CREATE TABLE IF NOT EXISTS public.late_detections
(
    id                 uuid NOT NULL DEFAULT gen_random_uuid(),
    file_type          varchar(20) NOT NULL,
    processing_date    date NOT NULL,
    sla_deadline       time NOT NULL,
    detected_at        timestamptz NOT NULL DEFAULT now(),
    resolved_at        timestamptz,
    is_resolved        boolean NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT late_detections_pkey PRIMARY KEY (id),
    CONSTRAINT uq_late_detection UNIQUE (file_type, processing_date)
);

CREATE INDEX IF NOT EXISTS idx_late_detections_date ON public.late_detections(processing_date);

-- Append-only: no UPDATE/DELETE is exposed by the application layer.
CREATE TABLE IF NOT EXISTS public.retry_audit_logs
(
    id                 uuid NOT NULL DEFAULT gen_random_uuid(),
    event_type         varchar(30) NOT NULL,
    trigger_type       varchar(10) NOT NULL,
    file_id            uuid,
    file_type          varchar(20) NOT NULL,
    file_checksum      varchar(64),
    processing_date    date NOT NULL,
    initiated_by       varchar(200) NOT NULL,
    outcome            varchar(20),
    duration_ms        integer,
    error_detail       text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT retry_audit_logs_pkey PRIMARY KEY (id)
);

COMMENT ON COLUMN public.retry_audit_logs.event_type IS 'retry_initiated | retry_completed';
COMMENT ON COLUMN public.retry_audit_logs.trigger_type IS 'auto | manual';

CREATE INDEX IF NOT EXISTS idx_retry_audit_date ON public.retry_audit_logs(processing_date);
CREATE INDEX IF NOT EXISTS idx_retry_audit_type ON public.retry_audit_logs(file_type);
CREATE INDEX IF NOT EXISTS idx_retry_audit_trigger ON public.retry_audit_logs(trigger_type);

CREATE TABLE IF NOT EXISTS public.scan_runs
(
    id                 uuid NOT NULL DEFAULT gen_random_uuid(),
    scan_type          varchar(20) NOT NULL,
    started_at         timestamptz NOT NULL,
    finished_at        timestamptz,
    status             varchar(20) NOT NULL DEFAULT 'running',
    files_detected     integer DEFAULT 0,
    error_message      text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scan_runs_pkey PRIMARY KEY (id)
);

COMMENT ON COLUMN public.scan_runs.scan_type IS 'failure_detection | late_detection';
COMMENT ON COLUMN public.scan_runs.status IS 'running | success | failed';

CREATE INDEX IF NOT EXISTS idx_scan_runs_started ON public.scan_runs(started_at DESC);

COMMIT;
