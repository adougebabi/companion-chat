import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {isInternalId} from '../src/v3/internal-id.mjs';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get, replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const tick = () => new Promise(resolve => setImmediate(resolve));
const resource = {
  id: 'res_resource42', kind: 'resource', fluctlight_id: '', title: '咖啡馆 <雨天> & 晴天',
  revision: 849, history_count: 3, updated_at: '2026-11-20T06:09:00Z',
  href: '/studio/edit/res_resource42',
  source_ref: 'world_item_source42', idempotency_key: 'req_retry42',
  metrics: {tokens: 578}, correlation_id: 'actor_trace42',
};

async function mount(t, mockRead, hash = '#/data/revisions') {
  const env = installDOM();
  const previous = get();
  const host = document.createElement('div');
  document.body.append(host);
  let dispose;
  t.after(() => { dispose?.(); replaceSnapshot(previous); env.restore(); });
  replaceSnapshot({...previous, session: {actorId: 'actor_owner'},
    characters: {role: {id: 'fluctlight_scope42', name: '小满'}}});
  configureDomainService('dataTools', {revisions: mockRead});
  const route = approvedRoute(hash);
  assert.equal(route.feature.title, '对象修订记录');
  assert.equal(route.feature.render, 'renderRealRevisions');
  const render = await route.feature.load();
  dispose = render(host, {query: Object.fromEntries(route.query)});
  await tick();
  return host;
}

test('修订目录左对齐显示中文摘要，逐条下载完整原始记录并保留业务跳转', async t => {
  const host = await mount(t, async () => ({items: [resource], next_cursor: ''}));
  const row = host.querySelector('[data-records] article');
  assert.ok(row, host.textContent);
  assert.equal(row.dataset.domainUi, 'card');
  for (const token of ['flex-col', 'items-start', 'text-left']) {
    assert.ok(row.classList.contains(token), token);
  }
  assert.equal(row.classList.contains('row'), false);
  const details = row.querySelector('a:not([download])');
  assert.equal(details.getAttribute('href'), `#${resource.href}`);
  assert.equal(details.querySelector('b').textContent, resource.title);
  assert.deepEqual([...details.querySelectorAll('p')].map(p => p.textContent), [
    '创作定义 · 最近更新 2026-11-20 14:09 (北京时间)', '3 条保留历史',
  ]);
  assert.equal(details.querySelector('time').dateTime, resource.updated_at);
  assert.doesNotMatch(row.textContent, /res_|world_item_|place_|req_|actor_|fluctlight_|\bv\d+\b|版本|849|578|尚未记录|\b(?:id|revision|source[ _]ref|idempotency[ _]key|metrics|correlation[ _]id|updated_at)\b|2026-11-20T06:09:00Z/i);
  assert.equal(row.querySelector('details, pre, code, .record-facts'), null);
  const download = row.querySelector('a[download]');
  assert.equal(download.textContent, '原始修订记录（诊断）');
  assert.equal(download.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = download.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), resource);
});

test('缺失或仅含内部引用的标题使用中文兜底，空时间与未知类型不透出原始值', async t => {
  const items = [
    {id: 'res_blank42', kind: 'resource', title: '  ', href: '/studio/edit/res_blank42', history_count: 0},
    {id: 'rel_edge42', kind: 'relationship', title: ' actor_other42 ', updated_at: 'broken-time', href: '/relations/rel_edge42'},
    {id: 'uuid-without-prefix', kind: 'unexpected_kind', title: 'uuid-without-prefix', href: '/unknown/uuid-without-prefix'},
    {id: 'place_hall42', kind: 'place', title: null, href: '/place/place_hall42'},
  ];
  const host = await mount(t, async () => ({items, next_cursor: ''}));
  const records = host.querySelector('[data-records]');
  assert.deepEqual([...records.querySelectorAll('article b')].map(node => node.textContent), [
    '未命名的创作定义', '未命名的关系', '未命名的对象', '未命名的地点字段',
  ]);
  assert.match(records.textContent, /0 条保留历史/);
  assert.doesNotMatch(records.textContent, /res_|actor_|place_|rel_|uuid-without-prefix|unexpected_kind|broken-time|尚未记录|时间未知|最近更新/);
  assert.equal(records.querySelector('time'), null);
});

test('精确对象 ID 和标题仍可按路由预填、提交筛选和分页', async t => {
  const reads = [];
  const host = await mount(t, async filters => {
    reads.push(filters);
    return {items: [resource], next_cursor: filters.cursor ? '' : 'next-page'};
  }, '#/data/revisions?objectId=res_resource42&kind=resource');
  assert.deepEqual(reads, [{kind: 'resource', objectId: resource.id, cursor: ''}]);
  const form = host.querySelector('[data-filter]');
  assert.equal(form.elements.namedItem('objectId').value, resource.id);
  host.querySelector('[data-page-next]').click();
  await tick();
  assert.deepEqual(reads.at(-1), {kind: 'resource', objectId: resource.id, cursor: 'next-page'});
  form.elements.namedItem('objectId').value = ' place_hall42 ';
  form.elements.namedItem('q').value = ' 大厅 ';
  form.elements.namedItem('fluctlightId').value = 'fluctlight_scope42';
  const formData = globalThis.FormData;
  try {
    globalThis.FormData = window.FormData;
    form.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
  } finally {
    globalThis.FormData = formData;
  }
  await tick();
  assert.deepEqual(reads.at(-1), {
    kind: 'resource', fluctlightId: 'fluctlight_scope42', objectId: 'place_hall42', q: '大厅', cursor: '',
  });
  assert.doesNotMatch(host.querySelector('[data-records]').textContent, /res_resource42|place_hall42|fluctlight_scope42/);
});

test('内部 ID 识别覆盖常见前缀，保留普通中英文标题', () => {
  for (const id of ['res_123', ' world_item_abc ', 'place_1', 'REQ_42', 'actor_u12', 'fluctlight_a-b', 'mem_12', 'rel_12']) {
    assert.equal(isInternalId(id), true, id);
  }
  for (const name of [null, 42, '', 'res_', '咖啡馆', 'world_item_展柜', 'Rainy Café', 'The res_123 exhibition']) {
    assert.equal(isInternalId(name), false, String(name));
  }
});
