import {InlineFeedback} from "@/react-app/shared/Presentation";
import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import ChatTurnFeedback from "../components/ChatTurnFeedback";
import {Sheet,SheetContent,SheetHeader,SheetTitle,SheetDescription,SheetClose} from "@/components/ui/sheet";
import {XIcon} from "lucide-react";
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  RotateCcwIcon,
  SendIcon,
  ShieldCheckIcon,
  ChevronRightIcon,
  LeafIcon,
} from "lucide-react";
import { useWorkspace } from "../page";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import CharacterAvatar from "../shared/CharacterAvatar";
import { CurrentOutfit } from "../components";
import {
  chatWaitingPresentation,
  chatDiagnosticTurn,
  chatFailureText,
} from "@/v3/chat-feedback.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import type { ConversationController } from "@/state/conversation";
export default function ChatPage({
  preview,
  onGo,
  onPreviewChange,
}: {
  preview: boolean;
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
  const [now, setNow] = useState(Date.now);
  const [narrow,setNarrow]=useState(()=>window.matchMedia("(max-width: 800px)").matches);
  useEffect(()=>{const media=window.matchMedia("(max-width: 800px)");const update=()=>setNarrow(media.matches);media.addEventListener("change",update);return()=>media.removeEventListener("change",update);},[]);
  const latestDraft = useRef(draft);
  latestDraft.current = draft;
  const composing = useRef(false),
    timeline = useRef<HTMLDivElement>(null),
    following = useRef(true);
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
    let active = true,
      polling = false,
      lastIdleRead = 0,
      lastSummaryRead = 0;
    const live = () => active && workspace.currentScope() === scope;
    void workspace
      .openConversation(character.id, live)
      .catch((error: Error) => {
        if (live()) workspace.state.errors.chat = error.message;
      });
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
          workspace.state.errors.chat =
            error instanceof Error ? error.message : "读取失败";
      } finally {
        polling = false;
      }
    }, 2000);
    return () => {
      active = false;
      clearInterval(timer);
      if (store.fluctlightId === character.id) store.invalidateRequest();
    };
  }, [workspace, store, scope, character.id]);
  const messages =
    snapshot.fluctlightId === character.id
      ? snapshot.messages.filter(
          (message) =>
            message.kind !== "system" && !message.id.startsWith("stream-"),
        )
      : [];
  useLayoutEffect(() => {
    if (following.current && timeline.current)
      timeline.current.scrollTop = timeline.current.scrollHeight;
  }, [snapshot.messages, busy]);
  async function send() {
    const submitted = latestDraft.current,
      text = submitted.trim();
    if (!text || busy) return;
    await store.send(text, false, null, () => {
      if (
        workspace.currentScope() === scope &&
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
      : workspace.state.errors.chat || "";
  const schedule =
    character.schedule.find(
      (item: any) => item._source?.action_status === "in_progress",
    ) ||
    character.schedule.find(
      (item: any) =>
        !item._source?.end_at || Date.parse(item._source.end_at) > now,
    );
  const dateLabel =
    "今天 · " +
    new Intl.DateTimeFormat("zh-CN", {
      timeZone: "Asia/Shanghai",
      month: "long",
      day: "numeric",
      weekday: "long",
    })
      .format(new Date(now))
      .replace(/(日)(星期)/, "$1 $2");
  const previewContent=<>

          <div className="preview-heading">
            <span>{character.name}的此刻</span>
            <Badge variant="secondary">实时预览</Badge>
          </div>
          <div className="scene">
            <div className="scene-window">
              <i />
              <i />
            </div>
            <div className="scene-sun" />
            <div className="scene-vase">♧</div>
            <div className="scene-desk" />
            <div className="scene-caption">
              <span className="scene-tag">A LITTLE MOMENT</span>
              <p>{character.location || character.activity || "此刻"}</p>
            </div>
          </div>
          <div className="compact-bio">
            <b>{[character.name, character.job].filter(Boolean).join(" · ")}</b>
            <p>{character.bio}</p>
            <small>
              {[character.mood, character.location].filter(Boolean).join(" · ")}
            </small>
          </div>
          <CurrentOutfit />
          <div className="schedule-preview">
            <div>
              <span>接下来的安排</span>
              <Button variant="ghost" onClick={() => {onPreviewChange?.(false);onGo("日程");}}>
                查看全部
                <ChevronRightIcon size={12} />
              </Button>
            </div>
            {schedule && (
              <div className="next-item">
                <span className="schedule-dot" />
                <div>
                  <small>
                    {schedule.time} —{" "}
                    {schedule._source?.end_at
                      ? workspace.clock(
                          schedule._source.end_at,
                          character.timezone,
                        )
                      : ""}
                  </small>
                  <p>{schedule.text}</p>
                  <Badge variant="secondary">
                    {schedule._source?.action_status === "in_progress"
                      ? "进行中"
                      : "日程安排"}
                  </Badge>
                </div>
              </div>
            )}
          </div>
          <div className="preview-tip">
            <LeafIcon size={16} />
            <p>
              生活在继续，故事也在慢慢生长。
              <br />
              这里是 {character.name} 的小小世界。
            </p>
          </div>

  </>;
  return (
    <div className={`chat-layout ${preview ? "" : "collapsed"}`}>
      <section className="chat-panel">
        <header className="chat-header">
          <CharacterAvatar
            name={character.name}
            initial={character.initial}
            color={character.color}
            src={character._directory?.avatar}
          />
          <div>
            <strong>{character.name}</strong>
            <small>
              <i className="online-dot" />{" "}
              {character.status === "正常"
                ? "运行中"
                : character.status || "状态未读取"}
              {character.job && <span> · {character.job}</span>}
            </small>
          </div>
          <span className="spacer" />
          <span className="chat-private">
            <ShieldCheckIcon size={13} />
            只属于你们的对话
          </span>
        </header>
        <div
          ref={timeline}
          className="timeline"
          aria-label="聊天消息"
          onScroll={() => {
            const node = timeline.current;
            if (node)
              following.current =
                node.scrollHeight - node.scrollTop - node.clientHeight < 80;
          }}
        >
          <div className="date-divider">
            <span>{dateLabel}</span>
          </div>
          <div className="conversation-note">
            从一句问候开始，让陪伴慢慢发生。
          </div>
          {messages.map((message) => {
            const mine = message.kind === "user",
              failed =
                mine &&
                ["failed", "cancelled"].includes(message.turnStatus || "");
            return (
              <div key={message.id} className={`message ${mine ? "mine" : ""}`}>
                <CharacterAvatar
                  className="message-avatar"
                  name={mine ? "我" : character.name}
                  initial={mine ? "我" : character.initial}
                  color={mine ? "#e9eae6" : character.color}
                />
                <div className="message-body">
                  <div className="message-meta">
                    {mine ? "我" : character.name}
                    <ControlHint text={userInstant(message.createdAt)}><time
                      dateTime={message.createdAt}
                      aria-label={userInstant(message.createdAt)}
                    >
                      {workspace.clock(message.createdAt, "Asia/Shanghai")}
                    </time></ControlHint>
                  </div>
                  <div className="bubble">
                    <span>{message.text}</span>
                    {(message.attachmentRefs || []).map((id) => (
                      <img
                        key={id}
                        src={workspace.services.mediaClient.url(id)}
                        alt={character.name + " 的附件"}
                        loading="lazy"
                        className="approved-chat-attachment"
                      />
                    ))}
                  </div>
                  {failed && (
                    <ChatTurnFeedback
                      message={message}
                      messages={messages}
                      store={store}
                    />
                  )}
                </div>
              </div>
            );
          })}
          {busy && (
            <div className="typing">
              {waiting.label}
              <span aria-hidden="true">•••</span>
              {waiting.detail && <small>{waiting.detail}</small>}
              {waiting.showDiagnostics && (
                <Button variant="link" asChild>
                  <a href={diagnosticHref}>查看本次运行 <ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/></a>
                </Button>
              )}
              {waiting.showStop && store.canCancel && (
                <Button variant="link" onClick={() => store.cancel()}>
                  停止本次生成
                </Button>
              )}
            </div>
          )}
          {error && !busy && (
            <InlineFeedback  role="alert">
              {error}
              <Button variant="link" asChild>
                <a href={diagnosticHref}>查看本次运行 <ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/></a>
              </Button>
            </InlineFeedback>
          )}
        </div>
        <div className="composer">
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
          <div className="composer-bottom">
            <small>Enter 发送 · Shift + Enter 换行</small>
            <Button
              className="touch-target"
              disabled={!draft.trim() || busy}
              onClick={() => void send()}
            >
              {busy ? "回应中" : "发送"}
              <SendIcon size={14} />
            </Button>
          </div>
        </div>
      </section>
      {preview&&!narrow&&<aside className="preview">{previewContent}</aside>}
      <Sheet open={preview&&narrow} onOpenChange={onPreviewChange}>
        <SheetContent className="react-workspace chat-preview-sheet data-[side=right]:w-full sm:max-w-[380px] gap-0 p-0" showCloseButton={false} onCloseAutoFocus={event=>{const trigger=document.querySelector<HTMLElement>('[data-testid="chat-preview-toggle"]');if(trigger){event.preventDefault();trigger.focus({preventScroll:true});}}}>
          <SheetHeader className="relative border-b pr-16"><SheetTitle>此刻预览</SheetTitle><SheetDescription>查看穿着与接下来的安排</SheetDescription><SheetClose asChild><Button type="button" variant="ghost" size="icon" className="touch-target absolute right-2 top-2" aria-label="关闭预览"><XIcon/></Button></SheetClose></SheetHeader>
          <div className="min-h-0 flex-1 overflow-y-auto p-4">{previewContent}</div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
import ControlHint from '../shared/ControlHint';
