/**
 * Which records have a change waiting for approval. Backed by the read-only
 * GET /admin/master-data/changes (status=pending); updates/disables/enables
 * carry the target entity_id. Creates have no id yet, so they cannot be
 * badged on a row -- usePendingCreates exposes their payload instead so a
 * list can show them as "waiting for approval" in its own scope.
 */

import { api } from "@/lib/api/client";
import { useQuery } from "@tanstack/react-query";

type PendingEntityType =
  | "vendor"
  | "atm"
  | "vendor_branch"
  | "vendor_vault"
  | "vendor_pic"
  | "vendor_package"
  | "vendor_package_price"
  | "dsr_location_map";

interface PendingChange {
  id: number;
  entity_id: number | null;
  op: "create" | "update" | "disable" | "enable";
  payload: Record<string, unknown> | null;
}

/** A staged create: the change request id plus the submitted payload. */
export interface PendingCreate {
  id: number;
  payload: Record<string, unknown>;
}

// ponytail: first 100 pending changes per entity type across all vendors;
// add an owner filter on the endpoint if pending queues ever get that long.
function usePendingChanges(entityType: PendingEntityType): PendingChange[] {
  const { data } = useQuery({
    queryKey: ["master-data", "pending", entityType],
    queryFn: async () => {
      const res = await api.get<{ changes: PendingChange[] }>(
        `/admin/master-data/changes?entity_type=${entityType}&status=pending&page=1&page_size=100`,
      );
      return res.data.changes;
    },
  });
  return data ?? [];
}

export function usePendingEntityIds(entityType: PendingEntityType): Set<number> {
  const ids = usePendingChanges(entityType).map((c) => c.entity_id);
  return new Set(ids.filter((id): id is number => id !== null));
}

export function usePendingCreates(entityType: PendingEntityType): PendingCreate[] {
  return usePendingChanges(entityType)
    .filter((c) => c.op === "create" && c.payload !== null)
    .map((c) => ({ id: c.id, payload: c.payload as Record<string, unknown> }));
}
