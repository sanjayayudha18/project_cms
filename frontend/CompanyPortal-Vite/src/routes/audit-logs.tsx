import { AuditLogPage } from "@/features/audit-log";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";

export const auditLogsRoute = createRoute({
  path: "/audit-logs",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: AuditLogPage,
});
