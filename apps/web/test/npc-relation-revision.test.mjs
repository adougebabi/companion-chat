import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {closeOverlay, overlaySnapshot, subscribeOverlays} from '../src/state/overlays.ts';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get, replaceSnapshot} from '../src/v3/proto/store.js';
import {renderNpc} from '../src/v3-domain/world-details.js';
import {installDOM} from './helpers/react-dom.mjs';

const fid = 'fluctlight_npc_fixture';
const npcId = 'npc_relation_fixture';
const actorId = 'actor_npc_fixture';
const tick = () => new Promise(resolve => setImmediate(resolve));
const hidden = /\bv\d+\b|版本|尚未记录|未记录|尚无关系记录|尚无关系修订|时间未知|\b(?:id|revision|source_ref|idempotency_key|metrics)\b|rel_npc_(?:current|other|first|later|sparse)|profile_npc_fixture|npc_relation_source|npc_retry_key|98327/i;

function npcFixture(t, relationshipContext, commands = {}) {
  const dom = installDOM();
  const previous = get();
  const host = document.body.appendChild(document.createElement('main'));
  const current = relationshipContext.relationships?.[0];
  replaceSnapshot({
    ...previous,
    session: {actorId: 'owner_fixture'},
    characters: {[fid]: {name: '小雨', timezone: 'Asia/Shanghai'}},
    relations: current ? {[current.id]: {id: current.id, charId: fid, targetActorId: actorId, revision: 2}} : {},
  });
  configureDomainService('world', {refresh: async charId => assert.equal(charId, fid)});
  configureDomainService('npc', {
    npc: async (...args) => {
      assert.deepEqual(args, [fid, npcId]);
      return {id: npcId, name: '阿晴', role: '店员', relationship_context: relationshipContext};
    },
    npcRecords: async () => ({items: [], next_cursor: ''}),
    ...commands,
  });
  const dispose = renderNpc(host, {charId: fid, params: {npcId}, query: {}});
  t.after(() => {
    dispose();
    replaceSnapshot(previous);
    dom.restore();
  });
  return host;
}

test('NPC 关系正文仅展示描述和北京时间，诊断下载完整，保存与恢复提交正确修订号', async t => {
  const relation = {
    id: 'rel_npc_current',
    revision: 7,
    profile_id: 'profile_npc_fixture',
    summary: '常在书店碰面',
    role: {label: '熟人'},
    metrics: {intimacy: 0.4, fixture_score: 98327},
    source_ref: 'npc_relation_source',
    idempotency_key: 'npc_retry_key',
  };
  const relationships = [relation, {id: 'rel_npc_other', revision: 9, stage: '初次认识'}];
  const history = [
    {id: 'rel_npc_first', revision: 0, summary: '在书店初识 <雨天>', created_at: '2026-11-22T01:30:00Z'},
    {id: 'rel_npc_later', revision: '4', reason: '补充结识经过', created_at: 'bad-time'},
    {id: 'rel_npc_sparse', revision: 6},
  ];
  const relationshipContext = {relationships, relationship_history: history};
  const saves = [];
  const restores = [];
  const host = npcFixture(t, relationshipContext, {
    readRelation: async (...args) => {
      assert.deepEqual(args, [fid, actorId, relation.id]);
      return {relation, history};
    },
    saveRelation: async (...args) => saves.push(args),
    restoreRelation: async (...args) => restores.push(args),
  });
  await tick();

  const links = [...host.querySelectorAll('a')].filter(link => link.textContent === '修正关系…');
  assert.deepEqual(links.map(link => link.previousElementSibling.textContent), ['常在书店碰面', '初次认识']);
  const rows = host.querySelectorAll('[data-test="rel-revs"] [data-domain-ui="item"]');
  assert.equal(rows.length, 3);
  assert.equal(rows[0].querySelector('span').textContent, '在书店初识 <雨天>');
  assert.equal(rows[0].querySelector('time').textContent, '2026-11-22 09:30 (北京时间)');
  assert.equal(rows[1].textContent, '补充结识经过');
  assert.equal(rows[2].textContent, '这次修订没有说明');
  assert.doesNotMatch(host.textContent, hidden);

  const downloads = host.querySelectorAll('a[download]');
  assert.equal(downloads.length, 1);
  const [download] = downloads;
  assert.equal(download.textContent, '原始关系记录（诊断）');
  assert.equal(download.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = download.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), relationshipContext);

  const route = approvedRoute(links[0].getAttribute('href'));
  assert.equal(route.feature.render, 'renderRelEdit');
  const renderEdit = await route.feature.load();
  const edit = document.body.appendChild(document.createElement('main'));
  const unsubscribe = subscribeOverlays(() => {});
  const destinations = [];
  let refreshes = 0;
  const dispose = renderEdit(edit, {
    charId: route.fid,
    params: route.feature.params,
    navigate: path => destinations.push(path),
    refresh: () => refreshes++,
  });
  t.after(() => {
    dispose();
    unsubscribe();
  });
  await tick();
  assert.doesNotMatch(edit.textContent, hidden);
  const form = edit.querySelector('[data-rf]');
  assert.ok(form, edit.textContent);
  form.elements.evidenceRefs.value = ' note:npc, , note:bookshop ';
  const restoring = edit.querySelectorAll('[data-restore-relation]')[1].onclick();
  await Promise.resolve();
  const [confirmation] = overlaySnapshot();
  assert.equal(confirmation?.title, '恢复关系修订字段');
  closeOverlay(confirmation.id, true);
  await restoring;
  assert.deepEqual(restores, [[fid, actorId, {
    targetRevision: 4,
    expectedRevision: 7,
    profileId: relation.profile_id,
    evidenceRefs: ['note:npc', 'note:bookshop'],
  }]]);
  assert.equal(refreshes, 1);

  form.elements.summary.value = '常一起逛书店';
  form.elements.reason.value = '补充近期相处情况';
  // jsdom exposes named controls through form.elements.
  for (const name of ['summary', 'reason', 'semantic', 'intimacy']) {
    Object.defineProperty(form, name, {value: form.elements.namedItem(name)});
  }
  await form.onsubmit({target: form, preventDefault() {}});
  assert.deepEqual(saves, [[fid, actorId, {
    expectedRevision: 7,
    profileId: relation.profile_id,
    summary: '常一起逛书店',
    role: relation.role,
    metrics: relation.metrics,
    evidenceRefs: ['note:npc', 'note:bookshop'],
    reason: '补充近期相处情况',
  }]]);
  assert.deepEqual(destinations, [`#/c/${fid}/people`]);
});

test('NPC 没有关系记录时省略空关系区块和诊断入口', async t => {
  const host = npcFixture(t, {relationships: [], relationship_history: []});
  await tick();
  assert.match(host.textContent, /阿晴.*位置与来源/s);
  assert.doesNotMatch(host.textContent, /与 小雨 的关系|关系变化|原始关系记录/);
  assert.doesNotMatch(host.textContent, hidden);
});
