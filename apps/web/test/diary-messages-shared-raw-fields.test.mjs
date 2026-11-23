import test from 'node:test';
import assert from 'node:assert/strict';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {proxy, snapshot, subscribe} from 'valtio/vanilla';
import {installDOM, mountReact} from './helpers/react-dom.mjs';
import {renderRealShared} from '../src/v3-domain/diary.js';
import {configureDomainService} from '../src/v3/proto/services.js';
import * as store from '../src/v3/proto/store.js';
import {closeOverlay, overlaySnapshot, subscribeOverlays} from '../src/state/overlays.ts';

const env = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL('../', import.meta.url)),
  envFile: false,
  appType: 'custom',
  logLevel: 'silent',
  server: {middlewareMode: true},
});
test.after(async () => { await vite.close(); env.restore(); });
const {default: DiaryPage} = await vite.ssrLoadModule('/src/react-app/pages/DiaryPage.jsx');
const fid = 'fluctlight_private', owner = 'actor_private';
const at = '2026-11-23T01:45:00Z', until = '2026-11-24T02:15:00Z';
const metadata = {
  revision: 19283, protocol_version: 'shared.v927', source_ref: 'world_item_private',
  idempotency_key: 'private_replay_key', correlation_id: 'private_trace_marker',
  metrics: {attempted: 58319}, unknown_data: {place_id: 'place_private'},
};
const message = {
  id: 'req_private', fluctlight_id: fid, kind: 'diary_reply', created_at: at,
  status: 'completed', decided_by: 'actor_agent_private',
  payload: {text: "我记得那天的风 <很轻> & 很暖。"},
  result: {reason: '留言已经读完，之后会再想想。', source_fact_id: 'res_fact_private'},
  result_refs: ['res_result_private'], ...metadata,
};
const emptyMessage = {id: 'req_empty', fluctlight_id: fid, status: 'pending', payload: {text: ''}, created_at: null, result: {}};

const button = (host, label) => [...host.querySelectorAll('button')].find(node => node.textContent.trim() === label);
function assertClean(host) {
  assert.doesNotMatch(host.textContent, /\b(?:id|revision|protocol.version|source.ref|idempotency(?:.key)?|metrics|correlation(?:.id)?|decided.by|result.refs|dateTime|completed|pending|diary_reply|remote_communication|footprints|as_of|timezone|unknown.data)\b|版本|尚未记录|时间未记录|时间未知|留言原文待读取|尚无关系记录|req_|actor_|fluctlight_|res_|place_|world_item_|private_replay_key|private_trace_marker|19283|58319|\bv927\b|2026-11-2[34]T/i);
  assert.equal(host.querySelector('.record-facts, pre, code'), null);
}

test('日记留言列表和跟进区显示中文处理结果，并分别下载原样记录', async () => {
  const entry = {
    id: 'diary_fixture', fluctlight_id: fid, date: '2026-11-23', status: 'ready', source: 'agent',
    body: '今天记下了散步的事。', revision: 1, active_attempt_id: 'attempt_fixture',
    annotation: {note: '', favorite: false, revision: 1},
  };
  const current = {...message, revision: message.revision + 1, tasks: [{task_id: 'res_task_private', status: 'processing'}]};
  const state = proxy({session: {actorId: owner}, selectedId: fid, characters: [{id: fid, name: '林青', status: '正常'}]});
  const reads = [];
  const workspace = {
    state, current: {get value() {return state.characters[0];}},
    subscribe: fn => subscribe(state, fn), getSnapshot: () => snapshot(state),
    isCurrent: (actor, role) => actor === owner && role === fid,
    currentScope: () => JSON.stringify([owner, fid]),
    client: {
      listDiary: async () => ({items: [entry], next_cursor: ''}),
      diary: async () => entry,
      diaryReplies: async () => ({items: [message, emptyMessage], next_cursor: ''}),
    },
    services: {lifeClient: {request: async (...args) => { reads.push(args); return current; }}},
  };
  const view = await mountReact(vite, DiaryPage, {workspace});
  const create = URL.createObjectURL, revoke = URL.revokeObjectURL;
  const anchor = window.HTMLAnchorElement.prototype, click = anchor.click;
  const blobs = [], filenames = [];
  URL.createObjectURL = blob => { blobs.push(blob); return 'blob:diary-message-fixture'; };
  URL.revokeObjectURL = () => {};
  anchor.click = function() { filenames.push(this.download); };
  async function download(host, expected) {
    const toggle = button(host, '原始留言处理记录（诊断）');
    assert.ok(toggle);
    await view.click(toggle);
    assertClean(host);
    await view.click(button(host, '下载原始记录'));
    assert.equal(filenames.at(-1), 'fluctlight-record.json');
    assert.deepEqual(JSON.parse(await blobs.at(-1).text()), expected);
  }
  try {
    const section = view.host.querySelector('[aria-label="留言与实际处理记录"]');
    const [full, empty] = section.querySelectorAll('.lp-source-card');
    assert.match(full.textContent, /处理状态：留言处理已结束/);
    assert.match(full.textContent, /处理结果：留言已经读完，之后会再想想。/);
    assert.match(full.textContent, /关联处理结果 1 条/);
    assert.equal(full.querySelector('p').textContent, message.payload.text);
    assert.equal(full.querySelector('time').textContent, '2026-11-23 09:45 (北京时间)');
    assert.match(empty.textContent, /处理状态：等待回应/);
    assert.doesNotMatch(empty.textContent, /提交时间|处理结果|关联处理结果|回复已生成|已经回答/);
    assert.equal(empty.querySelector('time'), null);
    assertClean(section);
    await download(full, message);
    await download(empty, emptyMessage);
    await view.click(button(full, '跟进这条留言'));
    const follow = view.host.querySelector('[data-testid="request-status"]');
    assert.ok(follow);
    assert.deepEqual(reads, [[fid, message.id]]);
    assert.equal(follow.querySelector('a[href="#/diag/request/req_private"]').textContent, '查看原请求与关联任务');
    assert.equal(follow.querySelector('a[href="#/diag/task/res_task_private"]').textContent, '查看任务 · 处理中');
    assert.ok(button(follow, '刷新原请求'));
    assert.match(follow.textContent, /留言处理已结束/);
    assertClean(follow);
    await download(follow, current);
    assert.equal(filenames.length, 3);
  } finally {
    URL.createObjectURL = create;
    URL.revokeObjectURL = revoke;
    anchor.click = click;
    await view.dispose();
  }
});

const measured = value => ({value, available: true, complete: true});
const overview = {
  ...metadata,
  as_of: until, timezone: 'Asia/Shanghai', fluctlight_id: fid, actor_id: owner,
  profile_id: 'place_profile_private', first_communication_at: at,
  metrics: {
    ...metadata.metrics, companionship_days: measured(2), completed_turns: measured(0),
    diaries: {value: 58319, available: true, complete: false},
    memories: measured(3),
    relationship: measured({summary: '我们常一起散步。', role: {label: '闺蜜'}, ...metadata}),
  },
  footprints: [
    {id: 'res_message_private', conversation_id: 'res_conversation_private', kind: 'remote_communication', at, title: "一起聊了 <风> & 雨。", ...metadata},
    {id: 'req_promise_private', kind: 'promise', at: null, title: '下次去散步。', status: 'deferred', ...metadata},
    {id: 'res_diary_private', kind: 'diary', local_date: '2026-11-22', title: '散步日记。', ...metadata},
    {id: 'res_memory_private', kind: 'memory', at: null, title: 'world_item_private', ...metadata},
  ],
  footprints_complete: false,
  semantics: {
    communication: 'actual_participation_intervals',
    turns: 'distinct_conversation_and_turn_with_persisted_owner_input_and_nonempty_character_reply',
    memories: 'active_owner_actor_refs_and_current_source_liveness',
    diaries: 'ready_day_with_nonempty_active_attempt_body', copresent: false,
  },
};
const ctx = {charId: fid, query: {}, raw: '/c/' + fid + '/life/shared', navigate() {}};
const recordsHref = '#/c/' + fid + '/life/shared?view=records';

async function mountStory(t, record, name = '林青') {
  const host = document.createElement('div');
  document.body.append(host);
  const previous = store.get();
  store.replaceSnapshot({...previous, session: {actorId: owner}, characters: {[fid]: {name}}});
  configureDomainService('diary', {overview: async () => record});
  const dispose = renderRealShared(host, ctx);
  const unsubscribe = subscribeOverlays(() => {});
  t.after(() => {
    dispose();
    host.remove();
    for (const {id} of overlaySnapshot()) closeOverlay(id);
    unsubscribe();
    store.replaceSnapshot(previous);
  });
  await new Promise(resolve => setImmediate(resolve));
  return host;
}

function basis(host) {
  host.querySelector('[data-basis]').click();
  const body = overlaySnapshot().at(-1).node;
  assert.ok(body);
  assertClean(body);
  return body;
}

function storyDownload(host) {
  const link = host.querySelector('a[download]');
  assert.ok(link);
  assert.equal(link.textContent, '原始共同经历记录（诊断）');
  assert.equal(link.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = link.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  return JSON.parse(decodeURIComponent(href.slice(prefix.length)));
}

test('共同经历保留来源入口与中文统计口径，诊断下载保留完整快照', async t => {
  const host = await mountStory(t, overview);
  host.querySelectorAll('details').forEach(details => { details.open = true; });
  assertClean(host);
  assert.match(host.textContent, /我们常一起散步。/);
  assert.match(host.textContent, /完成回复轮次0 轮/);
  assert.doesNotMatch(host.textContent, /她的日记0|她的日记—|关于你的记忆—/);
  assert.equal(host.querySelector('[data-story-metric="diaries"]'), null);
  assert.match(host.textContent, /她的主观日记 · 2026-11-22/);
  assert.equal(host.querySelector('.story-source-excerpt').textContent, overview.footprints[0].title);
  const href = host.querySelector('a[href*="/chat?"]').getAttribute('href');
  assert.equal(new URLSearchParams(href.split('?')[1]).get('message'), overview.footprints[0].id);
  assert.equal(host.querySelector('[data-story-promise]').textContent, '查看约定');
  assert.equal(host.querySelector('a[href="' + recordsHref + '"]').textContent, '查看全部交流');
  const drawer = basis(host);
  assert.match(drawer.textContent, /交流统计范围：2026-11-23 09:45 \(北京时间\) 至 2026-11-24 10:15 \(北京时间\)/);
  assert.match(drawer.textContent, /完成回复轮次：0 轮/);
  assert.match(drawer.textContent, /本页回顾记录 4 条，其中远程交流 1 条/);
  assert.match(drawer.textContent, /回顾范围：你和林青/);
  assert.match(drawer.textContent, /交流只纳入双方实际参与时段内的消息/);
  assert.match(drawer.textContent, /同一交流中的同一轮只统计一次/);
  assert.match(drawer.textContent, /本页只列出部分来源/);
  assert.match(drawer.textContent, /远程交流不表示双方曾在同一地点/);
  assert.equal(drawer.querySelector('a:not([download])').getAttribute('href'), recordsHref);
  assert.deepEqual(storyDownload(drawer), overview);
});

test('共同经历省略空时间、缺失统计和仅含内部标识的关系名称', async t => {
  const record = {
    ...overview, as_of: null, first_communication_at: null, footprints: [], semantics: {},
    metrics: {relationship: measured({role: {label: fid}}), completed_turns: measured(null)},
  };
  const host = await mountStory(t, record, fid);
  assertClean(host);
  assert.deepEqual([...host.querySelectorAll('.story-metric')].map(node => node.textContent), ['现在的我们']);
  assert.doesNotMatch(host.textContent, /截至|正在读取/);
  const drawer = basis(host);
  assert.doesNotMatch(drawer.textContent, /截至|统计范围|回顾范围|完成回复轮次/);
  assert.match(drawer.textContent, /本页回顾记录 0 条，其中远程交流 0 条/);
  assert.deepEqual(storyDownload(drawer), record);
});
