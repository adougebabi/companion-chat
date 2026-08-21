import {normalizeBootstrap, normalizeMessagePage, normalizeActivityPage, isRecord} from './contracts';
import type {ActivityPage, BootstrapResponse, ChatRequest, MessagePage} from './contracts';

export class ApiError extends Error {
  readonly status: number;
  readonly payload: unknown;

  constructor(message: string, status: number, payload: unknown = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.payload = payload;
  }
}

export interface RequestOptions extends RequestInit {
  signal?: AbortSignal;
}

async function readPayload(response: Response): Promise<unknown> {
  if (response.status === 204) return null;
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

function errorMessage(payload: unknown, status: number): string {
  if (isRecord(payload) && typeof payload.error === 'string' && payload.error.trim()) return payload.error;
  if (typeof payload === 'string' && payload.trim()) return payload;
  return `HTTP ${status}`;
}

export async function requestJson<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await fetch(path, {
    ...options,
    headers: {
      Accept: 'application/json',
      ...(options.body !== undefined ? {'Content-Type': 'application/json'} : {}),
      ...options.headers
    }
  });
  const payload = await readPayload(response);
  if (!response.ok) throw new ApiError(errorMessage(payload, response.status), response.status, payload);
  return payload as T;
}

export async function requestStream(path: string, body: unknown, options: RequestOptions = {}): Promise<Response> {
  const response = await fetch(path, {
    ...options,
    method: options.method ?? 'POST',
    headers: {
      Accept: 'text/event-stream',
      'Content-Type': 'application/json',
      ...options.headers
    },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    const payload = await readPayload(response);
    throw new ApiError(errorMessage(payload, response.status), response.status, payload);
  }
  if (!response.body) throw new ApiError('流式响应不可用', response.status);
  return response;
}

export function encodePath(value: string): string {
  return encodeURIComponent(value);
}

export function queryString(values: Record<string, string | number | null | undefined>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value === null || value === undefined || value === '') continue;
    params.set(key, String(value));
  }
  const query = params.toString();
  return query ? `?${query}` : '';
}

export async function getBootstrap(options: RequestOptions = {}): Promise<BootstrapResponse> {
  return normalizeBootstrap(await requestJson('/api/companion/bootstrap', options));
}

export async function getConversationPage(
  personaId: string,
  {cursor, limit = 20, ...options}: RequestOptions & {cursor?: string | null; limit?: number} = {}
): Promise<MessagePage> {
  const path = `/api/companion/conversations/${encodePath(personaId)}${queryString({cursor, limit})}`;
  return normalizeMessagePage(await requestJson(path, options));
}

export async function sendChat(request: ChatRequest, options: RequestOptions = {}): Promise<Response> {
  return requestStream('/api/companion/chat', request, options);
}

export async function getActivityPage(
  {personaId, cursor, limit = 20, visibility, ...options}: RequestOptions & {
    personaId?: string | null;
    cursor?: string | null;
    limit?: number;
    visibility?: 'visible' | 'hidden';
  } = {}
): Promise<ActivityPage> {
  const path = `/api/companion/activities${queryString({personaId, cursor, limit, visibility})}`;
  return normalizeActivityPage(await requestJson(path, options));
}

export const apiClient = Object.freeze({
  requestJson,
  requestStream,
  getBootstrap,
  getConversationPage,
  getActivityPage,
  sendChat
});

export default apiClient;
