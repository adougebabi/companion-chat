import {scheduleOwnerSchemas,scheduleOwnerPaths} from './schedule-owner-contract.mjs';
import {placeDefaultSchemas,placeDefaultPaths} from './place-default-contract.mjs';
import {dataToolsSchemas,dataToolsPaths} from './data-tools-contract.mjs';
import {diagnosticRecordSchemas,diagnosticRecordPaths} from './diagnostic-record-contract.mjs';
import {configurationSchemas,configurationPaths} from './configuration-contract.mjs';
import {momentSchemas,momentPaths} from './moment-contract.mjs';
import {mediaLibrarySchemas,mediaLibraryPaths,mediaUploadPath} from './media-library-contract.mjs';
import {worldItemSchemas,worldItemPaths} from './world-item-contract.mjs';
import {studioCatalogSchemas,studioCatalogPaths} from './studio-catalog-contract.mjs';
import {studioJobSchemas,studioJobPaths} from "./studio-job-contract.mjs";
import {worldEventSchemas,worldEventPaths} from "./world-event-contract.mjs";
import { writeFile } from "node:fs/promises";
import { worldSchemas, worldPaths } from "./world-contract.mjs";
import { memorySchemas, memoryPaths, managedMemoryRead } from "./memory-contract.mjs";
import { reflectionSchemas, reflectionPaths } from "./reflection-contract.mjs";
import { operationsSchemas, operationsPaths } from "./operations-contract.mjs";
import { diarySchemas, diaryPaths } from "./diary-contract.mjs";
import {governanceSchemas,governancePaths} from "./governance-contract.mjs";
import {visualBriefSchemas,visualBriefPaths} from "./visual-brief-contract.mjs";
import {inventorySchemas,inventoryPaths} from "./inventory-contract.mjs";
import {npcRuntimeSchemas,npcRuntimePaths} from "./npc-runtime-contract.mjs";
import {promiseSchemas,promisePaths} from "./promise-contract.mjs";
import { selfSchemas, selfPaths } from "./self-contract.mjs";

const requestBody = (schema) => ({
  requestBody: { required: true, content: { "application/json": { schema: { $ref: `#/components/schemas/${schema}` } } } },
});

const jsonResponse = (schema) => ({
  responses: { "200": { description: "Successful response", content: { "application/json": { schema: { $ref: `#/components/schemas/${schema}` } } } } },
});

const jsonObject = { type: "object", additionalProperties: true };
const jsonObjectArray = { type: "array", items: jsonObject };
const nullableString = { anyOf: [{ type: "string" }, { type: "null" }] };

const actorUserBackground = { type: "object", additionalProperties: false, properties: Object.fromEntries(["name","occupation","background","location_scope","location","timezone","relationship_distance","meeting_confirmed","interests","preferences","full_name","age","gender","personality","habits","relationship_description","shared_information","wants_you_to_know","rules"].map(key => [key, {anyOf: [key === "meeting_confirmed" ? {type:"boolean"} : {type:"string",minLength:1,maxLength:1024}, {type:"null"}]}])) };
const lifeCommandProperties = {
  expectedLifeContextRevision: { type: "string", minLength: 1, maxLength: 64 },
  idempotencyKey: { type: "string", minLength: 1, maxLength: 256 },
};

const schema = {
  openapi: "3.1.0",
  info: { title: "Fluctlight Browser Platform API", version: "0.1.0" },
  components: {
    schemas: {
      BrowserKevDecisionPage: { type:"object", additionalProperties:false, properties:{items:jsonObjectArray,next_cursor:{type:"string"},snapshot:{type:"string"}}, required:["items","next_cursor","snapshot"] },
      BrowserKevConnectionTest: { type:"object", additionalProperties:false, properties:{call_status:{type:"string"},policy_outcome:{type:"string"},decision_id:{type:"string"},config_version:{type:"integer"},error_code:{type:"string"}}, required:["call_status","policy_outcome","decision_id","config_version","error_code"] },
      BrowserGoal: {
        type:"object", additionalProperties:true,
        properties:{id:{type:"string"},desired_outcome:{type:"string"},motivation:{type:"string"},success_criteria:{type:"array",items:{type:"string"}},criterion_ids:{type:"array",items:{type:"string"}},criteria_version:{type:"integer"},revision:{type:"integer"},status:{type:"string",enum:["candidate","active","paused","completed","cancelled","abandoned"]},profile_id:nullableString,scope:{type:"string"},deadline:nullableString,deadline_policy:{type:"string",enum:["soft","hard"]},execution:jsonObject,stages:jsonObjectArray,commitments:jsonObjectArray,evaluations:jsonObjectArray,evidence:jsonObjectArray,evidence_next_cursor:{type:"string"},reviews:jsonObjectArray,resolutions:jsonObjectArray},
        required:["id","desired_outcome","motivation","success_criteria","criterion_ids","criteria_version","revision","status","scope","deadline_policy"]
      },
      BrowserGoalPage:{type:"object",additionalProperties:false,properties:{items:{type:"array",items:{$ref:"#/components/schemas/BrowserGoal"}},next_cursor:{type:"string"}},required:["items"]},
      BrowserGoalHistoryPage:{type:"object",additionalProperties:false,properties:{items:jsonObjectArray,next_cursor:{type:"string"}},required:["items"]},
      BrowserGoalSet: {type:"object",additionalProperties:true,properties:{revision:{type:"integer"},facts_revision:{type:"string"},active_count:{type:"integer"},max_active_goals:{type:"integer"},auto_planning_enabled:{type:"boolean"},ordering_mode:{type:"string",enum:["automatic","manual"]},capacity_violation:{type:"boolean"},goals:{type:"array",items:{$ref:"#/components/schemas/BrowserGoal"}},dependencies:jsonObjectArray},required:["revision","facts_revision","active_count","max_active_goals","auto_planning_enabled","ordering_mode","capacity_violation","goals","dependencies"]},
      BrowserGoalSetCommand:{type:"object",additionalProperties:false,properties:{expectedVersion:{type:"integer"},expectedFactsRevision:{type:"string"},idempotencyKey:{type:"string"},reason:{type:"string"},autoPlanningEnabled:{type:"boolean"},orderingMode:{type:"string",enum:["automatic","manual"]},maxActiveGoals:{type:"integer",minimum:1,maximum:5},order:{type:"array",items:{type:"string"}},dependencies:{type:"object",additionalProperties:{type:"array",items:{type:"string"}}},changes:jsonObjectArray,recoveryPolicy:{type:"object",additionalProperties:false,properties:{merge_window_seconds:{type:"integer",minimum:1,maximum:60},retry_backoff_seconds:{type:"integer",minimum:5,maximum:3600},max_attempts:{type:"integer",minimum:1,maximum:10},lease_seconds:{type:"integer",minimum:60,maximum:600},candidate_limit:{type:"integer",minimum:1,maximum:20}},required:["merge_window_seconds","retry_backoff_seconds","max_attempts","lease_seconds","candidate_limit"]},sourceReviews:jsonObjectArray,reviewedGoalIds:{type:"array",items:{type:"string"}}},required:["expectedVersion","expectedFactsRevision","idempotencyKey","reason"]},
      BrowserActorContext:{type:"object",additionalProperties:true,properties:{actor_id:{type:"string"},context_version:{type:"string"},background:actorUserBackground,facts:jsonObjectArray,history:jsonObjectArray,relationships:jsonObjectArray,relationship_history:jsonObjectArray},required:["actor_id","context_version","background","facts","history","relationships","relationship_history"]},
      BrowserActorContextUpdate:{type:"object",additionalProperties:false,properties:{expectedContextVersion:{type:"string",minLength:1,maxLength:64},idempotencyKey:{type:"string",minLength:1,maxLength:128},operation:{type:"string",enum:["correct","change"]},reason:{type:"string",minLength:1,maxLength:1000,description:"At most 1000 UTF-8 bytes"},background:actorUserBackground},required:["expectedContextVersion","idempotencyKey","operation","reason","background"]},
      BrowserGoalCommand:{type:"object",additionalProperties:false,properties:{targetActorId:{type:"string"},ownerProtected:{type:"boolean"},candidateOnly:{type:"boolean"},reviewPolicy:{type:"object",additionalProperties:false,properties:{missed_opportunity_threshold:{type:"integer",minimum:1,maximum:30},ineffective_attempt_threshold:{type:"integer",minimum:1,maximum:30}},required:["missed_opportunity_threshold","ineffective_attempt_threshold"]},resetReviewCounters:{type:"boolean"},expectedRevision:{type:"integer",minimum:0},idempotencyKey:{type:"string",minLength:1,maxLength:128},reason:{type:"string",minLength:1,maxLength:1000},profileId:{type:"string"},desiredOutcome:{type:"string",minLength:1,maxLength:2000},successCriteria:{type:"array",minItems:1,maxItems:16,items:{type:"string"}},motivation:{type:"string"},scope:{type:"string"},deadline:{type:"string",format:"date-time"},clearDeadline:{type:"boolean"},deadlinePolicy:{type:"string",enum:["soft","hard"]}},required:["idempotencyKey","reason"]},
      BrowserGoalCommandResult:{type:"object",additionalProperties:false,properties:{goal_id:{type:"string"},revision:{type:"integer"},status:{type:"string"},criteria_version:{type:"integer"},evaluation_request_id:{type:"string"}},required:["goal_id","revision","status"]},
      BrowserConversationTurnRequest: {
        type: "object", additionalProperties: false,
        properties: {
          text: { type: "string", minLength: 1, maxLength: 32000 },
          fluctlightId: { type: "string", minLength: 1, maxLength: 128 },
          senderActorId: { type: "string", minLength: 1, maxLength: 128 },
          attachmentRefs: { type: "array", maxItems: 16, items: { type: "string", minLength: 1, maxLength: 512 } },
          idempotencyKey: { type: "string", minLength: 1, maxLength: 256 },
          turnId: { type: "string", minLength: 1, maxLength: 256 },
          senderTimezone: { type: "string", minLength: 1, maxLength: 128, description: "IANA timezone at first submission; present with senderUtcOffsetMinutes and senderSentAt." },
          senderUtcOffsetMinutes: { type: "integer", minimum: -840, maximum: 840 },
          senderSentAt: { type: "string", format: "date-time" },
        },
        required: ["text", "fluctlightId", "idempotencyKey"],
      },
      BrowserConversationMessage: {
        type: "object", additionalProperties: false,
        properties: {
          id: { type: "string" }, sourceRef:{type:"string"}, conversationId: { type: "string" }, sequence: { type: "integer" },
          authorActorId: { type: "string" }, kind: { type: "string" }, text: { type: "string" },
          attachmentRefs: { type: "array", items: { type: "string" } }, createdAt: { type: "string", format: "date-time" },
          senderTimezone: nullableString, senderUtcOffsetMinutes: { anyOf: [{ type: "integer" }, { type: "null" }] }, senderSentAt: nullableString,
          turnId: { type: "string" }, idempotencyKey: { type: "string" }, turnStatus: { type: "string", enum:["pending","running","completed","failed","cancelled"] },
          turnErrorCode: { type: "string" }, turnRetryable: { type: "boolean" },
          identity: nullableString, actor: nullableString, recall: {}, memoryRef: {}, goalRef: {}, actionId: nullableString, link: {},
        },
        required: ["id", "conversationId", "sequence", "authorActorId", "kind", "text", "attachmentRefs", "createdAt"],
      },
      BrowserConversation: {type:"object",additionalProperties:false,properties:{id:{type:"string"},createdByActorId:{type:"string"},title:nullableString,revision:{type:"integer"},createdAt:{type:"string"},updatedAt:{type:"string"}},required:["id","createdByActorId","revision","createdAt","updatedAt"]},
      BrowserConversationDirectoryPage: {type:"object",additionalProperties:false,properties:{items:{type:"array",items:{type:"object",additionalProperties:false,properties:{conversation:{$ref:"#/components/schemas/BrowserConversation"},unreadCount:{type:"integer"},lastReadSequence:{type:"integer"},lastDeliveredSequence:{type:"integer"},lastMessage:{$ref:"#/components/schemas/BrowserConversationMessage"}},required:["conversation","unreadCount","lastReadSequence","lastDeliveredSequence"]}},currentConversationId:{type:"string"},nextCursor:{type:"string"}},required:["items","currentConversationId","nextCursor"]},
      BrowserConversationSearchPage: {type:"object",additionalProperties:false,properties:{items:{type:"array",items:{$ref:"#/components/schemas/BrowserConversationMessage"}},nextCursor:{type:"string"}},required:["items","nextCursor"]},
      BrowserNewDirectConversationRequest: {type:"object",additionalProperties:false,properties:{idempotencyKey:{type:"string",minLength:1,maxLength:256},title:{type:"string",maxLength:200}},required:["idempotencyKey"]},
      BrowserConversationPage: {
        type: "object", additionalProperties: false,
        properties: {
          conversation: jsonObject,
          participants: jsonObjectArray,
          messages: { type: "array", items: { $ref: "#/components/schemas/BrowserConversationMessage" } },
          nextBeforeSequence: { anyOf: [{ type: "integer" }, { type: "null" }] },
          long_term_revision_unchanged: { type: "boolean" },
        },
        required: ["conversation", "participants", "messages"],
      },
      BrowserDiagnosticToolSummary: {
        type: "object", additionalProperties: false,
        properties: { callId: { type: "string" }, capability: { type: "string" }, status: { type: "string" }, errorCode: { type: "string" } },
        required: ["callId", "capability", "status"],
      },
      BrowserDiagnosticModelRun: {
        type: "object", additionalProperties: false,
        properties: {
          id: { type: "string" }, role: { type: "string" }, bindingRole: { type: "string" }, scenario: { type: "string" }, status: { type: "string" },
          priority: { type: "integer" }, queuePendingCount: { type: "integer" }, queuePosition: { anyOf: [{ type: "integer" }, { type: "null" }] },
          endpointId: { anyOf: [{ type: "string" }, { type: "null" }] }, modelId: { type: "string" },
          errorCode: { anyOf: [{ type: "string" }, { type: "null" }] },
          correlationId: { type: "string" }, logicalRunId: { type: "string" }, parentRunId: {type:"string"}, modelCallId: { type: "string" },
          sequence: { anyOf: [{ type: "integer" }, { type: "null" }] }, roundCount: { type: "integer" }, stage: { type: "string" },
          toolSummaries: { type: "array", maxItems: 32, items: { $ref: "#/components/schemas/BrowserDiagnosticToolSummary" } },
          goalEvaluation:jsonObject,
          tools: {},
          prompt: {}, response: {}, orderingKey: {type:"string"}, createdAt: { type: "string", format: "date-time" },
          queuedAt: { type: "string", format: "date-time" }, startedAt: { anyOf: [{ type: "string", format: "date-time" }, { type: "null" }] }, completedAt: { anyOf: [{ type: "string", format: "date-time" }, { type: "null" }] },
        },
        required: ["id", "role", "modelId", "status", "correlationId", "prompt", "createdAt"],
      },
      BrowserDiagnosticModelRuns: { type: "object", additionalProperties: false, properties: {items:{type:"array",items:{$ref:"#/components/schemas/BrowserDiagnosticModelRun"}},nextCursor:{type:"string"},snapshot:{type:"string"}}, required:["items","nextCursor","snapshot"] },
      BrowserDiagnosticAgentRun: {
        type: "object", additionalProperties: false,
        properties: {
          fluctlightId: { type: "string" }, agentId: { type: "string" }, runId: { type: "string" },
          correlationId: { type: "string" }, associationStatus: { type: "string" }, status: { type: "string" }, source: { type: "string" },
          orderingKey: {type:"string"}, failureStage: { type: "string" }, failureCode: { type: "string" }, safeCause: { type: "string" },
          startedAt: { type: "string", format: "date-time" }, finishedAt: { type: "string", format: "date-time" },
        },
        required: ["fluctlightId", "agentId", "runId", "correlationId", "associationStatus", "status", "startedAt"],
      },
      BrowserDiagnosticAgentRuns: { type: "object", additionalProperties: false, properties: {items:{type:"array",items:{$ref:"#/components/schemas/BrowserDiagnosticAgentRun"}},nextCursor:{type:"string"},snapshot:{type:"string"}}, required:["items","nextCursor","snapshot"] },
      BrowserFluctlightCreateRequest: {
        type: "object",
        additionalProperties: false,
        properties: {
          id: { type: "string", minLength: 1, maxLength: 128 },
          name: { type: "string", maxLength: 256 },
        },
      },
      BrowserActorUserSettings: {type:"object",additionalProperties:false,properties:{background:actorUserBackground},required:["background"]},
      BrowserActorUserBackgroundUpdate: {type:"object",additionalProperties:false,properties:{background:actorUserBackground,operation:{type:"string",enum:["correct","change"]},reason:{type:"string",minLength:1,maxLength:500},idempotencyKey:{type:"string",minLength:1,maxLength:256},expectedCurrentFactsRevision:{type:"string",minLength:1,maxLength:64}},required:["background","operation","reason","idempotencyKey","expectedCurrentFactsRevision"]},
      BrowserActorUserState: {type:"object",additionalProperties:true,properties:{background:actorUserBackground,facts:jsonObjectArray,current_facts_revision:{type:"string"},replayed:{type:"boolean"}},required:["background","facts"]},
      BrowserFluctlightCreationAnalysis: {
        type: "object",
        additionalProperties: false,
        properties: {
          analysis_id: { type: "string", minLength: 1, maxLength: 128 },
          correlation_id: { type: "string", minLength: 1, maxLength: 256 },
          schema_version: { type: "integer", minimum: 1 },
          core_persona: jsonObject,
          developing_self: jsonObject,
          extensions: jsonObject,
          initial_goals: jsonObjectArray,
          initial_intentions: jsonObjectArray,
          initial_relationships: jsonObjectArray,
          actor_user: {$ref:"#/components/schemas/BrowserActorUserSettings"},
        },
        required: ["analysis_id", "correlation_id", "schema_version", "core_persona", "developing_self", "extensions", "initial_goals", "initial_intentions", "initial_relationships"],
      },
      BrowserFluctlightActivationRequest: {
        type: "object",
        additionalProperties: false,
        properties: {
          requestId: { type: "string", minLength: 1, maxLength: 256 },
          initializationMode: { type: "string", enum: ["blank_slate", "llm_defined"] },
          initializationTimezone: { type: "string", minLength: 1, maxLength: 128, description: "Device IANA timezone at activation, used only when no explicit or inferred identity timezone exists." },
          analysisId: { type: "string", minLength: 1, maxLength: 128 },
          schemaVersion: { type: "integer", minimum: 1 },
          name: { type: "string", maxLength: 256 },
          corePersona: jsonObject,
          developingSelf: jsonObject,
          extensions: jsonObject,
          initialGoals: jsonObjectArray,
          initialIntentions: jsonObjectArray,
          initialRelationships: jsonObjectArray,
          actorUser: {$ref:"#/components/schemas/BrowserActorUserSettings"},
        },
        required: ["requestId", "initializationMode"],
      },
      BrowserInitializationSource: {
        type: "object",
        additionalProperties: false,
        properties: {
          id: { type: "string" },
          correlation_id: { type: "string" },
          source_text: { type: "string" },
          source_digest: { type: "string" },
          provider_endpoint_id: nullableString,
          model_id: nullableString,
          prompt_version: { type: "string" },
          schema_version: { type: "string" },
          classification: { type: "string", enum: ["single", "multiple", "unknown"] },
          classification_evidence: jsonObject,
          field_derivations: jsonObject,
          coverage: jsonObject,
          structured_projection: jsonObject,
          projection_digest: { type: "string" },
          created_at: { type: "string", format: "date-time" },
          linked_at: { type: "string", format: "date-time" },
        },
        required: ["id", "correlation_id", "source_text", "source_digest", "prompt_version", "schema_version", "classification", "classification_evidence", "field_derivations", "coverage", "structured_projection", "projection_digest", "created_at", "linked_at"],
      },
      BrowserPersonalityRuntime:{type:"object",additionalProperties:false,properties:{active_profile_id:{type:"string"},previous_profile_id:{type:"string"},revision:{type:"integer"},switch_reason:{type:"string"},switched_at:{type:"string"},cooldown_until:{type:"string"}},required:["active_profile_id","revision"]},
      BrowserPersonaSwitchHistoryItem:{type:"object",additionalProperties:false,properties:{audit_id:{type:"string"},occurred_at:{type:"string"},active_profile_id:{type:"string"},previous_profile_id:{type:"string"},revision:{type:"integer"},switch_reason:{type:"string"},cooldown_until:{type:"string"},disposition:{type:"string"}},required:["audit_id","occurred_at"]},
      BrowserPersonaSwitchRequest: { type:"object", additionalProperties:false, properties:{targetPersonaId:{type:"string",minLength:1,maxLength:128},expectedRevision:{type:"integer",minimum:0,maximum:2147483647},expectedRuntimeRevision:{type:"integer",minimum:0,maximum:2147483647},reason:{type:"string",minLength:1,maxLength:1000},confirm:{type:"boolean",const:true},idempotencyKey:{type:"string",minLength:1,maxLength:256}},required:["targetPersonaId","expectedRevision","expectedRuntimeRevision","reason","confirm","idempotencyKey"] },
      BrowserPersonaSwitchResult: {type:"object",additionalProperties:false,properties:{request_id:{type:"string"},status:{type:"string",enum:["completed","pending"]},status_url:{type:"string"},source_fact_id:{type:"string"},disposition:{type:"string",enum:["applied","no_change","assessment_pending"]},revision:{type:"integer"},runtime_revision:{type:"integer"},active_profile_id:{type:"string"},previous_profile_id:{type:"string"},audit_id:{type:"string"},persona_action_audit_id:{anyOf:[{type:"string"},{type:"null"}]},request:{type:"object",additionalProperties:true}},required:["request_id","status","disposition","revision","runtime_revision","active_profile_id","previous_profile_id","audit_id","persona_action_audit_id","request"]},
      BrowserPersonaSwitchTerminal: {type:"object",additionalProperties:false,properties:{request_id:{type:"string"},status:{type:"string",enum:["completed","rejected","failed"]},disposition:{type:"string",enum:["applied","no_change"]},revision:{type:"integer"},runtime_revision:{type:"integer"},active_profile_id:{type:"string"},previous_profile_id:{type:"string"},audit_id:{type:"string"},persona_action_audit_id:{anyOf:[{type:"string"},{type:"null"}]},source_fact_id:{type:"string"},error_code:{type:"string"},retryable:{type:"boolean"}},required:["request_id","status"]},
      BrowserPersonaCompilation:{type:"object",additionalProperties:false,properties:{profile_id:{type:"string"},foundation_revision:{type:"integer"},overlay_revision:{type:"integer"},status:{type:"string",enum:["ready","missing","stale","invalid"]},working_persona:{anyOf:[jsonObject,{type:"null"}]},rules_version:{type:"string"},source_hash:{type:"string"},owner_switch_status:{type:"string",enum:["assessment_required"]},owner_switch_targets:{type:"array",items:{type:"object",additionalProperties:false,properties:{rule_id:{type:"string"},target_profile_id:{type:"string"}},required:["rule_id","target_profile_id"]}}},required:["profile_id","foundation_revision","overlay_revision","status","working_persona","owner_switch_targets","owner_switch_status"]},
      BrowserFluctlightDetail: {
        type: "object",
        additionalProperties: true,
        properties: {
          personality_runtime: {$ref:"#/components/schemas/BrowserPersonalityRuntime"},
          persona_compilation: {$ref:"#/components/schemas/BrowserPersonaCompilation"},
          persona_switch_history: {type:"array",items:{$ref:"#/components/schemas/BrowserPersonaSwitchHistoryItem"}},
          persona_switch_history_has_more: {type:"boolean"},
          actor_user: {$ref:"#/components/schemas/BrowserActorUserState"},
          initialization_source: { anyOf: [{ $ref: "#/components/schemas/BrowserInitializationSource" }, { type: "null" }] },
        },
      },
      BrowserWardrobeItem: {
        type: "object", additionalProperties: false,
        properties: {
          id: { type: "string" }, category: { type: "string" }, slot: { type: "string" },
          description: { type: "string" }, ownership: { type: "string" }, availability: { type: "string" },
          source_kind: { type: "string" }, revision: { type: "integer" },
          item_kind: {type:"string",enum:["wearable","object"]},
        },
        required: ["id", "category", "slot", "description", "ownership", "availability", "source_kind", "revision", "item_kind"],
      },
      BrowserWardrobePage: {
        type: "object", additionalProperties: false,
        properties: {
          operation: { type: "string", enum: ["list"] },
          revision: { type: "integer" }, inventory_complete: { type: "boolean" },
          items: { type: "array", items: { $ref: "#/components/schemas/BrowserWardrobeItem" } },
          has_more: { type: "boolean" }, next_cursor: { type: "string" },
          searched_scope: { type: "string" }, can_conclude_absent: { type: "boolean" },
          worn_items: jsonObjectArray, used_items: jsonObjectArray, owner_revision: { type: "integer" },
        },
        required: ["operation", "revision", "inventory_complete", "items", "has_more", "next_cursor", "searched_scope", "can_conclude_absent"],
      },
      BrowserLifeEventRequest: {
        type: "object",
        additionalProperties: false,
        properties: {
          kind: { type: "string", minLength: 1, maxLength: 128 },
          placeId:{type:"string",maxLength:128},
          startAt: { type: "string", format: "date-time" },
          endAt: { type: "string", format: "date-time" },
          scene: { type: "string", maxLength: 512 },
          activity: { type: "string", maxLength: 512 },
          location: { type: "string", maxLength: 512 },
          evidenceRefs: { type: "array", minItems: 1, maxItems: 32, items: { type: "string" } },
          ...lifeCommandProperties,
        },
        required: ["kind", "startAt", "endAt", "evidenceRefs", "expectedLifeContextRevision", "idempotencyKey"],
      },
      BrowserLifeEventCancelRequest: {
        type: "object",
        additionalProperties: false,
        properties: { expectedEventRevision: { type: "integer", minimum: 1 }, ...lifeCommandProperties },
        required: ["expectedEventRevision", "expectedLifeContextRevision", "idempotencyKey"],
      },
      BrowserPresenceRequest: {
        type: "object",
        additionalProperties: false,
        properties: {
          currentTask: { type: "string", minLength: 1, maxLength: 512 },
          userPresence: { type: "string", minLength: 1, maxLength: 128 },
          expiresAt: { type: "string", format: "date-time" },
          ...lifeCommandProperties,
        },
        required: ["expectedLifeContextRevision", "idempotencyKey"],
        anyOf: [{ required: ["currentTask"] }, { required: ["userPresence"] }],
      },
      BrowserScheduleItemRequest: {
        type: "object",
        additionalProperties: false,
        properties: {
          startAt: { type: "string", format: "date-time" }, endAt: { type: "string", format: "date-time" },
	          activity: { type: "string", minLength: 1, maxLength: 128 }, scene: { type: "string", minLength: 1, maxLength: 128 },
	          location: { type: "string", maxLength: 512 },
          placeId: {type:"string",maxLength:128},
          itemType: { type: "string", minLength: 1, maxLength: 128 }, status: { type: "string", minLength: 1, maxLength: 128 },
          priority: { type: "number", minimum: 0, maximum: 1 }, flexibility: { type: "number", minimum: 0, maximum: 1 },
          interruptionCost: { type: "number", minimum: 0, maximum: 1 },
        },
        required: ["startAt", "endAt", "activity", "scene"],
      },
      BrowserScheduleRequest: {
        type: "object",
        additionalProperties: false,
        properties: {
          localDate: { type: "string", format: "date" }, timezone: { type: "string", minLength: 1, maxLength: 128 },
          items: { type: "array", minItems: 1, maxItems: 128, items: { $ref: "#/components/schemas/BrowserScheduleItemRequest" } },
          evidenceRefs: { type: "array", minItems: 1, maxItems: 32, items: { type: "string" } },
          expectedRevision: { type: "integer", minimum: 0 }, completedBefore: { type: "string", format: "date-time" },
          ...lifeCommandProperties,
        },
        required: ["localDate", "timezone", "items", "evidenceRefs", "expectedRevision", "expectedLifeContextRevision", "idempotencyKey"],
      },
      BrowserScheduleCancelRequest: {
        type: "object",
        additionalProperties: false,
        properties: { expectedRevision: { type: "integer", minimum: 1 }, ...lifeCommandProperties },
        required: ["expectedRevision", "expectedLifeContextRevision", "idempotencyKey"],
      },
    },
  },
  paths: {
    "/health/live": { get: { operationId: "browserLive" } },
    "/health/ready": { get: { operationId: "browserReady" } },
    "/api/platform/ping": { get: { operationId: "platformPing" } },
    "/auth/setup": { post: { operationId: "authSetup" } },
    "/auth/setup-status": { get: { operationId: "authSetupStatus" } },
    "/auth/login": { post: { operationId: "authLogin" } },
    "/auth/logout": { post: { operationId: "authLogout" } },
    "/auth/session": { get: { operationId: "authSession" } },
    "/auth/revoke-all": { post: { operationId: "authRevokeAll" } },
    "/auth/password": { post: { operationId: "authChangePassword" } },
    "/api/settings": { get: { operationId: "readSettings" }, put: { operationId: "updateSettings" } },
    "/api/capability-requests": { get: { operationId: "listCapabilityRequests" } },
    "/api/capability-requests/{requestId}/review": { post: { operationId: "reviewCapabilityRequest" } },
    "/api/providers/endpoints": { get: { operationId: "listProviderEndpoints" }, put: { operationId: "configureProviderEndpoint" } },
    "/api/providers/endpoints/{endpointId}/models": { get: { operationId: "listProviderEndpointModels" } },
    "/api/providers": { get: { operationId: "providerBindings" } },
    "/api/providers/roles": { put: { operationId: "configureModelRole" } },
    "/api/conversations": { post: { operationId: "createConversation" } },
    "/api/fluctlights": { get: { operationId: "listFluctlights" }, post: { operationId: "createFluctlight", ...requestBody("BrowserFluctlightCreateRequest") } },
    "/api/actor-groups": { get: { operationId: "listActorGroups" }, post: { operationId: "createActorGroup" } },
    "/api/actor-groups/{groupId}/members": { post: { operationId: "assignActorGroupMember" } },
    "/api/actor-groups/{groupId}/members/{actorId}": { delete: { operationId: "removeActorGroupMember" } },
    "/api/fluctlights/{fluctlightId}": { get: { operationId: "getFluctlight" } },
    "/api/fluctlights/{fluctlightId}/goals":{get:{operationId:"listGoals",...jsonResponse("BrowserGoalPage")},post:{operationId:"createGoal",...requestBody("BrowserGoalCommand"),...jsonResponse("BrowserGoalCommandResult")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}":{get:{operationId:"goalDetail",...jsonResponse("BrowserGoal")},put:{operationId:"updateGoal",...requestBody("BrowserGoalCommand"),...jsonResponse("BrowserGoalCommandResult")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}/history":{get:{operationId:"goalHistory",...jsonResponse("BrowserGoalHistoryPage")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}/evidence":{get:{operationId:"goalEvidence",...jsonResponse("BrowserGoalHistoryPage")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}/pause":{post:{operationId:"pauseGoal",...requestBody("BrowserGoalCommand"),...jsonResponse("BrowserGoalCommandResult")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}/resume":{post:{operationId:"resumeGoal",...requestBody("BrowserGoalCommand"),...jsonResponse("BrowserGoalCommandResult")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}/cancel":{post:{operationId:"cancelGoal",...requestBody("BrowserGoalCommand"),...jsonResponse("BrowserGoalCommandResult")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}/abandon":{post:{operationId:"abandonGoal",...requestBody("BrowserGoalCommand"),...jsonResponse("BrowserGoalCommandResult")}},
    "/api/fluctlights/{fluctlightId}/goals/{goalId}/reassess":{post:{operationId:"reassessGoal",...requestBody("BrowserGoalCommand"),...jsonResponse("BrowserGoalCommandResult")}},
    "/api/fluctlights/{fluctlightId}/conversations": { get:{operationId:"conversationDirectory",...jsonResponse("BrowserConversationDirectoryPage")},post:{operationId:"newDirectConversation",...requestBody("BrowserNewDirectConversationRequest"),...jsonResponse("BrowserConversationPage")} },
    "/api/fluctlights/{fluctlightId}/conversations/{conversationId}/select": {post:{operationId:"selectDirectConversation",...jsonResponse("BrowserConversationPage")}},
    "/api/fluctlights/{fluctlightId}/messages/search": {get:{operationId:"searchConversationMessages",...jsonResponse("BrowserConversationSearchPage")}},
    "/api/conversations/{conversationId}/messages/{messageId}": {get:{operationId:"conversationMessage",...jsonResponse("BrowserConversationMessage")}},
    "/api/fluctlights/{fluctlightId}/conversation": { get: { operationId: "fluctlightDirectConversation" } },
    "/api/fluctlights/{fluctlightId}/moments": { get: { operationId: "fluctlightMoments" } },
    "/api/fluctlights/{fluctlightId}/moments/read": { post: { operationId: "markFluctlightMomentsRead" } },
    "/api/memories/{memoryId}": { put: { operationId: "reviseMemory" } },
    "/api/memories/{memoryId}/forget": { post: { operationId: "forgetMemory" } },
    "/api/fluctlights/{fluctlightId}/relationships/rollback": { post: { operationId: "rollbackRelationship" } },
    "/api/fluctlights/{fluctlightId}/relationships/{targetActorId}": { put: { operationId: "editRelationship" } },
    "/api/fluctlights/{fluctlightId}/autonomy-actions": { get: { operationId: "listAutonomyActions" } },
    "/api/autonomy-actions/{actionId}/govern": { post: { operationId: "governAutonomyAction" } },
    "/api/fluctlights/{fluctlightId}/events": { post: { operationId: "createLifeEvent", ...requestBody("BrowserLifeEventRequest") } },
    "/api/fluctlights/{fluctlightId}/events/{eventId}/cancel": { post: { operationId: "cancelLifeEvent", ...requestBody("BrowserLifeEventCancelRequest") } },
    "/api/fluctlights/{fluctlightId}/presence": { put: { operationId: "setLifePresence", ...requestBody("BrowserPresenceRequest") } },
    "/api/fluctlights/{fluctlightId}/schedules": { post: { operationId: "acceptLifeSchedule", ...requestBody("BrowserScheduleRequest") } },
    "/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/cancel": { post: { operationId: "cancelLifeSchedule", ...requestBody("BrowserScheduleCancelRequest") } },
    "/api/diagnostics/workflows": { get: { operationId: "listWorkflows" } },
    "/api/diagnostics/workflows/{workflowId}/status": { get: { operationId: "workflowStatus" } },
    "/api/diagnostics/workflows/{workflowId}/history": { get: { operationId: "workflowHistory" } },
    "/api/diagnostics/workflows/{workflowId}/pause": { post: { operationId: "pauseWorkflow" } },
    "/api/diagnostics/workflows/{workflowId}/resume": { post: { operationId: "resumeWorkflow" } },
    "/api/diagnostics/workflows/{workflowId}/cancel": { post: { operationId: "cancelWorkflow" } },
    "/api/diagnostics/workflows/{workflowId}/reset": { post: { operationId: "resetWorkflow" } },
    "/api/diagnostics/workflows/{workflowId}/restart": { post: { operationId: "restartWorkflow" } },
    "/api/fluctlights/{fluctlightId}/goal-set":{get:{operationId:"goalSet",...jsonResponse("BrowserGoalSet")},put:{operationId:"updateGoalSet",...requestBody("BrowserGoalSetCommand"),...jsonResponse("BrowserGoalSet")}},
    "/api/fluctlights/{fluctlightId}/goal-planning":{get:{operationId:"goalPlanningHistory",responses:{"200":{description:"Recent planning runs",content:{"application/json":{schema:jsonObjectArray}}}}},post:{operationId:"requestGoalPlanning",requestBody:{required:true,content:{"application/json":{schema:{type:"object",properties:{idempotencyKey:{type:"string"}},required:["idempotencyKey"]}}}},responses:{"200":{description:"Planning requested",content:{"application/json":{schema:jsonObject}}}}}},
    "/api/fluctlights/{fluctlightId}/actor-context/{actorId}":{get:{operationId:"actorContext",...jsonResponse("BrowserActorContext")},put:{operationId:"updateActorContext",...requestBody("BrowserActorContextUpdate"),...jsonResponse("BrowserActorContext")}},
    "/api/fluctlights/{fluctlightId}/actor-user-background": { put: { operationId:"updateActorUserBackground", ...requestBody("BrowserActorUserBackgroundUpdate"), ...jsonResponse("BrowserActorUserState") } },
    "/api/fluctlights/{fluctlightId}/detail": { get: { operationId: "fluctlightDetail", ...jsonResponse("BrowserFluctlightDetail") } },
    "/api/fluctlights/{fluctlightId}/wardrobe": { get: { operationId: "fluctlightWardrobe", ...jsonResponse("BrowserWardrobePage") } },
    "/api/fluctlights/{fluctlightId}/wardrobe/items": { post: { operationId: "addWardrobeItems" } },
    "/api/fluctlights/{fluctlightId}/wardrobe/items/{itemId}": { put: { operationId: "updateWardrobeItem" }, delete: { operationId: "deleteWardrobeItem" } },
    "/api/fluctlights/{fluctlightId}/wake-up": { post: { operationId: "triggerWakeUp" } },
    "/api/fluctlights/{fluctlightId}/developing-self": { get: { operationId: "developingSelf" } },
    "/api/fluctlights/{fluctlightId}/developing-self/{claimId}/rollback": { post: { operationId: "rollbackDevelopingSelf" } },
    "/api/fluctlights/{fluctlightId}/developing-self/{claimId}/forget": { post: { operationId: "forgetDevelopingSelf" } },
    "/api/fluctlights/{fluctlightId}/status": { put: { operationId: "setFluctlightStatus" } },
    "/api/fluctlights/{fluctlightId}/retire": { post: { operationId: "retireFluctlight" } },
    "/api/fluctlights/{fluctlightId}/foundation-revisions": { post: { operationId: "submitFoundationRevision" } },
    "/api/fluctlights/{fluctlightId}/foundation-revisions/{revisionId}/accept": { post: { operationId: "acceptFoundationRevision" } },
    "/api/fluctlights/{fluctlightId}/foundation-revisions/{revisionId}/reject": { post: { operationId: "rejectFoundationRevision" } },
    "/api/fluctlights/{fluctlightId}/foundation-revisions/rollback": { post: { operationId: "rollbackFoundationRevision" } },
    "/api/moments/{momentId}/comments": { post: { operationId: "commentOnMoment" } },
    "/api/moments/{momentId}/reactions": { post: { operationId: "reactToMoment" } },
    "/api/moments/{momentId}/hide": { post: { operationId: "hideMoment" } },
    "/api/moments/{momentId}/restore": { post: { operationId: "restoreMoment" } },
    "/api/fluctlight-creations/analysis": { post: { operationId: "analyzeFluctlightCreation", ...requestBody("BrowserFluctlightCreationAnalysisRequest"), ...jsonResponse("BrowserFluctlightCreationAnalysis") } },
    "/api/fluctlight-creations/activate": { post: { operationId: "activateFluctlightCreation", ...requestBody("BrowserFluctlightActivationRequest") } },
    "/api/conversations/{conversationId}/messages": { get: { operationId: "conversationMessages", ...jsonResponse("BrowserConversationPage") } },
    "/api/conversations/{conversationId}/read": { post: { operationId: "markConversationRead" } },
    "/api/conversations/{conversationId}/turn": { post: { operationId: "conversationTurn", ...requestBody("BrowserConversationTurnRequest") } },
    "/api/conversations/{conversationId}/turn/{turnId}/cancel": { post: { operationId: "cancelConversationTurn" } },
    "/api/diagnostics": { get: { operationId: "readDiagnostics" }, delete: { operationId: "clearDiagnostics" } },
    "/api/diagnostics/kev-decisions": {
      get: { operationId:"kevDecisions", parameters:["actor_self","actor_user","agent","decision_point","id","request_id","policy_outcome","call_status","application_status","correlation_id","run_id","from","to","limit","cursor"].map(name=>({name,in:"query",schema:{type:"string"}})), ...jsonResponse("BrowserKevDecisionPage") }
    },
    "/api/settings/vision/test": {
      post: {operationId:"testVisionUnderstanding",requestBody:{required:true,content:{"application/json":{schema:{type:"object",additionalProperties:false}}}},responses:{"200":{description:"Explicit image probe",content:{"application/json":{schema:{type:"object",additionalProperties:false,required:["state","config_version","image_input_verified","error_code"],properties:{state:{type:"string"},config_version:{type:"integer"},image_input_verified:{type:"boolean"},error_code:{type:"string"}}}}}}}}
    },
    "/api/settings/kev/test-connection": {
      post: { operationId:"testKevConnection", requestBody:{required:true,content:{"application/json":{schema:{type:"object",additionalProperties:false}}}}, ...jsonResponse("BrowserKevConnectionTest") }
    },
    "/api/diagnostics/lifecycle": { get: { operationId: "readLifecycleDiagnostics" } },
    "/api/diagnostics/model-runs": { get: { operationId: "readDiagnosticModelRuns", parameters:[{in:"query",name:"cursor",schema:{type:"string",maxLength:8192}},{in:"query",name:"limit",schema:{type:"integer",minimum:1,maximum:500}},{in:"query",name:"correlationId",schema:{type:"string",maxLength:128}}], ...jsonResponse("BrowserDiagnosticModelRuns") } },
    "/api/diagnostics/agent-runs": { get: { operationId: "readDiagnosticAgentRuns", parameters: [
      {in:"query",name:"cursor",required:false,schema:{type:"string",maxLength:8192}},
      { in: "query", name: "limit", required: false, schema: { type: "integer", minimum: 1, maximum: 500, default: 100 } },
      { in: "query", name: "correlationId", required: false, schema: { type: "string", maxLength: 128 } },
    ], ...jsonResponse("BrowserDiagnosticAgentRuns") } },
    "/api/diagnostics/media-prompts": { get: { operationId: "readDiagnosticMediaPrompts" } },
    "/api/diagnostics/media-prompts/{mediaIntentId}/retry": { post: { operationId: "retryDiagnosticMediaPrompt" } },
    "/api/diagnostics/export": { get: { operationId: "exportDiagnostics" } },
    "/api/media/{assetId}": { get: { operationId: "readMedia" } },
    "/api/fluctlights/{fluctlightId}/media": { get: { operationId: "fluctlightMedia" } },
    "/api/fluctlights/{fluctlightId}/sidebar": { get: { operationId: "fluctlightSidebar" } },
    "/api/fluctlights/{fluctlightId}/nearby": { get: { operationId: "fluctlightNearby" } },
    "/api/fluctlights/{fluctlightId}/diary": { get: { operationId: "listDiary" }, post: { operationId: "createDiaryEntry" } },
    "/api/fluctlights/{fluctlightId}/diary/{entryId}": { put: { operationId: "updateDiaryEntry" } },
    "/api/fluctlights/{fluctlightId}/world": { get: { operationId: "fluctlightWorld" }, put: { operationId: "updateRuntimeWorld" } },
    "/api/fluctlights/{fluctlightId}/world/places": { post: { operationId: "createWorldPlace" } },
    "/api/fluctlights/{fluctlightId}/npcs": { get: { operationId: "listRuntimeNPCs" } },
    "/api/fluctlights/{fluctlightId}/npcs/{npcId}": { get: { operationId: "readRuntimeNPC" }, put: { operationId: "updateRuntimeNPC" } },
    "/api/fluctlights/{fluctlightId}/npcs/{npcId}/encounters": { post: { operationId: "recordNPCEncounter" } },
    "/api/fluctlights/{fluctlightId}/world/templates": { get: { operationId: "listWorldTemplates" } },
    "/api/fluctlights/{fluctlightId}/world/templates/{templateId}": { put: { operationId: "updateWorldTemplate" } },
    "/api/fluctlights/{fluctlightId}/world/places/{placeId}": { get: { operationId: "worldPlace" }, put: { operationId: "updateWorldPlace" } },
    "/api/fluctlights/{fluctlightId}/autonomy-policy": { get: { operationId: "readAutonomyPolicy" }, put: { operationId: "updateAutonomyPolicy" } },
    "/api/fluctlights/{fluctlightId}/desires": { get: { operationId: "listDesires" } },
    "/api/fluctlights/{fluctlightId}/owner-requests": { get: { operationId: "listFluctlightOwnerRequests" } },
    "/api/fluctlights/{fluctlightId}/promises": { post: { operationId: "createPromise" } },
    "/api/fluctlights/{fluctlightId}/moment-suggestions": { post: { operationId: "suggestMoment" } },
    "/api/fluctlights/{fluctlightId}/persona-switch": { post: { operationId: "requestPersonaSwitch", ...requestBody("BrowserPersonaSwitchRequest"), responses: { ...jsonResponse("BrowserPersonaSwitchResult").responses, "202": {description:"Accepted for persistent rule assessment",content:{"application/json":{schema:{$ref:"#/components/schemas/BrowserPersonaSwitchResult"}}}} } } },
    "/api/fluctlights/{fluctlightId}/visual-identity": { put: { operationId: "updateVisualIdentity" } },
    "/api/fluctlights/{fluctlightId}/wardrobe/wear": { post: { operationId: "requestWardrobeWear" } },
    "/api/fluctlights/{fluctlightId}/wardrobe/items/{itemId}/use": { post: { operationId: "requestWardrobeUse" } },
    "/api/fluctlights/{fluctlightId}/wardrobe/items/{itemId}/photos": { post: { operationId: "requestWardrobePhoto" } },
    "/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items": { post: { operationId: "createScheduleItem" } },
    "/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items/{itemId}": { put: { operationId: "updateScheduleItem" }, delete: { operationId: "deleteScheduleItem" } },
    "/api/fluctlights/{fluctlightId}/reflections/{reflectionId}": { get: { operationId: "readReflection" } },
    "/api/fluctlights/{fluctlightId}/reflections/{reflectionId}/feedback": { post: { operationId: "submitReflectionFeedback" } },
    "/api/notifications": { get: { operationId: "listNotifications" } },
    "/api/notifications/unread-count": { get: { operationId: "notificationUnreadCount" } },
    "/api/notifications/{notificationId}/read": { post: { operationId: "markNotificationRead" } },
    "/api/pending": { get: { operationId: "listPending" } },
    "/api/pending/count": { get: { operationId: "pendingCount" } },
    "/api/studio/resources": { get: { operationId: "listStudioResources" }, post: { operationId: "createStudioResource" } },
    "/api/studio/resources/{resourceId}": { get: { operationId: "readStudioResource" }, put: { operationId: "updateStudioResource" } },
    "/api/studio/resources/{resourceId}/publish": { post: { operationId: "publishStudioResource" } },
    "/api/studio/resources/{resourceId}/apply-preview": { post: { operationId: "previewStudioApply" } },
    "/api/studio/resources/{resourceId}/apply": { post: { operationId: "applyStudioResource" } },
    "/api/studio/previews": { get: { operationId: "listStudioPreviews" }, post: { operationId: "createStudioPreview" } },
    "/api/studio/packs": { post: { operationId: "importStudioPack" } },
    "/api/studio/packs/validate": { post: { operationId: "validateStudioPack" } },
    "/api/studio/packs/export": { get: { operationId: "exportStudioPack" } },
    "/api/ops/backups": { get: { operationId: "listBackups" }, post: { operationId: "createBackup" } },
    "/api/ops/backups/{backupId}/restore": { post: { operationId: "restoreBackup" } },
    "/api/diagnostics/harm-intercepts": { get: { operationId: "harmIntercepts" } },
    "/api/diagnostics/tasks/{taskId}": { get: { operationId: "diagnosticTask" } },
    "/api/settings/agents": { get: { operationId: "listAgents" } },
    "/api/memories/{memoryId}/suggestions": { post: { operationId: "suggestMemoryRevision" } },
    "/api/owner-requests/{requestId}": { get: { operationId: "readOwnerRequest" } },
    "/api/moments": { get: { operationId: "globalMoments" }, post: { operationId: "publishOwnerMoment" } },
    "/api/media": { post: { operationId: "uploadMedia" } },
    "/api/actor-groups/{groupId}": { delete: { operationId: "deleteActorGroup" } },
  },
};

const scheduleItemCommand={...lifeCommandProperties,expectedRevision:{type:"integer",minimum:1},reason:{type:"string",minLength:1,maxLength:500}};
schema.components.schemas.ScheduleItemDeleteBody={type:"object",additionalProperties:false,properties:scheduleItemCommand,required:Object.keys(scheduleItemCommand)};
schema.components.schemas.ScheduleItemSuggestionBody={type:"object",additionalProperties:false,properties:{...scheduleItemCommand,activity:{type:"string",minLength:1,maxLength:512},scene:{type:"string",maxLength:512},location:{type:"string",maxLength:512},placeId:{type:"string",maxLength:128},startAt:{type:"string",format:"date-time"},endAt:{type:"string",format:"date-time"},evidenceRefs:{type:"array",minItems:1,items:{type:"string"}}},required:[...Object.keys(scheduleItemCommand),"activity","startAt","endAt","evidenceRefs"]};
schema.paths['/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items'].post={...schema.paths['/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items'].post,...requestBody('ScheduleItemSuggestionBody')};
for(const method of ['put','delete'])schema.paths['/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items/{itemId}'][method]={...schema.paths['/api/fluctlights/{fluctlightId}/schedules/{scheduleId}/items/{itemId}'][method],...requestBody(method==='delete'?'ScheduleItemDeleteBody':'ScheduleItemSuggestionBody')};
Object.assign(schema.components.schemas, worldSchemas);
Object.assign(schema.paths, worldPaths);
Object.assign(schema.components.schemas, memorySchemas);
Object.assign(schema.paths, memoryPaths);
schema.paths['/api/memories/{memoryId}'].get=managedMemoryRead;
Object.assign(schema.components.schemas,reflectionSchemas);
Object.assign(schema.paths,reflectionPaths);
Object.assign(schema.components.schemas,operationsSchemas);
Object.assign(schema.paths,operationsPaths);
Object.assign(schema.components.schemas,diarySchemas);
for(const [path,methods]of Object.entries(diaryPaths))schema.paths[path]={...schema.paths[path],...methods};
Object.assign(schema.components.schemas,governanceSchemas);Object.assign(schema.paths,governancePaths);
Object.assign(schema.components.schemas,visualBriefSchemas);
for(const [path,methods]of Object.entries(visualBriefPaths))schema.paths[path]={...schema.paths[path],...methods};
Object.assign(schema.components.schemas,inventorySchemas);Object.assign(schema.paths,inventoryPaths);
Object.assign(schema.components.schemas,npcRuntimeSchemas);
for(const [path,methods]of Object.entries(npcRuntimePaths))schema.paths[path]={...schema.paths[path],...methods};
Object.assign(schema.components.schemas,studioJobSchemas);Object.assign(schema.paths,studioJobPaths);
Object.assign(schema.components.schemas,diagnosticRecordSchemas);Object.assign(schema.paths,diagnosticRecordPaths);
Object.assign(schema.components.schemas,configurationSchemas);Object.assign(schema.paths,configurationPaths);
Object.assign(schema.components.schemas,momentSchemas);Object.assign(schema.paths,momentPaths);
Object.assign(schema.components.schemas,mediaLibrarySchemas);Object.assign(schema.paths,mediaLibraryPaths);schema.paths["/api/media"]=mediaUploadPath;
Object.assign(schema.components.schemas,worldItemSchemas);Object.assign(schema.paths,worldItemPaths);
Object.assign(schema.components.schemas,worldEventSchemas);
for(const [path,methods]of Object.entries(worldEventPaths))schema.paths[path]={...schema.paths[path],...methods};
Object.assign(schema.components.schemas,promiseSchemas);
for(const [path,methods]of Object.entries(promisePaths))schema.paths[path]={...schema.paths[path],...methods};
schema.components.schemas.BrowserOwnerRequest.properties.status.enum.push("awaiting_goal","cancelled");
Object.assign(schema.components.schemas,selfSchemas);
Object.assign(schema.paths,selfPaths);
Object.assign(schema.components.schemas.BrowserOwnerRequest.properties,{tasks_state:{type:'string',enum:['available','failed']},tasks:{type:'array',items:{$ref:'#/components/schemas/BrowserRequestTask'}},tasks_error_code:{type:'string'}});
schema.paths['/api/owner-requests/{requestId}'].get={...schema.paths['/api/owner-requests/{requestId}'].get,...jsonResponse('BrowserOwnerRequest')};

Object.assign(schema.components.schemas,studioCatalogSchemas);
for(const [path,methods]of Object.entries(studioCatalogPaths))schema.paths[path]={...schema.paths[path],...methods};

Object.assign(schema.components.schemas,dataToolsSchemas);for(const [path,methods]of Object.entries(dataToolsPaths))schema.paths[path]={...schema.paths[path],...methods};
Object.assign(schema.components.schemas,placeDefaultSchemas);for(const [path,methods]of Object.entries(placeDefaultPaths))schema.paths[path]={...schema.paths[path],...methods};
Object.assign(schema.components.schemas,scheduleOwnerSchemas);for(const [path,methods]of Object.entries(scheduleOwnerPaths))schema.paths[path]={...schema.paths[path],...methods};
await writeFile(
  new URL("../openapi.json", import.meta.url),
  `${JSON.stringify(schema, null, 2)}\n`,
);
