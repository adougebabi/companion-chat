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
  assert.match(source, /directConversation/);
  assert.match(source, /persistSelection/);
  assert.match(source, /fluctlightId,/);
});

test("control center exposes the required product views", async () => {
  const source = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  assert.match(source, /Diagnostics/);
  assert.match(source, /Settings/);
  assert.match(source, /Fluctlight 实例/);
  assert.match(source, /创建 Fluctlight/);
  assert.match(source, /media\.comfyui/);
  assert.match(source, /MEDIA PROVIDER/);
});

test("owner password change explains its policy without trapping the submit button", async () => {
  const source = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
  assert.match(source, /id="owner-password"/);
  assert.match(source, /新密码至少 12 个字符/);
  assert.match(source, /v-if="store\.authError"/);
  assert.match(source, /:disabled="store\.authLoading \|\| !changedOwnerPassword"/);
  assert.doesNotMatch(source, /changedOwnerPassword\.length < 12/);
});
