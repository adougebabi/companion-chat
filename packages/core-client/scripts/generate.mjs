import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const schema = JSON.parse(await readFile(new URL("openapi.json", root), "utf8"));
const operations = Object.keys(schema.paths).sort();
const source = `// Generated from packages/core-client/openapi.json. Do not edit by hand.
export const coreOperations = ${JSON.stringify(operations)} as const;

export type CoreHealth = { status: string; role: string };
export type CoreSession = { authenticated: boolean; actorId?: string };
export type CoreAuthenticatedSession = CoreSession & { sessionToken: string };
export type CoreSafeSettings = { values: Record<string, unknown>; configuredSecrets: string[] };
export type CoreProviderPreflight = { role: string; available: boolean; capability_version?: string };
export type CoreConversation = { id: string; created_by_actor_id: string; title?: string | null; revision: number; created_at: string; updated_at: string };
export type CoreParticipant = { conversation_id: string; actor_id: string; role: string; status: string; joined_at: string; left_at?: string | null };
export type CoreMessage = { id: string; conversation_id: string; sequence: number; author_actor_id: string; kind: string; text: string; attachment_refs: string[]; created_at: string };
export type CoreConversationPage = { conversation: CoreConversation; participants: CoreParticipant[]; messages: CoreMessage[]; next_before_sequence?: number | null };
export type CoreConversationCreate = { title?: string; participant_actor_ids?: string[] };
export type CoreConversationTurn = { text: string; fluctlight_id?: string; attachment_refs?: string[]; idempotency_key: string; turn_id?: string };
export type CoreFluctlight = { id: string; identity: Record<string, unknown>; status: string };
export type CoreDiagnosticEvent = { id: string; event_type: string; severity: string; fluctlight_id?: string | null; causation_id?: string | null; correlation_id: string; payload: Record<string, unknown>; created_at?: string | null };

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
    if (!response.ok) throw new Error(\`Core health request failed: \${response.status}\`);
    return response.json() as Promise<CoreHealth>;
  }
  async ping(): Promise<CoreHealth> {
    const response = await this.fetcher(new URL("/internal/platform/ping", this.baseUrl), { headers: { "x-fluctlight-service-key": this.serviceKey } });
    if (!response.ok) throw new Error(\`Core ping failed: \${response.status}\`);
    return response.json() as Promise<CoreHealth>;
  }
  async session(humanSession: string | undefined): Promise<CoreSession> {
    const headers: Record<string, string> = { "x-fluctlight-service-key": this.serviceKey };
    if (humanSession) headers["x-fluctlight-human-session"] = humanSession;
    const response = await this.fetcher(new URL("/internal/auth/session", this.baseUrl), { headers });
    if (!response.ok) throw new Error(\`Core session request failed: \${response.status}\`);
    return this.mapSession(await response.json());
  }
  async setup(setupToken: string, password: string): Promise<CoreAuthenticatedSession> { return this.authenticate("/internal/auth/setup", { setup_token: setupToken, password }); }
  async login(password: string): Promise<CoreAuthenticatedSession> { return this.authenticate("/internal/auth/login", { password }); }
  async revokeAll(humanSession: string): Promise<void> {
    const response = await this.fetcher(new URL("/internal/auth/revoke-all", this.baseUrl), { method: "POST", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!response.ok) throw new Error(\`Core revoke-all request failed: \${response.status}\`);
  }
  async revokeCurrent(humanSession: string): Promise<void> {
    const response = await this.fetcher(new URL("/internal/auth/revoke-current", this.baseUrl), { method: "POST", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!response.ok) throw new Error(\`Core revoke-current request failed: \${response.status}\`);
  }
  async resetPassword(humanSession: string, password: string): Promise<void> {
    const response = await this.fetcher(new URL("/internal/auth/reset-password", this.baseUrl), { method: "POST", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession, "content-type": "application/json" }, body: JSON.stringify({ password }) });
    if (!response.ok) throw new Error(\`Core reset-password request failed: \${response.status}\`);
  }
  async readSettings(humanSession: string): Promise<CoreSafeSettings> { return this.settings("GET", humanSession); }
  async updateSettings(humanSession: string, patch: object): Promise<CoreSafeSettings> { return this.settings("PUT", humanSession, patch); }
  async configureProviderEndpoint(humanSession: string, endpoint: object): Promise<void> { await this.provider("/internal/providers/endpoints", humanSession, endpoint); }
  async configureModelRole(humanSession: string, role: object): Promise<CoreProviderPreflight> { return this.provider("/internal/providers/roles", humanSession, role) as Promise<CoreProviderPreflight>; }
  async createConversation(humanSession: string, body: CoreConversationCreate): Promise<CoreConversationPage> { return this.json("/internal/conversations", humanSession, "POST", body) as Promise<CoreConversationPage>; }
  async createFluctlight(humanSession: string, body: { id?: string; name?: string }): Promise<CoreFluctlight> { return this.json("/internal/fluctlights", humanSession, "POST", body) as Promise<CoreFluctlight>; }
  async listFluctlights(humanSession: string): Promise<CoreFluctlight[]> { return this.json("/internal/fluctlights", humanSession, "GET") as Promise<CoreFluctlight[]>; }
  async getFluctlight(humanSession: string, fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}\`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async conversationHistory(humanSession: string, conversationId: string, beforeSequence?: number, limit = 50): Promise<CoreConversationPage> {
    const query = new URLSearchParams({ limit: String(limit) });
    if (beforeSequence !== undefined) query.set("before_sequence", String(beforeSequence));
    return this.json(\`/internal/conversations/\${encodeURIComponent(conversationId)}/history?\${query}\`, humanSession, "GET") as Promise<CoreConversationPage>;
  }
  async markConversationRead(humanSession: string, conversationId: string, body: { read_sequence: number; delivered_sequence?: number }): Promise<void> { await this.json(\`/internal/conversations/\${encodeURIComponent(conversationId)}/read\`, humanSession, "POST", body); }
  async acceptConversationTurn(humanSession: string, conversationId: string, body: CoreConversationTurn, signal?: AbortSignal): Promise<Response> {
    const response = await this.fetcher(new URL(\`/internal/conversations/\${encodeURIComponent(conversationId)}/turn\`, this.baseUrl), { method: "POST", headers: { "content-type": "application/json", accept: "application/x-ndjson", "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession }, body: JSON.stringify(body), signal });
    if (!response.ok) throw new Error(\`Core conversation turn failed: \${response.status}\`);
    return response;
  }
  async readDiagnostics(humanSession: string, options: { limit?: number; correlation_id?: string; fluctlight_id?: string } = {}): Promise<CoreDiagnosticEvent[]> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    if (options.fluctlight_id) query.set("fluctlight_id", options.fluctlight_id);
    const rows = await this.json(\`/internal/diagnostics?\${query}\`, humanSession, "GET") as Array<Record<string, unknown>>;
    return rows.map((row) => ({ id: String(row.id), event_type: String(row.event_type), severity: String(row.severity), fluctlight_id: row.fluctlight_id as string | null | undefined, causation_id: row.causation_id as string | null | undefined, correlation_id: String(row.correlation_id), payload: (row.payload ?? {}) as Record<string, unknown>, created_at: row.created_at as string | null | undefined }));
  }
  async clearDiagnostics(humanSession: string): Promise<number> {
    const result = await this.fetcher(new URL("/internal/diagnostics", this.baseUrl), { method: "DELETE", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!result.ok) throw new Error(\`Core diagnostics clear failed: \${result.status}\`);
    return Number((await result.json() as { cleared?: number }).cleared ?? 0);
  }
  async readMedia(humanSession: string, assetId: string, range?: string, signal?: AbortSignal): Promise<Response> {
    const headers: Record<string, string> = { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession };
    if (range) headers.Range = range;
    const response = await this.fetcher(new URL(\`/internal/media/\${encodeURIComponent(assetId)}\`, this.baseUrl), { headers, signal });
    if (!response.ok) throw new Error(\`Core media request failed: \${response.status}\`);
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
    if (!response.ok) throw new Error(\`Core request failed: \${response.status}\`);
    if (response.status === 204) return undefined;
    return response.json();
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
`;
await writeFile(fileURLToPath(new URL("src/index.ts", root)), source);
