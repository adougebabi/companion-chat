import SemanticTree from './SemanticTree';
import {useEffect,useMemo,useSyncExternalStore} from 'react';
import {Button} from '@/components/ui/button';
import {Dialog,DialogContent,DialogTitle,DialogDescription} from '@/components/ui/dialog';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {createDomainCommand} from '@/state/domain-command.mjs';
import {useWorkspace} from '../page';
import RecordEditor from './RecordEditor';
import JsonDetails from './JsonDetails';
export default function CommandDialog({action,onClose}:any){
 const w=useWorkspace(),owner=w.state.session.actorId;
 const model=useMemo(()=>createDomainCommand({...action,owner,ownerCurrent:()=>w.state.session?.actorId===owner}),[owner,action.identity]);
 const state:any=useSyncExternalStore(model.subscribe,model.getSnapshot,model.getSnapshot);
 useEffect(()=>{model.start();return()=>model.dispose();},[model]);
 useEffect(()=>{if(state.finished&&!action.keepOpen)onClose();},[state.finished]);
 return <Dialog open onOpenChange={open=>!open&&onClose()}><DialogContent className="react-workspace max-h-[90dvh] overflow-y-auto"><DialogTitle>{action.title}</DialogTitle><DialogDescription>{action.description||'核对当前输入后提交。原提交结果未知时，会保留原内容与身份。'}</DialogDescription><div className="grid gap-4">{state.error&&<Alert variant="destructive"><AlertDescription>{state.error}</AlertDescription></Alert>}{state.notice&&<p role="status">{state.notice}</p>}{state.draft&&<form className="grid gap-4" onSubmit={event=>{event.preventDefault();void model.submit();}}>{action.fields?action.fields(state.draft,model.edit):<RecordEditor value={Object.fromEntries((action.editable||['reason']).map((key:string)=>[key,state.draft[key]]))} fieldLabels={action.labels||{reason:'操作理由'}} onChange={(value:any)=>model.edit(value)}/>}<div className="flex flex-wrap gap-3"><Button type="submit" disabled={state.busy||!!state.pending||state.readbackPending||state.needsRebase||state.finished}>{action.submitLabel||'确认提交'}</Button>{state.pending&&!state.pending.rejection&&<Button type="button" variant="outline" disabled={state.busy} onClick={()=>void model.submit(true)}>核对或重试原提交</Button>}{action.readLatest&&<Button type="button" variant="outline" disabled={state.busy||!!state.pending&&!state.pending.rejection} onClick={()=>void model.reviewLatest()}>读取最新版本并保留输入</Button>}{state.readbackPending&&<Button type="button" variant="outline" disabled={state.busy} onClick={()=>void model.finishReadback()}>只读恢复页面结果</Button>}</div></form>}{state.pending&&<JsonDetails value={state.pending.value} title="原提交内容"/>}{action.snapshot&&<JsonDetails value={action.snapshot} title="打开操作时的对象"/>}{state.review&&<section className="grid gap-3"><h3 className="font-medium">核对最新对象</h3><p className="text-sm">最新版本：{state.review.object.revision??state.review.object.current_revision??'以完整对象为准'}</p><SemanticTree value={state.review.object.identity||state.review.object.background||state.review.object}/><JsonDetails value={state.review.object} title="最新对象与版本"/><Button type="button" disabled={state.busy} onClick={()=>model.confirmLatest()}>已核对，采用最新基线</Button></section>}</div></DialogContent></Dialog>;
}
