import { RequirePermission } from "@/features/auth/RequirePermission";
import { OrderDetailPage } from "@/features/orders/OrderDetailPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const orderDetailRoute = createRoute({
  path: "/orders/$id",
  getParentRoute: () => shellRoute,
  component: Guarded,
});

function Guarded() {
  return (
    <RequirePermission permissionKey="vendor-orders">
      <OrderDetailPage />
    </RequirePermission>
  );
}
