import { useAuthStore } from "@/lib/auth";
import { createRoute, redirect } from "@tanstack/react-router";
import { authRoute } from "./_auth";
import { LoginPage } from "./_auth/login";

export const loginRoute = createRoute({
  path: "/login",
  getParentRoute: () => authRoute,
  beforeLoad: () => {
    const { isAuthenticated, isAuthLoading } = useAuthStore.getState();
    if (!isAuthLoading && isAuthenticated) {
      throw redirect({ to: "/" });
    }
  },
  component: LoginPage,
});
