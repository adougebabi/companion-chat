import assert from 'node:assert/strict';
import test from 'node:test';
import {zonedInput,zonedInputInstant} from '../src/v3/zoned-input.mjs';

test('schedule editing displays its own zone and preserves an untouched instant including seconds and offset',()=>{
  const original='2026-10-20T16:30:45.123+00:00';
  assert.equal(zonedInput(original,'Asia/Shanghai'),'2026-10-21T00:30');
  assert.equal(zonedInputInstant('2026-10-21T00:30','Asia/Shanghai',original),original);
  assert.equal(zonedInputInstant('2026-10-21T01:30','Asia/Shanghai',original),'2026-10-21T01:30:00+08:00');
  assert.equal(zonedInputInstant('2026-10-21T09:00','Asia/Kathmandu'),'2026-10-21T09:00:00+05:45');
});
test('DST gaps and ambiguous clocks require correction rather than silently changing the requested time',()=>{
  assert.throws(()=>zonedInputInstant('2026-03-08T02:30','America/New_York'),/夏令时跳过/);
  assert.throws(()=>zonedInputInstant('2026-11-01T01:30','America/New_York'),/出现两次/);
  const original='2026-11-01T01:30:00-05:00';
  assert.equal(zonedInputInstant('2026-11-01T01:30','America/New_York',original),original);
  assert.equal(zonedInputInstant('2026-07-01T12:00','America/New_York'),'2026-07-01T12:00:00-04:00');
  assert.equal(zonedInputInstant('2026-01-01T12:00','America/New_York'),'2026-01-01T12:00:00-05:00');
});
test('malformed dates and absent values cannot become a fabricated valid instant',()=>{
  assert.equal(zonedInput(null,'Asia/Shanghai'),'');
  assert.equal(zonedInput('invalid','Asia/Shanghai'),'');
  for(const v of ['','2026-02-31T12:00','2026-10-21T25:00','2026-10-21'])assert.throws(()=>zonedInputInstant(v,'Asia/Shanghai'));
});
