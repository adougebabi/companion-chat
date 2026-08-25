import assert from "node:assert/strict";
import test from "node:test";

import { createBff } from "../src/app.js";

test("media route proxies Core bytes and forwards a bounded Range header", async () => {
  let range = "";
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    fetcher: async (url, init) => {
      const requestUrl = typeof url === "string" ? new URL(url) : url instanceof URL ? url : new URL(url.url);
      if (requestUrl.pathname.startsWith("/internal/media/")) {
        range = new Headers(init?.headers).get("Range") ?? "";
        return new Response("bytes", { status: 206, headers: { "content-type": "image/png", "content-range": "bytes 0-4/5", etag: "etag-1" } });
      }
      return new Response(null, { status: 204 });
    },
  });
  const response = await app.inject({ method: "GET", url: "/api/media/asset-1", headers: { range: "bytes=0-4" }, cookies: { fluctlight_session: "opaque" } });
  assert.equal(response.statusCode, 206);
  assert.equal(response.body, "bytes");
  assert.equal(range, "bytes=0-4");
  await app.close();
});
