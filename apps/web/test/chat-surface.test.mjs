import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
const server = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  appType: "custom",
  logLevel: "silent",
  server: {
    middlewareMode: true
  }
});
test("stream reveal keeps history immediate, snaps calibration, and refuses a long backlog", async () => {
  const {
    revealStep,
    graphemes
  } = await server.ssrLoadModule("/src/composables/stream-reveal.ts");
  assert.equal(revealStep("历史消息直接出现", "", {
    animate: false,
    reducedMotion: false
  }), "历史消息直接出现");
  assert.equal(revealStep("历史消息直接出现", "", {
    animate: true,
    reducedMotion: true
  }), "历史消息直接出现");
  assert.equal(revealStep("你好", "你", {
    animate: true,
    reducedMotion: false
  }), "你好");
  assert.equal(revealStep("服务端校准后的全文", "旧的半句", {
    animate: true,
    reducedMotion: false
  }), "服务端校准后的全文");
  const long = "这是一段不会逐字积压的回复文本，超过滞后上限后应一次对齐。";
  assert.equal(revealStep(long, "这", {
    animate: true,
    reducedMotion: false,
    maxLag: 8
  }), long);
  assert.deepEqual(graphemes("🌿"), ["🌿"]);
  assert.equal(graphemes("家人👨‍👩‍👧").at(-1), "👨‍👩‍👧");
});
test("older history keeps the reader in place and the bottom follow rule is explicit", async () => {
  const {
    isNearBottom,
    scrollTopAfterPrepend
  } = await server.ssrLoadModule("/src/composables/chat-scroll.ts");
  assert.equal(isNearBottom({
    scrollTop: 900,
    scrollHeight: 1000,
    clientHeight: 80
  }), true);
  assert.equal(isNearBottom({
    scrollTop: 100,
    scrollHeight: 1000,
    clientHeight: 80
  }), false);
  assert.equal(scrollTopAfterPrepend(400, 20, 700), 320);
});
test.after(async () => {
  await server.close();
});
