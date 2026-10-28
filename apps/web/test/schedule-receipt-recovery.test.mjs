import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
test("malformed or foreign schedule receipts preserve the original body and idempotency through provider recreation", async () => {
  const old = { sessionStorage: globalThis.sessionStorage };
  const saved = new Map();
  globalThis.sessionStorage = {
    getItem: (key) => saved.get(key) ?? null,
    setItem: (key, value) => saved.set(key, String(value)),
    removeItem: (key) => saved.delete(key),
  };
  const vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
  try {
    const { createLifeProvider } = await vite.ssrLoadModule(
        "/src/v3/providers/life.ts",
      ),
      store = await vite.ssrLoadModule("/src/v3/proto/store.js");
    store.replaceSnapshot({
      ...store.get(),
      session: { actorId: "owner" },
      characters: { A: { id: "A", status: "正常" } },
    });
    const calls = [];
    let itemReads = 0,
      planReads = 0;
    const client = {
      suggestScheduleItem: async (fid, id, body) => {
        calls.push(["item", structuredClone(body)]);
        return ++itemReads === 1
          ? { request: { id: "bad-without-scope" } }
          : {
              request: {
                id: "same-request",
                fluctlight_id: "A",
                status: "pending",
              },
            };
      },
      requestSchedule: async (fid, body) => {
        calls.push(["plan", structuredClone(body)]);
        return {
          request_id: "plan",
          fluctlight_id: ++planReads === 1 ? "other-role" : "A",
          status: "queued",
        };
      },
    };
    const original = createLifeProvider(client),
      detail = {
        schedule: { id: "schedule", revision: 1 },
        context: { context_revision: "context-one" },
      };
    await assert.rejects(
      original.scheduleItem(
        "A",
        detail,
        null,
        { activity: "原活动", reason: "原理由" },
        "create",
      ),
      /原提交身份已保留/,
    );
    const key = original.scheduleItemPending("A").idempotencyKey;
    const restored = createLifeProvider(client);
    await restored.retryScheduleItem("A");
    assert.equal(calls[1][1].idempotencyKey, key);
    assert.deepEqual(calls[1], calls[0]);
    assert.equal(restored.scheduleItemPending("A"), null);
    const plan = {
      localDate: "2026-10-29",
      reason: "原规划",
      expectedRevision: 0,
      expectedLifeContextRevision: "context-one",
    };
    await assert.rejects(
      original.submitSchedule("A", plan),
      /原提交身份已保留/,
    );
    const resumed = createLifeProvider(client);
    await resumed.retrySchedule("A", plan.localDate);
    assert.deepEqual(calls[3], calls[2]);
    assert.equal(resumed.schedulePending("A", plan.localDate), null);
  } finally {
    await vite.close();
    Object.assign(globalThis, old);
  }
});
test("plan rebase is limited to a first proven CAS rejection and preserves date/reason while refreshing only the reviewed versions", async () => {
  const old = { sessionStorage: globalThis.sessionStorage };
  const saved = new Map();
  globalThis.sessionStorage = {
    getItem: (key) => saved.get(key) ?? null,
    setItem: (key, value) => saved.set(key, String(value)),
    removeItem: (key) => saved.delete(key),
  };
  const vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
  try {
    const { createLifeProvider } = await vite.ssrLoadModule(
        "/src/v3/providers/life.ts",
      ),
      store = await vite.ssrLoadModule("/src/v3/proto/store.js");
    store.replaceSnapshot({
      ...store.get(),
      session: { actorId: "owner" },
      characters: { A: { id: "A", status: "正常" } },
    });
    const calls = [];
    let mode = "reject";
    const client = {
      requestSchedule: async (fid, body) => {
        calls.push(structuredClone(body));
        if (mode === "reject")
          throw Object.assign(Error("version"), {
            status: 409,
            code: "revision_conflict",
          });
        if (mode === "unknown")
          throw Object.assign(Error("transport"), { status: 503 });
        return {
          request_id: "confirmed-plan",
          fluctlight_id: "A",
          status: "queued",
        };
      },
    };
    const life = createLifeProvider(client),
      body = {
        localDate: "2026-10-29",
        reason: "保留原说明",
        expectedRevision: 0,
        expectedLifeContextRevision: "old-context",
      },
      calendar = {
        timezone: "Asia/Shanghai",
        context_revision: "reviewed-context",
        days: [{ local_date: body.localDate, schedule: null }],
      };
    await assert.rejects(life.submitSchedule("A", body));
    const key = life.schedulePending("A", body.localDate).idempotencyKey;
    await assert.rejects(
      life.rebaseSchedule(
        "A",
        body.localDate,
        {
          ...calendar,
          days: [
            {
              local_date: body.localDate,
              schedule: { id: "protected", items: [] },
            },
          ],
        },
        key,
      ),
      /已有日程记录/,
    );
    assert.equal(calls.length, 1);
    mode = "ok";
    await life.rebaseSchedule("A", body.localDate, calendar, key);
    assert.notEqual(calls[1].idempotencyKey, calls[0].idempotencyKey);
    assert.equal(calls[1].localDate, body.localDate);
    assert.equal(calls[1].reason, body.reason);
    assert.equal(calls[1].expectedLifeContextRevision, "reviewed-context");
    mode = "unknown";
    await assert.rejects(life.submitSchedule("A", body));
    mode = "reject";
    await assert.rejects(life.retrySchedule("A", body.localDate));
    const unknown = life.schedulePending("A", body.localDate);
    assert.equal(unknown.rejection, undefined);
    await assert.rejects(
      life.rebaseSchedule(
        "A",
        body.localDate,
        calendar,
        unknown.idempotencyKey,
      ),
      /原规划的状态已经变化/,
    );
    assert.throws(
      () => life.releaseSchedule("A", body.localDate),
      /尚未确认无效果/,
    );
  } finally {
    await vite.close();
    Object.assign(globalThis, old);
  }
});
