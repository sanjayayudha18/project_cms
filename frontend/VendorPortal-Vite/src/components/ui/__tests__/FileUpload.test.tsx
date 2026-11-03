import { FileUpload } from "@/components/ui/FileUpload";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const defaultProps = {
  maxFiles: 5,
  maxSizeBytes: 10 * 1024 * 1024, // 10MB
  acceptedTypes: ["image/jpeg", "image/png", "application/pdf"],
  files: [] as File[],
  onFilesChange: vi.fn(),
};

function createFile(name: string, size: number, type: string): File {
  const buffer = new ArrayBuffer(size);
  return new File([buffer], name, { type });
}

describe("FileUpload", () => {
  it("displays drop zone", () => {
    render(<FileUpload {...defaultProps} />);

    expect(screen.getByLabelText("Klik atau seret file untuk mengunggah")).toBeInTheDocument();
  });

  it("rejects files exceeding maxSizeBytes with appropriate error message", async () => {
    const user = userEvent.setup();
    const onFilesChange = vi.fn();

    render(<FileUpload {...defaultProps} onFilesChange={onFilesChange} />);

    const largeFile = createFile("big.jpg", 11 * 1024 * 1024, "image/jpeg");
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;

    await user.upload(input, largeFile);

    expect(screen.getByText(/melebihi batas 10MB/i)).toBeInTheDocument();
    expect(onFilesChange).not.toHaveBeenCalled();
  });

  it("rejects files with invalid type with appropriate error message", () => {
    const onFilesChange = vi.fn();

    render(<FileUpload {...defaultProps} onFilesChange={onFilesChange} />);

    const invalidFile = createFile("doc.exe", 1024, "application/x-msdownload");
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;

    // Use fireEvent.change to bypass browser accept-attribute filtering
    fireEvent.change(input, { target: { files: [invalidFile] } });

    expect(screen.getByText(/tidak didukung\. Gunakan JPEG, PNG, atau PDF/i)).toBeInTheDocument();
    expect(onFilesChange).not.toHaveBeenCalled();
  });

  it("rejects files exceeding maxFiles count", async () => {
    const user = userEvent.setup();
    const onFilesChange = vi.fn();

    // Start with 5 files already attached (at limit)
    const existingFiles = Array.from({ length: 5 }, (_, i) =>
      createFile(`existing-${i}.jpg`, 1024, "image/jpeg"),
    );

    render(<FileUpload {...defaultProps} files={existingFiles} onFilesChange={onFilesChange} />);

    const newFile = createFile("extra.jpg", 1024, "image/jpeg");
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;

    await user.upload(input, newFile);

    expect(screen.getByText(/Maksimal 5 file per pengiriman/i)).toBeInTheDocument();
    expect(onFilesChange).not.toHaveBeenCalled();
  });

  it("shows thumbnail preview for images", () => {
    const imageFile = createFile("photo.jpg", 2048, "image/jpeg");

    // Mock URL.createObjectURL
    const mockUrl = "blob:http://localhost/mock-image";
    vi.spyOn(URL, "createObjectURL").mockReturnValue(mockUrl);

    render(<FileUpload {...defaultProps} files={[imageFile]} />);

    const img = screen.getByAltText("photo.jpg");
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute("src", mockUrl);

    vi.restoreAllMocks();
  });
});

// ─── fe-coverage C4: drop, queue limits, removal, keyboard, external errors ──

describe("FileUpload (C4 paths)", () => {
  const zone = () => screen.getByLabelText("Klik atau seret file untuk mengunggah");

  it("accepts a valid dropped file and appends it to the current list", () => {
    const onFilesChange = vi.fn();
    const existing = createFile("a.pdf", 10, "application/pdf");
    render(<FileUpload {...defaultProps} files={[existing]} onFilesChange={onFilesChange} />);
    const dropped = createFile("b.png", 10, "image/png");

    fireEvent.dragOver(zone());
    fireEvent.dragLeave(zone());
    fireEvent.drop(zone(), { dataTransfer: { files: [dropped] } });

    expect(onFilesChange).toHaveBeenCalledWith([existing, dropped]);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("an empty drop does nothing", () => {
    const onFilesChange = vi.fn();
    render(<FileUpload {...defaultProps} onFilesChange={onFilesChange} />);

    fireEvent.drop(zone(), { dataTransfer: { files: [] } });

    expect(onFilesChange).not.toHaveBeenCalled();
  });

  it("a full queue rejects every new file", () => {
    const onFilesChange = vi.fn();
    const full = [createFile("a.pdf", 10, "application/pdf")];
    render(
      <FileUpload {...defaultProps} maxFiles={1} files={full} onFilesChange={onFilesChange} />,
    );

    fireEvent.drop(zone(), {
      dataTransfer: { files: [createFile("b.pdf", 10, "application/pdf")] },
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Maksimal 1 file per pengiriman");
    expect(onFilesChange).not.toHaveBeenCalled();
  });

  it("keeps the valid files of a mixed drop and lists the problems", () => {
    const onFilesChange = vi.fn();
    render(<FileUpload {...defaultProps} maxFiles={2} onFilesChange={onFilesChange} />);
    const ok = createFile("ok.jpg", 10, "image/jpeg");

    fireEvent.drop(zone(), {
      dataTransfer: {
        files: [
          ok,
          createFile("x.exe", 10, "application/x-msdownload"),
          createFile("c.pdf", 10, "application/pdf"),
        ],
      },
    });

    expect(onFilesChange).toHaveBeenCalledWith([ok]);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Maksimal 2 file per pengiriman");
    expect(alert).toHaveTextContent("File x.exe tidak didukung");
  });

  it("removing a file clears earlier validation errors", async () => {
    const onFilesChange = vi.fn();
    const files = [
      createFile("a.pdf", 10, "application/pdf"),
      createFile("b.pdf", 10, "application/pdf"),
    ];
    render(
      <FileUpload {...defaultProps} maxFiles={2} files={files} onFilesChange={onFilesChange} />,
    );
    fireEvent.drop(zone(), {
      dataTransfer: { files: [createFile("c.pdf", 10, "application/pdf")] },
    });
    expect(screen.getByRole("alert")).toBeInTheDocument();

    await userEvent.setup().click(screen.getByRole("button", { name: "Hapus file a.pdf" }));

    expect(onFilesChange).toHaveBeenCalledWith([files[1]]);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("Enter or Space on the drop zone opens the file picker", () => {
    render(<FileUpload {...defaultProps} />);
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const click = vi.spyOn(input, "click").mockImplementation(() => {});

    fireEvent.keyDown(zone(), { key: "Enter" });
    fireEvent.keyDown(zone(), { key: " " });
    fireEvent.keyDown(zone(), { key: "a" });

    expect(click).toHaveBeenCalledTimes(2);
  });

  it("shows errors passed in by the parent (e.g. from the server)", () => {
    render(<FileUpload {...defaultProps} errors={["Server menolak file"]} />);

    expect(screen.getByRole("alert")).toHaveTextContent("Server menolak file");
  });
});
