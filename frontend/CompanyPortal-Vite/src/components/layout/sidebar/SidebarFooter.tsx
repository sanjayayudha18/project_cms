import { APP_ENV, APP_VERSION, type AppEnv, ENV_LABEL } from "@/lib/config/appEnv";
import { cn } from "@/lib/utils/cn";
import { ChevronsLeft, ChevronsRight } from "lucide-react";

interface EnvBadgeProps {
  /** Default dari konfigurasi build; bisa di-override untuk test. */
  env?: AppEnv;
  version?: string;
}

/** Kartu lingkungan + versi (Req 5.1–5.4). Chip gold hanya untuk non-produksi. */
export function EnvBadge({ env = APP_ENV, version = APP_VERSION }: EnvBadgeProps) {
  const isProd = env === "prod";
  return (
    <div className="min-w-0">
      <p className="text-[10px] font-bold tracking-[0.14em] text-[var(--shell-sb-group)] uppercase">
        Lingkungan
      </p>
      <p className="mt-0.5 flex items-center gap-2 text-xs text-[var(--shell-sb-text)] tabular-nums">
        {!isProd && (
          <span className="rounded-md bg-[var(--shell-gold-bg)] px-1.5 py-0.5 text-[10px] font-bold tracking-wide text-[var(--shell-gold)]">
            {ENV_LABEL[env]}
          </span>
        )}
        <span>{isProd ? `${ENV_LABEL[env]} · v${version}` : `· v${version}`}</span>
      </p>
    </div>
  );
}

interface SidebarFooterProps extends EnvBadgeProps {
  /** Mode rail: EnvBadge tersembunyi, tombol tinggal ikon (Req 4.3). */
  rail: boolean;
  onToggleRail: () => void;
}

/** Footer Sidebar: EnvBadge + tombol ciutkan/lebarkan menu (Req 4.5, 5.1). */
export function SidebarFooter({ rail, onToggleRail, env, version }: SidebarFooterProps) {
  const Icon = rail ? ChevronsRight : ChevronsLeft;
  const label = rail ? "Lebarkan menu" : "Ciutkan menu";
  return (
    <div
      className={cn(
        "flex shrink-0 flex-col gap-2 border-t border-[var(--shell-sb-line)] p-2",
        rail && "items-center",
      )}
    >
      {!rail && (
        <div className="px-1 pt-1">
          <EnvBadge env={env} version={version} />
        </div>
      )}
      <button
        type="button"
        aria-label={label}
        aria-keyshortcuts="["
        title={label}
        onClick={onToggleRail}
        className={cn(
          "flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-semibold text-[var(--shell-sb-text)] outline-none hover:bg-[var(--shell-sb-hover)] focus-visible:ring-2 focus-visible:ring-[var(--shell-focus)]",
          rail ? "justify-center" : "w-full",
        )}
      >
        <Icon aria-hidden="true" className="size-4 shrink-0" />
        {!rail && (
          <>
            <span>{label}</span>
            <kbd className="ml-auto rounded border border-[var(--shell-sb-line)] px-1.5 font-mono text-[10px] text-[var(--shell-sb-group)]">
              [
            </kbd>
          </>
        )}
      </button>
    </div>
  );
}
