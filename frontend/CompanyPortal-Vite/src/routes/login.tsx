import { useAuthStore } from "@/lib/auth";
import { createRoute, lazyRouteComponent, redirect } from "@tanstack/react-router";
import { authRoute } from "./_auth";

export const loginRoute = createRoute({
  path: "/login",
  getParentRoute: () => authRoute,
  beforeLoad: () => {
    const { isAuthenticated, isAuthLoading } = useAuthStore.getState();
    if (!isAuthLoading && isAuthenticated) {
      throw redirect({ to: "/" });
    }
  },
  component: lazyRouteComponent(() => import("./_auth/login"), "LoginPage"),
});
