/**
 * Which records have a change waiting for approval. Backed by the read-only
 * GET /admin/master-data/changes (status=pending); updates/disables/enables
 * carry the target entity_id. Creates have no id yet, so they cannot be badged.
 */

import { api } from "@/lib/api/client";
import { useQuery } from "@tanstack/react-query";

interface PendingList {
  changes: { entity_id: number | null }[];
}

export function usePendingEntityIds(
  entityType:
    | "vendor"
    | "atm"
    | "vendor_branch"
    | "vendor_vault"
    | "vendor_pic"
    | "vendor_package"
    | "vendor_package_price",
): Set<number> {
  const { data } = useQuery({
    queryKey: ["master-data", "pending", entityType],
    queryFn: async () => {
      const res = await api.get<PendingList>(
        `/admin/master-data/changes?entity_type=${entityType}&status=pending&page=1&page_size=100`,
      );
      const ids = res.data.changes.map((c) => c.entity_id);
      return ids.filter((id): id is number => id !== null);
    },
  });
  return new Set(data ?? []);
}
