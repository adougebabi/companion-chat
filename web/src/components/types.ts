export type ViewName = 'contacts' | 'chat' | 'activity' | 'settings' | 'debug';

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

export interface MediaProviderSummary {
  id: string;
  label?: string;
  capabilities?: string[];
  portType?: string;
  configured?: boolean;
}

export interface SettingsSnapshot {
  lmStudioUrl?: string;
  lmStudioApiKey?: string;
  model?: string;
  imageProvider?: string;
  videoProvider?: string;
  comfyUrl?: string;
  imageWorkflow?: string;
  videoWorkflow?: string;
  h3Executable?: string;
  h3ModelDir?: string;
  h3Profile?: string;
  h3OutputDir?: string;
  h3AllowedRoot?: string;
  h3TimeoutMs?: number;
  h3Defaults?: Record<string, unknown>;
  h3Width?: number;
  h3Height?: number;
  h3Frames?: number;
  h3Steps?: number;
  h3Layers?: number;
  h3Reuse?: number;
  h3SsdStreaming?: boolean;
  h3ConfigSummary?: H3ConfigSummary;
  mediaProviders?: MediaProviderSummary[];
  hasH3Configuration?: boolean;
  hasLmStudioApiKey?: boolean;
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
  blueprint?: Record<string, unknown> | null;
  foundationSummary?: Record<string, unknown> | null;
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

export interface H3PreflightCheck extends H3ConfigCheck {}

export interface H3PreflightResult {
  ok: boolean;
  stage?: 'filesystem' | 'process' | string;
  checks?: Record<string, H3PreflightCheck>;
  process?: { started?: boolean; error?: string; output?: Array<{stream?: string; text?: string}> };
}

export interface InspectorActionResult {
  id?: string;
  status?: string;
  [key: string]: unknown;
}

export interface PromptRun {
  id?: string;
  personaId?: string | null;
  personaName?: string | null;
  jobId?: string | null;
  messageId?: string | null;
  operation?: string | null;
  status?: string | null;
  model?: string | null;
  request?: unknown;
  response?: unknown;
  error?: string | null;
  createdAt?: string | null;
  completedAt?: string | null;
  [key: string]: unknown;
}

export interface DebugContextState {
  situation?: string | null;
  scene?: string | null;
  outfit?: string | null;
  special?: string | null;
  mood?: string | null;
}

export interface DebugContext {
  version?: number;
  personaId?: string | null;
  persona?: { id?: string; name?: string } | null;
  state?: DebugContextState | null;
  layers?: Record<string, unknown> | null;
  recentRequests?: Array<Record<string, unknown>>;
  mediaJobs?: MediaJob[];
  [key: string]: unknown;
}

export interface DurableJob {
  id?: string | null;
  jobType?: string | null;
  status?: string | null;
  priority?: number | null;
  runAfter?: string | null;
  leaseExpiresAt?: string | null;
  attemptCount?: number | null;
  maxAttempts?: number | null;
  personaId?: string | null;
  activityId?: string | null;
  messageId?: string | null;
  traceId?: string | null;
  createdAt?: string | null;
  updatedAt?: string | null;
  completedAt?: string | null;
  error?: string | null;
  payloadSummary?: string | null;
  resultSummary?: string | null;
  [key: string]: unknown;
}

export interface DebugLifecycle {
  personaId?: string | null;
  events?: unknown[];
  affectEvents?: unknown[];
  jobs?: DurableJob[];
  [key: string]: unknown;
}

export interface DebugInspectorSnapshot {
  mediaJobs: MediaJob[];
  personaId?: string | null;
  lifecycle: DebugLifecycle | null;
  debugContext: DebugContext | null;
  promptRuns: PromptRun[];
}
