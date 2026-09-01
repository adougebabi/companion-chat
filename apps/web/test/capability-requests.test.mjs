import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const governance = await readFile(new URL("../src/views/GovernanceView.vue", import.meta.url), "utf8");
const client = await readFile(new URL("../../../packages/browser-client/src/index.ts", import.meta.url), "utf8");

test("governance exposes the owner capability request pool", () => {
  assert.match(governance, /能力需求池/);
  assert.match(governance, /reviewCapabilityRequest/);
  assert.match(governance, /capabilityRequestVersions/);
});

test("browser client keeps capability request endpoints at the BFF boundary", () => {
  assert.match(client, /listCapabilityRequests/);
  assert.match(client, /reviewCapabilityRequest/);
  assert.match(client, /api\/capability-requests/);
});
