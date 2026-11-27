import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
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
test.after(async () => { await vite.close(); env.restore(); });

const load = path => vite.ssrLoadModule('/src/' + path);
const {default: Life} = await load('react-app/pages/LifePage.jsx');
const {default: Chat} = await load('react-app/pages/ChatPage.tsx');
const {createConversationController} = await load('state/conversation.ts');

const emptyText = '此刻还没有可显示的状态';
const internalText = /尚未记录|此刻状态尚未更新|\b(?:id|world_id|revision|source_ref|idempotency_key|metrics|res_fixture|world_item_fixture|world_fixture|place_fixture|req_fixture|v2|9135)\b|原始幂等值|版本/;
const detailsTitle = '原始此刻记录（诊断）';
const button = (host, label) => [...host.querySelectorAll('button')].find(node => node.textContent.trim() === label);
const worldEntry = host => button(host, '看看这个世界');

const context = {
  id: 'res_fixture', revision: 9135, source_ref: 'req_fixture', place_id: 'place_fixture',
  idempotency_key: '原始幂等值', metrics: {count: 17},
  activity: '阅读新书', location: '   ',
};
const innerState = {id: 'world_item_fixture', revision: 2, mood: {label: '  '}};
const detail = {context, inner_state: innerState, current_state: {context, inner_state: innerState}};
const boundContext = {...context, world_id: 'world_fixture'};
const boundWorld = {id: 'world_fixture', name: '海边小镇', revision: 2};

function fixture() {
  const character = {
    id: 'A', name: '林青', activity: context.activity, location: context.location,
    mood: innerState.mood.label, world: '无法证明绑定的名称', detail, schedule: [],
    life: {appointments: [], diaries: [], memories: []},
  };
  const state = proxy({session: {actorId: 'owner'}, characters: [character], errors: {}, loading: {}, operations: []});
  const conversation = createConversationController();
  conversation.fluctlightId = 'A';
  conversation.conversation = {id: 'local-conversation'};
  const moves = [];
  const workspace = {
    state,
    current: {get value() { return state.characters[0]; }},
    subscribe: listener => subscribe(state, listener),
    getSnapshot: () => snapshot(state),
    currentScope: () => JSON.stringify(['owner', 'A']),
    isCurrent: (owner, fid) => owner === 'owner' && fid === 'A',
    client: {world: async () => ({world: null})},
    services: {lifeClient: {}, memoryClient: {}, diaryClient: {}},
    conversation: () => conversation,
    openConversation: async () => {},
  };
  return {workspace, moves, onGo: page => moves.push(page)};
}

async function assertDownload(view, host, expected) {
  const trigger = button(host, detailsTitle);
  assert.ok(trigger);
  if (trigger.getAttribute('aria-expanded') !== 'true') await view.click(trigger);
  assert.doesNotMatch(host.textContent, internalText);
  assert.equal(host.querySelector('pre, code'), null);

  let blob;
  const originalCreate = URL.createObjectURL;
  const originalRevoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype;
  const originalClick = anchor.click;
  URL.createObjectURL = value => { blob = value; return 'blob:now-test'; };
  URL.revokeObjectURL = () => {};
  anchor.click = function () { assert.equal(this.download, 'fluctlight-record.json'); };
  try {
    await view.click(button(trigger.closest('.product-disclosure'), '下载原始记录'));
    assert.deepEqual(JSON.parse(await blob.text()), expected);
  } finally {
    URL.createObjectURL = originalCreate;
    URL.revokeObjectURL = originalRevoke;
    anchor.click = originalClick;
  }
}

test('此刻页按有值字段显示、全空仅一条状态提示、确认绑定后才显示世界入口，并保留原始下载', async () => {
  const {workspace, moves, onGo} = fixture();
  const props = {page: '此刻', onGo};
  const view = await mountReact(vite, Life, {workspace, props});
  try {
    const hero = view.host.querySelector('.lp-hero');
    assert.equal(hero.querySelector('h2').textContent, '阅读新书');
    assert.equal(hero.querySelector('.lp-chip'), null);
    assert.doesNotMatch(view.host.textContent, /当前情绪|所在世界|无法证明绑定的名称/);
    assert.doesNotMatch(view.host.textContent, internalText);
    assert.equal(worldEntry(view.host), undefined);
    await assertDownload(view, view.host, {...detail, world: null});

    await act(async () => {
      Object.assign(workspace.current.value, {activity: null, location: '\t', mood: ''});
    });
    await view.flush();
    assert.equal(hero.querySelector('h2').textContent, emptyText);
    assert.equal(view.host.textContent.split(emptyText).length - 1, 1);
    assert.equal(hero.querySelector('.lp-chip'), null);
    assert.doesNotMatch(view.host.textContent, internalText);

    workspace.client.world = async () => ({world: boundWorld});
    const boundDetail = {...detail, context: {...boundContext, activity: 'req_fixture · v2', location: 'place_fixture'}};
    await act(async () => {
      Object.assign(workspace.current.value, {detail: boundDetail, activity: boundDetail.context.activity, location: boundDetail.context.location, mood: '平静'});
    });
    await view.flush();
    assert.match(view.host.querySelector('.lp-hero').textContent, /活动详情待确认.*地点详情待确认.*平静/);
    assert.match(view.host.textContent, /当前情绪平静/);
    assert.match(view.host.textContent, /海边小镇/);
    assert.doesNotMatch(view.host.textContent, internalText);
    await assertDownload(view, view.host, {...boundDetail, world: boundWorld});
    await view.click(worldEntry(view.host));
    assert.deepEqual(moves, ['世界']);

    workspace.client.world = async () => ({world: null});
    await act(async () => { workspace.current.value.detail = detail; });
    await view.flush();
    assert.equal(worldEntry(view.host), undefined);
    assert.doesNotMatch(view.host.textContent, /海边小镇|所在世界/);
  } finally {
    await view.dispose();
  }
});

test('聊天此刻卡省略单项空值、全空仅一条状态提示、按详情绑定显示世界入口，并保留原始下载', async () => {
  const {workspace, moves, onGo} = fixture();
  const view = await mountReact(vite, Chat, {workspace, props: {preview: true, onGo}});
  try {
    const card = view.host.querySelector('.replica-moment-card');
    const copy = card.querySelector('.companion-moment-copy');
    assert.equal(copy.querySelector('h3').textContent, '阅读新书');
    assert.equal(copy.querySelector('.companion-moment-location, p'), null);
    assert.doesNotMatch(card.textContent, internalText);
    assert.equal(worldEntry(card), undefined);
    await assertDownload(view, card, detail);

    await act(async () => {
      Object.assign(workspace.current.value, {activity: '', location: undefined, mood: ' '});
    });
    await view.flush();
    assert.equal(copy.textContent, emptyText);
    assert.doesNotMatch(card.textContent, internalText);

    await act(async () => {
      Object.assign(workspace.current.value, {detail: {...detail, context: boundContext}, location: '旧书店', mood: 'v2'});
    });
    await view.flush();
    assert.equal(copy.querySelector('.companion-moment-location').textContent, '旧书店');
    assert.equal(copy.querySelector('h3'), null);
    assert.equal(copy.querySelector('p').textContent, '心情详情待确认');
    assert.doesNotMatch(card.textContent, internalText);
    await view.click(worldEntry(card));
    assert.deepEqual(moves, ['世界']);

    await act(async () => { workspace.current.value.detail = detail; });
    await view.flush();
    assert.equal(worldEntry(card), undefined);
  } finally {
    await view.dispose();
  }
});
