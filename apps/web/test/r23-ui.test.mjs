import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,React,installDOM,mountReact} from './helpers/react-dom.mjs';
import {creationFixture} from './support/creation-fixture.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path),button=(root,text)=>[...root.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
test.after(async()=>{await vite.close();env.restore();});
function workspace(owner='r23-owner'){
 sessionStorage.clear();localStorage.clear();
 const state=proxy({session:{actorId:owner},selectedId:'A',characters:[{id:'A',name:'雨禾',initial:'雨',timezone:'Asia/Shanghai',schedule:[],wearing:{},garments:[]}],errors:{},loading:{},operations:[],managementTab:'配置中心',runtimeCatalog:{}});
 return {state,current:{get value(){return state.characters[0];}},subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),currentScope:()=>JSON.stringify([state.session?.actorId,state.selectedId]),isCurrent:(owner,fid)=>owner===state.session?.actorId&&fid===state.selectedId,client:{},services:{},navigatePath(){},navigate(){},notify(){},clock:()=> '12:00'};
}
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve};};
const conversationPage=id=>({conversation:{id:'C-'+id},participants:[],messages:[],nextBeforeSequence:null});
const {BrowserClient}=await vite.ssrLoadModule('/@fs'+fileURLToPath(new URL('../../../packages/browser-client/src/index.ts',import.meta.url)));
const {createConversationController,productConversation}=await load('state/conversation.ts');

test('P1-3 cancelled selection releases loading and obsolete completion cannot clear a newer role read',async()=>{
 const original=BrowserClient.prototype.directConversation,pending={A:deferred(),B:deferred()},s=createConversationController();s.fluctlights=[{id:'A',identity:{name:'雨禾'}},{id:'B',identity:{name:'林青'}}];
 BrowserClient.prototype.directConversation=id=>pending[id].promise;
 try{
  const a=s.selectFluctlight('A');assert.equal(s.loading,true);s.invalidateRequest();assert.equal(s.loading,false,'cancellation releases the read it invalidates');
  const b=s.selectFluctlight('B');assert.equal(s.loading,true);pending.A.resolve(conversationPage('A'));await a;assert.equal(s.loading,true,'obsolete finally must not clear B');assert.equal(s.conversation,null);
  pending.B.resolve(conversationPage('B'));await b;assert.equal(s.loading,false);assert.equal(s.fluctlightId,'B');assert.equal(s.conversation.id,'C-B');
 }finally{BrowserClient.prototype.directConversation=original;}
});

test('P1-3 leaving a pending chat then failing preflight keeps retry usable and empty state waits for successful reading',async()=>{
 const {default:Chat}=await load('react-app/pages/ChatPage.tsx'),provider=await load('v3/providers/real.ts'),w=workspace('r23-chat'),s=productConversation;
 s.$reset();s.fluctlights=[{id:'A',identity:{name:'雨禾'}}];s.fluctlightId='A';s.conversation={id:'previous'};
 const pending=deferred(),original=BrowserClient.prototype.directConversation,originalSession=provider.client.session,originalList=provider.client.listFluctlights,originalMessages=BrowserClient.prototype.messages;
 BrowserClient.prototype.directConversation=()=>pending.promise;provider.client.session=async()=>({authenticated:true,actorId:'r23-chat'});provider.client.listFluctlights=async()=>[{id:'A',identity:{name:'雨禾'}}];
 Object.assign(w,{conversation:()=>s,openConversation:provider.openConversation,detail:async()=>{},loadWardrobe:async()=>{}});
 let v=await mountReact(vite,Chat,{workspace:w,props:{preview:false,onGo(){}}});
 try{
  assert.match(v.host.textContent,/正在读取对话/);assert.equal(v.host.querySelector('.replica-chat-empty'),null);await v.dispose();assert.equal(s.loading,false);
  provider.client.session=async()=>{throw Error('会话前置读取失败');};v=await mountReact(vite,Chat,{workspace:w,props:{preview:false,onGo(){}}});
  assert.match(v.host.textContent,/会话前置读取失败/);assert.doesNotMatch(v.host.textContent,/正在读取对话/);assert.equal(button(v.host,'重新读取对话').disabled,false);assert.equal(v.host.querySelector('.replica-chat-empty'),null);
  provider.client.session=async()=>({authenticated:true,actorId:'r23-chat'});BrowserClient.prototype.directConversation=async()=>conversationPage('A');BrowserClient.prototype.messages=async()=>({messages:[],nextBeforeSequence:null});
  await v.click(button(v.host,'重新读取对话'));assert.ok(v.host.querySelector('.replica-chat-empty'));assert.doesNotMatch(v.host.textContent,/正在读取对话|会话前置读取失败/);
  pending.resolve(conversationPage('obsolete'));await v.flush();assert.equal(s.conversation.id,'previous');
 }finally{await v.dispose();BrowserClient.prototype.directConversation=original;provider.client.session=originalSession;provider.client.listFluctlights=originalList;BrowserClient.prototype.messages=originalMessages;s.$reset();}
});

test('P1-4 diagnostic task row speaks its title once and retains time and status',async()=>{
 const {default:Diagnostics}=await load('react-app/shared/DiagnosticsWorkspace.jsx'),w=workspace();w.services.diagnosticsClient={page:async()=>({items:[{ref:'task',occurred_at:'2026-10-31T06:15:00Z',data:{id:'task',kind:'world.events.generate',status:'running'}}],next_cursor:''})};
 const v=await mountReact(vite,Diagnostics,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
 try{const row=v.host.querySelector('.replica-task-row');assert.equal(row.querySelector('h3').textContent,'世界事件时钟');assert.doesNotMatch(row.querySelector('p').textContent,/世界事件时钟/);assert.match(row.querySelector('p').textContent,/10-31 14:00 整点（14:15 开始）/);assert.equal(row.querySelector('.replica-diagnostic-status').textContent,'状态待确认');}finally{await v.dispose();}
});

test('P2-1 quiet hours uses a labelled shadcn Switch row and preserves times while disabled',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx'),w=workspace();
 w.services={configurationClient:{read:async()=>({effective_values:{'product.autonomy':{quiet_hours:{start:'22:00',end:'07:00',days:[1,2]}}},versions:{},endpoints:[],bindings:[]}),models:async()=>({models:[]}),catalog:async()=>({agents:[]})},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}};
 const v=await mountReact(vite,Management,{workspace:w});
 try{await v.click(button(v.host,'自主性与安静时段'));const control=v.host.querySelector('#quiet-enabled'),row=control.closest('[data-slot=field]');assert.equal(control.getAttribute('role'),'switch');assert.ok(row.classList.contains('replica-quiet-toggle'));assert.ok(row.querySelector('label[for=quiet-enabled]'));assert.equal(row.querySelector('[data-slot=input]'),null);assert.equal(control.getAttribute('aria-checked'),'true');await v.click(control);assert.equal(control.getAttribute('aria-checked'),'false');assert.ok(v.host.querySelector('.replica-quiet-card fieldset').disabled);await v.click(control);assert.equal(control.getAttribute('aria-checked'),'true');assert.ok([...v.host.querySelectorAll('.replica-quiet-card input')].some(n=>n.value==='22:00'));}finally{await v.dispose();}
});

test('P2-2 character menu uses trigger width and selecting a role invokes the actual callback',async()=>{
 const {default:Navigation}=await load('react-app/shared/WorkspaceNavigation.tsx'),{SidebarProvider}=await load('components/ui/sidebar.tsx');let selected='';
 const characters=[{id:'A',name:'雨禾',job:'园艺师'},{id:'B',name:'很长的角色名字用于验证换行',job:'书店店主'}];
 function Shell(){return React.createElement(SidebarProvider,{defaultOpen:true},React.createElement(Navigation,{page:'聊天',character:characters[0],characters,onCharacterChange:id=>selected=id,onNavigate(){}}));}
 const v=await mountReact(vite,Shell);
 try{await act(async()=>v.host.querySelector('[aria-label="切换当前角色"]').dispatchEvent(new window.MouseEvent('pointerdown',{bubbles:true,button:0})));await v.flush();const menu=document.querySelector('[role=menu]');assert.ok(menu);assert.ok(menu.className.includes('w-[var(--radix-dropdown-menu-trigger-width)]'));await v.click([...menu.querySelectorAll('[role=menuitem]')][1]);assert.equal(selected,'B');}finally{await v.dispose();}
});

test('P2-3 empty diary shows a single leaf empty state and keeps filtered empty guidance',async()=>{
 const {default:Diary}=await load('react-app/pages/DiaryPage.jsx'),w=workspace();w.client={listDiary:async()=>({items:[],next_cursor:''}),sharedExperiences:async()=>({items:[],next_cursor:''})};
 const v=await mountReact(vite,Diary,{workspace:w});
 try{assert.equal(v.host.querySelectorAll('[data-slot=empty]').length,1);assert.match(v.host.textContent,/日记本还是空的/);assert.doesNotMatch(v.host.textContent,/没有找到这段回忆/);await v.input(v.host.querySelector('[aria-label=搜索日记]'),'不存在的回忆');assert.equal(v.host.querySelectorAll('[data-slot=empty]').length,1);assert.doesNotMatch(v.host.textContent,/没有找到这段回忆/);}finally{await v.dispose();}
});

test('P2-4 empty inbox uses the demo leaf and unread action restores the all tab',async()=>{
 const {default:Inbox}=await load('react-app/pages/InboxPage.jsx'),w=workspace();w.services.operationsClient={notifications:async()=>({items:[],as_of:'now',next_cursor:''}),pending:async()=>({groups:[]})};
 const v=await mountReact(vite,Inbox,{workspace:w});
 try{const empty=v.host.querySelector('.replica-notice-panel [data-slot=empty]');assert.equal(empty.dataset.emptyVariant,'page');assert.ok(empty.querySelector('.lucide-leaf'));assert.match(empty.textContent,/暂时没有通知/);await act(async()=>[...v.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.includes('未读')).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await v.flush();await v.click(button(v.host,'查看全部通知'));assert.match(v.host.querySelector('.replica-notice-panel').textContent,/暂时没有通知/);}finally{await v.dispose();}
});

test('P2-5 creation step one has a Chinese header and a separately spaced working world reread action',async()=>{
 const {default:Wizard}=await load('react-app/views/CharacterWizard.jsx'),f=creationFixture();let rereads=0;
 const m={c:f.c,draft:f.c.draft,step:'world',currentStep:0,previewFields:[],busy:false,pending:null,error:'',fieldErrors:{},groups:[],existingWorlds:[{id:'world',name:'日常世界',fluctlightId:'A',characterName:'雨禾'}],worlds:[],templates:[],worldsConfirmed:true,canContinue:false,loadExistingWorlds:async()=>rereads++,goStep(){},configHref:()=> '#/settings'};
 const v=await mountReact(vite,Wizard,{props:{model:m}});
 try{assert.equal(v.host.querySelector('.replica-eyebrow').textContent,'新的相识');assert.doesNotMatch(v.host.textContent,/A NEW BEGINNING/);const retry=button(v.host,'重新读取已有世界');assert.ok(retry.classList.contains('creation-source-action'));await v.click(retry);assert.equal(rereads,1);}finally{await v.dispose();}
});

test('P2-6 studio empty state uses shared leaf and its create action remains usable',async()=>{
 const {default:Studio}=await load('react-app/pages/StudioDirectory.jsx'),w=workspace();let calls=0;w.client.listStudioResources=async()=>({items:[],next_cursor:''});w.services.studioClient={};w.navigatePath=()=>calls++;
 const v=await mountReact(vite,Studio,{workspace:w});
 try{const empty=v.host.querySelector('[data-slot=empty]');assert.ok(empty.querySelector('.lucide-leaf'));assert.equal(empty.dataset.emptyVariant,'page');await v.click(button(empty,'新建资源'));assert.equal(calls,1);}finally{await v.dispose();}
});

test('P2-7 chat moment retains readable caption text in the scene overlay',async()=>{
 const {default:Chat}=await load('react-app/pages/ChatPage.tsx'),w=workspace(),s=createConversationController();s.fluctlightId='A';s.conversation={id:'C'};
 Object.assign(w,{conversation:()=>s,openConversation:async()=>{},detail:async()=>{},loadWardrobe:async()=>{}});
 const v=await mountReact(vite,Chat,{workspace:w,props:{preview:true,onGo(){}}});
 try{const caption=v.host.querySelector('.replica-moment-card .companion-scene-caption');assert.equal(caption.querySelector('strong').textContent,'把时间，留给喜欢的事。');assert.equal(caption.closest('[aria-hidden=true]'),null);}finally{await v.dispose();}
});

test('P2-8 unframed file picker keeps selection behavior and adds no nested drop border',async()=>{
 const {default:FilePicker}=await load('react-app/shared/FilePicker.tsx');let changed='';const v=await mountReact(vite,FilePicker,{props:{framed:false,accept:'.json',onChange:e=>changed=e.target.files[0].name}});
 try{const picker=v.host.firstElementChild;assert.equal(picker.classList.contains('border'),false);assert.equal(picker.querySelector('button').dataset.variant,'ghost');const input=picker.querySelector('input');Object.defineProperty(input,'files',{value:[new window.File(['{}'],'settings.json',{type:'application/json'})]});await act(async()=>input.dispatchEvent(new window.Event('change',{bubbles:true})));assert.equal(changed,'settings.json');assert.match(picker.textContent,/settings.json/);}finally{await v.dispose();}
});

test('P2-10 pending route chunks show shadcn skeletons until the page resolves',async()=>{
 const {default:Skeleton}=await load('react-app/shared/PageSkeleton.tsx'),pending=deferred(),Lazy=React.lazy(()=>pending.promise);
 function Screen(){return React.createElement(React.Suspense,{fallback:React.createElement(Skeleton)},React.createElement(Lazy));}
 const v=await mountReact(vite,Screen);
 try{assert.ok(v.host.querySelector('[data-testid=page-skeleton]'));assert.ok(v.host.querySelectorAll('[data-slot=skeleton]').length>=10);assert.equal(v.host.querySelectorAll('[role=status]').length,1);await act(async()=>pending.resolve({default:()=>React.createElement('h1',null,'日程')}));assert.equal(v.host.querySelector('[data-testid=page-skeleton]'),null);assert.equal(v.host.textContent,'日程');}finally{await v.dispose();}
});


test('P2-6 relations directory retains the shared leaf when no world people exist',async()=>{
 const {default:People}=await load('react-app/views/PeoplePage.jsx');let created=false;
 const source={items:[],ready:true,busy:false,error:'',extra:{},load:async()=>{}},m={catalog:source,profile:source,tab:'people',search:'',filter:'all',actorId:'owner',owner:'owner',fid:'A',trigger:{current:null},setSearch(){},setFilter(){},setTab(){},refresh(){},setDefinitionOpen:value=>created=value};
 const v=await mountReact(vite,People,{props:{model:m}});
 try{const empty=v.host.querySelector('[data-slot=empty]');assert.ok(empty.querySelector('.lucide-leaf'));assert.equal(empty.dataset.emptyVariant,'page');await v.click(button(v.host,'新增世界人物'));assert.equal(created,true);}finally{await v.dispose();}
});

test('D7 world clock waiting state shows the finished hour and next scheduled cycle',async()=>{
 const {default:Diagnostics}=await load('react-app/shared/DiagnosticsWorkspace.jsx'),w=workspace();
 w.services.diagnosticsClient={page:async()=>({items:[{ref:'clock',occurred_at:'2026-11-02T08:13:00Z',data:{intent_id:'world-clock',kind:'world.events.generate',status:'waiting',last_cycle_status:'failed',next_cycle_at:'2026-11-02T09:00:00Z'}}],next_cursor:''})};
 const v=await mountReact(vite,Diagnostics,{workspace:w,props:{view:{view:'tasks',openTaskDiagnostics(){}}}});
 try{const row=v.host.querySelector('.replica-task-row');assert.match(row.textContent,/等待下一小时/);assert.match(row.textContent,/上一轮：需要处理/);assert.match(row.textContent,/下一轮：.*17:00/);assert.equal(row.querySelector('.animate-spin'),null);}finally{await v.dispose();}
});
