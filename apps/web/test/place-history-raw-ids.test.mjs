import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get, replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const fid = 'fluctlight_history';
const wid = 'world_original';
const pid = 'place_removed';
const createdAt = '2026-11-17T03:36:00Z';
const revision = {
  id: 'place_history_hidden',
  place_id: pid,
  world_id: wid,
  operation: 'edit_fields',
  reason: '为读书会调整 <窗边> 的布置，参考 place_related。',
  created_at: createdAt,
  place_revision: 7893,
  default_revision: 8732,
  source: {source_kind: 'initial_saved', source_resource_id: 'res_hidden', source_version: 'v56'},
  source_ref: 'req_hidden',
  idempotency_key: 'private-retry',
  metrics: {latency: 9281},
  audit_id: 'audit-hidden',
  before_value: {
    name: '雨巷书屋 <北馆>',
    alias: '旧别称',
    description: '窗边摆着空桌',
    atmosphere: '安静',
    scene: '/scenes/private-old.svg',
    furniture: [{name: '旧长椅', source_ref: 'res_hidden'}],
  },
  after_value: {
    name: '雨巷书屋 <北馆>',
    alias: '',
    description: '窗边摆着阅读灯 & 花瓶',
    atmosphere: '安静',
    scene: '/scenes/private-new.svg',
    furniture: ['place_hidden', {name: '木书桌', description: '靠窗', id: 'item-hidden'}],
    room_items: [{label: '纸灯'}],
    future_field: 'unknown-value',
  },
};

const tick = () => new Promise(resolve => setImmediate(resolve));

async function mount(t, readHistory) {
  const env = installDOM();
  const previous = get();
  const host = document.body.appendChild(document.createElement('main'));
  let dispose;
  t.after(() => { dispose?.(); replaceSnapshot(previous); env.restore(); });
  replaceSnapshot({...previous, session: {actorId: 'actor_owner'}});
  configureDomainService('world', {placeDefaultHistory: readHistory});
  const route = approvedRoute(`#/c/${fid}/worlds/${wid}/place/${pid}/history`);
  assert.equal(route.feature.title, '地点修订历史');
  assert.equal(route.feature.render, 'renderPlaceHistory');
  const render = await route.feature.load();
  dispose = render(host, {charId: route.fid, params: route.feature.params});
  await tick();
  return host;
}

function assertDownload(card, expected) {
  assert.equal(card.querySelector('details, pre, code'), null);
  const links = card.querySelectorAll('a[download]');
  assert.equal(links.length, 1);
  assert.equal(links[0].textContent, '原始地点修订记录（诊断）');
  assert.equal(links[0].download, 'fluctlight-record.json');
  const href = links[0].getAttribute('href');
  const prefix = 'data:application/json;charset=utf-8,';
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), expected);
}

const hidden = /place_|world_original|res_hidden|req_hidden|private-retry|audit-hidden|item-hidden|unknown-value|private-(?:old|new)\.svg|\bv\d+\b|7893|8732|9281|版本|尚未记录|未记录|时间未知|完整记录与来源|\b(?:id|revision|source_ref|idempotency_key|metrics|created_at|before_value|after_value|description|room_items|edit_fields|future_field)\b|2026-11-17T03:36/i;

test('地点历史展示中文修订摘要，诊断下载保留完整记录', async t => {
  const calls = [];
  const host = await mount(t, async (...args) => {
    calls.push(args);
    return {items: [revision], next_cursor: ''};
  });
  assert.deepEqual(calls, [[fid, wid, pid, '']]);
  const card = host.querySelector('[data-history] article');
  assert.ok(card, host.textContent);
  assert.equal(card.querySelector('h3').textContent, '雨巷书屋 <北馆>');
  assert.match(card.textContent, /修改地点字段/);
  assert.match(card.textContent, /原因：为读书会调整 <窗边> 的布置，参考 相关记录。/);
  assert.equal(card.querySelector('time').textContent, '2026-11-17 11:36 (北京时间)');
  assert.equal(card.querySelector('time').dateTime, createdAt);
  assert.match(card.textContent, /基准来源：地点首次保存的字段/);
  assert.match(card.textContent, /变化的字段/);
  assert.deepEqual([...card.querySelectorAll('dt')].map(node => node.textContent), [
    '别名', '场景描述', '固定陈设', '房间物品', '场景素材',
  ]);
  assert.deepEqual([...card.querySelectorAll('dd')].map(node => node.textContent), [
    '旧别称 → 已清除', '窗边摆着空桌 → 窗边摆着阅读灯 & 花瓶',
    '旧长椅 → 木书桌（靠窗）', '纸灯', '已更换',
  ]);
  assert.doesNotMatch(host.textContent, hidden);
  assertDownload(card, revision);
});

test('地点历史分页保留稀疏旧记录与基准快照的诊断下载', async t => {
  const initial = {
    id: 'place_history_first', place_id: pid, operation: 'initial_saved',
    before_value: null, after_value: {name: '水边书屋', floor: '一楼'},
  };
  const partial = {
    id: 'place_history_old', place_id: pid, operation: 'unexpected_operation',
    reason: null, created_at: 'bad-time', before_value: {name: pid}, after_value: {alias: ' ', scene: ''},
    source: {source_kind: 'future_source'},
  };
  const baseline = {
    id: 'place_history_baseline', place_id: pid, operation: 'set_baseline',
    before_value: null, after_value: {name: '水边书屋', layout: '当前布局'},
    source: {source_kind: 'published_definition', definition: {layout: '靠墙留出通道', metrics: {latency: 9281}}},
  };
  const restored = {
    id: 'place_history_restore', place_id: pid, operation: 'restore_fields',
    before_value: {name: '水边书屋', layout: '当前布局'}, after_value: {layout: '靠墙留出通道'},
  };
  const cursors = [];
  const host = await mount(t, async (_fid, _wid, _pid, cursor) => {
    cursors.push(cursor);
    return cursor ? {items: [initial, partial], next_cursor: ''} : {items: [restored, baseline], next_cursor: 'earlier'};
  });
  const [restoredCard, baselineCard] = host.querySelectorAll('[data-history] article');
  assert.match(restoredCard.textContent, /水边书屋[\s\S]*恢复默认字段[\s\S]*空间布局当前布局 → 靠墙留出通道/);
  assert.match(baselineCard.textContent, /水边书屋[\s\S]*保存恢复基准[\s\S]*已应用的地点定义[\s\S]*恢复基准字段[\s\S]*空间布局靠墙留出通道/);
  assert.doesNotMatch(baselineCard.textContent, /当前布局|变化的字段/);
  assert.doesNotMatch(host.textContent, hidden);
  assertDownload(restoredCard, restored);
  assertDownload(baselineCard, baseline);

  host.querySelector('[data-page-next]').click();
  await tick();
  assert.deepEqual(cursors, ['', 'earlier']);
  const [initialCard, partialCard] = host.querySelectorAll('[data-history] article');
  assert.match(initialCard.textContent, /水边书屋[\s\S]*首次保存[\s\S]*楼层一楼/);
  assert.match(partialCard.textContent, /地点字段修订/);
  assert.doesNotMatch(partialCard.textContent, /原因|修订时间|基准来源|变化的字段|unexpected_operation|future_source|bad-time/);
  assert.equal(partialCard.querySelector('h3, time, dl'), null);
  assert.doesNotMatch(host.textContent, hidden);
  assertDownload(initialCard, initial);
  assertDownload(partialCard, partial);
});
