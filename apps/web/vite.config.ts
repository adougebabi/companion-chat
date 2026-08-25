import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

const devBffProxyTarget = process.env.VITE_DEV_BFF_PROXY_TARGET;

export default defineConfig({
  plugins: [vue()],
  server: devBffProxyTarget
    ? {
      proxy: {
          "/api": { target: devBffProxyTarget, headers: { origin: "http://127.0.0.1:13001" } },
          "/auth": { target: devBffProxyTarget, headers: { origin: "http://127.0.0.1:13001" } },
          "/health": { target: devBffProxyTarget, headers: { origin: "http://127.0.0.1:13001" } },
        },
      }
    : undefined,
});
