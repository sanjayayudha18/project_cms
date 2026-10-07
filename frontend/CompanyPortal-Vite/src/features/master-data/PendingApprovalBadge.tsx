import { Badge } from "@/components/ui/Badge";
import { Clock } from "lucide-react";

/** Icon + label, never colour alone (Sec 13). */
export function PendingApprovalBadge() {
  return <Badge variant="warning" icon={Clock} label="Menunggu approval" />;
}
