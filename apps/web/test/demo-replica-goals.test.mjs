import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { goalAssessment } from "../src/approved-ui/goal-presentation.mjs";
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
const { default: Goals } = await vite.ssrLoadModule(
  "/src/react-app/pages/GoalsPage.jsx",
);
const button = (root, text) =>
  [...root.querySelectorAll("button")].find(
    (n) => n.textContent.trim() === text,
  );
const dialog = () => [...document.querySelectorAll('[role="dialog"]')].at(-1);
const clone = (value) => JSON.parse(JSON.stringify(value));
const goal = (id = "one", status = "active") => ({
  id,
  fluctlight_id: "A",
  revision: 1,
  status,
  desired_outcome: "完成" + id + "的画作",
  motivation: "记录生活",
  success_criteria: ["画好草图", "保存画作"],
  criterion_ids: ["sketch", "saved"],
  criteria_version: 2,
  scope: "general",
  deadline_policy: "soft",
  owner_protected: false,
  deadline: null,
  progress: 0.25,
  execution: {},
  evaluations: [],
});
function workspace() {
  sessionStorage.clear();
  const state = proxy({
      session: { actorId: "goal-owner" },
      selectedId: "A",
      characters: [{ id: "A", name: "林青", status: "正常" }],
    }),
    calls = [],
    rows = new Map(
      [goal(), goal("paused", "paused"), goal("done", "completed")].map((g) => [
        g.id,
        g,
      ]),
    );
  const client = {
    goals: async (fid, options) => {
      calls.push(["list", fid, clone(options)]);
      return {
        items: clone(
          [...rows.values()].filter(
            (g) =>
              ["completed", "cancelled", "abandoned"].includes(g.status) ===
              options.history,
          ),
        ),
        next_cursor: "",
      };
    },
    goalDetail: async (fid, id) => {
      calls.push(["detail", fid, id]);
      return clone(rows.get(id));
    },
    goalHistory: async () => ({ items: [], next_cursor: "" }),
    goalEvidence: async () => ({ items: [], next_cursor: "" }),
    createGoal: async (fid, body) => {
      calls.push(["create", fid, clone(body)]);
      const g = {
        ...goal("created"),
        desired_outcome: body.desiredOutcome,
        motivation: body.motivation,
        success_criteria: body.successCriteria,
        deadline_policy: body.deadlinePolicy,
        owner_protected: body.ownerProtected,
        deadline: body.clearDeadline ? null : body.deadline,
      };
      rows.set(g.id, g);
      return { goal_id: g.id, revision: g.revision, status: g.status };
    },
    goalCommand: async (fid, id, operation, body) => {
      calls.push([operation, fid, id, clone(body)]);
      const g = rows.get(id);
      if (operation === "reassess") {
        g.execution.evaluation_request = {
          request_id: "evaluation-one",
          status: "pending",
        };
        return {
          goal_id: id,
          revision: g.revision,
          status: "pending",
          evaluation_request_id: "evaluation-one",
        };
      }
      g.revision++;
      if (operation === "update")
        Object.assign(g, {
          desired_outcome: body.desiredOutcome,
          motivation: body.motivation,
          success_criteria: body.successCriteria,
        deadline_policy: body.deadlinePolicy,
        owner_protected: body.ownerProtected,
        deadline: body.clearDeadline ? null : body.deadline,
        });
      else
        g.status = {
          pause: "paused",
          resume: "active",
          cancel: "cancelled",
          abandon: "abandoned",
        }[operation];
      return { goal_id: id, revision: g.revision, status: g.status };
    },
  };
  return {
    state,
    client,
    calls,
    rows,
    current: {
      get value() {
        return state.characters[0];
      },
    },
    subscribe: (fn) => subscribe(state, fn),
    getSnapshot: () => snapshot(state),
    isCurrent: (owner, fid) =>
      owner === state.session.actorId && fid === state.selectedId,
  };
}
const mount = (w) =>
  mountReact(vite, Goals, {
    workspace: w,
    props: { route: { query: new URLSearchParams() } },
  });
async function open(v, id = "one") {
  await v.click(
    v.host.querySelector('[aria-label="查看目标 完成' + id + '的画作"]'),
  );
  return dialog();
}
async function input(v, label, text) {
  await v.input(dialog().querySelector('[aria-label="' + label + '"]'), text);
}
test("goal judgments use current goal/version/criterion identity and preserve Core's policy progress", () => {
  const g = goal();
  g.evaluations = [
    {
      goal_id: g.id,
      criteria_version: 2,
      stage_id: "stage",
      judgments: [{ criterion_id: "saved", verdict: "satisfied" }],
    },
    {
      goal_id: g.id,
      criteria_version: 1,
      judgments: [{ criterion_id: "saved", verdict: "satisfied" }],
    },
    {
      goal_id: g.id,
      criteria_version: 2,
      judgments: [
        { criterion_id: "saved", verdict: "satisfied" },
        { criterion_id: "sketch", verdict: "unknown" },
      ],
    },
  ];
  const a = goalAssessment(g);
  assert.deepEqual(
    a.criteria.map((v) => v.verdict),
    ["unknown", "satisfied"],
  );
  assert.equal(a.verifiedCount, 1);
  assert.equal(a.progress, 0.25);
  g.evidence = [];
  assert.equal(goalAssessment(g).verifiedCount, 1);
  g.status = "completed";
  g.evaluations = [];
  assert.equal(goalAssessment(g).verifiedCount, null);
  g.execution.last_evaluation = {
    evaluation_id: "own",
    criteria_version: 2,
    judgments: [{ criterion_id: "saved", verdict: "satisfied" }],
  };
  assert.equal(goalAssessment(g).verifiedCount, 1);
  g.criterion_ids = ["same", "same"];
  assert.equal(goalAssessment(g).verifiedCount, null);
  assert.ok(goalAssessment(g).criteria.every((v) => v.verdict === "unknown"));
});
test("current/history pages are independent, filtered and both append cursors remain reachable", async () => {
  const w = workspace(),
    base = w.client.goals;
  w.client.goals = async (fid, o) => {
    if (o.cursor) {
      w.calls.push(["cursor", clone(o)]);
      return {
        items: [
          goal(
            o.history ? "older" : "candidate",
            o.history ? "abandoned" : "candidate",
          ),
        ],
        next_cursor: "",
      };
    }
    const p = await base(fid, o);
    p.next_cursor = o.history ? "history-next" : "current-next";
    return p;
  };
  const v = await mount(w);
  try {
    assert.match(v.host.textContent, /完成one的画作/);
    assert.doesNotMatch(v.host.textContent, /完成done的画作/);
    await act(async () =>
      [...v.host.querySelectorAll('[role="tab"]')]
        .find((n) => n.textContent === "全部")
        .dispatchEvent(
          new window.MouseEvent("mousedown", {
            bubbles: true,
            button: 0,
            ctrlKey: false,
          }),
        ),
    );
    await v.flush();
    assert.match(v.host.textContent, /完成done的画作/);
    await v.click(button(v.host, "加载更多当前目标"));
    await v.click(button(v.host, "加载更多历史目标"));
    assert.deepEqual(
      w.calls.filter((c) => c[0] === "cursor").map((c) => c[1].cursor),
      ["current-next", "history-next"],
    );
    await v.input(
      v.host.querySelector('[aria-label="搜索目标"]'),
      "无匹配内容",
    );
    assert.match(v.host.textContent, /这一页暂时没有目标/);
  } finally {
    await v.dispose();
  }
});
test("initial loading is not an empty result; failed history retries its original partition", async () => {
  const w = workspace(),
    base = w.client.goals;
  let release,
    fail = true;
  w.client.goals = async (fid, o) => {
    if (!o.history)
      return new Promise((r) => (release = () => base(fid, o).then(r)));
    if (fail) throw Error("历史暂不可读");
    return base(fid, o);
  };
  const v = await mount(w);
  try {
    assert.match(v.host.textContent, /正在读取目标/);
    assert.doesNotMatch(v.host.textContent, /这一页暂时没有目标/);
    fail = false;
    await v.click(button(v.host, "重新读取历史目标"));
    assert.ok(w.calls.some((c) => c[0] === "list" && c[2].history));
    await act(async () => release());
    await v.flush();
    assert.match(v.host.textContent, /完成one的画作/);
  } finally {
    await v.dispose();
  }
});
test("cancelled edit discards unsaved input; save uses the actual definition and returns readback", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "编辑"));
    await input(v, "目标内容", "未保存的标题");
    await v.click(button(dialog(), "取消"));
    assert.equal(w.rows.get("one").desired_outcome, "完成one的画作");
    await v.click(button(dialog(), "编辑"));
    assert.equal(
      dialog().querySelector('[aria-label="目标内容"]').value,
      "完成one的画作",
    );
    await input(v, "目标内容", "新的画作期待");
    await input(v, "目标操作原因", "更新观察");
    await v.click(button(dialog(), "保存目标"));
    assert.match(dialog().textContent, /新的画作期待/);
    const sent = w.calls.find((c) => c[0] === "update");
    assert.equal(sent[3].expectedRevision, 1);
    assert.ok(sent[3].idempotencyKey);
    assert.equal(sent[3].desiredOutcome, "新的画作期待");
    assert.equal("importance" in sent[3], false);
  } finally {
    await v.dispose();
  }
});
test("pause/resume and reassessment follow actual records; pending reassessment does not complete criteria", async () => {
  const w = workspace(),
    v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "暂停"));
    await input(v, "目标操作原因", "稍后继续");
    await v.click(button(dialog(), "暂停目标"));
    assert.equal(w.rows.get("one").status, "paused");
    await v.click(button(dialog(), "继续"));
    await input(v, "目标操作原因", "继续画画");
    await v.click(button(dialog(), "继续目标"));
    assert.equal(w.rows.get("one").status, "active");
    await v.click(button(dialog(), "核对进展"));
    await input(v, "目标操作原因", "检查实际证据");
    await v.click(button(dialog(), "核对进展"));
    assert.equal(w.rows.get("one").status, "active");
    assert.match(dialog().textContent, /等待实际证据核对/);
    assert.equal(button(dialog(), "核对进展").disabled, true);
  } finally {
    await v.dispose();
  }
});
test("unknown create survives reopen with equal body/key, and newer fields become an update after reviewed readback", async () => {
  const w = workspace(),
    base = w.client.createGoal;
  let attempts = 0;
  w.client.createGoal = async (fid, body) => {
    if (++attempts === 1) {
      w.calls.push(["unknown", clone(body)]);
      throw Error("回执未确认");
    }
    return base(fid, body);
  };
  let v = await mount(w);
  try {
    await v.click(button(v.host, "添加目标"));
    await input(v, "目标内容", "原期待");
    await input(v, "目标动机", "原动机");
    await input(v, "目标完成条件", "实际条件");
    await input(v, "目标操作原因", "新增期待");
    await v.click(button(dialog(), "保存目标"));
    await input(v, "目标内容", "后续新期待");
    await v.dispose();
    v = await mount(w);
    await v.click(button(v.host, "添加目标"));
    await v.click(button(dialog(), "核对或重试原提交"));
    const original = w.calls.find((c) => c[0] === "unknown")[1],
      sent = w.calls.find((c) => c[0] === "create")[2];
    assert.deepEqual(sent, original);
    assert.equal(
      dialog().querySelector('[aria-label="目标内容"]').value,
      "后续新期待",
    );
    await v.dispose();
    v = await mount(w);
    await v.click(button(v.host, "添加目标"));
    await v.click(button(dialog(), "读取最新版本并保留输入"));
    assert.match(dialog().textContent, /核对最新对象与版本/);
    await v.click(button(dialog(), "已核对，采用最新基线"));
    await v.click(button(dialog(), "保存目标"));
    assert.equal(attempts, 2);
    assert.equal(w.calls.filter((c) => c[0] === "update").length, 1);
    assert.equal(w.rows.get("created").desired_outcome, "后续新期待");
  } finally {
    await v.dispose();
  }
});
test("acknowledged create with failed readback is recovered by reads only", async () => {
  const w = workspace(),
    read = w.client.goalDetail;
  let fail = true;
  w.client.goalDetail = async (...args) => {
    if (fail) throw Error("读回暂不可用");
    return read(...args);
  };
  const v = await mount(w);
  try {
    await v.click(button(v.host, "添加目标"));
    for (const [label, text] of [
      ["目标内容", "期待"],
      ["目标动机", "观察"],
      ["目标完成条件", "实际条件"],
      ["目标操作原因", "新增"],
    ])
      await input(v, label, text);
    await v.click(button(dialog(), "保存目标"));
    assert.match(dialog().textContent, /原操作已有回执/);
    fail = false;
    await v.click(button(dialog(), "只读恢复页面结果"));
    assert.equal(w.calls.filter((c) => c[0] === "create").length, 1);
    assert.match(dialog().textContent, /期待/);
  } finally {
    await v.dispose();
  }
});
test("saving a new goal replaces the previous detail evidence and history with its own records", async () => {
  const w = workspace();
  w.client.goalEvidence = async (_fid, id) => ({
    items: [{ id: "evidence-" + id, source: { summary: "证据属于" + id } }],
    next_cursor: "",
  });
  w.client.goalHistory = async (_fid, id) => ({
    items: [{ id: "history-" + id, reason: "历史属于" + id }],
    next_cursor: "",
  });
  const v = await mount(w);
  try {
    await open(v);
    await v.click(button(dialog(), "关闭"));
    await v.click(button(v.host, "添加目标"));
    for (const [label, text] of [
      ["目标内容", "新的期待"],
      ["目标动机", "观察"],
      ["目标完成条件", "实际条件"],
      ["目标操作原因", "新增"],
    ])
      await input(v, label, text);
    await v.click(button(dialog(), "保存目标"));
    await v.click(button(dialog(), "证据来源"));
    assert.match(dialog().textContent, /证据属于created/);
    assert.doesNotMatch(dialog().textContent, /证据属于one/);
    await v.click(button(dialog(), "目标修订历史"));
    assert.match(dialog().textContent, /历史属于created/);
    assert.doesNotMatch(dialog().textContent, /历史属于one/);
  } finally {
    await v.dispose();
  }
});

test('acknowledged goal saves verify criteria, deadlines and protection before success and recover by GET only',async()=>{
 for(const mismatch of [{success_criteria:['被遗漏的完成条件']},{deadline_policy:'hard'},{owner_protected:true},{deadline:'2032-01-01T00:00:00Z'}]){
  const w=workspace(),read=w.client.goalDetail,write=w.client.goalCommand;let corrupt=false;
  w.client.goalCommand=async(...args)=>{const r=await write(...args);corrupt=true;return r;};
  w.client.goalDetail=async(...args)=>{const r=await read(...args);return corrupt?{...r,...mismatch}:r;};
  const v=await mount(w);
  try{
   await open(v);await v.click(button(dialog(),'编辑'));await input(v,'目标操作原因','核对完整定义');await v.click(button(dialog(),'保存目标'));
   assert.match(dialog().textContent,/读回内容尚未确认一致/);assert.match(dialog().textContent,/原操作已有回执/);
   assert.doesNotMatch(v.host.textContent,/目标已保存并读回/);
   corrupt=false;await v.click(button(dialog(),'只读恢复页面结果'));
   assert.equal(w.calls.filter(c=>c[0]==='update').length,1);assert.match(v.host.textContent,/目标已保存并读回/);
  }finally{await v.dispose();}
 }
});

test('goal details expose recorded stages, commitments and dependencies without claiming parent completion',async()=>{
 const w=workspace();Object.assign(w.rows.get('one'),{
  dependencies:[{prerequisite_id:'before',desired_outcome:'先确认读书会书单',satisfied:false,status:'active'}],
  stages:[{id:'stage-one',purpose:'准备读书会',strategy:'邀请愿意参与的读者',entry_basis:'书单已确认',exit_basis:'收到三位明确回复',criteria:[{id:'stage-c',text:'三位读者确认'}],status:'active',revision:2,criteria_version:1,dependency_ids:[]}],
  commitments:[{id:'commit-one',stage_id:'stage-one',expected_result:'联系第一位读者',opportunity_condition:'对方方便交流时',blocker:'尚未确认空闲时间',criteria:[{id:'commit-c',text:'取得明确回复'}],window_start:'2026-10-28T12:00:00Z',window_end:'2026-10-28T13:00:00Z',status:'active',revision:1}]
 });
 const v=await mount(w);try{await open(v);const d=dialog();assert.match(d.querySelector('[aria-label="目标阶段"]').textContent,/准备读书会.*邀请愿意参与的读者.*书单已确认.*收到三位明确回复.*三位读者确认/s);assert.match(d.querySelector('[aria-label="目标行动承诺"]').textContent,/联系第一位读者.*对方方便交流时.*尚未确认空闲时间.*取得明确回复/s);assert.match(d.querySelector('[aria-label="目标前置条件"]').textContent,/先确认读书会书单.*尚未完成/s);assert.equal(w.rows.get('one').status,'active');assert.equal(w.calls.some(c=>['update','reassess'].includes(c[0])),false);}finally{await v.dispose();}
});
