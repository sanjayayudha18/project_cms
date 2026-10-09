import { Forbidden } from "@/components/pages/Forbidden";
import { VaultPlanDetail } from "@/features/vault-plan/VaultPlanDetail";
import { VaultPlanList } from "@/features/vault-plan/VaultPlanList";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "../_protected";

// Penetapan vault (cit-acm-plan): same roles as the backend /api/v1/vault-plans guard.
const VAULT_PLAN_ROLES = requireRoles(["ACM-USER", "ACM-SPV", "ADMIN"]);

export const vaultPlanListRoute = createRoute({
  path: "/cit/vault-plans",
  getParentRoute: () => protectedRoute,
  beforeLoad: VAULT_PLAN_ROLES,
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
  component: function VaultPlanDetailRoute() {
    const context = vaultPlanDetailRoute.useRouteContext() as { forbidden?: boolean };
    return context.forbidden ? <Forbidden /> : <VaultPlanDetail />;
  },
});
