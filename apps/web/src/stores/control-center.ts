import { defineStore } from "pinia";
import {
  BrowserClient,
  BrowserApiError,
  type BrowserDiagnosticEvent,
  type BrowserDiagnosticMediaPrompt,
  type BrowserDiagnosticModelRun,
  type BrowserFluctlightActivationRequest,
  type BrowserFluctlightDetail,
  type BrowserLifecycleDiagnosticEvent,
  type BrowserLifecycleDiagnosticsFilter,
  type BrowserSafeSettings,
  type BrowserWorkflowIntentSnapshot,
} from "@fluctlight/browser-client";
import { apiOrigin } from "../runtime-config";
import { planDefaultGroupMembership } from "../lib/group-membership";
import { normalizeActorGroups, type ActorGroupSnapshot } from "../lib/actor-groups";

const client = new BrowserClient(apiOrigin);
const relationshipKey = (relationship: Record<string, unknown>): string => `${String(relationship.target_actor_id ?? "")}::${String(relationship.profile_id ?? "shared")}`;

export const useControlCenterStore = defineStore("control-center", {
  state: () => ({
    diagnostics: [] as BrowserDiagnosticEvent[],
    diagnosticModelRuns: [] as BrowserDiagnosticModelRun[],
    diagnosticMediaPrompts: [] as BrowserDiagnosticMediaPrompt[],
    lifecycleDiagnostics: [] as BrowserLifecycleDiagnosticEvent[],
    workflowIntentSnapshots: [] as BrowserWorkflowIntentSnapshot[],
    workflows: [] as Array<Record<string, unknown>>,
    workflowListLoading: false,
    workflowId: "",
    workflowStatus: null as Record<string, unknown> | null,
    workflowHistory: null as Record<string, unknown> | null,
    workflowHistoryPoint: "",
    diagnosticsCorrelationFilter: "",
    diagnosticsFluctlightFilter: "",
    diagnosticsIntentFilter: "",
    diagnosticsWorkflowFilter: "",
    diagnosticsRunFilter: "",
    diagnosticsSurfaceFilter: "",
    diagnosticsStatusFilter: "",
    diagnosticsSourceEpochs: { lifecycle: "", events: "", modelRuns: "", mediaPrompts: "" } as Record<string, string>,
    diagnosticsWarning: "",
    diagnosticsNotice: "",
    diagnosticsLoaded: false,
    diagnosticsLastLoadedAt: "",
    diagnosticsRequestId: 0,
    creationAnalysisRequestId: 0,
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
    fluctlightDetail: null as BrowserFluctlightDetail | null,
	fluctlightDetailRequestId: 0,
	fluctlightDetailFluctlightId: "",
    governanceReason: "",
    governanceNotice: "",
    revisionChangesJson: "",
    revisionReason: "",
    rollbackTargetRevision: "",
    governanceEvidence: "",
    memoryEdits: {} as Record<string, string>,
    relationshipRollbackTargets: {} as Record<string, string>,
    relationshipEditDrafts: {} as Record<string, { role: string; metrics: string; trend: string; summary: string; emotionalAssociation: string }>,
    autonomyActions: [] as Array<{ id: string; action_type: string; status: string; workflow_id: string; created_at: string }>,
    capabilityRequests: [] as Array<Record<string, unknown>>,
    capabilityRequestVersions: {} as Record<string, string>,
    lifeEvent: { kind: "", startAt: "", endAt: "", scene: "", activity: "", location: "" },
    presence: { currentTask: "", userPresence: "" },
    scheduleDraftJson: "",
    lifeCommandKeys: {} as Record<string, string>,
    actorGroups: [] as ActorGroupSnapshot[],
    newActorGroupName: "",
    selectedActorGroupId: "",
    autonomySettingsJson: "",
    wakeUpSettingsJson: "",
    diagnosticsRetentionJson: "",
    settings: null as BrowserSafeSettings | null,
    loading: false,
    saving: false,
    error: "",
  }),
  actions: {
    lifeCommandKey(identity: string): string {
		if (!this.lifeCommandKeys[identity]) this.lifeCommandKeys[identity] = `owner-ui:${crypto.randomUUID()}`;
		return this.lifeCommandKeys[identity];
	},
    clearLifeCommandKey(identity: string) {
		delete this.lifeCommandKeys[identity];
	},
    async analyzeFluctlight(description: string) {
	  const requestId = this.creationAnalysisRequestId + 1;
	  this.creationAnalysisRequestId = requestId;
      this.error = "";
      this.analysisFailureCorrelationId = "";
      try {
		const result = await client.analyzeFluctlightCreation(description);
		if (requestId !== this.creationAnalysisRequestId) return null;
		return result;
      } catch (error) {
		if (requestId !== this.creationAnalysisRequestId) return null;
        if (error instanceof BrowserApiError && typeof error.details.correlation_id === "string") this.analysisFailureCorrelationId = error.details.correlation_id;
        this.error = creationAnalysisFailureMessage(error);
        return null;
      }
    },
    async loadActorGroups() {
      try { this.actorGroups = normalizeActorGroups(await client.listActorGroups()); }
      catch { this.error = "无法加载实例分组。"; }
    },
    async ensureDefaultGroup(actorIds: string[]) {
      try {
        let groups = normalizeActorGroups(await client.listActorGroups());
        let plan = planDefaultGroupMembership(groups, actorIds);
        let defaultGroup = plan.defaultGroup;
        if (!defaultGroup) {
          const created = normalizeActorGroups([await client.createActorGroup("默认")])[0];
          if (!created) throw new Error("actor_group_invalid_response");
          defaultGroup = created;
          groups = [...groups, created];
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
    async activateFluctlight(body: BrowserFluctlightActivationRequest) {
      this.saving = true;
      this.error = "";
      try { return await client.activateFluctlightCreation(body); }
      catch (error) { this.error = creationActivationFailureMessage(error); return null; }
      finally { this.saving = false; }
    },
    async loadDiagnostics() {
      const requestId = this.diagnosticsRequestId + 1;
      this.diagnosticsRequestId = requestId;
      const initialLoad = !this.diagnosticsLoaded;
      if (initialLoad) this.loading = true;
      this.error = "";
      this.diagnosticsNotice = "";
      try {
        const correlationId = this.diagnosticsCorrelationFilter.trim() || undefined;
        const fluctlightId = this.diagnosticsFluctlightFilter.trim() || undefined;
        const lifecycleFilters: BrowserLifecycleDiagnosticsFilter = {
          limit: 100,
          correlationId,
          fluctlightId,
          intentId: this.diagnosticsIntentFilter.trim() || undefined,
          workflowId: this.diagnosticsWorkflowFilter.trim() || undefined,
          runId: this.diagnosticsRunFilter.trim() || undefined,
          surface: this.diagnosticsSurfaceFilter.trim() || undefined,
          status: this.diagnosticsStatusFilter.trim() || undefined,
        };
        const epochs = {
          lifecycle: JSON.stringify(lifecycleFilters),
          events: JSON.stringify({ correlationId, fluctlightId }),
          modelRuns: JSON.stringify({ correlationId }),
          mediaPrompts: "unfiltered",
        };
        if (this.diagnosticsSourceEpochs.lifecycle !== epochs.lifecycle) {
          this.lifecycleDiagnostics = [];
          this.workflowIntentSnapshots = [];
          this.diagnosticsSourceEpochs.lifecycle = epochs.lifecycle;
        }
        if (this.diagnosticsSourceEpochs.events !== epochs.events) {
          this.diagnostics = [];
          this.diagnosticsSourceEpochs.events = epochs.events;
        }
        if (this.diagnosticsSourceEpochs.modelRuns !== epochs.modelRuns) {
          this.diagnosticModelRuns = [];
          this.diagnosticsSourceEpochs.modelRuns = epochs.modelRuns;
        }
        if (this.diagnosticsSourceEpochs.mediaPrompts !== epochs.mediaPrompts) {
          this.diagnosticMediaPrompts = [];
          this.diagnosticsSourceEpochs.mediaPrompts = epochs.mediaPrompts;
        }
        const [lifecycle, events, modelRuns, mediaPrompts] = await Promise.allSettled([
          client.lifecycleDiagnostics(lifecycleFilters),
          client.diagnostics({ limit: 20, correlationId, fluctlightId }),
          client.diagnosticModelRuns({ limit: 20, correlationId }),
          client.diagnosticMediaPrompts({ limit: 20 }),
        ]);
        if (requestId !== this.diagnosticsRequestId) return;
        if (lifecycle.status === "fulfilled" && this.diagnosticsSourceEpochs.lifecycle === epochs.lifecycle) {
          this.lifecycleDiagnostics = lifecycle.value.events;
          this.workflowIntentSnapshots = lifecycle.value.workflowIntents;
        }
        if (events.status === "fulfilled" && this.diagnosticsSourceEpochs.events === epochs.events) this.diagnostics = events.value;
        if (modelRuns.status === "fulfilled" && this.diagnosticsSourceEpochs.modelRuns === epochs.modelRuns) this.diagnosticModelRuns = modelRuns.value;
        if (mediaPrompts.status === "fulfilled" && this.diagnosticsSourceEpochs.mediaPrompts === epochs.mediaPrompts) this.diagnosticMediaPrompts = mediaPrompts.value;
        const readFailure = [lifecycle, events, modelRuns, mediaPrompts].find((result) => result.status === "rejected");
        if (readFailure?.status === "rejected") this.error = diagnosticsFailureMessage(readFailure.reason);
        this.diagnosticsLoaded = true;
        this.diagnosticsLastLoadedAt = new Date().toISOString();
        if (!this.error) this.diagnosticsNotice = correlationId ? `已刷新 ${correlationId} 的诊断记录。` : "诊断记录已刷新。";
      } finally {
        if (requestId === this.diagnosticsRequestId && initialLoad) this.loading = false;
      }
    },
    async loadWorkflows() {
      if (this.workflowListLoading) return;
      this.workflowListLoading = true;
      this.diagnosticsWarning = "";
      try {
        this.workflows = await client.listWorkflows();
      } catch {
        this.workflows = [];
        this.diagnosticsWarning = "工作流运行时暂不可用；模型运行和系统事件仍可查看。";
      } finally {
        this.workflowListLoading = false;
      }
    },
    async exportDiagnostics() {
      this.saving = true;
      this.error = "";
      try {
        const payload = await client.exportDiagnostics({
          limit: 500,
          correlationId: this.diagnosticsCorrelationFilter.trim() || undefined,
          fluctlightId: this.diagnosticsFluctlightFilter.trim() || undefined,
          intentId: this.diagnosticsIntentFilter.trim() || undefined,
          workflowId: this.diagnosticsWorkflowFilter.trim() || undefined,
          runId: this.diagnosticsRunFilter.trim() || undefined,
          surface: this.diagnosticsSurfaceFilter.trim() || undefined,
          status: this.diagnosticsStatusFilter.trim() || undefined,
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
	  const requestId = this.fluctlightDetailRequestId + 1;
	  this.fluctlightDetailRequestId = requestId;
	  const targetId = fluctlightId ?? "";
	  const idChanged = this.fluctlightDetailFluctlightId !== targetId;
	  this.fluctlightDetailFluctlightId = targetId;
	  if (idChanged) this.fluctlightDetail = null;
	  if (!targetId) { this.loading = false; return; }
	  if (idChanged || !this.fluctlightDetail) this.loading = true;
	  this.error = "";
      try {
		const detail = await client.detail(targetId);
		if (requestId !== this.fluctlightDetailRequestId || targetId !== this.fluctlightDetailFluctlightId) return;
		this.fluctlightDetail = detail;
		const relationships = Array.isArray(detail.relationships) ? detail.relationships as Array<Record<string, unknown>> : [];
        this.relationshipEditDrafts = Object.fromEntries(relationships.map((relationship) => {
          const key = relationshipKey(relationship);
          return [key, {
            role: JSON.stringify(relationship.role ?? { primary: "unknown", secondary: [] }, null, 2),
            metrics: JSON.stringify(relationship.metrics ?? {}, null, 2),
            trend: String(relationship.trend ?? "stable"),
            summary: String(relationship.summary ?? ""),
            emotionalAssociation: JSON.stringify(relationship.emotional_association ?? {}, null, 2),
          }];
        }));
      }
	  catch {
		if (requestId !== this.fluctlightDetailRequestId || fluctlightId !== this.fluctlightDetailFluctlightId) return;
		this.fluctlightDetail = null;
		this.error = "无法加载 Fluctlight 的当前状态。";
	  }
	  finally {
		if (requestId === this.fluctlightDetailRequestId && fluctlightId === this.fluctlightDetailFluctlightId) this.loading = false;
	  }
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
    async triggerWakeUp(fluctlightId: string | null) {
      const detail = this.fluctlightDetail;
      if (!fluctlightId || !detail || detail.status !== "active") {
        this.error = "只有运行中的摇光可以立即唤醒。";
        return;
      }
      this.saving = true;
      this.error = "";
      this.governanceNotice = "";
      try {
        const result = await client.triggerWakeUp(fluctlightId);
        const status = String(result.status ?? "queued");
        const cycle = result.cycle == null ? "" : `（第 ${String(result.cycle)} 次）`;
        this.governanceNotice = status === "running" ? `唤醒任务正在执行${cycle}。` : `唤醒任务已加入后台队列${cycle}。`;
        await this.loadFluctlightDetail(fluctlightId);
      } catch {
        this.error = "无法触发立即唤醒，请检查 Worker 是否在线。";
      } finally { this.saving = false; }
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
    async rollbackDevelopingSelf(fluctlightId: string | null, claim: Record<string, unknown>) {
      const reason = this.governanceReason.trim();
      const claimId = String(claim.id ?? "");
      const revision = Number(claim.revision ?? 0);
      if (!fluctlightId || !claimId || !reason || !Number.isInteger(revision) || revision < 1) {
        this.error = "回滚自我认知需要 claim、revision 和原因。";
        return;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.rollbackDevelopingSelf(fluctlightId, claimId, { expectedRevision: revision, reason });
        this.governanceReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法回滚自我认知，当前版本可能已变化或没有可回滚版本。"; }
      finally { this.saving = false; }
    },
    async forgetDevelopingSelf(fluctlightId: string | null, claim: Record<string, unknown>) {
      const reason = this.governanceReason.trim();
      const claimId = String(claim.id ?? "");
      const revision = Number(claim.revision ?? 0);
      if (!fluctlightId || !claimId || !reason || !Number.isInteger(revision) || revision < 1) {
        this.error = "标记自我认知不准确需要 claim、revision 和原因。";
        return;
      }
      this.saving = true;
      this.error = "";
      try {
        await client.forgetDevelopingSelf(fluctlightId, claimId, { expectedRevision: revision, reason });
        this.governanceReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法标记自我认知，当前版本可能已变化。"; }
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
      const key = relationshipKey(relationship);
      const targetRevision = Number(this.relationshipRollbackTargets[key]);
      if (!fluctlightId || !evidenceRefs.length || !Number.isInteger(targetRevision) || targetRevision < 0) { this.error = "关系回滚需要目标 revision 和至少一条证据引用。"; return; }
      this.saving = true;
      try { await client.rollbackRelationship(fluctlightId, { targetActorId: String(relationship.target_actor_id), profileId: String(relationship.profile_id ?? "") || undefined, targetRevision, expectedRevision: Number(relationship.revision ?? 0), evidenceRefs }); await this.loadFluctlightDetail(fluctlightId); }
      catch { this.error = "无法回滚关系，目标或当前版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async editRelationship(fluctlightId: string | null, relationship: Record<string, unknown>) {
      const targetActorId = String(relationship.target_actor_id ?? "");
      const key = relationshipKey(relationship);
      const draft = this.relationshipEditDrafts[key];
      const evidenceRefs = this.governanceEvidence.split(",").map((value) => value.trim()).filter(Boolean);
      const reason = this.governanceReason.trim();
      if (!fluctlightId || !targetActorId || !draft || !evidenceRefs.length || !reason) {
        this.error = "编辑关系需要填写完整内容、证据引用和治理原因。";
        return;
      }
      const parseObject = (value: string, field: string): Record<string, unknown> | null => {
        try {
          const parsed = JSON.parse(value) as unknown;
          if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error(field);
          return parsed as Record<string, unknown>;
        } catch {
          this.error = `${field}必须是 JSON 对象。`;
          return null;
        }
      };
      const role = parseObject(draft.role, "关系角色");
      const metrics = parseObject(draft.metrics, "关系指标");
      const emotionalAssociation = parseObject(draft.emotionalAssociation, "情绪关联");
      if (!role || !metrics || !emotionalAssociation) return;
      this.saving = true;
      this.error = "";
      try {
        await client.editRelationship(fluctlightId, targetActorId, {
          profileId: String(relationship.profile_id ?? "") || undefined,
          expectedRevision: Number(relationship.revision ?? 0),
          role,
          metrics,
          trend: draft.trend,
          summary: draft.summary,
          emotionalAssociation,
          evidenceRefs,
          reason,
        });
        this.governanceReason = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法编辑关系，当前版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async loadAutonomyActions(fluctlightId: string | null) {
      if (!fluctlightId) { this.autonomyActions = []; return; }
      try { this.autonomyActions = await client.listAutonomyActions(fluctlightId); }
      catch { this.error = "无法加载自治动作。"; }
    },
    async loadCapabilityRequests() {
      try { this.capabilityRequests = await client.listCapabilityRequests(); }
      catch { this.error = "无法加载能力需求。"; }
    },
    async reviewCapabilityRequest(requestId: string, status: "reviewing" | "accepted" | "rejected" | "fulfilled" | "cancelled", capabilityVersion = "") {
      const note = this.governanceReason.trim();
      if (!note) { this.error = "审核能力需求需要填写原因或备注。"; return; }
      this.saving = true;
      try { await client.reviewCapabilityRequest(requestId, { status, note, ...(capabilityVersion.trim() ? { capabilityVersion: capabilityVersion.trim() } : {}) }); this.governanceReason = ""; await this.loadCapabilityRequests(); }
      catch { this.error = "无法更新能力需求状态。"; }
      finally { this.saving = false; }
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
	  const context = this.fluctlightDetail?.context as Record<string, unknown> | null | undefined;
	  const expectedLifeContextRevision = String(context?.context_revision ?? "");
	  if (!expectedLifeContextRevision) { this.error = "当前生活上下文缺少 revision，请刷新后重试。"; return; }
	  const commandIdentity = `event:create:${fluctlightId}:${expectedLifeContextRevision}:${JSON.stringify(event)}:${JSON.stringify(evidenceRefs)}`;
	  const idempotencyKey = this.lifeCommandKey(commandIdentity);
	  this.saving = true;
      try {
        await client.createLifeEvent(fluctlightId, {
          ...event,
          startAt: new Date(event.startAt).toISOString(),
          endAt: new Date(event.endAt).toISOString(),
          evidenceRefs,
		  expectedLifeContextRevision,
		  idempotencyKey,
        });
		this.clearLifeCommandKey(commandIdentity);
        this.lifeEvent = { kind: "", startAt: "", endAt: "", scene: "", activity: "", location: "" };
        await this.loadFluctlightDetail(fluctlightId);
      }
      catch { this.error = "无法创建 Event，请检查时间范围和证据引用。"; }
      finally { this.saving = false; }
    },
    async setPresence(fluctlightId: string | null) {
      if (!fluctlightId) return;
	  const context = this.fluctlightDetail?.context as Record<string, unknown> | null | undefined;
	  const expectedLifeContextRevision = String(context?.context_revision ?? "");
	  if (!expectedLifeContextRevision || (!this.presence.currentTask && !this.presence.userPresence)) { this.error = "更新 Presence 需要当前上下文 revision 和至少一个状态字段。"; return; }
	  const commandIdentity = `presence:set:${fluctlightId}:${expectedLifeContextRevision}:${JSON.stringify(this.presence)}`;
	  const idempotencyKey = this.lifeCommandKey(commandIdentity);
      this.saving = true;
	  try { await client.setLifePresence(fluctlightId, { currentTask: this.presence.currentTask || undefined, userPresence: this.presence.userPresence || undefined, expectedLifeContextRevision, idempotencyKey }); this.clearLifeCommandKey(commandIdentity); await this.loadFluctlightDetail(fluctlightId); }
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
	  const context = this.fluctlightDetail?.context as Record<string, unknown> | null | undefined;
	  const expectedLifeContextRevision = String(context?.context_revision ?? "");
	  if (!expectedLifeContextRevision) { this.error = "当前生活上下文缺少 revision，请刷新后重试。"; return; }
	  const normalizedExpectedRevision = typeof expectedRevision === "number" ? expectedRevision : 0;
	  const commandIdentity = `schedule:accept:${fluctlightId}:${normalizedExpectedRevision}:${expectedLifeContextRevision}:${JSON.stringify(draft)}:${JSON.stringify(evidenceRefs)}`;
	  const idempotencyKey = this.lifeCommandKey(commandIdentity);
      this.saving = true;
      try {
        await client.acceptLifeSchedule(fluctlightId, {
	          ...(draft as { localDate: string; timezone: string; items: Array<{ startAt: string; endAt: string; activity: string; scene: string; location?: string }> }),
          evidenceRefs,
		  expectedRevision: normalizedExpectedRevision,
		  expectedLifeContextRevision,
		  idempotencyKey,
        });
		this.clearLifeCommandKey(commandIdentity);
        this.scheduleDraftJson = "";
        await this.loadFluctlightDetail(fluctlightId);
      } catch { this.error = "无法提交日程。它必须覆盖完整本地日，并与当前 revision 一致。"; }
      finally { this.saving = false; }
    },
    async cancelSchedule(fluctlightId: string | null) {
      const schedule = this.fluctlightDetail?.schedule as Record<string, unknown> | null | undefined;
	  const context = this.fluctlightDetail?.context as Record<string, unknown> | null | undefined;
	  const expectedLifeContextRevision = String(context?.context_revision ?? "");
	  if (!fluctlightId || !schedule?.id || typeof schedule.revision !== "number" || !expectedLifeContextRevision) return;
	  const commandIdentity = `schedule:cancel:${fluctlightId}:${String(schedule.id)}:${schedule.revision}:${expectedLifeContextRevision}`;
	  const idempotencyKey = this.lifeCommandKey(commandIdentity);
      this.saving = true;
	  try { await client.cancelLifeSchedule(fluctlightId, String(schedule.id), { expectedRevision: schedule.revision, expectedLifeContextRevision, idempotencyKey }); this.clearLifeCommandKey(commandIdentity); await this.loadFluctlightDetail(fluctlightId); }
      catch { this.error = "无法取消日程，当前版本可能已变化。"; }
      finally { this.saving = false; }
    },
    async cancelLifeEvent(fluctlightId: string | null, eventId: string) {
      if (!fluctlightId) return;
	  const context = this.fluctlightDetail?.context as Record<string, unknown> | null | undefined;
	  const event = (this.fluctlightDetail?.events as Array<Record<string, unknown>> | undefined)?.find((item) => String(item.id) === eventId);
	  const expectedLifeContextRevision = String(context?.context_revision ?? "");
	  const expectedEventRevision = Number(event?.revision ?? 0);
	  if (!expectedLifeContextRevision || expectedEventRevision < 1) { this.error = "Event revision 已变化，请刷新后重试。"; return; }
	  const commandIdentity = `event:cancel:${fluctlightId}:${eventId}:${expectedEventRevision}:${expectedLifeContextRevision}`;
	  const idempotencyKey = this.lifeCommandKey(commandIdentity);
      this.saving = true;
	  try { await client.cancelLifeEvent(fluctlightId, eventId, { expectedEventRevision, expectedLifeContextRevision, idempotencyKey }); this.clearLifeCommandKey(commandIdentity); await this.loadFluctlightDetail(fluctlightId); }
      catch { this.error = "无法取消 Event。"; }
      finally { this.saving = false; }
    },
    async saveOperationalSettings() {
      let autonomy: Record<string, unknown>;
      let wakeUp: Record<string, unknown>;
      let retention: Record<string, unknown>;
      try {
        autonomy = JSON.parse(this.autonomySettingsJson) as Record<string, unknown>;
        wakeUp = JSON.parse(this.wakeUpSettingsJson) as Record<string, unknown>;
        retention = JSON.parse(this.diagnosticsRetentionJson) as Record<string, unknown>;
        if (!autonomy || Array.isArray(autonomy) || !wakeUp || Array.isArray(wakeUp) || !retention || Array.isArray(retention)) throw new Error("invalid_settings");
      } catch { this.error = "自治、定期唤醒和诊断保留策略必须是 JSON 对象。"; return; }
      await this.saveSettings({ "product.autonomy": autonomy, "product.wakeup": wakeUp, "diagnostics.retention": retention });
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
        this.diagnosticMediaPrompts = [];
        this.lifecycleDiagnostics = [];
        this.workflowIntentSnapshots = [];
        this.diagnosticsSourceEpochs = { lifecycle: "", events: "", modelRuns: "", mediaPrompts: "" };
        this.diagnosticsNotice = "诊断记录已清空。";
      } catch {
        this.error = "无法清空诊断信息。";
      } finally { this.saving = false; }
    },
    async retryMediaPrompt(mediaIntentId: string) {
      const intentId = mediaIntentId.trim();
      if (!intentId) return;
      this.saving = true;
      this.error = "";
      try {
        await client.retryDiagnosticMediaPrompt(intentId);
        this.diagnosticsNotice = "媒体生成已重新排队。";
        await this.loadDiagnostics();
      } catch (error) {
        this.error = mediaRetryFailureMessage(error);
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
      contextWindowTokens?: number;
      maxInputTokens?: number;
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
          ...(input.contextWindowTokens ? { contextWindowTokens: input.contextWindowTokens } : {}),
          ...(input.maxInputTokens ? { maxInputTokens: input.maxInputTokens } : {}),
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
    case "provider_prompt_budget_invalid":
      return "Token 预算超出当前上下文窗口限制，请调小 Token 预算或调整上下文窗口。";
    case "prompt_budget_policy_unknown":
      return "未知的 Prompt 预算策略版本。";
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
  if (error.code === "initialization_response_semantic_empty") return "初始化模型没有提取出角色卡中的有效语义，请查看本次失败诊断后重试。";
  if (error.code === "initialization_persona_invalid" || error.code === "initialization_foundation_invalid") {
    const detail = error.details.validation_error;
    return typeof detail === "string"
      ? `初始化模型返回的 Persona 分层不符合要求：${detail}`
	  : "初始化模型返回的 Persona 分层结构不符合要求，请查看本次失败诊断中的校验类型与路径。";
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
  if (error.code === "initialization_provider_timeout") return "复杂角色卡分析超过了初始化专用时限，请从本次诊断继续排查 Provider 性能后重试。";
  if (error.code === "initialization_provider_cancelled") return "初始化模型请求已取消，请重新分析。";
  if (error.code === "initialization_provider_unavailable") return "初始化模型 Provider 当前不可用，请检查连接和模型状态。";
  return error.userMessage || "Fluctlight 分析失败。";
}

function creationActivationFailureMessage(error: unknown): string {
  if (!(error instanceof BrowserApiError)) return "Fluctlight 激活服务暂时不可用。";
  if (error.code === "unauthenticated") return "登录会话已失效，请重新登录后再激活。";
  const cause = typeof error.details?.cause === "string" && error.details.cause.trim() ? error.details.cause.trim() : "";
  if (error.code === "activation_persona_invalid" || error.code === "activation_foundation_invalid") {
    const validationError = error.details?.validation_error as Record<string, unknown> | undefined;
    if (validationError && (validationError.path || validationError.type)) {
      const parts = [validationError.path, validationError.type].filter(Boolean);
      return `预览中的 Persona 分层结构无效（${parts.join(" · ")}）。`;
    }
    if (cause) {
      return `预览中的 Persona 分层结构无效（${cause}）。`;
    }
    return "预览中的 Persona 分层结构无效。";
  }
  if (error.code === "activation_analysis_required") return "当前预览缺少分析身份，请重新分析后再激活。";
  if (error.code === "activation_analysis_invalid") return "当前预览的分析身份无效，请重新分析后再激活。";
  if (error.code === "activation_analysis_stale") return "当前预览已被更新的分析取代，请使用最新预览激活。";
  if (error.code === "activation_analysis_conflict") return "该分析结果已经绑定到另一个激活请求，请重新分析。";
  if (error.code === "activation_request_conflict") return "该激活请求已被不同的预览内容占用。";
  if (error.code === "activation_persistence_failed") {
    return cause ? `Fluctlight 数据无法保存（${cause}）。` : "Fluctlight 数据无法保存，请查看诊断信息。";
  }
  if (cause) {
    return `${error.userMessage || "Fluctlight 激活失败"}（${cause}）。`;
  }
  return error.userMessage || "Fluctlight 激活失败。";
}

function diagnosticsFailureMessage(error: unknown): string {
  if (error instanceof BrowserApiError) {
    if (error.status === 401) return "登录状态已失效，请重新登录后查看诊断。";
    if (error.status === 403) return "诊断信息仅对所有者可用。";
    const correlationId = typeof error.details.correlation_id === "string" ? error.details.correlation_id : "";
    const diagnosticIdentity = [error.code, correlationId].filter(Boolean).join(" · ");
    if (error.status >= 500) return `诊断运行时暂时不可用，请确认 Core、数据库和 Worker 正在运行。${diagnosticIdentity ? `（${diagnosticIdentity}）` : ""}`;
    return `无法读取诊断信息：${error.userMessage}${diagnosticIdentity ? `（${diagnosticIdentity}）` : ""}`;
  }
  return "无法读取诊断信息，请确认 API 与 Worker 均在运行。";
}

function mediaRetryFailureMessage(error: unknown): string {
  if (!(error instanceof BrowserApiError)) return "媒体生成重试失败，请稍后重试。";
  if (error.code === "unauthenticated") return "登录状态已失效，请重新登录后重试。";
  if (error.status === 409) return "媒体生成仍在运行中，暂时不能重复重试。";
  const reason = error.details.reason;
  if (typeof reason === "string" && reason.trim()) return `媒体生成重试失败：${reason.trim()}`;
  return error.userMessage || "媒体生成重试失败，请稍后重试。";
}
