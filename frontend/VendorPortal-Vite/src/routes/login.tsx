import { LoginPage } from "@/features/auth/LoginPage";
import { createRoute } from "@tanstack/react-router";
import { z } from "zod";
import { authRoute } from "./_auth";

/**
 * Returns true only for safe internal paths: must start with a single "/".
 * Rejects protocol-relative ("//evil.com") and absolute URLs ("http://", "https://",
 * which do not start with "/") to prevent open-redirect (Requirement 3.4).
 */
export function isSafeInternalPath(value: string): boolean {
  return value.startsWith("/") && !value.startsWith("//");
}

/**
 * Typed + validated search schema for the `redirect` param (Requirement 3.4, 3.8).
 * Unsafe or missing values transform to `undefined`, so LoginPage falls back to the
 * default destination (/orders).
 */
export const redirectSearchSchema = z.object({
  redirect: z
    .string()
    .optional()
    .transform((v) => (v && isSafeInternalPath(v) ? v : undefined)),
});

export type RedirectSearch = z.infer<typeof redirectSearchSchema>;

export const loginRoute = createRoute({
  path: "/login",
  getParentRoute: () => authRoute,
  validateSearch: redirectSearchSchema,
  component: LoginPage,
});
