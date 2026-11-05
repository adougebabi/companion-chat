import {Badge} from '../controls';
import {Button} from '@/components/ui/button';
import {planWindow,planWindowLabel,userInstant} from '@/v3/presentation-time.mjs';
import {stateLabel} from '@/v3/domain-presentation.mjs';
import useMinuteClock from './useMinuteClock';
export default function ScheduleTimeline({items,onSelect,compact=false}:{items:any[];onSelect:(item:any,event?:React.MouseEvent<HTMLButtonElement>)=>void;compact?:boolean}){
 const now=useMinuteClock();
 return <div className="lp-timeline" data-testid="schedule-list">{items.map((item,index)=>{
  const source=item._source||{},tone=source.action_status==='in_progress'?'current':source.action_status==='completed'?'done':'';
  const window=planWindow(source.start_at,source.end_at,now),windowLabel=planWindowLabel(window);
  const cancelled=['cancelled','rejected'].includes(source.status);
  const actionLabel=source.action_status?stateLabel(source.action_status):'执行尚未确认';
  const end=Number.isFinite(Date.parse(source.end_at))?new Intl.DateTimeFormat('zh-CN',{timeZone:item._schedule?.timezone||'Asia/Shanghai',hour:'2-digit',minute:'2-digit',hour12:false}).format(new Date(source.end_at)):'';
  return <div className="lp-timeline-item" key={source.id||index}><time className="lp-timeline-time" dateTime={source.start_at} title={userInstant(source.start_at)} aria-label={userInstant(source.start_at)}>{item.time||'时间待定'}</time><div className="lp-timeline-track"><span className={'lp-timeline-dot '+tone}/></div><Button variant="ghost" className={'lp-timeline-card '+tone+(window==='current'&&!cancelled?' plan-current':'')} onClick={event=>onSelect(item,event)} data-testid={'schedule-'+item.time}><div className="lp-row lp-between"><h3>{item.text}</h3>{!compact&&<Badge variant="outline" className="lp-status" data-status={source.action_status==='failed'?'failed':tone==='current'?'accepted':'neutral'}>{cancelled?stateLabel(source.status):actionLabel}</Badge>}</div><p>{item.time}{end?'—'+end:''}{source.location?'　·　'+source.location:''}</p><p className="lp-timeline-window">{cancelled?stateLabel(source.status):windowLabel}{compact&&!cancelled?' · '+actionLabel:''}</p></Button></div>;
 })}</div>;
}
