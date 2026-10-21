import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const schema = JSON.parse(await readFile(new URL("openapi.json", root), "utf8"));
const paths = Object.keys(schema.paths).sort();
const requestTypeNames = [
 ...Object.keys(schema.components.schemas).filter(n=>n.startsWith("BrowserPlaceDefault")),
 ...Object.keys(schema.components.schemas).filter(n=>n.startsWith("BrowserResourcePack")||n.startsWith("BrowserBusinessBackup")||n==="BrowserObjectRevisionPage"),
 ...Object.keys(schema.components.schemas).filter(n=>n.startsWith("BrowserWorkflow")),
 ...Object.keys(schema.components.schemas).filter(n=>["BrowserDiagnosticRecord","BrowserDiagnosticRecordPage","BrowserDiagnosticClearBody","BrowserDiagnosticClearResult","BrowserDiagnosticMediaRecoveryBody"].includes(n)),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserConfiguration")),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserMoment")),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserMedia")),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserStudio")),
 ...Object.keys(schema.components.schemas).filter(name=>["BrowserNPCPresence","BrowserNPCRecord","BrowserNPCRecordPage","BrowserNPCOpportunityRetryBody","BrowserNPCOpportunityRetryResult"].includes(name)),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserPromise")),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserInventory")),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserVisualBrief")||name.startsWith("BrowserVisualGenerate")||name.startsWith("BrowserVisualReview")||["BrowserVisualManagement","BrowserVisualRecordPage","BrowserVisualAttempt","BrowserVisualSession"].includes(name)),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith("BrowserFoundation")||name.startsWith("BrowserCapability")||name==="BrowserGovernanceHistory"),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith('BrowserSelf')),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith('BrowserDiary')||['BrowserOwnerRequest','BrowserSharedExperiencePage'].includes(name)),
 ...Object.keys(schema.components.schemas).filter(name=>name.startsWith('BrowserNotification')||name.startsWith('BrowserPending')||name.startsWith('BrowserAutonomyPolicy')||['BrowserRequestTask','BrowserTask'].includes(name)),
 ...Object.keys(schema.components.schemas).filter(name => name.startsWith("BrowserWorld") || ["BrowserRuntimeWorld", "BrowserRuntimePlace", "BrowserRuntimeNPC", "BrowserNPCRoutineSlot", "BrowserPlaceWriteBody", "BrowserNPCWriteBody", "BrowserNearbyActors"].includes(name)),
 "BrowserConversationMessage", "BrowserConversationDirectoryPage", "BrowserConversationSearchPage", "BrowserNewDirectConversationRequest",
 "BrowserPersonalityRuntime", "BrowserPersonaSwitchHistoryItem", "BrowserPersonaSwitchRequest", "BrowserPersonaSwitchResult", "BrowserPersonaSwitchTerminal", "BrowserPersonaCompilation",
 "BrowserGoalSet","BrowserGoalSetCommand","BrowserActorContext","BrowserActorContextUpdate",
 "BrowserGoal", "BrowserGoalPage", "BrowserGoalHistoryPage", "BrowserGoalCommand", "BrowserGoalCommandResult",
  "BrowserActorUserSettings",
 "BrowserActorUserBackgroundUpdate",
 "BrowserActorUserState",
 "BrowserFluctlightCreateRequest",
  "BrowserFluctlightCreationAnalysisRequest",
  "BrowserFluctlightCreationAnalysis",
  "BrowserFluctlightActivationRequest",
  "BrowserInitializationSource",
  "BrowserFluctlightDetail",
  "BrowserWardrobeItem",
  "BrowserWardrobePage",
  "BrowserLifeEventRequest",
  "BrowserLifeEventCancelRequest",
  "BrowserPresenceRequest",
  "BrowserScheduleItemRequest",
  "BrowserScheduleRequest",
  "BrowserScheduleCancelRequest",
];
const typeForSchema = (value) => {
  if (value?.$ref) return value.$ref.split("/").pop();
  if (Array.isArray(value?.anyOf)) return value.anyOf.map(typeForSchema).join(" | ");
  if (value?.type === "null") return "null";
  if (value?.type === "string" && Array.isArray(value.enum)) return value.enum.map((item) => JSON.stringify(item)).join(" | ");
  if (value?.type === "string") return "string";
  if (value?.type === "integer" || value?.type === "number") return "number";
  if (value?.type === "boolean") return typeof value.const === "boolean" ? String(value.const) : "boolean";
  if (value?.type === "array") return `Array<${typeForSchema(value.items)}>`;
  if (value?.type === "object" || value?.properties) {
    const required = new Set(value.required ?? []);
    const fields = Object.entries(value.properties ?? {}).map(([name, child]) => `${name}${required.has(name) ? "" : "?"}: ${typeForSchema(child)}`);
    const objectType = `{ ${fields.join("; ")} }`;
	if(value.additionalProperties && typeof value.additionalProperties === "object" && fields.length===0)return `Record<string, ${typeForSchema(value.additionalProperties)}>`;
	if (value.additionalProperties === true && fields.length === 0) return "Record<string, unknown>";
    return value.additionalProperties === true ? `(${objectType} & Record<string, unknown>)` : objectType;
  }
  return "unknown";
};
const requestTypes = requestTypeNames.map((name) => {
  const component = schema.components?.schemas?.[name];
  if (!component) throw new Error(`OpenAPI component ${name} is missing`);
  return `export type ${name} = ${typeForSchema(component)};`;
}).join("\n");
const source = `// Generated from packages/browser-client/openapi.json. Do not edit by hand.
export const browserOperations = ${JSON.stringify(paths)} as const;
${requestTypes}
export type BrowserHealth = { status: string; role: string };
export type BrowserSession = { authenticated: boolean; actorId?: string };
export type BrowserSetupStatus = { setupAvailable: boolean };
export type BrowserSafeSettings = { values: Record<string, unknown>; configuredSecrets: string[]; versions?: Record<string,number>; command?:Record<string,unknown> };
export type BrowserDiagnosticEvent = { id: string; eventType: string; severity: string; fluctlightId?: string | null; causationId?: string | null; correlationId: string; payload: Record<string, unknown>; createdAt?: string | null };
export type BrowserLifecycleDiagnosticsFilter = { limit?: number; fluctlightId?: string; correlationId?: string; intentId?: string; workflowId?: string; runId?: string; surface?: string; status?: string };
export type BrowserLifecycleDiagnosticEvent = { id: string; eventType: string; surface: string; transition: string; severity: string; fluctlightId?: string | null; correlationId: string; causationId?: string | null; intentId?: string; workflowId?: string; runId?: string; activityType?: string; activityId?: string; providerAttemptId?: string; providerRequestId?: string; modelRunId?: string; stage: string; status: string; reasonCode: string; errorCategory?: string; errorCode?: string; safeCause?: string; retryable: boolean; attempt?: number; maxAttempts?: number; nextDueAt?: string; occurredAt?: string; occurrenceCount?: number; metadata?: Record<string, unknown>; createdAt: string };
export type BrowserWorkflowIntentSnapshot = { intentId: string; workflowId: string; runtimeWorkflowId: string; taskQueue: string; intentType: string; status: string; attemptCount: number; fluctlightId?: string | null; correlationId: string; causationId?: string | null; lastError?: string | null; nextAttemptAt?: string; startedAt?: string; completedAt?: string; createdAt: string };
export type BrowserLifecycleDiagnosticsPage = { events: BrowserLifecycleDiagnosticEvent[]; workflowIntents: BrowserWorkflowIntentSnapshot[]; filters: Required<BrowserLifecycleDiagnosticsFilter> };
export type BrowserDiagnosticModelRun = { id: string; role: string; bindingRole?: string; scenario?: string; priority?: number; queuePendingCount?: number; queuePosition?: number; endpointId?: string | null; modelId: string; prompt: unknown; response?: unknown; tools?: unknown; status: string; goalEvaluation?: Record<string,unknown>; errorCode?: string | null; correlationId: string; logicalRunId?: string; parentRunId?: string; modelCallId?: string; roundCount?: number; sequence?: number | null; stage?: string; toolSummaries?: Array<{ callId: string; capability: string; status: string; errorCode?: string }>; createdAt: string; orderingKey?: string; queuedAt?: string; startedAt?: string | null; completedAt?: string | null };
export type BrowserDiagnosticAgentRun = { fluctlightId: string; agentId: string; runId: string; correlationId: string; associationStatus: "linked" | "unknown"; status: string; source?: "agent_runs" | "termination_event"; failureStage?: string; failureCode?: string; safeCause?: string; startedAt: string; orderingKey?: string; finishedAt?: string };
export type BrowserDiagnosticRunPage<T> = { items: T[]; nextCursor: string; snapshot: string };
export type BrowserDiagnosticMediaPrompt = { id: string; mediaIntentId: string; fluctlightId: string; kind: string; mimeType: string; prompt: unknown; providerPrompt: string; submittedPrompt?: string; requestPayload?: unknown; providerRequestId: string; providerJobId?: string; workflowId: string; status: string; qualityVerdict?: string; correlationId: string; createdAt: string; submittedEventId?: string; submittedAt?: string; errorMessage?: string; failureStage?: string; workflowStatus?: string; attemptCount?: number; modelRun?: BrowserDiagnosticModelRun };
export type BrowserConversation = { id: string; createdByActorId: string; title?: string | null; revision: number; createdAt: string; updatedAt: string };
export type BrowserParticipant = { conversationId: string; actorId: string; role: string; status: string; joinedAt: string; leftAt?: string | null };
export type BrowserMessage = BrowserConversationMessage;
export type BrowserVisualIdentityTimelineEvent = { session_id: string; attempt_id?: string; stage: string; status: string; summary: string; asset_ids: string[]; metadata: Record<string, unknown>; correlation_id: string; occurred_at: string };
export type BrowserVisualIdentity = { schema_version: string; id: string; fluctlight_id: string; status: string; current_revision: number; identity_snapshot: Record<string, unknown>; renderer_constraints: Record<string, unknown>; canonical_asset_id?: string; character_sheet_asset_id?: string; adapter_version: string; active_session_id?: string; timeline: BrowserVisualIdentityTimelineEvent[] };
export type BrowserConversationPage = { conversation: BrowserConversation; participants: BrowserParticipant[]; messages: BrowserMessage[]; nextBeforeSequence?: number | null };
export type BrowserTurnEvent = { type: "token" | "message" | "media" | "completed" | "error" | "heartbeat"; turnId: string; sequence: number; payload: Record<string, unknown> };

const lifecycleDiagnosticsQuery = (options: BrowserLifecycleDiagnosticsFilter, defaultLimit: number): string => {
  const query = new URLSearchParams({ limit: String(options.limit ?? defaultLimit) });
  if (options.fluctlightId) query.set("fluctlightId", options.fluctlightId);
  if (options.correlationId) query.set("correlationId", options.correlationId);
  if (options.intentId) query.set("intentId", options.intentId);
  if (options.workflowId) query.set("workflowId", options.workflowId);
  if (options.runId) query.set("runId", options.runId);
  if (options.surface) query.set("surface", options.surface);
  if (options.status) query.set("status", options.status);
  return query.toString();
};

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
  constructor(private readonly baseUrl = "", private readonly fetcher: typeof fetch = globalThis.fetch.bind(globalThis), private readonly readTimeoutMs = 15000) {}
  private url(path: string): URL {
    const origin = this.baseUrl || (typeof window !== "undefined" ? window.location.origin : "");
    if (!origin) throw new Error("BrowserClient requires a base URL outside the browser");
    return new URL(path, origin);
  }
  async health(path: "/health/live" | "/health/ready"): Promise<BrowserHealth> { return this.json(path) as Promise<BrowserHealth>; }
  async session(): Promise<BrowserSession> {
    return this.json("/auth/session", { anonymousSession: true, timeoutMs: Math.min(this.readTimeoutMs, 8000) }) as Promise<BrowserSession>;
  }
  async login(password: string): Promise<BrowserSession> { return this.json("/auth/login", { method: "POST", body: { password } }) as Promise<BrowserSession>; }
  async setup(setupToken: string, password: string): Promise<BrowserSession> { return this.json("/auth/setup", { method: "POST", body: { setupToken, password } }) as Promise<BrowserSession>; }
  async setupStatus(): Promise<BrowserSetupStatus> { return this.json("/auth/setup-status") as Promise<BrowserSetupStatus>; }
  async logout(): Promise<void> { await this.json("/auth/logout", { method: "POST" }); }
  async changePassword(password: string): Promise<void> { await this.json("/auth/password", { method: "POST", body: { password } }); }
  async recoverConfigurationCommand(key:string):Promise<Record<string,unknown>>{return this.json('/api/configuration-commands/recover',{method:'POST',body:{key}}) as Promise<Record<string,unknown>>;}
  async cancelUnacceptedConfigurationCommand(key:string):Promise<Record<string,unknown>>{return this.json('/api/configuration-commands/cancel-unaccepted',{method:'POST',body:{key}}) as Promise<Record<string,unknown>>;}
  async ownerWorkflowInspection(workflowId:string):Promise<BrowserWorkflowInspection>{return this.json(\`/api/workflow-controls/\${encodeURIComponent(workflowId)}\`) as Promise<BrowserWorkflowInspection>;}
  async ownerWorkflowHistoryPage(workflowId:string,runId:string,cursor=''):Promise<BrowserWorkflowHistoryPage>{return this.json(\`/api/workflow-controls/\${encodeURIComponent(workflowId)}/history?\${new URLSearchParams({runId,cursor})}\`) as Promise<BrowserWorkflowHistoryPage>;}
  async ownerWorkflowControl(workflowId:string,action:'pause'|'resume'|'cancel'|'restart'|'reset',body:BrowserWorkflowControlBody):Promise<Record<string,unknown>>{return this.json(\`/api/workflow-controls/\${encodeURIComponent(workflowId)}/\${action}\`,{method:'POST',body}) as Promise<Record<string,unknown>>;}
  async retryOwnerWorkflowCommand(workflowId:string,commandId:string):Promise<Record<string,unknown>>{return this.json(\`/api/workflow-controls/\${encodeURIComponent(workflowId)}/commands/\${encodeURIComponent(commandId)}/retry\`,{method:'POST',body:{}}) as Promise<Record<string,unknown>>;}
  async retryOwnerDiagnosticMedia(recordId:string,body:BrowserDiagnosticMediaRecoveryBody):Promise<Record<string,unknown>>{return this.json(\`/api/diagnostic-records/media-prompts/\${encodeURIComponent(recordId)}/retry\`,{method:'POST',body}) as Promise<Record<string,unknown>>;}
  async placeDefaults(fid:string,worldId:string,placeId:string,source='recorded'):Promise<BrowserPlaceDefaultView>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/places/\${encodeURIComponent(placeId)}/defaults?\${new URLSearchParams({worldId,source})}\`) as Promise<BrowserPlaceDefaultView>;}
  async savePlaceDefaults(fid:string,placeId:string,body:BrowserPlaceDefaultBody):Promise<Record<string,unknown>>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/places/\${encodeURIComponent(placeId)}/defaults\`,{method:'POST',body}) as Promise<Record<string,unknown>>;}
  async placeDefaultHistory(fid:string,worldId:string,placeId:string,cursor=''):Promise<BrowserPlaceDefaultHistory>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/places/\${encodeURIComponent(placeId)}/defaults/history?\${new URLSearchParams({worldId,cursor})}\`) as Promise<BrowserPlaceDefaultHistory>;}
  async businessBackups(cursor=''):Promise<BrowserBusinessBackupPage>{return this.json(\`/api/ops/backups?\${new URLSearchParams({cursor})}\`) as Promise<BrowserBusinessBackupPage>;}
  async businessBackup(id:string):Promise<BrowserBusinessBackupJob>{return this.json(\`/api/ops/backups/\${encodeURIComponent(id)}\`) as Promise<BrowserBusinessBackupJob>;}
  async businessBackupCommand(id:string,action:'create'|'restore'|'retry'|'cancel',body:BrowserBusinessBackupBody):Promise<Record<string,unknown>>{return this.json(action==='create'?'/api/ops/backups':\`/api/ops/backups/\${encodeURIComponent(id)}/\${action}\`,{method:'POST',body}) as Promise<Record<string,unknown>>;}
  async exportResourcePack(resourceId=''):Promise<BrowserResourcePackExport>{return this.json(\`/api/studio/packs/export?\${new URLSearchParams({resourceId})}\`) as Promise<BrowserResourcePackExport>;}
  async validateResourcePack(pack:BrowserResourcePack):Promise<BrowserResourcePackValidation>{return this.json('/api/studio/packs/validate',{method:'POST',body:{pack}}) as Promise<BrowserResourcePackValidation>;}
  async importResourcePack(body:BrowserResourcePackImportBody):Promise<BrowserResourcePackImportResult>{return this.json('/api/studio/packs',{method:'POST',body}) as Promise<BrowserResourcePackImportResult>;}
  async resourcePack(id:string):Promise<Record<string,unknown>>{return this.json(\`/api/studio/packs/\${encodeURIComponent(id)}\`) as Promise<Record<string,unknown>>;}
  async adoptResourcePackGroup(id:string,index:number,body:BrowserResourcePackGroupBody):Promise<Record<string,unknown>>{return this.json(\`/api/studio/packs/\${encodeURIComponent(id)}/groups/\${index}/adopt\`,{method:'POST',body}) as Promise<Record<string,unknown>>;}
  async ownerObjectRevisions(options:Record<string,string>={}):Promise<BrowserObjectRevisionPage>{return this.json(\`/api/data/revisions?\${new URLSearchParams(options)}\`) as Promise<BrowserObjectRevisionPage>;}
  async ownerHarmSummary():Promise<Record<string,unknown>>{return this.json('/api/diagnostics/harm-intercepts') as Promise<Record<string,unknown>>;}
  async ownerDiagnosticRecords(kind:string,options:Record<string,string|number>={}):Promise<BrowserDiagnosticRecordPage>{const query=new URLSearchParams(Object.entries(options).map(([k,v])=>[k,String(v)]));return this.json(\`/api/diagnostic-records/\${encodeURIComponent(kind)}?\${query}\`) as Promise<BrowserDiagnosticRecordPage>;}
  async ownerDiagnosticRecord(kind:string,recordRef:string):Promise<BrowserDiagnosticRecord>{return this.json(\`/api/diagnostic-records/\${encodeURIComponent(kind)}/\${encodeURIComponent(recordRef)}\`) as Promise<BrowserDiagnosticRecord>;}
  async clearOwnerDiagnosticRecords(body:BrowserDiagnosticClearBody):Promise<BrowserDiagnosticClearResult>{return this.json('/api/diagnostic-records/clear',{method:'POST',body}) as Promise<BrowserDiagnosticClearResult>;}
  async streamDiagnosticExport(options:Record<string,string>={}):Promise<Response>{const response=await this.fetcher(this.url(\`/api/diagnostic-export?\${new URLSearchParams(options)}\`),{credentials:'include'});if(!response.ok){const value=await response.json().catch(()=>({}));throw new BrowserApiError(response.status,value.code||'diagnostic_export_failed',value.message||'诊断导出失败',value.details||{});}return response;}
  async configurationCatalog():Promise<BrowserConfigurationCatalog>{return this.json('/api/configuration-catalog') as Promise<BrowserConfigurationCatalog>;}
  async configurationCommandByKey(key:string):Promise<Record<string,unknown>>{return this.json(\`/api/configuration-commands/by-key?\${new URLSearchParams({key})}\`) as Promise<Record<string,unknown>>;}
  async runtimeCapabilityCatalog():Promise<BrowserConfigurationRuntimeCatalog>{return this.json('/api/settings/agents') as Promise<BrowserConfigurationRuntimeCatalog>;}
  async testSavedMediaConnection():Promise<Record<string,unknown>>{return this.json('/api/settings/media/test-connection',{method:'POST',body:{}}) as Promise<Record<string,unknown>>;}
  async revokeAllSessions():Promise<void>{await this.json('/auth/revoke-all',{method:'POST',body:{}});}
  async settings(): Promise<BrowserSafeSettings> { return this.json("/api/settings") as Promise<BrowserSafeSettings>; }
  async updateSettings(body: { values?: Record<string, unknown>; secrets?: Record<string, string | null>; clearSecrets?: string[];expectedVersions:Record<string,number>;idempotencyKey:string }): Promise<BrowserSafeSettings> { return this.json("/api/settings", { method: "PUT", body }) as Promise<BrowserSafeSettings>; }
  async governancePage(fluctlightId:string,kind:"foundation"|"capability",options:Record<string,string|number>={}):Promise<BrowserFoundationPage|BrowserCapabilityPage>{const params=new URLSearchParams(Object.entries(options).map(([key,value])=>[key,String(value)]));return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/\${kind==="foundation"?"foundation-revisions":"capability-requests"}?\${params}\`) as Promise<BrowserFoundationPage|BrowserCapabilityPage>;}
  async governanceObject(fluctlightId:string,kind:"foundation",id:string):Promise<BrowserFoundationRevision>;
  async governanceObject(fluctlightId:string,kind:"capability",id:string):Promise<BrowserCapabilityRequest>;
  async governanceObject(fluctlightId:string,kind:"foundation"|"capability",id:string):Promise<BrowserFoundationRevision|BrowserCapabilityRequest>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/\${kind==="foundation"?"foundation-revisions":"capability-requests"}/\${encodeURIComponent(id)}\`) as Promise<BrowserFoundationRevision|BrowserCapabilityRequest>;}
  async governanceHistory(fluctlightId:string,kind:"foundation"|"capability",id:string,cursor=""):Promise<BrowserGovernanceHistory>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/\${kind==="foundation"?"foundation-revisions":"capability-requests"}/\${encodeURIComponent(id)}/history?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserGovernanceHistory>;}
  async reviewScopedCapabilityRequest(fid:string,id:string,body:BrowserCapabilityReviewBody):Promise<BrowserCapabilityRequest>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/capability-requests/\${encodeURIComponent(id)}/review\`,{method:"POST",body}) as Promise<BrowserCapabilityRequest>;}
  async reviewVisualIdentity(fid:string,sessionId:string,body:BrowserVisualReviewBody):Promise<BrowserVisualReviewResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/visual-sessions/\${encodeURIComponent(sessionId)}/review\`,{method:"POST",body}) as Promise<BrowserVisualReviewResult>;}
  async requestInventoryAction(fid:string,body:BrowserInventoryActionBody):Promise<BrowserInventoryActionResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/requests\`,{method:"POST",body}) as Promise<BrowserInventoryActionResult>;}
  async npcOpportunity(fid:string,npc:string,id:string):Promise<BrowserNPCRecord>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/npcs/\${encodeURIComponent(npc)}/opportunities/\${encodeURIComponent(id)}\`) as Promise<BrowserNPCRecord>;}
  async npcRecords(fid:string,id:string,kind:"encounters"|"presences"|"opportunities",cursor=""):Promise<BrowserNPCRecordPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/npcs/\${encodeURIComponent(id)}/\${kind}?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserNPCRecordPage>;}
  async retryNPCOpportunity(fid:string,npc:string,id:string,body:BrowserNPCOpportunityRetryBody):Promise<BrowserNPCOpportunityRetryResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/npcs/\${encodeURIComponent(npc)}/opportunities/\${encodeURIComponent(id)}/retry\`,{method:"POST",body}) as Promise<BrowserNPCOpportunityRetryResult>;}
  async worldLifeEvents(fid:string,cursor=""):Promise<BrowserWorldLifeEventPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/events?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserWorldLifeEventPage>;}
  async worldLifeEvent(fid:string,id:string):Promise<BrowserWorldLifeEvent>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/events/\${encodeURIComponent(id)}\`) as Promise<BrowserWorldLifeEvent>;}
  async mediaReferenceGrants(options:{asset_id?:string;target_id?:string;target_scope?:'studio_resource'|'fluctlight';status?:'active'|'revoked';cursor?:string}={}):Promise<BrowserMediaReferenceGrantPage>{const query=new URLSearchParams();for(const [key,value]of Object.entries(options))if(value!==undefined)query.set(key,value);return this.json(\`/api/media-reference-grants?\${query}\`) as Promise<BrowserMediaReferenceGrantPage>;}
  async mediaReferenceGrant(id:string):Promise<BrowserMediaReferenceGrant>{return this.json(\`/api/media-reference-grants/\${encodeURIComponent(id)}\`) as Promise<BrowserMediaReferenceGrant>;}
  async createMediaReferenceGrant(body:BrowserMediaReferenceGrantBody):Promise<BrowserMediaReferenceGrant>{return this.json('/api/media-reference-grants',{method:'POST',body}) as Promise<BrowserMediaReferenceGrant>;}
  async revokeMediaReferenceGrant(id:string,body:BrowserMediaReferenceRevokeBody):Promise<BrowserMediaReferenceGrant>{return this.json(\`/api/media-reference-grants/\${encodeURIComponent(id)}/revoke\`,{method:'POST',body}) as Promise<BrowserMediaReferenceGrant>;}
  async mediaReferenceGrantRecords(id:string,kind:'history'|'uses',cursor=''):Promise<BrowserMediaReferenceGrantRecords>{return this.json(\`/api/media-reference-grants/\${encodeURIComponent(id)}/\${kind}?\${new URLSearchParams({cursor})}\`) as Promise<BrowserMediaReferenceGrantRecords>;}
  async requestMediaGeneration(fid:string,body:BrowserMediaGenerateBody):Promise<BrowserMediaRequestEnvelope>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/media-generations\`,{method:'POST',body}) as Promise<BrowserMediaRequestEnvelope>;}
  async mediaGenerationRequests(fid:string,cursor=''):Promise<BrowserMediaGenerationPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/media-generations?\${new URLSearchParams({cursor})}\`) as Promise<BrowserMediaGenerationPage>;}
  async mediaGenerationRequest(fid:string,id:string):Promise<BrowserMediaGenerationRead>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/media-generations/\${encodeURIComponent(id)}\`) as Promise<BrowserMediaGenerationRead>;}
  async momentNPCInteractions(id:string,cursor=''):Promise<BrowserMomentNPCPage>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}/npc-interactions?\${new URLSearchParams({cursor})}\`) as Promise<BrowserMomentNPCPage>;}
  async momentNPCInteraction(id:string,interaction:string):Promise<Record<string,unknown>>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}/npc-interactions/\${encodeURIComponent(interaction)}\`) as Promise<Record<string,unknown>>;}
  async retryMomentNPCInteraction(id:string,interaction:string,body:BrowserMomentStatusBody):Promise<Record<string,unknown>>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}/npc-interactions/\${encodeURIComponent(interaction)}/retry\`,{method:'POST',body}) as Promise<Record<string,unknown>>;}
  async momentSuggestions(fluctlightId='',cursor=''):Promise<BrowserMomentSuggestions>{return this.json(\`/api/moment-suggestions?\${new URLSearchParams({fluctlightId,cursor})}\`) as Promise<BrowserMomentSuggestions>;}
  async momentFeed(options:{fluctlightId?:string;status?:'visible'|'hidden'|'all';q?:string;cursor?:string;limit?:number}={}):Promise<BrowserMomentPage>{const query=new URLSearchParams();for(const [key,value] of Object.entries(options))if(value!==undefined)query.set(key,String(value));return this.json(\`/api/moment-feed?\${query}\`) as Promise<BrowserMomentPage>;}
  async momentRecord(id:string):Promise<BrowserMomentRecord>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}\`) as Promise<BrowserMomentRecord>;}
  async momentComments(id:string,cursor=''):Promise<BrowserMomentComments>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}/comments?\${new URLSearchParams({cursor})}\`) as Promise<BrowserMomentComments>;}
  async writeMomentComment(id:string,body:BrowserMomentCommentBody):Promise<BrowserMomentRecord>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}/comments\`,{method:'POST',body}) as Promise<BrowserMomentRecord>;}
  async writeMomentReaction(id:string,body:BrowserMomentReactionBody):Promise<BrowserMomentRecord>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}/reaction\`,{method:'POST',body}) as Promise<BrowserMomentRecord>;}
  async writeMomentStatus(id:string,action:'hide'|'restore',body:BrowserMomentStatusBody):Promise<BrowserMomentRecord>{return this.json(\`/api/moment-feed/\${encodeURIComponent(id)}/\${action}\`,{method:'POST',body}) as Promise<BrowserMomentRecord>;}
  async mediaAssetReferences(id:string,cursor=''):Promise<BrowserMediaReferencePage>{return this.json(\`/api/media-assets/\${encodeURIComponent(id)}/references?\${new URLSearchParams({cursor})}\`) as Promise<BrowserMediaReferencePage>;}
  async deleteMediaAsset(id:string,body:BrowserMediaDeleteBody):Promise<BrowserMediaDeletion>{return this.json(\`/api/media-assets/\${encodeURIComponent(id)}\`,{method:'DELETE',body}) as Promise<BrowserMediaDeletion>;}
  async mediaDeletion(id:string):Promise<BrowserMediaDeletion>{return this.json(\`/api/media-deletions/\${encodeURIComponent(id)}\`) as Promise<BrowserMediaDeletion>;}
  async retryMediaDeletion(id:string,body:BrowserMediaDeleteRetryBody):Promise<BrowserMediaDeletion>{return this.json(\`/api/media-deletions/\${encodeURIComponent(id)}/retry\`,{method:'POST',body}) as Promise<BrowserMediaDeletion>;}
  async studioMaterial(id:string):Promise<BrowserStudioMaterial>{return this.json(\`/api/studio/materials/\${encodeURIComponent(id)}\`) as Promise<BrowserStudioMaterial>;}
  async setStudioMaterialRetention(id:string,operation:'retain'|'release',body:BrowserStudioJobCommandBody):Promise<BrowserStudioMaterial>{return this.json(\`/api/studio/materials/\${encodeURIComponent(id)}/\${operation}\`,{method:'POST',body}) as Promise<BrowserStudioMaterial>;}
  async studioMaterialHistory(id:string,cursor=''):Promise<BrowserStudioMaterialHistory>{return this.json(\`/api/studio/materials/\${encodeURIComponent(id)}/history?\${new URLSearchParams({cursor})}\`) as Promise<BrowserStudioMaterialHistory>;}
  async displayAvatar(fid:string):Promise<BrowserMediaAvatar>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/display-avatar\`) as Promise<BrowserMediaAvatar>;}
  async selectDisplayAvatar(fid:string,body:BrowserMediaAvatarBody):Promise<BrowserMediaAvatar>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/display-avatar\`,{method:'PUT',body}) as Promise<BrowserMediaAvatar>;}
  async displayAvatarHistory(fid:string,cursor=''):Promise<BrowserMediaAvatarHistory>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/display-avatar/history?\${new URLSearchParams({cursor})}\`) as Promise<BrowserMediaAvatarHistory>;}
  async ownerMediaLibrary(options:{scope?:'all'|'fluctlight'|'studio'|'owner_upload';fluctlightId?:string;kind?:'image'|'video'|'audio';q?:string;hidden?:boolean;cursor?:string;limit?:number}={}):Promise<BrowserMediaLibraryPage>{const query=new URLSearchParams();for(const [key,value]of Object.entries(options))if(value!==undefined)query.set(key,String(value));return this.json(\`/api/media-library?\${query}\`) as Promise<BrowserMediaLibraryPage>;}
  async ownerMediaAsset(id:string,fluctlightId?:string):Promise<BrowserMediaAsset>{return this.json(\`/api/media-assets/\${encodeURIComponent(id)}?\${new URLSearchParams(fluctlightId?{fluctlightId}:{})}\`) as Promise<BrowserMediaAsset>;}
  async ownerMediaUploadByKey(key:string):Promise<BrowserMediaUploadLookup>{return this.json(\`/api/media-uploads/by-key?\${new URLSearchParams({key})}\`) as Promise<BrowserMediaUploadLookup>;}
  async saveMediaMetadata(id:string,body:BrowserMediaMetadataBody):Promise<BrowserMediaAsset>{return this.json(\`/api/media-assets/\${encodeURIComponent(id)}\`,{method:'PUT',body}) as Promise<BrowserMediaAsset>;}
  async worldItems(fid:string,options:{cursor?:string;q?:string}={}):Promise<BrowserWorldItemPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/items?\${new URLSearchParams(options)}\`) as Promise<BrowserWorldItemPage>;}
  async worldItem(fid:string,id:string):Promise<BrowserWorldItem>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/items/\${encodeURIComponent(id)}\`) as Promise<BrowserWorldItem>;}
  async configureWorldItem(fid:string,id:string,body:BrowserWorldItemConfigBody):Promise<BrowserWorldItem>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/items/\${encodeURIComponent(id)}\`,{method:'PUT',body}) as Promise<BrowserWorldItem>;}
  async worldEventTemplate(fid:string,id:string):Promise<BrowserWorldEventTemplate>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/templates/\${encodeURIComponent(id)}\`) as Promise<BrowserWorldEventTemplate>;}
  async worldEventRuns(fid:string,id:string,cursor=""):Promise<BrowserWorldEventPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/templates/\${encodeURIComponent(id)}/runs?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserWorldEventPage>;}
  async worldEventRun(fid:string,id:string,run:string):Promise<BrowserWorldEventRun>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/templates/\${encodeURIComponent(id)}/runs/\${encodeURIComponent(run)}\`) as Promise<BrowserWorldEventRun>;}
  async triggerWorldEvent(fid:string,id:string,body:BrowserWorldEventTriggerBody):Promise<BrowserWorldEventRun>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/templates/\${encodeURIComponent(id)}/runs\`,{method:"POST",body}) as Promise<BrowserWorldEventRun>;}
  async retryWorldEvent(fid:string,id:string,run:string,body:BrowserWorldEventRetryBody):Promise<BrowserWorldEventCommandResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/world/templates/\${encodeURIComponent(id)}/runs/\${encodeURIComponent(run)}/retry\`,{method:"POST",body}) as Promise<BrowserWorldEventCommandResult>;}
  async promises(fid:string,cursor=""):Promise<BrowserPromisePage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/promises?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserPromisePage>;}
  async promise(fid:string,id:string):Promise<BrowserPromise>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/promises/\${encodeURIComponent(id)}\`) as Promise<BrowserPromise>;}
  async promiseHistory(fid:string,id:string,cursor=""):Promise<BrowserPromiseHistory>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/promises/\${encodeURIComponent(id)}/history?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserPromiseHistory>;}
  async reschedulePromise(fid:string,id:string,body:BrowserPromiseRescheduleBody):Promise<BrowserPromiseRequestResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/promises/\${encodeURIComponent(id)}/reschedule\`,{method:"POST",body}) as Promise<BrowserPromiseRequestResult>;}
  async promiseCommand(fid:string,id:string,operation:"cancel"|"retry",body:BrowserPromiseCommandBody):Promise<BrowserPromiseCommandResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/promises/\${encodeURIComponent(id)}/\${operation}\`,{method:"POST",body}) as Promise<BrowserPromiseCommandResult>;}
  async inventoryPreparation(fid:string):Promise<BrowserInventoryPreparation>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/preparation\`) as Promise<BrowserInventoryPreparation>;}
  async prepareInventory(fid:string,body:BrowserInventoryPreparationBody):Promise<BrowserInventoryPreparation>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/preparation\`,{method:"POST",body}) as Promise<BrowserInventoryPreparation>;}
  async inventoryTags(fid:string):Promise<BrowserInventoryTags>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/tags\`) as Promise<BrowserInventoryTags>;}
  async saveInventoryTags(fid:string,body:BrowserInventoryTagsBody):Promise<BrowserInventoryTags>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/tags\`,{method:"PUT",body}) as Promise<BrowserInventoryTags>;}
  async inventoryOutfits(fid:string,cursor=""):Promise<BrowserInventoryOutfitPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/outfits?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserInventoryOutfitPage>;}
  async inventoryOutfit(fid:string,id:string):Promise<BrowserInventoryOutfit>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/outfits/\${encodeURIComponent(id)}\`) as Promise<BrowserInventoryOutfit>;}
  async createInventoryOutfit(fid:string,body:BrowserInventoryOutfitWriteBody):Promise<BrowserInventoryOutfitWriteResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/outfits\`,{method:"POST",body}) as Promise<BrowserInventoryOutfitWriteResult>;}
  async renameInventoryOutfit(fid:string,id:string,body:BrowserInventoryOutfitRenameBody):Promise<BrowserInventoryOutfitWriteResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/outfits/\${encodeURIComponent(id)}\`,{method:"PATCH",body}) as Promise<BrowserInventoryOutfitWriteResult>;}
  async updateInventoryOutfit(fid:string,id:string,body:BrowserInventoryOutfitWriteBody):Promise<BrowserInventoryOutfitWriteResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/outfits/\${encodeURIComponent(id)}\`,{method:"PUT",body}) as Promise<BrowserInventoryOutfitWriteResult>;}
  async inventoryOutfitRevisions(fid:string,id:string,cursor=""):Promise<BrowserInventoryHistory>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/outfits/\${encodeURIComponent(id)}/revisions?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserInventoryHistory>;}
  async inventoryWearingHistory(fid:string,cursor=""):Promise<BrowserInventoryHistory>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/wearing-history?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserInventoryHistory>;}
  async inventoryPage(fid:string,options:Record<string,string|number>={}):Promise<BrowserInventoryPage>{const query=new URLSearchParams(Object.entries(options).map(([key,value])=>[key,String(value)]));return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory?\${query}\`) as Promise<BrowserInventoryPage>;}
  async inventoryItem(fid:string,id:string):Promise<BrowserInventoryItem>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/items/\${encodeURIComponent(id)}\`) as Promise<BrowserInventoryItem>;}
  async inventoryHistory(fid:string,id:string,cursor=""):Promise<BrowserInventoryHistory>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/items/\${encodeURIComponent(id)}/history?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserInventoryHistory>;}
  async createInventoryItem(fid:string,body:BrowserInventoryCreateBody):Promise<BrowserInventoryResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/items\`,{method:"POST",body}) as Promise<BrowserInventoryResult>;}
  async updateInventoryItem(fid:string,id:string,body:BrowserInventoryUpdateBody):Promise<BrowserInventoryResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/items/\${encodeURIComponent(id)}\`,{method:"PUT",body}) as Promise<BrowserInventoryResult>;}
  async archiveInventoryItem(fid:string,id:string,body:BrowserInventoryArchiveBody):Promise<BrowserInventoryResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/inventory/items/\${encodeURIComponent(id)}\`,{method:"DELETE",body}) as Promise<BrowserInventoryResult>;}
  async visualBrief(fid:string):Promise<BrowserVisualBriefRead>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/visual-brief\`) as Promise<BrowserVisualBriefRead>;}
  async saveVisualBrief(fid:string,body:BrowserVisualBriefBody):Promise<BrowserVisualBriefRead>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/visual-brief\`,{method:"PUT",body}) as Promise<BrowserVisualBriefRead>;}
  async visualBriefHistory(fid:string,cursor=""):Promise<BrowserVisualRecordPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/visual-brief/history?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserVisualRecordPage>;}
  async visualIdentityManagement(fid:string):Promise<BrowserVisualManagement>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/visual-identity\`) as Promise<BrowserVisualManagement>;}
  async visualIdentityTimeline(fid:string,cursor=""):Promise<BrowserVisualRecordPage>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/visual-identity/timeline?cursor=\${encodeURIComponent(cursor)}\`) as Promise<BrowserVisualRecordPage>;}
  async generateVisualIdentity(fid:string,body:BrowserVisualGenerateBody):Promise<BrowserVisualGenerateResult>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fid)}/visual-identity/generate\`,{method:"POST",body}) as Promise<BrowserVisualGenerateResult>;}
  async listCapabilityRequests(): Promise<Array<Record<string, unknown>>> { return this.json("/api/capability-requests") as Promise<Array<Record<string, unknown>>>; }
  async reviewCapabilityRequest(requestId: string, body: BrowserCapabilityReviewBody): Promise<Record<string, unknown>> { return this.json(\`/api/capability-requests/\${encodeURIComponent(requestId)}/review\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async configureProviderEndpoint(body: { endpointId: string; kind: string; baseUrl: string; secretPurpose: string;expectedVersions:Record<string,number>;idempotencyKey:string }): Promise<void> { await this.json("/api/providers/endpoints", { method: "PUT", body }); }
  async providerEndpoints(): Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; secret_purpose:string;revision:number; capability_status: string; roles: Array<{ role: string; model_id: string }> }>> { return this.json("/api/providers/endpoints") as Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; secret_purpose:string;revision:number; capability_status: string; roles: Array<{ role: string; model_id: string }> }>>; }
  async providerEndpointModels(endpointId: string): Promise<{ endpointId: string; models: string[] }> { return this.json(\`/api/providers/endpoints/\${encodeURIComponent(endpointId)}/models\`) as Promise<{ endpointId: string; models: string[] }>; }
  async providerBindings(): Promise<Array<{ role: string; endpoint_id: string; model_id: string; token_budget: number; timeout_seconds: number; context_window_tokens:number;max_input_tokens:number;prompt_budget_policy_version:string;safety_margin_tokens:number;endpoint_status: string }>> { return this.json("/api/providers") as Promise<Array<{ role: string; endpoint_id: string; model_id: string; token_budget: number; timeout_seconds: number; context_window_tokens:number;max_input_tokens:number;prompt_budget_policy_version:string;safety_margin_tokens:number;endpoint_status: string }>>; }
  async configureModelRole(body: { role: string; endpointId: string; modelId: string; tokenBudget: number; timeoutSeconds: number; contextWindowTokens?: number; maxInputTokens?: number;expectedVersions:Record<string,number>;idempotencyKey:string }): Promise<Record<string, unknown>> { return this.json("/api/providers/roles", { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async createConversation(body: { title?: string; participantActorIds: string[] }): Promise<BrowserConversationPage> { return this.json("/api/conversations", { method: "POST", body }) as Promise<BrowserConversationPage>; }
  async createFluctlight(body: BrowserFluctlightCreateRequest): Promise<{ id: string; identity: Record<string, unknown>; status: string }> { return this.json("/api/fluctlights", { method: "POST", body }) as Promise<{ id: string; identity: Record<string, unknown>; status: string }>; }
  async analyzeFluctlightCreation(input: string | BrowserFluctlightCreationAnalysisRequest): Promise<BrowserFluctlightCreationAnalysis> { const body: BrowserFluctlightCreationAnalysisRequest = typeof input === "string" ? { description: input } : input; return this.json("/api/fluctlight-creations/analysis", { method: "POST", body }) as Promise<BrowserFluctlightCreationAnalysis>; }
  async activateFluctlightCreation(body: BrowserFluctlightActivationRequest): Promise<{ id: string; core_persona?: Record<string, unknown>; identity: Record<string, unknown>; status: string }> { return this.json("/api/fluctlight-creations/activate", { method: "POST", body }) as Promise<{ id: string; core_persona?: Record<string, unknown>; identity: Record<string, unknown>; status: string }>; }
  async listFluctlights(): Promise<Array<{ id: string; identity: Record<string, unknown>; status: string; unread_count?: number; last_conversation_at?: string | null }>> { return this.json("/api/fluctlights") as Promise<Array<{ id: string; identity: Record<string, unknown>; status: string; unread_count?: number; last_conversation_at?: string | null }>>; }
  async listActorGroups(): Promise<Array<{ id: string; name: string; actor_ids?: string[]; members?: string[] }>> { return this.json("/api/actor-groups") as Promise<Array<{ id: string; name: string; actor_ids?: string[]; members?: string[] }>>; }
  async createActorGroup(name: string): Promise<{ id: string; name: string; actor_ids?: string[]; members?: string[] }> { return this.json("/api/actor-groups", { method: "POST", body: { name } }) as Promise<{ id: string; name: string; actor_ids?: string[]; members?: string[] }>; }
  async assignActorGroupMember(groupId: string, actorId: string): Promise<void> { await this.json(\`/api/actor-groups/\${encodeURIComponent(groupId)}/members\`, { method: "POST", body: { actorId } }); }
  async removeActorGroupMember(groupId: string, actorId: string): Promise<void> { await this.json(\`/api/actor-groups/\${encodeURIComponent(groupId)}/members/\${encodeURIComponent(actorId)}\`, { method: "DELETE", body: {} }); }
  async getFluctlight(fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}\`) as Promise<Record<string, unknown>>; }
  async updateActorUserBackground(fluctlightId: string, body: BrowserActorUserBackgroundUpdate): Promise<BrowserActorUserState> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/actor-user-background\`, {method:"PUT",body}) as Promise<BrowserActorUserState>; }
  async goals(fluctlightId: string, options: {history?: boolean; limit?: number; cursor?: string} = {}): Promise<BrowserGoalPage> { const query = new URLSearchParams({history:String(options.history??false),limit:String(options.limit??20)}); if(options.cursor) query.set("cursor",options.cursor); return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goals?\${query}\`) as Promise<BrowserGoalPage>; }
  async goalSet(fluctlightId:string):Promise<BrowserGoalSet>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-set\`) as Promise<BrowserGoalSet>;}
  async updateGoalSet(fluctlightId:string,body:BrowserGoalSetCommand):Promise<BrowserGoalSet>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-set\`,{method:"PUT",body}) as Promise<BrowserGoalSet>;}
  async goalPlanningHistory(fluctlightId:string):Promise<Record<string,unknown>[]>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-planning\`) as Promise<Record<string,unknown>[]>;}
  async requestGoalPlanning(fluctlightId:string,idempotencyKey:string){return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-planning\`,{method:"POST",body:{idempotencyKey}});}
  async actorContext(fluctlightId:string,actorId:string):Promise<BrowserActorContext>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/actor-context/\${encodeURIComponent(actorId)}\`) as Promise<BrowserActorContext>;}
  async updateActorContext(fluctlightId:string,actorId:string,body:BrowserActorContextUpdate):Promise<BrowserActorContext>{return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/actor-context/\${encodeURIComponent(actorId)}\`,{method:"PUT",body}) as Promise<BrowserActorContext>;}
  async goalDetail(fluctlightId: string, goalId: string): Promise<BrowserGoal> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}\`) as Promise<BrowserGoal>; }
  async goalHistory(fluctlightId: string, goalId: string, cursor = ""): Promise<BrowserGoalHistoryPage> { const query = new URLSearchParams({limit:"20"}); if(cursor) query.set("cursor",cursor); return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}/history?\${query}\`) as Promise<BrowserGoalHistoryPage>; }
  async goalEvidence(fluctlightId: string, goalId: string, cursor = ""): Promise<BrowserGoalHistoryPage> { const query = new URLSearchParams({limit:"20"}); if(cursor) query.set("cursor",cursor); return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}/evidence?\${query}\`) as Promise<BrowserGoalHistoryPage>; }
  async createGoal(fluctlightId: string, body: BrowserGoalCommand): Promise<BrowserGoalCommandResult> {return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goals\`,{method:"POST",body}) as Promise<BrowserGoalCommandResult>;}
  async goalCommand(fluctlightId: string, goalId: string, operation: "update"|"pause"|"resume"|"cancel"|"abandon"|"reassess", body: BrowserGoalCommand): Promise<BrowserGoalCommandResult> {const suffix = operation === "update" ? "" : "/"+operation;return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}\${suffix}\`,{method:operation === "update" ? "PUT" : "POST",body}) as Promise<BrowserGoalCommandResult>;}
  async detail(fluctlightId: string): Promise<BrowserFluctlightDetail> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/detail\`) as Promise<BrowserFluctlightDetail>; }
  async wardrobe(fluctlightId: string, cursor = ""): Promise<BrowserWardrobePage> { const query = cursor ? \`?cursor=\${encodeURIComponent(cursor)}\` : ""; return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/wardrobe\${query}\`) as Promise<BrowserWardrobePage>; }
  async addWardrobeItems(fluctlightId: string, body: Record<string, unknown>): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/wardrobe/items\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async updateWardrobeItem(fluctlightId: string, itemId: string, body: Record<string, unknown>): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/wardrobe/items/\${encodeURIComponent(itemId)}\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async deleteWardrobeItem(fluctlightId: string, itemId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/wardrobe/items/\${encodeURIComponent(itemId)}\`, { method: "DELETE", body: {} }) as Promise<Record<string, unknown>>; }
  async triggerWakeUp(fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/wake-up\`, { method: "POST", body: {} }) as Promise<Record<string, unknown>>; }
  async developingSelf(fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/developing-self\`) as Promise<Record<string, unknown>>; }
  async rollbackDevelopingSelf(fluctlightId: string, claimId: string, body: BrowserFoundationDecisionBody): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/developing-self/\${encodeURIComponent(claimId)}/rollback\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async forgetDevelopingSelf(fluctlightId: string, claimId: string, body: BrowserFoundationDecisionBody): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/developing-self/\${encodeURIComponent(claimId)}/forget\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async setStatus(fluctlightId: string, body: { status: "active" | "paused"; expectedRevision: number; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/status\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async retireFluctlight(fluctlightId: string, body: BrowserFoundationDecisionBody): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/retire\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async submitFoundationRevision(fluctlightId: string, body: BrowserFoundationProposalBody): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async acceptFoundationRevision(fluctlightId: string, revisionId: string, body: BrowserFoundationDecisionBody): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/\${encodeURIComponent(revisionId)}/accept\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async rejectFoundationRevision(fluctlightId: string, revisionId: string, body: BrowserFoundationDecisionBody): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/\${encodeURIComponent(revisionId)}/reject\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async rollbackFoundationRevision(fluctlightId: string, body: BrowserFoundationRestoreBody): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/rollback\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async conversationDirectory(fluctlightId: string, options: { limit?: number; cursor?: string } = {}): Promise<BrowserConversationDirectoryPage> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 50) }); if(options.cursor) query.set("cursor",options.cursor);
    return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/conversations?\${query}\`) as Promise<BrowserConversationDirectoryPage>;
  }
  async newDirectConversation(fluctlightId: string, body: BrowserNewDirectConversationRequest): Promise<BrowserConversationPage> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/conversations\`,{method:"POST",body}) as Promise<BrowserConversationPage>; }
  async selectDirectConversation(fluctlightId: string, conversationId: string): Promise<BrowserConversationPage> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/conversations/\${encodeURIComponent(conversationId)}/select\`,{method:"POST",body:{}}) as Promise<BrowserConversationPage>; }
  async searchConversationMessages(fluctlightId: string, q: string, options: {limit?:number;cursor?:string}={}): Promise<BrowserConversationSearchPage> {
    const query = new URLSearchParams({q,limit:String(options.limit??50)}); if(options.cursor) query.set("cursor",options.cursor);
    return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/messages/search?\${query}\`) as Promise<BrowserConversationSearchPage>;
  }
  async conversationMessage(conversationId:string,messageId:string): Promise<BrowserMessage> { return this.json(\`/api/conversations/\${encodeURIComponent(conversationId)}/messages/\${encodeURIComponent(messageId)}\`) as Promise<BrowserMessage>; }
  async directConversation(fluctlightId: string): Promise<BrowserConversationPage> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/conversation\`) as Promise<BrowserConversationPage>; }
  async moments(fluctlightId: string, includeHidden = false): Promise<Array<{ id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null }>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/moments?includeHidden=\${includeHidden}\`) as Promise<Array<{ id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null }>>; }
  async globalMoments(includeHidden = false): Promise<Array<{ id: string; owner_fluctlight_id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null; unread_count?: number }>> { return this.json(\`/api/moments?includeHidden=\${includeHidden}\`) as Promise<Array<{ id: string; owner_fluctlight_id: string; text: string; author_actor_id: string; created_at: string; media_asset_ids: string[]; media: Array<{ id: string; kind: string; mime_type: string }>; status: string; comments: Array<{ id: string; author_actor_id: string; text: string; created_at: string }>; reaction_count: number; viewer_reaction?: string | null; unread_count?: number }>>; }
  async markMomentsRead(fluctlightId: string): Promise<void> { await this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/moments/read\`, { method: "POST", body: {} }); }
  async reviseMemory(memoryId: string, body: { expectedRevision: number; content: string; evidenceRefs: string[] }): Promise<Record<string, unknown>> { return this.json(\`/api/memories/\${encodeURIComponent(memoryId)}\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async forgetMemory(memoryId: string, body: { expectedRevision: number; evidenceRefs: string[] }): Promise<Record<string, unknown>> { return this.json(\`/api/memories/\${encodeURIComponent(memoryId)}/forget\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async rollbackRelationship(fluctlightId: string, body: { targetActorId: string; profileId?: string; targetRevision: number; expectedRevision: number; evidenceRefs: string[] }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/relationships/rollback\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async editRelationship(fluctlightId: string, targetActorId: string, body: { profileId?: string; expectedRevision: number; role?: Record<string, unknown>; metrics?: Record<string, unknown>; trend?: string; summary?: string; emotionalAssociation?: Record<string, unknown>; evidenceRefs: string[]; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/relationships/\${encodeURIComponent(targetActorId)}\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async listAutonomyActions(fluctlightId: string): Promise<Array<{ id: string; action_type: string; status: string; workflow_id: string; created_at: string }>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/autonomy-actions\`) as Promise<Array<{ id: string; action_type: string; status: string; workflow_id: string; created_at: string }>>; }
  async governAutonomyAction(actionId: string, body: { status: "paused" | "deferred" | "cancelled"; reason: string }): Promise<Record<string, unknown>> { return this.json(\`/api/autonomy-actions/\${encodeURIComponent(actionId)}/govern\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async createLifeEvent(fluctlightId: string, body: BrowserLifeEventRequest): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/events\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async cancelLifeEvent(fluctlightId: string, eventId: string, body: BrowserLifeEventCancelRequest): Promise<void> { await this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/events/\${encodeURIComponent(eventId)}/cancel\`, { method: "POST", body }); }
  async setLifePresence(fluctlightId: string, body: BrowserPresenceRequest): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/presence\`, { method: "PUT", body }) as Promise<Record<string, unknown>>; }
  async acceptLifeSchedule(fluctlightId: string, body: BrowserScheduleRequest): Promise<Record<string, unknown>> { return this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/schedules\`, { method: "POST", body }) as Promise<Record<string, unknown>>; }
  async cancelLifeSchedule(fluctlightId: string, scheduleId: string, body: BrowserScheduleCancelRequest): Promise<void> { await this.json(\`/api/fluctlights/\${encodeURIComponent(fluctlightId)}/schedules/\${encodeURIComponent(scheduleId)}/cancel\`, { method: "POST", body }); }
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
  async cancelTurn(conversationId: string, turnId: string): Promise<void> {
    await this.json(\`/api/conversations/\${encodeURIComponent(conversationId)}/turn/\${encodeURIComponent(turnId)}/cancel\`, { method: "POST", body: {} });
  }
  async turn(conversationId: string, body: { text: string; fluctlightId: string; senderActorId?: string; attachmentRefs?: string[]; idempotencyKey: string; turnId?: string; senderTimezone?: string; senderUtcOffsetMinutes?: number; senderSentAt?: string }, signal?: AbortSignal): Promise<Response> {
    const response = await this.fetcher(this.url(\`/api/conversations/\${encodeURIComponent(conversationId)}/turn\`), {
      method: "POST", credentials: "include", headers: { "content-type": "application/json", accept: "application/x-ndjson", ...this.csrfHeaders() }, body: JSON.stringify(body), signal,
    });
    if (!response.ok) {
      const payload = await response.json().catch(() => null) as { code?: unknown; message?: unknown; details?: unknown; detail?: unknown } | null;
      const detail = payload?.detail && typeof payload.detail === "object" && !Array.isArray(payload.detail)
        ? payload.detail as { code?: unknown; message?: unknown; details?: unknown }
        : null;
      const code = typeof payload?.code === "string" ? payload.code : typeof detail?.code === "string" ? detail.code : "browser_request_failed";
      const message = typeof payload?.message === "string" ? payload.message : typeof detail?.message === "string" ? detail.message : \`Browser conversation turn failed: \${response.status}\`;
      const rawDetails = payload?.details ?? detail?.details;
      const details = rawDetails && typeof rawDetails === "object" && !Array.isArray(rawDetails)
        ? rawDetails as Record<string, unknown>
        : {};
      throw new BrowserApiError(response.status, code, message, details);
    }
    return response;
  }
  async diagnostics(options: { limit?: number; correlationId?: string; fluctlightId?: string } = {}): Promise<BrowserDiagnosticEvent[]> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlationId) query.set("correlationId", options.correlationId);
    if (options.fluctlightId) query.set("fluctlightId", options.fluctlightId);
    return this.json(\`/api/diagnostics?\${query}\`) as Promise<BrowserDiagnosticEvent[]>;
  }
  async lifecycleDiagnostics(options: BrowserLifecycleDiagnosticsFilter = {}): Promise<BrowserLifecycleDiagnosticsPage> {
    return this.json(\`/api/diagnostics/lifecycle?\${lifecycleDiagnosticsQuery(options, 100)}\`) as Promise<BrowserLifecycleDiagnosticsPage>;
  }
  async diagnosticModelRuns(options: { limit?: number; correlationId?: string; cursor?: string } = {}): Promise<BrowserDiagnosticRunPage<BrowserDiagnosticModelRun>> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlationId) query.set("correlationId", options.correlationId);
    if (options.cursor) query.set("cursor", options.cursor);
    return this.json(\`/api/diagnostics/model-runs?\${query}\`) as Promise<BrowserDiagnosticRunPage<BrowserDiagnosticModelRun>>;
  }
  async diagnosticAgentRuns(options: { limit?: number; correlationId?: string; cursor?: string } = {}): Promise<BrowserDiagnosticRunPage<BrowserDiagnosticAgentRun>> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlationId) query.set("correlationId", options.correlationId);
    if (options.cursor) query.set("cursor", options.cursor);
    return this.json(\`/api/diagnostics/agent-runs?\${query}\`) as Promise<BrowserDiagnosticRunPage<BrowserDiagnosticAgentRun>>;
  }
  async kevDecisions(options: Record<string,string> = {}): Promise<{items: Array<Record<string,unknown>>; next_cursor: string; snapshot: string}> {
    const query = new URLSearchParams(options);
    return this.json(\`/api/diagnostics/kev-decisions?\${query}\`) as Promise<{items: Array<Record<string,unknown>>; next_cursor:string; snapshot:string}>;
  }
  async testVisionUnderstanding(): Promise<Record<string,unknown>> { return this.json("/api/settings/vision/test",{method:"POST",body:{}}) as Promise<Record<string,unknown>>; }
  async testKevConnection(): Promise<Record<string,unknown>> { return this.json("/api/settings/kev/test-connection",{method:"POST",body:{}}) as Promise<Record<string,unknown>>; }
  async diagnosticMediaPrompts(options: { limit?: number } = {}): Promise<BrowserDiagnosticMediaPrompt[]> {
    const limit = Math.min(Math.max(options.limit ?? 20, 1), 20);
    return this.json(\`/api/diagnostics/media-prompts?limit=\${limit}\`) as Promise<BrowserDiagnosticMediaPrompt[]>;
  }
  async retryDiagnosticMediaPrompt(mediaIntentId: string): Promise<Record<string, unknown>> { return this.json(\`/api/diagnostics/media-prompts/\${encodeURIComponent(mediaIntentId)}/retry\`, { method: "POST", body: {} }) as Promise<Record<string, unknown>>; }
  async exportDiagnostics(options: BrowserLifecycleDiagnosticsFilter = {}): Promise<Record<string, unknown>> {
    return this.json(\`/api/diagnostics/export?\${lifecycleDiagnosticsQuery(options, 500)}\`) as Promise<Record<string, unknown>>;
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
  private async json(path: string, options: { method?: string; body?: unknown; timeoutMs?: number; anonymousSession?: boolean } = {}): Promise<unknown> {
    const method = options.method ?? "GET";
    const controller = new AbortController();
    const timeoutMs = options.timeoutMs ?? (method === "GET" ? this.readTimeoutMs : 0);
    let timer: ReturnType<typeof setTimeout> | undefined;
    const work = (async () => {
      const response = await this.fetcher(this.url(path), {
        method, credentials: "include", signal: controller.signal,
        headers: { ...(options.body ? { "content-type": "application/json" } : {}), ...(method !== "GET" ? this.csrfHeaders() : {}) },
        body: options.body ? JSON.stringify(options.body) : undefined,
      });
      if (options.anonymousSession && response.status === 401) return { authenticated: false };
      if (!response.ok) {
        const payload = await response.json().catch(() => null) as { code?: unknown; message?: unknown; details?: unknown } | null;
        const code = typeof payload?.code === "string" ? payload.code : "browser_request_failed";
        const message = typeof payload?.message === "string" ? payload.message : \`Browser request failed: \${response.status}\`;
        const details = payload?.details && typeof payload.details === "object" && !Array.isArray(payload.details) ? payload.details as Record<string, unknown> : {};
        throw new BrowserApiError(response.status, code, message, details);
      }
      if (response.status === 204) return undefined;
      return response.json();
    })();
    // Model-triggering writes keep their server-owned task/timeout lifecycle.
    if (timeoutMs <= 0) return work;
    const deadline = new Promise<never>((_, reject) => {
      timer = setTimeout(() => {
        reject(new BrowserApiError(0, "browser_request_timeout", "服务读取超时，请重试；原会话和已提交请求已保留。", { path, method, retryable: true }));
        controller.abort();
      }, timeoutMs);
    });
    try { return await Promise.race([work, deadline]); }
    finally { if (timer) clearTimeout(timer); }
  }

}

import { applyV3Contract } from "./v3-contract.ts";

applyV3Contract(BrowserClient);

export { currentRevision, studioApplyUnfinished } from "./v3-contract.ts";
export type { ManagedMemory, MemoryRevision, MemorySource, MemoryBrowseOptions, WorldCatalog, WorldCreateBody, WorldSelectBody, WorldDeleteBody, WorldDeleteResult, NearbyActors, RuntimeWorld, RuntimePlace, RuntimeNPC, NPCRoutineSlot, WorldCommand, WorldWriteBody, PlaceWriteBody, NPCWriteBody, NPCEncounterBody, WorldWriteResult, StudioResource, StudioCatalogCommand, StudioCopyBody, StudioPublicationRecord, StudioPublication, StudioResourceQuery, StudioResourceWriteBody, ApplyPreviewResponse, ApplyResourceBody, ApplyResourceResponse, OwnerRequest, OwnerRequestStatus, OwnerRequestEnvelope, CursorPage, CreateMomentBody, CreateMomentResponse, SceneImage, EvidenceState, AutonomyPolicy, DiaryEntry, ReflectionDetail, WorldView, StudioPreview, HarmIntercepts, DiagnosticTask, MessageIdentity, MessageViewFields, MomentAuthorFields } from "./v3-contract.ts";
`;
await writeFile(fileURLToPath(new URL("src/index.ts", root)), source);
