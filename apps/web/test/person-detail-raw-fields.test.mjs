import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { featureRoute } from "../src/approved-ui/feature-routes.mjs";
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
const { default: PeoplePage } = await vite.ssrLoadModule("/src/react-app/pages/PeoplePage.jsx");

const fid = "reader";
const world = { id: "world_private", fluctlight_id: fid, name: "旧街" };
const places = [
  { id: "place_cafe", world_id: world.id, name: "街角咖啡馆" },
  { id: "place_bookshop", world_id: world.id, name: "迟山书店" },
  { id: "place_park", world_id: world.id, name: "河岸公园" },
];
const occurredAt = "2026-11-23T02:25:00Z";
const contact = {
  id: "encounter_private",
  npc_id: "npc_private",
  world_id: world.id,
  owner_actor_id: "actor_private_owner",
  place_id: places[1].id,
  record_kind: "encounters",
  revision: 937,
  occurred_at: occurredAt,
  summary: "两人谈起了上次借阅的诗集。",
  outcome: "contacted",
  result: {
    summary: "约好下次交换读书笔记。",
    actor_id: "actor_private_result",
    source_ref: "private-result-source",
    metrics: { familiarity: 0.937 },
    unused_internal_key: "private-result-extra",
  },
  source_kind: "owner_confirmed_encounter",
  source_ref: "private-encounter-source",
  evidence_refs: ["private-evidence"],
  participants: ["actor_private", "actor_private_owner"],
  life_event_id: "event_private",
  idempotency_key: "private-encounter-command",
};
const npc = {
  id: contact.npc_id,
  actor_id: contact.participants[0],
  world_id: world.id,
  name: "苏禾",
  role: "书店店主",
  bio: "喜欢旅行和旧书，常在午后整理书架。",
  status: "active",
  enabled: true,
  position_mode: "routine",
  revision: 861,
  home_place_id: places[0].id,
  planned_position: {
    place_id: places[0].id,
    source: "schedule_prediction",
    window_start: "2026-11-23T01:00:00Z",
    window_end: "2026-11-23T04:00:00Z",
    activity: "阅读诗集",
  },
  current_presence: {
    place_id: places[2].id,
    definition_revision: 861,
    status: "active",
    start_at: "2000-01-01T00:00:00Z",
    end_at: "2100-01-01T00:00:00Z",
  },
  confirmed_place_id: places[1].id,
  confirmed_at: occurredAt,
  routine: [
    { day: "weekday", start: "8:30", end: "12:00", place_id: places[1].id, activity: "整理书架" },
    { day: "weekend", start: "13:00", end: "16:00", place_id: places[0].id, activity: "阅读旧书" },
  ],
  source_resource_id: "res_private",
  source_version: "private-version-861",
  local_overrides: { arbitrary_key: "private-npc-extra" },
  relationship_context: {
    relationships: [{
      target_actor_id: "actor_private",
      role: { label: "acquaintance" },
      summary: "已经互通姓名。",
      revision: 781,
      metrics: { familiarity: 0.781 },
    }],
  },
};

async function mountDetail(person, sightings) {
  const character = { id: fid, name: "林青", status: "active" };
  const snapshot = { session: { actorId: contact.owner_actor_id }, selectedId: fid };
  const workspace = {
    current: { value: character },
    state: snapshot,
    subscribe: () => () => {},
    getSnapshot: () => snapshot,
    isCurrent: (owner, id) => owner === snapshot.session.actorId && id === fid,
    client: {
      detail: async () => character,
      world: async () => ({ world, places }),
      listRuntimeNPCs: async () => ({ items: [person], next_cursor: "" }),
      runtimeNPC: async () => person,
      npcRecords: async () => ({ items: sightings, next_cursor: "" }),
    },
  };
  const path = `/c/${fid}/people/npc/${person.id}`;
  const query = new URLSearchParams();
  const feature = featureRoute(path, query);
  assert.equal(feature.native, "people");
  return mountReact(vite, PeoplePage, {
    props: { route: { path, query, feature }, initialNpcId: feature.params.npcId },
    workspace,
  });
}

const dialog = () => [...document.querySelectorAll('[role="dialog"]')].at(-1);
const button = (root, label) => [...root.querySelectorAll("button")]
  .find((node) => node.textContent.trim() === label);
function fact(root, label) {
  return [...root.querySelectorAll(".profile-key-value")]
    .find((node) => node.firstElementChild.textContent === label)?.lastElementChild;
}

function assertSummaryOnly(root) {
  assert.doesNotMatch(root.textContent, /\b(?:id|revision|source(?:_\w+)?|routine|day|start|end|weekday|weekend|activity|participants|outcome|evidence_refs|idempotency_key|metrics|familiarity|unused_internal_key|arbitrary_key)\b|版本|尚未记录|未记录|时间未知/);
  assert.doesNotMatch(root.textContent, /\b(?:npc|actor|world|place|encounter|event|res)_[\w-]+\b|private-|\b(?:861|937|781)\b|78[.,]1%/);
  assert.equal(root.querySelector(".record-facts, pre, code"), null);
}

test("人物与接触详情显示中文业务摘要并下载完整原始记录", async (t) => {
  const view = await mountDetail(npc, [contact]);
  const blobs = [];
  const filenames = [];
  t.mock.method(URL, "createObjectURL", (blob) => {
    blobs.push(blob);
    return "blob:person-detail-fixture";
  });
  t.mock.method(URL, "revokeObjectURL", () => {});
  t.mock.method(window.HTMLAnchorElement.prototype, "click", function () {
    filenames.push(this.download);
  });
  try {
    const person = dialog();
    assert.equal(fact(person, "人物背景").textContent, npc.bio);
    assert.equal(fact(person, "预计地点").textContent, places[0].name);
    assert.equal(fact(person, "预计时段").textContent, "2026-11-23 09:00 (北京时间) 至 2026-11-23 12:00 (北京时间)");
    assert.equal(fact(person, "预计活动").textContent, "阅读诗集");
    assert.equal(fact(person, "已读在场地点").textContent, places[2].name);
    assert.equal(fact(person, "上次确认地点").textContent, places[1].name);
    assert.equal(fact(person, "上次实际接触").textContent, "2026-11-23 10:25 (北京时间)");
    assert.deepEqual([...fact(person, "生活规律").querySelectorAll("li")].map((slot) => slot.textContent), [
      "工作日 · 08:30–12:00 · 迟山书店 · 整理书架",
      "周末 · 13:00–16:00 · 街角咖啡馆 · 阅读旧书",
    ]);
    assert.equal(fact(person, "关系称谓").textContent, "相识的人");
    assert.equal(fact(person, "关系摘要").textContent, "已经互通姓名。");
    assert.equal(person.querySelector('a[href="#/studio/edit/res_private"]').textContent, "查看人物定义");

    await view.click(button(person, "原始人物记录（诊断）"));
    assertSummaryOnly(person);
    await view.click(button(person, "下载原始记录"));
    assert.deepEqual(JSON.parse(await blobs[0].text()), npc);
    await view.click(button(person, "接触、在场与相遇记录"));
    assert.match(person.querySelector(".companion-timeline").textContent, /10:25 \(北京时间\).*两人谈起了上次借阅的诗集/);
    assertSummaryOnly(person);

    await view.click(button(person, "详情"));
    const encounter = dialog();
    assert.equal(fact(encounter, "接触时间").querySelector("time").dateTime, occurredAt);
    assert.equal(fact(encounter, "接触时间").textContent, "2026-11-23 10:25 (北京时间)");
    assert.equal(fact(encounter, "地点").textContent, places[1].name);
    assert.equal(fact(encounter, "发生事项").textContent, contact.summary);
    assert.deepEqual([...fact(encounter, "结果").querySelectorAll("p")].map((p) => p.textContent), [
      "接触已记录", contact.result.summary,
    ]);
    assert.equal(encounter.querySelector('a[href="#/c/reader/world/event/event_private"]').textContent, "查看实际生活事件");
    await view.click(button(encounter, "原始接触记录（诊断）"));
    assertSummaryOnly(encounter);
    await view.click(button(encounter, "下载原始记录"));
    assert.deepEqual(JSON.parse(await blobs[1].text()), contact);
    assert.deepEqual(filenames, ["fluctlight-record.json", "fluctlight-record.json"]);
  } finally {
    await view.dispose();
  }
});

test("人物与接触记录省略空字段、无效时间与无法解析的地点", async () => {
  const person = {
    ...npc,
    bio: " ",
    routine: [{ day: "", start: "", end: "", place_id: "place_unknown", activity: "" }],
    home_place_id: null,
    current_presence: null,
    planned_position: { place_id: "place_unknown" },
    confirmed_place_id: "place_unknown",
    confirmed_at: "",
    relationship_context: { relationships: [] },
  };
  const encounter = {
    id: contact.id, npc_id: npc.id, world_id: world.id, place_id: "place_unknown",
    occurred_at: "invalid-instant", summary: "", status: "unknown_internal_status", result: {},
  };
  const view = await mountDetail(person, [encounter]);
  try {
    const detail = dialog();
    assert.ok(button(detail, "原始人物记录（诊断）"));
    assert.doesNotMatch(detail.textContent, /人物背景|生活规律|预计地点|上次确认地点|上次实际接触|尚未设定|地点名称待读取/);
    await view.click(button(detail, "接触、在场与相遇记录"));
    assert.equal(detail.querySelector(".companion-timeline time"), null);
    await view.click(button(detail, "详情"));
    const record = dialog();
    assert.ok(button(record, "原始接触记录（诊断）"));
    assert.equal(record.querySelectorAll(".profile-key-value").length, 0);
    assert.doesNotMatch(record.textContent, /unknown_internal_status|invalid-instant/);
    assertSummaryOnly(detail);
    assertSummaryOnly(record);
  } finally {
    await view.dispose();
  }
});

test("旧接触记录保留发生事项并按已有引用描述结果", async () => {
  const legacy = {
    id: contact.id, npc_id: npc.id, world_id: world.id, occurred_at: occurredAt,
    summary: "在书店交谈。", life_event_id: contact.life_event_id,
    relationship_id: "relationship_private", memory_id: "memory_private",
  };
  const view = await mountDetail(npc, [legacy]);
  try {
    await view.click(button(dialog(), "接触、在场与相遇记录"));
    await view.click(button(dialog(), "详情"));
    const record = dialog();
    assert.equal(fact(record, "发生事项").textContent, legacy.summary);
    assert.deepEqual([...fact(record, "结果").querySelectorAll("p")].map((p) => p.textContent), [
      "接触已记录", "已关联关系记录", "已关联记忆记录",
    ]);
    assert.doesNotMatch(record.textContent, /relationship_private|memory_private|建立结识|新建立/);
  } finally {
    await view.dispose();
  }
});
