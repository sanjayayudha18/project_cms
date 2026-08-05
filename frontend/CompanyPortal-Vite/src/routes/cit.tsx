import { CitTracker } from "@/features/cit";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "./_protected";

export const citRoute = createRoute({
  path: "/cit",
  getParentRoute: () => protectedRoute,
  component: CitTracker,
});
