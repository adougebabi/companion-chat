import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const appSource = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");

test("control center keeps keyboard, live-region, and form boundaries explicit", () => {
  assert.match(appSource, /aria-label="Fluctlight 控制中心"/);
  assert.match(appSource, /aria-live="polite"/);
  assert.match(appSource, /role="alert"/);
  assert.match(appSource, /for="message-composer"/);
  assert.match(appSource, /for="attachment-reference"/);
  assert.match(appSource, /id="attachment-reference"/);
  assert.match(appSource, /aria-label="附件引用"/);
  assert.match(appSource, /id="auth-password"/);
  assert.match(appSource, /autocomplete="current-password"/);
  assert.match(appSource, /aria-labelledby="auth-title"/);
  assert.match(appSource, /@click="store.logout"/);
  assert.match(appSource, /@keydown="onKeydown"/);
  assert.doesNotMatch(appSource, /v-html/);
});
