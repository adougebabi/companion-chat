import test from 'node:test';
import assert from 'node:assert/strict';
import {installDOM} from './helpers/react-dom.mjs';
import {renderRealDiaryEntry} from '../src/v3-domain/diary.js';
import {configureDomainService} from '../src/v3/proto/services.js';
import {RawRecordDownload} from '../src/v3/proto/components/records.js';
import {closeOverlay, overlaySnapshot, subscribeOverlays} from '../src/state/overlays.ts';

const env = installDOM();
test.after(() => env.restore());
const tick = () => new Promise(resolve => setImmediate(resolve));
const diary = {
  id: 'private-diary', date: '2026-11-23', status: 'ready', revision: 1, source: 'agent',
  body: '我把今天的事写下来了。',
  refs: ['message:private-message', 'event:private-event', 'message:private-message'],
  source_snapshot: {sources: [
    {ref: 'message:private-message', kind: 'remote_communication', conversation_id: 'private-conversation', text: '一起看了 <雨> & 风。'},
    {ref: 'event:private-event', kind: 'confirmed_life_event', activity: '散步', scene: '街边', location: '河堤', revision: 429},
    {ref: 'event:private-unquoted', kind: 'confirmed_life_event', activity: '未引用的素材'},
  ]},
  persona_provenance: {id: 'private-persona', revision: 341, source_ref: 'private-source'},
  model_provenance: {correlation_id: 'private-correlation', metrics: {tokens: 365}, idempotency_key: 'private-key'},
  annotation: {favorite: true, note: '留住这段话。', revision: 718},
  attempts: [
    {id: 'private-attempt', attempt: 1, status: 'pending', created_at: '2026-11-23T01:45:00Z', task_id: 'private/task'},
    {attempt: 2, status: 'no_content', plan: {reason: 'no_source_material'}},
    {attempt: 3, status: 'failed', error_code: 'private_error_code'},
    {attempt: 4, status: 'ready', plan: {reason: '今天的交流值得记下。'}},
    {attempt: 5, status: 'private_status'},
  ],
  has_more_attempts: true,
};

async function mount(t, entry) {
  const ctx = {charId: 'role', params: {entryId: entry.id}, query: {}};
  const host = document.createElement('div');
  document.body.append(host);
  let current = entry;
  configureDomainService('diary', {read: async () => current});
  const dispose = renderRealDiaryEntry(host, ctx);
  t.after(() => { dispose(); host.remove(); });
  await tick();
  assert.ok(host.querySelector('.reading-paper'), host.textContent);
  return {host, async reload(next) {
    current = next;
    await host.querySelector('[data-reload]').onclick();
  }};
}

function assertClean(host) {
  assert.doesNotMatch(host.textContent, /正文来源|独立人物表达|\bv\d+\b|版本|尚未记录|模型来源|关联标识|private[-_]|\b(?:id|revision|source[ _]ref|idempotency(?:[ _]key)?|metrics)\b|429|341|365|718|719/);
  assert.equal(host.querySelector('.record-facts, pre, code, img'), null);
}

function download(host, label) {
  const anchor = [...host.querySelectorAll('a[download]')].find(link => link.textContent === label);
  assert.ok(anchor, label);
  assert.equal(anchor.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = anchor.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  return JSON.parse(decodeURIComponent(href.slice(prefix.length)));
}

test('独立日记详情保留正文、中文来源摘要与原链接，并完整下载诊断记录', async t => {
  const {host} = await mount(t, diary);
  const [sources, attempts] = host.querySelectorAll('details');
  sources.open = attempts.open = true;
  assertClean(host);
  assert.equal(host.querySelector('.reading-paper .memory-content').textContent, diary.body);
  assert.match(host.querySelector('.reading-paper').textContent, /按日记原日期归档/);
  assert.equal(host.querySelector('[data-saved-note]').textContent, diary.annotation.note);
  assert.equal(host.querySelector('[data-saved-favorite]').textContent, '已收藏');
  assert.match(sources.textContent, /可用素材 3 条（远程交流 1 条）/);
  assert.match(sources.textContent, /正文引用来源 2 条/);
  assert.deepEqual([...sources.querySelectorAll('.memory-content')].map(node => node.textContent), ['一起看了 <雨> & 风。', '散步 · 街边 · 河堤']);
  assert.doesNotMatch(sources.textContent, /未引用的素材|查看原来源快照/);
  assert.deepEqual([...sources.querySelectorAll('a:not([download])')].map(link => [link.textContent, link.getAttribute('href')]), [
    ['原交流消息', '#/c/role/chat?conversation=private-conversation&message=private-message'],
    ['已确认生活事件', '#/c/role/world/event/private-event'],
  ]);
  assert.equal(sources.querySelector('details'), null);
  assert.deepEqual(download(sources, '原始日记记录（诊断）'), diary);
  const rows = [...attempts.querySelectorAll('[data-domain-ui="item"]')];
  assert.deepEqual(rows.map(row => row.querySelector('p').textContent), [
    '第 1 次整理 · 等待整理 · 2026-11-23 09:45 (北京时间)',
    '第 2 次整理 · 无正文',
    '第 3 次整理 · 整理未完成',
    '第 4 次整理 · 已经写下',
    '第 5 次整理 · 状态待确认',
  ]);
  assert.equal(rows[0].querySelector('time').dateTime, diary.attempts[0].created_at);
  assert.equal(rows[1].querySelector('time'), null);
  assert.match(rows[1].textContent, /没有可用于整理的素材。/);
  assert.match(rows[2].textContent, /这次整理遇到问题，请查看原任务诊断。/);
  assert.match(rows[3].textContent, /今天的交流值得记下。/);
  assert.doesNotMatch(attempts.textContent, /no_source_material/);
  assert.equal(rows[0].querySelector('a').textContent, '查看原任务');
  assert.equal(rows[0].querySelector('a').getAttribute('href'), '#/diag/task/private%2Ftask');
  assert.match(attempts.textContent, /更早执行记录仍保留在任务诊断中/);
});

test('独立日记详情省略空素材、引用和注记占位，保留原始日记下载', async t => {
  const entry = {...diary, id: 'empty-diary', body: null, refs: [], source_snapshot: null, persona_provenance: {}, model_provenance: null, annotation: {favorite: false, note: '', revision: 0}, attempts: [{status: 'running'}, {status: 'inactive'}], has_more_attempts: false};
  const {host} = await mount(t, entry);
  assertClean(host);
  assert.doesNotMatch(host.textContent, /可用素材|正文引用来源|还没有已|未记录|时间未知|更早执行记录/);
  assert.equal(host.querySelector('[data-saved-note]').hidden, true);
  assert.deepEqual([...host.querySelectorAll('details [data-domain-ui="item"] p')].map(node => node.textContent), [
    '第 1 次整理 · 正在回顾', '第 2 次整理 · 尚未运行',
  ]);
  assert.deepEqual(download(host, '原始日记记录（诊断）'), entry);
  for (const empty of [null, undefined, '', {}, []]) assert.equal(RawRecordDownload(empty, '原始记录（诊断）'), '');
});

test('独立日记注记冲突显示最新注记和收藏状态，保留草稿与原始记录下载', async t => {
  const {host, reload} = await mount(t, {...diary, id: 'conflict-diary'});
  const unsubscribe = subscribeOverlays(() => {});
  t.after(unsubscribe);
  const note = host.querySelector('[data-note]');
  const textarea = note.elements.namedItem('note');
  const favorite = note.elements.namedItem('favorite');
  // jsdom exposes named controls via elements, while browsers also expose them on the form.
  Object.assign(note, {note: textarea, favorite});
  textarea.value = '我的本地草稿';
  note.oninput();
  const entry = {...diary, id: 'conflict-diary', annotation: {note: '另一处刚保存的注记。', favorite: false, revision: 719}};
  await reload(entry);
  host.querySelector('[data-open-note]').click();
  const drawer = overlaySnapshot().at(-1);
  try {
    const form = drawer.node.querySelector('[data-note]');
    assertClean(form);
    assert.match(form.querySelector('[role="alert"]').textContent, /已保存的注记或收藏状态已更新。你的草稿已保留。/);
    assert.match(form.textContent, /最新已保存注记\s+未收藏\s+另一处刚保存的注记。/);
    assert.equal(form.elements.namedItem('note').value, '我的本地草稿');
    assert.equal(form.elements.namedItem('favorite').checked, true);
    assert.equal(form.querySelector('button[type=submit]').disabled, true);
    assert.deepEqual(download(form, '原始注记记录（诊断）'), entry.annotation);
    form.querySelector('[data-keep]').click();
    assert.equal(form.querySelector('[data-note-error]').textContent, '注记草稿已保留。');
    form.querySelector('[data-refresh]').click();
    const confirmation = overlaySnapshot().at(-1);
    assert.equal(confirmation.kind, 'confirm');
    closeOverlay(confirmation.id, true);
    await tick();
    assert.equal(host.querySelector('[data-note] textarea').value, '我的本地草稿');
    assert.equal(host.querySelector('[data-note] button[type=submit]').disabled, false);
  } finally { closeOverlay(drawer.id); }
});
