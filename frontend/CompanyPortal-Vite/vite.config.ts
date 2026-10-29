import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const pkg = JSON.parse(readFileSync(resolve(__dirname, "package.json"), "utf-8")) as {
  version: string;
};

export default defineConfig({
  define: { __APP_VERSION__: JSON.stringify(pkg.version) },
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": resolve(__dirname, "./src"),
    },
  },
  // dist/.vite/manifest.json feeds scripts/check-bundle.mjs (FE-04 initial-load budget).
  build: { manifest: true },
  server: {
    proxy: {
      // EOD monitoring is the Python eod_retry_scheduler (:8091), not the Go API; it must
      // come before "/api". /api/eod/summary -> :8091/summary
      "/api/eod": {
        target: "http://localhost:8091",
        rewrite: (path) => path.replace(/^\/api\/eod/, ""),
      },
      // Async exports live in backend-exq (:8081); must come before "/api".
      "/api/v1/exports": "http://localhost:8081",
      "/api": "http://localhost:8080",
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    // fe-coverage (FE-13): `pnpm test -- --coverage`. Thresholds only ever go up, to 80.
    coverage: {
      provider: "v8",
      include: ["src/**"],
      exclude: [
        "src/**/*.{test,spec}.{ts,tsx}",
        "src/**/__tests__/**",
        "src/test/**",
        "src/main.tsx",
        "src/**/*.d.ts",
        "src/data/**",
      ],
      // Floor after fe-coverage C5 (2026-10-19): FE-13 target (80 %) reached. Never lower.
      thresholds: { statements: 84, branches: 80, functions: 80, lines: 85 },
    },
  },
});
