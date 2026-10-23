import { GROUP_LABELS, type NavItem } from "@/lib/config/navigation";
import { ChevronRight } from "lucide-react";

interface BreadcrumbProps {
  /** Item nav aktif dari `findActiveNavItem`; undefined = tidak ada yang dirender (Req 7.4). */
  active: NavItem | undefined;
}

/** Breadcrumb topbar: "Grup › Item" (Req 7.2–7.4). Pengaturan menjadi "Sistem › Pengaturan" lewat `GROUP_LABELS.system`. */
export function Breadcrumb({ active }: BreadcrumbProps) {
  if (!active) return null;

  return (
    <nav aria-label="Breadcrumb" className="min-w-0">
      <ol className="flex min-w-0 items-center gap-1.5 text-sm">
        <li className="truncate text-[var(--shell-muted)]">{GROUP_LABELS[active.group]}</li>
        <li aria-hidden="true" className="flex shrink-0 text-[var(--shell-muted)]">
          <ChevronRight className="size-3.5" />
        </li>
        <li aria-current="page" className="truncate font-semibold text-[var(--shell-ink)]">
          {active.label}
        </li>
      </ol>
    </nav>
  );
}
