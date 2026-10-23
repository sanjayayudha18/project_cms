export type AppEnv = "dev" | "uat" | "prod";

const KNOWN: readonly AppEnv[] = ["dev", "uat", "prod"];

export function parseAppEnv(raw: string | undefined): AppEnv {
  const v = raw?.trim().toLowerCase();
  return KNOWN.find((e) => e === v) ?? "dev";
}

export const APP_ENV: AppEnv = parseAppEnv(import.meta.env.VITE_APP_ENV);
export const APP_VERSION: string = __APP_VERSION__;

export const ENV_LABEL: Record<AppEnv, string> = {
  dev: "DEV",
  uat: "UAT",
  prod: "PRODUKSI",
};
