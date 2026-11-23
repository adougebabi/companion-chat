import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get,replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

const roleId = 'fluctlight_pending_role';

async function mount(t, groups, characters) {
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
  configureDomainService('operations', {pending: async () => ({groups})});
  const route = approvedRoute('#/diag/pending');
  assert.equal(route.feature.render, 'renderRealPending');
  const render = await route.feature.load();
  const host = document.body.appendChild(document.createElement('main'));
  dispose = render(host, {query: Object.fromEntries(route.query)});
  await new Promise(resolve => setImmediate(resolve));
  return host;
}

function assertDownload(row, item) {
  assert.equal(row.querySelector('details, dl, pre, code'), null);
  const links = row.querySelectorAll('a[download]');
  assert.equal(links.length, 1);
  const [link] = links;
  assert.equal(link.textContent, '原始事项记录（诊断）');
  assert.equal(link.download, 'fluctlight-record.json');
  const href = link.getAttribute('href');
  const prefix = 'data:application/json;charset=utf-8,';
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), item);
}

test('完整待处理显示中文来源、角色和北京时间，诊断下载保留原对象', async t => {
  const item = {
    id: 'workflow:private-pending-id',
    source_id: 'private-source-id',
    fluctlight_id: roleId,
    title: 'persona.compile',
    status: 'failed',
    href: '#/diag/task/private-source-id?from=pending&label=%22original%22',
    created_at: '2026-11-23T17:06:19.004+00:00',
    revision: 98765,
    source_ref: 'private-source-ref',
    idempotency_key: 'private-retry-key',
    metrics: {latency: 76543, detail: '原始记录 <&> "引号"'},
    correlation_id: 'private-correlation-id',
    optional: null,
  };
  const host = await mount(t, [{kind: 'failed_task', status: 'ok', count: 1, items: [item]}], {
    [roleId]: {id: roleId, name: '小禾 <北馆> & 夏棠'},
  });
  const row = host.querySelector('.pending-item');
  assert.ok(row, host.textContent);
  assert.equal(row.querySelector('b').textContent, '人格资料整理');
  assert.equal(row.querySelector('.pending-state').textContent, '执行失败');
  assert.match(row.textContent, /后台服务未完成这项工作/);
  assert.match(row.textContent, /来源：失败任务 · 角色：小禾 <北馆> & 夏棠/);
  assert.equal(row.querySelector('time').textContent, '2026-11-24 01:06 (北京时间)');
  assert.equal(row.querySelector('time').hasAttribute('datetime'), false);
  const navigation = [...row.querySelectorAll('a:not([download])')];
  assert.deepEqual(navigation.map(link => link.textContent), ['人格资料整理', '打开来源']);
  assert.ok(navigation.every(link => link.getAttribute('href') === item.href));
  assert.doesNotMatch(host.textContent, /fluctlight_|private-(?:pending|source|retry|correlation)|2026-11-23T17:06|#\/diag\/task\/|98765|76543|版本|标识|事项来源与标识|尚未记录|\b(?:id|href|revision|source[ _](?:id|ref)|idempotency(?:_key)?|metrics|correlation(?:_id)?|created_at)\b/i);
  assertDownload(row, item);
});

test('完整待处理省略空值和未解析的角色，仍下载完整诊断记录', async t => {
  const empty = {title: '手动确认', fluctlight_id: '', created_at: null, href: '', status: ''};
  const unresolved = {...empty, fluctlight_id: 'fluctlight_missing', created_at: 'invalid-time'};
  const host = await mount(t, [{kind: 'capability', status: 'ok', count: 2, items: [empty, unresolved]}], {});
  const rows = [...host.querySelectorAll('.pending-item')];
  assert.equal(rows.length, 2);
  for (const [index, row] of rows.entries()) {
    assert.equal(row.querySelector('.small.muted').textContent, '来源：能力请求');
    assert.equal(row.querySelector('a:not([download]), time, .pending-state'), null);
    assert.doesNotMatch(row.textContent, /角色|fluctlight_|invalid-time|打开来源|系统任务|尚未记录|时间未知|状态待确认/);
    assertDownload(row, [empty, unresolved][index]);
  }
});
