// Production-shaped schedule APIs for isolated UI verification only.
export function scheduleReplayFixture({
  owner,
  nowSchedule,
  scenario = "",
  lifeScenario = "",
}) {
  let sequence = 0,
    itemWrites = 0,
    planWrites = 0;
  const calendars = new Map(),
    contexts = new Map(),
    requests = new Map(),
    plans = new Map(),
    commands = new Map();
  const today = () => nowSchedule().local_date;
  const context = (fid) => contexts.get(fid) || "life-fixture-1";
  const calendarKey = (fid, date) => JSON.stringify([fid, date]);
  const original = (fid, date) => {
    const key = calendarKey(fid, date);
    if (!calendars.has(key))
      calendars.set(
        key,
        date === today() && lifeScenario !== "empty"
          ? {
              status: "accepted",
              reschedule_policy: null,
              ...structuredClone(nowSchedule()),
              ...(scenario === "protected-empty" ? { items: [] } : {}),
            }
          : null,
      );
    return calendars.get(key);
  };
  const changed = (fid) => {
    const value =
      "life-fixture-" + (Number(context(fid).split("-").at(-1)) + 1);
    contexts.set(fid, value);
    return value;
  };
  const stamp = (date, time) => date + "T" + time + ":00+08:00";
  const requestFor = (fid, date) =>
    [...plans.values()].find(
      (value) => value.fluctlight_id === fid && value.local_date === date,
    ) || null;
  function commitItem(record) {
    const payload = record.payload,
      schedule = original(record.fluctlight_id, payload.local_date);
    if (!schedule) return;
    const item = schedule.items.find((value) => value.id === payload.item_id);
    if (payload.operation === "cancel" && item) item.status = "cancelled";
    else if (item)
      Object.assign(item, {
        activity: payload.activity,
        start_at: payload.start_at,
        end_at: payload.end_at,
        scene: payload.scene,
        location: payload.location,
        place_id: payload.place_id,
      });
    schedule.revision++;
    changed(record.fluctlight_id);
  }
  const handler = ({ path, method, body, url, json, res }) => {
    const send = (value, status = 200) => {
      json(res, value, status);
      return true;
    };
    const match = /^\/api\/fluctlights\/([^/]+)\/(.*)$/.exec(path);
    if (match) {
      const [, fid, part] = match;
      if (part === "schedules" && method === "GET") {
        const from = url.searchParams.get("from") || today(),
          to = url.searchParams.get("to") || from;
        const days = [];
        for (let current = from; current <= to && days.length < 31;) {
          days.push({
            local_date: current,
            schedule: structuredClone(original(fid, current)),
            request: structuredClone(requestFor(fid, current)),
          });
          const next = new Date(current + "T12:00:00Z");
          if (!Number.isFinite(next.getTime())) break;
          next.setUTCDate(next.getUTCDate() + 1);
          current = next.toISOString().slice(0, 10);
        }
        return send({
          today: today(),
          timezone: "Asia/Shanghai",
          from,
          to,
          context_revision: context(fid),
          days,
        });
      }
      if (part === "owner-requests" && method === "GET")
        return send({
          items: [...requests.values()]
            .filter((value) => value.fluctlight_id === fid)
            .map((value) => structuredClone(value)),
          next_cursor: "",
        });
      const itemMatch = /^schedules\/([^/]+)\/items\/([^/]+)$/.exec(part);
      if (itemMatch && ["PUT", "DELETE"].includes(method)) {
        const commandKey = JSON.stringify([owner, fid, body.idempotencyKey]);
        if (commands.has(commandKey))
          return send(structuredClone(commands.get(commandKey)));
        const schedule = original(fid, today());
        if (scenario === "conflict" && itemWrites++ === 0 && schedule) {
          schedule.revision++;
          changed(fid);
        }
        if (
          !schedule ||
          schedule.id !== itemMatch[1] ||
          body.expectedRevision !== schedule.revision ||
          body.expectedLifeContextRevision !== context(fid)
        )
          return send(
            {
              code: "revision_conflict",
              message: "日程或生活版本已变化，请核对原建议。",
            },
            409,
          );
        const id = "schedule-owner-request-" + ++sequence,
          record = {
            id,
            kind: "schedule_item_suggestion",
            fluctlight_id: fid,
            status: "pending",
            payload: {
              operation: method === "DELETE" ? "cancel" : "update",
              schedule_id: schedule.id,
              item_id: itemMatch[2],
              local_date: today(),
              activity: body.activity,
              start_at: body.startAt,
              end_at: body.endAt,
              location: body.location,
              scene: body.scene,
              place_id: body.placeId,
              reason: body.reason,
            },
            decided_by: null,
            decision_reason: null,
            result: {},
            result_refs: [],
            tasks_state: "available",
            tasks: [],
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
            revision: 1,
          };
        requests.set(id, record);
        const envelope = {
          request: structuredClone(record),
          audit_id: "schedule-audit-" + sequence,
        };
        commands.set(commandKey, envelope);
        if (scenario === "unknown-item" && itemWrites++ === 0)
          return send(
            {
              code: "request_failed",
              message: "回执连接暂时中断，原建议可能已经受理。",
            },
            503,
          );
        return send(envelope, 202);
      }
      if (part === "schedules/requests" && method === "POST") {
        const commandKey = JSON.stringify([owner, fid, body.idempotencyKey]);
        if (commands.has(commandKey))
          return send(structuredClone(commands.get(commandKey)));
        if (scenario === "plan-conflict" && planWrites++ === 0) {
          changed(fid);
          return send(
            {
              code: "revision_conflict",
              message: "规划前生活版本发生变化，请核对原请求。",
            },
            409,
          );
        }
        if (body.expectedLifeContextRevision !== context(fid))
          return send(
            { code: "revision_conflict", message: "生活版本已变化" },
            409,
          );
        const request_id = "schedule-plan-request-" + ++sequence,
          record = {
            request_id,
            fluctlight_id: fid,
            kind: "generate",
            local_date: body.localDate,
            timezone: "Asia/Shanghai",
            status: "queued",
            result: {},
            error_code: null,
            task_id: "fixture-schedule-task-" + sequence,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          };
        plans.set(request_id, record);
        commands.set(commandKey, structuredClone(record));
        if (scenario === "unknown-plan" && planWrites++ === 0)
          return send(
            {
              code: "request_failed",
              message: "规划回执暂时无法读取，原请求已保留。",
            },
            503,
          );
        return send(record, 202);
      }
      const planMatch = /^schedules\/requests\/([^/]+)$/.exec(part);
      if (planMatch && method === "GET") {
        const record = plans.get(planMatch[1]);
        if (!record || record.fluctlight_id !== fid)
          return send(
            { code: "not_found", message: "没有读到这个原规划请求" },
            404,
          );
        if (record.status === "queued") record.status = "running";
        else if (record.status === "running") {
          const existing = original(fid, record.local_date);
          record.status = "completed";
          record.result = {
            disposition: existing ? "preserved" : "generated",
            local_date: record.local_date,
          };
          if (!existing)
            calendars.set(calendarKey(fid, record.local_date), {
              id: "generated-" + record.request_id,
              revision: 1,
              local_date: record.local_date,
              timezone: "Asia/Shanghai",
              status: "accepted",
              reschedule_policy: null,
              items: [
                {
                  id: "generated-morning",
                  start_at: stamp(record.local_date, "09:30"),
                  end_at: stamp(record.local_date, "11:30"),
                  activity: "整理手边的事情",
                  scene: "日常",
                  location: "家中",
                  status: "scheduled",
                  action_status: null,
                },
                {
                  id: "generated-afternoon",
                  start_at: stamp(record.local_date, "16:00"),
                  end_at: stamp(record.local_date, "17:00"),
                  activity: "附近散步",
                  scene: "日常",
                  location: "街区",
                  status: "scheduled",
                  action_status: null,
                },
              ],
            });
          changed(fid);
        }
        record.updated_at = new Date().toISOString();
        return send(structuredClone(record));
      }
    }
    const ownerMatch = /^\/api\/owner-requests\/([^/]+)$/.exec(path);
    if (ownerMatch && method === "GET" && requests.has(ownerMatch[1])) {
      const record = requests.get(ownerMatch[1]);
      if (record.status === "pending") {
        record.status = "processing";
        record.revision++;
      } else if (record.status === "processing") {
        record.status = scenario === "rejected" ? "rejected" : "completed";
        record.decided_by = "core_persona_assessment";
        record.decision_reason =
          scenario === "rejected"
            ? "这段安排暂时保留，稍后再商量。"
            : "已根据建议更新实际安排。";
        if (record.status === "completed") {
          commitItem(record);
          record.result = { mutation_committed: true };
          record.result_refs = ["schedule:" + record.payload.schedule_id];
        }
        record.revision++;
      }
      record.updated_at = new Date().toISOString();
      return send(structuredClone(record));
    }
    return false;
  };
  return {
    handler,
    schedule: (fid, date = today()) => structuredClone(original(fid, date)),
    context,
  };
}
