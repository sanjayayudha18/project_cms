import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ErrorBoundary } from "./ErrorBoundary";
import { NetworkError } from "./NetworkError";
import { ServerError, containsSensitiveDetails, sanitizeServerError } from "./ServerError";
import { Skeleton, SkeletonCard, SkeletonTable, SkeletonText } from "./Skeleton";

// ─── ErrorBoundary ────────────────────────────────────────────────────────────

describe("ErrorBoundary", () => {
  // Suppress React error boundary console.error noise in tests
  const originalError = console.error;
  beforeEach(() => {
    console.error = vi.fn();
  });
  afterEach(() => {
    console.error = originalError;
  });

  function ThrowingComponent({ shouldThrow }: { shouldThrow: boolean }) {
    if (shouldThrow) {
      throw new Error("Test error");
    }
    return <p>Normal content</p>;
  }

  it("renders children when no error occurs", () => {
    render(
      <ErrorBoundary>
        <p>Hello</p>
      </ErrorBoundary>,
    );
    expect(screen.getByText("Hello")).toBeInTheDocument();
  });

  it("renders fallback UI when child throws", () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText("Terjadi Kesalahan")).toBeInTheDocument();
    expect(screen.getByText("Muat Ulang Halaman")).toBeInTheDocument();
  });

  it("does not expose technical error details in default fallback", () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(screen.queryByText("Test error")).not.toBeInTheDocument();
  });

  it("renders custom fallback when provided", () => {
    function CustomFallback() {
      return <p>Custom error UI</p>;
    }

    render(
      <ErrorBoundary fallback={CustomFallback}>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    expect(screen.getByText("Custom error UI")).toBeInTheDocument();
  });

  it("reload button triggers window.location.reload", () => {
    const reloadMock = vi.fn();
    Object.defineProperty(window, "location", {
      value: { reload: reloadMock },
      writable: true,
    });

    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    fireEvent.click(screen.getByText("Muat Ulang Halaman"));
    expect(reloadMock).toHaveBeenCalledTimes(1);
  });

  it("displays an icon alongside error text", () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </ErrorBoundary>,
    );

    // lucide-react renders SVG elements
    const alert = screen.getByRole("alert");
    const svg = alert.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});

// ─── NetworkError ─────────────────────────────────────────────────────────────

describe("NetworkError", () => {
  it("renders network error message", () => {
    render(<NetworkError onRetry={() => {}} />);

    expect(screen.getByText("Koneksi Terputus")).toBeInTheDocument();
    expect(screen.getByText(/Tidak dapat terhubung ke server/)).toBeInTheDocument();
  });

  it("renders retry button that calls onRetry", () => {
    const onRetry = vi.fn();
    render(<NetworkError onRetry={onRetry} />);

    fireEvent.click(screen.getByText("Coba Lagi"));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("has role=alert for accessibility", () => {
    render(<NetworkError onRetry={() => {}} />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("displays an icon alongside error text", () => {
    render(<NetworkError onRetry={() => {}} />);

    const alert = screen.getByRole("alert");
    const svg = alert.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});

// ─── ServerError ──────────────────────────────────────────────────────────────

describe("ServerError", () => {
  it("renders generic server error message", () => {
    render(<ServerError />);

    expect(screen.getByText("Kesalahan Server")).toBeInTheDocument();
    expect(
      screen.getByText("Terjadi kesalahan pada server. Silakan coba lagi nanti."),
    ).toBeInTheDocument();
  });

  it("renders retry button when onRetry is provided", () => {
    const onRetry = vi.fn();
    render(<ServerError onRetry={onRetry} />);

    fireEvent.click(screen.getByText("Coba Lagi"));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("does not render retry button when onRetry is not provided", () => {
    render(<ServerError />);
    expect(screen.queryByText("Coba Lagi")).not.toBeInTheDocument();
  });

  it("has role=alert for accessibility", () => {
    render(<ServerError />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});

describe("sanitizeServerError", () => {
  it("always returns a generic message regardless of input", () => {
    const expected = "Terjadi kesalahan pada server. Silakan coba lagi nanti.";

    expect(sanitizeServerError("SQL error: relation does not exist")).toBe(expected);
    expect(sanitizeServerError("panic: runtime error")).toBe(expected);
    expect(sanitizeServerError("Error at /app/internal/handler/user.go:42")).toBe(expected);
    expect(sanitizeServerError(null)).toBe(expected);
    expect(sanitizeServerError(undefined)).toBe(expected);
    expect(sanitizeServerError("")).toBe(expected);
  });
});

describe("containsSensitiveDetails", () => {
  it("detects stack traces", () => {
    expect(containsSensitiveDetails("at handleRequest (")).toBe(true);
  });

  it("detects SQL references", () => {
    expect(containsSensitiveDetails("sql: no rows in result set")).toBe(true);
  });

  it("detects file paths", () => {
    expect(containsSensitiveDetails("main.go:142")).toBe(true);
    expect(containsSensitiveDetails("handler.ts:30")).toBe(true);
  });

  it("detects postgres references", () => {
    expect(containsSensitiveDetails("postgres connection refused")).toBe(true);
  });

  it("returns false for safe messages", () => {
    expect(containsSensitiveDetails("Something went wrong")).toBe(false);
    expect(containsSensitiveDetails("Please try again later")).toBe(false);
  });
});

// ─── Skeleton ─────────────────────────────────────────────────────────────────

describe("Skeleton", () => {
  it("renders with aria-hidden", () => {
    const { container } = render(<Skeleton />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toHaveAttribute("aria-hidden", "true");
  });

  it("applies custom width and height as numbers", () => {
    const { container } = render(<Skeleton width={200} height={40} />);
    const el = container.firstElementChild as HTMLElement;
    expect(el.style.width).toBe("200px");
    expect(el.style.height).toBe("40px");
  });

  it("applies custom width and height as strings", () => {
    const { container } = render(<Skeleton width="100%" height="2rem" />);
    const el = container.firstElementChild as HTMLElement;
    expect(el.style.width).toBe("100%");
    expect(el.style.height).toBe("2rem");
  });

  it("applies custom className", () => {
    const { container } = render(<Skeleton className="my-custom" />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toHaveClass("my-custom");
  });

  it("has animate-pulse class", () => {
    const { container } = render(<Skeleton />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toHaveClass("animate-pulse");
  });
});

describe("SkeletonText", () => {
  it("renders default 3 lines", () => {
    const { container } = render(<SkeletonText />);
    const lines = container.querySelectorAll("[aria-hidden] > div");
    expect(lines).toHaveLength(3);
  });

  it("renders custom number of lines", () => {
    const { container } = render(<SkeletonText lines={5} />);
    const lines = container.querySelectorAll("[aria-hidden] > div");
    expect(lines).toHaveLength(5);
  });

  it("last line is shorter (60% width)", () => {
    const { container } = render(<SkeletonText lines={2} />);
    const lines = container.querySelectorAll("[aria-hidden] > div");
    const lastLine = lines[1] as HTMLElement;
    expect(lastLine.style.width).toBe("60%");
  });
});

describe("SkeletonCard", () => {
  it("renders with aria-hidden", () => {
    const { container } = render(<SkeletonCard />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toHaveAttribute("aria-hidden", "true");
  });

  it("renders 3 shimmer blocks", () => {
    const { container } = render(<SkeletonCard />);
    const blocks = container.querySelectorAll(".animate-pulse");
    expect(blocks).toHaveLength(3);
  });
});

describe("SkeletonTable", () => {
  it("renders default 5 rows + 1 header", () => {
    const { container } = render(<SkeletonTable />);
    // header row + 5 data rows = 6 flex rows
    const rows = container.querySelectorAll("[aria-hidden] > div");
    expect(rows).toHaveLength(6);
  });

  it("renders custom row and column count", () => {
    const { container } = render(<SkeletonTable rows={3} columns={6} />);
    const rows = container.querySelectorAll("[aria-hidden] > div");
    expect(rows).toHaveLength(4); // 1 header + 3 data rows

    // Each row should have 6 columns
    const firstRow = rows[0] as HTMLElement;
    expect(firstRow.children).toHaveLength(6);
  });
});
