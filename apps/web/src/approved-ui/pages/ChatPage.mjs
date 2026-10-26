import {
  defineComponent,
  ref,
  computed,
  watch,
  onMounted,
  onUnmounted,
  nextTick,
} from "vue";
import { useWorkspace } from "../workspace.mjs";
import { renderChatPage } from "../renderers/ChatPage.mjs";
import { CurrentOutfit } from "../shared/components.mjs";
import { joined } from "../presentation.mjs";
import {
  chatWaitingPresentation,
  chatDiagnosticTurn,
  chatFailureText,
} from "../../v3/chat-feedback.mjs";
import { userInstant } from "../../v3/presentation-time.mjs";

export default defineComponent({
  name: "ChatPage",
  props: { preview: Boolean },
  emits: ["go"],
  setup(props, { emit }) {
    const workspace = useWorkspace(),
      store = workspace.conversation(),
      draft = ref(""),
      timeline = ref(),
      character = workspace.current,
      scope = workspace.currentScope();
    let active = true,
      poll,
      polling = false,
      lastIdleRead = 0,
      lastSummaryRead = 0,
      composing = false;
    const now = ref(Date.now());
    const live = () => active && workspace.currentScope() === scope;
    const storageKey = "fluctlight:approved-chat-draft:" + scope;
    try {
      draft.value = sessionStorage.getItem(storageKey) || "";
    } catch {
      /* The mounted draft is still usable. */
    }
    const scroll = () =>
      nextTick(() => {
        const node = timeline.value;
        if (node) node.scrollTop = node.scrollHeight;
      });
    const project = () => {
      if (!live() || store.fluctlightId !== character.value.id) return;
      const node = timeline.value,
        following =
          !node || node.scrollHeight - node.scrollTop - node.clientHeight < 80;
      character.value.messages = store.messages
        .filter((m) => m.kind !== "system" && !m.id.startsWith('stream-'))
        .map((message) => ({
          id: message.id,
          text: message.text,
          attachments: message.attachmentRefs || [],
          user: message.kind === "user",
          time: workspace.clock(message.createdAt, "Asia/Shanghai"),
          instant: message.createdAt,
          fullTime: userInstant(message.createdAt),
          action:
            message.kind === "user" &&
            ["failed", "cancelled", "pending", "running"].includes(
              message.turnStatus,
            )
              ? {
                  status: ["failed", "cancelled"].includes(message.turnStatus)
                    ? "error"
                    : "pending",
                  reason: chatFailureText(message.turnErrorCode || store.error,store.messages.some((m)=>m.kind==='assistant'&&!m.id.startsWith('stream-')&&m.turnId===message.turnId)),
                  label: store.retrying ? "重试中…" : message.turnRetryable ? "重试原回应" : "查看状态",
                  disabled: store.sending || store.retrying,
                  run: () => {
                    if (message.turnRetryable) return store.retryMessage(message.id);
                    return store.refreshActiveTurn(true);
                  },
                }
              : undefined,
        }));
      if (following) void scroll();
    };
    const stop = store.$subscribe(project, { detached: true, flush: "sync" });
    const busy = computed(
      () => store.sending || store.hasPendingTurn || store.hasSettlingTurn,
    );
    const send = async () => {
      const submitted = draft.value, text = submitted.trim();
      if (!text || busy.value) return;
      await store.send(text,false,null,()=>{
        if(live()&&draft.value===submitted){draft.value="";sessionStorage.removeItem(storageKey);}
      });
      project();
    };
    watch(draft, (value) => {
      try {
        sessionStorage.setItem(storageKey, value);
      } catch {
        /* No authoritative data is stored here. */
      }
    });
    onMounted(async () => {
      try {
        await workspace.openConversation(character.value.id, live);
        if (!live()) return;
        project();
        void scroll();
        const tick = async () => {
          if (!live() || document.hidden || polling) return;
          now.value = Date.now();
          polling = true;
          try {
            if (store.hasPendingTurn || store.hasSettlingTurn)
              await store.refreshActiveTurn(true);
            else if (
              !store.sending &&
              !composing &&
              !draft.value.trim() &&
              Date.now() - lastIdleRead > 10000
            ) {
              lastIdleRead = Date.now();
              await store.refreshActiveTurn(true);
              if (!live()) return;
              project();
              if (Date.now() - lastSummaryRead > 30000) {
                lastSummaryRead = Date.now();
                await workspace.detail();
                if (!live()) return;
                await workspace.loadWardrobe();
              }
            }
          } catch (error) {
            if (live()) workspace.state.errors.chat = error.message;
          } finally {
            polling = false;
          }
        };
        poll = setInterval(() => void tick(), 2000);
      } catch (error) {
        if (live()) workspace.state.errors.chat = error.message;
      }
    });
    onUnmounted(() => {
      active = false;
      stop();
      clearInterval(poll);
      if (store.fluctlightId === character.value.id) store.invalidateRequest();
    });
    const currentSchedule = computed(() => {
      const item =
        character.value.schedule.find(
          (i) => i._source?.action_status === "in_progress",
        ) ||
        character.value.schedule.find(
          (i) =>
            !i._source?.end_at || Date.parse(i._source.end_at) > Date.now(),
        );
      return item
        ? {
            ...item,
            endTime: item._source?.end_at
              ? workspace.clock(item._source.end_at, character.value.timezone)
              : "",
            stateLabel:
              item._source?.action_status === "in_progress"
                ? "进行中"
                : "日程安排",
          }
        : null;
    });
    const dateLabel = computed(
      () =>
        "今天 · " +
        new Intl.DateTimeFormat("zh-CN", {
          timeZone: "Asia/Shanghai",
          month: "long",
          day: "numeric",
          weekday: "long",
        })
          .format(new Date(now.value))
          .replace(/(日)(星期)/, "$1 $2"),
    );
    const waiting = computed(() => {
      const pending = store.messages.findLast(
        (m) =>
          m.kind === "user" && ["pending", "running"].includes(m.turnStatus),
      );
      const presentation=chatWaitingPresentation(
        store,
        store.attemptStartedAt ?? (pending?.createdAt ? Date.parse(pending.createdAt) : now.value),
        now.value,
        character.value.name,
      );
      return {...presentation,showStop:presentation.showStop&&store.canCancel};
    });
    const diagnosticHref = computed(() => {
      const id = chatDiagnosticTurn(store);
      return id
        ? "#/diag/turns?correlationId=" + encodeURIComponent("turn:" + id)
        : "#/diag";
    });
    return renderChatPage({
      diagnosticHref,
      openDiagnostics: () => workspace.navigatePath(diagnosticHref.value.slice(1)),
      openReplySettings: () => workspace.navigatePath('/settings/reply-vision?returnTo=' + encodeURIComponent('#/c/' + encodeURIComponent(character.value.id) + '/chat')),
      readStatus: () =>
        store.refreshActiveTurn(true).catch((error) => {
          if (live()) workspace.state.errors.chat = error.message;
        }),
      cancel: () => store.cancel(),
      joined,
      waiting,
      errorText:computed(()=>store.fluctlightId===character.value.id&&store.error?chatFailureText(store.error,store.messages.some(m=>m.kind==='assistant'&&!m.id.startsWith('stream-')&&m.turnId===chatDiagnosticTurn(store))):workspace.state.errors.chat||''),
      props,
      composition: (value) => (composing = value),
      mediaUrl: (id) => workspace.services.mediaClient.url(id),
      dateLabel,
      currentSchedule,
      character,
      i: timeline,
      l: draft,
      u: send,
      f: busy,
      n: emit,
      CurrentOutfit,
    });
  },
});
