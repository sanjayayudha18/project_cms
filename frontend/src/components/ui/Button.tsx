import type { ReactNode } from 'react';

interface ButtonProps {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  className?: string;
  type?: 'button' | 'submit';
}

/**
 * Design-system button with variant-based styling.
 * All variants enforce 44×44px minimum touch target.
 *
 * - Primary: brand red-500 (one per view)
 * - Secondary: neutral with border
 * - Ghost: transparent with red text
 * - Danger: rose hue 12 (NOT brand red) — destructive actions only
 */
export function Button({
  variant = 'primary',
  children,
  onClick,
  disabled = false,
  className = '',
  type = 'button',
}: ButtonProps) {
  const base =
    'inline-flex items-center justify-center min-h-[44px] min-w-[44px] px-4 rounded-[var(--radius-md)] font-medium text-sm transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-[var(--red-100)] focus-visible:border-[var(--red-400)] outline-none cursor-pointer';

  const variants: Record<string, string> = {
    primary: 'bg-[var(--red-500)] text-[var(--n-0)] hover:bg-[var(--red-600)]',
    secondary: 'bg-[var(--n-0)] border border-[var(--n-300)] text-[var(--n-800)] hover:bg-[var(--n-50)]',
    ghost: 'bg-transparent text-[var(--red-600)] hover:bg-[var(--red-50)]',
    danger: 'bg-[var(--danger-solid)] text-[var(--n-0)] hover:opacity-90',
  };

  const disabledStyle = 'bg-[var(--n-200)] text-[var(--n-400)] cursor-not-allowed hover:bg-[var(--n-200)]';

  const variantClass = disabled ? disabledStyle : variants[variant];

  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={`${base} ${variantClass} ${className}`.trim()}
    >
      {children}
    </button>
  );
}

export type { ButtonProps };
