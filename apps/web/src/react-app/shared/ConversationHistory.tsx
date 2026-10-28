import {useEffect,useRef,useState} from 'react';
import type {BrowserConversationDirectoryPage,BrowserConversationMessage} from '@fluctlight/browser-client';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {InlineFeedback} from './Presentation';
import LoadingState from './LoadingState';
import WorkspaceDialog from './WorkspaceDialog';
import {useWorkspace} from '../page';
import {userInstant} from '@/v3/presentation-time.mjs';
import {errorMessage} from '@/v3/domain-presentation.mjs';
type Session=BrowserConversationDirectoryPage['items'][number];
export default function ConversationHistory({onClose,busy=false,returnFocus,initialQuery='',initialConversationId='',initialMessageId=''}:{onClose:()=>void;busy?:boolean;returnFocus?:()=>HTMLElement|null;initialQuery?:string;initialConversationId?:string;initialMessageId?:string}){
 const w=useWorkspace(),character=w.current.value,scope=w.currentScope(),owner=w.state.session.actorId;
 const [sessions,setSessions]=useState<Session[]>([]),[rows,setRows]=useState<BrowserConversationMessage[]>([]),[selected,setSelected]=useState(''),[directoryCursor,setDirectoryCursor]=useState(''),[cursor,setCursor]=useState(''),[query,setQuery]=useState(initialQuery),[applied,setApplied]=useState(initialQuery),[loading,setLoading]=useState(false),[directoryLoading,setDirectoryLoading]=useState(false),[switching,setSwitching]=useState(false),[failure,setFailure]=useState(''),[directoryReady,setDirectoryReady]=useState(false);
 const alive=useRef(false),epoch=useRef(0),directoryEpoch=useRef(0),retryRead=useRef<(()=>void)|null>(null);
 const sourceMessage=useRef<HTMLDivElement|null>(null);
 const live=()=>alive.current&&w.currentScope()===scope;
 async function readMessages(id:string,search='',append=false,readCursor=append?cursor:'',messageId=''){
  const token=++epoch.current;setLoading(true);setFailure('');
  try{
   const anchor=messageId?await w.client.conversationMessage(id,messageId):null;
   if(!live()||token!==epoch.current)return;
   if(anchor&&(anchor.id!==messageId||anchor.conversationId!==id||!Number.isSafeInteger(anchor.sequence)))throw Error('来源消息与原会话不匹配，请重新核对来源。');
   const page=search?await w.client.searchConversationMessages(character.id,search,{limit:50,...(append?{cursor:readCursor}: {})}):await w.client.messages(id,anchor?anchor.sequence+1:append&&readCursor?Number(readCursor):undefined,50);
   if(!live()||token!==epoch.current)return;
   const items:BrowserConversationMessage[]=search?page.items:page.messages;
   if(anchor&&!items.some(row=>row.id===anchor.id))items.push(anchor);
   setRows(old=>append?[...old,...items.filter(row=>!old.some(value=>value.id===row.id))]:items);
   setCursor(String(search?page.nextCursor||'':page.nextBeforeSequence||''));
  }catch(cause){if(live()&&token===epoch.current){retryRead.current=()=>void readMessages(id,search,append,readCursor,messageId);setFailure(errorMessage(cause,'历史暂时无法读取，请重试。'));}}
  finally{if(live()&&token===epoch.current)setLoading(false);}
 }
 async function readDirectory(append=false,readCursor=append?directoryCursor:''){
  const token=++directoryEpoch.current;setDirectoryLoading(true);setFailure('');
  try{
   const page:BrowserConversationDirectoryPage=await w.client.conversationDirectory(character.id,{limit:30,...(append?{cursor:readCursor}:{})});
   if(!live()||token!==directoryEpoch.current)return;
   setSessions(old=>append?[...old,...page.items.filter(row=>!old.some(value=>value.conversation.id===row.conversation.id))]:page.items);setDirectoryCursor(page.nextCursor||'');setDirectoryReady(true);
   const sourceFound=initialConversationId&&page.items.some(row=>row.conversation.id===initialConversationId);
   if(!append||(!selected&&sourceFound)){
    const id=initialConversationId?(sourceFound?initialConversationId:''):page.currentConversationId||page.items[0]?.conversation.id||'';
    setSelected(id);
    if(id)void readMessages(id,initialQuery,false,'',initialMessageId);
    else{setRows([]);if(initialConversationId){retryRead.current=()=>void readDirectory();setFailure('来源会话尚未在当前角色目录中读到。请加载更多会话或重新读取；不会改用另一段对话。');}}
   }
  }catch(cause){if(live()&&token===directoryEpoch.current){retryRead.current=()=>void readDirectory(append,readCursor);setFailure(errorMessage(cause,'会话目录暂时无法读取，请重试。'));}}
  finally{if(live()&&token===directoryEpoch.current)setDirectoryLoading(false);}
 }
 useEffect(()=>{alive.current=true;void readDirectory();return()=>{alive.current=false;epoch.current++;directoryEpoch.current++;};},[scope]);
 useEffect(()=>{sourceMessage.current?.scrollIntoView?.({block:'center'});},[rows]);
 async function resume(){
  if(!selected||busy||switching)return;setSwitching(true);setFailure('');
  try{await w.client.selectDirectConversation(character.id,selected);const current=()=>w.isCurrent(owner,character.id);if(current())await w.openConversation(character.id,current);if(live())onClose();}
  catch(cause){if(live())setFailure(errorMessage(cause,'会话切换尚未确认，请重新读取。'));}
  finally{if(live())setSwitching(false);}
 }
 return <WorkspaceDialog open busy={switching} returnFocus={returnFocus} onOpenChange={open=>{if(!open&&!switching)onClose();}} wide title="那些聊过的日常" description={`${character.name}与你的会话记录。查看历史不会替换当前输入。`}>
  <form className="replica-history-search" onSubmit={event=>{event.preventDefault();if(loading||directoryLoading||!directoryReady||switching)return;const value=query.trim();setApplied(value);void readMessages(selected,value);}}><Input aria-label="搜索全部会话" placeholder="搜索这个角色的全部会话…" value={query} onChange={event=>setQuery(event.target.value)}/><Button type="submit" variant="outline" disabled={loading||directoryLoading||!directoryReady||switching}>搜索</Button></form>
  <div className="companion-history-layout"><div className="companion-history-list">{sessions.map(session=><Button variant="ghost" key={session.conversation.id} className={selected===session.conversation.id&&!applied?'active':''} disabled={switching} onClick={()=>{setSelected(session.conversation.id);setApplied('');void readMessages(session.conversation.id);}}><strong>{session.conversation.title||'私聊会话'}</strong><small>{userInstant(session.conversation.createdAt)}{session.unreadCount>0&&` · ${session.unreadCount} 条未读`}</small></Button>)}{directoryLoading&&<LoadingState label="正在读取会话列表…"/>}{!directoryLoading&&!sessions.length&&!failure&&<p>还没有会话记录。</p>}{directoryCursor&&<Button variant="outline" disabled={directoryLoading||switching} onClick={()=>void readDirectory(true)}>加载更多会话</Button>}</div>
   <div className="companion-history-reading">{loading&&<LoadingState label="正在读取历史消息…"/>}{rows.map(row=><div key={row.id} ref={row.id===initialMessageId?sourceMessage:undefined} aria-label={row.id===initialMessageId?'来源消息':undefined}><span>{row.id===initialMessageId?'来源消息 · ':''}{row.kind==='user'?'我':character.name} · {userInstant(row.createdAt)}</span><p>{row.text}</p></div>)}{!loading&&!rows.length&&!failure&&<p>当前范围没有匹配消息。</p>}{cursor&&<Button variant="outline" disabled={loading||switching} onClick={()=>void readMessages(selected,applied,true)}>加载更多消息</Button>}</div>
  </div>{failure&&<InlineFeedback role="alert"><p>{failure}</p><Button variant="outline" disabled={loading||directoryLoading} onClick={()=>retryRead.current?.()}>重新读取历史</Button></InlineFeedback>}
  {switching&&<p role="status">正在切换会话并读回消息，完成后返回聊天。</p>}{selected&&!applied&&<div className="replica-history-footer"><span>选择会话只用于阅读，当前草稿会保留。</span><Button disabled={busy||switching||loading} onClick={()=>void resume()}>{switching?'正在切换…':'继续这段对话'}</Button></div>}
 </WorkspaceDialog>;
}
