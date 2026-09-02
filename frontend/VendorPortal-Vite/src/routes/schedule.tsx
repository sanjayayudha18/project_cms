import { SchedulePage } from "@/features/schedule/SchedulePage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const scheduleRoute = createRoute({
  path: "/schedule",
  getParentRoute: () => shellRoute,
  component: SchedulePage,
});
