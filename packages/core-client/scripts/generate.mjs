import {scheduleOwnerSchemas,scheduleOwnerPaths} from '../../browser-client/scripts/schedule-owner-contract.mjs';
import {placeDefaultSchemas,placeDefaultPaths} from '../../browser-client/scripts/place-default-contract.mjs';
import {dataToolsSchemas,dataToolsPaths} from '../../browser-client/scripts/data-tools-contract.mjs';
import {diagnosticRecordSchemas,diagnosticRecordPaths} from '../../browser-client/scripts/diagnostic-record-contract.mjs';
import {configurationSchemas,configurationPaths} from '../../browser-client/scripts/configuration-contract.mjs';
import {momentSchemas,momentPaths} from '../../browser-client/scripts/moment-contract.mjs';
import {mediaLibrarySchemas,mediaLibraryPaths} from '../../browser-client/scripts/media-library-contract.mjs';
import {worldItemSchemas,worldItemPaths} from '../../browser-client/scripts/world-item-contract.mjs';
import {studioCatalogSchemas,studioCatalogPaths} from '../../browser-client/scripts/studio-catalog-contract.mjs';
import {studioJobSchemas,studioJobPaths} from "../../browser-client/scripts/studio-job-contract.mjs";
import {worldEventSchemas,worldEventPaths} from "../../browser-client/scripts/world-event-contract.mjs";
import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import {operationsSchemas,operationsPaths} from '../../browser-client/scripts/operations-contract.mjs';
import {diarySchemas,diaryPaths} from '../../browser-client/scripts/diary-contract.mjs';
import {governanceSchemas,governancePaths} from "../../browser-client/scripts/governance-contract.mjs";
import {visualBriefSchemas,visualBriefPaths} from "../../browser-client/scripts/visual-brief-contract.mjs";
import {npcRuntimeSchemas,npcRuntimePaths} from "../../browser-client/scripts/npc-runtime-contract.mjs";
import {worldSchemas} from "../../browser-client/scripts/world-contract.mjs";
import {promiseSchemas,promisePaths} from "../../browser-client/scripts/promise-contract.mjs";
import {inventorySchemas,inventoryPaths} from "../../browser-client/scripts/inventory-contract.mjs";
import {selfSchemas,selfPaths} from '../../browser-client/scripts/self-contract.mjs';

const root = new URL("../", import.meta.url);
const schema = JSON.parse(await readFile(new URL("openapi.json", root), "utf8"));
const memoryRead=schema.paths["/internal/fluctlights/{fluctlight_id}/memories"]?.get;
if(memoryRead&&!memoryRead.parameters.some(p=>p.name==="aboutOwner"))memoryRead.parameters.push({name:"aboutOwner",in:"query",schema:{type:"boolean"}});
schema.components.schemas.DiagnosticModelRunResponse.properties.parent_run_id={type:'string'};
const snake=value=>value.replace(/[A-Z]/g,ch=>'_'+ch.toLowerCase());
const coreShape=(value,request=false)=>{
 if(Array.isArray(value))return value.map(v=>coreShape(v,request));
 if(!value||typeof value!=='object')return typeof value==='string'&&value.startsWith('#/components/schemas/Browser')?value.replace('/Browser','/Core'):value;
 return Object.fromEntries(Object.entries(value).map(([key,v])=>[key,key==='properties'&&request?Object.fromEntries(Object.entries(v).map(([field,shape])=>[snake(field),coreShape(shape,request)])):key==='required'&&request?v.map(snake):coreShape(v,request)]));
};
const deliverySchemas={...scheduleOwnerSchemas,...placeDefaultSchemas,...dataToolsSchemas,...diagnosticRecordSchemas,...configurationSchemas,...momentSchemas,...mediaLibrarySchemas,...worldItemSchemas,...studioCatalogSchemas,...operationsSchemas,...diarySchemas,...selfSchemas,...governanceSchemas,...visualBriefSchemas,...inventorySchemas,...promiseSchemas,...npcRuntimeSchemas,...worldEventSchemas,...studioJobSchemas, BrowserRuntimeNPC:worldSchemas.BrowserRuntimeNPC,BrowserNPCRoutineSlot:worldSchemas.BrowserNPCRoutineSlot,BrowserNearbyActors:worldSchemas.BrowserNearbyActors};
for(const [name,value]of Object.entries(deliverySchemas))schema.components.schemas[name.replace(/^Browser/,'Core')]=coreShape(value,name.endsWith('Body')||name.endsWith('Request')||name==='BrowserConfigurationWrite'||name==='BrowserConfigurationSaved');
for(const [path,methods]of Object.entries({...scheduleOwnerPaths,...placeDefaultPaths,...dataToolsPaths,...diagnosticRecordPaths,...configurationPaths,...momentPaths,...mediaLibraryPaths,...worldItemPaths,...studioCatalogPaths,...operationsPaths,...diaryPaths,...selfPaths,...governancePaths,...visualBriefPaths,...inventoryPaths,...promisePaths,...npcRuntimePaths,...worldEventPaths,...studioJobPaths})){
 const nativePath=path.replace('/api/','/internal/').replace(/\{([^}]+)\}/g,(_,name)=>'{'+snake(name)+'}');
 schema.paths[nativePath]={...schema.paths[nativePath],...coreShape(methods)};
 for(const operation of Object.values(schema.paths[nativePath]))if(operation.parameters)for(const parameter of operation.parameters)parameter.name=snake(parameter.name);
 for(const operation of Object.values(schema.paths[nativePath]))for(const [status,response]of Object.entries(operation.responses||{}))if(Number(status)>=400&&response.content?.['application/json'])response.content['application/json'].schema={$ref:'#/components/schemas/CoreGoalError'};
}
schema.paths['/internal/fluctlight-creations/analysis'].post.requestBody={required:true,content:{'application/json':{schema:{$ref:'#/components/schemas/CoreFluctlightCreationAnalysisRequest'}}}};
schema.components.schemas.CoreOwnerRequest.properties.status.enum=['pending','processing','accepted','rejected','deferred','awaiting_goal','cancelled','failed','completed'];
Object.assign(schema.components.schemas.CoreOwnerRequest.properties,{tasks_state:{type:'string',enum:['available','failed']},tasks:{type:'array',items:{$ref:'#/components/schemas/CoreRequestTask'}},tasks_error_code:{type:'string'}});
schema.paths['/internal/owner-requests/{request_id}']={get:{operationId:'readOwnerRequest',responses:{200:{description:'Authorized request state and actual workflow pointers',content:{'application/json':{schema:{$ref:'#/components/schemas/CoreOwnerRequest'}}}}}}};
await writeFile(new URL('openapi.json',root),JSON.stringify(schema,null,2)+'\n');
const operations = Object.keys(schema.paths).sort();
const goalSchemaType = (value) => {
  if (value?.$ref) return value.$ref.split("/").pop();
  if (value?.anyOf) return value.anyOf.map(goalSchemaType).join(" | ");
  if (value?.enum) return value.enum.map(JSON.stringify).join(" | ");
  if (value?.type === "string") return "string";
  if (value?.type === "integer" || value?.type === "number") return "number";
  if (value?.type === "boolean") return "boolean";
  if (value?.type === "null") return "null";
  if (value?.type === "array") return `Array<${goalSchemaType(value.items)}>`;
  if (value?.type === "object" || value?.properties) {
    const required = new Set(value.required ?? []);
    const fields = Object.entries(value.properties ?? {}).map(([key, child]) => `${key}${required.has(key) ? "" : "?"}: ${goalSchemaType(child)}`);
    if (!fields.length && value.additionalProperties) return "Record<string, unknown>";
    const object = `{ ${fields.join("; ")} }`;
    return value.additionalProperties ? `(${object} & Record<string, unknown>)` : object;
  }
  return "unknown";
};
const goalTypes = [...Object.keys(deliverySchemas).map(name=>name.replace(/^Browser/,'Core')),"CoreOwnerRequest","CoreGoalSet","CoreGoalSetCommand","CoreActorContext","CoreActorContextUpdate","CoreGoal", "CoreGoalPage", "CoreGoalRecordPage", "CoreGoalOwnerCommand", "CoreGoalCommandResult"].map(name => {
  if (!schema.components.schemas[name]) throw new Error(`Missing Goal schema ${name}`);
  return `export type ${name} = ${goalSchemaType(schema.components.schemas[name])};`;
}).join("\n");
const source = `// Generated from packages/core-client/openapi.json. Do not edit by hand.
export const coreOperations = ${JSON.stringify(operations)} as const;
${goalTypes}

export type CoreHealth = { status: string; role: string };
export type CoreSession = { authenticated: boolean; actorId?: string };
export type CoreSetupStatus = { setupAvailable: boolean };
export type CoreAuthenticatedSession = CoreSession & { sessionToken: string };
export type CoreSafeSettings = { values: Record<string, unknown>; configuredSecrets: string[]; versions: Record<string,number>; command?:Record<string,unknown> };
export type CoreProviderPreflight = { role: string; available: boolean; capability_version?: string };
export type CoreProviderModels = { endpoint_id: string; models: string[] };
export type CoreConversation = { id: string; created_by_actor_id: string; title?: string | null; revision: number; created_at: string; updated_at: string };
export type CoreParticipant = { conversation_id: string; actor_id: string; role: string; status: string; joined_at: string; left_at?: string | null };
export type CoreMessage = { id: string; conversation_id: string; sequence: number; author_actor_id: string; kind: string; text: string; attachment_refs: string[]; created_at: string; turn_id?: string; idempotency_key?: string; turn_status?: "pending" | "running" | "completed" | "failed" | "cancelled"; turn_error_code?: string; turn_retryable?: boolean };
export type CoreConversationPage = { conversation: CoreConversation; participants: CoreParticipant[]; messages: CoreMessage[]; next_before_sequence?: number | null };
export type CoreConversationCreate = { title?: string; participant_actor_ids: string[] };
export type CoreConversationTurn = { text: string; fluctlight_id: string; attachment_refs?: string[]; idempotency_key: string; turn_id?: string };
export type CoreFluctlight = { id: string; identity: Record<string, unknown>; status: string };
export type CoreDiagnosticEvent = { id: string; event_type: string; severity: string; fluctlight_id?: string | null; causation_id?: string | null; correlation_id: string; payload: Record<string, unknown>; created_at?: string | null };
export type CoreDiagnosticModelRun = { id: string; role: string; endpoint_id?: string | null; model_id: string; prompt: unknown; response?: unknown; status: string; goal_evaluation?: Record<string,unknown>; parent_run_id?:string; error_code?: string | null; correlation_id: string; created_at: string };
export type CoreDiagnosticRunPage<T> = {items:T[];next_cursor:string;snapshot:string};
export type CoreDiagnosticAgentRun = { fluctlight_id: string; agent_id: string; run_id: string; correlation_id: string; association_status: "linked" | "unknown"; status: string; source?: "agent_runs" | "termination_event"; failure_stage?: string; failure_code?: string; safe_cause?: string; started_at: string; finished_at?: string };
export type CoreActorUserSettings = { background: CoreActorContext["background"] };
export type CoreActorUserBackgroundUpdate = { background: CoreActorUserSettings["background"]; operation: "correct" | "change"; reason: string; expected_current_facts_revision: string; idempotency_key: string };
export type CoreFluctlightCreationAnalysis = { analysis_id: string; correlation_id: string; schema_version: number; core_persona: Record<string, unknown>; actor_user?: CoreActorUserSettings; developing_self: Record<string, unknown>; extensions: Record<string, unknown>; initial_goals: Array<Record<string, unknown>>; initial_intentions: Array<Record<string, unknown>>; initial_relationships: Array<Record<string, unknown>> };
export type CoreFluctlightActivationRequest = { request_id: string; initialization_mode: "blank_slate" | "llm_defined"; analysis_id?: string; schema_version?: number; name?: string | null; core_persona?: Record<string, unknown>; actor_user?: CoreActorUserSettings; developing_self?: Record<string, unknown>; extensions?: Record<string, unknown>; initial_goals?: Array<Record<string, unknown>>; initial_intentions?: Array<Record<string, unknown>>; initial_relationships?: Array<Record<string, unknown>> };
export type CoreLifecycleDiagnosticsFilter = { limit?: number; fluctlight_id?: string; correlation_id?: string; intent_id?: string; workflow_id?: string; run_id?: string; surface?: string; status?: string };
export type CoreLifecycleDiagnosticEvent = { id: string; event_type: string; surface: string; transition: string; severity: string; fluctlight_id?: string | null; correlation_id: string; causation_id?: string | null; intent_id?: string; workflow_id?: string; run_id?: string; stage: string; status: string; reason_code: string; retryable: boolean; metadata?: Record<string, unknown>; created_at: string };
export type CoreWorkflowIntentSnapshot = { intent_id: string; workflow_id: string; runtime_workflow_id: string; task_queue: string; intent_type: string; status: string; attempt_count: number; fluctlight_id?: string | null; correlation_id: string; causation_id?: string | null; last_error?: string | null; next_attempt_at?: string | null; started_at?: string | null; completed_at?: string | null; created_at: string };
export type CoreLifecycleDiagnosticsPage = { events: CoreLifecycleDiagnosticEvent[]; workflow_intents: CoreWorkflowIntentSnapshot[]; filters: Required<CoreLifecycleDiagnosticsFilter> };

const lifecycleDiagnosticsQuery = (options: CoreLifecycleDiagnosticsFilter, defaultLimit: number): string => {
  const query = new URLSearchParams({ limit: String(options.limit ?? defaultLimit) });
  if (options.fluctlight_id) query.set("fluctlight_id", options.fluctlight_id);
  if (options.correlation_id) query.set("correlation_id", options.correlation_id);
  if (options.intent_id) query.set("intent_id", options.intent_id);
  if (options.workflow_id) query.set("workflow_id", options.workflow_id);
  if (options.run_id) query.set("run_id", options.run_id);
  if (options.surface) query.set("surface", options.surface);
  if (options.status) query.set("status", options.status);
  return query.toString();
};

export class CoreApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: Record<string, unknown>;

  constructor(status: number, code: string, message?: string, details: Record<string, unknown> = {}) {
    super(message ?? \`Core request failed: \${status} \${code}\`);
    this.name = "CoreApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

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
  async setupStatus(): Promise<CoreSetupStatus> {
    const value = await this.json("/internal/auth/setup-status", undefined, "GET") as { setup_available?: boolean; setupAvailable?: boolean };
    return { setupAvailable: Boolean(value.setup_available ?? value.setupAvailable) };
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
  async listCapabilityRequests(humanSession: string): Promise<Array<Record<string, unknown>>> { return this.json("/internal/capability-requests", humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async reviewCapabilityRequest(humanSession: string, requestId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/capability-requests/\${encodeURIComponent(requestId)}/review\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async configureProviderEndpoint(humanSession: string, endpoint: object): Promise<void> { await this.provider("/internal/providers/endpoints", humanSession, endpoint); }
  async providerEndpoints(humanSession: string): Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; capability_status: string; roles: Array<{ role: string; model_id: string }> }>> { return this.json("/internal/providers/endpoints", humanSession, "GET") as Promise<Array<{ id: string; kind: string; base_url: string; secret_configured: boolean; capability_status: string; roles: Array<{ role: string; model_id: string }> }>>; }
  async providerEndpointModels(humanSession: string, endpointId: string): Promise<CoreProviderModels> { return this.json(\`/internal/providers/endpoints/\${encodeURIComponent(endpointId)}/models\`, humanSession, "GET") as Promise<CoreProviderModels>; }
  async providerBindings(humanSession: string): Promise<Array<Record<string, unknown>>> { return this.json("/internal/providers", humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async configureModelRole(humanSession: string, role: object): Promise<CoreProviderPreflight> { return this.provider("/internal/providers/roles", humanSession, role) as Promise<CoreProviderPreflight>; }
  async createConversation(humanSession: string, body: CoreConversationCreate): Promise<CoreConversationPage> { return this.json("/internal/conversations", humanSession, "POST", body) as Promise<CoreConversationPage>; }
  async createFluctlight(humanSession: string, body: { id?: string; name?: string }): Promise<CoreFluctlight> { return this.json("/internal/fluctlights", humanSession, "POST", body) as Promise<CoreFluctlight>; }
  async updateActorUserBackground(humanSession: string, fluctlightId: string, body: CoreActorUserBackgroundUpdate): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/actor-user-background\`, humanSession, "PUT", body) as Promise<Record<string, unknown>>; }
  async analyzeFluctlightCreation(humanSession: string, description: string): Promise<CoreFluctlightCreationAnalysis> { return this.json("/internal/fluctlight-creations/analysis", humanSession, "POST", { description }) as Promise<CoreFluctlightCreationAnalysis>; }
  async activateFluctlightCreation(humanSession: string, body: CoreFluctlightActivationRequest): Promise<Record<string, unknown>> { return this.json("/internal/fluctlight-creations/activate", humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async listFluctlights(humanSession: string): Promise<CoreFluctlight[]> { return this.json("/internal/fluctlights", humanSession, "GET") as Promise<CoreFluctlight[]>; }
  async listActorGroups(humanSession: string): Promise<Array<{ id: string; name: string; actor_ids: string[] }>> { return this.json("/internal/actor-groups", humanSession, "GET") as Promise<Array<{ id: string; name: string; actor_ids: string[] }>>; }
  async createActorGroup(humanSession: string, name: string): Promise<{ id: string; name: string; actor_ids: string[] }> { return this.json("/internal/actor-groups", humanSession, "POST", { name }) as Promise<{ id: string; name: string; actor_ids: string[] }>; }
  async assignActorGroupMember(humanSession: string, groupId: string, actorId: string): Promise<void> { await this.json(\`/internal/actor-groups/\${encodeURIComponent(groupId)}/members\`, humanSession, "POST", { actor_id: actorId }); }
  async removeActorGroupMember(humanSession: string, groupId: string, actorId: string): Promise<void> { await this.delete(\`/internal/actor-groups/\${encodeURIComponent(groupId)}/members/\${encodeURIComponent(actorId)}\`, humanSession); }
  async getFluctlight(humanSession: string, fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}\`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async goals(humanSession: string, fluctlightId: string, options: {history?: boolean; limit?: number; cursor?: string} = {}): Promise<CoreGoalPage> { const query = new URLSearchParams({history:String(options.history??false),limit:String(options.limit??20)}); if(options.cursor) query.set("cursor",options.cursor); return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goals?\${query}\`,humanSession,"GET") as Promise<CoreGoalPage>; }
  async goalSet(humanSession:string,fluctlightId:string):Promise<CoreGoalSet>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-set\`,humanSession,"GET") as Promise<CoreGoalSet>;}
  async updateGoalSet(humanSession:string,fluctlightId:string,body:CoreGoalSetCommand):Promise<CoreGoalSet>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-set\`,humanSession,"PUT",body) as Promise<CoreGoalSet>;}
  async actorContext(humanSession:string,fluctlightId:string,actorId:string):Promise<CoreActorContext>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/actor-context/\${encodeURIComponent(actorId)}\`,humanSession,"GET") as Promise<CoreActorContext>;}
  async updateActorContext(humanSession:string,fluctlightId:string,actorId:string,body:CoreActorContextUpdate):Promise<CoreActorContext>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/actor-context/\${encodeURIComponent(actorId)}\`,humanSession,"PUT",body) as Promise<CoreActorContext>;}
  async goalPlanningHistory(humanSession:string,fluctlightId:string):Promise<Record<string,unknown>[]>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-planning\`,humanSession,"GET") as Promise<Record<string,unknown>[]>;}
  async requestGoalPlanning(humanSession:string,fluctlightId:string,idempotencyKey:string){return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goal-planning\`,humanSession,"POST",{idempotency_key:idempotencyKey});}
  async goalDetail(humanSession: string, fluctlightId: string, goalId: string): Promise<CoreGoal> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}\`,humanSession,"GET") as Promise<CoreGoal>; }
  async goalHistory(humanSession: string, fluctlightId: string, goalId: string, cursor = ""): Promise<CoreGoalRecordPage> { const query = new URLSearchParams({limit:"20"}); if(cursor) query.set("cursor",cursor); return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}/history?\${query}\`,humanSession,"GET") as Promise<CoreGoalRecordPage>; }
  async goalEvidence(humanSession: string, fluctlightId: string, goalId: string, cursor = ""): Promise<CoreGoalRecordPage> { const query = new URLSearchParams({limit:"20"}); if(cursor) query.set("cursor",cursor); return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}/evidence?\${query}\`,humanSession,"GET") as Promise<CoreGoalRecordPage>; }
  async createGoal(humanSession: string, fluctlightId: string, body: CoreGoalOwnerCommand): Promise<CoreGoalCommandResult> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goals\`,humanSession,"POST",body) as Promise<CoreGoalCommandResult>; }
  async goalCommand(humanSession: string, fluctlightId: string, goalId: string, operation: "update"|"pause"|"resume"|"cancel"|"abandon"|"reassess", body: CoreGoalOwnerCommand & {expected_revision: number}): Promise<CoreGoalCommandResult> { const suffix=operation==="update"?"":"/"+operation;return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/goals/\${encodeURIComponent(goalId)}\${suffix}\`,humanSession,operation==="update"?"PUT":"POST",body) as Promise<CoreGoalCommandResult>; }

  async fluctlightDetail(humanSession: string, fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/detail\`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async fluctlightWardrobe(humanSession: string, fluctlightId: string, cursor = ""): Promise<Record<string, unknown>> { const query = cursor ? \`?cursor=\${encodeURIComponent(cursor)}\` : ""; return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/wardrobe\${query}\`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async triggerWakeUp(humanSession: string, fluctlightId: string): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/wake-up\`, humanSession, "POST", {}) as Promise<Record<string, unknown>>; }
  async setFluctlightStatus(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/status\`, humanSession, "PUT", body) as Promise<Record<string, unknown>>; }
  async retireFluctlight(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/retire\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async submitFoundationRevision(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async acceptFoundationRevision(humanSession: string, fluctlightId: string, revisionId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/\${encodeURIComponent(revisionId)}/accept\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async rejectFoundationRevision(humanSession: string, fluctlightId: string, revisionId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/\${encodeURIComponent(revisionId)}/reject\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async rollbackFoundationRevision(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/foundation-revisions/rollback\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async fluctlightMoments(humanSession: string, fluctlightId: string, includeHidden = false): Promise<Array<Record<string, unknown>>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/moments?include_hidden=\${includeHidden}\`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async globalMoments(humanSession: string, includeHidden = false): Promise<Array<Record<string, unknown>>> { return this.json(\`/internal/moments?include_hidden=\${includeHidden}\`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async markFluctlightMomentsRead(humanSession: string, fluctlightId: string): Promise<void> { await this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/moments/read\`, humanSession, "POST", {}); }
  async reviseMemory(humanSession: string, memoryId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/memories/\${encodeURIComponent(memoryId)}\`, humanSession, "PUT", body) as Promise<Record<string, unknown>>; }
  async forgetMemory(humanSession: string, memoryId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/memories/\${encodeURIComponent(memoryId)}/forget\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async managedMemories(humanSession: string, fluctlightId: string, options: {q?:string;type?:string;status?:string;cursor?:string;limit?:number} = {}): Promise<Record<string,unknown>> { const query=new URLSearchParams(Object.entries(options).filter(([,v])=>v!==undefined).map(([k,v])=>[k,String(v)]));return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/memories?\${query}\`,humanSession,"GET") as Promise<Record<string,unknown>>; }
  async managedMemory(humanSession: string, memoryId: string): Promise<Record<string,unknown>> {return this.json(\`/internal/memories/\${encodeURIComponent(memoryId)}\`,humanSession,"GET") as Promise<Record<string,unknown>>;}
  async memoryRevisions(humanSession: string, memoryId: string, cursor = ""): Promise<Record<string,unknown>> {return this.json(\`/internal/memories/\${encodeURIComponent(memoryId)}/revisions?cursor=\${encodeURIComponent(cursor)}\`,humanSession,"GET") as Promise<Record<string,unknown>>;}
  async memorySources(humanSession: string, memoryId: string, revision: number, cursor = ""): Promise<Record<string,unknown>> {return this.json(\`/internal/memories/\${encodeURIComponent(memoryId)}/sources?revision=\${revision}&cursor=\${encodeURIComponent(cursor)}\`,humanSession,"GET") as Promise<Record<string,unknown>>;}
  async rollbackMemory(humanSession: string, memoryId: string, body: object): Promise<Record<string,unknown>> {return this.json(\`/internal/memories/\${encodeURIComponent(memoryId)}/rollback\`,humanSession,"POST",body) as Promise<Record<string,unknown>>;}
  async reflections(humanSession:string,fluctlightId:string,cursor=""):Promise<Record<string,unknown>>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/reflections?cursor=\${encodeURIComponent(cursor)}\`,humanSession,"GET") as Promise<Record<string,unknown>>;}
  async reflection(humanSession:string,fluctlightId:string,reflectionId:string):Promise<Record<string,unknown>>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/reflections/\${encodeURIComponent(reflectionId)}\`,humanSession,"GET") as Promise<Record<string,unknown>>;}
  async reflectionFeedback(humanSession:string,fluctlightId:string,reflectionId:string,cursor=""):Promise<Record<string,unknown>>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/reflections/\${encodeURIComponent(reflectionId)}/feedback?cursor=\${encodeURIComponent(cursor)}\`,humanSession,"GET") as Promise<Record<string,unknown>>;}
  async submitReflectionFeedback(humanSession:string,fluctlightId:string,reflectionId:string,body:{text:string;stance?:"agree"|"disagree";idempotency_key:string}):Promise<Record<string,unknown>>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/reflections/\${encodeURIComponent(reflectionId)}/feedback\`,humanSession,"POST",body) as Promise<Record<string,unknown>>;}
  async rollbackRelationship(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/relationships/rollback\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async listAutonomyActions(humanSession: string, fluctlightId: string): Promise<Array<Record<string, unknown>>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/autonomy-actions\`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async governAutonomyAction(humanSession: string, actionId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/autonomy-actions/\${encodeURIComponent(actionId)}/govern\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async createLifeEvent(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/events\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async cancelLifeEvent(humanSession: string, fluctlightId: string, eventId: string, body: object): Promise<void> { await this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/events/\${encodeURIComponent(eventId)}/cancel\`, humanSession, "POST", body); }
  async setLifePresence(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/presence\`, humanSession, "PUT", body) as Promise<Record<string, unknown>>; }
  async schedules(humanSession:string,fid:string,options:{from?:string;to?:string}={}):Promise<CoreScheduleCalendar>{const q=new URLSearchParams();for(const [key,value]of Object.entries(options))if(value)q.set(key,value);return this.json(\`/internal/fluctlights/\${encodeURIComponent(fid)}/schedules?\${q}\`,humanSession) as Promise<CoreScheduleCalendar>;}
  async requestSchedule(humanSession:string,fid:string,body:CoreOwnerScheduleBody):Promise<CoreScheduleRequestView>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fid)}/schedules/requests\`,humanSession,"POST",body) as Promise<CoreScheduleRequestView>;}
  async scheduleRequest(humanSession:string,fid:string,id:string):Promise<CoreScheduleRequestView>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fid)}/schedules/requests/\${encodeURIComponent(id)}\`,humanSession) as Promise<CoreScheduleRequestView>;}
  async acceptLifeSchedule(humanSession: string, fluctlightId: string, body: object): Promise<Record<string, unknown>> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/schedules\`, humanSession, "POST", body) as Promise<Record<string, unknown>>; }
  async cancelLifeSchedule(humanSession: string, fluctlightId: string, scheduleId: string, body: object): Promise<void> { await this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/schedules/\${encodeURIComponent(scheduleId)}/cancel\`, humanSession, "POST", body); }
  async listWorkflows(humanSession: string, query = ""): Promise<Array<Record<string, unknown>>> { return this.json(\`/internal/diagnostics/workflows?query=\${encodeURIComponent(query)}\`, humanSession, "GET") as Promise<Array<Record<string, unknown>>>; }
  async workflowStatus(humanSession: string, workflowId: string): Promise<Record<string, unknown>> { return this.json(\`/internal/diagnostics/workflows/\${encodeURIComponent(workflowId)}/status\`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async workflowHistory(humanSession: string, workflowId: string): Promise<Record<string, unknown>> { return this.json(\`/internal/diagnostics/workflows/\${encodeURIComponent(workflowId)}/history\`, humanSession, "GET") as Promise<Record<string, unknown>>; }
  async workflowCommand(humanSession: string, workflowId: string, action: "pause" | "resume" | "cancel"): Promise<void> { await this.json(\`/internal/diagnostics/workflows/\${encodeURIComponent(workflowId)}/\${action}\`, humanSession, "POST", {}); }
  async resetWorkflow(humanSession: string, workflowId: string, historyPoint: number): Promise<Record<string, unknown>> { return this.json(\`/internal/diagnostics/workflows/\${encodeURIComponent(workflowId)}/reset\`, humanSession, "POST", { history_point: historyPoint }) as Promise<Record<string, unknown>>; }
  async restartWorkflow(humanSession: string, workflowId: string): Promise<Record<string, unknown>> { return this.json(\`/internal/diagnostics/workflows/\${encodeURIComponent(workflowId)}/restart\`, humanSession, "POST", {}) as Promise<Record<string, unknown>>; }
  async commentOnMoment(humanSession: string, momentId: string, text: string): Promise<Record<string, unknown>> { return this.json(\`/internal/moments/\${encodeURIComponent(momentId)}/comments\`, humanSession, "POST", { text }) as Promise<Record<string, unknown>>; }
  async reactToMoment(humanSession: string, momentId: string, kind: string): Promise<Record<string, unknown>> { return this.json(\`/internal/moments/\${encodeURIComponent(momentId)}/reactions\`, humanSession, "POST", { kind }) as Promise<Record<string, unknown>>; }
  async setMomentStatus(humanSession: string, momentId: string, action: "hide" | "restore"): Promise<void> { await this.json(\`/internal/moments/\${encodeURIComponent(momentId)}/\${action}\`, humanSession, "POST", {}); }
  async conversationDirectory(humanSession: string, fluctlightId: string, options: {limit?:number;cursor?:string}={}): Promise<{items:Array<{conversation:CoreConversation;unread_count:number;last_read_sequence:number;last_delivered_sequence:number;last_message?:CoreMessage}>;current_conversation_id:string;next_cursor:string}> {
    const query=new URLSearchParams({limit:String(options.limit??50)});if(options.cursor)query.set("cursor",options.cursor);
    return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/conversations?\${query}\`,humanSession,"GET") as Promise<{items:Array<{conversation:CoreConversation;unread_count:number;last_read_sequence:number;last_delivered_sequence:number;last_message?:CoreMessage}>;current_conversation_id:string;next_cursor:string}>;
  }
  async newDirectConversation(humanSession:string,fluctlightId:string,body:{idempotency_key:string;title?:string}):Promise<CoreConversationPage>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/conversations\`,humanSession,"POST",body) as Promise<CoreConversationPage>;}
  async selectDirectConversation(humanSession:string,fluctlightId:string,conversationId:string):Promise<CoreConversationPage>{return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/conversations/\${encodeURIComponent(conversationId)}/select\`,humanSession,"POST",{}) as Promise<CoreConversationPage>;}
  async searchConversationMessages(humanSession:string,fluctlightId:string,q:string,options:{limit?:number;cursor?:string}={}):Promise<{items:CoreMessage[];next_cursor:string}>{
    const query=new URLSearchParams({q,limit:String(options.limit??50)});if(options.cursor)query.set("cursor",options.cursor);
    return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/messages/search?\${query}\`,humanSession,"GET") as Promise<{items:CoreMessage[];next_cursor:string}>;
  }
  async conversationMessage(humanSession:string,conversationId:string,messageId:string):Promise<CoreMessage>{return this.json(\`/internal/conversations/\${encodeURIComponent(conversationId)}/messages/\${encodeURIComponent(messageId)}\`,humanSession,"GET") as Promise<CoreMessage>;}
  async fluctlightDirectConversation(humanSession: string, fluctlightId: string): Promise<CoreConversationPage> { return this.json(\`/internal/fluctlights/\${encodeURIComponent(fluctlightId)}/conversation\`, humanSession, "GET") as Promise<CoreConversationPage>; }
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
  async readLifecycleDiagnostics(humanSession: string, options: CoreLifecycleDiagnosticsFilter = {}): Promise<CoreLifecycleDiagnosticsPage> {
    return this.json(\`/internal/diagnostics/lifecycle?\${lifecycleDiagnosticsQuery(options, 100)}\`, humanSession, "GET") as Promise<CoreLifecycleDiagnosticsPage>;
  }
  async readDiagnostics(humanSession: string, options: { limit?: number; correlation_id?: string; fluctlight_id?: string } = {}): Promise<CoreDiagnosticEvent[]> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    if (options.fluctlight_id) query.set("fluctlight_id", options.fluctlight_id);
    const rows = await this.json(\`/internal/diagnostics?\${query}\`, humanSession, "GET") as Array<Record<string, unknown>>;
    return rows.map((row) => ({ id: String(row.id), event_type: String(row.event_type), severity: String(row.severity), fluctlight_id: row.fluctlight_id as string | null | undefined, causation_id: row.causation_id as string | null | undefined, correlation_id: String(row.correlation_id), payload: (row.payload ?? {}) as Record<string, unknown>, created_at: row.created_at as string | null | undefined }));
  }
  async readDiagnosticModelRuns(humanSession: string, options: { limit?: number; correlation_id?: string; cursor?: string } = {}): Promise<CoreDiagnosticRunPage<CoreDiagnosticModelRun>> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    if (options.cursor) query.set("cursor", options.cursor);
    return this.json(\`/internal/diagnostics/model-runs?\${query}\`, humanSession, "GET") as Promise<CoreDiagnosticRunPage<CoreDiagnosticModelRun>>;
  }
  async readDiagnosticAgentRuns(humanSession: string, options: { limit?: number; correlation_id?: string; cursor?: string } = {}): Promise<CoreDiagnosticRunPage<CoreDiagnosticAgentRun>> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 100) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    if (options.cursor) query.set("cursor", options.cursor);
    return this.json(\`/internal/diagnostics/agent-runs?\${query}\`, humanSession, "GET") as Promise<CoreDiagnosticRunPage<CoreDiagnosticAgentRun>>;
  }
  async exportDiagnostics(humanSession: string, options: { limit?: number; correlation_id?: string } = {}): Promise<Record<string, unknown>> {
    const query = new URLSearchParams({ limit: String(options.limit ?? 500) });
    if (options.correlation_id) query.set("correlation_id", options.correlation_id);
    return this.json(\`/internal/diagnostics/export?\${query}\`, humanSession, "GET") as Promise<Record<string, unknown>>;
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
    if (!response.ok) {
      const payload = await response.json().catch(() => null) as { detail?: unknown; code?: unknown; message?: unknown; details?: unknown } | null;
      const detail = payload?.detail;
      const detailObject = detail && typeof detail === "object" && !Array.isArray(detail)
        ? detail as { code?: unknown; message?: unknown; details?: unknown }
        : null;
      const validationErrors = Array.isArray(detail) ? detail as Array<Record<string, unknown>> : [];
      const code: string = typeof detailObject?.code === "string"
        ? detailObject.code
        : typeof detail === "string"
          ? detail
          : typeof payload?.code === "string"
            ? payload.code
            : validationErrors.length
              ? "core_request_validation_failed"
              : "core_request_failed";
      const details: Record<string, unknown> = detailObject?.details && typeof detailObject.details === "object" && !Array.isArray(detailObject.details)
        ? detailObject.details as Record<string, unknown>
        : validationErrors.length
          ? { validation_errors: validationErrors }
          : payload?.details && typeof payload.details === "object" && !Array.isArray(payload.details)
            ? payload.details as Record<string, unknown>
            : {};
      const message = typeof detailObject?.message === "string"
        ? detailObject.message
        : typeof payload?.message === "string"
          ? payload.message
          : validationErrors.length
            ? "Core request validation failed"
            : undefined;
      throw new CoreApiError(response.status, code, message, details);
    }
    if (response.status === 204) return undefined;
    return response.json();
  }
  private async delete(path: string, humanSession: string): Promise<void> {
    const response = await this.fetcher(new URL(path, this.baseUrl), { method: "DELETE", headers: { "x-fluctlight-service-key": this.serviceKey, "x-fluctlight-human-session": humanSession } });
    if (!response.ok) throw new Error(\`Core delete failed: \${response.status}\`);
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
    const row = value as { values?: Record<string, unknown>; configuredSecrets?: string[]; configured_secrets?: string[];versions?:Record<string,number>;command?:Record<string,unknown> };
    return { values: row.values ?? {}, configuredSecrets: row.configuredSecrets ?? row.configured_secrets ?? [],versions:row.versions??{},...(row.command?{command:row.command}:{}) };
  }
}
`;
await writeFile(fileURLToPath(new URL("src/index.ts", root)), source);
