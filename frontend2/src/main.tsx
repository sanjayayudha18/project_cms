import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { rootRoute } from "./routes/__root";
import { authRoute } from "./routes/_auth";
import { protectedRoute } from "./routes/_protected";
import { cashCountRoute } from "./routes/cash-count/index";
import { dsrUploadRoute } from "./routes/forecasting/dsr-upload";
import { forecastingRoute } from "./routes/forecasting/index";
import { indexRoute } from "./routes/index";
import { invoiceRoute } from "./routes/invoice/index";
import { loginRoute } from "./routes/login";
import "./styles/index.css";

// ─── Route Tree ───────────────────────────────────────────────────────────────

const routeTree = rootRoute.addChildren([
  authRoute.addChildren([loginRoute]),
  protectedRoute.addChildren([
    indexRoute,
    forecastingRoute,
    dsrUploadRoute,
    invoiceRoute,
    cashCountRoute,
  ]),
]);

// ─── Router Instance ──────────────────────────────────────────────────────────

const router = createRouter({ routeTree });

// Register the router for type safety
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

// ─── Render ───────────────────────────────────────────────────────────────────

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}

createRoot(rootElement).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
