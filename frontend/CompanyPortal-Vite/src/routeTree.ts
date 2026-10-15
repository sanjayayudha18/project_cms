// Route tree, kept out of main.tsx so tests can build a router from it (FE-05 T-D1).
import { rootRoute } from "./routes/__root";
import { authRoute } from "./routes/_auth";
import { protectedRoute } from "./routes/_protected";
import { approvalsRoute } from "./routes/approvals";
import { atmPortalRoute } from "./routes/atm-portal";
import { atmProfileRoute } from "./routes/atm-portal.$terminalId";
import { auditLogsRoute } from "./routes/audit-logs";
import { cashCountRoute } from "./routes/cash-count/index";
import { cashFlowRoute } from "./routes/cash-flow";
import { vaultPlanDetailRoute, vaultPlanListRoute } from "./routes/cit/vault-plans";
import { eodMonitoringRoute } from "./routes/eod-monitoring";
import { dmaaForecastRoute } from "./routes/forecasting/dmaa-forecast";
import { forecastRoute } from "./routes/forecasting/forecast";
import { forecastingRoute } from "./routes/forecasting/index";
import { indexRoute } from "./routes/index";
import { invoiceRoute } from "./routes/invoice/index";
import { loginRoute } from "./routes/login";
import { replenishmentRoute } from "./routes/replenishment";
import { forecastBrowserRoute } from "./routes/replenishment/forecast-browser";
import { vendorRequestDetailRoute } from "./routes/replenishment/vendor-requests/$id";
import { vendorRequestListRoute } from "./routes/replenishment/vendor-requests/index";
import { vendorRequestNewRoute } from "./routes/replenishment/vendor-requests/new";
import { settingsRoute } from "./routes/settings";
import { adminAcmAreasRoute } from "./routes/settings/admin/acm-areas";
import { adminATMsRoute } from "./routes/settings/admin/atms";
import { adminHolidaysRoute } from "./routes/settings/admin/holidays";
import { adminMasterDataIORoute } from "./routes/settings/admin/master-data-io";
import { adminRegionsRoute } from "./routes/settings/admin/regions";
import { adminUsersRoute } from "./routes/settings/admin/users";
import { adminVendorBranchDetailRoute } from "./routes/settings/admin/vendor-branch-detail";
import { adminVendorBranchEditRoute } from "./routes/settings/admin/vendor-branch-edit";
import { adminVendorDetailRoute } from "./routes/settings/admin/vendor-detail";
import { adminVendorsRoute } from "./routes/settings/admin/vendors";
import { rbacDelegationsRoute } from "./routes/settings/rbac/delegations";
import { rbacLeavesRoute } from "./routes/settings/rbac/leaves";
import { rbacPoliciesRoute } from "./routes/settings/rbac/policies";
import { rbacUsersRoute } from "./routes/settings/rbac/users";
import { rolesRoute } from "./routes/settings/roles";

// ─── Route Tree ───────────────────────────────────────────────────────────────

export const routeTree = rootRoute.addChildren([
  authRoute.addChildren([loginRoute]),
  protectedRoute.addChildren([
    indexRoute,
    atmPortalRoute,
    atmProfileRoute,
    cashFlowRoute,
    vaultPlanListRoute,
    vaultPlanDetailRoute,
    replenishmentRoute,
    forecastBrowserRoute,
    vendorRequestNewRoute,
    vendorRequestListRoute,
    vendorRequestDetailRoute,
    forecastingRoute,
    forecastRoute,
    dmaaForecastRoute,
    invoiceRoute,
    cashCountRoute,
    eodMonitoringRoute,
    auditLogsRoute,
    settingsRoute,
    adminUsersRoute,
    adminVendorsRoute,
    adminVendorDetailRoute,
    adminVendorBranchDetailRoute,
    adminVendorBranchEditRoute,
    adminMasterDataIORoute,
    adminATMsRoute,
    adminRegionsRoute,
    adminHolidaysRoute,
    adminAcmAreasRoute,
    approvalsRoute,
    rbacUsersRoute,
    rbacDelegationsRoute,
    rbacLeavesRoute,
    rbacPoliciesRoute,
    rolesRoute,
  ]),
]);
