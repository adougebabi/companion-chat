import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {createServer} from './support/module-server.mjs';
import {installDOM, mountReact} from './helpers/react-dom.mjs';
import {profileDetail} from './support/native-profile-fixture.mjs';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  server: {middlewareMode: true},
});
test.after(async () => { await vite.close(); env.restore(); });

const {createWorkspace, day} = await vite.ssrLoadModule('/src/state/workspace.mjs');
const {default: ChatPage} = await vite.ssrLoadModule('/src/react-app/pages/ChatPage.tsx');
const {ScheduleTimelineItem} = await vite.ssrLoadModule('/src/v3/proto/components/schedule-records.js');

const timezone = 'Asia/Shanghai';
const dayLength = 24 * 60 * 60 * 1000;
const today = Date.now();
const tomorrow = day(new Date(today + dayLength), timezone);
const dayAfter = day(new Date(today + 2 * dayLength), timezone);
const hiddenText = /默认未安排|版本|尚未记录|\b(?:id|revision|source_ref|idempotency_key|metrics|462913)\b|private-schedule-/;
const fallback = {
  id: 'private-schedule-fallback', revision: 462913, source_ref: 'private-schedule-source',
  idempotency_key: 'private-schedule-key', metrics: {trace: 'private-schedule-trace'},
  activity: '默认未安排', status: 'planned', start_at: `${tomorrow}T00:00:00+08:00`,
  end_at: `${dayAfter}T00:00:00+08:00`,
};
const reading = {
  ...fallback, id: 'private-schedule-reading', activity: '去图书馆阅读', location: '图书馆',
  start_at: `${tomorrow}T09:15:00+08:00`, end_at: `${tomorrow}T10:45:00+08:00`,
};

async function mountChat(t, items) {
  const id = 'fixture-character';
  const detail = {
    ...profileDetail({id, name: '小满'}),
    context: {activity: '整理书桌'},
    schedule: {timezone, items},
  };
  const workspace = createWorkspace();
  t.after(() => workspace.dispose());
  Object.assign(workspace.state, {
    session: {actorId: 'fixture-owner'}, selectedId: id,
    characters: [{id, name: '小满', status: 'active'}],
  });
  workspace.client.fetcher = async () => assert.fail('测试只允许使用本地夹具');
  workspace.client.detail = async () => detail;
  workspace.loadWardrobe = async () => {};
  workspace.openConversation = async () => {};
  Object.assign(workspace.conversation(), {fluctlightId: id, conversation: {id: 'fixture-chat'}});
  await workspace.detail();
  const view = await mountReact(vite, ChatPage, {workspace, props: {preview: true, onGo() {}}});
  t.after(() => view.dispose());
  const card = view.host.querySelector('.replica-moment-card');
  const heading = [...card.querySelectorAll('.companion-row-between span')].find(node => node.textContent === '接下来的安排');
  assert.ok(heading);
  const next = heading.closest('.companion-mini-section');
  return {view, card, next};
}

async function assertChatDownload(t, view, card, expectedSchedule) {
  const button = (root, label) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === label);
  const trigger = button(card, '原始此刻记录（诊断）');
  assert.ok(trigger);
  await view.click(trigger);
  assert.doesNotMatch(card.textContent, hiddenText);
  assert.equal(card.querySelector('pre, code'), null);
  let blob, filename;
  t.mock.method(URL, 'createObjectURL', value => { blob = value; return 'blob:now-skip-fallback'; });
  t.mock.method(URL, 'revokeObjectURL', () => {});
  t.mock.method(window.HTMLAnchorElement.prototype, 'click', function () { filename = this.download; });
  await view.click(button(trigger.closest('[data-slot="collapsible"]'), '下载原始记录'));
  assert.equal(filename, 'fluctlight-record.json');
  assert.deepEqual(JSON.parse(await blob.text()).schedule, expectedSchedule);
}

test('聊天此刻卡跳过整日兜底并显示下一条真实安排，两条原始记录均可下载', async t => {
  const {view, card, next} = await mountChat(t, [fallback, reading]);
  assert.equal(next.querySelector('.companion-next-schedule p').textContent, '去图书馆阅读');
  const times = [...next.querySelectorAll('time')];
  assert.deepEqual(times.map(time => time.getAttribute('datetime')), [reading.start_at]);
  assert.equal(next.querySelector('.companion-next-schedule small').textContent, '日程安排');
  assert.equal(next.querySelector('.replica-caption'), null);
  assert.doesNotMatch(next.textContent, /这一天还没有安排|00:00/);
  await assertChatDownload(t, view, card, [fallback, reading]);
});

test('聊天此刻卡只有整日兜底时显示无安排文案，保留原始诊断下载', async t => {
  const {view, card, next} = await mountChat(t, [fallback]);
  assert.equal(next.querySelector('.replica-caption').textContent, '还没有读到接下来的安排。');
  assert.equal(next.querySelector('.companion-next-schedule, time'), null);
  await assertChatDownload(t, view, card, fallback);
});

test('日程规划隐藏整日兜底的计划标签，保留真实全天安排的计划标签和完整下载', () => {
  const duty = {...fallback, id: 'private-schedule-duty', activity: '全天值班', location: '值班室'};
  const timeline = document.createElement('div');
  timeline.innerHTML = [fallback, duty].map(item => ScheduleTimelineItem(item, timezone, {expanded: true})).join('');
  const [unplanned, planned] = timeline.children;
  const heading = row => [...row.querySelector('.schedule-item-heading').children].map(node => node.textContent);
  assert.deepEqual(heading(unplanned), ['这一天还没有安排']);
  assert.equal(unplanned.querySelector('time'), null);
  assert.deepEqual(heading(planned), ['全天值班', '计划']);
  assert.deepEqual([...planned.querySelectorAll('summary time')].map(time => time.getAttribute('datetime')), [duty.start_at, duty.end_at]);
  assert.match(planned.querySelector('summary').textContent, /值班室/);
  assert.doesNotMatch(timeline.textContent, hiddenText);

  for (const [row, record] of [[unplanned, fallback], [planned, duty]]) {
    const link = row.querySelector('a[download="fluctlight-record.json"]');
    assert.equal(link.textContent, '原始日程记录（诊断）');
    const prefix = 'data:application/json;charset=utf-8,';
    assert.ok(link.getAttribute('href').startsWith(prefix));
    assert.deepEqual(JSON.parse(decodeURIComponent(link.getAttribute('href').slice(prefix.length))), record);
  }
});
