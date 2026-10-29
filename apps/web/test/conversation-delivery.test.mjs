import assert from "node:assert/strict";
import test from "node:test";
	import { fileURLToPath } from "node:url";


import { createServer } from "vite";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

test("a new submit sends its own identity without implicitly retrying a confirmed failure", async () => {
  const originalWindow = globalThis.window;
  const originalFetch = globalThis.fetch;
  const originalLocalStorage = globalThis.localStorage;
  const values = new Map();
  globalThis.window = { location: { origin: "http://fluctlight.test" } };
  globalThis.localStorage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, String(value)),
    removeItem: (key) => values.delete(key),
  };
  let notifyStarted;
  const started = new Promise((resolve) => { notifyStarted = resolve; });
  globalThis.fetch = async (_input, init = {}) => {
    notifyStarted();
    return new Promise((_resolve, reject) => {
      init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
    });
  };
	  const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
  try {

    const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
    const store = createConversationController();
    store.conversation = { id: "conversation-1", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
    store.fluctlightId = "fluctlight-1";
    store.fluctlights = [{ id: "fluctlight-1", identity: { name: "摇光" }, status: "active" }];
    store.retryTurn = {
      conversationId: "conversation-1", fluctlightId: "fluctlight-1", text: "旧的待重试消息",
      idempotencyKey: "turn-old", turnId: "turn_old", attachmentRefs: [],
    };
    store.messages = [{id:"old-failed",kind:"user",text:"旧的待重试消息",turnId:"turn_old",turnStatus:"failed",turnRetryable:true}];
    const sending = store.send("刚刚点击发送的新消息");
    await started;
	    assert.equal(store.sending, true);
	    assert.ok(store.messages.some((message) => message.kind === "user" && message.text === "刚刚点击发送的新消息"), "the submitted message disappeared while the retry cognition was running");
		assert.ok(values.has("fluctlight.retry-turns.v3"), "the submitted turn was not persisted for reload recovery");
    store.invalidateRequest();
    await sending;
  } finally {
    await server.close();
    globalThis.window = originalWindow;
    globalThis.fetch = originalFetch;
    globalThis.localStorage = originalLocalStorage;
  }
});

test("a stale post-stream history read cannot erase committed streamed messages", async () => {
  const originalWindow = globalThis.window;
  const originalFetch = globalThis.fetch;
  const originalLocalStorage = globalThis.localStorage;
  globalThis.window = { location: { origin: "http://fluctlight.test" } };
  globalThis.localStorage = { getItem: () => null, setItem: () => {}, removeItem: () => {} };
  const conversation = { id: "conversation-2", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
  const user = { id: "message-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "请回复", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z" };
  const assistant = { id: "message-assistant", conversationId: conversation.id, sequence: 2, authorActorId: "fluctlight-2", kind: "assistant", text: "我在。", attachmentRefs: [], createdAt: "2026-09-11T00:00:02Z" };
  globalThis.fetch = async (input, init = {}) => {
    const url = String(input);
    if (url.includes("/turn")) {
	  const turnId = JSON.parse(String(init.body)).turnId;
      const frames = [
		{ type: "message", turnId, sequence: 0, payload: { message: user } },
		{ type: "token", turnId, sequence: 1, payload: { text: assistant.text } },
		{ type: "message", turnId, sequence: 2, payload: { message: assistant } },
		{ type: "completed", turnId, sequence: 3, payload: { messageIds: [assistant.id] } },
      ].map((frame) => JSON.stringify(frame)).join("\n") + "\n";
      return new Response(frames, { status: 200, headers: { "content-type": "application/x-ndjson" } });
    }
	    if (url.includes("/messages")) {
		  return Response.json({ conversation, participants: [], messages: [user, { ...assistant, text: "旧快照" }], nextBeforeSequence: null });
    }
    if (url.includes("/read")) return new Response(null, { status: 204 });
    throw new Error(`unexpected request ${url}`);
  };
	  const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
  try {

    const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
    const store = createConversationController();
    store.conversation = conversation;
    store.fluctlightId = "fluctlight-2";
    store.fluctlights = [{ id: "fluctlight-2", identity: { name: "摇光" }, status: "active" }];
	await store.send(user.text);
    assert.equal(store.error, "");
	    assert.ok(store.messages.some((message) => message.id === assistant.id && message.text === assistant.text), "the stale history response erased the committed assistant message");
  } finally {
    await server.close();
    globalThis.window = originalWindow;
    globalThis.fetch = originalFetch;
    globalThis.localStorage = originalLocalStorage;
  }
});

test("a new turn after a confirmed failure never replays the old request", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)), removeItem: (key) => values.delete(key) };
	const conversation = { id: "conversation-queue", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
	const oldUser = { id: "old-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "继续", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z" };
	const oldAssistant = { id: "old-assistant", conversationId: conversation.id, sequence: 2, authorActorId: "fluctlight-queue", kind: "assistant", text: "旧请求已完成。", attachmentRefs: [], createdAt: "2026-09-11T00:00:02Z" };
	const newUser = { id: "new-user", conversationId: conversation.id, sequence: 3, authorActorId: "owner", kind: "user", text: "继续", attachmentRefs: [], createdAt: "2026-09-11T00:00:03Z" };
	const newAssistant = { id: "new-assistant", conversationId: conversation.id, sequence: 4, authorActorId: "fluctlight-queue", kind: "assistant", text: "新请求也已完成。", attachmentRefs: [], createdAt: "2026-09-11T00:00:04Z" };
	let turnCount = 0;
	const submittedTexts = [];
	globalThis.fetch = async (input, init = {}) => {
		const url = String(input);
		if (url.includes("/turn")) {
			turnCount += 1;
			const body = JSON.parse(String(init.body));
			submittedTexts.push(body.text);
			const user = newUser;
			const assistant = newAssistant;
			const frames = [
				{ type: "message", turnId: body.turnId, sequence: 0, payload: { message: user } },
				{ type: "token", turnId: body.turnId, sequence: 1, payload: { text: assistant.text } },
				{ type: "message", turnId: body.turnId, sequence: 2, payload: { message: assistant } },
				{ type: "completed", turnId: body.turnId, sequence: 3, payload: { messageIds: [assistant.id] } },
			].map((frame) => JSON.stringify(frame)).join("\n") + "\n";
			return new Response(frames, { status: 200, headers: { "content-type": "application/x-ndjson" } });
		}
		if (url.includes("/messages")) {
			const messages = [oldUser, newUser, newAssistant];
			return Response.json({ conversation, participants: [], messages, nextBeforeSequence: null });
		}
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.conversation = conversation;
		store.fluctlightId = "fluctlight-queue";
		store.fluctlights = [{ id: "fluctlight-queue", identity: { name: "摇光" }, status: "active" }];
		store.messages = [{...oldUser,turnId:"old_turn",turnStatus:"failed",turnRetryable:true}];
		store.retryTurn = { conversationId: conversation.id, fluctlightId: "fluctlight-queue", text: oldUser.text, idempotencyKey: "old-turn", turnId: "old_turn", attachmentRefs: [], messageId: oldUser.id };
		await store.send("继续");
		assert.deepEqual(submittedTexts, ["继续"]);
        assert.equal(turnCount, 1);
		assert.equal(store.retryTurn, null);
		assert.equal(store.queuedTurn, null);
		assert.equal(values.has("fluctlight.queued-turn.v1"), false);
		assert.deepEqual(store.messages.filter((message) => message.kind === "user").map((message) => message.id), [oldUser.id, newUser.id]);
		assert.deepEqual(store.messages.filter((message) => message.kind === "assistant").map((message) => message.id), [newAssistant.id]);
		assert.equal(store.messages.some((message) => message.id.startsWith("local-") || message.id.startsWith("stream-")), false);
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("a terminal cognition error keeps the authoritative user message visible and retry-bound", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = {
		getItem: (key) => values.get(key) ?? null,
		setItem: (key, value) => values.set(key, String(value)),
		removeItem: (key) => values.delete(key),
	};
	const conversation = { id: "conversation-terminal-error", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
	const user = { id: "message-terminal-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "在吗？", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z" };
	let submitted;
	globalThis.fetch = async (input, init = {}) => {
		const url = String(input);
		if (url.includes("/messages")) return Response.json({ conversation, participants: [], messages: [{ ...user, turnId: submitted.turnId, idempotencyKey: submitted.idempotencyKey, turnStatus: "failed", turnErrorCode: "conversation_settlement_failed" }], nextBeforeSequence: null });
		if (url.includes("/read")) return new Response(null, { status: 204 });
		if (!url.includes("/turn")) throw new Error(`unexpected request ${url}`);
		const body = JSON.parse(String(init.body));
		submitted = body;
		const frames = [
			{ type: "message", turnId: body.turnId, sequence: 0, payload: { message: user } },
			// Core's current shape carries the stable code; the API boundary still accepts
			// the legacy payload.error form during rolling upgrades.
			{ type: "error", turnId: body.turnId, sequence: 1, payload: { status: "failed", code: "conversation_settlement_failed" } },
		].map((frame) => JSON.stringify(frame)).join("\n") + "\n";
		return new Response(frames, { status: 200, headers: { "content-type": "application/x-ndjson" } });
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.conversation = conversation;
		store.fluctlightId = "fluctlight-terminal-error";
		store.fluctlights = [{ id: "fluctlight-terminal-error", identity: { name: "摇光" }, status: "active" }];
		await store.send(user.text);
		assert.ok(store.messages.some((message) => message.id === user.id && message.kind === "user" && message.text === user.text), "terminal error erased the committed user message");
		assert.equal(store.retryTurn?.messageId, user.id);
		assert.match(store.error, /conversation_settlement_failed/);
		assert.ok(values.has("fluctlight.retry-turn.v2"), "terminal error did not preserve durable browser retry identity");
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("a non-2xx turn response keeps the API error code instead of collapsing to turn_failed", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = {
		getItem: (key) => values.get(key) ?? null,
		setItem: (key, value) => values.set(key, String(value)),
		removeItem: (key) => values.delete(key),
	};
	const conversation = { id: "conversation-http-error", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
	globalThis.fetch = async (input) => {
		const url = String(input);
		if (url.includes("/turn")) {
			return Response.json({ detail: { code: "conversation_turn_conflict", message: "The conversation turn failed", details: { private: "must not surface" } } }, { status: 502 });
		}
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.conversation = conversation;
		store.fluctlightId = "fluctlight-http-error";
		store.fluctlights = [{ id: "fluctlight-http-error", identity: { name: "摇光" }, status: "active" }];
		await store.send("在吗？");
		assert.match(store.error, /conversation_turn_conflict/);
		assert.doesNotMatch(store.error, /turn_failed/);
		assert.equal(values.has("fluctlight.retry-turn.v2"), false, "a rejected turn must not become a model retry");
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("a retry from a discarded conversation is pruned when the server returns a new conversation", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = { getItem: () => null, setItem: () => {}, removeItem: () => {} };
	const page = {
		conversation: { id: "conversation-new", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" },
		participants: [], messages: [], nextBeforeSequence: null,
	};
	globalThis.fetch = async (input) => {
		const url = String(input);
		if (url.includes("/conversation")) return Response.json(page);
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.fluctlights = [{ id: "fluctlight-reset", identity: { name: "摇光" }, status: "active" }];
		store.retryTurn = { conversationId: "conversation-deleted", fluctlightId: "fluctlight-reset", text: "旧消息", idempotencyKey: "old", turnId: "old", attachmentRefs: [], messageId: "old-user" };
		await store.selectFluctlight("fluctlight-reset", { loading: false });
		assert.equal(store.retryTurn, null);
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("a completed retry from another conversation no longer blocks the selected conversation", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)), removeItem: (key) => values.delete(key) };
	const currentPage = {
		conversation: { id: "conversation-current", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" },
		participants: [], messages: [], nextBeforeSequence: null,
	};
	const completedForeignPage = {
		conversation: { id: "conversation-foreign", createdByActorId: "owner", revision: 2, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:02Z" },
		participants: [],
		messages: [
			{ id: "foreign-user", conversationId: "conversation-foreign", sequence: 1, authorActorId: "owner", kind: "user", text: "旧请求", turnId: "old", idempotencyKey: "old", turnStatus: "completed", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z" },
			{ id: "foreign-assistant", conversationId: "conversation-foreign", sequence: 2, authorActorId: "foreign", kind: "assistant", text: "已完成", turnId: "old", attachmentRefs: [], createdAt: "2026-09-11T00:00:02Z" },
		],
		nextBeforeSequence: null,
	};
	globalThis.fetch = async (input, init = {}) => {
		const url = String(input);
		if (url.endsWith("/api/fluctlights/fluctlight-current/conversation")) return Response.json(currentPage);
		if (url.endsWith("/api/fluctlights/fluctlight-foreign/conversation")) return Response.json(completedForeignPage);
		if (url.includes("/turn")) {
			const body = JSON.parse(String(init.body));
			const user = { id: "current-user", conversationId: currentPage.conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: body.text, attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z" };
			const assistant = { id: "current-assistant", conversationId: currentPage.conversation.id, sequence: 2, authorActorId: "fluctlight-current", kind: "assistant", text: "收到", attachmentRefs: [], createdAt: "2026-09-11T00:00:02Z" };
			const frames = [
				{ type: "message", turnId: body.turnId, sequence: 0, payload: { message: user } },
				{ type: "message", turnId: body.turnId, sequence: 1, payload: { message: assistant } },
				{ type: "completed", turnId: body.turnId, sequence: 2, payload: { messageIds: [assistant.id] } },
			].map((frame) => JSON.stringify(frame)).join("\n") + "\n";
			return new Response(frames, { status: 200, headers: { "content-type": "application/x-ndjson" } });
		}
		if (url.includes("/messages")) return Response.json(currentPage);
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.conversation = currentPage.conversation;
		store.fluctlightId = "fluctlight-current";
		store.fluctlights = [
			{ id: "fluctlight-current", identity: { name: "当前" }, status: "active" },
			{ id: "fluctlight-foreign", identity: { name: "旧会话" }, status: "active" },
		];
		store.retryTurn = { conversationId: "conversation-foreign", fluctlightId: "fluctlight-foreign", text: "旧请求", idempotencyKey: "old", turnId: "old", attachmentRefs: [], messageId: "foreign-user" };
		await store.send("新请求");
		assert.equal(store.error, "");
		assert.equal(store.retryTurn, null);
		assert.ok(store.messages.some((message) => message.id === "current-assistant"));
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("reconcilePersistedRetry matches durable turn identity and clears completed turns without duplicates", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = {
		getItem: (key) => values.get(key) ?? null,
		setItem: (key, value) => values.set(key, String(value)),
		removeItem: (key) => values.delete(key),
	};
	const conversation = { id: "conversation-reconcile", createdByActorId: "owner", revision: 2, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:02Z" };
	const persistedUser = { id: "message-real-user-123", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "你好摇光", turnId: "turn_test", idempotencyKey: "turn-test", turnStatus: "completed", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z" };
	const persistedAssistant = { id: "message-real-assistant-456", conversationId: conversation.id, sequence: 2, authorActorId: "fluctlight-reconcile", kind: "assistant", text: "你好呀！", turnId: "turn_test", attachmentRefs: [], createdAt: "2026-09-11T00:00:02Z" };
	const page = {
		conversation,
		participants: [],
		messages: [persistedUser, persistedAssistant],
		nextBeforeSequence: null,
	};
	globalThis.fetch = async (input) => {
		const url = String(input);
		if (url.endsWith("/api/fluctlights/fluctlight-reconcile/conversation")) return Response.json(page);
		if (url.includes("/messages")) return Response.json(page);
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.fluctlights = [{ id: "fluctlight-reconcile", identity: { name: "摇光" }, status: "active" }];
		// Simulate a retry saved with local- transient ID when user refreshed during request
		const transientRetry = {
			conversationId: conversation.id,
			fluctlightId: "fluctlight-reconcile",
			text: "你好摇光",
			idempotencyKey: "turn-test",
			turnId: "turn_test",
			attachmentRefs: [],
			messageId: "local-transient-999",
		};
		store.retryTurn = transientRetry;
		values.set("fluctlight.retry-turn.v2", JSON.stringify(transientRetry));

		await store.reconcilePersistedRetry();
		assert.equal(store.retryTurn, null, "retryTurn should be cleared because assistant already replied in DB");
		assert.equal(values.has("fluctlight.retry-turn.v2"), false, "localStorage retry should be cleared");

		await store.selectFluctlight("fluctlight-reconcile");
		// Ensure messages contains only the 2 persisted messages from DB, no duplicate local message
		assert.equal(store.messages.length, 2);
		assert.equal(store.messages.some((m) => m.id.startsWith("local-")), false, "no duplicate transient local message should be pushed");
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("refresh restores a pending server turn and later shows its committed reply", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)), removeItem: (key) => values.delete(key) };
	const conversation = { id: "conversation-background", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
	const user = { id: "background-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "后台处理", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z", turnId: "background-turn", idempotencyKey: "background-key", turnStatus: "pending" };
	const assistant = { id: "background-assistant", conversationId: conversation.id, sequence: 2, authorActorId: "fluctlight-background", kind: "assistant", text: "处理完成", attachmentRefs: [], createdAt: "2026-09-11T00:00:02Z", turnId: "background-turn" };
	let completed = false;
	globalThis.fetch = async (input) => {
		const url = String(input);
		if (url.includes("/conversation") || url.includes("/messages")) return Response.json({ conversation, participants: [], messages: completed ? [{ ...user, turnStatus: "completed" }, assistant] : [user], nextBeforeSequence: null });
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.fluctlights = [{ id: "fluctlight-background", identity: { name: "摇光" }, status: "active" }];
		store.retryTurn = { conversationId: conversation.id, fluctlightId: "fluctlight-background", text: user.text, turnId: user.turnId, idempotencyKey: user.idempotencyKey, attachmentRefs: [], messageId: user.id };
		await store.selectFluctlight("fluctlight-background");
		assert.equal(store.hasPendingTurn, true);
		assert.equal(store.canRetry, false);
		assert.equal(store.error, "");
		completed = true;
		await store.refreshActiveTurn();
		assert.equal(store.hasPendingTurn, false);
		assert.ok(store.messages.some((message) => message.id === assistant.id));
		assert.equal(store.messages.find((message) => message.id === user.id)?.turnStatus, "completed");
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("explicit cancel calls the server and restores a retryable cancelled state", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)), removeItem: (key) => values.delete(key) };
	const conversation = { id: "conversation-explicit-cancel", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
	const user = { id: "cancel-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "停止", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z", turnId: "cancel-turn", idempotencyKey: "cancel-key", turnStatus: "pending" };
	let cancelCount = 0;
	let settled = false;
	globalThis.fetch = async (input) => {
		const url = String(input);
		if (url.endsWith("/turn/cancel-turn/cancel")) { cancelCount += 1; return Response.json({}); }
		if (url.includes("/messages")) return Response.json({ conversation, participants: [], messages: [{ ...user, turnStatus: "cancelled", turnErrorCode: "user_cancelled", turnRetryable: settled }], nextBeforeSequence: null });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.conversation = conversation;
		store.fluctlightId = "fluctlight-cancel";
		store.messages = [user];
		await store.cancel();
		assert.equal(cancelCount, 1);
		assert.equal(store.messages[0].turnStatus, "cancelled");
		assert.equal(store.canRetry, false);
		assert.equal(store.hasSettlingTurn, true);
		settled = true;
		await store.refreshActiveTurn();
		assert.equal(store.canRetry, true);
		assert.match(store.error, /已取消/);
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("transport loss after server acceptance keeps background cognition pending", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)), removeItem: (key) => values.delete(key) };
	const conversation = { id: "conversation-transport-loss", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
	let submitted;
	globalThis.fetch = async (input, init = {}) => {
		const url = String(input);
		if (url.includes("/turn")) {
			submitted = JSON.parse(String(init.body));
			const body = new ReadableStream({ start(controller) { setTimeout(() => controller.error(new Error("transport disconnected")), 0); } });
			return new Response(body, { status: 200, headers: { "content-type": "application/x-ndjson" } });
		}
		if (url.includes("/messages")) return Response.json({ conversation, participants: [], messages: [{ id: "transport-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: submitted.text, attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z", turnId: submitted.turnId, idempotencyKey: submitted.idempotencyKey, turnStatus: "pending" }], nextBeforeSequence: null });
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.conversation = conversation;
		store.fluctlightId = "fluctlight-transport-loss";
		await store.send("仍在处理");
		assert.equal(store.hasPendingTurn, true);
		assert.equal(store.canRetry, false);
		assert.equal(store.error, "");
		assert.equal(store.messages.filter((message) => message.kind === "user").length, 1);
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});

test("sending a new message when previous turn is stuck pending auto-cancels and sends", async () => {
	const originalWindow = globalThis.window;
	const originalFetch = globalThis.fetch;
	const originalLocalStorage = globalThis.localStorage;
	const storageValues = new Map();
	globalThis.window = { addEventListener() {}, removeEventListener() {}, document: { visibilityState: "visible" }, location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = {
		getItem: (key) => storageValues.get(key) ?? null,
		setItem: (key, value) => storageValues.set(key, String(value)),
		removeItem: (key) => storageValues.delete(key),
	};
	const conversation = { id: "conversation-stuck-pending", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
	let cancelled = false;
	let newSent = false;
	globalThis.fetch = async (input, init = {}) => {
		const url = String(input);
		if (url.includes("/cancel")) {
			cancelled = true;
			return Response.json({status:"cancelled"});
		}
		if (url.includes("/turn")) {
			newSent = true;
			const body = new ReadableStream({ start(controller) { controller.close(); } });
			return new Response(body, { status: 200, headers: { "content-type": "application/x-ndjson" } });
		}
		if (url.includes("/messages")) {
			return Response.json({
				conversation,
				participants: [],
				messages: [
					{ id: "stuck-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "卡住的消息", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z", turnId: "turn_stuck", idempotencyKey: "idem_stuck", turnStatus: cancelled ? "cancelled" : "pending" },
				],
				nextBeforeSequence: null,
			});
		}
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {

		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.conversation = conversation;
		store.fluctlightId = "fluctlight-stuck";
		store.messages = [
			{ id: "stuck-user", conversationId: conversation.id, sequence: 1, authorActorId: "owner", kind: "user", text: "卡住的消息", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z", turnId: "turn_stuck", idempotencyKey: "idem_stuck", turnStatus: "pending" },
		];
		assert.equal(store.hasPendingTurn, true);
		await store.send("新发送的消息");
		assert.equal(cancelled, true);
		assert.equal(newSent, true);
	} finally {
		await server.close();
		globalThis.window = originalWindow;
		globalThis.fetch = originalFetch;
		globalThis.localStorage = originalLocalStorage;
	}
});


async function withRetryFixture(clock, run) {
	const originals = { window: globalThis.window, fetch: globalThis.fetch, localStorage: globalThis.localStorage };
	const values = new Map();
	globalThis.window = { location: { origin: "http://fluctlight.test" } };
	globalThis.localStorage = { getItem: (key) => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)), removeItem: (key) => values.delete(key) };
	const conversation = { id: "conversation-retry-clock", createdByActorId: "owner", revision: 1, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:02Z" };
	const user = { id: "persisted-retry-user", conversationId: conversation.id, sequence: 1, authorActorId: "participant", kind: "user", text: "原始消息", attachmentRefs: ["asset-original"], createdAt: "2026-09-11T00:00:01Z", turnId: "original-turn", idempotencyKey: "original-key", turnStatus: "failed", turnErrorCode: "conversation_native_output_missing", turnRetryable: true, ...clock };
	const page = { conversation, participants: [], messages: [user], nextBeforeSequence: null };
	const requests = [];
 let cancelResponse=async()=>Response.json({status:"cancelled"});
	let turnResponse = async () => {
		user.turnStatus = "completed";
		return new Response(JSON.stringify({ type: "completed", turnId: user.turnId, sequence: 0, payload: {} }) + "\n", { headers: { "content-type": "application/x-ndjson" } });
	};
	globalThis.fetch = async (input, init = {}) => {
		const url = String(input);
 if(url.endsWith("/cancel"))return cancelResponse();
		if (url.endsWith("/turn")) {
			requests.push(JSON.parse(String(init.body)));
			return turnResponse();
		}
		if (url.includes("/conversation") || url.includes("/messages")) return Response.json(page);
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {


		const { createConversationController } = await server.ssrLoadModule("/src/state/conversation.ts");
		const store = createConversationController();
		store.fluctlights = [{ id: "fluctlight-retry-clock", identity: { name: "摇光" }, status: "active" }];
		await store.selectFluctlight("fluctlight-retry-clock");
		await run({ store, server,  user, requests, values, setCancelResponse:response=>{cancelResponse=response;}, setTurnResponse: (response) => { turnResponse = response; } });
	} finally {
		await server.close();
		Object.assign(globalThis, originals);
	}
}

test('an unconfirmed cancel never invents a cancelled server turn',async()=>{
 await withRetryFixture({},async({store,user,setCancelResponse})=>{
  setCancelResponse(async()=>Response.json({error:{code:'unavailable'}},{status:503}));
  store.messages=[{...user,turnStatus:'running'}];
  await store.cancel();
  assert.equal(store.messages[0].turnStatus,'running');assert.equal(store.hasPendingTurn,true);assert.match(store.error,/尚未确认/);
 });
});

test('one follow-up stays visible and sends once after the running reply completes',async()=>{
 await withRetryFixture({},async({store,user,requests,values,setTurnResponse})=>{
  store.dismissRetry();user.turnStatus='completed';store.messages=[{...user}];
  let release,entered;const started=new Promise(resolve=>entered=resolve),response=new Promise(resolve=>release=resolve);
  setTurnResponse(async()=>{
   const request=requests.at(-1);if(requests.length===1){entered();await response;}
   Object.assign(user,{id:'accepted-'+requests.length,turnId:request.turnId,idempotencyKey:request.idempotencyKey,text:request.text,turnStatus:'completed'});
   return new Response(JSON.stringify({type:'completed',turnId:request.turnId,sequence:0,payload:{}})+'\n',{headers:{'content-type':'application/x-ndjson'}});
  });
  const first=store.send('第一条');await started;await store.send('再补充一句');
  assert.equal(requests.length,1);assert.equal(store.queuedTurn.text,'再补充一句');assert.ok(store.messages.some(m=>m.text==='再补充一句'));assert.ok(values.has('fluctlight.queued-turn.v1'));
  const key=store.queuedTurn.idempotencyKey;release();await first;
  assert.equal(requests.length,2);assert.equal(requests[1].text,'再补充一句');assert.equal(requests[1].idempotencyKey,key);assert.equal(store.queuedTurn,null);
 });
});

test('a failed original reply cannot replace an existing follow-up with a third submit',async()=>{
 await withRetryFixture({},async({store,requests,values})=>{
  store.sending=true;await store.send('保留的追发');store.sending=false;
  const original=JSON.parse(JSON.stringify(store.queuedTurn)),persisted=values.get('fluctlight.queued-turn.v1');
  await store.send('第三条尚未发送');
  assert.deepEqual(JSON.parse(JSON.stringify(store.queuedTurn)),original);assert.equal(values.get('fluctlight.queued-turn.v1'),persisted);assert.equal(requests.length,0);
 });
});

test('targeted status inspection never sends an already queued follow-up',async()=>{
 await withRetryFixture({},async({store,user,requests,values})=>{
  store.dismissRetry();user.turnStatus='completed';store.messages=[{...user}];
  store.sending=true;await store.send('保留的排队消息');store.sending=false;const queued=JSON.parse(JSON.stringify(store.queuedTurn)),persisted=values.get('fluctlight.queued-turn.v1');
  assert.equal(await store.refreshActiveTurn(true,user.turnId),'completed');
  assert.equal(requests.length,0);assert.deepEqual(JSON.parse(JSON.stringify(store.queuedTurn)),queued);assert.equal(values.get('fluctlight.queued-turn.v1'),persisted);
 });
});
test('explicit refresh confirming completion sends the exact queued follow-up before the old observer closes',async()=>{
 await withRetryFixture({},async({store,user,requests,setTurnResponse})=>{
  store.dismissRetry();user.turnStatus='completed';store.messages=[{...user}];
  let release,entered;const started=new Promise(resolve=>entered=resolve),blocked=new Promise(resolve=>release=resolve);
  setTurnResponse(async()=>{const request=requests.at(-1);if(requests.length===1){entered();await blocked;}Object.assign(user,{id:'accepted-'+requests.length,turnId:request.turnId,idempotencyKey:request.idempotencyKey,text:request.text,turnStatus:'completed'});return new Response(JSON.stringify({type:'completed',turnId:request.turnId,sequence:0,payload:{}})+'\n',{headers:{'content-type':'application/x-ndjson'}});});
  const original=store.send('等待读回');await started;await store.send('原追发');const key=store.queuedTurn.idempotencyKey;
  Object.assign(user,{id:'accepted-a',turnId:requests[0].turnId,idempotencyKey:requests[0].idempotencyKey,turnStatus:'completed'});
  await store.refreshActiveTurn(true);assert.equal(requests.length,2);assert.equal(requests[1].text,'原追发');assert.equal(requests[1].idempotencyKey,key);assert.equal(store.queuedTurn,null);
  release();await original;
 });
});

for (const [name, clock] of [
	["frozen sender clock", { senderTimezone: "Asia/Shanghai", senderUtcOffsetMinutes: 480, senderSentAt: "2026-09-11T00:00:00.123Z" }],
	["zero UTC offset", { senderTimezone: "UTC", senderUtcOffsetMinutes: 0, senderSentAt: "2026-09-11T00:00:00.123Z" }],
	["unknown legacy clock", {}],
	["null legacy clock", { senderTimezone: null, senderUtcOffsetMinutes: null, senderSentAt: null }],
]) {
	test(`server-restored retry preserves ${name} and the original durable identity`, async () => {
		await withRetryFixture(clock, async ({ store, user, requests, values }) => {
			assert.equal(store.canRetry, true);
			for (const field of ["senderTimezone", "senderUtcOffsetMinutes", "senderSentAt"]) {
				assert.equal(store.retryTurn[field], user[field] ?? undefined);
				assert.equal(JSON.parse(values.get("fluctlight.retry-turn.v2"))[field], user[field] ?? undefined);
			}
			await store.retry();
			assert.deepEqual(requests, [{ text: user.text, fluctlightId: "fluctlight-retry-clock", senderActorId: user.authorActorId, attachmentRefs: user.attachmentRefs, idempotencyKey: user.idempotencyKey, turnId: user.turnId, ...Object.fromEntries(Object.entries(clock).filter(([, value]) => value != null)) }]);
			assert.equal(store.messages.filter((message) => message.kind === "user").length, 1);
			assert.equal(store.messages[0].id, user.id);
			assert.equal(store.retryTurn, null);
		});
	});
}

test("retry stays visibly disabled in flight and a 409 keeps identity, draft and useful failure", async () => {
	await withRetryFixture({ senderTimezone: "UTC", senderUtcOffsetMinutes: 0, senderSentAt: "2026-09-11T00:00:00.123Z" }, async ({ store, server,  user, requests, setTurnResponse }) => {
		let releaseResponse;
		setTurnResponse(() => new Promise((resolve) => { releaseResponse = resolve; }));
		store.attachmentRef = "asset-unsent-draft";
		const retry = store.retry();
		try {
			assert.equal(store.retrying, true);
			assert.equal(store.canRetry, false);
			assert.equal(store.hasPendingTurn, false, "a local request must not fabricate server acceptance");
			const { default: ChatTurnFeedback } = await server.ssrLoadModule("/src/react-app/components/ChatTurnFeedback.tsx");
			const html = renderToStaticMarkup(React.createElement(ChatTurnFeedback,{message:store.messages.find(message=>message.kind==="user"),messages:store.messages,store}));
			assert.match(html, /正在提交重试请求并等待回复/);
			assert.match(html, /<button[^>]*disabled[^>]*>[\s\S]*重试中…[\s\S]*<\/button>/);
		} finally {
			releaseResponse(Response.json({ code: "conversation_turn_conflict", message: "Message identity conflict" }, { status: 409 }));
			await retry;
		}
		assert.equal(requests.length, 1);
		assert.equal(store.retrying, false);
		assert.equal(store.canRetry, true);
		assert.equal(store.retryTurn.turnId, user.turnId);
		assert.equal(store.retryTurn.idempotencyKey, user.idempotencyKey);
		assert.equal(store.retryTurn.messageId, user.id);
		assert.equal(store.attachmentRef, "asset-unsent-draft");
		assert.match(store.error, /重试请求未被接受：conversation_turn_conflict/);
		assert.match(store.error, /conversation_native_output_missing/);
		assert.equal(store.messages.filter((message) => message.kind === "user").length, 1);
	});
});

test("restored local retry and queue bubbles preserve attachments, actor, clock and turn identity", async () => {
	await withRetryFixture({}, async ({ store }) => {
		const original = { ...store.retryTurn, senderActorId: "participant", senderTimezone: "UTC", senderUtcOffsetMinutes: 0, senderSentAt: "2026-09-11T00:00:00.123Z" };
		store.messages = [];
		store.retryTurn = original;
		store.ensureRetryMessageVisible();
		store.queuedTurn = { ...original, messageId: "local-queued-original", turnId: "queued-turn", idempotencyKey: "queued-key", createdAt: "2026-09-11T00:00:04Z" };
		store.ensureQueuedMessageVisible();
		for (const [message, request] of [[store.messages[0], original], [store.messages[1], store.queuedTurn]]) {
			assert.equal(message.id, request.messageId);
			assert.equal(message.authorActorId, request.senderActorId);
			for (const field of ["attachmentRefs", "turnId", "idempotencyKey", "senderTimezone", "senderUtcOffsetMinutes", "senderSentAt"]) assert.deepEqual(message[field], request[field]);
		}
		assert.equal(store.messages[0].createdAt, original.senderSentAt);
		assert.equal(store.messages[1].createdAt, store.queuedTurn.createdAt);
	});
});

test("a partial committed failure stays non-retryable and does not claim it will settle into retry", async () => {
	await withRetryFixture({}, async ({ store, user, server,  requests }) => {
		store.messages = [{ ...user, turnErrorCode: "agent_partial_committed", turnRetryable: false }];
		store.syncServerTurnState();
		assert.equal(store.canRetry, false);
		assert.equal(store.hasSettlingTurn, false);
		assert.match(store.error, /已执行部分操作，不能直接重试/);
		assert.doesNotMatch(store.error, /完成后可以重试/);
		await store.retry();
		assert.equal(requests.length, 0);
		const { default: ChatTurnFeedback } = await server.ssrLoadModule("/src/react-app/components/ChatTurnFeedback.tsx");
		const html = renderToStaticMarkup(React.createElement(ChatTurnFeedback,{message:store.messages.find(message=>message.kind==="user"),messages:store.messages,store}));
		assert.match(html, /部分操作已执行，不能直接重试/);
        assert.doesNotMatch(html, /重试原回应/);
		assert.doesNotMatch(html, /本次生成正在结束|重试回复/);
	});
});
