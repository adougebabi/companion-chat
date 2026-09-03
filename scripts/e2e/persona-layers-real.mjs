import assert from "node:assert/strict";

const baseUrl = process.env.FLUCTLIGHT_E2E_BASE_URL ?? "http://127.0.0.1:13000";
const origin = process.env.FLUCTLIGHT_E2E_ORIGIN ?? "http://127.0.0.1:13001";
const password = process.env.FLUCTLIGHT_E2E_PASSWORD;
const timeoutMs = Number(process.env.FLUCTLIGHT_E2E_TIMEOUT_MS ?? 600000);
const pollMs = Number(process.env.FLUCTLIGHT_E2E_POLL_MS ?? 5000);
const pollRequestTimeoutMs = Number(process.env.FLUCTLIGHT_E2E_POLL_REQUEST_TIMEOUT_MS ?? 30000);

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

function pollJson(path) {
  return jsonRequest(path, { signal: AbortSignal.timeout(pollRequestTimeoutMs) });
}

function isObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function assertNonEmptyObject(value, label) {
  assert.equal(isObject(value), true, `${label} must be an object`);
  assert.ok(Object.keys(value).length > 0, `${label} must not be empty`);
}

function assertNoLegacyLayerNames(value, label) {
  assert.equal("foundation" in value, false, `${label} must not expose the legacy foundation envelope`);
  assert.equal("self_model" in value, false, `${label} must not expose the legacy self_model envelope`);
  assert.equal("personality_candidates" in value, false, `${label} must not expose personality_candidates`);
  assert.equal("self_model_candidates" in value, false, `${label} must not expose self_model_candidates`);
}

function assertCoreLayers(value, label) {
  assertNonEmptyObject(value?.core_persona, `${label}.core_persona`);
  for (const key of ["identity", "personality", "behavioral_policy", "life_profile"]) {
    assertNonEmptyObject(value.core_persona[key], `${label}.core_persona.${key}`);
  }
  assertNonEmptyObject(value?.developing_self, `${label}.developing_self`);
  assert.ok(Array.isArray(value.developing_self.claims), `${label}.developing_self.claims must be an array`);
  for (const [index, claim] of value.developing_self.claims.entries()) {
    assert.ok(isObject(claim), `${label}.developing_self.claims[${index}] must be an object`);
    for (const key of ["category", "claim", "value", "confidence", "evidence_refs", "provenance"]) {
      assert.ok(key in claim, `${label}.developing_self.claims[${index}] missing ${key}`);
    }
    assert.ok(Number.isFinite(Number(claim.confidence)) && Number(claim.confidence) >= 0 && Number(claim.confidence) <= 1, `${label}.developing_self.claims[${index}] confidence invalid`);
    assert.ok(Array.isArray(claim.evidence_refs), `${label}.developing_self.claims[${index}] evidence_refs must be an array`);
    assertNonEmptyObject(claim.provenance, `${label}.developing_self.claims[${index}].provenance`);
    assert.ok(String(claim.provenance.source ?? "").trim(), `${label}.developing_self.claims[${index}].provenance.source missing`);
  }
  assertNoLegacyLayerNames(value, label);
}

async function waitUntil(label, read, predicate, waitTimeoutMs = timeoutMs) {
  const deadline = Date.now() + waitTimeoutMs;
  let latest;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      latest = await read();
      lastError = "";
    } catch (error) {
      lastError = String(error);
      latest = { error: lastError };
    }
    if (predicate(latest)) return latest;
    await new Promise((resolve) => setTimeout(resolve, pollMs));
  }
  throw new Error(`${label} was not observed before timeout${lastError ? ` (${lastError})` : ""}: ${JSON.stringify(latest).slice(0, 4000)}`);
}

function messageAttachments(message) {
  const refs = message?.attachmentRefs ?? message?.attachment_refs ?? [];
  return Array.isArray(refs) ? refs.map((value) => String(value)).filter(Boolean) : [];
}

async function readNDJSON(response) {
  if (!response.ok) {
    return { events: [], error: `HTTP ${response.status}: ${(await response.text()).slice(0, 2000)}` };
  }
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
  return { events, error: null };
}

async function sendTurn(conversationId, fluctlightId, text, idempotencyKey, turnId) {
  let lastFailure = "";
  for (let attempt = 0; attempt < 2; attempt += 1) {
    let response;
    try {
      response = await request(`/api/conversations/${encodeURIComponent(conversationId)}/turn`, {
        method: "POST",
        body: JSON.stringify({ text, fluctlightId, idempotencyKey, turnId }),
      });
    } catch (error) {
      lastFailure = String(error);
      if (attempt === 0) continue;
      throw new Error(`conversation turn request failed: ${lastFailure}`);
    }
    const parsed = await readNDJSON(response);
    if (parsed.error) {
      lastFailure = parsed.error;
      if (attempt === 0) continue;
      throw new Error(`conversation turn request failed: ${lastFailure}`);
    }
    const terminals = parsed.events.filter((event) => event?.type === "completed" || event?.type === "error");
    if (terminals.length === 1 && terminals[0].type === "completed") {
      return { events: parsed.events, terminal: terminals[0] };
    }
    lastFailure = `terminal events: ${JSON.stringify(terminals).slice(-2000)}`;
    if (terminals.length === 1 && terminals[0].type === "error" && attempt === 0) continue;
    throw new Error(`conversation turn did not complete: ${lastFailure}`);
  }
  throw new Error(`conversation turn did not complete: ${lastFailure}`);
}

async function waitForSchedule(fluctlightId) {
  return waitUntil(
    "schedule",
    () => pollJson(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/detail`),
    (detail) => isObject(detail?.schedule) && Array.isArray(detail.schedule.items) && detail.schedule.items.length > 0,
  );
}

async function waitForWakeUp(fluctlightId) {
  return waitUntil(
    "first wake-up cognition",
    () => pollJson(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/detail`),
    (detail) => Array.isArray(detail?.wake_ups) && detail.wake_ups.some((wake) => wake.status === "completed"),
  );
}

async function waitForImageAttachment(conversationId) {
  return waitUntil(
    "generated image attachment",
    () => pollJson(`/api/conversations/${encodeURIComponent(conversationId)}/messages?limit=100`),
    (page) => Array.isArray(page?.messages) && page.messages.some((message) => messageAttachments(message).length > 0),
  );
}

async function waitForAutonomyAction(fluctlightId, actionType) {
  return waitUntil(
    `completed ${actionType} autonomy action`,
    () => pollJson(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/autonomy-actions`),
    (actions) => Array.isArray(actions) && actions.some((action) => action.action_type === actionType && action.status === "completed"),
  );
}

async function waitForReflection(fluctlightId, createdAfter) {
  return waitUntil(
    "reflection model run",
    () => pollJson("/api/diagnostics/model-runs?limit=200"),
    (runs) => Array.isArray(runs) && runs.some((run) => {
      if (run.role !== "reflection" || run.status !== "completed" || Date.parse(run.createdAt ?? "") < createdAfter) return false;
      return JSON.stringify(run.prompt ?? {}).includes(fluctlightId) || JSON.stringify(run.response ?? {}).includes(fluctlightId);
    }),
  );
}

async function createFluctlight(description, requestPrefix, options = {}) {
  const analysis = await jsonRequest("/api/fluctlight-creations/analysis", {
    method: "POST",
    body: JSON.stringify({ description }),
  });
  assertCoreLayers(analysis, "analysis");
  assert.ok(Array.isArray(analysis.initial_goals) && analysis.initial_goals.length > 0, "analysis.initial_goals must be non-empty");
  assert.ok(Array.isArray(analysis.initial_intentions) && analysis.initial_intentions.length > 0, "analysis.initial_intentions must be non-empty");
  const corePersona = JSON.parse(JSON.stringify(analysis.core_persona));
  if (options.timezone) {
    corePersona.identity.timezone = options.timezone;
  }
  if (options.behavioralPolicy) {
    corePersona.behavioral_policy = { ...corePersona.behavioral_policy, ...options.behavioralPolicy };
  }
  const requestId = `${requestPrefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
  let created;
  try {
    created = await jsonRequest("/api/fluctlight-creations/activate", {
      method: "POST",
      body: JSON.stringify({
        requestId,
        initializationMode: "llm_defined",
        corePersona,
        developingSelf: analysis.developing_self,
        initialGoals: analysis.initial_goals,
        initialIntentions: analysis.initial_intentions,
      }),
    });
  } catch (error) {
    console.error("activation rejected; analysis payload:", JSON.stringify(analysis, null, 2));
    throw error;
  }
  assert.ok(typeof created?.id === "string" && created.id.length > 0, "activation did not return a Fluctlight id");
  return { analysis, requestId, id: created.id };
}

const report = { baseUrl, cases: {} };
const announce = (message) => console.error(`[persona-e2e] ${message}`);

// Authenticate against the real BFF and obtain the CSRF cookie used by all
// subsequent mutations. No provider, database, or HTTP layer is mocked.
announce("authenticating against the real BFF");
await jsonRequest("/auth/setup-status");
await jsonRequest("/auth/login", { method: "POST", body: JSON.stringify({ password }) });
assert.equal((await jsonRequest("/auth/session")).authenticated, true);

// Case 1: natural-language description -> real initialization provider ->
// preview payload -> explicit activation.
announce("case 1: analyzing description and activating a new Fluctlight");
const baseFixture = await createFluctlight(
  "创建一个名为影者的高冷成熟理性女性摇光。她独立、主动、有判断力，不刻意讨好。她是一名上海研究生，生活中会学习、阅读、散步，喜欢安静环境但这只是待观察的偏好。请把以下作为明确的生活计划和行为策略：今天完成研究后发布一条公开动态，今晚主动给 Owner 发一条消息；如果 Owner 明确索要一张照片，必须调用图片能力并把真实图片发送到聊天。请生成完整生活设定、目标、初始意图和今天的日程。",
  "persona-layer-e2e",
);
const { analysis, requestId, id: fluctlightId } = baseFixture;
report.cases.create = { status: "passed", coreKeys: Object.keys(analysis.core_persona), developingSelfClaims: analysis.developing_self.claims.length };
report.cases.activation = { status: "passed", fluctlightId };
announce(`case 1 passed: ${fluctlightId}`);

// Case 2: wait for the real schedule workflow, then verify the detail read
// model contains all three layers and the important current-day sections.
announce("case 2: waiting for the real schedule workflow and reading detail");
const detail = await waitForSchedule(fluctlightId);
const scheduleObservedAt = Date.now();
assertCoreLayers(detail, "detail");
for (const key of ["identity", "personality", "behavioral_policy", "life_profile", "provenance", "current_state", "context"]) {
  assertNonEmptyObject(detail[key], `detail.${key}`);
}
for (const key of ["goals", "intentions", "relationships", "memories", "events", "cognition_history", "wake_ups", "foundation_revisions", "evolution_revisions", "developing_self_revisions"]) {
  assert.ok(Array.isArray(detail[key]), `detail.${key} must be an array`);
}
const innerState = detail.current_state.inner_state ?? detail.inner_state;
assertNonEmptyObject(innerState, "detail.current_state.inner_state");
for (const key of ["pad", "mood", "momentum", "regulation", "conflicts"]) assert.ok(key in innerState, `detail current state missing ${key}`);
for (const key of ["core_persona_revision", "developing_self_revision", "current_state_revision"]) assert.ok(Number.isInteger(detail[key]) && detail[key] >= 0, `detail.${key} must be a non-negative revision`);
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
announce(`case 2 passed: ${detail.schedule.items.length} schedule items`);

// The first internal cognition must follow schedule acceptance. This is a
// real durable wake-up row, not a client-side timer or a synthetic fixture.
announce("case 2b: waiting for the first real wake-up after schedule acceptance");
const detailWithWakeUp = await waitForWakeUp(fluctlightId);
const firstWakeAt = Math.min(...detailWithWakeUp.wake_ups.map((wake) => Date.parse(wake.occurred_at)).filter(Number.isFinite));
assert.ok(Number.isFinite(firstWakeAt), "wake-up record has no occurred_at timestamp");
assert.ok(firstWakeAt >= scheduleObservedAt - 5000, `first wake-up (${new Date(firstWakeAt).toISOString()}) preceded observed schedule acceptance (${new Date(scheduleObservedAt).toISOString()})`);
report.cases.wakeup = {
  status: "passed",
  wakeUpCount: detailWithWakeUp.wake_ups.length,
  firstWakeAt: new Date(firstWakeAt).toISOString(),
  scheduleObservedAt: new Date(scheduleObservedAt).toISOString(),
};
announce(`case 2b passed: first wake-up recorded at ${new Date(firstWakeAt).toISOString()}`);

// Cases 3–4: direct conversation lookup -> real NDJSON turns -> persisted
// assistant replies. This is the same transport used by the Web client.
announce("case 3: sending a normal text turn");
const conversation = await jsonRequest(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/conversation`);
const conversationId = conversation?.conversation?.id;
assert.ok(typeof conversationId === "string" && conversationId.length > 0, "direct conversation id missing");
const normalTurn = await sendTurn(
  conversationId,
  fluctlightId,
  "请根据你当前的核心人格、正在形成的自我认知和此刻状态，简洁地告诉我你今天最想完成什么。",
  `${requestId}:chat`,
  `${requestId}:turn`,
);
const normalMessages = await jsonRequest(`/api/conversations/${encodeURIComponent(conversationId)}/messages?limit=100`);
const normalAssistant = Array.isArray(normalMessages?.messages) ? normalMessages.messages.filter((message) => message.kind === "assistant" && String(message.text ?? "").trim()) : [];
assert.ok(normalAssistant.length > 0, "chat completed but no assistant message was persisted");
report.cases.chat = { status: "passed", conversationId, eventTypes: normalTurn.events.map((event) => event.type), assistantMessages: normalAssistant.length };
announce("case 3 passed: assistant reply persisted");

announce("case 4: requesting a real generated photo");
const imageTurn = await sendTurn(
  conversationId,
  fluctlightId,
  "影者，请现在为我拍一张照片并发送给我。必须调用 media.image.generate 生成真实图片，不要只用文字描述或解释不能生图。",
  `${requestId}:image`,
  `${requestId}:image-turn`,
);
const imageMessages = await waitForImageAttachment(conversationId);
const imageMessage = imageMessages.messages.find((message) => messageAttachments(message).length > 0);
const imageAssetIds = messageAttachments(imageMessage);
assert.ok(imageAssetIds.length > 0, "photo request completed but no image attachment was persisted");
const imageAssets = [];
for (const assetId of imageAssetIds) {
  const assetResponse = await request(`/api/media/${encodeURIComponent(assetId)}`);
  assert.equal(assetResponse.ok, true, `generated image ${assetId} could not be read`);
  assert.match(assetResponse.headers.get("content-type") ?? "", /^image\//i, `generated asset ${assetId} is not an image`);
  const bytes = new Uint8Array(await assetResponse.arrayBuffer());
  assert.ok(bytes.length > 0, `generated image ${assetId} is empty`);
  imageAssets.push({ id: assetId, bytes: bytes.length, contentType: assetResponse.headers.get("content-type") });
}
report.cases.image = { status: "passed", eventTypes: imageTurn.events.map((event) => event.type), assets: imageAssets };
announce(`case 4 passed: ${imageAssets.length} image asset(s) persisted and readable`);

// Case 7: use a real fixture whose owner-edited plan contains only a
// proactive Owner contact. A daily-review run produces one autonomous action
// type, so keeping this fixture separate avoids making the result depend on
// which of two valid intentions the model picks first.
announce("case 7: creating a proactive-contact fixture and waiting for a real Owner message");
const proactiveFixture = await createFluctlight(
  "创建一个名为影者主动联系的高冷成熟理性女性摇光。她独立、主动、有判断力，不刻意讨好。她不发布动态，不调用图片能力；她的明确日程计划是每天完成研究后必须主动给 Owner 发一条简短消息，汇报一个真实进展。请生成完整生活设定、目标、初始意图和今天的日程，并把主动联系 Owner 作为唯一的自主输出计划。",
  "persona-layer-proactive-e2e",
  {
    timezone: "UTC",
    behavioralPolicy: {
      required_action_type: "proactive_message",
      autonomy: "当前 daily review 必须执行一次 proactive_message，向 Owner 发送一条简短、事实导向的研究进展；不得选择 no_op。",
    },
  },
);
const proactiveAction = await waitForAutonomyAction(proactiveFixture.id, "proactive_message");
const proactiveConversation = await pollJson(`/api/fluctlights/${encodeURIComponent(proactiveFixture.id)}/conversation`);
const proactiveConversationId = proactiveConversation?.conversation?.id;
assert.ok(typeof proactiveConversationId === "string" && proactiveConversationId.length > 0, "proactive fixture conversation id missing");
const proactiveMessages = await waitUntil(
  "proactive Owner message",
  () => pollJson(`/api/conversations/${encodeURIComponent(proactiveConversationId)}/messages?limit=100`),
  (page) => Array.isArray(page?.messages) && page.messages.some((message) => message.kind === "assistant" && String(message.text ?? "").trim()),
);
assert.ok(proactiveMessages.messages.some((message) => message.kind === "assistant" && String(message.text ?? "").trim()), "proactive action completed but no assistant message was persisted");
report.cases.proactive = { status: "passed", actionId: proactiveAction.id, fixtureId: proactiveFixture.id };
announce("case 7 passed: proactive assistant message persisted");

// Case 6: use a second real fixture whose owner-edited policy forbids
// proactive DMs and requires a public dynamic. This keeps the assertion
// deterministic without inserting a Moment or forcing an LLM decision in the
// database; both analysis and activation still use the public API.
announce("case 6: creating a Moment-only fixture and waiting for a real published Moment");
const momentAnalysis = await jsonRequest("/api/fluctlight-creations/analysis", {
  method: "POST",
  body: JSON.stringify({
    description: "创建一个名为影者动态记录的高冷成熟理性女性摇光。她独立、主动、有判断力，不刻意讨好。她每天完成研究后必须发布一条公开动态，动态是她与外界的主要连接方式；她禁止主动私聊 Owner，也不发送主动消息。请生成完整生活设定、目标、初始意图和今天的日程。",
  }),
});
assertCoreLayers(momentAnalysis, "moment analysis");
assert.ok(Array.isArray(momentAnalysis.initial_goals) && momentAnalysis.initial_goals.length > 0, "moment analysis.initial_goals must be non-empty");
assert.ok(Array.isArray(momentAnalysis.initial_intentions) && momentAnalysis.initial_intentions.length > 0, "moment analysis.initial_intentions must be non-empty");
const momentCorePersona = JSON.parse(JSON.stringify(momentAnalysis.core_persona));
momentCorePersona.identity.timezone = "UTC";
momentCorePersona.behavioral_policy = {
  ...momentCorePersona.behavioral_policy,
  required_action_type: "moment",
  autonomy: "当前 daily review 必须执行一次 moment，发布一条简短、事实导向的公开动态；不得选择 no_op 或 proactive_message。",
};
const momentRequestId = `persona-layer-moment-e2e-${Date.now()}-${Math.random().toString(36).slice(2)}`;
const momentCreated = await jsonRequest("/api/fluctlight-creations/activate", {
  method: "POST",
  body: JSON.stringify({
    requestId: momentRequestId,
    initializationMode: "llm_defined",
    corePersona: momentCorePersona,
    developingSelf: momentAnalysis.developing_self,
    initialGoals: momentAnalysis.initial_goals,
    initialIntentions: momentAnalysis.initial_intentions,
  }),
});
assert.ok(typeof momentCreated?.id === "string" && momentCreated.id.length > 0, "Moment fixture activation did not return an id");
const momentFluctlightId = momentCreated.id;
const momentAction = await waitForAutonomyAction(momentFluctlightId, "moment");
const moments = await waitUntil(
  "published Moment",
  () => pollJson(`/api/fluctlights/${encodeURIComponent(momentFluctlightId)}/moments`),
  (items) => Array.isArray(items) && items.some((item) => item.status === "visible" && String(item.text ?? "").trim()),
);
const visibleMoment = moments.find((item) => item.status === "visible" && String(item.text ?? "").trim());
assert.ok(visibleMoment, "moment action completed but no visible Moment was persisted");
report.cases.moment = { status: "passed", actionId: momentAction.id, momentId: visibleMoment.id, fixtureId: momentFluctlightId };
announce("case 6 passed: visible Moment persisted");

// Case 8: ask the model to record an explicit long-lived fact through the
// memory_event capability, then verify a later fresh turn can recall it.
announce("case 8: recording and recalling a historical memory");
const detailBeforeMemory = await jsonRequest(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/detail`);
const memoryCountBefore = Array.isArray(detailBeforeMemory.memories) ? detailBeforeMemory.memories.length : 0;
const reflectionStartedAt = Date.now();
const memoryText = "请把这个事实作为长期记忆记录：我偏爱雾蓝色、阴天和低饱和的安静色调。以后选择视觉方案时优先考虑这些偏好。请使用 memory_event 保存。";
await sendTurn(conversationId, fluctlightId, memoryText, `${requestId}:memory`, `${requestId}:memory-turn`);
const detailWithMemory = await waitUntil(
  "historical memory",
  () => pollJson(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/detail`),
  (value) => Array.isArray(value?.memories) && value.memories.length > memoryCountBefore && value.memories.some((memory) => /雾蓝色|阴天|低饱和/.test(String(memory.content ?? ""))),
);
const remembered = detailWithMemory.memories.find((memory) => /雾蓝色|阴天|低饱和/.test(String(memory.content ?? "")));
assert.ok(remembered, "memory_event did not create the requested historical memory");
const recallTurn = await sendTurn(conversationId, fluctlightId, "你还记得我偏好的颜色和天气氛围吗？请直接回答记忆中的具体内容。", `${requestId}:recall`, `${requestId}:recall-turn`);
const recallMessages = await jsonRequest(`/api/conversations/${encodeURIComponent(conversationId)}/messages?limit=100`);
const latestAssistantText = [...recallMessages.messages].reverse().find((message) => message.kind === "assistant" && String(message.text ?? "").trim())?.text ?? "";
assert.match(latestAssistantText, /雾蓝色|阴天|低饱和/, "recall reply did not mention the persisted memory");
report.cases.memory = { status: "passed", memoryId: remembered.id, recallEventTypes: recallTurn.events.map((event) => event.type) };
announce(`case 8 passed: recalled memory ${remembered.id}`);

// Case 9: the same evidence window must be reflected by a completed real
// reflection model run associated with this Fluctlight, not just a chat reply.
announce("case 9: waiting for a completed reflection and applied revision");
const reflectionRun = await waitForReflection(fluctlightId, reflectionStartedAt);
const reflectedDetail = await jsonRequest(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/detail`);
assert.ok(Array.isArray(reflectedDetail.developing_self_revisions), "detail is missing Developing Self revisions after reflection");
assert.equal(reflectionRun.status, "completed", "reflection model run did not complete");
assert.ok(reflectedDetail.developing_self_revisions.length >= detailWithMemory.developing_self_revisions.length, "Developing Self revision history regressed after reflection");
report.cases.reflection = { status: "passed", modelRunId: reflectionRun.id, developingSelfRevisions: reflectedDetail.developing_self_revisions.length };
announce("case 9 passed: reflection revision applied");

console.log(JSON.stringify(report, null, 2));
