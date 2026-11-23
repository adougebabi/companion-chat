import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { installDOM, mountReact } from "./helpers/react-dom.mjs";

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  envFile: false,
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
test.after(async () => {
  await vite.close();
  env.restore();
});
const { default: FeedPost } = await vite.ssrLoadModule("/src/react-app/shared/FeedPost.jsx");
const { default: FeedDetail } = await vite.ssrLoadModule("/src/react-app/shared/FeedDetail.jsx");

const internalFields = {
  revision: 93417,
  protocol_version: "npc_response_v1",
  source_ref: "place_private",
  idempotency_key: "command_private",
  correlation: "correlation_private",
  metrics: { latency_ms: 48019 },
};
const post = {
  ...internalFields,
  id: "res_private",
  owner_fluctlight_id: "fluctlight_private",
  author_actor_id: "actor_private",
  author_type: "fluctlight",
  author_name: "青岚",
  status: "visible",
  text: "雨过后去看了山茶花。",
  created_at: "2026-11-23T03:12:00Z",
  request_id: "req_private",
  source_event_id: "world_item_private",
  comment_count: 1,
  like_count: 1,
  reaction_count: 2,
  viewer_reaction: "care",
  media: [],
};
const comment = {
  id: "comment_private",
  author_type: "npc",
  author_name: "苏禾",
  text: "山茶花开得很早。",
  created_at: "2026-11-23T03:16:00Z",
  source_interaction_id: "interaction_private",
};
const interaction = {
  ...internalFields,
  id: "interaction_private",
  npc_actor_id: "actor_npc_private",
  status: "completed",
  updated_at: "2026-11-23T03:17:00Z",
  response: { decision: "comment", text: comment.text, reason: "想分享花期。" },
  tasks: [{ task_id: "task_private", result: { source_ref: "place_private" } }],
};
const failed = {
  id: "failed_private",
  status: "failed",
  response: {},
  error_code: "npc_moment_assessment_failed",
};

function model(row, comments, interactions) {
  const state = { session: { actorId: "feed-owner" }, characters: [] };
  const page = (items) => async () => ({ items, next_cursor: "" });
  return {
    owner: state.session.actorId,
    w: { state, subscribe: () => () => {}, getSnapshot: () => state },
    feed: { items: [row] },
    service: { comments: page(comments), npcInteractions: page(interactions) },
    readMoment: async () => row,
    live: () => true,
    adoptPost() {},
    momentEnabled: true,
    momentEnabledRef: { current: true },
    trigger: { current: null },
  };
}

const button = (root, label) => [...root.querySelectorAll("button")]
  .find((node) => node.textContent.trim() === label);
const link = (root, label) => [...root.querySelectorAll("a")]
  .find((node) => node.textContent.trim() === label);

function assertBusinessView(root) {
  assert.doesNotMatch(root.textContent, /\b(?:id|revision|protocol_version|author_name|author_actor_id|created_at|updated_at|source_ref|source_event_id|idempotency_key|correlation|metrics|latency_ms|error_code|npc_moment_assessment_failed)\b|版本|未记录|时间未知|实际结果尚未提供/);
  assert.doesNotMatch(root.textContent, /\b\w+_private\b|93417|48019|npc_response_v1|\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/);
  assert.equal(root.querySelector("pre, code, .record-facts"), null);
}

test("动态列表显示作者、中文时间、正文和评论预览，缺失或为内部 ID 的名称省略", async () => {
  const m = model(post, [comment], []);
  const view = await mountReact(vite, FeedPost, {
    workspace: m.w,
    props: { post, model: m, onAction() {} },
  });
  try {
    assert.equal(view.host.querySelector("header strong").textContent, "青岚");
    assert.equal(view.host.querySelector("time").textContent, "2026-11-23 11:12 (北京时间)");
    assert.equal(view.host.querySelector(".companion-post-text").textContent, post.text);
    assert.match(view.host.querySelector(".companion-comment-preview").textContent, /苏禾：山茶花开得很早/);
    assertBusinessView(view.host);

    const sparse = { ...post, author_name: "actor_private", created_at: null };
    await view.update({ post: sparse, model: m, onAction() {} });
    assert.equal(view.host.querySelector("header strong, header time"), null);
    assertBusinessView(view.host);
  } finally {
    await view.dispose();
  }
});

test("动态详情显示中文互动结果并仅下载完整原始动态和互动记录", async (t) => {
  const empty = { id: "pending_private", status: "pending", response: {}, updated_at: null };
  const reaction = {
    id: "reaction_private",
    status: "completed",
    response: { decision: "react", reaction: "celebrate" },
  };
  const uncommitted = {
    ...empty,
    id: "uncommitted_private",
    response: { decision: "comment", text: "这条评论还没有发布。" },
  };
  const m = model(post, [comment, { ...comment, id: "second_private", author_name: null, created_at: "" }],
    [interaction, failed, empty, reaction, uncommitted]);
  const view = await mountReact(vite, FeedDetail, {
    workspace: m.w,
    props: { id: post.id, model: m, onClose() {} },
  });
  const blobs = [], downloads = [];
  t.mock.method(URL, "createObjectURL", (blob) => {
    blobs.push(blob);
    return "blob:feed-record";
  });
  t.mock.method(URL, "revokeObjectURL", () => {});
  t.mock.method(window.HTMLAnchorElement.prototype, "click", function () {
    downloads.push({ filename: this.download, href: this.href });
  });

  try {
    const dialog = document.querySelector("[role=dialog]");
    assert.match(dialog.textContent, /作者：青岚 · 角色生活分享/);
    assert.equal(dialog.querySelector(".companion-comments time").textContent, "2026-11-23 11:16 (北京时间)");
    assert.equal(dialog.querySelectorAll(".companion-comments time").length, 1);
    assert.match(dialog.textContent, /来自已保存的人物互动/);
    await view.click(button(dialog, "更多回应与实际来源"));
    assert.match(dialog.textContent, /全部回应 2/);
    assert.match(dialog.textContent, /你已关心/);
    assert.equal(link(dialog, "原发布建议与处理结果").getAttribute("href"), "#/diag/request/req_private");
    await view.click(button(dialog, "原始动态记录（诊断）"));
    assertBusinessView(dialog);
    await view.click(button(dialog, "下载原始记录"));

    await view.click(button(dialog, "世界人物互动与实际来源"));
    const [completed, unsuccessful, pending, reacted, candidate] = dialog.querySelectorAll(".feed-npc-row");
    assert.match(completed.textContent, /实际互动已提交/);
    assert.match(completed.textContent, /评论：山茶花开得很早。/);
    assert.match(completed.textContent, /原因：想分享花期。/);
    assert.equal(completed.querySelector("time").textContent, "2026-11-23 11:17 (北京时间)");
    assert.equal(link(completed, "查看原任务").getAttribute("href"), "#/diag/task/task_private");
    assert.match(unsuccessful.textContent, /执行失败.*原因：人物互动判断失败/);
    assert.ok(button(unsuccessful, "重试本次互动"));
    assert.match(pending.textContent, /等待独立判断/);
    assert.equal(pending.querySelector("time, p"), null);
    assert.match(reacted.textContent, /实际互动已提交.*回应：庆祝/);
    assert.match(candidate.textContent, /等待独立判断/);
    assert.doesNotMatch(candidate.textContent, /这条评论还没有发布/);
    await view.click(button(completed, "原始动态互动记录（诊断）"));
    assertBusinessView(dialog);
    await view.click(button(completed, "下载原始记录"));

    assert.deepEqual(downloads, Array.from({ length: 2 }, () => ({
      filename: "fluctlight-record.json",
      href: "blob:feed-record",
    })));
    assert.deepEqual(blobs.map((blob) => blob.type), ["application/json", "application/json"]);
    assert.deepEqual(await Promise.all(blobs.map(async (blob) => JSON.parse(await blob.text()))), [post, interaction]);
  } finally {
    await view.dispose();
  }
});
