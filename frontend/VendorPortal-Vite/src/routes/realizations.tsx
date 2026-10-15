import { RealizationsPage } from "@/features/realizations/RealizationsPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const realizationsRoute = createRoute({
  path: "/realizations",
  getParentRoute: () => shellRoute,
  component: RealizationsPage,
});
