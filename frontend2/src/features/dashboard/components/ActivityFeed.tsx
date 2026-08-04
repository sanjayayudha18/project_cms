import { CheckCircle, FileOutput, GitCompare, Upload } from "lucide-react";
import type { ActivityEvent } from "../types";

// ─── ActivityFeed ─────────────────────────────────────────────────────────────

export interface ActivityFeedProps {
  events: ActivityEvent[];
}

const TYPE_ICONS = {
  upload: Upload,
  approval: CheckCircle,
  reconciliation: GitCompare,
  generation: FileOutput,
} as const;

/**
 * Formats a timestamp into relative time in Bahasa Indonesia.
 */
function formatRelativeTime(timestamp: string): string {
  const now = Date.now();
  const then = new Date(timestamp).getTime();
  const diffMs = now - then;
  const diffMinutes = Math.floor(diffMs / 60_000);

  if (diffMinutes < 1) return "Baru saja";
  if (diffMinutes < 60) return `${diffMinutes} menit yang lalu`;

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours} jam yang lalu`;

  const diffDays = Math.floor(diffHours / 24);
  if (diffDays === 1) return "Kemarin";
  return `${diffDays} hari yang lalu`;
}

export function ActivityFeed({ events }: ActivityFeedProps) {
  if (events.length === 0) {
    return (
      <p className="text-sm" style={{ color: "var(--n-500)" }}>
        Belum ada aktivitas terbaru.
      </p>
    );
  }

  return (
    <ul className="flex flex-col" aria-label="Aktivitas terbaru">
      {events.map((event, index) => {
        const Icon = TYPE_ICONS[event.type];
        return (
          <li
            key={event.id}
            className="flex items-start gap-[var(--space-3)] py-[var(--space-3)]"
            style={{
              borderTop: index > 0 ? "1px solid var(--n-100)" : undefined,
            }}
          >
            <div
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
              style={{ backgroundColor: "var(--n-100)", color: "var(--n-600)" }}
            >
              <Icon size={16} aria-hidden="true" />
            </div>

            <div className="flex min-w-0 flex-1 flex-col gap-[var(--space-1)]">
              <p className="text-sm leading-snug" style={{ color: "var(--n-800)" }}>
                {event.description}
              </p>
              <div className="flex items-center gap-[var(--space-2)]">
                <span className="text-xs" style={{ color: "var(--n-500)" }}>
                  {event.actor}
                </span>
                <span className="text-xs" style={{ color: "var(--n-400)" }} aria-hidden="true">
                  ·
                </span>
                <time
                  className="text-xs"
                  style={{ color: "var(--n-500)" }}
                  dateTime={event.timestamp}
                >
                  {formatRelativeTime(event.timestamp)}
                </time>
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}
