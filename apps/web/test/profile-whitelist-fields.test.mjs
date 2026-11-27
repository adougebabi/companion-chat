import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import React from "react";
import { createServer } from "./support/module-server.mjs";
import { installDOM, mountReact } from "./helpers/react-dom.mjs";

const dom = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  envFile: false,
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
test.after(async () => { await vite.close(); dom.restore(); });
const load = path => vite.ssrLoadModule("/src/" + path);
const source = path => readFileSync(new URL("../src/" + path, import.meta.url), "utf8");
const hidden = /隐私样例|\b(?:media_preferences|relationship_seeds|social_background|target_actor_id|profile_id|evidence_refs|source_ref|revision|schema_version|idempotency_key|implementation_key|registry_key|update_policy|metrics|run_latency)\b|尚未记录/;
const technical = {
  id: "隐私样例编号", revision: 7629, schema_version: "隐私样例协议",
  source_ref: "隐私样例引用", idempotency_key: "隐私样例去重",
  metrics: { run_latency: "隐私样例耗时" },
};
const privateKeys = { ...technical, implementation_key: "隐私样例实现", registry_key: "隐私样例注册", schema: { description: "隐私样例结构" } };

function assertFacts(root, ...facts) {
  assert.doesNotMatch(root.textContent, hidden);
  assert.equal(root.querySelector("pre, code"), null);
  for (const fact of facts) assert.match(root.textContent, fact);
}

async function assertRecord(view, title, expected) {
  const button = text => [...view.host.querySelectorAll("button")].find(node => node.textContent.trim() === text);
  assert.ok(button(title), title);
  await view.click(button(title));
  const create = URL.createObjectURL, revoke = URL.revokeObjectURL;
  const click = window.HTMLAnchorElement.prototype.click;
  let blob;
  URL.createObjectURL = data => { blob = data; return "blob:profile-fields-fixture"; };
  URL.revokeObjectURL = () => {};
  window.HTMLAnchorElement.prototype.click = function () { assert.equal(this.download, "fluctlight-record.json"); };
  try {
    assertFacts(view.host);
    await view.click(button("下载原始记录"));
    assert.deepEqual(JSON.parse(await blob.text()), expected);
  } finally {
    URL.createObjectURL = create;
    URL.revokeObjectURL = revoke;
    window.HTMLAnchorElement.prototype.click = click;
  }
}

test("生活设定恢复媒体偏好、关系起点及社交背景，完整原对象仍可下载", async () => {
  const { ProfileFieldsSummary, lifeShape, ProfileDiagnostic } = await load("react-app/shared/ProfileRecordSummary.jsx");
  // Dense initialization cards supply the wording and the report supplies the life-profile keys.
  const life = {
    ...technical,
    media_preferences: {
      moments: { frequency: "偶尔", triggers: ["修复完一页旧书"] },
      artwork: { painting_frequency: "每周一次", triggers: ["完成满意的水彩"] },
      selfies: { triggers: [{ detail: "完成高满意度作品时拍自拍", source_ref: "隐私样例自拍" }] },
    },
    relationship_seeds: [{
      target_actor_id: "actor_user", profile_id: "profile_twilight", role: { label: "挚友" },
      summary: "互相尊重的挚友", trend: "improving",
      metrics: { respect: 0.8, trust: 0, cooperation: 0.6, run_latency: "隐私样例量表" },
      evidence_refs: ["隐私样例证据"],
    }],
    social_background: { family: { background: "在古籍书店长大" }, upbringing: ["童年长期搬家"], education: null, work: { occupation: "书籍修复师" }, craft_history: { custom_detail: "记录颜料配方" } },
    appearance: { clothing_style: "", physical_features: { eyes: "金棕色眼睛" }, wardrobe_items: [{ description: "亚麻衬衫", ownership: "owned" }] },
    preferences: { creative_medium: ["画水彩", { description: "也喜欢修复旧书", ...technical }] },
    life_habits: [{ action: "每周日整理工作台" }],
    recurring_commitments: [{ condition: "每周日有空" }],
    character_constraints: [{ rule: "不喜欢被催促" }],
  };
  const Page = () => React.createElement(React.Fragment, null,
    React.createElement(ProfileFieldsSummary, { value: life, shape: lifeShape }),
    React.createElement(ProfileDiagnostic, { value: life, title: "原始生活设定记录（诊断）" }),
  );
  const view = await mountReact(vite, Page);
  try {
    assertFacts(view.host, /媒体偏好动态频率偶尔触发条件修复完一页旧书/, /画作画作频率每周一次/, /自拍触发条件.*完成高满意度作品时拍自拍/,
      /关系起点关系角色挚友.*互相尊重的挚友.*趋势改善/, /关系评估尊重80%信任0%合作程度60%/,
      /社交背景家庭背景在古籍书店长大/, /成长经历童年长期搬家/, /工作经历职业书籍修复师/, /技艺经历补充说明记录颜料配方/,
      /金棕色眼睛/, /初始衣物设定亚麻衬衫/, /偏好.*画水彩.*也喜欢修复旧书/, /生活习惯行动每周日整理工作台/,
      /固定承诺条件每周日有空/, /性格边界.*不喜欢被催促/);
    assert.doesNotMatch(view.host.textContent, /actor_user|profile_twilight|custom_detail|painting_frequency|creative_medium|7629|教育经历/);
    await assertRecord(view, "原始生活设定记录（诊断）", life);
    assert.match(source("react-app/views/CharacterDetailPage.jsx"), /value=\{core\.life_profile\} shape=\{lifeShape\}/);
  } finally { await view.dispose(); }
});

test("独立治理与档案治理共用能力契约摘要和完整诊断下载", async () => {
  const { default: Page } = await load("react-app/pages/GovernancePage.tsx");
  const { headerWorkspace } = await vite.ssrLoadModule("/test/fixtures/governance-header-fixture.mjs");
  const workspace = headerWorkspace();
  const record = {
    id: "test-capability", status: "fulfilled", title: "查阅日历",
    desired_contract: {
      title: "日历查询契约", summary: "查找忙碌时间",
      input: { date_range: { description: "未来七天", ...privateKeys } },
      output: "busy_intervals", limits: ["每次最多查询七天"],
      constraints: { read_only: true }, preconditions: "所有者已授权",
      examples: [{ request_example: "查询本周日的安排" }],
      error_handling: { timeout: "等待超时后提示稍后再试" },
      ...privateKeys,
    },
  };
  workspace.services.governanceClient.capability = async () => record;
  workspace.services.governanceClient.history = async () => ({ items: [], next_cursor: "" });
  const route = { fid: "A", query: new URLSearchParams(), feature: { params: { governanceKind: "capability", governanceId: record.id } } };
  const view = await mountReact(vite, Page, { workspace, props: { route } });
  try {
    assertFacts(view.host, /期望能力.*日历查询契约.*查找忙碌时间/, /输入日期范围说明未来七天/, /输出忙碌时段/,
      /限制每次最多查询七天/, /约束只读是/, /前置条件所有者已授权/, /示例请求示例查询本周日的安排/, /错误处理超时等待超时后提示稍后再试/);
    assert.doesNotMatch(view.host.textContent, /\b(?:busy_intervals|date_range|error_handling|preconditions|read_only)\b|7629/);
    await assertRecord(view, "原始治理记录（诊断）", record);
    const profileGovernance = source("react-app/shared/ProfileGovernance.jsx");
    assert.match(profileGovernance, /<CapabilitySummary record=\{/);
    assert.match(profileGovernance, /原始能力申请记录（诊断）/);
  } finally { await view.dispose(); }
});

test("角色档案、身份提议、自我认知与人格快照恢复结构化设定", async () => {
  const { ProfileFieldsSummary, ClaimValueSummary, personalityShape } = await load("react-app/shared/ProfileRecordSummary.jsx");
  const personality = { core_mechanisms: { coping_style: "先安静一下" }, update_policy: { reason: "隐私样例规则" }, ...technical };
  const settings = {
    ...technical, personality,
    identity: { education: { school: "工艺学校" } },
    behavioral_policy: { output_preferences: { frequency: "每周分享一次" }, behavior_loops: { loops: ["检查门锁"] } },
    personality_system: {
      forced_activation: { condition: "公开质疑时由星火接管" },
      takeover_rules: [{ condition: "公开质疑", target_profile_id: "profile_spark", ...technical }],
      profiles: [{ name: "暮光", stable_motivations: ["保护宁静生活"] }],
    },
  };
  const Page = () => React.createElement(React.Fragment, null,
    React.createElement(ProfileFieldsSummary, { value: settings }),
    React.createElement(ProfileFieldsSummary, { value: personality, shape: personalityShape, title: "稳定性格" }),
    React.createElement(ClaimValueSummary, { value: { boundaries: ["先征询我的意见"], ...technical } }),
  );
  const view = await mountReact(vite, Page);
  try {
    assertFacts(view.host, /教育经历学校工艺学校/, /性格与行为机制.*先安静一下/, /表达偏好频率每周分享一次/,
      /反复出现的习惯习惯描述检查门锁/, /强制切换条件条件公开质疑时由星火接管/, /接管条件条件公开质疑/,
      /人格档案名称暮光长期动机保护宁静生活/, /稳定性格.*先安静一下/, /认识内容.*先征询我的意见/);
    assert.doesNotMatch(view.host.textContent, /profile_spark|coping_style|7629/);
    assert.match(source("react-app/pages/SelfPage.tsx"), /shape=\{personalityShape\} title="稳定性格"/);
    assert.match(source("react-app/pages/GovernancePage.tsx"), /value=\{record\.changes\} title="提议变更" showClears/);
  } finally { await view.dispose(); }
});

test("目标修订、目标规划和记忆来源显示补回的内容", async () => {
  const { goalRecordSummary } = await load("approved-ui/goal-record-presentation.mjs");
  const { memoryFacts, memorySourceFacts } = await load("approved-ui/memory-record-summary.mjs");
  const { SummaryFacts, ClaimSummary } = await load("react-app/pages/MemoryRecordSummary.jsx");
  const goal = goalRecordSummary({ snapshot: { motivation: "想修复祖父留下的书", ...technical } }, "revision");
  const plan = goalRecordSummary({ result: { decision: "suggestions", suggestions: [{
    title: "修复旧书", motivation: "保留家人的笔迹", success_criteria: ["保护书脊"], next_step: "核对纸质", wait_condition: "等胶水干透", ...technical,
  }] } }, "planning");
  const facts = [...goal.facts, ...plan.facts, ...memoryFacts({ importance: 0 }), ...memorySourceFacts({ description: "当时写在书签背面", ...technical })];
  const Page = () => React.createElement(React.Fragment, null,
    React.createElement(SummaryFacts, { facts }),
    React.createElement(ClaimSummary, { record: { value: { boundaries: ["读书时请勿打扰"], ...technical } } }),
  );
  const view = await mountReact(vite, Page);
  try {
    assertFacts(view.host, /动机想修复祖父留下的书/, /修复旧书 · 动机保留家人的笔迹/, /完成条件保护书脊/,
      /下一步核对纸质/, /等待条件等胶水干透/, /重要程度0%/, /来源说明当时写在书签背面/, /认识内容补充边界读书时请勿打扰/);
    assert.match(source("v3-domain/goal-planning.js"), /kind:'planning'/);
    assert.match(source("v3-domain/memory.js"), /memorySourceFacts\(/);
  } finally { await view.dispose(); }
});

test("世界物品分类、世界事件身体影响以中文显示", async () => {
  const { WorldItemCard } = await load("v3/proto/components/world-items.js");
  const { EventDefinitionSummary } = await load("v3/proto/components/world-event-runtime.js");
  const item = { ...technical, name: "登山外套", definition: { item_kind: "wearable", category: "outdoor_clothing", slot: "outer", description: "防风外套" } };
  const event = { ...technical, effects: [{ kind: "temporary_injury", body_effect: { description: "短暂擦伤", body_part: "hand", severity: "minor", impact: "pain", ...technical } }] };
  const host = document.createElement("div");
  host.innerHTML = WorldItemCard(item, "test-character") + EventDefinitionSummary(event);
  assertFacts(host, /物品分类：户外服装/, /短暂擦伤/, /身体部位手部/, /严重程度轻微/, /影响疼痛/);
  assert.doesNotMatch(host.textContent, /outdoor_clothing|body_part|minor|7629/);
  const link = host.querySelector("a[download]");
  assert.equal(link.textContent, "原始世界物品记录（诊断）");
  assert.deepEqual(JSON.parse(decodeURIComponent(link.getAttribute("href").split(",")[1])), item);
});

test("此刻、日程和视觉身份使用限定的设定摘要字段", async () => {
  const { default: VisualIdentityRecord } = await load("react-app/shared/VisualIdentityRecord.jsx");
  const value = {
    ...technical, identity_snapshot: { life_profile: { appearance: { description: "栗色卷发", ...technical } } },
    session: { current_attempt: 2, attempts: [{ attempt_number: 2, review: { feedback: "保留自然肤色", observations: ["衣服颜色偏深"], ...technical } }] },
    generated_reviews: { avatar: { feedback: "头像需要更柔和", ...technical }, character_sheet: { feedback: "保留工装短裤", ...technical } },
  };
  const view = await mountReact(vite, VisualIdentityRecord, { props: { value } });
  try {
    assertFacts(view.host, /视觉身份外貌外貌说明栗色卷发/, /候选评审意见反馈保留自然肤色评审观察衣服颜色偏深/, /头像评审意见反馈头像需要更柔和/,
      /角色设定图评审意见反馈保留工装短裤/, /原始媒体记录（诊断）/);
    const visual = source("react-app/shared/VisualWorkspace.jsx");
    assert.match(visual, /briefFields\.flatMap\(\(\[key, label\]\) => \{[\s\S]*?row\.before_value\?\.\[key\][\s\S]*?row\.after_value\?\.\[key\]/);
    assert.match(visual, /<VisualBriefRevision row=\{row\} \/>[\s\S]*?value=\{row\} downloadOnly/);
    assert.match(visual, /<VisualReviewReading value=\{row\.review\} title="评审意见" \/>/);
    const now = source("react-app/views/NowPage.jsx");
    assert.match(now, /value=\{c\.detail\?\.current_state\?\.appearance\} shape=\{currentStateShape\.appearance\} title="此刻的外貌"/);
    assert.match(now, /readingFields\("label description reason summary"\)/);
    assert.match(now, /shape=\{innerStateShape\} title="内在状态"/);
    const schedule = source("react-app/pages/SchedulePage.jsx");
    assert.match(schedule, /readingFields\("description reason desired_hair_color desired_hair_length duration_minutes"\)/);
    assert.match(schedule, /items: readingFields\("description name"\)/);
    assert.match(schedule, /value=\{selected\.item\._source\.action_plan\} shape=\{scheduleActionShape\} title="行动安排"/);
  } finally { await view.dispose(); }
});

test("页面正文使用补回的业务字段，诊断入口继续使用原对象", () => {
  const person = source("react-app/shared/PersonDetail.jsx");
  assert.match(person, /label="已读在场时段"[\s\S]*?userInstant\(npc\.current_presence\.start_at\)[\s\S]*?userInstant\(npc\.current_presence\.end_at\)/);
  const notice = source("react-app/pages/InboxPage.jsx");
  assert.match(notice, /row\.quiet === true && ' · 静默通知'/);
  assert.match(notice, /value=\{row\} title="原始通知记录（诊断）" downloadOnly/);
  const feed = source("react-app/shared/FeedPost.jsx");
  assert.match(feed, /owner: "仅所有者可见", participants: "参与者可见"/);
  assert.match(feed, /可见范围：\{visibilityLabels\[post\.visibility\]\}[\s\S]*?value=\{momentRecord\(post\)\}/);
  const diary = source("react-app/pages/DiaryPage.jsx");
  assert.match(diary, /const summary = row\.result\?\.summary;/);
  assert.match(diary, /<ProfileNote value=\{summary\} label="结果说明" \/>[\s\S]*?value=\{row\} title="原始留言处理记录（诊断）"/);
  const story = source("v3-domain/diary.js");
  assert.match(story, /const relationshipRole=storyText\(typeof relationship\?\.role === "string" \? relationship\.role : relationship\?\.role\?\.label\)/);
  assert.match(story, /关系称谓：\$\{esc\(relationshipRole\)\}/);
  const wardrobe = source("react-app/views/WardrobePage.jsx");
  assert.match(wardrobe, /value\?\.category && recordText\(wardrobeCategoryLabel\(value\.category\)\)/);
  assert.match(wardrobe, /"服装分类", clothingCategory\(value\)/);
  assert.match(wardrobe, /"服装分类", clothingCategory\(draft\._source\)/);
  const place = source("v3/proto/components/place-defaults.js");
  assert.match(place, /readingFields\('quantity position placement material color condition notes details'\)/);
  assert.match(place, /historyCollectionDetails\(previous\)[\s\S]*?historyCollectionDetails\(next\)[\s\S]*?变更后补充/);
  assert.match(place, /RawRecordDownload\(row,'原始地点修订记录（诊断）'\)/);
  const request = source("v3-domain/operations.js");
  assert.match(request, /readingFields\("description notes expected_outcome location scene desired_hair_color desired_hair_length"\)/);
  assert.match(request, /ReadingFields\(payload,requestContentShape\)/);
  assert.match(request, /'期望时间',\[requestDetailTime\(payload\?\.start_at\),requestDetailTime\(payload\?\.end_at\)\]/);
  const reflection = source("v3-domain/reflection.js");
  assert.match(reflection, /capability_constraints:'能力约束'/);
  assert.match(reflection, /'重要程度',scoreText\(candidate\?\.importance\)/);
  assert.match(reflection, /'情感意义',scoreText\(candidate\?\.emotional_significance\)/);
  const memoryReflection = source("react-app/pages/MemoryRecordSummary.jsx");
  assert.match(memoryReflection, /"能力约束", textList\(candidate\.capability_constraints\)/);
  assert.match(memoryReflection, /"重要程度", scoreText\(candidate\.importance\)/);
  assert.match(memoryReflection, /"情感意义", scoreText\(candidate\.emotional_significance\)/);
});
