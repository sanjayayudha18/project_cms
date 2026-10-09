import { OrderDetailPage } from "@/features/orders/OrderDetailPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const orderDetailRoute = createRoute({
  path: "/orders/$id",
  getParentRoute: () => shellRoute,
  component: OrderDetailPage,
});
