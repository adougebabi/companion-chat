import {useEffect,useRef,useState} from 'react';
import {ArrowRight,CheckCheck,ChevronRight,Leaf,ShieldCheck,Clock3,RefreshCw,MessageCircle,Image,BookOpen,Target,Users,CalendarDays,Package,TriangleAlert} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Tabs,TabsList,TabsTrigger} from '@/components/ui/tabs';
import {Surface,ChoiceSelect,Badge} from '../controls';
import {useWorkspace} from '../page';
import useWorkspaceRead from '../shared/useWorkspaceRead';
import useManagementView from '../shared/useManagementView';
import WorkspacePageHeader from '../shared/WorkspacePageHeader';
import WorkspaceDialog from '../shared/WorkspaceDialog';
import {EmptyState,InlineFeedback} from '../shared/Presentation';
import LoadingState from '../shared/LoadingState';
import JsonDetails from '../shared/JsonDetails';
import {notificationPath,relativeTime} from '@/approved-ui/presentation.mjs';
import {pendingCategory,pendingSummary} from '@/v3/pending-presentation.mjs';
import {readNotificationPreviews} from '@/approved-ui/notification-previews.mjs';
import {userInstant} from '@/v3/presentation-time.mjs';
import {errorMessage,stateLabel} from '@/v3/domain-presentation.mjs';
const kinds={capability:'能力请求',persona_proposal:'人格与身份提议',studio_apply:'待应用定义',failed_task:'服务异常',owner_request:'正在处理的请求'};
const sourceIcons={'conversation.reply.published':MessageCircle,'moment.published':MessageCircle,'moment.comment.created':MessageCircle,'moment.reaction.created':MessageCircle,'capability.requested':ShieldCheck,'owner.request.settled':CalendarDays,'media.asset.ready':Image,'world.npc.encountered':Users,'diary.generated':BookOpen,'goal.evaluated':Target};
const pendingIcons={capability:ShieldCheck,persona_proposal:ShieldCheck,studio_apply:Package,failed_task:TriangleAlert,owner_request:Clock3};
function NoticeIcon({kind}){const Icon=Object.hasOwn(sourceIcons,kind)?sourceIcons[kind]:Leaf;return <Icon size={19}/>;}
function PendingGroup({group,fid,onOpen}){
 const w=useWorkspace(),source=useRef(group),Icon=Object.hasOwn(pendingIcons,group.kind)?pendingIcons[group.kind]:ShieldCheck;
 const rows=useWorkspaceRead({identity:['pending',fid,group.kind],characterScoped:false,read:async cursor=>{
  const current=source.current;
  const g=current&&!cursor?current:(await w.services.operationsClient.pending(fid,group.kind,cursor)).groups.find(g=>g.kind===group.kind);
  source.current=null;
  if(!g||g.status!=='ok')throw Error(g?.message||'该来源尚未完整读到');
  return {items:g.items||[],next_cursor:g.next_cursor};
 }});
 return <section className="replica-pending-group"><h3>{kinds[group.kind]||group.kind} <Badge variant="secondary">{group.status==='ok'?group.count:'尚未读全'}</Badge></h3>{rows.items.map(row=><button className="replica-pending-card" key={row.id} onClick={()=>onOpen(row.href)}><Icon size={17}/><div><strong>{row.title||'查看待处理事项'}</strong><p>{stateLabel(row.status)} · {row.fluctlight_id?w.state.characters.find(c=>c.id===row.fluctlight_id)?.name||'角色资料待读取':'系统任务'}</p></div><ArrowRight size={14}/></button>)}<ReadFeedback source={rows}/>{rows.ready&&!rows.items.length&&<p className="replica-inbox-caption">当前没有这类事项。</p>}{rows.cursor&&<Button variant="ghost" disabled={rows.busy} onClick={rows.more}>读取更多{kinds[group.kind]}</Button>}</section>;
}
function ReadFeedback({source}){return <>{source.busy&&<LoadingState label="正在读取…"/>}{source.error&&<InlineFeedback role="alert"><p>{source.error}</p><Button variant="outline" onClick={source.retry}>重新读取原列表</Button></InlineFeedback>}</>;}
export default function InboxPage(){
 const w=useWorkspace(),api=w.services.operationsClient,owner=w.state.session?.actorId;
 const [preferences,setPreferences]=useManagementView('inbox',{mode:'all',fid:''}),mode=preferences.mode,fid=preferences.fid,setMode=mode=>setPreferences({mode}),setFid=fid=>setPreferences({fid});
 const [selected,setSelected]=useState(null),[busy,setBusy]=useState(false),[message,setMessage]=useState(''),[previews,setPreviews]=useState({}),[readIds,setReadIds]=useState([]),[pendingVersion,setPendingVersion]=useState(0);
 const trigger=useRef(null),active=useRef(true),scope=useRef(owner);scope.current=owner;
 const live=()=>active.current&&w.state.session?.actorId===owner&&scope.current===owner;
 useEffect(()=>{active.current=true;return()=>{active.current=false;};},[owner]);
 const notices=useWorkspaceRead({identity:['inbox-notices',fid],characterScoped:false,read:cursor=>api.notifications(cursor,false,fid)});
 const pending=useWorkspaceRead({identity:['inbox-pending',fid,pendingVersion],characterScoped:false,read:async()=>{const value=await api.pending(fid);return {items:value.groups.map(g=>({...g,id:g.kind})),next_cursor:''};}});
 const groups=pending.items,summary=pendingSummary(groups);
 useEffect(()=>{let current=true;void readNotificationPreviews(w.client,notices.items,()=>current&&live()).then(value=>{if(current&&live())setPreviews(value);});return()=>{current=false;};},[notices.items,owner]);
 const isRead=row=>!!row.read_at||readIds.includes(row.id);
 const rows=notices.items.filter(row=>(mode==='all'||!isRead(row))&&(!fid||row.fluctlight_id===fid));
 const scopedNotices=notices.items.filter(row=>!fid||row.fluctlight_id===fid);
 const unread=scopedNotices.filter(row=>!isRead(row)).length;
 const unreadTotal=!fid&&w.state.notificationCount?w.state.notificationCount.count:notices.ready?unread+(notices.cursor?'+':''):'待读取';
 const canMarkAll=w.state.notificationCount?w.state.notificationCount.count>0:unread>0;
 async function mark(row){try{const receipt=await w.markNotificationRead(row.id);if(live()){if(receipt?.read_at)setSelected(old=>old?.id===row.id?{...old,read_at:receipt.read_at}:old);setReadIds(old=>[...new Set([...old,row.id])]);if(w.state.notificationCountError)setMessage('已读已保存，未读计数暂未更新：'+w.state.notificationCountError);}}catch(cause){if(live())setMessage('通知可继续查看，已读状态暂未确认：'+errorMessage(cause));}}
 function open(row,event){trigger.current=event.currentTarget;setSelected(row);if(!isRead(row))void mark(row);}
 function goSource(href){const path=notificationPath(href);if(path)w.navigatePath(path+(path.includes('?')?'&':'?')+'returnTo='+encodeURIComponent('#/inbox'));else setMessage('这项记录没有可访问的来源链接，请重新读取或前往运行诊断核对。');}
 async function markAll(){if(busy||!notices.extra.as_of)return;setBusy(true);setMessage('');const ids=notices.items.map(row=>row.id),before=notices.extra.as_of;try{await w.markAllNotificationsRead(before);if(live()){setReadIds(old=>[...new Set([...old,...ids])]);await notices.load();if(w.state.notificationCountError)setMessage('已读已保存，未读计数暂未更新：'+w.state.notificationCountError);}}catch(cause){if(live())setMessage(errorMessage(cause));}finally{if(live())setBusy(false);}}
 return <div className="replica-inbox"><WorkspacePageHeader eyebrow="WORKSPACE / A LITTLE ATTENTION" title="有些事，想让你知道。" description="新的近况、需要决定的事，以及可以继续的准备。" actions={<Button variant="outline" disabled={busy||!canMarkAll||!notices.extra.as_of} onClick={markAll}><CheckCheck size={15}/>{busy?'正在确认已读…':'全部标为已读'}</Button>}/>
 <div className="replica-inbox-toolbar"><Tabs value={mode} onValueChange={setMode}><TabsList variant="line"><TabsTrigger value="all">全部通知 <Badge variant="secondary">{scopedNotices.length}{notices.cursor?"+":""}</Badge></TabsTrigger><TabsTrigger value="unread">未读 <Badge variant="secondary">{unreadTotal}</Badge></TabsTrigger></TabsList></Tabs><ChoiceSelect aria-label="通知角色范围" value={fid} onChange={e=>setFid(e.target.value)}><option value="">所有角色</option>{w.state.characters.map(c=><option value={c.id} key={c.id}>{c.name}</option>)}</ChoiceSelect><Button variant="ghost" aria-label="重新读取通知" onClick={()=>{void notices.load();void w.refreshNotificationCount();}} disabled={notices.busy}><RefreshCw size={15}/></Button></div>
 {w.state.notificationCountError&&!message&&<InlineFeedback role="status" variant="default"><p>未读计数暂未更新，保留上次已确认的数量。</p><Button variant="outline" disabled={w.state.notificationCountLoading} onClick={()=>void w.refreshNotificationCount()}>重新读取未读计数</Button></InlineFeedback>}
 {message&&<InlineFeedback role="status" variant="default">{message}</InlineFeedback>}
 <div className="replica-inbox-layout"><Surface className="replica-notice-panel"><ReadFeedback source={notices}/>{rows.map(row=><article className={'replica-notice'+(!isRead(row)?' unread':'')} key={row.id}><button className="replica-notice-main" onClick={event=>open(row,event)}><span className="replica-notice-icon" data-source-kind={row.source_kind}><NoticeIcon kind={row.source_kind}/></span><div><div className="replica-notice-title"><h3>{row.title||'新通知'}</h3>{!isRead(row)&&<span className="replica-unread-dot"/>}</div><p>{previews[row.id]||row.body}</p><small>{w.state.characters.find(c=>c.id===row.fluctlight_id)?.name||'工作区'} · <time dateTime={row.source_occurred_at||row.occurred_at||row.created_at} title={userInstant(row.source_occurred_at||row.occurred_at||row.created_at)}>{relativeTime(row.source_occurred_at||row.occurred_at||row.created_at)||userInstant(row.source_occurred_at||row.occurred_at||row.created_at)}</time></small></div><ChevronRight size={15}/></button><div className="replica-notice-actions">{isRead(row)?<span>已读</span>:<Button variant="ghost" size="sm" onClick={()=>void mark(row)}>标为已读</Button>}</div></article>)}{notices.ready&&!rows.length&&<EmptyState><h3>{mode==='unread'?'未读已经看完了':'暂时没有通知'}</h3><p>生活中的新近况，会继续出现在这里。</p>{mode==='unread'&&<Button variant="outline" onClick={()=>setMode('all')}>查看全部通知</Button>}</EmptyState>}{notices.cursor&&<Button variant="ghost" disabled={notices.busy} onClick={notices.more}>读取更多通知</Button>}<p className="replica-inbox-caption">显示已读取的通知；全部标为已读会标记本次服务端读取时点之前的全部工作区通知，包含其他角色。</p></Surface>
 <aside className="replica-inbox-aside"><Surface><div className="replica-inbox-section-heading"><h2>需要你决定</h2><Badge variant="secondary">{summary.attentionKnown?summary.attention:'尚未读全'}</Badge></div><ReadFeedback source={pending}/>{groups.filter(g=>pendingCategory(g.kind)==='attention'&&(g.status!=='ok'||g.count>0)).map(g=><PendingGroup key={fid+':'+pendingVersion+':'+g.kind} group={g} fid={fid} onOpen={goSource}/>)}{summary.attentionKnown&&!summary.attention&&<EmptyState><h3>眼下都安排好了</h3><p>没有等待你处理的事项。</p></EmptyState>}<p className="replica-inbox-caption">标为已读不会代替处理。完成对应事项后，它才会从这里移出。</p><Button variant="ghost" onClick={()=>setPendingVersion(v=>v+1)}>重新读取待处理来源</Button></Surface>{['progress','service'].filter(category=>groups.some(g=>pendingCategory(g.kind)===category&&(g.status!=='ok'||g.count>0))).map(category=><Surface key={category}><h2>{category==='progress'?'正在处理的请求':'服务状态'}</h2>{category==='service'&&<p className="replica-inbox-caption">后台运行异常由服务处理，可查看原任务与恢复方式。</p>}{groups.filter(g=>pendingCategory(g.kind)===category&&(g.status!=='ok'||g.count>0)).map(g=><PendingGroup key={fid+':'+pendingVersion+':'+g.kind} group={g} fid={fid} onOpen={goSource}/>)}</Surface>)}{w.current.value.id&&<Surface className="replica-inbox-note"><span className="replica-eyebrow">BACK TO EVERYDAY</span><h3>也可以先去看看她。</h3><p>日常会继续。需要恢复的原任务，会好好留在这里。</p><Button variant="outline" onClick={()=>w.navigate('聊天')}>和{w.current.value.name}聊聊 <ArrowRight size={13}/></Button></Surface>}</aside></div>
 <WorkspaceDialog open={!!selected} onOpenChange={open=>{if(!open)setSelected(null);}} title={selected?.title||'通知详情'} description="通知内容与正式来源。已读和处理结果分别记录。" returnFocus={()=>trigger.current} footer={<><Button variant="outline" onClick={()=>setSelected(null)}>返回通知</Button><Button onClick={()=>goSource(selected?.link)}>查看原始来源 <ArrowRight size={14}/></Button></>}>{selected&&<><p className="replica-inbox-body">{previews[selected.id]||selected.body}</p><p className="replica-inbox-caption">{userInstant(selected.source_occurred_at||selected.occurred_at||selected.created_at)}</p><JsonDetails value={selected} title="通知来源与标识"/></>}</WorkspaceDialog>
 </div>;
}
