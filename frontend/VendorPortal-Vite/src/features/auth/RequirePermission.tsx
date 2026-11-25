import { NoAccess } from "@/components/NoAccess";
import { useAuth } from "@/features/auth/useAuth";
import { NAV_ITEMS } from "@/lib/constants";
import { Navigate } from "@tanstack/react-router";
import type { ReactNode } from "react";

interface RequirePermissionProps {
  /** Catalog key (menu_features) the route needs, e.g. "vendor-orders". */
  readonly permissionKey: string;
  readonly children: ReactNode;
}

/**
 * Route guard (spec role-management-revamp 8.2). Auth lives in React context, not router
 * context, so this wraps the route component instead of using beforeLoad. Without the key:
 * redirect to the first nav item the user holds, else show NoAccess. The backend still
 * enforces the same catalog on every call.
 */
export function RequirePermission({ permissionKey, children }: RequirePermissionProps) {
  const { state } = useAuth();
  const permissions = state.user?.permissions ?? [];
  if (permissions.includes(permissionKey)) return <>{children}</>;

  const fallback = NAV_ITEMS.find(
    (item) => !item.hiddenFromNav && item.key && permissions.includes(item.key),
  );
  if (fallback) return <Navigate to={fallback.path} replace />;
  return <NoAccess />;
}
