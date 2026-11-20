import type { components, operations } from "@/lib/api/schema";

export type PksListItem = components["schemas"]["PksListItem"];
export type PksListPage = components["schemas"]["PksListPage"];
export type PksRecordStatus = PksListItem["status"];
export type SubmitPksPayload = components["schemas"]["SubmitVendorPksRequest"];
export type PksReasonPayload = components["schemas"]["VendorPksReasonRequest"];
export type UpdatePksReminderResponse = components["schemas"]["UpdatePksReminderResponse"];

/** Filter status of the list endpoint (server default = "terkini"). */
export type PksListStatus = NonNullable<operations["listPks"]["parameters"]["query"]>["status"];

export interface PksListFilter {
  status?: PksListStatus;
  pks_type?: string;
  vendor?: string;
  q?: string;
  page?: number;
  page_size?: number;
}
