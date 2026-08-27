import { Forbidden } from "@/components/pages/Forbidden";
import { EodMonitoringPage } from "@/features/eod-monitoring";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";

export const eodMonitoringRoute = createRoute({
  path: "/eod-monitoring",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: EodMonitoringRouteComponent,
});

function EodMonitoringRouteComponent() {
  const context = eodMonitoringRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <EodMonitoringPage />;
}
