import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createServer } from "vite";

test("reopening after transport loss renders the persisted worker failure and retry button", async () => {
  const original = { window: globalThis.window, fetch: globalThis.fetch, localStorage: globalThis.localStorage };
  const saved = new Map();
  globalThis.window = { location: { origin: "http://r4.test" } };
  globalThis.localStorage = {
    getItem: key => saved.get(key) ?? null,
    setItem: (key, value) => saved.set(key, String(value)),
    removeItem: key => saved.delete(key),
  };
  const conversation = { id: "r4-conversation", createdByActorId: "owner", revision: 0 };
  let submitted, status = "pending", posts = 0;
  globalThis.fetch = async (input, init = {}) => {
    const url = String(input);
    if (url.endsWith("/turn")) {
      posts++;
      submitted = JSON.parse(init.body);
      return new Response(new ReadableStream({ start(controller) {
        setTimeout(() => controller.error(new Error("connection lost")), 0);
      } }), { headers: { "content-type": "application/x-ndjson" } });
    }
    if (url.endsWith("/read")) return new Response(null, { status: 204 });
    if (!url.includes("/messages") && !url.endsWith("/conversation")) throw new Error(`unexpected request ${url}`);
    return Response.json({ conversation, participants: [], nextBeforeSequence: null, messages: [{
      id: "r4-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner",
      kind: "user", text: submitted.text, attachmentRefs: [], createdAt: "2026-11-02T12:43:19Z",
      turnId: submitted.turnId, idempotencyKey: submitted.idempotencyKey, turnStatus: status,
      turnErrorCode: status === "failed" ? "request_timeout" : undefined, turnRetryable: status === "failed",
    }] });
  };
  const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
  try {
    const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
    const first = createConversationController();
    first.conversation = conversation;
    first.fluctlightId = "r4-character";
    await first.send("请保留这轮回应的状态");
    assert.equal(first.hasPendingTurn, true);
    assert.equal(first.canRetry, false);
    status = "failed";
    const reopened = createConversationController();
    reopened.fluctlights = [{ id: "r4-character", identity: { name: "林雨桐" }, status: "active" }];
    await reopened.selectFluctlight("r4-character");
    assert.equal(reopened.messages.length, 1);
    assert.equal(reopened.messages[0].turnId, submitted.turnId);
    assert.equal(reopened.messages[0].turnStatus, "failed");
    assert.match(reopened.error, /request_timeout/);
    assert.equal(reopened.canRetry, true);
    assert.equal(posts, 1, "reconnect must only read the accepted turn");
    const { default: ChatTurnFeedback } = await server.ssrLoadModule("/src/react-app/components/ChatTurnFeedback.tsx");
    const html = renderToStaticMarkup(React.createElement(ChatTurnFeedback, { message: reopened.messages[0], messages: reopened.messages, store: reopened }));
    assert.match(html, /role="alert"/);
    assert.match(html, /模型服务响应超时。原消息已保留，可以稍后重试。/);
    assert.match(html, /<button[^>]*>[\s\S]*重试原回应[\s\S]*<\/button>/);
  } finally {
    await server.close();
    Object.assign(globalThis, original);
  }
});
