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
  server: {
    port: 5174,
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test-setup.ts"],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    // fe-coverage (FE-13): `pnpm test -- --coverage`. Thresholds only ever go up, to 80.
    coverage: {
      provider: "v8",
      include: ["src/**"],
      exclude: [
        "src/**/*.{test,spec}.{ts,tsx}",
        "src/**/__tests__/**",
        "src/test/**",
        "src/test-setup.ts",
        "src/main.tsx",
        "src/**/*.d.ts",
        "src/data/**",
      ],
      // Floor after fe-coverage C5 (2026-10-19): FE-13 target (80 %) reached. Never lower.
      thresholds: { statements: 93, branches: 84, functions: 90, lines: 93 },
    },
  },
});
