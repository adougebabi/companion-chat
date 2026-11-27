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

// Geometry itself is verified in real Chromium (e2e/schedule-settings.spec.ts).
// This only guards the ownership contract: the sheet uses the shared
// ResponsiveSheet and no business rule overrides its width.
test('日程设置使用共享 ResponsiveSheet，业务 CSS 不再覆盖其宽度',async()=>{
 const css=await readFile(new URL('../src/styles/react.css',import.meta.url),'utf8');
 assert.doesNotMatch(css,/schedule-settings-sheet\s*\{[^}]*width/);
 assert.doesNotMatch(css,/75vw/);
 assert.match(css,/@import "\.\/overlay\.css";/);
 const v=await mount('promises');
 try{
  const dialog=document.querySelector('[data-slot=sheet-content]');
  assert.ok(dialog.classList.contains('schedule-settings-sheet'));
  assert.ok(dialog.hasAttribute('data-responsive-sheet'));
  assert.equal(dialog.dataset.side,'right');
  for(const part of ['header','body'])assert.ok(dialog.querySelector(`:scope > [data-responsive-sheet-part="${part}"]`),part);
 }finally{await v.dispose();}
});

for(const width of [390,1280])test(`${width}px 日程遮罩与 Sheet 同属 domain portal，覆盖视口且点击可关闭再打开`,async()=>{
 const v=await mount('promises');
 try{
  Object.defineProperty(window,'innerWidth',{configurable:true,value:width});
  Object.defineProperty(window,'innerHeight',{configurable:true,value:844});
  const dialog=document.querySelector('[data-slot=sheet-content]');
  const overlay=document.querySelector('[data-slot=sheet-overlay]');
  assert.ok(overlay);
  assert.ok(overlay.parentElement===dialog.parentElement, '遮罩与 Sheet 共用 portal 容器');
  assert.ok(overlay.nextElementSibling===dialog, '遮罩紧邻 Sheet 内容');
  assert.ok(v.host.querySelector('.domain-content').contains(overlay));
  for(const name of ['fixed','inset-0','z-50'])assert.ok(overlay.classList.contains(name));
  assert.equal(overlay.style.pointerEvents,'auto');
  assert.equal(dialog.querySelector('[role=tab][aria-selected=true]').textContent,'约定');
  assert.equal(dialog.querySelectorAll('[data-section]').length,5);
  // Radix defers outside dismissal until the click after pointerdown.
  await act(async()=>{
   overlay.dispatchEvent(new window.MouseEvent('pointerdown',{bubbles:true,button:0,clientX:48,clientY:422}));
   overlay.click();
  });
  await v.flush();
  assert.ok(document.querySelector('[role=dialog]')===null);
  assert.ok(document.querySelector('[data-slot=sheet-overlay]')===null);
  await v.click([...v.host.querySelectorAll('button')].find(node=>node.textContent==='约定与日程设置'));
  const reopened=document.querySelector('[role=dialog]');
  assert.equal(reopened.querySelector('[role=tab][aria-selected=true]').textContent,'约定');
  assert.equal(reopened.querySelector('[data-section=promises]').hidden,false);
  assert.equal(reopened.querySelectorAll('[data-section]').length,5);
 }finally{await v.dispose();}
});
