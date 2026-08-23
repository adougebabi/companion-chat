import {ref} from 'vue';
import {sendChat} from '../api/client';
import {isSseDoneEvent, parseSseEvent} from '../api/contracts';
import {useConversationsStore} from '../stores/conversations';
import type {Attachment, Message, SseDoneEvent} from '../types';

export interface ChatStreamInput {
  personaId: string;
  text: string;
  attachments?: Attachment[];
  userMessageId?: string;
  signal?: AbortSignal;
}

export class ChatStreamError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ChatStreamError';
  }
}

function idFor(prefix: string): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return `${prefix}-${crypto.randomUUID()}`;
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function scheduleFrame(callback: () => void): {cancel: () => void} {
  if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
    const handle = window.requestAnimationFrame(callback);
    return {cancel: () => window.cancelAnimationFrame(handle)};
  }
  const handle = setTimeout(callback, 16);
  return {cancel: () => clearTimeout(handle)};
}

function frameData(frame: string): string | null {
  const lines = frame.split(/\r?\n/);
  const data = lines.filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart());
  return data.length ? data.join('\n') : null;
}

async function* readSse(response: Response): AsyncGenerator<ReturnType<typeof parseSseEvent>> {
  if (!response.body) throw new ChatStreamError('流式响应不可用');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      const result = await reader.read();
      buffer += decoder.decode(result.value, {stream: !result.done});
      const frames = buffer.split(/\r?\n\r?\n/);
      buffer = frames.pop() ?? '';
      for (const frame of frames) {
        const data = frameData(frame);
        if (!data) continue;
        if (data === '[DONE]') continue;
        const event = parseSseEvent(data);
        if (!event) throw new ChatStreamError('流式响应格式无效');
        yield event;
      }
      if (result.done) break;
    }
    const data = frameData(buffer);
    if (data && data !== '[DONE]') {
      const event = parseSseEvent(data);
      if (!event) throw new ChatStreamError('流式响应格式无效');
      yield event;
    }
  } finally {
    reader.releaseLock();
  }
}

export function useChatStream() {
  const conversations = useConversationsStore();
  const isSending = ref(false);
  const error = ref<string | null>(null);
  let activePersonaId: string | null = null;
  let activeAbortController: AbortController | null = null;
  const mediaRefreshes = new Map<string, {messageIds: Set<string>; promise: Promise<void>}>();

  async function refreshMediaConversation(personaId: string, messageId?: string | null): Promise<void> {
    const existing = mediaRefreshes.get(personaId);
    if (existing) {
      if (messageId) existing.messageIds.add(messageId);
      return existing.promise;
    }
    const state = {messageIds: new Set<string>(messageId ? [messageId] : []), promise: Promise.resolve()};
    const refresh = (async () => {
      // Providers can legitimately take tens of seconds. Keep this bounded so
      // a failed worker cannot leave a refresh loop running forever.
      const delays = [0, 800, 1_600, 3_200, 6_400, 10_000, 10_000, 10_000, 10_000, 10_000, 10_000, 10_000];
      for (const delay of delays) {
        if (delay) await new Promise(resolve => setTimeout(resolve, delay));
        let page;
        try { page = await conversations.loadInitial(personaId); } catch { continue; }
        const targets = state.messageIds.size
          ? [...state.messageIds]
            .map(id => page.items.find(item => item.id === id))
            .filter((item): item is Message => Boolean(item))
          : page.items.filter(item => item.generation && item.attachments.length === 0);
        if (!targets.length) {
          if (state.messageIds.size === 0) return;
          continue;
        }
        const settled = targets.every(item => item.attachments.length > 0 || item.generation?.status === 'failed');
        if (settled && targets.length === state.messageIds.size) return;
      }
    })().finally(() => mediaRefreshes.delete(personaId));
    state.promise = refresh;
    mediaRefreshes.set(personaId, state);
    return refresh;
  }

  function signalFor(inputSignal?: AbortSignal): AbortSignal {
    const controller = new AbortController();
    activeAbortController = controller;
    if (!inputSignal) return controller.signal;
    if (typeof AbortSignal?.any === 'function') return AbortSignal.any([inputSignal, controller.signal]);
    if (inputSignal.aborted) controller.abort();
    else inputSignal.addEventListener('abort', () => controller.abort(), {once: true});
    return controller.signal;
  }

  async function send(input: ChatStreamInput): Promise<SseDoneEvent> {
    const text = input.text.trim();
    if (!text) throw new ChatStreamError('消息不能为空');
    if (isSending.value) throw new ChatStreamError('消息正在发送');
    isSending.value = true;
    error.value = null;
    activePersonaId = input.personaId;
    const requestSignal = signalFor(input.signal);
    const userId = input.userMessageId ?? idFor('local-user');
    const pendingId = idFor('pending-assistant');
    conversations.addOptimistic(input.personaId, {
      id: userId,
      role: 'user',
      text,
      attachments: input.attachments ?? []
    });
    conversations.addOptimistic(input.personaId, {
      id: pendingId,
      role: 'assistant',
      text: '',
      transient: 'typing'
    });
    conversations.startStream(input.personaId, pendingId);

    let scheduled: {cancel: () => void} | null = null;
    let tokenBuffer = '';
    const flush = () => {
      scheduled = null;
      if (!tokenBuffer) return;
      const token = tokenBuffer;
      tokenBuffer = '';
      conversations.appendTransientToken(input.personaId, pendingId, token);
    };
    const cancelScheduled = () => {
      const current = scheduled;
      scheduled = null;
      current?.cancel();
    };
    const queueToken = (token: string) => {
      tokenBuffer += token;
      if (!scheduled) scheduled = scheduleFrame(flush);
    };

    try {
      const response = await sendChat({personaId: input.personaId, text, attachments: input.attachments, userMessageId: userId}, {signal: requestSignal});
      let done: SseDoneEvent | null = null;
      for await (const event of readSse(response)) {
        if (!event) continue;
        if (event.type === 'token') queueToken(event.token);
        else if (event.type === 'error') throw new ChatStreamError(event.error || '发送失败');
        else if (isSseDoneEvent(event)) {
          done = event;
          break;
        }
      }
      cancelScheduled();
      flush();
      if (!done) throw new ChatStreamError('流式响应未完成');
      conversations.reconcileStream(input.personaId, pendingId, done.messages, done.message);
      const mediaEvent = done && typeof done.mediaEvent === 'object' && done.mediaEvent !== null
        ? done.mediaEvent as {messageId?: string | null}
        : null;
      if (mediaEvent || done.messages.some(message => message.generation || message.jobs.length > 0)) {
        void refreshMediaConversation(input.personaId, mediaEvent?.messageId).catch(() => {});
      }
      return done;
    } catch (caught) {
      cancelScheduled();
      tokenBuffer = '';
      const message = caught instanceof Error && caught.name === 'AbortError'
        ? '已取消'
        : caught instanceof Error ? caught.message : '发送失败';
      conversations.failStream(input.personaId, pendingId, message);
      error.value = message;
      throw caught instanceof ChatStreamError ? caught : new ChatStreamError(message);
    } finally {
      isSending.value = false;
      activeAbortController = null;
      activePersonaId = null;
    }
  }

  function cancel(): void {
    if (activePersonaId) {
      activeAbortController?.abort();
      conversations.setStreamStatus(activePersonaId, 'error', '已取消');
    }
  }

  function clearError(): void {
    error.value = null;
  }

  return {isSending, error, send, cancel, clearError};
}

export default useChatStream;
