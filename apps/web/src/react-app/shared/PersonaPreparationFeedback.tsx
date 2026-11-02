import {useEffect,useRef,useState} from 'react';
import {Button} from '@/components/ui/button';
import {Feedback} from './Presentation';
import {useWorkspace} from '../page';
import {errorMessage} from '@/v3/domain-presentation.mjs';

// The workspace command retains the original body/key and GET-only readback.
export default function PersonaPreparationFeedback({character}:{character:any}) {
  const workspace=useWorkspace(),scope=workspace.currentScope(),fid=character.id;
  const operation='persona-preparation:'+fid;
  const active=useRef(true),[busy,setBusy]=useState(false),[error,setError]=useState(''),[record,setRecord]=useState<any>(()=>workspace.state.operations?.find((entry:any)=>entry.fid===fid&&entry.operation===operation&&(entry.readbackPending||entry.outcomeUnknown))??null);
  useEffect(()=>{active.current=true;return()=>{active.current=false;};},[]);
  const live=()=>active.current&&workspace.currentScope()===scope;
  const preparation=character.detail?.persona_preparation;
  if(!preparation||preparation.status==='ready')return null;
  const failed=preparation.status==='failed',revision=character.detail?.current_revision;
  async function recover() {
    if(busy||!live())return;
    setBusy(true);setError('');
    try {
      if(record&&(record.readbackPending||record.outcomeUnknown)) {
        const before=workspace.current.value.detail;
        await record.run();
        if(!live())return;
        if(record.status==='error')setError(record.reason);
        else if(workspace.current.value.detail===before) {
          // Another page may own the original command's onSaved callback
          // and skip it after unmounting. Read this scope before clearing
          // recovery; a failed read must keep the accepted command GET-only.
          record.readbackPending=true;
          try {await workspace.detail();if(live())record.readbackPending=false;}
          catch(cause) {record.status='error';record.reason=errorMessage(cause);throw cause;}
        }
      } else {
        const next=workspace.command('聊天',operation,{expectedRevision:revision},
          (value:any,idempotencyKey:string)=>workspace.client.recoverPersonaPreparation(fid,{...value,idempotencyKey}),
          {success:'已继续准备人格资料',onSaved:async()=>{if(workspace.currentScope()===scope)await workspace.detail();}});
        setRecord(next);
        await next.completion;
      }
    } catch(cause) {if(live())setError(errorMessage(cause));}
    finally {if(live())setBusy(false);}
  }
  async function reread() {
    if(busy||!live())return;
    setBusy(true);setError('');
    try {await workspace.detail();}
    catch(cause) {if(live())setError(errorMessage(cause));}
    finally {if(live())setBusy(false);}
  }
  const taskId=preparation.id||preparation.intent_id||preparation.task_id;
  const chatPath='/c/'+encodeURIComponent(fid)+'/chat';
  return <Feedback aria-label="人格准备与恢复" kind={failed?'action-required':'progress'} title={failed?'人格资料准备失败':'人格资料正在准备'} actions={<>
    {preparation.can_recover&&<Button type="button" variant="outline" disabled={busy||!Number.isSafeInteger(revision)||character.status==='已退休'} onClick={()=>void recover()}>{busy?'处理中…':record?.readbackPending?'重新读取准备结果':record?.outcomeUnknown?'重试原准备请求':'继续准备人格'}</Button>}
    <Button type="button" variant="link" disabled={busy} onClick={()=>void reread()}>重新读取准备状态</Button>
    {taskId&&<Button variant="link" asChild><a href={'#/diag?view=tasks&taskId='+encodeURIComponent(taskId)+'&returnTo='+encodeURIComponent(chatPath)}>查看人格准备任务</a></Button>}
    {failed&&<Button variant="link" asChild><a href={'#/settings?section='+encodeURIComponent('模型用途')}>检查模型用途</a></Button>}
    <Button variant="link" asChild><a href={'#/c/'+encodeURIComponent(fid)+'/profile'}>查看角色资料</a></Button>
  </>}>
    {failed?'暂时无法聊天。请检查模型服务后继续准备人格，完成后再发送。':'准备完成后即可聊天。'}当前输入会保留。
    {error&&<p role="alert">{error}</p>}
  </Feedback>;
}
