import { useAuthStore } from "@/lib/auth/store";
import { cn } from "@/lib/utils/cn";
import { getInitials } from "@/lib/utils/initials";
import { ChevronDown, LogOut } from "lucide-react";
import { type KeyboardEvent, useEffect, useId, useRef, useState } from "react";

/**
 * UserMenu — avatar trigger + WAI-ARIA menu button (Req 11). Reads the user and
 * `logout` from the auth store, the same path Header.tsx used (D7).
 */
export function UserMenu() {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const menuId = useId();

  useEffect(() => {
    if (!open) return;
    menuRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    const onPointer = (e: PointerEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    return () => document.removeEventListener("pointerdown", onPointer);
  }, [open]);

  if (!user) return null;

  const onMenuKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      triggerRef.current?.focus();
      return;
    }
    const items = Array.from(
      menuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [],
    );
    if (items.length === 0) return;
    const current = items.indexOf(document.activeElement as HTMLElement);
    const target = {
      ArrowDown: (current + 1) % items.length,
      ArrowUp: (current - 1 + items.length) % items.length,
      Home: 0,
      End: items.length - 1,
    }[e.key];
    if (target === undefined) return;
    e.preventDefault();
    items[target]?.focus();
  };

  return (
    <div className="relative" ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="shell-focus flex items-center gap-2.5 rounded-xl px-1.5 py-1 text-[var(--shell-ink)] outline-none hover:bg-[var(--shell-canvas)]"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        data-testid="header-user-name"
      >
        <span
          aria-hidden="true"
          className="flex size-9 shrink-0 items-center justify-center rounded-full bg-[image:var(--shell-avatar-bg)] text-xs font-semibold text-[var(--shell-on-accent)]"
        >
          {getInitials(user.fullName)}
        </span>
        <span className="hidden min-w-0 flex-col items-start text-left sm:flex">
          <span className="max-w-40 truncate text-sm font-semibold leading-tight">
            {user.fullName}
          </span>
          <span className="text-xs leading-tight text-[var(--shell-muted)]">{user.role}</span>
        </span>
        <ChevronDown
          size={16}
          aria-hidden="true"
          className={cn(
            "hidden shrink-0 text-[var(--shell-muted)] transition-transform motion-reduce:transition-none sm:block",
            open && "rotate-180",
          )}
        />
      </button>

      {open && (
        <div
          ref={menuRef}
          id={menuId}
          role="menu"
          aria-label="Menu pengguna"
          tabIndex={-1}
          onKeyDown={onMenuKeyDown}
          className="shell-pop absolute right-0 top-full z-50 mt-2 w-60 origin-top-right overflow-hidden rounded-2xl border border-[var(--shell-topbar-line)] bg-[var(--shell-topbar-bg)] py-1 shadow-[var(--shell-shadow-pop)]"
        >
          <div className="flex flex-col px-3 py-2.5 sm:hidden">
            <span className="text-sm font-semibold text-[var(--shell-ink)]">{user.fullName}</span>
            <span className="text-xs text-[var(--shell-muted)]">{user.role}</span>
          </div>
          <div className="px-3 py-2.5">
            <span
              className="block min-w-0 truncate text-xs text-[var(--shell-muted)]"
              data-testid="header-email"
            >
              {user.email}
            </span>
          </div>
          <div className="my-1 h-px bg-[var(--shell-topbar-line)]" aria-hidden="true" />
          <button
            type="button"
            role="menuitem"
            onClick={() => void logout()}
            className="shell-focus mx-1 flex w-[calc(100%-0.5rem)] items-center gap-2 rounded-xl px-3 py-2 text-left text-sm text-[var(--danger-fg)] outline-none hover:bg-[var(--danger-bg)] focus-visible:bg-[var(--danger-bg)]"
            data-testid="header-logout-button"
            aria-label="Keluar dari sistem"
          >
            <LogOut size={16} className="shrink-0" aria-hidden="true" />
            <span className="min-w-0 truncate">Keluar</span>
          </button>
        </div>
      )}
    </div>
  );
}
