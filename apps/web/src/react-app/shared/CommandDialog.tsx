import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
import {Feedback} from './Presentation';
import SemanticTree from './SemanticTree';
import {useEffect,useMemo,useSyncExternalStore,useRef} from 'react';
import {Button} from '@/components/ui/button';
import {Dialog,DialogContent,DialogTitle,DialogDescription} from '@/components/ui/dialog';
import {createDomainCommand} from '@/state/domain-command.mjs';
import {useWorkspace,afterPaint} from '../page';
import RecordEditor from './RecordEditor';
import JsonDetails from './JsonDetails';
import CommandSyncAction from './CommandSyncAction';
export default function CommandDialog({action,onClose}:any){
 const w=useWorkspace(),owner=w.state.session.actorId;
 const model=useMemo(()=>createDomainCommand({...action,owner,ownerCurrent:()=>w.state.session?.actorId===owner}),[owner,action.identity]);
 const state:any=useSyncExternalStore(model.subscribe,model.getSnapshot,model.getSnapshot);
 const reviewHeading=useRef<HTMLHeadingElement|null>(null);
 useEffect(()=>{if(state.review)void afterPaint(()=>{if(reviewHeading.current?.isConnected){reviewHeading.current.scrollIntoView({block:'nearest'});reviewHeading.current.focus({preventScroll:true});}});},[state.review]);
 const syncAvailable=action.readLatest&&(!action.canReviewLatest||action.canReviewLatest(state.draft));
 useEffect(()=>{model.start();return()=>model.dispose();},[model]);
 useEffect(()=>{if(state.finished&&!action.keepOpen)onClose();},[state.finished]);
 return <Dialog open onOpenChange={open=>!open&&onClose()}><DialogContent className="react-workspace max-h-[90dvh] overflow-y-auto"><DialogTitle>{action.title}</DialogTitle><DialogDescription>{action.description||'核对当前输入后提交。原提交结果未知时，会保留原内容与身份。'}</DialogDescription><div className="grid gap-4">{state.error&&<Feedback kind={state.readbackPending||state.pending&&!state.pending.rejection?"action-required":state.needsRebase||state.pending?.rejection?"warning":"error"}>{state.error}</Feedback>}{state.notice&&!state.error&&<OperationStatus kind="information">{state.notice}</OperationStatus>}{state.draft&&<form className="grid gap-4" onSubmit={event=>{event.preventDefault();void model.submit();}}>{action.fields?action.fields(state.draft,model.edit):<RecordEditor value={Object.fromEntries((action.editable||['reason']).map((key:string)=>[key,state.draft[key]]))} fieldLabels={action.labels||{reason:'操作理由'}} onChange={(value:any)=>model.edit(value)}/>}<div className="flex flex-wrap gap-3"><Button type="submit" disabled={state.busy||!!state.pending||state.readbackPending||state.needsRebase||state.finished}>{action.submitLabel||'确认提交'}</Button>{state.pending&&!state.pending.rejection&&<Button type="button" variant="outline" disabled={state.busy} onClick={()=>void model.submit(true)}>核对或重试原提交</Button>}{syncAvailable&&<><CommandSyncAction state={state} onReview={model.reviewLatest}/><CommandSyncAction state={state} onReview={model.reviewLatest} placement="recovery"/></>}{state.readbackPending&&<Button type="button" variant="outline" disabled={state.busy} onClick={()=>void model.finishReadback()}>只读恢复页面结果</Button>}</div></form>}{state.pending&&<JsonDetails value={state.pending.value} title="原提交内容"/>}{action.snapshot&&<JsonDetails value={action.snapshot} title="打开操作时的对象"/>}{state.review&&<section className="grid gap-3"><h3 ref={reviewHeading} tabIndex={-1} className="font-medium">核对最新对象</h3><p className="text-sm">最新版本：{state.review.object.revision??state.review.object.current_revision??'以完整对象为准'}</p><SemanticTree value={state.review.object.identity||state.review.object.background||state.review.object}/><JsonDetails value={state.review.object} title="最新对象与版本"/><Button type="button" disabled={state.busy} onClick={()=>model.confirmLatest()}>已核对，采用最新基线</Button></section>}</div></DialogContent></Dialog>;
}
