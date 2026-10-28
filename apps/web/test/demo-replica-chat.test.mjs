import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {React,act,installDOM,mountReact} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const {default:History}=await vite.ssrLoadModule('/src/react-app/shared/ConversationHistory.tsx');
const time='2026-10-27T02:00:00Z',row=(id,text)=>({id,conversationId:id,kind:'assistant',text,createdAt:time}),session=id=>({conversation:{id,title:'会话 '+id,createdAt:time},unreadCount:0});
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent===text);
function workspace(){return {state:{session:{actorId:'owner'}},isCurrent:()=>true,current:{value:{id:'role',name:'林青'}},currentScope:()=>JSON.stringify(['owner','role']),client:{conversationDirectory:async()=>({items:[session('A'),session('B')],currentConversationId:'A',nextCursor:''}),messages:async id=>({messages:[row(id,'正文 '+id)],nextBeforeSequence:null}),searchConversationMessages:async()=>({items:[],nextCursor:''}),selectDirectConversation:async()=>{}},openConversation:async()=>{}};}
test('history browsing discards late session reads and closing never selects a conversation',async()=>{
 const w=workspace();let finish,selected=0,closed=0;w.client.messages=id=>id==='B'?new Promise(resolve=>{finish=resolve;}):Promise.resolve({messages:[row(id,'正文 '+id)]});w.client.selectDirectConversation=async()=>selected++;
 const v=await mountReact(vite,History,{workspace:w,props:{onClose:()=>closed++}});
 try{
  await v.flush();const dialog=document.querySelector('[role=dialog]');assert.match(dialog.textContent,/正文 A/);
  await v.click([...dialog.querySelectorAll('.companion-history-list button')].find(node=>node.textContent.startsWith('会话 B')));
  await v.click([...dialog.querySelectorAll('.companion-history-list button')].find(node=>node.textContent.startsWith('会话 A')));
  await act(async()=>finish({messages:[row('late','不应出现的旧结果')]}));await v.flush();assert.match(dialog.textContent,/正文 A/);assert.doesNotMatch(dialog.textContent,/不应出现/);assert.equal(selected,0);
  await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));assert.equal(closed,1);assert.equal(selected,0);
 }finally{await v.dispose();}
});
test('history retries the exact failed search cursor while retaining previous rows and newer unsubmitted query',async()=>{
 const w=workspace(),calls=[];let failed=false;
 w.client.searchConversationMessages=async(id,q,options)=>{calls.push({id,q,...options});if(options.cursor&&!failed){failed=true;throw Error('历史读取失败');}return {items:[row(options.cursor?'older':'newer',options.cursor?'更早的匹配':'原来的匹配')],nextCursor:options.cursor?'':'cursor-original'};};
 const v=await mountReact(vite,History,{workspace:w,props:{onClose(){}}});
 try{
  const dialog=document.querySelector('[role=dialog]'),input=dialog.querySelector('input');await v.input(input,'原搜索');await v.click(button(dialog,'搜索'));await v.click(button(dialog,'加载更多消息'));assert.match(dialog.textContent,/原来的匹配/);assert.match(dialog.textContent,/历史读取失败/);
  await v.input(input,'尚未提交的新搜索');await v.click(button(dialog,'重新读取历史'));assert.deepEqual(calls[2],calls[1]);assert.equal(calls[2].cursor,'cursor-original');assert.equal(calls[2].q,'原搜索');assert.equal(input.value,'尚未提交的新搜索');assert.match(dialog.textContent,/原来的匹配/);assert.match(dialog.textContent,/更早的匹配/);
 }finally{await v.dispose();}
});
test('history directory failure recovers the directory and only explicit resume switches a real session',async()=>{
 const w=workspace();let reads=0,switches=[],closed=0;w.client.conversationDirectory=async()=>{if(++reads===1)throw Error('目录读取失败');return {items:[session('A')],currentConversationId:'A',nextCursor:''};};w.client.selectDirectConversation=async(fid,id)=>switches.push([fid,id]);
 const v=await mountReact(vite,History,{workspace:w,props:{onClose:()=>closed++}});
 try{const dialog=document.querySelector('[role=dialog]');assert.match(dialog.textContent,/目录读取失败/);await v.click(button(dialog,'重新读取历史'));assert.equal(reads,2);assert.match(dialog.textContent,/正文 A/);assert.equal(switches.length,0);await v.click(button(dialog,'继续这段对话'));assert.deepEqual(switches,[['role','A']]);assert.equal(closed,1);}finally{await v.dispose();}
});
test('history cannot search before the initial directory completes and cannot dismiss an in-flight selection',async()=>{
 const w=workspace();let finishDirectory,finishSelection,closed=0,opened=0,searches=0;
 w.client.conversationDirectory=()=>new Promise(resolve=>{finishDirectory=resolve;});w.client.selectDirectConversation=()=>new Promise(resolve=>{finishSelection=resolve;});w.client.searchConversationMessages=async()=>{searches++;return {items:[],nextCursor:''};};w.openConversation=async()=>opened++;
 const v=await mountReact(vite,History,{workspace:w,props:{onClose:()=>closed++}});
 try{let dialog=document.querySelector('[role=dialog]');assert.equal(button(dialog,'搜索').disabled,true);await v.input(dialog.querySelector('input'),'预先输入');await act(async()=>dialog.querySelector('form').dispatchEvent(new window.Event('submit',{bubbles:true,cancelable:true})));assert.equal(searches,0);await act(async()=>finishDirectory({items:[session('A')],currentConversationId:'A',nextCursor:''}));await v.flush();await v.click(button(dialog,'继续这段对话'));assert.equal(button(dialog,'关闭'),undefined);await act(async()=>document.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));assert.equal(closed,0);assert.ok(document.querySelector('[role=dialog]'));await act(async()=>finishSelection({}));await v.flush();assert.equal(opened,1);assert.equal(closed,1);}finally{await v.dispose();}
});
test('a confirmed selection still reconciles the active Owner and role after its history view unmounts',async()=>{
 const w=workspace();let finish,opened=0;w.client.selectDirectConversation=()=>new Promise(resolve=>finish=resolve);w.openConversation=async()=>opened++;const v=await mountReact(vite,History,{workspace:w,props:{onClose(){}}});await v.click(button(document.querySelector('[role=dialog]'),'继续这段对话'));await v.dispose();await act(async()=>finish({}));assert.equal(opened,1);
});

test('source messages read the exact original anchor, retry the same read and never resume another conversation',async()=>{
 const w=workspace(),calls=[];let fail=true;
 w.client.conversationMessage=async(id,message)=>{calls.push(['anchor',id,message]);if(fail){fail=false;throw Error('source unavailable');}return {...row(message,'原交流正文'),conversationId:id,sequence:41};};
 w.client.messages=async(id,before)=>{calls.push(['messages',id,before]);return {messages:[{...row('previous','此前的原交流'),conversationId:id,sequence:40}],nextBeforeSequence:40};};
 w.client.selectDirectConversation=async()=>assert.fail('source reading must not resume');
 const v=await mountReact(vite,History,{workspace:w,props:{onClose(){},initialConversationId:'B',initialMessageId:'anchor'}});
 try{const d=document.querySelector('[role=dialog]');assert.match(d.textContent,/暂时/);await v.input(d.querySelector('input'),'尚未提交的搜索');await v.click(button(d,'重新读取历史'));assert.deepEqual(calls,[['anchor','B','anchor'],['anchor','B','anchor'],['messages','B',42]]);assert.equal(d.querySelector('input').value,'尚未提交的搜索');assert.match(d.querySelector('[aria-label="来源消息"]').textContent,/原交流正文/);assert.match(d.textContent,/此前的原交流/);}finally{await v.dispose();}
});

test('a source outside the first directory page waits for its own role membership and rejects a foreign anchor',async()=>{
 const w=workspace(),calls=[];
 w.client.conversationDirectory=async(fid,options)=>({items:[session(options.cursor?'B':'A')],currentConversationId:'A',nextCursor:options.cursor?'':'next-directory'});
 w.client.conversationMessage=async(id,message)=>{calls.push([id,message]);return {...row(message,'不能展示的外来正文'),conversationId:'foreign',sequence:1};};
 w.client.messages=async()=>assert.fail('foreign anchor must not read a message page');
 const v=await mountReact(vite,History,{workspace:w,props:{onClose(){},initialConversationId:'B',initialMessageId:'anchor'}});
 try{const d=document.querySelector('[role=dialog]');assert.equal(calls.length,0);assert.match(d.textContent,/来源会话尚未/);await v.click(button(d,'加载更多会话'));assert.deepEqual(calls,[['B','anchor']]);assert.match(d.textContent,/来源消息与原会话不匹配/);assert.doesNotMatch(d.textContent,/不能展示的外来正文/);}finally{await v.dispose();}
});
