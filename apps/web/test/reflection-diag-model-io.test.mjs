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
const metadata={id:'hidden-task-id',revision:8917,source_ref:'hidden-source-ref',idempotency_key:'hidden-replay-key',metrics:{hidden_count:8918},correlation_id:'hidden-correlation'};
const hidden=/\b(?:id|revision|source_ref|idempotency_key|metrics|correlation_id|8917|8918)\b|hidden-(?:task|source|replay|correlation|round)|版本|尚未记录/;

function button(scope,title){
  const matches=[...scope.querySelectorAll('button')].filter(node=>node.textContent.trim()===title);
  assert.equal(matches.length,1,title);
  return matches[0];
}

test('反思诊断每轮展示可读模型输入输出并分别下载三个原始记录',async()=>{
  const task={...metadata,kind:'reflection.run',task_id:'fixture-task',status:'completed',steps:[]};
  const rounds=[
    {ref:'hidden-round-1',data:{...metadata,sequence:1,status:'completed',
      prompt:[{role:'user',content:[{type:'text',text:'回顾今天关于倾听的交流。'}]}],
      response:{role:'assistant',content:'**先听完对方的想法**，再提出建议。',result:[false,0,null,'原值<&']},
    }},
    {ref:'hidden-round-2',data:{sequence:2,status:'pending',prompt:null,response:[]}},
  ];
  const route=featureRoute('/diag/task/'+task.task_id,new URLSearchParams());
  const state={session:{actorId:'fixture'},characters:[]};
  const workspace={state,subscribe:()=>()=>{},getSnapshot:()=>state,navigatePath(){},services:{
    operationsClient:{task:async id=>{assert.equal(id,task.task_id);return task;}},
    diagnosticsClient:{page:async(kind,query)=>{
      assert.equal(kind,'model-runs');
      assert.equal(query.correlationId,task.correlation_id);
      return {items:rounds,next_cursor:''};
    }},
  }};
  const view=await mountReact(vite,await route.load(),{workspace,props:{route:{feature:route,query:new URLSearchParams()}}});
  const {host}=view;
  const downloads=[],blobs=new Map();
  const create=URL.createObjectURL,revoke=URL.revokeObjectURL;
  const anchor=window.HTMLAnchorElement.prototype,click=anchor.click;
  URL.createObjectURL=blob=>{
    const url=`blob:reflection-fixture-${blobs.size}`;
    blobs.set(url,blob);
    return url;
  };
  URL.revokeObjectURL=()=>{};
  anchor.click=function(){downloads.push({filename:this.download,blob:blobs.get(this.href)});};
  try{
    assert.match(host.textContent,/回顾与自我认识/);
    const tab=[...host.querySelectorAll('[role=tab]')].find(node=>node.textContent.trim()==='模型调用');
    assert.ok(tab,'模型调用');
    await act(async()=>tab.dispatchEvent(new window.MouseEvent('mousedown',{bubbles:true,button:0})));
    await view.flush();
    const rows=host.querySelectorAll('.replica-task-result-list > li');
    assert.equal(rows.length,rounds.length);
    for(const [index,record] of rounds.entries()){
      await view.click(rows[index].querySelector('.replica-task-round-trigger'));
      const content=rows[index].querySelector('.replica-task-round-content');
      const [input,output]=content.querySelectorAll('section');
      assert.equal(input.querySelector('h3').textContent,'输入');
      assert.equal(output.querySelector('h3').textContent,'输出');
      if(index===0){
        assert.deepEqual([...input.querySelectorAll('.readable-prose')].map(node=>node.textContent),['用户','回顾今天关于倾听的交流。']);
        assert.equal(output.querySelector('.readable-prose strong').textContent,'先听完对方的想法');
        assert.match(output.textContent,/再提出建议。/);
      }
      for(const [title,filename,value] of [
        ['原始任务记录（诊断）','reflection-task.json',record],
        ['模型输入（诊断）','model-input.json',record.data.prompt],
        ['模型输出（诊断）','model-output.json',record.data.response],
      ]){
        const trigger=button(content,title);
        await view.click(trigger);
        const disclosure=trigger.closest('[data-slot=collapsible]');
        const previousCount=downloads.length;
        await view.click(button(disclosure,'下载原始记录'));
        assert.equal(downloads.length,previousCount+1);
        const downloaded=downloads.at(-1);
        assert.equal(downloaded.filename,filename);
        assert.deepEqual(JSON.parse(await downloaded.blob.text()),value);
      }
      assert.doesNotMatch(content.textContent,/未记录输入|未记录输出|未记录错误/);
    }
    assert.equal(host.querySelector('pre'),null);
    assert.doesNotMatch(host.textContent,hidden);
  }finally{
    URL.createObjectURL=create;
    URL.revokeObjectURL=revoke;
    anchor.click=click;
    await view.dispose();
  }
});

test('用户反思进度保留中文摘要与原始记录下载',async()=>{
  const {ReflectionProgress}=await vite.ssrLoadModule('/src/v3/proto/components/reflection-progress.js');
  const raw={latest_task:{...metadata,status:'retry_wait',attempt_count:2,next_attempt_at:'2026-11-23T02:15:00Z'},window:{status:'idle'}};
  const host=document.createElement('div');
  host.innerHTML=ReflectionProgress(raw);
  assert.match(host.textContent,/反思正在等待重试/);
  assert.match(host.textContent,/第 2 次尝试/);
  assert.match(host.textContent,/已安排重试：2026-11-23 10:15 \(北京时间\)/);
  assert.doesNotMatch(host.textContent,hidden);
  const links=host.querySelectorAll('a[download]');
  assert.equal(links.length,1);
  assert.equal(links[0].textContent,'原始反思任务记录（诊断）');
  const href=links[0].getAttribute('href'),prefix='data:application/json;charset=utf-8,';
  assert.ok(href.startsWith(prefix));
  assert.deepEqual(JSON.parse(decodeURIComponent(href.slice(prefix.length))),raw);
});
