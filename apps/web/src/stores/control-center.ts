import { defineStore } from "pinia";
import {
  BrowserClient,
  BrowserApiError,
  type BrowserDiagnosticEvent,
  type BrowserDiagnosticModelRun,
  type BrowserSafeSettings,
} from "@fluctlight/browser-client";
import { bffOrigin } from "../runtime-config";
import { planDefaultGroupMembership } from "../lib/group-membership";

const client = new BrowserClient(bffOrigin);

export const useControlCenterStore = defineStore("control-center", {
  state: () => ({
    diagnostics: [] as BrowserDiagnosticEvent[],
    diagnosticModelRuns: [] as BrowserDiagnosticModelRun[],
    workflows: [] as Array<Record<string, unknown>>,
    workflowId: "",
    workflowStatus: null as Record<string, unknown> | null,
    workflowHistory: null as Record<string, unknown> | null,
    workflowHistoryPoint: "",
    diagnosticsCorrelationFilter: "",
    diagnosticsWarning: "",
    diagnosticsNotice: "",
    diagnosticsLoaded: false,
    diagnosticsLastLoadedAt: "",
    diagnosticsRequestId: 0,
    analysisFailureCorrelationId: "",
    moments: [] as Array<{ id: string; owner_fluctlight_id?: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null; unread_count?: number }>,
    momentsScope: "global" as "global" | "fluctlight",
    includeHiddenMoments: false,
    momentDrafts: {} as Record<string, string>,
    momentNotice: "",
    providerBindings: [] as Array<{ role: string; endpoint_id: string; model_id: string; token_budget: number; timeout_seconds: number; endpoint_status: string }>,
    providerEndpoints: [] as Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; capability_status: string; roles: Array<{ role: string; model_id: string }> }>,
    providerModels: [] as string[],
    providerModelsEndpointId: "",
    providerModelsError: "",
    fluctlightDetail: null as Record<string, unknown> | null,
    governanceReason: "",
    revisionChangesJson: "",
    revisionReason: "",
    rollbackTargetRevision: "",
    governanceEvidence: "",
    memoryEdits: {} as Record<string, string>,
    relationshipRollbackTargets: {} as Record<string, string>,
    autonomyActions: [] as Array<{ id: string; action_type: string; status: string; workflow_id: string; created_at: string }>,
    lifeEvent: { kind: "", startAt: "", endAt: "", scene: "", activity: "", location: "" },
    presence: { currentTask: "", userPresence: "" },
    scheduleDraftJson: "",
    actorGroups: [] as Array<{ id: string; name: string; actor_ids: string[] }>,
    newActorGroupName: "",
    selectedActorGroupId: "",
    autonomySettingsJson: "",
    diagnosticsRetentionJson: "",
    settings: null as BrowserSafeSettings | null,
    loading: false,
    saving: false,
    error: "",
  }),
  actions: {
    async analyzeFluctlight(description: string) {
      this.error = "";
      this.analysisFailureCorrelationId = "";
      try {
        return await client.analyzeFluctlightCreation(description);
      } catch (error) {
        if (error instanceof BrowserApiError && typeof error.details.correlation_id === "string") this.analysisFailureCorrelationId = error.details.correlation_id;
        this.error = creationAnalysisFailureMessage(error);
        return null;
      }
    },
    async loadActorGroups() {
      try { this.actorGroups = await client.listActorGroups(); }
      catch { this.error = "无法加载实例分组。"; }
    },
    async ensureDefaultGroup(actorIds: string[]) {
      try {
        let groups = await client.listActorGroups();
        let plan = planDefaultGroupMembership(groups, actorIds);
        let defaultGroup = plan.defaultGroup;
        if (!defaultGroup) {
          defaultGroup = await client.createActorGroup("默认");
          groups = [...groups, defaultGroup];
          plan = planDefaultGroupMembership(groups, actorIds);
        }
        this.actorGroups = groups;
        for (const actorId of plan.ungroupedActorIds) await client.assignActorGroupMember(defaultGroup.id, actorId);
        await this.loadActorGroups();
        this.selectedActorGroupId = defaultGroup.id;
      } catch {
        this.error = "无法准备默认实例分组。";
      }
    },
    async createActorGroup() {
      const name = this.newActorGroupName.trim();
      if (!name) return null;
      this.saving = true;
      try {
        const created = await client.createActorGroup(name);
        this.newActorGroupName = "";
        await this.loadActorGroups();
        return created;
      } catch { this.error = "无法创建实例分组。"; return null; }
      finally { this.saving = false; }
    },
    async assignActorGroupMember(groupId: string, actorId: string) {
      this.saving = true;
      try { await client.assignActorGroupMember(groupId, actorId); await this.loadActorGroups(); }
      catch { this.error = "无法加入实例分组。"; }
      finally { this.saving = false; }
    },
    async removeActorGroupMember(groupId: string, actorId: string) {
      this.saving = true;
      try { await client.removeActorGroupMember(groupId, actorId); await this.loadActorGroups(); }
      catch { this.error = "无法移出实例分组。"; }
      finally { this.saving = false; }
    },
    async activateFluctlight(body: {
      requestId: string;
      initializationMode: "blank_slate" | "llm_defined";
      identity: Record<string, unknown>;
      personality?: Record<string, unknown>;
      behavioralPolicy?: Record<string, unknown>;
      lifeProfile?: Record<string, unknown>;
      foundationProvenance?: Record<string, unknown>;
      initialGoals?: Array<Record<string, unknown>>;
      initialIntentions?: Array<Record<string, unknown>>;
    }) {
      this.saving = true;
      this.error = "";
      try { return await client.activateFluctlightCreation(body); }
      catch (error) { this.error = creationActivationFailureMessage(error); return null; }
      finally { this.saving = false; }
    },
    async loadDiagnostics() {
      const requestId = this.diagnosticsRequestId + 1;
      this.diagnosticsRequestId = requestId;
      this.loading = true;
      this.error = "";
      this.diagnosticsWarning = "";
      this.diagnosticsNotice = "";
      try {
        const correlationId = this.diagnosticsCorrelationFilter.trim() || undefined;
        const [events, modelRuns, workflows] = await Promise.allSettled([
          client.diagnostics({ limit: 20, correlationId }),
          client.diagnosticModelRuns({ limit: 20, correlationId }),
          client.listWorkflows(),
        ]);
        if (requestId !== this.diagnosticsRequestId) return;
        if (events.status === "fulfilled") this.diagnostics = events.value;
        if (modelRuns.status === "fulfilled") this.diagnosticModelRuns = modelRuns.value;
        if (workflows.status === "fulfilled") this.workflows = workflows.value;
        const readFailure = [events, modelRuns].find((result) => result.status === "rejected");
        if (readFailure?.status === "rejected") this.error = diagnosticsFailureMessage(readFailure.reason);
        if (workflows.status === "rejected") {
          this.diagnosticsWarning = "工作流运行时暂不可用；模型运行和系统事件仍可查看。";
        }
        this.diagnosticsLoaded = true;
        this.diagnosticsLastLoadedAt = new Date().toISOString();
        if (!this.error) this.diagnosticsNotice = correlationId ? `已刷新 ${correlationId} 的诊断记录。` : "诊断记录已刷新。";
      } finally {
        if (requestId === this.diagnosticsRequestId) this.loading = false;
      }
    },
    async exportDiagnostics() {
      this.saving = true;
      this.error = "";
      try {
        const payload = await client.exportDiagnostics({
          limit: 500,
          correlationId: this.diagnosticsCorrelationFilter.trim() || undefined,
        });
        const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
        const link = document.createElement("a");
        link.href = URL.createObjectURL(blob);
        link.download = "fluctlight-diagnostics.json";
        link.click();
        URL.revokeObjectURL(link.href);
        this.diagnosticsNotice = "诊断记录已导出。";
      } catch { this.error = "无法导出诊断信息。"; }
      finally { this.saving = false; }
    },
    async queryWorkflowStatus() {
      const workflowId = this.workflowId.trim();
      if (!workflowId) return;
      this.error = "";
      try { this.workflowStatus = await client.workflowStatus(workflowId); }
      catch { this.error = "无法读取工作流状态。"; }
    },
    async queryWorkflowHistory() {
      const workflowId = this.workflowId.trim();
      if (!workflowId) return;
      try { this.workflowHistory = await client.workflowHistory(workflowId); }
      catch { this.error = "无法读取工作流历史。"; }
    },
    async commandWorkflow(action: "pause" | "resume" | "cancel") {
      const workflowId = this.workflowId.trim();
      if (!workflowId) return;
      this.saving = true;
      try { await client.workflowCommand(workflowId, action); await this.queryWorkflowStatus(); }
      catch { this.error = "工作流命令未被接受。"; }
      finally { this.saving = false; }
    },
    async resetWorkflow() {
      const workflowId = this.workflowId.trim();
      const historyPoint = Number(this.workflowHistoryPoint);
      if (!workflowId || !Number.isInteger(historyPoint) || historyPoint < 1) { this.error = "Reset 需要工作流 ID 和正的 history point。"; return; }
      this.saving = true;
      try { await client.resetWorkflow(workflowId, historyPoint); await this.queryWorkflowStatus(); }
      catch { this.error = "工作流 Reset 未被接受。"; }
      finally { this.saving = false; }
    },
    async restartWorkflow() {
      const workflowId = this.workflowId.trim();
      if (!workflowId) return;
      this.saving = true;
      try { await client.restartWorkflow(workflowId); await this.queryWorkflowStatus(); }
      catch { this.error = "工作流重启未被接受。"; }
      finally { this.saving = false; }
    },
    async loadMoments(fluctlightId: string | null) {
      if (this.momentsScope === "fluctlight" && !fluctlightId) { this.moments = []; return; }
      this.loading = true;
      this.error = "";
      try {
        if (this.momentsScope === "global") {
          this.moments = await client.globalMoments(this.includeHiddenMoments);
        } else if (fluctlightId) {
          this.moments = await client.moments(fluctlightId, this.includeHiddenMoments);
          await client.markMomentsRead(fluctlightId);
        }
      }
      catch { this.error = "无法加载 Fluctlight 动态。"; }
      finally { this.loading = false; }
    },
    async loadFluctlightDetail(fluctlightId: string | null) {
      if (!fluctlightId) { this.fluctlightDetail = null; return; }
      this.loading = true;
      this.error = "";
      try { this.fluctlightDetail = await client.detail(fluctlightId); }
      catch { this.error = "无法加载 Fluctlight 的当前状态。"; }
      finally { this.loading = false; }
    },
    async setFluctlightStatus(fluctlightId: string | null, status: "active" | "paused") {
      const detail = this.fluctlightDetail;
      const reason = this.governanceReason.trim();
      if (!fluctlightId || !detail || !reason) {
        this.error = "状态治理需要填写原因。";
        return;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.setStatus(fluctlightId, {
          status,
          expectedRevision: Number(detail.current_revision ?? 0),
          reason,
        });
        this.governanceReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法更新 Fluctlight 状态，可能已被其他治理操作更新。"; }
      finally { this.saving = false; }
    },
    async retireFluctlight(fluctlightId: string | null, reason: string) {
      const detail = this.fluctlightDetail;
      if (!fluctlightId || !detail || !reason.trim()) {
        this.error = "删除摇光需要填写原因。";
        return false;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.retireFluctlight(fluctlightId, {
          expectedRevision: Number(detail.current_revision ?? 0),
          reason: reason.trim(),
        });
        this.fluctlightDetail = null;
        this.autonomyActions = [];
        return true;
      } catch {
        this.error = "无法删除摇光，可能已被其他治理操作更新。";
        return false;
      } finally { this.saving = false; }
    },
    async submitFoundationRevision(fluctlightId: string | null) {
      const detail = this.fluctlightDetail;
      if (!fluctlightId || !detail || !this.revisionReason.trim()) {
        this.error = "修订需要填写变更原因。";
        return;
      }
      let changes: Record<string, unknown>;
      try {
        changes = JSON.parse(this.revisionChangesJson) as Record<string, unknown>;
        if (!changes || Array.isArray(changes) || !Object.keys(changes).length) throw new Error("invalid_changes");
      } catch {
        this.error = "修订内容必须是包含字段变更的 JSON 对象。";
        return;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.submitFoundationRevision(fluctlightId, {
          changes,
          expectedRevision: Number(detail.current_revision ?? 0),
          reason: this.revisionReason.trim(),
        });
        this.revisionChangesJson = "";
        this.revisionReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法提出修订，字段、revision 或治理策略可能不满足要求。"; }
      finally { this.saving = false; }
    },
    async acceptFoundationRevision(fluctlightId: string | null, revisionId: string) {
      const detail = this.fluctlightDetail;
      const reason = this.revisionReason.trim();
      if (!fluctlightId || !detail || !reason) {
        this.error = "接受修订需要填写原因。";
        return;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.acceptFoundationRevision(fluctlightId, revisionId, {
          expectedRevision: Number(detail.current_revision ?? 0),
          reason,
        });
        this.revisionReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法接受修订，当前基础版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async rejectFoundationRevision(fluctlightId: string | null, revisionId: string) {
      const detail = this.fluctlightDetail;
      const reason = this.revisionReason.trim();
      if (!fluctlightId || !detail || !reason) {
        this.error = "拒绝修订需要填写原因。";
        return;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.rejectFoundationRevision(fluctlightId, revisionId, {
          expectedRevision: Number(detail.current_revision ?? 0),
          reason,
        });
        this.revisionReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法拒绝修订，当前基础版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async rollbackFoundationRevision(fluctlightId: string | null) {
      const detail = this.fluctlightDetail;
      const reason = this.revisionReason.trim();
      const targetRevision = Number(this.rollbackTargetRevision);
      if (!fluctlightId || !detail || !reason || !Number.isInteger(targetRevision) || targetRevision < 0) {
        this.error = "回滚需要目标 revision 和原因。";
        return;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.rollbackFoundationRevision(fluctlightId, {
          targetRevision,
          expectedRevision: Number(detail.current_revision ?? 0),
          reason,
        });
        this.rollbackTargetRevision = "";
        this.revisionReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法回滚修订，目标必须是已接受 revision 且当前版本未变化。"; }
      finally { this.saving = false; }
    },
    async reviseMemory(memory: Record<string, unknown>) {
      const content = this.memoryEdits[String(memory.id)]?.trim();
      const evidenceRefs = this.governanceEvidence.split(",").map((value) => value.trim()).filter(Boolean);
      if (!content || !evidenceRefs.length) { this.error = "修正记忆需要新内容和至少一条证据引用。"; return; }
      this.saving = true;
      try { await client.reviseMemory(String(memory.id), { expectedRevision: Number(memory.revision ?? 0), content, evidenceRefs }); await this.loadFluctlightDetail(String(memory.owner_fluctlight_id ?? "") || null); }
      catch { this.error = "无法修正记忆，版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async forgetMemory(memory: Record<string, unknown>) {
      const evidenceRefs = this.governanceEvidence.split(",").map((value) => value.trim()).filter(Boolean);
      if (!evidenceRefs.length) { this.error = "遗忘记忆需要至少一条证据引用。"; return; }
      this.saving = true;
      try { await client.forgetMemory(String(memory.id), { expectedRevision: Number(memory.revision ?? 0), evidenceRefs }); await this.loadFluctlightDetail(String(memory.owner_fluctlight_id ?? "") || null); }
      catch { this.error = "无法遗忘记忆，版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async rollbackRelationship(fluctlightId: string | null, relationship: Record<string, unknown>) {
      const evidenceRefs = this.governanceEvidence.split(",").map((value) => value.trim()).filter(Boolean);
      const targetRevision = Number(this.relationshipRollbackTargets[String(relationship.target_actor_id)]);
      if (!fluctlightId || !evidenceRefs.length || !Number.isInteger(targetRevision) || targetRevision < 0) { this.error = "关系回滚需要目标 revision 和至少一条证据引用。"; return; }
      this.saving = true;
      try { await client.rollbackRelationship(fluctlightId, { targetActorId: String(relationship.target_actor_id), targetRevision, expectedRevision: Number(relationship.revision ?? 0), evidenceRefs }); await this.loadFluctlightDetail(fluctlightId); }
      catch { this.error = "无法回滚关系，目标或当前版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async loadAutonomyActions(fluctlightId: string | null) {
      if (!fluctlightId) { this.autonomyActions = []; return; }
      try { this.autonomyActions = await client.listAutonomyActions(fluctlightId); }
      catch { this.error = "无法加载自治动作。"; }
    },
    async governAutonomyAction(actionId: string, status: "paused" | "deferred" | "cancelled", fluctlightId: string | null) {
      const reason = this.governanceReason.trim();
      if (!reason) { this.error = "治理自治动作需要填写原因。"; return; }
      this.saving = true;
      try { await client.governAutonomyAction(actionId, { status, reason }); this.governanceReason = ""; await this.loadAutonomyActions(fluctlightId); }
      catch { this.error = "无法治理自治动作。"; }
      finally { this.saving = false; }
    },
    async createLifeEvent(fluctlightId: string | null) {
      const evidenceRefs = this.governanceEvidence.split(",").map((value) => value.trim()).filter(Boolean);
      const event = this.lifeEvent;
      if (!fluctlightId || !event.kind.trim() || !event.startAt || !event.endAt || !evidenceRefs.length) { this.error = "创建 Event 需要类型、起止时间和证据引用。"; return; }
      this.saving = true;
      try {
        await client.createLifeEvent(fluctlightId, {
          ...event,
          startAt: new Date(event.startAt).toISOString(),
          endAt: new Date(event.endAt).toISOString(),
          evidenceRefs,
        });
        this.lifeEvent = { kind: "", startAt: "", endAt: "", scene: "", activity: "", location: "" };
        await this.loadFluctlightDetail(fluctlightId);
      }
      catch { this.error = "无法创建 Event，请检查时间范围和证据引用。"; }
      finally { this.saving = false; }
    },
    async setPresence(fluctlightId: string | null) {
      if (!fluctlightId) return;
      this.saving = true;
      try { await client.setLifePresence(fluctlightId, { currentTask: this.presence.currentTask || undefined, userPresence: this.presence.userPresence || undefined }); await this.loadFluctlightDetail(fluctlightId); }
      catch { this.error = "无法更新 Presence overlay。"; }
      finally { this.saving = false; }
    },
    async acceptSchedule(fluctlightId: string | null) {
      const evidenceRefs = this.governanceEvidence.split(",").map((value) => value.trim()).filter(Boolean);
      if (!fluctlightId || !evidenceRefs.length) { this.error = "提交日程需要至少一条证据引用。"; return; }
      let draft: Record<string, unknown>;
      try {
        draft = JSON.parse(this.scheduleDraftJson) as Record<string, unknown>;
        if (!draft || Array.isArray(draft) || !Array.isArray(draft.items)) throw new Error("invalid_schedule");
      } catch { this.error = "日程必须是包含 localDate、timezone 和 items 的 JSON 对象。"; return; }
      const currentSchedule = this.fluctlightDetail?.schedule as Record<string, unknown> | null | undefined;
      const expectedRevision = currentSchedule?.revision;
      this.saving = true;
      try {
        await client.acceptLifeSchedule(fluctlightId, {
          ...(draft as { localDate: string; timezone: string; items: Array<{ startAt: string; endAt: string; activity: string; scene: string }> }),
          evidenceRefs,
          expectedRevision: typeof expectedRevision === "number" ? expectedRevision : undefined,
        });
        this.scheduleDraftJson = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法提交日程。它必须覆盖完整本地日，并与当前 revision 一致。"; }
      finally { this.saving = false; }
    },
    async cancelSchedule(fluctlightId: string | null) {
      const schedule = this.fluctlightDetail?.schedule as Record<string, unknown> | null | undefined;
      if (!fluctlightId || !schedule?.id || typeof schedule.revision !== "number") return;
      this.saving = true;
      try { await client.cancelLifeSchedule(fluctlightId, String(schedule.id), schedule.revision); await this.loadFluctlightDetail(fluctlightId); }
      catch { this.error = "无法取消日程，当前版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async cancelLifeEvent(fluctlightId: string | null, eventId: string) {
      if (!fluctlightId) return;
      this.saving = true;
      try { await client.cancelLifeEvent(fluctlightId, eventId); await this.loadFluctlightDetail(fluctlightId); }
      catch { this.error = "无法取消 Event。"; }
      finally { this.saving = false; }
    },
    async saveOperationalSettings() {
      let autonomy: Record<string, unknown>;
      let retention: Record<string, unknown>;
      try {
        autonomy = JSON.parse(this.autonomySettingsJson) as Record<string, unknown>;
        retention = JSON.parse(this.diagnosticsRetentionJson) as Record<string, unknown>;
        if (!autonomy || Array.isArray(autonomy) || !retention || Array.isArray(retention)) throw new Error("invalid_settings");
      } catch { this.error = "自治和诊断保留策略必须是 JSON 对象。"; return; }
      await this.saveSettings({ "product.autonomy": autonomy, "diagnostics.retention": retention });
    },
    async reactToMoment(momentId: string, fluctlightId: string | null) {
      this.error = "";
      try {
        await client.reactToMoment(momentId);
        this.momentNotice = "已记录反应。";
        await this.loadMoments(fluctlightId);
      }
      catch { this.error = "无法记录对动态的反应。"; }
    },
    async setMomentStatus(momentId: string, action: "hide" | "restore", fluctlightId: string | null) {
      this.error = "";
      try {
        await client.setMomentStatus(momentId, action);
        this.momentNotice = action === "hide" ? "动态已隐藏。" : "动态已恢复。";
        await this.loadMoments(fluctlightId);
      } catch { this.error = "无法更新动态状态。"; }
    },
    async commentOnMoment(momentId: string, fluctlightId: string | null) {
      const text = this.momentDrafts[momentId]?.trim();
      if (!text) return;
      this.error = "";
      try {
        await client.commentOnMoment(momentId, text);
        this.momentDrafts[momentId] = "";
        this.momentNotice = "评论已保存。";
        await this.loadMoments(fluctlightId);
      } catch { this.error = "无法保存评论。"; }
    },
    async clearDiagnostics() {
      if (typeof window !== "undefined" && !window.confirm("确定清空所有诊断记录吗？此操作不可撤销。")) return;
      this.saving = true;
      this.error = "";
      try {
        await client.clearDiagnostics();
        this.diagnostics = [];
        this.diagnosticModelRuns = [];
        this.workflows = [];
        this.diagnosticsWarning = "";
        this.diagnosticsNotice = "诊断记录已清空。";
      } catch {
        this.error = "无法清空诊断信息。";
      } finally { this.saving = false; }
    },
    async loadSettings() {
      this.loading = true;
      this.error = "";
      try {
        this.settings = await client.settings();
        this.providerBindings = await client.providerBindings();
        this.providerEndpoints = await client.providerEndpoints();
      } catch {
        this.error = "设置暂时不可用。";
      } finally {
        this.loading = false;
      }
    },
    async loadProviderModels(endpointId: string) {
      this.providerModels = [];
      this.providerModelsEndpointId = endpointId;
      this.providerModelsError = "";
      if (!endpointId) return;
      try {
        const result = await client.providerEndpointModels(endpointId);
        if (this.providerModelsEndpointId !== endpointId) return;
        this.providerModels = result.models;
      } catch {
        if (this.providerModelsEndpointId === endpointId) {
          this.providerModelsError = "无法读取该 endpoint 的模型列表，可手动填写模型 ID。";
        }
      }
    },
    async saveSettings(values: Record<string, unknown>, secrets: Record<string, string> = {}) {
      this.saving = true;
      this.error = "";
      try {
        this.settings = await client.updateSettings({
          values,
          secrets,
        });
      } catch {
        this.error = "无法保存设置。";
      } finally {
        this.saving = false;
      }
    },
    async configureProviderEndpoint(input: {
      endpointId: string;
      kind: string;
      baseUrl: string;
      secretPurpose: string;
    }) {
      this.saving = true;
      this.error = "";
      try {
        await client.configureProviderEndpoint({
          endpointId: input.endpointId,
          kind: input.kind,
          baseUrl: input.baseUrl,
          secretPurpose: input.secretPurpose,
        });
        this.providerBindings = await client.providerBindings();
        this.providerEndpoints = await client.providerEndpoints();
      } catch {
        this.error = "无法保存模型 endpoint。请检查地址和协议类型。";
      } finally {
        this.saving = false;
      }
    },
    async configureModelRole(input: {
      role: string;
      endpointId: string;
      modelId: string;
      tokenBudget: number;
      timeoutSeconds: number;
    }) {
      this.saving = true;
      this.error = "";
      try {
        await client.configureModelRole({
          role: input.role,
          endpointId: input.endpointId,
          modelId: input.modelId,
          tokenBudget: input.tokenBudget,
          timeoutSeconds: input.timeoutSeconds,
        });
        this.providerBindings = await client.providerBindings();
        this.providerEndpoints = await client.providerEndpoints();
      } catch (error) {
        this.error = providerRoleFailureMessage(error);
      } finally {
        this.saving = false;
      }
    },
  },
});

function providerRoleFailureMessage(error: unknown): string {
  if (!(error instanceof BrowserApiError)) {
    return "无法保存模型角色，请稍后重试。";
  }
  switch (error.code) {
    case "provider_endpoint_not_found":
      return "该 endpoint 尚未保存，请先保存 endpoint 后再绑定模型角色。";
    case "provider_endpoint_invalid":
      return "endpoint 配置无效，请检查服务地址和协议类型。";
    case "provider_model_not_available":
      return "该模型未出现在 endpoint 返回的模型列表中，请确认模型 ID 完全一致。";
    case "provider_models_unavailable":
      return "无法读取 endpoint 的模型列表，请检查地址、访问密钥，以及 Core 容器是否能访问该 endpoint。";
    case "provider_role_invalid":
      return "模型角色配置无效，请重新选择角色、endpoint 和模型。";
    default:
      return "模型预检失败，请检查 endpoint、访问密钥和模型配置。";
  }
}

function creationAnalysisFailureMessage(error: unknown): string {
  if (!(error instanceof BrowserApiError)) return "Fluctlight 分析服务暂时不可用。";
  if (error.code === "unauthenticated") return "登录会话已失效，请重新登录后再分析。";
  if (error.code === "initialization_role_unconfigured") return "初始化模型角色未配置或预检未通过。";
  if (error.code === "initialization_response_invalid_json") return "初始化模型没有返回合法 JSON。";
  if (error.code === "initialization_response_invalid") return "初始化模型返回的 JSON 结构无效。";
  if (error.code === "initialization_foundation_invalid") {
    const detail = error.details.validation_error;
    return typeof detail === "string"
      ? `初始化模型返回的 Foundation 不符合要求：${detail}`
      : "初始化模型返回的 Foundation 结构不符合要求，请查看诊断中的 Prompt 和 Response。";
  }
  if (error.code === "core_request_validation_failed") {
    const errors = error.details.validation_errors;
    if (Array.isArray(errors)) {
      const paths = errors.map((item) => {
        if (!item || typeof item !== "object") return "未知字段";
        const location = (item as Record<string, unknown>).location;
        return Array.isArray(location) ? location.join(".") : "未知字段";
      }).join("、");
      return `Core 请求校验失败：${paths || "请查看诊断日志"}`;
    }
    return "Core 请求校验失败，请查看诊断日志。";
  }
  if (error.code === "initialization_provider_unavailable") return "初始化模型 Provider 不可用或请求超时。";
  return error.userMessage || "Fluctlight 分析失败。";
}

function creationActivationFailureMessage(error: unknown): string {
  if (!(error instanceof BrowserApiError)) return "Fluctlight 激活服务暂时不可用。";
  if (error.code === "unauthenticated") return "登录会话已失效，请重新登录后再激活。";
  if (error.code === "activation_foundation_invalid") return "预览中的 Foundation 结构无效。";
  if (error.code === "activation_foundation_incomplete") return "预览缺少完整人格或行为策略。";
  if (error.code === "activation_request_conflict") return "该激活请求已被不同的预览内容占用。";
  if (error.code === "activation_persistence_failed") return "Fluctlight 数据无法保存，请查看诊断信息。";
  return error.userMessage || "Fluctlight 激活失败。";
}

function diagnosticsFailureMessage(error: unknown): string {
  if (error instanceof BrowserApiError) {
    if (error.status === 401) return "登录状态已失效，请重新登录后查看诊断。";
    if (error.status === 403) return "诊断信息仅对所有者可用。";
    if (error.status >= 500) return "诊断运行时暂时不可用，请确认 Core、数据库和 Worker 正在运行。";
    return `无法读取诊断信息：${error.message}`;
  }
  return "无法读取诊断信息，请确认 BFF 与 Core 均在运行。";
}
