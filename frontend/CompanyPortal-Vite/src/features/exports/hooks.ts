import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";
import { downloadExportFile, getExportJob, isApiError, listMyExports, requestExport } from "./api";
import { claimAutoDownload } from "./autoDownload";
import { isActiveStatus } from "./status";
import type { ExportJob, ExportRequestParams } from "./types";
import { useBeforeUnloadGuard } from "./useBeforeUnloadGuard";

export const exportKeys = {
  all: ["exports"] as const,
  mine: (limit?: number) => ["exports", "mine", limit ?? null] as const,
  job: (id: number | null) => ["exports", "job", id] as const,
};

const MY_EXPORTS_POLL_MS = 3000;
const JOB_POLL_MS = 1000;

export function useMyExports(limit?: number) {
  return useQuery({
    queryKey: exportKeys.mine(limit),
    queryFn: () => listMyExports(limit),
    refetchInterval: (q) =>
      q.state.data?.some((j) => isActiveStatus(j.status)) ? MY_EXPORTS_POLL_MS : false,
  });
}

export function useExportJob(id: number | null, opts: { poll?: boolean } = {}) {
  const poll = opts.poll ?? false;
  return useQuery({
    queryKey: exportKeys.job(id),
    queryFn: () => getExportJob(id as number),
    enabled: id !== null,
    refetchInterval: poll ? JOB_POLL_MS : false,
  });
}

export function useRequestExport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: requestExport,
    onSuccess: () => qc.invalidateQueries({ queryKey: [...exportKeys.all, "mine"] }),
  });
}

export type ExportPhase =
  | "idle"
  | "waiting"
  | "downloading"
  | "done"
  | "failed"
  | "download_failed";

export interface ExportDialogProps {
  open: boolean;
  phase: ExportPhase;
  job: ExportJob | null;
  /** Message for `failed` / `download_failed`. */
  message: string | null;
  onClose: () => void;
  onRetry: () => void;
}

const errMessage = (e: unknown) =>
  isApiError(e) || e instanceof Error ? e.message : "Unduhan gagal.";

/**
 * Drives one export from request to local download. A 429 (or any request error) does NOT
 * open the dialog: `request` resolves with `{error}` and it is kept in `requestError`
 * for inline display.
 */
export function useExportFlow() {
  const mutation = useRequestExport();
  const [params, setParams] = useState<ExportRequestParams | null>(null);
  const [jobId, setJobId] = useState<number | null>(null);
  const [phase, setPhase] = useState<ExportPhase>("idle");
  const [message, setMessage] = useState<string | null>(null);
  const [requestError, setRequestError] = useState<unknown>(null);

  const waiting = phase === "waiting";
  const { data: polled } = useExportJob(jobId, { poll: waiting });
  const job = polled ?? mutation.data?.job ?? null;

  useBeforeUnloadGuard(waiting || phase === "downloading");

  const { mutateAsync } = mutation;
  const request = useCallback(
    async (p: ExportRequestParams): Promise<{ job: ExportJob } | { error: unknown }> => {
      setRequestError(null);
      setParams(p);
      try {
        const { job: j } = await mutateAsync(p);
        setJobId(j.id);
        setMessage(null);
        setPhase("waiting");
        return { job: j };
      } catch (e) {
        setRequestError(e);
        return { error: e };
      }
    },
    [mutateAsync],
  );

  useEffect(() => {
    if (!waiting || !polled) return;
    if (polled.status === "failed") {
      setMessage(polled.error_message ?? "Ekspor gagal.");
      setPhase("failed");
    } else if (polled.status === "expired") {
      setMessage("File ekspor sudah kedaluwarsa. Minta ekspor lagi.");
      setPhase("download_failed");
    } else if (polled.status === "downloaded") {
      setPhase("done");
    } else if (polled.status === "succeeded" && claimAutoDownload(polled.id)) {
      setPhase("downloading");
      downloadExportFile(polled.id, polled.file_name).then(
        () => setPhase("done"),
        (e) => {
          setMessage(`${errMessage(e)} Cek menu Ekspor Saya bila file masih tersedia.`);
          setPhase("download_failed");
        },
      );
    }
  }, [waiting, polled]);

  const close = useCallback(() => setPhase("idle"), []);
  const retry = useCallback(() => {
    if (params) void request(params);
  }, [params, request]);

  const dialogProps: ExportDialogProps = {
    open: phase !== "idle",
    phase,
    job,
    message,
    onClose: close,
    onRetry: retry,
  };
  return { request, dialogProps, requestError, isRequesting: mutation.isPending };
}
