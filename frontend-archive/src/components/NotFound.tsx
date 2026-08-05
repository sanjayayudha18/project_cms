import { Link } from 'react-router-dom';
import { FileQuestion } from 'lucide-react';

export function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-24 text-center">
      <FileQuestion size={48} className="text-n-400" />
      <h2 className="text-xl font-semibold text-n-900">Page not found</h2>
      <p className="text-sm text-n-600 max-w-[40ch]">
        The page you're looking for doesn't exist or has been moved.
      </p>
      <Link
        to="/dashboard"
        className="inline-flex h-11 items-center rounded-md bg-red-500 px-5 text-sm font-medium text-white hover:bg-red-600 transition-colors duration-150"
      >
        Go to Dashboard
      </Link>
    </div>
  );
}
