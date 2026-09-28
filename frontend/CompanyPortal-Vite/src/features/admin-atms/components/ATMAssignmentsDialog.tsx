import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle, XCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { listVendorChildren, listVendorPackages, listVendors } from "../../admin-vendors/api";
import { pendingApprovalMessage } from "../../master-data/changeRequest";
import { type ATMAssignment, createATMAssignment, listATMAssignments } from "../api";
import { useUpdateATM } from "../hooks";
import type { AdminATM, UpdateATMPayload } from "../types";

const inputClass =
  "min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)]";

/**
 * The period the form is prefilled from: the active one covering `today`,
 * else the most recent active one, else none. `today` is YYYY-MM-DD.
 */
export function currentAssignment(list: ATMAssignment[], today: string): ATMAssignment | undefined {
  const active = list.filter((a) => a.is_active);
  return (
    active.find(
      (a) =>
        a.effective_start_date <= today &&
        (a.effective_end_date === null || a.effective_end_date >= today),
    ) ?? active[0]
  );
}

const idStr = (n: number | null | undefined) => (n ? String(n) : "");

interface Props {
  atm: AdminATM | null;
  onClose: () => void;
}

/**
 * Kelolaan ATM: the effective-dated periods in which a vendor package manages
 * this ATM, plus a form to assign a new package. Dates are set by the system
 * at approval (D1): the new period starts that day, open-ended, and the
 * running period is closed the day before.
 */
export function ATMAssignmentsDialog({ atm, onClose }: Props) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const atmId = atm?.id ?? 0;
  const [vendorId, setVendorId] = useState("");
  const [branchId, setBranchId] = useState("");
  const [packageId, setPackageId] = useState("");
  const [escrowAccount, setEscrowAccount] = useState(atm?.escrow_account ?? "");
  // Which ATM the form was last prefilled for; null = prefill on next data.
  const [prefilledFor, setPrefilledFor] = useState<number | null>(null);
  const updateATM = useUpdateATM();

  // biome-ignore lint/correctness/useExhaustiveDependencies: reset only when the dialog (re)opens for a given atm
  useEffect(() => {
    setEscrowAccount(atm?.escrow_account ?? "");
    setPrefilledFor(null);
  }, [atm?.id]);

  const list = useQuery({
    queryKey: ["admin-atms", "assignments", atmId],
    queryFn: () => listATMAssignments(atmId),
    enabled: atm !== null,
  });

  // Prefill the form with the current kelolaan; fields without data stay empty.
  useEffect(() => {
    if (!list.data || atmId === 0 || prefilledFor === atmId) return;
    const cur = currentAssignment(list.data, new Date().toLocaleDateString("en-CA"));
    setVendorId(idStr(cur?.vendor_id));
    setBranchId(idStr(cur?.vendor_branch_id));
    setPackageId(idStr(cur?.vendor_package_id));
    setPrefilledFor(atmId);
  }, [list.data, atmId, prefilledFor]);
  const vendors = useQuery({
    queryKey: ["admin-vendors", "options"],
    queryFn: () => listVendors({ page: 1, page_size: 100, status: "active" }),
    enabled: atm !== null,
  });
  const branches = useQuery({
    queryKey: ["admin-vendors", "children", Number(vendorId), "branches"],
    queryFn: () => listVendorChildren(Number(vendorId), "branches"),
    enabled: vendorId !== "",
  });
  // Only currently-effective branch packages (vendor_packages_branch has no
  // is_active since migration 016 -- "active" = effective_end_date open/future).
  const packages = useQuery({
    queryKey: ["admin-vendors", "packages-options", Number(vendorId), branchId],
    queryFn: () =>
      listVendorPackages(Number(vendorId), {
        page: 1,
        page_size: 100,
        status: "active",
        branch_id: branchId === "" ? undefined : Number(branchId),
      }),
    enabled: vendorId !== "",
  });
  const create = useMutation({
    mutationFn: () =>
      // No dates: the system starts the period on the approval date (open-ended)
      // and closes the running one (ATMAssignmentApplier).
      createATMAssignment(atmId, { vendor_package_id: Number(packageId) }),
    onSuccess: (res) => {
      toast({ type: "success", message: pendingApprovalMessage(res) });
      setPackageId("");
      qc.invalidateQueries({ queryKey: ["admin-atms", "assignments", atmId] });
    },
    onError: (err: Error) => toast({ type: "error", message: err.message }),
  });

  const activeBranches = (branches.data ?? []).filter((b) => b.is_active);
  const activePackages = packages.data?.packages ?? [];
  // Re-assigning the package already running would change nothing.
  const cur = list.data
    ? currentAssignment(list.data, new Date().toLocaleDateString("en-CA"))
    : undefined;
  const canSubmit = packageId !== "" && packageId !== idStr(cur?.vendor_package_id);
  const escrowChanged = atm !== null && escrowAccount !== (atm.escrow_account ?? "");

  // UpdateATMPayload has no partial-patch form (Sec: ATM Update requires the
  // whole record) -- resend the ATM's current fields unchanged, only
  // escrow_account differs. Same maker-checker flow as "Ubah ATM" (202/pending).
  async function handleSaveEscrow() {
    if (!atm) return;
    const payload: UpdateATMPayload = {
      location_id: atm.location_id,
      machine_type: atm.machine_type,
      brand: atm.brand,
      model: atm.model,
      operation_hours: atm.operation_hours,
      deployment_type: atm.deployment_type,
      capacity_amount: atm.capacity_amount,
      low_threshold_amount: atm.low_threshold_amount,
      critical_threshold_amount: atm.critical_threshold_amount,
      blacklisted: atm.blacklisted,
      escrow_account: escrowAccount || null,
      priority_class: atm.priority_class,
    };
    try {
      const res = await updateATM.mutateAsync({ id: atm.id, payload });
      toast({ type: "success", message: pendingApprovalMessage(res) });
    } catch (err) {
      toast({ type: "error", message: err instanceof Error ? err.message : "Gagal menyimpan" });
    }
  }

  return (
    <Dialog open={atm !== null} onClose={onClose} title={`Kelolaan ATM ${atm?.terminal_id ?? ""}`}>
      <div className="flex max-h-[70vh] flex-col gap-4 overflow-y-auto">
        <div className="flex items-end gap-3">
          <label className="flex flex-1 flex-col gap-1 text-xs font-medium text-[var(--n-600)]">
            Nomor Rekening Escrow
            <input
              className={inputClass}
              value={escrowAccount}
              onChange={(e) => setEscrowAccount(e.target.value)}
            />
          </label>
          <Button
            type="button"
            variant="secondary"
            disabled={!escrowChanged || updateATM.isPending}
            onClick={handleSaveEscrow}
          >
            Simpan
          </Button>
        </div>
        {list.isError && (
          <p role="alert" className="text-sm text-[var(--danger-fg)]">
            Gagal memuat kelolaan
          </p>
        )}
        {list.data && list.data.length === 0 && (
          <p className="text-sm text-[var(--n-500)]">Belum ada periode kelolaan.</p>
        )}
        {list.data && list.data.length > 0 && (
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="text-left text-[var(--n-500)]">
                <th className="py-2 pr-4 font-medium">Paket</th>
                <th className="py-2 pr-4 font-medium">Mulai</th>
                <th className="py-2 pr-4 font-medium">Selesai</th>
                <th className="py-2 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((a) => (
                <tr key={a.id} className="border-t border-[var(--n-200)]">
                  <td className="py-2 pr-4">{a.package_code}</td>
                  <td className="py-2 pr-4 tabular-nums">{a.effective_start_date}</td>
                  <td className="py-2 pr-4 tabular-nums">{a.effective_end_date ?? "Terbuka"}</td>
                  <td className="py-2">
                    {a.is_active ? (
                      <Badge variant="success" icon={CheckCircle} label="Aktif" />
                    ) : (
                      <Badge variant="danger" icon={XCircle} label="Nonaktif" />
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        <form
          className="flex flex-col gap-3 border-t border-[var(--n-200)] pt-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (canSubmit) create.mutate();
          }}
        >
          <h3 className="text-sm font-semibold text-[var(--n-800)]">Tetapkan paket baru</h3>
          <label className="flex flex-col gap-1 text-xs font-medium text-[var(--n-600)]">
            Vendor
            <select
              className={inputClass}
              value={vendorId}
              onChange={(e) => {
                setVendorId(e.target.value);
                setBranchId("");
                setPackageId("");
              }}
            >
              <option value="">Pilih vendor</option>
              {vendors.data?.vendors.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.code} · {v.name}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-[var(--n-600)]">
            Cabang
            <select
              className={inputClass}
              value={branchId}
              disabled={vendorId === ""}
              onChange={(e) => {
                setBranchId(e.target.value);
                setPackageId("");
              }}
            >
              <option value="">Semua cabang</option>
              {activeBranches.map((b) => (
                <option key={String(b.id)} value={String(b.id)}>
                  {String(b.branch_code)} · {String(b.branch_name)}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-xs font-medium text-[var(--n-600)]">
            Paket
            <select
              className={inputClass}
              value={packageId}
              disabled={vendorId === ""}
              onChange={(e) => setPackageId(e.target.value)}
            >
              <option value="">Pilih paket</option>
              {activePackages.map((p) => (
                <option key={p.id} value={String(p.id)}>
                  {p.package_code} · {p.machine_group} · {p.price_class}
                  {p.atm_id !== null ? ` · ATM #${p.atm_id}` : ""}
                </option>
              ))}
            </select>
          </label>
          <p className="text-xs text-[var(--n-500)]">
            Periode diatur otomatis: mulai berlaku pada tanggal disetujui dan berakhir terbuka;
            periode yang sedang berjalan ditutup sehari sebelumnya.
          </p>
          <div className="flex justify-end gap-3">
            <Button type="button" variant="secondary" onClick={onClose}>
              Tutup
            </Button>
            <Button type="submit" disabled={!canSubmit || create.isPending}>
              Ajukan
            </Button>
          </div>
        </form>
      </div>
    </Dialog>
  );
}
