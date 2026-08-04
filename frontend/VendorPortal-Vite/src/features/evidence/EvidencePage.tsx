import { useParams, Link } from 'react-router';
import { ArrowLeft, FileText, Image, Calendar, User, StickyNote } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { useEvidence } from '@/features/evidence/useEvidence';
import { EvidenceForm } from '@/features/evidence/EvidenceForm';
import type { EvidenceFile } from '@/lib/types';

function formatDateTime(iso: string): string {
  const date = new Date(iso);
  return date.toLocaleDateString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function isImageFile(file: EvidenceFile): boolean {
  return file.type === 'image/jpeg' || file.type === 'image/png';
}

function ExistingEvidence({
  evidence,
}: {
  readonly evidence: {
    readonly files: readonly EvidenceFile[];
    readonly handoverTimestamp: string;
    readonly recipientName: string;
    readonly notes?: string;
    readonly uploadedAt: string;
  };
}) {
  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-center gap-2 text-success-fg">
        <Image className="size-5" aria-hidden="true" />
        <p className="text-sm font-medium">
          Bukti serah terima sudah diunggah pada {formatDateTime(evidence.uploadedAt)}
        </p>
      </div>

      {/* File thumbnails */}
      <div className="flex flex-col gap-2">
        <h3 className="text-sm font-medium text-surface-text">File Bukti</h3>
        <ul className="flex flex-wrap gap-3">
          {evidence.files.map((file) => (
            <li
              key={file.name}
              className="flex flex-col items-center gap-1 w-20"
            >
              <div className="w-16 h-16 rounded-md border border-neutral-200 overflow-hidden flex items-center justify-center bg-neutral-50">
                {isImageFile(file) ? (
                  <img
                    src={file.url}
                    alt={file.name}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <FileText className="size-8 text-neutral-400" aria-hidden="true" />
                )}
              </div>
              <span className="text-[10px] text-neutral-500 truncate w-full text-center">
                {file.name}
              </span>
            </li>
          ))}
        </ul>
      </div>

      {/* Metadata */}
      <div className="flex flex-col gap-3 pt-3 border-t border-neutral-200">
        <div className="flex items-center gap-2">
          <Calendar className="size-4 text-neutral-400" aria-hidden="true" />
          <span className="text-sm text-neutral-600">
            <span className="font-medium">Waktu serah terima:</span>{' '}
            {formatDateTime(evidence.handoverTimestamp)}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <User className="size-4 text-neutral-400" aria-hidden="true" />
          <span className="text-sm text-neutral-600">
            <span className="font-medium">Penerima:</span>{' '}
            {evidence.recipientName}
          </span>
        </div>
        {evidence.notes && (
          <div className="flex items-start gap-2">
            <StickyNote className="size-4 text-neutral-400 mt-0.5" aria-hidden="true" />
            <span className="text-sm text-neutral-600">
              <span className="font-medium">Catatan:</span>{' '}
              {evidence.notes}
            </span>
          </div>
        )}
      </div>
    </Card>
  );
}

export function EvidencePage() {
  const { id } = useParams<{ id: string }>();
  const orderId = id ?? '';
  const { data: evidence, isLoading } = useEvidence(orderId);

  return (
    <div className="flex flex-col gap-6">
      {/* Header with back link */}
      <div className="flex flex-col gap-2">
        <Link
          to="/orders"
          className="inline-flex items-center gap-1.5 text-sm text-sidebar-active hover:underline min-h-[44px]"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          Kembali ke Daftar Order
        </Link>
        <h1 className="text-xl font-semibold text-surface-text">
          Upload Bukti Serah Terima
        </h1>
        {orderId && (
          <p className="text-sm text-neutral-500">Order: {orderId}</p>
        )}
      </div>

      {/* Content */}
      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <span className="text-sm text-neutral-500">Memuat data...</span>
        </div>
      ) : evidence ? (
        <ExistingEvidence evidence={evidence} />
      ) : (
        <EvidenceForm orderId={orderId} />
      )}
    </div>
  );
}
