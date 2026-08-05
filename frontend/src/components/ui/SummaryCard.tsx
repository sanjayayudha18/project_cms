import { formatIDR } from '@/lib/utils/formatCurrency';

interface SummaryCardProps {
  label: string;
  value: string | number;
  format?: 'currency' | 'number' | 'text';
}

/**
 * Metric card displaying a label and formatted value.
 * Supports currency (IDR), plain number, or text formatting.
 */
export function SummaryCard({ label, value, format = 'text' }: SummaryCardProps) {
  const formattedValue = formatValue(value, format);

  return (
    <div className="bg-[var(--n-0)] shadow-sm rounded-[var(--radius-lg)] p-4">
      <p className="text-sm text-[var(--n-600)] font-medium">{label}</p>
      <p className="text-xl font-semibold text-[var(--n-900)] mt-1 tabular-nums">
        {formattedValue}
      </p>
    </div>
  );
}

function formatValue(
  value: string | number,
  format: 'currency' | 'number' | 'text',
): string {
  if (format === 'currency' && typeof value === 'number') {
    return formatIDR(value);
  }
  if (format === 'number' && typeof value === 'number') {
    return value.toLocaleString('id-ID');
  }
  return String(value);
}
