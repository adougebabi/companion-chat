import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

const devBffProxyTarget = process.env.VITE_DEV_BFF_PROXY_TARGET;
const devBffProxyOrigin = process.env.VITE_DEV_BFF_PROXY_ORIGIN;

if (devBffProxyTarget && !devBffProxyOrigin) {
  throw new Error("VITE_DEV_BFF_PROXY_ORIGIN is required when VITE_DEV_BFF_PROXY_TARGET is set");
}

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: devBffProxyTarget
    ? {
      proxy: {
          "/api": { target: devBffProxyTarget, headers: { origin: devBffProxyOrigin } },
          "/auth": { target: devBffProxyTarget, headers: { origin: devBffProxyOrigin } },
          "/health": { target: devBffProxyTarget, headers: { origin: devBffProxyOrigin } },
        },
      }
    : undefined,
});
