import test from "node:test";
import { creationRecovery, migrateCreationRecovery, creationHistoryKey } from "../src/approved-ui/creation-recovery.mjs";
import assert from "node:assert/strict";
import { proxy as reactive } from "valtio/vanilla";
import { profileFields, profileChanges, profileSaveCommand, rebaseProfileFields } from "../src/approved-ui/profile-fields.mjs";
import { createCreationController } from "../src/state/creation-controller.mjs";
import { scheduleProjection, diagnosticProjection, pendingIssues, readableSummary } from "../src/approved-ui/presentation.mjs";
const storage = () => {
  const values = new Map();
  return {
    getItem: k => values.get(k) ?? null,
    setItem: (k, v) => values.set(k, String(v)),
    removeItem: k => values.delete(k),
    values
  };
};
const analysis = () => ({
  analysis_id: "initialization_source_" + "a".repeat(32),
  correlation_id: "real-source-correlation",
  schema_version: 2,
  core_persona: {
    identity: {
      name: "雨禾",
      age: "未知",
      height_cm: "尚未确定",
      notes: "保留的记录",
      core_values: []
    },
    personality: {
      openness: 0.5
    },
    behavioral_policy: {},
    life_profile: {
      preferences: {
        likes: ["旧书"],
        custom_record: {
          value: "保留"
        }
      },
      recurring_commitments: [{
        id: "commitment-1",
        activity: "阅读",
        notes: "未编辑"
      }]
    },
    personality_system: {
      mode: "single",
      profiles: []
    }
  },
  developing_self: {
    claims: []
  },
  extensions: {
    source: "kept"
  },
  initial_relationships: [],
  initial_goals: [],
  initial_intentions: []
});
function fixture({
  activate,
  analyze
} = {}) {
  const saved = storage(),
    calls = [],
    a = analysis();
  let world = null;
  const client = {
    detail: async () => ({
      current_revision: 0,
      persona_preparation: {
        status: "ready",
        can_recover: false
      }
    }),
    world: async () => ({
      world
    }),
    updateRuntimeWorld: async (_fid, body) => {
      world = {
        id: "world-new",
        name: body.name
      };
      return world;
    },
    inventoryPreparation: async fid => ({
      fluctlight_id: fid,
      status: "completed",
      can_recover: false
    }),
    visualIdentityManagement: async fid => ({
      fluctlight_id: fid,
      status: "active",
      current_revision: 1,
      session: {
        status: "completed"
      },
      canonical_asset_id: "reference",
      character_sheet_asset_id: "sheet"
    }),
    displayAvatar: async fid => ({
      fluctlight_id: fid,
      avatar: {
        url: "/api/media/avatar"
      }
    }),
    visualIdentityTimeline: async () => ({
      items: []
    }),
    ownerMediaAsset: async () => ({
      readable: true
    }),
    schedules: async (_fid, options) => ({
      days: [{
        local_date: options.from,
        schedule: {
          id: "schedule",
          items: []
        }
      }]
    }),
    lifecycleDiagnostics: async () => ({
      workflowIntents: []
    })
  };
  const directory = {
    analyze: async (request, options) => {
      calls.push(["analysis", structuredClone(request)]);
      return analyze ? analyze(request, options) : structuredClone(a);
    },
    activate: async body => {
      calls.push(["activation", structuredClone(body)]);
      return activate ? activate(body) : {
        id: "new-character"
      };
    },
    move: async () => {}
  };
  const options = {
    owner: "owner",
    client,
    directory,
    storage: saved,
    ownerCurrent: () => true,
    refreshDirectory: async () => {},
    analysisToActivation: (value, name, requestId, timezone) => ({
      requestId,
      name,
      initializationMode: "llm_defined",
      analysisId: value.analysis_id,
      corePersona: structuredClone(value.core_persona),
      initialGoals: value.initial_goals,
      initializationTimezone: timezone
    }),
    today: () => "2026-10-23"
  };
  return {
    saved,
    calls,
    options,
    client
  };
}
test("malformed and incomplete analyses keep the editable original input until a valid retry", async () => {
  let attempt = 0;
  const f = fixture({
    analyze: () => {
      if (++attempt === 1) throw Object.assign(Error("invalid json"), {
        status: 503,
        code: "initialization_response_invalid_json"
      });
      const value = analysis();
      if (attempt === 2) delete value.core_persona.identity;
      return value;
    }
  });
  const c = createCreationController(f.options);
  c.draft.bio = "原始描述";
  c.draft.name = "小林";
  c.next();
  await c.analyze();
  assert.equal(c.step.value, "person");
  assert.equal(c.draft.bio, "原始描述");
  assert.equal(c.pending.value, null);
  assert.equal(c.draft.analysis, null);
  await c.analyze();
  assert.equal(c.step.value, "person");
  assert.equal(c.feedback.value.path, "core_persona.identity");
  assert.match(c.error.value, /模型返回.*字段校验/);
  c.draft.bio = "修正后的描述";
  c.invalidate();
  await c.analyze();
  assert.equal(c.step.value, "preview");
  assert.match(f.calls.at(-1)[1].description, /修正后的描述/);
  await c.activate();
  assert.equal(c.draft.activatedId, "new-character");
  assert.equal(f.calls.filter(([kind]) => kind === "activation").length, 1);
});
test("definitive activation rejection returns to input and never reuses rejected analysis", async () => {
  let attempts = 0;
  const f = fixture({
    activate: () => {
      if (++attempts === 1) throw Object.assign(Error("missing field"), {
        status: 422,
        code: "activation_persona_invalid",
        details: {
          validation_error: {
            path: "core_persona.identity.name",
            type: "required"
          }
        }
      });
      return {
        id: "new-character"
      };
    }
  });
  const c = createCreationController(f.options);
  c.draft.bio = "原始描述";
  c.next();
  await c.analyze();
  await c.activate();
  assert.equal(c.step.value, "person");
  assert.equal(c.pending.value, null);
  assert.equal(c.draft.analysis, null);
  assert.equal(c.draft.bio, "原始描述");
  const first = f.calls.find(([kind]) => kind === "activation")[1].requestId;
  c.draft.bio = "补充身份后的描述";
  c.invalidate();
  await c.analyze();
  await c.activate();
  assert.equal(c.draft.activatedId, "new-character");
  assert.equal(f.calls.filter(([kind]) => kind === "analysis").length, 2);
  assert.notEqual(f.calls.filter(([kind]) => kind === "activation")[1][1].requestId, first);
});
test("activation rejection after an unknown response retains the exact original command", async () => {
  let attempts = 0;
  const f = fixture({
    activate: () => {
      if (++attempts === 1) throw Error("connection lost");
      throw Object.assign(Error("missing field"), {
        status: 422,
        code: "activation_persona_invalid"
      });
    }
  });
  const c = createCreationController(f.options);
  c.draft.bio = "原描述";
  c.next();
  await c.analyze();
  await c.activate();
  const original = JSON.parse(JSON.stringify(c.pending.value.value));
  await c.activate(true);
  assert.deepEqual(c.pending.value.value, original);
  c.reanalyze();
  assert.deepEqual(c.pending.value.value, original);
  assert.notEqual(c.step.value, "person");
});
test("a malformed imported JSON stays available for repair without another upload", async () => {
  const f = fixture(),
    c = createCreationController(f.options);
  c.changeMode("json");
  c.next();
  await c.importFile({
    name: "broken.json",
    size: 10,
    text: async () => '{broken'
  });
  assert.equal(c.draft.json, "{broken");
  assert.equal(c.draft.fileName, "broken.json");
  c.draft.json = '{"name":"修正"}';
  c.invalidate();
  await c.analyze();
  assert.equal(c.step.value, "preview");
  assert.deepEqual(f.calls.find(([kind]) => kind === "analysis")[1].json, {
    name: "修正"
  });
});
test("legacy recovery migration cannot resurrect a released activation command", () => {
  const saved = storage(),
    previous = storage();
  const initial = creationRecovery({
    owner: 'owner',
    storage: previous,
    query: new URLSearchParams()
  });
  previous.setItem(initial.key, JSON.stringify(initial.read()));
  previous.setItem(initial.commandKey, JSON.stringify({
    value: {
      body: {
        requestId: initial.read().requestId
      }
    }
  }));
  migrateCreationRecovery('owner', saved, previous);
  assert.equal(previous.getItem(initial.commandKey), null);
  saved.removeItem(initial.commandKey);
  migrateCreationRecovery('owner', saved, previous);
  assert.equal(saved.getItem(initial.commandKey), null);
});
test("archived progress never adopts or releases another character activation", () => {
  const saved = storage(),
    query = new URLSearchParams('resume=old-role');
  const base = creationRecovery({
    owner: 'owner',
    storage: saved,
    query: new URLSearchParams()
  });
  const draft = {
    ...base.read(),
    activatedId: 'old-role'
  };
  saved.setItem(creationHistoryKey('owner', 'old-role'), JSON.stringify(draft));
  saved.setItem(base.commandKey, 'another-activation');
  const archived = creationRecovery({
    owner: 'owner',
    storage: saved,
    query
  });
  assert.notEqual(archived.commandKey, base.commandKey);
  assert.equal(saved.getItem(archived.commandKey), null);
  archived.clear();
  assert.equal(saved.getItem(base.commandKey), 'another-activation');
});
test("migration never pairs a legacy activation with a newer durable draft", () => {
  const saved = storage(),
    previous = storage();
  const source = creationRecovery({
    owner: 'owner',
    storage: previous,
    query: new URLSearchParams()
  });
  const oldDraft = source.read();
  previous.setItem(source.key, JSON.stringify(oldDraft));
  const command = JSON.stringify({
    value: {
      body: {
        requestId: oldDraft.requestId
      }
    }
  });
  previous.setItem(source.commandKey, command);
  const newer = {
    ...oldDraft,
    requestId: 'newer-request',
    bio: '新的输入'
  };
  saved.setItem(source.key, JSON.stringify(newer));
  migrateCreationRecovery('owner', saved, previous);
  assert.deepEqual(JSON.parse(saved.getItem(source.key)), newer);
  assert.equal(saved.getItem(source.commandKey), null);
  assert.equal(previous.getItem(source.commandKey), command);
});
test("a late activation receipt cannot attach the old role to a replacement draft", async () => {
  let release;
  const waiting = new Promise(resolve => {
    release = resolve;
  });
  const f = fixture({
      activate: () => waiting
    }),
    c = createCreationController(f.options);
  c.draft.bio = "原输入";
  c.next();
  await c.analyze();
  const activation = c.activate();
  await Promise.resolve();
  const key = 'fluctlight-creation:owner:draft';
  const replacement = {
    ...JSON.parse(f.saved.getItem(key)),
    requestId: 'replacement',
    bio: '新的输入',
    analysis: null,
    activatedId: null
  };
  f.saved.setItem(key, JSON.stringify(replacement));
  release({
    id: 'original-role'
  });
  await activation;
  assert.deepEqual(JSON.parse(f.saved.getItem(key)), replacement);
  assert.equal(c.draft.activatedId, null);
});
test("stopping analysis aborts observation and a late result cannot end a newer analysis", async () => {
  let releaseOld,
    releaseNew,
    oldSignal,
    calls = 0;
  const oldResult = new Promise(resolve => releaseOld = resolve),
    newResult = new Promise(resolve => releaseNew = resolve);
  const f = fixture({
    analyze: (_request, options) => {
      calls++;
      if (calls === 1) {
        oldSignal = options.signal;
        return oldResult;
      }
      return newResult;
    }
  });
  const c = createCreationController(f.options);
  c.draft.bio = "原描述";
  c.next();
  const old = c.analyze();
  await Promise.resolve();
  assert.ok(c.analysisStartedAt.value);
  c.cancelAnalysis();
  assert.equal(oldSignal.aborted, true);
  assert.equal(c.step.value, "person");
  assert.equal(c.draft.bio, "原描述");
  c.draft.bio = "新的描述";
  c.invalidate();
  const current = c.analyze();
  await Promise.resolve();
  releaseOld(analysis());
  await old;
  assert.equal(c.busy.value, true);
  assert.equal(c.step.value, "analysis");
  assert.equal(c.draft.analysis, null);
  releaseNew(analysis());
  await current;
  assert.equal(c.step.value, "preview");
  assert.equal(c.draft.bio, "新的描述");
  assert.equal(c.analysisStartedAt.value, null);
});
test("leaving during analysis aborts its observer and reopening cannot adopt its late result", async () => {
  let releaseOld,
    oldSignal,
    calls = 0;
  const oldResult = new Promise(resolve => releaseOld = resolve);
  const f = fixture({
    analyze: (_request, options) => {
      calls++;
      if (calls === 1) {
        oldSignal = options.signal;
        return oldResult;
      }
      return analysis();
    }
  });
  const c = createCreationController(f.options);
  c.draft.bio = "离页前的描述";
  c.next();
  const old = c.analyze();
  await Promise.resolve();
  const oldId = c.draft.requestId;
  c.dispose();
  assert.equal(oldSignal.aborted, true);
  const restored = createCreationController(f.options);
  assert.equal(restored.step.value, "person");
  assert.equal(restored.busy.value, false);
  assert.equal(restored.draft.bio, "离页前的描述");
  assert.notEqual(restored.draft.requestId, oldId);
  assert.equal(calls, 1);
  releaseOld(analysis());
  await old;
  assert.equal(restored.draft.analysis, null);
  assert.equal(JSON.parse(f.saved.getItem("fluctlight-creation:owner:draft")).analysis, null);
  await restored.analyze();
  assert.equal(restored.step.value, "preview");
  assert.equal(calls, 2);
  restored.dispose();
});
test("restoring an interrupted analysis screen returns to editable input and fences the previous controller", async () => {
  let release;
  const f = fixture({
    analyze: () => new Promise(resolve => release = resolve)
  });
  const previous = createCreationController(f.options);
  previous.draft.bio = "刷新前的描述";
  previous.next();
  const old = previous.analyze();
  await Promise.resolve();
  const oldId = previous.draft.requestId,
    restored = createCreationController(f.options);
  assert.equal(restored.step.value, "person");
  assert.equal(restored.draft.bio, "刷新前的描述");
  assert.notEqual(restored.draft.requestId, oldId);
  assert.match(restored.error.value, /原输入已保留/);
  previous.dispose();
  assert.equal(JSON.parse(f.saved.getItem("fluctlight-creation:owner:draft")).requestId, restored.draft.requestId);
  release(analysis());
  await old;
  assert.equal(JSON.parse(f.saved.getItem("fluctlight-creation:owner:draft")).analysis, null);
  restored.dispose();
});
test("profile fields follow canonical nullable numeric/list contracts and preserve untouched nested data", () => {
  const a = analysis(),
    base = reactive({
      identity: a.core_persona.identity,
      behavioral_policy: {},
      life_profile: a.core_persona.life_profile
    }),
    fields = profileFields(base);
  assert.equal(fields.find(f => f.path.join(".") === "identity.age").kind, "text");
  assert.equal(fields.find(f => f.path.join(".") === "identity.height_cm").value, "尚未确定");
  fields.find(f => f.path.at(-1) === "directness").value = "0.6";
  fields.find(f => f.path.at(-1) === "core_values").value = "温柔\n独立";
  fields.find(f => f.path.join(".") === "life_profile.recurring_commitments.0.activity").value = "散步";
  const changes = profileChanges(base, fields);
  assert.equal(changes.behavioral_policy.directness, 0.6);
  assert.deepEqual(changes.identity.core_values, ["温柔", "独立"]);
  assert.equal(changes.life_profile.recurring_commitments[0].id, "commitment-1");
  assert.equal(changes.life_profile.recurring_commitments[0].notes, "未编辑");
  assert.equal(changes.identity.notes, undefined);
  assert.equal(changes.life_profile.preferences, undefined);
  fields.find(f => f.path.at(-1) === "directness").value = "2";
  assert.throws(() => profileChanges(base, fields), /数值/);
});
test("profile save recovers the same proposal before accepting and preserves decision identity", async () => {
  const calls = [];
  let once = true;
  const client = {
    submitFoundationRevision: async (fid, body) => {
      calls.push(["propose", fid, body]);
      return {
        id: "proposal-1",
        fluctlight_id: fid
      };
    },
    acceptFoundationRevision: async (fid, id, body) => {
      calls.push(["accept", fid, id, body]);
      if (once) {
        once = false;
        throw Error("lost response");
      }
      return {id,fluctlight_id:fid,revision:4,status:"accepted"};
    }
  };
  const run = profileSaveCommand({
      client,
      fid: "role",
      live: () => true
    }),
    value = {
      changes: {
        identity: {
          notes: "new"
        }
      },
      expectedRevision: 3,
      reason: "所有者编辑角色设定资料"
    };
  await assert.rejects(run(value, "same-key"));
  await run(value, "same-key");
  assert.deepEqual(calls[0], calls[2]);
  assert.deepEqual(calls[1], calls[3]);
});
test("editing existing primitive lists preserves their number and boolean types", () => {
  const base = {
    identity: {},
    behavioral_policy: {},
    life_profile: {
      preferences: {
        numeric_values: [1, 2],
        boolean_values: [true, false]
      }
    }
  };
  const fields = profileFields(base);
  fields.find(f => f.path.at(-1) === "numeric_values").value = "3\n4";
  fields.find(f => f.path.at(-1) === "boolean_values").value = "否\n是";
  assert.deepEqual(profileChanges(base, fields), {
    life_profile: {
      preferences: {
        numeric_values: [3, 4],
        boolean_values: [false, true]
      }
    }
  });
  fields.find(f => f.path.at(-1) === "numeric_values").value = "invalid";
  assert.throws(() => profileChanges(base, fields), /有效数值/);
});
test("profile save readback preserves newer input even when reverted to the original value", () => {
  const base = {
    identity: {
      name: "Alice"
    },
    behavioral_policy: {},
    life_profile: {}
  };
  const current = profileFields(base);
  current.find(f => f.path.at(-1) === "name").value = "Bob";
  const submitted = structuredClone(current);
  current.find(f => f.path.at(-1) === "name").value = "Alice";
  const accepted = {
    ...base,
    identity: {
      name: "Bob"
    }
  };
  const rebased = rebaseProfileFields(accepted, current, undefined, submitted);
  assert.deepEqual(profileChanges(accepted, rebased), {
    identity: {
      name: "Alice"
    }
  });
});
test("reanalysis and a different published world discard the prior preview edits", async () => {
  const f = fixture(),
    c = createCreationController(f.options);
  c.draft.bio = "第一份描述";
  c.draft.timezone = "Asia/Shanghai";
  await c.analyze();
  c.previewFields.value.find(f => f.path.at(-1) === "name").value = "旧预览";
  c.persistPreview();
  await c.analyze();
  assert.notEqual(c.previewFields.value.find(f => f.path.at(-1) === "name").value, "旧预览");
  c.previewFields.value.find(f => f.path.at(-1) === "name").value = "另一份旧预览";
  c.persistPreview();
  c.selectWorld({
    id: "published-world",
    published_version: "2",
    name: "新世界"
  });
  assert.equal(c.draft.analysis, null);
  assert.deepEqual(c.draft.previewDraft, []);
  assert.deepEqual(c.previewFields.value, []);
  c.dispose();
});
test("description analysis stops at review, preserves preview edits through leaving, and then activates", async () => {
  const f = fixture(),
    c = createCreationController(f.options);
  c.draft.bio = "明确的人物描述";
  c.draft.timezone = "Asia/Shanghai";
  await c.analyze();
  assert.equal(c.step.value, "preview");
  assert.equal(f.calls.filter(c => c[0] === "activation").length, 0);
  const before = c.draft.requestId;
  c.previewFields.value.find(f => f.path.join(".") === "identity.name").value = "审阅后的名字";
  c.previewFields.value.find(f => f.path.at(-1) === "directness").value = ".6";
  c.persistPreview();
  c.dispose();
  const restored = createCreationController(f.options);
  assert.equal(restored.draft.requestId, before);
  assert.equal(restored.previewFields.value.find(f => f.path.at(-1) === "name").value, "审阅后的名字");
  await restored.activate();
  const submitted = f.calls.find(c => c[0] === "activation")[1];
  assert.equal(submitted.name, "审阅后的名字");
  assert.equal(submitted.corePersona.identity.name, "审阅后的名字");
  assert.equal(submitted.corePersona.behavioral_policy.directness, 0.6);
  assert.equal(restored.draft.activatedId, "new-character");
  assert.equal(restored.progress.value.complete, true);
  restored.dispose();
});
test("unknown activation refresh replays the immutable original request rather than edited live data", async () => {
  let once = true;
  const f = fixture({
      activate: async () => {
        if (once) {
          once = false;
          throw Error("network unknown");
        }
        return {
          id: "original-character"
        };
      }
    }),
    c = createCreationController(f.options);
  c.draft.bio = "原描述";
  c.draft.timezone = "Asia/Shanghai";
  await c.analyze();
  await c.activate();
  assert.ok(c.pending.value);
  const original = f.calls.find(c => c[0] === "activation")[1];
  c.dispose();
  const restored = createCreationController(f.options);
  restored.draft.name = "较新的未提交输入";
  await restored.activate(true);
  const requests = f.calls.filter(c => c[0] === "activation");
  assert.deepEqual(requests[1][1], original);
  assert.equal(restored.draft.activatedId, "original-character");
  restored.dispose();
});
test("restored activated drafts without dates use a saved local-day baseline for schedule progress", async () => {
  const f = fixture();
  f.saved.setItem("fluctlight-creation:owner:draft", JSON.stringify({
    version: 1,
    step: 3,
    requestId: "existing",
    activatedId: "existing-character",
    timezone: "Asia/Shanghai",
    mode: "blank",
    name: "已有角色",
    useDefaultWorld: true,
    worldRuntimeId: "world-new",
    groupConfirmed: true
  }));
  const c = createCreationController(f.options);
  assert.equal(c.draft.activationDate, "2026-10-23");
  await c.refreshProgress();
  assert.equal(c.progress.value.stages.find(s => s.key === "schedule").state, "ready");
  c.dispose();
});
test("blank creation never calls a model or supplies a foundation payload", async () => {
  const f = fixture(),
    c = createCreationController(f.options);
  c.changeMode("blank");
  c.draft.name = "空白角色";
  c.draft.timezone = "Asia/Shanghai";
  await c.analyze();
  assert.equal(c.step.value, "preview");
  assert.equal(f.calls.length, 0);
  await c.activate();
  const body = f.calls.find(c => c[0] === "activation")[1];
  assert.equal(body.initializationMode, "blank_slate");
  assert.equal(body.corePersona, undefined);
  c.dispose();
});
test("diagnostic summary hides raw JSON, shows system scope and rounded authoritative timing", () => {
  const r = {
    ref: "actual",
    occurred_at: "2026-10-23T00:00:00Z",
    data: {
      status: "completed",
      scenario: "initialization",
      started_at: "2026-10-23T00:00:00Z",
      completed_at: "2026-10-23T00:01:53.744Z",
      latency_ms: 113744,
      prompt: {
        content: '{"items":[1,2]}'
      },
      response: {
        content: '{"items":[1,2]}'
      }
    }
  };
  const view = diagnosticProjection(r, 0, []);
  assert.equal(view.character, "创建中的人物");
  assert.equal(view.durationLabel, "114 秒");
  assert.doesNotMatch(view.summary, /[{}]/);
  assert.match(view.response, /items/);
  assert.equal(readableSummary({
    content: '{"text":"可读回应"}'
  }), "可读回应");
});
test("pending wardrobe failures deduplicate by character while retaining the latest original link", () => {
  const issues = pendingIssues([{
    id: "old",
    title: "wardrobe.initialize",
    fluctlight_id: "A",
    status: "failed",
    created_at: "2026-10-22",
    href: "/diag/task/old"
  }, {
    id: "new",
    title: "wardrobe.initialize",
    fluctlight_id: "A",
    status: "failed",
    created_at: "2026-10-23",
    href: "/diag/task/new"
  }], [{
    id: "A",
    name: "雨禾"
  }]);
  assert.equal(issues.length, 1);
  assert.equal(issues[0]._pending.id, "new");
  assert.match(issues[0].label, /雨禾.*初始穿搭/);
  assert.doesNotMatch(issues[0].reason, /failed/);
});
test("unknown schedule fields do not manufacture midnight while real midnight stays valid", () => {
  const format = value => value ? "00:00" : "";
  assert.equal(scheduleProjection({
    id: "s",
    activity: "睡觉",
    start_at: "2026-10-23T00:00:00Z"
  }, {}, format).time, "00:00");
  const placeholder = scheduleProjection({
    activity: "未安排时间",
    start_at: "2026-10-23T00:00:00Z"
  }, {}, format);
  assert.equal(placeholder.time, "");
  assert.equal(placeholder.editable, false);
});
test("failed initial schedule exposes recovery and retries the identical command without creating another character", async () => {
  const f = fixture(),
    received = [];
  f.client.schedules = async () => ({
    days: []
  });
  f.client.lifecycleDiagnostics = async () => ({
    workflowIntents: [{
      intentType: "schedule.current_day",
      intentId: "schedule_intent:existing",
      status: "failed",
      lastError: "empty_response"
    }]
  });
  let attempt = 0;
  f.client.recoverInitialSchedule = async (fid, body) => {
    received.push([fid, structuredClone(body)]);
    if (++attempt === 1) throw Error("connection lost");
    return {
      status: "pending",
      restarted: true
    };
  };
  f.saved.setItem("fluctlight-creation:owner:draft", JSON.stringify({
    version: 1,
    step: 3,
    requestId: "original",
    activatedId: "existing",
    timezone: "Asia/Shanghai",
    mode: "blank",
    name: "已有角色",
    useDefaultWorld: true,
    worldRuntimeId: "world-new",
    groupConfirmed: true
  }));
  const c = createCreationController(f.options);
  await c.refreshProgress();
  const stage = c.progress.value.stages.find(s => s.key === "schedule");
  assert.equal(stage.state, "failed");
  assert.equal(stage.retryable, true);
  assert.equal(c.progress.value.stages.find(s => s.key === "reference").retryable, false);
  await c.recoverStage(stage);
  assert.match(c.error.value, /恢复结果尚未确认/);
  await c.recoverStage(stage);
  assert.equal(received.length, 2);
  assert.deepEqual(received[0], received[1]);
  assert.ok(received[0][1].idempotencyKey);
  assert.equal(c.draft.activatedId, "existing");
  assert.equal(f.calls.length, 0);
  c.dispose();
});
