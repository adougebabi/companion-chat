import assert from "node:assert/strict";
import test from "node:test";

import { createBff } from "../src/app.js";

test("provider endpoint configuration forwards session without a secret response", async () => {
  let receivedSession = "";
  let body = "";
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (_url, init) => {
      receivedSession = new Headers(init?.headers).get("x-fluctlight-human-session") ?? "";
      body = typeof init?.body === "string" ? init.body : "";
      return new Response(null, { status: 204 });
    },
  });
  const response = await app.inject({
    method: "PUT",
    url: "/api/providers/endpoints",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque-session", fluctlight_csrf: "csrf-token" },
    payload: { endpointId: "local", kind: "openai", baseUrl: "http://provider", secretPurpose: "provider:local" },
  });
  assert.equal(response.statusCode, 204);
  assert.equal(receivedSession, "opaque-session");
  assert.match(body, /secret_purpose/);
  assert.doesNotMatch(response.body, /provider:local/);
  const blocked = await app.inject({
    method: "PUT",
    url: "/api/providers/endpoints",
    cookies: { fluctlight_session: "opaque-session" },
    payload: { endpointId: "local", kind: "openai", baseUrl: "http://provider", secretPurpose: "provider:local" },
  });
  assert.equal(blocked.statusCode, 403);
  await app.close();
});

test("provider role maps Core preflight metadata to browser-safe camelCase", async () => {
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async () => Response.json({ role: "embedding", available: true, capability_version: "dimensions:768" }),
  });
  const response = await app.inject({
    method: "PUT",
    url: "/api/providers/roles",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque-session", fluctlight_csrf: "csrf-token" },
    payload: { role: "embedding", endpointId: "local", modelId: "embed", tokenBudget: 100, timeoutSeconds: 10 },
  });
  assert.equal(response.statusCode, 200);
  assert.deepEqual(response.json(), { role: "embedding", available: true, capabilityVersion: "dimensions:768" });
  await app.close();
});

test("provider model list is an authenticated safe endpoint lookup", async () => {
  let requestedPath = "";
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (url) => {
      requestedPath = new URL(
        typeof url === "string" ? url : url instanceof URL ? url : url.url,
      ).pathname;
      return Response.json({ endpoint_id: "lmstudio", models: ["embedding", "general"] });
    },
  });
  const response = await app.inject({
    method: "GET",
    url: "/api/providers/endpoints/lmstudio/models",
    cookies: { fluctlight_session: "opaque-session" },
  });
  assert.equal(response.statusCode, 200);
  assert.equal(requestedPath, "/internal/providers/endpoints/lmstudio/models");
  assert.deepEqual(response.json(), { endpointId: "lmstudio", models: ["embedding", "general"] });
  await app.close();
});

test("creation analysis preserves initialization errors instead of masking them as generic failures", async () => {
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async () => new Response(JSON.stringify({ detail: "initialization_foundation_invalid" }), {
      status: 422,
      headers: { "content-type": "application/json" },
    }),
  });
  const response = await app.inject({
    method: "POST",
    url: "/api/fluctlight-creations/analysis",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque-session", fluctlight_csrf: "csrf-token" },
    payload: { description: "一位喜欢摄影和散步的人" },
  });
  assert.equal(response.statusCode, 422);
  assert.deepEqual(response.json(), {
    code: "initialization_foundation_invalid",
    message: "Fluctlight analysis was rejected",
  });
  await app.close();
});

test("creation analysis returns unauthenticated when Core rejects an expired session", async () => {
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async () => new Response(JSON.stringify({ detail: "unauthenticated" }), {
      status: 401,
      headers: { "content-type": "application/json" },
    }),
  });
  const response = await app.inject({
    method: "POST",
    url: "/api/fluctlight-creations/analysis",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "expired-session", fluctlight_csrf: "csrf-token" },
    payload: { description: "一位喜欢摄影和散步的人" },
  });
  assert.equal(response.statusCode, 401);
  assert.deepEqual(response.json(), {
    code: "unauthenticated",
    message: "Authentication is required",
  });
  await app.close();
});
