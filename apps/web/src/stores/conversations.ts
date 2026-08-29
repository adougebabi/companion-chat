import { defineStore } from "pinia";
import {
  BrowserClient,
  type BrowserConversation,
  type BrowserMessage,
  type BrowserTurnEvent,
} from "@fluctlight/browser-client";
import { bffOrigin } from "../runtime-config";
import { randomId } from "../random-id";

const client = new BrowserClient(bffOrigin);

type StreamPayload = {
  text?: string;
  message?: BrowserMessage;
  message_id?: string;
  code?: string;
  detail?: string;
};
type RetryTurn = {
  conversationId: string;
  fluctlightId: string;
  text: string;
  idempotencyKey: string;
  turnId: string;
  attachmentRefs: string[];
};
export type FluctlightListItem = {
  id: string;
  identity: Record<string, unknown>;
  status: string;
  unread_count?: number;
  last_conversation_at?: string | null;
};

const selectedFluctlightStorageKey = "fluctlight.selected-instance-id";
const retryTurnStorageKey = "fluctlight.retry-turn.v2";

function persistedSelection(): string | null {
  if (typeof localStorage === "undefined") return null;
  return localStorage.getItem(selectedFluctlightStorageKey);
}

function persistSelection(fluctlightId: string | null): void {
  if (typeof localStorage === "undefined") return;
  if (fluctlightId) localStorage.setItem(selectedFluctlightStorageKey, fluctlightId);
  else localStorage.removeItem(selectedFluctlightStorageKey);
}

function persistedRetry(): RetryTurn | null {
  if (typeof localStorage === "undefined") return null;
  const raw = localStorage.getItem(retryTurnStorageKey);
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as Partial<RetryTurn>;
    if (
      typeof value.conversationId === "string" &&
      typeof value.fluctlightId === "string" &&
      typeof value.text === "string" &&
      typeof value.idempotencyKey === "string" &&
      typeof value.turnId === "string" &&
      Array.isArray(value.attachmentRefs) &&
      value.attachmentRefs.every((item) => typeof item === "string")
    ) {
      return { ...value, attachmentRefs: value.attachmentRefs as string[] } as RetryTurn;
    }
  } catch {
    // Ignore malformed local retry state and let the server remain authoritative.
  }
  localStorage.removeItem(retryTurnStorageKey);
  return null;
}

function persistRetry(retry: RetryTurn | null): void {
  if (typeof localStorage === "undefined") return;
  if (retry) localStorage.setItem(retryTurnStorageKey, JSON.stringify(retry));
  else localStorage.removeItem(retryTurnStorageKey);
}

function createLocalMessage(conversationId: string, text: string, sequence: number, authorActorId = "human") : BrowserMessage {
  return {
    id: `local-${randomId()}`,
    conversationId,
    sequence,
    authorActorId,
    kind: "user",
    text,
    attachmentRefs: [],
    createdAt: new Date().toISOString(),
  };
}

function lastSequence(messages: BrowserMessage[]): number {
  return messages.reduce((highest, message) => Math.max(highest, message.sequence), 0);
}

export const useConversationStore = defineStore("conversations", {
  state: () => ({
    conversation: null as BrowserConversation | null,
    fluctlightId: null as string | null,
    fluctlights: [] as FluctlightListItem[],
    messages: [] as BrowserMessage[],
    nextBeforeSequence: null as number | null,
    authenticated: null as boolean | null,
    setupAvailable: false,
    authLoading: false,
    authError: "" as string,
    loading: false,
    sending: false,
    error: "" as string,
    attachmentRef: "",
    abortController: null as AbortController | null,
    retryTurn: persistedRetry(),
    requestEpoch: 0,
    retrying: false,
  }),
  getters: {
    hasConversation: (state) => Boolean(state.conversation?.id),
    selectedFluctlight: (state) =>
      state.fluctlights.find((fluctlight) => fluctlight.id === state.fluctlightId) ?? null,
    selectedFluctlightName(): string | null {
      const name = this.selectedFluctlight?.identity.name;
      return typeof name === "string" && name.trim() ? name : this.selectedFluctlight?.id ?? null;
    },
    canRetry: (state) => Boolean(state.retryTurn) && !state.sending && !state.retrying,
  },
  actions: {
    async initialize() {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.session();
        this.authenticated = session.authenticated;
        if (this.authenticated) await this.bootstrap();
        else this.setupAvailable = (await client.setupStatus()).setupAvailable;
      } catch {
        if (this.authenticated !== true) {
          this.authenticated = false;
          this.authError = "Fluctlight 服务暂时不可用。";
        }
      } finally {
        this.authLoading = false;
      }
    },
    async login(password: string) {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.login(password);
        this.authenticated = session.authenticated;
        if (this.authenticated) await this.bootstrap();
      } catch {
        if (this.authenticated !== true) {
          this.authenticated = false;
          this.authError = "密码未被接受。";
        } else {
          this.error = "无法加载 Fluctlight 对话。";
        }
      } finally {
        this.authLoading = false;
      }
    },
    async setup(setupToken: string, password: string) {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.setup(setupToken, password);
        this.authenticated = session.authenticated;
        this.setupAvailable = false;
        if (this.authenticated) await this.bootstrap();
      } catch {
        this.authenticated = false;
        this.authError = "设置令牌或密码未被接受。";
      } finally {
        this.authLoading = false;
      }
    },
    async changePassword(password: string) {
      this.invalidateRequest();
      this.authLoading = true;
      this.authError = "";
      try {
        await client.changePassword(password);
        this.authenticated = false;
        this.conversation = null;
        this.messages = [];
        this.fluctlightId = null;
        this.retryTurn = null;
        persistRetry(null);
        return true;
      } catch {
        this.authError = "无法修改所有者密码。请确认当前登录会话仍有效后重试。";
        return false;
      } finally {
        this.authLoading = false;
      }
    },
    async logout() {
      this.invalidateRequest();
      try {
        await client.logout();
      } finally {
        this.authenticated = false;
        this.conversation = null;
        this.fluctlightId = null;
        this.fluctlights = [];
        this.messages = [];
        this.nextBeforeSequence = null;
        this.retryTurn = null;
        persistRetry(null);
      }
    },
    async bootstrap() {
      if (this.authenticated === false) return;
      this.loading = true;
      this.error = "";
      try {
        this.fluctlights = await client.listFluctlights();
        const restoredId = persistedSelection();
        const selectedId = this.fluctlights.some((item) => item.id === restoredId)
          ? restoredId
          : this.fluctlights[0]?.id ?? null;
        if (selectedId) await this.selectFluctlight(selectedId, { loading: false });
        else this.clearActiveConversation();
      } catch {
        this.error = "无法加载 Fluctlight 实例目录。";
      } finally {
        this.loading = false;
      }
    },
    async selectFluctlight(fluctlightId: string, options: { loading?: boolean } = {}) {
      if (!this.fluctlights.some((item) => item.id === fluctlightId)) {
        this.error = "所选 Fluctlight 实例不可用。";
        return;
      }
      if (options.loading !== false) this.loading = true;
      this.invalidateRequest();
      this.error = "";
      try {
        const page = await client.directConversation(fluctlightId);
        this.conversation = page.conversation;
        this.messages = page.messages;
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
        this.fluctlightId = fluctlightId;
        if (this.retryTurn?.fluctlightId !== fluctlightId) {
          this.retryTurn = null;
          persistRetry(null);
        }
        persistSelection(fluctlightId);
        await this.reportReadPosition();
      } catch {
        this.clearActiveConversation();
        this.error = "无法打开该 Fluctlight 的对话。";
      } finally {
        if (options.loading !== false) this.loading = false;
      }
    },
    clearActiveConversation() {
      this.invalidateRequest();
      this.conversation = null;
      this.fluctlightId = null;
      this.messages = [];
      this.nextBeforeSequence = null;
      this.retryTurn = null;
      persistRetry(null);
      persistSelection(null);
    },
    async send(text: string, retry = false) {
      const normalized = text.trim();
      const conversationId = this.conversation?.id;
      const fluctlightId = this.fluctlightId;
      const pendingRetry = this.retryTurn;
      if (pendingRetry && !retry) {
        this.error = "上一条消息发送失败，请先重试。";
        return;
      }
      if (!normalized || !conversationId || !fluctlightId || this.sending) return;
      if (retry && (!pendingRetry || pendingRetry.conversationId !== conversationId || pendingRetry.fluctlightId !== fluctlightId)) return;
      this.error = "";
      this.sending = true;
      this.abortController = new AbortController();
      const requestEpoch = this.requestEpoch;
      const request = pendingRetry && retry
        ? pendingRetry
        : {
            conversationId,
            fluctlightId,
            text: normalized,
            idempotencyKey: `turn-${randomId()}`,
            turnId: `turn_${randomId()}`,
            attachmentRefs: this.attachmentRef ? [this.attachmentRef] : [],
          };
      if (!retry) {
        const userMessage = createLocalMessage(conversationId, normalized, this.messages.length + 1);
        this.messages.push(userMessage);
      }
      let assistantDraft: BrowserMessage | null = null;
      try {
        const response = await client.turn(
          request.conversationId,
          {
            text: request.text,
            fluctlightId: request.fluctlightId,
            attachmentRefs: request.attachmentRefs,
            idempotencyKey: request.idempotencyKey,
            turnId: request.turnId,
          },
          this.abortController.signal,
        );
        if (!response.body) throw new Error("stream_missing");
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let assistantText = "";
        let expectedSequence = 0;
        const applyEvent = (event: BrowserTurnEvent) => {
          if (this.requestEpoch !== requestEpoch) return;
          if (event.turnId !== request.turnId) throw new Error("turn_id_mismatch");
          if (event.sequence !== expectedSequence) throw new Error("turn_sequence_invalid");
          expectedSequence += 1;
          const payload = event.payload as StreamPayload;
          if (event.type === "token") {
            assistantText += payload.text ?? "";
            if (!assistantDraft) {
              const streamedMessage: BrowserMessage = {
                id: `stream-${randomId()}`,
                conversationId: request.conversationId,
                sequence: this.messages.length + 1,
                authorActorId: request.fluctlightId,
                kind: "assistant",
                text: assistantText,
                attachmentRefs: [],
                createdAt: new Date().toISOString(),
              };
              assistantDraft = streamedMessage;
              this.messages.push(streamedMessage);
            } else {
              assistantDraft.text = assistantText;
            }
          }
          if (event.type === "message" && payload.message) {
            const optimisticIndex = this.messages.findIndex(
              (message) =>
                message.id.startsWith("local-") &&
                message.kind === "user" &&
                message.conversationId === payload.message!.conversationId &&
                message.text === payload.message!.text,
            );
            const persistedIndex = this.messages.findIndex(
              (message) =>
                message.id === payload.message!.id ||
                (message.conversationId === payload.message!.conversationId &&
                  message.sequence === payload.message!.sequence),
            );
            if (persistedIndex >= 0) this.messages.splice(persistedIndex, 1, payload.message);
            else if (optimisticIndex >= 0) this.messages.splice(optimisticIndex, 1, payload.message);
            else this.messages.push(payload.message);
          }
          if (event.type === "error") {
            const code = payload.code ?? "turn_failed";
            const detail = payload.detail?.trim();
            throw new Error(detail ? `${code}: ${detail}` : code);
          }
        };
        while (true) {
          const next = await reader.read();
          if (this.requestEpoch !== requestEpoch) return;
          if (next.done) break;
          buffer += decoder.decode(next.value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";
          for (const line of lines) {
            if (!line.trim()) continue;
            applyEvent(JSON.parse(line) as BrowserTurnEvent);
          }
        }
        if (this.requestEpoch !== requestEpoch) return;
        buffer += decoder.decode();
        if (buffer.trim()) {
          applyEvent(JSON.parse(buffer) as BrowserTurnEvent);
        }
        const page = await client.messages(request.conversationId);
        this.conversation = page.conversation;
        this.messages = page.messages;
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
        await this.reportReadPosition();
        this.attachmentRef = "";
        this.retryTurn = null;
        persistRetry(null);
      } catch (error) {
        if (this.requestEpoch !== requestEpoch) return;
        if (assistantDraft) this.messages = this.messages.filter((message) => message.id !== assistantDraft?.id);
        const cancelled =
          this.abortController?.signal.aborted ||
          (error instanceof DOMException && error.name === "AbortError");
        if (cancelled) {
          this.error = "回复已取消，可以重试。";
        } else {
          const message = error instanceof Error ? error.message : "turn_failed";
          this.error = `回复未完成：${message}`;
        }
        this.retryTurn = request;
        persistRetry(request);
      } finally {
        if (this.requestEpoch === requestEpoch) {
          this.abortController = null;
          this.sending = false;
        }
      }
    },
    invalidateRequest() {
      this.requestEpoch += 1;
      this.abortController?.abort();
      this.abortController = null;
      this.sending = false;
      this.retrying = false;
    },
    cancel() {
      this.abortController?.abort();
    },
    async retry() {
      const pending = this.retryTurn;
      if (!pending || this.sending || this.retrying) return;
      this.retrying = true;
      try {
        await this.send(pending.text, true);
      } finally {
        this.retrying = false;
      }
    },
    dismissRetry() {
      this.retryTurn = null;
      this.retrying = false;
      this.error = "";
      persistRetry(null);
    },
    async loadOlder() {
      if (!this.conversation || !this.nextBeforeSequence || this.loading) return;
      this.loading = true;
      try {
        const page = await client.messages(this.conversation.id, this.nextBeforeSequence);
        const existing = new Set(this.messages.map((message) => message.id));
        this.messages = [...page.messages.filter((message) => !existing.has(message.id)), ...this.messages];
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
      } catch {
        this.error = "无法加载更早的对话记录。";
      } finally {
        this.loading = false;
      }
    },
    async reportReadPosition() {
      if (!this.conversation) return;
      const sequence = lastSequence(this.messages);
      if (!sequence) return;
      try {
        await client.markRead(this.conversation.id, {
          readSequence: sequence,
          deliveredSequence: sequence,
        });
      } catch {
        this.error = "无法保存已读位置。";
      }
    },
  },
});
