import { describe, expect, it } from "vitest";
import { APP_VERSION, ENV_LABEL, parseAppEnv } from "./appEnv";

describe("parseAppEnv", () => {
  it.each(["dev", "uat", "prod"] as const)("accepts %s", (env) => {
    expect(parseAppEnv(env)).toBe(env);
  });

  it("is case- and whitespace-insensitive", () => {
    expect(parseAppEnv(" UAT ")).toBe("uat");
    expect(parseAppEnv("Prod")).toBe("prod");
  });

  it.each([undefined, "", "   ", "staging", "production"])("falls back to dev for %j", (raw) => {
    expect(parseAppEnv(raw)).toBe("dev");
  });
});

describe("ENV_LABEL", () => {
  it("has a label per env", () => {
    expect(ENV_LABEL).toEqual({ dev: "DEV", uat: "UAT", prod: "PRODUKSI" });
  });
});

describe("APP_VERSION", () => {
  it("is a non-empty string defined by Vite", () => {
    expect(typeof APP_VERSION).toBe("string");
    expect(APP_VERSION.length).toBeGreaterThan(0);
  });
});
