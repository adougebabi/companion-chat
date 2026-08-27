import assert from "node:assert/strict";
import test from "node:test";

import { createBff } from "../src/app.js";

test("control center maps redacted diagnostics and keeps mutation origin checks", async () => {
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (url, init) => {
      const requestUrl = typeof url === "string" ? new URL(url) : url instanceof URL ? url : new URL(url.url);
      if (requestUrl.pathname === "/internal/diagnostics" && init?.method !== "DELETE") {
        return Response.json([{ id: "diag-1", event_type: "turn", severity: "info", correlation_id: "corr-1", payload: { visible: "ok" } }]);
      }
      return Response.json({ cleared: 1 });
    },
  });
  const read = await app.inject({ method: "GET", url: "/api/diagnostics", cookies: { fluctlight_session: "opaque" } });
  assert.equal(read.statusCode, 200);
  assert.equal(read.json()[0].correlationId, "corr-1");
  const blocked = await app.inject({ method: "DELETE", url: "/api/diagnostics", cookies: { fluctlight_session: "opaque" } });
  assert.equal(blocked.statusCode, 403);
  const cleared = await app.inject({ method: "DELETE", url: "/api/diagnostics", headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" }, cookies: { fluctlight_session: "opaque", fluctlight_csrf: "csrf-token" } });
  assert.equal(cleared.statusCode, 200);
  assert.deepEqual(cleared.json(), { cleared: 1 });
  await app.close();
});

test("diagnostics runtime failures are not misreported as owner authorization failures", async () => {
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    fetcher: async () => Response.json({ detail: "diagnostics_store_unavailable" }, { status: 503 }),
  });
  const response = await app.inject({ method: "GET", url: "/api/diagnostics", cookies: { fluctlight_session: "opaque" } });
  assert.equal(response.statusCode, 503);
  assert.equal(response.json().code, "diagnostics_runtime_unavailable");
  await app.close();
});

test("retiring a Fluctlight requires CSRF and forwards an auditable reason", async () => {
  let forwardedBody: unknown;
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (url, init) => {
      const requestUrl = typeof url === "string" ? new URL(url) : url instanceof URL ? url : new URL(url.url);
      assert.equal(requestUrl.pathname, "/internal/fluctlights/fl-1/retire");
      forwardedBody = init?.body ? JSON.parse(String(init.body)) : null;
      return Response.json({ id: "fl-1", status: "retired", retired_at: "2026-08-27T00:00:00Z" });
    },
  });
  const blocked = await app.inject({
    method: "POST",
    url: "/api/fluctlights/fl-1/retire",
    payload: { expectedRevision: 0, reason: "不再使用" },
    cookies: { fluctlight_session: "opaque" },
  });
  assert.equal(blocked.statusCode, 403);
  const retired = await app.inject({
    method: "POST",
    url: "/api/fluctlights/fl-1/retire",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    payload: { expectedRevision: 0, reason: "不再使用" },
    cookies: { fluctlight_session: "opaque", fluctlight_csrf: "csrf-token" },
  });
  assert.equal(retired.statusCode, 200);
  assert.deepEqual(forwardedBody, { expected_revision: 0, reason: "不再使用" });
  assert.equal(retired.json().status, "retired");
  await app.close();
});
