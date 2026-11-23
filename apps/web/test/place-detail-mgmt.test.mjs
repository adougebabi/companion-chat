import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get, replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const fid = 'fluctlight_reader';
const world = {id: 'world_reading', name: '雨巷'};
const place = {
  id: 'place_0ce8fbe79c3d7da39219d88485429f28',
  world_id: world.id,
  name: '雨巷书屋',
  alias: '窗边书房',
  desc: '窗边摆着阅读灯 & 花瓶。',
  floor: '二楼',
  atmosphere: '  ',
  furniture: ['木书桌 <靠窗>'],
  source_resource_id: 'res_reading_room',
  revision: 7893,
  source_ref: 'req_place_source',
  idempotency_key: 'retry-reading-room',
  metrics: {latency: 9281},
};

async function mount(t, {record = place, resource, includeRaw = true} = {}) {
  const env = installDOM();
  const previous = get();
  const host = document.body.appendChild(document.createElement('main'));
  const projection = {...record, worldId: world.id, furniture: record.furniture?.join('\n')};
  let dispose;
  t.after(() => { dispose?.(); replaceSnapshot(previous); env.restore(); });
  replaceSnapshot({
    ...previous,
    characters: {[fid]: {id: fid, worldId: world.id, locationId: 'place_unresolved'}},
    worlds: {[world.id]: world},
    places: {[place.id]: projection},
    resources: resource ? {[resource.id]: resource} : {},
  });
  configureDomainService('world', {refresh: async charId => {
    assert.equal(charId, fid);
    return includeRaw ? {places: [record]} : {};
  }});
  const route = approvedRoute(`#/c/${fid}/world/place/${place.id}`);
  assert.equal(route.feature.render, 'renderPlace');
  const render = await route.feature.load();
  dispose = render(host, {charId: route.fid, params: route.feature.params});
  await new Promise(resolve => setImmediate(resolve));
  return host;
}

function assertDownload(host, expected) {
  assert.doesNotMatch(host.textContent, /place_|res_reading_room|req_place_source|retry-reading-room|7893|9281|\b(?:id|revision|source_ref|idempotency_key|metrics)\b|版本|尚未记录|尚未补充|无真实 ID/i);
  const links = host.querySelectorAll('a[download]');
  assert.equal(links.length, 1);
  const [download] = links;
  assert.equal(download.textContent, '原始地点记录（诊断）');
  assert.equal(download.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = download.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), expected);
}

test('地点详情省略空管理折叠并下载含原始 ID 的完整地点记录', async t => {
  const host = await mount(t);
  assert.equal(host.querySelector('h2').textContent, '窗边书房');
  assert.match(host.textContent, /正式名称：雨巷书屋/);
  assert.match(host.textContent, /窗边摆着阅读灯 & 花瓶。/);
  assert.deepEqual([...host.querySelectorAll('.reading-paper dt')].map(node => node.textContent), ['楼层', '陈设']);
  assert.deepEqual([...host.querySelectorAll('.reading-paper dd')].map(node => node.textContent), ['二楼', '木书桌 <靠窗>']);
  assert.equal(host.querySelector('details, pre, code'), null);
  assert.equal(host.querySelector(`a[href="#/c/${fid}/worlds/${world.id}/place/${place.id}/edit"]`).textContent, '编辑地点');
  assertDownload(host, place);
});

test('有可读定义的地点显示世界地点链接并保留诊断下载', async t => {
  const resource = {id: place.source_resource_id, name: '书房 <原始定义>'};
  const host = await mount(t, {resource});
  assert.equal(host.querySelector('details[data-test="mgmt"]'), null);
  const link = [...host.querySelectorAll('a')].find(node => node.textContent === place.name);
  assert.equal(link?.getAttribute('href'), `#/c/${fid}/world/place/${place.id}`);
  assert.equal(host.querySelector('a[href^="#/studio/"]'), null);
  assertDownload(host, place);
});

test('地点详情省略空摘要字段并在仅有现存地点记录时保留诊断下载', async t => {
  const record = {id: place.id, world_id: world.id, name: place.name, desc: ' ', floor: 'place_unresolved'};
  const host = await mount(t, {record, includeRaw: false});
  assert.equal(host.querySelector('h2').textContent, place.name);
  assert.equal(host.querySelector('.reading-paper, details'), null);
  assertDownload(host, {...record, worldId: world.id});
});
