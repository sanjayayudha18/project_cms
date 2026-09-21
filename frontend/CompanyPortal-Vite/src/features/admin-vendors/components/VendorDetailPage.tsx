import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable } from "@/components/ui/DataTable";
import { PageHeader } from "@/components/ui/PageHeader";
import { Link } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle, XCircle } from "lucide-react";
import { useState } from "react";
import type { VendorChildKind } from "../api";
import { useVendor, useVendorChildren } from "../hooks";

type Row = Record<string, unknown>;

const statusCol: ColumnDef<Row, unknown> = {
  id: "status",
  header: "Status",
  cell: ({ row }) =>
    row.original.is_active ? (
      <Badge variant="success" icon={CheckCircle} label="Aktif" />
    ) : (
      <Badge variant="danger" icon={XCircle} label="Nonaktif" />
    ),
};

const col = (key: string, header: string): ColumnDef<Row, unknown> => ({
  accessorKey: key,
  header,
  cell: ({ getValue }) => {
    const v = getValue();
    return v === null || v === undefined || v === "" ? "—" : String(v);
  },
});

const yesNo = (key: string, header: string): ColumnDef<Row, unknown> => ({
  accessorKey: key,
  header,
  cell: ({ getValue }) => (getValue() ? "Ya" : "Tidak"),
});

const money = (key: string, header: string): ColumnDef<Row, unknown> => ({
  accessorKey: key,
  header,
  meta: { align: "right" },
  cell: ({ getValue }) => <span className="tabular-nums">IDR {String(getValue() ?? "—")}</span>,
});

const TABS: { id: "info" | VendorChildKind; label: string; columns?: ColumnDef<Row, unknown>[] }[] =
  [
    { id: "info", label: "Info" },
    {
      id: "branches",
      label: "Cabang",
      columns: [
        col("branch_code", "Kode"),
        col("branch_name", "Nama"),
        col("region", "Wilayah"),
        statusCol,
      ],
    },
    {
      id: "vaults",
      label: "Vault",
      columns: [
        col("vault_code", "Kode"),
        col("category", "Kategori"),
        money("max_capacity_amount", "Kapasitas maks"),
        col("operating_hours", "Jam operasional"),
        statusCol,
      ],
    },
    {
      id: "pics",
      label: "PIC",
      columns: [
        col("name", "Nama"),
        col("position", "Jabatan"),
        col("phone", "Telepon"),
        col("email", "Email"),
        yesNo("is_notification_recipient", "Penerima notifikasi"),
        statusCol,
      ],
    },
    {
      id: "packages",
      label: "Paket",
      columns: [
        col("code", "Kode"),
        col("priority_class", "Kelas prioritas"),
        money("price", "Harga"),
        statusCol,
      ],
    },
  ];

function ChildTab({
  vendorId,
  kind,
  columns,
}: { vendorId: number; kind: VendorChildKind; columns: ColumnDef<Row, unknown>[] }) {
  const q = useVendorChildren(vendorId, kind);
  if (q.isLoading) return <p className="text-sm text-[var(--n-500)]">Memuat…</p>;
  if (q.isError)
    return (
      <p role="alert" className="text-sm text-[var(--danger-fg)]">
        Gagal memuat data
      </p>
    );
  return <DataTable data={q.data ?? []} columns={columns} />;
}

/**
 * Vendor detail: Info + read-only Cabang / Vault / PIC / Paket tabs.
 * Changes to child records go through CSV import (T6.5) or the API; each is
 * still a maker-checker change request (plan D1).
 */
export function VendorDetailPage({ vendorId }: { vendorId: number }) {
  const [tab, setTab] = useState<(typeof TABS)[number]["id"]>("info");
  const vendorQuery = useVendor(vendorId);
  const vendor = vendorQuery.data;
  const active = TABS.find((t) => t.id === tab);

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        eyebrow="Manajemen Vendor"
        title={vendor ? `${vendor.code} · ${vendor.name}` : "Detail Vendor"}
        description="Data vendor beserta cabang, vault, PIC, dan paketnya."
      />
      <Link to="/settings/admin/vendors" search={{}}>
        <Button variant="ghost">← Kembali ke daftar</Button>
      </Link>

      <div
        role="tablist"
        aria-label="Detail vendor"
        className="flex gap-1 border-b border-[var(--n-200)]"
      >
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
            className={`px-4 py-2 text-sm font-medium ${
              tab === t.id
                ? "border-b-2 border-[var(--red-600)] text-[var(--n-900)]"
                : "text-[var(--n-500)]"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {vendorQuery.isError && (
        <p role="alert" className="text-sm text-[var(--danger-fg)]">
          Vendor tidak ditemukan
        </p>
      )}

      {tab === "info" && vendor && (
        <dl className="grid max-w-2xl grid-cols-[12rem_1fr] gap-x-4 gap-y-3 text-sm">
          {(
            [
              ["Kode", vendor.code],
              ["Nama", vendor.name],
              ["Nama badan hukum", vendor.legal_name],
              ["NPWP", vendor.npwp],
              ["Email kontak", vendor.contact_email],
              ["Telepon", vendor.contact_phone],
              ["Alamat kantor pusat", vendor.hq_address],
            ] as const
          ).map(([label, value]) => (
            <div key={label} className="contents">
              <dt className="font-medium text-[var(--n-600)]">{label}</dt>
              <dd className="text-[var(--n-900)]">{value || "—"}</dd>
            </div>
          ))}
          <dt className="font-medium text-[var(--n-600)]">Status</dt>
          <dd>
            {vendor.is_active ? (
              <Badge variant="success" icon={CheckCircle} label="Aktif" />
            ) : (
              <Badge variant="danger" icon={XCircle} label="Nonaktif" />
            )}
          </dd>
        </dl>
      )}

      {active?.columns && (
        <ChildTab
          vendorId={vendorId}
          kind={active.id as VendorChildKind}
          columns={active.columns}
        />
      )}
    </div>
  );
}
