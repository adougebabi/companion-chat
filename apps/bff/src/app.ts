import { CoreApiError, CoreClient } from "@fluctlight/core-client";
import { Type } from "@sinclair/typebox";
import cookie from "@fastify/cookie";
import { randomBytes, timingSafeEqual } from "node:crypto";
import Fastify, { type FastifyInstance, type FastifyReply } from "fastify";
import { translateCoreNdjson } from "./ndjson.js";

export type BffOptions = {
  coreBaseUrl: string;
  coreServiceKey: string;
  trustedOrigin?: string;
  secureCookies?: boolean;
  fetcher?: typeof fetch;
};

const healthResponse = Type.Object({
  status: Type.String(),
  role: Type.Literal("bff"),
});

const sessionResponse = Type.Object({
  authenticated: Type.Boolean(),
  actorId: Type.Optional(Type.String()),
});
const setupStatusResponse = Type.Object({ setupAvailable: Type.Boolean() });
const serviceUnavailableResponse = Type.Object({ code: Type.String(), message: Type.String() });
const passwordRequest = Type.Object({ password: Type.String({ minLength: 6 }) });
const setupRequest = Type.Intersect([passwordRequest, Type.Object({ setupToken: Type.String({ minLength: 16 }) })]);
const settingsPatchRequest = Type.Object({
  values: Type.Optional(Type.Record(Type.String(), Type.Unknown())),
  secrets: Type.Optional(Type.Record(Type.String(), Type.Union([Type.String(), Type.Null()]))),
  clearSecrets: Type.Optional(Type.Array(Type.String())),
});
const providerEndpointRequest = Type.Object({
  endpointId: Type.String({ minLength: 1, maxLength: 128 }),
  kind: Type.String({ minLength: 1, maxLength: 64 }),
  baseUrl: Type.String({ minLength: 8 }),
  secretPurpose: Type.String({ minLength: 1, maxLength: 128 }),
});
const modelRoleRequest = Type.Object({
  role: Type.String({ minLength: 1, maxLength: 64 }),
  endpointId: Type.String({ minLength: 1, maxLength: 128 }),
  modelId: Type.String({ minLength: 1, maxLength: 256 }),
  tokenBudget: Type.Integer({ minimum: 1 }),
  timeoutSeconds: Type.Integer({ minimum: 1 }),
});
const providerEndpointModelsParams = Type.Object({
  endpointId: Type.String({ minLength: 1, maxLength: 128 }),
});
const conversationCreateRequest = Type.Object({
  title: Type.Optional(Type.String({ maxLength: 256 })),
  participantActorIds: Type.Array(Type.String({ minLength: 1, maxLength: 128 }), { minItems: 1, maxItems: 1 }),
});
const fluctlightCreateRequest = Type.Object({
  id: Type.Optional(Type.String({ minLength: 1, maxLength: 128 })),
  name: Type.Optional(Type.String({ maxLength: 256 })),
});
const fluctlightCreationAnalysisRequest = Type.Object({ description: Type.String({ minLength: 1, maxLength: 12_000 }) });
const fluctlightCreationActivationRequest = Type.Object({
  requestId: Type.String({ minLength: 1, maxLength: 256 }),
  initializationMode: Type.Union([Type.Literal("blank_slate"), Type.Literal("llm_defined")]),
  identity: Type.Record(Type.String(), Type.Unknown()),
  personality: Type.Optional(Type.Record(Type.String(), Type.Unknown())),
  behavioralPolicy: Type.Optional(Type.Record(Type.String(), Type.Unknown())),
  lifeProfile: Type.Optional(Type.Record(Type.String(), Type.Unknown())),
  foundationProvenance: Type.Optional(Type.Record(Type.String(), Type.Unknown())),
});
const fluctlightStatusRequest = Type.Object({
  status: Type.Union([Type.Literal("active"), Type.Literal("paused")]),
  expectedRevision: Type.Integer({ minimum: 0 }),
  reason: Type.String({ minLength: 1, maxLength: 1024 }),
});
const fluctlightRetireRequest = Type.Object({
  expectedRevision: Type.Integer({ minimum: 0 }),
  reason: Type.String({ minLength: 1, maxLength: 1024 }),
});
const foundationRevisionRequest = Type.Object({
  changes: Type.Record(Type.String(), Type.Unknown(), { minProperties: 1 }),
  expectedRevision: Type.Integer({ minimum: 0 }),
  reason: Type.String({ minLength: 1, maxLength: 1024 }),
});
const foundationRevisionAcceptRequest = Type.Object({
  expectedRevision: Type.Integer({ minimum: 0 }),
  reason: Type.String({ minLength: 1, maxLength: 1024 }),
});
const foundationRevisionRollbackRequest = Type.Intersect([
  foundationRevisionAcceptRequest,
  Type.Object({ targetRevision: Type.Integer({ minimum: 0 }) }),
]);
const momentCommentRequest = Type.Object({ text: Type.String({ minLength: 1, maxLength: 32_000 }) });
const momentReactionRequest = Type.Object({ kind: Type.Optional(Type.Union([Type.Literal("like"), Type.Literal("care"), Type.Literal("celebrate")])) });
const memoryRevisionRequest = Type.Object({ expectedRevision: Type.Integer({ minimum: 0 }), content: Type.String({ minLength: 1, maxLength: 4096 }), evidenceRefs: Type.Array(Type.String({ minLength: 1, maxLength: 512 }), { minItems: 1, maxItems: 32 }) });
const memoryForgetRequest = Type.Object({ expectedRevision: Type.Integer({ minimum: 0 }), evidenceRefs: Type.Array(Type.String({ minLength: 1, maxLength: 512 }), { minItems: 1, maxItems: 32 }) });
const relationshipRollbackRequest = Type.Object({ targetActorId: Type.String({ minLength: 1, maxLength: 128 }), targetRevision: Type.Integer({ minimum: 0 }), expectedRevision: Type.Integer({ minimum: 0 }), evidenceRefs: Type.Array(Type.String({ minLength: 1, maxLength: 512 }), { minItems: 1, maxItems: 32 }) });
const autonomyGovernanceRequest = Type.Object({ status: Type.Union([Type.Literal("paused"), Type.Literal("deferred"), Type.Literal("cancelled")]), reason: Type.String({ minLength: 1, maxLength: 1024 }) });
const lifeEventRequest = Type.Object({ kind: Type.String({ minLength: 1, maxLength: 128 }), startAt: Type.String({ minLength: 1 }), endAt: Type.String({ minLength: 1 }), scene: Type.Optional(Type.String({ maxLength: 512 })), activity: Type.Optional(Type.String({ maxLength: 512 })), location: Type.Optional(Type.String({ maxLength: 512 })), evidenceRefs: Type.Array(Type.String({ minLength: 1, maxLength: 512 }), { minItems: 1, maxItems: 32 }) });
const presenceRequest = Type.Object({ currentTask: Type.Optional(Type.String({ maxLength: 512 })), userPresence: Type.Optional(Type.String({ maxLength: 128 })) });
const scheduleRequest = Type.Object({ localDate: Type.String({ minLength: 10, maxLength: 10 }), timezone: Type.String({ minLength: 1, maxLength: 128 }), items: Type.Array(Type.Object({ startAt: Type.String({ minLength: 1 }), endAt: Type.String({ minLength: 1 }), activity: Type.String({ minLength: 1, maxLength: 128 }), scene: Type.String({ minLength: 1, maxLength: 128 }), itemType: Type.Optional(Type.String({ minLength: 1, maxLength: 128 })), status: Type.Optional(Type.String({ minLength: 1, maxLength: 128 })), priority: Type.Optional(Type.Number({ minimum: 0, maximum: 1 })), flexibility: Type.Optional(Type.Number({ minimum: 0, maximum: 1 })), interruptionCost: Type.Optional(Type.Number({ minimum: 0, maximum: 1 })) }), { minItems: 1, maxItems: 128 }), evidenceRefs: Type.Array(Type.String({ minLength: 1, maxLength: 512 }), { minItems: 1, maxItems: 32 }), expectedRevision: Type.Optional(Type.Integer({ minimum: 0 })), completedBefore: Type.Optional(Type.String({ minLength: 1 })) });
const workflowResetRequest = Type.Object({ historyPoint: Type.Integer({ minimum: 1 }) });
const actorGroupRequest = Type.Object({ name: Type.String({ minLength: 1, maxLength: 128 }) });
const actorGroupMemberRequest = Type.Object({ actorId: Type.String({ minLength: 1, maxLength: 128 }) });
const conversationTurnRequest = Type.Object({
  text: Type.String({ minLength: 1, maxLength: 32_000 }),
  fluctlightId: Type.String({ minLength: 1, maxLength: 128 }),
  attachmentRefs: Type.Optional(Type.Array(Type.String({ minLength: 1, maxLength: 512 }), { maxItems: 16 })),
  idempotencyKey: Type.String({ minLength: 1, maxLength: 256 }),
  turnId: Type.Optional(Type.String({ minLength: 1, maxLength: 256 })),
});
const readPositionRequest = Type.Object({
  readSequence: Type.Integer({ minimum: 0 }),
  deliveredSequence: Type.Optional(Type.Integer({ minimum: 0 })),
});

const sessionCookie = "fluctlight_session";
const csrfCookie = "fluctlight_csrf";

function rejectUntrustedMutation(
  origin: string | undefined,
  trustedOrigin: string | undefined,
  csrfCookieValue: string | undefined,
  csrfHeaderValue: string | string[] | undefined,
): boolean {
  const headerValue = Array.isArray(csrfHeaderValue) ? csrfHeaderValue[0] : csrfHeaderValue;
  if (!trustedOrigin || origin !== trustedOrigin || !csrfCookieValue || !headerValue) return true;
  const cookieBytes = Buffer.from(csrfCookieValue);
  const headerBytes = Buffer.from(headerValue);
  return cookieBytes.length !== headerBytes.length || !timingSafeEqual(cookieBytes, headerBytes);
}

function newCsrfToken(): string {
  return randomBytes(32).toString("base64url");
}

function issueCsrfCookie(reply: FastifyReply, secure: boolean): void {
  reply.setCookie(csrfCookie, newCsrfToken(), {
    httpOnly: false,
    secure,
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24,
  });
}

function browserPage(page: Record<string, any>) {
  const conversation = page.conversation;
  return {
    conversation: {
      id: conversation.id,
      createdByActorId: conversation.created_by_actor_id,
      title: conversation.title,
      revision: conversation.revision,
      createdAt: conversation.created_at,
      updatedAt: conversation.updated_at,
    },
    participants: (page.participants ?? []).map((participant: Record<string, any>) => ({
      conversationId: participant.conversation_id,
      actorId: participant.actor_id,
      role: participant.role,
      status: participant.status,
      joinedAt: participant.joined_at,
      leftAt: participant.left_at,
    })),
    messages: (page.messages ?? []).map((message: Record<string, any>) => ({
      id: message.id,
      conversationId: message.conversation_id,
      sequence: message.sequence,
      authorActorId: message.author_actor_id,
      kind: message.kind,
      text: message.text,
      attachmentRefs: message.attachment_refs ?? [],
      createdAt: message.created_at,
    })),
    nextBeforeSequence: page.next_before_sequence ?? null,
  };
}

export function createBff(options: BffOptions): FastifyInstance {
  const app = Fastify({ logger: true });
  const core = new CoreClient(options.coreBaseUrl, options.coreServiceKey, options.fetcher);
  const secureCookies = options.secureCookies ?? options.trustedOrigin?.startsWith("https://") ?? true;
  void app.register(cookie);
  app.addHook("onSend", async (request, reply) => {
    const origin = request.headers.origin;
    if (origin && origin === options.trustedOrigin) {
      reply.header("access-control-allow-origin", origin);
      reply.header("access-control-allow-credentials", "true");
      reply.header("vary", "Origin");
    }
    if (!request.cookies?.[csrfCookie]) {
      issueCsrfCookie(reply, secureCookies);
    }
  });
  app.options("/*", async (request, reply) => {
    const origin = request.headers.origin;
    if (!origin || origin !== options.trustedOrigin) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    return reply
      .header("access-control-allow-origin", origin)
      .header("access-control-allow-credentials", "true")
      .header("access-control-allow-methods", "GET,POST,PUT,DELETE,OPTIONS")
      .header("access-control-allow-headers", "content-type,range,x-csrf-token")
      .code(204)
      .send();
  });
  app.get("/health/live", { schema: { response: { 200: healthResponse } } }, async () => ({
    status: "ok",
    role: "bff" as const,
  }));

  app.get(
    "/health/ready",
    { schema: { response: { 200: healthResponse, 503: healthResponse } } },
    async (_, reply) => {
    try {
      await core.health("/health/ready");
      return { status: "ready", role: "bff" as const };
    } catch {
      return reply.code(503).send({ status: "unavailable", role: "bff" });
    }
    },
  );

  app.get("/api/platform/ping", async (_, reply) => {
    try {
      return await core.ping();
    } catch {
      return reply.code(502).send({ code: "core_unavailable", message: "Core platform is unavailable" });
    }
  });

  app.get("/auth/session", { schema: { response: { 200: sessionResponse, 401: sessionResponse } } }, async (request, reply) => {
    try {
      const session = await core.session(request.cookies[sessionCookie]);
      return session.authenticated ? session : reply.code(401).send({ authenticated: false });
    } catch {
      return reply.code(401).send({ authenticated: false });
    }
  });

  app.get("/auth/setup-status", { schema: { response: { 200: setupStatusResponse, 502: serviceUnavailableResponse } } }, async (_request, reply) => {
    try {
      const status = await core.setupStatus();
      return { setupAvailable: status.setupAvailable };
    } catch {
      return reply.code(502).send({ code: "core_unavailable", message: "Core authentication is unavailable" });
    }
  });

  app.post("/auth/logout", async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (session) {
      try {
        await core.revokeCurrent(session);
      } catch {
        return reply.code(403).send({ code: "logout_failed", message: "Session could not be revoked" });
      }
    }
    reply.clearCookie(sessionCookie, { path: "/", httpOnly: true, sameSite: "lax", secure: secureCookies });
    issueCsrfCookie(reply, secureCookies);
    return reply.code(204).send();
  });

  app.post("/auth/revoke-all", async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try {
      await core.revokeAll(session);
      reply.clearCookie(sessionCookie, { path: "/", httpOnly: true, sameSite: "lax", secure: secureCookies });
      issueCsrfCookie(reply, secureCookies);
      return reply.code(204).send();
    } catch {
      return reply.code(403).send({ code: "revoke_failed", message: "Session revocation failed" });
    }
  });

  app.post("/auth/password", { schema: { body: passwordRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try {
      const body = request.body as { password: string };
      await core.resetPassword(session, body.password);
      reply.clearCookie(sessionCookie, { path: "/", httpOnly: true, sameSite: "lax", secure: secureCookies });
      issueCsrfCookie(reply, secureCookies);
      return reply.code(204).send();
    } catch {
      return reply.code(403).send({ code: "password_change_failed", message: "Password could not be changed" });
    }
  });

  app.post("/auth/setup", { schema: { body: setupRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    try {
      const body = request.body as { setupToken: string; password: string };
      const session = await core.setup(body.setupToken, body.password);
      reply.setCookie(sessionCookie, session.sessionToken, { httpOnly: true, secure: secureCookies, sameSite: "lax", path: "/" });
      issueCsrfCookie(reply, secureCookies);
      return { authenticated: true, actorId: session.actorId };
    } catch {
      return reply.code(401).send({ authenticated: false });
    }
  });

  app.post("/auth/login", { schema: { body: passwordRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    try {
      const body = request.body as { password: string };
      const session = await core.login(body.password);
      reply.setCookie(sessionCookie, session.sessionToken, { httpOnly: true, secure: secureCookies, sameSite: "lax", path: "/" });
      issueCsrfCookie(reply, secureCookies);
      return { authenticated: true, actorId: session.actorId };
    } catch {
      return reply.code(401).send({ authenticated: false });
    }
  });

  app.get("/api/settings", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try {
      return await core.readSettings(session);
    } catch {
      return reply.code(403).send({ code: "settings_unavailable", message: "Settings are unavailable" });
    }
  });

  app.put("/api/settings", { schema: { body: settingsPatchRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { values?: Record<string, unknown>; secrets?: Record<string, string | null>; clearSecrets?: string[] };
    try {
      return await core.updateSettings(session, {
        values: body.values ?? {},
        secrets: body.secrets ?? {},
        clear_secrets: body.clearSecrets ?? [],
      });
    } catch {
      return reply.code(403).send({ code: "settings_unavailable", message: "Settings are unavailable" });
    }
  });

  app.put("/api/providers/endpoints", { schema: { body: providerEndpointRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { endpointId: string; kind: string; baseUrl: string; secretPurpose: string };
    try {
      await core.configureProviderEndpoint(session, {
        endpoint_id: body.endpointId,
        kind: body.kind,
        base_url: body.baseUrl,
        secret_purpose: body.secretPurpose,
      });
      return reply.code(204).send();
    } catch {
      return reply.code(403).send({ code: "provider_configuration_failed", message: "Provider configuration failed" });
    }
  });

  app.get("/api/providers/endpoints", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.providerEndpoints(session); }
    catch { return reply.code(403).send({ code: "provider_configuration_failed", message: "Provider endpoints are unavailable" }); }
  });

  app.get("/api/providers/endpoints/:endpointId/models", { schema: { params: providerEndpointModelsParams } }, async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const { endpointId } = request.params as { endpointId: string };
    try {
      const result = await core.providerEndpointModels(session, endpointId);
      return { endpointId: result.endpoint_id, models: result.models };
    } catch {
      return reply.code(422).send({ code: "provider_models_unavailable", message: "Provider models are unavailable" });
    }
  });

  app.get("/api/providers", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.providerBindings(session); }
    catch { return reply.code(403).send({ code: "provider_configuration_failed", message: "Provider configuration is unavailable" }); }
  });

  app.put("/api/providers/roles", { schema: { body: modelRoleRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { role: string; endpointId: string; modelId: string; tokenBudget: number; timeoutSeconds: number };
    try {
      const result = await core.configureModelRole(session, {
        role: body.role,
        endpoint_id: body.endpointId,
        model_id: body.modelId,
        token_budget: body.tokenBudget,
        timeout_seconds: body.timeoutSeconds,
      });
      return {
        role: result.role,
        available: result.available,
        capabilityVersion: result.capability_version,
      };
    } catch {
      return reply.code(422).send({ code: "provider_preflight_failed", message: "Provider preflight failed" });
    }
  });

  app.post("/api/conversations", { schema: { body: conversationCreateRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { title?: string; participantActorIds: string[] };
    try {
      return browserPage(await core.createConversation(session, {
        title: body.title,
        participant_actor_ids: body.participantActorIds,
      }));
    } catch {
      return reply.code(502).send({ code: "conversation_unavailable", message: "Conversation is unavailable" });
    }
  });

  app.post("/api/fluctlights", { schema: { body: fluctlightCreateRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { id?: string; name?: string };
    try {
      return await core.createFluctlight(session, body);
    } catch {
      return reply.code(409).send({ code: "fluctlight_create_failed", message: "Fluctlight could not be created" });
    }
  });

  app.post("/api/fluctlight-creations/analysis", { schema: { body: fluctlightCreationAnalysisRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try {
      return await core.analyzeFluctlightCreation(
        session,
        (request.body as { description: string }).description,
      );
    } catch (error) {
      if (error instanceof CoreApiError) {
        if (error.status === 401) {
          return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
        }
        if (error.status === 422) {
          return reply.code(422).send({
            code: error.code,
            message: Object.keys(error.details).length ? error.message : "Fluctlight analysis was rejected",
            ...(Object.keys(error.details).length ? { details: error.details } : {}),
          });
        }
        if (error.status >= 500) {
          return reply.code(503).send({ code: error.code, message: "Fluctlight analysis service is unavailable" });
        }
      }
      return reply.code(502).send({ code: "fluctlight_analysis_unavailable", message: "Fluctlight analysis service is unavailable" });
    }
  });

  app.post("/api/fluctlight-creations/activate", { schema: { body: fluctlightCreationActivationRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { requestId: string; initializationMode: string; identity: Record<string, unknown>; personality?: Record<string, unknown>; behavioralPolicy?: Record<string, unknown>; lifeProfile?: Record<string, unknown>; foundationProvenance?: Record<string, unknown>; initialGoals?: Array<Record<string, unknown>>; initialIntentions?: Array<Record<string, unknown>> };
    try {
      return await core.activateFluctlightCreation(session, { request_id: body.requestId, initialization_mode: body.initializationMode, identity: body.identity, personality: body.personality, behavioral_policy: body.behavioralPolicy, life_profile: body.lifeProfile, foundation_provenance: body.foundationProvenance, initial_goals: body.initialGoals, initial_intentions: body.initialIntentions });
    } catch (error) {
      if (error instanceof CoreApiError) {
        if (error.status === 401) {
          return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
        }
        if (error.status === 422) {
          return reply.code(422).send({
            code: error.code,
            message: Object.keys(error.details).length ? error.message : "Fluctlight activation was rejected",
            ...(Object.keys(error.details).length ? { details: error.details } : {}),
          });
        }
        if (error.status >= 500) {
          return reply.code(503).send({ code: error.code, message: "Fluctlight activation service is unavailable" });
        }
      }
      return reply.code(502).send({ code: "fluctlight_activation_unavailable", message: "Fluctlight activation service is unavailable" });
    }
  });

  app.get("/api/fluctlights", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try {
      return await core.listFluctlights(session);
    } catch {
      return reply.code(403).send({ code: "fluctlights_unavailable", message: "Fluctlights are unavailable" });
    }
  });

  app.get("/api/actor-groups", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.listActorGroups(session); }
    catch { return reply.code(403).send({ code: "actor_groups_unavailable", message: "Actor groups are unavailable" }); }
  });

  app.post("/api/actor-groups", { schema: { body: actorGroupRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.createActorGroup(session, (request.body as { name: string }).name); }
    catch { return reply.code(422).send({ code: "actor_group_create_failed", message: "Actor group could not be created" }); }
  });

  app.post("/api/actor-groups/:groupId/members", { schema: { body: actorGroupMemberRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { await core.assignActorGroupMember(session, (request.params as { groupId: string }).groupId, (request.body as { actorId: string }).actorId); return reply.code(204).send(); }
    catch { return reply.code(422).send({ code: "actor_group_assign_failed", message: "Actor could not be assigned" }); }
  });

  app.delete("/api/actor-groups/:groupId/members/:actorId", async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { await core.removeActorGroupMember(session, (request.params as { groupId: string; actorId: string }).groupId, (request.params as { actorId: string }).actorId); return reply.code(204).send(); }
    catch { return reply.code(422).send({ code: "actor_group_remove_failed", message: "Actor could not be removed" }); }
  });

  app.get("/api/fluctlights/:fluctlightId", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { fluctlightId: string };
    try {
      return await core.getFluctlight(session, params.fluctlightId);
    } catch {
      return reply.code(404).send({ code: "fluctlight_not_found", message: "Fluctlight is unavailable" });
    }
  });

  app.get("/api/fluctlights/:fluctlightId/conversation", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { fluctlightId: string };
    try {
      return browserPage(await core.fluctlightDirectConversation(session, params.fluctlightId));
    } catch {
      return reply.code(404).send({ code: "fluctlight_conversation_unavailable", message: "Fluctlight conversation is unavailable" });
    }
  });

  app.get("/api/fluctlights/:fluctlightId/moments", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { fluctlightId: string };
    const query = request.query as { includeHidden?: string };
    try { return await core.fluctlightMoments(session, params.fluctlightId, query.includeHidden === "true"); }
    catch { return reply.code(404).send({ code: "fluctlight_moments_unavailable", message: "Fluctlight Moments are unavailable" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/moments/read", async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { await core.markFluctlightMomentsRead(session, (request.params as { fluctlightId: string }).fluctlightId); return reply.code(204).send(); }
    catch { return reply.code(422).send({ code: "moment_read_failed", message: "Moments could not be marked read" }); }
  });

  app.get("/api/moments", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const query = request.query as { includeHidden?: string };
    try { return await core.globalMoments(session, query.includeHidden === "true"); }
    catch { return reply.code(403).send({ code: "moments_unavailable", message: "Moments are unavailable" }); }
  });

  app.get("/api/fluctlights/:fluctlightId/detail", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.fluctlightDetail(session, (request.params as { fluctlightId: string }).fluctlightId); }
    catch { return reply.code(404).send({ code: "fluctlight_not_found", message: "Fluctlight detail is unavailable" }); }
  });

  app.put("/api/fluctlights/:fluctlightId/status", { schema: { body: fluctlightStatusRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { status: "active" | "paused"; expectedRevision: number; reason: string };
    try {
      return await core.setFluctlightStatus(session, (request.params as { fluctlightId: string }).fluctlightId, {
        status: body.status,
        expected_revision: body.expectedRevision,
        reason: body.reason,
      });
    } catch { return reply.code(422).send({ code: "fluctlight_status_failed", message: "Fluctlight status could not be changed" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/retire", { schema: { body: fluctlightRetireRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { expectedRevision: number; reason: string };
    try {
      return await core.retireFluctlight(session, (request.params as { fluctlightId: string }).fluctlightId, {
        expected_revision: body.expectedRevision,
        reason: body.reason,
      });
    } catch { return reply.code(422).send({ code: "fluctlight_retire_failed", message: "Fluctlight could not be retired" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/foundation-revisions", { schema: { body: foundationRevisionRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { changes: Record<string, unknown>; expectedRevision: number; reason: string };
    try { return await core.submitFoundationRevision(session, (request.params as { fluctlightId: string }).fluctlightId, { changes: body.changes, expected_revision: body.expectedRevision, reason: body.reason }); }
    catch { return reply.code(422).send({ code: "foundation_revision_failed", message: "Foundation revision could not be proposed" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/foundation-revisions/:revisionId/accept", { schema: { body: foundationRevisionAcceptRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { fluctlightId: string; revisionId: string };
    const body = request.body as { expectedRevision: number; reason: string };
    try { return await core.acceptFoundationRevision(session, params.fluctlightId, params.revisionId, { expected_revision: body.expectedRevision, reason: body.reason }); }
    catch { return reply.code(422).send({ code: "foundation_revision_accept_failed", message: "Foundation revision could not be accepted" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/foundation-revisions/:revisionId/reject", { schema: { body: foundationRevisionAcceptRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { fluctlightId: string; revisionId: string };
    const body = request.body as { expectedRevision: number; reason: string };
    try { return await core.rejectFoundationRevision(session, params.fluctlightId, params.revisionId, { expected_revision: body.expectedRevision, reason: body.reason }); }
    catch { return reply.code(422).send({ code: "foundation_revision_reject_failed", message: "Foundation revision could not be rejected" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/foundation-revisions/rollback", { schema: { body: foundationRevisionRollbackRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { targetRevision: number; expectedRevision: number; reason: string };
    try { return await core.rollbackFoundationRevision(session, (request.params as { fluctlightId: string }).fluctlightId, { target_revision: body.targetRevision, expected_revision: body.expectedRevision, reason: body.reason }); }
    catch { return reply.code(422).send({ code: "foundation_revision_rollback_failed", message: "Foundation revision could not be rolled back" }); }
  });

  app.post("/api/moments/:momentId/comments", { schema: { body: momentCommentRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.commentOnMoment(session, (request.params as { momentId: string }).momentId, (request.body as { text: string }).text); }
    catch { return reply.code(422).send({ code: "moment_comment_failed", message: "Moment comment could not be saved" }); }
  });

  app.put("/api/memories/:memoryId", { schema: { body: memoryRevisionRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { expectedRevision: number; content: string; evidenceRefs: string[] };
    try { return await core.reviseMemory(session, (request.params as { memoryId: string }).memoryId, { expected_revision: body.expectedRevision, content: body.content, evidence_refs: body.evidenceRefs }); }
    catch { return reply.code(422).send({ code: "memory_revision_failed", message: "Memory could not be revised" }); }
  });

  app.post("/api/memories/:memoryId/forget", { schema: { body: memoryForgetRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { expectedRevision: number; evidenceRefs: string[] };
    try { return await core.forgetMemory(session, (request.params as { memoryId: string }).memoryId, { expected_revision: body.expectedRevision, evidence_refs: body.evidenceRefs }); }
    catch { return reply.code(422).send({ code: "memory_forget_failed", message: "Memory could not be forgotten" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/relationships/rollback", { schema: { body: relationshipRollbackRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { targetActorId: string; targetRevision: number; expectedRevision: number; evidenceRefs: string[] };
    try { return await core.rollbackRelationship(session, (request.params as { fluctlightId: string }).fluctlightId, { target_actor_id: body.targetActorId, target_revision: body.targetRevision, expected_revision: body.expectedRevision, evidence_refs: body.evidenceRefs }); }
    catch { return reply.code(422).send({ code: "relationship_rollback_failed", message: "Relationship could not be rolled back" }); }
  });

  app.get("/api/fluctlights/:fluctlightId/autonomy-actions", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.listAutonomyActions(session, (request.params as { fluctlightId: string }).fluctlightId); }
    catch { return reply.code(403).send({ code: "autonomy_actions_unavailable", message: "Autonomy actions are unavailable" }); }
  });

  app.post("/api/autonomy-actions/:actionId/govern", { schema: { body: autonomyGovernanceRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { status: "paused" | "deferred" | "cancelled"; reason: string };
    try { return await core.governAutonomyAction(session, (request.params as { actionId: string }).actionId, body); }
    catch { return reply.code(422).send({ code: "autonomy_governance_failed", message: "Autonomy action could not be governed" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/events", { schema: { body: lifeEventRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { kind: string; startAt: string; endAt: string; scene?: string; activity?: string; location?: string; evidenceRefs: string[] };
    try { return await core.createLifeEvent(session, (request.params as { fluctlightId: string }).fluctlightId, { kind: body.kind, start_at: body.startAt, end_at: body.endAt, scene: body.scene, activity: body.activity, location: body.location, evidence_refs: body.evidenceRefs }); }
    catch { return reply.code(422).send({ code: "life_event_failed", message: "Life event could not be created" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/events/:eventId/cancel", async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { await core.cancelLifeEvent(session, (request.params as { fluctlightId: string; eventId: string }).fluctlightId, (request.params as { eventId: string }).eventId); return reply.code(204).send(); }
    catch { return reply.code(422).send({ code: "life_event_cancel_failed", message: "Life event could not be cancelled" }); }
  });

  app.put("/api/fluctlights/:fluctlightId/presence", { schema: { body: presenceRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { currentTask?: string; userPresence?: string };
    try { return await core.setLifePresence(session, (request.params as { fluctlightId: string }).fluctlightId, { current_task: body.currentTask, user_presence: body.userPresence }); }
    catch { return reply.code(422).send({ code: "life_presence_failed", message: "Presence could not be updated" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/schedules", { schema: { body: scheduleRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const body = request.body as { localDate: string; timezone: string; items: Array<Record<string, unknown>>; evidenceRefs: string[]; expectedRevision?: number; completedBefore?: string };
    try {
      return await core.acceptLifeSchedule(session, (request.params as { fluctlightId: string }).fluctlightId, {
        local_date: body.localDate,
        timezone: body.timezone,
        items: body.items.map((item) => ({ start_at: item.startAt, end_at: item.endAt, activity: item.activity, scene: item.scene, item_type: item.itemType, status: item.status, priority: item.priority, flexibility: item.flexibility, interruption_cost: item.interruptionCost })),
        evidence_refs: body.evidenceRefs,
        expected_revision: body.expectedRevision,
        completed_before: body.completedBefore,
      });
    } catch { return reply.code(422).send({ code: "schedule_accept_failed", message: "Schedule could not be accepted" }); }
  });

  app.post("/api/fluctlights/:fluctlightId/schedules/:scheduleId/cancel", { schema: { body: Type.Object({ expectedRevision: Type.Integer({ minimum: 0 }) }) } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { await core.cancelLifeSchedule(session, (request.params as { fluctlightId: string; scheduleId: string }).fluctlightId, (request.params as { scheduleId: string }).scheduleId, (request.body as { expectedRevision: number }).expectedRevision); return reply.code(204).send(); }
    catch { return reply.code(422).send({ code: "schedule_cancel_failed", message: "Schedule could not be cancelled" }); }
  });

  app.get("/api/diagnostics/workflows", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const query = request.query as { query?: string };
    try { return await core.listWorkflows(session, query.query ?? ""); }
    catch { return reply.code(502).send({ code: "workflow_runtime_unavailable", message: "Workflow runtime is unavailable" }); }
  });

  app.get("/api/diagnostics/workflows/:workflowId/status", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.workflowStatus(session, (request.params as { workflowId: string }).workflowId); }
    catch { return reply.code(422).send({ code: "workflow_status_failed", message: "Workflow status is unavailable" }); }
  });

  app.get("/api/diagnostics/workflows/:workflowId/history", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.workflowHistory(session, (request.params as { workflowId: string }).workflowId); }
    catch { return reply.code(422).send({ code: "workflow_history_failed", message: "Workflow history is unavailable" }); }
  });

  for (const action of ["pause", "resume", "cancel"] as const) {
    app.post(`/api/diagnostics/workflows/:workflowId/${action}`, async (request, reply) => {
      if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
      const session = request.cookies[sessionCookie];
      if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
      try { await core.workflowCommand(session, (request.params as { workflowId: string }).workflowId, action); return reply.code(204).send(); }
      catch { return reply.code(422).send({ code: "workflow_command_failed", message: "Workflow command failed" }); }
    });
  }

  app.post("/api/diagnostics/workflows/:workflowId/reset", { schema: { body: workflowResetRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.resetWorkflow(session, (request.params as { workflowId: string }).workflowId, (request.body as { historyPoint: number }).historyPoint); }
    catch { return reply.code(422).send({ code: "workflow_reset_failed", message: "Workflow reset failed" }); }
  });

  app.post("/api/diagnostics/workflows/:workflowId/restart", async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.restartWorkflow(session, (request.params as { workflowId: string }).workflowId); }
    catch { return reply.code(422).send({ code: "workflow_restart_failed", message: "Workflow restart failed" }); }
  });

  app.post("/api/moments/:momentId/reactions", { schema: { body: momentReactionRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try { return await core.reactToMoment(session, (request.params as { momentId: string }).momentId, (request.body as { kind?: string }).kind ?? "like"); }
    catch { return reply.code(422).send({ code: "moment_reaction_failed", message: "Moment reaction could not be saved" }); }
  });

  for (const action of ["hide", "restore"] as const) {
    app.post(`/api/moments/:momentId/${action}`, async (request, reply) => {
      if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
      const session = request.cookies[sessionCookie];
      if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
      try { await core.setMomentStatus(session, (request.params as { momentId: string }).momentId, action); return reply.code(204).send(); }
      catch { return reply.code(422).send({ code: "moment_status_failed", message: "Moment status could not be changed" }); }
    });
  }

  app.get("/api/conversations/:conversationId/messages", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { conversationId: string };
    const query = request.query as { beforeSequence?: string; limit?: string };
    try {
      const page = await core.conversationHistory(session, params.conversationId, query.beforeSequence ? Number(query.beforeSequence) : undefined, query.limit ? Number(query.limit) : 50);
      return browserPage(page);
    } catch {
      return reply.code(404).send({ code: "conversation_not_found", message: "Conversation is unavailable" });
    }
  });

  app.post("/api/conversations/:conversationId/read", { schema: { body: readPositionRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { conversationId: string };
    const body = request.body as { readSequence: number; deliveredSequence?: number };
    try {
      await core.markConversationRead(session, params.conversationId, { read_sequence: body.readSequence, delivered_sequence: body.deliveredSequence });
      return reply.code(204).send();
    } catch {
      return reply.code(403).send({ code: "conversation_read_failed", message: "Read state is unavailable" });
    }
  });

  app.post("/api/conversations/:conversationId/turn", { schema: { body: conversationTurnRequest } }, async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { conversationId: string };
    const body = request.body as { text: string; fluctlightId: string; attachmentRefs?: string[]; idempotencyKey: string; turnId?: string };
    const abortController = new AbortController();
    request.raw.once("aborted", () => abortController.abort());
    reply.raw.once("close", () => {
      if (!reply.raw.writableEnded) abortController.abort();
    });
    try {
      const upstream = await core.acceptConversationTurn(session, params.conversationId, {
        text: body.text,
        fluctlight_id: body.fluctlightId,
        attachment_refs: body.attachmentRefs ?? [],
        idempotency_key: body.idempotencyKey,
        turn_id: body.turnId,
      }, abortController.signal);
      reply.header("content-type", "application/x-ndjson; charset=utf-8");
      return reply.send(translateCoreNdjson(upstream, abortController.signal));
    } catch {
      if (abortController.signal.aborted || reply.raw.destroyed) return;
      return reply.code(502).send({ code: "conversation_turn_failed", message: "The conversation turn failed" });
    }
  });

  app.get("/api/diagnostics", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const query = request.query as { limit?: string; correlationId?: string; fluctlightId?: string };
    try {
      const events = await core.readDiagnostics(session, {
        limit: query.limit ? Number(query.limit) : 100,
        correlation_id: query.correlationId,
        fluctlight_id: query.fluctlightId,
      });
      return events.map((event) => ({
        id: event.id,
        eventType: event.event_type,
        severity: event.severity,
        fluctlightId: event.fluctlight_id,
        causationId: event.causation_id,
        correlationId: event.correlation_id,
        payload: event.payload,
        createdAt: event.created_at,
      }));
    } catch {
      return reply.code(403).send({ code: "diagnostics_unavailable", message: "Diagnostics are unavailable" });
    }
  });

  app.delete("/api/diagnostics", async (request, reply) => {
    if (rejectUntrustedMutation(request.headers.origin, options.trustedOrigin, request.cookies[csrfCookie], request.headers["x-csrf-token"])) {
      return reply.code(403).send({ code: "invalid_origin", message: "Origin is not allowed" });
    }
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try {
      return { cleared: await core.clearDiagnostics(session) };
    } catch {
      return reply.code(403).send({ code: "diagnostics_clear_failed", message: "Diagnostics could not be cleared" });
    }
  });

  app.get("/api/diagnostics/model-runs", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const query = request.query as { limit?: string; correlationId?: string };
    try {
      const rows = await core.readDiagnosticModelRuns(session, {
        limit: query.limit ? Number(query.limit) : 100,
        correlation_id: query.correlationId,
      });
      return rows.map((row) => ({
        id: row.id, role: row.role, endpointId: row.endpoint_id, modelId: row.model_id,
        prompt: row.prompt, response: row.response, status: row.status, errorCode: row.error_code,
        correlationId: row.correlation_id, createdAt: row.created_at,
      }));
    } catch { return reply.code(403).send({ code: "diagnostics_unavailable", message: "Diagnostics are unavailable" }); }
  });

  app.get("/api/diagnostics/export", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const query = request.query as { limit?: string; correlationId?: string };
    try {
      return await core.exportDiagnostics(session, {
        limit: query.limit ? Number(query.limit) : 500,
        correlation_id: query.correlationId,
      });
    } catch { return reply.code(403).send({ code: "diagnostics_unavailable", message: "Diagnostics are unavailable" }); }
  });

  app.get("/api/media/:assetId", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    const params = request.params as { assetId: string };
    const range = request.headers.range;
    try {
      const upstream = await core.readMedia(session, params.assetId, range);
      reply.header("content-type", upstream.headers.get("content-type") ?? "application/octet-stream");
      for (const name of ["content-length", "content-range", "accept-ranges", "etag"]) {
        const value = upstream.headers.get(name);
        if (value) reply.header(name, value);
      }
      return reply.code(upstream.status).send(upstream.body);
    } catch {
      return reply.code(404).send({ code: "media_unavailable", message: "Media is unavailable" });
    }
  });

  return app;
}
