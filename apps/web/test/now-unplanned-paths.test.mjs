import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';
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

const {createWorkspace} = await vite.ssrLoadModule('/src/state/workspace.mjs');
const {default: NowPage} = await vite.ssrLoadModule('/src/react-app/views/NowPage.jsx');
const {default: ChatPage} = await vite.ssrLoadModule('/src/react-app/pages/ChatPage.tsx');
const {default: ProfilePage} = await vite.ssrLoadModule('/src/react-app/pages/CharacterDetailPage.jsx');
const currentFallback = '此刻没有特别的安排';
const hiddenText = /默认未安排|未安排时间|尚未记录|\b(?:id|revision|source_ref|idempotency_key|metrics)\b|private-now-|864209/;
const diagnosticTitle = '原始此刻记录（诊断）';
const privateFields = {
  id: 'private-now-record',
  revision: 864209,
  source_ref: 'private-now-ref',
  idempotency_key: 'private-now-key',
  metrics: {trace: 'private-now-trace'},
};

async function setup(t, activity, schedule = []) {
  const id = 'fixture-character';
  const detail = {
    ...profileDetail({id, name: '小满'}),
    context: {...privateFields, activity, location: '书房'},
    inner_state: {mood: {label: '平静'}},
    current_state: {
      ...privateFields,
      activity: '默认未安排',
      context: {activity: '未安排时间'},
      doing: {text: '整理书桌'},
    },
    schedule: {items: schedule},
  };
  const workspace = createWorkspace();
  t.after(() => workspace.dispose());
  Object.assign(workspace.state, {
    session: {actorId: 'fixture-owner'},
    selectedId: id,
    characters: [{id, name: '小满', status: 'active'}],
  });
  workspace.client.fetcher = async () => assert.fail('测试只允许使用本地夹具');
  workspace.client.detail = async () => detail;
  workspace.client.world = async () => ({world: null});
  workspace.client.selfClaims = async () => ({items: [], next_cursor: ''});
  workspace.loadWardrobe = async () => {};
  workspace.openConversation = async () => {};
  const conversation = workspace.conversation();
  conversation.fluctlightId = id;
  conversation.conversation = {id: 'fixture-conversation'};
  await workspace.detail();
  return {workspace, detail};
}

const button = (root, title) => [...root.querySelectorAll('button')]
  .find(node => node.textContent.trim() === title);

async function assertDownload(t, view, root, title, expected) {
  const trigger = button(root, title);
  assert.ok(trigger);
  await view.click(trigger);
  assert.doesNotMatch(root.textContent, hiddenText);
  assert.equal(root.querySelector('pre, code'), null);
  let blob;
  t.mock.method(URL, 'createObjectURL', value => { blob = value; return 'blob:now-unplanned'; });
  t.mock.method(URL, 'revokeObjectURL', () => {});
  t.mock.method(window.HTMLAnchorElement.prototype, 'click', function () {
    assert.equal(this.download, 'fluctlight-record.json');
  });
  const download = button(trigger.closest('[data-slot="collapsible"]'), '下载原始记录');
  assert.ok(download);
  await view.click(download);
  assert.deepEqual(JSON.parse(await blob.text()), expected);
}

test('此刻页主卡把默认活动换成中性标题，保留状态和原始诊断', async t => {
  const {workspace, detail} = await setup(t, ' 默认未安排 ');
  const {context, inner_state, current_state} = detail;
  const view = await mountReact(vite, NowPage, {workspace, props: {view: {
    character: workspace.current, schedule: {value: []}, emit() {}, readError: () => '',
  }}});
  try {
    const hero = view.host.querySelector('.lp-hero');
    assert.equal(hero.querySelector('h2').textContent, currentFallback);
    assert.deepEqual([...hero.querySelectorAll('.lp-chip')].map(chip => chip.textContent), ['书房', '平静']);
    assert.doesNotMatch(view.host.textContent, hiddenText);
    await assertDownload(t, view, view.host, diagnosticTitle, {context, inner_state, current_state, world: null});
  } finally {
    await view.dispose();
  }
});

test('聊天此刻卡替换未安排标题并跳过下一项兜底，原始日程仍可下载', async t => {
  const start = Date.now() + 3600000;
  const item = {
    ...privateFields,
    activity: '默认未安排',
    start_at: new Date(start).toISOString(),
    end_at: new Date(start + 3600000).toISOString(),
  };
  const {workspace, detail} = await setup(t, '未安排时间', [item]);
  const {context, inner_state, current_state} = detail;
  const view = await mountReact(vite, ChatPage, {workspace, props: {preview: true, onGo() {}}});
  try {
    const card = view.host.querySelector('.replica-moment-card');
    assert.equal(card.querySelector('.companion-moment-copy h3').textContent, currentFallback);
    assert.match(card.querySelector('.companion-moment-copy').textContent, /书房.*平静/);
    const next = button(card, '查看全部').closest('.companion-mini-section');
    assert.equal(next.querySelector('.replica-caption').textContent, '还没有读到接下来的安排。');
    assert.equal(next.querySelector('.companion-next-schedule, time'), null);
    assert.doesNotMatch(card.textContent, hiddenText);
    await assertDownload(t, view, card, diagnosticTitle, {context, inner_state, current_state, schedule: item});
  } finally {
    await view.dispose();
  }
});

test('角色档案「正在做」与实际状态统一显示未安排摘要，诊断保留原值', async t => {
  const {workspace, detail} = await setup(t, '默认未安排');
  const view = await mountReact(vite, ProfilePage, {workspace});
  try {
    const tab = [...view.host.querySelectorAll('[role="tab"]')]
      .find(node => node.textContent.trim() === '生活与此刻');
    assert.ok(tab);
    await act(async () => tab.dispatchEvent(new window.MouseEvent('mousedown', {bubbles: true, button: 0})));
    await view.flush();
    const panel = view.host.querySelector('[role="tabpanel"][data-state="active"]');
    const doing = [...panel.querySelectorAll('.profile-key-value')]
      .find(row => row.firstElementChild.textContent === '正在做');
    assert.equal(doing.lastElementChild.textContent, currentFallback);
    assert.match(panel.textContent, /实际外观与状态.*活动此刻没有特别的安排.*整理书桌/);
    assert.doesNotMatch(panel.textContent, hiddenText);
    await assertDownload(t, view, panel, '原始当前状态记录（诊断）', detail.current_state);
  } finally {
    await view.dispose();
  }
});
