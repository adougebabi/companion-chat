import assert from "node:assert/strict";
import test from "node:test";

import { createBff } from "../src/app.js";

test("session forwards only the opaque cookie and logout enforces trusted origin", async () => {
  let receivedSession = "";
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (_url, init) => {
      receivedSession = new Headers(init?.headers).get("x-fluctlight-human-session") ?? "";
      assert.equal(new Headers(init?.headers).get("x-fluctlight-service-key"), "internal");
      return Response.json({ authenticated: true, actorId: "human-owner" });
    },
  });
  const session = await app.inject({ method: "GET", url: "/auth/session", cookies: { fluctlight_session: "opaque" } });
  assert.equal(session.statusCode, 200);
  assert.equal(receivedSession, "opaque");
  const seedCookie = String(session.headers["set-cookie"] ?? "");
  assert.match(seedCookie, /fluctlight_csrf=/);
  assert.match(seedCookie, /SameSite=Lax/);
  const badCsrf = await app.inject({
    method: "POST",
    url: "/auth/logout",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "wrong-token" },
    cookies: { fluctlight_csrf: "csrf-token" },
  });
  assert.equal(badCsrf.statusCode, 403);
  const rejected = await app.inject({ method: "POST", url: "/auth/logout", headers: { origin: "https://attacker.invalid" } });
  assert.equal(rejected.statusCode, 403);
  const logout = await app.inject({ method: "POST", url: "/auth/logout", headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" }, cookies: { fluctlight_csrf: "csrf-token" } });
  assert.equal(logout.statusCode, 204);
  const setCookie = logout.headers["set-cookie"];
  assert.match(Array.isArray(setCookie) ? setCookie.join(";") : (setCookie ?? ""), /fluctlight_session=;/);
  await app.close();
});

test("revoke-all requires Origin and clears the session after Core accepts it", async () => {
  let receivedSession = "";
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (_url, init) => {
      receivedSession = new Headers(init?.headers).get("x-fluctlight-human-session") ?? "";
      return new Response(null, { status: 204 });
    },
  });
  const response = await app.inject({
    method: "POST",
    url: "/auth/revoke-all",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque", fluctlight_csrf: "csrf-token" },
  });
  assert.equal(response.statusCode, 204);
  assert.equal(receivedSession, "opaque");
  assert.match(String(response.headers["set-cookie"]), /fluctlight_session=;/);
  await app.close();
});

test("setup status reveals no token and password change clears all browser sessions", async () => {
  const requests: Array<{ path: string; session: string }> = [];
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (url, init) => {
      const path = new URL(typeof url === "string" ? url : url.toString()).pathname;
      requests.push({ path, session: new Headers(init?.headers).get("x-fluctlight-human-session") ?? "" });
      if (path === "/internal/auth/setup-status") return Response.json({ setup_available: true });
      return new Response(null, { status: 204 });
    },
  });
  const status = await app.inject({ method: "GET", url: "/auth/setup-status" });
  assert.deepEqual(status.json(), { setupAvailable: true });
  const password = await app.inject({
    method: "POST",
    url: "/auth/password",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque", fluctlight_csrf: "csrf-token" },
    payload: { password: "a-long-enough-password" },
  });
  assert.equal(password.statusCode, 204);
  assert.deepEqual(requests, [
    { path: "/internal/auth/setup-status", session: "" },
    { path: "/internal/auth/reset-password", session: "opaque" },
  ]);
  assert.match(String(password.headers["set-cookie"]), /fluctlight_session=;/);
  await app.close();
});
