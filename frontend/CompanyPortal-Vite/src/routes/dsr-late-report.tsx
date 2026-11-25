import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "./_protected";

const DsrLateReportScreen = lazyRouteComponent(
  () => import("@/features/dsr-late-report"),
  "DsrLateReportScreen",
);

export const dsrLateReportRoute = createRoute({
  path: "/dsr-late-report",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("monitoring.dsr-late-report"),
  loader: () => DsrLateReportScreen.preload?.(),
  component: DsrLateReportRouteComponent,
});

function DsrLateReportRouteComponent() {
  const context = dsrLateReportRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <DsrLateReportScreen />;
}
