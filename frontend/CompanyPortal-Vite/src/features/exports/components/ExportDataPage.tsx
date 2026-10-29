import { Card } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { type ReactNode, useState } from "react";
import { useExportFlow } from "../hooks";
import type { ExportRequestParams } from "../types";
import { ExportProgressDialog } from "./ExportProgressDialog";
import { MyExportsTable } from "./MyExportsTable";
import { PackagePriceExportForm } from "./PackagePriceExportForm";
import { VendorExportForm } from "./VendorExportForm";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card>
      <section aria-label={title} className="flex flex-col gap-4">
        <h2 className="text-lg font-semibold text-[var(--n-900)]">{title}</h2>
        {children}
      </section>
    </Card>
  );
}

type Source = "vendor" | "harga";

export function ExportDataPage() {
  const { request, dialogProps, requestError, isRequesting } = useExportFlow();
  // The shared flow keeps one error; show it only beside the form that made the request.
  const [source, setSource] = useState<Source | null>(null);
  const from = (s: Source) => (p: ExportRequestParams) => {
    setSource(s);
    void request(p);
  };
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Data master"
        title="Export Data"
        description="Minta file ekspor data vendor dan harga paket. File siap diunduh selama 10 menit."
      />
      <Section title="Data Vendor">
        <VendorExportForm
          onRequest={from("vendor")}
          isRequesting={isRequesting}
          error={source === "vendor" ? requestError : null}
        />
      </Section>
      <Section title="Harga Paket">
        <PackagePriceExportForm
          onRequest={from("harga")}
          isRequesting={isRequesting}
          error={source === "harga" ? requestError : null}
        />
      </Section>
      <Section title="Ekspor Saya">
        <MyExportsTable />
      </Section>
      <ExportProgressDialog {...dialogProps} />
    </div>
  );
}
