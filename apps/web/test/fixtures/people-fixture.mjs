// Isolated People/Actor UI transport. No production imports or real model calls.
export function peopleFixture({
  owner = "people-owner",
  scenario = "normal",
  baseDetail,
  worldState,
  externalMemories = false,
} = {}) {
  const stores = new Map(),
    resources = new Map(),
    calls = [],
    receipts = new Map(),
    clone = (v) => JSON.parse(JSON.stringify(v));
  let sequence = 0,
    firstCatalog = 0,
    firstAbout = 0,
    lost = false;
  const stamp = () => new Date().toISOString();
  const backgroundKeys = [
    "name",
    "full_name",
    "age",
    "gender",
    "occupation",
    "background",
    "interests",
    "preferences",
    "personality",
    "habits",
    "relationship_description",
    "shared_information",
    "wants_you_to_know",
    "rules",
    "location_scope",
    "location",
    "timezone",
    "relationship_distance",
    "meeting_confirmed",
  ];
  function state(fid) {
    if (!stores.has(fid)) {
      const seed = baseDetail?.(fid) || {
        id: fid,
        status: "active",
        current_revision: 4,
        identity: { name: "林青", timezone: "Asia/Shanghai" },
        personality_runtime: { active_profile_id: "sunny", revision: 2 },
        core_persona: {
          personality_system: {
            profiles: [
              { id: "sunny", name: "日常人格" },
              { id: "focused", name: "专注人格" },
            ],
          },
        },
        context: { context_revision: "life-people-1" },
      };
      const wid =
        fid === "fluctlight-fixture-one" ? "fixture-world" : fid + "-world";
      const ownWorld = worldState?.(fid) || {
        worlds: [
          {
            id: wid,
            fluctlight_id: fid,
            name: "青岚小城",
            background: "普通生活里的相遇。",
            rules: "尊重每个人的生活。",
            flavor: "现代城市",
            revision: 2,
            venue_categories: [{ id: "daily", name: "日常" }],
          },
        ],
        binding: { world_id: wid, revision: 1 },
        places: [
          {
            id: fid + "-bookshop",
            world_id: wid,
            name: "迟山书店",
            alias: "",
            status: "active",
            revision: 1,
          },
          {
            id: fid + "-riverside",
            world_id: wid,
            name: "河岸",
            alias: "",
            status: "active",
            revision: 1,
          },
        ],
        npcs: [],
        events: [],
      };
      ownWorld.npcs ||= [];
      ownWorld.events ||= [];
      const currentWorld =
        ownWorld.binding?.world_id || ownWorld.worlds[0]?.id || wid;
      if (!ownWorld.npcs.length && scenario !== "empty")
        ownWorld.npcs.push(
          ...["苏禾", "陈渡", "旧同事"].map((name, index) => ({
            id: fid + "-npc-" + index,
            world_id: currentWorld,
            actor_id: fid + "-actor-npc-" + index,
            name,
            role: ["花店店主", "常来书店的读者", "旧街的邻居"][index],
            bio: [
              "习惯把当天好看的花留在门口，喜欢听来往的人说说近况。",
              "喜欢旅行和旧书，偶尔来借一本诗集。",
              "归档资料仍保留原来的来源。",
            ][index],
            home_place_id:
              ownWorld.places[index % Math.max(1, ownWorld.places.length)]
                ?.id || null,
            routine: [
              {
                day: "daily",
                start: "09:00",
                end: "17:00",
                place_id: ownWorld.places[0]?.id || "",
                activity: "经营自己的小店",
              },
            ],
            status: index === 2 ? "archived" : "active",
            enabled: index !== 2,
            position_mode: "routine",
            revision: 3,
            source_resource_id: "source-person-" + index,
            source_version: "v1",
            local_overrides: { preserved: { customKey: true } },
            confirmed_place_id: null,
            confirmed_at: null,
            full_fluctlight: false,
            planned_position:
              index === 0
                ? {
                    place_id: ownWorld.places[0]?.id,
                    source: "schedule_prediction",
                    window_start: stamp(),
                    window_end: new Date(Date.now() + 3600000).toISOString(),
                    activity: "日常安排",
                  }
                : null,
            current_presence: null,
            runtime_reference_ids: [],
          })),
        );
      const bg =
        scenario === "empty"
          ? {}
          : {
              name: "验收用户",
              full_name: "若雨",
              age: "二十多岁",
              gender: "不透露",
              occupation: "编辑",
              relationship_description: "我填写的朋友关系",
              wants_you_to_know: "最近喜欢读短篇故事。",
              interests: "阅读",
              preferences: "直接、自然的交流",
              habits: "晚上看书",
              rules: "尊重彼此的选择",
              personality: "安静而坦率",
              background: "独立保留的背景",
              shared_information: "保留的补充信息",
              location_scope: "同城",
              location: "上海",
              timezone: "Asia/Shanghai",
              relationship_distance: "线上交流",
              meeting_confirmed: false,
            };
      const actors = new Map(),
        factsFor = (actor, background) =>
          Object.entries(background).map(([attribute, value], index) => ({
            fact_id: actor + "-fact-" + index,
            actor_id: actor,
            attribute,
            value,
            epistemic_kind: "owner_defined",
            status: "active",
            effective_at: stamp(),
            revision: 1,
            transition_kind: "assert",
            source_ref: "owner-command:initial-fixture",
          }));
      const initialFacts = factsFor(owner, bg);
      if (scenario !== "empty")
        initialFacts.push({
          fact_id: owner + "-conversation-fact",
          actor_id: owner,
          attribute: "recent_work",
          value: "你曾在交流中提过最近工作较忙。",
          epistemic_kind: "user_statement",
          status: "active",
          effective_at: stamp(),
          revision: 1,
          transition_kind: "assert",
          source_ref: "message:fixture-conversation-source",
        });
      actors.set(owner, {
        background: bg,
        facts: initialFacts,
        history: clone(initialFacts),
        version: 1,
      });
      actors.set("human-other", {
        background: { name: "另一位交流参与者", location: "保留的所在地" },
        facts: factsFor("human-other", {
          name: "另一位交流参与者",
          location: "保留的所在地",
        }),
        history: [],
        version: 1,
      });
      const active = seed.personality_runtime?.active_profile_id || "sunny",
        relation = (id, target, profile, trust, summary) => ({
          id,
          owner_fluctlight_id: fid,
          target_actor_id: target,
          profile_id: profile,
          role: { label: "朋友" },
          metrics: { trust, familiarity: 0.64, comfort: 0.78 },
          summary,
          revision: 2,
          trend: "stable",
          provenance: { source: "fixture_recorded" },
          updated_at: stamp(),
        });
      const relations =
        scenario === "empty"
          ? []
          : [
              relation(
                fid + "-owner-relation",
                owner,
                active,
                0.72,
                "能自然分享生活，也会保留彼此不同的意见。",
              ),
              relation(
                fid + "-owner-focused",
                owner,
                "focused",
                0.91,
                "另一人格范围的关系。",
              ),
              relation(
                fid + "-npc-relation",
                ownWorld.npcs[0]?.actor_id,
                null,
                0.6,
                "已经有真实关系记录。",
              ),
              relation(
                fid + "-npc-old-profile",
                ownWorld.npcs[1]?.actor_id,
                "focused",
                0.8,
                "其他人格的关系，不应冒充当前人格已结识。",
              ),
            ];
      const s = {
        seed,
        world: ownWorld,
        actors,
        relations,
        factsGeneration: 5,
        lifeRevision: "life-people-1",
        relationHistory: relations.map((r) => ({
          id: "history-" + r.id,
          relationship_id: r.id,
          revision: 2,
          base_revision: 1,
          role: clone(r.role),
          metrics: clone(r.metrics),
          summary: r.summary,
          evidence_refs: ["event:fixture-source"],
          actor_id: owner,
          created_at: stamp(),
        })),
        records: new Map(),
        memories: new Map(),
        failDetail: 0,
        failAbout: 0,
      };
      for (const npc of ownWorld.npcs)
        s.records.set(npc.id, {
          encounters: npc.id.endsWith("-1")
            ? [
                {
                  id: npc.id + "-declined",
                  world_id: npc.world_id,
                  npc_id: npc.id,
                  record_kind: "encounters",
                  summary: "一次没有获得接受的接触尝试。",
                  outcome: "declined",
                  relationship_id: null,
                  memory_id: null,
                  occurred_at: stamp(),
                  source_kind: "virtual_contact",
                },
              ]
            : [],
          presences: [],
          opportunities: npc.id.endsWith("-1")
            ? [
                {
                  id: npc.id + "-opportunity",
                  world_id: npc.world_id,
                  npc_id: npc.id,
                  record_kind: "opportunities",
                  fluctlight_id: fid,
                  status: "failed",
                  revision: 4,
                  expires_at: new Date(Date.now() + 3600000).toISOString(),
                  created_at: stamp(),
                  error_code: "fixture_processing_failed",
                  result: {},
                },
              ]
            : [],
        });
      stores.set(fid, s);
    }
    return stores.get(fid);
  }
  function context(s, actor) {
    const a = s.actors.get(actor);
    if (!a) return null;
    const rel = s.relations.filter((r) => r.target_actor_id === actor);
    return {
      actor_id: actor,
      actor_type: "human",
      background: clone(a.background),
      facts: clone(a.facts),
      history: clone(a.history),
      relationships: clone(rel),
      relationship_history: clone(
        s.relationHistory.filter((row) =>
          rel.some((r) => r.id === row.relationship_id),
        ),
      ),
      context_version:
        "ctx_" +
        actor +
        "_" +
        a.version +
        "_" +
        rel.reduce((sum, r) => sum + r.revision, 0),
    };
  }
  function detail(s, fid) {
    return {
      ...clone(s.seed),
      id: fid,
      status: s.seed.status || "active",
      actor_user: { background: clone(s.actors.get(owner).background) },
      relationships: clone(s.relations),
      current_facts_revision: "facts_gen_" + s.factsGeneration,
      context: { ...s.seed.context, context_revision: s.lifeRevision },
      editable_actor_ids: [owner],
    };
  }
  function npcDetail(s, id) {
    const npc = s.world.npcs.find(
      (n) => n.id === id && n.world_id === s.world.binding?.world_id,
    );
    if (!npc) return null;
    const relationships = s.relations.filter(
      (r) => r.target_actor_id === npc.actor_id,
    );
    return {
      ...clone(npc),
      encounters: clone(s.records.get(id)?.encounters || []),
      relationship_context: {
        actor_id: npc.actor_id,
        actor_type: "npc",
        background: {},
        facts: [],
        history: [],
        relationships: clone(relationships),
        relationship_history: clone(
          s.relationHistory.filter((row) =>
            relationships.some((r) => r.id === row.relationship_id),
          ),
        ),
        context_version: "npc-ctx-" + npc.revision,
      },
    };
  }
  async function handler({
    path,
    method = "GET",
    body = {},
    url = new URL(path, "http://people.test"),
    json,
    res,
  }) {
    const memoryId = path.match(/^\/api\/memories\/([^/]+)$/)?.[1];
    if (memoryId && method === "GET") {
      for (const s of stores.values()) {
        const row = s.memories.get(memoryId);
        if (row) {
          calls.push({ path, method, body: clone(body) });
          json(res, clone(row), 200);
          return true;
        }
      }
      return false;
    }
    const match = path.match(/^\/api\/fluctlights\/([^/]+)\/(.*)$/),
      studio = path.match(
        /^\/api\/studio\/resources(?:\/([^/]+))?(?:\/(publish|apply-preview|apply))?$/,
      );
    if (!match && !studio) return false;
    const fid = match?.[1] || body.fluctlightId || "A",
      end = match?.[2],
      s = state(fid);
    if (end?.startsWith("world/catalog") && worldState) return false;
    const supported =
      studio ||
      end === "detail" ||
      end === "world" ||
      end?.startsWith("world/catalog") ||
      end?.startsWith("world/events") ||
      end === "npcs" ||
      end?.startsWith("npcs/") ||
      end === "actor-user-background" ||
      end?.startsWith("actor-context/") ||
      (end === "memories" && !externalMemories);
    if (!supported) return false;
    calls.push({
      path,
      method,
      body: clone(body),
      query: Object.fromEntries(url.searchParams),
    });
    const reply = (value, status = 200) => {
        json(res, clone(value), status);
        return true;
      },
      fail = () =>
        reply(
          { code: "request_failed", message: "隔离人物场景读取未完成" },
          503,
        ),
      conflict = (code = "revision_conflict") =>
        reply({ code, message: "资料版本已变化，请核对最新记录" }, 409),
      missing = () =>
        reply({ code: "not_found", message: "当前范围未找到这条记录" }, 404);
    const identity = owner + ":" + path + ":" + body.idempotencyKey;
    if (method !== "GET" && body.idempotencyKey && receipts.has(identity)) {
      const old = receipts.get(identity);
      return JSON.stringify(old.body) === JSON.stringify(body)
        ? reply({ ...old.result, replayed: true })
        : conflict("idempotency_conflict");
    }
    const save = (result) => {
      receipts.set(identity, { body: clone(body), result: clone(result) });
      if (scenario === "readback-failure") s.failAbout = 1;
      if (scenario === "unknown" && !lost) {
        lost = true;
        return fail();
      }
      return reply(result);
    };
    if (end === "detail") {
      if (s.failDetail > 0) {
        s.failDetail--;
        return fail();
      }
      return reply(detail(s, fid));
    }
    if (end === "world") {
      const world = s.world.worlds.find(
        (w) => w.id === s.world.binding?.world_id,
      );
      return reply({
        available: true,
        world: world || null,
        places: s.world.places.filter((p) => p.world_id === world?.id),
        npcs: s.world.npcs.filter((p) => p.world_id === world?.id),
        scenes: [],
      });
    }
    if (end === "world/catalog")
      return reply({
        items: s.world.worlds.map((row) => ({
          ...row,
          active: row.id === s.world.binding?.world_id,
        })),
        active_binding: s.world.binding,
        next_cursor: null,
      });
    if (end?.startsWith("world/catalog/")) {
      const id = end.split("/")[2],
        world = s.world.worlds.find((row) => row.id === id);
      if (!world) return missing();
      return reply({
        available: true,
        world,
        places: s.world.places.filter((row) => row.world_id === id),
        npcs: s.world.npcs.filter((row) => row.world_id === id),
        scenes: [],
      });
    }
    if (end?.startsWith("world/events")) {
      const id = end.split("/")[2];
      if (id) {
        const row = s.world.events.find((row) => row.id === id);
        return row ? reply(row) : missing();
      }
      return reply({
        items: s.world.events.filter(
          (row) => !row.world_id || row.world_id === s.world.binding?.world_id,
        ),
        world_id: s.world.binding?.world_id || "",
        next_cursor: "",
        includes_unattributed_history: true,
      });
    }
    if (end === "npcs") {
      firstCatalog++;
      if (scenario === "loading" && firstCatalog === 1)
        await new Promise((r) => setTimeout(r, 5000));
      if (scenario === "failure" && firstCatalog === 1) return fail();
      return reply({
        items: s.world.npcs.filter(
          (n) => n.world_id === s.world.binding?.world_id,
        ),
        next_cursor: null,
      });
    }
    if (end?.startsWith("actor-context/") || end === "actor-user-background") {
      const actor =
          end === "actor-user-background"
            ? owner
            : decodeURIComponent(end.slice("actor-context/".length)),
        a = s.actors.get(actor);
      if (!a) return missing();
      if (method === "GET") {
        firstAbout++;
        if(scenario==='about-loading'&&firstAbout===1)await new Promise(r=>setTimeout(r,5000));
        if (s.failAbout > 0) {
          s.failAbout--;
          return fail();
        }
        if (scenario === "about-failure" && firstAbout === 1) return fail();
        return reply(context(s, actor));
      }
      if (
        end === "actor-user-background"
          ? body.expectedCurrentFactsRevision !==
            "facts_gen_" + s.factsGeneration
          : body.expectedContextVersion !== context(s, actor).context_version
      )
        return conflict(
          end === "actor-user-background"
            ? "actor_user_background_conflict"
            : "actor_context_conflict",
        );
      for (const [attribute, value] of Object.entries(body.background || {})) {
        if (!backgroundKeys.includes(attribute))
          return reply(
            { code: "invalid_request", message: "不支持的自填字段" },
            400,
          );
        const old = a.facts.find((f) => f.attribute === attribute);
        if (old && old.value === value) continue;
        if (old) {
          old.status = body.operation === "correct" ? "corrected" : "active";
          a.facts = a.facts.filter((f) => f.fact_id !== old.fact_id);
          const history = a.history.find((f) => f.fact_id === old.fact_id);
          if (history) {
            history.status = old.status;
            history.valid_until = stamp();
          }
        }
        const fact = {
          fact_id: "people-fact-" + ++sequence,
          actor_id: actor,
          attribute,
          value,
          epistemic_kind: "owner_defined",
          status: "active",
          effective_at: stamp(),
          revision: 1,
          transition_kind: old ? body.operation : "assert",
          source_ref: "owner-command:people-" + sequence,
        };
        a.facts.push(fact);
        a.history.unshift(clone(fact));
        a.background[attribute] = value;
      }
      a.version++;
      s.factsGeneration++;
      return save(
        end === "actor-user-background"
          ? {
              background: a.background,
              facts: a.facts,
              current_facts_revision: "facts_gen_" + s.factsGeneration,
              replayed: false,
            }
          : context(s, actor),
      );
    }
    if (end === "memories")
      return reply({
        items: [...s.memories.values()].filter(
          (row) =>
            url.searchParams.get("aboutOwner") !== "true" ||
            row.actor_refs.includes(owner),
        ),
        next_cursor: "",
      });
    const npcMatch = end?.match(
      /^npcs\/([^/]+)(?:\/(encounters|presences|opportunities)(?:\/([^/]+)(?:\/(retry))?)?)?$/,
    );
    if (npcMatch) {
      const [, id, kind, recordId, retry] = npcMatch,
        npc = s.world.npcs.find(
          (n) => n.id === id && n.world_id === s.world.binding?.world_id,
        );
      if (!npc) return missing();
      const records = s.records.get(id) || {
        encounters: [],
        presences: [],
        opportunities: [],
      };
      if (kind) {
        if (kind === "encounters" && method === "POST") {
          if (body.expectedLifeContextRevision !== s.lifeRevision)
            return conflict();
          if (!body.confirmed)
            return reply(
              { code: "invalid_request", message: "接触未确认" },
              400,
            );
          const eventId = "people-event-" + ++sequence,
            encounterId = "people-encounter-" + sequence,
            memoryId = "people-memory-" + sequence;
          let relation = s.relations.find(
            (r) => r.target_actor_id === npc.actor_id && r.profile_id == null,
          );
          if (!relation) {
            relation = {
              id: "people-relation-" + sequence,
              owner_fluctlight_id: fid,
              target_actor_id: npc.actor_id,
              profile_id: null,
              role: { label: "相识" },
              metrics: {},
              summary: body.summary,
              revision: 1,
              provenance: { source: "owner_confirmed_encounter" },
            };
            s.relations.push(relation);
          }
          const record = {
            id: encounterId,
            world_id: npc.world_id,
            npc_id: id,
            record_kind: "encounters",
            summary: body.summary,
            occurred_at: body.endAt,
            life_event_id: eventId,
            relationship_id: relation.id,
            memory_id: memoryId,
            source_kind: "owner_confirmed_encounter",
            outcome: "contacted",
            evidence_refs: body.evidenceRefs,
          };
          records.encounters.unshift(record);
          s.records.set(id, records);
          npc.confirmed_place_id = body.placeId;
          npc.confirmed_at = body.endAt;
          npc.revision++;
          s.lifeRevision = "life-people-" + ++sequence;
          s.world.events.unshift({
            id: eventId,
            fluctlight_id: fid,
            world_id: npc.world_id,
            title: body.summary,
            kind: "npc_encounter",
            status: "completed",
            context_scope: "life",
            start_at: body.startAt,
            end_at: body.endAt,
            revision: 1,
            result: {
              npc_id: id,
              source: "owner_confirmed_encounter",
              owner_copresent: false,
            },
            evidence_refs: body.evidenceRefs,
            impacts: [{ kind: "memory", id: memoryId }],
          });
          s.memories.set(memoryId, {
            id: memoryId,
            owner_fluctlight_id: fid,
            type: "episodic",
            content: body.summary,
            status: "active",
            revision: 1,
            confidence: 1,
            importance: 0.5,
            emotional_significance: 0,
            actor_refs: [fid, npc.actor_id],
            event_refs: [eventId],
            evidence_refs: [eventId],
            visibility: "private",
            created_at: stamp(),
          });
          return save({
            id: encounterId,
            event_id: eventId,
            npc_id: id,
            relationship_id: relation.id,
            memory_id: memoryId,
            status: "recorded",
            source: "owner_confirmed_encounter",
            outcome: "contacted",
            audit_id: "people-encounter-audit",
          });
        }
        if (!recordId) return reply({ items: records[kind], next_cursor: "" });
        const row = records[kind].find((r) => r.id === recordId);
        if (!row) return missing();
        if (!retry) return reply(row);
        if (row.revision !== body.expectedRevision) return conflict();
        if (
          !["failed", "deferred"].includes(row.status) ||
          Date.parse(row.expires_at) <= Date.now()
        )
          return reply(
            { code: "invalid_request", message: "原窗口不可重新判断" },
            400,
          );
        row.status = "pending";
        row.revision++;
        return save({
          id: row.id,
          revision: row.revision,
          status: "pending",
          source: "owner_requested_reassessment",
          acquaintance_created: false,
          audit_id: "people-opportunity-audit",
        });
      }
      if (method === "GET") return reply(npcDetail(s, id));
      if (body.expectedRevision !== npc.revision) return conflict();
      if (method === "DELETE") {
        if (
          s.relations.some((r) => r.target_actor_id === npc.actor_id) ||
          records.encounters.length
        )
          return reply(
            {
              code: "resource_referenced",
              message: "人物已有历史引用，请使用归档",
            },
            409,
          );
        s.world.npcs = s.world.npcs.filter((n) => n.id !== id);
        return save({
          id,
          status: "deleted",
          actor_id: npc.actor_id,
          audit_id: "people-delete-audit",
        });
      }
      for (const [input, field] of Object.entries({
        name: "name",
        role: "role",
        background: "bio",
        homePlaceId: "home_place_id",
        routine: "routine",
        status: "status",
        enabled: "enabled",
        positionMode: "position_mode",
      }))
        if (Object.hasOwn(body, input)) npc[field] = clone(body[input]);
      npc.revision++;
      npc.current_presence = null;
      return save({
        id,
        actor_id: npc.actor_id,
        revision: npc.revision,
        status: "saved",
        history_preserved: true,
        audit_id: "people-save-audit",
      });
    }
    if (studio) {
      const [, id, operation] = studio;
      let row = id ? resources.get(id) : null;
      if (id && !row) return missing();
      if (method === "GET") return reply(row);
      if (operation === "publish") {
        if (body.expectedRevision !== row.revision) return conflict();
        row.published_version = row.draft_version;
        row.revision++;
        row.versions.unshift({
          version: row.published_version,
          kind: "npc",
          name: row.name,
          body: clone(row.body),
          immutable: true,
        });
        return save({
          id: row.id,
          revision: row.revision,
          published_version: row.published_version,
          status: "published",
        });
      }
      if (operation === "apply-preview") {
        const current = s.world.npcs.find(
          (n) =>
            n.source_resource_id === id &&
            n.world_id === s.world.binding?.world_id,
        );
        return reply({
          available: true,
          diff: [
            { field: "name", from: current?.name || null, to: row.name },
            { field: "role", from: current?.role || null, to: row.body.role },
            {
              field: "background",
              from: current?.bio || null,
              to: row.body.background,
            },
          ],
          conflicts: [],
          instance_revision: s.seed.current_revision || 4,
          runtime_revision: current?.revision || 0,
          runtime_id: current?.id || null,
        });
      }
      if (operation === "apply") {
        const current = s.world.npcs.find(
          (n) =>
            n.source_resource_id === id &&
            n.world_id === s.world.binding?.world_id,
        );
        if (
          body.expectedInstanceRevision !== (s.seed.current_revision || 4) ||
          body.expectedRuntimeRevision !== (current?.revision || 0)
        )
          return conflict();
        if (!row.versions.some((v) => v.version === body.resourceVersion))
          return reply(
            { code: "invalid_request", message: "发布版本不存在" },
            400,
          );
        const npc = current || {
          id: "applied-npc-" + ++sequence,
          world_id: s.world.binding?.world_id,
          actor_id: "applied-actor-" + sequence,
          name: row.name,
          role: row.body.role || "",
          bio: row.body.background || "",
          home_place_id: body.homePlaceId || null,
          routine: [],
          status: "active",
          enabled: true,
          position_mode: "unknown",
          revision: 0,
          source_resource_id: id,
          source_version: body.resourceVersion,
          local_overrides: {},
          confirmed_place_id: null,
          confirmed_at: null,
          full_fluctlight: false,
          current_presence: null,
          runtime_reference_ids: [],
        };
        npc.revision++;
        if (!current) {
          s.world.npcs.push(npc);
          s.records.set(npc.id, {
            encounters: [],
            presences: [],
            opportunities: [],
          });
        }
        return save({
          id: npc.id,
          fluctlight_id: fid,
          status: "applied",
          applied: true,
          runtime_revision: npc.revision,
          resource_id: id,
          resource_version: body.resourceVersion,
        });
      }
      if (!row) {
        row = {
          id: "person-resource-" + ++sequence,
          kind: "npc",
          name: body.name,
          body: clone(body.body),
          revision: 1,
          draft_version: "v1",
          published_version: null,
          versions: [],
          status: "active",
        };
        resources.set(row.id, row);
      } else {
        if (body.expectedVersion !== row.draft_version) return conflict();
        row.name = body.name;
        row.body = clone(body.body);
        row.revision++;
        row.draft_version = "v" + row.revision;
      }
      return save(row);
    }
    return false;
  }
  return { handler, state, resources, calls };
}
