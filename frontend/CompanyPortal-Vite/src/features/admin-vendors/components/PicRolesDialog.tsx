import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/lib/hooks/useToast";
import { useEffect, useState } from "react";
import { useSetPicRoles } from "../hooks";
import { PIC_ROLE_OPTIONS } from "../lib/picRoles";
import type { AdminVendorPic } from "../types";

interface PicRolesDialogProps {
  open: boolean;
  onClose: () => void;
  vendorId: number;
  pic: AdminVendorPic | null;
}

/**
 * Ubah peran PIC (vendor-party-scope Req 5.9). Berbeda dengan field PIC lain,
 * perubahan ini langsung berlaku tanpa maker-checker (audit dicatat di tx yang sama).
 */
export function PicRolesDialog({ open, onClose, vendorId, pic }: PicRolesDialogProps) {
  const { toast } = useToast();
  const mutation = useSetPicRoles(vendorId);
  const [selected, setSelected] = useState<string[]>([]);

  useEffect(() => {
    if (open && pic) setSelected(pic.roles);
  }, [open, pic]);

  function toggle(role: string, checked: boolean): void {
    setSelected((prev) => (checked ? [...prev, role] : prev.filter((r) => r !== role)));
  }

  async function onSubmit(e: React.FormEvent): Promise<void> {
    e.preventDefault();
    if (!pic) return;
    try {
      await mutation.mutateAsync({ picId: pic.id, roles: selected });
      toast({ type: "success", message: "Peran PIC diperbarui" });
      onClose();
    } catch (err) {
      const message =
        err && typeof err === "object" && "message" in err
          ? String((err as { message: unknown }).message)
          : "Terjadi kesalahan yang tidak diketahui";
      toast({ type: "error", message });
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title={`Ubah peran PIC${pic ? ` — ${pic.name}` : ""}`}>
      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <p className="text-sm text-[var(--n-700)]">
          Email tiket hanya dikirim ke PIC yang perannya cocok. Tanpa peran, PIC hanya menjadi
          kontak.
        </p>

        <fieldset className="flex flex-col gap-1">
          <legend className="mb-1 text-xs font-medium uppercase tracking-wider text-[var(--n-600)]">
            Peran
          </legend>
          {PIC_ROLE_OPTIONS.map((opt) => (
            <label
              key={opt.value}
              className="flex min-h-[44px] items-center gap-2 text-sm text-[var(--n-800)]"
            >
              <input
                type="checkbox"
                className="h-4 w-4"
                checked={selected.includes(opt.value)}
                onChange={(e) => toggle(opt.value, e.target.checked)}
              />
              {opt.label}
            </label>
          ))}
        </fieldset>

        <output className="text-xs font-medium text-[var(--warning-fg)]">
          Langsung berlaku, tanpa persetujuan.
        </output>

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" disabled={mutation.isPending} onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
