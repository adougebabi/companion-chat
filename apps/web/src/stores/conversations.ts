import { defineStore } from "pinia";
import {
  BrowserClient,
  BrowserApiError,
  type BrowserConversation,
  type BrowserParticipant,
  type BrowserMessage,
  type BrowserTurnEvent,
} from "@fluctlight/browser-client";
import { bffOrigin } from "../runtime-config";
import { randomId } from "../random-id";

const client = new BrowserClient(bffOrigin);

type StreamPayload = {
  text?: string;
  message?: BrowserMessage;
  messages?: BrowserMessage[];
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
  senderActorId?: string;
	messageId?: string;
};
type QueuedTurn = RetryTurn & {
	messageId: string;
	createdAt: string;
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
const queuedTurnStorageKey = "fluctlight.queued-turn.v1";

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

function persistedQueuedTurn(): QueuedTurn | null {
	if (typeof localStorage === "undefined") return null;
	const raw = localStorage.getItem(queuedTurnStorageKey);
	if (!raw) return null;
	try {
		const value = JSON.parse(raw) as Partial<QueuedTurn>;
		if (
			typeof value.conversationId === "string" && typeof value.fluctlightId === "string" &&
			typeof value.text === "string" && typeof value.idempotencyKey === "string" &&
			typeof value.turnId === "string" && typeof value.messageId === "string" &&
			typeof value.createdAt === "string" && Array.isArray(value.attachmentRefs) &&
			value.attachmentRefs.every((item) => typeof item === "string")
		) {
			return { ...value, attachmentRefs: value.attachmentRefs as string[] } as QueuedTurn;
		}
	} catch {
		// Ignore malformed local queue state; only a fully identified turn is safe to resume.
	}
	localStorage.removeItem(queuedTurnStorageKey);
	return null;
}

function persistQueuedTurn(turn: QueuedTurn | null): void {
	if (typeof localStorage === "undefined") return;
	if (turn) localStorage.setItem(queuedTurnStorageKey, JSON.stringify(turn));
	else localStorage.removeItem(queuedTurnStorageKey);
}

function createLocalMessage(conversationId: string, text: string, sequence: number, authorActorId = "human", messageId?: string, createdAt?: string) : BrowserMessage {
  return {
	id: messageId ?? `local-${randomId()}`,
    conversationId,
    sequence,
    authorActorId,
    kind: "user",
    text,
    attachmentRefs: [],
	createdAt: createdAt ?? new Date().toISOString(),
  };
}

function lastSequence(messages: BrowserMessage[]): number {
	return messages.reduce((highest, message) => isTransientMessage(message) ? highest : Math.max(highest, message.sequence), 0);
}

function isTransientMessage(message: BrowserMessage): boolean {
	return message.id.startsWith("local-") || message.id.startsWith("stream-");
}

function mergeConversationMessages(authoritative: BrowserMessage[], current: BrowserMessage[]): BrowserMessage[] {
	const merged = [...current];
	for (const message of authoritative) {
		const duplicate = merged.some((candidate) =>
			candidate.id === message.id ||
			(!isTransientMessage(candidate) && candidate.conversationId === message.conversationId && candidate.sequence === message.sequence && candidate.kind === message.kind),
		);
		if (!duplicate) merged.push(message);
	}
	return merged.sort((left, right) => {
		const leftTransient = isTransientMessage(left);
		const rightTransient = isTransientMessage(right);
		if (leftTransient !== rightTransient) return leftTransient ? 1 : -1;
		return left.sequence - right.sequence || (left.createdAt ?? "").localeCompare(right.createdAt ?? "");
	});
}

export const useConversationStore = defineStore("conversations", {
  state: () => ({
    conversation: null as BrowserConversation | null,
    conversationParticipants: [] as BrowserParticipant[],
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
		queuedTurn: persistedQueuedTurn(),
    senderActorId: null as string | null,
  }),
  getters: {
    hasConversation: (state) => Boolean(state.conversation?.id),
    selectedFluctlight: (state) =>
      state.fluctlights.find((fluctlight) => fluctlight.id === state.fluctlightId) ?? null,
    selectedFluctlightName(): string | null {
      const name = this.selectedFluctlight?.identity.name;
      return typeof name === "string" && name.trim() ? name : this.selectedFluctlight?.id ?? null;
    },
	canRetry: (state) => Boolean(state.retryTurn) && state.retryTurn?.conversationId === state.conversation?.id && state.retryTurn?.fluctlightId === state.fluctlightId && !state.sending && !state.retrying,
	queuedMessageId: (state) => state.queuedTurn?.conversationId === state.conversation?.id && state.queuedTurn?.fluctlightId === state.fluctlightId ? state.queuedTurn.messageId : null,
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
			this.queuedTurn = null;
			persistQueuedTurn(null);
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
			this.queuedTurn = null;
			persistQueuedTurn(null);
      }
    },
    async bootstrap() {
      if (this.authenticated === false) return;
      this.loading = true;
      this.error = "";
      try {
        this.fluctlights = await client.listFluctlights();
        this.pruneOrphanedLocalTurns();
        await this.reconcilePersistedRetry();
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
		if (this.sending || this.retrying) {
			this.error = "当前消息仍在处理中；请先等待完成或取消后再切换会话。";
			return;
		}
      if (!this.fluctlights.some((item) => item.id === fluctlightId)) {
        this.error = "所选 Fluctlight 实例不可用。";
        return;
      }
      if (options.loading !== false) this.loading = true;
      this.invalidateRequest();
      this.error = "";
      try {
        const page = await client.directConversation(fluctlightId);
        if (this.retryTurn && this.retryTurn.fluctlightId === fluctlightId && this.retryTurn.conversationId !== page.conversation.id) {
          this.retryTurn = null;
          persistRetry(null);
        }
        if (this.queuedTurn && this.queuedTurn.fluctlightId === fluctlightId && this.queuedTurn.conversationId !== page.conversation.id) {
          this.queuedTurn = null;
          persistQueuedTurn(null);
        }
        this.conversation = page.conversation;
        this.conversationParticipants = page.participants;
		this.messages = page.messages;
        this.senderActorId = null;
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
		this.fluctlightId = fluctlightId;
			this.ensureRetryMessageVisible();
			this.ensureQueuedMessageVisible();
	        persistSelection(fluctlightId);
	        await this.reportReadPosition();
			if (!this.retryTurn && this.queuedTurn?.conversationId === this.conversation?.id && this.queuedTurn.fluctlightId === this.fluctlightId) {
				await this.sendQueuedTurn();
			}
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
      this.conversationParticipants = [];
      this.fluctlightId = null;
      this.messages = [];
      this.senderActorId = null;
      this.nextBeforeSequence = null;
	      persistSelection(null);
    },
	async send(text: string, retry = false, queuedRequest: QueuedTurn | null = null) {
		const normalized = text.trim();
		if (!normalized || this.sending) return;
		const initialConversationId = this.conversation?.id;
		const initialFluctlightId = this.fluctlightId;
		const retryBelongsToCurrent = this.retryTurn?.conversationId === initialConversationId && this.retryTurn?.fluctlightId === initialFluctlightId;
		if (!retry && this.retryTurn && !retryBelongsToCurrent) await this.reconcilePersistedRetry();
		const conversationId = this.conversation?.id;
		const fluctlightId = this.fluctlightId;
		const pendingRetry = this.retryTurn?.conversationId === conversationId && this.retryTurn?.fluctlightId === fluctlightId ? this.retryTurn : null;
		if (!conversationId || !fluctlightId) return;
		if (retry && !pendingRetry) return;
		if (!retry && this.retryTurn && !pendingRetry) {
			this.error = "另一个会话仍有未完成消息；请返回该会话处理后再发送。";
			return;
		}
		if (queuedRequest && (queuedRequest.conversationId !== conversationId || queuedRequest.fluctlightId !== fluctlightId)) return;
		if (pendingRetry && !retry && !queuedRequest) {
			const queuedMessage = createLocalMessage(conversationId, normalized, 0, this.senderActorId ?? "human");
			const queuedTurn: QueuedTurn = {
				conversationId,
				fluctlightId,
				text: normalized,
				idempotencyKey: `turn-${randomId()}`,
				turnId: `turn_${randomId()}`,
				attachmentRefs: this.attachmentRef ? [this.attachmentRef] : [],
				senderActorId: this.senderActorId ?? undefined,
				messageId: queuedMessage.id,
				createdAt: queuedMessage.createdAt,
			};
			this.queuedTurn = queuedTurn;
			persistQueuedTurn(queuedTurn);
			this.messages.push(queuedMessage);
			await this.retry();
			return;
		}
	      this.error = "";
	      this.sending = true;
	      this.abortController = new AbortController();
	      const requestEpoch = this.requestEpoch;
	      const request: RetryTurn = pendingRetry && retry
	        ? pendingRetry
			: queuedRequest
				? { ...queuedRequest }
	        : {
            conversationId,
            fluctlightId,
            text: normalized,
            idempotencyKey: `turn-${randomId()}`,
            turnId: `turn_${randomId()}`,
            attachmentRefs: this.attachmentRef ? [this.attachmentRef] : [],
	            senderActorId: this.senderActorId ?? undefined,
	          };
		let optimisticMessageId = request.messageId ?? null;
	      if (!retry) {
			const localAlreadyVisible = optimisticMessageId && this.messages.some((message) => message.id === optimisticMessageId);
			if (!localAlreadyVisible) {
				const userMessage = createLocalMessage(conversationId, normalized, 0, request.senderActorId ?? "human", optimisticMessageId ?? undefined, queuedRequest?.createdAt);
				optimisticMessageId = userMessage.id;
				request.messageId = userMessage.id;
				this.messages.push(userMessage);
			}
	      }
      let assistantDraft: BrowserMessage | null = null;
      try {
        const response = await client.turn(
          request.conversationId,
          {
            text: request.text,
            fluctlightId: request.fluctlightId,
            senderActorId: request.senderActorId,
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
        let terminalType: "completed" | "error" | null = null;
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
	                sequence: 0,
                authorActorId: request.fluctlightId,
                kind: "assistant",
                text: assistantText,
                attachmentRefs: [],
                createdAt: new Date().toISOString(),
              };
              assistantDraft = streamedMessage;
              this.messages.push(streamedMessage);
            } else {
              // `assistantDraft` is the raw object originally pushed into a
              // reactive Pinia array. Mutating that raw reference bypasses
              // Vue's proxy, which made the UI freeze after the first token
              // until a refresh. Replace the array entry so every delta is
              // observable.
              const draftIndex = this.messages.findIndex((message) => message.id === assistantDraft?.id);
              if (draftIndex >= 0) {
                this.messages.splice(draftIndex, 1, { ...this.messages[draftIndex], text: assistantText });
              }
            }
          }
	          if ((event.type === "message" || event.type === "media") && (payload.message || payload.messages?.length)) {
	            const incoming = payload.messages?.length ? payload.messages : payload.message ? [payload.message] : [];
	            for (const message of incoming) {
				const optimisticIndex = message.kind === "user" && optimisticMessageId
					? this.messages.findIndex((candidate) => candidate.id === optimisticMessageId)
					: -1;
				if (message.kind === "assistant" && assistantDraft) {
					const draftIndex = this.messages.findIndex((candidate) => candidate.id === assistantDraft?.id);
					if (draftIndex >= 0) {
						this.messages.splice(draftIndex, 1, message);
						assistantDraft = null;
						continue;
					}
				}
	              const persistedIndex = this.messages.findIndex(
	                (candidate) =>
	                  candidate.id === message.id ||
	                  (!isTransientMessage(candidate) && candidate.conversationId === message.conversationId && candidate.sequence === message.sequence && candidate.kind === message.kind),
	              );
	              if (persistedIndex >= 0) this.messages.splice(persistedIndex, 1, message);
	              else if (optimisticIndex >= 0) {
					this.messages.splice(optimisticIndex, 1, message);
					request.messageId = message.id;
					optimisticMessageId = message.id;
				}
	              else this.messages.push(message);
            }
          }
          if (event.type === "error") {
            const code = payload.code ?? "turn_failed";
            const detail = payload.detail?.trim();
            terminalType = "error";
            throw new Error(detail ? `${code}: ${detail}` : code);
          }
          if (event.type === "completed") terminalType = "completed";
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
	        if (terminalType !== "completed") throw new Error("turn_stream_incomplete");
	        const page = await client.messages(request.conversationId);
			if (this.requestEpoch !== requestEpoch || this.conversation?.id !== request.conversationId || this.fluctlightId !== request.fluctlightId) return;
	        this.conversation = page.conversation;
		this.messages = mergeConversationMessages(page.messages, this.messages);
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
        await this.reportReadPosition();
        this.attachmentRef = "";
			if (this.retryTurn?.idempotencyKey === request.idempotencyKey || retry) {
				this.retryTurn = null;
				persistRetry(null);
			}
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
			request.messageId = request.messageId ?? optimisticMessageId ?? undefined;
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
    pruneOrphanedLocalTurns() {
      const available = new Set(this.fluctlights.map((item) => item.id));
      if (this.retryTurn && !available.has(this.retryTurn.fluctlightId)) {
        this.retryTurn = null;
        persistRetry(null);
      }
      if (this.queuedTurn && !available.has(this.queuedTurn.fluctlightId)) {
        this.queuedTurn = null;
        persistQueuedTurn(null);
      }
    },
    async reconcilePersistedRetry() {
      const retry = this.retryTurn;
      if (!retry || !this.fluctlights.some((item) => item.id === retry.fluctlightId)) return;
      try {
        const page = await client.directConversation(retry.fluctlightId);
        if (page.conversation.id !== retry.conversationId) {
          this.retryTurn = null;
          persistRetry(null);
          return;
        }
        let historyPage = page;
        const allMessages = [...page.messages];
        let userMessage: BrowserMessage | undefined;
        while (true) {
          userMessage = retry.messageId
            ? historyPage.messages.find((message) => message.id === retry.messageId && message.kind === "user")
            : [...historyPage.messages].reverse().find((message) => message.kind === "user" && message.text === retry.text);
          if (userMessage || !historyPage.nextBeforeSequence) break;
          historyPage = await client.messages(retry.conversationId, historyPage.nextBeforeSequence, 200);
          allMessages.push(...historyPage.messages);
        }
        if (!userMessage) return;
        const completed = allMessages.some((message) => message.kind === "assistant" && message.sequence > userMessage.sequence);
        if (completed) {
          this.retryTurn = null;
          persistRetry(null);
        }
      } catch (error) {
        if (error instanceof BrowserApiError && error.status === 404) {
          this.retryTurn = null;
          persistRetry(null);
          return;
        }
        // Preserve the retry on transient read failures; blocking is safer than
        // dropping a turn whose authoritative state could not be checked.
      }
    },
    cancel() {
      this.abortController?.abort();
    },
	    async retry() {
	      const pending = this.retryTurn;
	      if (!pending || pending.conversationId !== this.conversation?.id || pending.fluctlightId !== this.fluctlightId || this.sending || this.retrying) return;
	      this.retrying = true;
	      try {
	        await this.send(pending.text, true);
	        if (!this.retryTurn && this.queuedTurn?.conversationId === this.conversation?.id && this.queuedTurn.fluctlightId === this.fluctlightId) {
			await this.sendQueuedTurn();
	        }
      } finally {
        this.retrying = false;
      }
    },
	    dismissRetry() {
		const queued = this.queuedTurn;
	      this.retryTurn = null;
	      this.retrying = false;
	      this.error = "";
	      persistRetry(null);
		if (queued && queued.conversationId === this.conversation?.id && queued.fluctlightId === this.fluctlightId) void this.sendQueuedTurn();
	    },
	async sendQueuedTurn() {
		const queued = this.queuedTurn;
		if (!queued || queued.conversationId !== this.conversation?.id || queued.fluctlightId !== this.fluctlightId || this.sending) return;
		this.queuedTurn = null;
		persistQueuedTurn(null);
		await this.send(queued.text, false, queued);
	},
		ensureRetryMessageVisible() {
		  const pending = this.retryTurn;
		  const conversationId = this.conversation?.id;
		  if (!pending || !conversationId || pending.conversationId !== conversationId || pending.fluctlightId !== this.fluctlightId) return;
		if (pending.messageId && this.messages.some((message) => message.id === pending.messageId)) return;
		if (!pending.messageId) {
			const legacyMatch = [...this.messages].reverse().find((message) => message.kind === "user" && message.conversationId === conversationId && message.text === pending.text);
			if (legacyMatch) {
				pending.messageId = legacyMatch.id;
				persistRetry(pending);
				return;
			}
		}
		const local = createLocalMessage(conversationId, pending.text, 0, pending.senderActorId ?? "human", pending.messageId);
		pending.messageId = local.id;
		persistRetry(pending);
		this.messages.push(local);
		},
	ensureQueuedMessageVisible() {
		const queued = this.queuedTurn;
		const conversationId = this.conversation?.id;
		if (!queued || !conversationId || queued.conversationId !== conversationId || queued.fluctlightId !== this.fluctlightId) return;
		if (!this.messages.some((message) => message.id === queued.messageId)) {
			this.messages.push(createLocalMessage(conversationId, queued.text, 0, queued.senderActorId ?? "human", queued.messageId, queued.createdAt));
		}
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
