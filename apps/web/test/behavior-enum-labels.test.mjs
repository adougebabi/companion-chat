import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { JSDOM } from "jsdom";
import React from "react";
import { createServer } from "vite";
import { mountReact } from "./helpers/react-dom.mjs";
import { profileFields, profileChanges } from "../src/approved-ui/profile-fields.mjs";
import { formatDisplayValue } from "../src/lib/fluctlight-display.ts";
import { ReadingFields } from "../src/v3/proto/components/reading-fields.js";

const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  envFile: false,
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true, ws: false },
});
test.after(() => vite.close());

const load = path => vite.ssrLoadModule("/src/" + path);
const { WorkingPortraitSummary, ProfileFieldsSummary, ProfileDiagnostic } = await load("react-app/shared/ProfileRecordSummary.jsx");
const { ReflectionChanges, RecordDownload, reflectionDownload } = await load("react-app/pages/MemoryRecordSummary.jsx");
const { configureDomainService } = await load("v3/proto/services.js");
const { renderRealReflection } = await load("v3-domain/reflection.js");
const h = React.createElement;
const hidden = /\b(?:id|revision|source_ref|idempotency_key|metrics)\b|res_enum_hidden|907234|origin-hidden|replay-hidden|metric-hidden|尚未记录|版本/;
const technical = {
  id: "res_enum_hidden", revision: 907234, source_ref: "origin-hidden",
  idempotency_key: "replay-hidden", metrics: "metric-hidden",
};

function button(host, text) {
  const node = [...host.querySelectorAll("button")].find(item => item.textContent.trim() === text);
  assert.ok(node, text);
  return node;
}

async function assertReactDownload(view, title, expected) {
  await view.click(button(view.host, title));
  const create = URL.createObjectURL;
  const prototype = window.HTMLAnchorElement.prototype;
  const click = prototype.click;
  let blob, filename;
  try {
    URL.createObjectURL = value => { blob = value; return create(value); };
    prototype.click = function () { filename = this.download; };
    await view.click(button(view.host, "下载原始记录"));
    assert.equal(filename, "fluctlight-record.json");
    assert.deepEqual(JSON.parse(await blob.text()), expected);
    assert.equal(view.host.querySelector("pre, code, [data-testid=record-view]"), null);
  } finally {
    URL.createObjectURL = create;
    prototype.click = click;
  }
}

test("表达画像翻译已知代号，为未知代号显示兜底，完整原文可下载", async () => {
  const portrait_text = [
    "refusal_style: active",
    "用户原文 refusal_style: overseas_refusal",
    "用户原文：conflict_style：Give us time to talk",
    "communication.tone: warm",
    "initiative.mode：future-mode",
    "attachment_style: secure",
    "speech_style: Speak with clear pauses",
    "先听完再回应。",
    ...Object.entries(technical).map(([key, value]) => `用户原文 ${key}: ${value}`),
    "response_style: ",
  ].join("\n");
  const record = { working_persona: { portrait_text }, ...technical };
  const title = "原始表达画像记录（诊断）";
  const view = await mountReact(vite, () => h(React.Fragment, null,
    h(WorkingPortraitSummary, { value: record }),
    h(ProfileDiagnostic, { value: record, title }),
  ));
  try {
    const portrait = view.host.querySelector("[data-testid=current-working-portrait]");
    assert.deepEqual([...portrait.querySelectorAll("p")].map(row => row.textContent), [
      "拒绝风格：主动拒绝", "用户原文 · 拒绝风格：其他方式",
      "用户原文 · 冲突风格：Give us time to talk", "交流语气：温暖",
      "主动沟通方式：其他方式", "依恋方式：安全型", "说话风格：Speak with clear pauses", "先听完再回应。",
    ]);
    const rawCodes = /\b(?:refusal_style|conflict_style|communication\.tone|initiative\.mode|attachment_style|active|overseas_refusal|warm|future-mode|secure)\b/;
    await assertReactDownload(view, title, record);
    assert.doesNotMatch(view.host.textContent, rawCodes);
    assert.doesNotMatch(view.host.textContent, hidden);
  } finally {
    await view.dispose();
  }
});

test("结构化档案和旧字符串摘要使用相同的字段译名与兜底", async () => {
  const record = {
    ...technical,
    identity: { name: "April" },
    personality: { attachment_style: "future_attachment" },
    behavioral_policy: {
      response_style: "concise", message_length: "long", refusal_style: "active",
      punctuation_style: "natural", humor_style: "gentle", intimacy_expression: "reserved",
      conflict_style: "先听完对方的意见", support: { style: "future_support" },
      response: { style: "warm_concise" }, initiative: { mode: "active" },
      communication: { tone: "warm" }, conflict: { approach: "direct" },
    },
    personality_system: { mode: "multiple", profiles: [] },
  };
  const view = await mountReact(vite, ProfileFieldsSummary, { props: { value: record } });
  const dom = new JSDOM(ReadingFields(record));
  try {
    const facts = [
      ["回复风格", "简洁"], ["消息长度", "详细"], ["拒绝风格", "主动拒绝"],
      ["标点风格", "自然"], ["幽默风格", "温和"], ["亲密表达", "含蓄"],
      ["冲突风格", "先听完对方的意见"], ["回应方式", "温暖简洁"], ["冲突处理方式", "直接"],
      ["交流语气", "温暖"], ["主动沟通方式", "主动"], ["支持方式", "其他方式"],
      ["依恋方式", "自定义依恋方式"], ["人格模式", "多重人格"],
    ];
    for (const host of [view.host, dom.window.document.body]) {
      for (const [label, value] of facts) {
        assert.ok([...host.querySelectorAll("dt")].some(node => node.textContent === label &&
          node.nextElementSibling.textContent === value), label + " " + value);
      }
      assert.match(host.textContent, /April/);
      assert.doesNotMatch(host.textContent, /\b(?:active|concise|long|natural|gentle|reserved|warm_concise|warm|direct|multiple|future_attachment|future_support)\b/);
      assert.doesNotMatch(host.textContent, hidden);
    }
    assert.equal(formatDisplayValue({ mode: "standard", refusal_style: "active", initiative: { mode: "future_mode" } }),
      "自定义字段（mode）：standard；拒绝风格：主动拒绝；主动性：主动沟通方式：其他方式");
    const core = { identity: {}, behavioral_policy: { message_length: "future_length" }, life_profile: {} };
    const fields = profileFields(core);
    const messageLength = fields.find(field => field.path.at(-1) === "message_length");
    assert.equal(messageLength.value, "future_length");
    assert.deepEqual(messageLength.options.at(-1), { value: "future_length", label: "其他方式" });
    assert.deepEqual(profileChanges(core, fields), {});
  } finally {
    dom.window.close();
    await view.dispose();
  }
});

test("记忆中的反思和独立反思使用相同译名，诊断下载保留代号", async () => {
  const record = {
    ...technical,
    summary: "她重新考虑了怎样回应。",
    proposal: {
      behavior_policy_evolution_candidates: [
        { field_path: "response.style", semantic_value: "warm_concise" },
        { field_path: "initiative.mode", semantic_value: "unlisted_mode" },
        { field_path: "communication.tone", semantic_value: "Speak a little softer" },
      ],
    },
  };
  const view = await mountReact(vite, () => h(React.Fragment, null,
    h(ReflectionChanges, { record, fid: "fixture" }),
    h(RecordDownload, { value: record, title: reflectionDownload }),
  ));
  const dom = new JSDOM("<main></main>");
  const host = dom.window.document.querySelector("main");
  configureDomainService("reflection", {
    read: async () => record,
    feedback: async () => ({ items: [], next_cursor: "" }),
  });
  const dispose = renderRealReflection(host, { charId: "fixture", params: { refId: record.id } });
  try {
    await new Promise(resolve => setImmediate(resolve));
    await assertReactDownload(view, reflectionDownload, record);
    for (const node of [view.host, host]) {
      for (const text of ["回应方式", "温暖简洁", "主动沟通方式", "其他方式", "交流语气", "Speak a little softer"]) {
        assert.ok(node.textContent.includes(text), text);
      }
      assert.doesNotMatch(node.textContent, /response\.style|initiative\.mode|communication\.tone|unlisted_mode|warm_concise/);
      assert.doesNotMatch(node.textContent, hidden);
    }
    const link = host.querySelector("a[download]");
    assert.equal(link.textContent, reflectionDownload);
    assert.equal(link.download, "fluctlight-record.json");
    const prefix = "data:application/json;charset=utf-8,";
    assert.ok(link.href.startsWith(prefix));
    assert.deepEqual(JSON.parse(decodeURIComponent(link.href.slice(prefix.length))), record);
  } finally {
    dispose();
    dom.window.close();
    await view.dispose();
  }
});
