import { createRoute, redirect } from "@tanstack/react-router";
import { protectedRoute } from "../../_protected";

/** Old location of the export screen; kept so bookmarks land on the `/exports` menu. */
export const adminExportsRoute = createRoute({
  path: "/settings/admin/exports",
  getParentRoute: () => protectedRoute,
  beforeLoad: () => {
    throw redirect({ to: "/exports" });
  },
});
