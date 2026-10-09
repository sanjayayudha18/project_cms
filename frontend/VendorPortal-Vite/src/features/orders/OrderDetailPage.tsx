import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import type { PartyAtm, VendorOrderDetail } from "@/features/orders/api";
import { ROLE_LABEL, denomLabel, formatDate, orderStatus } from "@/features/orders/labels";
import { useDecideOrder, useOrder } from "@/features/orders/useOrders";
import type { ApiError } from "@/lib/api/client";
import { formatIDR } from "@/lib/formatters";
import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useState } from "react";

// cit-send-vendor FR3/FR4/FR9.2: one party of a replenish request. A replenish
// branch sees where to collect cash (vault branch + address); a vault branch
// sees who collects and how much to prepare per denom. Accept / reject only
// while the request is with the vendors (can_decide, re-checked server-side).

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
  const denoms = order.content.totals.map((t) => t.denom);

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
          {order.content.totals.map((t) => (
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

      <section aria-labelledby="atms-heading" className="flex flex-col gap-3">
        <h2 id="atms-heading" className="text-base font-semibold text-surface-text">
          ATM ({order.atm_count})
        </h2>
        <div className="overflow-x-auto rounded-lg border border-neutral-200 bg-white">
          <table className="w-full text-sm">
            <thead className="bg-neutral-50 text-left text-xs uppercase tracking-wide text-neutral-500">
              <tr>
                <th scope="col" className="px-3 py-2">
                  Terminal
                </th>
                <th scope="col" className="px-3 py-2">
                  Lokasi
                </th>
                {!isVault && (
                  <th scope="col" className="px-3 py-2">
                    Tiket
                  </th>
                )}
                {denoms.map((d) => (
                  <th key={d} scope="col" className="px-3 py-2 text-right">
                    {denomLabel(d)} (IDR)
                  </th>
                ))}
                <th scope="col" className="px-3 py-2">
                  {isVault ? "Diambil oleh (replenish)" : "Ambil uang di (vault)"}
                </th>
              </tr>
            </thead>
            <tbody>
              {order.content.atms.map((atm) => (
                <AtmRow key={atm.terminal_id} atm={atm} denoms={denoms} showTicket={!isVault} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {order.can_decide && <DecisionPanel partyID={order.id} />}
    </>
  );
}

function AtmRow({
  atm,
  denoms,
  showTicket,
}: { readonly atm: PartyAtm; readonly denoms: readonly number[]; readonly showTicket: boolean }) {
  const amountOf = (d: number) => atm.denoms.find((x) => x.denom === d)?.amount;
  const c = atm.counterpart;
  return (
    <tr className="border-t border-neutral-100">
      <td className="px-3 py-2 font-medium">{atm.terminal_id}</td>
      <td className="px-3 py-2">{atm.lokasi || "-"}</td>
      {showTicket && <td className="px-3 py-2 tabular-nums">{atm.ticket_number || "-"}</td>}
      {denoms.map((d) => {
        const amount = amountOf(d);
        return (
          <td key={d} className="px-3 py-2 text-right tabular-nums">
            {amount === undefined ? "-" : amount.toLocaleString("id-ID")}
          </td>
        );
      })}
      <td className="px-3 py-2">
        <span className="font-medium">{c.name}</span> · {c.vendor_name}
        {c.address && <span className="block text-xs text-neutral-500">{c.address}</span>}
      </td>
    </tr>
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
