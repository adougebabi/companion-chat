import {EmptyState} from './Presentation';
import {Button} from '@/components/ui/button';
import {PageSection} from '@/components/patterns';
import {taskModelFailureReason} from '@/approved-ui/task-presentation.mjs';
import {Code2} from 'lucide-react';
import DiagnosticStatus from './DiagnosticStatus';
import {callDurationLabel,recordedCallValue,callValueLabel,callStageLabel,callPurpose,modelCallGroups} from '@/approved-ui/task-model-call-presentation.mjs';
import '../../styles/replica-diagnostic-calls.css';
export default function DiagnosticRecords({view}:{view:any}){
 const records=view.rows.value;
 const byRecord=new Map(records.map((record:any)=>[record._record,record]));
 const groups=modelCallGroups(records.map((record:any)=>record._record));
 const history=!!view.diagnosticQuery?.value,filtered=history||view.search?.value?.trim()||Object.values(view.filters?.value||{}).some(Boolean);
 const renderCall=(record:any,grouped=false)=><article key={record.id} id={'diag-record-'+record.id} data-testid={'diag-record-'+record.id}>
  <button type="button" className="replica-model-call-row" data-testid={'diag-open-'+record.id} aria-label={'查看'+record.label+'详情'} onClick={event=>view.selectRecord(record,event.currentTarget)}><span className="replica-model-call-icon"><Code2 size={16}/></span><span className="replica-model-call-content"><strong>{grouped?[(Number.isInteger(recordedCallValue(record._record?.data||{},'sequence'))?'调用 '+recordedCallValue(record._record.data,'sequence'):'调用序号未记录'),callStageLabel(record._record?.data||{})].join(' · '):record.label}</strong><span>{!grouped&&<>{record.character} · </>}{callValueLabel(recordedCallValue(record._record?.data||{},'model_id'),'模型未记录')} · {view.recordTime(record)} · {callDurationLabel(record._record?.data||{},{includeSource:false})}</span></span><DiagnosticStatus status={record._record?.data?.status} label={record._record?.data?.status==='timeout'?'超时':record._record?.data?.status==='cancelled'?'已取消':view.statusLabel(record.status)} tone={record._record?.data?.status==='cancelled'?'neutral':record.status==='error'?'red':record.status==='success'?'green':record.status==='pending'?'blue':'neutral'}/></button>{grouped&&['failed','error','dead_letter','timeout'].includes(record._record?.data?.status)&&<p className="text-sm text-destructive mb-3">{taskModelFailureReason(record._record.data)}</p>}
 </article>;
 return <section className="replica-model-call-list" data-testid="diagnostic-feed" aria-label="调用记录">
 {records.length?groups.map((group:any)=>{
  const first:any=byRecord.get(group.records[0]);
  if(!group.associated)return renderCall(first);
  return <PageSection key={group.key} title={callPurpose(group.records[0].data,first.label).label} description={[first.character,view.recordTime(first),group.summary].filter(Boolean).join(' · ')} className="mb-5" >
   <div data-model-call-group="" aria-label={callPurpose(group.records[0].data,first.label).label+'调用过程'}>{group.records.map((record:any)=>renderCall(byRecord.get(record),true))}</div>
   {group.repeatedSequence&&<p className="text-sm text-muted-foreground">存在重复调用序号；各次记录分别保留，请结合时间与来源核对。</p>}
   {group.missingTime&&<p className="text-sm text-muted-foreground">部分调用时间未记录，先后顺序需结合原始记录核对。</p>}
  </PageSection>;
 }):<EmptyState><p>{filtered?'没有符合筛选条件的调用。':'当前还没有这类调用记录。'}</p>{filtered&&<Button type="button" variant="outline" data-testid="diag-empty-clear" onClick={()=>{if(history){view.historySearch.value='';void view.searchHistory();}else view.clear();}}>清除筛选</Button>}</EmptyState>}
 {groups.some((group:any)=>group.associated)&&<p className="text-xs text-muted-foreground">统计仅含当前已读取结果。历史重试分别保留，任务结果以任务状态为准。</p>}
 </section>;
}
