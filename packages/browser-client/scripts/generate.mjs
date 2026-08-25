import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const schema = JSON.parse(await readFile(new URL("openapi.json", root), "utf8"));
const paths = Object.keys(schema.paths).sort();
const source = `// Generated from packages/browser-client/openapi.json. Do not edit by hand.
export const browserOperations = ${JSON.stringify(paths)} as const;
export type BrowserHealth = { status: string; role: string };
export type BrowserSession = { authenticated: boolean; actorId?: string };
export type BrowserSafeSettings = { values: Record<string, unknown>; configuredSecrets: string[] };
export type BrowserDiagnosticEvent = { id: string; eventType: string; severity: string; fluctlightId?: string | null; causationId?: string | null; correlationId: string; payload: Record<string, unknown>; createdAt?: string | null };
export type BrowserConversation = { id: string; createdByActorId: string; title?: string | null; revision: number; createdAt: string; updatedAt: string };
export type BrowserParticipant = { conversationId: string; actorId: string; role: string; status: string; joinedAt: string; leftAt?: string | null };
export type BrowserMessage = { id: string; conversationId: string; sequence: number; authorActorId: string; kind: string; text: string; attachmentRefs: string[]; createdAt: string };
export type BrowserConversationPage = { conversation: BrowserConversation; participants: BrowserParticipant[]; messages: BrowserMessage[]; nextBeforeSequence?: number | null };
export type BrowserTurnEvent = { type: "token" | "message" | "media" | "completed" | "error" | "heartbeat"; turnId: string; sequence: number; payload: Record<string, unknown> };

export class BrowserClient {
  constructor(private readonly baseUrl = "", private readonly fetcher: typeof fetch = globalThis.fetch.bind(globalThis)) {}
  private url(path: string): URL {
    const origin = this.baseUrl || (typeof window !== "undefined" ? window.location.origin : "http://127.0.0.1:13000");
    return new URL(path, origin);
  }
  async health(path: "/health/live" | "/health/ready"): Promise<BrowserHealth> { return this.json(path) as Promise<BrowserHealth>; }
  async session(): Promise<BrowserSession> {
    const response = await this.fetcher(this.url("/auth/session"), { credentials: "include" });
    if (response.status === 401) return { authenticated: false };
    if (!response.ok) throw new Error(\`Browser request failed: \${response.status}\`);
    return response.json() as Promise<BrowserSession>;
  }
  async login(password: string): Promise<BrowserSession> { return this.json("/auth/login", { method: "POST", body: { password } }) as Promise<BrowserSession>; }
  async setup(setupToken: string, password: string): Promise<BrowserSession> { return this.json("/auth/setup", { method: "POST", body: { setupToken, password } }) as Promise<BrowserSession>; }
  async logout(): Promise<void> { await this.json("/auth/logout", { method: "POST" }); }
  async settings(): Promise<BrowserSafeSettings> { return this.json("/api/settings") as Promise<BrowserSafeSettings>; }
  async updateSettings(body: { values?: Record<string, unknown>; secrets?: Record<string, string | null>; clearSecrets?: string[] }): Promise<BrowserSafeSettings> { return this.json("/api/settings", { method: "PUT", body }) as Promise<BrowserSafeSettings>; }
  async configureProviderEndpoint(body: { endpointId: string; kind: string; baseUrl: string; secretPurpose: string }): Promise<void> { await this.json("/api/providers/endpoints", { method: "PUT", body }); }
  async configureModelRole(body: { role: string; endpointId: string; modelId: string; tokenBudget: number; timeoutSeconds: number }): Promise<Record<string, unknown>> { return this.json("/api/providers/roles", { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async createConversation(body: { title?: string; participantActorIds?: string[] }): Promise<BrowserConversationPage> { return this.json("/api/conversations", { method: "POST", body }) as Promise<BrowserConversationPage>; }
  async createFluctlight(body: { id?: string; name?: string }): Promise<{ id: string; identity: Record<string, unknown>; status: string }> { return this.json("/api/fluctlights", { method: "POST", body }) as Promise<{ id: string; identity: Record<string, unknown>; status: string }>; }
  async listFluctlights(): Promise<Array<{ id: string; identity: Record<string, unknown>; status: string }>> { return this.json("/api/fluctlights") as Promise<Array<{ id: string; identity: Record<string, unknown>; status: string }>>; }
  async getFluctlight(fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}\`) as Promise<Record<string, unknown>>; }
  async messages(conversationId: string, beforeSequence?: number, limit = 50): Promise<BrowserConversationPage> {
    const query = new URLSearchParams({ limit: String(limit) });
    if (beforeSequence !== undefined) query.set("beforeSequence", String(beforeSequence));
    return this.json(\`/api/conversations/\${encodeURIComponent(conversationId)}/messages?\${query}\`) as Promise<BrowserConversationPage>;
  }
  async markRead(conversationId: string, body: { readSequence: number; deliveredSequence?: number }): Promise<void> {
    await this.json(\`/api/conversations/\${encodeURIComponent(conversationId)}/read\`, { method: "POST", body });
  }
  async turn(conversationId: string, body: { text: string; fluctlightId?: string; attachmentRefs?: string[]; idempotencyKey: string; turnId?: string }, signal?: AbortSignal): Promise<Response> {
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
    if (!response.ok) throw new Error(\`Browser request failed: \${response.status}\`);
    if (response.status === 204) return undefined;
    return response.json();
  }
}
`;
await writeFile(fileURLToPath(new URL("src/index.ts", root)), source);
