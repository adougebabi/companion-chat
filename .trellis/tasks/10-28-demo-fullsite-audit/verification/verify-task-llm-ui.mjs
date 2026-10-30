import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const {chromium}=require('playwright-core'),phase=process.env.AUDIT_PHASE||'task-llm-final';
const output=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/task-llm/'+phase),shots=resolve('/private/tmp/fluctlight-demo-audit-20261028/task-llm-shots/'+phase);
await mkdir(output,{recursive:true});await mkdir(shots,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [size,width,height] of [['wide',1920,1113],['desktop',1348,925],['mobile',390,844]])for(const name of ['content','loading','read-failure','pagination','missing-fields','empty','unlinked']){
 if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(name))continue;
 if(process.env.AUDIT_SIZES&&!process.env.AUDIT_SIZES.split(',').includes(size))continue;
 const fixture=await startOwnedFixture({port:4522,env:{FLUCTLIGHT_UI_FIXTURE_MANAGEMENT:'normal'}}),context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(10000);
 const record={name,size,status:'running',checks:[],requests:[],errors:[],local_screenshots:[],geometry:[],boundary:'production dist + scoped controlled HTTP; no real model invocation; images outside Git'};
 page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const u=new URL(r.url());if(u.pathname.startsWith('/api/'))record.requests.push({method:r.method(),path:u.pathname,query:Object.fromEntries(u.searchParams)});});
 const taskId='actual-task-'+name,correlation='task-call-correlation-'+name,fid='fluctlight-fixture-one',stamp='2026-10-30T00:47:13.489Z';
 const task={task_id:taskId,workflow_id:'original-workflow-'+name,kind:'wardrobe.initialize',type:'wardrobe.initialize',title:'wardrobe.initialize',fluctlight_id:fid,status:'failed',runs:1,created_at:stamp,started_at:stamp,completed_at:'2026-10-30T00:47:21.576Z',...(name==='unlinked'?{}:{correlation_id:correlation}),source:{last_error:'衣柜准备没有得到有效方案。'},steps:[{id:'dispatcher',name:'dispatcher',event_type:'lifecycle.workflow.queued',status:'queued',at:stamp},{id:'activity',name:'activity',event_type:'lifecycle.activity.failed',status:'failed',at:'2026-10-30T00:47:21.576Z'}]};
 const prompt={model:'stored-model',messages:[{role:'system',content:'已保存的完整运行规则。'.repeat(90)},{role:'user',content:'REQUEST_BODY_ORIGINAL：请整理已有衣柜与穿搭。'+ '长输入'.repeat(140)},{role:'assistant',content:null,tool_calls:[{id:'original-tool-call',function:{name:'inventory_lookup',arguments:'{"owned":false,"count":0}'}}]}],extensions:{keep:false,count:0,unknown:null,key:'long-extension-'.repeat(40)}};
 let response={choices:[{message:{role:'assistant',content:'RESPONSE_BODY_ORIGINAL：当前尚缺可执行穿搭方案。',tool_calls:[{id:'actual-tool-return',function:{name:'wardrobe_wear',arguments:'{"item_ids":[],"confirmed":false}'}}]}}],usage:{input_tokens:321,output_tokens:57},extensions:{false:false,zero:0,list:[]}},failed=false,release;
 const held=new Promise(r=>release=r),rows=(second=false)=>[{ref:second?'second-call':'original-call',kind:'model-runs',occurred_at:stamp,data:{id:second?'second-call':'original-call',model_id:'stored-model',correlation_id:correlation,status:'failed',error_code:'provider_invalid_output',error:'ACTUAL_CALL_ERROR: returned tool arguments did not contain a wearable outfit',prompt:second?{messages:[{role:'user',content:'SECOND_REQUEST'}]}:prompt,response:second?{choices:[{message:{content:'SECOND_RESPONSE'}}]}:response,created_at:stamp}}];
 const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
 const dialog=()=>page.getByRole('dialog').first(),shot=async label=>{await page.evaluate(()=>document.fonts.ready);const path=resolve(shots,name+'-'+size+'-'+label+'.png');await page.screenshot({path,animations:'disabled'});record.local_screenshots.push(path);check(label+': no horizontal document overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));};
 await page.route(/\/api\/diagnostic-records\/workflow-intents(?:\?|$)/,route=>route.fulfill({json:{items:[{ref:taskId,kind:'workflow-intents',occurred_at:stamp,data:{...task,intent_type:task.kind}}],next_cursor:''}}));
 await page.route('**/api/diagnostics/tasks/*',route=>route.fulfill({json:task}));
 await page.route('**/api/fluctlights/'+fid+'/inventory/preparation',route=>route.fulfill({json:{id:'preparation-original',task_id:taskId,fluctlight_id:fid,status:'failed',result:{inventory_committed:false,failure_reason:'衣物和穿搭方案尚未保存。'}}}));
 await page.route('**/api/workflow-controls/original-workflow-*',route=>route.fulfill({json:{intent:{revision:1,status:'failed'},runtime_state:'unavailable',allowed_actions:[],commands:[],recovery:{mode:'domain_recovery',href:'/c/'+fid+'/life/wardrobe',label:'查看初始衣柜与恢复'}}}));
 await page.route(/\/api\/diagnostic-records\/model-runs(?:\?|$)/,async route=>{
  const q=new URL(route.request().url()).searchParams;
  if(q.get('correlationId')!==correlation){await route.fulfill({json:{items:[],next_cursor:''}});return;}
  if(name==='loading')await held;
  if((name==='read-failure'&&!failed)||(name==='pagination'&&q.has('cursor')&&!failed)){failed=true;await route.fulfill({status:503,json:{code:'request_failed',message:'controlled model record read failure'}});return;}
  await route.fulfill({json:{items:name==='empty'?[]:name==='missing-fields'?[{ref:'missing-call',kind:'model-runs',data:{status:'failed',error_code:'empty_response',prompt:null,response:null}}]:rows(q.has('cursor')),next_cursor:name==='pagination'&&!q.has('cursor')?'original-next-cursor':''}});
 });
 try{
  await page.goto(fixture.base+'/#/diag?view=tasks',{waitUntil:'networkidle'});await page.locator('.replica-task-row').first().waitFor();const start=record.requests.length;await page.locator('.replica-task-row').first().click();await dialog().waitFor();await dialog().getByRole('tab',{name:'执行记录',exact:true}).waitFor();check('LLM records are the default task view',await dialog().getByRole('tab',{name:'执行记录',exact:true}).getAttribute('aria-selected')==='true');
  check('task dialog translates production enum title',!(await dialog().locator('[data-slot=dialog-title]').innerText()).includes('wardrobe.initialize'));
  const box=await dialog().boundingBox();record.geometry.push({dialog:box});check('wide dialog matches the Demo width',size==='mobile'?box.width<=width-30:Math.abs(box.width-860)<1);
  if(name==='loading'){await dialog().getByText('正在读取关联的模型调用…',{exact:true}).waitFor();await shot('loading');release();}
  if(name==='read-failure'){await dialog().getByRole('button',{name:'重试原读取',exact:true}).click();}
  if(['content','loading','read-failure','pagination'].includes(name)){
   await dialog().getByText('Request · 请求',{exact:true}).first().waitFor();const raw=await dialog().locator('.replica-task-call-payloads pre').allTextContents();check('full stored Request is shown unchanged',raw[0]===JSON.stringify(prompt,null,2));check('full stored Response is shown unchanged',raw[1]===JSON.stringify(response,null,2));check('actual model call error is visible',await dialog().getByText(/ACTUAL_CALL_ERROR/).isVisible());
   check('workflow facility names are not the record body',!(await dialog().innerText()).includes('workflow_start')&&!(await dialog().innerText()).includes('dispatcher'));
   record.geometry.push({payloads:await dialog().locator('.replica-task-call-payloads').first().evaluate(e=>({columns:getComputedStyle(e).gridTemplateColumns,rect:e.getBoundingClientRect().toJSON(),panes:[...e.querySelectorAll('pre')].map(p=>({rect:p.getBoundingClientRect().toJSON(),scrollHeight:p.scrollHeight,clientHeight:p.clientHeight,scrollWidth:p.scrollWidth,clientWidth:p.clientWidth}))}))});
   const geometry=record.geometry.at(-1).payloads;check('request and response use the appropriate responsive columns',geometry.columns.split(' ').length===(size==='mobile'?1:2));check('long request remains bounded and scrollable',geometry.panes[0].scrollHeight>geometry.panes[0].clientHeight&&geometry.panes[0].clientHeight<=361);check('long values do not overflow either pane',geometry.panes.every(p=>p.scrollWidth<=p.clientWidth+1));await shot('request-response');
  }
  if(name==='pagination'){
   await dialog().getByRole('button',{name:'加载更多模型调用',exact:true}).click();await dialog().getByRole('button',{name:'重试原读取',exact:true}).waitFor();check('read-more failure keeps the confirmed request',await dialog().getByText(/REQUEST_BODY_ORIGINAL/).isVisible());await shot('read-more-failed');await dialog().getByRole('button',{name:'重试原读取',exact:true}).click();await dialog().getByText(/SECOND_RESPONSE/).waitFor();const cursors=record.requests.slice(start).filter(r=>r.path.endsWith('/model-runs')).map(r=>r.query.cursor||'');check('retry retains original correlation and failed cursor',JSON.stringify(cursors)===JSON.stringify(['','original-next-cursor','original-next-cursor']));
  }
  if(name==='content'){
   const requestPane=dialog().locator('.replica-task-call-payloads pre').first();await requestPane.evaluate(e=>e.scrollTop=e.scrollHeight);check('long request can scroll to its original final fields',await requestPane.evaluate(e=>e.scrollTop>0));await dialog().locator('.replica-task-call-payloads>section').nth(1).evaluate(e=>e.scrollIntoView({block:'center',behavior:'instant'}));await shot('response-readable');
   response={choices:[{message:{content:'REFRESHED_RESPONSE'}}]};await dialog().getByRole('button',{name:'刷新调用记录',exact:true}).click();await dialog().getByText(/REFRESHED_RESPONSE/).waitFor();check('manual refresh reads actual updated response',!(await dialog().innerText()).includes('RESPONSE_BODY_ORIGINAL'));await dialog().getByRole('tab',{name:'执行阶段',exact:true}).click();await dialog().getByRole('heading',{name:'整理衣物与穿搭方案',exact:true}).waitFor();check('matching nested preparation supplies real business phases',await dialog().getByRole('heading',{name:'确认初始穿着',exact:true}).count()===1&&!(await dialog().innerText()).includes('已记录环节'));await shot('real-phases');await dialog().getByRole('tab',{name:'任务信息',exact:true}).click();await dialog().getByRole('button',{name:'生命周期记录（技术详情）',exact:true}).click();check('workflow records remain secondary and explicitly expandable',(await dialog().innerText()).includes('dispatcher'));await shot('secondary-info');
  }
  if(name==='missing-fields'){await dialog().getByText(/此调用未记录 Request/).waitFor();await dialog().getByText(/此调用未记录 Response/).waitFor();check('missing wire fields are explicit',await dialog().locator('.replica-task-call-payloads pre').allTextContents().then(x=>JSON.stringify(x)===JSON.stringify(['null','null'])));await shot('missing-wire');}
  if(name==='empty'){await dialog().getByText(/不能据此判断模型是否已被调用/).waitFor();await shot('no-records');}
  if(name==='unlinked'){await dialog().getByText(/未提供模型调用关联标识/).waitFor();check('unlinked task never starts an unscoped model query',!record.requests.slice(start).some(r=>r.path.endsWith('/model-runs')));await shot('missing-association');}
  const calls=record.requests.slice(start).filter(r=>r.path.endsWith('/model-runs'));check('all task model reads retain their exact correlation',calls.every(r=>r.query.correlationId===correlation));check('viewing and refreshing never submits a command',record.requests.every(r=>r.method==='GET'));
  await page.keyboard.press('Escape');await dialog().waitFor({state:'hidden'});check('Escape returns focus to original task entry',await page.locator('.replica-task-row').first().evaluate(e=>document.activeElement===e));check('no uncaught browser error',record.errors.length===0);record.status='passed';
 }catch(error){record.status='failed';record.error=error.stack;await shot('failure').catch(()=>{});}
 finally{release();results.push(record);await writeFile(resolve(output,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);await context.close();await fixture.stop();}
}}finally{await browser.close();}
if(results.some(r=>r.status==='failed'))process.exitCode=1;
