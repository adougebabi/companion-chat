import assert from "node:assert/strict";
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
