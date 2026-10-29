import {Button} from '@/components/ui/button';
import {Feedback} from './Presentation';
import {downloadCreationRecords} from '@/approved-ui/creation-recovery-state.mjs';
export default function CreationRecoveryFeedback({state,onReload,busy=false}){
 if(!state||['empty','complete'].includes(state.state))return null;
 return <Feedback kind={state.kind} title={state.title} className="library-recovery-feedback replica-feedback-compact" data-testid="creation-recovery-feedback" detailsLabel="技术详情"
  actions={<>{state.href&&<Button asChild variant="outline"><a href={state.href}>{state.actionLabel}</a></Button>}{onReload&&(state.blocked||state.state==='preparation')&&<Button type="button" variant="ghost" disabled={busy} onClick={onReload}>{busy?'正在读取…':state.blocked?'重新读取本机记录':'刷新准备状态'}</Button>}{state.blocked&&state.records?.length>0&&<Button type="button" variant="ghost" onClick={()=>downloadCreationRecords(state.records)}>下载原记录</Button>}</>}
  details={state.diagnostic&&<pre>{JSON.stringify(state.diagnostic,null,2)}</pre>}>
  {state.description}
 </Feedback>;
}
