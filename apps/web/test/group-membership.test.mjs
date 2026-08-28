import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = await readFile(new URL("../src/lib/group-membership.ts", import.meta.url), "utf8");
const storeSource = await readFile(new URL("../src/stores/control-center.ts", import.meta.url), "utf8");

test("default group reconciliation keeps only ungrouped actor IDs for assignment", () => {
  assert.match(source, /group\.name === "默认"/);
  assert.match(source, /actorIds\.filter\(\(actorId\) => !assignedActorIds\.has\(actorId\)\)/);
  assert.match(source, /defaultGroup/);
  assert.match(storeSource, /ensureDefaultGroup/);
  assert.match(storeSource, /createActorGroup\("默认"\)/);
  assert.match(storeSource, /assignActorGroupMember\(defaultGroup\.id, actorId\)/);
});
