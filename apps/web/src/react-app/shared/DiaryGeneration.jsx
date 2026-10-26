import { useController } from "@/react-app/page";
import { selector, readValue } from "@/state/view-model";
import { ActionButton, FieldInput } from "@/react-app/controls";
import { displayText } from "@/react-app/render-helpers";
import { day } from "@/state/workspace.mjs";
import {
  previousDiaryDate,
  diaryRequestText,
} from "@/approved-ui/diary-generation.mjs";
import { errorMessage } from "@/v3/domain-presentation.mjs";
function createDiaryGenerationController(__props, { emit: __emit }, lifecycle) {
  const props = __props,
    emit = __emit;
  const w = lifecycle.workspace,
    scope = w.currentScope(),
    fid = w.current.value.id;
  const receipt = lifecycle.cell(null),
    error = lifecycle.cell(""),
    busy = lifecycle.cell(false),
    clock = lifecycle.cell(Date.now());
  const maximum = selector(() =>
    previousDiaryDate(
      day(clock.value, w.current.value.timezone || "Asia/Shanghai"),
    ),
  );
  const target = selector(() => props.date || maximum.value);
  let active = true,
    epoch = 0,
    timer,
    reading = false;
  const live = () => active && w.currentScope() === scope;
  const pending = selector(
    () =>
      busy.value ||
      ["pending", "running"].includes(receipt.value?.status) ||
      w.state.operations.some(
        (r) => r.fid === fid && r.label === "日记" && r.status === "pending",
      ),
  );
  async function read(id) {
    const token = ++epoch;
    try {
      const actual = await w.services.diaryClient.read(fid, id);
      if (!live() || token !== epoch || actual.local_date !== target.value)
        return;
      receipt.value = actual;
      error.value = "";
      if (actual.status === "completed") void w.loadLife();
    } catch (cause) {
      if (live() && token === epoch)
        error.value = errorMessage(cause, "状态暂未读到，原日记请求已保留。");
    }
  }
  lifecycle.observe(
    () => [props.date, w.current.value.life.diaryStates],
    () => {
      epoch++;
      receipt.value =
        (w.current.value.life.diaryStates || []).find(
          (d) => d.local_date === target.value,
        ) || null;
      error.value = "";
    },
    {
      immediate: true,
    },
  );
  async function generate() {
    if (pending.value || !live()) return;
    const date = target.value;
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || date > maximum.value) {
      error.value = "请选择已经结束的日期，今天的日记需要等这一天结束后回顾。";
      return;
    }
    busy.value = true;
    error.value = "";
    try {
      const command = w.command(
        "日记",
        "diary:" + date,
        {
          localDate: date,
          timezone: w.current.value.timezone || "Asia/Shanghai",
          reason: "回顾所选已结束日期的真实经历",
        },
        (value, idempotencyKey) =>
          w.client.generateDiary(fid, {
            ...value,
            idempotencyKey,
          }),
        {
          success: date + " 的日记回顾已受理",
        },
      );
      const result = await command.completion;
      if (!live() || target.value !== date) return;
      if (result?.diary_id) {
        receipt.value = {
          id: result.diary_id,
          local_date: date,
          status: result.status,
        };
        await read(result.diary_id);
      } else
        error.value = "日记请求已提交，回执尚未完整读取，请查看本次操作记录。";
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    } finally {
      if (live()) busy.value = false;
    }
  }
  async function poll() {
    clock.value = Date.now();
    if (
      !live() ||
      document.hidden ||
      reading ||
      busy.value ||
      !receipt.value?.id ||
      !["pending", "running"].includes(receipt.value.status)
    )
      return;
    reading = true;
    try {
      await read(receipt.value.id);
    } finally {
      reading = false;
    }
  }
  const visible = () => {
    if (!document.hidden) void poll();
  };
  lifecycle.onStart(() => {
    timer = setInterval(() => void poll(), 5000);
    document.addEventListener("visibilitychange", visible);
  });
  lifecycle.onDispose(() => {
    active = false;
    epoch++;
    clearInterval(timer);
    document.removeEventListener("visibilitychange", visible);
  });
  return (_ctx, _cache) => {
    return (
      <div className="diary-generation">
        <div className="row">
          <label>
            {"回顾日期"}
            <FieldInput
              type="date"
              value={target.value}
              max={maximum.value}
              disabled={busy.value}
              onInput={($event) => emit("select", $event.target.value)}
            />
          </label>
          <ActionButton
            className="button primary"
            data-testid="diary-generate"
            disabled={pending.value}
            onClick={generate}
          >
            {displayText(
              pending.value ? "正在回顾…" : "补写 " + target.value + " 日记",
            )}
          </ActionButton>
        </div>
        <small className="muted">
          {"只回顾已经结束的一天；今天结束后，再整理今天的经历。"}
        </small>
        {receipt.value ? (
          <p key={0} role="status">
            {displayText(readValue(diaryRequestText)(receipt.value)) + " "}
            {receipt.value.workflow_id ? (
              <ActionButton asChild className="text-button" key={0}>
                <a
                  key={0}
                  className="text-button"
                  href={
                    "#/diag/task/" +
                    encodeURIComponent(receipt.value.workflow_id)
                  }
                >
                  {"查看原任务"}
                </a>
              </ActionButton>
            ) : null}
          </p>
        ) : null}
        {error.value ? (
          <p key={1} className="inline-error" role="alert">
            {displayText(error.value)}
          </p>
        ) : null}
      </div>
    );
  };
}
export default function DiaryGeneration(input = {}) {
  return useController(createDiaryGenerationController, {
    date: "",
    ...input,
  });
}
