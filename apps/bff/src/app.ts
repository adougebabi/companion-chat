import { CoreClient } from "@fluctlight/core-client";
import { Type } from "@sinclair/typebox";
import Fastify, { type FastifyInstance } from "fastify";

export type BffOptions = {
  coreBaseUrl: string;
  coreServiceKey: string;
  fetcher?: typeof fetch;
};

const healthResponse = Type.Object({
  status: Type.String(),
  role: Type.Literal("bff"),
});

export function createBff(options: BffOptions): FastifyInstance {
  const app = Fastify({ logger: true });
  const core = new CoreClient(options.coreBaseUrl, options.coreServiceKey, options.fetcher);

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

  return app;
}
