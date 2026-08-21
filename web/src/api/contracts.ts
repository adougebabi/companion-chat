import type {
  Activity,
  ActivityPage,
  Attachment,
  BootstrapResponse,
  ChatRequest,
  ContactGroup,
  JsonObject,
  Message,
  MessagePage,
  PersonaSummary,
  PublicSettings,
  SseDoneEvent,
  SseErrorEvent,
  SseEvent,
  SseTokenEvent
} from '../types';

export type { 
  Activity,
  ActivityPage,
  Attachment,
  BootstrapResponse,
  ChatRequest,
  ContactGroup,
  JsonObject,
  Message,
  MessagePage,
  PersonaSummary,
  PublicSettings,
  SseDoneEvent,
  SseErrorEvent,
  SseEvent,
  SseTokenEvent
} from '../types';

export function isRecord(value: unknown): value is JsonObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

function nullableString(value: unknown): string | null {
  return typeof value === 'string' && value.length > 0 ? value : null;
}

function numberValue(value: unknown, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function booleanValue(value: unknown, fallback = false): boolean {
  if (typeof value === 'boolean') return value;
  if (value === 1 || value === '1' || value === 'true') return true;
  if (value === 0 || value === '0' || value === 'false') return false;
  return fallback;
}

function normalizeAttachment(value: unknown): Attachment | null {
  if (!isRecord(value)) return null;
  const url = stringValue(value.url);
  if (!url) return null;
  const kind = stringValue(value.kind, 'file');
  return {
    ...value,
    ...(typeof value.id === 'string' ? {id: value.id} : {}),
    kind,
    url,
    ...(typeof value.name === 'string' ? {name: value.name} : {}),
    ...(typeof value.mimeType === 'string' ? {mimeType: value.mimeType} : {})
  };
}

export function normalizeMessage(value: unknown, fallbackId?: string): Message | null {
  if (!isRecord(value)) return null;
  const id = stringValue(value.id, fallbackId ?? '');
  const role = value.role === 'assistant' || value.role === 'user' ? value.role : null;
  if (!id || !role) return null;
  const attachments = Array.isArray(value.attachments)
    ? value.attachments.map(normalizeAttachment).filter((item): item is Attachment => item !== null)
    : [];
  const createdAt = stringValue(value.createdAt, new Date(0).toISOString());
  return {
    ...value,
    id,
    role,
    text: stringValue(value.text),
    attachments,
    jobs: Array.isArray(value.jobs) ? value.jobs : [],
    createdAt,
    ...(value.generation && isRecord(value.generation) ? {generation: value.generation} : {}),
    ...(typeof value.readAt === 'string' ? {readAt: value.readAt} : {}),
    ...(typeof value.proactiveEventId === 'string' ? {proactiveEventId: value.proactiveEventId} : {}),
    ...(typeof value.proactivePendingEventId === 'string'
      ? {proactivePendingEventId: value.proactivePendingEventId}
      : {})
  };
}

export function isMessage(value: unknown): value is Message {
  return normalizeMessage(value) !== null;
}

export function normalizeMessagePage(value: unknown): MessagePage {
  if (!isRecord(value)) return {items: [], nextCursor: null};
  const items = Array.isArray(value.items)
    ? value.items.map(item => normalizeMessage(item)).filter((item): item is Message => item !== null)
    : [];
  const nextCursor = value.nextCursor === null || value.nextCursor === undefined
    ? null
    : stringValue(value.nextCursor) || null;
  return {items, nextCursor};
}

export function isMessagePage(value: unknown): value is MessagePage {
  if (!isRecord(value) || !Array.isArray(value.items)) return false;
  return value.items.every(isMessage)
    && (value.nextCursor === null || typeof value.nextCursor === 'string');
}

function normalizePersona(value: unknown): PersonaSummary | null {
  if (!isRecord(value)) return null;
  const id = stringValue(value.id);
  if (!id) return null;
  return {
    ...value,
    id,
    name: stringValue(value.name, id),
    role: stringValue(value.role),
    ...(typeof value.color === 'string' ? {color: value.color} : {}),
    groupId: nullableString(value.groupId),
    groupName: nullableString(value.groupName),
    screened: booleanValue(value.screened),
    currentSituation: stringValue(value.currentSituation),
    mood: stringValue(value.mood),
    unreadCount: numberValue(value.unreadCount),
    ...(typeof value.updatedAt === 'string' ? {updatedAt: value.updatedAt} : {})
  };
}

function normalizeGroup(value: unknown): ContactGroup | null {
  if (!isRecord(value)) return null;
  const id = stringValue(value.id);
  if (!id) return null;
  return {
    ...value,
    id,
    name: stringValue(value.name, id),
    isDefault: booleanValue(value.isDefault),
    personaCount: numberValue(value.personaCount)
  };
}

export function normalizeBootstrap(value: unknown): BootstrapResponse {
  const source = isRecord(value) ? value : {};
  const personas = Array.isArray(source.personas)
    ? source.personas.map(normalizePersona).filter((item): item is PersonaSummary => item !== null)
    : [];
  const groups = Array.isArray(source.groups)
    ? source.groups.map(normalizeGroup).filter((item): item is ContactGroup => item !== null)
    : [];
  const settings: PublicSettings = isRecord(source.settings) ? source.settings : {};
  return {
    settings,
    personas,
    groups,
    activityUnread: booleanValue(source.activityUnread),
    ...(typeof source.defaultTimezone === 'string' ? {defaultTimezone: source.defaultTimezone} : {}),
    ...(source.debugInspector === true ? {debugInspector: true} : {})
  };
}

export function isBootstrapResponse(value: unknown): value is BootstrapResponse {
  const normalized = normalizeBootstrap(value);
  return isRecord(value)
    && isRecord(value.settings)
    && Array.isArray(value.personas)
    && normalized.personas.length === value.personas.length
    && Array.isArray(value.groups)
    && normalized.groups.length === value.groups.length
    && typeof value.activityUnread === 'boolean';
}

function normalizeActivity(value: unknown): Activity | null {
  if (!isRecord(value)) return null;
  const id = stringValue(value.id);
  if (!id) return null;
  const persona = normalizePersona(value.persona);
  const media = Array.isArray(value.media)
    ? value.media.filter(isRecord).map(item => {
      const mediaId = stringValue(item.id);
      const url = stringValue(item.url);
      return mediaId && url ? {id: mediaId, url, ...(typeof item.kind === 'string' ? {kind: item.kind} : {})} : null;
    }).filter((item): item is Activity['media'][number] => item !== null)
    : [];
  const comments = Array.isArray(value.comments)
    ? value.comments.filter(isRecord).map(comment => {
      const commentId = stringValue(comment.id);
      return commentId ? {
        id: commentId,
        authorKind: stringValue(comment.authorKind),
        authorName: stringValue(comment.authorName),
        content: stringValue(comment.content),
        createdAt: stringValue(comment.createdAt, new Date(0).toISOString())
      } : null;
    }).filter((item): item is Activity['comments'][number] => item !== null)
    : [];
  return {
    ...value,
    id,
    persona,
    content: stringValue(value.content),
    mediaMode: stringValue(value.mediaMode, 'none'),
    mediaStatus: stringValue(value.mediaStatus, 'none'),
    createdAt: stringValue(value.createdAt, new Date(0).toISOString()),
    comments,
    liked: booleanValue(value.liked),
    media
  };
}

export function normalizeActivityPage(value: unknown): ActivityPage {
  if (!isRecord(value)) return {items: [], nextCursor: null};
  const items = Array.isArray(value.items)
    ? value.items.map(normalizeActivity).filter((item): item is Activity => item !== null)
    : [];
  const nextCursor = value.nextCursor === null || value.nextCursor === undefined
    ? null
    : stringValue(value.nextCursor) || null;
  return {items, nextCursor};
}

export function isActivityPage(value: unknown): value is ActivityPage {
  if (!isRecord(value) || !Array.isArray(value.items)) return false;
  return value.items.every(item => normalizeActivity(item) !== null)
    && (value.nextCursor === null || typeof value.nextCursor === 'string');
}

export function isSseTokenEvent(value: unknown): value is SseTokenEvent {
  return isRecord(value) && value.type === 'token' && typeof value.token === 'string';
}

export function isSseDoneEvent(value: unknown): value is SseDoneEvent {
  if (!isRecord(value) || value.type !== 'done' || !Array.isArray(value.messages)) return false;
  return value.messages.every(isMessage)
    && (value.message === null || value.message === undefined || isMessage(value.message));
}

export function isSseErrorEvent(value: unknown): value is SseErrorEvent {
  return isRecord(value) && value.type === 'error' && typeof value.error === 'string';
}

export function isSseEvent(value: unknown): value is SseEvent {
  return isSseTokenEvent(value) || isSseDoneEvent(value) || isSseErrorEvent(value);
}

/** Parse one outer SSE data envelope; capability/tool payloads are opaque. */
export function parseSseEvent(data: string): SseEvent | null {
  if (data === '[DONE]') return null;
  try {
    const value: unknown = JSON.parse(data);
    return isSseEvent(value) ? value : null;
  } catch {
    return null;
  }
}
