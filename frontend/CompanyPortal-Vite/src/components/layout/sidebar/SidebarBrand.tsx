import { cn } from "@/lib/utils/cn";

interface SidebarBrandProps {
  /** Mode rail: hanya kotak logo di tengah header (Req 1.3). */
  rail: boolean;
}

/** Header brand Sidebar: logo 36px + dot status + nama aplikasi (Req 1.1–1.3). */
export function SidebarBrand({ rail }: SidebarBrandProps) {
  return (
    <div
      className={cn(
        "flex h-16 shrink-0 items-center gap-3 border-b border-[var(--shell-sb-line)]",
        rail ? "justify-center px-0" : "px-5",
      )}
    >
      <div className="relative shrink-0">
        {/* Belum ada aset logo di repo; spec (Req 1.1) mendefinisikan logo sebagai kotak "C". */}
        <div
          aria-hidden="true"
          className="flex size-9 items-center justify-center rounded-xl bg-[var(--shell-accent)] text-base font-bold text-[var(--shell-on-accent)]"
        >
          C
        </div>
        <span
          aria-hidden="true"
          className="absolute -top-0.5 -right-0.5 size-2 rounded-full bg-[var(--success-solid)] ring-2 ring-[var(--shell-sb-bg)]"
        />
        <span className="sr-only">Status: online</span>
      </div>
      {!rail && (
        <div className="min-w-0 leading-tight">
          <p className="truncate text-[15px] font-extrabold text-[var(--shell-sb-strong)]">CROWN</p>
          <p className="truncate text-xs text-[var(--shell-sb-group)]">Cash Management</p>
        </div>
      )}
    </div>
  );
}
