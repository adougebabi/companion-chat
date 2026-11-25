// Scenario preview mode: answers the business API at the fetch boundary so the
// real routes, pages and components render a chosen state. Opt-in only
// (VITE_API_SCENARIOS=1); see boot.ts. Unmatched business requests fail loudly
// with 501 instead of reaching a real service.
import {SCENARIO_MARKER,callRecord,createDataset,scenarioCharacter,scenarioOwner,scenarios,type Scenario} from './data';

const json=(status:number,body:unknown)=>new Response(JSON.stringify(body),{status,headers:{'content-type':'application/json'}});
const businessPath=/^\/(api|auth|health)(\/|$)/;

export function installApiScenarios(){
 const params=new URLSearchParams(location.search);
 const scenario:Scenario=scenarios.find(entry=>entry.id===params.get('scenario'))||scenarios[0];
 if(!location.hash||location.hash==='#'||location.hash==='#/')history.replaceState(null,'',location.pathname+location.search+scenario.hash);
 const data=createDataset(),failures=new Map<RegExp,number>(),unmatched:string[]=[];
 let runningReads=0;
 const original=globalThis.fetch.bind(globalThis);

 function advanceRunning(){
  const running=data.tasks.running;
  if(!scenario.runningCompletesAfter||running.status!=='running')return;
  runningReads++;
  if(runningReads<scenario.runningCompletesAfter)return;
  Object.assign(running,{status:'completed',completed_at:new Date().toISOString(),steps:[...running.steps,{id:'s3',event_type:'workflow.completed',name:'workflow.completed',status:'completed',reason_code:'',at:new Date().toISOString()}]});
  data.calls['corr-running']=[...(data.calls['corr-running']||[]),{sequence:2,status:'completed',minute:6,seconds:2.6,prompt:'请确认计划已写入。',response:'计划已写入。',stage:'final_response'}];
  data.workflows[running.workflow_id].intent.status='completed';data.workflows[running.workflow_id].allowed_actions=[];
 }
 function control(workflowId:string,action:string){
  const inspection=data.workflows[workflowId],target=Object.values(data.tasks).find(entry=>entry.workflow_id===workflowId);
  if(!inspection||!target)return json(404,{code:'not_found',message:'没有找到原任务。'});
  const now=new Date().toISOString(),status=action==='cancel'?'cancelled':'queued';
  Object.assign(target,{status,runs:action==='restart'?target.runs+1:target.runs,completed_at:action==='cancel'?now:null,last_error:action==='restart'?undefined:target.last_error,steps:[...target.steps,{id:'cmd-'+target.steps.length,event_type:action==='restart'?'workflow.restarted':'workflow.cancel_requested',name:action,status,reason_code:'',at:now}]});
  inspection.intent.status=status;inspection.intent.revision++;inspection.allowed_actions=status==='queued'?['cancel']:[];
  inspection.runtime={run_id:'run-'+target.task_id+'-'+inspection.intent.revision,status};
  return json(200,{status:'accepted',action,workflow_id:workflowId});
 }
 function resumeVisual(sessionId:string,body:any){
  const session=data.visual.session,stage=session?.submission_recovery?.stages.find(entry=>entry.stage===body?.stage);
  if(!session||session.id!==sessionId||!stage||body.submissionReviewed!==true||body.expectedAttempt!==session.current_attempt||body.expectedVisualRevision!==data.visual.current_revision||body.expectedMediaRevision!==stage.revision)return json(409,{code:'visual_submission_recovery_stale',message:'原图片恢复条件已变化，请重新读取。'});
  data.visualResumes.push(body);
  session.candidate_submission_uncertain=false;session.submission_recovery={status:'unavailable',stages:[]};data.visual.current_revision++;
  return json(200,{fluctlight_id:scenarioCharacter.id,session_id:sessionId,stage:body.stage,media_intent_id:stage.media_intent_id,asset_id:'asset_'+stage.media_intent_id,workflow_id:body.workflowId,source_run_id:body.sourceRunId,media_status:'completed',status:'resume_requested',command_id:'cmd-visual-'+data.visualResumes.length});
 }
 function route(method:string,url:URL,body?:any):Response|null{
  const path=url.pathname,q=url.searchParams;let match:RegExpMatchArray|null;
  if(method==='GET'&&path==='/auth/session')return json(200,{authenticated:true,actorId:scenarioOwner});
  if(method==='GET'&&path==='/api/fluctlights')return json(200,[scenarioCharacter]);
  if(method==='GET'&&path==='/api/notifications/unread-count')return json(200,{count:0,alert_count:0});
  if(method==='GET'&&path==='/api/media-library')return json(200,{items:[],next_cursor:''});
  if(method==='GET'&&(match=path.match(/^\/api\/diagnostics\/tasks\/([^/]+)$/))){
   const id=decodeURIComponent(match[1]);if(id==='running')advanceRunning();
   const value=data.tasks[id];return value?json(200,value):json(404,{code:'task_not_found',message:'没有找到这个任务。'});
  }
  if(method==='GET'&&(match=path.match(/^\/api\/workflow-controls\/([^/]+)$/))){const value=data.workflows[decodeURIComponent(match[1])];return value?json(200,value):json(404,{code:'not_found',message:'没有找到原任务。'});}
  if(method==='POST'&&(match=path.match(/^\/api\/workflow-controls\/([^/]+)\/(restart|cancel)$/)))return control(decodeURIComponent(match[1]),match[2]);
  if(method==='GET'&&(match=path.match(/^\/api\/diagnostic-records\/([^/]+)$/))){
   const kind=decodeURIComponent(match[1]),page=(items:unknown[])=>json(200,{items,next_cursor:'',snapshot:'scenario',as_of:new Date().toISOString(),filters:{},snapshot_semantics:'scenario',coverage:'complete'});
   if(kind==='model-runs'){const id=q.get('correlationId')||'';return page((data.calls[id]||[]).map((seed,index)=>callRecord(id,seed,index)));}
   if(kind==='workflow-intents'){const type=q.get('intentType');return page(Object.values(data.tasks).filter(entry=>!type||entry.kind===type).map(entry=>({id:entry.task_id,ref:'intent:'+entry.task_id,kind:'workflow-intents',occurred_at:entry.created_at,data:{...entry,intent_id:entry.task_id,intent_type:entry.kind}})));}
   if(kind==='media-prompts')return page(data.media[q.get('correlationId')||'']||[]);
  }
  if((match=path.match(/^\/api\/fluctlights\/([^/]+)\/(visual-identity|visual-identity\/timeline|visual-brief|visual-brief\/history|visual-sessions\/([^/]+)\/resume-reviewed)$/))&&decodeURIComponent(match[1])===scenarioCharacter.id){
   const empty={items:[],next_cursor:''};
   if(method==='GET'&&match[2]==='visual-identity')return json(200,data.visual);
   if(method==='GET'&&match[2]==='visual-brief')return json(200,{fluctlight_id:scenarioCharacter.id,revision:3,brief:null});
   if(method==='GET'&&/\/(timeline|history)$/.test(match[2]))return json(200,empty);
   if(method==='POST'&&match[3])return resumeVisual(decodeURIComponent(match[3]),body);
  }
  if(method==='GET'&&path==='/api/pending'&&q.get('kind')==='failed_task'){
   const items=Object.values(data.tasks).filter(entry=>entry.status==='failed').map(entry=>({id:'workflow:'+entry.task_id,source_id:entry.task_id,fluctlight_id:entry.fluctlight_id||'',title:entry.kind,status:entry.status,href:'#/diag/task/'+entry.task_id,created_at:entry.created_at}));
   return json(200,{groups:[{kind:'failed_task',status:'ok',items,count:items.length,next_cursor:''}],count:items.length,partial:false});
  }
  return null;
 }
 globalThis.fetch=async(input:RequestInfo|URL,init?:RequestInit)=>{
  const url=new URL(input instanceof Request?input.url:String(input),location.origin);
  if(url.origin!==location.origin||!businessPath.test(url.pathname))return original(input,init);
  const method=(init?.method||(input instanceof Request?input.method:'GET')).toUpperCase();
  if(scenario.delayMs)await new Promise(resolve=>setTimeout(resolve,scenario.delayMs));
  if(scenario.failFirst?.pattern.test(url.pathname+url.search)){const count=failures.get(scenario.failFirst.pattern)||0;if(count<scenario.failFirst.times){failures.set(scenario.failFirst.pattern,count+1);return json(503,{code:'service_unavailable',message:'预览模式模拟的读取失败，请重试。'});}}
  let body:unknown;
  if(method!=='GET'){try{body=JSON.parse(String(init?.body??(input instanceof Request?await input.clone().text():''))||'null');}catch{body=null;}}
  const response=route(method,url,body);
  if(response)return response;
  const label=`${method} ${url.pathname}`;
  if(!unmatched.includes(label)){unmatched.push(label);console.warn(`[预览模式] 未模拟的业务接口：${label}`);renderBadge();}
  return json(501,{code:'mock_unmatched',message:`预览模式未模拟此接口：${label}`});
 };

 function select(next:string){
  // Drop per-owner persisted state so a new scenario never inherits drafts,
  // pending commands or view preferences from the previous one.
  for(const store of [localStorage,sessionStorage])try{for(const key of Object.keys(store))if(key.includes(scenarioOwner))store.removeItem(key);}catch{/* storage unavailable */}
  const target=scenarios.find(entry=>entry.id===next)||scenarios[0];
  history.replaceState(null,'',location.pathname+'?scenario='+encodeURIComponent(target.id)+target.hash);
  location.reload();
 }
 let badge:HTMLDetailsElement|null=null;
 function renderBadge(){
  if(!document.body){document.addEventListener('DOMContentLoaded',renderBadge,{once:true});return;}
  badge??=document.body.appendChild(document.createElement('details'));
  badge.dataset.apiScenarios=SCENARIO_MARKER;badge.setAttribute('aria-label','预览模式');
  badge.style.cssText='pointer-events:auto;position:fixed;left:8px;bottom:8px;z-index:2147483000;max-width:min(360px,calc(100vw - 16px));font:12px/1.5 Inter,"PingFang SC",sans-serif;color:var(--foreground,#2e3931);background:var(--popover,#fff);border:1px solid var(--destructive,#b94d48);border-radius:var(--radius,0.6rem);padding:4px 10px;opacity:.95';
  badge.innerHTML='';
  const summary=badge.appendChild(document.createElement('summary'));summary.style.cursor='pointer';
  summary.textContent=`预览模式 · ${scenario.label}${unmatched.length?` · 未模拟接口 ${unmatched.length}`:''}`;
  const panel=badge.appendChild(document.createElement('div'));panel.style.cssText='display:grid;gap:6px;padding:6px 0';
  const picker=panel.appendChild(document.createElement('select'));picker.setAttribute('aria-label','选择预览场景');picker.style.minHeight='32px';
  for(const entry of scenarios){const option=picker.appendChild(document.createElement('option'));option.value=entry.id;option.textContent=entry.label;option.selected=entry.id===scenario.id;}
  picker.onchange=()=>select(picker.value);
  const reset=panel.appendChild(document.createElement('button'));reset.type='button';reset.textContent='恢复本场景初始状态';reset.style.minHeight='32px';reset.onclick=()=>select(scenario.id);
  const note=panel.appendChild(document.createElement('p'));note.textContent='数据来自本地模拟，不会访问真实服务。'+(unmatched.length?'未模拟：'+unmatched.join('、'):'');
 }
 renderBadge();
}
