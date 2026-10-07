import { formatBadgeCount } from "@/lib/utils/formatters";
import { useRouter } from "@tanstack/react-router";
import { Bell } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { NotificationItem } from "./api";
import { useMarkAllRead, useMarkRead, useRecentNotifications, useUnreadCount } from "./hooks";

const timeFormat = new Intl.DateTimeFormat("id-ID", {
  dateStyle: "medium",
  timeStyle: "short",
  timeZone: "Asia/Jakarta",
});

const messageStyle = "px-[var(--space-3)] py-[var(--space-3)] text-sm";

/**
 * NotificationBell — header bell with the unread count (polled every 60 s)
 * and a panel of the 10 newest notifications (spec FR5.1). The count is
 * shown as text and in the accessible name, never by colour alone.
 */
export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const router = useRouter();
  const { data: unread = 0 } = useUnreadCount();
  const recent = useRecentNotifications(open);
  const markRead = useMarkRead();
  const markAll = useMarkAllRead();
  const badge = formatBadgeCount(unread);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const onPointer = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onPointer);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onPointer);
    };
  }, [open]);

  const openItem = (n: NotificationItem) => {
    if (!n.is_read) markRead.mutate(n.id);
    setOpen(false);
    if (n.link) router.history.push(n.link);
  };

  return (
    <div className="relative" ref={rootRef}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="relative flex h-9 w-9 items-center justify-center rounded-[var(--radius-md)] hover:bg-[var(--n-100)]"
        style={{ color: "var(--n-600)" }}
        aria-label={unread > 0 ? `Notifikasi, ${unread} belum dibaca` : "Notifikasi"}
        aria-expanded={open}
        aria-haspopup="dialog"
      >
        <Bell size={18} aria-hidden="true" />
        {badge && (
          <span
            data-testid="notification-badge"
            aria-hidden="true"
            className="absolute -right-1 -top-1 inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full px-1 text-[10px] font-semibold leading-none tabular-nums text-white"
            style={{ backgroundColor: "var(--red-600)" }}
          >
            {badge}
          </span>
        )}
      </button>

      {open && (
        <dialog
          open
          aria-label="Notifikasi"
          className="absolute left-auto right-0 top-full z-50 m-0 mt-[var(--space-1)] w-[min(360px,calc(100vw-32px))] overflow-hidden rounded-[var(--radius-md)] border p-0"
          style={{
            backgroundColor: "var(--n-0)",
            borderColor: "var(--n-200)",
            boxShadow: "var(--shadow-md)",
          }}
        >
          <div
            className="flex items-center justify-between px-[var(--space-3)] py-[var(--space-2)]"
            style={{ borderBottom: "1px solid var(--n-100)" }}
          >
            <span className="text-sm font-medium" style={{ color: "var(--n-800)" }}>
              Notifikasi
            </span>
            <button
              type="button"
              onClick={() => markAll.mutate()}
              disabled={unread === 0 || markAll.isPending}
              className="text-xs font-semibold disabled:opacity-50"
              style={{ color: "var(--red-600)" }}
            >
              Tandai semua dibaca
            </button>
          </div>

          <ul className="max-h-[360px] overflow-y-auto">
            {recent.isLoading && (
              <li className={messageStyle} style={{ color: "var(--n-500)" }}>
                Memuat...
              </li>
            )}
            {recent.isError && (
              <li className={messageStyle} style={{ color: "var(--danger-fg)" }}>
                Gagal memuat notifikasi.
              </li>
            )}
            {recent.data?.items.length === 0 && (
              <li className={messageStyle} style={{ color: "var(--n-500)" }}>
                Belum ada notifikasi.
              </li>
            )}
            {recent.data?.items.map((n) => (
              <li key={n.id} style={{ borderBottom: "1px solid var(--n-100)" }}>
                <button
                  type="button"
                  onClick={() => openItem(n)}
                  className="flex w-full flex-col gap-[2px] px-[var(--space-3)] py-[var(--space-2)] text-left hover:bg-[var(--n-50)]"
                >
                  <span className="flex items-center gap-[var(--space-2)]">
                    {!n.is_read && (
                      <span
                        className="rounded-full px-[6px] text-[10px] font-semibold"
                        style={{ backgroundColor: "var(--red-50)", color: "var(--red-600)" }}
                      >
                        Baru
                      </span>
                    )}
                    <span
                      className={n.is_read ? "text-sm font-normal" : "text-sm font-semibold"}
                      style={{ color: "var(--n-800)" }}
                    >
                      {n.title}
                    </span>
                  </span>
                  {n.body && (
                    <span className="line-clamp-2 text-xs" style={{ color: "var(--n-600)" }}>
                      {n.body}
                    </span>
                  )}
                  <time
                    dateTime={n.created_at}
                    className="text-[11px]"
                    style={{ color: "var(--n-500)" }}
                  >
                    {timeFormat.format(new Date(n.created_at))} WIB
                  </time>
                </button>
              </li>
            ))}
          </ul>
        </dialog>
      )}
    </div>
  );
}
