import assert from "node:assert/strict";
import test from "node:test";

import { messageTimeLabel } from "../src/lib/message-time.ts";
import { zonedLocalInputToISO } from "../src/lib/zoned-local-input.ts";

test("message wall clock remains at the original sender offset", () => {
  const message = { createdAt: "2026-07-01T12:00:05Z", senderTimezone: "America/New_York", senderUtcOffsetMinutes: -240, senderSentAt: "2026-07-01T12:00:00Z" };
  assert.equal(messageTimeLabel(message), "08:00 · America/New_York (UTC−04:00)");
  assert.match(messageTimeLabel({ createdAt: "2026-07-01T12:00:00Z" }), /旧消息，发送时区未知/);
});

test("datetime-local is interpreted in Fluctlight timezone with explicit DST policy", () => {
  assert.equal(zonedLocalInputToISO("2026-07-01T08:00", "America/New_York"), "2026-07-01T12:00:00.000Z");
  assert.equal(zonedLocalInputToISO("2026-11-01T01:30", "America/New_York"), "2026-11-01T05:30:00.000Z");
  assert.throws(() => zonedLocalInputToISO("2026-03-08T02:30", "America/New_York"), /local_time_nonexistent/);
});
