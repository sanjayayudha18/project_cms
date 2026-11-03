# Design Document: EOD Monitoring Frontend

## Overview

The EOD Monitoring Frontend is a single-page admin screen within `frontend/CompanyPortal-Vite/` that provides real-time visibility into the End-of-Day ETL file processing pipeline. It surfaces processing summaries, per-file status with retry history, SLA breach alerts, and an audit trail — all consumed from the Go backend proxy (`/api/eod/*`) to the Python EOD Retry Scheduler microservice.

The page is a scrollable layout with four section cards, supports 60-second polling via TanStack Query's `refetchInterval`, and allows manual retry via a confirmation dialog. Access is restricted to `ADMIN` and `ADMIN_PARAM` roles.

---

## Architecture

### Component Tree

```
_protected.tsx (route layout, auth guard)
└── eod-monitoring.tsx (route file)
    └── EodMonitoringPage
        ├── PageHeader (shared, with actions slot)
        │   ├── ProcessingDatePicker
        │   ├── PollingToggle
        │   └── LastUpdatedIndicator
        ├── SummarySection
        │   └── SummaryCard × 6 (shared)
        ├── FileStatusSection
        │   ├── DataTable (shared, with sortable columns)
        │   └── Badge (shared, status mapping)
        ├── LateAlertsSection
        │   ├── LateAlertCard (feature-specific)
        │   └── Badge (shared)
        ├── AuditLogSection
        │   ├── FilterSelect × 2 (shared)
        │   └── DataTable (shared)
        ├── RetryDrawer (feature-specific)
        │   ├── RetryAttemptList
        │   └── Button (shared)
        └── RetryConfirmationDialog (feature-specific)
            ├── Button × 2 (shared)
            └── Loader
```

### Data Flow

```
Processing_Date_Picker ──► React State (processingDate)
                                │
        ┌───────────────────────┼───────────────────────────┐
        │                       │                           │
        ▼                       ▼                           ▼
useEodSummary()        useEodStatus()              useEodLate()
        │                       │                           │
        │              useEodAudit(filters)                 │
        │                       │                           │
        ▼                       ▼                           ▼
  SummarySection        FileStatusSection          LateAlertsSection
                                │                           
                                │ row click                  
                                ▼                           
                      RetryDrawer (file_id)
                                │
                      useFileHistory(file_id)
                                │
                                │ "Retry Manual" click
                                ▼
                      RetryConfirmationDialog
                                │
                      useRetryFile (mutation)
                                │ onSuccess
                                ▼
                      invalidateQueries(['eod'])
```

### Polling Architecture

All TanStack Query hooks accept a shared `refetchInterval` value derived from a `usePollingToggle` hook. When polling is enabled, all queries refetch every 60 seconds. When disabled, `refetchInterval` is set to `false`.

```typescript
// Polling state lives in a simple React state hook, not global store.
// It only affects this page's queries.
const [pollingEnabled, setPollingEnabled] = useState(true);
const refetchInterval = pollingEnabled ? 60_000 : false;
```

---

## Components

### Feature Module Structure

```
src/features/eod-monitoring/
├── index.ts                        # Public barrel export
├── types.ts                        # API response types, enums, mappings
├── hooks/
│   └── useEodQueries.ts            # All TanStack Query hooks + mutation
├── components/
│   ├── EodMonitoringPage.tsx        # Page orchestrator component
│   ├── SummarySection.tsx           # Summary cards grid
│   ├── FileStatusSection.tsx        # File status DataTable
│   ├── LateAlertsSection.tsx        # Late detection cards
│   ├── AuditLogSection.tsx          # Audit log with filters
│   ├── RetryDrawer.tsx              # Side drawer for retry history
│   ├── RetryConfirmationDialog.tsx  # Modal confirmation before retry
│   ├── PollingToggle.tsx            # Toggle control for auto-refresh
│   ├── ProcessingDatePicker.tsx     # Date input control
│   ├── LastUpdatedIndicator.tsx     # "Last updated" timestamp display
│   └── SectionErrorState.tsx        # Inline error with retry button
└── __tests__/
    ├── EodMonitoringPage.test.tsx
    ├── hooks.test.ts
    └── mappings.property.test.ts
```

### Route Registration

```typescript
// src/routes/eod-monitoring.tsx
import { createRoute } from "@tanstack/react-router";
import { protectedRoute, requireRoles } from "./_protected";
import { EodMonitoringPage } from "@/features/eod-monitoring";

export const eodMonitoringRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/eod-monitoring",
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  component: EodMonitoringPage,
});
```

### Component Responsibilities

| Component | Responsibility |
|-----------|---------------|
| `EodMonitoringPage` | Page orchestrator: manages `processingDate` state, polling toggle state, passes `refetchInterval` to all sections. Renders PageHeader with actions slot containing date picker, toggle, and last-updated indicator. |
| `SummarySection` | Fetches summary data, renders 6 SummaryCards in a responsive grid (3/2/1 columns). Handles loading skeleton and error state independently. |
| `FileStatusSection` | Fetches file status data, renders DataTable with sorting. Row click handler sets `selectedFileId` to open RetryDrawer. |
| `LateAlertsSection` | Fetches late detection data, renders LateAlertCard list with resolution status badges. |
| `AuditLogSection` | Manages local filter state (file_type, trigger), fetches audit data with filters, renders DataTable. |
| `RetryDrawer` | Slides in from right on `selectedFileId` change. Fetches file history. Renders metadata + attempt list. Houses "Retry Manual" button. |
| `RetryConfirmationDialog` | Modal confirmation before POST. Shows file metadata + warning. Handles mutation loading/error states. |
| `PollingToggle` | Simple toggle switch rendering polling state. Calls parent's `setPollingEnabled`. |
| `ProcessingDatePicker` | Date input defaulting to current WIB date. Calls parent's `setProcessingDate`. |
| `LastUpdatedIndicator` | Displays most recent `dataUpdatedAt` from any query, formatted in WIB. |
| `SectionErrorState` | Inline error display with "Coba Lagi" button that calls `refetch()` on the section's query. |

---

## Interfaces

### TypeScript Types (`types.ts`)

```typescript
// ─── API Response Envelope ────────────────────────────────────────────────────

export interface ApiEnvelope<T> {
  status: "success" | "error";
  data: T | null;
  error: string | null;
}

// ─── Summary ──────────────────────────────────────────────────────────────────

export interface EodSummary {
  processing_date: string;
  counts: SummaryCounts;
  by_file_type: Record<FileType, Partial<SummaryCounts>>;
}

export interface SummaryCounts {
  pending: number;
  processing: number;
  completed: number;
  failed: number;
  max_retries_exhausted: number;
  late: number;
}

// ─── File Status ──────────────────────────────────────────────────────────────

export type FileType = "dmaa" | "itm_cashpos" | "itm_replenish";

export type ProcessingStatus =
  | "pending"
  | "processing"
  | "completed"
  | "failed"
  | "max_retries_exhausted";

export interface FileStatusItem {
  file_id: string;
  filename: string;
  checksum: string;
  processing_status: ProcessingStatus;
  retry_count: number;
  max_retries_exhausted: boolean;
  detected_at: string; // ISO 8601
  last_retry_at: string | null;
  failure_reason: string | null;
}

export interface FileStatusResponse {
  processing_date: string;
  by_file_type: Record<FileType, FileStatusItem[]>;
}

// ─── File History ─────────────────────────────────────────────────────────────

export type TriggerType = "auto" | "manual";

export interface RetryAttempt {
  attempt_number: number;
  trigger: TriggerType;
  started_at: string;
  completed_at: string | null;
  outcome: "completed" | "failed" | null;
  duration_ms: number | null;
  error_detail: string | null;
}

export interface FileHistoryResponse {
  file_id: string;
  filename: string;
  file_type: FileType;
  attempts: RetryAttempt[];
}

// ─── Manual Retry ─────────────────────────────────────────────────────────────

export interface ManualRetryResponse {
  job_id: string;
  file_id: string;
  processing_status: ProcessingStatus;
  triggered_by: string;
}

// ─── Late Detection ───────────────────────────────────────────────────────────

export interface LateDetectionItem {
  id: string;
  file_type: FileType;
  processing_date: string;
  sla_deadline: string; // "HH:mm:ss" or "HH:mm"
  detected_at: string;
  resolved_at: string | null;
  is_resolved: boolean;
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

export interface AuditLogEntry {
  id: string;
  event_type: string;
  trigger: TriggerType;
  file_id: string | null;
  file_type: FileType;
  file_checksum: string | null;
  processing_date: string;
  initiated_by: string;
  outcome: "completed" | "failed" | null;
  duration_ms: number | null;
  error_detail: string | null;
  created_at: string;
}

// ─── Badge Mapping ────────────────────────────────────────────────────────────

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

export const STATUS_BADGE_MAP: Record<ProcessingStatus, BadgeVariant> = {
  completed: "success",
  failed: "danger",
  max_retries_exhausted: "danger",
  processing: "info",
  pending: "neutral",
};

export const TRIGGER_BADGE_MAP: Record<TriggerType, BadgeVariant> = {
  auto: "info",
  manual: "neutral",
};

export const OUTCOME_BADGE_MAP: Record<string, BadgeVariant> = {
  completed: "success",
  failed: "danger",
  in_progress: "info",
};

// ─── Display Labels ───────────────────────────────────────────────────────────

export const STATUS_LABELS: Record<ProcessingStatus, string> = {
  completed: "Completed",
  failed: "Failed",
  max_retries_exhausted: "Max Retries",
  processing: "Processing",
  pending: "Pending",
};

export const FILE_TYPE_LABELS: Record<FileType, string> = {
  dmaa: "DMAA",
  itm_cashpos: "ITM Cash Position",
  itm_replenish: "ITM Replenishment",
};

export const TRIGGER_LABELS: Record<TriggerType, string> = {
  auto: "Auto",
  manual: "Manual",
};
```

### TanStack Query Hooks (`hooks/useEodQueries.ts`)

```typescript
import { api } from "@/lib/api/client";
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryOptions,
} from "@tanstack/react-query";
import type {
  ApiEnvelope,
  AuditLogEntry,
  EodSummary,
  FileHistoryResponse,
  FileStatusResponse,
  LateDetectionItem,
  ManualRetryResponse,
} from "../types";

// ─── Query Keys ───────────────────────────────────────────────────────────────

export const eodKeys = {
  all: ["eod"] as const,
  summary: (date: string) => [...eodKeys.all, "summary", date] as const,
  status: (date: string) => [...eodKeys.all, "status", date] as const,
  history: (fileId: string) => [...eodKeys.all, "history", fileId] as const,
  late: (date: string) => [...eodKeys.all, "late", date] as const,
  audit: (date: string, fileType: string | null, trigger: string | null) =>
    [...eodKeys.all, "audit", date, fileType, trigger] as const,
};

// ─── Hooks ────────────────────────────────────────────────────────────────────

export function useEodSummary(
  processingDate: string,
  refetchInterval: number | false,
) {
  return useQuery({
    queryKey: eodKeys.summary(processingDate),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<EodSummary>>(
        `/api/eod/summary?processing_date=${processingDate}`,
      );
      if (res.data.status === "error") throw new Error(res.data.error ?? "Unknown error");
      return res.data.data!;
    },
    refetchInterval,
  });
}

export function useEodStatus(
  processingDate: string,
  refetchInterval: number | false,
) {
  return useQuery({
    queryKey: eodKeys.status(processingDate),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<FileStatusResponse>>(
        `/api/eod/status?processing_date=${processingDate}`,
      );
      if (res.data.status === "error") throw new Error(res.data.error ?? "Unknown error");
      return res.data.data!;
    },
    refetchInterval,
  });
}

export function useFileHistory(fileId: string | null) {
  return useQuery({
    queryKey: eodKeys.history(fileId ?? ""),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<FileHistoryResponse>>(
        `/api/eod/status/${fileId}/history`,
      );
      if (res.data.status === "error") throw new Error(res.data.error ?? "Unknown error");
      return res.data.data!;
    },
    enabled: !!fileId,
  });
}

export function useEodLate(
  processingDate: string,
  refetchInterval: number | false,
) {
  return useQuery({
    queryKey: eodKeys.late(processingDate),
    queryFn: async () => {
      const res = await api.get<ApiEnvelope<LateDetectionItem[]>>(
        `/api/eod/late?processing_date=${processingDate}`,
      );
      if (res.data.status === "error") throw new Error(res.data.error ?? "Unknown error");
      return res.data.data!;
    },
    refetchInterval,
  });
}

export function useEodAudit(
  processingDate: string,
  fileType: string | null,
  trigger: string | null,
  refetchInterval: number | false,
) {
  return useQuery({
    queryKey: eodKeys.audit(processingDate, fileType, trigger),
    queryFn: async () => {
      const params = new URLSearchParams({ processing_date: processingDate });
      if (fileType) params.set("file_type", fileType);
      if (trigger) params.set("trigger", trigger);

      const res = await api.get<ApiEnvelope<AuditLogEntry[]>>(
        `/api/eod/audit?${params.toString()}`,
      );
      if (res.data.status === "error") throw new Error(res.data.error ?? "Unknown error");
      return res.data.data!;
    },
    refetchInterval,
  });
}

export function useRetryFile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (fileId: string) => {
      const res = await api.post<ApiEnvelope<ManualRetryResponse>>(
        `/api/eod/retry/${fileId}`,
      );
      if (res.data.status === "error") {
        const error = new Error(res.data.error ?? "Retry failed");
        (error as any).status = res.status;
        throw error;
      }
      return res.data.data!;
    },
    onSuccess: () => {
      // Invalidate all EOD queries to refetch fresh data
      queryClient.invalidateQueries({ queryKey: eodKeys.all });
    },
  });
}
```

---

## Data Models

### Page State

```typescript
interface EodMonitoringPageState {
  processingDate: string;       // "YYYY-MM-DD" in Asia/Jakarta
  pollingEnabled: boolean;      // default: true
  selectedFileId: string | null; // controls drawer open/close
  retryDialogFile: FileStatusItem | null; // controls dialog open/close
}
```

The page state is managed via `useState` hooks within `EodMonitoringPage` — no global store needed since this state is local to the page lifecycle.

### Flattened File Status for Table

The API returns file status grouped by `file_type`. The table component flattens this into a single array for rendering:

```typescript
function flattenFileStatus(response: FileStatusResponse): FileStatusRow[] {
  const rows: FileStatusRow[] = [];
  for (const [fileType, files] of Object.entries(response.by_file_type)) {
    for (const file of files) {
      rows.push({ ...file, file_type: fileType as FileType });
    }
  }
  return rows;
}

// Extended type for table row (adds file_type since API groups by it)
interface FileStatusRow extends FileStatusItem {
  file_type: FileType;
}
```

---

## Key Patterns

### Polling Toggle Pattern

```typescript
// In EodMonitoringPage
const [pollingEnabled, setPollingEnabled] = useState(true);
const refetchInterval = pollingEnabled ? 60_000 : false;

// Passed to each section as a prop
<SummarySection
  processingDate={processingDate}
  refetchInterval={refetchInterval}
/>
```

### Section Error Isolation

Each section independently manages its query state. A failed query in one section does not affect others:

```typescript
// In SummarySection
const { data, isLoading, isError, error, refetch } = useEodSummary(
  processingDate,
  refetchInterval,
);

if (isLoading) return <SummarySkeleton />;
if (isError) return <SectionErrorState message={error.message} onRetry={refetch} />;
return <SummaryCards data={data} />;
```

### Drawer Open/Close with Focus Management

```typescript
// RetryDrawer manages focus trap via a ref and useEffect
function RetryDrawer({ fileId, onClose }: RetryDrawerProps) {
  const drawerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (fileId) {
      // Store the currently focused element (the triggering row)
      triggerRef.current = document.activeElement as HTMLElement;
      // Move focus into drawer on next frame
      requestAnimationFrame(() => {
        drawerRef.current?.querySelector<HTMLElement>("[data-autofocus]")?.focus();
      });
    }
    return () => {
      // Return focus to trigger on unmount/close
      triggerRef.current?.focus();
    };
  }, [fileId]);

  // Escape key handler
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  // ... render with transition classes
}
```

### Retry Mutation Flow

```typescript
// In RetryConfirmationDialog
const retryMutation = useRetryFile();
const { toast } = useToast();

function handleConfirm() {
  retryMutation.mutate(file.file_id, {
    onSuccess: () => {
      toast({ type: "success", message: `Retry berhasil dipicu untuk ${file.filename}` });
      onClose();
    },
    onError: (error) => {
      toast({ type: "error", message: error.message });
      onClose();
    },
  });
}
```

### Timestamp Formatting Utility

```typescript
import { format } from "date-fns";
import { toZonedTime } from "date-fns-tz";

const WIB = "Asia/Jakarta";

export function formatWibDateTime(iso: string): string {
  const zonedDate = toZonedTime(new Date(iso), WIB);
  return format(zonedDate, "dd MMM yyyy HH:mm");
}

export function formatWibDateTimeSec(iso: string): string {
  const zonedDate = toZonedTime(new Date(iso), WIB);
  return format(zonedDate, "dd MMM yyyy HH:mm:ss");
}

export function formatSlaTime(timeStr: string): string {
  // Input: "06:00:00" or "06:00"
  const parts = timeStr.split(":");
  return `${parts[0]}:${parts[1]} WIB`;
}

export function formatDuration(ms: number | null): string {
  if (ms === null) return "-";
  return `${ms.toLocaleString("id-ID")} ms`;
}
```

### Status-to-Badge Mapping with Icon

```typescript
import {
  CheckCircle,
  Clock,
  AlertTriangle,
  XCircle,
  Loader,
  type LucideIcon,
} from "lucide-react";
import type { ProcessingStatus, BadgeVariant } from "../types";

interface StatusBadgeConfig {
  variant: BadgeVariant;
  icon: LucideIcon;
  label: string;
}

export const STATUS_BADGE_CONFIG: Record<ProcessingStatus, StatusBadgeConfig> = {
  completed: { variant: "success", icon: CheckCircle, label: "Completed" },
  failed: { variant: "danger", icon: XCircle, label: "Failed" },
  max_retries_exhausted: { variant: "danger", icon: AlertTriangle, label: "Max Retries" },
  processing: { variant: "info", icon: Loader, label: "Processing" },
  pending: { variant: "neutral", icon: Clock, label: "Pending" },
};
```

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Section query fails (network/server) | Section shows inline error with "Coba Lagi" button. Other sections unaffected. |
| Retry POST returns 409 (file already completed) | Close dialog, show error toast with message from `error` field. |
| Retry POST returns 4xx/5xx | Close dialog, show error toast with error message. |
| Retry POST in-flight | Disable both dialog buttons, show spinner on confirm button. |
| Auth token expired during polling | `apiClient` handles 401 → refresh → retry transparently. |
| Polling query silently fails | TanStack Query will retry on next interval. No user-visible error for background refetch failures (only initial load shows error). |

### Error State Component

```typescript
interface SectionErrorStateProps {
  message: string;
  onRetry: () => void;
}

function SectionErrorState({ message, onRetry }: SectionErrorStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-center">
      <AlertTriangle className="h-8 w-8 text-[var(--danger-fg)] mb-2" aria-hidden="true" />
      <p className="text-sm text-[var(--n-700)] mb-4">{message}</p>
      <Button variant="secondary" onClick={onRetry}>
        Coba Lagi
      </Button>
    </div>
  );
}
```

---

## Accessibility

1. **Semantic landmarks**: `<main>` wraps page content. Each section card uses `<section aria-labelledby="section-title-id">`.
2. **Focus management**: RetryDrawer and RetryConfirmationDialog trap focus while open. Focus returns to trigger element on close.
3. **Status badges**: Always render with icon + label text. Never signal status by color alone.
4. **Touch targets**: All interactive elements enforce `min-h-[44px] min-w-[44px]` via the Button/FilterSelect/toggle components.
5. **Dynamic announcements**: Toast notifications use an `aria-live="polite"` region (existing ToastContainer).
6. **Keyboard navigation**: DataTable rows are focusable and activatable with Enter/Space. Drawer closes on Escape.
7. **Screen reader**: Loading states use `aria-busy="true"` on the section container.

---

## Responsive Layout

| Breakpoint | Summary Grid | Table | Drawer |
|-----------|-------------|-------|--------|
| Desktop (≥1024px) | 3 columns | Full width, all columns | 400px fixed width |
| Tablet (≥768px) | 2 columns | Full width, horizontal scroll if needed | 400px fixed width |
| Mobile (<768px) | 1 column | Horizontal scroll | Full width overlay |

```typescript
// Summary grid classes
const summaryGridClass = "grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3";
```

---

## Motion

- **Drawer enter**: 300ms `cubic-bezier(0.22, 1, 0.36, 1)`, animate `transform: translateX(100%) → translateX(0)` and `opacity: 0 → 1`.
- **Drawer exit**: 225ms (75% of enter), same easing reversed.
- **Dialog**: 200ms scale + opacity enter, 150ms exit.
- **Toasts**: Use existing `toast-enter` keyframe animation (300ms).
- **Row hover**: 100ms `transition-colors` (existing DataTable pattern).

All animations use `transform` and `opacity` only — no layout property animations.

---

## Security Constraints

1. **Route guard**: `requireRoles(["ADMIN", "ADMIN_PARAM"])` in `beforeLoad` prevents component render for unauthorized roles.
2. **API auth**: All requests flow through `api` client which injects Bearer token and handles 401 refresh.
3. **No sensitive data display**: File checksums are truncated to 12 characters in the UI (full hash never shown to user).
4. **CSRF**: Fetch requests use `credentials: "include"` with SameSite cookie policy (existing pattern).

---

## Dependencies

No new dependencies required. The feature uses existing project libraries:

| Library | Usage |
|---------|-------|
| `@tanstack/react-query` v5 | Server state, polling, mutation |
| `@tanstack/react-table` v8 | File status table, audit log table |
| `@tanstack/react-router` | File-based route registration |
| `lucide-react` | Status icons in badges |
| `date-fns` + `date-fns-tz` | Timestamp formatting in WIB |
| `zustand` | Toast store (existing) |

If `date-fns-tz` is not already installed, it needs to be added for WIB timezone conversion. Alternative: use `Intl.DateTimeFormat` with `timeZone: "Asia/Jakarta"` which requires no additional dependency.

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Role-based access denial

*For any* user with a role that is NOT `ADMIN` and NOT `ADMIN_PARAM`, navigating to `/eod-monitoring` SHALL result in a 403 Forbidden state being rendered — the page content SHALL NOT be visible.

**Validates: Requirements 1.2, 1.3**

### Property 2: Date change triggers universal refetch

*For any* date value selected in the ProcessingDatePicker, ALL section queries (summary, status, late, audit) SHALL include that date as the `processing_date` parameter in their next API request.

**Validates: Requirements 2.5, 9.5**

### Property 3: Summary card completeness

*For any* valid EodSummary response containing counts for the 6 states (pending, processing, completed, failed, max_retries_exhausted, late), the SummarySection SHALL render exactly 6 SummaryCard components with values matching the corresponding count fields.

**Validates: Requirements 3.2**

### Property 4: Error message propagation

*For any* API error response containing a non-null `error` field, the section that received the error SHALL display the exact error message text from that field in its inline error state.

**Validates: Requirements 3.5, 10.2**

### Property 5: Status-to-badge mapping correctness

*For any* `processing_status` value in the set {completed, failed, max_retries_exhausted, processing, pending}, the status-to-badge mapping function SHALL return the correct BadgeVariant: completed→success, failed→danger, max_retries_exhausted→danger, processing→info, pending→neutral. Likewise, *for any* `trigger` value in {auto, manual}, the mapping SHALL return auto→info, manual→neutral. And *for any* `outcome` value in {completed, failed, null}, the mapping SHALL return completed→success, failed→danger, null→info.

**Validates: Requirements 4.3, 8.5, 8.6**

### Property 6: Badge accessibility — icon plus label

*For any* Badge component rendered on the EOD Monitoring page (status, trigger, outcome, or late resolution), the Badge SHALL contain both a visible icon element and a text label — never color alone.

**Validates: Requirements 4.4, 7.3, 7.4**

### Property 7: Timestamp WIB formatting

*For any* valid ISO 8601 timestamp string, the `formatWibDateTime` function SHALL produce a string in the format `DD MMM YYYY HH:mm` representing that instant in the Asia/Jakarta timezone. Similarly, `formatWibDateTimeSec` SHALL produce `DD MMM YYYY HH:mm:ss` in Asia/Jakarta timezone.

**Validates: Requirements 4.5, 8.7**

### Property 8: File status table column completeness

*For any* non-empty array of FileStatusRow objects, the rendered DataTable SHALL display all 7 columns (File Type, Filename, Status, Retry Count, Detected At, Last Retry At, Failure Reason) with corresponding cell values derived from the row data.

**Validates: Requirements 4.2**

### Property 9: Retry button visibility state machine

*For any* file displayed in the RetryDrawer, the "Retry Manual" button state SHALL be determined solely by `processing_status`: if `failed` or `max_retries_exhausted` → button rendered and enabled; if `processing` → button rendered and disabled; if `completed` or `pending` → button NOT rendered.

**Validates: Requirements 5.5, 5.6, 5.7**

### Property 10: Checksum truncation

*For any* file checksum string of length ≥ 12, the RetryDrawer SHALL display only the first 12 characters followed by an ellipsis indicator. *For any* checksum string of length < 12, the full string SHALL be displayed.

**Validates: Requirements 5.3**

### Property 11: Retry attempt list field completeness

*For any* array of RetryAttempt objects returned by the history API, each attempt item in the RetryDrawer SHALL render: attempt number, trigger type badge, started_at timestamp, duration in milliseconds, outcome badge, and error detail (when outcome is "failed").

**Validates: Requirements 5.4**

### Property 12: Confirmation dialog metadata display

*For any* FileStatusItem passed to the RetryConfirmationDialog, the dialog SHALL display the file's filename, file_type label, and current retry_count value.

**Validates: Requirements 6.2**

### Property 13: Retry error toast message propagation

*For any* failed retry POST response (409 or other HTTP error), the error Toast SHALL display the message from the response's `error` field (for envelope errors) or the HTTP status message.

**Validates: Requirements 6.7, 6.8**

### Property 14: Late alert field completeness and badge mapping

*For any* LateDetectionItem, the rendered LateAlertCard SHALL display file_type, sla_deadline (formatted as `HH:mm WIB`), detected_at timestamp, and a Badge — where `is_resolved === false` produces a warning Badge labeled "Late" and `is_resolved === true` produces a success Badge labeled "Resolved".

**Validates: Requirements 7.2, 7.3, 7.4, 7.5**

### Property 15: Audit filter propagation

*For any* filter combination selected in the AuditLogSection (file_type and/or trigger), the audit API request SHALL include exactly those non-null filter values as query parameters, and the returned data SHALL be re-rendered in the table.

**Validates: Requirements 8.3**

### Property 16: Duration formatting

*For any* numeric duration value in milliseconds, the formatted display SHALL be the locale-formatted number followed by " ms" suffix, right-aligned with `tabular-nums` styling. *For any* null duration value, the display SHALL be "-".

**Validates: Requirements 8.8**

### Property 17: Section failure isolation

*For any* combination where one or more section queries fail and others succeed, the successful sections SHALL render their data normally — a failure in one section SHALL NOT prevent rendering of any other section.

**Validates: Requirements 10.2, 10.3**

### Property 18: SLA deadline formatting

*For any* SLA deadline time string (in "HH:mm" or "HH:mm:ss" format), the `formatSlaTime` function SHALL produce a string in "HH:mm WIB" format.

**Validates: Requirements 7.5**
