import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "./support/module-server.mjs";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";

const dom = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  envFile: false,
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true, ws: false },
});
test.after(async () => { await vite.close(); dom.restore(); });
const { default: MemoryPage } = await vite.ssrLoadModule("/src/react-app/pages/MemoryPage.jsx");
const { featureRoute } = await vite.ssrLoadModule("/src/approved-ui/feature-routes.mjs");
const { configureDomainService } = await vite.ssrLoadModule("/src/v3/proto/services.js");

const fid = "fluctlight_hidden";
const time = "2026-11-23T01:44:08Z";
const technical = {
  id: "mem_hidden", revision: 9713, schema_version: "fluctlight.memory.v7",
  protocol_version: "fluctlight.records.v7", source_ref: "res_hidden", source_revision: 4810, idempotency_key: "private-replay",
  metrics: { world_item_hidden: 89017 }, correlation_id: "req_hidden",
  arbitrary_key: { place_hidden: ["中文<&'\"", false, 0, null] },
};
const memory = {
  ...technical, owner_fluctlight_id: fid, type: "episodic", status: "active", content: "我们在雨中散步。",
  created_at: time, occurred_at: time, provenance_status: "valid", visibility: "owner",
  importance: null, confidence: 0, emotional_significance: 0.6, last_confirmed_at: null,
  actor_refs: ["actor_hidden"], event_refs: ["world_item_hidden"], tags: ["散步", "res_hidden"],
  personality_perspectives: [{ profile_id: "actor_hidden", interpretation: "她很喜欢雨天的安静。", emotion: "平静" }],
};
const source = {
  ...technical, source_kind: "message", source_id: "res_hidden", conversation_id: "place_hidden",
  occurred_at: time, live: true, superseded_by_ref: "world_item_hidden", memory_revision: 9713,
};
const revision = {
  ...technical, revision: 9712, operation: "revise", status: "active", content: "较早记得我们看雨。",
  created_at: time, actor_id: "actor_hidden", semantic_reason: "owner:actor_hidden:补充当时的情景。",
};
const reflection = {
  ...technical, id: "refl_hidden", fluctlight_id: fid, title: "反思窗口 617—619", summary: "她发现散步能让心情放松。",
  at: time, status: "applied", body: "她想留意让自己放松的小事。",
  source_window: { evidence: [{ kind: "conversation.turn", occurred_at: time, source_ref: "res_hidden" }] },
  proposal: { memory_candidates: [{ operation: "revise", content: "新增雨天散步的感受。", source_ref: "res_hidden" }] },
  dispositions: [{ domain: "memory", candidate_index: 0, disposition: "accepted" }],
  result: { ...technical, status: "applied", changed_refs: ["res_hidden"], counts: { memory: { applied: 1 } } },
};
const latestTask = { ...technical, status: "completed", completed_at: time };
const feedback = {
  ...technical, id: "req_hidden", fluctlight_id: fid, status: "completed", created_at: time,
  payload: { reflection_id: reflection.id, stance: "agree", text: "我也喜欢雨天散步。" },
};
const claim = {
  ...technical, id: "claim_hidden", fluctlight_id: fid, category: "self_perception",
  claim: "我喜欢观察雨天的光。", confidence: null, created_at: time, status: "active", expires_at: null,
};
const claimRevision = { ...technical, revision: 9712, after_value: { ...claim, claim: "我喜欢细听雨声。" } };
const review = { ...technical, claim_revision: 9713, reason: "已核对雨天的认识。", created_at: time };
const page = (...items) => ({ items, next_cursor: "" });
const hidden = /\b(?:id|revision|(?:schema|protocol)[ _]version|source[ _](?:ref|kind|id|revision|window)|memory[ _]revision|(?:actor|event)[ _]refs|fluctlight[ _]id|idempotency[ _]key|metrics|correlation[ _]id|arbitrary[ _]key|candidate[ _]index|created[ _]at|accepted|applied|message|owner)\b|(?:mem|refl|claim|fluctlight|actor|req|res|place|world_item)_hidden|private-replay|89017|4810|971[23]|617—619|\.v7|版本|第\s*\d+\s*版|未记录|尚未提供|未提供|时间未知|未设置期限|2026-11-23T/i;
const memoryTitle = "原始记忆记录（诊断）";
const reflectionTitle = "原始反思记录（诊断）";
const claimTitle = "原始认识记录（诊断）";
const button = (root, label) => [...root.querySelectorAll("button")].find(node => node.textContent.trim() === label);
const dialog = () => document.querySelector("[role=dialog]");

function assertSummary(root, ...expected) {
  for (const text of expected) assert.ok(root.textContent.includes(text), text);
  assert.doesNotMatch(root.textContent, hidden);
  assert.equal(root.querySelector("pre, code"), null);
  for (const attribute of root.querySelectorAll("[title], [datetime], [aria-label]")) {
    assert.doesNotMatch(["title", "datetime", "aria-label"].map(key => attribute.getAttribute(key)).join(" "), hidden);
  }
}

async function tab(view, root, label) {
  const trigger = [...root.querySelectorAll("[role=tab]")].find(node => node.textContent.trim() === label);
  assert.ok(trigger, label);
  await act(async () => trigger.dispatchEvent(new window.MouseEvent("mousedown", { bubbles: true, button: 0 })));
  await view.flush();
}

async function reactDownload(t, view, root, title, expected) {
  const disclosure = button(root, title);
  assert.ok(disclosure, title);
  await view.click(disclosure);
  const panel = document.getElementById(disclosure.getAttribute("aria-controls"));
  const blobs = [], links = [];
  const create = t.mock.method(URL, "createObjectURL", blob => { blobs.push(blob); return "blob:memory-fixture"; });
  const click = t.mock.method(window.HTMLAnchorElement.prototype, "click", function () { links.push(this.download); });
  try {
    await view.click(button(panel || root, "下载原始记录"));
    assert.deepEqual(links, ["fluctlight-record.json"]);
    assert.equal(blobs.length, 1);
    assert.deepEqual(JSON.parse(await blobs[0].text()), expected);
    assertSummary(root);
  } finally {
    create.mock.restore();
    click.mock.restore();
  }
}

async function mountPage(t, route = {query:new URLSearchParams()}, overrides = {}) {
  const state = proxy({ session: { actorId: "actor_hidden" }, selectedId: fid, characters: [{ id: fid, name: "林青", status: "正常" }] });
  const navigations = [];
  const workspace = {
    state, current: { get value() { return state.characters[0]; } },
    subscribe: fn => subscribe(state, fn), getSnapshot: () => snapshot(state),
    isCurrent: (owner, character) => owner === state.session.actorId && character === fid,
    currentScope: () => JSON.stringify([state.session.actorId, fid]),
    navigatePath: path => navigations.push(path),
    client: {
      managedMemories: async () => page(memory), managedMemory: async () => memory,
      memorySources: async () => page(source), memoryRevisions: async () => page(revision),
      listReflections: async () => ({ ...page(reflection), latest_task: latestTask }), reflection: async () => reflection,
      reflectionFeedback: async () => page(feedback), selfClaims: async () => page(claim), selfClaim: async () => claim,
      selfClaimHistory: async (_fid, _id, kind) => kind === "revisions" ? page(claimRevision) : page(review),
      ...overrides,
    },
    services: { lifeClient: { request: async () => feedback } },
  };
  const view = await mountReact(vite, MemoryPage, { workspace, props:{route} });
  t.after(() => view.dispose());
  return { ...view, navigations };
}

test("记忆主页面展示中文正文、来源和修订，三类记忆原对象均可下载", async t => {
  const view = await mountPage(t);
  const card = view.host.querySelector(".lp-memory-card");
  assertSummary(card, memory.content, "经历", "#散步");
  await view.click(card);
  assertSummary(dialog(), "记忆详情", "所有者可见", "2026-11-23 09:44 (北京时间)", "置信度0%", "情感意义60%", "她很喜欢雨天的安静。", "感受：平静");
  assert.doesNotMatch(dialog().textContent, /重要度|最近确认/);
  await reactDownload(t, view, dialog(), memoryTitle, memory);

  await tab(view, dialog(), "原始来源");
  assertSummary(dialog(), "原消息", "来源有效", "已被其他来源替代");
  await reactDownload(t, view, dialog(), memoryTitle, source);
  await view.click(button(dialog(), "查看原消息"));
  const chat = new URL(view.navigations.at(-1), "http://react.test");
  assert.equal(chat.searchParams.get("message"), source.source_id);
  assert.equal(chat.searchParams.get("conversation"), source.conversation_id);

  await tab(view, dialog(), "修订记录");
  assertSummary(dialog(), "历史修订", revision.content, "纠正正文", "补充当时的情景。");
  assert.ok(button(dialog(), "补偿恢复至此版"));
  await reactDownload(t, view, dialog(), memoryTitle, revision);
});

test("记忆主页面反思和认识展示业务结果、快照与复核，下载深度等于原记录", async t => {
  const view = await mountPage(t);
  await tab(view, view.host, "反思记录");
  assertSummary(view.host, reflection.summary);
  await view.click(button(view.host, "最近反思任务"));
  assertSummary(view.host, "已完成", "2026-11-23 09:44 (北京时间)");
  await reactDownload(t, view, view.host, reflectionTitle, latestTask);
  await view.click(button(view.host, "展开这次反思"));
  await view.click(button(dialog(), "提议与实际改变"));
  assertSummary(dialog(), reflection.body, "对话交流 1 条", "新增雨天散步的感受。", "已采纳", "已应用 1 项");
  await reactDownload(t, view, dialog(), reflectionTitle, reflection);
  await view.click(button(dialog(), "所有者反馈与处理记录"));
  const feedbackCard = button(dialog(), "跟进原反馈请求").closest(".lp-source-card");
  assertSummary(feedbackCard, "我也喜欢雨天散步。", "本次处理已结束");
  await reactDownload(t, view, feedbackCard, reflectionTitle, feedback);
  await view.click(button(feedbackCard, "跟进原反馈请求"));
  const request = dialog().querySelector('[data-testid="request-status"]');
  assertSummary(request, "本次处理已结束");
  await reactDownload(t, view, request, reflectionTitle, feedback);
  await view.click(button(dialog(), "返回列表"));

  await tab(view, view.host, "自我认知");
  await view.click(button(view.host, "查看依据"));
  assertSummary(dialog(), claim.claim, "已记录的认识", "形成时间");
  assert.doesNotMatch(dialog().textContent, /置信度未提供|有效期限/);
  await reactDownload(t, view, dialog(), claimTitle, claim);
  await view.click(button(dialog(), "之前的认识与历史恢复"));
  const historical = dialog().querySelector(".lp-history-item");
  assertSummary(historical, "历史认识", claimRevision.after_value.claim);
  await reactDownload(t, view, historical, claimTitle, claimRevision);
  await view.click(button(dialog(), "所有者复核记录"));
  const reviewCard = dialog().querySelector(".lp-source-card");
  assertSummary(reviewCard, "已核对雨天的认识。", "当前认识");
  await reactDownload(t, view, reviewCard, claimTitle, review);
});

test("独立记忆详情复用主页面并按所选修订读取来源，下载深度等于原记录", async t => {
  const query = new URLSearchParams({returnTo:'#/c/'+fid+'/people/owner',returnLabel:'返回关于你'});
  const feature = featureRoute('/c/'+fid+'/growth/memory/'+memory.id, query);
  assert.equal(feature.native, 'memory');
  const sources = [];
  const view = await mountPage(t, {fid,query,feature}, {
    managedMemories:async()=>page(),
    memorySources:async(id,{revision:rev,cursor})=>{sources.push([id,rev,cursor]);return page({...source,memory_revision:rev});},
  });
  assertSummary(dialog(), memory.content, '记忆详情');
  assert.ok(button(dialog(), '记错了，纠正一下'));
  await reactDownload(t, view, dialog(), memoryTitle, memory);
  await tab(view, dialog(), '原始来源');
  assertSummary(dialog(), '来源有效');
  assert.deepEqual(sources, [[memory.id,9713,'']]);
  await reactDownload(t, view, dialog(), memoryTitle, {...source,memory_revision:9713});
  await tab(view, dialog(), '修订记录');
  await view.click(button(dialog(),'查看此版来源'));
  assert.deepEqual(sources.at(-1),[memory.id,9712,'']);
  assertSummary(dialog(),'这次历史修订的来源');
  await reactDownload(t,view,dialog(),memoryTitle,{...source,memory_revision:9712});
  await view.click(button(dialog(),'返回关于你'));
  assert.equal(view.navigations.at(-1),'/c/'+fid+'/people/owner');
});

test('unread object does not expose correction or feedback actions before a matching detail is read',async t=>{
  const feature=featureRoute('/c/'+fid+'/growth/memory/'+memory.id,new URLSearchParams());
  const view=await mountPage(t,{fid,feature,query:new URLSearchParams()},{managedMemory:async()=>{throw Error('读取失败');}});
  assert.ok(dialog());
  assert.equal(button(dialog(),'记错了，纠正一下'),undefined);
  assert.equal(button(dialog(),'遗忘这段记忆'),undefined);
  assert.match(dialog().textContent,/读取失败/);
});

test('formal memory query retains explicit aboutOwner=false in the actual client read',async t=>{
 const queries=[];
 await mountPage(t,{query:new URLSearchParams({aboutOwner:'false'})},{managedMemories:async(_fid,query)=>{queries.push(query);return page(memory);}});
 assert.equal(queries[0].aboutOwner,false);
});
