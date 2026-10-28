// Explicit loopback QA: fictional diary records, never imported into production.
export function diaryFixture({ owner, scenario }) {
  const scopes = new Map(),
    receipts = new Map(),
    requests = new Map(),
    clone = (v) => JSON.parse(JSON.stringify(v));
  let reads = 0,
    lost = false,
    counter = 0;
  const make = (fid, id, date) => ({
    id,
    fluctlight_id: fid,
    date,
    timezone: "Asia/Shanghai",
    status: "ready",
    revision: 1,
    latest_attempt: 1,
    last_error_code: null,
    body: "这是一篇隔离验收日记。\n\n今天读到一段平常的故事，也把手边的画作慢慢整理好。\n\n日子里有许多小事，认真记下来，之后还可以回来看看。",
    source: "agent",
    refs: [],
    created_at: date + "T14:00:00Z",
    updated_at: date + "T14:00:00Z",
    active_attempt_id: "attempt-" + id,
    source_snapshot: { fixture: true },
    persona_provenance: {},
    model_provenance: {},
    annotation: { note: "", favorite: false, revision: 0 },
    attempts: [
      {
        id: "attempt-" + id,
        attempt: 1,
        status: "ready",
        error_code: null,
        created_at: date + "T14:00:00Z",
        finished_at: date + "T14:01:00Z",
        plan: {},
        task_id: null,
        workflow_id: null,
        workflow_status: null,
      },
    ],
    has_more_attempts: false,
  });
  const rows = (fid) => {
    if (!scopes.has(fid))
      scopes.set(
        fid,
        scenario === "empty"
          ? []
          : [
              make(fid, "diary-fixture-one", "2026-10-26"),
              {
                ...make(fid, "diary-fixture-two", "2026-10-25"),
                annotation: {
                  note: "这一天的片段，很想慢慢记住。",
                  favorite: true,
                  revision: 2,
                },
              },
            ],
      );
    return scopes.get(fid);
  };
  return async ({ path, method, body, url, json, res }) => {
    const requestMatch = path.match(
      /^\/api\/owner-requests\/(diary-reply-[^/]+)$/,
    );
    if (requestMatch && requests.has(requestMatch[1])) {
      const request = requests.get(requestMatch[1]);
      request.reads++;
      if (request.reads >= 3) request.value.status = "completed";
      json(res, clone(request.value));
      return true;
    }
    const match = path.match(
      /^\/api\/fluctlights\/([^/]+)\/(diary(?:\/([^/]+))?(?:\/([^/]+))?|shared-experiences)$/,
    );
    if (!match) return false;
    const [, fid, end, id, part] = match,
      items = rows(fid),
      entry = items.find((v) => v.id === id),
      reply = (value, status = 200) => {
        json(res, value, status);
        return true;
      };
    if (end === "shared-experiences")
      return reply({
        items:
          scenario === "empty"
            ? []
            : [
                {
                  id: "shared-fixture-message",
                  kind: "remote_communication",
                  title: "一起分享一段正在读的故事",
                  author_actor_id: owner,
                  message_kind: "user",
                  occurred_at: "2026-10-26T12:00:00Z",
                  source_ref: "message:shared-fixture-message",
                  conversation_id: "conversation-fixture",
                  copresent: false,
                  href:
                    "#/c/" +
                    fid +
                    "/chat?conversation=conversation-fixture&message=shared-fixture-message",
                },
              ],
        next_cursor: "",
        physical_presence_source:
          "requires_explicit_confirmed_joint_participation",
      });
    if (method === "GET" && !id) {
      reads++;
      if (scenario === "loading" && reads === 1)
        await new Promise((r) => setTimeout(r, 5000));
      if (scenario === "failure" && reads === 1)
        return reply(
          { code: "request_failed", message: "日记暂时无法读取" },
          503,
        );
      return reply({
        items: clone(
          items.filter(
            (v) =>
              url.searchParams.get("favorite") !== "true" ||
              v.annotation.favorite,
          ),
        ),
        next_cursor: "",
        timezone: "Asia/Shanghai",
      });
    }
    if (method === "GET") {
      if (!entry)
        return reply({ code: "not_found", message: "日记不存在" }, 404);
      if (part === "replies")
        return reply({
          items: [...requests.values()]
            .map((v) => v.value)
            .filter((v) => v.payload.diary_id === id),
          next_cursor: "",
        });
      if (["pending", "running"].includes(entry.status)) {
        entry._fixtureReads = (entry._fixtureReads || 0) + 1;
        if (entry._fixtureReads >= 3) {
          entry.status = "ready";
          entry.body = "这是隔离服务整理完成后返回的真实样本正文。";
        }
      }
      return reply(clone(entry));
    }
    const key = JSON.stringify([owner, fid, body.idempotencyKey]);
    if (receipts.has(key)) {
      const prior = receipts.get(key);
      if (JSON.stringify(prior.body) !== JSON.stringify(body))
        return reply(
          { code: "idempotency_conflict", message: "原输入不匹配" },
          409,
        );
      return reply(clone(prior.result));
    }
    let result;
    if (id === "generate") {
      let existing = items.find(
        (v) => v.date === body.localDate && v.timezone === body.timezone,
      );
      if (
        existing &&
        ["failed", "inactive"].includes(existing.status) &&
        body.expectedRevision !== existing.revision
      )
        return reply(
          { code: "diary_revision_conflict", message: "原日记版本变化" },
          409,
        );
      if (!existing) {
        existing = make(fid, "diary-generated-" + ++counter, body.localDate);
        Object.assign(existing, {
          timezone: body.timezone,
          status: "pending",
          body: null,
        });
        items.unshift(existing);
      } else if (["failed", "inactive"].includes(existing.status)) {
        existing.status = "pending";
        existing.revision++;
        existing.body = null;
      }
      result = {
        diary_id: existing.id,
        fluctlight_id: fid,
        status: existing.status,
        attempt: existing.latest_attempt,
        status_url: "/api/fluctlights/" + fid + "/diary/" + existing.id,
      };
    } else if (!entry)
      return reply({ code: "not_found", message: "日记不存在" }, 404);
    else if (part === "annotation") {
      if (body.expectedRevision !== entry.annotation.revision)
        return reply(
          { code: "diary_annotation_conflict", message: "批注版本已变化" },
          409,
        );
      entry.annotation = {
        note: body.note,
        favorite: body.favorite,
        revision: entry.annotation.revision + 1,
      };
      result = { diary_id: id, ...entry.annotation };
    } else if (part === "replies") {
      if (body.expectedRevision !== entry.revision)
        return reply(
          { code: "diary_revision_conflict", message: "日记版本已变化" },
          409,
        );
      const request = {
        id: "diary-reply-" + ++counter,
        fluctlight_id: fid,
        kind: "diary_reply",
        status: "pending",
        revision: 1,
        payload: { diary_id: id, text: body.text },
        result: {},
        result_refs: [],
        created_at: "2026-10-27T00:00:00Z",
      };
      requests.set(request.id, { value: request, reads: 0 });
      result = { request, audit_id: "diary-reply-audit" };
    } else return false;
    receipts.set(key, { body: clone(body), result: clone(result) });
    if (scenario === "unknown" && !lost) {
      lost = true;
      return reply(
        { code: "request_failed", message: "日记回执尚未确认" },
        503,
      );
    }
    return reply(result);
  };
}
