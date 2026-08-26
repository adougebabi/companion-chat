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
