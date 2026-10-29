import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { memoryRevisionReason } from "../src/approved-ui/memory-presentation.mjs";
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
const { default: Memory } = await vite.ssrLoadModule(
  "/src/react-app/pages/MemoryPage.jsx",
);
const clone = (v) => JSON.parse(JSON.stringify(v)),
  button = (root, text) =>
    [...root.querySelectorAll("button")].find(
      (n) => n.textContent.trim() === text,
    ),
  dialog = () => [...document.querySelectorAll("[role=dialog]")].at(-1);
const memory = {
  id: "M",
  owner_fluctlight_id: "A",
  type: "relationship",
  content: "你喜欢听雨声。",
  actor_refs: ["memory-owner"],
  conversation_id: "conversation",
  event_refs: [],
  evidence_refs: ["message:source"],
  personality_perspectives: [],
  confidence: 0.8,
  importance: 0.8,
  emotional_significance: 0.5,
  visibility: "owner",
  status: "active",
  revision: 2,
  provenance_status: "valid",
  occurred_at: "2026-10-26T12:00:00Z",
  created_at: "2026-10-27T01:00:00Z",
  last_confirmed_at: null,
  superseded_by_memory_id: null,
  supersedes_memory_id: null,
  revision_count: 2,
  source_count: 1,
};
test("the current revision reason wins over older merged support references", () => {
  const row = {
    operation: "forget",
    actor_id: "owner",
    evidence_refs: ["owner:owner:旧说明", "owner:owner:新说明"],
    semantic_reason: "owner:owner:新说明",
  };
  assert.equal(memoryRevisionReason(row), "新说明");
  assert.equal(
    memoryRevisionReason({ ...row, semantic_reason: "本版明确说明" }),
    "本版明确说明",
  );
  assert.equal(
    memoryRevisionReason({ ...row, semantic_reason: "" }),
    "遗忘记录",
  );
});
const claim = {
  id: "C",
  fluctlight_id: "A",
  category: "self_perception",
  claim: "我愿意把对话慢慢听完。",
  value: {},
  confidence: 0.7,
  evidence_refs: ["message:source"],
  provenance: {},
  status: "active",
  expires_at: null,
  revision: 2,
  superseded_by: null,
  created_at: "2026-10-26T12:00:00Z",
  updated_at: "2026-10-26T12:00:00Z",
  source_valid: true,
  expired: false,
  owner_review: null,
};
function workspace() {
  sessionStorage.clear();
  const state = proxy({
      session: { actorId: "memory-owner" },
      selectedId: "A",
      characters: [{ id: "A", name: "林青", status: "正常" }],
    }),
    calls = [],
    m = clone(memory),
    s = clone(claim),
    feedback = [],
    reflection = {
      id: "R",
      fluctlight_id: "A",
      title: "听完，再说",
      summary: "注意到倾听让对话更自然。",
      status: "applied",
      state: "applied",
      at: "2026-10-26T12:00:00Z",
      source_window_status: "available",
      proposal: { developing_self_candidates: [{ claim: "我愿意倾听" }] },
      source_window: {},
      dispositions: [{ domain: "developing_self", disposition: "accepted" }],
      result: {
        status: "no_change",
        counts: { developing_self: { applied: 0, no_change: 1 } },
      },
    };
  const client = {
    managedMemories: async (fid, options) => {
      calls.push(["list", fid, clone(options)]);
      return {
        items:
          (!options.status || options.status === m.status) &&
          (!options.type || options.type === m.type) &&
          (!options.q || m.content.includes(options.q))
            ? [clone(m)]
            : [],
        next_cursor: "",
      };
    },
    managedMemory: async (id) => {
      calls.push(["read", id]);
      return clone(m);
    },
    memorySources: async (id, options) => {
      calls.push(["sources", id, clone(options)]);
      return {
        items: [
          {
            memory_revision: options.revision,
            source_ref: "message:source",
            source_kind: "message",
            source_id: "source",
            source_revision: null,
            occurred_at: "2026-10-26T12:00:00Z",
            recorded_at: "2026-10-27T01:00:00Z",
            status: "valid",
            live: false,
            superseded_by_ref: null,
            conversation_id: "conversation",
          },
        ],
        next_cursor: "",
      };
    },
    memoryRevisions: async () => ({
      items: [
        {
          revision: m.revision,
          content: m.content,
          status: m.status,
          operation: "revise",
          created_at: m.created_at,
        },
        {
          revision: 1,
          content: "过去的记忆正文。",
          status: "active",
          operation: "create",
          created_at: m.created_at,
        },
      ],
      next_cursor: "",
    }),
    reviseMemory: async (id, body) => {
      calls.push(["revise", id, clone(body)]);
      m.content = body.content;
      m.revision++;
      return {
        id,
        operation: "revise",
        revision: m.revision,
        status: m.status,
      };
    },
    forgetMemory: async (id, body) => {
      calls.push(["forget", id, clone(body)]);
      m.status = "forgotten";
      m.revision++;
      return {
        id,
        operation: "forget",
        revision: m.revision,
        status: m.status,
      };
    },
    rollbackMemory: async (id, body) => {
      calls.push(["rollback", id, clone(body)]);
      m.content = "过去的记忆正文。";
      m.status = "active";
      m.revision++;
      return {
        id,
        operation: "rollback",
        revision: m.revision,
        status: m.status,
        target_revision: body.targetRevision,
      };
    },
    listReflections: async () => ({
      items: [clone(reflection)],
      next_cursor: "",
      window: {},
      latest_task: null,
    }),
    reflection: async () => clone(reflection),
    reflectionFeedback: async () => ({
      items: clone(feedback),
      next_cursor: "",
    }),
    submitReflectionFeedback: async (fid, id, body) => {
      calls.push(["feedback", fid, id, clone(body)]);
      const request = {
        id: "reflection-request",
        fluctlight_id: fid,
        kind: "reflection_feedback",
        status: "pending",
        revision: 1,
        payload: { reflection_id: id, text: body.text },
        result: {},
        created_at: "2026-10-27T01:00:00Z",
      };
      feedback.push(request);
      return { request };
    },
    selfClaims: async (fid, options) => ({
      items:
        options.status === "all" || options.status === s.status
          ? [clone(s)]
          : [],
      next_cursor: "",
    }),
    selfClaim: async () => clone(s),
    selfClaimHistory: async (fid, id, kind) => ({
      items:
        kind === "reviews"
          ? s.owner_review
            ? [clone(s.owner_review)]
            : []
          : [
              {
                revision: 1,
                status: "accepted",
                after_value: {
                  ...clone(claim),
                  claim: "以前的认识。",
                  status: "active",
                },
              },
            ],
      next_cursor: "",
    }),
    manageSelfClaim: async (fid, id, operation, body) => {
      calls.push(["self", fid, id, operation, clone(body)]);
      if (operation === "review")
        s.owner_review = {
          id: "review",
          fluctlight_id: fid,
          claim_id: id,
          owner_actor_id: state.session.actorId,
          claim_revision: s.revision,
          reason: body.reason,
          created_at: "2026-10-27T01:00:00Z",
        };
      else {
        s.revision++;
        s.status = operation === "forget" ? "forgotten" : "active";
        if (operation === "restore") s.claim = "以前的认识。";
      }
      return {
        id,
        fluctlight_id: fid,
        revision: s.revision,
        status: s.status,
        operation,
      };
    },
  };
  return {
    state,
    calls,
    m,
    s,
    reflection,
    client,
    current: {
      get value() {
        return state.characters[0];
      },
    },
    subscribe: (fn) => subscribe(state, fn),
    getSnapshot: () => snapshot(state),
    isCurrent: (owner, fid) =>
      state.session.actorId === owner && state.selectedId === fid,
    currentScope: () =>
      JSON.stringify([state.session.actorId, state.selectedId]),
    navigatePath: (path) => calls.push(["navigate", path]),
    services: { lifeClient: { request: async () => clone(feedback[0]) } },
  };
}
const mount = (w) =>
  mountReact(vite, Memory, {
    workspace: w,
    props: { route: { query: new URLSearchParams() } },
  });
async function tab(v, root, text) {
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
async function open(v) {
  await v.click(v.host.querySelector('[aria-label="查看记忆 你喜欢听雨声。"]'));
}
test("memory card scores use the actual 0–1 scale and search sends the applied scope rather than fabricating tags", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    assert.ok(v.host.querySelector('[aria-label="重要度 80%"]'));
    assert.match(v.host.textContent, /含你的引用/);
    await v.input(
      v.host.querySelector('[aria-label="搜索记忆正文"]'),
      "不存在的片段",
    );
    await v.click(v.host.querySelector('[aria-label="应用记忆搜索"]'));
    assert.match(v.host.textContent, /这里没有找到匹配的记忆/);
    assert.equal(
      w.calls.filter((c) => c[0] === "list").at(-1)[2].q,
      "不存在的片段",
    );
    await v.click(button(v.host, "清除筛选"));
    assert.match(v.host.textContent, /你喜欢听雨声/);
  } finally {
    await v.dispose();
  }
});
test("memory edit cancellation preserves original; correction sends CAS body without client idempotency fields and reads real revision back", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "记错了，纠正一下"));
    await v.input(
      dialog().querySelector('[aria-label="记忆修正正文"]'),
      "取消的内容",
    );
    await v.click(button(dialog(), "取消"));
    assert.equal(w.m.content, memory.content);
    await v.click(button(dialog(), "记错了，纠正一下"));
    assert.equal(
      dialog().querySelector('[aria-label="记忆修正正文"]').value,
      memory.content,
    );
    await v.input(
      dialog().querySelector('[aria-label="记忆修正正文"]'),
      "你喜欢在窗边听雨。",
    );
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "补充原来遗漏的情境",
    );
    await v.click(button(dialog(), "保存纠正"));
    const sent = w.calls.find((c) => c[0] === "revise")[2];
    assert.equal(sent.expectedRevision, 2);
    assert.equal(sent.content, "你喜欢在窗边听雨。");
    assert.deepEqual(sent.evidenceRefs, [
      "owner:memory-owner:补充原来遗漏的情境",
    ]);
    assert.equal("idempotencyKey" in sent, false);
    assert.equal("reason" in sent, false);
    assert.equal(w.m.revision, 3);
    assert.match(dialog().textContent, /你喜欢在窗边听雨/);
    assert.ok(w.calls.some((c) => c[0] === "sources" && c[2].revision === 3));
  } finally {
    await v.dispose();
  }
});
test("an unknown CAS-only correction requires actual current-object review, and equal confirmed prose cannot be submitted again", async () => {
  const w = workspace(),
    write = w.client.reviseMemory;
  w.client.reviseMemory = async (...args) => {
    await write(...args);
    throw Error("回执未知");
  };
  const v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "记错了，纠正一下"));
    await v.input(
      dialog().querySelector('[aria-label="记忆修正正文"]'),
      "实际已经提交的正文",
    );
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "核对事实",
    );
    await v.click(button(dialog(), "保存纠正"));
    assert.equal(w.calls.filter((c) => c[0] === "revise").length, 1);
    assert.equal(button(dialog(), "保存纠正").disabled, true);
    assert.equal(button(dialog(), "核对或重试原提交"), undefined);
    await v.click(button(dialog(), "同步资料，保留输入"));
    await v.click(button(dialog(), "已核对，采用最新基线"));
    await v.click(button(dialog(), "保存纠正"));
    assert.match(dialog().textContent, /当前版本已经是这段正文/);
    assert.equal(w.calls.filter((c) => c[0] === "revise").length, 1);
    await v.click(button(dialog(), "取消"));
    assert.match(dialog().textContent, /第 3 版/);
    assert.match(dialog().textContent, /实际已经提交的正文/);
    assert.match(v.host.textContent, /实际已经提交的正文/);
    assert.equal(w.calls.filter((c) => c[0] === "revise").length, 1);
  } finally {
    await v.dispose();
  }
});
test("an acknowledged memory mutation with failed readback recovers using reads only", async () => {
  const w = workspace(),
    read = w.client.managedMemory;
  let fail = false;
  w.client.managedMemory = async (...args) => {
    if (fail && w.calls.some((c) => c[0] === "revise")) throw Error("读回断网");
    return read(...args);
  };
  const v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "记错了，纠正一下"));
    await v.input(
      dialog().querySelector('[aria-label="记忆修正正文"]'),
      "只读恢复后的正文",
    );
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "纠正记录",
    );
    fail = true;
    await v.click(button(dialog(), "保存纠正"));
    assert.match(dialog().textContent, /原操作已有回执/);
    fail = false;
    await v.click(button(dialog(), "只读恢复页面结果"));
    assert.equal(w.calls.filter((c) => c[0] === "revise").length, 1);
    assert.match(dialog().textContent, /只读恢复后的正文/);
  } finally {
    await v.dispose();
  }
});
test("a validated readback stays visible when secondary refreshes cannot reread the changed record", async () => {
  const w = workspace(),
    read = w.client.managedMemory;
  let postReads = 0;
  w.client.managedMemory = async (...args) => {
    if (w.calls.some((c) => c[0] === "revise") && ++postReads > 1)
      throw Error("后续来源刷新失败");
    return read(...args);
  };
  const v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "记错了，纠正一下"));
    await v.input(
      dialog().querySelector('[aria-label="记忆修正正文"]'),
      "已验证的新版本正文",
    );
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "实际修正",
    );
    await v.click(button(dialog(), "保存纠正"));
    assert.equal(w.m.revision, 3);
    await tab(v, dialog(), "这段记忆");
    assert.match(
      dialog().querySelector(".lp-memory-quote").textContent,
      /已验证的新版本正文/,
    );
    assert.match(dialog().textContent, /第 3 版/);
    assert.equal(w.calls.filter((c) => c[0] === "revise").length, 1);
  } finally {
    await v.dispose();
  }
});
test("source pagination pins the chosen revision and never marks a returned nonlive source as valid", async () => {
  const w = workspace(),
    source = w.client.memorySources;
  w.client.memorySources = async (id, o) => {
    const page = await source(id, o);
    page.next_cursor = o.cursor ? "" : "source-next";
    return page;
  };
  const v = await mount(w);
  try {
    await open(v);
    await tab(v, dialog(), "原始来源");
    assert.match(dialog().textContent, /来源失效或未知/);
    await v.click(button(dialog(), "加载更多来源"));
    assert.equal(
      w.calls.filter((c) => c[0] === "sources").at(-1)[2].revision,
      2,
    );
    await tab(v, dialog(), "修订记录");
    const row = [...dialog().querySelectorAll(".lp-history-item")].find((n) =>
      n.textContent.startsWith("第 1 版"),
    );
    await v.click(button(row, "查看此版来源"));
    assert.equal(
      w.calls.filter((c) => c[0] === "sources").at(-1)[2].revision,
      1,
    );
    assert.match(dialog().textContent, /第 1 版的来源/);
  } finally {
    await v.dispose();
  }
});
test("forget cancellation does not write and historical memory restoration appends a new revision", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "遗忘这段记忆"));
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "暂时不再采用",
    );
    await v.click(button(dialog(), "取消"));
    assert.equal(
      w.calls.some((c) => c[0] === "forget"),
      false,
    );
    await tab(v, dialog(), "修订记录");
    const row = [...dialog().querySelectorAll(".lp-history-item")].find((n) =>
      n.textContent.startsWith("第 1 版"),
    );
    await v.click(button(row, "补偿恢复至此版"));
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "恢复旧版并保留历史",
    );
    await v.click(button(dialog(), "确认补偿恢复"));
    const sent = w.calls.find((c) => c[0] === "rollback")[2];
    assert.equal(sent.expectedRevision, 2);
    assert.equal(sent.targetRevision, 1);
    assert.equal("idempotencyKey" in sent, false);
    assert.equal(w.m.revision, 3);
    assert.equal(w.m.content, "过去的记忆正文。");
    await v.click(button(dialog(), "遗忘这段记忆"));
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "明确不再采用",
    );
    await v.click(button(dialog(), "确认遗忘"));
    assert.equal(w.m.status, "forgotten");
    assert.equal(w.m.revision, 4);
    assert.equal(w.m.content, "过去的记忆正文。");
  } finally {
    await v.dispose();
  }
});
test("reflection feedback preserves actual proposal/application state and follows a pending OwnerRequest", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await tab(v, v.host, "反思记录");
    await v.click(button(v.host, "展开这次反思"));
    await v.click(button(dialog(), "赞同这次观察"));
    await v.input(
      dialog().querySelector('[aria-label="反思反馈内容"]'),
      "我也觉得倾听很重要。",
    );
    await v.click(button(dialog(), "提交反馈"));
    const sent = w.calls.find((c) => c[0] === "feedback")[3];
    assert.equal(sent.stance, "agree");
    assert.ok(sent.idempotencyKey);
    assert.equal(w.reflection.status, "applied");
    assert.equal(w.s.claim, claim.claim);
    assert.match(dialog().textContent, /等待回应/);
    assert.doesNotMatch(dialog().textContent, /已采纳，这条认识已加入/);
  } finally {
    await v.dispose();
  }
});
test("self review appends acknowledgement without changing claim revision or confidence; historical restore is a real command", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await tab(v, v.host, "自我认知");
    await v.click(button(v.host, "查看依据"));
    assert.equal(dialog().querySelector("textarea"), null);
    await v.click(button(dialog(), "确认已核对"));
    await v.input(
      dialog().querySelector('[aria-label="认识操作说明"]'),
      "已阅读这一版本",
    );
    await v.click(button(dialog(), "确认已核对"));
    assert.equal(w.s.revision, 2);
    assert.equal(w.s.confidence, 0.7);
    assert.equal(w.s.claim, claim.claim);
    const review = w.calls.find((c) => c[0] === "self");
    assert.equal(review[3], "review");
    assert.ok(review[4].idempotencyKey);
    await v.click(button(dialog(), "之前的认识与历史恢复"));
    await v.click(button(dialog(), "恢复此版内容"));
    await v.input(
      dialog().querySelector('[aria-label="认识操作说明"]'),
      "保留历史依据，恢复原内容",
    );
    await v.click(button(dialog(), "确认历史恢复"));
    assert.equal(w.s.claim, "以前的认识。");
    assert.equal(w.s.revision, 3);
    assert.equal(
      w.calls.filter((c) => c[0] === "self").at(-1)[4].targetRevision,
      1,
    );
  } finally {
    await v.dispose();
  }
});
test("failed append retries its original server query despite unsubmitted edits; a stale scope cannot admit a correction", async () => {
  const w = workspace(),
    base = w.client.managedMemories;
  let fail = true;
  w.client.managedMemories = async (fid, o) => {
    if (o.cursor) {
      w.calls.push(["append", clone(o)]);
      if (fail) throw Error("翻页读取失败");
      return { items: [], next_cursor: "" };
    }
    const page = await base(fid, o);
    page.next_cursor = "memory-next";
    return page;
  };
  const v = await mount(w);
  try {
    await v.click(button(v.host, "加载更多记忆"));
    assert.match(v.host.textContent, /翻页读取失败/);
    await v.input(
      v.host.querySelector('[aria-label="搜索记忆正文"]'),
      "未提交的另一个搜索",
    );
    fail = false;
    await v.click(button(v.host, "重新读取记忆"));
    const attempts = w.calls.filter((c) => c[0] === "append").map((c) => c[1]);
    assert.deepEqual(attempts[1], attempts[0]);
    assert.equal(attempts[0].q, "");
    await open(v);
    await v.click(button(dialog(), "记错了，纠正一下"));
    await v.input(
      dialog().querySelector('[aria-label="记忆修正正文"]'),
      "旧角色的新草稿",
    );
    await v.input(
      dialog().querySelector('[aria-label="记忆操作说明"]'),
      "保留说明",
    );
    let release;
    w.client.managedMemory = () =>
      new Promise((resolve) => (release = () => resolve(clone(w.m))));
    await v.click(button(dialog(), "保存纠正"));
    await act(async () => {
      w.state.selectedId = "B";
      w.state.characters[0].id = "B";
    });
    await act(async () => release());
    await v.flush();
    assert.equal(
      w.calls.some((c) => c[0] === "revise"),
      false,
    );
  } finally {
    await v.dispose();
  }
});
test("growth route query updates close the old detail and select the requested tab/filter in the same mounted page", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await open(v);
    await v.update({
      route: { query: new URLSearchParams({ tab: "self", status: "all" }) },
    });
    assert.equal(dialog(), undefined);
    assert.equal(
      [...v.host.querySelectorAll("[role=tab]")]
        .find((n) => n.textContent === "自我认知")
        .getAttribute("data-state"),
      "active",
    );
    assert.match(v.host.textContent, /我愿意把对话慢慢听完/);
    await v.update({
      route: {
        query: new URLSearchParams({
          aboutOwner: "true",
          type: "relationship",
        }),
      },
    });
    assert.equal(
      w.calls.filter((c) => c[0] === "list").at(-1)[2].aboutOwner,
      true,
    );
    assert.equal(
      w.calls.filter((c) => c[0] === "list").at(-1)[2].type,
      "relationship",
    );
  } finally {
    await v.dispose();
  }
});

for (const recovery of ["adopt", "refresh"]) test(`self ${recovery} reconciles detail, revision history and independent reviews`, async () => {
  const w = workspace();
  let history = [{ revision: 1, status: "accepted", after_value: { ...clone(claim), revision: 1 } }], reviews = [];
  w.client.selfClaimHistory = async (fid, id, kind) => ({ items: clone(kind === "reviews" ? reviews : history), next_cursor: "" });
  const v = await mount(w);
  try {
    await tab(v, v.host, "自我认知");
    await v.click(button(v.host, "查看依据"));
    if (recovery === "adopt") {
      await v.click(button(dialog(), "不再采用"));
      await v.input(dialog().querySelector('[aria-label="认识操作说明"]'), "保留来源，停止采用");
    }
    w.s.revision = 3;
    w.s.status = "forgotten";
    history = [{ revision: 3, status: "forgotten", after_value: clone(w.s) }, ...history];
    reviews = [{ id: "external-review", claim_revision: 2, reason: "独立复核记录", created_at: claim.created_at }];
    if (recovery === "adopt") {
      await v.click(button(dialog(), "同步资料，保留输入"));
      await v.click(button(dialog(), "已核对，采用最新基线"));
      await v.click(button(dialog(), "取消"));
    } else await v.click(button(dialog(), "刷新当前详情"));
    assert.match(dialog().textContent, /已遗忘/);
    await v.click(button(dialog(), "之前的认识与历史恢复"));
    assert.ok([...dialog().querySelectorAll("h4")].some(n => n.textContent.startsWith("第 3 版")));
    await v.click(button(dialog(), "所有者复核记录"));
    assert.match(dialog().textContent, /独立复核记录/);
    assert.equal(w.calls.filter(c => c[0] === "self").length, 0);
  } finally { await v.dispose(); }
});

test("self accepted forget with inconsistent same-revision status remains GET-only recoverable", async () => {
  const w = workspace(), execute = w.client.manageSelfClaim;
  w.client.manageSelfClaim = async (...args) => { const result = await execute(...args); w.s.status = "active"; return result; };
  const v = await mount(w);
  try {
    await tab(v, v.host, "自我认知"); await v.click(button(v.host, "查看依据"));
    await v.click(button(dialog(), "不再采用"));
    await v.input(dialog().querySelector('[aria-label="认识操作说明"]'), "停止采用并核对状态");
    await v.click(button(dialog(), "确认不再采用"));
    assert.ok(button(dialog(), "只读恢复页面结果"));
    w.s.status = "forgotten";
    await v.click(button(dialog(), "只读恢复页面结果"));
    assert.match(dialog().textContent, /已遗忘/);
    assert.equal(w.calls.filter(c => c[0] === "self").length, 1);
  } finally { await v.dispose(); }
});

for (const fault of ["unknown", "readback"]) test(`self ${fault} recovery preserves the original command identity and confirmed revision`, async () => {
  const w = workspace(), execute = w.client.manageSelfClaim, read = w.client.selfClaim;
  let receipt, attempts = [], failRead = false;
  w.client.manageSelfClaim = async (...args) => {
    attempts.push(clone(args));
    if (receipt) return clone(receipt);
    receipt = await execute(...args);
    if (fault === "unknown") throw Error("回执丢失");
    failRead = true;
    return clone(receipt);
  };
  w.client.selfClaim = async () => { if (failRead) { failRead = false; throw Error("回读暂时失败"); } return read(); };
  const v = await mount(w);
  try {
    await tab(v, v.host, "自我认知"); await v.click(button(v.host, "查看依据"));
    await v.click(button(dialog(), "不再采用"));
    await v.input(dialog().querySelector('[aria-label="认识操作说明"]'), "核对原操作");
    await v.click(button(dialog(), "确认不再采用"));
    if (fault === "unknown") {
      assert.equal(button(dialog(), "同步资料，保留输入").disabled, true);
      await v.click(button(dialog(), "核对或重试原提交"));
      assert.deepEqual(attempts[1], attempts[0]);
    } else await v.click(button(dialog(), "只读恢复页面结果"));
    assert.match(dialog().textContent, /已遗忘/);
    assert.match(dialog().textContent, /第 3 版/);
    assert.equal(w.s.revision, 3);
    assert.equal(w.calls.filter(c => c[0] === "self").length, 1);
    assert.equal(attempts.length, fault === "unknown" ? 2 : 1);
  } finally { await v.dispose(); }
});

test("self confirmed detail survives secondary history failure without repeating the command", async () => {
  const w = workspace(), history = w.client.selfClaimHistory;
  let fail = false;
  w.client.selfClaimHistory = async (...args) => {
    if (fail) throw Error("次级记录暂时不可读");
    return history(...args);
  };
  const v = await mount(w);
  try {
    await tab(v, v.host, "自我认知"); await v.click(button(v.host, "查看依据"));
    await v.click(button(dialog(), "不再采用"));
    await v.input(dialog().querySelector('[aria-label="认识操作说明"]'), "保留已确认的遗忘结果");
    fail = true;
    await v.click(button(dialog(), "确认不再采用"));
    assert.match(dialog().textContent, /已遗忘/);
    assert.equal(button(dialog(), "只读恢复页面结果"), undefined);
    await v.click(button(dialog(), "之前的认识与历史恢复"));
    assert.ok(button(dialog(), "重新读取认识修订"));
    fail = false;
    await v.click(button(dialog(), "重新读取认识修订"));
    assert.match(dialog().textContent, /已遗忘/);
    assert.equal(w.calls.filter(c => c[0] === "self").length, 1);
  } finally { await v.dispose(); }
});

for (const operation of ["review", "forget"]) test(`self ${operation} accepts independent review status or newer authoritative revision`, async () => {
  const w = workspace(), execute = w.client.manageSelfClaim;
  w.client.manageSelfClaim = async (...args) => {
    const result = await execute(...args);
    if (operation === "review") result.status = "acknowledged";
    else { w.s.revision++; w.s.status = "active"; }
    return result;
  };
  const v = await mount(w);
  try {
    await tab(v, v.host, "自我认知"); await v.click(button(v.host, "查看依据"));
    await v.click(button(dialog(), operation === "review" ? "确认已核对" : "不再采用"));
    await v.input(dialog().querySelector('[aria-label="认识操作说明"]'), "核对权威读回");
    await v.click(button(dialog(), operation === "review" ? "确认已核对" : "确认不再采用"));
    assert.match(dialog().textContent, /这条认识从哪里来/);
    assert.equal(button(dialog(), "只读恢复页面结果"), undefined);
    assert.equal(w.calls.filter(c => c[0] === "self").length, 1);
  } finally { await v.dispose(); }
});
