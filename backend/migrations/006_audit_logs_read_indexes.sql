-- audit_logs read-path indexes for the Audit Log Viewer (list ordering + action filter).
-- Re-issue of archived 029 (dropped by the 2026-09-18 baseline squash, which only kept
-- actor_idx/entity_idx).
--
-- WHY: the viewer's list query orders by (created_at DESC, id DESC) for stable
-- pagination and filters by action; neither is covered by the existing indexes.
--
-- SAFETY:
--   * DDL only, additive, no data touched. IF NOT EXISTS: re-runnable.
--   * Built inline (not CONCURRENTLY): audit_logs is small, keeping it atomic is worth more.
--   * Forward-only: no down migration ships (project convention).

BEGIN;

CREATE INDEX IF NOT EXISTS audit_logs_created_at_id_idx
    ON public.audit_logs (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS audit_logs_action_idx
    ON public.audit_logs (action);

COMMIT;
