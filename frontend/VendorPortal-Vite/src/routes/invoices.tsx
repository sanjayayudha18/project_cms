import { InvoicesPage } from "@/features/invoices/InvoicesPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const invoicesRoute = createRoute({
  path: "/invoices",
  getParentRoute: () => shellRoute,
  component: InvoicesPage,
});
