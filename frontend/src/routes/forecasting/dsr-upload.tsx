import { DSRUploadForm } from "@/features/forecasting/components/DSRUploadForm";
import { UploadHistory } from "@/features/forecasting/components/UploadHistory";
import { createRoute } from "@tanstack/react-router";
import { protectedRoute } from "../_protected";

export const dsrUploadRoute = createRoute({
  path: "/forecasting/dsr-upload",
  getParentRoute: () => protectedRoute,
  component: DSRUploadPage,
});

function DSRUploadPage() {
  return (
    <div className="flex flex-col gap-[var(--space-6)]">
      <div className="flex flex-col gap-[var(--space-2)]">
        <h1 className="text-xl font-semibold" style={{ color: "var(--n-900)" }}>
          Unggah DSR
        </h1>
        <p className="text-sm" style={{ color: "var(--n-600)" }}>
          Unggah laporan harian saldo vault, rencana pengisian, dan hasil rekonsiliasi dari vendor.
        </p>
      </div>

      <DSRUploadForm />
      <UploadHistory />
    </div>
  );
}
