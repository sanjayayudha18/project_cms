import { cn } from "@kits/shadcn-ui";
import { PenLine, ShieldCheck, KeyRound } from "lucide-react";
import { ROLES, roleLabel } from "./data.js";

const TONES = {
  neutral: "bg-[var(--surface-sunken)] text-[var(--ink-secondary)]",
  info: "bg-[var(--info-tint)] text-[var(--info)]",
  warning: "bg-[var(--warning-tint)] text-[var(--warning-text)]",
  success: "bg-[var(--success-tint)] text-[var(--success)]",
  danger: "bg-[var(--danger-tint)] text-[var(--danger)]",
  brand: "bg-[var(--brand-tint)] text-[var(--brand-text)]",
};

/* §6 Status pills: uppercase, tracked, icon + label, tinted, no border. */
export function Pill({ tone = "neutral", icon: Icon, children, className }) {
  return (
    <span
      className={cn(
        "label-xs inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2 py-[3px] align-middle",
        TONES[tone],
        className,
      )}
    >
      {Icon ? <Icon className="size-3 shrink-0" strokeWidth={2.25} /> : null}
      {children}
    </span>
  );
}

const LEVEL_ICON = { maker: PenLine, checker: ShieldCheck, admin: KeyRound };

export function RolePill({ role, tone }) {
  const level = ROLES[role].level;
  return (
    <Pill tone={tone ?? (level === "checker" ? "info" : "neutral")} icon={LEVEL_ICON[level]}>
      {roleLabel(role)}
    </Pill>
  );
}

export function Eyebrow({ children, className }) {
  return <p className={cn("label-xs text-[var(--brand-text)]", className)}>{children}</p>;
}

export function Field({ label, children }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="label-xs text-[var(--ink-muted)]">{label}</span>
      <span className="text-sm text-[var(--ink)]">{children}</span>
    </div>
  );
}

export function AuthTag({ auth }) {
  return auth === "entra_id" ? (
    <span className="text-sm text-[var(--ink-secondary)]">Entra ID</span>
  ) : (
    <span className="mono text-sm text-[var(--ink-secondary)]">local</span>
  );
}
