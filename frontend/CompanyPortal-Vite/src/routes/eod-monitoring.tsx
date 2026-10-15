import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const EodMonitoringPage = lazyRouteComponent(
  () => import("@/features/eod-monitoring"),
  "EodMonitoringPage",
);

export const eodMonitoringRoute = createRoute({
  path: "/eod-monitoring",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  loader: () => EodMonitoringPage.preload?.(),
  component: EodMonitoringRouteComponent,
});

function EodMonitoringRouteComponent() {
  const context = eodMonitoringRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <EodMonitoringPage />;
}
