import { Dialog } from "@/components/ui/Dialog";
import type { AdminUser } from "../types";
import { UserForm } from "./UserForm";

interface UserFormDialogProps {
  open: boolean;
  onClose: () => void;
  user: AdminUser | null;
}

/** Dialog wrapper around UserForm; the screen uses it for edit (create has its own page). */
export function UserFormDialog({ open, onClose, user }: UserFormDialogProps) {
  return (
    <Dialog open={open} onClose={onClose} title={user ? "Ubah Pengguna" : "Tambah Pengguna"}>
      {/* Mounted per open + user so the form starts from that user's values. */}
      {open && (
        <UserForm key={user?.id ?? "new"} user={user} onSaved={onClose} onCancel={onClose} />
      )}
    </Dialog>
  );
}
