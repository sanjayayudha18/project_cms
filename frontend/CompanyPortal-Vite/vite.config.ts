import { resolve } from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
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
      // Floor after fe-coverage C4 (2026-10-15). Raise after each slice; never lower.
      thresholds: { statements: 78, branches: 73, functions: 74, lines: 79 },
    },
  },
});
