import assert from "node:assert/strict";
import test from "node:test";

import { BrowserClient, currentRevision, studioApplyUnfinished } from "../src/index.ts";

function jsonResponse(body: unknown, status = 200, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json", ...headers } });
}

test("wardrobe wear returns the 202 owner request and does not imply success", async () => {
  let body = "";
  const client = new BrowserClient("http://fluctlight.local", async (_input, init) => {
    body = String(init?.body || "");
    return jsonResponse({ request: { id: "req_1", kind: "wardrobe_wear", fluctlight_id: "c1", status: "pending", payload: {}, decided_by: null, decision_reason: null, result_refs: [], created_at: "2026-10-15T00:00:00Z", updated_at: "2026-10-15T00:00:00Z", revision: 1 }, audit_id: "au_1" }, 202);
  });
  const response = await client.requestWardrobeWear("c1", { itemId: "it_coat", slot: "outer", expectedRevision: 4, reason: "想换上", idempotencyKey: "idem-1" });
  assert.equal(response.request.status, "pending");
  assert.equal(response.request.kind, "wardrobe_wear");
  assert.match(body, /idempotencyKey/);
  assert.match(body, /expectedRevision/);
  assert.doesNotMatch(body, /Idempotency-Key/);
});

test("409 current revision is read from details and is absent for other errors", () => {
  assert.equal(currentRevision({ status: 409, details: { current_revision: 12, expected_revision: 11, resource: "wardrobe" } }), 12);
  assert.equal(currentRevision({ status: 422, details: { current_revision: 12 } }), undefined);
  assert.equal(currentRevision({ status: 409, details: {} }), undefined);
});

test("createMoment refuses a character author before the request is sent", async () => {
  let called = false;
  const client = new BrowserClient("http://fluctlight.local", async () => { called = true; return jsonResponse({}); });
  await assert.rejects(() => client.createMoment({ text: "代发", mediaIds: [], idempotencyKey: "idem-2", authorFluctlightId: "c1" } as never), (error: { code?: string }) => error.code === "moment_author_forbidden");
  assert.equal(called, false);
});

test("multipart upload puts the idempotency key in the header", async () => {
  let header = "";
  const client = new BrowserClient("http://fluctlight.local", async (_input, init) => {
    header = new Headers(init?.headers).get("Idempotency-Key") || "";
    return jsonResponse({ asset_id: "up_1", kind: "image", mime: "image/png", size: 3, factual: true, uploaded_by: "owner" }, 201);
  });
  const uploaded = await client.uploadOwnerMedia(new Blob(["png"]), "image/png", "idem-upload");
  assert.equal(header, "idem-upload");
  assert.equal(uploaded.factual, true);
});

test("studio apply stays unfinished when the choice was recorded and not applied", () => {
  assert.equal(studioApplyUnfinished({ status: "recorded_not_applied", applied: false, resource_version: "v4", instance_revision: null, choices: [{ field: "营业状态", decision: "keep" }], conflicts: null }), true);
  assert.equal(studioApplyUnfinished({ status: "recorded_not_applied", applied: false, instance_revision: 7, conflicts: null }), true);
  assert.equal(studioApplyUnfinished({ available: false }), true);
  assert.equal(studioApplyUnfinished(null), true);
});

test("timeline rollback is not a client method", () => {
  assert.equal("listTimelineCheckpoints" in BrowserClient.prototype, false);
  assert.equal("restoreTimeline" in BrowserClient.prototype, false);
  assert.equal(typeof BrowserClient.prototype.requestWardrobeWear, "function");
  assert.equal(typeof BrowserClient.prototype.suggestMoment, "function");
  assert.equal(typeof BrowserClient.prototype.unreadNotificationCount, "function");
});
