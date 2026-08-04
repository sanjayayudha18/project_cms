import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod/v4';
import { zodResolver } from '@hookform/resolvers/zod';
import { CheckCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { FileUpload } from '@/components/ui/FileUpload';
import { useUploadEvidence } from '@/features/evidence/useEvidence';

const evidenceSchema = z.object({
  handoverTimestamp: z
    .string()
    .min(1, 'Waktu serah terima wajib diisi')
    .refine((val) => new Date(val) <= new Date(), {
      message: 'Waktu serah terima tidak boleh di masa depan',
    })
    .refine((val) => new Date(val) >= new Date(Date.now() - 72 * 60 * 60 * 1000), {
      message: 'Waktu serah terima tidak boleh lebih dari 72 jam yang lalu',
    }),
  recipientName: z
    .string()
    .min(1, 'Nama penerima wajib diisi')
    .max(100, 'Nama penerima maksimal 100 karakter'),
  notes: z
    .string()
    .max(500, 'Catatan maksimal 500 karakter')
    .optional()
    .or(z.literal('')),
});

type EvidenceFormData = z.infer<typeof evidenceSchema>;

interface EvidenceFormProps {
  readonly orderId: string;
}

export function EvidenceForm({ orderId }: EvidenceFormProps) {
  const [files, setFiles] = useState<File[]>([]);
  const [fileError, setFileError] = useState<string | null>(null);
  const [isSuccess, setIsSuccess] = useState(false);

  const uploadMutation = useUploadEvidence();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<EvidenceFormData>({
    resolver: zodResolver(evidenceSchema),
    defaultValues: {
      handoverTimestamp: '',
      recipientName: '',
      notes: '',
    },
  });

  const onSubmit = (data: EvidenceFormData) => {
    // Validate files separately
    if (files.length === 0) {
      setFileError('Minimal 1 file wajib diunggah');
      return;
    }
    if (files.length > 5) {
      setFileError('Maksimal 5 file per pengiriman');
      return;
    }

    setFileError(null);

    uploadMutation.mutate(
      {
        orderId,
        files,
        handoverTimestamp: data.handoverTimestamp,
        recipientName: data.recipientName,
        notes: data.notes || undefined,
      },
      {
        onSuccess: () => {
          setIsSuccess(true);
        },
      },
    );
  };

  if (isSuccess) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-12">
        <div className="flex items-center justify-center w-16 h-16 rounded-full bg-success-bg">
          <CheckCircle className="size-8 text-success-fg" aria-hidden="true" />
        </div>
        <h2 className="text-lg font-semibold text-surface-text">
          Bukti berhasil diunggah
        </h2>
        <p className="text-sm text-neutral-500">
          Bukti serah terima untuk order {orderId} telah tersimpan.
        </p>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-6" noValidate>
      {/* File upload */}
      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium text-surface-text">
          File Bukti <span className="text-danger-fg">*</span>
        </legend>
        <FileUpload
          maxFiles={5}
          maxSizeBytes={10_485_760}
          acceptedTypes={['image/jpeg', 'image/png', 'application/pdf']}
          files={files}
          onFilesChange={(newFiles) => {
            setFiles(newFiles);
            if (newFiles.length > 0) setFileError(null);
          }}
          errors={fileError ? [fileError] : undefined}
        />
      </fieldset>

      {/* Handover timestamp */}
      <div className="flex flex-col gap-1.5">
        <label
          htmlFor="handoverTimestamp"
          className="text-sm font-medium text-surface-text"
        >
          Waktu Serah Terima <span className="text-danger-fg">*</span>
        </label>
        <input
          id="handoverTimestamp"
          type="datetime-local"
          {...register('handoverTimestamp')}
          aria-invalid={!!errors.handoverTimestamp}
          aria-describedby={errors.handoverTimestamp ? 'handoverTimestamp-error' : undefined}
          className={[
            'min-h-[44px] px-3 py-2 rounded-md border text-sm text-surface-text bg-white',
            'transition-colors duration-150',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-active/30 focus-visible:border-sidebar-active',
            errors.handoverTimestamp
              ? 'border-danger-fg'
              : 'border-neutral-300',
          ].join(' ')}
        />
        {errors.handoverTimestamp && (
          <p id="handoverTimestamp-error" className="text-sm text-danger-fg">
            {errors.handoverTimestamp.message}
          </p>
        )}
      </div>

      {/* Recipient name */}
      <div className="flex flex-col gap-1.5">
        <label
          htmlFor="recipientName"
          className="text-sm font-medium text-surface-text"
        >
          Nama Penerima <span className="text-danger-fg">*</span>
        </label>
        <input
          id="recipientName"
          type="text"
          maxLength={100}
          placeholder="Masukkan nama penerima"
          {...register('recipientName')}
          aria-invalid={!!errors.recipientName}
          aria-describedby={errors.recipientName ? 'recipientName-error' : undefined}
          className={[
            'min-h-[44px] px-3 py-2 rounded-md border text-sm text-surface-text bg-white',
            'transition-colors duration-150',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-active/30 focus-visible:border-sidebar-active',
            errors.recipientName
              ? 'border-danger-fg'
              : 'border-neutral-300',
          ].join(' ')}
        />
        {errors.recipientName && (
          <p id="recipientName-error" className="text-sm text-danger-fg">
            {errors.recipientName.message}
          </p>
        )}
      </div>

      {/* Notes (optional) */}
      <div className="flex flex-col gap-1.5">
        <label
          htmlFor="notes"
          className="text-sm font-medium text-surface-text"
        >
          Catatan <span className="text-xs text-neutral-400">(opsional)</span>
        </label>
        <textarea
          id="notes"
          maxLength={500}
          rows={3}
          placeholder="Tambahkan catatan jika diperlukan"
          {...register('notes')}
          aria-invalid={!!errors.notes}
          aria-describedby={errors.notes ? 'notes-error' : undefined}
          className={[
            'min-h-[44px] px-3 py-2 rounded-md border text-sm text-surface-text bg-white resize-y',
            'transition-colors duration-150',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-active/30 focus-visible:border-sidebar-active',
            errors.notes
              ? 'border-danger-fg'
              : 'border-neutral-300',
          ].join(' ')}
        />
        {errors.notes && (
          <p id="notes-error" className="text-sm text-danger-fg">
            {errors.notes.message}
          </p>
        )}
      </div>

      {/* Error message from mutation */}
      {uploadMutation.isError && (
        <div role="alert" className="p-3 rounded-md bg-danger-bg text-danger-fg text-sm">
          Gagal mengunggah. Silakan coba lagi.
        </div>
      )}

      {/* Submit button */}
      <div className="flex items-center gap-3">
        <Button
          type="submit"
          variant="primary"
          isLoading={uploadMutation.isPending}
          disabled={uploadMutation.isPending}
        >
          {uploadMutation.isPending ? 'Mengunggah...' : 'Unggah Bukti'}
        </Button>

        {uploadMutation.isPending && (
          <span className="text-sm text-neutral-500">
            Sedang mengunggah file...
          </span>
        )}
      </div>
    </form>
  );
}
