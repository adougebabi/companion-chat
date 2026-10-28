import {Badge} from '../controls';
import {Button} from '@/components/ui/button';
import {userInstant} from '@/v3/presentation-time.mjs';
export default function ScheduleTimeline({items,onSelect,compact=false}:{items:any[];onSelect:(item:any,event?:React.MouseEvent<HTMLButtonElement>)=>void;compact?:boolean}){
 return <div className="lp-timeline" data-testid="schedule-list">{items.map((item,index)=>{
  const source=item._source||{},tone=source.action_status==='in_progress'?'current':source.action_status==='completed'?'done':'';
  const end=Number.isFinite(Date.parse(source.end_at))?new Intl.DateTimeFormat('zh-CN',{timeZone:item._schedule?.timezone||'Asia/Shanghai',hour:'2-digit',minute:'2-digit',hour12:false}).format(new Date(source.end_at)):'';
  return <div className="lp-timeline-item" key={source.id||index}><time className="lp-timeline-time" dateTime={source.start_at} title={userInstant(source.start_at)} aria-label={userInstant(source.start_at)}>{item.time||'时间待定'}</time><div className="lp-timeline-track"><span className={'lp-timeline-dot '+tone}/></div><Button variant="ghost" className={'lp-timeline-card '+tone} onClick={event=>onSelect(item,event)} data-testid={'schedule-'+item.time}><div className="lp-row lp-between"><h3>{item.text}</h3>{!compact&&<Badge variant="outline" className="lp-status" data-status={tone==='current'?'accepted':'neutral'}>{tone==='current'?'正在进行':tone==='done'?'已结束':source.status==='cancelled'?'已取消':'计划中'}</Badge>}</div><p>{item.time}{end?'—'+end:''}{source.location?'　·　'+source.location:''}</p></Button></div>;
 })}</div>;
}
