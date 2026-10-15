import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

// fe-coverage C4: vendor notification inbox. Notification hooks + router are mocked.
const m = vi.hoisted(() => ({
  list: {} as Record<string, unknown>,
  unread: 0,
  markAsRead: vi.fn(),
  markAll: vi.fn(),
  navigate: vi.fn(),
}));
vi.mock("../useNotifications", () => ({
  useNotifications: () => m.list,
  useUnreadCount: () => ({ data: m.unread }),
  useMarkAsRead: () => ({ mutate: m.markAsRead }),
  useMarkAllAsRead: () => ({ mutate: m.markAll, isPending: false }),
}));
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => m.navigate }));

import { NotificationsPage } from "../NotificationsPage";

const UNREAD = {
  id: 1,
  title: "Order baru",
  body: "REP-G-JKT-20261016-001 menunggu konfirmasi Anda",
  link: "/orders/55",
  is_read: false,
  created_at: "2026-10-15T02:30:00Z",
};
const READ = {
  id: 2,
  title: "DSR terlambat",
  body: "x".repeat(130),
  link: null,
  is_read: true,
  created_at: "2026-10-14T17:05:00Z",
};

beforeEach(() => {
  m.list = { data: [UNREAD, READ], isLoading: false, isError: false };
  m.unread = 1;
  for (const fn of [m.markAsRead, m.markAll, m.navigate]) fn.mockReset();
});

describe("NotificationsPage", () => {
  it("shows loading, error and empty states", () => {
    m.list = { data: undefined, isLoading: true, isError: false };
    const { rerender } = render(<NotificationsPage />);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();

    m.list = { data: undefined, isLoading: false, isError: true };
    rerender(<NotificationsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat notifikasi");

    m.list = { data: [], isLoading: false, isError: false };
    rerender(<NotificationsPage />);
    expect(screen.getByText("Tidak ada notifikasi")).toBeInTheDocument();
  });

  it("lists notifications with Jakarta time, truncated body and a text read status", () => {
    render(<NotificationsPage />);

    expect(screen.getByText("Order baru")).toBeInTheDocument();
    expect(screen.getByText("Unread")).toBeInTheDocument();
    expect(screen.getByText("Read")).toBeInTheDocument();
    // 2026-10-15T02:30Z = 09:30 WIB; 2026-10-14T17:05Z = 15 Okt 00:05 WIB
    expect(screen.getByText(/09\.30/)).toBeInTheDocument();
    expect(screen.getByText(/15 Okt 2026.*00\.05/)).toBeInTheDocument();
    const truncated = screen.getByText(/^x+/);
    expect(truncated.textContent?.length).toBeLessThan(130);
  });

  it("opening an unread notification marks it read and follows its link", async () => {
    render(<NotificationsPage />);

    await userEvent.setup().click(screen.getByRole("row", { name: /Order baru/ }));

    expect(m.markAsRead).toHaveBeenCalledWith(1);
    expect(m.navigate).toHaveBeenCalledWith({ href: "/orders/55" });
  });

  it("a read notification without a link does nothing on open (keyboard too)", () => {
    render(<NotificationsPage />);
    const row = screen.getByRole("row", { name: /DSR terlambat/ });

    fireEvent.keyDown(row, { key: "Enter" });
    fireEvent.keyDown(row, { key: " " });
    fireEvent.keyDown(row, { key: "a" });

    expect(m.markAsRead).not.toHaveBeenCalled();
    expect(m.navigate).not.toHaveBeenCalled();
  });

  it("Enter on an unread row opens it like a click", () => {
    render(<NotificationsPage />);

    fireEvent.keyDown(screen.getByRole("row", { name: /Order baru/ }), { key: "Enter" });

    expect(m.markAsRead).toHaveBeenCalledWith(1);
  });

  it("'Mark all as read' works only while something is unread", async () => {
    const { rerender } = render(<NotificationsPage />);
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Mark all notifications as read" }));
    expect(m.markAll).toHaveBeenCalledTimes(1);

    m.unread = 0;
    rerender(<NotificationsPage />);
    expect(screen.getByRole("button", { name: "Mark all notifications as read" })).toBeDisabled();
  });
});
