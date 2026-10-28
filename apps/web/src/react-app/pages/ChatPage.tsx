import {MessageScrollerProvider,MessageScroller,MessageScrollerViewport,MessageScrollerContent,MessageScrollerItem,MessageScrollerButton} from '@/components/ui/message-scroller';
import {Message,MessageContent,MessageHeader} from '@/components/ui/message';
import {Bubble,BubbleContent} from '@/components/ui/bubble';
import {AttachmentGroup} from '@/components/ui/attachment';
import ChatAttachment from '../shared/ChatAttachment';
import WorkspaceDialog from '../shared/WorkspaceDialog';
import MediaPreview from '../shared/MediaPreview';
import {Marker,MarkerContent} from '@/components/ui/marker';
import {Spinner} from '@/components/ui/spinner';
import WorkspacePageHeader from '../shared/WorkspacePageHeader';
import WorkspaceMomentArt from '../shared/WorkspaceMomentArt';
import WearingSummary from '../shared/WearingSummary';
import {nextScheduleItem} from '@/approved-ui/presentation.mjs';
import {currentReadError} from '@/approved-ui/read-feedback.mjs';
import ConversationHistory from '../shared/ConversationHistory';
import {Surface} from '../controls';
import {Select,SelectContent,SelectItem,SelectTrigger,SelectValue} from '@/components/ui/select';
import {SidebarProvider,Sidebar,SidebarContent} from '@/components/ui/sidebar';
import {InlineFeedback} from "@/react-app/shared/Presentation";
import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import ChatTurnFeedback from "../components/ChatTurnFeedback";
import {
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  HistoryIcon,SlidersHorizontalIcon,MapPinIcon,CheckCheckIcon,
  SendIcon,
  ShieldCheckIcon,
  ChevronRightIcon,
  LeafIcon, XIcon,
} from "lucide-react";
import { useWorkspace } from "../page";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import CharacterAvatar from "../shared/CharacterAvatar";
import { DetailDrawer } from "../components";
import {
  chatWaitingPresentation,
  chatDiagnosticTurn,
  chatFailureText,
} from "@/v3/chat-feedback.mjs";
import {errorMessage} from '@/v3/domain-presentation.mjs';
import { userInstant } from "@/v3/presentation-time.mjs";
import {approvedRoute} from "@/approved-ui/routing.mjs";
import DomainPage from "./DomainPage";
import type { ConversationController } from "@/state/conversation";
function messageDay(value:string|undefined){const at=Date.parse(value||'');return Number.isFinite(at)?new Intl.DateTimeFormat('zh-CN',{timeZone:'Asia/Shanghai',dateStyle:'long'}).format(new Date(at)):'日期未记录';}
export default function ChatPage({
  preview,
  onGo,
  onPreviewChange,
  historyRoute,
  route,
}: {
  preview: boolean;
  historyRoute?: any;
  route?: any;
  onPreviewChange?: (open:boolean)=>void;
  onGo: (page: string) => void;
}) {
  const workspace = useWorkspace(),
    store: ConversationController = workspace.conversation();
  const snapshot = useSyncExternalStore(
    store.subscribe,
    store.getSnapshot,
    store.getSnapshot,
  );
  useSyncExternalStore(
    workspace.subscribe,
    workspace.getSnapshot,
    workspace.getSnapshot,
  );
  const character = workspace.current.value,
    scope = workspace.currentScope();
  const storageKey = "fluctlight:approved-chat-draft:" + scope;
  const [draft, setDraft] = useState(() => {
    try {
      return sessionStorage.getItem(storageKey) || "";
    } catch {
      return "";
    }
  });
  const [diagnosticRoute,setDiagnosticRoute]=useState<any>(null);
  const [now, setNow] = useState(Date.now);
  const [historyOpen,setHistoryOpen]=useState(false);
  const [promptOffer,setPromptOffer]=useState('');
  const [readFailure,setReadFailure]=useState(''),[readLoading,setReadLoading]=useState(true);
  const latestDraft = useRef(draft);
  latestDraft.current = draft;
  const incomingPrompt=route?.query?.get('say')||'';
  useEffect(()=>{
    setPromptOffer('');
    if(!incomingPrompt.trim()||(route?.fid&&route.fid!==character.id))return;
    const current=latestDraft.current;
    if(!current.trim()||current.trim()===incomingPrompt.trim())setDraft(incomingPrompt);
    else setPromptOffer(incomingPrompt);
  },[incomingPrompt,route?.fid,character.id,scope]);
  const pageActive=useRef(false),historyFocus=useRef<HTMLButtonElement>(null);
  const composing = useRef(false),
    attachmentFocus = useRef<HTMLButtonElement>(null);
  const [attachment,setAttachment]=useState<string|null>(null);
  function closeHistory(){
    setHistoryOpen(false);
    if(historyRoute){
      const target=historyRoute.query.get('returnTo');
      workspace.navigatePath(/^#?\/(?!\/)/.test(target||'')?target.replace(/^#/,''):'/c/'+encodeURIComponent(character.id)+'/chat');
    }
  }
  const busy =
    snapshot.sending || store.hasPendingTurn || store.hasSettlingTurn;
  useEffect(() => {
    try {
      sessionStorage.setItem(storageKey, draft);
    } catch {
      /* Mounted draft remains available. */
    }
  }, [draft, storageKey]);
  useEffect(() => {
    pageActive.current=true;
    let active = true,
      polling = false,
      lastIdleRead = 0,
      lastSummaryRead = 0;
    const live = () => active && workspace.currentScope() === scope;
    setReadLoading(true);setReadFailure('');
    void workspace.openConversation(character.id, live)
      .catch((cause: unknown) => { if(live())setReadFailure(errorMessage(cause,'对话暂时无法读取。')); })
      .finally(()=>{if(live())setReadLoading(false);});
    const timer = setInterval(async () => {
      if (!live() || document.hidden || polling) return;
      setNow(Date.now());
      polling = true;
      try {
        if (store.hasPendingTurn || store.hasSettlingTurn)
          await store.refreshActiveTurn(true);
        else if (
          !store.sending &&
          !composing.current &&
          !latestDraft.current.trim() &&
          Date.now() - lastIdleRead > 10000
        ) {
          lastIdleRead = Date.now();
          await store.refreshActiveTurn(true);
          if (live() && Date.now() - lastSummaryRead > 30000) {
            lastSummaryRead = Date.now();
            await workspace.detail();
            if (live()) await workspace.loadWardrobe();
          }
        }
      } catch (error) {
        if (live())
          setReadFailure(errorMessage(error,"读取失败"));
      } finally {
        polling = false;
      }
    }, 2000);
    return () => {
      active = false;pageActive.current=false;
      clearInterval(timer);
      if (store.fluctlightId === character.id) store.invalidateRequest();
    };
  }, [workspace, store, scope, character.id]);
  const messages =
    snapshot.fluctlightId === character.id
      ? snapshot.messages.filter(
          (message) =>
            message.kind !== "system",
        )
      : [];
  async function send() {
    const submitted = latestDraft.current,
      text = submitted.trim();
    if (!text || busy || readLoading || snapshot.fluctlightId!==character.id || !snapshot.conversation) return;
    await store.send(text, false, null, () => {
      if (
        pageActive.current && workspace.currentScope() === scope &&
        latestDraft.current === submitted
      ) {
        setDraft("");
        sessionStorage.removeItem(storageKey);
      }
    });
  }
  const turn = chatDiagnosticTurn(store),
    diagnosticHref = turn
      ? "#/diag/turns?correlationId=" + encodeURIComponent("turn:" + turn)
      : "#/diag";
  const openDiagnostic=(event:any)=>{event.preventDefault();setDiagnosticRoute(approvedRoute(diagnosticHref+(diagnosticHref.includes('?')?'&':'?')+'returnTo='+encodeURIComponent(location.hash)));};
  const pending = messages.findLast(
    (message) =>
      message.kind === "user" &&
      ["pending", "running"].includes(message.turnStatus || ""),
  );
  const waiting = chatWaitingPresentation(
    store,
    snapshot.attemptStartedAt ??
      (pending?.createdAt ? Date.parse(pending.createdAt) : now),
    now,
    character.name,
  );
  const error =
    snapshot.fluctlightId === character.id && snapshot.error
      ? chatFailureText(
          snapshot.error,
          messages.some(
            (message) =>
              message.kind === "assistant" && message.turnId === turn,
          ),
        )
      : readFailure;
  async function reread(){
    setReadLoading(true);setReadFailure('');
    const live=()=>pageActive.current&&workspace.currentScope()===scope;
    try{if(!store.conversation||store.fluctlightId!==character.id)await workspace.openConversation(character.id,live);else await store.refreshActiveTurn(true);}
    catch(cause){if(live())setReadFailure(errorMessage(cause,'对话暂时无法读取。'));}
    finally{if(live())setReadLoading(false);}
  }
  const schedule = nextScheduleItem(character.schedule,now);
  const previewContent=<><Surface className="replica-moment-card">
    <div className="replica-section-heading"><h2>{character.name}的此刻</h2><Badge variant="outline">实时预览</Badge></div>
    <WorkspaceMomentArt/>
    <div className="companion-moment-copy"><span className="companion-moment-location"><MapPinIcon size={13}/>{character.location||'位置尚未记录'}</span><h3>{character.activity||'此刻活动尚未记录'}</h3><p>{character.mood||'心情尚未记录'}</p></div>
    <div className="companion-mini-section"><span>今天的穿着</span><WearingSummary character={character} error={currentReadError(workspace.state,'wardrobe',scope)} loading={workspace.state.loading.wardrobe}/><Button variant="link" onClick={()=>onGo('衣柜')}>去衣柜看看<ChevronRightIcon size={13}/></Button></div>
    <div className="companion-mini-section"><div className="companion-row-between"><span>接下来的安排</span><Button variant="link" onClick={()=>onGo('日程')}>查看全部<ChevronRightIcon size={12}/></Button></div>{schedule?<div className="companion-next-schedule"><time dateTime={schedule._source?.start_at} title={userInstant(schedule._source?.start_at)}>{schedule.time}</time><div><p>{schedule.text}</p><small>{schedule._source?.action_status==='in_progress'?'正在进行':'日程安排'}</small></div></div>:<p className="replica-caption">还没有读到接下来的安排。</p>}</div>
    </Surface><div className="companion-soft-note"><LeafIcon size={17}/><p>生活在继续，故事也在慢慢生长。<br/>这里是 {character.name} 的小小世界。</p></div></>;
  return (
    <div className="replica-chat companion-page">
      <WorkspacePageHeader eyebrow="DAILY / A LITTLE CONVERSATION" title="从一句话，慢慢走近。" description="分享今天的小事，也留一点安静相处的时间。" actions={<><Button ref={historyFocus} variant="outline" onClick={()=>setHistoryOpen(true)}><HistoryIcon size={15}/>会话历史</Button><Button data-testid="chat-preview-toggle" variant="outline" onClick={()=>onPreviewChange?.(!preview)}><SlidersHorizontalIcon size={15}/>{preview?'收起此刻':'展开此刻'}</Button></>}/>
      <div className={`companion-chat-layout ${preview ? "" : "without-preview"}`}>

      <Surface as="section" className="companion-chat-panel">
        <header className="companion-chat-heading">
          <CharacterAvatar
            name={character.name}
            initial={character.initial}
            color={character.color}
            src={character._directory?.avatar}
          />
          <div className="companion-chat-person">
            <strong className="line-clamp-2" title={character.name}>{character.name}</strong>
            <span>
              <i className="online-dot" />{" "}
              {character.status === "正常"
                ? "生活进行中"
                : character.status || "状态未读取"}
              {character.job && <span> · {character.job}</span>}
            </span>
          </div>
          <Select value={character.id} onValueChange={id=>{workspace.select(id);onGo("聊天");}}><SelectTrigger className="companion-select" aria-label="切换聊天角色"><SelectValue/></SelectTrigger><SelectContent>{(workspace.state.characters||[]).map((value:any)=><SelectItem key={value.id} value={value.id}>{value.name}</SelectItem>)}</SelectContent></Select>
          <span className="companion-private">
            <ShieldCheckIcon size={13} />
            只属于你们的对话
          </span>
        </header>
        <MessageScrollerProvider autoScroll defaultScrollPosition="end" scrollEdgeThreshold={80}>
          <MessageScroller className="replica-chat-scroller">
            <MessageScrollerViewport aria-label="聊天消息" className="companion-message-list">
              <MessageScrollerContent className="replica-chat-messages">

                {(snapshot.loading||readLoading)&&<MessageScrollerItem messageId="loading"><Spinner/>正在读取对话…</MessageScrollerItem>}
                {!snapshot.loading&&!readLoading&&!messages.length&&!error&&!busy&&<MessageScrollerItem messageId="intro"><div className="replica-chat-empty"><LeafIcon size={24}/><p>从一句问候开始，让陪伴慢慢发生。</p></div></MessageScrollerItem>}
          {messages.map((message,index) => {
            const mine = message.kind === "user",
              failed =
                mine &&
                ["failed", "cancelled"].includes(message.turnStatus || "");
            const day=messageDay(message.createdAt),previous=messages[index-1],newDay=!previous||day!==messageDay(previous.createdAt);
            return (
              <MessageScrollerItem key={message.id} messageId={message.id}>{newDay&&<Marker className="companion-date-divider"><MarkerContent>{day}</MarkerContent></Marker>}<Message align={mine?"end":"start"} className={"companion-message"+(mine?" is-mine":"")}>
                <CharacterAvatar
                  className={mine?"companion-self-avatar":"replica-message-avatar"}
                  name={mine ? "我" : character.name}
                  initial={mine ? "我" : character.initial}
                  color={mine ? "#e9eae6" : character.color}
                  src={mine ? undefined : character._directory?.avatar}
                />
                <MessageContent className="companion-message-body">
                  <MessageHeader className="companion-message-meta">
                    <span>{mine ? "我" : character.name}</span>
                    <ControlHint text={userInstant(message.createdAt)}><time
                      dateTime={message.createdAt}
                      aria-label={userInstant(message.createdAt)}
                    >
                      {workspace.clock(message.createdAt, "Asia/Shanghai")}
                    </time></ControlHint>
                  </MessageHeader>
                  <Bubble className="replica-message-bubble" variant={mine?"tinted":"outline"} align={mine?"end":"start"}>
                    <BubbleContent className="companion-bubble">
                      {message.text}
                      {!!message.attachmentRefs?.length&&<AttachmentGroup className="companion-message-attachments">{message.attachmentRefs.map(id=><ChatAttachment key={id} src={workspace.services.mediaClient.url(id)} alt={character.name+' 的附件'} onOpen={source=>{attachmentFocus.current=source;setAttachment(id);}}/>)}</AttachmentGroup>}
                    </BubbleContent>
                  </Bubble>
                  {mine&&message.turnStatus==='completed'&&<div className="companion-message-state"><CheckCheckIcon size={12}/>已送达</div>}
                  {failed && (
                    <ChatTurnFeedback
                      message={message}
                      messages={messages}
                      store={store}
                    />
                  )}
                </MessageContent>
              </Message></MessageScrollerItem>
            );
          })}
          {busy && (
            <MessageScrollerItem messageId="waiting"><Message><MessageContent role="status" className="companion-typing"><div className="replica-typing-dots" aria-hidden="true"><i/><i/><i/></div><div>
              {waiting.label}</div>
              {waiting.detail && <small>{waiting.detail}</small>}
              {waiting.showDiagnostics && (
                <Button variant="link" asChild>
                  <a href={diagnosticHref} onClick={openDiagnostic}>查看本次运行 <ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/></a>
                </Button>
              )}
              {store.canCancel && (
                <Button variant="link" onClick={() => store.cancel()}>
                  停止本次生成
                </Button>
              )}
            </MessageContent></Message></MessageScrollerItem>
          )}
          {error && (busy || !messages.some(message=>message.kind==='user'&&['failed','cancelled'].includes(message.turnStatus||''))) && (
            <MessageScrollerItem messageId="error"><InlineFeedback role="alert">
              {error}<Button variant="outline" disabled={snapshot.loading||readLoading} onClick={()=>void reread()}>重新读取对话</Button>
              <Button variant="link" asChild>
                <a href={diagnosticHref} onClick={openDiagnostic}>查看本次运行 <ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/></a>
              </Button>
            </InlineFeedback></MessageScrollerItem>
          )}
              </MessageScrollerContent>
            </MessageScrollerViewport>
            <MessageScrollerButton className="bottom-3" aria-label="回到最新消息"/>
          </MessageScroller>
        </MessageScrollerProvider>
        <div className="companion-composer">
          {promptOffer&&<div className="companion-prefill-note"><span>已保留你的草稿，还有一条来自其他页面的请求。</span><Button variant="link" onClick={()=>{setDraft(current=>current.trim()?current+'\n\n'+promptOffer:promptOffer);setPromptOffer('');}}>加入草稿</Button><Button variant="ghost" size="icon" aria-label="放下这条请求" onClick={()=>setPromptOffer('')}><XIcon size={12}/></Button></div>}
          <Textarea
            aria-label="聊天消息"
            placeholder="说点什么，分享你的今天…"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onCompositionStart={() => (composing.current = true)}
            onCompositionEnd={() => (composing.current = false)}
            onKeyDown={(event) => {
              if (
                event.key === "Enter" &&
                !event.shiftKey &&
                !event.ctrlKey &&
                !event.altKey &&
                !event.metaKey &&
                !event.nativeEvent.isComposing &&
                !composing.current &&
                event.keyCode !== 229
              ) {
                event.preventDefault();
                void send();
              }
            }}
          />
          <div className="companion-composer-bottom">
            <span>Enter 发送 <i/> Shift + Enter 换行</span>
            <Button
              className="touch-target"
              disabled={!draft.trim() || busy || readLoading || !snapshot.conversation}
              onClick={() => void send()}
            >
              {busy ? "回应中" : "发送"}
              <SendIcon size={14} />
            </Button>
          </div>
        </div>
      </Surface>
      {preview&&<aside className="companion-moment-preview"><SidebarProvider keyboardShortcut={false} open={preview} onOpenChange={onPreviewChange} className="contents min-h-0"><Sidebar side="right" data-side="right" collapsible="none" className="replica-chat-preview"><SidebarContent className="replica-chat-preview-content">{previewContent}</SidebarContent></Sidebar></SidebarProvider></aside>}
      </div>
      {(historyOpen||historyRoute)&&<ConversationHistory key={historyRoute?historyRoute.path+'?'+historyRoute.query:'inline'} initialQuery={historyRoute?.query.get('q')||''} initialConversationId={historyRoute?.query.get('conversation')||''} initialMessageId={historyRoute?.query.get('message')||''} returnFocus={()=>historyFocus.current} busy={busy} onClose={closeHistory}/>}
      {attachment&&<WorkspaceDialog open onOpenChange={open=>{if(!open)setAttachment(null);}} title="对话附件" description={character.name+' 的历史对话图片'} returnFocus={()=>attachmentFocus.current}><MediaPreview src={workspace.services.mediaClient.url(attachment)} alt={character.name+' 的附件'}/></WorkspaceDialog>}
      {diagnosticRoute&&<DetailDrawer title="本次对话运行" testId="chat-diagnostic-drawer" onClose={()=>setDiagnosticRoute(null)}><DomainPage route={diagnosticRoute}/></DetailDrawer>}
    </div>
  );
}
import ControlHint from '../shared/ControlHint';
