/**
 * fe-coverage C5: handover-evidence prototype (hidden from the route tree by fe-tech-debt S2,
 * still counted per fe-coverage Q1). Data comes from src/data/evidence.json.
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const m = vi.hoisted(() => ({
  params: { id: "CIT-2024-001" } as { id?: string },
  upload: { mutate: vi.fn(), isPending: false, isError: false },
  evidence: null as null | { isLoading: boolean; data: unknown },
}));
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: ReactNode }) => <a href="/orders">{children}</a>,
  useParams: () => m.params,
}));
vi.mock("@/features/evidence/useEvidence", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/evidence/useEvidence")>();
  return {
    ...actual,
    useEvidence: (id: string) => (m.evidence ? m.evidence : actual.useEvidence(id)),
    useUploadEvidence: () => m.upload,
  };
});

import { EvidenceForm } from "../EvidenceForm";
import { EvidencePage } from "../EvidencePage";
import { useEvidence } from "../useEvidence";

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

/** datetime-local value for "now minus `hours`" in local time. */
function localInput(hoursAgo: number): string {
  const d = new Date(Date.now() - hoursAgo * 3_600_000);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

const pdf = () => new File(["x"], "bukti.pdf", { type: "application/pdf" });

beforeEach(() => {
  m.params = { id: "CIT-2024-001" };
  m.upload = { mutate: vi.fn(), isPending: false, isError: false };
  m.evidence = null;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useEvidence", () => {
  it("finds an order's evidence, null for an unknown order, idle without one", async () => {
    const found = renderHook(() => useEvidence("CIT-2024-001"), { wrapper });
    await waitFor(() => expect(found.result.current.isSuccess).toBe(true));
    expect(found.result.current.data).toMatchObject({ recipientName: "Pak Rudi Hartono" });

    const missing = renderHook(() => useEvidence("NOPE"), { wrapper });
    await waitFor(() => expect(missing.result.current.isSuccess).toBe(true));
    expect(missing.result.current.data).toBeNull();

    const idle = renderHook(() => useEvidence(""), { wrapper });
    expect(idle.result.current.fetchStatus).toBe("idle");
  });
});

describe("EvidencePage", () => {
  it("shows loading", () => {
    m.evidence = { isLoading: true, data: undefined };
    render(<EvidencePage />, { wrapper });

    expect(screen.getByText("Memuat data...")).toBeInTheDocument();
  });

  it("shows evidence already uploaded: image + file thumbnails, recipient and notes", () => {
    m.evidence = {
      isLoading: false,
      data: {
        files: [
          { name: "foto.jpg", url: "/e/foto.jpg", type: "image/jpeg" },
          { name: "ba.pdf", url: "/e/ba.pdf", type: "application/pdf" },
        ],
        handoverTimestamp: "2026-10-14T03:00:00Z",
        recipientName: "Bu Sari",
        notes: "Kondisi baik",
        uploadedAt: "2026-10-14T04:00:00Z",
      },
    };
    render(<EvidencePage />, { wrapper });

    expect(screen.getByText(/Bukti serah terima sudah diunggah pada/)).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "foto.jpg" })).toBeInTheDocument();
    expect(screen.queryByRole("img", { name: "ba.pdf" })).not.toBeInTheDocument();
    expect(screen.getByText("ba.pdf")).toBeInTheDocument();
    expect(screen.getByText("Bu Sari")).toBeInTheDocument();
    expect(screen.getByText("Kondisi baik")).toBeInTheDocument();
    expect(screen.getByText("Order: CIT-2024-001")).toBeInTheDocument();
  });

  it("shows the upload form when there is no evidence yet (no order id line without an id)", () => {
    m.params = {};
    m.evidence = { isLoading: false, data: null };
    render(<EvidencePage />, { wrapper });

    expect(screen.getByRole("button", { name: "Unggah Bukti" })).toBeInTheDocument();
    expect(screen.queryByText(/^Order:/)).not.toBeInTheDocument();
  });
});

describe("EvidenceForm", () => {
  async function addFile() {
    await userEvent
      .setup()
      .upload(document.querySelector('input[type="file"]') as HTMLInputElement, pdf());
  }

  it("requires a file, a handover time and a recipient", async () => {
    render(<EvidenceForm orderId="CIT-1" />);

    await userEvent.setup().click(screen.getByRole("button", { name: "Unggah Bukti" }));

    expect(await screen.findByText("Waktu serah terima wajib diisi")).toBeInTheDocument();
    expect(screen.getByText("Nama penerima wajib diisi")).toBeInTheDocument();
    expect(m.upload.mutate).not.toHaveBeenCalled();
  });

  it("asks for a file when the fields are valid but nothing is attached", async () => {
    render(<EvidenceForm orderId="CIT-1" />);

    fireEvent.change(screen.getByLabelText(/Waktu Serah Terima/), {
      target: { value: localInput(1) },
    });
    await userEvent.setup().type(screen.getByLabelText(/Nama Penerima/), "Pak Budi");
    await userEvent.setup().click(screen.getByRole("button", { name: "Unggah Bukti" }));

    expect(await screen.findByText("Minimal 1 file wajib diunggah")).toBeInTheDocument();
    expect(m.upload.mutate).not.toHaveBeenCalled();
  });

  it.each([
    [-2, "Waktu serah terima tidak boleh di masa depan"],
    [80, "Waktu serah terima tidak boleh lebih dari 72 jam yang lalu"],
  ])("rejects a handover time %i hours ago", async (hoursAgo, message) => {
    render(<EvidenceForm orderId="CIT-1" />);

    fireEvent.change(screen.getByLabelText(/Waktu Serah Terima/), {
      target: { value: localInput(hoursAgo) },
    });
    await userEvent.setup().click(screen.getByRole("button", { name: "Unggah Bukti" }));

    expect(await screen.findByText(message)).toBeInTheDocument();
  });

  it("rejects a note over 500 characters", async () => {
    render(<EvidenceForm orderId="CIT-1" />);

    fireEvent.change(screen.getByLabelText(/Catatan/), { target: { value: "x".repeat(501) } });
    await userEvent.setup().click(screen.getByRole("button", { name: "Unggah Bukti" }));

    expect(await screen.findByText("Catatan maksimal 500 karakter")).toBeInTheDocument();
  });

  it("submits the evidence and shows the success screen", async () => {
    m.upload.mutate.mockImplementation((_data, opts: { onSuccess: () => void }) =>
      opts.onSuccess(),
    );
    render(<EvidenceForm orderId="CIT-1" />);

    await addFile();
    const when = localInput(2);
    fireEvent.change(screen.getByLabelText(/Waktu Serah Terima/), { target: { value: when } });
    await userEvent.setup().type(screen.getByLabelText(/Nama Penerima/), "Pak Budi");
    await userEvent.setup().click(screen.getByRole("button", { name: "Unggah Bukti" }));

    expect(await screen.findByText("Bukti berhasil diunggah")).toBeInTheDocument();
    const [payload] = m.upload.mutate.mock.calls[0] as [Record<string, unknown>];
    expect(payload).toMatchObject({
      orderId: "CIT-1",
      handoverTimestamp: when,
      recipientName: "Pak Budi",
      notes: undefined,
    });
    expect((payload.files as File[]).map((f) => f.name)).toEqual(["bukti.pdf"]);
  });

  it("shows the upload error and the pending state", () => {
    m.upload = { mutate: vi.fn(), isPending: true, isError: true };
    render(<EvidenceForm orderId="CIT-1" />);

    expect(screen.getByRole("alert")).toHaveTextContent("Gagal mengunggah");
    expect(screen.getByText("Sedang mengunggah file...")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Mengunggah/ })).toBeDisabled();
  });
});
