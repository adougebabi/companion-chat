import assert from "node:assert/strict";
import test from "node:test";

import { translateCoreNdjson } from "../src/ndjson.js";

async function readStream(stream: ReadableStream<Uint8Array>): Promise<string> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let text = "";
  while (true) {
    const next = await reader.read();
    if (next.done) return text;
    text += decoder.decode(next.value, { stream: true });
  }
}

function response(chunks: string[]): Response {
  return new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        for (const chunk of chunks) controller.enqueue(new TextEncoder().encode(chunk));
        controller.close();
      },
    }),
    { headers: { "content-type": "application/x-ndjson" } },
  );
}

test("translator accepts split UTF-8 frames and emits one browser terminal", async () => {
  const core = [
    JSON.stringify({ type: "token", turn_id: "turn-1", sequence: 0, payload: { text: "你" } }),
    JSON.stringify({ type: "completed", turn_id: "turn-1", sequence: 1, payload: {} }),
  ].join("\n") + "\n";
  const bytes = new TextEncoder().encode(core);
  const output = await readStream(translateCoreNdjson(response([new TextDecoder().decode(bytes.slice(0, 8)), new TextDecoder().decode(bytes.slice(8))])));
  const events = output.trim().split("\n").map((line) => JSON.parse(line) as { type: string; sequence: number });
  assert.deepEqual(events.map((event) => event.type), ["token", "completed"]);
  assert.deepEqual(events.map((event) => event.sequence), [0, 1]);
});

test("translator closes with a bounded error on a sequence violation", async () => {
  const core = `${JSON.stringify({ type: "token", turn_id: "turn-1", sequence: 1, payload: {} })}\n`;
  const output = await readStream(translateCoreNdjson(response([core])));
  const events = output.trim().split("\n").map((line) => JSON.parse(line) as { type: string; payload: { code: string } });
  assert.equal(events.at(-1)?.type, "error");
  assert.equal(events.at(-1)?.payload.code, "core_sequence_invalid");
});

test("translator rejects unknown event types and invalid UTF-8", async () => {
  const unknown = `${JSON.stringify({ type: "unknown", turn_id: "turn-1", sequence: 0, payload: {} })}\n`;
  const unknownOutput = await readStream(translateCoreNdjson(response([unknown])));
  const unknownEvents = unknownOutput.trim().split("\n").map((line) => JSON.parse(line) as { type: string; payload: { code: string } });
  assert.equal(unknownEvents.at(-1)?.payload.code, "invalid_core_event");

  const invalidUtf8 = new Response(new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new Uint8Array([0xff, 0xfe, 0x0a]));
      controller.close();
    },
  }), { headers: { "content-type": "application/x-ndjson" } });
  const invalidOutput = await readStream(translateCoreNdjson(invalidUtf8));
  const invalidEvents = invalidOutput.trim().split("\n").map((line) => JSON.parse(line) as { type: string; payload: { code: string } });
  assert.equal(invalidEvents.at(-1)?.payload.code, "core_stream_invalid");
});

test("translator cancels the Core reader and emits no later frame after browser abort", async () => {
  let upstreamCancelled = false;
  let release: (() => void) | undefined;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  const core = new Response(
    new ReadableStream<Uint8Array>({
      async start(controller) {
        controller.enqueue(new TextEncoder().encode(`${JSON.stringify({ type: "token", turn_id: "turn-1", sequence: 0, payload: { text: "first" } })}\n`));
        await pending;
        controller.enqueue(new TextEncoder().encode(`${JSON.stringify({ type: "completed", turn_id: "turn-1", sequence: 1, payload: {} })}\n`));
        controller.close();
      },
      cancel() {
        upstreamCancelled = true;
      },
    }),
  );
  const controller = new AbortController();
  const reader = translateCoreNdjson(core, controller.signal).getReader();
  const first = await reader.read();
  assert.equal(new TextDecoder().decode(first.value).includes("first"), true);

  controller.abort();
  release?.();
  const afterAbort = await reader.read();

  assert.equal(afterAbort.done, true);
  assert.equal(upstreamCancelled, true);
});
