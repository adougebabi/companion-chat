import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {readFile} from 'node:fs/promises';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {default:Domain}=await load('react-app/pages/DomainPage.tsx');
const {renderRealSchedule}=await load('v3-domain/life-schedule.js');
const services=await load('v3/proto/services.js'),store=await load('v3/proto/store.js');
const sections=[['promises','约定'],['intentions','意向'],['upcoming','接下来几天']];
async function mount(section){
 const state=proxy({session:{actorId:'sheet-test-owner'},selectedId:'A'});
 store.replaceSnapshot({...store.get(),session:{actorId:state.session.actorId}});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){}};
 const calls=[];
 services.configureDomainService('life',{
  detail:async()=>({context:{},schedule:{items:[]}}),
  schedules:async(_fid,query={})=>{calls.push(query);return {timezone:'Asia/Shanghai',today:'2026-11-16',from:query.from||'2026-11-16',days:[{local_date:query.from||'2026-11-16',schedule:{items:[]}}]};},
  requests:async()=>({items:[],next_cursor:''}),promises:async()=>({items:[],next_cursor:''}),
 });
 const route={page:'日程',path:'/c/A/life/schedule/planning',query:new URLSearchParams({section}),feature:{title:'安排与调整日程',params:{},load:async()=>renderRealSchedule}};
 const v=await mountReact(vite,Domain,{workspace:w,props:{route}});
 await v.flush();return {...v,calls,route};
}
for(const [section,label] of sections)test('planning route '+section+' directly shows '+label,async()=>{
 const v=await mount(section);
 try{
  assert.equal(document.querySelector('[role=dialog]'),null);
  assert.equal(v.host.querySelector('[role=tab][aria-selected=true]').textContent,label);
  assert.equal(v.host.querySelectorAll('[data-section]').length,3);
  assert.equal(v.host.querySelector('[data-section="'+section+'"]').hidden,false);
  const back=[...v.host.querySelectorAll('a')].find(a=>a.textContent.includes('返回当天安排'));
  assert.equal(back.getAttribute('href'),'#/c/A/life/schedule');
  assert.doesNotMatch(v.host.textContent,/undefined|active_profile_id|context_revision/);
 }finally{await v.dispose();}
});
test('switching planning tabs reads seven future days and keeps panels on the page',async()=>{
 const v=await mount('promises');
 try{
  for(const [section,label] of sections){
   await act(async()=>[...v.host.querySelectorAll('[role=tab]')].find(n=>n.textContent===label).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));
   await v.flush();
   assert.equal(v.host.querySelector('[role=tab][aria-selected=true]').textContent,label);
   for(const panel of v.host.querySelectorAll('[data-section]'))assert.equal(panel.hidden,panel.dataset.section!==section);
  }
  assert.ok(v.calls.some(query=>query.from==='2026-11-17'&&query.to==='2026-11-23'));
  assert.equal(document.querySelector('[role=dialog]'),null);
 }finally{await v.dispose();}
});

test('old today and habits deep links retain their date and use the main schedule implementation',()=>{
 for(const [section,tab] of [['today','timeline'],['habits','habits']]){
  const moves=[];
  renderRealSchedule(document.createElement('div'),{charId:'A',query:{section,date:'2026-11-18'},navigate:path=>moves.push(path)});
  assert.deepEqual(moves,['#/c/A/life/schedule?tab='+tab+'&date=2026-11-18']);
 }
});
