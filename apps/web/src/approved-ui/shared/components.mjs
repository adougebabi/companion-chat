import {
  defineComponent,
  ref,
  computed,
  onMounted,
  onUnmounted,
  nextTick,
  h,
  Fragment,
} from "vue";
import { useWorkspace } from "../workspace.mjs";
import { renderCurrentOutfit } from "../renderers/CurrentOutfit.mjs";
import { renderActionFeedback } from "../renderers/ActionFeedback.mjs";
import { renderFilterChips } from "../renderers/FilterChips.mjs";
import { renderDetailDrawer } from "../renderers/DetailDrawer.mjs";
import { renderGarmentArt } from "../renderers/GarmentArt.mjs";

/** Shared read-only wearing projection. Editing is an explicit navigation action. */
export const CurrentOutfit = defineComponent({
  name: "CurrentOutfit",
  props: { compact: Boolean },
  setup(props) {
    const workspace = useWorkspace();
    return renderCurrentOutfit({
      props,
      character: workspace.current,
      readStatus: computed(()=>workspace.state.errors?.wardrobe ? '穿着读取未完成，保留上次确认的记录。' : workspace.state.loading?.wardrobe ? '正在读取实际穿着…' : ''),
      t: () => {
        workspace.state.outfitTab = "为她搭配";
        workspace.navigate("衣柜");
      },
      outfitNames: (character, slot) =>
        !Array.isArray(character._wardrobe?.wearing?.worn_items) ? '状态待读取' : (character.wearing[slot] || [])
          .map((id) => character.garments.find((g) => g.id === id)?.name || character._wardrobe.wearing.worn_items.find(item=>(item.item_id||item.id)===id)?.description || '衣物名称待读取')
          .filter(Boolean)
          .join("、") || "未登记",
    });
  },
});
/** Inline failure feedback reuses the original operation's retry identity. */
export const ActionFeedback = defineComponent({
  name: "ActionFeedback",
  props: { labels: { type: Array, required: true },operation:{type:String,default:''},excludeOperationPrefix:{type:String,default:''},showStatus:Boolean },
  setup(props) {
    const workspace = useWorkspace();
    const latest = computed(() =>
      workspace.state.operations
        .filter(
          (r) =>
            r.fid === workspace.state.selectedId &&
            props.labels.includes(r.label) && (!props.operation || r.operation===props.operation) &&
            (!props.excludeOperationPrefix || !r.operation?.startsWith(props.excludeOperationPrefix)),
        )
        .at(-1),
    );
    const render = renderActionFeedback({ s: latest });
    return (context, cache) =>
      h(Fragment, [
        props.showStatus && latest.value && latest.value.status !== 'error'
          ? h('p',{class:'muted',role:'status','data-testid':'request-status'},latest.value.status === 'pending' ? '换装请求已受理，等待处理；当前穿着以实际记录为准。' : latest.value.response || '请求已处理，请核对实际结果。') : null,
        render(context, cache),
        latest.value?.readbackError
          ? h("p", { class: "inline-error", role: "alert" }, [
              latest.value.readbackError + " ",
              h("button", { class: "text-button", onClick: () => latest.value.refreshResult() }, "重新读取页面结果"),
            ])
          : null,
        latest.value?.originalInput
          ? h("details", { class: "inline-error" }, [
              h("summary", "查看等待核对的原提交"),
              h("pre", latest.value.originalInput),
            ])
          : null,
        latest.value?.review
          ? h("div", { class: "inline-error" }, [
              h(
                "p",
                "原提交已明确因版本冲突被拒绝，草稿仍保留。读取最新对象后，请重新核对变更。",
              ),
              h(
                "button",
                { class: "button", onClick: () => latest.value.release() },
                "读取最新版本并保留草稿",
              ),
            ])
          : null,
      ]);
  },
});
/** Props: search:string, filters:Record<string,string>, groups:{key,label,options[]}[]. */
export const FilterChips = defineComponent({
  name: "FilterChips",
  props: {
    search: { type: String, default: "" },
    filters: { type: Object, required: true },
    groups: { type: Array, required: true },
    placeholder: { type: String, default: "搜索…" },
    testId: String,
  },
  emits: ["update:search", "update:filters", "clear"],
  setup(props, { emit }) {
    const hasFilters = computed(
      () => !!props.search.trim() || Object.values(props.filters).some(Boolean),
    );
    return renderFilterChips({
      props,
      n: emit,
      l: hasFilters,
      i: (key, value) =>
        emit("update:filters", {
          ...props.filters,
          [key]: props.filters[key] === value ? "" : value,
        }),
      r: () => {
        emit("update:search", "");
        emit(
          "update:filters",
          Object.fromEntries(props.groups.map((g) => [g.key, ""])),
        );
        emit("clear");
      },
    });
  },
});
/** One focus-trapped shared drawer; slots hold the unchanged page-specific form. */
export const DetailDrawer = defineComponent({
  name: "DetailDrawer",
  props: { title: { type: String, required: true }, testId: String },
  emits: ["close"],
  setup(props, { emit }) {
    const panel = ref();
    let previous;
    const keydown = (event) => {
      if (event.key === "Escape") emit("close");
      if (event.key !== "Tab") return;
      const controls = [
        ...(panel.value?.querySelectorAll(
          'button:not(:disabled),input:not(:disabled),textarea:not(:disabled),select:not(:disabled),a[href],summary,[tabindex="0"]',
        ) || []),
      ].filter((n) => n.getClientRects().length);
      if (!controls.length) return;
      const first = controls[0],
        last = controls.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    onMounted(() => {
      previous = document.activeElement;
      document.addEventListener("keydown", keydown);
      nextTick(() => panel.value?.focus());
    });
    onUnmounted(() => {
      document.removeEventListener("keydown", keydown);
      previous?.focus({ preventScroll: true });
    });
    return renderDetailDrawer({ props, s: emit, n: panel });
  },
});
/** Decorative garment type illustration, not a generated photo or wearing proof. */
export const GarmentArt = defineComponent({
  name: "GarmentArt",
  props: { type: { type: String, required: true } },
  setup: (props) => renderGarmentArt({ props }),
});
