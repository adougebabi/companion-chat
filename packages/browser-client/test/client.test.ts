import assert from "node:assert/strict";
import test from "node:test";

import { BrowserClient } from "../src/index.ts";

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
