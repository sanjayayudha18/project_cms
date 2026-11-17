import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

const CashCountParametersPage = lazyRouteComponent(
  () => import("@/features/cash-count-parameters/CashCountParametersPage"),
  "CashCountParametersPage",
);

// Parameter Cash Count (5.1): CC-CWM + CC-CCR are makers, ADMIN reads -- same as the API groups.
export const cashCountParametersRoute = createRoute({
  path: "/cash-count/parameters",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "CC-CWM", "CC-CCR"]),
  loader: () => CashCountParametersPage.preload?.(),
  component: CashCountParametersRouteComponent,
});

function CashCountParametersRouteComponent() {
  const context = cashCountParametersRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <CashCountParametersPage />;
}
