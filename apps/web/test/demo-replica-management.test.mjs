import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {installDOM,mountReact,React,act} from './helpers/react-dom.mjs';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path),button=(host,text)=>[...host.querySelectorAll('button')].find(n=>n.textContent.trim()===text);
test.after(async()=>{await vite.close();env.restore();});
test('configuration categories preserve a newer draft while the actual services and four roles remain independently reachable',async()=>{
 const {default:Management}=await load('react-app/pages/ManagementPage.jsx');const state=proxy({session:{actorId:'management-owner'},managementTab:'配置中心',characters:[],operations:[],runtimeCatalog:{}}),writes=[];
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),services:{configurationClient:{read:async()=>({effective_values:{},versions:{},endpoints:[{id:'S',kind:'openai-compatible',base_url:'https://example.test/v1'}],bindings:[]}),models:async()=>({models:[]}),catalog:async()=>({agents:[]}),endpoint:async b=>{writes.push(b);return {};},command:async()=>({applied_versions:{}})},diagnosticsClient:{page:async()=>({items:[],next_cursor:''})}},notify(){},navigatePath(){}};
 const v=await mountReact(vite,Management,{workspace:w});try{const input=v.host.querySelector('#config-服务地址 input[type=url]');await v.input(input,'https://example.test/draft');await v.click(button(v.host,'聊天与视觉'));assert.equal(v.host.querySelector('#config-服务地址').hidden,true);assert.equal(v.host.querySelector('#config-聊天回复').hidden,false);assert.equal(v.host.querySelector('#config-看图理解').hidden,false);await v.click(button(v.host,'模型服务与用途'));assert.equal(input.value,'https://example.test/draft');await act(async()=>[...v.host.querySelectorAll('[role=tab]')].find(n=>n.textContent.includes('用途分配')).dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await v.flush();assert.equal(v.host.querySelector('#config-模型用途').hidden,false);assert.match(v.host.querySelector('#config-模型用途').textContent,/记忆检索向量/);assert.equal(writes.length,0);await v.click(button(v.host,'放弃未保存修改'));assert.equal(v.host.querySelector('#config-服务地址 input[type=url]').value,'https://example.test/v1');}finally{await v.dispose();}
});
test('task records use source actions and never turn a status into numeric progress',async()=>{
 const {default:Task}=await load('react-app/pages/TaskDetailPage.tsx');const state=proxy({session:{actorId:'task-page-owner'}}),w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:{task:async()=>({task_id:'T',kind:'visual_identity.prepare',status:'running',steps:[]})}}};const v=await mountReact(vite,Task,{workspace:w,props:{route:{feature:{params:{taskId:'T'}},query:new URLSearchParams()}}});try{assert.equal(v.host.querySelector('[role=progressbar]'),null);assert.match(v.host.textContent,/暂未读到阶段记录/);assert.equal(button(v.host,'重试任务'),undefined);await act(async()=>button(v.host,'任务信息').dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));await v.flush();assert.match(v.host.textContent,/技术详情与实际任务信息/);}finally{await v.dispose();}
});
test('data validation refuses a changed input after the read and failed export preserves the exact raw draft',async()=>{
 const {configureDomainService}=await load('v3/proto/services.js'),store=await load('v3/proto/store.js'),{renderRealData}=await load('v3-domain/data-tools.js');store.replaceSnapshot({...store.get(),session:{actorId:'data-test-owner'},characters:{A:{id:'A'}},groups:{}});let finish;configureDomainService('dataTools',{backups:async()=>({items:[],next_cursor:''}),validatePack:()=>new Promise(r=>finish=r),exportPack:async()=>{throw Error('export unavailable');}});const host=document.createElement('div');document.body.append(host);const dispose=renderRealData(host,{navigate(){}});try{const field=host.querySelector('[data-input]');field.value='{"unknown":false,"number":0}';field.dispatchEvent(new window.Event('input',{bubbles:true}));const pending=host.querySelector('[data-validate]').onclick({target:host.querySelector('[data-validate]')});field.value='{"newer":null}';field.dispatchEvent(new window.Event('input',{bubbles:true}));finish({entries:[],external_bindings:[],publish_order:[],digest:'original'});await pending;assert.match(host.querySelector('[data-error]').textContent,/草稿已变化/);assert.equal(host.querySelector('[data-import]'),null);await host.querySelector('[data-export]').onclick({target:host.querySelector('[data-export]')});assert.equal(field.value,'{"newer":null}');assert.equal(host.querySelector('[data-export-result]').textContent,'');assert.match(host.querySelector('[data-error]').textContent,/服务暂时无法连接/);assert.match(host.textContent,/位已读取角色/);assert.equal(host.querySelector('[data-original]').hidden,true);}finally{dispose();host.remove();}
});

test('domain field blur records carrier selection without stealing pointer or Tab focus; explicit restoration still forwards',async()=>{
 const {default:Ports}=await load('react-app/shared/DomainPrimitives.tsx'),{Sheet,SheetContent,SheetTitle,SheetDescription}=await load('components/ui/sheet.tsx');
 function View(){const [root,setRoot]=React.useState(null);return React.createElement(Sheet,{open:true},React.createElement(SheetContent,null,React.createElement(SheetTitle,null,'双字段确认'),React.createElement(SheetDescription,null,'焦点同步回归'),React.createElement('div',{ref:setRoot,dangerouslySetInnerHTML:{__html:'<label>名称<input id="first" name="name" data-domain-ui="field" value="draft value"></label><label>理由<textarea id="second" name="reason" data-domain-ui="field"></textarea></label>'}}),React.createElement(Ports,{root})));}
 const v=await mountReact(vite,View);try{const source=document.querySelector('#first'),first=document.querySelector('#first--ui'),second=document.querySelector('#second--ui'),native=first.setSelectionRange.bind(first);let forwarded=0,related;source.addEventListener('focusout',event=>related=event.relatedTarget);first.setSelectionRange=(...args)=>{forwarded++;native(...args);first.focus();};await act(async()=>{first.focus();native(2,5);second.focus();});assert.equal(document.activeElement,second);assert.equal(source.selectionStart,2);assert.equal(source.selectionEnd,5);assert.equal(forwarded,0);assert.equal(related,second);await act(async()=>{source.focus();source.setSelectionRange(1,4);});assert.equal(document.activeElement,first);assert.equal(first.selectionStart,1);assert.equal(first.selectionEnd,4);assert.equal(forwarded,1);await act(async()=>{first.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Tab',bubbles:true}));second.focus();});assert.equal(document.activeElement,second);assert.equal(source.selectionStart,1);}finally{await v.dispose();}
});

test('a received durable command recovers failed readback after reopening without a second mutation',async()=>{
 const {default:OverlayHost}=await load('react-app/shared/OverlayHost.tsx'),{durableCommandDrawer}=await load('v3/proto/components/durable-command-drawer.js'),store=await load('v3/proto/store.js');
 store.replaceSnapshot({...store.get(),session:{actorId:'drawer-readback-owner'}});
 const state=proxy({session:{actorId:'drawer-readback-owner'}}),w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state)};
 const v=await mountReact(vite,OverlayHost,{workspace:w});let writes=0,reads=0,fail=true,d;
 const options={slot:'readback-proof',title:'原结果恢复',body:'<input name="reason" value="original">',read:el=>({reason:el.querySelector('input').value}),execute:async()=>{writes++;return {id:'confirmed'};},onSaved:async result=>{reads++;assert.equal(result.id,'confirmed');if(fail)throw Error('GET unavailable');}};
 try{
  await act(async()=>{d=durableCommandDrawer(options);});await v.flush();
  await act(async()=>{d.el.querySelector('[data-command-send]').click();});await v.flush();
  assert.match(d.el.textContent,/提交已有回执/);assert.equal(d.el.querySelector('[data-command-send]').disabled,true);
  await act(async()=>d.close());await v.flush();
  await act(async()=>{d=durableCommandDrawer(options);});await v.flush();
  assert.equal(d.el.querySelector('[data-command-send]').disabled,true);assert.match(d.el.querySelector('[data-command-retry]').textContent,/重新读取原结果/);
  d.el.querySelector('input').value='newer recovery reason';fail=false;await act(async()=>d.el.querySelector('[data-command-retry]').click());await v.flush();
  assert.equal(writes,1);assert.equal(reads,2);assert.ok(d.el.isConnected);assert.equal(d.el.querySelector('input').value,'newer recovery reason');assert.equal(sessionStorage.getItem('fluctlight-studio:drawer-readback-owner:readback-proof:command:accepted'),null);
 }finally{await v.dispose();}
});

test('a durable command checkpoints a late receipt after close, but never after logout and same-Owner re-entry',async()=>{
 const {default:OverlayHost}=await load('react-app/shared/OverlayHost.tsx'),{durableCommandDrawer}=await load('v3/proto/components/durable-command-drawer.js'),store=await load('v3/proto/store.js');
 const owner='drawer-late-owner';store.replaceSnapshot({...store.get(),session:{actorId:owner}});
 const state=proxy({session:{actorId:owner}}),w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state)},v=await mountReact(vite,OverlayHost,{workspace:w});
 let finish,d,writes=0,reads=0;const options={slot:'late-proof',title:'迟到回执',body:'<input name="reason" value="original">',read:()=>({reason:'original'}),execute:()=>{writes++;return new Promise(resolve=>finish=resolve);},onSaved:async()=>{reads++;}};
 const key=`fluctlight-studio:${owner}:late-proof:command`;
 try{
  await act(async()=>{d=durableCommandDrawer(options);});await v.flush();await act(async()=>{d.el.querySelector('[data-command-send]').click();});
  await act(async()=>d.close());await act(async()=>finish({id:'late'}));await v.flush();assert.ok(sessionStorage.getItem(key+':accepted'));assert.equal(reads,0);
  await act(async()=>{d=durableCommandDrawer(options);});await v.flush();await act(async()=>d.el.querySelector('[data-command-retry]').click());await v.flush();assert.equal(writes,1);assert.equal(reads,1);
  await act(async()=>{d=durableCommandDrawer(options);});await v.flush();await act(async()=>d.el.querySelector('[data-command-send]').click());
  await act(async()=>{store.replaceSnapshot({...store.get(),session:null});store.replaceSnapshot({...store.get(),session:{actorId:owner}});});
  const newer='{"version":1,"idempotencyKey":"new-session","value":{"reason":"new"},"attempts":1}';sessionStorage.setItem(key,newer);
  await act(async()=>finish({id:'obsolete'}));await v.flush();assert.equal(sessionStorage.getItem(key),newer);assert.equal(sessionStorage.getItem(key+':accepted'),null);assert.equal(reads,1);
 }finally{await v.dispose();}
});

test('a durable receipt readback retains input edited while submission is waiting',async()=>{
 const {default:OverlayHost}=await load('react-app/shared/OverlayHost.tsx'),{durableCommandDrawer}=await load('v3/proto/components/durable-command-drawer.js'),store=await load('v3/proto/store.js');
 const owner='drawer-newer-owner';store.replaceSnapshot({...store.get(),session:{actorId:owner}});const state=proxy({session:{actorId:owner}}),w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state)},v=await mountReact(vite,OverlayHost,{workspace:w});let finish,d;
 try{await act(async()=>{d=durableCommandDrawer({slot:'newer-proof',title:'保留新输入',body:'<input name="reason" value="original">',read:el=>({reason:el.querySelector('input').value}),execute:()=>new Promise(resolve=>finish=resolve),onSaved:async()=>{}});});await v.flush();await act(async()=>d.el.querySelector('[data-command-send]').click());d.el.querySelector('input').value='newer';await act(async()=>finish({id:'saved'}));await v.flush();assert.ok(d.el.isConnected);assert.equal(d.el.querySelector('input').value,'newer');assert.match(d.el.textContent,/新输入仍保留/);}finally{await v.dispose();}
});


test('logout clears configuration recovery and inactive management preferences for exactly the original Owner',async()=>{
 const store=await load('v3/proto/store.js');await load('v3/proto/components/studio-draft-storage.js');
 const owner='config-cleanup-owner';store.replaceSnapshot({...store.get(),session:{actorId:owner}});
 const keys=[`fluctlight:approved-config-command:${JSON.stringify([owner,'endpoint'])}`,`fluctlight:approved-policy:${JSON.stringify([owner,'A'])}`,`fluctlight:management-view:${owner}:inbox`];
 const foreign=`fluctlight:approved-policy:${JSON.stringify(['other-owner','A'])}`;
 for(const key of [...keys,foreign])sessionStorage.setItem(key,'original');
 store.replaceSnapshot({...store.get(),session:null});
 for(const key of keys)assert.equal(sessionStorage.getItem(key),null);assert.equal(sessionStorage.getItem(foreign),'original');
});

test('the native history deep route waits for its role directory, retries the original cursor and returns without losing the chat draft',async()=>{
 const {approvedRoute}=await load('approved-ui/routing.mjs'),{createConversationController}=await load('state/conversation.ts');
 const route=approvedRoute('#/c/A/chat/history?q=bookmark&returnTo=%23%2Fc%2FA%2Fchat%3Fmessage%3Danchor');assert.equal(route.feature.native,'chat-history');
 const Chat=await route.feature.load(),store=createConversationController();store.fluctlightId='A';store.conversation={id:'C'};store.messages=[];
 const state=proxy({session:{actorId:'history-route-owner'},selectedId:'A',characters:[{id:'A',name:'原角色',schedule:[],garments:[],wearing:{},_directory:{}}],errors:{},loading:{}});
 let finishDirectory,fail=true;const calls=[],moves=[];
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),current:{get value(){return state.characters[0];}},currentScope:()=>JSON.stringify([state.session.actorId,state.selectedId]),isCurrent:(owner,fid)=>owner===state.session.actorId&&fid===state.selectedId,conversation:()=>store,openConversation:async()=>{},clock:()=>'',detail:async()=>{},loadWardrobe:async()=>{},services:{mediaClient:{url:id=>id}},navigatePath:path=>moves.push(path),client:{conversationDirectory:fid=>{calls.push(['directory',fid]);return new Promise(resolve=>finishDirectory=resolve);},searchConversationMessages:async(fid,q,options)=>{calls.push(['search',fid,q,options]);if(options.cursor&&fail){fail=false;throw Error('append unavailable');}return {items:[{id:options.cursor?'older':'current',kind:'assistant',text:options.cursor?'older row':'original row'}],nextCursor:options.cursor?'':'frozen-cursor'};},selectDirectConversation:async()=>{throw Error('browsing must never select');}}};
 const props={preview:false,onGo(){},historyRoute:route};const v=await mountReact(vite,Chat,{workspace:w,props});
 try{
  assert.deepEqual(calls,[['directory','A']]);
  await act(async()=>finishDirectory({items:[{conversation:{id:'C',title:'原会话'},unreadCount:0}],currentConversationId:'C',nextCursor:''}));await v.flush();
  assert.equal(calls[1][1],'A');assert.equal(calls[1][2],'bookmark');const dialog=()=>document.querySelector('[role=dialog]');
  await v.click(button(dialog(),'加载更多消息'));assert.match(dialog().textContent,/original row/);assert.match(dialog().textContent,/服务暂时无法连接/);
  await v.input(dialog().querySelector('input'),'new unsubmitted query');await v.click(button(dialog(),'重新读取历史'));assert.deepEqual(calls[3],calls[2]);assert.equal(dialog().querySelector('input').value,'new unsubmitted query');assert.match(dialog().textContent,/older row/);
  await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));assert.equal(moves.at(-1),'/c/A/chat?message=anchor');
  await v.update({...props,historyRoute:undefined});const composer=v.host.querySelector('textarea[aria-label="聊天消息"]');await v.input(composer,'unsent role A draft');
  const next=approvedRoute('#/c/A/chat/history?q=second');await v.update({...props,historyRoute:next});await act(async()=>finishDirectory({items:[{conversation:{id:'C',title:'原会话'},unreadCount:0}],currentConversationId:'C',nextCursor:''}));await v.flush();assert.equal(calls.at(-1)[2],'second');
  await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));assert.equal(moves.at(-1),'/c/A/chat');await v.update({...props,historyRoute:undefined});assert.equal(v.host.querySelector('textarea[aria-label="聊天消息"]'),composer);assert.equal(composer.value,'unsent role A draft');
 }finally{await v.dispose();}
});
