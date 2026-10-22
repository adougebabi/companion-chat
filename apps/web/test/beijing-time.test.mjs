import test from 'node:test';
import assert from 'node:assert/strict';
import {formatBeijingInstant,formatInstantInZone} from '../src/lib/instant.ts';
import {userInstant,readingInstant} from '../src/v3/presentation-time.mjs';
import {formatZonedRange} from '../src/lib/fluctlight-display.ts';
import {zonedInput,zonedInputInstant} from '../src/v3/zoned-input.mjs';

test('ordinary time display converts the instant rather than relabelling the source clock',()=>{
 for(const value of ['2026-10-20T07:00:45.123Z','2026-10-20T15:00:45.123+08:00','2026-10-20T03:00:45.123-04:00']){
  assert.equal(formatBeijingInstant(value),'2026-10-20 15:00 (北京时间)');
  assert.equal(userInstant(value,'America/New_York'),readingInstant(value,'invalid-zone'));
 }
 assert.equal(formatBeijingInstant('2026-12-31T16:00:00Z'),'2027-01-01 00:00 (北京时间)');
 assert.equal(formatBeijingInstant('2026-10-20'),'2026-10-20');
 assert.equal(formatBeijingInstant('invalid'),'时间未知');
 assert.equal(formatBeijingInstant(null),'未记录');
 assert.equal(formatZonedRange('2026-12-31T15:59:00Z','2026-12-31T16:00:00Z','America/New_York'),'2026-12-31 23:59 (北京时间) – 2027-01-01 00:00 (北京时间)');
});
test('Beijing editing preserves original precision and offset until the minute changes',()=>{
 const original='2026-10-20T03:00:45.123-04:00';
 assert.equal(zonedInput(original,'Asia/Shanghai'),'2026-10-20T15:00');
 assert.equal(zonedInputInstant('2026-10-20T15:00','Asia/Shanghai',original),original);
 assert.equal(zonedInputInstant('2026-10-20T15:01','Asia/Shanghai',original),'2026-10-20T15:01:00+08:00');
 assert.equal(formatInstantInZone(original,'Asia/Shanghai'),'2026-10-20T15:00:45.123+08:00');
});
