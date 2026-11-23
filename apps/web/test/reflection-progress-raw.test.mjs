import assert from 'node:assert/strict';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import {createServer} from 'vite';
import {act, installDOM, mountReact} from './helpers/react-dom.mjs';

const environment=installDOM();
const vite=await createServer({
  root:fileURLToPath(new URL('../',import.meta.url)),
  envFile:false,
  appType:'custom',
  logLevel:'silent',
  server:{middlewareMode:true,ws:false},
});
test.after(async()=>{await vite.close();environment.restore();});

const {featureRoute}=await vite.ssrLoadModule('/src/approved-ui/feature-routes.mjs');
const {configureDomainService}=await vite.ssrLoadModule('/src/v3/proto/services.js');

const character='fluctlight_fixture';
const intent='reflection_intent:daily:autonomy_private';
const started='2026-11-20T03:32:00Z';
const completed='2026-11-20T03:34:00Z';
const next='2026-11-23T02:15:00Z';
const latestTask={
  intent_id:intent, workflow_id:'reflection:daily:autonomy_private',
  status:'retry_wait', attempt_count:2, started_at:started, completed_at:completed,
  next_attempt_at:next, error_code:'reflection_task_failed', created_at:'2026-11-20T03:31:00Z',
  source_ref:'private-source-ref', idempotency_key:'private-replay', metrics:{internal_count:9021},
  future_data:[false,0,null,'中文<&"\''],
};
const inputWindow={
  status:'idle', watermark:9022, state_revision:9023, updated_at:completed,
  evolution_revisions:{active_memory:9024, affect_profile:null},
};

const forbidden=/\b(?:id|attempt[ _]count|started[ _]at|completed[ _]at|error[ _]code|intent[ _]id|workflow[ _]id|source[ _]ref|idempotency|metrics|watermark|revision|evolution[ _]revisions|active[ _]memory|affect[ _]profile|902[1-4])\b|autonomy_private|reflection_intent|reflection_task_failed|private-(?:source|replay|model)|\d{4}-\d\d-\d\dT\d\d:\d\d|版本|尚未记录|完整任务状态/i;

function checkDownload(link, expected) {
  assert.ok(link);
  assert.equal(link.download,'fluctlight-record.json');
  const prefix='data:application/json;charset=utf-8,';
  const href=link.getAttribute('href');
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))),expected);
}

async function mountTimeline(t, progress) {
  const env=installDOM();
  let dispose;
  t.after(()=>{dispose?.();env.restore();});
  const route=featureRoute('/c/'+character+'/growth/reflection',new URLSearchParams());
  const catalog={items:[{id:'reflection_result_fixture',title:'一次关于倾听的反思',summary:'她想先听完对方的想法。',status:'applied',at:started}],next_cursor:'',...progress};
  const listCalls=[];
  configureDomainService('reflection',{
    list:async(...args)=>{listCalls.push(args);return catalog;},
  });
  const host=document.querySelector('#react-test-root');
  dispose=(await route.load())(host,{charId:character});
  await new Promise(resolve=>setImmediate(resolve));
  assert.deepEqual(listCalls,[[character,'']]);
  return host;
}

test('反思时间线展示中文进度，原始任务与输入窗口仅下载且保持原样',async t=>{
  const raw={latest_task:latestTask,window:inputWindow};
  const host=await mountTimeline(t,raw);
  const disclosure=host.querySelector('details[data-progress]');
  assert.equal(disclosure.open,true);
  assert.equal(disclosure.querySelector('details, pre, code'),null);
  assert.doesNotMatch(host.textContent,forbidden);
  for(const expected of [
    '一次关于倾听的反思','她想先听完对方的想法。','反思正在等待重试','第 2 次尝试',
    '开始时间2026-11-20 11:32 (北京时间)','完成时间2026-11-20 11:34 (北京时间)',
    '已安排重试：2026-11-23 10:15 (北京时间)','失败原因反思处理失败',
    '反思输入范围：等待新的经历',
  ])assert.ok(host.textContent.includes(expected),expected);
  const links=disclosure.querySelectorAll('a[download]');
  assert.equal(links.length,1);
  assert.equal(links[0].textContent,'原始反思任务记录（诊断）');
  checkDownload(links[0],raw);
  const original=[...disclosure.querySelectorAll('a')].find(link=>link.textContent==='查看原运行记录');
  const originalRoute=featureRoute(original.hash.slice(1),new URLSearchParams());
  assert.equal(originalRoute.render,'TaskDetailPage');
  assert.equal(originalRoute.params.taskId,intent);
});

test('反思进度省略空字段和无效时间，保留旧任务原值',async t=>{
  const raw={latest_task:{status:'completed',attempt_count:null,started_at:'',completed_at:'invalid',next_attempt_at:next,error_code:null},window:null};
  const host=await mountTimeline(t,raw);
  const progress=host.querySelector('[data-reflection-progress]');
  assert.match(progress.textContent,/已完成/);
  assert.doesNotMatch(progress.textContent,/尚未记录|未记录|时间未知|开始时间|完成时间|已安排重试|第 .* 次尝试|失败原因|反思输入范围|查看原运行记录/);
  assert.equal(progress.querySelector('dl, p'),null);
  checkDownload(progress.querySelector('a[download]'),raw);
  const {ReflectionProgress}=await vite.ssrLoadModule('/src/v3/proto/components/reflection-progress.js');
  progress.innerHTML=ReflectionProgress({latest_task:null,window:{}});
  assert.match(progress.textContent,/还没有反思任务记录/);
  assert.equal(progress.querySelector('a[download]'),null);
  progress.innerHTML=ReflectionProgress({latest_task:null,window:inputWindow});
  assert.match(progress.textContent,/反思输入范围：等待新的经历/);
  checkDownload(progress.querySelector('a[download]'),{latest_task:null,window:inputWindow});
});

test('原反思运行记录展开后保留执行摘要和完整诊断下载',async()=>{
  const route=featureRoute('/diag/task/'+encodeURIComponent(intent),new URLSearchParams());
  const task={...latestTask,task_id:intent,kind:'reflection.run',status:'failed',runs:2,correlation_id:'private-model-correlation',steps:[]};
  const snapshot={intent:{intent_type:task.kind,status:task.status,payload:{intent_id:intent}},runtime_state:'available',runtime:{run_id:'private-run',status:'failed'},allowed_actions:[],commands:[]};
  const record={ref:'private-model-ref',data:{id:'private-model-id',status:'failed',sequence:1,started_at:started,prompt:inputWindow,response:{...latestTask},error_code:'request_timeout'}};
  const state={session:{actorId:'fixture'},characters:[]};
  const taskReads=[];
  const workspace={
    state,subscribe:()=>()=>{},getSnapshot:()=>state,navigatePath(){},
    services:{
      operationsClient:{task:async id=>{taskReads.push(id);return task;}},
      diagnosticsClient:{workflow:async()=>snapshot,page:async()=>({items:[record],next_cursor:''})},
    },
  };
  const view=await mountReact(vite,await route.load(),{workspace,props:{route:{feature:route,query:new URLSearchParams()}}});
  const {host}=view;
  const button=title=>[...host.querySelectorAll('button')].find(node=>node.textContent.trim()===title);
  const tab=async title=>{
    const trigger=[...host.querySelectorAll('[role=tab]')].find(node=>node.textContent.trim()===title);
    assert.ok(trigger,title);
    await act(async()=>trigger.dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));
    await view.flush();
  };
  const blobs=[];
  const create=URL.createObjectURL,revoke=URL.revokeObjectURL;
  const anchor=window.HTMLAnchorElement.prototype,click=anchor.click;
  URL.createObjectURL=blob=>{blobs.push(blob);return 'blob:reflection-fixture';};
  URL.revokeObjectURL=()=>{};
  anchor.click=function(){assert.equal(this.download,'fluctlight-record.json');};
  const checkRawDownload=async expected=>{
    await view.click(button('下载原始记录'));
    assert.deepEqual(JSON.parse(await blobs.at(-1).text()),expected);
  };
  const checkSummary=()=>{
    assert.doesNotMatch(host.textContent,/private-run|request_timeout|未记录输入|未记录输出|未记录错误|错误代码/);
    assert.doesNotMatch(host.textContent,forbidden);
    assert.equal(host.querySelector('pre, code'),null);
  };
  try{
    assert.deepEqual(taskReads,[intent]);
    assert.match(host.textContent,/回顾与自我认识/);
    await tab('执行记录');
    assert.ok(button('模型调用记录 · 1 次'),host.textContent);
    await view.click(button('模型调用记录 · 1 次'));
    await view.click(host.querySelector('.replica-task-round-trigger'));
    assert.match(host.textContent,/第 1 轮/);
    assert.match(host.textContent,/模型服务响应超时/);
    await view.click(button('本轮原始模型调用（诊断）'));
    checkSummary();
    await checkRawDownload(record);

    await tab('任务信息');
    await view.click(button('查看完整任务记录'));
    await view.click(button('原始反思任务记录（诊断）'));
    assert.match(host.textContent,/2026-11-20 11:32:00\.000（北京时间）/);
    checkSummary();
    await checkRawDownload(task);
    await view.click(button('原始反思任务记录（诊断）'));
    await view.click(button('管理命令与技术详情'));
    await view.click(button('完整恢复记录'));
    checkSummary();
    await checkRawDownload(snapshot);
  }finally{
    URL.createObjectURL=create;
    URL.revokeObjectURL=revoke;
    anchor.click=click;
    await view.dispose();
  }
});
