// Isolated production-build QA only. No real Provider, Core or persistent data.
export function goalsFixture({ goals, owner, scenario, date }) {
  const clone = (value) => JSON.parse(JSON.stringify(value)),
    scopes = new Map(),
    receipts = new Map(),
    histories = new Map(),
    evaluations = new Map();
  let counter = 0,
    reads = 0,
    lost = false;
  function rows(fid) {
    if (!scopes.has(fid))
      scopes.set(
        fid,
        scenario === "empty"
          ? []
          : goals.map((g, index) => ({
              ...clone(g),
              fluctlight_id: fid,
              progress: index === 0 ? 0.5 : 0,
              importance: 0.7,
              urgency: 0.3,
              execution: { next_step: "继续观察实际进展" },
              ...(index === 0
                ? {
                    success_criteria: ["完成街景草图", "保存最终画作"],
                    criterion_ids: ["sketch", "painting"],
                    evaluations: [
                      {
                        id: "evaluation-fixture",
                        goal_id: g.id,
                        criteria_version: 1,
                        stage_id: null,
                        commitment_id: null,
                        judgments: [
                          {
                            criterion_id: "painting",
                            verdict: "unknown",
                            reason: "尚未保存最终画作",
                          },
                          {
                            criterion_id: "sketch",
                            verdict: "satisfied",
                            reason: "已有草图记录",
                            evidence_refs: ["evidence-sketch"],
                          },
                        ],
                      },
                    ],
                  }
                : {}),
            })),
      );
    return scopes.get(fid);
  }
  return async ({ path, method, body, url, json, res }) => {
    const match = path.match(
      /^\/api\/fluctlights\/([^/]+)\/goals(?:\/([^/]+))?(?:\/([^/]+))?$/,
    );
    if (!match) return false;
    const [, fid, id, operation] = match,
      items = rows(fid),
      g = items.find((v) => v.id === id);
    if (method === "GET" && !id) {
      reads++;
      if (scenario === "loading" && reads <= 2)
        await new Promise((r) => setTimeout(r, 5000));
      if (scenario === "failure" && reads === 1) {
        json(res, { code: "request_failed", message: "目标暂时无法读取" }, 503);
        return true;
      }
      json(res, {
        items: clone(
          items.filter(
            (g) =>
              ["completed", "cancelled", "abandoned"].includes(g.status) ===
              (url.searchParams.get("history") === "true"),
          ),
        ),
        next_cursor: "",
      });
      return true;
    }
    if (id && !g) {
      json(res, { code: "not_found", message: "目标不存在" }, 404);
      return true;
    }
    if (method === "GET") {
      if (operation === "history")
        json(res, {
          items: clone(
            histories.get(fid + id) || [
              {
                id: "history-" + id,
                operation: "create",
                reason: "隔离样本目标",
                created_at: date,
              },
            ],
          ),
          next_cursor: "",
        });
      else if (operation === "evidence")
        json(res, {
          items:
            id === items[0]?.id
              ? [
                  {
                    id: "evidence-sketch",
                    goal_id: id,
                    source: {
                      kind: "diary",
                      summary: "已保存街景草图的样本记录",
                      recorded_at: date,
                    },
                  },
                ]
              : [],
          next_cursor: "",
        });
      else {
        const request = evaluations.get(fid + id);
        if (request && ++request.reads >= 3)
          g.execution.evaluation_request.status = "succeeded";
        json(res, clone(g));
      }
      return true;
    }
    const receiptKey = JSON.stringify([owner, fid, body.idempotencyKey]);
    if (receipts.has(receiptKey)) {
      const saved = receipts.get(receiptKey);
      if (JSON.stringify(saved.body) !== JSON.stringify(body)) {
        json(
          res,
          { code: "idempotency_conflict", message: "原请求输入不匹配" },
          409,
        );
      } else json(res, saved.result);
      return true;
    }
    if (g && body.expectedRevision !== g.revision) {
      json(res, { code: "revision_conflict", message: "目标版本已变化" }, 409);
      return true;
    }
    let target = g;
    if (!id) {
      target = {
        id: "goal-fixture-created-" + ++counter,
        fluctlight_id: fid,
        revision: 0,
        status: body.candidateOnly ? "candidate" : "active",
        scope: body.scope || "general",
        criteria_version: 0,
        execution: {},
        evaluations: [],
      };
      items.push(target);
    }
    if (!id || method === "PUT" || operation === "update") {
      const changed =
        JSON.stringify(target.success_criteria) !==
        JSON.stringify(body.successCriteria);
      target.revision++;
      Object.assign(target, {
        desired_outcome: body.desiredOutcome,
        motivation: body.motivation,
        success_criteria: body.successCriteria,
        deadline_policy: body.deadlinePolicy,
        owner_protected: body.ownerProtected,
      });
      if (body.clearDeadline) delete target.deadline;
      else if (body.deadline) target.deadline = body.deadline;
      if (changed) {
        target.criteria_version++;
        target.criterion_ids = body.successCriteria.map(
          (_, i) => target.id + "-" + target.criteria_version + "-" + i,
        );
        target.progress = 0;
      }
    } else if (operation !== "reassess") {
      target.revision++;
      target.status =
        {
          pause: "paused",
          resume: "active",
          abandon: "abandoned",
          cancel: "cancelled",
        }[operation] || target.status;
    }
    const result = {
      goal_id: target.id,
      revision: target.revision,
      status: target.status,
    };
    if (operation === "reassess") {
      result.status = "pending";
      result.evaluation_request_id = "evaluation-request-" + ++counter;
      target.execution.evaluation_request = {
        request_id: result.evaluation_request_id,
        status: "pending",
        attempt_count: 1,
      };
      evaluations.set(fid + id, { reads: 0 });
    }
    const historyKey = fid + target.id;
    histories.set(historyKey, [
      {
        id: "history-" + ++counter,
        operation: operation || (!id ? "create" : "update"),
        reason: body.reason,
        created_at: date,
      },
      ...(histories.get(historyKey) || []),
    ]);
    receipts.set(receiptKey, { body: clone(body), result: clone(result) });
    if (scenario === "unknown" && !lost) {
      lost = true;
      json(
        res,
        { code: "request_failed", message: "目标回执未确认，保留原提交" },
        503,
      );
    } else json(res, result);
    return true;
  };
}
