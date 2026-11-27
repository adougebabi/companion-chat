import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get,replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const namedId = 'fluctlight_named_role';
const missingId = 'fluctlight_missing_role';
const unnamedId = 'fluctlight_unnamed_role';
const rawFields = {
  id: 'private-pending-id',
  revision: 39277,
  source_ref: 'private-source-ref',
  idempotency_key: 'private-retry-key',
  metrics: {latency: 87654},
};
const cases = [
  {item: {title: '系统操作确认'}, summary: '来源：能力请求 · 系统任务'},
  {item: {title: '日程确认', fluctlight_id: namedId}, summary: '来源：能力请求 · 角色：小禾'},
  {item: {title: '请求确认', fluctlight_id: missingId}, summary: '来源：能力请求'},
  {item: {title: '设定确认', fluctlight_id: unnamedId}, summary: '来源：能力请求'},
];

async function checkPendingItems(t, loadRender) {
  const env = installDOM();
  const previous = get();
  let dispose;
  t.after(() => {
    dispose?.();
    configureDomainService('operations', {});
    replaceSnapshot(previous);
    env.restore();
  });
  replaceSnapshot({...previous, characters: {
    [namedId]: {id: namedId, name: ' 小禾 '},
    [unnamedId]: {id: unnamedId, name: ' '},
  }});
  const items = cases.map(({item}) => ({...rawFields, ...item}));
  configureDomainService('operations', {
    notifications: async () => ({items: []}),
    notificationCount: async () => ({count: 0}),
    pending: async () => ({groups: [
      {kind: 'capability', status: 'ok', count: items.length, items},
      {kind: 'persona_proposal', status: 'ok', count: 0, items: []},
      {kind: 'studio_apply', status: 'ok', count: 0, items: []},
    ]}),
  });
  const render = await loadRender();
  const host = document.body.appendChild(document.createElement('main'));
  dispose = render(host, {query: {}});
  await new Promise(resolve => setImmediate(resolve));

  const pending = host.querySelector('[data-pending]');
  assert.ok(pending, host.textContent);
  assert.match(pending.textContent, /4 件事需要你决定/);
  const rows = [...pending.querySelectorAll('.pending-item')];
  assert.equal(rows.length, items.length);
  for (const [index, row] of rows.entries()) {
    const item = items[index];
    assert.equal(row.querySelector('b')?.textContent, item.title);
    assert.equal(row.querySelector('.small.muted')?.textContent, cases[index].summary);
    assert.equal(row.textContent.includes('系统任务'), index === 0, item.title);
    assert.doesNotMatch(row.textContent, /fluctlight_|private-|39277|87654|版本|尚未记录|\b(?:id|revision|source_ref|idempotency(?:_key)?|metrics)\b/i);
    const download = row.querySelector('a[download="fluctlight-record.json"]');
    assert.equal(download?.textContent, '原始事项记录（诊断）');
    const href = download.getAttribute('href');
    const prefix = 'data:application/json;charset=utf-8,';
    assert.ok(href.startsWith(prefix));
    assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), item);
  }
}

test('诊断待处理按角色关联显示系统任务、角色名或仅显示来源', async t => {
  await checkPendingItems(t, () => approvedRoute('#/diag/pending').feature.load());
});

test('收件箱需要你决定按角色关联显示系统任务、角色名或仅显示来源', async t => {
  await checkPendingItems(t, async () => (await import('../src/v3-domain/operations.js')).renderRealInbox);
});
