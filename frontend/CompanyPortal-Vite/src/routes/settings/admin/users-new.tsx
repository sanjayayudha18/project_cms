import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requirePermission } from "../../_protected";

// Page chunk loads on navigation; the loader preloads it so the guard wrapper never suspends (FE-04).
const CreateUserPage = lazyRouteComponent(() => import("@/features/admin-users"), "CreateUserPage");

export const adminUsersNewRoute = createRoute({
  path: "/settings/admin/users/new",
  getParentRoute: () => protectedRoute,
  beforeLoad: requirePermission("settings.users"),
  loader: () => CreateUserPage.preload?.(),
  component: AdminUsersNewRouteComponent,
});

function AdminUsersNewRouteComponent() {
  const context = adminUsersNewRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <CreateUserPage />;
}
