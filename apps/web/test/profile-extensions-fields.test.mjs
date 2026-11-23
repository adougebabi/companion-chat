import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import React from "react";
import { createServer } from "vite";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { profileDetail } from "./support/native-profile-fixture.mjs";

const dom = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  envFile: false,
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
test.after(async () => { await vite.close(); dom.restore(); });
const load = path => vite.ssrLoadModule(path);

const technical = {
  id: "内部样例编号", version: "内部样例版本", revision: 912345,
  schema: { description: "内部样例结构" }, protocol: "内部样例协议",
  updatePolicy: { description: "内部样例更新规则" }, source: { description: "内部样例来源" },
  hash: "内部样例摘要", source_ref: "内部样例引用",
  idempotency_key: "内部样例去重", metrics: { latency: "内部样例耗时" },
};
const forbidden = /内部样例|912345|尚未记录|\b(?:extensions|id|version|revision|schema|protocol|updatePolicy|source|hash|source_ref|idempotency_key|metrics|opaque_namespace|one_off_detail|another_detail)\b/;

function assertReading(root, ...facts) {
  assert.doesNotMatch(root.textContent, forbidden);
  assert.equal(root.querySelector("pre, code"), null);
  for (const fact of facts) assert.match(root.textContent, fact);
}

function findButton(root, label) {
  const button = [...root.querySelectorAll("button")].find(node => node.textContent.trim() === label);
  assert.ok(button, label);
  return button;
}

async function assertDownload(view, root, title, expected) {
  const trigger = findButton(root, title);
  await view.click(trigger);
  const disclosure = trigger.closest('[data-slot="collapsible"]');
  const { createObjectURL, revokeObjectURL } = URL;
  const anchor = window.HTMLAnchorElement.prototype;
  const { click } = anchor;
  let blob, filename;
  URL.createObjectURL = value => { blob = value; return "blob:profile-extensions-fixture"; };
  URL.revokeObjectURL = () => {};
  anchor.click = function () { filename = this.download; };
  try {
    assertReading(disclosure);
    await view.click(findButton(disclosure, "下载原始记录"));
    assert.equal(filename, "fluctlight-record.json");
    assert.equal(blob.type, "application/json");
    assert.deepEqual(JSON.parse(await blob.text()), expected);
  } finally {
    URL.createObjectURL = createObjectURL;
    URL.revokeObjectURL = revokeObjectURL;
    anchor.click = click;
  }
}

test("档案扩展按原顺序展示中文标签和补充说明，空值和内部字段仅存在于原始下载", async () => {
  const { ProfileDiagnostic, ProfileExtensionsSummary, ProfileFieldsSummary } = await load("/src/react-app/shared/ProfileRecordSummary.jsx");
  const record = {
    identity: { name: "林青", unknown_outside: "正文之外的普通字段" },
    extensions: {
      occupation: "修复师",
      读书记录: { one_off_detail: "第一条补充", another_detail: "第二条补充" },
      opaque_namespace: {
        description: "未知命名空间的简介",
        ...technical,
        note: ["保留 <手稿> & 批注", null, { description: "附页", ...technical }, [], {}],
        empty_null: null, empty_text: "  ", empty_object: {}, empty_array: [],
        empty_namespace: { ...technical },
        enabled: false, score: 0,
        extensions: { summary: "更深层的摘要" },
      },
    },
    personality: { core_mechanisms: { extensions: { summary: "嵌套扩展也可阅读" } } },
    source: { extensions: { summary: "内部样例来源扩展" } },
  };
  const title = "原始角色档案（诊断）";
  const Page = () => React.createElement(React.Fragment, null,
    React.createElement(ProfileFieldsSummary, { value: record }),
    React.createElement(ProfileExtensionsSummary, { value: record }),
    React.createElement(ProfileDiagnostic, { value: record, title }),
  );
  const view = await mountReact(vite, Page);
  try {
    assertReading(view.host, /林青/, /职业修复师/, /读书记录补充说明第一条补充补充说明第二条补充/,
      /补充说明说明未知命名空间的简介/, /保留 <手稿> & 批注说明附页/, /否.*0/,
      /摘要更深层的摘要/, /摘要嵌套扩展也可阅读/);
    assert.doesNotMatch(view.host.textContent, /正文之外的普通字段/);
    assert.ok([...view.host.querySelectorAll("dd")].every(node => node.textContent.trim()));
    await assertDownload(view, view.host, title, record);
  } finally { await view.dispose(); }
});

test("完整角色档案在正文显示扩展并下载同一记录", async () => {
  const fid = "fixture-profile-extensions";
  location.hash = "#/c/" + fid + "/profile";
  const { createWorkspace } = await load("/src/state/workspace.mjs");
  const store = await load("/src/v3/proto/store.js");
  const { default: Page } = await load("/src/react-app/pages/CharacterDetailPage.jsx");
  const detail = {
    ...profileDetail({ id: fid, name: "林青" }),
    extensions: { summary: "练习纸张修复", ...technical },
  };
  const workspace = createWorkspace();
  store.replaceSnapshot({
    ...store.get(), session: { actorId: "fixture-owner" },
    characters: { [fid]: { id: fid, name: "林青", status: "active" } }, ui: { charId: fid },
  });
  workspace.client.fetcher = async () => { throw Error("Unexpected live request"); };
  workspace.client.detail = async () => detail;
  workspace.client.world = async () => ({ world: null });
  workspace.client.governancePage = async () => ({ items: [], next_cursor: "" });
  workspace.client.selfClaims = async () => ({ items: [], next_cursor: "" });
  workspace.loadWardrobe = async () => {};
  const view = await mountReact(vite, Page, { workspace });
  try {
    const tabs = [...view.host.querySelectorAll('[role="tab"]')];
    const tab = tabs.find(node => node.textContent.trim().startsWith("修订与来源"));
    assert.ok(tab, tabs.map(node => node.textContent).join("、"));
    await act(async () => tab.dispatchEvent(new window.MouseEvent("mousedown", { bubbles: true, button: 0 })));
    await view.flush();
    await view.click(findButton(view.host, "查看完整设定与来源"));
    const dialog = document.querySelector('[role="dialog"]');
    assertReading(dialog, /林青/, /摘要练习纸张修复/);
    await assertDownload(view, dialog, "原始角色档案（诊断）", detail);
  } finally {
    await view.dispose();
    workspace.dispose();
  }
});

test("成长认识、人格快照与身份提议都保留扩展正文和原始下载", async () => {
  const { default: Page } = await load("/src/react-app/pages/SelfPage.tsx");
  const { headerWorkspace } = await load("/test/fixtures/governance-header-fixture.mjs");
  const workspace = headerWorkspace();
  const personality = { description: "性格沉着", extensions: { summary: "喜欢独立思考", ...technical } };
  const compilation = { working_persona: { extensions: { description: "说话留有余地" } } };
  const proposal = {
    id: "fixture-proposal", status: "proposed", reason: "补足生活习惯",
    changes: { life_profile: { life_habits: "早餐后散步", extensions: { one_off_detail: "沿河慢走", ...technical } } },
    extensions: { summary: "来自近期观察", empty_text: "" },
  };
  const claim = {
    id: "fixture-claim", category: "preference", claim: "偏好在安静处读书",
    value: { description: "常在窗边阅读", extensions: { description: "重视自然光", ...technical } },
    extensions: { summary: "会随季节调整" },
  };
  workspace.services.selfClient.summary = async () => ({
    core_persona: { personality }, persona_compilation: compilation, foundation_revisions: [proposal],
  });
  workspace.services.selfClient.list = async () => ({ items: [claim], next_cursor: "" });
  const route = view => ({ fid: "A", query: new URLSearchParams({ view }), feature: { params: {} } });
  const view = await mountReact(vite, Page, { workspace, props: { route: route("") } });
  try {
    assertReading(view.host, /常在窗边阅读/, /说明重视自然光/, /摘要会随季节调整/);
    await assertDownload(view, view.host, "原始认识记录（诊断）", claim);
    await view.update({ route: route("snapshot") });
    assertReading(view.host, /性格沉着/, /摘要喜欢独立思考/, /说明说话留有余地/,
      /早餐后散步/, /补充说明沿河慢走/, /摘要来自近期观察/);
    await assertDownload(view, view.host, "原始人格快照（诊断）", { personality, persona_compilation: compilation });
    await assertDownload(view, view.host, "原始身份提议记录（诊断）", proposal);
  } finally { await view.dispose(); }
});

test("独立治理页展示身份提议和生效历史的扩展正文", async () => {
  const { default: Page } = await load("/src/react-app/pages/GovernancePage.tsx");
  const { headerWorkspace } = await load("/test/fixtures/governance-header-fixture.mjs");
  const workspace = headerWorkspace();
  const proposal = {
    id: "fixture-foundation", status: "accepted", reason: "补充学习经历",
    changes: { identity: { education: { school: "工艺学校" }, extensions: { description: "学习修书" } } },
    extensions: { summary: "新增修复课程", ...technical },
  };
  const history = { id: "fixture-history", action: "accept", extensions: { description: "课程已确认", ...technical } };
  workspace.services.governanceClient.foundation = async () => proposal;
  workspace.services.governanceClient.history = async () => ({ items: [history], next_cursor: "" });
  const route = {
    fid: "A", query: new URLSearchParams(),
    feature: { params: { governanceKind: "foundation", governanceId: proposal.id } },
  };
  const view = await mountReact(vite, Page, { workspace, props: { route } });
  try {
    assertReading(view.host, /教育经历学校工艺学校/, /说明学习修书/, /摘要新增修复课程/, /说明课程已确认/);
    await assertDownload(view, view.host, "原始治理记录（诊断）", proposal);
    await assertDownload(view, view.host, "原始治理历史记录（诊断）", history);
  } finally { await view.dispose(); }
});
