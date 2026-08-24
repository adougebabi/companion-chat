import { writeFile } from "node:fs/promises";

const schema = {
  openapi: "3.1.0",
  info: { title: "Fluctlight Browser Platform API", version: "0.1.0" },
  paths: {
    "/health/live": { get: { operationId: "browserLive" } },
    "/health/ready": { get: { operationId: "browserReady" } },
    "/api/platform/ping": { get: { operationId: "platformPing" } },
  },
};

await writeFile(
  new URL("../../../packages/browser-client/openapi.json", import.meta.url),
  `${JSON.stringify(schema, null, 2)}\n`,
);
