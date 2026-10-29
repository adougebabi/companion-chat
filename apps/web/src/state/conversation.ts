import { ref as untracked } from "valtio/vanilla";
import { defineDomain, createDomainStore } from "./domain-store";
import {
  BrowserClient,
  BrowserApiError,
  type BrowserConversation,
  type BrowserParticipant,
  type BrowserMessage,
  type BrowserTurnEvent,
} from "@fluctlight/browser-client";
import { apiOrigin } from "../runtime-config";
import { randomId } from "../random-id";

const client = new BrowserClient(apiOrigin);

const turnErrorCodes = new Set([
  "reply_media_no_progress",
  "active_memory_capability_unavailable",
  "active_memory_plan_invalid",
  "active_memory_plan_stale",
  "affect_capability_unavailable",
  "affect_idempotency_conflict",
  "affect_profile_revision_conflict",
  "affect_source_revision_conflict",
  "affect_state_revision_conflict",
  "appraisal_required",
  "capability_busy",
  "capability_execution_class_invalid",
  "capability_execution_failed",
  "capability_not_found",
  "capability_output_invalid",
  "capability_prepare_failed",
  "capability_prepared_payload_invalid",
  "capability_request_arguments_invalid",
  "capability_request_contract_invalid",
  "capability_result_identity_mismatch",
  "capability_result_missing",
  "capability_result_status_invalid",
  "capability_settlement_failed",
  "adk_final_message_empty",
  "adk_final_message_missing",
  "adk_final_output_invalid",
  "adk_final_output_missing",
  "adk_final_text_missing",
  "adk_run_failed",
  "agent_cognition_settlement_failed",
  "agent_final_contract_invalid",
  "agent_output_publication_failed",
  "agent_partial_committed",
  "agent_run_failed",
  "cognition_visible_text_missing",
  "conversation_native_output_missing",
  "conversation_not_found",
  "conversation_settlement_failed",
  "conversation_turn_conflict",
  "conversation_turn_failed",
  "conversation_turn_invalid",
  "conversation_unauthorized",
  "current_state_revision_stale",
  "decision_effect_invalid",
  "execution_cancelled",
  "execution_failed",
  "foundation_revision_stale",
  "fluctlight_inactive",
  "fluctlight_paused",
  "frozen_context_projection_missing",
  "life_context_stale",
  "media_arguments_invalid",
  "media_capability_unavailable",
  "media_context_stale",
  "media_intent_failed",
  "media_intent_invalid",
  "media_prepare_required",
  "memory_capability_unavailable",
  "memory_plan_invalid",
  "memory_plan_stale",
  "memory_recall_failed",
  "memory_recall_unavailable",
  "moment_arguments_invalid",
  "moment_target_invalid",
  "moment_text_invalid",
  "native_test_plan_invalid",
  "native_test_retryable",
  "native_test_wrong_execution_path",
  "personality_decision_plan_invalid",
  "presence_capability_unavailable",
  "presence_context_stale",
  "presence_idempotency_conflict",
  "presence_plan_expired",
  "presence_plan_invalid",
  "presence_prepared_context_mismatch",
  "presence_replay_result_invalid",
  "presence_resulting_context_invalid",
  "provider_request_failed",
  "query_continuation_contract_invalid",
  "query_continuation_digest_invalid",
  "query_continuation_query_failed",
  "query_continuation_tool_call_forbidden",
  "relationship_lookup_arguments_invalid",
  "relationship_lookup_failed",
  "relationship_lookup_not_found",
  "relationship_lookup_owner_failed",
  "relationship_lookup_scope_failed",
  "relationship_lookup_target_forbidden",
  "relationship_lookup_target_invalid",
  "relationship_lookup_target_required",
  "request_cancelled",
  "request_timeout",
  "required_capability_failed",
  "scene_capability_unavailable",
  "scene_context_stale",
  "scene_idempotency_conflict",
  "scene_plan_expired",
  "scene_plan_invalid",
  "scene_prepared_context_mismatch",
  "scene_replay_result_invalid",
  "scene_resulting_context_invalid",
  "schedule_prepared_context_mismatch",
  "schedule_replan_arguments_invalid",
  "schedule_replan_context_stale",
  "schedule_replan_failed",
  "schedule_replan_local_date_invalid",
  "schedule_replan_owner_missing",
  "schedule_replan_persist_failed",
  "schedule_replan_planner_failed",
  "schedule_replan_revision_invalid",
  "schedule_replan_schedule_read_failed",
  "schedule_replan_timezone_invalid",
  "structured_turn_settlement_failed",
  "takeover_failed",
  "takeover_frozen_turn_missing",
  "takeover_reply_budget_exhausted",
  "takeover_resume_decision_invalid",
  "takeover_resume_rule_missing",
  "takeover_target_profile_missing",
  "tool_call_invalid",
  "tool_call_failed",
  "tool_execution_failed",
  "turn_stage_invalid",
  "turn_stage_not_executable",
  "visible_text_source_conflict",
  "user_cancelled",
  "provider_gateway_timeout",
  "reply_gateway_timeout",
  "reply_quality_repair_failed",
]);

type StreamPayload = {
  text?: string;
  message?: BrowserMessage;
  messages?: BrowserMessage[];
  message_id?: string;
  code?: string;
  error?: string;
  detail?: string;
};

function turnErrorCode(payload: StreamPayload): string {
  for (const candidate of [payload.code, payload.error]) {
    if (typeof candidate !== "string") continue;
    const code = candidate.trim();
    if (turnErrorCodes.has(code)) return code;
  }
  return "turn_failed";
}
type RetryTurn = {
  attemptStartedAt?: number;
  conversationId: string;
  fluctlightId: string;
  text: string;
  idempotencyKey: string;
  turnId: string;
  attachmentRefs: string[];
  senderActorId?: string;
  senderTimezone?: string;
  senderUtcOffsetMinutes?: number;
  senderSentAt?: string;
  messageId?: string;
};
type QueuedTurn = RetryTurn & {
  messageId: string;
  createdAt: string;
};
export type FluctlightListItem = {
  id: string;
  identity: Record<string, unknown>;
  status: string;
  unread_count?: number;
  last_conversation_at?: string | null;
};

const selectedFluctlightStorageKey = "fluctlight.selected-instance-id";
const retryTurnStorageKey = "fluctlight.retry-turn.v2";
const queuedTurnStorageKey = "fluctlight.queued-turn.v1";
const recoveryStorageMessage='本机原请求记录暂时无法读取。请恢复浏览器存储访问后重新读取；当前输入尚未发送。';
let recoveryStorageFailure='';
function localRecoveryStorage():Storage|undefined{
  try{return globalThis.localStorage;}catch{recoveryStorageFailure=recoveryStorageMessage;return undefined;}
}

function persistedSelection(): string | null {
  try{return globalThis.localStorage?.getItem(selectedFluctlightStorageKey)||null;}catch{return null;}
}

function persistSelection(fluctlightId: string | null): void {
  // The selected role is an optional UI preference, separate from durable turns.
  try{if (fluctlightId)globalThis.localStorage?.setItem(selectedFluctlightStorageKey, fluctlightId);else globalThis.localStorage?.removeItem(selectedFluctlightStorageKey);}catch{}
}

const retryArchiveKey = "fluctlight.retry-turns.v3";
const queuedArchiveKey = "fluctlight.queued-turns.v2";
const recoveryScope = (conversationId: string, fluctlightId: string) =>
  JSON.stringify([fluctlightId, conversationId]);
function validRecovery(value: unknown, queued = false): value is QueuedTurn {
  const v = value as Partial<QueuedTurn> | null;
  return (
    !!v &&
    typeof v.conversationId === "string" &&
    typeof v.fluctlightId === "string" &&
    typeof v.text === "string" &&
    typeof v.idempotencyKey === "string" &&
    typeof v.turnId === "string" &&
    Array.isArray(v.attachmentRefs) &&
    v.attachmentRefs.every((item) => typeof item === "string") &&
    (!queued ||
      (typeof v.messageId === "string" && typeof v.createdAt === "string"))
  );
}
function recoveryRecords(
  archiveKey: string,
  legacyKey: string,
  queued = false,
): Record<string, RetryTurn> {
  const records: Record<string, RetryTurn> = {};
  const storage=localRecoveryStorage();if(!storage)return records;
  let archiveRaw,legacyRaw;
  try{archiveRaw=storage.getItem(archiveKey);legacyRaw=storage.getItem(legacyKey);}catch{recoveryStorageFailure=recoveryStorageMessage;return records;}
  try {
    const stored = JSON.parse(archiveRaw || "{}");
    for (const value of Object.values(stored))
      if (validRecovery(value, queued))
        records[recoveryScope(value.conversationId, value.fluctlightId)] =
          value;
  } catch {
    /* Only valid local identities can be resumed. */
  }
  try {
    const legacy = JSON.parse(legacyRaw || "null");
    if (validRecovery(legacy, queued))
      records[recoveryScope(legacy.conversationId, legacy.fluctlightId)] ??=
        legacy;
  } catch {
    /* An unreadable legacy slot cannot block another conversation. */
  }
  return records;
}
function readRecovery(
  archive: string,
  legacy: string,
  queued: boolean,
  conversationId?: string,
  fluctlightId?: string,
): RetryTurn | null {
  const records = recoveryRecords(archive, legacy, queued);
  if (conversationId && fluctlightId)
    return records[recoveryScope(conversationId, fluctlightId)] || null;
  return Object.values(records).at(-1) || null;
}
function writeRecovery(
  archive: string,
  legacy: string,
  queued: boolean,
  value: RetryTurn | null,
  conversationId?: string | null,
  fluctlightId?: string | null,
): void {
  const storage=localRecoveryStorage();if(!storage){if(recoveryStorageFailure)throw Error(recoveryStorageMessage);return;}
  const records = recoveryRecords(archive, legacy, queued);
  if(recoveryStorageFailure)throw Error(recoveryStorageMessage);
  if (value)
    records[recoveryScope(value.conversationId, value.fluctlightId)] = value;
  else if (conversationId && fluctlightId)
    delete records[recoveryScope(conversationId, fluctlightId)];
  else {
    storage.removeItem(archive);
    storage.removeItem(legacy);
    return;
  }
  if (Object.keys(records).length)
    storage.setItem(archive, JSON.stringify(records));
  else storage.removeItem(archive);
  const current = value || Object.values(records).at(-1);
  if (current) storage.setItem(legacy, JSON.stringify(current));
  else storage.removeItem(legacy);
}
function persistedRetry(
  conversationId?: string,
  fluctlightId?: string,
): RetryTurn | null {
  return readRecovery(
    retryArchiveKey,
    retryTurnStorageKey,
    false,
    conversationId,
    fluctlightId,
  );
}
function persistRetry(
  value: RetryTurn | null,
  conversationId?: string | null,
  fluctlightId?: string | null,
): void {
  writeRecovery(
    retryArchiveKey,
    retryTurnStorageKey,
    false,
    value,
    conversationId,
    fluctlightId,
  );
}
function persistedQueuedTurn(
  conversationId?: string,
  fluctlightId?: string,
): QueuedTurn | null {
  return readRecovery(
    queuedArchiveKey,
    queuedTurnStorageKey,
    true,
    conversationId,
    fluctlightId,
  ) as QueuedTurn | null;
}
function persistQueuedTurn(
  value: QueuedTurn | null,
  conversationId?: string | null,
  fluctlightId?: string | null,
): void {
  writeRecovery(
    queuedArchiveKey,
    queuedTurnStorageKey,
    true,
    value,
    conversationId,
    fluctlightId,
  );
}

function captureSenderTime(): Pick<
  RetryTurn,
  "senderTimezone" | "senderUtcOffsetMinutes" | "senderSentAt"
> {
  const sentAt = new Date();
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  if (!zone) return {};
  return {
    senderTimezone: zone,
    senderUtcOffsetMinutes: -sentAt.getTimezoneOffset(),
    senderSentAt: sentAt.toISOString(),
  };
}

function createLocalMessage(
  conversationId: string,
  text: string,
  sequence: number,
  authorActorId = "human",
  messageId?: string,
  createdAt?: string,
  senderTime?: Partial<
    Pick<
      RetryTurn,
      | "senderTimezone"
      | "senderUtcOffsetMinutes"
      | "senderSentAt"
      | "attachmentRefs"
      | "turnId"
      | "idempotencyKey"
    >
  >,
): BrowserMessage {
  return {
    id: messageId ?? `local-${randomId()}`,
    conversationId,
    sequence,
    authorActorId,
    kind: "user",
    text,
    attachmentRefs: senderTime?.attachmentRefs ?? [],
    createdAt: createdAt ?? new Date().toISOString(),
    senderTimezone: senderTime?.senderTimezone,
    senderUtcOffsetMinutes: senderTime?.senderUtcOffsetMinutes,
    senderSentAt: senderTime?.senderSentAt,
    turnId: senderTime?.turnId,
    idempotencyKey: senderTime?.idempotencyKey,
  };
}

function lastSequence(messages: BrowserMessage[]): number {
  return messages.reduce(
    (highest, message) =>
      isTransientMessage(message)
        ? highest
        : Math.max(highest, message.sequence),
    0,
  );
}

function isTransientMessage(message: BrowserMessage): boolean {
  return message.id.startsWith("local-") || message.id.startsWith("stream-");
}

function mergeConversationMessages(
  authoritative: BrowserMessage[],
  current: BrowserMessage[],
  preserveStreamText = false,
): BrowserMessage[] {
  const merged = [...current];
  for (const message of authoritative) {
    const duplicateIndex = merged.findIndex(
      (candidate) =>
        candidate.id === message.id ||
        (!isTransientMessage(candidate) &&
          candidate.conversationId === message.conversationId &&
          candidate.sequence === message.sequence &&
          candidate.kind === message.kind),
    );
    if (duplicateIndex >= 0) {
      const previous = merged[duplicateIndex];
      merged.splice(
        duplicateIndex,
        1,
        preserveStreamText &&
          message.kind === "assistant" &&
          previous.id === message.id
          ? { ...message, text: previous.text }
          : message,
      );
    } else merged.push(message);
  }
  return merged.sort((left, right) => {
    const leftTransient = isTransientMessage(left);
    const rightTransient = isTransientMessage(right);
    if (leftTransient !== rightTransient) return leftTransient ? 1 : -1;
    return (
      left.sequence - right.sequence ||
      (left.createdAt ?? "").localeCompare(right.createdAt ?? "")
    );
  });
}

export const conversationDefinition = defineDomain({
  state: () => ({
    conversation: null as BrowserConversation | null,
    conversationParticipants: [] as BrowserParticipant[],
    fluctlightId: null as string | null,
    fluctlights: [] as FluctlightListItem[],
    messages: [] as BrowserMessage[],
    nextBeforeSequence: null as number | null,
    authenticated: null as boolean | null,
    authenticatedActorId: null as string | null,
    setupAvailable: false,
    authLoading: false,
    authError: "" as string,
    loading: false,
    historyLoading: false,
    sending: false,
    activeTurnId: null as string | null,
    attemptStartedAt: null as number | null,
    error: "" as string,
    directoryError: "" as string,
    attachmentRef: "",
    abortController: null as AbortController | null,
    retryTurn: persistedRetry(),
    requestEpoch: 0,
    retrying: false,
    queuedTurn: persistedQueuedTurn(),
    recoveryStorageError: recoveryStorageFailure,
    dismissedTurnId: null as string | null,
    senderActorId: null as string | null,
  }),
  getters: {
    hasConversation: (state) => Boolean(state.conversation?.id),
    selectedFluctlight: (state) =>
      state.fluctlights.find(
        (fluctlight) => fluctlight.id === state.fluctlightId,
      ) ?? null,
    selectedFluctlightName(): string | null {
      const name = this.selectedFluctlight?.identity.name;
      return typeof name === "string" && name.trim()
        ? name
        : (this.selectedFluctlight?.id ?? null);
    },
    canRetry: (state) =>
      Boolean(state.retryTurn) &&
      state.retryTurn?.conversationId === state.conversation?.id &&
      state.retryTurn?.fluctlightId === state.fluctlightId &&
      !state.sending &&
      !state.retrying &&
      !state.messages.some(
        (message) =>
          message.kind === "user" &&
          ["pending", "running"].includes(message.turnStatus ?? ""),
      ) &&
      !state.messages.some(
        (message) =>
          message.kind === "user" &&
          message.id === state.retryTurn?.messageId &&
          message.turnRetryable === false,
      ),
    retryIsModelFailure: (state) =>
      Boolean(
        state.retryTurn &&
        state.messages.some(
          (message) =>
            message.kind === "user" &&
            message.id === state.retryTurn?.messageId &&
            (message.turnStatus === "failed" ||
              message.turnStatus === "cancelled"),
        ),
      ),
    hasPendingTurn: (state) =>
      state.messages.some(
        (message) =>
          message.kind === "user" &&
          (message.turnStatus === "pending" ||
            message.turnStatus === "running"),
      ),
    hasSettlingTurn: (state) =>
      state.messages.some(
        (message) =>
          message.kind === "user" &&
          (message.turnStatus === "failed" ||
            message.turnStatus === "cancelled") &&
          message.turnRetryable === false &&
          message.turnErrorCode !== "agent_partial_committed",
      ),
    canCancel(): boolean {
      return this.hasPendingTurn;
    },
    queuedMessageId: (state) =>
      state.queuedTurn?.conversationId === state.conversation?.id &&
      state.queuedTurn?.fluctlightId === state.fluctlightId
        ? state.queuedTurn.messageId
        : null,
  },
  actions: {
    rereadLocalRecovery(){
      recoveryStorageFailure='';
      const retry=this.conversation&&this.fluctlightId?persistedRetry(this.conversation.id,this.fluctlightId):persistedRetry();
      const queued=this.conversation&&this.fluctlightId?persistedQueuedTurn(this.conversation.id,this.fluctlightId):persistedQueuedTurn();
      this.recoveryStorageError=recoveryStorageFailure;
      if(!recoveryStorageFailure){if(retry)this.retryTurn=retry;if(queued)this.queuedTurn=queued;if(this.error===recoveryStorageMessage)this.error='';}
      return !recoveryStorageFailure;
    },
    async initialize() {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.session();
        this.authenticated = session.authenticated;
        this.authenticatedActorId = session.authenticated
          ? (session.actorId ?? null)
          : null;
        if (this.authenticated) await this.bootstrap();
        else this.setupAvailable = (await client.setupStatus()).setupAvailable;
      } catch {
        if (this.authenticated !== true) {
          this.authenticated = false;
          this.authenticatedActorId = null;
          this.authError = "Fluctlight 服务暂时不可用。";
        }
      } finally {
        this.authLoading = false;
      }
    },
    async login(password: string) {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.login(password);
        this.authenticated = session.authenticated;
        this.authenticatedActorId = session.authenticated
          ? (session.actorId ?? null)
          : null;
        if (this.authenticated) await this.bootstrap();
      } catch {
        if (this.authenticated !== true) {
          this.authenticated = false;
          this.authenticatedActorId = null;
          this.authError = "密码未被接受。";
        } else {
          this.error = "无法加载 Fluctlight 对话。";
        }
      } finally {
        this.authLoading = false;
      }
    },
    async setup(setupToken: string, password: string) {
      this.authLoading = true;
      this.authError = "";
      try {
        const session = await client.setup(setupToken, password);
        this.authenticated = session.authenticated;
        this.authenticatedActorId = session.authenticated
          ? (session.actorId ?? null)
          : null;
        this.setupAvailable = false;
        if (this.authenticated) await this.bootstrap();
      } catch {
        this.authenticated = false;
        this.authenticatedActorId = null;
        this.authError = "设置令牌或密码未被接受。";
      } finally {
        this.authLoading = false;
      }
    },
    async changePassword(password: string) {
      this.invalidateRequest();
      this.authLoading = true;
      this.authError = "";
      try {
        await client.changePassword(password);
        this.authenticated = false;
        this.authenticatedActorId = null;
        this.conversation = null;
        this.messages = [];
        this.fluctlightId = null;
        this.retryTurn = null;
        persistRetry(null);
        this.queuedTurn = null;
        persistQueuedTurn(null);
        return true;
      } catch {
        this.authError = "无法修改所有者密码。请确认当前登录会话仍有效后重试。";
        return false;
      } finally {
        this.authLoading = false;
      }
    },
    async logout() {
      this.invalidateRequest();
      await client.logout();
      {
        this.authenticated = false;
        this.authenticatedActorId = null;
        this.conversation = null;
        this.fluctlightId = null;
        this.fluctlights = [];
        this.messages = [];
        this.nextBeforeSequence = null;
        this.retryTurn = null;
        persistRetry(null);
        this.queuedTurn = null;
        persistQueuedTurn(null);
      }
    },
    async bootstrap() {
      if (this.authenticated === false) return;
      this.loading = true;
      this.error = "";
      this.directoryError = "";
      try {
        this.fluctlights = await client.listFluctlights();
        this.pruneOrphanedLocalTurns();
        await this.reconcilePersistedRetry();
        const restoredId = persistedSelection();
        const selectedId = this.fluctlights.some(
          (item) => item.id === restoredId,
        )
          ? restoredId
          : (this.fluctlights[0]?.id ?? null);
        if (selectedId)
          await this.selectFluctlight(selectedId, { loading: false });
        else this.clearActiveConversation();
      } catch {
        this.error = "无法加载 Fluctlight 实例目录。";
        this.directoryError = this.error;
      } finally {
        this.loading = false;
      }
    },
    async selectFluctlight(
      fluctlightId: string,
      options: { loading?: boolean; current?: () => boolean } = {},
    ) {
      if (options.current && !options.current()) return;
      if (!this.fluctlights.some((item) => item.id === fluctlightId)) {
        this.error = "所选 Fluctlight 实例不可用。";
        return;
      }
      if (options.loading !== false) this.loading = true;
      if (this.retryTurn) persistRetry(this.retryTurn);
      if (this.queuedTurn) persistQueuedTurn(this.queuedTurn);
      this.invalidateRequest();
      this.error = "";
      const selectionEpoch = this.requestEpoch;
      try {
        const page = await client.directConversation(fluctlightId);
        if (
          selectionEpoch !== this.requestEpoch ||
          (options.current && !options.current())
        )
          return;
        this.retryTurn = persistedRetry(page.conversation.id, fluctlightId);
        this.queuedTurn = persistedQueuedTurn(
          page.conversation.id,
          fluctlightId,
        );
        this.recoveryStorageError=recoveryStorageFailure;
        this.attemptStartedAt = this.retryTurn?.attemptStartedAt ?? null;
        this.conversation = page.conversation;
        this.conversationParticipants = page.participants;
        this.messages = page.messages;
        this.senderActorId = null;
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
        this.fluctlightId = fluctlightId;
        this.syncServerTurnState();
        this.ensureRetryMessageVisible();
        this.ensureQueuedMessageVisible();
        persistSelection(fluctlightId);
        if (
          !this.retryTurn &&
          this.queuedTurn?.conversationId === this.conversation?.id &&
          this.queuedTurn.fluctlightId === this.fluctlightId
        ) {
          await this.sendQueuedTurn();
        }
      } catch {
        if (
          selectionEpoch !== this.requestEpoch ||
          (options.current && !options.current())
        )
          return;
        this.clearActiveConversation();
        this.error = "无法打开该 Fluctlight 的对话。";
      } finally {
        if (selectionEpoch === this.requestEpoch && options.loading !== false)
          this.loading = false;
      }
    },
    clearActiveConversation() {
      this.invalidateRequest();
      this.loading = false;
      this.conversation = null;
      this.conversationParticipants = [];
      this.fluctlightId = null;
      this.messages = [];
      this.senderActorId = null;
      this.nextBeforeSequence = null;
      this.dismissedTurnId = null;
      persistSelection(null);
    },
    syncServerTurnState() {
      const conversationId = this.conversation?.id;
      const fluctlightId = this.fluctlightId;
      if (!conversationId || !fluctlightId) return;
      const latest = [...this.messages]
        .reverse()
        .find(
          (message) => message.kind === "user" && !isTransientMessage(message),
        );
      if (!latest?.turnStatus) return;
      if (
        (latest.turnStatus === "failed" || latest.turnStatus === "cancelled") &&
        latest.turnId &&
        latest.idempotencyKey
      ) {
        if (this.dismissedTurnId && this.dismissedTurnId === latest.turnId) {
          return;
        }
        const retry: RetryTurn = {
          attemptStartedAt:
            this.retryTurn?.turnId === latest.turnId
              ? this.retryTurn.attemptStartedAt
              : undefined,
          conversationId,
          fluctlightId,
          text: latest.text,
          turnId: latest.turnId,
          idempotencyKey: latest.idempotencyKey,
          attachmentRefs: latest.attachmentRefs,
          senderTimezone: latest.senderTimezone ?? undefined,
          senderUtcOffsetMinutes: latest.senderUtcOffsetMinutes ?? undefined,
          senderSentAt: latest.senderSentAt ?? undefined,
          messageId: latest.id,
          senderActorId:
            latest.authorActorId === this.conversation?.createdByActorId
              ? undefined
              : latest.authorActorId,
        };
        this.retryTurn = retry;
        persistRetry(retry);
        this.error =
          latest.turnErrorCode === "agent_partial_committed"
            ? "本次生成已执行部分操作，不能直接重试，以免重复执行。详情请查看诊断：agent_partial_committed"
            : latest.turnRetryable === false
              ? "正在结束本次生成，完成后可以重试。"
              : latest.turnStatus === "cancelled"
                ? "回复已取消，可以重试。"
                : `回复未完成：${latest.turnErrorCode ?? "模型处理失败"}`;
        return;
      }
      if (["pending", "running"].includes(latest.turnStatus)) {
        this.error = "";
        return;
      }
      if (
        this.retryTurn?.conversationId === conversationId &&
        this.retryTurn?.fluctlightId === fluctlightId
      ) {
        this.retryTurn = null;
        persistRetry(null, this.conversation?.id, this.fluctlightId);
      }
      this.error = "";
    },
    async refreshActiveTurn(explicit = false, targetTurnId?: string) {
      const conversationId = this.conversation?.id;
      const fluctlightId = this.fluctlightId,
        epoch = this.requestEpoch,
        turnId = this.activeTurnId;
      if (
        !conversationId ||
        (!explicit &&
          ((!this.hasPendingTurn && !this.hasSettlingTurn) || this.sending))
      )
        return;
      try {
        const page = await client.messages(conversationId);
        if (
          this.conversation?.id !== conversationId ||
          this.fluctlightId !== fluctlightId ||
          this.requestEpoch !== epoch
        )
          return;
        if (this.sending) {
          const confirmed = page.messages.find(
            (message) => message.kind === "user" && message.turnId === turnId,
          );
          // An explicit read may finish a stuck observer only after the server
          // confirms a terminal turn. Pending reads preserve incremental text.
          if (
            !confirmed ||
            !["completed", "failed", "cancelled"].includes(
              confirmed.turnStatus ?? "",
            )
          )
            return confirmed?.turnStatus;
          this.invalidateRequest();
          this.messages = mergeConversationMessages(
            page.messages,
            this.messages.filter(
              (message) =>
                !message.id.startsWith("stream-") &&
                !(message.id.startsWith("local-") && message.turnId === turnId),
            ),
          );
        } else
          this.messages = mergeConversationMessages(
            page.messages,
            this.messages,
          );
        this.syncServerTurnState();
        const status = page.messages.findLast(
          (message) => message.kind === "user",
        )?.turnStatus;
        if (
          !targetTurnId &&
          status === "completed" &&
          this.queuedTurn &&
          !this.sending &&
          !this.hasPendingTurn &&
          !this.hasSettlingTurn &&
          !this.retryTurn &&
          !this.retrying
        )
          await this.sendQueuedTurn();
        return targetTurnId?page.messages.findLast(message=>message.kind==='user'&&message.turnId===targetTurnId)?.turnStatus:status;
      } catch (error) {
        // A read failure cannot change an accepted turn's server state.
        if (explicit) throw error;
      }
    },
    async send(
      text: string,
      retry = false,
      queuedRequest: QueuedTurn | null = null,
      onAccepted?: () => void,
    ): Promise<boolean> {
      const normalized = text.trim();
      if (!normalized) return false;
      if(!this.rereadLocalRecovery()){this.error=recoveryStorageMessage;return false;}
      if (
        !retry &&
        !queuedRequest &&
        this.queuedTurn?.conversationId === this.conversation?.id &&
        this.queuedTurn?.fluctlightId === this.fluctlightId
      ) {
        this.error ||=
          "已有一条追发消息保留，请先处理原回应，当前输入尚未发送。";
        return false;
      }
      if (this.sending) {
        if (
          retry ||
          this.queuedTurn?.conversationId === this.conversation?.id ||
          !this.conversation?.id ||
          !this.fluctlightId
        )
          return false;
        const senderTime = captureSenderTime();
        const local = createLocalMessage(
          this.conversation.id,
          normalized,
          0,
          this.senderActorId ?? "human",
          undefined,
          senderTime.senderSentAt,
          senderTime,
        );
        const queued: QueuedTurn = {
          conversationId: this.conversation.id,
          fluctlightId: this.fluctlightId,
          text: normalized,
          idempotencyKey: `turn-${randomId()}`,
          turnId: `turn_${randomId()}`,
          attachmentRefs: [],
          senderActorId: this.senderActorId ?? undefined,
          ...senderTime,
          messageId: local.id,
          createdAt: local.createdAt,
        };
        try{persistQueuedTurn(queued);}catch{this.recoveryStorageError=recoveryStorageMessage;this.error=recoveryStorageMessage;return false;}
        this.queuedTurn = queued;
        this.messages.push({
          ...local,
          turnId: queued.turnId,
          idempotencyKey: queued.idempotencyKey,
        });
        onAccepted?.();
        return true;
      }
      if (!retry && this.hasPendingTurn) {
        if (!this.sending) {
          await this.cancel();
        }
        if (this.hasPendingTurn) {
          this.error = "上一条消息仍在后台处理；可点击取消后再发送。";
          return false;
        }
      }
      if (
        this.retryTurn &&
        (this.retryTurn.conversationId !== this.conversation?.id ||
          this.retryTurn.fluctlightId !== this.fluctlightId)
      ) {
        persistRetry(this.retryTurn);
        this.retryTurn =
          this.conversation && this.fluctlightId
            ? persistedRetry(this.conversation.id, this.fluctlightId)
            : null;
      }
      const conversationId = this.conversation?.id;
      const fluctlightId = this.fluctlightId;
      const pendingRetry =
        this.retryTurn?.conversationId === conversationId &&
        this.retryTurn?.fluctlightId === fluctlightId
          ? this.retryTurn
          : null;
      if (!conversationId || !fluctlightId) return false;
      if (retry && !pendingRetry) return false;
      if (
        queuedRequest &&
        (queuedRequest.conversationId !== conversationId ||
          queuedRequest.fluctlightId !== fluctlightId)
      )
        return false;
      // A new message is a new command. Only retry() can replay an older failed turn.
      if (
        pendingRetry &&
        !retry &&
        !this.messages.some(
          (message) =>
            message.kind === "user" &&
            (message.turnId === pendingRetry.turnId ||
              message.id === pendingRetry.messageId) &&
            ["failed", "cancelled", "completed"].includes(
              message.turnStatus ?? "",
            ),
        )
      ) {
        this.error =
          "上一条消息的发送状态尚未确认，请先查询或重新发送原消息。当前输入已保留。";
        return false;
      }
      if (pendingRetry && !retry) this.dismissedTurnId = pendingRetry.turnId;
      this.error = "";
      this.sending = true;
      this.abortController = untracked(new AbortController());
      const requestEpoch = this.requestEpoch;
      const request: RetryTurn =
        pendingRetry && retry
          ? pendingRetry
          : queuedRequest
            ? { ...queuedRequest }
            : {
                conversationId,
                fluctlightId,
                text: normalized,
                idempotencyKey: `turn-${randomId()}`,
                turnId: `turn_${randomId()}`,
                attachmentRefs: this.attachmentRef ? [this.attachmentRef] : [],
                senderActorId: this.senderActorId ?? undefined,
                ...captureSenderTime(),
              };
      this.activeTurnId = request.turnId;
      request.attemptStartedAt = Date.now();
      this.attemptStartedAt = request.attemptStartedAt;
      try{persistRetry(request);}catch{this.recoveryStorageError=recoveryStorageMessage;this.error=recoveryStorageMessage;this.sending=false;this.abortController=null;this.activeTurnId=null;this.attemptStartedAt=null;return false;}
      this.retryTurn = request;
      let optimisticMessageId = request.messageId ?? null;
      if (!retry) {
        const localAlreadyVisible =
          optimisticMessageId &&
          this.messages.some((message) => message.id === optimisticMessageId);
        if (!localAlreadyVisible) {
          const userMessage = createLocalMessage(
            conversationId,
            normalized,
            0,
            request.senderActorId ?? "human",
            optimisticMessageId ?? undefined,
            request.senderSentAt ?? queuedRequest?.createdAt,
            request,
          );
          optimisticMessageId = userMessage.id;
          request.messageId = userMessage.id;
          this.messages.push(userMessage);
        }
      }
      let assistantDraft: BrowserMessage | null = null;
      try {
        const response = await client.turn(
          request.conversationId,
          {
            text: request.text,
            fluctlightId: request.fluctlightId,
            senderActorId: request.senderActorId,
            attachmentRefs: request.attachmentRefs,
            idempotencyKey: request.idempotencyKey,
            turnId: request.turnId,
            senderTimezone: request.senderTimezone,
            senderUtcOffsetMinutes: request.senderUtcOffsetMinutes,
            senderSentAt: request.senderSentAt,
          },
          this.abortController.signal,
        );
        if (!response.body) throw new Error("stream_missing");
        onAccepted?.();
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let assistantText = "";
        let expectedSequence = 0;
        let terminalType: "completed" | "error" | null = null;
        const applyEvent = (event: BrowserTurnEvent) => {
          if (this.requestEpoch !== requestEpoch) return false;
          if (event.turnId !== request.turnId)
            throw new Error("turn_id_mismatch");
          if (event.sequence !== expectedSequence)
            throw new Error("turn_sequence_invalid");
          expectedSequence += 1;
          const payload = event.payload as StreamPayload;
          if (event.type === "token") {
            assistantText += payload.text ?? "";
            if (!assistantDraft) {
              const streamedMessage: BrowserMessage = {
                id: `stream-${randomId()}`,
                conversationId: request.conversationId,
                sequence: 0,
                authorActorId: request.fluctlightId,
                kind: "assistant",
                text: assistantText,
                attachmentRefs: [],
                createdAt: new Date().toISOString(),
              };
              assistantDraft = streamedMessage;
              this.messages.push(streamedMessage);
            } else {
              // `assistantDraft` is the raw object originally pushed into a
              // observable array. Replace the entry rather than mutating
              // the original raw reference, so every delta is observable.
              const draftIndex = this.messages.findIndex(
                (message) => message.id === assistantDraft?.id,
              );
              if (draftIndex >= 0) {
                this.messages.splice(draftIndex, 1, {
                  ...this.messages[draftIndex],
                  text: assistantText,
                });
              }
            }
          }
          if (
            (event.type === "message" || event.type === "media") &&
            (payload.message || payload.messages?.length)
          ) {
            const incoming = payload.messages?.length
              ? payload.messages
              : payload.message
                ? [payload.message]
                : [];
            for (const message of incoming) {
              const optimisticIndex =
                message.kind === "user" && optimisticMessageId
                  ? this.messages.findIndex(
                      (candidate) => candidate.id === optimisticMessageId,
                    )
                  : -1;
              if (message.kind === "assistant" && assistantDraft) {
                const draftIndex = this.messages.findIndex(
                  (candidate) => candidate.id === assistantDraft?.id,
                );
                if (draftIndex >= 0) {
                  this.messages.splice(draftIndex, 1, message);
                  assistantDraft = null;
                  continue;
                }
              }
              const persistedIndex = this.messages.findIndex(
                (candidate) =>
                  candidate.id === message.id ||
                  (!isTransientMessage(candidate) &&
                    candidate.conversationId === message.conversationId &&
                    candidate.sequence === message.sequence &&
                    candidate.kind === message.kind),
              );
              if (persistedIndex >= 0)
                this.messages.splice(persistedIndex, 1, message);
              else if (optimisticIndex >= 0) {
                this.messages.splice(optimisticIndex, 1, message);
                request.messageId = message.id;
                optimisticMessageId = message.id;
              } else this.messages.push(message);
            }
          }
          if (event.type === "error") {
            const code = turnErrorCode(payload);
            const detail =
              typeof payload.detail === "string" &&
              turnErrorCodes.has(payload.detail.trim())
                ? payload.detail.trim()
                : "";
            terminalType = "error";
            throw new Error(detail ? `${code}: ${detail}` : code);
          }
          if (event.type === "completed") terminalType = "completed";
        };
        while (true) {
          const next = await reader.read();
          if (this.requestEpoch !== requestEpoch) return false;
          if (next.done) break;
          buffer += decoder.decode(next.value, { stream: true });
          const lines = buffer.split("\n");
          buffer = lines.pop() ?? "";
          for (const line of lines) {
            if (!line.trim()) continue;
            applyEvent(JSON.parse(line) as BrowserTurnEvent);
          }
        }
        if (this.requestEpoch !== requestEpoch) return false;
        buffer += decoder.decode();
        if (buffer.trim()) {
          applyEvent(JSON.parse(buffer) as BrowserTurnEvent);
        }
        if (terminalType !== "completed")
          throw new Error("turn_stream_incomplete");
        const page = await client.messages(request.conversationId);
        if (
          this.requestEpoch !== requestEpoch ||
          this.conversation?.id !== request.conversationId ||
          this.fluctlightId !== request.fluctlightId
        )
          return false;
        this.conversation = page.conversation;
        this.messages = mergeConversationMessages(
          page.messages,
          this.messages,
          true,
        );
        this.syncServerTurnState();
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
        this.attachmentRef = "";
        if (
          this.retryTurn?.idempotencyKey === request.idempotencyKey ||
          retry
        ) {
          this.retryTurn = null;
          persistRetry(null, this.conversation?.id, this.fluctlightId);
        }
        this.dismissedTurnId = null;
      } catch (error) {
        if (this.requestEpoch !== requestEpoch) return false;
        if (assistantDraft)
          this.messages = this.messages.filter(
            (message) => message.id !== assistantDraft?.id,
          );
        try {
          const page = await client.messages(request.conversationId);
          if (
            this.requestEpoch !== requestEpoch ||
            this.conversation?.id !== request.conversationId
          )
            return false;
          this.messages = mergeConversationMessages(
            page.messages,
            this.messages,
          );
          const confirmed = page.messages.find(
            (message) =>
              message.kind === "user" && message.turnId === request.turnId,
          );
          if (confirmed) {
            this.messages = this.messages.filter(
              (message) =>
                message.id !== optimisticMessageId ||
                message.id === confirmed.id,
            );
            this.syncServerTurnState();
            if (
              error instanceof BrowserApiError &&
              (confirmed.turnStatus === "failed" ||
                confirmed.turnStatus === "cancelled")
            ) {
              this.error = `${retry ? "重试" : "发送"}请求未被接受：${turnErrorCode({ code: error.code })}。${this.error}`;
            }
            return false;
          }
        } catch {
          // Keep the submitted identity until a later authoritative read.
        }
        if (
          !retry &&
          error instanceof BrowserApiError &&
          ([400, 401, 403, 404].includes(error.status) ||
            error.code === "conversation_turn_conflict")
        ) {
          this.messages = this.messages.filter(
            (message) => message.id !== optimisticMessageId,
          );
          this.retryTurn = null;
          persistRetry(null, this.conversation?.id, this.fluctlightId);
          this.error = `消息未被接受：${turnErrorCode({ code: error.code })}`;
          return false;
        }
        request.messageId =
          request.messageId ?? optimisticMessageId ?? undefined;
        this.retryTurn = request;
        persistRetry(request);
        this.error = "发送状态未确认；重新发送会沿用同一消息身份。";
      } finally {
        if (this.requestEpoch === requestEpoch) {
          this.abortController = null;
          this.sending = false;
          this.activeTurnId = null;
          if (
            !retry &&
            !this.retrying &&
            this.queuedTurn &&
            !this.hasPendingTurn &&
            !this.hasSettlingTurn &&
            !this.retryTurn
          )
            await this.sendQueuedTurn();
        }
      }
      return true;
    },
    invalidateRequest() {
      this.requestEpoch += 1;
      this.abortController?.abort();
      this.abortController = null;
      this.sending = false;
      this.activeTurnId = null;
      this.retrying = false;
    },
    pruneOrphanedLocalTurns() {
      const available = new Set(this.fluctlights.map((item) => item.id));
      for (const recovery of Object.values(
        recoveryRecords(retryArchiveKey, retryTurnStorageKey),
      )) {
        if (!available.has(recovery.fluctlightId))
          persistRetry(null, recovery.conversationId, recovery.fluctlightId);
      }
      for (const recovery of Object.values(
        recoveryRecords(queuedArchiveKey, queuedTurnStorageKey, true),
      )) {
        if (!available.has(recovery.fluctlightId))
          persistQueuedTurn(
            null,
            recovery.conversationId,
            recovery.fluctlightId,
          );
      }
      if (this.retryTurn && !available.has(this.retryTurn.fluctlightId)) {
        this.retryTurn = null;
      }
      if (this.queuedTurn && !available.has(this.queuedTurn.fluctlightId)) {
        this.queuedTurn = null;
      }
    },
    async reconcilePersistedRetry() {
      const retry = this.retryTurn;
      if (
        !retry ||
        !this.fluctlights.some((item) => item.id === retry.fluctlightId)
      )
        return;
      try {
        const page = await client.directConversation(retry.fluctlightId);
        if (page.conversation.id !== retry.conversationId) {
          this.retryTurn = null;
          persistRetry(null, retry.conversationId, retry.fluctlightId);
          return;
        }
        let historyPage = page;
        const allMessages = [...page.messages];
        let userMessage: BrowserMessage | undefined;
        while (true) {
          userMessage = historyPage.messages.find(
            (message) =>
              message.kind === "user" &&
              (message.turnId === retry.turnId ||
                message.id === retry.messageId),
          );
          if (userMessage || !historyPage.nextBeforeSequence) break;
          historyPage = await client.messages(
            retry.conversationId,
            historyPage.nextBeforeSequence,
            200,
          );
          allMessages.push(...historyPage.messages);
        }
        if (!userMessage) return;
        if (retry.messageId !== userMessage.id) {
          retry.messageId = userMessage.id;
          persistRetry(retry);
        }
        const completed =
          userMessage.turnStatus === "completed" ||
          allMessages.some(
            (message) =>
              message.kind === "assistant" && message.turnId === retry.turnId,
          );
        if (completed) {
          this.retryTurn = null;
          persistRetry(null, retry.conversationId, retry.fluctlightId);
        }
      } catch (error) {
        if (error instanceof BrowserApiError && error.status === 404) {
          this.retryTurn = null;
          persistRetry(null, retry.conversationId, retry.fluctlightId);
          return;
        }
        // Preserve the retry on transient read failures; blocking is safer than
        // dropping a turn whose authoritative state could not be checked.
      }
    },
    async cancel() {
      const conversationId = this.conversation?.id;
      const pending = [...this.messages]
        .reverse()
        .find(
          (message) =>
            message.kind === "user" &&
            (message.turnStatus === "pending" ||
              message.turnStatus === "running") &&
            message.turnId,
        );
      if (!conversationId || !pending?.turnId) {
        this.error = "还未读到可取消的消息标识，请刷新进度确认受理结果。";
        return;
      }
      try {
        await client.cancelTurn(conversationId, pending.turnId);
        this.abortController?.abort();
        const page = await client.messages(conversationId);
        if (this.conversation?.id !== conversationId) return;
        this.messages = mergeConversationMessages(page.messages, this.messages);
        this.syncServerTurnState();
      } catch {
        this.error = "取消请求结果尚未确认，请刷新进度核对。原消息已保留。";
      }
    },
    async retryMessage(messageId: string) {
      if (
        this.sending ||
        this.retrying ||
        this.hasPendingTurn ||
        this.hasSettlingTurn ||
        !this.conversation ||
        !this.fluctlightId
      )
        return;
      const message = this.messages.find(
        (message) =>
          message.id === messageId &&
          message.kind === "user" &&
          message.turnRetryable === true &&
          ["failed", "cancelled"].includes(message.turnStatus ?? ""),
      );
      if (!message?.turnId || !message.idempotencyKey) return;
      if (
        this.retryTurn &&
        this.retryTurn.turnId !== message.turnId &&
        !this.messages.some(
          (item) =>
            item.kind === "user" &&
            item.turnId === this.retryTurn?.turnId &&
            ["failed", "cancelled", "completed"].includes(
              item.turnStatus ?? "",
            ),
        )
      )
        return;
      this.retryTurn = {
        conversationId: this.conversation.id,
        fluctlightId: this.fluctlightId,
        text: message.text,
        turnId: message.turnId,
        idempotencyKey: message.idempotencyKey,
        attachmentRefs: message.attachmentRefs,
        messageId: message.id,
        senderActorId:
          message.authorActorId === this.conversation.createdByActorId
            ? undefined
            : message.authorActorId,
        senderTimezone: message.senderTimezone ?? undefined,
        senderUtcOffsetMinutes: message.senderUtcOffsetMinutes ?? undefined,
        senderSentAt: message.senderSentAt ?? undefined,
      };
      persistRetry(this.retryTurn);
      await this.retry();
    },
    async retry() {
      const pending = this.retryTurn;
      if (
        !pending ||
        pending.conversationId !== this.conversation?.id ||
        pending.fluctlightId !== this.fluctlightId ||
        !this.canRetry
      )
        return;
      this.retrying = true;
      try {
        await this.send(pending.text, true);
        if (
          !this.retryTurn &&
          this.queuedTurn?.conversationId === this.conversation?.id &&
          this.queuedTurn.fluctlightId === this.fluctlightId
        ) {
          await this.sendQueuedTurn();
        }
      } finally {
        this.retrying = false;
      }
    },
    dismissRetry() {
      const queued = this.queuedTurn;
      this.dismissedTurnId = this.retryTurn?.turnId ?? null;
      this.retryTurn = null;
      this.retrying = false;
      this.error = "";
      persistRetry(null, this.conversation?.id, this.fluctlightId);
      if (
        queued &&
        queued.conversationId === this.conversation?.id &&
        queued.fluctlightId === this.fluctlightId
      )
        void this.sendQueuedTurn();
    },
    async sendQueuedTurn() {
      const queued = this.queuedTurn;
      if (
        !queued ||
        queued.conversationId !== this.conversation?.id ||
        queued.fluctlightId !== this.fluctlightId ||
        this.sending
      )
        return;
      this.queuedTurn = null;
      persistQueuedTurn(null, this.conversation?.id, this.fluctlightId);
      await this.send(queued.text, false, queued);
    },
    ensureRetryMessageVisible() {
      const pending = this.retryTurn;
      const conversationId = this.conversation?.id;
      if (
        !pending ||
        !conversationId ||
        pending.conversationId !== conversationId ||
        pending.fluctlightId !== this.fluctlightId
      )
        return;
      const matched = this.messages.find(
        (message) =>
          message.kind === "user" &&
          message.conversationId === conversationId &&
          (message.turnId === pending.turnId ||
            message.id === pending.messageId),
      );

      if (matched) {
        if (pending.messageId !== matched.id) {
          pending.messageId = matched.id;
          persistRetry(pending);
        }
        const completed = this.messages.some(
          (message) =>
            message.kind === "assistant" && message.turnId === pending.turnId,
        );
        if (completed) {
          this.retryTurn = null;
          persistRetry(null, this.conversation?.id, this.fluctlightId);
          return;
        }
        return;
      }
      const local = createLocalMessage(
        conversationId,
        pending.text,
        0,
        pending.senderActorId ?? "human",
        pending.messageId,
        pending.senderSentAt,
        pending,
      );
      pending.messageId = local.id;
      persistRetry(pending);
      this.messages.push(local);
    },
    ensureQueuedMessageVisible() {
      const queued = this.queuedTurn;
      const conversationId = this.conversation?.id;
      if (
        !queued ||
        !conversationId ||
        queued.conversationId !== conversationId ||
        queued.fluctlightId !== this.fluctlightId
      )
        return;
      const exists = this.messages.some(
        (message) =>
          message.id === queued.messageId ||
          (message.kind === "user" && message.turnId === queued.turnId),
      );
      if (!exists) {
        this.messages.push(
          createLocalMessage(
            conversationId,
            queued.text,
            0,
            queued.senderActorId ?? "human",
            queued.messageId,
            queued.createdAt,
            queued,
          ),
        );
      }
    },
    async loadOlder() {
      if (
        !this.conversation ||
        !this.nextBeforeSequence ||
        this.loading ||
        this.historyLoading
      )
        return;
      const conversationId = this.conversation.id;
      this.historyLoading = true;
      try {
        const page = await client.messages(
          conversationId,
          this.nextBeforeSequence,
        );
        if (this.conversation?.id !== conversationId) return;
        const existing = new Set(this.messages.map((message) => message.id));
        this.messages = [
          ...page.messages.filter((message) => !existing.has(message.id)),
          ...this.messages,
        ];
        this.nextBeforeSequence = page.nextBeforeSequence ?? null;
      } catch {
        if (this.conversation?.id === conversationId)
          this.error = "无法加载更早的对话记录。";
      } finally {
        this.historyLoading = false;
      }
    },
    async reportReadPosition(visibleSequence?: number) {
      if (!this.conversation) return;
      const sequence = visibleSequence;
      if (
        !Number.isSafeInteger(sequence) ||
        sequence! <= 0 ||
        sequence! > lastSequence(this.messages)
      )
        return;
      if (!sequence) return;
      try {
        await client.markRead(this.conversation.id, {
          readSequence: sequence,
          deliveredSequence: sequence,
        });
      } catch {
        this.error = "无法保存已读位置。";
      }
    },
  },
});

export const createConversationController = () =>
  createDomainStore(conversationDefinition);
export type ConversationController = ReturnType<
  typeof createConversationController
>;
export const productConversation = createConversationController();
const scopes = new WeakMap<object, ConversationController>();
export function bindConversationScope(
  scope: object,
  controller: ConversationController,
) {
  scopes.set(scope, controller);
}
export function conversationForScope(scope: object) {
  let controller = scopes.get(scope);
  if (!controller) {
    controller = createConversationController();
    scopes.set(scope, controller);
  }
  return controller;
}
