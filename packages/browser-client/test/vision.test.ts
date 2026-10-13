import assert from "node:assert/strict";
import test from "node:test";
import { BrowserClient } from "../src/index.ts";

test("vision probe is an explicit authenticated mutation without an uploaded image", async () => {
  const calls: Array<{ url: string; method: string; body: unknown }> = [];
  const client = new BrowserClient("http://fluctlight.local", async (input, init) => {
    calls.push({ url: String(input), method: String(init?.method), body: init?.body ? JSON.parse(String(init.body)) : null });
    return Response.json({ state: "ready", config_version: 3, image_input_verified: true, error_code: "" });
  });
  assert.equal(calls.length, 0);
  const result = await client.testVisionUnderstanding();
  assert.equal(result.image_input_verified, true);
  assert.deepEqual(calls, [{ url: "http://fluctlight.local/api/settings/vision/test", method: "POST", body: {} }]);
});
