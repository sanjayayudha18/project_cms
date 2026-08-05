import { cn } from "@/lib/utils/cn";
import { Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";

export interface ModuleCardProps {
  id: string;
  title: string;
  description: string;
  href: string;
  icon: LucideIcon;
  disabled: boolean;
}

export function ModuleCard({ title, description, href, icon: Icon, disabled }: ModuleCardProps) {
  if (disabled) {
    return (
      <div
        className={cn(
          "relative flex flex-col gap-[var(--space-3)] rounded-[var(--radius-lg)] border p-[var(--space-5)]",
          "bg-[var(--n-0)] border-[var(--n-200)] shadow-[var(--shadow-sm)]",
          "opacity-60 cursor-not-allowed select-none",
        )}
      >
        {/* Segera Hadir badge */}
        <span
          className="absolute top-[var(--space-3)] right-[var(--space-3)] inline-flex items-center rounded-[var(--radius-sm)] px-[var(--space-2)] py-0.5 text-xs font-medium"
          style={{
            backgroundColor: "var(--n-100)",
            color: "var(--n-500)",
          }}
        >
          Segera Hadir
        </span>

        <div
          className="flex h-10 w-10 items-center justify-center rounded-[var(--radius-md)]"
          style={{ backgroundColor: "var(--n-100)" }}
        >
          <Icon size={20} style={{ color: "var(--n-400)" }} aria-hidden="true" />
        </div>

        <div className="flex flex-col gap-[var(--space-1)]">
          <h3 className="text-sm font-semibold" style={{ color: "var(--n-600)" }}>
            {title}
          </h3>
          <p className="text-xs" style={{ color: "var(--n-400)" }}>
            {description}
          </p>
        </div>
      </div>
    );
  }

  return (
    <Link
      to={href}
      className={cn(
        "group flex flex-col gap-[var(--space-3)] rounded-[var(--radius-lg)] border p-[var(--space-5)]",
        "bg-[var(--n-0)] border-[var(--n-200)] shadow-[var(--shadow-sm)]",
        "transition-all duration-200",
        "hover:border-[var(--red-200)] hover:shadow-[var(--shadow-md)]",
      )}
    >
      <div
        className="flex h-10 w-10 items-center justify-center rounded-[var(--radius-md)] transition-colors duration-200 group-hover:bg-[var(--red-50)]"
        style={{ backgroundColor: "var(--n-100)" }}
      >
        <Icon
          size={20}
          className="transition-colors duration-200 group-hover:text-[var(--red-500)]"
          style={{ color: "var(--n-700)" }}
          aria-hidden="true"
        />
      </div>

      <div className="flex flex-col gap-[var(--space-1)]">
        <h3
          className="text-sm font-semibold transition-colors duration-200 group-hover:text-[var(--red-600)]"
          style={{ color: "var(--n-900)" }}
        >
          {title}
        </h3>
        <p className="text-xs" style={{ color: "var(--n-600)" }}>
          {description}
        </p>
      </div>
    </Link>
  );
}
