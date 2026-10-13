import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import type {
  DenomAmount,
  PartyAtm,
  PartyBranch,
  PartyVault,
  VendorOrderDetail,
} from "@/features/orders/api";
import { ROLE_LABEL, denomLabel, formatDate, orderStatus } from "@/features/orders/labels";
import { useDecideOrder, useOrder } from "@/features/orders/useOrders";
import type { ApiError } from "@/lib/api/client";
import { formatIDR } from "@/lib/formatters";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useState } from "react";

// cit-send-vendor FR3/FR4/FR9.2 + cit-vault-select FR7/FR9.4: one party of a
// replenish request. A replenish branch sees its ATMs and every vault of the
// request (ticket, address, amount) to collect from; a vault branch sees its own
// vaults (ticket + amount to prepare) and the replenish branches that collect.
// Accept / reject only while the request is with the vendors (can_decide,
// re-checked server-side).

const REASON_MIN = 10;
const REASON_MAX = 500;

function errorMessage(err: unknown): string {
  const e = err as Partial<ApiError> | null;
  if (e?.status === 409)
    return "Request ini sudah tidak dapat diterima atau ditolak. Muat ulang halaman.";
  return e?.message ?? "Terjadi kesalahan. Coba lagi.";
}

export function OrderDetailPage() {
  const { id } = useParams({ strict: false }) as { id?: string };
  const partyID = Number(id);
  const { data, isLoading, isError } = useOrder(partyID);

  return (
    <div className="flex flex-col gap-6">
      <Link
        to="/orders"
        className="inline-flex items-center gap-1 text-sm text-sidebar-active hover:underline"
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        Kembali ke daftar
      </Link>
      {isLoading ? (
        <p className="text-sm text-neutral-500">Memuat data...</p>
      ) : isError || !data ? (
        <p role="alert" className="text-sm font-semibold text-danger-fg">
          Request tidak ditemukan atau Anda tidak berhak melihatnya.
        </p>
      ) : (
        <OrderDetail order={data} />
      )}
    </div>
  );
}

function OrderDetail({ order }: { readonly order: VendorOrderDetail }) {
  const status = orderStatus(order);
  const isVault = order.role === "vault";
  const { content } = order;

  return (
    <>
      <header className="flex flex-col gap-2">
        <h1 className="text-xl font-semibold text-surface-text">{order.request_number}</h1>
        <div className="flex flex-wrap items-center gap-3 text-sm text-neutral-600">
          <Badge variant={status.variant}>{status.label}</Badge>
          <span>{ROLE_LABEL[order.role]}</span>
          <span>Branch {order.branch_name}</span>
          <span>Tanggal replenish {formatDate(order.replenish_date)}</span>
        </div>
        {order.rejection_reason && (
          <p className="text-sm">
            <span className="font-semibold">Alasan penolakan: </span>
            {order.rejection_reason}
          </p>
        )}
      </header>

      <section aria-labelledby="totals-heading" className="flex flex-col gap-3">
        <h2 id="totals-heading" className="text-base font-semibold text-surface-text">
          {isVault ? "Total uang yang harus disiapkan" : "Total order"}
        </h2>
        <div className="flex flex-wrap gap-3">
          {content.totals.map((t) => (
            <Card key={t.denom} className="min-w-[160px]">
              <p className="text-xs font-medium uppercase tracking-wide text-neutral-500">
                Denom {denomLabel(t.denom)}
              </p>
              <p className="text-lg font-semibold tabular-nums text-surface-text">
                {formatIDR(t.amount)}
              </p>
            </Card>
          ))}
        </div>
      </section>

      {!isVault && (
        <section aria-labelledby="atms-heading" className="flex flex-col gap-3">
          <h2 id="atms-heading" className="text-base font-semibold text-surface-text">
            ATM ({content.atms.length})
          </h2>
          <AtmTable atms={content.atms} />
        </section>
      )}

      <section aria-labelledby="vaults-heading" className="flex flex-col gap-3">
        <h2 id="vaults-heading" className="text-base font-semibold text-surface-text">
          {isVault ? "Vault Anda" : "Ambil uang di vault"} ({content.vaults.length})
        </h2>
        <VaultTable vaults={content.vaults} showBranch={!isVault} />
      </section>

      {isVault && <CollectorList branches={content.replenish_branches} />}

      {order.can_decide && <DecisionPanel partyID={order.id} />}
    </>
  );
}

/** Denoms present in any row, high to low. */
function denomsOf(rows: readonly { readonly denoms: readonly DenomAmount[] }[]): number[] {
  return [...new Set(rows.flatMap((r) => r.denoms.map((d) => d.denom)))].sort((a, b) => b - a);
}

const thClass = "px-3 py-2";
const tableWrap = "overflow-x-auto rounded-lg border border-neutral-200 bg-white";
const theadClass = "bg-neutral-50 text-left text-xs uppercase tracking-wide text-neutral-500";

function DenomHeaders({ denoms }: { readonly denoms: readonly number[] }) {
  return denoms.map((d) => (
    <th key={d} scope="col" className={`${thClass} text-right`}>
      {denomLabel(d)} (IDR)
    </th>
  ));
}

function AmountCells({
  denoms,
  amounts,
}: { readonly denoms: readonly number[]; readonly amounts: readonly DenomAmount[] }) {
  return denoms.map((d) => {
    const amount = amounts.find((x) => x.denom === d)?.amount;
    return (
      <td key={d} className="px-3 py-2 text-right tabular-nums">
        {amount === undefined ? "-" : amount.toLocaleString("id-ID")}
      </td>
    );
  });
}

function AtmTable({ atms }: { readonly atms: readonly PartyAtm[] }) {
  const denoms = denomsOf(atms);
  return (
    <div className={tableWrap}>
      <table className="w-full text-sm">
        <thead className={theadClass}>
          <tr>
            <th scope="col" className={thClass}>
              Terminal
            </th>
            <th scope="col" className={thClass}>
              Lokasi
            </th>
            <th scope="col" className={thClass}>
              Tiket
            </th>
            <DenomHeaders denoms={denoms} />
          </tr>
        </thead>
        <tbody>
          {atms.map((atm) => (
            <tr key={atm.terminal_id} className="border-t border-neutral-100">
              <td className="px-3 py-2 font-medium">{atm.terminal_id}</td>
              <td className="px-3 py-2">{atm.lokasi || "-"}</td>
              <td className="px-3 py-2 tabular-nums">{atm.ticket_number || "-"}</td>
              <AmountCells denoms={denoms} amounts={atm.denoms} />
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function VaultTable({
  vaults,
  showBranch,
}: { readonly vaults: readonly PartyVault[]; readonly showBranch: boolean }) {
  const denoms = denomsOf(vaults);
  return (
    <div className={tableWrap}>
      <table className="w-full text-sm">
        <thead className={theadClass}>
          <tr>
            <th scope="col" className={thClass}>
              Vault
            </th>
            <th scope="col" className={thClass}>
              Tiket vault
            </th>
            {showBranch && (
              <th scope="col" className={thClass}>
                Branch vault
              </th>
            )}
            <th scope="col" className={thClass}>
              Alamat
            </th>
            <DenomHeaders denoms={denoms} />
          </tr>
        </thead>
        <tbody>
          {vaults.map((v) => (
            <tr key={v.vault.id} className="border-t border-neutral-100">
              <td className="px-3 py-2 font-medium">{v.vault.code}</td>
              <td className="px-3 py-2 tabular-nums">{v.ticket_number || "-"}</td>
              {showBranch && (
                <td className="px-3 py-2">
                  <span className="font-medium">{v.branch.name}</span> · {v.branch.vendor_name}
                </td>
              )}
              <td className="px-3 py-2">{v.vault.address || "-"}</td>
              <AmountCells denoms={denoms} amounts={v.denoms} />
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Vault party: the replenish branches that collect from its vaults. */
function CollectorList({ branches }: { readonly branches: readonly PartyBranch[] }) {
  return (
    <section aria-labelledby="collectors-heading" className="flex flex-col gap-2">
      <h2 id="collectors-heading" className="text-base font-semibold text-surface-text">
        Diambil oleh (cabang pengisi)
      </h2>
      <ul className="flex flex-col gap-1 text-sm">
        {branches.map((b) => (
          <li key={b.id}>
            <span className="font-medium">{b.name}</span> ({b.code}) · {b.vendor_name}
          </li>
        ))}
      </ul>
    </section>
  );
}

function DecisionPanel({ partyID }: { readonly partyID: number }) {
  const decide = useDecideOrder(partyID);
  const [isRejecting, setIsRejecting] = useState(false);
  const [reason, setReason] = useState("");
  const trimmed = reason.trim().length;
  const reasonValid = trimmed >= REASON_MIN && trimmed <= REASON_MAX;

  return (
    <section
      aria-labelledby="decision-heading"
      className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-4"
    >
      <h2 id="decision-heading" className="text-base font-semibold text-surface-text">
        Keputusan
      </h2>
      {decide.isError && (
        <p role="alert" className="text-sm font-semibold text-danger-fg">
          {errorMessage(decide.error)}
        </p>
      )}
      {isRejecting ? (
        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (reasonValid) decide.mutate(reason.trim());
          }}
        >
          <label htmlFor="reject-reason" className="text-sm font-medium">
            Alasan penolakan (min. {REASON_MIN} karakter)
          </label>
          <textarea
            id="reject-reason"
            value={reason}
            maxLength={REASON_MAX}
            rows={3}
            onChange={(e) => setReason(e.target.value)}
            className="rounded-md border border-neutral-300 p-2 text-sm"
          />
          <p className="text-xs tabular-nums text-neutral-500">
            {trimmed}/{REASON_MAX}
          </p>
          <div className="flex gap-2">
            <Button
              type="submit"
              variant="danger"
              disabled={!reasonValid}
              isLoading={decide.isPending}
            >
              Kirim penolakan
            </Button>
            <Button type="button" variant="secondary" onClick={() => setIsRejecting(false)}>
              Batal
            </Button>
          </div>
        </form>
      ) : (
        <div className="flex gap-2">
          <Button onClick={() => decide.mutate(null)} isLoading={decide.isPending}>
            Terima
          </Button>
          <Button
            variant="secondary"
            onClick={() => setIsRejecting(true)}
            disabled={decide.isPending}
          >
            Tolak
          </Button>
        </div>
      )}
    </section>
  );
}
