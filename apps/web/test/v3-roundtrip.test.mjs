import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { bundleFromSeed, rewriteSeedAssets, viewFromBundle } from "../src/v3/contract-map.mjs";
import { seed } from "./fixtures/proto-seed.mjs";

const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");

const KEYS = ["groups", "characters", "worlds", "user", "places", "npcs", "relations", "messages", "memories", "goals", "schedule", "actions", "items", "outfit", "media", "events", "diary", "tasks", "proposals", "resources", "instances", "studioDrafts", "posts", "postSuggestions", "reflections", "worldSettings", "prefs", "notices"];

test("contract projection round-trips the prototype view model", () => {
  const expected = rewriteSeedAssets(seed());
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

test("management wardrobe writes use the wardrobe revision and keep the draft on conflict", () => {
  const store = read("../src/v3/proto/store.js");
  const studio = read("../src/v3/proto/pages/studio-edit.js");
  assert.match(store, /confirm: true/);
  assert.match(store, /revision_conflict/);
  assert.match(store, /resource === "wardrobe"/);
  assert.match(store, /owner_edit/);
  assert.match(store, /她刚更新过。草稿已保留/);
  assert.match(store, /未完成·已记录选择，尚未写入实例/);
  assert.match(store, /recorded_by === "owner"/);
  assert.doesNotMatch(store, /applied_version|applied_at/);
  assert.doesNotMatch(store, /expectedRevision:\s*data\.meta\.ownerRevisions/);
  assert.match(studio, /recordStudioApply/);
  assert.doesNotMatch(studio, /\.applied\s*=[^=]/);
});

test("JSON wire shape still round-trips page collections", () => {
  const expected = rewriteSeedAssets(seed());
  const wire = JSON.parse(JSON.stringify(bundleFromSeed(seed())));
  const view = viewFromBundle(wire);
  for (const key of KEYS) assert.deepEqual(view[key], expected[key], key);
});
