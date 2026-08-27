interface PollingToggleProps {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
}

/** Toggle switch controlling 60s auto-refresh polling for the EOD Monitoring page. */
export function PollingToggle({ enabled, onChange }: PollingToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={enabled}
      onClick={() => onChange(!enabled)}
      className="inline-flex min-h-[44px] items-center gap-2 rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm font-medium text-[var(--n-800)] transition-colors duration-150 hover:bg-[var(--n-50)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)] focus-visible:border-[var(--red-400)] outline-none"
    >
      <span
        className="relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors duration-150"
        style={{ backgroundColor: enabled ? "var(--red-500)" : "var(--n-300)" }}
      >
        <span
          className="inline-block h-4 w-4 transform rounded-full bg-[var(--n-0)] transition-transform duration-150"
          style={{ transform: enabled ? "translateX(18px)" : "translateX(2px)" }}
        />
      </span>
      Polling {enabled ? "Aktif" : "Nonaktif"}
    </button>
  );
}
