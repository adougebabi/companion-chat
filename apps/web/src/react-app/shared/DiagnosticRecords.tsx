import {EmptyState} from './Presentation';
import {Button} from '@/components/ui/button';
import {Code2} from 'lucide-react';
import DiagnosticStatus from './DiagnosticStatus';
import {callDurationLabel,recordedCallValue,callValueLabel} from '@/approved-ui/task-model-call-presentation.mjs';
import '../../styles/replica-diagnostic-calls.css';
export default function DiagnosticRecords({view}:{view:any}){
 const records=view.rows.value;
 const history=!!view.diagnosticQuery?.value,filtered=history||view.search?.value?.trim()||Object.values(view.filters?.value||{}).some(Boolean);
 return <section className="replica-model-call-list" data-testid="diagnostic-feed" aria-label="调用记录">
 {records.length?records.map((record:any)=><article key={record.id} id={'diag-record-'+record.id} data-testid={'diag-record-'+record.id}>
  <button type="button" className="replica-model-call-row" data-testid={'diag-open-'+record.id} aria-label={'查看'+record.label+'详情'} onClick={event=>view.selectRecord(record,event.currentTarget)}><span className="replica-model-call-icon"><Code2 size={16}/></span><span className="replica-model-call-content"><strong>{record.label}</strong><span>{record.character} · {callValueLabel(recordedCallValue(record._record?.data||{},'model_id'),'模型未记录')} · {view.recordTime(record)} · {callDurationLabel(record._record?.data||{},{includeSource:false})}</span></span><DiagnosticStatus status={record._record?.data?.status} label={record._record?.data?.status==='timeout'?'超时':record._record?.data?.status==='cancelled'?'已取消':view.statusLabel(record.status)} tone={record.status==='error'?'red':record.status==='success'?'green':record.status==='pending'?'blue':'neutral'}/></button>
 </article>):<EmptyState><p>{filtered?'没有符合筛选条件的调用。':'当前还没有这类调用记录。'}</p>{filtered&&<Button type="button" variant="outline" data-testid="diag-empty-clear" onClick={()=>{if(history){view.historySearch.value='';void view.searchHistory();}else view.clear();}}>清除筛选</Button>}</EmptyState>}
 </section>;
}
