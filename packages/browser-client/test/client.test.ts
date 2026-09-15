import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { BrowserApiError, BrowserClient } from "../src/index.ts";

test("BrowserClient resolves an empty base URL against the browser origin", async () => {
  const previousWindow = globalThis.window;
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: { location: { origin: "http://fluctlight.local" } },
  });
  try {
    let requestedUrl = "";
    const client = new BrowserClient("", async (input) => {
      requestedUrl = String(input);
      return Response.json({ authenticated: false });
    });

    await client.session();
    assert.equal(requestedUrl, "http://fluctlight.local/auth/session");
  } finally {
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: previousWindow,
    });
  }
});

test("BrowserClient maps an unauthenticated session response without treating it as a platform failure", async () => {
  const client = new BrowserClient("http://fluctlight.local", async () =>
    new Response(JSON.stringify({ authenticated: false }), {
      status: 401,
      headers: { "content-type": "application/json" },
    }),
  );

  assert.deepEqual(await client.session(), { authenticated: false });
});

test("BrowserClient requires an explicit base URL outside the browser", async () => {
  const previousWindow = globalThis.window;
  // @ts-expect-error This test deliberately exercises the non-browser boundary.
  delete globalThis.window;
  try {
    const client = new BrowserClient("", async () => Response.json({ authenticated: false }));
    await assert.rejects(() => client.session(), /requires a base URL outside the browser/);
  } finally {
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: previousWindow,
    });
  }
});

test("BrowserClient preserves safe BFF failure codes", async () => {
  const client = new BrowserClient("http://fluctlight.local", async () =>
    new Response(JSON.stringify({
      code: "initialization_response_invalid_json",
      message: "Fluctlight analysis was rejected",
    }), {
      status: 422,
      headers: { "content-type": "application/json" },
    }),
  );
  await assert.rejects(
    () => client.analyzeFluctlightCreation("测试描述"),
    (error: unknown) =>
      error instanceof BrowserApiError
      && error.status === 422
      && error.code === "initialization_response_invalid_json",
  );
});

test("BrowserClient preserves structured diagnostics details", async () => {
  const client = new BrowserClient("http://fluctlight.local", async () =>
    Response.json({ code: "initialization_persona_invalid", message: "分析失败", details: { correlation_id: "corr-1" } }, { status: 422 }),
  );
  await assert.rejects(
    () => client.analyzeFluctlightCreation("测试描述"),
    (error: unknown) => error instanceof BrowserApiError && error.details.correlation_id === "corr-1",
  );
});

test("BrowserClient preserves a non-2xx conversation turn error code", async () => {
  const client = new BrowserClient("http://fluctlight.local", async () =>
    Response.json({
      detail: { code: "conversation_turn_conflict", message: "The conversation turn failed", details: { internal: "must not surface" } },
    }, { status: 502 }),
  );
  await assert.rejects(
    () => client.turn("conversation-1", { text: "hello", fluctlightId: "fl-1", idempotencyKey: "turn-1" }),
    (error: unknown) =>
      error instanceof BrowserApiError
      && error.status === 502
      && error.code === "conversation_turn_conflict",
  );
});

test("BrowserClient transports initialization analysis authority and typed detail source", async () => {
	const requests: Array<{ url: string; body: unknown }> = [];
	const analysis = {
		analysis_id: "initialization_source_abc",
		correlation_id: "initialization-analysis:corr",
		schema_version: 2,
		core_persona: { identity: { name: "测试" } },
		developing_self: { claims: [] },
		extensions: {},
		initial_goals: [{ description: "编辑后的目标" }],
		initial_intentions: [{ action: "编辑后的意图" }],
		initial_relationships: [],
	};
	const client = new BrowserClient("http://fluctlight.local", async (input, init) => {
		const url = String(input);
		requests.push({ url, body: init?.body ? JSON.parse(String(init.body)) : undefined });
		if (url.endsWith("/analysis")) return Response.json(analysis);
		if (url.endsWith("/detail")) return Response.json({ id: "fl-1", initialization_source: null });
		return Response.json({ id: "fl-1", identity: {}, status: "active" });
	});
	const analyzed = await client.analyzeFluctlightCreation("复杂人格描述");
	assert.equal(analyzed.analysis_id, analysis.analysis_id);
	await client.activateFluctlightCreation({
		requestId: "activate-1",
		initializationMode: "llm_defined",
		analysisId: analyzed.analysis_id,
		schemaVersion: analyzed.schema_version,
		corePersona: analyzed.core_persona,
		developingSelf: analyzed.developing_self,
		extensions: analyzed.extensions,
		initialGoals: analyzed.initial_goals,
		initialIntentions: analyzed.initial_intentions,
		initialRelationships: analyzed.initial_relationships,
	});
	assert.deepEqual(requests[0].body, { description: "复杂人格描述" });
	assert.equal((requests[1].body as Record<string, unknown>).analysisId, analysis.analysis_id);
	assert.equal((await client.detail("fl-1")).initialization_source, null);

	const openapi = await readFile(new URL("../openapi.json", import.meta.url), "utf8");
	assert.match(openapi, /"BrowserFluctlightCreationAnalysisRequest"[\s\S]*?"maxLength": 60000,[\s\S]*?"x-maxBytes": 60000/);
	assert.match(openapi, /"\/api\/fluctlight-creations\/analysis"[\s\S]*?"requestBody"[\s\S]*?"required": true/);
});

test("BrowserClient requests the bounded media prompt diagnostics module", async () => {
  let requestedUrl = "";
  const client = new BrowserClient("http://fluctlight.local", async (input) => {
    requestedUrl = String(input);
    return Response.json([]);
  });

  await client.diagnosticMediaPrompts({ limit: 99 });
  assert.equal(requestedUrl, "http://fluctlight.local/api/diagnostics/media-prompts?limit=20");
});

test("BrowserClient exposes media prompt retry", async () => {
  let requestedUrl = "";
  let requestedMethod = "";
  const client = new BrowserClient("http://fluctlight.local", async (input, init) => {
    requestedUrl = String(input);
    requestedMethod = init?.method ?? "";
    return Response.json({ media_intent_id: "media-1", status: "retry_queued" });
  });

  await client.retryDiagnosticMediaPrompt("media-1");
  assert.equal(requestedUrl, "http://fluctlight.local/api/diagnostics/media-prompts/media-1/retry");
  assert.equal(requestedMethod, "POST");
});

test("BrowserClient serializes every lifecycle diagnostics filter for query and export", async () => {
  const requested: string[] = [];
  const client = new BrowserClient("http://fluctlight.local", async (input) => {
    requested.push(String(input));
    return Response.json({ events: [], workflowIntents: [], filters: {} });
  });
  const filters = {
    limit: 25,
    fluctlightId: "fl-1",
    correlationId: "corr-1",
    intentId: "intent-1",
    workflowId: "go:wake-1",
    runId: "run-1",
    surface: "wake_up",
    status: "retry",
  };
  await client.lifecycleDiagnostics(filters);
  await client.exportDiagnostics(filters);
  for (const url of requested) {
    const parsed = new URL(url);
    assert.equal(parsed.searchParams.get("limit"), "25");
    assert.equal(parsed.searchParams.get("fluctlightId"), "fl-1");
    assert.equal(parsed.searchParams.get("correlationId"), "corr-1");
    assert.equal(parsed.searchParams.get("intentId"), "intent-1");
    assert.equal(parsed.searchParams.get("workflowId"), "go:wake-1");
    assert.equal(parsed.searchParams.get("runId"), "run-1");
    assert.equal(parsed.searchParams.get("surface"), "wake_up");
    assert.equal(parsed.searchParams.get("status"), "retry");
  }
});

test("BrowserClient serializes the schema-derived Life Context command contracts", async () => {
	const requests: Array<{ url: string; method: string; body: unknown }> = [];
	const client = new BrowserClient("http://fluctlight.local", async (input, init) => {
		const body = init?.body ? JSON.parse(String(init.body)) : undefined;
		requests.push({ url: String(input), method: init?.method ?? "GET", body });
		return String(input).includes("/cancel") ? new Response(null, { status: 204 }) : Response.json({});
	});
	const lifeRevision = "life_ctx_0123456789abcdef0123456789abcdef";
	await client.createLifeEvent("fl-1", {
		kind: "meeting", startAt: "2026-09-11T08:00:00Z", endAt: "2026-09-11T09:00:00Z",
		evidenceRefs: ["fact-1"], expectedLifeContextRevision: lifeRevision, idempotencyKey: "event-create-1",
	});
	await client.cancelLifeEvent("fl-1", "event-1", { expectedEventRevision: 2, expectedLifeContextRevision: lifeRevision, idempotencyKey: "event-cancel-1" });
	await client.setLifePresence("fl-1", { currentTask: "review", expectedLifeContextRevision: lifeRevision, idempotencyKey: "presence-1" });
	await client.acceptLifeSchedule("fl-1", {
		localDate: "2026-09-11", timezone: "UTC", expectedRevision: 0, expectedLifeContextRevision: lifeRevision,
		idempotencyKey: "schedule-1", evidenceRefs: ["fact-1"],
		items: [{ startAt: "2026-09-11T00:00:00Z", endAt: "2026-09-12T00:00:00Z", activity: "review", scene: "office", location: "Shanghai" }],
	});
	await client.cancelLifeSchedule("fl-1", "schedule-1", { expectedRevision: 1, expectedLifeContextRevision: lifeRevision, idempotencyKey: "schedule-cancel-1" });
	assert.deepEqual(requests, [
		{ url: "http://fluctlight.local/api/fluctlights/fl-1/events", method: "POST", body: { kind: "meeting", startAt: "2026-09-11T08:00:00Z", endAt: "2026-09-11T09:00:00Z", evidenceRefs: ["fact-1"], expectedLifeContextRevision: lifeRevision, idempotencyKey: "event-create-1" } },
		{ url: "http://fluctlight.local/api/fluctlights/fl-1/events/event-1/cancel", method: "POST", body: { expectedEventRevision: 2, expectedLifeContextRevision: lifeRevision, idempotencyKey: "event-cancel-1" } },
		{ url: "http://fluctlight.local/api/fluctlights/fl-1/presence", method: "PUT", body: { currentTask: "review", expectedLifeContextRevision: lifeRevision, idempotencyKey: "presence-1" } },
		{ url: "http://fluctlight.local/api/fluctlights/fl-1/schedules", method: "POST", body: { localDate: "2026-09-11", timezone: "UTC", expectedRevision: 0, expectedLifeContextRevision: lifeRevision, idempotencyKey: "schedule-1", evidenceRefs: ["fact-1"], items: [{ startAt: "2026-09-11T00:00:00Z", endAt: "2026-09-12T00:00:00Z", activity: "review", scene: "office", location: "Shanghai" }] } },
		{ url: "http://fluctlight.local/api/fluctlights/fl-1/schedules/schedule-1/cancel", method: "POST", body: { expectedRevision: 1, expectedLifeContextRevision: lifeRevision, idempotencyKey: "schedule-cancel-1" } },
	]);
});
