import test from 'node:test';
import assert from 'node:assert/strict';
import {connectionDraft,restoreConnectionDraft,connectionDraftKey} from '../src/approved-ui/configuration-draft.mjs';
import {configurationPlan,weekdays} from '../src/approved-ui/configuration-plan.mjs';
import {notificationGroups,readableSummary,diagnosticProjection} from '../src/approved-ui/presentation.mjs';
import {previousDiaryDate,diaryRequestText} from '../src/approved-ui/diary-generation.mjs';
import {creationFeedback} from '../src/v3/creation-feedback.mjs';
import {parseCreationInput} from '../src/v3/creation-json.mjs';
test('notification previews read exact owned sources and ignore foreign or stale results',async()=>{
 const {readNotificationPreviews}=await import('../src/approved-ui/notification-previews.mjs');
 const rows=[{id:'one',fluctlight_id:'A',link:'/c/A/chat?conversation=C&message=M1'},{id:'two',fluctlight_id:'A',link:'/c/A/chat?conversation=C&message=M2'},
 {id:'foreign',fluctlight_id:'A',link:'/c/B/chat?conversation=foreign&message=M3'},{id:'external',fluctlight_id:'A',link:'https://outside.example/c/A/chat?conversation=foreign&message=M4'}];
 let calls=0;
 const client={messages:async id=>{calls++;assert.equal(id,'C');return {conversation:{id:'C'},messages:[{id:'M1',conversationId:'C',authorActorId:'A',kind:'assistant',text:'已保存的正文'},{id:'M2',conversationId:'C',authorActorId:'B',kind:'assistant',text:'不能挪用的其他角色正文'}]};}};
 const result=await readNotificationPreviews(client,rows);
 assert.equal(calls,1);assert.deepEqual(result,{one:'已保存的正文'});
 let live=true;
 const late={messages:async()=>{live=false;return {conversation:{id:'C'},messages:[{id:'M1',conversationId:'C',authorActorId:'A',kind:'assistant',text:'迟到正文'}]};}};
 assert.deepEqual(await readNotificationPreviews(late,rows,()=>live),{});
 assert.deepEqual(await readNotificationPreviews({messages:async()=>{throw Error('forbidden')}},rows),{});
});

test('first-use connection save needs no unused model roles',()=>{
 const input={catalog:{versions:{},effective_values:{}},settings:[],endpoints:[{id:'new',name:'service',url:'https://model.example/v1',protocol:'openai-compatible'}],
 bindings:['generic_llm','conversation_reply','vision_understanding','embedding'].map(role=>({role,name:role,endpoint:'',model:'',timeout:''})),
 quiet:{enabled:false,start:'',end:'',days:[...weekdays],behavior:'暂停主动联系',character:'全部角色'},characters:[]};
 assert.deepEqual(configurationPlan(input).map(w=>w.operation),['endpoint']);
 input.bindings[0]={...input.bindings[0],endpoint:'new',model:'model',timeout:120};
 assert.deepEqual(configurationPlan(input).map(w=>w.operation),['endpoint','binding']);
 input.bindings[3].timeout=10;
 assert.throws(()=>configurationPlan(input),/服务地址/);
});
test('connection draft restores noncredential fields and preserves the original CAS baseline',()=>{
 const endpoint={id:'service',name:'service',url:'https://model.example/v1',protocol:'openai-compatible',secret:'PRIVATE_PASSWORD_CANARY',clearSecret:true};
 const draft=connectionDraft({endpoints:[endpoint],bindings:[{role:'generic_llm',endpoint:'service',model:'model',timeout:120}],versions:{'endpoint:service':2}});
 const raw=JSON.stringify(draft);
 assert.doesNotMatch(raw,/PRIVATE_PASSWORD_CANARY|"secret"|"clearSecret"/);
 const restored=restoreConnectionDraft(raw,{endpoints:[{id:'service',base_url:'https://saved.example',kind:'openai-compatible'}],versions:{'endpoint:service':3}});
 assert.equal(restored.endpoints[0].secret,'');assert.equal(restored.endpoints[0].clearSecret,false);
 assert.equal(restored.credentialPending,true);assert.equal(restored.stale,true);assert.equal(restored.versions['endpoint:service'],2);
 assert.equal(restored.endpoints[0].url,endpoint.url);
 assert.notEqual(connectionDraftKey('owner-A'),connectionDraftKey('owner-B'));
 for(const url of ['https://user:password@example.test/v1','https://example.test/v1?api_key=PRIVATE_QUERY_CANARY','https://example.test/#PRIVATE_FRAGMENT_CANARY']){
  const saved=JSON.stringify(connectionDraft({endpoints:[{...endpoint,url}],bindings:[],versions:{}}));
  assert.doesNotMatch(saved,/PRIVATE_QUERY_CANARY|PRIVATE_FRAGMENT_CANARY|password@example/);
 }
 assert.equal(restoreConnectionDraft('{',{}),null);
});
test('five distinct replies become one character group without losing notice identities',()=>{
 const notices=Array.from({length:5},(_,i)=>({id:'notice-'+i,fluctlight_id:'A',title:'新回复',body:'有一条已保存的角色回复',link:'/c/A/chat?conversation=C&message=M'+i}));
 const groups=notificationGroups([...notices,{...notices[0],id:'duplicate'},{id:'other',fluctlight_id:'B',read_at:'already-read'}],[{id:'A',name:'雨禾'}]);
 assert.equal(groups.length,1);assert.equal(groups[0].name,'雨禾');assert.equal(groups[0].notices.length,6);
 assert.deepEqual(groups[0].notices.map(n=>n.id),[...notices.map(n=>n.id),'duplicate']);
});
test('diary dates and terminal receipts never manufacture generated prose',()=>{
 assert.equal(previousDiaryDate('2026-01-01'),'2025-12-31');
 assert.equal(previousDiaryDate('2024-03-01'),'2024-02-29');
 const receipt={local_date:'2026-10-23',status:'no_content'};
 assert.match(diaryRequestText(receipt),/2026-10-23.*没有足够/);
 assert.equal(receipt.body,undefined);
 assert.match(diaryRequestText({...receipt,status:'pending'}),/排队/);
 assert.match(diaryRequestText({...receipt,status:'failed'}),/没有生成/);
});
test('diagnostic previews keep raw formatting in detail rather than leaking it into summaries',()=>{
 for(const text of ['```json\n{"x":1}\n```','|col|value|\n|--|--|','English analysis from a model']){
  assert.equal(readableSummary(text,'查看原记录'),'查看原记录');
 }
 const record={ref:'run',data:{status:'completed',metrics:{scenario:'conversation_segment'},response:{content:'```json\n{}\n```'}}};
 const view=diagnosticProjection(record,0,[]);
 assert.equal(view.label,'对话片段归纳');assert.equal(view.character,'尚未关联角色');
 assert.doesNotMatch(view.summary,/```|English/);
 assert.match(view.response,/```/);
});
test('truncated generation and malformed imports have actionable Chinese feedback',()=>{
 const feedback=creationFeedback({code:'initialization_response_truncated'});
 assert.match(feedback.message,/长度终止.*尚未确认/);assert.doesNotMatch(feedback.message,/服务.*不可用/);
 assert.throws(()=>parseCreationInput('{broken'),/JSON 格式不正确.*原内容已保留/);
});
