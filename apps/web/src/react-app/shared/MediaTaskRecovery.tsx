import {createPortal} from 'react-dom';
import {useEffect,useId,useMemo,useRef,useState} from 'react';
import {Button} from '@/components/ui/button';
import {Feedback} from './Presentation';
import WorkspaceDialog from './WorkspaceDialog';
import {Label,Textarea} from '../controls';
import {useWorkspace} from '../page';
import {persistentCommand} from '@/v3/providers/persistent-command.mjs';
import {studioDraftStorage} from '@/v3/proto/components/studio-draft-storage.js';
import {errorMessage} from '@/v3/domain-presentation.mjs';

const modeNames:Record<string,string>={resume:'从失败处重试',regenerate:'重新生成'};
const workflowKey=(value:unknown)=>typeof value==='string'?value.replace(/^go:/,''):'';
function mediaPurpose(value:any){
 if(typeof value==='string')try{return JSON.parse(value)?.purpose||'';}catch{return '';}
 return value&&typeof value==='object'?value.purpose||'':'';
}

// The media command has its own revision and idempotency identity. It is not a
// generic workflow restart; the server checks the current attempt and effects.
export default function MediaTaskRecovery({mediaId,workflowId,snapshot,reader,controlTarget,onRecovered}:{mediaId:string;workflowId:string;snapshot:any;reader:any;controlTarget?:HTMLElement|null;onRecovered:(id:string)=>void}){
 const w=useWorkspace(),owner=w.state.session.actorId,service=w.services.diagnosticsClient;
 const [pending,setPending]=useState<any>(null),[selection,setSelection]=useState<any>(null),[reason,setReason]=useState(''),[error,setError]=useState(''),[storageBlocked,setStorageBlocked]=useState(false),[busy,setBusy]=useState(false),[acknowledged,setAcknowledged]=useState(false),[review,setReview]=useState<any>(null);
 const active=useRef(false),trigger=useRef<HTMLButtonElement|null>(null),scope=useRef(mediaId),formId=useId();scope.current=mediaId;
 const live=()=>active.current&&w.state.session?.actorId===owner&&scope.current===mediaId;
 const command=useMemo(()=>persistentCommand<any,any>({key:`fluctlight-media:${owner}:recovery:${mediaId}:command`,storage:studioDraftStorage,preAdmissionConflicts:['revision_conflict','media_result_already_ready','media_not_retryable','media_execution_active'],execute:async(value:any,key:string)=>{
  if(w.state.session?.actorId!==owner)throw Error('会话已切换，请回原账号核对');
  const receipt=await service.mediaRetry(value.id,{...value.body,idempotencyKey:key});
  if(!receipt||receipt.media_intent_id!==value.id||receipt.mode!==value.body.mode||typeof receipt.intent_id!=='string'||!receipt.intent_id||typeof receipt.workflow_id!=='string'||!receipt.workflow_id)throw Error('恢复回执尚未完整读到，原提交身份已保留，请核对或重试原提交。');
  return receipt;
 }}),[owner,mediaId,service,w]);
 useEffect(()=>{active.current=true;try{setPending(command.pending());setStorageBlocked(false);}catch(cause){setStorageBlocked(true);setError(errorMessage(cause));}return()=>{active.current=false;};},[command]);
 const record=reader.items.find((row:any)=>row.id===mediaId&&row.data?.id===mediaId),media=record?.data;
 const isCurrent=!!media&&workflowKey(media.workflow_id)===workflowKey(workflowId);
 const studio=!!media?.studio_job_id,visual=mediaPurpose(media?.prompt)==='visual_identity';
 const settled=['failed','dead_letter'].includes(snapshot?.intent?.status);
 const eligible=isCurrent&&!studio&&!visual&&media.status==='failed'&&settled&&Number.isSafeInteger(media.revision)&&media.revision>0;
 const fresh=reader.ready&&!reader.busy&&!reader.error&&!!snapshot;
 async function submit(retry=false){
  if(busy||!live()||storageBlocked||!retry&&(!selection||!reason.trim()))return;
  setBusy(true);setError('');
  try{
   if(!retry&&command.pending())return;
   const value=retry?command.pending()?.value:{id:mediaId,body:{expectedRevision:selection.revision,mode:selection.mode,reason:reason.trim()}};
   const result=await(retry?command.retry():command.submit(value));
   if(!live())return;
   setSelection(null);setReason('');setAcknowledged(true);setPending(null);
   onRecovered(result.intent_id);
  }catch(cause){if(live())setError(errorMessage(cause));}
  finally{if(live()){try{setPending(command.pending());}catch(cause){setStorageBlocked(true);setError(errorMessage(cause));}setBusy(false);}}
 }
 async function openStudio(){
  if(busy||!media?.studio_job_id)return;setBusy(true);setError('');
  try{const job=await w.services.studioClient.job(media.studio_job_id);if(!live())return;if(!job?.resource_id)throw Error('创作任务没有返回所属资源，请重新读取。');w.navigatePath('/studio/preview?res='+encodeURIComponent(job.resource_id)+'&job='+encodeURIComponent(media.studio_job_id));}
  catch(cause){if(live())setError(errorMessage(cause));}finally{if(live())setBusy(false);}
 }
 async function reviewRejected(){
  if(busy||!pending?.rejection)return;setBusy(true);setError('');
  try{
   const page=await service.page('media-prompts',{correlationId:'media:'+mediaId,limit:20});
   if(!live())return;
   if(!Array.isArray(page.items)||!page.items.some((row:any)=>row.id===mediaId&&row.data?.id===mediaId))throw Error('没有读到原媒体任务，请重新读取。');
   reader.adopt(page);setReview({key:pending.idempotencyKey});
  }catch(cause){if(live())setError(errorMessage(cause));}finally{if(live())setBusy(false);}
 }
 const currentIntent=media?.execution?.intent_id;
 const controls=<div className="replica-task-recovery-controls">
  {reader.busy&&!reader.ready&&<Feedback kind="progress">正在读取媒体任务及恢复状态…</Feedback>}
  {reader.error&&<Feedback kind={reader.ready?'warning':'error'} actions={<Button type="button" variant="outline" disabled={busy} onClick={()=>void reader.load()}>重新读取媒体任务</Button>}>{reader.error}</Feedback>}
  {error&&!selection&&<Feedback kind={pending?'action-required':'warning'}>{error}</Feedback>}
  {pending?<Feedback kind={pending.rejection?'warning':'action-required'}><p>原恢复操作的输入和身份已保留，请先核对原提交。</p>{pending.rejection?<Button type="button" variant="outline" disabled={busy} onClick={()=>void reviewRejected()}>核对新状态并审阅原提交</Button>:<Button type="button" disabled={busy} onClick={()=>void submit(true)}>核对或重试原提交</Button>}</Feedback>:<>
   {eligible&&<div className="flex flex-wrap gap-3"><Button type="button" disabled={busy||!fresh||acknowledged||storageBlocked} onClick={event=>{trigger.current=event.currentTarget;setSelection({mode:'resume',revision:media.revision});setReason('');}}>从失败处重试</Button><Button type="button" variant="outline" disabled={busy||!fresh||acknowledged||storageBlocked} onClick={event=>{trigger.current=event.currentTarget;setSelection({mode:'regenerate',revision:media.revision});setReason('');}}>重新生成</Button></div>}
   {acknowledged&&<Feedback kind="progress">恢复请求已受理，正在等待实际执行结果。</Feedback>}
   {media&&!isCurrent&&currentIntent&&<Button type="button" variant="outline" onClick={()=>onRecovered(currentIntent)}>查看当前执行任务</Button>}
   {media&&isCurrent&&studio&&<><p className="replica-inbox-caption">此媒体属于创作任务，请在原创作任务中处理。</p><Button type="button" variant="outline" disabled={busy} onClick={()=>void openStudio()}>查看创作任务</Button></>}
   {media&&isCurrent&&visual&&media.owner_fluctlight_id&&<><p className="replica-inbox-caption">此媒体属于视觉身份会话，请查看原会话状态。</p><Button type="button" variant="outline" onClick={()=>w.navigatePath('/c/'+encodeURIComponent(media.owner_fluctlight_id)+'/media/identity/edit')}>查看视觉身份会话</Button></>}
   {media&&isCurrent&&!studio&&!visual&&!eligible&&!acknowledged&&<><p className="replica-inbox-caption">{media.status==='failed'?'原媒体工作流尚未结束，或恢复所需的版本不可用；请先核对最新状态。':['pending','running','retry'].includes(media.status)?'媒体任务正在执行或等待，完成情况以实际结果为准。':'请核对媒体任务的实际状态。'}</p><Button type="button" variant="outline" disabled={busy||reader.busy} onClick={()=>void reader.load()}>重新读取媒体任务</Button></>}
   {reader.ready&&!media&&<Feedback kind="warning">没有读到对应的媒体任务，不能发起恢复。<Button type="button" variant="outline" onClick={()=>void reader.load()}>重新读取媒体任务</Button></Feedback>}
  </>}
 </div>;
 const dialogs=<>
  <WorkspaceDialog className="replica-overlay" busy={busy} open={!!selection} onOpenChange={open=>{if(!open&&!busy)setSelection(null);}} returnFocus={()=>trigger.current} title={modeNames[selection?.mode]||'恢复媒体任务'} description={selection?.mode==='resume'?'保留已保存的提示词、生成服务任务和结果，从可恢复的位置继续。':'保留原尝试记录，重新提交图片生成；已就绪的资产不会被覆盖。'} footer={<><Button type="button" variant="outline" disabled={busy} onClick={()=>setSelection(null)}>取消</Button><Button type="submit" form={formId} disabled={busy||!reason.trim()||!!pending}>确认{modeNames[selection?.mode]}</Button></>}>
   <form id={formId} className="grid gap-4" onSubmit={event=>{event.preventDefault();void submit();}}>{selection?.mode==='resume'&&!media?.provider_job_id&&<Feedback kind="warning">没有保存生成服务的任务编号，继续时可能重新提交。如果上次请求已被服务接收，可能产生重复生成。</Feedback>}{busy&&<Feedback kind="progress">正在提交恢复请求；受理不代表生成完成。</Feedback>}{error&&!busy&&<Feedback kind={pending?'action-required':'error'}>{error}</Feedback>}<Label htmlFor={formId+'-reason'}>恢复理由<Textarea id={formId+'-reason'} required maxLength={1000} value={reason} onChange={event=>setReason(event.target.value)}/></Label></form>
  </WorkspaceDialog>
  <WorkspaceDialog className="replica-overlay" open={!!review} onOpenChange={open=>!open&&setReview(null)} returnFocus={()=>trigger.current} title="核对原操作与最新状态" description="原操作已明确被拒绝。解除后可以根据最新媒体状态重新选择恢复方式。"><Button type="button" onClick={()=>{if(live()&&command.pending()?.idempotencyKey===review?.key){command.releaseRejected();setPending(null);setReview(null);setError('');}}}>已核对，解除被拒绝的原提交</Button></WorkspaceDialog>
 </>;
 return <>{controlTarget?createPortal(controls,controlTarget):controls}{dialogs}</>;
}
