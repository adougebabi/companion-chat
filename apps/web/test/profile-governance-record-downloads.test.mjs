import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { createServer } from "./support/module-server.mjs";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { profileDetail } from "./support/native-profile-fixture.mjs";

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  envFile: false,
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
const load = (path) => vite.ssrLoadModule(path);
test.after(async () => { await vite.close(); env.restore(); });

const when = "2026-11-23T01:35:00.000Z";
const privateFields = {
  id: "res_hidden",
  revision: 872635,
  schema_version: "fluctlight.profile.v42",
  source_ref: "world_item_hidden",
  idempotency_key: "req_hidden",
  metrics: { run_latency: "METRIC_HIDDEN" },
  actor_id: "actor_hidden",
  place_id: "place_hidden",
  fluctlight_id: "fluctlight_hidden",
  updated_at: when,
};
const hidden = /\b(?:id|revision|schema[ _]version|source[ _]ref|idempotency[ _]key|overlay[ _]revision|owner[ _]switch[ _]targets|profile[ _]id|rules[ _]version|metrics|run[ _]latency|registered[ _]capability|update[ _]policy|reason[ _]code|source[ _]window)\b|res_hidden|world_item_hidden|req_hidden|actor_hidden|place_hidden|fluctlight_hidden|METRIC_HIDDEN|secret_source_hash|872635|fluctlight\.profile\.v42|2026-11-23T|尚未记录|未记录额外说明|\bv8\b|当前协议|身份版本|运行版本/i;

function button(root, title) {
  const control = [...root.querySelectorAll("button")].find((node) => node.textContent.trim() === title);
  assert.ok(control, "缺少按钮 " + title);
  return control;
}

async function openTab(view, title, root = view.host) {
  const tab = [...root.querySelectorAll('[role="tab"]')].find((node) => node.textContent.trim().startsWith(title));
  assert.ok(tab, "缺少标签 " + title);
  await act(async () => tab.dispatchEvent(new window.MouseEvent("mousedown", { bubbles: true, button: 0 })));
  await view.flush();
}

function assertReading(root, ...facts) {
  assert.doesNotMatch(root.textContent, hidden);
  assert.equal(root.querySelector("pre, code, [data-testid=record-view]"), null);
  for (const fact of facts) assert.match(root.textContent, fact);
}

async function assertDownload(view, root, title, expected) {
  const trigger = button(root, title);
  await view.click(trigger);
  const disclosure = trigger.closest('[data-slot="collapsible"]');
  const originalCreate = URL.createObjectURL;
  const originalRevoke = URL.revokeObjectURL;
  const prototype = window.HTMLAnchorElement.prototype;
  const originalClick = prototype.click;
  let blob, filename;
  URL.createObjectURL = (value) => { blob = value; return "blob:profile-governance-fixture"; };
  URL.revokeObjectURL = () => {};
  prototype.click = function () { filename = this.download; };
  try {
    assertReading(disclosure);
    await view.click(button(disclosure, "下载原始记录"));
    assert.equal(filename, "fluctlight-record.json");
    assert.equal(blob.type, "application/json");
    assert.deepEqual(JSON.parse(await blob.text()), expected);
  } finally {
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
    prototype.click = originalClick;
  }
}

test("角色档案保留画像、实际状态和切换原因，诊断下载保留原始对象", async () => {
  const fid = "fluctlight_hidden";
  location.hash = "#/c/" + fid + "/profile";
  const { createWorkspace } = await load("/src/state/workspace.mjs");
  const store = await load("/src/v3/proto/store.js");
  const { default: Page } = await load("/src/react-app/pages/CharacterDetailPage.jsx");
  const detail = {
    ...profileDetail({ id: fid, name: "林青" }),
    personality: { ...privateFields, openness: 0.82, description: "愿意倾听" },
    persona_compilation: {
      ...privateFields, status: "ready", profile_id: "warm",
      overlay_revision: 872635, owner_switch_targets: [], rules_version: "fluctlight.profile.v42",
      source_hash: "secret_source_hash",
      working_persona: { portrait_text: "表达版本 v8 当前协议\n回复会留出思考的时间。\nsource_ref: res_hidden" },
    },
    current_state: {
      ...privateFields, mood: { label: "平静", source_ref: "res_hidden", metrics: privateFields.metrics },
      doing: { text: "整理书桌", action_id: "req_hidden" },
    },
    persona_switch_history: [{
      ...privateFields, previous_profile_id: "focused", active_profile_id: "warm",
      switch_reason: "讨论结束，回到日常交流", created_at: when,
    }],
  };
  const proposal = {
    ...privateFields, fluctlight_id: fid, source: "reflection", status: "proposed", reason: "补充阅读习惯", created_at: when,
    changes: { life_profile: { life_habits: "睡前阅读", metrics: privateFields.metrics } },
  };
  const capability = {
    ...privateFields, id: "req_hidden", fluctlight_id: fid, status: "fulfilled",
    title: "查阅书单", description: "列出近期想读的书", rationale: "便于回顾阅读计划", created_at: when,
  };
  const review = { ...privateFields, status: "accepted", note: "同意读取书单", created_at: when };
  const workspace = createWorkspace();
  store.replaceSnapshot({
    ...store.get(), session: { actorId: "fixture-owner" },
    characters: { [fid]: { id: fid, name: "林青", status: "active" } }, ui: { charId: fid },
  });
  workspace.client.fetcher = async () => { throw Error("Unexpected live request"); };
  workspace.client.detail = async () => detail;
  workspace.client.world = async () => ({ world: { name: "青岚" } });
  workspace.client.governancePage = async (_fid, kind) => ({ items: kind === "foundation" ? [proposal] : [capability], next_cursor: "" });
  workspace.client.governanceObject = async () => capability;
  workspace.client.governanceHistory = async () => ({ items: [review], next_cursor: "" });
  workspace.client.selfClaims = async () => ({ items: [], next_cursor: "" });
  workspace.loadWardrobe = async () => {};
  const view = await mountReact(vite, Page, { workspace });
  try {
    assertReading(view.host, /当前表达画像/, /回复会留出思考的时间/, /开放性/, /愿意倾听/);
    await assertDownload(view, view.host, "原始表达画像记录（诊断）", detail.persona_compilation);
    await assertDownload(view, view.host, "原始性格记录（诊断）", detail.personality);
    await openTab(view, "性格与人格");
    assertReading(view.host, /日常人格/, /专注人格/, /讨论结束，回到日常交流/, /2026-11-23 09:35 \(北京时间\)/);
    await assertDownload(view, view.host, "原始人格切换记录（诊断）", detail.persona_switch_history[0]);
    await openTab(view, "生活与此刻");
    assertReading(view.host, /实际外观与状态/, /平静/, /整理书桌/);
    await assertDownload(view, view.host, "原始当前状态记录（诊断）", detail.current_state);
    await openTab(view, "修订与来源");
    assertReading(view.host, /身份修订/, /补充阅读习惯/, /睡前阅读/);
    await assertDownload(view, view.host, "原始资料修订记录（诊断）", proposal);
    await view.click(button(view.host, "身份与能力"));
    const governance = document.querySelector('[role="dialog"]');
    await view.click(button(governance, "变更与设定对照"));
    assertReading(governance, /身份核心提议/, /当前读到的设定/, /补充阅读习惯/, /睡前阅读/);
    await assertDownload(view, governance, "原始身份提议记录（诊断）", proposal);
    await openTab(view, "能力申请", governance);
    await view.click(button(governance, "查看与审核"));
    assertReading(governance, /查阅书单/, /列出近期想读的书/, /便于回顾阅读计划/, /已记录交付/);
    await assertDownload(view, governance, "原始能力申请记录（诊断）", capability);
    await view.click(button(governance, "审核历史"));
    assertReading(governance, /已接受/, /同意读取书单/);
    await assertDownload(view, governance, "原始治理历史记录（诊断）", review);
    await view.click(governance.querySelector('[data-slot="dialog-close"]'));
    await view.click(button(view.host, "查看完整设定与来源"));
    const dialog = document.querySelector('[role="dialog"]');
    assertReading(dialog, /林青/, /愿意倾听/);
    await assertDownload(view, dialog, "原始角色档案（诊断）", detail);
  } finally {
    await view.dispose();
    workspace.dispose();
  }
});

test("成长认识与人格快照保留性格、表达和身份提议，原始下载保留编译资料", async () => {
  const { default: Page } = await load("/src/react-app/pages/SelfPage.tsx");
  const { headerWorkspace } = await load("/test/fixtures/governance-header-fixture.mjs");
  const workspace = headerWorkspace();
  const personality = { ...privateFields, openness: 0.92, description: "喜欢寻找新视角", update_policy: { cooldown_seconds: 98765 } };
  const compilation = {
    ...privateFields, profile_id: "warm",
    working_persona: { portrait_text: "表达版本 v8 当前协议\nresponse_style: 温和\n先听完再回答。\nmetrics: METRIC_HIDDEN" },
  };
  const proposal = {
    ...privateFields, status: "proposed", reason: "说明新的生活习惯",
    changes: { life_profile: { life_habits: "周末去图书馆" } }, created_at: when,
  };
  const claim = {
    ...privateFields, status: "active", category: "preference", claim: "喜欢晨间读书",
    confidence: 0, value: false, source_valid: false, provenance: { kind: "reflection", ...privateFields },
  };
  workspace.services.selfClient.summary = async () => ({
    core_persona: { personality }, persona_compilation: compilation,
    personality_runtime: { active_profile_id: "warm" }, foundation_revisions: [proposal],
  });
  workspace.services.selfClient.list = async () => ({ items: [claim], next_cursor: "" });
  const route = (view) => ({ fid: "A", query: new URLSearchParams({ view }), feature: { params: {} } });
  const view = await mountReact(vite, Page, { workspace, props: { route: route("") } });
  try {
    assertReading(view.host, /成长认识/, /喜欢晨间读书/, /认识类别.*偏好/, /置信度 0/, /认识内容.*否/, /形成依据.*反思/, /依赖的事实来源已变化/);
    await assertDownload(view, view.host, "原始认识记录（诊断）", claim);
    await view.update({ route: route("snapshot") });
    assertReading(view.host, /人格快照/, /稳定性格/, /开放性/, /喜欢寻找新视角/, /回复风格：温和/, /先听完再回答/, /说明新的生活习惯/, /周末去图书馆/);
    assert.doesNotMatch(view.host.textContent, /98765|喜欢晨间读书/);
    await assertDownload(view, view.host, "原始人格快照（诊断）", { personality, persona_compilation: compilation });
    await assertDownload(view, view.host, "原始身份提议记录（诊断）", proposal);
  } finally { await view.dispose(); }
});

test("认识详情以中文呈现修订和复核，诊断下载可还原证据与前后快照", async () => {
  const { default: Page } = await load("/src/react-app/pages/SelfPage.tsx");
  const { headerWorkspace } = await load("/test/fixtures/governance-header-fixture.mjs");
  const workspace = headerWorkspace();
  const claim = {
    ...privateFields, status: "active", category: "preference", claim: "偏好在安静处阅读",
    value: { description: "喜欢窗边的位置", metrics: privateFields.metrics },
    confidence: null, owner_review: { reason: "这符合实际交流", ...privateFields },
  };
  const { owner_review, ...historicalClaim } = claim;
  const revision = {
    ...privateFields, change_type: "update", reason_code: "PRIVATE_REASON_CODE",
    source_window: "PRIVATE_SOURCE_WINDOW", reason: "补充近期交流所得",
    before_value: { ...historicalClaim, claim: "偏好在家阅读" }, after_value: historicalClaim,
    evidence_refs: ["PRIVATE_EVIDENCE_REF"], created_at: when,
  };
  const review = { ...privateFields, reason: "已核对阅读习惯", created_at: when, claim_revision: 872635 };
  workspace.services.selfClient.read = async () => claim;
  workspace.services.selfClient.history = async (_fid, _id, kind) => ({ items: kind === "revisions" ? [revision] : [review], next_cursor: "" });
  const route = { fid: "A", query: new URLSearchParams(), feature: { params: { claimId: claim.id } } };
  const view = await mountReact(vite, Page, { workspace, props: { route } });
  try {
    assertReading(view.host, /偏好在安静处阅读/, /喜欢窗边的位置/, /这符合实际交流/, /修订认识/, /补充近期交流所得/, /修改前的认识.*偏好在家阅读/, /修改后的认识.*偏好在安静处阅读/, /已核对阅读习惯/, /2026-11-23 09:35 \(北京时间\)/);
    assert.doesNotMatch(view.host.textContent, /PRIVATE_(?:REASON_CODE|SOURCE_WINDOW|EVIDENCE_REF)/);
    assert.ok([...view.host.querySelectorAll("p")].every((node) => !/^置信度(?:\s|：|$)/.test(node.textContent)));
    await assertDownload(view, view.host, "原始认识记录（诊断）", claim);
    await assertDownload(view, view.host, "原始认识修订记录（诊断）", revision);
    await assertDownload(view, view.host, "原始复核记录（诊断）", review);
  } finally { await view.dispose(); }
});

test("治理读取展示能力内容、审核结果、中文时间，下载可还原申请和历史", async () => {
  const { default: Page } = await load("/src/react-app/pages/GovernancePage.tsx");
  const { headerWorkspace } = await load("/test/fixtures/governance-header-fixture.mjs");
  const workspace = headerWorkspace();
  const capability = {
    ...privateFields, status: "fulfilled", title: "查阅阅读记录", description: "回顾读过的故事",
    rationale: "避免重复推荐", desired_contract: { description: "按书名查询", ...privateFields },
    side_effect_class: "read", priority: "normal", review_note: "已核对交付", created_at: when,
    registered_capability: { available: true, version: "cap-private-v77" },
  };
  const history = { ...privateFields, status: "accepted", note: "同意这项需求", created_at: when };
  workspace.services.governanceClient.capability = async () => capability;
  workspace.services.governanceClient.history = async () => ({ items: [history], next_cursor: "" });
  const route = { fid: "A", query: new URLSearchParams(), feature: { params: { governanceKind: "capability", governanceId: capability.id } } };
  const view = await mountReact(vite, Page, { workspace, props: { route } });
  try {
    assertReading(view.host, /查阅阅读记录/, /回顾读过的故事/, /避免重复推荐/, /按书名查询/, /只读/, /已核对交付/, /已接受/, /同意这项需求/, /2026-11-23 09:35 \(北京时间\)/);
    assert.doesNotMatch(view.host.textContent, /cap-private-v77/);
    await assertDownload(view, view.host, "原始治理记录（诊断）", capability);
    await assertDownload(view, view.host, "原始治理历史记录（诊断）", history);
  } finally { await view.dispose(); }
});

test("治理身份对照和认识目录隐藏内部字段，空字段省略且保留原始记录", async () => {
  const { default: Page } = await load("/src/react-app/pages/GovernancePage.tsx");
  const { headerWorkspace } = await load("/test/fixtures/governance-header-fixture.mjs");
  const workspace = headerWorkspace();
  const current = { ...privateFields, identity: { name: "林青", biography: "", ...privateFields } };
  const proposal = {
    ...privateFields, status: "proposed", reason: "把爱好纳入档案",
    base_snapshot: { ...privateFields, identity: { occupation: "书籍编辑" } },
    changes: { identity: { occupation: "独立译者", notes: "", ...privateFields } },
  };
  const claim = {
    ...privateFields, status: "active", claim: "喜欢在晨间阅读",
    value: { description: "偏好安静阅读", source_ref: "res_hidden" }, confidence: null,
  };
  workspace.services.governanceClient.current = async () => current;
  workspace.services.governanceClient.list = async () => ({ items: [proposal], next_cursor: "" });
  workspace.services.selfClient.list = async () => ({ items: [claim], next_cursor: "" });
  const route = (tab) => ({ fid: "A", query: new URLSearchParams({ tab }), feature: { params: {} } });
  const view = await mountReact(vite, Page, { workspace, props: { route: route("foundation") } });
  try {
    assertReading(view.host, /当前生效身份/, /林青/, /提议时的设定/, /书籍编辑/, /独立译者/, /备注清空该设定/, /把爱好纳入档案/);
    assert.doesNotMatch(view.host.textContent, /生平|时间未知/);
    await assertDownload(view, view.host, "原始当前身份记录（诊断）", current);
    await assertDownload(view, view.host, "原始治理记录（诊断）", proposal);
    await view.update({ route: route("developing") });
    assertReading(view.host, /喜欢在晨间阅读/, /偏好安静阅读/);
    assert.doesNotMatch(view.host.textContent, /置信度/);
    await assertDownload(view, view.host, "原始认识记录（诊断）", claim);
  } finally { await view.dispose(); }
});
