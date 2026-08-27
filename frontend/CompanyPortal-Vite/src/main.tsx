import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { rootRoute } from "./routes/__root";
import { authRoute } from "./routes/_auth";
import { protectedRoute } from "./routes/_protected";
import { atmPortalRoute } from "./routes/atm-portal";
import { atmProfileRoute } from "./routes/atm-portal.$terminalId";
import { cashCountRoute } from "./routes/cash-count/index";
import { cashFlowRoute } from "./routes/cash-flow";
import { citRoute } from "./routes/cit";
import { eodMonitoringRoute } from "./routes/eod-monitoring";
import { dmaaForecastRoute } from "./routes/forecasting/dmaa-forecast";
import { dsrDashboardRoute } from "./routes/forecasting/dsr-dashboard";
import { dsrUploadRoute } from "./routes/forecasting/dsr-upload";
import { forecastRoute } from "./routes/forecasting/forecast";
import { forecastingRoute } from "./routes/forecasting/index";
import { indexRoute } from "./routes/index";
import { invoiceRoute } from "./routes/invoice/index";
import { invoiceListRoute } from "./routes/invoice/list";
import { reconciliationRoute } from "./routes/invoice/reconciliation";
import { loginRoute } from "./routes/login";
import { replenishmentRoute } from "./routes/replenishment";
import "./styles/index.css";

// ─── Route Tree ───────────────────────────────────────────────────────────────

const routeTree = rootRoute.addChildren([
  authRoute.addChildren([loginRoute]),
  protectedRoute.addChildren([
    indexRoute,
    atmPortalRoute,
    atmProfileRoute,
    cashFlowRoute,
    citRoute,
    replenishmentRoute,
    forecastingRoute,
    forecastRoute,
    dmaaForecastRoute,
    dsrUploadRoute,
    dsrDashboardRoute,
    invoiceRoute,
    invoiceListRoute,
    reconciliationRoute,
    cashCountRoute,
    eodMonitoringRoute,
  ]),
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
