import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
const client = await readFile(new URL("../../../packages/browser-client/src/index.ts", import.meta.url), "utf8");
test("browser client keeps capability request endpoints at the API boundary", () => {
  assert.match(client, /listCapabilityRequests/);
  assert.match(client, /reviewCapabilityRequest/);
  assert.match(client, /api\/capability-requests/);
});
