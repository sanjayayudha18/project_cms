import { PageHeader } from "@/components/ui/PageHeader";
import { useAuthStore } from "@/lib/auth/store";
import { useState } from "react";
import { AnomalyTab } from "./AnomalyTab";
import { AreasTab } from "./AreasTab";
import { CategoryParamsTab } from "./CategoryParamsTab";
import { ChecklistTab } from "./ChecklistTab";

type TabId = "category" | "anomaly" | "checklist" | "area";

/**
 * Each tab is readable by the roles its API allows (ADMIN reads everything) and
 * writable only by its maker role -- the service re-checks (AC-5.1-14), this
 * just hides the buttons.
 */
const TABS: { id: TabId; label: string; maker: "CC-CWM" | "CC-CCR" }[] = [
  { id: "category", label: "Kategori Vault", maker: "CC-CWM" },
  { id: "anomaly", label: "Kategori ATM Anomaly", maker: "CC-CCR" },
  { id: "checklist", label: "Checklist BA", maker: "CC-CCR" },
  { id: "area", label: "Area", maker: "CC-CWM" },
];

export function CashCountParametersPage() {
  const role = useAuthStore((s) => s.user?.role);
  const visible = TABS.filter((t) => role === "ADMIN" || role === t.maker);
  const [selected, setSelected] = useState<TabId>(visible[0]?.id ?? "category");
  const active = visible.find((t) => t.id === selected) ?? visible[0];
  if (!active) return null;
  const canWrite = role === active.maker;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Perhitungan Kas"
        title="Parameter Cash Count"
        description="Setiap perubahan diajukan lewat maker-checker dan baru berlaku setelah disetujui."
      />
      <div
        role="tablist"
        aria-label="Parameter Cash Count"
        className="flex gap-1 overflow-x-auto border-b border-[var(--n-200)]"
      >
        {visible.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            id={`cc-tab-${t.id}`}
            aria-selected={active.id === t.id}
            aria-controls="cc-tabpanel"
            onClick={() => setSelected(t.id)}
            className={`min-h-[44px] whitespace-nowrap px-4 py-2 text-sm font-medium ${
              active.id === t.id
                ? "border-b-2 border-[var(--red-600)] text-[var(--n-900)]"
                : "text-[var(--n-500)]"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div role="tabpanel" id="cc-tabpanel" aria-labelledby={`cc-tab-${active.id}`}>
        {active.id === "category" && <CategoryParamsTab canWrite={canWrite} />}
        {active.id === "anomaly" && <AnomalyTab canWrite={canWrite} />}
        {active.id === "checklist" && <ChecklistTab canWrite={canWrite} />}
        {active.id === "area" && <AreasTab canWrite={canWrite} />}
      </div>
    </div>
  );
}
