-- audit_logs read-path indexes for the Audit Log Viewer (list ordering + action filter).
--
-- WHY: 023_audit_logs.sql (RBAC-Setup) already created audit_logs with actor_idx and
-- entity_idx. The viewer's list query orders by (created_at DESC, id DESC) for stable
-- pagination and filters by action; neither is covered by the existing indexes.
--
-- SAFETY:
--   * DDL only, additive, no data touched.
--   * IF NOT EXISTS: re-runnable, and a no-op if another spec already added these.
--   * Built inline (not CONCURRENTLY): audit_logs is a new, low-row-count table, so the
--     lock window is negligible and keeping the migration atomic is worth more than
--     avoiding it. Revisit (CONCURRENTLY, drop BEGIN/COMMIT) if the table grows large
--     before this runs.

BEGIN;

CREATE INDEX IF NOT EXISTS audit_logs_created_at_id_idx
    ON public.audit_logs (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS audit_logs_action_idx
    ON public.audit_logs (action);

COMMIT;
