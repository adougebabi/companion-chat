import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
const env = installDOM(),
  vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
test.after(async () => {
  await new Promise((r) => setTimeout(r, 20));
  await vite.close();
  env.restore();
});


const { default: Wardrobe } = await vite.ssrLoadModule(
    "/src/react-app/pages/WardrobePage.jsx",
  ),
  { selector } = await vite.ssrLoadModule("/src/state/view-model.ts");
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll("[role=dialog]")].at(-1);
const source = {
  id: "G",
  fluctlight_id: "A",
  description: "白衬衫",
  slot: "upper_body",
  category: "shirt",
  clothing_type: "upper_body",
  ownership: "owned",
  availability: "available",
  seasons: ["autumn"],
  revision: 2,
  archived_at: null,
};
function workspace() {
  const state = proxy({
      session: { actorId: "wardrobe-owner" },
      selectedId: "A",
      characters: [
        {
          id: "A",
          name: "林青",
          status: "正常",
          garments: [
            {
              id: "G",
              name: "白衬衫",
              type: "上衣",
              seasons: ["秋"],
              _source: { ...source },
            },
          ],
          outfits: [],
          outfitTags: ["日常", "外出"],
          wearing: { 上衣: ["G"] },
          _wardrobe: {
            catalog: { revision: 4 },
            wearing: { revision: 4, worn_items: [] },
            library: { profile_id: "profile" },
            tags: { revision: 3 },
          },
        },
      ],
      operations: [],
      errors: {},
      loading: {},
      outfitTab: "全部衣服",
    }),
    calls = [];
  const inventoryClient = {
    preparation: async () => ({
      status: "completed",
      result: { inventory_committed: true },
    }),
    history: async () => ({ items: [], next_cursor: "" }),
    outfitHistory: async () => ({ items: [], next_cursor: "" }),
    list: async () => ({ items: [], next_cursor: "" }),
    update: async (fid, id, body) => {
      calls.push(["update", fid, id, structuredClone(body)]);
      return { item: { ...source, ...body.item, revision: 3 }, revision: 5 };
    },
    create: async (fid, body) => {
      calls.push(["create", fid, structuredClone(body)]);
      return { item: { ...source, id: "new", ...body.item }, revision: 5 };
    },
    saveOutfit: async (fid, id, body) => {
      calls.push(["outfit", fid, id, structuredClone(body)]);
      return { outfit: { id: "O", revision: 1 }, revision: 5 };
    },
    archive: async () => {
      throw Error("归档尚未完成");
    },
  };
  const w = {
    state,
    calls,
    current: selector(() => state.characters[0]),
    subscribe: (fn) => subscribe(state, fn),
    getSnapshot: () => snapshot(state),
    currentScope: () =>
      JSON.stringify([state.session.actorId, state.selectedId]),
    services: { inventoryClient },
    navigate: (page) => calls.push(["navigate", page]),
    loadWardrobe: async () => calls.push(["readback"]),
    command: (label, operation, body, execute, options) => {
      calls.push(["command", label, operation, structuredClone(body)]);
      const record = {
        completion: Promise.resolve()
          .then(() => execute(body, "wardrobe-original-key"))
          .then(async (result) => {
            await options.onSaved?.(result);
            return result;
          }),
      };
      record.completion.catch(() => {});
      return record;
    },
  };
  return w;
}
const mount = (w) => mountReact(vite, Wardrobe, { workspace: w });
async function tab(v, text) {
  await act(async () =>
    [...v.host.querySelectorAll("[role=tab]")]
      .find((n) => n.textContent.startsWith(text))
      .dispatchEvent(
        new window.MouseEvent("mousedown", {
          bubbles: true,
          button: 0,
          ctrlKey: false,
        }),
      ),
  );
  await v.flush();
}
test("wardrobe reads confirmed wearing only, search emptiness clears without changing the catalogue", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    assert.match(
      v.host.querySelector("[data-testid=current-outfit]").textContent,
      /还没有登记实际穿着/,
    );
    assert.equal(v.host.querySelector(".r-floating-badge"), null);
    await v.input(v.host.querySelector('[aria-label="搜索衣服"]'), "没有这件");
    assert.match(v.host.textContent, /没有符合条件的衣服/);
    await v.click(button(v.host, "清除筛选"));
    assert.match(v.host.querySelector(".r-garment-grid").textContent, /白衬衫/);
    assert.equal(w.state.characters[0].garments.length, 1);
  } finally {
    await v.dispose();
  }
});
test("garment cancel preserves original; save sends real ownership/availability and exact revision then reads back", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await v.click(v.host.querySelector('[aria-label="查看白衬衫"]'));
    await v.input(
      dialog().querySelector('[aria-label="衣服名称"]'),
      "未提交的衣服名",
    );
    await v.click(button(dialog(), "取消"));
    assert.equal(w.state.characters[0].garments[0].name, "白衬衫");
    await v.click(v.host.querySelector('[aria-label="查看白衬衫"]'));
    assert.equal(
      dialog().querySelector('[aria-label="衣服名称"]').value,
      "白衬衫",
    );
    await v.input(
      dialog().querySelector('[aria-label="衣服名称"]'),
      "柔软白衬衫",
    );
    await v.click(dialog().querySelector('[aria-label="拥有关系"]'));
    await v.click(
      [...document.querySelectorAll("[role=option]")].find(
        (n) => n.textContent === "借用",
      ),
    );
    await v.click(dialog().querySelector('[aria-label="可用状态"]'));
    await v.click(
      [...document.querySelectorAll("[role=option]")].find(
        (n) => n.textContent === "暂不可用",
      ),
    );
    await v.click(button(dialog(), "保存衣服"));
    const sent = w.calls.find((c) => c[0] === "update");
    assert.equal(sent[3].expectedRevision, 4);
    assert.equal(sent[3].expectedItemRevision, 2);
    assert.equal(sent[3].item.ownership, "borrowed");
    assert.equal(sent[3].item.availability, "unavailable");
    assert.equal(sent[3].item.description, "柔软白衬衫");
    assert.equal(sent[3].item.category, "shirt");
    assert.ok(w.calls.some((c) => c[0] === "readback"));
    assert.equal(dialog(), undefined);
  } finally {
    await v.dispose();
  }
});
test("composition saves actual membership without wearing; cancellation and missing member protection retain original plans", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await tab(v, "搭配工作台");
    await v.input(
      v.host.querySelector('[aria-label="方案名称"]'),
      "安静的日常",
    );
    await v.click(button(v.host, "保存为穿搭方案"));
    const sent = w.calls.find((c) => c[0] === "outfit");
    assert.deepEqual(sent[3].itemIds, ["G"]);
    assert.equal(sent[3].profileId, "profile");
    assert.equal(sent[3].expectedRevision, 4);
    assert.equal(sent[3].expectedOutfitRevision, 0);
    assert.equal(
      w.calls.some((c) => c[0] === "wear"),
      false,
    );
    await act(async () => {
      w.state.characters[0].outfits = [
        {
          id: "O",
          name: "旧方案",
          tag: "日常",
          items: { 上衣: ["missing"] },
          _source: { revision: 3, items: [] },
        },
      ];
      w.state.outfitTab = "方案库";
    });
    await v.flush();
    await v.click(button(v.host, "预览"));
    assert.equal(button(dialog(), "请换上这套").disabled, true);
    await v.click(button(dialog(), "编辑方案"));
    await v.click(button(v.host, "保存方案修改"));
    assert.match(v.host.textContent, /部分衣物已不在当前目录/);
    assert.equal(w.calls.filter((c) => c[0] === "outfit").length, 1);
    await v.click(button(v.host, "取消编辑"));
    assert.equal(w.state.characters[0].outfits[0].items.上衣[0], "missing");
  } finally {
    await v.dispose();
  }
});
test("archived pagination failure retries its frozen cursor while filter edits remain local", async () => {
  const w = workspace(),
    calls = [];
  let fail = true;
  w.services.inventoryClient.list = async (fid, options) => {
    calls.push(structuredClone(options));
    if (options.cursor && fail) throw Error("归档读取中断");
    return {
      items: [
        {
          ...source,
          id: options.cursor ? "archived-two" : "archived-one",
          description: options.cursor ? "归档二" : "归档一",
          archived_at: "2026-10-27T00:00:00Z",
        },
      ],
      next_cursor: options.cursor ? "" : "next-archived",
    };
  };
  const v = await mount(w);
  try {
    await v.click(v.host.querySelector("[role=checkbox]"));
    assert.match(v.host.textContent, /归档一/);
    await v.click(button(v.host, "加载更多归档衣服"));
    assert.match(v.host.textContent, /归档读取中断/);
    await v.input(v.host.querySelector('[aria-label="搜索衣服"]'), "归档");
    fail = false;
    await v.click(button(v.host, "重新读取归档衣服"));
    assert.deepEqual(calls[2], calls[1]);
    assert.equal(calls[2].cursor, "next-archived");
    assert.match(v.host.textContent, /归档一/);
    assert.match(v.host.textContent, /归档二/);
    await v.click(v.host.querySelector('[aria-label="查看归档一"]'));
    assert.equal(
      dialog().querySelector('[aria-label="衣服名称"]').disabled,
      true,
    );
    assert.equal(button(dialog(), "保存衣服"), undefined);
  } finally {
    await v.dispose();
  }
});
test("an archive failure stays in the confirmation; loading/retired scopes cannot admit writes", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await v.click(v.host.querySelector('[aria-label="查看白衬衫"]'));
    await v.click(button(dialog(), "归档服装"));
    await v.click(
      button(document.querySelector("[role=alertdialog]"), "确认归档"),
    );
    assert.match(
      document.querySelector("[role=alertdialog]").textContent,
      /归档尚未完成/,
    );
    await v.click(button(document.querySelector("[role=alertdialog]"), "取消"));
    await v.click(button(dialog(), "取消"));
    await act(async () => {
      w.state.characters[0].status = "已退休";
    });
    await v.flush();
    assert.equal(button(v.host, "添加衣服").disabled, true);
    assert.equal(w.calls.filter((c) => c[0] === "command").length, 1);
  } finally {
    await v.dispose();
  }
});
test("a changed clothing slot cannot silently drop a selected item, and retirement fences an already open archive confirmation", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await tab(v, "搭配工作台");
    await v.input(
      v.host.querySelector('[aria-label="方案名称"]'),
      "保留明确选择",
    );
    await act(async () => {
      w.state.characters[0].garments[0].type = "外套";
    });
    await v.flush();
    await v.click(button(v.host, "保存为穿搭方案"));
    assert.match(v.host.textContent, /服装部位已变化/);
    assert.equal(
      w.calls.some((c) => c[0] === "command"),
      false,
    );
    await tab(v, "全部衣服");
    await v.click(v.host.querySelector('[aria-label="查看白衬衫"]'));
    await v.click(button(dialog(), "归档服装"));
    await act(async () => {
      w.state.characters[0].status = "已退休";
    });
    await v.flush();
    await v.click(
      button(document.querySelector("[role=alertdialog]"), "确认归档"),
    );
    assert.equal(
      w.calls.some((c) => c[0] === "command"),
      false,
    );
    assert.ok(document.querySelector("[role=alertdialog]"));
  } finally {
    await v.dispose();
  }
});

test('retained wardrobe return queries restore the native search and stable plan tabs',async()=>{
 const w=workspace(),route=q=>({query:new URLSearchParams(q)});
 const v=await mountReact(vite,Wardrobe,{workspace:w,props:{route:route('tab=clothes&q=不存在的衣服')}});
 try{
  assert.equal(v.host.querySelector('[aria-label="搜索衣服"]').value,'不存在的衣服');
  assert.equal(v.host.querySelector('[aria-label="查看白衬衫"]'),null);
  await v.update({route:route('tab=clothes&q=白衬衫')});
  assert.equal(v.host.querySelector('[aria-label="搜索衣服"]').value,'白衬衫');
  assert.ok(v.host.querySelector('[aria-label="查看白衬衫"]'));
  await v.update({route:route('tab=plans')});
  assert.match(v.host.querySelector('[role=tab][aria-selected=true]').textContent,/穿搭方案/);
  assert.equal(w.calls.some(c=>c[0]==='command'),false);
 }finally{await v.dispose();}
});