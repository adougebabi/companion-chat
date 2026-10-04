import assert from "node:assert/strict";
import test from "node:test";
import { formatInstantInZone } from "../src/lib/instant.ts";
test("diagnostic instant display keeps milliseconds and numeric offsets across DST",()=>{
 assert.equal(formatInstantInZone("2026-10-03T11:26:18.123Z","Asia/Shanghai"),"2026-10-03T19:26:18.123+08:00");
 assert.equal(formatInstantInZone("2026-11-01T05:30:00Z","America/New_York"),"2026-11-01T01:30:00.000-04:00");
 assert.equal(formatInstantInZone("2026-11-01T06:30:00Z","America/New_York"),"2026-11-01T01:30:00.000-05:00");
 assert.equal(formatInstantInZone("2026-01-01T00:00:00Z","UTC"),"2026-01-01T00:00:00.000+00:00");
 assert.equal(formatInstantInZone("","UTC"),"时间未知");
});
