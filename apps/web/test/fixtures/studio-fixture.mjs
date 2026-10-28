// Isolated UI evidence only. Never loaded by the product bundle.
const clone = (v) => JSON.parse(JSON.stringify(v));
export function studioFixture({
  owner = "studio-owner",
  fid = "A",
  scenario = "normal",
  autoJobs = false,
  externalWorld = false,
} = {}) {
  const date = "2026-10-27T08:00:00Z",
    resources = new Map(),
    publications = new Map(),
    jobs = new Map(),
    materials = new Map(),
    receipts = new Map(),
    calls = [];
  let serial = 0,
    firstRead = true,
    firstUnknown = true;
  const definitions = {
    world: [
      "晨雾小镇",
      {
        schema: "world-definition.v1",
        background: "沿着河流生活的小镇，书店与花园在清晨醒来。",
        rules: "普通日常，尊重各自的生活。",
        framework: {
          schema: "world-framework.v1",
          venues: [
            {
              key: "home",
              name: "住处",
              places: [
                {
                  key: "study",
                  name: "书房",
                  description: "靠窗的书桌",
                  layout: "书架与窗台",
                },
              ],
            },
          ],
        },
        custom_preserved: { value: false },
      },
    ],
    character_template: [
      "旅行中的编辑",
      {
        schema: "character-template.v1",
        description: "一位喜欢记录沿途故事的编辑。",
      },
    ],
    place: [
      "河畔书店",
      {
        schema: "place-definition.v1",
        description: "窗外能看到缓慢的河水。",
        atmosphere: "安静",
        layout: "书架与阅读桌",
      },
    ],
    item: [
      "亚麻衬衫",
      {
        schema: "item-definition.v1",
        item_kind: "wearable",
        category: "long_sleeve_top",
        slot: "top",
        description: "适合初秋的浅色亚麻衬衫。",
      },
    ],
    outfit: [
      "秋日散步",
      {
        schema: "outfit-definition.v1",
        description: "轻松的日常搭配。",
        members: [],
      },
    ],
    npc: [
      "花店店主",
      { role: "花店经营者", background: "喜欢与附近的人交流植物。" },
    ],
    event_template: [
      "周末读书会",
      {
        schema_version: "world-event.v1",
        description: "在书店分享阅读。",
        category: "environment",
        intensity: "low",
        duration_minutes: 30,
        trigger: { mode: "manual", interval_minutes: 60, cooldown_minutes: 120, probability: 1 },
        condition: { day: "daily", start: "00:00", end: "00:00", place_ids: [], require_presence: false },
        effects: [],
      },
    ],
    persona_fragment: [
      "阅读时的习惯",
      {
        schema: "persona-fragment.v1",
        changes: { identity: { interests: ["阅读"] } },
      },
    ],
    visual_style: [
      "柔和水彩",
      {
        schema: "visual-style.v1",
        appearance: "自然的神情",
        clothing: "日常衣着",
        constraints: "保持人物一致",
        references: "水彩纸感",
      },
    ],
  };
  const publish = (r, version = r.draft_version) => {
    publications.set(r.id + ":" + version, {
      id: r.id,
      version,
      kind: r.kind,
      name: r.name,
      body: clone(r.body),
      immutable: true,
      created_at: date,
    });
    r.published_version = version;
    r.status = "published";
  };
  if (scenario !== "empty")
    for (const [kind, [name, body]] of Object.entries(definitions)) {
      const r = {
        id: "studio-" + kind,
        kind,
        name,
        body: clone(body),
        revision: 1,
        draft_version: "draft:1",
        published_version: null,
        status: "draft",
        owner_actor_id: owner,
        created_at: date,
        updated_at: date,
        referenced_by: [],
      };
      if (kind !== "place") publish(r);
      resources.set(r.id, r);
    }
  const page = (items) => ({ items: clone(items), next_cursor: "" });
  function completeJob(job) {
    if (job.status !== "queued" && job.status !== "running") return;
    job.status = "completed";
    job.revision++;
    job.result =
      job.kind === "assistant"
        ? {
            disposition: "proposal",
            name: job.snapshot.name,
            body: {
              ...clone(job.snapshot.body),
              background: "小镇的河岸增添了一座可供阅读的花园。",
            },
            reply: "这是基于本次冻结草稿的建议，采用前可以核对。",
          }
        : job.kind === "text_preview"
          ? { text: "窗边的光线慢慢铺开，故事从这里开始。" }
          : { asset_id: "studio-picture", status: "ready" };
    job.attempts = [
      {
        id: "attempt-" + job.id,
        job_id: job.id,
        attempt: 1,
        status: "completed",
        output: clone(job.result),
        provenance: { isolated_fixture: true },
        created_at: date,
        finished_at: date,
      },
    ];
  }
  async function handler({ path, url, method, body = {}, json, res }) {
    if (
      path === "/api/media/reference-grants" &&
      url.searchParams.get("target_scope") !== "studio_resource"
    )
      return false;
    if (externalWorld && path === `/api/fluctlights/${fid}/world`) return false;
    if (
      !path.startsWith("/api/studio/") &&
      path !== "/api/media/reference-grants" &&
      path !== `/api/fluctlights/${fid}/world`
    )
      return false;
    calls.push({
      path,
      method,
      body: clone(body),
      query: Object.fromEntries(url.searchParams),
    });
    const send = (v, status = 200) => {
      json(res, clone(v), status);
      return true;
    };
    if (method === "GET" && path === "/api/studio/resources" && firstRead) {
      firstRead = false;
      if (scenario === "loading") await new Promise((r) => setTimeout(r, 2200));
      if (scenario === "failure")
        return send(
          {
            code: "fixture_studio_unavailable",
            message: "资源目录暂时无法读取",
          },
          503,
        );
    }
    const originalKey = body.idempotencyKey;
    if (method !== "GET" && originalKey && receipts.has(originalKey))
      return send(receipts.get(originalKey));
    const ack = (v, status = 200) => {
      if (originalKey) receipts.set(originalKey, clone(v));
      if (scenario === "unknown" && firstUnknown && method !== "GET") {
        firstUnknown = false;
        return send({ code: "fixture_response_lost" }, 503);
      }
      return send(v, status);
    };
    const conflict = () =>
      send(
        { code: "revision_conflict", message: "版本已变化，请读取后确认" },
        409,
      );
    if (path === "/api/media/reference-grants") return send(page([]));
    if (path === `/api/fluctlights/${fid}/world`)
      return send({
        world: {
          id: "studio-world",
          fluctlight_id: fid,
          name: "运行小镇",
          revision: 1,
        },
        active_binding: { world_id: "studio-world", revision: 1 },
        places: [{ id: "home-place", name: "书房", status: "active" }],
        npcs: [],
      });
    if (path === "/api/studio/resources" && method === "GET") {
      let rows = [...resources.values()].filter((r) =>
        url.searchParams.get("archived") === "true"
          ? r.status === "archived"
          : r.status !== "archived",
      );
      const kind = url.searchParams.get("kind"),
        q = url.searchParams.get("q");
      if (kind) rows = rows.filter((r) => r.kind === kind);
      if (q) rows = rows.filter((r) => r.name.includes(q));
      if (url.searchParams.get("published") === "true")
        rows = rows.filter((r) => r.published_version);
      return send(page(rows));
    }
    if (path === "/api/studio/resources" && method === "POST") {
      const r = {
        id: "studio-created-" + ++serial,
        owner_actor_id: owner,
        kind: body.kind,
        name: body.name.trim(),
        body: clone(body.body),
        revision: 1,
        draft_version: "draft:1",
        published_version: null,
        status: "draft",
        created_at: date,
        updated_at: date,
        referenced_by: [],
      };
      resources.set(r.id, r);
      return ack(r, 201);
    }
    const resourceMatch = path.match(
      /^\/api\/studio\/resources\/([^/]+)(?:\/(.*))?$/,
    );
    if (resourceMatch) {
      const id = decodeURIComponent(resourceMatch[1]),
        op = resourceMatch[2] || "",
        r = resources.get(id);
      if (!r) return send({ code: "not_found" }, 404);
      if (!op && method === "GET") return send(r);
      if (!op && method === "PUT") {
        if (body.expectedVersion !== r.draft_version) return conflict();
        r.revision++;
        r.draft_version = "draft:" + r.revision;
        r.name = body.name.trim();
        r.body = clone(body.body);
        return ack(r);
      }
      if (op === "versions" && method === "GET")
        return send(
          page(
            [...publications.values()]
              .filter((v) => v.id === id)
              .map((v) => ({ ...v, resource_id: id, published_at: date })),
          ),
        );
      if (op.startsWith("versions/") && method === "GET") {
        const v = publications.get(id + ":" + decodeURIComponent(op.slice(9)));
        return v ? send(v) : send({ code: "not_found" }, 404);
      }
      if (op === "publish" && method === "POST") {
        if (body.expectedRevision !== r.revision) return conflict();
        publish(r);
        r.revision++;
        return ack({
          id,
          revision: r.revision,
          published_version: r.published_version,
          status: r.status,
        });
      }
      if (op === "copy" && method === "POST") {
        if (body.expectedRevision !== r.revision) return conflict();
        const v = body.sourceVersion
          ? publications.get(id + ":" + body.sourceVersion)
          : r;
        const copy = {
          ...clone(r),
          id: "studio-copy-" + ++serial,
          name: body.name,
          body: clone(v?.body || r.body),
          revision: 1,
          draft_version: "draft:1",
          published_version: null,
          status: "draft",
          referenced_by: [],
        };
        resources.set(copy.id, copy);
        return ack(copy);
      }
      if (
        (!op && method === "DELETE") ||
        (op === "restore" && method === "POST")
      ) {
        if (body.expectedRevision !== r.revision) return conflict();
        r.status =
          op === "restore"
            ? r.published_version
              ? "published"
              : "draft"
            : "archived";
        r.revision++;
        return ack(r);
      }
      if (op === "apply-preview" && method === "POST") {
        const v = publications.get(id + ":" + body.resourceVersion);
        if (!v) return send({ code: "not_found" }, 404);
        if (v.kind === "character_template" && !v.body.initialization)
          return send({
            target_kind: "foundation",
            preparation_required: true,
            create_required: false,
            diff: [],
            instance_revision: 0,
            runtime_revision: 0,
            semantics: "先解析已发布文本，再审阅完整定义。",
          });
        const target = {
          world: "world",
          place: "world_place",
          item: "world_item",
          outfit: "wardrobe_outfit",
          npc: "world_npc",
          event_template: "world_event_template",
          visual_style: "visual_brief",
          character_template: "foundation",
          persona_fragment: "foundation",
        }[v.kind];
        return send({
          target_kind: target,
          create_required: true,
          diff: Object.entries(v.body)
            .filter(([k]) => k !== "schema")
            .map(([field, to]) => ({ field, from: null, to })),
          instance_revision: 0,
          runtime_revision: 1,
          world_id: "studio-world",
          world_binding_revision: 1,
          ...(v.kind === "outfit"
            ? { profile_id: "profile-1", wardrobe_revision: 1, members: [] }
            : {}),
        });
      }
      if (op === "apply" && method === "POST") {
        const v = publications.get(id + ":" + body.resourceVersion);
        if (!v) return send({ code: "not_found" }, 404);
        const foundation = ["character_template", "persona_fragment"].includes(
          v.kind,
        );
        const result = {
          id: "studio-application-" + ++serial,
          resource_id: id,
          resource_version: body.resourceVersion,
          instance_revision: 1,
          runtime_revision: 2,
          applied: !foundation,
          status: foundation ? "proposed" : "applied",
          ...(foundation
            ? {
                proposal_id: "studio-foundation-proposal",
                target_kind: "foundation",
                fluctlight_id: body.fluctlightId,
              }
            : {}),
        };
        r.referenced_by.push({
          id: result.id,
          fluctlight_id: body.fluctlightId,
          running_version: body.resourceVersion,
          status: result.status,
        });
        return ack(result);
      }
    }
    if (path === "/api/studio/jobs" && method === "GET")
      return send(
        page(
          [...jobs.values()].filter(
            (j) =>
              !url.searchParams.get("resource_id") ||
              j.resource_id === url.searchParams.get("resource_id"),
          ),
        ),
      );
    if (path === "/api/studio/jobs" && method === "POST") {
      const r = resources.get(body.resourceId);
      if (!r || r.revision !== body.resourceRevision) return conflict();
      const j = {
        id: "studio-job-" + ++serial,
        owner_actor_id: owner,
        resource_id: r.id,
        resource_revision: r.revision,
        kind: body.kind,
        snapshot: clone(body),
        source_digest: "fixture-source-" + serial,
        status: "queued",
        revision: 1,
        latest_attempt: 1,
        result: {},
        created_at: date,
        updated_at: date,
        attempts: [],
        life_write: false,
      };
      jobs.set(j.id, j);
      return ack(j, 201);
    }
    const jobMatch = path.match(/^\/api\/studio\/jobs\/([^/]+)(?:\/(.*))?$/);
    if (jobMatch) {
      const id = decodeURIComponent(jobMatch[1]),
        op = jobMatch[2] || "",
        j = jobs.get(id);
      if (!j) return send({ code: "not_found" }, 404);
      if (!op && method === "GET") {
        if (autoJobs) completeJob(j);
        return send(j);
      }
      if (method === "POST") {
        if (body.expectedRevision !== j.revision) return conflict();
        if (op === "retry") {
          j.status = "queued";
          j.revision++;
          return ack(j);
        }
        if (op === "discard") {
          j.status = "discarded";
          j.revision++;
          return ack(j);
        }
        if (op === "save-material") {
          const m = {
            id: "studio-material-" + ++serial,
            job_id: id,
            resource_id: j.resource_id,
            owner_actor_id: owner,
            source_digest: j.source_digest,
            label: "创作片段",
            text: j.result.text || null,
            asset_id: j.result.asset_id || null,
            revision: 1,
            released_at: null,
            created_at: date,
          };
          materials.set(m.id, m);
          return ack({
            id: m.id,
            job_id: id,
            resource_id: j.resource_id,
            status: "saved",
            life_write: false,
          });
        }
      }
    }
    if (path === "/api/studio/materials" && method === "GET")
      return send(
        page(
          [...materials.values()].filter(
            (m) =>
              (!url.searchParams.get("resource_id") ||
                m.resource_id === url.searchParams.get("resource_id")) &&
              (url.searchParams.get("released") === "true"
                ? !!m.released_at
                : !m.released_at),
          ),
        ),
      );
    const materialMatch = path.match(
      /^\/api\/studio\/materials\/([^/]+)(?:\/(.*))?$/,
    );
    if (materialMatch) {
      const m = materials.get(decodeURIComponent(materialMatch[1])),
        op = materialMatch[2] || "";
      if (!m) return send({ code: "not_found" }, 404);
      if (!op && method === "GET") return send(m);
      if (op === "history")
        return send(
          page([
            {
              operation: m.released_at ? "release" : "retain",
              revision: m.revision,
              created_at: date,
              reason: "隔离样本",
            },
          ]),
        );
      if (["retain", "release"].includes(op) && method === "POST") {
        if (body.expectedRevision !== m.revision) return conflict();
        m.revision++;
        m.released_at = op === "release" ? date : null;
        return ack(m);
      }
    }
    return false;
  }
  return {
    handler,
    resources,
    publications,
    jobs,
    materials,
    calls,
    completeJob,
  };
}
