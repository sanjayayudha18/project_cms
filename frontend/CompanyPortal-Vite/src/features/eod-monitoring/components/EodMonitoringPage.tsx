import { PageHeader } from "@/components/ui/PageHeader";
import { useState } from "react";
import { useEodSummary } from "../hooks/useEodQueries";
import type { FileStatusRow } from "../types";
import { getTodayWib } from "../utils";
import { AuditLogSection } from "./AuditLogSection";
import { FileStatusSection } from "./FileStatusSection";
import { LastUpdatedIndicator } from "./LastUpdatedIndicator";
import { LateAlertsSection } from "./LateAlertsSection";
import { PollingToggle } from "./PollingToggle";
import { ProcessingDatePicker } from "./ProcessingDatePicker";
import { RetryConfirmationDialog } from "./RetryConfirmationDialog";
import { RetryDrawer } from "./RetryDrawer";
import { SummarySection } from "./SummarySection";

const POLLING_INTERVAL_MS = 60_000;

/** Page orchestrator for the EOD Monitoring admin screen. */
export function EodMonitoringPage() {
  const [processingDate, setProcessingDate] = useState(getTodayWib);
  const [pollingEnabled, setPollingEnabled] = useState(true);
  const [selectedFile, setSelectedFile] = useState<FileStatusRow | null>(null);
  const [retryDialogFile, setRetryDialogFile] = useState<FileStatusRow | null>(null);

  const refetchInterval = pollingEnabled ? POLLING_INTERVAL_MS : false;

  // ponytail: reuses the SummarySection's own query (same key, deduped by TanStack Query)
  // as a proxy "last updated" signal instead of lifting fetch state out of every section.
  const { dataUpdatedAt } = useEodSummary(processingDate, refetchInterval);

  return (
    <main className="py-6 max-[759px]:py-4">
      <PageHeader
        eyebrow="Admin"
        title="EOD Monitoring"
        description="Pantau status pemrosesan file ETL End-of-Day, riwayat retry, keterlambatan SLA, dan log audit."
        actions={
          <>
            <ProcessingDatePicker value={processingDate} onChange={setProcessingDate} />
            <PollingToggle enabled={pollingEnabled} onChange={setPollingEnabled} />
            <LastUpdatedIndicator updatedAt={dataUpdatedAt || null} />
          </>
        }
      />

      <div className="flex flex-col gap-8">
        <SummarySection processingDate={processingDate} refetchInterval={refetchInterval} />
        <FileStatusSection
          processingDate={processingDate}
          refetchInterval={refetchInterval}
          onFileSelect={setSelectedFile}
        />
        <LateAlertsSection processingDate={processingDate} refetchInterval={refetchInterval} />
        <AuditLogSection processingDate={processingDate} refetchInterval={refetchInterval} />
      </div>

      <RetryDrawer
        file={selectedFile}
        onClose={() => setSelectedFile(null)}
        onRetryClick={setRetryDialogFile}
      />
      <RetryConfirmationDialog file={retryDialogFile} onClose={() => setRetryDialogFile(null)} />
    </main>
  );
}
