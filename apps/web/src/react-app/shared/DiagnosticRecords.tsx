import {EmptyState} from "@/react-app/shared/Presentation";
import {Table,TableBody,TableCell,TableHead,TableHeader,TableRow} from '@/components/ui/table';
import {Tooltip,TooltipContent,TooltipProvider,TooltipTrigger} from '@/components/ui/tooltip';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Card,CardContent} from '@/components/ui/card';

function Hint({text,children}:{text:string;children:React.ReactNode}){return <Tooltip><TooltipTrigger asChild><span tabIndex={0} className="min-w-0 truncate">{children}</span></TooltipTrigger><TooltipContent className="max-w-96 break-words">{text}</TooltipContent></Tooltip>;}
export default function DiagnosticRecords({view}:{view:any}){
 const records=view.rows.value;
 return <TooltipProvider delayDuration={250}><section className="diagnostic-feed" data-testid="diagnostic-feed">
 {records.length?<><div className="diagnostic-table hidden sm:block"><Table><TableHeader className="sticky top-0 z-10 bg-card"><TableRow>{['时间','类型','角色','状态','用时','摘要','操作'].map(label=><TableHead key={label}>{label}</TableHead>)}</TableRow></TableHeader><TableBody>{records.map((record:any)=>{
 const full=view.recordTime(record),time=record.start?new Intl.DateTimeFormat('zh-CN',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false}).format(new Date(record.start)):'—';
 const summary=record.status==='pending'&&record.summary===`执行中 · ${record.durationLabel}`?'正在处理，结果尚未返回。':record.summary;
 return <TableRow key={record.id} id={'diag-record-'+record.id} data-testid={'diag-record-'+record.id} className="diagnostic-record diagnostic-row cursor-pointer" onClick={()=>view.selectRecord(record)}>
  <TableCell className="diagnostic-time"><Hint text={full}><time dateTime={record.start?new Date(record.start).toISOString():undefined}>{time}</time></Hint></TableCell>
  <TableCell className="diagnostic-kind"><Badge variant="secondary">{record.label}</Badge></TableCell>
  <TableCell className="diagnostic-character">{record.character||'尚未关联角色'}</TableCell>
  <TableCell className="diagnostic-state"><Badge variant={record.status==='error'?'destructive':'outline'}>{view.statusLabel(record.status)}</Badge></TableCell>
  <TableCell className="diagnostic-duration whitespace-nowrap text-right">{record.durationLabel}</TableCell>
  <TableCell className="diagnostic-summary-cell"><Hint text={summary}><span className="diagnostic-summary">{summary?.replace(/\*\*/g,'')}</span></Hint></TableCell>
  <TableCell className="diagnostic-actions"><RecordActions record={record} view={view}/></TableCell>
 </TableRow>;
 })}</TableBody></Table></div><div className="grid min-w-0 gap-3 sm:hidden" data-testid="diagnostic-mobile-cards">{records.map((record:any)=>{
 const time=record.start?new Intl.DateTimeFormat('zh-CN',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false}).format(new Date(record.start)):'—';
 const summary=record.status==='pending'&&record.summary===`执行中 · ${record.durationLabel}`?'正在处理，结果尚未返回。':record.summary;
 return <Card key={record.id}><CardContent className="grid min-w-0 gap-3 p-4"><div className="flex min-w-0 flex-wrap items-center justify-between gap-2"><Badge variant="secondary">{record.label}</Badge><Badge variant={record.status==='error'?'destructive':'outline'}>{view.statusLabel(record.status)}</Badge></div><p className="m-0 line-clamp-2 break-words text-sm">{summary}</p><div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground"><Hint text={view.recordTime(record)}><time>{time}</time></Hint><span>{record.durationLabel}</span>{record.character&&record.character!=='尚未关联角色'&&<span>{record.character}</span>}</div><div className="flex flex-wrap justify-end gap-2"><RecordActions record={record} view={view} mobile/></div></CardContent></Card>;
 })}</div></>:<EmptyState><p>{view.search?.value?.trim()||Object.values(view.filters?.value||{}).some(Boolean)?'没有符合筛选条件的调用。':'当前还没有这类调用记录。'}</p>{(view.search?.value?.trim()||Object.values(view.filters?.value||{}).some(Boolean))&&<Button type="button" variant="outline" data-testid="diag-empty-clear" onClick={view.clear}>清除筛选</Button>}</EmptyState>}
 </section></TooltipProvider>;
}
function RecordActions({record,view,mobile=false}:any){return <><Button type="button" variant="outline" size="sm" data-testid={'diag-open-'+record.id+(mobile?'-mobile':'')} aria-label={'查看'+record.label+'详情'} onClick={e=>{e.stopPropagation();view.selectRecord(record);}}>详情</Button>{record.status==='error'&&<Button type="button" variant="outline" size="sm" data-testid={'diag-retry-'+record.id+(mobile?'-mobile':'')} onClick={e=>{e.stopPropagation();view.retry(record);}}>查看恢复方式</Button>}</>;}
