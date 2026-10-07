import { AuditLogPage } from "@/features/audit-log";
import { AUDIT_LOG_SEARCH_SCHEMA } from "@/features/audit-log/useAuditLogUrlState";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";

export const auditLogsRoute = createRoute({
  path: "/audit-logs",
  getParentRoute: () => protectedRoute,
  validateSearch: AUDIT_LOG_SEARCH_SCHEMA,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AuditLogPage,
});
