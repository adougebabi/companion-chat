import {currentReadError} from '@/approved-ui/read-feedback.mjs';
import { useController, afterPaint } from "@/react-app/page";
import { selector } from "@/state/view-model";
import { joined } from "@/approved-ui/presentation.mjs";
import { day } from "@/state/workspace.mjs";
import { zonedInputInstant } from "@/v3/zoned-input.mjs";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { LifePageView } from "@/react-app/views/LifePage";
import DiaryGeneration from "@/react-app/shared/DiaryGeneration";
import { previousDiaryDate } from "@/approved-ui/diary-generation.mjs";
import {
  CurrentOutfit,
  FilterChips,
  DetailDrawer,
  ActionFeedback,
} from "@/react-app/components";
function createLifePageController(props, { emit }, lifecycle) {
  const w = lifecycle.workspace,
    character = w.current,
    { lifeClient, memoryClient, diaryClient } = w.services;
  const life = selector(() => character.value.life),
    busy = (labels) =>
      selector(() =>
        w.state.operations.some(
          (r) =>
            r.fid === character.value.id &&
            labels.includes(r.label) &&
            r.status === "pending",
        ),
      );
  const diaryBusy = busy(["日记"]),
    scheduleBusy = busy(["日程"]),
    memoryBusy = busy(["记忆"]);
  const schedule = selector(() =>
    [...character.value.schedule].sort((a, b) => a.time.localeCompare(b.time)),
  );
  const active = selector(() =>
    schedule.value.find((v) => v._source?.action_status === "in_progress"),
  );
  const upcoming = selector(() =>
    schedule.value.filter(
      (v) =>
        v === active.value ||
        !v._source?.end_at ||
        Date.parse(v._source.end_at) > Date.now(),
    ),
  );
  const editing = lifecycle.cell(null),
    time = lifecycle.cell(""),
    text = lifecycle.cell("");
  function editSchedule(item, event) {
    event.target.closest("details")?.removeAttribute("open");
    editing.value = character.value.schedule.indexOf(item);
    time.value = item.time;
    text.value = item.text;
  }
  async function saveSchedule() {
    const fid = character.value.id,
      item = character.value.schedule[editing.value];
    if (!item || !text.value.trim() || !time.value) return;
    const source = item._source,
      start = item._source.start_at;
    const date = day(start, character.value.timezone);
    let startAt;
    try {
      startAt = zonedInputInstant(
        date + "T" + time.value,
        character.value.timezone || "Asia/Shanghai",
        start,
      );
    } catch (error) {
      w.notify(errorMessage(error));
      return;
    }
    const body = {
      activity: text.value.trim(),
      startAt,
      endAt: source.end_at,
      scene: source.scene || "",
      location: source.location || "",
      placeId: source.place_id || "",
      reason: "提出日程条目修改建议",
    };
    const submitted = {
      index: editing.value,
      time: time.value,
      text: text.value,
    };
    const detail = {
      ...character.value.detail,
      schedule: item._schedule,
    };
    const record = w.command(
      "日程",
      "schedule:" + source.id,
      {
        detail,
        source,
        body,
      },
      ({ detail, source, body }, key) =>
        w.client.updateScheduleItem(fid, detail.schedule.id, source.id, {
          ...body,
          expectedRevision: detail.schedule.revision,
          expectedLifeContextRevision: detail.context.context_revision,
          evidenceRefs: ["owner:" + w.state.session.actorId],
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadLife(),
        success: "日程建议已提交",
      },
    );
    try {
      await record.completion;
      if (
        editing.value === submitted.index &&
        time.value === submitted.time &&
        text.value === submitted.text
      )
        editing.value = null;
    } catch {}
  }
  const search = lifecycle.cell(""),
    filters = lifecycle.cell({
      category: "",
      tag: "",
    }),
    selectedMemoryId = lifecycle.cell(null),
    draftMemory = lifecycle.cell(null),
    forgetMemory = lifecycle.cell(null),
    newTag = lifecycle.cell("");
  const tags = selector(() => [
    ...new Set(
      life.value.memories
        .flatMap((m) => m.tags)
        .concat(draftMemory.value?.tags || []),
    ),
  ]);
  const groups = selector(() => [
    {
      key: "category",
      label: "类型",
      options: ["长期记忆", "发生过的事情", "最近在想"],
    },
    {
      key: "tag",
      label: "标签",
      options: tags.value,
    },
  ]);
  const memories = selector(() =>
    life.value.memories.filter(
      (m) =>
        [m.text, m.source, ...m.tags]
          .join(" ")
          .toLowerCase()
          .includes(search.value.trim().toLowerCase()) &&
        (!filters.value.category || filters.value.category === m.category) &&
        (!filters.value.tag || m.tags.includes(filters.value.tag)),
    ),
  );
  lifecycle.observe(
    () => [props.initialMemoryId, life.value.memories],
    () => {
      if (props.initialMemoryId)
        selectedMemoryId.value = life.value.memories.some(
          (item) => item.id === props.initialMemoryId,
        )
          ? props.initialMemoryId
          : null;
    },
    {
      immediate: true,
    },
  );
  function toggleTag(tag) {
    if (!draftMemory.value) return;
    draftMemory.value.tags = draftMemory.value.tags.includes(tag)
      ? draftMemory.value.tags.filter((t) => t !== tag)
      : [...draftMemory.value.tags, tag];
  }
  function addTag() {
    const tag = newTag.value.trim().replace(/^#\s*/, "");
    if (tag && draftMemory.value && !draftMemory.value.tags.includes(tag))
      draftMemory.value.tags.push(tag);
    newTag.value = "";
  }
  async function saveMemory() {
    const value = draftMemory.value,
      fid = character.value.id;
    if (!value?.text.trim()) return;
    const submitted = value.text;
    const record = w.command(
      "记忆",
      "memory:" + value.id,
      {
        source: value._source,
        content: value.text,
      },
      (body) =>
        memoryClient.revise(
          fid,
          body.source,
          body.content,
          "在记忆页面更正记录",
        ),
      {
        onSaved: () => w.loadLife(),
        success: "记忆正文已更正",
      },
    );
    try {
      await record.completion;
      if (
        draftMemory.value?.id === value.id &&
        draftMemory.value?.text === submitted
      )
        draftMemory.value = null;
    } catch {}
  }
  function toggleGoal(goal) {
    const fid = character.value.id;
    return w.command(
      "目标",
      "goal:" + goal.id,
      {
        id: goal.id,
        revision: goal._source.revision,
      },
      (body, key) =>
        w.client.goalCommand(fid, body.id, "reassess", {
          expectedRevision: body.revision,
          reason: "请求核对目标的实际完成情况",
          idempotencyKey: key,
        }),
      {
        onSaved: () => w.loadLife(),
        success: "已请求重新核对目标进展",
      },
    );
  }
  const diaries = selector(() =>
      [...life.value.diaries].sort((a, b) => a.date.localeCompare(b.date)),
    ),
    selectedDate = lifecycle.cell(""),
    direction = lifecycle.cell("page-next");
  lifecycle.observe(
    diaries,
    (rows) => {
      if (!selectedDate.value)
        selectedDate.value =
          rows.at(-1)?.date ||
          previousDiaryDate(
            day(new Date(), character.value.timezone || "Asia/Shanghai"),
          );
    },
    {
      immediate: true,
    },
  );
  const diaryIndex = selector(() =>
      diaries.value.findIndex((d) => d.date === selectedDate.value),
    ),
    entry = selector(() => diaries.value[diaryIndex.value]);
  const selectDate = (date) => {
    if (
      /^\d{4}-\d{2}-\d{2}$/.test(date) &&
      date <=
        previousDiaryDate(
          day(new Date(), character.value.timezone || "Asia/Shanghai"),
        ) &&
      date !== selectedDate.value
    ) {
      direction.value = date > selectedDate.value ? "page-next" : "page-prev";
      selectedDate.value = date;
    }
  };
  lifecycle.observe(
    () => props.page,
    async (page) => {
      editing.value = null;
      selectedMemoryId.value = null;
      draftMemory.value = null;
      forgetMemory.value = null;
      if (page === "目标" && w.state.appointmentHighlight) {
        await afterPaint();
        document.getElementById("appointments")?.scrollIntoView({
          block: "center",
        });
      } else if (page !== "目标") w.state.appointmentHighlight = false;
    },
  );
  async function forget() {
    const memory = forgetMemory.value,
      fid = character.value.id;
    if (!memory) return;
    const record = w.command(
      "记忆",
      "forget-memory:" + memory.id,
      {
        source: memory._source,
      },
      (body) => memoryClient.forget(fid, body.source, "明确遗忘这段记忆"),
      {
        onSaved: () => w.loadLife(),
        success: "记忆已遗忘，原修订历史保留",
      },
    );
    try {
      await record.completion;
      forgetMemory.value = null;
    } catch {}
  }
  const todayLabel = selector(() =>
    new Intl.DateTimeFormat("en-US", {
      timeZone: character.value.timezone || "Asia/Shanghai",
      weekday: "long",
      month: "long",
      day: "numeric",
    })
      .format(new Date())
      .toUpperCase(),
  );
  const bindings = {
    recoverInitialSchedule: () => {
      const fid = character.value.id;
      const record = w.command(
        "日程",
        "recover-initial-schedule",
        {
          reason: "恢复当前角色的原日程准备任务",
        },
        (body, key) =>
          w.client.recoverInitialSchedule(fid, {
            ...body,
            idempotencyKey: key,
          }),
        {
          onSaved: () => w.loadLife(),
          success: "已核对原日程任务，等待后台继续准备",
        },
      );
      void record.completion.catch(() => {});
    },
    joined,
    props,
    todayLabel,
    forget,
    character,
    readError:key=>currentReadError(w.state,key,w.currentScope()),
    openTimeline:()=>w.navigatePath("/c/"+encodeURIComponent(character.value.id)+"/life/timeline"),
    CurrentOutfit,
    FilterChips,
    DetailDrawer,
    ActionFeedback,
    appointmentHighlight: selector(() => w.state.appointmentHighlight),
    emit,
    life,
    upcoming,
    schedule,
    active,
    editing,
    saveSchedule,
    time,
    text,
    scheduleBusy,
    editSchedule,
    openSchedulePlanning: () => w.navigatePath("/c/" + encodeURIComponent(character.value.id) + "/life/schedule/planning"),
    diaryBusy,
    DiaryGeneration,
    closedDiaryDate: selector(() =>
      previousDiaryDate(
        day(new Date(), character.value.timezone || "Asia/Shanghai"),
      ),
    ),
    diaries,
    selectedDate,
    selectDate,
    direction,
    entry,
    diaryIndex,
    moveDiary: (step) => {
      const d = diaries.value[diaryIndex.value + step];
      if (d) selectDate(d.date);
    },
    search,
    filters,
    groups,
    memories,
    selectedMemoryId,
    draftMemory,
    newTag,
    forgetMemory,
    clearFilters: () => {
      search.value = "";
      filters.value = {
        category: "",
        tag: "",
      };
    },
    saveMemory,
    tags,
    toggleTag,
    addTag,
    memoryBusy,
    toggleGoal,
  };
  return () => <LifePageView view={bindings} />;
}
/** @param {{page: string, initialMemoryId?: string, onGo?: (page: string) => void}} input */
export default function LifePage(input) {
  return useController(createLifePageController, {
    ...input,
  });
}
