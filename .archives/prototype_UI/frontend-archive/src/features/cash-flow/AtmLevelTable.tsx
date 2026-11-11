import type { AtmLevel, CashLevelTier } from './types';

/**
 * Determine the semantic color tier based on ATM cash level percentage.
 *
 * - >= 50%: success (green)
 * - 20–49%: warning (amber)
 * - < 20%: danger (red)
 */
export function getCashLevelTier(percentage: number): CashLevelTier {
  if (percentage >= 50) return 'success';
  if (percentage >= 20) return 'warning';
  return 'danger';
}

const tierStyles: Record<CashLevelTier, string> = {
  success: 'bg-[oklch(0.560_0.130_155)]', // --success-solid
  warning: 'bg-[oklch(0.760_0.150_78)]', // --warning-solid
  danger: 'bg-[oklch(0.545_0.205_12)]', // --danger-solid
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
      <td className="font-mono text-sm text-[var(--n-800)] py-2">
        {atm.label}
      </td>
      <td className="py-2 px-3 w-full">
        <div
          className="h-2 rounded-full bg-[var(--n-100)] overflow-hidden"
          role="progressbar"
          aria-valuenow={atm.percentage}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${atm.label} cash level ${atm.percentage}%`}
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
