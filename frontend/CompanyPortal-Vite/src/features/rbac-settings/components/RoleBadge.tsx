/**
 * Role badge for the RBAC settings menu's user hierarchy table (Requirement
 * 9.2: role rendered as a badge pairing label with icon). Wraps the existing
 * Badge component; variant comes from ROLE_BADGE_VARIANT (semantic
 * info/neutral/warning tokens only, no brand red -- red is reserved for
 * primary actions per the Merah Sirih theme).
 */

import { Badge } from "@/components/ui/Badge";
import {
  Building2,
  KeyRound,
  type LucideIcon,
  Settings2,
  ShieldCheck,
  Truck,
  User,
  UserCog,
} from "lucide-react";
import { ROLE_BADGE_VARIANT } from "../types";

const ROLE_ICON: Record<string, LucideIcon> = {
  ADMIN: ShieldCheck,
  ADMIN_PARAM: Settings2,
  APPACCESS: KeyRound,
  "ATM-USER": User,
  "ATM-SPV": UserCog,
  "BRANCH-USER": Building2,
  "BRANCH-SPV": Building2,
  "BRANCH-ATM-USER": Building2,
  "BRANCH-ATM-SPV": Building2,
  "VENDOR-USER": Truck,
};

interface RoleBadgeProps {
  role: string;
}

export function RoleBadge({ role }: RoleBadgeProps) {
  return (
    <Badge
      variant={ROLE_BADGE_VARIANT[role] ?? "neutral"}
      icon={ROLE_ICON[role] ?? User}
      label={role}
    />
  );
}
