import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { createServer } from "./support/module-server.mjs";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";

const timestamp = "2026-11-23T01:34:56.000Z";
const diagnostics = {
  fluctlight_id: "fluctlight_wardrobe_test",
  revision: 814,
  schema_version: "inventory.v42",
  source_ref: "world_item_private_source",
  source_resource_id: "res_private_resource",
  source_version: "v614",
  source_overrides: { location: "place_private_location" },
  idempotency_key: "private-idempotency-token",
  correlation_id: "req_private_correlation",
  metrics: { provider: "actor_private_provider" },
};
const garment = {
  ...diagnostics,
  id: "item_test_coat",
  name: "深蓝外套",
  description: "带银扣的深蓝外套",
  clothing_type: "outer",
  ownership: "owned",
  availability: "available",
  seasons: ["winter"],
  acquired_at: timestamp,
  archived_at: null,
};
const outfit = {
  ...diagnostics,
  id: "outfit_test_plan",
  profile_id: "profile_private_test",
  name: "晚间散步",
  tags: ["出门", "散步"],
  created_at: timestamp,
  updated_at: null,
  items: [
    garment,
    { id: "item_saved_scarf", slot: "accessory", description: "暖灰围巾" },
    { id: "item_private_member", slot: "accessory", description: "res_private_member_description" },
  ],
};
const garmentRevision = {
  ...diagnostics,
  id: "inventory_history_test_coat",
  operation: "UPDATE",
  occurred_at: timestamp,
  reason: "",
  owner_audit: { reason: "归还借来的外套后购入了同款", owner_actor_id: "actor_audit_private" },
  before_value: { ...garment, ownership: "borrowed", availability: "unavailable" },
  after_value: garment,
};
const outfitRevision = {
  ...diagnostics,
  id: "outfit_revision_test_plan",
  created_at: timestamp,
  reason: "改为更保暖的组合",
  before_value: { name: "傍晚出门", items: [{ description: "薄外套" }] },
  after_value: outfit,
};

function workspace(selector) {
  const state = proxy({
    outfitTab: "全部衣服",
    operations: [],
    errors: {},
    loading: {},
    characters: [{
      id: garment.fluctlight_id,
      name: "林青",
      garments: [{
        id: garment.id,
        name: garment.name,
        type: "外套",
        seasons: ["冬"],
        _source: garment,
      }],
      outfits: [{
        id: outfit.id,
        name: outfit.name,
        tags: outfit.tags,
        tag: outfit.tags[0],
        items: { 外套: [garment.id], 配饰: outfit.items.slice(1).map((item) => item.id) },
        _source: outfit,
      }],
      outfitTags: outfit.tags,
      wearing: {},
      _wardrobe: { wearing: { revision: 3, worn_items: [] } },
    }],
  });
  return {
    state,
    current: selector(() => state.characters[0]),
    currentScope: () => "wardrobe-test-scope",
    subscribe: (listener) => subscribe(state, listener),
    getSnapshot: () => snapshot(state),
    services: { inventoryClient: {
      preparation: async () => ({ status: "ready" }),
      history: async () => ({ items: [garmentRevision, {}] }),
      outfitHistory: async () => ({ items: [outfitRevision] }),
    } },
  };
}

function button(root, label) {
  const found = [...root.querySelectorAll("button")].find(
    (node) => node.textContent.trim() === label || node.getAttribute("aria-label") === label,
  );
  assert.ok(found, `缺少按钮「${label}」`);
  return found;
}

function assertNoRawFields(root) {
  const text = [
    root.textContent,
    ...[...root.querySelectorAll("[aria-label]")].map((node) => node.getAttribute("aria-label")),
    ...[...root.querySelectorAll("input")].map((node) => node.value),
  ].join("\n");
  assert.doesNotMatch(text, /\b(?:id|revision|schema[_ ]version|profile[_ ]id|source[_ ](?:ref|resource[_ ]id|version|overrides)|idempotency[_ ]key|correlation[_ ]id|metrics|(?:occurred|created|acquired)[_ ]at|(?:before|after)[_ ]value|owner[_ ]audit|UPDATE)\b/);
  assert.doesNotMatch(text, /(?:world_item|res|place|req|actor|fluctlight|item|profile|outfit|inventory_history)_[a-z0-9_]+/i);
  assert.doesNotMatch(text, /版本|\bv\d+\b|\b814\b|private-idempotency-token|尚未记录|未记录相关|2026-11-23T01:34:56/);
  assert.equal(root.querySelector("pre"), null);
}

test("衣柜主页面展示中文业务摘要并下载完整服装、方案及修订记录", async (t) => {
  const environment = installDOM();
  t.after(() => environment.restore());
  t.mock.method(globalThis, "fetch", () => {
    assert.fail("衣柜夹具不应发出网络请求");
  });
  const vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    envFile: false,
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true, hmr: false },
  });
  t.after(() => vite.close());
  const { default: WardrobePage } = await vite.ssrLoadModule("/src/react-app/pages/WardrobePage.jsx");
  const { selector } = await vite.ssrLoadModule("/src/state/view-model.ts");
  const w = workspace(selector);
  const view = await mountReact(vite, WardrobePage, { workspace: w });

  const blobs = new Map();
  const downloads = [];
  t.mock.method(URL, "createObjectURL", (blob) => {
    const url = `blob:wardrobe-test-${blobs.size}`;
    blobs.set(url, blob);
    return url;
  });
  t.mock.method(window.HTMLAnchorElement.prototype, "click", function () {
    downloads.push({ href: this.href, filename: this.download });
  });
  async function assertDownload(trigger, filename, original) {
    await view.click(trigger);
    const details = document.getElementById(trigger.getAttribute("aria-controls"));
    assertNoRawFields(details);
    await view.click(button(details, "下载原始记录"));
    const download = downloads.at(-1);
    assert.equal(download.filename, filename);
    const blob = blobs.get(download.href);
    assert.equal(blob.type, "application/json");
    assert.deepEqual(JSON.parse(await blob.text()), original);
  }

  try {
    await view.click(button(view.host, "查看深蓝外套"));
    const garmentDialog = document.querySelector("[role=dialog]");
    assert.equal(garmentDialog.querySelector('[aria-label="衣服名称"]').value, garment.name);
    assert.match(garmentDialog.textContent, /服装描述：带银扣的深蓝外套/);
    assert.match(garmentDialog.textContent, /拥有关系已拥有/);
    assert.match(garmentDialog.textContent, /可用状态可穿着/);
    assert.match(garmentDialog.textContent, /登记时间：2026-11-23 09:34 \(北京时间\)/);
    assert.doesNotMatch(garmentDialog.textContent, /归档时间/);
    assert.ok(button(garmentDialog, "保存衣服"));
    await assertDownload(button(garmentDialog, "原始服装记录（诊断）"), "wardrobe-item.json", garment);
    await view.click(button(garmentDialog, "登记与修订历史"));
    assert.equal(garmentDialog.querySelectorAll("article").length, 1);
    const garmentHistory = garmentDialog.querySelector("article");
    assert.match(garmentHistory.textContent, /修订原因：归还借来的外套后购入了同款/);
    assert.match(garmentHistory.textContent, /修订时间：2026-11-23 09:34 \(北京时间\)/);
    assert.match(garmentHistory.textContent, /变更前.*拥有关系：借用.*可用状态：不可用/);
    assert.match(garmentHistory.textContent, /变更后.*拥有关系：已拥有.*可用状态：可用/);
    await assertDownload(button(garmentHistory, "原始服装记录（诊断）"), "wardrobe-item-history.json", garmentRevision);
    assertNoRawFields(garmentDialog);

    await view.click(button(garmentDialog, "取消"));
    await act(async () => { w.state.outfitTab = "方案库"; });
    await view.flush();
    assert.match(view.host.textContent, /晚间散步3 件服装/);
    assertNoRawFields(view.host);
    await view.click(button(view.host, "预览"));
    const outfitDialog = document.querySelector("[role=dialog]");
    assert.match(outfitDialog.textContent, /深蓝外套.*暖灰围巾/);
    assert.match(outfitDialog.textContent, /保存时间：2026-11-23 09:34 \(北京时间\)/);
    assert.doesNotMatch(outfitDialog.textContent, /最近修改/);
    assert.ok(button(outfitDialog, "编辑方案"));
    await assertDownload(button(outfitDialog, "原始穿搭方案（诊断）"), "wardrobe-outfit.json", outfit);
    await view.click(button(outfitDialog, "登记与修订历史"));
    const outfitHistory = outfitDialog.querySelector("article");
    assert.match(outfitHistory.textContent, /修订时间：2026-11-23 09:34 \(北京时间\)/);
    assert.match(outfitHistory.textContent, /修订原因：改为更保暖的组合/);
    assert.match(outfitHistory.textContent, /变更前.*搭配成员：薄外套/);
    assert.match(outfitHistory.textContent, /变更后.*搭配成员：带银扣的深蓝外套、暖灰围巾/);
    await assertDownload(button(outfitHistory, "原始穿搭方案（诊断）"), "wardrobe-outfit-history.json", outfitRevision);
    assertNoRawFields(outfitDialog);
    assert.equal(downloads.length, 4);
  } finally {
    await view.dispose();
  }
});
