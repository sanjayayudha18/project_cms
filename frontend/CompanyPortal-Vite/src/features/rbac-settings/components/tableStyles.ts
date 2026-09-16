/**
 * Shared table classes for the RBAC settings list pages (users, delegations,
 * leaves, policies): uppercase --n-500 headers on --n-50, --n-100 row
 * dividers, --red-50 row hover (design.md).
 */

export function rbacThClass(align: "left" | "right" = "left"): string {
  return `px-3 py-2 font-medium uppercase text-xs tracking-wide text-[var(--n-500)] ${
    align === "right" ? "text-right" : "text-left"
  }`;
}

export const rbacTheadRowClass = "border-[var(--n-200)] border-b bg-[var(--n-50)]";
export const rbacRowClass = "border-[var(--n-100)] border-b last:border-0 hover:bg-[var(--red-50)]";
export const rbacTdClass = "px-3 py-2";
