import {Button} from '@/components/ui/button';
import {createPortal} from 'react-dom';
import {useState} from 'react';
import {MoreHorizontal} from 'lucide-react';
import {Tabs,TabsTrigger,TabsContent} from '@/components/ui/tabs';
import {DetailFields,ResponsiveTabsList,Feedback} from '@/components/patterns';
import DiagnosticStatus from './DiagnosticStatus';
import {taskModelFailureReason} from '@/approved-ui/task-presentation.mjs';
import {recordedCallValue,callValueLabel,callDurationLabel,callStageLabel,callPurpose} from '@/approved-ui/task-model-call-presentation.mjs';
import '../../styles/replica-diagnostic-calls.css';
import DiagnosticReplayPanel from './DiagnosticReplayPanel';
import RawContent from './RawContent';

/** The exact stored wire value, without parsing or flattening provider structures. */
export function StoredCallValue({value,missing}:{value:unknown;missing:string}){
 if(value===undefined)return <p className="replica-model-call-missing">{missing}</p>;
 return <>{missing&&(value===null||value==='')&&<p className="replica-model-call-missing">{missing}（存储值为{value===null?' null':'空字符串'}。）</p>}<pre className="replica-model-call-wire">{typeof value==='string'?value:JSON.stringify(value,null,2)}</pre></>;
}
/** Present values collapse behind RawContent; missing/null/empty keep StoredCallValue's distinct wording. */
function CallValue({label,value,missing}:{label:string;value:unknown;missing:string}){
 if(value===undefined||value===null||value==='')return <div className="replica-model-call-stored"><h4>{label}</h4><StoredCallValue value={value} missing={missing}/></div>;
 return <RawContent label={label} value={value} missing={missing}><StoredCallValue value={value} missing=""/></RawContent>;
}
export default function ModelCallDetails({record,onRecover,onOpenMedia,technicalTarget,embedded=false}:{record:any;onRecover?:()=>unknown;onOpenMedia?:(id:string)=>unknown;technicalTarget?:HTMLElement|null;embedded?:boolean}){
 const [tab,setTab]=useState('content');
 const call=record._record?.data||{},media=record._record?.kind==='media-prompts',present=(v:unknown)=>v!==undefined&&v!==null&&v!=='';
 const model=callValueLabel(recordedCallValue(call,'model_id'),record.model||'模型未记录');
 const attributes=[['用途',callPurpose(call,record.label||'用途未记录').label],['耗时',callDurationLabel(call,{includeSource:false})]];
 for(const [key,label] of [['sequence','调用序号'],['round_count','所属运行调用数'],['stage','调用阶段']])if(present(recordedCallValue(call,key)))attributes.push([label,key==='stage'?callStageLabel(call):callValueLabel(recordedCallValue(call,key),'未记录')]);
 const contents=<>
  <section className="replica-model-call-field" aria-label="调用内容"><CallValue label="请求内容" value={call.prompt} missing="此调用未记录 Request（请求）。"/><CallValue label="返回内容" value={call.response} missing="此调用未记录 Response（响应）。"/></section>
  {media&&<section className="replica-model-call-field" data-testid="media-call-details"><h3>媒体生成记录</h3>{['provider_prompt','negative_prompt','parameters','workflow_id','result'].filter(key=>Object.hasOwn(call,key)).map(key=><CallValue key={key} label={({provider_prompt:'服务请求',negative_prompt:'负面提示词',parameters:'生成参数',workflow_id:'工作流',result:'输出'} as Record<string,string>)[key]} value={call[key]} missing="尚未记录"/>)}</section>}
 </>;
 return <div className="replica-model-call-detail">
  {!embedded&&<div className="replica-model-call-overview"><strong>{model}</strong><DiagnosticStatus status={call.status}/><p>{callDurationLabel(call,{includeSource:false})} · {callStageLabel(call)}</p></div>}
  {(present(call.error_code)||present(call.error))&&<section aria-label="调用错误"><Feedback kind="error" title="调用错误"><p>{taskModelFailureReason(call)}</p><div className="replica-model-call-error">{present(call.error_code)&&<StoredCallValue value={call.error_code} missing=""/>}{present(call.error)&&<CallValue label="完整调用错误" value={call.error} missing=""/>}</div></Feedback></section>}
  {/* Actions stay above long payloads so they are reachable without scrolling past them. */}
  {(onRecover||media&&call.result?.asset_id&&onOpenMedia)&&<div className="replica-model-call-actions">
   {onRecover&&<Button variant="outline" className="replica-model-call-recovery" data-testid="diag-drawer-retry" onClick={onRecover}>查看原任务与恢复</Button>}
   {media&&call.result?.asset_id&&onOpenMedia&&<Button variant="outline" onClick={()=>onOpenMedia(call.result.asset_id)}>查看已记录素材</Button>}
  </div>}
  {embedded?<>{contents}<div className="ui-actions"><Button asChild variant="link"><a href={'#/diag/record/'+encodeURIComponent(record._record?.kind||'model-runs')+'/'+encodeURIComponent(record._record?.ref||'')+'?returnTo='+encodeURIComponent(globalThis.location?.hash||'#/diag')}>查看完整调用记录与来源</a></Button>{call.correlation_id&&<Button asChild variant="link"><a href={'#/diag/events?correlationId='+encodeURIComponent(call.correlation_id)}>查看此运行的关联事件</a></Button>}</div></>:<Tabs value={tab} onValueChange={setTab}>
  <ResponsiveTabsList value={tab} aria-label="调用详情分区"><TabsTrigger value="content">请求与返回</TabsTrigger>{typeof call.correlation_id==='string'&&call.correlation_id.trim()&&<TabsTrigger value="replay">只读回放此运行</TabsTrigger>}<TabsTrigger value="info">调用信息</TabsTrigger></ResponsiveTabsList>
  <TabsContent value="content">
  {contents}
  </TabsContent>
  <TabsContent value="replay">{typeof call.correlation_id==='string'&&call.correlation_id.trim()&&<DiagnosticReplayPanel correlationId={call.correlation_id}/>}</TabsContent>
  <TabsContent value="info"><DetailFields items={attributes.map(([label,value])=>({label,value}))}/><RawContent label="技术详情与关联标识" value={record._record} missing="完整来源未记录"/></TabsContent>
  </Tabs>}
  {technicalTarget&&createPortal(<Button variant="ghost" size="icon" aria-label="技术详情与关联标识" onClick={()=>setTab('info')}><MoreHorizontal size={16}/></Button>,technicalTarget)}
 </div>;
}
