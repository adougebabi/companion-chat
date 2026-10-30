import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {creationFixture} from './support/creation-fixture.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const {default:Page}=await vite.ssrLoadModule('/src/react-app/pages/CharacterWizardPage.jsx'),
 {default:View}=await vite.ssrLoadModule('/src/react-app/views/CharacterWizard.jsx'),
 {default:Preparation}=await vite.ssrLoadModule('/src/react-app/shared/PreparationSteps.jsx');
test.after(async()=>{await vite.close();env.restore();});
const button=(root,text)=>[...root.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
const tabs=root=>[...root.querySelectorAll('[role=tab]')].map(n=>n.textContent.trim());
async function select(v,text){await act(async()=>[...v.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.trim()===text).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0,ctrlKey:false})));await v.flush();}
function workspace({characters=[],worldCatalog=async()=>({items:[]}),listStudioResources=async()=>({items:[],next_cursor:''})}={}){
 const state=proxy({session:{actorId:'r20-empty-owner'},characters,errors:{}}),mutations=[];
 return {state,mutations,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),day:()=> '2026-10-30',refreshDirectory:async()=>{},services:{configurationClient:{read:async()=>({bindings:[]})}},client:{worldCatalog,listStudioResources,listActorGroups:async()=>[],updateRuntimeWorld:async body=>mutations.push(body)}};
}
function model(c,extra={}){return {c,draft:c.draft,step:'world',currentStep:0,busy:false,pending:null,previewFields:[],error:'',fieldErrors:{},groups:[],existingWorlds:[],worlds:[],templates:[],canContinue:false,goStep(){},configHref:()=> '#/settings',...extra};}
test('confirmed full empty catalogs show only an unselected daily card, create no world and block Next until explicit choice',async()=>{
 localStorage.clear();const w=workspace(),v=await mountReact(vite,Page,{workspace:w});
 try{assert.deepEqual(tabs(v.host),[]);assert.ok(button(v.host,'使用日常生活世界'));assert.equal(v.host.querySelector('.creation-daily.selected'),null);assert.equal(button(v.host,'继续，认识角色').disabled,true);assert.ok(v.host.querySelector('a[href="#/studio/edit/new?kind=world"]'));assert.deepEqual(w.mutations,[]);
 const savedBefore=Object.keys(localStorage).map(key=>localStorage.getItem(key)).join('');assert.doesNotMatch(savedBefore,/"worldChoiceConfirmed":true/);
 await v.click(button(v.host,'使用日常生活世界'));assert.ok(v.host.querySelector('.creation-daily.selected'));assert.equal(button(v.host,'继续，认识角色').disabled,false);assert.deepEqual(w.mutations,[]);
 }finally{await v.dispose();}
});
test('loading and failed source reads retain their tabs and retry the published source without writes',async()=>{
 localStorage.clear();let resolveDirectory,failed=true;
 const w=workspace({listStudioResources:async query=>query.kind==='world'?await new Promise((resolve,reject)=>{resolveDirectory=()=>failed?reject(Error('fetch failed')):resolve({items:[],next_cursor:''});}):{items:[],next_cursor:''}}),v=await mountReact(vite,Page,{workspace:w});
 try{assert.ok(tabs(v.host).includes('已发布的世界底稿'));assert.equal(button(v.host,'继续，认识角色').disabled,true);await select(v,'已发布的世界底稿');assert.match(v.host.textContent,/正在读取已发布/);await act(async()=>resolveDirectory());await v.flush();assert.ok(tabs(v.host).includes('已发布的世界底稿'));assert.ok(button(v.host,'重新读取世界'));failed=false;await v.click(button(v.host,'重新读取世界'));await act(async()=>resolveDirectory());await v.flush();assert.deepEqual(tabs(v.host),[]);assert.equal(v.host.querySelector('.creation-daily.selected'),null);assert.deepEqual(w.mutations,[]);
 }finally{await v.dispose();}
});
test('partial or cursor catalogs cannot hide empty source tabs',async()=>{
 localStorage.clear();const w=workspace({characters:[{id:'role',name:'角色'}],worldCatalog:async()=>({items:[],next_cursor:'unread'}),listStudioResources:async query=>({items:[],next_cursor:query.kind==='world'?'next-world':''})}),v=await mountReact(vite,Page,{workspace:w});
 try{assert.deepEqual(tabs(v.host),['已有角色的世界','已发布的世界底稿','日常生活世界']);await select(v,'已发布的世界底稿');assert.ok(button(v.host,'读取更多世界'));assert.equal(button(v.host,'继续，认识角色').disabled,true);assert.deepEqual(w.mutations,[]);}finally{await v.dispose();}
});
test('an automatic empty fallback reveals late worlds and read failures, while an explicit daily view remains chosen',async()=>{
 localStorage.clear();let fail=false;
 const w=workspace({worldCatalog:async()=>{if(fail)throw Error('fetch failed');return {items:[{id:'late-world',name:'后来读到的世界',revision:1}]};}}),v=await mountReact(vite,Page,{workspace:w});
 try{
  assert.deepEqual(tabs(v.host),[]);assert.ok(button(v.host,'使用日常生活世界'));
  await act(async()=>{w.state.characters=[{id:'late-role',name:'后来读到的角色'}];});await v.flush();
  assert.ok(tabs(v.host).includes('已有角色的世界'));assert.match(v.host.textContent,/后来读到的世界/);
  await select(v,'日常生活世界');
  fail=true;await act(async()=>{w.state.characters[0].name='读取失败的角色';});await v.flush();
  assert.ok(tabs(v.host).includes('已有角色的世界'));assert.ok(button(v.host,'使用日常生活世界'));
  await select(v,'已有角色的世界');assert.ok(button(v.host,'重新读取世界'));assert.match(v.host.textContent,/后来读到的世界/);assert.deepEqual(w.mutations,[]);
 }finally{await v.dispose();}
});
test('filter empty keeps nonempty catalog tabs; saved exact sources absent from the first page stay readable or explicitly unresolved',async()=>{
 const f=creationFixture();f.c.draft.worldResourceId='selected-id';f.c.draft.worldResourceVersion='saved:v2';f.c.draft.worldName='保存的世界';
 const m=model(f.c,{worldsConfirmed:true,publishedWorldsConfirmed:true,selectedPublicationError:'精确版本读取失败'}),v=await mountReact(vite,View,{props:{model:m}});
 try{assert.ok(tabs(v.host).includes('已发布的世界底稿'));const row=v.host.querySelector('.creation-world-card');assert.match(row.textContent,/保存的世界/);assert.match(row.textContent,/saved:v2/);assert.equal(row.disabled,true);assert.ok(button(v.host,'重新读取所选世界版本'));
 await v.update({model:{...m,selectedPublicationError:'',selectedPublication:{id:'selected-id',version:'saved:v2',name:'精确发布名称',body:{background:'原始背景'},immutable:true},canContinue:true}});assert.equal(v.host.querySelector('.creation-world-card').disabled,false);assert.match(v.host.textContent,/精确发布名称/);assert.doesNotMatch(v.host.querySelector('.creation-world-card').textContent,/尚未读到/);
 await v.input(v.host.querySelector('input[type=search]'),'无匹配');assert.equal(v.host.querySelector('.creation-world-card'),null);assert.ok(tabs(v.host).includes('已发布的世界底稿'));
 }finally{await v.dispose();f.c.dispose();}
 const g=creationFixture();g.c.draft.worldSource={worldId:'old-world',fluctlightId:'old-role',name:'原世界',revision:4};const missing=await mountReact(vite,View,{props:{model:model(g.c,{worldsConfirmed:true,publishedWorldsConfirmed:true})}});
 try{assert.ok(tabs(missing.host).includes('已有角色的世界'));assert.match(missing.host.textContent,/所选世界尚未读到/);assert.equal(missing.host.querySelector('.creation-world-card').disabled,true);}finally{await missing.dispose();g.c.dispose();}
});
test('failed initialization states the actual blocker, preserves early chat and dispatches only the supported failed retry',async()=>{
 const f=creationFixture();f.c.draft.activatedId='created';let recovered;
 f.c.recoverStage=stage=>{recovered=stage;};
 const failed={key:'wardrobe',title:'初始衣柜',state:'failed',note:'读取角色配置失败',retryable:true},blocked={key:'schedule',title:'日程',state:'blocked',retryable:true};
 const m=model(f.c,{currentStep:3,step:'progress',progress:{complete:false,stages:[{key:'character',title:'角色',state:'ready'},{key:'world',title:'世界',state:'ready'},failed,blocked]}}),v=await mountReact(vite,View,{props:{model:m}});
 try{assert.match(v.host.querySelector('.creation-meet').textContent,/角色与世界已建立，初始化仍需处理/);assert.match(v.host.querySelector('.creation-meet').textContent,/当前需要处理「初始衣柜」/);assert.match(v.host.querySelector('[data-testid=prep-step-wardrobe]').textContent,/读取角色配置失败/);assert.doesNotMatch(v.host.textContent,/已经可以开始相处了/);assert.equal(v.host.querySelector('.creation-meet a').getAttribute('href'),'#/c/created/chat');assert.equal(v.host.querySelector('[data-testid=prep-step-schedule] button'),null);await v.click(button(v.host,'重试此环节'));assert.equal(recovered.key,'wardrobe');assert.equal(recovered.note,failed.note);
 }finally{await v.dispose();f.c.dispose();}
});
test('preparation opens original task stages locally and links separately to model records without fetching them',async()=>{
 const f=creationFixture();f.c.draft.activatedId='created';const reads=[],calls=[],w=workspace();
 w.services.operationsClient={task:async id=>{reads.push(id);return {task_id:id,title:'真实衣柜任务',status:'failed',correlation_id:'original-correlation'};}};
 w.services.diagnosticsClient={page:async(kind,options)=>{calls.push([kind,options]);return {items:[{ref:'actual-call',data:{prompt:{original:0},response:{kept:false},status:'failed'}}],next_cursor:''};}};
 const m=model(f.c,{currentStep:3,step:'progress',progress:{complete:false,stages:[{key:'world',title:'世界',state:'ready'},{key:'wardrobe',title:'衣柜',state:'failed',href:'#/diag/task/task%2Fexact%3Fid'}]}}),v=await mountReact(vite,View,{workspace:w,props:{model:m}});
 try{location.hash='#/library/create?resume=original';const hash=location.hash,trigger=button(v.host,'查看任务记录');await v.click(trigger);const sheet=document.querySelector('[data-slot=sheet-content]');assert.ok(sheet);assert.equal(document.querySelectorAll('[role=dialog]').length,1);assert.equal(sheet.querySelector('[data-slot=sheet-title]').textContent,'真实衣柜任务');assert.equal(sheet.querySelector('[role=tab][data-state=active]').textContent,'执行阶段');assert.doesNotMatch(sheet.textContent,/"original": 0/);await act(async()=>[...sheet.querySelectorAll('[role=tab]')].find(node=>node.textContent==='任务信息').dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await v.flush();assert.match(sheet.querySelector('a[href^="#/diag?view=calls"]').getAttribute('href'),/correlationId=original-correlation/);assert.deepEqual(reads,['task/exact?id']);assert.deepEqual(calls,[]);assert.equal(location.hash,hash);
 await act(async()=>sheet.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));await v.flush();assert.equal(document.querySelector('[role=dialog]'),null);assert.equal(document.activeElement,trigger);assert.equal(location.hash,hash);assert.deepEqual(w.mutations,[]);
 }finally{await v.dispose();f.c.dispose();}
});
test('non-task and malformed task URLs retain the existing navigation fallback',async()=>{
 const opened=[],v=await mountReact(vite,Preparation,{props:{stages:[{key:'world',title:'世界',state:'unknown',href:'#/worlds'},{key:'malformed',title:'错误链接',state:'unknown',href:'#/diag/task/%E0%A4%A'}],onOpenTask:(...args)=>opened.push(args)}});
 try{const links=[...v.host.querySelectorAll('a')];assert.equal(links.length,2);assert.ok(links[0].getAttribute('href').startsWith('#/worlds?returnTo='));assert.equal(opened.length,0);}finally{await v.dispose();}
});
