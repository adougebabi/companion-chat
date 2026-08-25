import assert from "node:assert/strict";
import test from "node:test";

import { createBff } from "../src/app.js";

test("settings forwards opaque session and never returns a configured secret value", async () => {
  let body = "";
  let session = "";
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (_url, init) => {
      session = new Headers(init?.headers).get("x-fluctlight-human-session") ?? "";
      body = typeof init?.body === "string" ? init.body : "";
      return Response.json({ values: { providerUrl: "http://provider" }, configuredSecrets: ["provider:key"] });
    },
  });
  const update = await app.inject({
    method: "PUT",
    url: "/api/settings",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque-session", fluctlight_csrf: "csrf-token" },
    payload: { secrets: { "provider:key": "do-not-return" } },
  });
  assert.equal(update.statusCode, 200);
  assert.equal(session, "opaque-session");
  assert.match(body, /do-not-return/);
  assert.doesNotMatch(update.body, /do-not-return/);
  assert.match(update.body, /configuredSecrets/);
  const blocked = await app.inject({ method: "PUT", url: "/api/settings", cookies: { fluctlight_session: "opaque-session" }, payload: {} });
  assert.equal(blocked.statusCode, 403);
  await app.close();
});
