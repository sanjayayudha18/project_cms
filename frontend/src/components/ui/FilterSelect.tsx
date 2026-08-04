interface FilterSelectProps {
  label: string;
  options: { value: string; label: string }[];
  value: string | null;
  onChange: (value: string | null) => void;
  placeholder?: string;
}

/**
 * Dropdown filter control with nullable value and 44×44px min touch target.
 * An empty selection maps to null (show all).
 *
 * @validates Requirements 4.6, 4.7, 5.4, 5.5
 */
export function FilterSelect({
  label,
  options,
  value,
  onChange,
  placeholder = 'All',
}: FilterSelectProps) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-xs font-medium text-n-600 uppercase tracking-wider">
        {label}
      </label>
      <select
        value={value ?? ''}
        onChange={(e) => {
          const val = e.target.value;
          onChange(val === '' ? null : val);
        }}
        className="min-h-[44px] min-w-[44px] rounded-md border border-n-300 bg-n-0 px-3 text-sm text-n-800 font-sans focus-visible:ring-2 focus-visible:ring-red-100 focus-visible:border-red-400 outline-none"
      >
        <option value="">{placeholder}</option>
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
    </div>
  );
}
