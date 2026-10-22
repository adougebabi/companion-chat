import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {requestDecisionText,RequestDecisionSource,RequestFailureLink} from '../src/v3/proto/components/request-failure.js';
import {esc,badge} from '../src/v3/proto/components/controls.js';
import {Tabs} from '../src/v3/proto/components/forms.js';
import {JsonDetails} from '../src/v3/proto/components/records.js';
import {userInstant} from '../src/v3/presentation-time.mjs';
import {stateLabel,domainName} from '../src/v3/domain-presentation.mjs';

const failed={id:'original-request',status:'failed',decision_reason:'The request did not produce a committed result',created_at:'2026-10-20T15:01:58.000+00:00',payload:{text:'我的建议',stance:'agree'},result:{error_code:'owner_life_request_not_applied',disposition:'not_applied'}};
const bindings={get:()=>({actorId:"owner"}),errorMessage:()=>"读取暂不可用",domainName,esc,badge,JsonDetails,userInstant,instant:userInstant,domainStateLabel:stateLabel,requestDecisionText,RequestDecisionSource,RequestFailureLink};
function load(path,names,extras={}){
 const source=readFileSync(new URL('../src/v3/proto/'+path,import.meta.url),'utf8').replace(/^import .+;\s*$/gm,'').replaceAll('export ','');
 const args={...bindings,...extras};return Function(...Object.keys(args),source+`\nreturn {${names.join(',')}};`)(...Object.values(args));
}
function element(){const nodes=new Map();return {isConnected:true,innerHTML:'',querySelector(selector){if(!nodes.has(selector))nodes.set(selector,{innerHTML:'',insertAdjacentHTML(){},addEventListener(){}});return nodes.get(selector);},querySelectorAll:()=>[]};}
function ordinary(html){return html.replace(/<details\b[^>]*>[\s\S]*?<\/details>/g,'');}
function checkDecision(html){
 assert.match(ordinary(html),/没有可确认的已提交结果/);
 assert.doesNotMatch(ordinary(html),/failed|The request|2026-10-20T15:01:58/);
 assert.match(html,/<details[^>]*><summary>请求原始状态、理由与时间<\/summary>/);
 assert.match(html,/The request did not produce a committed result/);
 assert.match(html,/2026-10-20T15:01:58.000\+00:00/);
}

test('fixed settlement reasons are localized with status/effect evidence and authored Chinese stays intact',()=>{
 const backend=readFileSync(new URL('../../core-go/internal/core/owner_life_requests.go',import.meta.url),'utf8');
 const cases=[
  [failed,'没有可确认'],
  [{status:'completed',decision_reason:'The requested change was committed'},'变更已提交'],
  [{status:'accepted',decision_reason:'An intention was recorded; fulfillment requires later evidence'},'后续实际证据'],
  [{status:'awaiting_goal',decision_reason:'The proposed action awaits explicit Planner goal binding; no accepted promise yet'},'约定尚未接受'],
  [{status:'completed',decision_reason:'The exact requested target state was already recorded; no new mutation was needed',result:{disposition:'no_change'}},'无需新增操作'],
 ];
 for(const [request,expected] of cases){assert.ok(backend.includes(request.decision_reason));assert.ok(requestDecisionText(request).includes(expected));}
 for(const status of ['rejected','deferred','failed'])assert.equal(requestDecisionText({...failed,status,decision_reason:'先休息，明天再说。'}),'先休息，明天再说。');
 const unknown=requestDecisionText({...failed,decision_reason:'Unexpected engine fault',result:{error_code:'unknown',mutation_committed:true}});
 assert.match(unknown,/处理失败/);assert.doesNotMatch(unknown,/成功|未产生|没有操作|无副作用|Unexpected/);
 const assessment=requestDecisionText({...failed,result:{error_code:'owner_life_assessment_failed',mutation_committed:true}});
 assert.match(assessment,/未能完成判断/);assert.doesNotMatch(assessment,/未产生|无副作用|没有操作/);
 assert.doesNotMatch(requestDecisionText({status:'mystery',decision_reason:'The requested change was committed'}),/变更已提交|成功/);
 const unsafe=RequestDecisionSource({...failed,decision_reason:'<img src=x onerror=alert(1)>'});
 assert.match(unsafe,/&lt;img/);assert.doesNotMatch(unsafe,/<img|<details[^>]*\bopen\b/);
 assert.equal(failed.decision_reason,'The request did not produce a committed result');
});

test('actual dynamic suggestion renderer localizes the exact failure and converts UTC to Beijing time',()=>{
 const panels=[];const el=element();
 const {renderRealFeed}=load('pages/real-moments.js',['renderRealFeed'],{stateLabel,get:()=>({actorId:'owner'}),list:()=>[],Field:()=>'',location:{hash:'#/feed'},mountMomentComposer:()=>({dispose(){}}),mountCursorRecords:(_el,options)=>{panels.push(options);return {dispose(){}};}});
 const dispose=renderRealFeed(el,{query:{}});const html=panels[1].render(failed);checkDecision(html);
 assert.match(ordinary(html),/执行失败 · .* · 2026-10-20 23:01 \(北京时间\)/);
 assert.match(html,/#\/diag\/request\/original-request/);assert.doesNotMatch(html,/data-retry/);dispose();
});

test('actual reflection feedback and promise details share localized decisions and original source disclosure',async()=>{
 let feedback;const reflectionEl=element();
 const reflection={title:'反思',status:'applied',summary:'一次回顾',at:failed.created_at,dispositions:[],references:[]};
 const {renderRealReflection}=load('pages/real-reflection.js',['renderRealReflection'],{reflectionClient:{read:async()=>reflection},asyncPage:async(_el,read,paint)=>paint(await read(),()=>{},()=>true),mountCursorRecords:(_el,options)=>{feedback=options;return {dispose(){}};}});
 await renderRealReflection(reflectionEl,{charId:'role',params:{refId:'reflection'}});
 const html=feedback.render(failed);checkDecision(html);assert.match(ordinary(html),/认同 · 2026-10-20 23:01 \(北京时间\)/);assert.match(ordinary(html),/执行失败/);
 const d={el:element(),close(){}};
 const {openPromise}=load('components/promise-catalogue.js',['openPromise'],{subscribe(){},get:()=>null,drawer:()=>d,lifeClient:{},asyncPage:(_el,_read,paint)=>paint({title:'约定',lifecycle:'failed',legacy_unlinked:true,request:failed},()=>{},()=>true),mountCursorRecords:()=>({dispose(){}})});
 openPromise('role','original-request');checkDecision(d.el.querySelector('[data-promise-detail]').innerHTML);
});

test('terminal request receipt keeps original identity and exposes raw diagnostic record without resubmission',async()=>{
 const d={el:element(),close(){}};let observed;const queries=[];
 const {requestReceipt}=load('components/async-page.js',['requestReceipt'],{stateLabel,drawer:()=>d,InventoryEffects:()=>'',lifeClient:{request:async(fid,id)=>{queries.push([fid,id]);return failed;}}});
 requestReceipt('role',{request:failed},r=>{observed=r;});checkDecision(d.el.querySelector('[data-request]').innerHTML);
 await d.el.querySelector('[data-query]').onclick();assert.deepEqual(queries,[['role','original-request']]);
 requestReceipt('role',{request:{...failed,result:{disposition:'no_change'}}});
 assert.doesNotMatch(ordinary(d.el.querySelector('[data-request]').innerHTML),/目标状态已满足/);
 assert.equal(observed,failed);assert.match(d.el.querySelector('[data-request]').innerHTML,/#\/diag\/request\/original-request/);
});

test('request detail keeps its complete original record after shared localization',async()=>{
 const el=element();
 const {renderRealRequest}=load('pages/real-operations.js',['renderRealRequest'],{stateLabel,domainName,operationsClient:{request:async()=>failed},asyncPage:async(_el,read,paint)=>paint(await read(),()=>{})});
 await renderRealRequest(el,{params:{requestId:failed.id}});checkDecision(el.innerHTML);
 assert.match(ordinary(el.innerHTML),/2026-10-20 23:01 \(北京时间\)/);
});

test('diary renders only all/favorite filters while generation and independent shared route/sidebar remain',()=>{
 let calendar;const el=element();
 const {renderRealDiaryList}=load('pages/real-diary.js',['renderRealDiaryList'],{store:{subscribe(){},get:()=>({actorId:'owner',timezone:'Asia/Shanghai'})},Tabs,mountCalendarReading:(_el,options)=>{calendar=options;return ()=>{};}});
 const dispose=renderRealDiaryList(el,{charId:'role',query:{}});
 assert.match(el.innerHTML,/全部日记/);assert.match(el.innerHTML,/>收藏</);assert.equal((el.innerHTML.match(/<a /g)||[]).length,2);
 assert.doesNotMatch(el.innerHTML,/共同经历|故事|life\/shared|diary-shared-action/);
 assert.match(el.innerHTML,/data-open-generation/);assert.deepEqual(calendar.scope,['role','diary',false]);dispose();
 const navigation=readFileSync(new URL('../src/v3/workspace-navigation.mjs',import.meta.url),'utf8');
 assert.match(navigation,/共同经历/);assert.match(navigation,/life\/shared/);
 const module=readFileSync(new URL('../src/v3/proto/modules/生活成长.js',import.meta.url),'utf8');assert.match(module,/renderRealShared/);
});
