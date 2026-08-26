// Generated from packages/core-client/openapi.json. Do not edit by hand.
export const coreOperations = ["/health/live","/health/ready","/internal/actor-groups","/internal/actor-groups/{group_id}/members","/internal/actor-groups/{group_id}/members/{actor_id}","/internal/auth/login","/internal/auth/reset-password","/internal/auth/revoke-all","/internal/auth/revoke-current","/internal/auth/session","/internal/auth/setup","/internal/auth/setup-status","/internal/autonomy-actions/{action_id}/govern","/internal/conversations","/internal/conversations/{conversation_id}/history","/internal/conversations/{conversation_id}/read","/internal/conversations/{conversation_id}/turn","/internal/diagnostics","/internal/diagnostics/export","/internal/diagnostics/model-runs","/internal/diagnostics/workflows","/internal/diagnostics/workflows/{workflow_id}/cancel","/internal/diagnostics/workflows/{workflow_id}/history","/internal/diagnostics/workflows/{workflow_id}/pause","/internal/diagnostics/workflows/{workflow_id}/reset","/internal/diagnostics/workflows/{workflow_id}/restart","/internal/diagnostics/workflows/{workflow_id}/resume","/internal/diagnostics/workflows/{workflow_id}/status","/internal/fluctlight-creations/activate","/internal/fluctlight-creations/analysis","/internal/fluctlights","/internal/fluctlights/{fluctlight_id}","/internal/fluctlights/{fluctlight_id}/autonomy-actions","/internal/fluctlights/{fluctlight_id}/conversation","/internal/fluctlights/{fluctlight_id}/detail","/internal/fluctlights/{fluctlight_id}/events","/internal/fluctlights/{fluctlight_id}/events/{event_id}/cancel","/internal/fluctlights/{fluctlight_id}/foundation-revisions","/internal/fluctlights/{fluctlight_id}/foundation-revisions/rollback","/internal/fluctlights/{fluctlight_id}/foundation-revisions/{revision_id}/accept","/internal/fluctlights/{fluctlight_id}/foundation-revisions/{revision_id}/reject","/internal/fluctlights/{fluctlight_id}/moments","/internal/fluctlights/{fluctlight_id}/moments/read","/internal/fluctlights/{fluctlight_id}/presence","/internal/fluctlights/{fluctlight_id}/relationships/rollback","/internal/fluctlights/{fluctlight_id}/schedules","/internal/fluctlights/{fluctlight_id}/schedules/{schedule_id}/cancel","/internal/fluctlights/{fluctlight_id}/status","/internal/media/{asset_id}","/internal/memories/{memory_id}","/internal/memories/{memory_id}/forget","/internal/moments","/internal/moments/{moment_id}/comments","/internal/moments/{moment_id}/hide","/internal/moments/{moment_id}/reactions","/internal/moments/{moment_id}/restore","/internal/platform/ping","/internal/providers","/internal/providers/endpoints","/internal/providers/endpoints/{endpoint_id}/models","/internal/providers/roles","/internal/settings"] as const;

export type CoreHealth = { status: string; role: string };
export type CoreSession = { authenticated: boolean; actorId?: string };
export type CoreSetupStatus = { setupAvailable: boolean };
export type CoreAuthenticatedSession = CoreSession & { sessionToken: string };
export type CoreSafeSettings = { values: Record<string, unknown>; configuredSecrets: string[] };
export type CoreProviderPreflight = { role: string; available: boolean; capability_version?: string };
export type CoreProviderModels = { endpoint_id: string; models: string[] };
export type CoreConversation = { id: string; created_by_actor_id: string; title?: string | null; revision: number; created_at: string; updated_at: string };
export type CoreParticipant = { conversation_id: string; actor_id: string; role: string; status: string; joined_at: string; left_at?: string | null };
export type CoreMessage = { id: string; conversation_id: string; sequence: number; author_actor_id: string; kind: string; text: string; attachment_refs: string[]; created_at: string };
export type CoreConversationPage = { conversation: CoreConversation; participants: CoreParticipant[]; messages: CoreMessage[]; next_before_sequence?: number | null };
export type CoreConversationCreate = { title?: string; participant_actor_ids: string[] };
export type CoreConversationTurn = { text: string; fluctlight_id: string; attachment_refs?: string[]; idempotency_key: string; turn_id?: string };
export type CoreFluctlight = { id: string; identity: Record<string, unknown>; status: string };
export type CoreDiagnosticEvent = { id: string; event_type: string; severity: string; fluctlight_id?: string | null; causation_id?: string | null; correlation_id: string; payload: Record<string, unknown>; created_at?: string | null };
export type CoreDiagnosticModelRun = { id: string; role: string; endpoint_id?: string | null; model_id: string; prompt: Record<string, unknown>; response?: Record<string, unknown> | null; status: string; error_code?: string | null; correlation_id: string; created_at: string };

export class CoreClient {
  private readonly baseUrl: string;
  private readonly serviceKey: string;
  private readonly fetcher: typeof fetch;
  constructor(baseUrl: string, serviceKey: string, fetcher: typeof fetch = fetch) {
    this.baseUrl = baseUrl;
    this.serviceKey = serviceKey;
    this.fetcher = fetcher;
  }
  async health(path: "/health/live" | "/health/ready"): Promise<CoreHealth> {
    const response = await this.fetcher(new URL(path, this.baseUrl), { headers: { "x-fluctlight-service-key": this.serviceKey } });
    if (!response.ok) throw new Error(`Core health request failed: ${response.status}`);
    return response.json() as Promise<CoreHealth>;
  }
  async ping(): Promise<CoreHealth> {
    const response = await this.fetcher(new URL("/internal/platform/ping", this.baseUrl), { headers: { "x-fluctlight-service-key": this.serviceKey } });
    if (!response.ok) throw new Error(`Core ping failed: ${response.status}`);
    return response.json() as Promise<CoreHealth>;
  }
  async session(humanSession: string | undefined): Promise<CoreSession> {
    const headers: Record<string, string> = { "x-fluctlight-service-key": this.serviceKey };
    if (humanSession) headers["x-fluctlight-human-session"] = humanSession;
    const response = await this.fetcher(new URL("/internal/auth/session", this.baseUrl), { headers });
    if (!response.ok) throw new Error(`Core session request failed: ${response.status}`);
    return this.mapSession(await response.json());
  }
  async setupStatus(): Promise<CoreSetupStatus> {
    const value = await this.json("/internal/auth/setup-status", undefined, "GET") as { setup_available?: boolean; setupAvailable?: boolean };
    return { setupAvailable: Boolean(value.setup_available ?? value.setupAvailable) };
  }
  async setup(setupToken: string, password: string): Promise<CoreAuthenticatedSession> { return this.authenticate("/internal/auth/setup", { setup_token: setupToken, password }); }
  async login(password: string): Promise<CoreAuthenticatedSession> { return this.authenticate("/internal/auth/login", { password }); }
  async revokeAll(humanSession: string): Promise<void> {
    const response = await this.fetcher(new URL("/internal/auth/revoke-all", this.baseUrl), { method: "POST", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!response.ok) throw new Error(`Core revoke-all request failed: ${response.status}`);
  }
  async revokeCurrent(humanSession: string): Promise<void> {
    const response = await this.fetcher(new URL("/internal/auth/revoke-current", this.baseUrl), { method: "POST", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!response.ok) throw new Error(`Core revoke-current request failed: ${response.status}`);
  }
  async resetPassword(humanSession: string, password: string): Promise<void> {
    const response = await this.fetcher(new URL("/internal/auth/reset-password", this.baseUrl), { method: "POST", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession, "content-type": "application/json" }, body: JSON.stringify({ password }) });
    if (!response.ok) throw new Error(`Core reset-password request failed: ${response.status}`);
  }
  async readSettings(humanSession: string): Promise<CoreSafeSettings> { return this.settings("GET", humanSession); }
  async updateSettings(humanSession: string, patch: object): Promise<CoreSafeSettings> { return this.settings("PUT", humanSession, patch); }
  async configureProviderEndpoint(humanSession: string, endpoint: object): Promise<void> { await this.provider("/internal/providers/endpoints", humanSession, endpoint); }
  async providerEndpoints(humanSession: string): Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; capability_status: string; roles: Array<{ role: string; model_id: string }> }>> { return this.json("/internal/providers/endpoints", humanSession, "GET") as Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; capability_status: string; roles: Array<{ role: string; model_id: string }> }>>; }
  async providerEndpointModels(humanSession: string, endpointId: string): Promise<CoreProviderModels> { return this.json(`/internal/providers/endpoints/${encodeURIComponent(endpointId)}/models`, humanSession, "GET") as Promise<CoreProviderModels>; }
  async providerBindings(humanSession: string): Promise<Array<Record<string, unknown>>> { return this.json("/internal/providers", humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async configureModelRole(humanSession: string, role: object): Promise<CoreProviderPreflight> { return this.provider("/internal/providers/roles", humanSession, role) as Promise<CoreProviderPreflight>; }
  async createConversation(humanSession: string, body: CoreConversationCreate): Promise<CoreConversationPage> { return this.json("/internal/conversations", humanSession, "POST", body) as Promise<CoreConversationPage>; }
  async createFluctlight(humanSession: string, body: { id?: string; name?: string }): Promise<CoreFluctlight> { return this.json("/internal/fluctlights", humanSession, "POST", body) as Promise<CoreFluctlight>; }
  async analyzeFluctlightCreation(humanSession: string, description: string): Promise<Record<string, unknown>> { return this.json("/internal/fluctlight-creations/analysis", humanSession, "POST", { description }) as Promise<Record<string, unknown>>; }
  async activateFluctlightCreation(humanSession: string, body: object): Promise<Record<string, unknown>> { return this.json("/internal/fluctlight-creations/activate", humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async listFluctlights(humanSession: string): Promise<CoreFluctlight[]> { return this.json("/internal/fluctlights", humanSession, "GET") as Promise<CoreFluctlight[]>; }
  async listActorGroups(humanSession: string): Promise<Array<{ id: string; name: string; actor_ids: string[] }>> { return this.json("/internal/actor-groups", humanSession, "GET") as Promise<Array<{ id: string; name: string; actor_ids: string[] }>>; }
  async createActorGroup(humanSession: string, name: string): Promise<{ id: string; name: string; actor_ids: string[] }> { return this.json("/internal/actor-groups", humanSession, "POST", { name }) as Promise<{ id: string; name: string; actor_ids: string[] }>; }
  async assignActorGroupMember(humanSession: string, groupId: string, actorId: string): Promise<void> { await this.json(`/internal/actor-groups/${encodeURIComponent(groupId)}/members`, humanSession, "POST", { actor_id: actorId }); }
  async removeActorGroupMember(humanSession: string, groupId: string, actorId: string): Promise<void> { await this.delete(`/internal/actor-groups/${encodeURIComponent(groupId)}/members/${encodeURIComponent(actorId)}`, humanSession); }
  async getFluctlight(humanSession: string, fluctlightId: string): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async fluctlightDetail(humanSession: string, fluctlightId: string): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/detail`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async setFluctlightStatus(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/status`, humanSession, "PUT", body) as Promise<Record<string, unknown>>; }
  async submitFoundationRevision(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/foundation-revisions`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async acceptFoundationRevision(humanSession: string, fluctlightId: string, revisionId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/foundation-revisions/${encodeURIComponent(revisionId)}/accept`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async rejectFoundationRevision(humanSession: string, fluctlightId: string, revisionId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/foundation-revisions/${encodeURIComponent(revisionId)}/reject`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async rollbackFoundationRevision(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/foundation-revisions/rollback`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async fluctlightMoments(humanSession: string, fluctlightId: string, includeHidden = false): Promise<Array<Record<string, unknown>>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/moments?include_hidden=${includeHidden}`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async globalMoments(humanSession: string, includeHidden = false): Promise<Array<Record<string, unknown>>> { return this.json(`/internal/moments?include_hidden=${includeHidden}`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async markFluctlightMomentsRead(humanSession: string, fluctlightId: string): Promise<void> { await this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/moments/read`, humanSession, "POST", {}); }
  async reviseMemory(humanSession: string, memoryId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/memories/${encodeURIComponent(memoryId)}`, humanSession, "PUT", body) as Promise<Record<string, unknown>>; }
  async forgetMemory(humanSession: string, memoryId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/memories/${encodeURIComponent(memoryId)}/forget`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async rollbackRelationship(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/relationships/rollback`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async listAutonomyActions(humanSession: string, fluctlightId: string): Promise<Array<Record<string, unknown>>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/autonomy-actions`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async governAutonomyAction(humanSession: string, actionId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/autonomy-actions/${encodeURIComponent(actionId)}/govern`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async createLifeEvent(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/events`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async cancelLifeEvent(humanSession: string, fluctlightId: string, eventId: string): Promise<void> { await this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/events/${encodeURIComponent(eventId)}/cancel`, humanSession, "POST", {}); }
  async setLifePresence(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/presence`, humanSession, "PUT", body) as Promise<Record<string, unknown>>; }
  async acceptLifeSchedule(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/schedules`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async cancelLifeSchedule(humanSession: string, fluctlightId: string, scheduleId: string, expectedRevision: number): Promise<void> { await this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/schedules/${encodeURIComponent(scheduleId)}/cancel`, humanSession, "POST", { expected_revision: expectedRevision }); }
  async listWorkflows(humanSession: string, query = ""): Promise<Array<Record<string, unknown>>> { return this.json(`/internal/diagnostics/workflows?query=${encodeURIComponent(query)}`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async workflowStatus(humanSession: string, workflowId: string): Promise<Record<string, unknown>> { return this.json(`/internal/diagnostics/workflows/${encodeURIComponent(workflowId)}/status`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async workflowHistory(humanSession: string, workflowId: string): Promise<Record<string, unknown>> { return this.json(`/internal/diagnostics/workflows/${encodeURIComponent(workflowId)}/history`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async workflowCommand(humanSession: string, workflowId: string, action: "pause" | "resume" | "cancel"): Promise<void> { await this.json(`/internal/diagnostics/workflows/${encodeURIComponent(workflowId)}/${action}`, humanSession, "POST", {}); }
  async resetWorkflow(humanSession: string, workflowId: string, historyPoint: number): Promise<Record<string, unknown>> { return this.json(`/internal/diagnostics/workflows/${encodeURIComponent(workflowId)}/reset`, humanSession, "POST", { history_point: historyPoint }) as Promise<Record<string, unknown>>; }
  async restartWorkflow(humanSession: string, workflowId: string): Promise<Record<string, unknown>> { return this.json(`/internal/diagnostics/workflows/${encodeURIComponent(workflowId)}/restart`, humanSession, "POST", {}) as Promise<Record<string, unknown>>; }
  async commentOnMoment(humanSession: string, momentId: string, text: string): Promise<Record<string, unknown>> { return this.json(`/internal/moments/${encodeURIComponent(momentId)}/comments`, humanSession, "POST", { text }) as Promise<Record<string, unknown>>; }
  async reactToMoment(humanSession: string, momentId: string, kind: string): Promise<Record<string, unknown>> { return this.json(`/internal/moments/${encodeURIComponent(momentId)}/reactions`, humanSession, "POST", { kind }) as Promise<Record<string, unknown>>; }
  async setMomentStatus(humanSession: string, momentId: string, action: "hide" | "restore"): Promise<void> { await this.json(`/internal/moments/${encodeURIComponent(momentId)}/${action}`, humanSession, "POST", {}); }
  async fluctlightDirectConversation(humanSession: string, fluctlightId: string): Promise<CoreConversationPage> { return this.json(`/internal/fluctlights/${encodeURIComponent(fluctlightId)}/conversation`, humanSession, "GET") as Promise<CoreConversationPage>; }
  async conversationHistory(humanSession: string, conversationId: string, beforeSequence?: number, limit = 50): Promise<CoreConversationPage> {
    const query = new URLSearchParams({ limit: String(limit) });
    if (beforeSequence !== undefined) query.set("before_sequence", String(beforeSequence));
    return this.json(`/internal/conversations/${encodeURIComponent(conversationId)}/history?${query}`, humanSession, "GET") as Promise<CoreConversationPage>;
  }
  async markConversationRead(humanSession: string, conversationId: string, body: { read_sequence: number; delivered_sequence?: number }): Promise<void> { await this.json(`/internal/conversations/${encodeURIComponent(conversationId)}/read`, humanSession, "POST", body); }
  async acceptConversationTurn(humanSession: string, conversationId: string, body: CoreConversationTurn, signal?: AbortSignal): Promise<Response> {
    const response = await this.fetcher(new URL(`/internal/conversations/${encodeURIComponent(conversationId)}/turn`, this.baseUrl), { method: "POST", headers: { "content-type": "application/json", accept: "application/x-ndjson", "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession }, body: JSON.stringify(body), signal });
    if (!response.ok) throw new Error(`Core conversation turn failed: ${response.status}`);
    return response;
  }
  async readDiagnostics(humanSession: string, options: { limit?: number; correlation_id?: string; fluctlight_id?: string } = {}): Promise<CoreDiagnosticEvent[]> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    if (options.fluctlight_id) query.set("fluctlight_id", options.fluctlight_id);
    const rows = await this.json(`/internal/diagnostics?${query}`, humanSession, "GET") as Array<Record<string, unknown>>;
    return rows.map((row) => ({ id: String(row.id), event_type: String(row.event_type), severity: String(row.severity), fluctlight_id: row.fluctlight_id as string | null | undefined, causation_id: row.causation_id as string | null | undefined, correlation_id: String(row.correlation_id), payload: (row.payload ?? {}) as Record<string, unknown>, created_at: row.created_at as string | null | undefined }));
  }
  async readDiagnosticModelRuns(humanSession: string, options: { limit?: number; correlation_id?: string } = {}): Promise<CoreDiagnosticModelRun[]> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    const rows = await this.json(`/internal/diagnostics/model-runs?${query}`, humanSession, "GET") as Array<Record<string, unknown>>;
    return rows.map((row) => ({ id: String(row.id), role: String(row.role), endpoint_id: row.endpoint_id as string | null | undefined, model_id: String(row.model_id), prompt: (row.prompt ?? {}) as Record<string, unknown>, response: row.response as Record<string, unknown> | null | undefined, status: String(row.status), error_code: row.error_code as string | null | undefined, correlation_id: String(row.correlation_id), created_at: String(row.created_at) }));
  }
  async exportDiagnostics(humanSession: string, options: { limit?: number; correlation_id?: string } = {}): Promise<Record<string, unknown>> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 500) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    return this.json(`/internal/diagnostics/export?${query}`, humanSession, "GET") as Promise<Record<string, unknown>>;
  }
  async clearDiagnostics(humanSession: string): Promise<number> {
    const result = await this.fetcher(new URL("/internal/diagnostics", this.baseUrl), { method: "DELETE", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!result.ok) throw new Error(`Core diagnostics clear failed: ${result.status}`);
    return Number((await result.json() as { cleared?: number }).cleared ?? 0);
  }
  async readMedia(humanSession: string, assetId: string, range?: string, signal?: AbortSignal): Promise<Response> {
    const headers: Record<string, string> = { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession };
    if (range) headers.Range = range;
    const response = await this.fetcher(new URL(`/internal/media/${encodeURIComponent(assetId)}`, this.baseUrl), { headers, signal });
    if (!response.ok) throw new Error(`Core media request failed: ${response.status}`);
    return response;
  }
  private async authenticate(path: string, body: object): Promise<CoreAuthenticatedSession> { return this.mapAuthenticated(await this.json(path, undefined, "POST", body)); }
  private async settings(method: "GET" | "PUT", humanSession: string, body?: object): Promise<CoreSafeSettings> { return this.mapSettings(await this.json("/internal/settings", humanSession, method, body)); }
  private async provider(path: string, humanSession: string, body: object): Promise<unknown> { return this.json(path, humanSession, "PUT", body); }
  private async json(path: string, humanSession: string | undefined, method: "GET" | "POST" | "PUT", body?: object): Promise<unknown> {
    const headers: Record<string, string> = { "x-fluctlight-service-key": this.serviceKey };
    if (humanSession) headers["x-fluctlight-human-session"] = humanSession;
    if (body) headers["content-type"] = "application/json";
    const response = await this.fetcher(new URL(path, this.baseUrl), { method, headers, body: body ? JSON.stringify(body) : undefined });
    if (!response.ok) throw new Error(`Core request failed: ${response.status}`);
    if (response.status === 204) return undefined;
    return response.json();
  }
  private async delete(path: string, humanSession: string): Promise<void> {
    const response = await this.fetcher(new URL(path, this.baseUrl), { method: "DELETE", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!response.ok) throw new Error(`Core delete failed: ${response.status}`);
  }
  private mapSession(value: unknown): CoreSession {
    const row = value as { authenticated?: boolean; actorId?: string; actor_id?: string };
    return { authenticated: Boolean(row.authenticated), actorId: row.actorId ?? row.actor_id };
  }
  private mapAuthenticated(value: unknown): CoreAuthenticatedSession {
    const row = value as { authenticated?: boolean; actorId?: string; actor_id?: string; sessionToken?: string; session_token?: string };
    return { authenticated: Boolean(row.authenticated), actorId: row.actorId ?? row.actor_id, sessionToken: row.sessionToken ?? row.session_token ?? "" };
  }
  private mapSettings(value: unknown): CoreSafeSettings {
    const row = value as { values?: Record<string, unknown>; configuredSecrets?: string[]; configured_secrets?: string[] };
    return { values: row.values ?? {}, configuredSecrets: row.configuredSecrets ?? row.configured_secrets ?? [] };
  }
}
