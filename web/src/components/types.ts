export type ViewName = 'contacts' | 'chat' | 'activity' | 'settings';

export interface MediaAsset {
  id?: string;
  kind?: 'image' | 'video' | string;
  url?: string | null;
  alt?: string | null;
  width?: number | null;
  height?: number | null;
  aspectRatio?: number | null;
}

export interface MediaGeneration {
  kind?: 'image' | 'video' | string;
  status?: string;
  request?: string | null;
}

export interface Message {
  id: string;
  role: 'user' | 'assistant' | string;
  text?: string | null;
  createdAt?: string | null;
  attachments?: MediaAsset[] | null;
  generation?: MediaGeneration | null;
  transient?: 'typing' | string | null;
}

export interface PersonaSummary {
  id: string;
  name: string;
  role?: string | null;
  color?: string | null;
  currentSituation?: string | null;
  mood?: string | null;
  unreadCount?: number | null;
  groupId?: string | null;
  screened?: boolean;
  imageGenerationPolicy?: string | null;
}

export interface ContactGroup {
  id: string;
  name: string;
  isDefault?: boolean;
}

export interface ActivityComment {
  id?: string;
  authorName?: string | null;
  content: string;
  createdAt?: string | null;
}

export interface ActivityItem {
  id: string;
  persona?: PersonaSummary | null;
  personaId?: string | null;
  content?: string | null;
  createdAt?: string | null;
  liked?: boolean;
  mediaMode?: string | null;
  mediaStatus?: string | null;
  media?: MediaAsset[] | null;
  comments?: ActivityComment[] | null;
}

export interface SettingsSnapshot {
  simplifiedMediaMode?: boolean;
  debugInspector?: boolean;
  [key: string]: unknown;
}

export interface ScheduleItem {
  id: string;
  title: string;
  startsAt?: string | null;
  endsAt?: string | null;
  details?: { scene?: string | null; [key: string]: unknown } | null;
}

export interface MemoryItem {
  id: string;
  key: string;
  value: string;
}

export interface FoundationRevision {
  id: string;
  version: number;
  reason?: string | null;
  createdAt?: string | null;
}

export interface EvolutionItem {
  id: string;
  reason: string;
  evidenceSummary?: string | null;
  createdAt?: string | null;
  status?: string | null;
  changes?: Array<{ field: string; before: string; after: string }>;
}

export interface PersonaDetailData extends PersonaSummary {
  foundation?: string | null;
  foundationRevisions?: FoundationRevision[] | null;
  memories?: MemoryItem[] | null;
  evolutions?: EvolutionItem[] | null;
  supportingCharacters?: Array<{ name: string }> | null;
  state?: {
    situation?: string | null;
    mood?: string | null;
    scene?: string | null;
    location?: string | null;
    room?: string | null;
    source?: { label?: string | null; rationale?: string | null } | string | null;
    appearance?: Record<string, string> | null;
  } | null;
  schedule?: ScheduleItem[] | null;
  inferredFields?: string[] | null;
  lifecycle?: Record<string, unknown> | null;
}

export interface MediaJob {
  id?: string;
  kind?: string;
  provider?: string;
  status?: string;
  createdAt?: string | null;
  finalPrompt?: string | null;
  prompt?: string | null;
  error?: string | null;
  progress?: Record<string, unknown> | null;
  trigger?: unknown;
  envelope?: unknown;
  personaConcept?: unknown;
  promptTemplate?: unknown;
  workflow?: unknown;
  workflowSummary?: unknown;
  [key: string]: unknown;
}

