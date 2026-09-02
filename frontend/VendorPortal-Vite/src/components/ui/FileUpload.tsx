import { FileText, Upload, X } from "lucide-react";
import { useCallback, useRef, useState } from "react";

interface FileUploadProps {
  readonly maxFiles: number;
  readonly maxSizeBytes: number;
  readonly acceptedTypes: string[];
  readonly files: File[];
  readonly onFilesChange: (files: File[]) => void;
  readonly errors?: string[];
}

/**
 * File upload component with drag-and-drop, thumbnail previews, and validation.
 * Validates file type, size, and count on add. Displays error messages for violations.
 * Enforces 44px minimum touch target on all interactive elements.
 */
export function FileUpload({
  maxFiles,
  maxSizeBytes,
  acceptedTypes,
  files,
  onFilesChange,
  errors: externalErrors,
}: FileUploadProps) {
  const [isDragOver, setIsDragOver] = useState(false);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  const allErrors = [...(externalErrors ?? []), ...validationErrors];

  const validateAndAddFiles = useCallback(
    (incoming: FileList | File[]) => {
      const newErrors: string[] = [];
      const validFiles: File[] = [];
      const remainingSlots = maxFiles - files.length;

      if (remainingSlots <= 0) {
        newErrors.push(`Maksimal ${maxFiles} file per pengiriman`);
        setValidationErrors(newErrors);
        return;
      }

      const filesToProcess = Array.from(incoming).slice(0, remainingSlots);
      const rejected = Array.from(incoming).length - filesToProcess.length;

      if (rejected > 0) {
        newErrors.push(`Maksimal ${maxFiles} file per pengiriman`);
      }

      for (const file of filesToProcess) {
        if (!acceptedTypes.includes(file.type)) {
          newErrors.push(`File ${file.name} tidak didukung. Gunakan JPEG, PNG, atau PDF.`);
          continue;
        }
        if (file.size > maxSizeBytes) {
          newErrors.push(
            `File ${file.name} melebihi batas ${Math.round(maxSizeBytes / 1_048_576)}MB`,
          );
          continue;
        }
        validFiles.push(file);
      }

      setValidationErrors(newErrors);

      if (validFiles.length > 0) {
        onFilesChange([...files, ...validFiles]);
      }
    },
    [acceptedTypes, files, maxFiles, maxSizeBytes, onFilesChange],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setIsDragOver(false);
      if (e.dataTransfer.files.length > 0) {
        validateAndAddFiles(e.dataTransfer.files);
      }
    },
    [validateAndAddFiles],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
  }, []);

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      if (e.target.files && e.target.files.length > 0) {
        validateAndAddFiles(e.target.files);
      }
      // Reset input so the same file can be selected again
      if (inputRef.current) {
        inputRef.current.value = "";
      }
    },
    [validateAndAddFiles],
  );

  const handleRemove = useCallback(
    (index: number) => {
      const updated = files.filter((_, i) => i !== index);
      onFilesChange(updated);
      setValidationErrors([]);
    },
    [files, onFilesChange],
  );

  const handleZoneClick = useCallback(() => {
    inputRef.current?.click();
  }, []);

  const handleZoneKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        handleZoneClick();
      }
    },
    [handleZoneClick],
  );

  const isImage = (file: File) => file.type === "image/jpeg" || file.type === "image/png";

  return (
    <div className="flex flex-col gap-3">
      {/* Drop zone */}
      <div
        role="button"
        tabIndex={0}
        aria-label="Klik atau seret file untuk mengunggah"
        onClick={handleZoneClick}
        onKeyDown={handleZoneKeyDown}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        className={[
          "flex flex-col items-center justify-center gap-2 min-h-[120px] p-6",
          "border-2 border-dashed rounded-lg cursor-pointer",
          "transition-colors duration-150",
          "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active",
          isDragOver
            ? "border-sidebar-active bg-sidebar-active/5"
            : allErrors.length > 0
              ? "border-danger-fg/50 bg-danger-bg/30"
              : "border-neutral-300 hover:border-sidebar-active/50 hover:bg-neutral-50",
        ].join(" ")}
      >
        <Upload
          className={["size-8", isDragOver ? "text-sidebar-active" : "text-neutral-400"].join(" ")}
          aria-hidden="true"
        />
        <p className="text-sm text-neutral-500 text-center">
          Seret file ke sini atau{" "}
          <span className="font-medium text-sidebar-active">klik untuk memilih</span>
        </p>
        <p className="text-xs text-neutral-400">
          JPEG, PNG, atau PDF. Maks {Math.round(maxSizeBytes / 1_048_576)}MB per file. Maks{" "}
          {maxFiles} file.
        </p>
      </div>

      {/* Hidden file input */}
      <input
        ref={inputRef}
        type="file"
        multiple
        accept={acceptedTypes.join(",")}
        onChange={handleInputChange}
        className="hidden"
        aria-hidden="true"
        tabIndex={-1}
      />

      {/* Validation errors */}
      {allErrors.length > 0 && (
        <ul className="flex flex-col gap-1" role="alert">
          {allErrors.map((error, i) => (
            <li key={i} className="text-sm text-danger-fg">
              {error}
            </li>
          ))}
        </ul>
      )}

      {/* File previews */}
      {files.length > 0 && (
        <ul className="flex flex-wrap gap-3">
          {files.map((file, index) => (
            <li
              key={`${file.name}-${index}`}
              className="relative flex flex-col items-center gap-1 w-20"
            >
              {/* Thumbnail or file icon */}
              <div className="w-16 h-16 rounded-md border border-neutral-200 overflow-hidden flex items-center justify-center bg-neutral-50">
                {isImage(file) ? (
                  <img
                    src={URL.createObjectURL(file)}
                    alt={file.name}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <FileText className="size-8 text-neutral-400" aria-hidden="true" />
                )}
              </div>

              {/* File name */}
              <span className="text-[10px] text-neutral-500 truncate w-full text-center">
                {file.name}
              </span>

              {/* Remove button */}
              <button
                type="button"
                onClick={() => handleRemove(index)}
                aria-label={`Hapus file ${file.name}`}
                className="absolute -top-2 -right-2 inline-flex min-h-[44px] min-w-[44px] items-center justify-center"
              >
                <span className="inline-flex items-center justify-center w-5 h-5 rounded-full bg-danger-fg text-white">
                  <X className="size-3" aria-hidden="true" />
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
