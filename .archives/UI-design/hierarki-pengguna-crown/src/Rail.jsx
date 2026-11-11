import { cn, InputGroup, InputGroupAddon, InputGroupInput } from "@kits/shadcn-ui";
import { AlertTriangle, KeyRound, Search, ShieldCheck } from "lucide-react";
import { DOMAINS, ROLES, VENDORS, levelOf, roleLabel } from "./data.js";

const DOMAIN_ORDER = ["atm", "cam", "parameter", "vendor"];

function Row({ active, onClick, title, meta, count, tone = "default", icon: Icon }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-current={active ? "true" : undefined}
      className={cn(
        "group flex w-full items-center gap-2.5 rounded-[4px] px-2.5 py-2 text-left transition-colors duration-[120ms]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--brand)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--surface)]",
        active
          ? "bg-[var(--brand-tint)] text-[var(--brand-text)]"
          : "text-[var(--ink)] hover:bg-[var(--surface-sunken)]",
      )}
    >
      {Icon ? (
        <Icon
          className={cn(
            "size-4 shrink-0",
            tone === "warning" && !active ? "text-[var(--warning)]" : "",
            active ? "text-[var(--brand-text)]" : "text-[var(--ink-muted)]",
          )}
          strokeWidth={2}
        />
      ) : null}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium leading-tight">{title}</span>
        {meta ? (
          <span
            className={cn(
              "mt-0.5 block truncate text-xs",
              active ? "text-[var(--brand-text)]/75" : "text-[var(--ink-secondary)]",
            )}
          >
            {meta}
          </span>
        ) : null}
      </span>
      <span
        className={cn(
          "amount shrink-0 text-xs",
          tone === "warning" && count > 0 && !active
            ? "text-[var(--warning)]"
            : active
              ? "text-[var(--brand-text)]"
              : "text-[var(--ink-secondary)]",
        )}
      >
        {count}
      </span>
    </button>
  );
}

function GroupLabel({ children, hint }) {
  return (
    <div className="flex items-baseline justify-between px-2.5 pb-1.5 pt-5 first:pt-1">
      <span className="label-xs text-[var(--ink-secondary)]">{children}</span>
      {hint ? <span className="text-xs text-[var(--ink-secondary)]">{hint}</span> : null}
    </div>
  );
}

export default function Rail({ users, selected, onSelect, query, setQuery, unassignedCount }) {
  const supervisors = users.filter((u) => levelOf(u) === "checker");
  const outside = users.filter((u) => levelOf(u) === "admin");
  const q = query.trim().toLowerCase();

  const teamOf = (spv) => users.filter((u) => u.supervisorId === spv.id);
  const matches = (spv) => {
    if (!q) return true;
    if (spv.name.toLowerCase().includes(q) || spv.username.toLowerCase().includes(q)) return true;
    return teamOf(spv).some(
      (m) => m.name.toLowerCase().includes(q) || m.username.toLowerCase().includes(q),
    );
  };

  return (
    <nav
      aria-label="Daftar supervisor"
      className="flex max-h-[42vh] min-h-0 shrink-0 flex-col border-b border-[var(--border-c)] bg-[var(--surface)] md:h-full md:max-h-none md:w-[288px] md:border-b-0 md:border-r"
    >
      <div className="border-b border-[var(--border-c)] p-3">
        <InputGroup className="h-9 rounded-[4px] bg-[var(--surface)]">
          <InputGroupAddon>
            <Search className="text-[var(--ink-muted)]" />
          </InputGroupAddon>
          <InputGroupInput
            aria-label="Cari supervisor atau anak buah"
            placeholder="Nama atau username"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </InputGroup>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-6">
        <GroupLabel>Perlu tindakan</GroupLabel>
        <Row
          icon={AlertTriangle}
          tone="warning"
          active={selected.type === "unassigned"}
          onClick={() => onSelect({ type: "unassigned" })}
          title="Tanpa atasan"
          meta="Maker yang belum punya checker"
          count={unassignedCount}
        />

        {DOMAIN_ORDER.map((domain) => {
          const list = supervisors
            .filter((s) => ROLES[s.role].domain === domain)
            .filter(matches)
            .sort((a, b) => a.name.localeCompare(b.name));
          if (!list.length) return null;
          return (
            <div key={domain}>
              <GroupLabel hint={DOMAINS[domain].label === "Vendor" ? "per PJPUR" : null}>
                {DOMAINS[domain].label}
              </GroupLabel>
              <div className="flex flex-col gap-0.5">
                {list.map((spv) => (
                  <Row
                    key={spv.id}
                    icon={ShieldCheck}
                    active={selected.type === "spv" && selected.id === spv.id}
                    onClick={() => onSelect({ type: "spv", id: spv.id })}
                    title={spv.name}
                    meta={
                      spv.vendor
                        ? `${roleLabel(spv.role)} · ${VENDORS[spv.vendor]}`
                        : roleLabel(spv.role)
                    }
                    count={teamOf(spv).length}
                  />
                ))}
              </div>
            </div>
          );
        })}

        <GroupLabel>Di luar rantai</GroupLabel>
        <Row
          icon={KeyRound}
          active={selected.type === "admin"}
          onClick={() => onSelect({ type: "admin" })}
          title="Admin & App Support"
          meta="Tidak butuh atasan"
          count={outside.length}
        />
      </div>
    </nav>
  );
}
