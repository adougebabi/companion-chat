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

test("memory deep links expand by ID and the confirmation supports Escape cancellation", async () => {
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
    assert.equal(toggle.getAttribute("aria-expanded"), "true");
    assert.ok(view.host.querySelector("#memory-details-M"));
    await view.click(
      view.host.querySelector('[data-testid="memory-forget-M"]'),
    );
    assert.ok(document.querySelector('[role="alertdialog"]'));
    await act(async () =>
      document.dispatchEvent(
        new window.KeyboardEvent("keydown", { key: "Escape", bubbles: true }),
      ),
    );
    await view.flush();
    assert.equal(document.querySelector('[role="alertdialog"]'), null);
    assert.equal(toggle.getAttribute("aria-expanded"), "true");
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

test("world deletion confirmation supports its Radix close control without issuing a mutation", async () => {
  const { default: World } = await load("react-app/pages/WorldPage.jsx"),
    w = workspace();
  w.state.worlds = [
    {
      id: "W",
      name: "保留世界",
      genre: "日常",
      residents: [],
      sites: [],
      _source: { revision: 1 },
    },
  ];
  const view = await mountReact(vite, World, { workspace: w });
  try {
    const menuTrigger=view.host.querySelector('[aria-label="保留世界的世界操作"]');menuTrigger.focus();act(()=>menuTrigger.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Enter',bubbles:true})));await view.flush();
    await view.click([...document.querySelectorAll('[role="menuitem"]')].find(node=>node.textContent==='删除'));
    const dialog = document.querySelector('[role="alertdialog"]');
    assert.ok(dialog);
    await view.click(dialog.querySelector('[aria-label="关闭确认"]'));
    assert.equal(document.querySelector('[role="alertdialog"]'), null);
    assert.match(view.host.textContent, /保留世界/);
  } finally {
    await view.dispose();
  }
});
test('world delete keeps its original confirmation on a rejected command and closes only after acknowledged completion',async()=>{
 const {default:World}=await load('react-app/pages/WorldPage.jsx'),w=workspace(),bodies=[];let settle,reject;
 w.state.worlds=[{id:'W',name:'保留的世界',genre:'日常',residents:[],sites:[],_source:{revision:7}}];
 w.command=(_label,operation,body)=>{bodies.push({operation,body});return{completion:new Promise((resolve,no)=>{settle=resolve;reject=no;})};};
 const v=await mountReact(vite,World,{workspace:w});try{const trigger=v.host.querySelector('[aria-label="保留的世界的世界操作"]');trigger.focus();await act(async()=>trigger.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Enter',bubbles:true})));await v.flush();await v.click([...document.querySelectorAll('[role=menuitem]')].find(n=>n.textContent==='删除'));await v.click(document.querySelector('[data-slot=alert-dialog-action]'));assert.equal(bodies.length,1);assert.equal(bodies[0].body.expectedRevision,7);assert.ok(document.querySelector('[role=alertdialog]'));await act(async()=>reject(Error('写入失败')));await v.flush();assert.ok(document.querySelector('[role=alertdialog]'));assert.match(document.querySelector('[data-slot=alert-dialog-description]').textContent,/保留的世界/);await v.click(document.querySelector('[data-slot=alert-dialog-action]'));assert.equal(bodies.length,2);assert.deepEqual(bodies[1],bodies[0]);await act(async()=>settle({}));await v.flush();assert.equal(document.querySelector('[role=alertdialog]'),null);}finally{await v.dispose();}
});
test('memory forget keeps the original selected memory on failure and waits for completion before closing',async()=>{
 const {default:Life}=await load('react-app/pages/LifePage.jsx'),w=workspace();let reject,settle,writes=0;
 w.current.value.life.memories=[{id:'M',text:'保留的记忆正文',date:'2026-10-26',importance:3,category:'长期记忆',tags:[],_source:{id:'M',revision:4}}];
 w.command=()=>{writes++;return{completion:new Promise((resolve,no)=>{settle=resolve;reject=no;})};};
 const v=await mountReact(vite,Life,{workspace:w,props:{page:'记忆',initialMemoryId:'M'}});try{await v.click(v.host.querySelector('[data-testid=memory-forget-M]'));await v.click(document.querySelector('[data-testid=memory-confirm-forget]'));assert.equal(writes,1);assert.ok(document.querySelector('[role=alertdialog]'));await act(async()=>reject(Error('写入失败')));await v.flush();assert.ok(document.querySelector('[role=alertdialog]'));assert.match(document.querySelector('[data-slot=alert-dialog-description]').textContent,/保留的记忆正文/);await v.click(document.querySelector('[data-testid=memory-confirm-forget]'));assert.equal(writes,2);await act(async()=>settle({}));await v.flush();assert.equal(document.querySelector('[role=alertdialog]'),null);}finally{await v.dispose();}
});
