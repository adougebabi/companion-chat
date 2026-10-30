import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,mountReact,installDOM} from './helpers/react-dom.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const load=path=>vite.ssrLoadModule('/src/react-app/shared/'+path),button=(host,label)=>[...host.querySelectorAll('button')].find(n=>n.textContent.trim()===label);
let ownerSequence=0;
function workspace({configuration={endpoints:[],bindings:[],effective_values:{}},tasks=[]}={}){
 const state=proxy({session:{actorId:'diagnostic-owner-'+(++ownerSequence)},characters:[{id:'role-real',name:'真实角色名'}],runtimeCatalog:{versions:{revision:1}}}),reads=[],tests=[];
 return {state,reads,tests,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(path){this.navigated=path;},services:{configurationClient:{read:async()=>{reads.push('configuration');return configuration;}},operationsClient:{health:async()=>{reads.push('health');return {status:'ready'};}},diagnosticsClient:{page:async(kind,options)=>{reads.push([kind,options]);return {items:tasks,next_cursor:'opaque/+?cursor'};}}}};
}
const view={bindings:{value:[{},{},{},{}]},testResult:{value:'stale global success'},testRaw:{value:{status:'ok'}},testKind:{value:'success'},openTaskDiagnostics(){},test:async()=>({status:'reachable',generation_tested:false})};
test('storage probe cleans only exact random probe keys and reports access/write/read/cleanup failures',async()=>{
 const {probeBrowserStorage}=await load('ServiceDiagnostics.tsx');
 const entries=new Map([['original-draft','keep']]),removed=[],storage={setItem:(k,v)=>entries.set(k,v),getItem:k=>entries.get(k),removeItem:k=>{removed.push(k);entries.delete(k);}};
 assert.deepEqual(probeBrowserStorage({localStorage:storage,sessionStorage:storage}).map(r=>r.status),['ok','ok']);assert.deepEqual([...entries],[['original-draft','keep']]);assert.equal(removed.length,2);assert.notEqual(removed[0],removed[1]);assert.ok(removed.every(k=>k.startsWith('fluctlight:diagnostic-probe:')));
 const blocked={get localStorage(){throw Error('getter blocked');},sessionStorage:{setItem(){throw Error('write blocked');},removeItem:k=>removed.push(k)}};
 assert.deepEqual(probeBrowserStorage(blocked).map(r=>r.reason),['getter blocked','write blocked']);assert.equal(removed.length,3);
 assert.match(probeBrowserStorage({localStorage:{...storage,getItem:()=>null},sessionStorage:storage})[0].reason,/未能读取/);
});
test('health reads saved zero bindings independently of editable slots; refresh never invokes explicit tests',async()=>{
 const {default:Health}=await load('ServiceDiagnostics.tsx'),w=workspace();let post=0;
 const v=await mountReact(vite,Health,{workspace:w,props:{view:{...view,test:async()=>{post++;return {status:'ok'};}}}});
 try{assert.match(v.host.textContent,/已保存 0 个服务地址、0 个模型用途绑定/);assert.match(v.host.textContent,/未配置/);assert.doesNotMatch(v.host.textContent,/stale global success|4 个模型用途/);assert.equal(v.host.querySelectorAll('.replica-service-row').length,3);await v.click(button(v.host,'重新检查'));assert.deepEqual(w.reads,['configuration','health','configuration','health']);assert.equal(post,0);}finally{await v.dispose();}
});
test('configured media stays untested; explicit reachability never claims generation success',async()=>{
 const {default:Health}=await load('ServiceDiagnostics.tsx'),w=workspace({configuration:{endpoints:[{id:'saved'}],bindings:[{role:'vision'}],effective_values:{'media.comfyui':{baseUrl:'http://saved.example'}}}}),targets=[];
 const v=await mountReact(vite,Health,{workspace:w,props:{view:{...view,test:async target=>{targets.push(target);return {status:'reachable',generation_tested:false};}}}});
 try{assert.equal(v.host.querySelectorAll('.replica-diagnostic-status[data-tone=green]').length,1);assert.match(v.host.textContent,/已配置 · 未测试/);await v.click(button(v.host,'显式连接测试与检查记录'));await v.click(button(v.host,'检查 ComfyUI 连接与工作流'));assert.deepEqual(targets,['图片（ComfyUI）']);assert.match(v.host.textContent,/尚未执行图片生成/);assert.doesNotMatch(v.host.textContent,/图片生成成功/);assert.equal(v.host.querySelectorAll('.replica-diagnostic-status[data-tone=green]').length,1);}finally{await v.dispose();}
});
test('late test receipt is ignored after Owner or saved configuration changes',async()=>{
 const {default:Health}=await load('ServiceDiagnostics.tsx'),w=workspace(),pending=[];
 const v=await mountReact(vite,Health,{workspace:w,props:{view:{...view,test:()=>new Promise(resolve=>pending.push(resolve))}}});
 try{await v.click(button(v.host,'显式连接测试与检查记录'));await v.click(button(v.host,'验证视觉模型图片输入'));await act(async()=>w.state.session.actorId='new-owner');await v.flush();await act(async()=>pending[0]({status:'failed',reason:'STALE OWNER TEST'}));await v.flush();assert.doesNotMatch(v.host.textContent,/STALE OWNER TEST/);await v.click(button(v.host,'验证视觉模型图片输入'));await act(async()=>w.state.runtimeCatalog.versions.revision=2);await v.flush();await act(async()=>pending[1]({status:'failed',reason:'STALE CONFIG TEST'}));await v.flush();assert.doesNotMatch(v.host.textContent,/STALE CONFIG TEST/);}finally{await v.dispose();}
});
test('task host defaults to tasks, searches resolved role names, preserves raw types and opaque cursor',async()=>{
 const {default:Host}=await load('DiagnosticsWorkspace.jsx'),w=workspace({tasks:[{ref:'first',data:{task_id:'t1',title:'wardrobe.initialize',kind:'wardrobe.initialize',status:'failed',fluctlight_id:'role-real'}},{ref:'second',data:{task_id:'t2',title:'其他任务',kind:'actual.unknown-kind',status:'running'}}]});
 globalThis.location.hash='#/diag';
 const v=await mountReact(vite,Host,{workspace:w,props:{view}});
 try{assert.match(v.host.querySelector('[role=tab][data-state=active]').textContent,/任务中心2/);assert.equal(v.host.querySelectorAll('.replica-task-row').length,2);assert.equal(v.host.querySelectorAll('.replica-task-toolbar [role=combobox]').length,2);const input=v.host.querySelector('input[aria-label="搜索任务或角色"]');await act(async()=>{Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set.call(input,'真实角色名');input.dispatchEvent(new window.Event('input',{bubbles:true}));});await v.flush();assert.equal(v.host.querySelectorAll('.replica-task-row').length,1);assert.match(v.host.querySelector('.replica-task-row').textContent,/真实角色名/);await v.click(button(v.host,'读取更多任务'));assert.equal(w.reads.at(-1)[1].cursor,'opaque/+?cursor');assert.equal(v.host.querySelectorAll('.replica-task-row>svg').length,0);}finally{await v.dispose();}
});
test('task type and status controls filter actual raw kinds and clear filters restores rows',async()=>{
 const {default:Host}=await load('DiagnosticsWorkspace.jsx'),w=workspace({tasks:[{ref:'a',data:{kind:'actual.unknown-kind',title:'未知类型任务',status:'paused'}},{ref:'b',data:{kind:'wardrobe.initialize',title:'衣柜任务',status:'completed'}}]});
 const v=await mountReact(vite,Host,{workspace:w,props:{view}});
 const choose=async(label,text)=>{const trigger=v.host.querySelector(`[role=combobox][aria-label="${label}"]`);await act(async()=>trigger.dispatchEvent(new window.KeyboardEvent('keydown',{key:'ArrowDown',bubbles:true})));await v.flush();const option=[...document.querySelectorAll('[role=option]')].find(n=>n.textContent===text);assert.ok(option,`option ${text}`);await act(async()=>option.click());await v.flush();};
 try{await choose('任务类型','actual.unknown-kind');assert.equal(v.host.querySelectorAll('.replica-task-row').length,1);assert.match(v.host.textContent,/未知类型任务/);await choose('任务状态','已完成');assert.equal(v.host.querySelectorAll('.replica-task-row').length,0);await v.click(button(v.host,'清除筛选'));assert.equal(v.host.querySelectorAll('.replica-task-row').length,2);}finally{await v.dispose();}
});
test('failed readiness and configuration reads stay separate from browser storage and remain retryable',async()=>{
 const {default:Health}=await load('ServiceDiagnostics.tsx'),w=workspace();let fail=true;
 w.services.configurationClient.read=async()=>{if(fail)throw Error('saved configuration unavailable');return {bindings:[],endpoints:[],effective_values:{}};};
 w.services.operationsClient.health=async()=>{throw Error('core unavailable');};
 const v=await mountReact(vite,Health,{workspace:w,props:{view}});
 try{assert.match(v.host.textContent,/已保存配置读取失败/);assert.match(v.host.querySelector('.replica-service-row').textContent,/正常/);assert.match(v.host.textContent,/读取失败/);fail=false;await v.click(button(v.host,'重新读取配置'));assert.doesNotMatch(v.host.textContent,/已保存配置读取失败/);await v.click(button(v.host,'显式连接测试与检查记录'));assert.match(v.host.textContent,/core unavailable/);}finally{await v.dispose();}
});

test('a test receipt for another saved version stays visible as stale rather than marking the displayed configuration passed',async()=>{
 const {default:Health}=await load('ServiceDiagnostics.tsx'),configuration={endpoints:[],bindings:[],effective_values:{'media.comfyui':{baseUrl:'http://saved.example'}},versions:{'media.comfyui':1}},w=workspace({configuration});
 const v=await mountReact(vite,Health,{workspace:w,props:{view:{...view,test:async()=>({state:'reachable',generation_tested:false,config_version:2})}}});
 try{await v.click(button(v.host,'显式连接测试与检查记录'));await v.click(button(v.host,'检查 ComfyUI 连接与工作流'));assert.match(v.host.textContent,/配置已变化/);assert.match(v.host.textContent,/配置版本与当前页面不同/);assert.doesNotMatch(v.host.textContent,/已检查服务连接与工作流/);configuration.versions['media.comfyui']=2;await v.click(button(v.host,'重新检查'));assert.doesNotMatch(v.host.textContent,/配置版本与当前页面不同/);}finally{await v.dispose();}
});
