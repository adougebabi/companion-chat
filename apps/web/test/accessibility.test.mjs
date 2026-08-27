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

const appSource = (await readVueSources(fileURLToPath(new URL("../src", import.meta.url)))).join("\n");

test("control center keeps keyboard, live-region, and form boundaries explicit", () => {
  assert.match(appSource, /aria-label="Fluctlight 主导航"/);
  assert.match(appSource, /aria-live="polite"/);
  assert.match(appSource, /role="alert"/);
  assert.match(appSource, /for="message-composer"/);
  assert.doesNotMatch(appSource, /attachment-reference/);
  assert.match(appSource, /id="auth-password"/);
  assert.match(appSource, /autocomplete="current-password"/);
  assert.match(appSource, /aria-labelledby="auth-title"/);
  assert.match(appSource, /@logout="store.logout"/);
  assert.match(appSource, /@keydown="onKeydown"/);
  assert.doesNotMatch(appSource, /v-html/);
});
