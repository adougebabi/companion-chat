import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";

const environment = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
const { default: MediaDetail } = await vite.ssrLoadModule("/src/react-app/shared/MediaDetail.jsx");
const { default: Results } = await vite.ssrLoadModule("/src/react-app/shared/MediaSourceResults.tsx");
test.after(async () => { await vite.close(); environment.restore(); });
const button = (root, label) => [...root.querySelectorAll("button")].find(node => node.textContent.trim() === label);
const row = (root, label) => [...root.querySelectorAll('[role="listitem"]')].find(node => node.querySelector('[data-slot="item-title"]').textContent.startsWith(label));
function asset(overrides = {}) {
  return {
    id: "internal-asset-id", asset_id: "internal-asset-id",
    generation_intent_id: "internal-generation-id", fluctlight_id: "internal-character-id",
    title: "午后的窗边", kind: "image", byte_size: 435003, status: "ready",
    readable: true, read_state: "ready", origin: "fluctlight", generation_status: "completed",
    asset_revision: 1, metadata_revision: 0, version: "internal-version", sha256: "a".repeat(64),
    url: "/api/media/internal-asset-id", hidden: false, factual: false, deletion_id: null,
    source_state: "recorded", source_snapshot: {
      purpose: "visual_identity", description: "午后的窗边与一杯热茶。",
      reference_asset: { asset_id: "internal-reference-id", sha256: "b".repeat(64) },
      context_binding: { actor_id: "internal-context-id" },
      unexpected_field: { secret: "future-internal-data" },
    }, ...overrides,
  };
}

test("media detail disclosure shows grouped Chinese results and icons without raw metadata", async () => {
  const original = asset(), before = structuredClone(original), reads = [];
  const state = { session: { actorId: "owner" } };
  const workspace = { state, subscribe: () => () => {}, getSnapshot: () => state };
  let closed = 0;
  const model = {
    route: { query: new URLSearchParams() }, trigger: { current: null }, live: () => true,
    readAsset: async id => { reads.push(id); return original; }, adoptAsset() {}, target: "",
  };
  const view = await mountReact(vite, MediaDetail, { workspace, props: { model, id: original.id, onClose: () => { closed++; } } });
  try {
    const dialog = document.querySelector('[role="dialog"]');
    const disclosure = button(dialog, "生成时来源与资产校验信息");
    assert.equal(disclosure.getAttribute("aria-expanded"), "false");
    await view.click(disclosure);
    assert.equal(disclosure.getAttribute("aria-expanded"), "true");
    const results = dialog.querySelector('[data-testid="media-source-results"]');
    assert.deepEqual([...results.querySelectorAll("h3")].map(node => node.textContent), ["资产可用性", "生成时来源", "库中状态"]);
    for (const label of ["资产状态", "读取校验", "生成状态", "来源记录"]) {
      assert.equal(row(results, label).dataset.result, "pass", label);
      assert.match(row(results, label).textContent, /通过/);
      assert.ok(row(results, label).querySelector('svg[aria-hidden="true"]'));
    }
    assert.match(results.textContent, /角色视觉形象.*午后的窗边与一杯热茶。.*在素材库显示.*不作为角色生活事实/s);
    assert.doesNotMatch(dialog.textContent, /asset_id|generation_intent_id|source_snapshot|read_state|sha256|internal-|future-internal-data|\bnull\b|\bfalse\b/);
    assert.equal(dialog.querySelector("pre"), null);
    for (const node of dialog.querySelectorAll("[title], [aria-label]")) {
      assert.doesNotMatch((node.title || "") + node.getAttribute("aria-label"), /internal-|sha256|a{64}|b{64}/);
    }
    const link = [...dialog.querySelectorAll("a")].find(node => node.textContent === "查看原始参考素材");
    assert.equal(link.getAttribute("href"), "#/media/internal-reference-id");
    assert.deepEqual(original, before, "presentation must preserve the original provenance");
    assert.deepEqual(reads, [original.id]);
    await act(async () => dialog.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Escape", bubbles: true })));
    assert.equal(closed, 1);
  } finally { await view.dispose(); }
});

test("media results distinguish failures, pending and missing source records without inventing passes", async () => {
  const view = await mountReact(vite, Results, { props: { asset: asset({
    readable: false, read_state: "source_unavailable", status: "unavailable", generation_status: "failed",
    source_state: "legacy_unstructured", source_snapshot: null,
  }) } });
  try {
    for (const label of ["资产状态", "读取校验", "生成状态"]) {
      assert.equal(row(view.host, label).dataset.result, "fail");
      assert.match(row(view.host, label).textContent, /失败/);
      assert.ok(row(view.host, label).querySelector("svg"));
    }
    assert.match(row(view.host, "读取校验").textContent, /来源暂不可用/);
    assert.equal(row(view.host, "来源记录").dataset.result, "pending");
    assert.match(row(view.host, "来源记录").textContent, /旧记录未提供结构化来源/);
    await view.update({ asset: asset({ status: "constructor", generation_status: "new_internal_status", readable: null, source_state: "recorded", source_snapshot: {} }) });
    for (const label of ["资产状态", "读取校验", "生成状态", "来源记录"]) assert.equal(row(view.host, label).dataset.result, "pending", label);
    assert.doesNotMatch(view.host.textContent, /constructor|new_internal_status|null|通过/);
    await view.update({ asset: {} });
    assert.equal(view.host.querySelector('[data-result="pass"]'), null);
    assert.match(view.host.textContent, /事实属性尚未记录/);
  } finally { await view.dispose(); }
});

test("uploaded media treats absent generation snapshots as original sources and translates booleans", async () => {
  const view = await mountReact(vite, Results, { props: { asset: asset({ origin: "owner_upload", source_state: "owner_original", source_snapshot: null, generation_status: null, hidden: true }) } });
  try {
    assert.equal(row(view.host, "生成状态"), undefined);
    assert.equal(row(view.host, "来源记录").dataset.result, "pass");
    assert.match(view.host.textContent, /我上传的素材.*已记录原始上传来源.*已在素材库隐藏/s);
    assert.doesNotMatch(view.host.textContent, /null|true|false/);
  } finally { await view.dispose(); }
});

test("long media source values remain complete and wrap without a nested scrolling region", async () => {
  const description = "长说明\n" + "连续内容".repeat(180) + "完整末尾";
  const view = await mountReact(vite, Results, { props: { asset: asset({ source_snapshot: { description, studio_source: { name: "创作名称".repeat(50), resource_id: "internal-studio-id" } } }) } });
  try {
    const value = row(view.host, "画面说明").querySelector('[data-slot="item-description"]');
    assert.equal(value.textContent, description);
    assert.ok(value.classList.contains("whitespace-pre-wrap"));
    assert.ok(value.classList.contains("[overflow-wrap:anywhere]"));
    assert.ok(value.classList.contains("line-clamp-none"));
    assert.match(row(view.host, "创作名称").textContent, new RegExp("创作名称".repeat(50)));
    assert.equal(view.host.querySelector("pre, textarea, [data-slot=scroll-area], [class*=overflow-auto], [class*=overflow-y-auto], [class*=overflow-x-auto], [class*=max-h-]"), null);
    assert.doesNotMatch(view.host.textContent, /internal-studio-id|resource_id/);
  } finally { await view.dispose(); }
});
