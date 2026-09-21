import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { rootRoute } from "./routes/__root";
import { authRoute } from "./routes/_auth";
import { protectedRoute } from "./routes/_protected";
import { approvalsRoute } from "./routes/approvals";
import { atmPortalRoute } from "./routes/atm-portal";
import { atmProfileRoute } from "./routes/atm-portal.$terminalId";
import { auditLogsRoute } from "./routes/audit-logs";
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
import { forecastBrowserRoute } from "./routes/replenishment/forecast-browser";
import { vendorRequestDetailRoute } from "./routes/replenishment/vendor-requests/$id";
import { vendorRequestListRoute } from "./routes/replenishment/vendor-requests/index";
import { vendorRequestNewRoute } from "./routes/replenishment/vendor-requests/new";
import { settingsRoute } from "./routes/settings";
import { adminATMsRoute } from "./routes/settings/admin/atms";
import { adminMasterDataIORoute } from "./routes/settings/admin/master-data-io";
import { adminUsersRoute } from "./routes/settings/admin/users";
import { adminVendorDetailRoute } from "./routes/settings/admin/vendor-detail";
import { adminVendorsRoute } from "./routes/settings/admin/vendors";
import { rbacDelegationsRoute } from "./routes/settings/rbac/delegations";
import { rbacLeavesRoute } from "./routes/settings/rbac/leaves";
import { rbacPoliciesRoute } from "./routes/settings/rbac/policies";
import { rbacUsersRoute } from "./routes/settings/rbac/users";
import { rolesRoute } from "./routes/settings/roles";
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
    forecastBrowserRoute,
    vendorRequestNewRoute,
    vendorRequestListRoute,
    vendorRequestDetailRoute,
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
    auditLogsRoute,
    settingsRoute,
    adminUsersRoute,
    adminVendorsRoute,
    adminVendorDetailRoute,
    adminMasterDataIORoute,
    adminATMsRoute,
    approvalsRoute,
    rbacUsersRoute,
    rbacDelegationsRoute,
    rbacLeavesRoute,
    rbacPoliciesRoute,
    rolesRoute,
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
