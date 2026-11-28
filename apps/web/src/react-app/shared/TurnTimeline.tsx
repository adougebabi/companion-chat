import {useCallback,useEffect,useRef,useState,type ReactNode} from 'react';
import {Check,ChevronDown,Copy} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Collapsible,CollapsibleContent,CollapsibleTrigger} from '@/components/ui/collapsible';
import {DataState,DetailFields,Feedback,type FeedbackKind} from '@/components/patterns';
import {DetailDrawer} from '../components';
import {Disclosure} from '../controls';
import {useWorkspace} from '../page';
import DiagnosticStatus from './DiagnosticStatus';
import ModelCallDetails from './ModelCallDetails';
import {callDurationLabel,callPurpose} from '@/approved-ui/task-model-call-presentation.mjs';
import {taskModelFailureReason} from '@/approved-ui/task-presentation.mjs';
import {domainName,errorMessage,stateLabel} from '@/v3/domain-presentation.mjs';

/**
 * 本轮详情 — every physical model call and logical Agent run recorded for one
 * chat turn (correlation id `turn:<turnId>`), oldest first. Read-only. Failed
 * calls open on arrival so the cause is visible at once; raw request/response
 * stay behind ModelCallDetails' collapsed RawContent.
 */

/** Owner diagnostic records (`/api/diagnostic-records/{kind}`): `data` is the raw snake_case row. */
export type TurnRecord={id?:string;ref:string;kind:string;occurred_at?:string;data:Record<string,any>};
export type TurnRecordPage={items:TurnRecord[];next_cursor?:string};
export type TurnRecordReader=(kind:'model-runs'|'agent-runs',options:Record<string,string|number>)=>Promise<TurnRecordPage>;
export type TurnRecordsState=
 |{status:'loading'}
 |{status:'error';error:string}
 |{status:'ready';modelRuns:TurnRecord[];agentRuns:TurnRecord[];truncated:boolean};

const LIMIT=100;
export const turnCorrelationId=(turnId:string)=>'turn:'+turnId;
const FAILED=['failed','error','dead_letter','timeout'],DONE=['completed','succeeded','success'];
const instant=(value:unknown)=>typeof value==='string'?Date.parse(value):NaN;

/** Chronological order by recorded start; the per-run sequence only breaks ties. */
export function sortTurnRecords(records:TurnRecord[]){
 return records.map((record,index)=>({record,index})).sort((a,b)=>{
  const da=a.record.data||{},db=b.record.data||{};
  const ta=instant(da.created_at??da.started_at??a.record.occurred_at),tb=instant(db.created_at??db.started_at??b.record.occurred_at);
  if(Number.isFinite(ta)&&Number.isFinite(tb)&&ta!==tb)return ta-tb;
  const sa=Number.isInteger(da.sequence)?da.sequence:Infinity,sb=Number.isInteger(db.sequence)?db.sequence:Infinity;
  return sa!==sb?sa-sb:a.index-b.index;
 }).map(entry=>entry.record);
}

export function agentRunName(agentId:unknown){
 if(typeof agentId!=='string'||!agentId.trim())return 'Agent（未记录名称）';
 const named=domainName(agentId,'');
 if(named)return named;
 const purpose=callPurpose({role:agentId});
 return purpose.known?purpose.label:'Agent · '+agentId;
}
function agentFailureReason(run:Record<string,any>){
 return errorMessage({code:run.failure_code||'',message:''},'这次 Agent 运行没有完成，原因已保留在原始记录中。');
}

export function turnSummary(modelRuns:TurnRecord[],agentRuns:TurnRecord[]):{kind:FeedbackKind;title:string;text:string}{
 const calls=modelRuns.map(record=>record.data||{}),agents=agentRuns.map(record=>record.data||{});
 const failedIndex=calls.findIndex(call=>FAILED.includes(call.status)),failedAgents=agents.filter(run=>FAILED.includes(run.status));
 const pending=[...calls,...agents].some(row=>!FAILED.includes(row.status)&&!DONE.includes(row.status));
 let text=`共 ${calls.length} 次模型调用`;
 if(failedIndex>=0)text+=`，第 ${failedIndex+1} 次失败：${taskModelFailureReason(calls[failedIndex])}`;
 else if(calls.length&&!pending)text+='，全部成功。';
 else text+='。';
 if(agents.length)text+=` 记录了 ${agents.length} 次 Agent 运行${failedAgents.length?`，其中 ${failedAgents.length} 次失败`:''}。`;
 if(failedIndex>=0||failedAgents.length)return {kind:'error',title:'这一轮有失败的步骤',text};
 if(pending)return {kind:'progress',title:'这一轮还有步骤未结束',text};
 return {kind:'success',title:'这一轮已完成',text};
}

/** Reads both record kinds for one turn through the existing owner diagnostics client. */
export function useTurnRecords(turnId:string,read?:TurnRecordReader){
 const workspace=useWorkspace();
 const reader:TurnRecordReader|undefined=read||((kind,options)=>workspace?.services?.diagnosticsClient?.page(kind,options));
 const [state,setState]=useState<TurnRecordsState>({status:'loading'});
 const [attempt,setAttempt]=useState(0),readerRef=useRef(reader);
 readerRef.current=reader;
 useEffect(()=>{
  let alive=true;setState({status:'loading'});
  const options={correlationId:turnCorrelationId(turnId),limit:LIMIT};
  (async()=>{
   try{
    const current=readerRef.current;if(!current)throw new Error('诊断读取服务尚未连接。');
    const [models,agents]=await Promise.all([current('model-runs',options),current('agent-runs',options)]);
    if(alive)setState({status:'ready',modelRuns:sortTurnRecords(models?.items||[]),agentRuns:sortTurnRecords(agents?.items||[]),truncated:!!(models?.next_cursor||agents?.next_cursor)});
   }catch(cause){if(alive)setState({status:'error',error:errorMessage(cause,'本轮记录暂时无法读取，请稍后重试。')});}
  })();
  return()=>{alive=false;};
 },[turnId,attempt]);
 return {state,retry:useCallback(()=>setAttempt(value=>value+1),[])};
}

function CallRow({record,index}:{record:TurnRecord;index:number}){
 const call=record.data||{},failed=FAILED.includes(call.status);
 const status=DONE.includes(call.status)?'成功':failed?'失败':undefined;
 const purpose=callPurpose(call,'模型调用').label;
 return <li data-call-status={failed?'failure':DONE.includes(call.status)?'success':'other'}>
  <Collapsible className="replica-task-result-row" defaultOpen={failed}>
   <CollapsibleTrigger asChild><Button type="button" variant="ghost" className="replica-task-round-trigger">
    <span className="min-w-0 break-words">{index+1}. {purpose}<span className="block text-muted-foreground">{call.model_id||'模型未记录'}</span></span>
    <DiagnosticStatus status={call.status} label={status}/>
    <span className="replica-task-round-duration">{callDurationLabel(call,{includeSource:false})}</span>
    <ChevronDown size={14} aria-hidden="true"/>
   </Button></CollapsibleTrigger>
   <CollapsibleContent className="replica-task-round-content">
    <section aria-label={failed?'失败原因':undefined}><ModelCallDetails embedded record={{_record:record,model:call.model_id,label:purpose,status:call.status}}/></section>
   </CollapsibleContent>
  </Collapsible>
 </li>;
}

function AgentRuns({runs}:{runs:TurnRecord[]}){
 if(!runs.length)return null;
 return <section className="grid min-w-0 gap-3" aria-label="Agent 运行">
  <div className="replica-task-section-head"><h3>Agent 运行</h3><span>{runs.length} 次</span></div>
  <ul className="grid min-w-0 list-none gap-2 p-0">{runs.map(record=>{const run=record.data||{},failed=FAILED.includes(run.status);return <li key={record.ref} className="grid min-w-0 gap-1 rounded-md border p-3 text-sm" data-agent-status={failed?'failure':'other'}>
   <div className="flex min-w-0 flex-wrap items-center justify-between gap-2"><strong className="min-w-0 break-words font-medium">{agentRunName(run.agent_id)}</strong><DiagnosticStatus status={run.status}/></div>
   {failed&&<p className="m-0 text-destructive">{agentFailureReason(run)}{run.failure_stage?`（${domainName(run.failure_stage,stateLabel(run.failure_stage,'阶段未记录'))}）`:''}</p>}
  </li>;})}</ul>
 </section>;
}

function technicalInfo(turnId:string,modelRuns:TurnRecord[],agentRuns:TurnRecord[]){
 return {
  turnId,correlationId:turnCorrelationId(turnId),
  modelCalls:modelRuns.map(record=>({ref:record.ref,modelCallId:record.data?.model_call_id,logicalRunId:record.data?.logical_run_id,sequence:record.data?.sequence,role:record.data?.role,scenario:record.data?.scenario,errorCode:record.data?.error_code})),
  agentRuns:agentRuns.map(record=>({ref:record.ref,agentId:record.data?.agent_id,runId:record.data?.run_id,failureCode:record.data?.failure_code,failureStage:record.data?.failure_stage})),
 };
}

/** The timeline body for one turn's records; loading/empty/error come from DataState. */
export function TurnTimelineView({turnId,state,onRetry,technical}:{turnId:string;state:TurnRecordsState;onRetry:()=>void;technical?:Record<string,unknown>}){
 const ready=state.status==='ready'?state:null;
 const modelRuns=ready?.modelRuns||[],agentRuns=ready?.agentRuns||[];
 const empty=!!ready&&!modelRuns.length&&!agentRuns.length;
 const summary=ready&&!empty?turnSummary(modelRuns,agentRuns):null;
 return <div className="grid min-w-0 gap-5" data-turn-timeline data-state={state.status}>
  <DataState label="本轮记录" loading={state.status==='loading'} error={state.status==='error'?state.error:undefined} hasData={!!ready} empty={empty}
   emptyTitle="这一轮没有记录到模型调用" emptyDescription="可能还在排队，或在调用模型之前就已停止。" onRetry={onRetry}>
   {summary&&<div className="grid min-w-0 gap-5">
    <Feedback kind={summary.kind} title={summary.title} role="note" data-turn-summary>{summary.text}</Feedback>
    {ready?.truncated&&<Feedback kind="information">只显示了前 {LIMIT} 条记录，其余记录请在诊断中心查看。</Feedback>}
    <section className="grid min-w-0 gap-3" aria-label="模型调用">
     <div className="replica-task-section-head"><h3>模型调用</h3><span>按发生时间排列</span></div>
     {modelRuns.length?<ol className="replica-task-result-list">{modelRuns.map((record,index)=><CallRow key={record.ref} record={record} index={index}/>)}</ol>:<p className="m-0 text-sm text-muted-foreground">这一轮没有记录到模型调用。</p>}
    </section>
    <AgentRuns runs={agentRuns}/>
   </div>}
  </DataState>
  <Disclosure title="技术信息"><DetailFields items={[{label:'回合标识',value:<span className="break-all font-mono">{turnId}</span>},{label:'关联标识',value:<span className="break-all font-mono">{turnCorrelationId(turnId)}</span>}]}/>{(ready||technical)&&<pre className="replica-model-call-wire">{JSON.stringify({...technical,...(ready?technicalInfo(turnId,modelRuns,agentRuns):{})},null,2)}</pre>}</Disclosure>
 </div>;
}

function CopyTurnJson({turnId,state}:{turnId:string;state:TurnRecordsState}){
 const [copy,setCopy]=useState<'idle'|'done'|'failed'>('idle');
 useEffect(()=>{if(copy==='idle')return;const timer=setTimeout(()=>setCopy('idle'),2500);return()=>clearTimeout(timer);},[copy]);
 const ready=state.status==='ready'?state:null;
 async function run(){
  if(!ready)return;
  try{await navigator.clipboard.writeText(JSON.stringify({turnId,modelRuns:ready.modelRuns,agentRuns:ready.agentRuns},null,2));setCopy('done');}catch{setCopy('failed');}
 }
 return <>
  <Button type="button" variant="outline" disabled={!ready} onClick={()=>void run()}>{copy==='done'?<Check aria-hidden="true"/>:<Copy aria-hidden="true"/>}{copy==='done'?'已复制':copy==='failed'?'复制失败':'复制整轮 JSON'}</Button>
  <span className="sr-only" role="status">{copy==='done'?'整轮 JSON 已复制':copy==='failed'?'复制失败，请展开技术信息后手动选择':''}</span>
 </>;
}

/** One-click 本轮详情 drawer, opened from a chat message. `read` replaces the workspace client (UI Lab). */
export default function TurnDetailsDrawer({turnId,onClose,read,returnFocus,technical,children}:{turnId:string;onClose:()=>void;read?:TurnRecordReader;returnFocus?:()=>HTMLElement|null;technical?:Record<string,unknown>;children?:ReactNode}){
 const {state,retry}=useTurnRecords(turnId,read);
 return <DetailDrawer title="本轮详情" testId="turn-details" onClose={onClose} returnFocus={returnFocus}
  footer={<><CopyTurnJson turnId={turnId} state={state}/><Button type="button" variant="outline" onClick={onClose}>关闭</Button></>}>
  <div className="grid min-w-0 gap-5">{children}<TurnTimelineView turnId={turnId} state={state} onRetry={retry} technical={technical}/></div>
 </DetailDrawer>;
}
