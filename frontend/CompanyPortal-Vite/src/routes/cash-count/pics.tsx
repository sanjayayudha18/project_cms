import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "../_protected";

const CashCountPicsPage = lazyRouteComponent(
  () => import("@/features/cash-count-pics/CashCountPicsPage"),
  "CashCountPicsPage",
);

// Mapping PIC (5.1): CC-CWM is the maker, ADMIN reads -- same as the /cash-count/pics API group.
export const cashCountPicsRoute = createRoute({
  path: "/cash-count/pics",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("cash-count.pic-mapping"),
  loader: () => CashCountPicsPage.preload?.(),
  component: CashCountPicsRouteComponent,
});

function CashCountPicsRouteComponent() {
  const context = cashCountPicsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <CashCountPicsPage />;
}
