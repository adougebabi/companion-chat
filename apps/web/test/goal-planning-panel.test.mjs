import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import ts from 'typescript';
import {actorUserBackgroundFields,backgroundFromDraft,backgroundDraft} from '../src/lib/actor-user-background.ts';
import {validateFields} from '../src/lib/form-validation.ts';
import {userError} from '../src/lib/user-feedback.ts';
async function harness(file,client,extra,options={}){
 const source=await readFile(new URL('../src/components/instances/'+file,import.meta.url),'utf8');
 const script=source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/^import .*?;\s*$/gm,'');
 const compiled=ts.transpile(script,{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.None});
 class FakeClient{constructor(){return client}}
 class ErrorType extends Error{constructor(status){super('conflict');this.status=status}}
 return new Function('ref','watch','computed','defineProps','BrowserClient','BrowserApiError','apiOrigin','randomId','actorUserBackgroundFields','backgroundFromDraft','backgroundDraft','validateFields','userError','focusField','defineEmits','onBeforeUnmount',compiled+';return '+extra)(value=>({value}),()=>{},fn=>({get value(){return fn()}}),()=>({fluctlightId:'instance',actors:[{id:'A',label:'A'}],...options.props}),FakeClient,ErrorType,'',options.randomId ?? (()=> 'stable-key'),actorUserBackgroundFields,backgroundFromDraft,backgroundDraft,validateFields,userError,async()=>{},()=>()=>{},()=>{});
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

test('Owner background keeps the current-facts CAS and tri-state relationship fields',async()=>{
 const context={actor_id:'A',context_version:'context-v1',background:{meeting_confirmed:false,interests:'绘画'},facts:[],history:[],relationships:[],relationship_history:[]};
 let payload;const client={actorContext:async()=>context,detail:async()=>({actor_user:{background:context.background},current_facts_revision:'facts-v9'}),updateActorUserBackground:async(id,body)=>{payload=body;return {};},updateActorContext:async()=>assert.fail('Owner adapter must not substitute the context version')};
 const h=await harness('ActorContextEditor.vue',client,'{load,save,target,draft,reason,operation}',{props:{ownerActorId:'A'}});
 h.target.value='A';await h.load();h.reason.value='确认背景';h.operation.value='correct';h.draft.value.preferences='安静';await h.save();
 assert.equal(payload.expectedCurrentFactsRevision,'facts-v9');assert.equal(payload.expectedContextVersion,undefined);
 assert.equal(payload.background.meeting_confirmed,false);assert.equal(payload.background.interests,'绘画');assert.equal(payload.background.preferences,'安静');assert.equal(payload.operation,'correct');
});
test('Actor drafts use a new idempotency identity when the payload changes',async()=>{
 const context={actor_id:'A',context_version:'v1',background:{location:'同城'},facts:[],history:[],relationships:[],relationship_history:[]};
 const calls=[];let next=0;
 const h=await harness('ActorContextEditor.vue',{actorContext:async()=>context,updateActorContext:async(_,__,body)=>{calls.push(body);throw new Error('failure');}},'{load,save,target,draft,reason}',{randomId:()=>`request-${++next}`});
 h.target.value='A';await h.load();h.reason.value='资料变化';h.draft.value.location='国外';await h.save();await h.save();h.draft.value.location='同城';await h.save();
 assert.equal(calls[0].idempotencyKey,calls[1].idempotencyKey);assert.notEqual(calls[1].idempotencyKey,calls[2].idempotencyKey);
});
test('clearing an Actor target fences a late context response',async()=>{
 let release;const gate=new Promise(resolve=>release=resolve);
 const h=await harness('ActorContextEditor.vue',{actorContext:async()=>gate},'{load,target,context,busy}');
 h.target.value='A';const pending=h.load();h.target.value='';await h.load();release({actor_id:'A',context_version:'old',background:{},facts:[],history:[],relationships:[],relationship_history:[]});await pending;
 assert.equal(h.context.value,null);assert.equal(h.busy.value,false);
});

test('Owner revision reasons obey the 500-character boundary while Actor reasons obey the byte budget',async()=>{
 const context={actor_id:'A',context_version:'v1',background:{},facts:[],history:[],relationships:[],relationship_history:[]};
 let writes=0;const client={actorContext:async()=>context,detail:async()=>({current_facts_revision:'facts-v1'}),updateActorUserBackground:async()=>writes++,updateActorContext:async()=>writes++};
 const owner=await harness('ActorContextEditor.vue',client,'{load,save,target,reason,fieldErrors}',{props:{ownerActorId:'A'}});
 owner.target.value='A';await owner.load();owner.reason.value='明'.repeat(501);await owner.save();assert.equal(writes,0);assert.match(owner.fieldErrors.value['actor-change-reason'],/500/);
 owner.reason.value='明'.repeat(500);await owner.save();assert.equal(writes,1);
 const actor=await harness('ActorContextEditor.vue',client,'{load,save,target,reason,fieldErrors}');actor.target.value='A';await actor.load();actor.reason.value='明'.repeat(334);await actor.save();assert.equal(writes,1);assert.match(actor.fieldErrors.value['actor-change-reason'],/过长/);
});

test('failed initial background read can be retried without changing the target',async()=>{
 let reads=0;const context={actor_id:'A',context_version:'v1',background:{name:'sample'},facts:[],history:[],relationships:[],relationship_history:[]};
 const h=await harness('ActorContextEditor.vue',{actorContext:async()=>{if(++reads===1)throw new Error('unavailable');return context;}},'{load,target,context,error,busy}');
 h.target.value='A';await h.load();assert.equal(h.context.value,null);assert.match(h.error.value,/重试/);assert.equal(h.busy.value,false);await h.load();assert.equal(h.context.value.actor_id,'A');assert.equal(h.error.value,'');
});
