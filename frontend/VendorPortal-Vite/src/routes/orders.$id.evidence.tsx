import { EvidencePage } from "@/features/evidence/EvidencePage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const evidenceRoute = createRoute({
  path: "/orders/$id/evidence",
  getParentRoute: () => shellRoute,
  component: EvidencePage,
});
