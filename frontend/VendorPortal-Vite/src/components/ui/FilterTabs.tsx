interface FilterTabOption<T extends string> {
  readonly value: T;
  readonly label: string;
  readonly count?: number;
}

interface FilterTabsProps<T extends string> {
  readonly options: readonly FilterTabOption<T>[];
  readonly selected: T;
  readonly onChange: (value: T) => void;
}

/**
 * Horizontal filter tabs with active state (underline + text color).
 * Shows count badge next to label if provided. Enforces 44px minimum touch target.
 */
export function FilterTabs<T extends string>({
  options,
  selected,
  onChange,
}: FilterTabsProps<T>) {
  return (
    <div className="flex gap-1 border-b border-neutral-200" role="tablist">
      {options.map((option) => {
        const isActive = option.value === selected;

        return (
          <button
            key={option.value}
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => onChange(option.value)}
            className={[
              'inline-flex items-center gap-1.5 min-h-[44px] px-3 py-2 text-sm font-medium',
              'border-b-2 transition-colors duration-150 -mb-px',
              'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active',
              isActive
                ? 'text-sidebar-active border-sidebar-active'
                : 'text-neutral-500 border-transparent hover:text-surface-text hover:border-neutral-300',
            ].join(' ')}
          >
            {option.label}
            {option.count !== undefined && (
              <span
                className={[
                  'inline-flex items-center justify-center min-w-[20px] h-5 px-1.5 rounded-full text-xs font-semibold',
                  isActive
                    ? 'bg-sidebar-active/10 text-sidebar-active'
                    : 'bg-neutral-100 text-neutral-500',
                ].join(' ')}
              >
                {option.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
