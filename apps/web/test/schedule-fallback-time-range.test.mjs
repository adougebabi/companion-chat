import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {installDOM, mountReact} from './helpers/react-dom.mjs';

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

const {default: SchedulePage} = await vite.ssrLoadModule('/src/react-app/pages/SchedulePage.jsx');
const {default: NowPage} = await vite.ssrLoadModule('/src/react-app/views/NowPage.jsx');
const {day, clock} = await vite.ssrLoadModule('/src/state/workspace.mjs');
const {scheduleProjection} = await vite.ssrLoadModule('/src/approved-ui/presentation.mjs');
const timezone = 'Asia/Shanghai';
const date = day(new Date(), timezone);
const nextDate = new Date(Date.parse(date) + 86400000).toISOString().slice(0, 10);
const midnight = `${date}T00:00:00+08:00`;
const nextMidnight = `${nextDate}T00:00:00+08:00`;
const regularStart = `${date}T09:15:00+08:00`;
const regularEnd = `${date}T10:45:00+08:00`;
const privateFields = /\b(?:id|revision|source_ref|idempotency_key|metrics)\b|private-|尚未记录/;
const button = (root, label) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === label);

function fixture(items) {
  const schedule = {id: 'private-schedule', local_date: date, timezone, revision: 37, items};
  const character = {id: 'reader', name: '小满', status: '正常', timezone};
  const state = {session: {actorId: 'owner'}, loading: {}};
  const workspace = {
    current: {value: character},
    state,
    subscribe: () => () => {},
    getSnapshot: () => state,
    clock,
    isCurrent: () => true,
    detail: async () => character,
    services: {lifeClient: {
      schedules: async () => ({today: date, timezone, days: [{local_date: date, schedule}]}),
      scheduleItemPending: () => null,
      schedulePending: () => null,
      requests: async () => ({items: [], next_cursor: ''}),
    }},
  };
  return {schedule, character, workspace};
}

test('日程列表和详情保留条目，只展示可用的时间段，完整记录仅通过诊断下载', async () => {
  const items = [
    {activity: '默认未安排', start_at: midnight, end_at: midnight, location: '家中'},
    {activity: '整理书架', end_at: regularEnd, location: '书房', scene: '家务'},
    {activity: '去花店', start_at: 'invalid-time', end_at: regularEnd, location: '花店'},
    {activity: '阅读', start_at: regularStart, end_at: regularEnd, location: '图书馆'},
    {activity: '夜间值班', start_at: midnight, end_at: nextMidnight, location: '值班室'},
    {activity: '晨间记录', start_at: regularEnd, end_at: regularStart},
    {activity: '写信', start_at: regularStart, end_at: '2026-11-23T99:00:00+08:00'},
  ].map((item, index) => ({
    id: `private-item-${index}`, revision: 37, place_id: 'private-place', source_ref: 'private-source',
    idempotency_key: 'private-key', metrics: {trace: 'private-metric'}, action_status: 'completed', ...item,
  }));
  const {workspace} = fixture(items);
  const view = await mountReact(vite, SchedulePage, {workspace, props: {route: {query: new URLSearchParams({date})}}});
  try {
    const rows = [...view.host.querySelectorAll('.lp-timeline-item')];
    assert.equal(rows.length, items.length);
    assert.doesNotMatch(view.host.textContent, privateFields);
    const noTime = (row) => {
      assert.equal(row.querySelector('time'), null);
      assert.doesNotMatch(row.textContent, /\d{2}:\d{2}|—|invalid-time|默认未安排/);
    };
    for (const index of [0, 1, 2, 5, 6]) noTime(rows[index]);
    assert.match(rows[0].textContent, /这一天还没有安排.*已完成.*家中/);
    assert.match(rows[1].textContent, /整理书架.*书房.*时间待核对/);
    assert.match(rows[2].textContent, /去花店.*花店.*时间待核对/);
    assert.match(rows[5].textContent, /晨间记录.*时间待核对/);
    assert.match(rows[6].textContent, /写信.*时间待核对/);
    assert.match(rows[3].textContent, /阅读.*09:15—10:45.*图书馆/);
    assert.match(rows[4].textContent, /夜间值班.*00:00—00:00.*值班室/);

    for (const index of [0, 1, 2]) {
      await view.click(rows[index].querySelector('.lp-timeline-card'));
      const dialog = document.querySelector('[role="dialog"]');
      noTime(dialog);
      assert.doesNotMatch(dialog.textContent, privateFields);
      assert.match(dialog.textContent, index === 0 ? /这一天还没有安排.*地点家中/ : /时间待核对/);
      assert.ok(button(dialog, '原始日程记录（诊断）'));
      if (index === 1) {
        assert.match(dialog.textContent, /整理书架.*地点书房.*类型家务.*已完成/);
        await view.click(button(dialog, '原始日程记录（诊断）'));
        assert.doesNotMatch(dialog.textContent, privateFields);
        assert.equal(dialog.querySelector('pre'), null);
        const create = URL.createObjectURL;
        const revoke = URL.revokeObjectURL;
        const anchor = window.HTMLAnchorElement.prototype;
        const click = anchor.click;
        let blob, filename;
        URL.createObjectURL = value => { blob = value; return 'blob:schedule-item'; };
        URL.revokeObjectURL = () => {};
        anchor.click = function () { filename = this.download; };
        try {
          await view.click(button(dialog, '下载原始记录'));
          assert.equal(filename, 'fluctlight-record.json');
          assert.deepEqual(JSON.parse(await blob.text()), items[index]);
        } finally {
          URL.createObjectURL = create;
          URL.revokeObjectURL = revoke;
          anchor.click = click;
        }
      }
      await view.click(button(dialog, '关闭'));
    }
  } finally {
    await view.dispose();
  }
});

test('此刻摘要以中文展示后端整天兜底，下一项保留有效业务信息并省略无效时间', async () => {
  const fallback = {id: 'private-fallback', activity: '默认未安排', start_at: midnight, end_at: nextMidnight, location: '家中'};
  const {schedule, character, workspace} = fixture([fallback]);
  schedule.reschedule_policy = {default_initial_schedule: true, replaceable: true};
  const futureFallback = {...fallback, start_at: nextMidnight, end_at: new Date(Date.parse(nextMidnight) + 86400000).toISOString()};
  const futureInvalid = {id: 'private-invalid', activity: '未定时间的安排', start_at: nextMidnight};
  const projected = [
    scheduleProjection(fallback, schedule, clock),
    ...[futureFallback, futureInvalid].map(item => scheduleProjection(item, {...schedule, local_date: nextDate}, clock)),
  ];
  const view = await mountReact(vite, NowPage, {workspace, props: {view: {
    character: {value: character}, schedule: {value: projected}, emit() {}, readError: () => '',
  }}});
  try {
    const list = view.host.querySelector('[data-testid="schedule-list"]');
    assert.match(list.textContent, /这一天还没有安排.*家中/);
    assert.equal(list.querySelector('time'), null);
    assert.doesNotMatch(list.textContent, /00:00|默认未安排|当前计划时段/);
    const next = view.host.querySelector('.replica-next-time');
    assert.equal(next.textContent, '时间待核对');
    assert.equal(next.nextElementSibling.textContent, '未定时间的安排');
    assert.doesNotMatch(view.host.textContent, privateFields);
    assert.ok(button(view.host, '原始此刻记录（诊断）'));
  } finally {
    await view.dispose();
  }
});
