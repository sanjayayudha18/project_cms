import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { Loader2 } from 'lucide-react';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';
type ButtonSize = 'sm' | 'md' | 'lg';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  readonly variant?: ButtonVariant;
  readonly size?: ButtonSize;
  readonly isLoading?: boolean;
  readonly children: ReactNode;
}

const variantStyles: Record<ButtonVariant, string> = {
  primary:
    'bg-sidebar-active text-white hover:bg-sidebar-active/90 focus-visible:outline-sidebar-active',
  secondary:
    'bg-white border border-neutral-300 text-surface-text hover:bg-neutral-50 focus-visible:outline-sidebar-active',
  ghost:
    'text-sidebar-active hover:bg-sidebar-active/10 focus-visible:outline-sidebar-active',
  danger:
    'bg-danger-fg text-white hover:bg-danger-fg/90 focus-visible:outline-danger-fg',
};

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'min-h-[44px] px-3 py-1.5 text-sm',
  md: 'min-h-[44px] px-4 py-2 text-sm',
  lg: 'min-h-[44px] px-6 py-2.5 text-base',
};

/**
 * Button component with primary/secondary/ghost/danger variants.
 * All sizes enforce 44px minimum touch target. Visible focus ring on keyboard navigation.
 */
export function Button({
  variant = 'primary',
  size = 'md',
  isLoading = false,
  disabled,
  children,
  className = '',
  ...props
}: ButtonProps) {
  const isDisabled = disabled || isLoading;

  return (
    <button
      disabled={isDisabled}
      className={[
        'inline-flex items-center justify-center gap-2 rounded-md font-medium',
        'transition-colors duration-150',
        'focus-visible:outline-2 focus-visible:outline-offset-2',
        variantStyles[variant],
        sizeStyles[size],
        isDisabled ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer',
        className,
      ].join(' ')}
      {...props}
    >
      {isLoading && (
        <Loader2 className="size-4 animate-spin" aria-hidden="true" />
      )}
      {children}
    </button>
  );
}
