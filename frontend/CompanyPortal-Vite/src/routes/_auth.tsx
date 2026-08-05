import { Outlet, createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";

export const authRoute = createRoute({
  id: "_auth",
  getParentRoute: () => rootRoute,
  component: AuthLayout,
});

function AuthLayout() {
  return (
    <div
      className="grid min-h-screen lg:grid-cols-2"
      style={{ backgroundColor: "var(--n-50)" }}
    >
      {/* Left panel — brand / decorative (hidden on mobile) */}
      <div
        className="hidden flex-col justify-between p-[var(--space-12)] lg:flex"
        style={{ backgroundColor: "var(--red-500)" }}
      >
        <div className="flex items-center gap-[var(--space-3)]">
          <div
            className="flex h-10 w-10 items-center justify-center rounded-[var(--radius-md)]"
            style={{ backgroundColor: "oklch(1 0 0 / 0.15)" }}
          >
            <svg
              width="22"
              height="22"
              viewBox="0 0 24 24"
              fill="none"
              stroke="white"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <rect x="3" y="3" width="7" height="7" rx="1" />
              <rect x="14" y="3" width="7" height="7" rx="1" />
              <rect x="3" y="14" width="7" height="7" rx="1" />
              <rect x="14" y="14" width="7" height="7" rx="1" />
            </svg>
          </div>
          <span className="text-base font-semibold text-white">CMS</span>
        </div>

        <div className="flex flex-col gap-[var(--space-4)]">
          <blockquote className="text-2xl font-semibold leading-snug text-white" style={{ textWrap: "balance" }}>
            Kelola operasional kas ATM secara terintegrasi, dari peramalan hingga rekonsiliasi.
          </blockquote>
          <p className="text-sm" style={{ color: "oklch(1 0 0 / 0.7)" }}>
            End-to-End Cash Management System
          </p>
        </div>

        <p className="text-xs" style={{ color: "oklch(1 0 0 / 0.5)" }}>
          © 2026 CIMB Niaga STCC
        </p>
      </div>

      {/* Right panel — login form */}
      <div className="flex flex-col items-center justify-center px-[var(--space-6)] py-[var(--space-12)]">
        {/* Mobile brand (visible only < lg) */}
        <div className="mb-[var(--space-8)] flex items-center gap-[var(--space-3)] lg:hidden">
          <div
            className="flex h-10 w-10 items-center justify-center rounded-[var(--radius-md)]"
            style={{ backgroundColor: "var(--red-500)" }}
          >
            <svg
              width="22"
              height="22"
              viewBox="0 0 24 24"
              fill="none"
              stroke="white"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <rect x="3" y="3" width="7" height="7" rx="1" />
              <rect x="14" y="3" width="7" height="7" rx="1" />
              <rect x="3" y="14" width="7" height="7" rx="1" />
              <rect x="14" y="14" width="7" height="7" rx="1" />
            </svg>
          </div>
          <span className="text-base font-semibold" style={{ color: "var(--n-900)" }}>
            Cash Management System
          </span>
        </div>

        <div className="w-full max-w-[380px]">
          <Outlet />
        </div>

        {/* Mobile footer */}
        <p
          className="mt-[var(--space-8)] text-xs lg:hidden"
          style={{ color: "var(--n-400)" }}
        >
          © 2026 CIMB Niaga STCC
        </p>
      </div>
    </div>
  );
}
