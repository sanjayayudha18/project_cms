import { RequirePermission } from "@/features/auth/RequirePermission";
import { DsrPage } from "@/features/dsr/DsrPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const dsrRoute = createRoute({
  path: "/dsr",
  getParentRoute: () => shellRoute,
  component: Guarded,
});

function Guarded() {
  return (
    <RequirePermission permissionKey="vendor-dsr">
      <DsrPage />
    </RequirePermission>
  );
}
