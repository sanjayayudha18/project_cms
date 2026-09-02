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
