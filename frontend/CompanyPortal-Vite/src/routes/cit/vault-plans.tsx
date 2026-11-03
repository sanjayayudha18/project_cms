import { Forbidden } from "@/components/pages/Forbidden";
import { createRoute, lazyRouteComponent } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

// Page chunks load on navigation; each loader preloads its page so the guard wrapper never suspends (FE-04).
const VaultPlanList = lazyRouteComponent(
  () => import("@/features/vault-plan/VaultPlanList"),
  "VaultPlanList",
);
const VaultPlanDetail = lazyRouteComponent(
  () => import("@/features/vault-plan/VaultPlanDetail"),
  "VaultPlanDetail",
);

// Penetapan vault (cit-acm-plan): same roles as the backend /api/v1/vault-plans guard.
const VAULT_PLAN_ROLES = requireRoles(["ACM-USER", "ACM-SPV", "ADMIN"]);

export const vaultPlanListRoute = createRoute({
  path: "/cit/vault-plans",
  getParentRoute: () => protectedRoute,
  beforeLoad: VAULT_PLAN_ROLES,
  loader: () => VaultPlanList.preload?.(),
  component: function VaultPlanListRoute() {
    const context = vaultPlanListRoute.useRouteContext() as { forbidden?: boolean };
    return context.forbidden ? <Forbidden /> : <VaultPlanList />;
  },
});

// Notification links point here (backend vault_plan_create.go: /cit/vault-plans/{id}).
export const vaultPlanDetailRoute = createRoute({
  path: "/cit/vault-plans/$id",
  getParentRoute: () => protectedRoute,
  beforeLoad: VAULT_PLAN_ROLES,
  loader: () => VaultPlanDetail.preload?.(),
  component: function VaultPlanDetailRoute() {
    const context = vaultPlanDetailRoute.useRouteContext() as { forbidden?: boolean };
    return context.forbidden ? <Forbidden /> : <VaultPlanDetail />;
  },
});
