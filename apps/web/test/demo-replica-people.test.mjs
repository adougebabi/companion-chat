import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { peopleFixture } from "./fixtures/people-fixture.mjs";
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
  { default: People } = await vite.ssrLoadModule(
    "/src/react-app/pages/PeoplePage.jsx",
  ),
  helpers = await vite.ssrLoadModule(
    "/src/approved-ui/people-presentation.mjs",
  );
test.after(async () => {
  await new Promise((r) => setTimeout(r, 20));
  await vite.close();
  env.restore();
});
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll("[role=dialog]")].at(-1),
  clone = (v) => JSON.parse(JSON.stringify(v));
async function tab(v, text, root = v.host) {
  await act(async () =>
    [...root.querySelectorAll("[role=tab]")]
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
async function choose(v, label, text) {
  await v.click(document.querySelector('[aria-label="' + label + '"]'));
  await v.click(
    [...document.querySelectorAll("[role=option]")].find(
      (n) => n.textContent.trim() === text,
    ),
  );
}
const route = (query = {}) => ({
  page: "人物关系",
  fid: "A",
  path: "/c/A/people",
  query: new URLSearchParams(query),
});
async function fixture({
  scenario = "normal",
  api,
  retainStorage = false,
  intercept,
  query,
} = {}) {
  if (!retainStorage) sessionStorage.clear();
  api ||= peopleFixture({ owner: "people-owner", scenario });
  const w = createWorkspace();
  store.replaceSnapshot({
    ...store.get(),
    session: { actorId: "people-owner" },
    characters: { A: { id: "A", name: "林青", status: "active" } },
    ui: { charId: "A" },
  });
  w.client.fetcher = async (input, init = {}) => {
    const url = new URL(input, "http://people.test"),
      method = init.method || "GET",
      body = init.body ? JSON.parse(init.body) : {};
    const override = await intercept?.({ url, method, body, api });
    if (override instanceof Response) return override;
    let response;
    const handled = await api.handler({
      url,
      path: url.pathname,
      method,
      body,
      json: (_res, value, status = 200) =>
        (response = new Response(JSON.stringify(value), {
          status,
          headers: { "content-type": "application/json" },
        })),
    });
    assert.ok(handled, "Unhandled " + method + " " + url.pathname);
    return response;
  };
  const v = await mountReact(vite, People, {
    workspace: w,
    props: { route: route(query) },
  });
  return {
    v,
    w,
    api,
    s: api.state("A"),
    writes: () =>
      api.calls.filter(
        (c) => c.method !== "GET" && !c.path.endsWith("apply-preview"),
      ),
    async dispose() {
      await v.dispose();
      w.dispose();
      await new Promise((r) => setTimeout(r, 10));
    },
  };
}
async function editAbout(f) {
  await tab(f.v, "关于你");
  await f.v.click(button(f.v.host, "编辑资料"));
}
async function openPerson(f, name = "苏禾") {
  await f.v.click(
    f.v.host.querySelector('[aria-label="查看人物 ' + name + '"]'),
  );
}
test("met status follows current-profile or global relationships, never directory membership or a declined encounter", async () => {
  const f = await fixture();
  try {
    assert.equal(f.v.host.querySelectorAll(".companion-person-card").length, 2);
    await choose(f.v, "人物范围", "已经结识");
    assert.equal(f.v.host.querySelectorAll(".companion-person-card").length, 1);
    assert.match(
      f.v.host.querySelector(".companion-person-card").textContent,
      /苏禾/,
    );
    await choose(f.v, "人物范围", "尚未结识");
    assert.match(
      f.v.host.querySelector(".companion-person-card").textContent,
      /陈渡/,
    );
    assert.equal(f.s.records.get("A-npc-1").encounters.length, 1);
    await f.v.input(
      f.v.host.querySelector('[aria-label="搜索名字、身份或故事"]'),
      "没有这个人",
    );
    assert.match(f.v.host.textContent, /没有找到这个人/);
    await f.v.click(button(f.v.host, "清除筛选"));
    assert.equal(f.v.host.querySelectorAll(".companion-person-card").length, 2);
  } finally {
    await f.dispose();
  }
});
test("predicted/default/historical positions cannot become actual current presence and old-profile relationships remain separate", () => {
  const p = [
      { id: "home", name: "默认家" },
      { id: "last", name: "上次地点" },
      { id: "predicted", name: "预计地点" },
    ],
    npc = {
      id: "N",
      revision: 3,
      home_place_id: "home",
      confirmed_place_id: "last",
      confirmed_at: "2026-10-01T00:00:00Z",
      planned_position: {
        place_id: "predicted",
        source: "schedule_prediction",
      },
    };
  assert.equal(helpers.npcLocation(npc, p).label, "预计地点");
  assert.equal(
    helpers.npcLocation({ ...npc, planned_position: null }, p).name,
    "默认家",
  );
  const rows = [
    { id: "old", target_actor_id: "actor", profile_id: "old" },
    { id: "shared", target_actor_id: "actor", profile_id: null },
    { id: "active", target_actor_id: "actor", profile_id: "sunny" },
  ];
  assert.equal(
    helpers.relationshipFor(rows, "actor", "sunny", "A").id,
    "active",
  );
  assert.equal(helpers.relationshipFor(rows, "actor", null, "A").id, "shared");
  assert.equal(helpers.relationshipFor([rows[0]], "actor", "sunny", "A"), null);
});
test("about-you cancellation asks before discarding and never writes", async () => {
  const f = await fixture();
  try {
    await editAbout(f);
    await f.v.input(
      f.v.host.querySelector('[aria-label="称呼"]'),
      "未保存的称呼",
    );
    await f.v.click(button(f.v.host, "取消编辑"));
    assert.ok(document.querySelector("[role=alertdialog]"));
    await f.v.click(
      button(document.querySelector("[role=alertdialog]"), "继续编辑"),
    );
    assert.equal(
      f.v.host.querySelector('[aria-label="称呼"]').value,
      "未保存的称呼",
    );
    await f.v.click(button(f.v.host, "取消编辑"));
    await f.v.click(
      button(document.querySelector("[role=alertdialog]"), "放弃修改"),
    );
    await f.v.flush();
    assert.equal(document.activeElement?.textContent?.trim(), "编辑资料");
    assert.equal(f.writes().length, 0);
    assert.match(f.v.host.textContent, /验收用户/);
  } finally {
    await f.dispose();
  }
});
test("Owner background saves only changed fields with facts CAS and does not turn self-authored facts into learned records", async () => {
  const f = await fixture();
  try {
    await editAbout(f);
    await f.v.input(
      f.v.host.querySelector('[aria-label="称呼"]'),
      "新的自填称呼",
    );
    await f.v.input(f.v.host.querySelector('[aria-label="年龄"]'), "三十多岁");
    await f.v.input(
      f.v.host.querySelector('[aria-label="修改说明"]'),
      "我修正自己的称呼与年龄",
    );
    await choose(f.v, "修改含义", "之前写错了");
    await f.v.click(button(f.v.host, "保存资料"));
    const sent = f.writes()[0];
    assert.match(sent.path, /actor-user-background$/);
    assert.equal(sent.body.expectedCurrentFactsRevision, "facts_gen_5");
    assert.equal("expectedContextVersion" in sent.body, false);
    assert.deepEqual(sent.body.background, {
      name: "新的自填称呼",
      age: "三十多岁",
    });
    assert.equal(sent.body.operation, "correct");
    assert.equal(f.s.actors.get("people-owner").background.location, "上海");
    assert.equal(
      f.s.actors.get("people-owner").background.meeting_confirmed,
      false,
    );
    await f.v.click(button(f.v.host, "查看了解记录"));
    assert.match(dialog().textContent, /最近工作较忙/);
    assert.doesNotMatch(dialog().textContent, /新的自填称呼|三十多岁/);
  } finally {
    await f.dispose();
  }
});
test("actor draft review updates untouched fields while retaining edits and an older Owner detail cannot borrow newer context fact IDs to prove readback", () => {
  const initial = {
      background: { name: "原称呼", location: "原地点", age: "成年" },
      factsRevision: "facts_gen_5",
      context_version: "ctx-old",
    },
    d = helpers.authoredDraft(initial);
  d.fields.name = "保留的输入";
  const latest = {
    ...initial,
    background: { ...initial.background, location: "另一会话的新地点" },
    factsRevision: "facts_gen_6",
    context_version: "ctx-new",
  };
  const rebased = helpers.rebaseAuthoredDraft(d, latest);
  assert.equal(rebased.fields.location, "另一会话的新地点");
  assert.equal(rebased.fields.name, "保留的输入");
  assert.deepEqual(helpers.backgroundPatch(rebased), { name: "保留的输入" });
  const receipt = {
      background: { name: "已保存" },
      facts: [{ attribute: "name", fact_id: "new-fact" }],
      current_facts_revision: "facts_gen_6",
    },
    stale = {
      background: { name: "原称呼" },
      factsRevision: "facts_gen_5",
      facts: [{ attribute: "name", fact_id: "new-fact" }],
      history: [],
    };
  assert.equal(
    helpers.backgroundReadbackProven(
      receipt,
      stale,
      { input: { background: { name: "已保存" } } },
      true,
    ),
    false,
  );
  assert.equal(
    helpers.backgroundReadbackProven(
      receipt,
      {
        ...stale,
        background: receipt.background,
        factsRevision: "facts_gen_6",
      },
      { input: { background: { name: "已保存" } } },
      true,
    ),
    true,
  );
});
test("unknown Owner save resumes the same body/key after remount and retains newer fields", async () => {
  const first = await fixture({ scenario: "unknown" });
  await editAbout(first);
  await first.v.input(
    first.v.host.querySelector('[aria-label="称呼"]'),
    "原提交 A",
  );
  await first.v.input(
    first.v.host.querySelector('[aria-label="修改说明"]'),
    "确认原称呼",
  );
  await first.v.click(button(first.v.host, "保存资料"));
  await first.v.input(
    first.v.host.querySelector('[aria-label="称呼"]'),
    "后续草稿 B",
  );
  const api = first.api,
    original = clone(first.writes()[0].body);
  await first.dispose();
  const next = await fixture({
    api,
    retainStorage: true,
    query: { tab: "you" },
  });
  try {
    await next.v.click(button(next.v.host, "继续核对资料修改"));
    assert.equal(
      next.v.host.querySelector('[aria-label="称呼"]').value,
      "后续草稿 B",
    );
    await next.v.click(button(next.v.host, "核对或重试原提交"));
    assert.deepEqual(next.writes()[1].body, original);
    assert.equal(next.s.factsGeneration, 6);
    assert.equal(
      next.v.host.querySelector('[aria-label="称呼"]').value,
      "后续草稿 B",
    );
    await next.v.click(button(next.v.host, "同步资料，保留输入"));
    await next.v.click(button(next.v.host, "已核对，采用最新基线"));
    await next.v.click(button(next.v.host, "保存资料"));
    assert.equal(
      next.s.actors.get("people-owner").background.name,
      "后续草稿 B",
    );
    assert.equal(next.s.factsGeneration, 7);
  } finally {
    await next.dispose();
  }
});
test("accepted Owner save with a failed readback recovers through GET only", async () => {
  const f = await fixture({ scenario: "readback-failure" });
  try {
    await editAbout(f);
    await f.v.input(
      f.v.host.querySelector('[aria-label="称呼"]'),
      "只读恢复称呼",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="修改说明"]'),
      "保存资料",
    );
    await f.v.click(button(f.v.host, "保存资料"));
    assert.match(f.v.host.textContent, /原操作已有回执/);
    assert.equal(f.writes().length, 1);
    await f.v.click(button(f.v.host, "只读恢复页面结果"));
    assert.equal(f.writes().length, 1);
    assert.equal(f.v.host.querySelector('[aria-label="称呼"]'), null);
    assert.match(f.v.host.textContent, /只读恢复称呼/);
  } finally {
    await f.dispose();
  }
});
test("another human actor uses its context-version contract and a valid free-form age remains a string", async () => {
  const f = await fixture({ query: { actor: "human-other", tab: "you" } });
  try {
    await f.v.click(button(f.v.host, "编辑资料"));
    await f.v.input(f.v.host.querySelector('[aria-label="年龄"]'), "成年");
    await f.v.input(
      f.v.host.querySelector('[aria-label="修改说明"]'),
      "补充交流参与者资料",
    );
    await f.v.click(button(f.v.host, "保存资料"));
    const sent = f.writes()[0];
    assert.match(sent.path, /actor-context\/human-other$/);
    assert.equal(sent.body.background.age, "成年");
    assert.match(sent.body.expectedContextVersion, /ctx_human-other_/);
    assert.equal("expectedCurrentFactsRevision" in sent.body, false);
    assert.equal(f.s.actors.get("people-owner").background.age, "二十多岁");
    assert.equal(
      f.s.actors.get("human-other").background.location,
      "保留的所在地",
    );
  } finally {
    await f.dispose();
  }
});
test("relationship metrics select the active persona and exclude other persona snapshots", async () => {
  const f = await fixture({ query: { tab: "relationship" } });
  try {
    assert.equal(
      f.v.host
        .querySelector('[role=progressbar][aria-label="信任"]')
        .getAttribute("aria-valuenow"),
      "72",
    );
    f.s.seed.personality_runtime.active_profile_id = "focused";
    await f.v.click(f.v.host.querySelector('[aria-label="刷新人物目录"]'));
    assert.equal(
      f.v.host
        .querySelector('[role=progressbar][aria-label="信任"]')
        .getAttribute("aria-valuenow"),
      "91",
    );
  } finally {
    await f.dispose();
  }
});
test("runtime person edit preserves routine, appearance flags and relationships while using NPC revision", async () => {
  const f = await fixture();
  try {
    const oldRoutine = clone(f.s.world.npcs[0].routine),
      relations = clone(f.s.relations);
    await openPerson(f);
    await f.v.click(button(dialog(), "编辑人物资料"));
    await f.v.click(button(dialog(), "取消"));
    await f.v.flush();
    assert.equal(document.activeElement?.textContent?.trim(), "编辑人物资料");
    assert.equal(f.writes().length, 0);
    await f.v.click(button(dialog(), "编辑人物资料"));
    await f.v.input(
      dialog().querySelector('[aria-label="名字"]'),
      "苏禾的新名字",
    );
    await f.v.click(button(dialog(), "保存人物"));
    const sent = f.writes()[0];
    assert.equal(sent.body.expectedRevision, 3);
    assert.deepEqual(sent.body.routine, oldRoutine);
    assert.equal(sent.body.positionMode, "routine");
    assert.equal(sent.body.enabled, true);
    assert.equal("local_overrides" in sent.body, false);
    assert.deepEqual(f.s.relations, relations);
    assert.equal(f.s.world.npcs[0].revision, 4);
    assert.match(dialog().textContent, /苏禾的新名字/);
  } finally {
    await f.dispose();
  }
});
test("saving and publishing a person definition does not create a runtime person; explicit reviewed application creates one without acquaintance", async () => {
  const f = await fixture();
  try {
    await f.v.click(button(f.v.host, "新增世界人物"));
    await f.v.input(
      dialog().querySelector('[aria-label="名字"]'),
      "新来的书友",
    );
    await f.v.input(
      dialog().querySelector('[aria-label="职业 / 身份"]'),
      "装帧设计师",
    );
    await f.v.click(button(dialog(), "保存人物定义"));
    assert.equal(f.s.world.npcs.length, 3);
    assert.equal(f.api.resources.size, 1);
    assert.match(dialog().textContent, /尚未发布/);
    await f.v.click(button(dialog(), "发布此保存版本"));
    await f.v.click(button(dialog(), "确认发布人物定义"));
    assert.equal(f.s.world.npcs.length, 3);
    await f.v.click(button(dialog(), "读取实际应用差异"));
    await f.v.click(button(dialog(), "明确应用到世界"));
    await f.v.click(button(dialog(), "确认应用已发布定义"));
    assert.equal(f.s.world.npcs.length, 4);
    assert.equal(
      f.s.relations.some(
        (r) => r.target_actor_id === f.s.world.npcs.at(-1).actor_id,
      ),
      false,
    );
    assert.match(
      f.v.host.querySelector('[aria-label="查看人物 新来的书友"]').textContent,
      /尚未结识/,
    );
    assert.equal(dialog(), undefined);
    await f.v.click(button(f.v.host, "新增世界人物"));
    await f.v.click(button(dialog(), "创建另一份人物定义"));
    assert.equal(dialog().querySelector('[aria-label="名字"]').value, "");
  } finally {
    await f.dispose();
  }
});
test("actual encounter requires confirmation and current life-context revision before recording event/relationship/memory references", async () => {
  const f = await fixture();
  try {
    await openPerson(f, "陈渡");
    await f.v.click(button(dialog(), "记录实际接触"));
    await choose(f.v, "接触地点", "迟山书店");
    await f.v.click(
      [...dialog().querySelectorAll("label")]
        .find((n) => n.textContent.includes("使用含时区偏移的时间"))
        .querySelector("[role=checkbox]"),
    );
    await f.v.input(
      dialog().querySelector('[aria-label="开始时间"]'),
      new Date(Date.now() - 3600000).toISOString(),
    );
    await f.v.input(
      dialog().querySelector('[aria-label="结束时间"]'),
      new Date(Date.now() - 1800000).toISOString(),
    );
    await f.v.input(
      dialog().querySelector('[aria-label="实际经过"]'),
      "在角色生活中聊了最近读过的书。",
    );
    await f.v.click(button(dialog(), "确认记录实际接触"));
    assert.equal(f.writes().length, 0);
    assert.match(dialog().textContent, /请确认/);
    await f.v.click(dialog().querySelector('[aria-label="确认接触实际发生"]'));
    await f.v.click(button(dialog(), "确认记录实际接触"));
    const sent = f.writes()[0];
    assert.equal(sent.body.expectedLifeContextRevision, "life-people-1");
    assert.equal(sent.body.confirmed, true);
    assert.deepEqual(sent.body.evidenceRefs, ["owner_confirmation"]);
    assert.ok(f.s.world.events[0].id);
    assert.ok(
      f.s.relations.some(
        (r) => r.target_actor_id === "A-actor-npc-1" && r.profile_id === null,
      ),
    );
    assert.match(dialog().textContent, /已经结识/);
  } finally {
    await f.dispose();
  }
});
test("retrying an opportunity changes only its original request state and does not mark the person met", async () => {
  const f = await fixture();
  try {
    await openPerson(f, "陈渡");
    await f.v.click(button(dialog(), "接触、在场与相遇记录"));
    await tab(f.v, "相遇机会", dialog());
    await f.v.click(button(dialog(), "详情"));
    await f.v.click(button(dialog(), "请求重新判断"));
    await f.v.input(
      dialog().querySelector('[aria-label="重新判断说明"]'),
      "重新核对原窗口中的条件",
    );
    await f.v.click(button(dialog(), "请求重新判断"));
    const row = f.s.records.get("A-npc-1").opportunities[0],
      sent = f.writes()[0];
    assert.equal(sent.body.expectedRevision, 4);
    assert.equal(row.status, "pending");
    assert.equal(row.revision, 5);
    assert.equal(
      f.s.relations.some(
        (r) => r.target_actor_id === "A-actor-npc-1" && r.profile_id === null,
      ),
      false,
    );
    assert.match(dialog().textContent, /等待判断/);
  } finally {
    await f.dispose();
  }
});
test("loading, failure recovery and confirmed empty states stay distinct in all primary people tabs", async () => {
  let release;
  const gate = new Promise((r) => (release = r));
  let first = true;
  const f = await fixture({
    intercept: async ({ url }) => {
      if (first && url.pathname.endsWith("/npcs")) {
        first = false;
        await gate;
      }
    },
  });
  try {
    assert.match(f.v.host.textContent, /正在读取世界人物/);
    assert.doesNotMatch(f.v.host.textContent, /这个世界还没有已读人物/);
    release();
    await f.v.flush();
    assert.equal(f.v.host.querySelectorAll(".companion-person-card").length, 2);
  } finally {
    release();
    await f.dispose();
  }
  const bad = await fixture({ scenario: "failure" });
  try {
    await bad.v.click(button(bad.v.host, "重新读取世界人物"));
    assert.equal(
      bad.v.host.querySelectorAll(".companion-person-card").length,
      2,
    );
    assert.equal(bad.writes().length, 0);
  } finally {
    await bad.dispose();
  }
  const empty = await fixture({ scenario: "empty" });
  try {
    assert.match(empty.v.host.textContent, /这个世界还没有已读人物/);
    await tab(empty.v, "关于你");
    assert.match(empty.v.host.textContent, /还没有填写/);
    await tab(empty.v, "你们的关系");
    assert.equal(empty.v.host.querySelector("[role=progressbar]"), null);
  } finally {
    await empty.dispose();
  }
});
test("a late Owner acknowledgement cannot replace another actor view and restores by reading when the original target returns", async () => {
  let release,
    first = true;
  const gate = new Promise((r) => (release = r));
  const f = await fixture({
    query: { tab: "you" },
    intercept: async ({ method, url }) => {
      if (
        first &&
        method === "PUT" &&
        url.pathname.endsWith("actor-user-background")
      ) {
        first = false;
        await gate;
      }
    },
  });
  try {
    await f.v.click(button(f.v.host, "编辑资料"));
    await f.v.input(
      f.v.host.querySelector('[aria-label="称呼"]'),
      "原Owner已提交",
    );
    await f.v.input(
      f.v.host.querySelector('[aria-label="修改说明"]'),
      "保存原Owner资料",
    );
    await f.v.click(button(f.v.host, "保存资料"));
    await f.v.update({ route: route({ tab: "you", actor: "human-other" }) });
    await act(async () => release());
    await f.v.flush();
    assert.match(f.v.host.textContent, /另一位交流参与者/);
    assert.doesNotMatch(f.v.host.textContent, /原Owner已提交/);
    await f.v.update({ route: route({ tab: "you" }) });
    await f.v.click(button(f.v.host, "继续核对资料修改"));
    await f.v.click(button(f.v.host, "只读恢复页面结果"));
    assert.equal(f.writes().length, 1);
    assert.match(f.v.host.textContent, /原Owner已提交/);
    assert.equal(!!button(f.v.host, "继续核对资料修改"), false);
  } finally {
    release();
    await f.dispose();
  }
});
test("incomplete encounter receipts retain the original command until every promised reference is confirmed", async () => {
  let incomplete = true;
  const f = await fixture({
    intercept: async ({ method, url, body, api }) => {
      if (
        incomplete &&
        method === "POST" &&
        url.pathname.endsWith("/encounters")
      ) {
        incomplete = false;
        let result;
        await api.handler({
          path: url.pathname,
          url,
          method,
          body,
          json: (_res, value) => (result = value),
        });
        delete result.memory_id;
        return new Response(JSON.stringify(result), {
          status: 200,
          headers: { "content-type": "application/json" },
        });
      }
    },
  });
  try {
    await openPerson(f, "陈渡");
    await f.v.click(button(dialog(), "记录实际接触"));
    await choose(f.v, "接触地点", "迟山书店");
    await f.v.click(
      [...dialog().querySelectorAll("label")]
        .find((n) => n.textContent.includes("使用含时区偏移的时间"))
        .querySelector("[role=checkbox]"),
    );
    await f.v.input(
      dialog().querySelector('[aria-label="开始时间"]'),
      new Date(Date.now() - 3600000).toISOString(),
    );
    await f.v.input(
      dialog().querySelector('[aria-label="结束时间"]'),
      new Date(Date.now() - 1800000).toISOString(),
    );
    await f.v.input(
      dialog().querySelector('[aria-label="实际经过"]'),
      "一次已确认的角色生活接触。",
    );
    await f.v.click(dialog().querySelector('[aria-label="确认接触实际发生"]'));
    await f.v.click(button(dialog(), "确认记录实际接触"));
    assert.match(dialog().textContent, /实际接触回执尚未完整确认/);
    assert.equal(f.s.records.get("A-npc-1").encounters.length, 2);
    assert.equal(f.s.memories.size, 1);
    await f.v.click(button(dialog(), "核对或重试原提交"));
    assert.equal(f.s.memories.size, 1);
    assert.equal(f.s.records.get("A-npc-1").encounters.length, 2);
    assert.deepEqual(f.writes()[0].body, f.writes()[1].body);
    assert.match(dialog().textContent, /已经结识/);
  } finally {
    await f.dispose();
  }
});
test("a superseded application confirms the original receipt but reports the actual later source instead of claiming an exact current match", async () => {
  let supersede = true;
  const f = await fixture({
    intercept: async ({ method, url, body, api }) => {
      if (supersede && method === "POST" && url.pathname.endsWith("/apply")) {
        supersede = false;
        let receipt;
        await api.handler({
          path: url.pathname,
          url,
          method,
          body,
          json: (_res, value) => (receipt = value),
        });
        const npc = api.state("A").world.npcs.find((n) => n.id === receipt.id);
        npc.source_version = "v2";
        npc.revision++;
        npc.name = "后续应用的名字";
        return new Response(JSON.stringify(receipt), {
          status: 200,
          headers: { "content-type": "application/json" },
        });
      }
    },
  });
  try {
    await f.v.click(button(f.v.host, "新增世界人物"));
    await f.v.input(
      dialog().querySelector('[aria-label="名字"]'),
      "原版本人物",
    );
    await f.v.click(button(dialog(), "保存人物定义"));
    await f.v.click(button(dialog(), "发布此保存版本"));
    await f.v.click(button(dialog(), "确认发布人物定义"));
    await f.v.click(button(dialog(), "读取实际应用差异"));
    await f.v.click(button(dialog(), "明确应用到世界"));
    await f.v.click(button(dialog(), "确认应用已发布定义"));
    assert.match(dialog().textContent, /当前人物已有后续应用/);
    assert.match(dialog().textContent, /运行来源是 v2/);
    assert.equal(f.s.world.npcs.length, 4);
    assert.equal(f.writes().filter((c) => c.path.endsWith("/apply")).length, 1);
  } finally {
    await f.dispose();
  }
});
test("person management preserves referenced actors through archive and deletes only unreferenced actors with their revision", async () => {
  const f = await fixture();
  try {
    await choose(f.v, "人物范围", "已归档人物");
    await openPerson(f, "旧同事");
    await f.v.click(button(dialog(), "人物管理"));
    await f.v.click(button(dialog(), "移除无引用人物"));
    await f.v.input(
      dialog().querySelector('[aria-label="人物移除说明"]'),
      "移除没有历史引用的归档人物",
    );
    await f.v.click(button(dialog(), "确认移除人物"));
    assert.equal(f.s.world.npcs.length, 2);
    assert.equal(f.writes().at(-1).body.expectedRevision, 3);
    assert.equal(dialog(), undefined);
    await choose(f.v, "人物范围", "全部人物");
    await openPerson(f);
    await f.v.click(button(dialog(), "人物管理"));
    await f.v.click(button(dialog(), "移除无引用人物"));
    await f.v.input(
      dialog().querySelector('[aria-label="人物移除说明"]'),
      "核对已有引用的人物",
    );
    await f.v.click(button(dialog(), "确认移除人物"));
    assert.match(dialog().textContent, /历史引用/);
    await f.v.click(button(dialog(), "取消"));
    await f.v.click(button(dialog(), "编辑出现、位置规律与归档"));
    await f.v.click(button(dialog(), "生活规律、出现与归档"));
    await choose(f.v, "人物状态", "归档并保留历史");
    const relationIds = f.s.relations.map((r) => r.id);
    await f.v.click(button(dialog(), "保存人物"));
    assert.equal(f.s.world.npcs[0].status, "archived");
    assert.deepEqual(
      f.s.relations.map((r) => r.id),
      relationIds,
    );
  } finally {
    await f.dispose();
  }
});
