import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "./_protected";

/**
 * Approval inbox. No role guard on purpose: approver authority comes from the
 * approval hierarchy, not from a role, so any signed-in user may open it (it is
 * simply empty for someone with no pending steps). The API enforces who can act.
 */
export const approvalsRoute = createRoute({
  path: "/approvals",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("approvals"),
  component: lazyRouteComponent(() => import("@/features/approvals"), "ApprovalInboxPage"),
});
