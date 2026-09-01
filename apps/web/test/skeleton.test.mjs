import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

async function readVueSources(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const sources = [];
  for (const entry of entries) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) sources.push(...await readVueSources(path));
    else if (entry.name.endsWith(".vue")) sources.push(await readFile(path, "utf8"));
  }
  return sources;
}

const vueSource = (await readVueSources(fileURLToPath(new URL("../src", import.meta.url)))).join("\n");

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
  assert.match(source, /retryTurn/);
  assert.match(source, /retrying/);
  assert.match(source, /queuedText/);
  assert.match(source, /persistRetry/);
  assert.match(source, /event\.turnId/);
});

test("web keeps a failed turn retryable instead of allowing later messages to overtake it", async () => {
  const source = await readFile(new URL("../src/stores/conversations.ts", import.meta.url), "utf8");
  const viewSource = await readFile(new URL("../src/views/ChatView.vue", import.meta.url), "utf8");
  assert.match(source, /if \(pendingRetry && !retry\)/);
  assert.match(source, /await this\.retry\(\)/);
  assert.match(source, /idempotencyKey: request\.idempotencyKey/);
  assert.match(source, /turnId: request\.turnId/);
  assert.match(viewSource, /store\.canRetry/);
  assert.match(viewSource, /store\.retry/);
});

test("control center exposes the required product views", async () => {
  const source = vueSource;
  assert.match(source, /Diagnostics/);
  assert.match(source, /Settings/);
  assert.match(source, /Fluctlight 实例/);
  assert.match(source, /创建 Fluctlight/);
  assert.match(source, /media\.comfyui/);
  assert.match(source, /MEDIA PROVIDER/);
});

test("owner password change explains its policy without trapping the submit button", async () => {
  const source = vueSource;
  assert.match(source, /id="owner-password"/);
  assert.match(source, /新密码至少 6 个字符/);
  assert.match(source, /id="setup-password"[^>]*minlength="6"/);
  assert.match(source, /id="auth-password"[^>]*minlength="6"/);
  assert.match(source, /v-if="store\.authError"/);
  assert.match(source, /:disabled="store\.authLoading \|\| !changedOwnerPassword"/);
  assert.doesNotMatch(source, /changedOwnerPassword\.length < 6/);
});

test("web uses the secure random-ID compatibility helper instead of randomUUID directly", async () => {
  const appSource = vueSource;
  const storeSource = await readFile(new URL("../src/stores/conversations.ts", import.meta.url), "utf8");
  const helperSource = await readFile(new URL("../src/random-id.ts", import.meta.url), "utf8");
  assert.match(appSource, /randomId\(\)/);
  assert.match(storeSource, /randomId\(\)/);
  assert.doesNotMatch(appSource, /crypto\.randomUUID/);
  assert.doesNotMatch(storeSource, /crypto\.randomUUID/);
  assert.match(helperSource, /crypto\?\.randomUUID/);
  assert.match(helperSource, /crypto\?\.getRandomValues/);
});

test("creation dialog presents activation failures where the user can act on them", async () => {
  const source = vueSource;
  assert.match(source, /<Dialog[\s\S]*<DialogContent[^>]*class="create-surface"/);
  assert.match(source, /class="create-dialog-body"[\s\S]*controlCenter\.error/);
  assert.match(source, /class="create-dialog-footer"/);
  assert.match(source, /form="activate-preview-form"/);
  assert.match(source, /预览必须包含 identity/);
});

test("diagnostics entry loads records and creation keeps a direct correlation link", async () => {
  const source = vueSource;
  assert.match(source, /诊断中心/);
  assert.match(source, /creationDiagnosticsCorrelationId/);
  assert.match(source, /查看本次分析诊断/);
  assert.match(source, /behavioral_policy/);
});
