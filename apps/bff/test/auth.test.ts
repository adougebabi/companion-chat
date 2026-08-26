import assert from "node:assert/strict";
import type { OutgoingHttpHeaders } from "node:http";
import test from "node:test";

import { createBff } from "../src/app.js";

function cookieValue(response: { headers: OutgoingHttpHeaders }, name: string): string {
  const raw = response.headers["set-cookie"];
  const values = Array.isArray(raw) ? raw : [raw ?? ""];
  return values
    .map((value) => value.match(new RegExp(`${name}=([^;]+)`))?.[1] ?? "")
    .find(Boolean) ?? "";
}

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

test("password change clears the session and issues CSRF for the required next login", async () => {
  const requests: Array<{ path: string; session: string }> = [];
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (url, init) => {
      const path = new URL(typeof url === "string" ? url : url.toString()).pathname;
      requests.push({ path, session: new Headers(init?.headers).get("x-fluctlight-human-session") ?? "" });
      if (path === "/internal/auth/setup-status") return Response.json({ setup_available: true });
      if (path === "/internal/auth/login") {
        return Response.json({ authenticated: true, actor_id: "human-owner", session_token: "next-session" });
      }
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
  const nextCsrf = cookieValue(password, "fluctlight_csrf");
  assert.notEqual(nextCsrf, "");
  const login = await app.inject({
    method: "POST",
    url: "/auth/login",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": nextCsrf },
    cookies: { fluctlight_csrf: nextCsrf },
    payload: { password: "123456" },
  });
  assert.equal(login.statusCode, 200);
  assert.deepEqual(requests, [
    { path: "/internal/auth/setup-status", session: "" },
    { path: "/internal/auth/reset-password", session: "opaque" },
    { path: "/internal/auth/login", session: "" },
  ]);
  assert.match(String(password.headers["set-cookie"]), /fluctlight_session=;/);
  await app.close();
});

test("password endpoints reject fewer than six characters before calling Core", async () => {
  let called = false;
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async () => {
      called = true;
      return new Response(null, { status: 204 });
    },
  });
  const response = await app.inject({
    method: "POST",
    url: "/auth/password",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque", fluctlight_csrf: "csrf-token" },
    payload: { password: "12345" },
  });
  assert.equal(response.statusCode, 400);
  assert.equal(called, false);
  await app.close();
});
