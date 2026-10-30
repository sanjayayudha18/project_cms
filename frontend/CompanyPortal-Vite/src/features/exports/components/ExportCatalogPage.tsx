import { Card } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { useAuthStore } from "@/lib/auth/store";
import { Link } from "@tanstack/react-router";
import { ChevronRight, FileBarChart, Table2 } from "lucide-react";
import { type ExportDataset, type MasterDataset, datasetsForRole } from "../datasets";
import { MyExportsTable } from "./MyExportsTable";

const ROW_CLASS =
  "group flex items-center gap-4 rounded-md px-3 py-3 hover:bg-[var(--n-50)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-[var(--red-600)]";

/** One-line summary of the filters a master dataset offers (Req 1.3). */
function filterSummary(d: MasterDataset): string {
  const parts: string[] = [];
  if (d.filters.vendor === "required") parts.push("Vendor (wajib)");
  else if (d.filters.vendor === "optional") parts.push("Vendor");
  if (d.filters.branch) parts.push("Cabang");
  if (d.filters.priceLevel) parts.push("Tingkat harga");
  parts.push("Status");
  return `Filter: ${parts.join(", ")}`;
}

function RowBody({ dataset }: { dataset: ExportDataset }) {
  const Icon = dataset.group === "master" ? Table2 : FileBarChart;
  return (
    <>
      <Icon aria-hidden="true" className="size-5 shrink-0 text-[var(--n-500)]" />
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-sm font-semibold text-[var(--n-900)]">{dataset.label}</span>
        <span className="text-xs text-[var(--n-600)]">{dataset.description}</span>
        <span className="text-xs text-[var(--n-500)]">
          {dataset.group === "master" ? filterSummary(dataset) : "Buka layar laporan"}
        </span>
      </span>
      <ChevronRight
        aria-hidden="true"
        className="size-4 shrink-0 text-[var(--n-400)] group-hover:text-[var(--n-700)]"
      />
    </>
  );
}

function Group({ title, datasets }: { title: string; datasets: ExportDataset[] }) {
  if (datasets.length === 0) return null;
  const headingId = `export-group-${title.replace(/\s+/g, "-").toLowerCase()}`;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2">
      <h2 id={headingId} className="text-lg font-semibold text-[var(--n-900)]">
        {title}
      </h2>
      <ul className="flex flex-col divide-y divide-[var(--n-200)]">
        {datasets.map((d) => (
          <li key={d.id}>
            {d.group === "master" ? (
              <Link to="/exports/$dataset" params={{ dataset: d.id }} className={ROW_CLASS}>
                <RowBody dataset={d} />
              </Link>
            ) : (
              <Link to={d.href} className={ROW_CLASS}>
                <RowBody dataset={d} />
              </Link>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

/** `/exports`: catalog of datasets the signed-in role may export, plus the user's own export jobs. */
export function ExportCatalogPage() {
  const role = useAuthStore((s) => s.user?.role ?? null);
  const datasets = role ? datasetsForRole(role) : [];
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Ekspor"
        title="Export Data"
        description="Pilih data yang akan diekspor. Data master dipratinjau dulu sebelum diekspor."
      />
      <Card>
        <div className="flex flex-col gap-6">
          <Group title="Data master" datasets={datasets.filter((d) => d.group === "master")} />
          <Group title="Laporan" datasets={datasets.filter((d) => d.group === "report")} />
        </div>
      </Card>
      <Card>
        <section aria-labelledby="export-mine" className="flex flex-col gap-4">
          <h2 id="export-mine" className="text-lg font-semibold text-[var(--n-900)]">
            Ekspor Saya
          </h2>
          <MyExportsTable />
        </section>
      </Card>
    </div>
  );
}
