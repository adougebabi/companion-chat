import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { React, act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { studioFixture } from "./fixtures/studio-fixture.mjs";
const env = installDOM(),
  vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
const { createWorkspace } = await vite.ssrLoadModule(
    "/src/state/workspace.mjs",
  ),
  store = await vite.ssrLoadModule("/src/v3/proto/store.js"),
  { installStudioProvider } = await vite.ssrLoadModule(
    "/src/v3/providers/studio.ts",
  ),
  { default: Directory } = await vite.ssrLoadModule(
    "/src/react-app/pages/StudioDirectory.jsx",
  ),
  { default: Editor } = await vite.ssrLoadModule(
    "/src/react-app/pages/StudioEditorPage.tsx",
  ),
  { default: Jobs } = await vite.ssrLoadModule(
    "/src/react-app/shared/StudioJobs.tsx",
  ),
  { default: Lifecycle } = await vite.ssrLoadModule(
    "/src/react-app/shared/StudioLifecycle.tsx",
  );
const { default: OverlayHost } = await vite.ssrLoadModule(
  "/src/react-app/shared/OverlayHost.tsx",
);
test.after(async () => {
  await vite.close();
  env.restore();
});
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll('[role="dialog"]')].at(-1);
async function fixture({
  scenario = "normal",
  api = studioFixture({ scenario }),
  Component = Directory,
  props = {},
  retainStorage = false,
  intercept,
} = {}) {
  if (!retainStorage) {
    sessionStorage.clear();
    localStorage.clear();
  }
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "studio-owner" },
    characters: { A: { id: "A", name: "林青", status: "active" } },
    ui: { charId: "A" },
  });
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, "http://studio.test"),
      method = init.method || "GET",
      body = init.body ? JSON.parse(init.body) : {};
    const overridden = await intercept?.({ url, method, body, api });
    if (overridden instanceof Response) return overridden;
    let response;
    const handled = await api.handler({
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
    assert.ok(handled, "Unhandled " + method + " " + url.pathname);
    return response;
  };
  installStudioProvider(w.client);
  const moves = [];
  w.navigatePath = (p) => moves.push(p);
  const Wrapped = () =>
    React.createElement(
      React.Fragment,
      null,
      React.createElement(Component, props),
      React.createElement(OverlayHost),
    );
  const v = await mountReact(vite, Wrapped, { workspace: w });
  return {
    api,
    w,
    v,
    moves,
    writes: () =>
      api.calls.filter(
        (c) => !["GET"].includes(c.method) && !c.path.endsWith("apply-preview"),
      ),
    async dispose() {
      await v.dispose();
      w.dispose();
      await new Promise((r) => setTimeout(r, 10));
    },
  };
}
async function choose(f, label, text) {
  await f.v.click(
    document.querySelector('[role=combobox][aria-label="' + label + '"]'),
  );
  await f.v.click(
    [...document.querySelectorAll("[role=option]")].find(
      (n) => n.textContent.trim() === text,
    ),
  );
}
test("Studio directory reads nine actual resource types, searches and keeps archived collection separate", async () => {
  const f = await fixture();
  try {
    assert.equal(f.v.host.querySelectorAll(".r-resource-card").length, 9);
    await f.v.input(
      f.v.host.querySelector('[aria-label="搜索资源名称"]'),
      "晨雾",
    );
    await f.v.click(button(f.v.host, "查询"));
    assert.equal(f.v.host.querySelectorAll(".r-resource-card").length, 1);
    await f.v.click(f.v.host.querySelector('[aria-label="打开资源 晨雾小镇"]'));
    assert.deepEqual(f.moves, ["/studio/edit/studio-world"]);
    assert.equal(f.writes().length, 0);
  } finally {
    await f.dispose();
  }
});
test("Studio empty directory and failed initial read are distinct and recover via original GET", async () => {
  const empty = await fixture({ scenario: "empty" });
  try {
    assert.match(empty.v.host.textContent, /没有找到相关资源/);
    assert.equal(empty.writes().length, 0);
  } finally {
    await empty.dispose();
  }
  const failed = await fixture({ scenario: "failure" });
  try {
    assert.match(failed.v.host.textContent, /服务暂时无法连接/);
    assert.doesNotMatch(failed.v.host.textContent, /没有找到相关资源/);
    await failed.v.click(button(failed.v.host, "重新读取资源"));
    assert.equal(failed.v.host.querySelectorAll(".r-resource-card").length, 9);
  } finally {
    await failed.dispose();
  }
});
test("Studio saves typed edits using the opaque draft version and publishes an exact immutable saved version", async () => {
  const f = await fixture({
    Component: Editor,
    props: { resourceId: "studio-world" },
  });
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="世界背景"]'),
      "保留河岸与完整扩展",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="保存原因"]'),
      "调整背景",
    );
    assert.equal(button(f.v.host, "发布已保存版本").disabled, true);
    await f.v.click(button(f.v.host, "保存草稿"));
    const save = f.writes().find((c) => c.method === "PUT");
    assert.equal(save.body.expectedVersion, "draft:1");
    assert.deepEqual(save.body.body.custom_preserved, { value: false });
    assert.equal(
      f.api.resources.get("studio-world").published_version,
      "draft:1",
    );
    await f.v.click(button(f.v.host, "发布已保存版本"));
    await f.v.click(button(dialog(), "确认发布保存版本"));
    assert.equal(
      f.api.resources.get("studio-world").published_version,
      "draft:2",
    );
    assert.match(f.v.host.textContent, /已发布确认/);
    assert.equal(
      f.api.publications.get("studio-world:draft:1").body.background,
      "沿着河流生活的小镇，书店与花园在清晨醒来。",
    );
    await f.v.click(button(f.v.host, "版本历史"));
    const versions = [...dialog().querySelectorAll(".r-version-row")];
    assert.ok(versions.some((row) => row.textContent.includes("draft:2")));
    assert.ok(versions.some((row) => row.textContent.includes("draft:1")));
  } finally {
    await f.dispose();
  }
});
test("Studio new save checkpoints the actual identity and canonical name before navigation", async () => {
  const f = await fixture({
    Component: Editor,
    props: { resourceId: "new", kind: "npc" },
  });
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="资源名称"]'),
      " 花匠 ",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="身份 / 职业"]'),
      "园丁",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="保存原因"]'),
      "保存新人物",
    );
    await f.v.click(button(f.v.host, "保存草稿"));
    assert.match(f.moves[0], /studio-created-/);
    const id = f.moves[0].split("/").at(-1),
      d = JSON.parse(
        sessionStorage.getItem(`fluctlight-studio:studio-owner:${id}:draft`),
      );
    assert.equal(d.name, "花匠");
    assert.equal(f.api.resources.get(id).status, "draft");
    assert.equal(f.api.resources.get(id).body.role, "园丁");
  } finally {
    await f.dispose();
  }
});
test("Studio apply freezes the exact publication and generic accepted receipt does not require a fabricated target kind", async () => {
  const f = await fixture({
    Component: Editor,
    props: { resourceId: "studio-world" },
  });
  try {
    await f.v.click(button(f.v.host, "应用到角色"));
    await choose(f, "目标角色", "林青");
    await f.v.click(button(dialog(), "读取应用差异"));
    assert.match(dialog().textContent, /采用发布内容/);
    await f.v.click(button(dialog(), "确认应用所选字段"));
    assert.match(dialog().textContent, /原应用回执已核对/);
    const write = f.writes().find((c) => c.path.endsWith("/apply"));
    assert.equal(write.body.resourceVersion, "draft:1");
    assert.equal(write.body.fluctlightId, "A");
    assert.equal(write.body.expectedWorldRevision, 1);
    assert.equal(write.body.expectedRuntimeRevision, 1);
  } finally {
    await f.dispose();
  }
});
test("Studio version history reads immutable content, while independent copies stay unpublished", async () => {
  const f = await fixture({
    Component: Editor,
    props: { resourceId: "studio-world" },
  });
  try {
    await f.v.click(button(f.v.host, "版本历史"));
    assert.match(dialog().textContent, /draft:1/);
    await f.v.click(dialog().querySelector('[data-slot="dialog-close"]'));
    await f.v.click(button(f.v.host, "创建独立副本"));
    await f.v.input(dialog().querySelector("#copy-resource-name"), "独立小镇");
    await f.v.input(
      dialog().querySelector("#copy-resource-reason"),
      "独立使用",
    );
    await f.v.click(button(dialog(), "创建副本"));
    const row = [...f.api.resources.values()].find(
      (r) => r.name === "独立小镇",
    );
    assert.ok(row);
    assert.equal(row.status, "draft");
    assert.equal(row.published_version, null);
    assert.deepEqual(row.referenced_by, []);
  } finally {
    await f.dispose();
  }
});
test("sandbox freezes current working input, observes actual jobs and adopts suggestions only after explicit review", async () => {
  const api = studioFixture(),
    resource = api.resources.get("studio-world"),
    draft = {
      name: "未保存名称",
      body: { background: "未保存草稿", extra: false },
    };
  let adopted;
  const f = await fixture({
    api,
    Component: Jobs,
    props: {
      resource,
      readDraft: () => draft,
      adopt: (value) => (adopted = value),
    },
  });
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="创作请求"]'),
      "补全花园",
    );
    await f.v.click(button(f.v.host, "开始创作"));
    const j = [...api.jobs.values()][0];
    assert.equal(j.snapshot.name, draft.name);
    assert.deepEqual(j.snapshot.body, draft.body);
    assert.match(f.v.host.textContent, /正在等待实际执行/);
    api.completeJob(j);
    await f.v.click(button(f.v.host, "刷新任务状态"));
    assert.equal(adopted, undefined);
    await f.v.click(button(f.v.host, "核对并采用到草稿"));
    await f.v.click(
      button(document.querySelector("[role=alertdialog]"), "采用到草稿"),
    );
    assert.equal(adopted.body.extra, false);
    assert.match(adopted.body.background, /花园/);
    assert.equal(
      api.resources.get(resource.id).body.custom_preserved.value,
      false,
    );
  } finally {
    await f.dispose();
  }
});
test("acknowledged sandbox readback failure survives remount and recovers with GET only while preserving newer composer text", async () => {
  const api = studioFixture(),
    resource = api.resources.get("studio-world"),
    draft = { name: "工作名称", body: { background: "工作内容" } };
  let fail = true;
  const intercept = ({ url, method }) => {
    if (
      method === "GET" &&
      url.pathname.startsWith("/api/studio/jobs/") &&
      fail
    )
      return new Response('{"code":"unavailable"}', { status: 503 });
  };
  let f = await fixture({
    api,
    Component: Jobs,
    props: { resource, readDraft: () => draft },
    intercept,
  });
  await f.v.input(f.v.host.querySelector('[aria-label="创作请求"]'), "原指令");
  await f.v.click(button(f.v.host, "开始创作"));
  await f.v.input(
    f.v.host.querySelector('[aria-label="创作请求"]'),
    "后来输入",
  );
  assert.ok(
    sessionStorage.getItem(
      "fluctlight-studio:studio-owner:studio-world:jobs:receipt",
    ),
  );
  assert.equal(api.jobs.size, 1);
  await f.dispose();
  fail = false;
  f = await fixture({
    api,
    Component: Jobs,
    props: { resource, readDraft: () => draft },
    retainStorage: true,
    intercept,
  });
  try {
    await f.v.flush();
    assert.equal(
      f.v.host.querySelector('[aria-label="创作请求"]').value,
      "后来输入",
    );
    assert.equal(
      api.calls.filter(
        (c) => c.path === "/api/studio/jobs" && c.method === "POST",
      ).length,
      1,
    );
    assert.equal(
      sessionStorage.getItem(
        "fluctlight-studio:studio-owner:studio-world:jobs:receipt",
      ),
      null,
    );
  } finally {
    await f.dispose();
  }
});
test("late sandbox acceptance after unmount retains its pointer", async () => {
  const api = studioFixture(),
    resource = api.resources.get("studio-world"),
    draft = { name: "名称", body: { background: "背景" } };
  let resolveWrite, entered;
  const started = new Promise((r) => (entered = r)),
    intercept = async ({ url, method }) => {
      if (method === "POST" && url.pathname === "/api/studio/jobs") {
        entered();
        await new Promise((r) => (resolveWrite = r));
      }
    };
  const f = await fixture({
    api,
    Component: Jobs,
    props: { resource, readDraft: () => draft },
    intercept,
  });
  await f.v.input(f.v.host.querySelector('[aria-label="创作请求"]'), "原指令");
  await f.v.click(button(f.v.host, "开始创作"));
  await started;
  await f.v.dispose();
  resolveWrite();
  await act(async () => {
    await new Promise((r) => setTimeout(r, 20));
  });
  await f.v.flush();
  assert.ok(
    sessionStorage.getItem(
      "fluctlight-studio:studio-owner:studio-world:jobs:receipt",
    ),
  );
  f.w.dispose();
});
test("legacy directory recovery replays the original localStorage identity and body", async () => {
  const api = studioFixture(),
    key = "approved-studio:studio-owner:directory-command";
  sessionStorage.clear();
  localStorage.clear();
  localStorage.setItem(
    key,
    JSON.stringify({
      version: 1,
      idempotencyKey: "legacy-original",
      attempts: 1,
      value: {
        operation: "archive",
        id: "studio-world",
        revision: 1,
        reason: "原归档原因",
      },
    }),
  );
  const f = await fixture({ api, retainStorage: true });
  try {
    await f.v.click(button(f.v.host, "继续核对旧版原提交"));
    await f.v.click(button(dialog(), "核对或重试原提交"));
    const write = f.writes().find((c) => c.method === "DELETE");
    assert.deepEqual(write.body, {
      expectedRevision: 1,
      reason: "原归档原因",
      idempotencyKey: "legacy-original",
    });
    assert.equal(localStorage.getItem(key), null);
    assert.equal(f.api.resources.get("studio-world").status, "archived");
  } finally {
    await f.dispose();
  }
});

test("raw JSON edits cannot be silently replaced by an assistant proposal, including edits made while confirmation is open", async () => {
  const api = studioFixture(),
    resource = api.resources.get("studio-world"),
    draft = { name: "名称", body: { background: "原稿" } };
  let adopted;
  const f = await fixture({
    api,
    Component: Jobs,
    props: { resource, readDraft: () => draft, adopt: (v) => (adopted = v) },
  });
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="创作请求"]'),
      "丰富草稿",
    );
    await f.v.click(button(f.v.host, "开始创作"));
    api.completeJob([...api.jobs.values()][0]);
    await f.v.click(button(f.v.host, "刷新任务状态"));
    draft.bodyText = '{"background":"未核对的新输入"}';
    await f.v.click(button(f.v.host, "核对并采用到草稿"));
    assert.match(f.v.host.textContent, /请先核对完整 JSON/);
    assert.equal(adopted, undefined);
    assert.equal(draft.bodyText, '{"background":"未核对的新输入"}');
    delete draft.bodyText;
    await f.v.click(button(f.v.host, "核对并采用到草稿"));
    draft.bodyText = '{"background":"确认期间的新输入"}';
    await f.v.click(
      button(document.querySelector("[role=alertdialog]"), "采用到草稿"),
    );
    assert.equal(adopted, undefined);
    assert.match(f.v.host.textContent, /草稿或建议已经变化/);
    assert.match(draft.bodyText, /确认期间/);
  } finally {
    await f.dispose();
  }
});

test("event scope changes remove only the invalid place reference while preserving unknown extensions", async () => {
  const { default: Fields } = await vite.ssrLoadModule(
    "/src/react-app/shared/StudioFields.jsx",
  );
  let changed;
  const original = {
    schema_version: "world-event.v1",
    description: "小雨",
    trigger: { mode: "manual" },
    condition: { require_presence: false },
    effects: [
      {
        kind: "weather",
        scope: "place",
        place_id: "place-1",
        value: "rain",
        custom: { keep: false },
      },
    ],
  };
  const f = await fixture({
    Component: Fields,
    props: {
      kind: "event_template",
      value: original,
      onChange: (v) => (changed = v),
    },
  });
  try {
    await f.v.click(button(f.v.host, "可能结果与范围"));
    await choose(f, "效果范围1", "整个应用世界");
    assert.equal(changed.effects[0].scope, "world");
    assert.equal(Object.hasOwn(changed.effects[0], "place_id"), false);
    assert.deepEqual(changed.effects[0].custom, { keep: false });
    assert.equal(original.effects[0].place_id, "place-1");
  } finally {
    await f.dispose();
  }
});

test("an Owner purge prevents a late accepted sandbox task from recreating another Owners recovery pointer", async () => {
  const api = studioFixture(),
    resource = api.resources.get("studio-world"),
    draft = { name: "名称", body: { background: "背景" } };
  let release;
  const f = await fixture({
    api,
    Component: Jobs,
    props: { resource, readDraft: () => draft },
    intercept: async ({ url, method }) => {
      if (method === "POST" && url.pathname === "/api/studio/jobs")
        await new Promise((r) => (release = r));
    },
  });
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="创作请求"]'),
      "原指令",
    );
    await f.v.click(button(f.v.host, "开始创作"));
    assert.ok(release);
    await act(async () =>
      store.replaceSnapshot({
        ...store.get(),
        session: { actorId: "other-owner" },
      }),
    );
    assert.equal(
      sessionStorage.getItem(
        "fluctlight-studio:studio-owner:studio-world:jobs:command",
      ),
      null,
    );
    release();
    await f.v.flush();
    assert.equal(
      sessionStorage.getItem(
        "fluctlight-studio:studio-owner:studio-world:jobs:receipt",
      ),
      null,
    );
  } finally {
    await f.dispose();
  }
});

test("a late independent-copy acknowledgement after unmount preserves its actual copied identity", async () => {
  const api = studioFixture(),
    resource = api.resources.get("studio-world");
  let release;
  const f = await fixture({
    api,
    Component: Lifecycle,
    props: { resource, onChanged: () => {} },
    intercept: async ({ url, method }) => {
      if (method === "POST" && url.pathname.endsWith("/copy"))
        await new Promise((r) => (release = r));
    },
  });
  await f.v.click(button(f.v.host, "创建独立副本"));
  await f.v.input(dialog().querySelector("#copy-resource-reason"), "保留副本");
  await f.v.click(button(dialog(), "创建副本"));
  assert.ok(release);
  await f.v.dispose();
  release();
  await act(async () => {
    await new Promise((r) => setTimeout(r, 20));
  });
  const pointer = JSON.parse(
    sessionStorage.getItem(
      "fluctlight-studio:studio-owner:studio-world:lifecycle-receipt",
    ),
  );
  assert.ok(api.resources.has(pointer.targetId));
  assert.equal(pointer.operation, "copy");
  assert.equal(
    sessionStorage.getItem(
      "fluctlight-studio:studio-owner:studio-world:lifecycle",
    ),
    null,
  );
  f.w.dispose();
});

test("copying a historical publication uses its exact immutable definition and restores dialog source focus on cancel", async () => {
  const api = studioFixture(),
    row = api.resources.get("studio-world"),
    old = structuredClone(row.body);
  row.body.background = "较新的已发布背景";
  row.draft_version = "draft:2";
  row.published_version = "draft:2";
  row.revision = 3;
  api.publications.set(row.id + ":draft:2", {
    id: row.id,
    version: "draft:2",
    kind: row.kind,
    name: row.name,
    body: structuredClone(row.body),
    immutable: true,
  });
  const f = await fixture({
    api,
    Component: Editor,
    props: { resourceId: row.id },
  });
  try {
    const source = button(f.v.host, "版本历史");
    await f.v.click(source);
    await f.v.click(
      [...dialog().querySelectorAll(".r-version-row")].find((n) =>
        n.textContent.includes("draft:1"),
      ),
    );
    await f.v.click(button(dialog(), "以此版本创建独立副本"));
    await f.v.input(dialog().querySelector("#copy-resource-name"), "历史副本");
    await f.v.input(
      dialog().querySelector("#copy-resource-reason"),
      "复用历史定义",
    );
    await f.v.click(button(dialog(), "创建副本"));
    const write = f.writes().find((c) => c.path.endsWith("/copy"));
    assert.equal(write.body.source, "published");
    assert.equal(write.body.sourceVersion, "draft:1");
    assert.deepEqual(
      [...api.resources.values()].find((r) => r.name === "历史副本").body,
      old,
    );
    await f.v.click(button(f.v.host, "应用到角色"));
    await f.v.click(button(dialog(), "取消"));
    assert.equal(document.activeElement, button(f.v.host, "应用到角色"));
  } finally {
    await f.dispose();
  }
});

test("sandbox text materials retain their original job and release only the resource association", async () => {
  const api = studioFixture(),
    resource = api.resources.get("studio-world");
  const f = await fixture({
    api,
    Component: Jobs,
    props: {
      resource,
      readDraft: () => ({ name: resource.name, body: resource.body }),
      initialKind: "text_preview",
    },
  });
  try {
    await choose(f, "创作方式", "文本预览");
    await f.v.input(
      f.v.host.querySelector('[aria-label="创作请求"]'),
      "写一段窗边的故事",
    );
    await f.v.click(button(f.v.host, "开始创作"));
    const j = [...api.jobs.values()][0];
    api.completeJob(j);
    await f.v.click(button(f.v.host, "刷新任务状态"));
    await f.v.click(button(f.v.host, "存为资源素材"));
    const m = [...api.materials.values()][0];
    assert.equal(m.job_id, j.id);
    assert.match(m.text, /窗边/);
    await f.v.click(button(f.v.host, "取消保留"));
    await f.v.input(
      dialog().querySelector('[aria-label="修改原因"]'),
      "暂不作为资源素材",
    );
    await f.v.click(button(dialog(), "确认提交"));
    assert.ok(m.released_at);
    assert.equal(api.jobs.get(j.id).status, "completed");
    assert.ok(api.resources.has(resource.id));
    assert.equal(
      f.writes().filter((c) => c.path.endsWith("/release")).length,
      1,
    );
  } finally {
    await f.dispose();
  }
});
