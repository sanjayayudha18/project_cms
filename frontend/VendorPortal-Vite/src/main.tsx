import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles/index.css";
import { rootRoute } from "@/routes/__root";
import { authRoute } from "@/routes/_auth";
import { protectedRoute, shellRoute } from "@/routes/_protected";
import { adminRoute } from "@/routes/admin";
import { dashboardRoute } from "@/routes/dashboard";
import { dsrRoute } from "@/routes/dsr";
import { forecastingRoute } from "@/routes/forecasting";
import { indexRoute } from "@/routes/index";
import { invoicesRoute } from "@/routes/invoices";
import { loginRoute } from "@/routes/login";
import { notificationsRoute } from "@/routes/notifications";
import { ordersRoute } from "@/routes/orders";
import { evidenceRoute } from "@/routes/orders.$id.evidence";
import { reconciliationRoute } from "@/routes/reconciliation";
import { scheduleRoute } from "@/routes/schedule";

// ─── Route Tree ───────────────────────────────────────────────────────────────

const routeTree = rootRoute.addChildren([
  authRoute.addChildren([loginRoute]),
  protectedRoute.addChildren([
    shellRoute.addChildren([
      ordersRoute,
      evidenceRoute,
      invoicesRoute,
      scheduleRoute,
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

// ─── Router Instance ──────────────────────────────────────────────────────────

const router = createRouter({ routeTree });

// Register the router for type safety
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

// ─── Render ───────────────────────────────────────────────────────────────────

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}

createRoot(rootElement).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
