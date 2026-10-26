import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { act, mountReact, installDOM } from "./helpers/react-dom.mjs";
const environment = installDOM();
const vite = await createServer({
  root: fileURLToPath(new URL("../", import.meta.url)),
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});
test.after(async () => {
  await vite.close();
  environment.restore();
});
const load = (path) => vite.ssrLoadModule("/src/" + path);
const button = (text, root = document) =>
  [...root.querySelectorAll("button")].find(
    (node) => node.textContent.trim() === text,
  );
function storeFacade(state) {
  return {
    state,
    subscribe: (listener) => subscribe(state, listener),
    getSnapshot: () => snapshot(state),
  };
}

test("React settings stay busy through receipt readback and preserve newer model input", async () => {
  const { default: Management } = await load(
    "react-app/pages/ManagementPage.jsx",
  );
  let finishWrite,
    finishRead,
    calls = 0;
  const catalog = {
    endpoints: [
      {
        id: "service",
        base_url: "https://model.example",
        kind: "openai-compatible",
      },
    ],
    bindings: [
      {
        role: "generic_llm",
        endpoint_id: "service",
        model_id: "old",
        timeout_seconds: 120,
      },
    ],
    versions: {},
    effective_values: {},
  };
  const w = {
    ...storeFacade(
      proxy({
        session: { actorId: "owner-react" },
        managementTab: "配置中心",
        characters: [],
        operations: [],
        managementSection: "",
        runtimeCatalog: {},
      }),
    ),
    services: {
      configurationClient: {
        read: async () => structuredClone(catalog),
        models: async () => ({ models: [] }),
        catalog: async () => ({ agents: [] }),
        binding: async () => {
          calls++;
          return new Promise((resolve) => (finishWrite = resolve));
        },
        command: async () => new Promise((resolve) => (finishRead = resolve)),
      },
      diagnosticsClient: { page: async () => ({ items: [] }) },
    },
    notify() {},
    navigatePath() {},
  };
  const view = await mountReact(vite, Management, { workspace: w });
  try {
    const model = view.host.querySelector('input[list="models-generic_llm"]');
    assert.ok(model);
    await view.input(model, "new");
    const form = view.host.querySelector("form");
    await act(async () =>
      form.dispatchEvent(
        new window.Event("submit", { bubbles: true, cancelable: true }),
      ),
    );
    await view.flush();
    const save = () => view.host.querySelector('[data-testid="config-save"]'),
      status = () =>
        view.host.querySelector('[data-testid="config-save-status"]');
    assert.equal(calls, 1);
    assert.equal(save().disabled, true);
    assert.match(status().textContent, /等待服务端确认/);
    await view.input(model, "newer-unsaved");
    await act(async () => finishWrite({}));
    await view.flush();
    assert.equal(save().disabled, true);
    assert.equal(typeof finishRead, "function");
    await act(async () =>
      finishRead({
        status: "applied",
        applied_versions: { "binding:generic_llm": 1 },
      }),
    );
    await view.flush();
    assert.equal(save().disabled, false);
    assert.match(status().textContent, /已保存/);
    assert.equal(model.value, "newer-unsaved");
  } finally {
    await view.dispose();
  }
});

test("React world editor preserves world identity, CAS and saved category on reread", async () => {
  const { createWorkspace } = await load("state/workspace.mjs"),
    store = await load("v3/proto/store.js"),
    services = await load("v3/proto/services.js"),
    { default: World } = await load("react-app/pages/WorldPage.jsx");
  let server = {
      id: "W",
      name: "原世界",
      revision: 1,
      flavor: "日常",
      active: true,
      venueCategories: [],
    },
    writes = 0;
  services.configureDomainService("world", {
    worlds: async () => [structuredClone(server)],
    places: async () => [],
  });
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "world-owner" },
    characters: { A: { id: "A", name: "测试角色" } },
    ui: { charId: "A" },
  });
  await w.loadWorlds();
  const oldUpdate = w.client.updateCatalogWorld,
    oldCreate = w.client.createCatalogWorld;
  w.client.createCatalogWorld = async () =>
    assert.fail("must not recreate an existing world");
  w.client.updateCatalogWorld = async (fid, id, body) => {
    assert.equal(fid, "A");
    assert.equal(id, "W");
    assert.equal(body.expectedRevision, server.revision);
    writes++;
    server = {
      ...server,
      name: body.name,
      revision: server.revision + 1,
      ...(body.venueCategories
        ? { venueCategories: structuredClone(body.venueCategories) }
        : {}),
    };
    return { id: "W", revision: server.revision };
  };
  const view = await mountReact(vite, World, { workspace: w });
  try {
    await view.click(button("进入"));
    await view.click(button("新增场地分类"));
    const input = document.querySelector('[role="dialog"] input');
    assert.ok(input);
    await view.input(input, "商店");
    await act(async () =>
      input
        .closest("form")
        .dispatchEvent(
          new window.Event("submit", { bubbles: true, cancelable: true }),
        ),
    );
    await view.flush();
    assert.equal(writes, 1);
    assert.equal(server.venueCategories[0].name, "商店");
    assert.match(view.host.textContent, /场地 · 商店/);
    await act(async () => {
      w.state.worlds = [];
      await w.loadWorlds();
    });
    await view.flush();
    assert.match(view.host.textContent, /场地 · 商店/);
    await view.click(button("编辑世界信息"));
    const name = document.querySelector('[data-testid="world-name"]');
    assert.ok(name);
    await view.input(name, "更新世界");
    await act(async () =>
      name
        .closest("form")
        .dispatchEvent(
          new window.Event("submit", { bubbles: true, cancelable: true }),
        ),
    );
    await view.flush();
    assert.equal(writes, 2);
    assert.equal(server.name, "更新世界");
    assert.equal(server.venueCategories.length, 1);
  } finally {
    await view.dispose();
    w.client.updateCatalogWorld = oldUpdate;
    w.client.createCatalogWorld = oldCreate;
    w.dispose();
  }
});

test("React composer retains draft and selection during updates and never sends a composing Enter", async () => {
  const { default: Chat } = await load("react-app/pages/ChatPage.tsx"),
    { createConversationController } = await load("state/conversation.ts");
  const store = createConversationController();
  store.fluctlightId = "A";
  store.conversation = { id: "C" };
  store.messages = [];
  let accepted,
    submissions = 0;
  Object.defineProperty(store, "send", {
    configurable: true,
    value: async (text, _retry, _queue, onAccepted) => {
      submissions++;
      accepted = onAccepted;
    },
  });
  const state = proxy({
    session: { actorId: "chat-owner" },
    selectedId: "A",
    characters: [
      {
        id: "A",
        name: "测试角色",
        initial: "角",
        color: "#eee",
        schedule: [],
        wearing: {},
        garments: [],
        _directory: {},
      },
    ],
    errors: {},
    loading: {},
  });
  const w = {
    ...storeFacade(state),
    current: {
      get value() {
        return state.characters[0];
      },
    },
    currentScope: () => JSON.stringify(["chat-owner", state.selectedId]),
    conversation: () => store,
    openConversation: async () => {},
    clock: () => "",
    detail: async () => {},
    loadWardrobe: async () => {},
    services: { mediaClient: { url: (id) => id } },
  };
  const view = await mountReact(vite, Chat, {
    workspace: w,
    props: { preview: false, onGo() {} },
  });
  try {
    const field = view.host.querySelector("textarea");
    await view.input(field, "尚未发送的草稿");
    field.focus();
    field.setSelectionRange(2, 5);
    await act(async () => {
      store.messages = [
        {
          id: "server",
          kind: "assistant",
          text: "后台读回",
          createdAt: "2026-10-26T00:00:00Z",
        },
      ];
    });
    await view.flush();
    assert.equal(field, view.host.querySelector("textarea"));
    assert.equal(field.value, "尚未发送的草稿");
    assert.equal(document.activeElement, field);
    assert.equal(field.selectionStart, 2);
    await act(async () =>
      field.dispatchEvent(
        new window.KeyboardEvent("keydown", {
          key: "Enter",
          isComposing: true,
          bubbles: true,
          cancelable: true,
        }),
      ),
    );
    assert.equal(submissions, 0);
    await view.click(button("发送"));
    assert.equal(submissions, 1);
    await view.input(field, "提交期间的新草稿");
    await act(async () => accepted());
    await view.flush();
    assert.equal(field.value, "提交期间的新草稿");
  } finally {
    await view.dispose();
  }
});

test("React setup confirms the exact password and focuses the mismatch without authenticating", async () => {
  const { default: Auth } = await load("react-app/pages/AuthPage.tsx"),
    real = await load("v3/providers/real.ts");
  const oldStatus = real.client.setupStatus,
    oldSetup = real.auth.setup;
  let calls = 0;
  real.client.setupStatus = async () => ({ setupAvailable: true });
  real.auth.setup = async (token, password) => {
    calls++;
    assert.equal(token, "controlled-setup-token");
    assert.equal(password, " exact password ");
  };
  const view = await mountReact(vite, Auth, {
    props: { onAuthenticated() {} },
  });
  try {
    await view.input(
      view.host.querySelector("#setup-token"),
      "controlled-setup-token",
    );
    await view.input(
      view.host.querySelector("#owner-password"),
      " exact password ",
    );
    await view.input(
      view.host.querySelector("#owner-password-confirm"),
      "different",
    );
    await view.click(button("完成初始化"));
    assert.equal(calls, 0);
    assert.match(view.host.textContent, /两次输入的密码不一致/);
    assert.equal(document.activeElement.id, "owner-password-confirm");
    await view.input(
      view.host.querySelector("#owner-password-confirm"),
      " exact password ",
    );
    await view.click(button("完成初始化"));
    assert.equal(calls, 1);
  } finally {
    await view.dispose();
    real.client.setupStatus = oldStatus;
    real.auth.setup = oldSetup;
  }
});

test("React diagnostic detail preserves the complete original request and retains it on read failure", async () => {
  const { default: Detail } = await load(
    "react-app/pages/DiagnosticsDetailPage.tsx",
  );
  let fail = false,
    reads = 0;
  const source = {
    id: "original-request",
    status: "failed",
    reason: "失败原因",
    payload: { actual: "未经改写的原内容" },
    result: { mutation_committed: false },
  };
  const w = {
    client: {
      ownerRequest: async (id) => {
        assert.equal(id, "original-request");
        reads++;
        if (fail) throw Error("读取失败");
        return structuredClone(source);
      },
    },
    services: { diagnosticsClient: {} },
  };
  const view = await mountReact(vite, Detail, {
    workspace: w,
    props: { target: { kind: "request", id: "original-request" } },
  });
  try {
    await view.click(button("原始记录与技术详情"));
    assert.match(view.host.textContent, /未经改写的原内容/);
    assert.match(view.host.textContent, /original-request/);
    fail = true;
    await view.click(button("重新读取"));
    assert.equal(reads, 2);
    assert.match(view.host.textContent, /读取失败/);
    assert.match(view.host.textContent, /未经改写的原内容/);
  } finally {
    await view.dispose();
  }
});

test("React resource editor rejects whitespace and preserves edits made while an original save is pending", async () => {
  const { default: Editor } = await load(
    "react-app/pages/StudioEditorPage.tsx",
  );
  let release,
    writes = 0,
    payload;
  const resource = {
    id: "place",
    kind: "place",
    name: "原名称",
    body: { description: "原描述" },
    draft_version: "v1",
    revision: 1,
  };
  const w = {
    ...storeFacade(proxy({ session:{actorId:"editor-render-owner"},characters:[],selectedId: "A", operations: [] })),
    services: {
      studioClient: {
        jobs:async()=>({items:[],next_cursor:""}),materials:async()=>({items:[],next_cursor:""}),referenceGrants:async()=>({items:[],next_cursor:""}),versions:async()=>({items:[],next_cursor:""}),
        resource: async () => structuredClone(resource),
        save: async (id, body) => {
          assert.equal(id, "place");
          writes++;
          payload = body;
          return new Promise((resolve) => (release = value=>{Object.assign(resource,value);resolve(structuredClone(resource));}));
        },
      },
    },
    navigatePath() {},
  };
  const view = await mountReact(vite, Editor, {
    workspace: w,
    props: { resourceId: "place" },
  });
  try {
    const name = view.host.querySelector("#resource-name");
    await view.input(view.host.querySelector("#resource-reason"),"明确保存理由");
    await view.input(name, "   ");
    await view.click(button("保存草稿"));
    assert.equal(writes, 0);
    assert.match(view.host.textContent, /请填写资源名称/);
    await view.input(name, "新名称");
    await view.click(button("保存草稿"));
    assert.equal(writes, 1);
    assert.equal(payload.name, "新名称");
    assert.equal(payload.expectedVersion, "v1");
    await view.input(name, "保存期间的新名称");
    await act(async () =>
      release({
        ...resource,
        name: "新名称",
        draft_version: "v2",
        revision: 2,
      }),
    );
    await view.flush();
    assert.equal(name.value, "保存期间的新名称");
    assert.equal(button("保存草稿").disabled, false);
  } finally {
    await view.dispose();
  }
});
