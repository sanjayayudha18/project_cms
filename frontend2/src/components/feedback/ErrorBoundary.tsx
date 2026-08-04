import { cn } from "@/lib/utils/cn";
import { AlertTriangle } from "lucide-react";
import { Component, type ErrorInfo, type ReactNode } from "react";

// ─── Types ────────────────────────────────────────────────────────────────────

export interface ErrorFallbackProps {
  error: Error;
  resetErrorBoundary: () => void;
}

export interface ErrorBoundaryProps {
  children: ReactNode;
  fallback?: React.ComponentType<ErrorFallbackProps>;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

// ─── Default Fallback ─────────────────────────────────────────────────────────

function DefaultErrorFallback({ resetErrorBoundary }: ErrorFallbackProps) {
  return (
    <div
      role="alert"
      className={cn("flex flex-col items-center justify-center gap-4 p-8 text-center")}
    >
      <div
        className="flex items-center justify-center rounded-full p-3"
        style={{ backgroundColor: "var(--danger-bg)" }}
      >
        <AlertTriangle size={32} style={{ color: "var(--danger-fg)" }} aria-hidden="true" />
      </div>
      <h2 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
        Terjadi Kesalahan
      </h2>
      <p className="max-w-md text-sm" style={{ color: "var(--n-600)" }}>
        Terjadi kesalahan yang tidak terduga. Silakan muat ulang halaman untuk melanjutkan.
      </p>
      <button
        type="button"
        onClick={resetErrorBoundary}
        className={cn(
          "mt-2 cursor-pointer rounded-md px-4 py-2 text-sm font-semibold text-white",
          "transition-colors duration-150 ease-out",
          "focus-visible:outline-none",
        )}
        style={{
          backgroundColor: "var(--red-500)",
          borderRadius: "var(--radius-md)",
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.backgroundColor = "var(--red-600)";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.backgroundColor = "var(--red-500)";
        }}
      >
        Muat Ulang Halaman
      </button>
    </div>
  );
}

// ─── ErrorBoundary Class Component ────────────────────────────────────────────

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(_error: Error, _info: ErrorInfo): void {
    // Error logging could be added here in the future.
    // Technical details are intentionally NOT exposed to the user.
  }

  resetErrorBoundary = (): void => {
    window.location.reload();
  };

  render(): ReactNode {
    if (this.state.hasError && this.state.error) {
      const FallbackComponent = this.props.fallback ?? DefaultErrorFallback;
      return (
        <FallbackComponent error={this.state.error} resetErrorBoundary={this.resetErrorBoundary} />
      );
    }

    return this.props.children;
  }
}
