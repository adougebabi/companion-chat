// Task-owned isolated media/visual transport; never imported by production.
export function mediaFixture({
  owner = "media-owner",
  scenario = "normal",
  defaultFid = "A",
  onAvatar,
  externalWorld = false,
  externalInventory = false,
} = {}) {
  const clone = (v) => JSON.parse(JSON.stringify(v)),
    calls = [],
    stores = new Map(),
    assets = new Map(),
    receipts = new Map(),
    grants = new Map(),
    deletions = new Map(),
    reads = new Map(),
    lost = new Set();
  let seq = 10;
  const date = "2026-10-28T03:00:00.000Z";
  function asset(
    id,
    title,
    origin = "owner_upload",
    fid = null,
    kind = "image",
  ) {
    const a = {
      id,
      title,
      source_filename:
        title +
        "." +
        (kind === "image" ? "png" : kind === "audio" ? "mp3" : "mp4"),
      kind,
      origin,
      fluctlight_id: fid,
      byte_size: 10240,
      sha256: "a".repeat(64),
      version: "version-" + id,
      asset_revision: 3,
      metadata_revision: 1,
      hidden: false,
      status: "ready",
      readable: true,
      read_state: "readable",
      url: "/api/media/" + id,
      created_at: date,
      factual: false,
      label: "隔离样本·非事实",
      source_state: origin === "owner_upload" ? "owner_original" : "recorded",
      source_snapshot: {
        origin,
        description: "隔离API样本，不是实际生成或生活事实。",
      },
    };
    assets.set(id, a);
    return a;
  }
  if (scenario !== "empty") {
    asset("replica-landscape", "午后的窗边");
    asset("replica-unused", "雨后的街角");
    asset("replica-forest", "林间的微光", "studio");
    const audio = asset(
      "replica-audio",
      "尚未就绪的声音",
      "owner_upload",
      null,
      "audio",
    );
    Object.assign(audio, {
      status: "unavailable",
      readable: false,
      read_state: "storage_not_configured",
      url: null,
    });
  }
  function state(fid = defaultFid) {
    if (!stores.has(fid)) {
      const empty = scenario === "empty",
        canonical = empty ? null : fid + "-replica-canonical",
        candidate = empty ? null : fid + "-replica-candidate";
      if (canonical) asset(canonical, "已接受的规范参考", "fluctlight", fid);
      if (candidate) asset(candidate, "当前视觉候选", "fluctlight", fid);
      const brief = {
        fluctlight_id: fid,
        revision: empty ? 0 : 2,
        brief: empty
          ? null
          : {
              appearance: "深色短发，自然清晰的轮廓。",
              clothing: "低饱和日常衣物，以实际穿着为准。",
              constraints: "保持面部特征和身体比例。",
              references: "柔和自然光。",
              source_resource_id: null,
              unknown_extension: { preserve: true },
            },
      };
      const visual = {
        schema_version: "visual-identity.v1",
        fluctlight_id: fid,
        status: empty ? "missing" : "ready",
        current_revision: empty ? 0 : 1,
        identity_snapshot: { name: "林青", frozen: true },
        renderer_constraints: { keep_unknown: { enabled: true } },
        canonical_asset_id: canonical,
        character_sheet_asset_id: null,
        session: empty
          ? null
          : {
              id: fid + "-visual-session",
              status: "awaiting_review",
              last_error: null,
              current_attempt: 2,
              created_at: date,
              task_id: "task-" + fid + "-visual",
              attempts: [
                {
                  id: fid + "-attempt-one",
                  attempt_number: 1,
                  status: "accepted",
                  candidate_asset_id: canonical,
                  decision: "accepted",
                  brief_revision: 1,
                  created_at: date,
                  review: { source: "owner_manual", summary: "原规范已接受。" },
                },
                {
                  id: fid + "-attempt-two",
                  attempt_number: 2,
                  status: "awaiting_review",
                  candidate_asset_id: candidate,
                  decision: null,
                  brief_revision: 2,
                  created_at: date,
                  review: null,
                },
              ],
            },
      };
      stores.set(fid, {
        brief,
        briefHistory: empty
          ? []
          : [
              {
                id: "brief-history-" + fid,
                revision: 2,
                reason: "完善初始参考",
                before_value: {},
                after_value: clone(brief.brief),
                created_at: date,
              },
            ],
        visual,
        timeline: empty
          ? []
          : [
              {
                id: "visual-record-" + fid,
                stage: "accepted",
                status: "completed",
                summary: "已接受初始规范参考",
                asset_ids: [canonical],
                occurred_at: date,
                review: { source: "owner_manual", summary: "保留实际观察。" },
              },
            ],
        avatar: {
          fluctlight_id: fid,
          revision: 1,
          mode: "canonical",
          selection: null,
          avatar: canonical
            ? { asset_id: canonical, url: "/api/media/" + canonical }
            : null,
          read_state: canonical ? "readable" : "not_configured",
        },
        avatarHistory: [],
        requests: new Map(),
        visualReads: 0,
      });
    }
    return stores.get(fid);
  }
  state(defaultFid);
  const addGrant = (assetId, targetId = defaultFid) => {
    const a = assets.get(assetId),
      g = {
        id: "grant-" + ++seq,
        owner_actor_id: owner,
        asset_id: assetId,
        asset_version: a.version,
        asset_revision: a.asset_revision,
        sha256: a.sha256,
        byte_size: a.byte_size,
        mime_type: "image/png",
        target_scope: "fluctlight",
        target_id: targetId,
        purpose: "scene",
        provenance: "owner_explicit_unverified_content",
        status: "active",
        revision: 1,
        created_at: date,
      };
    grants.set(g.id, g);
    return g;
  };
  if (scenario !== "empty") addGrant("replica-landscape");
  async function handler({ path, method, body = {}, url, json, res }) {
    const scoped = path.match(
        /^\/api\/fluctlights\/([^/]+)\/(visual-brief|visual-identity|visual-sessions|display-avatar|media-generations)(?:\/(.*))?$/,
      ),
      fid = scoped ? decodeURIComponent(scoped[1]) : null,
      s = scoped ? state(fid) : null;
    if (!externalWorld && /^\/api\/fluctlights\/[^/]+\/world$/.test(path)) {
      const f = decodeURIComponent(path.split("/")[3]);
      json(res, {
        world: { id: f + "-world", fluctlight_id: f },
        places: [
          {
            id: f + "-window",
            world_id: f + "-world",
            name: "窗边",
            status: "active",
            revision: 4,
          },
        ],
      });
      return true;
    }
    if (
      !externalInventory &&
      /^\/api\/fluctlights\/[^/]+\/wardrobe$/.test(path)
    ) {
      json(res, { worn_items: [], used_items: [], revision: 1 });
      return true;
    }
    if (
      !externalInventory &&
      /^\/api\/fluctlights\/[^/]+\/inventory(?:\/items\/[^/]+)?$/.test(path)
    ) {
      const f = decodeURIComponent(path.split("/")[3]),
        item = {
          id: f + "-coat",
          description: "登记的针织开衫",
          fluctlight_id: f,
          revision: 5,
          status: "active",
          slot: "outerwear",
          category: "outerwear",
        };
      json(
        res,
        path.split("/").length > 6 ? item : { items: [item], next_cursor: "" },
      );
      return true;
    }
    if (path === "/api/studio/resources" && method === "GET") {
      json(res, {
        items: [
          {
            id: "media-style-resource",
            name: "窗边的视觉风格",
            kind: "visual_style",
            revision: 1,
            status: "active",
          },
        ],
        next_cursor: "",
      });
      return true;
    }
    if (
      !scoped &&
      !/^\/api\/(media-library|media-assets\/|media-reference-grants|media-deletions\/|media-uploads\/)/.test(
        path,
      )
    )
      return false;
    calls.push({
      method,
      path,
      query: Object.fromEntries(url.searchParams),
      body: clone(body),
    });
    const send = (value, status = 200) => {
        json(res, clone(value), status);
        return true;
      },
      error = (code, status = 409) => send({ code, message: code }, status),
      page = (rows, cursor, key) => {
        const offset = cursor ? Number(cursor.split(":").at(-1)) : 0;
        return {
          items: rows.slice(offset, offset + 3),
          next_cursor: offset + 3 < rows.length ? key + ":" + (offset + 3) : "",
        };
      };
    const replay = () => {
      const p = receipts.get(body.idempotencyKey);
      return !p
        ? null
        : p.path !== path || p.body !== JSON.stringify(body)
          ? error("idempotency_conflict")
          : send(p.result);
    };
    const accept = (value, type) => {
      receipts.set(body.idempotencyKey, {
        path,
        body: JSON.stringify(body),
        result: clone(value),
      });
      if (scenario === "unknown" && !lost.has(type)) {
        lost.add(type);
        return error("request_failed", 503);
      }
      return send(value);
    };
    if (method === "GET") {
      const key = scoped
        ? scoped[2]
        : path === "/api/media-library"
          ? "library"
          : "";
      if (key) {
        const count = (reads.get(key) || 0) + 1;
        reads.set(key, count);
        if (scenario === "loading" && count === 1)
          await new Promise((r) => setTimeout(r, 4500));
        if (scenario === "failure" && count === 1)
          return error("request_failed", 503);
      }
      if (
        scenario === "readback" &&
        receipts.size &&
        !lost.has("readback") &&
        path.startsWith("/api/media-assets/")
      ) {
        lost.add("readback");
        return error("request_failed", 503);
      }
    }
    if (path === "/api/media-library") {
      const f = url.searchParams;
      let rows = [...assets.values()].filter(
        (a) =>
          a.status !== "deleted" &&
          a.hidden === (f.get("hidden") === "true") &&
          (!f.get("kind") || a.kind === f.get("kind")) &&
          (!f.get("fluctlightId") ||
            a.fluctlight_id === f.get("fluctlightId")) &&
          (!f.get("scope") ||
            f.get("scope") === "all" ||
            a.origin === f.get("scope")) &&
          (a.title + " " + a.source_filename)
            .toLowerCase()
            .includes((f.get("q") || "").toLowerCase()),
      );
      return send(page(rows, f.get("cursor"), "media"));
    }
    const assetMatch = path.match(
      /^\/api\/media-assets\/([^/]+)(?:\/(references))?$/,
    );
    if (assetMatch) {
      const id = decodeURIComponent(assetMatch[1]),
        a = assets.get(id);
      if (!a) return error("not_found", 404);
      if (assetMatch[2]) {
        const refs = [];
        for (const [f, st] of stores) {
          if (st.visual.canonical_asset_id === id)
            refs.push({
              id: "reference-visual-" + f,
              fluctlight_id: f,
              target_type: "visual_identity",
              target_id: f,
            });
          if (
            st.visual.session?.attempts.some((r) => r.candidate_asset_id === id)
          )
            refs.push({
              id: "reference-attempt-" + f,
              fluctlight_id: f,
              target_type: "visual_attempt",
              target_id: f,
            });
          if (st.avatar.selection?.asset_id === id)
            refs.push({
              id: "reference-avatar-" + f,
              fluctlight_id: f,
              target_type: "display_avatar",
              target_id: f,
            });
        }
        for (const g of grants.values())
          if (g.asset_id === id && g.status === "active")
            refs.push({
              id: "reference-" + g.id,
              fluctlight_id: g.target_scope === "fluctlight" ? g.target_id : "",
              target_type: "model_reference_grant",
              target_id: g.id,
            });
        return send({
          ...page(refs, url.searchParams.get("cursor"), "refs"),
          reference_count: refs.length,
        });
      }
      if (method === "DELETE") {
        const hit = replay();
        if (hit) return hit;
        if (body.expectedAssetRevision !== a.asset_revision)
          return error("revision_conflict");
        const protectedRef =
          [...stores.values()].some(
            (st) =>
              st.visual.canonical_asset_id === id ||
              st.visual.session?.attempts.some(
                (v) => v.candidate_asset_id === id,
              ) ||
              st.avatar.selection?.asset_id === id,
          ) ||
          [...grants.values()].some(
            (g) => g.asset_id === id && g.status === "active",
          );
        if (protectedRef) return error("media_asset_referenced");
        const d = {
          id: "deletion-" + ++seq,
          asset_id: id,
          status: scenario === "delete-failure" ? "failed" : "pending",
          revision: 1,
          attempt: 1,
          workflow_id: "delete-workflow-" + seq,
          task_id: "delete-task-" + seq,
          error_code:
            scenario === "delete-failure" ? "storage_unavailable" : null,
          execution_status:
            scenario === "delete-failure" ? "failed" : "pending",
          execution_error_code: null,
          created_at: date,
          updated_at: date,
          deleted_at: null,
          reads: 0,
        };
        deletions.set(d.id, d);
        Object.assign(a, {
          status: "tombstoned",
          read_state: "tombstoned",
          readable: false,
          asset_revision: a.asset_revision + 1,
          deletion_id: d.id,
          url: null,
        });
        return accept(d, "delete");
      }
      if (method === "GET") return send(a);
      const hit = replay();
      if (hit) return hit;
      if (
        body.expectedAssetRevision !== a.asset_revision ||
        body.expectedMetadataRevision !== a.metadata_revision
      )
        return error("revision_conflict");
      if (scenario === "conflict" && !lost.has("metadata-conflict")) {
        lost.add("metadata-conflict");
        a.metadata_revision++;
        return error("revision_conflict");
      }
      Object.assign(a, {
        title: body.title,
        hidden: body.hidden,
        metadata_revision: a.metadata_revision + 1,
      });
      return accept(a, "metadata");
    }
    if (path === "/api/media-reference-grants") {
      if (method === "GET")
        return send(
          page(
            [...grants.values()].filter(
              (g) =>
                (!url.searchParams.get("asset_id") ||
                  g.asset_id === url.searchParams.get("asset_id")) &&
                (!url.searchParams.get("target_id") ||
                  g.target_id === url.searchParams.get("target_id")) &&
                (!url.searchParams.get("target_scope") ||
                  g.target_scope === url.searchParams.get("target_scope")) &&
                g.status === (url.searchParams.get("status") || "active"),
            ),
            url.searchParams.get("cursor"),
            "grants",
          ),
        );
      const hit = replay();
      if (hit) return hit;
      const a = assets.get(body.assetId);
      if (
        !a ||
        !a.readable ||
        a.kind !== "image" ||
        a.asset_revision !== body.expectedAssetRevision
      )
        return error("media_reference_source_changed");
      if (!body.confirm) throw Error("Missing explicit reference confirmation");
      const g = {
        ...addGrant(a.id, body.targetId),
        target_scope: body.targetScope,
        purpose: body.purpose,
      };
      grants.set(g.id, g);
      return accept(g, "grant");
    }
    const gm = path.match(
      /^\/api\/media-reference-grants\/([^/]+)(?:\/(.*))?$/,
    );
    if (gm) {
      const g = grants.get(decodeURIComponent(gm[1]));
      if (!g) return error("not_found", 404);
      if (method === "GET")
        return send(
          gm[2]
            ? {
                items:
                  gm[2] === "uses"
                    ? []
                    : [
                        {
                          id: "grant-history",
                          revision: g.revision,
                          after_value: g,
                          reason: "隔离样本授权历史",
                        },
                      ],
                next_cursor: "",
              }
            : g,
        );
      const hit = replay();
      if (hit) return hit;
      if (body.expectedRevision !== g.revision)
        return error("revision_conflict");
      g.status = "revoked";
      g.revision++;
      return accept(g, "revoke");
    }
    const dm = path.match(/^\/api\/media-deletions\/([^/]+)(?:\/retry)?$/);
    if (dm) {
      const d = deletions.get(dm[1]);
      if (!d) return error("not_found", 404);
      if (method === "GET") {
        d.reads++;
        if (d.status !== "failed" && d.reads >= 3) {
          d.status = "deleted";
          const a = assets.get(d.asset_id);
          a.status = "deleted";
          a.readable = false;
          a.read_state = "deleted";
        }
        return send(d);
      }
      const hit = replay();
      if (hit) return hit;
      if (body.expectedRevision !== d.revision)
        return error("revision_conflict");
      Object.assign(d, {
        status: "pending",
        revision: d.revision + 1,
        attempt: d.attempt + 1,
        reads: 0,
        execution_status: "pending",
      });
      return accept(d, "delete-retry");
    }
    if (!scoped) return false;
    const kind = scoped[2],
      tail = scoped[3] || "";
    if (kind === "visual-brief") {
      if (method === "GET")
        return send(
          tail === "history"
            ? page(
                s.briefHistory,
                url.searchParams.get("cursor"),
                "brief-history",
              )
            : s.brief,
        );
      const hit = replay();
      if (hit) return hit;
      if (body.expectedRevision !== s.brief.revision)
        return error("visual_brief_conflict");
      const before = clone(s.brief.brief);
      s.brief.brief = {
        ...s.brief.brief,
        ...Object.fromEntries(
          ["appearance", "clothing", "constraints", "references"].map((k) => [
            k,
            body[k],
          ]),
        ),
      };
      s.brief.revision++;
      s.briefHistory.unshift({
        id: "brief-history-" + ++seq,
        revision: s.brief.revision,
        before_value: before,
        after_value: clone(s.brief.brief),
        reason: body.reason,
        created_at: date,
      });
      return accept(s.brief, "brief");
    }
    if (kind === "visual-identity") {
      if (method === "GET") {
        if (tail === "timeline")
          return send(
            page(s.timeline, url.searchParams.get("cursor"), "timeline"),
          );
        if (s.visual.session?.status === "queued" && ++s.visualReads >= 2) {
          const vs = s.visual.session;
          const a = asset(
            fid + "-replica-candidate-" + vs.current_attempt,
            "新视觉候选",
            "fluctlight",
            fid,
          );
          vs.attempts.push({
            id: "attempt-" + ++seq,
            attempt_number: vs.current_attempt,
            status: "awaiting_review",
            candidate_asset_id: a.id,
            decision: null,
            brief_revision: s.brief.revision,
            created_at: date,
          });
          vs.status = "awaiting_review";
        }
        if (
          s.visual.session?.status === "character_sheet_pending" &&
          ++s.visualReads >= 3
        ) {
          const a = asset(
            fid + "-replica-sheet",
            "角色设定图",
            "fluctlight",
            fid,
          );
          s.visual.character_sheet_asset_id = a.id;
          s.visual.session.status = "completed";
        }
        return send(s.visual);
      }
      const hit = replay();
      if (hit) return hit;
      if (
        body.expectedVisualRevision !== s.visual.current_revision ||
        body.expectedBriefRevision !== s.brief.revision
      )
        return error("visual_generation_conflict");
      const occupied =
          !!s.visual.session &&
          [
            "awaiting_review",
            "queued",
            "running",
            "character_sheet_pending",
          ].includes(s.visual.session.status),
        id = "visual-generation-" + ++seq;
      if (!occupied) {
        s.visual.session = {
          id: "session-" + seq,
          status: "queued",
          current_attempt: 1,
          attempts: [],
          created_at: date,
          task_id: "task-session-" + seq,
        };
      }
      const r = {
        fluctlight_id: fid,
        request_id: id,
        request: {
          id,
          status: occupied ? "pending" : "queued",
          ...(!occupied ? { session_id: s.visual.session.id } : {}),
        },
        status: occupied ? "pending" : "queued",
        brief_revision: s.brief.revision,
        audit_id: "visual-audit",
        ...(!occupied
          ? {
              session_id: s.visual.session.id,
              task_id: s.visual.session.task_id,
            }
          : {}),
      };
      return accept(r, "visual-generate");
    }
    if (kind === "visual-sessions") {
      const hit = replay();
      if (hit) return hit;
      const sid = tail.split("/")[0],
        v = s.visual,
        session = v.session,
        attempt = session?.attempts.find(
          (a) => a.attempt_number === session.current_attempt,
        );
      if (
        session?.id !== sid ||
        session.current_attempt !== body.expectedAttempt ||
        v.current_revision !== body.expectedVisualRevision ||
        attempt?.decision
      )
        return error("visual_review_conflict");
      attempt.decision = body.decision;
      attempt.review = { source: "owner_manual", ...clone(body) };
      const r = {
        fluctlight_id: fid,
        session_id: sid,
        operation_id: "review-" + ++seq,
        replayed: false,
        result: {
          session_id: sid,
          attempt: body.expectedAttempt,
          decision: body.decision,
          candidate_asset_id: attempt.candidate_asset_id,
        },
      };
      if (body.decision === "accepted") {
        v.canonical_asset_id = attempt.candidate_asset_id;
        v.current_revision++;
        session.status = "character_sheet_pending";
        Object.assign(r.result, {
          status: "character_sheet_pending",
          canonical_asset_id: attempt.candidate_asset_id,
          character_sheet_media_intent_id: "sheet-intent-" + seq,
        });
      } else {
        session.current_attempt++;
        session.status = "queued";
        s.visualReads = 0;
        r.result.status = "regenerating";
        r.result.next_attempt = session.current_attempt;
      }
      s.timeline.unshift({
        id: r.operation_id,
        stage: body.decision,
        status: session.status,
        summary: body.summary,
        asset_ids: [attempt.candidate_asset_id],
        review: attempt.review,
        occurred_at: date,
      });
      return accept(r, "review");
    }
    if (kind === "display-avatar") {
      if (method === "GET") {
        if (tail === "history")
          return send(
            page(
              s.avatarHistory,
              url.searchParams.get("cursor"),
              "avatar-history",
            ),
          );
        if (s.avatar.mode === "canonical") {
          const image = assets.get(s.visual.canonical_asset_id);
          s.avatar.avatar = image?.readable
            ? { asset_id: image.id, url: "/api/media/" + image.id }
            : null;
          s.avatar.read_state = image?.readable ? "readable" : "not_configured";
        }
        return send(s.avatar);
      }
      const hit = replay();
      if (hit) return hit;
      if (body.expectedRevision !== s.avatar.revision)
        return error("revision_conflict");
      const a =
        body.mode === "asset"
          ? assets.get(body.assetId)
          : assets.get(s.visual.canonical_asset_id);
      if (
        body.mode === "asset" &&
        (!a?.readable || a.asset_revision !== body.expectedAssetRevision)
      )
        return error("revision_conflict");
      const before = clone(s.avatar);
      s.avatar = {
        fluctlight_id: fid,
        mode: body.mode,
        revision: before.revision + 1,
        selection:
          body.mode === "asset"
            ? {
                asset_id: a.id,
                asset_revision: a.asset_revision,
                version: a.version,
                sha256: a.sha256,
              }
            : null,
        avatar: a?.readable
          ? { asset_id: a.id, url: "/api/media/" + a.id }
          : null,
        read_state: a?.readable ? "readable" : "not_configured",
      };
      s.avatarHistory.unshift({
        id: "avatar-history-" + ++seq,
        revision: s.avatar.revision,
        reason: body.reason,
        before_value: before,
        after_value: clone(s.avatar),
        created_at: date,
      });
      onAvatar?.(fid, clone(s.avatar));
      return accept(s.avatar, "avatar");
    }
    if (kind === "media-generations") {
      if (method === "GET") {
        if (!tail)
          return send(
            page(
              [...s.requests.values()].map((r) => r.request),
              url.searchParams.get("cursor"),
              "image-requests",
            ),
          );
        const r = s.requests.get(tail);
        if (!r) return error("not_found", 404);
        r.reads++;
        if (r.reads >= 3 && scenario !== "generation-failure") {
          r.request.status = "completed";
          r.media = {
            id: "intent-" + tail,
            status: "completed",
            workflow_id: "workflow-" + tail,
            source: {
              item: r.request.payload.itemId || null,
              place: r.request.payload.placeId || null,
            },
          };
          r.asset = asset(
            "replica-generated-" + tail,
            r.request.payload.prompt.slice(0, 30),
            "fluctlight",
            fid,
          );
        } else if (scenario === "generation-failure") {
          r.request.status = "failed";
          r.request.decision_reason = "图片服务暂时不可用。";
          r.media = {
            id: "intent-" + tail,
            status: "failed",
            error_code: "provider_unavailable",
          };
        }
        return send(r);
      }
      const hit = replay();
      if (hit) return hit;
      const id = "media-request-" + ++seq,
        request = {
          id,
          fluctlight_id: fid,
          kind: "media_generation",
          status: "pending",
          revision: 1,
          payload: clone(body),
          result: {},
          tasks: [],
          created_at: date,
        };
      s.requests.set(id, {
        fluctlight_id: fid,
        request,
        media: null,
        asset: null,
        factual: false,
        label: "生成图·非事实",
        reads: 0,
      });
      return accept({ request, audit_id: "media-audit" }, "generation");
    }
    return false;
  }
  return { handler, calls, state, assets, grants, deletions, receipts };
}
