export type JsonObject = Record<string, unknown>;

export type MessageRole = 'assistant' | 'user';

export type AttachmentKind = 'image' | 'video' | 'file' | string;

export interface Attachment {
  id?: string;
  kind: AttachmentKind;
  url: string;
  name?: string;
  mimeType?: string;
  width?: number;
  height?: number;
  [key: string]: unknown;
}

export interface GenerationState {
  id?: string;
  kind?: 'image' | 'video' | string;
  status?: string;
  error?: string;
  [key: string]: unknown;
}

export interface Message {
  id: string;
  role: MessageRole;
  text: string;
  attachments: Attachment[];
  generation?: GenerationState;
  jobs: unknown[];
  proactiveEventId?: string;
  proactivePendingEventId?: string;
  createdAt: string;
  readAt?: string;
  transient?: boolean | 'typing';
  [key: string]: unknown;
}

export interface PersonaSummary {
  id: string;
  name: string;
  role: string;
  initializationMode?: 'llm_defined' | 'blank_slate' | string;
  color?: string;
  groupId: string | null;
  groupName: string | null;
  screened: boolean;
  currentSituation: string;
  mood: string;
  unreadCount: number;
  updatedAt?: string;
  [key: string]: unknown;
}

export interface ContactGroup {
  id: string;
  name: string;
  isDefault: boolean;
  personaCount: number;
  [key: string]: unknown;
}

export interface MediaProviderSummary {
  id: string;
  label?: string;
  capabilities?: string[];
  portType?: string;
  configured?: boolean;
}

export interface H3ConfigCheck {
  configured: boolean;
  valid: boolean;
  displayName?: string;
  error?: string;
}

export interface H3ConfigSummary {
  executable?: H3ConfigCheck;
  modelDir?: H3ConfigCheck;
  outputDir?: H3ConfigCheck;
}

export type PublicSettings = JsonObject & {
  lmStudioUrl?: string;
  model?: string;
  imageProvider?: string;
  videoProvider?: string;
  comfyUrl?: string;
  imageWorkflow?: string;
  videoWorkflow?: string;
  h3TimeoutMs?: number;
  h3Defaults?: JsonObject;
  h3ConfigSummary?: H3ConfigSummary;
  mediaProviders?: MediaProviderSummary[];
  hasH3Configuration?: boolean;
  hasLmStudioApiKey?: boolean;
  defaultTimezone?: string;
};

export interface BootstrapResponse {
  settings: PublicSettings;
  personas: PersonaSummary[];
  groups: ContactGroup[];
  activityUnread: boolean;
  defaultTimezone?: string;
  debugInspector?: boolean;
}

export interface MessagePage {
  items: Message[];
  nextCursor: string | null;
}

export interface ActivityComment {
  id: string;
  authorKind: string;
  authorName: string;
  content: string;
  createdAt: string;
}

export interface ActivityMedia {
  id: string;
  kind?: AttachmentKind;
  url: string;
}

export interface Activity {
  id: string;
  persona: PersonaSummary | null;
  content: string;
  mediaMode: string;
  mediaStatus: string;
  createdAt: string;
  comments: ActivityComment[];
  liked: boolean;
  media: ActivityMedia[];
  [key: string]: unknown;
}

export interface ActivityPage {
  items: Activity[];
  nextCursor: string | null;
}

export interface ChatRequest {
  personaId: string;
  text: string;
  attachments?: Attachment[];
  userMessageId?: string;
}

export interface SseTokenEvent {
  type: 'token';
  token: string;
}

export interface SseDoneEvent {
  type: 'done';
  messages: Message[];
  message: Message | null;
  learned: unknown[];
  jobs: unknown[];
  [key: string]: unknown;
}

export interface SseErrorEvent {
  type: 'error';
  error: string;
}

export type SseEvent = SseTokenEvent | SseDoneEvent | SseErrorEvent;

export type ConversationStreamStatus = 'idle' | 'sending' | 'done' | 'error';

export interface ConversationStreamState {
  status: ConversationStreamStatus;
  pendingId?: string;
  error?: string | null;
}

export interface ConversationState {
  items: Message[];
  nextCursor: string | null;
  hasMore: boolean;
  loadingInitial: boolean;
  loadingOlder: boolean;
  historyError: string | null;
  stream: ConversationStreamState;
}
