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
  ),
  { profileFields, profileChanges } = await vite.ssrLoadModule(
    "/src/approved-ui/profile-fields.mjs",
  ),
  { createCreationController } = await vite.ssrLoadModule("/src/state/creation-controller.mjs");
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
test("model array recovery is visible and preserved original values can be reviewed", async () => {
  const f = await preview();
  try {
    const definition = f.c.previewDefinition();
    definition.extensions = {
      initialization_warnings: [
        {path:"core_persona.life_profile.relationship_seeds[0]", message:"模型返回的列表项格式不正确，原值已保留。"},
        {path:"core_persona.life_profile.life_habits", message:"原值已保留。"},
        {path:"initial_goals", message:"原值已保留。"},
        {path:"initial_intentions", message:"原值已保留。"},
        {path:"core_persona.identity.core_values", message:"原值已保留。"},
      ],
      recovered_fields: {"core_persona.life_profile.relationship_seeds[0]":"独居 <script>不执行</script>",
        "core_persona.life_profile.life_habits":"long", "initial_goals":[], "initial_intentions":{}, "core_persona.identity.core_values":false},
    };
    f.c.updatePreviewDefinition(definition);
    f.m.previewFields = f.c.previewFields.value;
    await f.v.update({model:f.m});
    const notice = f.v.host.querySelector('[data-feedback-kind="warning"]');
    assert.ok(notice);
    assert.match(notice.textContent, /部分资料的格式需要核对/);
    await f.v.click(button(notice, "查看需要核对的内容（5）"));
    assert.match(notice.textContent, /初始关系线索 · 第 1 项/);
    assert.match(notice.textContent, /独居 <script>不执行<\/script>/);
    assert.match(notice.textContent, /文字：long/);
    assert.doesNotMatch(notice.textContent, /长发/);
    assert.match(notice.textContent, /空列表/);
    assert.match(notice.textContent, /空对象/);
    assert.match(notice.textContent, /布尔值：false/);
    assert.equal(notice.querySelector('script'), null);
    assert.equal(f.c.previewDefinition().extensions.recovered_fields["core_persona.life_profile.relationship_seeds[0]"], "独居 <script>不执行</script>");
  } finally {await f.v.dispose();f.c.dispose();}
});
test("policy fields follow their actual types without rewriting legacy analysis", () => {
  const core = {identity:{}, behavioral_policy:{punctuation_style:2, humor_style:3, response_delay:"随当前活动自然回复", emoji_frequency:0.35}, life_profile:{}};
  const fields = profileFields(core), find = key => fields.find(field => field.path.at(-1) === key);
  assert.equal(find("punctuation_style").kind, "text");
  assert.equal(find("punctuation_style").value, "2");
  assert.equal(find("humor_style").kind, "text");
  assert.equal(find("response_delay").kind, "text");
  assert.equal(find("emoji_frequency").kind, "number");
  assert.equal(find("emoji_frequency").max, 1);
  assert.deepEqual(profileChanges(core, fields), {}, "untouched legacy values remain intact");
  find("punctuation_style").value = "自然使用标点";
  assert.deepEqual(profileChanges(core, fields), {behavioral_policy:{punctuation_style:"自然使用标点"}});
  const delay = profileFields({identity:{},behavioral_policy:{response_delay:45},life_profile:{}}).find(field => field.path.at(-1) === "response_delay");
  assert.equal(delay.kind, "number");assert.equal(delay.max, undefined);
  const legacy = profileFields({identity:{},behavioral_policy:{humor_style:false,message_length:0},life_profile:{}});
  assert.equal(legacy.find(field => field.path.at(-1) === "humor_style").kind,"text");
  assert.ok(legacy.find(field => field.path.at(-1) === "message_length").options.some(option => option.value === 0));
});

test("expression preview explains real ranges, keeps free text and uses Select for message length", async () => {
  const f = await preview();
  try {
    const definition = f.c.previewDefinition();
    definition.core_persona.personality.openness = 0.25;
    definition.core_persona.behavioral_policy = {emoji_frequency:0.3, punctuation_style:2, humor_style:"温柔的冷幽默", response_delay:45, message_length:"medium", response_style:"自然随和"};
    f.c.updatePreviewDefinition(definition);
    f.m.previewFields = f.c.previewFields.value;
    await f.v.update({model:f.m});
    await selectTab(f,"性格与表达");
    const root=f.v.host;
    assert.ok(root.querySelector('[role=slider][aria-label="开放性"]'));
    assert.match(root.textContent,/0 · 偏稳妥熟悉/);assert.match(root.textContent,/1 · 愿意探索新事物/);
    assert.ok(root.querySelector('[aria-label="了解开放性的数值含义"]'));
    assert.ok(root.querySelector('[role=slider][aria-label="表情频率"]'));
    assert.match(root.textContent,/0 · 很少使用表情/);assert.match(root.textContent,/1 · 更常使用表情/);
    assert.equal(root.querySelector('[aria-label="标点习惯"]').type,'text');
    assert.equal(root.querySelector('[aria-label="标点习惯"]').value,'2');
    assert.equal(root.querySelector('[aria-label="幽默方式"]').type,'text');
    assert.equal(root.querySelector('[aria-label="回应节奏"]').type,'number');
    assert.equal(root.querySelector('[aria-label="回应节奏"]').value,'45');
    assert.match(root.textContent,/单位：秒/);
    for(const label of ['回复风格','消息长短','分歧处理','拒绝方式']) assert.equal(root.querySelector(`[aria-label="${label}"]`).dataset.slot,'select-trigger');
    const emoji = f.c.previewFields.value.find(field=>field.path.at(-1)==='emoji_frequency');
    emoji.value=0.73;
    assert.equal(f.c.previewDefinition().core_persona.behavioral_policy.emoji_frequency,0.73);
    assert.equal(f.c.previewDefinition().core_persona.behavioral_policy.punctuation_style,2);
  } finally {await f.v.dispose();f.c.dispose();}
});
test("opening and applying a nullable structured extension preserves null without manufacturing an empty collection", async () => {
  const f = await preview();
  try {
    await selectTab(f, "生活设定");
    const card = [...f.v.host.querySelectorAll(".creation-structured")].find(
      (n) => n.querySelector("h2")?.textContent === "其他设定 1",
    );
    await f.v.click(button(card, "编辑内容"));
    assert.equal(dialog(), undefined, "theme editing is inline");
    assert.match(card.textContent, /尚未填写/);
    await f.v.click(button(card, "应用修改"));
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
    await f.v.input(card.querySelector("textarea"), "取消的输入");
    await f.v.click(button(card, "取消"));
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

test("preparation actions use a keyboard menu and preserve actions and dialog focus", async () => {
  const f = await preview();
  let refreshes = 0;
  try {
    f.c.draft.activatedId = "created /角色";
    f.c.refreshProgress = async () => { refreshes++; };
    const model = {
      ...f.m,
      step: "progress",
      currentStep: 3,
      progress: { complete: false, stages: [] },
    };
    await f.v.update({ model });
    const actions = f.v.host.querySelector(".creation-progress-actions");
    const trigger = button(actions, "更多准备操作");
    const openMenu = async () => {
      await act(async () => trigger.dispatchEvent(new window.KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true })));
      await f.v.flush();
      return document.querySelector("[role=menu]");
    };
    assert.equal(actions.querySelector('a[href="#/library"]').textContent, "保留进度，返回角色库");
    assert.equal(document.querySelector("[role=menu]"), null);
    let menu = await openMenu();
    assert.ok(menu);
    assert.deepEqual([...menu.querySelectorAll("[role=menuitem]")].map(item => item.textContent), ["重新读取进度", "角色详情", "开启另一段相识"]);
    assert.equal(menu.querySelector('a[role=menuitem]').getAttribute("href"), "#/c/created%20%2F%E8%A7%92%E8%89%B2/profile");
    await f.v.click(menu.querySelector("[role=menuitem]"));
    assert.equal(refreshes, 1);
    assert.equal(document.querySelector("[role=menu]"), null);

    menu = await openMenu();
    await f.v.click([...menu.querySelectorAll("[role=menuitem]")].find(item => item.textContent === "开启另一段相识"));
    assert.match(dialog().textContent, /开启另一段新的相识/);
    assert.ok(dialog().contains(document.activeElement));
    await f.v.click(button(dialog(), "取消"));
    assert.equal(dialog(), undefined);
    assert.equal(document.activeElement, trigger);

    const snapshot = button(actions, "查看创建资料");
    await f.v.click(snapshot);
    assert.match(dialog().textContent, /这次相遇的起点/);
    await act(async () => document.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Escape", bubbles: true })));
    await f.v.flush();
    assert.equal(dialog(), undefined);
    assert.equal(document.activeElement, snapshot);
  } finally {
    await f.v.dispose();
    f.c.dispose();
  }
});

test("preparation menu retains disabled and hidden action conditions", async () => {
  const f = await preview();
  try {
    const model = { ...f.m, step: "progress", currentStep: 3, progress: { complete: false, stages: [] } };
    const openMenu = async () => {
      const trigger = button(f.v.host, "更多准备操作");
      await act(async () => trigger.dispatchEvent(new window.MouseEvent("pointerdown", { bubbles: true, button: 0 })));
      await f.v.flush();
      return document.querySelector("[role=menu]");
    };
    const closeMenu = async () => {
      await act(async () => document.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Escape", bubbles: true })));
      await f.v.flush();
    };
    await f.v.update({ model });
    let menu = await openMenu();
    assert.deepEqual([...menu.querySelectorAll("[role=menuitem]")].map(item => item.textContent), ["重新读取进度"]);
    assert.equal(menu.querySelector("[role=menuitem]").getAttribute("aria-disabled"), "true");
    await closeMenu();

    f.c.draft.activatedId = "created";
    await f.v.update({ model: { ...model, busy: true } });
    menu = await openMenu();
    assert.equal(menu.querySelector("[role=menuitem]").getAttribute("aria-disabled"), "true");
    assert.equal([...menu.querySelectorAll("[role=menuitem]")].at(-1).getAttribute("aria-disabled"), "true");
    await closeMenu();

    f.c.archived = true;
    await f.v.update({ model: { ...model, busy: false } });
    menu = await openMenu();
    assert.deepEqual([...menu.querySelectorAll("[role=menuitem]")].map(item => item.textContent), ["重新读取进度", "角色详情"]);
    await closeMenu();

    f.c.archived = false;
    await f.v.update({ model: { ...model, pending: { value: {} } } });
    menu = await openMenu();
    assert.deepEqual([...menu.querySelectorAll("[role=menuitem]")].map(item => item.textContent), ["重新读取进度", "角色详情"]);
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
      ).querySelector('[role=radio]').disabled,
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
    assert.deepEqual([...appearance.querySelectorAll('dt')].map(n=>n.textContent),['说明','发型','眼睛 / 瞳色','体态','其他细节']);
    assert.match(appearance.textContent,/棕色/);assert.match(appearance.textContent,/否/);
    assert.match(f.v.host.textContent,/家庭背景与家人保持稳定的联系/);
    assert.doesNotMatch(f.v.host.textContent,/extra_detail|extension_count/);
    await selectTab(f,'人格与关系');
    const relation=[...f.v.host.querySelectorAll('.creation-structured')].find(n=>n.querySelector('h2')?.textContent==='初始关系');
    assert.match(relation.textContent,/关系初识的朋友初始指标熟悉程度10%信任15%/);
    assert.doesNotMatch(relation.textContent,/关系对象/,'unproven owner defaults stay in the complete source until reviewed');
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

test('initial wardrobe preview reads and edits the canonical appearance items without changing other facts', async () => {
  const f = await preview();
  try {
    const definition = f.c.previewDefinition();
    definition.core_persona.life_profile.appearance = {
      description: '黑色长发',
      wardrobe_items: [{category:'upper', slot:'shirt', description:'白衬衫', ownership:'owned', available:true, currently_worn:false}],
    };
    definition.core_persona.life_profile.social_background = {family:'教师家庭'};
    f.c.updatePreviewDefinition(definition);
    await selectTab(f, '生活设定');
    const wardrobe = [...f.v.host.querySelectorAll('.creation-structured')].find(n => n.querySelector('h2')?.textContent === '初始衣物');
    assert.match(wardrobe.textContent, /白衬衫/);
    await f.v.click(button(wardrobe, '编辑内容'));
    assert.equal(dialog(), undefined);
    assert.match(wardrobe.querySelector('[aria-label="是否正在穿着"]').textContent, /否/);
    await f.v.input(wardrobe.querySelector('[aria-label="说明"]'), '白色亚麻衬衫');
    await f.v.click(button(wardrobe, '应用修改'));
    const after = f.c.previewDefinition().core_persona.life_profile;
    assert.equal(after.appearance.wardrobe_items[0].description, '白色亚麻衬衫');
    assert.equal(after.appearance.wardrobe_items[0].currently_worn, false);
    assert.equal(after.appearance.description, '黑色长发');
    assert.deepEqual(after.social_background, {family:'教师家庭'});
    assert.deepEqual(after.wardrobe_items, []);
  } finally {await f.v.dispose(); f.c.dispose();}
});

test('missing initial wardrobe opens an empty array without adding it when cancelled', async () => {
  const f = await preview();
  try {
    const definition = f.c.previewDefinition();
    delete definition.core_persona.life_profile.wardrobe_items;
    definition.core_persona.life_profile.appearance = {description:'黑色长发'};
    f.c.updatePreviewDefinition(definition);
    const before = JSON.stringify(f.c.previewDefinition());
    await selectTab(f, '生活设定');
    const wardrobe = [...f.v.host.querySelectorAll('.creation-structured')].find(n => n.querySelector('h2')?.textContent === '初始衣物');
    assert.match(wardrobe.textContent, /尚未记录/);
    await f.v.click(button(wardrobe, '编辑内容'));
    assert.match(wardrobe.textContent, /还没有内容/);
    assert.ok(button(wardrobe, '添加衣物'));
    await f.v.click(button(wardrobe, '取消'));
    assert.equal(JSON.stringify(f.c.previewDefinition()), before);
  } finally {await f.v.dispose(); f.c.dispose();}
});

test('inline life and relationship forms add records while preserving untouched nested extensions', async () => {
  const f = await preview();
  try {
    const definition=f.c.previewDefinition();
    definition.core_persona.life_profile.social_background={future:{flag:false,count:0,missing:null}};
    definition.initial_relationships=[];definition.initial_goals=[];definition.initial_intentions=[];
    f.c.updatePreviewDefinition(definition);
    await selectTab(f,'生活设定');
    const card = title => [...f.v.host.querySelectorAll('.creation-structured')].find(node => node.querySelector('h2')?.textContent === title);
    const social=card('社会背景');
    await f.v.click(button(social,'编辑内容'));
    await f.v.click(social.querySelector('[aria-label="补充常用字段"]'));
    await f.v.click([...document.querySelectorAll('[role=option]')].find(node=>node.textContent==='家庭背景'));
    await f.v.input(social.querySelector('[aria-label="家庭背景"]'),'与家人保持联系');
    await f.v.click(button(social,'应用修改'));
    assert.deepEqual(f.c.previewDefinition().core_persona.life_profile.social_background,{future:{flag:false,count:0,missing:null},family:'与家人保持联系'});
    await selectTab(f,'人格与关系');
    const relation=card('初始关系');
    await f.v.click(button(relation,'编辑内容'));await f.v.click(button(relation,'添加与你的关系'));
    assert.match(relation.querySelector('[aria-label="关系对象"]').textContent,/你/);
    await f.v.input(relation.querySelector('[aria-label="关系"]'),'朋友');
    await f.v.input(relation.querySelector('[aria-label="关系摘要"]'),'一起散步');
    await f.v.click(button(relation,'应用修改'));
    assert.deepEqual(f.c.previewDefinition().initial_relationships,[{target_actor_id:'actor_user',role:'朋友',summary:'一起散步'}]);
    const goals=card('初始目标');
    await f.v.click(button(goals,'编辑内容'));await f.v.click(button(goals,'添加目标'));
    await f.v.input(goals.querySelector('[aria-label="说明"]'),'学习摄影');await f.v.click(button(goals,'应用修改'));
    assert.deepEqual(f.c.previewDefinition().initial_goals,[{description:'学习摄影'}]);
    const intentions=card('初始意图');
    await f.v.click(button(intentions,'编辑内容'));await f.v.click(button(intentions,'添加意图'));
    await f.v.input(intentions.querySelector('[aria-label="意图"]'),'周末去拍照');await f.v.click(button(intentions,'应用修改'));
    assert.deepEqual(f.c.previewDefinition().initial_intentions,[{action:'周末去拍照'}]);
  } finally {await f.v.dispose();f.c.dispose();}
});

test('unfinished inline edits survive a fresh controller and prevent activation until resolved', async () => {
  const f=await preview();
  let restored, second;
  try {
    await selectTab(f,'生活设定');
    const habits=[...f.v.host.querySelectorAll('.creation-structured')].find(node=>node.querySelector('h2')?.textContent==='生活习惯');
    await f.v.click(button(habits,'编辑内容'));
    await f.v.input(habits.querySelector('textarea'),'刷新后仍保留的习惯');
    assert.equal(button(f.v.host,'确认，准备相遇').disabled,true);
    assert.match(f.v.host.textContent,/分组修改已保留/);
    const saved=JSON.parse(f.saved.getItem('fluctlight-creation:owner:draft'));
    assert.equal(saved.structureDraft.value,'刷新后仍保留的习惯');
    await f.v.dispose();
    restored=createCreationController({owner:'owner',storage:f.saved,client:f.client,directory:f.directory,ownerCurrent:()=>true,label:key=>key,
      analysisToActivation:(analysis,name,requestId,timezone)=>({requestId,name,corePersona:analysis.core_persona,initializationTimezone:timezone})});
    const model={...f.m,c:restored,draft:restored.draft,previewFields:restored.previewFields.value};
    second=await mountReact(vite,View,{props:{model}});
    const resumed=[...second.host.querySelectorAll('.creation-structured')].find(node=>node.querySelector('h2')?.textContent==='生活习惯');
    assert.equal(resumed.querySelector('textarea').value,'刷新后仍保留的习惯');
    assert.equal(button(second.host,'确认，准备相遇').disabled,true);
    await second.click(button(resumed,'应用修改'));
    assert.equal(restored.previewDefinition().core_persona.life_profile.life_habits,'刷新后仍保留的习惯');
    assert.equal(restored.draft.structureDraft,undefined);
    assert.equal(button(second.host,'确认，准备相遇').disabled,false);
  } finally {if(second)await second.dispose();else await f.v.dispose();restored?.dispose();f.c.dispose();}
});
