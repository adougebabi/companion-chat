import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { profileFixture } from "./fixtures/profile-fixture.mjs";
const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
const { createWorkspace } = await vite.ssrLoadModule(
  "/src/state/workspace.mjs",
);
const store = await vite.ssrLoadModule("/src/v3/proto/store.js");
const { createProfileModel } = await vite.ssrLoadModule(
  "/src/react-app/pages/CharacterDetailPage.jsx",
);
const { useController } = await vite.ssrLoadModule("/src/react-app/page.tsx");
const services = await vite.ssrLoadModule("/src/v3/proto/services.js");
const { createInventoryProvider } = await vite.ssrLoadModule(
  "/src/v3/providers/inventory.ts",
);
test.after(async () => {
  await new Promise((r) => setTimeout(r, 20));
  await vite.close();
  env.restore();
});
const button = (root, text) =>
  [...root.querySelectorAll("button")].find(
    (n) => n.textContent.trim() === text,
  );
const dialog = () => [...document.querySelectorAll('[role="dialog"]')].at(-1);
async function tab(v, text, root = v.host) {
  await act(async () =>
    [...root.querySelectorAll('[role="tab"]')]
      .find((n) => n.textContent.trim().startsWith(text))
      .dispatchEvent(
        new window.MouseEvent("mousedown", {
          bubbles: true,
          button: 0,
          ctrlKey: false,
        }),
      ),
  );
  await v.flush();
}
async function fixture({
  scenario = "normal",
  intercept,
  retainStorage = false,
  existing,
} = {}) {
  if (!retainStorage) sessionStorage.clear();
  const api = existing || profileFixture({ owner: "profile-owner", scenario });
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "profile-owner" },
    characters: { A: { id: "A", name: "林青", status: "active" } },
    ui: { charId: "A" },
  });
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, "http://fixture.test"),
      method = init.method || "GET",
      body = init.body ? JSON.parse(init.body) : {};
    const handled = await intercept?.({ url, method, body, api });
    if (handled instanceof Response) return handled;
    let response;
    const ok = await api.handler({
      path: url.pathname,
      url,
      method,
      body,
      json: (_res, value, status = 200) =>
        (response = new Response(JSON.stringify(value), {
          status,
          headers: { "content-type": "application/json" },
        })),
    });
    assert.ok(ok, "Unhandled fixture request " + method + " " + url.pathname);
    return response;
  };
  w.refreshDirectory = async () => {};
  services.configureDomainService("life", {
    request: (_fid, id) => w.client.ownerRequest(id),
    wardrobe: (fid) => w.client.wardrobe(fid),
  });
  services.configureDomainService(
    "inventory",
    createInventoryProvider(w.client),
  );
  let model;
  function Harness() {
    return useController((_props, _events, lifecycle) => {
      model = createProfileModel(lifecycle);
      return model.render;
    }, {});
  }
  const v = await mountReact(vite, Harness, { workspace: w });
  return {
    v,
    w,
    api,
    get c() {
      return model;
    },
    async dispose() {
      await v.dispose();
      w.dispose();
      await new Promise((r) => setTimeout(r, 10));
    },
  };
}
const writes = (f) => f.api.calls.filter((call) => call.method !== "GET");
test("profile uses actual trait scores and separates initial appearance from current wearing", async () => {
  const f = await fixture();
  try {
    assert.equal(
      f.v.host
        .querySelector('[role="progressbar"][aria-label="开放性"]')
        .getAttribute("aria-valuenow"),
      "82",
    );
    assert.match(f.v.host.textContent, /书籍编辑/);
    await tab(f.v, "生活与此刻");
    assert.match(f.v.host.textContent, /齐肩长发/);
    assert.match(f.v.host.textContent, /正式登记的白色衬衫/);
    assert.equal(f.api.state("A").detail.current_state.appearance, null);
    await tab(f.v, "修订与来源");
    assert.match(f.v.host.textContent, /身份修订 v5/);
    assert.ok(
      f.api.calls.some((call) => call.path.endsWith("foundation-revisions")),
    );
  } finally {
    await f.dispose();
  }
});
test("profile discard never mutates; save proposes only changed fields and retains unknown nested fields", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "编辑资料"));
    await f.v.input(
      dialog().querySelector('[aria-label="名字"]'),
      "放弃的名字",
    );
    await f.v.click(button(dialog(), "取消"));
    await f.v.click(
      button(document.querySelector('[role="alertdialog"]'), "放弃修改"),
    );
    assert.equal(writes(f).length, 0);
    await f.v.click(button(f.v.host, "编辑资料"));
    assert.equal(dialog().querySelector('[aria-label="名字"]').value, "林青");
    await f.v.input(
      dialog().querySelector('[aria-label="名字"]'),
      "林青的新名字",
    );
    await f.v.click(button(dialog(), "保存资料"));
    const sent = writes(f);
    assert.equal(sent.length, 2);
    assert.deepEqual(sent[0].body.changes, {
      identity: { name: "林青的新名字" },
    });
    assert.ok(sent[0].body.idempotencyKey);
    assert.equal("idempotencyKey" in sent[1].body, false);
    assert.equal(f.api.state("A").detail.current_revision, 5);
    assert.equal(
      f.api.state("A").detail.identity.custom_field,
      "自定义字段保留",
    );
    assert.deepEqual(
      f.api.state("A").detail.life_profile.extension.numeric_values,
      [1, 2],
    );
    assert.equal(dialog(), undefined);
    assert.match(f.v.host.textContent, /林青的新名字/);
  } finally {
    await f.dispose();
  }
});
test("unknown profile acceptance retries the frozen original while newer edits stay editable", async () => {
  const f = await fixture({ scenario: "accept-unknown" });
  try {
    await f.v.click(button(f.v.host, "编辑资料"));
    await f.v.input(dialog().querySelector('[aria-label="名字"]'), "原提交 A");
    await f.v.click(button(dialog(), "保存资料"));
    assert.equal(f.api.state("A").detail.identity.name, "原提交 A");
    await f.v.input(
      dialog().querySelector('[aria-label="名字"]'),
      "后续草稿 B",
    );
    await f.v.click(button(dialog(), "保存资料"));
    assert.equal(writes(f).length, 2);
    assert.match(dialog().textContent, /原提交结果尚未确认/);
    await act(async () => f.c.last.value.run());
    await f.v.flush();
    assert.deepEqual(writes(f)[2].body, writes(f)[0].body);
    assert.deepEqual(writes(f)[3].body, writes(f)[1].body);
    assert.equal(f.api.state("A").detail.current_revision, 5);
    assert.equal(
      dialog().querySelector('[aria-label="名字"]').value,
      "后续草稿 B",
    );
    assert.match(f.c.last.value.input, /原提交 A/);
    assert.doesNotMatch(f.c.last.value.input, /后续草稿 B/);
    assert.equal(f.c.pendingProfile.value, null);
    await f.v.click(button(dialog(), "保存资料"));
    assert.equal(f.api.state("A").detail.identity.name, "后续草稿 B");
    assert.equal(f.api.state("A").detail.current_revision, 6);
  } finally {
    await f.dispose();
  }
});
test("acknowledged profile save with failed readback recovers through reads only", async () => {
  const f = await fixture({ scenario: "readback-failure" });
  try {
    await f.v.click(button(f.v.host, "编辑资料"));
    await f.v.input(
      dialog().querySelector('[aria-label="名字"]'),
      "只读恢复后的名字",
    );
    await f.v.click(button(dialog(), "保存资料"));
    assert.equal(writes(f).length, 2);
    assert.ok(f.c.readbackPending.value);
    await f.v.click(button(dialog(), "重新读取已保存资料"));
    assert.equal(writes(f).length, 2);
    assert.equal(dialog(), undefined);
    assert.equal(f.c.snapshot.value.foundationRevision, 5);
  } finally {
    await f.dispose();
  }
});
test("unknown profile save survives remount and replays the original proposal and acceptance", async () => {
  const first = await fixture({ scenario: "accept-unknown" });
  await first.v.click(button(first.v.host, "编辑资料"));
  await first.v.input(
    dialog().querySelector('[aria-label="名字"]'),
    "跨刷新原保存",
  );
  await first.v.click(button(dialog(), "保存资料"));
  const api = first.api,
    original = writes(first).map((call) => call.body);
  await first.dispose();
  const next = await fixture({ existing: api, retainStorage: true });
  try {
    assert.ok(button(next.v.host, "核对或重试原资料保存"));
    await next.v.click(button(next.v.host, "核对或重试原资料保存"));
    assert.deepEqual(
      writes(next)
        .slice(2)
        .map((call) => call.body),
      original,
    );
    assert.equal(api.state("A").detail.current_revision, 5);
    assert.equal(dialog(), undefined);
    assert.equal(next.c.pendingProfile.value, null);
  } finally {
    await next.dispose();
  }
});
test("unread profile stays unknown; first failure offers reread and confirmed empty profile has independent empty panels", async () => {
  let release;
  const gate = new Promise((r) => (release = r));
  let first = true;
  const f = await fixture({
    intercept: async ({ url }) => {
      if (first && url.pathname.endsWith("/detail")) {
        first = false;
        await gate;
      }
    },
  });
  try {
    assert.match(f.v.host.textContent, /正在读取角色资料/);
    assert.doesNotMatch(f.v.host.textContent, /还没有提供简介/);
    release();
    await f.v.flush();
    assert.match(f.v.host.textContent, /书籍编辑/);
  } finally {
    release();
    await f.dispose();
  }
  const failed = await fixture({ scenario: "failure" });
  try {
    assert.match(failed.v.host.textContent, /服务暂时无法连接/);
    await failed.v.click(button(failed.v.host, "重新读取角色资料"));
    assert.match(failed.v.host.textContent, /书籍编辑/);
  } finally {
    await failed.dispose();
  }
  const empty = await fixture({ scenario: "empty" });
  try {
    assert.match(empty.v.host.textContent, /当前尚未提供可读取的性格维度/);
    await tab(empty.v, "性格与人格");
    assert.match(empty.v.host.textContent, /尚未登记人格档案/);
    await tab(empty.v, "修订与来源");
    assert.match(empty.v.host.textContent, /尚未读到资料修订记录/);
  } finally {
    await empty.dispose();
  }
});
test("pending persona requires a terminal receipt and matching versions; an older readback cannot prove success", async () => {
  const f = await fixture();
  try {
    await act(async () => {
      f.c.target.value = "focused";
      f.c.reason.value = "专注讨论";
      f.c.confirm.value = true;
      await f.c.switchPersona();
    });
    const id = f.c.personaRequestId.value;
    assert.ok(id);
    assert.ok(f.c.personaReadbackPending.value);
    assert.equal(f.c.snapshot.value.activeProfileId, "sunny");
    await tab(f.v, "性格与人格");
    assert.match(f.v.host.textContent, /人格切换请求已受理/);
    assert.doesNotMatch(f.v.host.textContent, /换装请求已受理/);
    let terminal;
    for (let i = 0; i < 3; i++) terminal = await f.w.client.ownerRequest(id);
    f.api.state("A").lagRuntime = { active_profile_id: "sunny", revision: 2 };
    await act(async () => f.c.readPersonaOutcome(terminal));
    await f.v.flush();
    assert.ok(f.c.personaReadbackPending.value);
    assert.doesNotMatch(f.c.personaFeedback.value, /已生效/);
    f.api.state("A").lagRuntime = null;
    await act(async () => f.c.readPersonaOutcome());
    await f.v.flush();
    assert.equal(f.c.personaReadbackPending.value, false);
    assert.equal(f.c.snapshot.value.activeProfileId, "focused");
    assert.match(f.c.personaFeedback.value, /已生效/);
    assert.equal(
      writes(f).filter((c) => c.path.endsWith("persona-switch")).length,
      1,
    );
  } finally {
    await f.dispose();
  }
});
test("rejected persona unlocks after reading current state without calling the rejected request successful", async () => {
  const f = await fixture({ scenario: "switch-rejected" });
  try {
    await act(async () => {
      f.c.target.value = "focused";
      f.c.reason.value = "保留原因";
      f.c.confirm.value = true;
      await f.c.switchPersona();
    });
    const id = f.c.personaRequestId.value;
    let terminal;
    for (let i = 0; i < 3; i++) terminal = await f.w.client.ownerRequest(id);
    f.api.state("A").detail.personality_runtime = {
      active_profile_id: "focused",
      revision: 3,
    };
    await act(async () => f.c.readPersonaOutcome(terminal));
    await f.v.flush();
    assert.equal(f.c.personaReadbackPending.value, false);
    assert.equal(f.c.reason.value, "保留原因");
    assert.equal(f.c.target.value, "focused");
    assert.equal(f.c.confirm.value, false);
    assert.match(f.c.personaFeedback.value, /未被采纳/);
    assert.doesNotMatch(f.c.personaFeedback.value, /已生效/);
  } finally {
    await f.dispose();
  }
});
test("unknown persona remount resumes original key and body while preserving a newer reason", async () => {
  const first = await fixture({ scenario: "switch-unknown" });
  await act(async () => {
    first.c.target.value = "focused";
    first.c.reason.value = "原切换原因";
    first.c.confirm.value = true;
    await first.c.switchPersona();
    first.c.reason.value = "后续未提交的原因";
  });
  await first.v.flush();
  const api = first.api,
    original = writes(first).find((c) => c.path.endsWith("persona-switch"));
  await first.dispose();
  const next = await fixture({ existing: api, retainStorage: true });
  try {
    assert.equal(next.c.reason.value, "后续未提交的原因");
    assert.ok(next.c.pendingPersona.value);
    await act(async () => next.c.resumePersona());
    await next.v.flush();
    const sent = writes(next).filter((c) => c.path.endsWith("persona-switch"));
    assert.equal(sent.length, 2);
    assert.deepEqual(sent[1].body, original.body);
    assert.equal(next.c.reason.value, "后续未提交的原因");
  } finally {
    await next.dispose();
  }
});
test("foundation and capability decisions use their own revisions and historical restoration appends a new foundation version", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "身份与能力"));
    await f.v.click(button(dialog(), "接受这条提议"));
    await f.v.input(
      dialog().querySelector('[aria-label="身份提议操作说明"]'),
      "确认生活习惯",
    );
    await f.v.click(button(dialog(), "接受并应用"));
    assert.equal(f.api.state("A").detail.current_revision, 5);
    const accept = writes(f).at(-1);
    assert.equal(accept.body.expectedRevision, 4);
    assert.equal("idempotencyKey" in accept.body, false);
    await f.v.click(button(dialog(), "恢复此版身份内容"));
    await f.v.input(
      dialog().querySelector('[aria-label="身份提议操作说明"]'),
      "恢复初始内容",
    );
    await f.v.click(button(dialog(), "确认历史恢复"));
    assert.equal(f.api.state("A").detail.current_revision, 6);
    assert.equal(
      f.api.state("A").detail.life_profile.life_habits,
      "睡前阅读。",
    );
    await tab(f.v, "能力申请", dialog());
    await f.v.click(button(dialog(), "查看与审核"));
    await f.v.click(button(dialog(), "接受需求"));
    await f.v.input(
      dialog().querySelector('[aria-label="能力审核说明"]'),
      "需求有明确用途",
    );
    await f.v.click(button(dialog(), "接受需求"));
    const accepted = writes(f).at(-1);
    assert.equal(accepted.body.expectedRevision, 1);
    assert.ok(accepted.body.idempotencyKey);
    await f.v.click(button(dialog(), "记录已交付实现"));
    assert.equal(
      dialog().querySelector('[aria-label="能力实现版本"]').value,
      "1.0-fixture",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="能力审核说明"]'),
      "核对实际实现版本",
    );
    await f.v.click(button(dialog(), "记录已交付实现"));
    const fulfilled = writes(f).at(-1);
    assert.equal(fulfilled.body.expectedRevision, 2);
    assert.equal(fulfilled.body.capabilityVersion, "1.0-fixture");
    assert.equal(f.api.state("A").capabilities[0].status, "fulfilled");
  } finally {
    await f.dispose();
  }
});
test("refreshing an open capability request adopts external decisions and refreshes its history", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "身份与能力"));
    await tab(f.v, "能力申请", dialog());
    await f.v.click(button(dialog(), "查看与审核"));
    const s = f.api.state("A"),
      row = s.capabilities[0];
    row.status = "accepted";
    row.revision = 2;
    s.histories.set(row.id, [
      {
        id: "external-review",
        note: "另一会话已接受",
        status: "accepted",
        created_at: "2026-10-27T00:00:00Z",
      },
    ]);
    await f.v.click(button(dialog(), "刷新能力申请"));
    assert.ok(button(dialog(), "记录已交付实现"));
    assert.equal(button(dialog(), "接受需求"), undefined);
    const reads = f.api.calls.filter((call) =>
      call.path.endsWith("/" + row.id + "/history"),
    );
    assert.ok(reads.length >= 2);
    assert.equal(writes(f).length, 0);
  } finally {
    await f.dispose();
  }
});
