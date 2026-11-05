import type { ReactNode } from 'react';

interface CardProps {
  children: ReactNode;
  className?: string;
}

/**
 * Generic card wrapper with shadow and padding tokens.
 * Used as a building block for content areas, panels, and summaries.
 */
export function Card({ children, className = '' }: CardProps) {
  return (
    <div
      className={`bg-n-0 shadow-sm rounded-lg p-6 ${className}`.trim()}
    >
      {children}
    </div>
  );
}
