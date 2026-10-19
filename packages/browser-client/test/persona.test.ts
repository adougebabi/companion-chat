import assert from "node:assert/strict";
import test from "node:test";
import { BrowserClient } from "../src/index.ts";

test("Owner persona switch preserves both CAS authorities and reads a terminal receipt", async () => {
  let payload: unknown;
  const client = new BrowserClient("http://fluctlight.local", async (_input, init) => {
    payload = JSON.parse(String(init?.body));
    return new Response(JSON.stringify({ request_id: "req-1", status: "completed", disposition: "applied", revision: 4, runtime_revision: 8, active_profile_id: "b", previous_profile_id: "a", audit_id: "owner-audit", persona_action_audit_id: "action-audit", request: { status: "completed" } }), { status: 200, headers: { "content-type": "application/json" } });
  });
  const body = { targetPersonaId: "b", expectedRevision: 4, expectedRuntimeRevision: 7, reason: "Owner command", confirm: true as const, idempotencyKey: "persona-1" };
  const result = await client.switchPersona("fl-1", body);
  assert.deepEqual(payload, body);
  assert.equal(result.status, "completed");
  assert.equal(result.runtime_revision, 8);
  assert.equal(result.revision, 4);
});

test("generated BrowserClient methods follow accepted Owner persona request to typed terminal", async () => {
  const urls: string[] = [];
  const client = new BrowserClient("http://fluctlight.local", async (input, init) => {
    urls.push(String(input));
    if (init?.method === "POST") {
      return Response.json({ request_id: "req-1", status: "pending", status_url: "/api/owner-requests/req-1", source_fact_id: "req-1", disposition: "assessment_pending", revision: 4, runtime_revision: 7, active_profile_id: "a", previous_profile_id: "a", audit_id: "owner-audit", persona_action_audit_id: null, request: { id: "req-1", status: "pending" } }, { status: 202 });
    }
    return Response.json({ id: "req-1", kind: "persona_switch", status: "completed", fluctlight_id: "fl-1", result: { request_id: "req-1", status: "completed", active_profile_id: "b", runtime_revision: 8, source_fact_id: "req-1" } });
  });
  assert.equal(typeof client.switchPersona, "function");
  assert.equal(typeof client.ownerRequest, "function");
  const pending = await client.switchPersona("fl-1", { targetPersonaId: "b", expectedRevision: 4, expectedRuntimeRevision: 7, reason: "Owner request", confirm: true, idempotencyKey: "pending-persona" });
  assert.equal(pending.status, "pending");
  assert.equal(pending.active_profile_id, "a");
  const terminal = await client.ownerRequest<import("../src/index.ts").BrowserPersonaSwitchTerminal>(pending.request_id);
  assert.equal(terminal.result?.status, "completed");
  assert.equal(terminal.result?.active_profile_id, "b");
  assert.equal(terminal.result?.runtime_revision, 8);
  assert.deepEqual(urls, ["http://fluctlight.local/api/fluctlights/fl-1/persona-switch", "http://fluctlight.local/api/owner-requests/req-1"]);
});
