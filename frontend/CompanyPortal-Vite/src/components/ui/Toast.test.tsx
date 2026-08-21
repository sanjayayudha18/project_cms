import { useToastStore } from "@/lib/hooks/useToast";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastContainer } from "./Toast";

describe("Toast System", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // Reset store between tests
    useToastStore.setState({ toasts: [] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders toast container with aria-live polite", () => {
    render(<ToastContainer />);
    expect(document.querySelector('[aria-live="polite"]')).toBeInTheDocument();
  });

  it("displays a success toast with correct icon and label", () => {
    render(<ToastContainer />);

    act(() => {
      useToastStore.getState().toast({ type: "success", message: "Data tersimpan" });
    });

    expect(screen.getByText("Berhasil")).toBeInTheDocument();
    expect(screen.getByText("Data tersimpan")).toBeInTheDocument();
  });

  it("displays an error toast with correct icon and label", () => {
    render(<ToastContainer />);

    act(() => {
      useToastStore.getState().toast({ type: "error", message: "Gagal menyimpan" });
    });

    expect(screen.getByText("Gagal")).toBeInTheDocument();
    expect(screen.getByText("Gagal menyimpan")).toBeInTheDocument();
  });

  it("auto-dismisses success toast after 5 seconds", () => {
    render(<ToastContainer />);

    act(() => {
      useToastStore.getState().toast({ type: "success", message: "Upload berhasil" });
    });

    expect(screen.getByText("Upload berhasil")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(5000);
    });

    expect(screen.queryByText("Upload berhasil")).not.toBeInTheDocument();
  });

  it("does NOT auto-dismiss error toast", () => {
    render(<ToastContainer />);

    act(() => {
      useToastStore.getState().toast({ type: "error", message: "Koneksi gagal" });
    });

    expect(screen.getByText("Koneksi gagal")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(10000);
    });

    // Error toast still present after 10s
    expect(screen.getByText("Koneksi gagal")).toBeInTheDocument();
  });

  it("manually dismisses error toast via close button", async () => {
    vi.useRealTimers();
    const user = userEvent.setup();
    render(<ToastContainer />);

    act(() => {
      useToastStore.getState().toast({ type: "error", message: "Error persisten" });
    });

    expect(screen.getByText("Error persisten")).toBeInTheDocument();

    const closeBtn = screen.getByLabelText("Tutup notifikasi");
    await user.click(closeBtn);

    expect(screen.queryByText("Error persisten")).not.toBeInTheDocument();
  });

  it("enforces max 5 visible toasts, removing oldest", () => {
    render(<ToastContainer />);

    act(() => {
      const store = useToastStore.getState();
      store.toast({ type: "info", message: "Toast 1" });
      store.toast({ type: "info", message: "Toast 2" });
      store.toast({ type: "info", message: "Toast 3" });
      store.toast({ type: "info", message: "Toast 4" });
      store.toast({ type: "info", message: "Toast 5" });
      store.toast({ type: "info", message: "Toast 6" });
    });

    // Toast 1 should be removed (oldest), Toast 2-6 visible
    expect(screen.queryByText("Toast 1")).not.toBeInTheDocument();
    expect(screen.getByText("Toast 2")).toBeInTheDocument();
    expect(screen.getByText("Toast 6")).toBeInTheDocument();
  });

  it("dismissAll removes all toasts", () => {
    render(<ToastContainer />);

    act(() => {
      const store = useToastStore.getState();
      store.toast({ type: "success", message: "A" });
      store.toast({ type: "error", message: "B" });
    });

    expect(screen.getByText("A")).toBeInTheDocument();
    expect(screen.getByText("B")).toBeInTheDocument();

    act(() => {
      useToastStore.getState().dismissAll();
    });

    expect(screen.queryByText("A")).not.toBeInTheDocument();
    expect(screen.queryByText("B")).not.toBeInTheDocument();
  });

  it("returns unique IDs for each toast", () => {
    const store = useToastStore.getState();
    const id1 = store.toast({ type: "success", message: "First" });
    const id2 = store.toast({ type: "success", message: "Second" });

    expect(id1).toBeDefined();
    expect(id2).toBeDefined();
    expect(id1).not.toBe(id2);
  });

  it("dismiss removes only the targeted toast", () => {
    render(<ToastContainer />);

    let targetId = "";
    act(() => {
      const store = useToastStore.getState();
      store.toast({ type: "error", message: "Keep me" });
      targetId = store.toast({ type: "error", message: "Remove me" });
    });

    expect(screen.getByText("Keep me")).toBeInTheDocument();
    expect(screen.getByText("Remove me")).toBeInTheDocument();

    act(() => {
      useToastStore.getState().dismiss(targetId);
    });

    expect(screen.getByText("Keep me")).toBeInTheDocument();
    expect(screen.queryByText("Remove me")).not.toBeInTheDocument();
  });
});
