import { OrdersPage } from "@/features/orders/OrdersPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const ordersRoute = createRoute({
  path: "/orders",
  getParentRoute: () => shellRoute,
  component: OrdersPage,
});
