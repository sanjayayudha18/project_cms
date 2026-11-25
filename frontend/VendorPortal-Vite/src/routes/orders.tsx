import { RequirePermission } from "@/features/auth/RequirePermission";
import { OrdersPage } from "@/features/orders/OrdersPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const ordersRoute = createRoute({
  path: "/orders",
  getParentRoute: () => shellRoute,
  component: Guarded,
});

function Guarded() {
  return (
    <RequirePermission permissionKey="vendor-orders">
      <OrdersPage />
    </RequirePermission>
  );
}
