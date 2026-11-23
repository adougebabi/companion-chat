import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { createServer } from "vite";
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
test.after(async () => { await vite.close(); env.restore(); });

const hidden = /\b(?:id|revision|source_ref|idempotency_key|metrics|run_latency)\b|record-private|report-private|retry-private|latency-private|93451|版本|尚未记录|\b0\.[15]\b/;
const technical = {
  id: "record-private",
  revision: 93451,
  source_ref: "report-private",
  idempotency_key: "retry-private",
  metrics: { run_latency: "latency-private" },
};

function button(root, label) {
  const control = [...root.querySelectorAll("button")].find(node => node.textContent.trim() === label);
  assert.ok(control, `缺少按钮 ${label}`);
  return control;
}

async function openTab(view, label) {
  const tab = [...view.host.querySelectorAll('[role="tab"]')].find(node => node.textContent.startsWith(label));
  assert.ok(tab, `缺少标签 ${label}`);
  await act(async () => tab.dispatchEvent(new window.MouseEvent("mousedown", { bubbles: true, button: 0 })));
  await view.flush();
  return document.getElementById(tab.getAttribute("aria-controls"));
}

async function assertDownload(view, root, title, expected) {
  const trigger = button(root, title);
  await view.click(trigger);
  const disclosure = trigger.closest('[data-slot="collapsible"]');
  assert.doesNotMatch(disclosure.textContent, hidden);
  assert.equal(disclosure.querySelector("pre, code"), null);

  const create = URL.createObjectURL;
  const revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype;
  const click = anchor.click;
  let blob, filename;
  URL.createObjectURL = value => { blob = value; return "blob:portrait-degree"; };
  URL.revokeObjectURL = () => {};
  anchor.click = function () { filename = this.download; };
  try {
    await view.click(button(disclosure, "下载原始记录"));
    assert.equal(filename, "fluctlight-record.json");
    assert.equal(blob.type, "application/json");
    assert.deepEqual(JSON.parse(await blob.text()), expected);
  } finally {
    URL.createObjectURL = create;
    URL.revokeObjectURL = revoke;
    anchor.click = click;
  }
}

async function mountProfile(overrides) {
  const fid = "fixture-role";
  location.hash = "#/c/" + fid + "/profile";
  const { default: Page } = await vite.ssrLoadModule("/src/react-app/pages/CharacterDetailPage.jsx");
  const { createWorkspace } = await vite.ssrLoadModule("/src/state/workspace.mjs");
  const store = await vite.ssrLoadModule("/src/v3/proto/store.js");
  const detail = {
    ...profileDetail({ id: fid, name: "林青" }),
    ...overrides,
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
  return { view, detail, workspace };
}

test("角色档案用中文描述表达画像和表达设定，心情零强度显示数字，诊断下载保留原值", async () => {
  const { view, detail, workspace } = await mountProfile({
    behavioral_policy: {
      ...technical,
      response_delay: 0,
      expression: { emoji_frequency: 0.1, initiative: "0.5", silence_tolerance: null },
    },
    persona_compilation: {
      ...technical,
      profile_id: "warm",
      working_persona: {
        portrait_text: "emoji_frequency: 0.1\ninitiative: 0.5\nresponse_delay: 0\nsource_ref: report-private",
        expression: { emoji_frequency: 0.1, initiative: 0.5, response_delay: 0 },
      },
    },
    current_state: { ...technical, mood: { description: "放松", intensity: 0, ...technical } },
    inner_state: { mood: { description: "安静", intensity: "0" } },
  });
  try {
    const portrait = view.host.querySelector('[data-testid="current-working-portrait"]');
    assert.deepEqual([...portrait.querySelectorAll("p")].map(row => row.textContent), [
      "表情频率：很少使用表情", "主动性：适中", "回复延迟：即时回复",
    ]);
    const overview = view.host.querySelector('[role="tabpanel"]');
    assert.doesNotMatch(overview.textContent, hidden);
    assert.match(overview.textContent, /表达与相处.*回复延迟即时回复.*表情频率很少使用表情.*主动性适中/);
    assert.doesNotMatch(overview.textContent, /沉默容忍度/);
    await assertDownload(view, overview, "原始表达画像记录（诊断）", detail.persona_compilation);

    const life = await openTab(view, "生活与此刻");
    assert.doesNotMatch(life.textContent, hidden);
    assert.doesNotMatch(life.textContent, /强度\s*平稳/);
    assert.match(life.textContent, /心情说明安静强度0(?![\d.%％])/);
    assert.match(life.textContent, /实际外观与状态心情说明放松强度0(?![\d.%％])/);
    await assertDownload(view, life, "原始当前状态记录（诊断）", detail.current_state);

    const history = await openTab(view, "修订与来源");
    await view.click(button(history, "查看完整设定与来源"));
    const fullProfile = document.querySelector('[role="dialog"]');
    assert.doesNotMatch(fullProfile.textContent, hidden);
    await assertDownload(view, fullProfile, "原始角色档案（诊断）", detail);
  } finally {
    await view.dispose();
    workspace.dispose();
  }
});

test("角色档案按原精度显示非零心情强度", async () => {
  const { view, detail, workspace } = await mountProfile({
    inner_state: { mood: { description: "安静", intensity: 0.6, ...technical } },
    current_state: { mood: { description: "放松", intensity: "0.600", ...technical } },
  });
  try {
    const life = await openTab(view, "生活与此刻");
    assert.doesNotMatch(life.textContent, hidden);
    const intensityRows = [...life.querySelectorAll("dt")].filter(label => label.textContent === "强度");
    assert.deepEqual(intensityRows.map(label => label.nextElementSibling?.textContent), ["0.6", "0.600"]);
    await assertDownload(view, life, "原始当前状态记录（诊断）", detail.current_state);
  } finally {
    await view.dispose();
    workspace.dispose();
  }
});

test("表达画像按五档边界转换倾向值，并按秒或分钟描述延迟", async () => {
  const { WorkingPortraitSummary, describeProfileExpression } = await vite.ssrLoadModule("/src/react-app/shared/ProfileRecordSummary.jsx");
  const bounds = [0, 0.19, 0.2, 0.39, 0.4, 0.59, 0.6, 0.79, 0.8, 1];
  const delays = ["0", "0.5", "5", "60", "95", "有空时回复"];
  const portrait_text = [
    ...bounds.map(score => `initiative: ${score}`),
    ...delays.map(delay => `response_delay: ${delay}`),
  ].join("\n");
  const view = await mountReact(vite, WorkingPortraitSummary, {
    props: { value: { portrait_text }, localizePolicy: true },
  });
  try {
    assert.deepEqual([...view.host.querySelectorAll("p")].map(row => row.textContent), [
      "主动性：很低", "主动性：很低", "主动性：较低", "主动性：较低", "主动性：适中",
      "主动性：适中", "主动性：较高", "主动性：较高", "主动性：很高", "主动性：很高",
      "回复延迟：即时回复", "回复延迟：不到 1 秒", "回复延迟：约 5 秒", "回复延迟：约 1 分钟",
      "回复延迟：约 1 分钟 35 秒", "回复延迟：有空时回复",
    ]);
    assert.deepEqual(describeProfileExpression({ initiative: 1.2, directness: -0.2, response_delay: "随情境调整" }),
      { initiative: 1.2, directness: -0.2, response_delay: "随情境调整" });
  } finally {
    await view.dispose();
  }
});
