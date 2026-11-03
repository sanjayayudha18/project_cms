/**
 * Active tab of /settings/admin/master-data-io, kept in the `?tab` search param
 * (master-data-import-revamp Req 2.5). Same pattern as
 * admin-vendors/useAdminVendorsUrlState.ts: read with `useSearch({ strict: false })`
 * so the feature never imports the route; the default tab is omitted from the URL.
 */

import { useNavigate, useSearch } from "@tanstack/react-router";
import type { MasterDataIoSearchParams } from "./searchSchema";

export type MasterDataIoTab = NonNullable<MasterDataIoSearchParams["tab"]>;

export function useMasterDataIoTab(): {
  tab: MasterDataIoTab;
  setTab: (t: MasterDataIoTab) => void;
} {
  const raw = useSearch({ strict: false }) as Record<string, unknown>;
  const navigate = useNavigate();
  const tab: MasterDataIoTab = raw.tab === "history" ? "history" : "new";

  function setTab(next: MasterDataIoTab): void {
    // Switching tab drops the history filters and the open batch.
    navigate({ to: ".", search: next === "new" ? {} : { tab: next } });
  }

  return { tab, setTab };
}
