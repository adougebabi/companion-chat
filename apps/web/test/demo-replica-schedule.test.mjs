import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { React, act, installDOM, mountReact } from "./helpers/react-dom.mjs";
const env = installDOM(),
  vite = await createServer({
    root: fileURLToPath(new URL("../", import.meta.url)),
    appType: "custom",
    logLevel: "silent",
    server: { middlewareMode: true },
  });
test.after(async () => {
  await vite.close();
  env.restore();
});
const { default: Schedule } = await vite.ssrLoadModule(
  "/src/react-app/pages/SchedulePage.jsx",
);
const { default: Status } = await vite.ssrLoadModule(
  "/src/react-app/shared/RequestStatus.jsx",
);
const button = (root, text) =>
  [...root.querySelectorAll("button")].find(
    (node) => node.textContent.trim() === text,
  );
const today = "2026-10-27",
  future = "2026-10-28";
function workspace() {
  const calls = [],
    state = proxy({
      session: { actorId: "schedule-owner" },
      selectedId: "A",
      characters: [
        { id: "A", name: "林青", status: "正常", timezone: "Asia/Shanghai" },
      ],
      route: { query: new URLSearchParams() },
      operations: [],
    });
  const item = {
    id: "slot",
    activity: "已确认的阅读",
    start_at: today + "T09:00:12.123+08:00",
    end_at: today + "T10:00:00+08:00",
    scene: "休息",
    location: "家中",
    status: "scheduled",
    action_status: null,
  };
  const records = new Map([
    [
      today,
      {
        id: "schedule",
        local_date: today,
        timezone: "Asia/Shanghai",
        revision: 3,
        items: [item],
      },
    ],
  ]);
  let pending = null;
  const request = {
    id: "request-one",
    fluctlight_id: "A",
    kind: "schedule_item_suggestion",
    status: "pending",
    payload: { activity: "建议活动" },
    revision: 1,
    created_at: today + "T00:00:00Z",
    result: {},
    result_refs: [],
  };
  const life = {
    detail: async () =>
      proxy({
        id: "A",
        status: "active",
        context: { context_revision: "context-3" },
        schedule: records.get(today),
        life_profile: { life_habits: "睡前阅读。" },
        extension: { schema_id: "keep-this-field", text: "原文\n保留" },
      }),
    schedules: async (fid, options = {}) => {
      calls.push(["read", fid, { ...options }]);
      const date = options.from || today;
      return {
        today,
        from: date,
        to: date,
        timezone: "Asia/Shanghai",
        context_revision: "context-" + date,
        days: [
          {
            local_date: date,
            schedule: records.get(date) || null,
            request: null,
          },
        ],
      };
    },
    requests: async () => ({ items: [], next_cursor: "" }),
    request: async () => request,
    scheduleItemPending: () => pending,
    schedulePending: () => null,
    scheduleItem: async (...args) => {
      calls.push(["item", ...args]);
      return { request };
    },
    retryScheduleItem: async () => {
      calls.push(["retry-item"]);
      pending = null;
      return { request };
    },
    submitSchedule: async (...args) => {
      calls.push(["plan", ...args]);
      return {
        request_id: "plan-one",
        fluctlight_id: "A",
        local_date: args[1].localDate,
        status: "queued",
      };
    },
    scheduleRequest: async () => ({
      request_id: "plan-one",
      fluctlight_id: "A",
      local_date: future,
      status: "completed",
      result: { disposition: "preserved" },
    }),
  };
  return {
    state,
    calls,
    records,
    item,
    request,
    life,
    setPending: (value) => (pending = value),
    current: {
      get value() {
        return state.characters[0];
      },
    },
    services: { lifeClient: life },
    detail: () => life.detail("A"),
    subscribe: (fn) => subscribe(state, fn),
    getSnapshot: () => snapshot(state),
    isCurrent: (owner, fid) =>
      owner === state.session.actorId && fid === state.selectedId,
    navigatePath: (path) => calls.push(["navigate", path]),
    clock: (value) => value?.slice(11, 16) || "",
    currentScope: () =>
      JSON.stringify([state.session.actorId, state.selectedId]),
  };
}
async function mount(w, date = today) {
  return mountReact(vite, Schedule, {
    workspace: w,
    props: { route: { query: new URLSearchParams({ date }) } },
  });
}
test("schedule reading/edit cancellation preserve the saved record; submission carries frozen version, source item and exact time", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    assert.match(v.host.textContent, /已确认的阅读/);
    assert.equal(v.host.querySelector(".lp-timeline-card.current"), null);
    await v.click(v.host.querySelector(".lp-timeline-card"));
    let dialog = document.querySelector("[role=dialog]");
    await v.click(button(dialog, "提出修改"));
    await v.input(
      dialog.querySelector('[aria-label="建议活动内容"]'),
      "未提交的修改",
    );
    await v.click(button(dialog, "取消修改"));
    assert.match(dialog.textContent, /已确认的阅读/);
    assert.equal(
      w.calls.some((call) => call[0] === "item"),
      false,
    );
    await v.click(button(dialog, "提出修改"));
    assert.equal(
      dialog.querySelector('[aria-label="建议活动内容"]').value,
      "已确认的阅读",
    );
    await v.input(
      dialog.querySelector('[aria-label="建议活动内容"]'),
      "建议调整的阅读",
    );
    await v.input(
      dialog.querySelector('[aria-label="日程建议原因"]'),
      "想留出更安静的时间",
    );
    await v.click(button(dialog, "提交调整建议"));
    const sent = w.calls.find((call) => call[0] === "item");
    assert.equal(sent[1], "A");
    assert.equal(sent[2].schedule.revision, 3);
    assert.equal(sent[2].extension.schema_id, "keep-this-field");
    assert.equal(sent[2].extension.text, "原文\n保留");
    assert.equal(sent[2].context.context_revision, "context-3");
    assert.equal(sent[3].id, "slot");
    assert.equal(sent[4].startAt, w.item.start_at);
    assert.equal(sent[4].activity, "建议调整的阅读");
    assert.equal(sent[5], "update");
    assert.equal(w.item.activity, "已确认的阅读");
    assert.match(
      document.querySelector("[role=dialog]").textContent,
      /日程建议处理进度/,
    );
  } finally {
    await v.dispose();
  }
});
test("schedule failed refresh retains the correct date; an older date response cannot overwrite a newer selection", async () => {
  const w = workspace(),
    base = w.life.schedules;
  let fail = false,
    release;
  w.life.schedules = async (fid, options) => {
    if (fail) throw Error("日程读取失败");
    if (options.from === future)
      return new Promise((resolve) => (release = resolve));
    return base(fid, options);
  };
  const v = await mount(w);
  try {
    fail = true;
    await v.click(button(v.host, "刷新"));
    assert.match(v.host.textContent, /已确认的阅读/);
    assert.match(v.host.textContent, /日程读取失败/);
    fail = false;
    await v.click(button(v.host, "重新读取这一天"));
    await v.update({ route: { query: new URLSearchParams({ date: future }) } });
    await v.update({
      route: { query: new URLSearchParams({ date: "2026-10-29" }) },
    });
    await act(async () =>
      release({
        today,
        from: future,
        to: future,
        timezone: "Asia/Shanghai",
        context_revision: "old",
        days: [
          {
            local_date: future,
            schedule: {
              id: "late",
              revision: 1,
              items: [{ ...w.item, id: "wrong", activity: "过期日期的内容" }],
            },
          },
        ],
      }),
    );
    await v.flush();
    assert.match(v.host.textContent, /2026-10-29/);
    assert.doesNotMatch(v.host.textContent, /过期日期的内容/);
    assert.match(v.host.textContent, /空白的一页/);
  } finally {
    await v.dispose();
  }
});
test("planning protects a present empty schedule and re-reads an edited target date before admission", async () => {
  const w = workspace();
  w.records.get(today).items = [];
  const v = await mount(w);
  try {
    await v.click(button(v.host, "规划日程"));
    let dialog = document.querySelector("[role=dialog]");
    assert.equal(button(dialog, "请求安排日程").disabled, true);
    assert.match(dialog.textContent, /已有日程记录/);
    await v.click(dialog.querySelector('[aria-label="规划日期"]'));
    const day = [...document.querySelectorAll("button[data-day]")].find(
      (node) =>
        node.getAttribute("data-day") ===
        new Date(2026, 9, 29, 12).toLocaleDateString("zh-CN"),
    );
    await v.click(day);
    await v.input(
      dialog.querySelector('[aria-label="安排说明"]'),
      "为这一天留出生活安排",
    );
    await v.click(button(dialog, "请求安排日程"));
    const sent = w.calls.find((call) => call[0] === "plan");
    assert.equal(sent[2].localDate, "2026-10-29");
    assert.equal(sent[2].expectedLifeContextRevision, "context-2026-10-29");
    assert.equal(sent[2].expectedRevision, 0);
    assert.ok(
      w.calls.some(
        (call) => call[0] === "read" && call[2].from === "2026-10-29",
      ),
    );
    assert.match(
      document.querySelector("[role=dialog]").textContent,
      /原安排已保留/,
    );
  } finally {
    await v.dispose();
  }
});
test("an unknown item admission is recovered after remount through the original provider operation, never a fresh suggestion", async () => {
  const w = workspace();
  let attempts = 0;
  w.life.scheduleItem = async (...args) => {
    attempts++;
    w.setPending({
      idempotencyKey: "original-key",
      value: {
        operation: "update",
        scheduleId: "schedule",
        itemId: "slot",
        request: { ...args[3] },
      },
    });
    throw Error("回执未确认");
  };
  let v = await mount(w);
  try {
    await v.click(v.host.querySelector(".lp-timeline-card"));
    let dialog = document.querySelector("[role=dialog]");
    await v.click(button(dialog, "提出修改"));
    await v.input(
      dialog.querySelector('[aria-label="日程建议原因"]'),
      "原建议原因",
    );
    await v.click(button(dialog, "提交调整建议"));
    assert.match(dialog.textContent, /回执未确认/);
    assert.equal(button(dialog, "提交调整建议").disabled, true);
    await v.dispose();
    v = await mount(w);
    assert.match(v.host.textContent, /继续上次日程建议/);
    await v.click(button(v.host, "查询或重试原提交"));
    assert.equal(attempts, 1);
    assert.equal(w.calls.filter((call) => call[0] === "retry-item").length, 1);
    assert.equal(w.life.scheduleItemPending(), null);
  } finally {
    await v.dispose();
  }
});
test("leaving the Owner scope during the preflight read prevents plan admission", async () => {
  const w = workspace(),
    v = await mount(w, future);
  let release;
  try {
    await v.click(button(v.host, "规划日程"));
    const dialog = document.querySelector("[role=dialog]");
    await v.input(
      dialog.querySelector('[aria-label="安排说明"]'),
      "原账户的说明",
    );
    w.life.schedules = () => new Promise((resolve) => (release = resolve));
    await v.click(button(dialog, "请求安排日程"));
    w.state.session.actorId = "another-owner";
    await act(async () =>
      release({
        today,
        from: future,
        to: future,
        timezone: "Asia/Shanghai",
        context_revision: "new-context",
        days: [{ local_date: future, schedule: null }],
      }),
    );
    assert.equal(
      w.calls.some((call) => call[0] === "plan"),
      false,
    );
  } finally {
    await v.dispose();
  }
});
test("request observer retains confirmed status through a failed refresh and never turns accepted intention into completed action", async () => {
  const w = workspace();
  w.request.status = "accepted";
  w.request.decision_reason =
    "An intention was recorded; fulfillment requires later evidence";
  let fail = false;
  w.life.request = async () => {
    if (fail) throw Error("请求暂不可读");
    return w.request;
  };
  const v = await mountReact(vite, Status, {
    workspace: w,
    props: { fid: "A", requestId: w.request.id },
  });
  try {
    assert.match(v.host.textContent, /等待实际行动/);
    assert.match(v.host.textContent, /后续实际证据/);
    fail = true;
    await v.click(button(v.host, "刷新原请求"));
    assert.match(v.host.textContent, /保留上次状态/);
    assert.match(v.host.textContent, /等待实际行动/);
  } finally {
    await v.dispose();
  }
});
test("a first rejected plan requires explicit latest-date review and preserves the original reason", async () => {
  const w = workspace();
  let original = null,
    rebaseArgs;
  w.life.schedulePending = () => original;
  w.life.submitSchedule = async (fid, body) => {
    original = {
      idempotencyKey: "rejected-key",
      attempts: 1,
      rejection: { code: "revision_conflict" },
      value: structuredClone(body),
    };
    throw Object.assign(Error("版本变化"), {
      status: 409,
      code: "revision_conflict",
    });
  };
  w.life.rebaseSchedule = async (...args) => {
    rebaseArgs = args;
    original = null;
    return {
      request_id: "rebased-plan",
      fluctlight_id: "A",
      local_date: future,
      status: "queued",
    };
  };
  const v = await mount(w, future);
  try {
    await v.click(button(v.host, "规划日程"));
    let dialog = document.querySelector("[role=dialog]");
    await v.input(
      dialog.querySelector('[aria-label="安排说明"]'),
      "原规划说明",
    );
    await v.click(button(dialog, "请求安排日程"));
    assert.match(dialog.textContent, /原规划因版本变化未被接受/);
    assert.equal(button(dialog, "查询或重试原规划"), undefined);
    await v.click(button(dialog, "保留原规划，核对最新版本"));
    const review = [...document.querySelectorAll("[role=dialog]")].at(-1);
    assert.match(review.textContent, /原规划说明/);
    assert.equal(rebaseArgs, undefined);
    await v.click(button(review, "保留原规划并重新提交"));
    assert.equal(rebaseArgs[0], "A");
    assert.equal(rebaseArgs[1], future);
    assert.equal(rebaseArgs[2].context_revision, "context-" + future);
    assert.equal(rebaseArgs[3], "rejected-key");
  } finally {
    await v.dispose();
  }
});
test("selecting a date with an unknown plan restores that date original input before offering retry", async () => {
  const w = workspace(),
    target = "2026-10-29",
    original = {
      idempotencyKey: "unknown-date-key",
      value: { localDate: target, reason: "目标日期的原说明" },
    };
  w.life.schedulePending = (_fid, date) => (date === target ? original : null);
  const v = await mount(w, future);
  try {
    await v.click(button(v.host, "规划日程"));
    const dialog = document.querySelector("[role=dialog]");
    await v.input(
      dialog.querySelector('[aria-label="安排说明"]'),
      "另一天的新说明",
    );
    await v.click(dialog.querySelector('[aria-label="规划日期"]'));
    await v.click(
      [...document.querySelectorAll("button[data-day]")].find(
        (node) =>
          node.dataset.day ===
          new Date(2026, 9, 29, 12).toLocaleDateString("zh-CN"),
      ),
    );
    assert.equal(
      dialog.querySelector('[aria-label="安排说明"]').value,
      "目标日期的原说明",
    );
    assert.equal(
      dialog.querySelector('[aria-label="安排说明"]').disabled,
      true,
    );
    assert.ok(button(dialog, "查询或重试原规划"));
  } finally {
    await v.dispose();
  }
});
test("awaiting goal binding remains observable and does not notify completion before a later real result", async () => {
  const w = workspace();
  let completed = 0;
  w.request.status = "awaiting_goal";
  w.request.decision_reason = "等待目标规划";
  const v = await mountReact(vite, Status, {
    workspace: w,
    props: { fid: "A", requestId: w.request.id, onTerminal: () => completed++ },
  });
  try {
    assert.equal(completed, 0);
    assert.match(v.host.textContent, /等待目标规划/);
    w.request.status = "completed";
    w.request.revision++;
    await act(async () =>
      document.dispatchEvent(new window.Event("visibilitychange")),
    );
    await v.flush();
    assert.equal(completed, 1);
    assert.match(v.host.textContent, /本次处理已结束/);
  } finally {
    await v.dispose();
  }
});
test("retired directory status blocks new proposals even when a detail response still shows active", async () => {
  const w = workspace();
  w.state.characters[0].status = "已退休";
  const v = await mount(w);
  try {
    assert.equal(button(v.host, "规划日程").disabled, true);
    await v.click(v.host.querySelector(".lp-timeline-card"));
    assert.equal(
      button(document.querySelector("[role=dialog]"), "提出修改").disabled,
      true,
    );
    assert.equal(
      w.calls.some((call) => call[0] === "item" || call[0] === "plan"),
      false,
    );
  } finally {
    await v.dispose();
  }
});
test("closing a frozen detail after a date change restores the current heading when its source row was removed", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    const source = v.host.querySelector(".lp-timeline-card");
    source.focus();
    await v.click(source);
    await v.update({ route: { query: new URLSearchParams({ date: future }) } });
    assert.equal(source.isConnected, false);
    await v.click(button(document.querySelector("[role=dialog]"), "关闭"));
    assert.equal(
      document.activeElement,
      v.host.querySelector(".replica-page-heading h1"),
    );
  } finally {
    await v.dispose();
  }
});
test("an empty character scope cannot start a schedule read while the shell returns to the library", async () => {
  const w = workspace();
  w.state.selectedId = "";
  w.state.characters[0].id = "";
  const v = await mount(w);
  try {
    assert.deepEqual(w.calls, []);
    assert.equal(v.host.querySelectorAll(".lp-timeline-card").length, 0);
    assert.equal(button(v.host, "规划日程").disabled, true);
  } finally {
    await v.dispose();
  }
});
