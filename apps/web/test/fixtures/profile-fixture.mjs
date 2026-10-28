// Owner-authorized isolated UI fixture. No Provider/model calls or production import.
export function profileFixture({
  owner = "profile-owner",
  scenario = "normal",
  baseDetail,
  onIdentity,
} = {}) {
  const stores = new Map(),
    calls = [],
    clone = (v) => JSON.parse(JSON.stringify(v));
  let sequence = 0,
    firstRead = 0,
    lost = false;
  const now = "2026-10-26T12:00:00Z";
  const merge = (base, patch) => {
    const next = clone(base || {});
    for (const [key, value] of Object.entries(patch || {}))
      next[key] =
        value && typeof value === "object" && !Array.isArray(value)
          ? merge(next[key], value)
          : clone(value);
    return next;
  };
  function state(fid) {
    if (!stores.has(fid)) {
      const seed = baseDetail?.(fid) || {
        id: fid,
        status: "active",
        identity: {
          name: "林青",
          gender: "female",
          age: 26,
          occupation: "书籍编辑",
          timezone: "Asia/Shanghai",
          background_story: "把日常慢慢读成一段故事。",
          custom_field: "自定义字段保留",
        },
        context: { location: "家中", activity: "阅读" },
        inner_state: { mood: { label: "平静" } },
      };
      const identity = {
          ...seed.identity,
          nickname: "小青",
          residence: "青岚",
          birthplace: "南方小城",
          core_values: ["诚实", "尊重"],
        },
        personality = {
          openness: 0.82,
          conscientiousness: 0.72,
          agreeableness: 0.74,
          update_policy: { cooldown_seconds: 86400 },
        },
        policy = {
          response_style: "自然随和",
          message_length: "medium",
          directness: 0.7,
          initiative: 0.4,
          conflict_style: "温和协商",
          refusal_style: "温和而坚定",
        },
        life = {
          life_habits: "睡前阅读。",
          preferences: {
            likes: ["小说", "绘画"],
            stable_preferences: ["留出独处时间"],
          },
          character_constraints: "尊重彼此的选择。",
          appearance: {
            description: "齐肩长发，偏好简单舒适的穿搭。",
            physical_features: { hair_color: "black" },
          },
          extension: { schema_id: "keep-extension", numeric_values: [1, 2] },
        };
      let detail = {
        ...clone(seed),
        id: fid,
        current_revision: 4,
        identity,
        personality,
        behavioral_policy: policy,
        life_profile: life,
        core_persona: {
          identity: clone(identity),
          personality: clone(personality),
          behavioral_policy: clone(policy),
          life_profile: clone(life),
          personality_system: {
            profiles: [
              {
                id: "sunny",
                name: "日常人格",
                description: "日常的交流方式。",
                behavioral_policy: { response_style: "自然随和" },
              },
              {
                id: "focused",
                name: "专注人格",
                description: "认真讨论时保留清晰的表达。",
                behavioral_policy: { response_style: "简洁直接" },
              },
            ],
            persistent_switch_rules: [
              {
                id: "rule-fixture",
                from_profile_id: "sunny",
                target_profile_id: "focused",
              },
            ],
          },
        },
        personality_runtime: { active_profile_id: "sunny", revision: 2 },
        persona_compilation: {
          owner_switch_targets: [{ target_profile_id: "focused" }],
          owner_switch_status: "assessment_required",
          working_persona: {
            portrait_text: "openness: 0.82\n愿意听完日常里的细节。",
          },
        },
        persona_preparation: { status: "ready", can_recover: false },
        persona_switch_history: [],
        persona_switch_history_has_more: false,
        current_state: { appearance: null },
        developing_self: { claims: [] },
        initialization_mode: "structured",
        initialization_source: { mode: "structured", source_kind: "owner" },
        provenance: { fixture: true },
      };
      if (scenario === "empty")
        detail = {
          id: fid,
          status: "active",
          current_revision: 0,
          identity: {
            name: seed.identity?.name || "新角色",
            timezone: "Asia/Shanghai",
          },
          core_persona: {
            identity: {},
            personality: {},
            behavioral_policy: {},
            life_profile: {},
            personality_system: { profiles: [] },
          },
          personality: {},
          behavioral_policy: {},
          life_profile: {},
          current_state: { appearance: null },
          developing_self: { claims: [] },
          persona_preparation: { status: "ready", can_recover: false },
          initialization_mode: "blank_slate",
        };
      const initial = {
        id: fid + "-foundation-initial",
        fluctlight_id: fid,
        revision: detail.current_revision,
        base_revision: detail.current_revision,
        source: "initialization",
        status: "accepted",
        actor_id: owner,
        reason: "创建时确认的隔离样本",
        created_at: now,
        accepted_at: now,
        changes: { identity: clone(detail.identity) },
        identity: clone(detail.identity),
        core_persona: clone(detail.core_persona),
        personality: clone(detail.personality),
        behavioral_policy: clone(detail.behavioral_policy),
        life_profile: clone(detail.life_profile),
        base_snapshot: null,
      };
      const proposal = {
        ...clone(initial),
        id: fid + "-foundation-proposed",
        revision: detail.current_revision + 1,
        base_revision: detail.current_revision,
        source: "reflection",
        status: "proposed",
        reason: "把已有的生活偏好带入设定审阅。",
        accepted_at: null,
        changes: {
          life_profile: { life_habits: "睡前阅读，给自己留一点安静的时间。" },
        },
        base_snapshot: {
          identity: clone(detail.identity),
          personality: clone(detail.personality),
          behavioral_policy: clone(detail.behavioral_policy),
          life_profile: clone(detail.life_profile),
        },
      };
      stores.set(fid, {
        detail,
        foundations: scenario === "empty" ? [] : [proposal, initial],
        capabilities:
          scenario === "empty"
            ? []
            : [
                {
                  id: fid + "-capability-one",
                  fluctlight_id: fid,
                  capability_key: "fixture.story.read",
                  title: "查阅自己的阅读记录",
                  description: "读取已有资料，用于回顾读过的故事。",
                  rationale: "隔离样本中的实际需求。",
                  desired_contract: {},
                  side_effect_class: "read",
                  priority: "normal",
                  source_fact_id: "fixture-source",
                  evidence_refs: [],
                  status: "proposed",
                  revision: 1,
                  review_note: null,
                  reviewer_actor_id: null,
                  capability_version: null,
                  created_at: now,
                  updated_at: now,
                },
              ],
        histories: new Map(),
        proposals: new Map(),
        requests: new Map(),
        capReceipts: new Map(),
        rollbackReceipts: new Map(),
        failDetails: 0,
        lagRuntime: null,
        switchTerminal:
          scenario === "switch-rejected" ? "rejected" : "completed",
        switchReads: 0,
      });
    }
    return stores.get(fid);
  }
  function failure(res, json, message = "隔离样本读取未完成") {
    json(res, { code: "request_failed", message }, 503);
    return true;
  }
  async function handle({
    path,
    method = "GET",
    body = {},
    url = new URL(path, "http://fixture.test"),
    json,
    res,
  }) {
    const role = path.match(
      /^\/api\/fluctlights\/([^/]+)\/(detail|world|persona-switch|persona-preparation\/recover|foundation-revisions(?:\/([^/]+))?(?:\/(accept|reject|history))?|capability-requests(?:\/([^/]+))?(?:\/(review|history))?|self-claims|wardrobe|inventory(?:\/([^/]+))?)$/,
    );
    const req = path.match(/^\/api\/owner-requests\/(profile-persona-[^/]+)$/);
    if (!role && !req) return false;
    calls.push({
      path,
      method,
      body: clone(body),
      query: Object.fromEntries(url.searchParams),
    });
    const reply = (value, status = 200) => {
      json(res, clone(value), status);
      return true;
    };
    if (req) {
      for (const [fid, s] of stores) {
        const record = s.requests.get(req[1]);
        if (!record) continue;
        if (record.status === "pending" && ++record._reads >= 3) {
          if (s.switchTerminal === "rejected") {
            record.status = "rejected";
            record.result = {
              request_id: record.id,
              status: "rejected",
              error_code: "persona_switch_condition_not_met",
            };
          } else {
            const target = record.payload.target_persona_id,
              old = s.detail.personality_runtime.active_profile_id;
            s.detail.personality_runtime = {
              active_profile_id: target,
              revision:
                s.detail.personality_runtime.revision +
                (target === old ? 0 : 1),
            };
            record.status = "completed";
            record.result = {
              request_id: record.id,
              status: "completed",
              disposition: target === old ? "no_change" : "applied",
              revision: s.detail.current_revision,
              runtime_revision: s.detail.personality_runtime.revision,
              active_profile_id: target,
              previous_profile_id: old,
            };
            s.detail.persona_switch_history.unshift({
              audit_id: "switch-audit-" + record.id,
              occurred_at: now,
              active_profile_id: target,
              previous_profile_id: old,
              switch_reason: record.payload.reason,
              revision: s.detail.personality_runtime.revision,
            });
            s.detail.persona_compilation.owner_switch_targets =
              s.detail.core_persona.personality_system.profiles
                .filter((p) => p.id !== target)
                .map((p) => ({ target_profile_id: p.id }));
          }
          record.revision++;
        }
        return reply(record);
      }
      return reply({ code: "not_found", message: "原请求不存在" }, 404);
    }
    const [, fid, end, foundationId, decision, capId, capPart, inventoryPart] =
        role,
      s = state(fid),
      d = s.detail;
    if (end === "detail") {
      firstRead++;
      if (s.failDetails > 0) {
        s.failDetails--;
        return failure(res, json);
      }
      if (scenario === "loading" && firstRead === 1)
        await new Promise((r) => setTimeout(r, 5000));
      if (scenario === "failure" && firstRead === 1)
        return failure(res, json, "角色资料暂时无法读取");
      const result = clone(d);
      result.foundation_revisions = clone(s.foundations);
      if (s.lagRuntime) result.personality_runtime = clone(s.lagRuntime);
      return reply(result);
    }
    if (end === "world")
      return reply({
        available: scenario !== "empty",
        world:
          scenario === "empty"
            ? null
            : { id: "fixture-world", name: "温和魔法世界" },
        places: [],
        npcs: [],
        scenes: [],
      });
    if (end === "self-claims") return reply({ items: [], next_cursor: "" });
    if (end === "persona-preparation/recover") {
      d.persona_preparation = { status: "ready", can_recover: false };
      return reply(d.persona_preparation);
    }
    if (end === "persona-switch") {
      const stored = s.proposals.get("switch:" + body.idempotencyKey);
      if (stored) {
        if (JSON.stringify(stored.body) !== JSON.stringify(body))
          return reply(
            { code: "idempotency_key_reused", message: "原切换内容不一致" },
            422,
          );
        return reply(stored.response);
      }
      if (
        body.expectedRevision !== d.current_revision ||
        body.expectedRuntimeRevision !== d.personality_runtime?.revision
      )
        return reply(
          { code: "revision_conflict", message: "人格或运行版本已变化" },
          409,
        );
      const id = "profile-persona-" + ++sequence,
        old = d.personality_runtime.active_profile_id,
        same = old === body.targetPersonaId;
      const result = {
        request_id: id,
        status: same ? "completed" : "pending",
        status_url: "/api/owner-requests/" + id,
        disposition: same ? "no_change" : "assessment_pending",
        revision: d.current_revision,
        runtime_revision: d.personality_runtime.revision,
        active_profile_id: old,
        previous_profile_id: old,
        audit_id: "switch-audit-" + id,
        persona_action_audit_id: null,
      };
      const request = {
        id,
        kind: "persona_switch",
        fluctlight_id: fid,
        status: result.status,
        revision: 1,
        payload: {
          target_persona_id: body.targetPersonaId,
          expected_revision: body.expectedRevision,
          expected_runtime_revision: body.expectedRuntimeRevision,
          reason: body.reason,
        },
        result: clone(result),
        result_refs: [],
        created_at: now,
        _reads: 0,
      };
      s.requests.set(id, request);
      const response = { ...result, request: clone(request) };
      s.proposals.set("switch:" + body.idempotencyKey, {
        body: clone(body),
        response,
      });
      if (scenario === "switch-unknown" && !lost) {
        lost = true;
        return failure(res, json, "人格切换回执尚未确认");
      }
      return reply(response, same ? 200 : 202);
    }
    if (end.startsWith("foundation-revisions")) {
      if (foundationId === "rollback" && method === "POST") {
        const key = body.expectedRevision + ":" + body.targetRevision;
        const prior = s.rollbackReceipts.get(key);
        if (prior)
          return prior.reason === body.reason
            ? reply({ ...prior.result, replayed: true })
            : reply(
                {
                  code: "foundation_rollback_idempotency_conflict",
                  message: "原恢复说明不一致",
                },
                409,
              );
        if (body.expectedRevision !== d.current_revision)
          return reply(
            { code: "revision_conflict", message: "身份版本已变化" },
            409,
          );
        const source = s.foundations.find(
          (row) =>
            row.revision === body.targetRevision && row.status === "accepted",
        );
        if (!source || !source.core_persona)
          return reply(
            {
              code: "foundation_revision_not_found",
              message: "历史身份快照不存在",
            },
            422,
          );
        const row = {
          ...clone(source),
          id: fid + "-rollback-" + ++sequence,
          revision: d.current_revision + 1,
          base_revision: d.current_revision,
          source: "owner_rollback",
          reason: body.reason,
          changes: { rollback_from_revision: body.targetRevision },
          created_at: now,
          accepted_at: now,
        };
        for (const key of [
          "identity",
          "personality",
          "behavioral_policy",
          "life_profile",
          "core_persona",
        ])
          d[key] = clone(source[key]);
        d.current_revision = row.revision;
        s.foundations.unshift(row);
        onIdentity?.(fid, d.identity);
        const result = {
          id: row.id,
          fluctlight_id: fid,
          revision: row.revision,
          status: "accepted",
          reason: body.reason,
          replayed: false,
        };
        s.rollbackReceipts.set(key, {
          reason: body.reason,
          result: clone(result),
        });
        return reply(result);
      }
      if (method === "GET") {
        if (foundationId) {
          const row = s.foundations.find((r) => r.id === foundationId);
          if (!row)
            return reply({ code: "not_found", message: "修订不存在" }, 404);
          return decision === "history"
            ? reply({
                items: s.histories.get(foundationId) || [],
                next_cursor: "",
              })
            : reply(row);
        }
        return reply({ items: s.foundations, next_cursor: "" });
      }
      if (foundationId) {
        const row = s.foundations.find((r) => r.id === foundationId);
        if (!row)
          return reply({ code: "not_found", message: "修订不存在" }, 404);
        if (decision === "accept" || decision === "reject") {
          const wanted = decision === "accept" ? "accepted" : "rejected";
          if (row.status === wanted) {
            if (row.reason !== body.reason)
              return reply(
                {
                  code: "foundation_decision_idempotency_conflict",
                  message: "原决定说明不一致",
                },
                409,
              );
            return reply({
              id: row.id,
              fluctlight_id: fid,
              revision: row.revision,
              status: wanted,
              reason: row.reason,
              replayed: true,
            });
          }
          if (row.status !== "proposed")
            return reply(
              {
                code: "foundation_revision_not_proposed",
                message: "修订已经处理",
              },
              409,
            );
          if (
            body.expectedRevision !== d.current_revision ||
            (decision === "accept" && row.base_revision !== d.current_revision)
          )
            return reply(
              { code: "foundation_proposal_stale", message: "修订基线已变化" },
              409,
            );
          row.status = wanted;
          row.reason = body.reason;
          if (wanted === "accepted") {
            for (const key of [
              "identity",
              "personality",
              "behavioral_policy",
              "life_profile",
            ])
              if (row.changes[key]) d[key] = merge(d[key], row.changes[key]);
            d.core_persona = merge(d.core_persona, row.changes);
            d.current_revision++;
            row.revision = d.current_revision;
            row.accepted_at = now;
            for (const key of [
              "identity",
              "personality",
              "behavioral_policy",
              "life_profile",
              "core_persona",
            ])
              row[key] = clone(d[key]);
            onIdentity?.(fid, d.identity);
          }
          s.histories.set(row.id, [
            {
              id: "history-" + ++sequence,
              reason: body.reason,
              operation: decision,
              created_at: now,
            },
          ]);
          if (scenario === "readback-failure") s.failDetails = 1;
          const result = {
            id: row.id,
            fluctlight_id: fid,
            revision: row.revision,
            status: wanted,
            reason: body.reason,
            replayed: false,
          };
          if (scenario === "accept-unknown" && !lost) {
            lost = true;
            return failure(res, json, "接受修订回执尚未确认");
          }
          return reply(result);
        }
        return false;
      }
      const key = body.idempotencyKey,
        prior = s.proposals.get(key);
      if (prior) {
        if (JSON.stringify(prior.body) !== JSON.stringify(body))
          return reply(
            { code: "idempotency_key_reused", message: "原提议内容不一致" },
            422,
          );
        return reply(prior.row);
      }
      if (body.expectedRevision !== d.current_revision)
        return reply(
          { code: "revision_conflict", message: "身份版本已变化" },
          409,
        );
      const row = {
        id: fid + "-proposal-" + ++sequence,
        fluctlight_id: fid,
        revision: d.current_revision + 1,
        base_revision: d.current_revision,
        status: "proposed",
        source: "owner",
        actor_id: owner,
        reason: body.reason,
        created_at: now,
        accepted_at: null,
        changes: clone(body.changes),
        base_snapshot: {
          identity: clone(d.identity),
          personality: clone(d.personality),
          behavioral_policy: clone(d.behavioral_policy),
          life_profile: clone(d.life_profile),
        },
      };
      s.foundations.unshift(row);
      s.proposals.set(key, { body: clone(body), row });
      return reply(row);
    }
    if (end.startsWith("capability-requests")) {
      if (method === "GET") {
        if (!capId) return reply({ items: s.capabilities, next_cursor: "" });
        const row = s.capabilities.find((r) => r.id === capId);
        if (!row)
          return reply({ code: "not_found", message: "申请不存在" }, 404);
        return capPart === "history"
          ? reply({ items: s.histories.get(capId) || [], next_cursor: "" })
          : reply({
              ...row,
              registered_capability: {
                available: true,
                version: "1.0-fixture",
              },
            });
      }
      const row = s.capabilities.find((r) => r.id === capId);
      if (!row) return reply({ code: "not_found", message: "申请不存在" }, 404);
      const cached = s.capReceipts.get(body.idempotencyKey);
      if (cached)
        return JSON.stringify(cached.body) === JSON.stringify(body)
          ? reply(cached.result)
          : reply(
              { code: "idempotency_key_reused", message: "原审核内容不一致" },
              422,
            );
      if (body.expectedRevision !== row.revision)
        return reply(
          { code: "capability_review_conflict", message: "申请版本已变化" },
          409,
        );
      if (
        !(
          {
            proposed: ["reviewing", "accepted", "rejected", "cancelled"],
            reviewing: ["accepted", "rejected", "cancelled"],
            accepted: ["fulfilled", "cancelled"],
          }[row.status] || []
        ).includes(body.status)
      )
        return reply(
          {
            code: "capability_review_transition_invalid",
            message: "申请状态不允许此操作",
          },
          422,
        );
      if (
        body.status === "fulfilled" &&
        body.capabilityVersion !== "1.0-fixture"
      )
        return reply(
          { code: "capability_version_mismatch", message: "实现版本不匹配" },
          422,
        );
      row.status = body.status;
      row.revision++;
      row.review_note = body.note;
      row.reviewer_actor_id = owner;
      row.capability_version = body.capabilityVersion || null;
      s.histories.set(capId, [
        {
          id: "cap-review-" + ++sequence,
          note: body.note,
          status: body.status,
          created_at: now,
        },
      ]);
      s.capReceipts.set(body.idempotencyKey, {
        body: clone(body),
        result: clone(row),
      });
      return reply(row);
    }
    const item = {
      id: "profile-shirt",
      fluctlight_id: fid,
      item_kind: "wearable",
      category: "upper_body",
      slot: "upper_body",
      clothing_type: "upper_body",
      description: "正式登记的白色衬衫",
      ownership: "owned",
      availability: "available",
      revision: 1,
      seasons: ["autumn"],
      source_kind: "owner",
      source_ref: "fixture",
      source_item_key: "fixture-shirt",
      source_verified: true,
      archived_at: null,
      media: [],
      worn: null,
      use: null,
    };
    if (end === "wardrobe")
      return reply({
        revision: 7,
        items: scenario === "empty" ? [] : [item],
        worn_items: scenario === "empty" ? [] : [item],
        used_items: [],
        inventory_complete: true,
        next_cursor: "",
        has_more: false,
      });
    if (end === "inventory")
      return reply({
        revision: 7,
        items: scenario === "empty" ? [] : [item],
        inventory_complete: true,
        next_cursor: "",
      });
    if (inventoryPart === "outfits")
      return reply({
        profile_id: d.personality_runtime?.active_profile_id || "sunny",
        items: [],
        next_cursor: "",
      });
    if (inventoryPart === "tags")
      return reply({ tags: ["日常", "外出"], revision: 2 });
    return false;
  }
  return { handler: handle, state, calls };
}
