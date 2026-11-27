import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get, replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const charId = 'fluctlight_test';
const worldId = 'world_test';
const placeId = 'place_test';
const resourceId = 'res_place_definition';
const placeHref = `#/c/${charId}/world/place/${placeId}`;
const place = {
  id: placeId,
  world_id: worldId,
  name: '河边 <书屋>',
  desc: '河边有一排临窗座位。',
  source_resource_id: resourceId,
  revision: 456789,
  source_ref: 'internal-source',
  idempotency_key: 'internal-retry',
  metrics: {duration: 987654},
};

async function mount(t, record, resources = {}) {
  const env = installDOM();
  const previous = get();
  const host = document.body.appendChild(document.createElement('main'));
  let dispose;
  t.after(() => { dispose?.(); replaceSnapshot(previous); env.restore(); });
  replaceSnapshot({
    ...previous,
    characters: {[charId]: {id: charId, worldId, locationId: placeId}},
    worlds: {[worldId]: {id: worldId, name: '河畔'}},
    places: record ? {[placeId]: record} : {},
    resources,
  });
  configureDomainService('world', {refresh: async requestedCharId => {
    assert.equal(requestedCharId, charId);
    return {places: record ? [record] : []};
  }});
  const route = approvedRoute(placeHref);
  assert.equal(route.feature.render, 'renderPlace');
  const render = await route.feature.load();
  dispose = render(host, {charId: route.fid, params: route.feature.params});
  await new Promise(resolve => setImmediate(resolve));
  return host;
}

function assertNoInternalText(host) {
  assert.doesNotMatch(host.textContent, /place_test|world_test|res_place_definition|internal-source|internal-retry|456789|987654|\b(?:id|revision|source_ref|idempotency_key|metrics)\b|版本|尚未记录|地点设定|定义：/);
  assert.equal(host.querySelector('a[href^="#/studio/"]'), null);
}

test('已归属世界的地点用地点名称链接到世界地点详情并可下载原始记录', async t => {
  const host = await mount(t, place);
  assert.equal(host.querySelector(`a[href="${placeHref}"]`)?.textContent, place.name);
  assert.equal(host.querySelector('.reading-paper .memory-content')?.textContent, place.desc);
  assertNoInternalText(host);

  const download = host.querySelector('a[download="fluctlight-record.json"]');
  assert.equal(download?.textContent, '原始地点记录（诊断）');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = download.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), place);
});

test('已归属世界但没有可读名称的地点使用「查看这个地点」', async t => {
  const record = {...place, worldId, world_id: undefined, name: placeId};
  const host = await mount(t, record);
  assert.equal(host.querySelector(`a[href="${placeHref}"]`)?.textContent, '查看这个地点');
  assertNoInternalText(host);
});

test('只有定义或地点归属缺失时不显示地点链接和内部 ID', async t => {
  const resources = {[resourceId]: {id: resourceId, name: '书屋定义'}};
  const cases = [
    ['只有定义资源', undefined],
    ['地点缺世界字段', {...place, world_id: undefined}],
    ['地点属于另一世界', {...place, world_id: 'world_other'}],
  ];
  for (const [description, record] of cases) {
    await t.test(description, async subtest => {
      const host = await mount(subtest, record, resources);
      assert.equal(host.querySelector('h2')?.textContent, '当前范围未找到该地点');
      assert.equal(host.querySelector('a'), null);
      assertNoInternalText(host);
    });
  }
});
