import {Button} from '@/components/ui/button';
import {createPortal} from 'react-dom';
import {useState} from 'react';
import {MoreHorizontal} from 'lucide-react';
import {recordedCallValue,callValueLabel,callDurationLabel,callStageLabel} from '@/approved-ui/task-model-call-presentation.mjs';
import {domainName} from '@/v3/domain-presentation.mjs';
import '../../styles/replica-diagnostic-calls.css';

/** The exact stored wire value, without parsing or flattening provider structures. */
export function StoredCallValue({value,missing}:{value:unknown;missing:string}){
 if(value===undefined)return <p className="replica-model-call-missing">{missing}</p>;
 return <>{missing&&(value===null||value==='')&&<p className="replica-model-call-missing">{missing}（存储值为{value===null?' null':'空字符串'}。）</p>}<pre className="replica-model-call-wire">{typeof value==='string'?value:JSON.stringify(value,null,2)}</pre></>;
}
export default function ModelCallDetails({record,onRecover,onOpenMedia,technicalTarget}:{record:any;onRecover?:()=>unknown;onOpenMedia?:(id:string)=>unknown;technicalTarget?:HTMLElement|null}){
 const [sourceOpen,setSourceOpen]=useState(false);
 const call=record._record?.data||{},media=record._record?.kind==='media-prompts',present=(v:unknown)=>v!==undefined&&v!==null&&v!=='';
 const attributes=[['模型',callValueLabel(recordedCallValue(call,'model_id'),record.model||'模型未记录')],['用途',({generic_llm:'通用分析',conversation_reply:'聊天回复',vision_understanding:'看图理解',embedding:'记忆检索向量'} as Record<string,string>)[call.binding_role||call.role]||domainName(call.binding_role||call.role,record.label||'用途未记录')],['耗时',callDurationLabel(call,{includeSource:false})]];
 for(const [key,label] of [['sequence','调用序号'],['round_count','所属运行调用数'],['stage','调用阶段']])if(present(recordedCallValue(call,key)))attributes.push([label,key==='stage'?callStageLabel(call):callValueLabel(recordedCallValue(call,key),'未记录')]);
 return <div className="replica-model-call-detail">
  <dl className="replica-model-call-attributes">{attributes.map(([label,value])=><div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
  {(present(call.error_code)||present(call.error))&&<section className="replica-model-call-error" aria-label="调用错误"><h3>调用错误</h3>{present(call.error_code)&&<StoredCallValue value={call.error_code} missing=""/>}{present(call.error)&&<StoredCallValue value={call.error} missing=""/>}</section>}
  <section className="replica-model-call-field"><h3>请求摘要</h3><StoredCallValue value={call.prompt} missing="此调用未记录 Request（请求）。"/></section>
  <section className="replica-model-call-field"><h3>返回内容</h3><StoredCallValue value={call.response} missing="此调用未记录 Response（响应）。"/></section>
  {media&&<section className="replica-model-call-field" data-testid="media-call-details"><h3>媒体生成记录</h3>{['provider_prompt','negative_prompt','parameters','workflow_id','result'].filter(key=>Object.hasOwn(call,key)).map(key=><section key={key}><h4>{({provider_prompt:'服务请求',negative_prompt:'负面提示词',parameters:'生成参数',workflow_id:'工作流',result:'输出'})[key]}</h4><StoredCallValue value={call[key]} missing="尚未记录"/></section>)}{call.result?.asset_id&&onOpenMedia&&<Button variant="outline" onClick={()=>onOpenMedia(call.result.asset_id)}>查看已记录素材</Button>}</section>}
  {onRecover&&<Button variant="outline" className="replica-model-call-recovery" data-testid="diag-drawer-retry" onClick={onRecover}>查看原任务与恢复</Button>}
  {technicalTarget?createPortal(<Button variant="ghost" size="icon" aria-label="技术详情与关联标识" aria-expanded={sourceOpen} onClick={()=>setSourceOpen(value=>!value)}><MoreHorizontal size={16}/></Button>,technicalTarget):<Button variant="ghost" size="sm" aria-expanded={sourceOpen} onClick={()=>setSourceOpen(value=>!value)}>技术详情与关联标识</Button>}
  {sourceOpen&&<section><h3>技术详情与关联标识</h3><StoredCallValue value={record._record} missing="完整来源未记录"/></section>}
 </div>;
}
