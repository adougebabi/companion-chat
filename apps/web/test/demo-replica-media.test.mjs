import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { mediaFixture } from "./fixtures/media-fixture.mjs";
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
  { default: Media } = await vite.ssrLoadModule(
    "/src/react-app/pages/MediaPage.tsx",
  ),
  { installMediaProvider } = await vite.ssrLoadModule(
    "/src/v3/providers/media.ts",
  ),
  { installInventoryProvider } = await vite.ssrLoadModule(
    "/src/v3/providers/inventory.ts",
  ),
  { installStudioProvider } = await vite.ssrLoadModule(
    "/src/v3/providers/studio.ts",
  );
test.after(async () => {
  await new Promise((r) => setTimeout(r, 20));
  await vite.close();
  env.restore();
});
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll("[role=dialog]")].at(-1),
  clone = (v) => JSON.parse(JSON.stringify(v));
async function tab(f, label) {
  await act(async () =>
    [...f.v.host.querySelectorAll("[role=tab]")]
      .find((n) => n.textContent.startsWith(label))
      .dispatchEvent(
        new window.MouseEvent("mousedown", {
          bubbles: true,
          button: 0,
          ctrlKey: false,
        }),
      ),
  );
  await f.v.flush();
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
async function score(f, label) {
  const root = dialog().querySelector('[aria-label="' + label + '"]'),
    node = root.matches("[role=slider]")
      ? root
      : root.querySelector("[role=slider]");
  await act(async () =>
    node.dispatchEvent(
      new window.KeyboardEvent("keydown", { key: "End", bubbles: true }),
    ),
  );
  await f.v.flush();
}
async function fixture({
  scenario = "normal",
  api,
  retainStorage = false,
  tab: initialTab,
  assetId,
  query = "",
  intercept,
} = {}) {
  if (!retainStorage) sessionStorage.clear();
  api ||= mediaFixture({ scenario });
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "media-owner" },
    characters: {
      A: { id: "A", name: "林青", status: "active" },
      B: { id: "B", name: "江屿", status: "active" },
    },
    ui: { charId: "A" },
  });
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, "http://media.test"),
      method = init.method || "GET",
      body = init.body ? JSON.parse(init.body) : {};
    const value = await intercept?.({ url, method, body, api });
    if (value instanceof Response) return value;
    let response;
    const handled = await api.handler({
      url,
      path: url.pathname,
      method,
      body,
      json: (_res, row, status = 200) =>
        (response = new Response(JSON.stringify(row), {
          status,
          headers: { "content-type": "application/json" },
        })),
    });
    assert.ok(handled, "Unhandled " + method + " " + url.pathname);
    return response;
  };
  installMediaProvider(w.client, async () => {});
  installInventoryProvider(w.client);
  installStudioProvider(w.client);
  const props = {
    route: { page: "媒体", path: "/media", query: new URLSearchParams(query) },
    initialTab,
    initialAssetId: assetId,
  };
  const v = await mountReact(vite, Media, { workspace: w, props });
  return {
    v,
    w,
    api,
    props,
    writes: () => api.calls.filter((c) => c.method !== "GET"),
    async dispose() {
      await v.dispose();
      w.dispose();
      await new Promise((r) => setTimeout(r, 12));
    },
  };
}
async function openAsset(f, title = "午后的窗边") {
  await f.v.click(
    f.v.host.querySelector('[aria-label="查看画面 ' + title + '"]'),
  );
}
test("a native asset deep link closes back to its explicit source context without a write", async () => {
  const target =
      "#/c/A/chat?conversation=source-conversation&message=source-message",
    f = await fixture({
      assetId: "replica-landscape",
      query: "returnTo=" + encodeURIComponent(target),
    });
  try {
    const moves = [];
    f.w.navigatePath = (p) => moves.push(p);
    await f.v.click(dialog().querySelector('[data-slot="dialog-close"]'));
    assert.deepEqual(moves, [target.slice(1)]);
    assert.equal(f.writes().length, 0);
  } finally {
    await f.dispose();
  }
});
test("media gallery reads real Owner scope and actual unavailable audio has no playable URL", async () => {
  const f = await fixture();
  try {
    assert.equal(
      f.api.calls.find((c) => c.path === "/api/media-library").query
        .fluctlightId,
      undefined,
    );
    assert.ok(f.v.host.querySelector(".r-media-grid"));
    await f.v.click(button(f.v.host, "加载更多素材"));
    assert.match(f.v.host.textContent, /尚未就绪的声音/);
    assert.equal(f.v.host.querySelector("audio"), null);
    assert.doesNotMatch(f.v.host.textContent, /只看喜欢/);
    await openAsset(f);
    assert.ok(dialog().querySelector("img"));
    assert.match(dialog().textContent, /保存的素材/);
  } finally {
    await f.dispose();
  }
});
test("metadata cancel never writes, saving uses both versions and hide removes the actual gallery card", async () => {
  const f = await fixture();
  try {
    await openAsset(f);
    await f.v.click(button(dialog(), "编辑库信息"));
    assert.equal(document.querySelectorAll('[role=dialog]').length,1);
    await f.v.input(
      dialog().querySelector('[aria-label="素材名称"]'),
      "不保存的新名字",
    );
    await f.v.click(button(dialog(), "取消"));
    assert.equal(f.writes().length, 0);
    await f.v.click(button(dialog(), "编辑库信息"));
    await f.v.input(
      dialog().querySelector('[aria-label="素材名称"]'),
      "窗边的阅读时光",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="修改原因"]'),
      "整理素材名称与展示",
    );
    await f.v.click(dialog().querySelector('[aria-label="在素材库中隐藏"]'));
    await f.v.click(button(dialog(), "保存库信息"));
    const write = f.writes()[0];
    assert.equal(write.body.expectedAssetRevision, 3);
    assert.equal(write.body.expectedMetadataRevision, 1);
    assert.equal(write.body.hidden, true);
    assert.equal(f.api.assets.get("replica-landscape").title, "窗边的阅读时光");
    assert.equal(
      f.v.host.querySelector('[aria-label="查看画面 午后的窗边"]'),
      null,
    );
  } finally {
    await f.dispose();
  }
});
test("unknown metadata reopens after refresh and replays identical body and key", async () => {
  const api = mediaFixture({ scenario: "unknown" });
  let f = await fixture({ api, assetId: "replica-landscape" });
  try {
    await f.v.click(button(dialog(), "编辑库信息"));
    await f.v.input(
      dialog().querySelector('[aria-label="素材名称"]'),
      "原名称修改",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="修改原因"]'),
      "核对原修改",
    );
    await f.v.click(button(dialog(), "保存库信息"));
    const first = clone(f.writes()[0]);
    assert.ok(button(dialog(), "核对或重试原提交"));
    await f.dispose();
    f = await fixture({
      api,
      retainStorage: true,
      assetId: "replica-landscape",
    });
    await f.v.click(button(dialog(), "编辑库信息"));
    await f.v.click(button(dialog(), "核对或重试原提交"));
    assert.deepEqual(f.writes()[1], first);
    assert.equal(api.assets.get("replica-landscape").metadata_revision, 2);
  } finally {
    await f.dispose();
  }
});
test("accepted metadata recovers only reads when subsequent asset readback failed", async () => {
  const f = await fixture({ scenario: "readback" });
  try {
    await openAsset(f);
    await f.v.click(button(dialog(), "编辑库信息"));
    await f.v.input(
      dialog().querySelector('[aria-label="修改原因"]'),
      "核对资料",
    );
    await f.v.click(button(dialog(), "保存库信息"));
    assert.ok(button(dialog(), "只读恢复页面结果"));
    await f.v.click(button(dialog(), "只读恢复页面结果"));
    assert.equal(f.writes().length, 1);
  } finally {
    await f.dispose();
  }
});
test("visual definition changes remain separate from canonical identity and preserve unknown source data", async () => {
  const f = await fixture({ tab: "identity" });
  try {
    const old = f.api.state().visual.canonical_asset_id;
    await f.v.click(button(f.v.host, "编辑视觉定义"));
    await f.v.input(
      dialog().querySelector('[aria-label="外貌要点"]'),
      "自然的短发，保留面部比例。",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="保存原因"]'),
      "细化参考约束",
    );
    await f.v.click(button(dialog(), "保存视觉定义"));
    assert.equal(f.writes()[0].body.expectedRevision, 2);
    assert.equal(f.api.state().brief.revision, 3);
    assert.equal(f.api.state().visual.canonical_asset_id, old);
    assert.deepEqual(f.api.state().brief.brief.unknown_extension, {
      preserve: true,
    });
    assert.equal("unknown_extension" in f.writes()[0].body, false);
  } finally {
    await f.dispose();
  }
});
test("visual generation can be accepted into pending queue without claiming the existing review session ended", async () => {
  const f = await fixture({ tab: "identity" });
  try {
    const before = clone(f.api.state().visual);
    await f.v.click(button(f.v.host, "生成新候选"));
    await f.v.input(
      dialog().querySelector('[aria-label="生成原因"]'),
      "希望核对新的候选",
    );
    await f.v.click(button(dialog(), "开始准备候选"));
    const write = f.writes()[0];
    assert.equal(write.body.expectedVisualRevision, 1);
    assert.equal(write.body.expectedBriefRevision, 2);
    assert.equal("supersedeSessionId" in write.body, false);
    assert.equal(f.api.state().visual.session.id, before.session.id);
    assert.match(f.v.host.textContent, /受理时等待可用会话/);
    assert.doesNotMatch(f.v.host.textContent, /结束等待评审|60%/);
  } finally {
    await f.dispose();
  }
});
test("accepted candidate review uses real 0–1 scores and keeps character sheet preparation separate", async () => {
  const f = await fixture({ tab: "identity" });
  try {
    await f.v.click(button(f.v.host, "查看候选并评审"));
    await score(f, "身份符合程度");
    await score(f, "评审置信程度");
    await f.v.input(
      dialog().querySelector('[aria-label="实际观察（一行一项）"]'),
      "五官与已保存定义一致",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="评审摘要"]'),
      "核对并接受这个候选",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="操作理由"]'),
      "人工查看实际图片后确认",
    );
    await f.v.click(button(dialog(), "提交人工评审"));
    const write = f.writes()[0];
    assert.equal(write.body.expectedAttempt, 2);
    assert.equal(write.body.expectedVisualRevision, 1);
    assert.equal(write.body.identityMatch, 1);
    assert.equal(write.body.confidence, 1);
    assert.deepEqual(write.body.observations, ["五官与已保存定义一致"]);
    assert.equal(
      f.api.state().visual.canonical_asset_id,
      "A-replica-candidate",
    );
    assert.match(f.v.host.textContent, /设定图仍在准备/);
    assert.equal(f.api.state().visual.character_sheet_asset_id, null);
    await f.v.click(button(f.v.host, "重新读取"));
    await f.v.click(button(f.v.host, "重新读取"));
    assert.equal(
      f.api.state().visual.character_sheet_asset_id,
      "A-replica-sheet",
    );
  } finally {
    await f.dispose();
  }
});
test("missing review sections prevent acceptance and regeneration does not replace canonical identity", async () => {
  const f = await fixture({ tab: "identity" });
  try {
    const canonical = f.api.state().visual.canonical_asset_id;
    await f.v.click(button(f.v.host, "查看候选并评审"));
    await score(f, "身份符合程度");
    await score(f, "评审置信程度");
    await f.v.input(
      dialog().querySelector('[aria-label="实际观察（一行一项）"]'),
      "需要补齐侧面",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="缺失分区（一行一项）"]'),
      "侧面",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="评审摘要"]'),
      "补齐后重新核对",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="操作理由"]'),
      "候选缺失必要分区",
    );
    await f.v.click(button(dialog(), "提交人工评审"));
    assert.equal(f.writes().length, 0);
    assert.match(dialog().textContent, /不能接受候选/);
    await choose(f, "评审决定", "需要重新生成");
    await f.v.click(button(dialog(), "提交人工评审"));
    assert.equal(f.api.state().visual.canonical_asset_id, canonical);
    assert.equal(f.api.state().visual.session.current_attempt, 3);
  } finally {
    await f.dispose();
  }
});
test("avatar selection saves its own revision and actual asset version without replacing canonical identity", async () => {
  const f = await fixture({ tab: "avatar" });
  try {
    const old = f.api.state().visual.canonical_asset_id;
    await f.v.click(
      f.v.host.querySelector('[role=radio][aria-label="选择一张媒体图片"]'),
    );
    await f.v.click(
      f.v.host.querySelector('[aria-label="选择头像 午后的窗边"]'),
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="选择原因"]'),
      "选择更熟悉的显示画面",
    );
    await f.v.click(button(f.v.host, "保存显示头像"));
    const write = f.writes()[0];
    assert.equal(write.body.mode, "asset");
    assert.equal(write.body.assetId, "replica-landscape");
    assert.equal(write.body.expectedAssetRevision, 3);
    assert.equal(write.body.expectedRevision, 1);
    assert.equal(f.api.state().avatar.selection.asset_id, "replica-landscape");
    assert.equal(f.api.state().visual.canonical_asset_id, old);
    assert.match(f.v.host.textContent, /显示头像已保存并读回/);
  } finally {
    await f.dispose();
  }
});
test("linked avatar assets require an explicit selection and save instead of automatically changing the avatar", async () => {
  const f = await fixture({ tab: "avatar", query: "asset=replica-unused" });
  try {
    assert.ok(button(f.v.host, "明确选择链接中的图片"));
    assert.equal(f.writes().length, 0);
    await f.v.click(button(f.v.host, "明确选择链接中的图片"));
    await f.v.input(
      f.v.host.querySelector('[aria-label="选择原因"]'),
      "确认链接图片作为头像",
    );
    await f.v.click(button(f.v.host, "保存显示头像"));
    assert.equal(f.writes()[0].body.assetId, "replica-unused");
  } finally {
    await f.dispose();
  }
});
test("image request freezes actual place/item/grant versions and separates acceptance from readable output", async () => {
  const f = await fixture({ tab: "generate" });
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="画面描述"]'),
      "窗边阅读的午后",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="请求原因"]'),
      "记录喜欢的画面",
    );
    await choose(f, "画面地点", "窗边");
    await f.v.click(button(f.v.host, "关联登记物品 · 不关联物品"));
    await f.v.click(button(f.v.host, "关联画面"));
    await f.v.click(button(f.v.host, "参考图片 · 不使用参考图片"));
    await f.v.click(button(f.v.host, "选择参考"));
    await f.v.click(button(f.v.host, "提交图片请求"));
    const write = f.writes()[0];
    assert.equal(write.body.placeId, "A-window");
    assert.equal(write.body.expectedPlaceRevision, 4);
    assert.equal(write.body.itemId, "A-coat");
    assert.equal(write.body.expectedItemRevision, 5);
    assert.equal(write.body.referenceAsset.grantRevision, 1);
    assert.match(f.v.host.textContent, /图片请求已受理/);
    assert.doesNotMatch(f.v.host.textContent, /图片已生成，实际资产已读回/);
    await f.v.click(button(f.v.host, "继续查询实际结果"));
    await f.v.click(button(f.v.host, "继续查询实际结果"));
    assert.match(f.v.host.textContent, /图片已生成，实际资产已读回/);
    assert.equal(f.writes().length, 1);
  } finally {
    await f.dispose();
  }
});
test("failed image generation keeps actual failure and does not show a fabricated completed image", async () => {
  const f = await fixture({ tab: "generate", scenario: "generation-failure" });
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="画面描述"]'),
      "尝试真实图片请求",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="请求原因"]'),
      "核对失败状态",
    );
    await f.v.click(button(f.v.host, "提交图片请求"));
    assert.match(
      f.v.host.textContent,
      /图片服务暂时不可用|provider_unavailable|服务暂时无法连接/,
    );
    assert.doesNotMatch(f.v.host.textContent, /图片已生成，实际资产已读回/);
    assert.equal(f.v.host.querySelector(".r-generated-result"), null);
    assert.equal(f.writes().length, 1);
  } finally {
    await f.dispose();
  }
});
test("reference grants require explicit purpose and confirmation, and revocation preserves actual history", async () => {
  const f = await fixture({ assetId: "replica-unused" });
  try {
    await f.v.click(button(dialog(), "引用授权与删除管理"));
    await f.v.click(button(dialog(), "新增参考授权"));
    await f.v.click(button(dialog(), "保存参考授权"));
    assert.equal(f.writes().length, 0);
    await choose(f, "参考用途", "物品参考");
    await f.v.input(
      dialog().querySelector('[aria-label="授权原因"]'),
      "仅用于物品画面的参考",
    );
    await f.v.click(dialog().querySelector('[aria-label="确认传送参考图片"]'));
    await f.v.click(button(dialog(), "保存参考授权"));
    const write = f.writes()[0];
    assert.equal(write.body.assetId, "replica-unused");
    assert.equal(write.body.expectedAssetRevision, 3);
    assert.equal(write.body.targetId, "A");
    assert.equal(write.body.purpose, "object");
    assert.equal(write.body.confirm, true);
    await f.v.click(button(dialog(), "撤销授权"));
    await f.v.input(
      dialog().querySelector('[aria-label="撤销原因"]'),
      "停止后续参考使用",
    );
    await f.v.click(button(dialog(), "确认撤销授权"));
    assert.equal(f.writes()[1].body.expectedRevision, 1);
    assert.equal(
      [...f.api.grants.values()].find((g) => g.asset_id === "replica-unused")
        .status,
      "revoked",
    );
  } finally {
    await f.dispose();
  }
});
test("a zero-reference deletion is an accepted background operation, not an immediate asset restoration or fake success", async () => {
  const f = await fixture({ assetId: "replica-unused" });
  try {
    await f.v.click(button(dialog(), "引用授权与删除管理"));
    await f.v.click(button(dialog(), "实际引用与删除"));
    await f.v.click(button(dialog(), "申请删除指定版本"));
    await f.v.input(
      dialog().querySelector('[aria-label="删除或重试原因"]'),
      "移除未使用的隔离样本",
    );
    await f.v.click(
      dialog().querySelector('[aria-label="确认删除指定对象版本"]'),
    );
    await f.v.click(button(dialog(), "确认申请删除"));
    assert.equal(f.writes()[0].method, "DELETE");
    assert.equal(f.writes()[0].body.expectedAssetRevision, 3);
    assert.equal(f.api.assets.get("replica-unused").status, "tombstoned");
    assert.match(dialog().textContent, /删除已受理/);
    assert.equal(button(dialog(), "恢复已删除图片"), undefined);
  } finally {
    await f.dispose();
  }
});
test("failed deletion retries the original deletion with its revision and keeps the asset unreadable", async () => {
  const f = await fixture({
    scenario: "delete-failure",
    assetId: "replica-unused",
  });
  try {
    await f.v.click(button(dialog(), "引用授权与删除管理"));
    await f.v.click(button(dialog(), "实际引用与删除"));
    await f.v.click(button(dialog(), "申请删除指定版本"));
    await f.v.input(
      dialog().querySelector('[aria-label="删除或重试原因"]'),
      "删除未使用的隔离素材",
    );
    await f.v.click(
      dialog().querySelector('[aria-label="确认删除指定对象版本"]'),
    );
    await f.v.click(button(dialog(), "确认申请删除"));
    assert.match(dialog().textContent, /删除未完成/);
    assert.equal(f.api.assets.get("replica-unused").readable, false);
    await f.v.click(button(dialog(), "重试失败的原删除"));
    await f.v.input(
      dialog().querySelector('[aria-label="删除或重试原因"]'),
      "继续处理失败的原删除",
    );
    await f.v.click(button(dialog(), "确认重试原删除"));
    const writes = f.writes();
    assert.equal(writes.length, 2);
    assert.match(writes[1].path, /\/media-deletions\/[^/]+\/retry$/);
    assert.equal(writes[1].body.expectedRevision, 1);
    assert.equal(f.api.deletions.size, 1);
    assert.equal(f.api.assets.get("replica-unused").readable, false);
  } finally {
    await f.dispose();
  }
});
test("unknown deletion has an explicit refresh-recovery entry and replays the same asset revision and key", async () => {
  const api = mediaFixture({ scenario: "unknown" });
  let f = await fixture({ api, assetId: "replica-unused" });
  try {
    await f.v.click(button(dialog(), "引用授权与删除管理"));
    await f.v.click(button(dialog(), "实际引用与删除"));
    await f.v.click(button(dialog(), "申请删除指定版本"));
    await f.v.input(
      dialog().querySelector('[aria-label="删除或重试原因"]'),
      "核对原删除申请",
    );
    await f.v.click(
      dialog().querySelector('[aria-label="确认删除指定对象版本"]'),
    );
    await f.v.click(button(dialog(), "确认申请删除"));
    const original = clone(f.writes()[0]);
    assert.ok(button(dialog(), "核对或重试原提交"));
    await f.dispose();
    f = await fixture({ api, assetId: "replica-unused", retainStorage: true });
    await f.v.click(button(dialog(), "引用授权与删除管理"));
    await f.v.click(button(dialog(), "实际引用与删除"));
    await f.v.click(button(dialog(), "继续核对原删除提交"));
    await f.v.click(button(dialog(), "核对或重试原提交"));
    assert.deepEqual(f.writes()[1], original);
    assert.equal(api.deletions.size, 1);
    assert.equal(api.assets.get("replica-unused").readable, false);
  } finally {
    await f.dispose();
  }
});
test("empty media, visual, avatar and image history are honest without fake seed images", async () => {
  const f = await fixture({ scenario: "empty" });
  try {
    assert.match(f.v.host.textContent, /还没有这样的画面/);
    await tab(f, "视觉身份");
    assert.match(f.v.host.textContent, /尚未建立|从第一个视觉候选开始/);
    assert.equal(f.v.host.querySelector(".r-identity-image img"), null);
    await tab(f, "生成画面");
    assert.match(f.v.host.textContent, /第一个想法/);
    await tab(f, "显示头像");
    assert.match(f.v.host.textContent, /尚未保存头像选择/);
  } finally {
    await f.dispose();
  }
});

async function fillReview(f, decision = "accepted") {
  await f.v.click(button(f.v.host, "查看候选并评审"));
  await score(f, "身份符合程度");
  await score(f, "评审置信程度");
  await f.v.input(
    dialog().querySelector('[aria-label="实际观察（一行一项）"]'),
    "实际查看五官和比例",
  );
  await f.v.input(
    dialog().querySelector('[aria-label="评审摘要"]'),
    "核对当前候选",
  );
  await f.v.input(
    dialog().querySelector('[aria-label="操作理由"]'),
    "保存人工观察与决定",
  );
  if (decision !== "accepted") await choose(f, "评审决定", "需要重新生成");
}
for (const decision of ["accepted", "regenerate"])
  test(
    "a lost " +
      decision +
      " review can restore its original session/attempt after that candidate is already decided",
    async () => {
      const api = mediaFixture({ scenario: "unknown" });
      let f = await fixture({ api, tab: "identity" });
      try {
        await fillReview(f, decision);
        await f.v.click(button(dialog(), "提交人工评审"));
        const original = clone(f.writes()[0]);
        assert.ok(button(dialog(), "核对或重试原提交"));
        await f.dispose();
        f = await fixture({ api, tab: "identity", retainStorage: true });
        const restore = button(f.v.host, "继续核对第2次人工评审");
        assert.ok(restore);
        await f.v.click(restore);
        await f.v.click(button(dialog(), "核对或重试原提交"));
        assert.deepEqual(f.writes()[1], original);
        assert.equal(f.writes()[1].body.expectedAttempt, 2);
        assert.equal(
          api.state().visual.current_revision,
          decision === "accepted" ? 2 : 1,
        );
        assert.equal(dialog(), undefined);
      } finally {
        await f.dispose();
      }
    },
  );
test("an unknown revocation remains recoverable from the revoked list after refresh", async () => {
  const api = mediaFixture({ scenario: "unknown" });
  let f = await fixture({ api, assetId: "replica-landscape" });
  try {
    await f.v.click(button(dialog(), "引用授权与删除管理"));
    await f.v.click(button(dialog(), "撤销授权"));
    await f.v.input(
      dialog().querySelector('[aria-label="撤销原因"]'),
      "停止这份原授权",
    );
    await f.v.click(button(dialog(), "确认撤销授权"));
    const original = clone(f.writes()[0]);
    assert.ok(button(dialog(), "核对或重试原提交"));
    await f.dispose();
    f = await fixture({
      api,
      assetId: "replica-landscape",
      retainStorage: true,
    });
    await f.v.click(button(dialog(), "引用授权与删除管理"));
    await f.v.click(button(dialog(), "查看已撤销授权"));
    await f.v.click(button(dialog(), "继续核对原撤销"));
    await f.v.click(button(dialog(), "核对或重试原提交"));
    assert.deepEqual(f.writes()[1], original);
    assert.equal([...api.grants.values()][0].revision, 2);
    assert.equal(button(dialog(), "继续核对原撤销"), undefined);
  } finally {
    await f.dispose();
  }
});
test("an invalid saved avatar source falls back to the name; only an explicit new image choice previews current bytes", async () => {
  const api = mediaFixture(),
    s = api.state(),
    a = api.assets.get("replica-landscape");
  s.avatar = {
    fluctlight_id: "A",
    revision: 4,
    mode: "asset",
    selection: {
      asset_id: a.id,
      asset_revision: a.asset_revision,
      version: a.version,
      sha256: a.sha256,
    },
    avatar: null,
    read_state: "source_identity_changed",
  };
  a.version = "later-version";
  a.sha256 = "b".repeat(64);
  const f = await fixture({ api, tab: "avatar" });
  try {
    assert.equal(f.v.host.querySelector(".r-avatar-big img"), null);
    assert.match(f.v.host.textContent, /实际显示回退为名字首字/);
    await f.v.click(
      f.v.host.querySelector('[aria-label="选择头像 午后的窗边"]'),
    );
    assert.ok(f.v.host.querySelector(".r-avatar-big img"));
    assert.equal(f.writes().length, 0);
    await f.v.input(
      f.v.host.querySelector('[aria-label="选择原因"]'),
      "明确采用重新选择的当前版本",
    );
    await f.v.click(button(f.v.host, "保存显示头像"));
    assert.equal(f.writes()[0].body.expectedRevision, 4);
    assert.equal(s.avatar.selection.version, "later-version");
    assert.equal(s.visual.canonical_asset_id, "A-replica-canonical");
  } finally {
    await f.dispose();
  }
});
test("metadata CAS review adopts untouched current fields while preserving actual edits and a new command identity", async () => {
  const f = await fixture({
    scenario: "conflict",
    assetId: "replica-landscape",
  });
  try {
    await f.v.click(button(dialog(), "编辑库信息"));
    await f.v.input(
      dialog().querySelector('[aria-label="素材名称"]'),
      "保留我正在改的名字",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="修改原因"]'),
      "核对并保留我的修改",
    );
    await f.v.click(button(dialog(), "保存库信息"));
    f.api.assets.get("replica-landscape").hidden = true;
    await f.v.click(button(dialog(), "同步资料，保留输入"));
    await f.v.click(button(dialog(), "已核对，采用最新基线"));
    assert.equal(
      dialog().querySelector('[aria-label="素材名称"]').value,
      "保留我正在改的名字",
    );
    assert.equal(
      dialog()
        .querySelector('[aria-label="在素材库中隐藏"]')
        .getAttribute("aria-checked"),
      "true",
    );
    await f.v.click(button(dialog(), "保存库信息"));
    assert.equal(f.writes().length, 2);
    assert.equal(f.writes()[1].body.expectedMetadataRevision, 2);
    assert.equal(f.writes()[1].body.hidden, true);
    assert.notEqual(
      f.writes()[1].body.idempotencyKey,
      f.writes()[0].body.idempotencyKey,
    );
  } finally {
    await f.dispose();
  }
});
for (const mode of ["generation", "avatar", "brief"])
  test(
    "unknown " +
      mode +
      " submission replays the same original body/key after remount",
    async () => {
      const api = mediaFixture({ scenario: "unknown" }),
        tabName = {
          generation: "generate",
          avatar: "avatar",
          brief: "identity",
        }[mode];
      let f = await fixture({ api, tab: tabName });
      try {
        if (mode === "generation") {
          await f.v.input(
            f.v.host.querySelector('[aria-label="画面描述"]'),
            "保留原画面描述",
          );
          await f.v.input(
            f.v.host.querySelector('[aria-label="请求原因"]'),
            "核对原图片请求",
          );
          await f.v.click(button(f.v.host, "提交图片请求"));
        } else if (mode === "avatar") {
          await f.v.input(
            f.v.host.querySelector('[aria-label="选择原因"]'),
            "核对原头像选择",
          );
          await f.v.click(button(f.v.host, "保存显示头像"));
        } else {
          await f.v.click(button(f.v.host, "编辑视觉定义"));
          await f.v.input(
            dialog().querySelector('[aria-label="保存原因"]'),
            "核对原视觉定义",
          );
          await f.v.click(button(dialog(), "保存视觉定义"));
        }
        const original = clone(f.writes()[0]);
        assert.ok(button(dialog() || f.v.host, "核对或重试原提交"));
        if (mode === "generation") {
          await f.v.click(button(f.v.host, "重置当前草稿"));
          assert.equal(
            f.v.host.querySelector('[aria-label="画面描述"]').value,
            "",
          );
          assert.ok(button(f.v.host, "核对或重试原提交"));
        }
        if (mode === "avatar") {
          await f.v.click(button(f.v.host, "撤销更改"));
          assert.equal(
            f.v.host.querySelector('[aria-label="选择原因"]').value,
            "",
          );
          assert.ok(button(f.v.host, "核对或重试原提交"));
        }
        await f.dispose();
        f = await fixture({ api, tab: tabName, retainStorage: true });
        if (mode === "brief") await f.v.click(button(f.v.host, "编辑视觉定义"));
        await f.v.click(button(dialog() || f.v.host, "核对或重试原提交"));
        assert.deepEqual(f.writes()[1], original);
        if (mode === "generation") assert.equal(api.state().requests.size, 1);
        if (mode === "avatar") assert.equal(api.state().avatar.revision, 2);
        if (mode === "brief") assert.equal(api.state().brief.revision, 3);
      } finally {
        await f.dispose();
      }
    },
  );
test("failed initial reads stay distinct from empty records and old library queries cannot repaint a newer filter", async () => {
  let f = await fixture({ scenario: "failure" });
  try {
    assert.ok(button(f.v.host, "重新读取素材"));
    assert.doesNotMatch(f.v.host.textContent, /还没有这样的画面/);
    await f.v.click(button(f.v.host, "重新读取素材"));
    assert.match(f.v.host.textContent, /午后的窗边/);
    await tab(f, "视觉身份");
    assert.ok(button(f.v.host, "重新读取视觉身份"));
    assert.doesNotMatch(f.v.host.textContent, /从第一个视觉候选开始/);
    assert.match(f.v.host.textContent, /规范参考待读取/);
    await f.v.click(button(f.v.host, "重新读取视觉身份"));
    await f.v.click(button(f.v.host, "重新读取视觉定义"));
    assert.match(f.v.host.textContent, /深色短发/);
  } finally {
    await f.dispose();
  }
  let release;
  const gate = new Promise((r) => (release = r));
  f = await fixture({
    intercept: async ({ url, method }) => {
      if (
        method === "GET" &&
        url.pathname === "/api/media-library" &&
        !url.searchParams.get("q")
      )
        await gate;
    },
  });
  try {
    assert.match(f.v.host.textContent, /正在读取素材/);
    await f.v.update({
      ...f.props,
      route: { path: "/media", query: new URLSearchParams("q=雨后") },
    });
    assert.match(f.v.host.textContent, /雨后的街角/);
    await act(async () => release());
    await f.v.flush();
    assert.equal(
      f.v.host.querySelector('[aria-label="查看画面 午后的窗边"]'),
      null,
    );
  } finally {
    release();
    await f.dispose();
  }
});
