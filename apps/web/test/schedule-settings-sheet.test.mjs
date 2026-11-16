import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/'+path);
const {default:Domain}=await load('react-app/pages/DomainPage.tsx');
const {renderRealSchedule}=await load('v3-domain/life-schedule.js');
const services=await load('v3/proto/services.js'),store=await load('v3/proto/store.js');
const sections=[['today','当天安排'],['upcoming','接下来几天'],['intentions','意向'],['promises','约定'],['habits','生活倾向']];
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
for(const [section,label] of sections)test('route section '+section+' selects '+label+' and mounts every panel inside the Sheet',async()=>{
 const v=await mount(section);
 try{
  const dialog=document.querySelector('[role=dialog]');assert.ok(dialog);
  assert.equal(dialog.querySelector('[role=tab][aria-selected=true]').textContent,label);
  assert.equal(dialog.querySelectorAll('[data-section]').length,5);
  for(const panel of v.host.querySelectorAll('[data-section]'))assert.ok(dialog.contains(panel));
  assert.equal(dialog.querySelector('[data-section="'+section+'"]').hidden,false);
  const list=dialog.querySelector('[role=tablist]');
  assert.ok(list.classList.contains('overflow-x-auto'));assert.ok(list.classList.contains('flex-nowrap'));
  for(const tab of list.querySelectorAll('[role=tab]'))assert.ok(tab.classList.contains('whitespace-nowrap'));
  assert.doesNotMatch(dialog.textContent,/undefined|active_profile_id|context_revision/);
 }finally{await v.dispose();}
});
test('switching all settings tabs, closing and reopening preserves selection and controller content',async()=>{
 const v=await mount('promises');
 try{
  for(const [section,label] of sections){
   const dialog=document.querySelector('[role=dialog]');
   await act(async()=>dialog.querySelectorAll('[role=tab]')[sections.findIndex(([key])=>key===section)].dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));
   await v.flush();
   assert.equal(dialog.querySelector('[role=tab][aria-selected=true]').textContent,label);
   for(const panel of dialog.querySelectorAll('[data-section]'))assert.equal(panel.hidden,panel.dataset.section!==section);
  }
  assert.ok(v.calls.some(query=>query.from==='2026-11-17'&&query.to==='2026-11-23'));
  assert.match(document.querySelector('[data-section=habits]').textContent,/当前人格尚未记录生活倾向/);
  await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();
  assert.equal(document.querySelector('[role=dialog]'),null);
  await v.click([...v.host.querySelectorAll('button')].find(node=>node.textContent==='约定与日程设置'));
  assert.equal(document.querySelector('[role=tab][aria-selected=true]').textContent,'生活倾向');
  assert.equal(document.querySelector('[data-section=habits]').hidden,false);
 }finally{await v.dispose();}
});
