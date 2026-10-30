import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { creationFixture, validAnalysis } from "./support/creation-fixture.mjs";
const env = installDOM(),
  vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  }),
  { default: View } = await vite.ssrLoadModule(
    "/src/react-app/views/CharacterWizard.jsx",
  );
test.after(async () => {
  await vite.close();
  env.restore();
});
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll("[role=dialog]")].at(-1);
async function preview() {
  const source = validAnalysis();
  source.core_persona.identity.name = "原名字";
  source.core_persona.life_profile = {
    custom_extension: null,
    life_habits: "阅读",
    wardrobe_items: [],
  };
  const f = creationFixture(async () => source);
  f.c.draft.bio = "一段描述";
  await f.c.analyze();
  const m = {
    c: f.c,
    draft: f.c.draft,
    step: "preview",
    busy: false,
    pending: null,
    currentStep: 2,
    previewFields: f.c.previewFields.value,
    error: "",
    fieldErrors: {},
    groups: [],
    existingWorlds: [],
    worlds: [],
    templates: [],
    canContinue: true,
    showError: (message) => (f.c.error.value = message),
    prev() {},
    goStep() {},
    newDraft() {},
    configHref: () => "#/settings",
  };
  const v = await mountReact(vite, View, { props: { model: m } });
  return { ...f, v, m };
}
async function selectTab(f, name) {
  await act(async () =>
    [...f.v.host.querySelectorAll("[role=tab]")]
      .find((n) => n.textContent.trim() === name)
      .dispatchEvent(
        new window.MouseEvent("mousedown", {
          bubbles: true,
          button: 0,
          ctrlKey: false,
        }),
      ),
  );
  await f.v.flush();
}
test("opening and applying a nullable structured extension preserves null without manufacturing an empty collection", async () => {
  const f = await preview();
  try {
    await selectTab(f, "生活设定");
    const card = [...f.v.host.querySelectorAll(".creation-structured")].find(
      (n) => n.querySelector("h2")?.textContent === "补充设定",
    );
    await f.v.click(button(card, "编辑内容"));
    assert.equal(dialog().querySelector("textarea").value, "null");
    await f.v.click(button(dialog(), "应用修改"));
    assert.equal(
      f.c.draft.analysis.core_persona.life_profile.custom_extension,
      null,
    );
    assert.equal(dialog(), undefined);
  } finally {
    await f.v.dispose();
    f.c.dispose();
  }
});
test("cancelling a structured edit leaves its original scalar type and value unchanged", async () => {
  const f = await preview();
  try {
    await selectTab(f, "生活设定");
    const card = [...f.v.host.querySelectorAll(".creation-structured")].find(
      (n) => n.textContent.includes("生活习惯"),
    );
    await f.v.click(button(card, "编辑内容"));
    await f.v.input(dialog().querySelector("textarea"), '"取消的输入"');
    await f.v.click(button(dialog(), "取消"));
    assert.equal(
      f.c.draft.analysis.core_persona.life_profile.life_habits,
      "阅读",
    );
  } finally {
    await f.v.dispose();
    f.c.dispose();
  }
});

test("legacy creation without a local world checkpoint exposes meeting after actual world readiness and keeps unresolved binding review reachable", async () => {
  const f = await preview();
  try {
    f.c.draft.activatedId = "created";
    f.c.draft.worldSource = { worldId: "source", fluctlightId: "source-character" };
    delete f.c.draft.worldSelectionConfirmed;
    const model = {
      ...f.m,
      step: "complete",
      currentStep: 3,
      progress: { complete: true, stages: [{ key: "world", title: "世界", state: "ready" }] },
    };
    await f.v.update({ model });
    assert.equal(f.v.host.querySelector('.creation-meet a')?.getAttribute('href'), "#/c/created/chat");
    assert.equal(button(f.v.host, "核对已有世界的最新配置"), undefined);
    await f.v.update({ model: {
      ...model,
      step: "progress",
      progress: { complete: false, stages: [{ key: "world", title: "世界", state: "pending" }] },
    } });
    assert.equal(f.v.host.querySelector('.creation-meet'), null);
    assert.ok(button(f.v.host, "核对已有世界的最新配置"));
  } finally {
    await f.v.dispose();
    f.c.dispose();
  }
});

test("retrying an unreadable publication page retains its original cursor and updates existing cards without appending duplicates", async () => {
  const { proxy, snapshot, subscribe } = await import("valtio/vanilla"),
    { default: Page } = await vite.ssrLoadModule(
      "/src/react-app/pages/CharacterWizardPage.jsx",
    ),
    { directory } = await vite.ssrLoadModule("/src/v3/providers/real.ts");
  const original = directory.publication,
    requests = [];
  let fail = true;
  directory.publication = async (id, version) => {
    if (id === "world-two" && fail) {
      fail = false;
      throw Error("publication offline");
    }
    return {
      id,
      version,
      kind: "world",
      name: id,
      body: { background: "精确发布背景" },
      immutable: true,
    };
  };
  const state = proxy({
      session: { actorId: "creation-page-owner" },
      characters: [],
      errors: {},
    }),
    w = {
      state,
      subscribe: (fn) => subscribe(state, fn),
      getSnapshot: () => snapshot(state),
      day: () => "2026-10-28",
      refreshDirectory: async () => {},
      services: {
        configurationClient: { read: async () => ({ bindings: [] }) },
      },
      client: {
        listActorGroups: async () => [],
        worldCatalog: async () => ({ items: [] }),
        listStudioResources: async (query) => {
          requests.push({ ...query });
          if (query.kind !== "world") return { items: [], next_cursor: "" };
          return query.cursor === "second"
            ? {
                items: [
                  {
                    id: "world-two",
                    kind: "world",
                    name: "第二个世界",
                    published_version: "version:2",
                  },
                ],
                next_cursor: "",
              }
            : {
                items: [
                  {
                    id: "world-one",
                    kind: "world",
                    name: "第一个世界",
                    published_version: "version:1",
                  },
                ],
                next_cursor: "second",
              };
        },
      },
    };
  localStorage.clear();
  const v = await mountReact(vite, Page, { workspace: w });
  try {
    await act(async () =>
      [...v.host.querySelectorAll("[role=tab]")]
        .find((n) => n.textContent.trim() === "已发布的世界底稿")
        .dispatchEvent(
          new window.MouseEvent("mousedown", {
            bubbles: true,
            button: 0,
            ctrlKey: false,
          }),
        ),
    );
    await v.flush();
    await v.click(button(v.host, "读取更多世界"));
    assert.equal(v.host.querySelectorAll(".creation-world-card").length, 2);
    assert.match(v.host.textContent, /精确发布定义暂未读到/);
    await v.click(button(v.host, "重新读取世界"));
    assert.equal(v.host.querySelectorAll(".creation-world-card").length, 2);
    assert.equal(
      [...v.host.querySelectorAll(".creation-world-card")].find((n) =>
        n.textContent.includes("第二个世界"),
      ).disabled,
      false,
    );
    assert.deepEqual(
      requests.filter((r) => r.kind === "world").map((r) => r.cursor),
      ["", "second", "second"],
    );
  } finally {
    await v.dispose();
    directory.publication = original;
  }
});

test('r21 creation reading labels aliases and unknown values, orders appearance, and keeps complete relations and motives without altering the document', async () => {
  const f = await preview();
  try {
    const definition = f.c.previewDefinition();
    definition.core_persona.life_profile.appearance = {body_type:'匀称', description:'外貌完整说明', eyes:'brown', hair:'深棕色中长发', extra_detail:false};
    definition.core_persona.life_profile.social_background = {family:'与家人保持稳定的联系', extension_count:0, ...JSON.parse('{"constructor":{"toString":"constructor"}}')};
    definition.initial_relationships = [{target_actor_id:'owner',role:'初识的朋友',metrics:{familiarity:0.1,trust:0.15,custom_metric:0},summary:'完整关系摘要',emotional_association:'适度的好奇',extra_note:null}];
    definition.initial_goals = [{description:'完整初始目标', importance:0, unknown_goal:false}];
    definition.initial_intentions = [{action:'完整初始意图', goal_index:0, custom_intention:'保留的补充'}];
    f.c.updatePreviewDefinition(definition);
    const before = JSON.stringify(f.c.previewDefinition());
    await selectTab(f,'生活设定');
    const appearance=[...f.v.host.querySelectorAll('.creation-structured')].find(n=>n.querySelector('h2')?.textContent==='外貌印象');
    assert.deepEqual([...appearance.querySelectorAll('dt')].map(n=>n.textContent),['说明','发型','眼睛 / 瞳色','体态','补充设定']);
    assert.match(appearance.textContent,/棕色/);assert.match(appearance.textContent,/否/);
    assert.match(f.v.host.textContent,/家庭背景与家人保持稳定的联系/);
    assert.doesNotMatch(f.v.host.textContent,/extra_detail|extension_count/);
    await selectTab(f,'人格与关系');
    const relation=[...f.v.host.querySelectorAll('.creation-structured')].find(n=>n.querySelector('h2')?.textContent==='初始关系');
    assert.match(relation.textContent,/关系对象你关系初识的朋友初始指标熟悉程度10%信任15%/);
    assert.match(relation.textContent,/关系摘要完整关系摘要情感联想适度的好奇/);
    assert.doesNotMatch(relation.textContent,/owner|target_actor_id|custom_metric|extra_note|\{"/);
    assert.match(f.v.host.textContent,/完整初始目标/);assert.match(f.v.host.textContent,/完整初始意图/);
    assert.equal(JSON.stringify(f.c.previewDefinition()),before);
    await f.v.click(f.v.host.querySelector('[role=checkbox]'));
    assert.equal(f.c.draft.includeGoals,false);
    assert.deepEqual(f.c.previewDefinition().initial_relationships,definition.initial_relationships);
    assert.deepEqual(f.c.previewDefinition().initial_goals,definition.initial_goals);
  } finally {await f.v.dispose();f.c.dispose();}
});

test('r21 personality mode edits use the original preview update and preserve nullable unknown nested siblings', async () => {
  const f=await preview();
  try {
    const definition=f.c.previewDefinition();
    definition.core_persona.personality_system={mode:'single',profiles:[],custom_mode:null,switching:{unknown_policy:false,nested:{zero:0}}};
    f.c.updatePreviewDefinition(definition);
    await selectTab(f,'人格与关系');
    const combo=f.v.host.querySelector('[role=combobox][aria-label="人格模式"]');
    assert.match(combo.textContent,/单一人格/);
    assert.ok([...f.v.host.querySelectorAll('.creation-structured')].some(n=>n.querySelector('h2')?.textContent==='人格档案与切换规则'));
    await f.v.click(combo);await f.v.click([...document.querySelectorAll('[role=option]')].find(n=>n.textContent==='多重人格'));
    const after=f.c.previewDefinition();
    assert.equal(after.core_persona.personality_system.mode,'multiple');
    assert.deepEqual(after.core_persona.personality_system,{...definition.core_persona.personality_system,mode:'multiple'});
    assert.deepEqual(after.initial_relationships,definition.initial_relationships);
  } finally {await f.v.dispose();f.c.dispose();}
});
