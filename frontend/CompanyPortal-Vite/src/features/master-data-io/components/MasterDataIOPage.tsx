import { PageHeader } from "@/components/ui/PageHeader";
import { type KeyboardEvent, useRef } from "react";
import { type MasterDataIoTab, useMasterDataIoTab } from "../useMasterDataIoTab";
import { ImportBatchDialog } from "./ImportBatchDialog";
import { ImportHistoryTab } from "./ImportHistoryTab";
import { ImportWizard } from "./ImportWizard";

const TABS: readonly { id: MasterDataIoTab; label: string }[] = [
  { id: "new", label: "Impor baru" },
  { id: "history", label: "Riwayat impor" },
];

const tabId = (id: MasterDataIoTab) => `mdio-tab-${id}`;
const panelId = (id: MasterDataIoTab) => `mdio-panel-${id}`;

/**
 * Halaman Impor CSV Data Master: shell dengan dua tab ("Impor baru" dan
 * "Riwayat impor") yang terhubung ke `?tab`. Mekanisme persetujuan tidak berubah:
 * file hanya diajukan, dan seluruhnya menunggu SATU persetujuan.
 */
export function MasterDataIOPage() {
  const { tab, setTab } = useMasterDataIoTab();
  const refs = useRef<Record<MasterDataIoTab, HTMLButtonElement | null>>({
    new: null,
    history: null,
  });

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>, index: number): void {
    let next = index;
    if (e.key === "ArrowRight") next = (index + 1) % TABS.length;
    else if (e.key === "ArrowLeft") next = (index - 1 + TABS.length) % TABS.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = TABS.length - 1;
    else return;
    e.preventDefault();
    const target = TABS[next]?.id;
    if (!target) return;
    setTab(target);
    refs.current[target]?.focus();
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Data master"
        title="Impor CSV Data Master"
        description="Unduh templat, ubah di spreadsheet, lalu impor. Data baru berubah setelah pengajuan disetujui."
      />

      <div role="tablist" aria-label="Tampilan impor" className="flex gap-2">
        {TABS.map((t, i) => (
          <button
            key={t.id}
            ref={(el) => {
              refs.current[t.id] = el;
            }}
            id={tabId(t.id)}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            aria-controls={panelId(t.id)}
            tabIndex={tab === t.id ? 0 : -1}
            onClick={() => setTab(t.id)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={`min-h-[44px] rounded-[var(--radius-md)] border px-4 text-sm font-medium ${
              tab === t.id
                ? "border-[var(--red-500)] bg-[var(--red-50)] text-[var(--red-700)]"
                : "border-[var(--n-300)] text-[var(--n-700)]"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div id={panelId(tab)} role="tabpanel" aria-labelledby={tabId(tab)}>
        {tab === "new" ? <ImportWizard /> : <ImportHistoryTab />}
      </div>
      {tab === "history" && <ImportBatchDialog />}
    </div>
  );
}
