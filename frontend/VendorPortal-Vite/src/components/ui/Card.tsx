import type { ReactNode } from "react";

interface CardProps {
  readonly children: ReactNode;
  readonly className?: string;
}

/**
 * Simple container card with white background, rounded corners, subtle shadow, and border.
 * Used for summary cards and content sections.
 */
export function Card({ children, className = "" }: CardProps) {
  return (
    <div
      className={["bg-white rounded-lg shadow-sm border border-neutral-200 p-4", className].join(
        " ",
      )}
    >
      {children}
    </div>
  );
}
