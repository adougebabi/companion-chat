// 摇光浏览器契约 v0.1（uploads/fluctlight-backend-contract.md）。
// 响应字段 snake_case。JSON 写操作把幂等键放在请求体 idempotencyKey（Q1 待裁决：
// CORS 目前不放行 Idempotency-Key 头，Allow-Methods 也可能没有 PATCH）。
// 契约仍写了 PATCH（日程条目、日记），这里按契约当前写法发送 PATCH。
// multipart 上传无法把键放进 JSON 体，按契约 B-12 放在请求头 Idempotency-Key。
// C-14 时间线回档不定义任何方法。

export type OwnerRequestStatus = "pending" | "processing" | "accepted" | "rejected" | "failed";

export type OwnerRequest = {
  id: string;
  kind: string;
  fluctlight_id: string;
  status: OwnerRequestStatus;
  payload: Record<string, unknown>;
  decided_by: "agent" | null;
  decision_reason: string | null;
  result_refs: string[];
  created_at: string;
  updated_at: string;
  revision: number;
};

export type OwnerRequestEnvelope = { request: OwnerRequest; audit_id?: string; schedule_revision?: number };

export type CursorPage<T> = { items: T[]; next_cursor: string | null };

export type FactLabel = { factual: false; label: "生成图·非事实" | "插画" };

export type SceneImage = { asset_id: string; url: string | null; captured_at: string; factual: false; label: "生成图·非事实" };

export type EvidenceState = { label: string; state: "推测" | "已确认"; evidence: string[]; updated_at: string };

export type ScheduleItemSuggestionBody = {
  activity: string;
  scene?: string;
  location?: string;
  startAt: string;
  endAt: string;
  evidenceRefs: string[];
  expectedRevision: number;
  expectedLifeContextRevision: number;
  reason: string;
  idempotencyKey: string;
};

export type ScheduleItemDeleteBody = {
  expectedRevision: number;
  expectedLifeContextRevision: number;
  reason: string;
  idempotencyKey: string;
};

export type WearRequestBody = { itemId: string; slot: string; expectedRevision?: number; reason: string; idempotencyKey: string; confirm?: true };

export type ItemUseRequestBody = { active: boolean; expectedRevision?: number; reason: string; evidenceRefs: string[]; idempotencyKey: string; confirm?: true };

export type ItemPhotoRequestBody = { prompt: string; outfitSnapshotId?: string; idempotencyKey: string; expectedRevision?: never };

export type ItemPhotoResponse = { asset_id: string; status: "queued" | "ready" | "failed"; factual: false; label: "生成图·非事实" };

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

export type UploadMediaResponse = { asset_id: string; kind: string; mime: string; size: number; factual: true; uploaded_by: "owner" };

export type HarmIntercepts = { count: number; window: string; latest_reason: string | null; items: Array<{ id: string; at: string; text: string; rule: string }> };

export type DiagnosticTask = {
  task_id: string;
  type: string;
  title: string;
  initiator: string;
  state: string;
  runs: number;
  correlation_id: string;
  fluctlight_id: string;
  action_id: string | null;
  steps: Array<{ n: number; name: string; status: "ok" | "failed" | "pending" | "running"; idempotency_key: string; error_code: string | null; error_message: string | null }>;
};

export type VisualIdentityWriteBody = { source: "owner_upload"; kind: "visual_identity"; identitySnapshot?: Record<string, unknown>; expectedRevision: number; reason: string; idempotencyKey: string };

export type NotificationItem = { id: string; title: string; body: string; link: string; read_at: string | null; created_at: string; source_kind: string; source_id: string };

export type NearbyActors = { place_id: string | null; actors: Array<{ actor_id: string; name: string; npc: boolean; full_fluctlight: boolean; since: string }> };

export type DiaryEntry = { id: string; date: string; body: string; occurred_at: string; event_id: string | null; refs: string[]; source: "agent" | "owner"; replies: Array<{ id: string; actor_id: string; text: string; at: string }>; revision: number };

export type DiaryWriteBody = { body: string; occurredAt: string; eventId?: string; expectedRevision?: number; reason: string; idempotencyKey: string };

export type ReflectionDetail = { id: string; title: string; body: string; state: string; growth: string | null; at: string; responded_at: string | null; claim_ids: string[]; source_refs: string[]; feedback: string | null };

export type WorldView = {
  world: { id: string; name: string; def_version?: string };
  places: Array<{ id: string; name: string; desc: string; scene_asset: { url: string | null; label: string } | null; resource_id: string | null; override: Record<string, unknown> | null }>;
  scenes: unknown[];
  map: Record<string, unknown> | null;
  npcs: Array<{ id: string; name: string; role: string; place_id: string; avatar_url: string | null; full_fluctlight: boolean; bio: string }>;
};

export type PlaceDetail = WorldView["places"][number] & { present_actors: unknown[]; related_event_ids: string[]; related_memory_ids: string[] };

export type WorldTemplate = { template_id: string; name: string; category: string; enabled: boolean; permanent_harm: boolean; locked: boolean };

export type WorldTemplateWriteBody = { enabled: boolean; expectedRevision: number; reason: string; idempotencyKey: string };

export type StudioResource = {
  id: string;
  kind: "character_template" | "persona_fragment" | "place" | "item" | "event_template" | "visual_style";
  name: string;
  published_version: string | null;
  draft_version: string | null;
  versions: Array<{ version: string; created_at: string; body?: Record<string, unknown> }>;
  referenced_by: Array<{ fluctlight_id: string; selected_version?: string; status?: string; instance_revision?: number | null; running_version?: string; overrides?: Record<string, unknown> }>;
  audit_id?: string;
  body?: Record<string, unknown>;
};

export type StudioResourceWriteBody = { kind: StudioResource["kind"]; name: string; body: Record<string, unknown>; expectedVersion?: string; reason: string; idempotencyKey: string };

export type StudioPreview = { preview_id: string; text: string; label: "创作预览·不进入生活"; life_write: false; created_at: string; resource_id?: string | null };

export type StudioPreviewBody = { resourceId?: string; text: string; idempotencyKey: string };

export type ApplyPreviewBody = { fluctlightId: string; resourceVersion: string };

export type ApplyPreviewResponse = { diff: Array<{ field: string; from: unknown; to: unknown }>; conflicts: unknown[]; instance_revision: number };

export type ApplyResourceBody = { fluctlightId: string; resourceVersion: string; expectedInstanceRevision: number; choices: Array<{ field: string; decision: "keep" | "override" }>; reason: string; idempotencyKey: string };

export type ApplyResourceResponse = {
  status: "recorded_not_applied";
  applied: false;
  runtime_effect?: "recorded_only";
  resource_id?: string;
  resource_version?: string;
  instance_revision: number | null;
  instance_revision_unchanged?: boolean;
  choices?: Array<{ field: string; decision: string }>;
  conflicts: null;
  audit_id?: string;
  available?: boolean;
};

/** 创作应用返回 recorded_not_applied / applied:false 时，选择已记录，实例版本没有变。 */
export function studioApplyUnfinished(response: { status?: string; applied?: boolean; runtime_effect?: string; available?: boolean } | null | undefined): boolean {
  if (!response || response.available === false) return true;
  if (response.applied === false || response.status === "recorded_not_applied") return true;
  return response.runtime_effect === "recorded_only" || response.status === "not_implemented";
}

export type AutonomyPolicy = {
  level: string;
  quiet_hours: { start: string; end: string } | null;
  spend_cap: { amount: number; currency: string; period: string } | null;
  outreach_limit: "low" | "medium" | "high" | null;
  allow_moments: boolean;
  confirm_action_types: string[];
  revision?: number;
  effective_at?: string;
  runtime_effect?: "preference_only";
  audit_id?: string;
};

export type AutonomyPolicyWriteBody = AutonomyPolicy & { expectedRevision: number; reason: string; idempotencyKey: string };

export type PendingGroup = { kind: "capability" | "persona_proposal" | "studio_apply" | "failed_task" | "owner_request"; status: "ok" | "unwired"; items?: Array<{ id: string; title: string; href: string; created_at: string }> };

export type BackupRecord = { backup_id: string; created_at: string; size_bytes: number | null; audit_id?: string };

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
  async uploadOwnerMedia(this: object, file: Blob, mime: string, idempotencyKey: string): Promise<UploadMediaResponse> {
    const runtime = clientOf(this);
    const form = new FormData();
    form.set("file", file);
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
  async saveVisualIdentity(this: object, fluctlightId: string, body: VisualIdentityWriteBody): Promise<{ current_revision: number; status: "saved"; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/visual-identity`, { method: "PUT", body }) as Promise<{ current_revision: number; status: "saved"; audit_id: string }>;
  },
  async listNotifications(this: object, options: { cursor?: string; limit?: number; unread?: boolean } = {}): Promise<CursorPage<NotificationItem>> {
    return clientOf(this).json(`/api/notifications${q(options)}`) as Promise<CursorPage<NotificationItem>>;
  },
  async markNotificationRead(this: object, notificationId: string, body: { idempotencyKey: string }): Promise<{ id: string; read_at: string }> {
    return clientOf(this).json(`/api/notifications/${encodeURIComponent(notificationId)}/read`, { method: "POST", body }) as Promise<{ id: string; read_at: string }>;
  },
  async unreadNotificationCount(this: object): Promise<{ count: number }> {
    return clientOf(this).json("/api/notifications/unread-count") as Promise<{ count: number }>;
  },
  async nearby(this: object, fluctlightId: string): Promise<NearbyActors> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/nearby`) as Promise<NearbyActors>;
  },
  async listDiary(this: object, fluctlightId: string, cursor = ""): Promise<CursorPage<DiaryEntry>> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary${q({ cursor })}`) as Promise<CursorPage<DiaryEntry>>;
  },
  async createDiaryEntry(this: object, fluctlightId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary`, { method: "POST", body }) as Promise<{ id: string; revision: number; source: string; audit_id: string }>;
  },
  async updateDiaryEntry(this: object, fluctlightId: string, entryId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/diary/${encodeURIComponent(entryId)}`, { method: "PUT", body }) as Promise<{ id: string; revision: number; source: string; audit_id: string }>;
  },
  async reflection(this: object, fluctlightId: string, reflectionId: string): Promise<ReflectionDetail> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/reflections/${encodeURIComponent(reflectionId)}`) as Promise<ReflectionDetail>;
  },
  async submitReflectionFeedback(this: object, fluctlightId: string, reflectionId: string, body: { text: string; idempotencyKey: string }): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/reflections/${encodeURIComponent(reflectionId)}/feedback`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async world(this: object, fluctlightId: string): Promise<WorldView> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world`) as Promise<WorldView>;
  },
  async placeDetail(this: object, fluctlightId: string, placeId: string): Promise<PlaceDetail> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/places/${encodeURIComponent(placeId)}`) as Promise<PlaceDetail>;
  },
  async worldTemplates(this: object, fluctlightId: string): Promise<{ items: WorldTemplate[]; revision: number }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/templates`) as Promise<{ items: WorldTemplate[]; revision: number }>;
  },
  async setWorldTemplate(this: object, fluctlightId: string, templateId: string, body: WorldTemplateWriteBody): Promise<{ template_id: string; enabled: boolean; revision: number; audit_id: string }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/world/templates/${encodeURIComponent(templateId)}`, { method: "PUT", body }) as Promise<{ template_id: string; enabled: boolean; revision: number; audit_id: string }>;
  },
  async listStudioResources(this: object, options: { kind?: string; cursor?: string } = {}): Promise<CursorPage<StudioResource>> {
    return clientOf(this).json(`/api/studio/resources${q(options)}`) as Promise<CursorPage<StudioResource>>;
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
  async pending(this: object, fluctlightId = ""): Promise<{ groups: PendingGroup[] }> {
    return clientOf(this).json(`/api/pending${q({ fluctlightId })}`) as Promise<{ groups: PendingGroup[] }>;
  },
  async pendingCount(this: object, fluctlightId = ""): Promise<{ count: number; partial: boolean }> {
    return clientOf(this).json(`/api/pending/count${q({ fluctlightId })}`) as Promise<{ count: number; partial: boolean }>;
  },
  async importStudioPack(this: object, body: { pack: Record<string, unknown>; idempotencyKey: string }): Promise<{ pack_id: string; resource_versions: Array<{ resource_id: string; version: string }>; audit_id: string; status?: string }> {
    return clientOf(this).json("/api/studio/packs", { method: "POST", body }) as Promise<{ pack_id: string; resource_versions: Array<{ resource_id: string; version: string }>; audit_id: string; status?: string }>;
  },
  async validateStudioPack(this: object, body: { pack: Record<string, unknown>; idempotencyKey: string }): Promise<{ valid?: boolean; status?: string; available?: boolean }> {
    return clientOf(this).json("/api/studio/packs/validate", { method: "POST", body }) as Promise<{ valid?: boolean; status?: string; available?: boolean }>;
  },
  async exportStudioPack(this: object): Promise<{ pack: Record<string, unknown>; available?: boolean }> {
    return clientOf(this).json("/api/studio/packs/export") as Promise<{ pack: Record<string, unknown>; available?: boolean }>;
  },
  async listBackups(this: object): Promise<CursorPage<BackupRecord>> {
    return clientOf(this).json("/api/ops/backups") as Promise<CursorPage<BackupRecord>>;
  },
  async createBackup(this: object, body: { idempotencyKey: string }): Promise<BackupRecord> {
    return clientOf(this).json("/api/ops/backups", { method: "POST", body }) as Promise<BackupRecord>;
  },
  async restoreBackup(this: object, backupId: string, body: { confirm: true; reason: string; idempotencyKey: string }): Promise<{ job_id: string; audit_id: string }> {
    return clientOf(this).json(`/api/ops/backups/${encodeURIComponent(backupId)}/restore`, { method: "POST", body }) as Promise<{ job_id: string; audit_id: string }>;
  },
  async switchPersona(this: object, fluctlightId: string, body: { targetPersonaId: string; expectedRevision: number; reason: string; confirm: true; idempotencyKey: string }): Promise<{ revision: number; audit_id: string; status: "pending" }> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/persona-switch`, { method: "POST", body }) as Promise<{ revision: number; audit_id: string; status: "pending" }>;
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
  async ownerRequest(this: object, requestId: string): Promise<OwnerRequest> {
    return clientOf(this).json(`/api/owner-requests/${encodeURIComponent(requestId)}`) as Promise<OwnerRequest>;
  },
  async listOwnerRequests(this: object, fluctlightId: string, options: { status?: OwnerRequestStatus; cursor?: string } = {}): Promise<CursorPage<OwnerRequest>> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/owner-requests${q(options)}`) as Promise<CursorPage<OwnerRequest>>;
  },
  async proposePromise(this: object, fluctlightId: string, body: { title: string; dueAt: string; reason: string; idempotencyKey: string }): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/promises`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  async suggestMoment(this: object, fluctlightId: string, body: { text: string; mediaIds: string[]; idempotencyKey: string }): Promise<OwnerRequestEnvelope> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/moment-suggestions`, { method: "POST", body }) as Promise<OwnerRequestEnvelope>;
  },
  // 待裁决：契约 C-04 只有按 id 读取，反思时间线没有列表路由。缺响应按未接入，不编造条目。
  async listReflections(this: object, fluctlightId: string): Promise<CursorPage<ReflectionDetail>> {
    return clientOf(this).json(`/api/fluctlights/${encodeURIComponent(fluctlightId)}/reflections`) as Promise<CursorPage<ReflectionDetail>>;
  },
};

export function applyV3Contract(ctor: { prototype: object }): void {
  Object.assign(ctor.prototype, v3ContractMethods);
}

declare module "./index.ts" {
  interface BrowserClient {
    suggestScheduleItem(fluctlightId: string, scheduleId: string, body: ScheduleItemSuggestionBody): Promise<OwnerRequestEnvelope>;
    updateScheduleItem(fluctlightId: string, scheduleId: string, itemId: string, body: ScheduleItemSuggestionBody): Promise<OwnerRequestEnvelope>;
    deleteScheduleItem(fluctlightId: string, scheduleId: string, itemId: string, body: ScheduleItemDeleteBody): Promise<OwnerRequestEnvelope>;
    requestWardrobeWear(fluctlightId: string, body: WearRequestBody): Promise<OwnerRequestEnvelope>;
    requestItemUse(fluctlightId: string, itemId: string, body: ItemUseRequestBody): Promise<OwnerRequestEnvelope>;
    requestItemPhoto(fluctlightId: string, itemId: string, body: ItemPhotoRequestBody): Promise<ItemPhotoResponse>;
    listCharacterMedia(fluctlightId: string, options?: { cursor?: string; limit?: number }): Promise<CursorPage<CharacterMediaItem>>;
    sidebar(fluctlightId: string): Promise<SidebarSummary>;
    createMoment(body: CreateMomentBody): Promise<CreateMomentResponse>;
    uploadOwnerMedia(file: Blob, mime: string, idempotencyKey: string): Promise<UploadMediaResponse>;
    deleteActorGroup(groupId: string, body: { expectedRevision: 0; idempotencyKey: string }): Promise<void>;
    harmIntercepts(options?: { fluctlightId?: string; window?: string }): Promise<HarmIntercepts>;
    diagnosticTask(taskId: string): Promise<DiagnosticTask>;
    saveVisualIdentity(fluctlightId: string, body: VisualIdentityWriteBody): Promise<{ current_revision: number; status: "saved"; audit_id: string }>;
    listNotifications(options?: { cursor?: string; limit?: number; unread?: boolean }): Promise<CursorPage<NotificationItem>>;
    markNotificationRead(notificationId: string, body: { idempotencyKey: string }): Promise<{ id: string; read_at: string }>;
    unreadNotificationCount(): Promise<{ count: number }>;
    nearby(fluctlightId: string): Promise<NearbyActors>;
    listDiary(fluctlightId: string, cursor?: string): Promise<CursorPage<DiaryEntry>>;
    createDiaryEntry(fluctlightId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }>;
    updateDiaryEntry(fluctlightId: string, entryId: string, body: DiaryWriteBody): Promise<{ id: string; revision: number; source: string; audit_id: string }>;
    reflection(fluctlightId: string, reflectionId: string): Promise<ReflectionDetail>;
    submitReflectionFeedback(fluctlightId: string, reflectionId: string, body: { text: string; idempotencyKey: string }): Promise<OwnerRequestEnvelope>;
    world(fluctlightId: string): Promise<WorldView>;
    placeDetail(fluctlightId: string, placeId: string): Promise<PlaceDetail>;
    worldTemplates(fluctlightId: string): Promise<{ items: WorldTemplate[]; revision: number }>;
    setWorldTemplate(fluctlightId: string, templateId: string, body: WorldTemplateWriteBody): Promise<{ template_id: string; enabled: boolean; revision: number; audit_id: string }>;
    listStudioResources(options?: { kind?: string; cursor?: string }): Promise<CursorPage<StudioResource>>;
    studioResource(resourceId: string): Promise<StudioResource>;
    createStudioResource(body: StudioResourceWriteBody): Promise<StudioResource>;
    updateStudioResource(resourceId: string, body: StudioResourceWriteBody): Promise<StudioResource>;
    createStudioPreview(body: StudioPreviewBody): Promise<StudioPreview>;
    listStudioPreviews(resourceId?: string): Promise<CursorPage<StudioPreview>>;
    previewStudioApply(resourceId: string, body: ApplyPreviewBody): Promise<ApplyPreviewResponse>;
    applyStudioResource(resourceId: string, body: ApplyResourceBody): Promise<ApplyResourceResponse>;
    autonomyPolicy(fluctlightId: string): Promise<AutonomyPolicy>;
    saveAutonomyPolicy(fluctlightId: string, body: AutonomyPolicyWriteBody): Promise<AutonomyPolicy>;
    pending(fluctlightId?: string): Promise<{ groups: PendingGroup[] }>;
    pendingCount(fluctlightId?: string): Promise<{ count: number; partial: boolean }>;
    importStudioPack(body: { pack: Record<string, unknown>; idempotencyKey: string }): Promise<{ pack_id: string; resource_versions: Array<{ resource_id: string; version: string }>; audit_id: string; status?: string }>;
    validateStudioPack(body: { pack: Record<string, unknown>; idempotencyKey: string }): Promise<{ valid?: boolean; status?: string; available?: boolean }>;
    exportStudioPack(): Promise<{ pack: Record<string, unknown>; available?: boolean }>;
    listBackups(): Promise<CursorPage<BackupRecord>>;
    createBackup(body: { idempotencyKey: string }): Promise<BackupRecord>;
    restoreBackup(backupId: string, body: { confirm: true; reason: string; idempotencyKey: string }): Promise<{ job_id: string; audit_id: string }>;
    switchPersona(fluctlightId: string, body: { targetPersonaId: string; expectedRevision: number; reason: string; confirm: true; idempotencyKey: string }): Promise<{ revision: number; audit_id: string; status: "pending" }>;
    settingsAgents(): Promise<{ agents: Array<{ id: string; name: string; description: string; tools: string[] }>; tools: Array<{ id: string; name: string; description: string }>; version: string }>;
    desires(fluctlightId: string): Promise<{ items: DesireItem[] }>;
    suggestMemoryRevision(memoryId: string, body: { text: string; reason: string; expectedRevision: number; idempotencyKey: string }): Promise<OwnerRequestEnvelope>;
    ownerRequest(requestId: string): Promise<OwnerRequest>;
    listOwnerRequests(fluctlightId: string, options?: { status?: OwnerRequestStatus; cursor?: string }): Promise<CursorPage<OwnerRequest>>;
    proposePromise(fluctlightId: string, body: { title: string; dueAt: string; reason: string; idempotencyKey: string }): Promise<OwnerRequestEnvelope>;
    suggestMoment(fluctlightId: string, body: { text: string; mediaIds: string[]; idempotencyKey: string }): Promise<OwnerRequestEnvelope>;
    listReflections(fluctlightId: string): Promise<CursorPage<ReflectionDetail>>;
  }
}
