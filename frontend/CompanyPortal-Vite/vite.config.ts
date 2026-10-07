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
  },
});
