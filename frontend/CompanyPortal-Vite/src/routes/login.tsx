import { createRoute } from "@tanstack/react-router";
import { authRoute } from "./_auth";
import { LoginPage } from "./_auth/login";

export const loginRoute = createRoute({
  path: "/login",
  getParentRoute: () => authRoute,
  component: LoginPage,
});
