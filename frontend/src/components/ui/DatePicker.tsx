interface DatePickerProps {
  value: string;
  onChange: (date: string) => void;
  label?: string;
}

/**
 * Simple date input styled with design tokens.
 * Uses native `<input type="date">` for broad browser support.
 * Min height 44px for touch target compliance.
 *
 * @validates Requirements 3.8
 */
export function DatePicker({ value, onChange, label }: DatePickerProps) {
  return (
    <div className="flex flex-col gap-1">
      {label && (
        <label className="text-xs font-medium text-n-600 uppercase tracking-wider">
          {label}
        </label>
      )}
      <input
        type="date"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="min-h-[44px] min-w-[44px] rounded-md border border-n-300 bg-n-0 px-3 text-sm text-n-800 font-sans focus-visible:ring-2 focus-visible:ring-red-100 focus-visible:border-red-400 outline-none"
      />
    </div>
  );
}
