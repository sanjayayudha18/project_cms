/**
 * Master-data writes are maker-checker: create/update/disable/enable do NOT
 * change the record, they stage a change request and answer 202
 * (backend changeRequestAcceptedResponse, plan.md D1). The row only changes
 * once an approver approves it.
 */

export interface ChangeRequestAccepted {
  change_request_id: number;
  status: string;
  entity_type: string;
  op: "create" | "update" | "disable" | "enable";
  /** Vendor disable only: the vendor still has linked active users. */
  warning?: string;
  linked_active_users?: number;
}

/** Toast text for a staged change: says "submitted", never "saved". */
export function pendingApprovalMessage(
  res: Pick<ChangeRequestAccepted, "change_request_id">,
): string {
  return `Perubahan diajukan dan menunggu persetujuan (#${res.change_request_id})`;
}
