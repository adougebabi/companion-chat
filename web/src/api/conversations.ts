import {encodePath, getConversationPage, requestJson} from './client';
import {normalizeMessagePage} from './contracts';
import type {Message, MessagePage} from '../types';

export interface ConversationPageOptions {
  cursor?: string | null;
  limit?: number;
  signal?: AbortSignal;
}

export function listConversationPage(personaId: string, options: ConversationPageOptions = {}): Promise<MessagePage> {
  return getConversationPage(personaId, {...options, limit: options.limit ?? 20});
}

export async function appendConversationMessage(
  personaId: string,
  message: Pick<Message, 'role' | 'text'> & Partial<Pick<Message, 'attachments' | 'generation' | 'jobs'>>,
  signal?: AbortSignal
): Promise<{message: Message | null; messages: Message[]}> {
  const payload = await requestJson(`/api/companion/conversations/${encodePath(personaId)}/messages`, {
    method: 'POST',
    signal,
    body: JSON.stringify(message)
  });
  const page = normalizeMessagePage(payload);
  const source = payload && typeof payload === 'object' && !Array.isArray(payload)
    ? payload as Record<string, unknown>
    : {};
  const candidate = source.message ? normalizeMessagePage({items: [source.message]}).items[0] : page.items[0];
  return {message: candidate ?? null, messages: page.items};
}

