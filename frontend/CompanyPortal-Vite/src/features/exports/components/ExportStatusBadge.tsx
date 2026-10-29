import { Badge } from "@/components/ui/Badge";
import { STATUS_META } from "../status";
import type { ExportStatus } from "../types";

/** Status as icon + text (never colour alone); tones come from Badge, not brand red. */
export function ExportStatusBadge({ status }: { status: ExportStatus }) {
  const meta = STATUS_META[status];
  return <Badge variant={meta.tone} icon={meta.icon} label={meta.label} />;
}
