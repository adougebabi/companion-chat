import assert from "node:assert/strict";
import test from "node:test";

import { createBff } from "../src/app.js";

test("BFF exposes only transport health and maps Core failures", async () => {
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    fetcher: async () => new Response("unavailable", { status: 503 }),
  });
  const live = await app.inject({ method: "GET", url: "/health/live" });
  const ready = await app.inject({ method: "GET", url: "/health/ready" });
  const ping = await app.inject({ method: "GET", url: "/api/platform/ping" });
  assert.equal(live.statusCode, 200);
  assert.equal(ready.statusCode, 503);
  assert.equal(ping.statusCode, 502);
  await app.close();
});
