/** Peran PIC vendor (vendor-party-scope Req 5.1): replenish | vault | cashcount. */

export type PicRole = "replenish" | "vault" | "cashcount";

export const PIC_ROLE_OPTIONS: { value: PicRole; label: string }[] = [
  { value: "replenish", label: "Pengisian ATM" },
  { value: "vault", label: "Penyedia uang" },
  { value: "cashcount", label: "Cash count" },
];

export const ALL_PIC_ROLES: PicRole[] = PIC_ROLE_OPTIONS.map((o) => o.value);

/** Daftar peran sebagai teks; himpunan kosong = PIC hanya kontak. */
export function formatPicRoles(roles: readonly string[] | undefined): string {
  if (!roles || roles.length === 0) return "Hanya kontak";
  return PIC_ROLE_OPTIONS.filter((o) => roles.includes(o.value))
    .map((o) => o.label)
    .join(", ");
}
