// Route tree, kept out of main.tsx so tests can build a router from it (FE-05 T-D1).
import { rootRoute } from "@/routes/__root";
import { authRoute } from "@/routes/_auth";
import { protectedRoute, shellRoute } from "@/routes/_protected";
import { adminRoute } from "@/routes/admin";
import { dashboardRoute } from "@/routes/dashboard";
import { dsrRoute } from "@/routes/dsr";
import { forecastingRoute } from "@/routes/forecasting";
import { indexRoute } from "@/routes/index";
import { loginRoute } from "@/routes/login";
import { notificationsRoute } from "@/routes/notifications";
import { ordersRoute } from "@/routes/orders";
import { orderDetailRoute } from "@/routes/orders.$id";
import { realizationsRoute } from "@/routes/realizations";
import { reconciliationRoute } from "@/routes/reconciliation";

// ─── Route Tree ───────────────────────────────────────────────────────────────

export const routeTree = rootRoute.addChildren([
  authRoute.addChildren([loginRoute]),
  protectedRoute.addChildren([
    shellRoute.addChildren([
      ordersRoute,
      orderDetailRoute,
      realizationsRoute,
      dsrRoute,
      notificationsRoute,
    ]),
  ]),
  indexRoute,
  dashboardRoute,
  adminRoute,
  forecastingRoute,
  reconciliationRoute,
]);
