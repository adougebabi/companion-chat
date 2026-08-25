import { CoreClient } from "@fluctlight/core-client";
import { Type } from "@sinclair/typebox";
import cookie from "@fastify/cookie";
import { randomBytes, timingSafeEqual } from "node:crypto";
import Fastify, { type FastifyInstance } from "fastify";
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
const passwordRequest = Type.Object({ password: Type.String({ minLength: 12 }) });
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
const conversationCreateRequest = Type.Object({
  title: Type.Optional(Type.String({ maxLength: 256 })),
  participantActorIds: Type.Optional(Type.Array(Type.String({ minLength: 1, maxLength: 128 }), { maxItems: 1 })),
});
const fluctlightCreateRequest = Type.Object({
  id: Type.Optional(Type.String({ minLength: 1, maxLength: 128 })),
  name: Type.Optional(Type.String({ maxLength: 256 })),
});
const conversationTurnRequest = Type.Object({
  text: Type.String({ minLength: 1, maxLength: 32_000 }),
  fluctlightId: Type.Optional(Type.String({ minLength: 1, maxLength: 128 })),
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
      reply.setCookie(csrfCookie, newCsrfToken(), {
        httpOnly: false,
        secure: secureCookies,
        sameSite: "lax",
        path: "/",
        maxAge: 60 * 60 * 24,
      });
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
    reply.clearCookie(csrfCookie, { path: "/", httpOnly: false, sameSite: "lax", secure: secureCookies });
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
      reply.clearCookie(csrfCookie, { path: "/", httpOnly: false, sameSite: "lax", secure: secureCookies });
      return reply.code(204).send();
    } catch {
      return reply.code(403).send({ code: "revoke_failed", message: "Session revocation failed" });
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
      reply.setCookie(csrfCookie, newCsrfToken(), { httpOnly: false, secure: secureCookies, sameSite: "lax", path: "/", maxAge: 60 * 60 * 24 });
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
      reply.setCookie(csrfCookie, newCsrfToken(), { httpOnly: false, secure: secureCookies, sameSite: "lax", path: "/", maxAge: 60 * 60 * 24 });
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
    const body = request.body as { title?: string; participantActorIds?: string[] };
    try {
      return browserPage(await core.createConversation(session, {
        title: body.title,
        participant_actor_ids: body.participantActorIds ?? [],
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

  app.get("/api/fluctlights", async (request, reply) => {
    const session = request.cookies[sessionCookie];
    if (!session) return reply.code(401).send({ code: "unauthenticated", message: "Authentication is required" });
    try {
      return await core.listFluctlights(session);
    } catch {
      return reply.code(403).send({ code: "fluctlights_unavailable", message: "Fluctlights are unavailable" });
    }
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
    const body = request.body as { text: string; fluctlightId?: string; attachmentRefs?: string[]; idempotencyKey: string; turnId?: string };
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
