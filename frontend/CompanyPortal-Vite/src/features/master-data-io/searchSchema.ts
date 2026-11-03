/**
 * URL search params of /settings/admin/master-data-io (master-data-import-revamp Req 2.5).
 * Kept apart from the feature index so the route imports it without pulling the lazy
 * page chunk (FE-04); same pattern as admin-vendors/useAdminVendorsUrlState.ts.
 */

import { z } from "zod";
import { IO_ENTITIES, type IoEntity } from "./api";
import { BATCH_STATUS_CODES } from "./batchStatus";

export const IO_ENTITY_IDS = IO_ENTITIES.map((e) => e.id) as [IoEntity, ...IoEntity[]];

export const MASTER_DATA_IO_SEARCH_SCHEMA = z.object({
  tab: z.enum(["new", "history"]).optional(),
  batch: z.number().int().positive().optional(),
  h_entity: z.enum(IO_ENTITY_IDS).optional(),
  h_status: z.enum(BATCH_STATUS_CODES).optional(),
  h_mine: z.boolean().optional(),
  h_page: z.number().int().min(1).optional(),
});

export type MasterDataIoSearchParams = z.infer<typeof MASTER_DATA_IO_SEARCH_SCHEMA>;
