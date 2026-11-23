import test from 'node:test';
import assert from 'node:assert/strict';
import {renderNpc, renderRelEdit} from '../src/v3-domain/world-details.js';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get, replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

test('关系修正与 NPC 历史显示无说明文案，保留有效时间和完整诊断', async t => {
  const dom = installDOM();
  const previous = get();
  const fid = 'fluctlight_nodesc_fixture';
  const npcId = 'npc_nodesc_fixture';
  const actorId = 'actor_nodesc_fixture';
  const relation = {id: 'rel_nodesc_current', summary: '常在书店碰面', revision: 7};
  const history = [
    {id: 'rel_nodesc_missing', revision: 4, source_ref: 'nodesc_fixture_source',
      idempotency_key: 'nodesc_fixture_key', metrics: {score: 62719}},
    {summary: '  ', created_at: '2026-11-22T01:30:00Z'},
    {summary: null, reason: '补充结识经过', created_at: 'bad-time'},
    {summary: '在书店初识', created_at: 'invalid-timestamp'},
  ];
  const relationshipContext = {relationships: [relation], relationship_history: history};
  replaceSnapshot({
    ...previous,
    characters: {[fid]: {name: '小雨', timezone: 'Asia/Shanghai'}},
    relations: {[relation.id]: {charId: fid, targetActorId: actorId, ...relation}},
  });
  configureDomainService('world', {refresh: async charId => assert.equal(charId, fid)});
  configureDomainService('npc', {
    npc: async () => ({id: npcId, name: '阿晴', relationship_context: relationshipContext}),
    npcRecords: async () => ({items: [], next_cursor: ''}),
    readRelation: async () => ({relation, history}),
  });

  const edit = document.body.appendChild(document.createElement('main'));
  const npc = document.body.appendChild(document.createElement('main'));
  const disposeEdit = renderRelEdit(edit, {charId: fid, params: {relId: relation.id}});
  const disposeNpc = renderNpc(npc, {charId: fid, params: {npcId}, query: {}});
  t.after(() => {
    disposeEdit();
    disposeNpc();
    replaceSnapshot(previous);
    dom.restore();
  });
  await new Promise(resolve => setImmediate(resolve));

  const noDescription = '这次修订没有说明';
  const time = '2026-11-22 09:30 (北京时间)';
  const restores = [...edit.querySelectorAll('[data-restore-relation]')];
  assert.deepEqual(restores.map(button => button.previousElementSibling.textContent), [
    noDescription, `${noDescription} · ${time}`, '补充结识经过', '在书店初识',
  ]);
  assert.equal(edit.querySelector('[name=summary]').value, '常在书店碰面');
  const rows = [...npc.querySelectorAll('[data-test="rel-revs"] [data-domain-ui="item"]')];
  assert.deepEqual(rows.map(row => row.querySelector('span').textContent), [
    noDescription, noDescription, '补充结识经过', '在书店初识',
  ]);
  assert.equal(rows[1].querySelector('time').textContent, time);
  assert.deepEqual(rows.map(row => row.querySelector('time')?.dateTime), [
    undefined, '2026-11-22T01:30:00Z', undefined, undefined,
  ]);
  assert.equal(rows[0].textContent, noDescription);
  assert.match(npc.textContent, /常在书店碰面/);

  const hidden = /\bv\d+\b|版本|尚未记录|时间未知|\b(?:id|revision|source_ref|idempotency_key|metrics)\b|rel_nodesc_(?:missing|current)|nodesc_fixture_(?:source|key)|62719|bad-time|invalid-timestamp/i;
  const prefix = 'data:application/json;charset=utf-8,';
  for (const [host, record] of [[edit, {relation, history}], [npc, relationshipContext]]) {
    assert.doesNotMatch(host.textContent, hidden);
    const downloads = host.querySelectorAll('a[download]');
    assert.equal(downloads.length, 1);
    const [download] = downloads;
    assert.equal(download.textContent, '原始关系记录（诊断）');
    assert.equal(download.download, 'fluctlight-record.json');
    const href = download.getAttribute('href');
    assert.ok(href.startsWith(prefix));
    assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), record);
  }
});
