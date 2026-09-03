import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const displaySource = await readFile(new URL("../src/lib/fluctlight-display.ts", import.meta.url), "utf8");
const detailSource = await readFile(new URL("../src/components/instances/InstanceDetailsDialog.vue", import.meta.url), "utf8");
const governanceSource = await readFile(new URL("../src/views/GovernanceView.vue", import.meta.url), "utf8");
const settingsSource = await readFile(new URL("../src/views/SettingsView.vue", import.meta.url), "utf8");
const stylesSource = await readFile(new URL("../src/styles/app.css", import.meta.url), "utf8");

test("detail display has safe object formatting and Chinese field labels", () => {
  assert.match(displaySource, /export function formatDisplayValue/);
  assert.match(displaySource, /Object\.entries\(value as JsonRecord\)/);
  assert.doesNotMatch(detailSource, /function valueText\(value: unknown\).*String\(value/);
  assert.doesNotMatch(governanceSource, /function stringify\(value: unknown\)/);
  assert.match(displaySource, /education: "教育经历"/);
  assert.match(displaySource, /fashion_preference: "穿衣偏好"/);
  assert.match(displaySource, /physical_attributes: "外貌特征"/);
  assert.match(displaySource, /core_values: "核心价值观"/);
  assert.match(displaySource, /worldview: "世界观"/);
  assert.match(displaySource, /key\.replace\(\/\(\[a-z\\d\]\)\(\[A-Z\]\)/);
  assert.match(displaySource, /return translatedParts\.join\(""\)/);
  assert.match(displaySource, /value_schema: "取值规则"/);
  assert.match(displaySource, /physical_attributes: "外貌特征"/);
  assert.match(displaySource, /enumLabels\[value\] \?\? "未分类"/);
  assert.match(displaySource, /export function isCustomLabel/);
  assert.match(displaySource, /rawLabel = displayLabel === "自定义字段"/);
  assert.match(detailSource, /class="raw-field-name"/);
  assert.match(governanceSource, /字段名：\{\{ String\(slot\.key\) \}\}/);
});

test("detail owns read-only life-world sections and uses timezone-aware timeline formatting", () => {
  assert.match(detailSource, /人格特征/);
  assert.match(detailSource, /目标与意图/);
  assert.match(detailSource, /今日日程/);
  assert.match(detailSource, /formatZonedRange\(item\.start_at, item\.end_at, scheduleTimezone\)/);
  assert.match(detailSource, /formatTimelineTime\(event\.occurred_at, scheduleTimezone\)/);
  assert.match(displaySource, /export function formatTimelineTime/);
  assert.match(detailSource, /resolveTimezone/);
  assert.match(detailSource, /关系状态/);
  assert.match(detailSource, /可展示记忆/);
  assert.doesNotMatch(governanceSource, /<p class="eyebrow">IDENTITY<\/p>/);
  assert.doesNotMatch(governanceSource, /<p class="eyebrow">LIFE WORLD<\/p>/);
  assert.match(governanceSource, /formatZonedRange\(event\.start_at, event\.end_at/);
});

test("detail exposes a Chinese progressive state drawer", () => {
  assert.match(detailSource, /<details class="detail-state-drawer">/);
  assert.match(detailSource, /状态数值与当前氛围/);
  assert.match(detailSource, /愉悦度/);
  assert.match(detailSource, /唤醒度/);
  assert.match(detailSource, /掌控感/);
  assert.match(detailSource, /行动动能/);
  assert.match(detailSource, /调节稳定性/);
  assert.match(detailSource, /人格动力与偏好/);
  assert.match(detailSource, /暂无独立氛围描述/);
  assert.match(stylesSource, /\.detail-state-drawer/);
  assert.match(stylesSource, /\.state-metric-grid/);
});

test("settings accordion remounts when the selected section changes", () => {
  assert.match(settingsSource, /<Accordion :key="currentSection"[^>]*:default-value="currentSection"/);
});

test("PC shell and role tabs do not scroll the outer frame", () => {
  assert.match(stylesSource, /body:has\(\.desktop-workspace\) \{ overflow: hidden;/);
  assert.match(stylesSource, /\.settings-section \.role-switcher \{[\s\S]*overflow-x: hidden;[\s\S]*overflow-y: hidden;/);
});

test("schedule timeline highlights the item active at the current instant", () => {
  assert.match(detailSource, /function isCurrentScheduleItem\(item: JsonRecord\)/);
  assert.match(detailSource, /:class="\{ active: isCurrentScheduleItem\(item\) \}"/);
  assert.match(detailSource, /class="timeline-now-badge">进行中/);
});

test("model run diagnostics show the server creation time", async () => {
  const diagnosticsSource = await readFile(new URL("../src/views/DiagnosticsView.vue", import.meta.url), "utf8");
  assert.match(diagnosticsSource, /function formatRunTime\(value: string\)/);
  assert.match(diagnosticsSource, /<time class="diagnostic-time" :datetime="run\.createdAt">/);
});
