import type { AtmLevel, CashLevelTier } from "./types";

/**
 * Menentukan tier warna semantik berdasarkan persentase level kas ATM.
 *
 * - >= 50%: success (hijau)
 * - 20–49%: warning (kuning)
 * - < 20%: danger (merah)
 */
export function getCashLevelTier(percentage: number): CashLevelTier {
  if (percentage >= 50) return "success";
  if (percentage >= 20) return "warning";
  return "danger";
}

const tierStyles: Record<CashLevelTier, string> = {
  success: "bg-[var(--success-solid)]",
  warning: "bg-[var(--warning-solid)]",
  danger: "bg-[var(--danger-500)]",
};

interface AtmLevelTableProps {
  readonly levels: readonly AtmLevel[];
}

export function AtmLevelTable({ levels }: AtmLevelTableProps) {
  return (
    <table className="w-full">
      <tbody>
        {levels.map((atm) => (
          <AtmLevelRow key={atm.id} atm={atm} />
        ))}
      </tbody>
    </table>
  );
}

export function AtmLevelRow({ atm }: { readonly atm: AtmLevel }) {
  const tier = getCashLevelTier(atm.percentage);

  return (
    <tr>
      <td className="font-mono text-sm text-[var(--n-800)] py-2">{atm.label}</td>
      <td className="py-2 px-3 w-full">
        <div
          className="h-2 rounded-full bg-[var(--n-100)] overflow-hidden"
          role="progressbar"
          tabIndex={0}
          aria-valuenow={atm.percentage}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${atm.label} level kas ${atm.percentage}%`}
        >
          <div
            className={`h-full rounded-full transition-all ${tierStyles[tier]}`}
            style={{ width: `${atm.percentage}%` }}
          />
        </div>
      </td>
      <td className="text-right text-sm tabular-nums text-[var(--n-700)] py-2 whitespace-nowrap">
        {atm.percentage}%
      </td>
    </tr>
  );
}
