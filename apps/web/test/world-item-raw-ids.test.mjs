import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {closeAllOverlays,overlaySnapshot,subscribeOverlays} from '../src/state/overlays.ts';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get,replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const fid = 'fluctlight_xiaoman';
const world = {id: 'world_xiaoman', name: '小满的世界'};
const place = {id: 'place_market', name: '旧街市集', world_id: world.id};
const item = {
  id: 'world_item_coat',
  world_id: world.id,
  fluctlight_id: fid,
  name: '雨衣 <薄款> & 便携',
  definition: {description: '雨天出门时穿。', item_kind: 'wearable', slot: 'upper_body', schema: 'item-definition.v1'},
  available: true,
  place_id: place.id,
  source_resource_id: 'res_coat',
  source_version: '1',
  revision: 1,
  updated_at: '2026-11-22T19:25:00Z',
  local_overrides: {name: '雨衣 <薄款> & 便携', definition: {description: '雨天出门时穿。'}},
  source_ref: 'req_source',
  idempotency_key: 'req_retry',
  metrics: {tokens: 945},
  correlation_id: 'actor_trace',
};
const directory = `#/c/${fid}/world/items`;
const tick = () => new Promise(resolve => setImmediate(resolve));

async function mount(t, {detail = false, record = item, readPlaces = async () => [place], configureItem} = {}) {
  const env = installDOM();
  const previous = get();
  const host = document.body.appendChild(document.createElement('main'));
  let dispose;
  t.after(() => { dispose?.(); replaceSnapshot(previous); env.restore(); });
  replaceSnapshot({
    ...previous, session: {actorId: 'actor_owner'}, worlds: {[world.id]: world}, places: {},
  });
  configureDomainService('world', {
    catalog: async () => ({active_binding: {world_id: world.id}, items: [world]}),
    items: async () => ({world_id: world.id, items: [record], next_cursor: ''}),
    item: async (charId, itemId) => {
      assert.equal(charId, fid);
      assert.equal(itemId, record.id);
      return record;
    },
    places: async (charId, worldId) => {
      assert.equal(charId, fid);
      assert.equal(worldId, world.id);
      return readPlaces();
    },
    configureItem,
  });
  const route = approvedRoute(detail ? `${directory}/${record.id}` : directory);
  assert.equal(route.feature.render, detail ? 'renderWorldItem' : 'renderWorldItems');
  const render = await route.feature.load();
  dispose = render(host, {charId: route.fid, params: route.feature.params, query: Object.fromEntries(route.query)});
  await tick();
  return host;
}

function assertDownload(row, expected) {
  assert.doesNotMatch(row.textContent, /world_item_|world_|place_|res_|req_|actor_|fluctlight_|\bv1\b|版本|尚未记录|未记录|\b(?:id|revision|source[ _]ref|idempotency[ _]key|metrics|correlation[ _]id|updated_at|upper_body)\b|2026-11-22T19:25:00Z|945/i);
  assert.equal(row.querySelector('details, pre, code, .record-facts'), null);
  const links = row.querySelectorAll('a[download]');
  assert.equal(links.length, 1);
  const [download] = links;
  assert.equal(download.textContent, '原始世界物品记录（诊断）');
  assert.equal(download.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = download.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), expected);
}

function assertSummary(row, heading) {
  assert.equal(row.querySelector(heading).textContent, item.name);
  assert.match(row.textContent, /雨天出门时穿。/);
  assert.equal(row.querySelector('[data-domain-ui="badge"]').textContent, '可提出取得请求');
  assert.match(row.textContent, /服饰 · 上装/);
  assert.match(row.textContent, /所属世界：小满的世界 · 取得地点：旧街市集/);
  assert.match(row.textContent, /更新时间：2026-11-23 03:25 \(北京时间\)/);
  assert.equal(row.querySelector('time').dateTime, item.updated_at);
}

test('世界物品详情显示中文摘要，保留编辑和请求入口，并只通过下载提供完整记录', async t => {
  const host = await mount(t, {detail: true});
  assertSummary(host, 'h2');
  assertDownload(host, item);
  assert.match(host.textContent, /此世界保留了自己的物品名称、物品定义/);
  assert.equal(host.querySelector('a[href="#/studio/edit/res_coat"]').textContent, '物品定义');
  assert.equal(host.querySelector(`a[href="${directory}"]`).textContent, '返回物品目录');
  const chat = host.querySelector('a[href*="/chat?say="]');
  assert.equal(chat.textContent, '在对话中提出取得请求');
  assert.match(decodeURIComponent(chat.getAttribute('href')), /目录引用：world_item_coat/);
  const picker = host.querySelector('select[name="placeId"]');
  assert.equal(picker.value, place.id);
  assert.deepEqual(Array.from(picker.options, option => option.textContent), ['不限地点', '旧街市集']);
});

test('世界物品目录按条显示中文摘要并下载原始记录，详情跳转保持可用', async t => {
  const host = await mount(t);
  const row = host.querySelector('[data-items] [data-domain-ui="card"]');
  assert.ok(row, host.textContent);
  assertSummary(row, 'h3');
  assertDownload(row, item);
  assert.match(row.textContent, /目录记录不证明角色已经持有/);
  assert.equal(row.querySelector('a:not([download])').getAttribute('href'), `${directory}/${item.id}`);
});

test('空名称、内部引用和缺失地点使用中文显示，保留当前地点限制并可重读', async t => {
  const record = {...item, name: ' world_other ', updated_at: '', definition: {description: ''}, local_overrides: {}, available: false};
  let fail = true;
  const host = await mount(t, {detail: true, record, readPlaces: async () => {
    if (fail) throw Error('place_private');
    return [place];
  }});
  assert.equal(host.querySelector('h2').textContent, '未命名的世界物品');
  assert.match(host.textContent, /仅限指定地点取得/);
  assert.match(host.textContent, /当前不可取得/);
  assert.doesNotMatch(host.textContent, /更新时间|此世界保留了|place_private/);
  assert.equal(host.querySelector('time, a[href*="/chat?say="]'), null);
  const picker = host.querySelector('select[name="placeId"]');
  assert.equal(picker.value, record.place_id);
  assert.equal(picker.selectedOptions[0].textContent, '保留当前限定地点');
  assertDownload(host, record);
  fail = false;
  host.querySelector('[data-place-retry]').click();
  await tick();
  assert.match(host.textContent, /取得地点：旧街市集/);
  assert.equal(host.querySelector('select[name="placeId"]').selectedOptions[0].textContent, place.name);
});

test('目录中的空名称、不限地点与无效时间保留区别', async t => {
  const record = {...item, name: '   ', updated_at: 'broken-time', place_id: null};
  const host = await mount(t, {record});
  assert.equal(host.querySelector('[data-items] h3').textContent, '未命名的世界物品');
  assert.match(host.textContent, /取得地点不限/);
  assert.doesNotMatch(host.textContent, /更新时间|broken-time/);
  assertDownload(host, record);
});

test('按名称选择取得地点后仍提交地点引用和原修订', async t => {
  const studio = {id: 'place_studio', name: '手工工坊', world_id: world.id};
  const submissions = [];
  const host = await mount(t, {
    detail: true,
    readPlaces: async () => [place, studio],
    configureItem: async (...args) => { submissions.push(args); return item; },
  });
  const overlayHost = document.body.appendChild(document.createElement('div'));
  const unsubscribe = subscribeOverlays(() => overlayHost.replaceChildren(...overlaySnapshot().map(request => request.node)));
  t.after(() => { closeAllOverlays(); unsubscribe(); });
  const picker = host.querySelector('select[name="placeId"]');
  picker.selectedIndex = 2;
  assert.equal(picker.selectedOptions[0].textContent, studio.name);
  host.querySelector('[data-config] button[type="submit"]').click();
  const reason = overlayHost.querySelector('[name="reason"]');
  assert.ok(reason);
  reason.value = '改在工坊取得';
  overlayHost.querySelector('[data-submit]').click();
  await tick();
  assert.deepEqual(submissions, [[fid, item.id, {
    available: true, placeId: studio.id, expectedRevision: item.revision, reason: '改在工坊取得',
  }]]);
});
