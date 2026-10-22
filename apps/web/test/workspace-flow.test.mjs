import test from 'node:test';
import assert from 'node:assert/strict';
import {workspaceForPath,globalNavigation,sidebarSectionPath} from '../src/v3/workspace-navigation.mjs';
import {readProduct} from '../src/v3/read-product.mjs';
import {creationFeedback,initializationWarnings} from '../src/v3/creation-feedback.mjs';
import {selectCreationWorld} from '../src/v3/creation-draft.mjs';
import {directoryGroupProjection} from '../src/v3/contract-map.mjs';

test('management tools stay in management navigation and direct URLs choose their workspace',()=>{
 for(const path of ['/settings/models','/diag/model-runs','/data','/studio','/setup','/library/create'])assert.equal(workspaceForPath(path),'管理');
 assert.ok(!globalNavigation('交流').some(([,href])=>href.startsWith('#/settings')||href.startsWith('#/diag')));
 assert.ok(globalNavigation('管理').some(([,href])=>href==='#/data'));
 assert.equal(workspaceForPath('/c/character/chat'),'交流');assert.equal(workspaceForPath('/inbox'),null);
});
test('editor and support routes retain their visible sidebar section without changing routes',()=>{
 assert.equal(sidebarSectionPath('/c/role/worlds/world/edit'),'/c/role/world/world/edit');
 assert.equal(sidebarSectionPath('/c/role/life/goal/new'),'/c/role/life/goals/new');
 assert.equal(sidebarSectionPath('/c/role/life/planning'),'/c/role/life/goals');
 assert.equal(sidebarSectionPath('/c/role/life'),'/c/role/life/now');
 for(const path of ['/diag','/diag/task/original','/data/backup/original','/setup'])assert.ok(sidebarSectionPath(path).startsWith('/settings/'));
 for(const path of ['/c/role/people/npc/original','/c/role/media/identity','/c/role/life/goals','/studio/edit/original','/library/create','/c/worlds/profile'])assert.equal(sidebarSectionPath(path),path);
});
test('first paint uses session and directory without waiting for optional modules',async()=>{
 const calls=[],client=new Proxy({session:async()=>({authenticated:true,actorId:'owner'}),listFluctlights:async()=>[{id:'character'}]},{get:(target,key)=>target[key]||(()=>{calls.push(key);return new Promise(()=>{});})});
 const bundle=await readProduct(client,{initialOnly:true});assert.equal(bundle.fluctlights[0].id,'character');assert.deepEqual(calls,[]);assert.equal(bundle.surfaces['global:settings'].status,'loading');
});
test('initialization failure exposes the exact field and correlation rather than only a generic message',()=>{
 const detail=creationFeedback({code:'initialization_persona_invalid',message:'Fluctlight analysis was rejected',details:{correlation_id:'initialization-analysis:actual',validation_error:{type:'value_invalid',path:'core_persona.identity.timezone'},retryable:true}});
 assert.equal(detail.correlationId,'initialization-analysis:actual');assert.equal(detail.path,'core_persona.identity.timezone');assert.equal(detail.retryable,true);assert.notEqual(detail.message,'Fluctlight analysis was rejected');
});
test('malformed optional review warnings cannot stop the creation preview',()=>{
 const source=[null,{message:'已保留原值'},'legacy'];const warnings=initializationWarnings(source);
 assert.equal(warnings.length,3);assert.ok(warnings.every(w=>typeof w.path==='string'&&typeof w.message==='string'));
 assert.equal(source[0],null);assert.equal(initializationWarnings(null).length,0);assert.equal(initializationWarnings({legacy:true}).length,1);
});
test('changing the selected world version invalidates analysis and preserves inputs and original pending activation',()=>{
 const draft={step:3,mode:'description',bio:'完整原描述',analysis:{analysis_id:'old-source'},analysisText:'old',requestId:'original',worldResourceId:'world',worldResourceVersion:'v1'};
 assert.equal(selectCreationWorld(draft,{resourceId:'world',version:'v1'}),false);assert.equal(draft.requestId,'original');
 assert.equal(selectCreationWorld(draft,{resourceId:'world',version:'v2'}),true);assert.equal(draft.step,2);assert.equal(draft.bio,'完整原描述');assert.equal(draft.analysis,null);assert.equal(draft.analysisText,'');assert.notEqual(draft.requestId,'original');
 const before=structuredClone(draft);assert.equal(selectCreationWorld(draft,{resourceId:'other',version:'v3',pending:true}),false);assert.deepEqual(draft,before);
 draft.activatedId='accepted';assert.equal(selectCreationWorld(draft,{resourceId:'other',version:'v3'}),false);assert.equal(draft.worldResourceId,'world');
 const late={activatedId:'accepted',requestId:'accepted-command'};assert.equal(selectCreationWorld(late,{resourceId:'world',version:'v1'}),true);assert.equal(late.requestId,'accepted-command');assert.equal(late.activatedId,'accepted');
});
test('confirmed group membership updates retain character data and reset removed membership',()=>{
 const previous={one:{id:'one',bio:'原资料',groupId:'old'},two:{id:'two',groupId:'old'}};
 const projected=directoryGroupProjection(previous,[{id:'new',name:'新组',actor_ids:['one']}]);
 assert.equal(projected.characters.one.groupId,'new');assert.equal(projected.characters.one.bio,'原资料');assert.equal(projected.characters.two.groupId,'ungrouped');assert.equal(projected.groups.new.name,'新组');assert.equal(previous.one.groupId,'old');
});
