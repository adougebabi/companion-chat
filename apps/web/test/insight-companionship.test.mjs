import test from 'node:test';
import assert from 'node:assert/strict';
import {sidebarSectionPath,exchangeNavigation,globalNavigation} from '../src/v3/workspace-navigation.mjs';
import {pendingSummary} from '../src/v3/pending-presentation.mjs';
import {lifeTimelineRows} from '../src/v3/life-timeline.mjs';
import {readFileSync} from 'node:fs';
import {chatContextText} from '../src/v3/chat-feedback.mjs';

test('chat context omits unknown values and labels a schedule as an arrangement',()=>{
 assert.equal(chatContextText(null),'');
 assert.equal(chatContextText({source:'schedule',activity:'看书',location:'书屋'}),'安排 · 看书 · 书屋');
 assert.equal(chatContextText({source:'event',activity:'看书',location:'书屋'}),'看书 · 书屋');
});

test('approved navigation is restored to the pre-ab4d33a3 structure',()=>{
 assert.deepEqual(exchangeNavigation.map(n=>n.label),['对话','生活','世界','人物关系','记忆成长','媒体','角色资料']);
 assert.deepEqual(exchangeNavigation.find(n=>n.k==='life').subs.map(([label])=>label),['此刻','日程','目标','衣柜','日记','共同经历']);
 assert.equal(sidebarSectionPath('/c/role/life'),'/c/role/life/now');
 assert.ok(globalNavigation('管理').some(([,href])=>href==='#/diag'));
});
test('17 service failures and one running request are not two actual human decisions',()=>{
 const groups=[{kind:'capability',count:1,status:'ok'},{kind:'persona_proposal',count:1,status:'ok'},{kind:'studio_apply',count:0,status:'ok'},{kind:'owner_request',count:1,status:'ok'},{kind:'failed_task',count:17,status:'ok'}];
 assert.deepEqual(pendingSummary(groups),{attention:2,attentionKnown:true,progress:1,progressKnown:true,service:17,serviceKnown:true});
 assert.equal(pendingSummary(groups.map(g=>g.kind==='capability'?{...g,status:'failed'}:g)).attentionKnown,false);
});
test('life timeline mixes authoritative reading sources without empty diaries or fabricated executions',()=>{
 const rows=lifeTimelineRows('role',{schedule:[{local_date:'2026-10-23',schedule:{items:[{id:'plan',start_at:'2026-10-23T13:00:00+08:00',activity:'去书店',status:'planned'}]}}],diary:[{id:'empty',date:'2026-10-22',body:''},{id:'diary',date:'2026-10-21',body:'一段日记'}],moments:[{id:'mine',author_type:'user',text:'不混入她的生活'},{id:'hers',author_type:'fluctlight',text:'她的分享',created_at:'2026-10-22T12:00:00Z'}],media:[{id:'pic',readable:true,kind:'image',created_at:'2026-10-20T12:00:00Z'}],goals:[{id:'goal',desired_outcome:'开一家书店'}]});
 assert.equal(rows.length,5);assert.ok(!rows.some(r=>r.id==='diary:empty'||r.id==='moments:mine'));assert.equal(rows.find(r=>r.id==='schedule:plan').label,'日程安排');assert.ok(rows.every(r=>r.href.startsWith('#/')));
});
function node(){const children=new Map();return {innerHTML:'',textContent:'',isConnected:true,querySelector(s){if(!children.has(s))children.set(s,node());return children.get(s);},querySelectorAll(){return [];}};}
test('default list shows sparse older records and excludes empty diary days without losing their cursor',async()=>{
 const source=readFileSync(new URL('../src/v3/proto/components/calendar-reading.js',import.meta.url),'utf8').replace(/^import .*;\s*$/gm,'').replaceAll('export function ','function ');
 const {mountCalendarReading}=Function('esc','Field','displayTimezone','store','errorMessage',source+';return {mountCalendarReading}') (String,()=>'',()=> 'Asia/Shanghai',{get:()=>({actorId:'owner'}),subscribe(){}},String);
 const el=node();let view;
 const dispose=mountCalendarReading(el,{scope:['role','diary'],initialMode:'list',read:async()=>({items:[{id:'old',date:'2025-01-01',body:'她的旧日记'},{id:'empty',date:'2026-10-23',body:''}],next_cursor:'older'}),dateOf:r=>r.date,visible:r=>!!r.body,render:r=>r.body,onView:v=>view=v});
 await new Promise(resolve=>setImmediate(resolve));assert.match(el.querySelector('[data-papers]').innerHTML,/她的旧日记/);assert.doesNotMatch(el.querySelector('[data-papers]').innerHTML,/empty/);assert.equal(view.month,'');assert.equal(el.querySelector('[data-reading-more]').hidden,false);dispose();
});

test('life refresh removes hidden records and reopens a newly grown cursor',async()=>{
 const source=readFileSync(new URL('../src/v3-domain/life-timeline.js',import.meta.url),'utf8').replace(/^import .*;\s*$/gm,'').replaceAll('export ','');
 let changed=false;const el=node();const document={hidden:false,addEventListener(){},removeEventListener(){}};
 const render=Function('get','subscribe','lifeClient','diaryClient','momentsClient','mediaClient','lifeTimelineRows','esc','instant','document','setTimeout','clearTimeout',source+';return renderLifeTimeline')(
 ()=>({actorId:'owner'}),()=>{}, {schedules:async()=>({days:[]}),goals:async()=>({items:[]})}, {list:async()=>({items:[],next_cursor:changed?'new-older-cursor':''})}, {list:async()=>({items:changed?[]:[{id:'post',author_type:'fluctlight',text:'now-hidden-post'}]})}, {list:async()=>({items:changed?[]:[{id:'photo',kind:'image',readable:true,title:'now-hidden-photo'}]}),url:id=>'/api/media/'+id},lifeTimelineRows,String,String,document,()=>1,()=>{});
 const stop=render(el,{charId:'role'});await new Promise(resolve=>setImmediate(resolve));assert.match(el.querySelector('[data-life-items]').innerHTML,/now-hidden-post|now-hidden-photo/);assert.equal(el.querySelector('[data-life-more]').hidden,true);
 changed=true;el.querySelector('[data-life-refresh]').onclick();await new Promise(resolve=>setImmediate(resolve));assert.doesNotMatch(el.querySelector('[data-life-items]').innerHTML,/now-hidden-post|now-hidden-photo/);assert.equal(el.querySelector('[data-life-more]').hidden,false);stop();
});

test('all-memory reading retains explicit false through persisted view URLs',()=>{
 const source=readFileSync(new URL('../src/v3-domain/memory.js',import.meta.url),'utf8').replace(/^import .*;\s*$/gm,'').replaceAll('export ','');let options,url='';const el=node();
 const render=Function('get','mountCalendarReading','Tabs','SearchForm','esc','history',source+';return renderRealMemoryList')(()=>({actorId:'owner'}),(_el,v)=>{options=v;return()=>{};},()=>'',()=>'',String,{replaceState:(_a,_b,value)=>url=value});
 render(el,{charId:'role',path:'/c/role/growth',query:{aboutOwner:'false'}});options.onView({mode:'list',date:'',month:''});assert.match(url,/aboutOwner=false/);assert.match(el.innerHTML,/她的全部记忆/);assert.doesNotMatch(options.emptyText,/关于你的事/);
});

test('reading empty states distinguish onboarding, active filters and uncovered dates',async()=>{
 const source=readFileSync(new URL('../src/v3/proto/components/calendar-reading.js',import.meta.url),'utf8').replace(/^import .*;\s*$/gm,'').replaceAll('export function ','function ');
 const {mountCalendarReading}=Function('esc','Field','displayTimezone','store','errorMessage',source+';return {mountCalendarReading}')(String,()=>'',()=> 'Asia/Shanghai',{get:()=>({actorId:'owner'}),subscribe(){}},String);
 for(const scenario of ['new','filtered','uncovered']){
  const el=node(),stop=mountCalendarReading(el,{scope:[scenario],initialMode:'list',initialDate:scenario==='uncovered'?'2025-01-01':'',filtered:scenario==='filtered',emptyText:'新手空态',read:async()=>({items:scenario==='uncovered'?[{id:'recent',date:'2026-10-23'}]:[],next_cursor:scenario==='uncovered'?'older':''}),dateOf:r=>r.date,render:()=>''});
  await new Promise(resolve=>setImmediate(resolve));const html=el.querySelector('[data-papers]').innerHTML;
  assert.match(html,scenario==='new'?/新手空态/:scenario==='filtered'?/当前筛选没有已载入/:/尚未读到这个日期范围/);if(scenario!=='new')assert.doesNotMatch(html,/新手空态/);stop();
 }
});
