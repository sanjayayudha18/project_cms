import { useAuth } from "@/features/auth/useAuth";
import { Lock } from "lucide-react";

/** Shown when the account holds no menu permission at all (Notifikasi stays in the sidebar). */
export function NoAccess() {
  const { logout } = useAuth();
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 p-8 text-center">
      <Lock className="size-16 text-neutral-400" aria-hidden="true" />
      <h1 className="text-xl font-semibold text-surface-text">Anda belum memiliki akses menu</h1>
      <p className="max-w-md text-sm text-neutral-600">
        Peran akun Anda belum diberi izin ke menu portal. Hubungi administrator CIMB Niaga untuk
        meminta akses.
      </p>
      <button
        type="button"
        onClick={() => void logout()}
        className="inline-flex min-h-[44px] items-center rounded-md bg-sidebar-active px-4 py-2.5 text-sm font-medium text-white
                   transition-colors hover:bg-sidebar-active/90
                   focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sidebar-active"
      >
        Keluar
      </button>
    </div>
  );
}
