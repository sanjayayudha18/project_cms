import { BackLink } from "@/components/ui/BackLink";
import { PageHeader } from "@/components/ui/PageHeader";
import { useNavigate } from "@tanstack/react-router";
import { UserForm } from "./UserForm";

/** Tambah Pengguna on its own page (small-works 2026-11-20); edit stays a dialog. */
export function CreateUserPage() {
  const navigate = useNavigate();
  const backToList = () => navigate({ to: "/settings/admin/users" });

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="self-start">
        <BackLink to="/settings/admin/users" label="Kembali ke Manajemen Pengguna" />
      </div>
      <PageHeader eyebrow="Manajemen Pengguna" title="Tambah Pengguna" />
      <div className="w-full max-w-lg rounded-lg border border-[var(--n-200)] bg-[var(--n-0)] p-6">
        <UserForm user={null} onSaved={backToList} onCancel={backToList} />
      </div>
    </div>
  );
}
