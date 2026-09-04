import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const schema = JSON.parse(await readFile(new URL("openapi.json", root), "utf8"));
const paths = Object.keys(schema.paths).sort();
const source = `// Generated from packages/browser-client/openapi.json. Do not edit by hand.
export const browserOperations = ${JSON.stringify(paths)} as const;
export type BrowserHealth = { status: string; role: string };
export type BrowserSession = { authenticated: boolean; actorId?: string };
export type BrowserSetupStatus = { setupAvailable: boolean };
export type BrowserSafeSettings = { values: Record<string, unknown>; configuredSecrets: string[] };
export type BrowserDiagnosticEvent = { id: string; eventType: string; severity: string; fluctlightId?: string | null; causationId?: string | null; correlationId: string; payload: Record<string, unknown>; createdAt?: string | null };
export type BrowserDiagnosticModelRun = { id: string; role: string; bindingRole?: string; scenario?: string; priority?: number; queuePendingCount?: number; queuePosition?: number; endpointId?: string | null; modelId: string; prompt: unknown; response?: unknown; status: string; errorCode?: string | null; correlationId: string; createdAt: string; queuedAt?: string; startedAt?: string | null; completedAt?: string | null };
export type BrowserConversation = { id: string; createdByActorId: string; title?: string | null; revision: number; createdAt: string; updatedAt: string };
export type BrowserParticipant = { conversationId: string; actorId: string; role: string; status: string; joinedAt: string; leftAt?: string | null };
export type BrowserMessage = { id: string; conversationId: string; sequence: number; authorActorId: string; kind: string; text: string; attachmentRefs: string[]; createdAt: string };
export type BrowserConversationPage = { conversation: BrowserConversation; participants: BrowserParticipant[]; messages: BrowserMessage[]; nextBeforeSequence?: number | null };
export type BrowserTurnEvent = { type: "token" | "message" | "media" | "completed" | "error" | "heartbeat"; turnId: string; sequence: number; payload: Record<string, unknown> };

export class BrowserApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly userMessage: string;
  readonly details: Record<string, unknown>;

  constructor(status: number, code: string, userMessage: string, details: Record<string, unknown> = {}) {
    super(userMessage);
    this.name = "BrowserApiError";
    this.status = status;
    this.code = code;
    this.userMessage = userMessage;
    this.details = details;
  }
}

export class BrowserClient {
  constructor(private readonly baseUrl = "", private readonly fetcher: typeof fetch = globalThis.fetch.bind(globalThis)) {}
  private url(path: string): URL {
    const origin = this.baseUrl || (typeof window !== "undefined" ? window.location.origin : "");
    if (!origin) throw new Error("BrowserClient requires a base URL outside the browser");
    return new URL(path, origin);
  }
  async health(path: "/health/live" | "/health/ready"): Promise<BrowserHealth> { return this.json(path) as Promise<BrowserHealth>; }
  async session(): Promise<BrowserSession> {
    const response = await this.fetcher(this.url("/auth/session"), { credentials: "include" });
    if (response.status === 401) return { authenticated: false };
    if (!response.ok) {
      const payload = await response.json().catch(() => null) as { code?: unknown; message?: unknown; details?: unknown } | null;
      const code = typeof payload?.code === "string" ? payload.code : "browser_request_failed";
      const message = typeof payload?.message === "string" ? payload.message : \`Browser request failed: \${response.status}\`;
      const details = payload?.details && typeof payload.details === "object" && !Array.isArray(payload.details)
        ? payload.details as Record<string, unknown>
        : {};
      throw new BrowserApiError(response.status, code, message, details);
    }
    return response.json() as Promise<BrowserSession>;
  }
  async login(password: string): Promise<BrowserSession> { return this.json("/auth/login", { method: "POST", body: { password } }) as Promise<BrowserSession>; }
  async setup(setupToken: string, password: string): Promise<BrowserSession> { return this.json("/auth/setup", { method: "POST", body: { setupToken, password } }) as Promise<BrowserSession>; }
  async setupStatus(): Promise<BrowserSetupStatus> { return this.json("/auth/setup-status") as Promise<BrowserSetupStatus>; }
  async logout(): Promise<void> { await this.json("/auth/logout", { method: "POST" }); }
  async changePassword(password: string): Promise<void> { await this.json("/auth/password", { method: "POST", body: { password } }); }
  async settings(): Promise<BrowserSafeSettings> { return this.json("/api/settings") as Promise<BrowserSafeSettings>; }
  async updateSettings(body: { values?: Record<string, unknown>; secrets?: Record<string, string | null>; clearSecrets?: string[] }): Promise<BrowserSafeSettings> { return this.json("/api/settings", { method: "PUT", body }) as Promise<BrowserSafeSettings>; }
  async listCapabilityRequests(): Promise<Array<Record<string, unknown>>> { return this.json("/api/capability-requests") as Promise<Array<Record<string, unknown>>>; }
  async reviewCapabilityRequest(requestId: string, body: { status: "reviewing" | "accepted" | "rejected" | "fulfilled" | "cancelled"; note?: string; capabilityVersion?: string }): Promise<Record<string, unknown>> { return this.json(\`/api/capability-requests/\${encodeURIComponent(requestId)}/review\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async configureProviderEndpoint(body: { endpointId: string; kind: string; baseUrl: string; secretPurpose: string }): Promise<void> { await this.json("/api/providers/endpoints", { method: "PUT", body }); }
  async providerEndpoints(): Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; capability_status: string; roles: Array<{ role: string; model_id: string }> }>> { return this.json("/api/providers/endpoints") as Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; capability_status: string; roles: Array<{ role: string; model_id: string }> }>>; }
  async providerEndpointModels(endpointId: string): Promise<{ endpointId: string; models: string[] }> { return this.json(\`/api/providers/endpoints/\${encodeURIComponent(endpointId)}/models\`) as Promise<{ endpointId: string; models: string[] }>; }
  async providerBindings(): Promise<Array<{ role: string; endpoint_id: string; model_id: string; token_budget: number; timeout_seconds: number; endpoint_status: string }>> { return this.json("/api/providers") as Promise<Array<{ role: string; endpoint_id: string; model_id: string; token_budget: number; timeout_seconds: number; endpoint_status: string }>>; }
  async configureModelRole(body: { role: string; endpointId: string; modelId: string; tokenBudget: number; timeoutSeconds: number }): Promise<Record<string, unknown>> { return this.json("/api/providers/roles", { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async createConversation(body: { title?: string; participantActorIds: string[] }): Promise<BrowserConversationPage> { return this.json("/api/conversations", { method: "POST", body }) as Promise<BrowserConversationPage>; }
  async createFluctlight(body: { id?: string; name?: string }): Promise<{ id: string; identity: Record<string, unknown>; status: string }> { return this.json("/api/fluctlights", { method: "POST", body }) as Promise<{ id: string; identity: Record<string, unknown>; status: string }>; }
  async analyzeFluctlightCreation(description: string): Promise<Record<string, unknown>> { return this.json("/api/fluctlight-creations/analysis", { method: "POST", body: { description } }) as Promise<Record<string, unknown>>; }
  async activateFluctlightCreation(body: { requestId: string; initializationMode: "blank_slate" | "llm_defined"; name?: string; corePersona?: Record<string, unknown>; developingSelf?: Record<string, unknown>; initialGoals?: Array<Record<string, unknown>>; initialIntentions?: Array<Record<string, unknown>> }): Promise<{ id: string; core_persona?: Record<string, unknown>; identity: Record<string, unknown>; status: string }> { return this.json("/api/fluctlight-creations/activate", { method: "POST", body }) as Promise<{ id: string; core_persona?: Record<string, unknown>; identity: Record<string, unknown>; status: string }>; }
  async listFluctlights(): Promise<Array<{ id: string; identity: Record<string, unknown>; status: string; unread_count?: number; last_conversation_at?: string | null }>> { return this.json("/api/fluctlights") as Promise<Array<{ id: string; identity: Record<string, unknown>; status: string; unread_count?: number; last_conversation_at?: string | null }>>; }
  async listActorGroups(): Promise<Array<{ id: string; name: string; actor_ids?: string[]; members?: string[] }>> { return this.json("/api/actor-groups") as Promise<Array<{ id: string; name: string; actor_ids?: string[]; members?: string[] }>>; }
  async createActorGroup(name: string): Promise<{ id: string; name: string; actor_ids?: string[]; members?: string[] }> { return this.json("/api/actor-groups", { method: "POST", body: { name } }) as Promise<{ id: string; name: string; actor_ids?: string[]; members?: string[] }>; }
  async assignActorGroupMember(groupId: string, actorId: string): Promise<void> { await this.json(\`/api/actor-groups/\${encodeURIComponent(groupId)}/members\`, { method: "POST", body: { actorId } }); }
  async removeActorGroupMember(groupId: string, actorId: string): Promise<void> { await this.json(\`/api/actor-groups/\${encodeURIComponent(groupId)}/members/\${encodeURIComponent(actorId)}\`, { method: "DELETE", body: {} }); }
  async getFluctlight(fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}\`) as Promise<Record<string, unknown>>; }
  async detail(fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/detail\`) as Promise<Record<string, unknown>>; }
  async developingSelf(fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/developing-self\`) as Promise<Record<string, unknown>>; }
  async rollbackDevelopingSelf(fluctlightId: string, claimId: string, body: { expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/developing-self/\${encodeURIComponent(claimId)}/rollback\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async forgetDevelopingSelf(fluctlightId: string, claimId: string, body: { expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/developing-self/\${encodeURIComponent(claimId)}/forget\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async setStatus(fluctlightId: string, body: { status: "active" | "paused"; expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/status\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async retireFluctlight(fluctlightId: string, body: { expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/retire\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async submitFoundationRevision(fluctlightId: string, body: { changes: Record<string, unknown>; expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async acceptFoundationRevision(fluctlightId: string, revisionId: string, body: { expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/\${encodeURIComponent(revisionId)}/accept\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async rejectFoundationRevision(fluctlightId: string, revisionId: string, body: { expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/\${encodeURIComponent(revisionId)}/reject\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async rollbackFoundationRevision(fluctlightId: string, body: { targetRevision: number; expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/rollback\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async directConversation(fluctlightId: string): Promise<BrowserConversationPage> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/conversation\`) as Promise<BrowserConversationPage>; }
  async moments(fluctlightId: string, includeHidden = false): Promise<Array<{ id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null }>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/moments?includeHidden=\${includeHidden}\`) as Promise<Array<{ id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null }>>; }
  async globalMoments(includeHidden = false): Promise<Array<{ id: string; owner_fluctlight_id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null; unread_count?: number }>> { return this.json(\`/api/moments?includeHidden=\${includeHidden}\`) as Promise<Array<{ id: string; owner_fluctlight_id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null; unread_count?: number }>>; }
  async markMomentsRead(fluctlightId: string): Promise<void> { await this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/moments/read\`, { method: "POST", body: {} }); }
  async reviseMemory(memoryId: string, body: { expectedRevision: number; content: string; evidenceRefs: string[] }): Promise<Record<string, unknown>> { return this.json(\`/api/memories/\${encodeURIComponent(memoryId)}\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async forgetMemory(memoryId: string, body: { expectedRevision: number; evidenceRefs: string[] }): Promise<Record<string, unknown>> { return this.json(\`/api/memories/\${encodeURIComponent(memoryId)}/forget\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async rollbackRelationship(fluctlightId: string, body: { targetActorId: string; targetRevision: number; expectedRevision: number; evidenceRefs: string[] }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/relationships/rollback\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async listAutonomyActions(fluctlightId: string): Promise<Array<{ id: string; action_type: string; status: string; workflow_id: string; created_at: string }>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/autonomy-actions\`) as Promise<Array<{ id: string; action_type: string; status: string; workflow_id: string; created_at: string }>>; }
  async governAutonomyAction(actionId: string, body: { status: "paused" | "deferred" | "cancelled"; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/autonomy-actions/\${encodeURIComponent(actionId)}/govern\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async createLifeEvent(fluctlightId: string, body: { kind: string; startAt: string; endAt: string; scene?: string; activity?: string; location?: string; evidenceRefs: string[] }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/events\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async cancelLifeEvent(fluctlightId: string, eventId: string): Promise<void> { await this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/events/\${encodeURIComponent(eventId)}/cancel\`, { method: "POST", body: {} }); }
  async setLifePresence(fluctlightId: string, body: { currentTask?: string; userPresence?: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/presence\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async acceptLifeSchedule(fluctlightId: string, body: { localDate: string; timezone: string; items: Array<{ startAt: string; endAt: string; activity: string; scene: string; itemType?: string; status?: string; priority?: number; flexibility?: number; interruptionCost?: number }>; evidenceRefs: string[]; expectedRevision?: number; completedBefore?: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/schedules\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async cancelLifeSchedule(fluctlightId: string, scheduleId: string, expectedRevision: number): Promise<void> { await this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/schedules/\${encodeURIComponent(scheduleId)}/cancel\`, { method: "POST", body: { expectedRevision } }); }
  async listWorkflows(query = ""): Promise<Array<Record<string, unknown>>> { return this.json(\`/api/diagnostics/workflows?query=\${encodeURIComponent(query)}\`) as Promise<Array<Record<string, unknown>>>; }
  async workflowStatus(workflowId: string): Promise<Record<string, unknown>> { return this.json(\`/api/diagnostics/workflows/\${encodeURIComponent(workflowId)}/status\`) as Promise<Record<string, unknown>>; }
  async workflowHistory(workflowId: string): Promise<Record<string, unknown>> { return this.json(\`/api/diagnostics/workflows/\${encodeURIComponent(workflowId)}/history\`) as Promise<Record<string, unknown>>; }
  async workflowCommand(workflowId: string, action: "pause" | "resume" | "cancel"): Promise<void> { await this.json(\`/api/diagnostics/workflows/\${encodeURIComponent(workflowId)}/\${action}\`, { method: "POST", body: {} }); }
  async resetWorkflow(workflowId: string, historyPoint: number): Promise<Record<string, unknown>> { return this.json(\`/api/diagnostics/workflows/\${encodeURIComponent(workflowId)}/reset\`, { method: "POST", body: { historyPoint } }) as Promise<Record<string, unknown>>; }
  async restartWorkflow(workflowId: string): Promise<Record<string, unknown>> { return this.json(\`/api/diagnostics/workflows/\${encodeURIComponent(workflowId)}/restart\`, { method: "POST", body: {} }) as Promise<Record<string, unknown>>; }
  async commentOnMoment(momentId: string, text: string): Promise<Record<string, unknown>> { return this.json(\`/api/moments/\${encodeURIComponent(momentId)}/comments\`, { method: "POST", body: { text } }) as Promise<Record<string, unknown>>; }
  async reactToMoment(momentId: string, kind: "like" | "care" | "celebrate" = "like"): Promise<Record<string, unknown>> { return this.json(\`/api/moments/\${encodeURIComponent(momentId)}/reactions\`, { method: "POST", body: { kind } }) as Promise<Record<string, unknown>>; }
  async setMomentStatus(momentId: string, action: "hide" | "restore"): Promise<void> { await this.json(\`/api/moments/\${encodeURIComponent(momentId)}/\${action}\`, { method: "POST", body: {} }); }
  async messages(conversationId: string, beforeSequence?: number, limit = 50): Promise<BrowserConversationPage> {
    const query = new URLSearchParams({ limit: String(limit) });
    if (beforeSequence !== undefined) query.set("beforeSequence", String(beforeSequence));
    return this.json(\`/api/conversations/\${encodeURIComponent(conversationId)}/messages?\${query}\`) as Promise<BrowserConversationPage>;
  }
  async markRead(conversationId: string, body: { readSequence: number; deliveredSequence?: number }): Promise<void> {
    await this.json(\`/api/conversations/\${encodeURIComponent(conversationId)}/read\`, { method: "POST", body });
  }
  async turn(conversationId: string, body: { text: string; fluctlightId: string; attachmentRefs?: string[]; idempotencyKey: string; turnId?: string }, signal?: AbortSignal): Promise<Response> {
    const response = await this.fetcher(this.url(\`/api/conversations/\${encodeURIComponent(conversationId)}/turn\`), {
      method: "POST", credentials: "include", headers: { "content-type": "application/json", accept: "application/x-ndjson", ...this.csrfHeaders() }, body: JSON.stringify(body), signal,
    });
    if (!response.ok) throw new Error(\`Browser conversation turn failed: \${response.status}\`);
    return response;
  }
  async diagnostics(options: { limit?: number; correlationId?: string; fluctlightId?: string } = {}): Promise<BrowserDiagnosticEvent[]> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlationId) query.set("correlationId", options.correlationId);
    if (options.fluctlightId) query.set("fluctlightId", options.fluctlightId);
    return this.json(\`/api/diagnostics?\${query}\`) as Promise<BrowserDiagnosticEvent[]>;
  }
  async diagnosticModelRuns(options: { limit?: number; correlationId?: string } = {}): Promise<BrowserDiagnosticModelRun[]> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlationId) query.set("correlationId", options.correlationId);
    return this.json(\`/api/diagnostics/model-runs?\${query}\`) as Promise<BrowserDiagnosticModelRun[]>;
  }
  async exportDiagnostics(options: { limit?: number; correlationId?: string } = {}): Promise<Record<string, unknown>> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 500) });
    if (options.correlationId) query.set("correlationId", options.correlationId);
    return this.json(\`/api/diagnostics/export?\${query}\`) as Promise<Record<string, unknown>>;
  }
  async clearDiagnostics(): Promise<number> {
    const result = await this.json("/api/diagnostics", { method: "DELETE" }) as { cleared?: number };
    return Number(result.cleared ?? 0);
  }
  async media(assetId: string, range?: string, signal?: AbortSignal): Promise<Response> {
    const headers: Record<string, string> = {};
    if (range) headers.Range = range;
    const response = await this.fetcher(this.url(\`/api/media/\${encodeURIComponent(assetId)}\`), { headers, credentials: "include", signal });
    if (!response.ok) throw new Error(\`Browser media request failed: \${response.status}\`);
    return response;
  }
  private csrfHeaders(): Record<string, string> {
    if (typeof document === "undefined") return {};
    const prefix = "fluctlight_csrf=";
    const token = document.cookie.split(";").map((part) => part.trim()).find((part) => part.startsWith(prefix))?.slice(prefix.length);
    return token ? { "x-csrf-token": decodeURIComponent(token) } : {};
  }
  private async json(path: string, options: { method?: string; body?: unknown } = {}): Promise<unknown> {
    const method = options.method ?? "GET";
    const response = await this.fetcher(this.url(path), {
      method, credentials: "include",
      headers: { ...(options.body ? { "content-type": "application/json" } : {}), ...(method !== "GET" ? this.csrfHeaders() : {}) },
      body: options.body ? JSON.stringify(options.body) : undefined,
    });
    if (!response.ok) {
      const payload = await response.json().catch(() => null) as { code?: unknown; message?: unknown; details?: unknown } | null;
      const code = typeof payload?.code === "string" ? payload.code : "browser_request_failed";
      const message = typeof payload?.message === "string" ? payload.message : \`Browser request failed: \${response.status}\`;
      const details = payload?.details && typeof payload.details === "object" && !Array.isArray(payload.details)
        ? payload.details as Record<string, unknown>
        : {};
      throw new BrowserApiError(response.status, code, message, details);
    }
    if (response.status === 204) return undefined;
    return response.json();
  }
}
`;
await writeFile(fileURLToPath(new URL("src/index.ts", root)), source);
