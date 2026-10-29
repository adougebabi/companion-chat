import test from "node:test";
import assert from "node:assert/strict";
import { creationFixture, validAnalysis } from "./support/creation-fixture.mjs";
const clone = (v) => JSON.parse(JSON.stringify(v));
for(const firstResult of ['success','failure'])test('r18 the newest selected JSON file wins when an older read finishes first: '+firstResult,async()=>{
 let completeA,failA,completeB;const f=creationFixture(async()=>validAnalysis());
 try{f.c.changeMode('json');const a=f.c.importFile({name:'old.json',size:12,text:()=>new Promise((resolve,reject)=>{completeA=resolve;failA=reject;})}),b=f.c.importFile({name:'new.json',size:12,text:()=>new Promise(resolve=>completeB=resolve)});if(firstResult==='success')completeA('{"name":"旧文件"}');else failA(Error('旧文件读取失败'));await a;assert.notEqual(f.c.draft.fileName,'old.json');completeB('{"name":"新文件"}');await b;assert.equal(f.c.draft.fileName,'new.json');assert.equal(f.c.draft.json,'{"name":"新文件"}');assert.equal(f.c.draft.jsonValid,true);assert.equal(f.c.error.value,'');}finally{f.c.dispose();}
});
test('r18 a late JSON file read cannot invalidate a newer running analysis or replace its input',async()=>{
 let readFile,complete;const f=creationFixture(()=>new Promise(resolve=>complete=resolve));
 try{f.c.draft.bio='本次需要分析的描述';f.c.draft.json='原来的 JSON 草稿';const reading=f.c.importFile({name:'late.json',size:2,text:()=>new Promise(resolve=>readFile=resolve)}),original=f.c.draft.requestId,analysis=f.c.analyze();assert.equal(f.c.busy.value,true);readFile('{}');await reading;assert.equal(f.c.draft.json,'原来的 JSON 草稿');assert.equal(f.c.draft.requestId,original);complete(validAnalysis());await analysis;assert.equal(f.c.step.value,'preview');}finally{f.c.dispose();}
});
async function analyzed() {
  const value = validAnalysis();
  value.core_persona.identity = {
    name: "林青",
    timezone: "Asia/Shanghai",
    background_story: "原背景",
    custom: { enabled: false, score: 0 },
  };
  value.core_persona.life_profile = {
    life_habits: "安静地阅读",
    wardrobe_items: [{ description: "旧衬衫", custom: false }],
  };
  value.extensions = { future: { empty: null, flags: [false, 0, "保留"] } };
  value.initial_relationships = [
    { role: "朋友", metrics: { trust: 0.3 }, custom: { keep: true } },
  ];
  value.initial_goals = [{ description: "完成画作" }];
  value.initial_intentions = [{ description: "整理书桌" }];
  const f = creationFixture(async () => clone(value));
  f.c.draft.bio = "喜欢书与绘画";
  await f.c.analyze();
  return { ...f, source: value };
}
test("creation preview JSON incorporates edited fields and preserves the complete nested source without exposing editable analysis credentials", async () => {
  const f = await analyzed();
  try {
    const field = f.c.previewFields.value.find(
      (x) => x.path.join(".") === "identity.background_story",
    );
    field.value = "预览中修改的背景";
    f.c.persistPreview();
    const value = f.c.previewDefinition();
    assert.equal(
      value.core_persona.identity.background_story,
      "预览中修改的背景",
    );
    assert.deepEqual(value.extensions, f.source.extensions);
    assert.deepEqual(
      value.initial_relationships,
      f.source.initial_relationships,
    );
    assert.deepEqual(
      value.core_persona.life_profile.wardrobe_items,
      f.source.core_persona.life_profile.wardrobe_items,
    );
    assert.equal(Object.hasOwn(value, "analysis_id"), false);
    assert.equal(Object.hasOwn(value, "correlation_id"), false);
    assert.equal(
      f.c.draft.analysis.core_persona.identity.background_story,
      "原背景",
    );
  } finally {
    f.c.dispose();
  }
});
test("explicit complete-preview edits preserve original authority and resynchronize the editable grouped fields", async () => {
  const f = await analyzed();
  try {
    const value = f.c.previewDefinition();
    value.core_persona.identity.name = "新的名字";
    value.extensions.future.added = { value: false };
    value.initial_relationships[0].summary = "保留的初始关系";
    assert.equal(f.c.updatePreviewDefinition(value), true);
    assert.equal(
      f.c.previewFields.value.find((x) => x.path.join(".") === "identity.name")
        .value,
      "新的名字",
    );
    assert.equal(f.c.draft.analysis.analysis_id, f.source.analysis_id);
    assert.equal(f.c.draft.analysis.correlation_id, f.source.correlation_id);
    assert.deepEqual(f.c.draft.analysis.extensions, value.extensions);
    assert.equal(
      f.c.draft.analysis.initial_relationships[0].summary,
      "保留的初始关系",
    );
  } finally {
    f.c.dispose();
  }
});
test("malformed or substituted-source preview edits remain unadopted and cannot erase the current definition", async () => {
  const f = await analyzed();
  try {
    const before = clone(f.c.draft.analysis);
    const value = f.c.previewDefinition();
    value.analysis_id = "initialization_source_" + "b".repeat(32);
    assert.equal(f.c.updatePreviewDefinition(value), false);
    assert.deepEqual(clone(f.c.draft.analysis), before);
    assert.match(f.c.error.value, /原分析来源/);
    const missing = f.c.previewDefinition();
    missing.initial_relationships = {};
    assert.equal(f.c.updatePreviewDefinition(missing), false);
    assert.deepEqual(clone(f.c.draft.analysis), before);
  } finally {
    f.c.dispose();
  }
});
test("unapplied full JSON is durable, blocks activation and cannot be discarded by another structured edit", async () => {
  const f = await analyzed();
  try {
    const value = f.c.previewDefinition();
    f.c.draft.previewJsonText = '{ "尚未应用": ';
    f.c.draft.previewJsonBase = JSON.stringify(value);
    f.c.persist();
    await f.c.activate();
    assert.equal(f.calls.length, 0);
    assert.match(f.c.error.value, /先应用或还原/);
    assert.equal(f.c.updatePreviewDefinition(value), false);
    assert.equal(f.c.draft.previewJsonText, '{ "尚未应用": ');
    value.core_persona.identity.name = "核对后的名字";
    assert.equal(
      f.c.updatePreviewDefinition(value, { consumeJson: true }),
      true,
    );
    assert.equal(f.c.draft.previewJsonText, undefined);
    assert.equal(
      f.c.previewFields.value.find((x) => x.path.join(".") === "identity.name")
        .value,
      "核对后的名字",
    );
  } finally {
    f.c.dispose();
  }
});
test("blank creation still skips analysis and its recorded accepted request has no Foundation or analysis authority", async () => {
  let analyses = 0;
  const f = creationFixture(async () => {
    analyses++;
    return validAnalysis();
  });
  try {
    f.c.changeMode("blank");
    f.c.draft.name = "空白朋友";
    await f.c.analyze();
    await f.c.activate();
    assert.equal(analyses, 0);
    assert.equal(f.calls.length, 1);
    const saved = JSON.parse(
      f.saved.getItem("fluctlight-creation:owner:draft"),
    );
    assert.deepEqual(saved.activationSnapshot.body, f.calls[0]);
    assert.equal(
      saved.activationSnapshot.body.initializationMode,
      "blank_slate",
    );
    assert.equal(
      Object.hasOwn(saved.activationSnapshot.body, "corePersona"),
      false,
    );
    assert.equal(
      Object.hasOwn(saved.activationSnapshot.body, "analysisId"),
      false,
    );
  } finally {
    f.c.dispose();
  }
});
test("accepted creation snapshots detach from later local objects and new drafts clear prior errors while history survives", async () => {
  const f = await analyzed();
  try {
    await f.c.activate();
    const saved = JSON.parse(
      f.saved.getItem("fluctlight-creation:owner:draft"),
    );
    assert.deepEqual(saved.activationSnapshot.body, f.calls[0]);
    const frozen = clone(saved.activationSnapshot);
    f.c.draft.analysis.core_persona.identity.name = "后来的本地对象";
    f.c.error.value = "旧失败";
    f.c.feedback.value = { type: "configuration" };
    f.c.fieldErrors.value = { name: "旧名字错误" };
    f.c.worldSourceReview.value = { preview: "旧审阅" };
    f.c.newDraft();
    assert.equal(f.c.draft.activatedId, null);
    assert.equal(f.c.draft.activationSnapshot, undefined);
    assert.equal(f.c.error.value, "");
    assert.equal(f.c.feedback.value, null);
    assert.deepEqual(clone(f.c.fieldErrors.value), {});
    assert.equal(f.c.worldSourceReview.value, null);
    assert.equal(frozen.body.corePersona.identity.name, "林青");
  } finally {
    f.c.dispose();
  }
});

test("clearing a previously named preview never silently restores the original name on activation", async () => {
  const f = await analyzed();
  try {
    f.c.previewFields.value.find(
      (x) => x.path.join(".") === "identity.name",
    ).value = "";
    await f.c.activate();
    assert.equal(f.calls.length, 0);
    assert.equal(f.c.draft.name, "");
    assert.match(f.c.error.value, /填写角色名字/);
  } finally {
    f.c.dispose();
  }
});
test("clearing the preview timezone blocks activation instead of reusing an older timezone", async () => {
  const f = await analyzed();
  try {
    f.c.previewFields.value.find(
      (x) => x.path.join(".") === "identity.timezone",
    ).value = "";
    await f.c.activate();
    assert.equal(f.calls.length, 0);
    assert.equal(f.c.draft.timezone, "");
    assert.match(f.c.error.value, /填写生活时区/);
  } finally {
    f.c.dispose();
  }
});

test("a recovery whose transport outcome is unknown stays reachable after its stage has already become ready", async () => {
  const f = await analyzed(),
    writes = [];
  let ready = false;
  try {
    await f.c.activate();
    f.client.wardrobe = async () => ({ revision: 7 });
    f.client.inventoryPreparation = async () => ({
      fluctlight_id: "created",
      status: ready ? "completed" : "failed",
      can_recover: !ready,
    });
    f.client.prepareInventory = async (fid, body) => {
      writes.push({ fid, body: clone(body) });
      ready = true;
      if (writes.length === 1) throw Error("response lost");
      return { fluctlight_id: fid, status: "completed" };
    };
    await f.c.recoverStage({ key: "wardrobe" });
    assert.equal(
      f.c.progress.value.stages.find((s) => s.key === "wardrobe").state,
      "ready",
    );
    const original = f.c.recoveryPending.value.find(
      (r) => r.key === "wardrobe",
    );
    assert.ok(original);
    f.client.wardrobe = async () => ({ revision: 9 });
    await f.c.recoverStage(original);
    assert.deepEqual(writes[1], writes[0]);
    assert.equal(f.c.recoveryPending.value.length, 0);
    assert.equal(f.calls.length, 1);
  } finally {
    f.c.dispose();
  }
});

test("a known rejected recovery is released only after reviewing actual authority and never resubmits until a new explicit recovery", async () => {
  const f = await analyzed(),
    writes = [];
  let revision = 7,
    ready = false;
  try {
    await f.c.activate();
    f.client.wardrobe = async () => ({ revision });
    f.client.inventoryPreparation = async () => ({
      fluctlight_id: "created",
      status: ready ? "completed" : "failed",
      can_recover: !ready,
    });
    f.client.prepareInventory = async (fid, body) => {
      writes.push({ fid, body: clone(body) });
      if (writes.length === 1)
        throw Object.assign(Error("stale"), {
          status: 409,
          code: "revision_conflict",
        });
      ready = true;
      return { fluctlight_id: fid, status: "completed" };
    };
    await f.c.recoverStage({ key: "wardrobe" });
    const entry = f.c.recoveryPending.value[0];
    assert.ok(entry.original.rejection);
    revision = 8;
    await f.c.reviewRejectedRecovery(entry);
    assert.equal(f.c.recoveryReview.value.actual.revision, 8);
    assert.equal(f.c.recoveryReview.value.original.expectedRevision, 7);
    assert.equal(writes.length, 1);
    await f.c.confirmRejectedRecovery();
    assert.equal(f.c.recoveryPending.value.length, 0);
    assert.equal(writes.length, 1);
    await f.c.recoverStage({ key: "wardrobe" });
    assert.equal(writes[1].body.expectedRevision, 8);
    assert.notEqual(
      writes[1].body.idempotencyKey,
      writes[0].body.idempotencyKey,
    );
    assert.equal(f.calls.length, 1);
  } finally {
    f.c.dispose();
  }
});

test("deleted and null complete-JSON names never reuse an older accepted input name", async () => {
  for (const replacement of ["delete", null, 27]) {
    const f = await analyzed();
    try {
      const value = f.c.previewDefinition();
      if (replacement === "delete") delete value.core_persona.identity.name;
      else value.core_persona.identity.name = replacement;
      assert.equal(f.c.updatePreviewDefinition(value), true);
      await f.c.activate();
      assert.equal(f.calls.length, 0);
      assert.equal(f.c.draft.name, "");
      assert.match(f.c.error.value, /填写角色名字/);
    } finally {
      f.c.dispose();
    }
  }
});
test("deleting the timezone through complete JSON remains intentionally missing and blocks activation", async () => {
  const f = await analyzed();
  try {
    const value = f.c.previewDefinition();
    delete value.core_persona.identity.timezone;
    assert.equal(f.c.updatePreviewDefinition(value), true);
    assert.equal(f.c.draft.analysis.core_persona.identity.timezone, undefined);
    await f.c.activate();
    assert.equal(f.calls.length, 0);
    assert.equal(f.c.draft.timezone, "");
    assert.match(f.c.error.value, /填写生活时区/);
  } finally {
    f.c.dispose();
  }
});

test("an unknown blank activation moves to preparation without claiming that a model analysis failed", async () => {
  const f = creationFixture();
  try {
    f.directory.activate = async () => {
      throw Error("transport lost");
    };
    f.c.changeMode("blank");
    f.c.draft.name = "原空白请求";
    await f.c.analyze();
    await f.c.activate();
    assert.ok(f.c.pending.value);
    assert.equal(f.c.step.value, "progress");
    assert.match(f.c.error.value, /创建请求结果尚未确认/);
    assert.doesNotMatch(f.c.error.value, /人物资料暂未整理/);
  } finally {
    f.c.dispose();
  }
});

test("a definitive activation rejection preserves edited preview data as a durable importable copy while discarding rejected authority", async () => {
  for (const status of [400, 422]) {
    const f = await analyzed();
    try {
      const value = f.c.previewDefinition();
      value.extensions.future.edited = "预览中新增";
      value.core_persona.identity.background_story = "完整 JSON 中修改";
      assert.equal(f.c.updatePreviewDefinition(value), true);
      f.directory.activate = async () => {
        throw Object.assign(Error("invalid definition"), { status });
      };
      await f.c.activate();
      assert.equal(f.c.step.value, "person");
      assert.equal(f.c.draft.analysis, null);
      assert.equal(f.c.pending.value, null);
      const saved = JSON.parse(
        f.saved.getItem("fluctlight-creation:owner:draft"),
      );
      const retained = JSON.parse(saved.rejectedPreviewText);
      assert.deepEqual(retained, value);
      assert.equal(Object.hasOwn(retained, "analysis_id"), false);
      assert.equal(Object.hasOwn(retained, "correlation_id"), false);
      assert.equal(saved.bio, "喜欢书与绘画");
      f.c.changeMode("json");
      f.c.draft.json = saved.rejectedPreviewText;
      let imported;
      f.directory.analyze = async (request) => {
        imported = request.json;
        return {
          ...clone(retained),
          analysis_id: validAnalysis().analysis_id,
          correlation_id: "new-analysis",
        };
      };
      await f.c.analyze();
      assert.deepEqual(imported, retained);
      assert.equal(f.c.draft.analysis.correlation_id, "new-analysis");
    } finally {
      f.c.dispose();
    }
  }
});

test("creation progress requires a nonempty world identity matched by actual readback, without depending on a local selection checkpoint", async () => {
  const f = await analyzed();
  try {
    await f.c.activate();
    f.client.detail = async () => ({ persona_preparation: { status: "ready" } });
    for (const worldRuntimeId of [undefined, null, "", "   "]) {
      f.c.draft.worldRuntimeId = worldRuntimeId;
      f.client.world = async () => ({ world: { id: worldRuntimeId } });
      await f.c.refreshProgress();
      const world = f.c.progress.value.stages.find((s) => s.key === "world");
      assert.equal(world.state, "pending");
      assert.equal(world.retryable, true);
      assert.equal(f.c.progress.value.complete, false);
    }
    f.c.draft.worldRuntimeId = "actual-world";
    f.c.draft.worldSelectionConfirmed = false;
    f.client.world = async () => ({ world: { id: "actual-world" } });
    await f.c.refreshProgress();
    const world = f.c.progress.value.stages.find((s) => s.key === "world");
    assert.equal(world.state, "ready");
    assert.equal(world.retryable, false);
  } finally {
    f.c.dispose();
  }
});
