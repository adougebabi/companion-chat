import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import ts from 'typescript';
async function harness(file,client,extra){
 const source=await readFile(new URL('../src/components/instances/'+file,import.meta.url),'utf8');
 const script=source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/^import .*?;\s*$/gm,'');
 const compiled=ts.transpile(script,{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.None});
 class FakeClient{constructor(){return client}}
 class ErrorType extends Error{constructor(status){super('conflict');this.status=status}}
 return new Function('ref','watch','computed','defineProps','BrowserClient','BrowserApiError','apiOrigin','randomId',compiled+';return '+extra)(value=>({value}),()=>{},fn=>({get value(){return fn()}}),()=>({fluctlightId:'instance',actors:[{id:'A',label:'A'}]}),FakeClient,ErrorType,'',()=> 'stable-key');
}
const snapshot={revision:1,facts_revision:'facts-1',active_count:2,max_active_goals:5,auto_planning_enabled:true,ordering_mode:'automatic',capacity_violation:false,goals:[{id:'old',status:'active',desired_outcome:'Old'},{id:'new',status:'active',desired_outcome:'New'}],dependencies:[]};
test('manual order and disabled planning are server commands carrying authoritative versions',async()=>{
 const calls=[];
 const h=await harness('GoalPlanningPanel.vue',{goalSet:async()=>snapshot,goalPlanningHistory:async()=>[],updateGoalSet:async(id,body)=>{calls.push(body);return {...snapshot,revision:2,ordering_mode:'manual'}},requestGoalPlanning:async(...args)=>calls.push(args)},'{load,apply,move,plan,state,order,error,notice}');
 await h.load();h.move(1,-1);await h.apply({order:h.order.value,orderingMode:'manual'},'Owner order');
 assert.deepEqual(calls[0].order,['new','old']);assert.equal(calls[0].expectedVersion,1);assert.equal(calls[0].expectedFactsRevision,'facts-1');assert.equal(calls[0].idempotencyKey,'stable-key');
 h.state.value.auto_planning_enabled=false;await h.plan();assert.match(h.notice.value,/仍保持关闭/);assert.equal(calls.length,2);
});
test('capacity/version failure keeps manual draft and late instance response is fenced',async()=>{
 let release;const gate=new Promise(r=>release=r);
 const h=await harness('GoalPlanningPanel.vue',{goalSet:async()=>snapshot,goalPlanningHistory:async()=>[],updateGoalSet:async()=>gate},'{load,apply,move,state,order,error,switchInstance(){epoch++;props.fluctlightId="other";state.value=null;}}');
 await h.load();h.move(1,-1);const pending=h.apply({order:h.order.value},'Owner');h.switchInstance();release({...snapshot,revision:100});await pending;assert.equal(h.state.value,null);
});
test('Actor conflict retains draft and explicit re-read only advances version',async()=>{
 const context={actor_id:'A',context_version:'v1',background:{location:'同城'},facts:[],history:[],relationships:[],relationship_history:[]};
 let version='v1';let payload;
 const h=await harness('ActorContextEditor.vue',{actorContext:async()=>({...context,context_version:version}),updateActorContext:async(id,target,body)=>{payload=body;throw new Error('stale')}},'{load,save,target,context,draft,error,reason}');
 h.target.value='A';await h.load();h.draft.value.location='国外';h.reason.value='Owner correction';await h.save();assert.equal(h.draft.value.location,'国外');assert.equal(payload.expectedContextVersion,'v1');version='v2';await h.load(true);assert.equal(h.context.value.context_version,'v2');assert.equal(h.draft.value.location,'国外');
});
