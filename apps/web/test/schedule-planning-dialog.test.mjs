import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  server: {middlewareMode: true},
});
test.after(async () => {
  await vite.close();
  env.restore();
});

const {default: DomainPage} = await vite.ssrLoadModule('/src/react-app/pages/DomainPage.tsx');
const {featureRoute} = await vite.ssrLoadModule('/src/approved-ui/feature-routes.mjs');
const {configureDomainService} = await vite.ssrLoadModule('/src/v3/proto/services.js');

const fid = 'schedule-test-role';
const date = '2026-11-23';
const path = '/c/' + fid + '/life/schedule/planning';
const query = new URLSearchParams();
const feature = featureRoute(path, query);
await feature.load();
const button = (root, label) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === label);
const visibleText = node => {
  const copy = node.cloneNode(true);
  copy.querySelectorAll('[hidden], [data-domain-carrier]').forEach(child => child.remove());
  return copy.textContent;
};
const internal = /尚未记录|版本|\b(?:82|id|revision|source[_ ]ref|idempotency[_ ]key|metrics)\b|private-/;
const rawFields = {revision: 82, source_ref: 'private-source', idempotency_key: 'private-key', metrics: {trace: 'private-trace'}, note: null};
const suggestion = {
  ...rawFields, id: 'private-suggestion', kind: 'schedule_item_suggestion', status: 'pending',
  payload: {activity: '改到下午去花店'},
  decision_reason: '先等她确认时间。',
};

function assertDownload(root, label, record) {
  const link = [...root.querySelectorAll('a[download]:not([data-domain-carrier])')]
    .find(node => node.textContent.trim() === label);
  assert.ok(link);
  assert.equal(link.getAttribute('download'), 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = link.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), record);
}

async function mount(t, {requests, items = []} = {}) {
  const schedule = {items};
  configureDomainService('life', {
    detail: async () => ({schedule}),
    schedules: async () => ({timezone: 'Asia/Shanghai', today: date, from: date, days: [{local_date: date, schedule}]}),
    promises: async () => ({items: [], next_cursor: ''}),
    ...(requests ? {requests} : {}),
  });
  const workspace = {state: {session: {actorId: 'schedule-test-owner'}, selectedId: fid}};
  const route = {page: '日程', path, query, feature};
  const view = await mountReact(vite, DomainPage, {workspace, props: {route}});
  t.after(() => view.dispose());

  const trigger = () => button(view.host, '约定与日程设置');
  const dialog = () => view.host.querySelector('[data-slot="sheet-content"]');
  const open = async () => {
    await view.click(trigger());
    assert.equal(dialog().querySelector('[role=tab][aria-selected=true]').textContent, '当天安排');
    return dialog();
  };
  return {view, dialog, open};
}

for (const {name, settle, expected, failure, hasRecord} of [
  {name: '空结果', settle: resolve => resolve({items: [], next_cursor: ''}), expected: '当前读取范围没有待确认的建议。'},
  {name: '有建议', settle: resolve => resolve({items: [suggestion], next_cursor: ''}), expected: '改到下午去花店 · 等她确认', hasRecord: true},
  {name: '读取报错', settle: (_resolve, reject) => reject(new Error('private-error')), expected: '日程调整建议读取失败，请稍后重试。', failure: true},
  {name: '服务返回 404', settle: (_resolve, reject) => reject(Object.assign(new Error('private-endpoint'), {status: 404})), expected: '当前服务未提供日程调整建议。', failure: true},
  {name: '返回格式错误', settle: resolve => resolve({message: 'private-invalid-response'}), expected: '日程调整建议返回格式不正确。', failure: true},
]) {
  test('调整建议在 Sheet 关闭时完成读取后可见：' + name, async t => {
    let resolve, reject;
    const response = new Promise((yes, no) => { resolve = yes; reject = no; });
    let calls = 0;
    const {view, dialog, open} = await mount(t, {requests: () => { calls++; return response; }});
    const sheet = await open();
    const panel = () => dialog().querySelector('[data-schedule-suggestions]');
    assert.equal(panel().querySelector('[data-domain-ui="skeleton"]').textContent, '正在读取…');

    await view.click(button(sheet, '关闭'));
    assert.equal(dialog(), null);
    assert.equal(calls, 1);
    await act(async () => settle(resolve, reject));
    await open();
    assert.equal(calls, 1);
    assert.equal(panel().querySelector('[data-domain-ui="skeleton"]'), null);
    assert.doesNotMatch(visibleText(panel()), /正在读取…/);
    assert.ok(visibleText(panel()).includes(expected));
    assert.doesNotMatch(visibleText(panel()), internal);
    if (failure) {
      assert.equal(panel().querySelector('[role=alert]').textContent, expected);
      assert.ok(button(panel(), '重新读取'));
    }
    if (hasRecord) {
      assert.match(visibleText(panel()), /先等她确认时间/);
      assertDownload(panel(), '原始日程调整建议记录（诊断）', suggestion);
    }
  });
}

test('调整建议接口未注册时显示中文错误并清除骨架', async t => {
  const {open} = await mount(t);
  const panel = (await open()).querySelector('[data-schedule-suggestions]');
  assert.equal(panel.querySelector('[data-domain-ui="skeleton"]'), null);
  assert.equal(panel.querySelector('[role=alert]').textContent, '当前服务未提供日程调整建议。');
  assert.doesNotMatch(visibleText(panel), /正在读取…|life\.requests/);
});

test('首次打开 Sheet 时显示已完成的建议读取，失败后可以重试', async t => {
  let resolve;
  let calls = 0;
  const firstRead = new Promise(yes => { resolve = yes; });
  const {view, dialog, open} = await mount(t, {requests: async () => {
    calls++;
    if (calls === 1) return firstRead;
    return {items: [suggestion], next_cursor: null};
  }});
  assert.equal(dialog(), null);
  assert.equal(calls, 1);
  await act(async () => resolve({message: 'private-response'}));

  const panel = (await open()).querySelector('[data-schedule-suggestions]');
  assert.equal(panel.querySelector('[data-domain-ui="skeleton"]'), null);
  assert.equal(panel.querySelector('[role=alert]').textContent, '日程调整建议返回格式不正确。');
  await view.click(button(panel, '重新读取'));
  assert.equal(calls, 2);
  assert.equal(panel.querySelector('[role=alert], [data-domain-ui="skeleton"]'), null);
  assert.match(visibleText(panel), /改到下午去花店 · 等她确认/);
});

test('未安排条目用中文摘要且不显示计划状态，完整记录可下载', async t => {
  const nextDay = '2026-11-24';
  const fallback = {
    ...rawFields, id: 'private-item', activity: '默认未安排', status: 'planned', scene: '未安排',
    start_at: date + 'T00:00:00+08:00', end_at: nextDay + 'T00:00:00+08:00',
  };
  const business = {
    ...fallback, id: 'private-duty', activity: '夜间值班', location: '值班室', scene: '轮值', action_status: 'completed',
  };
  const {view, open} = await mount(t, {items: [fallback, business]});
  const timeline = (await open()).querySelector('.schedule-timeline');
  const trigger = [...timeline.querySelectorAll('button')].find(node => node.textContent.includes('这一天还没有安排'));
  assert.ok(trigger);
  assert.equal(trigger.querySelector('time'), null);
  await view.click(trigger);
  const detail = timeline.querySelector('.schedule-timeline-body');
  assert.match(visibleText(detail), /未安排/);
  assert.equal(detail.querySelector('time'), null);
  const summary = visibleText(trigger) + ' ' + visibleText(detail);
  assert.doesNotMatch(summary, /默认未安排|00:00|当前计划时段|地点未记录|场景未记录/);
  // ui-1123c-02: an unplanned day shows no plan-status label at all (it used to
  // read as 「这一天还没有安排计划未安排」), so neither a duplicate nor a glued label.
  assert.equal((summary.match(/计划/g) || []).length, 0, summary);
  assert.doesNotMatch(summary, /计划未安排|没有安排计划/);
  assert.doesNotMatch(visibleText(timeline), internal);
  assertDownload(detail, '原始日程记录（诊断）', fallback);
  const duty = [...timeline.querySelectorAll('button')].find(node => node.textContent.includes('夜间值班'));
  assert.deepEqual([...duty.querySelectorAll('time')].map(node => node.textContent), ['2026/11/23 00:00', '2026/11/24 00:00']);
  assert.match(visibleText(duty), /夜间值班.*值班室/);
  await view.click(duty);
  assert.match(visibleText(timeline), /已完成\s*轮值/);
});
