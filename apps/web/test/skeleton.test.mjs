import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("web chat keeps the generated browser client at its boundary", async () => {
  const source = await readFile(new URL("../src/stores/conversations.ts", import.meta.url), "utf8");
  assert.match(source, /@fluctlight\/browser-client/);
  assert.match(source, /assistantDraft/);
  assert.match(source, /expectedSequence/);
  assert.match(source, /markRead/);
  assert.match(source, /optimisticIndex/);
});

test("control center exposes the required product views", async () => {
  const source = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  assert.match(source, /Diagnostics/);
  assert.match(source, /Settings/);
  assert.match(source, /Actors/);
});
