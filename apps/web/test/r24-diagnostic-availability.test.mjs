import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from './support/module-server.mjs';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {act,installDOM,mountReact} from './helpers/react-dom.mjs';
import {errorMessage} from '../src/v3/domain-presentation.mjs';
const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const raw='完整诊断读取未启用。请由服务管理员开启 FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ 后重启 API，再重新读取。';
const disabled=()=>Object.assign(new Error(raw),{code:'diagnostic_debug_read_disabled',status:403});
const forbidden=/FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ|diagnostic_debug_read_disabled|重启 API|secret-task/;
const metric=(v,id)=>v.host.querySelector(`[data-task-status-group=${id}] strong`).textContent;
let owner=0;
async function mount({page=async()=>{throw disabled();},pending=async()=>({groups:[{kind:'failed_task',status:'ok',items:[1,2,3].map(n=>({id:'workflow:secret-task-'+n,source_id:'secret-task-'+n,title:'goal.plan',status:'failed',href:'#/diag/task/secret-task-'+n}))}]})}={}){
 localStorage.clear();sessionStorage.clear();location.hash='#/diag?view=tasks';
 const state=proxy({session:{actorId:'r24-owner-'+owner++},characters:[]});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:{pending,task:async id=>({task_id:id,kind:'goal.plan',status:'failed'})},diagnosticsClient:{page}}};
 const {default:Host}=await vite.ssrLoadModule('/src/react-app/shared/DiagnosticsWorkspace.jsx');
 return mountReact(vite,Host,{workspace:w,props:{view:{openTaskDiagnostics(){}}}});
}
test('disabled diagnostic summary is safe for structured and legacy message-only responses; unrelated 403 keeps permission semantics',()=>{
 assert.equal(errorMessage(disabled()),'诊断读取未开启，需要管理员在服务端开启后再查看。');
 assert.equal(errorMessage({message:raw,status:403}),'诊断读取未开启，需要管理员在服务端开启后再查看。');
 assert.equal(errorMessage({status:403,message:'forbidden'}),'当前账户没有执行此操作的权限。');
 assert.match(errorMessage({message:'media_workflow_unclassified_lora_forbidden'}),/LoRA.*尚未提交渲染/);
 assert.match(errorMessage({message:'media_prompt_reference_forbidden'}),/参考图引用.*尚未提交渲染/);
});
test('three confirmed pending tasks count despite two debug source failures, with one error and download-only raw details',async()=>{
 const v=await mount();try{
  assert.equal(metric(v,'attention'),'3');assert.equal(metric(v,'running'),'暂不可用');
  assert.equal(v.host.querySelectorAll('.replica-task-row').length,3);
  assert.equal((v.host.textContent.match(/诊断读取未开启，需要管理员在服务端开启后再查看。/g)||[]).length,1);
  assert.match(v.host.textContent,/统计仅包含已读取的任务/);assert.match(v.host.textContent,/任务来源未能读到/);
  assert.doesNotMatch(v.host.textContent,forbidden);
  assert.equal(v.host.querySelectorAll('[data-feedback-kind=warning]').length,1);
  await v.click([...v.host.querySelectorAll('button')].find(n=>n.textContent.trim()==='原始错误记录（诊断）'));
  assert.doesNotMatch(v.host.textContent,forbidden);
  assert.equal(v.host.querySelector('pre'),null);
  const create=URL.createObjectURL,revoke=URL.revokeObjectURL,anchor=window.HTMLAnchorElement.prototype,click=anchor.click;
  let blob;URL.createObjectURL=value=>{blob=value;return 'blob:diagnostic-fixture';};URL.revokeObjectURL=()=>{};anchor.click=function(){assert.equal(this.download,'fluctlight-record.json');};
  try{
   await v.click([...v.host.querySelectorAll('button')].find(n=>n.textContent==='下载原始记录'));
   const failures=JSON.parse(await blob.text());
   assert.equal(failures.length,2);
   for(const failure of failures){assert.equal(failure.message,raw);assert.match(failure.stack,/FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ/);}
  }finally{URL.createObjectURL=create;URL.revokeObjectURL=revoke;anchor.click=click;}
 }finally{await v.dispose();}
});
test('all unreadable sources never claim zero or empty; grouped retry refreshes every failed source',async()=>{
 let fail=true;const reads=[];
 const v=await mount({page:async(_kind,options)=>{reads.push(options);if(fail)throw disabled();return {items:[],next_cursor:''};},pending:async()=>{if(fail)throw disabled();return {groups:[{kind:'failed_task',status:'ok',items:[]}]};}});
 try{
  assert.deepEqual([...v.host.querySelectorAll('.replica-diagnostic-metric strong')].map(n=>n.textContent),Array(6).fill('暂不可用'));
  assert.doesNotMatch(v.host.textContent,/暂时没有运行任务|—/);assert.doesNotMatch(v.host.textContent,forbidden);
  assert.equal((v.host.textContent.match(/诊断读取未开启，需要管理员在服务端开启后再查看。/g)||[]).length,1);
  fail=false;await v.click([...v.host.querySelectorAll('button')].find(n=>n.textContent==='重新读取失败的任务来源'));
  assert.equal(reads.length,4);assert.deepEqual([...v.host.querySelectorAll('.replica-diagnostic-metric strong')].map(n=>n.textContent),Array(6).fill('0'));
  assert.match(v.host.textContent,/暂时没有运行任务/);assert.doesNotMatch(v.host.textContent,/诊断读取未开启/);
 }finally{await v.dispose();}
});
test('confirmed recent rows stay counted when pending authority fails, and pagination failure retains counts',async()=>{
 let more=false;
 const v=await mount({page:async(_kind,options)=>{if(options.intentType)return {items:[],next_cursor:''};if(options.cursor){more=true;throw disabled();}return {items:[{ref:'secret-task',data:{task_id:'secret-task',kind:'goal.plan',status:'running'}}],next_cursor:'next'};},pending:async()=>{throw Error('fetch failed');}});
 try{assert.equal(metric(v,'running'),'1');await v.click([...v.host.querySelectorAll('button')].find(n=>n.textContent==='读取更多任务'));assert.equal(more,true);assert.equal(metric(v,'running'),'1');assert.equal(v.host.querySelectorAll('.replica-task-row').length,1);assert.doesNotMatch(v.host.textContent,forbidden);}finally{await v.dispose();}
});
test('events route owns one failure; downloads raw service errors and retry clears the failure',async()=>{
 const {configureDomainService}=await vite.ssrLoadModule('/src/v3/proto/services.js');
 const {get,replaceSnapshot}=await vite.ssrLoadModule('/src/v3/proto/store.js');
 const {renderRealDiagnosticRecords}=await vite.ssrLoadModule('/src/v3-domain/diagnostics.js');
 replaceSnapshot({...get(),session:{actorId:'r24-events'}});let fail=true;
 configureDomainService('diagnostics',{page:async()=>{if(fail)throw disabled();return {items:[],next_cursor:''};}});
 const host=document.createElement('div');document.body.append(host);
 const dispose=renderRealDiagnosticRecords(host,{def:{},params:{kind:'events'},query:{}});
 const flush=()=>new Promise(resolve=>setTimeout(resolve,30));
 try{
  await flush();assert.equal((host.textContent.match(/诊断读取未开启，需要管理员在服务端开启后再查看。/g)||[]).length,1);
  assert.equal(host.querySelector('[data-feedback]').textContent,'');assert.equal(host.querySelectorAll('[role=alert]').length,1);
  assert.doesNotMatch(host.textContent,forbidden);assert.equal(host.querySelector('[data-panel-failure] details, [data-panel-failure] pre'),null);
  const download=host.querySelector('[data-panel-failure] a[download]');
  assert.equal(download.textContent,'原始错误记录（诊断）');
  const record=JSON.parse(decodeURIComponent(download.getAttribute('href').split(',')[1]));
  assert.equal(record.message,raw);assert.match(record.stack,/FLUCTLIGHT_DIAGNOSTIC_DEBUG_READ/);
  fail=false;host.querySelector('[data-panel-retry]').click();await flush();assert.equal(host.querySelector('[data-panel-failure]'),null);
 }finally{dispose?.();host.remove();}
});
