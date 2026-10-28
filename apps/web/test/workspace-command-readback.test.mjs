import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { installDOM } from "./helpers/react-dom.mjs";
test("a received direct-write result permits readback recovery only, fencing repeated admission until the result is readable", async () => {
  const env = installDOM(),
    vite = await createServer({
      root: fileURLToPath(new URL("../", import.meta.url)),
      appType: "custom",
      logLevel: "silent",
      server: { middlewareMode: true },
    });
  let w;
  try {
    const { createWorkspace } = await vite.ssrLoadModule(
        "/src/state/workspace.mjs",
      ),
      store = await vite.ssrLoadModule("/src/v3/proto/store.js");
    w = createWorkspace();
    store.replaceSnapshot({
      ...store.get(),
      session: { actorId: "readback-owner" },
      characters: { A: { id: "A", name: "林青" } },
      ui: { charId: "A" },
    });
    let writes = 0,
      reads = 0;
    const submit = async () => {
        writes++;
        return { item: { id: "created-item" }, revision: 2 };
      },
      onSaved = async (result, value) => {
        assert.deepEqual(value, { description: "原服装" });
        assert.equal(result.item.id, "created-item");
        if (++reads === 1) throw Error("读取暂时断网");
      };
    const record = w.command(
      "衣柜",
      "garment:new",
      { description: "原服装" },
      submit,
      { onSaved },
    );
    await assert.rejects(record.completion, /读取暂时断网/);
    assert.equal(record.readbackPending, true);
    assert.match(record.reason, /只读恢复/);
    assert.equal(
      w.command("衣柜", "garment:new", { description: "后续输入" }, submit, {
        onSaved,
      }),
      record,
    );
    assert.equal(writes, 1);
    await record.run();
    assert.equal(reads, 2);
    assert.equal(writes, 1);
    assert.equal(record.readbackPending, false);
    assert.equal(record.status, "success");
  } finally {
    w?.dispose();
    await vite.close();
    env.restore();
  }
});
test("retrying original A after a newer B attempt acknowledges and displays A, leaving B unsubmitted", async () => {
  const env = installDOM(),
    vite = await createServer({
      root: fileURLToPath(new URL("../", import.meta.url)),
      appType: "custom",
      logLevel: "silent",
      server: { middlewareMode: true },
    });
  let w;
  try {
    const { createWorkspace } = await vite.ssrLoadModule(
        "/src/state/workspace.mjs",
      ),
      store = await vite.ssrLoadModule("/src/v3/proto/store.js");
    sessionStorage.clear();
    w = createWorkspace();
    store.replaceSnapshot({
      ...store.get(),
      session: { actorId: "original-body-owner" },
      characters: { A: { id: "A", name: "林青" } },
      ui: { charId: "A" },
    });
    const calls = [],
      saved = [];
    const submit = async (value, key) => {
      calls.push({ value: JSON.parse(JSON.stringify(value)), key });
      if (calls.length === 1) throw Error("回执未知");
      return { id: "saved-A" };
    };
    const first = w.command("档案", "profile:A", { name: "A" }, submit);
    await assert.rejects(first.completion, /回执未知/);
    const newer = w.command("档案", "profile:A", { name: "B" }, submit, {
      onSaved: async (_result, value) => saved.push(value),
    });
    await assert.rejects(newer.completion, /原提交结果尚未确认/);
    assert.equal(calls.length, 1);
    await newer.run();
    assert.deepEqual(calls[1], calls[0]);
    assert.deepEqual(saved, [{ name: "A" }]);
    assert.match(newer.input, /A/);
    assert.doesNotMatch(newer.input, /B/);
    assert.equal(newer.status, "success");
  } finally {
    w?.dispose();
    await vite.close();
    env.restore();
  }
});
