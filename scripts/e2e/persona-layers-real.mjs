import assert from "node:assert/strict";

const baseUrl = process.env.FLUCTLIGHT_E2E_BASE_URL ?? "http://127.0.0.1:13000";
const origin = process.env.FLUCTLIGHT_E2E_ORIGIN ?? "http://127.0.0.1:13001";
const password = process.env.FLUCTLIGHT_E2E_PASSWORD;
const timeoutMs = Number(process.env.FLUCTLIGHT_E2E_TIMEOUT_MS ?? 600000);

if (!password) {
  throw new Error("FLUCTLIGHT_E2E_PASSWORD is required; this test performs a real login and never mocks authentication/provider calls.");
}

const cookieJar = new Map();

function rememberCookies(response) {
  const values = typeof response.headers.getSetCookie === "function"
    ? response.headers.getSetCookie()
    : (response.headers.get("set-cookie") ?? "").split(/,(?=\s*[^;,=]+=[^;,]+)/g);
  for (const value of values) {
    const first = value.split(";", 1)[0];
    const separator = first.indexOf("=");
    if (separator > 0) cookieJar.set(first.slice(0, separator), first.slice(separator + 1));
  }
}

function cookieHeader() {
  return [...cookieJar.entries()].map(([key, value]) => `${key}=${value}`).join("; ");
}

function csrfToken() {
  return cookieJar.get("fluctlight_csrf") ?? "";
}

async function request(path, options = {}) {
  const headers = new Headers(options.headers ?? {});
  headers.set("Origin", origin);
  const cookies = cookieHeader();
  if (cookies) headers.set("Cookie", cookies);
  if (options.body !== undefined && !headers.has("content-type")) headers.set("content-type", "application/json");
  if (options.method && options.method !== "GET" && options.method !== "HEAD") {
    headers.set("X-CSRF-Token", csrfToken());
  }
  const response = await fetch(new URL(path, baseUrl), { ...options, headers, signal: options.signal ?? AbortSignal.timeout(timeoutMs) });
  rememberCookies(response);
  return response;
}

async function jsonRequest(path, options = {}) {
  const response = await request(path, options);
  const text = await response.text();
  let payload;
  try { payload = text ? JSON.parse(text) : null; } catch { payload = { raw: text }; }
  assert.equal(response.ok, true, `${options.method ?? "GET"} ${path} failed (${response.status}): ${JSON.stringify(payload).slice(0, 2000)}`);
  return payload;
}

function isObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function assertNonEmptyObject(value, label) {
  assert.equal(isObject(value), true, `${label} must be an object`);
  assert.ok(Object.keys(value).length > 0, `${label} must not be empty`);
}

async function readNDJSON(response) {
  assert.equal(response.ok, true, `chat request failed (${response.status})`);
  assert.match(response.headers.get("content-type") ?? "", /application\/x-ndjson/i);
  const reader = response.body?.getReader();
  assert.ok(reader, "chat response has no readable body");
  const decoder = new TextDecoder();
  const events = [];
  let buffer = "";
  for (;;) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done });
    const lines = buffer.split("\n");
    buffer = lines.pop() ?? "";
    for (const line of lines) {
      if (!line.trim()) continue;
      events.push(JSON.parse(line));
    }
    if (done) break;
  }
  if (buffer.trim()) events.push(JSON.parse(buffer));
  return events;
}

async function waitForSchedule(fluctlightId) {
  const deadline = Date.now() + timeoutMs;
  let lastDetail;
  while (Date.now() < deadline) {
    lastDetail = await jsonRequest(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/detail`);
    const schedule = lastDetail?.schedule;
    if (isObject(schedule) && Array.isArray(schedule.items) && schedule.items.length > 0) return lastDetail;
    await new Promise((resolve) => setTimeout(resolve, 5000));
  }
  throw new Error(`schedule was not generated before timeout: ${JSON.stringify(lastDetail?.schedule ?? null).slice(0, 2000)}`);
}

const report = { baseUrl, cases: {} };

// Authenticate against the real BFF and obtain the CSRF cookie used by all
// subsequent mutations. No provider, database, or HTTP layer is mocked.
await jsonRequest("/auth/setup-status");
await jsonRequest("/auth/login", { method: "POST", body: JSON.stringify({ password }) });
assert.equal((await jsonRequest("/auth/session")).authenticated, true);

// Case 1: natural-language description -> real initialization provider ->
// preview payload -> explicit activation.
const analysis = await jsonRequest("/api/fluctlight-creations/analysis", {
  method: "POST",
  body: JSON.stringify({
    description: "创建一个高冷成熟理性的女性摇光。她独立、主动、有判断力，不刻意讨好。她是一名上海研究生，生活中会学习、阅读、散步，喜欢安静环境但这只是待观察的偏好。请生成合理的生活设定、目标和今天的日程。",
  }),
});
assertNonEmptyObject(analysis?.core_persona, "analysis.core_persona");
for (const key of ["identity", "personality", "behavioral_policy", "life_profile"]) assertNonEmptyObject(analysis.core_persona[key], `analysis.core_persona.${key}`);
assertNonEmptyObject(analysis?.developing_self, "analysis.developing_self");
assert.ok(Array.isArray(analysis.developing_self.claims), "analysis.developing_self.claims must be an array");
assert.equal("foundation" in analysis, false, "analysis must not use the legacy foundation envelope");
report.cases.create = { status: "passed", coreKeys: Object.keys(analysis.core_persona), developingSelfClaims: analysis.developing_self.claims.length };

const requestId = `persona-layer-e2e-${Date.now()}-${Math.random().toString(36).slice(2)}`;
let created;
try {
  created = await jsonRequest("/api/fluctlight-creations/activate", {
    method: "POST",
    body: JSON.stringify({
      requestId,
      initializationMode: "llm_defined",
      corePersona: analysis.core_persona,
      developingSelf: analysis.developing_self,
      initialGoals: Array.isArray(analysis.initial_goals) ? analysis.initial_goals : [],
      initialIntentions: Array.isArray(analysis.initial_intentions) ? analysis.initial_intentions : [],
    }),
  });
} catch (error) {
  console.error("activation rejected; analysis payload:", JSON.stringify(analysis, null, 2));
  throw error;
}
assert.ok(typeof created?.id === "string" && created.id.length > 0, "activation did not return a Fluctlight id");
const fluctlightId = created.id;
report.cases.activation = { status: "passed", fluctlightId };

// Case 2: wait for the real schedule workflow, then verify the detail read
// model contains all three layers and the important current-day sections.
const detail = await waitForSchedule(fluctlightId);
assertNonEmptyObject(detail.core_persona, "detail.core_persona");
for (const key of ["identity", "personality", "behavioral_policy", "life_profile"]) assertNonEmptyObject(detail.core_persona[key], `detail.core_persona.${key}`);
assertNonEmptyObject(detail.developing_self, "detail.developing_self");
assert.ok(Array.isArray(detail.developing_self.claims), "detail.developing_self.claims must be an array");
assertNonEmptyObject(detail.current_state, "detail.current_state");
const innerState = detail.current_state.inner_state ?? detail.inner_state;
assertNonEmptyObject(innerState, "detail.current_state.inner_state");
for (const key of ["pad", "mood", "momentum", "regulation", "conflicts"]) assert.ok(key in innerState, `detail current state missing ${key}`);
assert.ok(Array.isArray(detail.goals) && detail.goals.length > 0, "detail.goals must contain initialized goals");
assert.ok(Array.isArray(detail.intentions) && detail.intentions.length > 0, "detail.intentions must contain initialized intentions");
assert.ok(isObject(detail.schedule) && Array.isArray(detail.schedule.items) && detail.schedule.items.length > 0, "detail.schedule.items must be non-empty");
for (const item of detail.schedule.items) {
  for (const key of ["start_at", "end_at", "activity", "scene"]) assert.ok(String(item[key] ?? "").trim(), `schedule item missing ${key}`);
}
report.cases.detail = {
  status: "passed",
  corePersonaRevision: detail.core_persona_revision,
  developingSelfClaims: detail.developing_self.claims.length,
  currentStateRevision: detail.current_state_revision,
  scheduleItems: detail.schedule.items.length,
};

// Case 3: direct conversation lookup -> real NDJSON turn -> persisted
// assistant message. This is the same transport used by the Web client.
const conversation = await jsonRequest(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/conversation`);
const conversationId = conversation?.conversation?.id;
assert.ok(typeof conversationId === "string" && conversationId.length > 0, "direct conversation id missing");
const turnResponse = await request(`/api/conversations/${encodeURIComponent(conversationId)}/turn`, {
  method: "POST",
  body: JSON.stringify({
    text: "请根据你当前的核心人格、正在形成的自我认知和此刻状态，简洁地告诉我你今天最想完成什么。",
    fluctlightId,
    idempotencyKey: `${requestId}:chat`,
    turnId: `${requestId}:turn`,
  }),
});
const events = await readNDJSON(turnResponse);
const terminals = events.filter((event) => event?.type === "completed" || event?.type === "error");
assert.equal(terminals.length, 1, `chat must have exactly one terminal event: ${JSON.stringify(events).slice(-2000)}`);
assert.equal(terminals[0].type, "completed", `chat returned error: ${JSON.stringify(terminals[0])}`);
const messageIds = terminals[0].payload?.message_ids ?? terminals[0].payload?.messageIds;
assert.ok(Array.isArray(messageIds) && messageIds.length > 0, "chat completed without persisted message ids");
const messages = await jsonRequest(`/api/conversations/${encodeURIComponent(conversationId)}/messages?limit=100`);
const assistantMessages = Array.isArray(messages?.messages) ? messages.messages.filter((message) => message.kind === "assistant" && String(message.text ?? "").trim()) : [];
assert.ok(assistantMessages.length > 0, "chat completed but no assistant message was persisted");
report.cases.chat = { status: "passed", conversationId, eventTypes: events.map((event) => event.type), assistantMessages: assistantMessages.length };

console.log(JSON.stringify(report, null, 2));
