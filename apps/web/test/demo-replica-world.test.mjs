import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { worldFixture } from "./fixtures/world-fixture.mjs";
import {
  placeDraft,
  placeWrite,
  worldDraft,
  worldWrite,
  rebasePlaceDraft,
  rebaseWorldDraft,
} from "../src/approved-ui/world-fields.mjs";
const env = installDOM(),
  vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
const { createWorkspace } = await vite.ssrLoadModule(
    "/src/state/workspace.mjs",
  ),
  store = await vite.ssrLoadModule("/src/v3/proto/store.js"),
  { default: World } = await vite.ssrLoadModule(
    "/src/react-app/pages/WorldPage.jsx",
  );
test.after(async () => {
  await new Promise((r) => setTimeout(r, 20));
  await vite.close();
  env.restore();
});
const button = (root, text) =>
  [...root.querySelectorAll("button")].find(
    (n) => n.textContent.trim() === text,
  );
const dialog = () => [...document.querySelectorAll('[role="dialog"]')].at(-1);
async function tab(v, text) {
  await act(async () =>
    [...v.host.querySelectorAll('[role="tab"]')]
      .find((n) => n.textContent.trim().startsWith(text))
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
async function fixture({
  scenario = "normal",
  api,
  retainStorage = false,
  intercept,
} = {}) {
  if (!retainStorage) sessionStorage.clear();
  api ||= worldFixture({ owner: "world-owner", scenario });
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "world-owner" },
    characters: { A: { id: "A", name: "林青", status: "active" } },
    ui: { charId: "A" },
  });
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, "http://world.test"),
      method = init.method || "GET",
      body = init.body ? JSON.parse(init.body) : {};
    await intercept?.({ url, method, body, api });
    let result;
    const handled = await api.handler({
      url,
      path: url.pathname,
      method,
      body,
      json: (_res, value, status = 200) =>
        (result = new Response(JSON.stringify(value), {
          status,
          headers: { "content-type": "application/json" },
        })),
    });
    assert.ok(handled, "Unhandled " + method + " " + url.pathname);
    return result;
  };
  const v = await mountReact(vite, World, { workspace: w });
  return {
    w,
    v,
    api,
    s: api.state("A"),
    writes: () => api.calls.filter((c) => c.method !== "GET"),
    async dispose() {
      await v.dispose();
      w.dispose();
      await new Promise((r) => setTimeout(r, 10));
    },
  };
}
async function enter(f, name = "温和魔法世界") {
  await f.v.click(
    [...f.v.host.querySelectorAll("button")].find(
      (n) => n.getAttribute("aria-label") === "进入世界 " + name,
    ),
  );
}
async function openPlace(f) {
  await enter(f);
  await f.v.click(button(f.v.host, "查看与编辑"));
}
test("world browse/search/back keeps the original filter and inactive catalog tabs cannot switch worlds or read active-world objects", async () => {
  const f = await fixture();
  try {
    await f.v.input(
      f.v.host.querySelector('[aria-label="搜索世界名称或类型"]'),
      "远山",
    );
    await enter(f, "远山森林");
    await tab(f.v, "物品目录");
    assert.match(f.v.host.textContent, /物品目录随当前运行世界读取/);
    assert.equal(
      f.api.calls.some((c) => c.path.endsWith("/world/items")),
      false,
    );
    await tab(f.v, "世界事件");
    assert.match(f.v.host.textContent, /事件列表随当前运行世界读取/);
    assert.equal(f.writes().length, 0);
    assert.equal(f.s.binding.revision, 4);
    await f.v.click(button(f.v.host, "全部世界"));
    assert.equal(
      f.v.host.querySelector('[aria-label="搜索世界名称或类型"]').value,
      "远山",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="搜索世界名称或类型"]'),
      "完全没有",
    );
    assert.match(f.v.host.textContent, /没有找到匹配的世界/);
  } finally {
    await f.dispose();
  }
});
test("world create cancellation never writes and an accepted create does not bind or move the character", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "新建世界"));
    await f.v.input(
      dialog().querySelector('[aria-label="世界名称"]'),
      "取消的世界",
    );
    await f.v.click(button(dialog(), "取消"));
    assert.equal(f.writes().length, 0);
    await f.v.click(button(f.v.host, "新建世界"));
    await f.v.input(
      dialog().querySelector('[aria-label="世界名称"]'),
      "新增的海岸",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="世界背景"]'),
      "平静海岸边的生活。",
    );
    await f.v.click(button(dialog(), "保存世界"));
    assert.equal(f.writes().length, 1);
    assert.equal(f.s.worlds.at(-1).name, "新增的海岸");
    assert.equal(f.s.binding.revision, 4);
    assert.equal(f.s.context.context_revision, "unchanged-context");
    assert.equal(dialog(), undefined);
    assert.match(f.v.host.textContent, /新增的海岸/);
  } finally {
    await f.dispose();
  }
});
test("place edit preserves immutable name/category and omits unchanged nested arrays while reading the actual revision back", async () => {
  const f = await fixture();
  try {
    await openPlace(f);
    const original = structuredClone(f.s.places[0]);
    assert.equal(
      dialog().querySelector('[aria-label="正式名称"]').readOnly,
      true,
    );
    assert.equal(
      dialog().querySelector('[aria-label="所属场地"]').disabled,
      true,
    );
    await f.v.input(
      dialog().querySelector('[aria-label="别名"]'),
      "安静的小房间",
    );
    await f.v.click(button(dialog(), "保存地点"));
    const sent = f.writes().at(-1).body;
    assert.equal(sent.expectedRevision, 1);
    assert.equal(sent.name, "卧室");
    assert.equal(sent.alias, "安静的小房间");
    for (const key of [
      "furniture",
      "connections",
      "roomItems",
      "connectionPlaceIds",
      "parentPlaceId",
    ])
      assert.equal(key in sent, false, key + " must remain untouched");
    assert.deepEqual(f.s.places[0].furniture, original.furniture);
    assert.deepEqual(f.s.places[0].connections, original.connections);
    assert.equal(f.s.places[0].revision, 2);
    assert.match(f.v.host.textContent, /安静的小房间/);
  } finally {
    await f.dispose();
  }
});
test("world field builders preserve unknown nested data and only explicitly changed collections enter the wire payload", () => {
  const api = worldFixture(),
    s = api.state("A"),
    p = s.places[0],
    world = s.worlds[0];
  const d = placeDraft(p, world);
  d.alias = "只改别名";
  assert.equal("furniture" in placeWrite(d).body, false);
  d.furniture = '[{"customKey":{"exactName":"保留正文"}}]';
  assert.deepEqual(placeWrite(d).body.furniture, [
    { customKey: { exactName: "保留正文" } },
  ]);
  const wd = worldDraft(world);
  wd.name = "改名";
  assert.equal("itemPool" in worldWrite(wd).body, false);
  assert.throws(
    () => placeWrite({ ...d, name: "不能改正式名称" }),
    /正式名称固定/,
  );
  assert.throws(
    () => placeWrite({ ...d, category: "shops" }),
    /已有场地分类固定/,
  );
  assert.throws(
    () => placeWrite({ ...d, furniture: "[格式错误" }),
    /JSON数组格式无效/,
  );
});
test("a lost world-create receipt resumes the same body and key after remount; newer input updates the acknowledged world instead of creating another", async () => {
  const first = await fixture({ scenario: "unknown" });
  await first.v.click(button(first.v.host, "新建世界"));
  await first.v.input(
    dialog().querySelector('[aria-label="世界名称"]'),
    "原创建 A",
  );
  await first.v.click(button(dialog(), "保存世界"));
  await first.v.input(
    dialog().querySelector('[aria-label="世界名称"]'),
    "后续草稿 B",
  );
  const api = first.api,
    original = first.writes()[0];
  await first.dispose();
  const next = await fixture({ api, retainStorage: true });
  try {
    await next.v.click(button(next.v.host, "继续核对世界创建"));
    assert.equal(
      dialog().querySelector('[aria-label="世界名称"]').value,
      "后续草稿 B",
    );
    await next.v.click(button(dialog(), "核对或重试原提交"));
    assert.deepEqual(next.writes()[1].body, original.body);
    assert.equal(next.s.worlds.length, 3);
    assert.equal(
      dialog().querySelector('[aria-label="世界名称"]').value,
      "后续草稿 B",
    );
    await next.v.click(button(dialog(), "读取最新版本并保留输入"));
    await next.v.click(button(dialog(), "已核对，采用最新基线"));
    await next.v.click(button(dialog(), "保存世界"));
    assert.equal(next.s.worlds.length, 3);
    assert.equal(next.s.worlds.at(-1).name, "后续草稿 B");
    assert.equal(next.writes().at(-1).method, "PUT");
    assert.equal(next.s.binding.revision, 4);
  } finally {
    await next.dispose();
  }
});
test("world readback failure retains the accepted result and recovery sends no additional mutation", async () => {
  const f = await fixture({ scenario: "readback-failure" });
  try {
    await enter(f);
    await f.v.click(button(f.v.host, "编辑世界"));
    await f.v.input(
      dialog().querySelector('[aria-label="世界名称"]'),
      "世界已保存",
    );
    await f.v.click(button(dialog(), "保存世界"));
    assert.equal(f.writes().length, 1);
    assert.match(dialog().textContent, /原操作已有回执/);
    await f.v.click(button(dialog(), "只读恢复页面结果"));
    assert.equal(f.writes().length, 1);
    assert.equal(dialog(), undefined);
    assert.match(f.v.host.textContent, /世界已保存/);
  } finally {
    await f.dispose();
  }
});
test("world binding uses its own revision and preserves the existing life context", async () => {
  const f = await fixture();
  try {
    await enter(f, "远山森林");
    await f.v.click(button(f.v.host, "切换到此世界"));
    await f.v.click(button(dialog(), "切换世界"));
    assert.equal(f.writes()[0].body.expectedRevision, 4);
    assert.equal(f.s.binding.revision, 5);
    assert.equal(f.s.worlds[1].revision, 7);
    assert.equal(f.s.context.context_revision, "unchanged-context");
    assert.match(f.v.host.textContent, /世界绑定已读回/);
  } finally {
    await f.dispose();
  }
});
test("unread world catalog remains unknown; failure retries a read and an empty catalog stays empty", async () => {
  let release;
  const gate = new Promise((r) => (release = r));
  let first = true;
  const f = await fixture({
    intercept: async ({ url }) => {
      if (first && url.pathname.endsWith("/world/catalog")) {
        first = false;
        await gate;
      }
    },
  });
  try {
    assert.match(f.v.host.textContent, /正在读取世界与地点/);
    assert.doesNotMatch(f.v.host.textContent, /还没有建立生活世界/);
    release();
    await f.v.flush();
    assert.match(f.v.host.textContent, /温和魔法世界/);
  } finally {
    release();
    await f.dispose();
  }
  const bad = await fixture({ scenario: "failure" });
  try {
    assert.ok(button(bad.v.host, "重新读取世界目录"));
    await bad.v.click(button(bad.v.host, "重新读取世界目录"));
    assert.match(bad.v.host.textContent, /温和魔法世界/);
    assert.equal(bad.writes().length, 0);
  } finally {
    await bad.dispose();
  }
  const empty = await fixture({ scenario: "empty" });
  try {
    assert.match(empty.v.host.textContent, /还没有建立生活世界/);
    assert.equal(empty.s.worlds.length, 0);
  } finally {
    await empty.dispose();
  }
});
test("item configuration uses the catalog item revision without creating inventory, and a request preserves existing chat draft without sending", async () => {
  const f = await fixture();
  try {
    await enter(f);
    await tab(f.v, "物品目录");
    await f.v.click(button(f.v.host, "详情"));
    await f.v.click(button(dialog(), "配置取得范围"));
    await f.v.click(dialog().querySelector('[role="switch"]'));
    await f.v.click(button(dialog(), "保存取得范围"));
    assert.equal(f.writes().length, 1);
    assert.equal(f.writes()[0].body.expectedRevision, 3);
    assert.equal(f.s.items[0].available, false);
    assert.equal(f.s.items[0].revision, 4);
    assert.equal(
      f.writes().some((c) => c.path.includes("inventory")),
      false,
    );
    assert.equal(button(dialog(), "向角色提出请求").disabled, true);
    await f.v.click(button(dialog(), "配置取得范围"));
    await f.v.click(dialog().querySelector('[role="switch"]'));
    await f.v.click(button(dialog(), "保存取得范围"));
    const key = "fluctlight:approved-chat-draft:" + f.w.currentScope();
    sessionStorage.setItem(key, "原来的未发送输入");
    await f.v.click(button(dialog(), "向角色提出请求"));
    assert.match(sessionStorage.getItem(key), /^原来的未发送输入\n\n/);
    assert.match(sessionStorage.getItem(key), /A-world-item/);
    assert.equal(f.writes().length, 2);
    assert.match(location.hash, /chat/);
  } finally {
    await f.dispose();
  }
});
test("world events distinguish admitted queued execution from applied results; failed execution retries its original run with two revisions", async () => {
  const f = await fixture();
  try {
    await enter(f);
    await tab(f.v, "世界事件");
    await f.v.click(button(f.v.host, "详情"));
    await f.v.click(button(dialog(), "请求触发一次"));
    await f.v.input(
      dialog().querySelector('[aria-label="世界事件操作说明"]'),
      "安排一次实际夜读",
    );
    await f.v.click(button(dialog(), "请求触发一次"));
    assert.equal(f.writes()[0].body.expectedRevision, 6);
    assert.equal(f.s.runs.length, 1);
    assert.match(dialog().textContent, /等待执行/);
    assert.doesNotMatch(dialog().textContent, /已应用/);
    const original = f.s.runs[0];
    original.status = "failed";
    original.revision = 4;
    original.error_code = "fixture_worker_failed";
    await f.v.click(button(dialog(), "刷新事件与执行"));
    await f.v.click(button(dialog(), "恢复这次失败执行"));
    await f.v.input(
      dialog().querySelector('[aria-label="世界事件操作说明"]'),
      "恢复原执行窗口",
    );
    await f.v.click(button(dialog(), "恢复这次失败执行"));
    assert.equal(f.s.runs.length, 1);
    const retry = f.writes().at(-1);
    assert.match(retry.path, new RegExp(original.id + "/retry$"));
    assert.equal(retry.body.expectedRevision, 6);
    assert.equal(retry.body.expectedRunRevision, 4);
    assert.equal(f.s.runs[0].status, "queued");
    assert.equal(f.s.runs[0].revision, 5);
  } finally {
    await f.dispose();
  }
});
test("place historical default restoration uses both versions and preserves spatial references", async () => {
  const f = await fixture();
  try {
    await openPlace(f);
    await f.v.input(
      dialog().querySelector('[aria-label="别名"]'),
      "需要恢复的别名",
    );
    await f.v.click(button(dialog(), "保存地点"));
    const p = f.s.places[0];
    p.connection_place_ids = [f.s.places[1].id];
    await f.v.click(button(f.v.host, "修订记录"));
    assert.match(dialog().textContent, /更新地点字段/);
    await f.v.click(button(dialog(), "恢复默认字段"));
    await f.v.input(
      dialog().querySelector('[aria-label="地点基准操作说明"]'),
      "恢复最初的字段，不改动空间引用",
    );
    await f.v.click(button(dialog(), "确认恢复默认字段"));
    const sent = f.writes().at(-1).body;
    assert.equal(sent.expectedRevision, 2);
    assert.equal(sent.expectedDefaultRevision, 1);
    assert.equal(sent.operation, "restore");
    assert.equal(p.alias, "她的房间");
    assert.equal(p.revision, 3);
    assert.deepEqual(p.connection_place_ids, [f.s.places[1].id]);
  } finally {
    await f.dispose();
  }
});
test("reviewing a newer baseline refreshes untouched nested fields while retaining the user's actual edits", () => {
  const s = worldFixture().state("A"),
    p = s.places[0],
    world = s.worlds[0],
    d = placeDraft(p, world),
    wd = worldDraft(world);
  d.alias = "自己的编辑";
  wd.name = "自己的世界名";
  const latestPlace = {
    ...structuredClone(p),
    revision: 2,
    furniture: [{ newExtension: { stillCamelCase: "另一会话的新值" } }],
    room_items: [{ ownerDefined: "新物品" }],
  };
  const rebased = rebasePlaceDraft(d, latestPlace),
    body = placeWrite(rebased).body;
  assert.equal(rebased.alias, "自己的编辑");
  assert.equal(body.expectedRevision, 2);
  assert.equal("furniture" in body, false);
  assert.equal("roomItems" in body, false);
  assert.equal(
    rebased.furniture,
    JSON.stringify(latestPlace.furniture, null, 2),
  );
  const latestWorld = {
    ...structuredClone(world),
    revision: 3,
    item_pool: [{ newPool: { exactKey: "新池" } }],
  };
  const worldRebased = rebaseWorldDraft(wd, latestWorld);
  assert.equal(worldRebased.name, "自己的世界名");
  assert.equal("itemPool" in worldWrite(worldRebased).body, false);
});
test("category conflict review preserves the externally updated world name rather than renaming it back", async () => {
  let conflictOnce = true;
  const f = await fixture({
    intercept: async ({ url, method, api }) => {
      if (
        conflictOnce &&
        method === "PUT" &&
        url.pathname.endsWith("/world/catalog/A-world-one")
      ) {
        conflictOnce = false;
        const world = api.state("A").worlds[0];
        world.name = "另一会话的新名字";
        world.revision++;
      }
    },
  });
  try {
    await enter(f);
    await f.v.click(button(f.v.host, "添加分类"));
    await f.v.input(
      dialog().querySelector('[aria-label="分类名称"]'),
      "河岸街区",
    );
    await f.v.click(button(dialog(), "添加分类"));
    await f.v.click(button(dialog(), "读取最新版本并保留输入"));
    await f.v.click(button(dialog(), "已核对，采用最新基线"));
    await f.v.click(button(dialog(), "添加分类"));
    assert.equal(f.s.worlds[0].name, "另一会话的新名字");
    assert.equal(f.writes().at(-1).body.name, "另一会话的新名字");
    assert.equal(f.writes().at(-1).body.expectedRevision, 3);
    assert.equal(f.s.worlds[0].venue_categories.at(-1).name, "河岸街区");
  } finally {
    await f.dispose();
  }
});
test("recovering an unknown place create records the saved immutable identity and retains later editable fields and unapplied identity input", async () => {
  const f = await fixture({ scenario: "unknown" });
  try {
    await enter(f);
    await f.v.click(button(f.v.host, "添加地点"));
    await f.v.input(
      dialog().querySelector('[aria-label="正式名称"]'),
      "原地点 A",
    );
    await f.v.click(button(dialog(), "保存地点"));
    await f.v.input(
      dialog().querySelector('[aria-label="正式名称"]'),
      "未应用的新名字 B",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="别名"]'),
      "保留的后续别名",
    );
    await f.v.click(button(dialog(), "核对或重试原提交"));
    assert.equal(f.s.places.length, 3);
    assert.equal(
      dialog().querySelector('[aria-label="正式名称"]').value,
      "原地点 A",
    );
    assert.equal(
      dialog().querySelector('[aria-label="正式名称"]').readOnly,
      true,
    );
    assert.match(dialog().textContent, /未应用的新名字 B/);
    assert.equal(
      dialog().querySelector('[aria-label="别名"]').value,
      "保留的后续别名",
    );
    await f.v.click(button(dialog(), "读取最新版本并保留输入"));
    await f.v.click(button(dialog(), "已核对，采用最新基线"));
    await f.v.click(button(dialog(), "保存地点"));
    assert.equal(f.s.places.length, 3);
    assert.equal(f.s.places.at(-1).name, "原地点 A");
    assert.equal(f.s.places.at(-1).alias, "保留的后续别名");
    assert.equal(f.s.places.at(-1).revision, 2);
    assert.equal(f.writes().at(-1).method, "PUT");
  } finally {
    await f.dispose();
  }
});
test("remounted event-toggle readback checks the originally executed target rather than the newly opened opposite action", async () => {
  let fail = true;
  const first = await fixture({
    intercept: async ({ url, method, api }) => {
      if (
        fail &&
        method === "GET" &&
        url.pathname.endsWith("/world/templates/A-template") &&
        api.calls.some(
          (c) =>
            c.method === "PUT" &&
            c.path.endsWith("/world/templates/A-template"),
        )
      )
        throw Error("切换后读取断网");
    },
  });
  await enter(first);
  await tab(first.v, "世界事件");
  await first.v.click(first.v.host.querySelector('[role="switch"]'));
  await first.v.input(
    dialog().querySelector('[aria-label="世界事件操作说明"]'),
    "暂停真实模板",
  );
  await first.v.click(button(dialog(), "暂停事件"));
  assert.match(dialog().textContent, /原操作已有回执/);
  assert.equal(first.s.templates[0].enabled, false);
  const api = first.api;
  await first.dispose();
  fail = false;
  const next = await fixture({ api, retainStorage: true });
  try {
    await enter(next);
    await tab(next.v, "世界事件");
    await next.v.click(next.v.host.querySelector('[role="switch"]'));
    assert.ok(button(dialog(), "只读恢复页面结果"));
    await next.v.click(button(dialog(), "只读恢复页面结果"));
    assert.equal(dialog(), undefined);
    assert.equal(next.s.templates[0].enabled, false);
    assert.equal(next.writes().length, 1);
  } finally {
    await next.dispose();
  }
});
