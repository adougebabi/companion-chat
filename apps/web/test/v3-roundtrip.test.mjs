import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { bundleFromSeed, rewriteSeedAssets, viewFromBundle } from "../src/v3/contract-map.mjs";
import { seed } from "./fixtures/proto-seed.mjs";

const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");

const KEYS = ["groups", "characters", "worlds", "user", "places", "npcs", "relations", "messages", "memories", "goals", "schedule", "actions", "items", "outfit", "media", "events", "diary", "tasks", "proposals", "resources", "instances", "studioDrafts", "posts", "postSuggestions", "reflections", "worldSettings", "prefs", "notices"];

test("contract projection round-trips the prototype view model", () => {
  const expected = rewriteSeedAssets(seed());
  expected.user.background = {}; // Actor background is scoped; it is never a global user snapshot.
  for (const id of Object.keys(expected.characters)) if (!expected.outfit[id]) expected.outfit[id] = { current: [], history: [], since: undefined };
  const view = viewFromBundle(bundleFromSeed(seed()));
  for (const key of KEYS) assert.deepEqual(view[key], expected[key], key);
});

test("an unfinished studio reference does not apply a version or resolve conflicts", () => {
  const bundle = bundleFromSeed(seed());
  const resource = bundle.resources.items.find((item) => item.id === "r_book");
  const real = resource.referenced_by.find((item) => item.fluctlight_id === "c1");
  resource.referenced_by.push({ fluctlight_id: "c1", selected_version: "v9", status: "recorded_not_applied", applied: false, instance_revision: null, conflicts: null });
  const view = viewFromBundle(bundle);
  assert.equal(view.instances.c1.r_book.applied, real.running_version);
  assert.deepEqual(view.instances.c1.r_book.overrides, real.overrides);
  assert.equal(view.meta.studioApplyNotes.r_book, "未完成·已记录选择，尚未写入实例");
});

test("Mock acceptance entry keeps wardrobe flow and persistence separate from the real transport", () => {
  const store = read('../src/v3/proto/mock/store.js');
  const wardrobe = read('../src/v3/proto/pages/wardrobe.js');
  const flows = read('../src/v3/proto/pages/flows.js');
  assert.match(store, /fluctlight.v3.acceptance.mock.v1/);
  assert.doesNotMatch(store, /BrowserClient|fetch\(/);
  assert.match(wardrobe, /requestOutfit/);
  assert.match(flows, /pending|current/);
  assert.match(wardrobe, /不会改变穿着、位置或任何事实/);
});

test("JSON wire shape still round-trips page collections", () => {
  const expected = rewriteSeedAssets(seed());
  expected.user.background = {}; // Actor background is scoped; it is never a global user snapshot.
  for (const id of Object.keys(expected.characters)) if (!expected.outfit[id]) expected.outfit[id] = { current: [], history: [], since: undefined };
  const wire = JSON.parse(JSON.stringify(bundleFromSeed(seed())));
  const view = viewFromBundle(wire);
  for (const key of KEYS) assert.deepEqual(view[key], expected[key], key);
});
