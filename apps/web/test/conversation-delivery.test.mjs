import assert from "node:assert/strict";
import test from "node:test";
	import { fileURLToPath } from "node:url";

import { createPinia, setActivePinia } from "pinia";
import { createServer } from "vite";

test("a new submit remains visible while an older durable retry is being processed", async () => {
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
    setActivePinia(createPinia());
    const { useConversationStore } = await server.ssrLoadModule("/src/stores/conversations.ts");
    const store = useConversationStore();
    store.conversation = { id: "conversation-1", createdByActorId: "owner", revision: 0, createdAt: "2026-09-11T00:00:00Z", updatedAt: "2026-09-11T00:00:00Z" };
    store.fluctlightId = "fluctlight-1";
    store.fluctlights = [{ id: "fluctlight-1", identity: { name: "摇光" }, status: "active" }];
    store.retryTurn = {
      conversationId: "conversation-1", fluctlightId: "fluctlight-1", text: "旧的待重试消息",
      idempotencyKey: "turn-old", turnId: "turn_old", attachmentRefs: [],
    };
    const sending = store.send("刚刚点击发送的新消息");
    await started;
	    assert.equal(store.sending, true);
	    assert.ok(store.messages.some((message) => message.kind === "user" && message.text === "刚刚点击发送的新消息"), "the submitted message disappeared while the retry cognition was running");
		assert.ok(values.has("fluctlight.queued-turn.v1"), "the accepted queued turn was not persisted for reload recovery");
    store.cancel();
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
    setActivePinia(createPinia());
    const { useConversationStore } = await server.ssrLoadModule("/src/stores/conversations.ts");
    const store = useConversationStore();
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

test("a successful older retry cannot overwrite or lose the separately identified queued user turn", async () => {
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
			const user = turnCount === 1 ? oldUser : newUser;
			const assistant = turnCount === 1 ? oldAssistant : newAssistant;
			const frames = [
				{ type: "message", turnId: body.turnId, sequence: 0, payload: { message: user } },
				{ type: "token", turnId: body.turnId, sequence: 1, payload: { text: assistant.text } },
				{ type: "message", turnId: body.turnId, sequence: 2, payload: { message: assistant } },
				{ type: "completed", turnId: body.turnId, sequence: 3, payload: { messageIds: [assistant.id] } },
			].map((frame) => JSON.stringify(frame)).join("\n") + "\n";
			return new Response(frames, { status: 200, headers: { "content-type": "application/x-ndjson" } });
		}
		if (url.includes("/messages")) {
			const messages = turnCount === 1 ? [oldUser, oldAssistant] : [oldUser, oldAssistant, newUser, newAssistant];
			return Response.json({ conversation, participants: [], messages, nextBeforeSequence: null });
		}
		if (url.includes("/read")) return new Response(null, { status: 204 });
		throw new Error(`unexpected request ${url}`);
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {
		setActivePinia(createPinia());
		const { useConversationStore } = await server.ssrLoadModule("/src/stores/conversations.ts");
		const store = useConversationStore();
		store.conversation = conversation;
		store.fluctlightId = "fluctlight-queue";
		store.fluctlights = [{ id: "fluctlight-queue", identity: { name: "摇光" }, status: "active" }];
		store.messages = [oldUser];
		store.retryTurn = { conversationId: conversation.id, fluctlightId: "fluctlight-queue", text: oldUser.text, idempotencyKey: "old-turn", turnId: "old_turn", attachmentRefs: [], messageId: oldUser.id };
		await store.send("继续");
		assert.deepEqual(submittedTexts, ["继续", "继续"]);
		assert.equal(store.retryTurn, null);
		assert.equal(store.queuedTurn, null);
		assert.equal(values.has("fluctlight.queued-turn.v1"), false);
		assert.deepEqual(store.messages.filter((message) => message.kind === "user").map((message) => message.id), [oldUser.id, newUser.id]);
		assert.deepEqual(store.messages.filter((message) => message.kind === "assistant").map((message) => message.id), [oldAssistant.id, newAssistant.id]);
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
	globalThis.fetch = async (input, init = {}) => {
		const url = String(input);
		if (!url.includes("/turn")) throw new Error(`unexpected request ${url}`);
		const body = JSON.parse(String(init.body));
		const frames = [
			{ type: "message", turnId: body.turnId, sequence: 0, payload: { message: user } },
			{ type: "error", turnId: body.turnId, sequence: 1, payload: { code: "conversation_settlement_failed", detail: "appraisal_required" } },
		].map((frame) => JSON.stringify(frame)).join("\n") + "\n";
		return new Response(frames, { status: 200, headers: { "content-type": "application/x-ndjson" } });
	};
	const server = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
	try {
		setActivePinia(createPinia());
		const { useConversationStore } = await server.ssrLoadModule("/src/stores/conversations.ts");
		const store = useConversationStore();
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
		setActivePinia(createPinia());
		const { useConversationStore } = await server.ssrLoadModule("/src/stores/conversations.ts");
		const store = useConversationStore();
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
			{ id: "foreign-user", conversationId: "conversation-foreign", sequence: 1, authorActorId: "owner", kind: "user", text: "旧请求", attachmentRefs: [], createdAt: "2026-09-11T00:00:01Z" },
			{ id: "foreign-assistant", conversationId: "conversation-foreign", sequence: 2, authorActorId: "foreign", kind: "assistant", text: "已完成", attachmentRefs: [], createdAt: "2026-09-11T00:00:02Z" },
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
		setActivePinia(createPinia());
		const { useConversationStore } = await server.ssrLoadModule("/src/stores/conversations.ts");
		const store = useConversationStore();
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
