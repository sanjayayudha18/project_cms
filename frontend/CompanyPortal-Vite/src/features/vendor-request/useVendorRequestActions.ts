/**
 * State, mutations and handlers behind the Vendor Request detail actions, plus
 * the state×role×creator gates mirroring the backend's checkActor
 * (vendor_request.go) — including its union cancel rule (creator OR
 * non-creator checker may cancel a pending request).
 */

import { useToast } from "@/lib/hooks/useToast";
import { useState } from "react";
import { toCompletionPayload } from "./CompletionPanels";
import { REJECTION_REASON_MAX } from "./VendorRequestDetailParts";
import {
  getVendorRequestErrorMessage,
  useApproveVendorRequest,
  useApproveVendorRequestCompletion,
  useCancelVendorRequest,
  useRejectVendorRequest,
  useRejectVendorRequestCompletion,
  useReviseVendorRequest,
  useSubmitVendorRequest,
  useSubmitVendorRequestCompletion,
  useUpdateVendorRequestItems,
} from "./hooks";
import type {
  CompletionResult,
  VendorRequestDetail as VendorRequestDetailType,
  VendorRequestItem,
} from "./types";
import { useVendorReturn } from "./vendorSide";

export const MAKER_ROLES = ["ADMIN", "ATM-USER", "BRANCH-ATM-USER"];
export const CHECKER_ROLES = ["ADMIN", "ATM-SPV", "BRANCH-ATM-SPV"];
const ERROR_TOAST_MS = 5000;
const OVER_QUOTA_TOAST_MS = 8000;
const AMOUNT_MIN = 1;
// atm-visit-quota: the per-ATM completion/kuota table is shown once a request
// can be (or has been) reported as replenished.
export const COMPLETION_STATUSES = [
  "approved",
  "ready",
  "vendor_accepted",
  "completion_pending",
  "completed",
];
// cit-acm-plan FR5.5: after approval, cancel is checker-only in every post-approve state.
const CHECKER_CANCEL_STATUSES = [
  "approved",
  "vault_assignment",
  "vault_review",
  "ready",
  // cit-send-vendor FR6.4
  "sent_to_vendor",
  "vendor_accepted",
  "vendor_rejected",
];

export interface ActionUser {
  id: number;
  role?: string;
}

export function getRequestPermissions(
  data: VendorRequestDetailType,
  user: ActionUser | null | undefined,
) {
  const role = user?.role;
  const isCreator = user !== null && user !== undefined && data.created_by.id === user.id;
  const isMaker = role ? MAKER_ROLES.includes(role) : false;
  const isChecker = role ? CHECKER_ROLES.includes(role) : false;

  const canEditItems = data.status === "draft" && isCreator && isMaker;
  const canSubmit = data.status === "draft" && isCreator && isMaker;
  const canApproveReject = data.status === "pending_approval" && isChecker && !isCreator;
  const canRevise = data.status === "rejected" && isCreator && isMaker;
  // CIT-2 (Task 13.3, Req 5.3): union rule for draft/pending_approval —
  // creator OR a non-creator checker may cancel. replenishment-request-
  // enhancements (Req 3.1, 3.7) adds a second, stricter arm: an approved
  // request is cancelable too, but Checker-only (no four-eyes here — direct
  // cancellation, not an approve/reject pair — so a Checker who is also the
  // creator is still allowed on this arm, mirroring the backend's checkActor).
  const canCancel =
    ((isCreator || (isChecker && !isCreator)) &&
      (data.status === "draft" || data.status === "pending_approval")) ||
    (isChecker && CHECKER_CANCEL_STATUSES.includes(data.status));
  // atm-visit-quota FR1: maker reports a legacy request (ready / approved, never
  // sent to a vendor) as replenished; a checker other than the reporter
  // approves/rejects that report. A vendor_accepted request is reported by its
  // replenish vendor in VendorPortal (replenishment-realisasi FR2.5; the API
  // answers 409 completion_by_vendor to an internal report).
  const canReportCompletion =
    (data.status === "ready" || data.status === "approved") && isMaker && !data.is_canceled;
  const awaitsVendorReport = data.status === "vendor_accepted" && !data.is_canceled;
  const canVendorReturn =
    data.status === "vendor_rejected" && isChecker && !isCreator && !data.is_canceled;
  const isReporter =
    user !== null && user !== undefined && data.completion_submitted_by?.id === user.id;
  const canReviewCompletion = data.status === "completion_pending" && isChecker && !isReporter;
  const showCompletionTable = COMPLETION_STATUSES.includes(data.status) && data.atms.length > 0;

  return {
    isChecker,
    canEditItems,
    canSubmit,
    canApproveReject,
    canRevise,
    canCancel,
    canReportCompletion,
    awaitsVendorReport,
    canVendorReturn,
    canReviewCompletion,
    showCompletionTable,
  };
}

/** Open flag + reason text + 1–500 validity for one reason dialog. */
function useReasonDialog() {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const trimmed = reason.trim().length;
  return {
    open,
    reason,
    setOpen,
    setReason,
    valid: trimmed >= 1 && trimmed <= REJECTION_REASON_MAX,
    close: () => {
      setOpen(false);
      setReason("");
    },
  };
}

export function useVendorRequestActions(data: VendorRequestDetailType) {
  const { toast, dismiss } = useToast();

  const [editingItems, setEditingItems] = useState<VendorRequestItem[] | null>(null);
  const reject = useReasonDialog();
  const cancel = useReasonDialog();
  // cit-send-vendor FR6.1: "Kembalikan ke pembuat" after a replenish branch rejects.
  const vendorReturn = useReasonDialog();
  // atm-visit-quota: laporan selesai dialogs.
  const rejectCompletion = useReasonDialog();
  const [completeOpen, setCompleteOpen] = useState(false);
  const [completeResults, setCompleteResults] = useState<Record<string, CompletionResult>>({});
  const [approveCompletionOpen, setApproveCompletionOpen] = useState(false);

  const updateItemsMutation = useUpdateVendorRequestItems();
  const submitMutation = useSubmitVendorRequest();
  const approveMutation = useApproveVendorRequest();
  const rejectMutation = useRejectVendorRequest();
  const reviseMutation = useReviseVendorRequest();
  const cancelMutation = useCancelVendorRequest();
  const submitCompletionMutation = useSubmitVendorRequestCompletion();
  const approveCompletionMutation = useApproveVendorRequestCompletion();
  const rejectCompletionMutation = useRejectVendorRequestCompletion();
  const vendorReturnMutation = useVendorReturn();

  const isBusy =
    updateItemsMutation.isPending ||
    submitMutation.isPending ||
    approveMutation.isPending ||
    rejectMutation.isPending ||
    reviseMutation.isPending ||
    cancelMutation.isPending ||
    submitCompletionMutation.isPending ||
    approveCompletionMutation.isPending ||
    rejectCompletionMutation.isPending ||
    vendorReturnMutation.isPending;

  function showErrorToast(message: string): void {
    const toastId = toast({ type: "error", message });
    setTimeout(() => dismiss(toastId), ERROR_TOAST_MS);
  }

  async function runAction(action: () => Promise<unknown>): Promise<void> {
    try {
      await action();
    } catch (err) {
      showErrorToast(getVendorRequestErrorMessage(err));
    }
  }

  function startEditItems(): void {
    setEditingItems(data.items.map((it) => ({ ...it })));
  }

  function updateEditedAmount(itemId: number, value: number): void {
    setEditingItems((cur) =>
      cur ? cur.map((it) => (it.id === itemId ? { ...it, amount_replenish: value } : it)) : cur,
    );
  }

  async function saveEditedItems(): Promise<void> {
    if (!editingItems) return;
    await runAction(async () => {
      await updateItemsMutation.mutateAsync({
        id: data.id,
        items: editingItems.map((it) => ({
          terminal_id: it.terminal_id,
          periode_pred: it.periode_pred,
          denom: it.denom,
          amount_replenish: it.amount_replenish,
        })),
      });
      setEditingItems(null);
    });
  }

  /** Runs a reason-gated mutation: nothing fires unless the reason is 1–500 chars. */
  async function submitWithReason(
    dialog: ReturnType<typeof useReasonDialog>,
    mutate: (reason: string) => Promise<unknown>,
  ): Promise<void> {
    const reason = dialog.reason.trim();
    if (reason.length < 1 || reason.length > REJECTION_REASON_MAX) return;
    await runAction(async () => {
      await mutate(reason);
      dialog.close();
    });
  }

  const handleReject = () =>
    submitWithReason(reject, (reason) => rejectMutation.mutateAsync({ id: data.id, reason }));
  const handleVendorReturn = () =>
    submitWithReason(vendorReturn, (reason) =>
      vendorReturnMutation.mutateAsync({ id: data.id, reason }),
    );
  // replenishment-request-enhancements (Req 3.2, 3.3): same 1-500
  // non-whitespace bound as reject, required before the mutation fires.
  const handleCancel = () =>
    submitWithReason(cancel, (reason) => cancelMutation.mutateAsync({ id: data.id, reason }));
  const handleRejectCompletion = () =>
    submitWithReason(rejectCompletion, (reason) =>
      rejectCompletionMutation.mutateAsync({ id: data.id, reason }),
    );

  async function handleSubmitCompletion(): Promise<void> {
    await runAction(async () => {
      await submitCompletionMutation.mutateAsync({
        id: data.id,
        results: toCompletionPayload(data.atms, completeResults),
      });
      setCompleteOpen(false);
      setCompleteResults({});
    });
  }

  async function handleApproveCompletion(): Promise<void> {
    await runAction(async () => {
      const result = await approveCompletionMutation.mutateAsync(data.id);
      setApproveCompletionOpen(false);
      const over = result.over_quota_terminals ?? [];
      if (over.length > 0) {
        const toastId = toast({
          type: "warning",
          message: `Laporan disetujui. ${over.length} ATM melebihi kuota kunjungan: ${over.join(", ")}`,
        });
        setTimeout(() => dismiss(toastId), OVER_QUOTA_TOAST_MS);
      }
    });
  }

  function dismissComplete(): void {
    setCompleteOpen(false);
    setCompleteResults({});
  }

  const editValid =
    editingItems?.every(
      (it) => Number.isInteger(it.amount_replenish) && it.amount_replenish >= AMOUNT_MIN,
    ) ?? false;

  return {
    isBusy,
    editingItems,
    setEditingItems,
    editValid,
    startEditItems,
    updateEditedAmount,
    saveEditedItems,
    reject,
    cancel,
    vendorReturn,
    rejectCompletion,
    completeOpen,
    setCompleteOpen,
    completeResults,
    setCompleteResults,
    dismissComplete,
    approveCompletionOpen,
    setApproveCompletionOpen,
    handleReject,
    handleVendorReturn,
    handleCancel,
    handleRejectCompletion,
    handleSubmitCompletion,
    handleApproveCompletion,
    submit: () => runAction(() => submitMutation.mutateAsync(data.id)),
    approve: () => runAction(() => approveMutation.mutateAsync(data.id)),
    revise: () => runAction(() => reviseMutation.mutateAsync(data.id)),
    pending: {
      updateItems: updateItemsMutation.isPending,
      submit: submitMutation.isPending,
      approve: approveMutation.isPending,
      revise: reviseMutation.isPending,
      reject: rejectMutation.isPending,
      cancel: cancelMutation.isPending,
      vendorReturn: vendorReturnMutation.isPending,
      submitCompletion: submitCompletionMutation.isPending,
      approveCompletion: approveCompletionMutation.isPending,
      rejectCompletion: rejectCompletionMutation.isPending,
    },
  };
}

export type VendorRequestActionsState = ReturnType<typeof useVendorRequestActions>;
export type RequestPermissions = ReturnType<typeof getRequestPermissions>;
