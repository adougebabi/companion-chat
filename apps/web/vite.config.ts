import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

const devApiProxyTarget = process.env.VITE_DEV_API_PROXY_TARGET;
const devApiProxyOrigin = process.env.VITE_DEV_API_PROXY_ORIGIN;

if (devApiProxyTarget && !devApiProxyOrigin) {
  throw new Error("VITE_DEV_API_PROXY_ORIGIN is required when VITE_DEV_API_PROXY_TARGET is set");
}

export default defineConfig({
  build: { target: "es2022" },
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: devApiProxyTarget
    ? {
      proxy: {
          "/api": { target: devApiProxyTarget, headers: { origin: devApiProxyOrigin } },
          "/auth": { target: devApiProxyTarget, headers: { origin: devApiProxyOrigin } },
          "/health": { target: devApiProxyTarget, headers: { origin: devApiProxyOrigin } },
        },
      }
    : undefined,
});
