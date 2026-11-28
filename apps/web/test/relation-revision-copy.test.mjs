import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {closeOverlay, overlaySnapshot, subscribeOverlays} from '../src/state/overlays.ts';
import {configureDomainService} from '../src/v3/proto/services.js';
import {get, replaceSnapshot} from '../src/v3/proto/store.js';
import {installDOM} from './helpers/react-dom.mjs';

test('关系修订用中文描述当前记录和历史，完整诊断保留版本，保存与恢复提交正确版本', async t => {
  const env = installDOM();
  const previous = get();
  const fid = 'fluctlight_relation_fixture';
  const actorId = 'actor_relation_fixture';
  const relation = {
    id: 'rel_fixture',
    profile_id: 'profile_fixture',
    revision: 7,
    summary: '常在书店碰面',
    role: {label: '熟人'},
    metrics: {intimacy: 0.4, fixture_score: 98765},
    source_ref: 'req_relation_source',
    idempotency_key: 'req_relation_key',
  };
  const history = [
    {id: 'rel_history_older', revision: 0, summary: '初次在书店认识', created_at: '2026-11-22T01:30:00Z'},
    {id: 'rel_history_later', revision: 4, summary: '偶尔在书店聊天', created_at: null},
  ];
  const host = document.body.appendChild(document.createElement('main'));
  const unsubscribe = subscribeOverlays(() => {});
  let dispose;
  t.after(() => {
    dispose?.();
    unsubscribe();
    replaceSnapshot(previous);
    env.restore();
  });

  replaceSnapshot({
    ...previous,
    session: {actorId: 'owner_fixture'},
    relations: {[relation.id]: {id: relation.id, charId: fid, targetActorId: actorId, revision: 1}},
  });
  const saves = [];
  const restores = [];
  const destinations = [];
  let refreshes = 0;
  configureDomainService('world', {refresh: async charId => assert.equal(charId, fid)});
  configureDomainService('npc', {
    readRelation: async (...args) => {
      assert.deepEqual(args, [fid, actorId, relation.id]);
      return {relation, history};
    },
    saveRelation: async (...args) => saves.push(args),
    restoreRelation: async (...args) => restores.push(args),
  });

  const route = approvedRoute(`#/c/${fid}/people/relation/${relation.id}`);
  assert.equal(route.feature.render, 'renderRelEdit');
  const render = await route.feature.load();
  dispose = render(host, {
    charId: route.fid,
    params: route.feature.params,
    navigate: path => destinations.push(path),
    refresh: () => refreshes++,
  });
  await new Promise(resolve => setImmediate(resolve));

  const form = host.querySelector('form');
  assert.ok(form, host.textContent);
  assert.equal(form.previousElementSibling.textContent, '在当前关系记录基础上追加修订，保留旧记录与来源。');
  assert.equal(form.elements.summary.value, '常在书店碰面');
  assert.equal(form.elements.semantic.value, '熟人');
  assert.equal(form.elements.intimacy.value, '40');

  const restoreButtons = [...host.querySelectorAll('[data-restore-relation]')];
  assert.deepEqual(restoreButtons.map(button => button.previousElementSibling.textContent), [
    '初次在书店认识 · 2026-11-22 09:30 (北京时间)',
    '偶尔在书店聊天',
  ]);
  assert.ok(restoreButtons.every(button => button.getAttribute('data-restore-relation') === ''));
  assert.doesNotMatch(host.textContent, /v\d+|为基础|版本|尚?未记录|时间未知/i);
  assert.doesNotMatch(host.textContent, /\b(?:id|revision|source_ref|idempotency_key|metrics)\b|rel_fixture|rel_history_older|rel_history_later|profile_fixture|req_relation_source|req_relation_key|98765/i);

  const download = host.querySelector('a[download]');
  assert.equal(download.textContent, '原始关系记录（诊断）');
  assert.equal(download.download, 'fluctlight-record.json');
  const prefix = 'data:application/json;charset=utf-8,';
  const href = download.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))), {relation, history});

  form.elements.evidenceRefs.value = ' note:first, , note:second ';
  const restoring = restoreButtons[1].onclick();
  await Promise.resolve();
  const [confirmation] = overlaySnapshot();
  assert.equal(confirmation.title, '恢复关系修订字段');
  closeOverlay(confirmation.id, true);
  await restoring;
  assert.deepEqual(restores, [[fid, actorId, {
    targetRevision: 4,
    expectedRevision: 7,
    profileId: 'profile_fixture',
    evidenceRefs: ['note:first', 'note:second'],
  }]]);
  assert.equal(refreshes, 1);

  form.elements.summary.value = '经常在书店聊天';
  form.elements.reason.value = '补充近期相处情况';
  // jsdom exposes named controls through elements but lacks browser form named properties.
  for (const name of ['summary', 'reason', 'semantic', 'intimacy']) {
    Object.defineProperty(form, name, {value: form.elements.namedItem(name)});
  }
  await form.onsubmit({target: form, preventDefault() {}});
  assert.deepEqual(saves, [[fid, actorId, {
    expectedRevision: 7,
    profileId: 'profile_fixture',
    summary: '经常在书店聊天',
    role: {label: '熟人'},
    metrics: {intimacy: 0.4, fixture_score: 98765},
    evidenceRefs: ['note:first', 'note:second'],
    reason: '补充近期相处情况',
  }]]);
  assert.deepEqual(destinations, [`#/c/${fid}/people`]);
});

for(const phase of ['read-leave','save-leave','save-owner','restore-leave'])test('relationship async completion cannot affect a departed scope: '+phase,async t=>{
 const unsubscribe=subscribeOverlays(()=>{});
 const env=installDOM(),previous=get(),fid='relation-late-role',relation={id:'relation-late',revision:7,summary:'书店里的熟人',metrics:{intimacy:0},role:{label:'熟人'}};
 const host=document.body.appendChild(document.createElement('main'));
 let release;const pending=new Promise(resolve=>{release=resolve;});
 let saves=0,restores=0,refreshes=0;const destinations=[];
 replaceSnapshot({...previous,session:{actorId:'relation-owner'},relations:{[relation.id]:{...relation,charId:fid,targetActorId:'actor-late'}}});
 configureDomainService('world',{refresh:async()=>{}});
 configureDomainService('npc',{
  readRelation:async()=>{if(phase==='read-leave')await pending;return {relation,history:[{revision:3,summary:'初次见面'}]};},
  saveRelation:async()=>{saves++;await pending;},restoreRelation:async()=>{restores++;await pending;},
 });
 const route=approvedRoute('#/c/'+fid+'/people/relation/'+relation.id),render=await route.feature.load();
 const dispose=render(host,{charId:fid,params:route.feature.params,navigate:path=>destinations.push(path),refresh:()=>refreshes++});
 t.after(()=>{dispose();unsubscribe();replaceSnapshot(previous);env.restore();});
 await new Promise(resolve=>setImmediate(resolve));
 if(phase==='read-leave'){
  dispose();release();await new Promise(resolve=>setImmediate(resolve));assert.equal(host.querySelector('form'),null);return;
 }
 const form=host.querySelector('form');form.elements.reason.value='核对最近相处';form.elements.evidenceRefs.value='owner:note';
 let completion;
 if(phase.startsWith('save')){
  completion=form.onsubmit({target:form,preventDefault(){}});
  await form.onsubmit({target:form,preventDefault(){}});
  await Promise.resolve();assert.equal(saves,1,'busy prevents a duplicate mutation');
 }else{
  completion=host.querySelector('[data-restore-relation]').onclick();
  await Promise.resolve();const [confirmation]=overlaySnapshot();closeOverlay(confirmation.id,true);
  await new Promise(resolve=>setImmediate(resolve));assert.equal(restores,1);
 }
 if(phase==='save-owner')replaceSnapshot({...get(),session:{actorId:'another-owner'}});else dispose();
 release();await completion;
 assert.deepEqual(destinations,[]);assert.equal(refreshes,0);
});
