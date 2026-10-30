import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";

const MyExportsPage = lazyRouteComponent(
  () => import("@/features/exports/components/MyExportsPage"),
  "MyExportsPage",
);

/** Union of the DSR-late and replenishment report viewers; ADMIN/ADMIN_PARAM bypass in requireRoles. */
export const EXPORTS_PAGE_ROLES = [
  "ADMIN",
  "ATM-USER",
  "ATM-SPV",
  "BRANCH-ATM-USER",
  "BRANCH-ATM-SPV",
] as const;

export const myExportsRoute = createRoute({
  path: "/exports",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles([...EXPORTS_PAGE_ROLES]),
  loader: () => MyExportsPage.preload?.(),
  component: MyExportsRouteComponent,
});

function MyExportsRouteComponent() {
  const context = myExportsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <MyExportsPage />;
}
