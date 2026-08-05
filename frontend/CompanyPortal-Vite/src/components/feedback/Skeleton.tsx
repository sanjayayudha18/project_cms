import { cn } from "@/lib/utils/cn";

// ─── Base Skeleton ────────────────────────────────────────────────────────────

export interface SkeletonProps {
  className?: string;
  width?: string | number;
  height?: string | number;
}

export function Skeleton({ className, width, height }: SkeletonProps) {
  return (
    <div
      aria-hidden="true"
      className={cn("animate-pulse rounded-md", className)}
      style={{
        backgroundColor: "var(--n-200)",
        width: typeof width === "number" ? `${width}px` : width,
        height: typeof height === "number" ? `${height}px` : height,
      }}
    />
  );
}

// ─── Preset: Text Line ────────────────────────────────────────────────────────

export interface SkeletonTextProps {
  lines?: number;
  className?: string;
}

export function SkeletonText({ lines = 3, className }: SkeletonTextProps) {
  return (
    <div className={cn("flex flex-col gap-2", className)} aria-hidden="true">
      {Array.from({ length: lines }, (_, i) => (
        <div
          key={i}
          className="animate-pulse rounded"
          style={{
            backgroundColor: "var(--n-200)",
            height: "14px",
            width: i === lines - 1 ? "60%" : "100%",
            borderRadius: "var(--radius-sm)",
          }}
        />
      ))}
    </div>
  );
}

// ─── Preset: Card ─────────────────────────────────────────────────────────────

export interface SkeletonCardProps {
  className?: string;
}

export function SkeletonCard({ className }: SkeletonCardProps) {
  return (
    <div
      className={cn("flex flex-col gap-3 p-4", className)}
      style={{
        backgroundColor: "var(--n-0)",
        borderRadius: "var(--radius-lg)",
        boxShadow: "var(--shadow-sm)",
      }}
      aria-hidden="true"
    >
      <div
        className="animate-pulse rounded"
        style={{
          backgroundColor: "var(--n-200)",
          height: "16px",
          width: "40%",
          borderRadius: "var(--radius-sm)",
        }}
      />
      <div
        className="animate-pulse rounded"
        style={{
          backgroundColor: "var(--n-200)",
          height: "32px",
          width: "70%",
          borderRadius: "var(--radius-sm)",
        }}
      />
      <div
        className="animate-pulse rounded"
        style={{
          backgroundColor: "var(--n-200)",
          height: "14px",
          width: "50%",
          borderRadius: "var(--radius-sm)",
        }}
      />
    </div>
  );
}

// ─── Preset: Table Row ────────────────────────────────────────────────────────

export interface SkeletonTableProps {
  rows?: number;
  columns?: number;
  className?: string;
}

export function SkeletonTable({ rows = 5, columns = 4, className }: SkeletonTableProps) {
  return (
    <div className={cn("flex flex-col gap-2", className)} aria-hidden="true">
      {/* Header row */}
      <div className="flex gap-3 py-2">
        {Array.from({ length: columns }, (_, i) => (
          <div
            key={i}
            className="animate-pulse flex-1 rounded"
            style={{
              backgroundColor: "var(--n-200)",
              height: "12px",
              borderRadius: "var(--radius-sm)",
            }}
          />
        ))}
      </div>
      {/* Data rows */}
      {Array.from({ length: rows }, (_, rowIdx) => (
        <div
          key={rowIdx}
          className="flex gap-3 py-2"
          style={{ borderTop: "1px solid var(--n-100)" }}
        >
          {Array.from({ length: columns }, (_, colIdx) => (
            <div
              key={colIdx}
              className="animate-pulse flex-1 rounded"
              style={{
                backgroundColor: "var(--n-200)",
                height: "14px",
                borderRadius: "var(--radius-sm)",
              }}
            />
          ))}
        </div>
      ))}
    </div>
  );
}
