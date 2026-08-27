import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const navigationSource = await readFile(new URL("../src/app/navigation.ts", import.meta.url), "utf8");
const stylesSource = await readFile(new URL("../src/styles/app.css", import.meta.url), "utf8");
const shellSource = await readFile(new URL("../src/components/layout/AppShell.vue", import.meta.url), "utf8");

test("Telegram-style workspace keeps exactly three primary tabs", () => {
  assert.match(navigationSource, /id: "instances"/);
  assert.match(navigationSource, /id: "moments"/);
  assert.match(navigationSource, /id: "settings"/);
  assert.doesNotMatch(navigationSource, /id: "chat"/);
  assert.match(navigationSource, /primaryNavigation[\s\S]*id: "instances"[\s\S]*id: "moments"[\s\S]*id: "settings"/);
  assert.match(shellSource, /activeView !== 'chat'/);
});

test("chat owns a fixed-height work surface with an internal message scroller", () => {
  assert.match(stylesSource, /\.app-shell\.chat-shell[^}]*height: calc\(100dvh - 32px\)/);
  assert.match(stylesSource, /\.app-shell\.chat-shell[^}]*padding-bottom: 0/);
  assert.match(stylesSource, /\.chat-page[^}]*height: 100%[^}]*min-height: 0/);
  assert.match(stylesSource, /\.message-timeline[^}]*min-height: 0[^}]*overflow-y: auto/);
});
