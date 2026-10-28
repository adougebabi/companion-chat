import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { act, mountReact, installDOM } from "./helpers/react-dom.mjs";
const environment = installDOM(),
  vite = await createServer({
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
const { selector } = await load("state/view-model.ts");
function workspace() {
  const character = {
    id: "A",
    name: "测试角色",
    job: "",
    bio: "",
    tags: [],
    people: [],
    sites: [],
    posts: [],
    garments: [],
    schedule: [],
    goals: [],
    life: { memories: [], diaries: [], appointments: [] },
    timezone: "Asia/Shanghai",
  };
  const state = proxy({
    session: { actorId: "review-owner" },
    selectedId: "A",
    characters: [character],
    media: [],
    operations: [],
    diagnostics: [],
    pending: [],
    notifications: [],
    errors: {},
    loading: {},
  });
  return {
    state,
    subscribe: (fn) => subscribe(state, fn),
    getSnapshot: () => snapshot(state),
    current: selector(() => state.characters[0]),
    currentScope: () =>
      JSON.stringify([state.session.actorId, state.selectedId]),
    services: {
      lifeClient: {},
      diaryClient: {},
      memoryClient: {},
      momentsClient: {},
    },
    day: () => "2026-10-26",
    notify() {},
    navigatePath() {},
    command() {
      throw Error("unexpected mutation");
    },
  };
}

test("media filtering changes the grid and previews use real img, video and audio elements", async () => {
  const { default: Browse } = await load("react-app/pages/BrowsePage.jsx"),
    w = workspace();
  w.state.media = [
    ["picture", "图片", "照片"],
    ["video", "视频", "视频片段"],
    ["audio", "语音", "声音"],
  ].map(([id, type, title], index) => ({
    id,
    type,
    title,
    url: "/fixture/" + id,
    character: "测试角色",
    tag: "日常",
    time: "今天",
    artIndex: index,
  }));
  const view = await mountReact(vite, Browse, {
    workspace: w,
    props: { page: "媒体" },
  });
  try {
    assert.equal(view.host.querySelectorAll(".media-tile").length, 3);
    assert.equal(view.host.querySelector("tag"), null);
    assert.ok(view.host.querySelector('img[src="/fixture/picture"]'));
    const search = view.host.querySelector('input[type="search"]');
    await view.input(search, "照片");
    assert.equal(view.host.querySelectorAll(".media-tile").length, 1);
    assert.match(view.host.textContent, /1 项/);
    await view.input(search, "没有");
    assert.equal(view.host.querySelectorAll(".media-tile").length, 0);
    assert.match(view.host.textContent, /暂无匹配媒体/);
    await view.input(search, "");
    for (const [id, tag] of [
      ["video", "video"],
      ["audio", "audio"],
    ]) {
      const tile = [...view.host.querySelectorAll(".media-tile")].find((node) =>
        node.textContent.includes(id === "video" ? "视频片段" : "声音"),
      );
      await view.click(tile.querySelector(".media-open"));
      assert.ok(document.querySelector(`[role="dialog"] ${tag}[controls]`));
      await view.click(
        [...document.querySelectorAll("button")].find(
          (node) => node.textContent === "关闭预览",
        ),
      );
    }
  } finally {
    await view.dispose();
  }
});

test("memory deep links open a reading Sheet by ID and the confirmation supports Escape cancellation", async () => {
  const { default: Life } = await load("react-app/pages/LifePage.jsx"),
    w = workspace();
  w.current.value.life.memories = [
    {
      id: "M",
      text: "需要核对的真实记忆",
      date: "2026-10-25",
      source: "对话",
      importance: 3,
      category: "长期记忆",
      tags: [],
      _source: {},
    },
  ];
  const view = await mountReact(vite, Life, {
    workspace: w,
    props: { page: "记忆", initialMemoryId: "M" },
  });
  try {
    const toggle = view.host.querySelector('[data-testid="memory-toggle-M"]');
    assert.equal(toggle.getAttribute("aria-haspopup"), "dialog");
    assert.ok(document.querySelector("#memory-details-M"));
    assert.equal(document.querySelectorAll('[role="dialog"]').length, 1);
    assert.equal(
      document.querySelector('[data-testid="memory-edit-content"]'),
      null,
    );
    await view.click(document.querySelector('[data-testid="memory-forget-M"]'));
    assert.ok(document.querySelector('[role="alertdialog"]'));
    await act(async () =>
      document.dispatchEvent(
        new window.KeyboardEvent("keydown", { key: "Escape", bubbles: true }),
      ),
    );
    await view.flush();
    assert.equal(document.querySelector('[role="alertdialog"]'), null);
    assert.ok(document.querySelector("#memory-details-M"));
    await view.click(document.querySelector('[data-testid="memory-edit-M"]'));
    const field = document.querySelector('[data-testid="memory-edit-content"]');
    await view.input(field, "取消时应保留原文");
    await view.click(
      document.querySelector('[data-testid="memory-drawer-close"]'),
    );
    assert.match(
      document.querySelector("#memory-details-M").textContent,
      /需要核对的真实记忆/,
    );
    assert.doesNotMatch(
      document.querySelector("#memory-details-M").textContent,
      /取消时应保留原文/,
    );
    await view.click(
      document.querySelector('[data-testid="memory-detail-drawer-close"]'),
    );
    assert.equal(document.activeElement, toggle);
  } finally {
    await view.dispose();
  }
});

test("comment candidate confirmation with IME never submits but ordinary Enter does", async () => {
  const { default: Browse } = await load("react-app/pages/BrowsePage.jsx"),
    w = workspace();
  let writes = 0;
  w.current.value.posts = [
    {
      id: "post",
      authorName: "测试角色",
      authorInitial: "角",
      time: "今天",
      text: "一条动态",
      comments: [],
      likes: 0,
      liked: false,
      _source: { revision: 2 },
    },
  ];
  w.command = () => {
    writes++;
    return { completion: Promise.resolve({}) };
  };
  const view = await mountReact(vite, Browse, {
    workspace: w,
    props: { page: "动态" },
  });
  try {
    await view.click(
      view.host.querySelector('[data-testid="comment-toggle-0"]'),
    );
    const field = view.host.querySelector("textarea");
    await view.input(field, "候选词");
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
    assert.equal(writes, 0);
    assert.equal(field.value, "候选词");
    await act(async () =>
      field.dispatchEvent(
        new window.KeyboardEvent("keydown", {
          key: "Enter",
          bubbles: true,
          cancelable: true,
        }),
      ),
    );
    await view.flush();
    assert.equal(writes, 1);
  } finally {
    await view.dispose();
  }
});

function worldWorkspace(name, revision = 1) {
  const w = workspace();
  let row = {
    id: "W",
    fluctlight_id: "A",
    name,
    revision,
    flavor: "日常",
    background: "",
    rules: "",
    description: "",
    item_pool: [],
    venue_categories: [],
  };
  sessionStorage.clear();
  w.isCurrent = (owner, fid) =>
    w.state.session.actorId === owner && w.state.selectedId === fid;
  w.client = {
    worldCatalog: async () => ({
      items: row ? [structuredClone(row)] : [],
      active_binding: null,
      next_cursor: null,
    }),
    catalogWorld: async () => ({
      available: true,
      world: structuredClone(row),
      places: [],
      npcs: [],
      scenes: [],
    }),
    deleteCatalogWorld: async () => {
      throw Error("unexpected mutation");
    },
  };
  w.deleted = () => {
    row = null;
  };
  return w;
}
async function openWorldRemoval(view, name) {
  await view.click(
    view.host.querySelector('[aria-label="进入世界 ' + name + '"]'),
  );
  await act(async () =>
    [...view.host.querySelectorAll("[role=tab]")]
      .find((n) => n.textContent === "世界设定")
      .dispatchEvent(
        new window.MouseEvent("mousedown", {
          bubbles: true,
          button: 0,
          ctrlKey: false,
        }),
      ),
  );
  await view.flush();
  await view.click(
    [...view.host.querySelectorAll("button")].find(
      (n) => n.textContent === "移除无引用世界",
    ),
  );
}
test("world removal confirmation supports its Radix close control without issuing a mutation", async () => {
  const { default: World } = await load("react-app/pages/WorldPage.jsx"),
    w = worldWorkspace("保留世界");
  const view = await mountReact(vite, World, { workspace: w });
  try {
    await openWorldRemoval(view, "保留世界");
    const dialog = document.querySelector("[role=dialog]");
    assert.ok(dialog);
    await view.click(
      [...dialog.querySelectorAll("button")].find(
        (node) => node.textContent === "关闭",
      ),
    );
    assert.equal(document.querySelector("[role=dialog]"), null);
    assert.match(view.host.textContent, /保留世界/);
  } finally {
    await view.dispose();
  }
});
test("world removal retains its confirmation through failure and replays the same original body before closing on readback", async () => {
  const { default: World } = await load("react-app/pages/WorldPage.jsx"),
    w = worldWorkspace("保留的世界", 7),
    bodies = [];
  let settle, reject;
  w.client.deleteCatalogWorld = (_fid, _id, body) => {
    bodies.push(structuredClone(body));
    return new Promise((resolve, no) => {
      settle = () => {
        w.deleted();
        resolve({ id: "W", status: "deleted", audit_id: "audit" });
      };
      reject = no;
    });
  };
  const v = await mountReact(vite, World, { workspace: w });
  try {
    await openWorldRemoval(v, "保留的世界");
    await v.input(
      document.querySelector('[aria-label="移除说明"]'),
      "移除没有引用的世界",
    );
    await v.click(
      [...document.querySelectorAll("button")].find(
        (n) => n.textContent === "确认移除",
      ),
    );
    assert.equal(bodies.length, 1);
    assert.equal(bodies[0].expectedRevision, 7);
    assert.ok(document.querySelector("[role=dialog]"));
    await act(async () => reject(Error("写入失败")));
    await v.flush();
    assert.match(
      document.querySelector("[role=dialog]").textContent,
      /保留的世界/,
    );
    await v.click(
      [...document.querySelectorAll("button")].find(
        (n) => n.textContent === "核对或重试原提交",
      ),
    );
    assert.equal(bodies.length, 2);
    assert.deepEqual(bodies[1], bodies[0]);
    await act(async () => settle());
    await v.flush();
    assert.equal(document.querySelector("[role=dialog]"), null);
  } finally {
    await v.dispose();
  }
});
test("memory forget keeps the original selected memory on failure and waits for completion before closing", async () => {
  const { default: Life } = await load("react-app/pages/LifePage.jsx"),
    w = workspace();
  let reject,
    settle,
    writes = 0;
  w.current.value.life.memories = [
    {
      id: "M",
      text: "保留的记忆正文",
      date: "2026-10-26",
      importance: 3,
      category: "长期记忆",
      tags: [],
      _source: { id: "M", revision: 4 },
    },
  ];
  w.command = () => {
    writes++;
    return {
      completion: new Promise((resolve, no) => {
        settle = resolve;
        reject = no;
      }),
    };
  };
  const v = await mountReact(vite, Life, {
    workspace: w,
    props: { page: "记忆", initialMemoryId: "M" },
  });
  try {
    await v.click(document.querySelector("[data-testid=memory-forget-M]"));
    await v.click(
      document.querySelector("[data-testid=memory-confirm-forget]"),
    );
    assert.equal(writes, 1);
    assert.ok(document.querySelector("[role=alertdialog]"));
    await act(async () => reject(Error("写入失败")));
    await v.flush();
    assert.ok(document.querySelector("[role=alertdialog]"));
    assert.match(
      document.querySelector("[data-slot=alert-dialog-description]")
        .textContent,
      /保留的记忆正文/,
    );
    await v.click(
      document.querySelector("[data-testid=memory-confirm-forget]"),
    );
    assert.equal(writes, 2);
    await act(async () => settle({}));
    await v.flush();
    assert.equal(document.querySelector("[role=alertdialog]"), null);
  } finally {
    await v.dispose();
  }
});
