/**
 * Shared label + input wrapper for the RBAC settings forms (hierarchy,
 * delegation, leave, policy -- Tasks 10-13): inline validation error as
 * icon + text (never color alone, Requirement 10.5), soft red focus ring
 * (Requirement 10.4, same tokens as Button.tsx).
 */

import { AlertCircle } from "lucide-react";
import type { ReactNode } from "react";

export const rbacInputClass =
  "w-full rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-3 py-2 text-sm text-[var(--n-900)] outline-none focus-visible:ring-2 focus-visible:ring-[var(--red-100)] focus-visible:border-[var(--red-400)]";

interface FormFieldProps {
  label: string;
  htmlFor: string;
  error?: string;
  children: ReactNode;
}

export function FormField({ label, htmlFor, error, children }: FormFieldProps) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={htmlFor} className="text-xs font-medium text-[var(--n-600)]">
        {label}
      </label>
      {children}
      {error && (
        <p className="flex items-center gap-1 text-xs text-[var(--danger-fg)]">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
    </div>
  );
}
