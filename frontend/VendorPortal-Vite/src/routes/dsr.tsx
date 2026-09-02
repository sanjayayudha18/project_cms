import { DsrPage } from "@/features/dsr/DsrPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const dsrRoute = createRoute({
  path: "/dsr",
  getParentRoute: () => shellRoute,
  component: DsrPage,
});
