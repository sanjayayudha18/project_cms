import { ModuleCard, type ModuleCardProps } from "@/components/ui/ModuleCard";
import { createRoute } from "@tanstack/react-router";
import { Calculator, FileOutput, GitCompare, Receipt } from "lucide-react";
import { protectedRoute } from "../_protected";

export const invoiceRoute = createRoute({
  path: "/invoice",
  getParentRoute: () => protectedRoute,
  component: InvoicePage,
});

const INVOICE_CARDS: ModuleCardProps[] = [
  {
    id: "invoice-upload",
    title: "Unggah Invoice",
    description: "Unggah rekap CIT dan CPC beserta dokumen pendukung dari vendor.",
    href: "/invoice/upload",
    icon: Receipt,
    disabled: true,
  },
  {
    id: "reconciliation",
    title: "Rekonsiliasi",
    description: "Pengecekan otomatis item baris CIT/CPC terhadap data sistem.",
    href: "/invoice/reconciliation",
    icon: GitCompare,
    disabled: true,
  },
  {
    id: "charge-calc",
    title: "Perhitungan Beban",
    description:
      "Hitung beban nasabah, beban bank, dan total invoice berdasarkan perjanjian kerja sama.",
    href: "/invoice/charge-calculation",
    icon: Calculator,
    disabled: true,
  },
  {
    id: "doc-gen",
    title: "Pembuatan Dokumen",
    description: "Buat surat konfirmasi pendebetan dan memo pendebetan secara otomatis.",
    href: "/invoice/documents",
    icon: FileOutput,
    disabled: true,
  },
];

function InvoicePage() {
  return (
    <div className="flex flex-col gap-[var(--space-6)]">
      <div className="flex flex-col gap-[var(--space-2)]">
        <h1 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
          Tagihan
        </h1>
        <p className="text-sm" style={{ color: "var(--n-600)" }}>
          Kelola proses pembayaran invoice vendor mulai dari unggah dokumen hingga pembuatan surat
          pendebetan.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-[var(--space-4)] sm:grid-cols-2 lg:grid-cols-3">
        {INVOICE_CARDS.map((card) => (
          <ModuleCard key={card.id} {...card} />
        ))}
      </div>
    </div>
  );
}
