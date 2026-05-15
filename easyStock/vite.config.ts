import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/** Desktop-first layout (see easyStock UI plan): dense tables, left sidebar nav. */
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
      "@shared": path.resolve(__dirname, "shared"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:4000",
        changeOrigin: true,
        // Long-lived SSE (e.g. market daily AI) must not idle-timeout in dev.
        timeout: 1_800_000,
        proxyTimeout: 1_800_000,
      },
    },
  },
});
