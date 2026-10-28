// Isolated runtime-world transport for UI verification. Never imported by production.
export function worldFixture({
  owner = "world-owner",
  scenario = "normal",
  autoRuns = false,
} = {}) {
  const stores = new Map(),
    calls = [],
    clone = (v) => JSON.parse(JSON.stringify(v));
  let sequence = 0,
    firstRead = 0,
    lost = false;
  const now = () => new Date().toISOString();
  function state(fid) {
    if (!stores.has(fid)) {
      const wid =
        fid === "fluctlight-fixture-one" ? "fixture-world" : fid + "-world-one";
      const world = {
        id: wid,
        fluctlight_id: fid,
        name: "温和魔法世界",
        background: "中世纪魔法与普通生活并存，集市和住家构成安静的小镇。",
        rules: "尊重居民的日常生活。",
        flavor: "温暖、安静",
        description: "河岸与书店",
        item_pool: [{ customKey: { preservedValue: true } }],
        revision: 2,
        venue_categories: [
          { id: "home", name: "生活空间" },
          { id: "shops", name: "商店" },
        ],
        source_resource_id: "world-resource",
        source_version: "v1",
      };
      const places = [
        {
          id: fid + "-room",
          world_id: wid,
          name: "卧室",
          venue_category_id: "home",
          alias: "她的房间",
          floor: "二层",
          desc: "安静的木质房间",
          atmosphere: "温暖",
          furniture: [
            { customKey: "keep-camel", material: { rawValue: "木" } },
          ],
          layout: "床靠西墙，窗朝东",
          connections: [{ unusualExtension: [1, 2] }],
          room_items: [{ schemaKey: "unchanged" }],
          connection_place_ids: [],
          parent_place_id: null,
          revision: 1,
          status: "active",
          source_resource_id: null,
          source_version: null,
        },
        {
          id: fid + "-shop",
          world_id: wid,
          name: "迟山书店",
          venue_category_id: "shops",
          alias: "",
          floor: "一层",
          desc: "出售书籍与文具的小店。",
          atmosphere: "安静",
          furniture: ["书架", "柜台"],
          layout: "中间留出走道。",
          connections: [],
          room_items: [],
          connection_place_ids: [],
          parent_place_id: null,
          revision: 1,
          status: "active",
          source_resource_id: "place-resource",
          source_version: "v1",
        },
      ];
      const defaults = new Map(
        places.map((p) => [
          p.id,
          {
            revision: 1,
            definition: placeFields(p),
            source_kind: "initial_saved",
            source_resource_id: p.source_resource_id,
            source_version: p.source_version,
            created_at: now(),
          },
        ]),
      );
      const histories = new Map(
        places.map((p) => [
          p.id,
          [
            {
              id: "initial-" + p.id,
              world_id: wid,
              place_id: p.id,
              fluctlight_id: fid,
              operation: "initial_saved",
              place_revision: 1,
              default_revision: 1,
              before_value: {},
              after_value: placeFields(p),
              created_at: now(),
              reason: "建立地点与初始字段基准",
            },
          ],
        ]),
      );
      stores.set(fid, {
        worlds:
          scenario === "empty"
            ? []
            : [
                world,
                {
                  ...clone(world),
                  id: wid + "-other",
                  name: "远山森林",
                  flavor: "自然与绿意",
                  revision: 7,
                  venue_categories: [],
                  item_pool: [],
                  background: "为另一段生活保留的世界。",
                },
              ],
        binding: scenario === "empty" ? null : { world_id: wid, revision: 4 },
        places: scenario === "empty" ? [] : places,
        defaults,
        histories,
        receipts: new Map(),
        failCatalog: 0,
        failItemAppend: false,
        items:
          scenario === "empty"
            ? []
            : [
                {
                  id: fid + "-world-item",
                  fluctlight_id: fid,
                  world_id: wid,
                  name: "旅行手记",
                  definition: {
                    schema_version: "item-definition.v1",
                    item_kind: "object",
                    category: "书籍与文具",
                    slot: "",
                    description: "亚麻封面的随身笔记本，记录路上偶然的发现。",
                  },
                  available: true,
                  place_id: places[1].id,
                  revision: 3,
                  source_resource_id: "item-resource",
                  source_version: "v2",
                  local_overrides: {},
                  created_at: now(),
                  updated_at: now(),
                },
              ],
        templates:
          scenario === "empty"
            ? []
            : [
                {
                  id: fid + "-template",
                  world_id: wid,
                  name: "迟山夜读",
                  definition: {
                    category: "daily",
                    description: "带一本最近喜欢的书，来聊聊它。",
                    trigger: { schedule: "每周五19:30" },
                  },
                  enabled: true,
                  revision: 6,
                  source_resource_id: "event-resource",
                  source_version: "v3",
                  last_triggered_at: null,
                  next_check_at: now(),
                },
              ],
        runs: [],
        npcs: [],
        runReads: new Map(),
        events: [],
        context: {
          location: "原有生活位置",
          activity: "阅读",
          context_revision: "unchanged-context",
        },
      });
    }
    return stores.get(fid);
  }
  function placeFields(p) {
    return {
      name: p.name,
      description: p.desc,
      alias: p.alias || "",
      group: p.group || "",
      floor: p.floor || "",
      scene: p.scene || "",
      atmosphere: p.atmosphere || "",
      furniture: clone(p.furniture || ""),
      layout: p.layout || "",
      connections: clone(p.connections || ""),
      room_items: clone(p.room_items || ""),
    };
  }
  function observedRun(s, row) {
    if (autoRuns && row.status === "queued") {
      const reads = (s.runReads.get(row.id) || 0) + 1;
      s.runReads.set(row.id, reads);
      if (reads >= 3) {
        row.status = "applied";
        row.revision++;
        row.life_event_id = "event-" + row.id;
        row.started_at = now();
        row.end_at = now();
        row.result = {
          event_id: row.life_event_id,
          source: "virtual_world_event",
          owner_copresent: false,
          actual_start_at: row.started_at,
          actual_end_at: row.end_at,
        };
        s.events.unshift({
          id: row.life_event_id,
          fluctlight_id: row.fluctlight_id,
          world_id: row.world_id,
          title: "迟山夜读",
          kind: "world_event",
          status: "completed",
          context_scope: "ambient",
          start_at: row.started_at,
          end_at: row.end_at,
          revision: 1,
          result: clone(row.result),
          evidence_refs: ["world-event-run:" + row.id],
          impacts: [],
          impact_scope:
            "Direct recorded references; no inferred downstream causality",
        });
      }
    }
    return row;
  }
  async function handler({
    path,
    method = "GET",
    body = {},
    url = new URL(path, "http://world.test"),
    json,
    res,
  }) {
    const match = path.match(
      /^\/api\/fluctlights\/([^/]+)\/(world(?:\/.*)?|nearby)$/,
    );
    if (!match) return false;
    const [, fid, end] = match,
      s = state(fid);
    calls.push({
      path,
      method,
      body: clone(body),
      query: Object.fromEntries(url.searchParams),
    });
    const reply = (v, status = 200) => {
      json(res, clone(v), status);
      return true;
    };
    const fail = () =>
      reply({ code: "request_failed", message: "隔离世界场景读取未完成" }, 503);
    const conflict = () =>
      reply(
        { code: "revision_conflict", message: "世界或地点版本已变化" },
        409,
      );
    const missing = () =>
      reply({ code: "not_found", message: "当前范围没有这个对象" }, 404);
    const world = s.worlds.find((row) => row.id === s.binding?.world_id);
    const view = (row) => ({
      available: true,
      world: row || null,
      places: s.places.filter((p) => p.world_id === row?.id),
      npcs: s.npcs.filter((npc) => npc.world_id === row?.id),
      scenes: [],
      current_context: clone(s.context),
    });
    const save = (result) => {
      if (body.idempotencyKey)
        s.receipts.set(body.idempotencyKey, {
          path,
          method,
          body: clone(body),
          result: clone(result),
        });
      if (scenario === "readback-failure") s.failCatalog = 1;
      if (scenario === "unknown" && !lost) {
        lost = true;
        return fail();
      }
      return reply(result);
    };
    if (
      method !== "GET" &&
      body.idempotencyKey &&
      s.receipts.has(body.idempotencyKey)
    ) {
      const prior = s.receipts.get(body.idempotencyKey);
      return prior.path === path &&
        prior.method === method &&
        JSON.stringify(prior.body) === JSON.stringify(body)
        ? reply(prior.result)
        : reply(
            { code: "idempotency_conflict", message: "原提交内容不一致" },
            409,
          );
    }
    if (end === "nearby")
      return reply({
        available: true,
        world_id: world?.id || null,
        location_known: false,
        place_id: null,
        actors: [],
      });
    if (end === "world") return reply(view(world));
    if (end === "world/catalog") {
      if (method === "GET") {
        firstRead++;
        if (s.failCatalog > 0) {
          s.failCatalog--;
          return fail();
        }
        if (scenario === "loading" && firstRead === 1)
          await new Promise((r) => setTimeout(r, 5000));
        if (scenario === "failure" && firstRead === 1) return fail();
        return reply({
          items: s.worlds.map((row) => ({
            ...row,
            active: row.id === s.binding?.world_id,
          })),
          active_binding: s.binding,
          next_cursor: null,
        });
      }
      if (method === "POST") {
        const row = {
          id: fid + "-created-" + ++sequence,
          fluctlight_id: fid,
          name: body.name,
          background: body.background || "",
          rules: body.rules || "",
          flavor: body.flavor || "",
          description: body.description || "",
          item_pool: body.itemPool || [],
          venue_categories: body.venueCategories || [],
          revision: 1,
          source_resource_id: null,
          source_version: null,
        };
        s.worlds.push(row);
        return save({
          id: row.id,
          revision: 1,
          status: "created",
          active: false,
          audit_id: "world-audit",
        });
      }
    }
    if (end === "world/active" && method === "PUT") {
      if (body.expectedRevision !== (s.binding?.revision || 0))
        return conflict();
      if (!s.worlds.some((row) => row.id === body.worldId)) return missing();
      s.binding = {
        world_id: body.worldId,
        revision: (s.binding?.revision || 0) + 1,
      };
      return save({
        id: body.worldId,
        world_id: body.worldId,
        revision: s.binding.revision,
        status: "selected",
        life_write: false,
        audit_id: "binding-audit",
      });
    }
    const catalogId = end.match(/^world\/catalog\/([^/]+)$/)?.[1];
    if (catalogId) {
      const row = s.worlds.find((row) => row.id === catalogId);
      if (!row) return missing();
      if (method === "GET") return reply(view(row));
      if (body.expectedRevision !== row.revision) return conflict();
      if (method === "DELETE") {
        if (
          s.binding?.world_id === row.id ||
          s.places.some((p) => p.world_id === row.id)
        )
          return reply(
            { code: "resource_referenced", message: "世界仍有引用" },
            409,
          );
        s.worlds = s.worlds.filter((w) => w.id !== row.id);
        return save({
          id: row.id,
          status: "deleted",
          audit_id: "delete-audit",
        });
      }
      if (
        body.venueCategories &&
        row.venue_categories.some(
          (old) =>
            !body.venueCategories.some(
              (next) => next.id === old.id && next.name === old.name,
            ),
        )
      )
        return reply(
          { code: "invalid_request", message: "已有分类不能更名或移除" },
          400,
        );
      for (const key of [
        "name",
        "background",
        "rules",
        "flavor",
        "description",
      ])
        if (Object.hasOwn(body, key)) row[key] = body[key];
      if (Object.hasOwn(body, "venueCategories"))
        row.venue_categories = clone(body.venueCategories);
      if (Object.hasOwn(body, "itemPool")) row.item_pool = clone(body.itemPool);
      row.revision++;
      return save({
        id: row.id,
        revision: row.revision,
        status: "saved",
        audit_id: "world-audit",
      });
    }
    const placeMatch = end.match(
      /^world\/places(?:\/([^/]+))?(?:\/(defaults)(?:\/(history))?)?$/,
    );
    if (placeMatch) {
      const [, pid, defaultsPart, historyPart] = placeMatch,
        p = pid ? s.places.find((p) => p.id === pid) : null;
      if (pid && !p) return missing();
      if (defaultsPart) {
        const wid = body.worldId || url.searchParams.get("worldId");
        if (wid !== p.world_id) return missing();
        const baseline = s.defaults.get(pid);
        if (historyPart)
          return reply({ items: s.histories.get(pid) || [], next_cursor: "" });
        if (method === "GET") {
          const source = url.searchParams.get("source") || "recorded";
          if (source === "current_definition")
            return reply(
              {
                code: "place_definition_source_missing",
                message: "这个地点没有已验证的定义来源",
              },
              422,
            );
          const target =
            source === "current_saved"
              ? {
                  revision: baseline?.revision || 0,
                  definition: placeFields(p),
                  source_kind: "owner_pinned_saved",
                  created_at: now(),
                }
              : baseline;
          return reply({
            world_id: wid,
            place_id: pid,
            place_revision: p.revision,
            default_revision: baseline?.revision || 0,
            baseline: target || null,
            source,
            current: placeFields(p),
            diff: target
              ? Object.keys(target.definition)
                  .filter(
                    (key) =>
                      key !== "name" &&
                      JSON.stringify(placeFields(p)[key]) !==
                        JSON.stringify(target.definition[key]),
                  )
                  .map((field) => ({
                    field,
                    from: placeFields(p)[field],
                    to: target.definition[field],
                  }))
              : [],
            semantics: "实际地点字段恢复",
          });
        }
        if (
          body.expectedRevision !== p.revision ||
          body.expectedDefaultRevision !== (baseline?.revision || 0)
        )
          return conflict();
        const before = placeFields(p);
        if (body.operation === "capture") {
          s.defaults.set(pid, {
            revision: (baseline?.revision || 0) + 1,
            definition: before,
            source_kind: "owner_pinned_saved",
            created_at: now(),
          });
        } else {
          if (!baseline)
            return reply(
              { code: "place_default_not_configured", message: "缺少恢复基准" },
              422,
            );
          for (const [key, value] of Object.entries(baseline.definition))
            if (key !== "name")
              p[key === "description" ? "desc" : key] = clone(value);
          p.revision++;
        }
        const history = {
          id: "history-" + ++sequence,
          world_id: wid,
          place_id: pid,
          fluctlight_id: fid,
          operation:
            body.operation === "capture" ? "set_baseline" : "restore_fields",
          before_value: before,
          after_value: placeFields(p),
          place_revision: p.revision,
          default_revision: s.defaults.get(pid).revision,
          reason: body.reason,
          created_at: now(),
        };
        s.histories.set(pid, [history, ...(s.histories.get(pid) || [])]);
        return save({
          id: pid,
          place_revision: p.revision,
          default_revision: s.defaults.get(pid).revision,
          status: body.operation === "capture" ? "baseline_saved" : "restored",
          life_write: false,
        });
      }
      if (method === "GET")
        return p
          ? reply(p)
          : reply({
              items: s.places.filter((p) => p.world_id === world?.id),
              next_cursor: "",
            });
      const wid = body.worldId || world?.id;
      if (!s.worlds.some((row) => row.id === wid)) return missing();
      if (p && (p.world_id !== wid || body.expectedRevision !== p.revision))
        return conflict();
      if (p && p.name !== body.name)
        return reply(
          { code: "place_name_immutable", message: "正式名称固定，请修改别名" },
          422,
        );
      if (
        p?.venue_category_id &&
        body.venueCategoryId &&
        body.venueCategoryId !== p.venue_category_id
      )
        return reply(
          { code: "place_category_immutable", message: "已有分类固定" },
          422,
        );
      const before = p ? placeFields(p) : {};
      const point = p || {
        id: fid + "-place-" + ++sequence,
        world_id: wid,
        name: body.name,
        revision: 0,
        status: "active",
        parent_place_id: null,
        connection_place_ids: [],
        source_resource_id: null,
        source_version: null,
      };
      point.desc = body.description || "";
      for (const [input, key] of Object.entries({
        alias: "alias",
        group: "group",
        floor: "floor",
        scene: "scene",
        atmosphere: "atmosphere",
        layout: "layout",
        furniture: "furniture",
        connections: "connections",
        roomItems: "room_items",
        parentPlaceId: "parent_place_id",
        connectionPlaceIds: "connection_place_ids",
        venueCategoryId: "venue_category_id",
        status: "status",
      }))
        if (Object.hasOwn(body, input)) point[key] = clone(body[input]);
      point.revision++;
      if (!p) {
        s.places.push(point);
        s.defaults.set(point.id, {
          revision: 1,
          definition: placeFields(point),
          source_kind: "initial_saved",
          created_at: now(),
        });
      }
      const h = {
        id: "history-" + ++sequence,
        world_id: wid,
        place_id: point.id,
        fluctlight_id: fid,
        operation: p ? "edit_fields" : "initial_saved",
        before_value: before,
        after_value: placeFields(point),
        place_revision: point.revision,
        default_revision: s.defaults.get(point.id)?.revision || 0,
        reason: body.reason,
        created_at: now(),
      };
      s.histories.set(point.id, [h, ...(s.histories.get(point.id) || [])]);
      return save({
        id: point.id,
        revision: point.revision,
        status: "saved",
        audit_id: "place-audit",
      });
    }
    const itemId = end.match(/^world\/items(?:\/([^/]+))?$/);
    if (itemId) {
      const id = itemId[1];
      if (!id) {
        const q = (url.searchParams.get("q") || "").toLocaleLowerCase();
        return reply({
          items: s.items.filter(
            (row) =>
              row.world_id === world?.id &&
              (row.name + row.definition.description)
                .toLocaleLowerCase()
                .includes(q),
          ),
          world_id: world?.id || "",
          next_cursor: "",
        });
      }
      const row = s.items.find((row) => row.id === id);
      if (!row) return missing();
      if (method === "GET") return reply(row);
      if (row.revision !== body.expectedRevision) return conflict();
      if (
        body.placeId &&
        !s.places.some(
          (p) =>
            p.id === body.placeId &&
            p.world_id === row.world_id &&
            p.status === "active",
        )
      )
        return reply({ code: "invalid_request", message: "取得地点无效" }, 400);
      row.available = body.available;
      row.place_id = body.placeId;
      row.revision++;
      return save(row);
    }
    const templateMatch = end.match(
      /^world\/templates(?:\/([^/]+))?(?:\/(runs)(?:\/([^/]+)(?:\/(retry))?)?)?$/,
    );
    if (templateMatch) {
      const [, tid, runsPart, rid, retry] = templateMatch;
      if (!tid)
        return reply({
          items: s.templates.filter((t) => t.world_id === world?.id),
          world_id: world?.id || "",
          catalog_status: "runtime",
          runtime_effect: "worker_executed",
        });
      const t = s.templates.find((t) => t.id === tid);
      if (!t) return missing();
      if (method === "GET") {
        if (!runsPart) return reply(t);
        if (rid) {
          const row = s.runs.find((r) => r.id === rid && r.template_id === tid);
          return row ? reply(observedRun(s, row)) : missing();
        }
        return reply({
          items: s.runs
            .filter((r) => r.template_id === tid)
            .map((row) => clone(observedRun(s, row))),
          next_cursor: "",
        });
      }
      if (t.world_id !== world?.id)
        return reply(
          { code: "invalid_request", message: "请先切换到模板所属世界" },
          400,
        );
      if (body.expectedRevision !== t.revision) return conflict();
      if (!runsPart) {
        t.enabled = body.enabled;
        t.revision++;
        return save({
          id: tid,
          template_id: tid,
          enabled: t.enabled,
          revision: t.revision,
          audit_id: "toggle-audit",
        });
      }
      if (!t.enabled)
        return reply({ code: "invalid_request", message: "模板未启用" }, 400);
      if (retry) {
        const row = s.runs.find((r) => r.id === rid && r.template_id === tid);
        if (!row) return missing();
        if (row.revision !== body.expectedRunRevision) return conflict();
        if (
          row.status !== "failed" ||
          row.template_revision !== t.revision ||
          Date.parse(row.expires_at) <= Date.now()
        )
          return reply(
            { code: "invalid_request", message: "原执行不允许恢复" },
            400,
          );
        row.status = "queued";
        row.revision++;
        return save(row);
      }
      const row = {
        id: "world-run-" + ++sequence,
        world_id: t.world_id,
        template_id: tid,
        fluctlight_id: fid,
        owner_actor_id: owner,
        template_revision: t.revision,
        definition: clone(t.definition),
        trigger_kind: "manual",
        status: "queued",
        source_key: "fixture-run",
        due_at: now(),
        expires_at: new Date(Date.now() + 1800000).toISOString(),
        started_at: null,
        end_at: null,
        attempt_count: 0,
        revision: 1,
        life_event_id: null,
        result: {},
        error_code: null,
        recovery_status: "not_required",
        recovery_result: {},
        created_at: now(),
        updated_at: now(),
      };
      s.runs.unshift(row);
      return save(row);
    }
    const eventId = end.match(/^world\/events(?:\/([^/]+))?$/);
    if (eventId) {
      if (eventId[1]) {
        const row = s.events.find((e) => e.id === eventId[1]);
        return row ? reply(row) : missing();
      }
      return reply({
        items: s.events.filter((e) => !e.world_id || e.world_id === world?.id),
        world_id: world?.id || "",
        next_cursor: "",
        includes_unattributed_history: true,
      });
    }
    return false;
  }
  return { handler, state, calls };
}
