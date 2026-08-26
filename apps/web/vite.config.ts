import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

const devBffProxyTarget = process.env.VITE_DEV_BFF_PROXY_TARGET;
const devBffProxyOrigin = process.env.VITE_DEV_BFF_PROXY_ORIGIN;

if (devBffProxyTarget && !devBffProxyOrigin) {
  throw new Error("VITE_DEV_BFF_PROXY_ORIGIN is required when VITE_DEV_BFF_PROXY_TARGET is set");
}

export default defineConfig({
  plugins: [vue()],
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
