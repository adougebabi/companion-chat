import { createBff } from "./app.js";

const app = createBff({
  coreBaseUrl: process.env.CORE_BASE_URL ?? "http://core:8080",
  coreServiceKey: process.env.FLUCTLIGHT_CORE_SERVICE_KEY ?? "",
  trustedOrigin: process.env.FLUCTLIGHT_TRUSTED_ORIGIN,
});

await app.listen({ host: process.env.BFF_HOST ?? "0.0.0.0", port: Number(process.env.BFF_PORT ?? 3000) });
