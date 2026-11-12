import assert from "node:assert/strict";
import test from "node:test";
import { BrowserClient } from "../src/index.ts";

test("Moment health check is explicit and does not publish", async () => {
  const calls: Array<{ url: string; method: string; body: unknown }> = [];
  const client = new BrowserClient("http://fluctlight.local", async (input, init) => {
    calls.push({ url: String(input), method: String(init?.method), body: init?.body ? JSON.parse(String(init.body)) : null });
    return Response.json({ state: "ready", config_version: 2, plugin_enabled: true, registry_checked: true, data_read_tested: true, publication_tested: false, model_tested: false });
  });
  assert.equal(calls.length, 0);
  const result = await client.testMomentPluginHealth();
  assert.equal(result.publication_tested, false);
  assert.equal(result.model_tested, false);
  assert.deepEqual(calls, [{ url: "http://fluctlight.local/api/settings/moment/test-health", method: "POST", body: {} }]);
});
