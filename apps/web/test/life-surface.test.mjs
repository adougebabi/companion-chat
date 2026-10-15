import assert from "node:assert/strict";
import test from "node:test";
import { moodRead, recordedCommitments, scheduleRead, sourceLabel, usedItems, wardrobeFilterAvailability, wardrobeItemVisible, wornItems } from "../src/lib/life-surface.ts";

const detail = {
  schedule: { id: "sch", revision: 2, items: [{ id: "a", activity: "整理" }] },
  goals: [{ id: "g", desired_outcome: "看书", commitments: [{ id: "c", title: "去花店" }] }],
  current_state: { appearance: { worn_items: [{ id: "coat" }], used_items: [{ id: "brush" }] } },
  inner_state: { mood: { label: "平静" } },
};

test("wearing, using, ownership and source stay separate", () => {
  const coat = { id: "coat", ownership: "borrowed", source_kind: "accepted_event" };
  const brush = { id: "brush", ownership: "owned", source_kind: "purchase_result" };
  assert.equal(wardrobeItemVisible("wearing", coat, detail), true);
  assert.equal(wardrobeItemVisible("borrowed", coat, detail), true);
  assert.equal(wardrobeItemVisible("owned", coat, detail), false);
  assert.equal(wardrobeItemVisible("using", brush, detail), true);
  assert.equal(wardrobeItemVisible("wearing", brush, detail), false);
  assert.equal(sourceLabel(coat.source_kind), "已接受事件");
  assert.notEqual(sourceLabel("initialization"), "已拥有");
  assert.equal(wardrobeFilterAvailability("inbound", detail), "unwired");
});

test("missing wearing list is unwired instead of treating a slot as worn", () => {
  const bare = { current_state: { appearance: {} } };
  assert.equal(wornItems(bare), null);
  assert.equal(usedItems(bare), null);
  assert.equal(wardrobeFilterAvailability("wearing", bare), "unwired");
  assert.equal(wardrobeItemVisible("wearing", { id: "coat", slot: "outer" }, bare), false);
});

test("mood without evidence is recorded but not inferred, and absent schedule stays missing", () => {
  assert.deepEqual(moodRead(detail), { state: "recorded", label: "平静", evidence: null });
  assert.equal(moodRead({ inner_state: {} }).state, "missing");
  assert.equal(scheduleRead({}).state, "missing");
  assert.equal(scheduleRead({ schedule: null }).state, "empty");
  assert.equal(recordedCommitments({}).state, "missing");
  assert.equal(recordedCommitments(detail).items[0].title, "去花店");
  assert.equal(recordedCommitments({ goals: [{ desired_outcome: "看起来像约定" }] }).items.length, 0);
});
