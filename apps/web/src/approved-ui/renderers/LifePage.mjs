import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, Fragment as O, createElementVNode as o, toDisplayString as v, unref as M, createVNode as me, renderList as oe, createTextVNode as Z, createCommentVNode as X, normalizeClass as ge, withModifiers as Ve, withDirectives as pe, vModelText as $e, vModelCheckbox as ss, normalizeStyle as yt, Transition as ru, withCtx as Ht, createBlock as Ae, withKeys as Gi } from 'vue';
const Qc = {
  class: "card now-card"
};
const Xc = {
  class: "grid now-grid"
};
const ed = {
  class: "card"
};
const td = {
  class: "card now-appointments"
};
const sd = {
  class: "row"
};
const nd = {
  key: 0,
  class: "muted"
};
const ld = {
  class: "card schedule-list",
  "data-testid": "schedule-list"
};
const id = ["disabled"];
const rd = ["disabled"];
const ad = {
  class: "schedule-text"
};
const ud = {
  key: 0,
  class: "pill",
  "data-testid": "schedule-current"
};
const cd = {
  key: 1,
  class: "actions-menu"
};
const dd = ["data-testid"];
const fd = {
  class: "scroll-lock-list",
  "data-scroll-lock": ""
};
const pd = ["disabled", "onClick"];
const od = ["data-testid"];
const md = {
  key: 0,
  class: "muted"
};
const vd = {
  class: "small-card-grid",
  "data-testid": "goal-grid"
};
const hd = {
  class: "row"
};
const gd = {
  class: "pill"
};
const yd = {
  class: "goal"
};
const bd = ["onUpdate:modelValue", "data-testid"];
const kd = ["data-testid"];
const _d = ["aria-label", "aria-valuenow"];
const wd = {
  class: "small-card-grid",
  "data-testid": "appointment-grid"
};
const $d = {
  class: "row diary-heading"
};
const Cd = ["disabled"];
const xd = {
  class: "diary-layout"
};
const Sd = {
  class: "card calendar"
};
const Td = {
  class: "calendar-grid"
};
const Ed = {
  key: 0
};
const Md = ["disabled", "aria-label", "aria-pressed", "data-testid", "onClick"];
const Ad = {
  class: "diary-book",
  "data-testid": "diary-book",
  "aria-label": "日记簿"
};
const Od = {
  class: "row diary-attributes"
};
const Vd = {
  class: "diary-text"
};
const Pd = {
  class: "diary-signature"
};
const Id = ["data-date"];
const Ld = {
  key: 1,
  class: "diary-paper empty-state"
};
const Ud = {
  class: "book-controls"
};
const Nd = ["disabled"];
const Fd = {
  "aria-live": "polite",
  "data-testid": "diary-page-indicator"
};
const Rd = ["disabled"];
const Dd = {
  class: "memory-list"
};
const Bd = {
  class: "memory-chevron"
};
const Kd = {
  class: "memory-content"
};
const Wd = {
  class: "pill"
};
const Hd = ["aria-expanded", "aria-controls", "data-testid", "onClick"];
const zd = ["aria-label"];
const Gd = {
  key: 0,
  class: "muted"
};
const Jd = {
  class: "row"
};
const Yd = ["data-testid", "onClick"];
const Zd = ["data-testid", "onClick"];
const qd = ["id"];
const jd = ["data-testid"];
const Qd = {
  key: 0,
  class: "card empty-state"
};
const Xd = {
  key: 0,
  class: "inline-error"
};
const ef = {
  class: "chip-field"
};
const tf = {
  class: "star-control",
  role: "group",
  "aria-label": "重要度"
};
const sf = ["aria-label", "aria-pressed", "data-testid", "onClick"];
const nf = {
  class: "chip-field"
};
const lf = {
  class: "chip-options scroll-lock-list",
  "data-scroll-lock": ""
};
const of = ["aria-pressed", "onClick"];
const rf = {
  class: "chip-field"
};
const af = {
  class: "chip-options scroll-lock-list",
  "data-scroll-lock": ""
};
const uf = ["aria-pressed", "data-testid", "onClick"];
const cf = {
  class: "input-row"
};
const df = ["disabled"];
const ff = ["disabled"];
const pf = {
  key: 2,
  class: "modal-backdrop"
};
const mf = {
  class: "card confirm-dialog",
  role: "alertdialog",
  "aria-modal": "true",
  "aria-label": "遗忘这段记忆？",
  "data-testid": "memory-forget-confirm"
};
const vf = {
  class: "row"
};
function renderLifePageBody(__view) {
  return (h, b) => (p(), g(O, null, [__view.props.page === "此刻" ? (p(), g(O, {
    key: 0
  }, [approvedSurfaceVNode("div", Qc, [o("span", {class:"eyebrow"}, v(M(__view.todayLabel)),1), o("h2", null, v(M(__view.character).activity||"此刻状态尚未更新"), 1), o("p", null, v(__view.joined(M(__view.character).location,M(__view.character).mood)), 1), o("p", null, v(M(__view.character).world), 1)]), o("div", Xc, [approvedSurfaceVNode("div", ed, [me(__view.CurrentOutfit, {
    compact: ""
  })]), approvedSurfaceVNode("section", td, [o("div", sd, [b[17] || (b[17] = o("h2", null, "约定", -1)), b[18] || (b[18] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    class: "text-button",
    "data-testid": "now-appointments",
    onClick: b[0] || (b[0] = k => {
      __view.appointmentHighlight.value = !0, __view.n("go", "目标");
    })
  }, "查看全部 ↗")]), (p(!0), g(O, null, oe(__view.l.value.appointments.slice(0, 2), k => (p(), g("div", {
    key: k.date + k.time,
    class: "appointment-row"
  }, [o("time", null, [Z(v(k.date.slice(5).replace("-", "月")) + "日", 1), b[19] || (b[19] = o("br", null, null, -1)), Z(v(k.time), 1)]), o("span", null, v(k.text), 1)]))), 128)), __view.l.value.appointments.length ? X("", !0) : (p(), g("p", nd, "还没有约定，留一些期待给明天。"))])])], 64)) : X("", !0), __view.props.page === "此刻" || __view.props.page === "日程" ? (p(), g(O, {
    key: 1
  }, [o("h2", null, v(__view.props.page === "此刻" ? "此刻与接下来" : "今天的安排"), 1), approvedSurfaceVNode("section", ld, [(p(!0), g(O, null, oe(__view.props.page === "此刻" ? __view.c.value : __view.u.value, k => (p(), g("div", {
    key: k.time + k.text,
    class: ge(["schedule-row", {
      "is-current": k === __view.f.value
    }]),
    "data-testid": "schedule-" + k.time
  }, [__view.d.value === M(__view.character).schedule.indexOf(k) && __view.props.page === "日程" ? (p(), g("form", {
    key: 0,
    class: "schedule-editor",
    onSubmit: Ve(__view.B, ["prevent"])
  }, [pe(o("input", {
    "onUpdate:modelValue": b[1] || (b[1] = ve => __view.C.value = ve),
    type: "time",
    "aria-label": "时间"
  }, null, 512), [[$e, __view.C.value]]), pe(o("input", {
    "onUpdate:modelValue": b[2] || (b[2] = ve => __view.w.value = ve),
    "aria-label": "日程内容"
  }, null, 512), [[$e, __view.w.value]]), approvedButtonVNode({
    class: "button",
    disabled: M(__view.r) || !__view.w.value.trim() || !__view.C.value,
    "data-testid": "schedule-save"
  }, v(M(__view.r) ? "处理中…" : "保存")), approvedButtonVNode({
    type: "button",
    class: "text-button",
    disabled: M(__view.r),
    onClick: b[3] || (b[3] = ve => __view.d.value = null)
  }, "取消")], 32)) : (p(), g(O, {
    key: 1
  }, [o("time", null, v(k.time), 1), b[20] || (b[20] = o("span", {
    class: "schedule-dot"
  }, null, -1)), o("span", ad, v(k.text), 1), k === __view.f.value ? (p(), g("span", ud, "进行中")) : X("", !0), __view.props.page === "日程" ? (p(), g("details", cd, [o("summary", {
    "aria-label": "日程操作",
    "data-testid": "schedule-menu-" + k.time
  }, "⋯", 8, dd), o("div", fd, [approvedButtonVNode({
    disabled: M(__view.r)||k.editable===false,
    onClick: ve => __view.A(k, ve)
  }, "编辑")])])) : X("", !0)], 64))], 10, od))), 128)), (__view.props.page === "此刻" ? __view.c.value : __view.u.value).length ? X("", !0) : (p(), g("div", md, [o("p",null,"暂时没有已确认的日程。"),__view.props.page==="日程"?approvedButtonVNode({class:"button",onClick:__view.openSchedulePlanning},"安排与调整日程"):X("",!0)]))])], 64)) : X("", !0), __view.props.page === "目标" ? (p(), g(O, {
    key: 2
  }, [b[24] || (b[24] = o("p", {
    class: "muted"
  }, "想做的事慢慢来，约好的事认真记住。", -1)), o("div", vd, [(p(!0), g(O, null, oe(M(__view.character).goals, (k, ve) => (p(), approvedSurfaceBlock("article", {
    key: k.id,
    class: "card goal-card"
  }, [o("div", hd, [o("span", gd, v(k.category||"目标"), 1), b[21] || (b[21] = o("span", {
    class: "spacer"
  }, null, -1)), o("small", null, v(k.done ? 100 : k.progress) + "%", 1)]), o("label", yd, [pe(o("input", {
    "onUpdate:modelValue": Ye => __view.toggleGoal(k, Ye),
    type: "checkbox", title:"请求核对进展，完成状态由实际证据决定",
    "data-testid": "goal-toggle-" + ve
  }, null, 8, bd), [[ss, k.done]]), o("span", {
    class: ge({
      done: k.done
    })
  }, v(k.text), 3)]), o("div", {
    class: "progress",
    role: "progressbar",
    "aria-label": k.text,
    "aria-valuenow": k.done ? 100 : k.progress,
    "aria-valuemin": 0,
    "aria-valuemax": 100
  }, [o("i", {
    style: yt({
      width: (k.done ? 100 : k.progress) + "%"
    }),
    "data-testid": "goal-progress-" + ve
  }, null, 12, kd)], 8, _d)]))), 128))]), o("section", {
    id: "appointments",
    class: ge({
      highlight: M(__view.appointmentHighlight)
    })
  }, [b[23] || (b[23] = o("h2", {
    class: "section-heading"
  }, "约定", -1)), o("div", wd, [(p(!0), g(O, null, oe([...__view.l.value.appointments].sort((k, ve) => (k.date + k.time).localeCompare(ve.date + ve.time)), k => (p(), approvedSurfaceBlock("article", {
    key: k.date + k.time,
    class: "card appointment-card"
  }, [o("small", null, v(k.date) + " · " + v(k.time), 1), o("h3", null, v(k.text), 1), b[22] || (b[22] = o("span", {
    class: "muted"
  }, "留一个位置，给共同的期待。", -1))]))), 128))])], 2)], 64)) : X("", !0), __view.props.page === "日记" ? (p(), g(O, {
    key: 3
  }, [o("div", $d, [b[25] || (b[25] = o("div", null, [o("h2", null, "写给自己的时光"), o("p", {
    class: "muted"
  }, "有些心事，留在纸页之间。")], -1)), b[26] || (b[26] = o("span", {
    class: "spacer"
  }, null, -1)), me(__view.DiaryGeneration,{date:__view.he.value,onSelect:__view.R})]), o("div", xd, [approvedSurfaceVNode("section", Sd, [o("h3", null, __view.calendarTitle.value, 1), o("div", Td, [(p(), g(O, null, oe(["一", "二", "三", "四", "五", "六", "日"], k => o("small", {
    key: k
  }, v(k), 1)), 64)), (p(), g(O, null, oe(__view.calendarOffset.value, k => o("span", {
    key: "blank" + k
  })), 64)), (p(), g(O, null, oe(__view.calendarDays.value, k => approvedButtonVNode({
    key: k,
    class: ge({
      marked: __view.ee.value.some(ve => ve.date === `${__view.calendarMonth.value}-${String(k).padStart(2,"0")}`),
      selected: __view.he.value === `${__view.calendarMonth.value}-${String(k).padStart(2, "0")}`
    }),
    disabled: `${__view.calendarMonth.value}-${String(k).padStart(2,"0")}`>__view.closedDiaryDate.value,
    "aria-label": `${__view.calendarMonth.value}-${k}`,
    "aria-pressed": __view.he.value === `${__view.calendarMonth.value}-${String(k).padStart(2, "0")}`,
    "data-testid": `calendar-day-${__view.calendarMonth.value}-${String(k).padStart(2, "0")}`,
    onClick: ve => __view.R(`${__view.calendarMonth.value}-${String(k).padStart(2, "0")}`)
  }, [Z(v(k), 1), __view.ee.value.some(ve => ve.date === `${__view.calendarMonth.value}-${String(k).padStart(2,"0")}`) ? (p(), g("i", Ed)) : X("", !0)])), 64))]), b[28] || (b[28] = o("p", {
    class: "calendar-note muted"
  }, "有圆点的日子，藏着一页故事。", -1))]), o("section", Ad, [me(ru, {
    name: __view.j.value,
    mode: "out-in"
  }, {
    default: Ht(() => [__view.x.value ? (p(), g("article", {
      key: __view.x.value.date,
      class: "diary-paper",
      "data-testid": "diary-entry",
      "data-date": __view.x.value.date
    }, [o("small", null, v(__view.joined(__view.x.value.date,__view.x.value.time)), 1), o("h2", null, v(__view.x.value.title), 1), o("div", Od, [__view.x.value.weather?o("span", null, "天气 · " + v(__view.x.value.weather), 1):X("",!0), __view.x.value.mood?o("span", null, "心情 · " + v(__view.x.value.mood), 1):X("",!0)]), o("p", Vd, v(__view.x.value.text), 1), o("p", Pd, "— " + v(M(__view.character).name), 1)], 8, Id)) : (p(), g("div", Ld, [...(b[29] || (b[29] = [o("p", null, "这一天还没有日记。", -1)]))]))]),
    _: 1
  }, 8, ["name"]), o("footer", Ud, [approvedButtonVNode({
    class: "text-button",
    disabled: __view.T.value <= 0,
    "data-testid": "diary-prev",
    onClick: b[4] || (b[4] = k => __view.ye(-1))
  }, "← 上一页"), o("small", Fd, v(__view.x.value ? __view.T.value + 1 : 0) + " / " + v(__view.ee.value.length), 1), approvedButtonVNode({
    class: "text-button",
    disabled: __view.T.value < 0 || __view.T.value >= __view.ee.value.length - 1,
    "data-testid": "diary-next",
    onClick: b[5] || (b[5] = k => __view.ye(1))
  }, "下一页 →")])])])], 64)) : X("", !0), __view.props.page === "记忆" ? (p(), g(O, {
    key: 4
  }, [b[41] || (b[41] = o("p", {
    class: "muted"
  }, "把相处留下的细节，慢慢收进记忆。", -1)), me(__view.FilterChips, {
    search: __view.te.value,
    "onUpdate:search": b[6] || (b[6] = k => __view.te.value = k),
    filters: __view.se.value,
    "onUpdate:filters": b[7] || (b[7] = k => __view.se.value = k),
    groups: __view.ue.value,
    "test-id": "memory-filters",
    placeholder: "搜索记忆 / 标签"
  }, null, 8, ["search", "filters", "groups"]), o("section", Dd, [(p(!0), g(O, null, oe(__view.S.value, k => (p(), g("article", {
    key: k.id,
    class: ge(["memory-item", {
      expanded: __view.I.value === k.id
    }]),
    "data-testid": "memory-row-" + k.id
  }, [approvedButtonVNode({
    class: "memory-summary",
    "aria-expanded": __view.I.value === k.id,
    "aria-controls": "memory-details-" + k.id,
    "data-testid": "memory-toggle-" + k.id,
    onClick: ve => __view.I.value = __view.I.value === k.id ? null : k.id
  }, [o("span", Bd, v(__view.I.value === k.id ? "−" : "+"), 1), o("span", Kd, v(k.text), 1), o("time", null, v(k.date), 1), o("span", Wd, v(k.category), 1)]), __view.I.value === k.id ? (p(), g("div", {
    key: 0,
    id: "memory-details-" + k.id,
    class: "memory-details"
  }, [o("dl", null, [o("div", null, [b[30] || (b[30] = o("dt", null, "来源", -1)), o("dd", null, v(k.source) + " · " + v(k.date), 1)]), o("div", null, [b[31] || (b[31] = o("dt", null, "重要度", -1)), o("dd", {
    class: "memory-stars",
    "aria-label": k.importance + "星，共5星"
  }, [Z(v("★".repeat(k.importance)), 1), o("span", null, v("☆".repeat(5 - k.importance)), 1)], 8, zd)]), o("div", null, [b[32] || (b[32] = o("dt", null, "标签", -1)), o("dd", null, [(p(!0), g(O, null, oe(k.tags, ve => (p(), g("span", {
    key: ve,
    class: "pill"
  }, "# " + v(ve), 1))), 128)), k.tags.length ? X("", !0) : (p(), g("span", Gd, "暂无标签"))])])]), o("div", Jd, [approvedButtonVNode({
    class: "text-button",
    "data-testid": "memory-edit-" + k.id,
    onClick: ve => {
      __view.K.value = {
        ...k,
        tags: [...k.tags]
      }, __view.H.value = "";
    }
  }, "编辑"), approvedButtonVNode({
    class: "text-button",
    "data-testid": "memory-forget-" + k.id,
    onClick: ve => __view.le.value = k
  }, "遗忘")])], 8, qd)) : X("", !0)], 10, jd))), 128))]), __view.S.value.length ? X("", !0) : (p(), approvedSurfaceBlock("div", Qd, [b[33] || (b[33] = o("p", null, "没有符合条件的记忆。", -1)), approvedButtonVNode({
    class: "button",
    "data-testid": "memory-empty-clear",
    onClick: __view.q
  }, "清除筛选")])), __view.K.value ? (p(), Ae(__view.DetailDrawer, {
    key: 1,
    title: "编辑记忆",
    "test-id": "memory-drawer",
    onClose: b[13] || (b[13] = k => __view.K.value = null)
  }, {
    default: Ht(() => [o("form", {
      class: "form-grid",
      onSubmit: Ve(__view._, ["prevent"])
    }, [o("label", null, [b[34] || (b[34] = Z("内容", -1)), pe(o("textarea", {
      "onUpdate:modelValue": b[8] || (b[8] = k => __view.K.value.text = k),
      rows: "4",
      "data-testid": "memory-edit-content"
    }, null, 512), [[$e, __view.K.value.text]]), __view.K.value.text.trim() ? X("", !0) : (p(), g("small", Xd, "请填写记忆内容"))]), o("label", null, [b[35] || (b[35] = Z("来源", -1)), pe(o("input", {
      "onUpdate:modelValue": b[9] || (b[9] = k => __view.K.value.source = k),
      readonly: true
    }, null, 512), [[$e, __view.K.value.source]])]), o("label", null, [b[36] || (b[36] = Z("来源日期", -1)), pe(o("input", {
      "onUpdate:modelValue": b[10] || (b[10] = k => __view.K.value.date = k),
      type: "date",
      readonly: true
    }, null, 512), [[$e, __view.K.value.date]])]), o("fieldset", ef, [b[37] || (b[37] = o("legend", null, "重要度", -1)), o("div", tf, [(p(), g(O, null, oe(5, k => approvedButtonVNode({
      key: k,
      type: "button",
      "aria-label": k + "星",
      "aria-pressed": __view.K.value.importance === k,
      "data-testid": "memory-star-" + k,
      class: ge({
        filled: k <= __view.K.value.importance
      }),
      disabled: true, title: "重要度由记忆记录维护", onClick: () => {}
    }, "★")), 64)), o("small", null, v(__view.K.value.importance) + " / 5", 1)])]), o("fieldset", nf, [b[38] || (b[38] = o("legend", null, "类型", -1)), o("div", lf, [(p(), g(O, null, oe(["长期记忆", "发生过的事情", "最近在想"], k => approvedButtonVNode({
      key: k,
      type: "button",
      class: ge(["filter-chip", {
        active: __view.K.value.category === k
      }]),
      "aria-pressed": __view.K.value.category === k,
      disabled: true, title: "类型由记忆记录维护", onClick: () => {}
    }, v(k))), 64))])]), o("fieldset", rf, [b[39] || (b[39] = o("legend", null, "标签", -1)), o("div", af, [(p(!0), g(O, null, oe(__view.ne.value, k => (p(), approvedButtonBlock({
      key: k,
      type: "button",
      class: ge(["filter-chip", {
        active: __view.K.value.tags.includes(k)
      }]),
      "aria-pressed": __view.K.value.tags.includes(k),
      "data-testid": "memory-tag-" + k,
      onClick: ve => __view.V(k),
      disabled: true,
      title: "\u6765\u6E90\u5206\u7C7B\u4E0E\u8BC4\u5206\u7531\u771F\u5B9E\u8BB0\u5FC6\u8BB0\u5F55\u7EF4\u62A4"
    }, "# " + v(k)))), 128))]), o("div", cf, [pe(o("input", {
      "onUpdate:modelValue": b[11] || (b[11] = k => __view.H.value = k),
      "aria-label": "新标签",
      placeholder: "添加一个新标签",
      "data-testid": "memory-new-tag",
      onKeydown: b[12] || (b[12] = Gi(Ve(k => !k.isComposing && __view.U(), ["prevent"]), ["enter"])),
      disabled: true,
      title: "\u6765\u6E90\u5206\u7C7B\u4E0E\u8BC4\u5206\u7531\u771F\u5B9E\u8BB0\u5FC6\u8BB0\u5F55\u7EF4\u62A4"
    }, null, 544), [[$e, __view.H.value]]), approvedButtonVNode({
      type: "button",
      class: "text-button",
      "data-testid": "memory-add-tag",
      onClick: __view.U,
      disabled: true,
      title: "\u6765\u6E90\u5206\u7C7B\u4E0E\u8BC4\u5206\u7531\u771F\u5B9E\u8BB0\u5FC6\u8BB0\u5F55\u7EF4\u62A4"
    }, "添加")])]), me(__view.ActionFeedback, {
      labels: ["记忆"]
    }), approvedButtonVNode({
      class: "button primary",
      disabled: M(__view.a) || !__view.K.value.text.trim(),
      "data-testid": "memory-save"
    }, v(M(__view.a) ? "处理中…" : "保存"))], 32)]),
    _: 1
  })) : X("", !0), __view.le.value ? (p(), g("div", pf, [approvedSurfaceVNode("section", mf, [b[40] || (b[40] = o("h2", null, "遗忘这段记忆？", -1)), o("p", null, v(__view.le.value.text), 1), o("div", vf, [approvedButtonVNode({
    class: "button",
    onClick: b[14] || (b[14] = k => __view.le.value = null)
  }, "取消"), approvedButtonVNode({
    class: "button primary",
    "data-testid": "memory-confirm-forget",
    onClick: () => __view.forgetMemory(),
    disabled: __view.a.value
  }, "确认遗忘")])])])) : X("", !0)], 64)) : X("", !0), __view.K.value ? X("", !0) : (p(), Ae(__view.ActionFeedback, {
    key: __view.props.page,
    labels: [__view.props.page === "此刻" ? "日程" : __view.props.page]
  }, null, 8, ["labels"]))], 64));
}

export const renderLifePage = unoptimizedRenderer(renderLifePageBody);
