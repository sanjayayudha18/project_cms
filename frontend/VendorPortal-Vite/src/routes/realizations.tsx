import { RequirePermission } from "@/features/auth/RequirePermission";
import { RealizationsGate } from "@/features/realizations/RealizationsPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const realizationsRoute = createRoute({
  path: "/realizations",
  getParentRoute: () => shellRoute,
  component: Guarded,
});

function Guarded() {
  return (
    <RequirePermission permissionKey="vendor-realizations">
      <RealizationsGate />
    </RequirePermission>
  );
}
