import { joined } from "../presentation.mjs";
import { defineComponent, ref, computed, watch, nextTick } from "vue";
import { useWorkspace, slots, day } from "../workspace.mjs";
import { zonedInputInstant } from "../../v3/zoned-input.mjs";
import { errorMessage } from "../../v3/domain-presentation.mjs";
import { renderLifePage } from "../renderers/LifePage.mjs";
import {
  CurrentOutfit,
  FilterChips,
  DetailDrawer,
  ActionFeedback,
} from "../shared/components.mjs";
export default defineComponent({
  name: "LifePage",
  props: { page: { type: String, required: true } },
  emits: ["go"],
  setup(props, { emit }) {
    const w = useWorkspace(),
      character = w.current,
      { lifeClient, memoryClient, diaryClient } = w.services;
    const life = computed(() => character.value.life),
      busy = (labels) =>
        computed(() =>
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
    const schedule = computed(() =>
      [...character.value.schedule].sort((a, b) =>
        a.time.localeCompare(b.time),
      ),
    );
    const active = computed(() =>
      schedule.value.find((v) => v._source?.action_status === "in_progress"),
    );
    const upcoming = computed(() =>
      schedule.value.filter(
        (v) =>
          v === active.value ||
          !v._source?.end_at ||
          Date.parse(v._source.end_at) > Date.now(),
      ),
    );
    const editing = ref(null),
      time = ref(""),
      text = ref("");
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
      const detail = { ...character.value.detail, schedule: item._schedule };
      const record = w.command(
        "日程",
        "schedule:" + source.id,
        { detail, source, body },
        ({ detail, source, body }, key) =>
          w.client.updateScheduleItem(fid, detail.schedule.id, source.id, {
            ...body,
            expectedRevision: detail.schedule.revision,
            expectedLifeContextRevision: detail.context.context_revision,
            evidenceRefs: ["owner:" + w.state.session.actorId],
            idempotencyKey: key,
          }),
        { onSaved: () => w.loadLife(), success: "日程建议已提交" },
      );
      try {
        await record.completion;
        if (
          editing.value === submitted.index &&
          time.value === submitted.time &&
          text.value === submitted.text
        )
          editing.value = null;
      } catch {
        /* Shared feedback preserves the draft. */
      }
    }
    const search = ref(""),
      filters = ref({ category: "", tag: "" }),
      detailMemory = ref(null),
      draftMemory = ref(null),
      forgetMemory = ref(null),
      newTag = ref("");
    const tags = computed(() => [
      ...new Set(
        life.value.memories
          .flatMap((m) => m.tags)
          .concat(draftMemory.value?.tags || []),
      ),
    ]);
    const groups = computed(() => [
      {
        key: "category",
        label: "类型",
        options: ["长期记忆", "发生过的事情", "最近在想"],
      },
      { key: "tag", label: "标签", options: tags.value },
    ]);
    const memories = computed(() =>
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
        { source: value._source, content: value.text },
        (body) =>
          memoryClient.revise(
            fid,
            body.source,
            body.content,
            "在记忆页面更正记录",
          ),
        { onSaved: () => w.loadLife(), success: "记忆正文已更正" },
      );
      try {
        await record.completion;
        if (
          draftMemory.value?.id === value.id &&
          draftMemory.value?.text === submitted
        )
          draftMemory.value = null;
      } catch {
        /* The edit and original revision remain available. */
      }
    }
    function toggleGoal(goal) {
      const fid = character.value.id;
      return w.command(
        "目标",
        "goal:" + goal.id,
        { id: goal.id, revision: goal._source.revision },
        (body, key) =>
          w.client.goalCommand(fid, body.id, "reassess", {
            expectedRevision: body.revision,
            reason: "请求核对目标的实际完成情况",
            idempotencyKey: key,
          }),
        { onSaved: () => w.loadLife(), success: "已请求重新核对目标进展" },
      );
    }
    const diaries = computed(() =>
        [...life.value.diaries].sort((a, b) => a.date.localeCompare(b.date)),
      ),
      selectedDate = ref(""),
      direction = ref("page-next");
    watch(
      diaries,
      (rows) => {
        if (!rows.some((d) => d.date === selectedDate.value))
          selectedDate.value = rows.at(-1)?.date || "";
      },
      { immediate: true },
    );
    const diaryIndex = computed(() =>
        diaries.value.findIndex((d) => d.date === selectedDate.value),
      ),
      entry = computed(() => diaries.value[diaryIndex.value]);
    const selectDate = (date) => {
      if (
        diaries.value.some((d) => d.date === date) &&
        date !== selectedDate.value
      ) {
        direction.value = date > selectedDate.value ? "page-next" : "page-prev";
        selectedDate.value = date;
      }
    };
    async function generateDiary() {
      const fid = character.value.id,
        today = day(new Date(), character.value.timezone),
        previous = new Date(Date.parse(today + "T00:00:00Z") - 86400000)
          .toISOString()
          .slice(0, 10);
      const date =
        selectedDate.value && selectedDate.value < today
          ? selectedDate.value
          : previous;
      const body = {
        localDate: date,
        timezone: character.value.timezone || "Asia/Shanghai",
        reason: "请求回顾已结束日期的真实经历",
      };
      const record = w.command(
        "日记",
        "diary:" + date,
        body,
        (value, key) =>
          w.client.generateDiary(fid, { ...value, idempotencyKey: key }),
        { onSaved: () => w.loadLife(), success: "日记生成请求已受理" },
      );
      try {
        const result = await record.completion;
        if (result?.diary_id)
          w.notify("已提交 " + date + " 的日记回顾，可稍后回来查看");
      } catch {
        /* Inline action feedback owns failure and retry. */
      }
    }
    watch(
      () => props.page,
      async (page) => {
        editing.value = null;
        draftMemory.value = null;
        forgetMemory.value = null;
        if (page === "目标" && w.state.appointmentHighlight) {
          await nextTick();
          document
            .getElementById("appointments")
            ?.scrollIntoView({ block: "center" });
        } else if (page !== "目标") w.state.appointmentHighlight = false;
      },
    );
    const calendarMonth = computed(() =>
      String(
        selectedDate.value || day(new Date(), character.value.timezone),
      ).slice(0, 7),
    );
    const calendarTitle = computed(() => {
      const [year, month] = calendarMonth.value.split("-");
      return year + " 年 " + Number(month) + " 月";
    });
    const calendarDays = computed(() => {
      const [year, month] = calendarMonth.value.split("-").map(Number);
      return new Date(Date.UTC(year, month, 0)).getUTCDate();
    });
    const calendarOffset = computed(() => {
      const [year, month] = calendarMonth.value.split("-").map(Number);
      return (new Date(Date.UTC(year, month - 1, 1)).getUTCDay() + 6) % 7;
    });
    async function forget() {
      const memory = forgetMemory.value,
        fid = character.value.id;
      if (!memory) return;
      const record = w.command(
        "记忆",
        "forget-memory:" + memory.id,
        { source: memory._source },
        (body) => memoryClient.forget(fid, body.source, "明确遗忘这段记忆"),
        { onSaved: () => w.loadLife(), success: "记忆已遗忘，原修订历史保留" },
      );
      try {
        await record.completion;
        forgetMemory.value = null;
      } catch {}
    }
    const todayLabel = computed(() =>
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
      joined,
      props,
      todayLabel,
      calendarMonth,
      calendarTitle,
      calendarDays,
      calendarOffset,
      forgetMemory: forget,
      character,
      CurrentOutfit,
      FilterChips,
      DetailDrawer,
      ActionFeedback,
      appointmentHighlight: computed(() => w.state.appointmentHighlight),
      n: emit,
      l: life,
      c: upcoming,
      u: schedule,
      f: active,
      d: editing,
      B: saveSchedule,
      C: time,
      w: text,
      r: scheduleBusy,
      A: editSchedule,
      i: diaryBusy,
      F: generateDiary,
      ee: diaries,
      he: selectedDate,
      R: selectDate,
      j: direction,
      x: entry,
      T: diaryIndex,
      ye: (step) => {
        const d = diaries.value[diaryIndex.value + step];
        if (d) selectDate(d.date);
      },
      te: search,
      se: filters,
      ue: groups,
      S: memories,
      I: detailMemory,
      K: draftMemory,
      H: newTag,
      le: forgetMemory,
      q: () => {
        search.value = "";
        filters.value = { category: "", tag: "" };
      },
      _: saveMemory,
      ne: tags,
      V: toggleTag,
      U: addTag,
      a: memoryBusy,
      toggleGoal,
    };
    return renderLifePage(bindings);
  },
});
