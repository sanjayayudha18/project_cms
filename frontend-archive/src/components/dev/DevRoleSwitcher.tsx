import { useRole } from '@/context/RoleContext';
import { ROLES } from '@/lib/constants';
import type { Role } from '@/lib/constants';

/**
 * Dev-only floating role switcher.
 * Renders a fixed pill in the bottom-right corner that lets you
 * swap roles instantly. Only included when import.meta.env.DEV is true.
 */
export function DevRoleSwitcher() {
  const { role, setRole } = useRole();

  // Never render in production
  if (!import.meta.env.DEV) return null;

  return (
    <div
      className="
        fixed bottom-4 right-4 z-[9999]
        flex items-center gap-2 rounded-full
        border border-[var(--n-300)] bg-[var(--n-0)]
        px-3 py-2 shadow-md
        text-xs font-medium text-[var(--n-700)]
      "
    >
      <span className="select-none opacity-60">DEV</span>
      <select
        value={role}
        onChange={(e) => setRole(e.target.value as Role)}
        aria-label="Switch role (dev only)"
        className="
          cursor-pointer rounded border-none bg-transparent
          text-xs font-semibold text-[var(--n-800)]
          focus:outline-none focus:ring-1 focus:ring-[var(--red-400)]
        "
      >
        {ROLES.map((r) => (
          <option key={r} value={r}>
            {r}
          </option>
        ))}
      </select>
    </div>
  );
}
