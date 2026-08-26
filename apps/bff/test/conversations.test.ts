import assert from "node:assert/strict";
import test from "node:test";

import { createBff } from "../src/app.js";

const page = {
  conversation: { id: "conversation-1", created_by_actor_id: "human-1", title: "Chat", revision: 0, created_at: "2026-08-25T00:00:00Z", updated_at: "2026-08-25T00:00:00Z" },
  participants: [{ conversation_id: "conversation-1", actor_id: "human-1", role: "owner", status: "active", joined_at: "2026-08-25T00:00:00Z", left_at: null }],
  messages: [],
  next_before_sequence: null,
};

test("conversation routes map Core fields and preserve the browser session boundary", async () => {
  const seen: string[] = [];
  const coreStream = `${JSON.stringify({ type: "token", turn_id: "turn-1", sequence: 0, payload: { text: "hello" } })}\n${JSON.stringify({ type: "completed", turn_id: "turn-1", sequence: 1, payload: {} })}\n`;
  const app = createBff({
    coreBaseUrl: "http://core.invalid",
    coreServiceKey: "internal",
    trustedOrigin: "https://fluctlight.local",
    fetcher: async (url, init) => {
      const requestUrl = typeof url === "string" ? new URL(url) : url instanceof URL ? url : new URL(url.url);
      seen.push(`${init?.method ?? "GET"} ${requestUrl.pathname}`);
      if (requestUrl.pathname.endsWith("/turn")) return new Response(coreStream, { headers: { "content-type": "application/x-ndjson" } });
      return Response.json(page);
    },
  });
  const created = await app.inject({
    method: "POST",
    url: "/api/conversations",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque-session", fluctlight_csrf: "csrf-token" },
    payload: { title: "Chat", participantActorIds: ["fl-1"] },
  });
  assert.equal(created.statusCode, 200);
  assert.equal(created.json().conversation.createdByActorId, "human-1");
  const stream = await app.inject({
    method: "POST",
    url: "/api/conversations/conversation-1/turn",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque-session", fluctlight_csrf: "csrf-token" },
    payload: { text: "hello", fluctlightId: "fl-1", idempotencyKey: "turn-1" },
  });
  assert.equal(stream.statusCode, 200);
  assert.match(stream.headers["content-type"] ?? "", /application\/x-ndjson/);
  assert.match(stream.body, /completed/);
  assert.ok(seen.some((value) => value.includes("/internal/conversations/conversation-1/turn")));
  const direct = await app.inject({
    method: "GET",
    url: "/api/fluctlights/fl-1/conversation",
    cookies: { fluctlight_session: "opaque-session" },
  });
  assert.equal(direct.statusCode, 200);
  assert.equal(direct.json().conversation.id, "conversation-1");
  assert.ok(seen.some((value) => value.includes("/internal/fluctlights/fl-1/conversation")));
  const preflight = await app.inject({
    method: "OPTIONS",
    url: "/api/conversations/conversation-1/turn",
    headers: {
      origin: "https://fluctlight.local",
      "access-control-request-headers": "content-type,x-csrf-token",
    },
  });
  assert.equal(preflight.statusCode, 204);
  assert.match(preflight.headers["access-control-allow-headers"] ?? "", /x-csrf-token/);
  const group = await app.inject({
    method: "POST",
    url: "/api/conversations",
    headers: { origin: "https://fluctlight.local", "x-csrf-token": "csrf-token" },
    cookies: { fluctlight_session: "opaque-session", fluctlight_csrf: "csrf-token" },
    payload: { participantActorIds: ["fl-1", "fl-2"] },
  });
  assert.equal(group.statusCode, 400);
  await app.close();
});
