import {ArrowUpRightIcon,BookOpenIcon,Clock3Icon,HeartIcon,LeafIcon,MapPinIcon,MessageSquareIcon,SunIcon} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {EmptyMedia} from '@/components/ui/empty';
import {Surface} from '../controls';
import WorkspacePageHeader from '../shared/WorkspacePageHeader';
import ScheduleTimeline from '../shared/ScheduleTimeline';
import {scheduleItemDisplay} from '../shared/schedule-presentation.mjs';
import WearingSummary from '../shared/WearingSummary';
import {nowEmptyText,nowStateSummary,nowStateText,NowStateDownload} from '../shared/NowState';
import LoadingState from '../shared/LoadingState';
import {EmptyState,InlineFeedback,ReaderFeedback} from '../shared/Presentation';
import useWorkspaceRead from '../shared/useWorkspaceRead';
import {useWorkspace} from '../page';
import {day} from '@/state/workspace.mjs';
import {nearbyScheduleItems,nextScheduleItem} from '@/approved-ui/presentation.mjs';
import useMinuteClock from '../shared/useMinuteClock';
import {useEffect,useRef} from 'react';
export default function NowPage({view}){
 const w=useWorkspace(),c=view.character.value,now=useMinuteClock(),zone=view.schedule.value.find(item=>item._schedule?.timezone)?._schedule.timezone||c.timezone||'Asia/Shanghai',today=day(new Date(now),zone);
 const previousDay=useRef(today);
 useEffect(()=>{if(previousDay.current!==today){previousDay.current=today;void w.loadLife({currentOnly:true}).catch(()=>{});}},[today,w]);
 const contextWorldId=c.detail?.context?.world_id;
 const worldRead=useWorkspaceRead({identity:['now-world',c.id,contextWorldId],enabled:!!c.id&&typeof w.client?.world==='function',read:async()=>{const result=await w.client.world(c.id);return {items:[],next_cursor:'',world:result.world||null};}});
 const world=worldRead.extra.world;
 const worldId=worldRead.ready?world?.id:contextWorldId;
 const hasWorld=typeof worldId==='string'&&!!worldId.trim();
 const worldName=nowStateText(worldRead.ready?world?.name:c.world,'世界');
 const {activity,location,mood}=nowStateSummary(c);
 const hasState=!!(activity||location||mood);
 const todayItems=view.schedule.value.filter(item=>item._schedule?.local_date===today||(Number.isFinite(Date.parse(item._source?.start_at))&&day(item._source.start_at,zone)===today));
 const todaySummary=nearbyScheduleItems(todayItems,now).map(item=>({
  ...item,text:nowStateText(item.text,'活动'),
  _source:{...item._source,location:nowStateText(item._source?.location,'地点')},
 }));
 const next=nextScheduleItem(view.schedule.value.filter(item=>!scheduleItemDisplay(item).unplanned),now),readingSchedule=!!(w.state.loading.life||w.state.loading.schedule);
 const nextDisplay=next&&scheduleItemDisplay(next);
 const dateParts=Object.fromEntries(new Intl.DateTimeFormat('zh-CN',{timeZone:zone,month:'numeric',day:'numeric',weekday:'long'}).formatToParts(new Date(now)).map(part=>[part.type,part.value]));
 const date=`${dateParts.month} 月 ${dateParts.day} 日　${dateParts.weekday}`;
 const go=page=>view.emit('go',page);
 return <div className="replica-life replica-now"><WorkspacePageHeader eyebrow="LIFE / RIGHT NOW" title={c.name+'的此刻'} description="生活正在发生。这里收着今天的状态和接下来的小事。" actions={<Button variant="outline" onClick={()=>go('聊天')}><MessageSquareIcon size={15}/>和{c.name}聊聊</Button>}/>
  <div className="lp-grid"><div className="lp-stack">
   <section className="lp-hero">
    <div>
     <div className="lp-row lp-day-stamp"><span>{date}</span></div>
     {!hasState&&(w.state.loading.detail?<LoadingState label="正在读取此刻状态…"/>:<h2>{nowEmptyText}</h2>)}
     {activity&&<h2>{activity}</h2>}
     {(location||mood)&&<div className="lp-row">
      {location&&<span className="lp-chip"><MapPinIcon size={12}/>{location}</span>}
      {mood&&<span className="lp-chip"><LeafIcon size={12}/>{mood}</span>}
     </div>}
    </div>
    <div className="lp-hero-art" aria-hidden="true"><BookOpenIcon/></div>
   </section>
   <Surface className="lp-pad"><div className="replica-section-heading"><div><h2>今天，慢慢走</h2><p>已确认的安排与即将到来的片刻</p></div><Button variant="link" onClick={()=>go('日程')}>完整日程<ArrowUpRightIcon size={12}/></Button></div>
    {readingSchedule&&<LoadingState label="正在读取日程…"/>}{view.readError('schedule')&&<InlineFeedback role="alert">日程读取未完成，保留已确认的安排。{view.readError('schedule')}<Button variant="outline" onClick={()=>void w.loadLife({currentOnly:true}).catch(()=>{})}>重新读取日程</Button></InlineFeedback>}
    {todayItems.length?<ScheduleTimeline items={todaySummary} compact onSelect={()=>go('日程')}/>:!readingSchedule&&!view.readError('schedule')&&<EmptyState className="replica-now-empty"><EmptyMedia><LeafIcon size={25} aria-hidden="true"/></EmptyMedia><h3>今天的生活，正在慢慢展开</h3><p>还没有确认的日程，可以先聊聊接下来的安排。</p><Button variant="outline" onClick={()=>go('日程')}>打开日程</Button></EmptyState>}
   </Surface>
  </div><div className="lp-stack"><Surface className="lp-pad"><div className="replica-section-heading"><h2>生活切片</h2></div>
   {hasWorld&&<div className="lp-runtime-item"><span className="lp-icon-box"><MapPinIcon size={17}/></span><div><span className="lp-muted">所在世界</span>{worldName&&<strong className="lp-title">{worldName}</strong>}<Button variant="link" onClick={()=>go('世界')}>看看这个世界<ArrowUpRightIcon size={12}/></Button></div></div>}
   <ReaderFeedback source={worldRead} label="所属世界"/>
   {mood&&<div className="lp-runtime-item"><span className="lp-icon-box"><HeartIcon size={17}/></span><div><span className="lp-muted">当前情绪</span><strong className="lp-title">{mood}</strong><span className="lp-muted">随相处和生活自然变化</span></div></div>}
   <div className="lp-runtime-item"><span className="lp-icon-box"><SunIcon size={17}/></span><div><span className="lp-muted">今天的穿着</span><WearingSummary character={c} error={view.readError('wardrobe')} loading={w.state.loading.wardrobe}/><Button variant="link" onClick={()=>go('衣柜')}>打开衣柜<ArrowUpRightIcon size={12}/></Button></div></div>
   <NowStateDownload detail={c.detail} world={world} schedule={view.schedule.value.length?view.schedule.value.map(item=>item._source):undefined}/>
  </Surface><Surface className="lp-pad"><div className="lp-row lp-between"><span className="lp-label">NEXT LITTLE THING</span><Clock3Icon size={15}/></div><strong className="replica-next-time">{(nextDisplay?.showTime ? next.time : nextDisplay?.timeNote)||'待安排'}</strong><p className="lp-title">{nowStateText(next?.text,'活动')||'接下来的安排尚未确认。'}</p><p className="lp-muted replica-next-note">未来的安排仍会变化，生活留一点余地。</p></Surface>
  <Button variant="ghost" className="lp-memory-card" onClick={()=>go('日记')}><div className="lp-row"><BookOpenIcon size={16}/><span className="lp-title">{c.name}写下的日常</span><ArrowUpRightIcon size={14}/></div><p className="lp-small">翻开日记，看看没有说出口的日常。</p></Button>
 </div></div></div>;
}
