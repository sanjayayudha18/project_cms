import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { DefaultErrorFallback } from "./components/feedback";
import { routeTree } from "./routeTree";
import "./styles/index.css";

// ─── Router Instance ──────────────────────────────────────────────────────────

// Render errors in any route show the fallback instead of a blank screen (FE-07).
const router = createRouter({
  routeTree,
  defaultErrorComponent: ({ error }) => (
    <DefaultErrorFallback
      error={error as Error}
      resetErrorBoundary={() => window.location.reload()}
    />
  ),
});

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
