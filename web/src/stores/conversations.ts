import {reactive} from 'vue';
import {defineStore} from 'pinia';
import {listConversationPage} from '../api/conversations';
import type {ConversationState, Message, MessagePage, ConversationStreamStatus} from '../types';

const PAGE_SIZE = 20;

function createConversationState(): ConversationState {
  return {
    items: [],
    nextCursor: null,
    hasMore: false,
    loadingInitial: false,
    loadingOlder: false,
    historyError: null,
    stream: {status: 'idle', pendingId: undefined, error: null}
  };
}

function mergeTail(current: Message[], incoming: Message[]): Message[] {
  const incomingById = new Map(incoming.map(item => [item.id, item]));
  const seen = new Set(current.map(item => item.id));
  const replaced = current.map(item => incomingById.get(item.id) ?? item);
  return [...replaced, ...incoming.filter(item => !seen.has(item.id))];
}

function mergeHead(current: Message[], incoming: Message[]): Message[] {
  const incomingById = new Map(incoming.map(item => [item.id, item]));
  const seen = new Set(current.map(item => item.id));
  const replaced = current.map(item => incomingById.get(item.id) ?? item);
  return [...incoming.filter(item => !seen.has(item.id)), ...replaced];
}

/**
 * Initial pages are authoritative for IDs they contain. Keep local-only
 * optimistic/previously loaded messages, but never let an older in-memory
 * placeholder overwrite a fresher server projection with the same ID.
 */
function mergeInitial(serverPage: Message[], current: Message[]): Message[] {
  const serverIds = new Set(serverPage.map(item => item.id));
  const localOnly = current.filter(item => !serverIds.has(item.id));
  const merged = [...serverPage, ...localOnly];
  return merged
    .map((item, index) => ({item, index}))
    .sort((left, right) => {
      const leftTime = Date.parse(left.item.createdAt || '') || 0;
      const rightTime = Date.parse(right.item.createdAt || '') || 0;
      return leftTime - rightTime || left.index - right.index;
    })
    .map(entry => entry.item);
}

function now(): string {
  return new Date().toISOString();
}

function optimisticMessage(message: Partial<Message> & Pick<Message, 'id' | 'role' | 'text'>): Message {
  return {
    ...message,
    attachments: message.attachments ?? [],
    jobs: message.jobs ?? [],
    createdAt: message.createdAt ?? now(),
  } as Message;
}

export const useConversationsStore = defineStore('conversations', () => {
  const conversations = reactive<Record<string, ConversationState>>({});

  function ensure(personaId: string): ConversationState {
    if (!conversations[personaId]) conversations[personaId] = createConversationState();
    return conversations[personaId];
  }

  function get(personaId: string | null | undefined): ConversationState | null {
    return personaId ? ensure(personaId) : null;
  }

  function applyPage(state: ConversationState, page: MessagePage, mode: 'initial' | 'older'): void {
    if (mode === 'older') state.items = mergeHead(state.items, page.items);
    else state.items = mergeInitial(page.items, state.items);
    state.nextCursor = page.nextCursor;
    state.hasMore = page.nextCursor !== null;
  }

  async function loadInitial(personaId: string, options: {signal?: AbortSignal} = {}): Promise<MessagePage> {
    const state = ensure(personaId);
    if (state.loadingInitial) return {items: state.items, nextCursor: state.nextCursor};
    state.loadingInitial = true;
    state.historyError = null;
    try {
      const page = await listConversationPage(personaId, {limit: PAGE_SIZE, signal: options.signal});
      // Merge with an active stream/optimistic tail so an initial response can
      // never discard a message that arrived while the request was in flight.
      applyPage(state, page, 'initial');
      return page;
    } catch (caught) {
      if ((caught as Error)?.name !== 'AbortError') state.historyError = caught instanceof Error ? caught.message : '会话加载失败';
      throw caught;
    } finally {
      state.loadingInitial = false;
    }
  }

  async function loadOlder(personaId: string, options: {signal?: AbortSignal} = {}): Promise<MessagePage | null> {
    const state = ensure(personaId);
    if (state.loadingOlder || !state.nextCursor || !state.hasMore) return null;
    state.loadingOlder = true;
    state.historyError = null;
    try {
      const page = await listConversationPage(personaId, {
        cursor: state.nextCursor,
        limit: PAGE_SIZE,
        signal: options.signal
      });
      applyPage(state, page, 'older');
      return page;
    } catch (caught) {
      if ((caught as Error)?.name !== 'AbortError') state.historyError = caught instanceof Error ? caught.message : '历史加载失败';
      throw caught;
    } finally {
      state.loadingOlder = false;
    }
  }

  function clearHistoryError(personaId: string): void {
    ensure(personaId).historyError = null;
  }

  function addOptimistic(personaId: string, message: Partial<Message> & Pick<Message, 'id' | 'role' | 'text'>): Message {
    const state = ensure(personaId);
    const next = optimisticMessage(message);
    state.items = mergeTail(state.items, [next]);
    return next;
  }

  function startStream(personaId: string, pendingId: string): void {
    const state = ensure(personaId);
    state.stream = {status: 'sending', pendingId, error: null};
  }

  function appendTransientToken(personaId: string, pendingId: string, token: string): void {
    const state = ensure(personaId);
    const pending = state.items.find(item => item.id === pendingId);
    if (pending) pending.text += token;
  }

  function reconcileStream(
    personaId: string,
    pendingId: string,
    messages: Message[],
    compatibilityMessage: Message | null = null
  ): void {
    const state = ensure(personaId);
    state.items = state.items.filter(item => item.id !== pendingId);
    const completed = messages.length ? messages : compatibilityMessage ? [compatibilityMessage] : [];
    state.items = mergeTail(state.items, completed);
    state.stream = {status: 'done', pendingId: undefined, error: null};
  }

  function failStream(personaId: string, pendingId: string, error: string): void {
    const state = ensure(personaId);
    state.items = state.items.filter(item => item.id !== pendingId);
    state.stream = {status: 'error', pendingId: undefined, error};
  }

  function setStreamStatus(personaId: string, status: ConversationStreamStatus, error: string | null = null): void {
    const state = ensure(personaId);
    state.stream = {...state.stream, status, error};
  }

  function mergeTailMessages(personaId: string, messages: Message[]): void {
    const state = ensure(personaId);
    state.items = mergeTail(state.items, messages);
  }

  return {
    conversations,
    ensure,
    get,
    loadInitial,
    loadOlder,
    clearHistoryError,
    addOptimistic,
    startStream,
    appendTransientToken,
    reconcileStream,
    failStream,
    setStreamStatus,
    mergeTailMessages
  };
});

export {PAGE_SIZE as CONVERSATION_PAGE_SIZE};
export default useConversationsStore;
