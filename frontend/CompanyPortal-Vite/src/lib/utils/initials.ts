/**
 * Avatar initials for the UserMenu (max 2 letters, uppercase).
 * Multi-word: first letter of first + last word. One word: its first 2 letters.
 * Empty/whitespace: "?".
 *
 * @example getInitials("Raden Budiman") // "RB"
 * @example getInitials("Raden") // "RA"
 */
export function getInitials(fullName: string): string {
  const parts = fullName.trim().split(/\s+/).filter(Boolean);
  const first = parts[0];
  const last = parts[parts.length - 1];
  if (first === undefined || last === undefined) return "?";
  if (parts.length === 1) return Array.from(first).slice(0, 2).join("").toUpperCase();
  return (
    Array.from(first).slice(0, 1).join("") + Array.from(last).slice(0, 1).join("")
  ).toUpperCase();
}
