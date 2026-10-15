import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");
const app = read("../src/App.vue");
const now = read("../src/views/life/LifeNowView.vue");
const schedule = read("../src/views/life/LifeScheduleView.vue");
const goals = read("../src/views/life/LifeGoalsView.vue");
const wardrobe = read("../src/views/life/LifeWardrobeView.vue");
const people = read("../src/views/life/PeopleView.vue");
const growth = read("../src/views/life/GrowthView.vue");
const profile = read("../src/views/life/ProfileView.vue");
const media = read("../src/views/life/MediaView.vue");
const pages = [now, schedule, goals, wardrobe, people, growth, profile, media];

test("phase 3 routes mount the sage pages and keep diary unwired", () => {
  assert.match(app, /id="v3-root"/);
  assert.match(app, /mountProduct/);
  assert.match(now, /LifeNow|此刻|心情尚未接入/);
  assert.doesNotMatch(app, /iframe/);
  assert.doesNotMatch(app, /<LifeNowView|<UnwiredView|<GovernanceView/);
});

test("writes stay on the existing revision, reason, evidence and idempotency commands", () => {
  assert.match(schedule, /acceptSchedule/);
  assert.match(schedule, /cancelSchedule/);
  assert.match(schedule, /createLifeEvent/);
  assert.match(schedule, /governanceEvidence/);
  assert.match(goals, /GoalPlanningPanel/);
  assert.match(goals, /GoalPanel/);
  assert.match(goals, /ActorContextEditor/);
  assert.match(wardrobe, /wardrobeItemPayload/);
  assert.match(wardrobe, /addWardrobeItems/);
  assert.match(people, /editRelationship/);
  assert.match(people, /rollbackRelationship/);
  assert.match(people, /governanceEvidence/);
  assert.match(people, /governanceReason/);
  assert.match(growth, /reviseMemory/);
  assert.match(growth, /forgetMemory/);
  assert.match(growth, /rollbackDevelopingSelf/);
  assert.match(growth, /forgetDevelopingSelf/);
  assert.match(profile, /acceptFoundationRevision/);
  assert.match(profile, /submitFoundationRevision/);
  assert.match(profile, /rollbackFoundationRevision/);
  assert.match(profile, /revisionReason/);
});

test("missing life data is labeled unwired and generated images stay non-factual", () => {
  for (const source of pages) assert.doesNotMatch(source, /v-html/);
  assert.match(now, /心情尚未接入/);
  assert.match(now, /此地附近尚未接入/);
  assert.match(now, /场景图尚未接入/);
  assert.match(wardrobe, /入库中尚未接入/);
  assert.match(wardrobe, /不会把部位当成穿着/);
  assert.match(wardrobe, /换装请求和拍照尚未接入/);
  assert.match(people, /所有者背景不是 NPC/);
  assert.match(media, /生成图·非事实/);
  assert.match(media, /角色媒体库列表尚未接入/);
  assert.match(growth, /反思详情尚未接入/);
  assert.doesNotMatch(pages.join("\n"), /prototype-v3|store\.js|演示值/);
});
