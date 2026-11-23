import test from 'node:test';
import assert from 'node:assert/strict';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get,replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

async function mountInbox(t, items, characters = {}) {
  const env = installDOM();
  const previous = get();
  let dispose;
  t.after(() => {
    dispose?.();
    configureDomainService('operations', {});
    replaceSnapshot(previous);
    env.restore();
  });
  replaceSnapshot({...previous, characters});
  configureDomainService('operations', {
    notifications: async () => ({items: []}),
    notificationCount: async () => ({count: 0}),
    pending: async () => ({groups: [
      {kind: 'capability', status: 'ok', count: items.length, items},
      {kind: 'persona_proposal', status: 'ok', count: 0, items: []},
      {kind: 'studio_apply', status: 'ok', count: 0, items: []},
    ]}),
  });
  const {renderRealInbox} = await import('../src/v3-domain/operations.js');
  const host = document.body.appendChild(document.createElement('main'));
  dispose = renderRealInbox(host, {query: {}});
  await new Promise(resolve => setImmediate(resolve));
  const pending = host.querySelector('[data-pending]');
  assert.ok(pending, host.textContent);
  assert.equal(pending.parentElement.querySelector('h3').textContent, '需要你决定');
  assert.match(pending.textContent, new RegExp(`${items.length} 件事需要你决定`));
  return pending;
}

function assertDownload(row, item) {
  assert.equal(row.querySelector('details, dl, pre, code'), null);
  const downloads = row.querySelectorAll('a[download]');
  assert.equal(downloads.length, 1);
  const [link] = downloads;
  assert.equal(link.textContent, '原始事项记录（诊断）');
  assert.equal(link.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = link.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), item);
}

test('收件箱紧凑列表显示中文来源、角色和北京时间，诊断下载保留原对象', async t => {
  const roleId = 'fluctlight_inbox_role';
  const item = {
    id: 'inbox-private-item',
    source_id: 'inbox-private-source',
    fluctlight_id: roleId,
    title: 'schedule.current_day',
    status: 'failed',
    created_at: '2026-11-23T17:06:19.004+00:00',
    href: '#/diag/task/inbox-private-source?from=inbox&label=%22raw%22',
    revision: 39277,
    source_ref: 'inbox-private-ref',
    idempotency_key: 'inbox-private-retry',
    metrics: {latency: 87654, detail: `原始记录 <&> "引号" '单引号'`},
    correlation_id: 'inbox-private-correlation',
    optional: null,
  };
  const pending = await mountInbox(t, [item], {[roleId]: {id: roleId, name: '小禾 <北馆> & 夏棠'}});
  const row = pending.querySelector('.pending-item');
  assert.ok(row, pending.textContent);
  assert.equal(row.querySelector('.small.muted').textContent, '来源：能力请求 · 角色：小禾 <北馆> & 夏棠 · 2026-11-24 01:06 (北京时间)');
  assert.equal(row.querySelector('time').hasAttribute('datetime'), false);
  assert.equal(row.querySelector('.pending-state').textContent, '执行失败');
  const links = [...row.querySelectorAll('a:not([download])')];
  assert.deepEqual(links.map(link => link.textContent), ['当日日程规划', '打开来源']);
  assert.ok(links.every(link => link.getAttribute('href') === item.href));
  assert.doesNotMatch(pending.textContent, /fluctlight_|inbox-private-|2026-11-23T17:06|#\/diag\/task\/|39277|87654|版本|标识|尚未记录|\b(?:id|href|revision|source[ _](?:id|ref)|idempotency(?:_key)?|metrics|correlation(?:_id)?|created_at)\b/i);
  assertDownload(row, item);
});

test('收件箱紧凑列表标记系统任务并省略未解析的角色，仍可下载原始记录', async t => {
  const empty = {title: '手动确认', status: '', fluctlight_id: '', created_at: null, href: ''};
  const unresolved = {...empty, fluctlight_id: 'fluctlight_missing', created_at: 'invalid-time', href: '  '};
  const items = [empty, unresolved];
  const pending = await mountInbox(t, items);
  const rows = [...pending.querySelectorAll('.pending-item')];
  assert.equal(rows.length, items.length);
  for (const [index, row] of rows.entries()) {
    assert.equal(row.querySelector('b').textContent, '手动确认');
    assert.equal(row.querySelector('.small.muted').textContent, index === 0 ? '来源：能力请求 · 系统任务' : '来源：能力请求');
    assert.equal(row.querySelector('a:not([download]), time, .pending-state'), null);
    assert.doesNotMatch(row.textContent, /角色|fluctlight_|invalid-time|打开来源|尚未记录|时间未知|状态待确认/);
    assert.equal(row.textContent.split('系统任务').length - 1, index === 0 ? 1 : 0);
    assertDownload(row, items[index]);
  }
});
