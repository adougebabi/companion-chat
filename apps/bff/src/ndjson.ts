export type CoreStreamEvent = {
  type: "token" | "action_result" | "completed" | "error" | "heartbeat";
  turn_id: string;
  sequence: number;
  payload: Record<string, unknown>;
};

export type BrowserStreamEvent = {
  type: "token" | "message" | "media" | "completed" | "error" | "heartbeat";
  turnId: string;
  sequence: number;
  payload: Record<string, unknown>;
};

export class NdjsonTranslationError extends Error {}

const hiddenKeys = new Set([
  "perception",
  "appraisal",
  "reasoning",
  "hiddenreasoning",
  "credentials",
  "authorization",
  "apikey",
  "rawprompt",
  "rawresponse",
]);
const coreTypes = new Set(["token", "action_result", "completed", "error", "heartbeat"]);

function hasHiddenPayload(value: unknown): boolean {
  if (Array.isArray(value)) return value.some(hasHiddenPayload);
  if (!value || typeof value !== "object") return false;
  return Object.entries(value).some(([key, child]) => {
    const normalizedKey = key.replace(/[-_]/g, "").toLowerCase();
    return hiddenKeys.has(normalizedKey) || hasHiddenPayload(child);
  });
}

function encode(event: BrowserStreamEvent): Uint8Array {
  return new TextEncoder().encode(`${JSON.stringify(event)}\n`);
}

function browserEvent(core: CoreStreamEvent): BrowserStreamEvent {
  if (
    !coreTypes.has(core.type) ||
    !core.turn_id ||
    !Number.isInteger(core.sequence) ||
    core.sequence < 0 ||
    !core.payload ||
    typeof core.payload !== "object" ||
    Array.isArray(core.payload)
  ) {
    throw new NdjsonTranslationError("invalid_core_event");
  }
  if (hasHiddenPayload(core.payload)) throw new NdjsonTranslationError("hidden_core_payload");
  let type: BrowserStreamEvent["type"] = core.type === "action_result" ? "message" : core.type;
  if (core.type === "action_result") {
    const message = core.payload.message;
    type = message && typeof message === "object" && (message as { kind?: string }).kind === "media_reference" ? "media" : "message";
  }
  return { type, turnId: core.turn_id, sequence: core.sequence, payload: core.payload };
}

function errorEvent(turnId: string, sequence: number, code: string): Uint8Array {
  return encode({ type: "error", turnId: turnId || "turn-unknown", sequence, payload: { code, message: "The conversation stream is unavailable" } });
}

export function translateCoreNdjson(response: Response, signal?: AbortSignal): ReadableStream<Uint8Array> {
  const body = response.body;
  if (!body) throw new NdjsonTranslationError("core_stream_body_missing");
  const reader = body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let buffer = "";
  let turnId = "";
  let expectedSequence = 0;
  let terminal = false;
  let ended = false;
  let aborted = false;
  const abort = () => {
    aborted = true;
    void reader.cancel();
  };
  const cleanup = () => signal?.removeEventListener("abort", abort);
  signal?.addEventListener("abort", abort, { once: true });

  if (signal?.aborted) {
    void reader.cancel();
    return new ReadableStream<Uint8Array>({ start(controller) { controller.close(); } });
  }

  return new ReadableStream<Uint8Array>({
    async pull(controller) {
      if (aborted || ended) {
        controller.close();
        return;
      }
      try {
        while (true) {
          const newline = buffer.indexOf("\n");
          if (newline >= 0) {
            const line = buffer.slice(0, newline).trim();
            buffer = buffer.slice(newline + 1);
            if (!line) continue;
            const parsed = JSON.parse(line) as CoreStreamEvent;
            if (!turnId) turnId = parsed.turn_id;
            if (terminal || parsed.turn_id !== turnId || parsed.sequence !== expectedSequence) {
              throw new NdjsonTranslationError("core_sequence_invalid");
            }
            const mapped = browserEvent(parsed);
            expectedSequence += 1;
            terminal = mapped.type === "completed" || mapped.type === "error";
            controller.enqueue(encode(mapped));
            if (terminal) {
              ended = true;
              controller.close();
              void reader.cancel();
              cleanup();
            }
            return;
          }
          const next = await reader.read();
          if (next.done) {
            if (aborted) {
              ended = true;
              cleanup();
              controller.close();
              return;
            }
            buffer += decoder.decode();
            if (buffer.trim() || !terminal) {
              controller.enqueue(errorEvent(turnId, expectedSequence, "core_stream_incomplete"));
            }
            ended = true;
            cleanup();
            controller.close();
            return;
          }
          buffer += decoder.decode(next.value, { stream: true });
        }
      } catch (error) {
        const code = error instanceof NdjsonTranslationError ? error.message : "core_stream_invalid";
        await reader.cancel();
        cleanup();
        if (aborted) {
          controller.close();
          return;
        }
        controller.enqueue(errorEvent(turnId, expectedSequence, code));
        ended = true;
        controller.close();
      }
    },
    async cancel() {
      aborted = true;
      cleanup();
      await reader.cancel();
    },
  });
}
