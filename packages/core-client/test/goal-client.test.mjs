import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/index.ts", import.meta.url), "utf8");
const js = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText;
const { CoreClient, CoreApiError } = await import(`data:text/javascript;base64,${Buffer.from(js).toString("base64")}`);

test("Core Goal methods preserve encoded scope, independent session headers and replay identity", async () => {
  const calls = [];
  const client = new CoreClient("http://core.invalid", "test-service", async (url, init) => {
    calls.push({ url: String(url), method: init.method, headers: init.headers, body: init.body });
    return Response.json({ items: [], next_cursor: "older", goal_id: "goal/1", revision: 2, status: "paused" });
  });
  await client.goals("test-human-session", "fl/1", { history: true, cursor: "history+/cursor" });
  await client.goalDetail("test-human-session", "fl/1", "goal/1");
  await client.goalHistory("test-human-session", "fl/1", "goal/1", "revision+cursor");
  await client.goalEvidence("test-human-session", "fl/1", "goal/1", "evidence+cursor");
  const command = { expected_revision: 1, reason: "wait", idempotency_key: "stable-command" };
  await client.goalCommand("test-human-session", "fl/1", "goal/1", "pause", command);
  await client.goalCommand("test-human-session", "fl/1", "goal/1", "pause", command);
  assert.match(calls[0].url, /fl%2F1\/goals\?history=true&limit=20&cursor=history%2B%2Fcursor$/);
  assert.match(calls[1].url, /fl%2F1\/goals\/goal%2F1$/);
  assert.match(calls[2].url, /history\?limit=20&cursor=revision%2Bcursor$/);
  assert.match(calls[3].url, /evidence\?limit=20&cursor=evidence%2Bcursor$/);
  assert.deepEqual(calls[4], calls[5]);
  for (const call of calls) {
    assert.equal(call.headers["x-fluctlight-service-key"], "test-service");
    assert.equal(call.headers["x-fluctlight-human-session"], "test-human-session");
  }
});

test("Core Goal CAS conflicts retain their typed error and details", async () => {
  const client = new CoreClient("http://core.invalid", "test-service", async () => Response.json({ detail: { code: "conflict", message: "stale revision", details: { current_revision: 3 } } }, { status: 409 }));
  await assert.rejects(client.goalCommand("test-human-session", "fl", "goal", "pause", { expected_revision: 1, reason: "wait", idempotency_key: "stable" }), error => {
    assert.ok(error instanceof CoreApiError);
    assert.equal(error.status, 409);
    assert.equal(error.code, "conflict");
    assert.equal(error.details.current_revision, 3);
    return true;
  });
});
