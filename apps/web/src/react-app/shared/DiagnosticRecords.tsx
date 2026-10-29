import {EmptyState} from './Presentation';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Code2,ChevronRight} from 'lucide-react';
export default function DiagnosticRecords({view}:{view:any}){
 const records=view.rows.value;
 return <section className="diagnostic-feed replica-call-list" data-testid="diagnostic-feed" aria-label="调用记录">
 {records.length?records.map((record:any)=><article key={record.id} id={'diag-record-'+record.id} data-testid={'diag-record-'+record.id} className="diagnostic-record replica-call-row" data-status={record.status}>
  <button type="button" className="replica-call-main" data-testid={'diag-open-'+record.id} aria-label={'查看'+record.label+'详情'} onClick={event=>view.selectRecord(record,event.currentTarget)}><span className="replica-task-icon"><Code2 size={17}/></span><span className="replica-call-content"><strong>{record.label}</strong><span className="replica-call-meta">{record.model||'模型未记录'} · {view.recordTime(record)} · {record.durationLabel}</span><span className="diagnostic-summary">{record.summary}</span><span className="replica-call-character">{record.character||'未记录角色来源'}</span></span><Badge variant={record.status==='error'?'destructive':record.status==='pending'?'secondary':'outline'}>{view.statusLabel(record.status)}</Badge><ChevronRight size={14}/></button>
  {record.status==='error'&&<div className="replica-call-actions"><Button type="button" variant="ghost" size="sm" data-testid={'diag-retry-'+record.id} onClick={()=>view.retry(record)}>查看原任务与恢复</Button></div>}
 </article>):<EmptyState><p>{view.search?.value?.trim()||Object.values(view.filters?.value||{}).some(Boolean)?'没有符合筛选条件的调用。':'当前还没有这类调用记录。'}</p>{(view.search?.value?.trim()||Object.values(view.filters?.value||{}).some(Boolean))&&<Button type="button" variant="outline" data-testid="diag-empty-clear" onClick={view.clear}>清除筛选</Button>}</EmptyState>}
 </section>;
}
