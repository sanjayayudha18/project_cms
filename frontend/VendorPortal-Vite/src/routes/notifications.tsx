import { NotificationsPage } from "@/features/notifications/NotificationsPage";
import { createRoute } from "@tanstack/react-router";
import { shellRoute } from "./_protected";

export const notificationsRoute = createRoute({
  path: "/notifications",
  getParentRoute: () => shellRoute,
  component: NotificationsPage,
});
