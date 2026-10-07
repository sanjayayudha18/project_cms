import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { NotificationListResponse } from "./api";

const fetchUnreadCount = vi.fn();
const fetchNotifications = vi.fn();
const markNotificationRead = vi.fn();
const markAllNotificationsRead = vi.fn();
vi.mock("./api", () => ({
  fetchUnreadCount: () => fetchUnreadCount(),
  fetchNotifications: (...a: unknown[]) => fetchNotifications(...a),
  markNotificationRead: (id: number) => markNotificationRead(id),
  markAllNotificationsRead: () => markAllNotificationsRead(),
}));

const push = vi.fn();
vi.mock("@tanstack/react-router", () => ({ useRouter: () => ({ history: { push } }) }));

import { NotificationBell } from "./NotificationBell";

function renderBell() {
  // staleTime mirrors the app default (routes/__root.tsx) so R1 is exercised.
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 5 * 60 * 1000 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <NotificationBell />
    </QueryClientProvider>,
  );
}

const list: NotificationListResponse = {
  items: [
    {
      id: 1,
      type: "visit_quota.over_quota",
      title: "Kelebihan kuota kunjungan",
      body: "Vendor request REP1: ATM T001 melebihi kuota kunjungan replenish.",
      link: "/replenishment/vendor-requests/9",
      is_read: false,
      created_at: "2026-10-07T02:00:00Z",
    },
    {
      id: 2,
      type: "x",
      title: "Sudah dibaca",
      body: "",
      link: null,
      is_read: true,
      created_at: "2026-10-06T02:00:00Z",
    },
  ],
  total: 2,
  page: 1,
  page_size: 10,
};

describe("NotificationBell", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchNotifications.mockResolvedValue(list);
    markNotificationRead.mockResolvedValue(undefined);
    markAllNotificationsRead.mockResolvedValue(1);
  });

  it("hides the badge when there is nothing unread", async () => {
    fetchUnreadCount.mockResolvedValue(0);
    renderBell();
    await waitFor(() => expect(fetchUnreadCount).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: "Notifikasi" })).toBeInTheDocument();
    expect(screen.queryByTestId("notification-badge")).not.toBeInTheDocument();
  });

  it("shows 99+ as text and in the accessible name", async () => {
    fetchUnreadCount.mockResolvedValue(120);
    renderBell();
    expect(await screen.findByTestId("notification-badge")).toHaveTextContent("99+");
    expect(
      screen.getByRole("button", { name: "Notifikasi, 120 belum dibaca" }),
    ).toBeInTheDocument();
  });

  it("opens the panel and marks all read without refetching", async () => {
    fetchUnreadCount.mockResolvedValue(1);
    renderBell();
    fireEvent.click(await screen.findByRole("button", { name: "Notifikasi, 1 belum dibaca" }));
    expect(await screen.findByText("Kelebihan kuota kunjungan")).toBeInTheDocument();
    expect(screen.getByText("Baru")).toBeInTheDocument(); // unread marked by text, not colour only

    fireEvent.click(screen.getByRole("button", { name: "Tandai semua dibaca" }));
    await waitFor(() => expect(screen.queryByTestId("notification-badge")).not.toBeInTheDocument());
    expect(markAllNotificationsRead).toHaveBeenCalledTimes(1);
    expect(fetchUnreadCount).toHaveBeenCalledTimes(1);
    expect(fetchNotifications).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("Baru")).not.toBeInTheDocument();
  });

  it("clicking an unread item marks it read and navigates to its link", async () => {
    fetchUnreadCount.mockResolvedValue(1);
    renderBell();
    fireEvent.click(await screen.findByRole("button", { name: "Notifikasi, 1 belum dibaca" }));
    fireEvent.click(await screen.findByText("Kelebihan kuota kunjungan"));

    await waitFor(() => expect(markNotificationRead).toHaveBeenCalledWith(1));
    expect(push).toHaveBeenCalledWith("/replenishment/vendor-requests/9");
    await waitFor(() => expect(screen.queryByTestId("notification-badge")).not.toBeInTheDocument());
  });

  it("closes the panel on Escape", async () => {
    fetchUnreadCount.mockResolvedValue(0);
    renderBell();
    fireEvent.click(screen.getByRole("button", { name: "Notifikasi" }));
    expect(await screen.findByRole("dialog", { name: "Notifikasi" })).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("refetches the list every time the panel is opened", async () => {
    fetchUnreadCount.mockResolvedValue(0);
    renderBell();
    const bell = screen.getByRole("button", { name: "Notifikasi" });
    fireEvent.click(bell);
    await screen.findByText("Kelebihan kuota kunjungan");
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(bell);
    await waitFor(() => expect(fetchNotifications).toHaveBeenCalledTimes(2));
  });
});
