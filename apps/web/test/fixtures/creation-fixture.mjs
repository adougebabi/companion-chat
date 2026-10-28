// Opt-in isolated initialization DTOs. Product code never imports this file.
const clone = (v) => JSON.parse(JSON.stringify(v));
export function creationHttpFixture({
  owner = "human-fixture-owner",
  scenario = "normal",
  instances = [],
  groups = [],
  baseDetail = () => ({}),
} = {}) {
  const records = new Map(),
    receipts = new Map(),
    calls = [],
    worlds = new Map();
  let serial = 0,
    firstAnalysis = true,
    firstActivation = true,
    firstRecovery = true,
    firstCatalog = true,
    firstCopy = true;
  const sourceId = "initialization_source_" + "c".repeat(32),
    date = "2026-10-28T01:00:00Z";
  const analysis = () => ({
    analysis_id: sourceId,
    correlation_id: "creation-fixture-analysis",
    schema_version: 2,
    core_persona: {
      identity: {
        name: "林青",
        nickname: "小青",
        age: 27,
        gender: "female",
        occupation: "图书编辑",
        timezone: "Asia/Shanghai",
        background_story: "在河畔书店工作，喜欢记录沿途见闻。",
        core_values: ["真诚", "尊重边界"],
      },
      personality: {
        openness: 0.78,
        conscientiousness: 0.72,
        extraversion: 0.4,
        agreeableness: 0.76,
        neuroticism: 0.3,
        custom_dimension: 0,
      },
      behavioral_policy: {
        response_style: "自然随和",
        directness: 0.6,
        initiative: 0.4,
      },
      life_profile: {
        life_habits: "空闲时阅读与绘画。",
        preferences: { likes: ["水彩", "旧书"], dislikes: [], custom: false },
        appearance: { description: "外貌留待明确的视觉定义。" },
        recurring_commitments: [],
        wardrobe_items: [
          {
            description: "亚麻衬衫",
            category: "top",
            slot: "upper_body",
            ownership: "owned",
            currently_worn: false,
            custom: { keep: true },
          },
        ],
      },
      personality_system: { mode: "single", profiles: [] },
    },
    developing_self: {},
    extensions: { future: { enabled: false, score: 0, empty: null } },
    initial_relationships: [
      {
        target_actor_id: owner,
        role: "朋友",
        metrics: { trust: 0.3 },
        summary: "初次相识",
        custom: { keep: true },
      },
    ],
    initial_goals: [
      {
        description: "完成一幅水彩作品",
        importance: 0.6,
        urgency: 0.3,
        scope: "general",
      },
    ],
    initial_intentions: [
      {
        description: "整理阅读桌",
        scope: "general",
        importance: 0.3,
        urgency: 0.2,
      },
    ],
  });
  const page = (items) => ({ items: clone(items), next_cursor: "" });
  const publishedWorld = {
    id: "creation-world",
    version: "published:world:1",
    kind: "world",
    name: "河畔小镇",
    body: {
      schema: "world-definition.v1",
      background: "沿着河流展开的小镇日常。",
      rules: "各自生活，真实相处。",
      framework: {
        schema: "world-framework.v1",
        venues: [
          {
            key: "home",
            name: "住处",
            places: [
              { key: "study", name: "书房", description: "有窗与阅读桌" },
            ],
          },
        ],
      },
    },
    immutable: true,
  };
  const template = {
    id: "creation-template",
    version: "published:template:1",
    kind: "character_template",
    name: "安静的记录者",
    body: {
      schema: "character-template.v1",
      description: "喜欢观察和记录日常的人。",
    },
    immutable: true,
  };
  const sourceWorld = {
    id: "creation-source-world",
    fluctlight_id: "fluctlight-fixture-one",
    name: "已有角色的小镇",
    revision: 3,
    background: "书店、住处与河边小路。",
    rules: "日常生活的规则。",
  };
  const stateFor = (id) => records.get(id);
  const wardrobe = (s) => ({
    fluctlight_id: s.id,
    status: s.ready
      ? "completed"
      : ["wardrobe-failure", "recovery-unknown"].includes(scenario)
        ? "failed"
        : "running",
    can_recover:
      !s.ready && ["wardrobe-failure", "recovery-unknown"].includes(scenario),
    expected_revision: 7,
    revision: 7,
    last_error:
      !s.ready && ["wardrobe-failure", "recovery-unknown"].includes(scenario)
        ? "provider_request_failed"
        : null,
    request_id: "creation-wardrobe-task",
    items: [],
  });
  async function handler({ path, url, method, body = {}, json, res }) {
    const match = path.match(/^\/api\/fluctlights\/([^/]+)(?:\/(.*))?$/),
      fid = match && decodeURIComponent(match[1]),
      tail = match?.[2] || "",
      state = stateFor(fid);
    const owns =
      /^\/api\/actor-groups\/[^/]+\/members(?:\/[^/]+)?$/.test(path) ||
      path.startsWith("/api/fluctlight-creations/") ||
      (path === "/api/diagnostics/lifecycle" &&
        records.has(url.searchParams.get("fluctlightId"))) ||
      state ||
      (path === "/api/studio/resources" &&
        ["world", "character_template"].includes(
          url.searchParams.get("kind"),
        )) ||
      path.startsWith("/api/studio/resources/creation-") ||
      path === "/api/media-assets/creation-reference" ||
      path === "/api/media-assets/creation-sheet" ||
      (match &&
        [
          "world/catalog",
          "world",
          "world/catalog/creation-source-world",
        ].includes(tail) &&
        instances.some((i) => i.id === fid));
    if (!owns) return false;
    calls.push({
      path,
      method,
      query: Object.fromEntries(url.searchParams),
      body: clone(body),
    });
    const send = (value, status = 200) => {
      json(res, clone(value), status);
      return true;
    };
    const key = body.idempotencyKey || body.requestId;
    if (method !== "GET" && key && receipts.has(key))
      return send(receipts.get(key));
    const ack = (value, { unknown = false } = {}) => {
      if (key) receipts.set(key, clone(value));
      return unknown
        ? send({ code: "fixture_response_lost" }, 503)
        : send(value);
    };
    if (path.startsWith("/api/actor-groups/")) {
      const group = groups.find(
        (g) => g.id === decodeURIComponent(path.split("/")[3]),
      );
      if (!group) return send({ code: "not_found" }, 404);
      if (method === "POST") {
        group.actor_ids ||= [];
        if (!group.actor_ids.includes(body.actorId))
          group.actor_ids.push(body.actorId);
      } else if (method === "DELETE")
        group.actor_ids = (group.actor_ids || []).filter(
          (id) => id !== decodeURIComponent(path.split("/").at(-1)),
        );
      return send({});
    }
    if (path === "/api/studio/resources")
      return send(
        page(
          scenario === "empty"
            ? []
            : [
                url.searchParams.get("kind") === "world"
                  ? {
                      ...publishedWorld,
                      draft_version: "draft:1",
                      published_version: publishedWorld.version,
                      status: "published",
                      revision: 2,
                    }
                  : {
                      ...template,
                      draft_version: "draft:1",
                      published_version: template.version,
                      status: "published",
                      revision: 2,
                    },
              ],
        ),
      );
    if (path.startsWith("/api/studio/resources/creation-")) {
      const parts = path.split("/"),
        isWorld = parts[4] === "creation-world",
        resource = isWorld ? publishedWorld : template;
      if (parts[5] === "versions") return send(resource);
      if (parts[5] === "apply-preview")
        return send({
          create_required: true,
          target_kind: "world",
          instance_revision: 0,
          runtime_revision: 0,
          diff: [
            { field: "background", from: null, to: resource.body.background },
          ],
        });
      if (parts[5] === "apply") {
        const s = stateFor(body.fluctlightId);
        if (!s) return send({ code: "not_found" }, 404);
        const w = {
          id: "creation-runtime-world-" + s.id,
          fluctlight_id: s.id,
          name: resource.name,
          revision: 1,
          background: resource.body.background,
          source_resource_id: resource.id,
          source_version: resource.version,
        };
        worlds.set(s.id, w);
        return ack({
          id: w.id,
          applied: true,
          resource_id: resource.id,
          resource_version: resource.version,
          instance_revision: 1,
          runtime_revision: 1,
        });
      }
    }
    if (path === "/api/fluctlight-creations/analysis") {
      if (firstAnalysis) {
        firstAnalysis = false;
        if (scenario === "analysis-loading")
          await new Promise((r) => setTimeout(r, 2200));
        if (scenario === "analysis-cancel")
          await new Promise((r) => setTimeout(r, 15000));
        if (scenario === "analysis-failure")
          return send(
            {
              code: "analysis_unavailable",
              message: "样本分析暂时不可用",
              details: { correlation_id: "creation-fixture-failed-analysis" },
            },
            503,
          );
      }
      const value = analysis();
      if (body.json?.core_persona)
        value.core_persona = {
          ...value.core_persona,
          ...clone(body.json.core_persona),
        };
      return send(value);
    }
    if (path === "/api/fluctlight-creations/activate") {
      if (scenario === "activation-rejected" && firstActivation) {
        firstActivation = false;
        return send(
          { code: "initialization_persona_invalid", message: "样本设定需要修正" },
          422,
        );
      }
      const id = "creation-character-" + ++serial,
        name = body.name || "林青",
        identity = body.corePersona?.identity || {
          name,
          timezone: body.initializationTimezone,
        };
      const s = {
        id,
        body: clone(body),
        identity: { ...identity, name },
        ready: !["wardrobe-failure", "pending", "recovery-unknown"].includes(
          scenario,
        ),
        created_at: date,
      };
      records.set(id, s);
      instances.push({
        id,
        status: "active",
        identity: clone(s.identity),
        unread_count: 0,
      });
      const result = { id, status: "active", identity: clone(s.identity) };
      const unknown = scenario === "activation-unknown" && firstActivation;
      firstActivation = false;
      return ack(result, { unknown });
    }
    if (
      path === "/api/media-assets/creation-reference" ||
      path === "/api/media-assets/creation-sheet"
    )
      return send({
        id: path.split("/").at(-1),
        kind: "image",
        readable: true,
        status: "available",
        asset_revision: 1,
        url: "/api/media/" + path.split("/").at(-1),
        version: "creation-image:1",
        owner_actor_id: owner,
      });
    if (path === "/api/diagnostics/lifecycle")
      return send({
        workflowIntents: stateFor(url.searchParams.get("fluctlightId"))?.ready
          ? []
          : [
              {
                intentId: "creation-schedule-task",
                intentType: "schedule.current_day",
                status: "queued",
                createdAt: date,
              },
            ],
        items: [],
        next_cursor: "",
      });
    if (match && !state) {
      if (tail === "world/catalog") {
        if (firstCatalog) {
          firstCatalog = false;
          if (scenario === "world-loading")
            await new Promise((r) => setTimeout(r, 2200));
          if (scenario === "world-failure")
            return send({ code: "fixture_world_unavailable" }, 503);
        }
        return send({
          items:
            scenario === "empty"
              ? []
              : [{ ...sourceWorld, fluctlight_id: fid }],
          active_binding: { world_id: sourceWorld.id, revision: 1 },
          next_cursor: "",
        });
      }
      if (tail === "world/catalog/creation-source-world")
        return send({
          world: { ...sourceWorld, fluctlight_id: fid },
          places: [
            {
              id: "creation-source-place",
              name: "书房",
              status: "active",
              description: "靠窗的书桌",
              revision: 1,
            },
          ],
          npcs: [],
          items: [],
        });
      if (tail === "world")
        return send({
          world: { ...sourceWorld, fluctlight_id: fid },
          places: [],
          npcs: [],
        });
    }
    if (state) {
      if (tail === "detail" && method === "GET")
        return send({
          ...baseDetail(fid),
          id: fid,
          status: "active",
          identity: clone(state.identity),
          core_persona: clone(state.body.corePersona || {}),
          current_revision: 0,
          persona_preparation: { status: "ready", can_recover: false },
          initialization_mode: state.body.initializationMode,
        });
      if (tail === "world/catalog" && method === "GET")
        return send({
          items: worlds.has(fid) ? [worlds.get(fid)] : [],
          active_binding: {
            world_id: state.selectedWorld || null,
            revision: state.selectedWorld ? 1 : 0,
          },
          next_cursor: "",
        });
      if (tail === "world/catalog" && method === "POST") {
        if (firstCopy && scenario === "world-conflict") {
          sourceWorld.revision = 4;
          firstCopy = false;
        }
        if (
          body.sourceWorldId &&
          body.expectedSourceRevision !== sourceWorld.revision
        )
          return send({ code: "revision_conflict" }, 409);
        const w = {
          id: "creation-runtime-world-" + fid,
          fluctlight_id: fid,
          name: body.name,
          background: sourceWorld.background,
          rules: sourceWorld.rules,
          revision: 1,
        };
        worlds.set(fid, w);
        return ack({
          id: w.id,
          revision: 1,
          source_revision: body.expectedSourceRevision,
        });
      }
      if (tail === "world/active" && method === "PUT") {
        state.selectedWorld = body.worldId;
        return ack({
          id: body.worldId,
          world_id: body.worldId,
          revision: 1,
          life_write: false,
        });
      }
      if (tail === "world" && method === "GET")
        return send({
          world: worlds.get(fid) || null,
          active_binding: {
            world_id: state.selectedWorld || null,
            revision: state.selectedWorld ? 1 : 0,
          },
          places: [],
          npcs: [],
        });
      if (tail === "world" && method === "PUT") {
        const w = {
          id: "creation-runtime-world-" + fid,
          fluctlight_id: fid,
          name: body.name,
          background: body.background,
          revision: 1,
        };
        worlds.set(fid, w);
        state.selectedWorld = w.id;
        return ack({ id: w.id, revision: 1 });
      }
      if (tail === "inventory/preparation" && method === "GET")
        return send(wardrobe(state));
      if (tail === "wardrobe")
        return send({
          items: [],
          revision: 7,
          next_cursor: "",
          fluctlight_id: fid,
        });
      if (tail === "inventory/preparation" && method === "POST") {
        state.ready = true;
        const unknown = scenario === "recovery-unknown" && firstRecovery;
        firstRecovery = false;
        return ack(wardrobe(state), { unknown });
      }
      if (tail === "schedules")
        return send({
          days: [
            {
              local_date: url.searchParams.get("from"),
              schedule: state.ready
                ? { id: "creation-day-" + fid, revision: 1, items: [] }
                : null,
            },
          ],
        });
      if (tail === "visual-identity")
        return send({
          fluctlight_id: fid,
          current_revision: state.ready ? 1 : 0,
          status: state.ready ? "active" : "pending",
          session: {
            id: "creation-visual-session-" + fid,
            status: state.ready ? "completed" : "queued",
          },
          ...(state.ready
            ? {
                canonical_asset_id: "creation-reference",
                character_sheet_asset_id: "creation-sheet",
              }
            : {}),
        });
      if (tail === "visual-identity/timeline") return send(page([]));
      if (tail === "display-avatar")
        return send({
          fluctlight_id: fid,
          revision: state.ready ? 1 : 0,
          avatar: state.ready
            ? {
                url: "/api/media/creation-reference",
                asset_id: "creation-reference",
              }
            : null,
        });
    }
    return false;
  }
  return { handler, records, receipts, calls, worlds };
}
