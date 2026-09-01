import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const displaySource = await readFile(new URL("../src/lib/fluctlight-display.ts", import.meta.url), "utf8");
const detailSource = await readFile(new URL("../src/components/instances/InstanceDetailsDialog.vue", import.meta.url), "utf8");
const governanceSource = await readFile(new URL("../src/views/GovernanceView.vue", import.meta.url), "utf8");
const settingsSource = await readFile(new URL("../src/views/SettingsView.vue", import.meta.url), "utf8");

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
  assert.match(displaySource, /value_schema: "取值规则"/);
  assert.match(displaySource, /physical_attributes: "外貌特征"/);
  assert.match(displaySource, /enumLabels\[value\] \?\? "未分类"/);
});

test("detail owns read-only life-world sections and uses timezone-aware timeline formatting", () => {
  assert.match(detailSource, /人格特征/);
  assert.match(detailSource, /目标与意图/);
  assert.match(detailSource, /今日日程/);
  assert.match(detailSource, /formatZonedRange\(item\.start_at, item\.end_at, scheduleTimezone\)/);
  assert.match(detailSource, /resolveTimezone/);
  assert.match(detailSource, /关系状态/);
  assert.match(detailSource, /可展示记忆/);
  assert.doesNotMatch(governanceSource, /<p class="eyebrow">IDENTITY<\/p>/);
  assert.doesNotMatch(governanceSource, /<p class="eyebrow">LIFE WORLD<\/p>/);
  assert.match(governanceSource, /formatZonedRange\(event\.start_at, event\.end_at/);
});

test("settings accordion remounts when the selected section changes", () => {
  assert.match(settingsSource, /<Accordion :key="currentSection"[^>]*:default-value="currentSection"/);
});
