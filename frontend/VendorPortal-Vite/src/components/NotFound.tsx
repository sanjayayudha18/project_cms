import { Link } from 'react-router';
import { FileQuestion } from 'lucide-react';

export function NotFound() {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 p-8">
      <FileQuestion
        className="size-16 text-neutral-400"
        aria-hidden="true"
      />
      <h1 className="text-xl font-semibold text-surface-text">
        Halaman tidak ditemukan
      </h1>
      <Link
        to="/orders"
        className="rounded-md bg-sidebar-active px-4 py-2.5 text-sm font-medium text-white
                   transition-colors hover:bg-sidebar-active/90
                   focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active
                   min-h-[44px] inline-flex items-center"
      >
        Kembali ke beranda
      </Link>
    </div>
  );
}
