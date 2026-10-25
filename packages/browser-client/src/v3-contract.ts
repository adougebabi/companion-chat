import type { BrowserStudioJob, BrowserStudioJobBody, BrowserStudioJobCommandBody, BrowserStudioJobPage, BrowserStudioMaterialPage, BrowserStudioMaterialReceipt } from "./index.js";
import type { BrowserPersonaSwitchRequest, BrowserPersonaSwitchResult } from "./index.js";
// 摇光浏览器契约 v0.1（uploads/fluctlight-backend-contract.md）。
// 响应字段 snake_case。JSON 写操作把幂等键放在请求体 idempotencyKey（Q1 待裁决：
// CORS 目前不放行 Idempotency-Key 头，Allow-Methods 也可能没有 PATCH）。
// 契约仍写了 PATCH（日程条目、日记），这里按契约当前写法发送 PATCH。
// multipart 上传无法把键放进 JSON 体，按契约 B-12 放在请求头 Idempotency-Key。
// C-14 时间线回档不定义任何方法。

export type OwnerRequestStatus = "pending" | "processing" | "accepted" | "rejected" | "deferred" | "awaiting_goal" | "cancelled" | "failed" | "completed";

export type OwnerRequest<Result = Record<string, unknown>> = {
  id: string;
  kind: string;
  fluctlight_id: string;
  status: OwnerRequestStatus;
  payload: Record<string, unknown>;
  decided_by: "agent" | "owner" | "core_persona_assessment" | null;
  result?: Result;
  tasks_state?: "available" | "failed";
  tasks_error_code?: string;
  tasks?: import("./index.js").BrowserRequestTask[];
  decision_reason: string | null;
  result_refs: string[];
  created_at: string;
  updated_at: string;
  revision: number;
};

export type OwnerRequestEnvelope = { request: OwnerRequest; audit_id?: string; schedule_revision?: number };

export type CursorPage<T> = { items: T[]; next_cursor: string | null };

export type ManagedMemory = {
  id: string; owner_fluctlight_id: string; type: 'episodic'|'semantic'|'relationship'|'autobiographical'; content: string;
  actor_refs: string[]; conversation_id: string|null; event_refs: string[]; evidence_refs: string[];
  personality_perspectives: Record<string,unknown>[]; confidence: number; importance: number; emotional_significance: number;
  visibility: 'private'|'owner'|'participants'; status: 'active'|'superseded'|'deprecated'|'forgotten'; revision: number;
  provenance_status: string; occurred_at: string|null; created_at: string; last_confirmed_at: string|null;
  superseded_by_memory_id: string|null; supersedes_memory_id: string|null; revision_count: number; source_count: number;
};
export type MemoryRevision = { revision: number; base_revision: number|null; operation: string; content: string; status: string; actor_id: string; evidence_refs: string[]; source_window: string|null; proposal_id: string|null; semantic_reason: string; related_memory_ids: string[]; reason_code: string|null; schema_version: string; created_at: string };
export type MemorySource = { memory_revision: number; source_ref: string; source_kind: string; source_id: string|null; source_revision: number|null; occurred_at: string|null; recorded_at: string; status: string; live: boolean; superseded_by_ref: string|null; conversation_id: string|null };
export type MemoryBrowseOptions = { aboutOwner?: boolean; q?: string; type?: ManagedMemory['type']|''; status?: ManagedMemory['status']|''; cursor?: string; limit?: number };

export type FactLabel = { factual: false; label: "生成图·非事实" | "插画" };

export type SceneImage = { asset_id: string; url: string | null; captured_at: string; factual: false; label: "生成图·非事实" };

export type EvidenceState = { label: string; state: "推测" | "已确认"; evidence: string[]; updated_at: string };

export type ScheduleItemSuggestionBody = {
  activity: string;
  scene?: string;
  location?: string;
  placeId?: string;
  startAt: string;
  endAt: string;
  evidenceRefs: string[];
  expectedRevision: number;
  expectedLifeContextRevision: string;
  reason: string;
  idempotencyKey: string;
};

export type ScheduleItemDeleteBody = {
  expectedRevision: number;
  expectedLifeContextRevision: string;
  reason: string;
  idempotencyKey: string;
};

export type WearRequestBody = { itemId: string; slot: string; expectedRevision?: number; reason: string; idempotencyKey: string; confirm?: true };

export type ItemUseRequestBody = { active: boolean; expectedRevision?: number; reason: string; evidenceRefs: string[]; idempotencyKey: string; confirm?: true };

export type ItemPhotoRequestBody = import("./index.js").BrowserMediaPhotoBody & {expectedRevision?:never};

export type ItemPhotoResponse = import("./index.js").BrowserMediaRequestEnvelope;

export type CharacterMediaItem = {
  asset_id: string;
  kind: "image" | "video";
  url: string | null;
  title: string | null;
  created_at: string;
  outfit_snapshot_id: string | null;
  item_id: string | null;
  source_message_id: string | null;
  factual: false;
  label: "生成图·非事实";
};

export type SidebarSummary = {
  location?: { place_id: string; name: string } | null;
  next_schedule?: { schedule_item_id: string; title: string; start_at: string } | null;
  worn_item_ids?: string[];
  goal_ids?: string[];
  memory_ids?: string[];
  doing?: { text: string; action_id: string | null } | null;
};

export type AvatarAsset = { asset_id: string; url: string; factual: false; label: "生成图·非事实" | "插画" } | null;

export type CreateMomentBody = { text: string; mediaIds: string[]; worldId?: string; refs?: string[]; idempotencyKey: string };

export type CreateMomentResponse = { id: string; author_actor_id: string; author_type: "user"; created_at: string; audit_id: string };

export type UploadMediaResponse = import("./index.js").BrowserMediaUploadReceipt;

export type HarmIntercepts = { count: number; window: string; latest_reason: string | null; items: Array<{ id: string; at: string; text: string; rule: string }> };

export type DiagnosticTask = import("./index.js").BrowserTask;

export type VisualIdentityWriteBody = import("./index.js").BrowserMediaAvatarBody;

export type NotificationItem = import("./index.js").BrowserNotification;
export type NotificationPage = import("./index.js").BrowserNotificationPage;
export type PendingPage = import("./index.js").BrowserPendingPage;

export type NearbyActors = import("./index.js").BrowserNearbyActors;

export type DiaryEntry = import("./index.js").BrowserDiaryEntry;

export type DiaryWriteBody = { body: string; occurredAt: string; eventId?: string; expectedRevision?: number; reason: string; idempotencyKey: string };

export type ReflectionSummary = { id:string;fluctlight_id:string;title:string;summary:string|null;body:string|null;state:string;status:string;growth:null;at:string;created_at:string;from_sequence:number;to_sequence:number;base_state_revision:number;source_refs:string[];evidence_refs:string[];schema_version:string;model_version:string|null;prompt_version:string|null;policy_version:string;correlation_id:string;expected_revisions:Record<string,number>;result:Record<string,unknown> };
export type ReflectionDetail = ReflectionSummary & { source_window:Record<string,unknown>|null;source_window_status:string;proposal:Record<string,unknown>;references:Array<{ref:string;kind:string;entity_id:string;revision:number}>;dispositions:Array<{domain:string;candidate_id:string;candidate_index:number;disposition:string;reason_code:string;target_ref:string|null;changed_ref:string|null;base_revision:number;resulting_revision:number;evidence_refs:string[]}> };
export type ReflectionCatalog = CursorPage<ReflectionSummary> & {window:Record<string,unknown>|null;latest_task:Record<string,unknown>|null};
export type ReflectionFeedbackBody = {text:string;stance?:'agree'|'disagree';idempotencyKey:string};

export type WorldCatalog = import("./index.js").BrowserWorldCatalog;
export type WorldCreateBody = import("./index.js").BrowserWorldCreateBody;
export type WorldSelectBody = import("./index.js").BrowserWorldSelectBody;
export type WorldDeleteBody = import("./index.js").BrowserWorldDeleteBody;
export type WorldDeleteResult = import("./index.js").BrowserWorldDeleteResult;
export type WorldCommand = import("./index.js").BrowserWorldCommand;
export type RuntimeWorld = import("./index.js").BrowserRuntimeWorld;
export type RuntimePlace = import("./index.js").BrowserRuntimePlace;
export type NPCRoutineSlot = import("./index.js").BrowserNPCRoutineSlot;
export type RuntimeNPC = import("./index.js").BrowserRuntimeNPC;
export type WorldView = import("./index.js").BrowserWorldView;
export type PlaceDetail = RuntimePlace;
export type WorldWriteBody = import("./index.js").BrowserWorldWriteBody;
export type PlaceWriteBody = import("./index.js").BrowserPlaceWriteBody;
export type NPCWriteBody = import("./index.js").BrowserNPCWriteBody;
export type NPCEncounterBody = { confirmed: true; summary: string; startAt: string; endAt: string; placeId: string; evidenceRefs: string[]; expectedLifeContextRevision: string; reason: string; idempotencyKey: string };
export type WorldWriteResult = import("./index.js").BrowserWorldWriteResult;

export type WorldTemplate = import("./index.js").BrowserWorldEventTemplate;

export type WorldTemplateWriteBody = { enabled: boolean; expectedRevision: number; reason: string; idempotencyKey: string };

export type StudioResource = {
  id: string;
  kind: "character_template" | "persona_fragment" | "place" | "item" | "outfit" | "event_template" | "visual_style" | "world" | "npc";
  revision: number;
  name: string;
  status?: "draft" | "published" | "archived";
  archived_at?: string;
  published_version: string | null;
  draft_version: string | null;
  versions: Array<{ version: string; created_at: string; body?: Record<string, unknown> }>;
  referenced_by: Array<{ fluctlight_id: string; selected_version?: string; status?: string; instance_revision?: number | null; running_version?: string; overrides?: Record<string, unknown> }>;
  audit_id?: string;
  body?: Record<string, unknown>;
};

export type StudioPublication = {id:string;version:string;kind:StudioResource["kind"];name:string;body:Record<string,unknown>;immutable:true};
export type StudioResourceQuery = {kind?:string;q?:string;published?:boolean;archived?:boolean;cursor?:string;limit?:number};

export type StudioPublicationRecord = {id:string;version:string;kind:StudioResource['kind'];name:string;created_at:string};
export type StudioCatalogCommand = {expectedRevision:number;reason:string;idempotencyKey:string};
export type StudioCopyBody = StudioCatalogCommand & {source:'draft'|'published';sourceVersion:string;name:string};
export type StudioResourceWriteBody = { kind: StudioResource["kind"]; name: string; body: Record<string, unknown>; expectedVersion?: string; reason: string; idempotencyKey: string };

export type StudioPreview = { preview_id: string; text: string; label: "创作预览·不进入生活"; life_write: false; created_at: string; resource_id?: string | null };

export type StudioPreviewBody = { resourceId?: string; text: string; idempotencyKey: string };

export type ApplyPreviewBody = { fluctlightId: string; resourceVersion: string; runtimeWorldId?:string };

export type ApplyPreviewResponse = { operation?:"create"|"update"|"unchanged"; create_required?:boolean; target_kind?:"visual_brief"|"foundation"|"world_item"|"wardrobe_outfit"; preparation_required?:boolean; world_id?:string; world_binding_revision?:number; profile_id?:string; wardrobe_revision?:number; members?:Array<{resource_id:string;resource_version:string;name:string;slot:string;description:string;more_candidates:boolean;candidates:Array<{item_id:string;description:string;ownership:string;availability:string;exact_source:boolean|null;source_verified:boolean}>}>; available?:boolean; semantics?:string; diff: Array<{ field: string; from: unknown; to: unknown }>; conflicts: unknown[]; instance_revision: number; runtime_revision?: number; runtime_id?: string | null };

export type ApplyResourceBody = { runtimeWorldId?:string; fluctlightId: string; resourceVersion: string; expectedInstanceRevision: number; expectedRuntimeRevision?: number; worldId?:string; expectedWorldRevision?:number; profileId?:string; expectedWardrobeRevision?:number; memberItems?:Array<{resourceId:string;resourceVersion:string;itemId:string}>; homePlaceId?: string | null; routine?: NPCRoutineSlot[]; choices: Array<{ field: string; decision: "keep" | "override" }>; reason: string; idempotencyKey: string };

export type ApplyResourceResponse = { target_kind?:"visual_brief"|"foundation"|"world_item"|"wardrobe_outfit"; brief?:Record<string,unknown>; life_write?:boolean; semantics?:string;
  status: "recorded_not_applied" | "applied" | "proposed";
  proposal_id?:string;
  fluctlight_id?:string;
  applied: boolean;
  id?: string;
  runtime_revision?: number;
  wardrobe_revision?:number;
  runtime_effect?: "recorded_only" | "applied";
  resource_id?: string;
  resource_version?: string;
  instance_revision: number | null;
  instance_revision_unchanged?: boolean;
  choices?: Array<{ field: string; decision: string }>;
  conflicts?: unknown[] | null;
  audit_id?: string;
  available?: boolean;
};

/** 创作应用返回 recorded_not_applied / applied:false 时，选择已记录，实例版本没有变。 */
export function studioApplyUnfinished(response: { status?: string; applied?: boolean; runtime_effect?: string; available?: boolean } | null | undefined): boolean {
  if (!response || response.available === false) return true;
  if (response.applied === false || response.status === "recorded_not_applied") return true;
  return response.runtime_effect === "recorded_only" || response.status === "not_implemented";
}

export type AutonomyPolicy = import("./index.js").BrowserAutonomyPolicy;
export type AutonomyPolicyWriteBody = import("./index.js").BrowserAutonomyPolicyWriteBody;

export type PendingGroup = import("./index.js").BrowserPendingGroup;

export type BackupRecord = import("./index.js").BrowserBusinessBackupJob;
export type BackupRequestReceipt = {id:string;status:string;kind:string;revision:number;stage?:string;accepted?:boolean;audit_id:string};

export type DesireItem = { id: string; text: string; status: string; why: string | null; source_message_id: string | null };

export type MomentAuthorFields = { author_type: "user" | "fluctlight" | "npc"; author_name: string; author_avatar_url: string | null; source_action_id: string | null };

export type MessageIdentity = "exchange" | "action_request" | "owner_edit" | "system_ops";

export type MessageViewFields = {
  identity?: MessageIdentity;
  kind?: "text" | "image" | "action" | "divider" | "system";
  recall?: boolean;
  memory_ref?: string | null;
  goal_ref?: string | null;
  action_id?: string | null;
  link?: string | null;
};

const MOMENT_AUTHOR_KEYS = ["authorFluctlightId", "asFluctlight", "author_fluctlight_id", "as_fluctlight", "proxy", "onBehalfOf"];

type RuntimeClient = {
  json(path: string, options?: { method?: string; body?: unknown }): Promise<unknown>;
  csrfHeaders(): Record<string, string>;
  fetcher: typeof fetch;
  url(path: string): URL;
};

function clientOf(self: object): RuntimeClient {
  return self as RuntimeClient;
}

function q(params: Record<string, string | number | boolean | undefined | null>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") search.set(key, String(value));
  }
  const text = search.toString();
  return text ? `?${text}` : "";
}

function rejectMomentAuthor(body: object): void {
  for (const key of MOMENT_AUTHOR_KEYS) {
    if (key in body) {
      const error = new Error("不能以角色身份发布动态");
      Object.assign(error, { status: 422, code: "moment_author_forbidden", userMessage: "不能以角色身份发布动态", details: {} });
      throw error;
    }
  }
}

/** 409 的当前版本。其它状态不返回数字，避免把缺失读成 0。 */
export function currentRevision(error: { status?: number; details?: Record<string, unknown> | null }): number | undefined {
  if (error?.status !== 409) return undefined;
  const value = error.details?.current_revision;
  return typeof value === "number" ? value : undefined;
}

export const v3ContractMethods = {
  async storyOverview(this: object, fluctlightId: string): Promise<import('./index.js').BrowserStoryOverview> { return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/shared-experiences/overview`) as Promise<import('./index.js').BrowserStoryOverview>; },
  async managedMemories(this: object, fluctlightId: string, options: MemoryBrowseOptions = {}): Promise<CursorPage<ManagedMemory>> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/memories${q(options)}`) as Promise<CursorPage<ManagedMemory>>;
  },
  async managedMemory(this: object, memoryId: string): Promise<ManagedMemory> {
    return clientOf(this).json(`/api/memories/${encodeURIComponent(memoryId)}`) as Promise<ManagedMemory>;
  },
  async memoryRevisions(this: object, memoryId: string, options: {cursor?:string;limit?:number} = {}): Promise<CursorPage<MemoryRevision>> {
    return clientOf(this).json(`/api/memories/${encodeURIComponent(memoryId)}/revisions${q(options)}`) as Promise<CursorPage<MemoryRevision>>;
  },
  async memorySources(this: object, memoryId: string, options: {cursor?:string;limit?:number;revision?:number} = {}): Promise<CursorPage<MemorySource>> {
    return clientOf(this).json(`/api/memories/${encodeURIComponent(memoryId)}/sources${q(options)}`) as Promise<CursorPage<MemorySource>>;
  },
  async rollbackMemory(this: object, memoryId: string, body: {targetRevision:number;expectedRevision:number;evidenceRefs:string[]}): Promise<Record<string,unknown>> {
    return clientOf(this).json(`/api/memories/${encodeURIComponent(memoryId)}/rollback`,{method:'POST',body}) as Promise<Record<string,unknown>>;
  },
  async suggestScheduleItem(this: object, fluctlightId: string, scheduleId: string, body: ScheduleItemSuggestionBody): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/schedules/${encodeURIComponent(scheduleId)}/items`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async updateScheduleItem(this: object, fluctlightId: string, scheduleId: string, itemId: string, body: ScheduleItemSuggestionBody): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/schedules/${encodeURIComponent(scheduleId)}/items/${encodeURIComponent(itemId)}`, { method: "PUT", body }) as Promise<OwnerRequestEnvelope>;
  },
  async deleteScheduleItem(this: object, fluctlightId: string, scheduleId: string, itemId: string, body: ScheduleItemDeleteBody): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/schedules/${encodeURIComponent(scheduleId)}/items/${encodeURIComponent(itemId)}`, { method: "DELETE", body }) as Promise<OwnerRequestEnvelope>;
  },
  async requestWardrobeWear(this: object, fluctlightId: string, body: WearRequestBody): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/wardrobe/wear`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async requestItemUse(this: object, fluctlightId: string, itemId: string, body: ItemUseRequestBody): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/wardrobe/items/${encodeURIComponent(itemId)}/use`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async requestItemPhoto(this: object, fluctlightId: string, itemId: string, body: ItemPhotoRequestBody): Promise<ItemPhotoResponse> {
    if (body.expectedRevision !== undefined) {
      throw Object.assign(new Error("拍照不能修改事实"), { status: 422, code: "photo_cannot_change_facts", userMessage: "拍照不能修改事实", details: {} });
    }
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/wardrobe/items/${encodeURIComponent(itemId)}/photos`, { method: "POST", body }) as Promise<ItemPhotoResponse>;
  },
  async listCharacterMedia(this: object, fluctlightId: string, options: { cursor?: string; limit?: number } = {}): Promise<CursorPage<CharacterMediaItem>> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/media${q(options)}`) as Promise<CursorPage<CharacterMediaItem>>;
  },
  async sidebar(this: object, fluctlightId: string): Promise<SidebarSummary> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/sidebar`) as Promise<SidebarSummary>;
  },
  async createMoment(this: object, body: CreateMomentBody): Promise<CreateMomentResponse> {
    rejectMomentAuthor(body);
    return clientOf(this).json("/api/moments", { method: "POST", body }) as Promise<CreateMomentResponse>;
  },
  async uploadOwnerMedia(this: object, file: Blob, mime: string, idempotencyKey: string, filename?: string): Promise<UploadMediaResponse> {
    const runtime = clientOf(this);
    const form = new FormData();
    form.set("file", file, filename ?? (file instanceof File ? file.name : "upload"));
    form.set("mime", mime);
    form.set("idempotencyKey", idempotencyKey);
    const response = await runtime.fetcher(runtime.url("/api/media"), {
      method: "POST",
      credentials: "include",
      headers: { "Idempotency-Key": idempotencyKey, ...runtime.csrfHeaders() },
      body: form,
    });
    if (!response.ok) {
      const payload = await response.json().catch(() => null) as { code?: string; message?: string; details?: Record<string, unknown> } | null;
      throw Object.assign(new Error(payload?.message || `upload failed: ${response.status}`), { status: response.status, code: payload?.code || "browser_request_failed", userMessage: payload?.message || "上传失败", details: payload?.details || {} });
    }
    return response.json() as Promise<UploadMediaResponse>;
  },
  // 分组没有修订列。删除时 expectedRevision 固定传 0，不要改用别的计数。
  async deleteActorGroup(this: object, groupId: string, body: { expectedRevision: 0; idempotencyKey: string }): Promise<void> {
    await clientOf(this).json(`/api/actor-groups/${encodeURIComponent(groupId)}`, { method: "DELETE", body });
  },
  async harmIntercepts(this: object, options: { fluctlightId?: string; window?: string } = {}): Promise<HarmIntercepts> {
    return clientOf(this).json(`/api/diagnostics/harm-intercepts${q({ fluctlightId: options.fluctlightId, window: options.window ?? "7d" })}`) as Promise<HarmIntercepts>;
  },
  async diagnosticTask(this: object, taskId: string): Promise<DiagnosticTask> {
    return clientOf(this).json(`/api/diagnostics/tasks/${encodeURIComponent(taskId)}`) as Promise<DiagnosticTask>;
  },
  async saveVisualIdentity(this: object, fluctlightId: string, body: VisualIdentityWriteBody): Promise<import("./index.js").BrowserMediaAvatar> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/display-avatar`, { method: "PUT", body }) as Promise<import("./index.js").BrowserMediaAvatar>;
  },
  async listNotifications(this: object, options: { cursor?: string; limit?: number; unread?: boolean; fluctlightId?: string } = {}): Promise<NotificationPage> {
    return clientOf(this).json(`/api/notifications${q(options)}`) as Promise<NotificationPage>;
  },
  async markNotificationRead(this: object, notificationId: string, body: { idempotencyKey: string }): Promise<{ id: string; read_at: string }> {
    return clientOf(this).json(`/api/notifications/${encodeURIComponent(notificationId)}/read`, { method: "POST", body }) as Promise<{ id: string; read_at: string }>;
  },
  async markAllNotificationsRead(this: object, body: { before: string; idempotencyKey: string }): Promise<{ updated: number; before: string }> {
    return clientOf(this).json("/api/notifications/read-all", { method: "POST", body }) as Promise<{ updated: number; before: string }>;
  },
  async unreadNotificationCount(this: object): Promise<{ count: number; alert_count: number }> {
    return clientOf(this).json("/api/notifications/unread-count") as Promise<{ count: number; alert_count: number }>;
  },
  async nearby(this: object, fluctlightId: string): Promise<NearbyActors> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/nearby`) as Promise<NearbyActors>;
  },
  async selfClaims(this:object,fid:string,options:{q?:string;status?:string;category?:string;cursor?:string;limit?:number}={}):Promise<import("./index.js").BrowserSelfPage>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fid)}/self-claims${q(options)}`) as Promise<import("./index.js").BrowserSelfPage>;},
  async selfClaim(this:object,fid:string,id:string):Promise<import("./index.js").BrowserSelfClaim>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fid)}/self-claims/${encodeURIComponent(id)}`) as Promise<import("./index.js").BrowserSelfClaim>;},
  async selfClaimHistory(this:object,fid:string,id:string,kind:'revisions'|'reviews',cursor=''):Promise<import("./index.js").BrowserSelfHistory>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fid)}/self-claims/${encodeURIComponent(id)}/${kind}${q({cursor})}`) as Promise<import("./index.js").BrowserSelfHistory>;},
  async manageSelfClaim(this:object,fid:string,id:string,...args: [command:'review'|'forget',body:import("./index.js").BrowserSelfCommandBody] | [command:'restore',body:import("./index.js").BrowserSelfRestoreBody]):Promise<import("./index.js").BrowserSelfCommandResult>{const [command,body]=args;return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fid)}/self-claims/${encodeURIComponent(id)}/${command}`,{method:'POST',body}) as Promise<import("./index.js").BrowserSelfCommandResult>;},
  async listDiary(this: object, fluctlightId: string, cursor = "", options: {favorite?:boolean;limit?:number} = {}): Promise<import("./index.js").BrowserDiaryPage> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary${q({cursor,...options})}`) as Promise<import("./index.js").BrowserDiaryPage>;
  },
  async diary(this:object,fluctlightId:string,id:string):Promise<DiaryEntry>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary/${encodeURIComponent(id)}`) as Promise<DiaryEntry>;},
  async generateDiary(this:object,fluctlightId:string,body:import("./index.js").BrowserDiaryGenerateBody):Promise<import("./index.js").BrowserDiaryGeneration>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary/generate`,{method:'POST',body}) as Promise<import("./index.js").BrowserDiaryGeneration>;},
  async annotateDiary(this:object,fluctlightId:string,id:string,body:import("./index.js").BrowserDiaryAnnotationBody):Promise<import("./index.js").BrowserDiaryAnnotationResult>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary/${encodeURIComponent(id)}/annotation`,{method:'PUT',body}) as Promise<import("./index.js").BrowserDiaryAnnotationResult>;},
  async diaryReplies(this:object,fluctlightId:string,id:string,cursor=''):Promise<import("./index.js").BrowserDiaryReplies>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary/${encodeURIComponent(id)}/replies${q({cursor})}`) as Promise<import("./index.js").BrowserDiaryReplies>;},
  async replyToDiary(this:object,fluctlightId:string,id:string,body:import("./index.js").BrowserDiaryReplyBody):Promise<OwnerRequestEnvelope>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary/${encodeURIComponent(id)}/replies`,{method:'POST',body}) as Promise<OwnerRequestEnvelope>;},
  async sharedExperiences(this:object,fluctlightId:string,cursor=''):Promise<import("./index.js").BrowserSharedExperiencePage>{return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/shared-experiences${q({cursor})}`) as Promise<import("./index.js").BrowserSharedExperiencePage>;},
  async createDiaryEntry(this: object, fluctlightId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary`, { method: "POST", body }) as Promise<{ id: string; revision: number; source: string; audit_id: string }>;
  },
  async updateDiaryEntry(this: object, fluctlightId: string, entryId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary/${encodeURIComponent(entryId)}`, { method: "PUT", body }) as Promise<{ id: string; revision: number; source: string; audit_id: string }>;
  },
  async reflection(this: object, fluctlightId: string, reflectionId: string): Promise<ReflectionDetail> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/reflections/${encodeURIComponent(reflectionId)}`) as Promise<ReflectionDetail>;
  },
  async submitReflectionFeedback(this: object, fluctlightId: string, reflectionId: string, body: ReflectionFeedbackBody): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/reflections/${encodeURIComponent(reflectionId)}/feedback`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async worldCatalog(this: object, fluctlightId: string): Promise<WorldCatalog> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/catalog`) as Promise<WorldCatalog>;
  },
  async catalogWorld(this: object, fluctlightId: string, worldId: string): Promise<WorldView> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/catalog/${encodeURIComponent(worldId)}`) as Promise<WorldView>;
  },
  async createCatalogWorld(this: object, fluctlightId: string, body: WorldCreateBody): Promise<WorldWriteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/catalog`, { method: "POST", body }) as Promise<WorldWriteResult>;
  },
  async updateCatalogWorld(this: object, fluctlightId: string, worldId: string, body: WorldWriteBody): Promise<WorldWriteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/catalog/${encodeURIComponent(worldId)}`, { method: "PUT", body }) as Promise<WorldWriteResult>;
  },
  async deleteCatalogWorld(this: object, fluctlightId: string, worldId: string, body: WorldCommand): Promise<WorldDeleteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/catalog/${encodeURIComponent(worldId)}`, { method: "DELETE", body }) as Promise<WorldDeleteResult>;
  },
  async selectWorld(this: object, fluctlightId: string, body: WorldSelectBody): Promise<WorldWriteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/active`, { method: "PUT", body }) as Promise<WorldWriteResult>;
  },
  async deleteWorldPlace(this: object, fluctlightId: string, placeId: string, body: WorldDeleteBody): Promise<WorldDeleteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/places/${encodeURIComponent(placeId)}`, { method: "DELETE", body }) as Promise<WorldDeleteResult>;
  },
  async deleteRuntimeNPC(this: object, fluctlightId: string, npcId: string, body: WorldCommand): Promise<WorldDeleteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/npcs/${encodeURIComponent(npcId)}`, { method: "DELETE", body }) as Promise<WorldDeleteResult>;
  },
  async world(this: object, fluctlightId: string): Promise<WorldView> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world`) as Promise<WorldView>;
  },
  async updateRuntimeWorld(this: object, fluctlightId: string, body: WorldWriteBody): Promise<WorldWriteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world`, { method: 'PUT', body }) as Promise<WorldWriteResult>;
  },
  async createWorldPlace(this: object, fluctlightId: string, body: PlaceWriteBody): Promise<WorldWriteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/places`, { method: 'POST', body }) as Promise<WorldWriteResult>;
  },
  async updateWorldPlace(this: object, fluctlightId: string, placeId: string, body: PlaceWriteBody): Promise<WorldWriteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/places/${encodeURIComponent(placeId)}`, { method: 'PUT', body }) as Promise<WorldWriteResult>;
  },
  async listRuntimeNPCs(this: object, fluctlightId: string): Promise<CursorPage<RuntimeNPC>> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/npcs`) as Promise<CursorPage<RuntimeNPC>>;
  },
  async runtimeNPC(this: object, fluctlightId: string, npcId: string): Promise<RuntimeNPC> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/npcs/${encodeURIComponent(npcId)}`) as Promise<RuntimeNPC>;
  },
  async updateRuntimeNPC(this: object, fluctlightId: string, npcId: string, body: NPCWriteBody): Promise<WorldWriteResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/npcs/${encodeURIComponent(npcId)}`, { method: 'PUT', body }) as Promise<WorldWriteResult>;
  },
  async recordNPCEncounter(this: object, fluctlightId: string, npcId: string, body: NPCEncounterBody): Promise<{ id: string; event_id: string; npc_id: string; relationship_id: string; memory_id: string; status: string; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/npcs/${encodeURIComponent(npcId)}/encounters`, { method: 'POST', body }) as Promise<{ id: string; event_id: string; npc_id: string; relationship_id: string; memory_id: string; status: string; audit_id: string }>;
  },
  async publishStudioResource(this: object, resourceId: string, body: WorldCommand): Promise<{ id: string; revision: number; published_version: string; status: 'published' }> {
    return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(resourceId)}/publish`, { method: 'POST', body }) as Promise<{ id: string; revision: number; published_version: string; status: 'published' }>;
  },
  async placeDetail(this: object, fluctlightId: string, placeId: string): Promise<PlaceDetail> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/places/${encodeURIComponent(placeId)}`) as Promise<PlaceDetail>;
  },
  async worldTemplates(this: object, fluctlightId: string): Promise<{ items: WorldTemplate[]; revision?: number; world_id:string; catalog_status:"runtime"; runtime_effect:"worker_executed" }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/templates`) as Promise<{ items: WorldTemplate[]; revision?: number; world_id:string; catalog_status:"runtime"; runtime_effect:"worker_executed" }>;
  },
  async setWorldTemplate(this: object, fluctlightId: string, templateId: string, body: WorldTemplateWriteBody): Promise<{ template_id: string; enabled: boolean; revision: number; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/templates/${encodeURIComponent(templateId)}`, { method: "PUT", body }) as Promise<{ template_id: string; enabled: boolean; revision: number; audit_id: string }>;
  },
  async listStudioResources(this: object, options: StudioResourceQuery = {}): Promise<CursorPage<StudioResource>> {
    return clientOf(this).json(`/api/studio/resources${q(options)}`) as Promise<CursorPage<StudioResource>>;
  },
  async studioVersions(this:object,id:string,options:{cursor?:string;limit?:number}={}):Promise<CursorPage<StudioPublicationRecord>>{return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(id)}/versions${q(options)}`) as Promise<CursorPage<StudioPublicationRecord>>;},
  async copyStudioResource(this:object,id:string,body:StudioCopyBody):Promise<StudioResource>{return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(id)}/copy`,{method:'POST',body}) as Promise<StudioResource>;},
  async archiveStudioResource(this:object,id:string,body:StudioCatalogCommand):Promise<StudioResource>{return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(id)}`,{method:'DELETE',body}) as Promise<StudioResource>;},
  async restoreStudioResource(this:object,id:string,body:StudioCatalogCommand):Promise<StudioResource>{return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(id)}/restore`,{method:'POST',body}) as Promise<StudioResource>;},
  async studioPublication(this:object,resourceId:string,version:string):Promise<StudioPublication>{
    return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(resourceId)}/versions/${encodeURIComponent(version)}`) as Promise<StudioPublication>;
  },
  async studioResource(this: object, resourceId: string): Promise<StudioResource> {
    return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(resourceId)}`) as Promise<StudioResource>;
  },
  async createStudioResource(this: object, body: StudioResourceWriteBody): Promise<StudioResource> {
    return clientOf(this).json("/api/studio/resources", { method: "POST", body }) as Promise<StudioResource>;
  },
  async updateStudioResource(this: object, resourceId: string, body: StudioResourceWriteBody): Promise<StudioResource> {
    return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(resourceId)}`, { method: "PUT", body }) as Promise<StudioResource>;
  },
  async createStudioJob(this: object, body: BrowserStudioJobBody): Promise<BrowserStudioJob> { return clientOf(this).json('/api/studio/jobs', {method:'POST',body}) as Promise<BrowserStudioJob>; },
  async studioJob(this: object,id:string):Promise<BrowserStudioJob>{ return clientOf(this).json(`/api/studio/jobs/${encodeURIComponent(id)}`) as Promise<BrowserStudioJob>; },
  async listStudioJobs(this:object,options:{resource_id?:string;cursor?:string}={}):Promise<BrowserStudioJobPage>{ return clientOf(this).json(`/api/studio/jobs${q(options)}`) as Promise<BrowserStudioJobPage>; },
  async listStudioMaterials(this:object,options:{resource_id?:string;cursor?:string;released?:boolean}={}):Promise<BrowserStudioMaterialPage>{ return clientOf(this).json(`/api/studio/materials${q(options)}`) as Promise<BrowserStudioMaterialPage>; },
  async retryStudioJob(this:object,id:string,body:BrowserStudioJobCommandBody):Promise<BrowserStudioJob>{return clientOf(this).json(`/api/studio/jobs/${encodeURIComponent(id)}/retry`,{method:'POST',body}) as Promise<BrowserStudioJob>;},
  async discardStudioJob(this:object,id:string,body:BrowserStudioJobCommandBody):Promise<BrowserStudioJob>{return clientOf(this).json(`/api/studio/jobs/${encodeURIComponent(id)}/discard`,{method:'POST',body}) as Promise<BrowserStudioJob>;},
  async saveStudioMaterial(this:object,id:string,body:BrowserStudioJobCommandBody):Promise<BrowserStudioMaterialReceipt>{return clientOf(this).json(`/api/studio/jobs/${encodeURIComponent(id)}/save-material`,{method:'POST',body}) as Promise<BrowserStudioMaterialReceipt>;},
  async createStudioPreview(this: object, body: StudioPreviewBody): Promise<StudioPreview> {
    return clientOf(this).json("/api/studio/previews", { method: "POST", body }) as Promise<StudioPreview>;
  },
  async listStudioPreviews(this: object, resourceId = ""): Promise<CursorPage<StudioPreview>> {
    return clientOf(this).json(`/api/studio/previews${q({ resourceId })}`) as Promise<CursorPage<StudioPreview>>;
  },
  async previewStudioApply(this: object, resourceId: string, body: ApplyPreviewBody): Promise<ApplyPreviewResponse> {
    return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(resourceId)}/apply-preview`, { method: "POST", body }) as Promise<ApplyPreviewResponse>;
  },
  async applyStudioResource(this: object, resourceId: string, body: ApplyResourceBody): Promise<ApplyResourceResponse> {
    return clientOf(this).json(`/api/studio/resources/${encodeURIComponent(resourceId)}/apply`, { method: "POST", body }) as Promise<ApplyResourceResponse>;
  },
  async autonomyPolicy(this: object, fluctlightId: string): Promise<AutonomyPolicy> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/autonomy-policy`) as Promise<AutonomyPolicy>;
  },
  async saveAutonomyPolicy(this: object, fluctlightId: string, body: AutonomyPolicyWriteBody): Promise<AutonomyPolicy> {
    const saved = await clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/autonomy-policy`, { method: "PUT", body }) as AutonomyPolicy;
    return saved;
  },
  async pending(this: object, fluctlightId = "", options: {kind?: PendingGroup["kind"];cursor?:string;limit?:number} = {}): Promise<PendingPage> {
    return clientOf(this).json(`/api/pending${q({ fluctlightId,...options })}`) as Promise<PendingPage>;
  },
  async pendingCount(this: object, fluctlightId = ""): Promise<{ count: number; partial: boolean }> {
    return clientOf(this).json(`/api/pending/count${q({ fluctlightId })}`) as Promise<{ count: number; partial: boolean }>;
  },
  async importStudioPack(this: object, body: import("./index.js").BrowserResourcePackImportBody): Promise<import("./index.js").BrowserResourcePackImportResult> {
    return clientOf(this).json("/api/studio/packs", { method: "POST", body }) as Promise<import("./index.js").BrowserResourcePackImportResult>;
  },
  async validateStudioPack(this: object, body: { pack: import("./index.js").BrowserResourcePack }): Promise<import("./index.js").BrowserResourcePackValidation> {
    return clientOf(this).json("/api/studio/packs/validate", { method: "POST", body }) as Promise<import("./index.js").BrowserResourcePackValidation>;
  },
  async exportStudioPack(this: object, resourceId=""): Promise<import("./index.js").BrowserResourcePackExport> {
    return clientOf(this).json(`/api/studio/packs/export${q({resourceId})}`) as Promise<import("./index.js").BrowserResourcePackExport>;
  },
  async listBackups(this: object, cursor=""): Promise<import("./index.js").BrowserBusinessBackupPage> {
    return clientOf(this).json(`/api/ops/backups${q({cursor})}`) as Promise<import("./index.js").BrowserBusinessBackupPage>;
  },
  async createBackup(this: object, body: { reason:string; idempotencyKey: string }): Promise<BackupRequestReceipt> {
    return clientOf(this).json("/api/ops/backups", { method: "POST", body }) as Promise<BackupRequestReceipt>;
  },
  async restoreBackup(this: object, backupId: string, body: { confirm: true; reason: string; targetName:string;expectedRevision:number;idempotencyKey: string }): Promise<BackupRequestReceipt> {
    return clientOf(this).json(`/api/ops/backups/${encodeURIComponent(backupId)}/restore`, { method: "POST", body }) as Promise<BackupRequestReceipt>;
  },
  async switchPersona(this: object, fluctlightId: string, body: BrowserPersonaSwitchRequest): Promise<BrowserPersonaSwitchResult> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/persona-switch`, { method: "POST", body }) as Promise<BrowserPersonaSwitchResult>;
  },
  async settingsAgents(this: object): Promise<{ agents: Array<{ id: string; name: string; description: string; tools: string[] }>; tools: Array<{ id: string; name: string; description: string }>; version: string }> {
    return clientOf(this).json("/api/settings/agents") as Promise<{ agents: Array<{ id: string; name: string; description: string; tools: string[] }>; tools: Array<{ id: string; name: string; description: string }>; version: string }>;
  },
  async desires(this: object, fluctlightId: string): Promise<{ items: DesireItem[] }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/desires`) as Promise<{ items: DesireItem[] }>;
  },
  async suggestMemoryRevision(this: object, memoryId: string, body: { text: string; reason: string; expectedRevision: number; idempotencyKey: string }): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/memories/${encodeURIComponent(memoryId)}/suggestions`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async ownerRequest<Result = Record<string, unknown>>(this: object, requestId: string): Promise<OwnerRequest<Result>> {
    return clientOf(this).json(`/api/owner-requests/${encodeURIComponent(requestId)}`) as Promise<OwnerRequest<Result>>;
  },
  async listOwnerRequests(this: object, fluctlightId: string, options: { status?: OwnerRequestStatus; cursor?: string } = {}): Promise<CursorPage<OwnerRequest>> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/owner-requests${q(options)}`) as Promise<CursorPage<OwnerRequest>>;
  },
  async proposePromise(this: object, fluctlightId: string, body: { title: string; dueAt: string; startsAt?: string; reason: string; idempotencyKey: string }): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/promises`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async suggestMoment(this: object, fluctlightId: string, body: { text: string; mediaIds: string[]; idempotencyKey: string }): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/moment-suggestions`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async listReflections(this: object, fluctlightId: string, options:{cursor?:string;limit?:number}={}): Promise<ReflectionCatalog> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/reflections${q(options)}`) as Promise<ReflectionCatalog>;
  },
  async reflectionFeedback(this:object,fluctlightId:string,reflectionId:string,options:{cursor?:string;limit?:number}={}):Promise<CursorPage<OwnerRequest>>{
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/reflections/${encodeURIComponent(reflectionId)}/feedback${q(options)}`) as Promise<CursorPage<OwnerRequest>>;
  },
};

export function applyV3Contract(ctor: { prototype: object }): void {
  Object.assign(ctor.prototype, v3ContractMethods);
}

declare module "./index.ts" {
  interface BrowserClient {
    storyOverview(fluctlightId:string):Promise<import('./index.js').BrowserStoryOverview>;
    managedMemories(fluctlightId:string,options?:MemoryBrowseOptions):Promise<CursorPage<ManagedMemory>>;
    managedMemory(memoryId:string):Promise<ManagedMemory>;
    memoryRevisions(memoryId:string,options?:{cursor?:string;limit?:number}):Promise<CursorPage<MemoryRevision>>;
    memorySources(memoryId:string,options?:{cursor?:string;limit?:number;revision?:number}):Promise<CursorPage<MemorySource>>;
    rollbackMemory(memoryId:string,body:{targetRevision:number;expectedRevision:number;evidenceRefs:string[]}):Promise<Record<string,unknown>>;
    suggestScheduleItem(fluctlightId: string, scheduleId: string, body: ScheduleItemSuggestionBody): Promise<OwnerRequestEnvelope>;
    updateScheduleItem(fluctlightId: string, scheduleId: string, itemId: string, body: ScheduleItemSuggestionBody): Promise<OwnerRequestEnvelope>;
    deleteScheduleItem(fluctlightId: string, scheduleId: string, itemId: string, body: ScheduleItemDeleteBody): Promise<OwnerRequestEnvelope>;
    requestWardrobeWear(fluctlightId: string, body: WearRequestBody): Promise<OwnerRequestEnvelope>;
    requestItemUse(fluctlightId: string, itemId: string, body: ItemUseRequestBody): Promise<OwnerRequestEnvelope>;
    requestItemPhoto(fluctlightId: string, itemId: string, body: ItemPhotoRequestBody): Promise<ItemPhotoResponse>;
    listCharacterMedia(fluctlightId: string, options?: { cursor?: string; limit?: number }): Promise<CursorPage<CharacterMediaItem>>;
    sidebar(fluctlightId: string): Promise<SidebarSummary>;
    createMoment(body: CreateMomentBody): Promise<CreateMomentResponse>;
    uploadOwnerMedia(file: Blob, mime: string, idempotencyKey: string, filename?: string): Promise<UploadMediaResponse>;
    deleteActorGroup(groupId: string, body: { expectedRevision: 0; idempotencyKey: string }): Promise<void>;
    harmIntercepts(options?: { fluctlightId?: string; window?: string }): Promise<HarmIntercepts>;
    diagnosticTask(taskId: string): Promise<DiagnosticTask>;
    saveVisualIdentity(fluctlightId: string, body: VisualIdentityWriteBody): Promise<import("./index.js").BrowserMediaAvatar>;
    listNotifications(options?: { cursor?: string; limit?: number; unread?: boolean; fluctlightId?: string }): Promise<NotificationPage>;
    markNotificationRead(notificationId: string, body: { idempotencyKey: string }): Promise<{ id: string; read_at: string }>;
    markAllNotificationsRead(body: { before: string; idempotencyKey: string }): Promise<{ updated: number; before: string }>;
    unreadNotificationCount(): Promise<{ count: number; alert_count: number }>;
    nearby(fluctlightId: string): Promise<NearbyActors>;
    selfClaims(fid:string,options?:{q?:string;status?:string;category?:string;cursor?:string;limit?:number}):Promise<import("./index.js").BrowserSelfPage>;
    selfClaim(fid:string,id:string):Promise<import("./index.js").BrowserSelfClaim>;
    selfClaimHistory(fid:string,id:string,kind:'revisions'|'reviews',cursor?:string):Promise<import("./index.js").BrowserSelfHistory>;
    manageSelfClaim(fid:string,id:string,...args: [command:'review'|'forget',body:import("./index.js").BrowserSelfCommandBody] | [command:'restore',body:import("./index.js").BrowserSelfRestoreBody]):Promise<import("./index.js").BrowserSelfCommandResult>;
    listDiary(fluctlightId: string, cursor?: string, options?:{favorite?:boolean;limit?:number}): Promise<import("./index.js").BrowserDiaryPage>;
    diary(fluctlightId:string,id:string):Promise<DiaryEntry>;
    generateDiary(fluctlightId:string,body:import("./index.js").BrowserDiaryGenerateBody):Promise<import("./index.js").BrowserDiaryGeneration>;
    annotateDiary(fluctlightId:string,id:string,body:import("./index.js").BrowserDiaryAnnotationBody):Promise<import("./index.js").BrowserDiaryAnnotationResult>;
    diaryReplies(fluctlightId:string,id:string,cursor?:string):Promise<import("./index.js").BrowserDiaryReplies>;
    replyToDiary(fluctlightId:string,id:string,body:import("./index.js").BrowserDiaryReplyBody):Promise<OwnerRequestEnvelope>;
    sharedExperiences(fluctlightId:string,cursor?:string):Promise<import("./index.js").BrowserSharedExperiencePage>;
    createDiaryEntry(fluctlightId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }>;
    updateDiaryEntry(fluctlightId: string, entryId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }>;
    reflection(fluctlightId: string, reflectionId: string): Promise<ReflectionDetail>;
    submitReflectionFeedback(fluctlightId: string, reflectionId: string, body: ReflectionFeedbackBody): Promise<OwnerRequestEnvelope>;
    reflectionFeedback(fluctlightId:string,reflectionId:string,options?:{cursor?:string;limit?:number}):Promise<CursorPage<OwnerRequest>>;
    worldCatalog(fluctlightId: string): Promise<WorldCatalog>;
    catalogWorld(fluctlightId: string, worldId: string): Promise<WorldView>;
    createCatalogWorld(fluctlightId: string, body: WorldCreateBody): Promise<WorldWriteResult>;
    updateCatalogWorld(fluctlightId: string, worldId: string, body: WorldWriteBody): Promise<WorldWriteResult>;
    deleteCatalogWorld(fluctlightId: string, worldId: string, body: WorldCommand): Promise<WorldDeleteResult>;
    selectWorld(fluctlightId: string, body: WorldSelectBody): Promise<WorldWriteResult>;
    deleteWorldPlace(fluctlightId: string, placeId: string, body: WorldDeleteBody): Promise<WorldDeleteResult>;
    deleteRuntimeNPC(fluctlightId: string, npcId: string, body: WorldCommand): Promise<WorldDeleteResult>;
    world(fluctlightId: string): Promise<WorldView>;
    updateRuntimeWorld(fluctlightId: string, body: WorldWriteBody): Promise<WorldWriteResult>;
    createWorldPlace(fluctlightId: string, body: PlaceWriteBody): Promise<WorldWriteResult>;
    updateWorldPlace(fluctlightId: string, placeId: string, body: PlaceWriteBody): Promise<WorldWriteResult>;
    listRuntimeNPCs(fluctlightId: string): Promise<CursorPage<RuntimeNPC>>;
    runtimeNPC(fluctlightId: string, npcId: string): Promise<RuntimeNPC>;
    updateRuntimeNPC(fluctlightId: string, npcId: string, body: NPCWriteBody): Promise<WorldWriteResult>;
    recordNPCEncounter(fluctlightId: string, npcId: string, body: NPCEncounterBody): Promise<{ id: string; event_id: string; npc_id: string; relationship_id: string; memory_id: string; status: string; audit_id: string }>;
    publishStudioResource(resourceId: string, body: WorldCommand): Promise<{ id: string; revision: number; published_version: string; status: 'published' }>;
    placeDetail(fluctlightId: string, placeId: string): Promise<PlaceDetail>;
    worldTemplates(fluctlightId: string): Promise<{ items: WorldTemplate[]; revision?: number; world_id:string; catalog_status:"runtime"; runtime_effect:"worker_executed" }>;
    setWorldTemplate(fluctlightId: string, templateId: string, body: WorldTemplateWriteBody): Promise<{ template_id: string; enabled: boolean; revision: number; audit_id: string }>;
    listStudioResources(options?: StudioResourceQuery): Promise<CursorPage<StudioResource>>;
    studioResource(resourceId: string): Promise<StudioResource>;
    studioPublication(resourceId:string,version:string):Promise<StudioPublication>;
    studioVersions(id:string,options?:{cursor?:string;limit?:number}):Promise<CursorPage<StudioPublicationRecord>>;
    copyStudioResource(id:string,body:StudioCopyBody):Promise<StudioResource>;
    archiveStudioResource(id:string,body:StudioCatalogCommand):Promise<StudioResource>;
    restoreStudioResource(id:string,body:StudioCatalogCommand):Promise<StudioResource>;
    createStudioResource(body: StudioResourceWriteBody): Promise<StudioResource>;
    updateStudioResource(resourceId: string, body: StudioResourceWriteBody): Promise<StudioResource>;
    createStudioJob(body:BrowserStudioJobBody):Promise<BrowserStudioJob>;
    studioJob(id:string):Promise<BrowserStudioJob>;
    listStudioJobs(options?:{resource_id?:string;cursor?:string}):Promise<BrowserStudioJobPage>;
    listStudioMaterials(options?:{resource_id?:string;cursor?:string;released?:boolean}):Promise<BrowserStudioMaterialPage>;
    retryStudioJob(id:string,body:BrowserStudioJobCommandBody):Promise<BrowserStudioJob>;
    discardStudioJob(id:string,body:BrowserStudioJobCommandBody):Promise<BrowserStudioJob>;
    saveStudioMaterial(id:string,body:BrowserStudioJobCommandBody):Promise<BrowserStudioMaterialReceipt>;
    createStudioPreview(body: StudioPreviewBody): Promise<StudioPreview>;
    listStudioPreviews(resourceId?: string): Promise<CursorPage<StudioPreview>>;
    previewStudioApply(resourceId: string, body: ApplyPreviewBody): Promise<ApplyPreviewResponse>;
    applyStudioResource(resourceId: string, body: ApplyResourceBody): Promise<ApplyResourceResponse>;
    autonomyPolicy(fluctlightId: string): Promise<AutonomyPolicy>;
    saveAutonomyPolicy(fluctlightId: string, body: AutonomyPolicyWriteBody): Promise<AutonomyPolicy>;
    pending(fluctlightId?: string, options?: {kind?: PendingGroup["kind"];cursor?:string;limit?:number}): Promise<PendingPage>;
    pendingCount(fluctlightId?: string): Promise<{ count: number; partial: boolean }>;
    importStudioPack(body: import("./index.js").BrowserResourcePackImportBody): Promise<import("./index.js").BrowserResourcePackImportResult>;
    validateStudioPack(body: { pack: import("./index.js").BrowserResourcePack }): Promise<import("./index.js").BrowserResourcePackValidation>;
    exportStudioPack(resourceId?:string): Promise<import("./index.js").BrowserResourcePackExport>;
    listBackups(cursor?:string): Promise<import("./index.js").BrowserBusinessBackupPage>;
    createBackup(body: { reason:string;idempotencyKey: string }): Promise<BackupRequestReceipt>;
    restoreBackup(backupId: string, body: { confirm: true; reason: string; targetName:string;expectedRevision:number;idempotencyKey: string }): Promise<BackupRequestReceipt>;
    switchPersona(fluctlightId: string, body: BrowserPersonaSwitchRequest): Promise<BrowserPersonaSwitchResult>;
    settingsAgents(): Promise<{ agents: Array<{ id: string; name: string; description: string; tools: string[] }>; tools: Array<{ id: string; name: string; description: string }>; version: string }>;
    desires(fluctlightId: string): Promise<{ items: DesireItem[] }>;
    suggestMemoryRevision(memoryId: string, body: { text: string; reason: string; expectedRevision: number; idempotencyKey: string }): Promise<OwnerRequestEnvelope>;
    ownerRequest<Result = Record<string, unknown>>(requestId: string): Promise<OwnerRequest<Result>>;
    listOwnerRequests(fluctlightId: string, options?: { status?: OwnerRequestStatus; cursor?: string }): Promise<CursorPage<OwnerRequest>>;
    proposePromise(fluctlightId: string, body: { title: string; dueAt: string; startsAt?: string; reason: string; idempotencyKey: string }): Promise<OwnerRequestEnvelope>;
    suggestMoment(fluctlightId: string, body: { text: string; mediaIds: string[]; idempotencyKey: string }): Promise<OwnerRequestEnvelope>;
    listReflections(fluctlightId: string,options?:{cursor?:string;limit?:number}): Promise<ReflectionCatalog>;
  }
}
