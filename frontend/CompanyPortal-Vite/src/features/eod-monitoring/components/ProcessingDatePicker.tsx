import { useId } from "react";

interface ProcessingDatePickerProps {
  value: string;
  onChange: (date: string) => void;
}

/** Native date input controlling the processing_date parameter for all EOD queries. */
export function ProcessingDatePicker({ value, onChange }: ProcessingDatePickerProps) {
  const inputId = useId();

  return (
    <div className="flex flex-col gap-1">
      <label
        htmlFor={inputId}
        className="text-xs font-medium text-[var(--n-600)] uppercase tracking-wider"
      >
        Tanggal Proses
      </label>
      <input
        id={inputId}
        type="date"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="min-h-[44px] rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 text-sm text-[var(--n-800)] font-sans focus-visible:ring-2 focus-visible:ring-[var(--red-100)] focus-visible:border-[var(--red-400)] outline-none"
      />
    </div>
  );
}
