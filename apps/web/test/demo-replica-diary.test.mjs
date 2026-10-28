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
const { default: Diary } = await vite.ssrLoadModule(
  "/src/react-app/pages/DiaryPage.jsx",
);
const button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () =>
    document.querySelector('[data-slot="workspace-command-inline"]') ||
    [...document.querySelectorAll("[role=dialog]")].at(-1),
  clone = (v) => JSON.parse(JSON.stringify(v));
const entry = (id, date = "2026-10-26") => ({
  id,
  fluctlight_id: "A",
  date,
  timezone: "Asia/Shanghai",
  status: "ready",
  body: "今天看了一本书。\n\n把平常的日子认真记下来。",
  source: "agent",
  revision: 3,
  active_attempt_id: "attempt-" + id,
  annotation: { note: "原来的批注", favorite: false, revision: 7 },
  attempts: [],
  created_at: date + "T14:00:00Z",
});
function workspace() {
  sessionStorage.clear();
  const state = proxy({
      session: { actorId: "diary-owner" },
      selectedId: "A",
      characters: [
        { id: "A", name: "林青", timezone: "Asia/Shanghai", status: "正常" },
      ],
    }),
    rows = new Map(
      [entry("D"), entry("older", "2026-10-25")].map((e) => [e.id, e]),
    ),
    calls = [],
    replies = [];
  const client = {
    listDiary: async (fid, cursor, options) => {
      calls.push(["list", fid, cursor, clone(options)]);
      return {
        items: clone(
          [...rows.values()].filter(
            (row) => !options.favorite || row.annotation.favorite,
          ),
        ),
        next_cursor: "",
        timezone: "Asia/Shanghai",
      };
    },
    diary: async (fid, id) => {
      calls.push(["detail", id]);
      return clone(rows.get(id));
    },
    diaryReplies: async () => ({ items: clone(replies), next_cursor: "" }),
    sharedExperiences: async () => ({ items: [], next_cursor: "" }),
    annotateDiary: async (fid, id, body) => {
      calls.push(["annotation", fid, id, clone(body)]);
      const row = rows.get(id);
      row.annotation = {
        note: body.note,
        favorite: body.favorite,
        revision: row.annotation.revision + 1,
      };
      return { diary_id: id, ...row.annotation };
    },
    replyToDiary: async (fid, id, body) => {
      calls.push(["reply", fid, id, clone(body)]);
      const request = {
        id: "reply-request",
        fluctlight_id: fid,
        status: "pending",
        revision: 1,
        payload: { text: body.text },
        created_at: "2026-10-27T00:00:00Z",
      };
      replies.push(request);
      return { request };
    },
    generateDiary: async (fid, body) => {
      calls.push(["generate", fid, clone(body)]);
      const row = {
        ...entry("generated", body.localDate),
        status: "running",
        body: null,
        timezone: body.timezone,
        revision: 1,
        annotation: { note: "", favorite: false, revision: 0 },
      };
      rows.set(row.id, row);
      return {
        diary_id: row.id,
        fluctlight_id: fid,
        status: "pending",
        attempt: 1,
      };
    },
  };
  const w = {
    state,
    rows,
    calls,
    client,
    current: {
      get value() {
        return state.characters[0];
      },
    },
    currentScope: () =>
      JSON.stringify([state.session.actorId, state.selectedId]),
    isCurrent: (owner, fid) =>
      owner === state.session.actorId && fid === state.selectedId,
    subscribe: (fn) => subscribe(state, fn),
    getSnapshot: () => snapshot(state),
    navigatePath: (path) => calls.push(["navigate", path]),
    services: { lifeClient: { request: async () => clone(replies[0]) } },
  };
  return w;
}
const mount = (w) => mountReact(vite, Diary, { workspace: w });
async function tab(v, label) {
  await act(async () =>
    [...v.host.querySelectorAll("[role=tab]")]
      .find((n) => n.textContent.startsWith(label))
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
test("Diary initial read selects a real entry exactly once, preserves original body and honest empty/search states", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    assert.equal(w.calls.filter((c) => c[0] === "list").length, 1);
    assert.match(
      v.host.querySelector('[aria-label="日记正文"]').textContent,
      /今天看了一本书/,
    );
    assert.doesNotMatch(v.host.textContent, /写于这一天的夜里/);
    await v.input(
      v.host.querySelector('[aria-label="搜索日记"]'),
      "未找到的片段",
    );
    assert.match(
      v.host.querySelector('[aria-label="日记目录"]').textContent,
      /没有找到这段回忆/,
    );
    assert.match(
      v.host.querySelector('[aria-label="日记正文"]').textContent,
      /今天看了一本书/,
    );
    await tab(v, "共同经历");
    assert.match(v.host.textContent, /共同的经历，慢慢积累/);
  } finally {
    await v.dispose();
  }
  const empty = workspace();
  empty.rows.clear();
  const e = await mount(empty);
  try {
    assert.match(e.host.textContent, /日记本还是空的/);
    assert.equal(e.host.querySelector('[aria-label="日记正文"]'), null);
  } finally {
    await e.dispose();
  }
});
test("annotation cancellation discards unsaved input; save uses annotation revision and leaves diary prose unchanged", async () => {
  const w = workspace(),
    v = await mount(w),
    body = w.rows.get("D").body;
  try {
    await v.click(button(v.host, "修改批注"));
    await v.input(
      dialog().querySelector('[aria-label="日记批注"]'),
      "取消的文字",
    );
    await v.click(button(dialog(), "取消"));
    assert.equal(w.rows.get("D").annotation.note, "原来的批注");
    await v.click(button(v.host, "修改批注"));
    assert.equal(
      dialog().querySelector('[aria-label="日记批注"]').value,
      "原来的批注",
    );
    await v.input(
      dialog().querySelector('[aria-label="日记批注"]'),
      "  保存空格与正文分隔\n新的感受  ",
    );
    await v.click(button(dialog(), "保存批注"));
    const sent = w.calls.find((c) => c[0] === "annotation")[3];
    assert.equal(sent.expectedRevision, 7);
    assert.equal(sent.note, "  保存空格与正文分隔\n新的感受  ");
    assert.ok(sent.idempotencyKey);
    assert.equal(w.rows.get("D").body, body);
    assert.match(v.host.textContent, /批注已保存/);
  } finally {
    await v.dispose();
  }
});
test("changing favorite keeps both previously loaded directory caches consistent", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await v.click(
      v.host.querySelector(".replica-diary-options [role=checkbox]"),
    );
    assert.match(v.host.textContent, /日记本还是空的/);
    await v.click(
      v.host.querySelector(".replica-diary-options [role=checkbox]"),
    );
    await v.click(button(v.host, "修改批注"));
    await v.click(dialog().querySelector("[role=checkbox]"));
    await v.click(button(dialog(), "保存批注"));
    await v.click(
      v.host.querySelector(".replica-diary-options [role=checkbox]"),
    );
    assert.equal(v.host.querySelectorAll(".lp-diary-date").length, 1);
    assert.match(
      v.host.querySelector('[aria-label="日记正文"]').textContent,
      /今天看了一本书/,
    );
  } finally {
    await v.dispose();
  }
});
test("annotation conflict requires explicit latest baseline review while preserving draft and a new reviewed key", async () => {
  const w = workspace(),
    write = w.client.annotateDiary;
  let attempts = 0;
  w.client.annotateDiary = async (fid, id, body) => {
    if (++attempts === 1) {
      w.calls.push(["rejected", clone(body)]);
      w.rows.get(id).annotation.revision = 8;
      throw Object.assign(Error("批注版本变化"), {
        status: 409,
        code: "diary_annotation_conflict",
      });
    }
    return write(fid, id, body);
  };
  const v = await mount(w);
  try {
    await v.click(button(v.host, "修改批注"));
    await v.input(
      dialog().querySelector('[aria-label="日记批注"]'),
      "保留我的批注",
    );
    await v.click(button(dialog(), "保存批注"));
    await v.click(button(dialog(), "读取最新版本并保留输入"));
    await v.click(button(dialog(), "已核对，采用最新基线"));
    await v.click(button(dialog(), "保存批注"));
    const sent = w.calls.find((c) => c[0] === "annotation")[3];
    assert.equal(sent.expectedRevision, 8);
    assert.equal(sent.note, "保留我的批注");
    assert.notEqual(
      sent.idempotencyKey,
      w.calls.find((c) => c[0] === "rejected")[1].idempotencyKey,
    );
  } finally {
    await v.dispose();
  }
});
test("an acknowledged annotation read failure recovers by reading only", async () => {
  const w = workspace(),
    read = w.client.diary;
  let fail = false;
  w.client.diary = async (...args) => {
    if (fail) throw Error("读回断网");
    return read(...args);
  };
  const v = await mount(w);
  try {
    await v.click(button(v.host, "修改批注"));
    await v.input(
      dialog().querySelector('[aria-label="日记批注"]'),
      "已经提交",
    );
    fail = true;
    await v.click(button(dialog(), "保存批注"));
    assert.match(dialog().textContent, /原操作已有回执/);
    fail = false;
    await v.click(button(dialog(), "只读恢复页面结果"));
    assert.equal(w.calls.filter((c) => c[0] === "annotation").length, 1);
    assert.match(v.host.textContent, /已经提交/);
  } finally {
    await v.dispose();
  }
});
test("reply uses diary day revision and displays the real pending request without inventing a conversation answer", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await v.click(button(v.host, "读完后，想对林青说"));
    await v.input(
      dialog().querySelector('[aria-label="日记留言"]'),
      "我也喜欢这段平常的日子。",
    );
    await v.click(button(dialog(), "发送留言"));
    const sent = w.calls.find((c) => c[0] === "reply")[3];
    assert.equal(sent.expectedRevision, 3);
    assert.equal(sent.text, "我也喜欢这段平常的日子。");
    assert.ok(sent.idempotencyKey);
    assert.match(v.host.textContent, /等待回应/);
    assert.doesNotMatch(v.host.textContent, /回复已生成|已经回答/);
    assert.equal(w.rows.get("D").body, entry("D").body);
  } finally {
    await v.dispose();
  }
});
test("generation rejects an unclosed date and observes actual ready/no_content vocabulary rather than a fake completed body", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await v.click(button(v.host, "整理一篇日记"));
    await v.input(
      dialog().querySelector('[aria-label="日记整理时区"]'),
      "Pacific/Kiritimati",
    );
    const dateButton = dialog().querySelector('[aria-label="日记整理日期"]');
    assert.ok(dateButton);
    await v.click(dateButton);
    const future = new Date();
    future.setDate(future.getDate() + 2);
    await v.click(
      [...document.querySelectorAll("button[data-day]")].find(
        (n) => n.dataset.day === future.toLocaleDateString("zh-CN"),
      ),
    );
    await v.click(button(dialog(), "请求整理日记"));
    assert.equal(
      w.calls.some((c) => c[0] === "generate"),
      false,
    );
    assert.match(dialog().textContent, /请选择该时区已经结束的一天/);
    await v.click(dialog().querySelector('[aria-label="日记整理日期"]'));
    const past = new Date();
    past.setDate(past.getDate() - 2);
    await v.click(
      [...document.querySelectorAll("button[data-day]")].find(
        (n) => n.dataset.day === past.toLocaleDateString("zh-CN"),
      ),
    );
    await v.click(button(dialog(), "请求整理日记"));
    const sent = w.calls.find((c) => c[0] === "generate");
    assert.ok(sent);
    assert.equal(sent[2].timezone, "Pacific/Kiritimati");
    assert.ok(sent[2].idempotencyKey);
    assert.equal(w.rows.get("generated").body, null);
    assert.match(v.host.textContent, /正在回顾|正在跟进/);
    assert.doesNotMatch(
      v.host.textContent,
      /把这些片段记下来，留给以后慢慢翻看/,
    );
    w.rows.get("generated").status = "no_content";
    await act(async () =>
      document.dispatchEvent(new window.Event("visibilitychange")),
    );
    await v.flush();
    assert.match(v.host.textContent, /没有形成正文/);
    assert.equal(w.calls.filter((c) => c[0] === "generate").length, 1);
  } finally {
    await v.dispose();
  }
});
test("favorite and shared pagination retry their own saved cursors, and a late detail cannot change the selected date", async () => {
  const w = workspace(),
    base = w.client.listDiary;
  let fail = true,
    release;
  w.rows.get("D").annotation.favorite = true;
  w.client.listDiary = async (fid, cursor, o) => {
    if (cursor) {
      w.calls.push(["cursor", cursor, clone(o)]);
      if (fail) throw Error("收藏翻页暂不可读");
      return { items: [], next_cursor: "" };
    }
    const result = await base(fid, cursor, o);
    if (o.favorite) result.next_cursor = "favorite-next";
    return result;
  };
  const read = w.client.diary;
  w.client.diary = async (fid, id) =>
    id === "older"
      ? new Promise((resolve) => (release = () => read(fid, id).then(resolve)))
      : read(fid, id);
  const v = await mount(w);
  try {
    await v.click(
      v.host.querySelector(".replica-diary-options [role=checkbox]"),
    );
    await v.click(button(v.host, "加载更早日记"));
    assert.match(v.host.textContent, /收藏翻页暂不可读/);
    fail = false;
    await v.click(button(v.host, "重新读取日记目录"));
    assert.deepEqual(
      w.calls.filter((c) => c[0] === "cursor").map((c) => c[1]),
      ["favorite-next", "favorite-next"],
    );
    await v.click(
      v.host.querySelector(".replica-diary-options [role=checkbox]"),
    );
    const dates = v.host.querySelectorAll(".lp-diary-date");
    await v.click(dates[1]);
    await v.click(dates[0]);
    await act(async () => release());
    await v.flush();
    assert.match(
      v.host.querySelector('[aria-label="日记正文"]').textContent,
      /2026 \/ 10 \/ 26/,
    );
  } finally {
    await v.dispose();
  }
});
