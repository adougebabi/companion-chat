import test from "node:test";
import assert from "node:assert/strict";
import { createSSRApp, h, ref, computed, createRenderer, nextTick } from "vue";
import { renderToString } from "vue/server-renderer";
import { configurationPlan } from "../src/approved-ui/configuration-plan.mjs";
import {
  acknowledgeConfiguration,
  clearAcknowledgedCredentials,
} from "../src/approved-ui/configuration-ack.mjs";
import { stagedCommand } from "../src/approved-ui/staged-command.mjs";
import { approvedHref, approvedRoute } from "../src/approved-ui/routing.mjs";
import {
  ActionButton,
  Surface,
  approvedButtonVNode,
  approvedSurfaceVNode,
} from "../src/approved-ui/shared/primitives.mjs";
import { renderBrowsePage } from "../src/approved-ui/renderers/BrowsePage.mjs";
import { renderFilterChips } from "../src/approved-ui/renderers/FilterChips.mjs";
const storage = () => {
  const values = new Map();
  return {
    values,
    getItem: (k) => values.get(k) ?? null,
    setItem: (k, v) => values.set(k, String(v)),
    removeItem: (k) => values.delete(k),
  };
};
const form = () => ({
  catalog: {
    versions: { "setting:test": 3, "endpoint:remote": 8 },
    effective_values: { test: { enabled: false } },
    endpoints: [],
    bindings: [],
  },
  settings: [
    {
      fields: [
        {
          label: "开关",
          _settingKey: "test",
          _key: "enabled",
          type: "checkbox",
          value: true,
          _initial: false,
        },
      ],
    },
  ],
  endpoints: [],
  bindings: [],
  quiet: {
    enabled: false,
    start: "",
    end: "",
    days: ["周日", "周一", "周二", "周三", "周四", "周五", "周六"],
    behavior: "暂停主动联系",
    character: "全部角色",
  },
  characters: [],
});
test("configuration validates the entire form before writing and maps temporary endpoint IDs", () => {
  const input = form();
  input.endpoints = [
    {
      id: "new-1",
      name: "new-server",
      protocol: "openai-compatible",
      url: "https://model.test/v1",
      secret: "",
      clearSecret: false,
    },
  ];
  input.bindings = [
    {
      name: "通用",
      role: "generic_llm",
      endpoint: "new-1",
      model: "model",
      tokens: 1000,
      timeout: 60,
      window: 8192,
      safetyMargin: 512,
    },
  ];
  const plan = configurationPlan(input);
  assert.equal(plan[0].body.endpointId, "new-server");
  assert.equal(plan[1].body.endpointId, "new-server");
  assert.equal(plan[1].body.maxInputTokens, undefined);
  assert.equal(plan[0].formId, "new-1");
  input.settings[0].fields.push({
    label: "配置",
    _settingKey: "test",
    _key: "json",
    type: "json",
    value: "{broken",
    _definition: {},
  });
  assert.throws(() => configurationPlan(input), /有效 JSON/);
  input.settings[0].fields.pop();
  input.endpoints[0].url = "https://user:secret@model.test/v1";
  assert.throws(() => configurationPlan(input), /嵌入凭据/);
});
test("acknowledgement advances only receipt versions and preserves newer input and revert changes", () => {
  const input = form(),
    versions = { ...input.catalog.versions },
    operation = configurationPlan(input)[0];
  input.settings[0].fields[0].value = false;
  acknowledgeConfiguration({ ...input, versions }, operation, {
    applied_versions: { "setting:test": 4 },
  });
  assert.equal(versions["endpoint:remote"], 8);
  assert.equal(versions["setting:test"], 4);
  assert.equal(input.settings[0].fields[0].value, false);
  assert.equal(input.settings[0].fields[0]._initial, true);
  assert.equal(input.catalog.effective_values.test.enabled, true);
  assert.equal(configurationPlan(input)[0].body.values.test.enabled, false);
});
test("endpoint acknowledgements preserve edits made in flight and rebind temporary IDs", () => {
  const input = form();
  input.endpoints = [
    {
      id: "temp",
      name: "new-server",
      protocol: "openai-compatible",
      url: "https://model.test/v1",
      secret: "",
      clearSecret: false,
    },
  ];
  input.bindings = [{ role: "generic_llm", endpoint: "temp", model: "draft" }];
  const operation = {
    operation: "endpoint",
    formId: "temp",
    body: {
      endpointId: "new-server",
      kind: "openai-compatible",
      baseUrl: "https://model.test/v1/",
      secretPurpose: "provider:new-server",
    },
  };
  input.endpoints[0].name = "newer-name";
  input.endpoints[0].url = "https://new.test/v1";
  acknowledgeConfiguration({ ...input, versions: {} }, operation, {
    applied_versions: { "endpoint:new-server": 1 },
  });
  assert.equal(input.endpoints[0].name, "newer-name");
  assert.equal(input.endpoints[0].url, "https://new.test/v1");
  assert.equal(input.endpoints[0]._source.base_url, "https://model.test/v1");
  assert.equal(input.bindings[0].endpoint, "new-server");
});
test("world stages recover immutable apply body and key after lost response and refresh", async () => {
  const saved = storage(),
    calls = [];
  let previews = 0;
  const create = () => {
    previews++;
    return { runtimeWorldId: "world-A", expectedRuntimeRevision: 4 };
  };
  const execute = async (body, key) => {
    calls.push({ body, key });
    if (calls.length === 1) throw new Error("lost response");
    return { revision: 5 };
  };
  await assert.rejects(
    stagedCommand({ storage: saved, key: "operation" }).stage(
      "apply",
      create,
      execute,
    ),
    /lost response/,
  );
  const next = stagedCommand({ storage: saved, key: "operation" });
  await next.stage(
    "apply",
    () => ({ runtimeWorldId: "world-B", expectedRuntimeRevision: 9 }),
    execute,
  );
  assert.equal(previews, 1);
  assert.deepEqual(calls[0], calls[1]);
  assert.equal(
    JSON.stringify([...saved.values.values()]).includes('"revision":5'),
    false,
  );
  next.finish();
  assert.equal(saved.values.size, 0);
});
test("canonical routes and legacy deep links preserve their targets", () => {
  for (const page of [
    "聊天",
    "此刻",
    "日程",
    "目标",
    "衣柜",
    "日记",
    "记忆",
    "世界",
    "人物关系",
    "媒体",
  ]) {
    const route = approvedRoute(approvedHref(page, "real-role"));
    assert.equal(route.page, page);
    assert.equal(route.fid, "real-role");
  }
  assert.equal(
    approvedRoute("#/c/real-role/growth/memory/memory-A").legacy,
    true,
  );
  assert.equal(approvedRoute("#/studio?legacy=1").legacy, true);
  assert.equal(approvedRoute("#/auth").legacy, true);
});
test("shared primitives preserve native DOM and native button types", async () => {
  const html = await renderToString(
    createSSRApp({
      render: () =>
        h(Surface, { as: "section", class: "card" }, () =>
          h(
            ActionButton,
            { class: "button", type: "submit", disabled: true },
            () => "<content>",
          ),
        ),
    }),
  );
  assert.equal(
    html,
    '<section class="card"><button class="button" type="submit" disabled>&lt;content&gt;</button></section>',
  );
});
test("extracted render primitives update classes, children and listeners after reactive changes", async () => {
  const create = (type) => ({
      type,
      children: [],
      props: {},
      parent: null,
      text: "",
    }),
    host = {
      createElement: create,
      createText: (text) => ({ ...create("#text"), text }),
      createComment: (text) => ({ ...create("#comment"), text }),
      setText: (n, t) => (n.text = t),
      setElementText: (n, t) => {
        n.text = t;
        n.children = [];
      },
      parentNode: (n) => n.parent,
      nextSibling: (n) => n.parent?.children[n.parent.children.indexOf(n) + 1],
      patchProp: (n, k, _old, v) => (n.props[k] = v),
      insert(n, p, anchor) {
        if (n.parent) n.parent.children.splice(n.parent.children.indexOf(n), 1);
        n.parent = p;
        const i = anchor ? p.children.indexOf(anchor) : -1;
        if (i < 0) p.children.push(n);
        else p.children.splice(i, 0, n);
      },
      remove(n) {
        n.parent?.children.splice(n.parent.children.indexOf(n), 1);
      },
    };
  const root = create("root"),
    value = ref("first"),
    calls = [],
    renderer = createRenderer(host);
  renderer
    .createApp({
      render: () =>
        approvedSurfaceVNode("section", { class: value.value }, [
          approvedButtonVNode(
            { onClick: () => calls.push(value.value) },
            value.value,
          ),
        ]),
    })
    .mount(root);
  value.value = "second";
  await nextTick();
  assert.equal(root.children[0].props.class, "second");
  const button = root.children[0].children[0];
  const renderedText = (node) =>
    node.text + node.children.map(renderedText).join("");
  assert.equal(renderedText(button), "second");
  button.props.onClick();
  assert.deepEqual(calls, ["second"]);
});
test("library renderer unwraps computed filter groups and renders real directory records", async () => {
  const Filter = {
    props: ["search", "filters", "groups"],
    setup: (props) =>
      renderFilterChips({
        props,
        n: () => {},
        l: computed(() => false),
        i: () => {},
        r: () => {},
      }),
  };
  const bindings = {
    props: { page: "角色库" },
    FilterChips: Filter,
    f: ref(""),
    c: ref({}),
    te: computed(() => [{ key: "tag", label: "标签", options: [] }]),
    le: ref([
      {
        id: "actual-role",
        name: "真实角色",
        initial: "角",
        job: "工程师",
        tags: [],
        color: "#eee",
      },
    ]),
    d: ref(null),
    selectedId: ref("actual-role"),
    createCharacter: () => {},
  };
  const html = await renderToString(
    createSSRApp({ setup: () => renderBrowsePage(bindings) }),
  );
  assert.match(html, /真实角色/);
  assert.match(html, /工程师/);
  assert.match(html, /标签/);
});

test("extracted app replaces page branches repeatedly without stale block references", async () => {
  const { renderApp } = await import("../src/approved-ui/renderers/App.mjs");
  const create = (type) => ({
      type,
      children: [],
      props: {},
      parent: null,
      text: "",
    }),
    host = {
      createElement: create,
      createText: (text) => ({ ...create("#text"), text }),
      createComment: (text) => ({ ...create("#comment"), text }),
      setText: (n, t) => (n.text = t),
      setElementText: (n, t) => {
        n.text = t;
        n.children = [];
      },
      parentNode: (n) => n.parent,
      nextSibling: (n) => n.parent?.children[n.parent.children.indexOf(n) + 1],
      patchProp: (n, k, _old, v) => (n.props[k] = v),
      insert(n, p, anchor) {
        assert.ok(p, "every patched node retains its actual parent");
        if (n.parent) n.parent.children.splice(n.parent.children.indexOf(n), 1);
        n.parent = p;
        const i = anchor ? p.children.indexOf(anchor) : -1;
        if (i < 0) p.children.push(n);
        else p.children.splice(i, 0, n);
      },
      remove(n) {
        n.parent?.children.splice(n.parent.children.indexOf(n), 1);
      },
    };
  const page = ref("衣柜"),
    character = ref({ id: "A", name: "A", initial: "A", color: "#fff" }),
    icon = () => h("svg"),
    surface = (name) => ({
      render: () => h("section", { "data-page": name }, name),
    }),
    bindings = {
      d: (value) => (page.value = value),
      u: ref(false),
      character,
      characters: ref([character.value]),
      selectedId: ref("A"),
      r: page,
      a: ref(true),
      t: ["此刻", "日程"],
      s: [["世界", icon]],
      n: [["角色库", icon]],
      c: ref(null),
      f: ref(true),
      isDemo: false,
      simulateFailure: ref(false),
      toast: ref(null),
      outfitTab: ref("为她搭配"),
      ChatPage: surface("actual-chat"),
      WardrobePage: surface("actual-wardrobe"),
      WorldPage: surface("actual-world"),
      LifePage: surface("actual-life"),
      ManagementPage: surface("actual-management"),
      BrowsePage: surface("actual-browse"),
    };
  const root = create("root"),
    renderer = createRenderer(host),
    errors = [];
  const app = renderer.createApp({ setup: () => renderApp(bindings) });
  app.config.errorHandler = (e) => errors.push(e);
  app.mount(root);
  const text = (node) => node.text + node.children.map(text).join("");
  for (const [next, label] of [
    ["聊天", "actual-chat"],
    ["此刻", "actual-life"],
    ["角色库", "actual-browse"],
    ["管理中心", "actual-management"],
    ["世界", "actual-world"],
    ["衣柜", "actual-wardrobe"],
  ]) {
    page.value = next;
    await nextTick();
    assert.match(text(root), new RegExp(label));
    assert.equal((text(root).match(/actual-/g) || []).length, 1);
  }
  assert.deepEqual(errors, []);
  app.unmount();
});

test("unconfigured optional form values do not turn a sibling save into empty configuration writes", () => {
  const input = form();
  input.settings.push({
    fields: [
      {
        label: "模型版本",
        _settingKey: "kev",
        _key: "model_revision",
        type: "text",
        value: "",
        _initial: undefined,
      },
      {
        label: "工作流",
        _settingKey: "media.comfyui",
        _key: "workflow",
        type: "json",
        value: "",
        _initial: undefined,
        _definition: {},
      },
    ],
  });
  const plan = configurationPlan(input);
  assert.deepEqual(Object.keys(plan[0].body.values), ["test"]);
});
test("character quiet policy keeps its own revision and validates weekdays and clocks before writes", () => {
  const input = form();
  input.characters = [{ id: "role-B", name: "B" }];
  input.quiet = {
    ...input.quiet,
    character: "B",
    enabled: true,
    start: "22:00",
    end: "08:00",
    days: ["周一", "周五"],
    _policy: { fluctlight_id: "role-B", revision: 8, quiet_hours: {} },
  };
  const operation = configurationPlan(input).find(
    (w) => w.operation === "autonomy",
  );
  assert.equal(operation.fid, "role-B");
  assert.equal(operation.body.expectedRevision, 8);
  assert.deepEqual(operation.body.quietHours.days, [1, 5]);
  input.quiet.start = "99:99";
  assert.throws(() => configurationPlan(input), /安静日期/);
});

test("secret acknowledgements clear only the acknowledged purpose and retain sibling/newer drafts", () => {
  const endpoints = [
      { id: "a", name: "a", secret: "A", clearSecret: false },
      { id: "b", name: "b", secret: "B", clearSecret: true },
    ],
    submitted = structuredClone(endpoints);
  clearAcknowledgedCredentials(endpoints, submitted, {
    secrets: { "provider:a": "A" },
  });
  assert.equal(endpoints[0].secret, "");
  assert.equal(endpoints[1].secret, "B");
  assert.equal(endpoints[1].clearSecret, true);
  endpoints[1].secret = "newer B";
  clearAcknowledgedCredentials(endpoints, submitted, {
    secrets: { "provider:b": "B" },
  });
  assert.equal(endpoints[1].secret, "newer B");
});
