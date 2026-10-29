// Fictional, loopback-only growth records for production UI verification.
export function growthFixture({ owner, scenario = "normal" }) {
  const stores = new Map(),
    receipts = new Map(),
    requests = new Map(),
    clone = (v) => JSON.parse(JSON.stringify(v));
  let sequence = 0,
    catalogReads = 0,
    lost = false;
  const created = "2026-10-26T12:00:00Z";
  function data(fid) {
    if (!stores.has(fid)) {
      const memories = [
        {
          id: fid + "-memory-one",
          owner_fluctlight_id: fid,
          type: "relationship",
          content: "你喜欢下雨天，也喜欢坐在窗边读书。",
          actor_refs: [owner],
          conversation_id: "conversation-fixture",
          event_refs: [],
          evidence_refs: ["message:source-fixture"],
          personality_perspectives: [],
          confidence: 0.8,
          importance: 0.8,
          emotional_significance: 0.5,
          visibility: "owner",
          status: "active",
          revision: 2,
          provenance_status: "valid",
          occurred_at: created,
          created_at: created,
          last_confirmed_at: created,
          superseded_by_memory_id: null,
          supersedes_memory_id: null,
          revision_count: 2,
          source_count: 1,
        },
        {
          id: fid + "-memory-two",
          owner_fluctlight_id: fid,
          type: "semantic",
          content: "这是一条等待来源核对的隔离样本记录。",
          actor_refs: [],
          conversation_id: null,
          event_refs: [],
          evidence_refs: [],
          personality_perspectives: [],
          confidence: 0.4,
          importance: 0.4,
          emotional_significance: 0.3,
          visibility: "owner",
          status: "active",
          revision: 1,
          provenance_status: "pending",
          occurred_at: null,
          created_at: created,
          last_confirmed_at: null,
          superseded_by_memory_id: null,
          supersedes_memory_id: null,
          revision_count: 1,
          source_count: 0,
        },
      ];
      const claims = [
        {
          id: fid + "-claim-one",
          fluctlight_id: fid,
          category: "self_perception",
          claim: "我想认真听完一个人的故事，再表达自己的想法。",
          value: {},
          confidence: 0.72,
          evidence_refs: ["message:source-fixture"],
          provenance: { kind: "reflection", fixture: true },
          status: "active",
          expires_at: null,
          revision: 2,
          superseded_by: null,
          created_at: created,
          updated_at: created,
          source_valid: true,
          expired: false,
          owner_review: null,
        },
      ];
      const reflections = [
        {
          id: fid + "-reflection-one",
          fluctlight_id: fid,
          title: "听完，再说",
          summary: "留意到认真听完，比马上给出建议更让对话自然。",
          body: "留意到认真听完，比马上给出建议更让对话自然。",
          state: "applied",
          status: "applied",
          growth: null,
          at: created,
          created_at: created,
          from_sequence: 1,
          to_sequence: 2,
          base_state_revision: 1,
          source_refs: ["message:source-fixture"],
          evidence_refs: ["message:source-fixture"],
          schema_version: "v2",
          model_version: null,
          prompt_version: null,
          policy_version: "fixture",
          correlation_id: "reflection-fixture",
          expected_revisions: {},
          result: {
            status: "no_change",
            counts: {
              developing_self: {
                applied: 0,
                rejected: 0,
                deferred: 0,
                no_change: 1,
              },
            },
            changed_refs: [],
            revisions: {},
            reason_codes: [],
          },
          source_window: { fixture: true },
          source_window_status: "available",
          proposal: {
            developing_self_candidates: [
              { claim: "我愿意先听完故事", category: "self_perception" },
            ],
          },
          references: [],
          dispositions: [
            {
              domain: "developing_self",
              candidate_id: "candidate-fixture",
              candidate_index: 0,
              disposition: "no_change",
              reason_code: "already_known",
              target_ref: null,
              changed_ref: null,
              base_revision: 2,
              resulting_revision: 2,
              evidence_refs: ["message:source-fixture"],
            },
          ],
        },
      ];
      const memoryHistory = new Map(
        memories.map((m) => [
          m.id,
          [
            {
              revision: m.revision,
              base_revision: m.revision - 1,
              operation: "revise",
              content: m.content,
              status: m.status,
              actor_id: owner,
              evidence_refs: clone(m.evidence_refs),
              semantic_reason: "隔离样本当前版本",
              created_at: created,
            },
            {
              revision: 0,
              base_revision: null,
              operation: "create",
              content: "你喜欢在安静的地方读书。",
              status: "active",
              actor_id: owner,
              evidence_refs: ["message:source-fixture"],
              semantic_reason: "隔离样本初始版本",
              created_at: created,
            },
          ],
        ]),
      );
      // Each semantic snapshot is complete and belongs to its own revision.
      // These fictional sources do not implement Core's actor-fact validation.
      const claimSnapshot = (c) => {
        const { owner_review, source_valid, expired, ...snapshot } = clone(c);
        return snapshot;
      };
      const claimHistory = new Map(
        claims.map((c) => {
          const current = claimSnapshot(c), prior = {
            ...clone(current), revision: 1,
            claim: "我想给彼此留一点倾听的空间。",
          };
          const record = (after, before, change) => ({
            id: c.id + "-revision-" + after.revision,
            fluctlight_id: fid, claim_id: c.id,
            revision: after.revision, base_revision: after.revision - 1,
            status: "accepted", change_type: change, reason_code: "accepted",
            before_value: clone(before), after_value: clone(after), candidate: clone(after),
            confidence: after.confidence, evidence_refs: clone(after.evidence_refs),
            provenance: clone(after.provenance), source_window: "reflection-fixture",
            created_at: created,
          });
          return [c.id, [record(current, prior, "update"), record(prior, {}, "create")]];
        }),
      );
      stores.set(fid, {
        memories: scenario === "empty" ? [] : memories,
        claims: scenario === "empty" ? [] : claims,
        reflections: scenario === "empty" ? [] : reflections,
        memoryHistory,
        claimHistory,
        reviews: new Map(),
      });
    }
    return stores.get(fid);
  }
  return async ({ path, method, body, url, json, res }) => {
    const reply = (value, status = 200) => {
      json(res, value, status);
      return true;
    };
    const requestMatch = path.match(
      /^\/api\/owner-requests\/(growth-feedback-[^/]+)$/,
    );
    if (requestMatch && requests.has(requestMatch[1])) {
      const r = requests.get(requestMatch[1]);
      if (++r.reads >= 3) {
        r.value.status = "completed";
        r.value.result = {
          reason: "反馈已进入真实认知处理，是否改变以实际候选和应用结果为准。",
        };
      }
      return reply(clone(r.value));
    }
    const catalog = path.match(
        /^\/api\/fluctlights\/([^/]+)\/(memories|reflections|self-claims)(?:\/([^/]+))?(?:\/([^/]+))?$/,
      ),
      memoryPath = path.match(/^\/api\/memories\/([^/]+)(?:\/([^/]+))?$/);
    if (!catalog && !memoryPath) return false;
    let fid, kind, id, part, state, row;
    if (catalog) {
      [, fid, kind, id, part] = catalog;
      state = data(fid);
      row = id
        ? (kind === "reflections"
            ? state.reflections
            : kind === "self-claims"
              ? state.claims
              : state.memories
          ).find((v) => v.id === id)
        : null;
    } else {
      [, id, part] = memoryPath;
      kind = "memories";
      for (const [candidate, s] of stores) {
        row = s.memories.find((v) => v.id === id);
        if (row) {
          fid = candidate;
          state = s;
          break;
        }
      }
    }
    if (!state)
      return reply({ code: "memory_not_found", message: "记录不存在" }, 404);
    if (method === "GET" && !id) {
      if (kind === "memories") {
        catalogReads++;
        if (scenario === "loading" && catalogReads === 1)
          await new Promise((r) => setTimeout(r, 5000));
        if (scenario === "failure" && catalogReads === 1)
          return reply(
            { code: "request_failed", message: "记忆目录暂时无法读取" },
            503,
          );
      }
      const options = Object.fromEntries(url.searchParams),
        items =
          kind === "memories"
            ? state.memories
            : kind === "reflections"
              ? state.reflections
              : state.claims;
      const filtered = items.filter(
        (v) =>
          (!options.status ||
            options.status === "all" ||
            v.status === options.status) &&
          (!options.type || v.type === options.type) &&
          (!options.q ||
            (v.content || v.claim || v.summary || "")
              .toLowerCase()
              .includes(options.q.toLowerCase())) &&
          (options.aboutOwner !== "true" ||
            (v.actor_refs?.includes(owner) &&
              v.status === "active" &&
              !["pending", "invalid"].includes(v.provenance_status))),
      );
      return reply({
        items: clone(filtered),
        next_cursor: "",
        ...(kind === "reflections" ? { window: {}, latest_task: null } : {}),
      });
    }
    if (!row) return reply({ code: "not_found", message: "记录不存在" }, 404);
    if (method === "GET") {
      if (kind === "memories" && part === "sources") {
        const revision = Number(
          url.searchParams.get("revision") ?? row.revision,
        );
        return reply({
          items: (
            (state.memoryHistory.get(id) || []).find(
              (record) => record.revision === revision,
            )?.evidence_refs || []
          ).map((ref) => ({
            memory_revision: revision,
            source_ref: ref,
            source_kind: ref.startsWith("message:") ? "message" : "owner",
            source_id: ref.startsWith("message:") ? ref.slice(8) : null,
            source_revision: null,
            occurred_at: created,
            recorded_at: created,
            status: "valid",
            live: true,
            superseded_by_ref: null,
            conversation_id: ref.startsWith("message:")
              ? "conversation-fixture"
              : null,
          })),
          next_cursor: "",
        });
      }
      if (part === "revisions")
        return reply({
          items: clone(
            kind === "memories"
              ? state.memoryHistory.get(id) || []
              : state.claimHistory.get(id) || [],
          ),
          next_cursor: "",
        });
      if (part === "reviews")
        return reply({
          items: clone(state.reviews.get(id) || []),
          next_cursor: "",
        });
      if (part === "feedback")
        return reply({
          items: clone(
            [...requests.values()]
              .map((v) => v.value)
              .filter((r) => r.payload.reflection_id === id),
          ),
          next_cursor: "",
        });
      return reply(clone(row));
    }
    const key = JSON.stringify([
      owner,
      fid,
      kind,
      id,
      part || "revise",
      kind === "memories" ? body : body.idempotencyKey,
    ]);
    if (receipts.has(key)) return reply(clone(receipts.get(key)));
    let result;
    if (kind === "memories") {
      if (body.expectedRevision !== row.revision)
        return reply(
          { code: "memory_revision_conflict", message: "记忆版本已变化" },
          409,
        );
      if (!part) {
        row.content = body.content;
        row.evidence_refs = body.evidenceRefs;
      } else if (part === "forget") {
        row.status = "forgotten";
        row.evidence_refs = [
          ...new Set([...row.evidence_refs, ...body.evidenceRefs]),
        ];
      } else if (part === "rollback") {
        const prior = (state.memoryHistory.get(id) || []).find(
          (v) => v.revision === body.targetRevision && v.status === "active",
        );
        if (!prior)
          return reply(
            { code: "memory_command_invalid", message: "历史版本不能恢复" },
            422,
          );
        row.content = prior.content;
        row.status = "active";
        row.evidence_refs = [
          ...new Set([...prior.evidence_refs, ...body.evidenceRefs]),
        ];
      } else return false;
      row.revision++;
      row.revision_count++;
      row.source_count = row.evidence_refs.length;
      const record = {
        revision: row.revision,
        base_revision: row.revision - 1,
        operation: part || "revise",
        content: row.content,
        status: row.status,
        actor_id: owner,
        evidence_refs: clone(row.evidence_refs),
        semantic_reason: body.evidenceRefs?.[0] || "",
        created_at: created,
      };
      state.memoryHistory.set(id, [
        record,
        ...(state.memoryHistory.get(id) || []),
      ]);
      result = {
        id,
        revision: row.revision,
        operation: part || "revise",
        status: row.status,
        disposition: "committed",
        reason_code: "owner_command",
        replayed: false,
        ...(part === "rollback"
          ? { target_revision: body.targetRevision }
          : {}),
      };
    } else if (kind === "reflections" && part === "feedback") {
      const request = {
        id: "growth-feedback-" + ++sequence,
        kind: "reflection_feedback",
        fluctlight_id: fid,
        status: "pending",
        revision: 1,
        payload: { reflection_id: id, text: body.text, stance: body.stance },
        created_at: created,
        result: {},
        result_refs: [],
      };
      requests.set(request.id, { value: request, reads: 0 });
      result = { request, audit_id: "feedback-audit" };
    } else if (kind === "self-claims") {
      if (body.expectedRevision !== row.revision)
        return reply(
          { code: "self_claim_revision_conflict", message: "认识版本已变化" },
          409,
        );
      const { owner_review, source_valid, expired, ...before } = clone(row);
      let reviewID;
      if (part === "review") {
        const review = {
          id: "self-review-" + ++sequence,
          fluctlight_id: fid,
          claim_id: id,
          claim_revision: row.revision,
          owner_actor_id: owner,
          reason: body.reason,
          disposition: "acknowledged",
          created_at: created,
        };
        reviewID = review.id;
        row.owner_review = review;
        state.reviews.set(id, [review, ...(state.reviews.get(id) || [])]);
      } else if (part === "forget") {
        row.revision++;
        row.status = "forgotten";
        row.owner_review = null;
      } else if (part === "restore") {
        const target = (state.claimHistory.get(id) || []).find(
          (v) => v.revision === body.targetRevision && v.status === "accepted",
        );
        if (!target)
          return reply(
            { code: "self_snapshot_unavailable", message: "历史快照不可恢复" },
            422,
          );
        const snapshot = target.after_value?.category ? target.after_value : target.candidate;
        if (!["category", "claim", "value", "confidence", "evidence_refs", "provenance", "status"].every(key => Object.hasOwn(snapshot || {}, key)))
          return reply({ code: "self_snapshot_incomplete", message: "旧修订缺少完整快照" }, 422);
        if (!["active", "uncertain"].includes(snapshot.status))
          return reply({ code: "self_snapshot_invalid", message: "该历史快照不能恢复为有效认识" }, 422);
        const revision = row.revision + 1;
        Object.assign(row, clone(snapshot), {
          id,
          fluctlight_id: fid,
          revision,
          owner_review: null,
        });
      } else return false;
      if (part !== "review") {
        row.updated_at = new Date(Date.parse(created) + ++sequence * 1000).toISOString();
        const { owner_review, source_valid, expired, ...after } = clone(row);
        state.claimHistory.set(id, [{
          id: id + "-revision-" + row.revision, fluctlight_id: fid, claim_id: id,
          revision: row.revision, base_revision: before.revision,
          status: part === "forget" ? "forgotten" : "accepted",
          change_type: part, reason_code: "owner_" + part, source_window: "owner:" + owner,
          candidate: { ...clone(after), reason: body.reason, owner_actor_id: owner },
          before_value: before, after_value: after, confidence: row.confidence,
          evidence_refs: clone(row.evidence_refs), provenance: clone(row.provenance),
          created_at: row.updated_at,
        }, ...(state.claimHistory.get(id) || [])]);
      }
      result = {
        id,
        fluctlight_id: fid,
        operation: part,
        revision: row.revision,
        status: row.status,
        audit_id: "self-audit-" + row.revision + "-" + part,
        ...(part === "review" ? { review_id: reviewID } : { target_revision: part === "restore" ? body.targetRevision : 0 }),
      };
    } else return false;
    receipts.set(key, clone(result));
    if (scenario === "unknown" && !lost) {
      lost = true;
      return reply(
        { code: "request_failed", message: "治理回执尚未确认" },
        503,
      );
    }
    return reply(result);
  };
}
