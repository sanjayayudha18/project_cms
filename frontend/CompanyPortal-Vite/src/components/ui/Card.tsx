import type { ReactNode } from "react";

interface CardProps {
  children: ReactNode;
  className?: string;
}

/**
 * Generic card wrapper with shadow and padding tokens.
 * Used as a building block for content areas, panels, and summaries.
 */
export function Card({ children, className = "" }: CardProps) {
  return (
    <div className={`bg-[var(--n-0)] shadow-sm rounded-[var(--radius-lg)] p-6 ${className}`.trim()}>
      {children}
    </div>
  );
}
