import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("web skeleton keeps the generated browser client at its boundary", async () => {
  const source = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  assert.match(source, /@fluctlight\/browser-client/);
});
